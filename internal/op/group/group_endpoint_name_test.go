package group

import (
	"context"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

func TestGroupCreateAllowsSameNameAcrossEndpointTypes(t *testing.T) {
	ctx := initGroupEndpointNameTestDB(t)

	chatGroup := &model.Group{
		Name:         "shared-model",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeRoundRobin,
	}
	if err := GroupCreate(chatGroup, ctx); err != nil {
		t.Fatalf("create chat group: %v", err)
	}

	embeddingGroup := &model.Group{
		Name:         "shared-model",
		EndpointType: model.EndpointTypeEmbeddings,
		Mode:         model.GroupModeRoundRobin,
	}
	if err := GroupCreate(embeddingGroup, ctx); err != nil {
		t.Fatalf("create embeddings group with same name: %v", err)
	}

	gotChat, err := GroupGetEnabledMapByEndpoint(model.EndpointTypeChat, "shared-model", ctx)
	if err != nil {
		t.Fatalf("lookup chat group: %v", err)
	}
	if gotChat.ID != chatGroup.ID || gotChat.EndpointType != model.EndpointTypeChat {
		t.Fatalf("chat lookup = %+v, want id %d endpoint %s", gotChat, chatGroup.ID, model.EndpointTypeChat)
	}

	gotEmbedding, err := GroupGetEnabledMapByEndpoint(model.EndpointTypeEmbeddings, "shared-model", ctx)
	if err != nil {
		t.Fatalf("lookup embeddings group: %v", err)
	}
	if gotEmbedding.ID != embeddingGroup.ID || gotEmbedding.EndpointType != model.EndpointTypeEmbeddings {
		t.Fatalf("embedding lookup = %+v, want id %d endpoint %s", gotEmbedding, embeddingGroup.ID, model.EndpointTypeEmbeddings)
	}
}

func TestGroupCreateRejectsSameNameWithinEndpointType(t *testing.T) {
	ctx := initGroupEndpointNameTestDB(t)

	first := &model.Group{
		Name:         "duplicate-model",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeRoundRobin,
	}
	if err := GroupCreate(first, ctx); err != nil {
		t.Fatalf("create first group: %v", err)
	}

	second := &model.Group{
		Name:         "duplicate-model",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeRoundRobin,
	}
	if err := GroupCreate(second, ctx); err == nil {
		t.Fatal("create duplicate chat group error = nil, want unique constraint")
	}
}

func initGroupEndpointNameTestDB(t *testing.T) context.Context {
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
