package goparser

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

// TestGoImportParser_ParseGoImportDeclaration_IndividualImports tests parsing of individual import statements.
// This is a RED PHASE test that defines expected behavior for Go individual import parsing.
func TestGoImportParser_ParseGoImportDeclaration_IndividualImports(t *testing.T) {
	sourceCode := `package main

import "fmt"
import "strings"
import "context"
import "encoding/json"
import "net/http"`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	// Find import declarations
	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 5, "Should find 5 import declarations")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test each individual import
	expectedImports := []struct {
		path     string
		alias    string
		wildcard bool
	}{
		{"fmt", "", false},
		{"strings", "", false},
		{"context", "", false},
		{"encoding/json", "", false},
		{"net/http", "", false},
	}

	for i, expected := range expectedImports {
		result := parser.ParseGoImportDeclaration(context.Background(), parseTree, importDecls[i], options, time.Now())
		require.Len(t, result, 1, "Should parse 1 import")

		importDecl := result[0]
		assert.Equal(t, expected.path, importDecl.Path)
		assert.Equal(t, expected.alias, importDecl.Alias)
		assert.Equal(t, expected.wildcard, importDecl.IsWildcard)
		assert.Empty(t, importDecl.ImportedSymbols)
		assert.Contains(t, importDecl.Content, expected.path)
		assert.Greater(t, importDecl.EndByte, importDecl.StartByte)
	}
}

// TestGoImportParser_ParseGoImportDeclaration_ImportBlock tests parsing of import blocks.
// This is a RED PHASE test that defines expected behavior for Go import block parsing.
func TestGoImportParser_ParseGoImportDeclaration_ImportBlock(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"
	"strings"
	"context"
	"encoding/json"
	"net/http"
	"time"
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	// Find import declaration (single block)
	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1, "Should find 1 import declaration block")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoImportDeclaration(context.Background(), parseTree, importDecls[0], options, time.Now())
	require.Len(t, result, 6, "Should parse 6 imports from block")

	expectedPaths := []string{"fmt", "strings", "context", "encoding/json", "net/http", "time"}

	for i, expected := range expectedPaths {
		importDecl := result[i]
		assert.Equal(t, expected, importDecl.Path)
		assert.Empty(t, importDecl.Alias)
		assert.False(t, importDecl.IsWildcard)
		assert.Empty(t, importDecl.ImportedSymbols)
		assert.Contains(t, importDecl.Content, expected)
	}
}

// TestGoImportParser_ParseGoImportDeclaration_WithAliases tests parsing of imports with aliases.
// This is a RED PHASE test that defines expected behavior for Go import alias parsing.
func TestGoImportParser_ParseGoImportDeclaration_WithAliases(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"
	f "fmt"              // Regular alias
	"context"
	ctx "context"        // Regular alias
	_ "github.com/lib/pq" // Blank import
	. "math"             // Dot import (wildcard)
	"net/http"
	h "net/http"         // Regular alias
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1, "Should find 1 import declaration block")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoImportDeclaration(context.Background(), parseTree, importDecls[0], options, time.Now())
	require.Len(t, result, 8, "Should parse 8 imports from block")

	expectedImports := []struct {
		path     string
		alias    string
		wildcard bool
	}{
		{"fmt", "", false},
		{"fmt", "f", false},
		{"context", "", false},
		{"context", "ctx", false},
		{"github.com/lib/pq", "_", false},
		{"math", ".", true}, // Dot import is wildcard
		{"net/http", "", false},
		{"net/http", "h", false},
	}

	for i, expected := range expectedImports {
		importDecl := result[i]
		assert.Equal(t, expected.path, importDecl.Path, "Path should match for import %d", i)
		assert.Equal(t, expected.alias, importDecl.Alias, "Alias should match for import %d", i)
		assert.Equal(t, expected.wildcard, importDecl.IsWildcard, "Wildcard flag should match for import %d", i)

		if expected.alias != "" {
			assert.Contains(t, importDecl.Content, expected.alias, "Content should contain alias for import %d", i)
		}
		assert.Contains(t, importDecl.Content, expected.path, "Content should contain path for import %d", i)
	}
}

