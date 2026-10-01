package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// setupAgentTokenTestDB 准备一个内存库并补种全部默认设置（含 Agent 令牌相关键）。
func setupAgentTokenTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()))
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := setting.RefreshCache(context.Background()); err != nil {
		t.Fatalf("refresh setting cache: %v", err)
	}
}

func TestGenerateAgentTokenFormat(t *testing.T) {
	first, err := GenerateAgentToken()
	if err != nil {
		t.Fatalf("generate agent token: %v", err)
	}
	second, err := GenerateAgentToken()
	if err != nil {
		t.Fatalf("generate agent token: %v", err)
	}
	if !strings.HasPrefix(first, AgentTokenPrefix) {
		t.Fatalf("token missing prefix %q: %q", AgentTokenPrefix, first)
	}
	if len(first) != len(AgentTokenPrefix)+agentTokenRandomLength {
		t.Fatalf("unexpected token length: %d", len(first))
	}
	if first == second {
		t.Fatal("expected two generated tokens to differ")
	}
	for _, ch := range strings.TrimPrefix(first, AgentTokenPrefix) {
		if !strings.ContainsRune(agentTokenKeyChars, ch) {
			t.Fatalf("unexpected character %q in token", ch)
		}
	}
}

func TestHashAgentTokenIsStableAndTrims(t *testing.T) {
	token, err := GenerateAgentToken()
	if err != nil {
		t.Fatalf("generate agent token: %v", err)
	}
	hash := HashAgentToken(token)
	if len(hash) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(hash))
	}
	if hash != HashAgentToken("  "+token+"  ") {
		t.Fatal("expected hash to ignore surrounding whitespace")
	}
	if hash != strings.ToLower(hash) {
		t.Fatal("expected lowercase hex hash")
	}
}

// TestAgentTokenLifecycle 覆盖默认关闭 → 激活 → 校验 → 吊销的完整闭环。
func TestAgentTokenLifecycle(t *testing.T) {
	setupAgentTokenTestDB(t)

	status := GetAgentTokenStatus()
	if status.Enabled || status.TokenSet {
		t.Fatalf("expected agent token to be disabled and unset by default: %+v", status)
	}

	token, err := GenerateAgentToken()
	if err != nil {
		t.Fatalf("generate agent token: %v", err)
	}

	// 未激活前：即便凭证格式正确也必须拒绝（默认关闭）。
	if ok, _ := VerifyAgentToken(token); ok {
		t.Fatal("expected token to be rejected before activation")
	}

	if err := ActivateAgentToken(token, "hermes-agent"); err != nil {
		t.Fatalf("activate agent token: %v", err)
	}

	ok, username := VerifyAgentToken(token)
	if !ok || username != "hermes-agent" {
		t.Fatalf("expected token to verify as hermes-agent, got ok=%v username=%q", ok, username)
	}

	// 摘要入库：设置里不应出现明文令牌。
	storedHash, err := setting.GetString("agent_api_token_hash")
	if err != nil {
		t.Fatalf("read stored hash: %v", err)
	}
	if storedHash == token || strings.Contains(storedHash, token) {
		t.Fatal("expected only the digest to be persisted, not the plaintext token")
	}

	status = GetAgentTokenStatus()
	if !status.Enabled || !status.TokenSet || status.Username != "hermes-agent" || status.CreatedAt == "" {
		t.Fatalf("unexpected status after activation: %+v", status)
	}
	if status.Prefix != AgentTokenDisplayPrefix(token) {
		t.Fatalf("unexpected prefix: %q", status.Prefix)
	}
	if len(status.Prefix) >= len(token) {
		t.Fatal("expected display prefix to be shorter than the token")
	}

	// 篡改/无前缀/空凭证都必须拒绝。
	for _, candidate := range []string{
		token + "x",
		strings.TrimSuffix(token, "x")[:len(token)-1],
		strings.TrimPrefix(token, AgentTokenPrefix),
		"sk-octopus-" + strings.TrimPrefix(token, AgentTokenPrefix),
		"",
	} {
		if ok, _ := VerifyAgentToken(candidate); ok {
			t.Fatalf("expected candidate %q to be rejected", candidate)
		}
	}

	if err := RevokeAgentToken(); err != nil {
		t.Fatalf("revoke agent token: %v", err)
	}
	if ok, _ := VerifyAgentToken(token); ok {
		t.Fatal("expected token to be rejected after revocation")
	}
	status = GetAgentTokenStatus()
	if status.Enabled || status.TokenSet || status.Prefix != "" || status.CreatedAt != "" {
		t.Fatalf("unexpected status after revocation: %+v", status)
	}
}

func TestActivateAgentTokenRejectsInvalidInput(t *testing.T) {
	setupAgentTokenTestDB(t)

	if err := ActivateAgentToken("ok-agent-whatever", "  "); err == nil {
		t.Fatal("expected empty username to be rejected")
	}
	if err := ActivateAgentToken("not-a-prefixed-token", "hermes-agent"); err == nil {
		t.Fatal("expected token without the agent prefix to be rejected")
	}
}
