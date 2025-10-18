package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

const (
	protocolClassType   = "Protocol"
	abcClassType        = "ABC"
	abstractmethodType  = "abstractmethod"
	decoratedDefinition = "decorated_definition"
	classDefinition     = "class_definition"
	functionDefinition  = "function_definition"
	argumentList        = "argument_list"
	identifier          = "identifier"
	block               = "block"
	decorator           = "decorator"
	expressionStatement = "expression_statement"
	ellipsis            = "ellipsis"
	passStatement       = "pass_statement"
)

// extractPythonInterfaces extracts Python protocols/interfaces from the parse tree.
func extractPythonInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Starting Python interface extraction", slogger.Fields{})

	startTime := time.Now()
	defer func() {
		slogger.Info(ctx, "Completed Python interface extraction", slogger.Fields{"duration": time.Since(startTime)})
	}()

	var interfaces []outbound.SemanticCodeChunk
	moduleName := extractModuleName(parseTree)

	// Keep track of class nodes that are inside decorated definitions
	decoratedClassNodes := make(map[*valueobject.ParseNode]bool)

	// Process decorated interfaces first
	decoratedInterfaces := processDecoratedInterfaces(ctx, parseTree, options, moduleName, decoratedClassNodes)
	interfaces = append(interfaces, decoratedInterfaces...)

	// Process standalone interfaces
	standaloneInterfaces := processStandaloneInterfaces(ctx, parseTree, options, moduleName, decoratedClassNodes)
	interfaces = append(interfaces, standaloneInterfaces...)

	return interfaces, nil
}

// processDecoratedInterfaces handles extraction of interfaces from decorated definitions.
//
// Decorated definitions have the AST structure:
//
//	decorated_definition
//	├── decorator (e.g., @runtime_checkable)
//	└── class_definition (the actual class)
//
// This function:
//  1. Finds all decorated_definition nodes
//  2. Extracts class_definition child from each
//  3. Filters to only interface classes (Protocol, ABC, or abstract methods)
//  4. Extracts decorators and creates interface chunks
//  5. Tracks processed class nodes to prevent duplicate extraction
//
// Returns empty slice if no decorated interfaces are found.
func processDecoratedInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	moduleName string,
	decoratedClassNodes map[*valueobject.ParseNode]bool,
) []outbound.SemanticCodeChunk {
	if parseTree == nil || decoratedClassNodes == nil {
		return nil
	}

	var interfaces []outbound.SemanticCodeChunk

	decoratedNodes := parseTree.GetNodesByType(decoratedDefinition)
	slogger.Debug(ctx, "Found decorated definitions", slogger.Fields{"decorated_count": len(decoratedNodes)})

	for _, node := range decoratedNodes {
		classChild := findChildByType(node, classDefinition)
		if classChild == nil {
			continue
		}

		if !isInterfaceClass(parseTree, classChild) {
			continue
		}

		decorators := extractDecoratorsFromDecoratedDefinition(parseTree, node)
		interfaceChunk := createInterfaceFromClass(ctx, parseTree, classChild, moduleName, options, decorators)
		if interfaceChunk != nil && shouldIncludeByVisibility(interfaceChunk.Visibility, options.IncludePrivate) {
			interfaces = append(interfaces, *interfaceChunk)
			decoratedClassNodes[classChild] = true
		}
	}

	return interfaces
}

// processStandaloneInterfaces handles extraction of interfaces that are not part of decorated definitions.
//
// This function processes plain class definitions like:
//
//	class Foo(Protocol): ...
//	class Bar(ABC): ...
//
// It skips any classes that were already processed as part of decorated_definition nodes
// to prevent duplicate extraction.
//
// Returns empty slice if:
//   - parseTree is nil
//   - decoratedClassNodes is nil
//   - no standalone interface classes are found
func processStandaloneInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	moduleName string,
	decoratedClassNodes map[*valueobject.ParseNode]bool,
) []outbound.SemanticCodeChunk {
	if parseTree == nil || decoratedClassNodes == nil {
		return nil
	}

	var interfaces []outbound.SemanticCodeChunk

	classNodes := parseTree.GetNodesByType(classDefinition)
	slogger.Debug(ctx, "Found class definitions", slogger.Fields{"class_count": len(classNodes)})

	for _, node := range classNodes {
		// Skip if this class node was already processed as part of a decorated definition
		if decoratedClassNodes[node] {
			continue
		}

		if !isInterfaceClass(parseTree, node) {
			continue
		}

		interfaceChunk := createInterfaceFromClass(ctx, parseTree, node, moduleName, options, nil)
		if interfaceChunk != nil && shouldIncludeByVisibility(interfaceChunk.Visibility, options.IncludePrivate) {
			interfaces = append(interfaces, *interfaceChunk)
		}
	}

	return interfaces
}

