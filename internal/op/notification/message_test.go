package notification

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

func TestSetMessage_WithArgs(t *testing.T) {
	n := &model.Notification{}
	SetMessage(n, KeyReportSent, KeyReportSent,
		map[string]any{"name": "daily-report"},
		map[string]any{"name": "daily-report", "channel": "gotify (gotify)"},
		[]any{"daily-report"},
		[]any{"daily-report", "gotify (gotify)"})

	// i18n 键
	if n.TitleKey != "report.sent" {
		t.Fatalf("expected title_key 'report.sent', got %q", n.TitleKey)
	}
	if n.ContentKey != "report.sent" {
		t.Fatalf("expected content_key 'report.sent', got %q", n.ContentKey)
	}

	// 参数 JSON 可反序列化
	var titleArgs map[string]any
	if err := json.Unmarshal([]byte(n.TitleArgs), &titleArgs); err != nil {
		t.Fatalf("failed to parse title_args: %v", err)
	}
	if titleArgs["name"] != "daily-report" {
		t.Fatalf("expected title_args.name='daily-report', got %v", titleArgs["name"])
	}

	var contentArgs map[string]any
	if err := json.Unmarshal([]byte(n.ContentArgs), &contentArgs); err != nil {
		t.Fatalf("failed to parse content_args: %v", err)
	}
	if contentArgs["channel"] != "gotify (gotify)" {
		t.Fatalf("expected content_args.channel, got %v", contentArgs["channel"])
	}

	// 英文回退串
	if !strings.Contains(n.Title, "daily-report") {
		t.Fatalf("expected title fallback to contain 'daily-report', got %q", n.Title)
	}
	if !strings.Contains(n.Content, "gotify (gotify)") {
		t.Fatalf("expected content fallback to contain channel, got %q", n.Content)
	}
}

func TestSetMessage_NoArgs(t *testing.T) {
	// 无参数的键（如 backup.skip）应留空 *Args，回退串为模板原文。
	n := &model.Notification{}
	SetMessage(n, KeyBackupSkip, KeyBackupSkip, nil, nil, nil, nil)

	if n.TitleKey != "backup.skip" {
		t.Fatalf("expected title_key 'backup.skip', got %q", n.TitleKey)
	}
	if n.TitleArgs != "" {
		t.Fatalf("expected empty title_args for nil args, got %q", n.TitleArgs)
	}
	if n.ContentArgs != "" {
		t.Fatalf("expected empty content_args for nil args, got %q", n.ContentArgs)
	}
	// 回退串应为模板原文（无 Sprintf）
	if n.Title == "" {
		t.Fatal("expected non-empty fallback title")
	}
	if n.Content == "" {
		t.Fatal("expected non-empty fallback content")
	}
}

func TestSetMessage_NumericArgs(t *testing.T) {
	// 整型参数（如 site.batch 的 success/failed 计数）应正确序列化进 JSON。
	n := &model.Notification{}
	SetMessage(n, KeySiteBatch, KeySiteBatch,
		map[string]any{"phase": "sync"},
		map[string]any{"trigger": "scheduled", "success": 10, "failed": 2},
		[]any{"sync"},
		[]any{"scheduled", 10, 0, 2, 0, 0})

	var contentArgs map[string]any
	if err := json.Unmarshal([]byte(n.ContentArgs), &contentArgs); err != nil {
		t.Fatalf("failed to parse content_args: %v", err)
	}
	// JSON 数字反序列化为 float64
	if contentArgs["success"].(float64) != 10 {
		t.Fatalf("expected success=10, got %v", contentArgs["success"])
	}
	if contentArgs["failed"].(float64) != 2 {
		t.Fatalf("expected failed=2, got %v", contentArgs["failed"])
	}
	if !strings.Contains(n.Content, "scheduled") {
		t.Fatalf("expected fallback content to contain trigger, got %q", n.Content)
	}
}

func TestAllKeysHaveFallbackTemplates(t *testing.T) {
	// 确保每个 NotifKey 常量都有对应的英文回退 title/content 模板，避免漏配。
	allKeys := []NotifKey{
		KeyAlertFiring, KeyAlertResolved, KeyChannelExpire,
		KeyReportSent, KeyReportFailed, KeyReportSkipped,
		KeySiteBatch, KeySiteAccountOK, KeySiteAccountFail,
		KeyBackupOK, KeyBackupFail, KeyBackupSkip,
		KeyRestoreOK, KeyRestoreFail,
		KeyMigrationOK, KeyMigrationFail,
		KeySelfUpdateOK, KeySelfUpdateFail,
	}
	for _, k := range allKeys {
		if _, ok := fallbackTitle[k]; !ok {
			t.Errorf("missing fallback title for key %q", k)
		}
		if _, ok := fallbackContent[k]; !ok {
			t.Errorf("missing fallback content for key %q", k)
		}
	}
}
