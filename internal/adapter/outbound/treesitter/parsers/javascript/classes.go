package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	nodeTypeIdentifier = "identifier"
)

// ExtractJavaScriptClasses extracts JavaScript classes from the parse tree using real AST analysis.
func ExtractJavaScriptClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	if parseTree == nil {
		return nil, errors.New("parse tree cannot be nil")
	}

	if parseTree.Language().Name() != valueobject.LanguageJavaScript {
		return nil, fmt.Errorf("unsupported language: %s", parseTree.Language().Name())
	}

	classes := make([]outbound.SemanticCodeChunk, 0)
	processedNodes := make(map[*valueobject.ParseNode]bool)

	// Extract module name from source code
	moduleName := extractModuleName(parseTree)
	if moduleName == "" {
		moduleName = "module"
	}

	// Find all class declarations in the parse tree
	classDeclarationNodes := parseTree.GetNodesByType("class_declaration")
	slogger.Info(ctx, "Found class declarations", slogger.Fields{"count": len(classDeclarationNodes)})

	// Find all class expressions in the parse tree (tree-sitter uses "class" for expressions too)
	classExpressionNodes := parseTree.GetNodesByType("class")
	slogger.Info(ctx, "Found class expressions", slogger.Fields{"count": len(classExpressionNodes)})

	// Process class declarations
	for _, classNode := range classDeclarationNodes {
		if classNode != nil && !processedNodes[classNode] {
			class := extractClassFromNode(parseTree, classNode, moduleName, options)
			if class != nil {
				classes = append(classes, *class)
				processedNodes[classNode] = true
			}
		}
	}

	// Process class expressions (filter out those that are part of class_declaration)
	for _, classNode := range classExpressionNodes {
		if classNode != nil && !processedNodes[classNode] && !isPartOfClassDeclaration(parseTree, classNode) {
			class := extractClassFromNode(parseTree, classNode, moduleName, options)
			if class != nil {
				classes = append(classes, *class)
				processedNodes[classNode] = true
			}
		}
	}

	// Find variable declarations with class expressions or mixin factories
	variableDeclarators := parseTree.GetNodesByType("variable_declarator")
	slogger.Info(ctx, "Found variable declarators", slogger.Fields{"count": len(variableDeclarators)})

	for _, varNode := range variableDeclarators {
		if varNode != nil && !processedNodes[varNode] {
			slogger.Info(ctx, "Processing variable declarator", slogger.Fields{
				"node_type":  varNode.Type,
				"start_byte": varNode.StartByte,
				"end_byte":   varNode.EndByte,
			})

			// Try to extract as class expression
			if class := extractClassExpressionFromVariable(parseTree, varNode, moduleName, options); class != nil {
				classes = append(classes, *class)
				processedNodes[varNode] = true
			} else if mixinClass := extractMixinFromVariable(parseTree, varNode, moduleName, options); mixinClass != nil {
				// Try to extract as mixin factory
				classes = append(classes, *mixinClass)
				processedNodes[varNode] = true
			} else {
				slogger.Info(ctx, "No class or mixin found in variable declarator", slogger.Fields{
					"node_type":  varNode.Type,
					"start_byte": varNode.StartByte,
					"end_byte":   varNode.EndByte,
				})
			}
		}
	}

	// Find functions that return classes (mixin pattern detection)
	functionNodes := parseTree.GetNodesByType("function_declaration")
	slogger.Info(ctx, "Found function declarations", slogger.Fields{"count": len(functionNodes)})

	for _, funcNode := range functionNodes {
		if funcNode != nil && !processedNodes[funcNode] {
			if mixinClass := extractMixinClass(parseTree, funcNode, moduleName, options); mixinClass != nil {
				classes = append(classes, *mixinClass)
				processedNodes[funcNode] = true
			}
		}
	}

	arrowFunctionNodes := parseTree.GetNodesByType("arrow_function")
	slogger.Info(ctx, "Found arrow functions", slogger.Fields{"count": len(arrowFunctionNodes)})

	for _, funcNode := range arrowFunctionNodes {
		if funcNode != nil && !processedNodes[funcNode] {
			if mixinClass := extractMixinClass(parseTree, funcNode, moduleName, options); mixinClass != nil {
				classes = append(classes, *mixinClass)
				processedNodes[funcNode] = true
			}
		}
	}

	slogger.Info(ctx, "Final extracted classes count", slogger.Fields{"count": len(classes)})

	return classes, nil
}

