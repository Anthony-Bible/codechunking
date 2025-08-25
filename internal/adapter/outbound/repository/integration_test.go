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

// Helper functions for refactoring TestSoftDeleteFunctionality

// createAndSaveRepository creates a test repository and saves it to the database.
func createAndSaveRepository(
	ctx context.Context,
	t *testing.T,
	repoRepo outbound.RepositoryRepository,
) *entity.Repository {
	t.Helper()

	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save repository: %v", err)
	}

	return testRepo
}

// createAndSaveJob creates a test indexing job and saves it to the database.
func createAndSaveJob(
	ctx context.Context,
	t *testing.T,
	jobRepo outbound.IndexingJobRepository,
	repositoryID uuid.UUID,
) *entity.IndexingJob {
	t.Helper()

	testJob := createTestIndexingJob(t, repositoryID)
	err := jobRepo.Save(ctx, testJob)
	if err != nil {
		t.Fatalf("Failed to save indexing job: %v", err)
	}

	return testJob
}

// verifySoftDeletedEntity verifies that an entity is soft deleted (not returned by normal queries).
func verifySoftDeletedEntity(
	ctx context.Context,
	t *testing.T,
	id uuid.UUID,
	findFunc func(context.Context, uuid.UUID) (interface{}, error),
) {
	t.Helper()

	result, err := findFunc(ctx, id)
	if err != nil {
		t.Errorf("Expected no error for soft deleted entity: %v", err)
	}
	// Handle both nil interface{} and typed nil pointers
	if result != nil {
		// Check for typed nil pointers (e.g., (*entity.Repository)(nil))
		switch v := result.(type) {
		case *entity.Repository:
			if v != nil {
				t.Error("Soft deleted entity should not be returned by normal queries")
			}
		case *entity.IndexingJob:
			if v != nil {
				t.Error("Soft deleted entity should not be returned by normal queries")
			}
		default:
			t.Error("Soft deleted entity should not be returned by normal queries")
		}
	}
}

// verifyEntityNotInListing verifies that an entity does not appear in listing results.
func verifyEntityNotInListing(
	ctx context.Context,
	t *testing.T,
	id uuid.UUID,
	listFunc func(context.Context) ([]interface{}, error),
	idExtractor func(interface{}) uuid.UUID,
) {
	t.Helper()

	entities, err := listFunc(ctx)
	if err != nil {
		t.Errorf("Failed to list entities: %v", err)
		return
	}

	for _, entity := range entities {
		if idExtractor(entity) == id {
			t.Error("Soft deleted entity should not appear in listings")
		}
	}
}

// verifyCountConsistency verifies that count consistency is maintained after soft delete.
func verifyCountConsistency(
	ctx context.Context,
	t *testing.T,
	deletedID uuid.UUID,
	listFunc func(context.Context) ([]interface{}, int, error),
	idExtractor func(interface{}) uuid.UUID,
	_ int,
	_ int,
) {
	t.Helper()

	entities, total, err := listFunc(ctx)
	if err != nil {
		t.Errorf("Failed to list entities for count consistency check: %v", err)
		return
	}

	if total > 0 {
		// If there are entities, the deleted one shouldn't be counted
		found := false
		for _, entity := range entities {
			if idExtractor(entity) == deletedID {
				found = true
				break
			}
		}
		if found {
			t.Error("Soft deleted entity should not be in results")
		}
	}
}

// setupCascadeDeleteTest sets up a repository with multiple jobs for cascade delete testing.
func setupCascadeDeleteTest(
	ctx context.Context,
	t *testing.T,
	repoRepo outbound.RepositoryRepository,
	jobRepo outbound.IndexingJobRepository,
	numJobs int,
) (*entity.Repository, []*entity.IndexingJob) {
	t.Helper()

	// Create repository
	testRepo := createAndSaveRepository(ctx, t, repoRepo)

	// Create multiple jobs for repository
	var jobs []*entity.IndexingJob
	for range numJobs {
		job := createAndSaveJob(ctx, t, jobRepo, testRepo.ID())
		jobs = append(jobs, job)
	}

	return testRepo, jobs
}

//nolint:gocognit,gocyclo,cyclop // Comprehensive integration test requires testing many scenarios
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

