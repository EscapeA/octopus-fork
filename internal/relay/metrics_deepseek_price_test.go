package relay

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

// setupRelayPriceDB 建测试库并显式写入人民币价目：
// 价格目录不再有任何内置/同步价格，涉及金额的断言必须自己建价。
//   - 峰谷规则 deepseek-v4-flash：高峰 ¥0.44/¥1.32（空闲 ×0.5 由规则给出）
//   - 手工价 gpt-4o：¥5/¥15（不套峰谷，两个窗口同价）
func setupRelayPriceDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := t.Context()
	if err := llm.SeedPriceSchedules(ctx); err != nil {
		t.Fatalf("SeedPriceSchedules: %v", err)
	}
	rows, err := llm.ListPriceSchedules(ctx)
	if err != nil {
		t.Fatalf("ListPriceSchedules: %v", err)
	}
	for _, row := range rows {
		if row.Name != "deepseek-v4-flash" {
			continue
		}
		row.LLMPrice = model.LLMPrice{Input: 0.44, Output: 1.32, CacheRead: 0.014}
		if _, err := llm.UpdatePriceSchedule(row, ctx); err != nil {
			t.Fatalf("UpdatePriceSchedule: %v", err)
		}
	}
	if err := llm.RefreshPriceScheduleCache(ctx); err != nil {
		t.Fatalf("RefreshPriceScheduleCache: %v", err)
	}
	if err := llm.Create(model.LLMInfo{
		Name:     "gpt-4o",
		LLMPrice: model.LLMPrice{Input: 5, Output: 15},
	}, ctx); err != nil {
		t.Fatalf("llm.Create(gpt-4o): %v", err)
	}
}

// shanghaiLocRelay 与 price 包 deepSeekLocation 相同固定偏移，用于构造北京时刻。
var shanghaiLocRelay = time.FixedZone("UTC+8", 8*3600)

func beijingRelay(t *testing.T, h, m int) time.Time {
	t.Helper()
	return time.Date(2026, 8, 17, h, m, 0, 0, shanghaiLocRelay)
}

// TestSetInternalResponseDeepSeekPeakPricing 验证 DeepSeek v4 白名单模型的
// 计费随请求开始时刻（StartTime）的峰谷窗口变化：
//   - 北京 10:00（高峰）→ 目录高峰价
//   - 北京 13:00（空闲）→ 高峰价 ×0.5
//   - 非 DeepSeek 模型两个时刻费用相同
func TestSetInternalResponseDeepSeekPeakPricing(t *testing.T) {
	setupRelayPriceDB(t)
	// 显式写入人民币价目（见 setupRelayPriceDB）。
	// 1e6 uncached input + 1e6 output，便于直接对比价格数值。
	resp := &transmodel.InternalLLMResponse{
		Usage: &transmodel.Usage{
			PromptTokens:     2_000_000, // 其中 1e6 cached、1e6 uncached
			CompletionTokens: 1_000_000,
			PromptTokensDetails: &transmodel.PromptTokensDetails{
				CachedTokens: 1_000_000,
			},
		},
	}

	peak := NewRelayMetrics(1, "deepseek-v4-flash", "chat", "chat", "127.0.0.1", nil)
	peak.StartTime = beijingRelay(t, 10, 0)
	peak.SetInternalResponse(resp, "deepseek-v4-flash")

	off := NewRelayMetrics(1, "deepseek-v4-flash", "chat", "chat", "127.0.0.1", nil)
	off.StartTime = beijingRelay(t, 13, 0)
	off.SetInternalResponse(resp, "deepseek-v4-flash")

	if peak.Stats.InputCost <= 0 {
		t.Fatalf("peak InputCost = %v, want > 0", peak.Stats.InputCost)
	}
	// 空闲 = 高峰 × 0.5（off-peak 倍率）
	if !floatNear(off.Stats.InputCost, peak.Stats.InputCost*0.5) {
		t.Fatalf("offpeak InputCost = %v, want peak*0.5 = %v", off.Stats.InputCost, peak.Stats.InputCost*0.5)
	}
	if !floatNear(off.Stats.OutputCost, peak.Stats.OutputCost*0.5) {
		t.Fatalf("offpeak OutputCost = %v, want peak*0.5 = %v", off.Stats.OutputCost, peak.Stats.OutputCost*0.5)
	}

	// 非 DeepSeek：两个时刻费用相同。
	gptResp := &transmodel.InternalLLMResponse{
		Usage: &transmodel.Usage{
			PromptTokens:     1_000_000,
			CompletionTokens: 1_000_000,
			PromptTokensDetails: &transmodel.PromptTokensDetails{
				CachedTokens: 0,
			},
		},
	}
	gptPeak := NewRelayMetrics(1, "gpt-4o", "chat", "chat", "127.0.0.1", nil)
	gptPeak.StartTime = beijingRelay(t, 10, 0)
	gptPeak.SetInternalResponse(gptResp, "gpt-4o")

	gptOff := NewRelayMetrics(1, "gpt-4o", "chat", "chat", "127.0.0.1", nil)
	gptOff.StartTime = beijingRelay(t, 13, 0)
	gptOff.SetInternalResponse(gptResp, "gpt-4o")

	if !floatNear(gptPeak.Stats.InputCost, gptOff.Stats.InputCost) {
		t.Fatalf("gpt-4o InputCost differs by window: %v vs %v", gptPeak.Stats.InputCost, gptOff.Stats.InputCost)
	}
}

// TestSetInternalResponseUnknownModel 验证未知模型（无价格）时 SetInternalResponse
// 不 panic、不产生费用（与原有 GetLLMPrice nil 行为一致）。
func TestSetInternalResponseUnknownModel(t *testing.T) {
	resp := &transmodel.InternalLLMResponse{
		Usage: &transmodel.Usage{
			PromptTokens:     100,
			CompletionTokens: 100,
			PromptTokensDetails: &transmodel.PromptTokensDetails{
				CachedTokens: 0,
			},
		},
	}
	m := NewRelayMetrics(1, "totally-unknown-model", "chat", "chat", "127.0.0.1", nil)
	m.StartTime = beijingRelay(t, 10, 0)
	m.SetInternalResponse(resp, "totally-unknown-model")
	if m.Stats.InputCost != 0 || m.Stats.OutputCost != 0 {
		t.Fatalf("unknown model costs = %v/%v, want 0/0", m.Stats.InputCost, m.Stats.OutputCost)
	}
}

func floatNear(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-9
}
