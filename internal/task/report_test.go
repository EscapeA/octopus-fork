package task

import (
	"strings"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

// ── insertCommas ──

func TestInsertCommas(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0", "0"},
		{"12", "12"},
		{"123", "123"},
		{"1234", "1,234"},
		{"673138", "673,138"},
		{"1000000", "1,000,000"},
		{"999", "999"},
	}
	for _, c := range cases {
		if got := insertCommas(c.in); got != c.want {
			t.Errorf("insertCommas(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── formatCount ──

func TestFormatCount(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{44, "44"},
		{673138, "673,138"},
		{-1, "-1"},
		{-673138, "-673,138"},
		{1000000, "1,000,000"},
	}
	for _, c := range cases {
		if got := formatCount(c.in); got != c.want {
			t.Errorf("formatCount(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── formatTokens ──

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1k"},
		{1500, "1.5k"},
		{12000, "12k"},
		{312000, "312k"},
		{1000000, "1000k"},
	}
	for _, c := range cases {
		if got := formatTokens(c.in); got != c.want {
			t.Errorf("formatTokens(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── formatOverviewSection ──

func TestFormatOverviewSection(t *testing.T) {
	o := &model.AnalyticsOverview{
		AnalyticsMetrics: model.AnalyticsMetrics{
			RequestCount: 44,
			TotalTokens:  673138,
			InputTokens:  643024,
			OutputTokens: 30114,
			TotalCost:    0.11,
			// SuccessRate 已是 0-100，不应再乘 100。
			SuccessRate: 93.18,
		},
		ProviderCount: 12,
		APIKeyCount:   19,
		ModelCount:    8,
		FallbackRate:  5.5,
	}
	lines := formatOverviewSection(o)
	joined := strings.Join(lines, "\n")

	// 千分位
	if !strings.Contains(joined, "请求总数: 44") {
		t.Errorf("expected 请求总数: 44, got:\n%s", joined)
	}
	if !strings.Contains(joined, "673,138") {
		t.Errorf("expected thousands-grouped 673,138, got:\n%s", joined)
	}
	if !strings.Contains(joined, "643,024") || !strings.Contains(joined, "30,114") {
		t.Errorf("expected input/output tokens grouped, got:\n%s", joined)
	}
	// 成功率不应出现 9318.18（双乘 bug 的标志）
	if strings.Contains(joined, "9318") {
		t.Errorf("success rate double-multiplied (9318), got:\n%s", joined)
	}
	if !strings.Contains(joined, "成功率: 93.18%") {
		t.Errorf("expected 成功率: 93.18%%, got:\n%s", joined)
	}
	if !strings.Contains(joined, "活跃渠道: 12") || !strings.Contains(joined, "活跃模型: 8") || !strings.Contains(joined, "API Key: 19") {
		t.Errorf("expected active counts, got:\n%s", joined)
	}
}

// ── formatErrorAnalysisSection ──

func TestFormatErrorAnalysisSection(t *testing.T) {
	t.Run("success rate not double-multiplied", func(t *testing.T) {
		o := &model.AnalyticsOverview{
			AnalyticsMetrics: model.AnalyticsMetrics{
				RequestCount: 44,
				SuccessRate:  93.18,
			},
			FallbackRate: 5.0,
		}
		lines := formatErrorAnalysisSection(o)
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "成功率: 93.18%") {
			t.Errorf("expected 成功率: 93.18%%, got:\n%s", joined)
		}
		if strings.Contains(joined, "9318") {
			t.Errorf("success rate double-multiplied, got:\n%s", joined)
		}
		// 回退率也不应双乘
		if !strings.Contains(joined, "回退率: 5.00%") {
			t.Errorf("expected 回退率: 5.00%%, got:\n%s", joined)
		}
		if strings.Contains(joined, "500%") {
			t.Errorf("fallback rate double-multiplied, got:\n%s", joined)
		}
	})

	t.Run("failed requests non-negative", func(t *testing.T) {
		// 44 请求，93.18% 成功率 -> 成功 ~41 -> 失败 3
		o := &model.AnalyticsOverview{
			AnalyticsMetrics: model.AnalyticsMetrics{
				RequestCount: 44,
				SuccessRate:  93.18,
			},
		}
		lines := formatErrorAnalysisSection(o)
		joined := strings.Join(lines, "\n")
		if strings.Contains(joined, "-") {
			t.Errorf("failed requests should not be negative, got:\n%s", joined)
		}
		if !strings.Contains(joined, "失败请求: 3") {
			t.Errorf("expected 失败请求: 3, got:\n%s", joined)
		}
	})

	t.Run("zero requests", func(t *testing.T) {
		o := &model.AnalyticsOverview{}
		lines := formatErrorAnalysisSection(o)
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "失败请求: 0") {
			t.Errorf("expected 失败请求: 0 for zero requests, got:\n%s", joined)
		}
	})
}

// ── formatCostBreakdownSection ──

func TestFormatCostBreakdownSection(t *testing.T) {
	t.Run("with data", func(t *testing.T) {
		o := &model.AnalyticsOverview{
			AnalyticsMetrics: model.AnalyticsMetrics{
				RequestCount: 44,
				TotalTokens:  673138,
				TotalCost:    0.11,
			},
		}
		lines := formatCostBreakdownSection(o)
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "总成本: $0.11") {
			t.Errorf("expected 总成本: $0.11, got:\n%s", joined)
		}
		// 0.11 / 44 = 0.0025
		if !strings.Contains(joined, "平均每请求: $0.0025") {
			t.Errorf("expected 平均每请求: $0.0025, got:\n%s", joined)
		}
		// (0.11 / 673138) * 1000 = 0.0002 (rounded)
		if !strings.Contains(joined, "每千 token:") {
			t.Errorf("expected 每千 token line, got:\n%s", joined)
		}
	})

	t.Run("zero guards", func(t *testing.T) {
		o := &model.AnalyticsOverview{}
		lines := formatCostBreakdownSection(o)
		if len(lines) != 1 {
			t.Errorf("expected only total-cost line when no requests/tokens, got %d lines: %v", len(lines), lines)
		}
		if !strings.Contains(lines[0], "总成本: $0.00") {
			t.Errorf("expected 总成本: $0.00, got %q", lines[0])
		}
	})
}

// ── isReportDue 时区判定 ──

func TestIsReportDue_TimezoneAlignment(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("failed to load Asia/Shanghai: %v", err)
	}

	sched := model.ReportSchedule{
		Type:     model.ReportTypeDaily,
		SendHour: 9, // 用户希望上海时间 09:00 发送
	}

	t.Run("fires at UTC 01:xx (Shanghai 09:xx)", func(t *testing.T) {
		// 上海 UTC+8，09:00 上海 = 01:00 UTC
		now := time.Date(2026, 7, 10, 1, 5, 0, 0, time.UTC).In(loc)
		if !isReportDue(sched, now, loc) {
			t.Fatalf("expected report due at Shanghai 09:05 (UTC 01:05), got not due")
		}
	})

	t.Run("does not fire at UTC 09:xx (Shanghai 17:xx)", func(t *testing.T) {
		// UTC 09:00 = 上海 17:00，不应触发 SendHour=9
		now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC).In(loc)
		if isReportDue(sched, now, loc) {
			t.Fatalf("expected NOT due at Shanghai 17:00 (UTC 09:00), got due")
		}
	})

	t.Run("already sent today skips", func(t *testing.T) {
		// 同一天（上海时区）已发过 -> 不再发
		now := time.Date(2026, 7, 10, 9, 5, 0, 0, time.UTC).In(loc)
		lastSent := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC).UnixMilli() // 上海 09:00 已发
		schedSent := sched
		schedSent.LastSentAt = lastSent
		if isReportDue(schedSent, now, loc) {
			t.Fatalf("expected not due (already sent today), got due")
		}
	})

	t.Run("sent yesterday does not skip today", func(t *testing.T) {
		// 昨天发过 -> 今天可发
		now := time.Date(2026, 7, 10, 1, 5, 0, 0, time.UTC).In(loc)         // 上海 7/10 09:05
		lastSent := time.Date(2026, 7, 9, 1, 0, 0, 0, time.UTC).UnixMilli() // 上海 7/9 09:00
		schedSent := sched
		schedSent.LastSentAt = lastSent
		if !isReportDue(schedSent, now, loc) {
			t.Fatalf("expected due (last sent yesterday), got not due")
		}
	})
}
