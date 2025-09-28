package parsererrors

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGoValidatorASTBasedPackageValidation tests that package validation in go_validator.go
// uses AST-based validation instead of line-by-line string parsing.
func TestGoValidatorASTBasedPackageValidation(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedError bool
		errorMessage  string
		description   string
	}{
		{
			name:          "valid package detected via AST",
			sourceCode:    "package main\n\nfunc main() {}",
			expectedError: false,
			description:   "Should use QueryPackageDeclarations to detect valid package",
		},
		{
			name:          "missing package detected via AST",
			sourceCode:    "func helper() int { return 42 }",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use AST node analysis instead of line counting",
		},
		{
			name: "type-only snippet allowed via AST analysis",
			sourceCode: `type Person struct {
				Name string
				Age  int
			}`,
			expectedError: false,
			description:   "Should use QueryTypeDeclarations to detect type-only snippets",
		},
		{
			name:          "invalid package name detected via tree-sitter",
			sourceCode:    "package 123invalid\n\nfunc main() {}",
			expectedError: true,
			errorMessage:  "invalid package declaration",
			description:   "Should use tree-sitter error nodes instead of string patterns",
		},
		{
			name: "comment-only code treated as snippet",
			sourceCode: `// This is just a comment
			// Another comment line`,
			expectedError: false,
			description:   "Should use AST to detect comment-only content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test expects validatePackageSyntax to be replaced with AST-based validation:
			// 1. Use QueryPackageDeclarations instead of strings.HasPrefix(line, "package ")
			// 2. Use QueryTypeDeclarations instead of manual type counting
			// 3. Use HasSyntaxErrors() instead of string pattern detection
			// 4. Use tree-sitter node analysis instead of line-by-line parsing
			err := validator.validatePackageSyntaxWithAST(tt.sourceCode)

			if tt.expectedError {
				assert.Error(t, err, tt.description)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestGoValidatorErrorNodeDetection tests that syntax error detection uses
// tree-sitter ERROR nodes for severe syntax errors.
func TestGoValidatorErrorNodeDetection(t *testing.T) {
	tests := []struct {
		name        string
		sourceCode  string
		expectError bool
		description string
	}{
		{
			name:        "valid syntax has no error nodes",
			sourceCode:  "package main\n\nfunc main() {\n    x := 42\n    _ = x\n}",
			expectError: false,
			description: "Should use tree-sitter to validate correct syntax",
		},
		{
			name:        "unclosed struct generates ERROR nodes",
			sourceCode:  "package main\n\ntype Config struct {\n    Host string",
			expectError: true,
			description: "Should detect severe syntax errors via ERROR nodes",
		},
		{
			name:        "unclosed parentheses generate ERROR nodes",
			sourceCode:  "package main\n\nfunc test() {\n    fmt.Println(\"hello\"",
			expectError: true,
			description: "Should detect unclosed parentheses via ERROR nodes",
		},
		{
			name:        "invalid token sequence generates ERROR nodes",
			sourceCode:  "package main\n\nfunc main() { @ # $ }",
			expectError: true,
			description: "Should detect invalid token sequences via ERROR nodes",
		},
		{
			name:        "mismatched braces generate ERROR nodes",
			sourceCode:  "package main\n\nfunc main() { if true { }",
			expectError: true,
			description: "Should detect mismatched braces via ERROR nodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test expects ERROR node detection to work properly:
			// 1. Parse source into AST using tree-sitter
			// 2. Traverse parse tree looking for nodes with Type == "ERROR"
			// 3. Return syntax error when ERROR nodes are found
			// 4. Use proper tree-sitter grammar-based validation
			err := validator.validateSyntaxWithErrorNodes(tt.sourceCode)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestGoValidatorHasErrorFlagDetection tests that syntax error detection uses
// tree-sitter HasError() flags for missing/incomplete constructs.
func TestGoValidatorHasErrorFlagDetection(t *testing.T) {
	tests := []struct {
		name        string
		sourceCode  string
		expectError bool
		description string
	}{
		{
			name:        "valid syntax has no HasError flags",
			sourceCode:  "package main\n\nfunc main() {\n    x := 42\n    _ = x\n}",
			expectError: false,
			description: "Should validate correct syntax with no HasError flags",
		},
		{
			name:        "missing package name detected via HasError flag",
			sourceCode:  "package // missing name\n\nfunc main() {}",
			expectError: true,
			description: "Should detect missing package name via HasError flag mechanism",
		},
		{
			name:        "incomplete function detected via HasError flag",
			sourceCode:  "package main\n\nfunc incomplete(",
			expectError: true,
			description: "Should detect incomplete function via HasError flag mechanism",
		},
		{
			name:        "missing function body detected via HasError flag",
			sourceCode:  "package main\n\nfunc test()",
			expectError: true,
			description: "Should detect missing function body via HasError flag",
		},
		{
			name:        "incomplete import detected via HasError flag",
			sourceCode:  "package main\n\nimport",
			expectError: true,
			description: "Should detect incomplete import via HasError flag",
		},
		{
			name:        "incomplete variable declaration detected via HasError flag",
			sourceCode:  "package main\n\nvar x",
			expectError: true,
			description: "Should detect incomplete variable declaration via HasError flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test expects HasError flag detection to work properly:
			// 1. Parse source into AST using tree-sitter
			// 2. Check the raw tree-sitter tree's HasError() method
			// 3. Also traverse nodes checking for HasError flag on individual nodes
			// 4. Return syntax error when HasError flags are detected
			// 5. Catch syntax errors that don't generate ERROR nodes but set flags
			err := validator.validateSyntaxWithHasErrorFlags(tt.sourceCode)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestGoValidatorComprehensiveErrorDetection tests that syntax error detection uses
// BOTH ERROR nodes AND HasError() flags for complete coverage.
func TestGoValidatorComprehensiveErrorDetection(t *testing.T) {
	tests := []struct {
		name               string
		sourceCode         string
		expectError        bool
		expectedErrorNodes bool
		expectedHasError   bool
		description        string
	}{
		{
			name:               "valid syntax has no errors",
			sourceCode:         "package main\n\nfunc main() {\n    x := 42\n    _ = x\n}",
			expectError:        false,
			expectedErrorNodes: false,
			expectedHasError:   false,
			description:        "Should validate perfect syntax with no error indicators",
		},
		{
			name:               "unclosed struct has both ERROR nodes and HasError flag",
			sourceCode:         "package main\n\ntype Config struct {\n    Host string",
			expectError:        true,
			expectedErrorNodes: true,
			expectedHasError:   true,
			description:        "Should detect via both ERROR nodes and HasError flag",
		},
		{
			name:               "missing package name has HasError flag but no ERROR nodes",
			sourceCode:         "package // missing name\n\nfunc main() {}",
			expectError:        true,
			expectedErrorNodes: false,
			expectedHasError:   true,
			description:        "Should detect via HasError flag when no ERROR nodes generated",
		},
		{
			name:               "incomplete function has HasError flag but no ERROR nodes",
			sourceCode:         "package main\n\nfunc incomplete(",
			expectError:        true,
			expectedErrorNodes: false,
			expectedHasError:   true,
			description:        "Should detect via HasError flag for incomplete functions",
		},
		{
			name:               "invalid tokens generate ERROR nodes and HasError flag",
			sourceCode:         "package main\n\nfunc main() { @ # $ }",
			expectError:        true,
			expectedErrorNodes: true,
			expectedHasError:   true,
			description:        "Should detect via both mechanisms for severe syntax errors",
		},
		{
			name:               "mixed valid and invalid generates HasError flag",
			sourceCode:         "package main\n\nfunc valid() {}\n\nfunc invalid(",
			expectError:        true,
			expectedErrorNodes: false,
			expectedHasError:   true,
			description:        "Should detect partial errors via HasError flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test expects comprehensive error detection:
			// 1. Parse source into AST using tree-sitter
			// 2. Check for ERROR nodes in the parse tree
			// 3. Check tree-sitter tree's HasError() method
			// 4. Check HasError flags on individual nodes
			// 5. Return syntax error if EITHER mechanism detects issues
			// 6. Provide detailed error information about detection method used
			result := validator.validateSyntaxWithComprehensiveErrorDetection(tt.sourceCode)

			if tt.expectError {
				assert.Error(t, result.Error, tt.description)
				if tt.expectedErrorNodes {
					assert.True(t, result.HasErrorNodes, "Should detect ERROR nodes")
				}
				if tt.expectedHasError {
					assert.True(t, result.HasErrorFlags, "Should detect HasError flags")
				}
			} else {
				assert.NoError(t, result.Error, tt.description)
				assert.False(t, result.HasErrorNodes, "Should not have ERROR nodes")
				assert.False(t, result.HasErrorFlags, "Should not have HasError flags")
			}
		})
	}
}

// TestGoValidatorErrorDetectionEdgeCases tests edge cases for complete error detection coverage.
func TestGoValidatorErrorDetectionEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		sourceCode  string
		expectError bool
		description string
	}{
		{
			name:        "empty source has no errors",
			sourceCode:  "",
			expectError: false,
			description: "Should handle empty source without errors",
		},
		{
			name:        "whitespace only has no errors",
			sourceCode:  "   \n\t  \n   ",
			expectError: false,
			description: "Should handle whitespace-only source without errors",
		},
		{
			name:        "comment only has no errors",
			sourceCode:  "// This is just a comment\n/* Block comment */",
			expectError: false,
			description: "Should handle comment-only source without errors",
		},
		{
			name:        "malformed comment generates ERROR nodes",
			sourceCode:  "package main\n\n/* unclosed comment",
			expectError: true,
			description: "Should detect malformed comments via ERROR nodes",
		},
		{
			name:        "nested incomplete constructs detected via HasError",
			sourceCode:  "package main\n\ntype Outer struct {\n    Inner struct {\n        Field",
			expectError: true,
			description: "Should detect nested incomplete constructs",
		},
		{
			name:        "multiple syntax errors detected",
			sourceCode:  "package\nfunc incomplete(\ntype Broken struct {",
			expectError: true,
			description: "Should detect multiple syntax errors",
		},
		{
			name:        "unicode content with syntax errors",
			sourceCode:  "package main\n\nfunc 测试(",
			expectError: true,
			description: "Should detect syntax errors in unicode content",
		},
		{
			name:        "very large incomplete construct",
			sourceCode:  "package main\n\nfunc test() {\n    " + strings.Repeat("var x", 1000) + "\n    incomplete(",
			expectError: true,
			description: "Should detect errors in large source content",
		},
		{
			name:        "mixed line endings with errors",
			sourceCode:  "package main\r\n\r\nfunc incomplete(\n",
			expectError: true,
			description: "Should handle mixed line endings with syntax errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test expects robust error detection:
			// 1. Handle edge cases gracefully
			// 2. Detect syntax errors regardless of source characteristics
			// 3. Use both ERROR nodes and HasError flags appropriately
			// 4. Provide consistent error detection across various inputs
			err := validator.validateSyntaxWithRobustErrorDetection(tt.sourceCode)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestGoValidatorErrorDetectionPerformance tests that error detection performs well.
func TestGoValidatorErrorDetectionPerformance(t *testing.T) {
	tests := []struct {
		name        string
		sourceCode  string
		expectError bool
		maxDuration time.Duration
		description string
	}{
		{
			name:        "small valid source performs quickly",
			sourceCode:  "package main\n\nfunc main() { fmt.Println(\"hello\") }",
			expectError: false,
			maxDuration: 10 * time.Millisecond,
			description: "Should validate small source quickly",
		},
		{
			name:        "small invalid source performs quickly",
			sourceCode:  "package main\n\nfunc incomplete(",
			expectError: true,
			maxDuration: 10 * time.Millisecond,
			description: "Should detect errors in small source quickly",
		},
		{
			name:        "large valid source performs reasonably",
			sourceCode:  generateLargeValidGoSource(1000),
			expectError: false,
			maxDuration: 100 * time.Millisecond,
			description: "Should validate large source within reasonable time",
		},
		{
			name:        "large invalid source performs reasonably",
			sourceCode:  generateLargeValidGoSource(1000) + "\n\nfunc incomplete(",
			expectError: true,
			maxDuration: 100 * time.Millisecond,
			description: "Should detect errors in large source within reasonable time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test expects performant error detection:
			// 1. Parse and validate within time constraints
			// 2. Use efficient tree-sitter traversal
			// 3. Avoid redundant parsing operations
			// 4. Scale reasonably with source size
			start := time.Now()
			err := validator.validateSyntaxWithPerformantErrorDetection(tt.sourceCode)
			duration := time.Since(start)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}

			assert.LessOrEqual(t, duration, tt.maxDuration,
				"Validation took %v, expected <= %v", duration, tt.maxDuration)
		})
	}
}

// generateLargeValidGoSource generates a large valid Go source for performance testing.
func generateLargeValidGoSource(functionCount int) string {
	var builder strings.Builder
	builder.WriteString("package main\n\nimport \"fmt\"\n\n")

	for i := range functionCount {
		builder.WriteString(fmt.Sprintf("func function%d() {\n    fmt.Println(\"Function %d\")\n}\n\n", i, i))
	}

	builder.WriteString("func main() {\n")
	for i := range functionCount {
		builder.WriteString(fmt.Sprintf("    function%d()\n", i))
	}
	builder.WriteString("}")

	return builder.String()
}

// ValidationMethodAnalysis and methods are now implemented in go_validator.go
