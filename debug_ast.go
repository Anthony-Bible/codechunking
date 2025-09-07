package main

import (
	pythonparser "codechunking/internal/adapter/outbound/treesitter/parsers/python"
	"codechunking/internal/domain/valueobject"
	"context"
	"fmt"
)

func main() {
	sourceCode := `@dataclass
class Dog:
    name: str

class Cat:
    pass`

	parser, err := pythonparser.NewPythonParser()
	if err != nil {
		panic(err)
	}

	result, err := parser.Parse(context.Background(), []byte(sourceCode))
	if err != nil {
		panic(err)
	}

	// Convert to valueobject.ParseTree
	parseTree := result.ParseTree

	fmt.Println("=== AST Structure ===")
	printAST(parseTree.RootNode(), 0, parseTree, sourceCode)

	// Show specific node types
	fmt.Println("\n=== Class definition nodes ===")
	classNodes := parseTree.GetNodesByType("class_definition")
	for i, node := range classNodes {
		fmt.Printf("Class %d: %s (bytes %d-%d)\n", i, parseTree.GetNodeText(node), node.StartByte(), node.EndByte())
	}

	fmt.Println("\n=== Decorated definition nodes ===")
	decoratedNodes := parseTree.GetNodesByType("decorated_definition")
	for i, node := range decoratedNodes {
		fmt.Printf("Decorated %d: %s (bytes %d-%d)\n", i, parseTree.GetNodeText(node), node.StartByte(), node.EndByte())
	}
}

func printAST(node valueobject.ParseNode, depth int, parseTree *valueobject.ParseTree, source string) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	text := parseTree.GetNodeText(&node)
	if len(text) > 50 {
		text = text[:50] + "..."
	}

	fmt.Printf("%s%s: \"%s\" (bytes %d-%d)\n", indent, node.Type(), text, node.StartByte(), node.EndByte())

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		printAST(*child, depth+1, parseTree, source)
	}
}
