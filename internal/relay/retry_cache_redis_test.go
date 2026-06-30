package relay

import (
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/lingyuins/octopus/internal/store"
	"github.com/redis/go-redis/v9"
)

// newRelayTestRedis 启动 miniredis 并注入到 store，返回 miniredis 实例（用于快进 TTL）。
func newRelayTestRedis(t *testing.T) *miniredis.Miniredis {
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

func TestFailureHintRedisRecordAndCheck(t *testing.T) {
	newRelayTestRedis(t)
	resetFailureHintCache()

	// recordFailureHint 内部调用 set → 走 KVStore.Set（Redis）。
	recordFailureHint(1, 2, "gpt-4.1",
		RetryDecision{Scope: ScopeSameChannel, Reason: "rate limited", Code: http.StatusTooManyRequests, IsError: true},
		nil, 10)
	// get 走 KVStore.Get（Redis），应命中。
	if _, ok := globalFailureHintCache.get(1, 2, "gpt-4.1"); !ok {
		t.Fatal("expected failure hint to be stored in redis")
	}
}

func TestFailureHintRedisIsolationByKey(t *testing.T) {
	newRelayTestRedis(t)
	resetFailureHintCache()

	recordFailureHint(1, 2, "gpt-4.1",
		RetryDecision{Scope: ScopeSameChannel, Reason: "rate limited", Code: http.StatusTooManyRequests, IsError: true},
		nil, 10)
	// 不同 (channel,key,model) 不应命中。
	if _, ok := globalFailureHintCache.get(1, 2, "claude-3-5-sonnet"); ok {
		t.Fatal("other model should not have a hint")
	}
	if _, ok := globalFailureHintCache.get(1, 3, "gpt-4.1"); ok {
		t.Fatal("other key should not have a hint")
	}
}

func TestFailureHintRedisExpiry(t *testing.T) {
	mr := newRelayTestRedis(t)
	resetFailureHintCache()

	// TTL 1 秒记录后快进 miniredis 时间使其过期。
	recordFailureHint(1, 2, "gpt-4.1",
		RetryDecision{Scope: ScopeSameChannel, Reason: "net err", Code: 0, IsError: true},
		&netErr{}, 1)
	if _, ok := globalFailureHintCache.get(1, 2, "gpt-4.1"); !ok {
		t.Fatal("hint should exist before TTL expiry")
	}
	mr.FastForward(2 * time.Second)
	if _, ok := globalFailureHintCache.get(1, 2, "gpt-4.1"); ok {
		t.Fatal("hint should be expired after TTL")
	}
}

// netErr 实现 net.Error 接口，用于触发 network failure hint 分支。
type netErr struct{}

func (netErr) Error() string   { return "network error" }
func (netErr) Timeout() bool   { return true }
func (netErr) Temporary() bool { return true }
