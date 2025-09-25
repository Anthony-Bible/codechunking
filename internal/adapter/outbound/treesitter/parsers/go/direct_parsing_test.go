package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
)

// TestDirectParsingOfGoSyntaxFragments tests what happens when we parse Go syntax fragments directly
// using tree-sitter without artificial context wrapping.
func TestDirectParsingOfGoSyntaxFragments(t *testing.T) {
	ctx := context.Background()

	// Test cases representing different Go syntax fragments we want to detect
	testCases := []struct {
		name          string
		fragment      string
		category      string
		expectsError  bool
		expectedNodes []string // Node types we expect to find
	}{
		// Struct field patterns - these should NOT parse successfully as standalone fragments
		{
			name:         "simple_field",
			fragment:     "name string",
			category:     "struct_field",
			expectsError: true, // Individual field declarations are not valid Go programs
		},
		{
			name:         "tagged_field",
			fragment:     "ID int `json:\"id\"`",
			category:     "struct_field",
			expectsError: true,
		},

		// Embedded types - these should NOT parse successfully as standalone fragments
		{
			name:         "embedded_type",
			fragment:     "io.Reader",
			category:     "embedded",
			expectsError: true, // Just a type identifier is not a valid Go program
		},

		// Interface method signatures - these should NOT parse successfully as standalone fragments
		{
			name:         "interface_method",
			fragment:     "Read([]byte) (int, error)",
			category:     "interface_method",
			expectsError: true, // Method signatures without context are not valid Go programs
		},

		// Invalid constructs - these should also not parse successfully
		{
			name:          "return_stmt",
			fragment:      "return nil",
			category:      "invalid",
			expectsError:  false, // Actually parses as expression_statement
			expectedNodes: []string{"return_statement"},
		},
		{
			name:          "func_call",
			fragment:      "fmt.Println(\"test\")",
			category:      "invalid",
			expectsError:  false, // Actually parses as expression_statement
			expectedNodes: []string{"call_expression"},
		},

		// Only complete Go constructs should parse successfully
		{
			name:          "complete_package",
			fragment:      "package main",
			category:      "valid_complete",
			expectsError:  false,
			expectedNodes: []string{"package_clause"},
		},
		{
			name:          "complete_import",
			fragment:      "import \"fmt\"",
			category:      "valid_complete",
			expectsError:  false,
			expectedNodes: []string{"import_declaration"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing %s (%s): %q", tc.name, tc.category, tc.fragment)

			// Try direct parsing
			result := treesitter.CreateTreeSitterParseTree(ctx, tc.fragment)

			if tc.expectsError {
				validateExpectedError(t, result, tc.fragment)
			} else {
				validateExpectedSuccess(t, result, tc.fragment, tc.expectedNodes)
			}
		})
	}
}

// validateExpectedError checks that parsing failed or has syntax errors as expected.
func validateExpectedError(t *testing.T, result *treesitter.ParseTreeResult, fragment string) {
	t.Helper()
	// We expect this to either fail to parse or have syntax errors
	if result.Error == nil {
		hasErrors, _ := result.ParseTree.HasSyntaxErrors()
		if !hasErrors {
			t.Errorf(
				"Expected parsing to fail or have syntax errors for fragment %q, but it succeeded",
				fragment,
			)
		} else {
			t.Logf("✅ Fragment correctly has syntax errors as expected")
		}
	} else {
		t.Logf("✅ Fragment correctly failed to parse as expected: %v", result.Error)
	}
}

// validateExpectedSuccess checks that parsing succeeded and contains expected node types.
func validateExpectedSuccess(
	t *testing.T,
	result *treesitter.ParseTreeResult,
	fragment string,
	expectedNodes []string,
) {
	t.Helper()
	// We expect this to parse successfully
	if result.Error != nil {
		t.Errorf("Expected parsing to succeed for fragment %q, but got error: %v", fragment, result.Error)
		return
	}

	hasErrors, _ := result.ParseTree.HasSyntaxErrors()
	if hasErrors {
		t.Errorf("Expected no syntax errors for fragment %q, but found errors", fragment)
		return
	}

	// Check for expected node types
	if len(expectedNodes) > 0 {
		foundTypes := analyzeParseTreeNodes(result.ParseTree)
		for _, expectedNode := range expectedNodes {
			if foundTypes[expectedNode] == 0 {
				t.Errorf("Expected to find node type %s for fragment %q", expectedNode, fragment)
			}
		}
	}
}

// analyzeParseTreeNodes walks the parse tree and counts node types.
func analyzeParseTreeNodes(parseTree *valueobject.ParseTree) map[string]int {
	if parseTree == nil {
		return make(map[string]int)
	}

	foundTypes := make(map[string]int)
	walkTreeNodes(parseTree.RootNode(), foundTypes)
	return foundTypes
}

// walkTreeNodes recursively walks the tree and counts node types.
func walkTreeNodes(node *valueobject.ParseNode, foundTypes map[string]int) {
	if node == nil {
		return
	}

	foundTypes[node.Type]++

	for _, child := range node.Children {
		walkTreeNodes(child, foundTypes)
	}
}
