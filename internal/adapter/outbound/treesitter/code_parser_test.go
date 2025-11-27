package treesitter

import (
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTreeSitterCodeParser_ParseDirectory_RelativePaths tests the core functionality
// of converting absolute workspace paths to repository-relative paths in code chunks.
func TestTreeSitterCodeParser_ParseDirectory_RelativePaths(t *testing.T) {
	// This test will FAIL initially because the current implementation
	// stores absolute paths instead of relative paths

	// Create a temporary workspace directory structure
	tempDir := t.TempDir()
	workspacePath := filepath.Join(tempDir, "workspace")
	repoPath := filepath.Join(workspacePath, "my-repo")

	// Create directory structure for all test files
	dirs := []string{
		"src/utils",
		"src/models",
		"internal/config",
		"docs",
	}

	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(repoPath, dir), 0o755))
	}

	// Create test files with different nesting levels
	testFiles := map[string]string{
		"main.go": `package main
func main() { println("root") }`,
		"src/utils/helper.go": `package utils
func Helper() { println("helper") }`,
		"src/models/user.go": `package models
type User struct { Name string }`,
		"internal/config/config.go": `package config
var Config = "test"`,
		"docs/README.md": `# Project Documentation`,
	}

	for relativePath, content := range testFiles {
		fullPath := filepath.Join(repoPath, relativePath)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}

	for relativePath, content := range testFiles {
		fullPath := filepath.Join(repoPath, relativePath)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}

	// Create parser
	ctx := context.Background()
	parser, err := NewTreeSitterCodeParser(ctx)
	require.NoError(t, err)

	// Test parsing with workspace path - this should produce relative paths
	config := outbound.CodeParsingConfig{
		ChunkSizeBytes:   1024,
		MaxFileSizeBytes: 10240,
		IncludeTests:     false,
		ExcludeVendor:    true,
	}

	t.Run("ParseDirectory_ReturnsRelativePaths", func(t *testing.T) {
		chunks, err := parser.ParseDirectory(ctx, repoPath, config)
		require.NoError(t, err)
		require.NotEmpty(t, chunks)

		// Verify that all chunks have repository-relative paths, not absolute paths
		for _, chunk := range chunks {
			// This assertion will FAIL initially because current implementation stores absolute paths
			assert.NotContains(t, chunk.FilePath, workspacePath,
				"Chunk file path should not contain workspace path: %s", chunk.FilePath)
			assert.NotContains(t, chunk.FilePath, tempDir,
				"Chunk file path should not contain temp directory: %s", chunk.FilePath)

			// Verify path is relative to repository root
			assert.False(t, filepath.IsAbs(chunk.FilePath),
				"Chunk file path should be relative, not absolute: %s", chunk.FilePath)

			// Verify the relative path matches expected structure
			found := false
			for expectedPath := range testFiles {
				if chunk.FilePath == expectedPath {
					found = true
					break
				}
			}
			assert.True(t, found, "Unexpected file path in chunk: %s", chunk.FilePath)
		}
	})

	t.Run("ParseDirectory_PreservesFileStructure", func(t *testing.T) {
		chunks, err := parser.ParseDirectory(ctx, repoPath, config)
		require.NoError(t, err)

		// Verify we have chunks from all expected directories
		pathsFound := make(map[string]bool)
		for _, chunk := range chunks {
			pathsFound[chunk.FilePath] = true
		}

		// All files should be represented with relative paths
		for expectedPath := range testFiles {
			assert.True(t, pathsFound[expectedPath],
				"Expected to find chunk for relative path: %s", expectedPath)
		}
	})
}

