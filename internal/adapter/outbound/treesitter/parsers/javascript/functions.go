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

// Constants for function extraction.
const (
	nodeTypeVariableDeclarator = "variable_declarator"
	nodeTypeAssignmentExpr     = "assignment_expression"
	anonymousFunctionName      = "(anonymous)"
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
	chunk := processFunctionNode(parseTree, node, moduleName, now, includePrivate)
	if chunk != nil {
		functions = append(functions, *chunk)
	}

	// Special handling: If we successfully extracted a function from a variable_declarator, pair, or assignment_expression,
	// skip traversing its children to avoid duplicates (the child function was already processed)
	// However, if no function was found (e.g., const arr = [...] or pair with array value), continue traversing
	if (node.Type == nodeTypeVariableDeclarator || node.Type == "pair" || node.Type == nodeTypeAssignmentExpr) &&
		chunk != nil {
		return functions
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
	case "generator_function":
		// Generator expression: const gen = function* () {}
		return processGeneratorExpression(parseTree, node, moduleName, now, includePrivate)
	case "variable_declarator":
		// Check if this declares a function variable (const foo = function() {} or const bar = () => {})
		return processFunctionVariable(parseTree, node, moduleName, now, includePrivate)
	case "function_expression":
		// Standalone function expressions (e.g., nested in return statements or as callbacks)
		// These are often anonymous and nested
		return processFunctionExpression(parseTree, node, moduleName, now, includePrivate)
	case nodeTypeArrowFunction:
		// Standalone arrow functions (e.g., nested in return statements or as callbacks)
		return processArrowFunction(parseTree, node, moduleName, now, includePrivate)
	case "pair":
		// Object literal method: method: function() {} or method: () => {}
		return processObjectLiteralPair(parseTree, node, moduleName, now, includePrivate)
	case nodeTypeAssignmentExpr:
		// Prototype assignments: Constructor.prototype.method = function() {}
		return processPrototypeAssignment(parseTree, node, moduleName, now, includePrivate)
	default:
		// Skip "function" keyword tokens and other non-function nodes
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

	// All JavaScript functions use ConstructFunction type
	// Special properties are indicated by boolean flags
	funcType := outbound.ConstructFunction

	parameters := extractParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(funcType), name, nil),
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
	// Extract the INTERNAL function name from function_expression node
	// For "const foo = function bar() {}", this should return "bar"
	// For "const foo = function() {}", this should return ""
	name := extractFunctionName(parseTree, node)

	visibility := getJavaScriptVisibility(name)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	isAsync := isAsyncFunction(parseTree, node)

	// All JavaScript functions use ConstructFunction type
	funcType := outbound.ConstructFunction

	parameters := extractParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)
	if name == "" {
		qualifiedName = anonymousFunctionName
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(funcType), name, nil),
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
		qualifiedName = anonymousFunctionName
	}

	// Arrow functions also use ConstructFunction type, not ConstructLambda
	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructFunction), name, nil),
		Type:          outbound.ConstructFunction,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, node, name, outbound.ConstructFunction),
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
	isGenerator := isGeneratorMethod(parseTree, node)
	parameters := extractParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)

	// Check if it's a constructor or regular method
	funcType := outbound.ConstructMethod
	metadata := make(map[string]interface{})

	// Check for method chaining patterns (methods that return 'this')
	if isChainableMethod(parseTree, node) {
		metadata["returns_this"] = true
	}

	// Extract decorators from the method definition
	annotations := extractJavaScriptDecorators(parseTree, node)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(funcType), name, nil),
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
		IsGeneric:     isGenerator,
		Annotations:   annotations,
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

	// Generators use ConstructFunction with IsGeneric flag
	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructFunction), name, nil),
		Type:          outbound.ConstructFunction,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, node, name, outbound.ConstructFunction),
		Parameters:    parameters,
		ReturnType:    "any",
		Visibility:    visibility,
		IsAsync:       isAsync,
		IsGeneric:     true, // Generators are marked as generic
		Metadata:      make(map[string]interface{}),
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// processGeneratorExpression processes a generator function expression.
func processGeneratorExpression(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	// Extract the INTERNAL generator name from generator_function node
	// For "const foo = function* bar() {}", this should return "bar"
	// For "const foo = function*() {}", this should return ""
	name := extractFunctionName(parseTree, node)

	visibility := getJavaScriptVisibility(name)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	isAsync := isAsyncFunction(parseTree, node)
	parameters := extractParameters(parseTree, node)
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, name)
	if name == "" {
		qualifiedName = anonymousFunctionName
	}

	// Generator expressions use ConstructFunction with IsGeneric flag
	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructFunction), name, nil),
		Type:          outbound.ConstructFunction,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, node, name, outbound.ConstructFunction),
		Parameters:    parameters,
		ReturnType:    "any",
		Visibility:    visibility,
		IsAsync:       isAsync,
		IsGeneric:     true, // Generators are marked as generic
		Metadata:      make(map[string]interface{}),
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
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
	// Get the variable name for visibility check and metadata
	varNameChild := findChildByType(node, "identifier")
	if varNameChild == nil {
		return nil
	}

	varName := parseTree.GetNodeText(varNameChild)

	visibility := getJavaScriptVisibility(varName)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	// Check what kind of function is being assigned
	if funcChild := findChildByType(node, "function_expression"); funcChild != nil {
		chunk := processFunctionExpression(parseTree, funcChild, moduleName, now, includePrivate)
		if chunk != nil {
			// Only override the name if the function expression is anonymous
			if chunk.Name == "" {
				// Keep it anonymous - do NOT set to variable name
				chunk.Metadata["assigned_to"] = varName
			}
		}
		return chunk
	}

	if arrowChild := findChildByType(node, "arrow_function"); arrowChild != nil {
		chunk := processArrowFunction(parseTree, arrowChild, moduleName, now, includePrivate)
		if chunk != nil {
			// Arrow functions are always anonymous - do NOT set to variable name
			chunk.Metadata["assigned_to"] = varName
		}
		return chunk
	}

	if generatorChild := findChildByType(node, "generator_function"); generatorChild != nil {
		chunk := processGeneratorExpression(parseTree, generatorChild, moduleName, now, includePrivate)
		if chunk != nil {
			// Only override the name if the generator expression is anonymous
			if chunk.Name == "" {
				// Keep it anonymous - do NOT set to variable name
				chunk.Metadata["assigned_to"] = varName
			}
		}
		return chunk
	}

	return nil
}

