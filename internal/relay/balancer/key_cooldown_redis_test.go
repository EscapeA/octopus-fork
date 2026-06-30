package balancer

import (
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/lingyuins/octopus/internal/store"
	"github.com/redis/go-redis/v9"
)

// startMiniredisForCooldown 启动 miniredis 并注入到 store，返回 miniredis 实例
// （供 FastForward 推进 TTL）与清理。失败时 t.Fatal。
func startMiniredisForCooldown(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	restore := store.InjectForTest(c)
	t.Cleanup(func() {
		_ = c.Close()
		mr.Close()
		restore()
	})
	return mr, c
}

// TestKeyCooldownRedisRecordAndCheck 验证 Redis 后端下记录冷却后能被检查到。
func TestKeyCooldownRedisRecordAndCheck(t *testing.T) {
	_, _ = startMiniredisForCooldown(t)
	resetKeyCooldown()

	RecordKeyCooldown(1, 1, "gpt-4o", http.StatusTooManyRequests)
	if !IsKeyOnCooldown(1, 1, "gpt-4o") {
		t.Fatal("same (channel,key,model) should be on cooldown after recording (redis)")
	}
	// 其他模型不应被冷却（issue #94 核心诉求）
	if IsKeyOnCooldown(1, 1, "claude-3-5-sonnet") {
		t.Fatal("other model on same key should not be cooled down (redis)")
	}
}

// TestKeyCooldownRedisExpiry 验证 Redis TTL 过期后冷却失效。
func TestKeyCooldownRedisExpiry(t *testing.T) {
	mr, _ := startMiniredisForCooldown(t)
	resetKeyCooldown()

	RecordKeyCooldown(1, 1, "gpt-4o", http.StatusTooManyRequests)
	if !IsKeyOnCooldown(1, 1, "gpt-4o") {
		t.Fatal("should be on cooldown before TTL expiry (redis)")
	}
	// 推进 miniredis 时钟超过冷却时长（默认 300s）
	mr.FastForward(301 * time.Second)
	if IsKeyOnCooldown(1, 1, "gpt-4o") {
		t.Fatal("expired cooldown should report not on cooldown (redis)")
	}
}

// TestKeyCooldownRedisRemoveChannel 验证按渠道前缀删除冷却条目。
func TestKeyCooldownRedisRemoveChannel(t *testing.T) {
	_, _ = startMiniredisForCooldown(t)
	resetKeyCooldown()

	RecordKeyCooldown(1, 1, "model-a", http.StatusTooManyRequests)
	RecordKeyCooldown(1, 2, "model-b", http.StatusTooManyRequests)
	RecordKeyCooldown(2, 3, "model-c", http.StatusTooManyRequests)

	RemoveChannelKeyCooldowns(1)

	if IsKeyOnCooldown(1, 1, "model-a") {
		t.Fatal("channel 1 model-a should have been removed (redis)")
	}
	if IsKeyOnCooldown(1, 2, "model-b") {
		t.Fatal("channel 1 model-b should have been removed (redis)")
	}
	if !IsKeyOnCooldown(2, 3, "model-c") {
		t.Fatal("channel 2 model-c should remain (redis)")
	}
}

// TestKeyCooldownRedisRemoveByKeyID 验证按 keyID 子串删除冷却条目。
func TestKeyCooldownRedisRemoveByKeyID(t *testing.T) {
	_, _ = startMiniredisForCooldown(t)
	resetKeyCooldown()

	RecordKeyCooldown(1, 5, "model-a", http.StatusTooManyRequests)
	RecordKeyCooldown(2, 5, "model-b", http.StatusTooManyRequests)
	RecordKeyCooldown(3, 6, "model-c", http.StatusTooManyRequests)

	RemoveKeyCooldowns(5)

	if IsKeyOnCooldown(1, 5, "model-a") {
		t.Fatal("keyID 5 model-a should have been removed (redis)")
	}
	if IsKeyOnCooldown(2, 5, "model-b") {
		t.Fatal("keyID 5 model-b should have been removed (redis)")
	}
	if !IsKeyOnCooldown(3, 6, "model-c") {
		t.Fatal("keyID 6 model-c should remain (redis)")
	}
}
