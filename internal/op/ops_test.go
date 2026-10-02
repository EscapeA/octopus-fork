package op

import (
	"context"
	"reflect"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

func TestNormalizeNavOrder_AppendsMissingRoutesAndDropsUnknown(t *testing.T) {
	defaults := []string{"home", "channel", "group", "model", "analytics", "log", "notification", "ops", "apikey", "setting", "user"}
	got := NormalizeNavOrder(`["group","group","unknown","setting"]`, defaults)
	want := []string{"group", "setting", "home", "channel", "model", "analytics", "log", "notification", "ops", "apikey", "user"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeNavOrder() = %v, want %v", got, want)
	}
}

func TestTelemetrySummaryGet_ReturnsValidData(t *testing.T) {
	ctx := context.Background()
	summary, err := TelemetrySummaryGet(ctx)
	if err != nil {
		t.Fatalf("TelemetrySummaryGet returned error: %v", err)
	}
	if summary == nil {
		t.Fatal("TelemetrySummaryGet returned nil summary")
	}
	if summary.Hero.UptimeSeconds < 0 {
		t.Error("uptime_seconds should be >= 0")
	}
	if summary.Hero.ActiveConnections < 0 {
		t.Error("active_connections should be >= 0")
	}
	if len(summary.DrilldownShortcuts) != 5 {
		t.Errorf("expected 5 drilldown shortcuts, got %d", len(summary.DrilldownShortcuts))
	}
}

func TestTelemetrySummaryGet_DrilldownKeys(t *testing.T) {
	ctx := context.Background()
	summary, err := TelemetrySummaryGet(ctx)
	if err != nil {
		t.Fatalf("TelemetrySummaryGet returned error: %v", err)
	}
	expectedKeys := []string{"cache", "quota", "health", "system", "audit"}
	for i, sc := range summary.DrilldownShortcuts {
		if sc.Key != expectedKeys[i] {
			t.Errorf("shortcut[%d]: expected key %q, got %q", i, expectedKeys[i], sc.Key)
		}
	}
}

func TestTelemetrySummaryGet_DatabaseHealthDefaults(t *testing.T) {
	ctx := context.Background()
	summary, err := TelemetrySummaryGet(ctx)
	if err != nil {
		t.Fatalf("TelemetrySummaryGet returned error: %v", err)
	}
	if summary.DatabaseHealth.Repairs != 0 {
		t.Error("database_health.repairs should be 0 in phase 1")
	}
	validStatuses := map[string]bool{"healthy": true, "degraded": true}
	if !validStatuses[summary.DatabaseHealth.Status] {
		t.Errorf("unexpected database_health.status: %q", summary.DatabaseHealth.Status)
	}
}

func TestTelemetrySummaryGet_ProviderHealthDefaults(t *testing.T) {
	ctx := context.Background()
	summary, err := TelemetrySummaryGet(ctx)
	if err != nil {
		t.Fatalf("TelemetrySummaryGet returned error: %v", err)
	}
	if summary.ProviderHealth.Monitored != len(summary.ProviderHealth.Providers) {
		t.Errorf("monitored=%d but providers len=%d", summary.ProviderHealth.Monitored, len(summary.ProviderHealth.Providers))
	}
	if summary.ProviderHealth.Active > summary.ProviderHealth.Monitored {
		t.Error("active should be <= monitored")
	}
}

func snapshotSettingCache() func() {
	snapshot := settingCache.GetAll()
	return func() {
		settingCache.Clear()
		for key, value := range snapshot {
			settingCache.Set(key, value)
		}
	}
}

func seedDefaultSettingsForTest(overrides map[model.SettingKey]string) {
	settingCache.Clear()
	for _, setting := range model.DefaultSettings() {
		value := setting.Value
		if override, ok := overrides[setting.Key]; ok {
			value = override
		}
		settingCache.Set(setting.Key, value)
	}
	for key, value := range overrides {
		if _, exists := settingCache.Get(key); exists {
			continue
		}
		settingCache.Set(key, value)
	}
}
