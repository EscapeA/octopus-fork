package balancer

import (
	"testing"
	"time"
)

func resetKeySpeed() {
	globalKeySpeed.Range(func(key, _ any) bool {
		globalKeySpeed.Delete(key)
		return true
	})
}

func TestKeySpeedInitialTPSIsZero(t *testing.T) {
	resetKeySpeed()
	tps := GetKeyTPS(1, 1, "gpt-4o")
	if tps != 0 {
		t.Fatalf("initial tps = %v, want 0", tps)
	}
}

func TestKeySpeedEmptyModelReturnsZero(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 1, "", 100, 5000)
	tps := GetKeyTPS(1, 1, "")
	if tps != 0 {
		t.Fatalf("empty model tps = %v, want 0", tps)
	}
}

func TestKeySpeedZeroKeyIDReturnsZero(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 0, "gpt-4o", 100, 5000)
	tps := GetKeyTPS(1, 0, "gpt-4o")
	if tps != 0 {
		t.Fatalf("zero keyID tps = %v, want 0", tps)
	}
}

func TestKeySpeedRecordSetsTPS(t *testing.T) {
	resetKeySpeed()
	// 100 tokens in 2000ms = 50 tokens/sec
	RecordKeySpeed(1, 1, "gpt-4o", 100, 2000)
	tps := GetKeyTPS(1, 1, "gpt-4o")
	if tps != 50.0 {
		t.Fatalf("tps = %v, want 50.0", tps)
	}
}

func TestKeySpeedEMASmoothing(t *testing.T) {
	resetKeySpeed()
	// First record: 100 tokens in 1000ms = 100 tps
	RecordKeySpeed(1, 1, "gpt-4o", 100, 1000)
	// Second record: 200 tokens in 1000ms = 200 tps
	// EMA = 0.3*200 + 0.7*100 = 60 + 70 = 130
	RecordKeySpeed(1, 1, "gpt-4o", 200, 1000)
	tps := GetKeyTPS(1, 1, "gpt-4o")
	want := 0.3*200 + 0.7*100
	if tps != want {
		t.Fatalf("ema tps = %v, want %v", tps, want)
	}
}

func TestKeySpeedZeroOutputSkips(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 1, "gpt-4o", 0, 1000)
	tps := GetKeyTPS(1, 1, "gpt-4o")
	if tps != 0 {
		t.Fatalf("zero output tps = %v, want 0 (skipped)", tps)
	}
}

func TestKeySpeedZeroDurationSkips(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 1, "gpt-4o", 100, 0)
	tps := GetKeyTPS(1, 1, "gpt-4o")
	if tps != 0 {
		t.Fatalf("zero duration tps = %v, want 0 (skipped)", tps)
	}
}

func TestKeySpeedIsPerModel(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 1, "gpt-4o", 100, 1000) // 100 tps
	// model B has no data
	tpsA := GetKeyTPS(1, 1, "gpt-4o")
	tpsB := GetKeyTPS(1, 1, "claude-3-5-sonnet")
	if tpsA != 100.0 {
		t.Fatalf("model A tps = %v, want 100.0", tpsA)
	}
	if tpsB != 0 {
		t.Fatalf("model B tps = %v, want 0 (no data)", tpsB)
	}
}

func TestRemoveChannelKeySpeed(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 1, "model-a", 100, 1000)
	RecordKeySpeed(1, 2, "model-b", 100, 1000)
	RecordKeySpeed(2, 3, "model-c", 100, 1000)

	RemoveChannelKeySpeed(1)
	if tps := GetKeyTPS(1, 1, "model-a"); tps != 0 {
		t.Fatalf("after remove channel 1, model-a tps = %v, want 0", tps)
	}
	if tps := GetKeyTPS(1, 2, "model-b"); tps != 0 {
		t.Fatalf("after remove channel 1, model-b tps = %v, want 0", tps)
	}
	// channel 2 should be untouched
	if tps := GetKeyTPS(2, 3, "model-c"); tps != 100.0 {
		t.Fatalf("channel 2 model-c tps = %v, want 100.0", tps)
	}
}

func TestRemoveKeySpeed(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 1, "model-a", 100, 1000)
	RecordKeySpeed(1, 2, "model-b", 100, 1000)

	RemoveKeySpeed(1)
	if tps := GetKeyTPS(1, 1, "model-a"); tps != 0 {
		t.Fatalf("after remove key 1, model-a tps = %v, want 0", tps)
	}
	// key 2 should be untouched
	if tps := GetKeyTPS(1, 2, "model-b"); tps != 100.0 {
		t.Fatalf("key 2 model-b tps = %v, want 100.0", tps)
	}
}

func TestPurgeStaleKeySpeed(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 1, "gpt-4o", 100, 1000)

	// Simulate entry going stale by backdating lastActivity
	key := speedKey(1, 1, "gpt-4o")
	v, _ := globalKeySpeed.Load(key)
	entry := v.(*keySpeedEntry)
	entry.mu.Lock()
	entry.lastActivity = time.Now().Add(-2 * time.Hour)
	entry.mu.Unlock()

	removed := PurgeStaleKeySpeed(time.Hour)
	if removed < 1 {
		t.Fatalf("expected purge, removed = %d", removed)
	}
	if tps := GetKeyTPS(1, 1, "gpt-4o"); tps != 0 {
		t.Fatalf("after purge, tps = %v, want 0", tps)
	}
}

func TestPurgeStaleKeySpeedReadPathDoesNotRefreshAnchor(t *testing.T) {
	resetKeySpeed()
	RecordKeySpeed(1, 1, "gpt-4o", 100, 1000)

	key := speedKey(1, 1, "gpt-4o")
	v, _ := globalKeySpeed.Load(key)
	entry := v.(*keySpeedEntry)
	entry.mu.Lock()
	entry.lastActivity = time.Now().Add(-2 * time.Hour)
	entry.mu.Unlock()

	// Repeated reads should not refresh lastActivity
	for i := 0; i < 3; i++ {
		_ = GetKeyTPS(1, 1, "gpt-4o")
	}

	removed := PurgeStaleKeySpeed(time.Hour)
	if removed < 1 {
		t.Fatalf("read path should not refresh purge anchor; expected purge, removed = %d", removed)
	}
}
