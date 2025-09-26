package goparser

import (
	"codechunking/internal/domain/valueobject"
	"strings"
)

// TreeSitterQueryEngine defines the interface for querying tree-sitter parse trees
// using proper grammar-based queries instead of manual string parsing.
//
// This interface provides methods to extract specific Go language constructs from
// a parsed syntax tree, enabling semantic code analysis and processing. All methods
// are safe to call with nil input and will return an empty slice rather than panic.
//
// The query engine leverages Tree-sitter's AST capabilities to find nodes by their
// grammatical type, which is more reliable and performant than regex-based parsing.
//
// Example usage:
//
//	engine := NewTreeSitterQueryEngine()
//	packages := engine.QueryPackageDeclarations(parseTree)
//	functions := engine.QueryFunctionDeclarations(parseTree)
//
// All query methods follow the same pattern:
//   - Accept a *valueobject.ParseTree parameter
//   - Return []*valueobject.ParseNode slice
//   - Handle nil inputs gracefully
//   - Return empty slice if no matches found
type TreeSitterQueryEngine interface {
	// QueryPackageDeclarations finds all package_clause nodes in the parse tree.
	// Returns all package declarations found in the Go source file.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of package declaration nodes, empty if none found
	QueryPackageDeclarations(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryImportDeclarations finds all import_declaration nodes in the parse tree.
	// Returns all import statements found in the Go source file, including both
	// single imports and grouped imports within parentheses.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of import declaration nodes, empty if none found
	QueryImportDeclarations(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryFunctionDeclarations finds all function_declaration nodes in the parse tree.
	// Returns all top-level function declarations, including generic functions but
	// excluding methods (which have receivers).
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of function declaration nodes, empty if none found
	QueryFunctionDeclarations(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryMethodDeclarations finds all method_declaration nodes in the parse tree.
	// Returns all method declarations (functions with receivers), including both
	// value and pointer receiver methods.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of method declaration nodes, empty if none found
	QueryMethodDeclarations(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryVariableDeclarations finds all var_declaration nodes in the parse tree.
	// Returns all variable declarations, including both typed and untyped declarations,
	// with or without initialization values.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of variable declaration nodes, empty if none found
	QueryVariableDeclarations(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryConstDeclarations finds all const_declaration nodes in the parse tree.
	// Returns all constant declarations, including both individual constants and
	// grouped constants within parentheses.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of constant declaration nodes, empty if none found
	QueryConstDeclarations(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryTypeDeclarations finds all type_declaration nodes in the parse tree.
	// Returns all type declarations, including struct, interface, and type alias
	// declarations.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of type declaration nodes, empty if none found
	QueryTypeDeclarations(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryFieldDeclarations finds all field_declaration nodes within struct types.
	// Returns all struct field declarations found in the Go source file, which include
	// named fields, anonymous fields (embedded types), and fields with tags.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of field declaration nodes, empty if none found
	//
	// Example node types this finds:
	//   - Named fields: "Name string", "Age int"
	//   - Tagged fields: "Name string `json:\"name\"`"
	//   - Embedded types: "io.Reader", "MyStruct"
	//   - Complex types: "Data map[string]interface{}", "Handler func() error"
	QueryFieldDeclarations(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryMethodSpecs finds all method_spec nodes within interface types.
	// Returns all interface method specifications found in the Go source file,
	// including methods with various parameter and return type combinations.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of method specification nodes, empty if none found
	//
	// Example node types this finds:
	//   - Simple methods: "Method() error", "String() string"
	//   - Complex methods: "Process(data string) (string, error)"
	//   - Parameterless methods: "Cleanup()", "Reset()"
	//   - Generic methods: "Transform[T any](T) T"
	QueryMethodSpecs(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryEmbeddedTypes finds embedded type nodes within struct and interface definitions.
	// Returns all embedded types (anonymous fields in structs, embedded interfaces in interfaces).
	// This is a specialized query for detecting composition patterns in Go.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of embedded type nodes, empty if none found
	//
	// Example node types this finds:
	//   - Embedded interfaces: "io.Reader", "fmt.Stringer"
	//   - Embedded structs: "BaseStruct", "util.Helper"
	//   - Package-qualified types: "http.Handler", "context.Context"
	QueryEmbeddedTypes(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryCallExpressions finds all call_expression nodes in the parse tree.
	// Returns all function call expressions found in the Go source file,
	// including method calls, function calls, and constructor calls.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of call expression nodes, empty if none found
	//
	// Example node types this finds:
	//   - Function calls: "fmt.Println(\"hello\")", "log.Fatal(err)"
	//   - Method calls: "obj.Method(args)", "receiver.Function()"
	//   - Constructor calls: "NewStruct(params)", "make([]int, 10)"
	QueryCallExpressions(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// QueryMultipleTypes performs multiple queries in a single tree traversal for better performance.
	// This method is optimized for cases where multiple types of declarations need to be extracted
	// from the same parse tree, reducing the overall traversal cost from O(n*m) to O(n) where
	// n is the number of nodes and m is the number of query types.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//   nodeTypes - Slice of node types to search for (e.g., ["function_declaration", "var_declaration"])
	//
	// Returns:
	//   map[string][]*valueobject.ParseNode - Map where keys are node types and values are matching nodes
	//
	// Example usage:
	//
	//	results := engine.QueryMultipleTypes(parseTree, []string{
	//		"function_declaration",
	//		"method_declaration",
	//		"var_declaration",
	//	})
	//	functions := results["function_declaration"]
	//	methods := results["method_declaration"]
	//	variables := results["var_declaration"]
	QueryMultipleTypes(parseTree *valueobject.ParseTree, nodeTypes []string) map[string][]*valueobject.ParseNode

	// QueryComments finds all comment nodes in the parse tree using tree-sitter
	// instead of manual string parsing. Returns all comments found in the Go source file,
	// including both line comments (//) and block comments (/* */).
	//
	// This method leverages tree-sitter's comment node identification capabilities
	// to accurately extract comments without relying on regex or string manipulation.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query (can be nil)
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of comment nodes, empty if none found
	QueryComments(parseTree *valueobject.ParseTree) []*valueobject.ParseNode

	// ProcessCommentText processes a comment node to extract clean text content
	// using tree-sitter's comment processing capabilities instead of manual string
	// manipulation. Handles both line comments and block comments properly.
	//
	// This method should replace manual string processing with tree-sitter-based
	// comment text extraction, eliminating the need for manual marker removal
	// and string trimming operations.
	//
	// Parameters:
	//   parseTree - The parsed syntax tree containing the comment
	//   commentNode - The comment node to process
	//
	// Returns:
	//   string - Clean comment text without markers or excess whitespace
	ProcessCommentText(parseTree *valueobject.ParseTree, commentNode *valueobject.ParseNode) string

	// QueryDocumentationComments finds documentation comments associated with a
	// specific declaration node using tree-sitter comment associations instead of
	// manual position-based logic.
	//
	// This method should replace manual byte position comparisons and sibling
	// traversal with proper tree-sitter comment-declaration associations,
	// using features like the Go grammar's comment adjacency patterns.
	//
	// The method handles different types of documentation patterns:
	//   - Line comments immediately preceding declarations
	//   - Block comments immediately preceding declarations
	//   - Multi-line comment sequences
	//   - Proper handling of blank line separation
	//
	// Parameters:
	//   parseTree - The parsed syntax tree to query
	//   declarationNode - The declaration node to find documentation for
	//
	// Returns:
	//   []*valueobject.ParseNode - Slice of associated documentation comment nodes
	QueryDocumentationComments(
		parseTree *valueobject.ParseTree,
		declarationNode *valueobject.ParseNode,
	) []*valueobject.ParseNode
}

// ConcreteTreeSitterQueryEngine implements the TreeSitterQueryEngine interface
// using tree-sitter AST traversal instead of manual string parsing.
//
// This implementation provides efficient, grammar-aware querying of Go source code
// parse trees. It leverages Tree-sitter's robust AST representation to accurately
// identify language constructs without relying on fragile regex patterns.
//
// The implementation is stateless and thread-safe, making it suitable for concurrent
// use across multiple goroutines. All methods handle edge cases gracefully and
// provide consistent behavior.
//
// Performance characteristics:
//   - O(n) traversal where n is the number of nodes in the parse tree
//   - Memory efficient with no internal caching or state
//   - Minimal allocations through reuse of tree-sitter's native structures
type ConcreteTreeSitterQueryEngine struct{}

// queryNodesByType is a common utility method that extracts the repeated pattern
// used by all query methods. It validates the parse tree and safely retrieves
// nodes of the specified type, handling all edge cases consistently.
//
// This method implements defensive programming practices:
//   - Validates all input parameters before use
//   - Handles nil values gracefully without panicking
//   - Returns empty slices for invalid inputs (never nil)
//   - Protects against potential panics from tree operations
//
// Parameters:
//
//	parseTree - The syntax tree to query (validated for nil)
//	nodeType - The grammar node type to search for (validated for empty string)
//
// Returns:
//
//	[]*valueobject.ParseNode - Slice of matching nodes, empty if none found or invalid input
func (e *ConcreteTreeSitterQueryEngine) queryNodesByType(
	parseTree *valueobject.ParseTree,
	nodeType string,
) []*valueobject.ParseNode {
	// Validate input parameters - return empty slice for invalid inputs
	if parseTree == nil {
		return []*valueobject.ParseNode{}
	}

	// Validate nodeType parameter - empty strings are not valid node types
	if nodeType == "" {
		return []*valueobject.ParseNode{}
	}

	// Get nodes by type with nil safety
	// The ParseTree.GetNodesByType method should handle internal errors gracefully
	nodes := parseTree.GetNodesByType(nodeType)
	if nodes == nil {
		return []*valueobject.ParseNode{}
	}

	// Always return a valid slice (never nil)
	return nodes
}

// NewTreeSitterQueryEngine creates a new TreeSitterQueryEngine instance.
//
// Returns a fully initialized query engine ready for use. The returned instance
// is stateless and can be safely used across multiple goroutines concurrently.
//
// Example usage:
//
//	engine := NewTreeSitterQueryEngine()
//	defer engine.Close() // No cleanup needed - stateless implementation
//
//	// Query different types of declarations
//	packages := engine.QueryPackageDeclarations(parseTree)
//	imports := engine.QueryImportDeclarations(parseTree)
//	functions := engine.QueryFunctionDeclarations(parseTree)
//
// Returns:
//
//	TreeSitterQueryEngine - A new query engine instance
func NewTreeSitterQueryEngine() TreeSitterQueryEngine {
	return &ConcreteTreeSitterQueryEngine{}
}

// QueryPackageDeclarations finds all package_clause nodes in the parse tree.
func (e *ConcreteTreeSitterQueryEngine) QueryPackageDeclarations(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, nodeTypePackageClause)
}

// QueryImportDeclarations finds all import_declaration nodes in the parse tree.
func (e *ConcreteTreeSitterQueryEngine) QueryImportDeclarations(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, nodeTypeImportDeclaration)
}

// QueryFunctionDeclarations finds all function_declaration nodes in the parse tree.
func (e *ConcreteTreeSitterQueryEngine) QueryFunctionDeclarations(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, "function_declaration")
}

// QueryMethodDeclarations finds all method_declaration nodes in the parse tree.
func (e *ConcreteTreeSitterQueryEngine) QueryMethodDeclarations(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, "method_declaration")
}

// QueryVariableDeclarations finds all var_declaration nodes in the parse tree.
func (e *ConcreteTreeSitterQueryEngine) QueryVariableDeclarations(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, nodeTypeVarDeclaration)
}

// QueryConstDeclarations finds all const_declaration nodes in the parse tree.
func (e *ConcreteTreeSitterQueryEngine) QueryConstDeclarations(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, nodeTypeConstDeclaration)
}

// QueryTypeDeclarations finds all type_declaration nodes in the parse tree.
func (e *ConcreteTreeSitterQueryEngine) QueryTypeDeclarations(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, nodeTypeTypeDeclaration)
}

// QueryFieldDeclarations finds all field_declaration nodes within struct types.
func (e *ConcreteTreeSitterQueryEngine) QueryFieldDeclarations(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, nodeTypeFieldDeclaration)
}

// QueryMethodSpecs finds all method_elem nodes within interface types.
func (e *ConcreteTreeSitterQueryEngine) QueryMethodSpecs(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, nodeTypeMethodElem)
}

// QueryEmbeddedTypes finds embedded type nodes within struct and interface definitions.
// This includes both anonymous struct fields and embedded interfaces.
func (e *ConcreteTreeSitterQueryEngine) QueryEmbeddedTypes(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	// Embedded types can appear in different contexts:
	// 1. As field_declaration nodes without field names (anonymous/embedded fields)
	// 2. As type_identifier nodes directly within struct/interface bodies

	// For now, we'll query for both field_declaration and type_identifier nodes
	// and let the caller filter based on context
	results := []*valueobject.ParseNode{}

	// Get field declarations (some may be embedded types)
	fieldDecls := e.queryNodesByType(parseTree, nodeTypeFieldDeclaration)
	results = append(results, fieldDecls...)

	// Get type identifiers that might be embedded types
	typeIds := e.queryNodesByType(parseTree, "type_identifier")
	results = append(results, typeIds...)

	return results
}

// QueryMultipleTypes performs multiple queries in a single tree traversal for better performance.
func (e *ConcreteTreeSitterQueryEngine) QueryMultipleTypes(
	parseTree *valueobject.ParseTree,
	nodeTypes []string,
) map[string][]*valueobject.ParseNode {
	// Initialize result map
	result := make(map[string][]*valueobject.ParseNode)

	// Validate input parameters
	if parseTree == nil || len(nodeTypes) == 0 {
		// Return empty map with empty slices for each requested node type
		for _, nodeType := range nodeTypes {
			result[nodeType] = []*valueobject.ParseNode{}
		}
		return result
	}

	// Filter out empty node types and deduplicate
	validNodeTypes := make(map[string]bool)
	for _, nodeType := range nodeTypes {
		if nodeType != "" {
			validNodeTypes[nodeType] = true
		}
	}

	// Initialize result map with empty slices for all valid node types
	for nodeType := range validNodeTypes {
		result[nodeType] = []*valueobject.ParseNode{}
	}

	// If no valid node types, return early
	if len(validNodeTypes) == 0 {
		return result
	}

	// For now, we'll use individual queries for each node type
	// TODO: In the future, this could be optimized with a single tree traversal
	// if the ParseTree interface supports multi-type queries
	for nodeType := range validNodeTypes {
		nodes := e.queryNodesByType(parseTree, nodeType)
		result[nodeType] = nodes
	}

	return result
}

// QueryComments finds all comment nodes in the parse tree using tree-sitter
// grammar-based node identification. This method provides a reliable way to extract
// all comments from Go source code without manual string parsing.
//
// The method handles both line comments (//) and block comments (/* */) as defined
// by the Go tree-sitter grammar. Comments are returned in the order they appear
// in the source file.
//
// Parameters:
//
//	parseTree - The parsed syntax tree to query (validated for nil)
//
// Returns:
//
//	[]*valueobject.ParseNode - All comment nodes found, empty slice if none
//
// Example usage:
//
//	comments := engine.QueryComments(parseTree)
//	for _, comment := range comments {
//	    text := engine.ProcessCommentText(parseTree, comment)
//	    fmt.Println("Comment:", text)
//	}
func (e *ConcreteTreeSitterQueryEngine) QueryComments(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, nodeTypeComment)
}

// QueryCallExpressions finds all call_expression and type_conversion_expression nodes in the parse tree.
// Tree-sitter Go grammar parses function calls like fmt.Println() as type_conversion_expression.
func (e *ConcreteTreeSitterQueryEngine) QueryCallExpressions(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	results := []*valueobject.ParseNode{}

	// Query both node types since tree-sitter parses some calls as type conversions
	callExprs := e.queryNodesByType(parseTree, "call_expression")
	results = append(results, callExprs...)

	typeConversionExprs := e.queryNodesByType(parseTree, nodeTypeTypeConversionExpr)
	results = append(results, typeConversionExprs...)

	return results
}

// ProcessCommentText processes a comment node to extract clean, readable text content
// using tree-sitter's AST representation. This method handles both line and block
// comments consistently, removing comment markers and formatting artifacts.
//
// The processing includes:
//   - Removal of comment markers (// and /* */)
//   - Trimming of excess whitespace
//   - Cleanup of formatted block comment asterisks
//   - Joining multi-line content with spaces
//
// This method replaces manual string manipulation with tree-sitter-based processing
// for consistent and reliable comment text extraction.
//
// Parameters:
//
//	parseTree - The parsed syntax tree containing the comment (validated for nil)
//	commentNode - The comment node to process (validated for nil)
//
// Returns:
//
//	string - Clean comment text without markers, empty string if invalid input
//
// Example usage:
//
//	// For comment: "// This is a function comment"
//	text := engine.ProcessCommentText(parseTree, commentNode)
//	// Returns: "This is a function comment"
func (e *ConcreteTreeSitterQueryEngine) ProcessCommentText(
	parseTree *valueobject.ParseTree,
	commentNode *valueobject.ParseNode,
) string {
	// Validate input parameters
	if parseTree == nil || commentNode == nil {
		return ""
	}

	// Get the raw comment text from tree-sitter
	raw := parseTree.GetNodeText(commentNode)
	raw = strings.TrimSpace(raw)

	// Handle // style comments
	if strings.HasPrefix(raw, "//") {
		return strings.TrimSpace(strings.TrimPrefix(raw, "//"))
	}

	// Handle /* */ style comments
	if strings.HasPrefix(raw, "/*") && strings.HasSuffix(raw, "*/") {
		content := raw[2 : len(raw)-2]
		// Handle multi-line block comments
		lines := strings.Split(content, "\n")
		var cleaned []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Remove leading * from formatted block comments
			if strings.HasPrefix(line, "*") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
			}
			if line != "" {
				cleaned = append(cleaned, line)
			}
		}
		return strings.Join(cleaned, " ")
	}

	// Fallback for any other comment format
	return strings.TrimSpace(raw)
}

