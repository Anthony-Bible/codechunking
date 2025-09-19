package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoParser_TreeSitter_FunctionContent verifies we parse full method content using Tree-sitter.
func TestGoParser_TreeSitter_FunctionContent(t *testing.T) {
	t.Parallel()

	// Load the function sample from repo root (test_function.txt)
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for range 6 { // ascend to repo root
		dir = filepath.Dir(dir)
	}
	funcPath := filepath.Join(dir, "test_function.txt")
	funcBytes, err := os.ReadFile(funcPath)
	require.NoError(t, err, "failed to read test_function.txt")

	// Build minimal compilable Go source: package + function
	source := "package example\n\n" + string(funcBytes) + "\n"

	// Create parser and parse source with real Tree-sitter
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	parseResult, err := parser.Parse(ctx, []byte(source))
	require.NoError(t, err)
	require.NotNil(t, parseResult)
	require.NotNil(t, parseResult.ParseTree)

	// Convert to domain parse tree for extraction
	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseResult.ParseTree)
	require.NoError(t, err)
	require.NotNil(t, domainTree)

	// Extract functions (includes methods)
	opts := outbound.SemanticExtractionOptions{IncludePrivate: true}
	chunks, err := parser.ExtractFunctions(ctx, domainTree, opts)
	require.NoError(t, err)
	require.NotEmpty(t, chunks, "expected at least one function/method chunk")

	// Find our method by name
	var found *outbound.SemanticCodeChunk
	for i := range chunks {
		if chunks[i].Name == "handleRunSafeDemo" {
			found = &chunks[i]
			break
		}
	}
	require.NotNil(t, found, "handleRunSafeDemo should be extracted")

	// Content should exactly match the function text from the file
	expected := strings.TrimSpace(string(funcBytes))
	actual := strings.TrimSpace(found.Content)
	assert.Equal(t, expected, actual, "extracted function content should match source")

	// Basic sanity checks on braces and key lines
	assert.True(t, strings.HasPrefix(actual, "func (api *APIServer) handleRunSafeDemo"))
	assert.True(t, strings.HasSuffix(actual, "}"))
	assert.Contains(t, actual, "if r.Method != http.MethodPost")
}
