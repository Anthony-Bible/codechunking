package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// createTestIndexingJob creates a test indexing job entity.
func createTestIndexingJob(_ *testing.T, repositoryID uuid.UUID) *entity.IndexingJob {
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

// verifyFoundJob verifies a found job matches expected field values.
func verifyFoundJob(t *testing.T, foundJob *entity.IndexingJob, expectedJob *entity.IndexingJob) {
	t.Helper()
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

// TestIndexingJobRepository_FindByID tests retrieving jobs by ID for existing, missing, and invalid inputs.
func TestIndexingJobRepository_FindByID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
		wantNil bool
	}{
		{
			name:    "existing job returns with correct fields",
			id:      testJob.ID(),
			wantErr: false,
			wantNil: false,
		},
		{
			name:    "non-existent job returns nil without error",
			id:      uuid.New(),
			wantErr: false,
			wantNil: true,
		},
		{
			name:    "nil UUID returns error",
			id:      uuid.Nil,
			wantErr: true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundJob, err := jobRepo.FindByID(ctx, tt.id)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if tt.wantNil && foundJob != nil {
				t.Error("expected nil job but got one")
			}
			if !tt.wantNil {
				if foundJob == nil {
					t.Fatal("expected job but got nil")
				}
				verifyFoundJob(t, foundJob, testJob)
			}
		})
	}
}

// setupJobForUpdate creates a test repository and a saved pending job, returning the repo, job, and context.
func setupJobForUpdate(
	t *testing.T,
	pool *pgxpool.Pool,
) (*PostgreSQLIndexingJobRepository, *entity.IndexingJob, context.Context) {
	t.Helper()
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	ctx := context.Background()
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	testJob := createTestIndexingJob(t, testRepo.ID())
	err = jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save test indexing job: %v", err)
	}

	return jobRepo, testJob, ctx
}

