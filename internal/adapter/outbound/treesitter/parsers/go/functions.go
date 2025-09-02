package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// ExtractFunctions extracts all function definitions from a Go parse tree.
func (p *GoParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Go functions", slogger.Fields{
		"include_private": options.IncludePrivate,
		"max_depth":       options.MaxDepth,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	var functions []outbound.SemanticCodeChunk
	packageName := p.extractPackageNameFromTree(parseTree)
	now := time.Now()

	// Find function declarations
	functionNodes := parseTree.GetNodesByType("function_declaration")
	for _, node := range functionNodes {
		function := p.parseGoFunction(ctx, parseTree, node, packageName, options, now)
		if function != nil {
			functions = append(functions, *function)
		}
	}

	// Find method declarations
	methodNodes := parseTree.GetNodesByType("method_declaration")
	for _, node := range methodNodes {
		method := p.parseGoMethod(ctx, parseTree, node, packageName, options, now)
		if method != nil {
			functions = append(functions, *method)
		}
	}

	slogger.Info(ctx, "Go function extraction completed", slogger.Fields{
		"extracted_count": len(functions),
	})

	return functions, nil
}

// parseGoFunction parses a Go function declaration.
func (p *GoParser) parseGoFunction(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find function name
	nameNode := p.findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	functionName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(node)
	visibility := p.getVisibility(functionName)

	// Skip private functions if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Parse parameters
	parameters := p.parseGoParameters(parseTree, node)

	// Parse return type
	returnType := p.parseGoReturnType(parseTree, node)

	// Parse generic parameters
	var genericParams []outbound.GenericParameter
	isGeneric := false
	typeParams := p.findChildByType(node, "type_parameter_list")
	if typeParams != nil {
		isGeneric = true
		genericParams = p.parseGoGenericParameters(parseTree, typeParams)
	}

	// Extract documentation
	documentation := ""
	if options.IncludeDocumentation {
		documentation = p.extractPrecedingComments(parseTree, node)
	}

	return &outbound.SemanticCodeChunk{
		ID:                utils.GenerateID("function", functionName, nil),
		Type:              outbound.ConstructFunction,
		Name:              functionName,
		QualifiedName:     packageName + "." + functionName,
		Language:          parseTree.Language(),
		StartByte:         node.StartByte,
		EndByte:           node.EndByte,
		Content:           content,
		Documentation:     documentation,
		Parameters:        parameters,
		ReturnType:        returnType,
		Visibility:        visibility,
		IsGeneric:         isGeneric,
		GenericParameters: genericParams,
		ExtractedAt:       now,
		Hash:              utils.GenerateHash(content),
	}
}

// parseGoMethod parses a Go method declaration.
func (p *GoParser) parseGoMethod(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find method name
	nameNode := p.findChildByType(node, "field_identifier")
	if nameNode == nil {
		return nil
	}

	methodName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(node)
	visibility := p.getVisibility(methodName)

	// Skip private methods if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Parse receiver to get the type
	receiver := p.findChildByType(node, "parameter_list")
	receiverType := ""
	qualifiedName := methodName // Default to just method name

	if receiver != nil {
		receiverType = p.parseGoReceiver(parseTree, receiver)
		if receiverType != "" {
			// Extract base type name for qualified name (remove * prefix)
			baseType := receiverType
			if strings.HasPrefix(receiverType, "*") {
				baseType = receiverType[1:]
			}
			qualifiedName = baseType + "." + methodName // Type.Method format, no package prefix
		}
	}

	// Parse parameters (skip receiver)
	parameters := p.parseGoMethodParameters(parseTree, node, receiverType)

	// Parse return type
	returnType := p.parseGoReturnType(parseTree, node)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("method", methodName, nil),
		Type:          outbound.ConstructMethod,
		Name:          methodName,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       content,
		Parameters:    parameters,
		ReturnType:    returnType,
		Visibility:    visibility,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// parseGoParameters parses function parameters.
func (p *GoParser) parseGoParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []outbound.Parameter {
	var parameters []outbound.Parameter

	paramList := p.findChildByType(node, "parameter_list")
	if paramList == nil {
		return parameters
	}

	paramDecls := p.findChildrenByType(paramList, "parameter_declaration")
	for _, paramDecl := range paramDecls {
		params := p.parseGoParameterDeclaration(parseTree, paramDecl)
		parameters = append(parameters, params...)
	}

	return parameters
}

// parseGoParameterDeclaration parses a parameter declaration.
func (p *GoParser) parseGoParameterDeclaration(
	parseTree *valueobject.ParseTree,
	paramDecl *valueobject.ParseNode,
) []outbound.Parameter {
	var parameters []outbound.Parameter

	// Find all identifiers (parameter names)
	identifiers := p.findChildrenByType(paramDecl, "identifier")

	// Find type
	var typeText string
	for _, child := range paramDecl.Children {
		if child.Type != "identifier" && child.Type != "," {
			typeText = parseTree.GetNodeText(child)
			break
		}
	}

	// If no identifiers found, this might be an unnamed parameter
	if len(identifiers) == 0 {
		// Check if this is just a type (unnamed parameter)
		if typeText != "" {
			parameters = append(parameters, outbound.Parameter{
				Name: "",
				Type: typeText,
			})
		}
		return parameters
	}

	// Create parameters for each identifier
	for _, identifier := range identifiers {
		name := parseTree.GetNodeText(identifier)
		parameters = append(parameters, outbound.Parameter{
			Name: name,
			Type: typeText,
		})
	}

	return parameters
}

// parseGoReturnType parses function return type.
func (p *GoParser) parseGoReturnType(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) string {
	// Look for return type after parameter list
	paramList := p.findChildByType(node, "parameter_list")
	if paramList == nil {
		return ""
	}

	// Find the type after the parameter list
	for i, child := range node.Children {
		if child == paramList && i+1 < len(node.Children) {
			nextChild := node.Children[i+1]
			if nextChild.Type != "block" {
				return parseTree.GetNodeText(nextChild)
			}
			break
		}
	}

	return ""
}

// parseGoMethodParameters parses method parameters, excluding the receiver.
func (p *GoParser) parseGoMethodParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	_ string,
) []outbound.Parameter {
	var parameters []outbound.Parameter

	// Parse receiver parameter from first parameter list
	receiverList := p.findChildByType(node, "parameter_list")
	if receiverList != nil {
		receiverParams := p.parseGoParameterList(parseTree, receiverList)
		parameters = append(parameters, receiverParams...)
	}

	// Parse regular method parameters from second parameter list
	// For methods, there can be multiple parameter_list children
	paramLists := p.findChildrenByType(node, "parameter_list")
	if len(paramLists) > 1 {
		// Second parameter list contains method parameters
		methodParams := p.parseGoParameterList(parseTree, paramLists[1])
		parameters = append(parameters, methodParams...)
	}

	return parameters
}

// parseGoParameterList parses parameters from a parameter_list node.
func (p *GoParser) parseGoParameterList(
	parseTree *valueobject.ParseTree,
	paramList *valueobject.ParseNode,
) []outbound.Parameter {
	var parameters []outbound.Parameter

	paramDecls := p.findChildrenByType(paramList, "parameter_declaration")
	for _, paramDecl := range paramDecls {
		params := p.parseGoParameterDeclaration(parseTree, paramDecl)
		parameters = append(parameters, params...)
	}

	return parameters
}

// extractPrecedingComments extracts comments that precede a node.
func (p *GoParser) extractPrecedingComments(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Simple implementation for nw - return empty
	// Full implementation would traverse backwards to find comments
	return ""
}
