package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
)

// LanguageParser interface defines language-specific parsing behavior.
// This interface abstracts language-specific logic from SemanticTraverserAdapter.
type LanguageParser interface {
	// Extraction methods for different semantic constructs
	ExtractFunctions(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.SemanticCodeChunk, error)

	ExtractClasses(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.SemanticCodeChunk, error)

	ExtractInterfaces(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.SemanticCodeChunk, error)

	ExtractVariables(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.SemanticCodeChunk, error)

	ExtractImports(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.ImportDeclaration, error)

	ExtractModules(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.SemanticCodeChunk, error)

	// Language information methods
	GetSupportedLanguage() valueobject.Language
	GetSupportedConstructTypes() []outbound.SemanticConstructType
	IsSupported(language valueobject.Language) bool
}

// GoLanguageParser interface extends LanguageParser with Go-specific methods.
type GoLanguageParser interface {
	LanguageParser
	// Go-specific methods for advanced parsing
	ExtractMethodsFromStruct(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		structName string,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.SemanticCodeChunk, error)

	ExtractGenericParameters(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.GenericParameter, error)
}

// LanguageParserFactory interface for creating language-specific parsers.
type LanguageParserFactory interface {
	CreateParser(language valueobject.Language) (LanguageParser, error)
	GetSupportedLanguages() []valueobject.Language
	IsLanguageSupported(language valueobject.Language) bool
}
