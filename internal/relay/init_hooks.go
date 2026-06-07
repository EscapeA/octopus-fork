package relay

import (
	"github.com/lingyuins/octopus/internal/op"
	"github.com/lingyuins/octopus/internal/relay/balancer"
)

func init() {
	// 注册渠道删除时的清理钩子：清除熔断器和 Auto 策略统计中的残留条目，
	// 防止 globalBreaker / globalAutoStats 无限增长。
	op.OnChannelDeletedHooks = append(op.OnChannelDeletedHooks, func(channelID int) {
		balancer.RemoveChannelEntries(channelID)
		balancer.RemoveChannelStats(channelID)
	})
}
