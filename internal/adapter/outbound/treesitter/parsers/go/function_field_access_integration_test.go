package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
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
		genericFunctionNode := findFunctionByNameWithTree(functionNodes, "GenericFunction", parseTree)
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

		// Handle pointer receivers
		// Note: pointer_type has NO fields in the grammar - only unnamed children
		// Children are: [0] = "*", [1] = type_identifier
		var baseReceiverType *valueobject.ParseNode
		if receiverTypeField.Type == "pointer_type" {
			// Find the type_identifier child (skip the "*")
			typeChildren := findDirectChildren(receiverTypeField, "type_identifier")
			require.NotEmpty(t, typeChildren, "pointer_type should have type_identifier child")
			baseReceiverType = typeChildren[0]
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

// Helper functions for TestFunctionFieldAccessIntegration

// createMockParseTree creates a parse tree using actual tree-sitter parsing.
func createMockParseTree(sourceCode string) *valueobject.ParseTree {
	// Get Go grammar from forest
	grammar := forest.GetLanguage("go")
	if grammar == nil {
		panic("Failed to get Go grammar from forest")
	}

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	if parser == nil {
		panic("Failed to create tree-sitter parser")
	}

	success := parser.SetLanguage(grammar)
	if !success {
		panic("Failed to set Go language")
	}

	// Parse the source code
	tree, err := parser.ParseString(context.Background(), nil, []byte(sourceCode))
	if err != nil {
		panic("Failed to parse Go source: " + err.Error())
	}
	if tree == nil {
		panic("Parse tree should not be nil")
	}
	// DON'T close the tree here - it's needed for field access via tsNode references
	// The ParseTree will manage the tree lifecycle

	// Convert tree-sitter tree to domain ParseNode
	rootTSNode := tree.RootNode()
	rootNode, nodeCount, maxDepth := convertTreeSitterNodeForIntegrationTest(rootTSNode, 0)

	// Create metadata with parsing statistics
	metadata, err := valueobject.NewParseMetadata(
		time.Millisecond, // placeholder duration
		"go-tree-sitter-bare",
		"1.0.0",
	)
	if err != nil {
		panic("Failed to create metadata: " + err.Error())
	}

	// Update metadata with actual counts
	metadata.NodeCount = nodeCount
	metadata.MaxDepth = maxDepth

	// Create ParseTree
	parseTree, err := valueobject.NewParseTree(
		context.Background(),
		valueobject.Go,
		rootNode,
		[]byte(sourceCode),
		metadata,
	)
	if err != nil {
		// Clean up tree if ParseTree creation fails
		tree.Close()
		panic("Failed to create ParseTree: " + err.Error())
	}

	// Set tree-sitter tree reference for field access and proper cleanup
	parseTree.SetTreeSitterTree(tree)

	return parseTree
}

// convertTreeSitterNodeForIntegrationTest converts a tree-sitter node to domain ParseNode.
func convertTreeSitterNodeForIntegrationTest(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Convert tree-sitter node to domain ParseNode with tsNode reference for field access
	parseNode, err := valueobject.NewParseNodeWithTreeSitter(
		node.Type(),
		safeUintToUint32ForIntegrationTest(node.StartByte()),
		safeUintToUint32ForIntegrationTest(node.EndByte()),
		valueobject.Position{
			Row:    safeUintToUint32ForIntegrationTest(node.StartPoint().Row),
			Column: safeUintToUint32ForIntegrationTest(node.StartPoint().Column),
		},
		valueobject.Position{
			Row:    safeUintToUint32ForIntegrationTest(node.EndPoint().Row),
			Column: safeUintToUint32ForIntegrationTest(node.EndPoint().Column),
		},
		make([]*valueobject.ParseNode, 0),
		node, // Store tree-sitter node reference for field access
	)
	if err != nil {
		return nil, 0, depth
	}

	nodeCount := 1
	maxDepth := depth

	// Convert children recursively
	childCount := node.ChildCount()
	for i := range childCount {
		childNode := node.Child(i)
		if childNode.IsNull() {
			continue
		}

		childParseNode, childNodeCount, childMaxDepth := convertTreeSitterNodeForIntegrationTest(childNode, depth+1)
		if childParseNode != nil {
			parseNode.Children = append(parseNode.Children, childParseNode)
			nodeCount += childNodeCount
			if childMaxDepth > maxDepth {
				maxDepth = childMaxDepth
			}
		}
	}

	return parseNode, nodeCount, maxDepth
}

// safeUintToUint32ForIntegrationTest safely converts uint to uint32 with bounds checking.
func safeUintToUint32ForIntegrationTest(val uint) uint32 {
	if val > uint(^uint32(0)) {
		return ^uint32(0) // Return max uint32 if overflow would occur
	}
	return uint32(val)
}

// findFunctionByName finds a function node by its name using field access.
// Note: This is a simple implementation without parseTree access.
func findFunctionByName(nodes []*valueobject.ParseNode, name string) *valueobject.ParseNode {
	for _, node := range nodes {
		if node == nil {
			continue
		}

		// Use field access to get the function name
		nameField := getChildByFieldName(node, "name")
		if nameField == nil {
			continue
		}

		// Get the text content of the name field (would use parseTree.GetNodeText in real code)
		// For now, we'll check children for identifier nodes
		for _, child := range node.Children {
			if child.Type == "identifier" || child.Type == "field_identifier" {
				// In a real implementation, we'd get the text from the parseTree
				// For this test, we assume the name matches the search criteria
				// This is a placeholder - actual implementation needs parseTree.GetNodeText
				return node
			}
		}
	}
	return nil
}

// findFunctionByNameWithTree finds a function node by its name using field access and parseTree.
func findFunctionByNameWithTree(
	nodes []*valueobject.ParseNode,
	targetName string,
	parseTree *valueobject.ParseTree,
) *valueobject.ParseNode {
	for _, node := range nodes {
		if node == nil {
			continue
		}

		// Use field access to get the function name
		nameField := getChildByFieldName(node, "name")
		if nameField == nil {
			continue
		}

		// Get the actual text from parseTree
		actualName := parseTree.GetNodeText(nameField)
		if actualName == targetName {
			return node
		}
	}
	return nil
}

// containsVariadicParam checks if parameter list contains variadic parameter.
func containsVariadicParam(params []outbound.Parameter) bool {
	for _, param := range params {
		if strings.HasPrefix(param.Type, "...") {
			return true
		}
	}
	return false
}
