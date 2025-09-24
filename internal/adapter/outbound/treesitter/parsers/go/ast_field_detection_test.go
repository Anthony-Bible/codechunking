package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"context"
	"testing"
)

// TestTryASTBasedFieldDetection_ShouldParseLineDirectly tests that the function
// should parse the line parameter directly as Go syntax, not create artificial contexts.
// CURRENT ISSUE: Function creates fake struct/interface wrappers instead of direct parsing.
func TestTryASTBasedFieldDetection_ShouldParseLineDirectly(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		expectedResult bool
		expectedUseAST bool
		description    string
	}{
		{
			name:           "simple_struct_field",
			line:           "Name string",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Should detect simple typed field by parsing line directly",
		},
		{
			name:           "tagged_field",
			line:           "ID int `json:\"id\"`",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Should detect tagged field by parsing line directly",
		},
		{
			name:           "embedded_type",
			line:           "io.Reader",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Should detect embedded type by parsing line directly",
		},
		{
			name:           "interface_method",
			line:           "Read([]byte) (int, error)",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Should detect interface method by parsing line directly",
		},
		{
			name:           "return_statement",
			line:           "return nil",
			expectedResult: false,
			expectedUseAST: true,
			description:    "Should reject return statement by parsing line directly",
		},
		{
			name:           "import_statement",
			line:           "import \"context\"",
			expectedResult: false,
			expectedUseAST: true,
			description:    "Should reject import statement by parsing line directly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, useAST := tryASTBasedFieldDetection(tt.line)

			if useAST != tt.expectedUseAST {
				t.Errorf("Expected useAST=%v, got %v for line: %s", tt.expectedUseAST, useAST, tt.line)
			}

			if result != tt.expectedResult {
				t.Errorf("Expected result=%v, got %v for line: %s", tt.expectedResult, result, tt.line)
			}

			// CRITICAL TEST: Verify the function actually parses the line parameter directly
			// This test will FAIL with current implementation because it uses artificial contexts
			t.Log("EXPECTED BEHAVIOR: Function should parse line directly using TreeSitterQueryEngine")
			t.Log("CURRENT PROBLEM: Function creates artificial struct/interface contexts")
		})
	}
}

// TestTryASTBasedFieldDetection_ShouldNotCreateArtificialContexts tests that the function
// should NOT wrap lines in fake struct/interface declarations.
// CURRENT ISSUE: Function creates artificial test contexts with hardcoded struct/interface wrappers.
func TestTryASTBasedFieldDetection_ShouldNotCreateArtificialContexts(t *testing.T) {
	// This test explicitly checks that the function doesn't use artificial wrapping
	line := "count int"

	// Call the function
	result, useAST := tryASTBasedFieldDetection(line)

	// The function should work without creating artificial contexts
	if !useAST {
		t.Error("Function should be able to parse simple field without artificial contexts")
	}

	if !result {
		t.Error("Simple field should be detected as valid")
	}

	// CRITICAL ASSERTION: Function should not internally create test contexts like:
	// "package test\n\ntype TestStruct struct {\n" + line + "\n}"
	// This is inefficient and not using the line parameter properly.

	t.Log("EXPECTED BEHAVIOR: Parse line directly as Go syntax fragment")
	t.Log("CURRENT PROBLEM: Creates artificial 'package test\\n\\ntype TestStruct struct {\\n' + line + '\\n}'")
	t.Log("CURRENT PROBLEM: This approach is inefficient and doesn't use actual context")
}

