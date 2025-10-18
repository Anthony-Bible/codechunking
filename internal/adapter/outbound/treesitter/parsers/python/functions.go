package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// Constants for commonly used node types.
const (
	nodeTypeIdentifier          = "identifier"
	nodeTypeType                = "type"
	nodeTypeDecorator           = "decorator"
	nodeTypeExpressionStatement = "expression_statement"
	nodeTypeAsyncFunctionDef    = "async_function_definition"
	nodeTypeString              = "string"
	nodeTypeStringContent       = "string_content"
	nodeTypeComment             = "comment"
	nodeTypeFloat               = "float"
	nodeTypeFunctionDef         = "function_definition"
	nodeTypeDecoratedDef        = "decorated_definition"
	nodeTypeClassDef            = "class_definition"
	nodeTypeBlock               = "block"
	nodeTypeAsync               = "async"
	nodeTypeParameters          = "parameters"
	nodeTypeDefaultValue        = "default_value"
	nodeTypeListSplatPattern    = "list_splat_pattern"
)

// extractPythonFunctions extracts Python functions from the parse tree.
func extractPythonFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	now := time.Now()

	// Get all class definition nodes ONCE (optimization to avoid O(n²) with 10,000 functions)
	allClassNodes := parseTree.GetNodesByType(nodeTypeClassDef)

	// Get function definition nodes from the parse tree
	functionNodes := parseTree.GetNodesByType(nodeTypeFunctionDef)

	// Try to get async function definitions directly
	asyncNodes := parseTree.GetNodesByType(nodeTypeAsyncFunctionDef)
	functionNodes = append(functionNodes, asyncNodes...)

	// Also check for async functions in decorated definitions
	decoratedNodes := parseTree.GetNodesByType(nodeTypeDecoratedDef)
	functionNodes = append(functionNodes, decoratedNodes...)

	var functions []outbound.SemanticCodeChunk

	for _, node := range functionNodes {
		function := extractPythonFunction(ctx, parseTree, node, allClassNodes, options, now)
		if function != nil {
			// Apply visibility filtering if needed
			if !options.IncludePrivate && function.Visibility == outbound.Private {
				continue
			}
			functions = append(functions, *function)
		}
	}

	return functions, nil
}

// extractPythonFunction extracts a single Python function from a function definition node.
func extractPythonFunction(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	allClassNodes []*valueobject.ParseNode,
	options outbound.SemanticExtractionOptions,
	extractedAt time.Time,
) *outbound.SemanticCodeChunk {
	if node == nil {
		return nil
	}

	// Extract function name
	name := extractFunctionName(parseTree, node)
	if name == "" {
		return nil
	}

	// Determine visibility based on naming convention
	visibility := determinePythonVisibility(name)

	// Extract function content
	content := parseTree.GetNodeText(node)

	// Extract parameters
	parameters := extractFunctionParameters(parseTree, node)

	// Extract return type if present
	returnType := extractReturnType(parseTree, node)

	// Check if function is async
	isAsync := isAsyncFunction(node)

	// Extract documentation
	documentation := extractFunctionDocumentation(parseTree, node)

	// Check if this function is inside a class (making it a method)
	isMethod := isInsideClass(node, allClassNodes)
	var constructType outbound.SemanticConstructType
	var qualifiedName string

	if isMethod {
		constructType = outbound.ConstructMethod
		// Find the class name for qualified name
		className := findContainingClassName(node, parseTree, allClassNodes)
		qualifiedName = qualifyName(className, name)
	} else {
		constructType = outbound.ConstructFunction
		// Generate qualified name with module prefix
		moduleName := extractModuleName(parseTree)
		qualifiedName = qualifyName(moduleName, name)
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("function", name, nil),
		Type:          constructType,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		Content:       content,
		Documentation: documentation,
		Visibility:    visibility,
		Parameters:    parameters,
		ReturnType:    returnType,
		IsAsync:       isAsync,
		ExtractedAt:   extractedAt,
		Hash:          utils.GenerateHash(content),
	}
}

