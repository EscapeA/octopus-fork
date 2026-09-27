package relay

import (
	"strconv"
	"strings"
	"testing"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	chop "github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/relay/balancer"
)

// seedAutoDisableSettings 注入自动禁用相关设置（内存缓存，测试用）。
func seedAutoDisableSettings(t *testing.T, enabled bool, threshold int) {
	t.Helper()
	setting.GetCache().Set(dbmodel.SettingKeyKeyAutoDisableEnabled, strconv.FormatBool(enabled))
	setting.GetCache().Set(dbmodel.SettingKeyKeyAutoDisableThreshold, strconv.Itoa(threshold))
}

// seedChannelWithKey 把渠道（含 Key 明文）放入渠道缓存，模拟运行态。
func seedChannelWithKey(channelID, keyID int) dbmodel.ChannelKey {
	key := dbmodel.ChannelKey{
		ID:              keyID,
		ChannelID:       channelID,
		Enabled:         true,
		ChannelKey:      "sk-test-0123456789",
		TotalCost:       1,
		SupportedModels: "deepseek-flash",
	}
	chop.GetCache().Set(channelID, dbmodel.Channel{
		ID:       channelID,
		Name:     "tokenrhythm-test",
		Enabled:  true,
		Type:     1,
		BaseUrls: []dbmodel.BaseUrl{{URL: "https://tokenrhythm.studio/v1"}},
		Keys:     []dbmodel.ChannelKey{key},
	})
	return key
}

// newTestAttempt 构造最小可用的尝试上下文（请求级 + 渠道/Key），
// 用于直接驱动 402 处理入口（含「同请求只计一次」的去重逻辑）。
func newTestAttempt(ch *dbmodel.Channel, key dbmodel.ChannelKey) *relayAttempt {
	return &relayAttempt{relayRequest: &relayRequest{}, channel: ch, usedKey: key}
}

func cachedKey(t *testing.T, channelID, keyID int) dbmodel.ChannelKey {
	t.Helper()
	channel, ok := chop.GetCache().Get(channelID)
	if !ok {
		t.Fatalf("channel %d not in cache", channelID)
	}
	for _, key := range channel.Keys {
		if key.ID == keyID {
			return key
		}
	}
	t.Fatalf("key %d not found in cached channel %d", keyID, channelID)
	return dbmodel.ChannelKey{}
}

func TestHandleUpstreamPaymentRequiredDisablesKeyAtThreshold(t *testing.T) {
	const channelID, keyID = 9201, 5301
	seedAutoDisableSettings(t, true, 3)
	key := seedChannelWithKey(channelID, keyID)
	t.Cleanup(func() { balancer.RemoveKeyAutoDisableCounter(keyID) })

	channel, _ := chop.GetCache().Get(channelID)
	for i := 1; i <= 2; i++ {
		handleUpstreamPaymentRequired(newTestAttempt(&channel, key), "deepseek-flash")
		if !cachedKey(t, channelID, keyID).Enabled {
			t.Fatalf("第 %d 次 402 就禁用了 Key，阈值应为 3", i)
		}
	}
	handleUpstreamPaymentRequired(newTestAttempt(&channel, key), "deepseek-flash")

	got := cachedKey(t, channelID, keyID)
	if got.Enabled {
		t.Fatal("达到阈值后 Key 应被自动禁用（Enabled=false）")
	}
	if got.AutoDisabledAt == nil {
		t.Fatal("自动禁用应写入 auto_disabled_at")
	}
	if !strings.Contains(got.AutoDisableReason, "upstream 402") {
		t.Fatalf("auto_disable_reason = %q, 应包含 'upstream 402'", got.AutoDisableReason)
	}
	// 触发禁用后计数清零，重新启用后需要重新累计。
	if n := balancer.GetAutoDisableFailureCount(channelID, keyID); n != 0 {
		t.Fatalf("禁用后计数 = %d, want 0", n)
	}
}

func TestHandleUpstreamPaymentRequiredRespectsSwitch(t *testing.T) {
	const channelID, keyID = 9202, 5302
	seedAutoDisableSettings(t, false, 1)
	key := seedChannelWithKey(channelID, keyID)
	t.Cleanup(func() { balancer.RemoveKeyAutoDisableCounter(keyID) })

	channel, _ := chop.GetCache().Get(channelID)
	for i := 0; i < 5; i++ {
		handleUpstreamPaymentRequired(newTestAttempt(&channel, key), "deepseek-flash")
	}
	if !cachedKey(t, channelID, keyID).Enabled {
		t.Fatal("开关关闭时不应自动禁用 Key")
	}
	if n := balancer.GetAutoDisableFailureCount(channelID, keyID); n != 0 {
		t.Fatalf("开关关闭时不应计数，得到 %d", n)
	}
}

