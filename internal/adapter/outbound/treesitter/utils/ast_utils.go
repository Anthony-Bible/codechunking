// Package utils provides tree-sitter parsing utilities for semantic code traversal.
package utils

import (
	"codechunking/internal/domain/valueobject"
)

const (
	defaultPackageName = "main"
)

// ParseTreeProvider defines the interface for parse tree operations.
// This matches the interface expected by the test files.
type ParseTreeProvider interface {
	GetNodesByType(nodeType string) []*valueobject.ParseNode
	GetNodeText(node *valueobject.ParseNode) string
}

// FindChildByType finds the first child node of a specific type.
// This is an optimized implementation extracted from the main semantic traverser.
func FindChildByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	if node == nil {
		return nil
	}

	// Iterate through children to find matching type
	for _, child := range node.Children {
		if child.Type == nodeType {
			return child
		}
	}
	return nil
}

// FindChildrenByType finds all child nodes of a specific type.
// Returns nil slice if parent is nil, empty slice if no children match.
func FindChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	if node == nil {
		return nil // Maintain original behavior from semantic traverser
	}

	var children []*valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeType {
			children = append(children, child)
		}
	}
	return children
}

// ExtractPackageNameFromTree extracts the package name from the parse tree.
// This implementation matches the production semantic traverser logic.
func ExtractPackageNameFromTree(parseTree ParseTreeProvider) string {
	packageNodes := parseTree.GetNodesByType("package_clause")
	if len(packageNodes) > 0 {
		identifier := FindChildByType(packageNodes[0], "package_identifier")
		if identifier != nil {
			packageName := parseTree.GetNodeText(identifier)
			if packageName != "" {
				return packageName
			}
		}
	}
	return defaultPackageName // Default fallback when no package clause found
}

// IsNodeType checks if a node is of a specific type.
func IsNodeType(node *valueobject.ParseNode, nodeType string) bool {
	if node == nil {
		return false
	}
	return node.Type == nodeType
}

// GetNodesByType gets all nodes of a specific type from the parse tree.
func GetNodesByType(parseTree ParseTreeProvider, nodeType string) []*valueobject.ParseNode {
	return parseTree.GetNodesByType(nodeType)
}
