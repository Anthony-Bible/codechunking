// Package utils provides tree-sitter parsing utilities for semantic code traversal.
package utils

import (
	"codechunking/internal/domain/valueobject"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindChildByType tests the findChildByType utility function.
// This is a RED PHASE test that defines expected behavior for finding first child node of specific type.
func TestFindChildByType(t *testing.T) {
	tests := []struct {
		name             string
		parentNode       *valueobject.ParseNode
		targetType       string
		expectedNodeType string
		expectedResult   bool
	}{
		{
			name: "finds identifier in function declaration",
			parentNode: &valueobject.ParseNode{
				Type: "function_declaration",
				Children: []*valueobject.ParseNode{
					{Type: "func", StartByte: 0, EndByte: 4},
					{Type: "identifier", StartByte: 5, EndByte: 12},
					{Type: "parameter_list", StartByte: 12, EndByte: 14},
				},
			},
			targetType:       "identifier",
			expectedNodeType: "identifier",
			expectedResult:   true,
		},
		{
			name: "finds type_identifier in struct field",
			parentNode: &valueobject.ParseNode{
				Type: "field_declaration",
				Children: []*valueobject.ParseNode{
					{Type: "field_identifier", StartByte: 0, EndByte: 4},
					{Type: "type_identifier", StartByte: 5, EndByte: 8},
				},
			},
			targetType:       "type_identifier",
			expectedNodeType: "type_identifier",
			expectedResult:   true,
		},
		{
			name: "returns nil for non-existent type",
			parentNode: &valueobject.ParseNode{
				Type: "function_declaration",
				Children: []*valueobject.ParseNode{
					{Type: "func", StartByte: 0, EndByte: 4},
					{Type: "identifier", StartByte: 5, EndByte: 12},
				},
			},
			targetType:     "parameter_list",
			expectedResult: false,
		},
		{
			name: "handles empty children",
			parentNode: &valueobject.ParseNode{
				Type:     "block",
				Children: []*valueobject.ParseNode{},
			},
			targetType:     "statement",
			expectedResult: false,
		},
		{
			name:           "handles nil parent node",
			parentNode:     nil,
			targetType:     "identifier",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindChildByType(tt.parentNode, tt.targetType)

			if tt.expectedResult {
				require.NotNil(t, result, "Expected to find child node of type %s", tt.targetType)
				assert.Equal(t, tt.expectedNodeType, result.Type, "Found node should have expected type")
				assert.GreaterOrEqual(t, result.EndByte, result.StartByte, "Node should have valid byte range")
			} else {
				assert.Nil(t, result, "Expected not to find child node of type %s", tt.targetType)
			}
		})
	}
}

