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
	nodeTypeFunctionDecl            = "function_declaration"
	nodeTypeMethodDecl              = "method_declaration"
	nodeTypeIdentifier              = "identifier"
	nodeTypeFieldIdentifier         = "field_identifier"
	nodeType_FieldIdentifier        = "_field_identifier"
	nodeTypeParameterList           = "parameter_list"
	nodeTypeParameterDecl           = "parameter_declaration"
	nodeTypeVariadicParameterDecl   = "variadic_parameter_declaration"
	nodeTypePointerType             = "pointer_type"
	nodeTypeTypeIdentifier          = "type_identifier"
	nodeTypeArrayType               = "array_type"
	nodeTypeSliceType               = "slice_type"
	nodeTypeMapType                 = "map_type"
	nodeTypeChannelType             = "channel_type"
	nodeTypeFunctionType            = "function_type"
	nodeTypeInterfaceType           = "interface_type"
	nodeTypeStructType              = "struct_type"
	nodeTypeQualifiedType           = "qualified_type"
	nodeTypeTupleType               = "tuple_type"
	nodeTypeTypeParameterList       = "type_parameter_list"
	nodeTypeImplicitLengthArrayType = "implicit_length_array_type"
	nodeTypeGenericType             = "generic_type"
)

// Common package/function names used in parsing.
const (
	defaultPackageName = "main"
)

// Tree-sitter field names from Go grammar.
// Currently no field name constants are needed

// FunctionInfo holds detailed information about a function parsed from source code.
// This struct provides a comprehensive representation of Go functions and methods
// extracted using tree-sitter parsing.
type FunctionInfo struct {
	Name          string               // The function or method name
	Documentation string               // Leading comments that document the function
	Content       string               // The complete source code of the function
	Parameters    []outbound.Parameter // Function parameters with names and types
	ReturnType    string               // Return type specification (may be empty for no return)
	StartByte     uint32               // Start position in source file (0-based)
	EndByte       uint32               // End position in source file (exclusive)
	IsMethod      bool                 // True if this is a method (has receiver)
	ReceiverType  string               // Receiver type for methods (empty for functions)
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

	// Delegate to shared extraction logic with logging enabled
	functions, err := o.parser.extractFunctionsShared(ctx, parseTree, true)
	if err != nil {
		return nil, err
	}

	slogger.Info(ctx, "ExtractFunctions completed", slogger.Fields{
		"total_functions_extracted": len(functions),
	})

	return functions, nil
}

// extractFunctionsShared contains the core function extraction logic shared between
// ObservableGoParser and GoParser implementations. It performs tree-sitter based
// parsing to identify and extract Go functions and methods from the parse tree.
//
// Parameters:
//   - ctx: Context for logging and cancellation
//   - parseTree: The tree-sitter parse tree to extract functions from
//   - enableLogging: Whether to emit detailed logging during extraction
//
// Returns:
//   - A slice of SemanticCodeChunk representing extracted functions/methods
//   - An error if extraction fails due to invalid input or parsing issues
func (p *GoParser) extractFunctionsShared(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	enableLogging bool,
) ([]outbound.SemanticCodeChunk, error) {
	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	var functions []outbound.SemanticCodeChunk
	functionNodes := parseTree.GetNodesByType(nodeTypeFunctionDecl)
	methodNodes := parseTree.GetNodesByType(nodeTypeMethodDecl)

	if enableLogging {
		slogger.Info(ctx, "Searching for function nodes using Tree-sitter", slogger.Fields{
			"function_nodes_count": len(functionNodes),
			"method_nodes_count":   len(methodNodes),
		})
	}

	packageName := p.extractPackageNameFromTree(parseTree)

	// Extract regular functions
	for _, node := range functionNodes {
		if fn, err := p.parseGoFunction(node, packageName, parseTree); err == nil {
			functions = append(functions, fn)
			if enableLogging {
				slogger.Info(ctx, "Extracted function", slogger.Fields{
					"function_name": fn.Name,
				})
			}
		} else if enableLogging {
			slogger.Error(ctx, "Failed to parse function node", slogger.Fields{
				"error":     err.Error(),
				"node_type": node.Type,
			})
		}
	}

	// Extract methods
	for _, node := range methodNodes {
		if fn, err := p.parseGoMethod(node, packageName, parseTree); err == nil {
			functions = append(functions, fn)
			if enableLogging {
				slogger.Info(ctx, "Extracted method", slogger.Fields{
					"method_name": fn.Name,
				})
			}
		}
	}

	// Log if Tree-sitter parsing found no functions
	if enableLogging && len(functions) == 0 {
		slogger.Warn(ctx, "Tree-sitter found no function nodes - check parse tree and grammar", slogger.Fields{
			"function_nodes_searched": nodeTypeFunctionDecl,
			"method_nodes_searched":   nodeTypeMethodDecl,
		})
	}

	return functions, nil
}

