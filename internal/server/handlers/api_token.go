package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/apitoken"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

// 机器令牌管理：让脚本 / agent / CI 无需管理员账号密码即可调用管理面 API。
// 明文只在 create 响应里返回一次；列表中只展示前缀与元信息。
func init() {
	apiTokenRoutes := router.NewGroupRouter("/api/v1/api-token").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSettingsRead))

	apiTokenRoutes.AddRoute(
		router.NewRoute("/list", http.MethodGet).
			Handle(listAPITokens),
	)

	apiTokenRoutes.AddRoute(
		router.NewRoute("/create", http.MethodPost).
			Use(middleware.RequirePermission(auth.PermSettingsWrite)).
			Use(middleware.RequireJSON()).
			Handle(createAPIToken),
	)

	apiTokenRoutes.AddRoute(
		router.NewRoute("/revoke", http.MethodPost).
			Use(middleware.RequirePermission(auth.PermSettingsWrite)).
			Use(middleware.RequireJSON()).
			Handle(revokeAPIToken),
	)

	apiTokenRoutes.AddRoute(
		router.NewRoute("/delete/:id", http.MethodDelete).
			Use(middleware.RequirePermission(auth.PermSettingsWrite)).
			Handle(deleteAPIToken),
	)
}

func listAPITokens(c *gin.Context) {
	tokens, err := apitoken.List(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, tokens)
}

func createAPIToken(c *gin.Context) {
	var req model.APITokenCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	token, row, err := apitoken.Create(c.Request.Context(), req, uint(c.GetInt("user_id")))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// 明文仅此一次；之后只能重建。
	resp.Success(c, model.APITokenCreateResponse{
		Token:     token,
		ID:        row.ID,
		Name:      row.Name,
		Prefix:    row.Prefix,
		UserID:    row.UserID,
		Username:  row.Username,
		ExpiresAt: row.ExpiresAt,
	})
}

func revokeAPIToken(c *gin.Context) {
	var req model.APITokenRevokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if req.ID == 0 {
		resp.Error(c, http.StatusBadRequest, "id is required")
		return
	}
	if err := apitoken.Revoke(c.Request.Context(), req.ID); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func deleteAPIToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	if err := apitoken.Delete(c.Request.Context(), uint(id)); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}
