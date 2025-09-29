package goparser

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTreeSitterNativeErrorDetectionMethods tests that error detection uses
// tree-sitter's native error detection methods: HasError(), IsError(), and IsMissing()
// instead of only checking node.Type == "ERROR".
func TestTreeSitterNativeErrorDetectionMethods(t *testing.T) {
	tests := []struct {
		name               string
		sourceCode         string
		expectedHasError   bool
		expectedIsError    bool
		expectedIsMissing  bool
		expectedErrorNodes []string
		description        string
	}{
		{
			name:               "valid Go code has no errors",
			sourceCode:         "package main\n\nfunc main() {\n    fmt.Println(\"hello\")\n}",
			expectedHasError:   false,
			expectedIsError:    false,
			expectedIsMissing:  false,
			expectedErrorNodes: []string{},
			description:        "Should use tree-sitter native methods to detect no errors in valid code",
		},
		{
			name:               "unclosed brace detected by HasError",
			sourceCode:         "package main\n\nfunc main() {\n    fmt.Println(\"hello\")",
			expectedHasError:   true,
			expectedIsError:    false,
			expectedIsMissing:  true,
			expectedErrorNodes: []string{"MISSING", "ERROR"},
			description:        "Should use HasError() to detect unclosed braces and IsMissing() for missing tokens",
		},
		{
			name:               "invalid package name detected by IsError",
			sourceCode:         "package 123invalid\n\nfunc main() {}",
			expectedHasError:   true,
			expectedIsError:    true,
			expectedIsMissing:  false,
			expectedErrorNodes: []string{"ERROR"},
			description:        "Should use IsError() to detect invalid identifiers",
		},
		{
			name:               "incomplete function detected by multiple error methods",
			sourceCode:         "package main\n\nfunc incomplete(",
			expectedHasError:   true,
			expectedIsError:    true,
			expectedIsMissing:  true,
			expectedErrorNodes: []string{"ERROR", "MISSING"},
			description:        "Should use multiple tree-sitter error methods for incomplete constructs",
		},
		{
			name:               "malformed struct detected by comprehensive error detection",
			sourceCode:         "package main\n\ntype BadStruct struct {\n    field string",
			expectedHasError:   true,
			expectedIsError:    false,
			expectedIsMissing:  true,
			expectedErrorNodes: []string{"MISSING"},
			description:        "Should detect malformed structs using comprehensive tree-sitter error analysis",
		},
		{
			name:               "syntax error with invalid tokens detected by IsError",
			sourceCode:         "package main\n\nfunc main() {\n    @ # $ invalid tokens\n}",
			expectedHasError:   true,
			expectedIsError:    true,
			expectedIsMissing:  false,
			expectedErrorNodes: []string{"ERROR"},
			description:        "Should use IsError() to detect invalid token sequences",
		},
		{
			name:               "missing semicolon detected by IsMissing",
			sourceCode:         "package main\n\nfunc main() {\n    x := 5\n    y := 10\n    return x + y",
			expectedHasError:   true,
			expectedIsError:    false,
			expectedIsMissing:  true,
			expectedErrorNodes: []string{"MISSING"},
			description:        "Should use IsMissing() to detect missing required tokens",
		},
		{
			name:               "unmatched parentheses detected by comprehensive error detection",
			sourceCode:         "package main\n\nfunc main() {\n    fmt.Println(\"hello\"\n}",
			expectedHasError:   true,
			expectedIsError:    false,
			expectedIsMissing:  true,
			expectedErrorNodes: []string{"MISSING", "ERROR"},
			description:        "Should use comprehensive error detection for unmatched delimiters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test expects error detection to use tree-sitter's native methods:
			// 1. HasError() - check if any part of the tree has errors
			// 2. IsError() - check if specific nodes are error nodes
			// 3. IsMissing() - check if required tokens are missing
			result := parser.detectErrorsWithNativeMethods(tt.sourceCode)

			// Verify HasError() detection
			assert.Equal(t, tt.expectedHasError, result.HasError,
				"HasError() should match expected result: %s", tt.description)

			// Verify IsError() detection
			assert.Equal(t, tt.expectedIsError, result.IsError,
				"IsError() should match expected result: %s", tt.description)

			// Verify IsMissing() detection
			assert.Equal(t, tt.expectedIsMissing, result.IsMissing,
				"IsMissing() should match expected result: %s", tt.description)

			// Verify specific error node types are detected
			for _, expectedNode := range tt.expectedErrorNodes {
				assert.Contains(t, result.ErrorNodeTypes, expectedNode,
					"Should detect error node type %s: %s", expectedNode, tt.description)
			}
		})
	}
}

