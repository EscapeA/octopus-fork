package migrate

import (
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 54,
		Up:      migrateChannelParamOverrideForce,
	})
}

// 054 历史上为 channels 增加 param_override_force 列（参数覆盖强制开关，
// 商汤时期的临时改造）。该功能已整体移除，模型字段随之删除。
// 迁移号保留为 no-op：老库已记录该版本，删号会导致启动失败；也不主动删除
// 已有列（数据无损，留作历史痕迹）。
func migrateChannelParamOverrideForce(_ *gorm.DB) error {
	return nil
}