// TestFindChildrenByType tests the findChildrenByType utility function.
// This is a RED PHASE test that defines expected behavior for finding all children of specific type.
func TestFindChildrenByType(t *testing.T) {
	tests := []struct {
		name          string
		parentNode    *valueobject.ParseNode
		targetType    string
		expectedCount int
		expectedTypes []string
	}{
		{
			name: "finds all identifiers in parameter list",
			parentNode: &valueobject.ParseNode{
				Type: "parameter_list",
				Children: []*valueobject.ParseNode{
					{Type: "parameter_declaration", StartByte: 1, EndByte: 5},
					{Type: "parameter_declaration", StartByte: 7, EndByte: 12},
					{Type: "parameter_declaration", StartByte: 14, EndByte: 20},
				},
			},
			targetType:    "parameter_declaration",
			expectedCount: 3,
			expectedTypes: []string{"parameter_declaration", "parameter_declaration", "parameter_declaration"},
		},
		{
			name: "finds all import_spec in import declaration",
			parentNode: &valueobject.ParseNode{
				Type: "import_declaration",
				Children: []*valueobject.ParseNode{
					{Type: "import", StartByte: 0, EndByte: 6},
					{Type: "import_spec", StartByte: 8, EndByte: 13},
					{Type: "comment", StartByte: 15, EndByte: 25},
					{Type: "import_spec", StartByte: 26, EndByte: 35},
				},
			},
			targetType:    "import_spec",
			expectedCount: 2,
			expectedTypes: []string{"import_spec", "import_spec"},
		},
		{
			name: "returns empty slice for non-existent type",
			parentNode: &valueobject.ParseNode{
				Type: "function_declaration",
				Children: []*valueobject.ParseNode{
					{Type: "func", StartByte: 0, EndByte: 4},
					{Type: "identifier", StartByte: 5, EndByte: 12},
				},
			},
			targetType:    "parameter_declaration",
			expectedCount: 0,
		},
		{
			name: "handles empty children",
			parentNode: &valueobject.ParseNode{
				Type:     "block",
				Children: []*valueobject.ParseNode{},
			},
			targetType:    "statement",
			expectedCount: 0,
		},
		{
			name:          "handles nil parent node",
			parentNode:    nil,
			targetType:    "identifier",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindChildrenByType(tt.parentNode, tt.targetType)

			assert.Len(t, result, tt.expectedCount, "Expected %d children of type %s", tt.expectedCount, tt.targetType)

			for i, expectedType := range tt.expectedTypes {
				require.Less(t, i, len(result), "Result should have at least %d elements", i+1)
				assert.Equal(t, expectedType, result[i].Type, "Child %d should have expected type", i)
				assert.GreaterOrEqual(
					t,
					result[i].EndByte,
					result[i].StartByte,
					"Child %d should have valid byte range",
					i,
				)
			}
		})
	}
}

// TestExtractPackageNameFromTree tests package name extraction from parse tree.
// This is a RED PHASE test that defines expected behavior for extracting package names.
func TestExtractPackageNameFromTree(t *testing.T) {
	tests := []struct {
		name            string
		parseTree       *mockParseTree
		expectedPackage string
	}{
		{
			name:            "extracts package name from main package",
			parseTree:       createMockParseTreeWithPackage("main"),
			expectedPackage: "main",
		},
		{
			name:            "extracts package name from custom package",
			parseTree:       createMockParseTreeWithPackage("utils"),
			expectedPackage: "utils",
		},
		{
			name: "returns default for missing package clause",
			parseTree: &mockParseTree{
				nodes:     []*valueobject.ParseNode{},
				nodeTexts: map[*valueobject.ParseNode]string{},
			},
			expectedPackage: "main",
		},
		{
			name: "returns default for malformed package clause",
			parseTree: &mockParseTree{
				nodes: []*valueobject.ParseNode{
					{
						Type: "package_clause",
						Children: []*valueobject.ParseNode{
							{Type: "package", StartByte: 0, EndByte: 7},
							// Missing package_identifier
						},
					},
				},
				nodeTexts: map[*valueobject.ParseNode]string{},
			},
			expectedPackage: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPackageNameFromTree(tt.parseTree)
			assert.Equal(t, tt.expectedPackage, result, "Package name should match expected")
		})
	}
}

// TestIsNodeType tests node type checking utility functions.
// This is a RED PHASE test that defines expected behavior for node type validation.
func TestIsNodeType(t *testing.T) {
	tests := []struct {
		name           string
		node           *valueobject.ParseNode
		nodeType       string
		expectedResult bool
	}{
		{
			name:           "matches exact type",
			node:           &valueobject.ParseNode{Type: "function_declaration"},
			nodeType:       "function_declaration",
			expectedResult: true,
		},
		{
			name:           "does not match different type",
			node:           &valueobject.ParseNode{Type: "function_declaration"},
			nodeType:       "method_declaration",
			expectedResult: false,
		},
		{
			name:           "handles nil node",
			node:           nil,
			nodeType:       "function_declaration",
			expectedResult: false,
		},
		{
			name:           "handles empty type",
			node:           &valueobject.ParseNode{Type: ""},
			nodeType:       "",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNodeType(tt.node, tt.nodeType)
			assert.Equal(t, tt.expectedResult, result, "Node type check should match expected result")
		})
	}
}

