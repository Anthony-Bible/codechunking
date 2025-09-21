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
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Constants for node types to avoid goconst issues.
const (
	nodeFunctionDeclaration = "function_declaration"
	nodeMethodDeclaration   = "method_declaration"
	nodeIdentifier          = "identifier"
	nodeFieldIdentifier     = "field_identifier"
	nodeFunctionExpression  = "function_expression"
	nodeArrowFunction       = "arrow_function"
	nodeVariableDeclarator  = "variable_declarator"
)

// SemanticExtractionOptions defines options for semantic code chunk extraction.
type SemanticExtractionOptions struct {
	IncludeFunctions     bool
	IncludeStructs       bool
	IncludeInterfaces    bool
	IncludeVariables     bool
	IncludeConstants     bool
	IncludePackages      bool
	IncludePrivate       bool
	IncludeComments      bool
	IncludeDocumentation bool
	IncludeTypeInfo      bool
	IncludeDependencies  bool
	IncludeMetadata      bool
	MaxDepth             int
}

// SemanticTraverserAdapter implements the SemanticTraverser interface using tree-sitter.
// This adapter integrates with the existing tree-sitter infrastructure to provide
// production-ready semantic code analysis.
type SemanticTraverserAdapter struct {
	parserFactory *ParserFactoryImpl
	// languageDispatcher *LanguageDispatcher // Disabled to avoid import cycle
}

// NewSemanticTraverserAdapter creates a new semantic traverser adapter with default factory.
func NewSemanticTraverserAdapter() *SemanticTraverserAdapter {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	if err != nil {
		slogger.InfoNoCtx("Failed to create parser factory for semantic traverser, using nil", slogger.Fields{
			"error": err.Error(),
		})
		factory = nil
	}

	return NewSemanticTraverserAdapterWithFactory(factory)
}

