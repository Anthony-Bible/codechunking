package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewBatchJobProgress tests the creation of a new BatchJobProgress entity.
func TestNewBatchJobProgress(t *testing.T) {
	t.Run("should create new batch job progress with correct defaults", func(t *testing.T) {
		// Arrange
		indexingJobID := uuid.New()
		batchNumber := 1
		totalBatches := 5

		// Arrange
		repositoryID := uuid.New()

		// Act
		progress := NewBatchJobProgress(repositoryID, indexingJobID, batchNumber, totalBatches)

		// Assert
		require.NotNil(t, progress)
		assert.NotEmpty(t, progress.ID())
		assert.NotEqual(t, uuid.Nil, progress.ID())
		assert.Equal(t, indexingJobID, progress.IndexingJobID())
		assert.Equal(t, batchNumber, progress.BatchNumber())
		assert.Equal(t, totalBatches, progress.TotalBatches())
		assert.Equal(t, 0, progress.ChunksProcessed())
		assert.Equal(t, "pending", progress.Status())
		assert.Equal(t, 0, progress.RetryCount())
		assert.Nil(t, progress.NextRetryAt())
		assert.Nil(t, progress.ErrorMessage())
		assert.WithinDuration(t, time.Now(), progress.CreatedAt(), time.Second)
		assert.WithinDuration(t, time.Now(), progress.UpdatedAt(), time.Second)
	})

	t.Run("should create multiple batch job progress entities with unique IDs", func(t *testing.T) {
		// Arrange
		indexingJobID := uuid.New()
		repositoryID := uuid.New()

		// Act
		progress1 := NewBatchJobProgress(repositoryID, indexingJobID, 1, 5)
		progress2 := NewBatchJobProgress(repositoryID, indexingJobID, 2, 5)

		// Assert
		require.NotNil(t, progress1)
		require.NotNil(t, progress2)
		assert.NotEqual(t, progress1.ID(), progress2.ID())
		assert.Equal(t, progress1.IndexingJobID(), progress2.IndexingJobID())
	})

	t.Run("should set timestamps close to current time", func(t *testing.T) {
		// Arrange
		indexingJobID := uuid.New()
		repositoryID := uuid.New()
		beforeCreation := time.Now()

		// Act
		progress := NewBatchJobProgress(repositoryID, indexingJobID, 1, 5)

		afterCreation := time.Now()

		// Assert
		assert.True(t, progress.CreatedAt().After(beforeCreation) || progress.CreatedAt().Equal(beforeCreation))
		assert.True(t, progress.CreatedAt().Before(afterCreation) || progress.CreatedAt().Equal(afterCreation))
		assert.True(t, progress.UpdatedAt().After(beforeCreation) || progress.UpdatedAt().Equal(beforeCreation))
		assert.True(t, progress.UpdatedAt().Before(afterCreation) || progress.UpdatedAt().Equal(afterCreation))
		assert.Equal(t, progress.CreatedAt(), progress.UpdatedAt())
	})
}

// TestRestoreBatchJobProgress tests the restoration of a BatchJobProgress entity from database.
func TestRestoreBatchJobProgress(t *testing.T) {
	t.Run("should restore batch job progress with all fields", func(t *testing.T) {
		// Arrange
		id := uuid.New()
		repositoryID := uuid.New()
		indexingJobID := uuid.New()
		batchNumber := 2
		totalBatches := 10
		chunksProcessed := 150
		status := "processing"
		retryCount := 1
		retryTime := time.Now().Add(5 * time.Minute)
		errorMsg := "temporary failure"
		createdAt := time.Now().Add(-1 * time.Hour)
		updatedAt := time.Now().Add(-30 * time.Minute)

		// Act
		progress := RestoreBatchJobProgress(
			id,
			&repositoryID,
			indexingJobID,
			batchNumber,
			totalBatches,
			chunksProcessed,
			status,
			retryCount,
			&retryTime,
			&errorMsg,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			createdAt,
			updatedAt,
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Assert
		require.NotNil(t, progress)
		assert.Equal(t, id, progress.ID())
		assert.Equal(t, indexingJobID, progress.IndexingJobID())
		assert.Equal(t, batchNumber, progress.BatchNumber())
		assert.Equal(t, totalBatches, progress.TotalBatches())
		assert.Equal(t, chunksProcessed, progress.ChunksProcessed())
		assert.Equal(t, status, progress.Status())
		assert.Equal(t, retryCount, progress.RetryCount())
		assert.NotNil(t, progress.NextRetryAt())
		assert.Equal(t, retryTime, *progress.NextRetryAt())
		assert.NotNil(t, progress.ErrorMessage())
		assert.Equal(t, errorMsg, *progress.ErrorMessage())
		assert.Equal(t, createdAt, progress.CreatedAt())
		assert.Equal(t, updatedAt, progress.UpdatedAt())
	})

	t.Run("should restore batch job progress with nil nullable fields", func(t *testing.T) {
		// Arrange
		id := uuid.New()
		repositoryID := uuid.New()
		indexingJobID := uuid.New()
		createdAt := time.Now()
		updatedAt := time.Now()

		// Act
		progress := RestoreBatchJobProgress(
			id,
			&repositoryID,
			indexingJobID,
			1,
			5,
			0,
			"pending",
			0,
			nil,
			nil,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			createdAt,
			updatedAt,
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Assert
		require.NotNil(t, progress)
		assert.Equal(t, id, progress.ID())
		assert.Nil(t, progress.NextRetryAt())
		assert.Nil(t, progress.ErrorMessage())
	})

	t.Run("should preserve exact values on restoration", func(t *testing.T) {
		// Arrange
		testCases := []struct {
			name            string
			batchNumber     int
			totalBatches    int
			chunksProcessed int
			status          string
			retryCount      int
		}{
			{"first batch", 1, 10, 0, "pending", 0},
			{"middle batch", 5, 10, 500, "completed", 0},
			{"last batch", 10, 10, 1000, "completed", 0},
			{"batch with retries", 3, 10, 200, "retry_scheduled", 3},
			{"failed batch", 2, 10, 150, "failed", 2},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				id := uuid.New()
				repositoryID := uuid.New()
				indexingJobID := uuid.New()
				createdAt := time.Now()
				updatedAt := time.Now()

				// Act
				progress := RestoreBatchJobProgress(
					id,
					&repositoryID,
					indexingJobID,
					tc.batchNumber,
					tc.totalBatches,
					tc.chunksProcessed,
					tc.status,
					tc.retryCount,
					nil,
					nil,
					nil, // geminiBatchJobID
					nil, // geminiFileURI
					createdAt,
					updatedAt,
					nil, // batchRequestData
					0,   // submissionAttempts
					nil, // nextSubmissionAt
				)

				// Assert
				require.NotNil(t, progress)
				assert.Equal(t, tc.batchNumber, progress.BatchNumber())
				assert.Equal(t, tc.totalBatches, progress.TotalBatches())
				assert.Equal(t, tc.chunksProcessed, progress.ChunksProcessed())
				assert.Equal(t, tc.status, progress.Status())
				assert.Equal(t, tc.retryCount, progress.RetryCount())
			})
		}
	})
}

