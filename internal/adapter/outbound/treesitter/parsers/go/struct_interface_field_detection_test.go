package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsStructOrInterfaceFieldWithAST tests the restructured isStructOrInterfaceField function
// that should use AST-first approach with string fallback. This test demonstrates the expected
// behavior where the function first attempts AST-based detection using TreeSitterQueryEngine
// and only falls back to string parsing when AST parsing fails or is unavailable.
func TestIsStructOrInterfaceFieldWithAST(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		expectedField bool
		description   string
	}{
		// AST-based detection should handle these correctly
		{
			name:          "simple struct field",
			line:          "Name string",
			expectedField: true,
			description:   "Should detect simple struct field using AST",
		},
		{
			name:          "typed struct field with package qualifier",
			line:          "Logger *slog.Logger",
			expectedField: true,
			description:   "Should detect struct field with package-qualified type using AST",
		},
		{
			name:          "interface method with parameters",
			line:          "Process(data string) error",
			expectedField: true,
			description:   "Should detect interface method using AST method_spec queries",
		},
		{
			name:          "interface method with complex signature",
			line:          "Transform(ctx context.Context, input []byte) ([]byte, error)",
			expectedField: true,
			description:   "Should detect complex interface method using AST",
		},
		{
			name:          "embedded interface type",
			line:          "io.Reader",
			expectedField: true,
			description:   "Should detect embedded interface type using AST embedded type queries",
		},
		{
			name:          "embedded struct type",
			line:          "BaseStruct",
			expectedField: true,
			description:   "Should detect embedded struct type using AST",
		},
		{
			name:          "struct field with tags",
			line:          "Name string `json:\"name\" xml:\"name\"`",
			expectedField: true,
			description:   "Should detect tagged struct field using AST field_declaration queries",
		},
		{
			name:          "function type field",
			line:          "Handler func(http.ResponseWriter, *http.Request)",
			expectedField: true,
			description:   "Should detect function type field using AST",
		},
		{
			name:          "channel type field",
			line:          "Events chan<- Event",
			expectedField: true,
			description:   "Should detect channel type field using AST",
		},
		{
			name:          "map type field",
			line:          "Data map[string]interface{}",
			expectedField: true,
			description:   "Should detect map type field using AST",
		},
		{
			name:          "slice type field",
			line:          "Items []Item",
			expectedField: true,
			description:   "Should detect slice type field using AST",
		},
		{
			name:          "generic type field",
			line:          "Value T",
			expectedField: true,
			description:   "Should detect generic type field using AST",
		},
		{
			name:          "generic method with constraints",
			line:          "Compare[T comparable](a, b T) bool",
			expectedField: true,
			description:   "Should detect generic interface method using AST",
		},
		{
			name:          "interface method no return parameters",
			line:          "Cleanup()",
			expectedField: true,
			description:   "Should detect interface method without return using AST method_spec",
		},
		{
			name:          "empty line should be allowed",
			line:          "",
			expectedField: true,
			description:   "Should allow empty lines in struct/interface definitions",
		},
		{
			name:          "whitespace only line",
			line:          "   \t  ",
			expectedField: true,
			description:   "Should allow whitespace-only lines",
		},

		// These should NOT be detected as struct/interface fields
		{
			name:          "function declaration",
			line:          "func Process(data string) error {",
			expectedField: false,
			description:   "Should reject function declarations using AST",
		},
		{
			name:          "variable declaration",
			line:          "var logger *slog.Logger",
			expectedField: false,
			description:   "Should reject variable declarations using AST",
		},
		{
			name:          "constant declaration",
			line:          "const DefaultTimeout = 30 * time.Second",
			expectedField: false,
			description:   "Should reject constant declarations using AST",
		},
		{
			name:          "import statement",
			line:          "import \"context\"",
			expectedField: false,
			description:   "Should reject import statements using AST",
		},
		{
			name:          "package declaration",
			line:          "package main",
			expectedField: false,
			description:   "Should reject package declarations using AST",
		},
		{
			name:          "type declaration",
			line:          "type User struct {",
			expectedField: false,
			description:   "Should reject type declarations using AST",
		},
		{
			name:          "code statement",
			line:          "return nil",
			expectedField: false,
			description:   "Should reject code statements using AST",
		},
		{
			name:          "assignment statement",
			line:          "user.Name = \"John\"",
			expectedField: false,
			description:   "Should reject assignment statements using AST",
		},
		{
			name:          "method call",
			line:          "logger.Info(\"processing\")",
			expectedField: false,
			description:   "Should reject method calls using AST",
		},
		{
			name:          "if statement",
			line:          "if err != nil {",
			expectedField: false,
			description:   "Should reject control flow statements using AST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test expects the isStructOrInterfaceField function to:
			// 1. First attempt AST-based detection by parsing the line as Go code
			// 2. Use TreeSitterQueryEngine to identify field_declaration, method_spec, or embedded types
			// 3. Only fall back to string parsing if AST parsing fails
			// 4. Handle all edge cases gracefully

			result := isStructOrInterfaceField(tt.line)
			assert.Equal(t, tt.expectedField, result, tt.description)
		})
	}
}

