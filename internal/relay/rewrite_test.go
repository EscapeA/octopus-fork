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

	rewritten, _, err := prepareInternalRequestForOutbound(rewriteChannel, baseRequest, &appmodel.Group{EndpointType: appmodel.EndpointTypeDeepSeek})
	if err != nil {
		t.Fatalf("prepareInternalRequestForOutbound() rewrite channel error = %v", err)
	}
	plain, _, err := prepareInternalRequestForOutbound(plainChannel, baseRequest, &appmodel.Group{EndpointType: appmodel.EndpointTypeChat})
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

func TestApplyParamOverride_NilOrEmptyConfig(t *testing.T) {
	base := &transmodel.InternalLLMRequest{Model: "m"}
	// 未配置 param_override / 渠道为 nil / 请求为 nil 均不应 panic 或改动请求。
	applyParamOverride(nil, base)
	applyParamOverride(&appmodel.Channel{ID: 1}, base)
	applyParamOverride(&appmodel.Channel{ID: 1, ParamOverride: ptrString("")}, base)
	if base.Temperature != nil {
		t.Fatalf("empty config mutated request: %+v", base)
	}
	applyParamOverride(&appmodel.Channel{ID: 1, ParamOverride: ptrString("{}")}, nil)
}

func TestApplyParamOverride_ClientPrecedence(t *testing.T) {
	clientMax := int64(100)
	clientTemp := 0.7
	request := &transmodel.InternalLLMRequest{
		Model:       "m",
		MaxTokens:   &clientMax,
		Temperature: &clientTemp,
	}
	channel := &appmodel.Channel{
		ID:            1,
		ParamOverride: ptrString(`{"max_tokens": 999, "temperature": 1.5, "top_p": 0.9}`),
	}
	applyParamOverride(channel, request)

	if *request.MaxTokens != 100 || *request.Temperature != 0.7 {
		t.Fatalf("client values overwritten: max_tokens=%d temperature=%v", *request.MaxTokens, *request.Temperature)
	}
	if request.TopP == nil || *request.TopP != 0.9 {
		t.Fatalf("top_p should be injected when absent, got %v", request.TopP)
	}
}

func TestApplyParamOverride_InvalidValueTypeIgnored(t *testing.T) {
	request := &transmodel.InternalLLMRequest{Model: "m"}
	channel := &appmodel.Channel{
		ID:            1,
		ParamOverride: ptrString(`{"reasoning_effort": 123, "max_tokens": "abc"}`),
	}
	applyParamOverride(channel, request)
	if request.ReasoningEffort != "" || request.MaxTokens != nil {
		t.Fatalf("invalid value types must be ignored: effort=%q max=%v", request.ReasoningEffort, request.MaxTokens)
	}
}

func TestApplyParamOverride_InvalidJSONNoop(t *testing.T) {
	request := &transmodel.InternalLLMRequest{Model: "m", ReasoningEffort: "medium"}
	channel := &appmodel.Channel{
		ID:            1,
		ParamOverride: ptrString(`not-json`),
	}
	applyParamOverride(channel, request)
	if request.ReasoningEffort != "medium" {
		t.Fatalf("invalid JSON must not touch request, got %q", request.ReasoningEffort)
	}
}

func ptrString(s string) *string {
	return &s
}
