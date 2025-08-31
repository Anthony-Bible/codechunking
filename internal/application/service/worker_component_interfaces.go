package service

import (
	"codechunking/internal/domain/entity"
	"time"
)

// ReliableCircuitBreaker defines the interface for a circuit breaker with reliability features.
type ReliableCircuitBreaker interface {
	// Execute wraps a function call with circuit breaker protection.
	Execute(fn func() error) error

	// IsOpen returns true if the circuit is currently open.
	IsOpen() bool

	// Reset resets the circuit breaker to closed state.
	Reset()

	// GetState returns the current state of the circuit breaker.
	GetState() string

	// GetStatistics returns current circuit breaker statistics.
	GetStatistics() CircuitBreakerStatistics
}

// CircuitBreakerStatistics contains statistics about circuit breaker performance.
type CircuitBreakerStatistics struct {
	State                string        `json:"state"`
	TotalCalls           int64         `json:"total_calls"`
	SuccessfulCalls      int64         `json:"successful_calls"`
	FailedCalls          int64         `json:"failed_calls"`
	RejectedCalls        int64         `json:"rejected_calls"`
	ConsecutiveFailures  int64         `json:"consecutive_failures"`
	ConsecutiveSuccesses int64         `json:"consecutive_successes"`
	FailureRate          float64       `json:"failure_rate"`
	SuccessRate          float64       `json:"success_rate"`
	RejectionRate        float64       `json:"rejection_rate"`
	AverageCallTime      time.Duration `json:"average_call_time"`
	StateTransitions     int64         `json:"state_transitions"`
	TimeInOpenState      time.Duration `json:"time_in_open_state"`
	TimeInHalfOpenState  time.Duration `json:"time_in_half_open_state"`
	LastFailureTime      time.Time     `json:"last_failure_time"`
	LastSuccessTime      time.Time     `json:"last_success_time"`
	LastStateChangeTime  time.Time     `json:"last_state_change_time"`
}

// NonBlockingErrorLogger defines the interface for non-blocking error logging.
type NonBlockingErrorLogger interface {
	// LogError logs an error in a non-blocking manner.
	LogError(err *entity.ClassifiedError) bool

	// GetQueueStatus returns the current status of the error queue.
	GetQueueStatus() (capacity, used int)

	// Shutdown gracefully shuts down the error logger.
	Shutdown() error
}

// CircularErrorBuffer defines the interface for a memory-efficient circular buffer.
type CircularErrorBuffer interface {
	// Add adds an error to the buffer.
	Add(err *entity.ClassifiedError) bool

	// GetAll returns all errors in the buffer.
	GetAll() []*entity.ClassifiedError

	// GetBatch returns a batch of errors from the buffer.
	GetBatch(count int) []*entity.ClassifiedError

	// Size returns the current size of the buffer.
	Size() int

	// Capacity returns the maximum capacity of the buffer.
	Capacity() int

	// Clear clears all errors from the buffer.
	Clear()

	// GetMemoryEstimate returns the estimated memory usage.
	GetMemoryEstimate() int64
}

// SmartAlertDeduplicator defines the interface for smart alert deduplication.
type SmartAlertDeduplicator interface {
	// ShouldSendAlert determines if an alert should be sent based on deduplication rules.
	ShouldSendAlert(alertType, message string) bool

	// RecordAlert records that an alert was sent.
	RecordAlert(alertType, message string)

	// GetStatistics returns deduplication statistics.
	GetStatistics() DeduplicationStatistics

	// Reset resets the deduplicator state.
	Reset()
}

// DeduplicationStatistics contains statistics about alert deduplication.
type DeduplicationStatistics struct {
	TotalAlerts           int64                `json:"total_alerts"`
	DeduplicatedAlerts    int64                `json:"deduplicated_alerts"`
	UniqueAlerts          int64                `json:"unique_alerts"`
	DeduplicationRate     float64              `json:"deduplication_rate"`
	AlertsByType          map[string]int64     `json:"alerts_by_type"`
	LastAlertTimes        map[string]time.Time `json:"last_alert_times"`
	TotalAlertsProcessed  int64                `json:"total_alerts_processed"`
	DuplicatesDetected    int64                `json:"duplicates_detected"`
	AlertsSuppressed      int64                `json:"alerts_suppressed"`
	ActiveKeys            int                  `json:"active_keys"`
	MemoryUsageBytes      int64                `json:"memory_usage_bytes"`
	OldestEntryAge        time.Duration        `json:"oldest_entry_age"`
	AverageKeyLifetime    time.Duration        `json:"average_key_lifetime"`
	SuppressionRate       float64              `json:"suppression_rate"`
	KeyDistribution       map[string]int       `json:"key_distribution"`
	HourlyDuplicateCounts map[int]int64        `json:"hourly_duplicate_counts"`
}
