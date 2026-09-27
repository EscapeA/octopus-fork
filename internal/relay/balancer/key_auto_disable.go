package balancer

import (
	"strings"
	"sync"
	"time"
)

// 上游 402（Payment Required / 余额不足 / 欠费）自动禁用计数。
//
// 背景：402 原本没有独立分类，落进 ClassifyRelayError 的 default 分支；key 冷却
// （ratelimit_cooldown，默认 60s）到期后又会被选中，于是每个冷却周期都白打一次 402，
// 既浪费 Key 级重试额度又增加首字节延迟。余额不足是**账号级长期状态**（需充值才能恢复），
// 与 429 限流/5xx 抖动不同，不适合用秒级冷却。
//
// 这里按 (channelID, keyID) 维度统计**连续** 402 次数（余额是账号级、与模型无关，
// 故不带 model 维度），达到阈值后由 relay 层执行「自动禁用该 Key」并落库
// （见 internal/relay/insufficient_balance_guard.go）。计数为纯内存的短期状态：
// 禁用结果已经落库，重启丢计数不会影响已禁用的 Key。
//
// 清零时机：① Key 上出现一次成功请求；② 触发自动禁用时；③ Key 被删除/渠道被删除时。

// globalAutoDisableCounters 全局计数存储，key: "channelID:keyID:"。
var globalAutoDisableCounters sync.Map // key: string -> *autoDisableCounter

type autoDisableCounter struct {
	mu        sync.Mutex
	count     int
	lastTouch time.Time
}

// autoDisableCounterKey 构造计数 key。用 buildKey3 保持与熔断/冷却一致的三段格式，
// 便于复用 buildKeyNeedle/buildKeyPrefix 做按 keyID/channelID 的清理。
func autoDisableCounterKey(channelID, keyID int) string {
	return buildKey3(channelID, keyID, "")
}

// RecordAutoDisableFailure 记录一次上游 402，返回该 Key 的连续 402 次数。
func RecordAutoDisableFailure(channelID, keyID int) int {
	if channelID == 0 || keyID == 0 {
		return 0
	}
	key := autoDisableCounterKey(channelID, keyID)
	entry := getOrCreateAutoDisableCounter(key)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.count++
	entry.lastTouch = time.Now()
	return entry.count
}

// ResetAutoDisableCounter 清零某 (channelID, keyID) 的连续 402 计数。
// 在 Key 请求成功、或触发自动禁用后调用。
func ResetAutoDisableCounter(channelID, keyID int) {
	if channelID == 0 || keyID == 0 {
		return
	}
	globalAutoDisableCounters.Delete(autoDisableCounterKey(channelID, keyID))
}

// GetAutoDisableFailureCount 查询当前连续 402 次数（只读，用于日志/测试）。
func GetAutoDisableFailureCount(channelID, keyID int) int {
	v, ok := globalAutoDisableCounters.Load(autoDisableCounterKey(channelID, keyID))
	if !ok {
		return 0
	}
	entry, ok := v.(*autoDisableCounter)
	if !ok {
		return 0
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.count
}

func getOrCreateAutoDisableCounter(key string) *autoDisableCounter {
	if v, ok := globalAutoDisableCounters.Load(key); ok {
		if entry, ok := v.(*autoDisableCounter); ok {
			return entry
		}
	}
	entry := &autoDisableCounter{}
	actual, _ := globalAutoDisableCounters.LoadOrStore(key, entry)
	return actual.(*autoDisableCounter)
}

// RemoveChannelAutoDisableCounters 删除指定渠道的所有计数（渠道被删除时调用）。
func RemoveChannelAutoDisableCounters(channelID int) {
	prefix := buildKeyPrefix(channelID)
	globalAutoDisableCounters.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			globalAutoDisableCounters.Delete(key)
		}
		return true
	})
}

// RemoveKeyAutoDisableCounter 删除指定 Key 的所有计数（Key 被禁用/删除时调用）。
func RemoveKeyAutoDisableCounter(keyID int) {
	if keyID == 0 {
		return
	}
	needle := buildKeyNeedle(keyID)
	globalAutoDisableCounters.Range(func(key, _ any) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if strings.Contains(k, needle) {
			globalAutoDisableCounters.Delete(key)
		}
		return true
	})
}

// RemoveKeyRuntimeState 清理某个 Key 在运行时的全部隔离状态（冷却/熔断/可用度/402 计数）。
// 自动禁用（或手动启用）后调用，避免残留状态在 Key 恢复后继续排斥它。
// 注意：不清理 relay 层的失败提示缓存（跨包，由 relay 侧自己清理）。
func RemoveKeyRuntimeState(keyID int) {
	if keyID == 0 {
		return
	}
	RemoveKeyCooldowns(keyID)
	RemoveKeyAvailability(keyID)
	RemoveKeyEntries(keyID)
	RemoveKeyAutoDisableCounter(keyID)
}

// PurgeIdleAutoDisableCounters 回收空闲计数条目，防止 map 无界增长。
// 计数为 0 的条目、或超过 idleFor 未被触碰的条目会被删除。由周期任务调用。
func PurgeIdleAutoDisableCounters(idleFor time.Duration) int {
	if idleFor <= 0 {
		return 0
	}
	threshold := time.Now().Add(-idleFor)
	removed := 0
	globalAutoDisableCounters.Range(func(key, value any) bool {
		entry, ok := value.(*autoDisableCounter)
		if !ok {
			globalAutoDisableCounters.Delete(key)
			removed++
			return true
		}
		entry.mu.Lock()
		idle := entry.count == 0 || entry.lastTouch.Before(threshold)
		entry.mu.Unlock()
		if idle {
			globalAutoDisableCounters.Delete(key)
			removed++
		}
		return true
	})
	return removed
}
