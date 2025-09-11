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

// ExtractClasses extracts struct definitions from a Go parse tree (Go's equivalent to classes).
func (p *GoParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return extractTypeDeclarations(ctx, parseTree, options, "struct_type", "Extracting Go structs", parseGoStruct)
}

// ExtractInterfaces extracts interface definitions from a Go parse tree.
func (p *GoParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return extractTypeDeclarations(
		ctx,
		parseTree,
		options,
		"interface_type",
		"Extracting Go interfaces",
		parseGoInterface,
	)
}

// extractTypeDeclarations is a generic method for extracting type declarations.
func extractTypeDeclarations(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	typeName string,
	progressMessage string,
	parserFunc func(context.Context, *valueobject.ParseTree, *valueobject.ParseNode, *valueobject.ParseNode, string, outbound.SemanticExtractionOptions, time.Time) *outbound.SemanticCodeChunk,
) ([]outbound.SemanticCodeChunk, error) {
	packageName := extractPackageNameFromTree(parseTree)
	var chunks []outbound.SemanticCodeChunk
	now := time.Now()

	// Find all type declarations
	typeDecls := findChildrenByType(parseTree.RootNode(), "type_declaration")
	for _, typeDecl := range typeDecls {
		// Find type specifications
		typeSpecs := findChildrenByTypeInNode(typeDecl, "type_spec")
		for _, typeSpec := range typeSpecs {
			// Check if this is the type we're looking for
			targetType := findChildByTypeInNode(typeSpec, typeName)
			if targetType == nil {
				continue
			}

			// Parse the construct
			chunk := parserFunc(ctx, parseTree, typeDecl, typeSpec, packageName, options, now)
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}
	}

	return chunks, nil
}

