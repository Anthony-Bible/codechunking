package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

//nolint:gocognit // Comprehensive soft delete test requires testing many scenarios
func TestSoftDeleteFunctionality(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Repository soft delete and restore", func(t *testing.T) {
		// Create and save repository
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save repository: %v", err)
		}

		// Verify repository exists
		foundRepo, err := repoRepo.FindByID(ctx, testRepo.ID())
		if err != nil || foundRepo == nil {
			t.Error("Repository should exist before deletion")
		}

		// Soft delete repository
		err = repoRepo.Delete(ctx, testRepo.ID())
		if err != nil {
			t.Errorf("Failed to soft delete repository: %v", err)
		}

		// Verify repository is not found by normal queries
		deletedRepo, err := repoRepo.FindByID(ctx, testRepo.ID())
		if err != nil {
			t.Errorf("Expected no error for soft deleted repository: %v", err)
		}
		if deletedRepo != nil {
			t.Error("Soft deleted repository should not be returned by normal queries")
		}

		// Verify repository doesn't appear in listing
		repos, total, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 100})
		if err != nil {
			t.Errorf("Failed to list repositories: %v", err)
		}

		for _, repo := range repos {
			if repo.ID() == testRepo.ID() {
				t.Error("Soft deleted repository should not appear in listings")
			}
		}

		// Verify soft delete affects count
		if total > 0 {
			// If there are other repos, the deleted one shouldn't be counted
			found := false
			for _, repo := range repos {
				if repo.ID() == testRepo.ID() {
					found = true
					break
				}
			}
			if found {
				t.Error("Soft deleted repository should not be in results")
			}
		}
	})

	t.Run("IndexingJob soft delete", func(t *testing.T) {
		// Create repository first
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save repository: %v", err)
		}

		// Create and save indexing job
		testJob := createTestIndexingJob(t, testRepo.ID())
		err = jobRepo.Save(ctx, testJob)
		if err != nil {
			t.Fatalf("Failed to save indexing job: %v", err)
		}

		// Verify job exists
		foundJob, err := jobRepo.FindByID(ctx, testJob.ID())
		if err != nil || foundJob == nil {
			t.Error("Indexing job should exist before deletion")
		}

		// Soft delete job
		err = jobRepo.Delete(ctx, testJob.ID())
		if err != nil {
			t.Errorf("Failed to soft delete indexing job: %v", err)
		}

		// Verify job is not found by normal queries
		deletedJob, err := jobRepo.FindByID(ctx, testJob.ID())
		if err != nil {
			t.Errorf("Expected no error for soft deleted job: %v", err)
		}
		if deletedJob != nil {
			t.Error("Soft deleted job should not be returned by normal queries")
		}

		// Verify job doesn't appear in repository job listings
		jobs, total, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), outbound.IndexingJobFilters{Limit: 100})
		if err != nil {
			t.Errorf("Failed to list jobs: %v", err)
		}

		for _, job := range jobs {
			if job.ID() == testJob.ID() {
				t.Error("Soft deleted job should not appear in listings")
			}
		}

		if total > 0 {
			// If there are other jobs, the deleted one shouldn't be counted
			found := false
			for _, job := range jobs {
				if job.ID() == testJob.ID() {
					found = true
					break
				}
			}
			if found {
				t.Error("Soft deleted job should not be in results")
			}
		}
	})

	t.Run("Cascade delete behavior", func(t *testing.T) {
		// Create repository
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save repository: %v", err)
		}

		// Create multiple jobs for repository
		for i := range 3 {
			job := createTestIndexingJob(t, testRepo.ID())
			err = jobRepo.Save(ctx, job)
			if err != nil {
				t.Fatalf("Failed to save job %d: %v", i, err)
			}
		}

		// Delete repository (should cascade to jobs if implemented)
		err = repoRepo.Delete(ctx, testRepo.ID())
		if err != nil {
			t.Errorf("Failed to delete repository: %v", err)
		}

		// Check if jobs are still accessible
		remainingJobs, _, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), outbound.IndexingJobFilters{Limit: 100})
		if err != nil {
			t.Errorf("Failed to query jobs after repository deletion: %v", err)
		}

		// This depends on implementation - jobs might be cascade deleted or orphaned
		// The test verifies the behavior is consistent
		t.Logf("After repository deletion, %d jobs remain", len(remainingJobs))
	})
}

