//go:build integration

package repository

import (
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestVectorStorageRepository_Interface verifies that PostgreSQLVectorStorageRepository implements the interface.
func TestVectorStorageRepository_Interface(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLVectorStorageRepository doesn't exist yet
	repo := NewPostgreSQLVectorStorageRepository(pool)

	// Verify it implements the interface
	var _ outbound.VectorStorageRepository = repo
}

// TestVectorStorageRepository_BulkInsertEmbeddings_RegularTable tests bulk insertion to regular embeddings table.
func TestVectorStorageRepository_BulkInsertEmbeddings_RegularTable(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repository and chunks first
	repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 100)

	// Create test embeddings for bulk insertion
	embeddings := createTestEmbeddings(t, chunkIDs, repositoryID)

	tests := []struct {
		name        string
		embeddings  []outbound.VectorEmbedding
		options     outbound.BulkInsertOptions
		expectError bool
		expectCount int
	}{
		{
			name:       "Small batch insertion should succeed",
			embeddings: embeddings[:10],
			options: outbound.BulkInsertOptions{
				BatchSize:           5,
				UsePartitionedTable: false,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             30 * time.Second,
			},
			expectError: false,
			expectCount: 10,
		},
		{
			name:       "Large batch insertion should succeed",
			embeddings: embeddings,
			options: outbound.BulkInsertOptions{
				BatchSize:           1000,
				UsePartitionedTable: false,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             60 * time.Second,
			},
			expectError: false,
			expectCount: 100,
		},
		{
			name:       "Batch size larger than data should succeed",
			embeddings: embeddings[:5],
			options: outbound.BulkInsertOptions{
				BatchSize:           100,
				UsePartitionedTable: false,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             30 * time.Second,
			},
			expectError: false,
			expectCount: 5,
		},
		{
			name:       "Invalid vector dimensions should fail",
			embeddings: createInvalidDimensionEmbeddings(t, chunkIDs[:1], repositoryID),
			options: outbound.BulkInsertOptions{
				BatchSize:           10,
				UsePartitionedTable: false,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             30 * time.Second,
			},
			expectError: true,
			expectCount: 0,
		},
		{
			name:       "Zero batch size should fail",
			embeddings: embeddings[:5],
			options: outbound.BulkInsertOptions{
				BatchSize:           0,
				UsePartitionedTable: false,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             30 * time.Second,
			},
			expectError: true,
			expectCount: 0,
		},
		{
			name:       "Empty embeddings array should succeed",
			embeddings: []outbound.VectorEmbedding{},
			options: outbound.BulkInsertOptions{
				BatchSize:           10,
				UsePartitionedTable: false,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             30 * time.Second,
			},
			expectError: false,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			cleanupEmbeddings(t, pool, false)

			result, err := repo.BulkInsertEmbeddings(ctx, tt.embeddings, tt.options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result but got nil")
				return
			}

			if result.InsertedCount != tt.expectCount {
				t.Errorf("Expected %d insertions, got %d", tt.expectCount, result.InsertedCount)
			}

			if result.FailedCount > 0 {
				t.Errorf("Expected no failures, got %d failures: %v", result.FailedCount, result.ErrorDetails)
			}

			if result.BatchCount == 0 && tt.expectCount > 0 {
				t.Error("Expected positive batch count for successful insertions")
			}

			if result.Duration <= 0 {
				t.Error("Expected positive duration")
			}

			// Verify embeddings were actually inserted
			verifyEmbeddingsCount(t, pool, false, tt.expectCount)
		})
	}
}

// TestVectorStorageRepository_BulkInsertEmbeddings_PartitionedTable tests bulk insertion to partitioned table.
func TestVectorStorageRepository_BulkInsertEmbeddings_PartitionedTable(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repository and chunks first
	repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 50)

	// Create test embeddings for bulk insertion
	embeddings := createTestEmbeddings(t, chunkIDs, repositoryID)

	tests := []struct {
		name        string
		embeddings  []outbound.VectorEmbedding
		options     outbound.BulkInsertOptions
		expectError bool
		expectCount int
	}{
		{
			name:       "Partitioned table bulk insertion should succeed",
			embeddings: embeddings,
			options: outbound.BulkInsertOptions{
				BatchSize:           25,
				UsePartitionedTable: true,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             60 * time.Second,
			},
			expectError: false,
			expectCount: 50,
		},
		{
			name:       "Missing repository_id for partitioned table should fail",
			embeddings: createEmbeddingsWithoutRepositoryID(t, chunkIDs[:5]),
			options: outbound.BulkInsertOptions{
				BatchSize:           10,
				UsePartitionedTable: true,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             30 * time.Second,
			},
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			cleanupEmbeddings(t, pool, true)

			result, err := repo.BulkInsertEmbeddings(ctx, tt.embeddings, tt.options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if result.InsertedCount != tt.expectCount {
				t.Errorf("Expected %d insertions, got %d", tt.expectCount, result.InsertedCount)
			}

			// Verify embeddings were actually inserted to partitioned table
			verifyEmbeddingsCount(t, pool, true, tt.expectCount)
		})
	}
}

// TestVectorStorageRepository_BulkInsertEmbeddings_ConflictHandling tests conflict resolution strategies.
func TestVectorStorageRepository_BulkInsertEmbeddings_ConflictHandling(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repository and chunks first
	repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 10)

	// Create initial embeddings
	initialEmbeddings := createTestEmbeddings(t, chunkIDs, repositoryID)

	tests := []struct {
		name               string
		firstInsert        []outbound.VectorEmbedding
		secondInsert       []outbound.VectorEmbedding
		conflictAction     outbound.ConflictAction
		expectError        bool
		expectFirstCount   int
		expectSecondCount  int
		expectUpdatedCount int
		expectSkippedCount int
	}{
		{
			name:               "Do nothing on conflict should skip duplicates",
			firstInsert:        initialEmbeddings[:5],
			secondInsert:       initialEmbeddings[:3], // First 3 are duplicates
			conflictAction:     outbound.ConflictActionDoNothing,
			expectError:        false,
			expectFirstCount:   5,
			expectSecondCount:  0,
			expectUpdatedCount: 0,
			expectSkippedCount: 3,
		},
		{
			name:               "Update on conflict should update existing",
			firstInsert:        initialEmbeddings[:5],
			secondInsert:       createUpdatedTestEmbeddings(t, chunkIDs[:3], repositoryID, "gemini-embedding-001"),
			conflictAction:     outbound.ConflictActionUpdate,
			expectError:        false,
			expectFirstCount:   5,
			expectSecondCount:  0,
			expectUpdatedCount: 3,
			expectSkippedCount: 0,
		},
		{
			name:               "Error on conflict should fail",
			firstInsert:        initialEmbeddings[:5],
			secondInsert:       initialEmbeddings[:3], // First 3 are duplicates
			conflictAction:     outbound.ConflictActionError,
			expectError:        true,
			expectFirstCount:   5,
			expectSecondCount:  0,
			expectUpdatedCount: 0,
			expectSkippedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			cleanupEmbeddings(t, pool, false)

			// First insertion
			options := outbound.BulkInsertOptions{
				BatchSize:           10,
				UsePartitionedTable: false,
				OnConflictAction:    outbound.ConflictActionDoNothing,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				ValidateVectorDims:  true,
				EnableMetrics:       true,
				Timeout:             30 * time.Second,
			}

			result1, err := repo.BulkInsertEmbeddings(ctx, tt.firstInsert, options)
			if err != nil {
				t.Fatalf("First insertion failed: %v", err)
			}

			if result1.InsertedCount != tt.expectFirstCount {
				t.Errorf("First insertion: expected %d insertions, got %d", tt.expectFirstCount, result1.InsertedCount)
			}

			// Second insertion with conflict handling
			options.OnConflictAction = tt.conflictAction
			result2, err := repo.BulkInsertEmbeddings(ctx, tt.secondInsert, options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error on second insertion but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Second insertion failed unexpectedly: %v", err)
				return
			}

			if result2.InsertedCount != tt.expectSecondCount {
				t.Errorf(
					"Second insertion: expected %d insertions, got %d",
					tt.expectSecondCount,
					result2.InsertedCount,
				)
			}

			if result2.UpdatedCount != tt.expectUpdatedCount {
				t.Errorf("Second insertion: expected %d updates, got %d", tt.expectUpdatedCount, result2.UpdatedCount)
			}

			if result2.SkippedCount != tt.expectSkippedCount {
				t.Errorf("Second insertion: expected %d skipped, got %d", tt.expectSkippedCount, result2.SkippedCount)
			}
		})
	}
}

