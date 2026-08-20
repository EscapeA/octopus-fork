package price

import (
	"math"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

// shanghaiLoc 与 deepSeekLocation 使用相同固定偏移构造，避免依赖系统 tzdata。
var shanghaiLoc = time.FixedZone("UTC+8", 8*3600)

func mustTime(t *testing.T, h, m, s int) time.Time {
	t.Helper()
	return time.Date(2026, 8, 17, h, m, s, 0, shanghaiLoc)
}

func TestDeepSeekFamily(t *testing.T) {
	cases := []struct {
		name      string
		modelName string
		want      string
	}{
		{"exact flash", "deepseek-v4-flash", "flash"},
		{"flash variant", "deepseek-v4-flash-0731", "flash"},
		{"deepseek prefix pro", "deepseek/deepseek-v4-pro", "pro"},
		{"deepseek-ai prefix flash", "deepseek-ai/deepseek-v4-flash-0731", "flash"},
		{"deepseek prefix flash free", "deepseek/deepseek-v4-flash:free", "flash"},
		{"uppercase", "DEEPSEEK-V4-FLASH", "flash"},

		// 中转商前缀必须排除
		{"olm prefix rejected", "olm/deepseek-v4-pro", ""},
		{"vova prefix rejected", "vova/deepseek-v4-flash", ""},
		{"opc prefix rejected", "opc/deepseek-v4-flash-free", ""},
		{"other provider prefix rejected", "openai/deepseek-v4-flash", ""},

		// 非 v4 模型
		{"deepseek-chat rejected", "deepseek-chat", ""},
		{"gpt-4o rejected", "gpt-4o", ""},
		{"empty rejected", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeepSeekFamily(tc.modelName)
			if got != tc.want {
				t.Fatalf("DeepSeekFamily(%q) = %q, want %q", tc.modelName, got, tc.want)
			}
		})
	}
}

func TestBillingWindow(t *testing.T) {
	// 高峰窗口 [09:00,12:00) ∪ [14:00,18:00)（北京），其余空闲。
	peak := []time.Time{
		mustTime(t, 9, 0, 0),
		mustTime(t, 9, 30, 0),
		mustTime(t, 11, 59, 59),
		mustTime(t, 14, 0, 0),
		mustTime(t, 16, 0, 0),
		mustTime(t, 17, 59, 59),
	}
	offpeak := []time.Time{
		mustTime(t, 0, 0, 0),
		mustTime(t, 8, 59, 59),
		mustTime(t, 12, 0, 0),
		mustTime(t, 13, 59, 59),
		mustTime(t, 18, 0, 0),
		mustTime(t, 23, 59, 59),
	}
	for _, at := range peak {
		if got := BillingWindow("deepseek-v4-flash", at); got != BillingWindowPeak {
			t.Errorf("BillingWindow(%v) = %q, want %q", at, got, BillingWindowPeak)
		}
	}
	for _, at := range offpeak {
		if got := BillingWindow("deepseek-v4-flash", at); got != BillingWindowOffPeak {
			t.Errorf("BillingWindow(%v) = %q, want %q", at, got, BillingWindowOffPeak)
		}
	}
}

