package notification

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

func TestPreferenceDeleteRemovesExistingPreference(t *testing.T) {
	setupNotificationTestDB(t)

	pref := model.NotificationPreference{
		UserID:          0,
		Type:            model.NotificationTypeAlert,
		InAppEnabled:    true,
		ExternalEnabled: true,
		MinSeverity:     model.NotificationSeverityInfo,
		ChannelIDs:      "[]",
		Enabled:         true,
	}
	if err := PreferenceSave(context.Background(), &pref); err != nil {
		t.Fatalf("PreferenceSave() error = %v", err)
	}
	if pref.ID == 0 {
		t.Fatal("PreferenceSave() did not persist an id")
	}

	if err := PreferenceDelete(context.Background(), pref.ID); err != nil {
		t.Fatalf("PreferenceDelete() error = %v", err)
	}

	items, err := PreferenceList(context.Background())
	if err != nil {
		t.Fatalf("PreferenceList() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("PreferenceList() len = %d, want 0", len(items))
	}
}

func TestPreferenceDeleteMissingReturnsNotFound(t *testing.T) {
	setupNotificationTestDB(t)

	err := PreferenceDelete(context.Background(), 404)
	if err == nil {
		t.Fatal("PreferenceDelete() error = nil, want not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("PreferenceDelete() error = %v, want not found", err)
	}
}

func TestPolicyDeleteRemovesExistingPolicy(t *testing.T) {
	setupNotificationTestDB(t)

	policy := model.NotificationPolicy{
		Name:        "critical alerts",
		Enabled:     true,
		Type:        model.NotificationTypeAlert,
		MinSeverity: model.NotificationSeverityCritical,
		ChannelIDs:  "[]",
	}
	if err := PolicyCreate(context.Background(), &policy); err != nil {
		t.Fatalf("PolicyCreate() error = %v", err)
	}
	if policy.ID == 0 {
		t.Fatal("PolicyCreate() did not persist an id")
	}

	if err := PolicyDelete(context.Background(), policy.ID); err != nil {
		t.Fatalf("PolicyDelete() error = %v", err)
	}

	items, err := PolicyList(context.Background())
	if err != nil {
		t.Fatalf("PolicyList() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("PolicyList() len = %d, want 0", len(items))
	}
}

func setupNotificationTestDB(t *testing.T) {
	t.Helper()

	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
}
