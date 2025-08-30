package entity

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorAggregation_Creation(t *testing.T) {
	t.Run("should create error aggregation with single error", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"database_connection_failure",
			"Failed to connect to database",
			nil,
		)

		aggregation, err := NewErrorAggregation(
			classifiedError.ErrorPattern(),
			time.Minute*5, // 5 minute window
		)

		require.NoError(t, err)
		assert.NotNil(t, aggregation)
		assert.Equal(t, classifiedError.ErrorPattern(), aggregation.Pattern())
		assert.Equal(t, time.Minute*5, aggregation.WindowDuration())
		assert.Equal(t, 0, aggregation.Count())
		assert.True(t, aggregation.IsEmpty())
		assert.WithinDuration(t, time.Now(), aggregation.WindowStart(), time.Second)
		assert.WithinDuration(t, time.Now().Add(time.Minute*5), aggregation.WindowEnd(), time.Second)
	})

	t.Run("should create error aggregation with custom window duration", func(t *testing.T) {
		pattern := "test_pattern:ERROR"
		windowDuration := time.Hour * 2

		aggregation, err := NewErrorAggregation(pattern, windowDuration)

		require.NoError(t, err)
		assert.Equal(t, pattern, aggregation.Pattern())
		assert.Equal(t, windowDuration, aggregation.WindowDuration())
	})
}

func TestErrorAggregation_ValidationErrors(t *testing.T) {
	testCases := []struct {
		name           string
		pattern        string
		windowDuration time.Duration
		expectedError  string
	}{
		{
			name:           "empty pattern",
			pattern:        "",
			windowDuration: time.Minute,
			expectedError:  "pattern cannot be empty",
		},
		{
			name:           "zero window duration",
			pattern:        "test:ERROR",
			windowDuration: 0,
			expectedError:  "window duration must be positive",
		},
		{
			name:           "negative window duration",
			pattern:        "test:ERROR",
			windowDuration: -time.Minute,
			expectedError:  "window duration must be positive",
		},
		{
			name:           "window duration too short",
			pattern:        "test:ERROR",
			windowDuration: time.Second * 10,
			expectedError:  "window duration must be at least 30 seconds",
		},
		{
			name:           "window duration too long",
			pattern:        "test:ERROR",
			windowDuration: time.Hour * 25,
			expectedError:  "window duration cannot exceed 24 hours",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aggregation, err := NewErrorAggregation(tc.pattern, tc.windowDuration)

			require.Error(t, err)
			assert.Nil(t, aggregation)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestErrorAggregation_AddError(t *testing.T) {
	t.Run("should add error to aggregation successfully", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"database_connection_failure",
			"Failed to connect to database",
			nil,
		)

		aggregation, _ := NewErrorAggregation(
			classifiedError.ErrorPattern(),
			time.Minute*5,
		)

		err := aggregation.AddError(classifiedError)

		require.NoError(t, err)
		assert.Equal(t, 1, aggregation.Count())
		assert.False(t, aggregation.IsEmpty())
		assert.Equal(t, classifiedError.Severity(), aggregation.HighestSeverity())
		assert.WithinDuration(t, classifiedError.Timestamp(), aggregation.FirstOccurrence(), time.Second)
		assert.WithinDuration(t, classifiedError.Timestamp(), aggregation.LastOccurrence(), time.Second)
	})

	t.Run("should add multiple errors and track occurrences", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		pattern := "database_connection_failure:ERROR"

		aggregation, _ := NewErrorAggregation(pattern, time.Minute*5)

		// Add first error
		error1, _ := NewClassifiedError(ctx, assert.AnError, severity, "database_connection_failure", "Error 1", nil)
		err := aggregation.AddError(error1)
		require.NoError(t, err)

		// Add second error after small delay
		time.Sleep(time.Millisecond * 10)
		error2, _ := NewClassifiedError(ctx, assert.AnError, severity, "database_connection_failure", "Error 2", nil)
		err = aggregation.AddError(error2)
		require.NoError(t, err)

		assert.Equal(t, 2, aggregation.Count())
		assert.WithinDuration(t, error1.Timestamp(), aggregation.FirstOccurrence(), time.Second)
		assert.WithinDuration(t, error2.Timestamp(), aggregation.LastOccurrence(), time.Second)
		assert.True(t, aggregation.FirstOccurrence().Before(aggregation.LastOccurrence()) ||
			aggregation.FirstOccurrence().Equal(aggregation.LastOccurrence()))
	})

	t.Run("should track highest severity across multiple errors", func(t *testing.T) {
		ctx := context.Background()
		errorSev, _ := valueobject.NewErrorSeverity("ERROR")
		criticalSev, _ := valueobject.NewErrorSeverity("CRITICAL")
		pattern := "system_failure:ERROR"

		aggregation, _ := NewErrorAggregation(pattern, time.Minute*5)

		// Add error-level first
		error1, _ := NewClassifiedError(ctx, assert.AnError, errorSev, "system_failure", "Error 1", nil)
		aggregation.AddError(error1)
		assert.Equal(t, errorSev, aggregation.HighestSeverity())

		// Add critical error - should become highest
		error2, _ := NewClassifiedError(ctx, assert.AnError, criticalSev, "system_failure", "Critical Error", nil)
		aggregation.AddError(error2)
		assert.Equal(t, criticalSev, aggregation.HighestSeverity())
	})

	t.Run("should reject error with different pattern", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		aggregation, _ := NewErrorAggregation("database_failure:ERROR", time.Minute*5)

		// Try to add error with different pattern
		differentError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"git_clone_failure",
			"Different error",
			nil,
		)
		err := aggregation.AddError(differentError)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "error pattern does not match aggregation pattern")
		assert.Equal(t, 0, aggregation.Count())
	})

	t.Run("should reject error outside time window", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		pattern := "test_error:ERROR"

		aggregation, _ := NewErrorAggregation(pattern, time.Minute*5)

		// Create an old error (simulate error from past window)
		oldError, _ := NewClassifiedError(ctx, assert.AnError, severity, "test_error", "Old error", nil)
		// Manipulate timestamp to be outside window (this would need reflection or test-specific methods)

		// For now, test the concept - in real implementation we'd have methods to check time windows
		err := aggregation.AddError(oldError)

		// Should succeed initially, but we'd need windowing logic
		require.NoError(t, err)
		// TODO: Add actual windowing validation when implementing
	})
}

