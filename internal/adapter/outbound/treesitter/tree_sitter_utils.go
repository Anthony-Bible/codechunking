package treesitter

import (
	"codechunking/internal/domain/valueobject"

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