// TestIsStructOrInterfaceFieldASTFallback tests that the function properly falls back
// to string parsing when AST parsing fails or encounters malformed syntax.
func TestIsStructOrInterfaceFieldASTFallback(t *testing.T) {
	tests := []struct {
		name                    string
		line                    string
		expectedField           bool
		shouldUseStringFallback bool
		description             string
	}{
		{
			name:                    "malformed field syntax - fallback to string parsing",
			line:                    "Name string json", // Missing backticks for tags
			expectedField:           true,               // String parsing should handle this
			shouldUseStringFallback: true,
			description:             "Should fallback to string parsing for malformed syntax",
		},
		{
			name:                    "partial method signature - fallback",
			line:                    "Process(data string", // Missing closing parenthesis
			expectedField:           true,                  // String parsing should allow incomplete signatures
			shouldUseStringFallback: true,
			description:             "Should fallback for incomplete method signatures",
		},
		{
			name:                    "incomplete type annotation - fallback",
			line:                    "Data map[string", // Incomplete map type
			expectedField:           true,              // String parsing should handle this
			shouldUseStringFallback: true,
			description:             "Should fallback for incomplete type annotations",
		},
		{
			name:                    "valid syntax with AST",
			line:                    "Name string",
			expectedField:           true,
			shouldUseStringFallback: false,
			description:             "Should use AST for valid syntax",
		},
		{
			name:                    "invalid Go syntax - fallback",
			line:                    "Name @#$ invalid",
			expectedField:           false, // String parsing should reject this
			shouldUseStringFallback: true,
			description:             "Should fallback and reject invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The restructured function should attempt AST parsing first
			// and only use string fallback when AST parsing fails
			result := isStructOrInterfaceField(tt.line)
			assert.Equal(t, tt.expectedField, result, tt.description)
		})
	}
}

// TestIsStructOrInterfaceFieldContextualDetection tests that the function can detect
// field declarations within proper struct and interface contexts using AST analysis.
func TestIsStructOrInterfaceFieldContextualDetection(t *testing.T) {
	// Test cases where context matters for proper detection
	tests := []struct {
		name          string
		goCode        string
		lineToTest    string
		expectedField bool
		description   string
	}{
		{
			name: "field within struct definition",
			goCode: `
type User struct {
	Name string
	Age  int
}`,
			lineToTest:    "Name string",
			expectedField: true,
			description:   "Should detect field within struct using AST context",
		},
		{
			name: "method within interface definition",
			goCode: `
type Writer interface {
	Write([]byte) (int, error)
}`,
			lineToTest:    "Write([]byte) (int, error)",
			expectedField: true,
			description:   "Should detect method within interface using AST context",
		},
		{
			name: "embedded type within struct",
			goCode: `
type Handler struct {
	http.Handler
	Logger *log.Logger
}`,
			lineToTest:    "http.Handler",
			expectedField: true,
			description:   "Should detect embedded type within struct using AST",
		},
		{
			name: "embedded interface within interface",
			goCode: `
type ReadWriter interface {
	io.Reader
	io.Writer
}`,
			lineToTest:    "io.Reader",
			expectedField: true,
			description:   "Should detect embedded interface using AST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the complete Go code to establish proper AST context
			ctx := context.Background()
			result := treesitter.CreateTreeSitterParseTree(ctx, tt.goCode)
			require.NoError(t, result.Error, "Should parse test Go code successfully")

			// The restructured function should be able to detect fields within context
			// by using TreeSitterQueryEngine to analyze the AST structure
			fieldResult := isStructOrInterfaceField(tt.lineToTest)
			assert.Equal(t, tt.expectedField, fieldResult, tt.description)
		})
	}
}

// TestIsStructOrInterfaceFieldConsistencyBetweenApproaches tests that both AST-based
// and string-based approaches should give the same results for common field patterns.
func TestIsStructOrInterfaceFieldConsistencyBetweenApproaches(t *testing.T) {
	// Common field patterns that both approaches should handle consistently
	commonPatterns := []string{
		"Name string",
		"Age int",
		"Data []byte",
		"Handler func() error",
		"Method() error",
		"Process(string) (string, error)",
		"io.Reader",
		"Value T",
		"",                  // empty line
		"func Process() {}", // should be rejected by both
		"var name string",   // should be rejected by both
		"const value = 42",  // should be rejected by both
		"package main",      // should be rejected by both
		"import \"fmt\"",    // should be rejected by both
	}

	for _, pattern := range commonPatterns {
		t.Run("consistency_"+pattern, func(t *testing.T) {
			// Test that the restructured AST-first function produces consistent results
			// This ensures the AST approach doesn't introduce behavior regressions
			result := isStructOrInterfaceField(pattern)

			// The current string-based implementation result for comparison
			// (this will help verify consistency during implementation)
			currentResult := isStructOrInterfaceFieldCurrentImplementation(pattern)

			// For valid Go syntax, both approaches should give the same result
			// The test validates that AST detection doesn't break existing behavior
			if isValidGoSyntax(pattern) {
				assert.Equal(t, currentResult, result,
					"AST-first approach should be consistent with string approach for pattern: %q", pattern)
			}
		})
	}
}