// NewSemanticTraverserAdapterWithFactory creates a semantic traverser with provided factory.
func NewSemanticTraverserAdapterWithFactory(parserFactory *ParserFactoryImpl) *SemanticTraverserAdapter {
	// Don't use language dispatcher to avoid import cycles
	// All functionality will use fallback implementations for now

	return &SemanticTraverserAdapter{
		parserFactory: parserFactory,
		// languageDispatcher: nil, // Skip to avoid import cycle
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
	// Fix nil pointer panic: check parseTree before calling Language()
	if parseTree == nil {
		return nil, errors.New("parse tree cannot be nil")
	}

	slogger.Info(ctx, "Extracting "+extractionType+" from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	// Try to use parser factory directly if available
	if result, err := s.tryParserFactory(ctx, parseTree, langParserExtractor); result != nil {
		return result, err
	} else {
		slogger.Warn(ctx, "Parser factory is nil, falling back to legacy implementation", slogger.Fields{})
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
	// Fix nil pointer panic: check parseTree before calling Language()
	if parseTree == nil {
		return nil, errors.New("parse tree cannot be nil")
	}

	slogger.Info(ctx, "Extracting imports from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	if err := s.validateInput(parseTree, options); err != nil {
		return nil, err
	}

	// Try to use language dispatcher if available
	// if s.languageDispatcher != nil {
	// 	parser, err := s.languageDispatcher.CreateParser(parseTree.Language())
	// 	if err == nil {
	// 		return langParserExtractor(parser)
	// 	}
	// 	slogger.Warn(
	// 		ctx,
	// 		"Tree-sitter language dispatcher unavailable for import extraction, falling back to legacy implementation",
	// 		slogger.Fields{
	// 			"error": err.Error(),
	// 		},
	// 	)
	// }

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
	// Fix nil pointer panic: check parseTree before calling Language()
	if parseTree == nil {
		return nil, errors.New("parse tree cannot be nil")
	}

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
			switch parseTree.Language().Name() {
			case valueobject.LanguageJavaScript:
				return s.extractJavaScriptImports(ctx, parseTree, options)
			case valueobject.LanguageGo:
				return s.extractGoImports(ctx, parseTree, options)
			default:
				return []outbound.ImportDeclaration{}
			}
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

// ExtractCodeChunks extracts all requested semantic code chunks from the parse tree based on options.
func (s *SemanticTraverserAdapter) ExtractCodeChunks(
	parseTree *valueobject.ParseTree,
	options *SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	ctx := context.Background()
	var allChunks []outbound.SemanticCodeChunk

	// Use the outbound.SemanticExtractionOptions type instead
	outboundOptions := outbound.SemanticExtractionOptions{
		IncludePrivate:       options.IncludePrivate,
		IncludeComments:      false,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		IncludeDependencies:  false,
		IncludeMetadata:      false,
		MaxDepth:             100,
	}

	// Extract functions if requested
	if options.IncludeFunctions {
		if funcs, err := s.ExtractFunctions(ctx, parseTree, outboundOptions); err == nil {
			allChunks = append(allChunks, funcs...)
		}
	}

	// Extract methods (included in functions)
	// Methods are extracted as part of ExtractFunctions

	// Extract classes/structs if requested
	if options.IncludeStructs {
		if classes, err := s.ExtractClasses(ctx, parseTree, outboundOptions); err == nil {
			allChunks = append(allChunks, classes...)
		}
	}

	// Extract interfaces if requested
	if options.IncludeInterfaces {
		if interfaces, err := s.ExtractInterfaces(ctx, parseTree, outboundOptions); err == nil {
			allChunks = append(allChunks, interfaces...)
		}
	}

	// Extract variables if requested
	if options.IncludeVariables || options.IncludeConstants {
		if vars, err := s.ExtractVariables(ctx, parseTree, outboundOptions); err == nil {
			allChunks = append(allChunks, vars...)
		}
	}

	// Extract modules/packages if requested
	if options.IncludePackages {
		if modules, err := s.ExtractModules(ctx, parseTree, outboundOptions); err == nil {
			allChunks = append(allChunks, modules...)
		}
	}

	return allChunks
}

// Legacy parsing methods have been removed and replaced with modular language parsers.
// All parsing functionality is now delegated to language-specific parsers via LanguageDispatcher.
// Legacy method stubs for backward compatibility - all functionality delegated to language parsers.
func (s *SemanticTraverserAdapter) traverseForFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	slogger.Info(ctx, "traverseForFunctions called", slogger.Fields{
		"language_name": parseTree.Language().Name(),
		"is_javascript": parseTree.Language().Name() == valueobject.LanguageJavaScript,
	})

	if parseTree.Language().Name() == valueobject.LanguageJavaScript {
		slogger.Info(ctx, "Routing to JavaScript function extraction", slogger.Fields{})
		return s.extractJavaScriptFunctions(parseTree)
	}

	if parseTree.Language().Name() == valueobject.LanguageGo {
		slogger.Info(ctx, "Routing to Go function extraction", slogger.Fields{})
		return s.extractGoFunctions(parseTree)
	}

	// This method is now a fallback - all functionality delegated to language parsers
	slogger.Warn(
		ctx,
		"Using tree-sitter legacy fallback for function extraction - language parser unavailable",
		slogger.Fields{
			"language": parseTree.Language().Name(),
		},
	)
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) traverseForClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	// Return empty for now as we're focusing on function extraction
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) traverseForModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.SemanticCodeChunk {
	// Return empty for now as we're focusing on function extraction
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) extractGoInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	// Return empty for now as we're focusing on function extraction
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) extractGoVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	// Return empty for now as we're focusing on function extraction
	return []outbound.SemanticCodeChunk{}
}

func (s *SemanticTraverserAdapter) extractGoImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.ImportDeclaration {
	// Return empty for now as we're focusing on function extraction
	return []outbound.ImportDeclaration{}
}

// extractJavaScriptFunctions implements a simple JavaScript function extraction directly.
func (s *SemanticTraverserAdapter) extractGoFunctions(
	parseTree *valueobject.ParseTree,
) []outbound.SemanticCodeChunk {
	ctx := context.Background()

	// Debug log at the start showing root node info
	slogger.Info(ctx, "Starting Go function extraction", slogger.Fields{
		"root_node_type": parseTree.RootNode().Type,
		"children_count": len(parseTree.RootNode().Children),
	})

	var chunks []outbound.SemanticCodeChunk

	// Traverse the AST for Go function nodes
	s.traverseGoAST(parseTree.RootNode(), func(node *valueobject.ParseNode) {
		// Debug: log all node types we encounter
		slogger.Debug(ctx, "Encountered AST node during traversal", slogger.Fields{
			"node_type": node.Type,
		})

		switch node.Type {
		case nodeFunctionDeclaration, nodeMethodDeclaration:
			if chunk := s.extractGoFunctionFromNode(node, parseTree); chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}
	})

	slogger.Info(ctx, "Go function extraction completed", slogger.Fields{
		"extracted_count": len(chunks),
	})

	return chunks
}