// TestGoImportParser_ParseGoImportSpec_ComplexPaths tests parsing of individual import specs with complex paths.
// This is a RED PHASE test that defines expected behavior for Go import spec parsing.
func TestGoImportParser_ParseGoImportSpec_ComplexPaths(t *testing.T) {
	sourceCode := `package main

import (
	// Standard library
	"fmt"
	
	// Standard library with subpackage
	"net/http"
	"encoding/json"
	
	// Third-party packages
	"github.com/stretchr/testify/assert"
	"github.com/gorilla/mux"
	
	// Local packages
	"./internal/config"
	"../shared/utils"
	
	// Versioned imports
	"example.com/module/v2/package"
	
	// With aliases
	testify "github.com/stretchr/testify/assert"
	localConfig "./internal/config"
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1, "Should find 1 import declaration block")

	// Find import specs within the declaration
	importSpecs := findChildrenByType(importDecls[0], "import_spec")
	require.Len(t, importSpecs, 10, "Should find 10 import specs")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	expectedSpecs := []struct {
		path     string
		alias    string
		wildcard bool
	}{
		{"fmt", "", false},
		{"net/http", "", false},
		{"encoding/json", "", false},
		{"github.com/stretchr/testify/assert", "", false},
		{"github.com/gorilla/mux", "", false},
		{"./internal/config", "", false},
		{"../shared/utils", "", false},
		{"example.com/module/v2/package", "", false},
		{"github.com/stretchr/testify/assert", "testify", false},
		{"./internal/config", "localConfig", false},
	}

	for i, expected := range expectedSpecs {
		result := parser.ParseGoImportSpec(parseTree, importSpecs[i], options)
		require.NotNil(t, result, "Should parse import spec %d", i)

		assert.Equal(t, expected.path, result.Path, "Path should match for import spec %d", i)
		assert.Equal(t, expected.alias, result.Alias, "Alias should match for import spec %d", i)
		assert.Equal(t, expected.wildcard, result.IsWildcard, "Wildcard flag should match for import spec %d", i)
		assert.Contains(t, result.Content, expected.path, "Content should contain path for import spec %d", i)

		if expected.alias != "" {
			assert.Contains(t, result.Content, expected.alias, "Content should contain alias for import spec %d", i)
		}
	}
}

// TestGoImportParser_ParseGoImportSpec_SpecialImports tests parsing of special import types.
// This is a RED PHASE test that defines expected behavior for Go special import parsing.
func TestGoImportParser_ParseGoImportSpec_SpecialImports(t *testing.T) {
	sourceCode := `package main

import (
	// Blank import for side effects
	_ "github.com/lib/pq"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	
	// Dot imports for namespace merging
	. "math"
	. "strings"
	
	// C imports for cgo
	"C"
	
	// Unsafe imports
	"unsafe"
	
	// Build tag specific imports (commented but parsed)
	// +build linux
	// "syscall"
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1, "Should find 1 import declaration block")

	importSpecs := findChildrenByType(importDecls[0], "import_spec")
	require.Len(t, importSpecs, 6, "Should find 6 import specs")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	expectedSpecs := []struct {
		path     string
		alias    string
		wildcard bool
	}{
		{"github.com/lib/pq", "_", false},
		{"github.com/golang-migrate/migrate/v4/source/file", "_", false},
		{"math", ".", true},
		{"strings", ".", true},
		{"C", "", false},
		{"unsafe", "", false},
	}

	for i, expected := range expectedSpecs {
		result := parser.ParseGoImportSpec(parseTree, importSpecs[i], options)
		require.NotNil(t, result, "Should parse special import spec %d", i)

		assert.Equal(t, expected.path, result.Path, "Path should match for special import %d", i)
		assert.Equal(t, expected.alias, result.Alias, "Alias should match for special import %d", i)
		assert.Equal(t, expected.wildcard, result.IsWildcard, "Wildcard flag should match for special import %d", i)

		// Special validation for blank imports
		if expected.alias == "_" {
			assert.Contains(t, result.Content, "_", "Blank import should contain underscore")
		}

		// Special validation for dot imports
		if expected.wildcard {
			assert.Contains(t, result.Content, ".", "Dot import should contain dot")
		}
	}
}