// TestTreeSitterErrorMessageContent tests that error detection provides
// detailed error messages with specific information about the syntax errors found.
func TestTreeSitterErrorMessageContent(t *testing.T) {
	tests := []struct {
		name                 string
		sourceCode           string
		expectedErrorMessage string
		expectedErrorType    string
		expectedErrorLine    int
		expectedErrorColumn  int
		description          string
	}{
		{
			name:                 "unclosed brace error with position info",
			sourceCode:           "package main\n\nfunc main() {\n    fmt.Println(\"hello\")",
			expectedErrorMessage: "missing closing brace",
			expectedErrorType:    "MISSING_TOKEN",
			expectedErrorLine:    4,
			expectedErrorColumn:  26,
			description:          "Should provide detailed error message with position for unclosed braces",
		},
		{
			name:                 "invalid identifier error with context",
			sourceCode:           "package 123invalid\n\nfunc main() {}",
			expectedErrorMessage: "invalid identifier: identifier cannot start with digit",
			expectedErrorType:    "INVALID_IDENTIFIER",
			expectedErrorLine:    1,
			expectedErrorColumn:  9,
			description:          "Should provide contextual error message for invalid identifiers",
		},
		{
			name:                 "incomplete function with specific error",
			sourceCode:           "package main\n\nfunc incomplete(",
			expectedErrorMessage: "incomplete function declaration: missing parameter list closing",
			expectedErrorType:    "INCOMPLETE_FUNCTION",
			expectedErrorLine:    3,
			expectedErrorColumn:  16,
			description:          "Should provide specific error message for incomplete functions",
		},
		{
			name:                 "malformed struct with field error",
			sourceCode:           "package main\n\ntype BadStruct struct {\n    field string",
			expectedErrorMessage: "incomplete struct declaration: missing closing brace",
			expectedErrorType:    "INCOMPLETE_STRUCT",
			expectedErrorLine:    4,
			expectedErrorColumn:  17,
			description:          "Should provide specific error message for malformed structs",
		},
		{
			name:                 "unexpected token error with recovery suggestion",
			sourceCode:           "package main\n\nfunc main() {\n    @ unexpected\n}",
			expectedErrorMessage: "unexpected token '@': expected identifier or expression",
			expectedErrorType:    "UNEXPECTED_TOKEN",
			expectedErrorLine:    4,
			expectedErrorColumn:  5,
			description:          "Should provide recovery suggestions for unexpected tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test expects detailed error reporting using tree-sitter's
			// position information and grammar-aware error classification
			errorResult := parser.analyzeErrorWithDetails(tt.sourceCode)

			require.Error(t, errorResult.Error, "Should detect syntax error: %s", tt.description)

			// Verify error message content
			assert.Contains(t, errorResult.Error.Error(), tt.expectedErrorMessage,
				"Error message should contain expected content: %s", tt.description)

			// Verify error type classification
			assert.Equal(t, tt.expectedErrorType, errorResult.ErrorType,
				"Error type should match expected classification: %s", tt.description)

			// Verify error position information
			assert.Equal(t, tt.expectedErrorLine, errorResult.Line,
				"Error line should match expected position: %s", tt.description)
			assert.Equal(t, tt.expectedErrorColumn, errorResult.Column,
				"Error column should match expected position: %s", tt.description)
		})
	}
}