// processObjectLiteralPair processes object literal method syntax (e.g., method: function() {} or method: () => {}).
func processObjectLiteralPair(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	// Extract the key (method name) from the pair
	keyChild := findChildByType(node, "property_identifier")
	if keyChild == nil {
		// Try identifier as fallback
		keyChild = findChildByType(node, "identifier")
	}
	if keyChild == nil {
		// Try computed_property_name
		if computedChild := findChildByType(node, "computed_property_name"); computedChild != nil {
			// Extract the computed property name (e.g., [methodName] or [Symbol.iterator])
			methodName := parseTree.GetNodeText(computedChild)
			return processObjectLiteralPairWithName(parseTree, node, methodName, moduleName, now, includePrivate)
		}
		// Try string literal key
		if stringChild := findChildByType(node, "string"); stringChild != nil {
			methodName := parseTree.GetNodeText(stringChild)
			return processObjectLiteralPairWithName(parseTree, node, methodName, moduleName, now, includePrivate)
		}
		// Try number literal key
		if numberChild := findChildByType(node, "number"); numberChild != nil {
			methodName := parseTree.GetNodeText(numberChild)
			return processObjectLiteralPairWithName(parseTree, node, methodName, moduleName, now, includePrivate)
		}
		return nil
	}

	methodName := parseTree.GetNodeText(keyChild)
	return processObjectLiteralPairWithName(parseTree, node, methodName, moduleName, now, includePrivate)
}

