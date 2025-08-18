package repository

import (
	"context"
	"testing"
	"time"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// createTestIndexingJob creates a test indexing job entity.
func createTestIndexingJob(t *testing.T, repositoryID uuid.UUID) *entity.IndexingJob {
	return entity.NewIndexingJob(repositoryID)
}

// TestIndexingJobRepository_Save tests saving indexing jobs to database.
func TestIndexingJobRepository_Save(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// Create a test repository first (needed for foreign key)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	ctx := context.Background()
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	// This will fail because PostgreSQLIndexingJobRepository doesn't exist yet
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	tests := []struct {
		name        string
		job         *entity.IndexingJob
		expectError bool
	}{
		{
			name:        "Valid indexing job should save successfully",
			job:         createTestIndexingJob(t, testRepo.ID()),
			expectError: false,
		},
		{
			name:        "Indexing job with non-existent repository should fail",
			job:         createTestIndexingJob(t, uuid.New()),
			expectError: true,
		},
		{
			name:        "Multiple indexing jobs for same repository should save successfully",
			job:         createTestIndexingJob(t, testRepo.ID()),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jobRepo.Save(ctx, tt.job)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// Helper function to verify found job matches expectations.
// Reduces validation duplication across FindByID tests.
func verifyFoundJob(t *testing.T, foundJob *entity.IndexingJob, expectedJob *entity.IndexingJob) {
	if foundJob.ID() != expectedJob.ID() {
		t.Errorf("Expected job ID %s, got %s", expectedJob.ID(), foundJob.ID())
	}
	if foundJob.RepositoryID() != expectedJob.RepositoryID() {
		t.Errorf("Expected repository ID %s, got %s", expectedJob.RepositoryID(), foundJob.RepositoryID())
	}
	if foundJob.Status() != expectedJob.Status() {
		t.Errorf("Expected status %s, got %s", expectedJob.Status(), foundJob.Status())
	}
}

// TestIndexingJobRepository_FindByID_ExistingJob tests finding an existing job by ID.
// This focused test verifies that saved jobs can be retrieved correctly.
func TestIndexingJobRepository_FindByID_ExistingJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	// Find the saved job
	foundJob, err := jobRepo.FindByID(ctx, testJob.ID())
	// Verify successful retrieval
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if foundJob == nil {
		t.Fatal("Expected job but got nil")
	}

	// Verify job details match
	verifyFoundJob(t, foundJob, testJob)
}

// TestIndexingJobRepository_FindByID_NonExistentJob tests finding a non-existent job.
// This focused test verifies that queries for non-existent jobs return nil without error.
func TestIndexingJobRepository_FindByID_NonExistentJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, _, ctx := setupJobForUpdate(t, pool)

	// Try to find a non-existent job
	nonExistentID := uuid.New()
	foundJob, err := jobRepo.FindByID(ctx, nonExistentID)
	// Should return nil without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if foundJob != nil {
		t.Error("Expected nil job but got one")
	}
}

// TestIndexingJobRepository_FindByID_InvalidInput tests error handling for invalid inputs.
// This focused test verifies that invalid inputs are properly rejected.
func TestIndexingJobRepository_FindByID_InvalidInput(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, _, ctx := setupJobForUpdate(t, pool)

	// Try to find job with nil UUID
	foundJob, err := jobRepo.FindByID(ctx, uuid.Nil)

	// Should return error
	if err == nil {
		t.Error("Expected error but got none")
	}

	if foundJob != nil {
		t.Error("Expected nil job on error")
	}
}

// Helper function to setup a job for update testing.
// Reduces setup duplication across focused update tests.
func setupJobForUpdate(
	t *testing.T,
	pool *pgxpool.Pool,
) (*PostgreSQLIndexingJobRepository, *entity.IndexingJob, context.Context) {
	// Create a test repository first (needed for foreign key)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	ctx := context.Background()
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	// This will fail because PostgreSQLIndexingJobRepository doesn't exist yet
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	// Create and save test indexing job
	testJob := createTestIndexingJob(t, testRepo.ID())
	err = jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save test indexing job: %v", err)
	}

	return jobRepo, testJob, ctx
}

