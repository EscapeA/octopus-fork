package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/alert"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/notification"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/op/stats"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/lingyuins/octopus/internal/utils/telemetry"
)

const TaskAlertEvaluate = "alert_evaluate"

const (
	alertNotifyLanguageDefault = "en"
	alertStateFiring           = "firing"
	alertStateResolved         = "resolved"
)

type alertEvaluation struct {
	Firing       bool
	CurrentValue float64
	Detail       string
}

func EvaluateAlertRules() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rules, err := alert.RuleList(ctx)
	if err != nil {
		log.Warnf("alert evaluate: failed to list rules: %v", err)
		return
	}

	channels, err := alert.NotifChannelList(ctx)
	if err != nil {
		log.Errorf("alert evaluate: failed to list notification channels: %v; all alert notifications will be skipped", err)
		channels = nil
	}
	channelMap := make(map[int]*model.AlertNotifChannel)
	for i := range channels {
		channelMap[channels[i].ID] = &channels[i]
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		currentState := alert.StateGet(rule.ID)
		eval := evaluateRule(ctx, &rule)
		prevState := currentState.State

		switch {
		case eval.Firing && prevState != model.AlertStateFiring:
			alert.StateSet(rule.ID, model.AlertStateFiring)
			currentState = alert.StateGet(rule.ID)
			notify := notifyAlert(&rule, channelMap, alertStateFiring, currentState, eval)
			recordHistory(&rule, model.AlertStateFiring, "alert triggered", notify, eval)

		case eval.Firing && prevState == model.AlertStateFiring && shouldRepeatAlert(rule, currentState):
			alert.StateSet(rule.ID, model.AlertStateFiring)
			currentState = alert.StateGet(rule.ID)
			notify := notifyAlert(&rule, channelMap, alertStateFiring, currentState, eval)
			recordHistory(&rule, model.AlertStateFiring, "alert reminder", notify, eval)

		case !eval.Firing && prevState == model.AlertStateFiring:
			alert.StateSet(rule.ID, model.AlertStateResolved)
			currentState = alert.StateGet(rule.ID)
			notify := notifyAlert(&rule, channelMap, alertStateResolved, currentState, eval)
			recordHistory(&rule, model.AlertStateResolved, "alert resolved", notify, eval)

		default:
			alert.StateSet(rule.ID, prevState) // update LastCheckedAt
		}
	}

	// Push quota exceed alert count to shared telemetry for ops dashboard
	var quotaFiringCount int64
	for _, rule := range rules {
		if rule.ConditionType == model.AlertConditionQuotaExceeded {
			state := alert.StateGet(rule.ID)
			if state.State == model.AlertStateFiring {
				quotaFiringCount++
			}
		}
	}
	telemetry.Global().SetQuotaAlerts(quotaFiringCount)
}

func evaluateRule(ctx context.Context, rule *model.AlertRule) alertEvaluation {
	// For now, check error rate using recent stats
	switch rule.ConditionType {
	case model.AlertConditionErrorRate:
		return evaluateErrorRate(rule)
	case model.AlertConditionCostThreshold:
		return evaluateCostThreshold(rule)
	case model.AlertConditionChannelDown:
		return evaluateChannelDown(ctx, rule)
	case model.AlertConditionQuotaExceeded:
		return evaluateQuotaExceeded(rule)
	default:
		return alertEvaluation{}
	}
}

func evaluateErrorRate(rule *model.AlertRule) alertEvaluation {
	stats := stats.TotalGet()
	total := stats.RequestSuccess + stats.RequestFailed
	if total == 0 {
		return alertEvaluation{CurrentValue: 0, Detail: "no requests"}
	}
	rate := float64(stats.RequestFailed) / float64(total) * 100
	return alertEvaluation{Firing: rate >= rule.Threshold, CurrentValue: rate, Detail: fmt.Sprintf("failed=%d total=%d", stats.RequestFailed, total)}
}

