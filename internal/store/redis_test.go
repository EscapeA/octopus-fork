package store

import (
	"sync"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/conf"
)

// resetState 重置 store 全局状态到「未初始化（内存后端）」，供测试隔离。
// 复用 ResetForTest 但额外确保 client=nil / enabled=false。
func resetState(t *testing.T) {
	t.Helper()
	ResetForTest()
}

// TestInit_ConnectionFailure_DoesNotBlock 验证 Init 连不上时快速返回错误，
// 不会卡 3 分钟（issue #135 核心诉求）。
func TestInit_ConnectionFailure_DoesNotBlock(t *testing.T) {
	resetState(t)
	// 用一个必然 refused 的端口。refused 场景下 go-redis ~1.7s 返回，
	// 远小于 3 分钟。DialTimeout=1s 进一步收紧。
	cfg := conf.RedisConfig{
		Addr:        "127.0.0.1:1", // 无监听，立即 RST
		DialTimeout: 1 * time.Second,
	}
	start := time.Now()
	err := Init(cfg)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("Init should fail on unreachable Redis")
	}
	// 核心断言：不卡 3 分钟。refused + DialTimeout=1s 应在数秒内返回。
	if elapsed > 10*time.Second {
		t.Fatalf("Init blocked too long: %v (issue #135 regression)", elapsed)
	}
	t.Logf("Init failed in %v as expected: %v", elapsed, err)
	resetState(t)
}

// TestInit_EmptyAddr 验证空 addr 返回错误，不 panic。
func TestInit_EmptyAddr(t *testing.T) {
	resetState(t)
	if err := Init(conf.RedisConfig{}); err == nil {
		t.Fatalf("Init with empty addr should error")
	}
	resetState(t)
}

// TestInit_SuccessEnablesRedis 验证连上真实 Redis（miniredis）后 Enabled()=true。
func TestInit_SuccessEnablesRedis(t *testing.T) {
	resetState(t)
	_, mr := newTestRedis(t)
	cfg := conf.RedisConfig{Addr: mr.Addr()}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init miniredis: %v", err)
	}
	if !Enabled() {
		t.Fatalf("Enabled() should be true after successful Init")
	}
	// Init 内部自建 client；resetState 会 Close 并切回内存。
	resetState(t)
}

// TestStartReconnect_AlreadyConnected 验证已连接时拒绝重复启动重连。
func TestStartReconnect_AlreadyConnected(t *testing.T) {
	resetState(t)
	_, mr := newTestRedis(t)
	defer func() { resetState(t) }()
	if err := Init(conf.RedisConfig{Addr: mr.Addr()}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := StartReconnect(conf.RedisConfig{Addr: mr.Addr()}, nil); err == nil {
		t.Fatalf("StartReconnect should error when already connected")
	}
}

// TestStartReconnect_BackgroundRetrySucceeds 验证后台重连最终成功并切换后端。
// 先用错误地址 Init 失败，再启动重连，期间切换 miniredis 地址为可达。
func TestStartReconnect_BackgroundRetrySucceeds(t *testing.T) {
	resetState(t)
	defer resetState(t)

	// 用 miniredis 提供最终可达地址。
	_, mr := newTestRedis(t)

	// 先用一个不可达地址 Init（必然失败），降级到内存后端。
	badCfg := conf.RedisConfig{Addr: "127.0.0.1:1", DialTimeout: 500 * time.Millisecond}
	if err := Init(badCfg); err == nil {
		t.Fatalf("Init bad addr should fail")
	}
	if Enabled() {
		t.Fatalf("should be in memory fallback mode after Init failure")
	}

	// 启动重连，指向 miniredis（可达）。
	goodCfg := conf.RedisConfig{Addr: mr.Addr(), DialTimeout: 1 * time.Second}
	done := make(chan struct{})
	if err := StartReconnect(goodCfg, func() { close(done) }); err != nil {
		t.Fatalf("StartReconnect: %v", err)
	}

	// 等待重连成功（首次退避 2s + ping），最多等 15s。
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatalf("reconnect did not succeed within 15s")
	}
	if !Enabled() {
		t.Fatalf("Enabled() should be true after successful reconnect")
	}
}

// TestGet_ConcurrentSafeDuringSwitch 验证 GetKV() 在 switchToRedis 期间并发
// 调用不会 race（-race 检测）。这是热切换安全性的核心保证。
func TestGet_ConcurrentSafeDuringSwitch(t *testing.T) {
	resetState(t)
	defer resetState(t)

	c, _ := newTestRedis(t)

	var stop atomicBool
	var wg sync.WaitGroup

	// 并发读者：持续调用所有 Get*()。
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.get() {
			_ = GetKV()
			_ = GetRateLimit()
			_ = GetStats()
			_ = GetRuntimeState()
			_ = GetChannelDelay()
		}
	}()

	// 并发切换者：在内存与 Redis 间反复切换。
	for i := 0; i < 5; i++ {
		switchToRedis(c)
		// 切回内存（模拟降级回退场景下的并发访问）。
		mu.Lock()
		client = nil
		enabled = false
		defaultKV = &memoryKV{}
		defaultRateLimit = &memoryRateLimit{}
		defaultStats = &memoryStats{}
		defaultRuntimeState = &memoryRuntimeState{}
		defaultChannelDelay = &memoryChannelDelay{}
		mu.Unlock()
	}

	stop.set(true)
	wg.Wait()
}

// atomicBool 是一个简易的原子布尔值，避免引入 sync/atomic 的繁琐。
type atomicBool struct {
	mu  sync.Mutex
	val bool
}

func (a *atomicBool) get() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.val
}
func (a *atomicBool) set(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.val = v
}

// TestReconnectBackoff 验证退避序列：2s, 4s, 8s, 16s, 30s, 30s...
func TestReconnectBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second},
		{6, 30 * time.Second},
		{100, 30 * time.Second},
	}
	for _, tc := range cases {
		got := reconnectBackoff(tc.attempt)
		if got != tc.want {
			t.Errorf("reconnectBackoff(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}