// TestIndexingJobRepository_Update tests all update scenarios: state transitions, progress, archival, and error cases.
func TestIndexingJobRepository_Update(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	ctx := context.Background()
	if err := repoRepo.Save(ctx, testRepo); err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	// newSavedJob creates and saves a fresh pending job, returning a DB-fetched copy.
	newSavedJob := func(t *testing.T) *entity.IndexingJob {
		t.Helper()
		j := createTestIndexingJob(t, testRepo.ID())
		if err := jobRepo.Save(ctx, j); err != nil {
			t.Fatalf("Failed to save test job: %v", err)
		}
		fetched, err := jobRepo.FindByID(ctx, j.ID())
		if err != nil || fetched == nil {
			t.Fatalf("Failed to fetch saved job: %v", err)
		}
		return fetched
	}

	tests := []struct {
		name    string
		prepare func(t *testing.T) *entity.IndexingJob
		wantErr bool
		verify  func(t *testing.T, id uuid.UUID)
	}{
		{
			name: "start pending job transitions to running with StartedAt set",
			prepare: func(t *testing.T) *entity.IndexingJob {
				t.Helper()
				j := newSavedJob(t)
				if err := j.Start(); err != nil {
					t.Fatalf("Failed to start job: %v", err)
				}
				return j
			},
			verify: func(t *testing.T, id uuid.UUID) {
				t.Helper()
				j, _ := jobRepo.FindByID(ctx, id)
				if j.Status() != valueobject.JobStatusRunning {
					t.Errorf("expected running, got %s", j.Status())
				}
				if j.StartedAt() == nil {
					t.Error("expected StartedAt to be set")
				}
			},
		},
		{
			name: "complete running job records files, chunks, and CompletedAt",
			prepare: func(t *testing.T) *entity.IndexingJob {
				t.Helper()
				j := newSavedJob(t)
				_ = j.Start()
				if err := j.Complete(100, 500); err != nil {
					t.Fatalf("Failed to complete job: %v", err)
				}
				return j
			},
			verify: func(t *testing.T, id uuid.UUID) {
				t.Helper()
				j, _ := jobRepo.FindByID(ctx, id)
				if j.Status() != valueobject.JobStatusCompleted {
					t.Errorf("expected completed, got %s", j.Status())
				}
				if j.FilesProcessed() != 100 {
					t.Errorf("expected 100 files processed, got %d", j.FilesProcessed())
				}
				if j.ChunksCreated() != 500 {
					t.Errorf("expected 500 chunks created, got %d", j.ChunksCreated())
				}
				if j.CompletedAt() == nil {
					t.Error("expected CompletedAt to be set")
				}
			},
		},
		{
			name: "fail running job stores error message and CompletedAt",
			prepare: func(t *testing.T) *entity.IndexingJob {
				t.Helper()
				j := newSavedJob(t)
				_ = j.Start()
				if err := j.Fail("test error"); err != nil {
					t.Fatalf("Failed to fail job: %v", err)
				}
				return j
			},
			verify: func(t *testing.T, id uuid.UUID) {
				t.Helper()
				j, _ := jobRepo.FindByID(ctx, id)
				if j.Status() != valueobject.JobStatusFailed {
					t.Errorf("expected failed, got %s", j.Status())
				}
				if j.ErrorMessage() == nil || *j.ErrorMessage() != "test error" {
					t.Errorf("expected error message 'test error', got %v", j.ErrorMessage())
				}
				if j.CompletedAt() == nil {
					t.Error("expected CompletedAt to be set")
				}
			},
		},
		{
			name: "cancel pending job sets status and CompletedAt",
			prepare: func(t *testing.T) *entity.IndexingJob {
				t.Helper()
				j := newSavedJob(t)
				if err := j.Cancel(); err != nil {
					t.Fatalf("Failed to cancel job: %v", err)
				}
				return j
			},
			verify: func(t *testing.T, id uuid.UUID) {
				t.Helper()
				j, _ := jobRepo.FindByID(ctx, id)
				if j.Status() != valueobject.JobStatusCancelled {
					t.Errorf("expected cancelled, got %s", j.Status())
				}
				if j.CompletedAt() == nil {
					t.Error("expected CompletedAt to be set")
				}
			},
		},
		{
			name: "progress update persists files and chunks while staying running",
			prepare: func(t *testing.T) *entity.IndexingJob {
				t.Helper()
				j := newSavedJob(t)
				_ = j.Start()
				j.UpdateProgress(50, 250)
				return j
			},
			verify: func(t *testing.T, id uuid.UUID) {
				t.Helper()
				j, _ := jobRepo.FindByID(ctx, id)
				if j.Status() != valueobject.JobStatusRunning {
					t.Errorf("expected running, got %s", j.Status())
				}
				if j.FilesProcessed() != 50 {
					t.Errorf("expected 50 files processed, got %d", j.FilesProcessed())
				}
				if j.ChunksCreated() != 250 {
					t.Errorf("expected 250 chunks created, got %d", j.ChunksCreated())
				}
			},
		},
		{
			name: "archive completed job makes it unreachable via FindByID",
			prepare: func(t *testing.T) *entity.IndexingJob {
				t.Helper()
				j := newSavedJob(t)
				_ = j.Start()
				_ = j.Complete(100, 500)
				if err := j.Archive(); err != nil {
					t.Fatalf("Failed to archive job: %v", err)
				}
				return j
			},
			verify: func(t *testing.T, id uuid.UUID) {
				t.Helper()
				found, err := jobRepo.FindByID(ctx, id)
				if err != nil {
					t.Errorf("expected no error when finding archived job, got: %v", err)
				}
				if found != nil {
					t.Error("expected archived job to not be found (soft deleted)")
				}
			},
		},
		{
			name: "updating non-existent job returns error",
			prepare: func(t *testing.T) *entity.IndexingJob {
				t.Helper()
				j := createTestIndexingJob(t, uuid.New())
				_ = j.Start()
				return j
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := tt.prepare(t)

			err := jobRepo.Update(ctx, job)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("expected no error but got: %v", err)
				return
			}

			if tt.verify != nil {
				tt.verify(t, job.ID())
			}
		})
	}
}

// createAndSaveTestJob creates and saves a pending test job, returning its ID.
func createAndSaveTestJob(
	ctx context.Context,
	t *testing.T,
	jobRepo *PostgreSQLIndexingJobRepository,
	repoID uuid.UUID,
) uuid.UUID {
	t.Helper()
	testJob := createTestIndexingJob(t, repoID)
	err := jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save test job: %v", err)
	}
	return testJob.ID()
}

