package op

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

func TestGroupUpdateNormalizesItemsToAdd(t *testing.T) {
	ctx := context.Background()
	if err := db.InitDB("sqlite", "file:group-update-normalizes-items-to-add?mode=memory&cache=shared", false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		groupCache.Clear()
		groupMap.Clear()
		rebuildGroupIndexesFromCache()
	})

	groupCache.Clear()
	groupMap.Clear()
	rebuildGroupIndexesFromCache()

	group := &model.Group{
		Name:         "test-group-update-normalize",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeRoundRobin,
		Items: []model.GroupItem{
			{ChannelID: 1, ModelName: "gpt-4o-mini", Priority: 1, Weight: 2},
			{ChannelID: 2, ModelName: "claude-3-5-haiku", Priority: 2, Weight: 3},
		},
	}
	if err := GroupCreate(group, ctx); err != nil {
		t.Fatalf("create group: %v", err)
	}

	itemsBeforeUpdate, err := GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("list group items before update: %v", err)
	}
	if len(itemsBeforeUpdate) != 2 {
		t.Fatalf("expected 2 initial items, got %d", len(itemsBeforeUpdate))
	}

	req := &model.GroupUpdateRequest{
		ID: group.ID,
		ItemsToUpdate: []model.GroupItemUpdateRequest{
			{ID: itemsBeforeUpdate[0].ID, Priority: 1, Weight: 0},
			{ID: itemsBeforeUpdate[1].ID, Priority: 4, Weight: 6},
		},
		ItemsToAdd: []model.GroupItemAddRequest{
			{ChannelID: 1, ModelName: "gpt-4o-mini", Priority: 9, Weight: 9},
			{ChannelID: 3, ModelName: " claude-3-5-sonnet ", Priority: 3, Weight: 0},
			{ChannelID: 4, ModelName: "  ", Priority: 5, Weight: 2},
			{ChannelID: 5, ModelName: "gpt-4.1-mini", Priority: 0, Weight: 0},
			{ChannelID: 3, ModelName: "claude-3-5-sonnet", Priority: 7, Weight: 3},
		},
	}

	updated, err := GroupUpdate(req, ctx)
	if err != nil {
		t.Fatalf("update group: %v", err)
	}

	if len(updated.Items) != 4 {
		t.Fatalf("expected 4 items after update, got %d", len(updated.Items))
	}

	itemsAfterUpdate, err := GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("list group items after update: %v", err)
	}
	if len(itemsAfterUpdate) != 4 {
		t.Fatalf("expected 4 persisted items after update, got %d", len(itemsAfterUpdate))
	}

	if itemsAfterUpdate[0].ChannelID != 1 || itemsAfterUpdate[0].ModelName != "gpt-4o-mini" || itemsAfterUpdate[0].Priority != 1 || itemsAfterUpdate[0].Weight != 2 {
		t.Fatalf("unexpected first persisted item: %+v", itemsAfterUpdate[0])
	}
	if itemsAfterUpdate[1].ChannelID != 3 || itemsAfterUpdate[1].ModelName != "claude-3-5-sonnet" || itemsAfterUpdate[1].Priority != 3 || itemsAfterUpdate[1].Weight != 1 {
		t.Fatalf("unexpected second persisted item: %+v", itemsAfterUpdate[1])
	}
	if itemsAfterUpdate[2].ChannelID != 2 || itemsAfterUpdate[2].ModelName != "claude-3-5-haiku" || itemsAfterUpdate[2].Priority != 4 || itemsAfterUpdate[2].Weight != 6 {
		t.Fatalf("unexpected third persisted item: %+v", itemsAfterUpdate[2])
	}
	if itemsAfterUpdate[3].ChannelID != 5 || itemsAfterUpdate[3].ModelName != "gpt-4.1-mini" || itemsAfterUpdate[3].Priority != 5 || itemsAfterUpdate[3].Weight != 1 {
		t.Fatalf("unexpected fourth persisted item: %+v", itemsAfterUpdate[3])
	}
}

// 「模型 + 渠道」行启用开关：只发 {id, enabled} 的更新必须
//   1) 改到该行的启用状态；
//   2) 不改动该行的 priority / weight（零值 = 保持原值）；
//   3) 不影响同批次里未携带 enabled 的其它行。
func TestGroupUpdateItemEnabledToggle(t *testing.T) {
	ctx := context.Background()
	if err := db.InitDB("sqlite", "file:group-update-item-enabled?mode=memory&cache=shared", false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		groupCache.Clear()
		groupMap.Clear()
		rebuildGroupIndexesFromCache()
	})

	groupCache.Clear()
	groupMap.Clear()
	rebuildGroupIndexesFromCache()

	group := &model.Group{
		Name:         "test-group-item-enabled",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeFailover,
		Items: []model.GroupItem{
			{ChannelID: 1, ModelName: "deepseek-flash", Priority: 1, Weight: 5},
			{ChannelID: 2, ModelName: "deepseek-flash", Priority: 2, Weight: 3},
		},
	}
	if err := GroupCreate(group, ctx); err != nil {
		t.Fatalf("create group: %v", err)
	}

	items, err := GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if !items[0].IsEnabled() || !items[1].IsEnabled() {
		t.Fatalf("items must default to enabled, got %+v / %+v", items[0], items[1])
	}

	// 只切换第一行为禁用：priority / weight 一律不携带。
	disabled := false
	req := &model.GroupUpdateRequest{
		ID: group.ID,
		ItemsToUpdate: []model.GroupItemUpdateRequest{
			{ID: items[0].ID, Enabled: &disabled},
		},
	}
	if _, err := GroupUpdate(req, ctx); err != nil {
		t.Fatalf("update group: %v", err)
	}

	after, err := GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("list items after update: %v", err)
	}
	var first, second model.GroupItem
	for _, it := range after {
		switch it.ID {
		case items[0].ID:
			first = it
		case items[1].ID:
			second = it
		}
	}

	if first.IsEnabled() {
		t.Fatalf("item %d must be disabled", first.ID)
	}
	if first.Priority != 1 || first.Weight != 5 {
		t.Fatalf("priority/weight must be preserved on toggle-only update, got priority=%d weight=%d", first.Priority, first.Weight)
	}
	if !second.IsEnabled() {
		t.Fatalf("item %d must stay enabled (not carried in this batch)", second.ID)
	}
	if second.Priority != 2 || second.Weight != 3 {
		t.Fatalf("untouched item changed: priority=%d weight=%d", second.Priority, second.Weight)
	}

	// 重新启用同样只发 enabled。
	enabled := true
	req2 := &model.GroupUpdateRequest{
		ID: group.ID,
		ItemsToUpdate: []model.GroupItemUpdateRequest{
			{ID: items[0].ID, Enabled: &enabled},
		},
	}
	if _, err := GroupUpdate(req2, ctx); err != nil {
		t.Fatalf("re-enable group item: %v", err)
	}
	afterReenable, err := GroupItemList(group.ID, ctx)
	if err != nil {
		t.Fatalf("list items after re-enable: %v", err)
	}
	for _, it := range afterReenable {
		if it.ID == items[0].ID && !it.IsEnabled() {
			t.Fatalf("item %d must be enabled again", it.ID)
		}
	}
}
