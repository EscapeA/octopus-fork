package task

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/alert"
	"github.com/lingyuins/octopus/internal/op/analytics"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/report"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// EvaluateReportSchedules checks all enabled report schedules and sends reports
// that are due based on their type (daily/weekly/monthly) and configured send time.
func EvaluateReportSchedules() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	schedules, err := report.GetEnabledSchedules(ctx)
	if err != nil {
		log.Warnf("report: failed to load enabled schedules: %v", err)
		return
	}

	if len(schedules) == 0 {
		return
	}

	// Load notification channel map for dispatch
	notifiChannels, err := alert.NotifChannelList(ctx)
	if err != nil {
		log.Warnf("report: failed to load notification channels: %v", err)
		return
	}
	channelMap := make(map[int]*model.AlertNotifChannel, len(notifiChannels))
	for _, ch := range notifiChannels {
		channelMap[ch.ID] = &ch
	}

	now := time.Now()
	offset, _ := setting.GetInt(model.SettingKeyStatsTimezoneOffset)
	localNow := now.UTC().Add(time.Duration(offset) * time.Hour)

	for _, sched := range schedules {
		if !isReportDue(sched, localNow, offset) {
			continue
		}

		log.Debugf("report: schedule %d (%s) is due, generating %s report", sched.ID, sched.Name, sched.Type)
		generateAndSendReport(ctx, sched, channelMap)
	}
}

// isReportDue checks if a schedule should run at the given time.
func isReportDue(sched model.ReportSchedule, now time.Time, timezoneOffset int) bool {
	// Check if we already sent today in the configured stats timezone.
	if sched.LastSentAt > 0 {
		lastSent := time.UnixMilli(sched.LastSentAt).UTC().Add(time.Duration(timezoneOffset) * time.Hour)
		if lastSent.Year() == now.Year() && lastSent.YearDay() == now.YearDay() {
			return false // Already sent today
		}
	}

	// Check hour
	if now.Hour() != sched.SendHour {
		return false
	}

	switch sched.Type {
	case model.ReportTypeDaily:
		return true
	case model.ReportTypeWeekly:
		return int(now.Weekday()) == sched.SendDayOfWeek
	case model.ReportTypeMonthly:
		return now.Day() == sched.SendDayOfMonth
	}

	return false
}

// generateAndSendReport generates a report for the given schedule and sends it.
func generateAndSendReport(ctx context.Context, sched model.ReportSchedule, channelMap map[int]*model.AlertNotifChannel) {
	// Determine report range based on type
	var rangeStr string
	var rangeTitle string
	now := time.Now()

	switch sched.Type {
	case model.ReportTypeDaily:
		rangeStr = "1d"
		rangeTitle = fmt.Sprintf("%s 日报", now.Format("2006-01-02"))
	case model.ReportTypeWeekly:
		rangeStr = "7d"
		rangeTitle = fmt.Sprintf("%s 周报", now.Format("2006-01-02"))
	case model.ReportTypeMonthly:
		rangeStr = "30d"
		rangeTitle = fmt.Sprintf("%s 月报", now.Format("2006-01"))
	default:
		log.Warnf("report: unknown schedule type: %s", sched.Type)
		return
	}

	// Parse metrics
	metrics := report.ParseMetrics(sched.Metrics)
	if len(metrics) == 0 {
		log.Warnf("report: schedule %d has no metrics configured", sched.ID)
		return
	}

	// Generate report content
	content, err := buildReportContent(ctx, rangeStr, rangeTitle, metrics)
	if err != nil {
		log.Warnf("report: failed to generate content for schedule %d: %v", sched.ID, err)
		recordReportHistory(sched, "", "failed", fmt.Sprintf("generate error: %v", err))
		return
	}

	// Find notification channel
	ch, ok := channelMap[sched.NotifChannelID]
	if !ok || ch == nil {
		log.Warnf("report: schedule %d references notif_channel_id=%d which was not found", sched.ID, sched.NotifChannelID)
		recordReportHistory(sched, content, "skipped", "notification channel not found")
		return
	}

	// Send notification
	title := fmt.Sprintf("%s - %s", sched.Name, rangeTitle)
	if err := helper.SendNotificationMessage(ch, title, content); err != nil {
		log.Warnf("report: failed to send for schedule %d: %v", sched.ID, err)
		recordReportHistory(sched, content, "failed", err.Error())
		return
	}

	// Update last sent time
	if err := report.ScheduleUpdateLastSent(ctx, sched.ID, time.Now().UnixMilli()); err != nil {
		log.Warnf("report: failed to update last_sent_at for schedule %d: %v", sched.ID, err)
	}

	recordReportHistory(sched, content, "sent", fmt.Sprintf("%s (%s)", ch.Name, ch.Type))
}

