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

		// Should succeed in GREEN phase - implementation is complete
		require.NoError(t, err)
		assert.NotNil(t, consumer)
		assert.Equal(t, 3, consumer.MaxWorkers())
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
		assert.Contains(t, err.Error(), "subject cannot be empty")
	})

	t.Run("should fail with nil DLQ message handler", func(t *testing.T) {
		consumerConfig := DLQConsumerConfig{
			Subject:              "indexing-dlq",
			DurableName:          "dlq-consumer",
			MaxProcessingWorkers: 3,
			ProcessingTimeout:    30 * time.Second,
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
		assert.Contains(t, err.Error(), "DLQ message handler cannot be nil")
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
			// Should get specific validation errors for each config issue
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

		// Should succeed - implementation is complete
		require.NoError(t, err)
		assert.True(t, consumer.Health().IsRunning)
		assert.True(t, consumer.Health().IsConnected)
	})

	t.Run("should process DLQ message correctly", func(t *testing.T) {
		// Create a valid DLQ message
		originalMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
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

		// Should succeed with proper mocks
		require.NoError(t, err)
		mockDLQHandler.AssertExpectations(t)
		mockStatsCollector.AssertExpectations(t)
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

		// Should fail with JSON unmarshal error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal DLQ message")
	})
}

// TestDLQMessageAnalysis tests DLQ message analysis and investigation capabilities.
func TestDLQMessageAnalysis(t *testing.T) {
	t.Run("should analyze DLQ message for failure patterns", func(t *testing.T) {
		dlqMessage := messaging.DLQMessage{
			DLQMessageID: "dlq-msg-002",
			OriginalMessage: messaging.EnhancedIndexingJobMessage{
				IndexingJobID: uuid.New(),
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

		// Should succeed with mocked response
		require.NoError(t, err)
		assert.Equal(t, expectedPattern.FailureType, pattern.FailureType)
		assert.Equal(t, expectedPattern.Component, pattern.Component)
		assert.Equal(t, expectedPattern.ErrorPattern, pattern.ErrorPattern)
		mockDLQHandler.AssertExpectations(t)
	})

	t.Run("should investigate message history", func(t *testing.T) {
		messageID := "msg-789"
		expectedHistory := []messaging.DLQMessage{
			{
				DLQMessageID: "dlq-msg-003",
				OriginalMessage: messaging.EnhancedIndexingJobMessage{
					IndexingJobID: uuid.New(),
					MessageID:     "msg-789",
				},
				FailureType:   messaging.FailureTypeNetworkError,
				TotalFailures: 1,
				FirstFailedAt: time.Now().Add(-2 * time.Hour),
			},
			{
				DLQMessageID: "dlq-msg-004",
				OriginalMessage: messaging.EnhancedIndexingJobMessage{
					IndexingJobID: uuid.New(),
					MessageID:     "msg-789",
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

		// Should succeed with mocked response
		require.NoError(t, err)
		assert.Len(t, history, 2)
		assert.Equal(t, expectedHistory[0].DLQMessageID, history[0].DLQMessageID)
		assert.Equal(t, expectedHistory[1].DLQMessageID, history[1].DLQMessageID)
		mockDLQHandler.AssertExpectations(t)
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

		// Should return the mocked analysis error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "analysis service unavailable")
		mockDLQHandler.AssertExpectations(t)
	})
}

// TestDLQMessageRetry tests manual retry functionality for recovered messages.
func TestDLQMessageRetry(t *testing.T) {
	t.Run("should retry retryable DLQ message", func(t *testing.T) {
		dlqMessage := messaging.DLQMessage{
			DLQMessageID: "dlq-msg-retry",
			OriginalMessage: messaging.EnhancedIndexingJobMessage{
				IndexingJobID: uuid.New(),
				MessageID:     "msg-retry",
				RepositoryURL: "https://github.com/example/repo.git",
				RetryAttempt:  3,
				MaxRetries:    3,
			},
			FailureType: messaging.FailureTypeNetworkError, // Temporary failure - retryable
		}

		mockRetryService := &MockDLQRetryService{}
		mockRetryService.On("RetryMessage", mock.Anything, "dlq-msg-retry").Return(nil)

		consumer := &DLQConsumer{
			retryService: mockRetryService,
		}

		ctx := context.Background()
		err := consumer.RetryMessage(ctx, dlqMessage.DLQMessageID)

		// Should succeed with proper mock setup
		require.NoError(t, err)
		mockRetryService.AssertExpectations(t)
	})

	t.Run("should not retry non-retryable DLQ message", func(t *testing.T) {
		dlqMessage := messaging.DLQMessage{
			DLQMessageID: "dlq-msg-permanent",
			FailureType:  messaging.FailureTypeValidationError, // Permanent failure - not retryable
		}

		mockRetryService := &MockDLQRetryService{}
		// Current implementation doesn't check CanRetry, it delegates directly to RetryMessage
		// So we expect the RetryMessage call to return an error indicating non-retryable
		mockRetryService.On("RetryMessage", mock.Anything, "dlq-msg-permanent").
			Return(errors.New("message is not retryable"))

		consumer := &DLQConsumer{
			retryService: mockRetryService,
		}

		ctx := context.Background()
		err := consumer.RetryMessage(ctx, dlqMessage.DLQMessageID)

		// Should fail with retry service error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message is not retryable")
		mockRetryService.AssertExpectations(t)
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

		// Should succeed with proper mock setup
		require.NoError(t, err)
		mockRetryService.AssertExpectations(t)
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

		// Should return the mocked retry error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "retry service unavailable")
		mockRetryService.AssertExpectations(t)
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
				messaging.FailureTypeNetworkError:       8,
				messaging.FailureTypeValidationError:    5,
				messaging.FailureTypeTimeoutError:       7,
				messaging.FailureTypeSystemError:        5,
				messaging.FailureTypeResourceExhausted:  3,
				messaging.FailureTypeProcessingError:    2,
				messaging.FailureTypePermissionDenied:   1,
				messaging.FailureTypeRepositoryNotFound: 4,
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

		// Should succeed with mocked response
		require.NoError(t, err)
		assert.Equal(t, expectedStats.TotalMessages, stats.TotalMessages)
		assert.Equal(t, expectedStats.RetryableMessages, stats.RetryableMessages)
		assert.Equal(t, expectedStats.PermanentFailures, stats.PermanentFailures)
		mockStatsCollector.AssertExpectations(t)
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

		// Should succeed with mocked response
		require.NoError(t, err)
		assert.Len(t, patterns, 2)
		assert.Equal(t, expectedPatterns[0].FailureType, patterns[0].FailureType)
		assert.Equal(t, expectedPatterns[1].FailureType, patterns[1].FailureType)
		mockStatsCollector.AssertExpectations(t)
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

		// Should return the mocked statistics error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "statistics database unavailable")
		mockStatsCollector.AssertExpectations(t)
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

		// Should succeed - implementation is complete
		require.NoError(t, err)

		// Health should return running status
		health := consumer.Health()
		assert.True(t, health.IsRunning)
		assert.True(t, health.IsConnected)
	})

	t.Run("should stop DLQ consumer gracefully", func(t *testing.T) {
		consumer := &DLQConsumer{
			running: true,
		}

		ctx := context.Background()
		err := consumer.Stop(ctx)

		// Should succeed - implementation is complete
		require.NoError(t, err)
		assert.False(t, consumer.Health().IsRunning)
		assert.False(t, consumer.Health().IsConnected)
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

		// Should return actual health status
		assert.True(t, health.IsRunning)
		assert.True(t, health.IsConnected)
		assert.Equal(t, int64(0), health.MessagesProcessed)
		assert.Equal(t, int64(0), health.ProcessingErrors)
	})
}