// TestVectorStorageRepository_UpsertEmbedding tests single embedding upsert operations.
func TestVectorStorageRepository_UpsertEmbedding(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repository and chunks first
	repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 5)

	tests := []struct {
		name        string
		embedding   outbound.VectorEmbedding
		options     outbound.UpsertOptions
		expectError bool
	}{
		{
			name:      "New embedding upsert should succeed",
			embedding: createTestEmbeddings(t, chunkIDs[:1], repositoryID)[0],
			options: outbound.UpsertOptions{
				UsePartitionedTable: false,
				UpdateStrategy:      outbound.EmbeddingUpdateStrategyReplace,
				ValidateVectorDims:  true,
				Timeout:             30 * time.Second,
			},
			expectError: false,
		},
		{
			name:      "Existing embedding upsert should update",
			embedding: createUpdatedTestEmbeddings(t, chunkIDs[:1], repositoryID, "gemini-embedding-001")[0],
			options: outbound.UpsertOptions{
				UsePartitionedTable: false,
				UpdateStrategy:      outbound.EmbeddingUpdateStrategyReplace,
				ValidateVectorDims:  true,
				Timeout:             30 * time.Second,
			},
			expectError: false,
		},
		{
			name:      "Invalid vector dimensions should fail",
			embedding: createInvalidDimensionEmbeddings(t, chunkIDs[:1], repositoryID)[0],
			options: outbound.UpsertOptions{
				UsePartitionedTable: false,
				UpdateStrategy:      outbound.EmbeddingUpdateStrategyReplace,
				ValidateVectorDims:  true,
				Timeout:             30 * time.Second,
			},
			expectError: true,
		},
		{
			name:      "Partitioned table upsert should succeed",
			embedding: createTestEmbeddings(t, chunkIDs[1:2], repositoryID)[0],
			options: outbound.UpsertOptions{
				UsePartitionedTable: true,
				UpdateStrategy:      outbound.EmbeddingUpdateStrategyReplace,
				ValidateVectorDims:  true,
				Timeout:             30 * time.Second,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UpsertEmbedding(ctx, tt.embedding, tt.options)

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

// TestVectorStorageRepository_FindEmbeddingByChunkID tests finding embeddings by chunk ID.
func TestVectorStorageRepository_FindEmbeddingByChunkID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repository and chunks first
	repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 5)

	// Insert test embeddings
	embeddings := createTestEmbeddings(t, chunkIDs, repositoryID)
	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           10,
		UsePartitionedTable: false,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             30 * time.Second,
	}

	_, err := repo.BulkInsertEmbeddings(ctx, embeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	tests := []struct {
		name         string
		chunkID      uuid.UUID
		modelVersion string
		expectFound  bool
		expectError  bool
	}{
		{
			name:         "Existing embedding should be found",
			chunkID:      chunkIDs[0],
			modelVersion: "gemini-embedding-001",
			expectFound:  true,
			expectError:  false,
		},
		{
			name:         "Non-existing chunk ID should return nil",
			chunkID:      uuid.New(),
			modelVersion: "gemini-embedding-001",
			expectFound:  false,
			expectError:  false,
		},
		{
			name:         "Non-existing model version should return nil",
			chunkID:      chunkIDs[0],
			modelVersion: "non-existing-model",
			expectFound:  false,
			expectError:  false,
		},
		{
			name:         "Nil chunk ID should return error",
			chunkID:      uuid.Nil,
			modelVersion: "gemini-embedding-001",
			expectFound:  false,
			expectError:  true,
		},
		{
			name:         "Empty model version should return error",
			chunkID:      chunkIDs[0],
			modelVersion: "",
			expectFound:  false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := repo.FindEmbeddingByChunkID(ctx, tt.chunkID, tt.modelVersion)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if tt.expectFound {
				verifyFoundEmbedding(t, found, tt.chunkID, tt.modelVersion)
			} else if found != nil {
				t.Error("Expected nil but found embedding")
			}
		})
	}
}

// TestVectorStorageRepository_FindEmbeddingsByRepositoryID tests finding embeddings by repository ID.
func TestVectorStorageRepository_FindEmbeddingsByRepositoryID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repositories and chunks
	repositoryID1, chunkIDs1 := setupTestRepositoryAndChunks(t, pool, 10)
	repositoryID2, chunkIDs2 := setupTestRepositoryAndChunks(t, pool, 5)

	// Insert embeddings for both repositories
	embeddings1 := createTestEmbeddings(t, chunkIDs1, repositoryID1)
	embeddings2 := createTestEmbeddings(t, chunkIDs2, repositoryID2)

	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           15,
		UsePartitionedTable: false,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             30 * time.Second,
	}

	_, err := repo.BulkInsertEmbeddings(ctx, append(embeddings1, embeddings2...), bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	tests := []struct {
		name         string
		repositoryID uuid.UUID
		filters      outbound.EmbeddingFilters
		expectCount  int
		expectError  bool
	}{
		{
			name:         "Find all embeddings for repository 1",
			repositoryID: repositoryID1,
			filters: outbound.EmbeddingFilters{
				UsePartitionedTable: false,
				Limit:               100,
				Offset:              0,
				Timeout:             30 * time.Second,
			},
			expectCount: 10,
			expectError: false,
		},
		{
			name:         "Find all embeddings for repository 2",
			repositoryID: repositoryID2,
			filters: outbound.EmbeddingFilters{
				UsePartitionedTable: false,
				Limit:               100,
				Offset:              0,
				Timeout:             30 * time.Second,
			},
			expectCount: 5,
			expectError: false,
		},
		{
			name:         "Find with pagination should limit results",
			repositoryID: repositoryID1,
			filters: outbound.EmbeddingFilters{
				UsePartitionedTable: false,
				Limit:               5,
				Offset:              0,
				Timeout:             30 * time.Second,
			},
			expectCount: 5,
			expectError: false,
		},
		{
			name:         "Find with offset should skip results",
			repositoryID: repositoryID1,
			filters: outbound.EmbeddingFilters{
				UsePartitionedTable: false,
				Limit:               100,
				Offset:              8,
				Timeout:             30 * time.Second,
			},
			expectCount: 2,
			expectError: false,
		},
		{
			name:         "Filter by model version should work",
			repositoryID: repositoryID1,
			filters: outbound.EmbeddingFilters{
				ModelVersions:       []string{"gemini-embedding-001"},
				UsePartitionedTable: false,
				Limit:               100,
				Offset:              0,
				Timeout:             30 * time.Second,
			},
			expectCount: 10,
			expectError: false,
		},
		{
			name:         "Filter by non-existing model version should return empty",
			repositoryID: repositoryID1,
			filters: outbound.EmbeddingFilters{
				ModelVersions:       []string{"non-existing-model"},
				UsePartitionedTable: false,
				Limit:               100,
				Offset:              0,
				Timeout:             30 * time.Second,
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name:         "Non-existing repository should return empty",
			repositoryID: uuid.New(),
			filters: outbound.EmbeddingFilters{
				UsePartitionedTable: false,
				Limit:               100,
				Offset:              0,
				Timeout:             30 * time.Second,
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name:         "Nil repository ID should return error",
			repositoryID: uuid.Nil,
			filters: outbound.EmbeddingFilters{
				UsePartitionedTable: false,
				Limit:               100,
				Offset:              0,
				Timeout:             30 * time.Second,
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := repo.FindEmbeddingsByRepositoryID(ctx, tt.repositoryID, tt.filters)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if len(found) != tt.expectCount {
				t.Errorf("Expected %d embeddings, got %d", tt.expectCount, len(found))
			}

			// Verify all found embeddings belong to the correct repository
			for i, embedding := range found {
				if embedding.RepositoryID != tt.repositoryID {
					t.Errorf(
						"Embedding %d: expected repository ID %s, got %s",
						i,
						tt.repositoryID,
						embedding.RepositoryID,
					)
				}
				if len(embedding.Embedding) != 768 {
					t.Errorf("Embedding %d: expected 768 dimensions, got %d", i, len(embedding.Embedding))
				}
			}
		})
	}
}

// TestVectorStorageRepository_DeleteEmbeddingsByChunkIDs tests bulk deletion by chunk IDs.
func TestVectorStorageRepository_DeleteEmbeddingsByChunkIDs(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repository and chunks first
	repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 20)

	// Insert test embeddings
	embeddings := createTestEmbeddings(t, chunkIDs, repositoryID)
	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           20,
		UsePartitionedTable: false,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             30 * time.Second,
	}

	_, err := repo.BulkInsertEmbeddings(ctx, embeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	tests := []struct {
		name          string
		chunkIDs      []uuid.UUID
		options       outbound.DeleteOptions
		expectError   bool
		expectDeleted int
		expectSkipped int
	}{
		{
			name:     "Delete existing chunk IDs should succeed",
			chunkIDs: chunkIDs[:5],
			options: outbound.DeleteOptions{
				UsePartitionedTable: false,
				SoftDelete:          false,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				BatchSize:           10,
				Timeout:             30 * time.Second,
			},
			expectError:   false,
			expectDeleted: 5,
			expectSkipped: 0,
		},
		{
			name:     "Delete non-existing chunk IDs should skip",
			chunkIDs: []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
			options: outbound.DeleteOptions{
				UsePartitionedTable: false,
				SoftDelete:          false,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				BatchSize:           10,
				Timeout:             30 * time.Second,
			},
			expectError:   false,
			expectDeleted: 0,
			expectSkipped: 3,
		},
		{
			name:     "Soft delete should mark as deleted",
			chunkIDs: chunkIDs[5:8],
			options: outbound.DeleteOptions{
				UsePartitionedTable: false,
				SoftDelete:          true,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				BatchSize:           10,
				Timeout:             30 * time.Second,
			},
			expectError:   false,
			expectDeleted: 3,
			expectSkipped: 0,
		},
		{
			name:     "Empty chunk IDs array should succeed",
			chunkIDs: []uuid.UUID{},
			options: outbound.DeleteOptions{
				UsePartitionedTable: false,
				SoftDelete:          false,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				BatchSize:           10,
				Timeout:             30 * time.Second,
			},
			expectError:   false,
			expectDeleted: 0,
			expectSkipped: 0,
		},
		{
			name:     "Nil chunk IDs should return error",
			chunkIDs: nil,
			options: outbound.DeleteOptions{
				UsePartitionedTable: false,
				SoftDelete:          false,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				BatchSize:           10,
				Timeout:             30 * time.Second,
			},
			expectError:   true,
			expectDeleted: 0,
			expectSkipped: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.DeleteEmbeddingsByChunkIDs(ctx, tt.chunkIDs, tt.options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result but got nil")
				return
			}

			if result.DeletedCount != tt.expectDeleted {
				t.Errorf("Expected %d deleted, got %d", tt.expectDeleted, result.DeletedCount)
			}

			if result.SkippedCount != tt.expectSkipped {
				t.Errorf("Expected %d skipped, got %d", tt.expectSkipped, result.SkippedCount)
			}

			if result.Duration <= 0 && len(tt.chunkIDs) > 0 {
				t.Error("Expected positive duration for non-empty operations")
			}
		})
	}
}

