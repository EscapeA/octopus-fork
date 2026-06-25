package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 18,
		Up:      migrateAPIKeyMaxTokens,
	})
}

// 018: 为 api_keys 增加 max_tokens 列（issue #108：Key 支持 Token 用量上限）。
// gorm AutoMigrate 通常也会加列，这里幂等兜底，确保跨方言（SQLite/MySQL/Postgres）
// 以及运行时切换 DB 类型后该列存在。
func migrateAPIKeyMaxTokens(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.APIKey{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.APIKey{}, "MaxTokens") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.APIKey{}, "MaxTokens"); err != nil {
		return fmt.Errorf("add column max_tokens: %w", err)
	}
	return nil
}