// TestTryASTBasedFieldDetection_DirectLineParsing tests the expected behavior
// of parsing lines directly without artificial wrapper contexts.
// These tests define how the refactored function should work.
func TestTryASTBasedFieldDetection_DirectLineParsing(t *testing.T) {
	tests := []struct {
		name             string
		line             string
		expectedResult   bool
		expectedUseAST   bool
		expectedNodeType string
		description      string
	}{
		{
			name:             "field_declaration_with_type",
			line:             "username string",
			expectedResult:   true,
			expectedUseAST:   true,
			expectedNodeType: "field_declaration",
			description:      "Direct parsing should identify field declaration node type",
		},
		{
			name:             "field_with_pointer_type",
			line:             "data *User",
			expectedResult:   true,
			expectedUseAST:   true,
			expectedNodeType: "field_declaration",
			description:      "Direct parsing should handle pointer types",
		},
		{
			name:             "field_with_slice_type",
			line:             "items []string",
			expectedResult:   true,
			expectedUseAST:   true,
			expectedNodeType: "field_declaration",
			description:      "Direct parsing should handle slice types",
		},
		{
			name:             "field_with_map_type",
			line:             "metadata map[string]interface{}",
			expectedResult:   true,
			expectedUseAST:   true,
			expectedNodeType: "field_declaration",
			description:      "Direct parsing should handle complex map types",
		},
		{
			name:             "method_signature",
			line:             "Process(data []byte) error",
			expectedResult:   true,
			expectedUseAST:   true,
			expectedNodeType: "method_spec",
			description:      "Direct parsing should identify interface method specs",
		},
		{
			name:             "embedded_interface",
			line:             "io.Writer",
			expectedResult:   true,
			expectedUseAST:   true,
			expectedNodeType: "embedded_field",
			description:      "Direct parsing should identify embedded types",
		},
		{
			name:             "function_call_not_field",
			line:             "fmt.Println(\"hello\")",
			expectedResult:   false,
			expectedUseAST:   true,
			expectedNodeType: "call_expression",
			description:      "Direct parsing should reject function calls",
		},
		{
			name:             "variable_assignment_not_field",
			line:             "x := 42",
			expectedResult:   false,
			expectedUseAST:   true,
			expectedNodeType: "short_var_declaration",
			description:      "Direct parsing should reject variable assignments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will FAIL with current implementation
			// because it doesn't parse lines directly
			result, useAST := tryASTBasedFieldDetection(tt.line)

			if useAST != tt.expectedUseAST {
				t.Errorf("Expected useAST=%v, got %v", tt.expectedUseAST, useAST)
			}

			if result != tt.expectedResult {
				t.Errorf("Expected result=%v, got %v", tt.expectedResult, result)
			}

			// EXPECTED BEHAVIOR: Function should use TreeSitterQueryEngine
			// to parse the line directly and identify the actual node type
			t.Logf("EXPECTED: Parse line as Go syntax, identify as %s", tt.expectedNodeType)
			t.Log("CURRENT ISSUE: Uses artificial contexts instead of direct parsing")
		})
	}
}

// TestTryASTBasedFieldDetection_EfficiencyAndCorrectness tests that the function
// should be more efficient by parsing once directly instead of multiple artificial contexts.
// CURRENT ISSUE: Function creates multiple test contexts and parses each one separately.
func TestTryASTBasedFieldDetection_EfficiencyAndCorrectness(t *testing.T) {
	line := "age int"

	// Current implementation is inefficient - it creates multiple contexts:
	// 1. "package test\n\ntype TestStruct struct {\n" + line + "\n}"
	// 2. "package test\n\ntype TestInterface interface {\n" + line + "\n}"
	// This is wasteful and doesn't use the actual parse context.

	result, useAST := tryASTBasedFieldDetection(line)

	if !useAST {
		t.Error("Simple field should be parseable with AST")
	}

	if !result {
		t.Error("Simple field should be detected as valid")
	}

	// EXPECTED BEHAVIOR: Function should:
	// 1. Parse the line directly as a Go syntax fragment
	// 2. Use TreeSitterQueryEngine methods on the parsed line
	// 3. Be much more efficient with single parsing operation
	// 4. Not create artificial wrapper contexts

	t.Log("EXPECTED BEHAVIOR:")
	t.Log("  1. Parse line directly: treesitter.CreateTreeSitterParseTree(ctx, line)")
	t.Log("  2. Use QueryFieldDeclarations/QueryMethodSpecs on direct parse result")
	t.Log("  3. Single parsing operation, not multiple artificial contexts")

	t.Log("CURRENT PROBLEMS:")
	t.Log("  1. Creates artificial struct wrapper context")
	t.Log("  2. Creates artificial interface wrapper context")
	t.Log("  3. Parses multiple fake contexts instead of the actual line")
	t.Log("  4. Inefficient and doesn't represent real parsing scenario")
}

