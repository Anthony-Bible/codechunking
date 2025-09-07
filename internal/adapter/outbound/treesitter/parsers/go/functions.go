package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

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
		if child.Type == "parameter_list" {
			parameters = p.parseGoParameterList(child, parseTree)
		} else if child.Type == "type_parameter_list" {
			genericParameters = p.parseGoGenericParameterList(child, parseTree)
		} else if child.Type == "type_identifier" || child.Type == "pointer_type" || child.Type == "qualified_type" || child.Type == "tuple_type" {
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
		ID:                id,
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
		if child.Type == "parameter_list" {
			// Skip receiver parameter list
			if child.StartByte < nameNode.StartByte {
				continue
			}
			parameters = p.parseGoParameterList(child, parseTree)
		} else if child.Type == "type_parameter_list" {
			genericParameters = p.parseGoGenericParameterList(child, parseTree)
		} else if child.Type == "type_identifier" || child.Type == "pointer_type" || child.Type == "qualified_type" || child.Type == "tuple_type" {
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
		ID:                id,
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
		if child.Type == "type_identifier" || child.Type == "pointer_type" ||
			child.Type == "qualified_type" || child.Type == "array_type" {
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
	case "type_identifier", "pointer_type", "qualified_type":
		return parseTree.GetNodeText(node)
	case "tuple_type":
		// Handle multiple return values
		var types []string
		for _, child := range node.Children {
			if child.Type == "type_identifier" || child.Type == "pointer_type" || child.Type == "qualified_type" {
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
		if child.Type == "type_identifier" || child.Type == "qualified_type" {
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

// parseGoParameters parses parameter list from a method specification node
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

// parseGoReturnType extracts return type from a method specification node
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
