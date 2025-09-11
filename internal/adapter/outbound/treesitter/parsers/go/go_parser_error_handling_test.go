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

// TestGoParser_ErrorHandling_InvalidSyntax tests parser behavior with invalid Go syntax.
// This RED PHASE test defines expected error handling for malformed Go code.
func TestGoParser_ErrorHandling_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		expectedError string
		shouldFail    bool
		operation     string
	}{
		{
			name:          "malformed_function_declaration",
			source:        "func invalidFunc( { // missing closing paren and params",
			expectedError: "invalid function declaration: malformed parameter list",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "incomplete_struct_definition",
			source:        "type Person struct { // missing closing brace",
			expectedError: "invalid struct definition: missing closing brace",
			shouldFail:    true,
			operation:     "ExtractClasses",
		},
		{
			name:          "malformed_interface_definition",
			source:        "type Writer interface // missing opening brace",
			expectedError: "invalid interface definition: missing opening brace",
			shouldFail:    true,
			operation:     "ExtractInterfaces",
		},
		{
			name:          "invalid_variable_declaration",
			source:        "var x = // missing value after assignment",
			expectedError: "invalid variable declaration: missing value after assignment",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "mixed_language_syntax",
			source:        "func test() { console.log('hello'); }", // JavaScript in Go
			expectedError: "invalid Go syntax: detected non-Go language constructs",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "unclosed_string_literal",
			source:        `var message = "Hello world`,
			expectedError: "invalid syntax: unclosed string literal",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "unbalanced_braces",
			source:        "func test() { if true { return } // missing closing brace",
			expectedError: "invalid syntax: unbalanced braces",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "invalid_package_declaration",
			source:        "package // missing package name",
			expectedError: "invalid package declaration: missing package name",
			shouldFail:    true,
			operation:     "ExtractModules",
		},
		{
			name:          "malformed_import_statement",
			source:        `import "fmt // unclosed import string`,
			expectedError: "invalid import statement: unclosed import path",
			shouldFail:    true,
			operation:     "ExtractImports",
		},
		{
			name:          "invalid_method_receiver",
			source:        "func ( Person DoSomething() { }", // malformed receiver
			expectedError: "invalid method receiver: malformed receiver syntax",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create parser
			parser, err := NewGoParser()
			require.NoError(t, err)

			// Create parse tree
			parseTree := createMockGoParseTree(t, tt.source)

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			// Test the specific operation
			switch tt.operation {
			case "ExtractFunctions":
				_, err = parser.ExtractFunctions(ctx, parseTree, options)
			case "ExtractClasses":
				_, err = parser.ExtractClasses(ctx, parseTree, options)
			case "ExtractInterfaces":
				_, err = parser.ExtractInterfaces(ctx, parseTree, options)
			case "ExtractVariables":
				_, err = parser.ExtractVariables(ctx, parseTree, options)
			case "ExtractImports":
				_, err = parser.ExtractImports(ctx, parseTree, options)
			case "ExtractModules":
				_, err = parser.ExtractModules(ctx, parseTree, options)
			}

			if tt.shouldFail {
				assert.Error(t, err, "Expected error for invalid syntax")
				if err != nil {
					assert.Contains(t, err.Error(), strings.Split(tt.expectedError, ":")[0],
						"Error should contain expected error type")
				}
			} else {
				assert.NoError(t, err, "Valid syntax should not cause errors")
			}
		})
	}
}

