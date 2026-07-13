package group

import (
	"context"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

func TestGroupDelAllResetsAIRouteSettingEvenWhenMissing(t *testing.T) {
	ctx := initGroupDelTestDB(t)

	// Create a group with items to exercise the full delete path.
	created := &model.Group{
		Name:         "delall-target",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeRandom,
		Items: []model.GroupItem{
			{ChannelID: 1, ModelName: "delall-model", Priority: 1, Weight: 1},
		},
	}
	if err := GroupCreate(created, ctx); err != nil {
		t.Fatalf("GroupCreate: %v", err)
	}

	// Simulate the ai_route_group_id setting row missing (corrupted/never seeded).
	if err := db.GetDB().Where("key = ?", model.SettingKeyAIRouteGroupID).Delete(&model.Setting{}).Error; err != nil {
		t.Fatalf("delete setting: %v", err)
	}
	setting.RefreshCache(ctx)

	count, err := GroupDelAll(ctx)
	if err != nil {
		t.Fatalf("GroupDelAll should succeed even when setting missing, got: %v", err)
	}
	if count != 1 {
		t.Fatalf("deleted count = %d, want 1", count)
	}

	// ai_route_group_id should exist and be 0 after reset.
	val, err := setting.GetString(model.SettingKeyAIRouteGroupID)
	if err != nil || val != "0" {
		t.Fatalf("ai_route_group_id after reset = %q err=%v, want \"0\"", val, err)
	}
}

func TestGroupDelRemovesGroupAndItems(t *testing.T) {
	ctx := initGroupDelTestDB(t)
	g := &model.Group{
		Name:         "single-del",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeFailover,
		Items: []model.GroupItem{
			{ChannelID: 2, ModelName: "single-model", Priority: 1, Weight: 1},
		},
	}
	if err := GroupCreate(g, ctx); err != nil {
		t.Fatalf("GroupCreate: %v", err)
	}
	if err := GroupDel(g.ID, ctx); err != nil {
		t.Fatalf("GroupDel: %v", err)
	}
	if _, err := GroupGet(g.ID, ctx); err == nil || !strings.Contains(err.Error(), "group not found") {
		t.Fatalf("expected group not found after delete, got %v", err)
	}
	var items []model.GroupItem
	if err := db.GetDB().Where("group_id = ?", g.ID).Find(&items).Error; err != nil {
		t.Fatalf("query items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items, got %d", len(items))
	}
}

func initGroupDelTestDB(t *testing.T) context.Context {
	t.Helper()
	ctx := context.Background()
	dsn := "file:" + strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()) + "?mode=memory&cache=shared"
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := RefreshAllCache(ctx); err != nil {
		t.Fatalf("refresh cache: %v", err)
	}
	t.Cleanup(func() {
		groupCache.Clear()
		groupMap.Clear()
		_ = db.Close()
	})
	return ctx
}
