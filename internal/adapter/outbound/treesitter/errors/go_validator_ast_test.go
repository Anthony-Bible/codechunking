package parsererrors

import (
	"testing"

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

// TestGoValidatorStringParsingReplacement tests that the current string parsing logic
// in validatePackageSyntax is replaced with proper AST-based analysis.
func TestGoValidatorStringParsingReplacement(t *testing.T) {
	tests := []struct {
		name              string
		sourceCode        string
		forbiddenPatterns []string
		requiredMethods   []string
		description       string
	}{
		{
			name:       "package detection uses AST not string parsing",
			sourceCode: "package main\n\nfunc main() {}",
			forbiddenPatterns: []string{
				"strings.Split(source, \"\\n\")",
				"strings.HasPrefix(line, \"package \")",
				"strings.TrimSpace(line)",
			},
			requiredMethods: []string{
				"QueryPackageDeclarations",
				"HasSyntaxErrors",
			},
			description: "Should replace line-by-line parsing with AST queries",
		},
		{
			name: "snippet detection uses AST not line counting",
			sourceCode: `type Config struct {
				Host string
				Port int
			}`,
			forbiddenPatterns: []string{
				"for _, line := range lines",
				"strings.HasPrefix(line, \"type \")",
				"strings.Contains(line, \"struct {\")",
				"nonCommentLines++",
				"typeOnlyLines++",
			},
			requiredMethods: []string{
				"QueryTypeDeclarations",
				"QueryFunctionDeclarations",
				"QueryVariableDeclarations",
			},
			description: "Should replace manual line counting with AST node analysis",
		},
		{
			name: "heuristic detection uses AST not string patterns",
			sourceCode: `func Add(a, b int) int {
				return a + b
			}`,
			forbiddenPatterns: []string{
				"strings.Contains(line, \"// Add adds\")",
				"strings.Contains(line, \"return a + b\")",
				"strings.Contains(line, \"// Person represents\")",
				"len(strings.Split(source, \"\\n\")) < 20",
			},
			requiredMethods: []string{
				"QueryFunctionDeclarations",
				"QueryComments",
			},
			description: "Should replace hardcoded heuristics with AST-based analysis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test validates that the implementation analysis shows proper method usage
			analysis := validator.analyzeValidationMethods(tt.sourceCode)

			// Check that forbidden string parsing methods are not used
			for _, forbidden := range tt.forbiddenPatterns {
				assert.False(t, analysis.UsedStringPatterns[forbidden],
					"Should not use forbidden string pattern: %s", forbidden)
			}

			// Check that required AST methods are used
			for _, required := range tt.requiredMethods {
				assert.True(t, analysis.UsedASTMethods[required],
					"Should use required AST method: %s", required)
			}

			assert.True(t, analysis.UsesASTValidation,
				"Should use AST-based validation instead of string parsing")
		})
	}
}

// TestGoValidatorErrorNodeDetection tests that syntax error detection uses
// tree-sitter error nodes instead of string pattern matching.
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
			name:        "malformed package detected via error nodes",
			sourceCode:  "package // missing name\n\nfunc main() {}",
			expectError: true,
			description: "Should use tree-sitter error nodes instead of strings.Contains",
		},
		{
			name:        "incomplete function detected via error nodes",
			sourceCode:  "package main\n\nfunc incomplete(",
			expectError: true,
			description: "Should detect syntax errors using tree-sitter parsing",
		},
		{
			name:        "unclosed struct detected via error nodes",
			sourceCode:  "package main\n\ntype Config struct {\n    Host string",
			expectError: true,
			description: "Should use error node detection for malformed structures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test expects error detection to use tree-sitter capabilities:
			// 1. Parse source into AST
			// 2. Use HasSyntaxErrors() to detect issues
			// 3. Use error node traversal for detailed error information
			// 4. Replace all string pattern matching with grammar-based validation
			err := validator.validateSyntaxWithErrorNodes(tt.sourceCode)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// ValidationMethodAnalysis and methods are now implemented in go_validator.go
