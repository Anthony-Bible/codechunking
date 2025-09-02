package chunking_test

import (
	"codechunking/internal/adapter/outbound/chunking"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLargeFunctionSizeBasedChunking tests Phase 4.3 Step 4: Add size-based chunking for very large functions.
// These tests validate the implemented functionality for intelligently splitting very large functions while preserving semantic boundaries and context.
func TestLargeFunctionSizeBasedChunking(t *testing.T) {
	t.Run("should create large function chunker", func(t *testing.T) {
		// Test creating chunker with default options
		largeFunctionChunker := chunking.NewLargeFunctionChunkerWithDefaults()
		require.NotNil(t, largeFunctionChunker, "Large function chunker should be created")

		// Test creating chunker with custom options
		options := chunking.SizeChunkingOptions{
			OptimalThreshold: 5 * 1024,  // 5KB
			MaxThreshold:     25 * 1024, // 25KB
			MinChunkSize:     512,       // 512 bytes
		}
		mockFactory := &chunking.MockTreeSitterParserFactory{}
		customChunker := chunking.NewLargeFunctionChunker(mockFactory, options)
		require.NotNil(t, customChunker, "Custom large function chunker should be created")
	})
}

// TestBasicSizeDetection tests basic size detection for large functions.
func TestBasicSizeDetection(t *testing.T) {
	t.Run("should detect function size correctly", func(t *testing.T) {
		ctx := context.Background()
		largeFunctionChunker := chunking.NewLargeFunctionChunkerWithDefaults()

		// Create a small function content
		smallFunction := `func small() { fmt.Println("small") }`
		smallChunk := createBasicSemanticChunk(t, smallFunction, "smallFunc")

		config := outbound.ChunkingConfiguration{
			MaxChunkSize: 10 * 1024, // 10KB
		}

		shouldSplit, recommendation, err := largeFunctionChunker.ShouldSplitFunction(ctx, smallChunk, config)
		require.NoError(t, err, "Size detection should not fail")
		assert.False(t, shouldSplit, "Small function should not be split")
		assert.Equal(t, 1, recommendation.EstimatedChunks, "Small function should be single chunk")

		// Create a larger function content (simulate 15KB)
		largeFunction := createLargeFunctionContent(15 * 1024)
		largeChunk := createBasicSemanticChunk(t, largeFunction, "largeFunc")

		shouldSplit, recommendation, err = largeFunctionChunker.ShouldSplitFunction(ctx, largeChunk, config)
		require.NoError(t, err, "Size detection should not fail")
		assert.True(t, shouldSplit, "Large function should be split")
		assert.Greater(t, recommendation.EstimatedChunks, 1, "Large function should be split into multiple chunks")
	})
}

// TestLargeFunctionSplitting tests the actual splitting of large functions.
func TestLargeFunctionSplitting(t *testing.T) {
	t.Run("should split large function into multiple chunks", func(t *testing.T) {
		ctx := context.Background()
		largeFunctionChunker := chunking.NewLargeFunctionChunkerWithDefaults()

		// Create a large function (20KB)
		largeFunction := createLargeFunctionContent(20 * 1024)
		largeChunk := createBasicSemanticChunk(t, largeFunction, "veryLargeFunc")

		config := outbound.ChunkingConfiguration{
			MaxChunkSize:     10 * 1024, // 10KB - should force splitting
			QualityThreshold: 0.7,       // Reasonable quality threshold
		}

		// Test the actual splitting
		splitChunks, err := largeFunctionChunker.SplitLargeFunction(ctx, largeChunk, config)
		require.NoError(t, err, "Function splitting should not fail")
		assert.Greater(t, len(splitChunks), 1, "Large function should be split into multiple chunks")

		// Verify each chunk respects size limits
		for i, chunk := range splitChunks {
			t.Logf("Chunk %d: %d bytes", i, len(chunk.Content))
			assert.LessOrEqual(t, len(chunk.Content), config.MaxChunkSize, "Split chunk should respect size limits")
			assert.NotEmpty(t, chunk.Content, "Split chunk should have content")
		}

		// Verify total content is preserved (approximately - there might be some context duplication)
		totalSplitSize := 0
		for _, chunk := range splitChunks {
			totalSplitSize += len(chunk.Content)
		}
		originalSize := len(largeChunk.Content)
		t.Logf("Original size: %d bytes, Split total: %d bytes", originalSize, totalSplitSize)

		// Allow for reasonable size difference due to context preservation
		sizeDifference := float64(totalSplitSize-originalSize) / float64(originalSize)
		assert.LessOrEqual(
			t,
			sizeDifference,
			0.5,
			"Split chunks should not be more than 50% larger than original due to context duplication",
		)
	})
}

// Helper functions.
func mustCreateLanguageForSizeTest(t *testing.T, langName string) valueobject.Language {
	t.Helper()
	lang, err := valueobject.NewLanguage(langName)
	require.NoError(t, err, "Should create language successfully")
	return lang
}

func createBasicSemanticChunk(t *testing.T, content, name string) outbound.SemanticCodeChunk {
	t.Helper()
	language := mustCreateLanguageForSizeTest(t, valueobject.LanguageGo)

	return outbound.SemanticCodeChunk{
		Name:          name,
		Type:          outbound.ConstructFunction,
		Language:      language,
		Content:       content,
		StartByte:     0,
		EndByte:       uint32(len(content)),
		StartPosition: valueobject.Position{Row: 1, Column: 1},
		EndPosition:   valueobject.Position{Row: uint32(len(content)/50 + 1), Column: 1}, // Rough estimate
		Documentation: "Test function",
	}
}

func createLargeFunctionContent(targetSize int) string {
	baseFunction := `func ProcessLargeData(data []string) error {
	// Initialize processing variables
	var result []string
	var errors []error
	count := 0

	// Main processing loop
	for i, item := range data {
		if item != "" {
			processed := strings.ToUpper(item)
			result = append(result, processed)
			count++
		} else {
			errors = append(errors, fmt.Errorf("empty item at index %d", i))
		}
	}

	// Validation and cleanup
	if len(errors) > 0 {
		return fmt.Errorf("processing errors: %v", errors)
	}

	return nil
}`

	// If we need more content, pad it
	if len(baseFunction) >= targetSize {
		return baseFunction
	}

	// Simple padding with repeated processing blocks
	content := baseFunction
	padding := targetSize - len(baseFunction)
	iterations := (padding / 200) + 1

	for range iterations {
		additionalCode := `
	// Additional processing step
	if count > 10 {
		tempResult := make([]string, len(result))
		copy(tempResult, result)
		result = tempResult
	}`
		content += additionalCode
		if len(content) >= targetSize {
			break
		}
	}

	return content + "\n}"
}
