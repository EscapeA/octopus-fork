package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 47,
		Up:      migrateModelNormalizeMarketDedupeDefault,
	})
}

// 047: 模型广场归一化去重默认开启。
// 旧默认 "false"（`model_normalize_market_dedupe_default`）导致模型广场不按
// 归一化名合并变体（如 dmxapi-kimi-k2.5 与 kimi-k2.5 各自成行）。将存量默认值
// 翻转为 "true"；条件限定 value='false'，重复执行安全（幂等），不会覆盖用户
// 后续主动改回的值。
func migrateModelNormalizeMarketDedupeDefault(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable("settings") {
		return nil
	}
	return db.Table("settings").
		Where("`key` = ? AND `value` = ?", "model_normalize_market_dedupe_default", "false").
		Update("value", "true").Error
}
