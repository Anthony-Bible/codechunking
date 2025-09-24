package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestASTBasedPackageValidation tests that package validation uses AST-based validation
// instead of string pattern matching like strings.Contains("package ").
func TestASTBasedPackageValidation(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedValid bool
		expectedError string
		description   string
	}{
		{
			name:          "valid package declaration detected via AST",
			sourceCode:    "package main\n\nfunc main() {}",
			expectedValid: true,
			description:   "Should use QueryPackageDeclarations to detect valid package",
		},
		{
			name:          "missing package declaration detected via AST",
			sourceCode:    "func main() {\n    fmt.Println(\"hello\")\n}",
			expectedValid: false,
			expectedError: "missing package declaration",
			description:   "Should use QueryPackageDeclarations to detect missing package, not strings.Contains",
		},
		{
			name:          "invalid package syntax detected via tree-sitter errors",
			sourceCode:    "package // missing package name\n\nfunc main() {}",
			expectedValid: false,
			expectedError: "invalid package declaration",
			description:   "Should use tree-sitter error nodes instead of strings.Contains for syntax errors",
		},
		{
			name:          "package declaration with comments handled properly",
			sourceCode:    "// Package comment\npackage myapp\n\n// Function comment\nfunc main() {}",
			expectedValid: true,
			description:   "Should use AST to distinguish between comments and actual package declarations",
		},
		{
			name:          "multiple package declarations detected as error",
			sourceCode:    "package first\npackage second\n\nfunc main() {}",
			expectedValid: false,
			expectedError: "multiple package declarations",
			description:   "Should use QueryPackageDeclarations count to detect multiple packages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test expects the validation to use AST-based methods:
			// 1. QueryPackageDeclarations() instead of strings.Contains("package ")
			// 2. HasSyntaxErrors() for syntax validation
			// 3. Error node detection for malformed syntax
			err := parser.validateModuleSyntaxWithAST(tt.sourceCode)

			if tt.expectedValid {
				assert.NoError(t, err, tt.description)
			} else {
				assert.Error(t, err, tt.description)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError, "Error message should match expected content")
				}
			}
		})
	}
}

// TestASTBasedSnippetDetection tests that snippet detection uses AST node analysis
// instead of line-by-line string parsing.
func TestASTBasedSnippetDetection(t *testing.T) {
	tests := []struct {
		name        string
		sourceCode  string
		isSnippet   bool
		description string
	}{
		{
			name: "type-only snippet detected via AST",
			sourceCode: `type Person struct {
				Name string
				Age  int
			}`,
			isSnippet:   true,
			description: "Should use QueryTypeDeclarations to identify type-only snippets, not line parsing",
		},
		{
			name: "interface snippet detected via AST",
			sourceCode: `type Writer interface {
				Write([]byte) (int, error)
			}`,
			isSnippet:   true,
			description: "Should use AST to detect interface declarations as snippets",
		},
		{
			name: "function code detected as non-snippet via AST",
			sourceCode: `func Calculate(a, b int) int {
				return a + b
			}`,
			isSnippet:   false,
			description: "Should use QueryFunctionDeclarations to detect functions require package",
		},
		{
			name: "mixed declarations detected correctly via AST",
			sourceCode: `type User struct {
				ID   int
				Name string
			}

			func (u User) String() string {
				return u.Name
			}`,
			isSnippet:   false,
			description: "Should use AST queries to detect mixed content requires package",
		},
		{
			name:        "empty source handled as snippet",
			sourceCode:  ``,
			isSnippet:   true,
			description: "Empty source should be treated as snippet",
		},
		{
			name: "comments only treated as snippet",
			sourceCode: `// This is just a comment
			// Another comment`,
			isSnippet:   true,
			description: "Should use AST to detect comment-only content as snippet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test expects snippet detection to use:
			// 1. QueryTypeDeclarations() for type analysis
			// 2. QueryFunctionDeclarations() for function analysis
			// 3. QueryVariableDeclarations() and QueryConstDeclarations() for comprehensive analysis
			// 4. AST node traversal instead of line-by-line string parsing
			isSnippet := parser.isPartialSnippetWithAST(tt.sourceCode)

			assert.Equal(t, tt.isSnippet, isSnippet, tt.description)
		})
	}
}

