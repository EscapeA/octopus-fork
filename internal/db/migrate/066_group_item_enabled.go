package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 66,
		Up:      migrateGroupItemEnabled,
	})
}

// 066: group_items 表增加 enabled 列（「模型 + 渠道」行的启用开关）。
//
// 用途：某个模型被上游限流/欠费时，快速摘除该分组项而不必「删除→稍后重新添加」。
// GroupItem.Enabled 是指针类型，nil 语义为「启用」，因此：
//   - 加列前的存量行 ALTER 后为 NULL → 扫成 nil → 视为启用，无需回填；
//   - 更新请求未携带该字段时保持 nil → 不改动（与 ChannelKeyUpdateRequest.Enabled 同族语义）。
//
// 幂等：HasColumn 守卫，重复执行安全。
func migrateGroupItemEnabled(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.GroupItem{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.GroupItem{}, "Enabled") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.GroupItem{}, "Enabled"); err != nil {
		return fmt.Errorf("add group_items.enabled: %w", err)
	}
	return nil
}