// createInterfaceFromClass creates a SemanticCodeChunk representing an interface from a class node.
//
// This function extracts all relevant interface metadata including:
//   - Class name and qualified name (with special nesting rules)
//   - Documentation (docstrings)
//   - Inheritance relationships (dependencies)
//   - Decorators (both class-level and from decorated_definition)
//   - Child methods (abstract and concrete)
//
// Handles both:
//   - Decorated definitions: @decorator class Foo(Protocol): ...
//   - Standalone classes: class Foo(Protocol): ...
//
// Returns nil if:
//   - classNode is nil
//   - parseTree is nil (via helper functions)
//   - class name cannot be extracted
func createInterfaceFromClass(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	additionalDecorators []outbound.Annotation,
) *outbound.SemanticCodeChunk {
	if classNode == nil || parseTree == nil {
		slogger.Warn(ctx, "Attempted to create interface from nil class or parse tree", slogger.Fields{})
		return nil
	}

	// Extract class name
	className := extractClassNameFromNode(parseTree, classNode)
	if className == "" {
		slogger.Warn(ctx, "Could not extract class name for interface", slogger.Fields{})
		return nil
	}

	// Extract class documentation
	var documentation string
	if options.IncludeDocumentation {
		documentation = extractClassDocstring(parseTree, classNode)
	}

	// Extract inheritance information
	dependencies := extractInheritanceDependencies(parseTree, classNode)

	// Extract class decorators
	classAnnotations := extractClassDecorators(parseTree, classNode)

	// Combine class decorators with additional decorators
	allAnnotations := make([]outbound.Annotation, 0, len(additionalDecorators)+len(classAnnotations))
	allAnnotations = append(allAnnotations, additionalDecorators...)
	allAnnotations = append(allAnnotations, classAnnotations...)

	// Extract interface methods (abstract methods)
	var childChunks []outbound.SemanticCodeChunk
	if options.MaxDepth > 0 {
		childChunks = extractInterfaceMethods(ctx, parseTree, classNode, className, moduleName, options)
	}

	// Get class content
	content := parseTree.GetNodeText(classNode)

	// Build qualified name for interface (different from classes - omits module prefix when nested)
	qualifiedName := buildQualifiedNameForInterface(parseTree, classNode, moduleName)

	now := time.Now()
	chunk := &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("interface", className, nil),
		Type:          outbound.ConstructInterface,
		Name:          className,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     classNode.StartByte,
		EndByte:       classNode.EndByte,
		Content:       content,
		Documentation: documentation,
		Visibility:    getPythonVisibility(className),
		Dependencies:  dependencies,
		Annotations:   allAnnotations,
		ChildChunks:   childChunks,
		IsAbstract:    true, // Interfaces are abstract by nature
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}

	slogger.Debug(ctx, "Created interface chunk", slogger.Fields{"interface_name": className})
	return chunk
}

// extractDecoratorsFromDecoratedDefinition extracts decorators from a decorated_definition node.
//
// Processes decorator nodes and converts them to Annotation objects:
//   - Strips @ prefix from decorator names
//   - Removes parameter lists (e.g., "@runtime_checkable()" -> "runtime_checkable")
//   - Preserves decorator order as they appear in source code
//
// Returns empty slice if:
//   - decoratedNode is nil
//   - parseTree is nil
//   - no decorators are found
func extractDecoratorsFromDecoratedDefinition(
	parseTree *valueobject.ParseTree,
	decoratedNode *valueobject.ParseNode,
) []outbound.Annotation {
	if decoratedNode == nil || parseTree == nil {
		return nil
	}

	var decorators []outbound.Annotation

	// Find all decorator nodes within the decorated_definition
	decoratorNodes := findChildrenByType(decoratedNode, decorator)
	for _, decoratorNode := range decoratorNodes {
		decoratorText := parseTree.GetNodeText(decoratorNode)

		// Clean up the decorator text (remove @ and any parentheses)
		decoratorText = strings.TrimPrefix(decoratorText, "@")
		decoratorText = strings.TrimSpace(decoratorText)

		// Strip parameter list if present
		if idx := strings.Index(decoratorText, "("); idx != -1 {
			decoratorText = decoratorText[:idx]
		}

		// Skip empty decorator names (defensive check)
		if decoratorText == "" {
			continue
		}

		decorators = append(decorators, outbound.Annotation{
			Name: decoratorText,
		})
	}

	return decorators
}

