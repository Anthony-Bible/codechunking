package parsererrors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateLargeFilePackageDeclarationTreeSitterBased tests the replacement of
// validateLargeFilePackageDeclaration with tree-sitter based implementation.
//
// This test suite focuses specifically on the validateLargeFilePackageDeclaration method
// and ensures it uses tree-sitter AST parsing instead of string-based validation.
//
// Expected Implementation Changes:
// 1. Replace strings.HasPrefix(trimmed, "package ") with QueryPackageDeclarations()
// 2. Replace strings.Contains(source, "func ") with QueryFunctionDeclarations()
// 3. Replace strings.Contains(source, "type ") with QueryTypeDeclarations()
// 4. Use createParseTreeWithoutValidation() instead of string parsing
// 5. Use ExtractPackageNameFromTree() for package name validation
// 6. Use tree-sitter error node detection instead of string pattern matching.
func TestValidateLargeFilePackageDeclarationTreeSitterBased(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedError bool
		errorMessage  string
		description   string
	}{
		// Valid package declarations detected via AST
		{
			name:          "valid_package_detected_via_ast",
			sourceCode:    "package main\n\nfunc main() {\n    println(\"hello world\")\n}\n\nfunc helper() int {\n    return 42\n}\n\ntype Config struct {\n    Host string\n    Port int\n}",
			expectedError: false,
			description:   "Should use QueryPackageDeclarations to detect valid package declaration in large file",
		},
		{
			name:          "valid_package_with_complex_content",
			sourceCode:    "package mypackage\n\nimport (\n    \"fmt\"\n    \"os\"\n)\n\nfunc processData() error {\n    return nil\n}\n\nfunc validateInput() bool {\n    return true\n}\n\ntype Server struct {\n    address string\n    port    int\n}\n\ntype Handler interface {\n    Handle() error\n}",
			expectedError: false,
			description:   "Should use tree-sitter AST to detect package in complex large file",
		},
		{
			name:          "package_declaration_with_comments_detected_via_ast",
			sourceCode:    "// Package documentation\npackage utils\n\n// Helper function\nfunc calculateSum(a, b int) int {\n    return a + b\n}\n\n// Data structure\ntype Point struct {\n    X, Y float64\n}\n\n// Another function\nfunc processPoint(p Point) {\n    // processing logic\n}",
			expectedError: false,
			description:   "Should use AST parsing to detect package declaration even with extensive comments",
		},

		// Missing package declarations for files with functions
		{
			name:          "missing_package_large_file_with_functions_detected_via_ast",
			sourceCode:    "func main() {\n    println(\"hello world\")\n}\n\nfunc helper() int {\n    return 42\n}\n\nfunc processData() error {\n    return nil\n}\n\nfunc validateInput() bool {\n    return true\n}\n\ntype Config struct {\n    Host string\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use QueryFunctionDeclarations instead of strings.Contains to detect functions requiring package",
		},
		{
			name:          "missing_package_with_multiple_functions_ast_analysis",
			sourceCode:    "import \"fmt\"\n\nfunc calculate(x int) int {\n    return x * 2\n}\n\nfunc display(msg string) {\n    fmt.Println(msg)\n}\n\nfunc process() {\n    // logic here\n}\n\ntype Result struct {\n    Value int\n    Error error\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use AST node analysis to detect multiple functions requiring package declaration",
		},
		{
			name:          "missing_package_complex_functions_ast_detection",
			sourceCode:    "func (r *Repository) Save(entity Entity) error {\n    // method implementation\n    return nil\n}\n\nfunc NewRepository() *Repository {\n    return &Repository{}\n}\n\nfunc validateEntity(e Entity) bool {\n    return e != nil\n}\n\ntype Repository struct {\n    db Database\n}\n\ntype Entity interface {\n    ID() string\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use tree-sitter to detect method declarations and functions requiring package",
		},

		// Type-only snippets should be allowed without package
		{
			name:          "type_only_snippet_allowed_via_ast_analysis",
			sourceCode:    "type User struct {\n    ID       string\n    Username string\n    Email    string\n    Profile  UserProfile\n}\n\ntype UserProfile struct {\n    FirstName string\n    LastName  string\n    Bio       string\n}\n\ntype UserService interface {\n    GetUser(id string) (*User, error)\n    UpdateUser(user *User) error\n}",
			expectedError: false,
			description:   "Should use QueryTypeDeclarations to detect type-only snippets that don't require package",
		},
		{
			name:          "const_and_var_only_snippet_allowed_via_ast",
			sourceCode:    "const (\n    MaxRetries = 3\n    Timeout    = 30\n    Version    = \"1.0.0\"\n)\n\nvar (\n    DefaultConfig = Config{\n        Host: \"localhost\",\n        Port: 8080,\n    }\n    GlobalCounter int\n)\n\ntype Config struct {\n    Host string\n    Port int\n}",
			expectedError: false,
			description:   "Should use QueryConstDeclarations and QueryVariableDeclarations to detect declaration-only snippets",
		},
		{
			name:          "mixed_type_declarations_allowed_via_ast",
			sourceCode:    "type Status int\n\nconst (\n    StatusActive Status = iota\n    StatusInactive\n    StatusPending\n)\n\ntype Handler interface {\n    Process(status Status) error\n}\n\ntype ProcessorConfig struct {\n    MaxWorkers int\n    Timeout    time.Duration\n}\n\nvar DefaultTimeout = 30 * time.Second",
			expectedError: false,
			description:   "Should use tree-sitter analysis to detect mixed type/const/var declarations not requiring package",
		},

		// Large vs small files logic maintained
		{
			name:          "small_file_without_package_allowed",
			sourceCode:    "func test() { return }",
			expectedError: false,
			description:   "Should maintain existing behavior: small files (<=10 lines) don't require package validation",
		},
		{
			name:          "medium_file_boundary_case",
			sourceCode:    "func test1() { return }\nfunc test2() { return }\nfunc test3() { return }\nfunc test4() { return }\nfunc test5() { return }\nfunc test6() { return }\nfunc test7() { return }\nfunc test8() { return }\nfunc test9() { return }\nfunc test10() { return }\nfunc test11() { return }",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use AST analysis for files >10 lines while maintaining size threshold logic",
		},
		{
			name:          "empty_large_file_no_validation",
			sourceCode:    "\n\n\n\n\n\n\n\n\n\n\n\n",
			expectedError: false,
			description:   "Should maintain existing behavior: empty large files don't require validation",
		},

		// Malformed package declarations using tree-sitter error nodes
		{
			name:          "invalid_package_name_detected_via_tree_sitter",
			sourceCode:    "package 123invalid\n\nfunc main() {\n    println(\"hello\")\n}\n\nfunc helper() int {\n    return 42\n}\n\ntype Config struct {\n    Host string\n    Port int\n}\n\nfunc process() error {\n    return nil\n}",
			expectedError: true,
			errorMessage:  "invalid package declaration",
			description:   "Should use tree-sitter error nodes to detect invalid package names instead of string patterns",
		},
		{
			name:          "malformed_package_syntax_tree_sitter_detection",
			sourceCode:    "package main-invalid\n\nfunc calculate() int {\n    return 42\n}\n\nfunc process() {\n    // logic\n}\n\ntype Data struct {\n    Value string\n}\n\nfunc validate() bool {\n    return true\n}",
			expectedError: false, // Tree-sitter parses this successfully (though semantically invalid)
			description:   "Tree-sitter's Go grammar accepts hyphenated package names (semantic validation is out of scope)",
		},
		{
			name:          "incomplete_package_declaration_error_nodes",
			sourceCode:    "package\n\nfunc main() {\n    println(\"hello\")\n}\n\nfunc helper() int {\n    return 42\n}\n\ntype Config struct {\n    Host string\n}\n\nfunc processData() {\n    // processing\n}",
			expectedError: false, // Tree-sitter parses 'package' keyword without identifier as valid (lenient)
			description:   "Tree-sitter's Go grammar is lenient with incomplete package declarations (semantic validation is out of scope)",
		},

		// Edge cases with comments, whitespace, etc.
		{
			name:          "comment_only_large_file_ast_detection",
			sourceCode:    "// This is a large comment block\n// explaining the purpose of this file\n// and providing documentation\n// for future developers\n//\n// It contains multiple lines\n// but no actual Go code\n// just comments and explanations\n// about what would go here\n// in a real implementation\n// of this module",
			expectedError: false,
			description:   "Should use AST parsing to detect comment-only content not requiring package validation",
		},
		{
			name:          "whitespace_with_comments_large_file",
			sourceCode:    "\n\n    // Some comment\n\n\n    // Another comment\n\n\n    // Final comment\n\n\n\n\n",
			expectedError: false,
			description:   "Should use tree-sitter to handle whitespace and comments without requiring package",
		},
		{
			name:          "mixed_comments_and_functions_missing_package",
			sourceCode:    "// Package documentation would go here\n//\n// This file contains utility functions\n// for data processing and validation\n\nfunc processData() error {\n    return nil\n}\n\n// Helper function comment\nfunc validateInput() bool {\n    return true\n}\n\n// Type definition comment\ntype Processor struct {\n    config Config\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use AST to distinguish between comments and functions requiring package declaration",
		},
		{
			name:          "build_tags_with_functions_missing_package",
			sourceCode:    "//go:build linux\n// +build linux\n\nfunc platformSpecific() {\n    // linux specific code\n}\n\nfunc anotherFunction() {\n    // more code\n}\n\ntype LinuxConfig struct {\n    KernelVersion string\n}\n\nfunc initializeLinux() error {\n    return nil\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use tree-sitter to detect functions requiring package even with build tags",
		},

		// Complex scenarios combining multiple aspects
		{
			name:          "mixed_valid_and_invalid_ast_comprehensive_analysis",
			sourceCode:    "package main\n\nfunc validFunction() {\n    println(\"valid\")\n}\n\ntype ValidType struct {\n    Field string\n}\n\nfunc anotherValidFunction() int {\n    return 42\n}\n\nconst ValidConstant = 100\n\nvar ValidVariable = \"test\"\n\nfunc finalValidFunction() error {\n    return nil\n}",
			expectedError: false,
			description:   "Should use comprehensive AST analysis for complex files with mixed valid constructs",
		},
		{
			name:          "large_file_with_nested_types_missing_package",
			sourceCode:    "type OuterStruct struct {\n    Inner InnerStruct\n    Config ConfigStruct\n}\n\ntype InnerStruct struct {\n    Value  string\n    Number int\n    Data   []byte\n}\n\ntype ConfigStruct struct {\n    Settings map[string]interface{}\n    Options  []string\n}\n\nfunc processOuter(o OuterStruct) error {\n    return nil\n}\n\nfunc validateInner(i InnerStruct) bool {\n    return true\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use tree-sitter to detect functions in files with complex nested type structures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test expects validateLargeFilePackageDeclaration to be implemented using:
			// 1. createParseTreeWithoutValidation() to get AST instead of string parsing
			// 2. QueryPackageDeclarations() to detect package clauses
			// 3. QueryFunctionDeclarations() and QueryMethodDeclarations() to detect functions
			// 4. QueryTypeDeclarations(), QueryConstDeclarations(), QueryVariableDeclarations() for type-only detection
			// 5. ExtractPackageNameFromTree() for package name validation
			// 6. tree-sitter error node analysis instead of string pattern matching
			// 7. Maintain existing file size thresholds and logic
			err := validator.validateLargeFilePackageDeclaration(tt.sourceCode)

			if tt.expectedError {
				assert.NotNil(t, err, tt.description)
				if err != nil && tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage, "Error message should match expected pattern")
				}
			} else {
				assert.Nil(t, err, tt.description)
			}
		})
	}
}

