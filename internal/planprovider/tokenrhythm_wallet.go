package planprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// 基元律动（tokenrhythm.studio）钱包明细（资金明细）。
//
// 数据源对应官网「用户中心 → 费用管理 → 钱包明细」页：
//   - GET /api/wallet/summary                  账户汇总（可用/赠送/充值/欠费/冻结/赠送总额）
//   - GET /api/wallet/transactions?pageSize=N  余额变动明细（游标分页 nextCursor，此处只取首屏）
//   - GET /api/usage-summary                   复用余额查询端点取 costCny 作「累计消费」
//
// 鉴权与 usage-summary 一致：Cookie（tr_session + tr_csrf）。账号密码模式下复用
// ensureTokenRhythmSession 的会话缓存，失效（HTTP 401）时丢弃会话重登一次再试。

var (
	tokenRhythmWalletSummaryURL      = "https://tokenrhythm.studio/api/wallet/summary"
	tokenRhythmWalletTransactionsURL = "https://tokenrhythm.studio/api/wallet/transactions"
)

// tokenRhythmWalletPageSize 明细条数：UI 只展示首屏，不做翻页。
const tokenRhythmWalletPageSize = 20

// TokenRhythmWalletTransaction 单条余额变动记录（字段与上游 JSON 一一对应）。
type TokenRhythmWalletTransaction struct {
	TransactionID string  `json:"transaction_id"`
	Type          string  `json:"type"`      // INVITE_REWARD / MODEL_USAGE / OTHER / RECHARGE ...
	Direction     string  `json:"direction"` // CREDIT 到账 / DEBIT 支出
	Status        string  `json:"status"`    // POSTED / PENDING / EXPIRED / FAILED ...
	AmountCNY     float64 `json:"amount_cny"`
	Description   string  `json:"description"`
	OccurredAt    string  `json:"occurred_at"` // RFC3339（UTC）
	ExpiresAt     *string `json:"expires_at"`  // 赠送额度到期时间，可能为 null
	// BalanceAfterCNY 该笔变动后的钱包余额（官网「余额 ¥xx」列）；
	// DebtDeltaCNY 该笔到账中用于抵扣欠费的部分（负数=抵扣欠费，官网「已消费」列 = |DebtDeltaCNY|）。
	// 两者均为上游原始精度，展示层的 2 位小数按官网口径 ceil 处理。
	BalanceAfterCNY float64 `json:"balance_after_cny"`
	DebtDeltaCNY    float64 `json:"debt_delta_cny"`
	// GiftDeltaCNY / RechargeDeltaCNY 该笔带来的赠送 / 充值余额净变动。
	// 「实际入账」列 = GiftDeltaCNY + RechargeDeltaCNY（抵扣欠费后为净额，如 68 − 1.18 = 66.83）。
	GiftDeltaCNY     float64 `json:"gift_delta_cny"`
	RechargeDeltaCNY float64 `json:"recharge_delta_cny"`
}

// TokenRhythmWallet 钱包明细查询结果。
//
// 汇总口径（2026-09-24 与用户确认）：
//   - TotalReceivedCNY 累计到账 = 官方 wallet/summary.giftTotalCny（累计赠送总额）
//   - TotalConsumedCNY 累计消费 = usage-summary.costCny（全生命周期累计成本，与卡片「已用额度」同源）
type TokenRhythmWallet struct {
	Currency            string                         `json:"currency"`
	AvailableBalanceCNY float64                        `json:"available_balance_cny"`
	GiftBalanceCNY      float64                        `json:"gift_balance_cny"`
	RechargeBalanceCNY  float64                        `json:"recharge_balance_cny"`
	DebtBalanceCNY      float64                        `json:"debt_balance_cny"`
	FrozenBalanceCNY    float64                        `json:"frozen_balance_cny"`
	TotalReceivedCNY    float64                        `json:"total_received_cny"`
	TotalConsumedCNY    float64                        `json:"total_consumed_cny"`
	AsOf                string                         `json:"as_of"`
	Transactions        []TokenRhythmWalletTransaction `json:"transactions"`
}