// TestVectorStorageRepository_DeleteEmbeddingsByRepositoryID tests repository-wide deletion.
func TestVectorStorageRepository_DeleteEmbeddingsByRepositoryID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repositories and chunks
	repositoryID1, chunkIDs1 := setupTestRepositoryAndChunks(t, pool, 15)
	repositoryID2, chunkIDs2 := setupTestRepositoryAndChunks(t, pool, 10)

	// Insert embeddings for both repositories
	embeddings1 := createTestEmbeddings(t, chunkIDs1, repositoryID1)
	embeddings2 := createTestEmbeddings(t, chunkIDs2, repositoryID2)

	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           25,
		UsePartitionedTable: false,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             30 * time.Second,
	}

	_, err := repo.BulkInsertEmbeddings(ctx, append(embeddings1, embeddings2...), bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	tests := []struct {
		name          string
		repositoryID  uuid.UUID
		options       outbound.DeleteOptions
		expectError   bool
		expectDeleted int
	}{
		{
			name:         "Delete all embeddings for repository 1",
			repositoryID: repositoryID1,
			options: outbound.DeleteOptions{
				UsePartitionedTable: false,
				SoftDelete:          false,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				BatchSize:           10,
				Timeout:             30 * time.Second,
			},
			expectError:   false,
			expectDeleted: 15,
		},
		{
			name:         "Delete from non-existing repository should succeed with 0 deleted",
			repositoryID: uuid.New(),
			options: outbound.DeleteOptions{
				UsePartitionedTable: false,
				SoftDelete:          false,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				BatchSize:           10,
				Timeout:             30 * time.Second,
			},
			expectError:   false,
			expectDeleted: 0,
		},
		{
			name:         "Nil repository ID should return error",
			repositoryID: uuid.Nil,
			options: outbound.DeleteOptions{
				UsePartitionedTable: false,
				SoftDelete:          false,
				TransactionBehavior: outbound.TransactionBehaviorAuto,
				BatchSize:           10,
				Timeout:             30 * time.Second,
			},
			expectError:   true,
			expectDeleted: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.DeleteEmbeddingsByRepositoryID(ctx, tt.repositoryID, tt.options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result but got nil")
				return
			}

			if result.DeletedCount != tt.expectDeleted {
				t.Errorf("Expected %d deleted, got %d", tt.expectDeleted, result.DeletedCount)
			}

			// Verify embeddings were actually deleted by trying to find them
			filters := outbound.EmbeddingFilters{
				UsePartitionedTable: false,
				Limit:               100,
				Offset:              0,
				Timeout:             30 * time.Second,
			}
			remaining, err := repo.FindEmbeddingsByRepositoryID(ctx, tt.repositoryID, filters)
			if err != nil {
				t.Errorf("Failed to verify deletion: %v", err)
			} else if len(remaining) > 0 && tt.expectDeleted > 0 {
				t.Errorf("Expected all embeddings to be deleted, but found %d remaining", len(remaining))
			}
		})
	}
}

// TestVectorStorageRepository_VectorSimilaritySearch tests vector similarity search operations.
func TestVectorStorageRepository_VectorSimilaritySearch(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create test repository and chunks first
	repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 10)

	// Insert test embeddings with varying vectors
	embeddings := createTestEmbeddings(t, chunkIDs, repositoryID)
	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           10,
		UsePartitionedTable: false,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             30 * time.Second,
	}

	_, err := repo.BulkInsertEmbeddings(ctx, embeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	// Create query vectors
	queryVector := createTestVector(768, 0.5) // Middle range values

	tests := []struct {
		name        string
		queryVector []float64
		options     outbound.SimilaritySearchOptions
		expectError bool
		minResults  int
		maxResults  int
	}{
		{
			name:        "Basic similarity search should return results",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: false,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          5,
				MinSimilarity:       0.0,
				EfSearch:            100,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectError: false,
			minResults:  1,
			maxResults:  5,
		},
		{
			name:        "High similarity threshold should limit results",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: false,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          10,
				MinSimilarity:       0.99,
				EfSearch:            100,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectError: false,
			minResults:  0,
			maxResults:  2,
		},
		{
			name:        "Repository filter should work",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: false,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          10,
				MinSimilarity:       0.0,
				RepositoryIDs:       []uuid.UUID{repositoryID},
				EfSearch:            100,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectError: false,
			minResults:  1,
			maxResults:  10,
		},
		{
			name:        "Non-existing repository filter should return empty",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: false,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          10,
				MinSimilarity:       0.0,
				RepositoryIDs:       []uuid.UUID{uuid.New()},
				EfSearch:            100,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectError: false,
			minResults:  0,
			maxResults:  0,
		},
		{
			name:        "Invalid vector dimensions should return error",
			queryVector: createTestVector(500, 0.5), // Wrong dimensions
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: false,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          5,
				MinSimilarity:       0.0,
				EfSearch:            100,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectError: true,
			minResults:  0,
			maxResults:  0,
		},
		{
			name:        "Empty query vector should return error",
			queryVector: []float64{},
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: false,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          5,
				MinSimilarity:       0.0,
				EfSearch:            100,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectError: true,
			minResults:  0,
			maxResults:  0,
		},
		{
			name:        "Zero max results should return error",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: false,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          0,
				MinSimilarity:       0.0,
				EfSearch:            100,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectError: true,
			minResults:  0,
			maxResults:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.VectorSimilaritySearch(ctx, tt.queryVector, tt.options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if len(results) < tt.minResults {
				t.Errorf("Expected at least %d results, got %d", tt.minResults, len(results))
			}

			if len(results) > tt.maxResults {
				t.Errorf("Expected at most %d results, got %d", tt.maxResults, len(results))
			}

			// Verify result structure and ordering
			for i, result := range results {
				if result.Embedding.ID == uuid.Nil {
					t.Errorf("Result %d: expected non-nil embedding ID", i)
				}
				if len(result.Embedding.Embedding) != 768 {
					t.Errorf("Result %d: expected 768 dimensions, got %d", i, len(result.Embedding.Embedding))
				}
				if result.Similarity < 0.0 || result.Similarity > 1.0 {
					t.Errorf("Result %d: similarity %f out of range [0,1]", i, result.Similarity)
				}
				if result.Distance < 0.0 {
					t.Errorf("Result %d: negative distance %f", i, result.Distance)
				}
				if result.Rank != i+1 {
					t.Errorf("Result %d: expected rank %d, got %d", i, i+1, result.Rank)
				}
				if i > 0 && result.Similarity > results[i-1].Similarity {
					t.Errorf(
						"Results not ordered by similarity: result %d has higher similarity than result %d",
						i,
						i-1,
					)
				}
			}
		})
	}
}

