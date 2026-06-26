package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 10,
		Up:      migrateStatsLatencyColumns,
	})
}

func migrateStatsLatencyColumns(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable("stats_hourlies") {
		return nil
	}

	columns := []string{
		"LatencyP50", "LatencyP95", "LatencyP99",
		"FtutAvg", "FtutP50", "FtutP95", "FtutP99",
		"HistogramLt100", "Histogram100to500", "Histogram500to1k",
		"Histogram1kto5k", "HistogramGt5k",
	}

	for _, field := range columns {
		if !db.Migrator().HasColumn(&model.StatsHourly{}, field) {
			// 用 GORM Migrator().AddColumn 而非裸 SQL，按方言自动转义表名/列名。
			if err := db.Migrator().AddColumn(&model.StatsHourly{}, field); err != nil {
				return fmt.Errorf("add column %s: %w", field, err)
			}
		}
	}

	return nil
}
