package handlers

import (
	"net/url"
	"strings"

	"github.com/lingyuins/octopus/internal/model"
)

const viewerMaskedDomain = "***"

func isViewerRole(role string) bool {
	return strings.TrimSpace(role) == model.UserRoleViewer
}

func maskURLDomainForViewer(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}

	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.User = nil
		parsed.Host = viewerMaskedDomain
		return parsed.String()
	}

	return viewerMaskedDomain
}

func redactChannelBaseURLsForViewer(channels []model.Channel) {
	for channelIndex := range channels {
		for baseURLIndex := range channels[channelIndex].BaseUrls {
			channels[channelIndex].BaseUrls[baseURLIndex].URL = maskURLDomainForViewer(channels[channelIndex].BaseUrls[baseURLIndex].URL)
		}
		// Mask custom proxy addresses (may contain credentials: socks5://user:pass@host)
		if channels[channelIndex].ChannelProxy != nil {
			masked := maskURLDomainForViewer(*channels[channelIndex].ChannelProxy)
			channels[channelIndex].ChannelProxy = &masked
		}
	}
}

func redactRemoteSiteBaseURLsForViewer(sites []model.RemoteSite) {
	for siteIndex := range sites {
		sites[siteIndex].BaseURL = maskURLDomainForViewer(sites[siteIndex].BaseURL)
	}
}

func redactCredentialBaseURLsForViewer(profiles []model.APICredentialProfile) {
	for profileIndex := range profiles {
		profiles[profileIndex].BaseURL = maskURLDomainForViewer(profiles[profileIndex].BaseURL)
	}
}

func redactSiteBaseURLsForViewer(sites []model.Site) {
	for siteIndex := range sites {
		sites[siteIndex].BaseURL = maskURLDomainForViewer(sites[siteIndex].BaseURL)
		// Mask site-level custom proxy addresses
		if sites[siteIndex].SiteProxy != nil {
			masked := maskURLDomainForViewer(*sites[siteIndex].SiteProxy)
			sites[siteIndex].SiteProxy = &masked
		}
		// Mask account-level custom proxy addresses (accounts are preloaded in list queries)
		for accountIndex := range sites[siteIndex].Accounts {
			if sites[siteIndex].Accounts[accountIndex].AccountProxy != nil {
				masked := maskURLDomainForViewer(*sites[siteIndex].Accounts[accountIndex].AccountProxy)
				sites[siteIndex].Accounts[accountIndex].AccountProxy = &masked
			}
		}
	}
}

func redactSettingsURLsForViewer(settings []model.Setting) {
	for settingIndex := range settings {
		switch settings[settingIndex].Key {
		case model.SettingKeyProxyURL,
			model.SettingKeyPublicAPIBaseURL,
			model.SettingKeySemanticCacheEmbeddingBaseURL,
			model.SettingKeyAIRouteBaseURL:
			settings[settingIndex].Value = maskURLDomainForViewer(settings[settingIndex].Value)
		}
	}
}
