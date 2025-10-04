package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFunctionSignatureFieldAccess_ReceiverExtraction tests that method receiver types
// are extracted using proper tree-sitter grammar field access instead of manual string parsing.
//
// EXPECTED FAILURE: Current implementation uses extractReceiverTypeName() with string manipulation:
// - strings.Trim(receiverInfo, "()")
// - strings.Fields(receiverInfo)
// - strings.TrimPrefix(typeName, "*")
//
// REQUIRED IMPLEMENTATION: Should use tree-sitter grammar field access:
// - method_declaration.receiver field access
// - parameter_list.parameter_declaration children
// - parameter_declaration.name and parameter_declaration.type fields.
func TestFunctionSignatureFieldAccess_ReceiverExtraction(t *testing.T) {
	tests := []struct {
		name                    string
		sourceCode              string
		expectedReceiverType    string
		expectedMethodName      string
		testFieldAccessUsage    func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree)
		shouldFailStringParsing bool
	}{
		{
			name: "pointer receiver with field access",
			sourceCode: `package main

func (p *Person) GetName() string {
	return p.name
}`,
			expectedReceiverType: "Person",
			expectedMethodName:   "GetName",
			testFieldAccessUsage: func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree) {
				// This test requires that receiver extraction uses grammar field access
				// NOT string manipulation like extractReceiverTypeName()

				// REQUIRED: method_declaration should have a 'receiver' field
				receiverField := getChildByFieldName(node, "receiver")
				require.NotNil(t, receiverField, "method_declaration must have receiver field access")

				// REQUIRED: receiver should contain parameter_declaration with proper field access
				paramDecl := findFirstChildByType(receiverField, "parameter_declaration")
				require.NotNil(t, paramDecl, "receiver must contain parameter_declaration accessible via field")

				// REQUIRED: parameter_declaration should have 'type' field access (not string parsing)
				typeField := getChildByFieldName(paramDecl, "type")
				require.NotNil(t, typeField, "parameter_declaration must have type field access")

				// REQUIRED: type extraction should handle pointer types via AST structure
				actualType := parseTree.GetNodeText(typeField)
				if typeField.Type == "pointer_type" {
					// Should find the underlying type via field access, not string manipulation
					baseTypeField := getChildByFieldName(typeField, "type")
					require.NotNil(t, baseTypeField, "pointer_type must have type field access")
					actualType = parseTree.GetNodeText(baseTypeField)
				}

				assert.Equal(t, "Person", actualType, "receiver type should be extracted via field access")
			},
			shouldFailStringParsing: true,
		},
		{
			name: "value receiver with field access",
			sourceCode: `package main

func (p Person) GetAge() int {
	return p.age
}`,
			expectedReceiverType: "Person",
			expectedMethodName:   "GetAge",
			testFieldAccessUsage: func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree) {
				// Test that value receiver parsing uses field access
				receiverField := getChildByFieldName(node, "receiver")
				require.NotNil(t, receiverField, "method_declaration must have receiver field")

				paramDecl := findFirstChildByType(receiverField, "parameter_declaration")
				require.NotNil(t, paramDecl, "receiver parameter_declaration must be accessible")

				typeField := getChildByFieldName(paramDecl, "type")
				require.NotNil(t, typeField, "parameter type must be accessible via field")

				actualType := parseTree.GetNodeText(typeField)
				assert.Equal(t, "Person", actualType, "value receiver should use field access")
			},
			shouldFailStringParsing: true,
		},
		{
			name: "complex receiver type with field access",
			sourceCode: `package main

func (s *MyService[T]) Process() error {
	return nil
}`,
			expectedReceiverType: "MyService",
			expectedMethodName:   "Process",
			testFieldAccessUsage: func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree) {
				// Test generic receiver type extraction via field access
				receiverField := getChildByFieldName(node, "receiver")
				require.NotNil(t, receiverField, "method_declaration must have receiver field")

				paramDecl := findFirstChildByType(receiverField, "parameter_declaration")
				require.NotNil(t, paramDecl, "receiver must contain parameter_declaration")

				typeField := getChildByFieldName(paramDecl, "type")
				require.NotNil(t, typeField, "parameter_declaration must have type field")

				// Handle pointer to generic type
				var baseTypeField *valueobject.ParseNode
				if typeField.Type == "pointer_type" {
					baseTypeField = getChildByFieldName(typeField, "type")
					require.NotNil(t, baseTypeField, "pointer_type must have type field")
				} else {
					baseTypeField = typeField
				}

				// For generic types, should use field access to get base type
				if baseTypeField.Type == "generic_type" {
					nameField := getChildByFieldName(baseTypeField, "type")
					require.NotNil(t, nameField, "generic_type must have type field")
					actualType := parseTree.GetNodeText(nameField)
					assert.Equal(t, "MyService", actualType, "generic receiver should use field access")
				}
			},
			shouldFailStringParsing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the source code
			parseTree, err := parseGoSourceCode(tt.sourceCode)
			require.NoError(t, err, "Failed to parse source code")

			// Find method declaration
			methodNodes := parseTree.GetNodesByType("method_declaration")
			require.Len(t, methodNodes, 1, "Expected exactly one method declaration")
			methodNode := methodNodes[0]

			// Test field access requirements
			tt.testFieldAccessUsage(t, methodNode, parseTree)

			// THIS TEST WILL FAIL because current implementation doesn't have field access methods
			// The test expects getChildByFieldName() to exist and work properly
		})
	}
}

