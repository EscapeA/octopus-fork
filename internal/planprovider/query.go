package planprovider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

const requestTimeout = 15 * time.Second

// BalanceResult 余额查询结果
type BalanceResult struct {
	Balance     float64 `json:"balance"`
	BalanceUsed float64 `json:"balance_used"`
	Currency    string  `json:"currency"`
}

// TokenPlanResult TokenPlan 查询结果
type TokenPlanResult struct {
	QuotaTotal    float64    `json:"quota_total"`
	QuotaUsed     float64    `json:"quota_used"`
	QuotaResetAt  *time.Time `json:"quota_reset_at"`
	WeeklyTotal   float64    `json:"weekly_total"`
	WeeklyUsed    float64    `json:"weekly_used"`
	WeeklyResetAt *time.Time `json:"weekly_reset_at"`
	// 各模型明细
	Models []TokenPlanModelUsage `json:"models,omitempty"`
}

// TokenPlanModelUsage 单个模型用量
type TokenPlanModelUsage struct {
	ModelName  string  `json:"model_name"`
	QuotaTotal float64 `json:"quota_total"`
	QuotaUsed  float64 `json:"quota_used"`
}

// QueryBalance 查询余额（额度类厂商）
func QueryBalance(ctx context.Context, category model.PlanProviderCategory, apiKey string, baseURL string) (*BalanceResult, error) {
	switch category {
	case model.PlanProviderDeepSeek:
		return queryDeepSeekBalance(ctx, apiKey)
	case model.PlanProviderKimi:
		return queryKimiBalance(ctx, apiKey)
	case model.PlanProviderSiliconFlow:
		return querySiliconFlowBalance(ctx, apiKey)
	case model.PlanProviderOpenRouter:
		return queryOpenRouterBalance(ctx, apiKey)
	case model.PlanProviderStepFun:
		return queryStepFunBalance(ctx, apiKey)
	case model.PlanProvider302AI:
		return query302AIBalance(ctx, apiKey)
	case model.PlanProviderNovita:
		return queryNovitaBalance(ctx, apiKey)
	case model.PlanProviderOpenAI:
		return queryOpenAIBalance(ctx, apiKey)
	default:
		return nil, fmt.Errorf("unsupported balance provider: %s", category)
	}
}

// QueryTokenPlan 查询套餐用量（TokenPlan 类厂商）
func QueryTokenPlan(ctx context.Context, category model.PlanProviderCategory, apiKey string, baseURL string) (*TokenPlanResult, error) {
	switch category {
	case model.PlanProviderMiniMax:
		return queryMiniMaxTokenPlan(ctx, apiKey)
	case model.PlanProviderSenseNovaPlan:
		return querySenseNovaPlanTokenPlan(ctx, apiKey)
	case model.PlanProviderStepFunPlan:
		return queryStepFunPlanTokenPlan(ctx, apiKey)
	case model.PlanProviderMiMoPlan:
		return queryMiMoPlanTokenPlan(ctx, apiKey)
	case model.PlanProviderZhipu:
		return queryZhipuTokenPlan(ctx, apiKey)
	default:
		return nil, fmt.Errorf("unsupported tokenplan provider: %s", category)
	}
}

// --- DeepSeek ---

func queryDeepSeekBalance(ctx context.Context, apiKey string) (*BalanceResult, error) {
	body, err := doGet(ctx, "https://api.deepseek.com/user/balance", apiKey)
	if err != nil {
		return nil, err
	}

	var resp struct {
		IsAvailable  bool `json:"is_available"`
		BalanceInfos []struct {
			Currency        string `json:"currency"`
			TotalBalance    string `json:"total_balance"`
			GrantedBalance  string `json:"granted_balance"`
			ToppedUpBalance string `json:"topped_up_balance"`
		} `json:"balance_infos"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("deepseek: parse response: %w", err)
	}

	var total float64
	for _, info := range resp.BalanceInfos {
		if v := parseFloat(info.TotalBalance); v > 0 {
			total += v
		}
	}
	return &BalanceResult{Balance: total, BalanceUsed: 0, Currency: "CNY"}, nil
}

// --- Kimi ---

func queryKimiBalance(ctx context.Context, apiKey string) (*BalanceResult, error) {
	body, err := doGet(ctx, "https://api.moonshot.cn/v1/users/me/balance", apiKey)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			AvailableBalance float64 `json:"available_balance"`
			VoucherBalance   float64 `json:"voucher_balance"`
			CashBalance      float64 `json:"cash_balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kimi: parse response: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("kimi: API error code=%d", resp.Code)
	}

	return &BalanceResult{
		Balance:     resp.Data.AvailableBalance,
		BalanceUsed: 0,
		Currency:    "CNY",
	}, nil
}

