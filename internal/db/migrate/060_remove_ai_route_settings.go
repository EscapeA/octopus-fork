package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 60,
		Up:      migrateRemoveAIRouteSettings,
	})
}

// legacyAIRouteSettingKeys 是「AI 自动路由」功能移除后遗留的设置键。
// 生成路由表的整条链路（服务池配置、批次并发、超时、目标分组）已删除，这些行不会再被读取，
// 且会在设置列表里暴露无对应页面的孤儿项，故一次性清理。
var legacyAIRouteSettingKeys = []string{
	"ai_route_group_id",
	"ai_route_base_url",
	"ai_route_api_key",
	"ai_route_model",
	"ai_route_timeout_seconds",
	"ai_route_parallelism",
	"ai_route_services",
	"ai_route_max_models_per_request",
}

func migrateRemoveAIRouteSettings(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Setting{}) {
		return nil
	}
	if err := db.Table("settings").Where("key IN ?", legacyAIRouteSettingKeys).
		Delete(&model.Setting{}).Error; err != nil {
		return fmt.Errorf("cleanup legacy ai route settings: %w", err)
	}
	return nil
}
