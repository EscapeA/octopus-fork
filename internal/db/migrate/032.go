package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 32,
		Up:      migrateNotificationI18nKeys,
	})
}

// 032: 为 notifications 增加 i18n 键化字段，使通知标题/正文可按用户 UI 语言渲染。
// 新增 title_key/title_args/content_key/content_args 四列。历史通知这些列为空，
// 前端回退到 title/content 原文，零破坏。幂等：HasColumn 守卫，已存在则跳过。
func migrateNotificationI18nKeys(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Notification{}) {
		return nil
	}
	columns := []string{"TitleKey", "TitleArgs", "ContentKey", "ContentArgs"}
	for _, col := range columns {
		if db.Migrator().HasColumn(&model.Notification{}, col) {
			continue
		}
		if err := db.Migrator().AddColumn(&model.Notification{}, col); err != nil {
			return fmt.Errorf("add notifications.%s: %w", col, err)
		}
	}
	return nil
}
