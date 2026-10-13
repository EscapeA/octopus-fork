package openai

import (
	"regexp"
	"strings"

	"github.com/lingyuins/octopus/internal/transformer/model"
)

// DSML（DeepSeek Model Language）工具调用标记。
//
// NV 上的 DeepSeek V4 在发起工具调用时，会在 assistant 消息 content 里残留
// DSML 标记（全角竖线 U+FF5C 包裹），例如：
//
//	\n\n<｜DSML｜ calls>\n
//
// 而结构化的 tool_calls 数组已经正常返回。该标记对客户端是噪声，会污染
// assistant 正文并进入后续历史继续回显。hermes 侧只有 <tool_call> 标签体系，
// 不识别 DSML，因此在本层（OpenAI Chat outbound 响应转换）统一剥离。
//
// ⚠️ 流式实测（NV 原始 SSE，2026-10-13）：
//
//	chunk A: content='\n\n<｜DSML｜ calls'   ← 标记被切开
//	chunk B: content='>\n'
//	chunk C..: tool_calls 分片
//	chunk Z: finish_reason='tool_calls'（无 content）
//
// 标记先于 tool_calls / finish_reason 到达，且会被切成两半 —— 因此
// 「逐 chunk 看 finish_reason」必然失效。流式必须：
//  1. 跨 chunk 状态机，把被切断的标记前缀留到下一 chunk 拼接；
//  2. 先扣留（hold）标记文本，等出现结构化 tool_calls 再丢弃（判定为残留）；
//  3. 若该轮以 finish_reason="stop" 收尾（正常文本轮次），把扣留文本放回，
//     绝不误删模型真正写在正文里的内容。
var (
	// dsmlFullWidthMarkerRE 匹配 <｜DSML｜ calls> 这类完整标记（允许变体名，如 calls/result/think）。
	// 全角竖线 U+FF5C 直接以字面量写入正则（Go regexp 不支持 \u 转义）。
	dsmlFullWidthMarkerRE = regexp.MustCompile(`<｜DSML｜[^>]*>`)
	// dsmlAsciiMarkerRE 兼容 ASCII 竖线写法 <|DSML| calls>（部分网关可能归一化全角）。
	dsmlAsciiMarkerRE = regexp.MustCompile(`<\|DSML\|[^>]*>`)
)

// dsmlFullMarkerStart / dsmlAsciiMarkerStart 是标记的共同开头（含竖线）。
var (
	dsmlFullMarkerStart  = "<\uff5cDSML\uff5c"
	dsmlAsciiMarkerStart = "<|DSML|"
)

// stripDSMLMarkers 从完整文本中剥离 DSML 标记（非流式 / 已聚合文本用）。
func stripDSMLMarkers(text string) string {
	if text == "" {
		return text
	}
	stripped := dsmlFullWidthMarkerRE.ReplaceAllString(text, "")
	if stripped == text {
		stripped = dsmlAsciiMarkerRE.ReplaceAllString(text, "")
	}
	return stripped
}

// dsmlMarkerAt 判断 s 的开头是否是一个完整 DSML 标记；是则返回其长度（字节）。
func dsmlMarkerAt(s string) int {
	if s == "" || s[0] != '<' {
		return 0
	}
	if loc := dsmlFullWidthMarkerRE.FindStringIndex(s); loc != nil && loc[0] == 0 {
		return loc[1]
	}
	if loc := dsmlAsciiMarkerRE.FindStringIndex(s); loc != nil && loc[0] == 0 {
		return loc[1]
	}
	return 0
}

// isPartialDSMLPrefix 判断 s（从 '<' 开始）是否是「尚未闭合的标记前缀」，
// 需要留到下一 chunk 再判定。
//
// ⚠️ 必须覆盖「本 chunk 恰好以孤立 '<' 结尾」的情形：NV 实测会把标记切在
// 首字符之后（chunk A = "\n\n<"、chunk B = "｜DSML｜ calls>\n"）。若此处
// 因 rune 数不足而放行 '<'，下一个 chunk 不含 '<'，标记就会被原样拼进正文
// （2026-10-13 线上泄漏 1/35 的根因）。
//
// 只要 remainder 以 '<' 开头且尚未出现 '>'，且与标记开头相容，就扣留；
// 若下一 chunk 证明它不是标记（普通文本如 "< b"），会在下一轮立即放行。
func isPartialDSMLPrefix(s string) bool {
	if s == "" || s[0] != '<' {
		return false
	}
	if strings.Contains(s, ">") {
		return false // 已出现 '>'，是完整标记或不含标记的普通文本
	}
	// s 与标记开头必须相容：或 s 是标记开头的前缀，或 s 以标记开头起头。
	return strings.HasPrefix(dsmlFullMarkerStart, s) ||
		strings.HasPrefix(s, dsmlFullMarkerStart) ||
		strings.HasPrefix(dsmlAsciiMarkerStart, s) ||
		strings.HasPrefix(s, dsmlAsciiMarkerStart)
}

// dsmlStreamStripper 是流式 delta 的跨 chunk DSML 剥离状态机。
// 同一请求的多个 chunk 必须复用同一实例（挂在 ChatOutbound 上）。
type dsmlStreamStripper struct {
	pending      string // 被切断的标记前缀（本 chunk 不足以判定）
	held         string // 已扣留的标记文本，等 tool_calls 判定
	sawToolCalls bool   // 本轮流是否出现结构化 tool_calls
}

