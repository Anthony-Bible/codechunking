// Package goparser provides Go-specific import extraction functionality for tree-sitter parsing.
//
// This file implements comprehensive import declaration extraction from Go source code
// using tree-sitter's Abstract Syntax Tree (AST) parsing capabilities. It handles all
// Go import patterns including:
//
// 1. Single imports: import "fmt"
// 2. Grouped imports: import ( "fmt"; "os" )
// 3. Aliased imports: import alias "package"
// 4. Dot imports: import . "package" (for wildcard imports)
// 5. Blank imports: import _ "package" (for side-effect imports)
//
// The implementation uses a hierarchical approach to parse different import structures:
// - parseGoImportDeclaration: Main entry point that delegates to specific parsers
// - parseGroupedImports: Handles parenthesized import groups
// - parseDirectImportSpecs: Handles direct import_spec nodes
// - parseBareImport: Handles simple single imports
//
// Key features:
// - Robust error handling and validation
// - Centralized alias detection logic
// - Enhanced logging for debugging and monitoring
// - Consistent metadata extraction
// - Tree-sitter query engine integration for reliable AST traversal
package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"strings"
	"time"
)

// ObservableGoParser delegation methods for import extraction

// ExtractImports extracts import declarations from a Go parse tree.
func (o *ObservableGoParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	return o.parser.ExtractImports(ctx, parseTree, options)
}

// ExtractImports extracts import declarations from a Go parse tree.
// This function provides comprehensive error handling and validation for import extraction.
func (p *GoParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	slogger.Info(ctx, "Extracting Go imports", slogger.Fields{})

	// Validate input parameters
	if err := p.validateInput(parseTree); err != nil {
		slogger.Error(ctx, "Import extraction validation failed", slogger.Fields{
			"error": err.Error(),
		})
		return nil, err
	}

	// Check for syntax errors in the parse tree
	if containsErrorNodes(parseTree) {
		return nil, errors.New("invalid syntax: syntax error detected by tree-sitter parser")
	}

	var imports []outbound.ImportDeclaration

	// Use TreeSitterQueryEngine for more robust AST querying
	queryEngine := NewTreeSitterQueryEngine()
	importNodes := queryEngine.QueryImportDeclarations(parseTree)

	slogger.Debug(ctx, "Found import declaration nodes", slogger.Fields{
		"import_node_count": len(importNodes),
	})

	// Process each import declaration node with error handling
	for i, node := range importNodes {
		if node == nil {
			slogger.Warn(ctx, "Encountered nil import node, skipping", slogger.Fields{
				"node_index": i,
			})
			continue
		}

		importDecls := parseGoImportDeclaration(parseTree, node)
		if len(importDecls) > 0 {
			imports = append(imports, importDecls...)
		}
	}

	slogger.Debug(ctx, "Import extraction completed", slogger.Fields{
		"total_imports_extracted": len(imports),
	})

	return imports, nil
}

// parseGoImportDeclaration parses an import declaration and returns all imports found.
// This function handles three different import patterns in Go:
// 1. Grouped imports with parentheses: import ( "fmt"; "os" )
// 2. Single imports within import_spec nodes: direct child import_spec
// 3. Bare single imports: import "fmt" (without import_spec wrapper).
func parseGoImportDeclaration(
	parseTree *valueobject.ParseTree,
	importDecl *valueobject.ParseNode,
) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	// Try grouped imports first (most common pattern)
	if groupedImports := parseGroupedImports(parseTree, importDecl); len(groupedImports) > 0 {
		return groupedImports
	}

	// Try direct import_spec children (single grouped import)
	if specImports := parseDirectImportSpecs(parseTree, importDecl); len(specImports) > 0 {
		return specImports
	}

	// Handle bare single imports (no import_spec wrapper)
	if bareImport := parseBareImport(parseTree, importDecl); bareImport != nil {
		imports = append(imports, *bareImport)
		return imports
	}

	// If we reach here, we couldn't parse any imports from this declaration
	// This could indicate a malformed or unsupported import structure
	return imports
}

