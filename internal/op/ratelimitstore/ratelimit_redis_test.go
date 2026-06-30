package ratelimitstore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/lingyuins/octopus/internal/store"
	"github.com/redis/go-redis/v9"
)

// newRLTestRedis 启动 miniredis 并注入到 store，返回 miniredis 实例（用于快进 TTL）。
func newRLTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store.InjectForTest(c)
	t.Cleanup(func() {
		_ = c.Close()
		store.ResetForTest()
		mr.Close()
	})
	return mr
}

func TestRateLimitRedisAllowsUnderLimit(t *testing.T) {
	newRLTestRedis(t)
	// rpm=10：前 10 次应放行。
	for i := 0; i < 10; i++ {
		allowed, _, _ := CheckRateLimit(1, "gpt-4o", 10, 0, 0)
		if !allowed {
			t.Fatalf("request %d should be allowed under rpm=10", i+1)
		}
	}
	// 第 11 次应被拒。
	if allowed, _, _ := CheckRateLimit(1, "gpt-4o", 10, 0, 0); allowed {
		t.Fatal("request 11 should be denied over rpm=10")
	}
}

func TestRateLimitRedisIsolatedByKeyModel(t *testing.T) {
	newRLTestRedis(t)
	// 用尽 apiKey=1 model=gpt-4o 的配额。
	for i := 0; i < 5; i++ {
		CheckRateLimit(1, "gpt-4o", 5, 0, 0)
	}
	// 不同 model 同 apiKey 不应受影响。
	if allowed, _, _ := CheckRateLimit(1, "claude-3", 5, 0, 0); !allowed {
		t.Fatal("different model should not be rate-limited")
	}
	// 不同 apiKey 同 model 不应受影响。
	if allowed, _, _ := CheckRateLimit(2, "gpt-4o", 5, 0, 0); !allowed {
		t.Fatal("different apiKey should not be rate-limited")
	}
}

func TestRateLimitRedisRefill(t *testing.T) {
	// 注意：Redis token bucket 用 time.Now()（进程真实时间）计算 refill，
	// miniredis.FastForward 只推进 miniredis 时钟，不影响 time.Now()。
	// 直接测 store 层以便指定 burst=1：rate=60 rpm → 1 token/秒，burst=1。
	rl := store.GetRateLimit()
	ctx := context.Background()
	// 第一次：burst=1，消耗 1 token，桶空。
	if ok, _, _, err := rl.CheckAndConsume(ctx, "rl:req:1:refill", 60, 1, 1); err != nil || !ok {
		t.Fatalf("first consume: ok=%v err=%v", ok, err)
	}
	// 立即第二次：应被拒（桶空，elapsed 不足以 refill 1 token）。
	if ok, _, _, _ := rl.CheckAndConsume(ctx, "rl:req:1:refill", 60, 1, 1); ok {
		t.Fatal("second consume should be denied (bucket empty)")
	}
	// 等待 > 1 秒让真实时间 refill 1 token（rate=1 token/秒）。
	time.Sleep(1100 * time.Millisecond)
	if ok, _, _, _ := rl.CheckAndConsume(ctx, "rl:req:1:refill", 60, 1, 1); !ok {
		t.Fatal("request after refill should be allowed")
	}
}

func TestRateLimitRedisRemoveByAPIKey(t *testing.T) {
	newRLTestRedis(t)
	for i := 0; i < 5; i++ {
		CheckRateLimit(7, "gpt-4o", 5, 0, 0)
	}
	// apiKey=7 已耗尽。
	if allowed, _, _ := CheckRateLimit(7, "gpt-4o", 5, 0, 0); allowed {
		t.Fatal("apiKey 7 should be exhausted")
	}
	RemoveAPIKeyBuckets(7)
	// 删除后应重新放行（bucket 重置为满）。
	if allowed, _, _ := CheckRateLimit(7, "gpt-4o", 5, 0, 0); !allowed {
		t.Fatal("apiKey 7 should be allowed after RemoveAPIKeyBuckets")
	}
}
