package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 59,
		Up:      migrateRemoveSemanticCacheSettings,
	})
}

// legacySemanticCacheSettingKeys 是「语义缓存」功能移除后遗留的设置键。
// 功能代码已全部删除（含默认值播种表），老实例的 settings 表里仍会留下这些行；
// 它们既不会被读取，也会在设置列表里暴露无对应页面的孤儿项，故一次性清理。
var legacySemanticCacheSettingKeys = []string{
	"semantic_cache_enabled",
	"semantic_cache_ttl",
	"semantic_cache_threshold",
	"semantic_cache_max_entries",
	"semantic_cache_embedding_base_url",
	"semantic_cache_embedding_api_key",
	"semantic_cache_embedding_model",
	"semantic_cache_embedding_timeout_seconds",
}

func migrateRemoveSemanticCacheSettings(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.Setting{}) {
		return nil
	}
	if err := db.Table("settings").Where("key IN ?", legacySemanticCacheSettingKeys).
		Delete(&model.Setting{}).Error; err != nil {
		return fmt.Errorf("cleanup legacy semantic cache settings: %w", err)
	}
	return nil
}
