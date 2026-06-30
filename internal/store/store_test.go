package store

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRedis 启动一个 miniredis 实例并返回已连接的 client 与清理函数。
// miniredis 是纯 Go 的内嵌 Redis，支持 KEYS/SCAN/Hash/Lua 等本实现用到的命令，
// 无需外部 Redis 进程，适合单测。
func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = c.Close()
		mr.Close()
	})
	return c, mr
}

// InjectRedisClient 直接注入一个已连接的 client 并切换所有后端到 Redis 实现，
// 清理时恢复为内存实现。供各子系统包的单测使用（绕过 Init 的 ping）。
// 用法：
//
//	c, mr := store.NewTestRedis(t)
//	store.InjectRedisClient(t, c)
//	defer mr.Close()
func InjectRedisClient(t *testing.T, c *redis.Client) {
	t.Helper()
	mu.Lock()
	origClient := client
	origEnabled := enabled
	origKV := defaultKV
	origRL := defaultRateLimit
	origStats := defaultStats
	origRT := defaultRuntimeState
	origCD := defaultChannelDelay
	client = c
	enabled = true
	defaultKV = newRedisKV(c)
	defaultRateLimit = newRedisRateLimit(c)
	defaultStats = newRedisStats(c)
	defaultRuntimeState = newRedisRuntimeState(c)
	defaultChannelDelay = newRedisChannelDelay(c)
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		client = origClient
		enabled = origEnabled
		defaultKV = origKV
		defaultRateLimit = origRL
		defaultStats = origStats
		defaultRuntimeState = origRT
		defaultChannelDelay = origCD
		mu.Unlock()
	})
}

// --- KVStore 契约测试（memory vs redis 一致性） ---

func runKVContract(t *testing.T, s KVStore) {
	t.Helper()
	ctx := context.Background()
	key := "fhint:1:2:gpt-4"
	val := []byte(`{"decision":"fail","code":429}`)

	// 初始 miss
	got, found, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get miss: %v", err)
	}
	if found {
		t.Fatalf("Get on missing key should return found=false")
	}
	if got != nil {
		t.Fatalf("Get on missing key should return nil val")
	}

	// Set + Get hit
	if err := s.Set(ctx, key, val, 10*time.Second); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, found, err = s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get hit: %v", err)
	}
	if !found {
		t.Fatalf("Get after Set should return found=true")
	}
	if string(got) != string(val) {
		t.Fatalf("Get returned %q, want %q", got, val)
	}

	// Del
	if err := s.Del(ctx, key); err != nil {
		t.Fatalf("Del: %v", err)
	}
	_, found, _ = s.Get(ctx, key)
	if found {
		t.Fatalf("Get after Del should return found=false")
	}

	// DelByPrefix：写入 2 个同前缀 key + 1 个不同前缀 key，删前缀后只剩后者。
	prefix := "cooldown:5:1:"
	toDelete := []string{"cooldown:5:1:modelA", "cooldown:5:1:modelB"}
	keep := "cooldown:6:1:modelD" // 不同前缀，应保留
	for _, k := range toDelete {
		if err := s.Set(ctx, k, []byte("x"), 30*time.Second); err != nil {
			t.Fatalf("Set prefix test: %v", err)
		}
	}
	if err := s.Set(ctx, keep, []byte("x"), 30*time.Second); err != nil {
		t.Fatalf("Set keep: %v", err)
	}
	if err := s.DelByPrefix(ctx, prefix); err != nil {
		t.Fatalf("DelByPrefix: %v", err)
	}
	for _, k := range toDelete {
		_, found, _ := s.Get(ctx, k)
		if found {
			t.Fatalf("key %q should have been deleted by DelByPrefix", k)
		}
	}
	_, found, _ = s.Get(ctx, keep)
	if !found {
		t.Fatalf("key %q should survive DelByPrefix of different prefix", keep)
	}
}

func TestMemoryKVContract(t *testing.T) {
	runKVContract(t, &memoryKV{})
}

func TestRedisKVContract(t *testing.T) {
	c, _ := newTestRedis(t)
	runKVContract(t, newRedisKV(c))
}

// --- KV TTL 过期测试（仅 Redis；memory 实现惰性过期由调用方处理） ---

func TestRedisKVTTLExpiry(t *testing.T) {
	c, mr := newTestRedis(t)
	s := newRedisKV(c)
	ctx := context.Background()
	if err := s.Set(ctx, "k", []byte("v"), 100*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	_, found, _ := s.Get(ctx, "k")
	if !found {
		t.Fatalf("key should exist before TTL")
	}
	// miniredis 不会按真实时间自动过期，需 FastForward 推进其内部时钟。
	mr.FastForward(200 * time.Millisecond)
	_, found, _ = s.Get(ctx, "k")
	if found {
		t.Fatalf("key should be expired after TTL")
	}
}