// buildReportContent generates the formatted report text.
func buildReportContent(ctx context.Context, rangeStr, rangeTitle string, metrics []model.ReportMetric) (string, error) {
	var sections []string

	// Header
	sections = append(sections, fmt.Sprintf("📊 %s", rangeTitle))
	sections = append(sections, "")

	// Parse range
	rangeEnum, err := model.ParseAnalyticsRange(rangeStr)
	if err != nil {
		return "", fmt.Errorf("invalid range: %w", err)
	}

	// Overview section
	if containsMetric(metrics, model.ReportMetricOverview) {
		overview, err := analytics.AnalyticsOverviewGet(ctx, rangeEnum)
		if err == nil && overview != nil {
			sections = append(sections, "## 总览")
			sections = append(sections, fmt.Sprintf("- 请求总数: %d", overview.RequestCount))
			sections = append(sections, fmt.Sprintf("- Token 总数: %d", overview.TotalTokens))
			sections = append(sections, fmt.Sprintf("  - 输入: %d", overview.InputTokens))
			sections = append(sections, fmt.Sprintf("  - 输出: %d", overview.OutputTokens))
			sections = append(sections, fmt.Sprintf("- 总成本: $%.2f", overview.TotalCost))
			sections = append(sections, fmt.Sprintf("- 成功率: %.2f%%", overview.SuccessRate*100))
			sections = append(sections, fmt.Sprintf("- 活跃渠道: %d", overview.ProviderCount))
			sections = append(sections, fmt.Sprintf("- 活跃模型: %d", overview.ModelCount))
			sections = append(sections, fmt.Sprintf("- API Key: %d", overview.APIKeyCount))
			sections = append(sections, "")
		}
	}

	// Top models
	if containsMetric(metrics, model.ReportMetricTopModels) {
		modelBreakdown, err := analytics.AnalyticsModelBreakdownGet(ctx, rangeEnum)
		if err == nil && len(modelBreakdown) > 0 {
			sections = append(sections, "## Top 5 模型")
			sort.Slice(modelBreakdown, func(i, j int) bool {
				return modelBreakdown[i].RequestCount > modelBreakdown[j].RequestCount
			})
			limit := 5
			if len(modelBreakdown) < limit {
				limit = len(modelBreakdown)
			}
			for i := 0; i < limit; i++ {
				m := modelBreakdown[i]
				sections = append(sections, fmt.Sprintf("%d. %s: %d 请求, %d tokens, $%.2f",
					i+1, m.ModelName, m.RequestCount, m.TotalTokens, m.TotalCost))
			}
			sections = append(sections, "")
		}
	}

	// Top channels
	if containsMetric(metrics, model.ReportMetricTopChannels) {
		providerBreakdown, err := analytics.AnalyticsProviderBreakdownGet(ctx, rangeEnum)
		if err == nil && len(providerBreakdown) > 0 {
			sections = append(sections, "## Top 5 渠道")
			sort.Slice(providerBreakdown, func(i, j int) bool {
				return providerBreakdown[i].RequestCount > providerBreakdown[j].RequestCount
			})
			limit := 5
			if len(providerBreakdown) < limit {
				limit = len(providerBreakdown)
			}
			for i := 0; i < limit; i++ {
				p := providerBreakdown[i]
				sections = append(sections, fmt.Sprintf("%d. %s: %d 请求, %d tokens, $%.2f",
					i+1, p.ChannelName, p.RequestCount, p.TotalTokens, p.TotalCost))
			}
			sections = append(sections, "")
		}
	}

	// Top API Keys
	if containsMetric(metrics, model.ReportMetricTopAPIKeys) {
		apiKeyBreakdown, err := analytics.AnalyticsAPIKeyBreakdownGet(ctx, rangeEnum)
		if err == nil && len(apiKeyBreakdown) > 0 {
			sections = append(sections, "## Top 5 API Key")
			sort.Slice(apiKeyBreakdown, func(i, j int) bool {
				return apiKeyBreakdown[i].RequestCount > apiKeyBreakdown[j].RequestCount
			})
			limit := 5
			if len(apiKeyBreakdown) < limit {
				limit = len(apiKeyBreakdown)
			}
			for i := 0; i < limit; i++ {
				k := apiKeyBreakdown[i]
				sections = append(sections, fmt.Sprintf("%d. %s: %d 请求, %d tokens, $%.2f",
					i+1, k.Name, k.RequestCount, k.TotalTokens, k.TotalCost))
			}
			sections = append(sections, "")
		}
	}

	// Cost breakdown
	if containsMetric(metrics, model.ReportMetricCostBreakdown) {
		overview, err := analytics.AnalyticsOverviewGet(ctx, rangeEnum)
		if err == nil && overview != nil {
			sections = append(sections, "## 成本明细")
			sections = append(sections, fmt.Sprintf("- 总成本: $%.2f", overview.TotalCost))
			if overview.RequestCount > 0 {
				costPerRequest := overview.TotalCost / float64(overview.RequestCount)
				sections = append(sections, fmt.Sprintf("- 平均每请求: $%.4f", costPerRequest))
			}
			if overview.TotalTokens > 0 {
				costPer1kTokens := (overview.TotalCost / float64(overview.TotalTokens)) * 1000
				sections = append(sections, fmt.Sprintf("- 每千 token: $%.4f", costPer1kTokens))
			}
			sections = append(sections, "")
		}
	}

	// Error analysis
	if containsMetric(metrics, model.ReportMetricErrorAnalysis) {
		overview, err := analytics.AnalyticsOverviewGet(ctx, rangeEnum)
		if err == nil && overview != nil {
			sections = append(sections, "## 错误分析")
			sections = append(sections, fmt.Sprintf("- 成功率: %.2f%%", overview.SuccessRate*100))
			failedRequests := overview.RequestCount - int64(float64(overview.RequestCount)*overview.SuccessRate)
			sections = append(sections, fmt.Sprintf("- 失败请求: %d", failedRequests))
			sections = append(sections, fmt.Sprintf("- 回退率: %.2f%%", overview.FallbackRate*100))
			sections = append(sections, "")
		}
	}

	// Footer
	sections = append(sections, "---")
	sections = append(sections, fmt.Sprintf("生成时间: %s", time.Now().Format("2006-01-02 15:04:05")))

	return strings.Join(sections, "\n"), nil
}

