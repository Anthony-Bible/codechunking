package messaging

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFailureType tests the FailureType enumeration and validation.
func TestFailureType(t *testing.T) {
	t.Run("should create valid failure types", func(t *testing.T) {
		validTypes := []string{
			"NETWORK_ERROR",
			"VALIDATION_ERROR",
			"PROCESSING_ERROR",
			"SYSTEM_ERROR",
			"TIMEOUT_ERROR",
			"RESOURCE_EXHAUSTED",
			"PERMISSION_DENIED",
			"REPOSITORY_NOT_FOUND",
		}

		for _, validType := range validTypes {
			failureType, err := NewFailureType(validType)
			require.NoError(t, err, "Should create valid failure type: %s", validType)
			assert.Equal(t, validType, string(failureType))
		}
	})

	t.Run("should reject invalid failure types", func(t *testing.T) {
		invalidTypes := []string{
			"INVALID_TYPE",
			"",
			"invalid_case",
			"UNKNOWN",
		}

		for _, invalidType := range invalidTypes {
			_, err := NewFailureType(invalidType)
			require.Error(t, err, "Should reject invalid failure type: %s", invalidType)
		}
	})

	t.Run("should classify failure types correctly", func(t *testing.T) {
		// Temporary failures that can be retried
		temporaryFailures := []FailureType{
			FailureTypeNetworkError,
			FailureTypeTimeoutError,
			FailureTypeResourceExhausted,
		}

		for _, failureType := range temporaryFailures {
			assert.True(t, failureType.IsTemporary(),
				"Should classify %s as temporary", string(failureType))
			assert.False(t, failureType.IsPermanent(),
				"Should not classify %s as permanent", string(failureType))
		}

		// Permanent failures that should not be retried
		permanentFailures := []FailureType{
			FailureTypeValidationError,
			FailureTypePermissionDenied,
			FailureTypeRepositoryNotFound,
		}

		for _, failureType := range permanentFailures {
			assert.True(t, failureType.IsPermanent(),
				"Should classify %s as permanent", string(failureType))
			assert.False(t, failureType.IsTemporary(),
				"Should not classify %s as temporary", string(failureType))
		}
	})
}

// TestFailureContext tests failure context metadata.
func TestFailureContext(t *testing.T) {
	t.Run("should create failure context with all fields", func(t *testing.T) {
		context := FailureContext{
			ErrorMessage:  "Repository not found",
			StackTrace:    "main.go:123\nworker.go:456",
			ErrorCode:     "REPO_404",
			Component:     "git-client",
			Operation:     "clone_repository",
			RequestID:     "req-123",
			CorrelationID: "corr-456",
			AdditionalInfo: map[string]interface{}{
				"repository_url": "https://github.com/invalid/repo.git",
				"attempt_count":  3,
			},
		}

		err := context.Validate()
		require.NoError(t, err)
		assert.NotEmpty(t, context.ErrorMessage)
		assert.NotEmpty(t, context.Component)
		assert.NotEmpty(t, context.Operation)
		assert.Len(t, context.AdditionalInfo, 2)
	})

	t.Run("should fail validation with empty error message", func(t *testing.T) {
		context := FailureContext{
			ErrorMessage: "", // Missing required field
			Component:    "git-client",
			Operation:    "clone_repository",
		}

		err := context.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "error_message is required")
	})

	t.Run("should fail validation with empty component", func(t *testing.T) {
		context := FailureContext{
			ErrorMessage: "Some error",
			Component:    "", // Missing required field
			Operation:    "clone_repository",
		}

		err := context.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "component is required")
	})

	t.Run("should fail validation with empty operation", func(t *testing.T) {
		context := FailureContext{
			ErrorMessage: "Some error",
			Component:    "git-client",
			Operation:    "", // Missing required field
		}

		err := context.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "operation is required")
	})
}

