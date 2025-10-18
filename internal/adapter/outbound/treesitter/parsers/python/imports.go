// Package pythonparser provides Python-specific import extraction functionality for tree-sitter parsing.
//
// This file implements comprehensive import declaration extraction from Python source code
// using tree-sitter's Abstract Syntax Tree (AST) parsing capabilities. It handles all
// Python import patterns including:
//
// 1. Standard imports: import module, import module as alias
// 2. From imports: from module import name, from module import name as alias
// 3. Relative imports: from .module import name, from ..parent import name
// 4. Wildcard imports: from module import *
// 5. Multi-name imports: from module import name1, name2, name3
//
// The implementation uses tree-sitter's AST structure to parse different import forms:
// - extractStandardImport: Handles "import module" and "import module as alias" statements
// - extractFromImport: Handles "from module import name" statements with full support for relative imports
// - extractAliasedImportDecl: Extracts alias information from aliased_import nodes
//
// Tree-sitter Python Grammar Structure:
//
// import_statement:
//   - Children: "import", dotted_name OR "import", aliased_import
//   - Examples: "import os", "import numpy as np"
//
// import_from_statement:
//   - Children: "from", dotted_name|relative_import, "import", dotted_name|aliased_import|wildcard_import
//   - Examples: "from os import path", "from . import utils", "from module import *"
//
// relative_import:
//   - Text contains leading dots indicating relative level: ".module" or "..parent"
//
// Key features:
// - Robust metadata tracking for relative imports (level tracking)
// - Proper handling of wildcard imports
// - Support for multi-name imports in from statements
// - Clean separation of concerns between import types
package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
)

// Tree-sitter node types for Python imports from the Python grammar.
const (
	nodeTypeDottedName     = "dotted_name"
	nodeTypeAliasedImport  = "aliased_import"
	nodeTypeRelativeImport = "relative_import"
	nodeTypeWildcardImport = "wildcard_import"
)

// extractPythonImports extracts Python imports from the parse tree.
//
// This is the main entry point for Python import extraction. It handles two distinct
// import statement types defined in the tree-sitter Python grammar:
//
// 1. import_statement: Standard imports like "import os" or "import numpy as np"
// 2. import_from_statement: From imports like "from os import path" or "from . import utils"
//
// The function delegates to specialized extractors for each import type, ensuring proper
// handling of aliases, relative imports, and wildcard imports.
//
// Parameters:
//   - ctx: Context for logging and cancellation (currently unused but available for future use)
//   - parseTree: The tree-sitter parse tree containing the Python source code
//   - options: Semantic extraction options (currently unused)
//
// Returns:
//   - []outbound.ImportDeclaration: All import declarations found in the source
//   - error: Always nil in current implementation (error return maintained for interface compatibility)
func extractPythonImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	var imports []outbound.ImportDeclaration

	// Extract standard imports (import module)
	importNodes := parseTree.GetNodesByType("import_statement")
	for _, node := range importNodes {
		importDecls := extractStandardImport(parseTree, node)
		imports = append(imports, importDecls...)
	}

	// Extract from imports (from module import name)
	fromImportNodes := parseTree.GetNodesByType("import_from_statement")
	for _, node := range fromImportNodes {
		importDecl := extractFromImport(parseTree, node)
		if importDecl != nil {
			imports = append(imports, *importDecl)
		}
	}

	return imports, nil
}

// extractStandardImport extracts imports from "import module" statements.
//
// This function handles the tree-sitter Python grammar structure for import_statement nodes:
//
// import_statement structure:
//   - Children: "import" (keyword), dotted_name OR aliased_import
//
// Examples:
//   - "import os" -> Children: ["import", dotted_name("os")]
//   - "import numpy as np" -> Children: ["import", aliased_import]
//   - "import os, sys" -> Multiple dotted_name children (though this is handled by tree-sitter as separate import statements)
//
// The function iterates through child nodes and:
//  1. For dotted_name nodes: Creates a simple import with just the path
//  2. For aliased_import nodes: Delegates to extractAliasedImportDecl for alias extraction
//
// Parameters:
//   - parseTree: The parse tree for extracting node text
//   - node: The import_statement node to process
//
// Returns:
//   - []outbound.ImportDeclaration: All imports found (may be multiple for comma-separated imports)
func extractStandardImport(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	if node == nil {
		return imports
	}

	// Children are directly: import, dotted_name OR import, aliased_import
	for _, child := range node.Children {
		switch child.Type {
		case nodeTypeDottedName:
			moduleName := parseTree.GetNodeText(child)
			imports = append(imports, outbound.ImportDeclaration{
				Path: moduleName,
			})
		case nodeTypeAliasedImport:
			importDecl := extractAliasedImportDecl(parseTree, child)
			if importDecl != nil {
				imports = append(imports, *importDecl)
			}
		}
	}

	return imports
}

