package worker

import (
	"codechunking/internal/domain/messaging"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDLQPublisher mocks the DLQ message publisher interface.
type MockDLQPublisher struct {
	mock.Mock
}

func (m *MockDLQPublisher) PublishDLQMessage(ctx context.Context, dlqMessage messaging.DLQMessage) error {
	args := m.Called(ctx, dlqMessage)
	return args.Error(0)
}

func (m *MockDLQPublisher) GetDLQStats(ctx context.Context) (messaging.DLQStatistics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return messaging.DLQStatistics{}, args.Error(1)
	}
	return args.Get(0).(messaging.DLQStatistics), args.Error(1)
}

// MockFailureAnalyzer mocks the failure analysis interface.
type MockFailureAnalyzer struct {
	mock.Mock
}

func (m *MockFailureAnalyzer) AnalyzeFailure(
	ctx context.Context,
	message messaging.EnhancedIndexingJobMessage,
	err error,
	processingStage string,
) messaging.FailureContext {
	args := m.Called(ctx, message, err, processingStage)
	return args.Get(0).(messaging.FailureContext)
}

func (m *MockFailureAnalyzer) ClassifyFailure(ctx context.Context, err error) messaging.FailureType {
	args := m.Called(ctx, err)
	return args.Get(0).(messaging.FailureType)
}

func (m *MockFailureAnalyzer) ShouldRetry(
	ctx context.Context,
	message messaging.EnhancedIndexingJobMessage,
	failureType messaging.FailureType,
) bool {
	args := m.Called(ctx, message, failureType)
	return args.Bool(0)
}

// TestDLQHandlerCreation tests DLQ handler creation and configuration.
func TestDLQHandlerCreation(t *testing.T) {
	t.Run("should create DLQ handler with valid configuration", func(t *testing.T) {
		config := DLQHandlerConfig{
			MaxRetryAttempts:      3,
			RetryBackoffDuration:  5 * time.Second,
			DLQTimeout:            30 * time.Second,
			FailureAnalysisDepth:  2,
			EnablePatternTracking: true,
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		require.NotNil(t, handler)
		assert.Equal(t, 3, handler.MaxRetryAttempts())
		assert.Equal(t, 5*time.Second, handler.RetryBackoffDuration())
		assert.True(t, handler.IsPatternTrackingEnabled())
	})

	t.Run("should fail with nil DLQ publisher", func(t *testing.T) {
		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		mockFailureAnalyzer := &MockFailureAnalyzer{}

		handler := NewDLQHandler(config, nil, mockFailureAnalyzer)

		// Should fail in RED phase with validation error
		require.Error(t, handler.Validate())
		assert.Contains(t, handler.Validate().Error(), "dlq publisher is required")
	})

	t.Run("should fail with nil failure analyzer", func(t *testing.T) {
		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		mockDLQPublisher := &MockDLQPublisher{}

		handler := NewDLQHandler(config, mockDLQPublisher, nil)

		// Should fail in RED phase with validation error
		require.Error(t, handler.Validate())
		assert.Contains(t, handler.Validate().Error(), "failure analyzer is required")
	})

	t.Run("should fail with invalid retry attempts", func(t *testing.T) {
		config := DLQHandlerConfig{
			MaxRetryAttempts: -1, // Invalid negative value
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		require.Error(t, handler.Validate())
		assert.Contains(t, handler.Validate().Error(), "max_retry_attempts must be positive")
	})
}

// TestFailureDetection tests automatic failure detection and routing decisions.
func TestFailureDetection(t *testing.T) {
	t.Run("should detect retry limit exceeded", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     "msg-123",
			CorrelationID: "corr-456",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  3,
			MaxRetries:    3, // Retry limit exceeded
		}

		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		handler := NewDLQHandler(config, nil, nil)

		shouldRoute := handler.ShouldRouteToDeceitLetterQueue(message, errors.New("processing failed"))

		// Should route to DLQ when retry limit exceeded
		assert.True(t, shouldRoute)
	})

	t.Run("should not route to DLQ when retries available", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     "msg-123",
			CorrelationID: "corr-456",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  1,
			MaxRetries:    3, // Still has retries available
		}

		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		mockFailureAnalyzer := &MockFailureAnalyzer{}
		mockFailureAnalyzer.On("ClassifyFailure", mock.Anything, mock.AnythingOfType("*errors.errorString")).
			Return(messaging.FailureTypeNetworkError)
		mockFailureAnalyzer.On("ShouldRetry", mock.Anything, message, messaging.FailureTypeNetworkError).
			Return(true)

		handler := NewDLQHandler(config, nil, mockFailureAnalyzer)

		shouldRoute := handler.ShouldRouteToDeceitLetterQueue(message, errors.New("temporary network error"))

		// Should not route to DLQ when retries are available and failure is temporary
		assert.False(t, shouldRoute)
	})

	t.Run("should route permanent failures to DLQ immediately", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     "msg-123",
			CorrelationID: "corr-456",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  1,
			MaxRetries:    3,
		}

		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		mockFailureAnalyzer := &MockFailureAnalyzer{}
		mockFailureAnalyzer.On("ClassifyFailure", mock.Anything, mock.AnythingOfType("*errors.errorString")).
			Return(messaging.FailureTypeValidationError) // Permanent failure
		mockFailureAnalyzer.On("ShouldRetry", mock.Anything, message, messaging.FailureTypeValidationError).
			Return(false)

		handler := NewDLQHandler(config, nil, mockFailureAnalyzer)

		shouldRoute := handler.ShouldRouteToDeceitLetterQueue(message, errors.New("invalid repository URL"))

		// Should route permanent failures to DLQ immediately
		assert.True(t, shouldRoute)
	})
}

