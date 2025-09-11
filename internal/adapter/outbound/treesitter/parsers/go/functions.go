package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Tree-sitter node types for functions from Go grammar.
const (
	nodeTypeFunctionDecl      = "function_declaration"
	nodeTypeMethodDecl        = "method_declaration"
	nodeTypeIdentifier        = "identifier"
	nodeTypeFieldIdentifier   = "field_identifier"
	nodeType_FieldIdentifier  = "_field_identifier"
	nodeTypeParameterList     = "parameter_list"
	nodeTypeParameterDecl     = "parameter_declaration"
	nodeTypePointerType       = "pointer_type"
	nodeTypeTypeIdentifier    = "type_identifier"
	nodeTypeArrayType         = "array_type"
	nodeTypeSliceType         = "slice_type"
	nodeTypeMapType           = "map_type"
	nodeTypeChannelType       = "channel_type"
	nodeTypeFunctionType      = "function_type"
	nodeTypeInterfaceType     = "interface_type"
	nodeTypeStructType        = "struct_type"
	nodeTypeQualifiedType     = "qualified_type"
	nodeTypeTupleType         = "tuple_type"
	nodeTypeTypeParameterList = "type_parameter_list"
)

// Tree-sitter field names from Go grammar.
// Currently no field name constants are needed

// ExtractFunctions extracts function and method declarations from a Go parse tree.
// This method implements proper tree-sitter AST traversal according to Go grammar.
func (o *ObservableGoParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "ObservableGoParser.ExtractFunctions called", slogger.Fields{
		"root_node_type": parseTree.RootNode().Type,
	})

	if err := o.parser.validateInput(parseTree); err != nil {
		return nil, err
	}

	var functions []outbound.SemanticCodeChunk
	functionNodes := parseTree.GetNodesByType(nodeTypeFunctionDecl)
	methodNodes := parseTree.GetNodesByType(nodeTypeMethodDecl)

	slogger.Info(ctx, "Searching for function nodes using Tree-sitter", slogger.Fields{
		"function_nodes_count": len(functionNodes),
		"method_nodes_count":   len(methodNodes),
	})

	packageName := o.extractPackageNameFromTree(parseTree)

	// Extract regular functions
	for _, node := range functionNodes {
		if fn, err := o.parser.parseGoFunction(node, packageName, parseTree); err == nil {
			functions = append(functions, fn)
			slogger.Info(ctx, "Extracted function", slogger.Fields{
				"function_name": fn.Name,
			})
		}
	}

	// Extract methods
	for _, node := range methodNodes {
		if fn, err := o.parser.parseGoMethod(node, packageName, parseTree); err == nil {
			functions = append(functions, fn)
			slogger.Info(ctx, "Extracted method", slogger.Fields{
				"method_name": fn.Name,
			})
		}
	}

	// Fallback to source-based parsing if Tree-sitter nodes are empty
	if len(functions) == 0 {
		slogger.Info(ctx, "No Tree-sitter nodes found, falling back to source-based parsing", slogger.Fields{})
		fallbackFunctions := o.extractFunctionsFromSourceText(parseTree, packageName)
		functions = append(functions, fallbackFunctions...)
	}

	slogger.Info(ctx, "ExtractFunctions completed", slogger.Fields{
		"total_functions_extracted": len(functions),
	})

	return functions, nil
}

// extractFunctionsFromSourceText provides fallback function extraction using source text parsing.
func (o *ObservableGoParser) extractFunctionsFromSourceText(
	parseTree *valueobject.ParseTree,
	packageName string,
) []outbound.SemanticCodeChunk {
	var functions []outbound.SemanticCodeChunk
	source := string(parseTree.Source())
	funcNames := o.extractGoFunctionNames(source)

	for i, funcName := range funcNames {
		functions = append(functions, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("go_func_%s_%d", funcName, i),
			Name:          funcName,
			QualifiedName: fmt.Sprintf("%s.%s", packageName, funcName),
			Language:      parseTree.Language(),
			Type:          outbound.ConstructFunction,
			Visibility:    o.getVisibility(funcName),
			Content:       fmt.Sprintf("// Go function: %s", funcName),
			ExtractedAt:   time.Now(),
		})
	}
	return functions
}

// extractPackageNameFromTree extracts package name from parse tree, with fallback.
func (o *ObservableGoParser) extractPackageNameFromTree(parseTree *valueobject.ParseTree) string {
	packageNodes := parseTree.GetNodesByType("package_clause")
	if len(packageNodes) > 0 {
		// Look for package identifier in the package clause
		for _, child := range packageNodes[0].Children {
			if child.Type == "package_identifier" || child.Type == "_package_identifier" {
				return parseTree.GetNodeText(child)
			}
		}
	}
	return "main" // Default fallback
}