// traverseGoAST recursively traverses the Go AST, calling visitFunc for each node.
func (s *SemanticTraverserAdapter) traverseGoAST(
	node *valueobject.ParseNode,
	visitFunc func(*valueobject.ParseNode),
) {
	if node == nil {
		return
	}

	slogger.Debug(context.Background(), "traverseGoAST called", slogger.Fields{
		"node_type":      node.Type,
		"children_count": len(node.Children),
	})

	visitFunc(node)
	for _, child := range node.Children {
		if child != nil {
			s.traverseGoAST(child, visitFunc)
		}
	}
}

// extractGoFunctionFromNode extracts a Go function from an AST node.
func (s *SemanticTraverserAdapter) extractGoFunctionFromNode(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) *outbound.SemanticCodeChunk {
	if node == nil || parseTree == nil {
		return nil
	}

	// Extract function name
	name := s.extractGoFunctionName(node, parseTree)
	if name == "" {
		return nil
	}

	// Determine visibility (public if starts with capital letter)
	visibility := outbound.Private
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		visibility = outbound.Public
	}

	// Extract package name from parse tree (simplified)
	packageName := s.extractGoPackageName(parseTree)
	qualifiedName := packageName + "." + name

	// Get function content
	content := parseTree.GetNodeText(node)

	// Generate hash for the function
	hash := s.simpleHash(content)

	// Determine construct type first
	constructType := outbound.ConstructFunction
	if node.Type == nodeMethodDeclaration {
		constructType = outbound.ConstructMethod
	}

	// Generate ID
	var id string
	if constructType == outbound.ConstructMethod {
		id = fmt.Sprintf("method_%s_%d", name, hash)
	} else {
		id = fmt.Sprintf("func_%s_%d", name, hash)
	}

	chunk := &outbound.SemanticCodeChunk{
		ChunkID:       id,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		Type:          constructType,
		Visibility:    visibility,
		Content:       content,
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		ExtractedAt:   time.Now(),
		Hash:          strconv.FormatUint(uint64(hash), 10),
	}

	return chunk
}

// extractGoFunctionName extracts the name of a Go function from an AST node.
// Uses tree-sitter Go grammar structure for robust name extraction.
func (s *SemanticTraverserAdapter) extractGoFunctionName(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) string {
	if node == nil || parseTree == nil {
		return ""
	}

	switch node.Type {
	case nodeFunctionDeclaration:
		// For function_declaration, look for identifier child
		// Based on tree-sitter go grammar: func <identifier> ...
		if identifierNode := s.findChildByType(node, nodeIdentifier); identifierNode != nil {
			return parseTree.GetNodeText(identifierNode)
		}

	case nodeMethodDeclaration:
		// For method_declaration, look for field_identifier child
		// Based on tree-sitter go grammar: func (<receiver>) <field_identifier> ...
		if fieldIdentifierNode := s.findChildByType(node, nodeFieldIdentifier); fieldIdentifierNode != nil {
			return parseTree.GetNodeText(fieldIdentifierNode)
		}
	}

	return ""
}

// findChildByType searches for a direct child node of the specified type.
// This provides a more robust alternative to positional traversal.
func (s *SemanticTraverserAdapter) findChildByType(
	node *valueobject.ParseNode,
	targetType string,
) *valueobject.ParseNode {
	if node == nil {
		return nil
	}

	for _, child := range node.Children {
		if child != nil && child.Type == targetType {
			return child
		}
	}
	return nil
}

// extractGoPackageName extracts the package name from the parse tree.
func (s *SemanticTraverserAdapter) extractGoPackageName(parseTree *valueobject.ParseTree) string {
	packageName := "main" // default fallback

	// Look for package declaration in the root node
	s.traverseGoAST(parseTree.RootNode(), func(node *valueobject.ParseNode) {
		if node.Type == "package_clause" {
			for _, child := range node.Children {
				if child != nil && child.Type == "package_identifier" {
					packageName = parseTree.GetNodeText(child)
					return
				}
			}
		}
	})

	return packageName
}