// isInterfaceClass determines if a class represents an interface/protocol.
//
// A class is considered an interface if it meets any of these criteria:
//  1. Inherits from typing.Protocol (structural typing interface)
//  2. Inherits from abc.ABC (abstract base class)
//  3. Contains methods decorated with @abstractmethod
//
// Returns false if:
//   - classNode is nil
//   - parseTree is nil
//   - none of the interface criteria are met
func isInterfaceClass(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	if classNode == nil || parseTree == nil {
		return false
	}

	// Check if class inherits from Protocol
	if inheritsFromProtocol(parseTree, classNode) {
		return true
	}

	// Check if class inherits from ABC
	if inheritsFromABC(parseTree, classNode) {
		return true
	}

	// Check if class has abstract methods
	if hasAbstractMethods(parseTree, classNode) {
		return true
	}

	return false
}

// extractBaseClassName extracts the base class name from an inheritance expression node.
//
// Handles different AST node types representing inheritance patterns:
//   - identifier: "Protocol" -> "Protocol"
//   - subscript: "Protocol[T, U]" -> "Protocol" (generic parameters stripped)
//   - attribute: "typing.Protocol" -> "Protocol" (module prefix stripped)
//
// Returns empty string for:
//   - nil nodes
//   - unsupported node types (e.g., punctuation)
//   - malformed expressions
func extractBaseClassName(parseTree *valueobject.ParseTree, exprNode *valueobject.ParseNode) string {
	if exprNode == nil || parseTree == nil {
		return ""
	}

	switch exprNode.Type {
	case identifier:
		// Simple case: class Foo(Protocol)
		return parseTree.GetNodeText(exprNode)

	case "subscript":
		// Generic case: class Foo(Protocol[T, U])
		// The base class name is in the 'value' child (first child that's an identifier)
		for _, child := range exprNode.Children {
			if child.Type == identifier {
				return parseTree.GetNodeText(child)
			}
		}
		return ""

	case "attribute":
		// Qualified case: class Foo(typing.Protocol)
		// Extract the rightmost identifier (e.g., "Protocol" from "typing.Protocol")
		text := parseTree.GetNodeText(exprNode)
		parts := strings.Split(text, ".")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return text

	default:
		// For any other node type, return empty string (e.g., punctuation like commas)
		return ""
	}
}

// inheritsFromProtocol checks if class inherits from typing.Protocol.
// Handles multiple inheritance patterns:
//   - Simple: class Foo(Protocol)
//   - Generic: class Foo(Protocol[T, U])
//   - Qualified: class Foo(typing.Protocol)
func inheritsFromProtocol(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	// Find argument list (base classes)
	argListNode := findChildByType(classNode, argumentList)
	if argListNode == nil {
		return false
	}

	// Check if any base class is "Protocol" (in any form)
	for _, child := range argListNode.Children {
		baseName := extractBaseClassName(parseTree, child)
		if baseName == protocolClassType {
			return true
		}
	}

	return false
}

// inheritsFromABC checks if class inherits from abc.ABC.
// Handles multiple inheritance patterns:
//   - Simple: class Foo(ABC)
//   - Qualified: class Foo(abc.ABC)
func inheritsFromABC(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	// Find argument list (base classes)
	argListNode := findChildByType(classNode, argumentList)
	if argListNode == nil {
		return false
	}

	// Check if any base class is "ABC" (in any form)
	for _, child := range argListNode.Children {
		baseName := extractBaseClassName(parseTree, child)
		if baseName == abcClassType {
			return true
		}
	}

	return false
}

// hasAbstractMethods checks if class has abstract methods (decorated with @abstractmethod).
// Handles both plain functions and decorated functions.
func hasAbstractMethods(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	// Find the class body
	bodyNode := findChildByType(classNode, block)
	if bodyNode == nil {
		return false
	}

	// Check plain function definitions with @abstractmethod decorator
	functionNodes := findChildrenByType(bodyNode, functionDefinition)
	for _, funcNode := range functionNodes {
		if hasAbstractMethodDecorator(parseTree, funcNode) {
			return true
		}
	}

	// Check decorated definitions for @abstractmethod
	decoratedNodes := findChildrenByType(bodyNode, decoratedDefinition)
	for _, decoratedNode := range decoratedNodes {
		// Check if the decorated_definition has @abstractmethod decorator
		decorators := extractDecoratorsFromDecoratedDefinition(parseTree, decoratedNode)
		for _, decorator := range decorators {
			if decorator.Name == abstractmethodType {
				return true
			}
		}
	}

	return false
}

