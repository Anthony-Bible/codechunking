package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// PythonClassParser represents the specialized Python class parser.
type PythonClassParser struct{}

// NewPythonClassParser creates a new Python class parser instance.
func NewPythonClassParser() *PythonClassParser {
	return &PythonClassParser{}
}

// ParsePythonClass parses a Python class definition.
func (p *PythonClassParser) ParsePythonClass(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if parseTree == nil || node == nil {
		return nil
	}

	// Extract class name
	className := extractClassNameFromNode(parseTree, node)
	if className == "" {
		return nil
	}

	// Extract class documentation
	var documentation string
	if options.IncludeDocumentation {
		documentation = extractClassDocstring(parseTree, node)
	}

	// Extract inheritance information
	dependencies := extractInheritanceDependencies(parseTree, node)

	// Extract decorators (both direct and from parent decorated_definition)
	annotations := extractAllClassDecorators(parseTree, node)

	// Extract child chunks (methods, class variables, nested classes)
	var childChunks []outbound.SemanticCodeChunk
	if options.MaxDepth > 0 {
		childChunks = extractClassChildren(ctx, parseTree, node, className, moduleName, options, now)
	}

	// Get class content
	content := parseTree.GetNodeText(node)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("class", className, nil),
		Type:          outbound.ConstructClass,
		Name:          className,
		QualifiedName: qualifyName(moduleName, className),
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       content,
		Documentation: documentation,
		Visibility:    getPythonVisibility(className),
		Dependencies:  dependencies,
		Annotations:   annotations,
		ChildChunks:   childChunks,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// extractPythonClasses extracts Python classes from the parse tree.
func extractPythonClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var classes []outbound.SemanticCodeChunk
	now := time.Now()
	moduleName := extractModuleName(parseTree)

	// Find class definitions (both plain and decorated)
	classNodes := parseTree.GetNodesByType("class_definition")
	decoratedNodes := parseTree.GetNodesByType("decorated_definition")

	parser := NewPythonClassParser()

	// Process plain class definitions
	plainClasses := extractPlainClasses(ctx, parseTree, classNodes, decoratedNodes, parser, moduleName, options, now)
	classes = append(classes, plainClasses...)

	// Process decorated class definitions
	decoratedClasses := extractDecoratedClasses(ctx, parseTree, decoratedNodes, parser, moduleName, options, now)
	classes = append(classes, decoratedClasses...)

	return classes, nil
}

// extractPlainClasses extracts plain (non-decorated) class definitions.
func extractPlainClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNodes []*valueobject.ParseNode,
	decoratedNodes []*valueobject.ParseNode,
	parser *PythonClassParser,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var classes []outbound.SemanticCodeChunk

	// Process plain class definitions (skip those that are inside decorated_definition)
	for _, node := range classNodes {
		// Check if this class node is part of a decorated definition
		if !isNodeInDecoratedDefinition(node, decoratedNodes) {
			class := parser.ParsePythonClass(ctx, parseTree, node, moduleName, options, now)
			if class != nil && shouldIncludeByVisibility(class.Visibility, options.IncludePrivate) {
				classes = append(classes, *class)
			}
		}
	}

	return classes
}

// extractDecoratedClasses extracts decorated class definitions.
func extractDecoratedClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	decoratedNodes []*valueobject.ParseNode,
	parser *PythonClassParser,
	moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var classes []outbound.SemanticCodeChunk

	// Process decorated definitions (which may contain classes)
	for _, decoratedNode := range decoratedNodes {
		// Look for class_definition within decorated_definition
		for _, child := range decoratedNode.Children {
			if child.Type == "class_definition" {
				// Parse the class
				class := parser.ParsePythonClass(ctx, parseTree, child, moduleName, options, now)
				if class != nil {
					// Extract decorators from the decorated_definition and add them
					decorators := extractDecoratorsFromDecoratedNode(parseTree, decoratedNode)
					class.Annotations = append(class.Annotations, decorators...)

					if shouldIncludeByVisibility(class.Visibility, options.IncludePrivate) {
						classes = append(classes, *class)
					}
				}
			}
		}
	}

	return classes
}

// extractClassNameFromNode extracts the class name from a class definition node.
func extractClassNameFromNode(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	nameNode := findChildByType(node, "identifier")
	if nameNode != nil {
		return parseTree.GetNodeText(nameNode)
	}
	return ""
}

