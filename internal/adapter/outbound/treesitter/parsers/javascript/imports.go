package javascriptparser

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// extractJavaScriptImports extracts import declarations from JavaScript code using real AST analysis.
func extractJavaScriptImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	slogger.Debug(ctx, "Starting JavaScript import extraction", slogger.Fields{})

	var imports []outbound.ImportDeclaration
	now := time.Now()

	// Traverse the AST to find all import-related constructs
	imports = append(imports, traverseForImports(parseTree, parseTree.RootNode(), now)...)

	slogger.Debug(ctx, "JavaScript import extraction completed", slogger.Fields{
		"imports_count": len(imports),
	})

	return imports, nil
}

// traverseForImports recursively traverses the AST to find import declarations.
func traverseForImports(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	now time.Time,
) []outbound.ImportDeclaration {
	if node == nil {
		return nil
	}

	var imports []outbound.ImportDeclaration

	// Process current node if it's an import-like construct
	if imp := processImportNode(parseTree, node, now); imp != nil {
		imports = append(imports, *imp)
	}

	// Recursively process children
	for _, child := range node.Children {
		imports = append(imports, traverseForImports(parseTree, child, now)...)
	}

	return imports
}

// processImportNode processes a single node if it represents an import construct.
func processImportNode(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	now time.Time,
) *outbound.ImportDeclaration {
	switch node.Type {
	case "import_statement":
		return parseES6Import(parseTree, node, now)
	case "variable_declarator":
		// Check if this is a require() statement
		return parseCommonJSRequire(parseTree, node, now)
	case "call_expression":
		// Check for dynamic import() calls
		return parseDynamicImport(parseTree, node, now)
	default:
		return nil
	}
}

// parseES6Import parses ES6 import statements.
func parseES6Import(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	now time.Time,
) *outbound.ImportDeclaration {
	// Get import source
	var source string
	if sourceNode := findChildByType(node, "string"); sourceNode != nil {
		source = strings.Trim(parseTree.GetNodeText(sourceNode), `"'`)
	}

	if source == "" {
		return nil
	}

	// Parse imported symbols
	var importedSymbols []string
	var alias string
	var isWildcard bool

	// Handle default imports (import React from 'react')
	if defaultNode := findChildByType(node, "import_default_specifier"); defaultNode != nil {
		if identifierNode := findChildByType(defaultNode, "identifier"); identifierNode != nil {
			name := parseTree.GetNodeText(identifierNode)
			importedSymbols = append(importedSymbols, name)
		}
	}

	// Handle named imports (import { useState, useEffect } from 'react')
	if namedImportsNode := findChildByType(node, "import_clause"); namedImportsNode != nil {
		if namedSpecifiersNode := findChildByType(namedImportsNode, "named_imports"); namedSpecifiersNode != nil {
			// Find all import specifiers
			for _, child := range namedSpecifiersNode.Children {
				if child.Type == "import_specifier" {
					if identifierNode := findChildByType(child, "identifier"); identifierNode != nil {
						name := parseTree.GetNodeText(identifierNode)
						importedSymbols = append(importedSymbols, name)
					}
				}
			}
		}
	}

	// Handle namespace imports (import * as React from 'react')
	if namespaceNode := findChildByType(node, "namespace_import"); namespaceNode != nil {
		if identifierNode := findChildByType(namespaceNode, "identifier"); identifierNode != nil {
			alias = parseTree.GetNodeText(identifierNode)
			isWildcard = true
			importedSymbols = append(importedSymbols, "*")
		}
	}

	return &outbound.ImportDeclaration{
		Path:            source,
		Alias:           alias,
		IsWildcard:      isWildcard,
		ImportedSymbols: importedSymbols,
		StartByte:       node.StartByte,
		EndByte:         node.EndByte,
		Content:         parseTree.GetNodeText(node),
		ExtractedAt:     now,
	}
}

// parseCommonJSRequire parses CommonJS require statements.
func parseCommonJSRequire(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	now time.Time,
) *outbound.ImportDeclaration {
	// Look for require() call in variable declarator
	var source string
	callNode := findChildByType(node, "call_expression")
	if callNode == nil {
		return nil
	}

	// Check if this is a require call
	functionNode := findChildByType(callNode, "identifier")
	if functionNode == nil {
		return nil
	}

	functionName := parseTree.GetNodeText(functionNode)
	if functionName != "require" {
		return nil
	}

	// Extract the module path
	argsNode := findChildByType(callNode, "arguments")
	if argsNode == nil {
		return nil
	}

	stringNode := findChildByType(argsNode, "string")
	if stringNode != nil {
		source = strings.Trim(parseTree.GetNodeText(stringNode), `"'`)
	}

	if source == "" {
		return nil
	}

	// Get variable name (the thing being assigned to)
	var importedSymbols []string
	var alias string
	if identifierNode := findChildByType(node, "identifier"); identifierNode != nil {
		alias = parseTree.GetNodeText(identifierNode)
		importedSymbols = append(importedSymbols, alias)
	}

	return &outbound.ImportDeclaration{
		Path:            source,
		Alias:           alias,
		IsWildcard:      false,
		ImportedSymbols: importedSymbols,
		StartByte:       node.StartByte,
		EndByte:         node.EndByte,
		Content:         parseTree.GetNodeText(node),
		ExtractedAt:     now,
	}
}

// parseDynamicImport parses dynamic import() calls.
func parseDynamicImport(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	now time.Time,
) *outbound.ImportDeclaration {
	// Check if this is import() call
	if funcNode := findChildByType(node, "import"); funcNode == nil {
		return nil
	}

	// Look for import() call arguments
	var source string
	if argsNode := findChildByType(node, "arguments"); argsNode != nil {
		if stringNode := findChildByType(argsNode, "string"); stringNode != nil {
			source = strings.Trim(parseTree.GetNodeText(stringNode), `"'`)
		}
	}

	if source == "" {
		return nil
	}

	return &outbound.ImportDeclaration{
		Path:        source,
		IsWildcard:  false,
		StartByte:   node.StartByte,
		EndByte:     node.EndByte,
		Content:     parseTree.GetNodeText(node),
		ExtractedAt: now,
	}
}

// Helper methods for parsing imports

// findChildByType finds the first child node with the specified type.
func findChildByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	if node == nil {
		return nil
	}

	for _, child := range node.Children {
		if child.Type == nodeType {
			return child
		}
	}
	return nil
}

// findChildrenByType finds all child nodes with the specified type.
func findChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	if node == nil {
		return nil
	}

	var matches []*valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeType {
			matches = append(matches, child)
		}
	}
	return matches
}

// findParentByType finds the parent node with the specified type (simplified implementation).
func findParentByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	// This is a simplified implementation - in a real scenario, we'd need to traverse up the tree
	// For now, return nil as this is minimal GREEN PHASE implementation
	return nil
}
