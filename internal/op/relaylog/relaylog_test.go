package relaylog

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

func TestRelayLogFlushToDBSkipsDuplicateIDsAndTruncatesCache(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	existing := model.RelayLog{ID: 101, Time: 1, RequestModelName: "existing"}
	if err := db.GetDB().Create(&existing).Error; err != nil {
		t.Fatalf("seed relay log failed: %v", err)
	}

	relayLogCacheLock.Lock()
	relayLogCache = []model.RelayLog{
		{ID: 101, Time: 2, RequestModelName: "duplicate"},
		{ID: 102, Time: 3, RequestModelName: "new"},
	}
	relayLogCacheLock.Unlock()
	t.Cleanup(func() {
		relayLogCacheLock.Lock()
		relayLogCache = make([]model.RelayLog, 0, relayLogMaxSize)
		relayLogCacheLock.Unlock()
	})

	if err := relayLogFlushToDB(context.Background()); err != nil {
		t.Fatalf("relayLogFlushToDB returned error: %v", err)
	}

	var count int64
	if err := db.GetDB().Model(&model.RelayLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count relay logs failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("relay log count = %d, want 2", count)
	}

	var inserted model.RelayLog
	if err := db.GetDB().First(&inserted, "id = ?", 102).Error; err != nil {
		t.Fatalf("new relay log was not inserted: %v", err)
	}

	relayLogCacheLock.Lock()
	cacheLen := len(relayLogCache)
	relayLogCacheLock.Unlock()
	if cacheLen != 0 {
		t.Fatalf("relay log cache len = %d, want 0", cacheLen)
	}
}

func TestRelayLogCleanupAll(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog-cleanup.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Seed DB with a mix of success and error logs
	seed := []model.RelayLog{
		{ID: 401, Time: 1, RequestModelName: "model-a", Error: ""},
		{ID: 402, Time: 2, RequestModelName: "model-b", Error: "timeout"},
		{ID: 403, Time: 3, RequestModelName: "model-c", Error: ""},
	}
	if err := db.GetDB().Create(&seed).Error; err != nil {
		t.Fatalf("seed relay logs failed: %v", err)
	}

	var before int64
	db.GetDB().Model(&model.RelayLog{}).Count(&before)
	if before != 3 {
		t.Fatalf("expected 3 seeded logs, got %d", before)
	}

	if err := relayLogCleanupAll(context.Background()); err != nil {
		t.Fatalf("relayLogCleanupAll returned error: %v", err)
	}

	var after int64
	if err := db.GetDB().Model(&model.RelayLog{}).Count(&after).Error; err != nil {
		t.Fatalf("count after cleanup failed: %v", err)
	}
	if after != 0 {
		t.Fatalf("relay log count = %d, want 0 (all logs should be deleted)", after)
	}
}

func TestRelayLogListExcludesConfiguredGroups(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog-exclude.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}

	// 日志保存关闭：RelayLogList 只读内存缓存，便于断言过滤行为。
	if err := setting.SetString(model.SettingKeyRelayLogKeepEnabled, "false"); err != nil {
		t.Fatalf("disable relay log keep failed: %v", err)
	}
	if err := setting.SetString(model.SettingKeyLogExcludedGroups, `["stress-test"]`); err != nil {
		t.Fatalf("set excluded groups failed: %v", err)
	}

	restore := SetCacheForTest([]model.RelayLog{
		{ID: 1, Time: 1, RequestModelName: "gpt-4"},
		{ID: 2, Time: 2, RequestModelName: "stress-test"},
		{ID: 3, Time: 3, RequestModelName: "claude"},
		{ID: 4, Time: 4, RequestModelName: "stress-test"},
	})
	t.Cleanup(restore)

	logs, err := RelayLogList(context.Background(), nil, nil, 1, 50)
	if err != nil {
		t.Fatalf("RelayLogList returned error: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("RelayLogList returned %d logs, want 2 (stress-test excluded)", len(logs))
	}
	for _, l := range logs {
		if l.RequestModelName == "stress-test" {
			t.Fatalf("excluded group log leaked into result: id=%d", l.ID)
		}
	}

	// 清空屏蔽配置后，全部日志都应返回。
	if err := setting.SetString(model.SettingKeyLogExcludedGroups, "[]"); err != nil {
		t.Fatalf("clear excluded groups failed: %v", err)
	}
	logs, err = RelayLogList(context.Background(), nil, nil, 1, 50)
	if err != nil {
		t.Fatalf("RelayLogList returned error: %v", err)
	}
	if len(logs) != 4 {
		t.Fatalf("RelayLogList returned %d logs, want 4 (no exclusion)", len(logs))
	}
}

