package planprovider

import (
	"context"
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
