package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 33,
		Up:      migrateChannelKeyHealth,
	})
}

// 033: 为 channels 增加 Key 巡检状态字段（issue #142）。
// 新增 key_health_passed / key_health_all_failed / key_health_at 三列，
// 与 groups 的 last_test_passed / last_test_all_failed / last_test_at 同构。
// 幂等：HasColumn 守卫，已存在则跳过。
func migrateChannelKeyHealth(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Channel{}) {
		return nil
	}
	columns := []string{"KeyHealthPassed", "KeyHealthAllFailed", "KeyHealthAt"}
	for _, col := range columns {
		if db.Migrator().HasColumn(&model.Channel{}, col) {
			continue
		}
		if err := db.Migrator().AddColumn(&model.Channel{}, col); err != nil {
			return fmt.Errorf("add channels.%s: %w", col, err)
		}
	}
	return nil
}
