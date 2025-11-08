package pythonparser

import (
	"bytes"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// Chunking thresholds for determining when to split large Python files
	// to prevent tree-sitter buffer overflow on extremely large files.

	// MaxFileSizeBeforeChunking is the maximum file size in bytes before chunking is required.
	// Files larger than 500KB are automatically chunked to prevent memory issues.
	MaxFileSizeBeforeChunking = 500 * 1024 // 500KB

	// MaxLineCountBeforeChunking is the maximum number of lines before chunking is required.
	// Files with more than 5000 lines are automatically chunked for performance.
	MaxLineCountBeforeChunking = 5000

	// MaxTokenLengthBeforeChunking is the maximum token length before chunking is required.
	// Very long identifiers (>200 chars) can cause tree-sitter buffer overflow.
	MaxTokenLengthBeforeChunking = 200

	// MaxDefinitionsBeforeChunking is the maximum number of top-level definitions before chunking.
	// Files with more than 2000 function/class definitions benefit from chunking.
	MaxDefinitionsBeforeChunking = 2000

	// DefaultChunkSize is the default maximum chunk size in bytes when splitting files.
	// Each chunk should contain complete top-level definitions to maintain semantic integrity.
	DefaultChunkSize = 5000

	// ErrorContextRadius is the number of bytes to show before and after an error location.
	// Used to provide meaningful context when reporting syntax errors.
	ErrorContextRadius = 25
)

// ChunkSourceByTopLevelDefinitions splits Python source code into chunks by top-level definitions.
//
// This function prevents tree-sitter buffer overflow on extremely large files by intelligently
// splitting the source code at natural boundaries (function and class definitions) while
// preserving module-level constructs (imports, docstrings, comments) in the first chunk.
//
// The chunking algorithm ensures:
//   - Each chunk contains complete top-level definitions (no partial functions/classes)
//   - Module-level imports, docstrings, and comments are preserved in the first chunk
//   - Chunks respect the maxChunkSize parameter to limit memory usage
//   - Empty source returns an empty slice (not an error)
//   - Source smaller than maxChunkSize returns a single-element slice
//   - Context cancellation is checked during processing loops
//
// Parameters:
//   - ctx: Context for cancellation and logging
//   - source: The Python source code to chunk (may be empty)
//   - maxChunkSize: Maximum size in bytes for each chunk (complete definitions may exceed this)
//
// Returns:
//   - [][]byte: Slice of source code chunks, each containing complete top-level definitions
//   - error: Returns error if context is cancelled or validation fails
//
// Example usage:
//
//	source := []byte("import os\n\ndef func1():\n    pass\n\ndef func2():\n    pass")
//	chunks, err := ChunkSourceByTopLevelDefinitions(ctx, source, 5000)
//	// chunks[0] will contain: "import os\n\ndef func1():\n    pass\n\ndef func2():\n    pass"
//
// Performance characteristics:
//   - Time complexity: O(n) where n is the number of lines in source
//   - Space complexity: O(n) for the chunks slice and line splits
//   - Single pass through source lines for optimal performance
func ChunkSourceByTopLevelDefinitions(ctx context.Context, source []byte, maxChunkSize int) ([][]byte, error) {
	// Validate context
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before chunking: %w", err)
	}

	// Validate input
	if maxChunkSize <= 0 {
		return nil, errors.New("maxChunkSize must be positive")
	}

	// Handle empty source
	if len(source) == 0 {
		return [][]byte{}, nil
	}

	// If source is smaller than max chunk, return as single chunk
	if len(source) <= maxChunkSize {
		return [][]byte{source}, nil
	}

	lines := bytes.Split(source, []byte("\n"))
	lineCount := len(lines)

	var chunks [][]byte
	var currentChunk [][]byte
	currentSize := 0
	moduleHeaderEnd := 0

	// Find end of module header (imports, module docstring, etc.)
	for i, line := range lines {
		// Check context cancellation periodically
		if i%1000 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context cancelled during module header scan: %w", err)
			}
		}

		trimmed := bytes.TrimSpace(line)
		// Stop at first top-level definition
		if bytes.HasPrefix(trimmed, []byte("def ")) || bytes.HasPrefix(trimmed, []byte("class ")) {
			moduleHeaderEnd = i
			break
		}
		// Also include shebangs, comments, imports, constants
		if len(trimmed) == 0 ||
			bytes.HasPrefix(trimmed, []byte("#")) ||
			bytes.HasPrefix(trimmed, []byte("\"\"\"")) ||
			bytes.HasPrefix(trimmed, []byte("'''")) ||
			bytes.HasPrefix(trimmed, []byte("import ")) ||
			bytes.HasPrefix(trimmed, []byte("from ")) ||
			bytes.HasPrefix(line, []byte("#!")) ||
			(len(trimmed) > 0 && !bytes.HasPrefix(trimmed, []byte("def ")) && !bytes.HasPrefix(trimmed, []byte("class "))) {
			moduleHeaderEnd = i + 1
		}
	}

	// Add module header to first chunk if present
	if moduleHeaderEnd > 0 {
		for i := 0; i < moduleHeaderEnd && i < len(lines); i++ {
			currentChunk = append(currentChunk, lines[i])
			currentSize += len(lines[i]) + 1 // +1 for newline
		}
	}

	// Process remaining lines
	for i := moduleHeaderEnd; i < len(lines); i++ {
		// Check context cancellation periodically (every 1000 lines)
		if i%1000 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context cancelled during chunking at line %d/%d: %w", i, lineCount, err)
			}
		}

		line := lines[i]
		trimmed := bytes.TrimSpace(line)

		// Check if this is a top-level definition (no leading whitespace except for empty lines)
		isTopLevel := (bytes.HasPrefix(trimmed, []byte("def ")) || bytes.HasPrefix(trimmed, []byte("class "))) &&
			(len(line) == 0 || line[0] != ' ' && line[0] != '\t')

		if isTopLevel && currentSize > 0 {
			// We hit a new top-level definition and have content in current chunk
			// Check if adding this definition would exceed max chunk size
			if currentSize >= maxChunkSize {
				// Flush current chunk before starting new one
				chunks = append(chunks, bytes.Join(currentChunk, []byte("\n")))
				currentChunk = nil
				currentSize = 0
			}
		}

		// Add line to current chunk
		currentChunk = append(currentChunk, line)
		currentSize += len(line) + 1
	}

	// Flush final chunk
	if len(currentChunk) > 0 {
		chunks = append(chunks, bytes.Join(currentChunk, []byte("\n")))
	}

	// If we produced no chunks, return original source
	if len(chunks) == 0 {
		return [][]byte{source}, nil
	}

	// Log successful chunking with metadata
	slogger.Info(ctx, "Source code chunked successfully", slogger.Fields{
		"source_size_bytes": len(source),
		"line_count":        lineCount,
		"chunk_count":       len(chunks),
		"max_chunk_size":    maxChunkSize,
	})

	return chunks, nil
}

