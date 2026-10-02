package relay

import (
	"strings"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/inbound"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

// relayEndpointFamily 把「渠道端点类型 + 入站格式」归一成参与「同文并发合并」的端点族标识。
// 只有 chat/completions 与 responses 两种入站格式参与合并，其余（如 embeddings、图片等）返回空串。
func relayEndpointFamily(endpointType string, inboundType inbound.InboundType) string {
	switch {
	case endpointType == dbmodel.EndpointTypeChat && inboundType == inbound.InboundTypeOpenAIChat:
		return "chat"
	case endpointType == dbmodel.EndpointTypeResponses && inboundType == inbound.InboundTypeOpenAIResponse:
		return "responses"
	default:
		return ""
	}
}

// extractRequestText 提取请求的可比较文本（逐条 "role: 内容" 归一化后换行拼接），
// 作为「同文并发合并」的请求指纹。无可比文本（空消息列表、纯图像/音频/工具调用）时返回 false，
// 此类请求不参与合并。
func extractRequestText(req *transmodel.InternalLLMRequest) (string, bool) {
	if req == nil || len(req.Messages) == 0 {
		return "", false
	}

	lines := make([]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		text := normalizeRequestText(extractMessageText(msg.Content))
		if text == "" {
			continue
		}

		role := normalizeRequestText(msg.Role)
		if role == "" {
			role = "message"
		}
		lines = append(lines, role+": "+text)
	}

	joined := strings.TrimSpace(strings.Join(lines, "\n"))
	if joined == "" {
		return "", false
	}

	return joined, true
}

func extractMessageText(content transmodel.MessageContent) string {
	textParts := make([]string, 0, 1+len(content.MultipleContent))
	if content.Content != nil {
		textParts = append(textParts, *content.Content)
	}
	for _, part := range content.MultipleContent {
		if !strings.EqualFold(strings.TrimSpace(part.Type), "text") || part.Text == nil {
			continue
		}
		textParts = append(textParts, *part.Text)
	}
	return strings.Join(textParts, " ")
}

func normalizeRequestText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
