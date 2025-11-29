//go:build integration

package repository

import (
	"codechunking/internal/domain/entity"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// createTestBatchJobProgress creates a test batch job progress entity.
func createTestBatchJobProgress(
	_ *testing.T,
	repositoryID, indexingJobID uuid.UUID,
	batchNumber, totalBatches int,
) *entity.BatchJobProgress {
	return entity.NewBatchJobProgress(repositoryID, indexingJobID, batchNumber, totalBatches)
}

// createTestBatchJobProgressWithDefaults creates a test batch job progress entity with generated IDs.
func createTestBatchJobProgressWithDefaults(
	_ *testing.T,
	batchNumber, totalBatches int,
) *entity.BatchJobProgress {
	return entity.NewBatchJobProgress(uuid.New(), uuid.New(), batchNumber, totalBatches)
}

// setupBatchProgressTest creates a test database and indexing job for batch progress testing.
func setupBatchProgressTest(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID, context.Context) {
	pool := setupTestDB(t)
	ctx := context.Background()

	// Create a test repository first (needed for foreign key)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	// Create and save an indexing job
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	testJob := createTestIndexingJob(t, testRepo.ID())
	err = jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save test indexing job: %v", err)
	}

	return pool, testRepo.ID(), testJob.ID(), ctx
}

// TestBatchProgressRepository_Save_SuccessfulSave tests saving a new batch progress successfully.
func TestBatchProgressRepository_Save_SuccessfulSave(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create test batch progress
	batchProgress := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)

	// Save the batch progress
	err := batchRepo.Save(ctx, batchProgress)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify the batch was saved by retrieving it
	saved, err := batchRepo.GetByID(ctx, batchProgress.ID())
	if err != nil {
		t.Errorf("Expected no error retrieving saved batch but got: %v", err)
	}

	if saved == nil {
		t.Error("Expected saved batch to be retrieved but got nil")
	}

	// Verify batch details match
	if saved.ID() != batchProgress.ID() {
		t.Errorf("Expected batch ID %s, got %s", batchProgress.ID(), saved.ID())
	}

	if saved.IndexingJobID() != indexingJobID {
		t.Errorf("Expected indexing job ID %s, got %s", indexingJobID, saved.IndexingJobID())
	}

	if saved.BatchNumber() != 1 {
		t.Errorf("Expected batch number 1, got %d", saved.BatchNumber())
	}

	if saved.TotalBatches() != 5 {
		t.Errorf("Expected total batches 5, got %d", saved.TotalBatches())
	}

	if saved.Status() != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", saved.Status())
	}
}

// TestBatchProgressRepository_Save_DuplicateBatch tests upserting duplicate batch (same job_id + batch_number).
func TestBatchProgressRepository_Save_DuplicateBatch(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create and save first batch
	batch1 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch1)
	if err != nil {
		t.Fatalf("Failed to save first batch: %v", err)
	}

	// Try to save duplicate batch with same indexing job ID and batch number
	// This should perform an upsert, updating the existing record
	batch2 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	_ = batch2.MarkSubmittedToGemini("batches/test-updated-gemini-job-id")
	err = batchRepo.Save(ctx, batch2)
	// Should succeed (upsert behavior)
	if err != nil {
		t.Errorf("Expected successful upsert but got error: %v", err)
	}

	// Verify the record was updated
	batches, err := batchRepo.GetByJobID(ctx, indexingJobID)
	if err != nil {
		t.Fatalf("Failed to retrieve batches: %v", err)
	}
	if len(batches) != 1 {
		t.Errorf("Expected 1 batch (upsert), got %d", len(batches))
	}
	if batches[0].GeminiBatchJobID() == nil || *batches[0].GeminiBatchJobID() != "batches/test-updated-gemini-job-id" {
		t.Errorf(
			"Expected updated gemini_batch_job_id 'batches/test-updated-gemini-job-id', got %v",
			batches[0].GeminiBatchJobID(),
		)
	}
}

// TestBatchProgressRepository_Save_InvalidIndexingJobID tests saving with non-existent indexing job ID.
func TestBatchProgressRepository_Save_InvalidIndexingJobID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create batch with non-existent indexing job ID
	invalidJobID := uuid.New()
	repositoryID := uuid.New()
	batch := createTestBatchJobProgress(t, repositoryID, invalidJobID, 1, 5)

	// Try to save with invalid foreign key
	err := batchRepo.Save(ctx, batch)

	// Should return error for foreign key constraint
	if err == nil {
		t.Error("Expected error when saving with invalid indexing_job_id but got none")
	}
}