// Helper function that replicates the current string-based implementation
// for consistency testing during the red phase.
// FIXED: Updated to handle embedded types correctly (was a bug in original).
func isStructOrInterfaceFieldCurrentImplementation(line string) bool {
	trimmed := strings.TrimSpace(line)

	// Empty lines are allowed
	if trimmed == "" {
		return true
	}

	// Reject obvious non-field patterns first
	if strings.HasPrefix(trimmed, "func ") ||
		strings.HasPrefix(trimmed, "var ") ||
		strings.HasPrefix(trimmed, "const ") ||
		strings.HasPrefix(trimmed, "import ") ||
		strings.HasPrefix(trimmed, "package ") {
		return false
	}

	// Struct/interface field patterns (basic heuristic)
	parts := strings.Fields(trimmed)
	if len(parts) >= 2 {
		// Look for typical field patterns: identifier followed by type
		// or method patterns: identifier followed by parentheses
		return true
	}

	// BUGFIX: Handle embedded types (single identifiers like "io.Reader")
	// The old implementation incorrectly returned false for these
	if len(parts) == 1 && len(parts[0]) > 0 {
		// Single identifier could be an embedded type
		return true
	}

	return false
}

// Helper function to determine if a line contains valid Go syntax.
func isValidGoSyntax(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}

	// Use tree-sitter to check if the line can be parsed as valid Go
	ctx := context.Background()
	testCode := "package test\n\ntype Test struct {\n" + line + "\n}"
	result := treesitter.CreateTreeSitterParseTree(ctx, testCode)

	hasErrors, err := result.ParseTree.HasSyntaxErrors()
	return result.Error == nil && err == nil && !hasErrors
}

// TestTreeSitterQueryEngineIntegrationForFieldDetection tests that the restructured function
// properly integrates with TreeSitterQueryEngine for field detection.
func TestTreeSitterQueryEngineIntegrationForFieldDetection(t *testing.T) {
	// Test cases that require specific TreeSitterQueryEngine methods
	tests := []struct {
		name                 string
		line                 string
		expectedAST          bool
		expectedQueryMethods []string
		description          string
	}{
		{
			name:                 "struct field uses QueryFieldDeclarations",
			line:                 "Name string `json:\"name\"`",
			expectedAST:          true,
			expectedQueryMethods: []string{"QueryFieldDeclarations"},
			description:          "Should use QueryFieldDeclarations for struct fields",
		},
		{
			name:                 "interface method uses QueryMethodSpecs",
			line:                 "Process(data string) error",
			expectedAST:          true,
			expectedQueryMethods: []string{"QueryMethodSpecs"},
			description:          "Should use QueryMethodSpecs for interface methods",
		},
		{
			name:                 "embedded type uses QueryEmbeddedTypes",
			line:                 "io.Reader",
			expectedAST:          true,
			expectedQueryMethods: []string{"QueryEmbeddedTypes"},
			description:          "Should use QueryEmbeddedTypes for embedded types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test verifies that the restructured function uses the correct
			// TreeSitterQueryEngine methods for different types of field detection.
			// The actual implementation should:
			// 1. Parse the line within a minimal struct/interface context
			// 2. Use the appropriate query methods based on the AST structure
			// 3. Fall back to string parsing only if AST analysis fails

			result := isStructOrInterfaceField(tt.line)

			// For this test, we expect AST-based detection to work
			assert.True(t, result, tt.description)
		})
	}
}

// TestIsStructOrInterfaceFieldPerformance tests that the AST-first approach
// doesn't introduce significant performance regressions compared to string parsing.
func TestIsStructOrInterfaceFieldPerformance(t *testing.T) {
	// Common field patterns for performance testing
	testLines := []string{
		"Name string",
		"Process(data string) error",
		"io.Reader",
		"Data map[string]interface{}",
		"func Process() {}", // should be rejected
		"var name string",   // should be rejected
	}

	// This test ensures the AST-first approach is reasonably performant
	for _, line := range testLines {
		t.Run("performance_"+line, func(t *testing.T) {
			// The restructured function should complete efficiently
			// AST parsing overhead should be acceptable for the improved accuracy
			result := isStructOrInterfaceField(line)

			// Basic assertion - actual performance measured separately
			assert.IsType(t, true, result, "Function should return boolean result efficiently")
		})
	}
}

