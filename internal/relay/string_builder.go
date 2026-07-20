package relay

import (
	"strconv"
	"strings"
)

// buildChannelName 构建 channel_N 格式的 channel name
func buildChannelName(channelID int) string {
	var b strings.Builder
	b.Grow(16) // "channel_" + 数字
	b.WriteString("channel_")
	b.WriteString(strconv.Itoa(channelID))
	return b.String()
}

// buildErrorMessage 拼接两段错误消息: msg + ": " + detail
func buildErrorMessage(msg, detail string) string {
	if detail == "" {
		return msg
	}
	var b strings.Builder
	b.Grow(len(msg) + len(detail) + 2)
	b.WriteString(msg)
	b.WriteString(": ")
	b.WriteString(detail)
	return b.String()
}

// buildAttemptMessage 构建 "attempt N/M: msg" 格式消息
func buildAttemptMessage(tryIndex, tryTotal int, msg string) string {
	var b strings.Builder
	// "attempt " + 数字 + "/" + 数字 + ": " + msg
	b.Grow(8 + 10 + 10 + len(msg))
	b.WriteString("attempt ")
	b.WriteString(strconv.Itoa(tryIndex))
	b.WriteByte('/')
	b.WriteString(strconv.Itoa(tryTotal))
	b.WriteString(": ")
	b.WriteString(msg)
	return b.String()
}

// buildFailureHintKey 构建 "channelID:keyID:modelName" 格式 key
func buildFailureHintKey(channelID, keyID int, modelName string) string {
	var b strings.Builder
	b.Grow(20 + len(modelName)) // 数字 + ":" + 数字 + ":" + modelName
	b.WriteString(strconv.Itoa(channelID))
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(keyID))
	b.WriteByte(':')
	b.WriteString(strings.TrimSpace(modelName))
	return b.String()
}

// buildSemanticCacheKey 构建 "apiKeyID|endpointFamily|requestModel|text|false" 格式 key
func buildSemanticCacheKey(apiKeyID int, endpointFamily, requestModel, text string) string {
	var b strings.Builder
	// 预估: 数字 + "|" + family + "|" + model + "|" + text + "|false"
	b.Grow(20 + len(endpointFamily) + len(requestModel) + len(text) + 10)
	b.WriteString(strconv.Itoa(apiKeyID))
	b.WriteByte('|')
	b.WriteString(endpointFamily)
	b.WriteByte('|')
	b.WriteString(requestModel)
	b.WriteByte('|')
	b.WriteString(text)
	b.WriteString("|false")
	return b.String()
}
