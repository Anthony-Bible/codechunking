package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

func extractJavaScriptVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var variables []outbound.SemanticCodeChunk
	moduleName := "main"

	// Find all variable declarations (var)
	varDeclarations := parseTree.GetNodesByType("variable_declaration")
	for _, node := range varDeclarations {
		declarators := findChildrenByType(node, "variable_declarator")
		for _, declarator := range declarators {
			nameNodes := findChildrenByType(declarator, "identifier")
			if len(nameNodes) == 0 {
				continue
			}
			nameNode := nameNodes[0]

			varName := parseTree.GetNodeText(nameNode)
			startByte := nameNode.StartByte
			endByte := nameNode.EndByte
			startPos := nameNode.StartPos
			endPos := nameNode.EndPos

			chunk := outbound.SemanticCodeChunk{
				ChunkID:       utils.GenerateID(string(outbound.ConstructVariable), varName, nil),
				Type:          outbound.ConstructVariable,
				Name:          varName,
				QualifiedName: moduleName + "." + varName,
				Language:      parseTree.Language(),
				StartByte:     startByte,
				EndByte:       endByte,
				StartPosition: valueobject.Position{Row: startPos.Row, Column: startPos.Column},
				EndPosition:   valueobject.Position{Row: endPos.Row, Column: endPos.Column},
				Content:       parseTree.GetNodeText(declarator),
				Metadata: map[string]interface{}{
					"declaration_type": "var",
					"scope":            "global",
				},
				Annotations: []outbound.Annotation{},
				ExtractedAt: time.Now(),
				Hash:        utils.GenerateHash(varName),
			}
			variables = append(variables, chunk)
		}
	}

	// Find all lexical declarations (let/const)
	lexicalDeclarations := parseTree.GetNodesByType("lexical_declaration")
	for _, node := range lexicalDeclarations {
		declarators := findChildrenByType(node, "variable_declarator")
		for _, declarator := range declarators {
			nameNodes := findChildrenByType(declarator, "identifier")
			if len(nameNodes) == 0 {
				continue
			}
			nameNode := nameNodes[0]

			varName := parseTree.GetNodeText(nameNode)
			startByte := nameNode.StartByte
			endByte := nameNode.EndByte
			startPos := nameNode.StartPos
			endPos := nameNode.EndPos

			// Determine if it's a let or const declaration
			declarationType := "let"
			if len(node.Children) > 0 && parseTree.GetNodeText(node.Children[0]) == "const" {
				declarationType = "const"
			}

			chunkType := outbound.ConstructVariable
			if declarationType == "const" {
				chunkType = outbound.ConstructConstant
			}

			// Build metadata with ES6 feature detection
			metadata := map[string]interface{}{
				"declaration_type": declarationType,
				"scope":            "block",
			}

			// Analyze the declarator value to detect ES6 features
			if len(declarator.Children) > 0 {
				analyzeVariableValue(declarator, parseTree, metadata)
			}

			chunk := outbound.SemanticCodeChunk{
				ChunkID:       utils.GenerateID(string(chunkType), varName, nil),
				Type:          chunkType,
				Name:          varName,
				QualifiedName: moduleName + "." + varName,
				Language:      parseTree.Language(),
				StartByte:     startByte,
				EndByte:       endByte,
				StartPosition: valueobject.Position{Row: startPos.Row, Column: startPos.Column},
				EndPosition:   valueobject.Position{Row: endPos.Row, Column: endPos.Column},
				Content:       parseTree.GetNodeText(declarator),
				Metadata:      metadata,
				ReturnType:    inferReturnType(declarator, parseTree),
				Annotations:   []outbound.Annotation{},
				ExtractedAt:   time.Now(),
				Hash:          utils.GenerateHash(varName),
			}
			variables = append(variables, chunk)
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
		if child.Type != "identifier" && child.Type != "=" {
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
		if child.Type != "identifier" && child.Type != "=" {
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
			if child.Type == "identifier" {
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
			if child.Type == "identifier" && parseTree.GetNodeText(child) == "Symbol" {
				return true
			}
		}
	}

	// Also check for member_expression like Symbol.for or Symbol.iterator
	memberExprs := findDescendantsByType(node, "member_expression")
	for _, memberExpr := range memberExprs {
		// Check if the object is "Symbol"
		for _, child := range memberExpr.Children {
			if child.Type == "identifier" && parseTree.GetNodeText(child) == "Symbol" {
				return true
			}
		}
	}

	return false
}