// createAndSaveCompletedTestJob creates and saves a completed test job, returning its ID.
func createAndSaveCompletedTestJob(
	ctx context.Context,
	t *testing.T,
	jobRepo *PostgreSQLIndexingJobRepository,
	repoID uuid.UUID,
) uuid.UUID {
	t.Helper()
	testJob := createTestIndexingJob(t, repoID)
	_ = testJob.Start()
	_ = testJob.Complete(100, 500)
	err := jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save completed test job: %v", err)
	}
	return testJob.ID()
}

// verifyJobDeleted asserts that a job is no longer retrievable after deletion.
func verifyJobDeleted(ctx context.Context, t *testing.T, jobRepo *PostgreSQLIndexingJobRepository, jobID uuid.UUID) {
	t.Helper()
	deletedJob, err := jobRepo.FindByID(ctx, jobID)
	if err == nil && deletedJob != nil {
		t.Error("Expected deleted job to not be found")
	}
}

// TestIndexingJobRepository_Delete tests soft deletion across pending, completed, missing, and invalid inputs.
func TestIndexingJobRepository_Delete(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo, testJob, ctx := setupJobForUpdate(t, pool)
	completedJobID := createAndSaveCompletedTestJob(ctx, t, jobRepo, testJob.RepositoryID())

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
	}{
		{
			name:    "existing pending job is soft deleted",
			id:      testJob.ID(),
			wantErr: false,
		},
		{
			name:    "existing completed job is soft deleted",
			id:      completedJobID,
			wantErr: false,
		},
		{
			name:    "non-existent job returns error",
			id:      uuid.New(),
			wantErr: true,
		},
		{
			name:    "nil UUID returns error",
			id:      uuid.Nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jobRepo.Delete(ctx, tt.id)
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if !tt.wantErr {
				verifyJobDeleted(ctx, t, jobRepo, tt.id)
			}
		})
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
func setupTestRepositoryWithJobs(
	ctx context.Context,
	t *testing.T,
	jobRepo *PostgreSQLIndexingJobRepository,
	repoRepo *PostgreSQLRepositoryRepository,
	numJobs int,
) *entity.Repository {
	t.Helper()
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

	return testRepo
}

// validateJobResults validates that returned jobs match expectations.
func validateJobResults(t *testing.T, jobs []*entity.IndexingJob, expectedRepoID uuid.UUID, expectedCount int) {
	t.Helper()
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
func TestIndexingJobRepository_FindByRepositoryID_BasicRetrieval(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	testRepo := setupTestRepositoryWithJobs(ctx, t, jobRepo, repoRepo, 5)

	filters := outbound.IndexingJobFilters{
		Limit:  10,
		Offset: 0,
	}

	jobs, total, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), filters)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if total != 5 {
		t.Errorf("Expected total count 5, got %d", total)
	}
	validateJobResults(t, jobs, testRepo.ID(), 5)
}

// TestIndexingJobRepository_FindByRepositoryID_Pagination tests limit, offset, and total count calculations.
func TestIndexingJobRepository_FindByRepositoryID_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	testRepo := setupTestRepositoryWithJobs(ctx, t, jobRepo, repoRepo, 15)

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

// TestIndexingJobRepository_FindByRepositoryID_RepositoryIsolation tests that jobs from different repositories
// don't interfere with each other.
func TestIndexingJobRepository_FindByRepositoryID_RepositoryIsolation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	testRepo1 := setupTestRepositoryWithJobs(ctx, t, jobRepo, repoRepo, 7)
	testRepo2 := setupTestRepositoryWithJobs(ctx, t, jobRepo, repoRepo, 3)

	filters := outbound.IndexingJobFilters{
		Limit:  10,
		Offset: 0,
	}

	jobs1, total1, err1 := jobRepo.FindByRepositoryID(ctx, testRepo1.ID(), filters)
	jobs2, total2, err2 := jobRepo.FindByRepositoryID(ctx, testRepo2.ID(), filters)

	if err1 != nil {
		t.Errorf("Repository 1 query failed: %v", err1)
	}
	if total1 != 7 {
		t.Errorf("Repository 1 expected total 7, got %d", total1)
	}
	validateJobResults(t, jobs1, testRepo1.ID(), 7)

	if err2 != nil {
		t.Errorf("Repository 2 query failed: %v", err2)
	}
	if total2 != 3 {
		t.Errorf("Repository 2 expected total 3, got %d", total2)
	}
	validateJobResults(t, jobs2, testRepo2.ID(), 3)
}

