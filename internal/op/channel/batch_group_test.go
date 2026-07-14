package channel

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

// setupBatchGroupTest initializes an isolated SQLite DB and registers group
// callbacks, restoring the originals on cleanup.
func setupBatchGroupTest(t *testing.T) {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db")
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := db.GetDB().AutoMigrate(&model.Channel{}, &model.ChannelKey{}, &model.ChannelGroup{}, &model.StatsChannel{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	origDefault := GroupDefaultID
	origGet := GroupGet
	t.Cleanup(func() {
		GroupDefaultID = origDefault
		GroupGet = origGet
		chCache.Clear()
		keyCache.Clear()
		if conn := db.GetDB(); conn != nil {
			if sqlDB, err := conn.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
	})

	// group 1 = default, groups 1..3 exist.
	GroupDefaultID = func(ctx context.Context) (int, error) { return 1, nil }
	GroupGet = func(id int, ctx context.Context) (*model.ChannelGroup, error) {
		if id >= 1 && id <= 3 {
			return &model.ChannelGroup{ID: id}, nil
		}
		return nil, errGroupNotFound
	}
}

var errGroupNotFound = &groupNotFoundError{}

type groupNotFoundError struct{}

func (*groupNotFoundError) Error() string { return "group not found" }

func seedChannel(t *testing.T, id, groupID int) {
	t.Helper()
	ch := &model.Channel{ID: id, Name: fmt.Sprintf("ch-%d", id), GroupID: groupID, Type: outbound.OutboundTypeOpenAIChat}
	if err := db.GetDB().Create(ch).Error; err != nil {
		t.Fatalf("seed channel %d failed: %v", id, err)
	}
	chCache.Set(id, *ch)
}

func TestBatchUpdateGroup_MovesAllToTarget(t *testing.T) {
	setupBatchGroupTest(t)
	seedChannel(t, 1, 1)
	seedChannel(t, 2, 2)
	seedChannel(t, 3, 1)

	res, err := BatchUpdateGroup([]int{1, 2, 3}, 3, context.Background())
	if err != nil {
		t.Fatalf("BatchUpdateGroup returned error: %v", err)
	}
	if len(res.SuccessIDs) != 3 {
		t.Fatalf("expected 3 successes, got %d (failed=%v)", len(res.SuccessIDs), res.FailedItems)
	}
	if len(res.FailedItems) != 0 {
		t.Fatalf("expected 0 failures, got %v", res.FailedItems)
	}
	for _, id := range []int{1, 2, 3} {
		ch, ok := chCache.Get(id)
		if !ok {
			t.Fatalf("channel %d missing from cache", id)
		}
		if ch.GroupID != 3 {
			t.Errorf("channel %d group = %d, want 3", id, ch.GroupID)
		}
	}
}

func TestBatchUpdateGroup_PartialFailure(t *testing.T) {
	setupBatchGroupTest(t)
	seedChannel(t, 1, 1)
	// channel 99 does not exist -> Update returns "channel not found".

	res, err := BatchUpdateGroup([]int{1, 99}, 2, context.Background())
	if err != nil {
		t.Fatalf("BatchUpdateGroup returned error: %v", err)
	}
	if len(res.SuccessIDs) != 1 || res.SuccessIDs[0] != 1 {
		t.Fatalf("expected success [1], got %v", res.SuccessIDs)
	}
	if len(res.FailedItems) != 1 || res.FailedItems[0].ID != 99 {
		t.Fatalf("expected failure for 99, got %v", res.FailedItems)
	}
	ch, _ := chCache.Get(1)
	if ch.GroupID != 2 {
		t.Errorf("channel 1 group = %d, want 2", ch.GroupID)
	}
}

func TestBatchUpdateGroup_ZeroResolvesDefault(t *testing.T) {
	setupBatchGroupTest(t)
	seedChannel(t, 1, 2)

	res, err := BatchUpdateGroup([]int{1}, 0, context.Background())
	if err != nil {
		t.Fatalf("BatchUpdateGroup returned error: %v", err)
	}
	if len(res.SuccessIDs) != 1 {
		t.Fatalf("expected 1 success, got %v", res.FailedItems)
	}
	ch, _ := chCache.Get(1)
	if ch.GroupID != 1 {
		t.Errorf("channel 1 group = %d, want default 1", ch.GroupID)
	}
}
