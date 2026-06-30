package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/redis/go-redis/v9"
)

// redisRuntimeState implements RuntimeStateStore backed by Redis.
//
// key scheme:
//
//	octopus:rt:circuit:{key} -> JSON(model.CircuitBreakerState)
//	octopus:rt:auto:{key}     -> JSON(model.AutoStrategyState)
//
// TTL guards against unbounded growth from high-cardinality model names (the
// same concern as the in-memory idle purges). Breaker TTL = max cooldown × 2
// (default 1200s); auto-strategy TTL = time window × 2 (default 600s). These
// are upper bounds — entries are continuously refreshed by SaveCircuit/SaveAuto
// during normal operation.
type redisRuntimeState struct {
	c          *redis.Client
	circuitTTL time.Duration
	autoTTL    time.Duration
}

// newRedisRuntimeState 构造 Redis 运行时状态存储。TTL 为各条目的过期上界
// （防 stale，防无界增长），取默认值：circuit = 20min（max cooldown 上界），
// auto = 10min（time window 上界）。条目在正常运行中由 SaveCircuit/SaveAuto 续期。
func newRedisRuntimeState(c *redis.Client) *redisRuntimeState {
	return &redisRuntimeState{
		c:          c,
		circuitTTL: 20 * time.Minute,
		autoTTL:    10 * time.Minute,
	}
}

const (
	rtKindCircuit = "circuit"
	rtKindAuto    = "auto"
)

func rtKey(kind, key string) string { return "rt:" + kind + ":" + key }

func (r *redisRuntimeState) SaveCircuit(ctx context.Context, key string, state model.CircuitBreakerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal circuit state: %w", err)
	}
	ttl := r.circuitTTL
	if ttl <= 0 {
		ttl = 20 * time.Minute
	}
	return r.c.Set(ctx, withPrefix(rtKey(rtKindCircuit, key)), data, ttl).Err()
}

func (r *redisRuntimeState) SaveAuto(ctx context.Context, key string, state model.AutoStrategyState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal auto state: %w", err)
	}
	ttl := r.autoTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return r.c.Set(ctx, withPrefix(rtKey(rtKindAuto, key)), data, ttl).Err()
}

func (r *redisRuntimeState) LoadCircuit(ctx context.Context) ([]model.CircuitBreakerState, error) {
	c := r.c
	if c == nil {
		return nil, ErrBackendDisabled
	}
	pattern := withPrefix(rtKey(rtKindCircuit, "*"))
	var result []model.CircuitBreakerState
	iter := c.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		data, err := c.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			continue
		}
		var s model.CircuitBreakerState
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		result = append(result, s)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("scan circuit states: %w", err)
	}
	return result, nil
}

func (r *redisRuntimeState) LoadAuto(ctx context.Context) ([]model.AutoStrategyState, error) {
	c := r.c
	if c == nil {
		return nil, ErrBackendDisabled
	}
	pattern := withPrefix(rtKey(rtKindAuto, "*"))
	var result []model.AutoStrategyState
	iter := c.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		data, err := c.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			continue
		}
		var s model.AutoStrategyState
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		result = append(result, s)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("scan auto states: %w", err)
	}
	return result, nil
}

// DeleteStale implements generation-replace semantics. With Redis TTL handling
// expiry, this is a no-op for normal operation; it exists for API compatibility
// with the memory implementation's generation-replace behavior used by
// balancer.SaveRuntimeState. Stale entries expire via TTL instead.
func (r *redisRuntimeState) DeleteStale(ctx context.Context, kind string, beforeGeneration int64) error {
	// Redis 条目自带 TTL，generation-replace 语义不再需要（TTL 自动清理 stale）。
	return nil
}