// TestBatchProgressRepository_GetByJobID_ReturnsAllBatches tests retrieving all batches for a job.
func TestBatchProgressRepository_GetByJobID_ReturnsAllBatches(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create and save multiple batches for the same job
	numBatches := 5
	batchIDs := make([]uuid.UUID, numBatches)

	for i := 1; i <= numBatches; i++ {
		batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, i, numBatches)
		err := batchRepo.Save(ctx, batch)
		if err != nil {
			t.Fatalf("Failed to save batch %d: %v", i, err)
		}
		batchIDs[i-1] = batch.ID()
	}

	// Retrieve all batches for the job
	batches, err := batchRepo.GetByJobID(ctx, indexingJobID)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Should return all batches
	if len(batches) != numBatches {
		t.Errorf("Expected %d batches, got %d", numBatches, len(batches))
	}

	// Verify batches are ordered by batch_number
	for i := range len(batches) - 1 {
		if batches[i].BatchNumber() > batches[i+1].BatchNumber() {
			t.Errorf("Expected batches ordered by batch_number, but found batch %d after batch %d",
				batches[i].BatchNumber(), batches[i+1].BatchNumber())
		}
	}
}

// TestBatchProgressRepository_GetByJobID_EmptyResults tests retrieving batches for job with no batches.
func TestBatchProgressRepository_GetByJobID_EmptyResults(t *testing.T) {
	pool, _, _, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Try to retrieve batches for a different job ID that has no batches
	nonExistentJobID := uuid.New()
	batches, err := batchRepo.GetByJobID(ctx, nonExistentJobID)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Should return empty slice
	if len(batches) != 0 {
		t.Errorf("Expected empty slice, got %d batches", len(batches))
	}

	// Should return empty slice, not nil
	if batches == nil {
		t.Error("Expected empty slice, not nil")
	}
}

// TestBatchProgressRepository_GetByJobID_InvalidJobIDFormat tests error handling for invalid job ID.
func TestBatchProgressRepository_GetByJobID_InvalidJobIDFormat(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Try to retrieve batches with nil UUID
	batches, err := batchRepo.GetByJobID(ctx, uuid.Nil)

	// Should return error for invalid input
	if err == nil {
		t.Error("Expected error for nil job ID but got none")
	}

	if batches != nil {
		t.Error("Expected nil result on error")
	}
}

// TestBatchProgressRepository_GetByID_SuccessfulRetrieval tests retrieving batch progress by ID.
func TestBatchProgressRepository_GetByID_SuccessfulRetrieval(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create and save a batch
	originalBatch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, originalBatch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Retrieve the batch by ID
	retrieved, err := batchRepo.GetByID(ctx, originalBatch.ID())
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected batch but got nil")
	}

	// Verify all fields are reconstructed correctly
	if retrieved.ID() != originalBatch.ID() {
		t.Errorf("Expected ID %s, got %s", originalBatch.ID(), retrieved.ID())
	}

	if retrieved.IndexingJobID() != indexingJobID {
		t.Errorf("Expected indexing job ID %s, got %s", indexingJobID, retrieved.IndexingJobID())
	}

	if retrieved.BatchNumber() != 1 {
		t.Errorf("Expected batch number 1, got %d", retrieved.BatchNumber())
	}

	if retrieved.TotalBatches() != 5 {
		t.Errorf("Expected total batches 5, got %d", retrieved.TotalBatches())
	}

	if retrieved.ChunksProcessed() != 0 {
		t.Errorf("Expected chunks processed 0, got %d", retrieved.ChunksProcessed())
	}

	if retrieved.Status() != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", retrieved.Status())
	}

	if retrieved.RetryCount() != 0 {
		t.Errorf("Expected retry count 0, got %d", retrieved.RetryCount())
	}

	if retrieved.ErrorMessage() != nil {
		t.Errorf("Expected error message nil, got %v", retrieved.ErrorMessage())
	}

	if retrieved.NextRetryAt() != nil {
		t.Errorf("Expected next retry at nil, got %v", retrieved.NextRetryAt())
	}
}

// TestBatchProgressRepository_GetByID_NonExistentID tests retrieving non-existent batch.
func TestBatchProgressRepository_GetByID_NonExistentID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Try to retrieve non-existent batch
	nonExistentID := uuid.New()
	retrieved, err := batchRepo.GetByID(ctx, nonExistentID)
	// Should succeed without error but return nil
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected nil batch for non-existent ID")
	}
}

// TestBatchProgressRepository_UpdateStatus_Successful tests updating batch status.
func TestBatchProgressRepository_UpdateStatus_Successful(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create and save a batch
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Update status to "processing"
	newStatus := "processing"
	err = batchRepo.UpdateStatus(ctx, batch.ID(), newStatus)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify status was updated
	updated, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve updated batch: %v", err)
	}

	if updated.Status() != newStatus {
		t.Errorf("Expected status '%s', got '%s'", newStatus, updated.Status())
	}
}

// TestBatchProgressRepository_UpdateStatus_UpdatesTimestamp tests that UpdateStatus updates updated_at.
func TestBatchProgressRepository_UpdateStatus_UpdatesTimestamp(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create and save a batch
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	originalUpdatedAt := batch.UpdatedAt()

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update status
	err = batchRepo.UpdateStatus(ctx, batch.ID(), "processing")
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Verify updated_at was changed
	updated, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve updated batch: %v", err)
	}

	if updated.UpdatedAt().Equal(originalUpdatedAt) {
		t.Error("Expected UpdatedAt timestamp to be updated")
	}

	if !updated.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be after original timestamp")
	}
}

