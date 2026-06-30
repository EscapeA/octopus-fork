package balancer

import (
	"net/http"
	"testing"
	"time"
)

func resetKeyAvailability() {
	globalKeyAvailability.Range(func(key, _ any) bool {
		globalKeyAvailability.Delete(key)
		return true
	})
	availabilityLocks.Range(func(key, _ any) bool {
		availabilityLocks.Delete(key)
		return true
	})
}

func TestKeyAvailabilityInitialScoreIsMax(t *testing.T) {
	resetKeyAvailability()
	score := GetKeyAvailabilityScore(1, 1, "gpt-4o")
	if score != keyAvailabilityMaxScore {
		t.Fatalf("initial score = %v, want %v", score, keyAvailabilityMaxScore)
	}
}

func TestKeyAvailabilityEmptyModelReturnsMax(t *testing.T) {
	resetKeyAvailability()
	// 空 model（后台任务场景）不应参与评分，返回满分。
	score := GetKeyAvailabilityScore(1, 1, "")
	if score != keyAvailabilityMaxScore {
		t.Fatalf("empty model score = %v, want %v", score, keyAvailabilityMaxScore)
	}
}

func TestKeyAvailabilityAuthFailureDropsToZero(t *testing.T) {
	resetKeyAvailability()
	RecordKeyAvailability(1, 1, "gpt-4o", http.StatusUnauthorized, false)
	score := GetKeyAvailabilityScore(1, 1, "gpt-4o")
	if score != 0 {
		t.Fatalf("after 401, score = %v, want 0", score)
	}
}

func TestKeyAvailabilityRateLimitPartialDecay(t *testing.T) {
	resetKeyAvailability()
	RecordKeyAvailability(1, 1, "gpt-4o", http.StatusTooManyRequests, false)
	score := GetKeyAvailabilityScore(1, 1, "gpt-4o")
	want := keyAvailabilityMaxScore - keyAvailabilityPenaltyRateLimit
	if score != want {
		t.Fatalf("after 429, score = %v, want %v", score, want)
	}
}

func TestKeyAvailabilitySuccessRecovery(t *testing.T) {
	resetKeyAvailability()
	// 先衰减
	RecordKeyAvailability(1, 1, "gpt-4o", http.StatusTooManyRequests, false)
	// 成功加分
	RecordKeyAvailability(1, 1, "gpt-4o", 0, true)
	score := GetKeyAvailabilityScore(1, 1, "gpt-4o")
	want := keyAvailabilityMaxScore - keyAvailabilityPenaltyRateLimit + keyAvailabilitySuccessGain
	if score != want {
		t.Fatalf("after success, score = %v, want %v", score, want)
	}
}

func TestKeyAvailabilitySuccessCappedAtMax(t *testing.T) {
	resetKeyAvailability()
	// 多次成功不应超过满分
	for i := 0; i < 10; i++ {
		RecordKeyAvailability(1, 1, "gpt-4o", 0, true)
	}
	score := GetKeyAvailabilityScore(1, 1, "gpt-4o")
	if score != keyAvailabilityMaxScore {
		t.Fatalf("capped score = %v, want %v", score, keyAvailabilityMaxScore)
	}
}

func TestKeyAvailabilityTimeRecovery(t *testing.T) {
	resetKeyAvailability()
	// 先衰减
	RecordKeyAvailability(1, 1, "gpt-4o", http.StatusTooManyRequests, false)

	// 模拟时间推进：手动修改 lastDecayAt 为 2 分钟前
	key := availabilityKey(1, 1, "gpt-4o")
	v, ok := globalKeyAvailability.Load(key)
	if !ok {
		t.Fatal("entry not found")
	}
	entry := v.(*keyAvailabilityEntry)
	mu := getAvailabilityLock(key)
	mu.Lock()
	entry.lastDecayAt = time.Now().Add(-2 * time.Minute)
	mu.Unlock()

	score := GetKeyAvailabilityScore(1, 1, "gpt-4o")
	want := keyAvailabilityMaxScore - keyAvailabilityPenaltyRateLimit + 2*keyAvailabilityRecoveryRate
	if score != want {
		t.Fatalf("after 2min recovery, score = %v, want %v", score, want)
	}
}

