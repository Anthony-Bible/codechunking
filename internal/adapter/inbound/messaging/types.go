package messaging

import (
	"time"

	"github.com/nats-io/nats.go"
)

const (
	// DefaultJobProcessingTimeout is the default timeout for job processing operations.
	DefaultJobProcessingTimeout = 30 * time.Second
	// IndexingStreamName is the name of the JetStream stream for indexing jobs.
	IndexingStreamName = "INDEXING"
	// StreamRetentionHours is the default retention period for stream messages in hours.
	StreamRetentionHours = 24
	// MessagesFetchBatch is the number of messages to fetch in each batch.
	MessagesFetchBatch = 10
	// MessageFetchMaxWaitSeconds is the maximum wait time for fetching messages.
	MessageFetchMaxWaitSeconds = 5
	// DrainCheckInterval is the interval for checking drain status.
	DrainCheckInterval = 100 * time.Millisecond
	// DummyMessageSize is a placeholder size for simplified GREEN phase implementation.
	DummyMessageSize = len("dummy-message")

	// String constants for config map keys.
	AckWaitKey        = "ack_wait"
	MaxDeliverKey     = "max_deliver"
	MaxAckPendingKey  = "max_ack_pending"
	ReplayPolicyKey   = "replay_policy"
	SubjectKey        = "subject"
	ConsumerTypeKey   = "consumer_type"
	ConsumerTypeValue = "nats_consumer"

	// Empty string constant.
	EmptyString = ""
)

// subscriptionWrapper wraps a NATS subscription to provide test-compatible behavior.
type subscriptionWrapper struct {
	*nats.Subscription

	Subject string // Shadow the embedded Subject field for test compatibility
}

// IsValid returns whether the underlying subscription is valid.
func (sw *subscriptionWrapper) IsValid() bool {
	if sw == nil || sw.Subscription == nil {
		return false
	}
	return sw.Subscription.IsValid()
}

// NATSConsumerInfo provides detailed information about a NATS consumer.
// This is a local definition to avoid circular import issues.
type NATSConsumerInfo struct {
	Name              string            `json:"name"`
	Subject           string            `json:"subject"`
	QueueGroup        string            `json:"queue_group"`
	DurableName       string            `json:"durable_name"`
	ConsumerConfig    map[string]any    `json:"consumer_config"`
	StreamInfo        map[string]any    `json:"stream_info"`
	ConnectionURL     string            `json:"connection_url"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	LastReconnectTime time.Time         `json:"last_reconnect_time,omitempty"`
	ReconnectCount    int64             `json:"reconnect_count"`
}

// ConsumerConfig holds configuration for the message consumer.
type ConsumerConfig struct {
	Subject           string
	QueueGroup        string
	DurableName       string
	AckWait           time.Duration
	MaxDeliver        int
	MaxAckPending     int
	ReplayPolicy      string
	FilterSubject     string
	DeliverPolicy     string
	OptStartSeq       uint64
	OptStartTime      *time.Time
	RateLimitBps      uint64
	MaxWaiting        int
	MaxRequestBatch   int
	MaxRequestExpires time.Duration
	InactiveThreshold time.Duration
}

// AckAlertThresholds defines thresholds for acknowledgment health alerts.
type AckAlertThresholds struct {
	ErrorRateThreshold   float64
	LatencyThresholdP99  time.Duration
	TimeoutRateThreshold float64
}

// AckConsumerConfig holds acknowledgment configuration for the enhanced consumer.
type AckConsumerConfig struct {
	EnableAcknowledgment       bool
	AckTimeout                 time.Duration
	EnableCorrelationTracking  bool
	EnableDuplicateDetection   bool
	DuplicateDetectionWindow   time.Duration
	EnableStatisticsCollection bool
	StatisticsInterval         time.Duration
	BackoffStrategy            string
	InitialBackoffDelay        time.Duration
	MaxBackoffDelay            time.Duration
	AckRetryAttempts           int
	AckRetryDelay              time.Duration
	TimeoutRecoveryMode        string
	TimeoutBackoffDelay        time.Duration
	PermanentFailureRecovery   string
	EnableFailureNotification  bool
	EnablePerformanceMetrics   bool
	MetricsWindowSize          int
	EnableHealthChecks         bool
	HealthCheckInterval        time.Duration
	AlertThresholds            AckAlertThresholds
	GracefulShutdownTimeout    time.Duration
	PreserveStateOnRestart     bool
	StatePreservationTimeout   time.Duration
}
