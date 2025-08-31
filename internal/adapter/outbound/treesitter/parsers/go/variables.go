package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"time"
)

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
		vars := p.parseGoVariableDeclaration(parseTree, node, packageName, outbound.ConstructVariable, options, now)
		variables = append(variables, vars...)
	}

	// Find constant declarations
	constNodes := parseTree.GetNodesByType("const_declaration")
	for _, node := range constNodes {
		consts := p.parseGoVariableDeclaration(parseTree, node, packageName, outbound.ConstructConstant, options, now)
		variables = append(variables, consts...)
	}

	// Find type declarations
	typeDecls := p.parseGoTypeDeclarations(parseTree, packageName, options, now)
	variables = append(variables, typeDecls...)

	return variables, nil
}

// parseGoVariableDeclaration parses variable/constant declarations.
func (p *GoParser) parseGoVariableDeclaration(
	parseTree *valueobject.ParseTree,
	varDecl *valueobject.ParseNode,
	packageName string,
	constructType outbound.SemanticConstructType,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var variables []outbound.SemanticCodeChunk

	// Find variable specifications
	varSpecs := p.findChildrenByType(varDecl, "var_spec")
	if len(varSpecs) == 0 {
		// Try const_spec for constants
		varSpecs = p.findChildrenByType(varDecl, "const_spec")
	}

	for _, varSpec := range varSpecs {
		vars := p.parseGoVariableSpec(parseTree, varDecl, varSpec, packageName, constructType, options, now)
		variables = append(variables, vars...)
	}

	return variables
}

// parseGoVariableSpec parses a variable specification.
func (p *GoParser) parseGoVariableSpec(
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
	identifiers := p.findChildrenByType(varSpec, "identifier")

	// Get variable type
	varType := p.getVariableType(parseTree, varSpec)

	// Get content (use full declaration for grouped vars)
	content := parseTree.GetNodeText(varDecl)
	if len(p.findChildrenByType(varDecl, "var_spec")) == 1 || len(p.findChildrenByType(varDecl, "const_spec")) == 1 {
		// Single variable, use spec content
		content = parseTree.GetNodeText(varSpec)
	}

	for _, identifier := range identifiers {
		varName := parseTree.GetNodeText(identifier)
		visibility := p.getVisibility(varName)

		// Skip private variables if not included
		if !options.IncludePrivate && visibility == outbound.Private {
			continue
		}

		variables = append(variables, outbound.SemanticCodeChunk{
			ID:            utils.GenerateID(string(constructType), varName, nil),
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
func (p *GoParser) parseGoTypeDeclarations(
	parseTree *valueobject.ParseTree,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var types []outbound.SemanticCodeChunk

	// Find type declarations that are aliases (not struct/interface)
	typeNodes := parseTree.GetNodesByType("type_declaration")
	for _, node := range typeNodes {
		typeSpecs := p.findChildrenByType(node, "type_spec")
		for _, typeSpec := range typeSpecs {
			// Skip struct and interface types (they're handled elsewhere)
			if p.findChildByType(typeSpec, "struct_type") != nil ||
				p.findChildByType(typeSpec, "interface_type") != nil {
				continue
			}

			typeChunk := p.parseGoTypeSpec(parseTree, node, typeSpec, packageName, options, now)
			if typeChunk != nil {
				types = append(types, *typeChunk)
			}
		}
	}

	return types
}

// parseGoTypeSpec parses a type specification (type alias).
func (p *GoParser) parseGoTypeSpec(
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find type name
	nameNode := p.findChildByType(typeSpec, "type_identifier")
	if nameNode == nil {
		return nil
	}

	typeName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(typeDecl)
	visibility := p.getVisibility(typeName)

	// Skip private types if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Get the aliased type
	aliasedType := p.getAliasedType(parseTree, typeSpec)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("type", typeName, nil),
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
func (p *GoParser) getVariableType(
	parseTree *valueobject.ParseTree,
	varSpec *valueobject.ParseNode,
) string {
	typeNode := p.getParameterType(varSpec)
	if typeNode != nil {
		return parseTree.GetNodeText(typeNode)
	}
	return ""
}

// getAliasedType gets the type that a type alias points to.
func (p *GoParser) getAliasedType(
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