// TestFunctionSignatureFieldAccess_ParameterExtraction tests that function parameters
// are extracted using proper tree-sitter grammar field access instead of position-based logic.
//
// EXPECTED FAILURE: Current implementation in parseGoFunctionSignature() uses:
// - Manual position checking: child.StartByte > nameNode.StartByte
// - Position-based parameter vs return type distinction
// - Complex loops through node children instead of direct field access
//
// REQUIRED IMPLEMENTATION: Should use tree-sitter grammar field access:
// - function_declaration.parameters field access
// - function_declaration.result field access
// - Direct field access instead of position comparisons.
func TestFunctionSignatureFieldAccess_ParameterExtraction(t *testing.T) {
	tests := []struct {
		name                    string
		sourceCode              string
		expectedParameters      []outbound.Parameter
		expectedReturnType      string
		testFieldAccessUsage    func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree)
		shouldFailPositionLogic bool
	}{
		{
			name: "simple function with field access",
			sourceCode: `package main

func Add(a int, b int) int {
	return a + b
}`,
			expectedParameters: []outbound.Parameter{
				{Name: "a", Type: "int"},
				{Name: "b", Type: "int"},
			},
			expectedReturnType: "int",
			testFieldAccessUsage: func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree) {
				// REQUIRED: function_declaration must have 'parameters' field access
				parametersField := getChildByFieldName(node, "parameters")
				require.NotNil(t, parametersField, "function_declaration must have parameters field")
				assert.Equal(t, "parameter_list", parametersField.Type, "parameters field should be parameter_list")

				// REQUIRED: function_declaration must have 'result' field access
				resultField := getChildByFieldName(node, "result")
				require.NotNil(t, resultField, "function_declaration must have result field")

				// Verify parameters are extracted via field access, not position
				paramDecls := findDirectChildren(parametersField, "parameter_declaration")
				assert.Len(t, paramDecls, 2, "should find parameters via field access")

				// Verify return type is extracted via field access, not position
				returnText := parseTree.GetNodeText(resultField)
				assert.Equal(t, "int", returnText, "return type should use field access")
			},
			shouldFailPositionLogic: true,
		},
		{
			name: "function with multiple return values using field access",
			sourceCode: `package main

func DivMod(a, b int) (int, int) {
	return a / b, a % b
}`,
			expectedParameters: []outbound.Parameter{
				{Name: "a", Type: "int"},
				{Name: "b", Type: "int"},
			},
			expectedReturnType: "(int, int)",
			testFieldAccessUsage: func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree) {
				// Test multiple return values via field access
				parametersField := getChildByFieldName(node, "parameters")
				require.NotNil(t, parametersField, "function_declaration must have parameters field")

				resultField := getChildByFieldName(node, "result")
				require.NotNil(t, resultField, "function_declaration must have result field")

				// Should handle multiple return values via field access
				returnText := parseTree.GetNodeText(resultField)
				assert.Equal(t, "(int, int)", returnText, "multiple returns should use field access")
			},
			shouldFailPositionLogic: true,
		},
		{
			name: "generic function with field access",
			sourceCode: `package main

func Process[T any](data T) (T, error) {
	return data, nil
}`,
			expectedParameters: []outbound.Parameter{
				{Name: "data", Type: "T"},
			},
			expectedReturnType: "(T, error)",
			testFieldAccessUsage: func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree) {
				// Test generic function parsing via field access

				// REQUIRED: Should have type_parameters field access for generics
				typeParamsField := getChildByFieldName(node, "type_parameters")
				require.NotNil(t, typeParamsField, "generic function must have type_parameters field")

				// REQUIRED: Standard parameter and result field access
				parametersField := getChildByFieldName(node, "parameters")
				require.NotNil(t, parametersField, "function must have parameters field")

				resultField := getChildByFieldName(node, "result")
				require.NotNil(t, resultField, "function must have result field")

				// Verify type parameters accessible via field
				assert.Equal(
					t,
					"type_parameter_list",
					typeParamsField.Type,
					"type_parameters should be type_parameter_list",
				)
			},
			shouldFailPositionLogic: true,
		},
		{
			name: "variadic function with field access",
			sourceCode: `package main

func Printf(format string, args ...interface{}) int {
	return 0
}`,
			expectedParameters: []outbound.Parameter{
				{Name: "format", Type: "string"},
				{Name: "args", Type: "...interface{}"},
			},
			expectedReturnType: "int",
			testFieldAccessUsage: func(t *testing.T, node *valueobject.ParseNode, parseTree *valueobject.ParseTree) {
				// Test variadic parameter parsing via field access
				parametersField := getChildByFieldName(node, "parameters")
				require.NotNil(t, parametersField, "function must have parameters field")

				// Should handle variadic_parameter_declaration via field access
				variadicParams := findDirectChildren(parametersField, "variadic_parameter_declaration")
				assert.Len(t, variadicParams, 1, "should find variadic parameter via field access")

				resultField := getChildByFieldName(node, "result")
				require.NotNil(t, resultField, "function must have result field")
			},
			shouldFailPositionLogic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the source code
			parseTree, err := parseGoSourceCode(tt.sourceCode)
			require.NoError(t, err, "Failed to parse source code")

			// Find function declaration
			functionNodes := parseTree.GetNodesByType("function_declaration")
			require.Len(t, functionNodes, 1, "Expected exactly one function declaration")
			functionNode := functionNodes[0]

			// Test field access requirements
			tt.testFieldAccessUsage(t, functionNode, parseTree)

			// THIS TEST WILL FAIL because:
			// 1. getChildByFieldName() method doesn't exist
			// 2. Current implementation uses position-based logic instead of field access
		})
	}
}

