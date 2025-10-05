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
	// Validate input parse tree
	if err := o.parser.validateInput(parseTree); err != nil {
		return nil, err
	}

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

	// Check for syntax errors in the parse tree with specific error detection
	if err := detectSpecificSyntaxError(parseTree); err != nil {
		return nil, err
	}

	// Check for resource limits (memory exhaustion, too many functions, etc.)
	if err := detectResourceLimits(parseTree); err != nil {
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
		if child.Type == nodeTypeIdentifier {
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
	nameNodes := FindChildrenRecursive(node, "identifier")
	var nameNode *valueobject.ParseNode
	if len(nameNodes) > 0 {
		nameNode = nameNodes[0]
	}

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

	qualifiedName := p.buildMethodQualifiedName(packageName, name, node, parseTree)
	parameters, returnType, genericParameters := p.parseMethodComponents(node, nameNode, parseTree)

	content, hash := p.extractMethodContent(node, parseTree)
	id := p.generateMethodID(name, packageName, hash, node, parseTree)

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

func (p *GoParser) buildMethodQualifiedName(
	packageName, name string,
	methodNode *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) string {
	qualifiedName := packageName + "." + name
	if methodNode != nil && parseTree != nil {
		if receiverType := extractReceiverTypeFromNode(methodNode, parseTree); receiverType != "" {
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

func (p *GoParser) generateMethodID(
	name, packageName, hashStr string,
	methodNode *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) string {
	if methodNode != nil && parseTree != nil {
		if receiverType := extractReceiverTypeFromNode(methodNode, parseTree); receiverType != "" {
			return fmt.Sprintf("method:%s:%s", receiverType, name)
		}
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

// extractReceiverTypeFromNode extracts the receiver type name using tree-sitter field access.
// This replaces the manual string parsing in extractReceiverTypeName() with proper AST navigation
// that handles complex types like generics, qualified types, channels, functions, etc.
//
// For method_declaration nodes, this:
// 1. Uses field access to get the 'receiver' field (parameter_list)
// 2. Gets the first parameter_declaration from the receiver
// 3. Uses field access to get the 'type' field
// 4. Handles pointer types by accessing the underlying type
// 5. Extracts the base type name for complex types
//
// Parameters:
//
//	methodNode - The method_declaration node
//	parseTree - The parse tree for text extraction
//
// Returns:
//
//	string - The receiver type name, or empty string if not found
//
// Example:
//
//	For "func (p *Person) GetName() string", returns "Person"
//	For "func (s *Service[T]) Process() error", returns "Service"
//	For "func (m map[string]int) Size() int", returns "map[string]int"
func extractReceiverTypeFromNode(methodNode *valueobject.ParseNode, parseTree *valueobject.ParseTree) string {
	if methodNode == nil || parseTree == nil {
		return ""
	}

	ctx := context.TODO()
	slogger.Debug(ctx, "extractReceiverTypeFromNode called", slogger.Fields{
		"methodNode_type": methodNode.Type,
	})

	// Use field access to get the receiver field
	receiverField := methodNode.ChildByFieldName("receiver")
	if receiverField == nil {
		slogger.Debug(ctx, "No receiver field found, trying manual traversal", slogger.Fields{
			"methodNode_type": methodNode.Type,
		})
		// Fallback: manually traverse children to find receiver parameter_list
		for _, child := range methodNode.Children {
			if child.Type == nodeTypeParameterList {
				// This might be the receiver parameter_list (first parameter_list in method)
				receiverField = child
				break
			}
		}
		if receiverField == nil {
			return ""
		}
	}

	slogger.Debug(ctx, "Found receiver field", slogger.Fields{
		"receiverField_type": receiverField.Type,
	})

	// The receiver field should be a parameter_list containing parameter_declaration
	var paramDecl *valueobject.ParseNode
	slogger.Debug(ctx, "Searching for parameter_declaration in receiver", slogger.Fields{
		"receiverField_children": func() []string {
			var types []string
			for _, child := range receiverField.Children {
				types = append(types, child.Type)
			}
			return types
		}(),
	})
	for _, child := range receiverField.Children {
		if child.Type == "parameter_declaration" {
			paramDecl = child
			break
		}
	}

	if paramDecl == nil {
		slogger.Debug(ctx, "No parameter_declaration found in receiver", slogger.Fields{
			"receiverField_children": func() []string {
				var types []string
				for _, child := range receiverField.Children {
					types = append(types, child.Type)
				}
				return types
			}(),
		})
		return ""
	}

	slogger.Debug(ctx, "Found parameter declaration", slogger.Fields{
		"paramDecl_type": paramDecl.Type,
	})

	// Use field access to get the type field from parameter_declaration
	typeField := paramDecl.ChildByFieldName("type")
	if typeField == nil {
		slogger.Debug(ctx, "No type field found in parameter_declaration, trying manual traversal", slogger.Fields{
			"paramDecl_children": func() []string {
				var types []string
				for _, child := range paramDecl.Children {
					types = append(types, child.Type)
				}
				return types
			}(),
		})
		// Fallback: look for type nodes manually
		for _, child := range paramDecl.Children {
			if strings.Contains(child.Type, "type") ||
				child.Type == nodeTypeTypeIdentifier ||
				child.Type == nodeTypePointerType ||
				child.Type == nodeTypeQualifiedType {
				typeField = child
				break
			}
		}
		if typeField == nil {
			return ""
		}
	}

	slogger.Debug(ctx, "Found type field", slogger.Fields{
		"typeField_type": typeField.Type,
	})

	// Handle different type structures through field access
	result := extractTypeNameFromTypeNode(typeField, parseTree)
	slogger.Debug(ctx, "Extracted receiver type", slogger.Fields{
		"result": result,
	})

	return result
}

// extractTypeNameFromTypeNode extracts the base type name from a type node using field access.
// This handles various type structures like pointer_type, generic_type, qualified_type, etc.
func extractTypeNameFromTypeNode(typeNode *valueobject.ParseNode, parseTree *valueobject.ParseTree) string {
	if typeNode == nil || parseTree == nil {
		return ""
	}

	ctx := context.TODO()
	slogger.Debug(ctx, "extractTypeNameFromTypeNode called", slogger.Fields{
		"typeNode_type": typeNode.Type,
	})

	switch typeNode.Type {
	case nodeTypePointerType:
		slogger.Debug(ctx, "Processing pointer type", slogger.Fields{})
		// For pointer types, get the underlying type via field access
		underlyingType := typeNode.ChildByFieldName("type")
		if underlyingType != nil {
			slogger.Debug(ctx, "Found underlying type via field access", slogger.Fields{
				"underlyingType_type": underlyingType.Type,
			})
			return extractTypeNameFromTypeNode(underlyingType, parseTree)
		}
		slogger.Debug(ctx, "No underlying type found via field access, trying manual traversal", slogger.Fields{
			"typeNode_children": func() []string {
				var types []string
				for _, child := range typeNode.Children {
					types = append(types, child.Type)
				}
				return types
			}(),
		})
		// Fallback: manual traversal for pointer types
		for _, child := range typeNode.Children {
			if child.Type == nodeTypeTypeIdentifier ||
				child.Type == nodeTypeQualifiedType ||
				strings.Contains(child.Type, "type") {
				slogger.Debug(ctx, "Found underlying type via manual traversal", slogger.Fields{
					"child_type": child.Type,
				})
				return extractTypeNameFromTypeNode(child, parseTree)
			}
		}
		return ""

	case "generic_type":
		// For generic types like Service[T], get the base type via field access
		baseType := typeNode.ChildByFieldName("type")
		if baseType != nil {
			return extractTypeNameFromTypeNode(baseType, parseTree)
		}
		return ""

	case "qualified_type":
		// For qualified types like pkg.Type, return the full qualified name
		return parseTree.GetNodeText(typeNode)

	case nodeTypeTypeIdentifier:
		// For simple type identifiers, return the text directly
		return parseTree.GetNodeText(typeNode)

	case "map_type", "slice_type", "channel_type", "function_type", nodeTypeInterfaceType, "array_type":
		// For complex types, return the full type text
		return parseTree.GetNodeText(typeNode)

	default:
		// For any other type, return the text representation
		return parseTree.GetNodeText(typeNode)
	}
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
		// For named returns like (result string, count int, err error)
		// we need to extract both names and types
		var returnParts []string
		for _, child := range node.Children {
			if child.Type == "parameter_declaration" {
				// Extract parameter names (identifiers) and type
				var names []string
				var typeText string

				for _, paramChild := range child.Children {
					if paramChild.Type == nodeTypeIdentifier {
						names = append(names, parseTree.GetNodeText(paramChild))
					}
				}

				if typeNode := p.getParameterType(child); typeNode != nil {
					typeText = parseTree.GetNodeText(typeNode)
				}

				// Build the return part with names if present
				if len(names) > 0 && typeText != "" {
					// Named returns: "name type" or "name1, name2 type"
					returnParts = append(returnParts, strings.Join(names, ", ")+" "+typeText)
				} else if typeText != "" {
					// Unnamed returns: just the type
					returnParts = append(returnParts, typeText)
				}
			}
		}
		if len(returnParts) > 1 {
			return "(" + strings.Join(returnParts, ", ") + ")"
		} else if len(returnParts) == 1 {
			return returnParts[0]
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
		case "type_constraint", nodeTypeTypeIdentifier, nodeTypeInterfaceType, "union_type":
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
	// Use TreeSitterQueryEngine for consistent comment extraction
	queryEngine := NewTreeSitterQueryEngine()
	documentationComments := queryEngine.QueryDocumentationComments(parseTree, node)

	// Process each comment using TreeSitterQueryEngine
	var comments []string
	for _, commentNode := range documentationComments {
		processedText := queryEngine.ProcessCommentText(parseTree, commentNode)
		if processedText != "" {
			comments = append(comments, processedText)
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
		if child.Type == nodeTypeTypeIdentifier || child.Type == nodeTypePointerType ||
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