// hasAbstractMethodDecorator checks if a function has @abstractmethod decorator.
func hasAbstractMethodDecorator(parseTree *valueobject.ParseTree, funcNode *valueobject.ParseNode) bool {
	// Look for decorator nodes before the function
	for _, child := range funcNode.Children {
		if child.Type == decorator {
			decoratorText := parseTree.GetNodeText(child)
			if strings.Contains(decoratorText, abstractmethodType) {
				return true
			}
		}
	}
	return false
}

// extractInterfaceMethods extracts methods from an interface/protocol class.
//
// This function handles both types of method definitions:
//  1. Plain function definitions: def method(self): ...
//  2. Decorated function definitions: @abstractmethod, @property, @staticmethod, etc.
//
// The function ensures no duplicate extraction by tracking decorated nodes separately.
//
// Returns empty slice if:
//   - classNode is nil
//   - parseTree is nil
//   - class has no method body
//   - options.MaxDepth is 0 or negative
func extractInterfaceMethods(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	if classNode == nil || parseTree == nil {
		return nil
	}

	var methods []outbound.SemanticCodeChunk

	// Find the class body
	bodyNode := findChildByType(classNode, block)
	if bodyNode == nil {
		return methods
	}

	// Reduce max depth for child extraction
	childOptions := options
	childOptions.MaxDepth = options.MaxDepth - 1

	// Get decorated definitions for filtering
	decoratedNodes := findChildrenByType(bodyNode, decoratedDefinition)

	// Extract plain function definitions (not inside decorated_definition)
	functionNodes := findChildrenByType(bodyNode, functionDefinition)
	for _, node := range functionNodes {
		// Skip if this function is inside a decorated definition
		if isNodeInDecoratedDefinition(node, decoratedNodes) {
			continue
		}
		method := parseInterfaceMethod(ctx, parseTree, node, className, moduleName, childOptions)
		if method != nil && shouldIncludeByVisibility(method.Visibility, options.IncludePrivate) {
			methods = append(methods, *method)
		}
	}

	// Process decorated function definitions
	for _, decoratedNode := range decoratedNodes {
		method := extractDecoratedInterfaceMethod(ctx, parseTree, decoratedNode, className, moduleName, childOptions)
		if method != nil && shouldIncludeByVisibility(method.Visibility, options.IncludePrivate) {
			methods = append(methods, *method)
		}
	}

	return methods
}

// extractDecoratedInterfaceMethod extracts a decorated method from a decorated_definition node.
func extractDecoratedInterfaceMethod(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	decoratedNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
) *outbound.SemanticCodeChunk {
	// Look for function definition within decorated_definition
	functionNode := findChildByType(decoratedNode, functionDefinition)
	if functionNode == nil {
		return nil
	}

	// Extract decorators from the decorated_definition
	decorators := extractDecoratorsFromDecoratedDefinition(parseTree, decoratedNode)

	// Parse the method
	method := parseInterfaceMethod(ctx, parseTree, functionNode, className, moduleName, options)
	if method == nil {
		return nil
	}

	// Merge decorators from decorated_definition
	method.Annotations = append(decorators, method.Annotations...)

	// Check if method is abstract based on decorators
	for _, ann := range method.Annotations {
		if ann.Name == abstractmethodType {
			method.IsAbstract = true
			break
		}
	}

	return method
}

// parseInterfaceMethod parses a method within an interface/protocol.
func parseInterfaceMethod(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
) *outbound.SemanticCodeChunk {
	if node == nil {
		slogger.Warn(ctx, "Attempted to parse nil interface method node", slogger.Fields{})
		return nil
	}

	// Extract method name
	nameNode := findChildByType(node, identifier)
	if nameNode == nil {
		slogger.Warn(ctx, "Could not find method name node", slogger.Fields{})
		return nil
	}
	methodName := parseTree.GetNodeText(nameNode)

	// Extract parameters
	parameters := extractFunctionParameters(parseTree, node)

	// Extract return type annotation
	returnType := extractReturnTypeAnnotation(parseTree, node)

	// Extract documentation
	var documentation string
	if options.IncludeDocumentation {
		documentation = extractFunctionDocstring(parseTree, node)
	}

	// Extract decorators
	annotations := extractDecorators(parseTree, node)

	// Get method content
	content := parseTree.GetNodeText(node)

	// Check if method is abstract
	isAbstract := hasAbstractMethodDecorator(parseTree, node)

	now := time.Now()
	chunk := &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("interface_method", methodName, nil),
		Type:          outbound.ConstructMethod,
		Name:          methodName,
		QualifiedName: qualifyName(moduleName, className, methodName),
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       content,
		Documentation: documentation,
		Visibility:    getPythonVisibility(methodName),
		Parameters:    parameters,
		ReturnType:    returnType,
		Annotations:   annotations,
		IsAbstract:    isAbstract,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}

	slogger.Debug(ctx, "Parsed interface method", slogger.Fields{"method_name": methodName})
	return chunk
}