// --- SiliconFlow ---

func querySiliconFlowBalance(ctx context.Context, apiKey string) (*BalanceResult, error) {
	body, err := doGet(ctx, "https://api.siliconflow.com/v1/user/info", apiKey)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int  `json:"code"`
		Ok   bool `json:"ok"`
		Data struct {
			Balance       string `json:"balance"`
			ChargeBalance string `json:"chargeBalance"`
			TotalBalance  string `json:"totalBalance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("siliconflow: parse response: %w", err)
	}
	if resp.Code != 20000 && !resp.Ok {
		return nil, fmt.Errorf("siliconflow: API error code=%d", resp.Code)
	}

	balance := parseFloat(resp.Data.TotalBalance)
	if balance == 0 {
		balance = parseFloat(resp.Data.Balance)
	}
	return &BalanceResult{Balance: balance, BalanceUsed: 0, Currency: "CNY"}, nil
}

// --- OpenRouter ---

func queryOpenRouterBalance(ctx context.Context, apiKey string) (*BalanceResult, error) {
	body, err := doGet(ctx, "https://openrouter.ai/api/v1/credits", apiKey)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			TotalCredits float64 `json:"total_credits"`
			TotalUsage   float64 `json:"total_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openrouter: parse response: %w", err)
	}

	return &BalanceResult{
		Balance:     resp.Data.TotalCredits - resp.Data.TotalUsage,
		BalanceUsed: resp.Data.TotalUsage,
		Currency:    "USD",
	}, nil
}

// --- MiniMax Token Plan ---

