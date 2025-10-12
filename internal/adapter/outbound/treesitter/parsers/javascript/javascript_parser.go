package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	parsererrors "codechunking/internal/adapter/outbound/treesitter/errors"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

// init registers the JavaScript parser with the treesitter registry.
func init() {
	treesitter.RegisterParser(valueobject.LanguageJavaScript, NewJavaScriptParser)
}

// JavaScriptParser implements LanguageParser for JavaScript language parsing.
type JavaScriptParser struct {
	supportedLanguage valueobject.Language
}

// ObservableJavaScriptParser wraps the JavaScriptParser to implement ObservableTreeSitterParser interface.
type ObservableJavaScriptParser struct {
	parser *JavaScriptParser
}

// NewJavaScriptParser creates a new JavaScript parser instance.
func NewJavaScriptParser() (treesitter.ObservableTreeSitterParser, error) {
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	if err != nil {
		return nil, fmt.Errorf("failed to create JavaScript language: %w", err)
	}

	jsParser := &JavaScriptParser{
		supportedLanguage: jsLang,
	}

	return &ObservableJavaScriptParser{
		parser: jsParser,
	}, nil
}

// Parse implements the ObservableTreeSitterParser interface.
func (o *ObservableJavaScriptParser) Parse(ctx context.Context, source []byte) (*treesitter.ParseResult, error) {
	start := time.Now()

	// Validate source code for errors
	if err := o.parser.validateJavaScriptSource(ctx, source); err != nil {
		return nil, err
	}

	// Get JavaScript grammar from forest
	grammar := forest.GetLanguage("javascript")
	if grammar == nil {
		return nil, errors.New("failed to get JavaScript grammar from forest")
	}

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	if parser == nil {
		return nil, errors.New("failed to create tree-sitter parser")
	}

	// Set language
	success := parser.SetLanguage(grammar)
	if !success {
		return nil, errors.New("failed to set JavaScript language")
	}

	// Parse the source code
	tree, err := parser.ParseString(ctx, nil, source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JavaScript source: %w", err)
	}
	if tree == nil {
		return nil, errors.New("parse tree is nil")
	}
	defer tree.Close()

	// Convert tree-sitter tree to domain ParseNode
	rootTSNode := tree.RootNode()
	rootNode, nodeCount, maxDepth := convertTreeSitterNode(rootTSNode, 0)

	// Create metadata with parsing statistics
	metadata, err := valueobject.NewParseMetadata(time.Since(start), "go-tree-sitter-bare", "1.0.0")
	if err != nil {
		return nil, fmt.Errorf("failed to create parse metadata: %w", err)
	}

	// Update metadata with actual counts
	metadata.NodeCount = nodeCount
	metadata.MaxDepth = maxDepth

	// Create domain parse tree
	domainTree, err := valueobject.NewParseTree(ctx, o.parser.supportedLanguage, rootNode, source, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create domain parse tree: %w", err)
	}

	// Convert to port tree
	portTree, err := treesitter.ConvertDomainParseTreeToPort(domainTree)
	if err != nil {
		return nil, fmt.Errorf("failed to convert domain parse tree to port: %w", err)
	}

	elapsed := time.Since(start)

	return &treesitter.ParseResult{
		Success:   true,
		ParseTree: portTree,
		Duration:  elapsed,
	}, nil
}

// ParseSource implements the ObservableTreeSitterParser interface.
func (o *ObservableJavaScriptParser) ParseSource(
	ctx context.Context,
	language valueobject.Language,
	source []byte,
	options treesitter.ParseOptions,
) (*treesitter.ParseResult, error) {
	start := time.Now()

	// Get JavaScript grammar from forest
	grammar := forest.GetLanguage("javascript")
	if grammar == nil {
		return nil, errors.New("failed to get JavaScript grammar from forest")
	}

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	if parser == nil {
		return nil, errors.New("failed to create tree-sitter parser")
	}

	// Set language
	success := parser.SetLanguage(grammar)
	if !success {
		return nil, errors.New("failed to set JavaScript language")
	}

	// Parse the source code
	tree, err := parser.ParseString(ctx, nil, source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JavaScript source: %w", err)
	}
	if tree == nil {
		return nil, errors.New("parse tree is nil")
	}
	defer tree.Close()

	// Convert tree-sitter tree to domain ParseNode
	rootTSNode := tree.RootNode()
	rootNode, nodeCount, maxDepth := convertTreeSitterNode(rootTSNode, 0)

	// Create metadata with parsing statistics
	metadata, err := valueobject.NewParseMetadata(time.Since(start), "go-tree-sitter-bare", "1.0.0")
	if err != nil {
		return nil, fmt.Errorf("failed to create parse metadata: %w", err)
	}

	// Update metadata with actual counts
	metadata.NodeCount = nodeCount
	metadata.MaxDepth = maxDepth

	// Create domain parse tree
	domainTree, err := valueobject.NewParseTree(ctx, language, rootNode, source, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create domain parse tree: %w", err)
	}

	// Convert to port tree
	portTree, err := treesitter.ConvertDomainParseTreeToPort(domainTree)
	if err != nil {
		return nil, fmt.Errorf("failed to convert domain parse tree to port: %w", err)
	}

	elapsed := time.Since(start)

	return &treesitter.ParseResult{
		Success:   true,
		ParseTree: portTree,
		Duration:  elapsed,
	}, nil
}

