package openai

import (
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

func strp(s string) *string { return &s }

func TestStripDSMLMarkers_FullWidthCalls(t *testing.T) {
	in := "\n\n<｜DSML｜ calls>\n"
	if got, want := stripDSMLMarkers(in), "\n\n\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStripDSMLMarkers_AsciiCalls(t *testing.T) {
	if got := stripDSMLMarkers("<|DSML| calls>"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestStripDSMLMarkers_NoMarkerUnchanged(t *testing.T) {
	in := "正常正文，没有标记"
	if got := stripDSMLMarkers(in); got != in {
		t.Fatalf("got %q, want unchanged", got)
	}
}

func TestStripDSMLMarkers_MixedContentKeepsText(t *testing.T) {
	in := "先说说思路\n\n<｜DSML｜ calls>\n然后调用工具"
	want := "先说说思路\n\n\n然后调用工具"
	if got := stripDSMLMarkers(in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// —— 非流式：只在 tool-call 轮次剥离 ——

func TestStripToolCallDSMLFromResponse_NonStreamToolCallTurn(t *testing.T) {
	tc := "tool_calls"
	resp := &model.InternalLLMResponse{Choices: []model.Choice{{
		FinishReason: &tc,
		Message: &model.Message{
			Content:   model.MessageContent{Content: strp("\n\n<｜DSML｜ calls>\n")},
			ToolCalls: []model.ToolCall{{ID: "call_1"}},
		},
	}}}
	stripToolCallDSMLFromResponse(resp)
	if resp.Choices[0].Message.Content.Content != nil {
		t.Fatalf("expected emptied content, got %q", *resp.Choices[0].Message.Content.Content)
	}
}

func TestStripToolCallDSMLFromResponse_StopTurnKeepsContent(t *testing.T) {
	stop := "stop"
	content := "正常回答，含 <｜DSML｜ 这种普通文本>"
	resp := &model.InternalLLMResponse{Choices: []model.Choice{{
		FinishReason: &stop,
		Message:      &model.Message{Content: model.MessageContent{Content: &content}},
	}}}
	stripToolCallDSMLFromResponse(resp)
	if *resp.Choices[0].Message.Content.Content != content {
		t.Fatalf("stop turn must not strip, got %q", *resp.Choices[0].Message.Content.Content)
	}
}

// —— 流式：NV 实测分片（标记被切成两半、finish_reason 在末 chunk）——

func TestDSMLStreamStripper_NVSplitChunksDroppedOnToolCalls(t *testing.T) {
	s := &dsmlStreamStripper{}
	// chunk A: "\n\n<｜DSML｜ calls" —— 前缀被切断，应扣留、不放出
	a := s.process("\n\n<｜DSML｜ calls")
	if a != "\n\n" {
		t.Fatalf("chunk A = %q, want %q", a, "\n\n")
	}
	// chunk B: ">\n" —— 拼出完整标记，只删标记本身，后续换行保留
	b := s.process(">\n")
	if b != "\n" {
		t.Fatalf("chunk B = %q, want %q", b, "\n")
	}
	// 合并结果应等于非流式 stripDSMLMarkers 的结果，且不含标记
	total := a + b
	if strings.Contains(total, "DSML") || total != "\n\n\n" {
		t.Fatalf("total = %q, want %q", total, "\n\n\n")
	}
	// tool_calls 出现 → 扣留内容清空（此时已无扣留）
	s.noteToolCalls()
	if s.held != "" || s.pending != "" {
		t.Fatalf("held=%q pending=%q, want both empty", s.held, s.pending)
	}
}

func TestDSMLStreamStripper_ReleasesOnStopTurn(t *testing.T) {
	s := &dsmlStreamStripper{}
	// 模型真在正文里写 "<｜DSML｜ 引用纪律>"，随后以 stop 收尾 → 必须放回
	if got := s.process("正文提到 <｜DSML｜ 引用纪律>"); got != "正文提到 " {
		t.Fatalf("process = %q, want %q", got, "正文提到 ")
	}
	released := s.release()
	if released != "<｜DSML｜ 引用纪律>" {
		t.Fatalf("released = %q, want %q", released, "<｜DSML｜ 引用纪律>")
	}
}

func TestDSMLStreamStripper_ToolCallsDropsHeldResidue(t *testing.T) {
	s := &dsmlStreamStripper{}
	s.process("\n\n<｜DSML｜ calls>") // 完整标记，扣留
	if s.held == "" {
		t.Fatal("expected marker held before tool_calls verdict")
	}
	s.noteToolCalls()
	if s.held != "" {
		t.Fatalf("held = %q, want empty after tool_calls", s.held)
	}
}

func TestDSMLStreamStripper_MarkerArrivingAfterToolCallsIsDropped(t *testing.T) {
	s := &dsmlStreamStripper{}
	s.noteToolCalls() // 先见到 tool_calls
	got := s.process("\n\n<｜DSML｜ calls>\n")
	if strings.Contains(got, "DSML") {
		t.Fatalf("marker must be dropped after tool_calls, got %q", got)
	}
	if got != "\n\n\n" {
		t.Fatalf("got %q, want %q", got, "\n\n\n")
	}
}

func TestDSMLStreamStripper_LoneLessThanNotHeld(t *testing.T) {
	s := &dsmlStreamStripper{}
	if got := s.process("a < b"); got != "a < b" {
		t.Fatalf("lone '<' must pass through, got %q", got)
	}
}

func TestDSMLStreamStripper_PlainTextUnaffected(t *testing.T) {
	s := &dsmlStreamStripper{}
	if got := s.process("普通一段流式文本"); got != "普通一段流式文本" {
		t.Fatalf("got %q", got)
	}
}

func TestDSMLStreamStripper_SplitAtLessThan(t *testing.T) {
	// 线上泄漏根因（2026-10-13）：NV 把标记切在首字符之后
	s := &dsmlStreamStripper{}
	a := s.process("\n\n<")
	if a != "\n\n" {
		t.Fatalf("chunk A = %q, want %q", a, "\n\n")
	}
	b := s.process("\uff5cDSML\uff5c calls")
	if b != "" {
		t.Fatalf("chunk B = %q, want empty", b)
	}
	c := s.process(">\n")
	if strings.Contains(c, "DSML") {
		t.Fatalf("chunk C leaks marker: %q", c)
	}
	if got := a + b + c; got != "\n\n\n" {
		t.Fatalf("concat = %q, want %q", got, "\n\n\n")
	}
}

func TestDSMLStreamStripper_LoneLessThanAtChunkEndIsRecovered(t *testing.T) {
	// 正文里的孤立 '<' 若恰好在 chunk 末尾被扣留，下一 chunk 证明非标记时必须放行
	s := &dsmlStreamStripper{}
	if got := s.process("a <"); got != "a " {
		t.Fatalf("chunk A = %q, want %q", got, "a ")
	}
	if got := s.process(" b"); got != "< b" {
		t.Fatalf("chunk B = %q, want %q", got, "< b")
	}
}

func TestDSMLStreamStripper_StopTurnKeepsHeldOrdering(t *testing.T) {
	// 正文里的 '<' 恰在 chunk 末尾被扣留，下一 chunk 证明非标记 → 按原顺序复原
	s := &dsmlStreamStripper{}
	if got := s.process("前言 <"); got != "前言 " {
		t.Fatalf("chunk A = %q, want %q", got, "前言 ")
	}
	stop := "stop"
	resp := &model.InternalLLMResponse{Choices: []model.Choice{{
		FinishReason: &stop,
		Delta:        &model.Message{Content: model.MessageContent{Content: strp(" 收尾")}},
	}}}
	s.stripResponse(resp)
	if got := *resp.Choices[0].Delta.Content.Content; got != "< 收尾" {
		t.Fatalf("got %q, want %q", got, "< 收尾")
	}
}

func TestDSMLStreamStripper_StopTurnAppendsHeldToExistingContent(t *testing.T) {
	// 完整标记被扣留后，末条 stop chunk 自带正文：放回时必须追加而非覆盖
	s := &dsmlStreamStripper{}
	if got := s.process("\n\n<｜DSML｜ calls>"); got != "\n\n" {
		t.Fatalf("marker chunk = %q, want %q", got, "\n\n")
	}
	if s.held == "" {
		t.Fatal("expected marker held")
	}
	stop := "stop"
	resp := &model.InternalLLMResponse{Choices: []model.Choice{{
		FinishReason: &stop,
		Delta:        &model.Message{Content: model.MessageContent{Content: strp("结论")}},
	}}}
	s.stripResponse(resp)
	got := *resp.Choices[0].Delta.Content.Content
	if !strings.Contains(got, "结论") {
		t.Fatalf("existing content lost: %q", got)
	}
	if !strings.Contains(got, "DSML") {
		t.Fatalf("held marker not released on stop: %q", got)
	}
}

func TestDSMLStreamStripper_Reset(t *testing.T) {
	s := &dsmlStreamStripper{}
	s.process("\n\n<｜DSML｜ calls")
	s.noteToolCalls()
	s.reset()
	if s.held != "" || s.pending != "" || s.sawToolCalls {
		t.Fatalf("reset failed: %+v", s)
	}
}

// —— 端到端：模拟 ChatOutbound.TransformStream 的 chunk 序列 ——

func TestDsmlStreamStripResponse_NVSequenceEndToEnd(t *testing.T) {
	s := &dsmlStreamStripper{}
	chunks := []string{"\n\n<｜DSML｜ calls", ">\n"}
	for _, c := range chunks {
		resp := &model.InternalLLMResponse{Choices: []model.Choice{{
			Delta: &model.Message{Content: model.MessageContent{Content: strp(c)}},
		}}}
		s.stripResponse(resp)
	}
	// tool_calls chunk
	tcResp := &model.InternalLLMResponse{Choices: []model.Choice{{
		Delta: &model.Message{ToolCalls: []model.ToolCall{{ID: "call_1"}}},
	}}}
	s.stripResponse(tcResp)
	if s.held != "" || s.pending != "" {
		t.Fatalf("residue not dropped: held=%q pending=%q", s.held, s.pending)
	}
}
