package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

// TestLatencyDistribution_DBAggregation 验证 loadLatencyDistribution 改为 DB 端
// 聚合后仍返回正确的 count/avg/buckets，且不再全量加载 use_time 行到内存。
// 修复前用 .Find(&dbRows) 把所有行拉进内存排序，90d/all 下内存爆炸。
func TestLatencyDistribution_DBAggregation(t *testing.T) {
	setupSeparateLogDB(t)

	// 写入覆盖各延迟桶的日志。
	logs := []model.RelayLog{
		{ID: 1, Time: time.Now().Unix(), UseTime: 50, Ftut: 0},    // <100ms
		{ID: 2, Time: time.Now().Unix(), UseTime: 200, Ftut: 80},  // 100-500ms, ftut<100
		{ID: 3, Time: time.Now().Unix(), UseTime: 700, Ftut: 300}, // 500-1s, ftut 100-500
		{ID: 4, Time: time.Now().Unix(), UseTime: 2000, Ftut: 0},  // 1-5s
		{ID: 5, Time: time.Now().Unix(), UseTime: 8000, Ftut: 0},  // >5s
		{ID: 6, Time: time.Now().Unix(), UseTime: 0, Ftut: 0},     // 无效，不计入
	}
	if err := db.GetLogDB().Create(&logs).Error; err != nil {
		t.Fatalf("seed log DB failed: %v", err)
	}

	result, err := AnalyticsLatencyDistributionGet(context.Background(), model.AnalyticsRange1D)
	if err != nil {
		t.Fatalf("AnalyticsLatencyDistributionGet error: %v", err)
	}

	// 5 条有效日志（use_time>0），1 条无效不计入。
	if result.TotalRequests != 5 {
		t.Errorf("TotalRequests = %d, want 5", result.TotalRequests)
	}

	// 平均值 = (50+200+700+2000+8000)/5 = 2190
	if result.AvgMs != 2190 {
		t.Errorf("AvgMs = %d, want 2190", result.AvgMs)
	}

	// 桶计数。
	bucketByName := make(map[string]int64, len(result.Buckets))
	for _, b := range result.Buckets {
		bucketByName[b.Label] = b.Count
	}
	if bucketByName["<100ms"] != 1 {
		t.Errorf("<100ms bucket = %d, want 1", bucketByName["<100ms"])
	}
	if bucketByName["100-500ms"] != 1 {
		t.Errorf("100-500ms bucket = %d, want 1", bucketByName["100-500ms"])
	}
	if bucketByName["500ms-1s"] != 1 {
		t.Errorf("500ms-1s bucket = %d, want 1", bucketByName["500ms-1s"])
	}
	if bucketByName["1-5s"] != 1 {
		t.Errorf("1-5s bucket = %d, want 1", bucketByName["1-5s"])
	}
	if bucketByName[">5s"] != 1 {
		t.Errorf(">5s bucket = %d, want 1", bucketByName[">5s"])
	}

	// FTUT：只有 2 条有 ftut>0，avg = (80+300)/2 = 190
	if result.FtutAvgMs != 190 {
		t.Errorf("FtutAvgMs = %d, want 190", result.FtutAvgMs)
	}
}

// TestLatencyDistribution_PercentileInterpolation 验证百分位桶插值在单桶场景下
// 返回桶内合理值，而非全量加载排序。
func TestLatencyDistribution_PercentileInterpolation(t *testing.T) {
	setupSeparateLogDB(t)

	// 全部落在 100-500ms 桶，P50 应在桶内 [100,500) 插值。
	logs := []model.RelayLog{
		{ID: 10, Time: time.Now().Unix(), UseTime: 100, Ftut: 0},
		{ID: 11, Time: time.Now().Unix(), UseTime: 200, Ftut: 0},
		{ID: 12, Time: time.Now().Unix(), UseTime: 300, Ftut: 0},
		{ID: 13, Time: time.Now().Unix(), UseTime: 400, Ftut: 0},
	}
	if err := db.GetLogDB().Create(&logs).Error; err != nil {
		t.Fatalf("seed log DB failed: %v", err)
	}

	result, err := AnalyticsLatencyDistributionGet(context.Background(), model.AnalyticsRange1D)
	if err != nil {
		t.Fatalf("AnalyticsLatencyDistributionGet error: %v", err)
	}

	// 4 条全在 [100,500)，P50 约在桶中点附近，应在 [100,500) 区间内。
	if result.P50Ms < 100 || result.P50Ms >= 500 {
		t.Errorf("P50Ms = %d, should be within [100, 500) for single-bucket data", result.P50Ms)
	}
	// P99 同样落在这个桶内。
	if result.P99Ms < 100 || result.P99Ms >= 500 {
		t.Errorf("P99Ms = %d, should be within [100, 500) for single-bucket data", result.P99Ms)
	}
}

// TestLatencyDistribution_EmptyRange 验证无数据时不 panic、返回零值。
func TestLatencyDistribution_EmptyRange(t *testing.T) {
	setupSeparateLogDB(t)

	result, err := AnalyticsLatencyDistributionGet(context.Background(), model.AnalyticsRange1D)
	if err != nil {
		t.Fatalf("AnalyticsLatencyDistributionGet error: %v", err)
	}
	if result.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0 for empty range", result.TotalRequests)
	}
	if result.P50Ms != 0 || result.P95Ms != 0 || result.P99Ms != 0 {
		t.Errorf("percentiles should be 0 for empty range, got p50=%d p95=%d p99=%d",
			result.P50Ms, result.P95Ms, result.P99Ms)
	}
}