// parseGroupedImports handles imports with import_spec_list (grouped imports with parentheses).
// This function specifically processes the Go import syntax:
//
//	import (
//	    "fmt"
//	    alias "package"
//	    . "dot-import"
//	    _ "blank-import"
//	)
//
// The function traverses the AST structure:
//
//	import_declaration -> import_spec_list -> [import_spec, import_spec, ...]
//
// Parameters:
//   - parseTree: The parsed syntax tree containing the source code
//   - importDecl: The import_declaration node to process
//
// Returns:
//   - []outbound.ImportDeclaration: All import declarations found within the grouped import
func parseGroupedImports(
	parseTree *valueobject.ParseTree,
	importDecl *valueobject.ParseNode,
) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	// Find all import_spec_list children (there should typically be only one)
	importSpecLists := FindDirectChildren(importDecl, "import_spec_list")
	for _, importSpecList := range importSpecLists {
		// Extract individual import specifications from the list
		importSpecs := FindDirectChildren(importSpecList, "import_spec")
		for _, importSpec := range importSpecs {
			if importDeclaration := parseGoImportSpec(parseTree, importSpec); importDeclaration != nil {
				imports = append(imports, *importDeclaration)
			}
		}
	}

	return imports
}

// parseDirectImportSpecs handles direct import_spec children (single grouped import).
// This function processes import declarations that have import_spec nodes directly
// under the import_declaration, without an intermediate import_spec_list.
//
// This handles cases like:
//
//	import "fmt" (where the parser creates an import_spec node)
//
// The function traverses the AST structure:
//
//	import_declaration -> [import_spec, import_spec, ...]
//
// Parameters:
//   - parseTree: The parsed syntax tree containing the source code
//   - importDecl: The import_declaration node to process
//
// Returns:
//   - []outbound.ImportDeclaration: All import declarations found as direct import_spec children
func parseDirectImportSpecs(
	parseTree *valueobject.ParseTree,
	importDecl *valueobject.ParseNode,
) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	// Find import_spec nodes directly under the import_declaration
	importSpecs := FindDirectChildren(importDecl, "import_spec")
	for _, importSpec := range importSpecs {
		if importDeclaration := parseGoImportSpec(parseTree, importSpec); importDeclaration != nil {
			imports = append(imports, *importDeclaration)
		}
	}

	return imports
}

// parseBareImport handles single imports without parentheses and without import_spec wrapper.
// This function processes the simplest form of Go imports where the import path is directly
// under the import_declaration node without any intermediate wrapper nodes.
//
// This handles cases like:
//
//	import "fmt"
//	import . "fmt"
//	import _ "fmt"
//	import alias "fmt"
//
// The function traverses the AST structure:
//
//	import_declaration -> interpreted_string_literal (and optional alias nodes)
//
// Parameters:
//   - parseTree: The parsed syntax tree containing the source code
//   - importDecl: The import_declaration node to process
//
// Returns:
//   - *outbound.ImportDeclaration: The import declaration if found, nil if not a bare import
func parseBareImport(
	parseTree *valueobject.ParseTree,
	importDecl *valueobject.ParseNode,
) *outbound.ImportDeclaration {
	// Look for direct interpreted_string_literal children (the import path)
	pathNodes := FindDirectChildren(importDecl, "interpreted_string_literal")
	if len(pathNodes) == 0 {
		// This is expected for imports that use import_spec or import_spec_list structures
		return nil
	}

	// Extract alias using centralized logic
	alias := extractImportAliasFromNode(parseTree, importDecl)

	// Create the import declaration with proper validation
	if importDeclaration := createImportFromPath(parseTree, importDecl, pathNodes[0], alias); importDeclaration != nil {
		return importDeclaration
	}

	return nil
}

// parseGoImportSpec parses an import specification with comprehensive validation.
// Returns nil if the import specification is malformed or invalid.
func parseGoImportSpec(
	parseTree *valueobject.ParseTree,
	importSpec *valueobject.ParseNode,
) *outbound.ImportDeclaration {
	// Validate input parameters
	if parseTree == nil || importSpec == nil {
		return nil
	}

	// Find import path using tree utilities
	pathNodes := FindDirectChildren(importSpec, "interpreted_string_literal")
	if len(pathNodes) == 0 {
		// No path found - this is a malformed import spec
		return nil
	}

	// Validate that we have a valid path node
	pathNode := pathNodes[0]
	if pathNode == nil {
		return nil
	}

	// Find alias using centralized logic
	alias := extractImportAliasFromNode(parseTree, importSpec)

	// Create and validate the import declaration
	importDecl := createImportFromPath(parseTree, importSpec, pathNode, alias)
	if importDecl == nil {
		// createImportFromPath handles its own validation and logging
		return nil
	}

	return importDecl
}

