package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMinimalParseTree creates a minimal ParseTree for testing adapter behavior.
func createMinimalParseTree(language valueobject.Language) *valueobject.ParseTree {
	// Create minimal parse node directly since ParseNode is a simple struct
	rootNode := &valueobject.ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   12,
		StartPos:  valueobject.Position{Row: 0, Column: 0},
		EndPos:    valueobject.Position{Row: 0, Column: 12},
		Children:  nil,
	}

	// Create minimal metadata
	metadata, err := valueobject.NewParseMetadata(
		time.Millisecond,
		"test-parser",
		"1.0.0",
	)
	if err != nil {
		panic("Failed to create minimal metadata: " + err.Error())
	}

	// Create parse tree
	parseTree, err := valueobject.NewParseTree(
		context.Background(),
		language,
		rootNode,
		[]byte("package main"),
		metadata,
	)
	if err != nil {
		panic("Failed to create minimal parse tree: " + err.Error())
	}

	return parseTree
}

// TestSemanticTraverserAdapter_ValidatesInput tests input validation.
func TestSemanticTraverserAdapter_ValidatesInput(t *testing.T) {
	ctx := context.Background()
	adapter := NewSemanticTraverserAdapterWithFactory(nil)
	options := outbound.SemanticExtractionOptions{}

	// Test nil parse tree
	result, err := adapter.ExtractFunctions(ctx, nil, options)
	assert.Error(t, err, "Should return error for nil parse tree")
	assert.Nil(t, result, "Should return nil result on error")
	assert.Contains(t, err.Error(), "parse tree cannot be nil", "Should have descriptive error message")
}

// TestSemanticTraverserAdapter_FallsBackToLegacy tests that the adapter falls back to legacy implementation when factory is nil.
func TestSemanticTraverserAdapter_FallsBackToLegacy(t *testing.T) {
	ctx := context.Background()

	// Create language
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	// Create minimal parse tree
	parseTree := createMinimalParseTree(language)

	// Create adapter with nil factory to test fallback
	adapter := NewSemanticTraverserAdapterWithFactory(nil)

	// Extract functions - should fall back to legacy implementation
	options := outbound.SemanticExtractionOptions{IncludePrivate: true}
	_, err = adapter.ExtractFunctions(ctx, parseTree, options)

	// Verify fallback behavior - should not error
	require.NoError(t, err, "Should not return error when falling back to legacy implementation")
	// Note: legacy implementation may return nil or empty slice depending on language/content
}

// TestSemanticTraverserAdapter_UnifiedInterface tests that all extraction methods work through the same interface.
func TestSemanticTraverserAdapter_UnifiedInterface(t *testing.T) {
	ctx := context.Background()

	// Create language and parse tree
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parseTree := createMinimalParseTree(language)
	options := outbound.SemanticExtractionOptions{IncludePrivate: true}

	// Create adapter with nil factory (will use fallback implementations)
	adapter := NewSemanticTraverserAdapterWithFactory(nil)

	// Test all extraction methods work through unified interface
	t.Run("ExtractFunctions", func(t *testing.T) {
		_, err := adapter.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)
	})

	t.Run("ExtractClasses", func(t *testing.T) {
		_, err := adapter.ExtractClasses(ctx, parseTree, options)
		require.NoError(t, err)
	})

	t.Run("ExtractInterfaces", func(t *testing.T) {
		_, err := adapter.ExtractInterfaces(ctx, parseTree, options)
		require.NoError(t, err)
	})

	t.Run("ExtractVariables", func(t *testing.T) {
		_, err := adapter.ExtractVariables(ctx, parseTree, options)
		require.NoError(t, err)
	})

	t.Run("ExtractImports", func(t *testing.T) {
		_, err := adapter.ExtractImports(ctx, parseTree, options)
		require.NoError(t, err)
	})

	t.Run("ExtractModules", func(t *testing.T) {
		_, err := adapter.ExtractModules(ctx, parseTree, options)
		require.NoError(t, err)
	})
}

// TestSemanticTraverserAdapter_CrossLanguageSupport tests that adapter works with different languages.
func TestSemanticTraverserAdapter_CrossLanguageSupport(t *testing.T) {
	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{IncludePrivate: true}

	languages := []string{
		valueobject.LanguageGo,
		valueobject.LanguagePython,
		valueobject.LanguageJavaScript,
	}

	for _, langName := range languages {
		t.Run(langName, func(t *testing.T) {
			// Create language and parse tree
			language, err := valueobject.NewLanguage(langName)
			require.NoError(t, err)
			parseTree := createMinimalParseTree(language)

			// Create adapter with nil factory (will use fallback implementations)
			adapter := NewSemanticTraverserAdapterWithFactory(nil)

			// Extract functions
			_, err = adapter.ExtractFunctions(ctx, parseTree, options)

			// Verify language-agnostic behavior
			require.NoError(t, err)
		})
	}
}

// TestSemanticTraverserAdapter_FactoryIntegration tests that the adapter works when created with the default constructor.
func TestSemanticTraverserAdapter_FactoryIntegration(t *testing.T) {
	ctx := context.Background()

	// Create language and parse tree
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parseTree := createMinimalParseTree(language)

	// Create adapter using the default constructor (may or may not have a real factory)
	adapter := NewSemanticTraverserAdapter()
	require.NotNil(t, adapter, "Adapter should be created successfully")

	// Test basic functionality works
	options := outbound.SemanticExtractionOptions{IncludePrivate: true}
	result, err := adapter.ExtractFunctions(ctx, parseTree, options)

	// Should work regardless of whether factory creation succeeded
	require.NoError(t, err, "Should handle factory creation gracefully")
	// Note: result might be nil or empty depending on the parse tree content
	// The important thing is that it doesn't error out
	_ = result // Just verify no error occurred
}

// TestSemanticTraverserAdapter_ErrorHandling tests error handling for various edge cases.
func TestSemanticTraverserAdapter_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	adapter := NewSemanticTraverserAdapterWithFactory(nil)

	t.Run("NilParseTree", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}
		result, err := adapter.ExtractFunctions(ctx, nil, options)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parse tree cannot be nil")
	})

	t.Run("InvalidOptions", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)
		parseTree := createMinimalParseTree(language)

		// Test with extreme options that might cause validation errors
		options := outbound.SemanticExtractionOptions{
			MaxDepth: -1, // Invalid negative depth
		}

		// Should handle gracefully - either succeed with defaults or return descriptive error
		result, err := adapter.ExtractFunctions(ctx, parseTree, options)
		if err != nil {
			// Check for either "validation" or "invalid option" in error message
			errorMsg := err.Error()
			assert.True(t,
				strings.Contains(errorMsg, "validation") || strings.Contains(errorMsg, "invalid option"),
				"Error should be related to validation, got: %s", errorMsg)
		} else {
			assert.NotNil(t, result, "If no error, should return valid result")
		}
	})
}