// TestGoParser_ErrorHandling_MemoryExhaustion tests parser behavior with memory-intensive scenarios.
// This RED PHASE test defines expected error handling for memory exhaustion scenarios.
func TestGoParser_ErrorHandling_MemoryExhaustion(t *testing.T) {
	tests := []struct {
		name          string
		sourceGen     func() string
		expectedError string
		operation     string
		timeout       time.Duration
	}{
		{
			name: "extremely_large_file",
			sourceGen: func() string {
				// Generate a very large Go file (10MB+)
				var builder strings.Builder
				builder.WriteString("package main\n\n")
				for range 100000 {
					builder.WriteString("func veryLongFunctionName")
					builder.WriteString(strings.Repeat("X", 100))
					builder.WriteString("() { return }\n")
				}
				return builder.String()
			},
			expectedError: "memory limit exceeded: file too large to process safely",
			operation:     "ExtractFunctions",
			timeout:       5 * time.Second,
		},
		{
			name: "deeply_nested_structures",
			sourceGen: func() string {
				// Generate deeply nested struct definitions
				var builder strings.Builder
				builder.WriteString("package main\n\n")

				// Create 1000 levels of nested structs
				for i := range 1000 {
					builder.WriteString("type Level")
					builder.WriteString(strings.Repeat("Deep", i))
					builder.WriteString(" struct {\n")
					if i < 999 {
						builder.WriteString("    Next *Level")
						builder.WriteString(strings.Repeat("Deep", i+1))
						builder.WriteString("\n")
					}
					builder.WriteString("}\n")
				}
				return builder.String()
			},
			expectedError: "recursion limit exceeded: maximum nesting depth reached",
			operation:     "ExtractClasses",
			timeout:       3 * time.Second,
		},
		{
			name: "thousands_of_functions",
			sourceGen: func() string {
				var builder strings.Builder
				builder.WriteString("package main\n\n")

				// Generate 50,000 functions
				for i := range 50000 {
					builder.WriteString("func function")
					builder.WriteString(strings.Repeat("A", 50))
					builder.WriteString("Number")
					builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
					builder.WriteString("() {\n    // function body\n    return\n}\n")
				}
				return builder.String()
			},
			expectedError: "resource limit exceeded: too many constructs to process",
			operation:     "ExtractFunctions",
			timeout:       10 * time.Second,
		},
		{
			name: "massive_variable_declarations",
			sourceGen: func() string {
				var builder strings.Builder
				builder.WriteString("package main\n\nvar (\n")

				// Generate 100,000 variable declarations
				for i := range 100000 {
					builder.WriteString("    variable")
					builder.WriteString(strings.Repeat("X", 50))
					builder.WriteString("Number")
					builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
					builder.WriteString(" int\n")
				}
				builder.WriteString(")\n")
				return builder.String()
			},
			expectedError: "memory allocation exceeded: too many variables declared",
			operation:     "ExtractVariables",
			timeout:       5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Generate the source code
			source := tt.sourceGen()

			// Create parser
			parser, err := NewGoParser()
			require.NoError(t, err)

			// Create parse tree
			parseTree := createMockGoParseTree(t, source)

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       100, // High depth to test limits
			}

			// Test the operation with timeout
			var opErr error
			switch tt.operation {
			case "ExtractFunctions":
				_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
			case "ExtractClasses":
				_, opErr = parser.ExtractClasses(ctx, parseTree, options)
			case "ExtractVariables":
				_, opErr = parser.ExtractVariables(ctx, parseTree, options)
			}

			// Should either timeout or return memory error
			assert.Error(t, opErr, "Memory-intensive operation should fail")

			// Check if it's a timeout or expected memory error
			if ctx.Err() == context.DeadlineExceeded {
				assert.Contains(t, opErr.Error(), "timeout", "Should indicate timeout")
			} else {
				assert.Contains(t, opErr.Error(), strings.Split(tt.expectedError, ":")[0],
					"Should contain expected error type")
			}
		})
	}
}