// TestBatchProgressRepository_UpdateStatus_NonExistentID tests updating non-existent batch.
func TestBatchProgressRepository_UpdateStatus_NonExistentID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Try to update non-existent batch
	nonExistentID := uuid.New()
	err := batchRepo.UpdateStatus(ctx, nonExistentID, "processing")

	// Should return error
	if err == nil {
		t.Error("Expected error when updating non-existent batch but got none")
	}
}

// TestBatchProgressRepository_GetNextRetryBatch_ReturnsReadyBatch tests retrieving next retry batch.
func TestBatchProgressRepository_GetNextRetryBatch_ReturnsReadyBatch(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Clean up any retry_scheduled batches from previous tests
	_, _ = pool.Exec(ctx, "DELETE FROM codechunking.batch_job_progress WHERE status = 'retry_scheduled'")

	// Create a batch and schedule retry in the past
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Mark batch as failed and schedule retry in the past
	pastRetryTime := time.Now().Add(-1 * time.Hour)
	batch.ScheduleRetry(pastRetryTime)
	// Save the modified batch (updates all fields including next_retry_at)
	err = batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch after scheduling retry: %v", err)
	}

	// Retrieve next retry batch
	nextRetry, err := batchRepo.GetNextRetryBatch(ctx)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Should return the batch ready for retry
	if nextRetry == nil {
		t.Error("Expected batch ready for retry but got nil")
	}

	if nextRetry.ID() != batch.ID() {
		t.Errorf("Expected batch ID %s, got %s", batch.ID(), nextRetry.ID())
	}

	if nextRetry.Status() != "retry_scheduled" {
		t.Errorf("Expected status 'retry_scheduled', got '%s'", nextRetry.Status())
	}
}

// TestBatchProgressRepository_GetNextRetryBatch_NoReadyBatches tests when no batches are ready for retry.
func TestBatchProgressRepository_GetNextRetryBatch_NoReadyBatches(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Clean up any retry_scheduled batches from previous tests
	_, _ = pool.Exec(ctx, "DELETE FROM codechunking.batch_job_progress WHERE status = 'retry_scheduled'")

	// Create a batch in pending status (not scheduled for retry)
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Retrieve next retry batch
	nextRetry, err := batchRepo.GetNextRetryBatch(ctx)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Should return nil when no batches are ready
	if nextRetry != nil {
		t.Errorf("Expected nil when no batches ready for retry, but got batch with status=%s, next_retry_at=%v",
			nextRetry.Status(), nextRetry.NextRetryAt())
	}
}

// TestBatchProgressRepository_GetNextRetryBatch_IgnoresFutureTimes tests that future retry times are ignored.
func TestBatchProgressRepository_GetNextRetryBatch_IgnoresFutureTimes(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Clean up any retry_scheduled batches from previous tests
	_, _ = pool.Exec(ctx, "DELETE FROM codechunking.batch_job_progress WHERE status = 'retry_scheduled'")

	// Create a batch and schedule retry in the future
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Schedule retry in the future
	futureRetryTime := time.Now().Add(1 * time.Hour)
	batch.ScheduleRetry(futureRetryTime)
	err = batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch after scheduling retry: %v", err)
	}

	// Retrieve next retry batch
	nextRetry, err := batchRepo.GetNextRetryBatch(ctx)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Should return nil because retry time is in the future
	if nextRetry != nil {
		t.Errorf("Expected nil when retry time is in the future, but got batch with status=%s, next_retry_at=%v",
			nextRetry.Status(), nextRetry.NextRetryAt())
	}
}

// TestBatchProgressRepository_GetNextRetryBatch_OrdersByRetryTime tests ordering by next_retry_at.
func TestBatchProgressRepository_GetNextRetryBatch_OrdersByRetryTime(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create multiple batches with different retry times
	batch1 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 3)
	batch2 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 2, 3)
	batch3 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 3, 3)

	err := batchRepo.Save(ctx, batch1)
	if err != nil {
		t.Fatalf("Failed to save batch1: %v", err)
	}
	err = batchRepo.Save(ctx, batch2)
	if err != nil {
		t.Fatalf("Failed to save batch2: %v", err)
	}
	err = batchRepo.Save(ctx, batch3)
	if err != nil {
		t.Fatalf("Failed to save batch3: %v", err)
	}

	// Schedule retries with different times (batch2 should be oldest)
	time1 := time.Now().Add(-3 * time.Hour) // Oldest - should be returned first
	time2 := time.Now().Add(-1 * time.Hour)
	time3 := time.Now().Add(1 * time.Hour) // Future - should not be returned

	batch1.ScheduleRetry(time2)
	batch2.ScheduleRetry(time1)
	batch3.ScheduleRetry(time3)

	err = batchRepo.Save(ctx, batch1)
	if err != nil {
		t.Fatalf("Failed to save batch1: %v", err)
	}
	err = batchRepo.Save(ctx, batch2)
	if err != nil {
		t.Fatalf("Failed to save batch2: %v", err)
	}
	err = batchRepo.Save(ctx, batch3)
	if err != nil {
		t.Fatalf("Failed to save batch3: %v", err)
	}

	// Retrieve next retry batch
	nextRetry, err := batchRepo.GetNextRetryBatch(ctx)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Should return batch2 (oldest retry time that's in the past)
	if nextRetry == nil {
		t.Error("Expected batch ready for retry but got nil")
	}

	if nextRetry.ID() != batch2.ID() {
		t.Errorf("Expected batch ID %s (batch2 with oldest retry time), got %s", batch2.ID(), nextRetry.ID())
	}
}