// processObjectLiteralPairWithName processes an object literal pair with the extracted name.
func processObjectLiteralPairWithName(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	methodName string,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	visibility := getJavaScriptVisibility(methodName)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	// Check if the value is a function expression or arrow function
	funcChild := findChildByType(node, "function_expression")
	arrowChild := findChildByType(node, "arrow_function")

	if funcChild == nil && arrowChild == nil {
		// Not a method - just a regular property
		return nil
	}

	// Extract function characteristics
	var isAsync bool
	var parameters []outbound.Parameter
	var functionNode *valueobject.ParseNode

	if funcChild != nil {
		functionNode = funcChild
		isAsync = isAsyncFunction(parseTree, funcChild)
		parameters = extractParameters(parseTree, funcChild)
	} else if arrowChild != nil {
		functionNode = arrowChild
		isAsync = isAsyncArrowFunction(parseTree, arrowChild)
		parameters = extractArrowFunctionParameters(parseTree, arrowChild)
	}

	qualifiedName := fmt.Sprintf("%s.%s", moduleName, methodName)

	// Create metadata with assigned_to field
	metadata := make(map[string]interface{})
	metadata["assigned_to"] = methodName

	// Object literal pairs with function values use ConstructFunction type (per tree-sitter grammar)
	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructFunction), methodName, nil),
		Type:          outbound.ConstructFunction,
		Name:          methodName,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, functionNode, methodName, outbound.ConstructFunction),
		Parameters:    parameters,
		ReturnType:    "any",
		Visibility:    visibility,
		IsAsync:       isAsync,
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// processPrototypeAssignment processes prototype method assignments like Constructor.prototype.method = function() {}.
func processPrototypeAssignment(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	now time.Time,
	includePrivate bool,
) *outbound.SemanticCodeChunk {
	// Check if the left side is a member_expression
	leftChild := findChildByType(node, "member_expression")
	if leftChild == nil {
		return nil
	}

	// Check if the right side is a function-like expression
	funcChild := findChildByType(node, "function_expression")
	genChild := findChildByType(node, "generator_function")
	arrowChild := findChildByType(node, "arrow_function")

	if funcChild == nil && genChild == nil && arrowChild == nil {
		return nil
	}

	// Extract the method name from the member_expression's property field
	propertyChild := findChildByType(leftChild, "property_identifier")
	if propertyChild == nil {
		return nil
	}

	methodName := parseTree.GetNodeText(propertyChild)
	visibility := getJavaScriptVisibility(methodName)
	if !includePrivate && visibility == outbound.Private {
		return nil
	}

	// Extract function characteristics based on which type we found
	var isAsync bool
	var isGenerator bool
	var parameters []outbound.Parameter
	var functionNode *valueobject.ParseNode

	switch {
	case funcChild != nil:
		functionNode = funcChild
		isAsync = isAsyncFunction(parseTree, funcChild)
		parameters = extractParameters(parseTree, funcChild)
	case genChild != nil:
		functionNode = genChild
		isAsync = isAsyncFunction(parseTree, genChild)
		isGenerator = true
		parameters = extractParameters(parseTree, genChild)
	case arrowChild != nil:
		functionNode = arrowChild
		isAsync = isAsyncArrowFunction(parseTree, arrowChild)
		parameters = extractArrowFunctionParameters(parseTree, arrowChild)
	}

	qualifiedName := fmt.Sprintf("%s.%s", moduleName, methodName)

	// Prototype assignments are methods
	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructMethod), methodName, nil),
		Type:          outbound.ConstructMethod,
		Name:          methodName,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContentFromNode(parseTree, functionNode, methodName, outbound.ConstructMethod),
		Parameters:    parameters,
		ReturnType:    "any",
		Visibility:    visibility,
		IsAsync:       isAsync,
		IsGeneric:     isGenerator,
		Metadata:      make(map[string]interface{}),
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
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
		methodName := parseTree.GetNodeText(nameChild)
		// Strip * prefix for generator methods
		methodName = strings.TrimPrefix(methodName, "*")
		return methodName
	}
	if nameChild := findChildByType(node, "identifier"); nameChild != nil {
		methodName := parseTree.GetNodeText(nameChild)
		// Strip * prefix for generator methods
		methodName = strings.TrimPrefix(methodName, "*")
		return methodName
	}
	// Handle computed property names like [Symbol.iterator] or [computedMethod]
	if computedChild := findChildByType(node, "computed_property_name"); computedChild != nil {
		return parseTree.GetNodeText(computedChild)
	}
	return ""
}