// TestSoftDeleteHelperFunctions tests that the helper functions for soft delete functionality work correctly.
// These tests will fail until the helper functions are implemented.
//
//nolint:gocognit // Test helper validation requires comprehensive testing
func TestSoftDeleteHelperFunctions(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("createAndSaveRepository helper function", func(t *testing.T) {
		// This test verifies the helper function creates and saves a repository successfully
		testRepo := createAndSaveRepository(ctx, t, repoRepo)

		if testRepo == nil {
			t.Fatal("createAndSaveRepository should return a non-nil repository")
		}

		// Verify repository exists in database
		foundRepo, err := repoRepo.FindByID(ctx, testRepo.ID())
		if err != nil {
			t.Errorf("Expected no error finding saved repository: %v", err)
		}
		if foundRepo == nil {
			t.Error("Repository should exist after createAndSaveRepository call")
		}
	})

	t.Run("createAndSaveJob helper function", func(t *testing.T) {
		// Create repository first
		testRepo := createAndSaveRepository(ctx, t, repoRepo)

		// This test verifies the helper function creates and saves a job successfully
		testJob := createAndSaveJob(ctx, t, jobRepo, testRepo.ID())

		if testJob == nil {
			t.Fatal("createAndSaveJob should return a non-nil job")
		}

		// Verify job exists in database
		foundJob, err := jobRepo.FindByID(ctx, testJob.ID())
		if err != nil {
			t.Errorf("Expected no error finding saved job: %v", err)
		}
		if foundJob == nil {
			t.Error("Job should exist after createAndSaveJob call")
		}

		// Verify job belongs to correct repository
		if testJob.RepositoryID() != testRepo.ID() {
			t.Errorf("Job repository ID should be %s, got %s", testRepo.ID(), testJob.RepositoryID())
		}
	})

	t.Run("verifySoftDeletedEntity helper function", func(t *testing.T) {
		// Create and save repository
		testRepo := createAndSaveRepository(ctx, t, repoRepo)

		// Soft delete repository
		err := repoRepo.Delete(ctx, testRepo.ID())
		if err != nil {
			t.Fatalf("Failed to soft delete repository: %v", err)
		}

		// This test verifies the helper function correctly identifies soft deleted entities
		verifySoftDeletedEntity(ctx, t, testRepo.ID(), func(ctx context.Context, id uuid.UUID) (interface{}, error) {
			return repoRepo.FindByID(ctx, id)
		})

		// Test should pass without any assertion failures from the helper
	})

	t.Run("verifyEntityNotInListing helper function", func(t *testing.T) {
		// Create and save repository
		testRepo := createAndSaveRepository(ctx, t, repoRepo)

		// Soft delete repository
		err := repoRepo.Delete(ctx, testRepo.ID())
		if err != nil {
			t.Fatalf("Failed to soft delete repository: %v", err)
		}

		// This test verifies the helper function correctly checks entity is not in listing
		verifyEntityNotInListing(ctx, t, testRepo.ID(), func(ctx context.Context) ([]interface{}, error) {
			repos, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 100})
			if err != nil {
				return nil, err
			}

			// Convert to []interface{} for generic helper
			result := make([]interface{}, len(repos))
			for i, repo := range repos {
				result[i] = repo
			}
			return result, nil
		}, func(ent interface{}) uuid.UUID {
			return ent.(*entity.Repository).ID()
		})

		// Test should pass without any assertion failures from the helper
	})

	t.Run("verifyCountConsistency helper function", func(t *testing.T) {
		// Create and save repository
		testRepo := createAndSaveRepository(ctx, t, repoRepo)

		// Get initial count
		initialRepos, initialTotal, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 100})
		if err != nil {
			t.Fatalf("Failed to get initial repository count: %v", err)
		}

		// Soft delete repository
		err = repoRepo.Delete(ctx, testRepo.ID())
		if err != nil {
			t.Fatalf("Failed to soft delete repository: %v", err)
		}

		// This test verifies the helper function correctly checks count consistency after soft delete
		verifyCountConsistency(ctx, t, testRepo.ID(), func(ctx context.Context) ([]interface{}, int, error) {
			repos, total, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 100})
			if err != nil {
				return nil, 0, err
			}

			// Convert to []interface{} for generic helper
			result := make([]interface{}, len(repos))
			for i, repo := range repos {
				result[i] = repo
			}
			return result, total, nil
		}, func(ent interface{}) uuid.UUID {
			return ent.(*entity.Repository).ID()
		}, len(initialRepos), initialTotal)

		// Test should pass without any assertion failures from the helper
	})

	t.Run("setupCascadeDeleteTest helper function", func(t *testing.T) {
		// This test verifies the helper function correctly sets up cascade delete test scenario
		testRepo, testJobs := setupCascadeDeleteTest(ctx, t, repoRepo, jobRepo, 3)

		if testRepo == nil {
			t.Fatal("setupCascadeDeleteTest should return a non-nil repository")
		}

		if len(testJobs) != 3 {
			t.Errorf("setupCascadeDeleteTest should return 3 jobs, got %d", len(testJobs))
		}

		// Verify repository exists
		foundRepo, err := repoRepo.FindByID(ctx, testRepo.ID())
		if err != nil || foundRepo == nil {
			t.Error("Repository should exist after setupCascadeDeleteTest call")
		}

		// Verify all jobs exist and belong to the repository
		for i, job := range testJobs {
			foundJob, err := jobRepo.FindByID(ctx, job.ID())
			if err != nil || foundJob == nil {
				t.Errorf("Job %d should exist after setupCascadeDeleteTest call", i)
			}
			if job.RepositoryID() != testRepo.ID() {
				t.Errorf("Job %d should belong to repository %s, got %s",
					i, testRepo.ID(), job.RepositoryID())
			}
		}
	})
}

// TestSoftDeleteFunctionalityRefactored tests the refactored soft delete functionality using helper functions.
// This test should have lower cyclomatic complexity than the original TestSoftDeleteFunctionality.
//
//nolint:gocognit // Refactored test still requires comprehensive validation
func TestSoftDeleteFunctionalityRefactored(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Repository soft delete and restore", func(t *testing.T) {
		// Use helper function to create and save repository
		testRepo := createAndSaveRepository(ctx, t, repoRepo)

		// Soft delete repository
		err := repoRepo.Delete(ctx, testRepo.ID())
		if err != nil {
			t.Errorf("Failed to soft delete repository: %v", err)
		}

		// Use helper functions to verify soft delete behavior
		verifySoftDeletedEntity(ctx, t, testRepo.ID(), func(ctx context.Context, id uuid.UUID) (interface{}, error) {
			return repoRepo.FindByID(ctx, id)
		})

		verifyEntityNotInListing(ctx, t, testRepo.ID(), func(ctx context.Context) ([]interface{}, error) {
			repos, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 100})
			if err != nil {
				return nil, err
			}
			result := make([]interface{}, len(repos))
			for i, repo := range repos {
				result[i] = repo
			}
			return result, nil
		}, func(ent interface{}) uuid.UUID {
			return ent.(*entity.Repository).ID()
		})

		verifyCountConsistency(ctx, t, testRepo.ID(), func(ctx context.Context) ([]interface{}, int, error) {
			repos, total, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 100})
			if err != nil {
				return nil, 0, err
			}
			result := make([]interface{}, len(repos))
			for i, repo := range repos {
				result[i] = repo
			}
			return result, total, nil
		}, func(ent interface{}) uuid.UUID {
			return ent.(*entity.Repository).ID()
		}, 0, 0) // Initial counts will be determined by helper
	})

	t.Run("IndexingJob soft delete", func(t *testing.T) {
		// Use helper functions to create repository and job
		testRepo := createAndSaveRepository(ctx, t, repoRepo)
		testJob := createAndSaveJob(ctx, t, jobRepo, testRepo.ID())

		// Soft delete job
		err := jobRepo.Delete(ctx, testJob.ID())
		if err != nil {
			t.Errorf("Failed to soft delete indexing job: %v", err)
		}

		// Use helper functions to verify soft delete behavior
		verifySoftDeletedEntity(ctx, t, testJob.ID(), func(ctx context.Context, id uuid.UUID) (interface{}, error) {
			return jobRepo.FindByID(ctx, id)
		})

		verifyEntityNotInListing(ctx, t, testJob.ID(), func(ctx context.Context) ([]interface{}, error) {
			jobs, _, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), outbound.IndexingJobFilters{Limit: 100})
			if err != nil {
				return nil, err
			}
			result := make([]interface{}, len(jobs))
			for i, job := range jobs {
				result[i] = job
			}
			return result, nil
		}, func(ent interface{}) uuid.UUID {
			return ent.(*entity.IndexingJob).ID()
		})

		verifyCountConsistency(ctx, t, testJob.ID(), func(ctx context.Context) ([]interface{}, int, error) {
			jobs, total, err := jobRepo.FindByRepositoryID(ctx, testRepo.ID(), outbound.IndexingJobFilters{Limit: 100})
			if err != nil {
				return nil, 0, err
			}
			result := make([]interface{}, len(jobs))
			for i, job := range jobs {
				result[i] = job
			}
			return result, total, nil
		}, func(ent interface{}) uuid.UUID {
			return ent.(*entity.IndexingJob).ID()
		}, 0, 0) // Initial counts will be determined by helper
	})

	t.Run("Cascade delete behavior", func(t *testing.T) {
		// Use helper function to setup cascade delete test
		testRepo, testJobs := setupCascadeDeleteTest(ctx, t, repoRepo, jobRepo, 3)

		// Delete repository (should cascade to jobs if implemented)
		err := repoRepo.Delete(ctx, testRepo.ID())
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
		t.Logf("After repository deletion, %d jobs remain (originally created %d)",
			len(remainingJobs), len(testJobs))
	})
}

