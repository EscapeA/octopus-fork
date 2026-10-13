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
// 不识别 DSML；因此在本层（OpenAI Chat outbound 响应转换）统一剥离。
//
// 只在工具调用轮次剥离：finish_reason == "tool_calls" 时 NV 才会输出该标记，
// 普通正文（stop）里出现的 <｜DSML｜ xxx> 不是工具标记，不应误伤。
var (
	// dsmlFullWidthMarkerRE 匹配 <｜DSML｜ calls> 这类完整标记（允许变体名，如 calls/result/think）。
	// 全角竖线 U+FF5C 直接以字面量写入正则（Go regexp 不支持 \u 转义）。
	dsmlFullWidthMarkerRE = regexp.MustCompile(`<｜DSML｜[^>]*>`)
	// dsmlAsciiMarkerRE 兼容 ASCII 竖线写法 <|DSML| calls>（部分网关可能归一化全角）。
	dsmlAsciiMarkerRE = regexp.MustCompile(`<\|DSML\|[^>]*>`)
)

// stripDSMLMarkers 从文本中剥离 DSML 工具调用标记。
// 同时处理全角（｜）与 ASCII（|）竖线两种变体；对普通正文无匹配时原样返回。
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

// stripToolCallDSMLMarker 清理 assistant 消息正文里的 DSML 标记。
// 当消息带 tool_calls 且 finish_reason 为 tool_calls 时（NV 工具调用轮次），
// content 里只应是标记残留；剥离标记后若剩余纯空白，则置空 content。
func stripToolCallDSMLMarker(msg *model.Message, isToolCallTurn bool) {
	if msg == nil || !isToolCallTurn || len(msg.ToolCalls) == 0 {
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
	// MultipleContent 里的 text part 同样剥离
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

// stripToolCallDSMLFromResponse 遍历响应中的 choices，剥离工具调用轮次的
// DSML 标记。同时覆盖非流式（Message）与流式（Delta）。
func stripToolCallDSMLFromResponse(resp *model.InternalLLMResponse) {
	if resp == nil {
		return
	}
	for i := range resp.Choices {
		choice := &resp.Choices[i]
		isToolCallTurn := choice.FinishReason != nil && *choice.FinishReason == "tool_calls"
		stripToolCallDSMLMarker(choice.Message, isToolCallTurn)
		stripToolCallDSMLMarker(choice.Delta, isToolCallTurn)
	}
}
