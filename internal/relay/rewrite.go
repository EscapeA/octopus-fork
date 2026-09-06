package relay

import (
	"fmt"
	"strings"

	appmodel "github.com/lingyuins/octopus/internal/model"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/rewrite"
	"github.com/lingyuins/octopus/internal/utils/log"
)

func prepareInternalRequestForOutbound(channel *appmodel.Channel, request *transmodel.InternalLLMRequest, groupEndpointType string) (*transmodel.InternalLLMRequest, *rewrite.EffectiveConfig, error) {
	if channel == nil {
		return nil, nil, fmt.Errorf("channel is nil")
	}
	if request == nil {
		return nil, nil, fmt.Errorf("request is nil")
	}

	effectiveRewrite, enabled, err := rewrite.Resolve(channel.Type, channel.RequestRewrite)
	if err != nil {
		return nil, nil, err
	}

	var target *transmodel.InternalLLMRequest
	if !enabled {
		target = request
	} else {
		rewritten, applyErr := rewrite.Apply(request, effectiveRewrite)
		if applyErr != nil {
			return nil, nil, applyErr
		}
		target = rewritten
	}

	applyParamOverride(channel, target)
	attachRelayGroupEndpointMetadata(target, groupEndpointType)
	return target, effectiveRewrite, nil
}

// paramOverrideForceWhitelist 强制覆盖字段白名单：仅这些字段在渠道开启
// ParamOverrideForce 时无视客户端取值强制覆盖；其余字段始终客户端优先。
// 应用分支见 applyParamOverrideForce 的 switch（字段类型各异，需逐字段处理）。
var paramOverrideForceWhitelist = []string{"reasoning_effort"}

// applyParamOverride merges channel-level param_override JSON into the outbound request.
// Only overrides fields that are not already set by the client request (client takes precedence),
// except whitelist fields (paramOverrideForceWhitelist) when the channel enables
// ParamOverrideForce — those are overridden unconditionally.
// An empty/invalid override value is always ignored and never clears a client-set field.
func applyParamOverride(channel *appmodel.Channel, request *transmodel.InternalLLMRequest) {
	if channel == nil || channel.ParamOverride == nil || *channel.ParamOverride == "" {
		return
	}
	if request == nil {
		return
	}

	var overrides map[string]RawMessage
	if err := jsonAPI.Unmarshal([]byte(*channel.ParamOverride), &overrides); err != nil {
		log.Warnf("param_override: invalid JSON for channel %d: %v", channel.ID, err)
		return
	}

	if channel.ParamOverrideForce {
		applyParamOverrideForce(overrides, request)
	}

	if v, ok := overrides["max_tokens"]; ok && request.MaxTokens == nil {
		var val int64
		if err := jsonAPI.Unmarshal(v, &val); err == nil {
			request.MaxTokens = &val
		}
	}
	if v, ok := overrides["max_completion_tokens"]; ok && request.MaxCompletionTokens == nil {
		var val int64
		if err := jsonAPI.Unmarshal(v, &val); err == nil {
			request.MaxCompletionTokens = &val
		}
	}
	if v, ok := overrides["temperature"]; ok && request.Temperature == nil {
		var val float64
		if err := jsonAPI.Unmarshal(v, &val); err == nil {
			request.Temperature = &val
		}
	}
	if v, ok := overrides["top_p"]; ok && request.TopP == nil {
		var val float64
		if err := jsonAPI.Unmarshal(v, &val); err == nil {
			request.TopP = &val
		}
	}
	// reasoning_effort：常规注入（客户端没传才填），与既有字段同语义。
	// 强制覆盖已在 applyParamOverrideForce 中处理，此处仅在字段为空时兜底注入。
	if v, ok := overrides["reasoning_effort"]; ok && strings.TrimSpace(request.ReasoningEffort) == "" {
		var val string
		if err := jsonAPI.Unmarshal(v, &val); err == nil && strings.TrimSpace(val) != "" {
			request.ReasoningEffort = strings.TrimSpace(val)
		}
	}
}

// applyParamOverrideForce 应用强制覆盖：仅白名单字段，无视客户端取值直接写入。
// 覆盖值类型不符或为空字符串时忽略该项，不清空客户端字段（强制语义只支持
// 「换成配置值」，不支持「强制移除」）。新增白名单字段时在此 switch 补充分支。
func applyParamOverrideForce(overrides map[string]RawMessage, request *transmodel.InternalLLMRequest) {
	for _, field := range paramOverrideForceWhitelist {
		v, ok := overrides[field]
		if !ok {
			continue
		}
		switch field {
		case "reasoning_effort":
			var val string
			if err := jsonAPI.Unmarshal(v, &val); err == nil && strings.TrimSpace(val) != "" {
				request.ReasoningEffort = strings.TrimSpace(val)
			}
		}
	}
}

func attachRelayGroupEndpointMetadata(request *transmodel.InternalLLMRequest, groupEndpointType string) {
	if request == nil {
		return
	}

	normalizedEndpointType := appmodel.NormalizeEndpointType(groupEndpointType)
	if normalizedEndpointType == "" {
		return
	}

	if request.TransformerMetadata == nil {
		request.TransformerMetadata = make(map[string]string)
	}
	request.TransformerMetadata[transmodel.TransformerMetadataGroupEndpointType] = normalizedEndpointType
}