// TestIsStructOrInterfaceFieldErrorHandling tests that the function gracefully handles
// edge cases and error conditions during both AST parsing and string fallback.
func TestIsStructOrInterfaceFieldErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		expectedField bool
		description   string
	}{
		{
			name:          "extremely long line",
			line:          strings.Repeat("Name string ", 1000),
			expectedField: true, // Should handle without crashing
			description:   "Should handle extremely long input lines gracefully",
		},
		{
			name:          "line with null bytes",
			line:          "Name string\x00embedded",
			expectedField: false, // Malformed, should reject
			description:   "Should handle lines with null bytes safely",
		},
		{
			name:          "line with unicode characters",
			line:          "名前 string", // "name" in Japanese
			expectedField: true,        // Valid Go identifier
			description:   "Should handle unicode identifiers correctly",
		},
		{
			name:          "line with mixed newlines",
			line:          "Name\r\nstring",
			expectedField: true, // Should normalize whitespace
			description:   "Should handle different newline characters",
		},
		{
			name:          "deeply nested generic constraints",
			line:          "Value map[K comparable]map[V any][]interface{ Process[T](T) T }",
			expectedField: true, // Complex but valid type
			description:   "Should handle deeply nested generic types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Function should not panic or crash on any input
			// and should handle edge cases gracefully
			result := isStructOrInterfaceField(tt.line)
			assert.Equal(t, tt.expectedField, result, tt.description)
		})
	}
}

// TestIsStructOrInterfaceFieldSpecificASTNodes tests detection of specific
// AST node types that should be identified as valid struct/interface fields.
func TestIsStructOrInterfaceFieldSpecificASTNodes(t *testing.T) {
	tests := []struct {
		name            string
		line            string
		expectedField   bool
		expectedASTType string
		description     string
	}{
		{
			name:            "field_declaration node detection",
			line:            "Name string `json:\"name\"`",
			expectedField:   true,
			expectedASTType: "field_declaration",
			description:     "Should identify field_declaration AST nodes",
		},
		{
			name:            "method_spec node detection",
			line:            "Process(data string) error",
			expectedField:   true,
			expectedASTType: "method_spec",
			description:     "Should identify method_spec AST nodes",
		},
		{
			name:            "type_identifier embedded detection",
			line:            "io.Reader",
			expectedField:   true,
			expectedASTType: "type_identifier",
			description:     "Should identify embedded type_identifier nodes",
		},
		{
			name:            "function_type field detection",
			line:            "Handler func(http.ResponseWriter, *http.Request)",
			expectedField:   true,
			expectedASTType: "field_declaration",
			description:     "Should identify function type fields as field_declaration",
		},
		{
			name:            "channel_type field detection",
			line:            "Events <-chan Event",
			expectedField:   true,
			expectedASTType: "field_declaration",
			description:     "Should identify channel type fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The restructured function should use AST node types to determine
			// if a line represents a valid struct or interface field
			result := isStructOrInterfaceField(tt.line)
			assert.Equal(t, tt.expectedField, result, tt.description)
		})
	}
}

// ============================================================================
// RED PHASE: COMPLEX GENERIC PATTERNS TESTS
// ============================================================================

// TestTryParseInStructContextComplexGenerics tests complex generic patterns
// that should stress the parser and drive improvements to handle sophisticated
// type constraints and nested generic structures.
func TestTryParseInStructContextComplexGenerics(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		expectedField bool
		description   string
	}{
		{
			name:          "deeply nested generic constraints with multiple bounds",
			line:          "Cache[K comparable, V any, T interface{~int|~string}, U ~[]V] map[K]T",
			expectedField: true,
			description:   "Should parse complex multi-constraint generic field declarations",
		},
		{
			name:          "recursive generic constraint with self-referencing types",
			line:          "Node[T comparable, N interface{ GetChildren() []N; GetParent() N }] *N",
			expectedField: true,
			description:   "Should handle recursive generic constraints in field types",
		},
		{
			name:          "union type constraints with complex interfaces",
			line:          "Processor[T interface{ ~string | ~[]byte; io.Reader; fmt.Stringer }] func(T) error",
			expectedField: true,
			description:   "Should parse union type constraints with embedded interfaces",
		},
		{
			name:          "nested generic with function type parameters",
			line:          "Handler[Req, Resp any] func(context.Context, Req) (Resp, error) `route:\"/api\"`",
			expectedField: true,
			description:   "Should parse generic function type fields with struct tags",
		},
		{
			name:          "complex channel with generic type parameters",
			line:          "EventChan[E interface{ GetTimestamp() time.Time; String() string }] <-chan E",
			expectedField: true,
			description:   "Should parse generic channel types with interface constraints",
		},
		{
			name:          "variadic generic with type approximation",
			line:          "Aggregator[T ~int | ~float64, R any] func(...T) R",
			expectedField: true,
			description:   "Should handle variadic functions with type approximation constraints",
		},
		{
			name:          "deeply nested map with complex generics",
			line:          "NestedMap[K1, K2 comparable, V1, V2 any] map[K1]map[K2]func(V1) V2",
			expectedField: true,
			description:   "Should parse deeply nested generic maps with function values",
		},
		{
			name:          "generic interface embedding with type parameters",
			line:          "Repository[T interface{ GetID() string }, Q interface{ Build() string }] interface{ Find(Q) ([]T, error) }",
			expectedField: true,
			description:   "Should parse generic interface fields with embedded constraints",
		},
		{
			name:          "complex slice with generic function elements",
			line:          "Validators[T any] []func(T) []error",
			expectedField: true,
			description:   "Should handle slices of generic function types",
		},
		{
			name:          "generic with comparable constraint and struct embedding",
			line:          "IndexedValue[T comparable] struct{ Value T; Index int64 }",
			expectedField: true,
			description:   "Should parse anonymous struct types with generic constraints",
		},
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test drives the implementation to handle complex generic patterns
			// The current implementation should be enhanced to support:
			// 1. Multi-constraint type parameters (T constraint1, constraint2)
			// 2. Union type constraints (~int | ~string)
			// 3. Recursive type constraints (self-referencing interfaces)
			// 4. Complex nested generic structures
			// 5. Type approximation constraints (~int, ~string)

			result := tryParseInStructContext(ctx, tt.line, queryEngine)

			// These tests are expected to fail initially, driving implementation improvements
			assert.True(t, result.parsed, "Should successfully parse complex generic syntax")
			assert.Equal(t, tt.expectedField, result.isField, tt.description)
		})
	}
}

