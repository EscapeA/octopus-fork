package planprovider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchTokenRhythmWallet 校验钱包明细三个端点的字段映射：
// 金额字符串/数字两种格式、expiresAt 可空、累计到账/消费口径。
func TestFetchTokenRhythmWallet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/wallet/summary":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"currency":"CNY",` +
				`"availableBalanceCny":"220.82454716","giftAvailableCny":"220.82454716",` +
				`"rechargeBalanceCny":"0.00000000","debtBalanceCny":"0.00000000",` +
				`"frozenBalanceCny":"0.00000000","giftTotalCny":"220.82454716",` +
				`"asOf":"2026-09-24T05:03:35.577Z"}}`))
		case "/api/wallet/transactions":
			if got := r.URL.Query().Get("pageSize"); got != "20" {
				t.Errorf("pageSize = %q, want 20", got)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[` +
				`{"transactionId":"t1","type":"INVITE_REWARD","direction":"CREDIT","status":"POSTED",` +
				`"amountCny":"68.00000000","description":"邀请奖励",` +
				`"occurredAt":"2026-09-06T14:36:17.193Z","expiresAt":"2026-10-07T14:36:17.193Z",` +
				`"balanceAfterCny":"66.82454716","debtDeltaCny":"-1.17545284",` +
				`"giftDeltaCny":"66.82454716","rechargeDeltaCny":"0.00000000"},` +
				`{"transactionId":"t2","type":"MODEL_USAGE","direction":"DEBIT","status":"POSTED",` +
				`"amountCny":0.019507,"description":"模型调用扣费",` +
				`"occurredAt":"2026-08-31T23:49:50.000Z","expiresAt":null,` +
				`"balanceAfterCny":-1.17545284,"debtDeltaCny":0}` +
				`],"nextCursor":"eyJjcm"}}`))
		case "/api/usage-summary":
			_, _ = w.Write([]byte(`{"code":0,"data":{"costCny":"1.19545284","balanceCny":"220.82454716"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	restore := swapTokenRhythmWalletURLs(ts.URL)
	defer restore()

	got, err := fetchTokenRhythmWallet(context.Background(), "tr_session=sess_test; tr_csrf=csrf_test")
	if err != nil {
		t.Fatalf("fetchTokenRhythmWallet: %v", err)
	}
	if got.Currency != "CNY" {
		t.Errorf("currency = %q, want CNY", got.Currency)
	}
	if got.AvailableBalanceCNY != 220.82454716 {
		t.Errorf("available = %v, want 220.82454716", got.AvailableBalanceCNY)
	}
	if got.TotalReceivedCNY != 220.82454716 {
		t.Errorf("total received = %v, want 220.82454716 (giftTotalCny)", got.TotalReceivedCNY)
	}
	if got.TotalConsumedCNY != 1.19545284 {
		t.Errorf("total consumed = %v, want 1.19545284 (usage-summary costCny)", got.TotalConsumedCNY)
	}
	if len(got.Transactions) != 2 {
		t.Fatalf("transactions len = %d, want 2", len(got.Transactions))
	}
	if got.Transactions[0].ExpiresAt == nil || *got.Transactions[0].ExpiresAt != "2026-10-07T14:36:17.193Z" {
		t.Errorf("tx0 expiresAt = %v, want 2026-10-07T14:36:17.193Z", got.Transactions[0].ExpiresAt)
	}
	// 官网「余额」「已消费」两列的数据源（保留上游原始精度，展示层按官网口径 ceil 到 2 位）。
	if got.Transactions[0].BalanceAfterCNY != 66.82454716 {
		t.Errorf("tx0 balanceAfter = %v, want 66.82454716", got.Transactions[0].BalanceAfterCNY)
	}
	if got.Transactions[0].DebtDeltaCNY != -1.17545284 {
		t.Errorf("tx0 debtDelta = %v, want -1.17545284", got.Transactions[0].DebtDeltaCNY)
	}
	if got.Transactions[1].BalanceAfterCNY != -1.17545284 {
		t.Errorf("tx1 balanceAfter = %v, want -1.17545284", got.Transactions[1].BalanceAfterCNY)
	}
	// 「实际入账」= giftDelta + rechargeDelta（净额，抵扣欠费后 68 − 1.17545284 = 66.82454716）。
	if got.Transactions[0].GiftDeltaCNY != 66.82454716 {
		t.Errorf("tx0 giftDelta = %v, want 66.82454716", got.Transactions[0].GiftDeltaCNY)
	}
	if got.Transactions[0].RechargeDeltaCNY != 0 {
		t.Errorf("tx0 rechargeDelta = %v, want 0", got.Transactions[0].RechargeDeltaCNY)
	}
	// 数字格式（非字符串）金额同样要能解析。
	if got.Transactions[1].AmountCNY != 0.019507 {
		t.Errorf("tx1 amount = %v, want 0.019507", got.Transactions[1].AmountCNY)
	}
	if got.Transactions[1].ExpiresAt != nil {
		t.Errorf("tx1 expiresAt = %v, want nil", *got.Transactions[1].ExpiresAt)
	}
	if got.Transactions[1].Direction != "DEBIT" || got.Transactions[1].Type != "MODEL_USAGE" {
		t.Errorf("tx1 type/direction = %s/%s, want MODEL_USAGE/DEBIT",
			got.Transactions[1].Type, got.Transactions[1].Direction)
	}
}

// TestFetchTokenRhythmWalletSessionInvalid 校验会话失效（HTTP 401）返回哨兵错误，
// 供调用方丢弃会话重登一次。
func TestFetchTokenRhythmWalletSessionInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"UNAUTHORIZED","message":"未认证或登录已过期"}`))
	}))
	defer ts.Close()

	restore := swapTokenRhythmWalletURLs(ts.URL)
	defer restore()

	_, err := fetchTokenRhythmWallet(context.Background(), "tr_session=sess_dead")
	if !errors.Is(err, errTokenRhythmSessionInvalid) {
		t.Fatalf("err = %v, want errTokenRhythmSessionInvalid", err)
	}
}

// TestFetchTokenRhythmWalletAPIError 校验上游业务错误码（code != 0）被识别为错误。
func TestFetchTokenRhythmWalletAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":40001,"message":"boom"}`))
	}))
	defer ts.Close()

	restore := swapTokenRhythmWalletURLs(ts.URL)
	defer restore()

	if _, err := fetchTokenRhythmWallet(context.Background(), "tr_session=sess_test"); err == nil {
		t.Fatal("expected error for API code != 0, got nil")
	}
}

// swapTokenRhythmWalletURLs 把三个上游 URL 指向 mock server，返回恢复函数。
func swapTokenRhythmWalletURLs(base string) func() {
	oldSummary, oldTx, oldUsage := tokenRhythmWalletSummaryURL, tokenRhythmWalletTransactionsURL, tokenRhythmUsageSummaryURL
	tokenRhythmWalletSummaryURL = base + "/api/wallet/summary"
	tokenRhythmWalletTransactionsURL = base + "/api/wallet/transactions"
	tokenRhythmUsageSummaryURL = base + "/api/usage-summary"
	return func() {
		tokenRhythmWalletSummaryURL = oldSummary
		tokenRhythmWalletTransactionsURL = oldTx
		tokenRhythmUsageSummaryURL = oldUsage
	}
}
