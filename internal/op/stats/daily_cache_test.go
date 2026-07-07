package stats

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

func TestGetDailyReflectsDailyUpdateAfterCache(t *testing.T) {
	cleanupNow := SetTimeNowForTest(func() time.Time {
		return time.Date(2026, 5, 20, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	})
	defer cleanupNow()

	setting.GetCache().Clear()
	setting.GetCache().Set(model.SettingKeyStatsTimezoneOffset, "8")
	ClearAllCachesForTest()

	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)
	if err := db.InitDB("sqlite", dsn, true); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	dbConn := db.GetDB().WithContext(ctx)
	if err := dbConn.Create(&model.StatsDaily{
		Date:         "20260519",
		StatsMetrics: model.StatsMetrics{RequestSuccess: 10},
	}).Error; err != nil {
		t.Fatalf("create historical daily row: %v", err)
	}

	if err := RefreshCache(ctx); err != nil {
		t.Fatalf("refresh cache: %v", err)
	}

	dailyList, err := GetDaily(ctx)
	if err != nil {
		t.Fatalf("first GetDaily: %v", err)
	}
	if got := len(dailyList); got != 1 {
		t.Fatalf("expected 1 cached daily row before update, got %d", got)
	}

	if err := DailyUpdate(ctx, model.StatsMetrics{RequestSuccess: 7}); err != nil {
		t.Fatalf("DailyUpdate: %v", err)
	}

	dailyList, err = GetDaily(ctx)
	if err != nil {
		t.Fatalf("second GetDaily: %v", err)
	}

	found := false
	for _, row := range dailyList {
		if row.Date == "20260520" {
			found = true
			if row.RequestSuccess != 7 {
				t.Fatalf("expected today's daily request count 7 after cache warm, got %d", row.RequestSuccess)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected second GetDaily to include today's row %q, got %#v", "20260520", dailyList)
	}
}

func TestGetDailyReturnsEmptySliceForFreshDatabase(t *testing.T) {
	cleanupNow := SetTimeNowForTest(func() time.Time {
		return time.Date(2026, 5, 20, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	})
	defer cleanupNow()

	setting.GetCache().Clear()
	setting.GetCache().Set(model.SettingKeyStatsTimezoneOffset, "8")
	ClearAllCachesForTest()

	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)
	if err := db.InitDB("sqlite", dsn, true); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := RefreshCache(ctx); err != nil {
		t.Fatalf("refresh cache: %v", err)
	}

	dailyList, err := GetDaily(ctx)
	if err != nil {
		t.Fatalf("GetDaily: %v", err)
	}
	if dailyList == nil {
		t.Fatalf("expected a non-nil empty slice so /api/v1/stats/daily serializes data as [], got nil")
	}
	if len(dailyList) != 0 {
		t.Fatalf("expected no rows for a fresh database, got %#v", dailyList)
	}
}