// isProtocolMethod checks if a method belongs to a Protocol-based interface.
func isProtocolMethod(parseTree *valueobject.ParseTree, methodNode *valueobject.ParseNode) bool {
	// Look for ellipsis (...) in method body (Protocol method signature)
	bodyNode := findChildByType(methodNode, block)
	if bodyNode == nil {
		return false
	}

	for _, child := range bodyNode.Children {
		if child.Type == expressionStatement {
			ellipsisNode := findChildByType(child, ellipsis)
			if ellipsisNode != nil {
				return true
			}
		}
	}

	return false
}

// hasOnlyPassStatement checks if a method has only pass statement (common in interfaces).
func hasOnlyPassStatement(parseTree *valueobject.ParseTree, methodNode *valueobject.ParseNode) bool {
	bodyNode := findChildByType(methodNode, block)
	if bodyNode == nil {
		return false
	}

	// Check if body contains only a pass statement
	passCount := 0
	totalStatements := 0

	for _, child := range bodyNode.Children {
		if child.Type == passStatement {
			passCount++
		}
		if child.Type != "comment" && child.Type != "newline" {
			totalStatements++
		}
	}

	return passCount == totalStatements && totalStatements == 1
}

// buildQualifiedNameForInterface builds a qualified name for an interface/protocol.
//
// Qualified name format rules:
//   - Nested in class(es): "ParentClass.ChildClass.InterfaceName" (no module prefix)
//   - Nested in function: "function_name.InterfaceName" (no module prefix)
//   - Top-level: "module.InterfaceName" (includes module prefix)
//
// Examples:
//   - class Outer: class Inner(Protocol): pass  -> "Outer.Inner"
//   - def func(): class Local(Protocol): pass   -> "func.Local"
//   - class TopLevel(Protocol): pass            -> "models.TopLevel"
//
// This differs from class qualified names, which always include the module prefix.
func buildQualifiedNameForInterface(
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	moduleName string,
) string {
	if classNode == nil || parseTree == nil {
		return moduleName
	}

	// Get class name
	className := extractClassNameFromNode(parseTree, classNode)
	if className == "" {
		return moduleName
	}

	// Find parent classes
	parentClasses := findParentClasses(parseTree, classNode)

	// If nested in classes, use class hierarchy without module
	if len(parentClasses) > 0 {
		parts := make([]string, 0, len(parentClasses)+1)
		parts = append(parts, parentClasses...)
		parts = append(parts, className)
		return qualifyName(parts...)
	}

	// Find parent function
	parentFunction := findParentNode(parseTree, classNode, functionDefinition, extractFunctionName)
	if parentFunction != "" {
		return qualifyName(parentFunction, className)
	}

	// Top-level interface - include module
	return qualifyName(moduleName, className)
}

// findParentNode finds the parent node of a specified type that contains the given node.
// It uses an extraction function to extract the name from the parent node.
// Returns empty string if no parent is found.
//
// This is a generic helper for finding parent functions, classes, or other enclosing scopes.
func findParentNode(
	parseTree *valueobject.ParseTree,
	childNode *valueobject.ParseNode,
	parentNodeType string,
	extractName func(*valueobject.ParseTree, *valueobject.ParseNode) string,
) string {
	if childNode == nil || parseTree == nil {
		return ""
	}

	// Get all nodes of the specified type
	allNodes := parseTree.GetNodesByType(parentNodeType)

	// Find node that contains this child
	for _, node := range allNodes {
		// Check if node contains childNode
		if childNode.StartByte > node.StartByte && childNode.EndByte <= node.EndByte {
			// Extract and return the name
			return extractName(parseTree, node)
		}
	}

	return ""
}
