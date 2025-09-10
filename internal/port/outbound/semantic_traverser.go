package outbound

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"time"
)

// SemanticTraverser defines the interface for traversing parse trees and extracting semantic code chunks.
// This port interface enables semantic code analysis and chunking for various programming languages.
type SemanticTraverser interface {
	// ExtractFunctions extracts all function definitions from a parse tree
	ExtractFunctions(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options SemanticExtractionOptions,
	) ([]SemanticCodeChunk, error)

	// ExtractClasses extracts all class/struct definitions from a parse tree
	ExtractClasses(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options SemanticExtractionOptions,
	) ([]SemanticCodeChunk, error)

	// ExtractModules extracts module/package level constructs from a parse tree
	ExtractModules(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options SemanticExtractionOptions,
	) ([]SemanticCodeChunk, error)

	// ExtractInterfaces extracts interface definitions (Go, TypeScript, etc.)
	ExtractInterfaces(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options SemanticExtractionOptions,
	) ([]SemanticCodeChunk, error)

	// ExtractVariables extracts variable and constant declarations
	ExtractVariables(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options SemanticExtractionOptions,
	) ([]SemanticCodeChunk, error)

	// ExtractComments extracts documentation and comments with context
	ExtractComments(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options SemanticExtractionOptions,
	) ([]SemanticCodeChunk, error)

	// ExtractImports extracts import/include statements with dependency information
	ExtractImports(
		ctx context.Context,
		parseTree *valueobject.ParseTree,
		options SemanticExtractionOptions,
	) ([]ImportDeclaration, error)

	// GetSupportedConstructTypes returns the semantic constructs this traverser can extract
	GetSupportedConstructTypes(ctx context.Context, language valueobject.Language) ([]SemanticConstructType, error)
}

