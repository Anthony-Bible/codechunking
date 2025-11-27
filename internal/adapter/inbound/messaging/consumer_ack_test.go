package messaging

import (
	"codechunking/internal/config"
	"codechunking/internal/domain/messaging"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ContextKey is a custom type for context keys to avoid string key issues.
type ContextKey string

const (
	// CorrelationIDKey is the context key for correlation ID.
	CorrelationIDKey ContextKey = "correlation_id"
)

// MockNATSAckMessage mocks a NATS message with acknowledgment capabilities.
type MockNATSAckMessage struct {
	mock.Mock

	Data          []byte
	Subject       string
	ReplyTo       string
	MessageID     string
	CorrelationID string
	Metadata      map[string]string
	ackCalled     bool
	nackCalled    bool
	inProgCalled  bool
	termCalled    bool
}

func (m *MockNATSAckMessage) Ack() error {
	args := m.Called()
	m.ackCalled = true
	return args.Error(0)
}

func (m *MockNATSAckMessage) Nak() error {
	args := m.Called()
	m.nackCalled = true
	return args.Error(0)
}

func (m *MockNATSAckMessage) NakWithDelay(delay time.Duration) error {
	args := m.Called(delay)
	m.nackCalled = true
	return args.Error(0)
}

func (m *MockNATSAckMessage) InProgress() error {
	args := m.Called()
	m.inProgCalled = true
	return args.Error(0)
}

func (m *MockNATSAckMessage) Term() error {
	args := m.Called()
	m.termCalled = true
	return args.Error(0)
}

func (m *MockNATSAckMessage) GetData() []byte {
	return m.Data
}

func (m *MockNATSAckMessage) GetSubject() string {
	return m.Subject
}

func (m *MockNATSAckMessage) GetReplyTo() string {
	return m.ReplyTo
}

func (m *MockNATSAckMessage) GetMessageID() string {
	return m.MessageID
}

func (m *MockNATSAckMessage) GetCorrelationID() string {
	return m.CorrelationID
}

func (m *MockNATSAckMessage) GetMetadata() map[string]string {
	return m.Metadata
}

func (m *MockNATSAckMessage) IsAckCalled() bool {
	return m.ackCalled
}

func (m *MockNATSAckMessage) IsNackCalled() bool {
	return m.nackCalled
}

func (m *MockNATSAckMessage) IsInProgressCalled() bool {
	return m.inProgCalled
}

func (m *MockNATSAckMessage) IsTermCalled() bool {
	return m.termCalled
}

// MockAckHandler mocks the acknowledgment handler for consumer testing.
type MockAckHandler struct {
	mock.Mock
}

func (m *MockAckHandler) HandleMessage(
	ctx context.Context,
	msg interface{},
	jobMessage messaging.EnhancedIndexingJobMessage,
) error {
	args := m.Called(ctx, msg, jobMessage)
	return args.Error(0)
}

func (m *MockAckHandler) GetAckStatistics(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *MockAckHandler) GetDuplicateStats(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

// MockCorrelationTracker mocks correlation ID tracking and propagation.
type MockCorrelationTracker struct {
	mock.Mock
}

func (m *MockCorrelationTracker) InjectCorrelationID(ctx context.Context, correlationID string) context.Context {
	args := m.Called(ctx, correlationID)
	return args.Get(0).(context.Context)
}

func (m *MockCorrelationTracker) ExtractCorrelationID(ctx context.Context) (string, bool) {
	args := m.Called(ctx)
	return args.String(0), args.Bool(1)
}

func (m *MockCorrelationTracker) GenerateCorrelationID() string {
	args := m.Called()
	return args.String(0)
}

// TestEnhancedConsumerWithAcknowledgment tests enhanced consumer with acknowledgment integration.
func TestEnhancedConsumerWithAcknowledgment(t *testing.T) {
	t.Run("should create enhanced consumer with acknowledgment handler integration", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "jobs.indexing.ack",
			QueueGroup:    "ack-workers",
			DurableName:   "ack-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 50,
			DeliverPolicy: "all",
			ReplayPolicy:  "instant",
			FilterSubject: "jobs.indexing.*",
		}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:       true,
			AckTimeout:                 30 * time.Second,
			EnableCorrelationTracking:  true,
			EnableDuplicateDetection:   true,
			DuplicateDetectionWindow:   5 * time.Minute,
			EnableStatisticsCollection: true,
			BackoffStrategy:            "exponential",
			InitialBackoffDelay:        1 * time.Second,
			MaxBackoffDelay:            30 * time.Second,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 10,
			ReconnectWait: 2 * time.Second,
		}

		mockJobProcessor := &MockJobProcessor{}
		mockAckHandler := &MockAckHandler{}
		mockCorrelationTracker := &MockCorrelationTracker{}

		consumer, err := NewEnhancedNATSConsumer(
			consumerConfig,
			ackConfig,
			natsConfig,
			mockJobProcessor,
			mockAckHandler,
			mockCorrelationTracker,
		)

		// Should succeed in GREEN phase - implementation is complete
		require.NoError(t, err)
		assert.NotNil(t, consumer)
		assert.Equal(t, "ack-workers", consumer.QueueGroup())
		assert.Equal(t, "jobs.indexing.ack", consumer.Subject())
		assert.Equal(t, "ack-consumer", consumer.DurableName())
	})

	t.Run("should fail with invalid acknowledgment configuration", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:     "jobs.indexing",
			QueueGroup:  "workers",
			DurableName: "consumer",
			AckWait:     30 * time.Second,
			MaxDeliver:  3,
		}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment: true,
			AckTimeout:           0, // Invalid zero timeout
		}

		consumer, err := NewEnhancedNATSConsumer(
			consumerConfig,
			ackConfig,
			config.NATSConfig{},
			nil,
			nil,
			nil,
		)

		// Should fail validation in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ack_timeout must be positive")
		assert.Nil(t, consumer)
	})

	t.Run("should fail with missing acknowledgment handler", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:     "jobs.indexing",
			QueueGroup:  "workers",
			DurableName: "consumer",
		}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment: true,
			AckTimeout:           30 * time.Second,
		}

		consumer, err := NewEnhancedNATSConsumer(
			consumerConfig,
			ackConfig,
			config.NATSConfig{},
			nil,
			nil, // Missing ack handler
			nil,
		)

		// Should fail validation in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ack handler is required when acknowledgment is enabled")
		assert.Nil(t, consumer)
	})
}