func TestReEnableAutoDisabledChannelKeyClearsMarkers(t *testing.T) {
	const channelID, keyID = 9203, 5303
	seedAutoDisableSettings(t, true, 1)
	key := seedChannelWithKey(channelID, keyID)

	channel, _ := chop.GetCache().Get(channelID)
	handleUpstreamPaymentRequired(newTestAttempt(&channel, key), "deepseek-flash")
	if cachedKey(t, channelID, keyID).Enabled {
		t.Fatal("阈值 1 时应立即禁用")
	}

	if !ReEnableAutoDisabledChannelKey(&channel, keyID) {
		t.Fatal("ReEnableAutoDisabledChannelKey 应报告状态变更")
	}
	got := cachedKey(t, channelID, keyID)
	if !got.Enabled || got.AutoDisabledAt != nil || got.AutoDisableReason != "" {
		t.Fatalf("恢复后应为 enabled 且标记清空，得到 enabled=%v at=%v reason=%q", got.Enabled, got.AutoDisabledAt, got.AutoDisableReason)
	}
	if ReEnableAutoDisabledChannelKey(&channel, keyID) {
		t.Fatal("重复恢复不应再报告变更")
	}
}

func TestUpstreamPaymentRequiredCountsOncePerRequest(t *testing.T) {
	const channelID, keyID = 9204, 5304
	seedAutoDisableSettings(t, true, 3)
	key := seedChannelWithKey(channelID, keyID)
	t.Cleanup(func() { balancer.RemoveKeyAutoDisableCounter(keyID) })

	channel, _ := chop.GetCache().Get(channelID)
	// 同一个请求（同一个 relayRequest）内重试 5 次 402，只应计 1 次。
	ra := newTestAttempt(&channel, key)
	for i := 0; i < 5; i++ {
		handleUpstreamPaymentRequired(ra, "deepseek-flash")
	}
	if n := balancer.GetAutoDisableFailureCount(channelID, keyID); n != 1 {
		t.Fatalf("同一请求内计数 = %d, want 1（阈值语义是连续 N 次请求）", n)
	}
	if !cachedKey(t, channelID, keyID).Enabled {
		t.Fatal("单次请求不应达到 3 次阈值")
	}
	// 新的请求（新 relayAttempt/新 relayRequest）继续累计。
	for i := 0; i < 2; i++ {
		handleUpstreamPaymentRequired(newTestAttempt(&channel, key), "deepseek-flash")
	}
	if cachedKey(t, channelID, keyID).Enabled {
		t.Fatal("累计 3 次请求后应自动禁用")
	}
}

func TestAutoDisabledKeyMarksRequestForStaleWriteProtection(t *testing.T) {
	// 自动禁用后必须在本请求上留下标记：relay 失败分支据此跳过 ch.KeyUpdate(旧副本)，
	// 否则会把 enabled=false 覆盖回 true（缓存立即失效 + 刷盘回滚 DB，实测踩过）。
	const channelID, keyID = 9205, 5305
	seedAutoDisableSettings(t, true, 1)
	key := seedChannelWithKey(channelID, keyID)

	channel, _ := chop.GetCache().Get(channelID)
	ra := newTestAttempt(&channel, key)
	if ra.keyAutoDisabledInRequest(keyID) {
		t.Fatal("禁用前不应有标记")
	}
	handleUpstreamPaymentRequired(ra, "deepseek-flash")
	if !ra.keyAutoDisabledInRequest(keyID) {
		t.Fatal("自动禁用后应在请求上标记该 Key，供陈旧写回保护使用")
	}
	if cachedKey(t, channelID, keyID).Enabled {
		t.Fatal("阈值 1 时应已禁用")
	}
}

func TestKeyAutoDisableProbeIntervalSetting(t *testing.T) {
	setting.GetCache().Set(dbmodel.SettingKeyKeyAutoDisableProbeInterval, "0")
	if d := KeyAutoDisableProbeInterval(); d != 0 {
		t.Fatalf("0 应表示关闭试活，得到 %v", d)
	}
	setting.GetCache().Set(dbmodel.SettingKeyKeyAutoDisableProbeInterval, "30")
	if d := KeyAutoDisableProbeInterval(); d.Minutes() != 30 {
		t.Fatalf("试活间隔 = %v, want 30m", d)
	}
}

func TestKeyAutoDisableLabelFallsBackToMaskedKey(t *testing.T) {
	label := keyAutoDisableLabel(dbmodel.ChannelKey{ID: 7, ChannelKey: "sk_tr_abcdefghijklmnop"})
	if !strings.Contains(label, "...") {
		t.Fatalf("无备注时应回退到脱敏 Key，得到 %q", label)
	}
}