// SemanticCodeChunk represents a semantic unit of code extracted from a parse tree.
type SemanticCodeChunk struct {
	ChunkID           string                 `json:"id"`
	Type              SemanticConstructType  `json:"type"`
	Name              string                 `json:"name"`
	QualifiedName     string                 `json:"qualified_name"`
	Language          valueobject.Language   `json:"language"`
	StartByte         uint32                 `json:"start_byte"`
	EndByte           uint32                 `json:"end_byte"`
	StartPosition     valueobject.Position   `json:"start_position"`
	EndPosition       valueobject.Position   `json:"end_position"`
	Content           string                 `json:"content"`
	Signature         string                 `json:"signature,omitempty"`
	Documentation     string                 `json:"documentation,omitempty"`
	Annotations       []Annotation           `json:"annotations,omitempty"`
	Parameters        []Parameter            `json:"parameters,omitempty"`
	ReturnType        string                 `json:"return_type,omitempty"`
	Visibility        VisibilityModifier     `json:"visibility"`
	IsStatic          bool                   `json:"is_static"`
	IsAsync           bool                   `json:"is_async"`
	IsAbstract        bool                   `json:"is_abstract"`
	IsGeneric         bool                   `json:"is_generic"`
	GenericParameters []GenericParameter     `json:"generic_parameters,omitempty"`
	Dependencies      []DependencyReference  `json:"dependencies,omitempty"`
	UsedTypes         []TypeReference        `json:"used_types,omitempty"`
	CalledFunctions   []FunctionCall         `json:"called_functions,omitempty"`
	ChildChunks       []SemanticCodeChunk    `json:"child_chunks,omitempty"`
	ParentChunk       *SemanticCodeChunk     `json:"parent_chunk,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ExtractedAt       time.Time              `json:"extracted_at"`
	Hash              string                 `json:"hash"`
}

// ID returns the unique identifier for this semantic code chunk.
func (s SemanticCodeChunk) ID() string {
	return s.ChunkID
}

// SemanticConstructType represents the type of semantic construct.
type SemanticConstructType string

const (
	ConstructFunction      SemanticConstructType = "function"
	ConstructMethod        SemanticConstructType = "method"
	ConstructClass         SemanticConstructType = "class"
	ConstructStruct        SemanticConstructType = "struct"
	ConstructInterface     SemanticConstructType = "interface"
	ConstructEnum          SemanticConstructType = "enum"
	ConstructVariable      SemanticConstructType = "variable"
	ConstructConstant      SemanticConstructType = "constant"
	ConstructField         SemanticConstructType = "field"
	ConstructProperty      SemanticConstructType = "property"
	ConstructModule        SemanticConstructType = "module"
	ConstructPackage       SemanticConstructType = "package"
	ConstructNamespace     SemanticConstructType = "namespace"
	ConstructType          SemanticConstructType = "type"
	ConstructComment       SemanticConstructType = "comment"
	ConstructDecorator     SemanticConstructType = "decorator"
	ConstructAttribute     SemanticConstructType = "attribute"
	ConstructLambda        SemanticConstructType = "lambda"
	ConstructClosure       SemanticConstructType = "closure"
	ConstructGenerator     SemanticConstructType = "generator"
	ConstructAsyncFunction SemanticConstructType = "async_function"
)

// SemanticExtractionOptions configures semantic extraction behavior.
type SemanticExtractionOptions struct {
	IncludePrivate       bool                        `json:"include_private"`
	IncludeComments      bool                        `json:"include_comments"`
	IncludeDocumentation bool                        `json:"include_documentation"`
	IncludeTypeInfo      bool                        `json:"include_type_info"`
	IncludeDependencies  bool                        `json:"include_dependencies"`
	IncludeMetadata      bool                        `json:"include_metadata"`
	MaxDepth             int                         `json:"max_depth"`
	FilterTypes          []SemanticConstructType     `json:"filter_types,omitempty"`
	NamePatterns         []string                    `json:"name_patterns,omitempty"`
	ExcludePatterns      []string                    `json:"exclude_patterns,omitempty"`
	PreservationStrategy ContextPreservationStrategy `json:"preservation_strategy"`
	ChunkingStrategy     ChunkingStrategy            `json:"chunking_strategy"`
	PerformanceHints     *ExtractionPerformanceHints `json:"performance_hints,omitempty"`
	CustomOptions        map[string]interface{}      `json:"custom_options,omitempty"`
}

// ImportDeclaration represents an import/include statement.
type ImportDeclaration struct {
	Path            string                 `json:"path"`
	Alias           string                 `json:"alias,omitempty"`
	IsWildcard      bool                   `json:"is_wildcard"`
	ImportedSymbols []string               `json:"imported_symbols,omitempty"`
	StartByte       uint32                 `json:"start_byte"`
	EndByte         uint32                 `json:"end_byte"`
	StartPosition   valueobject.Position   `json:"start_position"`
	EndPosition     valueobject.Position   `json:"end_position"`
	Content         string                 `json:"content"`
	ExtractedAt     time.Time              `json:"extracted_at"`
	Hash            string                 `json:"hash"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// Supporting types for semantic analysis

type VisibilityModifier string

const (
	Public    VisibilityModifier = "public"
	Private   VisibilityModifier = "private"
	Protected VisibilityModifier = "protected"
	Internal  VisibilityModifier = "internal"
)

type Annotation struct {
	Name      string                 `json:"name"`
	Arguments []string               `json:"arguments,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type Parameter struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	DefaultValue string `json:"default_value,omitempty"`
	IsOptional   bool   `json:"is_optional"`
	IsVariadic   bool   `json:"is_variadic"`
}

type GenericParameter struct {
	Name        string   `json:"name"`
	Constraints []string `json:"constraints,omitempty"`
}

type DependencyReference struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
}

type TypeReference struct {
	Name          string   `json:"name"`
	QualifiedName string   `json:"qualified_name"`
	IsGeneric     bool     `json:"is_generic"`
	GenericArgs   []string `json:"generic_args,omitempty"`
}

type FunctionCall struct {
	Name      string   `json:"name"`
	Arguments []string `json:"arguments,omitempty"`
	StartByte uint32   `json:"start_byte"`
	EndByte   uint32   `json:"end_byte"`
}

type ContextPreservationStrategy string

const (
	PreserveMinimal  ContextPreservationStrategy = "minimal"
	PreserveModerate ContextPreservationStrategy = "moderate"
	PreserveMaximal  ContextPreservationStrategy = "maximal"
)

type ChunkingStrategy string

const (
	ChunkByFunction ChunkingStrategy = "function"
	ChunkByClass    ChunkingStrategy = "class"
	ChunkByModule   ChunkingStrategy = "module"
	ChunkByFile     ChunkingStrategy = "file"
)

type ExtractionPerformanceHints struct {
	MaxChunkSize       int  `json:"max_chunk_size,omitempty"`
	EnableCaching      bool `json:"enable_caching"`
	ParallelProcessing bool `json:"parallel_processing"`
}