// Helper function to verify job persistence after update.
// Reduces validation duplication across focused update tests.
func verifyJobPersistence(
	t *testing.T,
	jobRepo *PostgreSQLIndexingJobRepository,
	ctx context.Context,
	expectedJob *entity.IndexingJob,
	originalUpdatedAt time.Time,
) {
	updatedJob, err := jobRepo.FindByID(ctx, expectedJob.ID())
	if err != nil {
		t.Fatalf("Failed to find updated job: %v", err)
	}

	if updatedJob == nil {
		t.Fatal("Updated job should not be nil")
	}

	if updatedJob.UpdatedAt().Equal(originalUpdatedAt) {
		t.Error("Expected UpdatedAt timestamp to be changed")
	}

	if updatedJob.Status() != expectedJob.Status() {
		t.Errorf("Expected updated status %s, got %s", expectedJob.Status(), updatedJob.Status())
	}

	if updatedJob.FilesProcessed() != expectedJob.FilesProcessed() {
		t.Errorf("Expected updated files processed %d, got %d",
			expectedJob.FilesProcessed(), updatedJob.FilesProcessed())
	}

	if updatedJob.ChunksCreated() != expectedJob.ChunksCreated() {
		t.Errorf("Expected updated chunks created %d, got %d",
			expectedJob.ChunksCreated(), updatedJob.ChunksCreated())
	}
}

// Helper function to verify archived job behavior.
// Reduces validation duplication for soft delete testing.
func verifyArchivedJobBehavior(
	t *testing.T,
	jobRepo *PostgreSQLIndexingJobRepository,
	ctx context.Context,
	archivedJob *entity.IndexingJob,
) {
	// For archived jobs, FindByID should return nil (soft deleted)
	foundJob, err := jobRepo.FindByID(ctx, archivedJob.ID())
	if err != nil {
		t.Errorf("Expected no error when finding archived job, got: %v", err)
	}

	if foundJob != nil {
		t.Error("Expected archived job to not be found (soft deleted)")
	}

	if !archivedJob.IsDeleted() {
		t.Error("Expected archived job to be marked as deleted")
	}
}

// TestIndexingJobRepository_Update_StartJob tests starting a pending job.
// This focused test verifies that job status transitions from pending to running correctly.
func TestIndexingJobRepository_Update_StartJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	// Get fresh copy and start the job
	jobToUpdate, err := jobRepo.FindByID(ctx, testJob.ID())
	if err != nil {
		t.Fatalf("Failed to find job for update: %v", err)
	}

	originalUpdatedAt := jobToUpdate.UpdatedAt()
	err = jobToUpdate.Start()
	if err != nil {
		t.Fatalf("Failed to start job: %v", err)
	}

	// Update the job in repository
	err = jobRepo.Update(ctx, jobToUpdate)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify persistence and status change
	verifyJobPersistence(t, jobRepo, ctx, jobToUpdate, originalUpdatedAt)

	// Verify specific start job behavior
	updatedJob, _ := jobRepo.FindByID(ctx, jobToUpdate.ID())
	if updatedJob.Status() != valueobject.JobStatusRunning {
		t.Errorf("Expected job status to be running, got %s", updatedJob.Status())
	}

	if updatedJob.StartedAt() == nil {
		t.Error("Expected StartedAt to be set when job is started")
	}
}

// TestIndexingJobRepository_Update_CompleteJob tests completing a running job.
// This focused test verifies that job completion with progress data is properly persisted.
func TestIndexingJobRepository_Update_CompleteJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	// Get fresh copy, start and complete the job
	jobToUpdate, err := jobRepo.FindByID(ctx, testJob.ID())
	if err != nil {
		t.Fatalf("Failed to find job for update: %v", err)
	}

	originalUpdatedAt := jobToUpdate.UpdatedAt()
	_ = jobToUpdate.Start()
	err = jobToUpdate.Complete(100, 500)
	if err != nil {
		t.Fatalf("Failed to complete job: %v", err)
	}

	// Update the job in repository
	err = jobRepo.Update(ctx, jobToUpdate)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify persistence and completion data
	verifyJobPersistence(t, jobRepo, ctx, jobToUpdate, originalUpdatedAt)

	// Verify specific completion behavior
	updatedJob, _ := jobRepo.FindByID(ctx, jobToUpdate.ID())
	if updatedJob.Status() != valueobject.JobStatusCompleted {
		t.Errorf("Expected job status to be completed, got %s", updatedJob.Status())
	}

	if updatedJob.FilesProcessed() != 100 {
		t.Errorf("Expected 100 files processed, got %d", updatedJob.FilesProcessed())
	}

	if updatedJob.ChunksCreated() != 500 {
		t.Errorf("Expected 500 chunks created, got %d", updatedJob.ChunksCreated())
	}

	if updatedJob.CompletedAt() == nil {
		t.Error("Expected CompletedAt to be set when job is completed")
	}
}