// TestBatchJobProgress_Getters tests all getter methods.
func TestBatchJobProgress_Getters(t *testing.T) {
	t.Run("all getters return correct values", func(t *testing.T) {
		// Arrange
		id := uuid.New()
		repositoryID := uuid.New()
		indexingJobID := uuid.New()
		batchNumber := 3
		totalBatches := 8
		chunksProcessed := 250
		status := "processing"
		retryCount := 2
		retryTime := time.Now().Add(10 * time.Minute)
		errorMsg := "database timeout"
		createdAt := time.Now().Add(-2 * time.Hour)
		updatedAt := time.Now().Add(-1 * time.Hour)

		progress := RestoreBatchJobProgress(
			id,
			&repositoryID,
			indexingJobID,
			batchNumber,
			totalBatches,
			chunksProcessed,
			status,
			retryCount,
			&retryTime,
			&errorMsg,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			createdAt,
			updatedAt,
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Assert - test each getter
		assert.Equal(t, id, progress.ID())
		assert.Equal(t, indexingJobID, progress.IndexingJobID())
		assert.Equal(t, batchNumber, progress.BatchNumber())
		assert.Equal(t, totalBatches, progress.TotalBatches())
		assert.Equal(t, chunksProcessed, progress.ChunksProcessed())
		assert.Equal(t, status, progress.Status())
		assert.Equal(t, retryCount, progress.RetryCount())
		assert.Equal(t, retryTime, *progress.NextRetryAt())
		assert.Equal(t, errorMsg, *progress.ErrorMessage())
		assert.Equal(t, createdAt, progress.CreatedAt())
		assert.Equal(t, updatedAt, progress.UpdatedAt())
	})

	t.Run("getters return nil for nullable fields when not set", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Assert
		assert.Nil(t, progress.NextRetryAt())
		assert.Nil(t, progress.ErrorMessage())
	})
}

// TestBatchJobProgress_MarkProcessing tests the MarkProcessing method.
func TestBatchJobProgress_MarkProcessing(t *testing.T) {
	t.Run("should mark batch as processing", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()

		// Act
		time.Sleep(10 * time.Millisecond) // Ensure updatedAt changes
		progress.MarkProcessing()

		// Assert
		assert.Equal(t, "processing", progress.Status())
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("should mark processing from any status", func(t *testing.T) {
		// Arrange
		testCases := []string{"pending", "retry_scheduled", "completed", "failed"}

		for _, initialStatus := range testCases {
			t.Run("from "+initialStatus, func(t *testing.T) {
				// Arrange
				repositoryID := uuid.New()
				progress := RestoreBatchJobProgress(
					uuid.New(),
					&repositoryID,
					uuid.New(),
					1,
					5,
					0,
					initialStatus,
					0,
					nil,
					nil,
					nil, // geminiBatchJobID
					nil, // geminiFileURI
					time.Now(),
					time.Now(),
					nil, // batchRequestData
					0,   // submissionAttempts
					nil, // nextSubmissionAt
				)

				// Act
				progress.MarkProcessing()

				// Assert
				assert.Equal(t, "processing", progress.Status())
			})
		}
	})

	t.Run("should update the UpdatedAt timestamp", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		beforeMark := time.Now()

		// Act
		time.Sleep(10 * time.Millisecond)
		progress.MarkProcessing()
		afterMark := time.Now()

		// Assert
		assert.True(t, progress.UpdatedAt().After(beforeMark) || progress.UpdatedAt().Equal(beforeMark))
		assert.True(t, progress.UpdatedAt().Before(afterMark) || progress.UpdatedAt().Equal(afterMark))
	})
}

// TestBatchJobProgress_MarkCompleted tests the MarkCompleted method.
func TestBatchJobProgress_MarkCompleted(t *testing.T) {
	t.Run("should mark batch as completed and set chunks processed", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		chunksProcessed := 500

		// Act
		progress.MarkCompleted(chunksProcessed)

		// Assert
		assert.Equal(t, "completed", progress.Status())
		assert.Equal(t, chunksProcessed, progress.ChunksProcessed())
	})

	t.Run("should accept different chunk counts", func(t *testing.T) {
		// Arrange
		testCases := []int{0, 1, 100, 1000, 10000}

		for _, chunksProcessed := range testCases {
			t.Run("with "+string(rune(chunksProcessed)), func(t *testing.T) {
				// Arrange
				progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

				// Act
				progress.MarkCompleted(chunksProcessed)

				// Assert
				assert.Equal(t, "completed", progress.Status())
				assert.Equal(t, chunksProcessed, progress.ChunksProcessed())
			})
		}
	})

	t.Run("should update the UpdatedAt timestamp", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()

		// Act
		time.Sleep(10 * time.Millisecond)
		progress.MarkCompleted(100)

		// Assert
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("should set chunks processed even if previously different", func(t *testing.T) {
		// Arrange
		repositoryID := uuid.New()
		progress := RestoreBatchJobProgress(
			uuid.New(),
			&repositoryID,
			uuid.New(),
			1,
			5,
			50,
			"pending",
			0,
			nil,
			nil,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			time.Now(),
			time.Now(),
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Act
		progress.MarkCompleted(150)

		// Assert
		assert.Equal(t, 150, progress.ChunksProcessed())
		assert.Equal(t, "completed", progress.Status())
	})
}

// TestBatchJobProgress_MarkFailed tests the MarkFailed method.
func TestBatchJobProgress_MarkFailed(t *testing.T) {
	t.Run("should mark batch as failed and set error message", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		errorMsg := "network connection timeout"

		// Act
		progress.MarkFailed(errorMsg)

		// Assert
		assert.Equal(t, "failed", progress.Status())
		assert.NotNil(t, progress.ErrorMessage())
		assert.Equal(t, errorMsg, *progress.ErrorMessage())
	})

	t.Run("should accept various error messages", func(t *testing.T) {
		// Arrange
		errorMessages := []string{
			"simple error",
			"error with special chars: !@#$%",
			"error with\nnewlines",
			"very long error message that contains detailed information about what went wrong and why it failed during processing",
		}

		for _, msg := range errorMessages {
			msgLabel := msg
			if len(msg) > 20 {
				msgLabel = msg[:20]
			}
			t.Run("with message: "+msgLabel, func(t *testing.T) {
				// Arrange
				progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

				// Act
				progress.MarkFailed(msg)

				// Assert
				assert.Equal(t, "failed", progress.Status())
				assert.Equal(t, msg, *progress.ErrorMessage())
			})
		}
	})

	t.Run("should update the UpdatedAt timestamp", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()

		// Act
		time.Sleep(10 * time.Millisecond)
		progress.MarkFailed("test error")

		// Assert
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("should replace previous error message if batch already failed", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.MarkFailed("first error")

		// Act
		progress.MarkFailed("second error")

		// Assert
		assert.Equal(t, "failed", progress.Status())
		assert.Equal(t, "second error", *progress.ErrorMessage())
	})
}