// TestDLQMessage tests the Dead Letter Queue message structure.
func TestDLQMessage(t *testing.T) {
	t.Run("should create valid DLQ message", func(t *testing.T) {
		originalMessage := EnhancedIndexingJobMessage{
			MessageID:     "msg-123",
			CorrelationID: "corr-456",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  2,
			MaxRetries:    3,
		}

		failureContext := FailureContext{
			ErrorMessage:  "Git clone failed",
			Component:     "git-client",
			Operation:     "clone_repository",
			RequestID:     "req-789",
			CorrelationID: "corr-456",
		}

		dlqMessage := DLQMessage{
			DLQMessageID:     "dlq-msg-001",
			OriginalMessage:  originalMessage,
			FailureType:      FailureTypeNetworkError,
			FailureContext:   failureContext,
			FirstFailedAt:    time.Now().Add(-1 * time.Hour),
			LastFailedAt:     time.Now(),
			TotalFailures:    3,
			LastRetryAttempt: 2,
			DeadLetterReason: "Maximum retry attempts exceeded",
			ProcessingStage:  "CLONE",
		}

		err := dlqMessage.Validate()
		require.NoError(t, err)
		assert.NotEmpty(t, dlqMessage.DLQMessageID)
		assert.Equal(t, "msg-123", dlqMessage.OriginalMessage.MessageID)
		assert.Equal(t, FailureTypeNetworkError, dlqMessage.FailureType)
	})

	t.Run("should fail validation with empty DLQ message ID", func(t *testing.T) {
		dlqMessage := DLQMessage{
			DLQMessageID: "", // Missing required field
			OriginalMessage: EnhancedIndexingJobMessage{
				MessageID: "msg-123",
			},
			FailureType: FailureTypeNetworkError,
		}

		err := dlqMessage.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "dlq_message_id is required")
	})

	t.Run("should fail validation with invalid original message", func(t *testing.T) {
		dlqMessage := DLQMessage{
			DLQMessageID: "dlq-msg-001",
			OriginalMessage: EnhancedIndexingJobMessage{
				// Missing required fields to make validation fail
				MessageID: "",
			},
			FailureType: FailureTypeNetworkError,
		}

		err := dlqMessage.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "original message validation failed")
	})

	t.Run("should fail validation with invalid failure type", func(t *testing.T) {
		dlqMessage := DLQMessage{
			DLQMessageID: "dlq-msg-001",
			OriginalMessage: EnhancedIndexingJobMessage{
				MessageID:     "msg-123",
				CorrelationID: "corr-456",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/repo.git",
			},
			FailureType: "", // Invalid failure type
		}

		err := dlqMessage.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "failure_type is required")
	})

	t.Run("should fail validation with negative total failures", func(t *testing.T) {
		dlqMessage := DLQMessage{
			DLQMessageID: "dlq-msg-001",
			OriginalMessage: EnhancedIndexingJobMessage{
				MessageID:     "msg-123",
				CorrelationID: "corr-456",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/repo.git",
			},
			FailureType:   FailureTypeNetworkError,
			TotalFailures: -1, // Invalid negative value
		}

		err := dlqMessage.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "total_failures cannot be negative")
	})

	t.Run("should fail validation with last failed before first failed", func(t *testing.T) {
		now := time.Now()
		dlqMessage := DLQMessage{
			DLQMessageID: "dlq-msg-001",
			OriginalMessage: EnhancedIndexingJobMessage{
				MessageID:     "msg-123",
				CorrelationID: "corr-456",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/repo.git",
			},
			FailureType:   FailureTypeNetworkError,
			FirstFailedAt: now,
			LastFailedAt:  now.Add(-1 * time.Hour), // Before first failed
		}

		err := dlqMessage.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "last_failed_at cannot be before first_failed_at")
	})
}