func TestErrorAggregation_WindowManagement(t *testing.T) {
	t.Run("should detect when window is expired", func(t *testing.T) {
		pattern := "test_error:ERROR"
		aggregation, err := NewErrorAggregation(pattern, time.Second*30) // Use minimum valid duration
		require.NoError(t, err)

		// Initially not expired
		assert.False(t, aggregation.IsWindowExpired())

		// For testing purposes, manually set a very short window
		aggregation.SetWindowEndForTesting(time.Now().Add(time.Millisecond * 50))

		// Wait for window to expire
		time.Sleep(time.Millisecond * 100)
		assert.True(t, aggregation.IsWindowExpired())
	})

	t.Run("should reset aggregation window", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		pattern := "test_error:ERROR"

		aggregation, _ := NewErrorAggregation(pattern, time.Minute*5)

		// Add some errors
		error1, _ := NewClassifiedError(ctx, assert.AnError, severity, "test_error", "Error 1", nil)
		aggregation.AddError(error1)

		assert.Equal(t, 1, aggregation.Count())
		assert.False(t, aggregation.IsEmpty())

		// Reset window
		err := aggregation.ResetWindow()
		require.NoError(t, err)

		assert.Equal(t, 0, aggregation.Count())
		assert.True(t, aggregation.IsEmpty())
		assert.WithinDuration(t, time.Now(), aggregation.WindowStart(), time.Second)
		assert.WithinDuration(t, time.Now().Add(time.Minute*5), aggregation.WindowEnd(), time.Second)
	})

	t.Run("should extend window duration", func(t *testing.T) {
		pattern := "test_error:ERROR"
		originalDuration := time.Minute * 5
		aggregation, _ := NewErrorAggregation(pattern, originalDuration)

		originalEnd := aggregation.WindowEnd()

		newDuration := time.Minute * 10
		err := aggregation.ExtendWindow(newDuration)

		require.NoError(t, err)
		assert.Equal(t, newDuration, aggregation.WindowDuration())
		assert.True(t, aggregation.WindowEnd().After(originalEnd))
	})
}

