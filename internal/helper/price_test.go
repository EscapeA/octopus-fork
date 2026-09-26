package helper

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
)

// 价格目录以人民币手工维护：渠道同步新发现的模型一律 0 价（未定价），
// 不再从 any 价格源（models.dev / 内置价表）自动填价。
func TestLLMPriceAddToDB_WritesZeroPrice(t *testing.T) {
	setupHelperDB(t)

	llmCache := llm.GetCache()
	oldLLMs := llmCache.GetAll()
	llmCache.Clear()
	defer func() {
		llmCache.Clear()
		for k, v := range oldLLMs {
			llmCache.Set(k, v)
		}
	}()

	ctx := context.Background()
	// 空串应被跳过；其余模型建 0 价占位行。
	if err := LLMPriceAddToDB([]string{"new-model-a", "", "new-model-b"}, ctx); err != nil {
		t.Fatalf("LLMPriceAddToDB() error = %v", err)
	}

	for _, name := range []string{"new-model-a", "new-model-b"} {
		got, err := llm.Get(name)
		if err != nil {
			t.Fatalf("llm.Get(%s) error = %v", name, err)
		}
		if got != (model.LLMPrice{}) {
			t.Fatalf("%s price = %+v, want zero（未定价，价格由界面按人民币填写）", name, got)
		}
	}
}

// TestLLMPriceDeleteFromDBWithNoPrice_SkipsManualModels 验证：手动创建的模型
// （price_manual=true，即使价格为 0）不会被"删 0 价格模型"任务删除。
func TestLLMPriceDeleteFromDBWithNoPrice_SkipsManualModels(t *testing.T) {
	setupHelperDB(t)

	llmCache := llm.GetCache()
	oldLLMs := llmCache.GetAll()
	llmCache.Clear()
	defer func() {
		llmCache.Clear()
		for k, v := range oldLLMs {
			llmCache.Set(k, v)
		}
	}()

	ctx := context.Background()
	// 手动创建 0 价模型（用户明确创建，即使没填价格也不应被自动删除）。
	if err := llm.Create(model.LLMInfo{Name: "manual-zero-model-xyz"}, ctx); err != nil {
		t.Fatalf("llm.Create(manual-zero) error = %v", err)
	}
	// 同步 0 价模型：应被删除。
	if err := llm.BatchCreate([]model.LLMInfo{{Name: "sync-zero-model-xyz"}}, ctx); err != nil {
		t.Fatalf("llm.BatchCreate(sync-zero) error = %v", err)
	}

	if err := LLMPriceDeleteFromDBWithNoPrice([]string{"manual-zero-model-xyz", "sync-zero-model-xyz"}, ctx); err != nil {
		t.Fatalf("LLMPriceDeleteFromDBWithNoPrice() error = %v", err)
	}

	// 手动模型仍存在。
	if _, err := llm.Get("manual-zero-model-xyz"); err != nil {
		t.Fatalf("manual-zero model deleted, want preserved: %v", err)
	}
	// 同步模型已删除。
	if _, err := llm.Get("sync-zero-model-xyz"); err == nil {
		t.Fatal("sync-zero model still exists, want deleted")
	}
}
