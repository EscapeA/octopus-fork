package ratelimitstore

import (
	"sync"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/utils/ratelimit"
)

// resetBuckets 清空全局 bucket map，返回恢复函数。
func resetBuckets() func() {
	snapshotReq := map[string]*ratelimit.TokenBucket{}
	snapshotTok := map[string]*ratelimit.TokenBucket{}
	requestBuckets.Range(func(k, v any) bool {
		snapshotReq[k.(string)] = v.(*ratelimit.TokenBucket)
		requestBuckets.Delete(k)
		return true
	})
	tokenBuckets.Range(func(k, v any) bool {
		snapshotTok[k.(string)] = v.(*ratelimit.TokenBucket)
		tokenBuckets.Delete(k)
		return true
	})
	return func() {
		for k, v := range snapshotReq {
			requestBuckets.Store(k, v)
		}
		for k, v := range snapshotTok {
			tokenBuckets.Store(k, v)
		}
	}
}

// TestPurgeStaleBuckets_RemovesIdle 验证 PurgeStaleBuckets 删除空闲超时的 bucket，
// 保留活跃 bucket。用 maxAge=1ns 确保刚创建的 bucket 也被判为陈旧（因为
// PurgeStaleBuckets 内部调用 time.Now()，bucket 的 lastUpdate 早于该 now）。
//
// 修复前 requestBuckets/tokenBuckets 只增不删，key 含客户端 modelName（基数不受控），
// 刷量/随机 model 名下无界增长（issue #46 同类遗漏）。
func TestPurgeStaleBuckets_RemovesIdle(t *testing.T) {
	restore := resetBuckets()
	defer restore()

	CheckRateLimit(1, "model-a", 60, 0, 0)
	CheckRateLimit(2, "model-b", 60, 0, 0)

	// 确认两个 bucket 都存在。
	if n := countBuckets(&requestBuckets); n != 2 {
		t.Fatalf("requestBuckets count = %d, want 2 before purge", n)
	}
	// 等待 1ms 确保 bucket 的 lastUpdate 早于 purge 调用时的 now - 1ms。
	time.Sleep(time.Millisecond)

	// maxAge=1ms：bucket.lastUpdate 距 purge 的 now 已超过 1ms，判为陈旧删除。
	removed := PurgeStaleBuckets(time.Millisecond)
	if removed < 2 {
		t.Fatalf("PurgeStaleBuckets(1ms) removed %d, want >= 2", removed)
	}
	if n := countBuckets(&requestBuckets); n != 0 {
		t.Errorf("requestBuckets count = %d after purge, want 0", n)
	}
}

// TestPurgeStaleBuckets_RetainsFresh 验证活跃 bucket 在合理 maxAge 下被保留。
func TestPurgeStaleBuckets_RetainsFresh(t *testing.T) {
	restore := resetBuckets()
	defer restore()

	CheckRateLimit(1, "fresh-model", 60, 0, 0)

	// 1 小时 maxAge：刚创建的 bucket 应保留。
	removed := PurgeStaleBuckets(time.Hour)
	if removed != 0 {
		t.Errorf("PurgeStaleBuckets(1h) removed %d, want 0 (all fresh)", removed)
	}
	if _, ok := requestBuckets.Load(rateLimitKey(1, "fresh-model")); !ok {
		t.Error("fresh bucket should be retained")
	}
}

// TestPurgeStaleBuckets_ZeroAge 验证 maxAge<=0 时为空操作。
func TestPurgeStaleBuckets_ZeroAge(t *testing.T) {
	restore := resetBuckets()
	defer restore()

	CheckRateLimit(1, "model-a", 60, 0, 0)

	if removed := PurgeStaleBuckets(0); removed != 0 {
		t.Errorf("PurgeStaleBuckets(0) removed %d, want 0", removed)
	}
	if removed := PurgeStaleBuckets(-time.Second); removed != 0 {
		t.Errorf("PurgeStaleBuckets(-1s) removed %d, want 0", removed)
	}
}

// TestRemoveAPIKeyBuckets 验证删除 API key 时清理其所有 bucket（跨模型）。
// 修复前 API key 删除只清 balancer sticky session，不清理 ratelimitstore bucket。
func TestRemoveAPIKeyBuckets(t *testing.T) {
	restore := resetBuckets()
	defer restore()

	// key 1 有两个模型，key 2 有一个模型。
	CheckRateLimit(1, "model-a", 60, 0, 0)
	CheckRateLimit(1, "model-b", 60, 0, 0)
	CheckRateLimit(2, "model-c", 60, 0, 0)

	RemoveAPIKeyBuckets(1)

	// key 1 的所有 bucket 应被删除。
	if _, ok := requestBuckets.Load(rateLimitKey(1, "model-a")); ok {
		t.Error("key 1 model-a bucket should be removed")
	}
	if _, ok := requestBuckets.Load(rateLimitKey(1, "model-b")); ok {
		t.Error("key 1 model-b bucket should be removed")
	}
	// key 2 的 bucket 应保留。
	if _, ok := requestBuckets.Load(rateLimitKey(2, "model-c")); !ok {
		t.Error("key 2 model-c bucket should be retained")
	}
}

// TestRemoveAPIKeyBuckets_InvalidID 验证 apiKeyID<=0 时空操作。
func TestRemoveAPIKeyBuckets_InvalidID(t *testing.T) {
	restore := resetBuckets()
	defer restore()

	CheckRateLimit(5, "model-x", 60, 0, 0)
	RemoveAPIKeyBuckets(0)
	RemoveAPIKeyBuckets(-1)

	if _, ok := requestBuckets.Load(rateLimitKey(5, "model-x")); !ok {
		t.Error("bucket should be retained when apiKeyID <= 0")
	}
}

// countBuckets 统计 sync.Map 中的条目数。
func countBuckets(m *sync.Map) int {
	n := 0
	m.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}
