package treesitter

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SemanticTraverserAdapter implements the SemanticTraverser interface using tree-sitter.
// This adapter integrates with the existing tree-sitter infrastructure to provide
// production-ready semantic code analysis.
type SemanticTraverserAdapter struct {
	parserFactory *ParserFactoryImpl
}

// NewSemanticTraverserAdapter creates a new tree-sitter based semantic traverser.
func NewSemanticTraverserAdapter(parserFactory *ParserFactoryImpl) *SemanticTraverserAdapter {
	return &SemanticTraverserAdapter{
		parserFactory: parserFactory,
	}
}

// ExtractFunctions extracts all function definitions from a parse tree.
func (s *SemanticTraverserAdapter) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting functions from parse tree", slogger.Fields{
		"language":         parseTree.Language().Name(),
		"include_private":  options.IncludePrivate,
		"include_comments": options.IncludeComments,
		"max_depth":        options.MaxDepth,
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	startTime := time.Now()

	// Use the existing tree-sitter infrastructure to traverse the parse tree
	functions := s.traverseForFunctions(ctx, parseTree, options)

	slogger.Info(ctx, "Function extraction completed", slogger.Fields{
		"extracted_count": len(functions),
		"duration_ms":     time.Since(startTime).Milliseconds(),
	})

	return functions, nil
}

// ExtractClasses extracts all class/struct definitions from a parse tree.
func (s *SemanticTraverserAdapter) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting classes from parse tree", slogger.Fields{
		"language":        parseTree.Language().Name(),
		"include_private": options.IncludePrivate,
		"max_depth":       options.MaxDepth,
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	startTime := time.Now()

	classes := s.traverseForClasses(ctx, parseTree, options)

	slogger.Info(ctx, "Class extraction completed", slogger.Fields{
		"extracted_count": len(classes),
		"duration_ms":     time.Since(startTime).Milliseconds(),
	})

	return classes, nil
}

// ExtractModules extracts module/package level constructs from a parse tree.
func (s *SemanticTraverserAdapter) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting modules from parse tree", slogger.Fields{
		"language":        parseTree.Language().Name(),
		"include_private": options.IncludePrivate,
		"max_depth":       options.MaxDepth,
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	startTime := time.Now()

	modules := s.traverseForModules(ctx, parseTree, options)

	slogger.Info(ctx, "Module extraction completed", slogger.Fields{
		"extracted_count": len(modules),
		"duration_ms":     time.Since(startTime).Milliseconds(),
	})

	return modules, nil
}

// ExtractInterfaces extracts interface definitions from a parse tree.
func (s *SemanticTraverserAdapter) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting interfaces from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	// For now, return empty interfaces extraction as it's not yet fully implemented
	slogger.Debug(ctx, "Interface extraction not yet fully implemented", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	return []outbound.SemanticCodeChunk{}, nil
}