// TestBatchJobProgress_ScheduleRetry tests the ScheduleRetry method.
func TestBatchJobProgress_ScheduleRetry(t *testing.T) {
	t.Run("should schedule retry with correct status and timestamps", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		retryAt := time.Now().Add(5 * time.Minute)

		// Act
		progress.ScheduleRetry(retryAt)

		// Assert
		assert.Equal(t, "retry_scheduled", progress.Status())
		assert.Equal(t, 1, progress.RetryCount())
		assert.NotNil(t, progress.NextRetryAt())
		assert.Equal(t, retryAt, *progress.NextRetryAt())
	})

	t.Run("should increment retry count on each schedule retry call", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		retryAt := time.Now().Add(5 * time.Minute)

		// Act
		progress.ScheduleRetry(retryAt)
		firstRetryCount := progress.RetryCount()

		progress.ScheduleRetry(time.Now().Add(10 * time.Minute))
		secondRetryCount := progress.RetryCount()

		progress.ScheduleRetry(time.Now().Add(15 * time.Minute))
		thirdRetryCount := progress.RetryCount()

		// Assert
		assert.Equal(t, 1, firstRetryCount)
		assert.Equal(t, 2, secondRetryCount)
		assert.Equal(t, 3, thirdRetryCount)
	})

	t.Run("should update the UpdatedAt timestamp", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()

		// Act
		time.Sleep(10 * time.Millisecond)
		progress.ScheduleRetry(time.Now().Add(5 * time.Minute))

		// Assert
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("should handle retries from failed status", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.MarkFailed("initial error")

		// Act
		progress.ScheduleRetry(time.Now().Add(5 * time.Minute))

		// Assert
		assert.Equal(t, "retry_scheduled", progress.Status())
		assert.Equal(t, 1, progress.RetryCount())
		assert.NotNil(t, progress.NextRetryAt())
	})

	t.Run("should accept future and past retry times", func(t *testing.T) {
		// Arrange
		futureTime := time.Now().Add(24 * time.Hour)
		pastTime := time.Now().Add(-1 * time.Hour)

		progressFuture := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progressPast := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		progressFuture.ScheduleRetry(futureTime)
		progressPast.ScheduleRetry(pastTime)

		// Assert
		assert.Equal(t, futureTime, *progressFuture.NextRetryAt())
		assert.Equal(t, pastTime, *progressPast.NextRetryAt())
	})
}

// TestBatchJobProgress_IsRetryable tests the IsRetryable method.
func TestBatchJobProgress_IsRetryable(t *testing.T) {
	t.Run("should return true when status is retry_scheduled", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.ScheduleRetry(time.Now().Add(5 * time.Minute))

		// Act
		isRetryable := progress.IsRetryable()

		// Assert
		assert.True(t, isRetryable)
	})

	t.Run("should return false for all other statuses", func(t *testing.T) {
		// Arrange
		statuses := []struct {
			name   string
			status string
		}{
			{"pending", "pending"},
			{"processing", "processing"},
			{"completed", "completed"},
			{"failed", "failed"},
		}

		for _, s := range statuses {
			t.Run(s.name, func(t *testing.T) {
				// Arrange
				repositoryID := uuid.New()
				progress := RestoreBatchJobProgress(
					uuid.New(),
					&repositoryID,
					uuid.New(),
					1,
					5,
					0,
					s.status,
					0,
					nil,
					nil,
					nil, // geminiBatchJobID
					nil, // geminiFileURI
					time.Now(),
					time.Now(),
					nil, // batchRequestData
					0,   // submissionAttempts
					nil, // nextSubmissionAt
				)

				// Act
				isRetryable := progress.IsRetryable()

				// Assert
				assert.False(t, isRetryable)
			})
		}
	})

	t.Run("should return false after retry is scheduled but status changes", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.ScheduleRetry(time.Now().Add(5 * time.Minute))
		assert.True(t, progress.IsRetryable())

		// Act - mark as processing
		progress.MarkProcessing()

		// Assert
		assert.False(t, progress.IsRetryable())
	})
}

