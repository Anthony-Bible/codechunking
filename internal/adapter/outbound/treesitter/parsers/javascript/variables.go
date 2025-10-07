package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

const (
	declarationTypeConst = "const"
	nodeTypeRestPattern  = "rest_pattern"
	nodeTypePropertyID   = "property_identifier"
)

func extractJavaScriptVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var variables []outbound.SemanticCodeChunk
	moduleName := "main"

	// Build namespace import symbol table for detecting namespace import variable assignments
	namespaceImports := collectNamespaceImports(ctx, parseTree)

	// Find all variable declarations (var)
	varDeclarations := parseTree.GetNodesByType("variable_declaration")
	for _, node := range varDeclarations {
		declarators := findChildrenByType(node, "variable_declarator")
		for _, declarator := range declarators {
			// Get the pattern (first child, which could be identifier or a destructuring pattern)
			if len(declarator.Children) == 0 {
				continue
			}

			pattern := declarator.Children[0]
			extractedVars := extractVariablesFromPattern(pattern, parseTree)

			for _, varInfo := range extractedVars {
				// Build metadata starting with var-specific info
				metadata := map[string]interface{}{
					"declaration_type": "var",
					"scope":            "global",
				}

				// Merge pattern-specific metadata
				for k, v := range varInfo.metadata {
					metadata[k] = v
				}

				chunk := outbound.SemanticCodeChunk{
					ChunkID:       utils.GenerateID(string(outbound.ConstructVariable), varInfo.name, nil),
					Type:          outbound.ConstructVariable,
					Name:          varInfo.name,
					QualifiedName: moduleName + "." + varInfo.name,
					Language:      parseTree.Language(),
					StartByte:     varInfo.node.StartByte,
					EndByte:       varInfo.node.EndByte,
					StartPosition: valueobject.Position{
						Row:    varInfo.node.StartPos.Row,
						Column: varInfo.node.StartPos.Column,
					},
					EndPosition: valueobject.Position{
						Row:    varInfo.node.EndPos.Row,
						Column: varInfo.node.EndPos.Column,
					},
					Content:     parseTree.GetNodeText(declarator),
					Metadata:    metadata,
					Annotations: []outbound.Annotation{},
					ExtractedAt: time.Now(),
					Hash:        utils.GenerateHash(varInfo.name),
				}
				variables = append(variables, chunk)
			}
		}
	}

	// Find all lexical declarations (let/const)
	lexicalDeclarations := parseTree.GetNodesByType("lexical_declaration")
	for _, node := range lexicalDeclarations {
		// Determine if it's a let or const declaration
		declarationType := "let"
		if len(node.Children) > 0 && parseTree.GetNodeText(node.Children[0]) == declarationTypeConst {
			declarationType = declarationTypeConst
		}

		chunkType := outbound.ConstructVariable
		if declarationType == declarationTypeConst {
			chunkType = outbound.ConstructConstant
		}

		declarators := findChildrenByType(node, "variable_declarator")
		for _, declarator := range declarators {
			// Get the pattern (first child, which could be identifier or a destructuring pattern)
			if len(declarator.Children) == 0 {
				continue
			}

			pattern := declarator.Children[0]
			extractedVars := extractVariablesFromPattern(pattern, parseTree)

			for _, varInfo := range extractedVars {
				// Build metadata with ES6 feature detection
				metadata := map[string]interface{}{
					"declaration_type": declarationType,
					"scope":            "block",
				}

				// Analyze the declarator value to detect ES6 features
				analyzeVariableValue(declarator, parseTree, metadata)

				// Check if this variable is assigned from a namespace import
				if detectNamespaceImportAssignment(declarator, parseTree, namespaceImports) {
					metadata["is_namespace_import"] = true
				}

				// Merge pattern-specific metadata
				for k, v := range varInfo.metadata {
					metadata[k] = v
				}

				chunk := outbound.SemanticCodeChunk{
					ChunkID:       utils.GenerateID(string(chunkType), varInfo.name, nil),
					Type:          chunkType,
					Name:          varInfo.name,
					QualifiedName: moduleName + "." + varInfo.name,
					Language:      parseTree.Language(),
					StartByte:     varInfo.node.StartByte,
					EndByte:       varInfo.node.EndByte,
					StartPosition: valueobject.Position{
						Row:    varInfo.node.StartPos.Row,
						Column: varInfo.node.StartPos.Column,
					},
					EndPosition: valueobject.Position{
						Row:    varInfo.node.EndPos.Row,
						Column: varInfo.node.EndPos.Column,
					},
					Content:     parseTree.GetNodeText(declarator),
					Metadata:    metadata,
					ReturnType:  inferReturnType(declarator, parseTree),
					Annotations: []outbound.Annotation{},
					ExtractedAt: time.Now(),
					Hash:        utils.GenerateHash(varInfo.name),
				}
				variables = append(variables, chunk)
			}
		}
	}

	return variables, nil
}

func extractJavaScriptInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var interfaces []outbound.SemanticCodeChunk

	moduleName := "main"

	interfaceDeclarations := parseTree.GetNodesByType("interface_declaration")
	for _, node := range interfaceDeclarations {
		nameNodes := findChildrenByType(node, "type_identifier")
		if len(nameNodes) == 0 {
			continue
		}
		nameNode := nameNodes[0]

		interfaceName := parseTree.GetNodeText(nameNode)
		startByte := nameNode.StartByte
		endByte := nameNode.EndByte
		startPos := nameNode.StartPos
		endPos := nameNode.EndPos

		chunk := outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID(string(outbound.ConstructInterface), interfaceName, nil),
			Type:          outbound.ConstructInterface,
			Name:          interfaceName,
			QualifiedName: moduleName + "." + interfaceName,
			Language:      parseTree.Language(),
			StartByte:     startByte,
			EndByte:       endByte,
			StartPosition: valueobject.Position{Row: startPos.Row, Column: startPos.Column},
			EndPosition:   valueobject.Position{Row: endPos.Row, Column: endPos.Column},
			Content:       parseTree.GetNodeText(node),
			Metadata: map[string]interface{}{
				"declaration_type": "interface",
				"scope":            "global",
			},
			Annotations: []outbound.Annotation{},
			ExtractedAt: time.Now(),
			Hash:        utils.GenerateHash(interfaceName),
		}
		interfaces = append(interfaces, chunk)
	}

	return interfaces, nil
}

// analyzeVariableValue analyzes the value of a variable declarator to detect ES6 features.
func analyzeVariableValue(
	declarator *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
	metadata map[string]interface{},
) {
	// Find the value node (it's typically after the '=' token)
	var valueNode *valueobject.ParseNode
	for _, child := range declarator.Children {
		if child.Type != nodeTypeIdentifier && child.Type != "=" {
			valueNode = child
			break
		}
	}

	if valueNode == nil {
		return
	}

	// Check for template strings
	if hasDescendantOfType(valueNode, "template_string") {
		metadata["has_template_expressions"] = true
	}

	// Check for symbols (new_expression with Symbol identifier)
	if hasSymbolExpression(valueNode, parseTree) {
		metadata["is_symbol"] = true
	}

	// Check for computed properties in object expressions
	if hasDescendantOfType(valueNode, "computed_property_name") {
		metadata["has_computed_properties"] = true
	}
}

// inferReturnType infers the return type from a variable declarator.
func inferReturnType(declarator *valueobject.ParseNode, parseTree *valueobject.ParseTree) string {
	// Find the value node
	var valueNode *valueobject.ParseNode
	for _, child := range declarator.Children {
		if child.Type != nodeTypeIdentifier && child.Type != "=" {
			valueNode = child
			break
		}
	}

	if valueNode == nil {
		return ""
	}

	// Check for new_expression to determine constructor type
	newExprs := findDescendantsByType(valueNode, "new_expression")
	for _, newExpr := range newExprs {
		// Get the constructor name (first identifier child)
		for _, child := range newExpr.Children {
			if child.Type == nodeTypeIdentifier {
				constructorName := parseTree.GetNodeText(child)
				// Return constructor names like Map, Set, Promise, etc.
				if constructorName == "Map" || constructorName == "Set" ||
					constructorName == "WeakMap" || constructorName == "WeakSet" ||
					constructorName == "Promise" {
					return constructorName
				}
				break
			}
		}
	}

	// Check for BigInt
	numberNodes := findDescendantsByType(valueNode, "number")
	for _, numNode := range numberNodes {
		numText := parseTree.GetNodeText(numNode)
		if strings.HasSuffix(numText, "n") {
			return "bigint"
		}
	}

	return ""
}

// hasDescendantOfType checks if a node has any descendant of the given type.
func hasDescendantOfType(node *valueobject.ParseNode, nodeType string) bool {
	if node.Type == nodeType {
		return true
	}

	for _, child := range node.Children {
		if hasDescendantOfType(child, nodeType) {
			return true
		}
	}

	return false
}

// findDescendantsByType finds all descendants of the given type.
func findDescendantsByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var result []*valueobject.ParseNode

	if node.Type == nodeType {
		result = append(result, node)
	}

	for _, child := range node.Children {
		result = append(result, findDescendantsByType(child, nodeType)...)
	}

	return result
}

