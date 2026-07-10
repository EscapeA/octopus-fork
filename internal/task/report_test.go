package task

import (
	"strings"
	"testing"

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