// MergeParseTreeChunks merges multiple ParseTree chunks into a single ParseTree.
//
// This function reconstructs the full parse tree after chunked parsing by combining
// all chunk parse trees into a single unified tree. It preserves the semantic structure
// of the original source code while aggregating metadata from all chunks.
//
// The merging process:
//   - Concatenates all chunk sources to reconstruct the original source
//   - Combines all root node children from each chunk into a single module node
//   - Aggregates metadata (node counts, max depth, total parse duration)
//   - Validates chunk integrity (no nil chunks allowed)
//   - Preserves language information from the first chunk
//   - Checks context cancellation during merge operations
//
// Parameters:
//   - ctx: Context for cancellation and logging
//   - chunks: Slice of ParseTree chunks to merge (must not be empty, no nil elements)
//
// Returns:
//   - *valueobject.ParseTree: Merged parse tree containing all chunks
//   - error: Returns error if chunks is empty, contains nil elements, or merge fails
//
// Error conditions:
//   - Context cancelled: "context cancelled during merge: ..."
//   - Empty chunks slice: "cannot merge empty chunk list"
//   - Nil chunk element: "chunk at index X is nil"
//   - Metadata creation failure: "failed to create merged metadata: ..."
//   - ParseTree creation failure: "failed to create merged parse tree: ..."
//
// Example usage:
//
//	chunk1, _ := parsePythonSource(ctx, source1)
//	chunk2, _ := parsePythonSource(ctx, source2)
//	merged, err := MergeParseTreeChunks(ctx, []*valueobject.ParseTree{chunk1, chunk2})
//	// merged contains combined parse tree with all nodes from both chunks
//
// Performance characteristics:
//   - Time complexity: O(n) where n is total number of nodes across all chunks
//   - Space complexity: O(n) for merged source and children slices
func MergeParseTreeChunks(ctx context.Context, chunks []*valueobject.ParseTree) (*valueobject.ParseTree, error) {
	// Validate context
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before merge: %w", err)
	}

	if len(chunks) == 0 {
		return nil, errors.New("cannot merge empty chunk list")
	}

	// Check for nil chunks
	for i, chunk := range chunks {
		if chunk == nil {
			return nil, fmt.Errorf("chunk at index %d is nil", i)
		}
	}

	// If single chunk, return it as-is
	if len(chunks) == 1 {
		return chunks[0], nil
	}

	// Merge all sources
	var mergedSource []byte
	for i, chunk := range chunks {
		// Check context cancellation periodically
		if i%100 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context cancelled during source merge at chunk %d/%d: %w", i, len(chunks), err)
			}
		}
		mergedSource = append(mergedSource, chunk.Source()...)
	}

	// Collect all children from all chunk root nodes
	var mergedChildren []*valueobject.ParseNode
	for i, chunk := range chunks {
		// Check context cancellation periodically
		if i%100 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context cancelled during node merge at chunk %d/%d: %w", i, len(chunks), err)
			}
		}

		rootNode := chunk.RootNode()
		if rootNode != nil && rootNode.Children != nil {
			mergedChildren = append(mergedChildren, rootNode.Children...)
		}
	}

	// Create new root node with merged children
	mergedRoot := &valueobject.ParseNode{
		Type:      "module",
		StartByte: 0,
		EndByte:   valueobject.ClampToUint32(len(mergedSource)),
		StartPos:  valueobject.Position{Row: 0, Column: 0},
		EndPos:    valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(len(mergedSource))},
		Children:  mergedChildren,
	}

	// Calculate merged metadata
	totalNodes := 0
	maxDepth := 0
	totalDuration := time.Duration(0)

	for _, chunk := range chunks {
		metadata := chunk.Metadata()
		totalNodes += metadata.NodeCount
		if metadata.MaxDepth > maxDepth {
			maxDepth = metadata.MaxDepth
		}
		totalDuration += metadata.ParseDuration
	}

	// Create merged metadata
	mergedMetadata, err := valueobject.NewParseMetadata(
		totalDuration,
		"0.0.0", // treeSitterVersion
		"0.0.0", // grammarVersion
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create merged metadata: %w", err)
	}

	// Create merged parse tree
	firstChunkLang := chunks[0].Language() // #nosec G602 - already checked len(chunks) > 0 above
	mergedTree, mergeErr := valueobject.NewParseTree(
		ctx,
		firstChunkLang,
		mergedRoot,
		mergedSource,
		mergedMetadata,
	)
	if mergeErr != nil {
		return nil, fmt.Errorf("failed to create merged parse tree: %w", mergeErr)
	}

	// Log successful merge with metadata
	slogger.Info(ctx, "Parse tree chunks merged successfully", slogger.Fields{
		"chunk_count":        len(chunks),
		"merged_node_count":  totalNodes,
		"merged_max_depth":   maxDepth,
		"merged_source_size": len(mergedSource),
		"total_parse_time":   totalDuration.String(),
	})

	return mergedTree, nil
}

