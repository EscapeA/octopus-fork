package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 55,
		Up:      migrateChannelKeyAutoDisable,
	})
}

// 055: channel_keys 表增加 auto_disabled_at / auto_disable_reason 两列。
//
// 用于记录「系统自动禁用该 Key」的时间与原因（上游连续返回 402 余额不足/欠费达阈值，
// 见 internal/relay/insufficient_balance_guard.go）。Enabled=false 才是生效的禁用开关，
// 这两列只承载展示信息（前端徽标/原因）与定时试活恢复的判定依据。
// 幂等：HasColumn 守卫，重复执行安全；存量行两列为 NULL/空。
func migrateChannelKeyAutoDisable(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.ChannelKey{}) {
		return nil
	}
	for _, column := range []string{"AutoDisabledAt", "AutoDisableReason"} {
		if db.Migrator().HasColumn(&model.ChannelKey{}, column) {
			continue
		}
		if err := db.Migrator().AddColumn(&model.ChannelKey{}, column); err != nil {
			return fmt.Errorf("add column channel_keys.%s: %w", column, err)
		}
	}
	return nil
}