// extractClassFromNode extracts a class from a class_declaration or class node.
func extractClassFromNode(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
) *outbound.SemanticCodeChunk {
	if parseTree == nil || node == nil {
		return nil
	}

	// Extract class name
	className := extractClassName(parseTree, node)
	if className == "" {
		return nil
	}

	// Check visibility based on naming conventions
	visibility := getJavaScriptVisibility(className)
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	now := time.Now()
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, className)

	// Extract inheritance info
	var dependencies []outbound.DependencyReference
	if superClass := extractSuperClass(parseTree, node); superClass != "" {
		dependencies = append(dependencies, outbound.DependencyReference{
			Name: superClass,
			Type: "inheritance",
		})
	}

	// Analyze static members and patterns
	metadata := make(map[string]interface{})

	// Always initialize static metadata arrays to prevent nil assertions
	metadata["static_properties"] = extractStaticProperties(parseTree, node)
	metadata["static_methods"] = extractStaticMethods(parseTree, node)

	if hasPrivateStaticMembers(parseTree, node) {
		metadata["has_private_static_members"] = true
	}

	if hasStaticInitBlock(parseTree, node) {
		metadata["has_static_init_block"] = true
	}

	if pattern := detectDesignPattern(parseTree, node); pattern != "" {
		metadata["design_pattern"] = pattern
	}

	// Keep existing private members check
	if hasPrivateMembers(parseTree, node) {
		metadata["has_private_members"] = true
	}

	// Add mixin chain detection
	if mixinChain := extractMixinChain(parseTree, node); len(mixinChain) > 0 {
		metadata["mixin_chain"] = mixinChain
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructClass), className, nil),
		Type:          outbound.ConstructClass,
		Name:          className,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateClassContent(parseTree, node),
		Visibility:    visibility,
		Dependencies:  dependencies,
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// extractMixinClass detects functions that return classes and treats them as mixin classes.
func extractMixinClass(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
) *outbound.SemanticCodeChunk {
	if parseTree == nil || node == nil {
		return nil
	}

	functionName := extractFunctionName(parseTree, node)
	if functionName == "" {
		return nil
	}

	// Check if function returns a class
	if !returnsClass(parseTree, node) {
		return nil
	}

	// Check visibility
	visibility := getJavaScriptVisibility(functionName)
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	now := time.Now()
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, functionName)

	metadata := make(map[string]interface{})
	metadata["returns_class"] = true

	if pattern := detectMixinPattern(parseTree, node); pattern != "" {
		metadata["design_pattern"] = pattern
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructClass), functionName, nil),
		Type:          outbound.ConstructClass,
		Name:          functionName,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContent(parseTree, node),
		Visibility:    visibility,
		Dependencies:  []outbound.DependencyReference{},
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// extractClassExpressionFromVariable handles variable declarations with class expressions.
func extractClassExpressionFromVariable(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
) *outbound.SemanticCodeChunk {
	if parseTree == nil || node == nil {
		return nil
	}

	// Check if this variable declarator contains a class expression
	var classNode *valueobject.ParseNode
	for _, child := range node.Children {
		if child != nil && child.Type == "class" {
			classNode = child
			break
		}
	}

	if classNode == nil {
		slogger.Info(context.Background(), "No class node found in variable declarator children", slogger.Fields{
			"children_count": len(node.Children),
		})
		for i, child := range node.Children {
			slogger.Info(context.Background(), "Child node", slogger.Fields{
				"index": i,
				"type":  child.Type,
			})
		}
		return nil
	}

	slogger.Info(context.Background(), "Found class node in variable declarator", slogger.Fields{
		"class_node_type":  classNode.Type,
		"class_start_byte": classNode.StartByte,
		"class_end_byte":   classNode.EndByte,
	})

	// Extract variable name as class name
	var className string
	if len(node.Children) > 0 && node.Children[0].Type == nodeTypeIdentifier {
		className = parseTree.GetNodeText(node.Children[0])
		slogger.Info(context.Background(), "Extracted class name from identifier", slogger.Fields{
			"class_name": className,
		})
	} else {
		className = "(anonymous class)"
		slogger.Info(context.Background(), "Using default anonymous class name", slogger.Fields{})
	}

	// Check visibility
	visibility := getJavaScriptVisibility(className)
	if !options.IncludePrivate && visibility == outbound.Private {
		slogger.Info(context.Background(), "Skipping private class", slogger.Fields{
			"class_name": className,
		})
		return nil
	}

	now := time.Now()
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, className)

	// Extract inheritance info
	var dependencies []outbound.DependencyReference
	if superClass := extractSuperClass(parseTree, classNode); superClass != "" {
		dependencies = append(dependencies, outbound.DependencyReference{
			Name: superClass,
			Type: "inheritance",
		})
	}

	// Analyze static members and patterns
	metadata := make(map[string]interface{})

	// Always initialize static metadata arrays to prevent nil assertions
	metadata["static_properties"] = extractStaticProperties(parseTree, classNode)
	metadata["static_methods"] = extractStaticMethods(parseTree, classNode)

	if hasPrivateStaticMembers(parseTree, classNode) {
		metadata["has_private_static_members"] = true
	}

	if hasStaticInitBlock(parseTree, classNode) {
		metadata["has_static_init_block"] = true
	}

	if pattern := detectDesignPattern(parseTree, classNode); pattern != "" {
		metadata["design_pattern"] = pattern
	}

	if hasPrivateMembers(parseTree, classNode) {
		metadata["has_private_members"] = true
	}

	// Add mixin chain detection
	if mixinChain := extractMixinChain(parseTree, classNode); len(mixinChain) > 0 {
		metadata["mixin_chain"] = mixinChain
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructClass), className, nil),
		Type:          outbound.ConstructClass,
		Name:          className,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateClassContent(parseTree, classNode),
		Visibility:    visibility,
		Dependencies:  dependencies,
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// extractMixinFromVariable handles variable declarations with arrow functions that return classes.
func extractMixinFromVariable(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
) *outbound.SemanticCodeChunk {
	if parseTree == nil || node == nil {
		return nil
	}

	// Check if this variable declarator contains an arrow function that returns a class
	var arrowFunctionNode *valueobject.ParseNode
	for _, child := range node.Children {
		if child != nil && child.Type == "arrow_function" {
			arrowFunctionNode = child
			break
		}
	}

	if arrowFunctionNode == nil {
		return nil
	}

	// Check if the arrow function returns a class
	if !returnsClass(parseTree, arrowFunctionNode) {
		return nil
	}

	// Extract variable name as mixin name
	var mixinName string
	if len(node.Children) > 0 && node.Children[0].Type == nodeTypeIdentifier {
		mixinName = parseTree.GetNodeText(node.Children[0])
	} else {
		return nil
	}

	// Check visibility
	visibility := getJavaScriptVisibility(mixinName)
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	now := time.Now()
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, mixinName)

	metadata := make(map[string]interface{})
	metadata["returns_class"] = true

	if pattern := detectMixinPattern(parseTree, arrowFunctionNode); pattern != "" {
		metadata["design_pattern"] = pattern
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID(string(outbound.ConstructClass), mixinName, nil),
		Type:          outbound.ConstructClass,
		Name:          mixinName,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateFunctionContent(parseTree, arrowFunctionNode),
		Visibility:    visibility,
		Dependencies:  []outbound.DependencyReference{},
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// extractClassName extracts the class name from a class_declaration or class node.
func extractClassName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	// For class declarations, look for identifier child node
	if node.Type == "class_declaration" {
		for _, child := range node.Children {
			if child != nil && child.Type == nodeTypeIdentifier {
				return parseTree.GetNodeText(child)
			}
		}
	}

	// For class expressions, first check if it's a named class expression
	if node.Type == "class" {
		for _, child := range node.Children {
			if child != nil && child.Type == nodeTypeIdentifier {
				return parseTree.GetNodeText(child)
			}
		}
		// If no name found, check if it's assigned to a variable
		if name := extractClassExpressionName(parseTree, node); name != "" {
			return name
		}
		// If no name found, return default anonymous name
		return "(anonymous class)"
	}

	return ""
}