// QueryTokenRhythmWalletByID 按 provider ID 查询钱包明细（供 HTTP handler 调用）。
func QueryTokenRhythmWalletByID(ctx context.Context, id int) (*TokenRhythmWallet, error) {
	var provider model.PlanProvider
	if err := db.GetDB().WithContext(ctx).First(&provider, id).Error; err != nil {
		return nil, fmt.Errorf("find plan provider: %w", err)
	}
	// 凭据密文解密回明文供查询使用（只读，不落库）。
	decryptProviderSecrets(&provider)

	if provider.Category != model.PlanProviderTokenRhythm {
		return nil, fmt.Errorf("tokenrhythm: wallet transactions is only supported for tokenrhythm provider")
	}
	if provider.ProviderType != model.PlanProviderTypeBalance {
		return nil, fmt.Errorf("tokenrhythm: wallet transactions requires a balance provider")
	}

	cookie, err := tokenRhythmCookieForQuery(ctx, &provider)
	if err != nil {
		return nil, err
	}
	result, err := fetchTokenRhythmWallet(ctx, cookie)
	if err == nil {
		return result, nil
	}
	// 会话失效：账号密码模式下丢弃会话重登一次再试（与 refreshBalanceWithLogin 同构）。
	if !errors.Is(err, errTokenRhythmSessionInvalid) || provider.LoginUsername == "" {
		return nil, err
	}
	log.Warnf("planprovider: tokenrhythm provider %d 钱包明细会话失效，重新登录后重试", provider.ID)
	invalidateTokenRhythmSession(provider.ID)
	provider.APIKey = ""
	cookie, err = tokenRhythmCookieForQuery(ctx, &provider)
	if err != nil {
		return nil, err
	}
	return fetchTokenRhythmWallet(ctx, cookie)
}

// tokenRhythmCookieForQuery 取查询用 Cookie：账号密码模式走会话缓存/自动登录，
// 手动模式直接用用户粘贴的 Cookie。
func tokenRhythmCookieForQuery(ctx context.Context, provider *model.PlanProvider) (string, error) {
	if provider.LoginUsername != "" && provider.LoginPasswordEnc != "" {
		return ensureTokenRhythmSession(ctx, provider)
	}
	if provider.APIKey == "" {
		return "", fmt.Errorf("tokenrhythm: 未配置凭据（控制台 Cookie 或账号密码）")
	}
	return provider.APIKey, nil
}

