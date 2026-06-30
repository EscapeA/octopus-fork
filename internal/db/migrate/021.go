package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 21,
		Up:      migrateGroupLastTestAllFailed,
	})
}

// 021: 为 groups 增加 last_test_all_failed 列（issue #119）。
//
// 区分"全部失败"与"部分失败"，前端据此决定是否对整张卡片灰色化
// （全部失败）还是仅标记部分失败（卡片边框保持正常）。
// gorm AutoMigrate 通常也会加列，这里幂等兜底。
func migrateGroupLastTestAllFailed(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Group{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.Group{}, "LastTestAllFailed") {
		if err := db.Migrator().AddColumn(&model.Group{}, "LastTestAllFailed"); err != nil {
			return fmt.Errorf("add column last_test_all_failed: %w", err)
		}
	}
	return nil
}