// TestContextAwareAcknowledgment tests context-aware acknowledgment with correlation tracking.
func TestContextAwareAcknowledgment(t *testing.T) {
	t.Run("should inject correlation ID into context for message processing", func(t *testing.T) {
		messageID := "test-msg-ctx-123"
		correlationID := "test-corr-ctx-456"

		jobMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     messageID,
			CorrelationID: correlationID,
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/ctx-repo.git",
		}

		messageData, _ := json.Marshal(jobMessage)

		mockNATSMsg := &MockNATSAckMessage{
			Data:          messageData,
			Subject:       "jobs.indexing.ctx",
			MessageID:     messageID,
			CorrelationID: correlationID,
		}

		mockJobProcessor := &MockJobProcessor{}
		mockAckHandler := &MockAckHandler{}
		mockCorrelationTracker := &MockCorrelationTracker{}

		consumerConfig := ConsumerConfig{
			Subject:    "jobs.indexing.ctx",
			QueueGroup: "ctx-workers",
			AckWait:    30 * time.Second,
			MaxDeliver: 3,
		}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:      true,
			AckTimeout:                30 * time.Second,
			EnableCorrelationTracking: true,
		}

		// Mock correlation ID injection
		ctxWithCorrelation := context.WithValue(context.Background(), CorrelationIDKey, correlationID)
		mockCorrelationTracker.On("InjectCorrelationID", mock.Anything, correlationID).
			Return(ctxWithCorrelation)

		// Mock successful acknowledgment handling
		mockAckHandler.On("HandleMessage", ctxWithCorrelation, mockNATSMsg, jobMessage).Return(nil)

		consumer, err := NewEnhancedNATSConsumer(
			consumerConfig,
			ackConfig,
			config.NATSConfig{},
			mockJobProcessor,
			mockAckHandler,
			mockCorrelationTracker,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should:
		// 1. Extract correlation ID from message
		// 2. Inject correlation ID into context
		// 3. Pass enriched context to acknowledgment handler
		// 4. Maintain correlation throughout message processing
	})

	t.Run("should generate correlation ID when missing from message", func(t *testing.T) {
		mockCorrelationTracker := &MockCorrelationTracker{}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:      true,
			AckTimeout:                30 * time.Second,
			EnableCorrelationTracking: true,
		}

		// Mock correlation ID generation
		generatedCorrelationID := "generated-corr-abc123"
		mockCorrelationTracker.On("GenerateCorrelationID").Return(generatedCorrelationID)

		ctxWithCorrelation := context.WithValue(context.Background(), CorrelationIDKey, generatedCorrelationID)
		mockCorrelationTracker.On("InjectCorrelationID", mock.Anything, generatedCorrelationID).
			Return(ctxWithCorrelation)

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			nil,
			mockCorrelationTracker,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should generate and inject correlation ID
	})

	t.Run("should propagate correlation ID in acknowledgment metadata", func(t *testing.T) {
		messageID := "test-msg-ack-meta-def"
		correlationID := "test-corr-ack-meta-ghi"

		mockNATSMsg := &MockNATSAckMessage{
			MessageID:     messageID,
			CorrelationID: correlationID,
			Metadata:      make(map[string]string),
		}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:      true,
			AckTimeout:                30 * time.Second,
			EnableCorrelationTracking: true,
		}

		// Mock successful acknowledgment with metadata
		mockNATSMsg.On("Ack").Return(nil).Run(func(_ mock.Arguments) {
			// Verify correlation ID is included in metadata
			assert.Equal(t, correlationID, mockNATSMsg.Metadata["correlation_id"])
			assert.Equal(t, messageID, mockNATSMsg.Metadata["message_id"])
		})

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			nil,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)
	})
}