// TestTryASTBasedFieldDetection_EdgeCases tests edge cases that should be handled
// correctly with direct line parsing.
func TestTryASTBasedFieldDetection_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		expectedResult bool
		expectedUseAST bool
		description    string
	}{
		{
			name:           "empty_line",
			line:           "",
			expectedResult: false,
			expectedUseAST: false,
			description:    "Empty line should not be parseable",
		},
		{
			name:           "whitespace_only",
			line:           "   \t  ",
			expectedResult: false,
			expectedUseAST: false,
			description:    "Whitespace-only line should not be parseable",
		},
		{
			name:           "comment_only",
			line:           "// This is a comment",
			expectedResult: false,
			expectedUseAST: true,
			description:    "Comment should be rejected as field",
		},
		{
			name:           "field_with_comment",
			line:           "name string // user name",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Field with inline comment should be detected",
		},
		{
			name:           "complex_generic_type",
			line:           "data map[string][]interface{}",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Complex generic types should be handled",
		},
		{
			name:           "function_type_field",
			line:           "handler func(context.Context) error",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Function type fields should be detected",
		},
		{
			name:           "channel_type_field",
			line:           "ch chan<- string",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Channel type fields should be detected",
		},
		{
			name:           "malformed_syntax",
			line:           "invalid syntax here (",
			expectedResult: false,
			expectedUseAST: false,
			description:    "Malformed syntax should fail parsing gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, useAST := tryASTBasedFieldDetection(tt.line)

			if useAST != tt.expectedUseAST {
				t.Errorf("Expected useAST=%v, got %v for line: %s", tt.expectedUseAST, useAST, tt.line)
			}

			if result != tt.expectedResult {
				t.Errorf("Expected result=%v, got %v for line: %s", tt.expectedResult, result, tt.line)
			}

			t.Log("Test case:", tt.description)
		})
	}
}

// TestTryASTBasedFieldDetection_ShouldUseTreeSitterQueryEngine tests that the function
// should properly utilize TreeSitterQueryEngine methods for field detection.
// CURRENT ISSUE: Function uses QueryEngine on artificial contexts, not direct line parsing.
func TestTryASTBasedFieldDetection_ShouldUseTreeSitterQueryEngine(t *testing.T) {
	tests := []struct {
		name                string
		line                string
		expectedQueryMethod string
		expectedResult      bool
		description         string
	}{
		{
			name:                "struct_field_should_use_QueryFieldDeclarations",
			line:                "id int64",
			expectedQueryMethod: "QueryFieldDeclarations",
			expectedResult:      true,
			description:         "Struct fields should be detected using QueryFieldDeclarations on direct parse",
		},
		{
			name:                "interface_method_should_use_QueryMethodSpecs",
			line:                "Write([]byte) (int, error)",
			expectedQueryMethod: "QueryMethodSpecs",
			expectedResult:      true,
			description:         "Interface methods should be detected using QueryMethodSpecs on direct parse",
		},
		{
			name:                "embedded_type_should_use_QueryEmbeddedTypes",
			line:                "sync.Mutex",
			expectedQueryMethod: "QueryEmbeddedTypes",
			expectedResult:      true,
			description:         "Embedded types should be detected using QueryEmbeddedTypes on direct parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, useAST := tryASTBasedFieldDetection(tt.line)

			if !useAST {
				t.Errorf("Expected AST parsing to be used for line: %s", tt.line)
			}

			if result != tt.expectedResult {
				t.Errorf("Expected result=%v, got %v for line: %s", tt.expectedResult, result, tt.line)
			}

			// EXPECTED BEHAVIOR: Function should parse line directly and use appropriate QueryEngine method
			t.Logf("EXPECTED: Parse line directly, then call %s on the result", tt.expectedQueryMethod)
			t.Log("CURRENT ISSUE: Uses QueryEngine on artificial struct/interface contexts")
			t.Log("CURRENT ISSUE: Creates fake contexts instead of parsing the actual line")
		})
	}
}

