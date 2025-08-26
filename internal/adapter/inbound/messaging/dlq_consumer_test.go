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
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDLQMessageHandler mocks the DLQ message handler interface.
type MockDLQMessageHandler struct {
	mock.Mock
}

func (m *MockDLQMessageHandler) ProcessDLQMessage(ctx context.Context, dlqMessage messaging.DLQMessage) error {
	args := m.Called(ctx, dlqMessage)
	return args.Error(0)
}

func (m *MockDLQMessageHandler) AnalyzeDLQMessage(
	ctx context.Context,
	dlqMessage messaging.DLQMessage,
) (messaging.FailurePattern, error) {
	args := m.Called(ctx, dlqMessage)
	if args.Get(0) == nil {
		return messaging.FailurePattern{}, args.Error(1)
	}
	return args.Get(0).(messaging.FailurePattern), args.Error(1)
}

func (m *MockDLQMessageHandler) GetDLQMessageHistory(
	ctx context.Context,
	messageID string,
) ([]messaging.DLQMessage, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]messaging.DLQMessage), args.Error(1)
}

// MockDLQRetryService mocks the DLQ retry service interface.
type MockDLQRetryService struct {
	mock.Mock
}

func (m *MockDLQRetryService) RetryMessage(ctx context.Context, dlqMessageID string) error {
	args := m.Called(ctx, dlqMessageID)
	return args.Error(0)
}

func (m *MockDLQRetryService) BulkRetryMessages(ctx context.Context, messageIDs []string) error {
	args := m.Called(ctx, messageIDs)
	return args.Error(0)
}

func (m *MockDLQRetryService) CanRetry(ctx context.Context, dlqMessage messaging.DLQMessage) bool {
	args := m.Called(ctx, dlqMessage)
	return args.Bool(0)
}

// MockDLQStatisticsCollector mocks the DLQ statistics collector interface.
type MockDLQStatisticsCollector struct {
	mock.Mock
}

func (m *MockDLQStatisticsCollector) RecordDLQMessage(ctx context.Context, dlqMessage messaging.DLQMessage) error {
	args := m.Called(ctx, dlqMessage)
	return args.Error(0)
}

func (m *MockDLQStatisticsCollector) GetStatistics(ctx context.Context) (messaging.DLQStatistics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return messaging.DLQStatistics{}, args.Error(1)
	}
	return args.Get(0).(messaging.DLQStatistics), args.Error(1)
}

func (m *MockDLQStatisticsCollector) GetFailurePatterns(ctx context.Context) ([]messaging.FailurePattern, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]messaging.FailurePattern), args.Error(1)
}