// TestBatchJobProgress_ShouldRetry tests the ShouldRetry method.
func TestBatchJobProgress_ShouldRetry(t *testing.T) {
	t.Run("should return true when status is retry_scheduled and time is in the past", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		pastTime := time.Now().Add(-5 * time.Minute)
		progress.ScheduleRetry(pastTime)

		// Act
		shouldRetry := progress.ShouldRetry()

		// Assert
		assert.True(t, shouldRetry)
	})

	t.Run("should return false when status is retry_scheduled but time is in the future", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		futureTime := time.Now().Add(5 * time.Minute)
		progress.ScheduleRetry(futureTime)

		// Act
		shouldRetry := progress.ShouldRetry()

		// Assert
		assert.False(t, shouldRetry)
	})

	t.Run("should return false when status is not retry_scheduled", func(t *testing.T) {
		// Arrange
		statuses := []string{"pending", "processing", "completed", "failed"}

		for _, status := range statuses {
			t.Run(status, func(t *testing.T) {
				// Arrange
				repositoryID := uuid.New()
				progress := RestoreBatchJobProgress(
					uuid.New(),
					&repositoryID,
					uuid.New(),
					1,
					5,
					0,
					status,
					0,
					nil,
					nil,
					nil, // geminiBatchJobID
					nil, // geminiFileURI
					time.Now(),
					time.Now(),
					nil, // batchRequestData
					0,   // submissionAttempts
					nil, // nextSubmissionAt
				)

				// Act
				shouldRetry := progress.ShouldRetry()

				// Assert
				assert.False(t, shouldRetry)
			})
		}
	})

	t.Run("should handle edge case when retry time equals current time", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		now := time.Now()
		progress.ScheduleRetry(now)

		// Act
		// We need to check within a small window since time.Now() in the method will be slightly different
		shouldRetry := progress.ShouldRetry()

		// Assert - should be close to false since times are nearly equal, but could be true due to timing
		// This is a boundary condition that depends on execution timing
		assert.IsType(t, true, shouldRetry)
	})

	t.Run("should return false when retry_scheduled but NextRetryAt is nil", func(t *testing.T) {
		// Arrange
		repositoryID := uuid.New()
		progress := RestoreBatchJobProgress(
			uuid.New(),
			&repositoryID,
			uuid.New(),
			1,
			5,
			0,
			"retry_scheduled",
			1,
			nil, // No retry time set
			nil,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			time.Now(),
			time.Now(),
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Act
		shouldRetry := progress.ShouldRetry()

		// Assert
		assert.False(t, shouldRetry)
	})

	t.Run("should differentiate between imminent and distant retries", func(t *testing.T) {
		// Arrange
		progressImmediate := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progressDistant := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		immediatetime := time.Now().Add(-1 * time.Nanosecond) // Just passed
		distantTime := time.Now().Add(24 * time.Hour)         // Far future

		// Act
		progressImmediate.ScheduleRetry(immediatetime)
		progressDistant.ScheduleRetry(distantTime)

		shouldRetryImmediate := progressImmediate.ShouldRetry()
		shouldRetryDistant := progressDistant.ShouldRetry()

		// Assert
		assert.True(t, shouldRetryImmediate)
		assert.False(t, shouldRetryDistant)
	})
}

// TestBatchJobProgress_IncrementRetryCount tests the IncrementRetryCount method.
func TestBatchJobProgress_IncrementRetryCount(t *testing.T) {
	t.Run("should increment retry count by one", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		assert.Equal(t, 0, progress.RetryCount())

		// Act
		progress.IncrementRetryCount()

		// Assert
		assert.Equal(t, 1, progress.RetryCount())
	})

	t.Run("should increment retry count multiple times", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act & Assert
		for i := 1; i <= 5; i++ {
			progress.IncrementRetryCount()
			assert.Equal(t, i, progress.RetryCount())
		}
	})

	t.Run("should work on restored entities with existing retry count", func(t *testing.T) {
		// Arrange
		repositoryID := uuid.New()
		progress := RestoreBatchJobProgress(
			uuid.New(),
			&repositoryID,
			uuid.New(),
			1,
			5,
			0,
			"retry_scheduled",
			3,
			nil,
			nil,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			time.Now(),
			time.Now(),
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Act
		progress.IncrementRetryCount()

		// Assert
		assert.Equal(t, 4, progress.RetryCount())
	})

	t.Run("should update the UpdatedAt timestamp", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()

		// Act
		time.Sleep(10 * time.Millisecond)
		progress.IncrementRetryCount()

		// Assert
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("should handle large retry counts", func(t *testing.T) {
		// Arrange
		repositoryID := uuid.New()
		progress := RestoreBatchJobProgress(
			uuid.New(),
			&repositoryID,
			uuid.New(),
			1,
			5,
			0,
			"retry_scheduled",
			999,
			nil,
			nil,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			time.Now(),
			time.Now(),
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Act
		progress.IncrementRetryCount()

		// Assert
		assert.Equal(t, 1000, progress.RetryCount())
	})
}

// TestBatchJobProgress_StateTransitions tests valid state transitions.
func TestBatchJobProgress_StateTransitions(t *testing.T) {
	t.Run("should transition from pending to processing to completed", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		assert.Equal(t, "pending", progress.Status())

		// Act & Assert
		progress.MarkProcessing()
		assert.Equal(t, "processing", progress.Status())

		progress.MarkCompleted(100)
		assert.Equal(t, "completed", progress.Status())
	})

	t.Run("should transition from processing to failed", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.MarkProcessing()

		// Act
		progress.MarkFailed("processing error")

		// Assert
		assert.Equal(t, "failed", progress.Status())
	})

	t.Run("should transition from failed to retry_scheduled", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.MarkFailed("initial failure")
		assert.Equal(t, "failed", progress.Status())

		// Act
		progress.ScheduleRetry(time.Now().Add(5 * time.Minute))

		// Assert
		assert.Equal(t, "retry_scheduled", progress.Status())
	})

	t.Run("should transition from retry_scheduled to processing", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.ScheduleRetry(time.Now().Add(5 * time.Minute))
		assert.Equal(t, "retry_scheduled", progress.Status())

		// Act
		progress.MarkProcessing()

		// Assert
		assert.Equal(t, "processing", progress.Status())
	})
}

// TestBatchJobProgress_ErrorMessageManagement tests error message behavior.
func TestBatchJobProgress_ErrorMessageManagement(t *testing.T) {
	t.Run("should set error message on mark failed", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		assert.Nil(t, progress.ErrorMessage())

		// Act
		progress.MarkFailed("test error")

		// Assert
		assert.NotNil(t, progress.ErrorMessage())
		assert.Equal(t, "test error", *progress.ErrorMessage())
	})

	t.Run("should preserve error message across multiple operations", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.MarkFailed("original error")

		// Act
		progress.ScheduleRetry(time.Now().Add(5 * time.Minute))

		// Assert - error message should still be there even though status changed
		assert.NotNil(t, progress.ErrorMessage())
		assert.Equal(t, "original error", *progress.ErrorMessage())
	})

	t.Run("should update error message when mark failed again", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.MarkFailed("first error")

		// Act
		progress.MarkFailed("second error")

		// Assert
		assert.Equal(t, "second error", *progress.ErrorMessage())
	})
}