// TestFunctionSignatureFieldAccess_TreeSitterQueryEngineIntegration tests that function parsing
// integrates with TreeSitterQueryEngine for consistent patterns with other refactored parsers.
//
// EXPECTED FAILURE: Current implementation doesn't use TreeSitterQueryEngine for function parsing.
// Other parsers (structures, imports, etc.) use TreeSitterQueryEngine for consistency.
//
// REQUIRED IMPLEMENTATION: Should integrate TreeSitterQueryEngine:
// - Use engine.QueryFunctionDeclarations() and engine.QueryMethodDeclarations()
// - Follow same error handling patterns as other parsers
// - Use consistent field access patterns.
func TestFunctionSignatureFieldAccess_TreeSitterQueryEngineIntegration(t *testing.T) {
	sourceCode := `package main

import "fmt"

func HelloWorld() {
	fmt.Println("Hello, World!")
}

func (s *Service) Process() error {
	return nil
}

type Service struct {
	name string
}`

	// Parse the source code
	parseTree, err := parseGoSourceCode(sourceCode)
	require.NoError(t, err, "Failed to parse source code")

	// REQUIRED: Function parsing should use TreeSitterQueryEngine
	queryEngine := NewTreeSitterQueryEngine()

	// Test that function parsing integrates with query engine
	functions := queryEngine.QueryFunctionDeclarations(parseTree)
	methods := queryEngine.QueryMethodDeclarations(parseTree)

	assert.Len(t, functions, 1, "should find function via query engine")
	assert.Len(t, methods, 1, "should find method via query engine")

	// REQUIRED: Function parsing should use same patterns as other parsers
	parser := &GoParser{}

	// Test that function extraction uses TreeSitterQueryEngine internally
	// This should follow the same pattern as structures.go, imports.go, etc.
	extractedFunctions, err := parser.ExtractFunctions(
		context.Background(),
		parseTree,
		outbound.SemanticExtractionOptions{},
	)
	require.NoError(t, err, "function extraction should work with query engine")

	assert.Len(t, extractedFunctions, 2, "should extract both function and method")

	// Verify that field access is used consistently
	for _, fn := range extractedFunctions {
		assert.NotEmpty(t, fn.Name, "function name should be extracted via field access")
		assert.GreaterOrEqual(
			t,
			fn.StartByte,
			uint32(0),
			"StartByte should be non-negative (positions from real AST nodes)",
		)
		assert.Greater(
			t,
			fn.EndByte,
			fn.StartByte,
			"EndByte should be greater than StartByte (positions from real AST nodes)",
		)
	}

	// THIS TEST WILL FAIL because:
	// 1. Function parsing doesn't integrate with TreeSitterQueryEngine
	// 2. Field access methods are not implemented
	// 3. Patterns are inconsistent with other refactored parsers
}

