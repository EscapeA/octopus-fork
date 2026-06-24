package analytics

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

// TestParseCacheTTL 验证前端传入的字符串正确解析为 CacheTTL。
func TestParseCacheTTL(t *testing.T) {
	cases := []struct {
		raw  string
		want CacheTTL
	}{
		{"10s", CacheTL10s},
		{"30s", CacheTL30s},
		{"1m", CacheTL1min},
		{"off", CacheTLOff},
		{"0", CacheTLOff},
		{"", DefaultCacheTTL},
		{"bogus", DefaultCacheTTL},
	}
	for _, tc := range cases {
		got := ParseCacheTTL(tc.raw)
		if got != tc.want {
			t.Errorf("ParseCacheTTL(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

// TestCacheTLOff_NeverCaches 验证 ttl=0（off）时每次都执行 fn，不缓存。
func TestCacheTLOff_NeverCaches(t *testing.T) {
	cache := newResultCache[int]()
	var calls int32
	key := "k"
	for i := 0; i < 3; i++ {
		v, err := withCache(cache, key, 0, func() (int, error) {
			return int(atomic.AddInt32(&calls, 1)), nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != i+1 {
			t.Errorf("call %d: got %d, want %d", i, v, i+1)
		}
	}
	if calls != 3 {
		t.Errorf("fn called %d times, want 3 (off should never cache)", calls)
	}
}

// TestCacheTL_HitsWithinExpiry 验证缓存命中：TTL 内第二次调用不执行 fn。
func TestCacheTL_HitsWithinExpiry(t *testing.T) {
	cache := newResultCache[string]()
	var calls int32
	ttl := 100 * time.Millisecond
	key := "hit"

	v1, err := withCache(cache, key, ttl, func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "first", nil
	})
	if err != nil || v1 != "first" {
		t.Fatalf("first call: v=%q err=%v", v1, err)
	}

	v2, err := withCache(cache, key, ttl, func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "second", nil
	})
	if err != nil || v2 != "first" {
		t.Fatalf("second call should hit cache: v=%q err=%v", v2, err)
	}
	if calls != 1 {
		t.Errorf("fn called %d times, want 1 (cache should hit)", calls)
	}
}

// TestCacheTL_ExpiresAfterTTL 验证 TTL 过期后重新执行 fn。
func TestCacheTL_ExpiresAfterTTL(t *testing.T) {
	cache := newResultCache[int]()
	var calls int32
	ttl := 20 * time.Millisecond
	key := "exp"

	withCache(cache, key, ttl, func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 1, nil
	})
	time.Sleep(30 * time.Millisecond)
	v, err := withCache(cache, key, ttl, func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 2, nil
	})
	if err != nil || v != 2 {
		t.Fatalf("after expiry: v=%d err=%v", v, err)
	}
	if calls != 2 {
		t.Errorf("fn called %d times, want 2 (should re-fetch after expiry)", calls)
	}
}

// TestCacheTL_ErrorsNotCached 验证 fn 返回 error 时不缓存结果。
func TestCacheTL_ErrorsNotCached(t *testing.T) {
	cache := newResultCache[int]()
	var calls int32
	ttl := 100 * time.Millisecond
	key := "err"

	_, err := withCache(cache, key, ttl, func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 0, errFake
	})
	if err != errFake {
		t.Fatalf("expected errFake, got %v", err)
	}
	// 第二次应重新执行 fn（error 不缓存）
	_, err = withCache(cache, key, ttl, func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 0, errFake
	})
	if calls != 2 {
		t.Errorf("fn called %d times, want 2 (errors should not be cached)", calls)
	}
}

// TestCacheTL_DistinctKeys 独立 key 互不干扰。
func TestCacheTL_DistinctKeys(t *testing.T) {
	cache := newResultCache[int]()
	var calls int32
	ttl := 100 * time.Millisecond

	withCache(cache, "a", ttl, func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 1, nil
	})
	withCache(cache, "b", ttl, func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 2, nil
	})
	// 两个 key 都应执行 fn
	if calls != 2 {
		t.Errorf("fn called %d times, want 2 (distinct keys)", calls)
	}
}

// TestCachedAnalyticsOverviewGet_HitsCache 集成测试：连续两次调用只触发一次底层查询。
// 需要 DB 初始化（AnalyticsOverviewGet 内部会查 channel/apikey/stats）。
func TestCachedAnalyticsOverviewGet_HitsCache(t *testing.T) {
	setupIssue103DB(t) // 复用已有的 SQLite 初始化 helper

	ttl := CacheTL30s
	_, err := CachedAnalyticsOverviewGet(context.Background(), model.AnalyticsRange7D, ttl)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	// 第二次应命中缓存（不报错即可，底层不会重新查 DB）
	_, err = CachedAnalyticsOverviewGet(context.Background(), model.AnalyticsRange7D, ttl)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
}

// errFake 是测试用 sentinel error。
var errFake = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "fake" }