// hasSymbolExpression checks if a node contains a Symbol() expression.
func hasSymbolExpression(node *valueobject.ParseNode, parseTree *valueobject.ParseTree) bool {
	// Look for call_expression with Symbol identifier
	callExprs := findDescendantsByType(node, "call_expression")
	for _, callExpr := range callExprs {
		// Check if the function being called is "Symbol"
		for _, child := range callExpr.Children {
			if child.Type == nodeTypeIdentifier && parseTree.GetNodeText(child) == "Symbol" {
				return true
			}
		}
	}

	// Also check for member_expression like Symbol.for or Symbol.iterator
	memberExprs := findDescendantsByType(node, "member_expression")
	for _, memberExpr := range memberExprs {
		// Check if the object is "Symbol"
		for _, child := range memberExpr.Children {
			if child.Type == nodeTypeIdentifier && parseTree.GetNodeText(child) == "Symbol" {
				return true
			}
		}
	}

	return false
}

// variableInfo represents extracted variable information from a pattern.
type variableInfo struct {
	name             string
	node             *valueobject.ParseNode
	metadata         map[string]interface{}
	isRest           bool
	hasDefault       bool
	originalProperty string // for renamed destructuring
}

// extractVariablesFromPattern recursively extracts all variables from a destructuring pattern.
func extractVariablesFromPattern(
	pattern *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) []variableInfo {
	var variables []variableInfo

	switch pattern.Type {
	case nodeTypeIdentifier:
		// Simple identifier - direct extraction
		varName := parseTree.GetNodeText(pattern)
		variables = append(variables, variableInfo{
			name:     varName,
			node:     pattern,
			metadata: make(map[string]interface{}),
		})

	case "array_pattern":
		// Array destructuring: [a, b, c] or [head, ...tail]
		for _, child := range pattern.Children {
			// Skip punctuation nodes like '[', ']', ','
			if child.Type == "[" || child.Type == "]" || child.Type == "," {
				continue
			}

			// Recurse on each element
			childVars := extractVariablesFromPattern(child, parseTree)
			variables = append(variables, childVars...)
		}

	case "object_pattern":
		// Object destructuring: {a, b, c} or {name: firstName}
		for _, child := range pattern.Children {
			// Skip punctuation nodes like '{', '}', ','
			if child.Type == "{" || child.Type == "}" || child.Type == "," {
				continue
			}

			switch child.Type {
			case "shorthand_property_identifier_pattern":
				// Shorthand: {name} where variable name = property name
				varName := parseTree.GetNodeText(child)
				variables = append(variables, variableInfo{
					name:     varName,
					node:     child,
					metadata: make(map[string]interface{}),
				})

			case "pair_pattern":
				// Renamed property: {name: firstName}
				// CRITICAL: Extract from VALUE field, not key!
				var keyNode, valueNode *valueobject.ParseNode

				for _, pairChild := range child.Children {
					if pairChild.Type == nodeTypePropertyID ||
						pairChild.Type == "string" ||
						pairChild.Type == "number" ||
						pairChild.Type == "computed_property_name" {
						keyNode = pairChild
					} else if pairChild.Type != ":" {
						// The value is anything that's not the key or ':'
						valueNode = pairChild
					}
				}

				if valueNode != nil {
					// Recurse on the value (which could be another pattern)
					valueVars := extractVariablesFromPattern(valueNode, parseTree)

					// Add original property metadata if we have a key
					if keyNode != nil {
						originalProp := parseTree.GetNodeText(keyNode)
						for i := range valueVars {
							if valueVars[i].metadata == nil {
								valueVars[i].metadata = make(map[string]interface{})
							}
							valueVars[i].originalProperty = originalProp
						}
					}

					variables = append(variables, valueVars...)
				}

			case nodeTypeRestPattern:
				// Rest element: {...rest}
				// Extract the identifier after '...'
				for _, restChild := range child.Children {
					if restChild.Type != "..." {
						restVars := extractVariablesFromPattern(restChild, parseTree)
						for i := range restVars {
							restVars[i].isRest = true
							if restVars[i].metadata == nil {
								restVars[i].metadata = make(map[string]interface{})
							}
							restVars[i].metadata["is_rest"] = true
						}
						variables = append(variables, restVars...)
					}
				}

			case "object_assignment_pattern":
				// Default value in object: {name = 'default'}
				// Extract the left side (the identifier)
				for _, assignChild := range child.Children {
					if assignChild.Type != "=" {
						assignVars := extractVariablesFromPattern(assignChild, parseTree)
						for i := range assignVars {
							assignVars[i].hasDefault = true
							if assignVars[i].metadata == nil {
								assignVars[i].metadata = make(map[string]interface{})
							}
							assignVars[i].metadata["has_default"] = true
						}
						variables = append(variables, assignVars...)
						break // Only want the left side, not the default value
					}
				}
			}
		}

	case nodeTypeRestPattern:
		// Rest element in array: [...tail]
		for _, child := range pattern.Children {
			if child.Type != "..." {
				restVars := extractVariablesFromPattern(child, parseTree)
				for i := range restVars {
					restVars[i].isRest = true
					if restVars[i].metadata == nil {
						restVars[i].metadata = make(map[string]interface{})
					}
					restVars[i].metadata["is_rest"] = true
				}
				variables = append(variables, restVars...)
			}
		}

	case "assignment_pattern":
		// Default value: [a = 10] or {a = 10}
		// Extract the left side (the pattern/identifier)
		for _, child := range pattern.Children {
			if child.Type != "=" {
				assignVars := extractVariablesFromPattern(child, parseTree)
				for i := range assignVars {
					assignVars[i].hasDefault = true
					if assignVars[i].metadata == nil {
						assignVars[i].metadata = make(map[string]interface{})
					}
					assignVars[i].metadata["has_default"] = true
				}
				variables = append(variables, assignVars...)
				break // Only want the left side, not the default value
			}
		}

	case "shorthand_property_identifier_pattern":
		// This can appear at top level in some cases
		varName := parseTree.GetNodeText(pattern)
		variables = append(variables, variableInfo{
			name:     varName,
			node:     pattern,
			metadata: make(map[string]interface{}),
		})
	}

	return variables
}