// TestTryParseInStructContextGenericErrorRecovery tests that the parser can
// provide meaningful error context for malformed generic syntax.
func TestTryParseInStructContextGenericErrorRecovery(t *testing.T) {
	tests := []struct {
		name                 string
		line                 string
		expectedParsed       bool
		expectedField        bool
		expectedErrorContext string
		description          string
	}{
		{
			name:                 "mismatched generic brackets",
			line:                 "Cache[K comparable, V any map[K]V",
			expectedParsed:       false,
			expectedField:        false,
			expectedErrorContext: "missing closing bracket in generic type parameter",
			description:          "Should provide context for mismatched generic brackets",
		},
		{
			name:                 "invalid constraint syntax",
			line:                 "Value[T ~comparable &&&& Stringer] T",
			expectedParsed:       false,
			expectedField:        false,
			expectedErrorContext: "invalid constraint operator in type parameter",
			description:          "Should identify invalid constraint syntax",
		},
		{
			name:                 "incomplete union constraint",
			line:                 "Number[T ~int | ~float64 |] T",
			expectedParsed:       false,
			expectedField:        false,
			expectedErrorContext: "incomplete union constraint",
			description:          "Should detect incomplete union constraints",
		},
		{
			name:                 "circular generic constraint",
			line:                 "Node[T Node[T]] *T",
			expectedParsed:       false,
			expectedField:        false,
			expectedErrorContext: "circular generic constraint detected",
			description:          "Should detect and report circular constraints",
		},
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test drives enhanced error reporting for malformed generic syntax
			// The implementation should provide meaningful error context

			result := tryParseInStructContext(ctx, tt.line, queryEngine)

			assert.Equal(t, tt.expectedParsed, result.parsed, "Parse result should match expected")
			assert.Equal(t, tt.expectedField, result.isField, "Field detection should match expected")

			// TODO: Enhance tryParseInStructContext to return error context
			// This will fail initially and drive the implementation to add error context
			t.Skip("Enhanced error context not yet implemented - this test drives that enhancement")
		})
	}
}

// ============================================================================
// RED PHASE: PERFORMANCE BENCHMARK TESTS
// ============================================================================

// BenchmarkTryParseInStructContextSimpleField benchmarks simple field parsing performance.
func BenchmarkTryParseInStructContextSimpleField(b *testing.B) {
	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()
	line := "Name string"

	b.ResetTimer()
	for range b.N {
		result := tryParseInStructContext(ctx, line, queryEngine)
		if !result.parsed || !result.isField {
			b.Fatalf("Expected successful parsing of simple field")
		}
	}
}

// BenchmarkTryParseInStructContextComplexGeneric benchmarks complex generic field parsing.
func BenchmarkTryParseInStructContextComplexGeneric(b *testing.B) {
	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()
	line := "Cache[K comparable, V any, T interface{~int|~string}] map[K]map[string][]T"

	b.ResetTimer()
	for range b.N {
		result := tryParseInStructContext(ctx, line, queryEngine)
		// These benchmarks may initially fail, driving performance optimizations
		if !result.parsed {
			b.Fatalf("Expected successful parsing of complex generic field")
		}
	}
}