// TestRepositoryURLUniquenessConstraint tests that repositories with duplicate URLs are rejected.
func TestRepositoryURLUniquenessConstraint(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/unique-constraint")

	// Create first repository
	repo1 := entity.NewRepository(testURL, "First Repository", nil, nil)
	err := repoRepo.Save(ctx, repo1)
	if err != nil {
		t.Fatalf("Failed to save first repository: %v", err)
	}

	// Try to create second repository with same URL
	repo2 := entity.NewRepository(testURL, "Second Repository", nil, nil)
	err = repoRepo.Save(ctx, repo2)

	if err == nil {
		t.Error("Expected unique constraint violation for duplicate URL")
	}

	// Verify error contains constraint information
	if !isUniqueConstraintError(err) {
		t.Errorf("Expected unique constraint error, got: %v", err)
	}
}

// TestIndexingJobForeignKeyConstraint tests that jobs with invalid repository IDs are rejected.
func TestIndexingJobForeignKeyConstraint(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	// Try to create job with non-existent repository ID
	nonExistentRepoID := uuid.New()
	invalidJob := createTestIndexingJob(t, nonExistentRepoID)

	err := jobRepo.Save(ctx, invalidJob)

	if err == nil {
		t.Error("Expected foreign key constraint violation")
	}

	// Verify error is foreign key constraint
	if !isForeignKeyConstraintError(err) {
		t.Errorf("Expected foreign key constraint error, got: %v", err)
	}
}

// TestRepositoryURLFormatValidation tests that valid repository URLs are accepted.
func TestRepositoryURLFormatValidation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// The domain layer should prevent invalid URLs, but test DB handles whatever is passed
	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Errorf("Valid repository should save successfully: %v", err)
	}
}

// TestRepositoryTimestampInitialization tests that timestamps are properly set on creation.
func TestRepositoryTimestampInitialization(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Test that timestamps are properly handled
	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save repository: %v", err)
	}

	// Verify created_at and updated_at are set
	savedRepo, err := repoRepo.FindByID(ctx, testRepo.ID())
	if err != nil {
		t.Fatalf("Failed to find saved repository: %v", err)
	}

	assertRepositoryTimestampsInitialized(t, savedRepo)
}

// TestRepositoryTimestampUpdates tests that timestamps are properly updated on modification.
func TestRepositoryTimestampUpdates(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create and save repository
	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save repository: %v", err)
	}

	savedRepo, err := repoRepo.FindByID(ctx, testRepo.ID())
	if err != nil {
		t.Fatalf("Failed to find saved repository: %v", err)
	}

	// Test update timestamp changes
	originalUpdatedAt := savedRepo.UpdatedAt()
	time.Sleep(1 * time.Millisecond) // Ensure timestamp difference

	savedRepo.UpdateName("Updated Name")
	err = repoRepo.Update(ctx, savedRepo)
	if err != nil {
		t.Errorf("Failed to update repository: %v", err)
	}

	updatedRepo, err := repoRepo.FindByID(ctx, savedRepo.ID())
	if err != nil {
		t.Errorf("Failed to find updated repository: %v", err)
	}

	if !updatedRepo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated on modification")
	}
}

// assertRepositoryTimestampsInitialized validates that repository timestamps are properly set.
func assertRepositoryTimestampsInitialized(t *testing.T, repo *entity.Repository) {
	t.Helper()

	if repo.CreatedAt().IsZero() {
		t.Error("CreatedAt should be set")
	}

	if repo.UpdatedAt().IsZero() {
		t.Error("UpdatedAt should be set")
	}

	if repo.CreatedAt().After(time.Now()) {
		t.Error("CreatedAt should not be in the future")
	}
}

