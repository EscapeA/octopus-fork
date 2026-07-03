package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 23,
		Up:      migratePlanProviderForwardAPIKey,
	})
}

// 023: 为 plan_providers 增加 forward_api_key 列。
// StepFun Plan 监控的主凭据是 Oasis-Token（存 api_key 字段，用于查套餐），
// forward_api_key 存可选的 sk- API Key，用于自动创建/复用转发渠道。
// gorm AutoMigrate 通常也会加列，这里幂等兜底，确保跨方言（SQLite/MySQL/Postgres）
// 以及运行时切换 DB 类型后该列存在。
func migratePlanProviderForwardAPIKey(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.PlanProvider{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.PlanProvider{}, "ForwardAPIKey") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.PlanProvider{}, "ForwardAPIKey"); err != nil {
		return fmt.Errorf("add column forward_api_key: %w", err)
	}
	return nil
}
