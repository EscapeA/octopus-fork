package relay

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/helper"
	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/alert"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/op/notification"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/relay/balancer"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// 上游 402（Payment Required：余额不足/欠费）自动禁用 Key。
//
// 背景：402 不是 429 限流式的短时抖动，而是账号级的长期状态（需充值才能恢复）。
// 修复前 402 落进 ClassifyRelayError 的 default 分支，只吃 60s 的通用 Key 冷却，
// 结果是每个冷却周期都被重新选中 → 白打一次 402、浪费一次 Key 级重试额度、
// 首字节多等数秒（实测每个 60s 周期一次）。
//
// 现在的行为（默认开启，阈值默认 3）：
//  1. 每次上游返回 402，按 (channelID, keyID) 累加连续计数（余额是账号级，不带模型维度）；
//  2. 计数达到阈值 → 把该 Key 的 enabled 置 false 并落库（Enabled=false 是既有路由
//     过滤条件，因此改完立即生效），同时写入 auto_disabled_at / auto_disable_reason；
//  3. 清理该 Key 的冷却/熔断/可用度/失败提示残留，避免 Key 恢复后被旧状态继续排斥；
//  4. 站内通知 + 渠道配置的外部通知渠道投递；
//  5. Key 上出现一次成功请求时计数清零。
//
// 恢复途径（两条都可用）：
//   - 手动：渠道编辑页把该 Key 重新启用（清空 auto_disabled_at/auto_disable_reason）；
//   - 自动：定时试活任务（internal/task/key_auto_disable_probe.go）按设置的间隔
//     用真实最小请求探测，成功即自动启用。
//
// 设计取舍：余额不足就无法服务请求，禁用只是把「上游往返一次 402」提前为「本地跳过」，
// 不会让本来可用的 Key 变不可用；因此不禁用「最后一个可用 Key」之外的任何保护都是多余的。

const (
	keyAutoDisableDefaultThreshold = 3
	// keyAutoDisableReasonPrefix 是写入 auto_disable_reason 的前缀，用于识别系统自动禁用。
	keyAutoDisableReasonPrefix = "auto disabled: upstream 402"
)

// KeyAutoDisableEnabled 上游 402 达阈值时是否自动禁用 Key。
func KeyAutoDisableEnabled() bool {
	v, err := setting.GetBool(dbmodel.SettingKeyKeyAutoDisableEnabled)
	if err != nil {
		return true
	}
	return v
}

// KeyAutoDisableThreshold 连续多少次 402 后自动禁用该 Key。
func KeyAutoDisableThreshold() int {
	v, err := setting.GetInt(dbmodel.SettingKeyKeyAutoDisableThreshold)
	if err != nil || v < 1 {
		return keyAutoDisableDefaultThreshold
	}
	return v
}

// KeyAutoDisableProbeInterval 自动禁用 Key 的试活间隔；0 表示不自动试活（仅手动恢复）。
func KeyAutoDisableProbeInterval() time.Duration {
	v, err := setting.GetInt(dbmodel.SettingKeyKeyAutoDisableProbeInterval)
	if err != nil || v <= 0 {
		return 0
	}
	return time.Duration(v) * time.Minute
}

// handleUpstreamPaymentRequired 处理一次上游 402（relay 失败分支入口）。
// 同一请求内的多次重试只计一次（阈值语义 = 连续 N 次请求都收到 402）。
func handleUpstreamPaymentRequired(ra *relayAttempt, modelName string) {
	if ra == nil || ra.channel == nil {
		return
	}
	if !KeyAutoDisableEnabled() {
		return
	}
	if ra.markInsufficientBalanceCounted(ra.usedKey.ID) {
		return
	}
	if handleChannelKeyInsufficientBalance(ra.channel, ra.usedKey, modelName) {
		// 本请求后续的尝试（含 adapter 降级）不能再把禁用前的旧副本写回。
		ra.markKeyAutoDisabled(ra.usedKey.ID)
	}
}

