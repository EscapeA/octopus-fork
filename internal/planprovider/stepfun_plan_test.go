package planprovider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// makeTestOasisToken 构造一个合法的 Oasis-Token 格式（access...refresh），
// refresh payload 含指定 device_id。内容不参与签名校验（测试用）。
func makeTestOasisToken(t *testing.T, deviceID string) string {
	t.Helper()
	access := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY3RpdmF0ZWQiOnRydWUsImV4cCI6OTk5OTk5OTk5OSwib2FzaXNfaWQiOjF9.sig"
	payload, _ := json.Marshal(map[string]any{
		"app_id":    10300,
		"device_id": deviceID,
		"exp":       9999999999,
		"platform":  "web",
	})
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	refresh := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." + encodedPayload + ".sig"
	return access + "..." + refresh
}

func TestDecodeStepFunWebID(t *testing.T) {
	token := makeTestOasisToken(t, "abc123device456")
	webid := decodeStepFunWebID(token)
	if webid != "abc123device456" {
		t.Errorf("decodeStepFunWebID() = %q, want %q", webid, "abc123device456")
	}
}

func TestDecodeStepFunWebID_NoSeparator(t *testing.T) {
	webid := decodeStepFunWebID("justanaccesstoken")
	if webid != "" {
		t.Errorf("decodeStepFunWebID() = %q, want empty", webid)
	}
}

func TestQueryStepFunPlanTokenPlan_Success(t *testing.T) {
	var gotCookie, gotAppID, gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCookie = r.Header.Get("Cookie")
		gotAppID = r.Header.Get("oasis-appid")

		if r.Header.Get("Connect-Protocol-Version") != "1" {
			t.Errorf("missing Connect-Protocol-Version header")
		}
		if r.Header.Get("oasis-platform") != "web" {
			t.Errorf("oasis-platform = %q, want web", r.Header.Get("oasis-platform"))
		}

		resp := map[string]any{
			"status":                     1,
			"desc":                       "",
			"five_hour_usage_left_rate":  0,
			"five_hour_usage_reset_time": "0",
			"weekly_usage_left_rate":     0,
			"weekly_usage_reset_time":    "0",
			"plan_family":                2,
			"plan_credit_rate_limit": map[string]any{
				"subscription_credit_left_rate":  0.9964648,
				"subscription_credit_reset_time": "1784379705",
				"topup_credit_left_rate":         0,
				"credit_buckets": []map[string]any{
					{
						"type":            1,
						"credit_total":    "400000000",
						"credit_residual": "398585926",
						"expire_at":       "1785675705",
						"next_reset_at":   "1784379705",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// 覆盖硬编码 URL 指向 mock server
	origURL := stepFunPlanURL
	stepFunPlanURL = ts.URL
	defer func() { stepFunPlanURL = origURL }()

	token := makeTestOasisToken(t, "test-device-id")
	result, err := queryStepFunPlanTokenPlan(context.Background(), token)
	if err != nil {
		t.Fatalf("queryStepFunPlanTokenPlan() error = %v", err)
	}

	// 校验请求头
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.Contains(gotCookie, "Oasis-Token=") {
		t.Errorf("cookie missing Oasis-Token: %s", gotCookie)
	}
	if !strings.Contains(gotCookie, "Oasis-Webid=test-device-id") {
		t.Errorf("cookie missing Oasis-Webid: %s", gotCookie)
	}
	if gotAppID != "10300" {
		t.Errorf("oasis-appid = %q, want 10300", gotAppID)
	}

	// 校验解析结果
	if result.QuotaTotal != 400000000 {
		t.Errorf("QuotaTotal = %f, want 400000000", result.QuotaTotal)
	}
	// used = total - residual = 400000000 - 398585926 = 1414074
	if result.QuotaUsed != 1414074 {
		t.Errorf("QuotaUsed = %f, want 1414074", result.QuotaUsed)
	}
	if result.QuotaResetAt == nil {
		t.Error("QuotaResetAt is nil")
	} else {
		// 1784379705 = 2026-07-18 21:01:45 UTC
		want := time.Unix(1784379705, 0)
		if !result.QuotaResetAt.Equal(want) {
			t.Errorf("QuotaResetAt = %v, want %v", result.QuotaResetAt, want)
		}
	}
}

func TestQueryStepFunPlanTokenPlan_AuthFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"code":    "unauthenticated",
			"message": "auth failed: oasis-token is embezzled",
		})
	}))
	defer ts.Close()

	origURL := stepFunPlanURL
	stepFunPlanURL = ts.URL
	defer func() { stepFunPlanURL = origURL }()

	token := makeTestOasisToken(t, "device")
	_, err := queryStepFunPlanTokenPlan(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "鉴权失败") {
		t.Errorf("error = %q, want '鉴权失败'", err.Error())
	}
}

func TestQueryStepFunPlanTokenPlan_EmptyToken(t *testing.T) {
	_, err := queryStepFunPlanTokenPlan(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "oasis token is required") {
		t.Errorf("error = %q, want 'oasis token is required'", err.Error())
	}
}
