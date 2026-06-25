package balancer

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// 可用度优先的 Key 选择策略（与 key_cooldown.go / circuit.go 同维度）。
//
// 背景：默认 cost 策略按 TotalCost 最低选 key，无法体现「某个 key 最近频繁出错、
// 应优先用其他健康 key」的语义。availability 策略维护每个 (channelID, keyID,
// modelName) 的可用度分数，出错衰减、成功/时间恢复，选分最高的 key；同分按 Keys
// 数组顺序取第一个（初始全满分 → 用第一个 key）。
//
// 与冷却/熔断/失败提示的关系：可用度是「软优先级」（决定从多个可用 key 中选哪个），
// 冷却/熔断/失败提示是「硬隔离」（选中后仍会跳过）。分数 ≤ 0 时可用度策略视为不可用，
// 但不写入冷却/熔断——硬隔离由各自的触发条件独立管理。

const (
	keyAvailabilityMaxScore     = 100.0 // 满分
	keyAvailabilityMinScore     = 0.0   // 下限，≤0 视为不可用
	keyAvailabilitySuccessGain  = 5.0   // 成功加分
	keyAvailabilityRecoveryRate = 1.0   // 每分钟恢复分数（懒计算）
	keyAvailabilityRecoveryStep = time.Minute

	// 按错误类型加权衰减（与 ClassifyRelayError 分类对应）
	keyAvailabilityPenaltyAuth      = 100.0 // 401/403 鉴权失败，直接降到 0
	keyAvailabilityPenaltyRateLimit = 15.0  // 429 限流
	keyAvailabilityPenaltyServer    = 10.0  // 5xx 服务端错误
	keyAvailabilityPenaltyTimeout   = 10.0  // 超时
	keyAvailabilityPenaltyNetwork   = 5.0   // 网络错误
	keyAvailabilityPenaltyGeneric   = 10.0  // 404/transformer 等其他可重试错误
)

// keyAvailabilityEntry 记录某个 (channelID, keyID, modelName) 的可用度分数。
type keyAvailabilityEntry struct {
	score       float64   // 当前分数（已含懒恢复补偿）
	lastDecayAt time.Time // 上次分数变化时刻，用于懒恢复计算
}

// globalKeyAvailability 全局可用度分数存储，key: "channelID:keyID:modelName"。
// 与 globalKeyCooldown / globalBreaker 同维度，便于复用清理。
var globalKeyAvailability sync.Map // key: string -> *keyAvailabilityEntry

// availabilityKey 构造存储 key，与 cooldownKey 格式一致。
func availabilityKey(channelID, keyID int, modelName string) string {
	return fmt.Sprintf("%d:%d:%s", channelID, keyID, strings.TrimSpace(modelName))
}

// penaltyForStatusCode 根据状态码返回衰减分数。
func penaltyForStatusCode(statusCode int) float64 {
	switch {
	case statusCode == 401 || statusCode == 403:
		return keyAvailabilityPenaltyAuth
	case statusCode == 429:
		return keyAvailabilityPenaltyRateLimit
	case statusCode >= 500:
		return keyAvailabilityPenaltyServer
	case statusCode == 408:
		return keyAvailabilityPenaltyTimeout
	case statusCode >= 400:
		return keyAvailabilityPenaltyGeneric
	default:
		return 0
	}
}

// getOrCreateAvailabilityEntry 读取或创建条目（初始满分）。
func getOrCreateAvailabilityEntry(key string) *keyAvailabilityEntry {
	now := time.Now()
	v, ok := globalKeyAvailability.Load(key)
	if ok {
		if entry, ok := v.(*keyAvailabilityEntry); ok {
			return entry
		}
	}
	entry := &keyAvailabilityEntry{score: keyAvailabilityMaxScore, lastDecayAt: now}
	actual, _ := globalKeyAvailability.LoadOrStore(key, entry)
	if e, ok := actual.(*keyAvailabilityEntry); ok {
		return e
	}
	return entry
}

// applyRecovery 懒恢复：根据距 lastDecayAt 的时间差，按每分钟 +1 补偿分数。
// 返回恢复后的分数（上限 100），并更新 lastDecayAt。
func applyRecovery(entry *keyAvailabilityEntry, now time.Time) float64 {
	if now.Before(entry.lastDecayAt) {
		return entry.score
	}
	elapsed := now.Sub(entry.lastDecayAt)
	if elapsed < keyAvailabilityRecoveryStep {
		return entry.score
	}
	minutes := int(elapsed / keyAvailabilityRecoveryStep)
	recovery := float64(minutes) * keyAvailabilityRecoveryRate
	entry.score = minFloat(entry.score+recovery, keyAvailabilityMaxScore)
	entry.lastDecayAt = entry.lastDecayAt.Add(time.Duration(minutes) * keyAvailabilityRecoveryStep)
	return entry.score
}

