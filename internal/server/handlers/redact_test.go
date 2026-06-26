package handlers

import (
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

func TestMaskURLDomainForViewer(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "https with path", raw: "https://api.example.com/v1", want: "https://***/v1"},
		{name: "host with port", raw: "http://127.0.0.1:3000/api", want: "http://***/api"},
		{name: "raw domain", raw: "api.example.com", want: "***"},
		{name: "empty", raw: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskURLDomainForViewer(tt.raw); got != tt.want {
				t.Fatalf("maskURLDomainForViewer(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestRedactChannelBaseURLsForViewer(t *testing.T) {
	proxy := "socks5://user:pass@proxy.example.com:1080"
	channels := []model.Channel{{
		BaseUrls:     []model.BaseUrl{{URL: "https://api.example.com/v1", Delay: 12}},
		ChannelProxy: &proxy,
	}}

	redactChannelBaseURLsForViewer(channels)

	if channels[0].BaseUrls[0].URL != "https://***/v1" {
		t.Fatalf("base url = %q, want masked", channels[0].BaseUrls[0].URL)
	}
	if channels[0].BaseUrls[0].Delay != 12 {
		t.Fatalf("delay = %d, want preserved", channels[0].BaseUrls[0].Delay)
	}
	if channels[0].ChannelProxy == nil || *channels[0].ChannelProxy != "socks5://***" {
		t.Fatalf("channel proxy = %v, want masked", channels[0].ChannelProxy)
	}
}

func TestRedactSiteProxyForViewer(t *testing.T) {
	siteProxy := "http://user:pass@proxy.example.com:8080"
	accountProxy := "socks5://10.0.0.1:1080"
	sites := []model.Site{{
		BaseURL:   "https://api.example.com",
		SiteProxy: &siteProxy,
		Accounts: []model.SiteAccount{{
			AccountProxy: &accountProxy,
		}},
	}}

	redactSiteBaseURLsForViewer(sites)

	if sites[0].SiteProxy == nil || *sites[0].SiteProxy != "http://***" {
		t.Fatalf("site proxy = %v, want masked", sites[0].SiteProxy)
	}
	if sites[0].Accounts[0].AccountProxy == nil || *sites[0].Accounts[0].AccountProxy != "socks5://***" {
		t.Fatalf("account proxy = %v, want masked", sites[0].Accounts[0].AccountProxy)
	}
}

func TestRedactSettingsURLsForViewer(t *testing.T) {
	settings := []model.Setting{
		{Key: model.SettingKeyPublicAPIBaseURL, Value: "https://octopus.example.com"},
		{Key: model.SettingKeySemanticCacheEmbeddingModel, Value: "text-embedding-3-small"},
	}

	redactSettingsURLsForViewer(settings)

	if settings[0].Value != "https://***" {
		t.Fatalf("public api base url = %q, want masked", settings[0].Value)
	}
	if settings[1].Value != "text-embedding-3-small" {
		t.Fatalf("non-url setting = %q, want unchanged", settings[1].Value)
	}
}