// TestIndexingJobRepository_Update_FailJob tests failing a running job with error message.
// This focused test verifies that job failure and error messages are properly persisted.
func TestIndexingJobRepository_Update_FailJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	// Get fresh copy, start and fail the job
	jobToUpdate, err := jobRepo.FindByID(ctx, testJob.ID())
	if err != nil {
		t.Fatalf("Failed to find job for update: %v", err)
	}

	originalUpdatedAt := jobToUpdate.UpdatedAt()
	_ = jobToUpdate.Start()
	errorMessage := "Test error message"
	err = jobToUpdate.Fail(errorMessage)
	if err != nil {
		t.Fatalf("Failed to fail job: %v", err)
	}

	// Update the job in repository
	err = jobRepo.Update(ctx, jobToUpdate)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify persistence and failure data
	verifyJobPersistence(t, jobRepo, ctx, jobToUpdate, originalUpdatedAt)

	// Verify specific failure behavior
	updatedJob, _ := jobRepo.FindByID(ctx, jobToUpdate.ID())
	if updatedJob.Status() != valueobject.JobStatusFailed {
		t.Errorf("Expected job status to be failed, got %s", updatedJob.Status())
	}

	if updatedJob.ErrorMessage() == nil {
		t.Error("Expected error message to be set for failed job")
	} else if *updatedJob.ErrorMessage() != errorMessage {
		t.Errorf("Expected error message '%s', got '%s'", errorMessage, *updatedJob.ErrorMessage())
	}

	if updatedJob.CompletedAt() == nil {
		t.Error("Expected CompletedAt to be set when job fails")
	}
}

// TestIndexingJobRepository_Update_CancelJob tests canceling a pending job.
// This focused test verifies that job cancellation is properly persisted.
func TestIndexingJobRepository_Update_CancelJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	// Get fresh copy and cancel the job
	jobToUpdate, err := jobRepo.FindByID(ctx, testJob.ID())
	if err != nil {
		t.Fatalf("Failed to find job for update: %v", err)
	}

	originalUpdatedAt := jobToUpdate.UpdatedAt()
	err = jobToUpdate.Cancel()
	if err != nil {
		t.Fatalf("Failed to cancel job: %v", err)
	}

	// Update the job in repository
	err = jobRepo.Update(ctx, jobToUpdate)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify persistence and cancellation
	verifyJobPersistence(t, jobRepo, ctx, jobToUpdate, originalUpdatedAt)

	// Verify specific cancellation behavior
	updatedJob, _ := jobRepo.FindByID(ctx, jobToUpdate.ID())
	if updatedJob.Status() != valueobject.JobStatusCancelled {
		t.Errorf("Expected job status to be cancelled, got %s", updatedJob.Status())
	}

	if updatedJob.CompletedAt() == nil {
		t.Error("Expected CompletedAt to be set when job is cancelled")
	}
}

// TestIndexingJobRepository_Update_ProgressUpdates tests updating job progress during execution.
// This focused test verifies that progress updates are properly persisted.
func TestIndexingJobRepository_Update_ProgressUpdates(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	// Get fresh copy, start and update progress
	jobToUpdate, err := jobRepo.FindByID(ctx, testJob.ID())
	if err != nil {
		t.Fatalf("Failed to find job for update: %v", err)
	}

	originalUpdatedAt := jobToUpdate.UpdatedAt()
	_ = jobToUpdate.Start()
	jobToUpdate.UpdateProgress(50, 250)

	// Update the job in repository
	err = jobRepo.Update(ctx, jobToUpdate)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify persistence and progress data
	verifyJobPersistence(t, jobRepo, ctx, jobToUpdate, originalUpdatedAt)

	// Verify specific progress update behavior
	updatedJob, _ := jobRepo.FindByID(ctx, jobToUpdate.ID())
	if updatedJob.Status() != valueobject.JobStatusRunning {
		t.Errorf("Expected job status to remain running, got %s", updatedJob.Status())
	}

	if updatedJob.FilesProcessed() != 50 {
		t.Errorf("Expected 50 files processed, got %d", updatedJob.FilesProcessed())
	}

	if updatedJob.ChunksCreated() != 250 {
		t.Errorf("Expected 250 chunks created, got %d", updatedJob.ChunksCreated())
	}
}

