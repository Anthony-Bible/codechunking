package goparser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSpecificSyntaxErrorPatterns tests that the enhanced error detection correctly
// identifies and reports specific Go syntax error patterns that should be caught
// by tree-sitter's native error detection methods.
func TestSpecificSyntaxErrorPatterns(t *testing.T) {
	tests := []struct {
		name                  string
		sourceCode            string
		expectedSyntaxError   bool
		expectedErrorType     string
		expectedErrorLocation string
		expectedRecoveryHint  string
		shouldUseHasError     bool
		shouldUseIsError      bool
		shouldUseIsMissing    bool
		description           string
	}{
		{
			name:                  "unclosed brace in function",
			sourceCode:            "package main\n\nfunc main() {\n    fmt.Println(\"hello\")",
			expectedSyntaxError:   true,
			expectedErrorType:     "MISSING_BRACE",
			expectedErrorLocation: "end of function body",
			expectedRecoveryHint:  "add closing brace '}'",
			shouldUseHasError:     true,
			shouldUseIsError:      false,
			shouldUseIsMissing:    true,
			description:           "Should detect unclosed braces using HasError() and IsMissing()",
		},
		{
			name:                  "invalid package name starting with digit",
			sourceCode:            "package 123invalid\n\nfunc main() {}",
			expectedSyntaxError:   true,
			expectedErrorType:     "INVALID_IDENTIFIER",
			expectedErrorLocation: "package declaration",
			expectedRecoveryHint:  "package name must start with letter or underscore",
			shouldUseHasError:     true,
			shouldUseIsError:      true,
			shouldUseIsMissing:    false,
			description:           "Should detect invalid identifiers using HasError() and IsError()",
		},
		{
			name:                  "incomplete function declaration missing parameters",
			sourceCode:            "package main\n\nfunc incomplete(",
			expectedSyntaxError:   true,
			expectedErrorType:     "INCOMPLETE_FUNCTION",
			expectedErrorLocation: "function parameter list",
			expectedRecoveryHint:  "complete function parameter list and body",
			shouldUseHasError:     true,
			shouldUseIsError:      true,
			shouldUseIsMissing:    true,
			description:           "Should detect incomplete functions using all tree-sitter error methods",
		},
		{
			name:                  "malformed struct missing closing brace",
			sourceCode:            "package main\n\ntype BadStruct struct {\n    field string",
			expectedSyntaxError:   true,
			expectedErrorType:     "INCOMPLETE_STRUCT",
			expectedErrorLocation: "struct definition",
			expectedRecoveryHint:  "add closing brace '}' to complete struct",
			shouldUseHasError:     true,
			shouldUseIsError:      false,
			shouldUseIsMissing:    true,
			description:           "Should detect incomplete structs using HasError() and IsMissing()",
		},
		{
			name:                  "unclosed string literal",
			sourceCode:            "package main\n\nfunc main() {\n    fmt.Println(\"unclosed string\n}",
			expectedSyntaxError:   true,
			expectedErrorType:     "UNCLOSED_STRING",
			expectedErrorLocation: "string literal",
			expectedRecoveryHint:  "add closing quote to string literal",
			shouldUseHasError:     true,
			shouldUseIsError:      true,
			shouldUseIsMissing:    false,
			description:           "Should detect unclosed strings using HasError() and IsError()",
		},
		{
			name:                  "missing semicolon in statement",
			sourceCode:            "package main\n\nfunc main() {\n    x := 5\n    y := 10\n    return x + y",
			expectedSyntaxError:   true,
			expectedErrorType:     "MISSING_DELIMITER",
			expectedErrorLocation: "return statement",
			expectedRecoveryHint:  "add missing delimiter or newline",
			shouldUseHasError:     true,
			shouldUseIsError:      false,
			shouldUseIsMissing:    true,
			description:           "Should detect missing delimiters using HasError() and IsMissing()",
		},
		{
			name:                  "invalid token sequence",
			sourceCode:            "package main\n\nfunc main() {\n    @ # $ invalid\n}",
			expectedSyntaxError:   true,
			expectedErrorType:     "INVALID_TOKEN",
			expectedErrorLocation: "function body",
			expectedRecoveryHint:  "remove invalid token characters",
			shouldUseHasError:     true,
			shouldUseIsError:      true,
			shouldUseIsMissing:    false,
			description:           "Should detect invalid tokens using HasError() and IsError()",
		},
		{
			name:                  "unmatched parentheses in function call",
			sourceCode:            "package main\n\nfunc main() {\n    fmt.Println(\"hello\"\n}",
			expectedSyntaxError:   true,
			expectedErrorType:     "UNMATCHED_DELIMITER",
			expectedErrorLocation: "function call",
			expectedRecoveryHint:  "add missing closing parenthesis",
			shouldUseHasError:     true,
			shouldUseIsError:      false,
			shouldUseIsMissing:    true,
			description:           "Should detect unmatched delimiters using HasError() and IsMissing()",
		},
		{
			name:                  "incomplete interface declaration",
			sourceCode:            "package main\n\ntype Writer interface {\n    Write([]byte) (int, error",
			expectedSyntaxError:   true,
			expectedErrorType:     "INCOMPLETE_INTERFACE",
			expectedErrorLocation: "interface method signature",
			expectedRecoveryHint:  "complete method signature and interface body",
			shouldUseHasError:     true,
			shouldUseIsError:      false,
			shouldUseIsMissing:    true,
			description:           "Should detect incomplete interfaces using HasError() and IsMissing()",
		},
		{
			name:                  "invalid syntax in variable declaration",
			sourceCode:            "package main\n\nvar x = func( invalid",
			expectedSyntaxError:   true,
			expectedErrorType:     "INVALID_DECLARATION",
			expectedErrorLocation: "variable declaration",
			expectedRecoveryHint:  "fix function literal syntax",
			shouldUseHasError:     true,
			shouldUseIsError:      true,
			shouldUseIsMissing:    true,
			description:           "Should detect invalid declarations using all tree-sitter error methods",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// This test expects enhanced error detection that uses tree-sitter's native methods
			result := parser.detectSpecificSyntaxErrors(tt.sourceCode)

			// Verify syntax error is detected
			if tt.expectedSyntaxError {
				assert.True(t, result.HasSyntaxError,
					"Should detect syntax error: %s", tt.description)
				assert.Equal(t, tt.expectedErrorType, result.ErrorType,
					"Should classify error type correctly: %s", tt.description)
				assert.Contains(t, result.ErrorLocation, tt.expectedErrorLocation,
					"Should identify error location: %s", tt.description)
				assert.Contains(t, result.RecoveryHint, tt.expectedRecoveryHint,
					"Should provide recovery hint: %s", tt.description)
			} else {
				assert.False(t, result.HasSyntaxError,
					"Should not detect syntax error: %s", tt.description)
			}

			// Verify that the correct tree-sitter methods are used
			assert.Equal(t, tt.shouldUseHasError, result.UsedHasError,
				"HasError() usage should match expected: %s", tt.description)
			assert.Equal(t, tt.shouldUseIsError, result.UsedIsError,
				"IsError() usage should match expected: %s", tt.description)
			assert.Equal(t, tt.shouldUseIsMissing, result.UsedIsMissing,
				"IsMissing() usage should match expected: %s", tt.description)
		})
	}
}