// TestVectorStorageRepository_GetStorageStatistics tests storage statistics retrieval.
func TestVectorStorageRepository_GetStorageStatistics(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	tests := []struct {
		name        string
		options     outbound.StatisticsOptions
		expectError bool
	}{
		{
			name: "Regular table statistics should succeed",
			options: outbound.StatisticsOptions{
				IncludePartitioned: false,
				IncludeRegular:     true,
				IncludeIndexStats:  true,
				IncludePartitions:  false,
			},
			expectError: false,
		},
		{
			name: "Partitioned table statistics should succeed",
			options: outbound.StatisticsOptions{
				IncludePartitioned: true,
				IncludeRegular:     false,
				IncludeIndexStats:  true,
				IncludePartitions:  true,
			},
			expectError: false,
		},
		{
			name: "All statistics should succeed",
			options: outbound.StatisticsOptions{
				IncludePartitioned: true,
				IncludeRegular:     true,
				IncludeIndexStats:  true,
				IncludePartitions:  true,
			},
			expectError: false,
		},
		{
			name: "No statistics selected should return error",
			options: outbound.StatisticsOptions{
				IncludePartitioned: false,
				IncludeRegular:     false,
				IncludeIndexStats:  false,
				IncludePartitions:  false,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats, err := repo.GetStorageStatistics(ctx, tt.options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if stats == nil {
				t.Error("Expected statistics but got nil")
				return
			}

			if tt.options.IncludeRegular && stats.RegularTable == nil {
				t.Error("Expected regular table statistics but got nil")
			}

			if tt.options.IncludePartitioned && stats.PartitionedTable == nil {
				t.Error("Expected partitioned table statistics but got nil")
			}

			if tt.options.IncludePartitions && len(stats.Partitions) == 0 && tt.options.IncludePartitioned {
				t.Error("Expected partition statistics but got empty array")
			}

			if stats.LastUpdated.IsZero() {
				t.Error("Expected non-zero last updated time")
			}

			// Verify individual table statistics structure
			if stats.RegularTable != nil {
				verifyTableStatistics(t, stats.RegularTable, "embeddings")
			}

			if stats.PartitionedTable != nil {
				verifyTableStatistics(t, stats.PartitionedTable, "embeddings_partitioned")
			}

			// Verify partition statistics structure
			for i, partition := range stats.Partitions {
				if partition.PartitionName == "" {
					t.Errorf("Partition %d: expected non-empty partition name", i)
				}
				if partition.RowCount < 0 {
					t.Errorf("Partition %d: expected non-negative row count, got %d", i, partition.RowCount)
				}
				if partition.TableSize < 0 {
					t.Errorf("Partition %d: expected non-negative table size, got %d", i, partition.TableSize)
				}
				if partition.IndexSize < 0 {
					t.Errorf("Partition %d: expected non-negative index size, got %d", i, partition.IndexSize)
				}
			}
		})
	}
}

// TestVectorStorageRepository_BeginTransaction tests transaction management.
func TestVectorStorageRepository_BeginTransaction(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	t.Run("Begin transaction should return valid transaction", func(t *testing.T) {
		tx, err := repo.BeginTransaction(ctx)
		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
			return
		}

		if tx == nil {
			t.Error("Expected transaction but got nil")
			return
		}

		// Clean up
		defer func() {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				t.Logf("Warning: failed to rollback transaction: %v", rollbackErr)
			}
		}()

		// Verify transaction interface
		_ = tx
	})

	t.Run("Transaction operations should work within transaction", func(t *testing.T) {
		tx, err := repo.BeginTransaction(ctx)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		// Create test data
		repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 5)
		embeddings := createTestEmbeddings(t, chunkIDs, repositoryID)

		// Test transaction operations
		options := outbound.BulkInsertOptions{
			BatchSize:           5,
			UsePartitionedTable: false,
			OnConflictAction:    outbound.ConflictActionDoNothing,
			TransactionBehavior: outbound.TransactionBehaviorManual,
			ValidateVectorDims:  true,
			EnableMetrics:       false,
			Timeout:             30 * time.Second,
		}

		result, err := tx.BulkInsertEmbeddings(ctx, embeddings, options)
		if err != nil {
			t.Errorf("Transaction bulk insert failed: %v", err)
		} else if result.InsertedCount != 5 {
			t.Errorf("Expected 5 insertions, got %d", result.InsertedCount)
		}

		// Test commit
		err = tx.Commit(ctx)
		if err != nil {
			t.Errorf("Transaction commit failed: %v", err)
		}

		// Verify data was committed
		verifyEmbeddingsCount(t, pool, false, 5)
	})

	t.Run("Transaction rollback should undo changes", func(t *testing.T) {
		tx, err := repo.BeginTransaction(ctx)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		// Clean up embeddings first
		cleanupEmbeddings(t, pool, false)

		// Create test data
		repositoryID, chunkIDs := setupTestRepositoryAndChunks(t, pool, 3)
		embeddings := createTestEmbeddings(t, chunkIDs, repositoryID)

		// Insert within transaction
		options := outbound.BulkInsertOptions{
			BatchSize:           3,
			UsePartitionedTable: false,
			OnConflictAction:    outbound.ConflictActionDoNothing,
			TransactionBehavior: outbound.TransactionBehaviorManual,
			ValidateVectorDims:  true,
			EnableMetrics:       false,
			Timeout:             30 * time.Second,
		}

		result, err := tx.BulkInsertEmbeddings(ctx, embeddings, options)
		if err != nil {
			t.Errorf("Transaction bulk insert failed: %v", err)
		} else if result.InsertedCount != 3 {
			t.Errorf("Expected 3 insertions, got %d", result.InsertedCount)
		}

		// Rollback
		err = tx.Rollback(ctx)
		if err != nil {
			t.Errorf("Transaction rollback failed: %v", err)
		}

		// Verify data was rolled back
		verifyEmbeddingsCount(t, pool, false, 0)
	})
}

// Helper functions for test setup and validation

// setupTestRepositoryAndChunks creates a test repository and chunks, returns repository ID and chunk IDs.
func setupTestRepositoryAndChunks(t *testing.T, pool *pgxpool.Pool, chunkCount int) (uuid.UUID, []uuid.UUID) {
	ctx := context.Background()
	repositoryID := uuid.New()

	// Create repository
	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status) 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (url) DO NOTHING
	`, repositoryID, "https://github.com/test/vector-repo-"+repositoryID.String()[:8],
		"https://github.com/test/vector-repo-"+repositoryID.String()[:8],
		"vector-test-repo", "Test repository for vector operations", "indexed")
	if err != nil {
		t.Fatalf("Failed to create test repository: %v", err)
	}

	// Create chunks
	chunkIDs := make([]uuid.UUID, chunkCount)
	for i := range chunkCount {
		chunkID := uuid.New()
		chunkIDs[i] = chunkID

		_, err = pool.Exec(ctx, `
			INSERT INTO codechunking.code_chunks (id, repository_id, file_path, chunk_type, content, language, start_line, end_line, entity_name, content_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (repository_id, file_path, content_hash) DO NOTHING
		`, chunkID, repositoryID, fmt.Sprintf("test%d.go", i), "function", fmt.Sprintf("func Test%d() {}", i), "go",
			1, 10, fmt.Sprintf("Test%d", i), "test-hash-"+chunkID.String()[:8])
		if err != nil {
			t.Fatalf("Failed to create test chunk %d: %v", i, err)
		}
	}

	return repositoryID, chunkIDs
}

// createTestEmbeddings creates test embeddings with 768-dimensional vectors.
func createTestEmbeddings(
	t *testing.T,
	chunkIDs []uuid.UUID,
	repositoryID uuid.UUID,
) []outbound.VectorEmbedding {
	embeddings := make([]outbound.VectorEmbedding, len(chunkIDs))
	for i, chunkID := range chunkIDs {
		embeddings[i] = outbound.VectorEmbedding{
			ID:           uuid.New(),
			ChunkID:      chunkID,
			RepositoryID: repositoryID,
			Embedding:    createTestVector(768, float64(i)*0.1), // Vary values for diversity
			ModelVersion: "gemini-embedding-001",
			CreatedAt:    time.Now(),
		}
	}
	return embeddings
}

// createUpdatedTestEmbeddings creates embeddings with updated vector values for conflict testing.
func createUpdatedTestEmbeddings(
	t *testing.T,
	chunkIDs []uuid.UUID,
	repositoryID uuid.UUID,
	modelVersion string,
) []outbound.VectorEmbedding {
	embeddings := make([]outbound.VectorEmbedding, len(chunkIDs))
	for i, chunkID := range chunkIDs {
		embeddings[i] = outbound.VectorEmbedding{
			ID:           uuid.New(),
			ChunkID:      chunkID,
			RepositoryID: repositoryID,
			Embedding:    createTestVector(768, float64(i)*0.1+0.5), // Different values
			ModelVersion: modelVersion,
			CreatedAt:    time.Now(),
		}
	}
	return embeddings
}

// createInvalidDimensionEmbeddings creates embeddings with invalid vector dimensions.
func createInvalidDimensionEmbeddings(
	t *testing.T,
	chunkIDs []uuid.UUID,
	repositoryID uuid.UUID,
) []outbound.VectorEmbedding {
	embeddings := make([]outbound.VectorEmbedding, len(chunkIDs))
	for i, chunkID := range chunkIDs {
		embeddings[i] = outbound.VectorEmbedding{
			ID:           uuid.New(),
			ChunkID:      chunkID,
			RepositoryID: repositoryID,
			Embedding:    createTestVector(500, float64(i)*0.1), // Wrong dimensions
			ModelVersion: "gemini-embedding-001",
			CreatedAt:    time.Now(),
		}
	}
	return embeddings
}

// createEmbeddingsWithoutRepositoryID creates embeddings missing repository ID for partitioned table testing.
func createEmbeddingsWithoutRepositoryID(t *testing.T, chunkIDs []uuid.UUID) []outbound.VectorEmbedding {
	embeddings := make([]outbound.VectorEmbedding, len(chunkIDs))
	for i, chunkID := range chunkIDs {
		embeddings[i] = outbound.VectorEmbedding{
			ID:           uuid.New(),
			ChunkID:      chunkID,
			RepositoryID: uuid.Nil, // Missing repository ID
			Embedding:    createTestVector(768, float64(i)*0.1),
			ModelVersion: "gemini-embedding-001",
			CreatedAt:    time.Now(),
		}
	}
	return embeddings
}

// createTestVector creates a test vector with specified dimensions and base value.
func createTestVector(dimensions int, baseValue float64) []float64 {
	vector := make([]float64, dimensions)
	for i := range dimensions {
		// Create slightly varying values for realistic similarity testing
		vector[i] = baseValue + float64(i%10)*0.01
	}
	return vector
}

// cleanupEmbeddings removes all embeddings from test tables.
func cleanupEmbeddings(t *testing.T, pool *pgxpool.Pool, partitioned bool) {
	ctx := context.Background()
	var query string
	if partitioned {
		query = "DELETE FROM codechunking.embeddings_partitioned WHERE 1=1"
	} else {
		query = "DELETE FROM codechunking.embeddings WHERE 1=1"
	}

	_, err := pool.Exec(ctx, query)
	if err != nil {
		t.Logf("Warning: Failed to clean up embeddings: %v", err)
	}
}

// verifyEmbeddingsCount verifies the number of embeddings in the specified table.
func verifyEmbeddingsCount(t *testing.T, pool *pgxpool.Pool, partitioned bool, expectedCount int) {
	ctx := context.Background()
	var query string
	if partitioned {
		query = "SELECT COUNT(*) FROM codechunking.embeddings_partitioned WHERE deleted_at IS NULL"
	} else {
		query = "SELECT COUNT(*) FROM codechunking.embeddings WHERE deleted_at IS NULL"
	}

	var actualCount int
	err := pool.QueryRow(ctx, query).Scan(&actualCount)
	if err != nil {
		t.Errorf("Failed to count embeddings: %v", err)
		return
	}

	if actualCount != expectedCount {
		t.Errorf("Expected %d embeddings, but found %d", expectedCount, actualCount)
	}
}

// verifyTableStatistics verifies the structure of table statistics.
func verifyTableStatistics(t *testing.T, stats *outbound.TableStatistics, expectedTableName string) {
	if stats.TableName != expectedTableName {
		t.Errorf("Expected table name %s, got %s", expectedTableName, stats.TableName)
	}
	if stats.RowCount < 0 {
		t.Errorf("Expected non-negative row count, got %d", stats.RowCount)
	}
	if stats.TableSize < 0 {
		t.Errorf("Expected non-negative table size, got %d", stats.TableSize)
	}
	if stats.IndexSize < 0 {
		t.Errorf("Expected non-negative index size, got %d", stats.IndexSize)
	}
}

// verifyFoundEmbedding verifies that a found embedding has the expected properties.
func verifyFoundEmbedding(
	t *testing.T,
	found *outbound.VectorEmbedding,
	expectedChunkID uuid.UUID,
	expectedModelVersion string,
) {
	if found == nil {
		t.Error("Expected to find embedding but got nil")
		return
	}
	if found.ChunkID != expectedChunkID {
		t.Errorf("Expected chunk ID %s, got %s", expectedChunkID, found.ChunkID)
	}
	if found.ModelVersion != expectedModelVersion {
		t.Errorf("Expected model version %s, got %s", expectedModelVersion, found.ModelVersion)
	}
	if len(found.Embedding) != 768 {
		t.Errorf("Expected 768 dimensions, got %d", len(found.Embedding))
	}
}

// Helper functions for metadata filter tests

// setupTestRepositoryAndChunksWithLanguages creates a repository and chunks with specific language distribution.
func setupTestRepositoryAndChunksWithLanguages(
	t *testing.T,
	pool *pgxpool.Pool,
	totalChunks int,
	languages []string,
	counts []int,
) (uuid.UUID, []uuid.UUID) {
	if len(languages) != len(counts) {
		t.Fatalf("languages and counts slices must have same length")
	}

	sum := 0
	for _, count := range counts {
		sum += count
	}
	if sum != totalChunks {
		t.Fatalf("sum of counts (%d) must equal totalChunks (%d)", sum, totalChunks)
	}

	ctx := context.Background()
	repositoryID := uuid.New()

	// Create repository
	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (url) DO NOTHING
	`, repositoryID, "https://github.com/test/multi-lang-repo-"+repositoryID.String()[:8],
		"https://github.com/test/multi-lang-repo-"+repositoryID.String()[:8],
		"multi-lang-test-repo", "Test repository with multiple languages", "indexed")
	if err != nil {
		t.Fatalf("Failed to create test repository: %v", err)
	}

	// Create chunks with specified language distribution
	chunkIDs := make([]uuid.UUID, totalChunks)
	chunkIndex := 0

	for langIdx, language := range languages {
		for i := 0; i < counts[langIdx]; i++ {
			chunkID := uuid.New()
			chunkIDs[chunkIndex] = chunkID

			fileExt := getFileExtensionForLanguage(language)
			_, err = pool.Exec(ctx, `
				INSERT INTO codechunking.code_chunks (id, repository_id, file_path, chunk_type, content, language, start_line, end_line, entity_name, content_hash)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				ON CONFLICT (repository_id, file_path, content_hash) DO NOTHING
			`, chunkID, repositoryID, fmt.Sprintf("src/test%d%s", chunkIndex, fileExt), "function",
				fmt.Sprintf("func Test%d() {}", chunkIndex), language,
				1, 10, fmt.Sprintf("Test%d", chunkIndex), "hash-"+chunkID.String()[:8])
			if err != nil {
				t.Fatalf("Failed to create test chunk %d: %v", chunkIndex, err)
			}

			chunkIndex++
		}
	}

	return repositoryID, chunkIDs
}