func queryMiniMaxTokenPlan(ctx context.Context, apiKey string) (*TokenPlanResult, error) {
	body, err := doGet(ctx, "https://www.minimaxi.com/v1/api/openplatform/coding_plan/remains", apiKey)
	if err != nil {
		return nil, err
	}

	var resp struct {
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
		StatusCode   int    `json:"status_code"`
		StatusMsg    string `json:"status_msg"`
		ModelRemains []struct {
			ModelName                 string  `json:"model_name"`
			StartTime                 int64   `json:"start_time"`
			EndTime                   int64   `json:"end_time"`
			RemainsTime               int64   `json:"remains_time"`
			CurrentIntervalTotalCount float64 `json:"current_interval_total_count"`
			CurrentIntervalUsageCount float64 `json:"current_interval_usage_count"`
			CurrentWeeklyTotalCount   float64 `json:"current_weekly_total_count"`
			CurrentWeeklyUsageCount   float64 `json:"current_weekly_usage_count"`
			WeeklyRemainsTime         int64   `json:"weekly_remains_time"`
		} `json:"model_remains"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("minimax: parse response: %w", err)
	}

	statusCode := resp.StatusCode
	if statusCode == 0 {
		statusCode = resp.BaseResp.StatusCode
	}
	if statusCode != 0 {
		statusMsg := resp.StatusMsg
		if statusMsg == "" {
			statusMsg = resp.BaseResp.StatusMsg
		}
		return nil, fmt.Errorf("minimax: API error code=%d msg=%s", statusCode, statusMsg)
	}

	result := &TokenPlanResult{}
	var models []TokenPlanModelUsage
	for _, m := range resp.ModelRemains {
		total := m.CurrentIntervalTotalCount
		usage := m.CurrentIntervalUsageCount
		// MiniMax 字段语义：total=该计费区间总额度，usage=已使用额度。
		// QuotaUsed 必须是"已使用"而非"剩余"（total-usage），否则前端用量百分比会被
		// 颠倒成剩余率（issue #126 修复项）。
		models = append(models, TokenPlanModelUsage{
			ModelName:  m.ModelName,
			QuotaTotal: total,
			QuotaUsed:  max(0, usage),
		})

		// 以第一个模型的数据作为汇总
		if result.QuotaTotal == 0 {
			result.QuotaTotal = total
			result.QuotaUsed = max(0, usage)
			if m.RemainsTime > 0 {
				t := time.Now().Add(time.Duration(m.RemainsTime) * time.Millisecond)
				result.QuotaResetAt = &t
			}
			if m.CurrentWeeklyTotalCount > 0 {
				result.WeeklyTotal = m.CurrentWeeklyTotalCount
				result.WeeklyUsed = max(0, m.CurrentWeeklyUsageCount)
				if m.WeeklyRemainsTime > 0 {
					t := time.Now().Add(time.Duration(m.WeeklyRemainsTime) * time.Millisecond)
					result.WeeklyResetAt = &t
				}
			}
		}
	}
	result.Models = models
	return result, nil
}

// --- 智谱 GLM Coding Plan ---

func queryZhipuTokenPlan(ctx context.Context, apiKey string) (*TokenPlanResult, error) {
	body, err := doGet(ctx, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", apiKey)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Limits []struct {
				ResourceType string  `json:"resource_type"`
				LimitPeriod  string  `json:"limit_period"`
				LimitValue   float64 `json:"limit_value"`
				UsedValue    float64 `json:"used_value"`
				RemainValue  float64 `json:"remain_value"`
				ResetTime    string  `json:"reset_time"`
			} `json:"limits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("zhipu: parse response: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("zhipu: API error code=%d msg=%s", resp.Code, resp.Msg)
	}

	result := &TokenPlanResult{}
	for _, limit := range resp.Data.Limits {
		switch {
		case limit.ResourceType == "TOKENS_LIMIT" && (limit.LimitPeriod == "MONTH" || limit.LimitPeriod == "QUARTER" || limit.LimitPeriod == "YEAR"):
			result.QuotaTotal = limit.LimitValue
			result.QuotaUsed = limit.UsedValue
			if limit.ResetTime != "" {
				if t, err := time.Parse(time.RFC3339, limit.ResetTime); err == nil {
					result.QuotaResetAt = &t
				}
			}
		case limit.ResourceType == "REQUESTS_LIMIT" && limit.LimitPeriod == "MONTH":
			if result.QuotaTotal == 0 {
				result.QuotaTotal = limit.LimitValue
				result.QuotaUsed = limit.UsedValue
			}
		case limit.ResourceType == "TIME_LIMIT" && limit.LimitPeriod == "DAY":
			result.WeeklyTotal = limit.LimitValue
			result.WeeklyUsed = limit.UsedValue
			if limit.ResetTime != "" {
				if t, err := time.Parse(time.RFC3339, limit.ResetTime); err == nil {
					result.WeeklyResetAt = &t
				}
			}
		}
	}
	return result, nil
}

// --- StepFun 阶跃星辰 ---