func TestErrorAggregation_ThresholdDetection(t *testing.T) {
	t.Run("should detect when count threshold is exceeded", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		pattern := "database_failure:ERROR"

		aggregation, _ := NewErrorAggregation(pattern, time.Minute*5)
		threshold := 3

		// Add errors below threshold
		for range threshold - 1 {
			error, _ := NewClassifiedError(ctx, assert.AnError, severity, "database_failure", "Error", nil)
			aggregation.AddError(error)
		}
		assert.False(t, aggregation.ExceedsCountThreshold(threshold))

		// Add one more to exceed threshold
		error, _ := NewClassifiedError(ctx, assert.AnError, severity, "database_failure", "Final error", nil)
		aggregation.AddError(error)
		assert.True(t, aggregation.ExceedsCountThreshold(threshold))
	})

	t.Run("should detect when rate threshold is exceeded", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		pattern := "performance_issue:ERROR"

		aggregation, _ := NewErrorAggregation(pattern, time.Minute)

		// Add multiple errors quickly
		for range 10 {
			error, _ := NewClassifiedError(ctx, assert.AnError, severity, "performance_issue", "Performance error", nil)
			aggregation.AddError(error)
		}

		// Rate: 10 errors per minute = 0.167 errors per second
		rateThreshold := 0.1 // 0.1 errors per second
		assert.True(t, aggregation.ExceedsRateThreshold(rateThreshold))

		// Lower rate threshold
		highRateThreshold := 1.0 // 1 error per second
		assert.False(t, aggregation.ExceedsRateThreshold(highRateThreshold))
	})
}

func TestErrorAggregation_Serialization(t *testing.T) {
	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		pattern := "system_failure:CRITICAL"

		aggregation, _ := NewErrorAggregation(pattern, time.Minute*10)

		// Add a test error
		error, _ := NewClassifiedError(ctx, assert.AnError, severity, "system_failure", "System failure", nil)
		aggregation.AddError(error)

		jsonData, err := aggregation.MarshalJSON()

		require.NoError(t, err)
		assert.Contains(t, string(jsonData), pattern)
		assert.Contains(t, string(jsonData), "CRITICAL")
		assert.Contains(t, string(jsonData), "600") // 10 minutes in seconds
	})

	t.Run("should deserialize from JSON correctly", func(t *testing.T) {
		jsonData := `{
			"pattern": "test_error:ERROR",
			"window_duration": 300,
			"window_start": "2023-01-01T00:00:00Z",
			"window_end": "2023-01-01T00:05:00Z",
			"count": 5,
			"highest_severity": "ERROR",
			"first_occurrence": "2023-01-01T00:00:30Z",
			"last_occurrence": "2023-01-01T00:04:30Z"
		}`

		var aggregation ErrorAggregation
		err := aggregation.UnmarshalJSON([]byte(jsonData))

		require.NoError(t, err)
		assert.Equal(t, "test_error:ERROR", aggregation.Pattern())
		assert.Equal(t, time.Minute*5, aggregation.WindowDuration())
		assert.Equal(t, 5, aggregation.Count())
		assert.True(t, aggregation.HighestSeverity().IsError())
	})
}