// TestIndexingJobRepository_Update_ArchiveJob tests archiving a completed job.
// This focused test verifies that job archival (soft delete) behavior works correctly.
func TestIndexingJobRepository_Update_ArchiveJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	// Get fresh copy, start, complete and archive the job
	jobToUpdate, err := jobRepo.FindByID(ctx, testJob.ID())
	if err != nil {
		t.Fatalf("Failed to find job for update: %v", err)
	}

	_ = jobToUpdate.Start()
	_ = jobToUpdate.Complete(100, 500)
	err = jobToUpdate.Archive()
	if err != nil {
		t.Fatalf("Failed to archive job: %v", err)
	}

	// Update the job in repository
	err = jobRepo.Update(ctx, jobToUpdate)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify archival behavior (soft delete)
	verifyArchivedJobBehavior(t, jobRepo, ctx, jobToUpdate)
}

// TestIndexingJobRepository_Update_NonExistentJob tests updating a non-existent job.
// This focused test verifies proper error handling when trying to update a job that doesn't exist.
func TestIndexingJobRepository_Update_NonExistentJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, _, ctx := setupJobForUpdate(t, pool)

	// Create a job that was never saved to the database
	nonExistentJob := createTestIndexingJob(t, uuid.New())
	_ = nonExistentJob.Start()

	// Attempt to update the non-existent job
	err := jobRepo.Update(ctx, nonExistentJob)

	// Should return an error for non-existent job
	if err == nil {
		t.Error("Expected error when updating non-existent job, but got none")
	}
}

// Helper function to create and save a test job.
// Reduces duplication across Delete tests.
func createAndSaveTestJob(
	t *testing.T,
	jobRepo *PostgreSQLIndexingJobRepository,
	repoID uuid.UUID,
	ctx context.Context,
) uuid.UUID {
	testJob := createTestIndexingJob(t, repoID)
	err := jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save test job: %v", err)
	}
	return testJob.ID()
}

// Helper function to create and save a completed test job.
// Reduces duplication across Delete tests.
func createAndSaveCompletedTestJob(
	t *testing.T,
	jobRepo *PostgreSQLIndexingJobRepository,
	repoID uuid.UUID,
	ctx context.Context,
) uuid.UUID {
	testJob := createTestIndexingJob(t, repoID)
	_ = testJob.Start()
	_ = testJob.Complete(100, 500)
	err := jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save completed test job: %v", err)
	}
	return testJob.ID()
}

// Helper function to verify job deletion.
// Reduces validation duplication across Delete tests.
func verifyJobDeleted(t *testing.T, jobRepo *PostgreSQLIndexingJobRepository, ctx context.Context, jobID uuid.UUID) {
	deletedJob, err := jobRepo.FindByID(ctx, jobID)
	if err == nil && deletedJob != nil {
		t.Error("Expected deleted job to not be found")
	}
}

// TestIndexingJobRepository_Delete_ExistingJob tests deleting an existing job.
// This focused test verifies that existing jobs can be soft deleted successfully.
func TestIndexingJobRepository_Delete_ExistingJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	// Delete the existing job
	err := jobRepo.Delete(ctx, testJob.ID())
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify job is soft deleted
	verifyJobDeleted(t, jobRepo, ctx, testJob.ID())
}

// TestIndexingJobRepository_Delete_CompletedJob tests deleting a completed job.
// This focused test verifies that completed jobs can be soft deleted successfully.
func TestIndexingJobRepository_Delete_CompletedJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// Create a test repository first (needed for foreign key)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	ctx := context.Background()
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	// Create and save completed job
	jobID := createAndSaveCompletedTestJob(t, jobRepo, testRepo.ID(), ctx)

	// Delete the completed job
	err = jobRepo.Delete(ctx, jobID)
	// Should succeed without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify job is soft deleted
	verifyJobDeleted(t, jobRepo, ctx, jobID)
}