// TestTreeSitterErrorRecoveryAndReporting tests that error detection can
// recover from errors and continue parsing to find multiple issues.
func TestTreeSitterErrorRecoveryAndReporting(t *testing.T) {
	tests := []struct {
		name                string
		sourceCode          string
		expectedErrorCount  int
		expectedErrorTypes  []string
		expectedCanRecover  bool
		expectedSuggestions []string
		description         string
	}{
		{
			name: "multiple syntax errors with recovery",
			sourceCode: `package main

func main() {
    fmt.Println("hello"
    @ invalid token
    missing brace`,
			expectedErrorCount:  3,
			expectedErrorTypes:  []string{"MISSING_PAREN", "UNEXPECTED_TOKEN", "MISSING_BRACE"},
			expectedCanRecover:  true,
			expectedSuggestions: []string{"add closing parenthesis", "remove invalid token", "add closing brace"},
			description:         "Should detect multiple errors and provide recovery suggestions",
		},
		{
			name: "cascading errors with limited recovery",
			sourceCode: `package 123invalid

func incomplete(
    @ # $ multiple errors`,
			expectedErrorCount: 4,
			expectedErrorTypes: []string{
				"INVALID_PACKAGE",
				"INCOMPLETE_FUNCTION",
				"UNEXPECTED_TOKEN",
				"UNEXPECTED_TOKEN",
			},
			expectedCanRecover:  false,
			expectedSuggestions: []string{"fix package name", "complete function declaration"},
			description:         "Should detect cascading errors and indicate limited recovery",
		},
		{
			name: "structural errors with good recovery potential",
			sourceCode: `package main

type BadStruct struct {
    field string
    // missing closing brace

func main() {
    // this should still be parseable
}`,
			expectedErrorCount:  1,
			expectedErrorTypes:  []string{"MISSING_STRUCT_BRACE"},
			expectedCanRecover:  true,
			expectedSuggestions: []string{"add closing brace for struct"},
			description:         "Should detect structural errors while allowing recovery for later constructs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test expects comprehensive error analysis with recovery information
			recoveryResult := parser.analyzeErrorsWithRecovery(tt.sourceCode)

			// Verify error count
			assert.Len(t, recoveryResult.Errors, tt.expectedErrorCount,
				"Should detect expected number of errors: %s", tt.description)

			// Verify error types
			actualErrorTypes := make([]string, len(recoveryResult.Errors))
			for i, err := range recoveryResult.Errors {
				actualErrorTypes[i] = err.Type
			}
			assert.ElementsMatch(t, tt.expectedErrorTypes, actualErrorTypes,
				"Should detect expected error types: %s", tt.description)

			// Verify recovery potential
			assert.Equal(t, tt.expectedCanRecover, recoveryResult.CanRecover,
				"Recovery potential should match expected: %s", tt.description)

			// Verify suggestions are provided
			assert.Len(t, recoveryResult.Suggestions, len(tt.expectedSuggestions),
				"Should provide expected number of suggestions: %s", tt.description)

			for _, expectedSuggestion := range tt.expectedSuggestions {
				found := false
				for _, suggestion := range recoveryResult.Suggestions {
					if strings.Contains(suggestion, expectedSuggestion) {
						found = true
						break
					}
				}
				assert.True(t, found,
					"Should provide suggestion containing '%s': %s", expectedSuggestion, tt.description)
			}
		})
	}
}

