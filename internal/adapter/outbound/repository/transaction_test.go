package repository

import (
	"context"
	"fmt"
	"testing"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestTransactionManager_BasicTransaction tests basic transaction operations
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
					return fmt.Errorf("intentional error for rollback test")
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

// TestTransactionManager_AtomicOperations tests that operations within transactions are atomic
func TestTransactionManager_AtomicOperations(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because TransactionManager doesn't exist yet
	txManager := NewTransactionManager(pool)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)

	ctx := context.Background()

	t.Run("Atomic repository and job creation", func(t *testing.T) {
		var repoID uuid.UUID

		err := txManager.WithTransaction(ctx, func(ctx context.Context) error {
			// Create repository
			testRepo := createTestRepository(t)
			if err := repoRepo.Save(ctx, testRepo); err != nil {
				return fmt.Errorf("failed to save repository: %w", err)
			}
			repoID = testRepo.ID()

			// Create indexing job for the repository
			testJob := createTestIndexingJob(t, testRepo.ID())
			if err := jobRepo.Save(ctx, testJob); err != nil {
				return fmt.Errorf("failed to save indexing job: %w", err)
			}

			return nil
		})
		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}

		// Verify both repository and job were saved
		savedRepo, err := repoRepo.FindByID(ctx, repoID)
		if err != nil || savedRepo == nil {
			t.Error("Expected repository to be saved in transaction")
		}

		jobs, _, err := jobRepo.FindByRepositoryID(ctx, repoID, outbound.IndexingJobFilters{Limit: 10})
		if err != nil || len(jobs) == 0 {
			t.Error("Expected indexing job to be saved in transaction")
		}
	})

	t.Run("Rollback prevents partial saves", func(t *testing.T) {
		initialRepoCount := countRepositories(t, pool)
		initialJobCount := countIndexingJobs(t, pool)

		err := txManager.WithTransaction(ctx, func(ctx context.Context) error {
			// Create repository successfully
			testRepo := createTestRepository(t)
			if err := repoRepo.Save(ctx, testRepo); err != nil {
				return fmt.Errorf("failed to save repository: %w", err)
			}

			// Try to create job with invalid repository ID (should fail)
			invalidJob := createTestIndexingJob(t, uuid.New())
			if err := jobRepo.Save(ctx, invalidJob); err != nil {
				return fmt.Errorf("failed to save indexing job: %w", err)
			}

			return nil
		})

		if err == nil {
			t.Error("Expected transaction to fail due to foreign key constraint")
		}

		// Verify rollback - no new repositories or jobs should be saved
		finalRepoCount := countRepositories(t, pool)
		finalJobCount := countIndexingJobs(t, pool)

		if finalRepoCount != initialRepoCount {
			t.Errorf("Expected repository count to remain %d, got %d", initialRepoCount, finalRepoCount)
		}

		if finalJobCount != initialJobCount {
			t.Errorf("Expected job count to remain %d, got %d", initialJobCount, finalJobCount)
		}
	})
}

// TestTransactionManager_Isolation tests transaction isolation levels
func TestTransactionManager_Isolation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because TransactionManager doesn't exist yet
	txManager := NewTransactionManager(pool)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)

	ctx := context.Background()

	// Create a test repository
	testRepo := createTestRepository(t)
	err := repoRepo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	t.Run("Read committed isolation", func(t *testing.T) {
		// Start a transaction that will update the repository
		updateDone := make(chan bool)
		updateErr := make(chan error)

		go func() {
			defer close(updateDone)

			err := txManager.WithTransactionIsolation(ctx, pgx.ReadCommitted, func(ctx context.Context) error {
				repo, err := repoRepo.FindByID(ctx, testRepo.ID())
				if err != nil {
					return err
				}

				repo.UpdateName("Updated in transaction")

				// Signal that we're about to update
				updateDone <- true

				// Wait a bit to simulate work
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-updateDone:
					// Continue with update
				}

				return repoRepo.Update(ctx, repo)
			})
			if err != nil {
				updateErr <- err
			}
		}()

		// Wait for the update transaction to start
		<-updateDone

		// In a separate transaction, try to read the repository
		// Should see the original value due to isolation
		readErr := txManager.WithTransactionIsolation(ctx, pgx.ReadCommitted, func(ctx context.Context) error {
			repo, err := repoRepo.FindByID(ctx, testRepo.ID())
			if err != nil {
				return err
			}

			// Should still see original name
			if repo.Name() == "Updated in transaction" {
				t.Error("Should not see uncommitted changes from other transaction")
			}

			return nil
		})

		// Signal the update transaction to complete
		updateDone <- true

		// Check for errors
		if readErr != nil {
			t.Errorf("Read transaction failed: %v", readErr)
		}

		select {
		case err := <-updateErr:
			if err != nil {
				t.Errorf("Update transaction failed: %v", err)
			}
		default:
			// No error
		}
	})
}

