package group

import (
	"context"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	channelop "github.com/lingyuins/octopus/internal/op/channel"
)

func TestAutoGroupCreatesSameNameForDifferentEndpointType(t *testing.T) {
	ctx := initAutoGroupEndpointNameTestDB(t)

	existing := &model.Group{
		Name:         "text-embedding-3-small",
		EndpointType: model.EndpointTypeChat,
		Mode:         model.GroupModeRoundRobin,
	}
	if err := GroupCreate(existing, ctx); err != nil {
		t.Fatalf("create existing chat group: %v", err)
	}

	channelop.GetCache().Set(10, model.Channel{
		ID:          10,
		Name:        "embedding-channel",
		Enabled:     true,
		CustomModel: "text-embedding-3-small",
	})

	result, err := AutoGroupModelsWithCategory(ctx, "")
	if err != nil {
		t.Fatalf("AutoGroupModelsWithCategory() error = %v", err)
	}
	if result.CreatedGroups != 1 {
		t.Fatalf("CreatedGroups = %d, want 1; result=%+v", result.CreatedGroups, result)
	}

	got, err := GroupGetEnabledMapByEndpoint(model.EndpointTypeEmbeddings, "text-embedding-3-small", ctx)
	if err != nil {
		t.Fatalf("lookup created embeddings group: %v", err)
	}
	if got.EndpointType != model.EndpointTypeEmbeddings || got.Name != "text-embedding-3-small" {
		t.Fatalf("created group = %+v, want embeddings text-embedding-3-small", got)
	}
}

func initAutoGroupEndpointNameTestDB(t *testing.T) context.Context {
	t.Helper()
	ctx := context.Background()
	dsn := "file:" + strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()) + "?mode=memory&cache=shared"
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := RefreshAllCache(ctx); err != nil {
		t.Fatalf("refresh group cache: %v", err)
	}
	t.Cleanup(func() {
		groupCache.Clear()
		groupMap.Clear()
		channelop.GetCache().Clear()
		_ = db.Close()
	})
	return ctx
}