func evaluateCostThreshold(rule *model.AlertRule) alertEvaluation {
	stats := stats.TotalGet()
	totalCost := stats.StatsMetrics.InputCost + stats.StatsMetrics.OutputCost
	return alertEvaluation{Firing: totalCost >= rule.Threshold, CurrentValue: totalCost}
}

func evaluateChannelDown(ctx context.Context, rule *model.AlertRule) alertEvaluation {
	if rule.ScopeChannelID == 0 {
		return alertEvaluation{Detail: "scope_channel_id is required"}
	}
	channels, err := channel.List(ctx)
	if err != nil {
		return alertEvaluation{Detail: err.Error()}
	}
	for _, ch := range channels {
		if ch.ID == rule.ScopeChannelID {
			if !ch.Enabled {
				return alertEvaluation{Firing: true, CurrentValue: 1, Detail: fmt.Sprintf("channel %s disabled", ch.Name)}
			}
			return alertEvaluation{CurrentValue: 0, Detail: fmt.Sprintf("channel %s enabled", ch.Name)}
		}
	}
	return alertEvaluation{Detail: "channel not found"}
}

func evaluateQuotaExceeded(rule *model.AlertRule) alertEvaluation {
	if rule.ScopeAPIKeyID == 0 {
		return alertEvaluation{Detail: "scope_api_key_id is required"}
	}
	stats := stats.APIKeyGet(rule.ScopeAPIKeyID)
	totalCost := stats.StatsMetrics.InputCost + stats.StatsMetrics.OutputCost
	return alertEvaluation{Firing: totalCost >= rule.Threshold, CurrentValue: totalCost}
}

// alertNotifyStatus describes the outcome of an alert notification dispatch.
// It is persisted into the alert history so users can see why a notification
// was (or was not) delivered — previously a missing/mismatched channel caused
// a silent skip while the history still reported "alert triggered".
type alertNotifyStatus struct {
	Status string // "sent", "skipped", "failed"
	Detail string // human-readable detail: channel name+type, skip reason, or error
}

func notifyAlert(rule *model.AlertRule, channelMap map[int]*model.AlertNotifChannel, state string, current model.AlertStateRecord, eval alertEvaluation) alertNotifyStatus {
	ch, ok := channelMap[rule.NotifChannelID]
	if !ok || ch == nil {
		log.Warnf("alert notify: rule %d references notif_channel_id=%d which was not found; notification skipped", rule.ID, rule.NotifChannelID)
		return alertNotifyStatus{Status: "skipped", Detail: "notification channel not found"}
	}
	language := resolveAlertNotifyLanguage()
	eventTime := current.LastFiredAt
	if state == alertStateResolved && current.LastResolvedAt > 0 {
		eventTime = current.LastResolvedAt
	}

	payload := helper.AlertWebhookPayload{
		RuleID:        rule.ID,
		RuleName:      rule.Name,
		ConditionType: rule.ConditionType,
		State:         state,
		Message:       buildAlertNotificationMessage(rule.Name, state, language),
		Threshold:     rule.Threshold,
		CurrentValue:  eval.CurrentValue,
		Time:          time.UnixMilli(eventTime).Format(time.RFC3339),
	}

	if err := helper.SendNotification(ch, payload); err != nil {
		log.Warnf("alert notify: failed to send notification via %s for rule %d: %v", ch.Type, rule.ID, err)
		return alertNotifyStatus{Status: "failed", Detail: err.Error()}
	}
	return alertNotifyStatus{Status: "sent", Detail: fmt.Sprintf("%s (%s)", ch.Name, ch.Type)}
}

func recordHistory(rule *model.AlertRule, state model.AlertState, message string, notify alertNotifyStatus, eval alertEvaluation) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entry := &model.AlertHistory{
		RuleID:     rule.ID,
		RuleName:   rule.Name,
		State:      state,
		Message:    message,
		DetailJSON: buildHistoryDetailJSON(notify, eval),
	}
	if err := alert.HistoryAdd(ctx, entry); err != nil {
		log.Warnf("alert history: failed to record for rule %d: %v", rule.ID, err)
	}
	createAlertNotification(ctx, rule, state, message, notify, eval)
}

