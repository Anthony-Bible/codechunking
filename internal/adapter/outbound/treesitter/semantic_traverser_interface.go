package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
)

// LocalLanguageParser defines the interface for language-specific parsers
// without creating import dependencies. This breaks the circular dependency
// by allowing semantic_traverser_adapter to work with any parser implementation.
type LocalLanguageParser interface {
	// ExtractVariables extracts variable and constant declarations
	ExtractVariables(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.SemanticCodeChunk, error)

	// ExtractFunctions extracts function and method declarations
	ExtractFunctions(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options outbound.SemanticExtractionOptions,
	) ([]outbound.SemanticCodeChunk, error)

	// Parse parses source code into a parse tree
	Parse(ctx context.Context, source []byte) (*ParseResult, error)
}

// LocalParserFactory creates parsers without import cycles
type LocalParserFactory interface {
	CreateLocalParser(ctx context.Context, language valueobject.Language) (LocalLanguageParser, error)
}