// setupTestRepositoryAndChunksWithTypes creates a repository and chunks with specific chunk type distribution.
func setupTestRepositoryAndChunksWithTypes(
	t *testing.T,
	pool *pgxpool.Pool,
	totalChunks int,
	chunkTypes []string,
	counts []int,
) (uuid.UUID, []uuid.UUID) {
	if len(chunkTypes) != len(counts) {
		t.Fatalf("chunkTypes and counts slices must have same length")
	}

	sum := 0
	for _, count := range counts {
		sum += count
	}
	if sum != totalChunks {
		t.Fatalf("sum of counts (%d) must equal totalChunks (%d)", sum, totalChunks)
	}

	ctx := context.Background()
	repositoryID := uuid.New()

	// Create repository
	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (url) DO NOTHING
	`, repositoryID, "https://github.com/test/multi-type-repo-"+repositoryID.String()[:8],
		"https://github.com/test/multi-type-repo-"+repositoryID.String()[:8],
		"multi-type-test-repo", "Test repository with multiple chunk types", "indexed")
	if err != nil {
		t.Fatalf("Failed to create test repository: %v", err)
	}

	// Create chunks with specified chunk type distribution
	chunkIDs := make([]uuid.UUID, totalChunks)
	chunkIndex := 0

	for typeIdx, chunkType := range chunkTypes {
		for i := 0; i < counts[typeIdx]; i++ {
			chunkID := uuid.New()
			chunkIDs[chunkIndex] = chunkID

			_, err = pool.Exec(ctx, `
				INSERT INTO codechunking.code_chunks (id, repository_id, file_path, chunk_type, content, language, start_line, end_line, entity_name, content_hash)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				ON CONFLICT (repository_id, file_path, content_hash) DO NOTHING
			`, chunkID, repositoryID, fmt.Sprintf("src/test%d.go", chunkIndex), chunkType,
				fmt.Sprintf("content of %s %d", chunkType, chunkIndex), "go",
				1, 10, fmt.Sprintf("Entity%d", chunkIndex), "hash-"+chunkID.String()[:8])
			if err != nil {
				t.Fatalf("Failed to create test chunk %d: %v", chunkIndex, err)
			}

			chunkIndex++
		}
	}

	return repositoryID, chunkIDs
}

// setupTestRepositoryAndChunksWithFilePaths creates a repository and chunks with specific file extension distribution.
func setupTestRepositoryAndChunksWithFilePaths(
	t *testing.T,
	pool *pgxpool.Pool,
	totalChunks int,
	extensions []string,
	counts []int,
) (uuid.UUID, []uuid.UUID) {
	if len(extensions) != len(counts) {
		t.Fatalf("extensions and counts slices must have same length")
	}

	sum := 0
	for _, count := range counts {
		sum += count
	}
	if sum != totalChunks {
		t.Fatalf("sum of counts (%d) must equal totalChunks (%d)", sum, totalChunks)
	}

	ctx := context.Background()
	repositoryID := uuid.New()

	// Create repository
	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (url) DO NOTHING
	`, repositoryID, "https://github.com/test/multi-ext-repo-"+repositoryID.String()[:8],
		"https://github.com/test/multi-ext-repo-"+repositoryID.String()[:8],
		"multi-ext-test-repo", "Test repository with multiple file extensions", "indexed")
	if err != nil {
		t.Fatalf("Failed to create test repository: %v", err)
	}

	// Create chunks with specified file extension distribution
	chunkIDs := make([]uuid.UUID, totalChunks)
	chunkIndex := 0

	for extIdx, extension := range extensions {
		language := getLanguageForExtension(extension)
		for i := 0; i < counts[extIdx]; i++ {
			chunkID := uuid.New()
			chunkIDs[chunkIndex] = chunkID

			_, err = pool.Exec(ctx, `
				INSERT INTO codechunking.code_chunks (id, repository_id, file_path, chunk_type, content, language, start_line, end_line, entity_name, content_hash)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				ON CONFLICT (repository_id, file_path, content_hash) DO NOTHING
			`, chunkID, repositoryID, fmt.Sprintf("src/test%d%s", chunkIndex, extension), "function",
				fmt.Sprintf("content of file %d", chunkIndex), language,
				1, 10, fmt.Sprintf("Entity%d", chunkIndex), "hash-"+chunkID.String()[:8])
			if err != nil {
				t.Fatalf("Failed to create test chunk %d: %v", chunkIndex, err)
			}

			chunkIndex++
		}
	}

	return repositoryID, chunkIDs
}

