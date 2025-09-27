package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFunctionFieldAccessIntegration tests that function parsing integrates properly
// with TreeSitterQueryEngine and uses consistent field access patterns like other parsers.
//
// EXPECTED FAILURE: Current function parsing doesn't integrate with TreeSitterQueryEngine
// or use the same field access patterns as structures.go, imports.go, etc.
//
// REQUIRED IMPLEMENTATION:
// 1. Function parsing should use TreeSitterQueryEngine for consistency
// 2. Should implement field access API similar to other parsers
// 3. Should eliminate manual string parsing and position-based logic.
func TestFunctionFieldAccessIntegration(t *testing.T) {
	// Complex Go source with various function types
	sourceCode := `package main

import (
	"context"
	"fmt"
)

// BasicFunction demonstrates simple function parsing
func BasicFunction(param string) error {
	fmt.Println(param)
	return nil
}

// GenericFunction demonstrates generic function parsing
func GenericFunction[T comparable](data T) (T, bool) {
	var zero T
	return data, data != zero
}

// VariadicFunction demonstrates variadic parameter parsing
func VariadicFunction(format string, args ...interface{}) int {
	return fmt.Sprintf(format, args...)
}

// MultipleReturnsFunction demonstrates multiple return values
func MultipleReturnsFunction(input string) (result string, count int, err error) {
	return input, len(input), nil
}

// ComplexSignatureFunction demonstrates complex parameter types
func ComplexSignatureFunction(
	ctx context.Context,
	data map[string]interface{},
	handler func(string) error,
	opts ...func(*Config),
) (*Response, error) {
	return nil, nil
}

// Service demonstrates method parsing requirements
type Service struct {
	name string
}

// SimpleMethod demonstrates basic method parsing
func (s *Service) SimpleMethod() string {
	return s.name
}

// GenericMethod demonstrates generic method parsing
func (s *Service[T]) GenericMethod(data T) (*T, error) {
	return &data, nil
}

// ComplexReceiverMethod demonstrates complex receiver types
func (r *Repository[K comparable, V any]) FindByKey(key K) (V, bool) {
	var zero V
	return zero, false
}

// Config and Response types for testing
type Config struct{}
type Response struct{}
type Repository[K comparable, V any] struct{}
`

	t.Run("TreeSitterQueryEngine Integration", func(t *testing.T) {
		// EXPECTED FAILURE: This test will fail because function parsing doesn't
		// integrate with TreeSitterQueryEngine like other parsers do

		// Parse the source code (this helper would need to be implemented)
		parseTree := createMockParseTree(sourceCode)

		// REQUIRED: Function parsing should use TreeSitterQueryEngine
		queryEngine := NewTreeSitterQueryEngine()

		// Test that query engine can find functions and methods
		functions := queryEngine.QueryFunctionDeclarations(parseTree)
		methods := queryEngine.QueryMethodDeclarations(parseTree)

		// Verify expected counts
		assert.Len(t, functions, 5, "should find 5 function declarations via query engine")
		assert.Len(t, methods, 3, "should find 3 method declarations via query engine")

		// REQUIRED: Function extraction should integrate with query engine internally
		parser := &GoParser{}
		extractedChunks, err := parser.ExtractFunctions(
			context.Background(),
			parseTree,
			outbound.SemanticExtractionOptions{},
		)
		require.NoError(t, err, "function extraction should work with query engine integration")

		// Should extract all functions and methods
		assert.Len(t, extractedChunks, 8, "should extract all functions and methods")

		// Verify that extraction uses query engine patterns
		for _, chunk := range extractedChunks {
			assert.NotEmpty(t, chunk.Name, "function name should be extracted via field access")
			assert.GreaterOrEqual(
				t,
				chunk.StartByte,
				uint32(0),
				"StartByte should be non-negative (positions from real AST nodes)",
			)
			assert.Greater(
				t,
				chunk.EndByte,
				chunk.StartByte,
				"EndByte should be greater than StartByte (positions from real AST nodes)",
			)

			// Verify that complex signatures are parsed correctly via field access
			if chunk.Name == "GenericFunction" {
				assert.Len(t, chunk.GenericParameters, 1, "generic parameters should be extracted via field access")
				assert.Equal(t, "T", chunk.GenericParameters[0].Name, "generic parameter name via field access")
			}

			if chunk.Name == "VariadicFunction" {
				assert.Len(t, chunk.Parameters, 2, "variadic function parameters via field access")
				assert.True(
					t,
					containsVariadicParam(chunk.Parameters),
					"should detect variadic parameter via field access",
				)
			}

			if chunk.Name == "MultipleReturnsFunction" {
				assert.Equal(
					t,
					"(result string, count int, err error)",
					chunk.ReturnType,
					"multiple returns via field access",
				)
			}
		}

		// THIS TEST WILL FAIL because:
		// 1. Function parsing doesn't use TreeSitterQueryEngine internally
		// 2. Field access methods for functions don't exist
		// 3. Integration patterns are inconsistent with other parsers
	})

	t.Run("Field Access API Consistency", func(t *testing.T) {
		// EXPECTED FAILURE: This test will fail because the required field access API doesn't exist

		parseTree := createMockParseTree(sourceCode)

		// Find a function declaration to test field access
		functionNodes := parseTree.GetNodesByType("function_declaration")
		require.NotEmpty(t, functionNodes, "should find function declarations")

		functionNode := functionNodes[0] // BasicFunction

		// REQUIRED API: function_declaration should have field access like other constructs

		// Test name field access
		nameField := getChildByFieldName(functionNode, "name")
		require.NotNil(t, nameField, "function_declaration.name field access required")
		assert.Equal(t, "identifier", nameField.Type, "function name should be identifier via field")

		// Test parameters field access
		parametersField := getChildByFieldName(functionNode, "parameters")
		require.NotNil(t, parametersField, "function_declaration.parameters field access required")
		assert.Equal(t, "parameter_list", parametersField.Type, "parameters should be parameter_list via field")

		// Test result field access (if present)
		resultField := getChildByFieldName(functionNode, "result")
		// Result field may be nil for functions without explicit return type
		if resultField != nil {
			// Should handle both single types and parameter_list for multiple returns
			assert.True(t,
				resultField.Type == "type_identifier" ||
					resultField.Type == "parameter_list" ||
					resultField.Type == "pointer_type",
				"result field should be accessible via grammar field")
		}

		// Test generic functions
		genericFunctionNode := findFunctionByName(functionNodes, "GenericFunction")
		if genericFunctionNode != nil {
			typeParametersField := getChildByFieldName(genericFunctionNode, "type_parameters")
			require.NotNil(t, typeParametersField, "generic function.type_parameters field access required")
			assert.Equal(t, "type_parameter_list", typeParametersField.Type, "type parameters via field access")
		}

		// THIS TEST WILL FAIL because:
		// 1. getChildByFieldName() method doesn't exist
		// 2. Field access API is not implemented
		// 3. Function parsing still uses manual traversal instead of field access
	})

	t.Run("Method Field Access Integration", func(t *testing.T) {
		// EXPECTED FAILURE: Method parsing should also use field access consistently

		parseTree := createMockParseTree(sourceCode)

		// Find method declarations
		methodNodes := parseTree.GetNodesByType("method_declaration")
		require.NotEmpty(t, methodNodes, "should find method declarations")

		methodNode := methodNodes[0] // SimpleMethod

		// REQUIRED API: method_declaration should have receiver field access
		receiverField := getChildByFieldName(methodNode, "receiver")
		require.NotNil(t, receiverField, "method_declaration.receiver field access required")
		assert.Equal(t, "parameter_list", receiverField.Type, "receiver should be parameter_list via field")

		// Test receiver parameter extraction via field access
		receiverParams := findDirectChildren(receiverField, "parameter_declaration")
		assert.Len(t, receiverParams, 1, "receiver should have one parameter via field access")

		receiverParam := receiverParams[0]
		receiverTypeField := getChildByFieldName(receiverParam, "type")
		require.NotNil(t, receiverTypeField, "receiver parameter.type field access required")

		// Handle pointer receivers via field access
		var baseReceiverType *valueobject.ParseNode
		if receiverTypeField.Type == "pointer_type" {
			baseTypeField := getChildByFieldName(receiverTypeField, "type")
			require.NotNil(t, baseTypeField, "pointer_type.type field access required")
			baseReceiverType = baseTypeField
		} else {
			baseReceiverType = receiverTypeField
		}

		// Should extract receiver type name via field access, not string parsing
		assert.Equal(t, "type_identifier", baseReceiverType.Type, "receiver type via field access")

		// Test method name, parameters, and result via same field access as functions
		nameField := getChildByFieldName(methodNode, "name")
		require.NotNil(t, nameField, "method_declaration.name field access required")

		parametersField := getChildByFieldName(methodNode, "parameters")
		require.NotNil(t, parametersField, "method_declaration.parameters field access required")

		// THIS TEST WILL FAIL because:
		// 1. Field access API doesn't exist for methods either
		// 2. Current method parsing uses extractReceiverTypeName() string manipulation
		// 3. Position-based logic instead of field access
	})

	t.Run("Consistency with Other Parsers", func(t *testing.T) {
		// EXPECTED FAILURE: Function parsing should follow same patterns as structures.go, imports.go

		parseTree := createMockParseTree(sourceCode)
		queryEngine := NewTreeSitterQueryEngine()

		// Test that function parsing follows same error handling as other parsers
		parser := &GoParser{}

		// Should handle nil inputs gracefully like other parsers
		_, err := parser.ExtractFunctions(context.Background(), nil, outbound.SemanticExtractionOptions{})
		assert.Error(t, err, "should handle nil parse tree like other parsers")

		// Should use query engine for node discovery like other parsers
		functions := queryEngine.QueryFunctionDeclarations(parseTree)
		methods := queryEngine.QueryMethodDeclarations(parseTree)

		// Should process nodes using same patterns as structures
		extractedFunctions, err := parser.ExtractFunctions(
			context.Background(),
			parseTree,
			outbound.SemanticExtractionOptions{},
		)
		require.NoError(t, err, "extraction should follow same patterns as other parsers")

		// Verify that all functions and methods are found via consistent patterns
		assert.Len(t, extractedFunctions, len(functions)+len(methods),
			"extraction should be consistent with query engine results")

		// Verify field access patterns are consistent
		for _, fn := range extractedFunctions {
			// Should have proper position extraction like other parsers
			assert.Less(t, fn.StartByte, fn.EndByte, "positions should be valid like other parsers")

			// Should have proper visibility detection like other parsers
			assert.NotEmpty(t, fn.Visibility,
				"visibility should be determined like other parsers")

			// Should extract content using same methods as other parsers
			assert.NotEmpty(t, fn.Content, "content extraction should follow same patterns")

			// Should generate consistent hash values like other parsers
			assert.NotEmpty(t, fn.Hash, "hash generation should follow same patterns")
		}

		// THIS TEST WILL FAIL because:
		// 1. Function parsing doesn't integrate with TreeSitterQueryEngine like other parsers
		// 2. Error handling patterns are inconsistent
		// 3. Field access approach is different from structures.go, imports.go, etc.
	})
}

// Helper functions that demonstrate the required API (EXPECTED TO FAIL)

// createMockParseTree creates a mock parse tree for testing
// EXPECTED TO FAIL: This helper doesn't exist and would need real tree-sitter parsing.
func createMockParseTree(sourceCode string) *valueobject.ParseTree {
	// This would use actual tree-sitter parsing in real implementation
	panic("createMockParseTree not implemented - expected test failure")
}

// Note: getChildByFieldName and findDirectChildren are declared in functions_field_access_test.go

// findFunctionByName finds a function node by its name
// EXPECTED TO FAIL: This utility doesn't exist.
func findFunctionByName(nodes []*valueobject.ParseNode, name string) *valueobject.ParseNode {
	// This would search through function nodes to find one with specific name
	// Implementation would use field access to get function names
	panic("findFunctionByName not implemented - expected test failure")
}

// containsVariadicParam checks if parameter list contains variadic parameter
// EXPECTED TO FAIL: This utility doesn't exist.
func containsVariadicParam(params []outbound.Parameter) bool {
	// This would check for parameters with "..." prefix in type
	// Implementation should use field access to detect variadic_parameter_declaration nodes
	panic("containsVariadicParam not implemented - expected test failure")
}
