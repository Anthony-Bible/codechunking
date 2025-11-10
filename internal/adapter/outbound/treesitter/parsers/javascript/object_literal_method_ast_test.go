package javascriptparser

import (
	"codechunking/internal/domain/valueobject"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestObjectLiteralMethodAST inspects the AST structure of object literal methods
// to verify that both regular shorthand methods and computed property methods
// are parsed as method_definition nodes by tree-sitter.
func TestObjectLiteralMethodAST(t *testing.T) {
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
const manualIterator = {
	next() {
		return { value: 1, done: false };
	},
	[Symbol.iterator]() {
		return this;
	}
};
`

	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	t.Logf("=== AST Structure for Object Literal Methods ===\n")
	printNodeTree(t, domainTree.RootNode(), domainTree, 0)

	// Verify that we find method_definition nodes
	methodDefinitions := findNodesByType(domainTree.RootNode(), "method_definition")
	t.Logf("\n=== Found %d method_definition nodes ===\n", len(methodDefinitions))
	for i, methodNode := range methodDefinitions {
		methodName := extractMethodNameForTest(domainTree, methodNode)
		t.Logf(
			"Method %d: %s (type=%s, bytes=%d-%d)\n",
			i+1,
			methodName,
			methodNode.Type,
			methodNode.StartByte,
			methodNode.EndByte,
		)
	}

	// Verify expectations
	require.Len(t, methodDefinitions, 2, "Expected 2 method_definition nodes (next and [Symbol.iterator])")
}

// findNodesByType recursively finds all nodes of a given type.
func findNodesByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var nodes []*valueobject.ParseNode

	if node == nil {
		return nodes
	}

	if node.Type == nodeType {
		nodes = append(nodes, node)
	}

	for _, child := range node.Children {
		nodes = append(nodes, findNodesByType(child, nodeType)...)
	}

	return nodes
}

// extractMethodNameForTest extracts the method name for testing purposes.
func extractMethodNameForTest(tree *valueobject.ParseTree, methodNode *valueobject.ParseNode) string {
	// Look for the name field using ChildByFieldName
	if nameNode := methodNode.ChildByFieldName("name"); nameNode != nil {
		return tree.GetNodeText(nameNode)
	}
	return "<unknown>"
}

// printNodeTree recursively prints the AST tree structure.
func printNodeTree(t *testing.T, node *valueobject.ParseNode, tree *valueobject.ParseTree, depth int) {
	if node == nil {
		return
	}

	indent := ""
	var indentSb87 strings.Builder
	for range depth {
		indentSb87.WriteString("  ")
	}
	indent += indentSb87.String()

	// Get node text (truncate if too long)
	nodeText := tree.GetNodeText(node)
	if len(nodeText) > 50 {
		nodeText = nodeText[:50] + "..."
	}
	nodeText = escapeNewlines(nodeText)

	// Print node info
	t.Logf("%s%s: %q (%d-%d)\n", indent, node.Type, nodeText, node.StartByte, node.EndByte)

	// Recursively print children
	for _, child := range node.Children {
		printNodeTree(t, child, tree, depth+1)
	}
}

// escapeNewlines replaces newlines with \n for better readability.
func escapeNewlines(s string) string {
	result := ""
	for _, ch := range s {
		switch ch {
		case '\n':
			result += "\\n"
		case '\t':
			result += "\\t"
		default:
			result += string(ch)
		}
	}
	return result
}