// handleUpstreamPaymentRequiredMedia 媒体链路（TTS/图片等）的 402 处理入口。
// counted 由调用方在请求作用域内持有（媒体链路没有 relayRequest 上下文），
// 保证同一请求只计一次。返回 true 表示本次刚禁用了该 Key（调用方应跳过陈旧写回）。
func handleUpstreamPaymentRequiredMedia(ch *dbmodel.Channel, key dbmodel.ChannelKey, modelName string, counted map[int]struct{}) bool {
	if ch == nil || key.ID == 0 || !KeyAutoDisableEnabled() {
		return false
	}
	if counted != nil {
		if _, ok := counted[key.ID]; ok {
			return false
		}
		counted[key.ID] = struct{}{}
	}
	return handleChannelKeyInsufficientBalance(ch, key, modelName)
}

// handleChannelKeyInsufficientBalance 累计某 (channel, key) 的连续 402 计数，
// 达到阈值时禁用该 Key 并通知（返回 true）；未达阈值时仅累计（返回 false）。
func handleChannelKeyInsufficientBalance(ch *dbmodel.Channel, key dbmodel.ChannelKey, modelName string) bool {
	if ch == nil || key.ID == 0 {
		return false
	}
	if !KeyAutoDisableEnabled() {
		return false
	}
	threshold := KeyAutoDisableThreshold()
	count := balancer.RecordAutoDisableFailure(ch.ID, key.ID)
	if count < threshold {
		log.Infof("key auto disable: channel %d(%s) key %d got 402 (%d/%d before disable), model=%s",
			ch.ID, ch.Name, key.ID, count, threshold, strings.TrimSpace(modelName))
		return false
	}
	DisableChannelKeyForInsufficientBalance(ch, key, count, modelName)
	return true
}

// DisableChannelKeyForInsufficientBalance 自动禁用 Key：落库 enabled=false + 标记，
// 清理运行时残留状态并发送通知。供 relay 失败分支与测试调用。
func DisableChannelKeyForInsufficientBalance(ch *dbmodel.Channel, key dbmodel.ChannelKey, count int, modelName string) {
	if ch == nil || key.ID == 0 {
		return
	}
	now := time.Now()
	reason := fmt.Sprintf("%s (consecutive %d, model %s)", keyAutoDisableReasonPrefix, count, strings.TrimSpace(modelName))
	if len(reason) > 255 {
		reason = reason[:255]
	}

	// 1) 运行时禁用：先改缓存，下一次 Key 选择立即生效（Enabled=false 是既有过滤条件）。
	key.Enabled = false
	key.AutoDisabledAt = &now
	key.AutoDisableReason = reason
	if err := channel.KeyUpdate(key); err != nil {
		log.Warnf("key auto disable: failed to update channel key cache (channel %d key %d): %v", ch.ID, key.ID, err)
	}

	// 2) 立即落库（不等 KeySaveDB 周期刷盘，避免重启后自动禁用状态丢失）。
	if conn := db.GetDB(); conn != nil {
		if err := conn.Model(&dbmodel.ChannelKey{}).Where("id = ?", key.ID).Updates(map[string]any{
			"enabled":             false,
			"auto_disabled_at":    now,
			"auto_disable_reason": reason,
		}).Error; err != nil {
			log.Warnf("key auto disable: failed to persist channel key (channel %d key %d): %v", ch.ID, key.ID, err)
		}
	}

	// 3) 清理该 Key 的运行时隔离残留（冷却/熔断/可用度/402 计数/失败提示），
	//    否则 Key 恢复后旧状态仍会排斥它。
	balancer.RemoveKeyRuntimeState(key.ID)
	removeFailureHintsByKey(key.ID)

	log.Warnf("key auto disable: channel %d(%s) key %d auto disabled after %d consecutive 402 (model=%s)",
		ch.ID, ch.Name, key.ID, count, strings.TrimSpace(modelName))

	notifyKeyAutoDisabled(ch, key, count, modelName)
}