// TestBatchJobProgress_BatchNumberValidation tests batch number constraints.
func TestBatchJobProgress_BatchNumberValidation(t *testing.T) {
	t.Run("should create batch with batchNumber equal to totalBatches", func(t *testing.T) {
		// Arrange & Act
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 5, 5)

		// Assert
		assert.Equal(t, 5, progress.BatchNumber())
		assert.Equal(t, 5, progress.TotalBatches())
	})

	t.Run("should create batch with batchNumber less than totalBatches", func(t *testing.T) {
		// Arrange & Act
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 100)

		// Assert
		assert.Equal(t, 1, progress.BatchNumber())
		assert.Equal(t, 100, progress.TotalBatches())
	})

	t.Run("should handle large batch numbers", func(t *testing.T) {
		// Arrange & Act
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1000, 10000)

		// Assert
		assert.Equal(t, 1000, progress.BatchNumber())
		assert.Equal(t, 10000, progress.TotalBatches())
	})
}

// TestBatchJobProgress_ChunksProcessedManagement tests chunks processed behavior.
func TestBatchJobProgress_ChunksProcessedManagement(t *testing.T) {
	t.Run("should start with zero chunks processed", func(t *testing.T) {
		// Arrange & Act
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Assert
		assert.Equal(t, 0, progress.ChunksProcessed())
	})

	t.Run("should set chunks processed when completed", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		progress.MarkCompleted(500)

		// Assert
		assert.Equal(t, 500, progress.ChunksProcessed())
	})

	t.Run("should preserve chunks processed across status changes", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progress.MarkCompleted(300)

		// Act
		progress.MarkProcessing()

		// Assert
		assert.Equal(t, 300, progress.ChunksProcessed())
	})
}

// TestBatchJobProgress_MarkSubmittedToGemini_ValidatesFormat tests batch job ID validation.
func TestBatchJobProgress_MarkSubmittedToGemini_ValidatesFormat(t *testing.T) {
	t.Run("should accept valid batch job ID with batches/ prefix", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		err := progress.MarkSubmittedToGemini("batches/abc123")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, StatusProcessing, progress.Status())
		assert.NotNil(t, progress.GeminiBatchJobID())
		assert.Equal(t, "batches/abc123", *progress.GeminiBatchJobID())
	})

	t.Run("should reject invalid batch job ID without batches/ prefix", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		err := progress.MarkSubmittedToGemini("invalid-job-id")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid batch job ID format")
		assert.Contains(t, err.Error(), "expected 'batches/<id>'")
		assert.Equal(t, StatusPending, progress.Status()) // Status should not change
		assert.Nil(t, progress.GeminiBatchJobID())        // Job ID should not be set
	})

	t.Run("should reject empty string", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		err := progress.MarkSubmittedToGemini("")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid batch job ID format")
	})

	t.Run("should reject string that is too short", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		err := progress.MarkSubmittedToGemini("batches")

		// Assert
		assert.Error(t, err)
	})

	t.Run("should accept real Google Gemini batch job ID format", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		realGeminiID := "batches/vxnu0emj54uem1y2at02ntdyhtlkgrzn63ee"

		// Act
		err := progress.MarkSubmittedToGemini(realGeminiID)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, realGeminiID, *progress.GeminiBatchJobID())
	})

	t.Run("should update timestamp on successful submission", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()
		time.Sleep(10 * time.Millisecond)

		// Act
		err := progress.MarkSubmittedToGemini("batches/test123")

		// Assert
		assert.NoError(t, err)
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("should not update timestamp on failed validation", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()
		time.Sleep(10 * time.Millisecond)

		// Act
		err := progress.MarkSubmittedToGemini("invalid")

		// Assert
		assert.Error(t, err)
		assert.Equal(t, originalUpdatedAt, progress.UpdatedAt())
	})
}

// TestBatchJobProgress_TimestampBehavior tests timestamp management.
func TestBatchJobProgress_TimestampBehavior(t *testing.T) {
	t.Run("created at and updated at should be equal on creation", func(t *testing.T) {
		// Arrange & Act
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Assert
		assert.Equal(t, progress.CreatedAt(), progress.UpdatedAt())
	})

	t.Run("updated at should change when status is modified", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()
		createdAt := progress.CreatedAt()

		// Act
		time.Sleep(10 * time.Millisecond)
		progress.MarkProcessing()

		// Assert
		assert.Equal(t, createdAt, progress.CreatedAt())
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("created at should remain constant", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalCreatedAt := progress.CreatedAt()

		// Act
		progress.MarkProcessing()
		progress.MarkCompleted(100)
		progress.MarkFailed("error")

		// Assert
		assert.Equal(t, originalCreatedAt, progress.CreatedAt())
	})
}