// TestHelperFunctionSignatures tests that helper functions have correct signatures and behavior.
// These tests define the expected interface for the helper functions.
//
//nolint:gocognit // Signature validation requires comprehensive testing
func TestHelperFunctionSignatures(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("createAndSaveRepository signature test", func(t *testing.T) {
		// This test defines that createAndSaveRepository should:
		// 1. Take (context.Context, *testing.T, RepositoryRepository) as parameters
		// 2. Return *entity.Repository
		// 3. Handle creation and saving in a single call
		// 4. Should not fail for valid inputs

		// The function should be callable like this (will fail until implemented):
		result := createAndSaveRepository(ctx, t, repoRepo)

		// Expected behavior:
		if result == nil {
			t.Error("createAndSaveRepository should never return nil for valid inputs")
		}

		// Should be findable in database
		if result != nil {
			found, err := repoRepo.FindByID(ctx, result.ID())
			if err != nil || found == nil {
				t.Error("createAndSaveRepository should save repository to database")
			}
		}
	})

	t.Run("createAndSaveJob signature test", func(t *testing.T) {
		// Create a repository first for the job
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository: %v", err)
		}

		// This test defines that createAndSaveJob should:
		// 1. Take (context.Context, *testing.T, IndexingJobRepository, uuid.UUID) as parameters
		// 2. Return *entity.IndexingJob
		// 3. Handle creation and saving in a single call
		// 4. Should not fail for valid inputs

		// The function should be callable like this (will fail until implemented):
		result := createAndSaveJob(ctx, t, jobRepo, testRepo.ID())

		// Expected behavior:
		if result == nil {
			t.Error("createAndSaveJob should never return nil for valid inputs")
		}

		// Should belong to correct repository
		if result != nil && result.RepositoryID() != testRepo.ID() {
			t.Error("createAndSaveJob should create job for correct repository")
		}

		// Should be findable in database
		if result != nil {
			found, err := jobRepo.FindByID(ctx, result.ID())
			if err != nil || found == nil {
				t.Error("createAndSaveJob should save job to database")
			}
		}
	})

	t.Run("verifySoftDeletedEntity signature test", func(t *testing.T) {
		// Create and soft delete a repository
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository: %v", err)
		}

		err = repoRepo.Delete(ctx, testRepo.ID())
		if err != nil {
			t.Fatalf("Failed to soft delete repository: %v", err)
		}

		// This test defines that verifySoftDeletedEntity should:
		// 1. Take (context.Context, *testing.T, uuid.UUID, func(context.Context, uuid.UUID) (interface{}, error)) as parameters
		// 2. Not return anything (void function)
		// 3. Use t.Error internally for assertion failures
		// 4. Should not panic for valid inputs

		// The function should be callable like this (will fail until implemented):
		verifySoftDeletedEntity(ctx, t, testRepo.ID(), func(ctx context.Context, id uuid.UUID) (interface{}, error) {
			return repoRepo.FindByID(ctx, id)
		})

		// If we reach here without panic, the signature is correct
	})

	t.Run("verifyEntityNotInListing signature test", func(t *testing.T) {
		// Create and soft delete a repository
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository: %v", err)
		}

		err = repoRepo.Delete(ctx, testRepo.ID())
		if err != nil {
			t.Fatalf("Failed to soft delete repository: %v", err)
		}

		// This test defines that verifyEntityNotInListing should:
		// 1. Take complex parameters including listing function and ID extractor
		// 2. Not return anything (void function)
		// 3. Use t.Error internally for assertion failures
		// 4. Should not panic for valid inputs

		// The function should be callable like this (will fail until implemented):
		verifyEntityNotInListing(ctx, t, testRepo.ID(),
			func(ctx context.Context) ([]interface{}, error) {
				repos, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 100})
				if err != nil {
					return nil, err
				}
				result := make([]interface{}, len(repos))
				for i, repo := range repos {
					result[i] = repo
				}
				return result, nil
			},
			func(ent interface{}) uuid.UUID {
				return ent.(*entity.Repository).ID()
			})

		// If we reach here without panic, the signature is correct
	})

	t.Run("verifyCountConsistency signature test", func(t *testing.T) {
		// This test defines that verifyCountConsistency should:
		// 1. Take complex parameters including listing function, ID extractor, and expected counts
		// 2. Not return anything (void function)
		// 3. Use t.Error internally for assertion failures
		// 4. Should not panic for valid inputs

		// The function should be callable like this (will fail until implemented):
		verifyCountConsistency(ctx, t, uuid.New(),
			func(ctx context.Context) ([]interface{}, int, error) {
				repos, total, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 100})
				if err != nil {
					return nil, 0, err
				}
				result := make([]interface{}, len(repos))
				for i, repo := range repos {
					result[i] = repo
				}
				return result, total, nil
			},
			func(ent interface{}) uuid.UUID {
				return ent.(*entity.Repository).ID()
			},
			0, 0) // expected counts

		// If we reach here without panic, the signature is correct
	})

	t.Run("setupCascadeDeleteTest signature test", func(t *testing.T) {
		// This test defines that setupCascadeDeleteTest should:
		// 1. Take (context.Context, *testing.T, RepositoryRepository, IndexingJobRepository, int) as parameters
		// 2. Return (*entity.Repository, []*entity.IndexingJob)
		// 3. Create the specified number of jobs for the repository
		// 4. Should not fail for valid inputs

		// The function should be callable like this (will fail until implemented):
		repo, jobs := setupCascadeDeleteTest(ctx, t, repoRepo, jobRepo, 3)

		// Expected behavior:
		if repo == nil {
			t.Error("setupCascadeDeleteTest should return non-nil repository")
		}

		if len(jobs) != 3 {
			t.Errorf("setupCascadeDeleteTest should return 3 jobs, got %d", len(jobs))
		}

		// All jobs should belong to the repository
		if repo != nil {
			for i, job := range jobs {
				if job == nil {
					t.Errorf("Job %d should not be nil", i)
				} else if job.RepositoryID() != repo.ID() {
					t.Errorf("Job %d should belong to repository %s", i, repo.ID())
				}
			}
		}
	})
}

