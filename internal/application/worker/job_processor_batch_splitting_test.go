package worker

import (
	"codechunking/internal/port/outbound"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitChunksIntoBatches_EmptyInput verifies that splitting an empty chunk array
// returns an empty slice of batches.
func TestSplitChunksIntoBatches_EmptyInput(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	chunks := []outbound.CodeChunk{}
	maxBatchSize := 500

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	assert.NotNil(t, batches, "batches should not be nil for empty input")
	assert.Empty(t, batches, "batches should be empty for empty input")
	assert.Empty(t, batches, "should return zero batches for empty input")
}

// TestSplitChunksIntoBatches_SingleBatch verifies that when the number of chunks
// is less than maxBatchSize, all chunks are returned in a single batch.
func TestSplitChunksIntoBatches_SingleBatch(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	chunkCount := 100
	maxBatchSize := 500
	chunks := createTestChunksForBatchSplitting(chunkCount)

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	require.NotNil(t, batches, "batches should not be nil")
	assert.Len(t, batches, 1, "should return exactly 1 batch")
	assert.Len(t, batches[0], chunkCount, "batch should contain all chunks")

	// Verify total chunk count is preserved
	totalChunks := 0
	for _, batch := range batches {
		totalChunks += len(batch)
	}
	assert.Equal(t, chunkCount, totalChunks, "total chunks should equal input chunks")
}

// TestSplitChunksIntoBatches_ExactMultiple verifies that when the chunk count
// is an exact multiple of maxBatchSize, the chunks are evenly distributed.
func TestSplitChunksIntoBatches_ExactMultiple(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	chunkCount := 1000
	maxBatchSize := 500
	chunks := createTestChunksForBatchSplitting(chunkCount)

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	require.NotNil(t, batches, "batches should not be nil")
	assert.Len(t, batches, 2, "should return exactly 2 batches")

	// Verify each batch has exactly maxBatchSize chunks
	for i, batch := range batches {
		assert.Len(t, batch, maxBatchSize,
			"batch %d should contain exactly %d chunks", i, maxBatchSize)
	}

	// Verify total chunk count is preserved
	totalChunks := 0
	for _, batch := range batches {
		totalChunks += len(batch)
	}
	assert.Equal(t, chunkCount, totalChunks, "total chunks should equal input chunks")
}

// TestSplitChunksIntoBatches_WithRemainder verifies that when the chunk count
// is not evenly divisible by maxBatchSize, the last batch contains the remainder.
func TestSplitChunksIntoBatches_WithRemainder(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	chunkCount := 1250
	maxBatchSize := 500
	chunks := createTestChunksForBatchSplitting(chunkCount)

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	require.NotNil(t, batches, "batches should not be nil")
	assert.Len(t, batches, 3, "should return exactly 3 batches")

	// Verify first two batches are full
	assert.Len(t, batches[0], maxBatchSize, "first batch should be full")
	assert.Len(t, batches[1], maxBatchSize, "second batch should be full")

	// Verify last batch contains remainder
	assert.Len(t, batches[2], 250, "third batch should contain remainder (250 chunks)")

	// Verify total chunk count is preserved
	totalChunks := 0
	for _, batch := range batches {
		totalChunks += len(batch)
	}
	assert.Equal(t, chunkCount, totalChunks, "total chunks should equal input chunks")

	// Verify no batch exceeds maxBatchSize
	for i, batch := range batches {
		assert.LessOrEqual(t, len(batch), maxBatchSize,
			"batch %d size should not exceed maxBatchSize", i)
	}
}

// TestSplitChunksIntoBatches_LargeDataset verifies correct batching for a large
// dataset similar to production scenarios (8269 chunks as mentioned in requirements).
func TestSplitChunksIntoBatches_LargeDataset(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	chunkCount := 8269
	maxBatchSize := 500
	chunks := createTestChunksForBatchSplitting(chunkCount)

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	require.NotNil(t, batches, "batches should not be nil")

	expectedBatches := 17 // 16 full batches of 500 + 1 batch of 269
	assert.Len(t, batches, expectedBatches, "should return exactly 17 batches")

	// Verify first 16 batches are full
	for i := range 16 {
		assert.Len(t, batches[i], maxBatchSize,
			"batch %d should contain exactly %d chunks", i, maxBatchSize)
	}

	// Verify last batch contains remainder
	assert.Len(t, batches[16], 269, "last batch should contain 269 chunks")

	// Verify total chunk count is preserved
	totalChunks := 0
	for _, batch := range batches {
		totalChunks += len(batch)
	}
	assert.Equal(t, chunkCount, totalChunks, "total chunks should equal input chunks")

	// Verify no batch exceeds maxBatchSize
	for i, batch := range batches {
		assert.LessOrEqual(t, len(batch), maxBatchSize,
			"batch %d size should not exceed maxBatchSize", i)
	}
}

// TestSplitChunksIntoBatches_SingleChunk verifies correct handling of a single chunk.
func TestSplitChunksIntoBatches_SingleChunk(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	chunkCount := 1
	maxBatchSize := 500
	chunks := createTestChunksForBatchSplitting(chunkCount)

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	require.NotNil(t, batches, "batches should not be nil")
	assert.Len(t, batches, 1, "should return exactly 1 batch")
	assert.Len(t, batches[0], 1, "batch should contain exactly 1 chunk")

	// Verify the chunk is the same as input
	assert.Equal(t, chunks[0].ID, batches[0][0].ID, "chunk ID should be preserved")
	assert.Equal(t, chunks[0].Content, batches[0][0].Content, "chunk content should be preserved")
}

// TestSplitChunksIntoBatches_BatchSizeOne verifies correct handling when maxBatchSize is 1.
func TestSplitChunksIntoBatches_BatchSizeOne(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	chunkCount := 10
	maxBatchSize := 1
	chunks := createTestChunksForBatchSplitting(chunkCount)

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	require.NotNil(t, batches, "batches should not be nil")
	assert.Len(t, batches, chunkCount, "should return 10 batches")

	// Verify each batch has exactly 1 chunk
	for i, batch := range batches {
		assert.Len(t, batch, 1, "batch %d should contain exactly 1 chunk", i)
	}

	// Verify total chunk count is preserved
	totalChunks := 0
	for _, batch := range batches {
		totalChunks += len(batch)
	}
	assert.Equal(t, chunkCount, totalChunks, "total chunks should equal input chunks")
}

// TestSplitChunksIntoBatches_PreservesOrder verifies that chunks maintain their
// original order across batches.
func TestSplitChunksIntoBatches_PreservesOrder(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	chunkCount := 1250
	maxBatchSize := 500
	chunks := createTestChunksWithIdentifiableContent(chunkCount)

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	require.NotNil(t, batches, "batches should not be nil")

	// Flatten batches back into a single slice
	var flattenedChunks []outbound.CodeChunk
	for _, batch := range batches {
		flattenedChunks = append(flattenedChunks, batch...)
	}

	// Verify order is preserved
	for i, chunk := range flattenedChunks {
		expectedContent := fmt.Sprintf("chunk-content-%d", i)
		assert.Equal(t, expectedContent, chunk.Content,
			"chunk at position %d should have correct content", i)
		assert.Equal(t, chunks[i].ID, chunk.ID,
			"chunk at position %d should have correct ID", i)
	}

	// Verify no duplicates
	seenIDs := make(map[string]bool)
	for _, chunk := range flattenedChunks {
		assert.False(t, seenIDs[chunk.ID], "chunk ID %s should not be duplicated", chunk.ID)
		seenIDs[chunk.ID] = true
	}
}

// TestSplitChunksIntoBatches_PreservesChunkData verifies that all chunk fields
// are preserved during batching (no data loss).
func TestSplitChunksIntoBatches_PreservesChunkData(t *testing.T) {
	// Arrange
	processor := createTestJobProcessor(t)
	repositoryID := uuid.New()

	// Create chunks with all fields populated
	chunks := []outbound.CodeChunk{
		{
			ID:            uuid.New().String(),
			RepositoryID:  repositoryID,
			FilePath:      "/src/main.go",
			StartLine:     10,
			EndLine:       20,
			Content:       "func main() { ... }",
			Language:      "go",
			Size:          100,
			Hash:          "abc123",
			CreatedAt:     time.Now(),
			Type:          "function",
			EntityName:    "main",
			ParentEntity:  "",
			QualifiedName: "main.main",
			Signature:     "func main()",
			Visibility:    "public",
		},
		{
			ID:            uuid.New().String(),
			RepositoryID:  repositoryID,
			FilePath:      "/src/utils.go",
			StartLine:     5,
			EndLine:       15,
			Content:       "func helper() { ... }",
			Language:      "go",
			Size:          80,
			Hash:          "def456",
			CreatedAt:     time.Now(),
			Type:          "function",
			EntityName:    "helper",
			ParentEntity:  "",
			QualifiedName: "main.helper",
			Signature:     "func helper()",
			Visibility:    "private",
		},
	}
	maxBatchSize := 500

	// Act
	batches := processor.splitChunksIntoBatches(chunks, maxBatchSize)

	// Assert
	require.NotNil(t, batches, "batches should not be nil")
	require.Len(t, batches, 1, "should return 1 batch")
	require.Len(t, batches[0], 2, "batch should contain 2 chunks")

	// Verify first chunk fields
	chunk0 := batches[0][0]
	assert.Equal(t, chunks[0].ID, chunk0.ID, "ID should be preserved")
	assert.Equal(t, chunks[0].RepositoryID, chunk0.RepositoryID, "RepositoryID should be preserved")
	assert.Equal(t, chunks[0].FilePath, chunk0.FilePath, "FilePath should be preserved")
	assert.Equal(t, chunks[0].StartLine, chunk0.StartLine, "StartLine should be preserved")
	assert.Equal(t, chunks[0].EndLine, chunk0.EndLine, "EndLine should be preserved")
	assert.Equal(t, chunks[0].Content, chunk0.Content, "Content should be preserved")
	assert.Equal(t, chunks[0].Language, chunk0.Language, "Language should be preserved")
	assert.Equal(t, chunks[0].Size, chunk0.Size, "Size should be preserved")
	assert.Equal(t, chunks[0].Hash, chunk0.Hash, "Hash should be preserved")
	assert.Equal(t, chunks[0].Type, chunk0.Type, "Type should be preserved")
	assert.Equal(t, chunks[0].EntityName, chunk0.EntityName, "EntityName should be preserved")
	assert.Equal(t, chunks[0].ParentEntity, chunk0.ParentEntity, "ParentEntity should be preserved")
	assert.Equal(t, chunks[0].QualifiedName, chunk0.QualifiedName, "QualifiedName should be preserved")
	assert.Equal(t, chunks[0].Signature, chunk0.Signature, "Signature should be preserved")
	assert.Equal(t, chunks[0].Visibility, chunk0.Visibility, "Visibility should be preserved")

	// Verify second chunk fields
	chunk1 := batches[0][1]
	assert.Equal(t, chunks[1].ID, chunk1.ID, "ID should be preserved")
	assert.Equal(t, chunks[1].RepositoryID, chunk1.RepositoryID, "RepositoryID should be preserved")
	assert.Equal(t, chunks[1].FilePath, chunk1.FilePath, "FilePath should be preserved")
	assert.Equal(t, chunks[1].StartLine, chunk1.StartLine, "StartLine should be preserved")
	assert.Equal(t, chunks[1].EndLine, chunk1.EndLine, "EndLine should be preserved")
	assert.Equal(t, chunks[1].Content, chunk1.Content, "Content should be preserved")
	assert.Equal(t, chunks[1].Language, chunk1.Language, "Language should be preserved")
	assert.Equal(t, chunks[1].Size, chunk1.Size, "Size should be preserved")
	assert.Equal(t, chunks[1].Hash, chunk1.Hash, "Hash should be preserved")
	assert.Equal(t, chunks[1].Type, chunk1.Type, "Type should be preserved")
	assert.Equal(t, chunks[1].EntityName, chunk1.EntityName, "EntityName should be preserved")
	assert.Equal(t, chunks[1].ParentEntity, chunk1.ParentEntity, "ParentEntity should be preserved")
	assert.Equal(t, chunks[1].QualifiedName, chunk1.QualifiedName, "QualifiedName should be preserved")
	assert.Equal(t, chunks[1].Signature, chunk1.Signature, "Signature should be preserved")
	assert.Equal(t, chunks[1].Visibility, chunk1.Visibility, "Visibility should be preserved")
}

// TestSplitChunksIntoBatches_TableDriven uses table-driven tests to verify
// various batch size scenarios.
func TestSplitChunksIntoBatches_TableDriven(t *testing.T) {
	processor := createTestJobProcessor(t)

	tests := []struct {
		name              string
		chunkCount        int
		maxBatchSize      int
		expectedBatches   int
		expectedLastBatch int
	}{
		{
			name:              "exact division 1000/500",
			chunkCount:        1000,
			maxBatchSize:      500,
			expectedBatches:   2,
			expectedLastBatch: 500,
		},
		{
			name:              "with remainder 1001/500",
			chunkCount:        1001,
			maxBatchSize:      500,
			expectedBatches:   3,
			expectedLastBatch: 1,
		},
		{
			name:              "large batch 8269/500",
			chunkCount:        8269,
			maxBatchSize:      500,
			expectedBatches:   17,
			expectedLastBatch: 269,
		},
		{
			name:              "small batch 50/100",
			chunkCount:        50,
			maxBatchSize:      100,
			expectedBatches:   1,
			expectedLastBatch: 50,
		},
		{
			name:              "batch size 1",
			chunkCount:        5,
			maxBatchSize:      1,
			expectedBatches:   5,
			expectedLastBatch: 1,
		},
		{
			name:              "batch larger than chunks",
			chunkCount:        200,
			maxBatchSize:      1000,
			expectedBatches:   1,
			expectedLastBatch: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			chunks := createTestChunksForBatchSplitting(tt.chunkCount)

			// Act
			batches := processor.splitChunksIntoBatches(chunks, tt.maxBatchSize)

			// Assert
			require.NotNil(t, batches, "batches should not be nil")
			assert.Len(t, batches, tt.expectedBatches, "batch count mismatch")

			// Verify last batch size
			if len(batches) > 0 {
				lastBatch := batches[len(batches)-1]
				assert.Len(t, lastBatch, tt.expectedLastBatch, "last batch size mismatch")
			}

			// Verify total chunk count
			totalChunks := 0
			for _, batch := range batches {
				totalChunks += len(batch)
			}
			assert.Equal(t, tt.chunkCount, totalChunks, "total chunks should equal input")

			// Verify no batch exceeds max size
			for i, batch := range batches {
				assert.LessOrEqual(t, len(batch), tt.maxBatchSize,
					"batch %d should not exceed maxBatchSize", i)
			}
		})
	}
}

