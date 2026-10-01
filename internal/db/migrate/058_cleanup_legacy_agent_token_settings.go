package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 58,
		Up:      migrateCleanupLegacyAgentTokenSettings,
	})
}

// migrateCleanupLegacyAgentTokenSettings 清掉 A 阶段遗留的 agent_api_token_* 设置行。
//
// 为什么需要独立迁移：057 的键表最初漏了 agent_api_token_enabled，而 057 已在线上实例
// 应用并记录，改 057 的内容不会重跑——所以用 058 幂等补齐（键表已修正，全新安装时 057
// 自己就清完了，058 是 no-op）。
func migrateCleanupLegacyAgentTokenSettings(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Setting{}) {
		return nil
	}
	if err := db.Table("settings").Where("key IN ?", legacyAgentTokenSettingKeys).
		Delete(&model.Setting{}).Error; err != nil {
		return fmt.Errorf("cleanup legacy agent token settings: %w", err)
	}
	return nil
}