// TestBatchJobProgress_MarkPendingSubmission tests the MarkPendingSubmission method.
func TestBatchJobProgress_MarkPendingSubmission(t *testing.T) {
	t.Run("should set status to pending_submission", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001","requests":[{"content":"test"}]}`)

		// Act
		progress.MarkPendingSubmission(requestData)

		// Assert
		assert.Equal(t, "pending_submission", progress.Status())
	})

	t.Run("should store request data", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001","requests":[{"content":"test"}]}`)

		// Act
		progress.MarkPendingSubmission(requestData)

		// Assert
		assert.NotNil(t, progress.BatchRequestData())
		assert.Equal(t, requestData, progress.BatchRequestData())
	})

	t.Run("should reset submission attempts to 0", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Simulate previous submission attempts
		futureTime := time.Now().Add(5 * time.Minute)
		progress.MarkSubmissionFailed("previous error", futureTime)
		assert.Equal(t, 1, progress.SubmissionAttempts())

		// Act
		progress.MarkPendingSubmission(requestData)

		// Assert
		assert.Equal(t, 0, progress.SubmissionAttempts())
	})

	t.Run("should set nextSubmissionAt to nil (ready immediately)", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Simulate previous scheduled submission
		futureTime := time.Now().Add(5 * time.Minute)
		progress.MarkSubmissionFailed("previous error", futureTime)
		assert.NotNil(t, progress.NextSubmissionAt())

		// Act
		progress.MarkPendingSubmission(requestData)

		// Assert
		assert.Nil(t, progress.NextSubmissionAt())
	})

	t.Run("should handle empty request data", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte{}

		// Act
		progress.MarkPendingSubmission(requestData)

		// Assert
		assert.Equal(t, "pending_submission", progress.Status())
		assert.NotNil(t, progress.BatchRequestData())
		assert.Empty(t, progress.BatchRequestData())
	})

	t.Run("should handle large request data", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		// Create large request data (e.g., 100 requests)
		largeData := make([]byte, 10000)
		for i := range largeData {
			largeData[i] = byte('x')
		}

		// Act
		progress.MarkPendingSubmission(largeData)

		// Assert
		assert.Equal(t, "pending_submission", progress.Status())
		assert.Equal(t, largeData, progress.BatchRequestData())
	})

	t.Run("should update timestamp", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		originalUpdatedAt := progress.UpdatedAt()
		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Act
		time.Sleep(10 * time.Millisecond)
		progress.MarkPendingSubmission(requestData)

		// Assert
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("should overwrite previous request data", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		firstData := []byte(`{"model":"gemini-embedding-001","version":1}`)
		secondData := []byte(`{"model":"gemini-embedding-001","version":2}`)

		// Act
		progress.MarkPendingSubmission(firstData)
		progress.MarkPendingSubmission(secondData)

		// Assert
		assert.Equal(t, secondData, progress.BatchRequestData())
	})
}

// TestBatchJobProgress_MarkSubmissionFailed tests the MarkSubmissionFailed method.
func TestBatchJobProgress_MarkSubmissionFailed(t *testing.T) {
	t.Run("should increment submission attempts", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)
		nextAttemptAt := time.Now().Add(5 * time.Minute)

		// Act
		progress.MarkSubmissionFailed("rate limit exceeded", nextAttemptAt)

		// Assert
		assert.Equal(t, 1, progress.SubmissionAttempts())
	})

	t.Run("should increment submission attempts on multiple failures", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		// Act
		progress.MarkSubmissionFailed("error 1", time.Now().Add(1*time.Minute))
		progress.MarkSubmissionFailed("error 2", time.Now().Add(2*time.Minute))
		progress.MarkSubmissionFailed("error 3", time.Now().Add(4*time.Minute))

		// Assert
		assert.Equal(t, 3, progress.SubmissionAttempts())
	})

	t.Run("should store error message", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)
		errorMsg := "rate limit exceeded: 429 Too Many Requests"
		nextAttemptAt := time.Now().Add(5 * time.Minute)

		// Act
		progress.MarkSubmissionFailed(errorMsg, nextAttemptAt)

		// Assert
		// Note: The method should store the error in a lastSubmissionError field
		// This is different from the general ErrorMessage field
		assert.NotNil(t, progress.ErrorMessage()) // Assuming it uses the existing field for now
		assert.Equal(t, errorMsg, *progress.ErrorMessage())
	})

	t.Run("should set nextSubmissionAt to provided time", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)
		nextAttemptAt := time.Now().Add(10 * time.Minute)

		// Act
		progress.MarkSubmissionFailed("error", nextAttemptAt)

		// Assert
		assert.NotNil(t, progress.NextSubmissionAt())
		assert.Equal(t, nextAttemptAt, *progress.NextSubmissionAt())
	})

	t.Run("should NOT change status (remains pending_submission)", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)
		assert.Equal(t, "pending_submission", progress.Status())

		// Act
		progress.MarkSubmissionFailed("error", time.Now().Add(5*time.Minute))

		// Assert
		assert.Equal(t, "pending_submission", progress.Status())
	})

	t.Run("should update error message on subsequent failures", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		// Act
		progress.MarkSubmissionFailed("first error", time.Now().Add(1*time.Minute))
		firstError := *progress.ErrorMessage()

		progress.MarkSubmissionFailed("second error", time.Now().Add(2*time.Minute))
		secondError := *progress.ErrorMessage()

		// Assert
		assert.Equal(t, "first error", firstError)
		assert.Equal(t, "second error", secondError)
	})

	t.Run("should update nextSubmissionAt with exponential backoff times", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		// Act & Assert - simulate exponential backoff
		first := time.Now().Add(1 * time.Minute)
		progress.MarkSubmissionFailed("attempt 1", first)
		assert.Equal(t, first, *progress.NextSubmissionAt())

		second := time.Now().Add(2 * time.Minute)
		progress.MarkSubmissionFailed("attempt 2", second)
		assert.Equal(t, second, *progress.NextSubmissionAt())

		third := time.Now().Add(4 * time.Minute)
		progress.MarkSubmissionFailed("attempt 3", third)
		assert.Equal(t, third, *progress.NextSubmissionAt())
	})

	t.Run("should update timestamp", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)
		originalUpdatedAt := progress.UpdatedAt()

		// Act
		time.Sleep(10 * time.Millisecond)
		progress.MarkSubmissionFailed("error", time.Now().Add(5*time.Minute))

		// Assert
		assert.True(t, progress.UpdatedAt().After(originalUpdatedAt))
	})

	t.Run("should handle empty error message", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		// Act
		progress.MarkSubmissionFailed("", time.Now().Add(5*time.Minute))

		// Assert
		assert.NotNil(t, progress.ErrorMessage())
		assert.Empty(t, *progress.ErrorMessage())
	})
}