// TestTreeSitterErrorDetectionPerformance tests that error detection is
// efficient and doesn't cause performance issues with large files or
// files with many errors.
func TestTreeSitterErrorDetectionPerformance(t *testing.T) {
	tests := []struct {
		name               string
		sourceCode         string
		maxParseTimeMs     int64
		maxErrorAnalysisMs int64
		expectedComplexity string
		description        string
	}{
		{
			name:               "large file with single error",
			sourceCode:         generateLargeGoFileWithSingleError(1000),
			maxParseTimeMs:     100,
			maxErrorAnalysisMs: 50,
			expectedComplexity: "O(n)",
			description:        "Should efficiently detect single error in large file",
		},
		{
			name:               "file with many small errors",
			sourceCode:         generateGoFileWithManyErrors(50),
			maxParseTimeMs:     200,
			maxErrorAnalysisMs: 100,
			expectedComplexity: "O(n*m)",
			description:        "Should efficiently detect multiple errors without exponential slowdown",
		},
		{
			name:               "deeply nested structure with errors",
			sourceCode:         generateDeeplyNestedGoWithErrors(20),
			maxParseTimeMs:     150,
			maxErrorAnalysisMs: 75,
			expectedComplexity: "O(n*d)",
			description:        "Should efficiently handle deeply nested structures with errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test expects efficient error detection that completes within time limits
			performanceResult := parser.analyzeErrorsWithPerformanceTracking(tt.sourceCode)

			// Verify parse time is within limits
			assert.LessOrEqual(t, performanceResult.ParseTimeMs, tt.maxParseTimeMs,
				"Parse time should be within limits: %s", tt.description)

			// Verify error analysis time is within limits
			assert.LessOrEqual(t, performanceResult.ErrorAnalysisTimeMs, tt.maxErrorAnalysisMs,
				"Error analysis time should be within limits: %s", tt.description)

			// Verify complexity class
			assert.Equal(t, tt.expectedComplexity, performanceResult.ComplexityClass,
				"Should have expected algorithmic complexity: %s", tt.description)

			// Verify that errors were still detected despite performance constraints
			assert.NotEmpty(t, performanceResult.ErrorsDetected,
				"Should still detect errors despite performance optimization: %s", tt.description)
		})
	}
}

// TestTreeSitterErrorDetectionIntegration tests that the enhanced error detection
// integrates properly with the existing GoParser validation pipeline.
func TestTreeSitterErrorDetectionIntegration(t *testing.T) {
	tests := []struct {
		name                    string
		sourceCode              string
		expectValidationToFail  bool
		expectParsingToFail     bool
		expectChunkingToFail    bool
		expectedValidationError string
		description             string
	}{
		{
			name:                    "valid code passes all stages",
			sourceCode:              "package main\n\nfunc main() {\n    fmt.Println(\"hello\")\n}",
			expectValidationToFail:  false,
			expectParsingToFail:     false,
			expectChunkingToFail:    false,
			expectedValidationError: "",
			description:             "Valid code should pass through entire pipeline without errors",
		},
		{
			name:                    "syntax error fails validation but allows limited parsing",
			sourceCode:              "package main\n\nfunc main() {\n    fmt.Println(\"hello\"",
			expectValidationToFail:  true,
			expectParsingToFail:     false,
			expectChunkingToFail:    true,
			expectedValidationError: "syntax error: missing closing parenthesis",
			description:             "Syntax errors should fail validation but allow some parsing for analysis",
		},
		{
			name:                    "severe errors fail all stages",
			sourceCode:              "@ # $ completely invalid",
			expectValidationToFail:  true,
			expectParsingToFail:     true,
			expectChunkingToFail:    true,
			expectedValidationError: "syntax error: invalid token sequence",
			description:             "Severe syntax errors should fail all pipeline stages",
		},
		{
			name:                    "recoverable errors allow partial processing",
			sourceCode:              "package main\n\ntype BadStruct struct {\n    field string\n\nfunc main() {}",
			expectValidationToFail:  true,
			expectParsingToFail:     false,
			expectChunkingToFail:    false,
			expectedValidationError: "syntax error: incomplete struct declaration",
			description:             "Recoverable errors should allow partial processing of valid parts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// Test validation stage integration
			validationErr := parser.validateSyntaxWithEnhancedTreeSitter(tt.sourceCode)
			if tt.expectValidationToFail {
				assert.Error(t, validationErr, "Validation should fail: %s", tt.description)
				if tt.expectedValidationError != "" {
					assert.Contains(t, validationErr.Error(), tt.expectedValidationError,
						"Validation error should contain expected message: %s", tt.description)
				}
			} else {
				assert.NoError(t, validationErr, "Validation should pass: %s", tt.description)
			}

			// Test parsing stage integration
			ctx := context.Background()
			parseResult, parseErr := parser.parseWithEnhancedErrorDetection(ctx, tt.sourceCode)
			if tt.expectParsingToFail {
				assert.Error(t, parseErr, "Parsing should fail: %s", tt.description)
			} else {
				assert.NoError(t, parseErr, "Parsing should succeed: %s", tt.description)
				assert.NotNil(t, parseResult, "Parse result should not be nil: %s", tt.description)
			}

			// Test chunking stage integration (if parsing succeeded)
			if !tt.expectParsingToFail && parseResult != nil {
				chunks, chunkErr := parser.extractChunksWithErrorHandling(ctx, parseResult)
				if tt.expectChunkingToFail {
					assert.Error(t, chunkErr, "Chunking should fail: %s", tt.description)
				} else {
					assert.NoError(t, chunkErr, "Chunking should succeed: %s", tt.description)
					// For valid code, we should get some chunks
					if !tt.expectValidationToFail {
						assert.NotEmpty(t, chunks, "Should extract some chunks from valid code: %s", tt.description)
					}
				}
			}
		})
	}
}