// TestDLQMessageOperations tests DLQ message operations.
func TestDLQMessageOperations(t *testing.T) {
	t.Run("should check if failure is retryable", func(t *testing.T) {
		// Temporary failure - should be retryable
		dlqMessage := DLQMessage{
			FailureType: FailureTypeNetworkError,
		}
		assert.True(t, dlqMessage.IsRetryable())

		// Permanent failure - should not be retryable
		dlqMessage.FailureType = FailureTypeValidationError
		assert.False(t, dlqMessage.IsRetryable())
	})

	t.Run("should calculate failure duration", func(t *testing.T) {
		now := time.Now()
		firstFailed := now.Add(-2 * time.Hour)

		dlqMessage := DLQMessage{
			FirstFailedAt: firstFailed,
			LastFailedAt:  now,
		}

		duration := dlqMessage.FailureDuration()
		assert.Equal(t, 2*time.Hour, duration)
	})

	t.Run("should generate retry message from DLQ", func(t *testing.T) {
		originalMessage := EnhancedIndexingJobMessage{
			MessageID:     "msg-123",
			CorrelationID: "corr-456",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  2,
			MaxRetries:    5,
		}

		dlqMessage := DLQMessage{
			DLQMessageID:    "dlq-msg-001",
			OriginalMessage: originalMessage,
			FailureType:     FailureTypeNetworkError,
		}

		retryMessage, err := dlqMessage.CreateRetryMessage()
		require.NoError(t, err)

		// Should reset retry attempt and get new message ID
		assert.Equal(t, 0, retryMessage.RetryAttempt)
		assert.NotEqual(t, originalMessage.MessageID, retryMessage.MessageID)
		assert.Equal(t, originalMessage.CorrelationID, retryMessage.CorrelationID)
		assert.Equal(t, originalMessage.RepositoryURL, retryMessage.RepositoryURL)
	})

	t.Run("should not create retry message for non-retryable failure", func(t *testing.T) {
		dlqMessage := DLQMessage{
			FailureType: FailureTypeValidationError, // Permanent failure
			OriginalMessage: EnhancedIndexingJobMessage{
				MessageID: "msg-123",
			},
		}

		_, err := dlqMessage.CreateRetryMessage()
		require.Error(t, err)
		assert.ErrorContains(t, err, "failure type is not retryable")
	})
}

// TestFailurePattern tests failure pattern analysis.
func TestFailurePattern(t *testing.T) {
	t.Run("should create valid failure pattern", func(t *testing.T) {
		pattern := FailurePattern{
			FailureType:     FailureTypeNetworkError,
			Component:       "git-client",
			ErrorPattern:    "connection timeout",
			OccurrenceCount: 5,
			FirstSeen:       time.Now().Add(-24 * time.Hour),
			LastSeen:        time.Now(),
			AffectedRepos: []string{
				"https://github.com/example/repo1.git",
				"https://github.com/example/repo2.git",
			},
			Severity: "HIGH",
		}

		err := pattern.Validate()
		require.NoError(t, err)
		assert.Equal(t, 5, pattern.OccurrenceCount)
		assert.Len(t, pattern.AffectedRepos, 2)
	})

	t.Run("should fail validation with negative occurrence count", func(t *testing.T) {
		pattern := FailurePattern{
			FailureType:     FailureTypeNetworkError,
			Component:       "git-client",
			ErrorPattern:    "connection timeout",
			OccurrenceCount: -1, // Invalid negative value
		}

		err := pattern.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "occurrence_count cannot be negative")
	})

	t.Run("should fail validation with invalid severity", func(t *testing.T) {
		pattern := FailurePattern{
			FailureType:     FailureTypeNetworkError,
			Component:       "git-client",
			ErrorPattern:    "connection timeout",
			OccurrenceCount: 5,
			Severity:        "INVALID", // Invalid severity
		}

		err := pattern.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "invalid severity")
	})

	t.Run("should check if pattern is critical", func(t *testing.T) {
		highSeverityPattern := FailurePattern{
			Severity: "HIGH",
		}
		assert.True(t, highSeverityPattern.IsCritical())

		criticalSeverityPattern := FailurePattern{
			Severity: "CRITICAL",
		}
		assert.True(t, criticalSeverityPattern.IsCritical())

		lowSeverityPattern := FailurePattern{
			Severity: "LOW",
		}
		assert.False(t, lowSeverityPattern.IsCritical())
	})
}