// TestTransactionManager_DeadlockHandling tests deadlock detection and handling
func TestTransactionManager_DeadlockHandling(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because TransactionManager doesn't exist yet
	txManager := NewTransactionManager(pool)
	repoRepo := NewPostgreSQLRepositoryRepository(pool)

	ctx := context.Background()

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

	t.Run("Deadlock detection and retry", func(t *testing.T) {
		tx1Done := make(chan error, 1)
		tx2Done := make(chan error, 1)

		// Transaction 1: Update repo1 then repo2
		go func() {
			err := txManager.WithTransactionRetry(ctx, 3, func(ctx context.Context) error {
				// Update repository 1 first
				repo1, err := repoRepo.FindByID(ctx, testRepo1.ID())
				if err != nil {
					return err
				}
				repo1.UpdateName("Updated by TX1")
				if err := repoRepo.Update(ctx, repo1); err != nil {
					return err
				}

				// Small delay to increase chance of deadlock
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				// Then update repository 2
				repo2, err := repoRepo.FindByID(ctx, testRepo2.ID())
				if err != nil {
					return err
				}
				repo2.UpdateName("Updated by TX1")
				return repoRepo.Update(ctx, repo2)
			})

			tx1Done <- err
		}()

		// Transaction 2: Update repo2 then repo1 (opposite order to create deadlock)
		go func() {
			err := txManager.WithTransactionRetry(ctx, 3, func(ctx context.Context) error {
				// Update repository 2 first
				repo2, err := repoRepo.FindByID(ctx, testRepo2.ID())
				if err != nil {
					return err
				}
				repo2.UpdateName("Updated by TX2")
				if err := repoRepo.Update(ctx, repo2); err != nil {
					return err
				}

				// Small delay to increase chance of deadlock
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				// Then update repository 1
				repo1, err := repoRepo.FindByID(ctx, testRepo1.ID())
				if err != nil {
					return err
				}
				repo1.UpdateName("Updated by TX2")
				return repoRepo.Update(ctx, repo1)
			})

			tx2Done <- err
		}()

		// Wait for both transactions to complete
		err1 := <-tx1Done
		err2 := <-tx2Done

		// At least one should succeed (retry mechanism should handle deadlock)
		if err1 != nil && err2 != nil {
			t.Errorf("Both transactions failed - retry mechanism should have resolved deadlock. TX1: %v, TX2: %v", err1, err2)
		}
	})
}

// TestTransactionManager_LongRunningTransaction tests handling of long-running transactions
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		if err == nil {
			t.Error("Expected timeout error but got none")
		}

		if timeoutCtx.Err() == nil {
			t.Error("Expected context to be cancelled due to timeout")
		}
	})
}

// Helper functions

// countRepositories counts the total number of repositories in the database
func countRepositories(t *testing.T, pool *pgxpool.Pool) int {
	ctx := context.Background()
	var count int

	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM codechunking.repositories WHERE deleted_at IS NULL").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count repositories: %v", err)
	}

	return count
}

// countIndexingJobs counts the total number of indexing jobs in the database
func countIndexingJobs(t *testing.T, pool *pgxpool.Pool) int {
	ctx := context.Background()
	var count int

	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM codechunking.indexing_jobs WHERE deleted_at IS NULL").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count indexing jobs: %v", err)
	}

	return count
}
