package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestTransactionManager_BasicTransaction tests basic transaction operations.
func TestTransactionManager_BasicTransaction(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because TransactionManager doesn't exist yet
	txManager := NewTransactionManager(pool)

	ctx := context.Background()

	tests := []struct {
		name        string
		operation   func(context.Context) error
		expectError bool
	}{
		{
			name: "Successful transaction should commit",
			operation: func(ctx context.Context) error {
				return txManager.WithTransaction(ctx, func(ctx context.Context) error {
					repoRepo := NewPostgreSQLRepositoryRepository(pool)
					testRepo := createTestRepository(t)

					// This should succeed within transaction
					return repoRepo.Save(ctx, testRepo)
				})
			},
			expectError: false,
		},
		{
			name: "Failed transaction should rollback",
			operation: func(ctx context.Context) error {
				return txManager.WithTransaction(ctx, func(ctx context.Context) error {
					repoRepo := NewPostgreSQLRepositoryRepository(pool)
					testRepo := createTestRepository(t)

					// Save repository successfully
					if err := repoRepo.Save(ctx, testRepo); err != nil {
						return err
					}

					// Force an error to trigger rollback
					return errors.New("intentional error for rollback test")
				})
			},
			expectError: true,
		},
		{
			name: "Nested transaction should work correctly",
			operation: func(ctx context.Context) error {
				return txManager.WithTransaction(ctx, func(ctx context.Context) error {
					repoRepo := NewPostgreSQLRepositoryRepository(pool)
					testRepo1 := createTestRepository(t)

					// Save first repository
					if err := repoRepo.Save(ctx, testRepo1); err != nil {
						return err
					}

					// Nested transaction (savepoint)
					return txManager.WithTransaction(ctx, func(ctx context.Context) error {
						testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/nested")
						testRepo2 := entity.NewRepository(testURL, "Nested Repository", nil, nil)

						return repoRepo.Save(ctx, testRepo2)
					})
				})
			},
			expectError: false,
		},
		{
			name: "Context cancellation should abort transaction",
			operation: func(ctx context.Context) error {
				cancelCtx, cancel := context.WithCancel(ctx)

				return txManager.WithTransaction(cancelCtx, func(ctx context.Context) error {
					repoRepo := NewPostgreSQLRepositoryRepository(pool)
					testRepo := createTestRepository(t)

					// Cancel context mid-transaction
					cancel()

					// This should fail due to cancelled context
					return repoRepo.Save(ctx, testRepo)
				})
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation(ctx)

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

// TestTransactionManager_AtomicSaveSuccess tests successful atomic operations within transactions.
func TestTransactionManager_AtomicSaveSuccess(t *testing.T) {
	setup := setupAtomicTransactionTest(t)
	defer setup.pool.Close()

	// Create test repository
	testRepo := createTestRepository(t)

	// Perform atomic save operation
	err := performAtomicRepositoryAndJobSave(t, setup, testRepo)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Verify both repository and job were saved successfully
	verifyAtomicSaveSuccess(t, setup, testRepo.ID())
}

// TestTransactionManager_AtomicSaveRollback tests that failed transactions rollback completely.
func TestTransactionManager_AtomicSaveRollback(t *testing.T) {
	setup := setupAtomicTransactionTest(t)
	defer setup.pool.Close()

	// Capture initial state
	initialRepoCount := countRepositories(t, setup.pool)
	initialJobCount := countIndexingJobs(t, setup.pool)

	// Perform operation that should trigger rollback
	err := performRollbackOperation(t, setup)
	if err == nil {
		t.Error("Expected transaction to fail due to foreign key constraint")
	}

	// Verify rollback occurred
	verifyRollbackOccurred(t, setup.pool, initialRepoCount, initialJobCount)
}

// isolationTestSetup holds the common test dependencies for isolation tests.
type isolationTestSetup struct {
	pool      *pgxpool.Pool
	txManager *TransactionManager
	repoRepo  outbound.RepositoryRepository
	testRepo  *entity.Repository
	ctx       context.Context
}

// setupIsolationTest creates common test setup for transaction isolation tests.
func setupIsolationTest(t *testing.T) *isolationTestSetup {
	pool := setupTestDB(t)
	txManager := NewTransactionManager(pool)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create a test repository
	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	return &isolationTestSetup{
		pool:      pool,
		txManager: txManager,
		repoRepo:  repoRepo,
		testRepo:  testRepo,
		ctx:       ctx,
	}
}

// runUpdateTransaction executes a transaction that updates a repository name.
func runUpdateTransaction(setup *isolationTestSetup, newName string, signal chan<- bool, waitSignal <-chan bool) error {
	return setup.txManager.WithTransactionIsolation(setup.ctx, pgx.ReadCommitted, func(ctx context.Context) error {
		repo, err := setup.repoRepo.FindByID(ctx, setup.testRepo.ID())
		if err != nil {
			return err
		}

		repo.UpdateName(newName)

		// Signal that we're about to update
		if signal != nil {
			signal <- true
		}

		// Wait for signal to proceed with update
		if waitSignal != nil {
			select {
			case <-waitSignal:
				// Continue with update
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return setup.repoRepo.Update(ctx, repo)
	})
}

// runReadTransaction executes a transaction that reads a repository and verifies isolation.
func runReadTransaction(setup *isolationTestSetup, t *testing.T, unexpectedName string) error {
	return setup.txManager.WithTransactionIsolation(setup.ctx, pgx.ReadCommitted, func(ctx context.Context) error {
		repo, err := setup.repoRepo.FindByID(ctx, setup.testRepo.ID())
		if err != nil {
			return err
		}

		// Should not see uncommitted changes from other transaction
		if repo.Name() == unexpectedName {
			t.Errorf("Should not see uncommitted changes: %s", unexpectedName)
		}

		return nil
	})
}

// TestTransactionManager_ReadCommittedIsolation tests read committed isolation level.
func TestTransactionManager_ReadCommittedIsolation(t *testing.T) {
	setup := setupIsolationTest(t)
	defer setup.pool.Close()

	updateSignal := make(chan bool, 1)
	proceedSignal := make(chan bool, 1)
	updateComplete := make(chan error, 1)
	const updatedName = "Updated in transaction"

	// Start update transaction in background
	go func() {
		err := runUpdateTransaction(setup, updatedName, updateSignal, proceedSignal)
		updateComplete <- err
	}()

	// Wait for update transaction to signal it's ready
	<-updateSignal

	// Run read transaction to verify isolation
	readErr := runReadTransaction(setup, t, updatedName)
	if readErr != nil {
		t.Errorf("Read transaction failed: %v", readErr)
	}

	// Signal update transaction to proceed
	proceedSignal <- true

	// Check if update transaction completed successfully
	updateErr := <-updateComplete
	if updateErr != nil {
		t.Errorf("Update transaction failed: %v", updateErr)
	}
}

// deadlockTestData holds the repositories used for deadlock testing.
type deadlockTestData struct {
	repo1 *entity.Repository
	repo2 *entity.Repository
}

// setupDeadlockTestRepositories creates and saves two test repositories for deadlock testing.
func setupDeadlockTestRepositories(
	t *testing.T,
	ctx context.Context,
	repoRepo outbound.RepositoryRepository,
) *deadlockTestData {
	// Create two test repositories
	testRepo1 := createTestRepository(t)
	uniqueID2 := uuid.New().String()
	testRepo2URL, _ := valueobject.NewRepositoryURL("https://github.com/test/repo2-" + uniqueID2)
	testRepo2 := entity.NewRepository(testRepo2URL, "Test Repository 2", nil, nil)

	err := repoRepo.Save(ctx, testRepo1)
	if err != nil {
		t.Fatalf("Failed to save test repository 1: %v", err)
	}

	err = repoRepo.Save(ctx, testRepo2)
	if err != nil {
		t.Fatalf("Failed to save test repository 2: %v", err)
	}

	return &deadlockTestData{
		repo1: testRepo1,
		repo2: testRepo2,
	}
}

// runDeadlockProneTransaction runs a transaction that updates repositories in a specific order.
func runDeadlockProneTransaction(
	ctx context.Context,
	txManager *TransactionManager,
	repoRepo outbound.RepositoryRepository,
	data *deadlockTestData,
	firstRepoID, secondRepoID uuid.UUID,
	txName string,
) error {
	return txManager.WithTransactionRetry(ctx, 3, func(ctx context.Context) error {
		// Update first repository
		firstRepo, err := repoRepo.FindByID(ctx, firstRepoID)
		if err != nil {
			return err
		}
		firstRepo.UpdateName(fmt.Sprintf("Updated by %s", txName))
		if err := repoRepo.Update(ctx, firstRepo); err != nil {
			return err
		}

		// Small delay to increase chance of deadlock
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Update second repository
		secondRepo, err := repoRepo.FindByID(ctx, secondRepoID)
		if err != nil {
			return err
		}
		secondRepo.UpdateName(fmt.Sprintf("Updated by %s", txName))
		return repoRepo.Update(ctx, secondRepo)
	})
}

// runConcurrentDeadlockTransactions runs two transactions concurrently in opposite order.
func runConcurrentDeadlockTransactions(
	ctx context.Context,
	txManager *TransactionManager,
	repoRepo outbound.RepositoryRepository,
	data *deadlockTestData,
) (error, error) {
	tx1Done := make(chan error, 1)
	tx2Done := make(chan error, 1)

	// Transaction 1: Update repo1 then repo2
	go func() {
		err := runDeadlockProneTransaction(ctx, txManager, repoRepo, data, data.repo1.ID(), data.repo2.ID(), "TX1")
		tx1Done <- err
	}()

	// Transaction 2: Update repo2 then repo1 (opposite order to create deadlock)
	go func() {
		err := runDeadlockProneTransaction(ctx, txManager, repoRepo, data, data.repo2.ID(), data.repo1.ID(), "TX2")
		tx2Done <- err
	}()

	// Wait for both transactions to complete
	err1 := <-tx1Done
	err2 := <-tx2Done

	return err1, err2
}

// TestTransactionManager_DeadlockDetection tests basic deadlock detection capability.
func TestTransactionManager_DeadlockDetection(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	txManager := NewTransactionManager(pool)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	t.Run("Deadlock detection and retry mechanism", func(t *testing.T) {
		data := setupDeadlockTestRepositories(t, ctx, repoRepo)
		err1, err2 := runConcurrentDeadlockTransactions(ctx, txManager, repoRepo, data)

		// At least one should succeed (retry mechanism should handle deadlock)
		if err1 != nil && err2 != nil {
			t.Errorf(
				"Both transactions failed - retry mechanism should have resolved deadlock. TX1: %v, TX2: %v",
				err1,
				err2,
			)
		}
	})
}

// TestTransactionManager_DeadlockRetry tests the retry mechanism for deadlocked transactions.
func TestTransactionManager_DeadlockRetry(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	txManager := NewTransactionManager(pool)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	t.Run("Retry mechanism resolves deadlocks", func(t *testing.T) {
		data := setupDeadlockTestRepositories(t, ctx, repoRepo)

		// Run multiple iterations to increase likelihood of testing retry mechanism
		for i := range 3 {
			err1, err2 := runConcurrentDeadlockTransactions(ctx, txManager, repoRepo, data)

			// At least one transaction should always succeed
			if err1 != nil && err2 != nil {
				t.Errorf("Iteration %d: Both transactions failed, retry should have resolved deadlock", i+1)
				break
			}
		}
	})
}

// TestTransactionManager_LongRunningTransaction tests handling of long-running transactions.
func TestTransactionManager_LongRunningTransaction(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because TransactionManager doesn't exist yet
	txManager := NewTransactionManager(pool)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)

	ctx := context.Background()

	t.Run("Transaction timeout handling", func(t *testing.T) {
		// Set a short timeout for this test
		timeoutCtx, cancel := context.WithTimeout(ctx, 100) // Very short timeout
		defer cancel()

		err := txManager.WithTransaction(timeoutCtx, func(ctx context.Context) error {
			// Create repository
			testRepo := createTestRepository(t)
			if err := repoRepo.Save(ctx, testRepo); err != nil {
				return err
			}

			// Simulate long-running operation that exceeds timeout
			<-ctx.Done()
			return ctx.Err()
		})

		if err == nil {
			t.Error("Expected timeout error but got none")
		}

		if timeoutCtx.Err() == nil {
			t.Error("Expected context to be cancelled due to timeout")
		}
	})
}

// Helper functions for atomic transaction tests

// atomicTransactionTestSetup holds the common test dependencies for atomic transaction tests.
type atomicTransactionTestSetup struct {
	pool      *pgxpool.Pool
	txManager *TransactionManager
	repoRepo  outbound.RepositoryRepository
	jobRepo   outbound.IndexingJobRepository
	ctx       context.Context
}

// setupAtomicTransactionTest creates common test setup for atomic transaction tests.
func setupAtomicTransactionTest(t *testing.T) *atomicTransactionTestSetup {
	pool := setupTestDB(t)
	txManager := NewTransactionManager(pool)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	return &atomicTransactionTestSetup{
		pool:      pool,
		txManager: txManager,
		repoRepo:  repoRepo,
		jobRepo:   jobRepo,
		ctx:       ctx,
	}
}

// performAtomicRepositoryAndJobSave executes atomic save operation within a transaction.
func performAtomicRepositoryAndJobSave(
	t *testing.T,
	setup *atomicTransactionTestSetup,
	testRepo *entity.Repository,
) error {
	return setup.txManager.WithTransaction(setup.ctx, func(ctx context.Context) error {
		// Create repository
		if err := setup.repoRepo.Save(ctx, testRepo); err != nil {
			return fmt.Errorf("failed to save repository: %w", err)
		}

		// Create indexing job for the repository
		testJob := createTestIndexingJob(t, testRepo.ID())
		if err := setup.jobRepo.Save(ctx, testJob); err != nil {
			return fmt.Errorf("failed to save indexing job: %w", err)
		}

		return nil
	})
}

// verifyAtomicSaveSuccess verifies that both repository and job were saved successfully.
func verifyAtomicSaveSuccess(t *testing.T, setup *atomicTransactionTestSetup, repoID uuid.UUID) {
	// Verify repository was saved
	savedRepo, err := setup.repoRepo.FindByID(setup.ctx, repoID)
	if err != nil || savedRepo == nil {
		t.Error("Expected repository to be saved in transaction")
	}

	// Verify job was saved
	jobs, _, err := setup.jobRepo.FindByRepositoryID(setup.ctx, repoID, outbound.IndexingJobFilters{Limit: 10})
	if err != nil || len(jobs) == 0 {
		t.Error("Expected indexing job to be saved in transaction")
	}
}

// performRollbackOperation executes an operation that should trigger a rollback.
func performRollbackOperation(t *testing.T, setup *atomicTransactionTestSetup) error {
	return setup.txManager.WithTransaction(setup.ctx, func(ctx context.Context) error {
		// Create repository successfully
		testRepo := createTestRepository(t)
		if err := setup.repoRepo.Save(ctx, testRepo); err != nil {
			return fmt.Errorf("failed to save repository: %w", err)
		}

		// Try to create job with invalid repository ID (should fail)
		invalidJob := createTestIndexingJob(t, uuid.New())
		if err := setup.jobRepo.Save(ctx, invalidJob); err != nil {
			return fmt.Errorf("failed to save indexing job: %w", err)
		}

		return nil
	})
}

// verifyRollbackOccurred verifies that the rollback successfully prevented any saves.
func verifyRollbackOccurred(t *testing.T, pool *pgxpool.Pool, initialRepoCount, initialJobCount int) {
	finalRepoCount := countRepositories(t, pool)
	finalJobCount := countIndexingJobs(t, pool)

	if finalRepoCount != initialRepoCount {
		t.Errorf("Expected repository count to remain %d, got %d", initialRepoCount, finalRepoCount)
	}

	if finalJobCount != initialJobCount {
		t.Errorf("Expected job count to remain %d, got %d", initialJobCount, finalJobCount)
	}
}

// countRepositories counts the total number of repositories in the database.
func countRepositories(t *testing.T, pool *pgxpool.Pool) int {
	ctx := context.Background()
	var count int

	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM codechunking.repositories WHERE deleted_at IS NULL").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count repositories: %v", err)
	}

	return count
}

// countIndexingJobs counts the total number of indexing jobs in the database.
func countIndexingJobs(t *testing.T, pool *pgxpool.Pool) int {
	ctx := context.Background()
	var count int

	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM codechunking.indexing_jobs WHERE deleted_at IS NULL").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count indexing jobs: %v", err)
	}

	return count
}
