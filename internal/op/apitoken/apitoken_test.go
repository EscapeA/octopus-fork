package apitoken

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

const testBoundUsername = "hermes-agent"

func setupAPITokenTestDB(t *testing.T) model.User {
	t.Helper()

	dsn := "file:" + strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()) + "?mode=memory&cache=shared"
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	bound := model.User{Username: testBoundUsername, Password: "unused-machine-identity", Role: model.UserRoleAdmin}
	if err := db.GetDB().Create(&bound).Error; err != nil {
		t.Fatalf("create bound user: %v", err)
	}
	return bound
}

func TestGenerateTokenFormat(t *testing.T) {
	first, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.HasPrefix(first, TokenPrefix) {
		t.Fatalf("token missing prefix %q: %q", TokenPrefix, first)
	}
	if len(first) != len(TokenPrefix)+tokenRandomLength {
		t.Fatalf("unexpected token length %d", len(first))
	}
	if first == second {
		t.Fatal("expected two tokens to differ")
	}
	hash := Hash(first)
	if len(hash) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(hash))
	}
	if hash != Hash("  "+first+"  ") {
		t.Fatal("expected hash to ignore surrounding whitespace")
	}
	if prefix := DisplayPrefix(first); len(prefix) != tokenPrefixKeep || !strings.HasPrefix(prefix, TokenPrefix) {
		t.Fatalf("unexpected display prefix %q", prefix)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	setupAPITokenTestDB(t)
	ctx := context.Background()

	if _, _, err := Create(ctx, model.APITokenCreateRequest{Name: "  ", Username: testBoundUsername}, 0); err == nil {
		t.Fatal("expected empty name to be rejected")
	}
	if _, _, err := Create(ctx, model.APITokenCreateRequest{Name: "cli"}, 0); err == nil {
		t.Fatal("expected empty username to be rejected")
	}
	if _, _, err := Create(ctx, model.APITokenCreateRequest{Name: "cli", Username: "nobody"}, 0); err == nil {
		t.Fatal("expected unknown bound user to be rejected")
	}
	if _, _, err := Create(ctx, model.APITokenCreateRequest{Name: "cli", Username: testBoundUsername, ExpiresDays: -1}, 0); err == nil {
		t.Fatal("expected negative expires_days to be rejected")
	}
}

func TestTokenLifecycle(t *testing.T) {
	bound := setupAPITokenTestDB(t)
	ctx := context.Background()

	token, row, err := Create(ctx, model.APITokenCreateRequest{Name: "cli", Username: testBoundUsername, ExpiresDays: 7}, 7)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if row.ID == 0 || row.UserID != bound.ID || row.ExpiresAt == 0 {
		t.Fatalf("unexpected created row: %+v", row)
	}

	got, ok := Authenticate(token, "100.64.0.9")
	if !ok || got.ID != row.ID {
		t.Fatalf("expected token to authenticate, got ok=%v id=%d", ok, got.ID)
	}

	// 摘要入库：表里不应出现明文。
	var stored model.APIToken
	if err := db.GetDB().First(&stored, row.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if stored.TokenHash == token || strings.Contains(stored.TokenHash, token) {
		t.Fatal("expected only the digest to be persisted")
	}

	// 篡改/无前缀/空凭证都必须拒绝。
	bad := []string{token + "x", token[:len(token)-1], strings.TrimPrefix(token, TokenPrefix), "sk-octopus-" + strings.TrimPrefix(token, TokenPrefix), ""}
	for _, candidate := range bad {
		if _, ok := Authenticate(candidate, "100.64.0.9"); ok {
			t.Fatalf("expected candidate %q to be rejected", candidate)
		}
	}

	// 列表视图不含摘要，且能看到绑定用户。
	views, err := List(ctx)
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(views) != 1 || !views[0].UserExists || views[0].Prefix != row.Prefix || views[0].Username != testBoundUsername {
		t.Fatalf("unexpected list view: %+v", views)
	}

	// 吊销后立即失效。
	if err := Revoke(ctx, row.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, ok := Authenticate(token, "100.64.0.9"); ok {
		t.Fatal("expected revoked token to be rejected")
	}

	// 硬删除后列表为空。
	if err := Delete(ctx, row.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	views, err = List(ctx)
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("expected empty list after delete, got %+v", views)
	}
}

func TestTokenExpiry(t *testing.T) {
	setupAPITokenTestDB(t)
	ctx := context.Background()

	token, row, err := Create(ctx, model.APITokenCreateRequest{Name: "short", Username: testBoundUsername, ExpiresDays: 1}, 0)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if _, ok := Authenticate(token, "127.0.0.1"); !ok {
		t.Fatal("expected fresh token to authenticate")
	}

	if err := db.GetDB().Model(&model.APIToken{}).Where("id = ?", row.ID).
		Update("expires_at", time.Now().Add(-time.Minute).Unix()).Error; err != nil {
		t.Fatalf("expire token: %v", err)
	}
	if _, ok := Authenticate(token, "127.0.0.1"); ok {
		t.Fatal("expected expired token to be rejected")
	}
}

// TestAuthenticateThrottlesLastUsed 保证最后使用时间按分钟粒度写库，避免逐请求写放大。
func TestAuthenticateThrottlesLastUsed(t *testing.T) {
	setupAPITokenTestDB(t)
	ctx := context.Background()

	token, row, err := Create(ctx, model.APITokenCreateRequest{Name: "cli", Username: testBoundUsername}, 0)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if _, ok := Authenticate(token, "10.0.0.1"); !ok {
		t.Fatal("expected token to authenticate")
	}

	var first model.APIToken
	if err := db.GetDB().First(&first, row.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if first.LastUsedAt == 0 || first.LastUsedIP != "10.0.0.1" {
		t.Fatalf("expected last used to be recorded, got %+v", first)
	}

	// 同一分钟内、同一 IP 再次使用：不刷新时间戳（分钟粒度节流）。
	time.Sleep(1100 * time.Millisecond)
	if _, ok := Authenticate(token, "10.0.0.1"); !ok {
		t.Fatal("expected token to authenticate again")
	}
	var second model.APIToken
	if err := db.GetDB().First(&second, row.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if second.LastUsedAt != first.LastUsedAt {
		t.Fatalf("expected last_used_at to be throttled, got %d -> %d", first.LastUsedAt, second.LastUsedAt)
	}
}
