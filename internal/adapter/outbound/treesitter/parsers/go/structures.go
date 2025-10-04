package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ExtractClasses extracts struct definitions from a Go parse tree (Go's equivalent to classes).
// It processes all struct type declarations and returns them as SemanticCodeChunk objects
// with proper visibility filtering, generic parameter extraction, and field parsing.
func (p *GoParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	if parseTree == nil {
		return nil, errors.New("parse tree cannot be nil")
	}

	// Check for syntax errors in the parse tree
	if containsErrorNodes(parseTree) {
		return nil, errors.New("invalid syntax: syntax error detected by tree-sitter parser")
	}

	results, err := extractTypeDeclarations(
		ctx,
		parseTree,
		options,
		"struct_type",
		"Extracting Go structs",
		parseGoStruct,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to extract struct declarations: %w", err)
	}

	return results, nil
}

// ExtractInterfaces extracts interface definitions from a Go parse tree.
// It processes all interface type declarations and returns them as SemanticCodeChunk objects
// with proper method extraction, embedded interface handling, and generic parameter support.
func (p *GoParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	if parseTree == nil {
		return nil, errors.New("parse tree cannot be nil")
	}

	// Check for syntax errors in the parse tree
	if containsErrorNodes(parseTree) {
		return nil, errors.New("invalid syntax: syntax error detected by tree-sitter parser")
	}

	result, err := extractTypeDeclarations(
		ctx,
		parseTree,
		options,
		"interface_type",
		"Extracting Go interfaces",
		parseGoInterface,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to extract interface declarations: %w", err)
	}

	return result, nil
}

// extractTypeDeclarations is a generic method for extracting type declarations.
// It handles the common pattern of finding type declarations, filtering by type,
// and delegating parsing to type-specific parser functions.
func extractTypeDeclarations(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	typeName string,
	progressMessage string,
	parserFunc func(context.Context, *valueobject.ParseTree, *valueobject.ParseNode, *valueobject.ParseNode, string, outbound.SemanticExtractionOptions, time.Time) *outbound.SemanticCodeChunk,
) ([]outbound.SemanticCodeChunk, error) {
	if parseTree == nil {
		return nil, errors.New("parse tree cannot be nil")
	}
	if typeName == "" {
		return nil, errors.New("type name cannot be empty")
	}
	if parserFunc == nil {
		return nil, errors.New("parser function cannot be nil")
	}

	packageName := extractPackageNameFromTree(parseTree)
	var chunks []outbound.SemanticCodeChunk
	now := time.Now()

	// Find all type declarations using TreeSitterQueryEngine
	queryEngine := NewTreeSitterQueryEngine()
	typeDecls := queryEngine.QueryTypeDeclarations(parseTree)

	for _, typeDecl := range typeDecls {
		if typeDecl == nil {
			continue
		}

		// Find type specifications
		typeSpecs := FindDirectChildren(typeDecl, "type_spec")
		for _, typeSpec := range typeSpecs {
			if typeSpec == nil {
				continue
			}

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

// parseGoStruct parses a Go struct declaration and returns a SemanticCodeChunk.
// It extracts the struct name, content, visibility, documentation, generic parameters,
// and field information while applying visibility filtering based on the provided options.
func parseGoStruct(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if parseTree == nil || typeDecl == nil || typeSpec == nil {
		slogger.Error(ctx, "Invalid input parameters for struct parsing", slogger.Fields{
			"parse_tree_nil": parseTree == nil,
			"type_decl_nil":  typeDecl == nil,
			"type_spec_nil":  typeSpec == nil,
		})
		return nil
	}

	// Find struct name
	nameNode := findChildByTypeInNode(typeSpec, "type_identifier")
	if nameNode == nil {
		slogger.Debug(ctx, "No type_identifier found in type_spec", slogger.Fields{
			"type_spec_type": typeSpec.Type,
		})
		return nil
	}

	structName := parseTree.GetNodeText(nameNode)
	if structName == "" {
		slogger.Debug(ctx, "Empty struct name extracted", slogger.Fields{
			"node_type": nameNode.Type,
		})
		return nil
	}

	content := parseTree.GetNodeText(typeDecl)
	visibility := getVisibility(structName)

	// Extract documentation from preceding comments
	documentation := extractDocumentationForTypeDeclaration(parseTree, typeDecl)

	// Skip private structs if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		slogger.Debug(ctx, "Skipping private struct", slogger.Fields{
			"struct_name": structName,
			"visibility":  string(visibility),
		})
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

	// Generate qualified name properly - handle empty package names
	qualifiedName := structName
	if packageName != "" {
		qualifiedName = packageName + "." + structName
	}

	// Generate ChunkID in expected format: "struct:StructName"
	chunkID := fmt.Sprintf("struct:%s", structName)

	// Extract and validate AST positions from tree-sitter node
	startByte, endByte, positionValid := ExtractPositionInfo(typeDecl)
	if !positionValid {
		slogger.Error(ctx, "Invalid position information for struct node", slogger.Fields{
			"struct_name": structName,
			"node_type":   typeDecl.Type,
			"start_byte":  typeDecl.StartByte,
			"end_byte":    typeDecl.EndByte,
		})
		return nil
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:           chunkID,
		Type:              outbound.ConstructStruct,
		Name:              structName,
		QualifiedName:     qualifiedName,
		Language:          valueobject.Go,
		StartByte:         startByte,
		EndByte:           endByte,
		Content:           content,
		Documentation:     documentation,
		Visibility:        visibility,
		IsGeneric:         isGeneric,
		GenericParameters: genericParams,
		ChildChunks:       childChunks,
		ExtractedAt:       now,
		Hash:              utils.GenerateHash(content),
	}
}

// parseGoInterface parses a Go interface declaration and returns a SemanticCodeChunk.
// It extracts the interface name, content, visibility, generic parameters, and method information
// while applying visibility filtering based on the provided options.
func parseGoInterface(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if parseTree == nil || typeDecl == nil || typeSpec == nil {
		slogger.Error(ctx, "Invalid input parameters for interface parsing", slogger.Fields{
			"parse_tree_nil": parseTree == nil,
			"type_decl_nil":  typeDecl == nil,
			"type_spec_nil":  typeSpec == nil,
		})
		return nil
	}

	// Find interface name
	nameNode := findChildByTypeInNode(typeSpec, "type_identifier")
	if nameNode == nil {
		slogger.Debug(ctx, "No type_identifier found in interface type_spec", slogger.Fields{
			"type_spec_type": typeSpec.Type,
		})
		return nil
	}

	interfaceName := parseTree.GetNodeText(nameNode)
	if interfaceName == "" {
		slogger.Debug(ctx, "Empty interface name extracted", slogger.Fields{
			"node_type": nameNode.Type,
		})
		return nil
	}

	content := parseTree.GetNodeText(typeDecl)
	visibility := getVisibility(interfaceName)

	// Extract documentation from preceding comments
	documentation := extractDocumentationForTypeDeclaration(parseTree, typeDecl)

	// Skip private interfaces if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		slogger.Debug(ctx, "Skipping private interface", slogger.Fields{
			"interface_name": interfaceName,
			"visibility":     string(visibility),
		})
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

	// Extract and validate AST positions from tree-sitter node
	startByte, endByte, positionValid := ExtractPositionInfo(typeDecl)
	if !positionValid {
		slogger.Error(ctx, "Invalid position information for interface node", slogger.Fields{
			"interface_name": interfaceName,
			"node_type":      typeDecl.Type,
			"start_byte":     typeDecl.StartByte,
			"end_byte":       typeDecl.EndByte,
		})
		return nil
	}

	// Generate qualified name properly - handle empty package names
	qualifiedName := interfaceName
	if packageName != "" {
		qualifiedName = packageName + "." + interfaceName
	}

	// Generate ChunkID in expected format: "interface:InterfaceName"
	chunkID := fmt.Sprintf("interface:%s", interfaceName)

	return &outbound.SemanticCodeChunk{
		ChunkID:           chunkID,
		Type:              outbound.ConstructInterface,
		Name:              interfaceName,
		QualifiedName:     qualifiedName,
		Language:          valueobject.Go,
		StartByte:         startByte,
		EndByte:           endByte,
		Content:           content,
		Documentation:     documentation,
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

	fieldDecls := FindDirectChildren(structType, "field_declaration")
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
	methodSpecs := FindDirectChildren(interfaceType, "method_spec")
	for _, methodSpec := range methodSpecs {
		method := parseGoInterfaceMethod(parseTree, methodSpec, packageName, interfaceName, options, now)
		if method != nil {
			methods = append(methods, *method)
		}
	}

	// Find embedded interfaces
	embeddedTypes := FindDirectChildren(interfaceType, "type_identifier")
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
	identifiers := FindDirectChildren(fieldDecl, "field_identifier")
	if len(identifiers) == 0 {
		// Handle embedded fields
		identifiers = FindDirectChildren(fieldDecl, "type_identifier")
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

// extractPackageNameFromTree extracts the package name from the parse tree.
func extractPackageNameFromTree(parseTree *valueobject.ParseTree) string {
	// Find the package declaration
	queryEngine := NewTreeSitterQueryEngine()
	packageDecls := queryEngine.QueryPackageDeclarations(parseTree)
	if len(packageDecls) == 0 {
		return ""
	}

	// Get the package identifier
	packageIdentifiers := FindDirectChildren(packageDecls[0], "package_identifier")
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

// ============================================================================
// Documentation Extraction Functions
// ============================================================================

// extractDocumentationForTypeDeclaration extracts documentation for a type declaration
// using TreeSitterQueryEngine for consistent comment processing.
func extractDocumentationForTypeDeclaration(parseTree *valueobject.ParseTree, typeDecl *valueobject.ParseNode) string {
	// Use TreeSitterQueryEngine for consistent comment extraction
	queryEngine := NewTreeSitterQueryEngine()
	documentationComments := queryEngine.QueryDocumentationComments(parseTree, typeDecl)

	// Process each comment using TreeSitterQueryEngine
	var comments []string
	for _, commentNode := range documentationComments {
		processedText := queryEngine.ProcessCommentText(parseTree, commentNode)
		if processedText != "" {
			comments = append(comments, processedText)
		}
	}

	return strings.Join(comments, " ")
}