// TestPaginationAndPerformance tests pagination functionality and basic performance characteristics.
//
//nolint:gocognit // Comprehensive pagination and performance test requires testing many scenarios
func TestPaginationAndPerformance(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	// Create test data set
	const numRepositories = 50
	const numJobsPerRepo = 5

	t.Run("Setup test data", func(t *testing.T) {
		start := time.Now()

		for i := range numRepositories {
			uniqueID := uuid.New().String()
			testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/repo-" + uniqueID)
			testRepo := entity.NewRepository(
				testURL,
				"Repository "+string(rune('A'+i%26))+string(rune('0'+i/26)),
				nil,
				nil,
			)

			// Set different statuses for variety
			switch i % 4 {
			case 1:
				_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCompleted)
			case 2:
				_ = testRepo.UpdateStatus(valueobject.RepositoryStatusFailed)
			case 3:
				_ = testRepo.UpdateStatus(valueobject.RepositoryStatusArchived)
				_ = testRepo.Archive()
			}

			err := repoRepo.Save(ctx, testRepo)
			if err != nil {
				t.Fatalf("Failed to save repository %d: %v", i, err)
			}

			// Create jobs for this repository (except archived ones)
			if !testRepo.IsDeleted() {
				for j := range numJobsPerRepo {
					job := createTestIndexingJob(t, testRepo.ID())

					// Set different job statuses
					switch j % 3 {
					case 1:
						_ = job.Start()
						_ = job.Complete(100, 500)
					case 2:
						_ = job.Start()
						_ = job.Fail("Test error")
					}

					err := jobRepo.Save(ctx, job)
					if err != nil {
						t.Fatalf("Failed to save job %d for repo %d: %v", j, i, err)
					}
				}
			}
		}

		duration := time.Since(start)
		t.Logf("Created %d repositories with jobs in %v", numRepositories, duration)

		if duration > 30*time.Second {
			t.Errorf("Data creation took too long: %v (expected < 30s)", duration)
		}
	})

	t.Run("Repository pagination", func(t *testing.T) {
		// First, let's check if we have any repositories at all
		repos, total, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Limit:  1,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("Failed to check for repositories: %v", err)
		}

		if len(repos) == 0 && total == 0 {
			t.Skip("No repositories found - setup may not have completed or repositories were deleted")
		}

		pageSize := 10
		var allRepos []*entity.Repository
		offset := 0

		start := time.Now()

		for {
			repos, total, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
				Limit:  pageSize,
				Offset: offset,
			})
			if err != nil {
				t.Fatalf("Failed to fetch page at offset %d: %v", offset, err)
			}

			if len(repos) == 0 {
				break
			}

			// Verify total count is consistent
			if total < 0 {
				t.Error("Total count should be non-negative")
			}

			// If we got repos but total is 0, that's inconsistent
			if len(repos) > 0 && total == 0 {
				t.Error("Got repositories but total count is 0")
			}

			// Check if we got results beyond total count BEFORE updating offset
			if offset >= total {
				t.Errorf("Got %d results at offset %d which is >= total count %d", len(repos), offset, total)
			}

			allRepos = append(allRepos, repos...)
			offset += pageSize

			// Safety break
			if len(allRepos) > numRepositories*2 {
				t.Fatal("Infinite pagination loop detected")
			}
		}

		duration := time.Since(start)
		t.Logf("Paginated through %d repositories in %v", len(allRepos), duration)

		// Verify no duplicates in pagination
		seenIDs := make(map[uuid.UUID]bool)
		for _, repo := range allRepos {
			if seenIDs[repo.ID()] {
				t.Errorf("Duplicate repository ID found in pagination: %s", repo.ID())
			}
			seenIDs[repo.ID()] = true
		}

		// Performance check - should be reasonably fast
		if duration > 5*time.Second {
			t.Errorf("Pagination took too long: %v (expected < 5s)", duration)
		}
	})

	t.Run("Repository filtering performance", func(t *testing.T) {
		start := time.Now()

		// Test status filtering
		completedRepos, total, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Status: &[]valueobject.RepositoryStatus{valueobject.RepositoryStatusCompleted}[0],
			Limit:  100,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("Failed to filter by status: %v", err)
		}

		duration := time.Since(start)
		t.Logf("Filtered %d completed repositories out of %d total in %v", len(completedRepos), total, duration)

		// Verify all returned repositories have correct status
		for _, repo := range completedRepos {
			if repo.Status() != valueobject.RepositoryStatusCompleted {
				t.Errorf("Repository %s has status %s, expected %s",
					repo.ID(), repo.Status(), valueobject.RepositoryStatusCompleted)
			}
		}

		// Performance check
		if duration > 2*time.Second {
			t.Errorf("Status filtering took too long: %v (expected < 2s)", duration)
		}
	})

	t.Run("Job pagination by repository", func(t *testing.T) {
		// Get a repository with jobs
		repos, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Status: &[]valueobject.RepositoryStatus{valueobject.RepositoryStatusCompleted}[0],
			Limit:  1,
			Offset: 0,
		})
		if err != nil || len(repos) == 0 {
			t.Skip("No completed repositories found for job pagination test")
		}

		testRepo := repos[0]
		pageSize := 2
		var allJobs []*entity.IndexingJob
		offset := 0

		start := time.Now()

		for {
			jobs, total, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), outbound.IndexingJobFilters{
				Limit:  pageSize,
				Offset: offset,
			})
			if err != nil {
				t.Fatalf("Failed to fetch jobs page at offset %d: %v", offset, err)
			}

			if len(jobs) == 0 {
				break
			}

			allJobs = append(allJobs, jobs...)
			offset += pageSize

			// Verify total count
			if total < 0 {
				t.Error("Total count should be non-negative")
			}

			// All jobs should belong to correct repository
			for _, job := range jobs {
				if job.RepositoryID() != testRepo.ID() {
					t.Errorf("Job %s belongs to repository %s, expected %s",
						job.ID(), job.RepositoryID(), testRepo.ID())
				}
			}

			// Safety break
			if len(allJobs) > numJobsPerRepo*2 {
				t.Fatal("Too many jobs returned")
			}
		}

		duration := time.Since(start)
		t.Logf("Paginated through %d jobs in %v", len(allJobs), duration)

		// Performance check
		if duration > 1*time.Second {
			t.Errorf("Job pagination took too long: %v (expected < 1s)", duration)
		}
	})

	t.Run("Sorting performance", func(t *testing.T) {
		start := time.Now()

		// Test sorting by name
		repos, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Limit:  20,
			Offset: 0,
			Sort:   "name:asc",
		})
		if err != nil {
			t.Fatalf("Failed to sort by name: %v", err)
		}

		duration := time.Since(start)
		t.Logf("Sorted %d repositories by name in %v", len(repos), duration)

		// Verify sort order
		for i := 1; i < len(repos); i++ {
			if repos[i-1].Name() > repos[i].Name() {
				t.Errorf("Repositories not sorted by name: %s > %s", repos[i-1].Name(), repos[i].Name())
			}
		}

		// Performance check
		if duration > 2*time.Second {
			t.Errorf("Sorting took too long: %v (expected < 2s)", duration)
		}
	})
}

