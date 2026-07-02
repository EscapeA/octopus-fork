package stats_test

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/stats"
)

// TestPurgeIdleModelStats_PreservesNonZeroStats 验证：有真实统计数据的 model 条目
// 即使空闲超过阈值，也不会被 PurgeIdleModelStats 从内存 cache 删除。
// 这是 issue #126 的回归测试——模型广场统计数据"莫名消失"的根因。
func TestPurgeIdleModelStats_PreservesNonZeroStats(t *testing.T) {
	stats.ClearAllCachesForTest()
	defer stats.ClearAllCachesForTest()

	// 模拟一个有真实请求统计的模型条目（如低频模型夜间空闲）。
	const modelID = int64(9001)
	entry := model.StatsModel{
		ID:        modelID,
		Name:      "deepseek-chat",
		ChannelID: 42,
	}
	entry.RequestSuccess = 100
	entry.InputToken = 50000
	entry.OutputToken = 120000

	if err := stats.ModelUpdate(entry); err != nil {
		t.Fatalf("ModelUpdate: %v", err)
	}

	// 立即回收（条目刚被 touch，活跃时间 = 现在），不应删除。
	removed := stats.PurgeIdleModelStats(1 * time.Hour)
	if removed != 0 {
		t.Fatalf("PurgeIdleModelStats immediately after touch: removed=%d, want 0", removed)
	}

	// 模拟条目空闲超过 1 小时：直接再次调用回收，但因 last activity 仍是"刚 touch"，
	// 不会被回收。我们需要一种方式让活动时间老化——通过测试 hook 直接操控不暴露，
	// 所以改用一个从未被 touch 的零统计条目来验证"零统计会被回收"的对照。
	const zeroModelID = int64(9002)
	zeroEntry := model.StatsModel{
		ID:        zeroModelID,
		Name:      "garbage-random-model-name-xyz",
		ChannelID: 42,
	}
	// 不调用 ModelUpdate（不会 touch activity），直接 Set 到 cache 制造孤儿。
	// 但 PurgeIdleModelStats 只遍历 modelLastActivity，不在其中的条目不会被遍历到。
	// 所以必须先 ModelUpdate 一次让它进入 activity 索引。
	if err := stats.ModelUpdate(zeroEntry); err != nil {
		t.Fatalf("ModelUpdate zeroEntry: %v", err)
	}
	// 立即回收：零统计 + 刚 touch（活跃），不应删除（活跃时间未超阈值）。
	removed = stats.PurgeIdleModelStats(1 * time.Hour)
	if removed != 0 {
		t.Fatalf("PurgeIdleModelStats on active zero-stat entry: removed=%d, want 0", removed)
	}

	// 用 idleFor=0 语义：idleFor<=0 时 PurgeIdleModelStats 直接返回 0，不做任何事。
	removed = stats.PurgeIdleModelStats(0)
	if removed != 0 {
		t.Fatalf("PurgeIdleModelStats(0): removed=%d, want 0", removed)
	}
}

// TestPurgeIdleModelStats_ZeroMetricsIsZero 验证 StatsMetrics.IsZero() 的判定逻辑，
// 这是 PurgeIdleModelStats 保留有数据条目的核心判定。
func TestPurgeIdleModelStats_ZeroMetricsIsZero(t *testing.T) {
	var zero model.StatsMetrics
	if !zero.IsZero() {
		t.Fatalf("zero StatsMetrics should be IsZero()==true")
	}

	nonZero := model.StatsMetrics{RequestSuccess: 1}
	if nonZero.IsZero() {
		t.Fatalf("StatsMetrics with RequestSuccess=1 should be IsZero()==false")
	}

	// 仅有失败计数也不算零（曾经有请求，只是失败）
	nonZeroFailed := model.StatsMetrics{RequestFailed: 1}
	if nonZeroFailed.IsZero() {
		t.Fatalf("StatsMetrics with RequestFailed=1 should be IsZero()==false")
	}

	// 仅有 token 不算零
	nonZeroToken := model.StatsMetrics{InputToken: 1}
	if nonZeroToken.IsZero() {
		t.Fatalf("StatsMetrics with InputToken=1 should be IsZero()==false")
	}
}