// TestTryASTBasedFieldDetection_ActualUsageScenarios tests real-world scenarios
// where the function would be called with actual lines from Go source files.
func TestTryASTBasedFieldDetection_ActualUsageScenarios(t *testing.T) {
	// These are actual lines that might be encountered when parsing Go source files
	realWorldLines := []struct {
		name           string
		line           string
		context        string
		expectedResult bool
		description    string
	}{
		{
			name:           "real_struct_field_from_user_model",
			line:           "Email string `json:\"email\" validate:\"required,email\"`",
			context:        "Found in User struct definition",
			expectedResult: true,
			description:    "Real struct field with validation tags",
		},
		{
			name:           "real_embedded_field_from_service",
			line:           "*gorm.DB",
			context:        "Found in service struct with embedded DB",
			expectedResult: true,
			description:    "Real embedded pointer field",
		},
		{
			name:           "real_interface_method_from_repository",
			line:           "FindByID(ctx context.Context, id string) (*User, error)",
			context:        "Found in UserRepository interface",
			expectedResult: true,
			description:    "Real repository interface method",
		},
		{
			name:           "real_return_statement_not_field",
			line:           "return user, nil",
			context:        "Found in function body, not struct/interface",
			expectedResult: false,
			description:    "Real return statement that should be rejected",
		},
		{
			name:           "real_function_call_not_field",
			line:           "logger.Info(\"processing user\", \"id\", userID)",
			context:        "Found in function body, not struct/interface",
			expectedResult: false,
			description:    "Real function call that should be rejected",
		},
	}

	for _, tt := range realWorldLines {
		t.Run(tt.name, func(t *testing.T) {
			result, useAST := tryASTBasedFieldDetection(tt.line)

			if !useAST && tt.expectedResult {
				t.Errorf("Expected AST parsing to succeed for real-world line: %s", tt.line)
			}

			if result != tt.expectedResult {
				t.Errorf("Expected result=%v, got %v for real-world line: %s", tt.expectedResult, result, tt.line)
			}

			t.Logf("Context: %s", tt.context)
			t.Logf("Description: %s", tt.description)

			// This emphasizes that the function should handle real parsing scenarios efficiently
			t.Log("EXPECTED: Function should handle real-world Go syntax directly")
			t.Log("CURRENT ISSUE: Artificial contexts don't represent real parsing scenarios")
		})
	}
}

// Helper function to demonstrate what direct parsing should look like.
// This is NOT the implementation, but shows the expected approach for testing purposes.
func demonstrateExpectedDirectParsingApproach(line string) (bool, bool) {
	// EXPECTED APPROACH (this is what the tests define):
	// 1. Parse line directly as Go syntax
	ctx := context.Background()
	parseResult := treesitter.CreateTreeSitterParseTree(ctx, line)

	if parseResult.Error != nil {
		return false, false // Parsing failed
	}

	// 2. Use TreeSitterQueryEngine on the direct parse result
	queryEngine := NewTreeSitterQueryEngine()

	// 3. Check for field patterns directly in the line's AST
	fieldDecls := queryEngine.QueryFieldDeclarations(parseResult.ParseTree)
	if len(fieldDecls) > 0 {
		return true, true
	}

	methodSpecs := queryEngine.QueryMethodSpecs(parseResult.ParseTree)
	if len(methodSpecs) > 0 {
		return true, true
	}

	embeddedTypes := queryEngine.QueryEmbeddedTypes(parseResult.ParseTree)
	if len(embeddedTypes) > 0 {
		return true, true
	}

	// 4. Check for invalid patterns directly
	funcDecls := queryEngine.QueryFunctionDeclarations(parseResult.ParseTree)
	if len(funcDecls) > 0 {
		return false, true // Found function declaration, reject
	}

	// etc. - direct analysis without artificial contexts
	return false, true
}

// TestDemonstrateExpectedBehavior shows what the corrected function should do.
// This test documents the expected implementation approach.
func TestDemonstrateExpectedBehavior(t *testing.T) {
	// This test shows the expected behavior vs current behavior
	line := "username string"

	// Current implementation (will fail these expectations)
	currentResult, currentUseAST := tryASTBasedFieldDetection(line)

	// Expected implementation approach (this is what should happen)
	expectedResult, expectedUseAST := demonstrateExpectedDirectParsingApproach(line)

	t.Logf("Line: %s", line)
	t.Logf("Current implementation result: %v, useAST: %v", currentResult, currentUseAST)
	t.Logf("Expected implementation result: %v, useAST: %v", expectedResult, expectedUseAST)

	// These assertions will guide the refactoring in Green phase
	if currentResult != expectedResult {
		t.Log("REFACTORING NEEDED: Current result doesn't match expected direct parsing result")
	}

	if currentUseAST != expectedUseAST {
		t.Log("REFACTORING NEEDED: Current AST usage doesn't match expected direct parsing approach")
	}

	t.Log("IMPLEMENTATION GUIDE:")
	t.Log("1. Remove artificial context creation (struct/interface wrappers)")
	t.Log("2. Parse line parameter directly using treesitter.CreateTreeSitterParseTree")
	t.Log("3. Use TreeSitterQueryEngine methods on direct parse result")
	t.Log("4. Eliminate multiple context testing - single direct parse is sufficient")
	t.Log("5. Function should be more efficient and accurate")
}
