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
			vars := extractVariablesFromExpressionStatement(parseTree, child, moduleName, options, now)
			variables = append(variables, vars...)
		case "assignment":
			vars := extractVariablesFromAssignment(parseTree, child, moduleName, options, now)
			variables = append(variables, vars...)
		case "annotated_assignment":
			variable := extractVariableFromAnnotatedAssignment(parseTree, child, moduleName, options, now)
			if variable != nil {
				variables = append(variables, *variable)
			}
		}
	}

	return variables
}

// extractVariablesFromExpressionStatement extracts variables from an expression statement.
func extractVariablesFromExpressionStatement(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	if node == nil {
		return nil
	}

	// Look for assignment within the expression statement
	assignmentNode := findChildByType(node, "assignment")
	if assignmentNode != nil {
		return extractVariablesFromAssignment(parseTree, assignmentNode, moduleName, options, now)
	}

	// Look for annotated assignment within the expression statement
	annotatedAssignmentNode := findChildByType(node, "annotated_assignment")
	if annotatedAssignmentNode != nil {
		variable := extractVariableFromAnnotatedAssignment(parseTree, annotatedAssignmentNode, moduleName, options, now)
		if variable != nil {
			return []outbound.SemanticCodeChunk{*variable}
		}
	}

	return nil
}

// extractVariablesFromAssignment extracts variables from an assignment node.
func extractVariablesFromAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	if assignmentNode == nil {
		return nil
	}

	// Extract variable names from the assignment
	variables := extractVariableNamesFromAssignment(parseTree, assignmentNode)
	if len(variables) == 0 {
		return nil
	}

	content := parseTree.GetNodeText(assignmentNode)
	returnType := extractTypeAnnotationFromAssignment(parseTree, assignmentNode)

	var chunks []outbound.SemanticCodeChunk
	for _, varName := range variables {
		// Determine if it's a constant (all uppercase) or variable
		constructType := outbound.ConstructVariable
		if strings.ToUpper(varName) == varName {
			constructType = outbound.ConstructConstant
		}

		chunk := outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID("variable", varName, nil),
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
		chunks = append(chunks, chunk)
	}

	return chunks
}

// extractVariableFromAnnotatedAssignment extracts a variable from an annotated assignment node.
func extractVariableFromAnnotatedAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if assignmentNode == nil {
		return nil
	}

	// Extract variable name from the annotated assignment
	varName := extractVariableNameFromAnnotatedAssignment(parseTree, assignmentNode)
	if varName == "" {
		return nil
	}

	content := parseTree.GetNodeText(assignmentNode)

	// Determine if it's a constant (all uppercase) or variable
	constructType := outbound.ConstructVariable
	if strings.ToUpper(varName) == varName {
		constructType = outbound.ConstructConstant
	}

	// Extract type annotation
	returnType := extractTypeAnnotationFromAnnotatedAssignment(parseTree, assignmentNode)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("variable", varName, nil),
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

// extractVariableNameFromAnnotatedAssignment extracts the variable name from an annotated assignment.
func extractVariableNameFromAnnotatedAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
) string {
	// Look for the left side of annotated assignment (target)
	for _, child := range assignmentNode.Children {
		if child.Type == "identifier" {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
}

// extractTypeAnnotationFromAnnotatedAssignment extracts type annotation from annotated assignment.
func extractTypeAnnotationFromAnnotatedAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
) string {
	// Look for type annotation in annotated assignment
	for _, child := range assignmentNode.Children {
		if child.Type == "type" || child.Type == "type_annotation" {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
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
			vars := extractInstanceVariablesFromStatement(parseTree, child, className, moduleName, options, now)
			variables = append(variables, vars...)
		}
	}

	return variables
}

// extractInstanceVariablesFromStatement extracts instance variables from a statement.
func extractInstanceVariablesFromStatement(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
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

	// Extract the attribute names
	varNames := extractVariableNamesFromAssignment(parseTree, assignmentNode)
	if len(varNames) == 0 {
		return nil
	}

	content := parseTree.GetNodeText(assignmentNode)
	returnType := extractTypeAnnotationFromAssignment(parseTree, assignmentNode)

	var chunks []outbound.SemanticCodeChunk
	for _, varName := range varNames {
		chunk := outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID("instance_var", varName, nil),
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
		chunks = append(chunks, chunk)
	}

	return chunks
}
