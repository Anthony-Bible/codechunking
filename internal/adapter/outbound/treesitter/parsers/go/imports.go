package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// ExtractImports extracts import declarations from a Go parse tree.
func (p *GoParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	slogger.Info(ctx, "Extracting Go imports", slogger.Fields{})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	var imports []outbound.ImportDeclaration

	// Find import declarations
	importNodes := parseTree.GetNodesByType("import_declaration")
	for _, node := range importNodes {
		importDecls := parseGoImportDeclaration(parseTree, node)
		imports = append(imports, importDecls...)
	}

	return imports, nil
}

// parseGoImportDeclaration parses an import declaration.
func parseGoImportDeclaration(
	parseTree *valueobject.ParseTree,
	importDecl *valueobject.ParseNode,
) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	// Find import specs
	importSpecs := findChildrenByType(importDecl, "import_spec")
	for _, importSpec := range importSpecs {
		importDeclaration := parseGoImportSpec(parseTree, importSpec)
		if importDeclaration != nil {
			imports = append(imports, *importDeclaration)
		}
	}

	// Handle single import without parentheses
	if len(importSpecs) == 0 {
		pathNode := findChildByTypeInNode(importDecl, "interpreted_string_literal")
		if pathNode != nil {
			importDeclaration := createImportFromPath(parseTree, importDecl, pathNode, "")
			if importDeclaration != nil {
				imports = append(imports, *importDeclaration)
			}
		}
	}

	return imports
}

// parseGoImportSpec parses an import specification.
func parseGoImportSpec(
	parseTree *valueobject.ParseTree,
	importSpec *valueobject.ParseNode,
) *outbound.ImportDeclaration {
	// Find import path
	pathNode := findChildByTypeInNode(importSpec, "interpreted_string_literal")
	if pathNode == nil {
		return nil
	}

	// Find alias (if any)
	alias := ""
	identifierNode := findChildByTypeInNode(importSpec, "package_identifier")
	if identifierNode != nil {
		alias = parseTree.GetNodeText(identifierNode)
	} else {
		// Check for dot import
		if dotNode := findChildByTypeInNode(importSpec, "."); dotNode != nil {
			alias = "."
		}
		// Check for blank import
		if blankNode := findChildByTypeInNode(importSpec, "_"); blankNode != nil {
			alias = "_"
		}
	}

	return createImportFromPath(parseTree, importSpec, pathNode, alias)
}

// createImportFromPath creates an ImportDeclaration from path and alias.
func createImportFromPath(
	parseTree *valueobject.ParseTree,
	importDecl *valueobject.ParseNode,
	pathNode *valueobject.ParseNode,
	alias string,
) *outbound.ImportDeclaration {
	path := parseTree.GetNodeText(pathNode)
	// Remove quotes
	path = strings.Trim(path, "\"'`")

	isWildcard := alias == "."
	content := parseTree.GetNodeText(importDecl)

	return &outbound.ImportDeclaration{
		Path:        path,
		Alias:       alias,
		IsWildcard:  isWildcard,
		Content:     content,
		StartByte:   importDecl.StartByte,
		EndByte:     importDecl.EndByte,
		ExtractedAt: time.Now(),
		Hash:        utils.GenerateHash(content),
		Metadata:    map[string]interface{}{},
	}
}