func TestKeyAvailabilityScoreIsPerModel(t *testing.T) {
	resetKeyAvailability()
	// 同一 key 对 model A 衰减，不影响 model B
	RecordKeyAvailability(1, 1, "gpt-4o", http.StatusTooManyRequests, false)
	scoreA := GetKeyAvailabilityScore(1, 1, "gpt-4o")
	scoreB := GetKeyAvailabilityScore(1, 1, "claude-3-5-sonnet")
	if scoreA == keyAvailabilityMaxScore {
		t.Fatal("model A should be penalized")
	}
	if scoreB != keyAvailabilityMaxScore {
		t.Fatalf("model B should be at max, got %v", scoreB)
	}
}

func TestRemoveChannelKeyAvailability(t *testing.T) {
	resetKeyAvailability()
	RecordKeyAvailability(1, 1, "model-a", http.StatusTooManyRequests, false)
	RecordKeyAvailability(1, 2, "model-b", http.StatusTooManyRequests, false)
	RecordKeyAvailability(2, 3, "model-c", http.StatusTooManyRequests, false)

	RemoveChannelKeyAvailability(1)
	if score := GetKeyAvailabilityScore(1, 1, "model-a"); score != keyAvailabilityMaxScore {
		t.Fatalf("channel 1 key 1 should be reset to max, got %v", score)
	}
	if score := GetKeyAvailabilityScore(2, 3, "model-c"); score == keyAvailabilityMaxScore {
		t.Fatal("channel 2 should be unaffected")
	}
}

func TestPurgeStaleKeyAvailability(t *testing.T) {
	resetKeyAvailability()
	RecordKeyAvailability(1, 1, "gpt-4o", http.StatusTooManyRequests, false)

	// 模拟条目过期。回收锚点是 lastActivity（仅写路径更新），非 lastDecayAt
	// （后者在读路径 applyRecovery 中也会前移，见 issue #124）。
	key := availabilityKey(1, 1, "gpt-4o")
	v, _ := globalKeyAvailability.Load(key)
	entry := v.(*keyAvailabilityEntry)
	mu := getAvailabilityLock(key)
	mu.Lock()
	entry.lastActivity = time.Now().Add(-2 * time.Hour)
	mu.Unlock()

	removed := PurgeStaleKeyAvailability(time.Hour)
	if removed < 1 {
		t.Fatalf("should purge stale entry, removed = %d", removed)
	}
	// 清理后回到满分
	score := GetKeyAvailabilityScore(1, 1, "gpt-4o")
	if score != keyAvailabilityMaxScore {
		t.Fatalf("after purge, score = %v, want max", score)
	}
}

// TestPurgeStaleKeyAvailability_ReadPathDoesNotRefreshAnchor 回归保护（issue #124）：
// 读路径 GetKeyAvailabilityScore 不应刷新回收锚点 lastActivity。把 lastActivity 倒拨
// 到过期阈值之前后反复读取，条目仍应被 PurgeStaleKeyAvailability 回收——否则频繁被
// 查询（但不再被写）的垃圾 key 永不满足空闲阈值，导致 map 无界增长。
func TestPurgeStaleKeyAvailability_ReadPathDoesNotRefreshAnchor(t *testing.T) {
	resetKeyAvailability()
	RecordKeyAvailability(1, 1, "gpt-4o", http.StatusTooManyRequests, false)

	key := availabilityKey(1, 1, "gpt-4o")
	v, _ := globalKeyAvailability.Load(key)
	entry := v.(*keyAvailabilityEntry)
	mu := getAvailabilityLock(key)
	mu.Lock()
	// 模拟条目已超过空闲阈值：lastActivity 倒拨到 2 小时前。
	entry.lastActivity = time.Now().Add(-2 * time.Hour)
	mu.Unlock()

	// 反复读取（读路径会前移 lastDecayAt，但不应触碰 lastActivity）。
	for i := 0; i < 3; i++ {
		_ = GetKeyAvailabilityScore(1, 1, "gpt-4o")
	}

	removed := PurgeStaleKeyAvailability(time.Hour)
	if removed < 1 {
		t.Fatalf("read path should not refresh purge anchor; expected purge, removed = %d", removed)
	}
}
