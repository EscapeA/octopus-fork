package model

// ReportType defines the frequency of a usage report schedule.
type ReportType string

const (
	ReportTypeDaily   ReportType = "daily"
	ReportTypeWeekly  ReportType = "weekly"
	ReportTypeMonthly ReportType = "monthly"
)

// ReportMetric defines which data points can be included in a usage report.
type ReportMetric string

const (
	ReportMetricOverview      ReportMetric = "overview"       // 总览：请求数、token、成本、成功率
	ReportMetricTopModels     ReportMetric = "top_models"     // Top 5 模型用量
	ReportMetricTopChannels   ReportMetric = "top_channels"   // Top 5 渠道用量
	ReportMetricTopAPIKeys    ReportMetric = "top_apikeys"    // Top 5 API Key 用量
	ReportMetricCostBreakdown ReportMetric = "cost_breakdown" // 成本明细
	ReportMetricErrorAnalysis ReportMetric = "error_analysis" // 错误分析
	ReportMetricDailyTrend    ReportMetric = "daily_trend"    // 每日趋势（仅周报/月报）
)

// AllReportMetrics returns all valid report metric values.
func AllReportMetrics() []ReportMetric {
	return []ReportMetric{
		ReportMetricOverview,
		ReportMetricTopModels,
		ReportMetricTopChannels,
		ReportMetricTopAPIKeys,
		ReportMetricCostBreakdown,
		ReportMetricErrorAnalysis,
		ReportMetricDailyTrend,
	}
}

// DefaultReportMetrics returns the default metric set for new schedules.
func DefaultReportMetrics() string {
	return `["overview","top_models","top_channels","cost_breakdown"]`
}

// ReportSchedule defines a recurring usage report configuration.
type ReportSchedule struct {
	ID             int        `json:"id" gorm:"primaryKey"`
	Name           string     `json:"name" gorm:"not null"`
	Enabled        bool       `json:"enabled" gorm:"default:true"`
	Type           ReportType `json:"type" gorm:"not null;default:'daily'"`
	Metrics        string     `json:"metrics" gorm:"not null;default:'[\"overview\",\"top_models\",\"top_channels\",\"cost_breakdown\"]'"` // JSON array of ReportMetric
	NotifChannelID int        `json:"notif_channel_id"`
	SendHour       int        `json:"send_hour" gorm:"default:8"`         // 0-23, hour of day to send (in stats timezone)
	SendDayOfWeek  int        `json:"send_day_of_week" gorm:"default:1"`  // 0=Sun..6=Sat, for weekly reports
	SendDayOfMonth int        `json:"send_day_of_month" gorm:"default:1"` // 1-28, for monthly reports
	LastSentAt     int64      `json:"last_sent_at"`                       // unix ms
}

// ReportHistory records a single sent report.
type ReportHistory struct {
	ID           int64      `json:"id" gorm:"primaryKey"`
	ScheduleID   int        `json:"schedule_id" gorm:"index"`
	ScheduleName string     `json:"schedule_name"`
	Type         ReportType `json:"type"`
	Title        string     `json:"title"`
	Content      string     `json:"content"`     // formatted report text
	SendStatus   string     `json:"send_status"` // sent / failed / skipped
	SendDetail   string     `json:"send_detail"` // channel name or error message
	SentAt       int64      `json:"sent_at" gorm:"autoCreateTime:milli"`
}
