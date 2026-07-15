package planprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// makeTestCodexOAuthKey 构造合法的 Codex OAuth JSON 凭据（测试用）。
func makeTestCodexOAuthKey(accessToken, accountID string) string {
	b, _ := json.Marshal(map[string]any{
		"access_token": accessToken,
		"account_id":   accountID,
		"type":         "codex",
	})
	return string(b)
}

func TestParseCodexOAuthKey_Success(t *testing.T) {
	raw := makeTestCodexOAuthKey("tok-abc", "acct-123")
	key, err := parseCodexOAuthKey(raw)
	if err != nil {
		t.Fatalf("parseCodexOAuthKey() error = %v", err)
	}
	if key.AccessToken != "tok-abc" {
		t.Errorf("AccessToken = %q, want %q", key.AccessToken, "tok-abc")
	}
	if key.AccountID != "acct-123" {
		t.Errorf("AccountID = %q, want %q", key.AccountID, "acct-123")
	}
}

func TestParseCodexOAuthKey_Empty(t *testing.T) {
	if _, err := parseCodexOAuthKey(""); err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

func TestParseCodexOAuthKey_NotJSON(t *testing.T) {
	if _, err := parseCodexOAuthKey("sk-not-json"); err == nil {
		t.Fatal("expected error for non-JSON key, got nil")
	}
}

func TestQueryCodexTokenPlan_Success(t *testing.T) {
	var gotAuth, gotAccountID, gotOriginator string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccountID = r.Header.Get("chatgpt-account-id")
		gotOriginator = r.Header.Get("originator")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"plan_type": "pro",
			"rate_limit": map[string]any{
				"primary_window": map[string]any{
					"used_percent":         42.5,
					"reset_at":             1750000000,
					"limit_window_seconds": 604800,
				},
				"secondary_window": map[string]any{
					"used_percent":         10.0,
					"reset_at":             1750001800,
					"limit_window_seconds": 18000,
				},
			},
		})
	}))
	defer ts.Close()

	origURL := codexWhamUsageURL
	codexWhamUsageURL = ts.URL + "/backend-api/wham/usage"
	defer func() { codexWhamUsageURL = origURL }()

	keyJSON := makeTestCodexOAuthKey("tok-test", "acct-999")
	result, err := queryCodexTokenPlan(context.Background(), keyJSON)
	if err != nil {
		t.Fatalf("queryCodexTokenPlan() error = %v", err)
	}

	// 校验请求头
	if gotAuth != "Bearer tok-test" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tok-test")
	}
	if gotAccountID != "acct-999" {
		t.Errorf("chatgpt-account-id = %q, want %q", gotAccountID, "acct-999")
	}
	if gotOriginator != "codex_cli_rs" {
		t.Errorf("originator = %q, want %q", gotOriginator, "codex_cli_rs")
	}

	// 校验解析
	if result.QuotaTotal != 100 {
		t.Errorf("QuotaTotal = %v, want 100", result.QuotaTotal)
	}
	if result.QuotaUsed != 42.5 {
		t.Errorf("QuotaUsed = %v, want 42.5", result.QuotaUsed)
	}
	if result.QuotaResetAt == nil {
		t.Fatal("QuotaResetAt is nil")
	}
	if !result.QuotaResetAt.Equal(time.Unix(1750000000, 0)) {
		t.Errorf("QuotaResetAt = %v, want %v", result.QuotaResetAt, time.Unix(1750000000, 0))
	}

	// secondary -> weekly
	if result.WeeklyTotal != 100 {
		t.Errorf("WeeklyTotal = %v, want 100", result.WeeklyTotal)
	}
	if result.WeeklyUsed != 10.0 {
		t.Errorf("WeeklyUsed = %v, want 10.0", result.WeeklyUsed)
	}
	if result.WeeklyResetAt == nil {
		t.Fatal("WeeklyResetAt is nil")
	}
	if !result.WeeklyResetAt.Equal(time.Unix(1750001800, 0)) {
		t.Errorf("WeeklyResetAt = %v, want %v", result.WeeklyResetAt, time.Unix(1750001800, 0))
	}
}

func TestQueryCodexTokenPlan_AuthFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"detail": "invalid token",
		})
	}))
	defer ts.Close()

	origURL := codexWhamUsageURL
	codexWhamUsageURL = ts.URL + "/backend-api/wham/usage"
	defer func() { codexWhamUsageURL = origURL }()

	keyJSON := makeTestCodexOAuthKey("bad-token", "acct-1")
	_, err := queryCodexTokenPlan(context.Background(), keyJSON)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error should mention auth failure, got: %v", err)
	}
}

func TestQueryCodexTokenPlan_EmptyAccessToken(t *testing.T) {
	keyJSON := makeTestCodexOAuthKey("", "acct-1")
	_, err := queryCodexTokenPlan(context.Background(), keyJSON)
	if err == nil {
		t.Fatal("expected error for empty access_token, got nil")
	}
	if !strings.Contains(err.Error(), "access_token") {
		t.Errorf("error should mention access_token, got: %v", err)
	}
}

func TestQueryCodexTokenPlan_EmptyAccountID(t *testing.T) {
	keyJSON := makeTestCodexOAuthKey("tok-abc", "")
	_, err := queryCodexTokenPlan(context.Background(), keyJSON)
	if err == nil {
		t.Fatal("expected error for empty account_id, got nil")
	}
	if !strings.Contains(err.Error(), "account_id") {
		t.Errorf("error should mention account_id, got: %v", err)
	}
}

func TestQueryCodexTokenPlan_InvalidJSON(t *testing.T) {
	_, err := queryCodexTokenPlan(context.Background(), "not-json-at-all")
	if err == nil {
		t.Fatal("expected error for non-JSON key, got nil")
	}
}
