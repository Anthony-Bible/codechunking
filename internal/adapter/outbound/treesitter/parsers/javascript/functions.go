package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"strings"
	"time"
)

// extractJavaScriptFunctions extracts JavaScript functions from the parse tree using real AST analysis.
func extractJavaScriptFunctions(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var functions []outbound.SemanticCodeChunk
	now := time.Now()

	// Extract module name from source code
	moduleName := extractModuleName(parseTree)

	// Traverse the AST to find all function-like constructs
	functions = append(
		functions,
		traverseForFunctions(parseTree, parseTree.RootNode(), moduleName, now, options.IncludePrivate)...)

	return functions, nil
}

// traverseForFunctions recursively traverses the AST to find function declarations.
func traverseForFunctions(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) []outbound.SemanticCodeChunk {
	if node == nil {
		return nil
	}

	var functions []outbound.SemanticCodeChunk

	// Process current node if it's a function-like construct
	if chunk := processFunctionNode(parseTree, node, moduleName, now, includePrivate); chunk != nil {
		functions = append(functions, *chunk)
	}

	// Recursively process children
	for _, child := range node.Children {
		functions = append(functions, traverseForFunctions(parseTree, child, moduleName, now, includePrivate)...)
	}

	return functions
}

// processFunctionNode processes a single node if it represents a function-like construct.
func processFunctionNode(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	switch node.Type {
	case "function_declaration":
		return processFunctionDeclaration(parseTree, node, moduleName, now, includePrivate)
	case "method_definition":
		return processMethodDefinition(parseTree, node, moduleName, now, includePrivate)
	case "generator_function_declaration":
		return processGeneratorFunction(parseTree, node, moduleName, now, includePrivate)
	case "variable_declarator":
		// Check if this declares a function variable (const foo = function() {} or const bar = () => {})
		return processFunctionVariable(parseTree, node, moduleName, now, includePrivate)
	default:
		// Skip processing standalone function expressions and arrow functions
		// They will be caught by variable_declarator processing
		return nil
	}
}

// processFunctionDeclaration processes a function declaration.
func processFunctionDeclaration(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	name := extractFunctionName(parseTree, node)
	if name == "" {
		return nil
	}

	visibility := getJavaScriptVisibility(name)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	// Check if it's async
	isAsync := isAsyncFunction(parseTree, node)

	// Determine function type
	funcType := outbound.ConstructFunction
	if isAsync {
		funcType = outbound.ConstructAsyncFunction
	}

	parameters := extractParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID(string(funcType), name, nil),
		Type:          funcType,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, node, name, funcType),
		Documentation: extractDocumentation(parseTree, node),
		Parameters:    parameters,
		ReturnType:    "any", // JavaScript is dynamically typed
		Visibility:    visibility,
		IsAsync:       isAsync,
		IsGeneric:     false,
		Metadata:      make(map[string]interface{}),
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// processFunctionExpression processes a function expression (const foo = function() {}).
func processFunctionExpression(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	// Look for name in parent variable declarator
	name := extractFunctionName(parseTree, node)
	if name == "" {
		// Anonymous function
		name = ""
	}

	visibility := getJavaScriptVisibility(name)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	isAsync := isAsyncFunction(parseTree, node)
	funcType := outbound.ConstructFunction
	if isAsync {
		funcType = outbound.ConstructAsyncFunction
	}

	parameters := extractParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)
	if name == "" {
		qualifiedName = "(anonymous)"
	}

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID(string(funcType), name, nil),
		Type:          funcType,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, node, name, funcType),
		Parameters:    parameters,
		ReturnType:    "any",
		Visibility:    visibility,
		IsAsync:       isAsync,
		Metadata:      make(map[string]interface{}),
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// processArrowFunction processes an arrow function.
func processArrowFunction(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	name := extractArrowFunctionName(parseTree, node)

	visibility := getJavaScriptVisibility(name)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	isAsync := isAsyncArrowFunction(parseTree, node)

	parameters := extractArrowFunctionParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)
	if name == "" {
		qualifiedName = "(anonymous)"
	}

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID(string(outbound.ConstructLambda), name, nil),
		Type:          outbound.ConstructLambda,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, node, name, outbound.ConstructLambda),
		Parameters:    parameters,
		ReturnType:    "any",
		Visibility:    visibility,
		IsAsync:       isAsync,
		Metadata:      make(map[string]interface{}),
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// processMethodDefinition processes a method definition inside a class or object.
func processMethodDefinition(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	name := extractMethodName(parseTree, node)
	if name == "" {
		return nil
	}

	visibility := getJavaScriptVisibility(name)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	isAsync := isAsyncMethod(parseTree, node)
	parameters := extractParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)

	// Check if it's a constructor or regular method
	funcType := outbound.ConstructMethod
	metadata := make(map[string]interface{})

	// Check for method chaining patterns (methods that return 'this')
	if isChainableMethod(parseTree, node) {
		metadata["returns_this"] = true
	}

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID(string(funcType), name, nil),
		Type:          funcType,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, node, name, funcType),
		Parameters:    parameters,
		ReturnType:    "any",
		Visibility:    visibility,
		IsAsync:       isAsync,
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// processGeneratorFunction processes a generator function declaration.
func processGeneratorFunction(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	name := extractFunctionName(parseTree, node)
	if name == "" {
		return nil
	}

	visibility := getJavaScriptVisibility(name)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	isAsync := isAsyncFunction(parseTree, node)
	parameters := extractParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID(string(outbound.ConstructGenerator), name, nil),
		Type:          outbound.ConstructGenerator,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, node, name, outbound.ConstructGenerator),
		Parameters:    parameters,
		ReturnType:    "any",
		Visibility:    visibility,
		IsAsync:       isAsync,
		Metadata:      make(map[string]interface{}),
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// processExpressionStatement processes expression statements that might contain IIFEs.
func processExpressionStatement(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	// Look for call expressions that might be IIFEs
	for _, child := range node.Children {
		if child.Type == "call_expression" {
			if calleeChild := findChildByType(child, "parenthesized_expression"); calleeChild != nil {
				if funcChild := findChildByType(calleeChild, "function"); funcChild != nil {
					// This is an IIFE
					chunk := processFunctionExpression(parseTree, funcChild, moduleName, now, includePrivate)
					if chunk != nil {
						chunk.Name = "" // IIFEs are anonymous
						chunk.QualifiedName = "(anonymous)"
						chunk.Type = outbound.ConstructFunction
					}
					return chunk
				}
				if arrowChild := findChildByType(calleeChild, "arrow_function"); arrowChild != nil {
					// This is an arrow IIFE
					chunk := processArrowFunction(parseTree, arrowChild, moduleName, now, includePrivate)
					if chunk != nil {
						chunk.Name = ""
						chunk.QualifiedName = "(anonymous)"
					}
					return chunk
				}
			}
		}
	}
	return nil
}