// TestTreeSitterCodeParser_RelativePathEdgeCases tests edge cases for path conversion.
func TestTreeSitterCodeParser_RelativePathEdgeCases(t *testing.T) {
	// These tests will FAIL initially because edge case handling is not implemented

	tempDir := t.TempDir()
	workspacePath := filepath.Join(tempDir, "workspace")
	repoPath := filepath.Join(workspacePath, "edge-case-repo")

	// Create complex directory structure with edge cases
	dirs := []string{
		".hidden/dir",
		"dir with spaces/subdir",
		"very/deep/nested/directory/structure/for/testing",
		"root-level",
	}

	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(repoPath, dir), 0o755))
	}

	// Create files with edge case names
	edgeCaseFiles := map[string]string{
		".hidden.go": `package main
func hidden() {}`,
		".hidden/dir/file.go": `package hidden
func inHiddenDir() {}`,
		"dir with spaces/file name.go": `package spaces
func spaced() {}`,
		"very/deep/nested/directory/structure/for/testing/deep.go": `package deep
func deep() {}`,
		"root-level/root.go": `package root
func root() {}`,
		"special-chars_123.go": `package special
func special() {}`,
	}

	for relativePath, content := range edgeCaseFiles {
		fullPath := filepath.Join(repoPath, relativePath)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}

	ctx := context.Background()
	parser, err := NewTreeSitterCodeParser(ctx)
	require.NoError(t, err)

	config := outbound.CodeParsingConfig{
		ChunkSizeBytes:   1024,
		MaxFileSizeBytes: 10240,
		IncludeTests:     false,
		ExcludeVendor:    true,
	}

	t.Run("EdgeCase_HiddenDirectories", func(t *testing.T) {
		chunks, err := parser.ParseDirectory(ctx, repoPath, config)
		require.NoError(t, err)

		// Find chunks from hidden directories
		var hiddenChunks []outbound.CodeChunk
		for _, chunk := range chunks {
			if chunk.FilePath == ".hidden.go" || chunk.FilePath == ".hidden/dir/file.go" {
				hiddenChunks = append(hiddenChunks, chunk)
			}
		}

		require.NotEmpty(t, hiddenChunks, "Should find chunks in hidden directories")

		for _, chunk := range hiddenChunks {
			// This will FAIL - hidden directories should be handled properly
			assert.NotEmpty(t, chunk.FilePath, "Hidden directory file path should not be empty")
			assert.NotContains(t, chunk.FilePath, workspacePath,
				"Hidden directory chunk should have relative path: %s", chunk.FilePath)
		}
	})

	t.Run("EdgeCase_SpacesInNames", func(t *testing.T) {
		chunks, err := parser.ParseDirectory(ctx, repoPath, config)
		require.NoError(t, err)

		// Find chunk with spaces in path
		var spacedChunk *outbound.CodeChunk
		for _, chunk := range chunks {
			if chunk.FilePath == "dir with spaces/file name.go" {
				spacedChunk = &chunk
				break
			}
		}

		require.NotNil(t, spacedChunk, "Should find chunk with spaces in path")

		// This will FAIL - spaces in paths should be preserved correctly
		assert.Equal(t, "dir with spaces/file name.go", spacedChunk.FilePath,
			"Spaces in file paths should be preserved")
	})

	t.Run("EdgeCase_DeepNesting", func(t *testing.T) {
		chunks, err := parser.ParseDirectory(ctx, repoPath, config)
		require.NoError(t, err)

		// Find deeply nested chunk
		var deepChunk *outbound.CodeChunk
		for _, chunk := range chunks {
			if chunk.FilePath == "very/deep/nested/directory/structure/for/testing/deep.go" {
				deepChunk = &chunk
				break
			}
		}

		require.NotNil(t, deepChunk, "Should find deeply nested chunk")

		// This will FAIL - deep nesting should be handled correctly
		assert.Equal(t, "very/deep/nested/directory/structure/for/testing/deep.go", deepChunk.FilePath,
			"Deep nesting should be preserved in relative paths")
	})
}