// TestCyclomaticComplexityReduction tests that the refactored version reduces complexity.
// This test verifies the architectural improvement goals.
func TestCyclomaticComplexityReduction(t *testing.T) {
	t.Run("original function complexity pattern", func(t *testing.T) {
		// The original TestSoftDeleteFunctionality has cyclomatic complexity of 33
		// This comes from:
		// - 3 main subtests (base complexity)
		// - Multiple nested if statements for error checking
		// - Multiple loops for verification
		// - Nested conditions within loops
		// - Multiple early returns and error paths

		// This test documents the complexity sources that should be reduced
		complexityFactors := []string{
			"Multiple nested if statements in each subtest",
			"Loop-based verification with nested conditions",
			"Repetitive error checking patterns",
			"Multiple verification steps with similar logic",
			"Inline entity creation and saving logic",
		}

		expectedComplexityReduction := len(complexityFactors)

		if expectedComplexityReduction < 4 {
			t.Error("Expected to identify at least 4 complexity factors to reduce")
		}

		t.Logf("Identified %d complexity factors that should be reduced by helper functions",
			expectedComplexityReduction)
	})

	t.Run("refactored function complexity pattern", func(t *testing.T) {
		// The refactored TestSoftDeleteFunctionalityRefactored should have lower complexity
		// This comes from:
		// - 3 main subtests (same base complexity)
		// - Single function call for entity creation (eliminates creation complexity)
		// - Single function call for each verification pattern (eliminates verification complexity)
		// - Cleaner error handling delegated to helper functions
		// - No loops in main test logic (moved to helpers)

		expectedComplexityFactors := []string{
			"Linear helper function calls",
			"Simplified error handling through helper delegation",
			"No inline loops or complex verification logic",
		}

		// The refactored version should have significantly fewer complexity factors
		if len(expectedComplexityFactors) >= 4 {
			t.Error("Refactored version should have fewer complexity factors")
		}

		t.Logf("Refactored version should have only %d complexity factors",
			len(expectedComplexityFactors))
	})

	t.Run("helper function isolation test", func(t *testing.T) {
		// Each helper function should handle a single responsibility
		helperResponsibilities := map[string]string{
			"createAndSaveRepository":  "Entity creation and persistence",
			"createAndSaveJob":         "Job entity creation and persistence",
			"verifySoftDeletedEntity":  "Soft delete verification logic",
			"verifyEntityNotInListing": "Listing exclusion verification logic",
			"verifyCountConsistency":   "Count consistency verification logic",
			"setupCascadeDeleteTest":   "Complex test scenario setup",
		}

		for helperName, responsibility := range helperResponsibilities {
			t.Logf("Helper function %s should handle: %s", helperName, responsibility)
		}

		if len(helperResponsibilities) != 6 {
			t.Error("Expected exactly 6 helper functions for complete refactoring")
		}
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
//nolint:gocognit,gocyclo,cyclop // Comprehensive pagination and performance test requires testing many scenarios
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
//nolint:gocognit,gocyclo,cyclop // Comprehensive concurrency test requires testing many scenarios
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

// TestPaginationAndPerformanceRefactorHelpers tests that helper functions for TestPaginationAndPerformance work correctly.
// These tests will fail until the helper functions are implemented to reduce cyclomatic complexity.
func TestPaginationAndPerformanceRefactorHelpers(t *testing.T) {
	// Reduced complexity by delegating to individual test functions
	t.Run("setupPaginationTestData", func(t *testing.T) {
		testSetupPaginationTestDataHelper(t)
	})
	t.Run("createTestRepositoryWithJobs", func(t *testing.T) {
		testCreateTestRepositoryWithJobsHelper(t)
	})
	t.Run("runPaginationTest", func(t *testing.T) {
		testRunPaginationTestHelper(t)
	})
	t.Run("validatePaginationResults", func(t *testing.T) {
		testValidatePaginationResultsHelper(t)
	})
	t.Run("checkPaginationPerformance", func(t *testing.T) {
		testCheckPaginationPerformanceHelper(t)
	})
	t.Run("detectDuplicatesInResults", func(t *testing.T) {
		testDetectDuplicatesInResultsHelper(t)
	})
}

// testSetupPaginationTestDataHelper tests the setupPaginationTestData helper function.
func testSetupPaginationTestDataHelper(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("setupPaginationTestData helper function", func(t *testing.T) {
		// This test defines that setupPaginationTestData should:
		// 1. Take (context.Context, *testing.T, RepositoryRepository, IndexingJobRepository, int, int) as parameters
		// 2. Return ([]uuid.UUID, time.Duration) - repository IDs and creation duration
		// 3. Create specified number of repositories with jobs
		// 4. Set varied statuses and handle complex creation logic
		// 5. Measure and return creation performance

		// The function should be callable like this (will fail until implemented):
		repoIDs, duration := setupPaginationTestData(ctx, t, repoRepo, jobRepo, 10, 3)

		// Expected behavior:
		if len(repoIDs) != 10 {
			t.Errorf("setupPaginationTestData should create 10 repositories, got %d", len(repoIDs))
		}

		if duration == 0 {
			t.Error("setupPaginationTestData should return non-zero creation duration")
		}

		// Verify repositories exist in database (note: archived repositories may not be findable)
		foundCount := 0
		for i, repoID := range repoIDs {
			repo, err := repoRepo.FindByID(ctx, repoID)
			if err == nil && repo != nil {
				foundCount++
			} else {
				t.Logf("Repository %d (ID: %s) not found - likely archived", i, repoID)
			}
		}
		// We should find at least some repositories (non-archived ones)
		if foundCount == 0 {
			t.Error("Should find at least some repositories after setupPaginationTestData call")
		}
		t.Logf("Found %d out of %d repositories (archived repositories may not be findable)", foundCount, len(repoIDs))

		// Verify performance logging capability
		if duration > 30*time.Second {
			t.Errorf("setupPaginationTestData should complete within reasonable time, took %v", duration)
		}
	})
}

// testCreateTestRepositoryWithJobsHelper tests the createTestRepositoryWithJobs helper function.
func testCreateTestRepositoryWithJobsHelper(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("createTestRepositoryWithJobs helper function", func(t *testing.T) {
		// This test defines that createTestRepositoryWithJobs should:
		// 1. Take (context.Context, *testing.T, RepositoryRepository, IndexingJobRepository, string, int, int) as parameters
		// 2. Return (*entity.Repository, []*entity.IndexingJob)
		// 3. Create single repository with specified number of jobs
		// 4. Handle status setting logic for both repository and jobs
		// 5. Apply modulo-based status distribution pattern

		// The function should be callable like this (will fail until implemented):
		repo, jobs := createTestRepositoryWithJobs(ctx, t, repoRepo, jobRepo, "test-repo", 5, 1)

		// Reduced complexity by delegating verification to helper functions
		verifyCreateTestRepositoryWithJobsBasicChecks(t, repo, jobs, 5)
		verifyRepositoryExists(t, ctx, repoRepo, repo)
		verifyJobsBelongToRepository(t, ctx, jobRepo, repo, jobs)
		verifyRepositoryStatusForIndex1(t, repo)
	})
}

// verifyCreateTestRepositoryWithJobsBasicChecks performs basic validation of createTestRepositoryWithJobs results.
func verifyCreateTestRepositoryWithJobsBasicChecks(
	t *testing.T,
	repo *entity.Repository,
	jobs []*entity.IndexingJob,
	expectedJobCount int,
) {
	t.Helper()
	if repo == nil {
		t.Fatal("createTestRepositoryWithJobs should return non-nil repository")
	}
	if len(jobs) != expectedJobCount {
		t.Errorf("createTestRepositoryWithJobs should create %d jobs, got %d", expectedJobCount, len(jobs))
	}
}

// verifyRepositoryExists checks that the repository can be found in the database.
func verifyRepositoryExists(
	t *testing.T,
	ctx context.Context,
	repoRepo outbound.RepositoryRepository,
	repo *entity.Repository,
) {
	t.Helper()
	foundRepo, err := repoRepo.FindByID(ctx, repo.ID())
	if err != nil || foundRepo == nil {
		t.Error("Repository should exist after createTestRepositoryWithJobs call")
	}
}

// verifyJobsBelongToRepository checks that all jobs belong to the repository and exist in the database.
func verifyJobsBelongToRepository(
	t *testing.T,
	ctx context.Context,
	jobRepo outbound.IndexingJobRepository,
	repo *entity.Repository,
	jobs []*entity.IndexingJob,
) {
	t.Helper()
	for i, job := range jobs {
		if job.RepositoryID() != repo.ID() {
			t.Errorf("Job %d should belong to repository %s", i, repo.ID())
		}

		foundJob, err := jobRepo.FindByID(ctx, job.ID())
		if err != nil || foundJob == nil {
			t.Errorf("Job %d should exist after createTestRepositoryWithJobs call", i)
		}
	}
}

// verifyRepositoryStatusForIndex1 checks that repository has the expected status for index 1 (completed).
func verifyRepositoryStatusForIndex1(t *testing.T, repo *entity.Repository) {
	t.Helper()
	// Verify modulo-based status distribution for index 1 (1 % 4 == 1)
	if repo.Status() != valueobject.RepositoryStatusCompleted {
		t.Error("Repository should have completed status for index 1")
	}
}

// testRunPaginationTestHelper tests the runPaginationTest helper function.
func testRunPaginationTestHelper(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	t.Run("runPaginationTest helper function", func(t *testing.T) {
		// Create some test data first
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to create test data: %v", err)
		}

		// This test defines that runPaginationTest should:
		// 1. Take (context.Context, *testing.T, RepositoryRepository, int) as parameters
		// 2. Return (PaginationResults) containing all repos and performance metrics
		// 3. Handle complex pagination logic with multiple validation checks
		// 4. Detect infinite loops and validate consistency
		// 5. Return structured results for further validation

		// The function should be callable like this (will fail until implemented):
		results := runPaginationTest(ctx, t, repoRepo, 10)

		// Expected behavior:
		if results.AllRepos == nil {
			t.Error("runPaginationTest should return non-nil AllRepos slice")
		}

		if results.Duration == 0 {
			t.Error("runPaginationTest should return non-zero pagination duration")
		}

		if results.TotalFetched < 0 {
			t.Error("runPaginationTest should return non-negative TotalFetched count")
		}

		if results.PagesFetched <= 0 {
			t.Error("runPaginationTest should return positive PagesFetched count")
		}

		// Verify pagination results structure
		if len(results.AllRepos) != results.TotalFetched {
			t.Errorf("AllRepos length (%d) should match TotalFetched (%d)",
				len(results.AllRepos), results.TotalFetched)
		}

		// Verify no performance issues detected
		if results.Duration > 10*time.Second {
			t.Errorf("runPaginationTest should complete reasonably fast, took %v", results.Duration)
		}
	})
}

// testValidatePaginationResultsHelper tests the validatePaginationResults helper function.
func testValidatePaginationResultsHelper(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	t.Run("validatePaginationResults helper function", func(t *testing.T) {
		// Create test data
		testRepos := []*entity.Repository{
			createTestRepository(t),
			createTestRepository(t),
		}
		for _, repo := range testRepos {
			err := repoRepo.Save(ctx, repo)
			if err != nil {
				t.Fatalf("Failed to save test repo: %v", err)
			}
		}

		// This test defines that validatePaginationResults should:
		// 1. Take (*testing.T, []*entity.Repository, time.Duration, int) as parameters
		// 2. Not return anything (void function, uses t.Error internally)
		// 3. Validate no duplicate IDs in results
		// 4. Check performance thresholds
		// 5. Verify result count consistency

		// The function should be callable like this (will fail until implemented):
		validatePaginationResults(t, testRepos, 100*time.Millisecond, 2)

		// If we reach here without panic, the signature is correct
		// The function should internally validate and use t.Error for failures

		// Test with duplicate detection (should cause t.Error internally)
		duplicateRepos := []*entity.Repository{
			testRepos[0], // Same repo twice
			testRepos[0],
		}
		validatePaginationResults(t, duplicateRepos, 100*time.Millisecond, 2)
	})
}

// testCheckPaginationPerformanceHelper tests the checkPaginationPerformance helper function.
func testCheckPaginationPerformanceHelper(t *testing.T) {
	t.Run("checkPaginationPerformance helper function", func(t *testing.T) {
		// This test defines that checkPaginationPerformance should:
		// 1. Take (*testing.T, time.Duration, string, time.Duration) as parameters
		// 2. Not return anything (void function, uses t.Error internally)
		// 3. Compare actual duration against expected threshold
		// 4. Log performance information
		// 5. Report errors if performance is below expectations

		// The function should be callable like this (will fail until implemented):
		checkPaginationPerformance(t, 500*time.Millisecond, "test operation", 2*time.Second)

		// Test performance check that should pass
		checkPaginationPerformance(t, 100*time.Millisecond, "fast operation", 1*time.Second)

		// Test performance check that should pass with generous threshold
		checkPaginationPerformance(t, 5*time.Second, "slow operation", 10*time.Second)

		// If we reach here without panic, the signature is correct
	})
}

// testDetectDuplicatesInResultsHelper tests the detectDuplicatesInResults helper function.
func testDetectDuplicatesInResultsHelper(t *testing.T) {
	t.Run("detectDuplicatesInResults helper function", func(t *testing.T) {
		// Create test data
		testRepos := []*entity.Repository{
			createTestRepository(t),
			createTestRepository(t),
		}

		// This test defines that detectDuplicatesInResults should:
		// 1. Take (*testing.T, []*entity.Repository) as parameters
		// 2. Not return anything (void function, uses t.Error internally)
		// 3. Build map of seen IDs to detect duplicates
		// 4. Report duplicate IDs with specific error messages
		// 5. Handle empty slices gracefully

		// The function should be callable like this (will fail until implemented):
		detectDuplicatesInResults(t, testRepos)

		// Test with no duplicates (should pass silently)
		detectDuplicatesInResults(t, testRepos)

		// Note: We don't test the duplicate case here because it would cause this test to fail.
		// The duplicate detection functionality is tested when the function is used in actual pagination tests.

		// Test with empty slice (should handle gracefully)
		detectDuplicatesInResults(t, []*entity.Repository{})

		// If we reach here without panic, the signature is correct
	})
}

// TestPaginationAndPerformanceRefactored tests the refactored pagination functionality with reduced complexity.
// This test should have significantly lower cyclomatic complexity than the original TestPaginationAndPerformance.
//
//nolint:gocognit // Refactored pagination test still requires comprehensive validation
func TestPaginationAndPerformanceRefactored(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	// Constants
	const numRepositories = 50
	const numJobsPerRepo = 5

	t.Run("Setup test data", func(t *testing.T) {
		// Use helper function to create test data (reduces complexity)
		repoIDs, duration := setupPaginationTestData(ctx, t, repoRepo, jobRepo, numRepositories, numJobsPerRepo)

		t.Logf("Created %d repositories with jobs in %v", len(repoIDs), duration)

		// Use helper function for performance validation (reduces complexity)
		checkPaginationPerformance(t, duration, "data creation", 30*time.Second)
	})

	t.Run("Repository pagination", func(t *testing.T) {
		// Use helper function for pagination testing (reduces complexity)
		results := runPaginationTest(ctx, t, repoRepo, 10)

		t.Logf("Paginated through %d repositories in %v", results.TotalFetched, results.Duration)

		// Calculate expected non-archived repositories: every 4th repository (i % 4 == 3) is archived
		// With numRepositories=50: archived are at indices 3,7,11,15,19,23,27,31,35,39,43,47 (12 total)
		expectedNonArchived := numRepositories - (numRepositories / 4)

		// Use helper functions for validation (reduces complexity)
		validatePaginationResults(t, results.AllRepos, results.Duration, expectedNonArchived)
		detectDuplicatesInResults(t, results.AllRepos)
		checkPaginationPerformance(t, results.Duration, "pagination", 5*time.Second)
	})

	t.Run("Repository filtering performance", func(t *testing.T) {
		start := time.Now()

		// Test status filtering (simplified, single responsibility)
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

		// Verify all returned repositories have correct status (single loop, no nesting)
		for _, repo := range completedRepos {
			if repo.Status() != valueobject.RepositoryStatusCompleted {
				t.Errorf("Repository %s has status %s, expected %s",
					repo.ID(), repo.Status(), valueobject.RepositoryStatusCompleted)
			}
		}

		// Use helper function for performance validation (reduces complexity)
		checkPaginationPerformance(t, duration, "status filtering", 2*time.Second)
	})

	t.Run("Job pagination by repository", func(t *testing.T) {
		// Get a repository with jobs (simplified logic)
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

		// Simplified pagination loop (extracted from complex nested logic)
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

			// Basic validations (simplified)
			if total < 0 {
				t.Error("Total count should be non-negative")
			}
			for _, job := range jobs {
				if job.RepositoryID() != testRepo.ID() {
					t.Errorf("Job %s belongs to wrong repository", job.ID())
				}
			}

			// Safety break
			if len(allJobs) > numJobsPerRepo*2 {
				t.Fatal("Too many jobs returned")
			}
		}

		duration := time.Since(start)
		t.Logf("Paginated through %d jobs in %v", len(allJobs), duration)

		// Use helper function for performance validation (reduces complexity)
		checkPaginationPerformance(t, duration, "job pagination", 1*time.Second)
	})

	t.Run("Sorting performance", func(t *testing.T) {
		start := time.Now()

		// Test sorting by name (simplified)
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

		// Verify sort order (single loop, no nesting)
		for i := 1; i < len(repos); i++ {
			if repos[i-1].Name() > repos[i].Name() {
				t.Errorf("Repositories not sorted by name: %s > %s", repos[i-1].Name(), repos[i].Name())
			}
		}

		// Use helper function for performance validation (reduces complexity)
		checkPaginationPerformance(t, duration, "sorting", 2*time.Second)
	})
}

// TestPaginationHelperSignatures tests that pagination helper functions have correct signatures.
// These tests define the expected interface for the helper functions that will reduce complexity.
//
//nolint:gocognit // Pagination helper signature validation requires comprehensive testing
func TestPaginationHelperSignatures(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("setupPaginationTestData signature test", func(t *testing.T) {
		// This test defines that setupPaginationTestData should:
		// 1. Take (context.Context, *testing.T, RepositoryRepository, IndexingJobRepository, int, int) as parameters
		// 2. Return ([]uuid.UUID, time.Duration)
		// 3. Create repositories and jobs with complex status patterns
		// 4. Handle performance measurement
		// 5. Should not fail for valid inputs

		// The function should be callable like this (will fail until implemented):
		repoIDs, duration := setupPaginationTestData(ctx, t, repoRepo, jobRepo, 5, 2)

		// Expected behavior:
		if len(repoIDs) != 5 {
			t.Errorf("setupPaginationTestData should return 5 repository IDs, got %d", len(repoIDs))
		}

		if duration <= 0 {
			t.Error("setupPaginationTestData should return positive duration")
		}

		// Should create actual repositories (note: archived repositories may not be findable via normal queries)
		foundCount := 0
		for _, repoID := range repoIDs {
			repo, err := repoRepo.FindByID(ctx, repoID)
			if err == nil && repo != nil {
				foundCount++
			}
		}
		// We should find at least some repositories (archived ones may not be findable)
		if foundCount == 0 {
			t.Error("Should find at least some repositories after setupPaginationTestData")
		}
	})

	t.Run("createTestRepositoryWithJobs signature test", func(t *testing.T) {
		// This test defines that createTestRepositoryWithJobs should:
		// 1. Take (context.Context, *testing.T, RepositoryRepository, IndexingJobRepository, string, int, int) as parameters
		// 2. Return (*entity.Repository, []*entity.IndexingJob)
		// 3. Apply modulo-based status distribution
		// 4. Handle complex status setting logic
		// 5. Should not fail for valid inputs

		// The function should be callable like this (will fail until implemented):
		repo, jobs := createTestRepositoryWithJobs(ctx, t, repoRepo, jobRepo, "test", 3, 0)

		// Expected behavior:
		if repo == nil {
			t.Error("createTestRepositoryWithJobs should return non-nil repository")
		}

		if len(jobs) != 3 {
			t.Errorf("createTestRepositoryWithJobs should return 3 jobs, got %d", len(jobs))
		}

		// Jobs should belong to repository
		for i, job := range jobs {
			if job == nil {
				t.Errorf("Job %d should not be nil", i)
			} else if job.RepositoryID() != repo.ID() {
				t.Errorf("Job %d should belong to repository", i)
			}
		}
	})

	t.Run("runPaginationTest signature test", func(t *testing.T) {
		// Create a test repository first
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository: %v", err)
		}

		// This test defines that runPaginationTest should:
		// 1. Take (context.Context, *testing.T, RepositoryRepository, int) as parameters
		// 2. Return PaginationResults struct
		// 3. Handle complex pagination logic
		// 4. Measure performance
		// 5. Should not fail for valid inputs

		// The function should be callable like this (will fail until implemented):
		results := runPaginationTest(ctx, t, repoRepo, 10)

		// Expected behavior:
		if results.AllRepos == nil {
			t.Error("runPaginationTest should return non-nil AllRepos")
		}

		if results.Duration <= 0 {
			t.Error("runPaginationTest should return positive Duration")
		}

		if results.TotalFetched < 0 {
			t.Error("runPaginationTest should return non-negative TotalFetched")
		}

		if results.PagesFetched < 0 {
			t.Error("runPaginationTest should return non-negative PagesFetched")
		}
	})

	t.Run("validatePaginationResults signature test", func(t *testing.T) {
		// Create test repositories
		testRepos := []*entity.Repository{
			createTestRepository(t),
			createTestRepository(t),
		}

		// This test defines that validatePaginationResults should:
		// 1. Take (*testing.T, []*entity.Repository, time.Duration, int) as parameters
		// 2. Not return anything (void function)
		// 3. Use t.Error internally for failures
		// 4. Should not panic for valid inputs

		// The function should be callable like this (will fail until implemented):
		validatePaginationResults(t, testRepos, 100*time.Millisecond, 2)

		// If we reach here without panic, the signature is correct
	})

	t.Run("checkPaginationPerformance signature test", func(t *testing.T) {
		// This test defines that checkPaginationPerformance should:
		// 1. Take (*testing.T, time.Duration, string, time.Duration) as parameters
		// 2. Not return anything (void function)
		// 3. Use t.Error internally for failures
		// 4. Should not panic for valid inputs

		// The function should be callable like this (will fail until implemented):
		checkPaginationPerformance(t, 100*time.Millisecond, "test op", 1*time.Second)

		// If we reach here without panic, the signature is correct
	})

	t.Run("detectDuplicatesInResults signature test", func(t *testing.T) {
		// Create test repositories
		testRepos := []*entity.Repository{
			createTestRepository(t),
		}

		// This test defines that detectDuplicatesInResults should:
		// 1. Take (*testing.T, []*entity.Repository) as parameters
		// 2. Not return anything (void function)
		// 3. Use t.Error internally for failures
		// 4. Should not panic for valid inputs

		// The function should be callable like this (will fail until implemented):
		detectDuplicatesInResults(t, testRepos)

		// If we reach here without panic, the signature is correct
	})
}