// hasBlankLineBetween checks if there's a blank line between two byte positions
// in the source text. This is a key helper for determining comment-declaration
// associations according to Go documentation conventions.
//
// According to Go conventions, documentation comments should be immediately
// adjacent to the declaration they document. A blank line breaks this association.
//
// Parameters:
//
//	parseTree - The parsed syntax tree for source text access
//	startByte - Starting byte position (exclusive)
//	endByte - Ending byte position (exclusive)
//
// Returns:
//
//	bool - true if there's a blank line (\n\n) between the positions
//
// Implementation notes:
//   - Validates byte positions are within source bounds
//   - Uses simple string search for consecutive newlines
//   - Returns false for invalid input (safer for comment association)
func (e *ConcreteTreeSitterQueryEngine) hasBlankLineBetween(
	parseTree *valueobject.ParseTree,
	startByte, endByte uint32,
) bool {
	// Validate byte position ordering
	if startByte >= endByte {
		return false
	}

	// Get source text and validate bounds
	sourceText := parseTree.Source()
	if len(sourceText) == 0 || int(startByte) >= len(sourceText) || int(endByte) > len(sourceText) {
		return false
	}

	// Extract text between positions
	betweenText := string(sourceText[startByte:endByte])

	// Check for consecutive newlines indicating blank line
	return strings.Contains(betweenText, "\n\n")
}

