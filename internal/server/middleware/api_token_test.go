package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/apitoken"
	"github.com/lingyuins/octopus/internal/server/auth"
)

const apiTokenTestUsername = "hermes-agent"

// setupAPITokenMiddlewareTest 准备内存库 + 绑定用户，并返回一枚已创建的机器令牌。
func setupAPITokenMiddlewareTest(t *testing.T) (string, model.User, model.APIToken) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name()))
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	bound := model.User{Username: apiTokenTestUsername, Password: "unused-machine-identity", Role: model.UserRoleAdmin}
	if err := db.GetDB().Create(&bound).Error; err != nil {
		t.Fatalf("create bound user: %v", err)
	}

	token, row, err := apitoken.Create(t.Context(), model.APITokenCreateRequest{
		Name: "cli", Username: apiTokenTestUsername,
	}, bound.ID)
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}
	return token, bound, row
}

func newAuthProbeEngine() *gin.Engine {
	engine := gin.New()
	engine.GET("/probe", Auth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":      c.GetInt("user_id"),
			"username":     c.GetString("username"),
			"role":         c.GetString("user_role"),
			"api_token_id": c.GetInt("api_token_id"),
			"api_token":    c.GetString("api_token_name"),
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

func TestAuthMiddlewareAcceptsAPIToken(t *testing.T) {
	token, bound, row := setupAPITokenMiddlewareTest(t)
	engine := newAuthProbeEngine()

	w := probeAuthRequest(engine, "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for api token, got %d (%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		`"username":"` + apiTokenTestUsername + `"`,
		`"role":"` + model.UserRoleAdmin + `"`,
		fmt.Sprintf(`"user_id":%d`, bound.ID),
		fmt.Sprintf(`"api_token_id":%d`, row.ID),
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %s in context, got %s", want, body)
		}
	}

	// 篡改的令牌、非机器令牌凭证、空 Authorization 都应被拒。
	if code := probeAuthRequest(engine, "Bearer "+token+"x").Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for tampered token, got %d", code)
	}
	if code := probeAuthRequest(engine, "Bearer sk-octopus-not-a-real-key").Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for non-token credential, got %d", code)
	}
	if code := probeAuthRequest(engine, "").Code; code != http.StatusBadRequest {
		t.Fatalf("expected 400 when Authorization is missing, got %d", code)
	}

	// 吊销后旧令牌立即失效。
	if err := apitoken.Revoke(t.Context(), row.ID); err != nil {
		t.Fatalf("revoke api token: %v", err)
	}
	if code := probeAuthRequest(engine, "Bearer "+token).Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after revocation, got %d", code)
	}
}

// TestAuthMiddlewareRejectsAPITokenWithUnknownBoundUser 保证绑定用户被删掉后
// 令牌不会退化成匿名放行。
func TestAuthMiddlewareRejectsAPITokenWithUnknownBoundUser(t *testing.T) {
	token, bound, _ := setupAPITokenMiddlewareTest(t)
	engine := newAuthProbeEngine()

	if err := db.GetDB().Delete(&model.User{}, bound.ID).Error; err != nil {
		t.Fatalf("delete bound user: %v", err)
	}
	if code := probeAuthRequest(engine, "Bearer "+token).Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when bound user is missing, got %d", code)
	}
}

// TestAuthMiddlewareRejectsExpiredAPIToken 覆盖有效期。
func TestAuthMiddlewareRejectsExpiredAPIToken(t *testing.T) {
	token, bound, row := setupAPITokenMiddlewareTest(t)
	engine := newAuthProbeEngine()

	if err := db.GetDB().Model(&model.APIToken{}).Where("id = ?", row.ID).
		Update("expires_at", int64(1)).Error; err != nil {
		t.Fatalf("expire token: %v", err)
	}
	if code := probeAuthRequest(engine, "Bearer "+token).Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d (user %d)", code, bound.ID)
	}
}

// TestAuthMiddlewareKeepsJWTPath 保证原有 JWT 通路行为不变。
func TestAuthMiddlewareKeepsJWTPath(t *testing.T) {
	_, bound, _ := setupAPITokenMiddlewareTest(t)
	conf.AppConfig.Auth.JWTSecret = "api-token-test-jwt-secret"
	engine := newAuthProbeEngine()

	jwt, _, err := auth.GenerateJWTToken(60, bound.ID, model.UserRoleAdmin)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}
	w := probeAuthRequest(engine, "Bearer "+jwt)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for jwt, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"username":"`+apiTokenTestUsername+`"`) {
		t.Fatalf("expected jwt identity in context, got %s", w.Body.String())
	}

	if code := probeAuthRequest(engine, "Bearer not-a-jwt").Code; code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for garbage credential, got %d", code)
	}
}
