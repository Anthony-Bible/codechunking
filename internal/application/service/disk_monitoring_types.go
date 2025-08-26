package service

import (
	"time"
)

// DiskUsageInfo contains current disk usage information.
type DiskUsageInfo struct {
	Path            string    `json:"path"`
	TotalSpaceBytes int64     `json:"total_space_bytes"`
	UsedSpaceBytes  int64     `json:"used_space_bytes"`
	AvailableBytes  int64     `json:"available_bytes"`
	UsagePercentage float64   `json:"usage_percentage"`
	CacheUsageBytes int64     `json:"cache_usage_bytes"`
	RepositoryCount int       `json:"repository_count"`
	LastUpdated     time.Time `json:"last_updated"`
	IOPSCurrent     int       `json:"iops_current"`
	ReadLatencyMs   float64   `json:"read_latency_ms"`
	WriteLatencyMs  float64   `json:"write_latency_ms"`
}

// DiskUsageUpdate represents a disk usage update event.
type DiskUsageUpdate struct {
	Path      string         `json:"path"`
	Usage     *DiskUsageInfo `json:"usage"`
	Change    int64          `json:"change"`
	Alert     *DiskAlert     `json:"alert,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// DiskThreshold defines disk usage thresholds.
type DiskThreshold struct {
	WarningPercentage  float64 `json:"warning_percentage"`
	CriticalPercentage float64 `json:"critical_percentage"`
	MaxUsageBytes      int64   `json:"max_usage_bytes"`
	MinFreeBytes       int64   `json:"min_free_bytes"`
}

// DiskAlert represents a disk usage alert.
type DiskAlert struct {
	ID           string          `json:"id"`
	Path         string          `json:"path"`
	Level        DiskAlertLevel  `json:"level"`
	Message      string          `json:"message"`
	CurrentUsage *DiskUsageInfo  `json:"current_usage"`
	Threshold    DiskThreshold   `json:"threshold"`
	CreatedAt    time.Time       `json:"created_at"`
	Actions      []string        `json:"actions"`
	Status       DiskAlertStatus `json:"status"`
}

// DiskAlertLevel represents alert severity levels.
type DiskAlertLevel int

const (
	DiskAlertWarning DiskAlertLevel = iota
	DiskAlertCritical
	DiskAlertEmergency
)

// DiskAlertStatus represents alert status.
type DiskAlertStatus int

const (
	DiskAlertActive DiskAlertStatus = iota
	DiskAlertAcknowledged
	DiskAlertResolved
)

// DiskUsageReport contains disk usage analytics over time.
type DiskUsageReport struct {
	Path               string                   `json:"path"`
	Period             time.Duration            `json:"period"`
	AverageUsage       float64                  `json:"average_usage"`
	PeakUsage          float64                  `json:"peak_usage"`
	GrowthRate         float64                  `json:"growth_rate"`
	UsageHistory       []*DiskUsageHistoryPoint `json:"usage_history"`
	TopRepositories    []*RepositoryUsage       `json:"top_repositories"`
	RecommendedActions []string                 `json:"recommended_actions"`
	EfficiencyScore    float64                  `json:"efficiency_score"`
}

// DiskUsageHistoryPoint represents a point in disk usage history.
type DiskUsageHistoryPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	UsageBytes  int64     `json:"usage_bytes"`
	Percentage  float64   `json:"percentage"`
	ChangeBytes int64     `json:"change_bytes"`
}

// RepositoryUsage represents disk usage by repository.
type RepositoryUsage struct {
	RepositoryURL string    `json:"repository_url"`
	UsageBytes    int64     `json:"usage_bytes"`
	LastAccessed  time.Time `json:"last_accessed"`
	AccessCount   int64     `json:"access_count"`
	Priority      int       `json:"priority"`
}

// DiskUsagePrediction contains predictions for future disk usage.
type DiskUsagePrediction struct {
	Path                 string        `json:"path"`
	CurrentUsage         int64         `json:"current_usage"`
	PredictedUsage       int64         `json:"predicted_usage"`
	TimeToFull           time.Duration `json:"time_to_full"`
	ConfidenceLevel      float64       `json:"confidence_level"`
	RecommendedCleanupGB int64         `json:"recommended_cleanup_gb"`
	PredictionModel      string        `json:"prediction_model"`
	Factors              []string      `json:"factors"`
}

// DiskHealthStatus represents overall disk system health.
type DiskHealthStatus struct {
	OverallHealth DiskHealthLevel         `json:"overall_health"`
	Paths         []*PathHealthStatus     `json:"paths"`
	ActiveAlerts  int                     `json:"active_alerts"`
	RecentErrors  []*DiskError            `json:"recent_errors"`
	Performance   *DiskPerformanceMetrics `json:"performance"`
	LastChecked   time.Time               `json:"last_checked"`
}

// DiskHealthLevel represents health status levels.
type DiskHealthLevel int

const (
	DiskHealthGood DiskHealthLevel = iota
	DiskHealthWarning
	DiskHealthCritical
	DiskHealthFailed
)

// PathHealthStatus represents health status for a specific path.
type PathHealthStatus struct {
	Path      string          `json:"path"`
	Health    DiskHealthLevel `json:"health"`
	Issues    []string        `json:"issues"`
	LastError *DiskError      `json:"last_error,omitempty"`
	IOLatency float64         `json:"io_latency"`
	ErrorRate float64         `json:"error_rate"`
}

// DiskError represents a disk-related error.
type DiskError struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
}

// DiskPerformanceMetrics contains disk performance metrics.
type DiskPerformanceMetrics struct {
	AverageReadLatency  time.Duration `json:"average_read_latency"`
	AverageWriteLatency time.Duration `json:"average_write_latency"`
	CurrentIOPS         int           `json:"current_iops"`
	PeakIOPS            int           `json:"peak_iops"`
	QueueDepth          int           `json:"queue_depth"`
	Throughput          int64         `json:"throughput"`
}
