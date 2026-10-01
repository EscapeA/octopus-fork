package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/server/auth"
)

const agentTokenTestUsername = "hermes-agent"

// setupAgentTokenMiddlewareTest 准备内存库 + 默认设置 + 绑定用户，并返回一枚
// 已激活的 Agent 令牌。
func setupAgentTokenMiddlewareTest(t *testing.T) (string, model.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)

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

	bound := model.User{Username: agentTokenTestUsername, Password: "unused-machine-identity", Role: model.UserRoleAdmin}
	if err := db.GetDB().Create(&bound).Error; err != nil {
		t.Fatalf("create bound user: %v", err)
	}

	token, err := auth.GenerateAgentToken()
	if err != nil {
		t.Fatalf("generate agent token: %v", err)
	}
	if err := auth.ActivateAgentToken(token, agentTokenTestUsername); err != nil {
		t.Fatalf("activate agent token: %v", err)
	}
	return token, bound
}

func newAuthProbeEngine() *gin.Engine {
	engine := gin.New()
	engine.GET("/probe", Auth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":  c.GetInt("user_id"),
			"username": c.GetString("username"),
			"role":     c.GetString("user_role"),
		})
	})
	return engine
}

func probeAuthRequest(engine *gin.Engine, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestAuthMiddlewareAcceptsAgentToken(t *testing.T) {
	token, bound := setupAgentTokenMiddlewareTest(t)
	engine := newAuthProbeEngine()

	w := probeAuthRequest(engine, "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent token, got %d (%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"username":"`+agentTokenTestUsername+`"`) {
		t.Fatalf("expected bound username in context, got %s", body)
	}
	if !strings.Contains(body, `"role":"`+model.UserRoleAdmin+`"`) {
		t.Fatalf("expected role from the bound user, got %s", body)
	}
	if !strings.Contains(body, fmt.Sprintf(`"user_id":%d`, bound.ID)) {
		t.Fatalf("expected bound user id in context, got %s", body)
	}

	// 篡改的令牌、无前缀的令牌、空 Authorization 都应被拒。
	if code := probeAuthRequest(engine, "Bearer "+token+"x").Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for tampered token, got %d", code)
	}
	if code := probeAuthRequest(engine, "Bearer sk-octopus-not-a-real-key").Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for non-agent credential, got %d", code)
	}
	if code := probeAuthRequest(engine, "").Code; code != http.StatusBadRequest {
		t.Fatalf("expected 400 when Authorization is missing, got %d", code)
	}

	// 吊销后旧令牌立即失效。
	if err := auth.RevokeAgentToken(); err != nil {
		t.Fatalf("revoke agent token: %v", err)
	}
	if code := probeAuthRequest(engine, "Bearer "+token).Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after revocation, got %d", code)
	}
}

// TestAuthMiddlewareRejectsAgentTokenWithUnknownBoundUser 保证绑定用户被删掉后
// 令牌不会退化成匿名放行。
func TestAuthMiddlewareRejectsAgentTokenWithUnknownBoundUser(t *testing.T) {
	token, bound := setupAgentTokenMiddlewareTest(t)
	engine := newAuthProbeEngine()

	if err := db.GetDB().Delete(&model.User{}, bound.ID).Error; err != nil {
		t.Fatalf("delete bound user: %v", err)
	}
	if code := probeAuthRequest(engine, "Bearer "+token).Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when bound user is missing, got %d", code)
	}
}

// TestAuthMiddlewareKeepsJWTPath 保证原有 JWT 通路行为不变。
func TestAuthMiddlewareKeepsJWTPath(t *testing.T) {
	_, bound := setupAgentTokenMiddlewareTest(t)
	conf.AppConfig.Auth.JWTSecret = "agent-token-test-jwt-secret"
	engine := newAuthProbeEngine()

	jwt, _, err := auth.GenerateJWTToken(60, bound.ID, model.UserRoleAdmin)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}
	w := probeAuthRequest(engine, "Bearer "+jwt)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for jwt, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"username":"`+agentTokenTestUsername+`"`) {
		t.Fatalf("expected jwt identity in context, got %s", w.Body.String())
	}

	if code := probeAuthRequest(engine, "Bearer not-a-jwt").Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for garbage credential, got %d", code)
	}
}