// extractFunctionsFromSourceText is now removed - forcing Tree-sitter only parsing

// Regex-based parsing methods removed - Tree-sitter only parsing now

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

	// Enhanced input validation with detailed error messages
	if node == nil {
		return outbound.SemanticCodeChunk{}, errors.New("parseGoFunction: parse node cannot be nil")
	}
	if parseTree == nil {
		return outbound.SemanticCodeChunk{}, errors.New("parseGoFunction: parse tree cannot be nil")
	}
	if packageName == "" {
		packageName = defaultPackageName // Provide fallback for empty package name
	}

	// Debug: log all children types
	var childTypes []string
	for _, child := range node.Children {
		childTypes = append(childTypes, child.Type)
	}
	slogger.Debug(ctx, "Function node children", slogger.Fields{
		"child_types": childTypes,
	})

	// Find function name based on node type - grammar-informed approach
	var nameNode *valueobject.ParseNode
	for _, child := range node.Children {
		// function_declaration uses "identifier", method_declaration uses "field_identifier"
		if child.Type == "identifier" {
			nameNode = child
			break
		}
	}
	if nameNode == nil {
		return outbound.SemanticCodeChunk{}, fmt.Errorf(
			"parseGoFunction: function name node not found in node type '%s' - available children: %s",
			node.Type, strings.Join(childTypes, ", "),
		)
	}

	name := parseTree.GetNodeText(nameNode)
	visibility := p.getVisibility(name)

	qualifiedName := p.generateQualifiedName(name, packageName)
	parameters, returnType, genericParameters := p.parseGoFunctionSignature(node, parseTree)

	comments := p.extractPrecedingComments(node, parseTree)
	content := parseTree.GetNodeText(node)
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	id := p.generateFunctionChunkID(name, packageName, hashStr)

	return outbound.SemanticCodeChunk{
		ChunkID:           id,
		Name:              name,
		QualifiedName:     qualifiedName,
		Language:          valueobject.Go,
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

func (p *GoParser) generateQualifiedName(name, packageName string) string {
	// For main package or empty package names, use just the function name
	// For other packages, use package.function format
	if packageName == "" || packageName == defaultPackageName {
		return name
	}
	return packageName + "." + name
}

func (p *GoParser) parseGoFunctionSignature(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) ([]outbound.Parameter, string, []outbound.GenericParameter) {
	var parameters []outbound.Parameter
	var returnType string
	var genericParameters []outbound.GenericParameter

	// Use tree-sitter field access for more reliable parsing
	nameNode := findChildByTypeInNode(node, "identifier")

	// Track parameter_list nodes to distinguish parameters from result
	var parameterLists []*valueobject.ParseNode

	// Find parameters field and result field
	for _, child := range node.Children {
		switch child.Type {
		case nodeTypeParameterList:
			parameterLists = append(parameterLists, child)
		case nodeTypeTypeParameterList:
			genericParameters = p.parseGoGenericParameterList(child, parseTree)
		case nodeTypeTypeIdentifier,
			nodeTypePointerType,
			nodeTypeQualifiedType,
			nodeTypeTupleType,
			nodeTypeSliceType,
			nodeTypeChannelType:
			// Single return type (not in parameter_list)
			if nameNode != nil && child.StartByte > nameNode.StartByte {
				returnType = p.parseGoReturnType(child, parseTree)
			}
		case "parenthesized_type":
			// Handle parenthesized return types like (int, int, error)
			if nameNode != nil && child.StartByte > nameNode.StartByte {
				returnType = parseTree.GetNodeText(child)
			}
		}
	}

	// Process parameter_list nodes based on position relative to function name
	if nameNode != nil {
	parameterLoop:
		for _, paramList := range parameterLists {
			switch {
			case paramList.StartByte < nameNode.StartByte:
				// This is a receiver (for methods), skip
				continue
			case len(parameters) == 0:
				// First parameter_list after name = function parameters
				parameters = p.parseGoParameterList(paramList, parseTree)
			default:
				// Second parameter_list after name = return types (result field)
				returnType = p.parseGoReturnType(paramList, parseTree)
				break parameterLoop
			}
		}
	}

	return parameters, returnType, genericParameters
}

func (p *GoParser) generateFunctionChunkID(name, packageName, hashStr string) string {
	// Use consistent ChunkID format for all functions
	return fmt.Sprintf("func:%s", name)
}

func (p *GoParser) parseGoMethod(
	node *valueobject.ParseNode,
	packageName string,
	parseTree *valueobject.ParseTree,
) (outbound.SemanticCodeChunk, error) {
	if err := p.validateMethodNode(node, parseTree, &packageName); err != nil {
		return outbound.SemanticCodeChunk{}, err
	}

	nameNode, name, err := p.extractMethodName(node, parseTree)
	if err != nil {
		return outbound.SemanticCodeChunk{}, err
	}

	receiverInfo := p.extractMethodReceiver(node, nameNode, parseTree)
	qualifiedName := p.buildMethodQualifiedName(packageName, name, receiverInfo)
	parameters, returnType, genericParameters := p.parseMethodComponents(node, nameNode, parseTree)

	content, hash := p.extractMethodContent(node, parseTree)
	id := p.generateMethodID(name, packageName, receiverInfo, hash)

	return p.buildMethodChunk(
		id, name, qualifiedName, p.getVisibility(name),
		parameters, returnType, genericParameters,
		content, node, parseTree, hash,
	), nil
}

func (p *GoParser) validateMethodNode(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
	packageName *string,
) error {
	if node == nil {
		return errors.New("parseGoMethod: parse node cannot be nil")
	}
	if parseTree == nil {
		return errors.New("parseGoMethod: parse tree cannot be nil")
	}
	if *packageName == "" {
		*packageName = defaultPackageName
	}
	return nil
}

func (p *GoParser) extractMethodName(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) (*valueobject.ParseNode, string, error) {
	for _, child := range node.Children {
		if child.Type == nodeTypeFieldIdentifier || child.Type == nodeTypeIdentifier {
			name := parseTree.GetNodeText(child)
			return child, name, nil
		}
	}
	return nil, "", fmt.Errorf("parseGoMethod: method name node not found in node type '%s'", node.Type)
}

func (p *GoParser) extractMethodReceiver(
	node *valueobject.ParseNode,
	nameNode *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) string {
	parameterNodes := findChildrenByType(node, nodeTypeParameterList)
	if len(parameterNodes) > 0 && parameterNodes[0].StartByte < nameNode.StartByte {
		return parseTree.GetNodeText(parameterNodes[0])
	}
	return ""
}

func (p *GoParser) buildMethodQualifiedName(packageName, name, receiverInfo string) string {
	qualifiedName := packageName + "." + name
	if receiverInfo != "" {
		if receiverType := extractReceiverTypeName(receiverInfo); receiverType != "" {
			qualifiedName = receiverType + "." + name
		}
	}
	return qualifiedName
}

func (p *GoParser) parseMethodComponents(
	node *valueobject.ParseNode,
	nameNode *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) ([]outbound.Parameter, string, []outbound.GenericParameter) {
	var parameters []outbound.Parameter
	var returnType string
	var genericParameters []outbound.GenericParameter

	for _, child := range node.Children {
		switch child.Type {
		case nodeTypeParameterList:
			if child.StartByte >= nameNode.StartByte {
				parameters = p.parseGoParameterList(child, parseTree)
			}
		case nodeTypeTypeParameterList:
			genericParameters = p.parseGoGenericParameterList(child, parseTree)
		case nodeTypeTypeIdentifier, nodeTypePointerType, nodeTypeQualifiedType, nodeTypeTupleType, nodeTypeSliceType:
			returnType = p.parseGoReturnType(child, parseTree)
		}
	}
	return parameters, returnType, genericParameters
}

func (p *GoParser) extractMethodContent(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) (string, string) {
	content := parseTree.GetNodeText(node)
	hash := sha256.Sum256([]byte(content))
	return content, hex.EncodeToString(hash[:])
}

func (p *GoParser) generateMethodID(name, packageName, receiverInfo, hashStr string) string {
	if receiverInfo != "" {
		if receiverType := extractReceiverTypeName(receiverInfo); receiverType != "" {
			return fmt.Sprintf("method:%s:%s", receiverType, name)
		}
		return utils.GenerateID(string(outbound.ConstructMethod), name, map[string]interface{}{
			"package":  packageName,
			"receiver": receiverInfo,
			"hash":     hashStr,
		})
	}
	return utils.GenerateID(string(outbound.ConstructMethod), name, map[string]interface{}{
		"package": packageName,
		"hash":    hashStr,
	})
}

func (p *GoParser) buildMethodChunk(
	id, name, qualifiedName string,
	visibility outbound.VisibilityModifier,
	parameters []outbound.Parameter,
	returnType string,
	genericParameters []outbound.GenericParameter,
	content string,
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
	hashStr string,
) outbound.SemanticCodeChunk {
	comments := p.extractPrecedingComments(node, parseTree)
	return outbound.SemanticCodeChunk{
		ChunkID:           id,
		Name:              name,
		QualifiedName:     qualifiedName,
		Language:          valueobject.Go,
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
	}
}

func (p *GoParser) parseGoParameterList(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) []outbound.Parameter {
	parameters := make([]outbound.Parameter, 0)

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

	// Also handle variadic parameter declarations
	variadicNodes := findChildrenByType(node, nodeTypeVariadicParameterDecl)
	for _, variadicNode := range variadicNodes {
		allParams := p.parseGoParameterDeclarations(variadicNode, parseTree)
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
	genericParams := make([]outbound.GenericParameter, 0)

	typeParams := findChildrenByType(node, "type_parameter_declaration")

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

	// Get the type node - enhanced to handle more complex types
	var typeNode *valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeTypeTypeIdentifier || child.Type == nodeTypePointerType ||
			child.Type == nodeTypeQualifiedType || child.Type == nodeTypeArrayType ||
			child.Type == nodeTypeSliceType || child.Type == nodeTypeInterfaceType ||
			child.Type == nodeTypeChannelType || child.Type == nodeTypeFunctionType ||
			child.Type == nodeTypeGenericType || child.Type == "receive_type" ||
			child.Type == "send_type" {
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
		// For variadic parameters, prefix the type with "..."
		if node.Type == nodeTypeVariadicParameterDecl {
			param.Type = "..." + param.Type
		}
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
	// Get the type node - enhanced to handle more complex types
	var typeNode *valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeTypeTypeIdentifier || child.Type == nodeTypePointerType ||
			child.Type == nodeTypeQualifiedType || child.Type == nodeTypeArrayType ||
			child.Type == nodeTypeSliceType || child.Type == nodeTypeInterfaceType ||
			child.Type == nodeTypeFunctionType || child.Type == nodeTypeChannelType ||
			child.Type == nodeTypeGenericType || child.Type == "receive_type" ||
			child.Type == "send_type" {
			typeNode = child
			break
		}
	}

	var typeStr string
	if typeNode != nil {
		typeStr = parseTree.GetNodeText(typeNode)
		// For variadic parameters, prefix the type with "..."
		if node.Type == nodeTypeVariadicParameterDecl {
			typeStr = "..." + typeStr
		}
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
			// For variadic parameters, prefix the type with "..."
			if node.Type == nodeTypeVariadicParameterDecl && param.Type != "" {
				param.Type = "..." + param.Type
			}
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
	case nodeTypeTypeIdentifier, nodeTypePointerType, nodeTypeQualifiedType,
		nodeTypeSliceType, nodeTypeChannelType, nodeTypeFunctionType,
		nodeTypeInterfaceType, nodeTypeArrayType:
		return parseTree.GetNodeText(node)
	case nodeTypeTupleType, "parenthesized_type":
		// Handle multiple return values like (int, int, error)
		return parseTree.GetNodeText(node)
	case nodeTypeParameterList:
		// Handle multiple return values in parameter_list format
		var types []string
		for _, child := range node.Children {
			if child.Type == "parameter_declaration" {
				if typeNode := p.getParameterType(child); typeNode != nil {
					types = append(types, parseTree.GetNodeText(typeNode))
				}
			}
		}
		if len(types) > 1 {
			return "(" + strings.Join(types, ", ") + ")"
		} else if len(types) == 1 {
			return types[0]
		}
		return ""
	default:
		return ""
	}
}

func (p *GoParser) parseGoGenericParameter(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) outbound.GenericParameter {
	var param outbound.GenericParameter

	// Get the name (identifier) - can have multiple identifiers for same constraint
	identifiers := findChildrenByType(node, "identifier")
	if len(identifiers) > 0 {
		// Use the first identifier as the parameter name
		param.Name = parseTree.GetNodeText(identifiers[0])
	}

	// Get the type constraint - look for various constraint node types
	var constraints []string
	for _, child := range node.Children {
		switch child.Type {
		case "type_constraint", nodeTypeTypeIdentifier, "interface_type", "union_type":
			constraintText := parseTree.GetNodeText(child)
			// Clean up constraint text (remove extra spaces/formatting)
			constraintText = strings.TrimSpace(constraintText)
			if constraintText != "" {
				constraints = append(constraints, constraintText)
			}
		}
	}

	// Default to "any" if no constraints found (common case)
	if len(constraints) == 0 {
		constraints = []string{"any"}
	}

	param.Constraints = constraints
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