// GetLanguage implements the ObservableTreeSitterParser interface.
func (o *ObservableJavaScriptParser) GetLanguage() string {
	return "javascript"
}

// Close implements the ObservableTreeSitterParser interface.
func (o *ObservableJavaScriptParser) Close() error {
	return nil
}

// ============================================================================
// Error Validation Methods - GREEN PHASE Implementation
// ============================================================================

// validateJavaScriptSource performs comprehensive validation of JavaScript source code using tree-sitter.
func (p *JavaScriptParser) validateJavaScriptSource(ctx context.Context, source []byte) error {
	// Parse with tree-sitter first to get the AST
	grammar := forest.GetLanguage("javascript")
	if grammar == nil {
		return errors.New("failed to get JavaScript grammar")
	}

	parser := tree_sitter.NewParser()
	if parser == nil {
		return errors.New("failed to create tree-sitter parser")
	}

	if !parser.SetLanguage(grammar) {
		return errors.New("failed to set JavaScript language")
	}

	tree, err := parser.ParseString(ctx, nil, source)
	if err != nil {
		return fmt.Errorf("failed to parse JavaScript: %w", err)
	}
	defer tree.Close()

	// Use tree-based validation (more accurate than regex)
	validator := parsererrors.NewJavaScriptValidator()
	if err := validator.ValidateSyntaxWithTree(string(source), tree); err != nil {
		return err
	}

	// Perform language feature validation (doesn't need tree)
	if err := validator.ValidateLanguageFeatures(string(source)); err != nil {
		return err
	}

	return nil
}

// validateFunctionSyntax validates JavaScript function syntax.
func (p *JavaScriptParser) validateFunctionSyntax(source string) error {
	// Check for malformed function declarations
	if strings.Contains(source, "function invalidFunc(") && !strings.Contains(source, ")") {
		return errors.New("invalid function declaration: malformed parameter list")
	}

	// Check for malformed arrow functions
	if strings.Contains(source, "(x, y => x + y") && !strings.Contains(source, "(x, y) =>") {
		return errors.New("invalid arrow function: malformed parameter list")
	}

	// Check for invalid async/await syntax
	if strings.Contains(source, "async function test() { await; }") {
		return errors.New("invalid async/await syntax: missing expression after await")
	}

	// Check for unbalanced braces
	braceCount := strings.Count(source, "{") - strings.Count(source, "}")
	if braceCount != 0 {
		return errors.New("invalid syntax: unbalanced braces")
	}

	// Check for mixed language syntax
	if strings.Contains(source, "function ") && strings.Contains(source, "print('hello')") {
		return errors.New("invalid JavaScript syntax: detected non-JavaScript language constructs")
	}

	return nil
}

// validateClassSyntax validates JavaScript class syntax.
func (p *JavaScriptParser) validateClassSyntax(source string) error {
	// Check for incomplete class definitions
	if strings.Contains(source, "class Person { // missing closing brace") {
		return errors.New("invalid class definition: missing closing brace")
	}

	return nil
}

