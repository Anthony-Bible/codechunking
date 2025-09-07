package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type GoParser struct {
	supportedLanguage valueobject.Language
}

func NewGoParser() (treesitter.ObservableTreeSitterParser, error) {
	lang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		return nil, fmt.Errorf("failed to create language value object: %w", err)
	}

	parser := &GoParser{
		supportedLanguage: lang,
	}

	return &ObservableGoParser{
		parser: parser,
	}, nil
}

func (p *GoParser) Parse(ctx context.Context, sourceCode string) (*valueobject.ParseTree, error) {
	if sourceCode == "" {
		return nil, errors.New("source code cannot be empty")
	}

	rootNode := &valueobject.ParseNode{}
	metadata, err := valueobject.NewParseMetadata(0, "0.0.0", "0.0.0")
	if err != nil {
		return nil, err
	}

	tree, err := valueobject.NewParseTree(ctx, p.supportedLanguage, rootNode, []byte(sourceCode), metadata)
	if err != nil {
		return nil, err
	}

	if err := p.validateInput(tree); err != nil {
		return nil, err
	}

	p.ExtractModules(sourceCode, tree)
	p.extractImports(sourceCode, tree)
	p.extractDeclarations(sourceCode, tree)

	return tree, nil
}

func (p *GoParser) ExtractModules(sourceCode string, tree *valueobject.ParseTree) {
	lines := strings.Split(sourceCode, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "package ") {
			packageName := strings.TrimSpace(strings.TrimPrefix(line, "package "))
			position := valueobject.Position{Row: uint32(i + 1), Column: 0}
			p.addConstruct(tree, "package", packageName, position, position)
			break
		}
	}
}

func (p *GoParser) GetSupportedLanguage() string {
	return p.supportedLanguage.String()
}

func (p *GoParser) GetSupportedConstructTypes() []string {
	return []string{
		"package",
		"import",
		"function",
		"struct",
		"interface",
		"method",
		"variable",
		"constant",
	}
}

func (p *GoParser) IsSupported(constructType string) bool {
	supported := p.GetSupportedConstructTypes()
	for _, t := range supported {
		if t == constructType {
			return true
		}
	}
	return false
}

func (p *GoParser) ExtractMethodsFromStruct(structName string, structBody string, tree *valueobject.ParseTree) {
	lines := strings.Split(structBody, "\n")
	for i, line := range lines {
		if strings.Contains(line, "func (") && strings.Contains(line, ")") {
			methodStart := strings.Index(line, structName)
			if methodStart != -1 {
				methodPart := line[methodStart+len(structName)+1:]
				endParen := strings.Index(methodPart, ")")
				if endParen != -1 {
					methodName := strings.TrimSpace(methodPart[:endParen])
					position := valueobject.Position{Row: uint32(i + 1), Column: 0}
					p.addConstruct(tree, "method", structName+"."+methodName, position, position)
				}
			}
		}
	}
}

func (p *GoParser) validateInput(tree *valueobject.ParseTree) error {
	if tree == nil {
		return errors.New("parse tree cannot be nil")
	}
	return nil
}

func (p *GoParser) findChildByType(tree *valueobject.ParseTree, constructType string) []*valueobject.ParseNode {
	return tree.GetNodesByType(constructType)
}

func findChildByTypeInNode(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	if node.Type == nodeType {
		return node
	}

	for _, child := range node.Children {
		if found := findChildByTypeInNode(child, nodeType); found != nil {
			return found
		}
	}

	return nil
}

func findChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	var results []*valueobject.ParseNode

	if node.Type == nodeType {
		results = append(results, node)
	}

	for _, child := range node.Children {
		childResults := findChildrenByType(child, nodeType)
		results = append(results, childResults...)
	}

	return results
}

func (p *GoParser) getVisibility(name string) outbound.VisibilityModifier {
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		return outbound.Public
	}
	return outbound.Private
}

func parseGoGenericParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []outbound.GenericParameter {
	var params []outbound.GenericParameter

	// Look for generic parameters in the node's children
	typeParams := findChildByTypeInNode(node, "type_parameters")
	if typeParams == nil {
		return params
	}

	// Find all type identifiers within the type parameters
	paramNodes := findChildrenByType(typeParams, "type_identifier")
	for _, paramNode := range paramNodes {
		params = append(params, outbound.GenericParameter{
			Name: parseTree.GetNodeText(paramNode),
		})
	}

	return params
}