func (s *SemanticTraverserAdapter) extractJavaScriptFunctions(
	parseTree *valueobject.ParseTree,
) []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Debug log at the start showing root node info
	slogger.Info(context.Background(), "Starting JavaScript function extraction", slogger.Fields{
		"root_node_type": parseTree.RootNode().Type,
		"children_count": len(parseTree.RootNode().Children),
	})

	// Traverse the AST for JavaScript function nodes
	s.traverseJavaScriptAST(parseTree.RootNode(), func(node *valueobject.ParseNode) {
		switch node.Type {
		case "function_declaration",
			nodeFunctionExpression,
			nodeArrowFunction,
			"method_definition",
			"generator_function_declaration":
			// Extract function name
			name := s.extractJavaScriptFunctionName(node, parseTree)
			slogger.Debug(context.Background(), "Found function node, attempting name extraction", slogger.Fields{
				"node_type":      node.Type,
				"extracted_name": name,
			})

			// Generate name for anonymous functions
			if name == "" {
				switch node.Type {
				case nodeFunctionExpression:
					name = "(anonymous function)"
				case nodeArrowFunction:
					name = "(arrow function)"
				case "method_definition":
					name = "(method)"
				default:
					name = "(unnamed)"
				}
			}

			// Always extract if we have a name (including generated ones)
			content := parseTree.GetNodeText(node)
			hash := s.simpleHash(content)

			chunk := outbound.SemanticCodeChunk{
				ChunkID:       fmt.Sprintf("func_%s_%d", name, hash),
				Name:          name,
				QualifiedName: name,
				Language:      parseTree.Language(),
				Type:          outbound.ConstructFunction,
				Content:       content,
				StartByte:     node.StartByte,
				EndByte:       node.EndByte,
				StartPosition: node.StartPos,
				EndPosition:   node.EndPos,
				ExtractedAt:   time.Now(),
				Hash:          strconv.FormatUint(uint64(hash), 10),
			}
			chunks = append(chunks, chunk)
			slogger.Info(context.Background(), "Successfully extracted function chunk", slogger.Fields{
				"function_name": name,
				"node_type":     node.Type,
			})
		case nodeVariableDeclarator:
			// Check if this variable declarator is for a function expression
			if s.isJavaScriptFunctionExpression(node) {
				name := s.extractJavaScriptVariableName(node, parseTree)

				// Generate name for anonymous functions
				if name == "" {
					name = "(anonymous variable function)"
				}

				// Always extract if we have a name (including generated ones)
				content := parseTree.GetNodeText(node)
				hash := s.simpleHash(content)

				chunk := outbound.SemanticCodeChunk{
					ChunkID:       fmt.Sprintf("var_%s_%d", name, hash),
					Name:          name,
					QualifiedName: name,
					Language:      parseTree.Language(),
					Type:          outbound.ConstructFunction,
					Content:       content,
					StartByte:     node.StartByte,
					EndByte:       node.EndByte,
					StartPosition: node.StartPos,
					EndPosition:   node.EndPos,
					ExtractedAt:   time.Now(),
					Hash:          strconv.FormatUint(uint64(hash), 10),
				}
				chunks = append(chunks, chunk)
			}
		}
	})

	// Debug log for total chunks found
	slogger.Info(context.Background(), "JavaScript function extraction completed", slogger.Fields{
		"total_chunks_found": len(chunks),
	})

	return chunks
}

// traverseJavaScriptAST recursively traverses the JavaScript AST.
func (s *SemanticTraverserAdapter) traverseJavaScriptAST(
	node *valueobject.ParseNode,
	visitFunc func(*valueobject.ParseNode),
) {
	slogger.Debug(context.Background(), "traverseJavaScriptAST called", slogger.Fields{
		"node_type":      node.Type,
		"children_count": len(node.Children),
	})

	visitFunc(node)

	for _, child := range node.Children {
		if child != nil {
			s.traverseJavaScriptAST(child, visitFunc)
		}
	}
}

// extractJavaScriptFunctionName extracts the name of a JavaScript function.
func (s *SemanticTraverserAdapter) extractJavaScriptFunctionName(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) string {
	// Look for the function name in the children
	for _, child := range node.Children {
		if child != nil && child.Type == "identifier" {
			name := parseTree.GetNodeText(child)
			return name
		}
	}
	return ""
}

// extractJavaScriptVariableName extracts the name of a JavaScript variable.
func (s *SemanticTraverserAdapter) extractJavaScriptVariableName(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) string {
	if node.Type != nodeVariableDeclarator {
		return ""
	}

	// The first child should be the identifier (variable name)
	if len(node.Children) > 0 && node.Children[0].Type == "identifier" {
		name := parseTree.GetNodeText(node.Children[0])
		return name
	}

	return ""
}