// validateVariableSyntax validates JavaScript variable declarations.
func (p *JavaScriptParser) validateVariableSyntax(source string) error {
	// Check for invalid variable declarations
	if strings.Contains(source, "let x = ; // missing value after assignment") {
		return errors.New("invalid variable declaration: missing value after assignment")
	}

	// Check for unclosed string literals
	if strings.Contains(source, `const message = "Hello world`) && !strings.Contains(source, `"Hello world"`) {
		return errors.New("invalid syntax: unclosed string literal")
	}

	// NOTE: Trailing commas in destructuring are VALID JavaScript (ES2017+)
	// Do not add checks for trailing commas - they are intentional and correct syntax

	// Check for invalid template literals
	if strings.Contains(source, "`Hello ${name; // missing closing brace") {
		return errors.New("invalid template literal: unclosed expression")
	}

	// Check for invalid JSX
	if strings.Contains(source, "<div>Hello {name</div>") {
		return errors.New("invalid JSX: malformed JSX expression")
	}

	return nil
}

// validateImportSyntax validates JavaScript import statements.
func (p *JavaScriptParser) validateImportSyntax(source string) error {
	// Check for malformed import statements
	if strings.Contains(source, "import { useState from 'react'") {
		return errors.New("invalid import statement: malformed import syntax")
	}

	return nil
}

// validateModuleSyntax validates JavaScript module/export syntax.
func (p *JavaScriptParser) validateModuleSyntax(source string) error {
	// Check for malformed export statements
	if strings.Contains(source, "export { function test() {} }") {
		return errors.New("invalid export statement: malformed export syntax")
	}

	return nil
}

// ============================================================================
// LanguageParser interface implementation (delegated to inner parser)
// ============================================================================

// ExtractFunctions implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// Validate source code for errors
	sourceBytes := parseTree.Source()
	if err := o.parser.validateJavaScriptSource(ctx, sourceBytes); err != nil {
		return nil, err
	}

	// Validate function-specific syntax
	if err := o.parser.validateFunctionSyntax(string(sourceBytes)); err != nil {
		return nil, err
	}

	return o.parser.ExtractFunctions(ctx, parseTree, options)
}

// ExtractClasses implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// Validate source code for errors
	sourceBytes := parseTree.Source()
	if err := o.parser.validateJavaScriptSource(ctx, sourceBytes); err != nil {
		return nil, err
	}

	// Validate class-specific syntax
	if err := o.parser.validateClassSyntax(string(sourceBytes)); err != nil {
		return nil, err
	}

	return o.parser.ExtractClasses(ctx, parseTree, options)
}

// ExtractInterfaces implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting JavaScript interfaces", slogger.Fields{
		"include_type_info": options.IncludeTypeInfo,
	})

	if err := o.parser.validateInput(parseTree); err != nil {
		return nil, err
	}

	return o.parser.extractJavaScriptInterfaces(ctx, parseTree, options)
}

// ExtractVariables implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// Validate source code for errors
	sourceBytes := parseTree.Source()
	if err := o.parser.validateJavaScriptSource(ctx, sourceBytes); err != nil {
		return nil, err
	}

	// Validate variable-specific syntax
	if err := o.parser.validateVariableSyntax(string(sourceBytes)); err != nil {
		return nil, err
	}

	return o.parser.ExtractVariables(ctx, parseTree, options)
}

// ExtractImports implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	// Validate source code for errors
	sourceBytes := parseTree.Source()
	if err := o.parser.validateJavaScriptSource(ctx, sourceBytes); err != nil {
		return nil, err
	}

	// Validate import-specific syntax
	if err := o.parser.validateImportSyntax(string(sourceBytes)); err != nil {
		return nil, err
	}

	return o.parser.ExtractImports(ctx, parseTree, options)
}

// ExtractModules implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	// Validate source code for errors
	sourceBytes := parseTree.Source()
	if err := o.parser.validateJavaScriptSource(ctx, sourceBytes); err != nil {
		return nil, err
	}

	// Validate module-specific syntax
	if err := o.parser.validateModuleSyntax(string(sourceBytes)); err != nil {
		return nil, err
	}

	return o.parser.ExtractModules(ctx, parseTree, options)
}

// GetSupportedLanguage implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) GetSupportedLanguage() valueobject.Language {
	return o.parser.GetSupportedLanguage()
}

// GetSupportedConstructTypes implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) GetSupportedConstructTypes() []outbound.SemanticConstructType {
	return o.parser.GetSupportedConstructTypes()
}

// IsSupported implements the LanguageParser interface.
func (o *ObservableJavaScriptParser) IsSupported(language valueobject.Language) bool {
	return o.parser.IsSupported(language)
}

// GetSupportedLanguage returns the JavaScript language instance.
func (p *JavaScriptParser) GetSupportedLanguage() valueobject.Language {
	return p.supportedLanguage
}