// extractClassExpressionName tries to find the variable name a class expression is assigned to.
func extractClassExpressionName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	// Find the parent assignment or variable declaration
	parent := findParentAssignmentOrVariableDeclaration(parseTree, node)
	if parent != nil {
		// First child should be the identifier (variable name)
		if len(parent.Children) > 0 && parent.Children[0] != nil && parent.Children[0].Type == nodeTypeIdentifier {
			return parseTree.GetNodeText(parent.Children[0])
		}
	}

	return ""
}

// extractSuperClass extracts the superclass name if the class extends another class.
func extractSuperClass(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	// Look for class_heritage node which contains "extends" information
	for _, child := range node.Children {
		if child != nil && child.Type == "class_heritage" {
			// Find the identifier after "extends"
			for _, heritageChild := range child.Children {
				if heritageChild != nil && heritageChild.Type == nodeTypeIdentifier {
					return parseTree.GetNodeText(heritageChild)
				}
				// Also check for member_expression (e.g., extends ParentClass.Base)
				if heritageChild != nil && heritageChild.Type == "member_expression" {
					return parseTree.GetNodeText(heritageChild)
				}
				// Check for call expressions (mixin patterns)
				if heritageChild != nil && heritageChild.Type == "call_expression" {
					return parseTree.GetNodeText(heritageChild)
				}
			}
		}
	}
	return ""
}