// TestDLQStatistics tests DLQ statistics collection.
func TestDLQStatistics(t *testing.T) {
	t.Run("should create valid DLQ statistics", func(t *testing.T) {
		stats := DLQStatistics{
			TotalMessages:     100,
			RetryableMessages: 60,
			PermanentFailures: 40,
			MessagesByFailureType: map[FailureType]int{
				FailureTypeNetworkError:       30,
				FailureTypeValidationError:    25,
				FailureTypeTimeoutError:       20,
				FailureTypeSystemError:        25,
				FailureTypeResourceExhausted:  0,
				FailureTypeProcessingError:    0,
				FailureTypePermissionDenied:   0,
				FailureTypeRepositoryNotFound: 0,
			},
			AverageTimeInDLQ: 2 * time.Hour,
			OldestMessageAge: 24 * time.Hour,
			MessagesLastHour: 15,
			MessagesLastDay:  85,
			RetrySuccessRate: 0.75,
			LastUpdated:      time.Now(),
		}

		err := stats.Validate()
		require.NoError(t, err)
		assert.Equal(t, 100, stats.TotalMessages)
		assert.Equal(t, 60, stats.RetryableMessages)
		assert.Len(t, stats.MessagesByFailureType, 8)
		assert.InEpsilon(t, 0.75, stats.RetrySuccessRate, 0.001)
	})

	t.Run("should fail validation with negative message counts", func(t *testing.T) {
		stats := DLQStatistics{
			TotalMessages: -1, // Invalid negative value
		}

		err := stats.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "total_messages cannot be negative")
	})

	t.Run("should fail validation with invalid retry success rate", func(t *testing.T) {
		stats := DLQStatistics{
			RetrySuccessRate: 1.5, // Invalid rate > 1.0
		}

		err := stats.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "retry_success_rate must be between 0 and 1")
	})

	t.Run("should calculate statistics correctly", func(t *testing.T) {
		stats := DLQStatistics{
			RetryableMessages: 60,
			PermanentFailures: 40,
		}

		assert.Equal(t, 100, stats.CalculateTotalMessages())
		assert.InEpsilon(t, 0.6, stats.CalculateRetryablePercentage(), 0.001)
		assert.InEpsilon(t, 0.4, stats.CalculatePermanentFailurePercentage(), 0.001)
	})
}

// TestDLQMessageUtilities tests utility functions for DLQ messages.
func TestDLQMessageUtilities(t *testing.T) {
	t.Run("should generate unique DLQ message ID", func(t *testing.T) {
		id1 := GenerateDLQMessageID()
		id2 := GenerateDLQMessageID()

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
		assert.Contains(t, id1, "dlq-")
		assert.Contains(t, id2, "dlq-")
	})

	t.Run("should classify failure based on error message", func(t *testing.T) {
		testCases := []struct {
			errorMessage string
			expectedType FailureType
		}{
			{"connection timeout", FailureTypeNetworkError},
			{"dial tcp: connection refused", FailureTypeNetworkError},
			{"invalid repository URL", FailureTypeValidationError},
			{"required field missing", FailureTypeValidationError},
			{"context deadline exceeded", FailureTypeTimeoutError},
			{"operation timed out", FailureTypeTimeoutError},
			{"permission denied", FailureTypePermissionDenied},
			{"access denied", FailureTypePermissionDenied},
			{"repository not found", FailureTypeRepositoryNotFound},
			{"404 not found", FailureTypeRepositoryNotFound},
			{"out of memory", FailureTypeResourceExhausted},
			{"disk full", FailureTypeResourceExhausted},
			{"unexpected error", FailureTypeSystemError},
		}

		for _, testCase := range testCases {
			failureType := ClassifyFailureFromError(testCase.errorMessage)
			assert.Equal(t, testCase.expectedType, failureType,
				"Should classify '%s' as %s", testCase.errorMessage, testCase.expectedType)
		}
	})

	t.Run("should transform message to DLQ format", func(t *testing.T) {
		originalMessage := EnhancedIndexingJobMessage{
			MessageID:     "msg-123",
			CorrelationID: "corr-456",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			RetryAttempt:  2,
			MaxRetries:    3,
		}

		failureContext := FailureContext{
			ErrorMessage: "Git clone failed",
			Component:    "git-client",
			Operation:    "clone_repository",
		}

		dlqMessage, err := TransformToDLQMessage(originalMessage, FailureTypeNetworkError, failureContext, "CLONE")
		require.NoError(t, err)

		assert.NotEmpty(t, dlqMessage.DLQMessageID)
		assert.Equal(t, originalMessage.MessageID, dlqMessage.OriginalMessage.MessageID)
		assert.Equal(t, FailureTypeNetworkError, dlqMessage.FailureType)
		assert.Equal(t, failureContext, dlqMessage.FailureContext)
		assert.Equal(t, "CLONE", dlqMessage.ProcessingStage)
		assert.Equal(t, 1, dlqMessage.TotalFailures)
		assert.False(t, dlqMessage.FirstFailedAt.IsZero())
		assert.False(t, dlqMessage.LastFailedAt.IsZero())
	})
}
