package store

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisChannelDelayRoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer c.Close()

	ds := newRedisChannelDelay(c)
	ctx := context.Background()

	if err := ds.SetDelay(ctx, 1, "https://a.com", 50); err != nil {
		t.Fatalf("SetDelay: %v", err)
	}
	if err := ds.SetDelay(ctx, 1, "https://b.com", 120); err != nil {
		t.Fatalf("SetDelay b: %v", err)
	}

	delays, err := ds.GetDelays(ctx, 1)
	if err != nil {
		t.Fatalf("GetDelays: %v", err)
	}
	if delays["https://a.com"] != 50 || delays["https://b.com"] != 120 {
		t.Fatalf("unexpected delays: %#v", delays)
	}

	if err := ds.DeleteChannel(ctx, 1); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
	delays, _ = ds.GetDelays(ctx, 1)
	if len(delays) != 0 {
		t.Fatalf("delays should be empty after delete, got %#v", delays)
	}
}
