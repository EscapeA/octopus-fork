package analytics

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/relaylog"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// setupSeparateLogDB 初始化主库 + 独立日志库，启用日志保留，返回清理函数。
// 独立日志库模式下 relay_logs 仅写入 LogDB，主库的 relay_logs 表为空——这正是
// issue #101 的触发条件。
func setupSeparateLogDB(t *testing.T) {
	t.Helper()
	mainPath := filepath.Join(t.TempDir(), "main.db")
	if err := db.InitDB("sqlite", mainPath, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "logs.db")
	if err := db.InitLogDB("sqlite", logPath, false); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}
	if err := setting.SetString(model.SettingKeyRelayLogKeepEnabled, "true"); err != nil {
		t.Fatalf("enable relay log keep failed: %v", err)
	}
	// 清空内存缓存，迫使查询只能依赖落库数据（模拟 7d 等长窗口下缓存已淘汰的场景）。
	t.Cleanup(relaylog.SetCacheForTest(nil))
}

// TestIssue101_ModelBreakdownReadsSeparateLogDB 验证独立日志库模式下「按模型」
// 用量分布仍能读到落库的历史日志。修复前 loadAnalyticsModelRows 用 db.GetDB()
// 读主库 relay_logs（独立库模式下为空），导致除内存缓存外的历史数据全部缺失，
// 表现为 1d 偶有数据、7d 起无数据（issue #101）。
func TestIssue101_ModelBreakdownReadsSeparateLogDB(t *testing.T) {
	setupSeparateLogDB(t)

	// 仅向独立日志库写入两条日志（模拟实际写入路径），主库 relay_logs 保持为空。
	logs := []model.RelayLog{
		{
			ID: 1, Time: time.Now().Unix(),
			RequestModelName: "gpt-4o", ActualModelName: "gpt-4o",
			ChannelId: 1, Error: "",
			InputTokens: 100, OutputTokens: 50, Cost: 0.2,
		},
		{
			ID: 2, Time: time.Now().Unix(),
			RequestModelName: "claude-3", ActualModelName: "claude-3-opus",
			ChannelId: 2, Error: "timeout",
			InputTokens: 80, OutputTokens: 0, Cost: 0.0,
		},
	}
	if err := db.GetLogDB().Create(&logs).Error; err != nil {
		t.Fatalf("seed log DB failed: %v", err)
	}

	// 主库 relay_logs 必须为空，确认测试确实覆盖独立库场景。
	var mainCount int64
	if err := db.GetDB().Model(&model.RelayLog{}).Count(&mainCount).Error; err != nil {
		t.Fatalf("count main DB failed: %v", err)
	}
	if mainCount != 0 {
		t.Fatalf("main DB relay_logs count = %d, want 0 in separate-LogDB mode", mainCount)
	}

	items, err := AnalyticsModelBreakdownGet(context.Background(), model.AnalyticsRange7D)
	if err != nil {
		t.Fatalf("AnalyticsModelBreakdownGet error: %v", err)
	}

	byModel := make(map[string]model.AnalyticsModelBreakdownItem, len(items))
	for _, it := range items {
		byModel[it.ModelName] = it
	}

	gpt, ok := byModel["gpt-4o"]
	if !ok {
		t.Fatalf("gpt-4o missing from model breakdown — issue #101: separate LogDB data not read")
	}
	if gpt.RequestCount != 1 || gpt.InputTokens != 100 {
		t.Errorf("gpt-4o metrics = {count:%d, in:%d}, want {1, 100}", gpt.RequestCount, gpt.InputTokens)
	}

	claude, ok := byModel["claude-3-opus"]
	if !ok {
		t.Fatalf("claude-3-opus missing from model breakdown — issue #101")
	}
	// 整体失败的请求应计入 request_count（success+failed）。
	if claude.RequestCount != 1 {
		t.Errorf("claude-3-opus request_count = %d, want 1", claude.RequestCount)
	}
}

// TestIssue101_ProviderBreakdownReadsSeparateLogDB 验证「按渠道」利用率在独立
// 日志库模式下同样能读到历史日志（与按模型同根因）。
func TestIssue101_ProviderBreakdownReadsSeparateLogDB(t *testing.T) {
	setupSeparateLogDB(t)

	logs := []model.RelayLog{
		{
			ID: 10, Time: time.Now().Unix(),
			RequestModelName: "m", ActualModelName: "m",
			ChannelId: 5, ChannelName: "providerA", Error: "",
			InputTokens: 30, OutputTokens: 10, Cost: 0.05,
		},
	}
	if err := db.GetLogDB().Create(&logs).Error; err != nil {
		t.Fatalf("seed log DB failed: %v", err)
	}

	items, err := AnalyticsProviderBreakdownGet(context.Background(), model.AnalyticsRange7D)
	if err != nil {
		t.Fatalf("AnalyticsProviderBreakdownGet error: %v", err)
	}

	var found bool
	for _, it := range items {
		if it.ChannelID == 5 {
			found = true
			if it.RequestCount != 1 || it.InputTokens != 30 {
				t.Errorf("providerA metrics = {count:%d, in:%d}, want {1, 30}", it.RequestCount, it.InputTokens)
			}
		}
	}
	if !found {
		t.Fatalf("providerA(5) missing from provider breakdown — issue #101: separate LogDB data not read")
	}
}