func queryStepFunBalance(ctx context.Context, apiKey string) (*BalanceResult, error) {
	body, err := doGet(ctx, "https://api.stepfun.ai/v1/accounts", apiKey)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Object              string  `json:"object"`
		Type                string  `json:"type"`
		Balance             float64 `json:"balance"`
		TotalCashBalance    float64 `json:"total_cash_balance"`
		TotalVoucherBalance float64 `json:"total_voucher_balance"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("stepfun: parse response: %w", err)
	}
	if resp.Object == "" {
		return nil, fmt.Errorf("stepfun: unexpected response")
	}

	return &BalanceResult{Balance: resp.Balance, BalanceUsed: 0, Currency: "CNY"}, nil
}

// stepFunPlanURL 是 StepFun 控制台套餐用量查询端点。
// 提取为包级变量以便测试覆盖（httptest mock server）。
var stepFunPlanURL = "https://platform.stepfun.com/api/step.openapi.devcenter.Dashboard/QueryStepPlanRateLimit"

// --- StepFun Plan 套餐用量（控制台 Oasis-Token 鉴权）---
//
// StepFun 控制台的套餐用量查询与 OpenAI 兼容 API 完全不同：
//   - 域名：platform.stepfun.com（控制台），非 api.stepfun.ai（API）
//   - 凭据：Oasis-Token（access_jwt...refresh_jwt 格式），非 sk- API key
//   - 鉴权：Cookie + oasis-appid header，非 Bearer auth
//   - access token 仅约 30 分钟有效，过期后查询失败需用户重新获取
//
// Oasis-Webid（device_id）从 refresh token payload 自动解码，用户只需填 Oasis-Token。
func queryStepFunPlanTokenPlan(ctx context.Context, oasisToken string) (*TokenPlanResult, error) {
	if oasisToken == "" {
		return nil, fmt.Errorf("stepfun_plan: oasis token is required")
	}

	// 从 refresh token（Oasis-Token 的 ... 后半段）解码 device_id 作为 Oasis-Webid
	webid := decodeStepFunWebID(oasisToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stepFunPlanURL, strings.NewReader("{}"))
	if err != nil {
		return nil, fmt.Errorf("stepfun_plan: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Cookie", "Oasis-Token="+oasisToken+"; Oasis-Webid="+webid)
	req.Header.Set("oasis-appid", "10300")
	req.Header.Set("oasis-platform", "web")
	req.Header.Set("oasis-webid", webid)
	req.Header.Set("Origin", "https://platform.stepfun.com")
	req.Header.Set("Referer", "https://platform.stepfun.com/plan-subscribe")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stepfun_plan: http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stepfun_plan: read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 解析 Connect 错误信息，给出友好提示
		var errResp struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = fmt.Sprintf("http status %d", resp.StatusCode)
		}
		// access token 过期或被判盗用时返回 401
		if resp.StatusCode == 401 {
			return nil, fmt.Errorf("stepfun_plan: 鉴权失败（%s），Oasis-Token 可能已过期，请重新获取", msg)
		}
		return nil, fmt.Errorf("stepfun_plan: %s", msg)
	}

	var data struct {
		Status              int    `json:"status"`
		Desc                string `json:"desc"`
		PlanFamily          int    `json:"plan_family"`
		PlanCreditRateLimit struct {
			SubscriptionCreditLeftRate  float64 `json:"subscription_credit_left_rate"`
			SubscriptionCreditResetTime string  `json:"subscription_credit_reset_time"`
			CreditBuckets               []struct {
				Type           int    `json:"type"`
				CreditTotal    string `json:"credit_total"`
				CreditResidual string `json:"credit_residual"`
				ExpireAt       string `json:"expire_at"`
				NextResetAt    string `json:"next_reset_at"`
			} `json:"credit_buckets"`
		} `json:"plan_credit_rate_limit"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("stepfun_plan: parse response: %w", err)
	}

	result := &TokenPlanResult{}
	// 取第一个 credit bucket 作为主配额
	if len(data.PlanCreditRateLimit.CreditBuckets) > 0 {
		bucket := data.PlanCreditRateLimit.CreditBuckets[0]
		total := parseFloat(bucket.CreditTotal)
		residual := parseFloat(bucket.CreditResidual)
		result.QuotaTotal = total
		result.QuotaUsed = max(0, total-residual)

		// next_reset_at 是 Unix 时间戳字符串
		if bucket.NextResetAt != "" {
			if ts, err := strconv.ParseInt(bucket.NextResetAt, 10, 64); err == nil && ts > 0 {
				t := time.Unix(ts, 0)
				result.QuotaResetAt = &t
			}
		}
	}
	return result, nil
}

// decodeStepFunWebID 从 Oasis-Token 中解码 refresh token 的 device_id 字段作为 Oasis-Webid。
// Oasis-Token 格式为 "access_jwt...refresh_jwt"，取 ... 后半段解码 payload。
// 解码失败返回空字符串（服务端会接受空 webid 的请求，或返回明确鉴权错误）。
func decodeStepFunWebID(oasisToken string) string {
	// 分割 access...refresh
	idx := strings.Index(oasisToken, "...")
	if idx < 0 {
		return ""
	}
	refresh := oasisToken[idx+3:]
	// JWT 格式 header.payload.signature，取 payload
	parts := strings.Split(refresh, ".")
	if len(parts) < 2 {
		return ""
	}
	payload := parts[1]
	// base64url 解码
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// 尝试不带 padding 的 base64url
		decoded, err = base64.RawURLEncoding.DecodeString(payload)
		if err != nil {
			return ""
		}
	}
	var claims struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}
	return claims.DeviceID
}

