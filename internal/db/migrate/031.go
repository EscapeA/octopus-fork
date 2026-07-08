package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 31,
		Up:      migrateSiteAccountSkipModelSync,
	})
}

// 031: 为站点账号增加「跳过模型同步」开关（issue #130）。
// SkipModelSync=true 时站点账号同步只拉取令牌/分组/余额，跳过模型列表拉取，
// 保留历史模型不变。幂等：HasTable/HasColumn 守卫，已存在则跳过。
func migrateSiteAccountSkipModelSync(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.SiteAccount{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.SiteAccount{}, "SkipModelSync") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.SiteAccount{}, "SkipModelSync"); err != nil {
		return fmt.Errorf("add site_accounts.skip_model_sync: %w", err)
	}
	return nil
}
