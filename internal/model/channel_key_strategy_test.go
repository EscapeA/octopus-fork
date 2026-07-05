package model

import (
	"testing"
)

// stubKeyAvailabilityScore 用于测试 selectKeyByAvailability，返回预设分数。
var stubKeyAvailabilityScores map[string]float64

func stubKeyAvailabilityScoreFunc(channelID, keyID int, modelName string) float64 {
	k := availabilityStubKey(channelID, keyID, modelName)
	if s, ok := stubKeyAvailabilityScores[k]; ok {
		return s
	}
	return keyAvailabilityMaxScore
}

func availabilityStubKey(channelID, keyID int, modelName string) string {
	return formatAvailabilityStubKey(channelID, keyID, modelName)
}

// formatAvailabilityStubKey 与 balancer.availabilityKey 格式一致，避免跨包依赖。
func formatAvailabilityStubKey(channelID, keyID int, modelName string) string {
	return intToStr(channelID) + ":" + intToStr(keyID) + ":" + trimSpace(modelName)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

const keyAvailabilityMaxScore = 100.0

func setupAvailabilityTest() func() {
	oldFunc := KeyAvailabilityScoreFunc
	oldStrategy := GlobalKeySelectionStrategyFunc
	KeyAvailabilityScoreFunc = stubKeyAvailabilityScoreFunc
	GlobalKeySelectionStrategyFunc = func() string { return "availability" }
	stubKeyAvailabilityScores = make(map[string]float64)
	return func() {
		KeyAvailabilityScoreFunc = oldFunc
		GlobalKeySelectionStrategyFunc = oldStrategy
		stubKeyAvailabilityScores = nil
	}
}

func makeChannelWithKeys(keys ...ChannelKey) *Channel {
	return &Channel{
		ID:   1,
		Name: "test-channel",
		Keys: keys,
	}
}

func enabledKey(id int, cost float64) ChannelKey {
	return ChannelKey{
		ID:         id,
		ChannelID:  1,
		Enabled:    true,
		ChannelKey: "sk-test-" + intToStr(id),
		TotalCost:  cost,
	}
}

func TestSelectKeyByAvailabilityPicksHighestScore(t *testing.T) {
	cleanup := setupAvailabilityTest()
	defer cleanup()

	ch := makeChannelWithKeys(
		enabledKey(1, 1.0),
		enabledKey(2, 2.0),
		enabledKey(3, 3.0),
	)
	// key 1 出错降到 50，key 2 满分，key 3 降到 30
	stubKeyAvailabilityScores[availabilityStubKey(1, 1, "gpt-4o")] = 50
	stubKeyAvailabilityScores[availabilityStubKey(1, 2, "gpt-4o")] = 100
	stubKeyAvailabilityScores[availabilityStubKey(1, 3, "gpt-4o")] = 30

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 2 {
		t.Fatalf("expected key 2 (highest score), got key %d", key.ID)
	}
}

func TestSelectKeyByAvailabilityTiePicksFirstInArray(t *testing.T) {
	cleanup := setupAvailabilityTest()
	defer cleanup()

	// 所有 key 满分（初始状态），应选数组第一个
	ch := makeChannelWithKeys(
		enabledKey(1, 5.0), // 成本最高，但可用度同分应优先
		enabledKey(2, 1.0), // 成本最低
	)

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 1 {
		t.Fatalf("expected key 1 (first in array on tie), got key %d", key.ID)
	}
}

func TestSelectKeyByAvailabilityAllZeroFallsBackToCost(t *testing.T) {
	cleanup := setupAvailabilityTest()
	defer cleanup()

	ch := makeChannelWithKeys(
		enabledKey(1, 5.0), // 成本高
		enabledKey(2, 1.0), // 成本低
	)
	// 所有 key 分数 ≤ 0
	stubKeyAvailabilityScores[availabilityStubKey(1, 1, "gpt-4o")] = 0
	stubKeyAvailabilityScores[availabilityStubKey(1, 2, "gpt-4o")] = 0

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 2 {
		t.Fatalf("expected key 2 (lowest cost fallback), got key %d", key.ID)
	}
}

func TestSelectKeyByCostWhenStrategyIsCost(t *testing.T) {
	cleanup := setupAvailabilityTest()
	defer cleanup()
	// 覆盖为 cost 策略
	GlobalKeySelectionStrategyFunc = func() string { return "cost" }

	ch := makeChannelWithKeys(
		enabledKey(1, 5.0), // 成本高
		enabledKey(2, 1.0), // 成本低
	)
	// 即使 key 1 可用度更高，cost 策略应选成本最低的 key 2
	stubKeyAvailabilityScores[availabilityStubKey(1, 1, "gpt-4o")] = 100
	stubKeyAvailabilityScores[availabilityStubKey(1, 2, "gpt-4o")] = 10

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 2 {
		t.Fatalf("expected key 2 (lowest cost), got key %d", key.ID)
	}
}

func TestChannelStrategyOverridesGlobal(t *testing.T) {
	cleanup := setupAvailabilityTest()
	defer cleanup()
	GlobalKeySelectionStrategyFunc = func() string { return "cost" }

	// 渠道显式设为 availability，覆盖全局 cost
	ch := makeChannelWithKeys(
		enabledKey(1, 1.0),
		enabledKey(2, 2.0),
	)
	ch.KeySelectionStrategy = "availability"
	stubKeyAvailabilityScores[availabilityStubKey(1, 1, "gpt-4o")] = 30
	stubKeyAvailabilityScores[availabilityStubKey(1, 2, "gpt-4o")] = 90

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 2 {
		t.Fatalf("expected key 2 (channel override to availability), got key %d", key.ID)
	}
}

func TestSelectKeyByAvailabilitySkipsZeroScore(t *testing.T) {
	cleanup := setupAvailabilityTest()
	defer cleanup()

	ch := makeChannelWithKeys(
		enabledKey(1, 1.0),
		enabledKey(2, 2.0),
	)
	// key 1 分数为 0（被 401 惩罚），key 2 满分
	stubKeyAvailabilityScores[availabilityStubKey(1, 1, "gpt-4o")] = 0
	stubKeyAvailabilityScores[availabilityStubKey(1, 2, "gpt-4o")] = 100

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 2 {
		t.Fatalf("expected key 2 (key 1 has zero score), got key %d", key.ID)
	}
}

func setupPriorityTest() func() {
	oldAvailabilityFunc := KeyAvailabilityScoreFunc
	oldStrategy := GlobalKeySelectionStrategyFunc
	oldCooldownFunc := KeyCooldownFunc
	KeyAvailabilityScoreFunc = nil
	GlobalKeySelectionStrategyFunc = func() string { return "priority" }
	KeyCooldownFunc = nil
	return func() {
		KeyAvailabilityScoreFunc = oldAvailabilityFunc
		GlobalKeySelectionStrategyFunc = oldStrategy
		KeyCooldownFunc = oldCooldownFunc
	}
}

func priorityKey(id int, priority int, cost float64) ChannelKey {
	k := enabledKey(id, cost)
	k.Priority = priority
	return k
}

func TestSelectKeyByPriorityPicksLowestPriority(t *testing.T) {
	cleanup := setupPriorityTest()
	defer cleanup()

	ch := makeChannelWithKeys(
		priorityKey(1, 20, 1.0),
		priorityKey(2, 10, 5.0),
		priorityKey(3, 30, 0.1),
	)

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 2 {
		t.Fatalf("expected key 2 (lowest priority value), got key %d", key.ID)
	}
}

func TestSelectKeyByPriorityTieFallsBackToLowestCost(t *testing.T) {
	cleanup := setupPriorityTest()
	defer cleanup()

	ch := makeChannelWithKeys(
		priorityKey(1, 10, 5.0),
		priorityKey(2, 10, 1.0),
		priorityKey(3, 20, 0.1),
	)

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 2 {
		t.Fatalf("expected key 2 (same priority, lowest cost), got key %d", key.ID)
	}
}

func TestSelectKeyByPrioritySkipsExcludedDisabledEmptyAndCooldown(t *testing.T) {
	cleanup := setupPriorityTest()
	defer cleanup()
	KeyCooldownFunc = func(channelID, keyID int, modelName string) bool {
		return channelID == 1 && keyID == 3 && modelName == "gpt-4o"
	}

	disabled := priorityKey(1, 1, 0.1)
	disabled.Enabled = false
	empty := priorityKey(2, 2, 0.1)
	empty.ChannelKey = ""
	cooldown := priorityKey(3, 3, 0.1)
	excluded := priorityKey(4, 4, 0.1)
	eligible := priorityKey(5, 5, 0.1)
	ch := makeChannelWithKeys(disabled, empty, cooldown, excluded, eligible)

	key := ch.GetChannelKeyExcludingWithCooldown([]int{4}, "gpt-4o", 300)
	if key.ID != 5 {
		t.Fatalf("expected key 5 (first eligible priority candidate), got key %d", key.ID)
	}
}

func TestChannelInheritsGlobalPriorityStrategy(t *testing.T) {
	cleanup := setupPriorityTest()
	defer cleanup()

	ch := makeChannelWithKeys(
		priorityKey(1, 20, 1.0),
		priorityKey(2, 10, 5.0),
	)
	ch.KeySelectionStrategy = ""

	key := ch.GetChannelKeyWithCooldown("gpt-4o", 300)
	if key.ID != 2 {
		t.Fatalf("expected key 2 (global priority strategy), got key %d", key.ID)
	}
}