// TestTreeSitterErrorMethodComparison tests that the enhanced error detection
// uses tree-sitter's native methods instead of simple string matching.
func TestTreeSitterErrorMethodComparison(t *testing.T) {
	tests := []struct {
		name                  string
		sourceCode            string
		legacyDetectsError    bool
		hasErrorDetectsError  bool
		isErrorDetectsError   bool
		isMissingDetectsError bool
		description           string
	}{
		{
			name:                  "simple unclosed brace",
			sourceCode:            "package main\n\nfunc main() {\n    fmt.Println(\"hello\")",
			legacyDetectsError:    false, // Legacy might miss this
			hasErrorDetectsError:  true,  // HasError() should catch this
			isErrorDetectsError:   false, // Not an ERROR node specifically
			isMissingDetectsError: true,  // Missing closing brace
			description:           "Native methods should be more accurate than legacy detection",
		},
		{
			name:                  "invalid identifier",
			sourceCode:            "package 123invalid\n\nfunc main() {}",
			legacyDetectsError:    false, // Legacy might not check identifier validity
			hasErrorDetectsError:  true,  // HasError() should catch this
			isErrorDetectsError:   true,  // Invalid identifier creates ERROR node
			isMissingDetectsError: false, // Nothing is missing, just invalid
			description:           "Native methods should detect semantic errors legacy misses",
		},
		{
			name:                  "completely invalid syntax",
			sourceCode:            "@ # $ invalid",
			legacyDetectsError:    true,  // Legacy might catch obvious errors
			hasErrorDetectsError:  true,  // HasError() should definitely catch this
			isErrorDetectsError:   true,  // Invalid tokens create ERROR nodes
			isMissingDetectsError: false, // Nothing is missing, just invalid
			description:           "Both methods should catch severe syntax errors",
		},
		{
			name:                  "subtle missing delimiter",
			sourceCode:            "package main\n\nfunc main() {\n    x := 5\n    return x",
			legacyDetectsError:    false, // Legacy might miss subtle issues
			hasErrorDetectsError:  true,  // HasError() should catch this
			isErrorDetectsError:   false, // Not necessarily an ERROR node
			isMissingDetectsError: true,  // Missing delimiter/semicolon
			description:           "Native methods should catch subtle errors legacy misses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}

			// Test legacy error detection (should be less accurate)
			legacyResult := parser.detectErrorsWithLegacyMethod(tt.sourceCode)
			assert.Equal(t, tt.legacyDetectsError, legacyResult.HasError,
				"Legacy detection should match expected: %s", tt.description)

			// Test native tree-sitter error detection methods
			nativeResult := parser.detectErrorsWithNativeTreeSitterMethods(tt.sourceCode)

			assert.Equal(t, tt.hasErrorDetectsError, nativeResult.HasErrorResult,
				"HasError() should match expected: %s", tt.description)
			assert.Equal(t, tt.isErrorDetectsError, nativeResult.IsErrorResult,
				"IsError() should match expected: %s", tt.description)
			assert.Equal(t, tt.isMissingDetectsError, nativeResult.IsMissingResult,
				"IsMissing() should match expected: %s", tt.description)

			// Verify that native methods provide more comprehensive detection
			nativeDetectedError := nativeResult.HasErrorResult || nativeResult.IsErrorResult ||
				nativeResult.IsMissingResult
			if !tt.legacyDetectsError && nativeDetectedError {
				// This proves native methods are more comprehensive
				assert.True(t, true, "Native methods detected error that legacy missed: %s", tt.description)
			}
		})
	}
}

