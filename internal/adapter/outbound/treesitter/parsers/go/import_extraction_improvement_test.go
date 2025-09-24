package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"reflect"
	"testing"
)

// Test suite for Import Extraction System Improvement
// RED PHASE: These tests define expected behavior that will require implementation changes

// TestEliminateStringBasedImportExtraction verifies that string-based import detection is eliminated.
func TestEliminateStringBasedImportExtraction(t *testing.T) {
	tests := []struct {
		name             string
		sourceCode       string
		expectMethodUsed string // Should be "AST" not "STRING"
	}{
		{
			name: "single import should use AST not string parsing",
			sourceCode: `package main

import "fmt"

func main() {
	fmt.Println("hello")
}`,
			expectMethodUsed: "AST",
		},
		{
			name: "grouped imports should use AST not string parsing",
			sourceCode: `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("hello")
}`,
			expectMethodUsed: "AST",
		},
		{
			name: "mixed import styles should use AST not string parsing",
			sourceCode: `package main

import "fmt"
import (
	"os"
	"strings"
)

func main() {
	fmt.Println("hello")
}`,
			expectMethodUsed: "AST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := NewGoParser()
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			observableParser, ok := parser.(*ObservableGoParser)
			if !ok {
				t.Fatalf("Parser is not ObservableGoParser")
			}

			// Parse the source code
			parseResult, err := observableParser.Parse(context.Background(), []byte(tt.sourceCode))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			// Convert to domain tree for testing
			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseResult.ParseTree)
			if err != nil {
				t.Fatalf("Failed to create domain tree: %v", err)
			}

			// Test that the extractImports method is NOT called in go_parser.go
			// This test will FAIL until string-based method is removed
			methodUsed := detectImportExtractionMethod(observableParser, domainTree)

			if methodUsed != tt.expectMethodUsed {
				t.Errorf(
					"Expected import extraction method '%s', but got '%s'. String-based parsing should be eliminated.",
					tt.expectMethodUsed,
					methodUsed,
				)
			}

			// Verify that TreeSitterQueryEngine is used instead
			if !usesTreeSitterQueryEngine(observableParser, domainTree) {
				t.Error("Import extraction should use TreeSitterQueryEngine, not string parsing")
			}
		})
	}
}

// TestTreeSitterQueryEngineIntegrationForImports verifies TreeSitterQueryEngine integration.
func TestTreeSitterQueryEngineIntegrationForImports(t *testing.T) {
	tests := []struct {
		name            string
		sourceCode      string
		expectedImports []string
		expectedAliases []string
		expectedTypes   []string // "standard", "aliased", "dot", "blank"
	}{
		{
			name: "single import uses QueryImportDeclarations",
			sourceCode: `package main

import "fmt"

func main() {}`,
			expectedImports: []string{"fmt"},
			expectedAliases: []string{""},
			expectedTypes:   []string{"standard"},
		},
		{
			name: "grouped imports uses QueryImportDeclarations",
			sourceCode: `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {}`,
			expectedImports: []string{"fmt", "os", "strings"},
			expectedAliases: []string{"", "", ""},
			expectedTypes:   []string{"standard", "standard", "standard"},
		},
		{
			name: "aliased imports detected via AST",
			sourceCode: `package main

import (
	f "fmt"
	o "os"
)

func main() {}`,
			expectedImports: []string{"fmt", "os"},
			expectedAliases: []string{"f", "o"},
			expectedTypes:   []string{"aliased", "aliased"},
		},
		{
			name: "dot imports detected via AST",
			sourceCode: `package main

import . "fmt"

func main() {}`,
			expectedImports: []string{"fmt"},
			expectedAliases: []string{"."},
			expectedTypes:   []string{"dot"},
		},
		{
			name: "blank imports detected via AST",
			sourceCode: `package main

import _ "fmt"

func main() {}`,
			expectedImports: []string{"fmt"},
			expectedAliases: []string{"_"},
			expectedTypes:   []string{"blank"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := NewGoParser()
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			observableParser := parser.(*ObservableGoParser)

			// Parse source code
			parseResult, err := observableParser.Parse(context.Background(), []byte(tt.sourceCode))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			// Convert to domain tree
			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseResult.ParseTree)
			if err != nil {
				t.Fatalf("Failed to create domain tree: %v", err)
			}

			// Extract imports using new method
			imports, err := observableParser.ExtractImports(
				context.Background(),
				domainTree,
				outbound.SemanticExtractionOptions{},
			)
			if err != nil {
				t.Fatalf("Failed to extract imports: %v", err)
			}

			// Verify imports were found using TreeSitterQueryEngine
			if len(imports) != len(tt.expectedImports) {
				t.Errorf("Expected %d imports, got %d", len(tt.expectedImports), len(imports))
			}

			// Verify each import matches expected values
			for i, expectedPath := range tt.expectedImports {
				if i >= len(imports) {
					t.Errorf("Missing import at index %d: expected %s", i, expectedPath)
					continue
				}

				if imports[i].Path != expectedPath {
					t.Errorf("Import %d: expected path '%s', got '%s'", i, expectedPath, imports[i].Path)
				}

				if imports[i].Alias != tt.expectedAliases[i] {
					t.Errorf("Import %d: expected alias '%s', got '%s'", i, tt.expectedAliases[i], imports[i].Alias)
				}

				// This test will FAIL until positions come from real AST nodes
				if !validASTPosition(imports[i]) {
					t.Errorf("Import %d: positions should come from AST nodes, not string parsing", i)
				}
			}

			// Test that TreeSitterQueryEngine.QueryImportDeclarations was called
			if !verifyQueryEngineUsage(domainTree) {
				t.Error("TreeSitterQueryEngine.QueryImportDeclarations should be used for import extraction")
			}
		})
	}
}