// TestIndexingJobRepository_Delete_NonExistentJob tests deleting a non-existent job.
// This focused test verifies that attempts to delete non-existent jobs return errors.
func TestIndexingJobRepository_Delete_NonExistentJob(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, _, ctx := setupJobForUpdate(t, pool)

	// Try to delete non-existent job
	nonExistentID := uuid.New()
	err := jobRepo.Delete(ctx, nonExistentID)

	// Should return error
	if err == nil {
		t.Error("Expected error but got none")
	}
}

// TestIndexingJobRepository_Delete_InvalidInput tests error handling for invalid inputs.
// This focused test verifies that invalid inputs are properly rejected.
func TestIndexingJobRepository_Delete_InvalidInput(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, _, ctx := setupJobForUpdate(t, pool)

	// Try to delete with nil UUID
	err := jobRepo.Delete(ctx, uuid.Nil)

	// Should return error
	if err == nil {
		t.Error("Expected error but got none")
	}
}

// TestIndexingJobRepository_ConcurrentUpdates tests concurrent job updates.
func TestIndexingJobRepository_ConcurrentUpdates(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// Create a test repository first (needed for foreign key)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	ctx := context.Background()
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	// This will fail because PostgreSQLIndexingJobRepository doesn't exist yet
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	// Create and save test indexing job
	testJob := createTestIndexingJob(t, testRepo.ID())
	_ = testJob.Start()
	err = jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save test indexing job: %v", err)
	}

	// Test concurrent progress updates
	const numGoroutines = 10
	errCh := make(chan error, numGoroutines)
	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer func() { done <- true }()

			// Get fresh copy of job
			job, err := jobRepo.FindByID(ctx, testJob.ID())
			if err != nil {
				errCh <- err
				return
			}

			// Update progress with different values
			job.UpdateProgress(id*10, id*50)

			// Update the job
			if err := jobRepo.Update(ctx, job); err != nil {
				errCh <- err
				return
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		select {
		case err := <-errCh:
			t.Errorf("Concurrent update failed: %v", err)
		case <-done:
			// Success
		case <-time.After(10 * time.Second):
			t.Fatal("Test timeout")
		}
	}

	// Verify final state is consistent
	finalJob, err := jobRepo.FindByID(ctx, testJob.ID())
	if err != nil {
		t.Errorf("Failed to find job after concurrent updates: %v", err)
	}

	if finalJob.Status() != valueobject.JobStatusRunning {
		t.Errorf("Expected job to still be running, got %s", finalJob.Status())
	}
}

// setupTestRepositoryWithJobs creates a test repository and saves the specified number of jobs.
// This helper reduces setup duplication across focused tests.
func setupTestRepositoryWithJobs(
	t *testing.T,
	jobRepo *PostgreSQLIndexingJobRepository,
	repoRepo *PostgreSQLRepositoryRepository,
	numJobs int,
) (*entity.Repository, []*entity.IndexingJob) {
	ctx := context.Background()
	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	jobs := make([]*entity.IndexingJob, numJobs)
	for i := range numJobs {
		job := createTestIndexingJob(t, testRepo.ID())
		err := jobRepo.Save(ctx, job)
		if err != nil {
			t.Fatalf("Failed to save test job %d: %v", i, err)
		}
		jobs[i] = job
	}

	return testRepo, jobs
}

// validateJobResults validates that returned jobs match expectations.
// This helper reduces validation duplication across focused tests.
func validateJobResults(t *testing.T, jobs []*entity.IndexingJob, expectedRepoID uuid.UUID, expectedCount int) {
	if len(jobs) != expectedCount {
		t.Errorf("Expected %d jobs, got %d", expectedCount, len(jobs))
	}

	for i, job := range jobs {
		if job == nil {
			t.Errorf("Job at index %d is nil", i)
			continue
		}
		if job.ID() == uuid.Nil {
			t.Errorf("Job at index %d has nil ID", i)
		}
		if job.RepositoryID() != expectedRepoID {
			t.Errorf("Job at index %d has wrong repository ID %s, expected %s",
				i, job.RepositoryID(), expectedRepoID)
		}
	}
}

