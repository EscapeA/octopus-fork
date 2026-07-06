package report

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/log"
)

var timeNow = func() int64 { return time.Now().UnixMilli() }

// In-memory caches for report schedules and history.
var (
	scheduleCache     []model.ReportSchedule
	scheduleCacheLock sync.RWMutex
	scheduleCacheOnce sync.Once

	historyCache     []model.ReportHistory
	historyCacheLock sync.RWMutex
	historyCacheOnce sync.Once
)

func invalidateScheduleCache() {
	scheduleCacheLock.Lock()
	scheduleCache = nil
	scheduleCacheLock.Unlock()
}

func invalidateHistoryCache() {
	historyCacheLock.Lock()
	historyCache = nil
	historyCacheLock.Unlock()
}

// ScheduleList returns all report schedules, using cache when available.
func ScheduleList(ctx context.Context) ([]model.ReportSchedule, error) {
	scheduleCacheLock.RLock()
	cached := scheduleCache
	scheduleCacheLock.RUnlock()

	if cached != nil {
		return cached, nil
	}

	scheduleCacheLock.Lock()
	defer scheduleCacheLock.Unlock()

	// Double-check after acquiring write lock
	if scheduleCache != nil {
		return scheduleCache, nil
	}

	var schedules []model.ReportSchedule
	if err := db.GetDB().WithContext(ctx).Find(&schedules).Error; err != nil {
		return nil, err
	}
	scheduleCache = schedules
	return schedules, nil
}

// ScheduleCreate creates a new report schedule.
func ScheduleCreate(ctx context.Context, schedule *model.ReportSchedule) error {
	if err := validateSchedule(schedule); err != nil {
		return err
	}

	schedule.ID = 0
	schedule.LastSentAt = 0

	if err := db.GetDB().WithContext(ctx).Create(schedule).Error; err != nil {
		return err
	}

	invalidateScheduleCache()
	return nil
}

// ScheduleUpdate updates an existing report schedule.
func ScheduleUpdate(ctx context.Context, schedule *model.ReportSchedule) error {
	if schedule.ID == 0 {
		return fmt.Errorf("report schedule not found")
	}

	if err := validateSchedule(schedule); err != nil {
		return err
	}

	result := db.GetDB().WithContext(ctx).Save(schedule)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("report schedule not found")
	}

	invalidateScheduleCache()
	return nil
}

// ScheduleDelete deletes a report schedule by ID.
func ScheduleDelete(ctx context.Context, id int) error {
	if id == 0 {
		return fmt.Errorf("report schedule not found")
	}

	result := db.GetDB().WithContext(ctx).Delete(&model.ReportSchedule{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("report schedule not found")
	}

	invalidateScheduleCache()
	return nil
}

// ScheduleGet retrieves a single schedule by ID.
func ScheduleGet(ctx context.Context, id int) (*model.ReportSchedule, error) {
	if id == 0 {
		return nil, fmt.Errorf("report schedule not found")
	}

	var schedule model.ReportSchedule
	if err := db.GetDB().WithContext(ctx).First(&schedule, id).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// ScheduleUpdateLastSent updates the LastSentAt timestamp for a schedule.
func ScheduleUpdateLastSent(ctx context.Context, id int, sentAt int64) error {
	if id == 0 {
		return fmt.Errorf("report schedule not found")
	}

	result := db.GetDB().WithContext(ctx).
		Model(&model.ReportSchedule{}).
		Where("id = ?", id).
		Update("last_sent_at", sentAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("report schedule not found")
	}

	invalidateScheduleCache()
	return nil
}

// HistoryList returns recent report history, using cache when available.
func HistoryList(ctx context.Context, limit int) ([]model.ReportHistory, error) {
	if limit <= 0 {
		limit = 100
	}

	historyCacheLock.RLock()
	cached := historyCache
	historyCacheLock.RUnlock()

	if cached != nil {
		if len(cached) > limit {
			return cached[:limit], nil
		}
		return cached, nil
	}

	historyCacheLock.Lock()
	defer historyCacheLock.Unlock()

	// Double-check after acquiring write lock
	if historyCache != nil {
		if len(historyCache) > limit {
			return historyCache[:limit], nil
		}
		return historyCache, nil
	}

	var history []model.ReportHistory
	if err := db.GetDB().WithContext(ctx).
		Order("sent_at desc").
		Limit(limit).
		Find(&history).Error; err != nil {
		return nil, err
	}
	historyCache = history
	return history, nil
}

// HistoryAdd creates a new report history entry.
func HistoryAdd(ctx context.Context, entry *model.ReportHistory) error {
	entry.ID = 0
	entry.SentAt = timeNow()

	if err := db.GetDB().WithContext(ctx).Create(entry).Error; err != nil {
		return err
	}

	invalidateHistoryCache()
	return nil
}

// Helper functions

func validateSchedule(schedule *model.ReportSchedule) error {
	if schedule.Name == "" {
		return fmt.Errorf("schedule name cannot be empty")
	}

	// Validate report type
	switch schedule.Type {
	case model.ReportTypeDaily, model.ReportTypeWeekly, model.ReportTypeMonthly:
		// Valid
	default:
		return fmt.Errorf("invalid report type: %s", schedule.Type)
	}

	// Validate send hour
	if schedule.SendHour < 0 || schedule.SendHour > 23 {
		return fmt.Errorf("send hour must be between 0 and 23")
	}

	// Validate day of week for weekly reports
	if schedule.Type == model.ReportTypeWeekly {
		if schedule.SendDayOfWeek < 0 || schedule.SendDayOfWeek > 6 {
			return fmt.Errorf("send day of week must be between 0 and 6")
		}
	}

	// Validate day of month for monthly reports
	if schedule.Type == model.ReportTypeMonthly {
		if schedule.SendDayOfMonth < 1 || schedule.SendDayOfMonth > 28 {
			return fmt.Errorf("send day of month must be between 1 and 28")
		}
	}

	// Validate metrics JSON
	if schedule.Metrics == "" {
		schedule.Metrics = model.DefaultReportMetrics()
	} else {
		var metrics []model.ReportMetric
		if err := json.Unmarshal([]byte(schedule.Metrics), &metrics); err != nil {
			log.Warnf("invalid metrics JSON: %v, using default", err)
			schedule.Metrics = model.DefaultReportMetrics()
		}
	}

	return nil
}

// GetEnabledSchedules returns all enabled report schedules.
func GetEnabledSchedules(ctx context.Context) ([]model.ReportSchedule, error) {
	all, err := ScheduleList(ctx)
	if err != nil {
		return nil, err
	}

	var enabled []model.ReportSchedule
	for _, s := range all {
		if s.Enabled {
			enabled = append(enabled, s)
		}
	}
	return enabled, nil
}

// ParseMetrics parses the metrics JSON string into a slice of ReportMetric.
func ParseMetrics(metricsJSON string) []model.ReportMetric {
	var metrics []model.ReportMetric
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		log.Warnf("failed to parse metrics JSON: %v", err)
		return nil
	}
	return metrics
}
