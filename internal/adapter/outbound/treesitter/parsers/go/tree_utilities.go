package goparser

import (
	"codechunking/internal/domain/valueobject"
)

// TreeTraversalUtilities provides helper functions for traversing tree-sitter parse trees.
// These utilities complement the TreeSitterQueryEngine by providing more granular
// tree traversal operations for specific use cases.
//
// The functions in this package are designed to be:
//   - Safe: Handle nil inputs gracefully without panicking
//   - Efficient: Minimize unnecessary allocations and traversals
//   - Consistent: Follow the same patterns and naming conventions
//   - Well-documented: Clear purpose and behavior documentation
type TreeTraversalUtilities struct{}

// FindChildrenRecursive recursively searches through the entire subtree starting from the given node
// to find all nodes of the specified type. This function performs a depth-first traversal
// and includes the starting node in the search.
//
// Use this function when you need to find all occurrences of a node type within
// a subtree, regardless of their depth or nesting level.
//
// Parameters:
//
//	node - The root node to start searching from (can be nil)
//	nodeType - The AST node type to search for (e.g., "function_declaration")
//
// Returns:
//
//	[]*valueobject.ParseNode - All matching nodes found in the subtree, empty slice if none found
//
// Example:
//
//	// Find all function declarations in the entire parse tree
//	functions := FindChildrenRecursive(parseTree.RootNode(), "function_declaration")
//
//	// Find all identifiers within a specific function
//	identifiers := FindChildrenRecursive(functionNode, "identifier")
func FindChildrenRecursive(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var results []*valueobject.ParseNode

	// Handle nil input gracefully
	if node == nil || nodeType == "" {
		return results
	}

	// Check if the current node matches the target type
	if node.Type == nodeType {
		results = append(results, node)
	}

	// Recursively search all children
	for _, child := range node.Children {
		childResults := FindChildrenRecursive(child, nodeType)
		results = append(results, childResults...)
	}

	return results
}

// FindDirectChildren searches only the immediate children of the given parent node
// to find nodes of the specified type. This function does NOT recurse into
// grandchildren or deeper levels.
//
// Use this function when you need to find direct children only, such as finding
// field declarations directly within a struct, or method specifications directly
// within an interface.
//
// Parameters:
//
//	parent - The parent node to search within (can be nil)
//	nodeType - The AST node type to search for (e.g., "field_declaration")
//
// Returns:
//
//	[]*valueobject.ParseNode - Direct child nodes matching the type, empty slice if none found
//
// Example:
//
//	// Find field declarations directly within a struct (not nested structs)
//	fields := FindDirectChildren(structNode, "field_declaration")
//
//	// Find package identifier directly within a package clause
//	packageIds := FindDirectChildren(packageClause, "package_identifier")
func FindDirectChildren(parent *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var result []*valueobject.ParseNode

	// Handle nil input gracefully
	if parent == nil || nodeType == "" {
		return result
	}

	// Search only direct children (no recursion)
	for _, child := range parent.Children {
		if child.Type == nodeType {
			result = append(result, child)
		}
	}

	return result
}

// ValidateNodePosition validates that a parse node has valid position information.
// This is useful for ensuring that position extraction is working correctly
// and that nodes have been properly parsed by tree-sitter.
//
// Parameters:
//
//	node - The node to validate (can be nil)
//
// Returns:
//
//	bool - True if the node has valid position information, false otherwise
//
// A node is considered to have valid position information if:
//   - The node is not nil
//   - StartByte is less than or equal to EndByte
//   - StartPos.Row is less than or equal to EndPos.Row
//   - If rows are equal, StartPos.Column is less than or equal to EndPos.Column
func ValidateNodePosition(node *valueobject.ParseNode) bool {
	if node == nil {
		return false
	}

	// Validate byte positions
	if node.StartByte > node.EndByte {
		return false
	}

	// Validate line/column positions
	if node.StartPos.Row > node.EndPos.Row {
		return false
	}

	// If on the same row, start column should be <= end column
	if node.StartPos.Row == node.EndPos.Row && node.StartPos.Column > node.EndPos.Column {
		return false
	}

	return true
}

// ExtractPositionInfo extracts standardized position information from a parse node.
// This utility ensures consistent position handling across all extraction methods.
//
// Parameters:
//
//	node - The node to extract position from (must not be nil)
//
// Returns:
//
//	startByte, endByte - The byte positions in the source code
//	valid - True if the position information is valid
//
// Example:
//
//	startByte, endByte, valid := ExtractPositionInfo(structNode)
//	if !valid {
//	    return nil, fmt.Errorf("invalid position information for struct node")
//	}
func ExtractPositionInfo(node *valueobject.ParseNode) (uint32, uint32, bool) {
	if !ValidateNodePosition(node) {
		return 0, 0, false
	}

	return node.StartByte, node.EndByte, true
}

// ============================================================================
// Shared TreeSitter Utilities Reference
// ============================================================================

// Note: TreeSitter parser factory utilities (CreateTreeSitterParseTree, ValidateSourceWithTreeSitter)
// have been moved to /internal/adapter/outbound/treesitter/tree_sitter_utils.go
// to be shared across packages and avoid import cycles.
//
// Use treesitter.CreateTreeSitterParseTree() and treesitter.ValidateSourceWithTreeSitter()
// instead of the local versions.