// Helper types and methods that the tests expect to exist but don't yet

// PerformanceResult represents performance analysis results.
type PerformanceResult struct {
	ParseTimeMs         int64
	ErrorAnalysisTimeMs int64
	ComplexityClass     string
	ErrorsDetected      []string
}

// Stub methods that will need to be implemented in the actual GoParser

func (p *GoParser) analyzeErrorsWithPerformanceTracking(sourceCode string) *PerformanceResult {
	// This method should track performance metrics during error analysis
	// Implementation will be done in the GREEN phase
	return &PerformanceResult{
		ParseTimeMs:         0,
		ErrorAnalysisTimeMs: 0,
		ComplexityClass:     "",
		ErrorsDetected:      []string{},
	}
}

func (p *GoParser) parseWithEnhancedErrorDetection(
	ctx context.Context,
	sourceCode string,
) (*valueobject.ParseTree, error) {
	// This method should parse with enhanced error detection and recovery
	// Implementation will be done in the GREEN phase
	// For now, delegate to the existing Parse method to avoid always returning nil
	if sourceCode == "" {
		return nil, errors.New("source code is empty")
	}

	// Use existing parse method as a fallback until enhanced implementation is done
	return p.Parse(ctx, sourceCode)
}

func (p *GoParser) extractChunksWithErrorHandling(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
) ([]interface{}, error) {
	// This method should extract chunks with proper error handling
	// Implementation will be done in the GREEN phase
	if parseTree == nil {
		return nil, errors.New("parse tree is nil")
	}
	return []interface{}{}, nil
}

// Helper functions to generate test data

func generateLargeGoFileWithSingleError(lines int) string {
	var sb strings.Builder
	sb.WriteString("package main\n\n")

	for range lines - 10 {
		sb.WriteString("// Comment line\n")
	}

	sb.WriteString("func main() {\n")
	sb.WriteString("    fmt.Println(\"hello\"\n") // Missing closing parenthesis
	sb.WriteString("}")

	return sb.String()
}

func generateGoFileWithManyErrors(errorCount int) string {
	var sb strings.Builder
	sb.WriteString("package main\n\n")

	for i := range errorCount {
		sb.WriteString("func error")
		sb.WriteRune(rune('0' + i%10))
		sb.WriteString("( // Missing closing parenthesis\n")
	}

	return sb.String()
}

func generateDeeplyNestedGoWithErrors(depth int) string {
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	sb.WriteString("func main() {\n")

	// Create deeply nested blocks
	for range depth {
		sb.WriteString("    if true {\n")
	}

	sb.WriteString("        @ invalid token\n") // Error in deepest level

	// Close most blocks but leave one unclosed (error)
	for range depth - 1 {
		sb.WriteString("    }\n")
	}

	sb.WriteString("}")

	return sb.String()
}
