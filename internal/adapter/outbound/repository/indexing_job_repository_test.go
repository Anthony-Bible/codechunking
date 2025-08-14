package repository

import (
	"context"
	"testing"
	"time"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
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

// TestIndexingJobRepository_FindByID tests finding indexing jobs by ID.
func TestIndexingJobRepository_FindByID(t *testing.T) {
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
	err = jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save test indexing job: %v", err)
	}

	tests := []struct {
		name        string
		id          uuid.UUID
		expectFound bool
		expectError bool
	}{
		{
			name:        "Existing indexing job ID should return job",
			id:          testJob.ID(),
			expectFound: true,
			expectError: false,
		},
		{
			name:        "Non-existing indexing job ID should return not found",
			id:          uuid.New(),
			expectFound: false,
			expectError: false,
		},
		{
			name:        "Nil UUID should return error",
			id:          uuid.Nil,
			expectFound: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundJob, err := jobRepo.FindByID(ctx, tt.id)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if foundJob != nil {
					t.Error("Expected nil job on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				if tt.expectFound {
					if foundJob == nil {
						t.Error("Expected job but got nil")
					} else {
						if foundJob.ID() != tt.id {
							t.Errorf("Expected job ID %s, got %s", tt.id, foundJob.ID())
						}
						if foundJob.RepositoryID() != testJob.RepositoryID() {
							t.Errorf("Expected repository ID %s, got %s", testJob.RepositoryID(), foundJob.RepositoryID())
						}
						if foundJob.Status() != testJob.Status() {
							t.Errorf("Expected status %s, got %s", testJob.Status(), foundJob.Status())
						}
					}
				} else {
					if foundJob != nil {
						t.Error("Expected nil job but got one")
					}
				}
			}
		})
	}
}

// TestIndexingJobRepository_FindByRepositoryID tests finding indexing jobs by repository ID with filters.
func TestIndexingJobRepository_FindByRepositoryID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// Create test repositories
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	testRepo1 := createTestRepository(t)
	uniqueID2 := uuid.New().String()
	testRepo2URL, _ := valueobject.NewRepositoryURL("https://github.com/test/repo2-" + uniqueID2)
	testRepo2 := entity.NewRepository(testRepo2URL, "Test Repository 2", nil, nil)

	ctx := context.Background()
	err := repoRepo.Save(ctx, testRepo1)
	if err != nil {
		t.Fatalf("Failed to save test repository 1: %v", err)
	}
	err = repoRepo.Save(ctx, testRepo2)
	if err != nil {
		t.Fatalf("Failed to save test repository 2: %v", err)
	}

	// This will fail because PostgreSQLIndexingJobRepository doesn't exist yet
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	// Create multiple indexing jobs for repository 1
	for i := range 15 {
		job := createTestIndexingJob(t, testRepo1.ID())

		// Set different statuses for testing filters
		if i < 5 {
			_ = job.Start()
			_ = job.Complete(100, 500)
		} else if i < 10 {
			_ = job.Start()
			_ = job.Fail("Test error message")
		}
		// Rest remain as pending

		err := jobRepo.Save(ctx, job)
		if err != nil {
			t.Fatalf("Failed to save test job %d for repo 1: %v", i, err)
		}
	}

	// Create a few jobs for repository 2
	for i := range 3 {
		job := createTestIndexingJob(t, testRepo2.ID())
		err := jobRepo.Save(ctx, job)
		if err != nil {
			t.Fatalf("Failed to save test job %d for repo 2: %v", i, err)
		}
	}

	tests := []struct {
		name          string
		repositoryID  uuid.UUID
		filters       outbound.IndexingJobFilters
		expectedCount int
		expectedTotal int
		expectError   bool
	}{
		{
			name:         "Repository 1 with no filters should return all jobs with pagination",
			repositoryID: testRepo1.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 10,
			expectedTotal: 15,
			expectError:   false,
		},
		{
			name:         "Repository 1 second page should return remaining jobs",
			repositoryID: testRepo1.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  10,
				Offset: 10,
			},
			expectedCount: 5,
			expectedTotal: 15,
			expectError:   false,
		},
		{
			name:         "Repository 2 should return only its jobs",
			repositoryID: testRepo2.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 3,
			expectedTotal: 3,
			expectError:   false,
		},
		{
			name:         "Non-existing repository should return empty results",
			repositoryID: uuid.New(),
			filters: outbound.IndexingJobFilters{
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectError:   false,
		},
		{
			name:         "Small limit should limit results",
			repositoryID: testRepo1.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  3,
				Offset: 0,
			},
			expectedCount: 3,
			expectedTotal: 15,
			expectError:   false,
		},
		{
			name:         "Negative offset should return error",
			repositoryID: testRepo1.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  10,
				Offset: -1,
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectError:   true,
		},
		{
			name:         "Zero or negative limit should return error",
			repositoryID: testRepo1.ID(),
			filters: outbound.IndexingJobFilters{
				Limit:  0,
				Offset: 0,
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectError:   true,
		},
		{
			name:         "Nil repository ID should return error",
			repositoryID: uuid.Nil,
			filters: outbound.IndexingJobFilters{
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, total, err := jobRepo.FindByRepositoryID(ctx, tt.repositoryID, tt.filters)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if jobs != nil {
					t.Error("Expected nil jobs on error")
				}
				if total != 0 {
					t.Errorf("Expected total count 0 on error, got %d", total)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if len(jobs) != tt.expectedCount {
					t.Errorf("Expected %d jobs, got %d", tt.expectedCount, len(jobs))
				}
				if total != tt.expectedTotal {
					t.Errorf("Expected total count %d, got %d", tt.expectedTotal, total)
				}

				// Verify jobs are valid entities and belong to correct repository
				for i, job := range jobs {
					if job == nil {
						t.Errorf("Job at index %d is nil", i)
					}
					if job.ID() == uuid.Nil {
						t.Errorf("Job at index %d has nil ID", i)
					}
					if job.RepositoryID() != tt.repositoryID {
						t.Errorf("Job at index %d has wrong repository ID %s, expected %s",
							i, job.RepositoryID(), tt.repositoryID)
					}
				}
			}
		})
	}
}