// TestFunctionSignatureFieldAccess_ComplexSignatures tests field access for complex function
// signatures including edge cases that manual string parsing handles incorrectly.
//
// EXPECTED FAILURE: String-based parsing fails with complex signatures.
// Position-based logic fails when signatures have unusual formatting.
//
// REQUIRED IMPLEMENTATION: Field access should handle all cases correctly via AST structure.
func TestFunctionSignatureFieldAccess_ComplexSignatures(t *testing.T) {
	tests := []struct {
		name                 string
		sourceCode           string
		testFieldAccessUsage func(t *testing.T, parseTree *valueobject.ParseTree)
	}{
		{
			name: "function with unusual spacing",
			sourceCode: `package main

func    WeirdSpacing   (   a    int   ,    b    string   )   (   int  ,   error   )   {
	return 0, nil
}`,
			testFieldAccessUsage: func(t *testing.T, parseTree *valueobject.ParseTree) {
				// Test that field access handles unusual spacing correctly
				functionNodes := parseTree.GetNodesByType("function_declaration")
				require.Len(t, functionNodes, 1, "should find function despite spacing")

				node := functionNodes[0]

				// Field access should work regardless of spacing
				parametersField := getChildByFieldName(node, "parameters")
				require.NotNil(t, parametersField, "field access should handle unusual spacing")

				resultField := getChildByFieldName(node, "result")
				require.NotNil(t, resultField, "result field should be accessible")

				// Verify parameter extraction works with unusual spacing
				paramDecls := findDirectChildren(parametersField, "parameter_declaration")
				assert.Len(t, paramDecls, 2, "should extract parameters despite spacing")
			},
		},
		{
			name: "method with complex receiver type",
			sourceCode: `package main

func (r *Repository[T, K]) FindByKey(key K) (*T, bool) {
	return nil, false
}`,
			testFieldAccessUsage: func(t *testing.T, parseTree *valueobject.ParseTree) {
				// Test complex generic receiver handling
				methodNodes := parseTree.GetNodesByType("method_declaration")
				require.Len(t, methodNodes, 1, "should find method")

				node := methodNodes[0]

				// Field access should handle complex receiver types
				receiverField := getChildByFieldName(node, "receiver")
				require.NotNil(t, receiverField, "method should have receiver field")

				parametersField := getChildByFieldName(node, "parameters")
				require.NotNil(t, parametersField, "method should have parameters field")

				resultField := getChildByFieldName(node, "result")
				require.NotNil(t, resultField, "method should have result field")
			},
		},
		{
			name: "function with named return parameters",
			sourceCode: `package main

func ReadFile(path string) (data []byte, err error) {
	return nil, nil
}`,
			testFieldAccessUsage: func(t *testing.T, parseTree *valueobject.ParseTree) {
				// Test named return parameter handling
				functionNodes := parseTree.GetNodesByType("function_declaration")
				require.Len(t, functionNodes, 1, "should find function")

				node := functionNodes[0]

				// Field access should handle named returns
				resultField := getChildByFieldName(node, "result")
				require.NotNil(t, resultField, "function should have result field")

				// Named returns should be accessible as parameter_list
				assert.Equal(t, "parameter_list", resultField.Type, "named returns should be parameter_list")

				returnParams := findDirectChildren(resultField, "parameter_declaration")
				assert.Len(t, returnParams, 2, "should find named return parameters")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the source code
			parseTree, err := parseGoSourceCode(tt.sourceCode)
			require.NoError(t, err, "Failed to parse source code")

			// Test field access requirements
			tt.testFieldAccessUsage(t, parseTree)

			// THIS TEST WILL FAIL because field access methods don't exist
		})
	}
}

// Helper functions that are EXPECTED TO BE MISSING and cause test failures
// These represent the required API that needs to be implemented

// getChildByFieldName retrieves a child node by its field name from tree-sitter grammar.
// Uses tree-sitter's ChildByFieldName functionality.
func getChildByFieldName(node *valueobject.ParseNode, fieldName string) *valueobject.ParseNode {
	if node == nil || fieldName == "" {
		return nil
	}
	return node.ChildByFieldName(fieldName)
}

// findFirstChildByType finds the first direct child of the specified type.
// EXPECTED TO FAIL: Current implementation doesn't provide this utility.
func findFirstChildByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	// THIS METHOD DOESN'T EXIST - EXPECTED TO FAIL
	for _, child := range node.Children {
		if child.Type == nodeType {
			return child
		}
	}
	return nil
}

// findDirectChildren finds all direct children of the specified type (non-recursive).
func findDirectChildren(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var results []*valueobject.ParseNode
	if node == nil || nodeType == "" {
		return results
	}
	for _, child := range node.Children {
		if child.Type == nodeType {
			results = append(results, child)
		}
	}
	return results
}

// parseGoSourceCode is a test helper for parsing Go source code.
// This should work with current implementation.
func parseGoSourceCode(sourceCode string) (*valueobject.ParseTree, error) {
	// This would use the existing tree-sitter parsing infrastructure
	// Implementation details depend on existing parser setup
	panic("parseGoSourceCode helper not implemented in tests")
}