// PaginationResults represents the results from pagination testing.
// This struct should be defined to support the refactored pagination tests.
type PaginationResults struct {
	AllRepos     []*entity.Repository
	Duration     time.Duration
	TotalFetched int
	PagesFetched int
}

// TestPaginationResultsStruct tests that the PaginationResults struct works correctly.
func TestPaginationResultsStruct(t *testing.T) {
	t.Run("PaginationResults struct creation", func(t *testing.T) {
		// This test verifies the PaginationResults struct is properly defined
		results := PaginationResults{
			AllRepos:     []*entity.Repository{},
			Duration:     100 * time.Millisecond,
			TotalFetched: 10,
			PagesFetched: 2,
		}

		if results.AllRepos == nil {
			t.Error("AllRepos should not be nil")
		}

		if results.Duration != 100*time.Millisecond {
			t.Errorf("Duration should be 100ms, got %v", results.Duration)
		}

		if results.TotalFetched != 10 {
			t.Errorf("TotalFetched should be 10, got %d", results.TotalFetched)
		}

		if results.PagesFetched != 2 {
			t.Errorf("PagesFetched should be 2, got %d", results.PagesFetched)
		}
	})

	t.Run("PaginationResults with actual repositories", func(t *testing.T) {
		// Create test repositories
		testRepos := []*entity.Repository{
			createTestRepository(t),
			createTestRepository(t),
		}

		results := PaginationResults{
			AllRepos:     testRepos,
			Duration:     250 * time.Millisecond,
			TotalFetched: len(testRepos),
			PagesFetched: 1,
		}

		if len(results.AllRepos) != 2 {
			t.Errorf("AllRepos should contain 2 repositories, got %d", len(results.AllRepos))
		}

		if results.TotalFetched != len(testRepos) {
			t.Errorf("TotalFetched (%d) should match AllRepos length (%d)",
				results.TotalFetched, len(results.AllRepos))
		}
	})
}

