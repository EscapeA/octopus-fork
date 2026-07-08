package sitesync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

// TestSyncManagementPlatformSkipModelSyncSkipsModelFetch 是 issue #130 的回归测试：
// SkipModelSync=true 时，站点账号同步只拉取令牌/分组/余额，跳过模型列表拉取，
// 不请求 /models 端点，snapshot.models 为空，但 token/group 仍正常同步。
func TestSyncManagementPlatformSkipModelSyncSkipsModelFetch(t *testing.T) {
	modelEndpointCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":11494,"username":"managed-user"}}`))
		case r.URL.Path == "/api/token/":
			_, _ = w.Write([]byte(`{"data":{"items":[{"name":"primary","key":"managed-key","group":"vip","status":1}]}}`))
		case r.URL.Path == "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"data":[{"id":"vip","name":"VIP"}]}`))
		case r.URL.Path == "/models":
			modelEndpointCalled = true
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	snapshot, err := syncManagementPlatform(context.Background(), &model.Site{
		Platform: model.SitePlatformNewAPI,
		BaseURL:  server.URL,
	}, &model.SiteAccount{
		Name:           "managed-user",
		CredentialType: model.SiteCredentialTypeAccessToken,
		AccessToken:    "test-access-token",
		Enabled:        true,
		AutoSync:       true,
		SkipModelSync:  true,
	})
	if err != nil {
		t.Fatalf("syncManagementPlatform returned error: %v", err)
	}

	// /models 端点不应被调用。
	if modelEndpointCalled {
		t.Fatal("model endpoint /models was called despite SkipModelSync=true")
	}

	// 令牌和分组仍应正常同步。
	if len(snapshot.tokens) != 1 || snapshot.tokens[0].Token != "managed-key" {
		t.Fatalf("unexpected synced tokens: %+v", snapshot.tokens)
	}
	if len(snapshot.groups) != 1 || snapshot.groups[0].GroupKey != "vip" {
		t.Fatalf("unexpected synced groups: %+v", snapshot.groups)
	}

	// 模型应为空（跳过拉取，不新增模型；历史模型由 persistSyncSnapshot 保留）。
	if len(snapshot.models) != 0 {
		t.Fatalf("expected empty models when SkipModelSync=true, got %d: %+v", len(snapshot.models), snapshot.models)
	}

	// 每个分组的同步结果应为 skipped 状态（Authoritative=true，不计入失败）。
	if len(snapshot.groupResults) == 0 {
		t.Fatal("expected non-empty group results")
	}
	for _, gr := range snapshot.groupResults {
		if gr.GroupKey == "vip" {
			if gr.Status != siteGroupSyncStatusSkipped {
				t.Fatalf("group %s status = %s, want %s", gr.GroupKey, gr.Status, siteGroupSyncStatusSkipped)
			}
			if !gr.Authoritative {
				t.Fatalf("group %s should be authoritative (skipped is a deliberate success)", gr.GroupKey)
			}
		}
	}

	// 整体状态应为 Success（跳过模型同步不是失败）。
	if snapshot.status != model.SiteExecutionStatusSuccess {
		t.Fatalf("snapshot status = %s, want %s", snapshot.status, model.SiteExecutionStatusSuccess)
	}
}

// TestSyncSub2APISkipModelSyncSkipsModelFetch 验证 sub2api 平台 SkipModelSync=true
// 同样跳过模型拉取。
func TestSyncSub2APISkipModelSyncSkipsModelFetch(t *testing.T) {
	modelEndpointCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v1/tokens":
			_, _ = w.Write([]byte(`{"data":{"items":[{"name":"default","key":"sub-key"}]}}`))
		case "/v1/models", "/antigravity/v1/models":
			modelEndpointCalled = true
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-3"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	snapshot, err := syncSub2API(context.Background(), &model.Site{
		Platform: model.SitePlatformSub2API,
		BaseURL:  server.URL,
	}, &model.SiteAccount{
		Name:           "sub2api-user",
		CredentialType: model.SiteCredentialTypeAPIKey,
		APIKey:         "sub-key",
		Enabled:        true,
		AutoSync:       true,
		SkipModelSync:  true,
	})
	if err != nil {
		t.Fatalf("syncSub2API returned error: %v", err)
	}

	if modelEndpointCalled {
		t.Fatal("model endpoint was called despite SkipModelSync=true")
	}
	if len(snapshot.tokens) == 0 {
		t.Fatal("expected tokens to be synced even with SkipModelSync=true")
	}
	if len(snapshot.models) != 0 {
		t.Fatalf("expected empty models when SkipModelSync=true, got %d", len(snapshot.models))
	}
	if snapshot.status != model.SiteExecutionStatusSuccess {
		t.Fatalf("snapshot status = %s, want %s", snapshot.status, model.SiteExecutionStatusSuccess)
	}
}

// TestSkipModelSyncGroupResultsProducesUnresolvedStatus 验证辅助函数生成
// 正确的 skipped 结果（Authoritative=true，状态=skipped）。
func TestSkipModelSyncGroupResultsProducesSkippedStatus(t *testing.T) {
	tokens := []model.SiteToken{
		{Token: "key1", GroupKey: "group1", GroupName: "Group 1"},
		{Token: "key2", GroupKey: "group2", GroupName: "Group 2"},
	}
	models, results := skipModelSyncGroupResults(tokens)

	if len(models) != 0 {
		t.Fatalf("expected empty models, got %d", len(models))
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 group results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != siteGroupSyncStatusSkipped {
			t.Fatalf("group %s status = %s, want %s", r.GroupKey, r.Status, siteGroupSyncStatusSkipped)
		}
		if !r.Authoritative {
			t.Fatalf("group %s should be authoritative", r.GroupKey)
		}
	}
}