// extractGoFunctionNames extracts function names from Go source code using regex parsing (fallback).
func (o *ObservableGoParser) extractGoFunctionNames(source string) []string {
	var funcNames []string
	lines := strings.Split(source, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "func ") {
			if funcName := o.extractFunctionNameFromLine(line); funcName != "" {
				funcNames = append(funcNames, funcName)
			}
		}
	}

	return funcNames
}

// extractFunctionNameFromLine extracts function name from a single line.
func (o *ObservableGoParser) extractFunctionNameFromLine(line string) string {
	funcLine := strings.TrimPrefix(line, "func ")

	// Handle method receivers: func (r Receiver) methodName(
	if strings.HasPrefix(funcLine, "(") {
		return o.extractMethodNameFromFuncLine(funcLine)
	}

	// Handle regular functions: func functionName(
	return o.extractRegularFunctionNameFromFuncLine(funcLine)
}

// extractMethodNameFromFuncLine extracts method name from a func line with receiver.
func (o *ObservableGoParser) extractMethodNameFromFuncLine(funcLine string) string {
	parenEnd := strings.Index(funcLine, ")")
	if parenEnd != -1 && parenEnd+1 < len(funcLine) {
		methodPart := strings.TrimSpace(funcLine[parenEnd+1:])
		parenStart := strings.Index(methodPart, "(")
		if parenStart > 0 {
			methodName := strings.TrimSpace(methodPart[:parenStart])
			if methodName != "" && !strings.Contains(methodName, " ") {
				return methodName
			}
		}
	}
	return ""
}

// extractRegularFunctionNameFromFuncLine extracts function name from a regular func line.
func (o *ObservableGoParser) extractRegularFunctionNameFromFuncLine(funcLine string) string {
	parenStart := strings.Index(funcLine, "(")
	if parenStart > 0 {
		funcName := strings.TrimSpace(funcLine[:parenStart])
		if funcName != "" && !strings.Contains(funcName, " ") {
			return funcName
		}
	}
	return ""
}

// getVisibility determines if a Go function is public or private.
func (o *ObservableGoParser) getVisibility(name string) outbound.VisibilityModifier {
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		return outbound.Public
	}
	return outbound.Private
}

// Core parser methods for the inner GoParser.
func (p *GoParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	var functions []outbound.SemanticCodeChunk
	functionNodes := parseTree.GetNodesByType("function_declaration")
	methodNodes := parseTree.GetNodesByType("method_declaration")

	packageName := p.extractPackageNameFromTree(parseTree)

	for _, node := range functionNodes {
		if fn, err := p.parseGoFunction(node, packageName, parseTree); err == nil {
			functions = append(functions, fn)
		}
	}

	for _, node := range methodNodes {
		if fn, err := p.parseGoMethod(node, packageName, parseTree); err == nil {
			functions = append(functions, fn)
		}
	}

	return functions, nil
}

func (p *GoParser) parseGoFunction(
	node *valueobject.ParseNode,
	packageName string,
	parseTree *valueobject.ParseTree,
) (outbound.SemanticCodeChunk, error) {
	if node == nil {
		return outbound.SemanticCodeChunk{}, errors.New("parse node cannot be nil")
	}
	if parseTree == nil {
		return outbound.SemanticCodeChunk{}, errors.New("parse tree cannot be nil")
	}

	nameNode := findChildByTypeInNode(node, "identifier")
	if nameNode == nil {
		return outbound.SemanticCodeChunk{}, errors.New("function name not found")
	}

	name := parseTree.GetNodeText(nameNode)
	visibility := p.getVisibility(name)
	qualifiedName := packageName + "." + name

	var parameters []outbound.Parameter
	var returnType string
	var genericParameters []outbound.GenericParameter

	// Find parameter list and return type
	for _, child := range node.Children {
		switch child.Type {
		case nodeTypeParameterList:
			parameters = p.parseGoParameterList(child, parseTree)
		case nodeTypeTypeParameterList:
			genericParameters = p.parseGoGenericParameterList(child, parseTree)
		case nodeTypeTypeIdentifier, nodeTypePointerType, nodeTypeQualifiedType, nodeTypeTupleType:
			returnType = p.parseGoReturnType(child, parseTree)
		}
	}

	comments := p.extractPrecedingComments(node, parseTree)
	content := parseTree.GetNodeText(node)
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	id := utils.GenerateID(string(outbound.ConstructFunction), name, map[string]interface{}{
		"package": packageName,
		"hash":    hashStr,
	})

	return outbound.SemanticCodeChunk{
		ChunkID:           id,
		Name:              name,
		QualifiedName:     qualifiedName,
		Language:          parseTree.Language(),
		Type:              outbound.ConstructFunction,
		Visibility:        visibility,
		Parameters:        parameters,
		ReturnType:        returnType,
		GenericParameters: genericParameters,
		Content:           content,
		Documentation:     strings.Join(comments, "\n"),
		ExtractedAt:       time.Now(),
		Hash:              hashStr,
		StartByte:         node.StartByte,
		EndByte:           node.EndByte,
	}, nil
}

