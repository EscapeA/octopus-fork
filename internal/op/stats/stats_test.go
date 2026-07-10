package stats_test

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/op/stats"
)

func TestNowUsesRuntimeLocationWhenTimezoneOffsetIsZero(t *testing.T) {
	cleanup := stats.SetTimeNowForTest(func() time.Time {
		// 用 time.Local 构造，使测试与运行时 TZ 无关：offset=0 时 Now() 走 time.Local，
		// timeNow().In(time.Local) 应保持原始 wall-clock 小时。
		return time.Date(2026, 5, 4, 15, 0, 0, 0, time.Local)
	})
	defer cleanup()

	setting.GetCache().Clear()
	setting.GetCache().Set(model.SettingKeyStatsTimezoneOffset, "0")

	now := stats.Now()
	if now.Hour() != 15 {
		t.Fatalf("expected runtime local hour 15 when timezone offset is 0, got %d (%s)", now.Hour(), now.Format(time.RFC3339))
	}
}

func TestNowUsesConfiguredTimezoneOffsetWhenProvided(t *testing.T) {
	cleanup := stats.SetTimeNowForTest(func() time.Time {
		return time.Date(2026, 5, 4, 7, 0, 0, 0, time.UTC)
	})
	defer cleanup()

	setting.GetCache().Clear()
	setting.GetCache().Set(model.SettingKeyStatsTimezoneOffset, "8")

	now := stats.Now()
	if now.Hour() != 15 {
		t.Fatalf("expected configured UTC+8 hour 15, got %d (%s)", now.Hour(), now.Format(time.RFC3339))
	}
}

func TestStatsLocation(t *testing.T) {
	// 保存并恢复 cache 原始状态，避免影响其它测试。
	cache := setting.GetCache()
	cache.Clear()
	t.Cleanup(func() { cache.Clear() })

	t.Run("IANA preferred over offset", func(t *testing.T) {
		cache.Clear()
		cache.Set(model.SettingKeyStatsTimezone, "Asia/Shanghai")
		cache.Set(model.SettingKeyStatsTimezoneOffset, "5") // 应被忽略

		loc := stats.StatsLocation()
		want, _ := time.LoadLocation("Asia/Shanghai")
		if loc.String() != want.String() {
			t.Fatalf("expected Asia/Shanghai, got %s", loc.String())
		}
	})

	t.Run("falls back to offset when IANA empty", func(t *testing.T) {
		cache.Clear()
		cache.Set(model.SettingKeyStatsTimezone, "")
		cache.Set(model.SettingKeyStatsTimezoneOffset, "8")

		loc := stats.StatsLocation()
		// UTC+8 fixed zone: offset 8*3600 = 28800 秒
		_, offset := time.Now().In(loc).Zone()
		if offset != 8*3600 {
			t.Fatalf("expected UTC+8 offset (28800s), got %d (%s)", offset, loc.String())
		}
	})

	t.Run("falls back to Local when both unset", func(t *testing.T) {
		cache.Clear()
		// 未设置任何时区 -> time.Local（容器运行时区），保持历史行为
		loc := stats.StatsLocation()
		if loc != time.Local {
			t.Fatalf("expected time.Local when nothing configured, got %s", loc.String())
		}
	})

	t.Run("invalid IANA falls back to offset", func(t *testing.T) {
		cache.Clear()
		cache.Set(model.SettingKeyStatsTimezone, "Not/A/Real/Zone")
		cache.Set(model.SettingKeyStatsTimezoneOffset, "8")

		loc := stats.StatsLocation()
		_, offset := time.Now().In(loc).Zone()
		if offset != 8*3600 {
			t.Fatalf("expected UTC+8 fallback after invalid IANA, got %d (%s)", offset, loc.String())
		}
	})

	t.Run("invalid IANA with no offset falls back to Local", func(t *testing.T) {
		cache.Clear()
		cache.Set(model.SettingKeyStatsTimezone, "Not/A/Real/Zone")
		cache.Set(model.SettingKeyStatsTimezoneOffset, "0")

		loc := stats.StatsLocation()
		if loc != time.Local {
			t.Fatalf("expected time.Local after invalid IANA + zero offset, got %s", loc.String())
		}
	})
}
