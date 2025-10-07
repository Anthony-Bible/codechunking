package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoParser_TreeSitter_FunctionContent verifies we parse full method content using Tree-sitter.
func TestGoParser_TreeSitter_FunctionContent(t *testing.T) {
	t.Parallel()

	// Embedded test fixture: realistic Go HTTP handler method
	methodSource := `func (api *APIServer) handleRunSafeDemo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code string ` + "`json:\"code\"`" + `
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := api.executeCode(req.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"result": result,
		"status": "success",
	})
}`

	// Build minimal compilable Go source: package + method
	source := "package example\n\n" + methodSource + "\n"

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

	// Content should exactly match the method source
	expected := strings.TrimSpace(methodSource)
	actual := strings.TrimSpace(found.Content)
	assert.Equal(t, expected, actual, "extracted function content should match source")

	// Basic sanity checks on braces and key lines
	assert.True(t, strings.HasPrefix(actual, "func (api *APIServer) handleRunSafeDemo"))
	assert.True(t, strings.HasSuffix(actual, "}"))
	assert.Contains(t, actual, "if r.Method != http.MethodPost")
}
