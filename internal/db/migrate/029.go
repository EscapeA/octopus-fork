package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 29,
		Up:      migrateAlertRuleScopes,
	})
}

// 029: 为告警规则增加错误率滑动窗口与作用域字段（issue #128）。
func migrateAlertRuleScopes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.AlertRule{}) {
		return nil
	}
	columns := []string{"WindowSec", "ScopeGroupID", "ScopeModelName"}
	for _, column := range columns {
		if db.Migrator().HasColumn(&model.AlertRule{}, column) {
			continue
		}
		if err := db.Migrator().AddColumn(&model.AlertRule{}, column); err != nil {
			return fmt.Errorf("add alert_rules.%s: %w", column, err)
		}
	}
	return nil
}