// TestBatchProgressRepository_MarkCompleted_Successful tests marking batch as completed.
func TestBatchProgressRepository_MarkCompleted_Successful(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create and save a batch
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Mark batch as completed with chunks count
	chunksProcessed := 1000
	err = batchRepo.MarkCompleted(ctx, batch.ID(), chunksProcessed)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify batch was marked as completed
	completed, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve completed batch: %v", err)
	}

	if completed.Status() != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", completed.Status())
	}

	if completed.ChunksProcessed() != chunksProcessed {
		t.Errorf("Expected chunks processed %d, got %d", chunksProcessed, completed.ChunksProcessed())
	}
}

// TestBatchProgressRepository_MarkCompleted_NonExistentID tests marking non-existent batch as completed.
func TestBatchProgressRepository_MarkCompleted_NonExistentID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Try to mark non-existent batch as completed
	nonExistentID := uuid.New()
	err := batchRepo.MarkCompleted(ctx, nonExistentID, 100)

	// Should return error
	if err == nil {
		t.Error("Expected error when marking non-existent batch as completed but got none")
	}
}

// TestBatchProgressRepository_Delete_Successful tests deleting a batch.
func TestBatchProgressRepository_Delete_Successful(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create and save a batch
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Delete the batch
	err = batchRepo.Delete(ctx, batch.ID())
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify batch was deleted
	deleted, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Errorf("Expected no error when retrieving deleted batch but got: %v", err)
	}

	if deleted != nil {
		t.Error("Expected deleted batch to not be found")
	}
}

// TestBatchProgressRepository_Delete_NonExistentID tests deleting non-existent batch (idempotent).
func TestBatchProgressRepository_Delete_NonExistentID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Try to delete non-existent batch
	nonExistentID := uuid.New()
	err := batchRepo.Delete(ctx, nonExistentID)
	// Should succeed (idempotent operation)
	if err != nil {
		t.Errorf("Expected no error for idempotent delete but got: %v", err)
	}
}

// TestBatchProgressRepository_MultipleJobsIsolation tests that batches from different jobs are isolated.
func TestBatchProgressRepository_MultipleJobsIsolation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Create two indexing jobs
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	job1 := createTestIndexingJob(t, testRepo.ID())
	job2 := createTestIndexingJob(t, testRepo.ID())

	err = jobRepo.Save(ctx, job1)
	if err != nil {
		t.Fatalf("Failed to save job1: %v", err)
	}

	err = jobRepo.Save(ctx, job2)
	if err != nil {
		t.Fatalf("Failed to save job2: %v", err)
	}

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create batches for job1
	batch1 := createTestBatchJobProgress(t, testRepo.ID(), job1.ID(), 1, 3)
	batch2 := createTestBatchJobProgress(t, testRepo.ID(), job1.ID(), 2, 3)

	err = batchRepo.Save(ctx, batch1)
	if err != nil {
		t.Fatalf("Failed to save batch1: %v", err)
	}

	err = batchRepo.Save(ctx, batch2)
	if err != nil {
		t.Fatalf("Failed to save batch2: %v", err)
	}

	// Create batches for job2
	batch3 := createTestBatchJobProgress(t, testRepo.ID(), job2.ID(), 1, 2)

	err = batchRepo.Save(ctx, batch3)
	if err != nil {
		t.Fatalf("Failed to save batch3: %v", err)
	}

	// Retrieve batches for job1
	job1Batches, err := batchRepo.GetByJobID(ctx, job1.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve job1 batches: %v", err)
	}

	// Should return only 2 batches for job1
	if len(job1Batches) != 2 {
		t.Errorf("Expected 2 batches for job1, got %d", len(job1Batches))
	}

	// Retrieve batches for job2
	job2Batches, err := batchRepo.GetByJobID(ctx, job2.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve job2 batches: %v", err)
	}

	// Should return only 1 batch for job2
	if len(job2Batches) != 1 {
		t.Errorf("Expected 1 batch for job2, got %d", len(job2Batches))
	}

	if job2Batches[0].ID() != batch3.ID() {
		t.Errorf("Expected batch3 ID %s, got %s", batch3.ID(), job2Batches[0].ID())
	}
}