// collectNamespaceImports scans the parse tree for namespace import statements
// and returns a map of imported namespace identifiers.
// Example: "import * as utils from './utils.js'" → map["utils"] = true.
func collectNamespaceImports(ctx context.Context, parseTree *valueobject.ParseTree) map[string]bool {
	namespaceImports := make(map[string]bool)

	// Find all import_statement nodes
	importStatements := parseTree.GetNodesByType("import_statement")

	for _, importStmt := range importStatements {
		// Look for import_clause child
		var importClause *valueobject.ParseNode
		for _, child := range importStmt.Children {
			if child.Type == "import_clause" {
				importClause = child
				break
			}
		}

		if importClause == nil {
			continue
		}

		// Look for namespace_import child within the import_clause
		var namespaceImport *valueobject.ParseNode
		for _, child := range importClause.Children {
			if child.Type == "namespace_import" {
				namespaceImport = child
				break
			}
		}

		if namespaceImport == nil {
			continue
		}

		// Extract the identifier from namespace_import (the "utils" in "* as utils")
		for _, child := range namespaceImport.Children {
			if child.Type == nodeTypeIdentifier {
				identifierName := parseTree.GetNodeText(child)
				namespaceImports[identifierName] = true
				break
			}
		}
	}

	return namespaceImports
}

// detectNamespaceImportAssignment checks if a variable declarator is assigned
// from a namespace import identifier. Returns true if the initializer is a
// simple identifier reference that matches a namespace import.
// Example: const utilsNamespace = utils; (where utils is a namespace import).
func detectNamespaceImportAssignment(
	declarator *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
	namespaceImports map[string]bool,
) bool {
	// Grammar structure (from tree-sitter-javascript/grammar.js:344-351):
	// variable_declarator: $ => seq(
	//   field('name', choice($.identifier, alias('of', $.identifier), $._destructuring_pattern)),
	//   optional($._initializer),  // _initializer: $ => seq('=', field('value', $.expression))
	// )
	//
	// In the AST, this appears as children: [name_node, '=' token, value_node]
	// We find the value node by looking for the first child after the '=' token.

	var valueNode *valueobject.ParseNode
	foundEquals := false

	for _, child := range declarator.Children {
		if foundEquals {
			// This is the first node after "=", which is our value
			valueNode = child
			break
		}
		if child.Type == "=" {
			foundEquals = true
		}
	}

	if valueNode == nil {
		// No value means no assignment (e.g., `let x;`)
		return false
	}

	// Check if the value is a simple identifier (not a complex expression)
	// We WANT it to be an identifier for a simple assignment like: const x = utils;
	if valueNode.Type == nodeTypeIdentifier {
		// Get the identifier text and check if it's a namespace import
		identifierText := parseTree.GetNodeText(valueNode)
		return namespaceImports[identifierText]
	}

	return false
}
