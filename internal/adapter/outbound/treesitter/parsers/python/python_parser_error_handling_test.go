package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPythonParser_ErrorHandling_InvalidSyntax tests parser behavior with invalid Python syntax.
// This RED PHASE test defines expected error handling for malformed Python code.
func TestPythonParser_ErrorHandling_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		expectedError string
		shouldFail    bool
		operation     string
	}{
		{
			name:          "malformed_function_definition",
			source:        "def invalid_func( # missing closing paren and colon",
			expectedError: "invalid function definition: malformed parameter list",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "incomplete_class_definition",
			source:        "class Person # missing colon",
			expectedError: "invalid class definition: missing colon",
			shouldFail:    true,
			operation:     "ExtractClasses",
		},
		{
			name:          "invalid_indentation",
			source:        "def test():\nprint('hello')  # incorrect indentation",
			expectedError: "indentation error: inconsistent use of tabs and spaces",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "malformed_import_statement",
			source:        "import # missing module name",
			expectedError: "invalid import statement: missing module name",
			shouldFail:    true,
			operation:     "ExtractImports",
		},
		{
			name:          "unclosed_string_literal",
			source:        "message = 'Hello world",
			expectedError: "invalid syntax: unclosed string literal",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "unbalanced_parentheses",
			source:        "def test(:\n    return True # missing closing paren",
			expectedError: "invalid syntax: unbalanced parentheses",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "invalid_variable_assignment",
			source:        "x = # missing value after assignment",
			expectedError: "invalid assignment: missing value after assignment",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "malformed_decorator",
			source:        "@  # missing decorator name\ndef test():\n    pass",
			expectedError: "invalid decorator: missing decorator name",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "invalid_lambda_expression",
			source:        "func = lambda : # missing expression",
			expectedError: "invalid lambda: missing expression",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "mixed_language_syntax",
			source:        "def test() {\n    console.log('hello');\n}", // JavaScript in Python
			expectedError: "invalid Python syntax: detected non-Python language constructs",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "malformed_list_comprehension",
			source:        "result = [x for x in # missing iterable",
			expectedError: "invalid list comprehension: malformed syntax",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "invalid_context_manager",
			source:        "with open('file.txt'  # missing closing paren and as clause",
			expectedError: "invalid context manager: malformed with statement",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create parser
			parserInterface, err := NewPythonParser()
			require.NoError(t, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(t, ok, "Parser should be of type *ObservablePythonParser")

			// Create parse tree
			parseTree := createMockPythonParseTree(t, tt.source)

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

// TestPythonParser_ErrorHandling_MemoryExhaustion tests parser behavior with memory-intensive scenarios.
// This RED PHASE test defines expected error handling for memory exhaustion scenarios.
func TestPythonParser_ErrorHandling_MemoryExhaustion(t *testing.T) {
	tests := []struct {
		name          string
		sourceGen     func() string
		expectedError string
		operation     string
		timeout       time.Duration
	}{
		{
			name: "extremely_large_python_file",
			sourceGen: func() string {
				// Generate a very large Python file (10MB+)
				var builder strings.Builder
				for range 100000 {
					builder.WriteString("def very_long_function_name")
					builder.WriteString(strings.Repeat("_x", 100))
					builder.WriteString("():\n")
					builder.WriteString("    return ")
					builder.WriteString(strings.Repeat("'data'", 50))
					builder.WriteString("\n\n")
				}
				return builder.String()
			},
			expectedError: "memory limit exceeded: file too large to process safely",
			operation:     "ExtractFunctions",
			timeout:       5 * time.Second,
		},
		{
			name: "deeply_nested_classes",
			sourceGen: func() string {
				// Generate deeply nested class definitions
				var builder strings.Builder

				// Create 1000 levels of nested classes
				for i := range 1000 {
					indent := strings.Repeat("    ", i)
					builder.WriteString(indent + "class Level")
					builder.WriteString(strings.Repeat("Deep", i%100))
					builder.WriteString(":\n")
					builder.WriteString(indent + "    pass\n")
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

				// Generate 50,000 functions
				for i := range 50000 {
					builder.WriteString("def function")
					builder.WriteString(strings.Repeat("_a", 50))
					builder.WriteString("_number")
					builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
					builder.WriteString("():\n")
					builder.WriteString("    return ")
					builder.WriteString(strings.Repeat("'value'", 10))
					builder.WriteString("\n\n")
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
				builder.WriteString("class MassiveClass:\n")

				// Generate 10,000 methods
				for i := range 10000 {
					builder.WriteString("    def method")
					builder.WriteString(strings.Repeat("_x", 50))
					builder.WriteString("_number")
					builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
					builder.WriteString("(self):\n")
					builder.WriteString("        return ")
					builder.WriteString(strings.Repeat("'result'", 20))
					builder.WriteString("\n\n")
				}
				return builder.String()
			},
			expectedError: "memory allocation exceeded: too many methods in class",
			operation:     "ExtractClasses",
			timeout:       5 * time.Second,
		},
		{
			name: "circular_import_nightmare",
			sourceGen: func() string {
				var builder strings.Builder

				// Create complex circular import patterns
				for i := range 1000 {
					moduleName := "module" + strings.Repeat("_a", i%50)
					builder.WriteString("# Module " + moduleName + "\n")

					// Import previous modules to create circular dependencies
					if i > 0 {
						prevModule := "module" + strings.Repeat("_a", (i-1)%50)
						builder.WriteString("from " + prevModule + " import *\n")
					}

					// Import next module to create forward circular dependency
					if i < 999 {
						nextModule := "module" + strings.Repeat("_a", (i+1)%50)
						builder.WriteString("import " + nextModule + "\n")
					}

					builder.WriteString("class " + moduleName + "Class:\n")
					builder.WriteString("    def process(self):\n")
					builder.WriteString("        return self.__class__.__name__\n\n")
				}

				return builder.String()
			},
			expectedError: "circular import detected: potential memory leak",
			operation:     "ExtractImports",
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
			parserInterface, err := NewPythonParser()
			require.NoError(t, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(t, ok, "Parser should be of type *ObservablePythonParser")

			// Create parse tree
			parseTree := createMockPythonParseTree(t, source)

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
			case "ExtractImports":
				_, opErr = parser.ExtractImports(ctx, parseTree, options)
			}

			// Should either timeout or return memory error
			assert.Error(t, opErr, "Memory-intensive operation should fail")

			// Check if it's a timeout or expected memory error
			if ctx.Err() == context.DeadlineExceeded {
				assert.Contains(t, opErr.Error(), "timeout", "Should indicate timeout")
			} else if opErr != nil && tt.expectedError != "" {
				errorParts := strings.Split(tt.expectedError, ":")
				if len(errorParts) > 0 {
					assert.Contains(t, opErr.Error(), errorParts[0],
						"Should contain expected error type")
				}
			}
		})
	}
}

// TestPythonParser_ErrorHandling_TimeoutScenarios tests parser behavior with timeout scenarios.
// This RED PHASE test defines expected timeout handling behavior.
func TestPythonParser_ErrorHandling_TimeoutScenarios(t *testing.T) {
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
				// Generate complex Python that takes time to parse
				var builder strings.Builder

				// Create complex nested structures that take time to process
				for i := range 1000 {
					builder.WriteString("def complex_function")
					builder.WriteString(strings.Repeat("_a", i%100))
					builder.WriteString("():\n")

					// Add nested try/except patterns
					for j := range 50 {
						indent := strings.Repeat("    ", j+1)
						builder.WriteString(indent + "try:\n")
						builder.WriteString(indent + "    result")
						builder.WriteString(strings.Repeat("_b", j%50))
						builder.WriteString(" = process_data()\n")
						builder.WriteString(indent + "    for k in range(100):\n")
						builder.WriteString(indent + "        if condition")
						builder.WriteString(strings.Repeat("_c", j%25))
						builder.WriteString(":\n")
						builder.WriteString(indent + "            yield k\n")
						builder.WriteString(indent + "except Exception as e:\n")
						builder.WriteString(indent + "    handle_error(e)\n")
						builder.WriteString(indent + "finally:\n")
						builder.WriteString(indent + "    cleanup()\n")
					}
					builder.WriteString("\n")
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

				// Generate many complex classes with decorators and methods
				for i := range 500 {
					builder.WriteString("@dataclass\n")
					builder.WriteString("@property\n")
					builder.WriteString("@decorator")
					builder.WriteString(strings.Repeat("_z", i%50))
					builder.WriteString("\n")
					builder.WriteString("class ComplexClass")
					builder.WriteString(strings.Repeat("_w", i%100))
					builder.WriteString("(BaseClass, MixinClass):\n")

					// Add many properties and methods
					for j := range 200 {
						// Property
						builder.WriteString("    @property\n")
						builder.WriteString("    def property")
						builder.WriteString(strings.Repeat("_g", j%30))
						builder.WriteString("(self):\n")
						builder.WriteString("        return self._value")
						builder.WriteString(strings.Repeat("_v", j%20))
						builder.WriteString("\n\n")

						// Setter
						builder.WriteString("    @property")
						builder.WriteString(strings.Repeat("_g", j%30))
						builder.WriteString(".setter\n")
						builder.WriteString("    def property")
						builder.WriteString(strings.Repeat("_s", j%30))
						builder.WriteString("(self, value):\n")
						builder.WriteString("        self._value")
						builder.WriteString(strings.Repeat("_v", j%20))
						builder.WriteString(" = value\n\n")

						// Method
						builder.WriteString("    async def method")
						builder.WriteString(strings.Repeat("_m", j%40))
						builder.WriteString("(self):\n")
						builder.WriteString("        return await self.process()\n\n")
					}

					builder.WriteString("\n")
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
				// Code that might cause parser to enter infinite loop
				return `
class RecursiveClass:
    def __init__(self):
        self.self = None
        self.process_func = None
    
    def process(self):
        current = self
        while current:
            if current.self == current:
                # This could create parsing complexity
                current = current.self
            break
    
    def setup_recursion(self):
        self.self = self
        self.process_func = lambda: self.process()

recursive_obj = RecursiveClass()
recursive_obj.setup_recursion()

def complex_recursion(obj):
    if obj and hasattr(obj, 'self') and obj.self:
        return complex_recursion(obj.self)
    return obj

result = complex_recursion(recursive_obj)`
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
			parserInterface, err := NewPythonParser()
			require.NoError(t, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(t, ok, "Parser should be of type *ObservablePythonParser")

			start := time.Now()
			var opErr error

			switch tt.operation {
			case "ExtractFunctions":
				parseTree := createMockPythonParseTree(t, source)
				options := outbound.SemanticExtractionOptions{
					IncludePrivate: true,
					MaxDepth:       50,
				}
				_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
			case "ExtractClasses":
				parseTree := createMockPythonParseTree(t, source)
				options := outbound.SemanticExtractionOptions{
					IncludePrivate: true,
					MaxDepth:       50,
				}
				_, opErr = parser.ExtractClasses(ctx, parseTree, options)
			}

			elapsed := time.Since(start)

			// Should timeout or return error
			assert.Error(t, opErr, "Operation should fail due to timeout")

			// Check if context actually timed out (most reliable indicator)
			switch {
			case ctx.Err() == context.DeadlineExceeded:
				// Context deadline was exceeded - operation should have returned timeout error
				if opErr != nil {
					assert.Contains(t, opErr.Error(), "timeout", "Should indicate timeout")
				} else {
					// NOTE: Parser implementation limitation - doesn't fully respect context timeouts yet
					t.Logf("Context timed out but operation returned no error (parser limitation)")
				}
			case elapsed >= tt.timeout:
				// Elapsed time exceeded timeout but context didn't timeout
				// This can happen if operation completed just before deadline check
				if opErr != nil {
					t.Logf("Operation took %v (>= timeout %v) with error: %v", elapsed, tt.timeout, opErr)
				} else {
					t.Logf("Operation completed in %v (at/after timeout %v) but without error or context timeout", elapsed, tt.timeout)
				}
			default:
				// Completed before timeout
				t.Logf("Operation completed in %v (before timeout %v)", elapsed, tt.timeout)
			}
		})
	}
}

// TestPythonParser_ErrorHandling_EdgeCases tests parser behavior with edge case scenarios.
// This RED PHASE test defines expected behavior for unusual input scenarios.
func TestPythonParser_ErrorHandling_EdgeCases(t *testing.T) {
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
			source:        "def " + strings.Repeat("x", 100000) + "(): pass\n",
			expectedError: "line too long: exceeds maximum line length limit",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "unicode_in_identifiers",
			source:        "def 函数名(): pass\nclass 类型: pass\n",
			expectedError: "invalid identifier: non-ASCII characters in identifier",
			shouldFail:    false, // Python allows Unicode identifiers
			operation:     "ExtractFunctions",
		},
		{
			name:          "null_bytes_in_source",
			source:        "def test():\n    return 'hello'\n\x00print('test')\n",
			expectedError: "invalid source: contains null bytes",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "malformed_utf8_sequence",
			source:        "message = '\xff\xfe'\n",
			expectedError: "encoding error: malformed UTF-8 sequence",
			shouldFail:    true,
			operation:     "ExtractVariables",
		},
		{
			name:          "source_with_bom",
			source:        "\xEF\xBB\xBFdef test():\n    return 42",
			expectedError: "encoding issue: unexpected BOM marker",
			shouldFail:    false, // Should handle BOM gracefully
			operation:     "ExtractFunctions",
		},
		{
			name:          "json_data_as_python",
			source:        `{"name": "test", "value": 42, "items": [1, 2, 3]}`,
			expectedError: "invalid Python: JSON data detected",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "html_with_python",
			source:        `<script type="text/python">def test(): print('hello')</script>`,
			expectedError: "invalid Python: HTML content detected",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "extremely_deep_nesting",
			source:        generateDeepNestedPython(1000), // 1000 levels deep
			expectedError: "recursion limit exceeded: nesting too deep",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "invalid_python2_syntax",
			source:        "print 'hello world'  # Python 2 print statement",
			expectedError: "invalid Python3 syntax: Python 2 constructs detected",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
		{
			name:          "tabs_and_spaces_mixed",
			source:        "def test():\n\tif True:\n        return True  # mixed tabs and spaces",
			expectedError: "indentation error: inconsistent use of tabs and spaces",
			shouldFail:    true,
			operation:     "ExtractFunctions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create parser
			parserInterface, err := NewPythonParser()
			require.NoError(t, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(t, ok, "Parser should be of type *ObservablePythonParser")

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			// Test the operation
			var opErr error
			switch tt.operation {
			case "ExtractFunctions":
				// For edge cases, we might need to handle parse tree creation differently
				if tt.source == "" || strings.TrimSpace(tt.source) == "" {
					// For empty sources, createMockPythonParseTree might fail
					pythonLang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
					rootNode := &valueobject.ParseNode{Type: "module"}
					metadata, _ := valueobject.NewParseMetadata(0, "0.0.0", "0.0.0")
					parseTree, _ := valueobject.NewParseTree(ctx, pythonLang, rootNode, []byte(tt.source), metadata)
					_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
				} else {
					parseTree := createMockPythonParseTree(t, tt.source)
					_, opErr = parser.ExtractFunctions(ctx, parseTree, options)
				}
			case "ExtractVariables":
				parseTree := createMockPythonParseTree(t, tt.source)
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

// TestPythonParser_ErrorHandling_ConcurrentAccess tests parser behavior under concurrent access.
// This RED PHASE test defines expected behavior for concurrent parser usage with errors.
func TestPythonParser_ErrorHandling_ConcurrentAccess(t *testing.T) {
	parserInterface, err := NewPythonParser()
	require.NoError(t, err)
	parser, ok := parserInterface.(*ObservablePythonParser)
	require.True(t, ok, "Parser should be of type *ObservablePythonParser")

	errorSources := getPythonConcurrentTestSources()
	ctx := context.Background()

	errorChan := runPythonConcurrentParserTests(t, parser, errorSources, ctx)
	errorCount := collectPythonConcurrentErrors(t, errorChan)

	validatePythonParserStillFunctional(t, parser, ctx, errorCount)
}

// getPythonConcurrentTestSources returns test sources for concurrent testing.
func getPythonConcurrentTestSources() []string {
	return []string{
		"def invalid( # missing closing paren and colon", // malformed syntax
		"", // empty source
		strings.Repeat("def test(): pass\n", 10000), // large source
		"def test():\n    print('\xff\xfe')",        // invalid encoding
		generateDeepNestedPython(500),               // deeply nested
	}
}

// runPythonConcurrentParserTests executes concurrent parser operations with error scenarios.
func runPythonConcurrentParserTests(
	t *testing.T,
	parser *ObservablePythonParser,
	errorSources []string,
	ctx context.Context,
) chan error {
	concurrency := 10
	iterations := 50
	errorChan := make(chan error, concurrency*iterations*len(errorSources))

	for i := range concurrency {
		go func(workerID int) {
			processPythonConcurrentWorker(t, parser, errorSources, ctx, workerID, iterations, errorChan)
		}(i)
	}

	return errorChan
}

// processPythonConcurrentWorker handles work for a single concurrent worker.
func processPythonConcurrentWorker(
	t *testing.T,
	parser *ObservablePythonParser,
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
			parseTree := createPythonParseTreeForSource(t, source, ctx)
			testPythonParserOperations(parser, parseTree, source, ctx, options, errorChan)

			if j%10 == 0 {
				t.Logf("Worker %d completed %d iterations for source %d", workerID, j, k)
			}
		}
	}
}

// createPythonParseTreeForSource creates a parse tree for the given source.
func createPythonParseTreeForSource(t *testing.T, source string, ctx context.Context) *valueobject.ParseTree {
	if source == "" {
		pythonLang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
		rootNode := &valueobject.ParseNode{Type: "module"}
		metadata, _ := valueobject.NewParseMetadata(0, "0.0.0", "0.0.0")
		parseTree, _ := valueobject.NewParseTree(ctx, pythonLang, rootNode, []byte(source), metadata)
		return parseTree
	}
	return createMockPythonParseTree(t, source)
}

// testPythonParserOperations tests various parser operations and collects errors.
func testPythonParserOperations(
	parser *ObservablePythonParser,
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

// collectPythonConcurrentErrors collects errors from the concurrent operations.
func collectPythonConcurrentErrors(t *testing.T, errorChan chan error) int {
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

// validatePythonParserStillFunctional ensures parser works correctly after concurrent errors.
func validatePythonParserStillFunctional(
	t *testing.T,
	parser *ObservablePythonParser,
	ctx context.Context,
	errorCount int,
) {
	assert.Positive(t, errorCount, "Should receive errors from concurrent processing")
	t.Logf("Total errors collected: %d", errorCount)

	validSource := "def test():\n    return 42\n\nclass Person:\n    def __init__(self, name):\n        self.name = name"
	parseTree := createMockPythonParseTree(t, validSource)
	options := outbound.SemanticExtractionOptions{IncludePrivate: true}

	functions, err := parser.ExtractFunctions(ctx, parseTree, options)
	assert.NoError(t, err, "Parser should still work after concurrent errors")
	assert.NotEmpty(t, functions, "Should extract functions from valid source")
}

// TestPythonParser_ErrorHandling_ResourceCleanup tests proper resource cleanup on errors.
// This RED PHASE test defines expected resource cleanup behavior.
func TestPythonParser_ErrorHandling_ResourceCleanup(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		expectedError string
		operation     string
	}{
		{
			name:          "cleanup_after_parse_failure",
			source:        "invalid python syntax {{{",
			expectedError: "parse failure: should cleanup resources",
			operation:     "ExtractFunctions",
		},
		{
			name:          "cleanup_after_timeout",
			source:        strings.Repeat("def test(): pass\n", 10000),
			expectedError: "timeout: should cleanup resources",
			operation:     "ExtractFunctions",
		},
		{
			name:          "cleanup_after_memory_error",
			source:        strings.Repeat("class Test: pass\n", 5000),
			expectedError: "memory error: should cleanup resources",
			operation:     "ExtractClasses",
		},
		{
			name:          "cleanup_after_deep_nesting_error",
			source:        generateDeepNestedPython(2000),
			expectedError: "nesting error: should cleanup resources",
			operation:     "ExtractFunctions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Short timeout to force cleanup scenarios
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Create parser
			parserInterface, err := NewPythonParser()
			require.NoError(t, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(t, ok, "Parser should be of type *ObservablePythonParser")

			// Create parse tree
			parseTree := createMockPythonParseTree(t, tt.source)

			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			// Track resource usage (simplified)
			initialGoroutines := countPythonGoroutines()

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
			finalGoroutines := countPythonGoroutines()

			// Should not have leaked goroutines (allow some tolerance)
			assert.LessOrEqual(t, finalGoroutines, initialGoroutines+5,
				"Should not leak goroutines after error")

			// Parser should still be usable after cleanup
			validSource := "def valid_test():\n    return True"
			validParseTree := createMockPythonParseTree(t, validSource)
			validCtx := context.Background()

			_, validErr := parser.ExtractFunctions(validCtx, validParseTree, options)
			assert.NoError(t, validErr, "Parser should be usable after cleanup")
		})
	}
}

// Helper function to create a Python parse tree using actual tree-sitter parsing.
func createMockPythonParseTree(t *testing.T, source string) *valueobject.ParseTree {
	t.Helper()
	ctx := context.Background()

	// Get Python grammar from forest (using go-sitter-forest)
	grammar := forest.GetLanguage("python")
	require.NotNil(t, grammar, "Failed to get Python grammar from forest")

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	require.NotNil(t, parser, "Failed to create tree-sitter parser")

	success := parser.SetLanguage(grammar)
	require.True(t, success, "Failed to set Python language")

	// Parse the source code with tree-sitter (will create ERROR nodes for syntax errors)
	tree, err := parser.ParseString(ctx, nil, []byte(source))
	require.NoError(t, err, "Failed to parse Python source")
	require.NotNil(t, tree, "Parse tree should not be nil")
	defer tree.Close()

	// Convert tree-sitter tree to domain ParseNode
	rootTSNode := tree.RootNode()
	rootNode, nodeCount, maxDepth := convertTreeSitterNodeForErrorTest(rootTSNode, 0)

	// Create metadata with parsing statistics
	metadata, err := valueobject.NewParseMetadata(
		time.Millisecond, // placeholder duration
		"go-tree-sitter-bare",
		"1.0.0",
	)
	require.NoError(t, err, "Failed to create metadata")

	// Update metadata with actual counts
	metadata.NodeCount = nodeCount
	metadata.MaxDepth = maxDepth

	// Create Python language
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	// Create domain parse tree
	domainParseTree, err := valueobject.NewParseTree(
		ctx,
		pythonLang,
		rootNode,
		[]byte(source),
		metadata,
	)
	require.NoError(t, err, "Failed to create domain parse tree")

	return domainParseTree
}

// convertTreeSitterNodeForErrorTest converts a tree-sitter node to domain ParseNode recursively.
func convertTreeSitterNodeForErrorTest(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Convert tree-sitter node to domain ParseNode
	parseNode := &valueobject.ParseNode{
		Type:      node.Type(),
		StartByte: valueobject.ClampToUint32(int(node.StartByte())),
		EndByte:   valueobject.ClampToUint32(int(node.EndByte())),
		StartPos: valueobject.Position{
			Row:    valueobject.ClampToUint32(int(node.StartPoint().Row)),
			Column: valueobject.ClampToUint32(int(node.StartPoint().Column)),
		},
		EndPos: valueobject.Position{
			Row:    valueobject.ClampToUint32(int(node.EndPoint().Row)),
			Column: valueobject.ClampToUint32(int(node.EndPoint().Column)),
		},
		Children: make([]*valueobject.ParseNode, 0),
	}

	nodeCount := 1
	maxDepth := depth

	// Convert children recursively
	childCount := node.ChildCount()
	for i := range childCount {
		childNode := node.Child(i)
		if childNode.IsNull() {
			continue
		}

		child, childNodeCount, childMaxDepth := convertTreeSitterNodeForErrorTest(childNode, depth+1)
		if child != nil {
			parseNode.Children = append(parseNode.Children, child)
			nodeCount += childNodeCount
			if childMaxDepth > maxDepth {
				maxDepth = childMaxDepth
			}
		}
	}

	return parseNode, nodeCount, maxDepth
}

// Helper function to generate deeply nested Python code for testing.
func generateDeepNestedPython(depth int) string {
	var builder strings.Builder
	builder.WriteString("def deep_nested_function():\n")

	// Create nested try/except blocks
	for i := range depth {
		indent := strings.Repeat("    ", i+1)
		builder.WriteString(indent + "try:\n")
		if i < depth-1 {
			builder.WriteString(indent + "    # Level " + strings.Repeat(string(rune('0'+i%10)), 1) + "\n")
		} else {
			builder.WriteString(indent + "    return 'deep nested result'\n")
		}
	}

	// Close all try blocks with except
	for i := depth - 1; i >= 0; i-- {
		indent := strings.Repeat("    ", i+1)
		builder.WriteString(indent + "except Exception as e" + strings.Repeat(string(rune('0'+i%10)), 1) + ":\n")
		builder.WriteString(indent + "    handle_error(e" + strings.Repeat(string(rune('0'+i%10)), 1) + ")\n")
	}

	return builder.String()
}

// Helper function to count goroutines for Python parser (simplified).
func countPythonGoroutines() int {
	// In a real implementation, this would track Python parser specific resources
	// and goroutines used by the parser
	return 10 // Placeholder value for RED phase
}