// TestBatchProgressRepository_StatusTransitions tests status transitions are persisted correctly.
func TestBatchProgressRepository_StatusTransitions(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create and save a batch
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Transition: pending -> processing
	err = batchRepo.UpdateStatus(ctx, batch.ID(), "processing")
	if err != nil {
		t.Fatalf("Failed to update to processing: %v", err)
	}

	processing, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve processing batch: %v", err)
	}

	if processing.Status() != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", processing.Status())
	}

	// Transition: processing -> completed
	err = batchRepo.MarkCompleted(ctx, batch.ID(), 1000)
	if err != nil {
		t.Fatalf("Failed to mark as completed: %v", err)
	}

	completed, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve completed batch: %v", err)
	}

	if completed.Status() != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", completed.Status())
	}

	if completed.ChunksProcessed() != 1000 {
		t.Errorf("Expected chunks processed 1000, got %d", completed.ChunksProcessed())
	}
}

// ========================================
// GetPendingSubmissionBatch() Tests
// ========================================

// TestBatchProgressRepository_GetPendingSubmissionBatch_Success tests retrieving a batch ready for submission.
func TestBatchProgressRepository_GetPendingSubmissionBatch_Success(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create batch with pending_submission status
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001"}]}`)
	batch.MarkPendingSubmission(requestData)
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Retrieve pending submission batch
	retrieved, err := batchRepo.GetPendingSubmissionBatch(ctx)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected batch but got nil")
	}

	// Verify all fields match
	if retrieved.ID() != batch.ID() {
		t.Errorf("Expected ID %s, got %s", batch.ID(), retrieved.ID())
	}

	if retrieved.Status() != entity.StatusPendingSubmission {
		t.Errorf("Expected status 'pending_submission', got '%s'", retrieved.Status())
	}

	if string(retrieved.BatchRequestData()) != string(requestData) {
		t.Errorf("Expected request data %s, got %s", requestData, retrieved.BatchRequestData())
	}

	if retrieved.SubmissionAttempts() != 0 {
		t.Errorf("Expected submission attempts 0, got %d", retrieved.SubmissionAttempts())
	}
}

// TestBatchProgressRepository_GetPendingSubmissionBatch_NoBatchesReady tests when no batches are ready.
func TestBatchProgressRepository_GetPendingSubmissionBatch_NoBatchesReady(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Test case 1: No batches in database
	retrieved, err := batchRepo.GetPendingSubmissionBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error when no batches exist but got: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected nil when no batches exist but got batch")
	}

	// Test case 2: Batches exist but have different status
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	// Leave in default "pending" status
	err = batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	retrieved, err = batchRepo.GetPendingSubmissionBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected nil when no pending_submission batches exist but got batch")
	}

	// Test case 3: Batch has pending_submission status but next_submission_at is in future
	batch2 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 2, 5)
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001"}]}`)
	batch2.MarkPendingSubmission(requestData)
	futureTime := time.Now().Add(1 * time.Hour)
	batch2.MarkSubmissionFailed("rate limit", futureTime)
	err = batchRepo.Save(ctx, batch2)
	if err != nil {
		t.Fatalf("Failed to save batch2: %v", err)
	}

	retrieved, err = batchRepo.GetPendingSubmissionBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected nil when next_submission_at is in future but got batch")
	}
}

// TestBatchProgressRepository_GetPendingSubmissionBatch_MultipleReady tests FIFO ordering.
func TestBatchProgressRepository_GetPendingSubmissionBatch_MultipleReady(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create 3 batches with different created_at timestamps
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001"}]}`)

	// Batch 1 - oldest
	batch1 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 3)
	batch1.MarkPendingSubmission(requestData)
	err := batchRepo.Save(ctx, batch1)
	if err != nil {
		t.Fatalf("Failed to save batch1: %v", err)
	}

	// Wait to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Batch 2 - middle
	batch2 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 2, 3)
	batch2.MarkPendingSubmission(requestData)
	err = batchRepo.Save(ctx, batch2)
	if err != nil {
		t.Fatalf("Failed to save batch2: %v", err)
	}

	// Wait to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Batch 3 - newest
	batch3 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 3, 3)
	batch3.MarkPendingSubmission(requestData)
	err = batchRepo.Save(ctx, batch3)
	if err != nil {
		t.Fatalf("Failed to save batch3: %v", err)
	}

	// Retrieve next pending submission batch
	retrieved, err := batchRepo.GetPendingSubmissionBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected batch but got nil")
	}

	// Should return oldest batch (batch1)
	if retrieved.ID() != batch1.ID() {
		t.Errorf("Expected oldest batch ID %s (batch1), got %s", batch1.ID(), retrieved.ID())
	}
}

