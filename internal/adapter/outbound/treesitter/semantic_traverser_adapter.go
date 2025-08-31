package treesitter

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"time"
)

// SemanticTraverserAdapter implements the SemanticTraverser interface using tree-sitter.
// This adapter integrates with the existing tree-sitter infrastructure to provide
// production-ready semantic code analysis.
type SemanticTraverserAdapter struct {
	parserFactory      *ParserFactoryImpl
	languageDispatcher *LanguageDispatcher
}

// NewSemanticTraverserAdapter creates a new tree-sitter based semantic traverser.
func NewSemanticTraverserAdapter(parserFactory *ParserFactoryImpl) *SemanticTraverserAdapter {
	languageDispatcher, err := NewLanguageDispatcher()
	if err != nil {
		// Log warning but continue - maintain backward compatibility
		slogger.InfoNoCtx("Failed to create language dispatcher, some features may be limited", slogger.Fields{
			"error": err.Error(),
		})
	}

	return &SemanticTraverserAdapter{
		parserFactory:      parserFactory,
		languageDispatcher: languageDispatcher,
	}
}

// extractSemanticChunks is a helper method to eliminate duplication in extraction methods.
func (s *SemanticTraverserAdapter) extractSemanticChunks(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	extractionType string,
	langParserExtractor func(LanguageParser) ([]outbound.SemanticCodeChunk, error),
	legacyExtractor func() []outbound.SemanticCodeChunk,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting "+extractionType+" from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	// Try to use language dispatcher if available
	if s.languageDispatcher != nil {
		parser, err := s.languageDispatcher.CreateParser(parseTree.Language())
		if err == nil {
			return langParserExtractor(parser)
		}
		slogger.Debug(ctx, "Language dispatcher unavailable, falling back to legacy implementation", slogger.Fields{
			"error": err.Error(),
		})
	}

	// Fallback to existing legacy implementation for backward compatibility
	return legacyExtractor(), nil
}

// extractImports is a helper method for import extraction (different return type).
func (s *SemanticTraverserAdapter) extractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	langParserExtractor func(LanguageParser) ([]outbound.ImportDeclaration, error),
	legacyExtractor func() []outbound.ImportDeclaration,
) ([]outbound.ImportDeclaration, error) {
	slogger.Info(ctx, "Extracting imports from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	// Try to use language dispatcher if available
	if s.languageDispatcher != nil {
		parser, err := s.languageDispatcher.CreateParser(parseTree.Language())
		if err == nil {
			return langParserExtractor(parser)
		}
		slogger.Debug(ctx, "Language dispatcher unavailable, falling back to legacy implementation", slogger.Fields{
			"error": err.Error(),
		})
	}

	// Fallback to existing legacy implementation for backward compatibility
	return legacyExtractor(), nil
}

// ExtractFunctions extracts all function definitions from a parse tree.
func (s *SemanticTraverserAdapter) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	startTime := time.Now()

	result, err := s.extractSemanticChunks(
		ctx,
		parseTree,
		options,
		"functions",
		func(parser LanguageParser) ([]outbound.SemanticCodeChunk, error) {
			return parser.ExtractFunctions(ctx, parseTree, options)
		},
		func() []outbound.SemanticCodeChunk {
			return s.traverseForFunctions(ctx, parseTree, options)
		},
	)

	if err == nil {
		slogger.Info(ctx, "Function extraction completed", slogger.Fields{
			"extracted_count": len(result),
			"duration_ms":     time.Since(startTime).Milliseconds(),
		})
	}

	return result, err
}

// ExtractClasses extracts all class/struct definitions from a parse tree.
func (s *SemanticTraverserAdapter) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	startTime := time.Now()

	result, err := s.extractSemanticChunks(
		ctx,
		parseTree,
		options,
		"classes",
		func(parser LanguageParser) ([]outbound.SemanticCodeChunk, error) {
			return parser.ExtractClasses(ctx, parseTree, options)
		},
		func() []outbound.SemanticCodeChunk {
			return s.traverseForClasses(ctx, parseTree, options)
		},
	)

	if err == nil {
		slogger.Info(ctx, "Class extraction completed", slogger.Fields{
			"extracted_count": len(result),
			"duration_ms":     time.Since(startTime).Milliseconds(),
		})
	}

	return result, err
}

// ExtractModules extracts module/package level constructs from a parse tree.
func (s *SemanticTraverserAdapter) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	startTime := time.Now()

	result, err := s.extractSemanticChunks(
		ctx,
		parseTree,
		options,
		"modules",
		func(parser LanguageParser) ([]outbound.SemanticCodeChunk, error) {
			return parser.ExtractModules(ctx, parseTree, options)
		},
		func() []outbound.SemanticCodeChunk {
			return s.traverseForModules(ctx, parseTree, options)
		},
	)

	if err == nil {
		slogger.Info(ctx, "Module extraction completed", slogger.Fields{
			"extracted_count": len(result),
			"duration_ms":     time.Since(startTime).Milliseconds(),
		})
	}

	return result, err
}