func (p *GoParser) parseGoMethod(
	node *valueobject.ParseNode,
	packageName string,
	parseTree *valueobject.ParseTree,
) (outbound.SemanticCodeChunk, error) {
	if node == nil {
		return outbound.SemanticCodeChunk{}, errors.New("parse node cannot be nil")
	}
	if parseTree == nil {
		return outbound.SemanticCodeChunk{}, errors.New("parse tree cannot be nil")
	}

	// Find method name (identifier after func keyword and receiver)
	var nameNode *valueobject.ParseNode
	for i, child := range node.Children {
		if child.Type == "identifier" && i > 2 {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return outbound.SemanticCodeChunk{}, errors.New("method name not found")
	}

	name := parseTree.GetNodeText(nameNode)
	visibility := p.getVisibility(name)

	// Extract receiver info
	receiverInfo := ""
	parameterNodes := findChildrenByType(node, "parameter_list")
	if len(parameterNodes) > 0 && parameterNodes[0].StartByte < nameNode.StartByte {
		receiverInfo = parseTree.GetNodeText(parameterNodes[0])
	}

	qualifiedName := packageName + "." + name
	if receiverInfo != "" {
		// Incorporate receiver info into qualified name
		qualifiedName = packageName + "." + receiverInfo + "." + name
	}

	var parameters []outbound.Parameter
	var returnType string
	var genericParameters []outbound.GenericParameter

	// Process all parameter lists and type information
	for _, child := range node.Children {
		switch child.Type {
		case nodeTypeParameterList:
			// Skip receiver parameter list
			if child.StartByte < nameNode.StartByte {
				continue
			}
			parameters = p.parseGoParameterList(child, parseTree)
		case nodeTypeTypeParameterList:
			genericParameters = p.parseGoGenericParameterList(child, parseTree)
		case nodeTypeTypeIdentifier, nodeTypePointerType, nodeTypeQualifiedType, nodeTypeTupleType:
			returnType = p.parseGoReturnType(child, parseTree)
		}
	}

	comments := p.extractPrecedingComments(node, parseTree)
	content := parseTree.GetNodeText(node)
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	id := utils.GenerateID(string(outbound.ConstructMethod), name, map[string]interface{}{
		"package":  packageName,
		"receiver": receiverInfo,
		"hash":     hashStr,
	})

	return outbound.SemanticCodeChunk{
		ChunkID:           id,
		Name:              name,
		QualifiedName:     qualifiedName,
		Language:          parseTree.Language(),
		Type:              outbound.ConstructMethod,
		Visibility:        visibility,
		Parameters:        parameters,
		ReturnType:        returnType,
		GenericParameters: genericParameters,
		Content:           content,
		Documentation:     strings.Join(comments, "\n"),
		ExtractedAt:       time.Now(),
		Hash:              hashStr,
		StartByte:         node.StartByte,
		EndByte:           node.EndByte,
	}, nil
}

func (p *GoParser) parseGoParameterList(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) []outbound.Parameter {
	var parameters []outbound.Parameter

	parameterNodes := findChildrenByType(node, "parameter_declaration")
	for _, paramNode := range parameterNodes {
		if param := p.parseGoParameterDeclaration(paramNode, parseTree); param.Name != "" {
			parameters = append(parameters, param)
		}
	}

	return parameters
}

func (p *GoParser) parseGoGenericParameterList(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) []outbound.GenericParameter {
	var genericParams []outbound.GenericParameter

	typeParams := findChildrenByType(node, "type_parameter")
	for _, typeParam := range typeParams {
		if param := p.parseGoGenericParameter(typeParam, parseTree); param.Name != "" {
			genericParams = append(genericParams, param)
		}
	}

	return genericParams
}

func (p *GoParser) parseGoParameterDeclaration(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) outbound.Parameter {
	var param outbound.Parameter

	// Get all identifiers in the parameter declaration
	identifiers := findChildrenByType(node, "identifier")

	// Get the type node
	var typeNode *valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeTypeTypeIdentifier || child.Type == nodeTypePointerType ||
			child.Type == nodeTypeQualifiedType || child.Type == nodeTypeArrayType {
			typeNode = child
			break
		}
	}

	if len(identifiers) > 0 {
		// The last identifier is typically the parameter name
		nameNode := identifiers[len(identifiers)-1]
		param.Name = parseTree.GetNodeText(nameNode)
	}

	if typeNode != nil {
		param.Type = parseTree.GetNodeText(typeNode)
	}

	return param
}

