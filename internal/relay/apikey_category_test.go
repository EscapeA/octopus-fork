package relay

import "testing"

func TestAPIKeyAllowsGroupCategory(t *testing.T) {
	tests := []struct {
		name     string
		allowed  string
		category string
		want     bool
	}{
		{name: "empty allowed permits uncategorized", allowed: "", category: "", want: true},
		{name: "empty allowed permits categorized", allowed: "", category: "premium", want: true},
		{name: "listed category permitted", allowed: "premium,budget", category: "premium", want: true},
		{name: "spaces are trimmed", allowed: " premium , budget ", category: " premium ", want: true},
		{name: "unlisted category rejected", allowed: "premium,budget", category: "free", want: false},
		{name: "non empty allowed rejects uncategorized", allowed: "premium,budget", category: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiKeyAllowsGroupCategory(tt.allowed, tt.category)
			if got != tt.want {
				t.Fatalf("apiKeyAllowsGroupCategory(%q, %q) = %v, want %v", tt.allowed, tt.category, got, tt.want)
			}
		})
	}
}
