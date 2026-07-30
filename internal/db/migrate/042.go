package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 42,
		Up:      migratePlanProviderTeamFields,
	})
}

// 042: 为 plan_providers 增加 team_organization_id / team_project_id 列
// （智谱 GLM 团队套餐 zhipu_team 专用，请求头 bigmodel-organization / bigmodel-project）。
// gorm AutoMigrate 通常也会加列，这里幂等兜底，确保跨方言以及运行时切换 DB 类型后该列存在。
func migratePlanProviderTeamFields(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.PlanProvider{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.PlanProvider{}, "TeamOrganizationID") {
		if err := db.Migrator().AddColumn(&model.PlanProvider{}, "TeamOrganizationID"); err != nil {
			return fmt.Errorf("add column team_organization_id: %w", err)
		}
	}
	if !db.Migrator().HasColumn(&model.PlanProvider{}, "TeamProjectID") {
		if err := db.Migrator().AddColumn(&model.PlanProvider{}, "TeamProjectID"); err != nil {
			return fmt.Errorf("add column team_project_id: %w", err)
		}
	}
	return nil
}