// isJavaScriptFunctionExpression checks if a variable declarator contains a function expression.
func (s *SemanticTraverserAdapter) isJavaScriptFunctionExpression(node *valueobject.ParseNode) bool {
	if node.Type != nodeVariableDeclarator {
		return false
	}

	// Look for function expression in the children
	for _, child := range node.Children {
		if child != nil && (child.Type == "function_expression" || child.Type == "arrow_function") {
			return true
		}
	}

	return false
}

// simpleHash generates a simple hash for content.
func (s *SemanticTraverserAdapter) simpleHash(content string) uint32 {
	h := fnv.New32a()
	if _, err := h.Write([]byte(content)); err != nil {
		// In the unlikely event of an error, return zero to avoid panics
		return 0
	}
	return h.Sum32()
}

// extractJavaScriptImports implements JavaScript import extraction for the GREEN phase.
func (s *SemanticTraverserAdapter) extractJavaScriptImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	content := string(parseTree.Source())
	now := time.Now()

	// Multi-line ES6 import pattern - handles imports across multiple lines
	importPattern := regexp.MustCompile(
		`(?s)import\s+(?:(?:\*\s+as\s+(\w+))|(?:\{([^}]+)\})|(\w+))\s+from\s+['"]([^'"]+)['"]`,
	)

	// Mixed imports pattern (default + named)
	mixedImportPattern := regexp.MustCompile(`(?s)import\s+(\w+),\s*\{([^}]+)\}\s+from\s+['"]([^'"]+)['"]`)

	// Import assertion pattern
	assertionPattern := regexp.MustCompile(
		`(?s)import\s+(?:(?:\*\s+as\s+(\w+))|(?:\{([^}]+)\})|(\w+))\s+from\s+['"]([^'"]+)['"]\s+assert\s*\{\s*type:\s*['"](\w+)['"]`,
	)

	// Process import assertions first
	assertionMatches := assertionPattern.FindAllStringSubmatch(content, -1)
	for _, match := range assertionMatches {
		var importedSymbols []string
		var alias string
		var isWildcard bool
		var aliases map[string]string

		namespaceImport := match[1]
		namedImports := match[2]
		defaultImport := match[3]
		path := match[4]
		assertionType := match[5]

		metadata := map[string]interface{}{
			"import_type":       "es6",
			"has_assertions":    true,
			"assertion_type":    assertionType,
			"import_style":      "default",
			"path_type":         getPathType(path),
			"relative_depth":    getRelativeDepth(path),
			"is_scoped_package": strings.HasPrefix(path, "@"),
		}

		switch {
		case namespaceImport != "":
			alias = namespaceImport
			isWildcard = true
			importedSymbols = append(importedSymbols, "*")
			metadata["import_style"] = "namespace"
		case namedImports != "":
			symbols := parseNamedImports(namedImports)
			importedSymbols = append(importedSymbols, symbols.Names...)
			aliases = symbols.Aliases
			metadata["aliases"] = aliases
			metadata["import_style"] = "named"
		case defaultImport != "":
			alias = defaultImport
			importedSymbols = append(importedSymbols, defaultImport)
			metadata["import_style"] = "default"
		}

		startIndex := strings.Index(content, match[0])
		endIndex := startIndex + len(match[0])
		hash := generateHash(match[0])

		imports = append(imports, outbound.ImportDeclaration{
			Path:            path,
			Alias:           alias,
			IsWildcard:      isWildcard,
			ImportedSymbols: importedSymbols,
			StartByte:       valueobject.ClampToUint32(startIndex),
			EndByte:         valueobject.ClampToUint32(endIndex),
			StartPosition:   valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(startIndex)},
			EndPosition:     valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(endIndex)},
			Content:         match[0],
			ExtractedAt:     now,
			Hash:            hash,
			Metadata:        metadata,
		})
	}

	// Process mixed imports next (default + named)
	mixedMatches := mixedImportPattern.FindAllStringSubmatch(content, -1)
	for _, match := range mixedMatches {
		defaultImport := match[1]
		namedImports := match[2]
		path := match[3]

		var importedSymbols []string
		aliases := make(map[string]string)

		// Add default import
		importedSymbols = append(importedSymbols, defaultImport)
		aliases[defaultImport] = defaultImport

		// Process named imports
		namedParts := strings.Split(namedImports, ",")
		for _, part := range namedParts {
			part = strings.TrimSpace(part)
			if aliasMatch := regexp.MustCompile(`(\w+)\s+as\s+(\w+)`).FindStringSubmatch(part); aliasMatch != nil {
				importedSymbols = append(importedSymbols, aliasMatch[1])
				aliases[aliasMatch[1]] = aliasMatch[2]
			} else {
				importedSymbols = append(importedSymbols, part)
				aliases[part] = part
			}
		}

		startInt := strings.Index(content, match[0])
		startByte := valueobject.ClampToUint32(startInt)
		endByte := valueobject.ClampToUint32(startInt + len(match[0]))
		hash := sha256.Sum256([]byte(match[0]))
		hashStr := hex.EncodeToString(hash[:])

		metadata := map[string]interface{}{
			"aliases":         aliases,
			"import_style":    "mixed",
			"import_type":     "es6",
			"path_type":       s.determinePathType(path),
			"is_mixed_import": true,
		}

		imports = append(imports, outbound.ImportDeclaration{
			Path:            path,
			Alias:           defaultImport,
			ImportedSymbols: importedSymbols,
			StartByte:       startByte,
			EndByte:         endByte,
			StartPosition:   valueobject.Position{Row: 0, Column: 0},
			EndPosition:     valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(len(match[0]))},
			Content:         match[0],
			ExtractedAt:     now,
			Hash:            hashStr,
			Metadata:        metadata,
		})
	}

	// Process regular ES6 imports (but skip ones already processed as mixed)
	importMatches := importPattern.FindAllStringSubmatch(content, -1)
	for _, match := range importMatches {
		// Skip if this was already processed as a mixed import
		if mixedImportPattern.MatchString(match[0]) {
			continue
		}

		path := match[4]
		var importedSymbols []string
		var importStyle string
		var alias string
		aliases := make(map[string]string)

		switch {
		case match[1] != "":
			// import * as name from 'path'
			importedSymbols = []string{"*"}
			aliases["*"] = match[1]
			importStyle = "namespace"
		case match[2] != "":
			// import { name1, name2 as alias } from 'path' (multi-line support)
			namedImports := match[2]
			// Clean up the named imports string - remove newlines and extra spaces
			namedImports = regexp.MustCompile(`\s+`).ReplaceAllString(namedImports, " ")
			namedImports = strings.TrimSpace(namedImports)

			namedParts := strings.Split(namedImports, ",")
			for _, part := range namedParts {
				part = strings.TrimSpace(part)
				if aliasMatch := regexp.MustCompile(`(\w+)\s+as\s+(\w+)`).FindStringSubmatch(part); aliasMatch != nil {
					importedSymbols = append(importedSymbols, aliasMatch[1])
					aliases[aliasMatch[1]] = aliasMatch[2]
				} else if part != "" {
					importedSymbols = append(importedSymbols, part)
					aliases[part] = part
				}
			}
			importStyle = "named"
		case match[3] != "":
			// import name from 'path'
			importedSymbols = []string{match[3]}
			alias = match[3]
			aliases[match[3]] = match[3]
			importStyle = "default"
		}

		startInt := strings.Index(content, match[0])
		startByte := valueobject.ClampToUint32(startInt)
		endByte := valueobject.ClampToUint32(startInt + len(match[0]))
		hash := sha256.Sum256([]byte(match[0]))
		hashStr := hex.EncodeToString(hash[:])

		metadata := map[string]interface{}{
			"aliases":      aliases,
			"import_style": importStyle,
			"import_type":  "es6",
			"path_type":    s.determinePathType(path),
		}

		imports = append(imports, outbound.ImportDeclaration{
			Path:            path,
			Alias:           alias,
			ImportedSymbols: importedSymbols,
			StartByte:       startByte,
			EndByte:         endByte,
			StartPosition:   valueobject.Position{Row: 0, Column: 0},
			EndPosition:     valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(len(match[0]))},
			Content:         match[0],
			ExtractedAt:     now,
			Hash:            hashStr,
			Metadata:        metadata,
		})
	}

	slogger.Info(ctx, "JavaScript import extraction completed", slogger.Fields{
		"extracted_count": len(imports),
	})

	return imports
}