// TestTreeSitterCodeParser_RelativePathErrorHandling tests error conditions.
func TestTreeSitterCodeParser_RelativePathErrorHandling(t *testing.T) {
	// These tests will FAIL initially because proper error handling is not implemented

	tempDir := t.TempDir()

	ctx := context.Background()
	parser, err := NewTreeSitterCodeParser(ctx)
	require.NoError(t, err)

	config := outbound.CodeParsingConfig{
		ChunkSizeBytes:   1024,
		MaxFileSizeBytes: 10240,
		IncludeTests:     false,
		ExcludeVendor:    true,
	}

	t.Run("Error_NonExistentDirectory", func(t *testing.T) {
		nonExistentPath := filepath.Join(tempDir, "does-not-exist")

		chunks, err := parser.ParseDirectory(ctx, nonExistentPath, config)

		// Should return error, not panic
		assert.Error(t, err)
		assert.Nil(t, chunks)
		// With improved validation, we get a more specific error message
		assert.Contains(t, err.Error(), "directory does not exist")
	})

	t.Run("Error_PathCalculationBoundary", func(t *testing.T) {
		// Create a directory that's actually outside the workspace
		// This tests path calculation edge cases
		repoPath := tempDir // Use temp dir as repo path

		// Create a test file
		testFile := filepath.Join(repoPath, "main.go")
		require.NoError(t, os.WriteFile(testFile, []byte("package main\nfunc main() {}"), 0o644))

		// Try to parse with the actual repo path
		// This should handle the path calculation correctly
		chunks, err := parser.ParseDirectory(ctx, repoPath, config)
		require.NoError(t, err)
		require.NotEmpty(t, chunks)

		// This will FAIL - path calculation should handle edge cases gracefully
		for _, chunk := range chunks {
			assert.NotEmpty(t, chunk.FilePath, "File path should not be empty even in edge cases")
		}
	})
}

// TestTreeSitterCodeParser_RelativePathIntegration tests integration with existing functionality.
func TestTreeSitterCodeParser_RelativePathIntegration(t *testing.T) {
	// These tests will FAIL initially because relative paths break existing assumptions

	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "integration-repo")

	// Create realistic project structure
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, "cmd", "api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, "internal", "service"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, "pkg", "utils"), 0o755))

	projectFiles := map[string]string{
		"go.mod": "module example.com/test\n\ngo 1.21",
		"main.go": `package main
import "example.com/test/internal/service"
func main() { service.Run() }`,
		"cmd/api/server.go": `package api
func StartServer() {}`,
		"internal/service/service.go": `package service
func Run() {}`,
		"pkg/utils/helper.go": `package utils
func Help() {}`,
	}

	for relativePath, content := range projectFiles {
		fullPath := filepath.Join(repoPath, relativePath)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}

	ctx := context.Background()
	parser, err := NewTreeSitterCodeParser(ctx)
	require.NoError(t, err)

	config := outbound.CodeParsingConfig{
		ChunkSizeBytes:   1024,
		MaxFileSizeBytes: 10240,
		IncludeTests:     false,
		ExcludeVendor:    true,
	}

	t.Run("Integration_WithSemanticAnalysis", func(t *testing.T) {
		chunks, err := parser.ParseDirectory(ctx, repoPath, config)
		require.NoError(t, err)
		require.NotEmpty(t, chunks)

		// Verify semantic information is preserved with relative paths
		var goChunks []outbound.CodeChunk
		for _, chunk := range chunks {
			if chunk.Language == "Go" {
				goChunks = append(goChunks, chunk)
			}
		}

		require.NotEmpty(t, goChunks)

		for _, chunk := range goChunks {
			// This will FAIL - semantic info should be preserved with relative paths
			assert.NotEmpty(t, chunk.FilePath, "Go chunk should have relative file path")
			assert.Equal(t, "Go", chunk.Language, "Language should be preserved")
			assert.NotEmpty(t, chunk.Content, "Content should be preserved")
			assert.NotEmpty(t, chunk.Hash, "Hash should be calculated")

			// Verify path is relative
			assert.False(t, filepath.IsAbs(chunk.FilePath),
				"Path should be relative: %s", chunk.FilePath)
			assert.NotContains(t, chunk.FilePath, tempDir,
				"Path should not contain temp dir: %s", chunk.FilePath)
		}
	})

	t.Run("Integration_WithRepositoryContext", func(t *testing.T) {
		// Test that chunks can be associated with repository context
		repoID := uuid.New()

		chunks, err := parser.ParseDirectory(ctx, repoPath, config)
		require.NoError(t, err)

		// Simulate associating chunks with repository
		for i := range chunks {
			chunks[i].RepositoryID = repoID
		}

		// Verify all chunks have the repository ID and relative paths
		for _, chunk := range chunks {
			assert.Equal(t, repoID, chunk.RepositoryID, "Repository ID should be set")
			assert.NotEmpty(t, chunk.FilePath, "File path should be relative")

			// This will FAIL - chunks should have repository-relative paths
			found := false
			for expectedPath := range projectFiles {
				if chunk.FilePath == expectedPath {
					found = true
					break
				}
			}
			assert.True(t, found, "Chunk should have expected relative path: %s", chunk.FilePath)
		}
	})
}