// TestMessageEnrichment tests message enrichment with failure context and metadata.
func TestMessageEnrichment(t *testing.T) {
	t.Run("should enrich message with failure context", func(t *testing.T) {
		originalMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     "msg-123",
			CorrelationID: "corr-456",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  2,
			MaxRetries:    3,
		}

		failureError := errors.New("git clone failed: connection timeout")
		processingStage := "CLONE"

		expectedFailureContext := messaging.FailureContext{
			ErrorMessage:  "git clone failed: connection timeout",
			Component:     "git-client",
			Operation:     "clone_repository",
			CorrelationID: "corr-456",
			AdditionalInfo: map[string]interface{}{
				"repository_url": "https://github.com/example/repo.git",
				"retry_attempt":  2,
			},
		}

		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		mockFailureAnalyzer.On("AnalyzeFailure", mock.Anything, originalMessage, failureError, processingStage).
			Return(expectedFailureContext)
		mockFailureAnalyzer.On("ClassifyFailure", mock.Anything, failureError).
			Return(messaging.FailureTypeNetworkError)

		// Expect DLQ message to be published with enriched context
		mockDLQPublisher.On("PublishDLQMessage", mock.Anything, mock.MatchedBy(func(dlqMsg messaging.DLQMessage) bool {
			return dlqMsg.OriginalMessage.MessageID == "msg-123" &&
				dlqMsg.FailureType == messaging.FailureTypeNetworkError &&
				dlqMsg.FailureContext.ErrorMessage == "git clone failed: connection timeout" &&
				dlqMsg.ProcessingStage == "CLONE"
		})).Return(nil)

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		ctx := context.Background()
		err := handler.RouteToDeadLetterQueue(ctx, originalMessage, failureError, processingStage)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should include processing metadata in DLQ message", func(t *testing.T) {
		originalMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     "msg-456",
			CorrelationID: "corr-789",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo2.git",
			RetryAttempt:  0,
			MaxRetries:    3,
			ProcessingMetadata: messaging.ProcessingMetadata{
				ChunkSizeBytes:   2048,
				MaxFileSizeBytes: 10485760,
			},
			ProcessingContext: messaging.ProcessingContext{
				TimeoutSeconds: 600,
				MaxMemoryMB:    2048,
			},
		}

		failureError := errors.New("parsing failed: invalid syntax")
		processingStage := "PARSE"

		config := DLQHandlerConfig{
			MaxRetryAttempts:      3,
			FailureAnalysisDepth:  2,
			EnablePatternTracking: true,
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		expectedFailureContext := messaging.FailureContext{
			ErrorMessage: "parsing failed: invalid syntax",
			Component:    "code-parser",
			Operation:    "parse_directory",
			AdditionalInfo: map[string]interface{}{
				"chunk_size_bytes":    2048,
				"max_file_size_bytes": int64(10485760),
				"timeout_seconds":     600,
			},
		}

		mockFailureAnalyzer.On("AnalyzeFailure", mock.Anything, originalMessage, failureError, processingStage).
			Return(expectedFailureContext)
		mockFailureAnalyzer.On("ClassifyFailure", mock.Anything, failureError).
			Return(messaging.FailureTypeProcessingError)

		mockDLQPublisher.On("PublishDLQMessage", mock.Anything, mock.AnythingOfType("messaging.DLQMessage")).
			Return(nil)

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		ctx := context.Background()
		err := handler.RouteToDeadLetterQueue(ctx, originalMessage, failureError, processingStage)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestFailureClassification tests failure type classification and analysis.
func TestFailureClassification(t *testing.T) {
	t.Run("should classify network failures correctly", func(t *testing.T) {
		testCases := []struct {
			errorMessage      string
			expectedType      messaging.FailureType
			expectedRetryable bool
		}{
			{"connection timeout", messaging.FailureTypeNetworkError, true},
			{"dial tcp: connection refused", messaging.FailureTypeNetworkError, true},
			{"invalid repository URL", messaging.FailureTypeValidationError, false},
			{"context deadline exceeded", messaging.FailureTypeTimeoutError, true},
			{"permission denied", messaging.FailureTypePermissionDenied, false},
			{"repository not found", messaging.FailureTypeRepositoryNotFound, false},
			{"out of memory", messaging.FailureTypeResourceExhausted, true},
		}

		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		mockFailureAnalyzer := &MockFailureAnalyzer{}

		for _, testCase := range testCases {
			mockFailureAnalyzer.On("ClassifyFailure", mock.Anything, mock.MatchedBy(func(err error) bool {
				return err.Error() == testCase.errorMessage
			})).Return(testCase.expectedType)

			mockFailureAnalyzer.On("ShouldRetry", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage"), testCase.expectedType).
				Return(testCase.expectedRetryable)
		}

		handler := NewDLQHandler(config, nil, mockFailureAnalyzer)

		for _, testCase := range testCases {
			t.Run(testCase.errorMessage, func(t *testing.T) {
				message := messaging.EnhancedIndexingJobMessage{
					RetryAttempt: 1,
					MaxRetries:   3,
				}

				err := errors.New(testCase.errorMessage)
				failureType := handler.ClassifyFailure(context.Background(), err)
				shouldRetry := handler.ShouldRetryMessage(context.Background(), message, failureType)

				assert.Equal(t, testCase.expectedType, failureType)
				assert.Equal(t, testCase.expectedRetryable, shouldRetry)
			})
		}
	})

	t.Run("should track failure patterns", func(t *testing.T) {
		config := DLQHandlerConfig{
			MaxRetryAttempts:      3,
			EnablePatternTracking: true,
		}

		handler := NewDLQHandler(config, nil, nil)

		// Multiple failures with same pattern
		failures := []struct {
			repoURL string
			error   string
		}{
			{"https://github.com/example/repo1.git", "connection timeout"},
			{"https://github.com/example/repo2.git", "connection timeout"},
			{"https://github.com/example/repo3.git", "connection timeout"},
		}

		ctx := context.Background()

		for _, failure := range failures {
			message := messaging.EnhancedIndexingJobMessage{
				RepositoryURL: failure.repoURL,
			}

			err := errors.New(failure.error)
			handler.TrackFailurePattern(ctx, message, messaging.FailureTypeNetworkError, err)
		}

		patterns := handler.GetFailurePatterns(ctx)

		// Should detect pattern in RED phase
		require.Error(t, patterns.Error)
		assert.Contains(t, patterns.Error.Error(), "not implemented yet")
	})
}

// TestDLQPublishing tests publishing messages to the dead letter queue.
func TestDLQPublishing(t *testing.T) {
	t.Run("should publish DLQ message successfully", func(t *testing.T) {
		originalMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     "msg-789",
			CorrelationID: "corr-abc",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  3,
			MaxRetries:    3,
		}

		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
			DLQTimeout:       30 * time.Second,
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		failureContext := messaging.FailureContext{
			ErrorMessage: "Maximum retry attempts exceeded",
			Component:    "job-processor",
			Operation:    "process_job",
		}

		mockFailureAnalyzer.On("AnalyzeFailure", mock.Anything, originalMessage, mock.AnythingOfType("*errors.errorString"), "PROCESSING").
			Return(failureContext)
		mockFailureAnalyzer.On("ClassifyFailure", mock.Anything, mock.AnythingOfType("*errors.errorString")).
			Return(messaging.FailureTypeSystemError)

		// Mock successful publishing
		mockDLQPublisher.On("PublishDLQMessage", mock.Anything, mock.MatchedBy(func(dlqMsg messaging.DLQMessage) bool {
			return dlqMsg.OriginalMessage.MessageID == "msg-789" &&
				dlqMsg.FailureType == messaging.FailureTypeSystemError &&
				dlqMsg.DeadLetterReason == "Maximum retry attempts exceeded"
		})).Return(nil)

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		ctx := context.Background()
		err := handler.RouteToDeadLetterQueue(ctx, originalMessage, errors.New("max retries exceeded"), "PROCESSING")

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle DLQ publishing failures", func(t *testing.T) {
		originalMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     "msg-failed",
			RepositoryID:  uuid.New(),
			RetryAttempt:  3,
			MaxRetries:    3,
		}

		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		failureContext := messaging.FailureContext{
			ErrorMessage: "Processing failed",
			Component:    "job-processor",
			Operation:    "process_job",
		}

		mockFailureAnalyzer.On("AnalyzeFailure", mock.Anything, originalMessage, mock.AnythingOfType("*errors.errorString"), "PROCESSING").
			Return(failureContext)
		mockFailureAnalyzer.On("ClassifyFailure", mock.Anything, mock.AnythingOfType("*errors.errorString")).
			Return(messaging.FailureTypeSystemError)

		// Mock publishing failure
		publishError := errors.New("DLQ stream unavailable")
		mockDLQPublisher.On("PublishDLQMessage", mock.Anything, mock.AnythingOfType("messaging.DLQMessage")).
			Return(publishError)

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		ctx := context.Background()
		err := handler.RouteToDeadLetterQueue(ctx, originalMessage, errors.New("processing failed"), "PROCESSING")

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should timeout DLQ operations", func(t *testing.T) {
		originalMessage := messaging.EnhancedIndexingJobMessage{
			IndexingJobID: uuid.New(),
			MessageID:     "msg-timeout",
			RepositoryID:  uuid.New(),
			RetryAttempt:  3,
			MaxRetries:    3,
		}

		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
			DLQTimeout:       1 * time.Millisecond, // Very short timeout
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		ctx := context.Background()
		err := handler.RouteToDeadLetterQueue(ctx, originalMessage, errors.New("timeout test"), "PROCESSING")

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestDLQHandlerIntegration tests integration with job processor for failure detection.
func TestDLQHandlerIntegration(t *testing.T) {
	t.Run("should integrate with job processor for automatic failure handling", func(t *testing.T) {
		config := DLQHandlerConfig{
			MaxRetryAttempts:      3,
			RetryBackoffDuration:  1 * time.Second,
			EnablePatternTracking: true,
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		// Simulate job processor calling DLQ handler
		messages := []messaging.EnhancedIndexingJobMessage{
			{
				IndexingJobID: uuid.New(),
				MessageID:     "msg-1",
				RetryAttempt:  3,
				MaxRetries:    3,
				RepositoryURL: "https://github.com/example/repo1.git",
			},
			{
				MessageID:     "msg-2",
				RetryAttempt:  3,
				MaxRetries:    3,
				RepositoryURL: "https://github.com/example/repo2.git",
			},
		}

		errors := []error{
			errors.New("git clone failed: connection timeout"),
			errors.New("parsing failed: invalid syntax"),
		}

		ctx := context.Background()

		for i, message := range messages {
			err := handler.HandleJobFailure(ctx, message, errors[i], "PROCESSING")

			// Should fail in RED phase - not implemented yet
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented yet")
		}
	})

	t.Run("should collect DLQ statistics and metrics", func(t *testing.T) {
		config := DLQHandlerConfig{
			MaxRetryAttempts: 3,
		}

		mockDLQPublisher := &MockDLQPublisher{}
		mockFailureAnalyzer := &MockFailureAnalyzer{}

		expectedStats := messaging.DLQStatistics{
			TotalMessages:     10,
			RetryableMessages: 6,
			PermanentFailures: 4,
			MessagesByFailureType: map[messaging.FailureType]int{
				messaging.FailureTypeNetworkError:       3,
				messaging.FailureTypeValidationError:    2,
				messaging.FailureTypeTimeoutError:       2,
				messaging.FailureTypeSystemError:        3,
				messaging.FailureTypeResourceExhausted:  0,
				messaging.FailureTypeProcessingError:    0,
				messaging.FailureTypePermissionDenied:   0,
				messaging.FailureTypeRepositoryNotFound: 0,
			},
			RetrySuccessRate: 0.8,
			LastUpdated:      time.Now(),
		}

		mockDLQPublisher.On("GetDLQStats", mock.Anything).Return(expectedStats, nil)

		handler := NewDLQHandler(config, mockDLQPublisher, mockFailureAnalyzer)

		ctx := context.Background()
		stats, err := handler.GetDLQStatistics(ctx)

		// Should fail in RED phase - not implemented yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// In GREEN phase, should return expected stats
		_ = stats // Will be used when implementation is complete
	})
}
