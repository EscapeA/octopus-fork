package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 24,
		Up:      migrateChannelKeyPriority,
	})
}

// 024: 为 channel_keys 增加 priority 列，用于 priority Key 调用策略。
// gorm AutoMigrate 通常也会加列，这里幂等兜底，确保跨方言（SQLite/MySQL/Postgres）
// 以及运行时切换 DB 类型后该列存在。
func migrateChannelKeyPriority(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.ChannelKey{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.ChannelKey{}, "Priority") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.ChannelKey{}, "Priority"); err != nil {
		return fmt.Errorf("add column priority: %w", err)
	}
	return nil
}