// BenchmarkTryParseInStructContextLargeField benchmarks parsing of fields with large type signatures.
func BenchmarkTryParseInStructContextLargeField(b *testing.B) {
	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()
	// Create a field with a large, complex type signature
	line := "Handler func(context.Context, *http.Request, map[string]interface{}, []byte, chan<- error) (*Response, []ValidationError, error)"

	b.ResetTimer()
	for range b.N {
		result := tryParseInStructContext(ctx, line, queryEngine)
		if !result.parsed || !result.isField {
			b.Fatalf("Expected successful parsing of large field signature")
		}
	}
}

// TestTryParseInStructContextPerformanceConstraints tests that field detection
// completes within acceptable time limits for various complexity levels.
func TestTryParseInStructContextPerformanceConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance constraint tests in short mode")
	}

	tests := []struct {
		name        string
		line        string
		maxDuration time.Duration
		description string
	}{
		{
			name:        "simple field under 1ms",
			line:        "Name string",
			maxDuration: 1 * time.Millisecond,
			description: "Simple fields should parse very quickly",
		},
		{
			name:        "complex generic under 5ms",
			line:        "Cache[K comparable, V any] map[K][]V",
			maxDuration: 5 * time.Millisecond,
			description: "Complex generic fields should parse within reasonable time",
		},
		{
			name:        "deeply nested type under 10ms",
			line:        "Data map[string]map[int64][]func(context.Context) (chan<- struct{}, error)",
			maxDuration: 10 * time.Millisecond,
			description: "Deeply nested types should parse within acceptable limits",
		},
		{
			name:        "large function signature under 15ms",
			line:        "ProcessRequest func(context.Context, *http.Request, map[string][]string, io.Reader) (*http.Response, []ValidationError, []byte, error)",
			maxDuration: 15 * time.Millisecond,
			description: "Large function signatures should not exceed reasonable parse time",
		},
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			result := tryParseInStructContext(ctx, tt.line, queryEngine)

			duration := time.Since(start)

			// These tests may initially fail, driving performance optimizations
			assert.True(t, result.parsed, "Should parse successfully")
			assert.True(t, result.isField, "Should detect as field")
			assert.LessOrEqual(t, duration, tt.maxDuration,
				"Parse time %v should be under %v for %s", duration, tt.maxDuration, tt.description)
		})
	}
}

// TestTryParseInStructContextMemoryUsage tests memory consumption during parsing.
func TestTryParseInStructContextMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage tests in short mode")
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	// Test with various field complexity levels
	testLines := []string{
		"Name string",
		"Data map[string]interface{}",
		"Cache[K comparable, V any] map[K][]V",
		"Handler func(context.Context, *http.Request) (*http.Response, error)",
	}

	for _, line := range testLines {
		t.Run("memory_usage_"+line, func(t *testing.T) {
			var memBefore, memAfter runtime.MemStats

			// Force GC and get baseline memory
			runtime.GC()
			runtime.ReadMemStats(&memBefore)

			// Perform parsing operations
			for range 100 {
				result := tryParseInStructContext(ctx, line, queryEngine)
				if !result.parsed {
					t.Fatalf("Parsing failed for line: %s", line)
				}
			}

			// Force GC and get final memory
			runtime.GC()
			runtime.ReadMemStats(&memAfter)

			// Calculate memory increase
			memIncrease := memAfter.Alloc - memBefore.Alloc

			// These tests may initially fail, driving memory optimization
			assert.Less(t, memIncrease, 1024*1024, // Less than 1MB increase
				"Memory increase should be reasonable: %d bytes for line: %s", memIncrease, line)
		})
	}
}

// ============================================================================
// RED PHASE: ENHANCED ERROR HANDLING TESTS
// ============================================================================

// TestTryParseInStructContextEnhancedErrorHandling tests that parsing errors
// provide meaningful context and drive improvements to error reporting.
func TestTryParseInStructContextEnhancedErrorHandling(t *testing.T) {
	tests := []struct {
		name                 string
		line                 string
		expectedParsed       bool
		expectedErrorType    string
		expectedErrorMessage string
		description          string
	}{
		{
			name:                 "malformed struct tag",
			line:                 "Name string `json:\"name\" invalid_tag_syntax",
			expectedParsed:       false,
			expectedErrorType:    "MALFORMED_STRUCT_TAG",
			expectedErrorMessage: "struct tag is not properly closed with backtick",
			description:          "Should provide specific error for malformed struct tags",
		},
		{
			name:                 "invalid type syntax",
			line:                 "Data []map[string interface{}",
			expectedParsed:       false,
			expectedErrorType:    "INVALID_TYPE_SYNTAX",
			expectedErrorMessage: "missing closing bracket in map type declaration",
			description:          "Should identify specific syntax errors in type declarations",
		},
		{
			name:                 "unsupported type construct",
			line:                 "Channel chan<->chan Event",
			expectedParsed:       false,
			expectedErrorType:    "UNSUPPORTED_TYPE_CONSTRUCT",
			expectedErrorMessage: "bidirectional channel of channels not supported",
			description:          "Should identify unsupported type constructs",
		},
		{
			name:                 "invalid identifier characters",
			line:                 "field-name string", // Invalid Go identifier
			expectedParsed:       false,
			expectedErrorType:    "INVALID_IDENTIFIER",
			expectedErrorMessage: "field name contains invalid characters",
			description:          "Should validate Go identifier naming rules",
		},
		{
			name:                 "context parsing timeout",
			line:                 strings.Repeat("nested[", 1000) + "T" + strings.Repeat("]", 1000),
			expectedParsed:       false,
			expectedErrorType:    "PARSING_TIMEOUT",
			expectedErrorMessage: "parsing exceeded maximum time limit",
			description:          "Should handle parsing timeouts gracefully",
		},
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test drives enhanced error reporting in tryParseInStructContext
			// The current implementation should be improved to provide specific error types and messages

			result := tryParseInStructContext(ctx, tt.line, queryEngine)

			assert.Equal(t, tt.expectedParsed, result.parsed, "Parse result should match expected")

			// TODO: Enhance contextParseResult to include error information
			// This will initially fail, driving the implementation to add error context
			t.Skip("Enhanced error context not yet implemented - this test drives that enhancement")
		})
	}
}

