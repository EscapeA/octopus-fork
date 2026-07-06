package migrate

import (
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 25,
		Up:      migrateReportTables,
	})
}

// 025: 创建使用量报表相关表 (report_schedules, report_history)
func migrateReportTables(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	// AutoMigrate handles table creation idempotently
	if err := db.AutoMigrate(&model.ReportSchedule{}, &model.ReportHistory{}); err != nil {
		return err
	}

	return nil
}
