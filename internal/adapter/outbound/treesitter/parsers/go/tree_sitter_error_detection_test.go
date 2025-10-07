package goparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			expectedIsError:    false,
			expectedIsMissing:  true,
			expectedErrorNodes: []string{"MISSING"},
			description:        "Should use multiple tree-sitter error methods for incomplete constructs",
		},
		{
			name:               "malformed struct detected by comprehensive error detection",
			sourceCode:         "package main\n\ntype BadStruct struct {\n    field string",
			expectedHasError:   true,
			expectedIsError:    true,
			expectedIsMissing:  false,
			expectedErrorNodes: []string{"ERROR"},
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