// --- SenseNova 套餐用量（商汤日日新 Coding Plan）---
//
// 与 StepFun Plan 类似，使用控制台 token 鉴权查询套餐用量，但更简单：
//   - 标准 Bearer JWT 鉴权（非 Cookie + 自定义头）
//   - Token 有效期约 3 小时（比 StepFun 的 30 分钟长）
//   - GET 请求，URL 参数带 account_id 和 model_ids
//   - 响应是每模型剩余百分比（非绝对值）
//
// account_id 从 JWT payload 的 ext.tenant_id 字段自动解码。
var senseNovaPlanURL = "https://platform.sensenova.cn/lite/console/v1/user/coding-plan/usages"

func querySenseNovaPlanTokenPlan(ctx context.Context, token string) (*TokenPlanResult, error) {
	if token == "" {
		return nil, fmt.Errorf("sensenova_plan: token is required")
	}

	// 从 JWT 解码 tenant_id 作为 account_id
	accountID := decodeSenseNovaAccountID(token)
	if accountID == "" {
		return nil, fmt.Errorf("sensenova_plan: 无法从 Token 解码 account_id，请检查 Token 是否完整")
	}

	// 构造 URL：account_id + 固定模型列表
	u := senseNovaPlanURL + "?account_id=" + accountID
	for _, m := range senseNovaPlanModels {
		u += "&model_ids=" + m
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("sensenova_plan: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://platform.sensenova.cn/console")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sensenova_plan: http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sensenova_plan: read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sensenova_plan: http status %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		ModelRemainingPercent map[string]float64 `json:"model_remaining_percent"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("sensenova_plan: parse response: %w", err)
	}

	// 响应是每模型剩余百分比，汇总为总量/已用
	// 百分比无法直接换算成绝对额度，用百分比作为 total=100, used=100-pct
	result := &TokenPlanResult{
		QuotaTotal: 100,
	}
	var models []TokenPlanModelUsage
	for _, modelName := range senseNovaPlanModels {
		remaining := data.ModelRemainingPercent[modelName]
		models = append(models, TokenPlanModelUsage{
			ModelName:  modelName,
			QuotaTotal: 100,
			QuotaUsed:  max(0, 100-remaining),
		})
	}

	// 汇总：取所有模型中已用比例最高的作为总体已用
	maxUsed := 0.0
	for _, m := range models {
		if m.QuotaUsed > maxUsed {
			maxUsed = m.QuotaUsed
		}
	}
	result.QuotaUsed = maxUsed
	result.Models = models
	return result, nil
}

// senseNovaPlanModels 是 SenseNova Coding Plan 支持的模型列表。
// 查询时需要传入 model_ids 参数，服务端返回每模型的剩余百分比。
var senseNovaPlanModels = []string{
	"sensenova-6.7-flash-lite",
	"sensenova-u1-fast",
	"deepseek-v4-flash",
}

// decodeSenseNovaAccountID 从 SenseNova JWT 的 payload 中解码 ext.tenant_id。
func decodeSenseNovaAccountID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload := parts[1]
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(payload)
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Ext struct {
			TenantID string `json:"tenant_id"`
		} `json:"ext"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}
	return claims.Ext.TenantID
}

// --- MiMo Token Plan (小米 MiMo Coding Plan) ---
//
// 使用浏览器 Cookie 鉴权查询小米 MiMo 套餐用量。
// 鉴权信息：Cookie 中的 api-platform_serviceToken + userId + api-platform_slh + api-platform_ph。
// Cookie 有效期较长（实测数天到数周），过期后需用户重新从浏览器获取。
//
// API 端点：
//   - 用量：GET https://platform.xiaomimimo.com/api/v1/tokenPlan/usage
//   - 详情：GET https://platform.xiaomimimo.com/api/v1/tokenPlan/detail
//
// apiKey 参数传入完整的 Cookie 字符串。
var mimoPlanUsageURL = "https://platform.xiaomimimo.com/api/v1/tokenPlan/usage"
var mimoPlanDetailURL = "https://platform.xiaomimimo.com/api/v1/tokenPlan/detail"

