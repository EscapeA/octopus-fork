package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/remotesite"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/remote-site-token").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSitesRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/list/:site_id", http.MethodGet).
				Handle(listRemoteTokens),
		).
		AddRoute(
			router.NewRoute("/sync/:site_id", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(syncTokens),
		).
		AddRoute(
			router.NewRoute("/sync-to-channel", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSitesWrite)).
				Handle(syncToChannel),
		)
}

func listRemoteTokens(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	tokens, err := remotesite.ListTokens(c.Request.Context(), siteID)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, tokens)
}

func syncTokens(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("site_id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidParam)
		return
	}
	count, err := remotesite.SyncTokens(c.Request.Context(), siteID)
	if err != nil {
		resp.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	resp.Success(c, gin.H{"synced": count})
}

func syncToChannel(c *gin.Context) {
	var req model.SyncToChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	ch, err := remotesite.SyncToChannel(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp.Success(c, ch)
}