// extractImportAliasFromNode extracts alias information from an import node.
// This function handles dot imports, blank imports, and regular package identifiers.
// It centralizes the alias detection logic that was duplicated across multiple functions.
// Returns empty string if no alias is found or if input is invalid.
func extractImportAliasFromNode(
	parseTree *valueobject.ParseTree,
	importNode *valueobject.ParseNode,
) string {
	// Validate input parameters
	if parseTree == nil || importNode == nil {
		return ""
	}

	// First check for package_identifier (named import)
	identifierNodes := FindDirectChildren(importNode, "package_identifier")
	if len(identifierNodes) > 0 && identifierNodes[0] != nil {
		alias := parseTree.GetNodeText(identifierNodes[0])
		// Validate that the alias is not empty
		if alias != "" {
			return alias
		}
	}

	// Check for blank_identifier node type specifically
	blankNodes := FindDirectChildren(importNode, "blank_identifier")
	if len(blankNodes) > 0 {
		return "_"
	}

	// Check for dot or blank import by examining child node text
	for _, child := range importNode.Children {
		if child == nil {
			continue
		}
		nodeText := parseTree.GetNodeText(child)
		switch nodeText {
		case ".":
			return "."
		case "_":
			return "_"
		}
	}

	// No alias found
	return ""
}

// extractImportPath extracts and cleans the import path from a path node.
// This function centralizes path extraction and cleaning logic with validation.
// Returns empty string if the path is invalid or malformed.
func extractImportPath(
	parseTree *valueobject.ParseTree,
	pathNode *valueobject.ParseNode,
) string {
	// Validate input parameters
	if parseTree == nil || pathNode == nil {
		return ""
	}

	// Extract the raw path text
	rawPath := parseTree.GetNodeText(pathNode)
	if rawPath == "" {
		return ""
	}

	// Remove quotes from import path
	path := strings.Trim(rawPath, "\"'`")

	// Validate that the cleaned path is not empty
	// An import path should not be empty after removing quotes
	if path == "" {
		return ""
	}

	return path
}

// createImportFromPath creates an ImportDeclaration from path and alias.
// Returns nil if the import cannot be created due to invalid input or malformed data.
func createImportFromPath(
	parseTree *valueobject.ParseTree,
	importDecl *valueobject.ParseNode,
	pathNode *valueobject.ParseNode,
	alias string,
) *outbound.ImportDeclaration {
	// Validate input parameters
	if parseTree == nil || importDecl == nil || pathNode == nil {
		return nil
	}

	// Extract and validate the import path
	path := extractImportPath(parseTree, pathNode)
	if path == "" {
		// Empty path indicates a malformed import
		return nil
	}

	// Validate node position information
	if importDecl.StartByte > importDecl.EndByte {
		// Invalid position information - this shouldn't happen with tree-sitter
		return nil
	}

	isWildcard := alias == "."
	content := parseTree.GetNodeText(importDecl)

	// Ensure content is not empty
	if content == "" {
		return nil
	}

	// Create the import declaration with all required metadata
	return &outbound.ImportDeclaration{
		Path:        path,
		Alias:       alias,
		IsWildcard:  isWildcard,
		Content:     content,
		StartByte:   importDecl.StartByte,
		EndByte:     importDecl.EndByte,
		ExtractedAt: time.Now(),
		Hash:        utils.GenerateHash(content),
		Metadata: map[string]interface{}{
			"import_type": determineImportType(alias),
			"node_type":   importDecl.Type,
		},
	}
}

// determineImportType returns a descriptive string for the type of import.
func determineImportType(alias string) string {
	switch alias {
	case "":
		return "standard"
	case "_":
		return "blank"
	case ".":
		return "dot"
	default:
		return "aliased"
	}
}
