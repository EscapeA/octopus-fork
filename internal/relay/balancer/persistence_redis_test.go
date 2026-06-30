package balancer

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/store"
	"github.com/redis/go-redis/v9"
)

// newRTTestRedis 启动 miniredis 并注入到 store。
func newRTTestRedis(t *testing.T) *redis.Client {
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
	return c
}

func TestRuntimeStateRedisSaveAndLoadCircuit(t *testing.T) {
	newRTTestRedis(t)
	ctx := context.Background()
	rs := store.GetRuntimeState()

	// 写入一条 circuit breaker 状态。
	key := circuitKey(1, 2, "gpt-4o")
	state := model.CircuitBreakerState{
		Key:                 key,
		ChannelID:           1,
		ChannelKeyID:        2,
		ModelName:           "gpt-4o",
		State:               int(StateOpen),
		ConsecutiveFailures: 5,
		TripCount:           3,
	}
	if err := rs.SaveCircuit(ctx, key, state); err != nil {
		t.Fatalf("SaveCircuit: %v", err)
	}

	// 读回应一致。
	loaded, err := rs.LoadCircuit(ctx)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	var found bool
	for _, s := range loaded {
		if s.Key == key {
			found = true
			if s.State != int(StateOpen) || s.ConsecutiveFailures != 5 {
				t.Fatalf("loaded circuit state mismatch: %#v", s)
			}
		}
	}
	if !found {
		t.Fatal("saved circuit state not found in LoadCircuit")
	}
}

func TestRuntimeStateRedisSaveAndLoadAuto(t *testing.T) {
	newRTTestRedis(t)
	ctx := context.Background()
	rs := store.GetRuntimeState()

	key := statsKey(1, "gpt-4o")
	state := model.AutoStrategyState{
		Key:       key,
		ChannelID: 1,
		ModelName: "gpt-4o",
		Records: []model.AutoStrategyRecord{
			{Timestamp: 1000, Success: true},
			{Timestamp: 2000, Success: false},
		},
	}
	if err := rs.SaveAuto(ctx, key, state); err != nil {
		t.Fatalf("SaveAuto: %v", err)
	}

	loaded, err := rs.LoadAuto(ctx)
	if err != nil {
		t.Fatalf("LoadAuto: %v", err)
	}
	var found bool
	for _, s := range loaded {
		if s.Key == key {
			found = true
			if len(s.Records) != 2 {
				t.Fatalf("loaded auto records len = %d, want 2", len(s.Records))
			}
		}
	}
	if !found {
		t.Fatal("saved auto state not found in LoadAuto")
	}
}