func (p *GoParser) parseGoReturnType(node *valueobject.ParseNode, parseTree *valueobject.ParseTree) string {
	switch node.Type {
	case nodeTypeTypeIdentifier, nodeTypePointerType, nodeTypeQualifiedType:
		return parseTree.GetNodeText(node)
	case nodeTypeTupleType:
		// Handle multiple return values
		var types []string
		for _, child := range node.Children {
			if child.Type == nodeTypeTypeIdentifier || child.Type == nodeTypePointerType ||
				child.Type == nodeTypeQualifiedType {
				types = append(types, parseTree.GetNodeText(child))
			}
		}
		return strings.Join(types, ", ")
	default:
		return ""
	}
}

func (p *GoParser) parseGoGenericParameter(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) outbound.GenericParameter {
	var param outbound.GenericParameter

	// Get the name (identifier)
	nameNode := findChildByTypeInNode(node, "identifier")
	if nameNode != nil {
		param.Name = parseTree.GetNodeText(nameNode)
	}

	// Get the type constraint if present
	var constraints []string
	for _, child := range node.Children {
		if child.Type == nodeTypeTypeIdentifier || child.Type == nodeTypeQualifiedType {
			constraints = append(constraints, parseTree.GetNodeText(child))
		}
	}

	if len(constraints) > 0 {
		param.Constraints = constraints
	}

	return param
}

func (p *GoParser) extractPrecedingComments(node *valueobject.ParseNode, parseTree *valueobject.ParseTree) []string {
	var comments []string

	// Get all comment nodes
	commentNodes := parseTree.GetNodesByType("comment")

	for _, commentNode := range commentNodes {
		// Only consider comments that come before the function node
		if commentNode.EndByte <= node.StartByte {
			commentText := parseTree.GetNodeText(commentNode)
			// Remove comment markers
			commentText = strings.TrimPrefix(commentText, "//")
			commentText = strings.TrimPrefix(commentText, "/*")
			commentText = strings.TrimSuffix(commentText, "*/")
			commentText = strings.TrimSpace(commentText)

			if commentText != "" {
				comments = append(comments, commentText)
			}
		}
	}

	return comments
}

// parseGoParameters parses parameter list from a method specification node.
func parseGoParameters(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) []outbound.Parameter {
	// Create a temporary parser to use the method
	parser := &GoParser{}
	var parameters []outbound.Parameter

	// Find parameter list in the node
	parameterListNode := findChildByTypeInNode(node, "parameter_list")
	if parameterListNode == nil {
		return parameters
	}

	// Parse parameter declarations
	parameterNodes := findChildrenByType(parameterListNode, "parameter_declaration")
	for _, paramNode := range parameterNodes {
		if param := parser.parseGoParameterDeclaration(paramNode, parseTree); param.Name != "" {
			parameters = append(parameters, param)
		}
	}

	return parameters
}

// parseGoReturnType extracts return type from a method specification node.
func parseGoReturnType(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Look for type information in the node
	for _, child := range node.Children {
		if child.Type == "type_identifier" || child.Type == "pointer_type" ||
			child.Type == "qualified_type" || child.Type == "tuple_type" {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
}

// Helper functions for node traversal (shared across parsers).
func findChildByTypeInNode(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	if node.Type == nodeType {
		return node
	}

	for _, child := range node.Children {
		if found := findChildByTypeInNode(child, nodeType); found != nil {
			return found
		}
	}

	return nil
}

func findChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var results []*valueobject.ParseNode

	if node.Type == nodeType {
		results = append(results, node)
	}

	for _, child := range node.Children {
		childResults := findChildrenByType(child, nodeType)
		results = append(results, childResults...)
	}

	return results
}
