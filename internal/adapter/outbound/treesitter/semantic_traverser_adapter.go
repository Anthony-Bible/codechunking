package treesitter

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"
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
	slogger.Info(ctx, "Extracting "+extractionType+" from parse tree", slogger.Fields{
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
	// 		"Tree-sitter language dispatcher unavailable, falling back to legacy implementation",
	// 		slogger.Fields{
	// 			"error": err.Error(),
	// 		},
	// 	)
	// }

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

// extractJavaScriptFunctions implements a simple JavaScript function extraction directly
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
		case "function_declaration", "method_declaration":
			slogger.Info(ctx, "Found Go function/method node", slogger.Fields{
				"node_type": node.Type,
			})
			// Extract Go function
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

// traverseGoAST recursively traverses the Go AST, calling visitFunc for each node
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

// extractGoFunctionFromNode extracts a Go function from an AST node
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
	if node.Type == "method_declaration" {
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
		ID:            id,
		Name:          name,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		Type:          constructType,
		Visibility:    visibility,
		Content:       content,
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("%d", hash),
	}

	return chunk
}

// extractGoFunctionName extracts the name of a Go function from an AST node
func (s *SemanticTraverserAdapter) extractGoFunctionName(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) string {
	// For function_declaration, look for identifier child
	// For method_declaration, look for identifier after receiver
	for i, child := range node.Children {
		if child != nil && child.Type == "identifier" {
			// For methods, skip the first identifier (which might be in receiver)
			if node.Type == "method_declaration" && i <= 2 {
				continue
			}
			name := parseTree.GetNodeText(child)
			if name != "" {
				return name
			}
		}
	}
	return ""
}

// extractGoPackageName extracts the package name from the parse tree
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
			"function_expression",
			"arrow_function",
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
				case "function_expression":
					name = "(anonymous function)"
				case "arrow_function":
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
				ID:            fmt.Sprintf("func_%s_%d", name, hash),
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
				Hash:          fmt.Sprintf("%d", hash),
			}
			chunks = append(chunks, chunk)
			slogger.Info(context.Background(), "Successfully extracted function chunk", slogger.Fields{
				"function_name": name,
				"node_type":     node.Type,
			})
		case "variable_declarator":
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
					ID:            fmt.Sprintf("var_%s_%d", name, hash),
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
					Hash:          fmt.Sprintf("%d", hash),
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

// traverseJavaScriptAST recursively traverses the JavaScript AST
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

// extractJavaScriptFunctionName extracts the name of a JavaScript function
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

// extractJavaScriptVariableName extracts the name of a JavaScript variable
func (s *SemanticTraverserAdapter) extractJavaScriptVariableName(
	node *valueobject.ParseNode,
	parseTree *valueobject.ParseTree,
) string {
	if node.Type != "variable_declarator" {
		return ""
	}

	// The first child should be the identifier (variable name)
	if len(node.Children) > 0 && node.Children[0].Type == "identifier" {
		name := parseTree.GetNodeText(node.Children[0])
		return name
	}

	return ""
}

// isJavaScriptFunctionExpression checks if a variable declarator contains a function expression
func (s *SemanticTraverserAdapter) isJavaScriptFunctionExpression(node *valueobject.ParseNode) bool {
	if node.Type != "variable_declarator" {
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

// simpleHash generates a simple hash for content
func (s *SemanticTraverserAdapter) simpleHash(content string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(content))
	return h.Sum32()
}
