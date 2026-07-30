package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 43,
		Up:      migratePoolAccountLifecycleFields,
	})
}

// 043: 为 pool_accounts 表增加 P0/P1/P2/P3 号池生命周期与调度字段
// （对齐 sub2api 2026-07-27 差距，见 plan pool-sub2api-gap-completion-plan）。
// gorm AutoMigrate 也会加列，这里幂等兜底。
func migratePoolAccountLifecycleFields(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.PoolAccount{}) {
		return nil
	}
	columns := []string{
		"TempUnschedUntil",
		"TempUnschedReason",
		"AuthErrorCount",
		"AuthErrorWindowStart",
		"ExpiresAt",
		"AutoPauseOnExpired",
		"Extra",
		"Weight",
		"LoadFactor",
	}
	for _, col := range columns {
		if !db.Migrator().HasColumn(&model.PoolAccount{}, col) {
			if err := db.Migrator().AddColumn(&model.PoolAccount{}, col); err != nil {
				return fmt.Errorf("add column %s: %w", col, err)
			}
		}
	}
	return nil
}
