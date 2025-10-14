//go:build integration
// +build integration

package repository

import (
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
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
		"vector-test-repo", "Test repository for vector operations", "completed")
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
