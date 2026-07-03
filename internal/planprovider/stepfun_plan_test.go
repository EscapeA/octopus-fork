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

// --- SenseNova Plan 测试 ---

// makeTestSenseNovaToken 构造一个合法的 SenseNova Bearer JWT，
// payload 含 ext.tenant_id。内容不参与签名校验（测试用）。
func makeTestSenseNovaToken(t *testing.T, tenantID string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"client_id": "nova",
		"exp":       9999999999,
		"ext": map[string]any{
			"tenant_id":    tenantID,
			"principal_id": "test-principal",
			"username":     "testuser",
		},
	})
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	return "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9." + encodedPayload + ".sig"
}

func TestDecodeSenseNovaAccountID(t *testing.T) {
	token := makeTestSenseNovaToken(t, "tenant-abc-123")
	accountID := decodeSenseNovaAccountID(token)
	if accountID != "tenant-abc-123" {
		t.Errorf("decodeSenseNovaAccountID() = %q, want %q", accountID, "tenant-abc-123")
	}
}

func TestDecodeSenseNovaAccountID_EmptyToken(t *testing.T) {
	accountID := decodeSenseNovaAccountID("invalid")
	if accountID != "" {
		t.Errorf("decodeSenseNovaAccountID() = %q, want empty", accountID)
	}
}

func TestQuerySenseNovaPlanTokenPlan_Success(t *testing.T) {
	var gotAuth, gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")

		// 校验 URL 参数
		if !strings.Contains(r.URL.RawQuery, "account_id=test-tenant-id") {
			t.Errorf("URL missing account_id: %s", r.URL.RawQuery)
		}
		if !strings.Contains(r.URL.RawQuery, "model_ids=sensenova-6.7-flash-lite") {
			t.Errorf("URL missing model_ids: %s", r.URL.RawQuery)
		}

		resp := map[string]any{
			"model_remaining_percent": map[string]any{
				"deepseek-v4-flash":        100,
				"sensenova-6.7-flash-lite": 80,
				"sensenova-u1-fast":        50,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	origURL := senseNovaPlanURL
	senseNovaPlanURL = ts.URL
	defer func() { senseNovaPlanURL = origURL }()

	token := makeTestSenseNovaToken(t, "test-tenant-id")
	result, err := querySenseNovaPlanTokenPlan(context.Background(), token)
	if err != nil {
		t.Fatalf("querySenseNovaPlanTokenPlan() error = %v", err)
	}

	// 校验请求
	if gotMethod != http.MethodGet {
		t.Errorf("method = %s, want GET", gotMethod)
	}
	if !strings.Contains(gotAuth, "Bearer ") {
		t.Errorf("Authorization = %q, want Bearer", gotAuth)
	}

	// 校验解析结果
	if result.QuotaTotal != 100 {
		t.Errorf("QuotaTotal = %f, want 100", result.QuotaTotal)
	}
	// 最高已用 = 100-50 = 50
	if result.QuotaUsed != 50 {
		t.Errorf("QuotaUsed = %f, want 50", result.QuotaUsed)
	}
	// 校验模型明细
	if len(result.Models) != 3 {
		t.Fatalf("Models len = %d, want 3", len(result.Models))
	}
	for _, m := range result.Models {
		var wantUsed float64
		switch m.ModelName {
		case "deepseek-v4-flash":
			wantUsed = 0 // 100-100=0
		case "sensenova-6.7-flash-lite":
			wantUsed = 20 // 100-80=20
		case "sensenova-u1-fast":
			wantUsed = 50 // 100-50=50
		default:
			t.Errorf("unexpected model: %s", m.ModelName)
			continue
		}
		if m.QuotaUsed != wantUsed {
			t.Errorf("model %s: QuotaUsed = %f, want %f", m.ModelName, m.QuotaUsed, wantUsed)
		}
	}
}

func TestQuerySenseNovaPlanTokenPlan_EmptyToken(t *testing.T) {
	_, err := querySenseNovaPlanTokenPlan(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Errorf("error = %q, want 'token is required'", err.Error())
	}
}

func TestQuerySenseNovaPlanTokenPlan_InvalidToken(t *testing.T) {
	// Token 无法解码出 account_id
	_, err := querySenseNovaPlanTokenPlan(context.Background(), "invalid.token")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
	if !strings.Contains(err.Error(), "account_id") {
		t.Errorf("error = %q, want 'account_id'", err.Error())
	}
}
