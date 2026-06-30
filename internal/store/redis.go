// Package store 提供可选的缓存与状态存储后端抽象。
//
// 设计目标（issue #123）：在不破坏现有内存 + 数据库策略的前提下，引入 Redis
// 作为可选后端，将统计/运行时状态/限流冷却/失败提示/频道延迟等运行时数据
// 卸载到 Redis，以支持低内存主机与多实例水平扩展。
//
// 接入方式：各子系统定义窄接口（KVStore / RateLimitStore / StatsStore /
// RuntimeStateStore / ChannelDelayStore），内存实现保持现有行为，Redis 实现
// 新增。启动时根据 conf.AppConfig.Cache.Type 注入后端（见 cmd/start.go）。
//
// 未配置 Redis 时（Type 为空），所有 Get* 返回内存实现，行为与旧版完全一致。
package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/redis/go-redis/v9"
)

// keyPrefix 统一所有 Redis key 前缀，避免与其他服务冲突。
const keyPrefix = "octopus:"

var (
	client  *redis.Client
	enabled bool
	mu      sync.RWMutex
)

// Init 初始化 Redis 连接并 ping 验证。cfg.Addr 为空或连接失败时返回错误。
// 成功后 Enabled() 返回 true，Get() 返回可用 client。
func Init(cfg conf.RedisConfig) error {
	if cfg.Addr == "" {
		return fmt.Errorf("redis addr is empty")
	}
	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.Username != "" {
		opts.Username = cfg.Username
	}
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}

	c := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return fmt.Errorf("redis ping %s: %w", cfg.Addr, err)
	}

	mu.Lock()
	client = c
	enabled = true
	// 注入 Redis 实现：后续 store.Get*() 返回这些实例。
	defaultKV = newRedisKV(c)
	defaultRateLimit = newRedisRateLimit(c)
	defaultStats = newRedisStats(c)
	defaultRuntimeState = newRedisRuntimeState(c)
	defaultChannelDelay = newRedisChannelDelay(c)
	mu.Unlock()
	log.Infof("redis backend connected: %s db=%d", cfg.Addr, cfg.DB)
	return nil
}

// TestConnection 验证给定 Redis 配置的连通性，不改变全局状态（不调用 Init、
// 不设置 client）。供设置页「测试连接」按钮调用——用户可在保存前先验证地址/
// 密码是否正确。成功后立即关闭临时连接。
func TestConnection(cfg conf.RedisConfig) error {
	if cfg.Addr == "" {
		return fmt.Errorf("redis addr is empty")
	}
	c := newClient(cfg)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping %s: %w", cfg.Addr, err)
	}
	return nil
}

// newClient 根据 RedisConfig 构造 go-redis client（Init 与 TestConnection 共用）。
func newClient(cfg conf.RedisConfig) *redis.Client {
	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.Username != "" {
		opts.Username = cfg.Username
	}
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	return redis.NewClient(opts)
}

// Get 返回 Redis client；未启用时返回 nil。调用方需自行判空。
func Get() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()
	return client
}

// Enabled 报告 Redis 后端是否已启用。
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled
}

// Close 关闭 Redis 连接。
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if client == nil {
		return nil
	}
	err := client.Close()
	client = nil
	enabled = false
	return err
}

// withPrefix 给 key 加上统一前缀。
func withPrefix(key string) string {
	return keyPrefix + key
}