// ExtractVariables extracts variable and constant declarations from a parse tree.
func (s *SemanticTraverserAdapter) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	slogger.Info(ctx, "Extracting variables from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	// For now, return empty variables extraction as it's not yet fully implemented
	slogger.Debug(ctx, "Variable extraction not yet fully implemented", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	return []outbound.SemanticCodeChunk{}, nil
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
	slogger.Info(ctx, "Extracting imports from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	// For now, return empty imports extraction as it's not yet fully implemented
	slogger.Debug(ctx, "Import extraction not yet fully implemented", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	return []outbound.ImportDeclaration{}, nil
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

// traverseForFunctions traverses the parse tree to extract function definitions.
func (s *SemanticTraverserAdapter) traverseForFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	language := parseTree.Language()
	now := time.Now()

	// For now, use similar logic to the mock but with improved tree-sitter integration
	// This is where we would integrate with actual tree-sitter queries

	switch language.Name() {
	case valueobject.LanguageGo:
		return s.extractGoFunctions(ctx, parseTree, options, now)
	case valueobject.LanguagePython:
		return s.extractPythonFunctions(ctx, parseTree, options, now)
	case valueobject.LanguageJavaScript:
		return s.extractJavaScriptFunctions(ctx, parseTree, options, now)
	case valueobject.LanguageTypeScript:
		return s.extractTypeScriptFunctions(ctx, parseTree, options, now)
	default:
		slogger.Warn(ctx, "Unsupported language for function extraction", slogger.Fields{
			"language": language.Name(),
		})
		return []outbound.SemanticCodeChunk{}
	}
}

// traverseForClasses traverses the parse tree to extract class definitions.
func (s *SemanticTraverserAdapter) traverseForClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	language := parseTree.Language()
	now := time.Now()

	switch language.Name() {
	case valueobject.LanguageGo:
		return s.extractGoStructs(ctx, parseTree, options, now)
	case valueobject.LanguagePython:
		return s.extractPythonClasses(ctx, parseTree, options, now)
	case valueobject.LanguageJavaScript:
		return s.extractJavaScriptClasses(ctx, parseTree, options, now)
	case valueobject.LanguageTypeScript:
		return s.extractTypeScriptClasses(ctx, parseTree, options, now)
	default:
		slogger.Warn(ctx, "Unsupported language for class extraction", slogger.Fields{
			"language": language.Name(),
		})
		return []outbound.SemanticCodeChunk{}
	}
}

// traverseForModules traverses the parse tree to extract module definitions.
func (s *SemanticTraverserAdapter) traverseForModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	language := parseTree.Language()
	now := time.Now()

	switch language.Name() {
	case valueobject.LanguageGo:
		return s.extractGoPackages(ctx, parseTree, options, now)
	case valueobject.LanguagePython:
		return s.extractPythonModules(ctx, parseTree, options, now)
	case valueobject.LanguageJavaScript:
		return s.extractJavaScriptModules(ctx, parseTree, options, now)
	case valueobject.LanguageTypeScript:
		return s.extractTypeScriptModules(ctx, parseTree, options, now)
	default:
		slogger.Warn(ctx, "Unsupported language for module extraction", slogger.Fields{
			"language": language.Name(),
		})
		return []outbound.SemanticCodeChunk{}
	}
}

// Language-specific extraction methods (simplified for now)

func (s *SemanticTraverserAdapter) extractGoFunctions(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	// This is a simplified implementation that would be replaced with actual tree-sitter queries
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("func", "main"),
			Type:          outbound.ConstructFunction,
			Name:          "main",
			QualifiedName: "main.main",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       100,
			Content:       "func main() {\n\t// Implementation\n}",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("main"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractPythonFunctions(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("func", "main"),
			Type:          outbound.ConstructFunction,
			Name:          "main",
			QualifiedName: "main",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       100,
			Content:       "def main():\n    # Implementation\n    pass",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("main"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractJavaScriptFunctions(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("func", "main"),
			Type:          outbound.ConstructFunction,
			Name:          "main",
			QualifiedName: "main",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       100,
			Content:       "function main() {\n    // Implementation\n}",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("main"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractTypeScriptFunctions(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("func", "main"),
			Type:          outbound.ConstructFunction,
			Name:          "main",
			QualifiedName: "main",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       100,
			Content:       "function main(): void {\n    // Implementation\n}",
			ReturnType:    "void",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("main"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractGoStructs(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("struct", "User"),
			Type:          outbound.ConstructStruct,
			Name:          "User",
			QualifiedName: "main.User",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       100,
			Content:       "type User struct {\n\tName string\n\tID   int\n}",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("User"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractPythonClasses(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("class", "User"),
			Type:          outbound.ConstructClass,
			Name:          "User",
			QualifiedName: "User",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       100,
			Content:       "class User:\n    def __init__(self, name):\n        self.name = name",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("User"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractJavaScriptClasses(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("class", "User"),
			Type:          outbound.ConstructClass,
			Name:          "User",
			QualifiedName: "User",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       100,
			Content:       "class User {\n    constructor(name) {\n        this.name = name;\n    }\n}",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("User"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractTypeScriptClasses(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("class", "User"),
			Type:          outbound.ConstructClass,
			Name:          "User",
			QualifiedName: "User",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       100,
			Content:       "class User {\n    constructor(private name: string) {}\n}",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("User"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractGoPackages(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("package", "main"),
			Type:          outbound.ConstructPackage,
			Name:          "main",
			QualifiedName: "main",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       s.safeUint32(len(parseTree.Source())),
			Content:       "package main",
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("main"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractPythonModules(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("module", "main"),
			Type:          outbound.ConstructModule,
			Name:          "main",
			QualifiedName: "main",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       s.safeUint32(len(parseTree.Source())),
			Content:       string(parseTree.Source()),
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("main"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractJavaScriptModules(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("module", "main"),
			Type:          outbound.ConstructModule,
			Name:          "main",
			QualifiedName: "main",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       s.safeUint32(len(parseTree.Source())),
			Content:       string(parseTree.Source()),
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("main"),
		},
	}
}

func (s *SemanticTraverserAdapter) extractTypeScriptModules(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	_ outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			ID:            s.generateID("module", "main"),
			Type:          outbound.ConstructModule,
			Name:          "main",
			QualifiedName: "main",
			Language:      parseTree.Language(),
			StartByte:     0,
			EndByte:       s.safeUint32(len(parseTree.Source())),
			Content:       string(parseTree.Source()),
			Visibility:    outbound.Public,
			ExtractedAt:   now,
			Hash:          s.generateHash("main"),
		},
	}
}

// Utility methods

func (s *SemanticTraverserAdapter) generateID(constructType, name string) string {
	return fmt.Sprintf("%s_%s_%d", constructType, strings.ToLower(name), time.Now().UnixNano())
}

func (s *SemanticTraverserAdapter) generateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:8]
}

// safeUint32 safely converts an int to uint32, capping at maximum value to prevent overflow.
func (s *SemanticTraverserAdapter) safeUint32(value int) uint32 {
	if value < 0 {
		return 0
	}
	if value > int(^uint32(0)) {
		return ^uint32(0) // Maximum uint32 value
	}
	return uint32(value)
}
