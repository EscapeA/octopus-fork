package task

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/relay"
	"github.com/lingyuins/octopus/internal/utils/log"
)

const TaskKeyAutoDisableProbe = "key_auto_disable_probe"

// keyAutoDisableProbeLast 记录每个 Key 上次试活时刻（内存态，重启重置）。
// 用「上次试活时间 + 设置间隔」判定是否到点，而不是靠任务注册间隔，
// 这样设置页调整间隔后无需重启即生效。
var keyAutoDisableProbeLast sync.Map // keyID(int) -> time.Time

// ProbeAutoDisabledKeys 定时试活被自动禁用的 Key（上游 402 余额不足，见
// relay/insufficient_balance_guard.go）。试活成功即自动重新启用并通知；失败保持禁用。
//
// 判据用**真实最小模型调用**（与渠道/分组测试同源），而不是 GET /models：
// 余额不足时上游的 /models 往往仍返回 200（模型列表属元数据），只有真实推理请求
// 才会回 402，用 /models 探测会把欠费的 Key 误判为已恢复。
//
// 跳过两类渠道：
//   - 渠道本身已禁用（不产生流量，无需恢复 Key）；
//   - SkipModelTest 渠道（约定不发真实调用，避免低字节请求扣费/封禁，见 issue #98）；
//     这类渠道里被自动禁用的 Key 只能手动恢复。
func ProbeAutoDisabledKeys() {
	interval := relay.KeyAutoDisableProbeInterval()
	if interval <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	channels, err := channel.List(ctx)
	if err != nil {
		log.Warnf("key auto disable probe: failed to list channels: %v", err)
		return
	}

	now := time.Now()
	for i := range channels {
		ch := &channels[i]
		if !ch.Enabled || ch.SkipModelTest {
			continue
		}
		for _, key := range ch.Keys {
			if key.AutoDisabledAt == nil {
				continue
			}
			if strings.TrimSpace(key.ChannelKey) == "" || !key.Enabled {
				// 已被手动恢复（标记应已清空）或 Key 为空：只清理残留标记，不探测。
				continue
			}
			if last, ok := keyAutoDisableProbeLast.Load(key.ID); ok {
				if ts, ok2 := last.(time.Time); ok2 && now.Sub(ts) < interval {
					continue
				}
			}
			baseURL, suffixMode := firstProbeBaseURL(ch)
			if baseURL == "" {
				continue
			}
			keyAutoDisableProbeLast.Store(key.ID, time.Now())

			status, body, probeErr := helper.TestChannelKeyWithModel(ctx, ch, baseURL, suffixMode, key.ChannelKey)
			if probeErr != nil {
				log.Infof("key auto disable probe: channel %d(%s) key %d still unavailable (status=%d, %s)",
					ch.ID, ch.Name, key.ID, status, truncateProbeBody(body))
				continue
			}
			if relay.ReEnableAutoDisabledChannelKey(ch, key.ID) {
				log.Infof("key auto disable probe: channel %d(%s) key %d recovered, re-enabled", ch.ID, ch.Name, key.ID)
			}
		}
	}
}

// firstProbeBaseURL 返回渠道首个 base_url 与 suffix_mode（与渠道测试一致地取首个）。
func firstProbeBaseURL(ch *model.Channel) (string, string) {
	for _, item := range ch.BaseUrls {
		if url := strings.TrimSpace(item.URL); url != "" {
			return url, item.SuffixMode
		}
	}
	return "", ""
}

func truncateProbeBody(body string) string {
	body = strings.TrimSpace(body)
	if len(body) > 200 {
		return body[:200]
	}
	return body
}
