package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	usr "github.com/lingyuins/octopus/internal/op/user"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

// Agent 令牌管理：让机器身份（脚本 / agent / CI）无需管理员账号密码即可调用
// 管理面 API。令牌明文只在 rotate 响应里返回一次，库里只存 SHA-256 摘要。
func init() {
	agentTokenRoutes := router.NewGroupRouter("/api/v1/agent-token").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSettingsRead))

	agentTokenRoutes.AddRoute(
		router.NewRoute("/status", http.MethodGet).
			Handle(getAgentTokenStatus),
	)

	agentTokenRoutes.AddRoute(
		router.NewRoute("/rotate", http.MethodPost).
			Use(middleware.RequirePermission(auth.PermSettingsWrite)).
			Use(middleware.RequireJSON()).
			Handle(rotateAgentToken),
	)

	agentTokenRoutes.AddRoute(
		router.NewRoute("/revoke", http.MethodPost).
			Use(middleware.RequirePermission(auth.PermSettingsWrite)).
			Use(middleware.RequireJSON()).
			Handle(revokeAgentToken),
	)
}

type rotateAgentTokenRequest struct {
	// Username 令牌绑定的用户，必须已存在；鉴权时按该用户的角色走 RBAC。
	Username string `json:"username"`
}

func getAgentTokenStatus(c *gin.Context) {
	resp.Success(c, auth.GetAgentTokenStatus())
}

func rotateAgentToken(c *gin.Context) {
	var req rotateAgentTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		resp.Error(c, http.StatusBadRequest, "username is required")
		return
	}
	if _, err := usr.GetByUsername(username, c.Request.Context()); err != nil {
		resp.Error(c, http.StatusBadRequest, "bound user does not exist: "+username)
		return
	}

	token, err := auth.GenerateAgentToken()
	if err != nil {
		resp.InternalError(c)
		return
	}
	if err := auth.ActivateAgentToken(token, username); err != nil {
		resp.InternalError(c)
		return
	}

	// 明文仅此一次；之后只能通过重新轮换获取新令牌。
	resp.Success(c, gin.H{
		"token":    token,
		"username": username,
		"prefix":   auth.AgentTokenDisplayPrefix(token),
	})
}

func revokeAgentToken(c *gin.Context) {
	if err := auth.RevokeAgentToken(); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, auth.GetAgentTokenStatus())
}