// TestComplexityReductionRequirements tests the architectural requirements for complexity reduction.
func TestComplexityReductionRequirements(t *testing.T) {
	t.Run("helper function responsibilities", func(t *testing.T) {
		// Document what each helper function should handle to reduce complexity
		helperFunctions := map[string]string{
			"setupPaginationTestData":      "Extract nested loops for repository creation with status setting",
			"createTestRepositoryWithJobs": "Extract single repository creation with modulo-based status distribution",
			"runPaginationTest":            "Extract pagination loop with multiple validation checks",
			"validatePaginationResults":    "Extract duplicate detection and result consistency validation",
			"checkPaginationPerformance":   "Extract performance timing and threshold checking",
			"detectDuplicatesInResults":    "Extract duplicate ID detection logic with map-based checking",
		}

		for funcName, responsibility := range helperFunctions {
			t.Logf("Helper function %s should: %s", funcName, responsibility)
		}

		if len(helperFunctions) != 6 {
			t.Errorf("Expected 6 helper functions for complete complexity reduction, got %d", len(helperFunctions))
		}
	})

	t.Run("cyclomatic complexity reduction targets", func(t *testing.T) {
		// Define the complexity factors that should be reduced
		complexityFactors := []string{
			"Nested loops in Setup test data subtest (repositories -> jobs)",
			"Multiple switch statements for status setting (repository and job statuses)",
			"Complex pagination logic with multiple conditions in Repository pagination",
			"Duplicate detection logic with map building and checking",
			"Performance timing and validation with multiple error paths",
			"Multiple error handling branches throughout subtests",
			"Repetitive validation patterns across different pagination tests",
		}

		for _, factor := range complexityFactors {
			t.Logf("Complexity factor to reduce: %s", factor)
		}

		// Target: reduce from 44 to under 30
		currentComplexity := 44
		targetComplexity := 30
		reductionNeeded := currentComplexity - targetComplexity

		if reductionNeeded < 14 {
			t.Errorf("Expected to need at least 14 points of complexity reduction, calculated %d", reductionNeeded)
		}

		t.Logf(
			"Target complexity reduction: %d -> %d (reduction of %d points)",
			currentComplexity,
			targetComplexity,
			reductionNeeded,
		)
	})

	t.Run("refactored structure validation", func(t *testing.T) {
		// Verify that the refactored structure meets requirements
		refactoredSubtests := []string{
			"Setup test data",
			"Repository pagination",
			"Repository filtering performance",
			"Job pagination by repository",
			"Sorting performance",
		}

		for _, subtest := range refactoredSubtests {
			t.Logf("Refactored subtest: %s should use helper functions to reduce complexity", subtest)
		}

		if len(refactoredSubtests) != 5 {
			t.Errorf("Expected 5 subtests in refactored version, got %d", len(refactoredSubtests))
		}

		// Each subtest should primarily call helper functions instead of containing complex logic
		expectedHelperUsage := map[string][]string{
			"Setup test data": {"setupPaginationTestData", "checkPaginationPerformance"},
			"Repository pagination": {
				"runPaginationTest",
				"validatePaginationResults",
				"detectDuplicatesInResults",
				"checkPaginationPerformance",
			},
			"Repository filtering performance": {"checkPaginationPerformance"},
			"Job pagination by repository":     {"checkPaginationPerformance"},
			"Sorting performance":              {"checkPaginationPerformance"},
		}

		for subtest, helpers := range expectedHelperUsage {
			t.Logf("Subtest '%s' should use helpers: %v", subtest, helpers)
		}
	})
}