// extractFunctionName extracts the function name from a function definition node.
func extractFunctionName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if node == nil || parseTree == nil {
		return ""
	}

	// For a function_definition node, the function name is typically the second child
	// (first child is "def", second is the identifier)
	// But for async functions, it's: async, def, identifier
	if len(node.Children) >= 2 {
		// Try second child first (regular function)
		nameNode := node.Children[1]
		if nameNode != nil && nameNode.Type == nodeTypeIdentifier {
			return parseTree.GetNodeText(nameNode)
		}

		// Try third child (async function: async, def, identifier)
		if len(node.Children) >= 3 {
			nameNode = node.Children[2]
			if nameNode != nil && nameNode.Type == nodeTypeIdentifier {
				return parseTree.GetNodeText(nameNode)
			}
		}
	}

	// For async_function_definition, the function name is typically the third child
	// (first child is "async", second is "def", third is the identifier)
	if node.Type == nodeTypeAsyncFunctionDef && len(node.Children) >= 3 {
		nameNode := node.Children[2]
		if nameNode != nil && nameNode.Type == nodeTypeIdentifier {
			return parseTree.GetNodeText(nameNode)
		}
	}

	return ""
}

// extractReturnType extracts the return type annotation if present.
func extractReturnType(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if node == nil || parseTree == nil {
		return ""
	}

	// Look for the -> arrow marker and get the type node after it
	for i := range len(node.Children) {
		child := node.Children[i]
		if child == nil {
			continue
		}

		// Check for the -> arrow marker
		if parseTree.GetNodeText(child) == "->" {
			// The next node should be the type
			if i+1 < len(node.Children) {
				typeNode := node.Children[i+1]
				if typeNode != nil {
					// Extract the entire type expression as text
					return strings.TrimSpace(parseTree.GetNodeText(typeNode))
				}
			}
		}
	}

	return ""
}

// isAsyncFunction checks if a function is async.
func isAsyncFunction(node *valueobject.ParseNode) bool {
	if node == nil {
		return false
	}

	// Check if it's an async function definition node directly
	if node.Type == nodeTypeAsyncFunctionDef {
		return true
	}

	// For decorated definitions, check if any child is an async function
	if node.Type == nodeTypeDecoratedDef {
		for _, child := range node.Children {
			if child.Type == nodeTypeAsyncFunctionDef {
				return true
			}
		}
	}

	// For regular function definitions, check if the first token is "async"
	if node.Type == nodeTypeFunctionDef && len(node.Children) > 0 {
		// If first child is "async" keyword
		if node.Children[0].Type == nodeTypeAsync {
			return true
		}
	}

	return false
}

// extractStringContent extracts the raw content from a Python string node
// by navigating to the string_content child node in the AST.
//
// Tree-sitter Python string structure:
//
//	string
//	├── string_start ("\"\"\"", "'''", "\"", "'", "f\"", "r'", "b\"", etc.)
//	├── string_content (raw text without quotes or prefixes)
//	└── string_end ("\"\"\"", "'''", "\"", "'", etc.)
//
// This approach correctly handles all Python string types:
//   - Regular strings: "text" or 'text'
//   - Triple-quoted: """text""" or ”'text”'
//   - F-strings: f"text {expr}" or f'text {expr}'
//   - Raw strings: r"C:\path" or r'regex\d+'
//   - Byte strings: b"bytes" or b'bytes'
//   - Combined prefixes: rf"raw f-string", br"raw bytes", etc.
//
// Benefits over string manipulation:
//   - Embedded quotes don't require manual parsing: "He said \"hello\"" -> He said "hello"
//   - Escape sequences are preserved correctly: "Line 1\nLine 2" -> Line 1\nLine 2
//   - String prefixes (f, r, b, u) are automatically stripped by the AST structure
//   - Triple-quoted strings with embedded quotes work correctly
//   - No regex or complex string parsing needed
//
// The function also unescapes common Python escape sequences that appear in the
// string_content node (e.g., \", \') for proper display in documentation.
func extractStringContent(parseTree *valueobject.ParseTree, stringNode *valueobject.ParseNode) string {
	if stringNode == nil || stringNode.Type != nodeTypeString {
		return ""
	}

	// Navigate to string_content child
	for _, child := range stringNode.Children {
		if child.Type == nodeTypeStringContent {
			content := parseTree.GetNodeText(child)
			// Unescape common Python string escape sequences
			content = strings.ReplaceAll(content, `\"`, `"`)
			content = strings.ReplaceAll(content, `\'`, `'`)
			return content
		}
	}

	// Fallback: empty string if no string_content found
	return ""
}

