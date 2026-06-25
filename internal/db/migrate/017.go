package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 17,
		Up:      migrateChannelKeySelectionStrategy,
	})
}

// 017: 为 channels 增加 key_selection_strategy 列。
// 空 = 继承全局设置；"cost" = 成本最低优先；"availability" = 可用度优先。
// gorm AutoMigrate 通常也会加列，这里幂等兜底，确保跨方言（SQLite/MySQL/Postgres）
// 以及运行时切换 DB 类型后该列存在。
func migrateChannelKeySelectionStrategy(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Channel{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.Channel{}, "KeySelectionStrategy") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.Channel{}, "KeySelectionStrategy"); err != nil {
		return fmt.Errorf("add column key_selection_strategy: %w", err)
	}
	return nil
}
