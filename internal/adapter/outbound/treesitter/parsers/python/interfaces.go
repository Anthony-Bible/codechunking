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
	decoratedInterfaces, err := processDecoratedInterfaces(ctx, parseTree, options, moduleName, decoratedClassNodes)
	if err != nil {
		return nil, err
	}
	interfaces = append(interfaces, decoratedInterfaces...)

	// Process standalone interfaces
	standaloneInterfaces, err := processStandaloneInterfaces(ctx, parseTree, options, moduleName, decoratedClassNodes)
	if err != nil {
		return nil, err
	}
	interfaces = append(interfaces, standaloneInterfaces...)

	return interfaces, nil
}

// processDecoratedInterfaces handles extraction of interfaces from decorated definitions.
func processDecoratedInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	moduleName string,
	decoratedClassNodes map[*valueobject.ParseNode]bool,
) ([]outbound.SemanticCodeChunk, error) {
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

	return interfaces, nil
}

// processStandaloneInterfaces handles extraction of interfaces that are not part of decorated definitions.
func processStandaloneInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	moduleName string,
	decoratedClassNodes map[*valueobject.ParseNode]bool,
) ([]outbound.SemanticCodeChunk, error) {
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

	return interfaces, nil
}

// createInterfaceFromClass creates a SemanticCodeChunk representing an interface from a class node.
// It handles both decorated and standalone class definitions.
func createInterfaceFromClass(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	additionalDecorators []outbound.Annotation,
) *outbound.SemanticCodeChunk {
	if classNode == nil {
		slogger.Warn(ctx, "Attempted to create interface from nil class node", slogger.Fields{})
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
	allAnnotations := append(additionalDecorators, classAnnotations...)

	// Extract interface methods (abstract methods)
	var childChunks []outbound.SemanticCodeChunk
	if options.MaxDepth > 0 {
		childChunks = extractInterfaceMethods(ctx, parseTree, classNode, className, moduleName, options)
	}

	// Get class content
	content := parseTree.GetNodeText(classNode)

	now := time.Now()
	chunk := &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("interface", className, nil),
		Type:          outbound.ConstructInterface,
		Name:          className,
		QualifiedName: qualifyName(moduleName, className),
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
func extractDecoratorsFromDecoratedDefinition(
	parseTree *valueobject.ParseTree,
	decoratedNode *valueobject.ParseNode,
) []outbound.Annotation {
	var decorators []outbound.Annotation

	// Find all decorator nodes within the decorated_definition
	decoratorNodes := findChildrenByType(decoratedNode, decorator)
	for _, decoratorNode := range decoratorNodes {
		decoratorText := parseTree.GetNodeText(decoratorNode)
		// Clean up the decorator text (remove @ and any parentheses)
		decoratorText = strings.TrimPrefix(decoratorText, "@")
		if idx := strings.Index(decoratorText, "("); idx != -1 {
			decoratorText = decoratorText[:idx]
		}
		decorators = append(decorators, outbound.Annotation{
			Name: decoratorText,
		})
	}

	return decorators
}

// isInterfaceClass determines if a class represents an interface/protocol.
func isInterfaceClass(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	if classNode == nil {
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

// inheritsFromProtocol checks if class inherits from typing.Protocol.
func inheritsFromProtocol(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	// Find argument list (base classes)
	argListNode := findChildByType(classNode, argumentList)
	if argListNode == nil {
		return false
	}

	// Check if any base class is "Protocol"
	for _, child := range argListNode.Children {
		if child.Type == identifier {
			baseName := parseTree.GetNodeText(child)
			if baseName == protocolClassType {
				return true
			}
		}
	}

	return false
}

// inheritsFromABC checks if class inherits from abc.ABC.
func inheritsFromABC(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	// Find argument list (base classes)
	argListNode := findChildByType(classNode, argumentList)
	if argListNode == nil {
		return false
	}

	// Check if any base class is "ABC"
	for _, child := range argListNode.Children {
		if child.Type == identifier {
			baseName := parseTree.GetNodeText(child)
			if baseName == abcClassType {
				return true
			}
		}
	}

	return false
}

// hasAbstractMethods checks if class has abstract methods (decorated with @abstractmethod).
func hasAbstractMethods(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	// Find the class body
	bodyNode := findChildByType(classNode, block)
	if bodyNode == nil {
		return false
	}

	// Look for function definitions with @abstractmethod decorator
	functionNodes := findChildrenByType(bodyNode, functionDefinition)
	for _, funcNode := range functionNodes {
		if hasAbstractMethodDecorator(parseTree, funcNode) {
			return true
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
func extractInterfaceMethods(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	var methods []outbound.SemanticCodeChunk

	// Find the class body
	bodyNode := findChildByType(classNode, block)
	if bodyNode == nil {
		return methods
	}

	// Reduce max depth for child extraction
	childOptions := options
	childOptions.MaxDepth = options.MaxDepth - 1

	// Extract function definitions within the interface
	functionNodes := findChildrenByType(bodyNode, functionDefinition)
	for _, node := range functionNodes {
		method := parseInterfaceMethod(ctx, parseTree, node, className, moduleName, childOptions)
		if method != nil && shouldIncludeByVisibility(method.Visibility, options.IncludePrivate) {
			methods = append(methods, *method)
		}
	}

	return methods
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
