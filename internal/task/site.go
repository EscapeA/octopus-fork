package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/notification"
	"github.com/lingyuins/octopus/internal/site"
	"github.com/lingyuins/octopus/internal/sitesync"
	"github.com/lingyuins/octopus/internal/utils/log"
)

func SiteSyncTask() {
	log.Debugf("site sync task started")
	startTime := time.Now()
	defer func() {
		log.Debugf("site sync task finished, update time: %s", time.Since(startTime))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	summary := site.SyncAllWithOptions(ctx, sitesync.SiteBatchOptions{Trigger: sitesync.SiteBatchTriggerScheduled})
	createSiteBatchNotification(ctx, summary)
}

func SiteCheckinTask() {
	log.Debugf("site checkin task started")
	startTime := time.Now()
	defer func() {
		log.Debugf("site checkin task finished, update time: %s", time.Since(startTime))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	summary := site.CheckinAllWithOptions(ctx, sitesync.SiteBatchOptions{Trigger: sitesync.SiteBatchTriggerScheduled})
	createSiteBatchNotification(ctx, summary)
}

func createSiteBatchNotification(ctx context.Context, summary sitesync.SiteBatchSummary) {
	if summary.Failed == 0 && summary.Warnings == 0 && !summary.Canceled && summary.Trigger == sitesync.SiteBatchTriggerScheduled {
		return
	}
	severity := model.NotificationSeveritySuccess
	if summary.Failed > 0 || summary.Canceled {
		severity = model.NotificationSeverityError
	} else if summary.Warnings > 0 || summary.Partial > 0 || summary.Skipped > 0 {
		severity = model.NotificationSeverityWarning
	}
	title := fmt.Sprintf("Site %s completed", summary.Phase)
	content := fmt.Sprintf("%s: success=%d partial=%d failed=%d skipped=%d warnings=%d", summary.Trigger, summary.Success, summary.Partial, summary.Failed, summary.Skipped, summary.Warnings)
	metadata, _ := json.Marshal(map[string]any{
		"phase":         summary.Phase,
		"trigger":       summary.Trigger,
		"total":         summary.Total,
		"attempted":     summary.Attempted,
		"success":       summary.Success,
		"partial":       summary.Partial,
		"failed":        summary.Failed,
		"skipped":       summary.Skipped,
		"warnings":      summary.Warnings,
		"canceled":      summary.Canceled,
		"cancel_reason": summary.CancelReason,
		"samples":       summary.Samples,
	})
	if err := notification.Create(ctx, &model.Notification{
		Type:         model.NotificationTypeSite,
		Severity:     severity,
		Title:        title,
		Content:      content,
		Source:       fmt.Sprintf("site_%s", summary.Phase),
		SourceID:     string(summary.Trigger),
		DedupeKey:    fmt.Sprintf("site:%s:%s:%d", summary.Phase, summary.Trigger, time.Now().UnixMilli()),
		MetadataJSON: string(metadata),
		Link:         "hub",
	}); err != nil {
		log.Warnf("notification: failed to create site batch notification: %v", err)
	}
}