// ExtractInterfaces extracts interface definitions from a parse tree.
func (s *SemanticTraverserAdapter) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return s.extractSemanticChunks(
		ctx,
		parseTree,
		options,
		"interfaces",
		func(parser LanguageParser) ([]outbound.SemanticCodeChunk, error) {
			return parser.ExtractInterfaces(ctx, parseTree, options)
		},
		func() []outbound.SemanticCodeChunk {
			return s.extractGoInterfaces(ctx, parseTree, options, time.Now())
		},
	)
}

// ExtractVariables extracts variable and constant declarations from a parse tree.
func (s *SemanticTraverserAdapter) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return s.extractSemanticChunks(
		ctx,
		parseTree,
		options,
		"variables",
		func(parser LanguageParser) ([]outbound.SemanticCodeChunk, error) {
			return parser.ExtractVariables(ctx, parseTree, options)
		},
		func() []outbound.SemanticCodeChunk {
			return s.extractGoVariables(ctx, parseTree, options, time.Now())
		},
	)
}

// ExtractComments extracts documentation and comments from a parse tree.
func (s *SemanticTraverserAdapter) ExtractComments(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting comments from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	// For now, return empty comments extraction as it's not yet fully implemented
	slogger.Debug(ctx, "Comment extraction not yet fully implemented", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	return []outbound.SemanticCodeChunk{}, nil
}

// ExtractImports extracts import/include statements from a parse tree.
func (s *SemanticTraverserAdapter) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	return s.extractImports(
		ctx,
		parseTree,
		options,
		func(parser LanguageParser) ([]outbound.ImportDeclaration, error) {
			return parser.ExtractImports(ctx, parseTree, options)
		},
		func() []outbound.ImportDeclaration {
			return s.extractGoImports(ctx, parseTree, options)
		},
	)
}

// GetSupportedConstructTypes returns the semantic constructs this traverser can extract.
func (s *SemanticTraverserAdapter) GetSupportedConstructTypes(
	ctx context.Context,
	language valueobject.Language,
) ([]outbound.SemanticConstructType, error) {
	slogger.Debug(ctx, "Getting supported construct types", slogger.Fields{
		"language": language.Name(),
	})

	// Check if the language is supported by our parser factory
	supportedLanguages := []string{
		valueobject.LanguageGo,
		valueobject.LanguagePython,
		valueobject.LanguageJavaScript,
		valueobject.LanguageTypeScript,
	}

	isSupported := false
	for _, supported := range supportedLanguages {
		if language.Name() == supported {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return nil, fmt.Errorf("unsupported language: %s", language.Name())
	}

	// Return the constructs we can extract based on language
	return []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructMethod,
		outbound.ConstructClass,
		outbound.ConstructStruct,
		outbound.ConstructModule,
		outbound.ConstructPackage,
	}, nil
}

// validateInput validates the input parameters for extraction methods.
func (s *SemanticTraverserAdapter) validateInput(
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) error {
	if parseTree == nil {
		return errors.New("parse tree cannot be nil")
	}

	if options.MaxDepth < 0 {
		return errors.New("invalid option: max depth cannot be negative")
	}

	return nil
}

// Legacy parsing methods have been removed and replaced with modular language parsers.
// All parsing functionality is now delegated to language-specific parsers via LanguageDispatcher.
// Legacy method stubs for backward compatibility - all functionality delegated to language parsers.
func (s *SemanticTraverserAdapter) traverseForFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	// This method is now a fallback - all functionality delegated to language parsers
	slogger.Warn(ctx, "Using legacy fallback - language parser unavailable", slogger.Fields{
		"language": parseTree.Language().Name(),
	})
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) traverseForClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	slogger.Warn(ctx, "Using legacy fallback - language parser unavailable", slogger.Fields{
		"language": parseTree.Language().Name(),
	})
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) traverseForModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	slogger.Warn(ctx, "Using legacy fallback - language parser unavailable", slogger.Fields{
		"language": parseTree.Language().Name(),
	})
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) extractGoInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	slogger.Warn(ctx, "Using legacy fallback - language parser unavailable", slogger.Fields{
		"language": parseTree.Language().Name(),
	})
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) extractGoVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	slogger.Warn(ctx, "Using legacy fallback - language parser unavailable", slogger.Fields{
		"language": parseTree.Language().Name(),
	})
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) extractGoImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.ImportDeclaration {
	slogger.Warn(ctx, "Using legacy fallback - language parser unavailable", slogger.Fields{
		"language": parseTree.Language().Name(),
	})
	return []outbound.ImportDeclaration{}
}