// TestBatchProgressRepository_GetPendingSubmissionBatch_WithNextSubmissionAt tests submission timing.
func TestBatchProgressRepository_GetPendingSubmissionBatch_WithNextSubmissionAt(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001"}]}`)

	// Test case 1: Batch with next_submission_at = nil should be returned
	batch1 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 4)
	batch1.MarkPendingSubmission(requestData)
	err := batchRepo.Save(ctx, batch1)
	if err != nil {
		t.Fatalf("Failed to save batch1: %v", err)
	}

	retrieved, err := batchRepo.GetPendingSubmissionBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if retrieved == nil {
		t.Error("Expected batch with nil next_submission_at to be returned")
	} else if retrieved.ID() != batch1.ID() {
		t.Errorf("Expected batch1 ID %s, got %s", batch1.ID(), retrieved.ID())
	}

	// Clean up batch1 to test next case
	err = batchRepo.Delete(ctx, batch1.ID())
	if err != nil {
		t.Fatalf("Failed to delete batch1: %v", err)
	}

	// Test case 2: Batch with next_submission_at in past should be returned
	batch2 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 2, 4)
	batch2.MarkPendingSubmission(requestData)
	pastTime := time.Now().Add(-1 * time.Hour)
	batch2.MarkSubmissionFailed("previous error", pastTime)
	err = batchRepo.Save(ctx, batch2)
	if err != nil {
		t.Fatalf("Failed to save batch2: %v", err)
	}

	retrieved, err = batchRepo.GetPendingSubmissionBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if retrieved == nil {
		t.Error("Expected batch with past next_submission_at to be returned")
	} else if retrieved.ID() != batch2.ID() {
		t.Errorf("Expected batch2 ID %s, got %s", batch2.ID(), retrieved.ID())
	}

	// Clean up batch2 to test next case
	err = batchRepo.Delete(ctx, batch2.ID())
	if err != nil {
		t.Fatalf("Failed to delete batch2: %v", err)
	}

	// Test case 3: Batch with next_submission_at in future should NOT be returned
	batch3 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 3, 4)
	batch3.MarkPendingSubmission(requestData)
	futureTime := time.Now().Add(1 * time.Hour)
	batch3.MarkSubmissionFailed("rate limit", futureTime)
	err = batchRepo.Save(ctx, batch3)
	if err != nil {
		t.Fatalf("Failed to save batch3: %v", err)
	}

	retrieved, err = batchRepo.GetPendingSubmissionBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected nil when next_submission_at is in future, but got batch")
	}
}

// TestBatchProgressRepository_GetPendingSubmissionBatch_DistributedLocking tests concurrent access safety.
func TestBatchProgressRepository_GetPendingSubmissionBatch_DistributedLocking(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001"}]}`)

	// Create two batches ready for submission
	batch1 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 2)
	batch1.MarkPendingSubmission(requestData)
	err := batchRepo.Save(ctx, batch1)
	if err != nil {
		t.Fatalf("Failed to save batch1: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	batch2 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 2, 2)
	batch2.MarkPendingSubmission(requestData)
	err = batchRepo.Save(ctx, batch2)
	if err != nil {
		t.Fatalf("Failed to save batch2: %v", err)
	}

	// Start first transaction
	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin tx1: %v", err)
	}
	defer tx1.Rollback(ctx)

	// First transaction gets a batch with FOR UPDATE SKIP LOCKED
	// Note: GetQueryInterface will use tx1 from context if present
	// For testing, we directly pass a transaction-wrapped context
	// In production, this would be handled by TransactionManager.WithTransaction
	ctx1 := context.Background() // Use clean context for tx1
	retrieved1Batch, err := func() (*entity.BatchJobProgress, error) {
		// Manually query within transaction to test FOR UPDATE SKIP LOCKED
		query := `SELECT id, repository_id, indexing_job_id, batch_number, total_batches,
			chunks_processed, status, retry_count, next_retry_at, error_message,
			gemini_batch_job_id, gemini_file_uri, created_at, updated_at,
			batch_request_data, submission_attempts, next_submission_at
			FROM codechunking.batch_job_progress
			WHERE status = 'pending_submission'
			AND (next_submission_at IS NULL OR next_submission_at <= CURRENT_TIMESTAMP)
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED`

		row := tx1.QueryRow(ctx1, query)

		var id, indexingJobID uuid.UUID
		var repositoryID *uuid.UUID
		var batchNumber, totalBatches, chunksProcessed, retryCount, submissionAttempts int
		var status string
		var nextRetryAt *time.Time
		var errorMessage *string
		var geminiBatchJobID *string
		var geminiFileURI *string
		var createdAt, updatedAt time.Time
		var batchRequestData []byte
		var nextSubmissionAt *time.Time

		err := row.Scan(
			&id, &repositoryID, &indexingJobID, &batchNumber, &totalBatches, &chunksProcessed,
			&status, &retryCount, &nextRetryAt, &errorMessage, &geminiBatchJobID, &geminiFileURI,
			&createdAt, &updatedAt, &batchRequestData, &submissionAttempts, &nextSubmissionAt,
		)
		if err != nil {
			return nil, err
		}

		return entity.RestoreBatchJobProgress(
			id, repositoryID, indexingJobID, batchNumber, totalBatches, chunksProcessed,
			status, retryCount, nextRetryAt, errorMessage, geminiBatchJobID, geminiFileURI,
			createdAt, updatedAt, batchRequestData, submissionAttempts, nextSubmissionAt,
		), nil
	}()
	if err != nil {
		t.Fatalf("Transaction 1 failed to get batch: %v", err)
	}
	if retrieved1Batch == nil {
		t.Fatal("Transaction 1 expected batch but got nil")
	}

	// Start second transaction before first commits
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin tx2: %v", err)
	}
	defer tx2.Rollback(ctx)

	// Second transaction should skip the locked batch and get the next one
	ctx2 := context.Background() // Use clean context for tx2
	retrieved2Batch, err := func() (*entity.BatchJobProgress, error) {
		// Manually query within transaction to test FOR UPDATE SKIP LOCKED
		query := `SELECT id, repository_id, indexing_job_id, batch_number, total_batches,
			chunks_processed, status, retry_count, next_retry_at, error_message,
			gemini_batch_job_id, gemini_file_uri, created_at, updated_at,
			batch_request_data, submission_attempts, next_submission_at
			FROM codechunking.batch_job_progress
			WHERE status = 'pending_submission'
			AND (next_submission_at IS NULL OR next_submission_at <= CURRENT_TIMESTAMP)
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED`

		row := tx2.QueryRow(ctx2, query)

		var id, indexingJobID uuid.UUID
		var repositoryID *uuid.UUID
		var batchNumber, totalBatches, chunksProcessed, retryCount, submissionAttempts int
		var status string
		var nextRetryAt *time.Time
		var errorMessage *string
		var geminiBatchJobID *string
		var geminiFileURI *string
		var createdAt, updatedAt time.Time
		var batchRequestData []byte
		var nextSubmissionAt *time.Time

		err := row.Scan(
			&id, &repositoryID, &indexingJobID, &batchNumber, &totalBatches, &chunksProcessed,
			&status, &retryCount, &nextRetryAt, &errorMessage, &geminiBatchJobID, &geminiFileURI,
			&createdAt, &updatedAt, &batchRequestData, &submissionAttempts, &nextSubmissionAt,
		)
		if err != nil {
			return nil, err
		}

		return entity.RestoreBatchJobProgress(
			id, repositoryID, indexingJobID, batchNumber, totalBatches, chunksProcessed,
			status, retryCount, nextRetryAt, errorMessage, geminiBatchJobID, geminiFileURI,
			createdAt, updatedAt, batchRequestData, submissionAttempts, nextSubmissionAt,
		), nil
	}()

	retrieved1 := retrieved1Batch
	retrieved2 := retrieved2Batch
	if err != nil {
		t.Fatalf("Transaction 2 failed to get batch: %v", err)
	}
	if retrieved2 == nil {
		t.Fatal("Transaction 2 expected batch but got nil")
	}

	// Verify no duplicate batch retrieval - both transactions should get different batches
	if retrieved1.ID() == retrieved2.ID() {
		t.Errorf("Expected different batches but both transactions got batch ID %s", retrieved1.ID())
	}

	// Verify transaction 1 got the older batch (batch1)
	if retrieved1.ID() != batch1.ID() {
		t.Errorf("Expected transaction 1 to get batch1 ID %s, got %s", batch1.ID(), retrieved1.ID())
	}

	// Verify transaction 2 got the newer batch (batch2)
	if retrieved2.ID() != batch2.ID() {
		t.Errorf("Expected transaction 2 to get batch2 ID %s, got %s", batch2.ID(), retrieved2.ID())
	}
}

