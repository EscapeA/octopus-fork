package relay

import (
	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	"github.com/lingyuins/octopus/internal/op/apikey"
	"github.com/lingyuins/octopus/internal/relay/balancer"
)

func init() {
	// 注入按模型粒度的 Key 冷却查询函数，打破 model → balancer 的循环依赖。
	// model 包不能直接 import balancer，故由 relay 层在初始化时注入实现。
	dbmodel.KeyCooldownFunc = balancer.IsKeyOnCooldown

	// 注册渠道删除时的清理钩子：清除熔断器、Auto 策略统计和 Key 冷却中的残留条目，
	// 防止 globalBreaker / globalAutoStats / globalKeyCooldown 无限增长。
	op.OnChannelDeletedHooks = append(op.OnChannelDeletedHooks, func(channelID int) {
		balancer.RemoveChannelEntries(channelID)
		balancer.RemoveChannelStats(channelID)
		balancer.RemoveChannelKeyCooldowns(channelID)
	})

	// 注册 API Key 删除时的清理钩子：清除粘性会话条目，
	// 防止 globalSession 无限增长。
	apikey.DeleteSessionFunc = func(id int) {
		balancer.RemoveAPIKeySticky(id)
	}
}