// determinePathType categorizes import paths.
func (s *SemanticTraverserAdapter) determinePathType(path string) string {
	if strings.HasPrefix(path, "@") {
		return "scoped-npm"
	}
	if strings.HasPrefix(path, ".") {
		return "relative"
	}
	if strings.HasPrefix(path, "/") {
		return "absolute"
	}
	if strings.Contains(path, "://") {
		return "protocol"
	}
	return "npm"
}

// Helper functions for JavaScript import extraction

type NamedImportSymbols struct {
	Names   []string
	Aliases map[string]string
}

func parseNamedImports(importClause string) NamedImportSymbols {
	symbols := NamedImportSymbols{
		Names:   []string{},
		Aliases: make(map[string]string),
	}

	parts := strings.Split(importClause, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "as") {
			aliasParts := strings.Split(part, "as")
			if len(aliasParts) == 2 {
				name := strings.TrimSpace(aliasParts[0])
				alias := strings.TrimSpace(aliasParts[1])
				symbols.Names = append(symbols.Names, alias)
				symbols.Aliases[alias] = name
			}
		} else {
			symbols.Names = append(symbols.Names, part)
		}
	}

	return symbols
}

func getPathType(path string) string {
	if strings.HasPrefix(path, "@") {
		return "scoped-npm"
	}
	if strings.HasPrefix(path, ".") {
		return "relative"
	}
	if strings.HasPrefix(path, "/") {
		return "absolute"
	}
	if strings.Contains(path, "://") {
		return "protocol"
	}
	return "npm"
}