// TestIndexingJobRepository_FindByRepositoryID_BasicRetrieval tests basic job retrieval functionality.
// This focused test verifies that jobs can be retrieved for a repository with basic pagination.
func TestIndexingJobRepository_FindByRepositoryID_BasicRetrieval(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	// Setup: Create repository with 5 test jobs
	testRepo, _ := setupTestRepositoryWithJobs(t, jobRepo, repoRepo, 5)
	ctx := context.Background()

	// Test: Retrieve all jobs with standard pagination
	filters := outbound.IndexingJobFilters{
		Limit:  10,
		Offset: 0,
	}

	jobs, total, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), filters)
	// Verify: Should return all 5 jobs successfully
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if total != 5 {
		t.Errorf("Expected total count 5, got %d", total)
	}
	validateJobResults(t, jobs, testRepo.ID(), 5)
}

// TestIndexingJobRepository_FindByRepositoryID_Pagination tests pagination functionality.
// This focused test verifies limit, offset, and total count calculations work correctly.
func TestIndexingJobRepository_FindByRepositoryID_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	// Setup: Create repository with 15 test jobs for pagination testing
	testRepo, _ := setupTestRepositoryWithJobs(t, jobRepo, repoRepo, 15)
	ctx := context.Background()

	tests := []struct {
		name          string
		limit         int
		offset        int
		expectedCount int
		expectedTotal int
	}{
		{
			name:          "First page with limit 10",
			limit:         10,
			offset:        0,
			expectedCount: 10,
			expectedTotal: 15,
		},
		{
			name:          "Second page with limit 10",
			limit:         10,
			offset:        10,
			expectedCount: 5,
			expectedTotal: 15,
		},
		{
			name:          "Small limit",
			limit:         3,
			offset:        0,
			expectedCount: 3,
			expectedTotal: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters := outbound.IndexingJobFilters{
				Limit:  tt.limit,
				Offset: tt.offset,
			}

			jobs, total, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), filters)
			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if total != tt.expectedTotal {
				t.Errorf("Expected total count %d, got %d", tt.expectedTotal, total)
			}
			validateJobResults(t, jobs, testRepo.ID(), tt.expectedCount)
		})
	}
}

// TestIndexingJobRepository_FindByRepositoryID_RepositoryIsolation tests repository isolation.
// This focused test verifies that jobs from different repositories don't interfere with each other.
func TestIndexingJobRepository_FindByRepositoryID_RepositoryIsolation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	// Setup: Create two repositories with different numbers of jobs
	testRepo1, _ := setupTestRepositoryWithJobs(t, jobRepo, repoRepo, 7)
	testRepo2, _ := setupTestRepositoryWithJobs(t, jobRepo, repoRepo, 3)

	filters := outbound.IndexingJobFilters{
		Limit:  10,
		Offset: 0,
	}

	// Test: Query jobs for repository 1
	jobs1, total1, err1 := jobRepo.FindByRepositoryID(ctx, testRepo1.ID(), filters)

	// Test: Query jobs for repository 2
	jobs2, total2, err2 := jobRepo.FindByRepositoryID(ctx, testRepo2.ID(), filters)

	// Verify: Repository 1 should return only its 7 jobs
	if err1 != nil {
		t.Errorf("Repository 1 query failed: %v", err1)
	}
	if total1 != 7 {
		t.Errorf("Repository 1 expected total 7, got %d", total1)
	}
	validateJobResults(t, jobs1, testRepo1.ID(), 7)

	// Verify: Repository 2 should return only its 3 jobs
	if err2 != nil {
		t.Errorf("Repository 2 query failed: %v", err2)
	}
	if total2 != 3 {
		t.Errorf("Repository 2 expected total 3, got %d", total2)
	}
	validateJobResults(t, jobs2, testRepo2.ID(), 3)
}

