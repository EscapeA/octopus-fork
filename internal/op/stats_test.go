package op

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
)

func TestStatsNowUsesRuntimeLocationWhenTimezoneOffsetIsZero(t *testing.T) {
	originalNow := statsTimeNow
	defer func() { statsTimeNow = originalNow }()

	statsTimeNow = func() time.Time {
		return time.Date(2026, 5, 4, 15, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	}

	settingCache.Clear()
	settingCache.Set(model.SettingKeyStatsTimezoneOffset, "0")

	now := statsNow()
	if now.Hour() != 15 {
		t.Fatalf("expected runtime local hour 15 when timezone offset is 0, got %d (%s)", now.Hour(), now.Format(time.RFC3339))
	}
}

func TestStatsNowUsesConfiguredTimezoneOffsetWhenProvided(t *testing.T) {
	originalNow := statsTimeNow
	defer func() { statsTimeNow = originalNow }()

	statsTimeNow = func() time.Time {
		return time.Date(2026, 5, 4, 7, 0, 0, 0, time.UTC)
	}

	settingCache.Clear()
	settingCache.Set(model.SettingKeyStatsTimezoneOffset, "8")

	now := statsNow()
	if now.Hour() != 15 {
		t.Fatalf("expected configured UTC+8 hour 15, got %d (%s)", now.Hour(), now.Format(time.RFC3339))
	}
}
