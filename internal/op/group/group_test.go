package group

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/channel"
)

func TestGroupListModel_EmptyGroupNotListed(t *testing.T) {
	groupCache.Clear()
	groupCache.Set(1, model.Group{
		ID:           1,
		Name:         "music-2.6",
		EndpointType: model.EndpointTypeMusicGeneration,
		Items:        nil,
	})

	models, err := GroupListModel(context.Background())
	if err != nil {
		t.Fatalf("GroupListModel returned error: %v", err)
	}

	for _, m := range models {
		if m == "music-2.6" {
			t.Errorf("music-2.6 should NOT appear in /v1/models when group has no items")
		}
	}
}

func TestGroupListModel_DisabledChannelNotListed(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Set(100, model.Channel{
		ID:      100,
		Name:    "test-channel",
		Enabled: false,
	})

	groupCache.Set(2, model.Group{
		ID:           2,
		Name:         "music-2.6",
		EndpointType: model.EndpointTypeMusicGeneration,
		Items: []model.GroupItem{
			{ID: 1, GroupID: 2, ChannelID: 100, ModelName: "music-2.6", Priority: 1, Weight: 1},
		},
	})

	models, err := GroupListModel(context.Background())
	if err != nil {
		t.Fatalf("GroupListModel returned error: %v", err)
	}

	for _, m := range models {
		if m == "music-2.6" {
			t.Errorf("music-2.6 should NOT appear when channel is disabled")
		}
	}

	// Cleanup
	chCache.Del(100)
}

func TestGroupListModel_ValidGroupListed(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Set(200, model.Channel{
		ID:      200,
		Name:    "enabled-channel",
		Enabled: true,
	})

	groupCache.Set(3, model.Group{
		ID:           3,
		Name:         "music-2.6",
		EndpointType: model.EndpointTypeMusicGeneration,
		Items: []model.GroupItem{
			{ID: 2, GroupID: 3, ChannelID: 200, ModelName: "music-2.6", Priority: 1, Weight: 1},
		},
	})

	models, err := GroupListModel(context.Background())
	if err != nil {
		t.Fatalf("GroupListModel returned error: %v", err)
	}

	found := false
	for _, m := range models {
		if m == "music-2.6" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("music-2.6 SHOULD appear when group has valid item and enabled channel")
	}

	// Cleanup
	chCache.Del(200)
}

// --- GroupListModelByEndpoint tests ---

func setupValidGroup(id int, name string, endpointType string) {
	chCache := channel.GetCache()
	channelID := id * 100
	chCache.Set(channelID, model.Channel{
		ID:      channelID,
		Name:    "ch-" + name,
		Enabled: true,
	})
	groupCache.Set(id, model.Group{
		ID:           id,
		Name:         name,
		EndpointType: endpointType,
		Items: []model.GroupItem{
			{ID: id, GroupID: id, ChannelID: channelID, ModelName: name, Priority: 1, Weight: 1},
		},
	})
}