// ========================================
// GetPendingSubmissionCount() Tests
// ========================================

// TestBatchProgressRepository_GetPendingSubmissionCount_Success tests counting pending submission batches.
func TestBatchProgressRepository_GetPendingSubmissionCount_Success(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001"}]}`)

	// Test case 1: No batches should return 0
	count, err := batchRepo.GetPendingSubmissionCount(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 when no batches exist, got %d", count)
	}

	// Test case 2: Create 3 pending_submission batches
	for i := 1; i <= 3; i++ {
		batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, i, 5)
		batch.MarkPendingSubmission(requestData)
		err := batchRepo.Save(ctx, batch)
		if err != nil {
			t.Fatalf("Failed to save batch %d: %v", i, err)
		}
	}

	count, err = batchRepo.GetPendingSubmissionCount(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3 after creating 3 pending_submission batches, got %d", count)
	}

	// Test case 3: Create batches with other statuses - should not be counted
	batch4 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 4, 5)
	// Leave in "pending" status
	err = batchRepo.Save(ctx, batch4)
	if err != nil {
		t.Fatalf("Failed to save batch4: %v", err)
	}

	batch5 := createTestBatchJobProgress(t, repositoryID, indexingJobID, 5, 5)
	batch5.MarkProcessing()
	err = batchRepo.Save(ctx, batch5)
	if err != nil {
		t.Fatalf("Failed to save batch5: %v", err)
	}

	count, err = batchRepo.GetPendingSubmissionCount(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count to remain 3 after adding non-pending_submission batches, got %d", count)
	}

	// Test case 4: Count includes batches regardless of next_submission_at value
	// Create batch with future next_submission_at - should still be counted
	batch6 := createTestBatchJobProgress(t, repositoryID, uuid.New(), 1, 1)
	batch6.MarkPendingSubmission(requestData)
	futureTime := time.Now().Add(1 * time.Hour)
	batch6.MarkSubmissionFailed("rate limit", futureTime)
	err = batchRepo.Save(ctx, batch6)
	if err != nil {
		t.Fatalf("Failed to save batch6: %v", err)
	}

	count, err = batchRepo.GetPendingSubmissionCount(ctx)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected count 4 including batch with future next_submission_at, got %d", count)
	}
}

