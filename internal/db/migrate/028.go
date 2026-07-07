package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 28,
		Up:      migrateGroupCategories,
	})
}

// 028: 为路由分组增加分类，并为 API Key 增加可调用分组分类白名单。
func migrateGroupCategories(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasColumn(&model.Group{}, "Category") {
		if err := db.Migrator().AddColumn(&model.Group{}, "Category"); err != nil {
			return fmt.Errorf("add groups.category: %w", err)
		}
	}
	if !db.Migrator().HasColumn(&model.APIKey{}, "AllowedGroupCategories") {
		if err := db.Migrator().AddColumn(&model.APIKey{}, "AllowedGroupCategories"); err != nil {
			return fmt.Errorf("add api_keys.allowed_group_categories: %w", err)
		}
	}
	return nil
}
