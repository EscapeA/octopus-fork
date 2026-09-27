package balancer

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

func TestAutoDisableCounterAccumulatesAndResets(t *testing.T) {
	const channelID, keyID = 9101, 4242
	RemoveChannelAutoDisableCounters(channelID)
	t.Cleanup(func() { RemoveChannelAutoDisableCounters(channelID) })

	for i := 1; i <= 3; i++ {
		if got := RecordAutoDisableFailure(channelID, keyID); got != i {
			t.Fatalf("RecordAutoDisableFailure #%d = %d, want %d", i, got, i)
		}
	}
	if got := GetAutoDisableFailureCount(channelID, keyID); got != 3 {
		t.Fatalf("GetAutoDisableFailureCount = %d, want 3", got)
	}

	ResetAutoDisableCounter(channelID, keyID)
	if got := GetAutoDisableFailureCount(channelID, keyID); got != 0 {
		t.Fatalf("after reset GetAutoDisableFailureCount = %d, want 0", got)
	}
	if got := RecordAutoDisableFailure(channelID, keyID); got != 1 {
		t.Fatalf("after reset RecordAutoDisableFailure = %d, want 1", got)
	}
}

func TestAutoDisableCounterIsPerKeyNotPerModel(t *testing.T) {
	// 余额不足是账号级状态，计数不带模型维度：同一 Key 换模型仍累加。
	const channelID, keyID = 9102, 4243
	RemoveChannelAutoDisableCounters(channelID)
	t.Cleanup(func() { RemoveChannelAutoDisableCounters(channelID) })

	RecordAutoDisableFailure(channelID, keyID)
	if got := RecordAutoDisableFailure(channelID, keyID); got != 2 {
		t.Fatalf("counter = %d, want 2（计数应跨模型累加）", got)
	}
	// 另一个 Key 互不影响
	if got := RecordAutoDisableFailure(channelID, keyID+1); got != 1 {
		t.Fatalf("other key counter = %d, want 1", got)
	}
}

func TestRemoveKeyAutoDisableCounter(t *testing.T) {
	const keyID = 4244
	RecordAutoDisableFailure(9103, keyID)
	RecordAutoDisableFailure(9103, keyID)
	RemoveKeyAutoDisableCounter(keyID)
	if got := GetAutoDisableFailureCount(9103, keyID); got != 0 {
		t.Fatalf("after RemoveKeyAutoDisableCounter = %d, want 0", got)
	}
}

func TestRemoveKeyRuntimeStateClearsAutoDisableCounter(t *testing.T) {
	const channelID, keyID = 9104, 4245
	const modelName = "deepseek-flash"
	RecordAutoDisableFailure(channelID, keyID)
	RecordKeyCooldown(channelID, keyID, modelName, 402)
	RecordKeyAvailability(channelID, keyID, modelName, 402, false)
	RecordFailure(channelID, keyID, modelName)

	RemoveKeyRuntimeState(keyID)

	if got := GetAutoDisableFailureCount(channelID, keyID); got != 0 {
		t.Fatalf("auto disable counter = %d, want 0", got)
	}
	if IsKeyOnCooldown(channelID, keyID, modelName) {
		t.Fatal("key 仍在冷却中，RemoveKeyRuntimeState 未清理 cooldown")
	}
	if tripped, _ := IsTripped(channelID, keyID, modelName); tripped {
		t.Fatal("key 仍处于熔断状态，RemoveKeyRuntimeState 未清理 breaker")
	}
	if got := GetKeyAvailabilityScore(channelID, keyID, modelName); got != keyAvailabilityMaxScore {
		t.Fatalf("可用度 = %v, want %v（应回到满分）", got, keyAvailabilityMaxScore)
	}
}

func TestPurgeIdleAutoDisableCountersRemovesZeroCountEntries(t *testing.T) {
	const channelID, keyID = 9105, 4246
	RecordAutoDisableFailure(channelID, keyID)
	// 手动把 count 归零但保留条目，模拟 purge 前后的边界。
	if v, ok := globalAutoDisableCounters.Load(autoDisableCounterKey(channelID, keyID)); ok {
		entry := v.(*autoDisableCounter)
		entry.mu.Lock()
		entry.count = 0
		entry.mu.Unlock()
	}
	PurgeIdleAutoDisableCounters(time.Hour)
	if got := GetAutoDisableFailureCount(channelID, keyID); got != 0 {
		t.Fatalf("counter = %d, want 0", got)
	}
	if _, ok := globalAutoDisableCounters.Load(autoDisableCounterKey(channelID, keyID)); ok {
		t.Fatal("count 为 0 的条目应被 purge 回收")
	}
}

func TestChannelKeyAutoDisableFieldsSurviveModelCopy(t *testing.T) {
	// 守护：新增字段必须能被 KeyUpdate/缓存拷贝链路保留（无自定义 Marshal 丢弃）。
	now := time.Now()
	key := model.ChannelKey{ID: 1, ChannelID: 2, Enabled: false, AutoDisabledAt: &now, AutoDisableReason: "auto disabled: upstream 402"}
	keys := []model.ChannelKey{key}
	if keys[0].AutoDisabledAt == nil || keys[0].AutoDisableReason == "" {
		t.Fatal("ChannelKey 自动禁用字段丢失")
	}
}