// TestGoImportParser_MixedImportStyles tests parsing of mixed import declaration styles.
// This is a RED PHASE test that defines expected behavior for mixed Go import styles.
func TestGoImportParser_MixedImportStyles(t *testing.T) {
	sourceCode := `package main

// Individual imports
import "fmt"
import "strings"

// Import block
import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Another individual import
import log "github.com/sirupsen/logrus"`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 4, "Should find 4 import declarations") // 2 individual + 1 block + 1 individual

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	allImports := []outbound.ImportDeclaration{}

	// Parse each import declaration
	for _, importDecl := range importDecls {
		result := parser.ParseGoImportDeclaration(context.Background(), parseTree, importDecl, options, time.Now())
		allImports = append(allImports, result...)
	}

	// Should have 8 total imports: 2 individual + 4 from block + 1 individual + 1 aliased
	require.Len(t, allImports, 8, "Should parse 8 total imports")

	expectedPaths := []string{
		"fmt", "strings", // Individual imports
		"context", "encoding/json", "net/http", "time", // Block imports
		"github.com/sirupsen/logrus", // Aliased import
	}

	pathCounts := make(map[string]int)
	for _, importDecl := range allImports {
		pathCounts[importDecl.Path]++
	}

	// Verify all expected paths are present
	for _, expectedPath := range expectedPaths {
		assert.Contains(t, pathCounts, expectedPath, "Should contain import for %s", expectedPath)
	}

	// Find the aliased import
	var aliasedImport *outbound.ImportDeclaration
	for i, importDecl := range allImports {
		if importDecl.Alias == "log" {
			aliasedImport = &allImports[i]
			break
		}
	}
	require.NotNil(t, aliasedImport, "Should find aliased import")
	assert.Equal(t, "github.com/sirupsen/logrus", aliasedImport.Path)
	assert.Equal(t, "log", aliasedImport.Alias)
}

// TestGoImportParser_ImportPathExtraction tests extraction of import paths and cleanup.
// This is a RED PHASE test that defines expected behavior for Go import path extraction.
func TestGoImportParser_ImportPathExtraction(t *testing.T) {
	sourceCode := `package main

import (
	// Quotes should be stripped from paths
	"fmt"
	"encoding/json"
	
	// Alias handling
	json "encoding/json"
	
	// Complex paths with quotes
	"github.com/gorilla/mux"
	"example.com/module/v2/package"
	
	// Relative paths
	"./internal/config"
	"../shared/utils"
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoImportDeclaration(context.Background(), parseTree, importDecls[0], options, time.Now())

	// Test that paths don't contain quotes
	for i, importDecl := range result {
		assert.NotContains(t, importDecl.Path, `"`, "Import path %d should not contain quotes: %s", i, importDecl.Path)
		assert.NotContains(
			t,
			importDecl.Path,
			"`",
			"Import path %d should not contain backticks: %s",
			i,
			importDecl.Path,
		)

		// Path should not be empty
		assert.NotEmpty(t, importDecl.Path, "Import path %d should not be empty", i)

		// Content should contain the original quoted path
		assert.True(t,
			strings.Contains(importDecl.Content, `"`+importDecl.Path+`"`) ||
				strings.Contains(importDecl.Content, importDecl.Alias+" "+`"`+importDecl.Path+`"`),
			"Content should contain quoted path for import %d: %s", i, importDecl.Content)
	}
}