func parseGoReceiver(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Look for receiver in the node's children
	receiverNode := findChildByTypeInNode(node, "receiver")
	if receiverNode == nil {
		return ""
	}

	// Find the identifier within the receiver
	identifierNode := findChildByTypeInNode(receiverNode, "identifier")
	if identifierNode != nil {
		return parseTree.GetNodeText(identifierNode)
	}

	// If not found, look for field_identifier
	fieldIdentifierNode := findChildByTypeInNode(receiverNode, "field_identifier")
	if fieldIdentifierNode != nil {
		return parseTree.GetNodeText(fieldIdentifierNode)
	}

	return ""
}

func (p *GoParser) extractPackageNameFromTree(tree *valueobject.ParseTree) string {
	// This is a stub to make it compile
	return ""
}

func (p *GoParser) extractImports(sourceCode string, tree *valueobject.ParseTree) {
	lines := strings.Split(sourceCode, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "import ") {
			importPath := strings.TrimSpace(strings.TrimPrefix(line, "import "))
			importPath = strings.Trim(importPath, "\"")
			position := valueobject.Position{Row: uint32(i + 1), Column: 0}
			p.addConstruct(tree, "import", importPath, position, position)
		}
	}
}

func (p *GoParser) extractDeclarations(sourceCode string, tree *valueobject.ParseTree) {
	lines := strings.Split(sourceCode, "\n")
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "func ") {
			p.extractFuncDecl(trimmedLine, i+1, tree)
		} else if strings.HasPrefix(trimmedLine, "type ") {
			p.extractTypeSpec(trimmedLine, i+1, tree)
		} else if strings.HasPrefix(trimmedLine, "var ") {
			p.extractValueSpec("variable", trimmedLine, i+1, tree)
		} else if strings.HasPrefix(trimmedLine, "const ") {
			p.extractValueSpec("constant", trimmedLine, i+1, tree)
		}
	}
}

func (p *GoParser) extractFuncDecl(line string, lineNumber int, tree *valueobject.ParseTree) {
	funcName := strings.TrimSpace(strings.TrimPrefix(line, "func "))
	if strings.Contains(funcName, "(") {
		funcName = funcName[:strings.Index(funcName, "(")]
	}

	position := valueobject.Position{Row: uint32(lineNumber), Column: 0}
	p.addConstruct(tree, "function", funcName, position, position)
}

func (p *GoParser) extractTypeSpec(line string, lineNumber int, tree *valueobject.ParseTree) {
	typeName := strings.TrimSpace(strings.TrimPrefix(line, "type "))
	if strings.Contains(typeName, "struct") {
		structName := strings.TrimSpace(typeName[:strings.Index(typeName, "struct")])
		position := valueobject.Position{Row: uint32(lineNumber), Column: 0}
		p.addConstruct(tree, "struct", structName, position, position)
	} else if strings.Contains(typeName, "interface") {
		interfaceName := strings.TrimSpace(typeName[:strings.Index(typeName, "interface")])
		position := valueobject.Position{Row: uint32(lineNumber), Column: 0}
		p.addConstruct(tree, "interface", interfaceName, position, position)
	}
}

func (p *GoParser) extractValueSpec(constructType string, line string, lineNumber int, tree *valueobject.ParseTree) {
	varName := strings.TrimSpace(strings.TrimPrefix(line, constructType+" "))
	if strings.Contains(varName, "=") {
		varName = strings.TrimSpace(varName[:strings.Index(varName, "=")])
	}

	position := valueobject.Position{Row: uint32(lineNumber), Column: 0}
	p.addConstruct(tree, constructType, varName, position, position)
}

func (p *GoParser) addConstruct(
	tree *valueobject.ParseTree,
	constructType, name string,
	start, end valueobject.Position,
) {
	// Stub method to replace removed AddConstruct
}

type ObservableGoParser struct {
	parser *GoParser
}

