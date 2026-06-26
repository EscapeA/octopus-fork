package relaylog

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// TestRelayLogClearContents 验证清空大字段：
//   - request_content / response_content 被置空；
//   - 元数据（token、cost、渠道、时间）保留；
//   - 内存缓存中的大字段也被清空。
func TestRelayLogClearContents(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "clear-contents.db")
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

	// 种入两条带大字段的日志。
	logs := []model.RelayLog{
		{ID: 1, Time: 100, RequestModelName: "gpt-4o", ChannelId: 5, ChannelName: "ch5",
			InputTokens: 100, OutputTokens: 50, Cost: 0.01,
			RequestContent:  `{"messages":[{"role":"user","content":"hello"}]}`,
			ResponseContent: `{"choices":[{"message":{"content":"hi"}}]}`,
		},
		{ID: 2, Time: 200, RequestModelName: "claude", ChannelId: 6, ChannelName: "ch6",
			InputTokens: 200, OutputTokens: 80, Cost: 0.02,
			RequestContent:  `{"messages":[{"role":"user","content":"world"}]}`,
			ResponseContent: `{"choices":[{"message":{"content":"hey"}}]}`,
		},
	}
	conn := db.GetLogDB()
	if conn == nil {
		t.Fatalf("GetLogDB returned nil")
	}
	if err := conn.Create(&logs).Error; err != nil {
		t.Fatalf("seed logs failed: %v", err)
	}

	// 放一条带大字段的日志进内存缓存，验证缓存也被清空。
	restore := SetCacheForTest([]model.RelayLog{
		{ID: 3, Time: 300, RequestModelName: "cached", RequestContent: "req-body", ResponseContent: "resp-body"},
	})
	t.Cleanup(restore)

	if err := RelayLogClearContents(context.Background()); err != nil {
		t.Fatalf("RelayLogClearContents error: %v", err)
	}

	// 验证数据库：大字段为空，元数据保留。
	var rows []model.RelayLog
	if err := conn.Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("query logs failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for i, r := range rows {
		if r.RequestContent != "" {
			t.Fatalf("row %d: RequestContent = %q, want empty", i, r.RequestContent)
		}
		if r.ResponseContent != "" {
			t.Fatalf("row %d: ResponseContent = %q, want empty", i, r.ResponseContent)
		}
	}
	// 元数据保留检查。
	if rows[0].InputTokens != 100 || rows[0].ChannelName != "ch5" {
		t.Fatalf("row 0 metadata lost: tokens=%d, channel=%q", rows[0].InputTokens, rows[0].ChannelName)
	}
	if rows[1].Cost != 0.02 || rows[1].RequestModelName != "claude" {
		t.Fatalf("row 1 metadata lost: cost=%f, model=%q", rows[1].Cost, rows[1].RequestModelName)
	}

	// 验证内存缓存：大字段也被清空。
	cache, _ := GetCacheAndLock()
	if len(cache) == 0 {
		t.Fatal("expected cached log to remain")
	}
	cached := cache[len(cache)-1]
	if cached.RequestContent != "" || cached.ResponseContent != "" {
		t.Fatalf("cache not cleared: req=%q resp=%q", cached.RequestContent, cached.ResponseContent)
	}
	// 缓存元数据保留。
	if cached.ID != 3 || cached.RequestModelName != "cached" {
		t.Fatalf("cache metadata lost: id=%d, model=%q", cached.ID, cached.RequestModelName)
	}
}