// shouldUseChunking determines if source code should be chunked before parsing.
// Large files or files with pathological patterns need chunking to avoid buffer overflow.
// This function uses various heuristics to detect files that may cause tree-sitter issues:
// - File size exceeds MaxFileSizeBeforeChunking (500KB)
// - Line count exceeds MaxLineCountBeforeChunking (5000 lines)
// - Contains extremely long identifiers (>MaxTokenLengthBeforeChunking chars)
// - Contains many top-level definitions (>MaxDefinitionsBeforeChunking).
func shouldUseChunking(source []byte) bool {
	// Empty files don't need chunking
	if len(source) == 0 {
		return false
	}

	// Early exit: Files over MaxFileSizeBeforeChunking need chunking (fastest check)
	if len(source) > MaxFileSizeBeforeChunking {
		return true
	}

	// Count lines - second fastest heuristic
	lineCount := bytes.Count(source, []byte("\n"))

	// Files with > MaxLineCountBeforeChunking lines need chunking
	if lineCount > MaxLineCountBeforeChunking {
		return true
	}

	// Check for very long identifiers (buffer overflow risk)
	// Look for extremely long tokens without whitespace
	maxTokenLength := 0
	currentTokenLength := 0
	for _, b := range source {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			if currentTokenLength > maxTokenLength {
				maxTokenLength = currentTokenLength
			}
			currentTokenLength = 0
		} else {
			currentTokenLength++
		}
	}
	if currentTokenLength > maxTokenLength {
		maxTokenLength = currentTokenLength
	}

	// If we find tokens longer than MaxTokenLengthBeforeChunking characters, use chunking
	if maxTokenLength > MaxTokenLengthBeforeChunking {
		return true
	}

	// For files with many classes/functions, use chunking
	// Estimate: count "def " and "class " occurrences
	defCount := bytes.Count(source, []byte("def "))
	classCount := bytes.Count(source, []byte("class "))
	totalDefs := defCount + classCount

	// If more than MaxDefinitionsBeforeChunking definitions, use chunking
	if totalDefs > MaxDefinitionsBeforeChunking {
		return true
	}

	return false
}
