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

	lowerTrimmed := strings.ToLower(trimmed)
	for _, mapping := range rules.ExplicitMappings {
		variant := strings.ToLower(strings.TrimSpace(mapping.Variant))
		canonical := strings.ToLower(strings.TrimSpace(mapping.Canonical))
		if variant != "" && canonical != "" && variant == lowerTrimmed {
			return canonical
		}
	}

	result := trimmed
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
