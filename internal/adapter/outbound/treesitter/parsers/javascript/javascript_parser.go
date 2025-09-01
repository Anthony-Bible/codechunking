package javascriptparser

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
)

// JavaScriptParser implements LanguageParser for JavaScript language parsing.
type JavaScriptParser struct {
	supportedLanguage valueobject.Language
}

// NewJavaScriptParser creates a new JavaScript parser instance.
func NewJavaScriptParser() (*JavaScriptParser, error) {
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	if err != nil {
		return nil, fmt.Errorf("failed to create JavaScript language: %w", err)
	}

	return &JavaScriptParser{
		supportedLanguage: jsLang,
	}, nil
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

	return extractJavaScriptClasses(ctx, parseTree, options)
}

// ExtractInterfaces extracts JavaScript interfaces/types from the parse tree.
func (p *JavaScriptParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting JavaScript interfaces", slogger.Fields{
		"include_type_info": options.IncludeTypeInfo,
	})

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