// hasPrivateMembers checks if the class has private fields or methods.
func hasPrivateMembers(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	if parseTree == nil || node == nil {
		return false
	}

	content := parseTree.GetNodeText(node)
	if content == "" {
		return false
	}
	// Check for ES2022 private fields (#) or convention-based private (_)
	return strings.Contains(content, "#") || strings.Contains(content, "private ")
}

// generateClassContent generates a content summary for the class.
func generateClassContent(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	content := parseTree.GetNodeText(node)
	if content == "" {
		return ""
	}

	// Truncate for display purposes
	const maxContentLength = 200
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

// generateFunctionContent generates a content summary for a function.
func generateFunctionContent(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	content := parseTree.GetNodeText(node)
	if content == "" {
		return ""
	}

	// Truncate for display purposes
	const maxContentLength = 200
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

// extractStaticProperties extracts static property names from the class.
func extractStaticProperties(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []string {
	var properties []string

	if parseTree == nil || node == nil {
		return properties
	}

	// Find class body
	classBody := findClassBody(node)
	if classBody == nil {
		return properties
	}

	// Look for static field definitions
	for _, child := range classBody.Children {
		if child != nil && child.Type == "field_definition" {
			// Check if it has static modifier
			if hasStaticModifier(parseTree, child) {
				if propName := extractPropertyName(parseTree, child); propName != "" &&
					!strings.HasPrefix(propName, "#") {
					properties = append(properties, propName)
				}
			}
		}
	}

	return properties
}

// extractStaticMethods extracts static method names from the class.
func extractStaticMethods(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []string {
	var methods []string

	if parseTree == nil || node == nil {
		return methods
	}

	// Find class body
	classBody := findClassBody(node)
	if classBody == nil {
		return methods
	}

	// Look for method definitions
	for _, child := range classBody.Children {
		if child != nil && child.Type == "method_definition" {
			// Check if it has static modifier
			if hasStaticModifier(parseTree, child) {
				if methodName := extractClassMethodName(parseTree, child); methodName != "" &&
					!strings.HasPrefix(methodName, "#") {
					methods = append(methods, methodName)
				}
			}
		}
	}

	return methods
}

// hasPrivateStaticMembers checks if the class has private static fields or methods (using # prefix).
func hasPrivateStaticMembers(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	if parseTree == nil || node == nil {
		return false
	}

	content := parseTree.GetNodeText(node)
	if content == "" {
		return false
	}

	// Look for static private fields/methods pattern: static #field
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "static") && strings.Contains(trimmed, "#") {
			return true
		}
	}
	return false
}

// hasStaticInitBlock checks if the class has static initialization blocks.
func hasStaticInitBlock(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	if parseTree == nil || node == nil {
		return false
	}

	// Find class body
	classBody := findClassBody(node)
	if classBody == nil {
		return false
	}

	// Look for static blocks
	for _, child := range classBody.Children {
		if child != nil && child.Type == "class_static_block" {
			return true
		}
	}

	return false
}

// detectDesignPattern detects common design patterns in the class.
func detectDesignPattern(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	content := parseTree.GetNodeText(node)
	if content == "" {
		return ""
	}

	// Detect Singleton pattern
	// Look for: static #instance, getInstance method, constructor that returns existing instance
	if strings.Contains(content, "#instance") &&
		(strings.Contains(content, "getInstance") || strings.Contains(content, "getinstance")) &&
		strings.Contains(content, "static") {
		return "singleton"
	}

	return ""
}

// detectMixinPattern detects mixin patterns in functions.
func detectMixinPattern(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	content := parseTree.GetNodeText(node)
	if content == "" {
		return ""
	}

	// Detect mixin pattern: function that returns a class
	if strings.Contains(content, "return class") {
		return "mixin"
	}

	return ""
}

// extractMixinChain detects mixin chains in class heritage.
func extractMixinChain(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []string {
	if parseTree == nil || node == nil {
		return []string{}
	}

	// Look for class_heritage node which contains "extends" information
	for _, child := range node.Children {
		if child != nil && child.Type == "class_heritage" {
			return extractMixinChainFromHeritage(parseTree, child)
		}
	}

	return []string{}
}

// extractMixinChainFromHeritage extracts mixin chain from a class_heritage node.
func extractMixinChainFromHeritage(parseTree *valueobject.ParseTree, heritageNode *valueobject.ParseNode) []string {
	if parseTree == nil || heritageNode == nil {
		return []string{}
	}

	var mixins []string

	// Look for call expressions which represent mixin patterns
	for _, child := range heritageNode.Children {
		if child != nil && child.Type == "call_expression" {
			mixins = append(mixins, extractMixinFromCallExpression(parseTree, child)...)
		}
	}

	return mixins
}

// extractMixinFromCallExpression extracts mixin names from nested call expressions.
func extractMixinFromCallExpression(parseTree *valueobject.ParseTree, callNode *valueobject.ParseNode) []string {
	if parseTree == nil || callNode == nil {
		return []string{}
	}

	var mixins []string

	for _, child := range callNode.Children {
		if child != nil {
			switch child.Type {
			case "identifier":
				// Direct mixin name
				name := parseTree.GetNodeText(child)
				if name != "" {
					mixins = append(mixins, name)
				}
			case "call_expression":
				// Nested mixin call
				mixins = append(mixins, extractMixinFromCallExpression(parseTree, child)...)
			}
		}
	}

	return mixins
}

// Helper functions for parsing tree-sitter nodes

// findClassBody finds the class_body node within a class.
func findClassBody(node *valueobject.ParseNode) *valueobject.ParseNode {
	if node == nil {
		return nil
	}

	for _, child := range node.Children {
		if child != nil && child.Type == "class_body" {
			return child
		}
	}
	return nil
}

// hasStaticModifier checks if a node has the static modifier.
func hasStaticModifier(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	if parseTree == nil || node == nil {
		return false
	}

	for _, child := range node.Children {
		if child != nil &&
			(child.Type == "static" || (child.Type == nodeTypeIdentifier && parseTree.GetNodeText(child) == "static")) {
			return true
		}
	}
	return false
}

// extractPropertyName extracts the property name from a field definition.
func extractPropertyName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	for _, child := range node.Children {
		if child != nil && (child.Type == "property_identifier" || child.Type == "private_property_identifier") {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
}

// extractClassMethodName extracts the method name from a method definition.
func extractClassMethodName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	if parseTree == nil || node == nil {
		return ""
	}

	for _, child := range node.Children {
		if child != nil && (child.Type == "property_identifier" || child.Type == "private_property_identifier") {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
}

// findParentAssignmentOrVariableDeclaration finds the parent assignment or variable declaration node.
func findParentAssignmentOrVariableDeclaration(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) *valueobject.ParseNode {
	if parseTree == nil || node == nil {
		return nil
	}

	// Since ParseNode doesn't have a Parent field, we need to traverse from root
	// and find the node that contains our target node
	return findContainingNode(parseTree.RootNode(), node)
}

// findContainingNode recursively searches for a node that contains the target node.
func findContainingNode(currentNode *valueobject.ParseNode, targetNode *valueobject.ParseNode) *valueobject.ParseNode {
	if currentNode == nil {
		return nil
	}

	// Check if currentNode is the parent we're looking for
	if isParentAssignmentOrVariableDeclaration(currentNode, targetNode) {
		return currentNode
	}

	// Recursively check children
	for _, child := range currentNode.Children {
		if result := findContainingNode(child, targetNode); result != nil {
			return result
		}
	}

	return nil
}

// isParentAssignmentOrVariableDeclaration checks if currentNode is a parent of targetNode
// and is either a variable_declarator or assignment_expression.
func isParentAssignmentOrVariableDeclaration(
	currentNode *valueobject.ParseNode,
	targetNode *valueobject.ParseNode,
) bool {
	if currentNode == nil || targetNode == nil {
		return false
	}

	// Check if currentNode is one of the target types
	if currentNode.Type != "variable_declarator" && currentNode.Type != "assignment_expression" {
		return false
	}

	// Check if targetNode is a child of currentNode
	for _, child := range currentNode.Children {
		if child == targetNode {
			return true
		}
	}

	return false
}

// returnsClass checks if a function returns a class.
func returnsClass(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	if parseTree == nil || node == nil {
		return false
	}

	content := parseTree.GetNodeText(node)
	if content == "" {
		return false
	}

	// Check for explicit return class pattern
	if strings.Contains(content, "return class") {
		return true
	}

	// Check for arrow function that directly returns a class (e.g., () => class {...})
	if node.Type == "arrow_function" {
		// Look for class nodes within the arrow function body
		return containsClassInBody(node)
	}

	return false
}

// containsClassInBody recursively checks if a function body contains a class.
func containsClassInBody(node *valueobject.ParseNode) bool {
	if node == nil {
		return false
	}

	// Direct class node
	if node.Type == "class" {
		return true
	}

	// Check children recursively
	for _, child := range node.Children {
		if containsClassInBody(child) {
			return true
		}
	}

	return false
}

// isPartOfClassDeclaration checks if a class node is part of a class_declaration (not a standalone class expression).
func isPartOfClassDeclaration(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	if parseTree == nil || classNode == nil {
		return false
	}

	// Search for a parent class_declaration node
	return findParentOfType(parseTree.RootNode(), classNode, "class_declaration") != nil
}

// findParentOfType recursively searches for a parent node of the specified type that contains the target node.
func findParentOfType(
	currentNode *valueobject.ParseNode,
	targetNode *valueobject.ParseNode,
	targetType string,
) *valueobject.ParseNode {
	if currentNode == nil {
		return nil
	}

	// Check if currentNode is the target type and contains the targetNode
	if currentNode.Type == targetType && containsNode(currentNode, targetNode) {
		return currentNode
	}

	// Recursively check children
	for _, child := range currentNode.Children {
		if result := findParentOfType(child, targetNode, targetType); result != nil {
			return result
		}
	}

	return nil
}

// containsNode checks if a parent node contains a target node anywhere in its subtree.
func containsNode(parentNode *valueobject.ParseNode, targetNode *valueobject.ParseNode) bool {
	if parentNode == nil || targetNode == nil {
		return false
	}

	// Direct match
	if parentNode == targetNode {
		return true
	}

	// Check children recursively
	for _, child := range parentNode.Children {
		if containsNode(child, targetNode) {
			return true
		}
	}

	return false
}