// ========================================
// Save() with New Fields Tests
// ========================================

// TestBatchProgressRepository_Save_WithBatchRequestData tests saving and retrieving batch request data.
func TestBatchProgressRepository_Save_WithBatchRequestData(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create batch with request data
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001","content":"test"}]}`)
	batch.MarkPendingSubmission(requestData)

	// Save the batch
	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Retrieve and verify request data persisted correctly
	retrieved, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve batch: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected batch but got nil")
	}

	if string(retrieved.BatchRequestData()) != string(requestData) {
		t.Errorf("Expected request data %s, got %s", requestData, retrieved.BatchRequestData())
	}

	if retrieved.Status() != entity.StatusPendingSubmission {
		t.Errorf("Expected status 'pending_submission', got '%s'", retrieved.Status())
	}
}

// TestBatchProgressRepository_Save_WithSubmissionAttempts tests saving and updating submission attempts.
func TestBatchProgressRepository_Save_WithSubmissionAttempts(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Create batch with submission attempts
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001"}]}`)
	batch.MarkPendingSubmission(requestData)

	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// Verify initial submission attempts is 0
	retrieved, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve batch: %v", err)
	}
	if retrieved.SubmissionAttempts() != 0 {
		t.Errorf("Expected initial submission attempts 0, got %d", retrieved.SubmissionAttempts())
	}

	// Update submission attempts by marking as failed
	futureTime := time.Now().Add(1 * time.Minute)
	batch.MarkSubmissionFailed("rate limit exceeded", futureTime)
	err = batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to update batch: %v", err)
	}

	// Verify submission attempts incremented
	updated, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve updated batch: %v", err)
	}
	if updated.SubmissionAttempts() != 1 {
		t.Errorf("Expected submission attempts 1 after first failure, got %d", updated.SubmissionAttempts())
	}

	// Increment again
	batch.MarkSubmissionFailed("rate limit exceeded again", futureTime)
	err = batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to update batch again: %v", err)
	}

	updated2, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve batch after second update: %v", err)
	}
	if updated2.SubmissionAttempts() != 2 {
		t.Errorf("Expected submission attempts 2 after second failure, got %d", updated2.SubmissionAttempts())
	}
}

// TestBatchProgressRepository_Save_WithNextSubmissionAt tests saving and updating next submission time.
func TestBatchProgressRepository_Save_WithNextSubmissionAt(t *testing.T) {
	pool, repositoryID, indexingJobID, ctx := setupBatchProgressTest(t)
	defer pool.Close()

	batchRepo := NewPostgreSQLBatchProgressRepository(pool)

	// Test case 1: Save batch with nil next_submission_at
	batch := createTestBatchJobProgress(t, repositoryID, indexingJobID, 1, 5)
	requestData := []byte(`{"requests":[{"model":"gemini-embedding-001"}]}`)
	batch.MarkPendingSubmission(requestData)

	err := batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	retrieved, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve batch: %v", err)
	}
	if retrieved.NextSubmissionAt() != nil {
		t.Errorf("Expected nil next_submission_at, got %v", retrieved.NextSubmissionAt())
	}

	// Test case 2: Save batch with future next_submission_at
	futureTime := time.Now().Add(5 * time.Minute)
	batch.MarkSubmissionFailed("rate limit", futureTime)

	err = batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to update batch: %v", err)
	}

	updated, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve updated batch: %v", err)
	}
	if updated.NextSubmissionAt() == nil {
		t.Error("Expected next_submission_at to be set but got nil")
	} else {
		// Allow 1 second tolerance for time comparison
		timeDiff := updated.NextSubmissionAt().Sub(futureTime)
		if timeDiff > 1*time.Second || timeDiff < -1*time.Second {
			t.Errorf("Expected next_submission_at around %v, got %v", futureTime, updated.NextSubmissionAt())
		}
	}

	// Test case 3: Update next_submission_at on retry scheduling
	newRetryTime := time.Now().Add(10 * time.Minute)
	batch.MarkSubmissionFailed("another error", newRetryTime)

	err = batchRepo.Save(ctx, batch)
	if err != nil {
		t.Fatalf("Failed to update batch with new retry time: %v", err)
	}

	updated2, err := batchRepo.GetByID(ctx, batch.ID())
	if err != nil {
		t.Fatalf("Failed to retrieve batch after retry scheduling: %v", err)
	}
	if updated2.NextSubmissionAt() == nil {
		t.Error("Expected updated next_submission_at but got nil")
	} else {
		// Allow 1 second tolerance for time comparison
		timeDiff := updated2.NextSubmissionAt().Sub(newRetryTime)
		if timeDiff > 1*time.Second || timeDiff < -1*time.Second {
			t.Errorf("Expected next_submission_at around %v, got %v", newRetryTime, updated2.NextSubmissionAt())
		}
	}
}
