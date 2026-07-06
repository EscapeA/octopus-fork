package migrate

import (
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 27,
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(
				&model.StatsDailyChannel{},
				&model.StatsDailyModel{},
				&model.StatsDailyAPIKey{},
				&model.StatsDailyChannelModel{},
			)
		},
	})
}