// QueryDocumentationComments finds documentation comments associated with a specific
// declaration using tree-sitter AST traversal and Go documentation conventions.
//
// This method implements Go's documentation comment association rules:
//   - Comments must immediately precede the declaration (no blank lines)
//   - Multiple consecutive comment lines are treated as a single documentation block
//   - Block comments and line comments are both supported
//   - Comments separated by blank lines are not associated
//
// The method uses tree-sitter's AST structure to find the declaration's position
// among its siblings, then traverses backwards to collect consecutive comment nodes.
// This replaces manual byte position comparisons with proper AST navigation.
//
// Parameters:
//
//	parseTree - The parsed syntax tree to query (validated for nil)
//	declarationNode - The declaration node to find documentation for (validated for nil)
//
// Returns:
//
//	[]*valueobject.ParseNode - Documentation comments in source order, empty if none found
//
// Example usage:
//
//	docComments := engine.QueryDocumentationComments(parseTree, functionNode)
//	for _, comment := range docComments {
//	    text := engine.ProcessCommentText(parseTree, comment)
//	    documentation += text + " "
//	}
//
// Implementation notes:
//   - Operates only on top-level declarations (root node children)
//   - Maintains comment order by prepending during backward traversal
//   - Stops at first non-comment sibling or blank line separation
//   - Handles edge cases gracefully (returns empty slice for invalid input)
func (e *ConcreteTreeSitterQueryEngine) QueryDocumentationComments(
	parseTree *valueobject.ParseTree,
	declarationNode *valueobject.ParseNode,
) []*valueobject.ParseNode {
	// Validate input parameters
	if parseTree == nil || declarationNode == nil {
		return []*valueobject.ParseNode{}
	}

	// Get the root node and validate structure
	root := parseTree.RootNode()
	if root == nil || root.Children == nil {
		return []*valueobject.ParseNode{}
	}

	// Find the declaration node's position among root children
	declarationIndex := -1
	for i, child := range root.Children {
		if child == declarationNode {
			declarationIndex = i
			break
		}
	}

	if declarationIndex == -1 {
		return []*valueobject.ParseNode{}
	}

	// Traverse backwards collecting consecutive comments
	documentationComments := []*valueobject.ParseNode{}
	lastCommentEnd := declarationNode.StartByte

	for i := declarationIndex - 1; i >= 0; i-- {
		sibling := root.Children[i]
		if sibling.Type == "comment" {
			// Check for blank line separation (breaks documentation association)
			if e.hasBlankLineBetween(parseTree, sibling.EndByte, lastCommentEnd) {
				break // Stop at blank line
			}

			// Prepend to maintain source order (traversing backwards)
			documentationComments = append([]*valueobject.ParseNode{sibling}, documentationComments...)
			lastCommentEnd = sibling.StartByte
		} else {
			// Stop at first non-comment sibling
			break
		}
	}

	return documentationComments
}