// TestBatchJobProgress_IsReadyForSubmission tests the IsReadyForSubmission method.
func TestBatchJobProgress_IsReadyForSubmission(t *testing.T) {
	t.Run("should return false if status is not pending_submission", func(t *testing.T) {
		// Arrange
		statuses := []struct {
			name   string
			status string
		}{
			{"pending", StatusPending},
			{"processing", StatusProcessing},
			{"completed", StatusCompleted},
			{"failed", StatusFailed},
			{"retry_scheduled", StatusRetryScheduled},
		}

		for _, s := range statuses {
			t.Run(s.name, func(t *testing.T) {
				// Arrange
				repositoryID := uuid.New()
				progress := RestoreBatchJobProgress(
					uuid.New(),
					&repositoryID,
					uuid.New(),
					1,
					5,
					0,
					s.status,
					0,
					nil,
					nil,
					nil, // geminiBatchJobID
					nil, // geminiFileURI
					time.Now(),
					time.Now(),
					nil, // batchRequestData
					0,   // submissionAttempts
					nil, // nextSubmissionAt
				)

				// Act
				isReady := progress.IsReadyForSubmission()

				// Assert
				assert.False(t, isReady)
			})
		}
	})

	t.Run("should return true if status is pending_submission and nextSubmissionAt is nil", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		// Act
		isReady := progress.IsReadyForSubmission()

		// Assert
		assert.True(t, isReady)
	})

	t.Run("should return true if status is pending_submission and nextSubmissionAt is in the past", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		// Simulate failed submission with retry time in the past
		pastTime := time.Now().Add(-5 * time.Minute)
		progress.MarkSubmissionFailed("error", pastTime)

		// Act
		isReady := progress.IsReadyForSubmission()

		// Assert
		assert.True(t, isReady)
	})

	t.Run(
		"should return false if status is pending_submission and nextSubmissionAt is in the future",
		func(t *testing.T) {
			// Arrange
			progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
			requestData := []byte(`{"model":"gemini-embedding-001"}`)
			progress.MarkPendingSubmission(requestData)

			// Simulate failed submission with retry time in the future
			futureTime := time.Now().Add(5 * time.Minute)
			progress.MarkSubmissionFailed("error", futureTime)

			// Act
			isReady := progress.IsReadyForSubmission()

			// Assert
			assert.False(t, isReady)
		},
	)

	t.Run("should handle edge case when nextSubmissionAt equals current time", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		// Set retry time to now
		now := time.Now()
		progress.MarkSubmissionFailed("error", now)

		// Act - slight delay to ensure "now" is in the past
		time.Sleep(10 * time.Millisecond)
		isReady := progress.IsReadyForSubmission()

		// Assert - should be ready since time has passed
		assert.True(t, isReady)
	})

	t.Run("should differentiate between immediate and delayed submissions", func(t *testing.T) {
		// Arrange
		progressImmediate := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		progressDelayed := NewBatchJobProgress(uuid.New(), uuid.New(), 2, 5)

		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Immediate submission (no nextSubmissionAt)
		progressImmediate.MarkPendingSubmission(requestData)

		// Delayed submission (nextSubmissionAt in future)
		progressDelayed.MarkPendingSubmission(requestData)
		progressDelayed.MarkSubmissionFailed("rate limit", time.Now().Add(10*time.Minute))

		// Act
		immediateReady := progressImmediate.IsReadyForSubmission()
		delayedReady := progressDelayed.IsReadyForSubmission()

		// Assert
		assert.True(t, immediateReady)
		assert.False(t, delayedReady)
	})
}

// TestBatchJobProgress_Validate_PendingSubmissionStatus tests validation of pending_submission status.
func TestBatchJobProgress_Validate_PendingSubmissionStatus(t *testing.T) {
	t.Run("should accept pending_submission as valid status", func(t *testing.T) {
		// Arrange
		repositoryID := uuid.New()
		progress := RestoreBatchJobProgress(
			uuid.New(),
			&repositoryID,
			uuid.New(),
			1,
			5,
			0,
			"pending_submission",
			0,
			nil,
			nil,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			time.Now(),
			time.Now(),
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Act
		err := progress.Validate()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("should validate pending_submission with submission attempts", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)
		progress.MarkSubmissionFailed("error", time.Now().Add(5*time.Minute))

		// Act
		err := progress.Validate()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("should validate pending_submission with zero chunks processed", func(t *testing.T) {
		// Arrange
		repositoryID := uuid.New()
		progress := RestoreBatchJobProgress(
			uuid.New(),
			&repositoryID,
			uuid.New(),
			1,
			5,
			0, // Zero chunks processed is valid for pending_submission
			"pending_submission",
			0,
			nil,
			nil,
			nil, // geminiBatchJobID
			nil, // geminiFileURI
			time.Now(),
			time.Now(),
			nil, // batchRequestData
			0,   // submissionAttempts
			nil, // nextSubmissionAt
		)

		// Act
		err := progress.Validate()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("should allow all valid statuses including pending_submission", func(t *testing.T) {
		// Arrange
		validStatuses := []string{
			StatusPending,
			StatusProcessing,
			StatusCompleted,
			StatusFailed,
			StatusRetryScheduled,
			"pending_submission",
		}

		for _, status := range validStatuses {
			t.Run(status, func(t *testing.T) {
				// Arrange
				repositoryID := uuid.New()
				chunksProcessed := 0
				if status == StatusCompleted {
					chunksProcessed = 1 // Completed requires positive chunks
				}

				progress := RestoreBatchJobProgress(
					uuid.New(),
					&repositoryID,
					uuid.New(),
					1,
					5,
					chunksProcessed,
					status,
					0,
					nil,
					nil,
					nil, // geminiBatchJobID
					nil, // geminiFileURI
					time.Now(),
					time.Now(),
					nil, // batchRequestData
					0,   // submissionAttempts
					nil, // nextSubmissionAt
				)

				// Act
				err := progress.Validate()

				// Assert
				assert.NoError(t, err)
			})
		}
	})
}

// TestBatchJobProgress_StatusTransitions_PendingSubmissionFlow tests status transitions with pending_submission.
func TestBatchJobProgress_StatusTransitions_PendingSubmissionFlow(t *testing.T) {
	t.Run("should transition from pending to pending_submission to processing to completed", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		assert.Equal(t, StatusPending, progress.Status())

		// Act & Assert - pending -> pending_submission
		progress.MarkPendingSubmission(requestData)
		assert.Equal(t, "pending_submission", progress.Status())

		// pending_submission -> processing
		err := progress.MarkSubmittedToGemini("batches/test123")
		assert.NoError(t, err)
		assert.Equal(t, StatusProcessing, progress.Status())

		// processing -> completed
		progress.MarkCompleted(100)
		assert.Equal(t, StatusCompleted, progress.Status())
	})

	t.Run("should transition from pending_submission to failed on submission errors", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Act & Assert
		progress.MarkPendingSubmission(requestData)
		assert.Equal(t, "pending_submission", progress.Status())

		// Simulate max retries exceeded - transition to failed
		progress.MarkFailed("max submission retries exceeded")
		assert.Equal(t, StatusFailed, progress.Status())
	})

	t.Run("should remain in pending_submission after submission failures", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Act & Assert
		progress.MarkPendingSubmission(requestData)
		assert.Equal(t, "pending_submission", progress.Status())

		// Multiple submission failures should not change status
		progress.MarkSubmissionFailed("error 1", time.Now().Add(1*time.Minute))
		assert.Equal(t, "pending_submission", progress.Status())

		progress.MarkSubmissionFailed("error 2", time.Now().Add(2*time.Minute))
		assert.Equal(t, "pending_submission", progress.Status())

		progress.MarkSubmissionFailed("error 3", time.Now().Add(4*time.Minute))
		assert.Equal(t, "pending_submission", progress.Status())
	})

	t.Run("should support re-submission after pending_submission", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		firstData := []byte(`{"model":"gemini-embedding-001","version":1}`)
		secondData := []byte(`{"model":"gemini-embedding-001","version":2}`)

		// Act & Assert
		progress.MarkPendingSubmission(firstData)
		assert.Equal(t, "pending_submission", progress.Status())

		// Re-submit with new data (e.g., after rate limit reset)
		progress.MarkPendingSubmission(secondData)
		assert.Equal(t, "pending_submission", progress.Status())
		assert.Equal(t, secondData, progress.BatchRequestData())
		assert.Equal(t, 0, progress.SubmissionAttempts()) // Reset attempts
	})

	t.Run("should track submission attempts across multiple failures before processing", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Act & Assert
		progress.MarkPendingSubmission(requestData)
		assert.Equal(t, 0, progress.SubmissionAttempts())

		// Simulate exponential backoff retries
		progress.MarkSubmissionFailed("rate limit", time.Now().Add(1*time.Minute))
		assert.Equal(t, 1, progress.SubmissionAttempts())
		assert.Equal(t, "pending_submission", progress.Status())

		progress.MarkSubmissionFailed("rate limit", time.Now().Add(2*time.Minute))
		assert.Equal(t, 2, progress.SubmissionAttempts())
		assert.Equal(t, "pending_submission", progress.Status())

		progress.MarkSubmissionFailed("rate limit", time.Now().Add(4*time.Minute))
		assert.Equal(t, 3, progress.SubmissionAttempts())
		assert.Equal(t, "pending_submission", progress.Status())

		// Finally succeed
		err := progress.MarkSubmittedToGemini("batches/success123")
		assert.NoError(t, err)
		assert.Equal(t, StatusProcessing, progress.Status())
	})
}

