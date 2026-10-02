package relay

import (
	"testing"

	appmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

func TestExtractRequestText_ChatMessagesOnlyUsesText(t *testing.T) {
	userText := "hello"
	req := &transmodel.InternalLLMRequest{
		Model: "gpt-4.1",
		Messages: []transmodel.Message{
			{
				Role: "user",
				Content: transmodel.MessageContent{
					Content: &userText,
				},
			},
		},
	}

	text, ok := extractRequestText(req)
	if !ok {
		t.Fatal("expected request with text to be comparable")
	}
	if text != "user: hello" {
		t.Fatalf("text = %q", text)
	}
}

func TestExtractRequestText_IgnoresNonTextPartsAndNormalizesWhitespace(t *testing.T) {
	text := "  hello\n\nworld\t "
	req := &transmodel.InternalLLMRequest{
		Model: "gpt-4.1",
		Messages: []transmodel.Message{
			{
				Role: "user",
				Content: transmodel.MessageContent{
					MultipleContent: []transmodel.MessageContentPart{
						{Type: "text", Text: &text},
						{Type: "image_url", ImageURL: &transmodel.ImageURL{URL: "https://example.com/image.png"}},
						{Type: "input_audio", Audio: &transmodel.Audio{Format: "mp3", Data: "abc"}},
					},
				},
			},
		},
	}

	normalized, ok := extractRequestText(req)
	if !ok {
		t.Fatal("expected request with text part to be comparable")
	}
	if normalized != "user: hello world" {
		t.Fatalf("normalized = %q", normalized)
	}
}

func TestExtractRequestText_RejectsRequestsWithoutStableText(t *testing.T) {
	req := &transmodel.InternalLLMRequest{
		Model: "gpt-4.1",
		Messages: []transmodel.Message{
			{
				Role: "user",
				Content: transmodel.MessageContent{
					MultipleContent: []transmodel.MessageContentPart{
						{Type: "image_url", ImageURL: &transmodel.ImageURL{URL: "https://example.com/image.png"}},
					},
				},
			},
		},
	}

	if _, ok := extractRequestText(req); ok {
		t.Fatal("expected non-text-only request to be non-comparable")
	}
	if _, ok := extractRequestText(&transmodel.InternalLLMRequest{Model: "gpt-4.1"}); ok {
		t.Fatal("expected request without messages to be non-comparable")
	}
	if _, ok := extractRequestText(nil); ok {
		t.Fatal("expected nil request to be non-comparable")
	}
}

func TestRelayEndpointFamily_UsesHandlerInputs(t *testing.T) {
	if got := relayEndpointFamily(appmodel.EndpointTypeChat, inbound.InboundTypeOpenAIChat); got != "chat" {
		t.Fatalf("chat family = %q", got)
	}
	if got := relayEndpointFamily(appmodel.EndpointTypeResponses, inbound.InboundTypeOpenAIResponse); got != "responses" {
		t.Fatalf("responses family = %q", got)
	}
	if got := relayEndpointFamily(appmodel.EndpointTypeMessages, inbound.InboundTypeAnthropic); got != "" {
		t.Fatalf("anthropic family = %q, want empty", got)
	}
	if got := relayEndpointFamily(appmodel.EndpointTypeEmbeddings, inbound.InboundTypeOpenAIEmbedding); got != "" {
		t.Fatalf("embedding family = %q, want empty", got)
	}
}

func TestRequestSingleflightKey_RequiresComparableRequest(t *testing.T) {
	nonStream := false
	userText := "hello"

	req := &transmodel.InternalLLMRequest{
		Model:  "gpt-4.1",
		Stream: &nonStream,
		Messages: []transmodel.Message{{
			Role:    "user",
			Content: transmodel.MessageContent{Content: &userText},
		}},
	}
	key, ok := requestSingleflightKey(7, "chat", "gpt-4.1", "user: hello", req)
	if !ok || key != "7|chat|gpt-4.1|user: hello|false" {
		t.Fatalf("key = %q, ok = %v", key, ok)
	}

	// 流式请求不参与合并（无法共享响应体）。
	stream := true
	req.Stream = &stream
	if _, ok := requestSingleflightKey(7, "chat", "gpt-4.1", "user: hello", req); ok {
		t.Fatal("expected streaming request to skip inflight dedupe")
	}

	// 带工具调用的请求不参与合并。
	req.Stream = &nonStream
	req.Tools = []transmodel.Tool{{Type: "function"}}
	if _, ok := requestSingleflightKey(7, "chat", "gpt-4.1", "user: hello", req); ok {
		t.Fatal("expected tool-calling request to skip inflight dedupe")
	}

	// 缺少可比文本 / apiKey 时不合并。
	req.Tools = nil
	if _, ok := requestSingleflightKey(7, "chat", "gpt-4.1", "  ", req); ok {
		t.Fatal("expected blank request text to skip inflight dedupe")
	}
	if _, ok := requestSingleflightKey(0, "chat", "gpt-4.1", "user: hello", req); ok {
		t.Fatal("expected missing api key to skip inflight dedupe")
	}
}