// TestGoImportParser_ErrorHandling tests error conditions for import parsing.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestGoImportParser_ErrorHandling(t *testing.T) {
	t.Run("nil parse tree should not panic", func(t *testing.T) {
		parser := NewGoImportParser()

		result := parser.ParseGoImportDeclaration(
			context.Background(),
			nil,
			nil,
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("nil node should not panic", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, "package main")
		parser := NewGoImportParser()

		result := parser.ParseGoImportDeclaration(
			context.Background(),
			parseTree,
			nil,
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("malformed import should handle gracefully", func(t *testing.T) {
		sourceCode := `package main

import (`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser := NewGoImportParser()

		importDecls := parseTree.GetNodesByType("import_declaration")
		if len(importDecls) > 0 {
			result := parser.ParseGoImportDeclaration(
				context.Background(),
				parseTree,
				importDecls[0],
				outbound.SemanticExtractionOptions{},
				time.Now(),
			)
			// Should either return nil/empty slice or partial results, but not panic
			for _, importDecl := range result {
				assert.NotEmpty(t, importDecl.Path, "Import path should not be empty if parsed")
			}
		}
	})

	t.Run("empty import path should handle gracefully", func(t *testing.T) {
		sourceCode := `package main

import (
	""
	"fmt"
)`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser := NewGoImportParser()

		importDecls := parseTree.GetNodesByType("import_declaration")
		if len(importDecls) > 0 {
			result := parser.ParseGoImportDeclaration(
				context.Background(),
				parseTree,
				importDecls[0],
				outbound.SemanticExtractionOptions{},
				time.Now(),
			)
			// Should handle empty paths gracefully
			for _, importDecl := range result {
				// Either filter out empty paths or handle them appropriately
				if importDecl.Path == "" {
					assert.NotEmpty(t, importDecl.Content, "Empty path import should still have content")
				}
			}
		}
	})
}

// TestGoImportParser_ImportMetadata tests parsing of import metadata and positioning.
// This is a RED PHASE test that defines expected behavior for Go import metadata extraction.
func TestGoImportParser_ImportMetadata(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"        // Standard library
	"net/http"   // HTTP utilities
	"context"
	json "encoding/json" // JSON with alias
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		IncludeComments: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoImportDeclaration(context.Background(), parseTree, importDecls[0], options, time.Now())
	require.Len(t, result, 4)

	for i, importDecl := range result {
		// Validate positioning
		assert.Greater(
			t,
			importDecl.EndByte,
			importDecl.StartByte,
			"End byte should be greater than start byte for import %d",
			i,
		)

		// Validate content
		assert.NotEmpty(t, importDecl.Content, "Content should not be empty for import %d", i)

		// Validate timestamps and hashing
		assert.NotZero(t, importDecl.ExtractedAt, "ExtractedAt should be set for import %d", i)
		assert.NotEmpty(t, importDecl.Hash, "Hash should be generated for import %d", i)

		// Validate that content matches path and alias expectations
		assert.Contains(t, importDecl.Content, importDecl.Path, "Content should contain path for import %d", i)
		if importDecl.Alias != "" {
			assert.Contains(t, importDecl.Content, importDecl.Alias, "Content should contain alias for import %d", i)
		}
	}

	// Test specific imports
	fmtImport := result[0]
	assert.Equal(t, "fmt", fmtImport.Path)
	assert.Empty(t, fmtImport.Alias)

	jsonImport := result[3] // Last import with alias
	assert.Equal(t, "encoding/json", jsonImport.Path)
	assert.Equal(t, "json", jsonImport.Alias)
}

// TestGoImportParser_VisibilityAndFiltering tests import filtering (though imports are typically all processed).
// This is a RED PHASE test that defines expected behavior for import filtering.
func TestGoImportParser_VisibilityAndFiltering(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"
	"strings"
	_ "github.com/lib/pq"  // Blank import
	. "math"               // Dot import
	log "github.com/sirupsen/logrus"
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoImportParser()

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoImportDeclaration(context.Background(), parseTree, importDecls[0], options, time.Now())
	require.Len(t, result, 5, "All imports should be included regardless of visibility settings")

	// Verify all import types are captured
	importTypes := make(map[string]bool)
	for _, importDecl := range result {
		switch {
		case importDecl.Alias == "_":
			importTypes["blank"] = true
		case importDecl.IsWildcard:
			importTypes["wildcard"] = true
		case importDecl.Alias != "":
			importTypes["aliased"] = true
		default:
			importTypes["standard"] = true
		}
	}

	assert.True(t, importTypes["standard"], "Should include standard imports")
	assert.True(t, importTypes["blank"], "Should include blank imports")
	assert.True(t, importTypes["wildcard"], "Should include wildcard/dot imports")
	assert.True(t, importTypes["aliased"], "Should include aliased imports")
}

// Helper functions - these will fail in RED phase as expected

// GoImportParser represents the specialized Go import parser.
type GoImportParser struct {
	goParser *GoParser
}

// NewGoImportParser creates a new Go import parser.
func NewGoImportParser() *GoImportParser {
	goParser, _ := NewGoParser()
	return &GoImportParser{
		goParser: goParser,
	}
}

// ParseGoImportDeclaration delegates to the GoParser implementation.
func (p *GoImportParser) ParseGoImportDeclaration(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	_ outbound.SemanticExtractionOptions,
	_ time.Time,
) []outbound.ImportDeclaration {
	if p.goParser == nil || parseTree == nil || node == nil {
		return nil
	}
	return p.goParser.parseGoImportDeclaration(parseTree, node)
}

// ParseGoImportSpec delegates to the GoParser implementation.
func (p *GoImportParser) ParseGoImportSpec(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	_ outbound.SemanticExtractionOptions,
) *outbound.ImportDeclaration {
	if p.goParser == nil || parseTree == nil || node == nil {
		return nil
	}
	return p.goParser.parseGoImportSpec(parseTree, node)
}