// fetchTokenRhythmWallet 拉取汇总 + 明细 + 累计消费。
func fetchTokenRhythmWallet(ctx context.Context, cookie string) (*TokenRhythmWallet, error) {
	summaryBody, err := doTokenRhythmGet(ctx, tokenRhythmWalletSummaryURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("tokenrhythm: query wallet summary: %w", err)
	}
	var summaryResp struct {
		Code int `json:"code"`
		Data struct {
			Currency            string          `json:"currency"`
			AvailableBalanceCNY flexibleFloat64 `json:"availableBalanceCny"`
			GiftAvailableCNY    flexibleFloat64 `json:"giftAvailableCny"`
			RechargeBalanceCNY  flexibleFloat64 `json:"rechargeBalanceCny"`
			DebtBalanceCNY      flexibleFloat64 `json:"debtBalanceCny"`
			FrozenBalanceCNY    flexibleFloat64 `json:"frozenBalanceCny"`
			GiftTotalCNY        flexibleFloat64 `json:"giftTotalCny"`
			AsOf                string          `json:"asOf"`
		} `json:"data"`
	}
	if err := json.Unmarshal(summaryBody, &summaryResp); err != nil {
		return nil, fmt.Errorf("tokenrhythm: parse wallet summary: %w", err)
	}
	if summaryResp.Code != 0 {
		return nil, fmt.Errorf("tokenrhythm: wallet summary API error code=%d", summaryResp.Code)
	}

	itemsURL := tokenRhythmWalletTransactionsURL + "?pageSize=" + strconv.Itoa(tokenRhythmWalletPageSize)
	itemsBody, err := doTokenRhythmGet(ctx, itemsURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("tokenrhythm: query wallet transactions: %w", err)
	}
	var itemsResp struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				TransactionID    string          `json:"transactionId"`
				Type             string          `json:"type"`
				Direction        string          `json:"direction"`
				Status           string          `json:"status"`
				AmountCNY        flexibleFloat64 `json:"amountCny"`
				Description      string          `json:"description"`
				OccurredAt       string          `json:"occurredAt"`
				ExpiresAt        *string         `json:"expiresAt"`
				BalanceAfterCNY  flexibleFloat64 `json:"balanceAfterCny"`
				DebtDeltaCNY     flexibleFloat64 `json:"debtDeltaCny"`
				GiftDeltaCNY     flexibleFloat64 `json:"giftDeltaCny"`
				RechargeDeltaCNY flexibleFloat64 `json:"rechargeDeltaCny"`
			} `json:"items"`
			NextCursor string `json:"nextCursor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(itemsBody, &itemsResp); err != nil {
		return nil, fmt.Errorf("tokenrhythm: parse wallet transactions: %w", err)
	}
	if itemsResp.Code != 0 {
		return nil, fmt.Errorf("tokenrhythm: wallet transactions API error code=%d", itemsResp.Code)
	}

	// 累计消费：复用 usage-summary 的 costCny（与卡片「已用额度」同源）。
	usageBody, err := doTokenRhythmGet(ctx, tokenRhythmUsageSummaryURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("tokenrhythm: query usage-summary for total consumed: %w", err)
	}
	var usageResp struct {
		Code int `json:"code"`
		Data struct {
			CostCny flexibleFloat64 `json:"costCny"`
		} `json:"data"`
	}
	if err := json.Unmarshal(usageBody, &usageResp); err != nil {
		return nil, fmt.Errorf("tokenrhythm: parse usage-summary: %w", err)
	}
	if usageResp.Code != 0 {
		return nil, fmt.Errorf("tokenrhythm: usage-summary API error code=%d", usageResp.Code)
	}

	currency := summaryResp.Data.Currency
	if currency == "" {
		currency = "CNY"
	}

	result := &TokenRhythmWallet{
		Currency:            currency,
		AvailableBalanceCNY: float64(summaryResp.Data.AvailableBalanceCNY),
		GiftBalanceCNY:      float64(summaryResp.Data.GiftAvailableCNY),
		RechargeBalanceCNY:  float64(summaryResp.Data.RechargeBalanceCNY),
		DebtBalanceCNY:      float64(summaryResp.Data.DebtBalanceCNY),
		FrozenBalanceCNY:    float64(summaryResp.Data.FrozenBalanceCNY),
		TotalReceivedCNY:    float64(summaryResp.Data.GiftTotalCNY),
		TotalConsumedCNY:    float64(usageResp.Data.CostCny),
		AsOf:                summaryResp.Data.AsOf,
		Transactions:        make([]TokenRhythmWalletTransaction, 0, len(itemsResp.Data.Items)),
	}
	for _, it := range itemsResp.Data.Items {
		result.Transactions = append(result.Transactions, TokenRhythmWalletTransaction{
			TransactionID:    it.TransactionID,
			Type:             it.Type,
			Direction:        it.Direction,
			Status:           it.Status,
			AmountCNY:        float64(it.AmountCNY),
			Description:      it.Description,
			OccurredAt:       it.OccurredAt,
			ExpiresAt:        it.ExpiresAt,
			BalanceAfterCNY:  float64(it.BalanceAfterCNY),
			DebtDeltaCNY:     float64(it.DebtDeltaCNY),
			GiftDeltaCNY:     float64(it.GiftDeltaCNY),
			RechargeDeltaCNY: float64(it.RechargeDeltaCNY),
		})
	}
	return result, nil
}