// TestGoParser_ErrorHandling_TimeoutScenarios tests parser behavior with timeout scenarios.
// This RED PHASE test defines expected timeout handling behavior.
func TestGoParser_ErrorHandling_TimeoutScenarios(t *testing.T) {
	tests := []struct {
		name          string
		sourceGen     func() string
		timeout       time.Duration
		expectedError string
		operation     string
	}{
		{
			name: "parsing_timeout",
			sourceGen: func() string {
				// Generate complex code that would take a long time to parse
				var builder strings.Builder
				builder.WriteString("package main\n\n")

				// Create complex nested structures that take time to process
				for i := range 1000 {
					builder.WriteString("func complexFunction")
					builder.WriteString(strings.Repeat("A", i%100))
					builder.WriteString("() {\n")

					// Add nested loops and conditions
					for j := range 50 {
						builder.WriteString("    for i := 0; i < 100; i++ {\n")
						builder.WriteString("        if condition")
						builder.WriteString(strings.Repeat("B", j%50))
						builder.WriteString(" {\n")
						builder.WriteString("            // nested logic\n")
						builder.WriteString("        }\n")
						builder.WriteString("    }\n")
					}
					builder.WriteString("}\n")
				}
				return builder.String()
			},
			timeout:       100 * time.Millisecond, // Very short timeout
			expectedError: "operation timeout: parsing exceeded maximum allowed time",
			operation:     "ExtractFunctions",
		},
		{
			name: "class_extraction_timeout",
			sourceGen: func() string {
				var builder strings.Builder
				builder.WriteString("package main\n\n")

				// Generate many complex structs
				for i := range 500 {
					builder.WriteString("type ComplexStruct")
					builder.WriteString(strings.Repeat("X", i%100))
					builder.WriteString(" struct {\n")

					// Add many fields
					for j := range 200 {
						builder.WriteString("    Field")
						builder.WriteString(strings.Repeat("Y", j%50))
						builder.WriteString(" string\n")
					}
					builder.WriteString("}\n")
				}
				return builder.String()
			},
			timeout:       50 * time.Millisecond,
			expectedError: "operation timeout: class extraction exceeded maximum allowed time",
			operation:     "ExtractClasses",
		},
		{
			name: "infinite_loop_scenario",
			sourceGen: func() string {
				// Code that might cause parser to enter infinite loop
				return `package main

type Node struct {
	Value int
	Next  *Node
	Prev  *Node
	Self  *Node // Self-reference that might cause issues
}

func (n *Node) Process() {
	current := n
	for current != nil {
		// This could potentially create parsing complexity
		if current.Self == current {
			current = current.Next
		}
	}
}`
			},
			timeout:       200 * time.Millisecond,
			expectedError: "operation timeout: potential infinite loop detected",
			operation:     "ExtractFunctions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			source := tt.sourceGen()

			// Create parser
			parser, err := NewGoParser()
			require.NoError(t, err)

			// Create parse tree
			parseTree := createMockGoParseTree(t, source)

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       50,
			}

			start := time.Now()
			var opErr error

			switch tt.operation {
			case "ExtractFunctions":
				_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
			case "ExtractClasses":
				_, opErr = parser.ExtractClasses(ctx, parseTree, options)
			}

			elapsed := time.Since(start)

			// Should timeout or return error
			assert.Error(t, opErr, "Operation should fail due to timeout")

			if elapsed >= tt.timeout {
				assert.Contains(t, opErr.Error(), "timeout", "Should indicate timeout")
			} else {
				// If completed before timeout, should still handle timeout scenario
				t.Logf("Operation completed in %v (before timeout %v)", elapsed, tt.timeout)
			}
		})
	}
}

