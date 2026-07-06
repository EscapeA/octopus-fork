package modelnormalize

import "testing"

func TestNormalizeWithRules_UsesBuiltinRules(t *testing.T) {
	rules := Rules{}
	tests := map[string]string{
		"kimi-k2.5":                "kimi-k2.5",
		"@cf/moonshotai/kimi-k2.5": "kimi-k2.5",
		"dmxapi-kimi-k2.5":         "kimi-k2.5",
		"moonshotai/kimi-k2.5":     "kimi-k2.5",
		"agent/kimi-k2.5":          "kimi-k2.5",
		"kimi-k2.5-cc":             "kimi-k2.5",
		"Kimi-K2.5-CC":             "kimi-k2.5",
		"kimi-k2.5-preview-fast":   "kimi-k2.5",
	}

	for input, want := range tests {
		if got := NormalizeWithRules(input, rules); got != want {
			t.Fatalf("NormalizeWithRules(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeWithRules_ExplicitMappingTakesPrecedence(t *testing.T) {
	rules := Rules{
		ExplicitMappings: []ExplicitMapping{{Variant: "dmxapi-kimi-k2.5-cc", Canonical: "kimi-k2.5-routing"}},
	}

	if got := NormalizeWithRules("DMXAPI-KIMI-K2.5-CC", rules); got != "kimi-k2.5-routing" {
		t.Fatalf("NormalizeWithRules explicit = %q, want kimi-k2.5-routing", got)
	}
}

func TestNormalizeWithRules_RuntimeRulesOverrideBuiltinPrefixesAndSuffixes(t *testing.T) {
	rules := Rules{
		RouterPrefixes:     []string{"custom-"},
		FunctionalSuffixes: []string{"-route"},
	}

	if got := NormalizeWithRules("dmxapi-kimi-k2.5-cc", rules); got != "dmxapi-kimi-k2.5-cc" {
		t.Fatalf("NormalizeWithRules with overridden rules = %q, want original lower-case", got)
	}
	if got := NormalizeWithRules("custom-kimi-k2.5-route", rules); got != "kimi-k2.5" {
		t.Fatalf("NormalizeWithRules custom = %q, want kimi-k2.5", got)
	}
}