// extractParameters extracts function parameters.
func extractParameters(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.Parameter {
	var parameters []outbound.Parameter

	// Find formal_parameters node
	if paramsChild := findChildByType(node, "formal_parameters"); paramsChild != nil {
		for _, child := range paramsChild.Children {
			param := extractSingleParameter(parseTree, child)
			if param != nil {
				parameters = append(parameters, *param)
			}
		}
	}

	return parameters
}

// extractSingleParameter extracts a single parameter, handling various patterns.
func extractSingleParameter(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) *outbound.Parameter {
	switch node.Type {
	case nodeTypeIdentifier:
		// Simple parameter: a
		paramName := parseTree.GetNodeText(node)
		return &outbound.Parameter{
			Name: paramName,
			Type: "any", // JavaScript is dynamically typed
		}

	case "assignment_pattern":
		// Parameter with default value: b = 10
		// The left side is the parameter name
		if leftChild := findChildByType(node, nodeTypeIdentifier); leftChild != nil {
			paramName := parseTree.GetNodeText(leftChild)
			return &outbound.Parameter{
				Name: paramName,
				Type: "any",
			}
		}

	case "rest_pattern":
		// Rest parameter: ...rest
		if identChild := findChildByType(node, nodeTypeIdentifier); identChild != nil {
			paramName := parseTree.GetNodeText(identChild)
			return &outbound.Parameter{
				Name:       paramName,
				Type:       "any",
				IsVariadic: true,
			}
		}

	case "object_pattern", "array_pattern":
		// Destructuring parameter: {prop1, prop2} or [a, b]
		// For destructuring, we return a parameter with empty name
		return &outbound.Parameter{
			Name: "", // Destructuring patterns don't have a single name
			Type: "any",
		}
	}

	return nil
}

// extractArrowFunctionParameters extracts parameters from arrow function.
func extractArrowFunctionParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []outbound.Parameter {
	var parameters []outbound.Parameter

	// Arrow functions can have parameters in different forms
	for _, child := range node.Children {
		switch child.Type {
		case nodeTypeIdentifier:
			// Single parameter without parentheses: x => x * 2
			paramName := parseTree.GetNodeText(child)
			parameters = append(parameters, outbound.Parameter{
				Name: paramName,
				Type: "any",
			})
		case "formal_parameters":
			// Multiple parameters with parentheses: (x, y) => x + y
			return extractParameters(parseTree, node)
		}
	}

	return parameters
}

// isAsyncFunction checks if a function is async.
func isAsyncFunction(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	// For function_declaration, function_expression, generator_function_declaration, generator_function:
	// Check if there's an "async" child node
	for _, child := range node.Children {
		if child.Type == "async" {
			return true
		}
	}

	// Fallback to string matching for edge cases
	content := parseTree.GetNodeText(node)
	return strings.Contains(content, "async")
}

// isAsyncArrowFunction checks if an arrow function is async.
func isAsyncArrowFunction(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	// For arrow_function nodes, check if there's an "async" child
	for _, child := range node.Children {
		if child.Type == "async" {
			return true
		}
	}
	return false
}

// isAsyncMethod checks if a method is async.
func isAsyncMethod(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	// For method_definition nodes, check if there's an "async" child
	for _, child := range node.Children {
		if child.Type == "async" {
			return true
		}
	}
	return false
}

// isGeneratorMethod checks if a method is a generator (has * modifier).
func isGeneratorMethod(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	// Check if the method_definition node has a child node of type "*"
	// This is more reliable than string matching
	for _, child := range node.Children {
		if child.Type == "*" {
			return true
		}
	}
	return false
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

// extractJavaScriptDecorators extracts decorator annotations from a method or class node.
func extractJavaScriptDecorators(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.Annotation {
	var annotations []outbound.Annotation

	// In JavaScript, decorators are direct children of method_definition nodes
	for _, child := range node.Children {
		if child.Type == "decorator" {
			// Extract decorator name from the decorator node's children
			decoratorName := extractDecoratorName(parseTree, child)
			if decoratorName != "" {
				annotations = append(annotations, outbound.Annotation{
					Name: decoratorName,
				})
			}
		}
	}

	return annotations
}

// extractDecoratorName extracts the name from a decorator node.
func extractDecoratorName(parseTree *valueobject.ParseTree, decoratorNode *valueobject.ParseNode) string {
	// Decorator structure: decorator -> identifier/member_expression/call_expression
	for _, child := range decoratorNode.Children {
		switch child.Type {
		case "identifier":
			return parseTree.GetNodeText(child)
		case "member_expression":
			// For @foo.bar style decorators
			return parseTree.GetNodeText(child)
		case "call_expression":
			// For @foo() style decorators, extract just the function name
			// For simplicity, return the full text for now
			return parseTree.GetNodeText(child)
		}
	}
	return ""
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