// TestTreeSitterErrorDetection tests that syntax validation uses tree-sitter error nodes
// instead of manual string pattern matching.
func TestTreeSitterErrorDetection(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		hasErrors     bool
		expectedError string
		description   string
	}{
		{
			name:        "valid Go code has no errors",
			sourceCode:  "package main\n\nfunc main() {\n    fmt.Println(\"hello\")\n}",
			hasErrors:   false,
			description: "Should use HasSyntaxErrors() to validate correct syntax",
		},
		{
			name:          "unclosed brace detected as error",
			sourceCode:    "package main\n\nfunc main() {\n    fmt.Println(\"hello\")",
			hasErrors:     true,
			expectedError: "syntax error",
			description:   "Should use tree-sitter error nodes to detect syntax errors",
		},
		{
			name:          "invalid package name detected as error",
			sourceCode:    "package 123invalid\n\nfunc main() {}",
			hasErrors:     true,
			expectedError: "syntax error",
			description:   "Should use tree-sitter parsing to detect invalid identifiers",
		},
		{
			name:          "incomplete function detected as error",
			sourceCode:    "package main\n\nfunc incomplete(",
			hasErrors:     true,
			expectedError: "syntax error",
			description:   "Should use error node detection for incomplete constructs",
		},
		{
			name:          "malformed struct detected as error",
			sourceCode:    "package main\n\ntype BadStruct struct {\n    field string",
			hasErrors:     true,
			expectedError: "syntax error",
			description:   "Should detect malformed structs using tree-sitter error nodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test expects syntax validation to use:
			// 1. HasSyntaxErrors() from the parse tree
			// 2. Error node detection instead of string patterns
			// 3. Tree-sitter's grammar-based error reporting
			err := parser.validateSyntaxWithTreeSitter(tt.sourceCode)

			if tt.hasErrors {
				assert.Error(t, err, tt.description)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestTreeSitterQueryEngineIntegrationForValidation tests that validation functions integrate
// with TreeSitterQueryEngine for consistent AST-based analysis.
func TestTreeSitterQueryEngineIntegrationForValidation(t *testing.T) {
	tests := []struct {
		name        string
		sourceCode  string
		description string
	}{
		{
			name: "query engine used for package validation",
			sourceCode: `package main

import "fmt"

func main() {
	fmt.Println("hello")
}`,
			description: "Should use TreeSitterQueryEngine.QueryPackageDeclarations",
		},
		{
			name: "query engine used for function detection",
			sourceCode: `func helper() int {
	return 42
}`,
			description: "Should use TreeSitterQueryEngine.QueryFunctionDeclarations",
		},
		{
			name: "query engine used for type analysis",
			sourceCode: `type Config struct {
	Host string
	Port int
}`,
			description: "Should use TreeSitterQueryEngine.QueryTypeDeclarations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create parse tree for AST validation
			parseTree := createParseTreeForValidation(t, tt.sourceCode)

			parser := &GoParser{}

			// This test expects validation to use TreeSitterQueryEngine methods:
			// 1. All validation should go through the query engine
			// 2. No direct string parsing should remain
			// 3. AST structure should drive validation decisions
			result := parser.validateWithQueryEngine(parseTree)

			// Validate that the query engine was used by checking that
			// the result contains AST-based analysis information
			assert.NotNil(t, result, tt.description)
			assert.True(t, result.UsedASTAnalysis, "Should use AST-based analysis via query engine")
			assert.False(t, result.UsedStringParsing, "Should not use string parsing")
		})
	}
}

// TestValidationReplacesStringPatterns tests that specific string pattern matching
// is replaced with AST-based validation.
func TestValidationReplacesStringPatterns(t *testing.T) {
	tests := []struct {
		name           string
		sourceCode     string
		forbiddenCalls []string
		requiredCalls  []string
		description    string
	}{
		{
			name:       "package validation uses AST not strings",
			sourceCode: "package main\n\nfunc main() {}",
			forbiddenCalls: []string{
				"strings.Contains(source, \"package \")",
				"strings.Contains(source, \"package // missing package name\")",
			},
			requiredCalls: []string{
				"queryEngine.QueryPackageDeclarations",
				"parseTree.HasSyntaxErrors",
			},
			description: "Package validation must use AST methods instead of string patterns",
		},
		{
			name:       "function detection uses AST not strings",
			sourceCode: "func main() {\n    return\n}",
			forbiddenCalls: []string{
				"strings.Contains(source, \"func main(\")",
				"strings.Contains(source, \"func \")",
			},
			requiredCalls: []string{
				"queryEngine.QueryFunctionDeclarations",
			},
			description: "Function detection must use AST queries instead of string searches",
		},
		{
			name: "snippet detection uses AST not line parsing",
			sourceCode: `type Person struct {
				Name string
			}`,
			forbiddenCalls: []string{
				"strings.Split(trimmed, \"\\n\")",
				"strings.HasPrefix(line, \"type \")",
				"strings.Contains(line, \"struct {\")",
			},
			requiredCalls: []string{
				"queryEngine.QueryTypeDeclarations",
				"queryEngine.QueryFunctionDeclarations",
			},
			description: "Snippet detection must use AST node analysis instead of line-by-line parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test validates that the implementation has moved away from
			// string pattern matching to proper AST-based validation
			validationResult := parser.performValidationAnalysis(tt.sourceCode)

			// Check that forbidden string-based methods are not used
			for _, forbidden := range tt.forbiddenCalls {
				assert.False(t, validationResult.CallsUsed[forbidden],
					"Should not use forbidden string-based call: %s", forbidden)
			}

			// Check that required AST-based methods are used
			for _, required := range tt.requiredCalls {
				assert.True(t, validationResult.CallsUsed[required],
					"Should use required AST-based call: %s", required)
			}
		})
	}
}

// Helper functions that the tests expect to exist but don't yet

// createParseTreeForValidation creates a parse tree for validation testing.
func createParseTreeForValidation(t *testing.T, sourceCode string) *valueobject.ParseTree {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	parseResult, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseResult.ParseTree)
	require.NoError(t, err)

	return domainTree
}

// ValidationResult is now implemented in go_parser.go
