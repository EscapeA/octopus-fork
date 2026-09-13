package relay

import (
	"encoding/json"
	"sync"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// relayLogContentAPIKeyFilter 缓存「只记录内容大字段的 API Key」白名单的解析结果。
// 以 setting 代际为缓存键，避免每次请求写日志时重复解析 JSON。
var relayLogContentAPIKeyFilter struct {
	mu         sync.RWMutex
	generation uint64
	loaded     bool
	ids        map[int]struct{}
}

// relayLogContentEnabledForKey 判断指定 API Key 的请求/响应内容大字段是否应落库。
//
// 语义（总开关是主开关，白名单只做收窄）：
//   - relay_log_content_enabled 关闭 → 一律不记录内容（保持既有行为）；
//   - 开启 + 白名单为空 → 记录全部 Key（保持既有行为）；
//   - 开启 + 白名单非空 → 只记录白名单内的 Key（其余 Key 只落元数据）。
//
// 用途：开发/调试类高流量 Key（几次一条、单条上百 KB）不再把磁盘写满，同时
// 保留业务 Key 的完整请求/响应明细。
func relayLogContentEnabledForKey(apiKeyID int) bool {
	enabled, err := setting.GetBool(dbmodel.SettingKeyRelayLogContentEnabled)
	if err != nil || !enabled {
		return false
	}
	ids := relayLogContentAPIKeyIDs()
	if len(ids) == 0 {
		return true
	}
	_, ok := ids[apiKeyID]
	return ok
}

// relayLogContentAPIKeyIDs 返回白名单集合（空集合表示不限制 Key）。
// 解析失败或设置缺失时返回空集合，退化为「跟随总开关」，不会误禁日志。
func relayLogContentAPIKeyIDs() map[int]struct{} {
	generation := setting.Generation()

	relayLogContentAPIKeyFilter.mu.RLock()
	if relayLogContentAPIKeyFilter.loaded && relayLogContentAPIKeyFilter.generation == generation {
		ids := relayLogContentAPIKeyFilter.ids
		relayLogContentAPIKeyFilter.mu.RUnlock()
		return ids
	}
	relayLogContentAPIKeyFilter.mu.RUnlock()

	ids := map[int]struct{}{}
	if raw, err := setting.GetString(dbmodel.SettingKeyRelayLogContentAPIKeyIDs); err == nil && raw != "" {
		var parsed []int
		if json.Unmarshal([]byte(raw), &parsed) == nil {
			for _, id := range parsed {
				if id >= 0 {
					ids[id] = struct{}{}
				}
			}
		}
	}

	relayLogContentAPIKeyFilter.mu.Lock()
	relayLogContentAPIKeyFilter.generation = generation
	relayLogContentAPIKeyFilter.loaded = true
	relayLogContentAPIKeyFilter.ids = ids
	relayLogContentAPIKeyFilter.mu.Unlock()

	return ids
}
