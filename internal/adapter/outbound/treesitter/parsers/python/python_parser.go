package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	parsererrors "codechunking/internal/adapter/outbound/treesitter/errors"
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"time"
)

// PythonParser implements LanguageParser for Python language parsing.
type PythonParser struct {
	supportedLanguage valueobject.Language
}

// ObservablePythonParser wraps PythonParser to implement both ObservableTreeSitterParser and LanguageParser interfaces.
type ObservablePythonParser struct {
	parser *PythonParser
}

// ParseOptions represents parsing options.
type ParseOptions struct {
	Timeout time.Duration
}

// NewPythonParser creates a new Python parser instance.
func NewPythonParser() (treesitter.ObservableTreeSitterParser, error) {
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	if err != nil {
		return nil, fmt.Errorf("failed to create Python language: %w", err)
	}

	parser := &PythonParser{
		supportedLanguage: pythonLang,
	}

	return &ObservablePythonParser{
		parser: parser,
	}, nil
}

// Parse implements the ObservableTreeSitterParser interface.
func (o *ObservablePythonParser) Parse(ctx context.Context, source []byte) (*treesitter.ParseResult, error) {
	start := time.Now()

	// Validate source code for errors
	if err := o.parser.validatePythonSource(ctx, source); err != nil {
		return nil, err
	}

	// Create a minimal rootNode
	rootNode := &valueobject.ParseNode{
		Type:      "module",
		StartByte: 0,
		EndByte:   valueobject.ClampToUint32(len(source)),
		StartPos:  valueobject.Position{Row: 0, Column: 0},
		EndPos:    valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(len(source))},
		Children:  nil,
	}

	// Create minimal metadata
	metadata, err := valueobject.NewParseMetadata(
		time.Since(start),
		"0.0.0", // treeSitterVersion
		"0.0.0", // grammarVersion
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse metadata: %w", err)
	}

	// Create a minimal parse tree
	domainTree, err := valueobject.NewParseTree(ctx, o.parser.supportedLanguage, rootNode, source, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create domain parse tree: %w", err)
	}

	// Convert to port tree
	portTree, err := treesitter.ConvertDomainParseTreeToPort(domainTree)
	if err != nil {
		return nil, fmt.Errorf("failed to convert parse tree: %w", err)
	}

	duration := time.Since(start)

	return &treesitter.ParseResult{
		Success:   true,
		ParseTree: portTree,
		Duration:  duration,
	}, nil
}

// ParseSource implements the ObservableTreeSitterParser interface.
func (o *ObservablePythonParser) ParseSource(
	ctx context.Context,
	language valueobject.Language,
	source []byte,
	options treesitter.ParseOptions,
) (*treesitter.ParseResult, error) {
	return o.Parse(ctx, source)
}

// GetLanguage implements the ObservableTreeSitterParser interface.
func (o *ObservablePythonParser) GetLanguage() string {
	return "python"
}

// Close implements the ObservableTreeSitterParser interface.
func (o *ObservablePythonParser) Close() error {
	return nil
}

// ============================================================================
// LanguageParser interface implementation (delegated to inner parser)
// ============================================================================

// ExtractFunctions implements the LanguageParser interface.
func (o *ObservablePythonParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return o.parser.ExtractFunctions(ctx, parseTree, options)
}

// ExtractClasses implements the LanguageParser interface.
func (o *ObservablePythonParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return o.parser.ExtractClasses(ctx, parseTree, options)
}

// ExtractInterfaces implements the LanguageParser interface.
func (o *ObservablePythonParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return o.parser.ExtractInterfaces(ctx, parseTree, options)
}

// ExtractVariables implements the LanguageParser interface.
func (o *ObservablePythonParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return o.parser.ExtractVariables(ctx, parseTree, options)
}

// ExtractImports implements the LanguageParser interface.
func (o *ObservablePythonParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	return o.parser.ExtractImports(ctx, parseTree, options)
}

// ExtractModules implements the LanguageParser interface.
func (o *ObservablePythonParser) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return o.parser.ExtractModules(ctx, parseTree, options)
}

// GetSupportedLanguage implements the LanguageParser interface.
func (o *ObservablePythonParser) GetSupportedLanguage() valueobject.Language {
	return o.parser.GetSupportedLanguage()
}