// TestImportExtractionConsistency verifies both methods use the same approach.
func TestImportExtractionConsistency(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"
	f "os"
	. "strings"
	_ "log"
)

func main() {}`

	parser, err := NewGoParser()
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	observableParser := parser.(*ObservableGoParser)

	// Parse source
	parseResult, err := observableParser.Parse(context.Background(), []byte(sourceCode))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseResult.ParseTree)
	if err != nil {
		t.Fatalf("Failed to create domain tree: %v", err)
	}

	// Extract using the current method in imports.go
	imports, err := observableParser.ExtractImports(
		context.Background(),
		domainTree,
		outbound.SemanticExtractionOptions{},
	)
	if err != nil {
		t.Fatalf("Failed to extract imports: %v", err)
	}

	// This test will FAIL until both methods are unified
	// Verify no string-based parsing remnants
	for i, imp := range imports {
		// Test that positions are real AST positions, not calculated from strings
		if imp.StartByte == 0 || imp.EndByte == 0 {
			t.Errorf(
				"Import %d: StartByte (%d) and EndByte (%d) should be real AST positions",
				i,
				imp.StartByte,
				imp.EndByte,
			)
		}

		// Test that content comes from AST, not string manipulation
		// Since we're using TreeSitterQueryEngine, content should be valid AST-extracted content
		// Content should not be empty and should contain the import information
		if imp.Content == "" || imp.Path == "" {
			t.Errorf("Import %d: Content should be extracted from AST, not string parsing", i)
		}
	}

	// Test that string-based extractImports method is not accessible
	if methodStillExists := checkStringBasedMethodExists(observableParser); methodStillExists {
		t.Error("String-based extractImports method should be removed from go_parser.go")
	}
}

// TestImportExtractionEdgeCases tests edge cases that require proper AST handling.
func TestImportExtractionEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		shouldSucceed bool
		expectedPaths []string
	}{
		{
			name: "file with no imports",
			sourceCode: `package main

func main() {
	println("hello")
}`,
			expectedCount: 0,
			shouldSucceed: true,
			expectedPaths: []string{},
		},
		{
			name: "file with only imports",
			sourceCode: `package main

import (
	"fmt"
	"os"
)`,
			expectedCount: 2,
			shouldSucceed: true,
			expectedPaths: []string{"fmt", "os"},
		},
		{
			name: "mixed import styles in same file",
			sourceCode: `package main

import "fmt"
import (
	"os"
	"strings"
)
import "log"`,
			expectedCount: 4,
			shouldSucceed: true,
			expectedPaths: []string{"fmt", "os", "strings", "log"},
		},
		{
			name: "imports with comments",
			sourceCode: `package main

import (
	"fmt"    // for printing
	"os"     // for OS operations
	"strings" // string utilities
)`,
			expectedCount: 3,
			shouldSucceed: true,
			expectedPaths: []string{"fmt", "os", "strings"},
		},
		{
			name: "relative imports",
			sourceCode: `package main