// ReEnableAutoDisabledChannelKey 自动/手动恢复后回来：清除自动禁用标记并重新启用该 Key。
// 返回是否确实发生了状态变更。内部按 keyID 从缓存取**最新**的 Key 快照再判断，
// 避免调用方拿着过期副本（例如后台任务遍历到的是启用前的旧值）误判。
// 供定时试活任务调用（手动启用在渠道编辑接口里清标记）。
func ReEnableAutoDisabledChannelKey(ch *dbmodel.Channel, keyID int) bool {
	if ch == nil || keyID == 0 {
		return false
	}
	current, ok := lookupCachedChannelKey(ch.ID, keyID)
	if !ok {
		return false
	}
	if current.Enabled && current.AutoDisabledAt == nil && current.AutoDisableReason == "" {
		return false
	}
	current.Enabled = true
	current.AutoDisabledAt = nil
	current.AutoDisableReason = ""
	if err := channel.KeyUpdate(current); err != nil {
		log.Warnf("key auto enable: failed to update channel key cache (channel %d key %d): %v", ch.ID, keyID, err)
	}
	if conn := db.GetDB(); conn != nil {
		if err := conn.Model(&dbmodel.ChannelKey{}).Where("id = ?", keyID).Updates(map[string]any{
			"enabled":             true,
			"auto_disabled_at":    nil,
			"auto_disable_reason": "",
		}).Error; err != nil {
			log.Warnf("key auto enable: failed to persist channel key (channel %d key %d): %v", ch.ID, keyID, err)
		}
	}
	balancer.RemoveKeyRuntimeState(keyID)
	removeFailureHintsByKey(keyID)
	log.Infof("key auto enable: channel %d(%s) key %d re-enabled", ch.ID, ch.Name, keyID)
	notifyKeyAutoEnabled(ch, current)
	return true
}

// lookupCachedChannelKey 从渠道缓存取指定 Key 的最新快照。
func lookupCachedChannelKey(channelID, keyID int) (dbmodel.ChannelKey, bool) {
	channel, err := channel.Get(channelID, context.Background())
	if err != nil || channel == nil {
		return dbmodel.ChannelKey{}, false
	}
	for _, key := range channel.Keys {
		if key.ID == keyID {
			return key, true
		}
	}
	return dbmodel.ChannelKey{}, false
}

// keyAutoDisableLabel 生成 Key 的展示标签：优先备注，其次脱敏 Key（前4后4）。
func keyAutoDisableLabel(key dbmodel.ChannelKey) string {
	if remark := strings.TrimSpace(key.Remark); remark != "" {
		return remark
	}
	raw := strings.TrimSpace(key.ChannelKey)
	if raw == "" {
		return fmt.Sprintf("#%d", key.ID)
	}
	if len(raw) <= 8 {
		return raw
	}
	return raw[:4] + "..." + raw[len(raw)-4:]
}

// notifyKeyAutoDisabled 站内通知 + 渠道配置的外部通知渠道投递。
func notifyKeyAutoDisabled(ch *dbmodel.Channel, key dbmodel.ChannelKey, count int, modelName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	label := keyAutoDisableLabel(key)
	modelName = strings.TrimSpace(modelName)
	metadata, _ := jsonAPI.Marshal(map[string]any{
		"channel_id":   ch.ID,
		"channel_name": ch.Name,
		"key_id":       key.ID,
		"key_label":    label,
		"count":        count,
		"model":        modelName,
	})
	n := &dbmodel.Notification{
		Type:         dbmodel.NotificationTypeKeyHealth,
		Severity:     dbmodel.NotificationSeverityWarning,
		Source:       "key_auto_disable",
		SourceID:     fmt.Sprintf("%d:%d", ch.ID, key.ID),
		DedupeKey:    fmt.Sprintf("key_auto_disable:%d:%d:%d", ch.ID, key.ID, time.Now().UnixMilli()),
		MetadataJSON: string(metadata),
		Link:         "channel",
	}
	titleArgs := map[string]any{"name": ch.Name, "key": label}
	contentArgs := map[string]any{"name": ch.Name, "id": ch.ID, "key": label, "fails": count, "model": modelName}
	notification.SetMessage(n, notification.KeyKeyAutoDisabled, notification.KeyKeyAutoDisabled,
		titleArgs, contentArgs,
		[]any{ch.Name, label}, []any{ch.Name, ch.ID, label, count, modelName})
	if err := safeCreateKeyNotification(ctx, n); err != nil {
		log.Warnf("key auto disable: failed to create notification for channel %d key %d: %v", ch.ID, key.ID, err)
	}
	if ch.NotifChannelID != nil {
		sendKeyAutoDisableExternalNotification(ctx, *ch.NotifChannelID,
			fmt.Sprintf("Channel \"%s\" key auto disabled", ch.Name),
			fmt.Sprintf("Channel \"%s\" (ID: %d) key %s auto disabled after %d consecutive HTTP 402 (payment required): model=%s",
				ch.Name, ch.ID, label, count, modelName))
	}
}