// TestIndexingJobRepository_FindByRepositoryID_InvalidParameters tests error handling for invalid parameters.
// This focused test verifies that invalid input parameters are properly rejected.
func TestIndexingJobRepository_FindByRepositoryID_InvalidParameters(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	// Setup: Create a test repository for valid repository ID comparisons
	testRepo, _ := setupTestRepositoryWithJobs(t, jobRepo, repoRepo, 1)

	tests := []struct {
		name         string
		repositoryID uuid.UUID
		filters      outbound.IndexingJobFilters
	}{
		{
			name:         "Nil repository ID should return error",
			repositoryID: uuid.Nil,
			filters: outbound.IndexingJobFilters{
				Limit:  10,
				Offset: 0,
			},
		},
		{
			name:         "Negative offset should return error",
			repositoryID: testRepo.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  10,
				Offset: -1,
			},
		},
		{
			name:         "Zero limit should return error",
			repositoryID: testRepo.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  0,
				Offset: 0,
			},
		},
		{
			name:         "Negative limit should return error",
			repositoryID: testRepo.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  -1,
				Offset: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, total, err := jobRepo.FindByRepositoryID(ctx, tt.repositoryID, tt.filters)

			// Verify: Should return error and no results
			if err == nil {
				t.Error("Expected error but got none")
			}
			if jobs != nil {
				t.Error("Expected nil jobs on error")
			}
			if total != 0 {
				t.Errorf("Expected total count 0 on error, got %d", total)
			}
		})
	}
}

// TestIndexingJobRepository_FindByRepositoryID_EmptyResults tests behavior with non-existent repositories.
// This focused test verifies that queries for non-existent repositories return empty results gracefully.
func TestIndexingJobRepository_FindByRepositoryID_EmptyResults(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	// Setup: Create a repository with jobs, but query for a different one
	_, _ = setupTestRepositoryWithJobs(t, jobRepo, repoRepo, 5)
	nonExistentRepoID := uuid.New()

	filters := outbound.IndexingJobFilters{
		Limit:  10,
		Offset: 0,
	}

	// Test: Query for non-existent repository
	jobs, total, err := jobRepo.FindByRepositoryID(ctx, nonExistentRepoID, filters)
	// Verify: Should return empty results without error
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if total != 0 {
		t.Errorf("Expected total count 0, got %d", total)
	}
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs, got %d", len(jobs))
	}
	if jobs == nil {
		t.Error("Expected empty slice, not nil")
	}
}

// TestIndexingJobRepository_StatusTransitions tests job status transitions are preserved.
func TestIndexingJobRepository_StatusTransitions(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// Create a test repository first (needed for foreign key)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	ctx := context.Background()
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	// This will fail because PostgreSQLIndexingJobRepository doesn't exist yet
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	// Test complete job lifecycle
	job := createTestIndexingJob(t, testRepo.ID())

	// Save initial pending job
	err = jobRepo.Save(ctx, job)
	if err != nil {
		t.Fatalf("Failed to save initial job: %v", err)
	}

	// Start the job
	_ = job.Start()
	err = jobRepo.Update(ctx, job)
	if err != nil {
		t.Errorf("Failed to update job to running: %v", err)
	}

	// Verify timestamps are set correctly
	runningJob, err := jobRepo.FindByID(ctx, job.ID())
	if err != nil {
		t.Errorf("Failed to find running job: %v", err)
	}

	if runningJob.StartedAt() == nil {
		t.Error("Expected StartedAt to be set for running job")
	}

	if runningJob.CompletedAt() != nil {
		t.Error("Expected CompletedAt to be nil for running job")
	}

	// Complete the job
	_ = job.Complete(100, 500)
	err = jobRepo.Update(ctx, job)
	if err != nil {
		t.Errorf("Failed to update job to completed: %v", err)
	}

	// Verify completion data
	completedJob, err := jobRepo.FindByID(ctx, job.ID())
	if err != nil {
		t.Errorf("Failed to find completed job: %v", err)
	}

	if completedJob.CompletedAt() == nil {
		t.Error("Expected CompletedAt to be set for completed job")
	}

	if completedJob.FilesProcessed() != 100 {
		t.Errorf("Expected 100 files processed, got %d", completedJob.FilesProcessed())
	}

	if completedJob.ChunksCreated() != 500 {
		t.Errorf("Expected 500 chunks created, got %d", completedJob.ChunksCreated())
	}

	if !completedJob.IsTerminal() {
		t.Error("Expected completed job to be in terminal state")
	}
}
