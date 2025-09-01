package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
)

// PythonParser implements LanguageParser for Python language parsing.
type PythonParser struct {
	supportedLanguage valueobject.Language
}

// NewPythonParser creates a new Python parser instance.
func NewPythonParser() (*PythonParser, error) {
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	if err != nil {
		return nil, fmt.Errorf("failed to create Python language: %w", err)
	}

	return &PythonParser{
		supportedLanguage: pythonLang,
	}, nil
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

// Helper function to determine Python visibility.
func getPythonVisibility(identifier string) outbound.VisibilityModifier {
	pythonLang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
	if utils.IsPublicIdentifier(identifier, pythonLang) {
		return outbound.Public
	}
	return outbound.Private
}
