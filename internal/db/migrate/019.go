package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 19,
		Up:      ensureRelayLogsCompositeIndexes,
	})
}

// 019: 为 relay_logs 增加 (channel_id, time) 与 (request_api_key_id, time) 复合索引。
//
// 日志列表查询最常用的过滤组合是「渠道 + 时间范围」和「API Key + 时间范围」
// （RelayLogList 的 ChannelID/APIKeyID + StartTime/EndTime）。此前只有 time 单列
// 索引，这些查询需先按 time 缩小范围再在结果集里过滤 channel_id/api_key_id；大表
// 上当时间范围较宽（如 7d/30d）时，单列索引命中的行数远超最终结果，产生大量
// 回表与丢弃。复合索引让数据库直接定位到「指定渠道/Key 在指定时间段」的行，
// 显著减少 IO（见日志性能审计：RelayLogList 在百万级表上慢查询根因之一）。
func ensureRelayLogsCompositeIndexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable("relay_logs") {
		return nil
	}

	// (channel_id, time)
	if !db.Migrator().HasIndex(&model.RelayLog{}, "idx_relay_logs_channel_time") {
		if err := db.Migrator().CreateIndex(&model.RelayLog{}, "idx_relay_logs_channel_time"); err != nil {
			return fmt.Errorf("failed to create relay_logs(channel_id,time) index: %w", err)
		}
	}

	// (request_api_key_id, time)
	if !db.Migrator().HasIndex(&model.RelayLog{}, "idx_relay_logs_apikey_time") {
		if err := db.Migrator().CreateIndex(&model.RelayLog{}, "idx_relay_logs_apikey_time"); err != nil {
			return fmt.Errorf("failed to create relay_logs(request_api_key_id,time) index: %w", err)
		}
	}

	return nil
}
