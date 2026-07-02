package balancer

import (
	"fmt"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

func clearAutoStatsForTest() {
	globalAutoStats.Range(func(key, _ any) bool {
		globalAutoStats.Delete(key)
		return true
	})
}

func recordOutcome(channelID int, modelName string, success bool, count int) {
	for i := 0; i < count; i++ {
		stats := getOrCreateStats(channelID, modelName)
		stats.Record(success)
	}
}

func TestAutoCandidatesPreferLowerSampleCountDuringExploration(t *testing.T) {
	clearAutoStatsForTest()

	modelName := fmt.Sprintf("auto-explore-%d", time.Now().UnixNano())
	items := []model.GroupItem{
		{ChannelID: 1, ModelName: modelName, Weight: 100, Priority: 1},
		{ChannelID: 2, ModelName: modelName, Weight: 1, Priority: 2},
	}

	recordOutcome(1, modelName, true, 1)

	got := (&Auto{}).Candidates(items)
	if len(got) != 2 {
		t.Fatalf("Candidates() len = %d, want 2", len(got))
	}
	if got[0].ChannelID != 2 {
		t.Fatalf("Candidates()[0].ChannelID = %d, want 2", got[0].ChannelID)
	}
}

func TestAutoCandidatesUseWeightPriorityWhenAllSamplesAreZero(t *testing.T) {
	clearAutoStatsForTest()

	modelName := fmt.Sprintf("auto-zero-%d", time.Now().UnixNano())
	items := []model.GroupItem{
		{ChannelID: 1, ModelName: modelName, Weight: 1, Priority: 2},
		{ChannelID: 2, ModelName: modelName, Weight: 10, Priority: 1},
	}

	got := (&Auto{}).Candidates(items)
	if len(got) != 2 {
		t.Fatalf("Candidates() len = %d, want 2", len(got))
	}
	if got[0].ChannelID != 2 {
		t.Fatalf("Candidates()[0].ChannelID = %d, want 2", got[0].ChannelID)
	}
}

func TestAutoCandidatesPreferHigherSuccessRateAfterMinSamples(t *testing.T) {
	clearAutoStatsForTest()

	modelName := fmt.Sprintf("auto-exploit-%d", time.Now().UnixNano())
	items := []model.GroupItem{
		{ChannelID: 1, ModelName: modelName, Weight: 1, Priority: 1},
		{ChannelID: 2, ModelName: modelName, Weight: 100, Priority: 2},
	}

	recordOutcome(1, modelName, true, 10)
	recordOutcome(2, modelName, true, 6)
	recordOutcome(2, modelName, false, 4)

	got := (&Auto{}).Candidates(items)
	if len(got) != 2 {
		t.Fatalf("Candidates() len = %d, want 2", len(got))
	}
	if got[0].ChannelID != 1 {
		t.Fatalf("Candidates()[0].ChannelID = %d, want 1", got[0].ChannelID)
	}
}

func TestAutoCandidatesUseWeightPriorityAsTieBreaker(t *testing.T) {
	clearAutoStatsForTest()

	modelName := fmt.Sprintf("auto-tie-%d", time.Now().UnixNano())
	items := []model.GroupItem{
		{ChannelID: 1, ModelName: modelName, Weight: 1, Priority: 2},
		{ChannelID: 2, ModelName: modelName, Weight: 10, Priority: 1},
	}

	recordOutcome(1, modelName, true, 10)
	recordOutcome(2, modelName, true, 10)

	got := (&Auto{}).Candidates(items)
	if len(got) != 2 {
		t.Fatalf("Candidates() len = %d, want 2", len(got))
	}
	if got[0].ChannelID != 2 {
		t.Fatalf("Candidates()[0].ChannelID = %d, want 2", got[0].ChannelID)
	}
}

func TestIteratorForwardedAttemptsExcludesSkippedAndCircuitBreak(t *testing.T) {
	it := &Iterator{
		attempts: []model.ChannelAttempt{
			{Status: model.AttemptSkipped},
			{Status: model.AttemptCircuitBreak},
			{Status: model.AttemptFailed},
			{Status: model.AttemptSuccess},
		},
	}

	if got := it.ForwardedAttempts(); got != 2 {
		t.Fatalf("ForwardedAttempts() = %d, want 2", got)
	}
}

func TestDisposableChannelsSortedFirst(t *testing.T) {
	// Disposable channels should be sorted before non-disposable ones,
	// regardless of the underlying balancer strategy.
	disposableSet := map[int]bool{3: true, 5: true}
	orig := DisposableChannelFunc
	DisposableChannelFunc = func(channelID int) bool {
		return disposableSet[channelID]
	}
	defer func() { DisposableChannelFunc = orig }()

	items := []model.GroupItem{
		{ChannelID: 1, ModelName: "gpt-4", Priority: 1, Weight: 10},
		{ChannelID: 3, ModelName: "gpt-4", Priority: 5, Weight: 1},
		{ChannelID: 2, ModelName: "gpt-4", Priority: 2, Weight: 5},
		{ChannelID: 5, ModelName: "gpt-4", Priority: 3, Weight: 3},
	}

	group := model.Group{Mode: model.GroupModeRoundRobin, Items: items}
	it := NewIterator(group, 0, "gpt-4", nil)

	// First two candidates should be the disposable channels (3 and 5)
	seen := make(map[int]bool)
	for i := 0; i < 2; i++ {
		if !it.Next() {
			t.Fatalf("Next() returned false at index %d", i)
		}
		chID := it.Item().ChannelID
		if !disposableSet[chID] {
			t.Fatalf("candidate[%d].ChannelID = %d, expected a disposable channel", i, chID)
		}
		seen[chID] = true
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2 distinct disposable channels in first 2 positions, got %d", len(seen))
	}
}

func TestDisposableChannelsRespectsSticky(t *testing.T) {
	// Sticky channel should take precedence over disposable priority.
	disposableSet := map[int]bool{3: true}
	orig := DisposableChannelFunc
	DisposableChannelFunc = func(channelID int) bool {
		return disposableSet[channelID]
	}
	defer func() { DisposableChannelFunc = orig }()

	items := []model.GroupItem{
		{ChannelID: 1, ModelName: "gpt-4", Priority: 1, Weight: 10},
		{ChannelID: 3, ModelName: "gpt-4", Priority: 5, Weight: 1},
	}

	// Set up sticky: apiKeyID=42, model=gpt-4 -> channel 1 (non-disposable)
	SetSticky(42, "gpt-4", 1, 0)
	defer globalSession.Delete(sessionKey(42, "gpt-4"))

	group := model.Group{Mode: model.GroupModeRoundRobin, SessionKeepTime: 300, Items: items}
	it := NewIterator(group, 42, "gpt-4", nil)

	if !it.Next() {
		t.Fatal("Next() returned false")
	}
	if it.Item().ChannelID != 1 {
		t.Fatalf("first candidate ChannelID = %d, want 1 (sticky, non-disposable)", it.Item().ChannelID)
	}
}
