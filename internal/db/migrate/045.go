package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 45,
		Up:      migratePlanProviderTotalUsed,
	})
}

// 045: 为 plan_providers 表增加累计已用额度字段（TotalUsed）。
// gorm AutoMigrate 也会加列，这里幂等兜底。
func migratePlanProviderTotalUsed(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.PlanProvider{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.PlanProvider{}, "TotalUsed") {
		if err := db.Migrator().AddColumn(&model.PlanProvider{}, "TotalUsed"); err != nil {
			return fmt.Errorf("add column TotalUsed: %w", err)
		}
	}
	return nil
}
