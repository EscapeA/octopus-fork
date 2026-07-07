package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	notifop "github.com/lingyuins/octopus/internal/op/notification"
)

func setupNotificationHandlerTest(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := op.InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
}

func TestDeleteNotificationPolicyReturns400ForInvalidID(t *testing.T) {
	setupNotificationHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/notification/policy/delete/0", nil)
	c.Params = gin.Params{{Key: "id", Value: "0"}}

	deleteNotificationPolicy(c)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestDeleteNotificationPolicyReturns404WhenMissing(t *testing.T) {
	setupNotificationHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/notification/policy/delete/404", nil)
	c.Params = gin.Params{{Key: "id", Value: "404"}}

	deleteNotificationPolicy(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestDeleteNotificationPolicyDeletesExisting(t *testing.T) {
	setupNotificationHandlerTest(t)

	ctx := context.Background()
	policy := &model.NotificationPolicy{
		Name:    "to-delete",
		Type:    model.NotificationTypeAlert,
		Enabled: true,
	}
	if err := notifop.PolicyCreate(ctx, policy); err != nil {
		t.Fatalf("create policy: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	path := fmt.Sprintf("/api/v1/notification/policy/delete/%d", policy.ID)
	c.Request = httptest.NewRequest(http.MethodDelete, path, nil)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", policy.ID)}}

	deleteNotificationPolicy(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Data any `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	items, err := notifop.PolicyList(ctx)
	if err != nil {
		t.Fatalf("policy list: %v", err)
	}
	for _, p := range items {
		if p.ID == policy.ID {
			t.Fatalf("policy %d still exists after delete", policy.ID)
		}
	}
}

func TestDeleteNotificationPreferenceReturns404WhenMissing(t *testing.T) {
	setupNotificationHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/notification/preference/delete/404", nil)
	c.Params = gin.Params{{Key: "id", Value: "404"}}

	deleteNotificationPreference(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestDeleteNotificationPreferenceDeletesExisting(t *testing.T) {
	setupNotificationHandlerTest(t)

	ctx := context.Background()
	pref := &model.NotificationPreference{
		Type:            model.NotificationTypeAlert,
		InAppEnabled:    true,
		ExternalEnabled: true,
		MinSeverity:     model.NotificationSeverityInfo,
		Enabled:         true,
	}
	if err := notifop.PreferenceSave(ctx, pref); err != nil {
		t.Fatalf("save preference: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	path := fmt.Sprintf("/api/v1/notification/preference/delete/%d", pref.ID)
	c.Request = httptest.NewRequest(http.MethodDelete, path, nil)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", pref.ID)}}

	deleteNotificationPreference(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	items, err := notifop.PreferenceList(ctx)
	if err != nil {
		t.Fatalf("preference list: %v", err)
	}
	for _, p := range items {
		if p.ID == pref.ID {
			t.Fatalf("preference %d still exists after delete", pref.ID)
		}
	}
}
