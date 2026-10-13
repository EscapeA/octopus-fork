package openai

import (
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func TestStripDSMLMarkers_FullWidthCalls(t *testing.T) {
	in := "\n\n<｜DSML｜ calls>\n"
	got := stripDSMLMarkers(in)
	if got != "\n\n\n" {
		t.Fatalf("stripDSMLMarkers(%q) = %q, want %q", in, got, "\n\n\n")
	}
}

func TestStripDSMLMarkers_AsciiCalls(t *testing.T) {
	in := "<|DSML| calls>"
	got := stripDSMLMarkers(in)
	if got != "" {
		t.Fatalf("stripDSMLMarkers(%q) = %q, want empty", in, got)
	}
}

func TestStripDSMLMarkers_NoMarkerUnchanged(t *testing.T) {
	in := "正常正文，没有标记"
	got := stripDSMLMarkers(in)
	if got != in {
		t.Fatalf("stripDSMLMarkers(%q) = %q, want unchanged", in, got)
	}
}

func TestStripDSMLMarkers_MixedContentKeepsText(t *testing.T) {
	in := "先说说思路\n\n<｜DSML｜ calls>\n然后调用工具"
	got := stripDSMLMarkers(in)
	want := "先说说思路\n\n\n然后调用工具"
	if got != want {
		t.Fatalf("stripDSMLMarkers(%q) = %q, want %q", in, got, want)
	}
}

func TestStripDSMLMarkers_ResultVariant(t *testing.T) {
	in := "\n\n<｜DSML｜ result>\n"
	got := stripDSMLMarkers(in)
	if got != "\n\n\n" {
		t.Fatalf("stripDSMLMarkers(%q) = %q, want %q", in, got, "\n\n\n")
	}
}

func TestStripToolCallDSMLMarker_OnlyOnToolCallTurn(t *testing.T) {
	// 非工具调用轮次（stop）不应剥离
	content := "\n\n<｜DSML｜ calls>\n"
	msg := &model.Message{
		Content: model.MessageContent{Content: &content},
	}
	stripToolCallDSMLMarker(msg, false)
	if *msg.Content.Content != content {
		t.Fatalf("non-tool-call turn must not strip, got %q", *msg.Content.Content)
	}

	// 工具调用轮次 → 剥离并置空
	msg2 := &model.Message{
		Content:   model.MessageContent{Content: &content},
		ToolCalls: []model.ToolCall{{ID: "call_1"}},
	}
	stripToolCallDSMLMarker(msg2, true)
	if msg2.Content.Content != nil {
		t.Fatalf("pure marker residue should be emptied, got %q", *msg2.Content.Content)
	}
}

func TestStripToolCallDSMLFromResponse_NonStream(t *testing.T) {
	content := "\n\n<｜DSML｜ calls>\n"
	tc := "tool_calls"
	resp := &model.InternalLLMResponse{
		Choices: []model.Choice{
			{
				FinishReason: &tc,
				Message: &model.Message{
					Content:   model.MessageContent{Content: &content},
					ToolCalls: []model.ToolCall{{ID: "call_1"}},
				},
			},
		},
	}
	stripToolCallDSMLFromResponse(resp)
	if resp.Choices[0].Message.Content.Content != nil {
		t.Fatalf("expected emptied content, got %q", *resp.Choices[0].Message.Content.Content)
	}
}

func TestStripToolCallDSMLFromResponse_Stream(t *testing.T) {
	content := "\n\n<｜DSML｜ calls>\n"
	tc := "tool_calls"
	resp := &model.InternalLLMResponse{
		Choices: []model.Choice{
			{
				FinishReason: &tc,
				Delta: &model.Message{
					Content:   model.MessageContent{Content: &content},
					ToolCalls: []model.ToolCall{{ID: "call_1"}},
				},
			},
		},
	}
	stripToolCallDSMLFromResponse(resp)
	if resp.Choices[0].Delta.Content.Content != nil {
		t.Fatalf("expected emptied delta content, got %q", *resp.Choices[0].Delta.Content.Content)
	}
}

func TestStripToolCallDSMLFromResponse_StopTurnKeepsContent(t *testing.T) {
	content := "正常的回答，含 <｜DSML｜ 这种普通文本>"
	stop := "stop"
	resp := &model.InternalLLMResponse{
		Choices: []model.Choice{
			{
				FinishReason: &stop,
				Message: &model.Message{
					Content: model.MessageContent{Content: &content},
				},
			},
		},
	}
	stripToolCallDSMLFromResponse(resp)
	if *resp.Choices[0].Message.Content.Content != content {
		t.Fatalf("stop turn must not strip, got %q", *resp.Choices[0].Message.Content.Content)
	}
}