func TestGroupListModelByEndpoint_AllReturnsVisibleModels(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "music-2.6", model.EndpointTypeMusicGeneration)
	setupValidGroup(2, "gpt-4o", model.EndpointTypeChat)

	models, err := GroupListModelByEndpoint("", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(models, "music-2.6") || !contains(models, "gpt-4o") {
		t.Errorf("empty endpoint should return all visible models, got: %v", models)
	}

	modelsStar, err := GroupListModelByEndpoint("*", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(modelsStar, "music-2.6") || !contains(modelsStar, "gpt-4o") {
		t.Errorf("* endpoint should return all visible models, got: %v", modelsStar)
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_MusicOnly(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "music-2.6", model.EndpointTypeMusicGeneration)
	setupValidGroup(2, "gpt-4o", model.EndpointTypeChat)

	models, err := GroupListModelByEndpoint("music_generation", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(models, "music-2.6") {
		t.Errorf("music_generation should include music-2.6, got: %v", models)
	}
	if contains(models, "gpt-4o") {
		t.Errorf("music_generation should NOT include gpt-4o, got: %v", models)
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_ChatExcludesMusicOnly(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "music-2.6", model.EndpointTypeMusicGeneration)
	setupValidGroup(2, "gpt-4o", model.EndpointTypeChat)

	models, err := GroupListModelByEndpoint("chat", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contains(models, "music-2.6") {
		t.Errorf("chat should NOT include music-2.6, got: %v", models)
	}
	if !contains(models, "gpt-4o") {
		t.Errorf("chat should include gpt-4o, got: %v", models)
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_ConversationFamilyMatchesResponses(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "gpt-4.1", model.EndpointTypeResponses)

	models, err := GroupListModelByEndpoint("chat", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(models, "gpt-4.1") {
		t.Errorf("chat should include responses model gpt-4.1, got: %v", models)
	}

	models2, err := GroupListModelByEndpoint("messages", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(models2, "gpt-4.1") {
		t.Errorf("messages should include responses model gpt-4.1, got: %v", models2)
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_GlobalGroupMatchesAll(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "universal-model", model.EndpointTypeAll)

	for _, ep := range []string{"chat", "music_generation", "embeddings", "image_generation"} {
		t.Run("endpoint="+ep, func(t *testing.T) {
			models, err := GroupListModelByEndpoint(ep, context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !contains(models, "universal-model") {
				t.Errorf("%s should include universal-model (endpoint_type=*), got: %v", ep, models)
			}
		})
	}

	chCache.Clear()
}

func TestGroupListModelByEndpoint_InvalidGroupNotListed(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	chCache.Set(999, model.Channel{ID: 999, Name: "disabled", Enabled: false})
	groupCache.Set(99, model.Group{
		ID:           99,
		Name:         "dead-model",
		EndpointType: model.EndpointTypeChat,
		Items: []model.GroupItem{
			{ID: 99, GroupID: 99, ChannelID: 999, ModelName: "dead-model", Priority: 1, Weight: 1},
		},
	})

	models, err := GroupListModelByEndpoint("chat", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contains(models, "dead-model") {
		t.Errorf("disabled channel should be excluded, got: %v", models)
	}

	chCache.Clear()
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// --- GroupItem 启用开关（模型 + 渠道 行）测试 ---

// 分组内该模型只有一行且被禁用 → 模型不再出现在 /v1/models。
func TestGroupListModel_DisabledItemNotListed(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	chCache.Set(300, model.Channel{ID: 300, Name: "enabled-ch", Enabled: true})
	disabled := false
	groupCache.Set(4, model.Group{
		ID:           4,
		Name:         "rate-limited-model",
		EndpointType: model.EndpointTypeChat,
		Items: []model.GroupItem{
			{ID: 3, GroupID: 4, ChannelID: 300, ModelName: "rate-limited-model", Priority: 1, Weight: 1, Enabled: &disabled},
		},
	})

	models, err := GroupListModel(context.Background())
	if err != nil {
		t.Fatalf("GroupListModel returned error: %v", err)
	}
	if contains(models, "rate-limited-model") {
		t.Errorf("disabled item should be excluded from /v1/models, got: %v", models)
	}

	chCache.Clear()
}

// 组内一行禁用、另一行启用 → 模型仍可见（部分禁用不影响整体可用性）。
func TestGroupListModel_PartiallyDisabledItemsStillListed(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	chCache.Set(400, model.Channel{ID: 400, Name: "ch-a", Enabled: true})
	chCache.Set(401, model.Channel{ID: 401, Name: "ch-b", Enabled: true})
	disabled := false
	groupCache.Set(5, model.Group{
		ID:           5,
		Name:         "deepseek-flash",
		EndpointType: model.EndpointTypeChat,
		Items: []model.GroupItem{
			{ID: 4, GroupID: 5, ChannelID: 400, ModelName: "deepseek-flash", Priority: 1, Weight: 1, Enabled: &disabled},
			{ID: 5, GroupID: 5, ChannelID: 401, ModelName: "deepseek-flash", Priority: 2, Weight: 1},
		},
	})

	models, err := GroupListModel(context.Background())
	if err != nil {
		t.Fatalf("GroupListModel returned error: %v", err)
	}
	if !contains(models, "deepseek-flash") {
		t.Errorf("model should stay visible while a sibling item is enabled, got: %v", models)
	}

	chCache.Clear()
}

// finalizeMatchedGroup 是选路收窄的唯一入口：禁用行必须被摘掉，
// 且不影响同组内启用的行。
func TestFinalizeMatchedGroup_DropsDisabledItems(t *testing.T) {
	chCache := channel.GetCache()
	chCache.Clear()

	chCache.Set(500, model.Channel{ID: 500, Name: "ch-a", Enabled: true})
	chCache.Set(501, model.Channel{ID: 501, Name: "ch-b", Enabled: true})
	disabled := false
	enabled := true

	group := model.Group{
		ID:           6,
		Name:         "g",
		EndpointType: model.EndpointTypeChat,
		Items: []model.GroupItem{
			{ID: 10, GroupID: 6, ChannelID: 500, ModelName: "m", Priority: 1, Weight: 1, Enabled: &disabled},
			{ID: 11, GroupID: 6, ChannelID: 501, ModelName: "m", Priority: 2, Weight: 1, Enabled: &enabled},
			// nil = 存量行语义，必须保留。
			{ID: 12, GroupID: 6, ChannelID: 500, ModelName: "m2", Priority: 3, Weight: 1},
		},
	}

	finalized := finalizeMatchedGroup(group)
	if len(finalized.Items) != 2 {
		t.Fatalf("expected 2 surviving items (disabled dropped), got %d: %+v", len(finalized.Items), finalized.Items)
	}
	for _, item := range finalized.Items {
		if item.ID == 10 {
			t.Errorf("disabled item 10 must not survive routing narrowing")
		}
	}
	if finalized.Items[0].ID != 11 || finalized.Items[1].ID != 12 {
		t.Errorf("expected surviving items [11,12], got [%d,%d]", finalized.Items[0].ID, finalized.Items[1].ID)
	}

	chCache.Clear()
}

// --- GroupListModelCapabilities tests ---

func TestGroupListModelCapabilities_ChatOnly(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "gpt-4o", model.EndpointTypeChat)

	caps, err := GroupListModelCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(caps))
	}
	c := caps[0]
	if c.Name != "gpt-4o" {
		t.Errorf("name = %q, want gpt-4o", c.Name)
	}
	if !c.Conversation {
		t.Error("gpt-4o should be conversation")
	}
	if !c.Available {
		t.Error("gpt-4o should be available")
	}
	if len(c.Endpoints) != 1 || c.Endpoints[0] != model.EndpointTypeChat {
		t.Errorf("endpoints = %v, want [chat]", c.Endpoints)
	}

	chCache.Clear()
}

func TestGroupListModelCapabilities_MusicOnly(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "music-2.6", model.EndpointTypeMusicGeneration)

	caps, err := GroupListModelCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(caps))
	}
	c := caps[0]
	if c.Conversation {
		t.Error("music-2.6 should NOT be conversation")
	}
	if len(c.Endpoints) != 1 || c.Endpoints[0] != model.EndpointTypeMusicGeneration {
		t.Errorf("endpoints = %v, want [music_generation]", c.Endpoints)
	}

	chCache.Clear()
}

func TestGroupListModelCapabilities_MultiEndpointAggregation(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	// Same model name across two groups with different endpoint types
	setupValidGroup(1, "gpt-4.1", model.EndpointTypeResponses)
	setupValidGroup(2, "gpt-4.1", model.EndpointTypeMessages)

	caps, err := GroupListModelCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability (aggregated), got %d: %+v", len(caps), caps)
	}
	c := caps[0]
	if c.Name != "gpt-4.1" {
		t.Errorf("name = %q, want gpt-4.1", c.Name)
	}
	if !c.Conversation {
		t.Error("gpt-4.1 should be conversation (has responses + messages)")
	}
	if len(c.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %v", c.Endpoints)
	}

	chCache.Clear()
}

func TestGroupListModelCapabilities_GlobalStarGroup(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	setupValidGroup(1, "universal-model", model.EndpointTypeAll)

	caps, err := GroupListModelCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(caps))
	}
	c := caps[0]
	if c.Name != "universal-model" {
		t.Errorf("name = %q, want universal-model", c.Name)
	}
	if len(c.Endpoints) != 1 || c.Endpoints[0] != model.EndpointTypeAll {
		t.Errorf("endpoints = %v, want [*]", c.Endpoints)
	}

	chCache.Clear()
}

func TestGroupListModelCapabilities_InvalidGroupsExcluded(t *testing.T) {
	groupCache.Clear()
	chCache := channel.GetCache()
	chCache.Clear()

	// Empty group — should not appear
	groupCache.Set(99, model.Group{
		ID:           99,
		Name:         "empty-model",
		EndpointType: model.EndpointTypeChat,
		Items:        nil,
	})

	// Disabled channel — should not appear
	chCache.Set(888, model.Channel{ID: 888, Name: "disabled-ch", Enabled: false})
	groupCache.Set(98, model.Group{
		ID:           98,
		Name:         "dead-model",
		EndpointType: model.EndpointTypeChat,
		Items: []model.GroupItem{
			{ID: 98, GroupID: 98, ChannelID: 888, ModelName: "dead-model", Priority: 1, Weight: 1},
		},
	})

	// Valid group
	setupValidGroup(1, "real-model", model.EndpointTypeChat)

	caps, err := GroupListModelCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability (only real-model), got %d: %+v", len(caps), caps)
	}
	if caps[0].Name != "real-model" {
		t.Errorf("expected real-model, got %q", caps[0].Name)
	}

	chCache.Clear()
}
