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
//
// 失败时（Redis 不可达）不阻塞调用方降级决策：返回错误后调用方可选择降级到
// 内存后端并启动 StartReconnect 后台重连（issue #135）。
func Init(cfg conf.RedisConfig) error {
	if cfg.Addr == "" {
		return fmt.Errorf("redis addr is empty")
	}
	// 启动期先打印「正在连接」，避免 Ping 超时窗口内日志空白（issue #135）。
	log.Infof("redis backend connecting: %s db=%d (dial_timeout=%s, read_timeout=%s)",
		cfg.Addr, cfg.DB, durationStr(cfg.DialTimeout), durationStr(cfg.ReadTimeout))

	c := newClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		log.Warnf("redis init failed, falling back to memory backend: %v", err)
		return fmt.Errorf("redis ping %s: %w", cfg.Addr, err)
	}

	switchToRedis(c)
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
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping %s: %w", cfg.Addr, err)
	}
	return nil
}

// newClient 根据 RedisConfig 构造 go-redis client（Init / TestConnection /
// StartReconnect 共用）。DialTimeout/ReadTimeout 为 0 时沿用 go-redis 默认值
// （5s/3s）；显式配置时覆盖，用于缩短远程 Redis 重启期间的阻塞窗口（issue #135）。
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
	if cfg.DialTimeout > 0 {
		opts.DialTimeout = cfg.DialTimeout
	}
	if cfg.ReadTimeout > 0 {
		opts.ReadTimeout = cfg.ReadTimeout
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

// reconnectBackoff 计算第 attempt 次（从 1 开始）重试前的等待时长：初始 2s，
// 指数增长（×2），上限 30s。
func reconnectBackoff(attempt int) time.Duration {
	d := 2 * time.Second
	for i := 1; i < attempt && d < 30*time.Second; i++ {
		d *= 2
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// StartReconnect 启动后台 goroutine 退避重试连接 Redis。供 cmd/start.go 在
// Init 失败后调用：服务以内存后端继续启动并监听端口，后台重连成功后热切换到
// Redis 后端（issue #135）。
//
// 成功后自动调用 switchToRedis 切换后端，并执行 onConnect 回调（如注册
// shutdown.Close）。回调在重连 goroutine 中同步执行，可为 nil。
//
// 已启用 Redis（Enabled()==true）时返回 ErrAlreadyConnected，不重复启动。
func StartReconnect(cfg conf.RedisConfig, onConnect func()) error {
	if cfg.Addr == "" {
		return fmt.Errorf("redis addr is empty")
	}
	if Enabled() {
		return fmt.Errorf("redis backend already connected")
	}
	log.Warnf("redis will retry in background (initial backoff=2s, max=30s); service continues with memory backend")
	go reconnectLoop(cfg, onConnect)
	return nil
}

// reconnectLoop 是 StartReconnect 的后台循环。每次重试用独立 client + 3s ping
// 超时；失败则按指数退避等待后重试，成功则切换后端并退出循环。
func reconnectLoop(cfg conf.RedisConfig, onConnect func()) {
	const pingTimeout = 3 * time.Second
	for attempt := 1; ; attempt++ {
		// 先等待退避再重试（首次也等待，给 Redis 一点恢复时间）。
		backoff := reconnectBackoff(attempt)
		time.Sleep(backoff)

		c := newClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		err := c.Ping(ctx).Err()
		cancel()
		if err != nil {
			_ = c.Close()
			next := reconnectBackoff(attempt + 1)
			log.Warnf("redis reconnect attempt %d failed: %v (next in %s)", attempt, err, next)
			continue
		}

		switchToRedis(c)
		log.Infof("redis backend reconnected after %d attempts: %s db=%d", attempt, cfg.Addr, cfg.DB)
		log.Warnf("redis backend switched from memory fallback to redis (stats accumulated in memory during fallback will NOT be backfilled)")
		if onConnect != nil {
			onConnect()
		}
		return
	}
}

// durationStr 返回 Duration 的可读字符串，0 时返回 "default"。
func durationStr(d time.Duration) string {
	if d <= 0 {
		return "default"
	}
	return d.String()
}

// withPrefix 给 key 加上统一前缀。
func withPrefix(key string) string {
	return keyPrefix + key
}
