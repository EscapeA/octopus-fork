package migrate

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

// TestMigrateChannelKeyHealthAddsColumns 验证 033 迁移为 channels 表添加
// key_health_passed / key_health_all_failed / key_health_at 三列。
func TestMigrateChannelKeyHealthAddsColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "key-health.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	// AutoMigrate 建表（含新字段声明），模拟正常启动路径。
	if err := db.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("auto migrate channels: %v", err)
	}

	// 迁移应能识别已存在的列并跳过（幂等）。
	if err := migrateChannelKeyHealth(db); err != nil {
		t.Fatalf("migrateChannelKeyHealth: %v", err)
	}

	for _, col := range []string{"key_health_passed", "key_health_all_failed", "key_health_at"} {
		if !db.Migrator().HasColumn(&model.Channel{}, col) {
			t.Fatalf("expected column %q to exist on channels table", col)
		}
	}
}

// TestMigrateChannelKeyHealthOnLegacyTable 验证迁移在旧库（无新列）上能补建。
func TestMigrateChannelKeyHealthOnLegacyTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "key-health-legacy.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	// 手动建旧版 channels 表（不含 KeyHealth 字段），模拟升级前旧库。
	if err := db.Exec(`CREATE TABLE channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		group_id INTEGER NOT NULL DEFAULT 0,
		type INTEGER,
		enabled INTEGER DEFAULT 1,
		base_urls TEXT,
		keys TEXT,
		model TEXT,
		custom_model TEXT,
		created_at INTEGER,
		updated_at INTEGER
	)`).Error; err != nil {
		t.Fatalf("create legacy channels table: %v", err)
	}

	// 插入一条旧数据
	if err := db.Exec(`INSERT INTO channels (name, group_id, type, enabled) VALUES ('legacy-ch', 0, 1, 1)`).Error; err != nil {
		t.Fatalf("insert legacy channel: %v", err)
	}

	// 运行迁移
	if err := migrateChannelKeyHealth(db); err != nil {
		t.Fatalf("migrateChannelKeyHealth on legacy: %v", err)
	}

	// 验证三列已添加
	for _, col := range []string{"key_health_passed", "key_health_all_failed", "key_health_at"} {
		if !db.Migrator().HasColumn(&model.Channel{}, col) {
			t.Fatalf("expected column %q to exist after migration on legacy table", col)
		}
	}

	// 验证旧数据未丢失
	var name string
	if err := db.Raw(`SELECT name FROM channels WHERE id = 1`).Scan(&name).Error; err != nil {
		t.Fatalf("read legacy channel name: %v", err)
	}
	if name != "legacy-ch" {
		t.Fatalf("legacy channel name = %q, want %q", name, "legacy-ch")
	}
}

// TestMigrateChannelKeyHealthIdempotent 验证多次运行迁移不会报错。
func TestMigrateChannelKeyHealthIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "key-health-idempotent.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	if err := db.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("auto migrate channels: %v", err)
	}

	// 运行两次
	for i := 0; i < 2; i++ {
		if err := migrateChannelKeyHealth(db); err != nil {
			t.Fatalf("migrateChannelKeyHealth run %d: %v", i+1, err)
		}
	}
}
