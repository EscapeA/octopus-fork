package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 20,
		Up:      migrateGroupLastTestStatus,
	})
}

// 020: 为 groups 增加 last_test_passed / last_test_at 列（issue #113）。
//
// 记录分组最近一次模型测试的成败状态，前端据此对失败分组做灰色化标记。
// gorm AutoMigrate 通常也会加列，这里幂等兜底，确保跨方言（SQLite/MySQL/Postgres）
// 以及运行时切换 DB 类型后该列存在。
func migrateGroupLastTestStatus(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Group{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.Group{}, "LastTestPassed") {
		if err := db.Migrator().AddColumn(&model.Group{}, "LastTestPassed"); err != nil {
			return fmt.Errorf("add column last_test_passed: %w", err)
		}
	}
	if !db.Migrator().HasColumn(&model.Group{}, "LastTestAt") {
		if err := db.Migrator().AddColumn(&model.Group{}, "LastTestAt"); err != nil {
			return fmt.Errorf("add column last_test_at: %w", err)
		}
	}
	return nil
}
