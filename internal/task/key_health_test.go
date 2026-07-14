package task

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
)

func resetKeyHealthState() {
	globalKeyHealthState.Range(func(key, _ any) bool {
		globalKeyHealthState.Delete(key)
		return true
	})
}

func TestKeyHealthStateInitial(t *testing.T) {
	resetKeyHealthState()
	state := getKeyHealthState(1)
	if state.consecutiveFails != 0 {
		t.Fatalf("initial consecutiveFails = %d, want 0", state.consecutiveFails)
	}
	if state.notifiedFailed {
		t.Fatalf("initial notifiedFailed = true, want false")
	}
}

func TestKeyHealthStateIncrementFails(t *testing.T) {
	resetKeyHealthState()
	state := getKeyHealthState(1)
	state.mu.Lock()
	state.consecutiveFails++
	state.consecutiveFails++
	state.mu.Unlock()

	state = getKeyHealthState(1)
	if state.consecutiveFails != 2 {
		t.Fatalf("consecutiveFails = %d, want 2", state.consecutiveFails)
	}
}

func TestRemoveChannelKeyHealthState(t *testing.T) {
	resetKeyHealthState()
	_ = getKeyHealthState(1)
	_ = getKeyHealthState(2)

	RemoveChannelKeyHealthState(1)

	if _, ok := globalKeyHealthState.Load(1); ok {
		t.Fatal("channel 1 state should have been removed")
	}
	if _, ok := globalKeyHealthState.Load(2); !ok {
		t.Fatal("channel 2 state should still exist")
	}
}

func TestHandleKeyHealthFailureIncrementsAndNotifiesAtThreshold(t *testing.T) {
	resetKeyHealthState()
	ch := &model.Channel{ID: 1, Name: "test-ch"}
	threshold := 3
	cooldownSec := 300
	now := time.Now()

	// Fail 1: below threshold, no notification
	handleKeyHealthFailure(nil, ch, threshold, true, cooldownSec, now, "fail1")
	state := getKeyHealthState(1)
	if state.consecutiveFails != 1 {
		t.Fatalf("after 1st fail, consecutiveFails = %d, want 1", state.consecutiveFails)
	}
	if state.notifiedFailed {
		t.Fatal("after 1st fail, should not have notified")
	}

	// Fail 2: below threshold
	handleKeyHealthFailure(nil, ch, threshold, true, cooldownSec, now, "fail2")
	state = getKeyHealthState(1)
	if state.consecutiveFails != 2 {
		t.Fatalf("after 2nd fail, consecutiveFails = %d, want 2", state.consecutiveFails)
	}

	// Fail 3: at threshold, should notify (notification.Create will fail without DB, but state is set)
	handleKeyHealthFailure(nil, ch, threshold, true, cooldownSec, now, "fail3")
	state = getKeyHealthState(1)
	if state.consecutiveFails != 3 {
		t.Fatalf("after 3rd fail, consecutiveFails = %d, want 3", state.consecutiveFails)
	}
	if !state.notifiedFailed {
		t.Fatal("after 3rd fail (at threshold), should have notified")
	}
	if state.lastNotifyAt.IsZero() {
		t.Fatal("lastNotifyAt should be set after notification")
	}
}

func TestHandleKeyHealthFailureCooldownSuppressesNotification(t *testing.T) {
	resetKeyHealthState()
	ch := &model.Channel{ID: 1, Name: "test-ch"}
	threshold := 1
	cooldownSec := 300
	now := time.Now()

	// First fail: threshold=1, should notify
	handleKeyHealthFailure(nil, ch, threshold, true, cooldownSec, now, "fail1")
	state := getKeyHealthState(1)
	if !state.notifiedFailed {
		t.Fatal("first fail should notify")
	}
	firstNotify := state.lastNotifyAt

	// Second fail: within cooldown, should NOT notify again
	handleKeyHealthFailure(nil, ch, threshold, true, cooldownSec, now, "fail2")
	state = getKeyHealthState(1)
	if state.lastNotifyAt != firstNotify {
		t.Fatal("lastNotifyAt should not change within cooldown")
	}
	if state.consecutiveFails != 2 {
		t.Fatalf("consecutiveFails = %d, want 2", state.consecutiveFails)
	}
}

func TestHandleKeyHealthRecoveryResetsState(t *testing.T) {
	resetKeyHealthState()
	ch := &model.Channel{ID: 1, Name: "test-ch"}
	now := time.Now()

	// Simulate a previously failed state
	state := getKeyHealthState(1)
	state.mu.Lock()
	state.consecutiveFails = 3
	state.notifiedFailed = true
	state.mu.Unlock()

	// Recovery: should reset state
	handleKeyHealthRecovery(nil, ch, true, now)

	state = getKeyHealthState(1)
	if state.consecutiveFails != 0 {
		t.Fatalf("after recovery, consecutiveFails = %d, want 0", state.consecutiveFails)
	}
	if state.notifiedFailed {
		t.Fatal("after recovery, notifiedFailed should be false")
	}
}

func TestHandleKeyHealthRecoveryNoNotificationIfNeverFailed(t *testing.T) {
	resetKeyHealthState()
	ch := &model.Channel{ID: 1, Name: "test-ch"}
	now := time.Now()

	// No prior failure - recovery should not send notification (wasNotified=false)
	handleKeyHealthRecovery(nil, ch, true, now)

	state := getKeyHealthState(1)
	if state.consecutiveFails != 0 {
		t.Fatalf("consecutiveFails = %d, want 0", state.consecutiveFails)
	}
}

func TestBuildKeyHealthFailureDetail(t *testing.T) {
	tests := []struct {
		name    string
		summary *helper.ChannelTestSummary
		want    string
	}{
		{
			name:    "nil summary",
			summary: nil,
			want:    "all keys failed",
		},
		{
			name:    "empty results",
			summary: &helper.ChannelTestSummary{Results: nil},
			want:    "all keys failed",
		},
		{
			name: "all passed",
			summary: &helper.ChannelTestSummary{
				Results: []helper.ChannelTestResult{{Passed: true}},
			},
			want: "all keys failed",
		},
		{
			name: "one failed with remark",
			summary: &helper.ChannelTestSummary{
				Results: []helper.ChannelTestResult{
					{Passed: true},
					{Passed: false, KeyRemark: "prod-key", Message: "HTTP 401"},
				},
			},
			want: "prod-key: HTTP 401",
		},
		{
			name: "multiple failed",
			summary: &helper.ChannelTestSummary{
				Results: []helper.ChannelTestResult{
					{Passed: false, KeyMasked: "sk-x...123", StatusCode: 429},
					{Passed: false, KeyRemark: "backup", Message: "timeout"},
				},
			},
			want: "sk-x...123: HTTP 429; backup: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKeyHealthFailureDetail(tt.summary)
			if got != tt.want {
				t.Fatalf("buildKeyHealthFailureDetail() = %q, want %q", got, tt.want)
			}
		})
	}
}
