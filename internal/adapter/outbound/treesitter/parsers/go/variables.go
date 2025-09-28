// Package goparser provides Go-specific variable extraction functionality for tree-sitter parsing.
//
// This file implements comprehensive variable, constant, and type alias extraction from Go source code
// using tree-sitter's Abstract Syntax Tree (AST) parsing capabilities. It handles all Go declaration
// patterns including:
//
// 1. Simple variable declarations: var x int
// 2. Initialized variables: var x int = 42
// 3. Multiple variable declarations: var x, y string
// 4. Grouped variable declarations: var ( x int; y string )
// 5. Constants: const MaxValue = 100
// 6. Grouped constants: const ( Alpha = "a"; Beta = "b" )
// 7. Type aliases: type UserID int
// 8. Complex type aliases: type Handler func(string) error
//
// The implementation uses a hierarchical approach to parse different declaration structures:
// - parseGoVariableDeclaration: Main entry point for variable/constant declarations
// - parseGoVariableSpec: Handles individual variable specifications
// - parseGoTypeSpec: Handles type alias specifications
//
// Key features:
// - Robust error handling and validation
// - Centralized type detection logic
// - Enhanced logging for debugging and monitoring
// - Consistent metadata extraction
// - Tree-sitter query engine integration for reliable AST traversal
package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Node type constants specific to variable extraction
// These constants define the AST node types used in variable, constant, and type alias extraction.
// Note: Type-related constants (nodeTypePointerType, nodeTypeArrayType, etc.) are imported from functions.go.
const (
	// Variable and constant declaration node types.
	nodeTypeVarSpec       = "var_spec"
	nodeTypeConstSpec     = "const_spec"
	nodeTypeTypeSpec      = "type_spec"
	nodeTypeVarSpecList   = "var_spec_list"
	nodeTypeConstSpecList = "const_spec_list"

	// Token types for parentheses in grouped declarations.
	tokenTypeOpenParen  = "("
	tokenTypeCloseParen = ")"

	// Grammar field names.
	fieldNameName  = "name"
	fieldNameType  = "type"
	fieldNameValue = "value"

	// Chunk ID prefixes.
	chunkPrefixVar   = "var"
	chunkPrefixConst = "const"
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
// This function provides comprehensive error handling and validation for variable extraction.
func (p *GoParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Go variables", slogger.Fields{})

	// Validate input parameters with detailed error context
	if err := p.validateInput(parseTree); err != nil {
		slogger.Error(ctx, "Variable extraction validation failed", slogger.Fields{
			"error":      err.Error(),
			"parse_tree": parseTree != nil,
			"root_node":  parseTree != nil && parseTree.RootNode() != nil,
		})
		return nil, fmt.Errorf("variable extraction failed validation: %w", err)
	}

	var variables []outbound.SemanticCodeChunk
	packageName := p.extractPackageNameFromTree(parseTree)
	now := time.Now()

	// Use TreeSitterQueryEngine for consistent AST querying
	queryEngine := NewTreeSitterQueryEngine()

	// Get both variable and constant declarations
	varNodes := queryEngine.QueryVariableDeclarations(parseTree)
	constNodes := queryEngine.QueryConstDeclarations(parseTree)

	slogger.Debug(ctx, "Found declaration nodes", slogger.Fields{
		"variable_node_count": len(varNodes),
		"constant_node_count": len(constNodes),
	})

	// Combine and sort declarations by source position to maintain source order
	type declarationInfo struct {
		node          *valueobject.ParseNode
		constructType outbound.SemanticConstructType
	}

	var allDeclarations []declarationInfo

	// Add variable declarations
	for _, node := range varNodes {
		if node != nil {
			allDeclarations = append(allDeclarations, declarationInfo{
				node:          node,
				constructType: outbound.ConstructVariable,
			})
		}
	}

	// Add constant declarations
	for _, node := range constNodes {
		if node != nil {
			allDeclarations = append(allDeclarations, declarationInfo{
				node:          node,
				constructType: outbound.ConstructConstant,
			})
		}
	}

	// Sort by start position to maintain source order using Go's standard library
	sort.Slice(allDeclarations, func(i, j int) bool {
		return allDeclarations[i].node.StartByte < allDeclarations[j].node.StartByte
	})

	// Process declarations in source order
	for i, decl := range allDeclarations {
		if decl.node == nil {
			slogger.Warn(ctx, "Encountered nil declaration node, skipping", slogger.Fields{
				"declaration_index": i,
			})
			continue
		}

		declVars := parseGoVariableDeclaration(parseTree, decl.node, packageName, decl.constructType, options, now)
		variables = append(variables, declVars...)
	}

	// Process type declarations with error handling
	typeNodes := queryEngine.QueryTypeDeclarations(parseTree)
	slogger.Debug(ctx, "Found type declaration nodes", slogger.Fields{
		"type_node_count": len(typeNodes),
	})

	typeDecls := parseGoTypeDeclarationsFromNodes(parseTree, typeNodes, packageName, options, now)
	variables = append(variables, typeDecls...)

	slogger.Debug(ctx, "Variable extraction completed", slogger.Fields{
		"total_variables_extracted": len(variables),
	})

	return variables, nil
}

// validateVariableDeclarationInputs validates the inputs for variable declaration parsing.
// This function provides comprehensive validation with detailed error messages for debugging.
func validateVariableDeclarationInputs(
	parseTree *valueobject.ParseTree,
	varDecl *valueobject.ParseNode,
	packageName string,
) error {
	if parseTree == nil {
		return errors.New("parseTree cannot be nil: required for node text extraction")
	}
	if varDecl == nil {
		return errors.New("varDecl cannot be nil: required for variable declaration parsing")
	}
	if packageName == "" {
		return errors.New("packageName cannot be empty: required for qualified name generation")
	}

	// Additional validation for parse tree integrity
	if parseTree.RootNode() == nil {
		return errors.New("parseTree root node is nil: parse tree appears corrupted")
	}

	return nil
}

// findVariableSpecs finds variable or constant specifications within a declaration node.
// This function handles both var_spec and const_spec node types, including grouped declarations.
func findVariableSpecs(varDecl *valueobject.ParseNode) []*valueobject.ParseNode {
	if varDecl == nil {
		return nil
	}

	var allSpecs []*valueobject.ParseNode

	// Check for grouped declarations (spec_list pattern)
	allSpecs = append(allSpecs, findGroupedSpecs(varDecl, nodeTypeVarSpecList, nodeTypeVarSpec)...)
	allSpecs = append(allSpecs, findGroupedSpecs(varDecl, nodeTypeConstSpecList, nodeTypeConstSpec)...)

	// Check for direct specifications (single declarations)
	allSpecs = append(allSpecs, FindDirectChildren(varDecl, nodeTypeVarSpec)...)
	allSpecs = append(allSpecs, FindDirectChildren(varDecl, nodeTypeConstSpec)...)

	return allSpecs
}

// findGroupedSpecs finds specs within grouped declarations (var_spec_list or const_spec_list).
// This helper function reduces duplication in findVariableSpecs.
func findGroupedSpecs(varDecl *valueobject.ParseNode, listType, specType string) []*valueobject.ParseNode {
	var specs []*valueobject.ParseNode

	specLists := FindDirectChildren(varDecl, listType)
	for _, specList := range specLists {
		specs = append(specs, FindDirectChildren(specList, specType)...)
	}

	return specs
}

// parseGoVariableDeclaration parses variable/constant declarations with proper validation.
// This function delegates the actual parsing to helper functions for better organization.
func parseGoVariableDeclaration(
	parseTree *valueobject.ParseTree,
	varDecl *valueobject.ParseNode,
	packageName string,
	constructType outbound.SemanticConstructType,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	// Validate inputs with comprehensive error handling
	if err := validateVariableDeclarationInputs(parseTree, varDecl, packageName); err != nil {
		// Log detailed error information for debugging
		slogger.Error(context.Background(), "Variable declaration parsing failed validation", slogger.Fields{
			"error": err.Error(),
			"var_decl_type": func() string {
				if varDecl != nil {
					return varDecl.Type
				}
				return "nil"
			}(),
			"construct_type": string(constructType),
		})
		return nil
	}

	var variables []outbound.SemanticCodeChunk

	// Find variable specifications
	varSpecs := findVariableSpecs(varDecl)
	if len(varSpecs) == 0 {
		return variables
	}

	for _, varSpec := range varSpecs {
		if varSpec == nil {
			continue
		}

		vars := parseGoVariableSpec(parseTree, varDecl, varSpec, packageName, constructType, options, now)
		variables = append(variables, vars...)
	}

	return variables
}

// determineContentForVariable determines the appropriate content text for a variable.
// For single declarations, it uses the full declaration; for grouped declarations, it uses just the spec.
func determineContentForVariable(
	parseTree *valueobject.ParseTree,
	varDecl *valueobject.ParseNode,
	varSpec *valueobject.ParseNode,
) string {
	if parseTree == nil || varDecl == nil || varSpec == nil {
		return ""
	}

	// Check if this is a grouped declaration by looking for var_spec_list or multiple specs
	isGrouped := isGroupedDeclaration(varDecl)

	if isGrouped {
		// Multiple variables in a group, use just the spec content
		// For grouped declarations, we need to include any trailing comments
		content := parseTree.GetNodeText(varSpec)

		// Check if there's a comment right after this var_spec
		content = includeTrailingComment(parseTree, varSpec, content)

		return content
	} else {
		// Single variable declaration, use the full declaration including keyword
		return parseTree.GetNodeText(varDecl)
	}
}

// includeTrailingComment includes any comment that immediately follows a variable specification.
// This is a minimal implementation to handle comments in grouped declarations.
func includeTrailingComment(
	parseTree *valueobject.ParseTree,
	varSpec *valueobject.ParseNode,
	originalContent string,
) string {
	if parseTree == nil || varSpec == nil {
		return originalContent
	}

	// Get the full source text to check for trailing comments
	sourceBytes := parseTree.Source()
	if sourceBytes == nil {
		return originalContent
	}

	source := string(sourceBytes)
	endPos := int(varSpec.EndByte)

	// Look for a comment starting right after the var_spec
	if endPos < len(source) {
		// Skip whitespace after the var_spec
		i := endPos
		for i < len(source) && (source[i] == ' ' || source[i] == '\t') {
			i++
		}

		// Check if there's a comment starting with "//"
		if i+1 < len(source) && source[i:i+2] == "//" {
			// Find the end of the comment (end of line)
			commentEnd := i
			for commentEnd < len(source) && source[commentEnd] != '\n' && source[commentEnd] != '\r' {
				commentEnd++
			}

			// Include the comment in the content
			return originalContent + source[endPos:commentEnd]
		}
	}

	return originalContent
}

// isGroupedDeclaration determines if a variable/constant declaration is grouped (has parentheses).
// This function checks for var_spec_list, const_spec_list nodes or parentheses patterns.
func isGroupedDeclaration(varDecl *valueobject.ParseNode) bool {
	if varDecl == nil {
		return false
	}

	// Check for var_spec_list which indicates grouped variables
	varSpecLists := FindDirectChildren(varDecl, nodeTypeVarSpecList)
	if len(varSpecLists) > 0 {
		return true
	}

	// Check for const_spec_list which indicates grouped constants
	constSpecLists := FindDirectChildren(varDecl, nodeTypeConstSpecList)
	if len(constSpecLists) > 0 {
		return true
	}

	// Check for parentheses which indicate grouped declarations
	// Look for "(" and ")" tokens in the children
	hasOpenParen := false
	hasCloseParen := false
	for _, child := range varDecl.Children {
		if child.Type == tokenTypeOpenParen {
			hasOpenParen = true
		}
		if child.Type == tokenTypeCloseParen {
			hasCloseParen = true
		}
	}
	if hasOpenParen && hasCloseParen {
		return true
	}

	// Check if there are multiple var_spec or const_spec nodes as a fallback
	varSpecs := FindDirectChildren(varDecl, nodeTypeVarSpec)
	constSpecs := FindDirectChildren(varDecl, nodeTypeConstSpec)
	totalSpecs := len(varSpecs) + len(constSpecs)

	return totalSpecs > 1
}

// createSemanticCodeChunk creates a SemanticCodeChunk for a variable with comprehensive validation.
// This function handles the complete creation process including position calculation, visibility determination,
// and content generation. It returns nil if any validation fails, ensuring robust error handling.
func createSemanticCodeChunk(
	identifier *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
	varDecl *valueobject.ParseNode,
	varSpec *valueobject.ParseNode,
	packageName string,
	constructType outbound.SemanticConstructType,
	varType string,
	content string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	if identifier == nil || parseTree == nil || varDecl == nil || varSpec == nil {
		return nil
	}

	varName := parseTree.GetNodeText(identifier)
	if varName == "" {
		return nil
	}

	visibility := getVisibility(varName)

	// Skip private variables if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Determine positions based on grouped vs non-grouped declarations
	var startByte, endByte uint32

	// Use the same logic as content determination for consistency
	isGrouped := isGroupedDeclaration(varDecl)

	if isGrouped {
		// Multiple variables in a group, use spec positions
		startByte, endByte = varSpec.StartByte, varSpec.EndByte
	} else {
		// Single variable declaration, use declaration positions
		startByte, endByte = varDecl.StartByte, varDecl.EndByte
	}

	// Ensure EndByte is exclusive (tree-sitter standard)
	// Don't subtract 1 to keep it consistent with other parsers

	// Generate ChunkID in expected format: var:name or const:name
	var chunkIDPrefix string
	switch constructType {
	case outbound.ConstructVariable:
		chunkIDPrefix = chunkPrefixVar
	case outbound.ConstructConstant:
		chunkIDPrefix = chunkPrefixConst
	case outbound.ConstructFunction, outbound.ConstructMethod, outbound.ConstructClass,
		outbound.ConstructStruct, outbound.ConstructInterface, outbound.ConstructEnum,
		outbound.ConstructField, outbound.ConstructProperty, outbound.ConstructModule,
		outbound.ConstructPackage, outbound.ConstructNamespace, outbound.ConstructType,
		outbound.ConstructComment, outbound.ConstructDecorator, outbound.ConstructAttribute,
		outbound.ConstructLambda, outbound.ConstructClosure, outbound.ConstructGenerator,
		outbound.ConstructAsyncFunction:
		chunkIDPrefix = string(constructType)
	}
	chunkID := fmt.Sprintf("%s:%s", chunkIDPrefix, varName)

	return &outbound.SemanticCodeChunk{
		ChunkID:       chunkID,
		Type:          constructType,
		Name:          varName,
		QualifiedName: varName, // Use just the name, not package.name
		Language:      valueobject.Go,
		StartByte:     startByte,
		EndByte:       endByte,
		Content:       content,
		ReturnType:    varType,
		Visibility:    visibility,
		IsStatic:      true,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// parseGoVariableSpec parses a variable specification with comprehensive validation and error handling.
// This function processes individual variable specifications (var_spec or const_spec) and extracts
// all identifiers within the specification. It handles both simple and complex variable declarations
// while maintaining proper error handling and logging throughout the process.
//
// The function supports:
// - Multiple variable names in a single specification (e.g., var a, b, c int)
// - Type inference from the specification
// - Grammar-based field access with fallback to direct children search
// - Proper content determination for grouped vs single declarations.
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

	// Validate inputs
	if parseTree == nil || varDecl == nil || varSpec == nil || packageName == "" {
		return variables
	}

	// Get variable names using grammar field access
	identifiers := GetMultipleFieldsByName(varSpec, fieldNameName)
	if len(identifiers) == 0 {
		// Fallback to direct children search for compatibility
		identifiers = FindDirectChildren(varSpec, nodeTypeIdentifier)
		if len(identifiers) == 0 {
			return variables
		}
	}

	// Get variable type using grammar field access
	varType := getVariableTypeFromField(parseTree, varSpec)

	// Determine appropriate content
	content := determineContentForVariable(parseTree, varDecl, varSpec)

	for _, identifier := range identifiers {
		if identifier == nil {
			continue
		}

		chunk := createSemanticCodeChunk(
			identifier,
			parseTree,
			varDecl,
			varSpec,
			packageName,
			constructType,
			varType,
			content,
			options,
			now,
		)

		if chunk != nil {
			variables = append(variables, *chunk)
		}
	}

	return variables
}

// shouldSkipTypeSpec determines if a type specification should be skipped.
// Struct and interface types are handled by dedicated extractors, so they are excluded here.
func shouldSkipTypeSpec(typeSpec *valueobject.ParseNode) bool {
	if typeSpec == nil {
		return true
	}

	// Skip struct and interface types (they're handled elsewhere)
	// Use the same logic as structures.go which is proven to work
	structType := findChildByTypeInNode(typeSpec, nodeTypeStructType)
	interfaceType := findChildByTypeInNode(typeSpec, nodeTypeInterfaceType)

	return structType != nil || interfaceType != nil
}

// parseGoTypeDeclarationsFromNodes parses type alias declarations from provided nodes.
// This function focuses on type aliases only, excluding struct and interface definitions
// which are handled by their respective specialized extractors.
//
// The function processes each type declaration node and extracts type aliases such as:
// - type UserID int
// - type Handler func(string) error
// - type StringMap map[string]interface{}.
//

func parseGoTypeDeclarationsFromNodes(
	parseTree *valueobject.ParseTree,
	typeNodes []*valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	var types []outbound.SemanticCodeChunk

	// Validate inputs
	if parseTree == nil || len(typeNodes) == 0 || packageName == "" {
		return types
	}

	// Process each type declaration node
	for _, node := range typeNodes {
		if node == nil {
			continue
		}

		typeSpecs := FindDirectChildren(node, nodeTypeTypeSpec)
		for _, typeSpec := range typeSpecs {
			if shouldSkipTypeSpec(typeSpec) {
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

// extractTypeNameFromSpec extracts the type name from a type specification node.
// Returns the first type_identifier found, which represents the alias name.
//

func extractTypeNameFromSpec(
	parseTree *valueobject.ParseTree,
	typeSpec *valueobject.ParseNode,
) string {
	if parseTree == nil || typeSpec == nil {
		return ""
	}

	nameNodes := FindChildrenRecursive(typeSpec, nodeTypeTypeIdentifier)
	if len(nameNodes) == 0 || nameNodes[0] == nil {
		return ""
	}

	return parseTree.GetNodeText(nameNodes[0])
}

// parseGoTypeSpec parses a type specification (type alias) with comprehensive validation.
// This function handles type alias declarations such as:
// - type UserID int (simple type alias)
// - type Handler func(string) error (function type alias)
// - type StringMap map[string]interface{} (complex type alias)
//
// The function extracts the alias name, determines the aliased type, and creates
// a SemanticCodeChunk with proper metadata and position information.
//

func parseGoTypeSpec(
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	// Validate inputs
	if parseTree == nil || typeDecl == nil || typeSpec == nil || packageName == "" {
		return nil
	}

	// Extract type name
	typeName := extractTypeNameFromSpec(parseTree, typeSpec)
	if typeName == "" {
		return nil
	}

	visibility := getVisibility(typeName)

	// Skip private types if not included
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	// Get content and aliased type
	content := parseTree.GetNodeText(typeDecl)
	aliasedType := getAliasedType(parseTree, typeSpec)

	// Use position validation for better error handling
	startByte, endByte, valid := ExtractPositionInfo(typeDecl)
	if !valid {
		// Fallback to node positions if validation fails
		startByte, endByte = typeDecl.StartByte, typeDecl.EndByte
	}
	if endByte > 0 {
		endByte-- // Make EndByte inclusive
	}

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("type", typeName, nil),
		Type:          outbound.ConstructType,
		Name:          typeName,
		QualifiedName: packageName + "." + typeName,
		Language:      valueobject.Go,
		StartByte:     startByte,
		EndByte:       endByte,
		Content:       content,
		ReturnType:    aliasedType,
		Visibility:    visibility,
		IsStatic:      true,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(content),
	}
}

// getSupportedTypeNodes returns the list of supported type node names for variable type detection.
// This centralized list ensures consistency across type detection functions.
func getSupportedTypeNodes() []string {
	return []string{
		nodeTypeTypeIdentifier,
		nodeTypePointerType,
		nodeTypeArrayType,
		nodeTypeSliceType,
		nodeTypeMapType,
		nodeTypeChannelType,
		nodeTypeFunctionType,
		nodeTypeInterfaceType,
		nodeTypeStructType,
	}
}

// findFirstTypeNode searches for the first occurrence of any supported type node in the given parent node.
// This utility function eliminates duplication between getVariableType and getAliasedType.
func findFirstTypeNode(
	parseTree *valueobject.ParseTree,
	parent *valueobject.ParseNode,
	typeNodes []string,
) string {
	if parent == nil || parseTree == nil {
		return ""
	}

	for _, typeNodeName := range typeNodes {
		typeMatchNodes := FindChildrenRecursive(parent, typeNodeName)
		if len(typeMatchNodes) > 0 {
			return parseTree.GetNodeText(typeMatchNodes[0])
		}
	}

	return ""
}

// getVariableType gets the type of a variable using centralized type detection logic.
func getVariableType(
	parseTree *valueobject.ParseTree,
	varSpec *valueobject.ParseNode,
) string {
	if varSpec == nil {
		return ""
	}

	return findFirstTypeNode(parseTree, varSpec, getSupportedTypeNodes())
}

// getVariableTypeFromField gets the type of a variable using grammar field access.
// This is the preferred method that uses tree-sitter's field-based access.
func getVariableTypeFromField(
	parseTree *valueobject.ParseTree,
	varSpec *valueobject.ParseNode,
) string {
	if parseTree == nil || varSpec == nil {
		return ""
	}

	// First try to get type using grammar field access
	typeField := GetFieldByName(varSpec, fieldNameType)
	if typeField != nil {
		return parseTree.GetNodeText(typeField)
	}

	// Fallback to the original method for compatibility
	return getVariableType(parseTree, varSpec)
}

// getAliasedType gets the type that a type alias points to.
// This function finds the type that comes after the first type_identifier (the alias name)
// using the centralized type detection logic.
//

func getAliasedType(
	parseTree *valueobject.ParseTree,
	typeSpec *valueobject.ParseNode,
) string {
	if typeSpec == nil || parseTree == nil {
		return ""
	}

	typeNodes := getSupportedTypeNodes()

	// Find the type that comes after the first type_identifier (the alias name)
	foundAlias := false
	for _, child := range typeSpec.Children {
		if child.Type == nodeTypeTypeIdentifier {
			if !foundAlias {
				// This is the alias name, skip it
				foundAlias = true
				continue
			}
			// This is the aliased type
			return parseTree.GetNodeText(child)
		}

		// Check for other type nodes after we've found the alias name
		if foundAlias {
			for _, typeNode := range typeNodes {
				if child.Type == typeNode {
					return parseTree.GetNodeText(child)
				}
			}
		}
	}

	return ""
}
