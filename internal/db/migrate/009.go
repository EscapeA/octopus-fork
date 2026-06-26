package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 9,
		Up: func(db *gorm.DB) error {
			hasColumn := func(table, column string) (bool, error) {
				return db.Migrator().HasColumn(table, column), nil
			}

			exists, err := hasColumn("groups", "endpoint_provider")
			if err != nil {
				return fmt.Errorf("failed to inspect groups.endpoint_provider: %w", err)
			}
			if exists {
				return nil
			}

			// 用 GORM Migrator().AddColumn 而非裸 SQL，让 GORM 按方言自动转义
			// 表名（groups 是 MySQL 保留字，裸 SQL "ALTER TABLE groups" 会报 1064）。
			if err := db.Migrator().AddColumn(&model.Group{}, "EndpointProvider"); err != nil {
				return fmt.Errorf("failed to add groups.endpoint_provider: %w", err)
			}
			return nil
		},
	})
}