// TestErrorDetectionWithRealTreeSitterAPI tests that error detection uses
// the actual tree-sitter API methods when available.
func TestErrorDetectionWithRealTreeSitterAPI(t *testing.T) {
	tests := []struct {
		name           string
		sourceCode     string
		expectedError  bool
		verifyAPIUsage bool
		description    string
	}{
		{
			name:           "valid code with real API",
			sourceCode:     "package main\n\nfunc main() {\n    fmt.Println(\"hello world\")\n}",
			expectedError:  false,
			verifyAPIUsage: true,
			description:    "Should use real tree-sitter API to verify valid code",
		},
		{
			name:           "syntax error with real API",
			sourceCode:     "package main\n\nfunc main() {\n    fmt.Println(\"hello\"",
			expectedError:  true,
			verifyAPIUsage: true,
			description:    "Should use real tree-sitter API to detect syntax errors",
		},
		{
			name:           "complex syntax error with real API",
			sourceCode:     "package main\n\nfunc incomplete(\ntype BadStruct struct {\n    field string",
			expectedError:  true,
			verifyAPIUsage: true,
			description:    "Should use real tree-sitter API for complex error detection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &GoParser{}
			ctx := context.Background()

			// This test verifies that we're actually using tree-sitter's native API
			result := parser.validateWithRealTreeSitterAPI(ctx, tt.sourceCode)

			if tt.expectedError {
				assert.Error(t, result.Error, "Should detect error using real API: %s", tt.description)
			} else {
				assert.NoError(t, result.Error, "Should not detect error using real API: %s", tt.description)
			}

			if tt.verifyAPIUsage {
				// Verify that actual tree-sitter methods were called
				assert.True(t, result.CalledHasError, "Should call HasError() method: %s", tt.description)
				assert.NotNil(t, result.ParseTree, "Should create actual parse tree: %s", tt.description)
				assert.True(t, result.UsedNativeAPI, "Should use native tree-sitter API: %s", tt.description)
			}
		})
	}
}

// Helper types for the test methods

// SpecificSyntaxErrorResult represents the result of specific syntax error detection.
type SpecificSyntaxErrorResult struct {
	HasSyntaxError bool
	ErrorType      string
	ErrorLocation  string
	RecoveryHint   string
	UsedHasError   bool
	UsedIsError    bool
	UsedIsMissing  bool
}

// LegacyErrorResult represents the result of legacy error detection.
type LegacyErrorResult struct {
	HasError bool
}

// NativeTreeSitterResult represents the result of native tree-sitter error detection.
type NativeTreeSitterResult struct {
	HasErrorResult  bool
	IsErrorResult   bool
	IsMissingResult bool
}

// RealAPIResult represents the result of using real tree-sitter API.
type RealAPIResult struct {
	Error          error
	CalledHasError bool
	ParseTree      interface{}
	UsedNativeAPI  bool
}

// Stub methods that will need to be implemented

func (p *GoParser) detectSpecificSyntaxErrors(sourceCode string) *SpecificSyntaxErrorResult {
	// Implementation will use tree-sitter's native error detection methods
	// This will be implemented in the GREEN phase
	return &SpecificSyntaxErrorResult{
		HasSyntaxError: false,
		ErrorType:      "",
		ErrorLocation:  "",
		RecoveryHint:   "",
		UsedHasError:   false,
		UsedIsError:    false,
		UsedIsMissing:  false,
	}
}

func (p *GoParser) detectErrorsWithLegacyMethod(sourceCode string) *LegacyErrorResult {
	// This represents the old way of detecting errors (string-based, limited)
	// Implementation will show the limitations of legacy detection
	return &LegacyErrorResult{
		HasError: false,
	}
}

func (p *GoParser) detectErrorsWithNativeTreeSitterMethods(sourceCode string) *NativeTreeSitterResult {
	// Implementation will use HasError(), IsError(), and IsMissing() from tree-sitter
	// This will be implemented in the GREEN phase
	return &NativeTreeSitterResult{
		HasErrorResult:  false,
		IsErrorResult:   false,
		IsMissingResult: false,
	}
}

func (p *GoParser) validateWithRealTreeSitterAPI(ctx context.Context, sourceCode string) *RealAPIResult {
	// Implementation will use actual tree-sitter API calls
	// This will be implemented in the GREEN phase
	return &RealAPIResult{
		Error:          nil,
		CalledHasError: false,
		ParseTree:      nil,
		UsedNativeAPI:  false,
	}
}
