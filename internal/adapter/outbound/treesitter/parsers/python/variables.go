package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// extractPythonVariables extracts Python variables from the parse tree.
func extractPythonVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var variables []outbound.SemanticCodeChunk
	now := time.Now()
	moduleName := extractModuleName(parseTree)

	// Extract module-level variables
	moduleVars := extractModuleLevelVariables(parseTree, moduleName, options, now)
	for _, variable := range moduleVars {
		if shouldIncludeByVisibility(variable.Visibility, options.IncludePrivate) {
			variables = append(variables, variable)
		}
	}

	// Extract class variables from all classes
	classNodes := parseTree.GetNodesByType("class_definition")
	for _, classNode := range classNodes {
		className := extractClassNameFromNode(parseTree, classNode)
		classVars := extractClassVariables(parseTree, classNode, className, moduleName, options, now)
		for _, variable := range classVars {
			if shouldIncludeByVisibility(variable.Visibility, options.IncludePrivate) {
				variables = append(variables, variable)
			}
		}
	}

	return variables, nil
}

// extractModuleLevelVariables extracts module-level variable assignments.
func extractModuleLevelVariables(
	parseTree *valueobject.ParseTree,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var variables []outbound.SemanticCodeChunk

	// Find all expression statements (potential assignments) at module level
	rootNode := parseTree.RootNode()
	if rootNode == nil {
		return variables
	}

	for _, child := range rootNode.Children {
		switch child.Type {
		case "expression_statement":
			variable := extractVariableFromExpressionStatement(parseTree, child, moduleName, options, now)
			if variable != nil {
				variables = append(variables, *variable)
			}
		case "assignment":
			variable := extractVariableFromAssignment(parseTree, child, moduleName, options, now)
			if variable != nil {
				variables = append(variables, *variable)
			}
		}
	}

	return variables
}

// extractVariableFromExpressionStatement extracts a variable from an expression statement.
func extractVariableFromExpressionStatement(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if node == nil {
		return nil
	}

	// Look for assignment within the expression statement
	assignmentNode := findChildByType(node, "assignment")
	if assignmentNode != nil {
		return extractVariableFromAssignment(parseTree, assignmentNode, moduleName, options, now)
	}

	return nil
}

// extractVariableFromAssignment extracts a variable from an assignment node.
func extractVariableFromAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if assignmentNode == nil {
		return nil
	}

	// Extract variable names from the assignment
	variables := extractVariableNamesFromAssignment(parseTree, assignmentNode)
	if len(variables) == 0 {
		return nil
	}

	// For now, handle single variable assignment
	// Multiple assignment (x, y = 1, 2) would need special handling
	varName := variables[0]
	content := parseTree.GetNodeText(assignmentNode)

	// Determine if it's a constant (all uppercase) or variable
	constructType := outbound.ConstructVariable
	if strings.ToUpper(varName) == varName {
		constructType = outbound.ConstructConstant
	}

	// Extract type annotation if present
	returnType := extractTypeAnnotationFromAssignment(parseTree, assignmentNode)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("variable", varName, nil),
		Type:          constructType,
		Name:          varName,
		QualifiedName: qualifyName(moduleName, varName),
		Language:      parseTree.Language(),
		StartByte:     assignmentNode.StartByte,
		EndByte:       assignmentNode.EndByte,
		Content:       content,
		Visibility:    getPythonVisibility(varName),
		ReturnType:    returnType,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// extractVariableNamesFromAssignment extracts variable names from an assignment.
func extractVariableNamesFromAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
) []string {
	var names []string

	// Look for the left side of assignment (targets)
	for _, child := range assignmentNode.Children {
		switch child.Type {
		case "identifier":
			names = append(names, parseTree.GetNodeText(child))
		case "pattern_list", "tuple_pattern":
			// Handle multiple assignment like x, y = 1, 2
			tupleNames := extractNamesFromTuple(parseTree, child)
			names = append(names, tupleNames...)
		}
	}

	return names
}

// extractNamesFromTuple extracts variable names from a tuple pattern.
func extractNamesFromTuple(parseTree *valueobject.ParseTree, tupleNode *valueobject.ParseNode) []string {
	var names []string

	for _, child := range tupleNode.Children {
		if child.Type == "identifier" {
			names = append(names, parseTree.GetNodeText(child))
		}
	}

	return names
}

// extractTypeAnnotationFromAssignment extracts type annotation from assignment.
func extractTypeAnnotationFromAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
) string {
	// Look for type annotation in assignment
	for _, child := range assignmentNode.Children {
		if child.Type == "type" || child.Type == "type_annotation" {
			return parseTree.GetNodeText(child)
		}
	}

	// Try to infer type from the value
	return inferTypeFromValue(parseTree, assignmentNode)
}

// inferTypeFromValue attempts to infer type from the assigned value.
func inferTypeFromValue(parseTree *valueobject.ParseTree, assignmentNode *valueobject.ParseNode) string {
	// Look for the value being assigned
	for _, child := range assignmentNode.Children {
		switch child.Type {
		case "string":
			return "str"
		case "integer":
			return "int"
		case "float":
			return "float"
		case "true", "false":
			return "bool"
		case "none":
			return "None"
		case "list":
			return "list"
		case "dictionary":
			return "dict"
		case "set":
			return "set"
		case "tuple":
			return "tuple"
		}
	}
	return ""
}

// extractInstanceVariables extracts instance variables from __init__ methods.
func extractInstanceVariables(
	parseTree *valueobject.ParseTree,
	initMethodNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var variables []outbound.SemanticCodeChunk

	if initMethodNode == nil {
		return variables
	}

	// Find the method body
	bodyNode := findChildByType(initMethodNode, "block")
	if bodyNode == nil {
		return variables
	}

	// Look for self.variable assignments
	for _, child := range bodyNode.Children {
		if child.Type == "expression_statement" {
			variable := extractInstanceVariableFromStatement(parseTree, child, className, moduleName, options, now)
			if variable != nil {
				variables = append(variables, *variable)
			}
		}
	}

	return variables
}

// extractInstanceVariableFromStatement extracts instance variable from a statement.
func extractInstanceVariableFromStatement(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if node == nil {
		return nil
	}

	// Look for assignment within the statement
	assignmentNode := findChildByType(node, "assignment")
	if assignmentNode == nil {
		return nil
	}

	// Check if it's a self.attribute assignment
	attributeNode := findChildByType(assignmentNode, "attribute")
	if attributeNode == nil {
		return nil
	}

	// Extract the attribute name
	identifierNode := findChildByType(attributeNode, "identifier")
	if identifierNode == nil {
		return nil
	}

	varName := parseTree.GetNodeText(identifierNode)
	content := parseTree.GetNodeText(assignmentNode)

	// Extract type annotation if present
	returnType := extractTypeAnnotationFromAssignment(parseTree, assignmentNode)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("instance_var", varName, nil),
		Type:          outbound.ConstructVariable,
		Name:          varName,
		QualifiedName: qualifyName(moduleName, className, varName),
		Language:      parseTree.Language(),
		StartByte:     assignmentNode.StartByte,
		EndByte:       assignmentNode.EndByte,
		Content:       content,
		Visibility:    getPythonVisibility(varName),
		ReturnType:    returnType,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}
