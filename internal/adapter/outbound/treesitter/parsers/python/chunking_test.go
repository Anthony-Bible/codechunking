package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// RED PHASE: Chunking Utility Tests
// =============================================================================
// These tests define the expected behavior for source code chunking utilities
// that will prevent tree-sitter buffer overflow crashes on extremely large files.
//
// PROBLEM: Tree-sitter crashes with 'Assertion length <= 1024 failed' when
// parsing files with 50k+ functions due to external scanner buffer overflow.
//
// SOLUTION: Chunk large source files by top-level definitions before parsing,
// then merge the resulting parse trees.
// =============================================================================

// TestChunkSourceByTopLevelDefinitions tests splitting source code into chunks.
// RED PHASE: This test will fail because ChunkSourceByTopLevelDefinitions doesn't exist yet.
//
// EXPECTED BEHAVIOR:
// - Split large Python files into smaller chunks by top-level definitions (functions, classes)
// - Each chunk should contain complete definitions (no mid-function splits)
// - No definitions should be lost or duplicated
// - Chunk boundaries should be on definition boundaries.
func TestChunkSourceByTopLevelDefinitions(t *testing.T) {
	t.Run("split 1000 functions into 5 chunks", func(t *testing.T) {
		// Generate source with 1000 functions
		var sb strings.Builder
		sb.WriteString("# Large Python file with 1000 functions\n\n")

		for i := range 1000 {
			// Generate unique function names: function_0000 through function_0999
			funcNum := paddedNumber(i+1, 4)
			sb.WriteString("def function_")
			sb.WriteString(funcNum)
			sb.WriteString("():\n")
			sb.WriteString("    \"\"\"Function ")
			sb.WriteString(funcNum)
			sb.WriteString(" docstring.\"\"\"\n")
			sb.WriteString("    pass\n\n")
		}

		source := []byte(sb.String())
		maxChunkSize := len(source) / 5 // Split into ~5 chunks

		// Call function that doesn't exist yet (RED PHASE)
		ctx := context.Background()
		chunks, err := ChunkSourceByTopLevelDefinitions(ctx, source, maxChunkSize)
		require.NoError(t, err, "Chunking should not fail")

		// Should get approximately 5 chunks
		assert.GreaterOrEqual(t, len(chunks), 3, "Should have at least 3 chunks")
		assert.LessOrEqual(t, len(chunks), 7, "Should have at most 7 chunks")

		// Verify each chunk has complete function definitions
		for i, chunk := range chunks {
			chunkStr := string(chunk)

			// Count 'def ' occurrences (function definitions)
			defCount := strings.Count(chunkStr, "def function_")

			// Count 'pass' statements (function bodies)
			passCount := strings.Count(chunkStr, "    pass")

			assert.Equal(t, defCount, passCount,
				"Chunk %d should have complete functions (def count == pass count)", i)

			// Each function should have complete docstring
			docstringCount := strings.Count(chunkStr, "\"\"\"")
			assert.Equal(t, defCount*2, docstringCount,
				"Chunk %d should have complete docstrings", i)
		}

		// Verify no functions are lost
		totalDefs := 0
		for _, chunk := range chunks {
			totalDefs += strings.Count(string(chunk), "def function_")
		}
		assert.Equal(t, 1000, totalDefs, "All 1000 functions should be present across chunks")

		// Verify no duplication by checking unique function names
		seen := make(map[string]bool)
		for _, chunk := range chunks {
			lines := strings.Split(string(chunk), "\n")
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "def function_") {
					funcName := strings.TrimSpace(strings.Split(line, "(")[0])
					assert.False(t, seen[funcName], "Function %s should not be duplicated", funcName)
					seen[funcName] = true
				}
			}
		}
	})

	t.Run("small file returns single chunk", func(t *testing.T) {
		source := []byte(`
def small_function():
    """A small function."""
    pass

def another_small():
    return 42
`)
		maxChunkSize := 10000 // Larger than source

		ctx := context.Background()
		chunks, err := ChunkSourceByTopLevelDefinitions(ctx, source, maxChunkSize)
		require.NoError(t, err)

		// Should return single chunk for small files
		require.Len(t, chunks, 1, "Small file should result in single chunk")
		assert.Equal(t, source, chunks[0], "Single chunk should match original source")
	})

	t.Run("chunk boundary respects class definitions", func(t *testing.T) {
		// Source with classes containing methods
		source := []byte(`
class FirstClass:
    def method_one(self):
        pass

    def method_two(self):
        pass

class SecondClass:
    def method_three(self):
        pass

    def method_four(self):
        pass

class ThirdClass:
    def method_five(self):
        pass
`)
		maxChunkSize := len(source) / 2 // Force at least 2 chunks

		ctx := context.Background()
		chunks, err := ChunkSourceByTopLevelDefinitions(ctx, source, maxChunkSize)
		require.NoError(t, err)

		// Verify no class is split across chunks
		for i, chunk := range chunks {
			chunkStr := string(chunk)

			// For each class in this chunk, verify all its methods are present
			if strings.Contains(chunkStr, "class FirstClass") {
				assert.Contains(t, chunkStr, "def method_one",
					"Chunk %d with FirstClass should have method_one", i)
				assert.Contains(t, chunkStr, "def method_two",
					"Chunk %d with FirstClass should have method_two", i)
			}

			if strings.Contains(chunkStr, "class SecondClass") {
				assert.Contains(t, chunkStr, "def method_three",
					"Chunk %d with SecondClass should have method_three", i)
				assert.Contains(t, chunkStr, "def method_four",
					"Chunk %d with SecondClass should have method_four", i)
			}

			if strings.Contains(chunkStr, "class ThirdClass") {
				assert.Contains(t, chunkStr, "def method_five",
					"Chunk %d with ThirdClass should have method_five", i)
			}
		}
	})

	t.Run("handles empty source", func(t *testing.T) {
		source := []byte("")
		maxChunkSize := 1000

		ctx := context.Background()
		chunks, err := ChunkSourceByTopLevelDefinitions(ctx, source, maxChunkSize)
		require.NoError(t, err)

		// Empty source should return no chunks or single empty chunk
		assert.LessOrEqual(t, len(chunks), 1, "Empty source should return 0 or 1 chunk")
	})

	t.Run("preserves module-level comments and imports", func(t *testing.T) {
		source := []byte(`#!/usr/bin/env python3
"""Module docstring."""

import os
import sys
from typing import List

# Module constant
MAX_SIZE = 100

def function_one():
    pass

def function_two():
    pass
`)
		maxChunkSize := len(source) / 2

		ctx := context.Background()
		chunks, err := ChunkSourceByTopLevelDefinitions(ctx, source, maxChunkSize)
		require.NoError(t, err)

		// First chunk should have module-level content
		firstChunk := string(chunks[0])
		assert.Contains(t, firstChunk, "#!/usr/bin/env python3",
			"First chunk should preserve shebang")
		assert.Contains(t, firstChunk, "Module docstring",
			"First chunk should preserve module docstring")
		assert.Contains(t, firstChunk, "import os",
			"First chunk should preserve imports")
	})
}

