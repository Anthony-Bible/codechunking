package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"

	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

// convertTreeSitterNode converts a tree-sitter node to domain ParseNode recursively.
func convertTreeSitterNode(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Convert tree-sitter node to domain ParseNode
	parseNode := &valueobject.ParseNode{
		Type:      node.Type(),
		StartByte: safeUintToUint32(node.StartByte()),
		EndByte:   safeUintToUint32(node.EndByte()),
		StartPos: valueobject.Position{
			Row:    safeUintToUint32(node.StartPoint().Row),
			Column: safeUintToUint32(node.StartPoint().Column),
		},
		EndPos: valueobject.Position{
			Row:    safeUintToUint32(node.EndPoint().Row),
			Column: safeUintToUint32(node.EndPoint().Column),
		},
		Children: []*valueobject.ParseNode{},
	}

	// Convert child nodes recursively
	nodeCount := 1 // Count current node
	maxDepth := depth

	childCount := int(node.ChildCount())
	for i := range childCount {
		childNode := node.Child(safeUintToUint32(safeIntToUint(i)))
		childParseNode, childNodeCount, childMaxDepth := convertTreeSitterNode(childNode, depth+1)
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

// ConvertTreeSitterNodeWithPreservation converts a tree-sitter node to domain ParseNode recursively,
// preserving tree-sitter node references for Content() method usage.
// This function is exported for use by language-specific parsers.
func ConvertTreeSitterNodeWithPreservation(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Use the new constructor that preserves tree-sitter node reference
	parseNode, err := valueobject.NewParseNodeWithTreeSitter(
		node.Type(),
		safeUintToUint32(node.StartByte()),
		safeUintToUint32(node.EndByte()),
		valueobject.Position{
			Row:    safeUintToUint32(node.StartPoint().Row),
			Column: safeUintToUint32(node.StartPoint().Column),
		},
		valueobject.Position{
			Row:    safeUintToUint32(node.EndPoint().Row),
			Column: safeUintToUint32(node.EndPoint().Column),
		},
		[]*valueobject.ParseNode{},
		node,
	)
	if err != nil {
		// Fallback to legacy conversion if new constructor fails
		return convertTreeSitterNode(node, depth)
	}

	// Convert child nodes recursively
	nodeCount := 1 // Count current node
	maxDepth := depth

	childCount := int(node.ChildCount())
	for i := range childCount {
		childNode := node.Child(safeUintToUint32(safeIntToUint(i)))
		childParseNode, childNodeCount, childMaxDepth := ConvertTreeSitterNodeWithPreservation(childNode, depth+1)
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

// safeUintToUint32 safely converts uint to uint32 with bounds checking.
func safeUintToUint32(val uint) uint32 {
	if val > uint(^uint32(0)) {
		return ^uint32(0) // Return max uint32 if overflow
	}
	return uint32(val)
}

// safeIntToUint safely converts int to uint with bounds checking.
func safeIntToUint(val int) uint {
	if val < 0 {
		return 0 // Return 0 if negative
	}
	return uint(val)
}

// ============================================================================
// Shared TreeSitter Utilities - Extract common patterns to avoid cycles
// ============================================================================

// ParseTreeResult represents the result of parsing source code with TreeSitter.
// This struct encapsulates both the domain parse tree and any error that occurred.
type ParseTreeResult struct {
	ParseTree *valueobject.ParseTree
	Error     error
}

// CreateTreeSitterParseTree creates a parse tree from Go source code using TreeSitter.
// This function consolidates the repeated pattern used across validation methods:
//  1. Create parser factory
//  2. Create Go language parser
//  3. Parse the source code
//  4. Convert to domain parse tree
//
// This eliminates code duplication and provides a consistent interface for
// all validation methods that need to parse Go source code.
//
// Parameters:
//
//	ctx - Context for cancellation and timeout control
//	source - Go source code to parse (as string)
//
// Returns:
//
//	*ParseTreeResult - Contains either the parsed tree or an error
//
// Example usage:
//
//	result := CreateTreeSitterParseTree(ctx, sourceCode)
//	if result.Error != nil {
//	    return fmt.Errorf("parse failed: %w", result.Error)
//	}
//	// Use result.ParseTree for validation
func CreateTreeSitterParseTree(ctx context.Context, source string) *ParseTreeResult {
	// Create TreeSitter parser factory
	factory, err := NewTreeSitterParserFactory(ctx)
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to create parser factory: %w", err),
		}
	}

	// Create Go language value object
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to create language: %w", err),
		}
	}

	// Create parser for Go language
	parser, err := factory.CreateParser(ctx, goLang)
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to create parser: %w", err),
		}
	}

	// Parse the source code
	parseResult, err := parser.Parse(ctx, []byte(source))
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to parse source: %w", err),
		}
	}

	// Convert to domain parse tree
	domainTree, err := ConvertPortParseTreeToDomain(parseResult.ParseTree)
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to convert parse tree: %w", err),
		}
	}

	return &ParseTreeResult{
		ParseTree: domainTree,
		Error:     nil,
	}
}

// ValidateSourceWithTreeSitter performs basic syntax validation using TreeSitter error detection.
// This function provides a common pattern for validating Go source code syntax
// by checking for tree-sitter error nodes in the parsed AST.
//
// Parameters:
//
//	ctx - Context for cancellation and timeout control
//	source - Go source code to validate
//
// Returns:
//
//	error - Syntax error if found, nil if valid
func ValidateSourceWithTreeSitter(ctx context.Context, source string) error {
	result := CreateTreeSitterParseTree(ctx, source)
	if result.Error != nil {
		return result.Error
	}

	// Check for syntax errors in the parse tree
	if hasErrors, _ := result.ParseTree.HasSyntaxErrors(); hasErrors {
		return errors.New("syntax error detected")
	}

	return nil
}