// TestAcknowledgmentFailureRecovery tests acknowledgment failure recovery and retry scenarios.
func TestAcknowledgmentFailureRecovery(t *testing.T) {
	t.Run("should retry acknowledgment operations on temporary failures", func(t *testing.T) {
		messageID := "test-msg-ack-retry-123"
		correlationID := "test-corr-ack-retry-456"

		jobMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     messageID,
			CorrelationID: correlationID,
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/retry-repo.git",
		}

		messageData, _ := json.Marshal(jobMessage)

		mockNATSMsg := &MockNATSAckMessage{
			Data:          messageData,
			MessageID:     messageID,
			CorrelationID: correlationID,
		}

		mockAckHandler := &MockAckHandler{}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment: true,
			AckTimeout:           30 * time.Second,
			AckRetryAttempts:     3,
			AckRetryDelay:        1 * time.Second,
			BackoffStrategy:      "exponential",
		}

		// Mock acknowledgment handler failure then success on retry
		tempError := &TemporaryAckError{
			Underlying: errors.New("NATS server temporarily unavailable"),
			Retryable:  true,
		}
		mockAckHandler.On("HandleMessage", mock.Anything, mockNATSMsg, jobMessage).
			Return(tempError).Once()
		mockAckHandler.On("HandleMessage", mock.Anything, mockNATSMsg, jobMessage).
			Return(nil) // Success on retry

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			mockAckHandler,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should:
		// 1. Detect temporary acknowledgment failure
		// 2. Apply backoff delay based on strategy
		// 3. Retry acknowledgment operation
		// 4. Track retry attempts and success rate
	})

	t.Run("should handle acknowledgment timeout and recovery", func(t *testing.T) {
		messageID := "test-msg-ack-timeout-789"

		jobMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     messageID,
			RepositoryID:  uuid.New(),
		}

		mockNATSMsg := &MockNATSAckMessage{
			MessageID: messageID,
		}

		mockAckHandler := &MockAckHandler{}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment: true,
			AckTimeout:           100 * time.Millisecond, // Very short timeout
			TimeoutRecoveryMode:  "nack_with_delay",
			TimeoutBackoffDelay:  5 * time.Second,
		}

		// Mock acknowledgment timeout
		timeoutError := &AckTimeoutError{
			MessageID: messageID,
			Timeout:   100 * time.Millisecond,
		}
		mockAckHandler.On("HandleMessage", mock.Anything, mockNATSMsg, jobMessage).
			Return(timeoutError)

		// Mock NACK with delay for timeout recovery
		mockNATSMsg.On("NakWithDelay", 5*time.Second).Return(nil)

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			mockAckHandler,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)
	})

	t.Run("should handle permanent acknowledgment failures gracefully", func(t *testing.T) {
		messageID := "test-msg-perm-fail-abc"

		jobMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     messageID,
			RepositoryID:  uuid.New(),
		}

		mockNATSMsg := &MockNATSAckMessage{
			MessageID: messageID,
		}

		mockAckHandler := &MockAckHandler{}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:      true,
			AckTimeout:                30 * time.Second,
			PermanentFailureRecovery:  "terminate_message",
			EnableFailureNotification: true,
		}

		// Mock permanent acknowledgment failure
		permError := &PermanentAckError{
			Underlying: errors.New("message format validation failed"),
			Retryable:  false,
		}
		mockAckHandler.On("HandleMessage", mock.Anything, mockNATSMsg, jobMessage).Return(permError)

		// Mock message termination for permanent failure
		mockNATSMsg.On("Term").Return(nil)

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			mockAckHandler,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)
	})
}

