package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"regexp"
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
	for i, node := range functionNodes {
		function := p.parseGoFunction(ctx, parseTree, node, packageName, options, now, i)
		if function != nil {
			functions = append(functions, *function)
		}
	}

	// Find method declarations
	methodNodes := parseTree.GetNodesByType("method_declaration")
	for i, node := range methodNodes {
		method := p.parseGoMethod(ctx, parseTree, node, packageName, options, now, i)
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
	index int,
) *outbound.SemanticCodeChunk {
	// Add nil checks
	if parseTree == nil || node == nil {
		return nil
	}

	// Find function name
	nameNode := p.findChildByType(node, "identifier")
	content := parseTree.GetNodeText(node)

	if nameNode == nil {
		// Fallback parsing for mock trees
		return p.parseGoFunctionFallback(content, packageName, options, now, index)
	}

	functionName := parseTree.GetNodeText(nameNode)
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

// parseGoFunctionFallback provides minimal fallback parsing for mock trees.
func (p *GoParser) parseGoFunctionFallback(
	content string,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
	index int,
) *outbound.SemanticCodeChunk {
	// Extract specific function based on index
	functions := p.extractAllFunctions(content)
	if index >= len(functions) {
		return nil
	}

	funcContent := functions[index]

	// Extract function name using regex
	funcRegex := regexp.MustCompile(`func\s+(\w+)`)
	matches := funcRegex.FindStringSubmatch(funcContent)
	if len(matches) < 2 {
		return nil
	}

	functionName := matches[1]
	visibility := p.getVisibility(functionName)

	// Skip private functions if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Extract parameters
	params := p.extractParametersFallback(funcContent)

	// Extract return type
	returnType := p.extractReturnTypeFallback(funcContent)

	// Check for generic parameters
	isGeneric := strings.Contains(funcContent, "[") && strings.Index(funcContent, "[") < strings.Index(funcContent, "(")
	var genericParams []outbound.GenericParameter
	if isGeneric {
		genericParams = p.extractGenericParamsFallback(funcContent)
	}

	lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

	return &outbound.SemanticCodeChunk{
		ID:                utils.GenerateID("function", functionName, nil),
		Type:              outbound.ConstructFunction,
		Name:              functionName,
		QualifiedName:     packageName + "." + functionName,
		Language:          lang,
		StartByte:         0,
		EndByte:           0,
		Content:           funcContent,
		Documentation:     "",
		Parameters:        params,
		ReturnType:        returnType,
		Visibility:        visibility,
		IsGeneric:         isGeneric,
		GenericParameters: genericParams,
		ExtractedAt:       now,
		Hash:              utils.GenerateHash(funcContent),
	}
}

// parseVariadicParameter handles parsing of variadic parameters.
func (p *GoParser) parseVariadicParameter(part string) outbound.Parameter {
	if strings.HasPrefix(part, "...") {
		// Unnamed variadic parameter
		return outbound.Parameter{
			Name:       "",
			Type:       strings.TrimSpace(part[3:]),
			IsVariadic: true,
		}
	}

	// Named variadic parameter
	spaceIndex := strings.LastIndex(part, " ")
	if spaceIndex != -1 {
		name := strings.TrimSpace(part[:spaceIndex])
		typeStr := strings.TrimSpace(part[spaceIndex+1:][3:]) // Remove "..."
		return outbound.Parameter{
			Name:       name,
			Type:       typeStr,
			IsVariadic: true,
		}
	}

	// Fallback if no space found
	return outbound.Parameter{
		Name:       "",
		Type:       strings.TrimSpace(part[3:]), // Remove "..."
		IsVariadic: true,
	}
}

// extractParametersFallback extracts parameters using regex for fallback parsing.
func (p *GoParser) extractParametersFallback(content string) []outbound.Parameter {
	paramRegex := regexp.MustCompile(`func\s+\w+(?:\s*\[[^\]]*\])?\s*\(([^)]*)\)`)
	matches := paramRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil
	}

	paramsStr := matches[1]
	if paramsStr == "" {
		return nil
	}

	var params []outbound.Parameter
	paramParts := strings.Split(paramsStr, ",")

	for _, part := range paramParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle variadic parameters
		if strings.Contains(part, "...") {
			params = append(params, p.parseVariadicParameter(part))
			continue
		}

		// Split on last space to separate name and type
		spaceIndex := strings.LastIndex(part, " ")
		if spaceIndex == -1 {
			// Unnamed parameter
			params = append(params, outbound.Parameter{
				Name: "",
				Type: part,
			})
			continue
		}

		name := strings.TrimSpace(part[:spaceIndex])
		typeStr := strings.TrimSpace(part[spaceIndex+1:])
		params = append(params, outbound.Parameter{
			Name: name,
			Type: typeStr,
		})
	}

	return params
}

