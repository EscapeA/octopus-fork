package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	wa "github.com/lingyuins/octopus/internal/op/webauthn"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"gorm.io/gorm"
)

func init() {
	// 公开路由：Passkey 登录
	publicWebAuthn := router.NewGroupRouter("/api/v1/webauthn").
		Use(middleware.RequireJSON())
	publicWebAuthn.AddRoute(
		router.NewRoute("/status", http.MethodGet).Handle(webauthnStatus),
	)
	publicWebAuthn.AddRoute(
		router.NewRoute("/login/begin", http.MethodPost).Handle(webauthnLoginBegin),
	)
	publicWebAuthn.AddRoute(
		router.NewRoute("/login/finish", http.MethodPost).
			Use(middleware.LoginRateLimit()).
			Handle(webauthnLoginFinish),
	)

	// 鉴权路由：凭证管理
	router.NewGroupRouter("/api/v1/webauthn").
		Use(middleware.Auth()).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/credentials", http.MethodGet).Handle(listWebAuthnCredentials),
		).
		AddRoute(
			router.NewRoute("/register/begin", http.MethodPost).Handle(webauthnRegisterBegin),
		).
		AddRoute(
			router.NewRoute("/register/finish", http.MethodPost).Handle(webauthnRegisterFinish),
		).
		AddRoute(
			router.NewRoute("/credentials/:id", http.MethodDelete).Handle(deleteWebAuthnCredential),
		)
}

// --- 公共状态 ---

func webauthnStatus(c *gin.Context) {
	_, err := wa.New()
	configured := err == nil
	resp.Success(c, gin.H{
		"enabled":        configured,
		"has_credentials": wa.HasAnyCredential(),
	})
}

// --- 登录 ---

func webauthnLoginBegin(c *gin.Context) {
	token, assertion, err := wa.BeginLogin()
	if err != nil {
		resp.Error(c, http.StatusBadRequest, webauthnErrorMessage(err))
		return
	}
	options, err := json.Marshal(assertion)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, gin.H{"session_token": token, "options": json.RawMessage(options)})
}

type webauthnFinishRequest struct {
	SessionToken string          `json:"session_token"`
	Name         string          `json:"name,omitempty"`
	Credential   json.RawMessage `json:"credential"`
}

func webauthnLoginFinish(c *gin.Context) {
	var req webauthnFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if req.SessionToken == "" || len(req.Credential) == 0 {
		resp.Error(c, http.StatusBadRequest, "missing session_token or credential")
		return
	}
	loginKey := c.GetString("login_rate_limit_key")
	user, err := wa.FinishLogin(req.SessionToken, req.Credential)
	if err != nil {
		if !errors.Is(err, wa.ErrInvalidToken) {
			middleware.RecordLoginFailure(loginKey, time.Now())
		}
		status := http.StatusUnauthorized
		if errors.Is(err, wa.ErrInvalidToken) {
			status = http.StatusBadRequest
		}
		resp.Error(c, status, webauthnErrorMessage(err))
		return
	}
	middleware.ClearLoginFailures(loginKey)
	token, expire, err := auth.GenerateJWTToken(0, user.ID, user.Role)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, resp.ErrInternalServer)
		return
	}
	resp.Success(c, model.UserLoginResponse{Token: token, ExpireAt: expire})
}

// --- 凭证管理 ---

func listWebAuthnCredentials(c *gin.Context) {
	currentUserID := uint(c.GetInt("user_id"))
	creds, err := wa.ListCredentials(currentUserID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, creds)
}

func webauthnRegisterBegin(c *gin.Context) {
	currentUserID := uint(c.GetInt("user_id"))
	token, creation, err := wa.BeginRegistration(currentUserID)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, webauthnErrorMessage(err))
		return
	}
	options, err := json.Marshal(creation)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, gin.H{"session_token": token, "options": json.RawMessage(options)})
}

func webauthnRegisterFinish(c *gin.Context) {
	var req webauthnFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if req.SessionToken == "" || len(req.Credential) == 0 {
		resp.Error(c, http.StatusBadRequest, "missing session_token or credential")
		return
	}
	if err := wa.FinishRegistration(req.SessionToken, req.Credential, req.Name); err != nil {
		resp.Error(c, http.StatusBadRequest, webauthnErrorMessage(err))
		return
	}
	resp.Success(c, nil)
}

func deleteWebAuthnCredential(c *gin.Context) {
	currentUserID := uint(c.GetInt("user_id"))
	idStr := c.Param("id")
	id, err := parseUintParam(idStr)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid credential id")
		return
	}
	if err := wa.DeleteCredential(currentUserID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Error(c, http.StatusNotFound, "credential not found")
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func webauthnErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, wa.ErrNotConfigured):
		return "Passkey login is not configured"
	case errors.Is(err, wa.ErrInvalidToken):
		return "Passkey session expired, please try again"
	default:
		return "Passkey authentication failed"
	}
}

func parseUintParam(s string) (uint, error) {
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