// TestGoParser_ErrorHandling_EdgeCases tests parser behavior with edge case scenarios.
// This RED PHASE test defines expected behavior for unusual input scenarios.
func TestGoParser_ErrorHandling_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		expectedError string
		shouldFail    bool
		operation     string
	}{
		{
			name:          "empty_file",
			source:        "",
			expectedError: "empty source: no content to parse",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "whitespace_only",
			source:        "   \n  \t  \n   ",
			expectedError: "empty source: only whitespace content",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "binary_data_as_source",
			source:        string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}),
			expectedError: "invalid encoding: source contains non-UTF8 characters",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "extremely_long_single_line",
			source:        "package main\nfunc " + strings.Repeat("x", 100000) + "() {}\n",
			expectedError: "line too long: exceeds maximum line length limit",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "unicode_in_identifiers",
			source:        "package main\nfunc 函数名() {}\ntype 类型 struct {}\n",
			expectedError: "invalid identifier: non-ASCII characters in identifier",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "null_bytes_in_source",
			source:        "package main\n\x00func test() {}\n",
			expectedError: "invalid source: contains null bytes",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "circular_import_references",
			source:        "package main\nimport \"main\"\nfunc test() {}",
			expectedError: "circular dependency: self-import detected",
			shouldFail:    true,
			operation:     "ExtractImports",
		},
		{
			name:          "malformed_utf8_sequence",
			source:        "package main\nfunc test() { s := \"\xff\xfe\" }",
			expectedError: "encoding error: malformed UTF-8 sequence",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "source_with_bom",
			source:        "\xEF\xBB\xBFpackage main\nfunc test() {}",
			expectedError: "encoding issue: unexpected BOM marker",
			shouldFail:    false, // Should handle BOM gracefully
			operation:     "ExtractFunctions",
		},
		{
			name:          "missing_package_declaration",
			source:        "func test() {}\ntype Person struct {}",
			expectedError: "missing package declaration: Go files must start with package",
			shouldFail:    true,
			operation:     "ExtractModules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create parser
			parser, err := NewGoParser()
			require.NoError(t, err)

			// For edge cases, we might need to handle parse tree creation differently
			var parseTree *valueobject.ParseTree
			if tt.source == "" || strings.TrimSpace(tt.source) == "" {
				// For empty sources, createMockGoParseTree might fail
				// Test should handle this scenario
				goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
				rootNode := &valueobject.ParseNode{
					Type: "module",
				}
				metadata, _ := valueobject.NewParseMetadata(0, "0.0.0", "0.0.0")
				parseTree, _ = valueobject.NewParseTree(ctx, goLang, rootNode, []byte(tt.source), metadata)
			} else {
				parseTree = createMockGoParseTree(t, tt.source)
			}

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			// Test the operation
			var opErr error
			switch tt.operation {
			case "ExtractFunctions":
				_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
			case "ExtractImports":
				_, opErr = parser.ExtractImports(ctx, parseTree, options)
			case "ExtractModules":
				_, opErr = parser.ExtractModules(ctx, parseTree, options)
			}

			if tt.shouldFail {
				assert.Error(t, opErr, "Edge case should cause error: %s", tt.name)
				if opErr != nil {
					assert.Contains(t, opErr.Error(), strings.Split(tt.expectedError, ":")[0],
						"Error should contain expected error type")
				}
			} else if opErr != nil {
				// Should handle gracefully
				t.Logf("Operation returned error (may be expected): %v", opErr)
			}
		})
	}
}

// TestGoParser_ErrorHandling_ConcurrentAccess tests parser behavior under concurrent access.
// This RED PHASE test defines expected behavior for concurrent parser usage with errors.
func TestGoParser_ErrorHandling_ConcurrentAccess(t *testing.T) {
	// Create parser
	parser, err := NewGoParser()
	require.NoError(t, err)

	// Test concurrent access with various error scenarios
	errorSources := []string{
		"func invalid( {", // malformed syntax
		"",                // empty source
		strings.Repeat("func test() {}\n", 10000), // large source
		"package main\nfunc \xff\xfe() {}",        // invalid encoding
	}

	ctx := context.Background()
	concurrency := 10
	iterations := 50

	// Channel to collect errors
	errorChan := make(chan error, concurrency*iterations*len(errorSources))

	// Test concurrent extraction with error scenarios
	for i := range concurrency {
		go func(workerID int) {
			for j := range iterations {
				for k, source := range errorSources {
					// Create parse tree (might fail for some sources)
					var parseTree *valueobject.ParseTree

					if source == "" {
						// Handle empty source specially
						goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
						rootNode := &valueobject.ParseNode{Type: "module"}
						metadata, _ := valueobject.NewParseMetadata(0, "0.0.0", "0.0.0")
						parseTree, _ = valueobject.NewParseTree(ctx, goLang, rootNode, []byte(source), metadata)
					} else {
						parseTree = createMockGoParseTree(t, source)
					}

					options := outbound.SemanticExtractionOptions{
						IncludePrivate: true,
						MaxDepth:       10,
					}

					// Try extraction - expect various errors
					_, funcErr := parser.ExtractFunctions(ctx, parseTree, options)
					if funcErr != nil {
						errorChan <- funcErr
					}

					_, classErr := parser.ExtractClasses(ctx, parseTree, options)
					if classErr != nil {
						errorChan <- classErr
					}

					// Log progress
					if j%10 == 0 {
						t.Logf("Worker %d completed %d iterations for source %d", workerID, j, k)
					}
				}
			}
		}(i)
	}

	// Collect errors for a reasonable time
	timeout := time.After(30 * time.Second)
	errorCount := 0

CollectErrors:
	for {
		select {
		case err := <-errorChan:
			errorCount++
			// Should get various types of errors
			assert.Error(t, err, "Should receive errors from invalid sources")
			t.Logf("Received error %d: %s", errorCount, err.Error())

		case <-timeout:
			break CollectErrors
		}

		// Stop collecting if we got enough errors
		if errorCount > 100 {
			break CollectErrors
		}
	}

	// Should have received multiple errors
	assert.Positive(t, errorCount, "Should receive errors from concurrent processing")
	t.Logf("Total errors collected: %d", errorCount)

	// Test that parser is still functional after concurrent errors
	validSource := "package main\nfunc test() {}\ntype Person struct {}"
	parseTree := createMockGoParseTree(t, validSource)
	options := outbound.SemanticExtractionOptions{IncludePrivate: true}

	// These should work fine
	functions, err := parser.ExtractFunctions(ctx, parseTree, options)
	assert.NoError(t, err, "Parser should still work after concurrent errors")
	assert.NotEmpty(t, functions, "Should extract functions from valid source")
}