// buildHistoryDetailJSON serializes the notification outcome into the alert
// history DetailJSON field so the management UI can surface why a notification
// was sent, skipped, or failed.
func buildHistoryDetailJSON(notify alertNotifyStatus, eval alertEvaluation) string {
	b, err := json.Marshal(map[string]any{
		"notification": notify,
		"evaluation": map[string]any{
			"current_value": eval.CurrentValue,
			"detail":        eval.Detail,
		},
	})
	if err != nil {
		return ""
	}
	return string(b)
}

func shouldRepeatAlert(rule model.AlertRule, current model.AlertStateRecord) bool {
	if rule.CooldownSec <= 0 || current.LastFiredAt <= 0 {
		return false
	}
	return time.Now().UnixMilli()-current.LastFiredAt >= int64(rule.CooldownSec)*1000
}

func createAlertNotification(ctx context.Context, rule *model.AlertRule, state model.AlertState, message string, notify alertNotifyStatus, eval alertEvaluation) {
	severity := model.NotificationSeverityWarning
	if state == model.AlertStateResolved {
		severity = model.NotificationSeveritySuccess
	}
	metadata, _ := json.Marshal(map[string]any{
		"rule_id":          rule.ID,
		"rule_name":        rule.Name,
		"condition_type":   rule.ConditionType,
		"threshold":        rule.Threshold,
		"current_value":    eval.CurrentValue,
		"detail":           eval.Detail,
		"scope_channel_id": rule.ScopeChannelID,
		"scope_api_key_id": rule.ScopeAPIKeyID,
		"state":            state,
		"notification":     notify,
	})
	title := buildAlertNotificationMessage(rule.Name, alertStateToString(state), resolveAlertNotifyLanguage())
	n := &model.Notification{
		Type:         model.NotificationTypeAlert,
		Severity:     severity,
		Title:        title,
		Content:      message,
		Source:       "alert_rule",
		SourceID:     fmt.Sprintf("%d", rule.ID),
		DedupeKey:    fmt.Sprintf("alert:%d:%d:%d", rule.ID, state, time.Now().UnixMilli()),
		MetadataJSON: string(metadata),
		Link:         "alert",
	}
	if err := notification.Create(ctx, n); err != nil {
		log.Warnf("notification: failed to create alert notification for rule %d: %v", rule.ID, err)
	}
}

func alertStateToString(state model.AlertState) string {
	if state == model.AlertStateResolved {
		return alertStateResolved
	}
	return alertStateFiring
}

func resolveAlertNotifyLanguage() string {
	language, err := setting.GetString(model.SettingKeyAlertNotifyLanguage)
	if err != nil {
		return alertNotifyLanguageDefault
	}
	return normalizeAlertNotifyLanguage(language)
}

func normalizeAlertNotifyLanguage(language string) string {
	switch language {
	case "zh-Hans", "zh-Hant", "en":
		return language
	default:
		return alertNotifyLanguageDefault
	}
}

func buildAlertNotificationMessage(ruleName, state, language string) string {
	switch normalizeAlertNotifyLanguage(language) {
	case "zh-Hans":
		switch state {
		case alertStateFiring:
			return fmt.Sprintf("告警规则 \"%s\" 已触发", ruleName)
		case alertStateResolved:
			return fmt.Sprintf("告警规则 \"%s\" 已恢复", ruleName)
		default:
			return fmt.Sprintf("告警规则 \"%s\" 状态已变更为 %s", ruleName, state)
		}
	case "zh-Hant":
		switch state {
		case alertStateFiring:
			return fmt.Sprintf("告警規則 \"%s\" 已觸發", ruleName)
		case alertStateResolved:
			return fmt.Sprintf("告警規則 \"%s\" 已恢復", ruleName)
		default:
			return fmt.Sprintf("告警規則 \"%s\" 狀態已變更為 %s", ruleName, state)
		}
	default:
		return fmt.Sprintf("Alert '%s' is %s", ruleName, state)
	}
}