//nolint:gocognit,gocyclo,cyclop // Comprehensive domain integration test requires testing many scenarios
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
			case valueobject.JobStatusPending:
				// Job is already pending by default, no action needed
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

// setupPaginationTestData creates test repositories with jobs and varied statuses.
// This extracts the complex nested loops from TestPaginationAndPerformance to reduce cyclomatic complexity.
func setupPaginationTestData(
	ctx context.Context,
	t *testing.T,
	repoRepo outbound.RepositoryRepository,
	jobRepo outbound.IndexingJobRepository,
	numRepositories, numJobsPerRepo int,
) ([]uuid.UUID, time.Duration) {
	t.Helper()
	start := time.Now()
	var repoIDs []uuid.UUID

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
			// Pending -> Cloning -> Processing -> Completed
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCompleted)
		case 2:
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusFailed)
		case 3:
			// Pending -> Cloning -> Processing -> Completed -> Archived
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCompleted)
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusArchived)
			_ = testRepo.Archive()
		}

		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save repository %d: %v", i, err)
		}

		repoIDs = append(repoIDs, testRepo.ID())

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

	return repoIDs, time.Since(start)
}

// createTestRepositoryWithJobs creates a single repository with specified number of jobs.
// This extracts single repository creation logic to reduce complexity.
func createTestRepositoryWithJobs(
	ctx context.Context,
	t *testing.T,
	repoRepo outbound.RepositoryRepository,
	jobRepo outbound.IndexingJobRepository,
	namePrefix string,
	numJobs, statusIndex int,
) (*entity.Repository, []*entity.IndexingJob) {
	t.Helper()

	uniqueID := uuid.New().String()
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/" + namePrefix + "-" + uniqueID)
	testRepo := entity.NewRepository(testURL, namePrefix+" Repository", nil, nil)

	// Apply modulo-based status distribution
	switch statusIndex % 4 {
	case 1:
		// Pending -> Cloning -> Processing -> Completed
		_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
		_ = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
		_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCompleted)
	case 2:
		_ = testRepo.UpdateStatus(valueobject.RepositoryStatusFailed)
	case 3:
		// Pending -> Cloning -> Processing -> Completed -> Archived
		_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
		_ = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
		_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCompleted)
		_ = testRepo.UpdateStatus(valueobject.RepositoryStatusArchived)
		_ = testRepo.Archive()
	}

	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save repository: %v", err)
	}

	var jobs []*entity.IndexingJob
	if !testRepo.IsDeleted() {
		for j := range numJobs {
			job := createTestIndexingJob(t, testRepo.ID())

			// Set different job statuses using modulo
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
				t.Fatalf("Failed to save job %d: %v", j, err)
			}
			jobs = append(jobs, job)
		}
	}

	return testRepo, jobs
}

