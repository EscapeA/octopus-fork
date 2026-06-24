package analytics

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

// CacheTTL 表示分析中心查询结果缓存的生存时间。前端可切换 10s/30s/1m/off。
// off (0) 表示禁用缓存，每次查询都直接命中 DB。
type CacheTTL time.Duration

const (
	CacheTLOff  CacheTTL = 0
	CacheTL10s  CacheTTL = CacheTTL(10 * time.Second)
	CacheTL30s  CacheTTL = CacheTTL(30 * time.Second)
	CacheTL1min CacheTTL = CacheTTL(60 * time.Second)

	// DefaultCacheTTL 是未显式指定时的默认缓存生存时间。
	DefaultCacheTTL = CacheTL30s
)

// ParseCacheTTL 把前端传入的字符串解析为 CacheTTL。空串或未知值回退到默认。
func ParseCacheTTL(raw string) CacheTTL {
	switch raw {
	case "off", "0":
		return CacheTLOff
	case "10s":
		return CacheTL10s
	case "30s":
		return CacheTL30s
	case "1m":
		return CacheTL1min
	default:
		return DefaultCacheTTL
	}
}

// (string) 方法让 CacheTTL 可作为 fmt/Stringer 用于日志。
func (t CacheTTL) String() string {
	switch t {
	case CacheTLOff:
		return "off"
	case CacheTL10s:
		return "10s"
	case CacheTL30s:
		return "30s"
	case CacheTL1min:
		return "1m"
	default:
		return time.Duration(t).String()
	}
}

// cacheEntry 存一个缓存结果及其过期时间。
type cacheEntry[T any] struct {
	value T
	exp   time.Time
}

// resultCache 是按 key 分桶的泛型短期缓存。每个 key 独立持有一份结果 + 过期时间。
// 当 ttl <= 0 时所有操作直通（不缓存），保证"关缓存"语义零开销。
//
// 设计参照 internal/op/ops/ops.go 的 providerPromptCacheResult / telemetryLogsCache
// 模式（sync.RWMutex + exp time.Time），但泛型化以复用于多种返回类型。
type resultCache[T any] struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry[T]
}

func newResultCache[T any]() *resultCache[T] {
	return &resultCache[T]{entries: make(map[string]cacheEntry[T])}
}

// get 返回缓存结果及是否命中。ttl <= 0 时永远未命中（直通）。
func (c *resultCache[T]) get(key string, now time.Time, ttl time.Duration) (T, bool) {
	if ttl <= 0 {
		var zero T
		return zero, false
	}
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || !now.Before(entry.exp) {
		var zero T
		return zero, false
	}
	return entry.value, true
}

// set 写入缓存项。ttl <= 0 时为空操作（直通模式不落盘）。
func (c *resultCache[T]) set(key string, value T, now time.Time, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	c.entries[key] = cacheEntry[T]{value: value, exp: now.Add(ttl)}
	c.mu.Unlock()
}

// withCache 是核心包装：先查缓存，命中则直接返回；未命中则执行 fn，
// 把结果写入缓存后返回。fn 的 error 一律不缓存（避免缓存错误态）。
func withCache[T any](c *resultCache[T], key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	now := time.Now()
	if v, ok := c.get(key, now, ttl); ok {
		return v, nil
	}
	v, err := fn()
	if err != nil {
		return v, err
	}
	c.set(key, v, now, ttl)
	return v, nil
}

// ---- 每种返回类型一个缓存实例（避免泛型 map 里类型断言） ----

var (
	overviewCache     = newResultCache[*model.AnalyticsOverview]()
	utilizationCache  = newResultCache[*model.AnalyticsUtilization]()
	providerCache     = newResultCache[[]model.AnalyticsProviderBreakdownItem]()
	modelCache        = newResultCache[[]model.AnalyticsModelBreakdownItem]()
	apiKeyCache       = newResultCache[[]model.AnalyticsAPIKeyBreakdownItem]()
	channelModelCache = newResultCache[[]model.AnalyticsChannelModelItem]()
	latencyCache      = newResultCache[*model.LatencyDistribution]()
	groupHealthCache  = newResultCache[[]model.AnalyticsGroupHealthItem]()
)