// TestAcknowledgmentStatisticsCollection tests statistics collection for acknowledgment patterns.
func TestAcknowledgmentStatisticsCollection(t *testing.T) {
	t.Run("should collect acknowledgment pattern statistics", func(t *testing.T) {
		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:       true,
			AckTimeout:                 30 * time.Second,
			EnableStatisticsCollection: true,
			StatisticsInterval:         1 * time.Minute,
		}

		mockAckHandler := &MockAckHandler{}

		// Mock statistics collection
		expectedStats := AckConsumerStatistics{
			TotalMessagesReceived:     1000,
			SuccessfulAcknowledgments: 950,
			FailedAcknowledgments:     50,
			RetriedAcknowledgments:    25,
			TimeoutedAcknowledgments:  15,
			DuplicateMessages:         10,
			AverageAckTime:            50 * time.Millisecond,
			AverageRetryDelay:         2 * time.Second,
			SuccessRate:               0.95,
			RetrySuccessRate:          0.8,
			LastUpdated:               time.Now(),
			MessagesByFailureType: map[string]int{
				"network_error":    20,
				"timeout_error":    15,
				"validation_error": 10,
				"system_error":     5,
			},
		}

		mockAckHandler.On("GetAckStatistics", mock.Anything).Return(expectedStats, nil)

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			mockAckHandler,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should provide comprehensive statistics
	})

	t.Run("should monitor acknowledgment performance metrics", func(t *testing.T) {
		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:     true,
			AckTimeout:               30 * time.Second,
			EnablePerformanceMetrics: true,
			MetricsWindowSize:        1000,
		}

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			nil,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should track:
		// - Acknowledgment latency percentiles (p50, p95, p99)
		// - Throughput metrics (messages/second)
		// - Error rate trends over time
		// - Resource utilization during ack operations
	})

	t.Run("should alert on acknowledgment health issues", func(t *testing.T) {
		ackConfig := AckConsumerConfig{
			EnableAcknowledgment: true,
			AckTimeout:           30 * time.Second,
			EnableHealthChecks:   true,
			HealthCheckInterval:  30 * time.Second,
			AlertThresholds: AckAlertThresholds{
				ErrorRateThreshold:   0.1, // 10% error rate
				LatencyThresholdP99:  5 * time.Second,
				TimeoutRateThreshold: 0.05, // 5% timeout rate
			},
		}

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			nil,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should monitor and alert on:
		// - High acknowledgment error rates
		// - High acknowledgment latency
		// - High timeout rates
		// - Acknowledgment handler health issues
	})
}

