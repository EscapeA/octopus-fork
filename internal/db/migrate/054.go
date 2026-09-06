package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 54,
		Up:      migrateChannelParamOverrideForce,
	})
}

// 054: 为 channels 增加 param_override_force 列（参数覆盖强制开关）。
// 开启后白名单字段（当前仅 reasoning_effort）无视客户端取值强制覆盖；
// 关闭时保持原有客户端优先语义。与 039/053 同模式：GORM AutoMigrate 通常
// 也会加列，这里幂等兜底，确保跨方言（SQLite/MySQL/Postgres）以及运行时
// 切换 DB 类型后该列存在。
func migrateChannelParamOverrideForce(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Channel{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.Channel{}, "ParamOverrideForce") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.Channel{}, "ParamOverrideForce"); err != nil {
		return fmt.Errorf("add column param_override_force: %w", err)
	}
	return nil
}
