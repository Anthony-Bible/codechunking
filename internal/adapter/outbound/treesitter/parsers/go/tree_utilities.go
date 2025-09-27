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
// Performance note: For large trees, consider using FindDirectChildren when you only
// need immediate children, as it's more efficient.
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
		if child != nil { // Additional nil check for safety
			childResults := FindChildrenRecursive(child, nodeType)
			results = append(results, childResults...)
		}
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
// IMPORTANT: This function considers StartByte=0 as VALID since tree-sitter
// correctly reports byte position 0 for constructs at the beginning of files
// (such as package declarations). Zero is not an error condition.
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
//   - StartByte is less than or equal to EndByte (StartByte=0 is valid)
//   - StartPos.Row is less than or equal to EndPos.Row
//   - If rows are equal, StartPos.Column is less than or equal to EndPos.Column
func ValidateNodePosition(node *valueobject.ParseNode) bool {
	if node == nil {
		return false
	}

	// Validate byte positions - StartByte can be 0 (valid for file start)
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
// It validates the node and its position data before returning the byte positions.
//
// IMPORTANT: StartByte=0 is VALID for nodes that appear at the very beginning of a file.
// Tree-sitter correctly reports StartByte=0 for package declarations and other constructs
// that start at file position 0. This is not an error condition and should be preserved.
//
// Parameters:
//
//	node - The node to extract position from (can be nil)
//
// Returns:
//
//	startByte, endByte - The byte positions in the source code (0, 0 if invalid)
//	valid - True if the position information is valid
//
// Example:
//
//	startByte, endByte, valid := ExtractPositionInfo(packageNode)
//	if !valid {
//	    return nil, fmt.Errorf("invalid position information")
//	}
//	// Note: startByte may be 0 for nodes at file start - this is correct!
func ExtractPositionInfo(node *valueobject.ParseNode) (uint32, uint32, bool) {
	if !ValidateNodePosition(node) {
		return 0, 0, false
	}

	return node.StartByte, node.EndByte, true
}

// ExtractPositionInfoWithFallback extracts position information with a fallback strategy.
// If the primary node's position is invalid, it attempts to use a fallback node's position.
// This is useful when dealing with complex AST structures where position information
// might be inconsistent across different node types.
//
// Parameters:
//
//	primaryNode - The primary node to extract position from
//	fallbackNode - The fallback node to use if primary fails (can be nil)
//
// Returns:
//
//	startByte, endByte - The byte positions in the source code
//	valid - True if either primary or fallback position is valid
//	source - Indicates which node was used ("primary", "fallback", or "none")
func ExtractPositionInfoWithFallback(
	primaryNode *valueobject.ParseNode,
	fallbackNode *valueobject.ParseNode,
) (uint32, uint32, bool, string) {
	// Try primary node first
	if startByte, endByte, valid := ExtractPositionInfo(primaryNode); valid {
		return startByte, endByte, true, "primary"
	}

	// Fall back to fallback node
	if startByte, endByte, valid := ExtractPositionInfo(fallbackNode); valid {
		return startByte, endByte, true, "fallback"
	}

	// Neither node has valid position information
	return 0, 0, false, "none"
}

// ExtractPackagePositionInfo is a specialized position extractor for package declarations
// that includes enhanced metadata tracking for package nodes at file start.
//
// This function handles the common case where package declarations appear at the very
// beginning of Go source files (StartByte=0), which is normal tree-sitter behavior.
//
// Parameters:
//
//	packageNode - The package_clause node to extract position from
//	packageName - The name of the package (for error reporting and metadata)
//
// Returns:
//
//	startByte, endByte - The byte positions in the source code
//	metadata - Optional metadata map for debugging/tracking (may be nil)
//	valid - True if the position information is valid
//
// Example:
//
//	startByte, endByte, metadata, valid := ExtractPackagePositionInfo(packageClause, "main")
//	if !valid {
//	    return nil, fmt.Errorf("invalid position information for package: %s", packageName)
//	}
//	chunk.StartByte = startByte  // May be 0 for packages at file start
//	if metadata != nil {
//	    chunk.Metadata = metadata
//	}
func ExtractPackagePositionInfo(
	packageNode *valueobject.ParseNode,
	packageName string,
) (uint32, uint32, map[string]interface{}, bool) {
	// Extract basic position information
	startByte, endByte, valid := ExtractPositionInfo(packageNode)
	if !valid {
		return 0, 0, nil, false
	}

	// Create metadata for packages at file start to aid debugging and documentation
	var metadata map[string]interface{}
	if startByte == 0 {
		metadata = map[string]interface{}{
			"ast_start_byte": startByte,
			"ast_end_byte":   endByte,
			"position_note":  "package at file start",
			"package_name":   packageName,
		}
	}

	return startByte, endByte, metadata, true
}

// ============================================================================
// Grammar Field Access Utilities
// ============================================================================

// GetFieldByName retrieves a child node using the tree-sitter grammar field name.
// This is the preferred method for accessing nodes according to the grammar specification.
//
// Parameters:
//
//	node - The parent node to search in (can be nil)
//	fieldName - The grammar field name (e.g., "name", "type", "value")
//
// Returns:
//
//	*valueobject.ParseNode - The field node if found, nil otherwise
func GetFieldByName(node *valueobject.ParseNode, fieldName string) *valueobject.ParseNode {
	if node == nil || fieldName == "" {
		return nil
	}

	return node.ChildByFieldName(fieldName)
}

// GetMultipleFieldsByName retrieves multiple child nodes for a field that can have multiple values.
// This is useful for fields like "name" in variable declarations which can contain multiple identifiers.
//
// This function handles the grammar-specific differences between var_spec and const_spec:
// - var_spec: has multiple "name" fields (one per identifier)
// - const_spec: has one "name" field containing a sequence of identifiers
//
// Parameters:
//
//	node - The parent node to search in (can be nil)
//	fieldName - The grammar field name (e.g., "name")
//
// Returns:
//
//	[]*valueobject.ParseNode - All field nodes if found, empty slice otherwise
func GetMultipleFieldsByName(node *valueobject.ParseNode, fieldName string) []*valueobject.ParseNode {
	if node == nil || fieldName == "" {
		return []*valueobject.ParseNode{}
	}

	var results []*valueobject.ParseNode

	// Handle var_spec nodes - they have multiple "name" fields
	if node.Type == "var_spec" && fieldName == "name" {
		return getVarSpecNames(node)
	}

	// Handle const_spec nodes - they have one "name" field with sequence of identifiers
	if node.Type == "const_spec" && fieldName == "name" {
		return getConstSpecNames(node)
	}

	// General field access for other node types
	fieldNode := node.ChildByFieldName(fieldName)
	if fieldNode != nil {
		results = append(results, fieldNode)
	}

	return results
}

// getVarSpecNames extracts variable names from var_spec nodes.
// var_spec has multiple "name" fields - one per identifier.
func getVarSpecNames(node *valueobject.ParseNode) []*valueobject.ParseNode {
	if node == nil {
		return []*valueobject.ParseNode{}
	}

	// For var_spec, we find all direct child identifiers
	// since each identifier is a separate "name" field
	return FindDirectChildren(node, nodeTypeIdentifier)
}

// getConstSpecNames extracts constant names from const_spec nodes.
// const_spec has one "name" field with a sequence of identifiers.
func getConstSpecNames(node *valueobject.ParseNode) []*valueobject.ParseNode {
	if node == nil {
		return []*valueobject.ParseNode{}
	}

	// For const_spec, get the "name" field first, then traverse its children
	nameField := node.ChildByFieldName("name")
	if nameField != nil {
		// The name field contains the sequence of identifiers
		return FindDirectChildren(nameField, nodeTypeIdentifier)
	}

	// Fallback: direct children that are identifiers
	return FindDirectChildren(node, nodeTypeIdentifier)
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
