package goparser

import (
	"testing"
)

// TestTryASTBasedFieldDetection_ShouldParseLineDirectly tests that the function
// correctly handles field/method detection using context-based parsing.
// NOTE: Tree-sitter's Go grammar requires structural context (struct/interface wrappers)
// to properly parse field declarations and method signatures.
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
			description:    "Should detect simple typed field using context-based parsing",
		},
		{
			name:           "tagged_field",
			line:           "ID int `json:\"id\"`",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Should detect tagged field using context-based parsing",
		},
		{
			name:           "embedded_type",
			line:           "io.Reader",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Should detect embedded type using context-based parsing",
		},
		{
			name:           "interface_method",
			line:           "Read([]byte) (int, error)",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Should detect interface method using context-based parsing",
		},
		{
			name:           "return_statement",
			line:           "return nil",
			expectedResult: false,
			expectedUseAST: false,
			description:    "Should reject return statement without AST parsing",
		},
		{
			name:           "import_statement",
			line:           "import \"context\"",
			expectedResult: false,
			expectedUseAST: false,
			description:    "Should reject import statement without AST parsing",
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
		})
	}
}

// TestTryASTBasedFieldDetection_DirectLineParsing tests field/method detection
// using context-based parsing when direct parsing fails.
// Tree-sitter requires structural context for field declarations and method signatures.
func TestTryASTBasedFieldDetection_DirectLineParsing(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		expectedResult bool
		expectedUseAST bool
		description    string
	}{
		{
			name:           "field_declaration_with_type",
			line:           "username string",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Context-based parsing should identify field declaration",
		},
		{
			name:           "field_with_pointer_type",
			line:           "data *User",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Context-based parsing should handle pointer types",
		},
		{
			name:           "field_with_slice_type",
			line:           "items []string",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Context-based parsing should handle slice types",
		},
		{
			name:           "field_with_map_type",
			line:           "metadata map[string]interface{}",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Context-based parsing should handle complex map types",
		},
		{
			name:           "method_signature",
			line:           "Process(data []byte) error",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Context-based parsing should identify interface method specs",
		},
		{
			name:           "embedded_interface",
			line:           "io.Writer",
			expectedResult: true,
			expectedUseAST: true,
			description:    "Context-based parsing should identify embedded types",
		},
		{
			name:           "function_call_not_field",
			line:           "fmt.Println(\"hello\")",
			expectedResult: false,
			expectedUseAST: true,
			description:    "Should reject function calls even with context",
		},
		{
			name:           "variable_assignment_not_field",
			line:           "x := 42",
			expectedResult: false,
			expectedUseAST: false,
			description:    "Should reject variable assignments before parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, useAST := tryASTBasedFieldDetection(tt.line)

			if useAST != tt.expectedUseAST {
				t.Errorf("Expected useAST=%v, got %v", tt.expectedUseAST, useAST)
			}

			if result != tt.expectedResult {
				t.Errorf("Expected result=%v, got %v", tt.expectedResult, result)
			}
		})
	}
}

// TestTryASTBasedFieldDetection_EdgeCases tests edge cases that should be handled
// correctly with context-based parsing.
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
// properly utilizes TreeSitterQueryEngine methods for field detection.
// Uses context-based parsing with QueryEngine methods on struct/interface wrappers.
func TestTryASTBasedFieldDetection_ShouldUseTreeSitterQueryEngine(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		expectedResult bool
		description    string
	}{
		{
			name:           "struct_field_detected_via_QueryFieldDeclarations",
			line:           "id int64",
			expectedResult: true,
			description:    "Struct fields detected using QueryFieldDeclarations on context-wrapped code",
		},
		{
			name:           "interface_method_detected_via_QueryMethodSpecs",
			line:           "Write([]byte) (int, error)",
			expectedResult: true,
			description:    "Interface methods detected using QueryMethodSpecs on context-wrapped code",
		},
		{
			name:           "embedded_type_detected_correctly",
			line:           "sync.Mutex",
			expectedResult: true,
			description:    "Embedded types detected using appropriate query methods",
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
		})
	}
}

// TestTryASTBasedFieldDetection_ActualUsageScenarios tests real-world scenarios
// where the function is called with actual lines from Go source files.
// Verifies that context-based parsing handles real-world field/method detection correctly.
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
		})
	}
}