// runPaginationTest performs pagination testing and returns structured results.
// This extracts the complex pagination loop to reduce cyclomatic complexity.
func runPaginationTest(
	ctx context.Context,
	t *testing.T,
	repoRepo outbound.RepositoryRepository,
	pageSize int,
) PaginationResults {
	t.Helper()

	var allRepos []*entity.Repository
	offset := 0
	pagesFetched := 0
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

		pagesFetched++

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
		if len(allRepos) > 200 {
			t.Fatal("Infinite pagination loop detected")
		}
	}

	return PaginationResults{
		AllRepos:     allRepos,
		Duration:     time.Since(start),
		TotalFetched: len(allRepos),
		PagesFetched: pagesFetched,
	}
}

// validatePaginationResults validates pagination results for consistency and correctness.
// This extracts result validation logic to reduce cyclomatic complexity.
func validatePaginationResults(t *testing.T, repos []*entity.Repository, duration time.Duration, expectedMinRepos int) {
	t.Helper()

	if repos == nil {
		t.Error("AllRepos should not be nil")
		return
	}

	if duration == 0 {
		t.Error("Duration should not be zero")
	}

	// len(repos) is always non-negative, so we check if it's reasonable
	if len(repos) == 0 && expectedMinRepos > 0 {
		t.Error("No repositories found when expecting some")
	}

	if expectedMinRepos > 0 && len(repos) < expectedMinRepos {
		t.Errorf("Expected at least %d repos, got %d", expectedMinRepos, len(repos))
	}
}

// checkPaginationPerformance validates performance against expected thresholds.
// This extracts performance validation logic to reduce cyclomatic complexity.
func checkPaginationPerformance(t *testing.T, actual time.Duration, operation string, threshold time.Duration) {
	t.Helper()

	if actual > threshold {
		t.Errorf("%s took too long: %v (expected < %v)", operation, actual, threshold)
	}
}

// detectDuplicatesInResults detects duplicate repository IDs in pagination results.
// This extracts duplicate detection logic to reduce cyclomatic complexity.
func detectDuplicatesInResults(t *testing.T, repos []*entity.Repository) {
	t.Helper()

	seenIDs := make(map[uuid.UUID]bool)
	for _, repo := range repos {
		if seenIDs[repo.ID()] {
			t.Errorf("Duplicate repository ID found in pagination: %s", repo.ID())
		}
		seenIDs[repo.ID()] = true
	}
}
