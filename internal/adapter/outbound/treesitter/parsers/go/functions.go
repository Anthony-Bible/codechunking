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

// FunctionInfo holds detailed information about a function parsed from source.
type FunctionInfo struct {
	Name          string
	Documentation string
	Content       string
	Parameters    []outbound.Parameter
	ReturnType    string
	StartByte     uint32
	EndByte       uint32
	IsMethod      bool
	ReceiverType  string
}

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
		} else {
			slogger.Error(ctx, "Failed to parse function node", slogger.Fields{
				"error":     err.Error(),
				"node_type": node.Type,
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

	// Log if Tree-sitter parsing found no functions (removed fallback to force Tree-sitter usage)
	if len(functions) == 0 {
		slogger.Warn(ctx, "Tree-sitter found no function nodes - check parse tree and grammar", slogger.Fields{
			"function_nodes_searched": nodeTypeFunctionDecl,
			"method_nodes_searched":   nodeTypeMethodDecl,
		})
	}

	slogger.Info(ctx, "ExtractFunctions completed", slogger.Fields{
		"total_functions_extracted": len(functions),
	})

	return functions, nil
}

// extractFunctionsFromSourceText is now removed - forcing Tree-sitter only parsing

// Regex-based parsing methods removed - Tree-sitter only parsing now

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
	ctx := context.TODO() // Add context for logging
	if node == nil {
		return outbound.SemanticCodeChunk{}, errors.New("parse node cannot be nil")
	}
	if parseTree == nil {
		return outbound.SemanticCodeChunk{}, errors.New("parse tree cannot be nil")
	}

	// Debug: log all children types
	var childTypes []string
	for _, child := range node.Children {
		childTypes = append(childTypes, child.Type)
	}
	slogger.Debug(ctx, "Function node children", slogger.Fields{
		"child_types": childTypes,
	})

	// Find the first identifier child (should be function name in function_declaration)
	var nameNode *valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == "identifier" {
			nameNode = child
			break
		}
	}
	if nameNode == nil {
		return outbound.SemanticCodeChunk{}, errors.New(
			"function name not found - children: " + strings.Join(childTypes, ", "),
		)
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

	// Find method name: Go grammar uses field_identifier for method names
	var nameNode *valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeTypeFieldIdentifier || child.Type == nodeTypeIdentifier {
			nameNode = child
			break
		}
	}

	if nameNode == nil {
		return outbound.SemanticCodeChunk{}, errors.New("method name not found")
	}

	name := parseTree.GetNodeText(nameNode)
	visibility := p.getVisibility(name)

	// Extract receiver info (first parameter_list before name)
	receiverInfo := ""
	parameterNodes := findChildrenByType(node, nodeTypeParameterList)
	if len(parameterNodes) > 0 && parameterNodes[0].StartByte < nameNode.StartByte {
		receiverInfo = parseTree.GetNodeText(parameterNodes[0])
	}

	qualifiedName := packageName + "." + name
	if receiverInfo != "" {
		// Extract receiver type name for qualified name (e.g., "(p *Person)" -> "Person")
		receiverType := extractReceiverTypeName(receiverInfo)
		if receiverType != "" {
			qualifiedName = receiverType + "." + name
		}
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
		// Handle multiple parameters with shared type (e.g., "a, b int")
		allParams := p.parseGoParameterDeclarations(paramNode, parseTree)
		for _, param := range allParams {
			if param.Name != "" {
				parameters = append(parameters, param)
			}
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

// parseGoParameterDeclarations handles parameter declarations that may have multiple names sharing a type.
func (p *GoParser) parseGoParameterDeclarations(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) []outbound.Parameter {
	var parameters []outbound.Parameter
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

	var typeStr string
	if typeNode != nil {
		typeStr = parseTree.GetNodeText(typeNode)
	}

	// Create a parameter for each identifier (for cases like "a, b int")
	for _, identifier := range identifiers {
		paramName := parseTree.GetNodeText(identifier)
		if paramName != "" {
			parameters = append(parameters, outbound.Parameter{
				Name: paramName,
				Type: typeStr,
			})
		}
	}

	// If no parameters found, fall back to original single parameter logic
	if len(parameters) == 0 {
		param := p.parseGoParameterDeclaration(node, parseTree)
		if param.Name != "" {
			parameters = append(parameters, param)
		}
	}

	return parameters
}

// extractReceiverTypeName extracts the type name from receiver info like "(p *Person)" -> "Person".
func extractReceiverTypeName(receiverInfo string) string {
	// Remove parentheses and extract type name
	receiverInfo = strings.Trim(receiverInfo, "()")

	// Split on whitespace to get parts like ["p", "*Person"] or ["p", "Person"]
	parts := strings.Fields(receiverInfo)
	if len(parts) >= 2 {
		typeName := parts[1]
		// Remove pointer indicator if present
		typeName = strings.TrimPrefix(typeName, "*")
		return typeName
	}

	return ""
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
