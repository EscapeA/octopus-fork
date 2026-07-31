package price

import (
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

// setPricesForTest replaces the global llmPrice map for the duration of a test
// and returns a restore function. Callers that invoke matchFallbackPrice
// directly must hold llmPriceLock (RLock) themselves, matching GetLLMPrice.
func setPricesForTest(prices map[string]model.LLMPrice) func() {
	llmPriceLock.Lock()
	old := llmPrice
	llmPrice = prices
	llmPriceLock.Unlock()
	return func() {
		llmPriceLock.Lock()
		llmPrice = old
		llmPriceLock.Unlock()
	}
}

func TestMatchFallbackPrice(t *testing.T) {
	prices := map[string]model.LLMPrice{
		"gpt-4o":            {Input: 1},
		"gpt-4o-mini":       {Input: 2},
		"claude-3-5-sonnet": {Input: 3},
	}
	restore := setPricesForTest(prices)
	t.Cleanup(restore)

	cases := []struct {
		name      string
		modelName string
		want      string // expected matched key, "" means no match
	}{
		// Strategy 1: provider/ prefix stripped
		{"provider prefix", "openai/gpt-4o", "gpt-4o"},
		{"nested provider prefix", "azure/openai/gpt-4o-mini", "gpt-4o-mini"},

		// Strategy 2: whole-word substring, longest match wins
		{"exact match", "gpt-4o", "gpt-4o"},
		{"longest wins", "gpt-4o-mini", "gpt-4o-mini"},
		{"prefix separator", "my-gpt-4o", "gpt-4o"},
		{"surrounding separators", "proxy.gpt-4o.relay", "gpt-4o"},

		// Boundary correctness: alphanumeric粘连 must NOT match
		{"leading alphanum rejected", "xgpt-4o", ""},
		{"trailing alphanum rejected", "gpt-4ox", ""},
		{"trailing uppercase rejected", "gpt-4oMini", ""},

		// No match
		{"unknown model", "totally-unknown-model", ""},
	}

	llmPriceLock.RLock()
	defer llmPriceLock.RUnlock()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchFallbackPrice(tc.modelName)
			switch {
			case tc.want == "" && got != nil:
				t.Fatalf("matchFallbackPrice(%q) = %+v, want nil", tc.modelName, got)
			case tc.want != "" && got == nil:
				t.Fatalf("matchFallbackPrice(%q) = nil, want key %q", tc.modelName, tc.want)
			case tc.want != "" && got.Input != prices[tc.want].Input:
				t.Fatalf("matchFallbackPrice(%q) Input = %v, want %v (key %q)",
					tc.modelName, got.Input, prices[tc.want].Input, tc.want)
			}
		})
	}
}

// TestDeepSeekV4PresetPrices 验证手工维护的 deepseek-v4 系列价格预设
// （presets_manual.go）存在且四值正确，并验证 provider/ 前缀回落可命中。
func TestDeepSeekV4PresetPrices(t *testing.T) {
	llmPriceLock.RLock()
	defer llmPriceLock.RUnlock()

	cases := []struct {
		model string
		want  model.LLMPrice
	}{
		{"deepseek-v4-flash", model.LLMPrice{Input: 0.14, Output: 0.28, CacheRead: 0.0028, CacheWrite: 0}},
		{"deepseek-v4-pro", model.LLMPrice{Input: 0.42, Output: 0.84, CacheRead: 0.0035, CacheWrite: 0}},
	}
	for _, tc := range cases {
		p, ok := llmPrice[tc.model]
		if !ok {
			t.Fatalf("llmPrice[%q] missing, want %+v", tc.model, tc.want)
		}
		if p != tc.want {
			t.Errorf("llmPrice[%q] = %+v, want %+v", tc.model, p, tc.want)
		}
		// provider/ 前缀形式应能经 matchFallbackPrice 命中
		got := matchFallbackPrice("deepseek/" + tc.model)
		if got == nil || got.Input != tc.want.Input {
			t.Errorf("matchFallbackPrice(\"deepseek/%s\") = %+v, want Input %v",
				tc.model, got, tc.want.Input)
		}
	}
}

func TestContainsWholeWord(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"gpt-4o", "gpt-4o", true},
		{"gpt-4o-mini", "gpt-4o", true},
		{"my-gpt-4o", "gpt-4o", true},
		{"a.gpt-4o.b", "gpt-4o", true},
		{"xgpt-4o", "gpt-4o", false},
		{"gpt-4ox", "gpt-4o", false},
		{"gpt-4ogpt-4o", "gpt-4o", false}, // 两次出现都被字母粘连
		{"totally-unknown", "gpt-4o", false},
	}
	for _, tc := range cases {
		if got := containsWholeWord(tc.s, tc.sub); got != tc.want {
			t.Errorf("containsWholeWord(%q, %q) = %v, want %v", tc.s, tc.sub, got, tc.want)
		}
	}
}