// process 处理一个 delta content 片段，返回「当前可放行」的文本。
// 完整标记与被切断的前缀会被扣留（返回时扣掉）。
func (s *dsmlStreamStripper) process(text string) string {
	if s == nil {
		return text
	}
	if text == "" && s.pending == "" {
		return text
	}
	buf := s.pending + text
	s.pending = ""

	var out strings.Builder
	i := 0
	for i < len(buf) {
		if buf[i] == '<' {
			if n := dsmlMarkerAt(buf[i:]); n > 0 {
				// 完整标记：出现 tool_calls 则直接丢弃，否则扣留待判定。
				if !s.sawToolCalls {
					s.held += buf[i : i+n]
				}
				i += n
				continue
			}
			if isPartialDSMLPrefix(buf[i:]) {
				// 前缀被切断：留到下一 chunk 拼接再判定（无论是否已见 tool_calls，
				// 都保留 remainder 供下一轮处理，避免吞掉同 chunk 的后续文本）。
				s.pending = buf[i:]
				return out.String()
			}
		}
		out.WriteByte(buf[i])
		i++
	}
	return out.String()
}

// noteToolCalls 标记「本轮流出现结构化 tool_calls」，并丢弃扣留的标记残留。
func (s *dsmlStreamStripper) noteToolCalls() {
	if s == nil {
		return
	}
	s.sawToolCalls = true
	s.held = ""
	s.pending = ""
}

// release 放回扣留文本（用于 finish_reason="stop" 的正常文本轮次），并返回它。
func (s *dsmlStreamStripper) release() string {
	if s == nil {
		return ""
	}
	released := s.held + s.pending
	s.held = ""
	s.pending = ""
	return released
}

// reset 清空状态（[DONE] 时调用，避免实例跨请求残留）。
func (s *dsmlStreamStripper) reset() {
	if s == nil {
		return
	}
	s.pending = ""
	s.held = ""
	s.sawToolCalls = false
}

// stripToolCallDSMLFromMessage 剥离单条消息里的 DSML 标记（非流式路径，完整文本）。
func stripToolCallDSMLFromMessage(msg *model.Message) {
	if msg == nil {
		return
	}
	if msg.Content.Content != nil {
		stripped := stripDSMLMarkers(*msg.Content.Content)
		if stripped != *msg.Content.Content {
			if strings.TrimSpace(stripped) == "" {
				msg.Content = model.MessageContent{} // 纯标记残留 → 置空
			} else {
				msg.Content.Content = &stripped
			}
		}
	}
	if len(msg.Content.MultipleContent) > 0 {
		parts := make([]model.MessageContentPart, 0, len(msg.Content.MultipleContent))
		for _, part := range msg.Content.MultipleContent {
			if part.Type == "text" && part.Text != nil {
				stripped := stripDSMLMarkers(*part.Text)
				if strings.TrimSpace(stripped) == "" {
					continue // 纯标记残留的 text part 丢弃
				}
				part.Text = &stripped
			}
			parts = append(parts, part)
		}
		msg.Content.MultipleContent = parts
	}
}

// dsmlResponseHasToolCalls 判断响应是否携带结构化 tool_calls（流式 delta 或非流式 message）。
func dsmlResponseHasToolCalls(resp *model.InternalLLMResponse) bool {
	if resp == nil {
		return false
	}
	for _, choice := range resp.Choices {
		if choice.Message != nil && len(choice.Message.ToolCalls) > 0 {
			return true
		}
		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			return true
		}
	}
	return false
}

// stripToolCallDSMLFromResponse 遍历非流式响应，剥离工具调用轮次的 DSML 标记。
// 仅在带 tool_calls 或 finish_reason="tool_calls" 时剥离，普通文本轮次不动。
func stripToolCallDSMLFromResponse(resp *model.InternalLLMResponse) {
	if resp == nil {
		return
	}
	for i := range resp.Choices {
		choice := &resp.Choices[i]
		isToolCallTurn := len(choice.Message.ToolCalls) > 0 ||
			(choice.FinishReason != nil && *choice.FinishReason == "tool_calls")
		if !isToolCallTurn {
			continue
		}
		stripToolCallDSMLFromMessage(choice.Message)
	}
}

// stripToolCallDSMLFromStream 处理流式响应 chunk（跨 chunk 状态机）。
// 见 dsmlStreamStripper 的说明。
func (s *dsmlStreamStripper) stripResponse(resp *model.InternalLLMResponse) {
	if s == nil || resp == nil {
		return
	}
	if dsmlResponseHasToolCalls(resp) {
		s.noteToolCalls()
	}
	for i := range resp.Choices {
		choice := &resp.Choices[i]
		for _, msg := range []*model.Message{choice.Message, choice.Delta} {
			if msg == nil || msg.Content.Content == nil {
				continue
			}
			out := s.process(*msg.Content.Content)
			if out == "" {
				msg.Content = model.MessageContent{}
			} else {
				msg.Content.Content = &out
			}
		}
		// 正常文本轮次收尾：放回扣留文本，绝不误删正文。
		if choice.FinishReason != nil && *choice.FinishReason == "stop" {
			if released := s.release(); released != "" {
				target := choice.Delta
				if target == nil {
					target = choice.Message
				}
				if target != nil {
					// 追加而不是覆盖：末条 chunk 自带 content 时不能丢。
					if target.Content.Content != nil {
						released = *target.Content.Content + released
					}
					target.Content.Content = &released
				}
			}
		}
	}
}