func (o *ObservableGoParser) Parse(ctx context.Context, source []byte) (*treesitter.ParseResult, error) {
	startTime := time.Now()
	sourceCode := string(source)

	parseTree, err := o.parser.Parse(ctx, sourceCode)
	if err != nil {
		return nil, err
	}

	convertedTree, err := treesitter.ConvertDomainParseTreeToPort(parseTree)
	if err != nil {
		return nil, fmt.Errorf("failed to convert parse tree: %w", err)
	}

	result := &treesitter.ParseResult{
		Success:   true,
		ParseTree: convertedTree,
		Duration:  time.Since(startTime),
	}

	return result, nil
}

func (o *ObservableGoParser) ParseSource(
	ctx context.Context,
	language valueobject.Language,
	source []byte,
	options treesitter.ParseOptions,
) (*treesitter.ParseResult, error) {
	// Minimal implementation to satisfy interface
	return o.Parse(ctx, source)
}

func (o *ObservableGoParser) GetLanguage() string {
	return "go"
}

func (o *ObservableGoParser) Close() error {
	return nil
}

// ============================================================================
// LanguageParser interface implementation (delegated to inner parser)
// ============================================================================

// ExtractFunctions implements the LanguageParser interface.
func (o *ObservableGoParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// TODO: Implement actual function extraction
	return []outbound.SemanticCodeChunk{}, nil
}

// ExtractClasses implements the LanguageParser interface.
func (o *ObservableGoParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// TODO: Implement actual struct extraction
	return []outbound.SemanticCodeChunk{}, nil
}

// ExtractInterfaces implements the LanguageParser interface.
func (o *ObservableGoParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// TODO: Implement actual interface extraction
	return []outbound.SemanticCodeChunk{}, nil
}

// ExtractVariables implements the LanguageParser interface.
func (o *ObservableGoParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// TODO: Implement actual variable extraction
	return []outbound.SemanticCodeChunk{}, nil
}

// ExtractImports implements the LanguageParser interface.
func (o *ObservableGoParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	// TODO: Implement actual import extraction
	return []outbound.ImportDeclaration{}, nil
}

// ExtractModules implements the LanguageParser interface.
func (o *ObservableGoParser) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// TODO: Implement actual module extraction
	return []outbound.SemanticCodeChunk{}, nil
}

// GetSupportedLanguage implements the LanguageParser interface.
func (o *ObservableGoParser) GetSupportedLanguage() valueobject.Language {
	return o.parser.supportedLanguage
}

// GetSupportedConstructTypes implements the LanguageParser interface.
func (o *ObservableGoParser) GetSupportedConstructTypes() []outbound.SemanticConstructType {
	return []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructMethod,
		outbound.ConstructStruct,
		outbound.ConstructInterface,
		outbound.ConstructVariable,
		outbound.ConstructConstant,
		outbound.ConstructPackage,
	}
}

// IsSupported implements the LanguageParser interface.
func (o *ObservableGoParser) IsSupported(language valueobject.Language) bool {
	return language.Name() == valueobject.LanguageGo
}

// ============================================================================
// Testing Helper Methods - Access to inner parser for testing
// ============================================================================

// parseGoFunction exposes the inner parser's parseGoFunction method for testing
func (o *ObservableGoParser) parseGoFunction(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	extractedAt time.Time,
	depth int,
) *outbound.SemanticCodeChunk {
	chunk, err := o.parser.parseGoFunction(node, packageName, parseTree)
	if err != nil {
		return nil
	}
	return &chunk
}

// parseGoMethod exposes the inner parser's parseGoMethod method for testing
func (o *ObservableGoParser) parseGoMethod(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	extractedAt time.Time,
	depth int,
) *outbound.SemanticCodeChunk {
	chunk, err := o.parser.parseGoMethod(node, packageName, parseTree)
	if err != nil {
		return nil
	}
	return &chunk
}

// parseGoParameters exposes the parseGoParameters function for testing
func (o *ObservableGoParser) parseGoParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []outbound.Parameter {
	return parseGoParameters(parseTree, node)
}

// parseGoReturnType exposes the parseGoReturnType function for testing
func (o *ObservableGoParser) parseGoReturnType(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	return parseGoReturnType(parseTree, node)
}

// parseGoMethodParameters is a placeholder for testing (may need implementation)
func (o *ObservableGoParser) parseGoMethodParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	receiverType string,
) []outbound.Parameter {
	return parseGoParameters(parseTree, node)
}