func TestErrorAggregation_SimilarityDetection(t *testing.T) {
	t.Run("should detect similar error patterns", func(t *testing.T) {
		pattern1 := "database_connection_failure:ERROR"
		pattern2 := "database_timeout_failure:ERROR"
		pattern3 := "git_clone_failure:ERROR"

		aggregation1, _ := NewErrorAggregation(pattern1, time.Minute*5)
		aggregation2, _ := NewErrorAggregation(pattern2, time.Minute*5)
		aggregation3, _ := NewErrorAggregation(pattern3, time.Minute*5)

		// Database-related errors should be similar
		similarity12 := aggregation1.CalculateSimilarity(aggregation2)
		t.Logf("Similarity between database_connection and database_timeout: %f", similarity12)
		assert.GreaterOrEqual(t, similarity12, 0.5) // At least 50% similar

		// Database vs Git should be less similar
		similarity13 := aggregation1.CalculateSimilarity(aggregation3)
		t.Logf("Similarity between database_connection and git_clone: %f", similarity13)
		assert.Less(t, similarity13, 0.3) // Less than 30% similar
	})

	t.Run("should group related aggregations", func(t *testing.T) {
		aggregations := make([]*ErrorAggregation, 0)

		// Create multiple related aggregations
		patterns := []string{
			"database_connection_failure:ERROR",
			"database_timeout_failure:ERROR",
			"database_query_failure:ERROR",
			"git_clone_failure:ERROR",
			"git_push_failure:ERROR",
		}

		for _, pattern := range patterns {
			agg, _ := NewErrorAggregation(pattern, time.Minute*5)
			aggregations = append(aggregations, agg)
		}

		groups := GroupSimilarAggregations(aggregations, 0.5) // 50% similarity threshold

		// Should have 2 groups: database-related and git-related
		assert.Len(t, groups, 2)

		// Database group should have 3 items
		var databaseGroupSize, gitGroupSize int
		for _, group := range groups {
			if len(group) == 3 {
				databaseGroupSize = len(group)
			} else if len(group) == 2 {
				gitGroupSize = len(group)
			}
		}

		assert.Equal(t, 3, databaseGroupSize)
		assert.Equal(t, 2, gitGroupSize)
	})
}

func TestErrorAggregation_CascadeFailureDetection(t *testing.T) {
	t.Run("should detect cascade failure patterns", func(t *testing.T) {
		ctx := context.Background()

		// Create sequence of errors that could indicate cascade failure
		databaseSev, _ := valueobject.NewErrorSeverity("CRITICAL")
		apiSev, _ := valueobject.NewErrorSeverity("ERROR")
		workerSev, _ := valueobject.NewErrorSeverity("ERROR")

		// Database failure first
		dbError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			databaseSev,
			"database_connection_failure",
			"Database down",
			nil,
		)
		dbAggregation, _ := NewErrorAggregation(dbError.ErrorPattern(), time.Minute*5)
		dbAggregation.AddError(dbError)

		// API failures follow
		time.Sleep(time.Millisecond * 10)
		apiError, _ := NewClassifiedError(ctx, assert.AnError, apiSev, "api_request_failure", "API failing", nil)
		apiAggregation, _ := NewErrorAggregation(apiError.ErrorPattern(), time.Minute*5)
		apiAggregation.AddError(apiError)

		// Worker failures follow
		time.Sleep(time.Millisecond * 10)
		workerError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			workerSev,
			"worker_processing_failure",
			"Worker failing",
			nil,
		)
		workerAggregation, _ := NewErrorAggregation(workerError.ErrorPattern(), time.Minute*5)
		workerAggregation.AddError(workerError)

		aggregations := []*ErrorAggregation{dbAggregation, apiAggregation, workerAggregation}

		// Detect cascade pattern
		isCascade := DetectCascadeFailure(aggregations, time.Second*1) // 1 second window for cascade detection
		assert.True(t, isCascade)
	})

	t.Run("should not detect cascade when errors are unrelated in time", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Create errors with significant time gaps
		error1, _ := NewClassifiedError(ctx, assert.AnError, severity, "error_type_1", "Error 1", nil)
		agg1, _ := NewErrorAggregation(error1.ErrorPattern(), time.Minute*5)
		agg1.AddError(error1)

		// Wait longer than cascade detection window
		time.Sleep(time.Millisecond * 100)
		error2, _ := NewClassifiedError(ctx, assert.AnError, severity, "error_type_2", "Error 2", nil)
		agg2, _ := NewErrorAggregation(error2.ErrorPattern(), time.Minute*5)
		agg2.AddError(error2)

		aggregations := []*ErrorAggregation{agg1, agg2}

		// Should not detect cascade with small window
		isCascade := DetectCascadeFailure(aggregations, time.Millisecond*50)
		assert.False(t, isCascade)
	})
}