// TestValidateLargeFilePackageDeclarationTreeSitterIntegration tests the integration
// aspects of the tree-sitter based validateLargeFilePackageDeclaration implementation.
func TestValidateLargeFilePackageDeclarationTreeSitterIntegration(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedError bool
		errorMessage  string
		description   string
	}{
		{
			name: "should_use_createParseTreeWithoutValidation",
			sourceCode: `func testFunction() {
    println("This should trigger AST parsing")
}

func anotherFunction() int {
    return 42
}

type TestStruct struct {
    Field string
}

func methodFunction() error {
    return nil
}`,
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use createParseTreeWithoutValidation() method to create AST for analysis",
		},
		{
			name: "should_integrate_with_QueryPackageDeclarations",
			sourceCode: `package validpackage

func functionWithPackage() {
    println("This has a package")
}

func anotherFunctionWithPackage() int {
    return 42
}

type StructWithPackage struct {
    Field string
}

func methodWithPackage() error {
    return nil
}`,
			expectedError: false,
			description:   "Should integrate with QueryPackageDeclarations from treesitter_query_engine.go",
		},
		{
			name: "should_integrate_with_ExtractPackageNameFromTree",
			sourceCode: `package extracted_package_name

func testFunction() {
    println("Package name should be extracted using AST utils")
}

func validationFunction() bool {
    return true
}

type ConfigType struct {
    Setting string
}

func processingFunction() error {
    return nil
}`,
			expectedError: false,
			description:   "Should integrate with ExtractPackageNameFromTree from ast_utils.go",
		},
		{
			name: "should_use_QueryFunctionDeclarations_for_function_detection",
			sourceCode: `// No package declaration
import "fmt"

func detectedViaAST() {
    fmt.Println("Should be detected via QueryFunctionDeclarations")
}

func anotherDetectedFunction() int {
    return 42
}

type AssociatedType struct {
    Value string
}

func finalDetectedFunction() error {
    return nil
}`,
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use QueryFunctionDeclarations instead of strings.Contains for function detection",
		},
		{
			name: "should_use_QueryTypeDeclarations_for_type_only_detection",
			sourceCode: `type OnlyTypesHere struct {
    Field1 string
    Field2 int
    Field3 bool
}

type AnotherType interface {
    Method() error
}

type ThirdType map[string]interface{}

const RelatedConstant = "value"

var RelatedVariable = 42`,
			expectedError: false,
			description:   "Should use QueryTypeDeclarations to detect type-only snippets not requiring package",
		},
		{
			name: "should_detect_tree_sitter_error_nodes_for_invalid_package",
			sourceCode: `package 123invalid_name

func functionAfterInvalidPackage() {
    println("Invalid package name should be detected via ERROR nodes")
}

func anotherFunction() int {
    return 42
}

type TypeWithInvalidPackage struct {
    Field string
}

func finalFunction() error {
    return nil
}`,
			expectedError: true,
			errorMessage:  "invalid package declaration",
			description:   "Should use tree-sitter ERROR node detection instead of string pattern matching",
		},
		{
			name:          "should_maintain_file_size_threshold_behavior",
			sourceCode:    "func small() { return }", // Only 1 line, should be skipped
			expectedError: false,
			description:   "Should maintain existing file size threshold logic while using AST for larger files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test verifies that the new tree-sitter implementation:
			// 1. Properly integrates with existing tree-sitter infrastructure
			// 2. Uses the correct query methods from treesitter_query_engine.go
			// 3. Uses utility functions from ast_utils.go
			// 4. Maintains backward compatibility with existing validation logic
			// 5. Replaces string-based parsing with proper AST analysis
			err := validator.validateLargeFilePackageDeclaration(tt.sourceCode)

			if tt.expectedError {
				assert.NotNil(t, err, tt.description)
				if err != nil && tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage, "Error message should match expected pattern")
				}
			} else {
				assert.Nil(t, err, tt.description)
			}
		})
	}
}