// TestMergeParseTreeChunks tests merging multiple parse trees into one.
// RED PHASE: This test will fail because MergeParseTreeChunks doesn't exist yet.
//
// EXPECTED BEHAVIOR:
// - Merge multiple ParseTree objects into a single ParseTree
// - All nodes from all chunks should be present in merged tree
// - No duplicate nodes in the merged result
// - Merged tree should be well-formed.
func TestMergeParseTreeChunks(t *testing.T) {
	t.Run("merge 3 parse trees into single tree", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		// Create 3 chunks with different functions
		chunk1 := []byte(`
def function_a():
    """Function A."""
    pass
`)
		chunk2 := []byte(`
def function_b():
    """Function B."""
    pass
`)
		chunk3 := []byte(`
def function_c():
    """Function C."""
    pass
`)

		// Parse each chunk separately
		parseTree1 := createMockParseTreeFromSource(t, language, string(chunk1))
		parseTree2 := createMockParseTreeFromSource(t, language, string(chunk2))
		parseTree3 := createMockParseTreeFromSource(t, language, string(chunk3))

		chunks := []*valueobject.ParseTree{parseTree1, parseTree2, parseTree3}

		// Call function that doesn't exist yet (RED PHASE)
		mergedTree, err := MergeParseTreeChunks(context.Background(), chunks)
		require.NoError(t, err, "Merging should not fail")
		require.NotNil(t, mergedTree, "Merged tree should not be nil")

		// Verify merged tree is well-formed
		isWellFormed, err := mergedTree.IsWellFormed()
		require.NoError(t, err)
		assert.True(t, isWellFormed, "Merged tree should be well-formed")

		// Verify all function definitions are present
		mergedSource := string(mergedTree.Source())
		assert.Contains(t, mergedSource, "function_a", "Merged tree should contain function_a")
		assert.Contains(t, mergedSource, "function_b", "Merged tree should contain function_b")
		assert.Contains(t, mergedSource, "function_c", "Merged tree should contain function_c")

		// Verify node count is reasonable (sum of individual trees minus duplicate module nodes)
		expectedMinNodes := parseTree1.GetTotalNodeCount() +
			parseTree2.GetTotalNodeCount() +
			parseTree3.GetTotalNodeCount() - 2 // Subtract module root duplicates
		actualNodes := mergedTree.GetTotalNodeCount()
		assert.GreaterOrEqual(t, actualNodes, expectedMinNodes/2,
			"Merged tree should have reasonable node count")
	})

	t.Run("merge single tree returns same tree", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		source := []byte(`
def single_function():
    pass
`)
		parseTree := createMockParseTreeFromSource(t, language, string(source))
		chunks := []*valueobject.ParseTree{parseTree}

		mergedTree, err := MergeParseTreeChunks(context.Background(), chunks)
		require.NoError(t, err)
		require.NotNil(t, mergedTree)

		// Should return equivalent tree
		assert.Equal(t, parseTree.GetTotalNodeCount(), mergedTree.GetTotalNodeCount(),
			"Single chunk merge should preserve node count")
	})

	t.Run("merge empty chunk list returns error", func(t *testing.T) {
		chunks := []*valueobject.ParseTree{}

		_, err := MergeParseTreeChunks(context.Background(), chunks)
		require.Error(t, err, "Merging empty chunk list should return error")
		assert.Contains(t, err.Error(), "empty", "Error should mention empty chunks")
	})

	t.Run("merge preserves module-level constructs", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		chunk1 := []byte(`
"""Module docstring."""
import os

def function_one():
    pass
`)
		chunk2 := []byte(`
def function_two():
    pass
`)

		parseTree1 := createMockParseTreeFromSource(t, language, string(chunk1))
		parseTree2 := createMockParseTreeFromSource(t, language, string(chunk2))

		chunks := []*valueobject.ParseTree{parseTree1, parseTree2}

		mergedTree, err := MergeParseTreeChunks(context.Background(), chunks)
		require.NoError(t, err)

		// Module docstring and imports should be preserved
		mergedSource := string(mergedTree.Source())
		assert.Contains(t, mergedSource, "Module docstring",
			"Merged tree should preserve module docstring")
		assert.Contains(t, mergedSource, "import os",
			"Merged tree should preserve imports")
	})

	t.Run("merge handles nil chunks gracefully", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		validTree := createMockParseTreeFromSource(t, language, "def valid(): pass")
		chunks := []*valueobject.ParseTree{validTree, nil, validTree}

		_, err = MergeParseTreeChunks(context.Background(), chunks)
		require.Error(t, err, "Merging with nil chunks should return error")
		assert.Contains(t, err.Error(), "nil", "Error should mention nil chunk")
	})
}

