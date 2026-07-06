package migrate

import (
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 26,
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(
				&model.Notification{},
				&model.NotificationDelivery{},
				&model.NotificationPreference{},
				&model.NotificationPolicy{},
			)
		},
	})
}