// extractAliasedImportDecl extracts "module as alias" from aliased_import node.
//
// This function handles the tree-sitter Python grammar structure for aliased_import nodes:
//
// aliased_import structure:
//   - Children: dotted_name (module), "as" (keyword), identifier (alias name)
//
// Examples:
//   - "numpy as np" -> Children: [dotted_name("numpy"), "as", identifier("np")]
//   - "os.path as ospath" -> Children: [dotted_name("os.path"), "as", identifier("ospath")]
//
// The function extracts both the module name and the alias by iterating through children
// and identifying nodes by type.
//
// Parameters:
//   - parseTree: The parse tree for extracting node text
//   - node: The aliased_import node to process
//
// Returns:
//   - *outbound.ImportDeclaration: Import declaration with Path and Alias set, or nil if node is nil
func extractAliasedImportDecl(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) *outbound.ImportDeclaration {
	if node == nil {
		return nil
	}

	var moduleName, alias string

	// Children are: dotted_name, "as", identifier
	for _, child := range node.Children {
		switch child.Type {
		case nodeTypeDottedName:
			moduleName = parseTree.GetNodeText(child)
		case nodeTypeIdentifier:
			alias = parseTree.GetNodeText(child)
		}
	}

	return &outbound.ImportDeclaration{
		Path:  moduleName,
		Alias: alias,
	}
}

// extractFromImport extracts imports from "from module import name" statements.
//
// This function handles the tree-sitter Python grammar structure for import_from_statement nodes:
//
// import_from_statement structure:
//   - Children (in order): "from" (keyword), dotted_name|relative_import, "import" (keyword), imported_names
//   - imported_names can be: dotted_name, aliased_import, wildcard_import, or comma-separated combinations
//
// Examples:
//   - "from os import path" -> ["from", dotted_name("os"), "import", dotted_name("path")]
//   - "from . import utils" -> ["from", relative_import("."), "import", dotted_name("utils")]
//   - "from ..parent import func1, func2" -> ["from", relative_import("..parent"), "import", dotted_name("func1"), comma, dotted_name("func2")]
//   - "from module import *" -> ["from", dotted_name("module"), "import", wildcard_import("*")]
//   - "from pkg import name as alias" -> ["from", dotted_name("pkg"), "import", aliased_import]
//
// The function uses state tracking (foundFrom, foundImport) to correctly distinguish between:
//  1. The module name (appears between "from" and "import")
//  2. The imported symbol names (appear after "import")
//
// For relative imports, it extracts the relative level (number of dots) and stores it in metadata.
//
// Parameters:
//   - parseTree: The parse tree for extracting node text
//   - node: The import_from_statement node to process
//
// Returns:
//   - *outbound.ImportDeclaration: Import with Path, ImportedSymbols, IsWildcard, and metadata, or nil if node is nil
func extractFromImport(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) *outbound.ImportDeclaration {
	if node == nil {
		return nil
	}

	var moduleName string
	var importedSymbols []string
	var isRelative bool
	var relativeLevel int
	foundFrom := false
	foundImport := false

	// Children are: from, dotted_name (module), import, dotted_name (symbol1), comma, dotted_name (symbol2), ...
	// Or: from, relative_import (.module or ..module), import, dotted_name, ...
	for _, child := range node.Children {
		switch child.Type {
		case "from":
			foundFrom = true
		case "import":
			foundImport = true
		case nodeTypeDottedName:
			if foundFrom && !foundImport {
				// First dotted_name after "from" is the module name
				moduleName = parseTree.GetNodeText(child)
			} else if foundImport {
				// dotted_name after "import" are imported symbols
				importedSymbols = append(importedSymbols, parseTree.GetNodeText(child))
			}
		case nodeTypeRelativeImport:
			if foundFrom && !foundImport {
				// Relative import like ".local_module" or "..parent_module"
				relativeText := parseTree.GetNodeText(child)
				isRelative = true
				relativeLevel = strings.Count(relativeText, ".")
				// Remove leading dots to get module name
				moduleName = strings.TrimLeft(relativeText, ".")
			}
		case nodeTypeAliasedImport:
			if foundImport {
				// Handle "from module import name as alias"
				symbol := parseTree.GetNodeText(child)
				importedSymbols = append(importedSymbols, symbol)
			}
		case nodeTypeWildcardImport:
			return &outbound.ImportDeclaration{
				Path:       moduleName,
				IsWildcard: true,
				Metadata:   createRelativeImportMetadata(isRelative, relativeLevel),
			}
		}
	}

	return &outbound.ImportDeclaration{
		Path:            moduleName,
		ImportedSymbols: importedSymbols,
		Metadata:        createRelativeImportMetadata(isRelative, relativeLevel),
	}
}

// createRelativeImportMetadata creates metadata map for relative imports.
// This centralizes the metadata creation logic to avoid duplication.
//
// Parameters:
//   - isRelative: Whether the import is relative
//   - relativeLevel: The number of dots in the relative import (0 for non-relative)
//
// Returns:
//   - map[string]interface{}: Metadata map with is_relative and relative_level, or nil if not relative
func createRelativeImportMetadata(isRelative bool, relativeLevel int) map[string]interface{} {
	if !isRelative {
		return nil
	}

	return map[string]interface{}{
		"is_relative":    true,
		"relative_level": relativeLevel,
	}
}

// isRelativeImport determines if import is relative by checking for leading dots.
// This is a helper function used for validation and debugging.
//
// Note: In current implementation, we detect relative imports through tree-sitter's
// relative_import node type, making this function primarily useful for validation.
func isRelativeImport(importText string) bool {
	return strings.HasPrefix(importText, ".")
}