func queryMiMoPlanTokenPlan(ctx context.Context, cookie string) (*TokenPlanResult, error) {
	if cookie == "" {
		return nil, fmt.Errorf("mimo_plan: cookie 不能为空，请在浏览器登录 platform.xiaomimimo.com 后从开发者工具复制 Cookie")
	}

	// 验证 cookie 中包含必要的字段
	if !strings.Contains(cookie, "api-platform_serviceToken") {
		return nil, fmt.Errorf("mimo_plan: Cookie 缺少 api-platform_serviceToken 字段，请确保复制了完整的 Cookie（需包含 api-platform_serviceToken、userId、api-platform_slh、api-platform_ph）")
	}

	// 1. 查询用量
	usageBody, err := doMiMoGet(ctx, mimoPlanUsageURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("mimo_plan: query usage: %w", err)
	}

	var usageResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			MonthUsage struct {
				Percent float64 `json:"percent"`
				Items   []struct {
					Name    string  `json:"name"`
					Used    float64 `json:"used"`
					Limit   float64 `json:"limit"`
					Percent float64 `json:"percent"`
				} `json:"items"`
			} `json:"monthUsage"`
			Usage struct {
				Percent float64 `json:"percent"`
				Items   []struct {
					Name    string  `json:"name"`
					Used    float64 `json:"used"`
					Limit   float64 `json:"limit"`
					Percent float64 `json:"percent"`
				} `json:"items"`
			} `json:"usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(usageBody, &usageResp); err != nil {
		return nil, fmt.Errorf("mimo_plan: parse usage response: %w", err)
	}
	if usageResp.Code != 0 {
		return nil, fmt.Errorf("mimo_plan: API error code=%d msg=%s", usageResp.Code, usageResp.Message)
	}

	// 2. 查询套餐详情（获取到期时间）
	detailBody, err := doMiMoGet(ctx, mimoPlanDetailURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("mimo_plan: query detail: %w", err)
	}

	var detailResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			PlanCode          string `json:"planCode"`
			PlanName          string `json:"planName"`
			CurrentPeriodEnd  string `json:"currentPeriodEnd"`
			Expired           bool   `json:"expired"`
			EnableAutoRenew   bool   `json:"enableAutoRenew"`
		} `json:"data"`
	}
	if err := json.Unmarshal(detailBody, &detailResp); err != nil {
		return nil, fmt.Errorf("mimo_plan: parse detail response: %w", err)
	}
	if detailResp.Code != 0 {
		return nil, fmt.Errorf("mimo_plan: detail API error code=%d msg=%s", detailResp.Code, detailResp.Message)
	}

	// 3. 组装结果
	// usage.items[0] = plan_total_token（套餐总额度）
	// usage.items[1] = compensation_total_token（补偿额度，通常为 0）
	// monthUsage.items[0] = month_total_token（本月已用）
	result := &TokenPlanResult{}

	// 套餐总额度（从 usage.items[0]）
	if len(usageResp.Data.Usage.Items) > 0 {
		planItem := usageResp.Data.Usage.Items[0]
		result.QuotaTotal = planItem.Limit
		result.QuotaUsed = planItem.Used
	}

	// 月度额度（从 monthUsage.items[0]）
	if len(usageResp.Data.MonthUsage.Items) > 0 {
		monthItem := usageResp.Data.MonthUsage.Items[0]
		result.WeeklyTotal = monthItem.Limit
		result.WeeklyUsed = monthItem.Used
	}

	// 到期时间
	if detailResp.Data.CurrentPeriodEnd != "" {
		// 格式: "2006-01-02 15:04:05"
		if t, err := time.Parse("2006-01-02 15:04:05", detailResp.Data.CurrentPeriodEnd); err == nil {
			result.QuotaResetAt = &t
		}
	}

	// 各模型明细（MiMo 不分模型，只有一个汇总）
	result.Models = []TokenPlanModelUsage{
		{
			ModelName:  "MiMo (Total)",
			QuotaTotal: result.QuotaTotal,
			QuotaUsed:  result.QuotaUsed,
		},
	}

	return result, nil
}

// doMiMoGet 执行 MiMo 平台的 GET 请求，使用 Cookie 鉴权。
func doMiMoGet(ctx context.Context, url, cookie string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// --- 302.ai ---

func query302AIBalance(ctx context.Context, apiKey string) (*BalanceResult, error) {
	body, err := doGet(ctx, "https://api.302.ai/dashboard/balance", apiKey)
	if err != nil {
		return nil, err
	}

	// 302.ai returns a simple JSON: { balance: float64, total: float64, ... }
	var resp struct {
		Balance float64 `json:"balance"`
		Total   float64 `json:"total"`
		Used    float64 `json:"used"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("302ai: parse response: %w", err)
	}

	balance := resp.Balance
	if balance == 0 {
		balance = resp.Total
	}
	return &BalanceResult{Balance: balance, BalanceUsed: resp.Used, Currency: "CNY"}, nil
}