// containsMetric checks if a metric is in the list.
func containsMetric(metrics []model.ReportMetric, target model.ReportMetric) bool {
	for _, m := range metrics {
		if m == target {
			return true
		}
	}
	return false
}

// recordReportHistory records the report send result.
func recordReportHistory(sched model.ReportSchedule, content, status, detail string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := &model.ReportHistory{
		ScheduleID:   sched.ID,
		ScheduleName: sched.Name,
		Type:         sched.Type,
		Title:        sched.Name,
		Content:      content,
		SendStatus:   status,
		SendDetail:   detail,
	}

	if err := report.HistoryAdd(ctx, entry); err != nil {
		log.Warnf("report: failed to record history for schedule %d: %v", sched.ID, err)
	}
}

// TestSendReport generates and sends a test report immediately (for UI test button).
func TestSendReport(ctx context.Context, sched model.ReportSchedule) error {
	// Load notification channel
	notifiChannels, err := alert.NotifChannelList(ctx)
	if err != nil {
		return fmt.Errorf("failed to load notification channels: %w", err)
	}

	var targetChannel *model.AlertNotifChannel
	for _, ch := range notifiChannels {
		if ch.ID == sched.NotifChannelID {
			targetChannel = &ch
			break
		}
	}

	if targetChannel == nil {
		return fmt.Errorf("notification channel not found")
	}

	// Generate test report (1d range)
	metrics := report.ParseMetrics(sched.Metrics)
	if len(metrics) == 0 {
		metrics = []model.ReportMetric{model.ReportMetricOverview}
	}

	content, err := buildReportContent(ctx, "1d", "测试报告", metrics)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Send
	title := fmt.Sprintf("%s - 测试报告", sched.Name)
	if err := helper.SendNotificationMessage(targetChannel, title, content); err != nil {
		return fmt.Errorf("failed to send: %w", err)
	}

	return nil
}

// ParseMetricsFromJSON parses metrics JSON string.
func ParseMetricsFromJSON(metricsJSON string) []model.ReportMetric {
	var metrics []model.ReportMetric
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		return nil
	}
	return metrics
}

// GetChannelsForReport returns all channels for report generation.
func GetChannelsForReport(ctx context.Context) ([]model.Channel, error) {
	return channel.List(ctx)
}