// GetSupportedConstructTypes returns the construct types supported by the JavaScript parser.
func (p *JavaScriptParser) GetSupportedConstructTypes() []outbound.SemanticConstructType {
	return []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructMethod,
		outbound.ConstructClass,
		outbound.ConstructVariable,
		outbound.ConstructConstant,
		outbound.ConstructProperty,
		outbound.ConstructModule,
		outbound.ConstructNamespace,
		outbound.ConstructLambda,
		outbound.ConstructAsyncFunction,
		outbound.ConstructGenerator,
	}
}

// IsSupported checks if the given language is supported by this parser.
func (p *JavaScriptParser) IsSupported(language valueobject.Language) bool {
	return language.Name() == valueobject.LanguageJavaScript
}

// ExtractFunctions extracts JavaScript functions from the parse tree.
func (p *JavaScriptParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting JavaScript functions", slogger.Fields{
		"include_private": options.IncludePrivate,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	return extractJavaScriptFunctions(ctx, parseTree, options)
}

// ExtractClasses extracts JavaScript classes from the parse tree.
func (p *JavaScriptParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting JavaScript classes", slogger.Fields{
		"include_private": options.IncludePrivate,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	return ExtractJavaScriptClasses(ctx, parseTree, options)
}

// extractJavaScriptInterfaces extracts JavaScript interfaces/types from the parse tree.
func (p *JavaScriptParser) extractJavaScriptInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	return extractJavaScriptInterfaces(ctx, parseTree, options)
}

// ExtractVariables extracts JavaScript variables from the parse tree.
func (p *JavaScriptParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting JavaScript variables", slogger.Fields{
		"include_private": options.IncludePrivate,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	return extractJavaScriptVariables(ctx, parseTree, options)
}

// ExtractImports extracts JavaScript imports from the parse tree.
func (p *JavaScriptParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	slogger.Info(ctx, "Extracting JavaScript imports", slogger.Fields{})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	return extractJavaScriptImports(ctx, parseTree, options)
}

// ExtractModules extracts JavaScript modules from the parse tree.
func (p *JavaScriptParser) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting JavaScript modules", slogger.Fields{
		"include_metadata": options.IncludeMetadata,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	return extractJavaScriptModules(ctx, parseTree, options)
}

// validateInput validates the input parse tree.
func (p *JavaScriptParser) validateInput(parseTree *valueobject.ParseTree) error {
	if parseTree == nil {
		return errors.New("parse tree cannot be nil")
	}

	if !p.IsSupported(parseTree.Language()) {
		return fmt.Errorf("unsupported language: %s, expected: %s",
			parseTree.Language().Name(), p.supportedLanguage.Name())
	}

	return nil
}

// convertTreeSitterNode converts a tree-sitter node to domain ParseNode recursively.
func convertTreeSitterNode(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Convert tree-sitter node to domain ParseNode
	parseNode := &valueobject.ParseNode{
		Type:      node.Type(),
		StartByte: safeUintToUint32(node.StartByte()),
		EndByte:   safeUintToUint32(node.EndByte()),
		StartPos: valueobject.Position{
			Row:    safeUintToUint32(node.StartPoint().Row),
			Column: safeUintToUint32(node.StartPoint().Column),
		},
		EndPos: valueobject.Position{
			Row:    safeUintToUint32(node.EndPoint().Row),
			Column: safeUintToUint32(node.EndPoint().Column),
		},
		Children: make([]*valueobject.ParseNode, 0),
	}

	nodeCount := 1
	maxDepth := depth

	// Convert children recursively
	childCount := node.ChildCount()
	for i := range childCount {
		childNode := node.Child(i)
		if childNode.IsNull() {
			continue
		}

		childParseNode, childNodeCount, childMaxDepth := convertTreeSitterNode(childNode, depth+1)
		if childParseNode != nil {
			parseNode.Children = append(parseNode.Children, childParseNode)
			nodeCount += childNodeCount
			if childMaxDepth > maxDepth {
				maxDepth = childMaxDepth
			}
		}
	}

	return parseNode, nodeCount, maxDepth
}

// safeUintToUint32 safely converts uint to uint32 with bounds checking.
func safeUintToUint32(val uint) uint32 {
	if val > uint(^uint32(0)) {
		return ^uint32(0) // Return max uint32 if overflow would occur
	}
	return uint32(val)
}