// TestConcurrentAccess tests concurrent access patterns.
//
//nolint:gocognit // Comprehensive concurrency test requires testing many scenarios
func TestConcurrentAccess(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Concurrent repository creation", func(t *testing.T) {
		const numGoroutines = 10
		const reposPerGoroutine = 5

		var wg sync.WaitGroup
		errCh := make(chan error, numGoroutines)
		createdRepos := make(chan uuid.UUID, numGoroutines*reposPerGoroutine)

		start := time.Now()

		for i := range numGoroutines {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := range reposPerGoroutine {
					uniqueID := uuid.New().String()
					testURL, _ := valueobject.NewRepositoryURL(
						"https://github.com/concurrent/repo-" + uniqueID)

					repo := entity.NewRepository(testURL,
						"Concurrent Repository "+string(rune('A'+workerID))+string(rune('0'+j)),
						nil, nil)

					if err := repoRepo.Save(ctx, repo); err != nil {
						errCh <- err
						return
					}

					createdRepos <- repo.ID()
				}
			}(i)
		}

		wg.Wait()
		close(errCh)
		close(createdRepos)

		duration := time.Since(start)
		t.Logf("Created %d repositories concurrently in %v", numGoroutines*reposPerGoroutine, duration)

		// Check for errors
		for err := range errCh {
			t.Errorf("Concurrent creation error: %v", err)
		}

		// Verify all repositories were created
		var repoIDs []uuid.UUID
		for id := range createdRepos {
			repoIDs = append(repoIDs, id)
		}

		if len(repoIDs) != numGoroutines*reposPerGoroutine {
			t.Errorf("Expected %d repositories, got %d", numGoroutines*reposPerGoroutine, len(repoIDs))
		}

		// Verify repositories exist
		for _, id := range repoIDs {
			repo, err := repoRepo.FindByID(ctx, id)
			if err != nil || repo == nil {
				t.Errorf("Repository %s not found after concurrent creation", id)
			}
		}

		// Performance check
		if duration > 10*time.Second {
			t.Errorf("Concurrent creation took too long: %v (expected < 10s)", duration)
		}
	})

	t.Run("Concurrent job status updates", func(t *testing.T) {
		// Create a repository first
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository: %v", err)
		}

		// Create multiple jobs
		const numJobs = 10
		var jobs []*entity.IndexingJob

		for i := range numJobs {
			job := createTestIndexingJob(t, testRepo.ID())
			err := jobRepo.Save(ctx, job)
			if err != nil {
				t.Fatalf("Failed to save job %d: %v", i, err)
			}
			jobs = append(jobs, job)
		}

		// Concurrently update job statuses
		var wg sync.WaitGroup
		errCh := make(chan error, numJobs)

		start := time.Now()

		for i, job := range jobs {
			wg.Add(1)
			go func(job *entity.IndexingJob, index int) {
				defer wg.Done()

				// Start job
				if err := job.Start(); err != nil {
					errCh <- err
					return
				}

				if err := jobRepo.Update(ctx, job); err != nil {
					errCh <- err
					return
				}

				// Complete or fail job based on index
				if index%2 == 0 {
					if err := job.Complete(100, 500); err != nil {
						errCh <- err
						return
					}
				} else {
					if err := job.Fail("Test error"); err != nil {
						errCh <- err
						return
					}
				}

				if err := jobRepo.Update(ctx, job); err != nil {
					errCh <- err
					return
				}
			}(job, i)
		}

		wg.Wait()
		close(errCh)

		duration := time.Since(start)
		t.Logf("Updated %d job statuses concurrently in %v", numJobs, duration)

		// Check for errors
		for err := range errCh {
			t.Errorf("Concurrent update error: %v", err)
		}

		// Verify final states
		for i, originalJob := range jobs {
			updatedJob, err := jobRepo.FindByID(ctx, originalJob.ID())
			if err != nil {
				t.Errorf("Failed to find job %d after update: %v", i, err)
				continue
			}

			if !updatedJob.IsTerminal() {
				t.Errorf("Job %d should be in terminal state", i)
			}

			expectedStatus := valueobject.JobStatusCompleted
			if i%2 == 1 {
				expectedStatus = valueobject.JobStatusFailed
			}

			if updatedJob.Status() != expectedStatus {
				t.Errorf("Job %d has status %s, expected %s", i, updatedJob.Status(), expectedStatus)
			}
		}

		// Performance check
		if duration > 5*time.Second {
			t.Errorf("Concurrent updates took too long: %v (expected < 5s)", duration)
		}
	})

	t.Run("Concurrent read operations", func(t *testing.T) {
		// Create some repositories for reading
		var repoIDs []uuid.UUID
		for i := range 5 {
			uniqueID := uuid.New().String()
			testURL, _ := valueobject.NewRepositoryURL("https://github.com/read-test/repo-" + uniqueID)
			repo := entity.NewRepository(testURL, "Read Test Repository "+string(rune('A'+i)), nil, nil)

			err := repoRepo.Save(ctx, repo)
			if err != nil {
				t.Fatalf("Failed to save repo for read test: %v", err)
			}
			repoIDs = append(repoIDs, repo.ID())
		}

		// Concurrent read operations
		const numReaders = 20
		var wg sync.WaitGroup
		errCh := make(chan error, numReaders)

		start := time.Now()

		for i := range numReaders {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()

				// Perform various read operations
				for _, id := range repoIDs {
					// Find by ID
					_, err := repoRepo.FindByID(ctx, id)
					if err != nil {
						errCh <- err
						return
					}

					// Find all
					_, _, err = repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 10})
					if err != nil {
						errCh <- err
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errCh)

		duration := time.Since(start)
		t.Logf("Performed concurrent reads with %d readers in %v", numReaders, duration)

		// Check for errors
		for err := range errCh {
			t.Errorf("Concurrent read error: %v", err)
		}

		// Performance check
		if duration > 3*time.Second {
			t.Errorf("Concurrent reads took too long: %v (expected < 3s)", duration)
		}
	})
}

