package dbmigration

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
)

// TestMigratePreservesRelayLogsCompositeIndexes 验证数据库类型切换迁移后，
// relay_logs 的复合索引 (channel_id, time) 与 (request_api_key_id, time) 存在。
// 迁移流程：OpenStandalone → db.Migrate(AutoMigrate + AfterAutoMigrate 含 019)
// → ImportWithModeToDB。AutoMigrate 依据 struct tag 建索引，019 迁移兜底。
func TestMigratePreservesRelayLogsCompositeIndexes(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("OCTOPUS_DATA_DIR", dataDir)

	sourceDSN := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()+"-source")
	if err := db.InitDB("sqlite", sourceDSN, false); err != nil {
		t.Fatalf("init source db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := op.InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}
	if err := op.UserBootstrapCreate("admin", "super-secret-123"); err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

	targetPath := filepath.Join(dataDir, "target-composite.db")
	restore := SetSaveDatabaseConfigFuncForTest(func(string, string) error { return nil })
	defer restore()

	if _, err := Migrate(context.Background(), model.DatabaseMigrationRequest{
		Type: "sqlite",
		Path: targetPath,
	}); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	target, err := db.OpenStandalone("sqlite", targetPath, false)
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	sqlDB, err := target.DB()
	if err != nil {
		t.Fatalf("target DB(): %v", err)
	}
	defer sqlDB.Close()

	// 验证两个复合索引在目标库存在。
	// relay_logs 默认不随迁移导出（IncludeLogs=false），这里只验证索引结构。
	for _, idxName := range []string{
		"idx_relay_logs_channel_time",
		"idx_relay_logs_apikey_time",
	} {
		var got string
		if err := target.Raw(`
			SELECT name
			FROM sqlite_master
			WHERE type = 'index' AND tbl_name = 'relay_logs' AND name = ?
			LIMIT 1
		`, idxName).Scan(&got).Error; err != nil {
			t.Fatalf("read index %s on target: %v", idxName, err)
		}
		if got != idxName {
			t.Fatalf("expected index %q on target after migration, got %q", idxName, got)
		}
	}
}