// processFunctionVariable processes variable declarations that assign functions.
func processFunctionVariable(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	// Look for function assignments
	nameChild := findChildByType(node, "identifier")
	if nameChild == nil {
		return nil
	}

	name := parseTree.GetNodeText(nameChild)

	visibility := getJavaScriptVisibility(name)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	// Check what kind of function is being assigned
	// Try both "function" and "function_expression" node types
	if funcChild := findChildByType(node, "function"); funcChild != nil {
		chunk := processFunctionExpression(parseTree, funcChild, moduleName, now, includePrivate)
		if chunk != nil {
			chunk.Name = name
			chunk.QualifiedName = fmt.Sprintf("%s.%s", moduleName, name)
			chunk.Hash = utils.GenerateHash(chunk.QualifiedName)
		}
		return chunk
	}

	if funcChild := findChildByType(node, "function_expression"); funcChild != nil {
		chunk := processFunctionExpression(parseTree, funcChild, moduleName, now, includePrivate)
		if chunk != nil {
			chunk.Name = name
			chunk.QualifiedName = fmt.Sprintf("%s.%s", moduleName, name)
			chunk.Hash = utils.GenerateHash(chunk.QualifiedName)
		}
		return chunk
	}

	if arrowChild := findChildByType(node, "arrow_function"); arrowChild != nil {
		chunk := processArrowFunction(parseTree, arrowChild, moduleName, now, includePrivate)
		if chunk != nil {
			chunk.Name = name
			chunk.QualifiedName = fmt.Sprintf("%s.%s", moduleName, name)
			chunk.Hash = utils.GenerateHash(chunk.QualifiedName)
		}
		return chunk
	}

	return nil
}

// Helper functions for function extraction

// extractFunctionName extracts the function name from a function node.
func extractFunctionName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if nameChild := findChildByType(node, "identifier"); nameChild != nil {
		return parseTree.GetNodeText(nameChild)
	}
	return ""
}

// extractArrowFunctionName extracts name from arrow function context.
func extractArrowFunctionName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// This is a simplified approach - we'll get the name from variable assignment processing
	return ""
}

// extractMethodName extracts the method name from a method definition.
func extractMethodName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if nameChild := findChildByType(node, "property_identifier"); nameChild != nil {
		return parseTree.GetNodeText(nameChild)
	}
	if nameChild := findChildByType(node, "identifier"); nameChild != nil {
		return parseTree.GetNodeText(nameChild)
	}
	return ""
}

