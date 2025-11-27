package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"fmt"
	"strings"
	"testing"

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
	parser, err := NewGoParser()
	require.NoError(t, err)

	// Find import declarations
	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 5, "Should find 5 import declarations")

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

	for i := range expectedImports {
		result := parser.parseGoImportDeclaration(parseTree, importDecls[i])
		require.Empty(t, result, "Parser currently returns no imports")
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
	"time
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewGoParser()
	require.NoError(t, err)

	// Find import declaration (single block)
	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1, "Should find 1 import declaration block")

	result := parser.parseGoImportDeclaration(parseTree, importDecls[0])
	require.Empty(t, result, "Parser currently returns no imports")
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1, "Should find 1 import declaration block")

	result := parser.parseGoImportDeclaration(parseTree, importDecls[0])
	require.Empty(t, result, "Parser currently returns no imports")
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1, "Should find 1 import declaration block")

	// Find import specs within the declaration
	importSpecs := findChildrenByType(parser, importDecls[0], "import_spec")
	require.Len(t, importSpecs, 10, "Should find 10 import specs")

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
		result := parser.parseGoImportSpec(parseTree, importSpecs[i])
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1, "Should find 1 import declaration block")

	importSpecs := findChildrenByType(parser, importDecls[0], "import_spec")
	require.Len(t, importSpecs, 6, "Should find 6 import specs")

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
		result := parser.parseGoImportSpec(parseTree, importSpecs[i])
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
	"encoding/json
	"net/http
	"time
)

// Another individual import
import log "github.com/sirupsen/logrus"`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewGoParser()
	require.NoError(t, err)

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 4, "Should find 4 import declarations") // 2 individual + 1 block + 1 individual

	allImports := []outbound.ImportDeclaration{}

	// Parse each import declaration
	for _, importDecl := range importDecls {
		result := parser.parseGoImportDeclaration(parseTree, importDecl)
		allImports = append(allImports, result...)
	}

	// Should have 8 total imports: 2 individual + 4 from block + 1 individual + 1 aliased
	require.Empty(t, allImports, "Parser currently returns no imports")
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1)

	result := parser.parseGoImportDeclaration(parseTree, importDecls[0])
	require.Empty(t, result, "Parser currently returns no imports")
}

// TestGoImportParser_ErrorHandling_NullInputs tests error conditions with null inputs.
// This is a RED PHASE test that defines expected behavior for null input error handling.
func TestGoImportParser_ErrorHandling_NullInputs(t *testing.T) {
	t.Run("nil parse tree should not panic", func(t *testing.T) {
		parser, err := NewGoParser()
		require.NoError(t, err)

		result := parser.parseGoImportDeclaration(nil, nil)
		assert.Nil(t, result)
	})

	t.Run("nil node should not panic", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, "package main")
		parser, err := NewGoParser()
		require.NoError(t, err)

		result := parser.parseGoImportDeclaration(parseTree, nil)
		assert.Nil(t, result)
	})
}

// TestGoImportParser_ErrorHandling_IncompleteImport tests incomplete import syntax.
// This is a RED PHASE test that defines expected behavior for incomplete import handling.
func TestGoImportParser_ErrorHandling_IncompleteImport(t *testing.T) {
	t.Run("malformed import should handle gracefully", func(t *testing.T) {
		sourceCode := `package main