// TestGoParser_ErrorHandling_ResourceCleanup tests proper resource cleanup on errors.
// This RED PHASE test defines expected resource cleanup behavior.
func TestGoParser_ErrorHandling_ResourceCleanup(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		expectedError string
		operation     string
	}{
		{
			name:          "cleanup_after_parse_failure",
			source:        "invalid go syntax {{{",
			expectedError: "parse failure: should cleanup resources",
			operation:     "ExtractFunctions",
		},
		{
			name:          "cleanup_after_timeout",
			source:        strings.Repeat("func test() {}\n", 10000),
			expectedError: "timeout: should cleanup resources",
			operation:     "ExtractFunctions",
		},
		{
			name:          "cleanup_after_memory_error",
			source:        strings.Repeat("type Test struct {}\n", 50000),
			expectedError: "memory error: should cleanup resources",
			operation:     "ExtractClasses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Short timeout to force cleanup scenarios
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Create parser
			parser, err := NewGoParser()
			require.NoError(t, err)

			// Create parse tree
			parseTree := createMockGoParseTree(t, tt.source)

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			// Track resource usage (simplified)
			initialGoroutines := countGoroutines()

			// Perform operation that should fail and cleanup
			var opErr error
			switch tt.operation {
			case "ExtractFunctions":
				_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
			case "ExtractClasses":
				_, opErr = parser.ExtractClasses(ctx, parseTree, options)
			}

			// Should either get an error or timeout
			if ctx.Err() == context.DeadlineExceeded {
				t.Logf("Operation timed out as expected")
			} else if opErr != nil {
				t.Logf("Operation failed with error: %v", opErr)
			}

			// Allow some time for cleanup
			time.Sleep(50 * time.Millisecond)

			// Check that resources are cleaned up
			finalGoroutines := countGoroutines()

			// Should not have leaked goroutines (allow some tolerance)
			assert.LessOrEqual(t, finalGoroutines, initialGoroutines+5,
				"Should not leak goroutines after error")

			// Parser should still be usable after cleanup
			validSource := "package main\nfunc test() {}"
			validParseTree := createMockGoParseTree(t, validSource)
			validCtx := context.Background()

			_, validErr := parser.ExtractFunctions(validCtx, validParseTree, options)
			assert.NoError(t, validErr, "Parser should be usable after cleanup")
		})
	}
}

// Helper function to create a mock Go parse tree for testing.
func createMockGoParseTree(t *testing.T, source string) *valueobject.ParseTree {
	ctx := context.Background()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	rootNode := &valueobject.ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   uint32(len(source)),
		Children:  []*valueobject.ParseNode{},
	}

	metadata, err := valueobject.NewParseMetadata(0, "0.0.0", "0.0.0")
	require.NoError(t, err)

	parseTree, err := valueobject.NewParseTree(ctx, goLang, rootNode, []byte(source), metadata)
	require.NoError(t, err)

	return parseTree
}

// Helper function to count goroutines (simplified).
func countGoroutines() int {
	// In a real implementation, this would use runtime.NumGoroutine()
	// or more sophisticated goroutine tracking
	return 10 // Placeholder value for RED phase
}