func getRelativeDepth(path string) int {
	if !strings.HasPrefix(path, ".") {
		return 0
	}

	depth := 0
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == ".." {
			depth++
		}
	}
	return depth
}

func generateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// TreeSitterParseTreeConverter converts tree-sitter parse trees to domain objects.
type TreeSitterParseTreeConverter struct{}

// NewTreeSitterParseTreeConverter creates a new TreeSitterParseTreeConverter.
func NewTreeSitterParseTreeConverter() *TreeSitterParseTreeConverter {
	return &TreeSitterParseTreeConverter{}
}

// ConvertToDomain converts a tree-sitter parse tree to a domain parse tree.
func (c *TreeSitterParseTreeConverter) ConvertToDomain(tree interface{}) (*valueobject.ParseTree, error) {
	// For GREEN phase, implement minimal conversion
	// In real implementation, this would convert from tree-sitter tree to our domain ParseTree
	if tree == nil {
		return nil, errors.New("tree cannot be nil")
	}

	// Handle ParseResult from stub parser
	if parseResult, ok := tree.(*ParseResult); ok {
		if parseResult.ParseTree == nil {
			return nil, errors.New("parse result contains nil parse tree")
		}
		return c.convertTreeSitterTreeToDomain(parseResult.ParseTree)
	}

	// Handle direct ParseTree from treesitter package
	if parseTree, ok := tree.(*ParseTree); ok {
		return c.convertTreeSitterTreeToDomain(parseTree)
	}

	// For GREEN phase: assume we already have a domain ParseTree
	// This is a minimal implementation to make tests pass
	if domainTree, ok := tree.(*valueobject.ParseTree); ok {
		return domainTree, nil
	}

	// For other cases, return an error for now
	return nil, fmt.Errorf("unsupported tree type: %T", tree)
}

// convertTreeSitterTreeToDomain converts treesitter.ParseTree to valueobject.ParseTree.
func (c *TreeSitterParseTreeConverter) convertTreeSitterTreeToDomain(
	tsTree *ParseTree,
) (*valueobject.ParseTree, error) {
	if tsTree == nil {
		return nil, errors.New("treesitter parse tree cannot be nil")
	}

	// Convert language string to valueobject.Language
	language, err := valueobject.NewLanguage(tsTree.Language)
	if err != nil {
		return nil, fmt.Errorf("invalid language %s: %w", tsTree.Language, err)
	}

	// Convert root node
	domainRootNode, err := c.convertParseNode(tsTree.RootNode)
	if err != nil {
		return nil, fmt.Errorf("failed to convert root node: %w", err)
	}

	// For GREEN phase: create a simple domain parse tree with proper constructor
	ctx := context.Background()
	metadata := valueobject.ParseMetadata{
		// Use empty struct for GREEN phase - let constructor set defaults
	}
	domainTree, err := valueobject.NewParseTree(ctx, language, domainRootNode, []byte(tsTree.Source), metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create domain parse tree: %w", err)
	}

	return domainTree, nil
}

