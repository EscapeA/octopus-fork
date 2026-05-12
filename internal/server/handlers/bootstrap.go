package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/bootstrap").
		AddRoute(
			router.NewRoute("/status", http.MethodGet).
				Handle(getBootstrapStatus),
		).
		AddRoute(
			router.NewRoute("/create-admin", http.MethodPost).
				Handle(createBootstrapAdmin),
		)
}

func getBootstrapStatus(c *gin.Context) {
	initialized, message, err := op.UserBootstrapStatus()
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, resp.ErrInternalServer)
		return
	}
	resp.Success(c, gin.H{
		"initialized": initialized,
		"message":     message,
	})
}

func createBootstrapAdmin(c *gin.Context) {
	var user model.UserBootstrapCreate
	if err := c.ShouldBindJSON(&user); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}
	if err := op.UserBootstrapCreate(user.Username, user.Password); err != nil {
		if errors.Is(err, op.ErrBootstrapAlreadySetUp) {
			resp.Error(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, op.ErrBootstrapCredentials) {
			resp.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		resp.InternalError(c)
		return
	}
	resp.Success(c, gin.H{"initialized": true})
}