// CachedAnalyticsOverviewGet 是 AnalyticsOverviewGet 的缓存包装。
func CachedAnalyticsOverviewGet(ctx context.Context, r model.AnalyticsRange, ttl CacheTTL) (*model.AnalyticsOverview, error) {
	return withCache(overviewCache, string(r), time.Duration(ttl), func() (*model.AnalyticsOverview, error) {
		return AnalyticsOverviewGet(ctx, r)
	})
}

// CachedAnalyticsUtilizationGet 是 AnalyticsUtilizationGet 的缓存包装。
func CachedAnalyticsUtilizationGet(ctx context.Context, r model.AnalyticsRange, ttl CacheTTL) (*model.AnalyticsUtilization, error) {
	return withCache(utilizationCache, string(r), time.Duration(ttl), func() (*model.AnalyticsUtilization, error) {
		return AnalyticsUtilizationGet(ctx, r)
	})
}

// CachedAnalyticsProviderBreakdownGet 是 AnalyticsProviderBreakdownGet 的缓存包装。
func CachedAnalyticsProviderBreakdownGet(ctx context.Context, r model.AnalyticsRange, ttl CacheTTL) ([]model.AnalyticsProviderBreakdownItem, error) {
	return withCache(providerCache, string(r), time.Duration(ttl), func() ([]model.AnalyticsProviderBreakdownItem, error) {
		return AnalyticsProviderBreakdownGet(ctx, r)
	})
}

// CachedAnalyticsModelBreakdownGet 是 AnalyticsModelBreakdownGet 的缓存包装。
func CachedAnalyticsModelBreakdownGet(ctx context.Context, r model.AnalyticsRange, ttl CacheTTL) ([]model.AnalyticsModelBreakdownItem, error) {
	return withCache(modelCache, string(r), time.Duration(ttl), func() ([]model.AnalyticsModelBreakdownItem, error) {
		return AnalyticsModelBreakdownGet(ctx, r)
	})
}

// CachedAnalyticsAPIKeyBreakdownGet 是 AnalyticsAPIKeyBreakdownGet 的缓存包装。
func CachedAnalyticsAPIKeyBreakdownGet(ctx context.Context, r model.AnalyticsRange, ttl CacheTTL) ([]model.AnalyticsAPIKeyBreakdownItem, error) {
	return withCache(apiKeyCache, string(r), time.Duration(ttl), func() ([]model.AnalyticsAPIKeyBreakdownItem, error) {
		return AnalyticsAPIKeyBreakdownGet(ctx, r)
	})
}

// CachedAnalyticsChannelModelBreakdownGet 是 AnalyticsChannelModelBreakdownGet 的缓存包装。
// groupID 参与 cache key，不同分组的查询独立缓存。
func CachedAnalyticsChannelModelBreakdownGet(ctx context.Context, r model.AnalyticsRange, groupID *int, ttl CacheTTL) ([]model.AnalyticsChannelModelItem, error) {
	key := string(r)
	if groupID != nil {
		key += "|g=" + strconv.Itoa(*groupID)
	}
	return withCache(channelModelCache, key, time.Duration(ttl), func() ([]model.AnalyticsChannelModelItem, error) {
		return AnalyticsChannelModelBreakdownGet(ctx, r, groupID)
	})
}

// CachedAnalyticsLatencyDistributionGet 是 AnalyticsLatencyDistributionGet 的缓存包装。
func CachedAnalyticsLatencyDistributionGet(ctx context.Context, r model.AnalyticsRange, ttl CacheTTL) (*model.LatencyDistribution, error) {
	return withCache(latencyCache, string(r), time.Duration(ttl), func() (*model.LatencyDistribution, error) {
		return AnalyticsLatencyDistributionGet(ctx, r)
	})
}

// CachedAnalyticsGroupHealthGet 是 AnalyticsGroupHealthGet 的缓存包装。
// group-health 无 range 参数，使用固定 key。
func CachedAnalyticsGroupHealthGet(ctx context.Context, ttl CacheTTL) ([]model.AnalyticsGroupHealthItem, error) {
	return withCache(groupHealthCache, "gh", time.Duration(ttl), func() ([]model.AnalyticsGroupHealthItem, error) {
		return AnalyticsGroupHealthGet(ctx)
	})
}
