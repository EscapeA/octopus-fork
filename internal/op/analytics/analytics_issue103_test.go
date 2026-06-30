package analytics

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/group"
	"github.com/lingyuins/octopus/internal/op/relaylog"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// setupIssue103DB 初始化 SQLite + LogDB 并启用日志保留，返回清理函数。
func setupIssue103DB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "issue103.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := db.InitLogDB("", "", false); err != nil {
		t.Fatalf("InitLogDB(shared) failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}
	if err := setting.SetString(model.SettingKeyRelayLogKeepEnabled, "true"); err != nil {
		t.Fatalf("enable relay log keep failed: %v", err)
	}
}

// seedIssue103Group 向组缓存注入一个 failover 组，含两个渠道。
func seedIssue103Group(t *testing.T, groupID int, itemModel string) {
	t.Helper()
	gc := group.GetCache()
	gc.Clear()
	gc.Set(groupID, model.Group{
		ID:   groupID,
		Name: "issue103-group",
		Mode: model.GroupModeFailover,
		Items: []model.GroupItem{
			{ChannelID: 11, ModelName: itemModel},
			{ChannelID: 22, ModelName: itemModel},
		},
	})
	t.Cleanup(gc.Clear)
}

// findIssue103Rows 在返回项中查找渠道 11/22 的行。
func findIssue103Rows(t *testing.T, items []model.AnalyticsChannelModelItem) (foundA, foundB bool, rateA, rateB float64) {
	t.Helper()
	for _, it := range items {
		switch it.ChannelID {
		case 11:
			foundA = true
			rateA = it.SuccessRate
		case 22:
			foundB = true
			rateB = it.SuccessRate
		}
	}
	return
}

// TestIssue103_FailingChannelVisibleWithAttempts 关联表有数据时，失败重试渠道应在
// 渠道×模型中显示（基线回归保护）。
func TestIssue103_FailingChannelVisibleWithAttempts(t *testing.T) {
	setupIssue103DB(t)
	seedIssue103Group(t, 1, "gpt-4o")

	relayLog := model.RelayLog{
		ID: 1, Time: time.Now().Unix(),
		RequestModelName: "gpt-4o", ActualModelName: "gpt-4o",
		ChannelId: 11, ChannelName: "channelA", Error: "",
		InputTokens: 100, OutputTokens: 50, Cost: 0.2, TotalAttempts: 2,
		Attempts: []model.ChannelAttempt{
			{ChannelID: 22, ChannelName: "channelB", ModelName: "gpt-4o", Status: model.AttemptFailed},
			{ChannelID: 11, ChannelName: "channelA", ModelName: "gpt-4o", Status: model.AttemptSuccess},
		},
	}
	if err := db.GetLogDB().Create(&relayLog).Error; err != nil {
		t.Fatalf("seed relay log failed: %v", err)
	}
	if err := relaylog.RelayLogAttemptsAdd(context.Background(), 1, relayLog.Attempts, relayLog.Time); err != nil {
		t.Fatalf("RelayLogAttemptsAdd error: %v", err)
	}
	t.Cleanup(relaylog.SetCacheForTest(nil))

	groupID := 1
	items, err := AnalyticsChannelModelBreakdownGet(context.Background(), model.AnalyticsRange1D, &groupID)
	if err != nil {
		t.Fatalf("AnalyticsChannelModelBreakdownGet error: %v", err)
	}
	foundA, foundB, _, rateB := findIssue103Rows(t, items)
	if !foundA {
		t.Errorf("channelA(11) missing from breakdown")
	}
	if !foundB {
		t.Errorf("channelB(22) [403 failing] missing from breakdown — issue #103")
	}
	if foundB && rateB > 0.01 {
		t.Errorf("channelB success_rate=%f, want 0 (failed)", rateB)
	}
}