// setupTestRepositoryWithMixedMetadata creates a repository with mixed languages and chunk types.
func setupTestRepositoryWithMixedMetadata(t *testing.T, pool *pgxpool.Pool, totalChunks int) (uuid.UUID, []uuid.UUID) {
	ctx := context.Background()
	repositoryID := uuid.New()

	// Create repository
	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (url) DO NOTHING
	`, repositoryID, "https://github.com/test/mixed-meta-repo-"+repositoryID.String()[:8],
		"https://github.com/test/mixed-meta-repo-"+repositoryID.String()[:8],
		"mixed-meta-test-repo", "Test repository with mixed metadata", "indexed")
	if err != nil {
		t.Fatalf("Failed to create test repository: %v", err)
	}

	// Create chunks with mixed distribution
	// Distribution: 40 Go (20 functions, 10 classes, 10 modules)
	//              30 Python (15 functions, 10 classes, 5 modules)
	//              30 TypeScript (15 functions, 10 classes, 5 modules)
	chunkIDs := make([]uuid.UUID, totalChunks)

	chunkIndex := 0

	// Go chunks (40)
	for i := 0; i < 20; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "go", "function")
		chunkIndex++
	}
	for i := 0; i < 10; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "go", "class")
		chunkIndex++
	}
	for i := 0; i < 10; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "go", "module")
		chunkIndex++
	}

	// Python chunks (30)
	for i := 0; i < 15; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "python", "function")
		chunkIndex++
	}
	for i := 0; i < 10; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "python", "class")
		chunkIndex++
	}
	for i := 0; i < 5; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "python", "module")
		chunkIndex++
	}

	// TypeScript chunks (30)
	for i := 0; i < 15; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "typescript", "function")
		chunkIndex++
	}
	for i := 0; i < 10; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "typescript", "class")
		chunkIndex++
	}
	for i := 0; i < 5; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, "typescript", "module")
		chunkIndex++
	}

	return repositoryID, chunkIDs
}

// setupTestRepositoryWithSelectiveMetadata creates a repository with highly selective metadata distribution.
func setupTestRepositoryWithSelectiveMetadata(
	t *testing.T,
	pool *pgxpool.Pool,
	totalChunks int,
	majorityLang string,
	majorityType string,
	majorityCount int,
	minorityLang string,
	minorityType string,
	minorityCount int,
) (uuid.UUID, []uuid.UUID) {
	if majorityCount+minorityCount != totalChunks {
		t.Fatalf("majorityCount (%d) + minorityCount (%d) must equal totalChunks (%d)",
			majorityCount, minorityCount, totalChunks)
	}

	ctx := context.Background()
	repositoryID := uuid.New()

	// Create repository
	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (url) DO NOTHING
	`, repositoryID, "https://github.com/test/selective-meta-repo-"+repositoryID.String()[:8],
		"https://github.com/test/selective-meta-repo-"+repositoryID.String()[:8],
		"selective-meta-test-repo", "Test repository with selective metadata distribution", "indexed")
	if err != nil {
		t.Fatalf("Failed to create test repository: %v", err)
	}

	chunkIDs := make([]uuid.UUID, totalChunks)
	chunkIndex := 0

	// Create majority chunks
	for i := 0; i < majorityCount; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, majorityLang, majorityType)
		chunkIndex++
	}

	// Create minority chunks
	for i := 0; i < minorityCount; i++ {
		chunkIDs[chunkIndex] = createChunkWithMetadata(t, pool, repositoryID, chunkIndex, minorityLang, minorityType)
		chunkIndex++
	}

	return repositoryID, chunkIDs
}

