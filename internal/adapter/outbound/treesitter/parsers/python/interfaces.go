package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// extractPythonInterfaces extracts Python protocols/interfaces from the parse tree.
func extractPythonInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var interfaces []outbound.SemanticCodeChunk
	now := time.Now()
	moduleName := extractModuleName(parseTree)

	// Find class definitions that represent interfaces/protocols
	classNodes := parseTree.GetNodesByType("class_definition")
	for _, node := range classNodes {
		if isInterfaceClass(parseTree, node) {
			interfaceChunk := parseInterfaceClass(ctx, parseTree, node, moduleName, options, now)
			if interfaceChunk != nil && shouldIncludeByVisibility(interfaceChunk.Visibility, options.IncludePrivate) {
				interfaces = append(interfaces, *interfaceChunk)
			}
		}
	}

	return interfaces, nil
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

	// Check if class inherits from ABC (Abstract Base Class)
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
	argListNode := findChildByType(classNode, "argument_list")
	if argListNode == nil {
		return false
	}

	// Check if any base class is "Protocol"
	for _, child := range argListNode.Children {
		if child.Type == "identifier" {
			baseName := parseTree.GetNodeText(child)
			if baseName == "Protocol" {
				return true
			}
		}
	}

	return false
}

// inheritsFromABC checks if class inherits from abc.ABC.
func inheritsFromABC(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	// Find argument list (base classes)
	argListNode := findChildByType(classNode, "argument_list")
	if argListNode == nil {
		return false
	}

	// Check if any base class is "ABC"
	for _, child := range argListNode.Children {
		if child.Type == "identifier" {
			baseName := parseTree.GetNodeText(child)
			if baseName == "ABC" {
				return true
			}
		}
	}

	return false
}

// hasAbstractMethods checks if class has abstract methods (decorated with @abstractmethod).
func hasAbstractMethods(parseTree *valueobject.ParseTree, classNode *valueobject.ParseNode) bool {
	// Find the class body
	bodyNode := findChildByType(classNode, "block")
	if bodyNode == nil {
		return false
	}

	// Look for function definitions with @abstractmethod decorator
	functionNodes := findChildrenByType(bodyNode, "function_definition")
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
	// This is a simplified implementation
	for _, child := range funcNode.Children {
		if child.Type == "decorator" {
			decoratorText := parseTree.GetNodeText(child)
			if strings.Contains(decoratorText, "abstractmethod") {
				return true
			}
		}
	}
	return false
}

// parseInterfaceClass parses a class that represents an interface/protocol.
func parseInterfaceClass(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if classNode == nil {
		return nil
	}

	// Extract class name
	className := extractClassNameFromNode(parseTree, classNode)
	if className == "" {
		return nil
	}

	// Extract class documentation
	var documentation string
	if options.IncludeDocumentation {
		documentation = extractClassDocstring(parseTree, classNode)
	}

	// Extract inheritance information
	dependencies := extractInheritanceDependencies(parseTree, classNode)

	// Extract decorators
	annotations := extractClassDecorators(parseTree, classNode)

	// Extract interface methods (abstract methods)
	var childChunks []outbound.SemanticCodeChunk
	if options.MaxDepth > 0 {
		childChunks = extractInterfaceMethods(ctx, parseTree, classNode, className, moduleName, options, now)
	}

	// Get class content
	content := parseTree.GetNodeText(classNode)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("interface", className, nil),
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
		Annotations:   annotations,
		ChildChunks:   childChunks,
		IsAbstract:    true, // Interfaces are abstract by nature
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// extractInterfaceMethods extracts methods from an interface/protocol class.
func extractInterfaceMethods(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var methods []outbound.SemanticCodeChunk

	// Find the class body
	bodyNode := findChildByType(classNode, "block")
	if bodyNode == nil {
		return methods
	}

	// Reduce max depth for child extraction
	childOptions := options
	childOptions.MaxDepth = options.MaxDepth - 1

	// Extract function definitions within the interface
	functionNodes := findChildrenByType(bodyNode, "function_definition")
	for _, node := range functionNodes {
		method := parseInterfaceMethod(ctx, parseTree, node, className, moduleName, childOptions, now)
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
	now time.Time,
) *outbound.SemanticCodeChunk {
	if node == nil {
		return nil
	}

	// Extract method name
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
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

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("interface_method", methodName, nil),
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
}

// isProtocolMethod checks if a method belongs to a Protocol-based interface.
func isProtocolMethod(parseTree *valueobject.ParseTree, methodNode *valueobject.ParseNode) bool {
	// Look for ellipsis (...) in method body (Protocol method signature)
	bodyNode := findChildByType(methodNode, "block")
	if bodyNode == nil {
		return false
	}

	for _, child := range bodyNode.Children {
		if child.Type == "expression_statement" {
			ellipsisNode := findChildByType(child, "ellipsis")
			if ellipsisNode != nil {
				return true
			}
		}
	}

	return false
}

// Helper function to detect if a method has only pass statement (common in interfaces).
func hasOnlyPassStatement(parseTree *valueobject.ParseTree, methodNode *valueobject.ParseNode) bool {
	bodyNode := findChildByType(methodNode, "block")
	if bodyNode == nil {
		return false
	}

	// Check if body contains only a pass statement
	passCount := 0
	totalStatements := 0

	for _, child := range bodyNode.Children {
		if child.Type == "pass_statement" {
			passCount++
		}
		if child.Type != "comment" && child.Type != "newline" {
			totalStatements++
		}
	}

	return passCount == totalStatements && totalStatements == 1
}