// TestIssue103_LegacyLogParsesAttemptsJSON 关联表无行（keepEnabled 切换或部署前
// 历史日志）时，legacy 分支从 relay_logs 的 attempts JSON 字段解析尝试维度
// （issue #121），使中间重试失败渠道（如 channelB）也能被捕获。此前仅用顶层列
// 聚合，中间重试失败不可见——该降级策略已由 issue #121 修复取消。
func TestIssue103_LegacyLogUsesTopLevelColumns(t *testing.T) {
	setupIssue103DB(t)
	seedIssue103Group(t, 1, "gpt-4o")

	relayLog := model.RelayLog{
		ID: 1, Time: time.Now().Unix(),
		RequestModelName: "gpt-4o", ActualModelName: "gpt-4o",
		ChannelId: 11, ChannelName: "channelA", Error: "",
		InputTokens: 100, OutputTokens: 50, Cost: 0.2, TotalAttempts: 2,
		Attempts: []model.ChannelAttempt{
			{ChannelID: 22, ChannelName: "channelB", ModelName: "gpt-4o", Status: model.AttemptFailed},
			{ChannelID: 11, ChannelName: "channelA", ModelName: "gpt-4o", Status: model.AttemptSuccess},
		},
	}
	if err := db.GetLogDB().Create(&relayLog).Error; err != nil {
		t.Fatalf("seed relay log failed: %v", err)
	}
	// 故意不调用 RelayLogAttemptsAdd —— 模拟关联表无行，attempts 仅存于 JSON 字段。
	t.Cleanup(relaylog.SetCacheForTest(nil))

	groupID := 1
	items, err := AnalyticsChannelModelBreakdownGet(context.Background(), model.AnalyticsRange1D, &groupID)
	if err != nil {
		t.Fatalf("AnalyticsChannelModelBreakdownGet error: %v", err)
	}
	foundA, foundB, _, rateB := findIssue103Rows(t, items)
	// 最终成功渠道 channelA(11) 由 attempts JSON 解析显示。
	if !foundA {
		t.Errorf("channelA(11) missing from breakdown")
	}
	// 中间失败渠道 channelB(22) 从 attempts JSON 解析可见（issue #121 修复）。
	if !foundB {
		t.Errorf("channelB(22) should appear in legacy breakdown (attempts JSON parsed)")
	}
	if rateB != 0 {
		t.Errorf("channelB success_rate=%f, want 0 (failed)", rateB)
	}
}

// TestIssue103_ZenGroupScopeWildcard zen 组的 GroupItem.ModelName="zen"，但 attempts
// 记录的 model_name 是解析后的上游模型。修复前 scope 用 "zen" 精确匹配，导致所有
// 渠道被过滤；修复后 zen/空 ModelName 按 channel 维度通配。
func TestIssue103_ZenGroupScopeWildcard(t *testing.T) {
	setupIssue103DB(t)
	seedIssue103Group(t, 1, "zen")

	relayLog := model.RelayLog{
		ID: 1, Time: time.Now().Unix(),
		RequestModelName: "zen/gpt-4o", ActualModelName: "gpt-4o",
		ChannelId: 11, ChannelName: "channelA", Error: "",
		InputTokens: 100, OutputTokens: 50, Cost: 0.2, TotalAttempts: 2,
		Attempts: []model.ChannelAttempt{
			{ChannelID: 22, ChannelName: "channelB", ModelName: "gpt-4o", Status: model.AttemptFailed},
			{ChannelID: 11, ChannelName: "channelA", ModelName: "gpt-4o", Status: model.AttemptSuccess},
		},
	}
	if err := db.GetLogDB().Create(&relayLog).Error; err != nil {
		t.Fatalf("seed relay log failed: %v", err)
	}
	if err := relaylog.RelayLogAttemptsAdd(context.Background(), 1, relayLog.Attempts, relayLog.Time); err != nil {
		t.Fatalf("RelayLogAttemptsAdd error: %v", err)
	}
	t.Cleanup(relaylog.SetCacheForTest(nil))

	groupID := 1
	items, err := AnalyticsChannelModelBreakdownGet(context.Background(), model.AnalyticsRange1D, &groupID)
	if err != nil {
		t.Fatalf("AnalyticsChannelModelBreakdownGet error: %v", err)
	}
	foundA, foundB, _, rateB := findIssue103Rows(t, items)
	if !foundA {
		t.Errorf("channelA(11) missing from zen breakdown")
	}
	if !foundB {
		t.Fatalf("channelB(22) [failing] missing from zen breakdown — issue #103 zen scope")
	}
	if rateB > 0.01 {
		t.Errorf("channelB success_rate=%f, want 0 (failed)", rateB)
	}
}