// extractClassDocstring extracts docstring from class.
func extractClassDocstring(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Find the class body
	bodyNode := findChildByType(node, "block")
	if bodyNode == nil {
		return ""
	}

	// Look for the first string literal (docstring)
	for _, child := range bodyNode.Children {
		if child.Type == nodeTypeExpressionStatement {
			stringNode := findChildByType(child, "string")
			if stringNode != nil {
				docstring := parseTree.GetNodeText(stringNode)
				// Clean up the docstring (remove quotes)
				docstring = strings.Trim(docstring, `"'`)
				return docstring
			}
		}
	}
	return ""
}

// extractInheritanceDependencies extracts inheritance information from class definition.
func extractInheritanceDependencies(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []outbound.DependencyReference {
	var dependencies []outbound.DependencyReference

	// Find argument list (base classes)
	argListNode := findChildByType(node, "argument_list")
	if argListNode == nil {
		return dependencies
	}

	// Extract base class names
	for _, child := range argListNode.Children {
		if child.Type == nodeTypeIdentifier {
			baseName := parseTree.GetNodeText(child)
			dependencies = append(dependencies, outbound.DependencyReference{
				Name: baseName,
				Type: "inheritance",
			})
		}
	}

	return dependencies
}

// extractClassDecorators extracts decorators from a class definition.
func extractClassDecorators(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.Annotation {
	var annotations []outbound.Annotation

	// Look for decorator nodes before the class
	// In tree-sitter, decorated classes are represented as "decorated_definition" nodes
	// which contain both decorators and the class definition

	// For decorated classes, decorators need to be handled at a higher level
	// This function only handles decorators that are direct children

	// Also check direct children (fallback)
	for _, child := range node.Children {
		if child.Type == "decorator" {
			decoratorName := parseTree.GetNodeText(child)
			decoratorName = strings.TrimPrefix(decoratorName, "@")

			annotation := outbound.Annotation{
				Name: decoratorName,
			}

			// Check for decorator arguments
			if strings.Contains(decoratorName, "(") {
				parts := strings.SplitN(decoratorName, "(", 2)
				annotation.Name = parts[0]
				if len(parts) > 1 {
					args := strings.TrimSuffix(parts[1], ")")
					annotation.Arguments = []string{args}
				}
			}

			annotations = append(annotations, annotation)
		}
	}

	return annotations
}

// extractDecoratorsFromDecoratedNode extracts decorators from a decorated_definition node.
func extractDecoratorsFromDecoratedNode(
	parseTree *valueobject.ParseTree,
	decoratedNode *valueobject.ParseNode,
) []outbound.Annotation {
	var annotations []outbound.Annotation

	// Look for decorator nodes in the decorated_definition
	for _, child := range decoratedNode.Children {
		if child.Type == nodeTypeDecorator {
			decoratorName := parseTree.GetNodeText(child)
			decoratorName = strings.TrimPrefix(decoratorName, "@")

			annotation := outbound.Annotation{
				Name: decoratorName,
			}

			// Check for decorator arguments
			if strings.Contains(decoratorName, "(") {
				parts := strings.SplitN(decoratorName, "(", 2)
				annotation.Name = parts[0]
				if len(parts) > 1 {
					args := strings.TrimSuffix(parts[1], ")")
					annotation.Arguments = []string{args}
				}
			}

			annotations = append(annotations, annotation)
		}
	}

	return annotations
}

// isNodeInDecoratedDefinition checks if a class node is contained within any decorated_definition.
func isNodeInDecoratedDefinition(classNode *valueobject.ParseNode, decoratedNodes []*valueobject.ParseNode) bool {
	for _, decoratedNode := range decoratedNodes {
		// Check if the class node is within the decorated definition's byte range
		if classNode.StartByte >= decoratedNode.StartByte && classNode.EndByte <= decoratedNode.EndByte {
			return true
		}
	}
	return false
}

// extractAllClassDecorators extracts decorators for a class, checking both direct decorators
// and decorators from parent decorated_definition nodes.
func extractAllClassDecorators(
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
) []outbound.Annotation {
	var annotations []outbound.Annotation

	// First, check for direct decorators (fallback)
	directAnnotations := extractClassDecorators(parseTree, classNode)
	annotations = append(annotations, directAnnotations...)

	// Then, check if this class is part of any decorated_definition
	decoratedNodes := parseTree.GetNodesByType("decorated_definition")
	for _, decoratedNode := range decoratedNodes {
		// Check if the class node is within this decorated definition's byte range
		if classNode.StartByte >= decoratedNode.StartByte && classNode.EndByte <= decoratedNode.EndByte {
			// Extract decorators from the decorated_definition
			decoratedAnnotations := extractDecoratorsFromDecoratedNode(parseTree, decoratedNode)
			annotations = append(annotations, decoratedAnnotations...)
			break // Found the parent decorated definition, no need to check others
		}
	}

	return annotations
}

// extractClassChildren extracts child elements from a class (methods, variables, nested classes).
func extractClassChildren(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var children []outbound.SemanticCodeChunk

	// Reduce max depth for child extraction
	childOptions := options
	childOptions.MaxDepth = options.MaxDepth - 1

	// Extract methods
	methods := extractMethodsFromClass(ctx, parseTree, classNode, className, moduleName, childOptions, now)
	for _, method := range methods {
		if shouldIncludeByVisibility(method.Visibility, options.IncludePrivate) {
			children = append(children, method)
		}
	}

	// Extract class variables
	classVars := extractClassVariables(parseTree, classNode, className, moduleName, childOptions, now)
	for _, classVar := range classVars {
		if shouldIncludeByVisibility(classVar.Visibility, options.IncludePrivate) {
			children = append(children, classVar)
		}
	}

	// Extract nested classes (if depth allows)
	if childOptions.MaxDepth > 0 {
		nestedClasses := extractNestedClasses(ctx, parseTree, classNode, className, moduleName, childOptions, now)
		for _, nestedClass := range nestedClasses {
			if shouldIncludeByVisibility(nestedClass.Visibility, options.IncludePrivate) {
				children = append(children, nestedClass)
			}
		}
	}

	return children
}

// extractClassVariables extracts class variables from a class definition.
func extractClassVariables(
	parseTree *valueobject.ParseTree,
	classNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var variables []outbound.SemanticCodeChunk

	// Find the class body
	bodyNode := findChildByType(classNode, "block")
	if bodyNode == nil {
		return variables
	}

	// Look for assignment statements at class level
	for _, child := range bodyNode.Children {
		if child.Type == nodeTypeExpressionStatement || child.Type == "assignment" {
			variable := extractClassVariableFromAssignment(parseTree, child, className, moduleName, options, now)
			if variable != nil {
				variables = append(variables, *variable)
			}
		}
	}

	return variables
}

// extractClassVariableFromAssignment extracts a class variable from an assignment statement.
func extractClassVariableFromAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
	className, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if assignmentNode == nil {
		return nil
	}

	// Find the variable name (left side of assignment)
	identifierNode := findChildByType(assignmentNode, "identifier")
	if identifierNode == nil {
		return nil
	}

	varName := parseTree.GetNodeText(identifierNode)
	content := parseTree.GetNodeText(assignmentNode)

	// Determine if it's a constant (all uppercase) or variable
	constructType := outbound.ConstructVariable
	if strings.ToUpper(varName) == varName {
		constructType = outbound.ConstructConstant
	}

	// Extract type annotation if present
	returnType := extractTypeAnnotation(parseTree, assignmentNode)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("class_var", varName, nil),
		Type:          constructType,
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

// extractNestedClasses extracts nested class definitions.
func extractNestedClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	parentClassNode *valueobject.ParseNode,
	parentClassName, moduleName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var nestedClasses []outbound.SemanticCodeChunk

	// Find the class body
	bodyNode := findChildByType(parentClassNode, "block")
	if bodyNode == nil {
		return nestedClasses
	}

	// Look for nested class definitions
	nestedClassNodes := findChildrenByType(bodyNode, "class_definition")
	parser := NewPythonClassParser()

	for _, node := range nestedClassNodes {
		// Create qualified module name for nested class
		nestedModuleName := qualifyName(moduleName, parentClassName)
		nestedClass := parser.ParsePythonClass(ctx, parseTree, node, nestedModuleName, options, now)
		if nestedClass != nil {
			nestedClasses = append(nestedClasses, *nestedClass)
		}
	}

	return nestedClasses
}

// extractTypeAnnotation extracts type annotation from an assignment or variable declaration.
func extractTypeAnnotation(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Look for type annotation
	for _, child := range node.Children {
		if child.Type == "type" || child.Type == "type_annotation" {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
}