// TestDLQConsumerCreation tests DLQ consumer creation and configuration.
func TestDLQConsumerCreation(t *testing.T) {
	t.Run("should create DLQ consumer with valid configuration", func(t *testing.T) {
		consumerConfig := DLQConsumerConfig{
			Subject:              "indexing-dlq",
			DurableName:          "dlq-consumer",
			AckWait:              60 * time.Second,
			MaxDeliver:           1, // DLQ messages should not be redelivered
			MaxAckPending:        50,
			ReplayPolicy:         "all",
			DeliverPolicy:        "all",
			ProcessingTimeout:    30 * time.Second,
			MaxProcessingWorkers: 3,
			EnableAnalysis:       true,
			EnableRetry:          true,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 10,
			ReconnectWait: 2 * time.Second,
		}

		mockDLQHandler := &MockDLQMessageHandler{}
		mockRetryService := &MockDLQRetryService{}
		mockStatsCollector := &MockDLQStatisticsCollector{}

		consumer, err := NewDLQConsumer(
			consumerConfig,
			natsConfig,
			mockDLQHandler,
			mockRetryService,
			mockStatsCollector,
		)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should fail with empty DLQ subject", func(t *testing.T) {
		consumerConfig := DLQConsumerConfig{
			Subject:     "", // Invalid empty subject
			DurableName: "dlq-consumer",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockDLQHandler := &MockDLQMessageHandler{}
		mockRetryService := &MockDLQRetryService{}
		mockStatsCollector := &MockDLQStatisticsCollector{}

		consumer, err := NewDLQConsumer(
			consumerConfig,
			natsConfig,
			mockDLQHandler,
			mockRetryService,
			mockStatsCollector,
		)

		require.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should fail with nil DLQ message handler", func(t *testing.T) {
		consumerConfig := DLQConsumerConfig{
			Subject:     "indexing-dlq",
			DurableName: "dlq-consumer",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockRetryService := &MockDLQRetryService{}
		mockStatsCollector := &MockDLQStatisticsCollector{}

		consumer, err := NewDLQConsumer(
			consumerConfig,
			natsConfig,
			nil, // nil handler
			mockRetryService,
			mockStatsCollector,
		)

		require.Error(t, err)
		assert.Nil(t, consumer)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should validate DLQ consumer configuration", func(t *testing.T) {
		invalidConfigs := []DLQConsumerConfig{
			{
				Subject:              "indexing-dlq",
				DurableName:          "", // Missing durable name
				MaxProcessingWorkers: 3,
			},
			{
				Subject:              "indexing-dlq",
				DurableName:          "dlq-consumer",
				MaxProcessingWorkers: 0, // Invalid worker count
			},
			{
				Subject:           "indexing-dlq",
				DurableName:       "dlq-consumer",
				ProcessingTimeout: 0, // Invalid timeout
			},
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockDLQHandler := &MockDLQMessageHandler{}
		mockRetryService := &MockDLQRetryService{}
		mockStatsCollector := &MockDLQStatisticsCollector{}

		for _, invalidConfig := range invalidConfigs {
			consumer, err := NewDLQConsumer(
				invalidConfig,
				natsConfig,
				mockDLQHandler,
				mockRetryService,
				mockStatsCollector,
			)

			require.Error(t, err)
			assert.Nil(t, consumer)
			assert.Contains(t, err.Error(), "not implemented yet")
		}
	})
}

// TestDLQConsumerSubscription tests DLQ message subscription and processing.
func TestDLQConsumerSubscription(t *testing.T) {
	t.Run("should subscribe to DLQ stream successfully", func(t *testing.T) {
		consumerConfig := DLQConsumerConfig{
			Subject:              "indexing-dlq",
			DurableName:          "dlq-consumer",
			AckWait:              60 * time.Second,
			MaxDeliver:           1,
			ProcessingTimeout:    30 * time.Second,
			MaxProcessingWorkers: 2,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockDLQHandler := &MockDLQMessageHandler{}
		mockRetryService := &MockDLQRetryService{}
		mockStatsCollector := &MockDLQStatisticsCollector{}

		consumer := &DLQConsumer{
			config:         consumerConfig,
			natsConfig:     natsConfig,
			dlqHandler:     mockDLQHandler,
			retryService:   mockRetryService,
			statsCollector: mockStatsCollector,
		}

		ctx := context.Background()
		err := consumer.Start(ctx)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should process DLQ message correctly", func(t *testing.T) {
		// Create a valid DLQ message
		originalMessage := messaging.EnhancedIndexingJobMessage{
			MessageID:     "msg-123",
			CorrelationID: "corr-456",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  3,
			MaxRetries:    3,
		}

		dlqMessage := messaging.DLQMessage{
			DLQMessageID:    "dlq-msg-001",
			OriginalMessage: originalMessage,
			FailureType:     messaging.FailureTypeNetworkError,
			FailureContext: messaging.FailureContext{
				ErrorMessage: "connection timeout",
				Component:    "git-client",
				Operation:    "clone_repository",
			},
			FirstFailedAt:    time.Now().Add(-1 * time.Hour),
			LastFailedAt:     time.Now(),
			TotalFailures:    3,
			DeadLetterReason: "Maximum retry attempts exceeded",
			ProcessingStage:  "CLONE",
		}

		jsonData, err := json.Marshal(dlqMessage)
		require.NoError(t, err)

		consumerConfig := DLQConsumerConfig{
			Subject:     "indexing-dlq",
			DurableName: "dlq-consumer",
		}

		mockDLQHandler := &MockDLQMessageHandler{}
		mockStatsCollector := &MockDLQStatisticsCollector{}

		mockDLQHandler.On("ProcessDLQMessage", mock.Anything, mock.AnythingOfType("messaging.DLQMessage")).
			Return(nil)
		mockStatsCollector.On("RecordDLQMessage", mock.Anything, mock.AnythingOfType("messaging.DLQMessage")).
			Return(nil)

		consumer := &DLQConsumer{
			config:         consumerConfig,
			dlqHandler:     mockDLQHandler,
			statsCollector: mockStatsCollector,
		}

		err = consumer.handleDLQMessage(&nats.Msg{
			Subject: "indexing-dlq",
			Data:    jsonData,
		})

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle invalid DLQ message format", func(t *testing.T) {
		invalidData := []byte("invalid json")

		consumerConfig := DLQConsumerConfig{
			Subject:     "indexing-dlq",
			DurableName: "dlq-consumer",
		}

		consumer := &DLQConsumer{
			config: consumerConfig,
		}

		err := consumer.handleDLQMessage(&nats.Msg{
			Subject: "indexing-dlq",
			Data:    invalidData,
		})

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestDLQMessageAnalysis tests DLQ message analysis and investigation capabilities.
func TestDLQMessageAnalysis(t *testing.T) {
	t.Run("should analyze DLQ message for failure patterns", func(t *testing.T) {
		dlqMessage := messaging.DLQMessage{
			DLQMessageID: "dlq-msg-002",
			OriginalMessage: messaging.EnhancedIndexingJobMessage{
				MessageID:     "msg-456",
				RepositoryURL: "https://github.com/example/repo.git",
			},
			FailureType: messaging.FailureTypeNetworkError,
			FailureContext: messaging.FailureContext{
				ErrorMessage: "connection timeout",
				Component:    "git-client",
				Operation:    "clone_repository",
			},
			TotalFailures: 5,
		}

		expectedPattern := messaging.FailurePattern{
			FailureType:     messaging.FailureTypeNetworkError,
			Component:       "git-client",
			ErrorPattern:    "connection timeout",
			OccurrenceCount: 1,
			AffectedRepos:   []string{"https://github.com/example/repo.git"},
			Severity:        "HIGH",
		}

		mockDLQHandler := &MockDLQMessageHandler{}
		mockDLQHandler.On("AnalyzeDLQMessage", mock.Anything, dlqMessage).
			Return(expectedPattern, nil)

		consumer := &DLQConsumer{
			dlqHandler: mockDLQHandler,
		}

		ctx := context.Background()
		pattern, err := consumer.AnalyzeDLQMessage(ctx, dlqMessage)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// In GREEN phase, should return expected pattern
		_ = pattern // Will be used when implementation is complete
	})

	t.Run("should investigate message history", func(t *testing.T) {
		messageID := "msg-789"
		expectedHistory := []messaging.DLQMessage{
			{
				DLQMessageID: "dlq-msg-003",
				OriginalMessage: messaging.EnhancedIndexingJobMessage{
					MessageID: "msg-789",
				},
				FailureType:   messaging.FailureTypeNetworkError,
				TotalFailures: 1,
				FirstFailedAt: time.Now().Add(-2 * time.Hour),
			},
			{
				DLQMessageID: "dlq-msg-004",
				OriginalMessage: messaging.EnhancedIndexingJobMessage{
					MessageID: "msg-789",
				},
				FailureType:   messaging.FailureTypeNetworkError,
				TotalFailures: 2,
				FirstFailedAt: time.Now().Add(-1 * time.Hour),
			},
		}

		mockDLQHandler := &MockDLQMessageHandler{}
		mockDLQHandler.On("GetDLQMessageHistory", mock.Anything, messageID).
			Return(expectedHistory, nil)

		consumer := &DLQConsumer{
			dlqHandler: mockDLQHandler,
		}

		ctx := context.Background()
		history, err := consumer.GetMessageHistory(ctx, messageID)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// In GREEN phase, should return expected history
		_ = history // Will be used when implementation is complete
	})

	t.Run("should handle analysis errors gracefully", func(t *testing.T) {
		dlqMessage := messaging.DLQMessage{
			DLQMessageID: "dlq-msg-error",
		}

		analysisError := errors.New("analysis service unavailable")
		mockDLQHandler := &MockDLQMessageHandler{}
		mockDLQHandler.On("AnalyzeDLQMessage", mock.Anything, dlqMessage).
			Return(messaging.FailurePattern{}, analysisError)

		consumer := &DLQConsumer{
			dlqHandler: mockDLQHandler,
		}

		ctx := context.Background()
		_, err := consumer.AnalyzeDLQMessage(ctx, dlqMessage)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestDLQMessageRetry tests manual retry functionality for recovered messages.
func TestDLQMessageRetry(t *testing.T) {
	t.Run("should retry retryable DLQ message", func(t *testing.T) {
		dlqMessage := messaging.DLQMessage{
			DLQMessageID: "dlq-msg-retry",
			OriginalMessage: messaging.EnhancedIndexingJobMessage{
				MessageID:     "msg-retry",
				RepositoryURL: "https://github.com/example/repo.git",
				RetryAttempt:  3,
				MaxRetries:    3,
			},
			FailureType: messaging.FailureTypeNetworkError, // Temporary failure - retryable
		}

		mockRetryService := &MockDLQRetryService{}
		mockRetryService.On("CanRetry", mock.Anything, dlqMessage).Return(true)
		mockRetryService.On("RetryMessage", mock.Anything, "dlq-msg-retry").Return(nil)

		consumer := &DLQConsumer{
			retryService: mockRetryService,
		}

		ctx := context.Background()
		err := consumer.RetryMessage(ctx, dlqMessage.DLQMessageID)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should not retry non-retryable DLQ message", func(t *testing.T) {
		dlqMessage := messaging.DLQMessage{
			DLQMessageID: "dlq-msg-permanent",
			FailureType:  messaging.FailureTypeValidationError, // Permanent failure - not retryable
		}

		mockRetryService := &MockDLQRetryService{}
		mockRetryService.On("CanRetry", mock.Anything, dlqMessage).Return(false)

		consumer := &DLQConsumer{
			retryService: mockRetryService,
		}

		ctx := context.Background()
		err := consumer.RetryMessage(ctx, dlqMessage.DLQMessageID)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should perform bulk retry of multiple messages", func(t *testing.T) {
		messageIDs := []string{
			"dlq-msg-bulk-1",
			"dlq-msg-bulk-2",
			"dlq-msg-bulk-3",
		}

		mockRetryService := &MockDLQRetryService{}
		mockRetryService.On("BulkRetryMessages", mock.Anything, messageIDs).Return(nil)

		consumer := &DLQConsumer{
			retryService: mockRetryService,
		}

		ctx := context.Background()
		err := consumer.BulkRetryMessages(ctx, messageIDs)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle retry failures", func(t *testing.T) {
		dlqMessageID := "dlq-msg-retry-fail"
		retryError := errors.New("retry service unavailable")

		mockRetryService := &MockDLQRetryService{}
		mockRetryService.On("RetryMessage", mock.Anything, dlqMessageID).Return(retryError)

		consumer := &DLQConsumer{
			retryService: mockRetryService,
		}

		ctx := context.Background()
		err := consumer.RetryMessage(ctx, dlqMessageID)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestDLQStatistics tests DLQ statistics collection and monitoring.
func TestDLQStatistics(t *testing.T) {
	t.Run("should collect DLQ statistics", func(t *testing.T) {
		expectedStats := messaging.DLQStatistics{
			TotalMessages:     25,
			RetryableMessages: 15,
			PermanentFailures: 10,
			MessagesByFailureType: map[messaging.FailureType]int{
				messaging.FailureTypeNetworkError:    8,
				messaging.FailureTypeValidationError: 5,
				messaging.FailureTypeTimeoutError:    7,
				messaging.FailureTypeSystemError:     5,
			},
			AverageTimeInDLQ: 2 * time.Hour,
			OldestMessageAge: 24 * time.Hour,
			MessagesLastHour: 3,
			MessagesLastDay:  12,
			RetrySuccessRate: 0.75,
			LastUpdated:      time.Now(),
		}

		mockStatsCollector := &MockDLQStatisticsCollector{}
		mockStatsCollector.On("GetStatistics", mock.Anything).Return(expectedStats, nil)

		consumer := &DLQConsumer{
			statsCollector: mockStatsCollector,
		}

		ctx := context.Background()
		stats, err := consumer.GetStatistics(ctx)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// In GREEN phase, should return expected stats
		_ = stats // Will be used when implementation is complete
	})

	t.Run("should get failure patterns for alerting", func(t *testing.T) {
		expectedPatterns := []messaging.FailurePattern{
			{
				FailureType:     messaging.FailureTypeNetworkError,
				Component:       "git-client",
				ErrorPattern:    "connection timeout",
				OccurrenceCount: 10,
				Severity:        "HIGH",
				AffectedRepos: []string{
					"https://github.com/example/repo1.git",
					"https://github.com/example/repo2.git",
				},
			},
			{
				FailureType:     messaging.FailureTypeValidationError,
				Component:       "validation-service",
				ErrorPattern:    "invalid URL format",
				OccurrenceCount: 5,
				Severity:        "MEDIUM",
				AffectedRepos: []string{
					"https://github.com/invalid/url.git",
				},
			},
		}

		mockStatsCollector := &MockDLQStatisticsCollector{}
		mockStatsCollector.On("GetFailurePatterns", mock.Anything).Return(expectedPatterns, nil)

		consumer := &DLQConsumer{
			statsCollector: mockStatsCollector,
		}

		ctx := context.Background()
		patterns, err := consumer.GetFailurePatterns(ctx)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// In GREEN phase, should return expected patterns
		_ = patterns // Will be used when implementation is complete
	})

	t.Run("should handle statistics collection errors", func(t *testing.T) {
		statsError := errors.New("statistics database unavailable")

		mockStatsCollector := &MockDLQStatisticsCollector{}
		mockStatsCollector.On("GetStatistics", mock.Anything).
			Return(messaging.DLQStatistics{}, statsError)

		consumer := &DLQConsumer{
			statsCollector: mockStatsCollector,
		}

		ctx := context.Background()
		_, err := consumer.GetStatistics(ctx)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestDLQConsumerLifecycle tests DLQ consumer lifecycle management.
func TestDLQConsumerLifecycle(t *testing.T) {
	t.Run("should start DLQ consumer successfully", func(t *testing.T) {
		consumerConfig := DLQConsumerConfig{
			Subject:              "indexing-dlq",
			DurableName:          "dlq-consumer",
			MaxProcessingWorkers: 2,
		}

		consumer := &DLQConsumer{
			config: consumerConfig,
		}

		ctx := context.Background()
		err := consumer.Start(ctx)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// Health should return empty status in RED phase
		health := consumer.Health()
		assert.False(t, health.IsRunning)
		assert.False(t, health.IsConnected)
	})

	t.Run("should stop DLQ consumer gracefully", func(t *testing.T) {
		consumer := &DLQConsumer{
			running: true,
		}

		ctx := context.Background()
		err := consumer.Stop(ctx)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle concurrent message processing", func(t *testing.T) {
		consumerConfig := DLQConsumerConfig{
			Subject:              "indexing-dlq",
			DurableName:          "dlq-consumer",
			MaxProcessingWorkers: 3, // Multiple workers
		}

		consumer := &DLQConsumer{
			config: consumerConfig,
		}

		// Test that consumer can handle multiple workers
		assert.Equal(t, 3, consumer.MaxWorkers())
	})

	t.Run("should provide health status", func(t *testing.T) {
		consumer := &DLQConsumer{
			running:   true,
			connected: true,
		}

		health := consumer.Health()

		// In RED phase, should return empty health status
		assert.False(t, health.IsRunning)
		assert.False(t, health.IsConnected)
		assert.Equal(t, int64(0), health.MessagesProcessed)
		assert.Equal(t, int64(0), health.ProcessingErrors)
	})
}