func TestRelayLogStreamExcluded(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog-stream-exclude.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}
	if err := setting.SetString(model.SettingKeyLogExcludedGroups, `["stress-test"]`); err != nil {
		t.Fatalf("set excluded groups failed: %v", err)
	}

	if !RelayLogStreamExcluded("stress-test") {
		t.Fatalf("expected stress-test to be excluded from stream")
	}
	if RelayLogStreamExcluded("gpt-4") {
		t.Fatalf("expected gpt-4 to NOT be excluded from stream")
	}
}

func TestRelayLogCleanupAllFastClearAllowsReinsert(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "relaylog-fastclear.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	seed := []model.RelayLog{
		{ID: 1, Time: 1, RequestModelName: "a"},
		{ID: 2, Time: 2, RequestModelName: "b"},
		{ID: 3, Time: 3, RequestModelName: "c"},
	}
	if err := db.GetDB().Create(&seed).Error; err != nil {
		t.Fatalf("seed relay logs failed: %v", err)
	}

	// FastClearTable 走 DROP + AutoMigrate 重建（SQLite）。
	if err := relayLogCleanupAll(context.Background()); err != nil {
		t.Fatalf("relayLogCleanupAll returned error: %v", err)
	}

	var count int64
	if err := db.GetDB().Model(&model.RelayLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count after fast clear failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("relay log count = %d, want 0 after fast clear", count)
	}

	// 重建后表与索引应可正常工作：再次写入并按时间范围查询。
	if err := db.GetDB().Create(&model.RelayLog{ID: 10, Time: 100, RequestModelName: "reinsert"}).Error; err != nil {
		t.Fatalf("reinsert after fast clear failed: %v", err)
	}
	var got model.RelayLog
	if err := db.GetDB().Where("time >= ?", 50).First(&got).Error; err != nil {
		t.Fatalf("query by time index after rebuild failed: %v", err)
	}
	if got.ID != 10 {
		t.Fatalf("reinserted row id = %d, want 10", got.ID)
	}
}

func TestRelayLogSeparateLogDBRoutesWrites(t *testing.T) {
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

	// Seed the in-memory cache and flush; logs must land on the log DB.
	restore := SetCacheForTest([]model.RelayLog{
		{ID: 1, Time: 1, RequestModelName: "m1"},
		{ID: 2, Time: 2, RequestModelName: "m2"},
	})
	t.Cleanup(restore)

	if err := relayLogFlushToDB(context.Background()); err != nil {
		t.Fatalf("relayLogFlushToDB failed: %v", err)
	}

	// Log DB should hold the rows.
	var logCount int64
	if err := db.GetLogDB().Model(&model.RelayLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count log DB failed: %v", err)
	}
	if logCount != 2 {
		t.Fatalf("log DB relay log count = %d, want 2", logCount)
	}

	// Main DB's relay_logs table must remain empty (writes did not leak there).
	var mainCount int64
	if err := db.GetDB().Model(&model.RelayLog{}).Count(&mainCount).Error; err != nil {
		t.Fatalf("count main DB failed: %v", err)
	}
	if mainCount != 0 {
		t.Fatalf("main DB relay log count = %d, want 0 (logs must not leak to main DB)", mainCount)
	}
}

func TestRelayLogApplyKeepEnabledClosesAndReopensLogDB(t *testing.T) {
	mainPath := filepath.Join(t.TempDir(), "main.db")
	if err := db.InitDB("sqlite", mainPath, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "logs.db")
	if err := db.InitLogDB("sqlite", logPath, false); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Disabling logs clears and disconnects the separate log DB.
	if err := ApplyKeepEnabledChange(context.Background(), false); err != nil {
		t.Fatalf("ApplyKeepEnabledChange(false) failed: %v", err)
	}
	if db.GetLogDB() != nil {
		t.Fatalf("GetLogDB() should be nil after disabling logs in separate mode")
	}

	// Re-enabling reconnects.
	if err := ApplyKeepEnabledChange(context.Background(), true); err != nil {
		t.Fatalf("ApplyKeepEnabledChange(true) failed: %v", err)
	}
	if db.GetLogDB() == nil {
		t.Fatalf("GetLogDB() should be non-nil after re-enabling logs")
	}
}