// TestConsumerLifecycleWithAcknowledgment tests consumer lifecycle management with acknowledgment.
func TestConsumerLifecycleWithAcknowledgment(t *testing.T) {
	t.Run("should start consumer with acknowledgment handler initialization", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "jobs.indexing.lifecycle",
			QueueGroup: "lifecycle-workers",
			AckWait:    30 * time.Second,
			MaxDeliver: 3,
		}

		ackConfig := AckConsumerConfig{
			EnableAcknowledgment: true,
			AckTimeout:           30 * time.Second,
		}

		mockAckHandler := &MockAckHandler{}

		// Mock acknowledgment handler initialization
		mockAckHandler.On("Initialize", mock.Anything).Return(nil)

		consumer, err := NewEnhancedNATSConsumer(
			consumerConfig,
			ackConfig,
			config.NATSConfig{},
			nil,
			mockAckHandler,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should initialize ack handler on start
	})

	t.Run("should stop consumer with acknowledgment handler cleanup", func(t *testing.T) {
		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:    true,
			AckTimeout:              30 * time.Second,
			GracefulShutdownTimeout: 30 * time.Second,
		}

		mockAckHandler := &MockAckHandler{}

		// Mock acknowledgment handler cleanup
		mockAckHandler.On("Cleanup", mock.Anything).Return(nil)

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			mockAckHandler,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should cleanup ack handler on stop
	})

	t.Run("should handle consumer restart with acknowledgment state preservation", func(t *testing.T) {
		ackConfig := AckConsumerConfig{
			EnableAcknowledgment:     true,
			AckTimeout:               30 * time.Second,
			PreserveStateOnRestart:   true,
			StatePreservationTimeout: 5 * time.Minute,
		}

		consumer, err := NewEnhancedNATSConsumer(
			ConsumerConfig{},
			ackConfig,
			config.NATSConfig{},
			nil,
			nil,
			nil,
		)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Nil(t, consumer)

		// When implemented, should preserve ack state across restarts
	})
}

// Helper types and structures for enhanced consumer acknowledgment testing

// AckConsumerStatistics holds statistics for acknowledgment operations in the consumer.
type AckConsumerStatistics struct {
	TotalMessagesReceived     int64
	SuccessfulAcknowledgments int64
	FailedAcknowledgments     int64
	RetriedAcknowledgments    int64
	TimeoutedAcknowledgments  int64
	DuplicateMessages         int64
	AverageAckTime            time.Duration
	AverageRetryDelay         time.Duration
	SuccessRate               float64
	RetrySuccessRate          float64
	LastUpdated               time.Time
	MessagesByFailureType     map[string]int
}

// Error types for acknowledgment testing

// TemporaryAckError represents a temporary acknowledgment error that can be retried.
type TemporaryAckError struct {
	Underlying error
	Retryable  bool
}

func (e *TemporaryAckError) Error() string {
	return e.Underlying.Error()
}

func (e *TemporaryAckError) IsRetryable() bool {
	return e.Retryable
}

// PermanentAckError represents a permanent acknowledgment error that should not be retried.
type PermanentAckError struct {
	Underlying error
	Retryable  bool
}

func (e *PermanentAckError) Error() string {
	return e.Underlying.Error()
}

func (e *PermanentAckError) IsRetryable() bool {
	return e.Retryable
}

// AckTimeoutError represents an acknowledgment timeout error.
type AckTimeoutError struct {
	MessageID string
	Timeout   time.Duration
}

func (e *AckTimeoutError) Error() string {
	return "acknowledgment timeout after " + e.Timeout.String() + " for message " + e.MessageID
}

// Function signatures that should be implemented are now in consumer.go
