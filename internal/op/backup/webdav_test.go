package backup

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseWebDAVResponse_ExtractsFileMetadata(t *testing.T) {
	// A typical PROPFIND Depth:1 response from the backup directory: the
	// directory itself plus one backup file. The directory entry must be
	// skipped, and the file entry must carry its size and last-modified date.
	xmlBody := `<?xml version="1.0" encoding="utf-8" ?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/octopus-backup/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/octopus-backup/octopus-backup-20260619-120000.json</D:href>
    <D:propstat>
      <D:prop>
        <D:getcontentlength>2048</D:getcontentlength>
        <D:getlastmodified>Mon, 19 Jun 2026 12:00:00 GMT</D:getlastmodified>
        <D:resourcetype/>
      </D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`

	files := parseWebDAVResponse(xmlBody, "/octopus-backup/")
	if len(files) != 1 {
		t.Fatalf("expected 1 file (directory skipped), got %d", len(files))
	}

	f := files[0]
	if f.Name != "octopus-backup-20260619-120000.json" {
		t.Fatalf("name = %q, want octopus-backup-20260619-120000.json", f.Name)
	}
	if f.Path != "/octopus-backup/octopus-backup-20260619-120000.json" {
		t.Fatalf("path = %q", f.Path)
	}
	if f.IsDir {
		t.Fatalf("file should not be flagged as directory")
	}
	if f.Size != 2048 {
		t.Fatalf("size = %d, want 2048", f.Size)
	}
	wantTime := time.Date(2026, time.June, 19, 12, 0, 0, 0, time.UTC)
	if !f.LastModified.Equal(wantTime) {
		t.Fatalf("last_modified = %v, want %v", f.LastModified, wantTime)
	}
}

func TestParseWebDAVResponse_ReturnsEmptySliceWhenNoFiles(t *testing.T) {
	// Only the queried directory is present. The parser must return a non-nil
	// (empty) slice so that downstream JSON serialization yields `[]` rather
	// than `null`, which the frontend treats as a missing array.
	xmlBody := `<?xml version="1.0" encoding="utf-8" ?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/octopus-backup/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`

	files := parseWebDAVResponse(xmlBody, "/octopus-backup/")
	if files == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestWebDAVFile_JSONTagsMatchFrontendContract(t *testing.T) {
	// The frontend WebDAVFile interface expects snake_case field names:
	// name, path, size, last_modified, is_dir. Guard against regressions
	// where the struct loses its JSON tags.
	file := WebDAVFile{
		Name:         "backup.json",
		Path:         "/octopus-backup/backup.json",
		Size:         1024,
		LastModified: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
		IsDir:        false,
	}

	data, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"name", "path", "size", "last_modified", "is_dir"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("JSON output missing expected field %q (got keys: %v)", key, decoded)
		}
	}

	// PascalCase keys (the Go field names without tags) must NOT appear.
	for _, key := range []string{"Name", "Path", "Size", "LastModified", "IsDir"} {
		if _, ok := decoded[key]; ok {
			t.Fatalf("JSON output should not contain PascalCase field %q", key)
		}
	}
}
