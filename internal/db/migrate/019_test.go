package migrate

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

// TestEnsureRelayLogsCompositeIndexes 验证 019 迁移为 relay_logs 创建
// (channel_id, time) 和 (request_api_key_id, time) 复合索引。
func TestEnsureRelayLogsCompositeIndexes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "relaylog-composite.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	// AutoMigrate 依据 struct tag 建表（含复合索引声明），模拟正常启动路径。
	if err := db.AutoMigrate(&model.RelayLog{}); err != nil {
		t.Fatalf("auto migrate relay_logs: %v", err)
	}

	// 迁移应能识别已存在的索引并跳过（幂等）。
	if err := ensureRelayLogsCompositeIndexes(db); err != nil {
		t.Fatalf("ensureRelayLogsCompositeIndexes: %v", err)
	}

	for _, idxName := range []string{
		"idx_relay_logs_channel_time",
		"idx_relay_logs_apikey_time",
	} {
		var got string
		if err := db.Raw(`
			SELECT name
			FROM sqlite_master
			WHERE type = 'index' AND tbl_name = 'relay_logs' AND name = ?
			LIMIT 1
		`, idxName).Scan(&got).Error; err != nil {
			t.Fatalf("read index %s: %v", idxName, err)
		}
		if got != idxName {
			t.Fatalf("expected index %q to exist, got %q", idxName, got)
		}
	}
}

// TestEnsureRelayLogsCompositeIndexesOnLegacyTable 验证迁移在
// AutoMigrate 已建表但未建复合索引的旧库上能补建索引。
func TestEnsureRelayLogsCompositeIndexesOnLegacyTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "relaylog-legacy.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	// 模拟旧库：只建表，不带复合索引 tag（直接 CREATE TABLE）。
	if err := db.Exec(`
		CREATE TABLE relay_logs (
			id INTEGER PRIMARY KEY,
			time INTEGER,
			request_model_name TEXT,
			request_api_key_id INTEGER,
			channel_id INTEGER
		)
	`).Error; err != nil {
		t.Fatalf("create legacy relay_logs table: %v", err)
	}

	if err := ensureRelayLogsCompositeIndexes(db); err != nil {
		t.Fatalf("ensureRelayLogsCompositeIndexes on legacy table: %v", err)
	}

	for _, idxName := range []string{
		"idx_relay_logs_channel_time",
		"idx_relay_logs_apikey_time",
	} {
		var got string
		if err := db.Raw(`
			SELECT name
			FROM sqlite_master
			WHERE type = 'index' AND tbl_name = 'relay_logs' AND name = ?
			LIMIT 1
		`, idxName).Scan(&got).Error; err != nil {
			t.Fatalf("read index %s: %v", idxName, err)
		}
		if got != idxName {
			t.Fatalf("expected index %q to exist on legacy table, got %q", idxName, got)
		}
	}
}