// parseGoStruct parses a Go struct declaration.
func parseGoStruct(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find struct name
	nameNode := findChildByTypeInNode(typeSpec, "type_identifier")
	if nameNode == nil {
		return nil
	}

	structName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(typeDecl)
	visibility := getVisibility(structName)

	// Skip private structs if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Parse generic parameters
	var genericParams []outbound.GenericParameter
	isGeneric := false
	typeParams := findChildByTypeInNode(typeSpec, "type_parameter_list")
	if typeParams != nil {
		isGeneric = true
		genericParams = parseGoGenericParameters(parseTree, typeParams)
	}

	// Parse struct fields as child chunks
	var childChunks []outbound.SemanticCodeChunk
	structType := findChildByTypeInNode(typeSpec, "struct_type")
	if structType != nil {
		childChunks = parseGoStructFields(parseTree, structType, packageName, structName, options, now)
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:           utils.GenerateID("struct", structName, nil),
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
func parseGoInterface(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find interface name
	nameNode := findChildByTypeInNode(typeSpec, "type_identifier")
	if nameNode == nil {
		return nil
	}

	interfaceName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(typeDecl)
	visibility := getVisibility(interfaceName)

	// Skip private interfaces if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Parse generic parameters
	var genericParams []outbound.GenericParameter
	isGeneric := false
	typeParams := findChildByTypeInNode(typeSpec, "type_parameter_list")
	if typeParams != nil {
		isGeneric = true
		genericParams = parseGoGenericParameters(parseTree, typeParams)
	}

	// Parse interface methods as child chunks
	var childChunks []outbound.SemanticCodeChunk
	interfaceType := findChildByTypeInNode(typeSpec, "interface_type")
	if interfaceType != nil {
		childChunks = parseGoInterfaceMethods(parseTree, interfaceType, packageName, interfaceName, options, now)
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:           utils.GenerateID("interface", interfaceName, nil),
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
func parseGoStructFields(
	parseTree *valueobject.ParseTree,
	structType *valueobject.ParseNode,
	packageName string,
	structName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var fields []outbound.SemanticCodeChunk

	fieldDecls := findChildrenByTypeInNode(structType, "field_declaration")
	for _, fieldDecl := range fieldDecls {
		fieldChunks := parseGoFieldDeclaration(parseTree, fieldDecl, packageName, structName, options, now)
		fields = append(fields, fieldChunks...)
	}

	return fields
}

// parseGoInterfaceMethods parses interface methods.
func parseGoInterfaceMethods(
	parseTree *valueobject.ParseTree,
	interfaceType *valueobject.ParseNode,
	packageName string,
	interfaceName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var methods []outbound.SemanticCodeChunk

	// Find method specifications
	methodSpecs := findChildrenByTypeInNode(interfaceType, "method_spec")
	for _, methodSpec := range methodSpecs {
		method := parseGoInterfaceMethod(parseTree, methodSpec, packageName, interfaceName, options, now)
		if method != nil {
			methods = append(methods, *method)
		}
	}

	// Find embedded interfaces
	embeddedTypes := findChildrenByTypeInNode(interfaceType, "type_identifier")
	for _, embeddedType := range embeddedTypes {
		embedded := parseGoEmbeddedInterface(parseTree, embeddedType, packageName, interfaceName, now)
		if embedded != nil {
			methods = append(methods, *embedded)
		}
	}

	return methods
}

// parseGoFieldDeclaration parses a struct field declaration.
func parseGoFieldDeclaration(
	parseTree *valueobject.ParseTree,
	fieldDecl *valueobject.ParseNode,
	packageName string,
	structName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var fields []outbound.SemanticCodeChunk

	// Get field names
	identifiers := findChildrenByTypeInNode(fieldDecl, "field_identifier")
	if len(identifiers) == 0 {
		// Handle embedded fields
		identifiers = findChildrenByTypeInNode(fieldDecl, "type_identifier")
	}

	// Get field type
	fieldType := getFieldType(parseTree, fieldDecl)

	// Get field tag
	tag := getFieldTag(parseTree, fieldDecl)
	content := parseTree.GetNodeText(fieldDecl)

	for _, identifier := range identifiers {
		fieldName := parseTree.GetNodeText(identifier)
		visibility := getVisibility(fieldName)

		// Skip private fields if not included
		if !options.IncludePrivate && visibility == outbound.Private {
			continue
		}

		// Parse annotations from tag
		annotations := parseFieldTags(tag)

		fields = append(fields, outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID("field", fieldName, nil),
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

// parseGoInterfaceMethod parses an interface method specification.
func parseGoInterfaceMethod(
	parseTree *valueobject.ParseTree,
	methodSpec *valueobject.ParseNode,
	packageName string,
	interfaceName string,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Find method name
	nameNode := findChildByTypeInNode(methodSpec, "field_identifier")
	if nameNode == nil {
		return nil
	}

	methodName := parseTree.GetNodeText(nameNode)
	content := parseTree.GetNodeText(methodSpec)
	visibility := getVisibility(methodName)

	// Parse parameters and return type from method signature
	parameters := parseGoParameters(parseTree, methodSpec)
	returnType := parseGoReturnType(parseTree, methodSpec)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("method", methodName, nil),
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
func parseGoEmbeddedInterface(
	parseTree *valueobject.ParseTree,
	embeddedType *valueobject.ParseNode,
	packageName string,
	interfaceName string,
	now time.Time,
) *outbound.SemanticCodeChunk {
	embeddedName := parseTree.GetNodeText(embeddedType)
	content := parseTree.GetNodeText(embeddedType)

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("interface", embeddedName, nil),
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
func getFieldType(
	parseTree *valueobject.ParseTree,
	fieldDecl *valueobject.ParseNode,
) string {
	if fieldDecl == nil {
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
		if typeNode := findChildByTypeInNode(fieldDecl, typeNodeName); typeNode != nil {
			return parseTree.GetNodeText(typeNode)
		}
	}

	return ""
}

// getFieldTag gets the tag of a struct field.
func getFieldTag(
	parseTree *valueobject.ParseTree,
	fieldDecl *valueobject.ParseNode,
) string {
	tagNode := findChildByTypeInNode(fieldDecl, "raw_string_literal")
	if tagNode != nil {
		return parseTree.GetNodeText(tagNode)
	}
	return ""
}

// parseFieldTags parses struct field tags into annotations.
func parseFieldTags(tag string) []outbound.Annotation {
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

// getVisibility determines if a name is public or private in Go.
func getVisibility(name string) outbound.VisibilityModifier {
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		return outbound.Public
	}
	return outbound.Private
}

// findChildrenByTypeInNode finds all direct children of a node with the specified type.
func findChildrenByTypeInNode(parent *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var result []*valueobject.ParseNode

	if parent == nil {
		return result
	}

	for _, child := range parent.Children {
		if child.Type == nodeType {
			result = append(result, child)
		}
	}

	return result
}

// extractPackageNameFromTree extracts the package name from the parse tree.
func extractPackageNameFromTree(parseTree *valueobject.ParseTree) string {
	// Find the package declaration
	packageDecls := findChildrenByType(parseTree.RootNode(), "package_clause")
	if len(packageDecls) == 0 {
		return ""
	}

	// Get the package identifier
	packageIdentifiers := findChildrenByTypeInNode(packageDecls[0], "package_identifier")
	if len(packageIdentifiers) == 0 {
		return ""
	}

	return parseTree.GetNodeText(packageIdentifiers[0])
}

// ============================================================================
// ObservableGoParser delegation methods for struct/interface extraction
// ============================================================================

// ExtractClasses implements the LanguageParser interface for ObservableGoParser.
// Delegates to the proper tree-sitter implementation in the inner GoParser.
func (o *ObservableGoParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "ObservableGoParser.ExtractClasses called", slogger.Fields{
		"language": parseTree.Language().String(),
	})

	if err := o.parser.validateInput(parseTree); err != nil {
		return nil, err
	}

	// Delegate to the inner parser's proper tree-sitter implementation
	return o.parser.ExtractClasses(ctx, parseTree, options)
}

// ExtractInterfaces implements the LanguageParser interface for ObservableGoParser.
// Delegates to the proper tree-sitter implementation in the inner GoParser.
func (o *ObservableGoParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "ObservableGoParser.ExtractInterfaces called", slogger.Fields{
		"language": parseTree.Language().String(),
	})

	if err := o.parser.validateInput(parseTree); err != nil {
		return nil, err
	}

	// Delegate to the inner parser's proper tree-sitter implementation
	return o.parser.ExtractInterfaces(ctx, parseTree, options)
}