//nolint:gocognit // Comprehensive integration test requires testing many scenarios
func TestDomainEntityIntegration(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Repository entity lifecycle", func(t *testing.T) {
		// Test complete repository lifecycle with domain operations
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/domain-test/lifecycle")
		description := "Domain integration test repository"
		defaultBranch := "main"

		// Create using domain constructor
		repo := entity.NewRepository(testURL, "Lifecycle Test Repository", &description, &defaultBranch)

		// Verify initial state
		if repo.Status() != valueobject.RepositoryStatusPending {
			t.Errorf("New repository should have pending status, got %s", repo.Status())
		}

		if repo.TotalFiles() != 0 || repo.TotalChunks() != 0 {
			t.Error("New repository should have zero files and chunks")
		}

		// Save to database
		err := repoRepo.Save(ctx, repo)
		if err != nil {
			t.Fatalf("Failed to save repository: %v", err)
		}

		// Retrieve from database
		savedRepo, err := repoRepo.FindByID(ctx, repo.ID())
		if err != nil || savedRepo == nil {
			t.Fatalf("Failed to retrieve saved repository: %v", err)
		}

		// Verify domain properties are preserved
		if !savedRepo.URL().Equal(testURL) {
			t.Errorf("URL not preserved: expected %s, got %s", testURL, savedRepo.URL())
		}

		if savedRepo.Name() != "Lifecycle Test Repository" {
			t.Errorf("Name not preserved: expected %s, got %s", "Lifecycle Test Repository", savedRepo.Name())
		}

		if savedRepo.Description() == nil || *savedRepo.Description() != description {
			t.Error("Description not preserved")
		}

		if savedRepo.DefaultBranch() == nil || *savedRepo.DefaultBranch() != defaultBranch {
			t.Error("Default branch not preserved")
		}

		// Test domain business logic - status transitions
		err = savedRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
		if err != nil {
			t.Errorf("Failed to update to cloning status: %v", err)
		}

		err = savedRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
		if err != nil {
			t.Errorf("Failed to update to processing status: %v", err)
		}

		// Mark indexing completed
		commitHash := "abc123def456"
		totalFiles := 150
		totalChunks := 750

		err = savedRepo.MarkIndexingCompleted(commitHash, totalFiles, totalChunks)
		if err != nil {
			t.Errorf("Failed to mark indexing completed: %v", err)
		}

		// Update in database
		err = repoRepo.Update(ctx, savedRepo)
		if err != nil {
			t.Errorf("Failed to update repository: %v", err)
		}

		// Verify updated state
		updatedRepo, err := repoRepo.FindByID(ctx, repo.ID())
		if err != nil {
			t.Errorf("Failed to retrieve updated repository: %v", err)
		}

		if updatedRepo.Status() != valueobject.RepositoryStatusCompleted {
			t.Errorf("Repository should be completed, got %s", updatedRepo.Status())
		}

		if updatedRepo.LastCommitHash() == nil || *updatedRepo.LastCommitHash() != commitHash {
			t.Error("Last commit hash not preserved")
		}

		if updatedRepo.TotalFiles() != totalFiles {
			t.Errorf("Total files: expected %d, got %d", totalFiles, updatedRepo.TotalFiles())
		}

		if updatedRepo.TotalChunks() != totalChunks {
			t.Errorf("Total chunks: expected %d, got %d", totalChunks, updatedRepo.TotalChunks())
		}

		if updatedRepo.LastIndexedAt() == nil {
			t.Error("LastIndexedAt should be set")
		}

		// Test archiving
		err = updatedRepo.Archive()
		if err != nil {
			t.Errorf("Failed to archive repository: %v", err)
		}

		err = repoRepo.Update(ctx, updatedRepo)
		if err != nil {
			t.Errorf("Failed to update archived repository: %v", err)
		}

		// Archived repository should not appear in normal queries
		archivedRepo, err := repoRepo.FindByID(ctx, repo.ID())
		if err != nil {
			t.Errorf("Expected no error finding archived repository: %v", err)
		}

		if archivedRepo != nil {
			t.Error("Archived repository should not be returned by normal queries")
		}
	})

	t.Run("IndexingJob entity lifecycle", func(t *testing.T) {
		// Create repository first
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save repository: %v", err)
		}

		// Create job using domain constructor
		job := entity.NewIndexingJob(testRepo.ID())

		// Verify initial state
		if job.Status() != valueobject.JobStatusPending {
			t.Errorf("New job should have pending status, got %s", job.Status())
		}

		if job.FilesProcessed() != 0 || job.ChunksCreated() != 0 {
			t.Error("New job should have zero processed files and chunks")
		}

		if job.StartedAt() != nil || job.CompletedAt() != nil {
			t.Error("New job should not have start or completion times")
		}

		// Save to database
		err = jobRepo.Save(ctx, job)
		if err != nil {
			t.Fatalf("Failed to save job: %v", err)
		}

		// Test job lifecycle
		err = job.Start()
		if err != nil {
			t.Errorf("Failed to start job: %v", err)
		}

		err = jobRepo.Update(ctx, job)
		if err != nil {
			t.Errorf("Failed to update started job: %v", err)
		}

		// Update progress
		job.UpdateProgress(50, 250)
		err = jobRepo.Update(ctx, job)
		if err != nil {
			t.Errorf("Failed to update job progress: %v", err)
		}

		// Complete job
		finalFiles := 100
		finalChunks := 500

		err = job.Complete(finalFiles, finalChunks)
		if err != nil {
			t.Errorf("Failed to complete job: %v", err)
		}

		err = jobRepo.Update(ctx, job)
		if err != nil {
			t.Errorf("Failed to update completed job: %v", err)
		}

		// Verify final state
		completedJob, err := jobRepo.FindByID(ctx, job.ID())
		if err != nil {
			t.Errorf("Failed to retrieve completed job: %v", err)
		}

		if completedJob.Status() != valueobject.JobStatusCompleted {
			t.Errorf("Job should be completed, got %s", completedJob.Status())
		}

		if completedJob.FilesProcessed() != finalFiles {
			t.Errorf("Files processed: expected %d, got %d", finalFiles, completedJob.FilesProcessed())
		}

		if completedJob.ChunksCreated() != finalChunks {
			t.Errorf("Chunks created: expected %d, got %d", finalChunks, completedJob.ChunksCreated())
		}

		if !completedJob.IsTerminal() {
			t.Error("Completed job should be in terminal state")
		}

		if completedJob.Duration() == nil {
			t.Error("Completed job should have duration")
		}
	})

	t.Run("Value object integration", func(t *testing.T) {
		// Test RepositoryURL value object
		validURLs := []string{
			"https://github.com/owner/repo",
			"https://gitlab.com/group/project",
			"https://bitbucket.org/user/repository",
		}

		for _, urlStr := range validURLs {
			testURL, err := valueobject.NewRepositoryURL(urlStr)
			if err != nil {
				t.Fatalf("Failed to create repository URL: %v", err)
			}

			repo := entity.NewRepository(testURL, "Value Object Test", nil, nil)
			err = repoRepo.Save(ctx, repo)
			if err != nil {
				t.Errorf("Failed to save repository with URL %s: %v", urlStr, err)
			}

			// Verify URL methods work
			if testURL.Host() == "" {
				t.Error("RepositoryURL.Host() should not be empty")
			}

			if testURL.Owner() == "" {
				t.Error("RepositoryURL.Owner() should not be empty")
			}

			if testURL.Name() == "" {
				t.Error("RepositoryURL.Name() should not be empty")
			}

			if testURL.FullName() == "" {
				t.Error("RepositoryURL.FullName() should not be empty")
			}

			// Retrieve and verify
			savedRepo, err := repoRepo.FindByURL(ctx, testURL)
			if err != nil {
				t.Errorf("Failed to find repository by URL: %v", err)
			}

			if savedRepo == nil {
				t.Error("Repository should be found by URL")
			} else if !savedRepo.URL().Equal(testURL) {
				t.Error("Retrieved repository URL should equal original")
			}
		}

		// Test repository status value object
		statuses := []valueobject.RepositoryStatus{
			valueobject.RepositoryStatusPending,
			valueobject.RepositoryStatusCloning,
			valueobject.RepositoryStatusProcessing,
			valueobject.RepositoryStatusCompleted,
			valueobject.RepositoryStatusFailed,
		}

		for _, status := range statuses {
			// Test filtering by status
			repos, total, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
				Status: &status,
				Limit:  100,
			})
			if err != nil {
				t.Errorf("Failed to filter by status %s: %v", status, err)
			}

			// Verify all returned repos have correct status
			for _, repo := range repos {
				if repo.Status() != status {
					t.Errorf("Repository has status %s, expected %s", repo.Status(), status)
				}
			}

			t.Logf("Found %d repositories with status %s (total: %d)", len(repos), status, total)
		}

		// Test job status value object
		jobStatuses := []valueobject.JobStatus{
			valueobject.JobStatusPending,
			valueobject.JobStatusRunning,
			valueobject.JobStatusCompleted,
			valueobject.JobStatusFailed,
			valueobject.JobStatusCancelled,
		}

		// Create a test repository for job testing
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository: %v", err)
		}

		// Create jobs with different statuses
		for _, status := range jobStatuses {
			job := createTestIndexingJob(t, testRepo.ID())

			// Set job to desired status
			switch status {
			case valueobject.JobStatusRunning:
				_ = job.Start()
			case valueobject.JobStatusCompleted:
				_ = job.Start()
				_ = job.Complete(100, 500)
			case valueobject.JobStatusFailed:
				_ = job.Start()
				_ = job.Fail("Test error")
			case valueobject.JobStatusCancelled:
				_ = job.Cancel()
			}

			err := jobRepo.Save(ctx, job)
			if err != nil {
				t.Errorf("Failed to save job with status %s: %v", status, err)
			}

			// Verify status
			savedJob, err := jobRepo.FindByID(ctx, job.ID())
			if err != nil {
				t.Errorf("Failed to find saved job: %v", err)
			} else if savedJob.Status() != status {
				t.Errorf("Job status: expected %s, got %s", status, savedJob.Status())
			}
		}
	})
}
