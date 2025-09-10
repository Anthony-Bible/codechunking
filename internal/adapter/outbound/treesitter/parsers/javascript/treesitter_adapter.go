package javascriptparser

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexaandru/go-sitter-forest/javascript"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

// JavaScriptTreeSitterAdapter provides tree-sitter integration for JavaScript parsing.
type JavaScriptTreeSitterAdapter struct {
	parser *tree_sitter.Parser
	lang   *tree_sitter.Language
}

// NewJavaScriptTreeSitterAdapter creates a new JavaScript tree-sitter adapter.
func NewJavaScriptTreeSitterAdapter() (*JavaScriptTreeSitterAdapter, error) {
	parser := tree_sitter.NewParser()
	jsLang := tree_sitter.NewLanguage(javascript.GetLanguage())

	if !parser.SetLanguage(jsLang) {
		return nil, errors.New("failed to set JavaScript language in tree-sitter parser")
	}

	return &JavaScriptTreeSitterAdapter{
		parser: parser,
		lang:   jsLang,
	}, nil
}

// ParseSource parses JavaScript source code and returns a ParseTree.
func (adapter *JavaScriptTreeSitterAdapter) ParseSource(
	ctx context.Context,
	language valueobject.Language,
	source []byte,
) (*valueobject.ParseTree, error) {
	startTime := time.Now()

	tree, err := adapter.parser.ParseString(ctx, nil, source)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parsing failed: %w", err)
	}
	defer tree.Close()

	parseDuration := time.Since(startTime)

	// Convert tree-sitter tree to our ParseNode structure
	rootNode := adapter.convertTreeSitterNode(tree.RootNode(), source)

	// Calculate metadata
	nodeCount := adapter.countNodes(rootNode)
	maxDepth := adapter.calculateMaxDepth(rootNode)

	metadata, err := valueobject.NewParseMetadata(
		parseDuration,
		"0.20.8",         // tree-sitter version
		"javascript-1.0", // grammar version
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse metadata: %w", err)
	}

	metadata.NodeCount = nodeCount
	metadata.MaxDepth = maxDepth

	parseTree, err := valueobject.NewParseTree(ctx, language, rootNode, source, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse tree: %w", err)
	}

	slogger.Debug(ctx, "JavaScript source parsed successfully", slogger.Fields{
		"source_length":  len(source),
		"node_count":     nodeCount,
		"max_depth":      maxDepth,
		"parse_duration": parseDuration.String(),
	})

	return parseTree, nil
}

// convertTreeSitterNode converts a tree-sitter node to our ParseNode structure.
func (adapter *JavaScriptTreeSitterAdapter) convertTreeSitterNode(
	tsNode tree_sitter.Node,
	source []byte,
) *valueobject.ParseNode {
	node := &valueobject.ParseNode{
		Type:      tsNode.Type(),
		StartByte: valueobject.ClampUintToUint32(tsNode.StartByte()),
		EndByte:   valueobject.ClampUintToUint32(tsNode.EndByte()),
		StartPos: valueobject.Position{
			Row:    valueobject.ClampUintToUint32(tsNode.StartPoint().Row),
			Column: valueobject.ClampUintToUint32(tsNode.StartPoint().Column),
		},
		EndPos: valueobject.Position{
			Row:    valueobject.ClampUintToUint32(tsNode.EndPoint().Row),
			Column: valueobject.ClampUintToUint32(tsNode.EndPoint().Column),
		},
		Children: make([]*valueobject.ParseNode, 0),
	}

	// Convert all children
	childCount := tsNode.ChildCount()
	for i := range childCount {
		child := tsNode.Child(i)
		if !child.IsNull() {
			childNode := adapter.convertTreeSitterNode(child, source)
			node.Children = append(node.Children, childNode)
		}
	}

	return node
}

// countNodes counts the total number of nodes in the parse tree.
func (adapter *JavaScriptTreeSitterAdapter) countNodes(node *valueobject.ParseNode) int {
	if node == nil {
		return 0
	}

	count := 1 // Count this node
	for _, child := range node.Children {
		count += adapter.countNodes(child)
	}

	return count
}

// calculateMaxDepth calculates the maximum depth of the parse tree.
func (adapter *JavaScriptTreeSitterAdapter) calculateMaxDepth(node *valueobject.ParseNode) int {
	if node == nil || len(node.Children) == 0 {
		return 1
	}

	maxChildDepth := 0
	for _, child := range node.Children {
		childDepth := adapter.calculateMaxDepth(child)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return 1 + maxChildDepth
}

// Close releases any resources held by the adapter.
func (adapter *JavaScriptTreeSitterAdapter) Close() {
	// The parser and language are automatically cleaned up by Go's runtime.AddCleanup
	// No explicit cleanup needed for go-tree-sitter-bare
}
