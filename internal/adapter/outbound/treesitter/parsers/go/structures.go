package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// ExtractClasses extracts struct definitions from a Go parse tree (Go's equivalent to classes).
func (p *GoParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return p.extractTypeDeclarations(ctx, parseTree, options, "struct_type", "Extracting Go structs", p.parseGoStruct)
}

// ExtractInterfaces extracts interface definitions from a Go parse tree.
func (p *GoParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return p.extractTypeDeclarations(
		ctx,
		parseTree,
		options,
		"interface_type",
		"Extracting Go interfaces",
		p.parseGoInterface,
	)
}

// parseGoStruct parses a Go struct declaration.
func (p *GoParser) parseGoStruct(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find struct name
	nameNode := p.findChildByType(typeSpec, "type_identifier")
	if nameNode == nil {
		return nil
	}

	structName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(typeDecl)
	visibility := p.getVisibility(structName)

	// Skip private structs if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Parse generic parameters
	var genericParams []outbound.GenericParameter
	isGeneric := false
	typeParams := p.findChildByType(typeSpec, "type_parameter_list")
	if typeParams != nil {
		isGeneric = true
		genericParams = p.parseGoGenericParameters(parseTree, typeParams)
	}

	// Parse struct fields as child chunks
	var childChunks []outbound.SemanticCodeChunk
	structType := p.findChildByType(typeSpec, "struct_type")
	if structType != nil {
		childChunks = p.parseGoStructFields(parseTree, structType, packageName, structName, options, now)
	}

	return &outbound.SemanticCodeChunk{
		ID:                utils.GenerateID("struct", structName, nil),
		Type:              outbound.ConstructStruct,
		Name:              structName,
		QualifiedName:     packageName + "." + structName,
		Language:          parseTree.Language(),
		StartByte:         typeDecl.StartByte,
		EndByte:           typeDecl.EndByte,
		Content:           content,
		Visibility:        visibility,
		IsGeneric:         isGeneric,
		GenericParameters: genericParams,
		ChildChunks:       childChunks,
		ExtractedAt:       now,
		Hash:              utils.GenerateHash(content),
	}
}

// parseGoInterface parses a Go interface declaration.
func (p *GoParser) parseGoInterface(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find interface name
	nameNode := p.findChildByType(typeSpec, "type_identifier")
	if nameNode == nil {
		return nil
	}

	interfaceName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(typeDecl)
	visibility := p.getVisibility(interfaceName)

	// Skip private interfaces if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Parse generic parameters
	var genericParams []outbound.GenericParameter
	isGeneric := false
	typeParams := p.findChildByType(typeSpec, "type_parameter_list")
	if typeParams != nil {
		isGeneric = true
		genericParams = p.parseGoGenericParameters(parseTree, typeParams)
	}

	// Parse interface methods as child chunks
	var childChunks []outbound.SemanticCodeChunk
	interfaceType := p.findChildByType(typeSpec, "interface_type")
	if interfaceType != nil {
		childChunks = p.parseGoInterfaceMethods(parseTree, interfaceType, packageName, interfaceName, options, now)
	}

	return &outbound.SemanticCodeChunk{
		ID:                utils.GenerateID("interface", interfaceName, nil),
		Type:              outbound.ConstructInterface,
		Name:              interfaceName,
		QualifiedName:     packageName + "." + interfaceName,
		Language:          parseTree.Language(),
		StartByte:         typeDecl.StartByte,
		EndByte:           typeDecl.EndByte,
		Content:           content,
		Visibility:        visibility,
		IsAbstract:        true,
		IsGeneric:         isGeneric,
		GenericParameters: genericParams,
		ChildChunks:       childChunks,
		ExtractedAt:       now,
		Hash:              utils.GenerateHash(content),
	}
}

// parseGoStructFields parses struct fields.
func (p *GoParser) parseGoStructFields(
	parseTree *valueobject.ParseTree,
	structType *valueobject.ParseNode,
	packageName string,
	structName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var fields []outbound.SemanticCodeChunk

	fieldDecls := p.findChildrenByType(structType, "field_declaration")
	for _, fieldDecl := range fieldDecls {
		fieldChunks := p.parseGoFieldDeclaration(parseTree, fieldDecl, packageName, structName, options, now)
		fields = append(fields, fieldChunks...)
	}

	return fields
}

// parseGoFieldDeclaration parses a struct field declaration.
func (p *GoParser) parseGoFieldDeclaration(
	parseTree *valueobject.ParseTree,
	fieldDecl *valueobject.ParseNode,
	packageName string,
	structName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var fields []outbound.SemanticCodeChunk

	// Get field names
	identifiers := p.findChildrenByType(fieldDecl, "field_identifier")
	if len(identifiers) == 0 {
		// Handle embedded fields
		identifiers = p.findChildrenByType(fieldDecl, "type_identifier")
	}

	// Get field type
	fieldType := p.getFieldType(parseTree, fieldDecl)

	// Get field tag
	tag := p.getFieldTag(parseTree, fieldDecl)
	content := parseTree.GetNodeText(fieldDecl)

	for _, identifier := range identifiers {
		fieldName := parseTree.GetNodeText(identifier)
		visibility := p.getVisibility(fieldName)

		// Skip private fields if not included
		if !options.IncludePrivate && visibility == outbound.Private {
			continue
		}

		// Parse annotations from tag
		annotations := p.parseFieldTags(tag)

		fields = append(fields, outbound.SemanticCodeChunk{
			ID:            utils.GenerateID("field", fieldName, nil),
			Type:          outbound.ConstructField,
			Name:          fieldName,
			QualifiedName: packageName + "." + structName + "." + fieldName,
			Language:      parseTree.Language(),
			StartByte:     fieldDecl.StartByte,
			EndByte:       fieldDecl.EndByte,
			Content:       content,
			ReturnType:    fieldType,
			Visibility:    visibility,
			Annotations:   annotations,
			ExtractedAt:   now,
			Hash:          utils.GenerateHash(content),
		})
	}

	return fields
}