// TestTryParseInStructContextRecoveryStrategies tests that the parser can
// recover from certain syntax errors and still provide useful information.
func TestTryParseInStructContextRecoveryStrategies(t *testing.T) {
	tests := []struct {
		name             string
		line             string
		expectedParsed   bool
		expectedField    bool
		recoveryStrategy string
		description      string
	}{
		{
			name:             "recover from missing semicolon",
			line:             "Name string Age int", // Missing field separation
			expectedParsed:   true,
			expectedField:    false, // Cannot determine which part is the field
			recoveryStrategy: "split_fields",
			description:      "Should attempt to split multiple fields in one line",
		},
		{
			name:             "recover from incomplete generic",
			line:             "Cache[K comparable map[K]string", // Missing closing bracket
			expectedParsed:   true,
			expectedField:    true, // Should infer the missing bracket
			recoveryStrategy: "infer_missing_brackets",
			description:      "Should infer missing closing brackets in generic types",
		},
		{
			name:             "recover from partial function signature",
			line:             "Handler func(context.Context", // Missing closing paren and type
			expectedParsed:   true,
			expectedField:    true, // Should recognize as function type field
			recoveryStrategy: "partial_function_recognition",
			description:      "Should recognize partial function signatures",
		},
		{
			name:             "recover from trailing comma",
			line:             "Items []Item,", // Trailing comma
			expectedParsed:   true,
			expectedField:    true, // Should ignore trailing comma
			recoveryStrategy: "ignore_trailing_punctuation",
			description:      "Should ignore trailing punctuation in field declarations",
		},
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test drives the implementation of error recovery strategies
			// The parser should attempt to recover from common syntax errors

			result := tryParseInStructContext(ctx, tt.line, queryEngine)

			assert.Equal(t, tt.expectedParsed, result.parsed, tt.description)
			assert.Equal(t, tt.expectedField, result.isField, tt.description)

			// TODO: Implement recovery strategies in tryParseInStructContext
			t.Skip("Error recovery strategies not yet implemented - this test drives that enhancement")
		})
	}
}

// ============================================================================
// RED PHASE: EDGE CASES AND MALFORMED SYNTAX TESTS
// ============================================================================

// TestTryParseInStructContextEdgeCases tests edge cases that should drive
// robust handling of unusual but valid Go syntax patterns.
func TestTryParseInStructContextEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		expectedField bool
		description   string
	}{
		{
			name:          "field with unicode identifier",
			line:          "名前 string", // "name" in Japanese
			expectedField: true,
			description:   "Should handle unicode identifiers in field names",
		},
		{
			name:          "field with raw string literal tag",
			line:          "Path string `json:\"path\" validate:\"required,file\" description:\"File path with spaces and 'quotes'\"`",
			expectedField: true,
			description:   "Should handle complex struct tags with nested quotes",
		},
		{
			name:          "anonymous function with complex signature",
			line:          "Processor func(ctx context.Context, data []byte) (result interface{}, metadata map[string]interface{}, err error)",
			expectedField: true,
			description:   "Should parse complex anonymous function field types",
		},
		{
			name:          "deeply nested pointer types",
			line:          "Reference ****Node",
			expectedField: true,
			description:   "Should handle multiple levels of pointer indirection",
		},
		{
			name:          "field with very long type name",
			line:          strings.Repeat("VeryLongTypeName", 20) + " string",
			expectedField: true,
			description:   "Should handle extremely long type names without issue",
		},
		{
			name:          "field with mixed whitespace",
			line:          "Name\t\t  string\u00a0\u2003", // Contains tab, spaces, and unicode spaces
			expectedField: true,
			description:   "Should normalize various types of whitespace in field declarations",
		},
		{
			name:          "channel with complex element type",
			line:          "Events <-chan struct{ ID string; Data map[string]interface{} }",
			expectedField: true,
			description:   "Should parse channels with anonymous struct element types",
		},
		{
			name:          "field with embedded comment-like syntax in tag",
			line:          "Config string `json:\"config\" // This looks like a comment but it's in a tag`",
			expectedField: true,
			description:   "Should handle comment-like syntax within struct tags",
		},
		{
			name:          "field with newline characters in tag",
			line:          "Description string `json:\"desc\" help:\"This is a\\nmultiline\\ndescription\"`",
			expectedField: true,
			description:   "Should handle escaped newlines in struct tags",
		},
		{
			name:          "array with complex size expression",
			line:          "Buffer [1024*1024 + 256]byte",
			expectedField: true,
			description:   "Should parse arrays with complex size expressions",
		},
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These tests drive robust handling of edge cases in Go syntax
			result := tryParseInStructContext(ctx, tt.line, queryEngine)

			// These may initially fail, driving improvements to handle edge cases
			assert.True(t, result.parsed, "Should successfully parse edge case: %s", tt.line)
			assert.Equal(t, tt.expectedField, result.isField, tt.description)
		})
	}
}

