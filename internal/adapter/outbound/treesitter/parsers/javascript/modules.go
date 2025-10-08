package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// extractJavaScriptModules extracts JavaScript export declarations from the parse tree.
// Returns individual SemanticCodeChunk for each export (not a single aggregated module).
func extractJavaScriptModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var chunks []outbound.SemanticCodeChunk
	now := time.Now()

	// Extract ES6 exports
	es6Exports := extractES6Exports(parseTree, now)
	chunks = append(chunks, es6Exports...)

	// Extract CommonJS exports (module.exports, exports.x)
	commonJSExports := extractCommonJSExports(parseTree, now)
	chunks = append(chunks, commonJSExports...)

	// Extract AMD exports (define() return values)
	amdExports := extractAMDExports(parseTree, now)
	chunks = append(chunks, amdExports...)

	return chunks, nil
}

// extractES6Exports extracts ES6 export statements and returns individual chunks.
func extractES6Exports(parseTree *valueobject.ParseTree, now time.Time) []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Find all export_statement nodes
	exportNodes := parseTree.GetNodesByType("export_statement")
	for _, node := range exportNodes {
		content := parseTree.GetNodeText(node)

		// Check if this is a default export
		if strings.Contains(content, "default") {
			// Extract default export
			chunk := extractDefaultExport(parseTree, node, content, now)
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
			continue
		}

		// Check for export clause (e.g., export { name1, name2 })
		exportClause := findChildByType(node, "export_clause")
		if exportClause != nil {
			// Extract named exports from clause
			namedChunks := extractNamedExportsFromClause(parseTree, node, exportClause, now)
			chunks = append(chunks, namedChunks...)
			continue
		}

		// Check for declaration export (e.g., export const x = 1)
		declarationNode := findChildByType(node, "lexical_declaration")
		if declarationNode == nil {
			declarationNode = findChildByType(node, "function_declaration")
		}
		if declarationNode == nil {
			declarationNode = findChildByType(node, "class_declaration")
		}
		if declarationNode != nil {
			// Extract declaration export
			chunk := extractDeclarationExport(parseTree, node, declarationNode, now)
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}
	}

	return chunks
}

// extractDefaultExport extracts a default export and returns a chunk.
func extractDefaultExport(
	parseTree *valueobject.ParseTree,
	exportNode *valueobject.ParseNode,
	content string,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Try to extract the name from the exported declaration
	var exportName string

	// Check for named function: export default function foo() {}
	funcDecl := findChildByType(exportNode, "function_declaration")
	if funcDecl != nil {
		nameNode := findChildByType(funcDecl, "identifier")
		if nameNode != nil {
			exportName = parseTree.GetNodeText(nameNode)
		}
	}

	// Check for named class: export default class Foo {}
	if exportName == "" {
		classDecl := findChildByType(exportNode, "class_declaration")
		if classDecl != nil {
			nameNode := findChildByType(classDecl, "identifier")
			if nameNode != nil {
				exportName = parseTree.GetNodeText(nameNode)
			}
		}
	}

	// If still no name, use "default" as the name
	if exportName == "" {
		exportName = "default"
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructModule), exportName, nil),
		Type:          outbound.ConstructModule,
		Name:          exportName,
		QualifiedName: exportName,
		Language:      parseTree.Language(),
		StartByte:     exportNode.StartByte,
		EndByte:       exportNode.EndByte,
		StartPosition: valueobject.Position{Row: exportNode.StartPos.Row, Column: exportNode.StartPos.Column},
		EndPosition:   valueobject.Position{Row: exportNode.EndPos.Row, Column: exportNode.EndPos.Column},
		Content:       content,
		Visibility:    outbound.Public,
		Metadata: map[string]interface{}{
			"export_type": "default",
			"is_default":  true,
		},
		ExtractedAt: now,
		Hash:        utils.GenerateHash(content),
	}
}

