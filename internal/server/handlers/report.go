package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/alert"
	"github.com/lingyuins/octopus/internal/op/report"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/lingyuins/octopus/internal/task"
)

func init() {
	router.NewGroupRouter("/api/v1/report").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermSettingsRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/schedule/list", http.MethodGet).
				Handle(listReportSchedules),
		).
		AddRoute(
			router.NewRoute("/schedule/create", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSettingsWrite)).
				Handle(createReportSchedule),
		).
		AddRoute(
			router.NewRoute("/schedule/update", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSettingsWrite)).
				Handle(updateReportSchedule),
		).
		AddRoute(
			router.NewRoute("/schedule/delete/:id", http.MethodDelete).
				Use(middleware.RequirePermission(auth.PermSettingsWrite)).
				Handle(deleteReportSchedule),
		).
		AddRoute(
			router.NewRoute("/schedule/test", http.MethodPost).
				Use(middleware.RequirePermission(auth.PermSettingsWrite)).
				Handle(testReportSchedule),
		).
		AddRoute(
			router.NewRoute("/history/list", http.MethodGet).
				Handle(listReportHistory),
		).
		AddRoute(
			router.NewRoute("/metrics", http.MethodGet).
				Handle(listReportMetrics),
		)
}

func listReportSchedules(c *gin.Context) {
	schedules, err := report.ScheduleList(c.Request.Context())
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, schedules)
}

func createReportSchedule(c *gin.Context) {
	var schedule model.ReportSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate notification channel exists
	if schedule.NotifChannelID > 0 {
		channels, err := alert.NotifChannelList(c.Request.Context())
		if err != nil {
			resp.InternalError(c)
			return
		}
		found := false
		for _, ch := range channels {
			if ch.ID == schedule.NotifChannelID {
				found = true
				break
			}
		}
		if !found {
			resp.Error(c, http.StatusBadRequest, "notification channel not found")
			return
		}
	}

	if err := report.ScheduleCreate(c.Request.Context(), &schedule); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, schedule)
}

func updateReportSchedule(c *gin.Context) {
	var schedule model.ReportSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if schedule.ID == 0 {
		resp.Error(c, http.StatusBadRequest, "schedule ID is required")
		return
	}

	// Validate notification channel exists
	if schedule.NotifChannelID > 0 {
		channels, err := alert.NotifChannelList(c.Request.Context())
		if err != nil {
			resp.InternalError(c)
			return
		}
		found := false
		for _, ch := range channels {
			if ch.ID == schedule.NotifChannelID {
				found = true
				break
			}
		}
		if !found {
			resp.Error(c, http.StatusBadRequest, "notification channel not found")
			return
		}
	}

	if err := report.ScheduleUpdate(c.Request.Context(), &schedule); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, schedule)
}

func deleteReportSchedule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid schedule ID")
		return
	}

	if err := report.ScheduleDelete(c.Request.Context(), id); err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, nil)
}

func testReportSchedule(c *gin.Context) {
	var req struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == 0 {
		resp.Error(c, http.StatusBadRequest, "schedule ID is required")
		return
	}

	schedule, err := report.ScheduleGet(c.Request.Context(), req.ID)
	if err != nil {
		resp.Error(c, http.StatusNotFound, "report schedule not found")
		return
	}

	if err := task.TestSendReport(c.Request.Context(), *schedule); err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, nil)
}

func listReportHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	history, err := report.HistoryList(c.Request.Context(), limit)
	if err != nil {
		resp.InternalError(c)
		return
	}
	resp.Success(c, history)
}

func listReportMetrics(c *gin.Context) {
	metrics := model.AllReportMetrics()
	resp.Success(c, metrics)
}