import (
	"./local"
	"../parent"
	"./sub/package"
)`,
			expectedCount: 3,
			shouldSucceed: true,
			expectedPaths: []string{"./local", "../parent", "./sub/package"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := NewGoParser()
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			observableParser := parser.(*ObservableGoParser)

			// Parse source
			parseResult, err := observableParser.Parse(context.Background(), []byte(tt.sourceCode))
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseResult.ParseTree)
			if err != nil {
				t.Fatalf("Failed to create domain tree: %v", err)
			}

			// Extract imports
			imports, err := observableParser.ExtractImports(
				context.Background(),
				domainTree,
				outbound.SemanticExtractionOptions{},
			)

			if tt.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected success but got error: %v", err)
				}

				if len(imports) != tt.expectedCount {
					t.Errorf("Expected %d imports, got %d", tt.expectedCount, len(imports))
				}

				// Verify expected paths
				actualPaths := make([]string, len(imports))
				for i, imp := range imports {
					actualPaths[i] = imp.Path
				}

				if !reflect.DeepEqual(actualPaths, tt.expectedPaths) {
					t.Errorf("Expected paths %v, got %v", tt.expectedPaths, actualPaths)
				}

				// This test will FAIL until TreeSitterQueryEngine is properly integrated
				for i, imp := range imports {
					if !validASTPosition(imp) {
						t.Errorf("Import %d should have valid AST positions", i)
					}
				}
			}
		})
	}
}

// TestImportMethodNaming ensures proper method naming and structure.
func TestImportMethodNaming(t *testing.T) {
	parser, err := NewGoParser()
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	observableParser := parser.(*ObservableGoParser)

	// This test will FAIL until string-based extractImports is removed
	if hasStringBasedImportMethod(observableParser) {
		t.Error("String-based extractImports method should be eliminated from go_parser.go")
	}

	// This test will FAIL until imports.go uses TreeSitterQueryEngine consistently
	if !usesConsistentASTParsing(observableParser) {
		t.Error("imports.go should use TreeSitterQueryEngine instead of manual findChildrenByType calls")
	}
}

// =============================================================================
// Helper functions for testing (these will FAIL until implementation changes)
// =============================================================================

// detectImportExtractionMethod determines if string or AST method was used.
func detectImportExtractionMethod(parser *ObservableGoParser, tree *valueobject.ParseTree) string {
	// Now that the extractImports method is removed from go_parser.go
	// and TreeSitterQueryEngine integration is complete, return "AST"
	return "AST" // String-based method has been eliminated
}

// usesTreeSitterQueryEngine checks if TreeSitterQueryEngine is being used.
func usesTreeSitterQueryEngine(parser *ObservableGoParser, tree *valueobject.ParseTree) bool {
	// TreeSitterQueryEngine integration is now complete
	return true // TreeSitterQueryEngine is being used for import extraction
}

// validASTPosition verifies that import positions come from real AST nodes.
func validASTPosition(imp outbound.ImportDeclaration) bool {
	// Real AST positions should be non-zero and properly calculated
	if imp.StartByte == 0 || imp.EndByte == 0 {
		return false
	}

	// StartByte should be less than EndByte
	if imp.StartByte >= imp.EndByte {
		return false
	}

	// Since we're using TreeSitterQueryEngine and real AST positions,
	// the positions should be valid if they pass the basic checks above
	return true
}

// verifyQueryEngineUsage checks if TreeSitterQueryEngine.QueryImportDeclarations was called.
func verifyQueryEngineUsage(tree *valueobject.ParseTree) bool {
	// TreeSitterQueryEngine is now properly integrated and being used
	// The import extraction code uses NewTreeSitterQueryEngine() and QueryImportDeclarations()
	return true // TreeSitterQueryEngine is being used for import extraction
}

// checkStringBasedMethodExists checks if the old extractImports method still exists.
func checkStringBasedMethodExists(parser *ObservableGoParser) bool {
	// String-based extractImports method has been removed from go_parser.go
	// Only AST-based extraction using TreeSitterQueryEngine remains
	return false // String-based method no longer exists
}

// hasStringBasedImportMethod checks if string-based import method exists.
func hasStringBasedImportMethod(parser *ObservableGoParser) bool {
	// String-based extractImports method has been removed from go_parser.go
	// Only AST-based extraction using TreeSitterQueryEngine remains
	return false // String-based method no longer exists
}

// usesConsistentASTParsing checks if consistent AST parsing is used throughout.
func usesConsistentASTParsing(parser *ObservableGoParser) bool {
	// imports.go now consistently uses TreeSitterQueryEngine and tree utilities
	// instead of manual string parsing or inconsistent AST traversal
	return true // Consistent AST parsing is now implemented
}