// --- Novita AI ---

func queryNovitaBalance(ctx context.Context, apiKey string) (*BalanceResult, error) {
	body, err := doGet(ctx, "https://api.novita.ai/openapi/v1/billing/balance/detail", apiKey)
	if err != nil {
		return nil, err
	}

	var resp struct {
		AvailableBalance    string `json:"availableBalance"`
		CashBalance         string `json:"cashBalance"`
		CreditLimit         string `json:"creditLimit"`
		PendingCharges      string `json:"pendingCharges"`
		OutstandingInvoices string `json:"outstandingInvoices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("novita: parse response: %w", err)
	}

	// Novita balance unit: 1/10000 USD (10000 = $1.00)
	balance := parseFloat(resp.AvailableBalance) / 10000
	return &BalanceResult{Balance: balance, BalanceUsed: 0, Currency: "USD"}, nil
}

// --- OpenAI ---

func queryOpenAIBalance(ctx context.Context, apiKey string) (*BalanceResult, error) {
	// OpenAI 官方余额接口。/dashboard/billing/subscription 已被 OpenAI 废弃
	// （需 session token，API key 无法访问，2023 年底停用），故不再回退。
	body, err := doGet(ctx, "https://api.openai.com/v1/balances", apiKey)
	if err != nil {
		return nil, fmt.Errorf("openai: query /v1/balances failed (this endpoint requires an organization with grant credits; standard pay-as-you-go accounts may not be supported): %w", err)
	}

	var resp struct {
		TotalGrantedUSD   float64 `json:"total_granted_usd"`
		TotalUsedUSD      float64 `json:"total_used_usd"`
		TotalAvailableUSD float64 `json:"total_available_usd"`
		ExpiresAt         string  `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openai: parse response: %w", err)
	}
	if resp.TotalGrantedUSD <= 0 {
		return nil, fmt.Errorf("openai: /v1/balances returned no grant credits (total_granted_usd=%.2f); this account may not support balance query via API key", resp.TotalGrantedUSD)
	}

	return &BalanceResult{
		Balance:     resp.TotalAvailableUSD,
		BalanceUsed: resp.TotalUsedUSD,
		Currency:    "USD",
	}, nil
}

// --- Helpers ---

func doGet(ctx context.Context, url, apiKey string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// parseFloat 解析 API 返回的数字字符串为 float64。
// 优先使用 strconv.ParseFloat（比 fmt.Sscanf 更严格、更快、容错更好）；
// 失败时清理常见干扰字符（货币符号、千分位逗号、空白）后重试，仍失败返回 0。
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	// 清理常见干扰字符：货币符号、千分位逗号、空白。
	cleaned := strings.NewReplacer(
		"$", "", "€", "", "£", "", "¥", "", "￥", "",
		",", "", " ", "", "\t", "",
	).Replace(s)
	if v, err := strconv.ParseFloat(cleaned, 64); err == nil {
		return v
	}
	return 0
}