// GetSupportedConstructTypes implements the LanguageParser interface.
func (o *ObservablePythonParser) GetSupportedConstructTypes() []outbound.SemanticConstructType {
	return o.parser.GetSupportedConstructTypes()
}

// IsSupported implements the LanguageParser interface.
func (o *ObservablePythonParser) IsSupported(language valueobject.Language) bool {
	return o.parser.IsSupported(language)
}

// GetSupportedLanguage returns the Python language instance.
func (p *PythonParser) GetSupportedLanguage() valueobject.Language {
	return p.supportedLanguage
}

// GetSupportedConstructTypes returns the construct types supported by the Python parser.
func (p *PythonParser) GetSupportedConstructTypes() []outbound.SemanticConstructType {
	return []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructMethod,
		outbound.ConstructClass,
		outbound.ConstructVariable,
		outbound.ConstructConstant,
		outbound.ConstructModule,
	}
}

// IsSupported checks if the given language is supported by this parser.
func (p *PythonParser) IsSupported(language valueobject.Language) bool {
	return language.Name() == valueobject.LanguagePython
}

// ExtractFunctions extracts Python functions from the parse tree.
func (p *PythonParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Python functions", slogger.Fields{
		"include_private": options.IncludePrivate,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	// Validate source syntax for function extraction
	if err := p.validatePythonSource(ctx, parseTree.Source()); err != nil {
		return nil, err
	}

	return extractPythonFunctions(ctx, parseTree, options)
}

// ExtractClasses extracts Python classes from the parse tree.
func (p *PythonParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Python classes", slogger.Fields{
		"include_private": options.IncludePrivate,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	// Validate source syntax for class extraction
	if err := p.validatePythonSource(ctx, parseTree.Source()); err != nil {
		return nil, err
	}

	return extractPythonClasses(ctx, parseTree, options)
}

// ExtractInterfaces extracts Python protocols/interfaces from the parse tree.
func (p *PythonParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Python interfaces/protocols", slogger.Fields{
		"include_type_info": options.IncludeTypeInfo,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	return extractPythonInterfaces(ctx, parseTree, options)
}

// ExtractVariables extracts Python variables from the parse tree.
func (p *PythonParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Python variables", slogger.Fields{
		"include_private": options.IncludePrivate,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	// Validate source syntax for variable extraction
	if err := p.validatePythonSource(ctx, parseTree.Source()); err != nil {
		return nil, err
	}

	return extractPythonVariables(ctx, parseTree, options)
}

// ExtractImports extracts Python imports from the parse tree.
func (p *PythonParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	slogger.Info(ctx, "Extracting Python imports", slogger.Fields{})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	// Validate source syntax for import extraction
	if err := p.validatePythonSource(ctx, parseTree.Source()); err != nil {
		return nil, err
	}

	return extractPythonImports(ctx, parseTree, options)
}

// ExtractModules extracts Python modules from the parse tree.
func (p *PythonParser) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting Python modules", slogger.Fields{
		"include_metadata": options.IncludeMetadata,
	})

	if err := p.validateInput(parseTree); err != nil {
		return nil, err
	}

	return extractPythonModules(ctx, parseTree, options)
}

// validateInput validates the input parse tree.
func (p *PythonParser) validateInput(parseTree *valueobject.ParseTree) error {
	if parseTree == nil {
		return errors.New("parse tree cannot be nil")
	}

	if !p.IsSupported(parseTree.Language()) {
		return fmt.Errorf("unsupported language: %s, expected: %s",
			parseTree.Language().Name(), p.supportedLanguage.Name())
	}

	return nil
}

// validatePythonSource performs comprehensive validation of Python source code using the new error handling system.
func (p *PythonParser) validatePythonSource(ctx context.Context, source []byte) error {
	// Use the shared validation system with Python-specific limits
	limits := parsererrors.DefaultValidationLimits()

	// Create a validator registry with Python validator
	registry := parsererrors.DefaultValidatorRegistry()
	registry.RegisterValidator("Python", parsererrors.NewPythonValidator())

	// Perform comprehensive validation
	if err := parsererrors.ValidateSourceWithLanguage(ctx, source, "Python", limits, registry); err != nil {
		return err
	}

	return nil
}

// Helper function to determine Python visibility.
func getPythonVisibility(identifier string) outbound.VisibilityModifier {
	pythonLang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
	if utils.IsPublicIdentifier(identifier, pythonLang) {
		return outbound.Public
	}
	return outbound.Private
}