// TestIndexingJobRepository_Update tests updating indexing jobs.
func TestIndexingJobRepository_Update(t *testing.T) {
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
	err = jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save test indexing job: %v", err)
	}

	tests := []struct {
		name        string
		setupJob    func(*entity.IndexingJob)
		expectError bool
	}{
		{
			name: "Start job should succeed",
			setupJob: func(job *entity.IndexingJob) {
				_ = job.Start()
			},
			expectError: false,
		},
		{
			name: "Complete job should succeed",
			setupJob: func(job *entity.IndexingJob) {
				_ = job.Start()
				_ = job.Complete(100, 500)
			},
			expectError: false,
		},
		{
			name: "Fail job should succeed",
			setupJob: func(job *entity.IndexingJob) {
				_ = job.Start()
				_ = job.Fail("Test error message")
			},
			expectError: false,
		},
		{
			name: "Cancel job should succeed",
			setupJob: func(job *entity.IndexingJob) {
				_ = job.Cancel()
			},
			expectError: false,
		},
		{
			name: "Update progress should succeed",
			setupJob: func(job *entity.IndexingJob) {
				_ = job.Start()
				job.UpdateProgress(50, 250)
			},
			expectError: false,
		},
		{
			name: "Archive job should succeed",
			setupJob: func(job *entity.IndexingJob) {
				_ = job.Start()
				_ = job.Complete(100, 500)
				_ = job.Archive()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get a fresh copy of the job
			jobToUpdate, err := jobRepo.FindByID(ctx, testJob.ID())
			if err != nil {
				t.Fatalf("Failed to find job for update test: %v", err)
			}

			originalUpdatedAt := jobToUpdate.UpdatedAt()

			// Apply the test modifications
			tt.setupJob(jobToUpdate)

			// Update the job
			err = jobRepo.Update(ctx, jobToUpdate)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Verify the update was persisted
				updatedJob, err := jobRepo.FindByID(ctx, jobToUpdate.ID())
				if err != nil {
					t.Errorf("Failed to find updated job: %v", err)
				}

				// For archived jobs, FindByID returns nil (soft deleted), so skip verification
				if updatedJob == nil {
					if !jobToUpdate.IsDeleted() {
						t.Error("Job not found but should not be deleted")
					}
					return
				}

				if updatedJob.UpdatedAt().Equal(originalUpdatedAt) {
					t.Error("Expected UpdatedAt timestamp to be changed")
				}

				if updatedJob.Status() != jobToUpdate.Status() {
					t.Errorf("Expected updated status %s, got %s", jobToUpdate.Status(), updatedJob.Status())
				}

				if updatedJob.FilesProcessed() != jobToUpdate.FilesProcessed() {
					t.Errorf("Expected updated files processed %d, got %d",
						jobToUpdate.FilesProcessed(), updatedJob.FilesProcessed())
				}

				if updatedJob.ChunksCreated() != jobToUpdate.ChunksCreated() {
					t.Errorf("Expected updated chunks created %d, got %d",
						jobToUpdate.ChunksCreated(), updatedJob.ChunksCreated())
				}

				// Check error message if job failed
				if jobToUpdate.Status() == valueobject.JobStatusFailed {
					if updatedJob.ErrorMessage() == nil || *updatedJob.ErrorMessage() != *jobToUpdate.ErrorMessage() {
						t.Errorf("Expected error message to be preserved")
					}
				}
			}
		})
	}
}

// TestIndexingJobRepository_Delete tests soft deleting indexing jobs.
func TestIndexingJobRepository_Delete(t *testing.T) {
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
		setupJob    func() uuid.UUID
		expectError bool
	}{
		{
			name: "Delete existing job should succeed",
			setupJob: func() uuid.UUID {
				testJob := createTestIndexingJob(t, testRepo.ID())
				err := jobRepo.Save(ctx, testJob)
				if err != nil {
					t.Fatalf("Failed to save test job: %v", err)
				}
				return testJob.ID()
			},
			expectError: false,
		},
		{
			name: "Delete completed job should succeed",
			setupJob: func() uuid.UUID {
				testJob := createTestIndexingJob(t, testRepo.ID())
				_ = testJob.Start()
				_ = testJob.Complete(100, 500)
				err := jobRepo.Save(ctx, testJob)
				if err != nil {
					t.Fatalf("Failed to save test job: %v", err)
				}
				return testJob.ID()
			},
			expectError: false,
		},
		{
			name: "Delete non-existing job should return error",
			setupJob: func() uuid.UUID {
				return uuid.New()
			},
			expectError: true,
		},
		{
			name: "Delete with nil UUID should return error",
			setupJob: func() uuid.UUID {
				return uuid.Nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobID := tt.setupJob()

			err := jobRepo.Delete(ctx, jobID)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Verify job is soft deleted (not returned by normal queries)
				deletedJob, err := jobRepo.FindByID(ctx, jobID)
				if err == nil && deletedJob != nil {
					t.Error("Expected deleted job to not be found")
				}
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