// TestGetNodesByType tests finding nodes by type in parse tree.
// This is a RED PHASE test that defines expected behavior for finding nodes by type.
func TestGetNodesByType(t *testing.T) {
	tests := []struct {
		name          string
		parseTree     *mockParseTree
		nodeType      string
		expectedCount int
		expectedTypes []string
	}{
		{
			name: "finds all function declarations",
			parseTree: &mockParseTree{
				nodesByType: map[string][]*valueobject.ParseNode{
					"function_declaration": {
						{Type: "function_declaration", StartByte: 0, EndByte: 50},
						{Type: "function_declaration", StartByte: 52, EndByte: 100},
					},
				},
			},
			nodeType:      "function_declaration",
			expectedCount: 2,
			expectedTypes: []string{"function_declaration", "function_declaration"},
		},
		{
			name: "finds all import declarations",
			parseTree: &mockParseTree{
				nodesByType: map[string][]*valueobject.ParseNode{
					"import_declaration": {
						{Type: "import_declaration", StartByte: 10, EndByte: 30},
					},
				},
			},
			nodeType:      "import_declaration",
			expectedCount: 1,
			expectedTypes: []string{"import_declaration"},
		},
		{
			name: "returns empty slice for non-existent type",
			parseTree: &mockParseTree{
				nodesByType: map[string][]*valueobject.ParseNode{},
			},
			nodeType:      "non_existent_type",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNodesByType(tt.parseTree, tt.nodeType)

			assert.Len(t, result, tt.expectedCount, "Expected %d nodes of type %s", tt.expectedCount, tt.nodeType)

			for i, expectedType := range tt.expectedTypes {
				require.Less(t, i, len(result), "Result should have at least %d elements", i+1)
				assert.Equal(t, expectedType, result[i].Type, "Node %d should have expected type", i)
				assert.GreaterOrEqual(
					t,
					result[i].EndByte,
					result[i].StartByte,
					"Node %d should have valid byte range",
					i,
				)
			}
		})
	}
}

// Mock types for testing.
type mockParseTree struct {
	nodes       []*valueobject.ParseNode
	nodeTexts   map[*valueobject.ParseNode]string
	nodesByType map[string][]*valueobject.ParseNode
}

func (m *mockParseTree) GetNodesByType(nodeType string) []*valueobject.ParseNode {
	if m.nodesByType == nil {
		return []*valueobject.ParseNode{}
	}
	return m.nodesByType[nodeType]
}

func (m *mockParseTree) GetNodeText(node *valueobject.ParseNode) string {
	if m.nodeTexts == nil {
		return ""
	}
	return m.nodeTexts[node]
}

// ParseTreeProvider interface is defined in ast_utils.go

// Test that the mock implements the expected interface.
var _ ParseTreeProvider = (*mockParseTree)(nil)

// Helper function to create mock parse tree with proper node references.
func createMockParseTreeWithPackage(packageName string) *mockParseTree {
	// Create the package identifier node
	packageIdentifier := &valueobject.ParseNode{
		Type:      "package_identifier",
		StartByte: 8,
		EndByte:   uint32(8 + len(packageName)),
	}

	// Create the package clause node
	packageClause := &valueobject.ParseNode{
		Type: "package_clause",
		Children: []*valueobject.ParseNode{
			{Type: "package", StartByte: 0, EndByte: 7},
			packageIdentifier,
		},
	}

	return &mockParseTree{
		nodesByType: map[string][]*valueobject.ParseNode{
			"package_clause": {packageClause},
		},
		nodeTexts: map[*valueobject.ParseNode]string{
			packageIdentifier: packageName,
		},
	}
}

// RED PHASE: These functions don't exist yet and will cause compilation errors.
// This is intentional - the tests define the expected behavior for implementation.

// Expected function signatures that need to be implemented:
// func FindChildByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode
// func FindChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode
// func ExtractPackageNameFromTree(parseTree ParseTreeProvider) string
// func IsNodeType(node *valueobject.ParseNode, nodeType string) bool
// func GetNodesByType(parseTree ParseTreeProvider, nodeType string) []*valueobject.ParseNode