// parseGoInterfaceMethods parses interface methods.
func (p *GoParser) parseGoInterfaceMethods(
	parseTree *valueobject.ParseTree,
	interfaceType *valueobject.ParseNode,
	packageName string,
	interfaceName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var methods []outbound.SemanticCodeChunk

	// Find method specifications
	methodSpecs := p.findChildrenByType(interfaceType, "method_spec")
	for _, methodSpec := range methodSpecs {
		method := p.parseGoInterfaceMethod(parseTree, methodSpec, packageName, interfaceName, options, now)
		if method != nil {
			methods = append(methods, *method)
		}
	}

	// Find embedded interfaces
	embeddedTypes := p.findChildrenByType(interfaceType, "type_identifier")
	for _, embeddedType := range embeddedTypes {
		embedded := p.parseGoEmbeddedInterface(parseTree, embeddedType, packageName, interfaceName, now)
		if embedded != nil {
			methods = append(methods, *embedded)
		}
	}

	return methods
}

// parseGoInterfaceMethod parses an interface method specification.
func (p *GoParser) parseGoInterfaceMethod(
	parseTree *valueobject.ParseTree,
	methodSpec *valueobject.ParseNode,
	packageName string,
	interfaceName string,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find method name
	nameNode := p.findChildByType(methodSpec, "field_identifier")
	if nameNode == nil {
		return nil
	}

	methodName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(methodSpec)
	visibility := p.getVisibility(methodName)

	// Parse parameters and return type from method signature
	parameters := p.parseGoParameters(parseTree, methodSpec)
	returnType := p.parseGoReturnType(parseTree, methodSpec)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("method", methodName, nil),
		Type:          outbound.ConstructMethod,
		Name:          methodName,
		QualifiedName: packageName + "." + interfaceName + "." + methodName,
		Language:      parseTree.Language(),
		StartByte:     methodSpec.StartByte,
		EndByte:       methodSpec.EndByte,
		Content:       content,
		Parameters:    parameters,
		ReturnType:    returnType,
		Visibility:    visibility,
		IsAbstract:    true,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// parseGoEmbeddedInterface parses an embedded interface.
func (p *GoParser) parseGoEmbeddedInterface(
	parseTree *valueobject.ParseTree,
	embeddedType *valueobject.ParseNode,
	packageName string,
	interfaceName string,
	now time.Time,
) *outbound.SemanticCodeChunk {
	embeddedName := parseTree.GetNodeText(embeddedType)
	content := parseTree.GetNodeText(embeddedType)

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID("interface", embeddedName, nil),
		Type:          outbound.ConstructInterface,
		Name:          embeddedName,
		QualifiedName: packageName + "." + interfaceName + "." + embeddedName,
		Language:      parseTree.Language(),
		StartByte:     embeddedType.StartByte,
		EndByte:       embeddedType.EndByte,
		Content:       content,
		Visibility:    outbound.Public,
		IsAbstract:    true,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// getFieldType gets the type of a struct field.
func (p *GoParser) getFieldType(
	parseTree *valueobject.ParseTree,
	fieldDecl *valueobject.ParseNode,
) string {
	typeNode := p.getParameterType(fieldDecl)
	if typeNode != nil {
		return parseTree.GetNodeText(typeNode)
	}
	return ""
}

// getFieldTag gets the tag of a struct field.
func (p *GoParser) getFieldTag(
	parseTree *valueobject.ParseTree,
	fieldDecl *valueobject.ParseNode,
) string {
	tagNode := p.findChildByType(fieldDecl, "raw_string_literal")
	if tagNode != nil {
		return parseTree.GetNodeText(tagNode)
	}
	return ""
}

// parseFieldTags parses struct field tags into annotations.
func (p *GoParser) parseFieldTags(tag string) []outbound.Annotation {
	var annotations []outbound.Annotation

	if tag == "" {
		return annotations
	}

	// Remove backticks
	if len(tag) >= 2 && tag[0] == '`' && tag[len(tag)-1] == '`' {
		tag = tag[1 : len(tag)-1]
	}

	// Simple parsing - look for key:"value" patterns
	parts := strings.Fields(tag)
	for _, part := range parts {
		if colonIndex := strings.Index(part, ":"); colonIndex > 0 {
			key := part[:colonIndex]
			value := part[colonIndex+1:]
			// Remove quotes
			value = strings.Trim(value, "\"'")

			annotations = append(annotations, outbound.Annotation{
				Name:      key,
				Arguments: []string{value},
			})
		}
	}

	return annotations
}

// getParameterType gets the type node from a parameter-like declaration.
func (p *GoParser) getParameterType(decl *valueobject.ParseNode) *valueobject.ParseNode {
	if decl == nil {
		return nil
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

	for _, typeNode := range typeNodes {
		if node := p.findChildByType(decl, typeNode); node != nil {
			return node
		}
	}

	return nil
}