// notifyKeyAutoEnabled 恢复通知（仅站内 + 外部渠道）。
func notifyKeyAutoEnabled(ch *dbmodel.Channel, key dbmodel.ChannelKey) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	label := keyAutoDisableLabel(key)
	metadata, _ := jsonAPI.Marshal(map[string]any{
		"channel_id":   ch.ID,
		"channel_name": ch.Name,
		"key_id":       key.ID,
		"key_label":    label,
	})
	n := &dbmodel.Notification{
		Type:         dbmodel.NotificationTypeKeyHealth,
		Severity:     dbmodel.NotificationSeveritySuccess,
		Source:       "key_auto_disable",
		SourceID:     fmt.Sprintf("%d:%d", ch.ID, key.ID),
		DedupeKey:    fmt.Sprintf("key_auto_enable:%d:%d:%d", ch.ID, key.ID, time.Now().UnixMilli()),
		MetadataJSON: string(metadata),
		Link:         "channel",
	}
	notification.SetMessage(n, notification.KeyKeyAutoEnabled, notification.KeyKeyAutoEnabled,
		map[string]any{"name": ch.Name, "key": label},
		map[string]any{"name": ch.Name, "id": ch.ID, "key": label},
		[]any{ch.Name, label}, []any{ch.Name, ch.ID, label})
	if err := safeCreateKeyNotification(ctx, n); err != nil {
		log.Warnf("key auto enable: failed to create notification for channel %d key %d: %v", ch.ID, key.ID, err)
	}
	if ch.NotifChannelID != nil {
		sendKeyAutoDisableExternalNotification(ctx, *ch.NotifChannelID,
			fmt.Sprintf("Channel \"%s\" key auto enabled", ch.Name),
			fmt.Sprintf("Channel \"%s\" (ID: %d) key %s auto enabled: probe succeeded.", ch.Name, ch.ID, label))
	}
}

// safeCreateKeyNotification 包装 notification.Create：DB 未初始化（单测/启动早期）时
// notification.Create 会对 nil *gorm.DB 调用方法而 panic，这里 recover 兜底，
// 保证自动禁用/恢复这类保护逻辑永不因通知失败而中断（与 key_health 的 safeCreateNotification 同思路）。
func safeCreateKeyNotification(ctx context.Context, n *dbmodel.Notification) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("notification create panic: %v", r)
		}
	}()
	return notification.Create(ctx, n)
}

// sendKeyAutoDisableExternalNotification 投递到渠道配置的外部通知渠道（webhook/email/tg 等）。
func sendKeyAutoDisableExternalNotification(ctx context.Context, notifChannelID int, title, message string) {
	if db.GetDB() == nil {
		return
	}
	channels, err := alert.NotifChannelList(ctx)
	if err != nil {
		log.Warnf("key auto disable: failed to list notif channels: %v", err)
		return
	}
	for i := range channels {
		if channels[i].ID != notifChannelID {
			continue
		}
		if err := helper.SendNotificationMessage(&channels[i], title, message); err != nil {
			log.Warnf("key auto disable: failed to send external notification (notif channel %d): %v", notifChannelID, err)
		}
		return
	}
}
