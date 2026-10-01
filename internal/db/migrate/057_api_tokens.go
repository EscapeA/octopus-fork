package migrate

import (
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 57,
		Up:      migrateAPITokens,
	})
}

// legacyAgentTokenSettingKeys 是 056 之前的「Agent 令牌」设置键（A 阶段实现）。
// 057 起机器令牌改由 api_tokens 表承载（多枚、可命名、可设有效期、可吊销），
// 这里把仍在使用的那一枚令牌平滑搬进表里，旧令牌继续有效，然后把设置行清掉。
//
// 用字面量而非 model.SettingKey 常量：常量已随本迁移删除，历史迁移不应依赖现役代码。
var legacyAgentTokenSettingKeys = []string{
	"agent_api_token_hash",
	"agent_api_token_username",
	"agent_api_token_enabled",
	"agent_api_token_prefix",
	"agent_api_token_created_at",
}

const legacyAgentTokenName = "legacy-agent-token"

// migrateAPITokens 把 A 阶段存在设置表里的单枚 Agent 令牌搬进 api_tokens。
//
// 幂等：表不存在或设置行已清掉时直接返回；摘要已存在于表中时跳过插入。
func migrateAPITokens(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.APIToken{}) {
		return nil
	}

	type settingRow struct {
		Key   string
		Value string
	}
	var rows []settingRow
	if db.Table("settings").Where("key IN ?", legacyAgentTokenSettingKeys).Find(&rows).Error != nil {
		return nil
	}
	if len(rows) == 0 {
		return nil
	}

	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Key] = row.Value
	}

	hash := values["agent_api_token_hash"]
	if hash != "" {
		if err := backfillLegacyAgentToken(db, hash, values); err != nil {
			return err
		}
	}

	// 清理遗留设置行（令牌已进表或本就是空配置）。
	if err := db.Table("settings").Where("key IN ?", legacyAgentTokenSettingKeys).
		Delete(&model.Setting{}).Error; err != nil {
		return fmt.Errorf("delete legacy agent token settings: %w", err)
	}
	return nil
}

func backfillLegacyAgentToken(db *gorm.DB, hash string, values map[string]string) error {
	var existing int64
	if err := db.Model(&model.APIToken{}).Where("token_hash = ?", hash).Count(&existing).Error; err != nil {
		return fmt.Errorf("inspect api_tokens: %w", err)
	}
	if existing > 0 {
		return nil
	}

	var bound model.User
	if err := db.Where("username = ?", values["agent_api_token_username"]).First(&bound).Error; err != nil {
		// 绑定用户已不存在：搬运没有意义（鉴权时也一定会被拒），直接放弃。
		return nil
	}

	prefix := values["agent_api_token_prefix"]
	if prefix == "" && len(hash) >= 8 {
		prefix = "ok-agent-"
	}
	createdAt := time.Now()
	if parsed, err := time.Parse(time.RFC3339, values["agent_api_token_created_at"]); err == nil {
		createdAt = parsed
	}

	row := model.APIToken{
		Name:      legacyAgentTokenName,
		TokenHash: hash,
		Prefix:    prefix,
		UserID:    bound.ID,
		Username:  bound.Username,
		CreatedAt: createdAt,
	}
	if err := db.Create(&row).Error; err != nil {
		return fmt.Errorf("backfill legacy agent token: %w", err)
	}
	return nil
}