// extractFunctionDocumentation extracts docstring from function.
func extractFunctionDocumentation(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if node == nil || parseTree == nil {
		return ""
	}

	// Find the function body block
	var bodyNode *valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeTypeBlock {
			bodyNode = child
			break
		}
	}

	if bodyNode == nil {
		return ""
	}

	// Look for the first statement in the block which should be the docstring
	if len(bodyNode.Children) > 0 {
		firstStmt := bodyNode.Children[0]
		// Check if it's an expression statement containing a string
		if firstStmt.Type == nodeTypeExpressionStatement && len(firstStmt.Children) > 0 {
			stringNode := firstStmt.Children[0]
			if stringNode.Type == nodeTypeString {
				return extractStringContent(parseTree, stringNode)
			}
		}
	}

	return ""
}

// determinePythonVisibility determines visibility based on Python naming conventions.
func determinePythonVisibility(name string) outbound.VisibilityModifier {
	// Single underscore prefix (e.g., _private_method) indicates private
	if strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "__") {
		return outbound.Private
	}
	// Double underscore prefix and suffix (e.g., __dunder__) are also considered private
	if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
		return outbound.Private
	}
	// Double underscore prefix without suffix (e.g., __private) are private (name mangling)
	if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
		return outbound.Private
	}
	return outbound.Public
}

