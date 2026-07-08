package db

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// legacyGroupWithUniqueName 模拟 issue #131 之前的 groups 表结构：
// Name 字段使用 gorm:"unique"（字段级 UNIQUE 约束），GORM 会创建
// CONSTRAINT uni_groups_name UNIQUE (name) + sqlite_autoindex_groups_1。
// commit 57cb548f7 将其改为 uniqueIndex:idx_groups_endpoint_name,priority:2，
// GORM AutoMigrate 检测到字段级 UNIQUE 被移除，调用 glebarez DropConstraint
// -> recreateTable -> DROP TABLE groups，但 group_items 有 FK 引用 groups，
// foreign_keys=ON 时报 "FOREIGN KEY constraint failed (787)" 启动崩溃。
type legacyGroupWithUniqueName struct {
	ID           int                             `gorm:"primaryKey"`
	Name         string                          `gorm:"unique;not null;size:191"`
	EndpointType string                          `gorm:"not null;default:*;index;size:191"`
	Mode         int                             `gorm:"not null"`
	Items        []legacyGroupItemWithFKToGroups `gorm:"foreignKey:GroupID"`
}

type legacyGroupItemWithFKToGroups struct {
	ID        int    `gorm:"primaryKey"`
	GroupID   int    `gorm:"not null"`
	ChannelID int    `gorm:"not null"`
	ModelName string `gorm:"not null;size:191"`
}

func (legacyGroupWithUniqueName) TableName() string     { return "groups" }
func (legacyGroupItemWithFKToGroups) TableName() string { return "group_items" }

// TestMigrateSQLiteDropsGroupsUniqueConstraintWithChildFK 是 issue #131 的回归测试：
// 旧库 groups 表有字段级 UNIQUE(name) 约束 + group_items 子表 FK 引用 groups(id)。
// 升级到新模型后 AutoMigrate 需删除该 UNIQUE 约束（recreateTable 路径执行 DROP TABLE groups），
// 修复前 foreign_keys=ON 阻止 DROP TABLE 报 787 崩溃；修复后 Migrate 临时关闭 foreign_keys，
// 迁移成功且数据/约束/外键校验完整恢复。
func TestMigrateSQLiteDropsGroupsUniqueConstraintWithChildFK(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "issue131.db")
	dsn := dbPath + "?_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)"

	// Phase 1: 用旧模型建库 + 种子数据，产生 fields 级 UNIQUE + 子表 FK。
	legacy, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open legacy sqlite: %v", err)
	}
	if err := legacy.AutoMigrate(&legacyGroupWithUniqueName{}, &legacyGroupItemWithFKToGroups{}); err != nil {
		t.Fatalf("legacy AutoMigrate: %v", err)
	}
	if err := legacy.Create(&legacyGroupWithUniqueName{
		Name: "gpt-4", EndpointType: "chat", Mode: 1,
		Items: []legacyGroupItemWithFKToGroups{{ChannelID: 1, ModelName: "gpt-4"}},
	}).Error; err != nil {
		t.Fatalf("seed legacy group: %v", err)
	}

	// 确认旧库确实有字段级 UNIQUE 约束（sqlite_autoindex）+ 子表 FK。
	var groupsDDL string
	if err := legacy.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='groups'").Scan(&groupsDDL).Error; err != nil {
		t.Fatalf("read groups DDL: %v", err)
	}
	if !contains(groupsDDL, "UNIQUE") {
		t.Fatalf("legacy groups DDL missing UNIQUE constraint (precondition):\n%s", groupsDDL)
	}
	var itemsDDL string
	if err := legacy.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='group_items'").Scan(&itemsDDL).Error; err != nil {
		t.Fatalf("read group_items DDL: %v", err)
	}
	if !contains(itemsDDL, "FOREIGN KEY") {
		t.Fatalf("legacy group_items DDL missing FOREIGN KEY (precondition):\n%s", itemsDDL)
	}

	sqlDB, _ := legacy.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close legacy conn: %v", err)
	}

	// Phase 2: 用当前模型运行 db.Migrate（复现 issue #131 的崩溃路径）。
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open sqlite for migrate: %v", err)
	}
	sqlDB2, _ := conn.DB()
	sqlDB2.SetMaxOpenConns(1)
	sqlDB2.SetMaxIdleConns(1)
	sqlDB2.SetConnMaxLifetime(0)
	sqlDB2.SetConnMaxIdleTime(0)
	t.Cleanup(func() { _ = sqlDB2.Close() })

	if err := Migrate(conn); err != nil {
		t.Fatalf("Migrate failed (issue #131 regression): %v", err)
	}

	// Phase 3: 迁移后不变量校验。
	// (a) foreign_keys 恢复为 ON。
	var fk int
	if err := conn.Raw(`PRAGMA foreign_keys`).Scan(&fk).Error; err != nil {
		t.Fatalf("read foreign_keys after migrate: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d after migrate, want 1 (not restored)", fk)
	}

	// (b) 数据未丢失。
	var groupCount, itemCount int64
	if err := conn.Model(&model.Group{}).Count(&groupCount).Error; err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if err := conn.Model(&model.GroupItem{}).Count(&itemCount).Error; err != nil {
		t.Fatalf("count group_items: %v", err)
	}
	if groupCount != 1 || itemCount != 1 {
		t.Fatalf("data lost: groups=%d group_items=%d, want 1/1", groupCount, itemCount)
	}

	// (c) 新复合唯一索引生效：同名不同 endpoint 允许，同名同 endpoint 拒绝。
	if err := conn.Create(&model.Group{Name: "shared", EndpointType: model.EndpointTypeEmbeddings, Mode: model.GroupModeRoundRobin}).Error; err != nil {
		t.Fatalf("create same-name different-endpoint group: %v", err)
	}
	if err := conn.Create(&model.Group{Name: "gpt-4", EndpointType: model.EndpointTypeChat, Mode: model.GroupModeRoundRobin}).Error; err == nil {
		t.Fatal("duplicate same-name same-endpoint group should be rejected by idx_groups_endpoint_name")
	}

	// (d) 外键校验恢复：孤儿 group_items 插入应被拒绝。
	if err := conn.Create(&model.GroupItem{GroupID: 99999, ChannelID: 1, ModelName: "orphan"}).Error; err == nil {
		t.Fatal("orphan group_item inserted; FK enforcement not restored after migrate")
	}
}

// TestDisableSQLiteForeignKeysForMigrationNoOpWhenAlreadyOff 验证当 foreign_keys
// 已经是 OFF 时，函数不会重复切换（返回 nil 恢复函数），避免无意义的 PRAGMA 往返。
func TestDisableSQLiteForeignKeysForMigrationNoOpWhenAlreadyOff(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fk-off.db")
	// 打开一个不带 foreign_keys(ON) pragma 的 SQLite 连接（驱动默认 OFF）。
	conn, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := conn.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	var enabled int
	conn.Raw(`PRAGMA foreign_keys`).Scan(&enabled)
	if enabled != 0 {
		t.Fatalf("precondition: foreign_keys = %d, want 0 (driver default)", enabled)
	}

	restore, err := disableSQLiteForeignKeysForMigration(conn)
	if err != nil {
		t.Fatalf("expected nil err when FK already off, got %v", err)
	}
	if restore != nil {
		t.Fatalf("expected nil restore func when FK already off, got non-nil")
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