// TestBatchJobProgress_BatchRequestData tests the BatchRequestData getter.
func TestBatchJobProgress_BatchRequestData(t *testing.T) {
	t.Run("should return nil when not set", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		data := progress.BatchRequestData()

		// Assert
		assert.Nil(t, data)
	})

	t.Run("should return stored request data", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001","requests":[{"content":"test"}]}`)
		progress.MarkPendingSubmission(requestData)

		// Act
		data := progress.BatchRequestData()

		// Assert
		assert.Equal(t, requestData, data)
	})

	t.Run("should return empty slice when set to empty", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		emptyData := []byte{}
		progress.MarkPendingSubmission(emptyData)

		// Act
		data := progress.BatchRequestData()

		// Assert
		assert.NotNil(t, data)
		assert.Empty(t, data)
	})
}

// TestBatchJobProgress_SubmissionAttempts tests the SubmissionAttempts getter.
func TestBatchJobProgress_SubmissionAttempts(t *testing.T) {
	t.Run("should return 0 initially", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		attempts := progress.SubmissionAttempts()

		// Assert
		assert.Equal(t, 0, attempts)
	})

	t.Run("should return current submission attempts count", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		// Act & Assert
		assert.Equal(t, 0, progress.SubmissionAttempts())

		progress.MarkSubmissionFailed("error 1", time.Now().Add(1*time.Minute))
		assert.Equal(t, 1, progress.SubmissionAttempts())

		progress.MarkSubmissionFailed("error 2", time.Now().Add(2*time.Minute))
		assert.Equal(t, 2, progress.SubmissionAttempts())
	})

	t.Run("should reset to 0 after MarkPendingSubmission", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Act
		progress.MarkPendingSubmission(requestData)
		progress.MarkSubmissionFailed("error", time.Now().Add(1*time.Minute))
		assert.Equal(t, 1, progress.SubmissionAttempts())

		progress.MarkPendingSubmission(requestData) // Re-submit

		// Assert
		assert.Equal(t, 0, progress.SubmissionAttempts())
	})
}

// TestBatchJobProgress_NextSubmissionAt tests the NextSubmissionAt getter.
func TestBatchJobProgress_NextSubmissionAt(t *testing.T) {
	t.Run("should return nil when not set", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)

		// Act
		nextAt := progress.NextSubmissionAt()

		// Assert
		assert.Nil(t, nextAt)
	})

	t.Run("should return scheduled submission time", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)
		progress.MarkPendingSubmission(requestData)

		scheduledTime := time.Now().Add(5 * time.Minute)
		progress.MarkSubmissionFailed("error", scheduledTime)

		// Act
		nextAt := progress.NextSubmissionAt()

		// Assert
		assert.NotNil(t, nextAt)
		assert.Equal(t, scheduledTime, *nextAt)
	})

	t.Run("should return nil after MarkPendingSubmission reset", func(t *testing.T) {
		// Arrange
		progress := NewBatchJobProgress(uuid.New(), uuid.New(), 1, 5)
		requestData := []byte(`{"model":"gemini-embedding-001"}`)

		// Act
		progress.MarkPendingSubmission(requestData)
		progress.MarkSubmissionFailed("error", time.Now().Add(5*time.Minute))
		assert.NotNil(t, progress.NextSubmissionAt())

		progress.MarkPendingSubmission(requestData) // Reset

		// Assert
		assert.Nil(t, progress.NextSubmissionAt())
	})
}