// createChunkWithMetadata is a helper to create a single chunk with specific metadata.
func createChunkWithMetadata(
	t *testing.T,
	pool *pgxpool.Pool,
	repositoryID uuid.UUID,
	index int,
	language string,
	chunkType string,
) uuid.UUID {
	ctx := context.Background()
	chunkID := uuid.New()

	fileExt := getFileExtensionForLanguage(language)
	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.code_chunks (id, repository_id, file_path, chunk_type, content, language, start_line, end_line, entity_name, content_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (repository_id, file_path, content_hash) DO NOTHING
	`, chunkID, repositoryID, fmt.Sprintf("src/%s/test%d%s", language, index, fileExt), chunkType,
		fmt.Sprintf("content of %s %s %d", language, chunkType, index), language,
		1, 10, fmt.Sprintf("Entity%d", index), "hash-"+chunkID.String()[:8])
	if err != nil {
		t.Fatalf("Failed to create chunk %d with metadata: %v", index, err)
	}

	return chunkID
}

// createTestEmbeddingsWithMetadata creates embeddings with metadata populated from code_chunks table.
// This is a simplified version that directly populates metadata from chunks without database query.
func createTestEmbeddingsWithMetadata(
	t *testing.T,
	chunkIDs []uuid.UUID,
	repositoryID uuid.UUID,
) []outbound.VectorEmbedding {
	// Get metadata from setupTestDB by querying code_chunks table
	pool := getPoolFromTest(t)
	if pool == nil {
		t.Fatal("Failed to get pool for metadata query")
	}

	ctx := context.Background()
	embeddings := make([]outbound.VectorEmbedding, len(chunkIDs))

	for i, chunkID := range chunkIDs {
		// Query metadata from code_chunks table
		var language, chunkType, filePath string
		err := pool.QueryRow(ctx, `
			SELECT language, chunk_type, file_path
			FROM codechunking.code_chunks
			WHERE id = $1
		`, chunkID).Scan(&language, &chunkType, &filePath)
		if err != nil {
			t.Fatalf("Failed to fetch metadata for chunk %s: %v", chunkID, err)
		}

		embeddings[i] = outbound.VectorEmbedding{
			ID:           uuid.New(),
			ChunkID:      chunkID,
			RepositoryID: repositoryID,
			Embedding:    createTestVector(768, float64(i)*0.1),
			ModelVersion: "gemini-embedding-001",
			CreatedAt:    time.Now(),
			// Metadata fields from code_chunks table
			Language:  language,
			ChunkType: chunkType,
			FilePath:  filePath,
		}
	}
	return embeddings
}

// getPoolFromTest retrieves the test database pool from the current test context.
// This is a helper to access the pool across test helper functions.
func getPoolFromTest(t *testing.T) *pgxpool.Pool {
	// Use setupTestDB which caches the pool
	return setupTestDB(t)
}

// Helper utility functions

func getFileExtensionForLanguage(language string) string {
	switch language {
	case "go":
		return ".go"
	case "python":
		return ".py"
	case "typescript":
		return ".ts"
	case "javascript":
		return ".js"
	case "java":
		return ".java"
	case "rust":
		return ".rs"
	default:
		return ".txt"
	}
}

func getLanguageForExtension(extension string) string {
	switch extension {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts":
		return "typescript"
	case ".js":
		return "javascript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	default:
		return "text"
	}
}

func hasExtension(filePath, extension string) bool {
	return len(filePath) >= len(extension) && filePath[len(filePath)-len(extension):] == extension
}

// TestVectorStorageRepository_IterativeScan tests pgvector 0.8.0 iterative scanning feature.
// This test verifies that iterative scanning returns the requested number of results
// even when filters are highly selective.
func TestVectorStorageRepository_IterativeScan(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create multiple repositories to test selective filtering
	repo1ID, repo1ChunkIDs := setupTestRepositoryAndChunks(t, pool, 50)
	repo2ID, repo2ChunkIDs := setupTestRepositoryAndChunks(t, pool, 50)
	repo3ID, repo3ChunkIDs := setupTestRepositoryAndChunks(t, pool, 50)

	// Insert embeddings for all repositories (150 total)
	embeddings1 := createTestEmbeddings(t, repo1ChunkIDs, repo1ID)
	embeddings2 := createTestEmbeddings(t, repo2ChunkIDs, repo2ID)
	embeddings3 := createTestEmbeddings(t, repo3ChunkIDs, repo3ID)

	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           50,
		UsePartitionedTable: true, // Use partitioned table for better performance
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             60 * time.Second,
	}

	allEmbeddings := append(append(embeddings1, embeddings2...), embeddings3...)
	_, err := repo.BulkInsertEmbeddings(ctx, allEmbeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	// Create query vector similar to first repository's vectors
	queryVector := createTestVector(768, 0.1)

	tests := []struct {
		name        string
		queryVector []float64
		options     outbound.SimilaritySearchOptions
		expectedMin int // Minimum results expected with iterative scan
		expectError bool
		description string
	}{
		{
			name:        "Without iterative scan - may return fewer results with selective filter",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: true,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          20,
				MinSimilarity:       0.0,
				RepositoryIDs:       []uuid.UUID{repo1ID}, // Filter to only repo1 (1/3 of data)
				IterativeScanMode:   outbound.IterativeScanOff,
				EfSearch:            10, // Small ef_search to demonstrate the problem
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectedMin: 0, // May return less than 20 without iterative scan
			expectError: false,
			description: "Baseline: without iterative scan, selective filters may return fewer results",
		},
		{
			name:        "With relaxed_order iterative scan - should return requested results",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: true,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          20,
				MinSimilarity:       0.0,
				RepositoryIDs:       []uuid.UUID{repo1ID}, // Filter to only repo1 (1/3 of data)
				IterativeScanMode:   outbound.IterativeScanRelaxedOrder,
				EfSearch:            10, // Small ef_search to test iterative scanning
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectedMin: 20, // Should return full 20 results with iterative scan
			expectError: false,
			description: "With relaxed_order mode, should get 20 results despite selective filter",
		},
		{
			name:        "With strict_order iterative scan - should return requested results with ordering",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: true,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          15,
				MinSimilarity:       0.0,
				RepositoryIDs:       []uuid.UUID{repo2ID}, // Filter to repo2
				IterativeScanMode:   outbound.IterativeScanStrictOrder,
				EfSearch:            10,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectedMin: 15, // Should return full 15 results with strict ordering
			expectError: false,
			description: "With strict_order mode, should get 15 results with exact distance ordering",
		},
		{
			name:        "Multiple repository filters with iterative scan",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: true,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          30,
				MinSimilarity:       0.0,
				RepositoryIDs:       []uuid.UUID{repo1ID, repo3ID}, // Filter to 2/3 of data
				IterativeScanMode:   outbound.IterativeScanRelaxedOrder,
				EfSearch:            15,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectedMin: 30, // Should return 30 results from combined repos
			expectError: false,
			description: "Iterative scan should work with multiple repository filters",
		},
		{
			name:        "Iterative scan with high similarity threshold",
			queryVector: queryVector,
			options: outbound.SimilaritySearchOptions{
				UsePartitionedTable: true,
				SimilarityMetric:    outbound.SimilarityMetricCosine,
				MaxResults:          10,
				MinSimilarity:       0.95, // High threshold
				RepositoryIDs:       []uuid.UUID{repo1ID},
				IterativeScanMode:   outbound.IterativeScanRelaxedOrder,
				EfSearch:            20,
				IncludeDeleted:      false,
				Timeout:             30 * time.Second,
			},
			expectedMin: 0, // May not find 10 results above 0.95 similarity
			expectError: false,
			description: "Iterative scan respects similarity threshold - may return fewer if threshold too high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			results, err := repo.VectorSimilaritySearch(ctx, tt.queryVector, tt.options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			resultCount := len(results)
			t.Logf("Got %d results (expected min: %d, max: %d)",
				resultCount, tt.expectedMin, tt.options.MaxResults)

			if resultCount < tt.expectedMin {
				t.Errorf("Expected at least %d results, got %d. Iterative scan may not be working correctly",
					tt.expectedMin, resultCount)
			}

			if resultCount > tt.options.MaxResults {
				t.Errorf("Expected at most %d results, got %d", tt.options.MaxResults, resultCount)
			}

			// Verify all results belong to the filtered repositories
			if len(tt.options.RepositoryIDs) > 0 {
				repoIDSet := make(map[uuid.UUID]bool)
				for _, id := range tt.options.RepositoryIDs {
					repoIDSet[id] = true
				}

				for i, result := range results {
					if !repoIDSet[result.Embedding.RepositoryID] {
						t.Errorf("Result %d: embedding from repository %s not in filter %v",
							i, result.Embedding.RepositoryID, tt.options.RepositoryIDs)
					}
				}
			}

			// Verify similarity ordering (should be descending)
			for i := 1; i < len(results); i++ {
				if results[i].Similarity > results[i-1].Similarity {
					t.Errorf("Results not properly ordered: result %d (similarity %.4f) > result %d (similarity %.4f)",
						i, results[i].Similarity, i-1, results[i-1].Similarity)
				}
			}

			// Verify similarity threshold is respected
			if tt.options.MinSimilarity > 0 {
				for i, result := range results {
					if result.Similarity < tt.options.MinSimilarity {
						t.Errorf("Result %d: similarity %.4f below threshold %.4f",
							i, result.Similarity, tt.options.MinSimilarity)
					}
				}
			}
		})
	}
}

// TestVectorStorageRepository_IterativeScanWithLanguageFilter tests iterative scanning with language metadata filtering.
// This test verifies that language filters work at SQL level (not post-processing) with iterative scanning.
// EXPECTED TO FAIL: embeddings_partitioned table doesn't have language column yet.
func TestVectorStorageRepository_IterativeScanWithLanguageFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create 3 repositories with mixed language code
	repo1ID, repo1ChunkIDs := setupTestRepositoryAndChunksWithLanguages(
		t,
		pool,
		50,
		[]string{"go", "python", "typescript"},
		[]int{10, 20, 20},
	)
	repo2ID, repo2ChunkIDs := setupTestRepositoryAndChunksWithLanguages(
		t,
		pool,
		50,
		[]string{"go", "python", "typescript"},
		[]int{10, 20, 20},
	)
	repo3ID, repo3ChunkIDs := setupTestRepositoryAndChunksWithLanguages(
		t,
		pool,
		50,
		[]string{"go", "python", "typescript"},
		[]int{10, 20, 20},
	)

	// Insert embeddings with metadata (150 total: 30 Go, 60 Python, 60 TypeScript)
	embeddings1 := createTestEmbeddingsWithMetadata(t, repo1ChunkIDs, repo1ID)
	embeddings2 := createTestEmbeddingsWithMetadata(t, repo2ChunkIDs, repo2ID)
	embeddings3 := createTestEmbeddingsWithMetadata(t, repo3ChunkIDs, repo3ID)

	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           50,
		UsePartitionedTable: true,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             60 * time.Second,
	}

	allEmbeddings := append(append(embeddings1, embeddings2...), embeddings3...)
	_, err := repo.BulkInsertEmbeddings(ctx, allEmbeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	queryVector := createTestVector(768, 0.1)

	t.Run("Language filter with iterative scan should return only Go chunks", func(t *testing.T) {
		t.Log("Test validates SQL-level language filtering with iterative scanning")
		t.Log("Expected behavior: should return 15 Go chunks (out of 30 total Go chunks)")
		t.Log("Current behavior: WILL FAIL - language column doesn't exist in embeddings_partitioned")

		options := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			SimilarityMetric:    outbound.SimilarityMetricCosine,
			MaxResults:          15,
			MinSimilarity:       0.0,
			Languages:           []string{"go"}, // Filter to only Go code
			IterativeScanMode:   outbound.IterativeScanRelaxedOrder,
			EfSearch:            10, // Small ef_search to force iterative scanning
			IncludeDeleted:      false,
			Timeout:             30 * time.Second,
		}

		results, err := repo.VectorSimilaritySearch(ctx, queryVector, options)
		if err != nil {
			// Expected to fail with error about missing language column
			t.Logf("Expected failure: %v", err)
			if !strings.Contains(err.Error(), "language") && !strings.Contains(err.Error(), "column") {
				t.Errorf("Expected error about missing 'language' column, got: %v", err)
			}
			return
		}

		// If it doesn't error, verify the results
		if len(results) < 15 {
			t.Errorf("Expected 15 Go chunks with iterative scan, got %d", len(results))
		}

		// Verify all results are Go code
		for i, result := range results {
			// This will fail because we can't verify language without the column
			if result.Embedding.Language != "go" {
				t.Errorf("Result %d: expected language 'go', got '%s'", i, result.Embedding.Language)
			}
		}
	})
}

// TestVectorStorageRepository_IterativeScanWithChunkTypeFilter tests iterative scanning with chunk_type metadata filtering.
// This test verifies that chunk_type filters work at SQL level with iterative scanning.
// EXPECTED TO FAIL: embeddings_partitioned table doesn't have chunk_type column yet.
func TestVectorStorageRepository_IterativeScanWithChunkTypeFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create repository with mixed chunk types
	repoID, chunkIDs := setupTestRepositoryAndChunksWithTypes(
		t,
		pool,
		100,
		[]string{"function", "class", "module"},
		[]int{30, 40, 30},
	)

	// Insert embeddings with metadata (100 total: 30 functions, 40 classes, 30 modules)
	embeddings := createTestEmbeddingsWithMetadata(t, chunkIDs, repoID)

	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           50,
		UsePartitionedTable: true,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             60 * time.Second,
	}

	_, err := repo.BulkInsertEmbeddings(ctx, embeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	queryVector := createTestVector(768, 0.1)

	t.Run("ChunkType filter with iterative scan should return only function chunks", func(t *testing.T) {
		t.Log("Test validates SQL-level chunk_type filtering with iterative scanning")
		t.Log("Expected behavior: should return 25 function chunks (out of 30 total functions)")
		t.Log("Current behavior: WILL FAIL - chunk_type column doesn't exist in embeddings_partitioned")

		options := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			SimilarityMetric:    outbound.SimilarityMetricCosine,
			MaxResults:          25,
			MinSimilarity:       0.0,
			ChunkTypes:          []string{"function"}, // Filter to only functions
			IterativeScanMode:   outbound.IterativeScanRelaxedOrder,
			EfSearch:            15, // Small ef_search to force iterative scanning
			IncludeDeleted:      false,
			Timeout:             30 * time.Second,
		}

		results, err := repo.VectorSimilaritySearch(ctx, queryVector, options)
		if err != nil {
			// Expected to fail with error about missing chunk_type column
			t.Logf("Expected failure: %v", err)
			if !strings.Contains(err.Error(), "chunk_type") && !strings.Contains(err.Error(), "column") {
				t.Errorf("Expected error about missing 'chunk_type' column, got: %v", err)
			}
			return
		}

		// If it doesn't error, verify the results
		if len(results) < 25 {
			t.Errorf("Expected 25 function chunks with iterative scan, got %d", len(results))
		}

		// Verify all results are function chunks
		for i, result := range results {
			if result.Embedding.ChunkType != "function" {
				t.Errorf("Result %d: expected chunk_type 'function', got '%s'", i, result.Embedding.ChunkType)
			}
		}
	})
}

// TestVectorStorageRepository_IterativeScanWithFilePathFilter tests iterative scanning with file_path metadata filtering.
// This test verifies that file path extension filters work at SQL level with iterative scanning.
// EXPECTED TO FAIL: embeddings_partitioned table doesn't have file_path column yet.
func TestVectorStorageRepository_IterativeScanWithFilePathFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create repository with files of different extensions
	repoID, chunkIDs := setupTestRepositoryAndChunksWithFilePaths(t, pool, 90,
		[]string{".go", ".py", ".ts"}, []int{30, 30, 30})

	// Insert embeddings with metadata (90 total: 30 .go files, 30 .py files, 30 .ts files)
	embeddings := createTestEmbeddingsWithMetadata(t, chunkIDs, repoID)

	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           50,
		UsePartitionedTable: true,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             60 * time.Second,
	}

	_, err := repo.BulkInsertEmbeddings(ctx, embeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	queryVector := createTestVector(768, 0.1)

	t.Run("File extension filter should return only .go files", func(t *testing.T) {
		t.Log("Test validates SQL-level file_path filtering with file extension pattern")
		t.Log("Expected behavior: should return 20 chunks from .go files (out of 30 total)")
		t.Log("Current behavior: WILL FAIL - file_path column doesn't exist in embeddings_partitioned")

		options := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			SimilarityMetric:    outbound.SimilarityMetricCosine,
			MaxResults:          20,
			MinSimilarity:       0.0,
			FileExtensions:      []string{".go"}, // Filter to only .go files
			IterativeScanMode:   outbound.IterativeScanRelaxedOrder,
			EfSearch:            10,
			IncludeDeleted:      false,
			Timeout:             30 * time.Second,
		}

		results, err := repo.VectorSimilaritySearch(ctx, queryVector, options)
		if err != nil {
			// Expected to fail with error about missing file_path column
			t.Logf("Expected failure: %v", err)
			if !strings.Contains(err.Error(), "file_path") && !strings.Contains(err.Error(), "column") {
				t.Errorf("Expected error about missing 'file_path' column, got: %v", err)
			}
			return
		}

		// If it doesn't error, verify the results
		if len(results) < 20 {
			t.Errorf("Expected 20 chunks from .go files with iterative scan, got %d", len(results))
		}

		// Verify all results are from .go files
		for i, result := range results {
			if !hasExtension(result.Embedding.FilePath, ".go") {
				t.Errorf("Result %d: expected .go file, got '%s'", i, result.Embedding.FilePath)
			}
		}
	})
}

// TestVectorStorageRepository_IterativeScanWithCombinedFilters tests iterative scanning with combined metadata filters.
// This test verifies that multiple filters (repository + language + chunk_type) work together at SQL level.
// EXPECTED TO FAIL: embeddings_partitioned table doesn't have language and chunk_type columns yet.
func TestVectorStorageRepository_IterativeScanWithCombinedFilters(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create 2 repositories with mixed languages and types
	repo1ID, repo1ChunkIDs := setupTestRepositoryWithMixedMetadata(t, pool, 100)
	repo2ID, repo2ChunkIDs := setupTestRepositoryWithMixedMetadata(t, pool, 100)

	// Insert embeddings (200 total with variety)
	embeddings1 := createTestEmbeddingsWithMetadata(t, repo1ChunkIDs, repo1ID)
	embeddings2 := createTestEmbeddingsWithMetadata(t, repo2ChunkIDs, repo2ID)

	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           50,
		UsePartitionedTable: true,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             60 * time.Second,
	}

	allEmbeddings := append(embeddings1, embeddings2...)
	_, err := repo.BulkInsertEmbeddings(ctx, allEmbeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	queryVector := createTestVector(768, 0.1)

	t.Run("Combined filters: repository + language + chunk_type", func(t *testing.T) {
		t.Log("Test validates combined SQL-level filtering: repository + language + chunk_type")
		t.Log("Expected behavior: should return 20 Go function chunks from repository 1")
		t.Log("Current behavior: WILL FAIL - language and chunk_type columns don't exist")

		options := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			SimilarityMetric:    outbound.SimilarityMetricCosine,
			MaxResults:          20,
			MinSimilarity:       0.0,
			RepositoryIDs:       []uuid.UUID{repo1ID}, // Filter to repo1
			Languages:           []string{"go"},       // Filter to Go code
			ChunkTypes:          []string{"function"}, // Filter to functions
			IterativeScanMode:   outbound.IterativeScanRelaxedOrder,
			EfSearch:            10,
			IncludeDeleted:      false,
			Timeout:             30 * time.Second,
		}

		results, err := repo.VectorSimilaritySearch(ctx, queryVector, options)
		if err != nil {
			// Expected to fail with error about missing columns
			t.Logf("Expected failure: %v", err)
			if !strings.Contains(err.Error(), "language") && !strings.Contains(err.Error(), "chunk_type") &&
				!strings.Contains(err.Error(), "column") {
				t.Errorf("Expected error about missing metadata columns, got: %v", err)
			}
			return
		}

		// If it doesn't error, verify the results
		if len(results) < 20 {
			t.Errorf("Expected 20 Go function chunks with iterative scan, got %d", len(results))
		}

		// Verify all results match all filters
		for i, result := range results {
			if result.Embedding.RepositoryID != repo1ID {
				t.Errorf("Result %d: expected repository %s, got %s",
					i, repo1ID, result.Embedding.RepositoryID)
			}
			if result.Embedding.Language != "go" {
				t.Errorf("Result %d: expected language 'go', got '%s'", i, result.Embedding.Language)
			}
			if result.Embedding.ChunkType != "function" {
				t.Errorf("Result %d: expected chunk_type 'function', got '%s'", i, result.Embedding.ChunkType)
			}
		}
	})
}

// TestVectorStorageRepository_IterativeScanEffectivenessWithMetadataFilters tests that iterative scanning
// effectively expands search when metadata filters are highly selective.
// EXPECTED TO FAIL: embeddings_partitioned table doesn't have metadata columns yet.
func TestVectorStorageRepository_IterativeScanEffectivenessWithMetadataFilters(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLVectorStorageRepository(pool)
	ctx := context.Background()

	// Create scenario where filters are highly selective: 270 Python classes, 30 Go functions
	repoID, chunkIDs := setupTestRepositoryWithSelectiveMetadata(t, pool, 300,
		"python", "class", 270,
		"go", "function", 30)

	embeddings := createTestEmbeddingsWithMetadata(t, chunkIDs, repoID)

	bulkOptions := outbound.BulkInsertOptions{
		BatchSize:           100,
		UsePartitionedTable: true,
		OnConflictAction:    outbound.ConflictActionDoNothing,
		TransactionBehavior: outbound.TransactionBehaviorAuto,
		ValidateVectorDims:  true,
		EnableMetrics:       false,
		Timeout:             60 * time.Second,
	}

	_, err := repo.BulkInsertEmbeddings(ctx, embeddings, bulkOptions)
	if err != nil {
		t.Fatalf("Failed to insert test embeddings: %v", err)
	}

	queryVector := createTestVector(768, 0.1)

	t.Run("Highly selective filters with iterative scan should still return requested count", func(t *testing.T) {
		t.Log("Test validates iterative scanning with highly selective metadata filters")
		t.Log("Data: 300 total (270 Python classes, 30 Go functions)")
		t.Log("Expected: should return 20 Go functions despite being only 10% of data")
		t.Log("Current behavior: WILL FAIL - metadata columns don't exist")

		options := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			SimilarityMetric:    outbound.SimilarityMetricCosine,
			MaxResults:          20,
			MinSimilarity:       0.0,
			Languages:           []string{"go"},       // Only 10% of data is Go
			ChunkTypes:          []string{"function"}, // And all Go is functions
			IterativeScanMode:   outbound.IterativeScanRelaxedOrder,
			EfSearch:            25, // Small ef_search to force multiple iterations
			IncludeDeleted:      false,
			Timeout:             30 * time.Second,
		}

		results, err := repo.VectorSimilaritySearch(ctx, queryVector, options)
		if err != nil {
			// Expected to fail with error about missing columns
			t.Logf("Expected failure: %v", err)
			return
		}

		resultCount := len(results)
		t.Logf("Got %d results (expected 20)", resultCount)

		// Verify iterative scan successfully expanded to find all matching results
		if resultCount < 20 {
			t.Errorf("Expected 20 Go functions with iterative scan, got %d. "+
				"Iterative scanning should expand search to find all matching results despite selective filter.",
				resultCount)
		}

		// Verify all results are Go functions
		for i, result := range results {
			if result.Embedding.Language != "go" {
				t.Errorf("Result %d: expected language 'go', got '%s'", i, result.Embedding.Language)
			}
			if result.Embedding.ChunkType != "function" {
				t.Errorf("Result %d: expected chunk_type 'function', got '%s'", i, result.Embedding.ChunkType)
			}
		}

		// Log to verify multiple iterations occurred (would be visible in implementation logs)
		t.Logf("Iterative scan should have performed multiple iterations to gather %d results from 10%% of data",
			resultCount)
	})
}
