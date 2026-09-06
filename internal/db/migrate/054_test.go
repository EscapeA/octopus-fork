package migrate

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

// 054 迁移测试：为 channels 补 param_override_force 列，幂等。
func TestMigrateChannelParamOverrideForce(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	// 模拟旧版 channels 表（无 param_override_force 列）
	if err := gormDB.Exec(`CREATE TABLE channels (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		param_override TEXT
	)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := gormDB.Exec(`INSERT INTO channels (id, name, param_override) VALUES (1, 'c1', '{"temperature":0.5}')`).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	if err := migrateChannelParamOverrideForce(gormDB); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !gormDB.Migrator().HasColumn(&model.Channel{}, "ParamOverrideForce") {
		t.Fatal("param_override_force column not added")
	}

	// 幂等：重复执行不报错
	if err := migrateChannelParamOverrideForce(gormDB); err != nil {
		t.Fatalf("second migrate should be a no-op: %v", err)
	}

	// 存量行可读，bool 默认 false（客户端优先语义不变）
	var got model.Channel
	if err := gormDB.First(&got, 1).Error; err != nil {
		t.Fatalf("load channel after migration: %v", err)
	}
	if got.ParamOverrideForce {
		t.Fatal("default ParamOverrideForce must be false")
	}
	if got.ParamOverride == nil || *got.ParamOverride != `{"temperature":0.5}` {
		t.Fatalf("ParamOverride = %v, want original JSON", got.ParamOverride)
	}
}