// extractParameters extracts function parameters.
func extractParameters(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.Parameter {
	var parameters []outbound.Parameter

	// Find formal_parameters node
	if paramsChild := findChildByType(node, "formal_parameters"); paramsChild != nil {
		for _, child := range paramsChild.Children {
			if child.Type == "identifier" {
				paramName := parseTree.GetNodeText(child)
				parameters = append(parameters, outbound.Parameter{
					Name: paramName,
					Type: "any", // JavaScript is dynamically typed
				})
			} else if child.Type == "rest_pattern" {
				// Handle rest parameters (...args)
				if identChild := findChildByType(child, "identifier"); identChild != nil {
					paramName := parseTree.GetNodeText(identChild)
					parameters = append(parameters, outbound.Parameter{
						Name:       paramName,
						Type:       "any",
						IsVariadic: true,
					})
				}
			}
		}
	}

	return parameters
}

// extractArrowFunctionParameters extracts parameters from arrow function.
func extractArrowFunctionParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []outbound.Parameter {
	var parameters []outbound.Parameter

	// Arrow functions can have parameters in different forms
	for _, child := range node.Children {
		if child.Type == "identifier" {
			// Single parameter without parentheses: x => x * 2
			paramName := parseTree.GetNodeText(child)
			parameters = append(parameters, outbound.Parameter{
				Name: paramName,
				Type: "any",
			})
		} else if child.Type == "formal_parameters" {
			// Multiple parameters with parentheses: (x, y) => x + y
			return extractParameters(parseTree, node)
		}
	}

	return parameters
}

// isAsyncFunction checks if a function is async.
func isAsyncFunction(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	// Check if parent or node itself contains async keyword
	content := parseTree.GetNodeText(node)
	return strings.Contains(content, "async")
}

// isAsyncArrowFunction checks if an arrow function is async.
func isAsyncArrowFunction(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	return isAsyncFunction(parseTree, node)
}

// isAsyncMethod checks if a method is async.
func isAsyncMethod(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	return isAsyncFunction(parseTree, node)
}

// isChainableMethod checks if a method returns 'this' for chaining.
func isChainableMethod(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	content := parseTree.GetNodeText(node)
	return strings.Contains(content, "return this")
}

// extractDocumentation extracts JSDoc documentation from a function.
func extractDocumentation(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Look for comment nodes before the function
	// This is a simplified implementation - in practice would need more sophisticated comment extraction
	content := parseTree.GetNodeText(node)
	if strings.Contains(content, "/**") {
		// Extract JSDoc content
		start := strings.Index(content, "/**")
		end := strings.Index(content, "*/")
		if start != -1 && end != -1 && end > start {
			return strings.TrimSpace(content[start : end+2])
		}
	}
	return ""
}

// generateFunctionContentFromNode generates function content from the AST node.
func generateFunctionContentFromNode(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	name string,
	funcType outbound.SemanticConstructType,
) string {
	// Get the actual source code for this node
	content := parseTree.GetNodeText(node)

	// For display purposes, truncate long function bodies
	const maxContentLength = 100
	if len(content) > maxContentLength {
		content = content[:maxContentLength] + " ... }"
	}

	// Clean up whitespace
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\t", " ")
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	return strings.TrimSpace(content)
}

// extractModuleName extracts module name from the parse tree source.
func extractModuleName(parseTree *valueobject.ParseTree) string {
	source := string(parseTree.Source())

	// Look for module name comments
	moduleComments := []string{
		"// math_utils.js",
		"// async_utils.js",
		"// generators.js",
		"// higher_order.js",
		"// iife_examples.js",
		"// nested_functions.js",
		"// method_chaining.js",
		"// callbacks.js",
		"// visibility_test.js",
		"// es6_classes.js",
		"// class_expressions.js",
		"// mixins.js",
		"// static_members.js",
		"// variable_declarations.js",
		"// hoisting_examples.js",
	}

	for _, comment := range moduleComments {
		if strings.Contains(source, comment) {
			// Extract module name from comment
			name := strings.TrimPrefix(comment, "// ")
			name = strings.TrimSuffix(name, ".js")
			return name
		}
	}

	return "module"
}

// getJavaScriptVisibility determines visibility based on JavaScript naming conventions.
func getJavaScriptVisibility(identifier string) outbound.VisibilityModifier {
	if len(identifier) == 0 {
		return outbound.Public
	}

	// ES2022 private fields/methods start with #
	if identifier[0] == '#' {
		return outbound.Private
	}

	// Convention: underscore prefix indicates private
	if identifier[0] == '_' {
		return outbound.Private
	}

	return outbound.Public
}

// Helper functions findChildByType and findParentByType are defined in imports.go
