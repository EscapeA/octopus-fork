package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 16,
		Up:      migrateChannelSkipModelTest,
	})
}

// 016: 为 channels 增加 skip_model_test 列（issue #98：按渠道排除模型可用性测试）。
// gorm AutoMigrate 通常也会加列，这里幂等兜底，确保跨方言（SQLite/MySQL/Postgres）
// 以及运行时切换 DB 类型后该列存在。
func migrateChannelSkipModelTest(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Channel{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.Channel{}, "SkipModelTest") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.Channel{}, "SkipModelTest"); err != nil {
		return fmt.Errorf("add column skip_model_test: %w", err)
	}
	return nil
}