// extractNamedExportsFromClause extracts individual named exports from an export clause.
func extractNamedExportsFromClause(
	parseTree *valueobject.ParseTree,
	exportNode *valueobject.ParseNode,
	exportClause *valueobject.ParseNode,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Find all export_specifier nodes
	specifiers := findChildrenByType(exportClause, "export_specifier")
	for _, specifier := range specifiers {
		specifierText := parseTree.GetNodeText(specifier)
		var exportName, originalName string

		// Check for aliased export (e.g., "name as alias")
		if strings.Contains(specifierText, " as ") {
			parts := strings.Split(specifierText, " as ")
			if len(parts) == 2 {
				originalName = strings.TrimSpace(parts[0])
				exportName = strings.TrimSpace(parts[1])
			}
		} else {
			// Simple export (e.g., "name")
			exportName = strings.TrimSpace(specifierText)
			originalName = exportName
		}

		if exportName == "" {
			continue
		}

		chunk := outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID(string(outbound.ConstructModule), exportName, nil),
			Type:          outbound.ConstructModule,
			Name:          exportName,
			QualifiedName: exportName,
			Language:      parseTree.Language(),
			StartByte:     exportNode.StartByte,
			EndByte:       exportNode.EndByte,
			StartPosition: valueobject.Position{Row: exportNode.StartPos.Row, Column: exportNode.StartPos.Column},
			EndPosition:   valueobject.Position{Row: exportNode.EndPos.Row, Column: exportNode.EndPos.Column},
			Content:       parseTree.GetNodeText(exportNode),
			Visibility:    outbound.Public,
			Metadata: map[string]interface{}{
				"export_type":   "named",
				"is_default":    false,
				"is_alias":      originalName != exportName,
				"original_name": originalName,
			},
			ExtractedAt: now,
			Hash:        utils.GenerateHash(parseTree.GetNodeText(exportNode)),
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

// extractDeclarationExport extracts an export with inline declaration.
func extractDeclarationExport(
	parseTree *valueobject.ParseTree,
	exportNode *valueobject.ParseNode,
	declarationNode *valueobject.ParseNode,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Extract the name from the declaration
	var exportName string

	// Try to find identifier in the declaration
	nameNode := findChildByType(declarationNode, "identifier")
	if nameNode != nil {
		exportName = parseTree.GetNodeText(nameNode)
	} else {
		// For variable declarations, look for variable_declarator
		declarator := findChildByType(declarationNode, "variable_declarator")
		if declarator != nil {
			nameNode = findChildByType(declarator, "identifier")
			if nameNode != nil {
				exportName = parseTree.GetNodeText(nameNode)
			}
		}
	}

	if exportName == "" {
		return nil
	}

	content := parseTree.GetNodeText(exportNode)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructModule), exportName, nil),
		Type:          outbound.ConstructModule,
		Name:          exportName,
		QualifiedName: exportName,
		Language:      parseTree.Language(),
		StartByte:     exportNode.StartByte,
		EndByte:       exportNode.EndByte,
		StartPosition: valueobject.Position{Row: exportNode.StartPos.Row, Column: exportNode.StartPos.Column},
		EndPosition:   valueobject.Position{Row: exportNode.EndPos.Row, Column: exportNode.EndPos.Column},
		Content:       content,
		Visibility:    outbound.Public,
		Metadata: map[string]interface{}{
			"export_type": "declaration",
			"is_default":  false,
		},
		ExtractedAt: now,
		Hash:        utils.GenerateHash(content),
	}
}