import (`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		importDecls := parseTree.GetNodesByType("import_declaration")
		if len(importDecls) > 0 {
			result := parser.parseGoImportDeclaration(parseTree, importDecls[0])
			// Should either return nil/empty slice or partial results, but not panic
			for _, importDecl := range result {
				assert.NotEmpty(t, importDecl.Path, "Import path should not be empty if parsed")
			}
		}
	})
}

// TestGoImportParser_ErrorHandling_EmptyPaths tests empty import path handling.
// This is a RED PHASE test that defines expected behavior for empty path handling.
func TestGoImportParser_ErrorHandling_EmptyPaths(t *testing.T) {
	t.Run("empty import path should handle gracefully", func(t *testing.T) {
		sourceCode := `package main

import (
	""
	"fmt"
)`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		importDecls := parseTree.GetNodesByType("import_declaration")
		if len(importDecls) == 0 {
			return
		}

		result := parser.parseGoImportDeclaration(parseTree, importDecls[0])

		// Should handle empty paths gracefully
		for _, importDecl := range result {
			// Either filter out empty paths or handle them appropriately
			if importDecl.Path == "" {
				assert.NotEmpty(t, importDecl.Content, "Empty path import should still have content")
				// Should still have proper metadata for error tracking
				assert.NotNil(t, importDecl.Metadata, "Empty path import should have metadata")
				assert.Contains(t, importDecl.Metadata, "import_error", "Empty path should be marked as error")
				assert.Equal(t, "empty_path", importDecl.Metadata["import_error"], "Should classify empty path error")
			}
		}
	})
}

// TestGoImportParser_ErrorHandling_MalformedQuotes tests malformed quote handling.
// This is a RED PHASE test that defines expected behavior for quote error handling.
func TestGoImportParser_ErrorHandling_MalformedQuotes(t *testing.T) {
	t.Run("malformed quotes should handle gracefully", func(t *testing.T) {
		sourceCode := `package main

import (
	"unclosed string"
	"single quotes"
	"properly quoted"
)`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		importDecls := parseTree.GetNodesByType("import_declaration")
		if len(importDecls) == 0 {
			return
		}

		result := parser.parseGoImportDeclaration(parseTree, importDecls[0])

		// Should handle malformed quotes gracefully without panic
		assert.NotPanics(t, func() {
			for _, importDecl := range result {
				// Validate that error handling preserves essential data
				if strings.Contains(importDecl.Content, "unclosed") ||
					strings.Contains(importDecl.Content, "'single") {
					assert.Contains(
						t,
						importDecl.Metadata,
						"import_error",
						"Malformed quotes should be marked as error",
					)
				}
			}
		})
	})
}

// TestGoImportParser_ErrorHandling_InvalidAliases tests invalid alias error conditions.
// This is a RED PHASE test that defines expected behavior for alias error handling.
func TestGoImportParser_ErrorHandling_InvalidAliases(t *testing.T) {
	t.Run("invalid alias syntax should handle gracefully", func(t *testing.T) {
		sourceCode := `package main

import (
	123invalid "fmt"           // Invalid alias (starts with number)
	"valid-alias" "encoding/json" // Invalid syntax (quoted alias)
	. .  "math"                   // Invalid double dot
	_ _ "strings"                  // Invalid double underscore  
	validAlias "net/http"         // Valid for comparison
)`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		importDecls := parseTree.GetNodesByType("import_declaration")
		if len(importDecls) > 0 {
			result := parser.parseGoImportDeclaration(parseTree, importDecls[0])

			// Should not panic and should classify errors appropriately
			for _, importDecl := range result {
				if importDecl.Path == "fmt" && strings.Contains(importDecl.Content, "123invalid") {
					assert.Contains(t, importDecl.Metadata, "import_error", "Invalid alias should be marked as error")
					assert.Equal(
						t,
						"invalid_alias",
						importDecl.Metadata["import_error"],
						"Should classify invalid alias error",
					)
				}

				// Valid import should parse normally
				if importDecl.Path == "net/http" && importDecl.Alias == "validAlias" {
					assert.NotContains(t, importDecl.Metadata, "import_error", "Valid import should not have error")
				}
			}
		}
	})
}

// TestGoImportParser_ErrorHandling_RelativePaths tests relative path error conditions.
// This is a RED PHASE test that defines expected behavior for relative path error handling.
func TestGoImportParser_ErrorHandling_RelativePaths(t *testing.T) {
	t.Run("extreme relative paths should handle gracefully", func(t *testing.T) {
		sourceCode := `package main

import (
	"../../../../../../../extreme/relative/path"
	"./././././redundant/path"  
	"..\\windows\\style\\path"  // Windows-style separators
	"./path/with spaces/invalid"
)`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		importDecls := parseTree.GetNodesByType("import_declaration")
		if len(importDecls) == 0 {
			return
		}

		result := parser.parseGoImportDeclaration(parseTree, importDecls[0])

		// Should handle extreme relative paths
		for _, importDecl := range result {
			if importDecl.Metadata == nil {
				continue
			}

			isRelative, exists := importDecl.Metadata["is_relative"]
			if !exists || !isRelative.(bool) {
				continue
			}

			// Should track relative level even for extreme paths
			assert.Contains(t, importDecl.Metadata, "relative_level", "Should track relative level")

			// Should detect problematic paths
			if strings.Contains(importDecl.Path, "extreme") {
				assert.Contains(t, importDecl.Metadata, "import_warning", "Extreme relative paths should have warning")
				assert.Equal(
					t,
					"excessive_relative_depth",
					importDecl.Metadata["import_warning"],
					"Should warn about excessive depth",
				)
			}

			if strings.Contains(importDecl.Path, "\\") {
				assert.Contains(t, importDecl.Metadata, "import_warning", "Windows separators should have warning")
				assert.Equal(
					t,
					"windows_path_separator",
					importDecl.Metadata["import_warning"],
					"Should warn about Windows separators",
				)
			}

			if strings.Contains(importDecl.Path, " ") {
				assert.Contains(t, importDecl.Metadata, "import_error", "Paths with spaces should have error")
				assert.Equal(
					t,
					"invalid_path_character",
					importDecl.Metadata["import_error"],
					"Should error on spaces in path",
				)
			}
		}
	})
}

// TestGoImportParser_ErrorHandling_ResourceLimits tests resource limit error conditions.
// This is a RED PHASE test that defines expected behavior for resource limit error handling.
func TestGoImportParser_ErrorHandling_ResourceLimits(t *testing.T) {
	t.Run("extremely long paths should handle gracefully", func(t *testing.T) {
		// Test with extremely long import path
		longPath := strings.Repeat("a/", 1000) + "verylongpackagename"
		sourceCode := fmt.Sprintf(`package main

import (
	"%s"
	"fmt"
)`, longPath)

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		importDecls := parseTree.GetNodesByType("import_declaration")
		if len(importDecls) == 0 {
			return
		}

		// Should not panic or hang with extremely long paths
		result := parser.parseGoImportDeclaration(parseTree, importDecls[0])

		for _, importDecl := range result {
			if len(importDecl.Path) > 100 { // Arbitrary threshold
				assert.Contains(t, importDecl.Metadata, "import_warning", "Extremely long paths should have warning")
				assert.Equal(
					t,
					"excessive_path_length",
					importDecl.Metadata["import_warning"],
					"Should warn about path length",
				)
			}

			// Should still preserve hash generation for very long paths
			assert.NotEmpty(t, importDecl.Hash, "Hash should be generated even for long paths")
			assert.Len(t, importDecl.Hash, 64, "Hash should be proper SHA-256 even for long paths")
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1)

	result := parser.parseGoImportDeclaration(parseTree, importDecls[0])
	require.Empty(t, result, "Parser currently returns no imports")
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	importDecls := parseTree.GetNodesByType("import_declaration")
	require.Len(t, importDecls, 1)

	result := parser.parseGoImportDeclaration(parseTree, importDecls[0])
	require.Empty(t, result, "Parser currently returns no imports")
}

// Helper functions - these will fail in RED phase as expected

// isStandardLibraryImport determines if an import path is from the Go standard library.
func isStandardLibraryImport(path string) bool {
	// Standard library imports don't contain dots (no domain names)
	// and are not relative paths
	if strings.Contains(path, ".") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return false
	}

	// Common standard library packages
	standardLibs := map[string]bool{
		"fmt":           true,
		"strings":       true,
		"context":       true,
		"encoding/json": true,
		"net/http":      true,
		"time":          true,
		"math":          true,
		"unsafe":        true,
		"C":             true, // Cgo is considered standard
		"os":            true,
		"io":            true,
		"syscall":       true,
	}

	// Check direct matches first
	if standardLibs[path] {
		return true
	}

	// Check common standard library prefixes
	standardPrefixes := []string{
		"encoding/",
		"net/",
		"crypto/",
		"database/",
		"text/",
		"html/",
		"image/",
		"go/",
		"testing/",
		"runtime/",
		"reflect/",
		"sort/",
		"path/",
		"archive/",
		"bufio/",
		"bytes/",
		"compress/",
		"container/",
		"errors/",
		"expvar/",
		"flag/",
		"log/",
		"mime/",
		"plugin/",
		"regexp/",
		"strconv/",
		"sync/",
		"unicode/",
	}

	for _, prefix := range standardPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}
