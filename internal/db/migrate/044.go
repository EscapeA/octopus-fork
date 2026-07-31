package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 44,
		Up:      migratePlanProviderRefreshFields,
	})
}

// 044: 为 plan_providers 表增加自动刷新与增量快照字段
// （RefreshIntervalMin 单个覆盖间隔、LastBalance/LastQuotaUsed 上次刷新快照）。
// gorm AutoMigrate 也会加列，这里幂等兜底。
func migratePlanProviderRefreshFields(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.PlanProvider{}) {
		return nil
	}
	columns := []string{
		"RefreshIntervalMin",
		"LastBalance",
		"LastQuotaUsed",
	}
	for _, col := range columns {
		if !db.Migrator().HasColumn(&model.PlanProvider{}, col) {
			if err := db.Migrator().AddColumn(&model.PlanProvider{}, col); err != nil {
				return fmt.Errorf("add column %s: %w", col, err)
			}
		}
	}
	return nil
}