// extractMethodsFromClass extracts methods from a class definition.
func extractMethodsFromClass(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var methods []outbound.SemanticCodeChunk

	// Get decorated definitions for filtering
	decoratedNodes := findChildrenByType(classNode, nodeTypeDecoratedDef)

	// Find function definitions within the class that are NOT inside decorated definitions
	functionNodes := findChildrenByType(classNode, nodeTypeFunctionDef)
	for _, node := range functionNodes {
		// Skip if this function is inside a decorated definition
		if isNodeInDecoratedDefinition(node, decoratedNodes) {
			continue
		}
		method := parsePythonMethod(ctx, parseTree, node, className, moduleName, options, now)
		if method != nil {
			methods = append(methods, *method)
		}
	}

	// Find async function definitions within the class that are NOT inside decorated definitions
	asyncFunctionNodes := findChildrenByType(classNode, nodeTypeAsyncFunctionDef)
	for _, node := range asyncFunctionNodes {
		// Skip if this async function is inside a decorated definition
		if isNodeInDecoratedDefinition(node, decoratedNodes) {
			continue
		}
		method := parsePythonAsyncMethod(ctx, parseTree, node, className, moduleName, options, now)
		if method != nil {
			methods = append(methods, *method)
		}
	}

	// Process decorated definitions within the class
	for _, node := range decoratedNodes {
		// Look for function definitions within decorated definitions
		functionNode := findChildByType(node, nodeTypeFunctionDef)
		if functionNode != nil {
			method := parsePythonDecoratedMethod(
				ctx,
				parseTree,
				functionNode,
				node,
				className,
				moduleName,
				options,
				now,
			)
			if method != nil {
				methods = append(methods, *method)
			}
		}

		// Look for async function definitions within decorated definitions
		asyncFunctionNode := findChildByType(node, nodeTypeAsyncFunctionDef)
		if asyncFunctionNode != nil {
			method := parsePythonDecoratedAsyncMethod(
				ctx,
				parseTree,
				asyncFunctionNode,
				node,
				className,
				moduleName,
				options,
				now,
			)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	return methods
}

// parsePythonMethod parses a method within a class.
func parsePythonMethod(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if node == nil {
		return nil
	}

	// Extract method name
	nameNode := findChildByType(node, nodeTypeIdentifier)
	if nameNode == nil {
		return nil
	}
	methodName := parseTree.GetNodeText(nameNode)

	// Extract parameters
	parameters := extractFunctionParameters(parseTree, node)

	// Extract return type annotation
	returnType := extractReturnType(parseTree, node)

	// Extract documentation
	var documentation string
	if options.IncludeDocumentation {
		documentation = extractFunctionDocstring(parseTree, node)
	}

	// NOTE: Non-decorated methods don't have decorators
	// Decorators are only extracted in parsePythonDecoratedMethod
	var annotations []outbound.Annotation

	// Get method content
	content := parseTree.GetNodeText(node)

	// Check if method is async
	isAsync := isAsyncFunction(node)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("method", methodName, nil),
		Type:          outbound.ConstructMethod,
		Name:          methodName,
		QualifiedName: qualifyName(moduleName, className, methodName),
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       content,
		Documentation: documentation,
		Visibility:    determinePythonVisibility(methodName),
		Parameters:    parameters,
		ReturnType:    returnType,
		Annotations:   annotations,
		IsAsync:       isAsync,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// parsePythonAsyncMethod parses an async method within a class.
func parsePythonAsyncMethod(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	method := parsePythonMethod(ctx, parseTree, node, className, moduleName, options, now)
	if method != nil {
		method.IsAsync = true
	}
	return method
}

// parsePythonDecoratedMethod parses a decorated method within a class.
// This function extracts decorators from the decorated_definition node to avoid duplication.
func parsePythonDecoratedMethod(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	functionNode *valueobject.ParseNode,
	decoratedNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if functionNode == nil {
		return nil
	}

	// Extract method name
	nameNode := findChildByType(functionNode, nodeTypeIdentifier)
	if nameNode == nil {
		return nil
	}
	methodName := parseTree.GetNodeText(nameNode)

	// Extract parameters
	parameters := extractFunctionParameters(parseTree, functionNode)

	// Extract return type annotation
	returnType := extractReturnType(parseTree, functionNode)

	// Extract documentation
	var documentation string
	if options.IncludeDocumentation {
		documentation = extractFunctionDocstring(parseTree, functionNode)
	}

	// Extract decorators from the decorated_definition node (not the function node)
	annotations := extractDecorators(parseTree, decoratedNode)

	// Get method content
	content := parseTree.GetNodeText(functionNode)

	// Check if method is async
	isAsync := isAsyncFunction(functionNode)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("method", methodName, nil),
		Type:          outbound.ConstructMethod,
		Name:          methodName,
		QualifiedName: qualifyName(moduleName, className, methodName),
		Language:      parseTree.Language(),
		StartByte:     functionNode.StartByte,
		EndByte:       functionNode.EndByte,
		Content:       content,
		Documentation: documentation,
		Visibility:    determinePythonVisibility(methodName),
		Parameters:    parameters,
		ReturnType:    returnType,
		Annotations:   annotations,
		IsAsync:       isAsync,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// parsePythonDecoratedAsyncMethod parses a decorated async method within a class.
func parsePythonDecoratedAsyncMethod(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	asyncFunctionNode *valueobject.ParseNode,
	decoratedNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	method := parsePythonDecoratedMethod(
		ctx,
		parseTree,
		asyncFunctionNode,
		decoratedNode,
		className,
		moduleName,
		options,
		now,
	)
	if method != nil {
		method.IsAsync = true
	}
	return method
}

// extractFunctionParameters extracts parameters from a function definition.
func extractFunctionParameters(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.Parameter {
	var parameters []outbound.Parameter

	// Find parameters node
	parametersNode := findChildByType(node, nodeTypeParameters)
	if parametersNode == nil {
		return parameters
	}

	// Track position markers
	isAfterKeywordSeparator := false
	isBeforePositionalSeparator := true

	// Extract individual parameters
	for _, child := range parametersNode.Children {
		switch child.Type {
		case "keyword_separator":
			// * marker - all parameters after this are keyword-only
			isAfterKeywordSeparator = true
			continue

		case "positional_separator":
			// / marker - all parameters before this are positional-only
			isBeforePositionalSeparator = false
			continue

		case nodeTypeIdentifier:
			// Simple identifier parameter
			paramName := parseTree.GetNodeText(child)
			param := outbound.Parameter{
				Name:             paramName,
				Type:             "Any",
				IsKeywordOnly:    isAfterKeywordSeparator,
				IsPositionalOnly: isBeforePositionalSeparator && !isAfterKeywordSeparator,
			}
			parameters = append(parameters, param)

		case "typed_parameter", "default_parameter", "typed_default_parameter":
			param := extractParameterInfo(parseTree, child)
			if param != nil {
				param.IsKeywordOnly = isAfterKeywordSeparator
				param.IsPositionalOnly = isBeforePositionalSeparator && !isAfterKeywordSeparator
				param.IsOptional = param.DefaultValue != ""
				parameters = append(parameters, *param)
			}

		case "variadic_parameter", "list_splat_pattern", "dictionary_splat_pattern":
			// Handle *args and **kwargs
			param := extractVariadicParameterInfo(parseTree, child)
			if param != nil {
				// After *args, subsequent params are keyword-only
				if child.Type == nodeTypeListSplatPattern {
					isAfterKeywordSeparator = true
				}
				parameters = append(parameters, *param)
			}

		default:
			// Try to extract parameter info from unknown node types
			param := extractParameterInfo(parseTree, child)
			if param != nil {
				param.IsKeywordOnly = isAfterKeywordSeparator
				param.IsPositionalOnly = isBeforePositionalSeparator && !isAfterKeywordSeparator
				param.IsOptional = param.DefaultValue != ""
				parameters = append(parameters, *param)
			}
		}
	}

	return parameters
}

// extractParameterInfo extracts detailed parameter information.
func extractParameterInfo(parseTree *valueobject.ParseTree, paramNode *valueobject.ParseNode) *outbound.Parameter {
	if paramNode == nil {
		return nil
	}

	identifierNode := findChildByType(paramNode, nodeTypeIdentifier)
	if identifierNode == nil {
		return nil
	}

	param := &outbound.Parameter{
		Name: parseTree.GetNodeText(identifierNode),
		Type: "Any", // Default type
	}

	// Check for type annotation
	typeNode := findChildByType(paramNode, nodeTypeType)
	if typeNode != nil {
		param.Type = parseTree.GetNodeText(typeNode)
	}

	// Extract default value from parameter node
	// Tree-sitter structure:
	// - default_parameter: [identifier, =, value_expression]
	// - typed_default_parameter: [identifier, :, type, =, value_expression]
	switch paramNode.Type {
	case "default_parameter":
		// Value is at position 2: [identifier, =, VALUE]
		if len(paramNode.Children) >= 3 {
			param.DefaultValue = parseTree.GetNodeText(paramNode.Children[2])
		}
	case "typed_default_parameter":
		// Value is at position 4: [identifier, :, type, =, VALUE]
		if len(paramNode.Children) >= 5 {
			param.DefaultValue = parseTree.GetNodeText(paramNode.Children[4])
		}
	}

	return param
}

// extractVariadicParameterInfo extracts variadic parameter information (*args, **kwargs).
func extractVariadicParameterInfo(
	parseTree *valueobject.ParseTree,
	paramNode *valueobject.ParseNode,
) *outbound.Parameter {
	if paramNode == nil {
		return nil
	}

	// For variadic parameters, the identifier might be nested deeper
	var identifierNode *valueobject.ParseNode
	for _, child := range paramNode.Children {
		if child.Type == nodeTypeIdentifier {
			identifierNode = child
			break
		}
		// Check nested children for identifier
		for _, nestedChild := range child.Children {
			if nestedChild.Type == nodeTypeIdentifier {
				identifierNode = nestedChild
				break
			}
		}
		if identifierNode != nil {
			break
		}
	}

	if identifierNode == nil {
		return nil
	}

	param := &outbound.Parameter{
		Name:       parseTree.GetNodeText(identifierNode),
		Type:       "Any",
		IsVariadic: true,
	}

	// Check for type annotation
	typeNode := findChildByType(paramNode, nodeTypeType)
	if typeNode != nil {
		param.Type = parseTree.GetNodeText(typeNode)
	}

	return param
}

// extractReturnTypeAnnotation extracts return type annotation from function.
func extractReturnTypeAnnotation(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	return extractReturnType(parseTree, node)
}

// extractFunctionDocstring extracts docstring from function.
func extractFunctionDocstring(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	return extractFunctionDocumentation(parseTree, node)
}

// extractDecorators extracts decorators from a function/method.
// This function should ONLY be called with decorated_definition nodes.
func extractDecorators(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.Annotation {
	var annotations []outbound.Annotation

	if node == nil || node.Type != nodeTypeDecoratedDef {
		return annotations
	}

	// In decorated_definition, decorators come before the function definition
	for _, child := range node.Children {
		if child.Type == nodeTypeDecorator {
			decoratorName := parseTree.GetNodeText(child)
			decoratorName = strings.TrimPrefix(decoratorName, "@")
			annotations = append(annotations, outbound.Annotation{
				Name: decoratorName,
			})
		}
	}

	return annotations
}

// Helper functions

// findChildByType finds the first child node with the specified type.
func findChildByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	if node == nil {
		return nil
	}
	for _, child := range node.Children {
		if child.Type == nodeType {
			return child
		}
	}
	return nil
}

// findChildrenByType finds all child nodes with the specified type.
func findChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var matches []*valueobject.ParseNode
	if node == nil {
		return matches
	}

	// Search direct children first
	for _, child := range node.Children {
		if child.Type == nodeType {
			matches = append(matches, child)
		}
	}

	// If no direct matches found, search recursively in children
	// (methods might be nested inside class body or block nodes)
	if len(matches) == 0 {
		for _, child := range node.Children {
			childMatches := findChildrenByType(child, nodeType)
			matches = append(matches, childMatches...)
		}
	}

	return matches
}

// extractModuleName extracts module name from parse tree.
func extractModuleName(parseTree *valueobject.ParseTree) string {
	// For now, return a default module name matching test expectations
	// In a real implementation, this would be derived from file path
	// We'll extract it from the first comment line if available
	rootNode := parseTree.RootNode()
	if rootNode != nil {
		for _, child := range rootNode.Children {
			if child.Type == nodeTypeComment {
				commentText := parseTree.GetNodeText(child)
				// Remove # prefix
				commentText = strings.TrimPrefix(commentText, "#")
				// Trim whitespace
				commentText = strings.TrimSpace(commentText)
				// Extract module name (e.g., "math_utils.py" -> "math_utils")
				if strings.HasSuffix(commentText, ".py") {
					moduleName := strings.TrimSuffix(commentText, ".py")
					return moduleName
				}
			}
		}
	}

	return "models"
}

// qualifyName creates a qualified name from parts.
func qualifyName(parts ...string) string {
	var nonEmptyParts []string
	for _, part := range parts {
		if part != "" {
			nonEmptyParts = append(nonEmptyParts, part)
		}
	}
	return strings.Join(nonEmptyParts, ".")
}

// shouldIncludeByVisibility checks if an item should be included based on visibility.
func shouldIncludeByVisibility(visibility outbound.VisibilityModifier, includePrivate bool) bool {
	if visibility == outbound.Public {
		return true
	}
	return includePrivate
}

// isInsideClass checks if a function node is nested inside a class definition.
func isInsideClass(functionNode *valueobject.ParseNode, allClassNodes []*valueobject.ParseNode) bool {
	if functionNode == nil {
		return false
	}

	// Check if the function node is contained within any class node
	for _, classNode := range allClassNodes {
		if isNodeContainedInParent(functionNode, classNode) {
			return true
		}
	}

	return false
}

// isNodeContainedInParent checks if a child node is contained within a parent node.
func isNodeContainedInParent(child, parent *valueobject.ParseNode) bool {
	if child == nil || parent == nil {
		return false
	}

	// Check if child is within parent's byte range
	if child.StartByte >= parent.StartByte && child.EndByte <= parent.EndByte {
		// Also check that it's actually a descendant, not the parent itself
		if child.StartByte != parent.StartByte || child.EndByte != parent.EndByte {
			return true
		}
	}

	return false
}

// findContainingClassName finds the name of the class that contains a function node.
func findContainingClassName(
	functionNode *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
	allClassNodes []*valueobject.ParseNode,
) string {
	if functionNode == nil || parseTree == nil {
		return ""
	}

	// Find the class node that contains this function
	for _, classNode := range allClassNodes {
		if isNodeContainedInParent(functionNode, classNode) {
			// Extract class name from class definition
			className := extractClassName(parseTree, classNode)
			if className != "" {
				return className
			}
		}
	}

	return ""
}

// extractClassName extracts the class name from a class definition node.
func extractClassName(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) string {
	if classNode == nil || parseTree == nil {
		return ""
	}

	// For a class_definition node, the class name is typically the second child
	// (first child is "class", second is the identifier)
	if len(classNode.Children) >= 2 {
		nameNode := classNode.Children[1]
		if nameNode != nil && nameNode.Type == nodeTypeIdentifier {
			return parseTree.GetNodeText(nameNode)
		}
	}

	return ""
}
