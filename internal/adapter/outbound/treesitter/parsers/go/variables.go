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
	"time"
)

// Node type constants specific to variable extraction
// These constants define the AST node types used in variable, constant, and type alias extraction.
// Note: Type-related constants (nodeTypePointerType, nodeTypeArrayType, etc.) are imported from functions.go.
const (
	// Variable and constant declaration node types.
	nodeTypeVarSpec   = "var_spec"
	nodeTypeConstSpec = "const_spec"
	nodeTypeTypeSpec  = "type_spec"
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

	// Validate input parameters
	if err := p.validateInput(parseTree); err != nil {
		slogger.Error(ctx, "Variable extraction validation failed", slogger.Fields{
			"error": err.Error(),
		})
		return nil, err
	}

	var variables []outbound.SemanticCodeChunk
	packageName := p.extractPackageNameFromTree(parseTree)
	now := time.Now()

	// Use TreeSitterQueryEngine for consistent AST querying
	queryEngine := NewTreeSitterQueryEngine()

	// Process variable declarations with error handling
	varNodes := queryEngine.QueryVariableDeclarations(parseTree)
	slogger.Debug(ctx, "Found variable declaration nodes", slogger.Fields{
		"variable_node_count": len(varNodes),
	})

	// Debug: check what types of nodes we have in the parse tree
	allNodes := parseTree.GetNodesByType("var_declaration")
	slogger.Debug(ctx, "Direct var_declaration query result", slogger.Fields{
		"direct_query_count": len(allNodes),
	})

	// Check for alternative node types that might represent grouped variables
	groupedVarNodes := parseTree.GetNodesByType("var_spec")
	slogger.Debug(ctx, "var_spec nodes found", slogger.Fields{
		"var_spec_count": len(groupedVarNodes),
	})

	for i, node := range varNodes {
		if node == nil {
			slogger.Warn(ctx, "Encountered nil variable node, skipping", slogger.Fields{
				"node_index": i,
			})
			continue
		}

		vars := parseGoVariableDeclaration(parseTree, node, packageName, outbound.ConstructVariable, options, now)
		variables = append(variables, vars...)
	}

	// Process constant declarations with error handling
	constNodes := queryEngine.QueryConstDeclarations(parseTree)
	slogger.Debug(ctx, "Found constant declaration nodes", slogger.Fields{
		"constant_node_count": len(constNodes),
	})

	for i, node := range constNodes {
		if node == nil {
			slogger.Warn(ctx, "Encountered nil constant node, skipping", slogger.Fields{
				"node_index": i,
			})
			continue
		}

		consts := parseGoVariableDeclaration(parseTree, node, packageName, outbound.ConstructConstant, options, now)
		variables = append(variables, consts...)
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
func validateVariableDeclarationInputs(
	parseTree *valueobject.ParseTree,
	varDecl *valueobject.ParseNode,
	packageName string,
) error {
	if parseTree == nil {
		return errors.New("parseTree cannot be nil")
	}
	if varDecl == nil {
		return errors.New("varDecl cannot be nil")
	}
	if packageName == "" {
		return errors.New("packageName cannot be empty")
	}
	return nil
}

// findVariableSpecs finds variable or constant specifications within a declaration node.
// This function handles both var_spec and const_spec node types.
func findVariableSpecs(varDecl *valueobject.ParseNode) []*valueobject.ParseNode {
	if varDecl == nil {
		return nil
	}

	// Find variable specifications
	varSpecs := FindDirectChildren(varDecl, nodeTypeVarSpec)
	if len(varSpecs) == 0 {
		// Try const_spec for constants
		varSpecs = FindDirectChildren(varDecl, nodeTypeConstSpec)
	}

	return varSpecs
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
	// Validate inputs
	if err := validateVariableDeclarationInputs(parseTree, varDecl, packageName); err != nil {
		// Log error but don't fail the entire extraction
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

	// Check if this is a grouped declaration (parentheses)
	varSpecs := FindDirectChildren(varDecl, nodeTypeVarSpec)
	constSpecs := FindDirectChildren(varDecl, nodeTypeConstSpec)
	totalSpecs := len(varSpecs) + len(constSpecs)

	if totalSpecs > 1 {
		// Multiple variables in a group, use just the spec content
		return parseTree.GetNodeText(varSpec)
	} else {
		// Single variable declaration, use the full declaration including keyword
		return parseTree.GetNodeText(varDecl)
	}
}

// createSemanticCodeChunk creates a SemanticCodeChunk for a variable with proper validation.
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

	// Check if this is a grouped declaration (parentheses)
	varSpecs := FindDirectChildren(varDecl, nodeTypeVarSpec)
	constSpecs := FindDirectChildren(varDecl, nodeTypeConstSpec)
	totalSpecs := len(varSpecs) + len(constSpecs)

	if totalSpecs > 1 {
		// Multiple variables in a group, use spec positions
		startByte, endByte = varSpec.StartByte, varSpec.EndByte
		if endByte > 0 {
			endByte-- // Make EndByte inclusive
		}
	} else {
		// Single variable declaration, use declaration positions
		startByte, endByte = varDecl.StartByte, varDecl.EndByte
		if endByte > 0 {
			endByte-- // Make EndByte inclusive
		}
	}

	// Generate ChunkID in expected format: var:name or const:name
	var chunkIDPrefix string
	switch constructType {
	case outbound.ConstructVariable:
		chunkIDPrefix = "var"
	case outbound.ConstructConstant:
		chunkIDPrefix = "const"
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

// parseGoVariableSpec parses a variable specification with improved validation and structure.
// This function has been broken down into smaller, focused helper functions.
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

	// Get variable names
	identifiers := FindDirectChildren(varSpec, nodeTypeIdentifier)
	if len(identifiers) == 0 {
		return variables
	}

	// Get variable type
	varType := getVariableType(parseTree, varSpec)

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
	structTypes := FindChildrenRecursive(typeSpec, nodeTypeStructType)
	interfaceTypes := FindChildrenRecursive(typeSpec, nodeTypeInterfaceType)
	return len(structTypes) > 0 || len(interfaceTypes) > 0
}

// parseGoTypeDeclarationsFromNodes parses type alias declarations from provided nodes.
// This function focuses on type aliases only, excluding struct and interface definitions
// which are handled by their respective specialized extractors.
//
// The function processes each type declaration node and extracts type aliases such as:
// - type UserID int
// - type Handler func(string) error
// - type StringMap map[string]interface{}.
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

// getAliasedType gets the type that a type alias points to.
// This function finds the type that comes after the first type_identifier (the alias name)
// using the centralized type detection logic.
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
