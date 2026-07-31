package modelnormalize

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

var builtinRouterPrefixes = []string{
	"dmxapi-",
	"agent-",
	"openai-",
	"anthropic-",
}

var builtinFunctionalSuffixes = []string{
	"-cc",
	"-fast",
	"-thinking",
	"-preview",
	"-beta",
	"-latest",
}

type ExplicitMapping struct {
	Variant   string `json:"variant"`
	Canonical string `json:"canonical"`
}

type Rules struct {
	RouterPrefixes     []string
	FunctionalSuffixes []string
	ExplicitMappings   []ExplicitMapping
}

var rulesCache struct {
	mu    sync.RWMutex
	gen   uint64
	rules Rules
	ready bool
}

func Normalize(name string) string {
	return NormalizeWithRules(name, CurrentRules())
}

func CurrentRules() Rules {
	gen := setting.Generation()
	rulesCache.mu.RLock()
	if rulesCache.ready && rulesCache.gen == gen {
		rules := rulesCache.rules
		rulesCache.mu.RUnlock()
		return rules
	}
	rulesCache.mu.RUnlock()

	rules := loadRules()
	rulesCache.mu.Lock()
	rulesCache.gen = gen
	rulesCache.rules = rules
	rulesCache.ready = true
	rulesCache.mu.Unlock()
	return rules
}

func NormalizeWithRules(name string, rules Rules) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	// 显式映射：匹配分三档——
	//  1. 精确全名（[官B]claude-opus-4-6-thinking 这类带渠道前缀的完整变体）；
	//  2. 输入剥离路径+路由前缀后的基础名（dmxapi-claude-opus-4-6 → claude-opus-4-6）；
	//  3. 映射 variant 也完整规范化（剥路径+前缀+后缀）后与输入规范化名比较，
	//     让 [官B]claude-opus-4-6-thinking 能命中 dmxapi-claude-opus-4-6-thinking
	//     （用户按前缀枚举映射，渠道侧却是任意前缀/无前缀，档 1/2 会漏）。
	lowerTrimmed := strings.ToLower(trimmed)
	base := strings.ToLower(stripPathAndRouterPrefix(trimmed, rules))
	normInput := normalizeToBase(trimmed, rules)
	for _, mapping := range rules.ExplicitMappings {
		variant := strings.ToLower(strings.TrimSpace(mapping.Variant))
		canonical := strings.ToLower(strings.TrimSpace(mapping.Canonical))
		if variant == "" || canonical == "" {
			continue
		}
		if variant == lowerTrimmed || variant == base {
			return canonical
		}
		if normVariant := normalizeToBase(mapping.Variant, rules); normVariant != "" && normVariant == normInput {
			return canonical
		}
	}

	result := stripPathAndRouterPrefix(trimmed, rules)

	for changed := true; changed; {
		changed = false
		currentLower := strings.ToLower(result)
		for _, suffix := range activeFunctionalSuffixes(rules) {
			suffix = strings.TrimSpace(suffix)
			if suffix == "" {
				continue
			}
			s := strings.ToLower(suffix)
			if strings.HasSuffix(currentLower, s) && len(result) > len(s) {
				result = result[:len(result)-len(s)]
				changed = true
				break
			}
		}
	}

	return strings.ToLower(result)
}

// normalizeToBase 完整规范化：剥路径 + 路由前缀 + 功能性后缀，返回小写基础名。
// 供显式映射的「规范化匹配」档使用（variant 与输入都规范化后再比较）。
func normalizeToBase(name string, rules Rules) string {
	result := stripPathAndRouterPrefix(name, rules)
	for changed := true; changed; {
		changed = false
		currentLower := strings.ToLower(result)
		for _, suffix := range activeFunctionalSuffixes(rules) {
			suffix = strings.TrimSpace(suffix)
			if suffix == "" {
				continue
			}
			s := strings.ToLower(suffix)
			if strings.HasSuffix(currentLower, s) && len(result) > len(s) {
				result = result[:len(result)-len(s)]
				changed = true
				break
			}
		}
	}
	return strings.ToLower(result)
}

// stripPathAndRouterPrefix 剥离路径前缀（最后一个 / 之后）与路由前缀，返回中间名。
// 显式映射的基础名匹配与主流程共用此剥离逻辑。
func stripPathAndRouterPrefix(name string, rules Rules) string {
	result := name
	if slashIndex := strings.LastIndex(result, "/"); slashIndex >= 0 {
		result = result[slashIndex+1:]
	}

	lower := strings.ToLower(result)
	for _, prefix := range activeRouterPrefixes(rules) {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			result = result[len(prefix):]
			break
		}
	}
	return result
}

func loadRules() Rules {
	return Rules{
		RouterPrefixes:     loadStringArray(model.SettingKeyModelNormalizeRouterPrefixes),
		FunctionalSuffixes: loadStringArray(model.SettingKeyModelNormalizeFunctionalSuffixes),
		ExplicitMappings:   loadExplicitMappings(model.SettingKeyModelNormalizeExplicitMappings),
	}
}

func activeRouterPrefixes(rules Rules) []string {
	if len(rules.RouterPrefixes) > 0 {
		return rules.RouterPrefixes
	}
	return builtinRouterPrefixes
}

func activeFunctionalSuffixes(rules Rules) []string {
	if len(rules.FunctionalSuffixes) > 0 {
		return rules.FunctionalSuffixes
	}
	return builtinFunctionalSuffixes
}

func loadStringArray(key model.SettingKey) []string {
	raw, err := setting.GetString(key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return compactStrings(values)
}

func loadExplicitMappings(key model.SettingKey) []ExplicitMapping {
	raw, err := setting.GetString(key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []ExplicitMapping
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	result := make([]ExplicitMapping, 0, len(values))
	for _, value := range values {
		variant := strings.TrimSpace(value.Variant)
		canonical := strings.TrimSpace(value.Canonical)
		if variant == "" || canonical == "" {
			continue
		}
		result = append(result, ExplicitMapping{Variant: variant, Canonical: canonical})
	}
	return result
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