// extractCommonJSExports extracts CommonJS module.exports and exports.x assignments.
func extractCommonJSExports(parseTree *valueobject.ParseTree, now time.Time) []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Find all assignment_expression nodes
	assignmentNodes := parseTree.GetNodesByType("expression_statement")
	for _, stmt := range assignmentNodes {
		assignmentExpr := findChildByType(stmt, "assignment_expression")
		if assignmentExpr == nil {
			continue
		}

		// Check the left side for module.exports or exports.x
		leftSide := findChildByType(assignmentExpr, "member_expression")
		if leftSide == nil {
			// Check if it's a direct identifier assignment
			leftSide = findChildByType(assignmentExpr, "identifier")
			if leftSide == nil {
				continue
			}
		}

		leftText := parseTree.GetNodeText(leftSide)

		// Check for module.exports or exports.x
		var exportName string
		switch {
		case strings.HasPrefix(leftText, "module.exports"):
			// Extract property name if present (e.g., module.exports.foo)
			parts := strings.Split(leftText, ".")
			if len(parts) == 3 {
				exportName = parts[2]
			} else {
				// module.exports = ...
				// Try to extract name from the right side (could be function_expression, function_declaration, class_expression, etc.)
				// Look for function_expression first (most common in CommonJS)
				rightSide := findChildByType(assignmentExpr, "function_expression")
				if rightSide == nil {
					rightSide = findChildByType(assignmentExpr, "function_declaration")
				}
				if rightSide == nil {
					rightSide = findChildByType(assignmentExpr, "class_expression")
				}
				if rightSide == nil {
					rightSide = findChildByType(assignmentExpr, "class_declaration")
				}

				if rightSide != nil {
					nameNode := findChildByType(rightSide, "identifier")
					if nameNode != nil {
						exportName = parseTree.GetNodeText(nameNode)
					}
				}

				// If still no name, use the default "module.exports"
				if exportName == "" {
					exportName = "module.exports"
				}
			}
		case strings.HasPrefix(leftText, "exports."):
			// exports.foo = ...
			parts := strings.Split(leftText, ".")
			if len(parts) >= 2 {
				exportName = parts[1]
			}
		default:
			continue
		}

		if exportName == "" {
			continue
		}

		content := parseTree.GetNodeText(stmt)

		chunk := outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID(string(outbound.ConstructModule), exportName, nil),
			Type:          outbound.ConstructModule,
			Name:          exportName,
			QualifiedName: exportName,
			Language:      parseTree.Language(),
			StartByte:     stmt.StartByte,
			EndByte:       stmt.EndByte,
			StartPosition: valueobject.Position{Row: stmt.StartPos.Row, Column: stmt.StartPos.Column},
			EndPosition:   valueobject.Position{Row: stmt.EndPos.Row, Column: stmt.EndPos.Column},
			Content:       content,
			Visibility:    outbound.Public,
			Metadata: map[string]interface{}{
				"export_type": "commonjs",
				"is_default":  leftText == "module.exports",
			},
			ExtractedAt: now,
			Hash:        utils.GenerateHash(content),
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

// extractAMDExports extracts AMD define() exports.
func extractAMDExports(parseTree *valueobject.ParseTree, now time.Time) []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Find all call_expression nodes
	callNodes := parseTree.GetNodesByType("call_expression")
	for _, node := range callNodes {
		// Check if the function being called is "define"
		functionNode := findChildByType(node, "identifier")
		if functionNode == nil {
			continue
		}

		functionName := parseTree.GetNodeText(functionNode)
		if functionName != "define" {
			continue
		}

		// AMD define() exports an anonymous module
		content := parseTree.GetNodeText(node)

		chunk := outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID(string(outbound.ConstructModule), "amd_module", nil),
			Type:          outbound.ConstructModule,
			Name:          "", // AMD modules are typically anonymous
			QualifiedName: "",
			Language:      parseTree.Language(),
			StartByte:     node.StartByte,
			EndByte:       node.EndByte,
			StartPosition: valueobject.Position{Row: node.StartPos.Row, Column: node.StartPos.Column},
			EndPosition:   valueobject.Position{Row: node.EndPos.Row, Column: node.EndPos.Column},
			Content:       content,
			Visibility:    outbound.Public,
			Metadata: map[string]interface{}{
				"export_type": "amd",
				"is_default":  false,
			},
			ExtractedAt: now,
			Hash:        utils.GenerateHash(content),
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}
