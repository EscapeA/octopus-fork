package price

import (
	"testing"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
)

// 分类规则只兜底未定价模型：DB 有非 0 手工人民币价时分类不生效。
func TestGetLLMPrice_CategoryDoesNotOverrideManualPrice(t *testing.T) {
	initPriceTestDB(t)
	if err := llm.Create(model.LLMInfo{
		Name:     "my-model-a",
		LLMPrice: model.LLMPrice{Input: 7, Output: 21},
	}, t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := llm.CreatePriceCategory(model.ModelPriceCategory{
		Name:      "my-models",
		RuleType:  string(model.ModelPriceCategoryRulePrefix),
		RuleValue: "my-",
		LLMPrice:  model.LLMPrice{Input: 42, Output: 84},
		SortOrder: 1,
		Enabled:   true,
	}, t.Context()); err != nil {
		t.Fatal(err)
	}

	got := GetLLMPrice("my-model-a")
	if got == nil || got.Input != 7 {
		t.Fatalf("GetLLMPrice = %+v, want manual price Input 7 (category 42 must not win)", got)
	}
}

// 0 价占位行（未定价）→ 分类兜底生效。
func TestGetLLMPrice_CategoryFillsUnpricedModel(t *testing.T) {
	initPriceTestDB(t)
	if err := llm.BatchCreate([]model.LLMInfo{{Name: "my-model-b"}}, t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := llm.CreatePriceCategory(model.ModelPriceCategory{
		Name:      "my-models",
		RuleType:  string(model.ModelPriceCategoryRulePrefix),
		RuleValue: "my-",
		LLMPrice:  model.LLMPrice{Input: 42, Output: 84},
		SortOrder: 1,
		Enabled:   true,
	}, t.Context()); err != nil {
		t.Fatal(err)
	}

	got := GetLLMPrice("my-model-b")
	if got == nil || got.Input != 42 {
		t.Fatalf("GetLLMPrice = %+v, want category price Input 42", got)
	}
}
