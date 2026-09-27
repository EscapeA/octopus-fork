package helper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	appmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

// keyRequestCounter 记录探测请求实际用到的 key（Authorization 头），
// 用于断言「测试」选了哪个 key。
type keyRequestCounter struct {
	mu   sync.Mutex
	seen map[string]int
}

func (c *keyRequestCounter) hit(auth string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen == nil {
		c.seen = map[string]int{}
	}
	c.seen[auth]++
}

func (c *keyRequestCounter) count(auth string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seen[auth]
}

func (c *keyRequestCounter) snapshot() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]int{}
	for k, v := range c.seen {
		out[k] = v
	}
	return out
}

// TestTestGroupModelItem_SelectsKeyByModel 验证按被测模型选 key：
// 成本最低的 key 不支持该模型（supported_models 限定为别的模型）时应被过滤掉，
// 只用支持该模型的 key —— 否则「模型只对某个 key 开放」时测试会误报 403。
func TestTestGroupModelItem_SelectsKeyByModel(t *testing.T) {
	setupHelperDB(t)

	counter := &keyRequestCounter{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		counter.hit(auth)
		if auth != "Bearer sk-beta" {
			// 模拟「该 key 无该模型权限」的上游响应
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"当前模型仅对灰度用户开放","code":"MODEL_ACCESS_DENIED"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"neohorse-1-9b",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer server.Close()

	channels := map[int]appmodel.Channel{
		56: {
			ID:       56,
			Name:     "relay",
			Enabled:  true,
			Type:     outbound.OutboundTypeOpenAIChat,
			BaseUrls: []appmodel.BaseUrl{{URL: server.URL}},
			Keys: []appmodel.ChannelKey{
				// 成本最低（cost 策略会优先选它），但不支持被测模型
				{ID: 11, Enabled: true, ChannelKey: "sk-cheap", TotalCost: 0, SupportedModels: "other-model"},
				// 支持被测模型，成本更高
				{ID: 12, Enabled: true, ChannelKey: "sk-beta", TotalCost: 5, SupportedModels: "neohorse-1-9b"},
			},
		},
	}
	item := appmodel.GroupItem{ID: 1, ChannelID: 56, ModelName: "neohorse-1-9b"}

	result := testGroupModelItem(context.Background(), appmodel.EndpointTypeChat, item, channels)

	if !result.Passed {
		t.Fatalf("probe Passed = false (message %q), want true: 应按模型过滤 key", result.Message)
	}
	if got := counter.count("Bearer sk-cheap"); got != 0 {
		t.Fatalf("probe used the key that does not support the model %d times, want 0 (seen=%v)", got, counter.snapshot())
	}
	if got := counter.count("Bearer sk-beta"); got == 0 {
		t.Fatalf("probe never used the model-scoped key (seen=%v)", counter.snapshot())
	}
}

// TestTestGroupModelItem_SwitchesKeyOnFailure 验证失败换 key 重试：
// 首个候选 key 对该模型返回 403 时，应切到同渠道下一个支持该模型的 key 并成功，
// 而不是把同一个 key 重试 6 次（旧行为）后整组测试判失败。
func TestTestGroupModelItem_SwitchesKeyOnFailure(t *testing.T) {
	setupHelperDB(t)

	counter := &keyRequestCounter{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		counter.hit(auth)
		if auth == "Bearer sk-bad" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"no permission","code":"MODEL_ACCESS_DENIED"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-2","object":"chat.completion","created":1,"model":"neohorse-1-9b",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer server.Close()

	channels := map[int]appmodel.Channel{
		56: {
			ID:       56,
			Name:     "relay",
			Enabled:  true,
			Type:     outbound.OutboundTypeOpenAIChat,
			BaseUrls: []appmodel.BaseUrl{{URL: server.URL}},
			Keys: []appmodel.ChannelKey{
				{ID: 21, Enabled: true, ChannelKey: "sk-bad", TotalCost: 0, SupportedModels: "neohorse-1-9b"},
				{ID: 22, Enabled: true, ChannelKey: "sk-good", TotalCost: 5, SupportedModels: "neohorse-1-9b"},
			},
		},
	}
	item := appmodel.GroupItem{ID: 1, ChannelID: 56, ModelName: "neohorse-1-9b"}

	result := testGroupModelItem(context.Background(), appmodel.EndpointTypeChat, item, channels)

	if !result.Passed {
		t.Fatalf("probe Passed = false (message %q), want true: 应在失败后换 key", result.Message)
	}
	seen := counter.snapshot()
	if seen["Bearer sk-good"] == 0 {
		t.Fatalf("probe never tried the second key (seen=%v)", seen)
	}
	if seen["Bearer sk-bad"] != 1 {
		t.Fatalf("sk-bad hit %d times, want 1 (失败即换 key，不重复打同一个 key；seen=%v)", seen["Bearer sk-bad"], seen)
	}
}

// TestTestGroupModelItem_ModelScopedKeylessChannel 验证 key 全部不支持被测模型时
// 返回 "no available key"（与真实转发的候选为空一致）。
func TestTestGroupModelItem_ModelScopedKeylessChannel(t *testing.T) {
	setupHelperDB(t)

	channels := map[int]appmodel.Channel{
		57: {
			ID:       57,
			Name:     "relay2",
			Enabled:  true,
			Type:     outbound.OutboundTypeOpenAIChat,
			BaseUrls: []appmodel.BaseUrl{{URL: "http://127.0.0.1:1"}},
			Keys: []appmodel.ChannelKey{
				{ID: 31, Enabled: true, ChannelKey: "sk-a", SupportedModels: "model-x"},
			},
		},
	}
	item := appmodel.GroupItem{ID: 1, ChannelID: 57, ModelName: "model-y"}

	result := testGroupModelItem(context.Background(), appmodel.EndpointTypeChat, item, channels)

	if result.Passed {
		t.Fatal("probe Passed = true, want false")
	}
	if result.Message != "no available key" {
		t.Fatalf("message = %q, want \"no available key\"", result.Message)
	}
}
