package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/model"
	notifop "github.com/lingyuins/octopus/internal/op/notification"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/lingyuins/octopus/internal/update"
	"github.com/lingyuins/octopus/internal/utils/log"
)

func init() {
	router.NewGroupRouter("/api/v1/update").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSettingsRead)).
		AddRoute(
			router.NewRoute("", http.MethodGet).
				Handle(latest),
		).
		AddRoute(
			router.NewRoute("/now-version", http.MethodGet).
				Handle(getNowVersion),
		).
		AddRoute(
			router.NewRoute("", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSettingsWrite)).
				Handle(updateFunc),
		)
}

func latest(c *gin.Context) {
	latestInfo, err := update.GetLatestInfo()
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, *latestInfo)
}

func getNowVersion(c *gin.Context) {
	resp.Success(c, conf.Version)
}

func updateFunc(c *gin.Context) {
	createUpdateNotification(c, "info", "Self update started", "Octopus self-update has started.", nil)
	err := update.UpdateCore()
	if err != nil {
		createUpdateNotification(c, "error", "Self update failed", err.Error(), err)
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	createUpdateNotification(c, "success", "Self update completed", "Octopus self-update completed successfully.", nil)
	resp.Success(c, "update success")
}

func createUpdateNotification(c *gin.Context, severity string, title string, content string, err error) {
	sev := model.NotificationSeverityInfo
	switch severity {
	case "success":
		sev = model.NotificationSeveritySuccess
	case "error":
		sev = model.NotificationSeverityError
	}
	metadata := map[string]any{"version": conf.Version}
	if err != nil {
		metadata["error"] = err.Error()
	}
	b, _ := json.Marshal(metadata)
	if createErr := notifop.Create(c.Request.Context(), &model.Notification{
		Type:         model.NotificationTypeSystem,
		Severity:     sev,
		Title:        title,
		Content:      content,
		Source:       "self_update",
		SourceID:     conf.Version,
		DedupeKey:    fmt.Sprintf("self_update:%s:%d", severity, time.Now().UnixMilli()),
		MetadataJSON: string(b),
		Link:         "setting",
	}); createErr != nil {
		log.Warnf("notification: failed to create update notification: %v", createErr)
	}
}