// GetKeyAvailabilityScore 查询某个 (channelID, keyID, modelName) 的可用度分数。
// 懒恢复：读取时基于 lastDecayAt 补偿时间恢复。modelName 为空时返回满分（后台任务
// 不携带模型语义，不参与可用度评分）。
func GetKeyAvailabilityScore(channelID, keyID int, modelName string) float64 {
	if strings.TrimSpace(modelName) == "" || keyID == 0 {
		return keyAvailabilityMaxScore
	}
	key := availabilityKey(channelID, keyID, modelName)
	v, ok := globalKeyAvailability.Load(key)
	if !ok {
		return keyAvailabilityMaxScore
	}
	entry, ok := v.(*keyAvailabilityEntry)
	if !ok {
		globalKeyAvailability.Delete(key)
		return keyAvailabilityMaxScore
	}
	now := time.Now()
	// 懒恢复需要写 lastDecayAt，加锁保证并发安全。
	mu := getAvailabilityLock(key)
	mu.Lock()
	defer mu.Unlock()
	return applyRecovery(entry, now)
}

// RecordKeyAvailability 记录某 (channelID, keyID, modelName) 的可用度变化。
// success=true 时加分（上限 100），success=false 时按 statusCode 衰减。
// modelName 为空或 keyID=0 时跳过（后台任务不评分）。
func RecordKeyAvailability(channelID, keyID int, modelName string, statusCode int, success bool) {
	if strings.TrimSpace(modelName) == "" || keyID == 0 {
		return
	}
	key := availabilityKey(channelID, keyID, modelName)
	mu := getAvailabilityLock(key)
	mu.Lock()
	defer mu.Unlock()

	entry := getOrCreateAvailabilityEntry(key)
	now := time.Now()
	// 先补偿自上次变化以来的时间恢复，再叠加本次事件。
	applyRecovery(entry, now)
	if success {
		entry.score = minFloat(entry.score+keyAvailabilitySuccessGain, keyAvailabilityMaxScore)
	} else {
		penalty := penaltyForStatusCode(statusCode)
		if penalty > 0 {
			entry.score = maxFloat(entry.score-penalty, keyAvailabilityMinScore)
		}
	}
	entry.lastDecayAt = now
}

// PurgeStaleKeyAvailability 清理长时间未活动的可用度条目，防止 map 无界增长。
// maxAge 为最大空闲时长，超过则删除。由 relay log flush 定时任务周期性调用。
// 注意：清理后该 key 下次查询会回到满分（冷启动语义），符合可用度的短期软信号定位。
func PurgeStaleKeyAvailability(maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}
	threshold := time.Now().Add(-maxAge)
	removed := 0
	globalKeyAvailability.Range(func(k, v any) bool {
		entry, ok := v.(*keyAvailabilityEntry)
		if !ok {
			globalKeyAvailability.Delete(k)
			removed++
			return true
		}
		if entry.lastDecayAt.Before(threshold) {
			globalKeyAvailability.Delete(k)
			removed++
			releaseAvailabilityLock(k.(string))
		}
		return true
	})
	return removed
}

// RemoveChannelKeyAvailability 删除指定渠道的所有可用度条目。
// 在渠道被删除时调用，注册于 OnChannelDeletedHooks。
func RemoveChannelKeyAvailability(channelID int) {
	prefix := fmt.Sprintf("%d:", channelID)
	globalKeyAvailability.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			globalKeyAvailability.Delete(key)
			releaseAvailabilityLock(k)
		}
		return true
	})
}

// RemoveKeyAvailability 删除指定 key 的所有可用度条目（跨模型）。
func RemoveKeyAvailability(keyID int) {
	if keyID == 0 {
		return
	}
	needle := fmt.Sprintf(":%d:", keyID)
	globalKeyAvailability.Range(func(key, _ any) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if strings.Contains(k, needle) {
			globalKeyAvailability.Delete(key)
			releaseAvailabilityLock(k)
		}
		return true
	})
}

// --- per-key mutex pool for concurrent score read/write safety ---

var availabilityLocks sync.Map // key: string -> *sync.Mutex

func getAvailabilityLock(key string) *sync.Mutex {
	v, ok := availabilityLocks.Load(key)
	if ok {
		return v.(*sync.Mutex)
	}
	mu := &sync.Mutex{}
	actual, _ := availabilityLocks.LoadOrStore(key, mu)
	return actual.(*sync.Mutex)
}

func releaseAvailabilityLock(key string) {
	availabilityLocks.Delete(key)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
