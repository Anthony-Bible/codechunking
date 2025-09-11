package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"time"
)

// ObservableGoParser delegation methods for variable extraction

// ExtractVariables extracts variable and constant declarations from a Go parse tree.
func (o *ObservableGoParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return o.parser.ExtractVariables(ctx, parseTree, options)
}

// ExtractVariables extracts variable and constant declarations from a Go parse tree.
func (p *GoParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Go variables", slogger.Fields{})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	var variables []outbound.SemanticCodeChunk
	packageName := p.extractPackageNameFromTree(parseTree)
	now := time.Now()

	// Find variable declarations
	varNodes := parseTree.GetNodesByType("var_declaration")
	for _, node := range varNodes {
		vars := parseGoVariableDeclaration(parseTree, node, packageName, outbound.ConstructVariable, options, now)
		variables = append(variables, vars...)
	}

	// Find constant declarations
	constNodes := parseTree.GetNodesByType("const_declaration")
	for _, node := range constNodes {
		consts := parseGoVariableDeclaration(parseTree, node, packageName, outbound.ConstructConstant, options, now)
		variables = append(variables, consts...)
	}

	// Find type declarations
	typeDecls := parseGoTypeDeclarations(parseTree, packageName, options, now)
	variables = append(variables, typeDecls...)

	return variables, nil
}

// parseGoVariableDeclaration parses variable/constant declarations.
func parseGoVariableDeclaration(
	parseTree *valueobject.ParseTree,
	varDecl *valueobject.ParseNode,
	packageName string,
	constructType outbound.SemanticConstructType,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var variables []outbound.SemanticCodeChunk

	// Find variable specifications
	varSpecs := findChildrenByType(varDecl, "var_spec")
	if len(varSpecs) == 0 {
		// Try const_spec for constants
		varSpecs = findChildrenByType(varDecl, "const_spec")
	}

	for _, varSpec := range varSpecs {
		vars := parseGoVariableSpec(parseTree, varDecl, varSpec, packageName, constructType, options, now)
		variables = append(variables, vars...)
	}

	return variables
}

// parseGoVariableSpec parses a variable specification.
func parseGoVariableSpec(
	parseTree *valueobject.ParseTree,
	varDecl *valueobject.ParseNode,
	varSpec *valueobject.ParseNode,
	packageName string,
	constructType outbound.SemanticConstructType,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var variables []outbound.SemanticCodeChunk

	// Get variable names
	identifiers := findChildrenByType(varSpec, "identifier")

	// Get variable type
	varType := getVariableType(parseTree, varSpec)

	// Get content (use full declaration for grouped vars)
	content := parseTree.GetNodeText(varDecl)
	if len(findChildrenByType(varDecl, "var_spec")) == 1 || len(findChildrenByType(varDecl, "const_spec")) == 1 {
		// Single variable, use spec content
		content = parseTree.GetNodeText(varSpec)
	}

	for _, identifier := range identifiers {
		varName := parseTree.GetNodeText(identifier)
		visibility := getVisibility(varName)

		// Skip private variables if not included
		if !options.IncludePrivate && visibility == outbound.Private {
			continue
		}

		variables = append(variables, outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID(string(constructType), varName, nil),
			Type:          constructType,
			Name:          varName,
			QualifiedName: packageName + "." + varName,
			Language:      parseTree.Language(),
			StartByte:     varSpec.StartByte,
			EndByte:       varSpec.EndByte,
			Content:       content,
			ReturnType:    varType,
			Visibility:    visibility,
			IsStatic:      true,
			ExtractedAt:   now,
			Hash:          utils.GenerateHash(content),
		})
	}

	return variables
}

// parseGoTypeDeclarations parses type alias declarations.
func parseGoTypeDeclarations(
	parseTree *valueobject.ParseTree,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var types []outbound.SemanticCodeChunk

	// Find type declarations that are aliases (not struct/interface)
	typeNodes := parseTree.GetNodesByType("type_declaration")
	for _, node := range typeNodes {
		typeSpecs := findChildrenByType(node, "type_spec")
		for _, typeSpec := range typeSpecs {
			// Skip struct and interface types (they're handled elsewhere)
			if findChildByTypeInNode(typeSpec, "struct_type") != nil ||
				findChildByTypeInNode(typeSpec, "interface_type") != nil {
				continue
			}

			typeChunk := parseGoTypeSpec(parseTree, node, typeSpec, packageName, options, now)
			if typeChunk != nil {
				types = append(types, *typeChunk)
			}
		}
	}

	return types
}

// parseGoTypeSpec parses a type specification (type alias).
func parseGoTypeSpec(
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find type name
	nameNode := findChildByTypeInNode(typeSpec, "type_identifier")
	if nameNode == nil {
		return nil
	}

	typeName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(typeDecl)
	visibility := getVisibility(typeName)

	// Skip private types if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Get the aliased type
	aliasedType := getAliasedType(parseTree, typeSpec)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("type", typeName, nil),
		Type:          outbound.ConstructType,
		Name:          typeName,
		QualifiedName: packageName + "." + typeName,
		Language:      parseTree.Language(),
		StartByte:     typeDecl.StartByte,
		EndByte:       typeDecl.EndByte,
		Content:       content,
		ReturnType:    aliasedType,
		Visibility:    visibility,
		IsStatic:      true,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// getVariableType gets the type of a variable.
func getVariableType(
	parseTree *valueobject.ParseTree,
	varSpec *valueobject.ParseNode,
) string {
	if varSpec == nil {
		return ""
	}

	// Look for various type nodes
	typeNodes := []string{
		"type_identifier",
		"pointer_type",
		"array_type",
		"slice_type",
		"map_type",
		"channel_type",
		"function_type",
		"interface_type",
		"struct_type",
	}

	for _, typeNodeName := range typeNodes {
		if typeNode := findChildByTypeInNode(varSpec, typeNodeName); typeNode != nil {
			return parseTree.GetNodeText(typeNode)
		}
	}

	return ""
}

// getAliasedType gets the type that a type alias points to.
func getAliasedType(
	parseTree *valueobject.ParseTree,
	typeSpec *valueobject.ParseNode,
) string {
	// Skip the type identifier and look for the actual type
	for _, child := range typeSpec.Children {
		if child.Type != "type_identifier" {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
}
