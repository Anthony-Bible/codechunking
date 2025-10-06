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
			shouldUseIsError:      false,
			shouldUseIsMissing:    true,
			description:           "Should detect incomplete functions using HasError() and IsMissing()",
		},
		{
			name:                  "malformed struct missing closing brace",
			sourceCode:            "package main\n\ntype BadStruct struct {\n    field string",
			expectedSyntaxError:   true,
			expectedErrorType:     "INCOMPLETE_STRUCT",
			expectedErrorLocation: "struct definition",
			expectedRecoveryHint:  "add closing brace '}' to complete struct",
			shouldUseHasError:     true,
			shouldUseIsError:      true,
			shouldUseIsMissing:    false,
			description:           "Should detect incomplete structs using HasError() and IsError()",
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
			expectedErrorType:     "MISSING_BRACE",
			expectedErrorLocation: "end of function body",
			expectedRecoveryHint:  "add closing brace '}'",
			shouldUseHasError:     true,
			shouldUseIsError:      false,
			shouldUseIsMissing:    true,
			description:           "Should detect missing closing brace using HasError() and IsMissing()",
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
			expectedErrorType:     "INCOMPLETE_INTERFACE",
			expectedErrorLocation: "interface method signature",
			expectedRecoveryHint:  "complete method signature and interface body",
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
			shouldUseIsError:      true,
			shouldUseIsMissing:    false,
			description:           "Should detect incomplete interfaces using HasError() and IsError()",
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
			shouldUseIsMissing:    false,
			description:           "Should detect invalid declarations using HasError() and IsError()",
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
