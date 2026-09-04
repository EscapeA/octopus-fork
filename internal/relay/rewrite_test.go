package relay

import (
	"testing"

	appmodel "github.com/lingyuins/octopus/internal/model"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

func TestPrepareInternalRequestForOutbound_IsScopedPerChannelAttempt(t *testing.T) {
	first := "first"
	second := "second"
	baseRequest := &transmodel.InternalLLMRequest{
		Model: "gpt-4o-mini",
		Messages: []transmodel.Message{
			{
				Role: "user",
				Content: transmodel.MessageContent{
					MultipleContent: []transmodel.MessageContentPart{
						{Type: "text", Text: &first},
						{Type: "text", Text: &second},
					},
				},
			},
		},
	}

	rewriteChannel := &appmodel.Channel{
		Type: outbound.OutboundTypeOpenAIChat,
		RequestRewrite: &appmodel.RequestRewriteConfig{
			Enabled: true,
			Profile: appmodel.RequestRewriteProfileOpenAIChatCompat,
		},
	}
	plainChannel := &appmodel.Channel{
		Type: outbound.OutboundTypeOpenAIChat,
	}

	rewritten, _, err := prepareInternalRequestForOutbound(rewriteChannel, baseRequest, appmodel.EndpointTypeDeepSeek)
	if err != nil {
		t.Fatalf("prepareInternalRequestForOutbound() rewrite channel error = %v", err)
	}
	plain, _, err := prepareInternalRequestForOutbound(plainChannel, baseRequest, appmodel.EndpointTypeChat)
	if err != nil {
		t.Fatalf("prepareInternalRequestForOutbound() plain channel error = %v", err)
	}

	if rewritten.Messages[0].Content.Content == nil || *rewritten.Messages[0].Content.Content != "first\nsecond" {
		t.Fatalf("rewritten content = %#v, want flattened string", rewritten.Messages[0].Content)
	}
	if plain.Messages[0].Content.Content != nil {
		t.Fatalf("plain channel content = %#v, want original multipart content", plain.Messages[0].Content)
	}
	if len(plain.Messages[0].Content.MultipleContent) != 2 {
		t.Fatalf("plain channel content parts len = %d, want 2", len(plain.Messages[0].Content.MultipleContent))
	}
	if baseRequest.Messages[0].Content.Content != nil {
		t.Fatal("base request was mutated across channel attempts")
	}
	if rewritten.TransformerMetadata[transmodel.TransformerMetadataGroupEndpointType] != appmodel.EndpointTypeDeepSeek {
		t.Fatalf("rewritten transformer metadata = %#v, want deepseek endpoint type", rewritten.TransformerMetadata)
	}
	if plain.TransformerMetadata[transmodel.TransformerMetadataGroupEndpointType] != appmodel.EndpointTypeChat {
		t.Fatalf("plain transformer metadata = %#v, want chat endpoint type", plain.TransformerMetadata)
	}
}

func TestApplyParamOverride_ReasoningEffort(t *testing.T) {
	effort := `{"reasoning_effort":"xhigh"}`

	// 客户端未设置 → 应被覆盖应用
	req := &transmodel.InternalLLMRequest{Model: "sensenova-6.8-flash-lite"}
	applyParamOverride(&appmodel.Channel{ID: 65, ParamOverride: &effort}, req)
	if req.ReasoningEffort != "xhigh" {
		t.Fatalf("ReasoningEffort = %q, want %q", req.ReasoningEffort, "xhigh")
	}

	// 客户端已设置 → 客户端优先，不得被覆盖
	clientReq := &transmodel.InternalLLMRequest{Model: "m", ReasoningEffort: "medium"}
	applyParamOverride(&appmodel.Channel{ID: 65, ParamOverride: &effort}, clientReq)
	if clientReq.ReasoningEffort != "medium" {
		t.Fatalf("ReasoningEffort = %q, want client value %q preserved", clientReq.ReasoningEffort, "medium")
	}

	// 非字符串值 → 静默丢弃，不 panic
	bad := `{"reasoning_effort":123}`
	numReq := &transmodel.InternalLLMRequest{Model: "m"}
	applyParamOverride(&appmodel.Channel{ID: 65, ParamOverride: &bad}, numReq)
	if numReq.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort = %q, want empty after non-string override", numReq.ReasoningEffort)
	}

	// 与既有字段同时存在，互不干扰
	both := `{"reasoning_effort":"low","temperature":0.2}`
	bothReq := &transmodel.InternalLLMRequest{Model: "m"}
	applyParamOverride(&appmodel.Channel{ID: 65, ParamOverride: &both}, bothReq)
	if bothReq.ReasoningEffort != "low" {
		t.Fatalf("ReasoningEffort = %q, want %q", bothReq.ReasoningEffort, "low")
	}
	if bothReq.Temperature == nil || *bothReq.Temperature != 0.2 {
		t.Fatalf("Temperature = %#v, want 0.2", bothReq.Temperature)
	}
}
