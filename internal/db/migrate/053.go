package migrate

import (
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 53,
		Up:      migratePlanProviderPoolsJSON,
	})
}

// 053 历史上为 plan_providers 增加 pools_json 列（SenseNova 分池明细）。
// fork 已改为跟随上游的 DedicatedPool* 独立列方案，该字段不存在了；
// 迁移号保留为 no-op，避免老库因缺号而报错，也不删除已有列（数据无损）。
func migratePlanProviderPoolsJSON(_ *gorm.DB) error {
	return nil
}