// TestIndexingJobRepository_FindByRepositoryID_InvalidParameters tests that invalid inputs are properly rejected.
func TestIndexingJobRepository_FindByRepositoryID_InvalidParameters(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	testRepo := setupTestRepositoryWithJobs(ctx, t, jobRepo, repoRepo, 1)

	tests := []struct {
		name         string
		repositoryID uuid.UUID
		filters      outbound.IndexingJobFilters
	}{
		{
			name:         "Nil repository ID should return error",
			repositoryID: uuid.Nil,
			filters:      outbound.IndexingJobFilters{Limit: 10, Offset: 0},
		},
		{
			name:         "Negative offset should return error",
			repositoryID: testRepo.ID(),
			filters:      outbound.IndexingJobFilters{Limit: 10, Offset: -1},
		},
		{
			name:         "Zero limit should return error",
			repositoryID: testRepo.ID(),
			filters:      outbound.IndexingJobFilters{Limit: 0, Offset: 0},
		},
		{
			name:         "Negative limit should return error",
			repositoryID: testRepo.ID(),
			filters:      outbound.IndexingJobFilters{Limit: -1, Offset: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, total, err := jobRepo.FindByRepositoryID(ctx, tt.repositoryID, tt.filters)

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

// TestIndexingJobRepository_FindByRepositoryID_EmptyResults tests that queries for non-existent repositories
// return empty results gracefully.
func TestIndexingJobRepository_FindByRepositoryID_EmptyResults(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	_ = setupTestRepositoryWithJobs(ctx, t, jobRepo, repoRepo, 5)
	nonExistentRepoID := uuid.New()

	filters := outbound.IndexingJobFilters{
		Limit:  10,
		Offset: 0,
	}

	jobs, total, err := jobRepo.FindByRepositoryID(ctx, nonExistentRepoID, filters)
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

// TestIndexingJobRepository_StatusTransitions tests that a full job lifecycle is preserved end-to-end.
func TestIndexingJobRepository_StatusTransitions(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo := createTestRepository(t)
	ctx := context.Background()
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	job := createTestIndexingJob(t, testRepo.ID())

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

// TestIndexingJobRepository_FindByRepositoryID_StatusFilter tests filtering by job status.
// Verifies that only jobs with the requested status are returned, and that nil Status
// returns all jobs regardless of status (backward compatibility).
func TestIndexingJobRepository_FindByRepositoryID_StatusFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	testRepo := createTestRepository(t)
	if err := repoRepo.Save(ctx, testRepo); err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	createAndSaveTestJob(ctx, t, jobRepo, testRepo.ID())

	completedJob := createTestIndexingJob(t, testRepo.ID())
	_ = completedJob.Start()
	_ = completedJob.Complete(10, 50)
	if err := jobRepo.Save(ctx, completedJob); err != nil {
		t.Fatalf("Failed to save completed job: %v", err)
	}

	t.Run("Status filter returns only matching jobs", func(t *testing.T) {
		pendingStatus := valueobject.JobStatusPending
		jobs, total, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), outbound.IndexingJobFilters{
			Limit:  10,
			Status: &pendingStatus,
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if total != 1 {
			t.Errorf("Expected total 1, got %d", total)
		}
		if len(jobs) != 1 {
			t.Fatalf("Expected 1 job, got %d", len(jobs))
		}
		if jobs[0].Status() != valueobject.JobStatusPending {
			t.Errorf("Expected pending status, got %s", jobs[0].Status())
		}
	})

	t.Run("Nil status returns all jobs", func(t *testing.T) {
		jobs, total, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), outbound.IndexingJobFilters{
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if total != 2 {
			t.Errorf("Expected total 2, got %d", total)
		}
		if len(jobs) != 2 {
			t.Errorf("Expected 2 jobs, got %d", len(jobs))
		}
	})
}
