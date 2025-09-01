package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
)

// extractPythonImports extracts Python imports from the parse tree.
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
func extractStandardImport(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	if node == nil {
		return imports
	}

	// Find import list
	for _, child := range node.Children {
		if child.Type == "dotted_as_names" || child.Type == "dotted_name" {
			moduleImports := extractModuleNames(parseTree, child)
			imports = append(imports, moduleImports...)
		}
	}

	return imports
}

// extractFromImport extracts imports from "from module import name" statements.
func extractFromImport(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) *outbound.ImportDeclaration {
	if node == nil {
		return nil
	}

	var moduleName string
	var importedSymbols []string
	var isRelative bool
	var relativeLevel int
	var isWildcard bool

	// Parse the import statement components
	for _, child := range node.Children {
		switch child.Type {
		case "dotted_name":
			moduleName = parseTree.GetNodeText(child)
		case "relative_import":
			// Handle relative imports like "from .module import name"
			relativeImportText := parseTree.GetNodeText(child)
			isRelative = true
			relativeLevel = strings.Count(relativeImportText, ".")
			// Extract module name after dots
			parts := strings.Split(relativeImportText, ".")
			if len(parts) > 1 && parts[len(parts)-1] != "" {
				moduleName = parts[len(parts)-1]
			}
		case "import_list":
			symbols := extractImportList(parseTree, child)
			importedSymbols = append(importedSymbols, symbols...)
		case "wildcard_import":
			isWildcard = true
		}
	}

	// Handle case where module name comes from dotted name in relative import
	if isRelative && moduleName == "" {
		for _, child := range node.Children {
			if child.Type == "dotted_name" {
				moduleName = parseTree.GetNodeText(child)
				break
			}
		}
	}

	// Create metadata for relative imports
	metadata := make(map[string]interface{})
	if isRelative {
		metadata["is_relative"] = true
		metadata["relative_level"] = relativeLevel
	}

	return &outbound.ImportDeclaration{
		Path:            moduleName,
		ImportedSymbols: importedSymbols,
		IsWildcard:      isWildcard,
		Metadata:        metadata,
	}
}

// extractModuleNames extracts module names from dotted names with optional aliases.
func extractModuleNames(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	if node == nil {
		return imports
	}

	// Handle single dotted name
	if node.Type == "dotted_name" {
		moduleName := parseTree.GetNodeText(node)
		imports = append(imports, outbound.ImportDeclaration{
			Path: moduleName,
		})
		return imports
	}

	// Handle multiple imports with possible aliases
	for _, child := range node.Children {
		switch child.Type {
		case "dotted_as_name":
			importDecl := extractDottedAsName(parseTree, child)
			if importDecl != nil {
				imports = append(imports, *importDecl)
			}
		case "dotted_name":
			moduleName := parseTree.GetNodeText(child)
			imports = append(imports, outbound.ImportDeclaration{
				Path: moduleName,
			})
		}
	}

	return imports
}

// extractDottedAsName extracts "module as alias" imports.
func extractDottedAsName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) *outbound.ImportDeclaration {
	if node == nil {
		return nil
	}

	var moduleName, alias string

	for _, child := range node.Children {
		switch child.Type {
		case "dotted_name":
			moduleName = parseTree.GetNodeText(child)
		case "identifier":
			// The identifier after "as" is the alias
			alias = parseTree.GetNodeText(child)
		}
	}

	return &outbound.ImportDeclaration{
		Path:  moduleName,
		Alias: alias,
	}
}

// extractImportList extracts names from import list (the part after "import").
func extractImportList(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []string {
	var symbols []string

	if node == nil {
		return symbols
	}

	for _, child := range node.Children {
		switch child.Type {
		case "identifier":
			symbols = append(symbols, parseTree.GetNodeText(child))
		case "aliased_import":
			symbol := extractAliasedImport(parseTree, child)
			if symbol != "" {
				symbols = append(symbols, symbol)
			}
		}
	}

	return symbols
}

// extractAliasedImport extracts "name as alias" from import list.
func extractAliasedImport(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if node == nil {
		return ""
	}

	// For now, just return the original name
	// A more complete implementation would track aliases
	for _, child := range node.Children {
		if child.Type == "identifier" {
			return parseTree.GetNodeText(child)
		}
	}

	return ""
}

// extractRelativeImportPath extracts the path from a relative import.
func extractRelativeImportPath(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) (string, int) {
	if node == nil {
		return "", 0
	}

	text := parseTree.GetNodeText(node)
	level := strings.Count(text, ".")

	// Remove leading dots to get module name
	moduleName := strings.TrimLeft(text, ".")

	return moduleName, level
}

// Helper function to determine if import is relative.
func isRelativeImport(importText string) bool {
	return strings.HasPrefix(importText, ".")
}

// Helper function to count relative import levels.
func getRelativeImportLevel(importText string) int {
	count := 0
	for _, char := range importText {
		if char == '.' {
			count++
		} else {
			break
		}
	}
	return count
}