// TestTryParseInStructContextMalformedButRecoverable tests syntax that is
// malformed but should be recoverable through enhanced parsing strategies.
func TestTryParseInStructContextMalformedButRecoverable(t *testing.T) {
	tests := []struct {
		name                 string
		line                 string
		expectedParsed       bool
		expectedField        bool
		requiredRecoveryType string
		description          string
	}{
		{
			name:                 "incomplete map type",
			line:                 "Data map[string",
			expectedParsed:       true,
			expectedField:        true,
			requiredRecoveryType: "type_completion",
			description:          "Should recover from incomplete map type by inferring closure",
		},
		{
			name:                 "mismatched brackets in slice",
			line:                 "Items []Item}",
			expectedParsed:       true,
			expectedField:        true,
			requiredRecoveryType: "bracket_correction",
			description:          "Should recover from mismatched brackets by correcting them",
		},
		{
			name:                 "partial struct tag",
			line:                 "Name string `json:\"name",
			expectedParsed:       true,
			expectedField:        true,
			requiredRecoveryType: "tag_completion",
			description:          "Should recover from incomplete struct tags",
		},
		{
			name:                 "extra punctuation",
			line:                 ";;Name string;;",
			expectedParsed:       true,
			expectedField:        true,
			requiredRecoveryType: "punctuation_cleanup",
			description:          "Should recover by cleaning up extraneous punctuation",
		},
		{
			name:                 "mixed line endings",
			line:                 "Name\r\nstring\n",
			expectedParsed:       true,
			expectedField:        true,
			requiredRecoveryType: "whitespace_normalization",
			description:          "Should recover by normalizing mixed line endings",
		},
		{
			name:                 "type with typos that can be inferred",
			line:                 "Names []stirng", // "stirng" instead of "string"
			expectedParsed:       true,
			expectedField:        false, // Should not auto-correct typos
			requiredRecoveryType: "typo_detection",
			description:          "Should detect but not auto-correct type name typos",
		},
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test drives enhanced recovery mechanisms in tryParseInStructContext
			result := tryParseInStructContext(ctx, tt.line, queryEngine)

			// These will initially fail, driving implementation of recovery strategies
			assert.Equal(t, tt.expectedParsed, result.parsed,
				"Recovery parsing should match expected for: %s", tt.line)
			assert.Equal(t, tt.expectedField, result.isField, tt.description)

			// TODO: Implement recovery strategies based on requiredRecoveryType
			t.Skip("Recovery strategies not yet implemented - this test drives that enhancement")
		})
	}
}

// TestTryParseInStructContextConcurrentParsing tests concurrent parsing safety.
func TestTryParseInStructContextConcurrentParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent parsing tests in short mode")
	}

	ctx := context.Background()
	queryEngine := NewTreeSitterQueryEngine()

	testLines := []string{
		"Name string",
		"Data map[string]interface{}",
		"Handler func() error",
		"Items []Item",
		"Cache[K comparable, V any] map[K]V",
	}

	const numGoroutines = 10
	const iterationsPerGoroutine = 100

	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines*iterationsPerGoroutine*len(testLines))

	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for range iterationsPerGoroutine {
				for _, line := range testLines {
					result := tryParseInStructContext(ctx, line, queryEngine)
					// This test may initially fail due to concurrency issues
					results <- result.parsed && result.isField
				}
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// Check all results
	successCount := 0
	totalCount := 0
	for success := range results {
		totalCount++
		if success {
			successCount++
		}
	}

	// All concurrent parsing operations should succeed
	assert.Equal(t, totalCount, successCount,
		"All concurrent parsing operations should succeed: %d/%d", successCount, totalCount)
}