// convertParseNode converts treesitter.ParseNode to valueobject.ParseNode.
func (c *TreeSitterParseTreeConverter) convertParseNode(tsNode *ParseNode) (*valueobject.ParseNode, error) {
	if tsNode == nil {
		return nil, errors.New("parse node is nil")
	}

	// Convert position structs
	startPos := valueobject.Position{
		Row:    tsNode.StartPoint.Row,
		Column: tsNode.StartPoint.Column,
	}
	endPos := valueobject.Position{
		Row:    tsNode.EndPoint.Row,
		Column: tsNode.EndPoint.Column,
	}

	// Convert children recursively
	var children []*valueobject.ParseNode
	for _, childNode := range tsNode.Children {
		domainChild, err := c.convertParseNode(childNode)
		if err != nil {
			return nil, fmt.Errorf("failed to convert child node: %w", err)
		}
		if domainChild != nil {
			children = append(children, domainChild)
		}
	}

	// For GREEN phase: create a simple domain parse node using available constructor
	// Use just the essential fields that should be available
	domainNode := &valueobject.ParseNode{
		Type:      tsNode.Type,
		StartByte: tsNode.StartByte,
		EndByte:   tsNode.EndByte,
		StartPos:  startPos,
		EndPos:    endPos,
		Children:  children,
	}

	return domainNode, nil
}

// SemanticCodeChunkExtractor extracts semantic code chunks from domain parse trees.
type SemanticCodeChunkExtractor struct{}

// NewSemanticCodeChunkExtractor creates a new SemanticCodeChunkExtractor.
func NewSemanticCodeChunkExtractor() *SemanticCodeChunkExtractor {
	return &SemanticCodeChunkExtractor{}
}

// Extract extracts semantic code chunks from a domain parse tree.
func (e *SemanticCodeChunkExtractor) Extract(
	ctx context.Context,
	domainTree *valueobject.ParseTree,
) ([]outbound.SemanticCodeChunk, error) {
	if domainTree == nil {
		return []outbound.SemanticCodeChunk{}, nil
	}

	// For GREEN phase: use existing semantic traverser adapter
	adapter := NewSemanticTraverserAdapter()

	// Extract all types of chunks using default options
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeComments:      false,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		IncludeDependencies:  false,
		IncludeMetadata:      false,
		MaxDepth:             100,
	}

	var allChunks []outbound.SemanticCodeChunk

	// Extract functions
	if functions, err := adapter.ExtractFunctions(ctx, domainTree, options); err == nil {
		allChunks = append(allChunks, functions...)
	}

	// Extract classes
	if classes, err := adapter.ExtractClasses(ctx, domainTree, options); err == nil {
		allChunks = append(allChunks, classes...)
	}

	// Extract interfaces
	if interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options); err == nil {
		allChunks = append(allChunks, interfaces...)
	}

	// Extract variables
	if variables, err := adapter.ExtractVariables(ctx, domainTree, options); err == nil {
		allChunks = append(allChunks, variables...)
	}

	// Extract modules
	if modules, err := adapter.ExtractModules(ctx, domainTree, options); err == nil {
		allChunks = append(allChunks, modules...)
	}

	return allChunks, nil
}

// tryParserFactory attempts to use the parser factory and returns the result if successful.
// This reduces nestif complexity by extracting the nested logic.
func (s *SemanticTraverserAdapter) tryParserFactory(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	langParserExtractor func(LanguageParser) ([]outbound.SemanticCodeChunk, error),
) ([]outbound.SemanticCodeChunk, error) {
	if s.parserFactory == nil {
		return nil, nil
	}

	slogger.Info(ctx, "Attempting to create parser from factory", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	parser, err := s.parserFactory.CreateParser(ctx, parseTree.Language())
	if err != nil {
		slogger.Warn(ctx, "Failed to create parser from factory, falling back to legacy implementation", slogger.Fields{
			"error": err.Error(),
		})
		return nil, err
	}

	slogger.Info(ctx, "Parser created successfully", slogger.Fields{
		"parser_type": fmt.Sprintf("%T", parser),
	})

	// Cast to LanguageParser interface if possible
	langParser, ok := parser.(LanguageParser)
	if !ok {
		slogger.Warn(
			ctx,
			"Parser does not implement LanguageParser interface, falling back to legacy implementation",
			slogger.Fields{
				"parser_type": fmt.Sprintf("%T", parser),
			},
		)
		return nil, nil
	}

	slogger.Info(ctx, "Parser cast to LanguageParser successful, calling extractor", slogger.Fields{})
	result, err := langParserExtractor(langParser)
	if err != nil {
		slogger.Warn(ctx, "LanguageParser extractor failed, falling back to legacy implementation", slogger.Fields{
			"error": err.Error(),
		})
		return nil, err
	}

	return result, nil
}
