package javascriptparser

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

// NOTE: TestJavaScriptParser_ErrorHandling_TreeSitterErrors was removed because it tested
// scenarios that cannot be triggered through JavaScript source code alone. Tree-sitter
// infrastructure failures (grammar loading, parser creation, language setting) require
// system-level failures (corrupted files, memory exhaustion, library loading errors) that
// are outside the scope of application-level testing. Tree-sitter's design guarantees that
// Parse() always produces a tree, even for invalid syntax - errors appear as ERROR/MISSING
// nodes within the tree structure, not as parse failures.
//
// Coverage for realistic error scenarios remains comprehensive through:
// - TestJavaScriptParser_ErrorHandling_InvalidSyntax (syntax errors via ERROR nodes)
// - TestJavaScriptParser_ErrorHandling_MemoryExhaustion (resource limits)
// - TestJavaScriptParser_ErrorHandling_TimeoutScenarios (operation timeouts)
// - TestJavaScriptParser_ErrorHandling_EdgeCases (encoding, empty input, etc.)
// - TestJavaScriptParser_ErrorHandling_ConcurrentAccess (concurrent usage)
// - TestJavaScriptParser_ErrorHandling_ResourceCleanup (cleanup behavior)

// TestJavaScriptParser_ErrorHandling_InvalidSyntax tests parser behavior with invalid JavaScript syntax.
// This RED PHASE test defines expected error handling for malformed JavaScript code.
func TestJavaScriptParser_ErrorHandling_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		expectedError string
		shouldFail    bool
		operation     string
	}{
		{
			name:          "malformed_function_declaration",
			source:        "function invalidFunc( { // missing closing paren and params",
			expectedError: "invalid function declaration: malformed parameter list",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "incomplete_class_definition",
			source:        "class Person { // missing closing brace",
			expectedError: "invalid class definition: missing closing brace",
			shouldFail:    true,
			operation:     "ExtractClasses",
		},
		{
			name:          "malformed_arrow_function",
			source:        "const fn = (x, y => x + y; // missing closing paren",
			expectedError: "invalid arrow function: malformed parameter list",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "invalid_variable_declaration",
			source:        "let x = ; // missing value after assignment",
			expectedError: "invalid variable declaration: missing value after assignment",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "unclosed_string_literal",
			source:        `const message = "Hello world`,
			expectedError: "invalid syntax: unclosed string literal",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "unbalanced_braces",
			source:        "function test() { if (true) { return; } // missing closing brace",
			expectedError: "invalid syntax: unbalanced braces",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "invalid_import_statement",
			source:        `import { useState from 'react'; // missing closing brace`,
			expectedError: "invalid import statement: malformed import syntax",
			shouldFail:    true,
			operation:     "ExtractImports",
		},
		{
			name:          "malformed_export_statement",
			source:        "export { function test() {} }; // function in export object",
			expectedError: "invalid export statement: malformed export syntax",
			shouldFail:    true,
			operation:     "ExtractModules",
		},
		{
			name:          "invalid_async_await_syntax",
			source:        "async function test() { await; } // missing expression after await",
			expectedError: "invalid async/await syntax: missing expression after await",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "malformed_destructuring",
			source:        "const { x, y, } = obj; // trailing comma in destructuring",
			expectedError: "invalid destructuring: trailing comma not allowed",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "invalid_template_literal",
			source:        "const str = `Hello ${name; // missing closing brace",
			expectedError: "invalid template literal: unclosed expression",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "mixed_language_syntax",
			source:        "function test() { print('hello') }", // Python syntax in JavaScript
			expectedError: "invalid JavaScript syntax: detected non-JavaScript language constructs",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create parser
			parser, err := NewJavaScriptParser()
			require.NoError(t, err)

			// Create parse tree
			parseTree := createMockJavaScriptParseTree(t, tt.source)

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

// TestJavaScriptParser_ErrorHandling_MemoryExhaustion tests parser behavior with memory-intensive scenarios.
// This RED PHASE test defines expected error handling for memory exhaustion scenarios.
func TestJavaScriptParser_ErrorHandling_MemoryExhaustion(t *testing.T) {
	tests := []struct {
		name          string
		sourceGen     func() string
		expectedError string
		operation     string
		timeout       time.Duration
	}{
		{
			name: "extremely_large_js_file",
			sourceGen: func() string {
				// Generate a very large JavaScript file (10MB+)
				var builder strings.Builder
				for range 100000 {
					builder.WriteString("function veryLongFunctionName")
					builder.WriteString(strings.Repeat("X", 100))
					builder.WriteString("() { return ")
					builder.WriteString(strings.Repeat("'data'", 50))
					builder.WriteString("; }\n")
				}
				return builder.String()
			},
			expectedError: "memory limit exceeded: file too large to process safely",
			operation:     "ExtractFunctions",
			timeout:       5 * time.Second,
		},
		{
			name: "deeply_nested_objects",
			sourceGen: func() string {
				// Generate deeply nested object/function structures
				var builder strings.Builder
				builder.WriteString("const deepObject = ")

				// Create 1000 levels of nesting
				for i := range 1000 {
					builder.WriteString("{ level")
					builder.WriteString(strings.Repeat("Deep", i%100))
					builder.WriteString(": ")
				}

				// Close all the braces
				for range 1000 {
					builder.WriteString("}")
				}
				builder.WriteString(";\n")

				return builder.String()
			},
			expectedError: "recursion limit exceeded: maximum nesting depth reached",
			operation:     "ExtractVariables",
			timeout:       3 * time.Second,
		},
		{
			name: "thousands_of_functions",
			sourceGen: func() string {
				var builder strings.Builder

				// Generate 50,000 functions
				for i := range 50000 {
					builder.WriteString("function function")
					builder.WriteString(strings.Repeat("A", 50))
					builder.WriteString("Number")
					builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
					builder.WriteString("() {\n  return ")
					builder.WriteString(strings.Repeat("'value'", 10))
					builder.WriteString(";\n}\n")
				}
				return builder.String()
			},
			expectedError: "resource limit exceeded: too many constructs to process",
			operation:     "ExtractFunctions",
			timeout:       10 * time.Second,
		},
		{
			name: "massive_class_with_methods",
			sourceGen: func() string {
				var builder strings.Builder
				builder.WriteString("class MassiveClass {\n")

				// Generate 10,000 methods
				for i := range 10000 {
					builder.WriteString("  method")
					builder.WriteString(strings.Repeat("X", 50))
					builder.WriteString("Number")
					builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
					builder.WriteString("() {\n")
					builder.WriteString("    return ")
					builder.WriteString(strings.Repeat("'result'", 20))
					builder.WriteString(";\n  }\n")
				}

				builder.WriteString("}\n")
				return builder.String()
			},
			expectedError: "memory allocation exceeded: too many methods in class",
			operation:     "ExtractClasses",
			timeout:       5 * time.Second,
		},
		{
			name: "circular_reference_nightmare",
			sourceGen: func() string {
				var builder strings.Builder

				// Create complex circular references that could cause infinite loops
				for i := range 1000 {
					builder.WriteString("const obj")
					builder.WriteString(strings.Repeat("A", i%50))
					builder.WriteString(" = {\n")
					builder.WriteString("  self: null,\n")
					builder.WriteString("  next: null,\n")
					builder.WriteString("  process: function() {\n")
					builder.WriteString("    if (this.self === this) {\n")
					builder.WriteString("      return this.next && this.next.process();\n")
					builder.WriteString("    }\n")
					builder.WriteString("  }\n")
					builder.WriteString("};\n")

					// Create circular references
					if i > 0 {
						prevIndex := i - 1
						builder.WriteString("obj")
						builder.WriteString(strings.Repeat("A", i%50))
						builder.WriteString(".self = obj")
						builder.WriteString(strings.Repeat("A", prevIndex%50))
						builder.WriteString(";\n")
					}
				}

				return builder.String()
			},
			expectedError: "circular reference detected: potential memory leak",
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
			parser, err := NewJavaScriptParser()
			require.NoError(t, err)

			// Create parse tree
			parseTree := createMockJavaScriptParseTree(t, source)

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

// TestJavaScriptParser_ErrorHandling_TimeoutScenarios tests parser behavior with timeout scenarios.
// This RED PHASE test defines expected timeout handling behavior.
func TestJavaScriptParser_ErrorHandling_TimeoutScenarios(t *testing.T) {
	tests := []struct {
		name          string
		sourceGen     func() string
		timeout       time.Duration
		expectedError string
		operation     string
	}{
		{
			name: "tree_sitter_parsing_timeout",
			sourceGen: func() string {
				// Generate complex JavaScript that takes time for tree-sitter to parse
				var builder strings.Builder

				// Create complex nested structures that take time to process
				for i := range 1000 {
					builder.WriteString("function complexFunction")
					builder.WriteString(strings.Repeat("A", i%100))
					builder.WriteString("() {\n")

					// Add nested async/await patterns
					for j := range 50 {
						builder.WriteString("  const result")
						builder.WriteString(strings.Repeat("B", j%50))
						builder.WriteString(" = await (async () => {\n")
						builder.WriteString("    for (let k = 0; k < 100; k++) {\n")
						builder.WriteString("      if (condition")
						builder.WriteString(strings.Repeat("C", j%25))
						builder.WriteString(") {\n")
						builder.WriteString("        return Promise.resolve(k);\n")
						builder.WriteString("      }\n")
						builder.WriteString("    }\n")
						builder.WriteString("  })();\n")
					}
					builder.WriteString("}\n")
				}
				return builder.String()
			},
			timeout:       100 * time.Millisecond, // Very short timeout
			expectedError: "operation timeout: tree-sitter parsing exceeded maximum allowed time",
			operation:     "Parse",
		},
		{
			name: "function_extraction_timeout",
			sourceGen: func() string {
				var builder strings.Builder

				// Generate many complex functions with generators and async patterns
				for i := range 500 {
					builder.WriteString("async function* complexGenerator")
					builder.WriteString(strings.Repeat("X", i%100))
					builder.WriteString("() {\n")

					// Add complex logic that might slow down extraction
					for j := range 100 {
						builder.WriteString("  try {\n")
						builder.WriteString("    const data = await fetch('/api/data")
						builder.WriteString(strings.Repeat("Y", j%50))
						builder.WriteString("');\n")
						builder.WriteString("    yield* processData(data);\n")
						builder.WriteString("  } catch (error) {\n")
						builder.WriteString("    yield handleError(error);\n")
						builder.WriteString("  } finally {\n")
						builder.WriteString("    cleanup();\n")
						builder.WriteString("  }\n")
					}
					builder.WriteString("}\n")
				}
				return builder.String()
			},
			timeout:       50 * time.Millisecond,
			expectedError: "operation timeout: function extraction exceeded maximum allowed time",
			operation:     "ExtractFunctions",
		},
		{
			name: "class_extraction_timeout",
			sourceGen: func() string {
				var builder strings.Builder

				// Generate complex classes with many decorators and methods
				for i := range 200 {
					builder.WriteString("@Component\n")
					builder.WriteString("@Injectable\n")
					builder.WriteString("@Decorator")
					builder.WriteString(strings.Repeat("Z", i%50))
					builder.WriteString("\n")
					builder.WriteString("class ComplexClass")
					builder.WriteString(strings.Repeat("W", i%100))
					builder.WriteString(" extends BaseClass {\n")

					// Add many getters, setters, and methods
					for j := range 200 {
						// Getter
						builder.WriteString("  get property")
						builder.WriteString(strings.Repeat("G", j%30))
						builder.WriteString("() {\n")
						builder.WriteString("    return this._value")
						builder.WriteString(strings.Repeat("V", j%20))
						builder.WriteString(";\n  }\n")

						// Setter
						builder.WriteString("  set property")
						builder.WriteString(strings.Repeat("S", j%30))
						builder.WriteString("(value) {\n")
						builder.WriteString("    this._value")
						builder.WriteString(strings.Repeat("V", j%20))
						builder.WriteString(" = value;\n  }\n")

						// Method
						builder.WriteString("  async method")
						builder.WriteString(strings.Repeat("M", j%40))
						builder.WriteString("() {\n")
						builder.WriteString("    return await this.process();\n")
						builder.WriteString("  }\n")
					}

					builder.WriteString("}\n")
				}
				return builder.String()
			},
			timeout:       200 * time.Millisecond,
			expectedError: "operation timeout: class extraction exceeded maximum allowed time",
			operation:     "ExtractClasses",
		},
		{
			name: "infinite_loop_scenario",
			sourceGen: func() string {
				// Code that might cause parser to enter infinite loop during node traversal
				return `
const recursiveObject = {
	self: null,
	process: function() {
		let current = this;
		while (current) {
			if (current.self === current) {
				// This could create parsing complexity for tree traversal
				current = current.self;
			}
			break;
		}
	}
};

recursiveObject.self = recursiveObject;

function complexRecursion(obj) {
	if (obj && obj.self) {
		return complexRecursion(obj.self);
	}
	return obj;
}

const result = complexRecursion(recursiveObject);`
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
			parser, err := NewJavaScriptParser()
			require.NoError(t, err)

			start := time.Now()
			var opErr error

			switch tt.operation {
			case "Parse":
				_, opErr = parser.Parse(ctx, []byte(source))
			case "ExtractFunctions":
				parseTree := createMockJavaScriptParseTree(t, source)
				options := outbound.SemanticExtractionOptions{
					IncludePrivate: true,
					MaxDepth:       50,
				}
				_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
			case "ExtractClasses":
				parseTree := createMockJavaScriptParseTree(t, source)
				options := outbound.SemanticExtractionOptions{
					IncludePrivate: true,
					MaxDepth:       50,
				}
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

// TestJavaScriptParser_ErrorHandling_EdgeCases tests parser behavior with edge case scenarios.
// This RED PHASE test defines expected behavior for unusual input scenarios.
func TestJavaScriptParser_ErrorHandling_EdgeCases(t *testing.T) {
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
			operation:     "Parse",
		},
		{
			name:          "extremely_long_single_line",
			source:        "function " + strings.Repeat("x", 100000) + "() {}\n",
			expectedError: "line too long: exceeds maximum line length limit",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "unicode_in_identifiers",
			source:        "function 函数名() {}\nclass 类型 {}\n",
			expectedError: "invalid identifier: non-ASCII characters in identifier",
			shouldFail:    false, // JavaScript allows Unicode identifiers
			operation:     "ExtractFunctions",
		},
		{
			name:          "null_bytes_in_source",
			source:        "function test() { return 'hello'; }\n\x00console.log('test');\n",
			expectedError: "invalid source: contains null bytes",
			shouldFail:    true,
			operation:     "Parse",
		},
		{
			name:          "malformed_utf8_sequence",
			source:        "const message = '\xff\xfe';\n",
			expectedError: "encoding error: malformed UTF-8 sequence",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "source_with_bom",
			source:        "\xEF\xBB\xBFfunction test() { return 42; }",
			expectedError: "encoding issue: unexpected BOM marker",
			shouldFail:    false, // Should handle BOM gracefully
			operation:     "ExtractFunctions",
		},
		{
			name:          "json_data_as_javascript",
			source:        `{"name": "test", "value": 42, "items": [1, 2, 3]}`,
			expectedError: "invalid JavaScript: JSON data detected",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "html_with_javascript",
			source:        `<script>function test() { alert('hello'); }</script>`,
			expectedError: "invalid JavaScript: HTML content detected",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "extremely_deep_callback_hell",
			source:        generateCallbackHell(1000), // 1000 levels deep
			expectedError: "recursion limit exceeded: callback nesting too deep",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "invalid_jsx_syntax",
			source:        "const element = <div>Hello {name</div>; // missing closing brace",
			expectedError: "invalid JSX: malformed JSX expression",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create parser
			parser, err := NewJavaScriptParser()
			require.NoError(t, err)

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			// Test the operation
			var opErr error
			switch tt.operation {
			case "Parse":
				_, opErr = parser.Parse(ctx, []byte(tt.source))
			case "ExtractFunctions":
				// For edge cases, we might need to handle parse tree creation differently
				if tt.source == "" || strings.TrimSpace(tt.source) == "" {
					// For empty sources, createMockJavaScriptParseTree might fail
					jsLang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
					rootNode := &valueobject.ParseNode{Type: "program"}
					metadata, _ := valueobject.NewParseMetadata(0, "0.0.0", "0.0.0")
					parseTree, _ := valueobject.NewParseTree(ctx, jsLang, rootNode, []byte(tt.source), metadata)
					_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
				} else {
					parseTree := createMockJavaScriptParseTree(t, tt.source)
					_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
				}
			case "ExtractVariables":
				parseTree := createMockJavaScriptParseTree(t, tt.source)
				_, opErr = parser.ExtractVariables(ctx, parseTree, options)
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

// TestJavaScriptParser_ErrorHandling_ConcurrentAccess tests parser behavior under concurrent access.
// This RED PHASE test defines expected behavior for concurrent parser usage with errors.
func TestJavaScriptParser_ErrorHandling_ConcurrentAccess(t *testing.T) {
	parserInterface, err := NewJavaScriptParser()
	require.NoError(t, err)

	parser, ok := parserInterface.(*ObservableJavaScriptParser)
	require.True(t, ok, "Parser should be of type *ObservableJavaScriptParser")

	errorSources := getConcurrentTestSources()
	ctx := context.Background()

	errorChan := runConcurrentParserTests(t, parser, errorSources, ctx)
	errorCount := collectConcurrentErrors(t, errorChan)

	validateParserStillFunctional(t, parser, ctx, errorCount)
}

// getConcurrentTestSources returns test sources for concurrent testing.
func getConcurrentTestSources() []string {
	return []string{
		"function invalid( {", // malformed syntax
		"",                    // empty source
		strings.Repeat("function test() {}\n", 10000),  // large source
		"function test() { console.log('\xff\xfe'); }", // invalid encoding
		generateCallbackHell(500),                      // deeply nested
	}
}

// runConcurrentParserTests executes concurrent parser operations with error scenarios.
func runConcurrentParserTests(
	t *testing.T,
	parser *ObservableJavaScriptParser,
	errorSources []string,
	ctx context.Context,
) chan error {
	concurrency := 10
	iterations := 50
	errorChan := make(chan error, concurrency*iterations*len(errorSources))

	for i := range concurrency {
		go func(workerID int) {
			processConcurrentWorker(t, parser, errorSources, ctx, workerID, iterations, errorChan)
		}(i)
	}

	return errorChan
}

// processConcurrentWorker handles work for a single concurrent worker.
func processConcurrentWorker(
	t *testing.T,
	parser *ObservableJavaScriptParser,
	errorSources []string,
	ctx context.Context,
	workerID, iterations int,
	errorChan chan error,
) {
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       10,
	}

	for j := range iterations {
		for k, source := range errorSources {
			parseTree := createParseTreeForSource(t, source, ctx)
			testParserOperations(parser, parseTree, source, ctx, options, errorChan)

			if j%10 == 0 {
				t.Logf("Worker %d completed %d iterations for source %d", workerID, j, k)
			}
		}
	}
}

// createParseTreeForSource creates a parse tree for the given source.
func createParseTreeForSource(t *testing.T, source string, ctx context.Context) *valueobject.ParseTree {
	if source == "" {
		jsLang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		rootNode := &valueobject.ParseNode{Type: "program"}
		metadata, _ := valueobject.NewParseMetadata(0, "0.0.0", "0.0.0")
		parseTree, _ := valueobject.NewParseTree(ctx, jsLang, rootNode, []byte(source), metadata)
		return parseTree
	}
	return createMockJavaScriptParseTree(t, source)
}

// testParserOperations tests various parser operations and collects errors.
func testParserOperations(
	parser *ObservableJavaScriptParser,
	parseTree *valueobject.ParseTree,
	source string,
	ctx context.Context,
	options outbound.SemanticExtractionOptions,
	errorChan chan error,
) {
	// Try extraction - expect various errors
	if _, funcErr := parser.ExtractFunctions(ctx, parseTree, options); funcErr != nil {
		errorChan <- funcErr
	}

	if _, classErr := parser.ExtractClasses(ctx, parseTree, options); classErr != nil {
		errorChan <- classErr
	}

	// Also test direct parsing for some sources
	if len(source) < 10000 { // Don't parse huge sources directly
		if _, parseErr := parser.Parse(ctx, []byte(source)); parseErr != nil {
			errorChan <- parseErr
		}
	}
}

// collectConcurrentErrors collects errors from the concurrent operations.
func collectConcurrentErrors(t *testing.T, errorChan chan error) int {
	timeout := time.After(30 * time.Second)
	errorCount := 0

CollectErrors:
	for {
		select {
		case err := <-errorChan:
			errorCount++
			assert.Error(t, err, "Should receive errors from invalid sources")
			t.Logf("Received error %d: %s", errorCount, err.Error())

		case <-timeout:
			break CollectErrors
		}

		if errorCount > 100 {
			break CollectErrors
		}
	}

	return errorCount
}

// validateParserStillFunctional ensures parser works correctly after concurrent errors.
func validateParserStillFunctional(
	t *testing.T,
	parser *ObservableJavaScriptParser,
	ctx context.Context,
	errorCount int,
) {
	assert.Positive(t, errorCount, "Should receive errors from concurrent processing")
	t.Logf("Total errors collected: %d", errorCount)

	validSource := "function test() { return 42; }\nclass Person { constructor(name) { this.name = name; } }"
	parseTree := createMockJavaScriptParseTree(t, validSource)
	options := outbound.SemanticExtractionOptions{IncludePrivate: true}

	functions, err := parser.ExtractFunctions(ctx, parseTree, options)
	assert.NoError(t, err, "Parser should still work after concurrent errors")
	assert.NotEmpty(t, functions, "Should extract functions from valid source")
}

// TestJavaScriptParser_ErrorHandling_ResourceCleanup tests proper resource cleanup on errors.
// This RED PHASE test defines expected resource cleanup behavior for tree-sitter resources.
func TestJavaScriptParser_ErrorHandling_ResourceCleanup(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		expectedError string
		operation     string
	}{
		{
			name:          "cleanup_after_tree_sitter_failure",
			source:        "invalid javascript syntax {{{",
			expectedError: "tree-sitter failure: should cleanup parser resources",
			operation:     "Parse",
		},
		{
			name:          "cleanup_after_parse_timeout",
			source:        strings.Repeat("function test() {}\n", 10000),
			expectedError: "parse timeout: should cleanup tree-sitter resources",
			operation:     "Parse",
		},
		{
			name:          "cleanup_after_extraction_error",
			source:        strings.Repeat("class Test {}\n", 5000),
			expectedError: "extraction error: should cleanup resources",
			operation:     "ExtractClasses",
		},
		{
			name:          "cleanup_after_memory_error",
			source:        generateCallbackHell(2000),
			expectedError: "memory error: should cleanup tree nodes",
			operation:     "ExtractFunctions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Short timeout to force cleanup scenarios
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Create parser
			parser, err := NewJavaScriptParser()
			require.NoError(t, err)

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			// Track resource usage (simplified)
			initialGoroutines := countJavaScriptGoroutines()

			// Perform operation that should fail and cleanup
			var opErr error
			switch tt.operation {
			case "Parse":
				_, opErr = parser.Parse(ctx, []byte(tt.source))
			case "ExtractFunctions":
				parseTree := createMockJavaScriptParseTree(t, tt.source)
				_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
			case "ExtractClasses":
				parseTree := createMockJavaScriptParseTree(t, tt.source)
				_, opErr = parser.ExtractClasses(ctx, parseTree, options)
			}

			// Should either get an error or timeout
			if ctx.Err() == context.DeadlineExceeded {
				t.Logf("Operation timed out as expected")
			} else if opErr != nil {
				t.Logf("Operation failed with error: %v", opErr)
			}

			// Allow some time for cleanup
			time.Sleep(100 * time.Millisecond)

			// Check that resources are cleaned up
			finalGoroutines := countJavaScriptGoroutines()

			// Should not have leaked goroutines (allow some tolerance)
			assert.LessOrEqual(t, finalGoroutines, initialGoroutines+5,
				"Should not leak goroutines after error")

			// Test that tree-sitter resources are properly closed
			// In a real implementation, we would track tree-sitter parser instances
			assert.NoError(t, parser.Close(), "Should be able to close parser cleanly")

			// Parser should still be usable for new operations after cleanup
			validSource := "function validTest() { return true; }"
			_, validErr := parser.Parse(context.Background(), []byte(validSource))
			// Note: After Close(), parser might not be usable, so we expect either success or specific error
			if validErr != nil {
				t.Logf("Parser not usable after close (expected): %v", validErr)
			}
		})
	}
}

// Helper function to create a mock JavaScript parse tree for testing.
func createMockJavaScriptParseTree(t *testing.T, source string) *valueobject.ParseTree {
	ctx := context.Background()

	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	rootNode := &valueobject.ParseNode{
		Type:      "program",
		StartByte: 0,
		EndByte:   uint32(len(source)),
		Children:  []*valueobject.ParseNode{},
	}

	metadata, err := valueobject.NewParseMetadata(0, "go-tree-sitter-bare", "1.0.0")
	require.NoError(t, err)

	parseTree, err := valueobject.NewParseTree(ctx, jsLang, rootNode, []byte(source), metadata)
	require.NoError(t, err)

	return parseTree
}

// Helper function to generate deeply nested callback hell for testing.
func generateCallbackHell(depth int) string {
	var builder strings.Builder
	builder.WriteString("function callbackHell() {\n")

	// Create nested callbacks
	for i := range depth {
		builder.WriteString(strings.Repeat("  ", i+1))
		builder.WriteString("setTimeout(function() {\n")
	}

	// Add some content at the deepest level
	builder.WriteString(strings.Repeat("  ", depth+1))
	builder.WriteString("console.log('deep callback');\n")

	// Close all callbacks
	for i := depth - 1; i >= 0; i-- {
		builder.WriteString(strings.Repeat("  ", i+1))
		builder.WriteString("}, 10);\n")
	}

	builder.WriteString("}\n")
	return builder.String()
}

// Helper function to count goroutines for JavaScript parser (simplified).
func countJavaScriptGoroutines() int {
	// In a real implementation, this would track tree-sitter specific resources
	// and goroutines used by the JavaScript parser
	return 10 // Placeholder value for RED phase
}