// TestTreeSitterCodeParser_RelativePathPerformance tests performance with relative paths.
func TestTreeSitterCodeParser_RelativePathPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// This test will verify that relative path calculation doesn't significantly impact performance
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "perf-repo")

	// Create many files in nested structure
	numFiles := 100
	for i := range numFiles {
		dirPath := filepath.Join(repoPath, "src", "module", fmt.Sprintf("subdir%d", i%10))
		require.NoError(t, os.MkdirAll(dirPath, 0o755))

		filePath := filepath.Join(dirPath, fmt.Sprintf("file%d.go", i))
		content := fmt.Sprintf(`package subdir%d
func Func%d() { return %d }`, i%10, i, i)
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))
	}

	ctx := context.Background()
	parser, err := NewTreeSitterCodeParser(ctx)
	require.NoError(t, err)

	config := outbound.CodeParsingConfig{
		ChunkSizeBytes:   1024,
		MaxFileSizeBytes: 10240,
		IncludeTests:     false,
		ExcludeVendor:    true,
	}

	t.Run("Performance_RelativePathCalculation", func(t *testing.T) {
		start := time.Now()

		chunks, err := parser.ParseDirectory(ctx, repoPath, config)

		elapsed := time.Since(start)

		require.NoError(t, err)
		require.NotEmpty(t, chunks)

		// Performance should be reasonable even with relative path calculation
		assert.Less(t, elapsed, 10*time.Second, "Parsing with relative paths should complete in reasonable time")

		// Verify all chunks have relative paths
		for _, chunk := range chunks {
			// This will FAIL - all chunks should have relative paths
			assert.NotContains(t, chunk.FilePath, tempDir,
				"Performance test: chunk should have relative path: %s", chunk.FilePath)
			assert.False(t, filepath.IsAbs(chunk.FilePath),
				"Performance test: path should be relative: %s", chunk.FilePath)
		}

		t.Logf("Processed %d files with relative path calculation in %v", len(chunks), elapsed)
	})
}

// TestTreeSitterCodeParser_EmptyChunkFiltering tests that chunks with empty content
// are filtered out during parsing and logged appropriately.
func TestTreeSitterCodeParser_EmptyChunkFiltering(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()

	// Create test files including one that might produce empty chunks
	files := map[string]string{
		"valid.go": `package test

func ValidFunction() {
	fmt.Println("Hello")
}

func AnotherFunction() int {
	return 42
}
`,
		"empty_interface.go": `package test

// This might produce an empty chunk depending on tree-sitter behavior
type EmptyInterface interface{}
`,
		"minimal.go": `package empty

// File with minimal content
`,
	}

	for filename, content := range files {
		path := filepath.Join(tempDir, filename)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	ctx := context.Background()
	parser, err := NewTreeSitterCodeParser(ctx)
	require.NoError(t, err)

	config := outbound.CodeParsingConfig{
		ChunkSizeBytes: 1024,
		IncludeTests:   false,
		ExcludeVendor:  true,
	}

	chunks, err := parser.ParseDirectory(ctx, tempDir, config)
	require.NoError(t, err)

	// Verify all returned chunks have non-empty content
	for i, chunk := range chunks {
		assert.NotEmpty(t, strings.TrimSpace(chunk.Content),
			"Chunk %d should not have empty content [file=%s, type=%s, entity=%s]",
			i, chunk.FilePath, chunk.Type, chunk.EntityName)
	}

	// Verify basic chunk structure
	assert.NotEmpty(t, chunks, "Should have at least some valid chunks")

	// Verify all chunks have required fields
	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk.ID, "Chunk should have an ID")
		assert.NotEmpty(t, chunk.FilePath, "Chunk should have a file path")
		assert.NotEmpty(t, chunk.Language, "Chunk should have a language")
		assert.NotZero(t, chunk.Size, "Chunk should have a size")
		assert.NotEmpty(t, chunk.Hash, "Chunk should have a hash")
	}
}
