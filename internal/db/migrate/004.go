package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 4,
		Up:      addAPIKeyQuotaColumns,
	})
}

func addAPIKeyQuotaColumns(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable("api_keys") {
		return nil
	}

	columns := []struct {
		name string
		typ  string
	}{
		{"rate_limit_rpm", "INT"},
		{"rate_limit_tpm", "INT"},
		{"per_model_quota_json", "TEXT"},
	}

	hasColumn := func(table, column string) (bool, error) {
		switch db.Dialector.Name() {
		case "sqlite":
			var name string
			if err := db.Raw("SELECT name FROM pragma_table_info(?) WHERE name = ? LIMIT 1", table, column).
				Scan(&name).Error; err != nil {
				return false, fmt.Errorf("failed to check sqlite column %s.%s: %w", table, column, err)
			}
			return name == column, nil
		default:
			return db.Migrator().HasColumn(table, column), nil
		}
	}

	for _, col := range columns {
		exists, err := hasColumn("api_keys", col.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := setColumnDefault(db, "api_keys", col.name, col.typ, "0"); err != nil {
			return fmt.Errorf("failed to add column api_keys.%s: %w", col.name, err)
		}
	}
	return nil
}

func setColumnDefault(db *gorm.DB, table, column, typ, defaultVal string) error {
	// 统一用 GORM Migrator().AddColumn，让 GORM 按方言自动转义表名/列名，
	// 避免 MySQL 保留字（如 groups）导致的 Error 1064。table 参数保留用于
	// 日志输出，实际 AddColumn 通过 model 类型推导表名。
	_ = table // 已通过 &model.APIKey{} 推导，保留参数签名兼容调用方
	_ = typ
	_ = defaultVal
	return db.Migrator().AddColumn(&model.APIKey{}, column)
}