// TestShouldUseChunking tests size-based chunking decision logic.
// RED PHASE: This test will fail because shouldUseChunking doesn't exist yet.
//
// EXPECTED BEHAVIOR:
// - Small files (< 1000 lines) → false (no chunking needed)
// - Large files (> 10000 lines) → true (chunking required)
// - Files with extremely long identifiers → true (buffer overflow risk)
// - Edge cases at threshold should be handled correctly.
func TestShouldUseChunking(t *testing.T) {
	t.Run("small file 100 lines returns false", func(t *testing.T) {
		// Generate small file with 100 functions
		var sb strings.Builder
		for i := range 100 {
			sb.WriteString("def func")
			sb.WriteRune(rune('0' + i%10))
			sb.WriteString("():\n    pass\n")
		}
		source := []byte(sb.String())

		// Call function that doesn't exist yet (RED PHASE)
		result := shouldUseChunking(source)

		assert.False(t, result, "Small file (100 lines) should not require chunking")
	})

	t.Run("large file 10000 lines returns true", func(t *testing.T) {
		// Generate large file with 10000 functions
		var sb strings.Builder
		for i := range 10000 {
			sb.WriteString("def function_")
			sb.WriteRune(rune('0' + i%10))
			sb.WriteString("():\n    pass\n")
		}
		source := []byte(sb.String())

		result := shouldUseChunking(source)

		assert.True(t, result, "Large file (10000 lines) should require chunking")
	})

	t.Run("medium file with long identifiers returns true", func(t *testing.T) {
		// File with very long function names (buffer overflow risk)
		var sb strings.Builder
		for range 1000 {
			sb.WriteString("def ")
			// Very long identifier (300 characters)
			sb.WriteString(strings.Repeat("very_long_function_name_", 12))
			sb.WriteString("():\n    pass\n")
		}
		source := []byte(sb.String())

		result := shouldUseChunking(source)

		assert.True(t, result,
			"File with long identifiers should require chunking (buffer overflow risk)")
	})

	t.Run("edge case at threshold", func(t *testing.T) {
		// File right at the threshold (around 5000 lines)
		var sb strings.Builder
		for i := range 5000 {
			sb.WriteString("def func")
			sb.WriteRune(rune('0' + i%10))
			sb.WriteString("():\n    pass\n")
		}
		source := []byte(sb.String())

		result := shouldUseChunking(source)

		// At threshold, either behavior is acceptable depending on implementation
		// but should be consistent
		_ = result // Just verify it doesn't panic
	})

	t.Run("empty file returns false", func(t *testing.T) {
		source := []byte("")

		result := shouldUseChunking(source)

		assert.False(t, result, "Empty file should not require chunking")
	})

	t.Run("file with many nested classes returns true", func(t *testing.T) {
		// Complex nested structure increases buffer pressure
		var sb strings.Builder
		for i := range 2000 {
			sb.WriteString("class Class")
			sb.WriteRune(rune('0' + i%10))
			sb.WriteString(":\n")
			sb.WriteString("    def method(self):\n")
			sb.WriteString("        pass\n\n")
		}
		source := []byte(sb.String())

		result := shouldUseChunking(source)

		assert.True(t, result, "File with many classes should require chunking")
	})

	t.Run("file size exceeds 1MB returns true", func(t *testing.T) {
		// Very large file (> 1MB)
		source := []byte(strings.Repeat("# comment\n", 100000))

		result := shouldUseChunking(source)

		assert.True(t, result, "File exceeding 1MB should require chunking")
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

// createMockParseTreeFromSource is defined in python_parser_test.go