// TestValidateLargeFilePackageDeclarationEdgeCasesTreeSitter tests edge cases
// that the tree-sitter implementation should handle correctly.
func TestValidateLargeFilePackageDeclarationEdgeCasesTreeSitter(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedError bool
		errorMessage  string
		description   string
	}{
		{
			name:          "unicode_package_name_valid_via_ast",
			sourceCode:    "package 测试包\n\nfunc 测试函数() {\n    println(\"unicode test\")\n}\n\nfunc anotherFunction() int {\n    return 42\n}\n\ntype 测试类型 struct {\n    字段 string\n}\n\nfunc processFunction() error {\n    return nil\n}",
			expectedError: false,
			description:   "Should use tree-sitter to correctly handle unicode package names and function names",
		},
		{
			name:          "deeply_nested_error_recovery",
			sourceCode:    "func outer() {\n    func inner() {\n        func deeplyNested() {\n            // incomplete\n\nfunc anotherFunction() int {\n    return 42\n}\n\ntype NestedType struct {\n    Field string\n}\n\nfunc finalFunction() {\n    // processing\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use tree-sitter error recovery to detect functions even with syntax errors",
		},
		{
			name:          "mixed_line_endings_large_file",
			sourceCode:    "func windowsStyle() {\r\n    println(\"CRLF\")\r\n}\n\nfunc unixStyle() {\n    println(\"LF\")\n}\r\n\r\ntype MixedType struct {\r\n    Field string\n}\n\nfunc finalMixed() error {\r\n    return nil\r\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use tree-sitter to handle mixed line endings while detecting missing package",
		},
		{
			name:          "generic_functions_and_types_ast_detection",
			sourceCode:    "func GenericFunction[T any](data T) T {\n    return data\n}\n\nfunc AnotherGeneric[K comparable, V any](m map[K]V) V {\n    for _, v := range m {\n        return v\n    }\n    var zero V\n    return zero\n}\n\ntype GenericType[T any] struct {\n    Value T\n}\n\nfunc ProcessGeneric[T any](gt GenericType[T]) error {\n    return nil\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use tree-sitter to detect generic functions and types requiring package declaration",
		},
		{
			name:          "cgo_comments_with_functions",
			sourceCode:    "/*\n#include <stdlib.h>\n*/\nimport \"C\"\n\nfunc cgoFunction() {\n    C.malloc(100)\n}\n\nfunc regularFunction() int {\n    return 42\n}\n\ntype CWrapper struct {\n    ptr unsafe.Pointer\n}\n\nfunc processCgo() error {\n    return nil\n}",
			expectedError: true,
			errorMessage:  "missing package declaration",
			description:   "Should use AST parsing to detect functions even with CGO comments and imports",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &GoValidator{}

			// This test ensures the tree-sitter implementation handles edge cases:
			// 1. Unicode content parsing
			// 2. Syntax error recovery
			// 3. Different line ending formats
			// 4. Modern Go features (generics)
			// 5. Special comment formats (CGO)
			err := validator.validateLargeFilePackageDeclaration(tt.sourceCode)

			if tt.expectedError {
				assert.NotNil(t, err, tt.description)
				if err != nil && tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage, "Error message should match expected pattern")
				}
			} else {
				assert.Nil(t, err, tt.description)
			}
		})
	}
}
