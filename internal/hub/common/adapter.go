// Package common implements the SiteAdapter for the One API / New API family.
// Most relay sites are API-compatible with New API, so this adapter serves as
// the default fallback registered under model.SiteTypeNewAPI.
package common

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
)

// Adapter implements hub.SiteAdapter for the New API / One API family.
type Adapter struct{}

func init() {
	hub.Register(model.SiteTypeNewAPI, &Adapter{})
	hub.Register(model.SiteTypeUnknown, &Adapter{})
}

// ── User / Account ──────────────────────────────────────────────────────────

type userSelfResponse struct {
	ID          int     `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Email       string  `json:"email"`
	Role        int     `json:"role"`
	Status      int     `json:"status"`
	Quota       float64 `json:"quota"`
	UsedQuota   float64 `json:"used_quota"`
	AccessToken string  `json:"access_token"`
}

func (a *Adapter) FetchUserInfo(ctx context.Context, site *model.RemoteSite) (*hub.UserInfoResult, error) {
	u, err := hub.FetchJSON[userSelfResponse](ctx, site, http.MethodGet, "/api/user/self", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}
	return &hub.UserInfoResult{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Role:        u.Role,
		Status:      u.Status,
		Quota:       u.Quota,
		UsedQuota:   u.UsedQuota,
		AccessToken: u.AccessToken,
	}, nil
}

// ── Check-in ────────────────────────────────────────────────────────────────

type checkInStatusResponse struct {
	Stats struct {
		CheckedInToday bool `json:"checked_in_today"`
	} `json:"stats"`
}

func (a *Adapter) FetchCheckInStatus(ctx context.Context, site *model.RemoteSite) (*bool, error) {
	month := time.Now().Format("2006-01")
	endpoint := fmt.Sprintf("/api/user/checkin?month=%s", month)
	status, err := hub.FetchJSON[checkInStatusResponse](ctx, site, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil // site may not support check-in
	}
	available := !status.Stats.CheckedInToday
	return &available, nil
}

type checkInResponse struct {
	Message string  `json:"message"`
	Quota   float64 `json:"quota"`
}

func (a *Adapter) PerformCheckIn(ctx context.Context, site *model.RemoteSite) (*hub.CheckInResult, error) {
	resp, err := hub.FetchJSON[checkInResponse](ctx, site, http.MethodPost, "/api/user/checkin", nil)
	if err != nil {
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "已") {
			return &hub.CheckInResult{Success: true, AlreadyDone: true, Message: err.Error()}, nil
		}
		return &hub.CheckInResult{Success: false, Message: err.Error()}, nil
	}
	return &hub.CheckInResult{
		Success:      true,
		Message:      resp.Message,
		QuotaAwarded: resp.Quota,
	}, nil
}

// ── Models ──────────────────────────────────────────────────────────────────

func (a *Adapter) FetchModels(ctx context.Context, site *model.RemoteSite) ([]string, error) {
	type modelEntry struct {
		ID string `json:"id"`
	}
	// OpenAI-compatible /v1/models
	type modelsResponse struct {
		Data []modelEntry `json:"data"`
	}

	resp, err := hub.FetchJSON[modelsResponse](ctx, site, http.MethodGet, "/v1/models", nil)
	if err != nil {
		// Fallback: some sites return a flat array
		models, err2 := hub.FetchJSON[[]string](ctx, site, http.MethodGet, "/v1/models", nil)
		if err2 != nil {
			return nil, fmt.Errorf("fetch models: %w (fallback: %w)", err, err2)
		}
		return models, nil
	}
	names := make([]string, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.ID != "" {
			names = append(names, m.ID)
		}
	}
	return names, nil
}

type pricingEntry struct {
	ModelName       string  `json:"model_name"`
	Quota           float64 `json:"quota"`
	CompletionRatio float64 `json:"completion_ratio"`
	GroupRatio      float64 `json:"group_ratio"`
}

func (a *Adapter) FetchModelPricing(ctx context.Context, site *model.RemoteSite) ([]hub.ModelPricingEntry, error) {
	entries, err := hub.FetchJSON[[]pricingEntry](ctx, site, http.MethodGet, "/api/pricing", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch model pricing: %w", err)
	}
	result := make([]hub.ModelPricingEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, hub.ModelPricingEntry{
			ModelName:       e.ModelName,
			Quota:           e.Quota,
			CompletionRatio: e.CompletionRatio,
			GroupRatio:      e.GroupRatio,
		})
	}
	return result, nil
}

// ── Tokens ──────────────────────────────────────────────────────────────────

type tokenResponse struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Key            string  `json:"key"`
	Status         int     `json:"status"`
	RemainQuota    float64 `json:"remain_quota"`
	UsedQuota      float64 `json:"used_quota"`
	UnlimitedQuota bool    `json:"unlimited_quota"`
	ModelLimits    string  `json:"model_limits_enabled"`
	ExpiredTime    int64   `json:"expired_time"`
	CreatedTime    int64   `json:"created_time"`
}

type tokenListResponse struct {
	Items []tokenResponse `json:"items"`
	Total int             `json:"total"`
}

func (a *Adapter) FetchTokens(ctx context.Context, site *model.RemoteSite) ([]hub.RemoteToken, error) {
	resp, err := hub.FetchJSON[tokenListResponse](ctx, site, http.MethodGet, "/api/token/?p=0&page_size=100", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch tokens: %w", err)
	}
	tokens := make([]hub.RemoteToken, 0, len(resp.Items))
	for _, t := range resp.Items {
		tokens = append(tokens, hub.RemoteToken{
			ID:             t.ID,
			Name:           t.Name,
			Key:            t.Key,
			Status:         t.Status,
			RemainQuota:    t.RemainQuota,
			UsedQuota:      t.UsedQuota,
			UnlimitedQuota: t.UnlimitedQuota,
			ModelLimits:    t.ModelLimits,
			ExpiredTime:    t.ExpiredTime,
			CreatedTime:    t.CreatedTime,
		})
	}
	return tokens, nil
}

func (a *Adapter) CreateToken(ctx context.Context, site *model.RemoteSite, req hub.CreateTokenRequest) error {
	_, err := hub.FetchJSON[interface{}](ctx, site, http.MethodPost, "/api/token/", req)
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}
	return nil
}

// ── Channels ────────────────────────────────────────────────────────────────

type channelResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    int    `json:"type"`
	Status  int    `json:"status"`
	Models  string `json:"models"`
	BaseURL string `json:"base_url"`
	Group   string `json:"group"`
}

type channelListResponse struct {
	Items []channelResponse `json:"items"`
	Total int               `json:"total"`
}

func (a *Adapter) ListChannels(ctx context.Context, site *model.RemoteSite) ([]hub.RemoteChannel, error) {
	var allChannels []hub.RemoteChannel
	page := 0
	pageSize := 100
	for {
		endpoint := fmt.Sprintf("/api/channel/?p=%d&page_size=%d", page, pageSize)
		resp, err := hub.FetchJSON[channelListResponse](ctx, site, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("list channels page %d: %w", page, err)
		}
		for _, ch := range resp.Items {
			allChannels = append(allChannels, hub.RemoteChannel{
				ID:      ch.ID,
				Name:    ch.Name,
				Type:    ch.Type,
				Status:  ch.Status,
				Models:  ch.Models,
				BaseURL: ch.BaseURL,
				Group:   ch.Group,
			})
		}
		if len(allChannels) >= resp.Total || len(resp.Items) < pageSize {
			break
		}
		page++
	}
	return allChannels, nil
}

func (a *Adapter) CreateChannel(ctx context.Context, site *model.RemoteSite, ch hub.RemoteChannelCreateReq) error {
	_, err := hub.FetchJSON[interface{}](ctx, site, http.MethodPost, "/api/channel/", ch)
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}
	return nil
}

func (a *Adapter) UpdateChannel(ctx context.Context, site *model.RemoteSite, ch hub.RemoteChannelUpdateReq) error {
	_, err := hub.FetchJSON[interface{}](ctx, site, http.MethodPut, "/api/channel/", ch)
	if err != nil {
		return fmt.Errorf("update channel: %w", err)
	}
	return nil
}

func (a *Adapter) DeleteChannel(ctx context.Context, site *model.RemoteSite, channelID int) error {
	endpoint := fmt.Sprintf("/api/channel/%d", channelID)
	_, err := hub.FetchJSON[interface{}](ctx, site, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	return nil
}

// ── Announcements ───────────────────────────────────────────────────────────

func (a *Adapter) FetchAnnouncement(ctx context.Context, site *model.RemoteSite) (string, error) {
	// /api/notice is a public endpoint on most New API sites
	noAuthSite := *site
	noAuthSite.AuthType = model.AuthTypeNone
	noAuthSite.AccessToken = ""

	notice, err := hub.FetchJSON[string](ctx, &noAuthSite, http.MethodGet, "/api/notice", nil)
	if err != nil {
		return "", nil // not critical
	}
	return strings.TrimSpace(notice), nil
}

// ── Site Status ─────────────────────────────────────────────────────────────

type siteStatusResponse struct {
	CheckInEnabled bool    `json:"checkin_enabled"`
	Price          float64 `json:"price"`
	SystemName     string  `json:"system_name"`
}

func (a *Adapter) FetchSiteStatus(ctx context.Context, site *model.RemoteSite) (*hub.SiteStatusInfo, error) {
	noAuthSite := *site
	noAuthSite.AuthType = model.AuthTypeNone
	noAuthSite.AccessToken = ""

	status, err := hub.FetchJSON[siteStatusResponse](ctx, &noAuthSite, http.MethodGet, "/api/status", nil)
	if err != nil {
		return nil, nil
	}
	return &hub.SiteStatusInfo{
		CheckInEnabled: status.CheckInEnabled,
		Price:          status.Price,
		SystemName:     status.SystemName,
	}, nil
}
