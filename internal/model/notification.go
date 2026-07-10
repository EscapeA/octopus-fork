package model

// NotificationType identifies the source/category of an in-app notification.
type NotificationType string

const (
	NotificationTypeAlert         NotificationType = "alert"
	NotificationTypeReport        NotificationType = "report"
	NotificationTypeChannelExpire NotificationType = "channel_expire"
	NotificationTypeSystem        NotificationType = "system"
	NotificationTypeSite          NotificationType = "site"
	NotificationTypeBackup        NotificationType = "backup"
	NotificationTypeUsage         NotificationType = "usage"
)

// NotificationSeverity controls sorting, filtering, and external routing priority.
type NotificationSeverity string

const (
	NotificationSeverityInfo     NotificationSeverity = "info"
	NotificationSeveritySuccess  NotificationSeverity = "success"
	NotificationSeverityWarning  NotificationSeverity = "warning"
	NotificationSeverityError    NotificationSeverity = "error"
	NotificationSeverityCritical NotificationSeverity = "critical"
)

// NotificationDeliveryStatus records the outcome of one external delivery attempt.
type NotificationDeliveryStatus string

const (
	NotificationDeliveryPending NotificationDeliveryStatus = "pending"
	NotificationDeliverySent    NotificationDeliveryStatus = "sent"
	NotificationDeliveryFailed  NotificationDeliveryStatus = "failed"
	NotificationDeliverySkipped NotificationDeliveryStatus = "skipped"
)

// Notification is the unified in-app notification inbox item.
type Notification struct {
	ID       int64                `json:"id" gorm:"primaryKey"`
	Type     NotificationType     `json:"type" gorm:"size:32;index;not null"`
	Severity NotificationSeverity `json:"severity" gorm:"size:16;index;not null;default:'info'"`
	Title    string               `json:"title" gorm:"not null"`
	Content  string               `json:"content" gorm:"type:text"`
	// i18n 键化字段：前端按当前 UI 语言用 t(TitleKey, TitleArgs) 渲染。
	// 为空时回退到上面的 Title/Content 原文（历史通知零破坏）。
	// 新通知同时填充这两组：Title/Content 存英文回退串（供搜索/未来外部分发），
	// TitleKey/ContentKey + *Args 存键与参数（前端优先使用）。
	TitleKey     string `json:"title_key,omitempty" gorm:"size:128;index"`
	TitleArgs    string `json:"title_args,omitempty" gorm:"type:text"`
	ContentKey   string `json:"content_key,omitempty" gorm:"size:128"`
	ContentArgs  string `json:"content_args,omitempty" gorm:"type:text"`
	Source       string `json:"source,omitempty" gorm:"size:64;index"`
	SourceID     string `json:"source_id,omitempty" gorm:"size:128;index"`
	DedupeKey    string `json:"dedupe_key,omitempty" gorm:"size:255;index"`
	MetadataJSON string `json:"metadata_json,omitempty" gorm:"type:text"`
	Link         string `json:"link,omitempty" gorm:"size:512"`
	ReadAt       *int64 `json:"read_at,omitempty" gorm:"index"`
	ArchivedAt   *int64 `json:"archived_at,omitempty" gorm:"index"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime:milli;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime:milli"`
}

// NotificationDelivery stores one external channel delivery result for a notification.
type NotificationDelivery struct {
	ID             int64                      `json:"id" gorm:"primaryKey"`
	NotificationID int64                      `json:"notification_id" gorm:"index;not null"`
	ChannelID      int                        `json:"channel_id" gorm:"index"`
	ChannelName    string                     `json:"channel_name"`
	ChannelType    string                     `json:"channel_type" gorm:"size:32"`
	Status         NotificationDeliveryStatus `json:"status" gorm:"size:16;index;not null;default:'pending'"`
	Attempts       int                        `json:"attempts" gorm:"default:0"`
	LastError      string                     `json:"last_error,omitempty" gorm:"type:text"`
	SentAt         *int64                     `json:"sent_at,omitempty" gorm:"index"`
	CreatedAt      int64                      `json:"created_at" gorm:"autoCreateTime:milli;index"`
	UpdatedAt      int64                      `json:"updated_at" gorm:"autoUpdateTime:milli"`
}

// NotificationPreference controls in-app and external subscription behavior.
type NotificationPreference struct {
	ID              int64                `json:"id" gorm:"primaryKey"`
	UserID          int                  `json:"user_id" gorm:"index;default:0"`
	Type            NotificationType     `json:"type" gorm:"size:32;index;not null"`
	InAppEnabled    bool                 `json:"in_app_enabled" gorm:"default:true"`
	ExternalEnabled bool                 `json:"external_enabled" gorm:"default:true"`
	MinSeverity     NotificationSeverity `json:"min_severity" gorm:"size:16;default:'info'"`
	ChannelIDs      string               `json:"channel_ids,omitempty" gorm:"type:text"` // JSON array of channel IDs.
	QuietStart      string               `json:"quiet_start,omitempty" gorm:"size:8"`
	QuietEnd        string               `json:"quiet_end,omitempty" gorm:"size:8"`
	Enabled         bool                 `json:"enabled" gorm:"default:true"`
	CreatedAt       int64                `json:"created_at" gorm:"autoCreateTime:milli"`
	UpdatedAt       int64                `json:"updated_at" gorm:"autoUpdateTime:milli"`
}

// NotificationPolicy routes matching notifications to one or more external channels.
type NotificationPolicy struct {
	ID          int64                `json:"id" gorm:"primaryKey"`
	Name        string               `json:"name" gorm:"not null"`
	Enabled     bool                 `json:"enabled" gorm:"default:true;index"`
	Type        NotificationType     `json:"type" gorm:"size:32;index"`
	MinSeverity NotificationSeverity `json:"min_severity" gorm:"size:16;default:'info'"`
	Source      string               `json:"source,omitempty" gorm:"size:64;index"`
	ChannelIDs  string               `json:"channel_ids" gorm:"type:text"` // JSON array of channel IDs.
	CreatedAt   int64                `json:"created_at" gorm:"autoCreateTime:milli"`
	UpdatedAt   int64                `json:"updated_at" gorm:"autoUpdateTime:milli"`
}
