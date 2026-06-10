package relaylog

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

func TestRelayLogFlushToDBSkipsDuplicateIDsAndTruncatesCache(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	existing := model.RelayLog{ID: 101, Time: 1, RequestModelName: "existing"}
	if err := db.GetDB().Create(&existing).Error; err != nil {
		t.Fatalf("seed relay log failed: %v", err)
	}

	relayLogCacheLock.Lock()
	relayLogCache = []model.RelayLog{
		{ID: 101, Time: 2, RequestModelName: "duplicate"},
		{ID: 102, Time: 3, RequestModelName: "new"},
	}
	relayLogCacheLock.Unlock()
	t.Cleanup(func() {
		relayLogCacheLock.Lock()
		relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
		relayLogCacheLock.Unlock()
	})

	if err := relayLogFlushToDB(context.Background()); err != nil {
		t.Fatalf("relayLogFlushToDB returned error: %v", err)
	}

	var count int64
	if err := db.GetDB().Model(&model.RelayLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count relay logs failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("relay log count = %d, want 2", count)
	}

	var inserted model.RelayLog
	if err := db.GetDB().First(&inserted, "id = ?", 102).Error; err != nil {
		t.Fatalf("new relay log was not inserted: %v", err)
	}

	relayLogCacheLock.Lock()
	cacheLen := len(relayLogCache)
	relayLogCacheLock.Unlock()
	if cacheLen != 0 {
		t.Fatalf("relay log cache len = %d, want 0", cacheLen)
	}
}

func TestRelayLogCleanupAll(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog-cleanup.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	// Seed DB with a mix of success and error logs
	seed := []model.RelayLog{
		{ID: 401, Time: 1, RequestModelName: "model-a", Error: ""},
		{ID: 402, Time: 2, RequestModelName: "model-b", Error: "timeout"},
		{ID: 403, Time: 3, RequestModelName: "model-c", Error: ""},
	}
	if err := db.GetDB().Create(&seed).Error; err != nil {
		t.Fatalf("seed relay logs failed: %v", err)
	}

	var before int64
	db.GetDB().Model(&model.RelayLog{}).Count(&before)
	if before != 3 {
		t.Fatalf("expected 3 seeded logs, got %d", before)
	}

	if err := relayLogCleanupAll(context.Background()); err != nil {
		t.Fatalf("relayLogCleanupAll returned error: %v", err)
	}

	var after int64
	if err := db.GetDB().Model(&model.RelayLog{}).Count(&after).Error; err != nil {
		t.Fatalf("count after cleanup failed: %v", err)
	}
	if after != 0 {
		t.Fatalf("relay log count = %d, want 0 (all logs should be deleted)", after)
	}
}