func TestBillingWindowUTCInput(t *testing.T) {
	// UTC 输入：01:00Z = 北京 09:00 → 高峰；04:00Z = 北京 12:00 → 空闲。
	cases := []struct {
		name string
		utc  time.Time
		want string
	}{
		{"01:00Z is beijing 09:00 peak", time.Date(2026, 8, 17, 1, 0, 0, 0, time.UTC), BillingWindowPeak},
		{"04:00Z is beijing 12:00 offpeak", time.Date(2026, 8, 17, 4, 0, 0, 0, time.UTC), BillingWindowOffPeak},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BillingWindow("deepseek-v4-pro", tc.utc); got != tc.want {
				t.Fatalf("BillingWindow = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBillingWindowNonDeepSeek(t *testing.T) {
	if got := BillingWindow("gpt-4o", mustTime(t, 10, 0, 0)); got != BillingWindowNone {
		t.Fatalf("BillingWindow(gpt-4o) = %q, want %q", got, BillingWindowNone)
	}
	// 中转商前缀模型不套时段
	if got := BillingWindow("olm/deepseek-v4-pro", mustTime(t, 10, 0, 0)); got != BillingWindowNone {
		t.Fatalf("BillingWindow(olm/deepseek-v4-pro) = %q, want %q", got, BillingWindowNone)
	}
}

func TestScaleLLMPrice(t *testing.T) {
	p := model.LLMPrice{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}
	got := ScaleLLMPrice(p, 0.5)
	want := model.LLMPrice{Input: 0.5, Output: 1, CacheRead: 1.5, CacheWrite: 2}
	if got != want {
		t.Fatalf("ScaleLLMPrice = %+v, want %+v", got, want)
	}
}

func TestEffectiveLLMPrice(t *testing.T) {
	// 用 setPricesForTest 替换 map，避免依赖 DB/预设实际值。
	prices := map[string]model.LLMPrice{
		"deepseek-v4-flash": {Input: 1, Output: 2, CacheRead: 0.1, CacheWrite: 0},
		"gpt-4o":            {Input: 5, Output: 15, CacheRead: 0, CacheWrite: 0},
	}
	restore := setPricesForTest(prices)
	t.Cleanup(restore)

	// 高峰 10:00 → 原价
	peak := EffectiveLLMPrice("deepseek-v4-flash", mustTime(t, 10, 0, 0))
	if peak == nil || peak.Input != 1 || peak.Output != 2 || peak.CacheRead != 0.1 {
		t.Fatalf("peak EffectiveLLMPrice = %+v, want {1 2 0.1 0}", peak)
	}
	// 空闲 13:00 → 一半
	off := EffectiveLLMPrice("deepseek-v4-flash", mustTime(t, 13, 0, 0))
	if off == nil || off.Input != 0.5 || off.Output != 1 || off.CacheRead != 0.05 || off.CacheWrite != 0 {
		t.Fatalf("offpeak EffectiveLLMPrice = %+v, want {0.5 1 0.05 0}", off)
	}
	// 非 DeepSeek 两个时刻相同
	a := EffectiveLLMPrice("gpt-4o", mustTime(t, 10, 0, 0))
	b := EffectiveLLMPrice("gpt-4o", mustTime(t, 13, 0, 0))
	if a == nil || b == nil || !floatEqual(a.Input, b.Input) {
		t.Fatalf("gpt-4o prices differ across windows: %+v vs %+v", a, b)
	}
	// 未知模型 → nil
	if got := EffectiveLLMPrice("totally-unknown", mustTime(t, 10, 0, 0)); got != nil {
		t.Fatalf("unknown model = %+v, want nil", got)
	}
}

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-12
}

// TestOverlayDeepSeekPeakPresets 验证价格同步（models.dev 旧平价覆盖后）
// overlay 会把 flash/pro 回盖成高峰预设。
func TestOverlayDeepSeekPeakPresets(t *testing.T) {
	// 模拟同步后 map 被旧平价覆盖
	prices := map[string]model.LLMPrice{
		"deepseek-v4-flash": {Input: 0.14, Output: 0.28, CacheRead: 0.0028, CacheWrite: 0},
		"deepseek-v4-pro":   {Input: 0.42, Output: 0.84, CacheRead: 0.0035, CacheWrite: 0},
		"gpt-4o":            {Input: 5, Output: 15, CacheRead: 2.5, CacheWrite: 0},
	}
	restore := setPricesForTest(prices)
	t.Cleanup(restore)

	overlayDeepSeekPeakPresets()

	want := model.LLMPrice{
		Input: 3.0 / deepSeekCNYPerUSD, Output: 9.0 / deepSeekCNYPerUSD,
		CacheRead: 0.10 / deepSeekCNYPerUSD, CacheWrite: 0,
	}
	got := llmPrice["deepseek-v4-flash"]
	if !floatEqual(got.Input, want.Input) || !floatEqual(got.Output, want.Output) ||
		!floatEqual(got.CacheRead, want.CacheRead) || !floatEqual(got.CacheWrite, want.CacheWrite) {
		t.Fatalf("flash after overlay = %+v, want %+v", got, want)
	}
	// 非白名单模型不受影响
	if g := llmPrice["gpt-4o"]; g.Input != 5 {
		t.Fatalf("gpt-4o changed after overlay: %+v", g)
	}
}

// TestPeakPresetPrice 验证白名单高峰预设取值。
func TestPeakPresetPrice(t *testing.T) {
	flash := PeakPresetPrice("deepseek-v4-flash")
	if flash == nil || !floatEqual(flash.Input, 3.0/deepSeekCNYPerUSD) {
		t.Fatalf("PeakPresetPrice(flash) = %+v", flash)
	}
	if p := PeakPresetPrice("deepseek-v4-pro"); p == nil || !floatEqual(p.Output, 27.0/deepSeekCNYPerUSD) {
		t.Fatalf("PeakPresetPrice(pro) = %+v", p)
	}
	if p := PeakPresetPrice("deepseek/deepseek-v4-flash"); p != nil {
		t.Fatalf("PeakPresetPrice(deepseek/deepseek-v4-flash) = %+v, want nil (仅精确键)", p)
	}
	if p := PeakPresetPrice("gpt-4o"); p != nil {
		t.Fatalf("PeakPresetPrice(gpt-4o) = %+v, want nil", p)
	}
}