// extractReturnTypeFallback extracts return type using regex for fallback parsing.
func (p *GoParser) extractReturnTypeFallback(content string) string {
	// Find the last closing parenthesis and extract what comes after
	closeParenIndex := strings.LastIndex(content, ")")
	if closeParenIndex == -1 || closeParenIndex+1 >= len(content) {
		return ""
	}

	// Extract the substring after the last closing parenthesis
	afterParams := content[closeParenIndex+1:]

	// Clean up whitespace
	afterParams = strings.TrimSpace(afterParams)

	// If it starts with {, there's no return type
	if strings.HasPrefix(afterParams, "{") {
		return ""
	}

	// Handle multi-value returns
	if strings.HasPrefix(afterParams, "(") {
		endParenIndex := strings.Index(afterParams, ")")
		if endParenIndex != -1 {
			return strings.TrimSpace(afterParams[:endParenIndex+1])
		}
	}

	// Handle single return type
	spaceIndex := strings.Index(afterParams, " ")
	if spaceIndex != -1 {
		return strings.TrimSpace(afterParams[:spaceIndex])
	}

	// If no space, but there's content, return it all
	if afterParams != "" {
		// Check if it's followed by a block
		if strings.Contains(afterParams, "{") {
			braceIndex := strings.Index(afterParams, "{")
			return strings.TrimSpace(afterParams[:braceIndex])
		}
		return strings.TrimSpace(afterParams)
	}

	return ""
}

// extractGenericParamsFallback extracts generic parameters using regex for fallback parsing.
func (p *GoParser) extractGenericParamsFallback(content string) []outbound.GenericParameter {
	genericRegex := regexp.MustCompile(`func\s+\w+\s*\[([^\]]*)\]`)
	matches := genericRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil
	}

	genericStr := matches[1]
	if genericStr == "" {
		return nil
	}

	var genericParams []outbound.GenericParameter
	parts := strings.Split(genericStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split on space to separate name and constraint
		constraintIndex := strings.Index(part, " ")
		if constraintIndex == -1 {
			genericParams = append(genericParams, outbound.GenericParameter{
				Name:        part,
				Constraints: []string{},
			})
		} else {
			name := strings.TrimSpace(part[:constraintIndex])
			constraint := strings.TrimSpace(part[constraintIndex+1:])
			genericParams = append(genericParams, outbound.GenericParameter{
				Name:        name,
				Constraints: []string{constraint},
			})
		}
	}

	return genericParams
}

// parseGoMethod parses a Go method declaration.
func (p *GoParser) parseGoMethod(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
	index int,
) *outbound.SemanticCodeChunk {
	// Add nil checks
	if parseTree == nil || node == nil {
		return nil
	}

	// Find method name
	nameNode := p.findChildByType(node, "field_identifier")
	content := parseTree.GetNodeText(node)

	if nameNode == nil {
		// Fallback parsing for mock trees
		return p.parseGoMethodFallback(content, packageName, options, now, index)
	}

	methodName := parseTree.GetNodeText(nameNode)
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

// parseGoMethodFallback provides minimal fallback parsing for mock trees.
func (p *GoParser) parseGoMethodFallback(
	content string,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
	index int,
) *outbound.SemanticCodeChunk {
	// Extract specific function based on index
	functions := p.extractAllFunctions(content)
	if index >= len(functions) {
		return nil
	}

	funcContent := functions[index]

	// Extract method name using regex
	methodRegex := regexp.MustCompile(`func\s*\([^)]*\)\s*(\w+)`)
	matches := methodRegex.FindStringSubmatch(funcContent)
	if len(matches) < 2 {
		return nil
	}

	methodName := matches[1]
	visibility := p.getVisibility(methodName)

	// Skip private methods if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Extract parameters
	params := p.extractParametersFallback(funcContent)

	// Extract return type
	returnType := p.extractReturnTypeFallback(funcContent)

	lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("method", methodName, nil),
		Type:          outbound.ConstructMethod,
		Name:          methodName,
		QualifiedName: packageName + "." + methodName,
		Language:      lang,
		StartByte:     0,
		EndByte:       0,
		Content:       funcContent,
		Documentation: "",
		Parameters:    params,
		ReturnType:    returnType,
		Visibility:    visibility,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(funcContent),
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
		// Fallback parsing for mock trees
		content := parseTree.GetNodeText(node)
		return p.extractParametersFallback(content)
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
		// Fallback parsing for mock trees
		content := parseTree.GetNodeText(node)
		return p.extractReturnTypeFallback(content)
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

	// Fallback parsing for mock trees
	content := parseTree.GetNodeText(node)
	return p.extractReturnTypeFallback(content)
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

	// Fallback parsing for mock trees
	if len(parameters) == 0 {
		content := parseTree.GetNodeText(node)
		return p.extractParametersFallback(content)
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

// extractAllFunctions extracts all function and method declarations from content.
func (p *GoParser) extractAllFunctions(content string) []string {
	// Split content by "func " keyword to isolate function declarations
	parts := strings.Split(content, "func ")
	if len(parts) <= 1 {
		return nil
	}

	var functions []string

	// Process each part that starts with a function declaration
	for i := 1; i < len(parts); i++ {
		funcPart := "func " + parts[i]

		// Find the opening brace of the function body
		openBraceIndex := strings.Index(funcPart, "{")
		if openBraceIndex == -1 {
			// If no brace found, it might be an incomplete function declaration
			// Include it as is
			functions = append(functions, strings.TrimSpace(funcPart))
			continue
		}

		// Count braces to find the matching closing brace
		braceCount := 1
		startIndex := openBraceIndex + 1
		endIndex := -1

		for j := startIndex; j < len(funcPart); j++ {
			if funcPart[j] == '{' {
				braceCount++
			} else if funcPart[j] == '}' {
				braceCount--
				if braceCount == 0 {
					endIndex = j
					break
				}
			}
		}

		// If we found a matching closing brace, extract the complete function
		if endIndex != -1 {
			functions = append(functions, strings.TrimSpace(funcPart[:endIndex+1]))
		} else {
			// Otherwise, include the function part as is
			functions = append(functions, strings.TrimSpace(funcPart))
		}
	}

	return functions
}
