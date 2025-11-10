//nolint:forbidigo // This is a test utility that intentionally uses fmt.Print for debugging
package main

import (
	"context"
	"fmt"
	"strings"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

func analyzeCode(name, sourceCode string) {
	fmt.Printf("\n=== %s ===\n", name)
	fmt.Printf("Source: %q\n\n", sourceCode)

	ctx := context.Background()
	grammar := forest.GetLanguage("go")
	parser := tree_sitter.NewParser()
	parser.SetLanguage(grammar)

	tree, _ := parser.ParseString(ctx, nil, []byte(sourceCode))
	defer tree.Close()

	root := tree.RootNode()
	fmt.Printf("Root node type: %s\n", root.Type())
	fmt.Printf("Root.HasError(): %v\n", root.HasError())
	fmt.Printf("Root.IsError(): %v\n", root.IsError())
	fmt.Printf("Root.IsMissing(): %v\n\n", root.IsMissing())

	// Print the tree structure
	fmt.Println("Tree structure:")
	printTree(root, 0, []byte(sourceCode))

	// Check all nodes
	fmt.Println("\nDetailed node analysis:")
	analyzeNode(root, []byte(sourceCode), "")
}

func printTree(node tree_sitter.Node, depth int, source []byte) {
	if node.IsNull() {
		return
	}

	var indent strings.Builder
	for range depth {
		indent.WriteString("  ")
	}
	indentStr := indent.String()

	nodeType := node.Type()
	content := source[node.StartByte():node.EndByte()]
	if len(content) > 40 {
		content = append(content[:40], []byte("...")...)
	}

	flags := ""
	if node.IsError() {
		flags += " [IsError]"
	}
	if node.IsMissing() {
		flags += " [IsMissing]"
	}
	if node.HasError() {
		flags += " [HasError]"
	}

	fmt.Printf("%s%s%s: %q\n", indentStr, nodeType, flags, content)

	for i := range node.ChildCount() {
		child := node.Child(i)
		printTree(child, depth+1, source)
	}
}

func analyzeNode(node tree_sitter.Node, source []byte, path string) {
	if node.IsNull() {
		return
	}

	nodeType := node.Type()
	currentPath := path + "/" + nodeType

	// Report any error conditions
	if node.IsError() {
		content := source[node.StartByte():node.EndByte()]
		if len(content) > 40 {
			content = append(content[:40], []byte("...")...)
		}
		fmt.Printf("  ERROR node at %s: %q\n", currentPath, content)
	}
	if node.IsMissing() {
		fmt.Printf("  MISSING node at %s: type=%s\n", currentPath, nodeType)
	}

	// Recurse to children
	for i := range node.ChildCount() {
		child := node.Child(i)
		analyzeNode(child, source, currentPath)
	}
}

func main() {
	// Test case 1: incomplete function
	analyzeCode(
		"Incomplete Function",
		"package main\n\nfunc incomplete(",
	)

	// Test case 2: malformed struct
	analyzeCode(
		"Malformed Struct",
		"package main\n\ntype BadStruct struct {\n    field string",
	)

	// Additional test: simpler incomplete function
	analyzeCode(
		"Simple Incomplete Function",
		"func incomplete(",
	)

	// Additional test: just struct incomplete
	analyzeCode(
		"Simple Incomplete Struct",
		"type BadStruct struct {\n    field string",
	)
}
