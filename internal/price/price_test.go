package price

import (
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
)

// initPriceTestDB 建独立测试库。价格目录只来自 DB（人民币手工价）与分类规则：
// 内存美元价表、models.dev 同步、整词子串兜底均已随「去美元」移除。
func initPriceTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	// llm 包的价格缓存是包级全局的，跨用例会互相污染：清空并在结束时恢复。
	cache := llm.GetCache()
	old := cache.GetAll()
	cache.Clear()
	t.Cleanup(func() {
		_ = db.Close()
		cache.Clear()
		for k, v := range old {
			cache.Set(k, v)
		}
	})
}

// DB 手工价命中即返回，模型名大小写不敏感。
func TestGetLLMPrice_ManualPriceWins(t *testing.T) {
	initPriceTestDB(t)
	if err := llm.Create(model.LLMInfo{
		Name:     "my-model",
		LLMPrice: model.LLMPrice{Input: 3, Output: 9, CacheRead: 0.3},
	}, t.Context()); err != nil {
		t.Fatal(err)
	}
	got := GetLLMPrice("MY-MODEL")
	if got == nil || got.Input != 3 || got.Output != 9 || got.CacheRead != 0.3 {
		t.Fatalf("GetLLMPrice = %+v, want 3/9/0.3", got)
	}
}

// 未定价占位行（四价全 0）→ 分类规则兜底生效。
func TestGetLLMPrice_UnpricedRowFallsBackToCategory(t *testing.T) {
	initPriceTestDB(t)
	if err := llm.BatchCreate([]model.LLMInfo{{Name: "cat-model-a"}}, t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := llm.CreatePriceCategory(model.ModelPriceCategory{
		Name:      "cat-models",
		RuleType:  string(model.ModelPriceCategoryRulePrefix),
		RuleValue: "cat-model",
		LLMPrice:  model.LLMPrice{Input: 42, Output: 84},
		SortOrder: 1,
		Enabled:   true,
	}, t.Context()); err != nil {
		t.Fatal(err)
	}
	got := GetLLMPrice("cat-model-a")
	if got == nil || got.Input != 42 {
		t.Fatalf("GetLLMPrice = %+v, want category price Input 42", got)
	}
}

// 未定价且无任何兜底 → 返回 0 价行（未定价 = 不计费），不是 nil。
func TestGetLLMPrice_UnpricedRowReturnsZero(t *testing.T) {
	initPriceTestDB(t)
	if err := llm.BatchCreate([]model.LLMInfo{{Name: "unpriced-model"}}, t.Context()); err != nil {
		t.Fatal(err)
	}
	got := GetLLMPrice("unpriced-model")
	if got == nil || *got != (model.LLMPrice{}) {
		t.Fatalf("GetLLMPrice = %+v, want zero price row", got)
	}
}

// DB 无行且无兜底 → nil（调用方按未定价处理）。
func TestGetLLMPrice_UnknownModelReturnsNil(t *testing.T) {
	initPriceTestDB(t)
	if got := GetLLMPrice("totally-unknown-model"); got != nil {
		t.Fatalf("GetLLMPrice = %+v, want nil", got)
	}
}

// 内置美元价目已彻底移除：历史内置条目名不再返回任何价格
// （既无内存价表命中，也无 provider 前缀/整词子串兜底）。
func TestGetLLMPrice_NoBuiltinUsdPricesRemain(t *testing.T) {
	initPriceTestDB(t)
	for _, name := range []string{"gpt-4o", "claude-3-5-sonnet", "deepseek-chat", "openai/gpt-4o", "my-gpt-4o-extra"} {
		if got := GetLLMPrice(name); got != nil {
			t.Fatalf("GetLLMPrice(%q) = %+v, want nil（内置美元价目与子串兜底应已移除）", name, got)
		}
	}
}

func TestIsZeroPrice(t *testing.T) {
	if !isZeroPrice(model.LLMPrice{}) {
		t.Fatal("isZeroPrice(zero) = false, want true")
	}
	if isZeroPrice(model.LLMPrice{Input: 0.0001}) {
		t.Fatal("isZeroPrice(non-zero) = true, want false")
	}
}