// Helper functions

// createTestJobProcessor creates a minimal job processor for testing the splitting logic.
func createTestJobProcessor(t *testing.T) *DefaultJobProcessor {
	t.Helper()

	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test",
		MaxConcurrentJobs: 1,
		JobTimeout:        30 * time.Second,
	}

	// Create processor with nil dependencies (we only need the struct for splitChunksIntoBatches)
	processor := &DefaultJobProcessor{
		config:     config,
		activeJobs: make(map[string]*JobExecution),
	}

	return processor
}

// createTestChunksForBatchSplitting creates a slice of test chunks with the specified count.
// This is specific to batch splitting tests and differs from other createTestChunks helpers.
func createTestChunksForBatchSplitting(count int) []outbound.CodeChunk {
	chunks := make([]outbound.CodeChunk, count)
	baseTime := time.Now()

	for i := range count {
		chunks[i] = outbound.CodeChunk{
			ID:        uuid.New().String(),
			FilePath:  fmt.Sprintf("/src/file_%d.go", i),
			StartLine: i * 10,
			EndLine:   i*10 + 10,
			Content:   fmt.Sprintf("test content %d", i),
			Language:  "go",
			Size:      100,
			Hash:      fmt.Sprintf("hash-%d", i),
			CreatedAt: baseTime.Add(time.Duration(i) * time.Second),
		}
	}

	return chunks
}

// createTestChunksWithIdentifiableContent creates chunks with content that
// can be used to verify ordering is preserved.
func createTestChunksWithIdentifiableContent(count int) []outbound.CodeChunk {
	chunks := make([]outbound.CodeChunk, count)
	baseTime := time.Now()

	for i := range count {
		chunks[i] = outbound.CodeChunk{
			ID:        fmt.Sprintf("chunk-id-%d", i),
			FilePath:  fmt.Sprintf("/src/file_%d.go", i),
			StartLine: i * 10,
			EndLine:   i*10 + 10,
			Content:   fmt.Sprintf("chunk-content-%d", i), // Identifiable content for order verification
			Language:  "go",
			Size:      100,
			Hash:      fmt.Sprintf("hash-%d", i),
			CreatedAt: baseTime.Add(time.Duration(i) * time.Second),
		}
	}

	return chunks
}
