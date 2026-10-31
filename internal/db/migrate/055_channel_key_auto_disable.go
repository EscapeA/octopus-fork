package migrate

import (
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 55,
		Up:      migrateChannelKeyAutoDisable,
	})
}

// 055 历史上为 channel_keys 增加 auto_disabled_at / auto_disable_reason 两列，
// 服务于「上游连续 402 自动禁用 Key」。该功能已整体移除，字段随之删除；
// 迁移号保留为 no-op（老库已记录该版本、且不删已有列，数据无损）。
func migrateChannelKeyAutoDisable(_ *gorm.DB) error {
	return nil
}
