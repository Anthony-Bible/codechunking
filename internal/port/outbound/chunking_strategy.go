package outbound

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"time"
)

// CodeChunkingStrategy defines the interface for intelligent code chunking strategies.
// This port interface enables language-agnostic chunking of semantic code constructs
// with configurable context preservation and boundary detection.
type CodeChunkingStrategy interface {
	// ChunkCode takes semantic chunks and applies the chunking strategy to create
	// optimized code chunks with preserved context and intelligent boundaries
	ChunkCode(
		ctx context.Context,
		semanticChunks []SemanticCodeChunk,
		config ChunkingConfiguration,
	) ([]EnhancedCodeChunk, error)

	// GetOptimalChunkSize calculates the optimal chunk size based on the content
	// and language characteristics
	GetOptimalChunkSize(
		ctx context.Context,
		language valueobject.Language,
		contentSize int,
	) (ChunkSizeRecommendation, error)

	// ValidateChunkBoundaries ensures chunk boundaries preserve semantic integrity
	ValidateChunkBoundaries(
		ctx context.Context,
		chunks []EnhancedCodeChunk,
	) ([]ChunkValidationResult, error)

	// GetSupportedStrategies returns the chunking strategies supported by this implementation
	GetSupportedStrategies(ctx context.Context) ([]ChunkingStrategyType, error)

	// PreserveContext applies context preservation strategies to maintain
	// semantic relationships between chunks
	PreserveContext(
		ctx context.Context,
		chunks []EnhancedCodeChunk,
		strategy ContextPreservationStrategy,
	) ([]EnhancedCodeChunk, error)
}

// EnhancedCodeChunk represents an optimized chunk of code with preserved context and boundaries.
// This extends the basic CodeChunk with semantic analysis and chunking strategy information.
type EnhancedCodeChunk struct {
	ID                 string                      `json:"id"`
	SourceFile         string                      `json:"source_file"`
	Language           valueobject.Language        `json:"language"`
	StartByte          uint32                      `json:"start_byte"`
	EndByte            uint32                      `json:"end_byte"`
	StartPosition      valueobject.Position        `json:"start_position"`
	EndPosition        valueobject.Position        `json:"end_position"`
	Content            string                      `json:"content"`
	PreservedContext   PreservedContext            `json:"preserved_context"`
	SemanticConstructs []SemanticCodeChunk         `json:"semantic_constructs"`
	Dependencies       []ChunkDependency           `json:"dependencies"`
	ChunkType          ChunkType                   `json:"chunk_type"`
	ChunkingStrategy   ChunkingStrategyType        `json:"chunking_strategy"`
	ContextStrategy    ContextPreservationStrategy `json:"context_strategy"`
	QualityMetrics     ChunkQualityMetrics         `json:"quality_metrics"`
	ParentChunk        *CodeChunk                  `json:"parent_chunk,omitempty"`
	ChildChunks        []CodeChunk                 `json:"child_chunks,omitempty"`
	RelatedChunks      []ChunkRelation             `json:"related_chunks,omitempty"`
	Metadata           map[string]interface{}      `json:"metadata,omitempty"`
	CreatedAt          time.Time                   `json:"created_at"`
	Hash               string                      `json:"hash"`
	Size               ChunkSize                   `json:"size"`
	ComplexityScore    float64                     `json:"complexity_score"`
	CohesionScore      float64                     `json:"cohesion_score"`
}

// ChunkingConfiguration defines the configuration for chunking strategies.
type ChunkingConfiguration struct {
	Strategy                ChunkingStrategyType        `json:"strategy"`
	ContextPreservation     ContextPreservationStrategy `json:"context_preservation"`
	MaxChunkSize            int                         `json:"max_chunk_size"`
	MinChunkSize            int                         `json:"min_chunk_size"`
	OverlapSize             int                         `json:"overlap_size"`
	PreferredBoundaries     []BoundaryType              `json:"preferred_boundaries"`
	IncludeImports          bool                        `json:"include_imports"`
	IncludeDocumentation    bool                        `json:"include_documentation"`
	IncludeComments         bool                        `json:"include_comments"`
	PreserveDependencies    bool                        `json:"preserve_dependencies"`
	EnableSplitting         bool                        `json:"enable_splitting"`
	QualityThreshold        float64                     `json:"quality_threshold"`
	PerformanceHints        *ChunkingPerformanceHints   `json:"performance_hints,omitempty"`
	LanguageSpecificOptions map[string]interface{}      `json:"language_specific_options,omitempty"`
}

// ChunkingStrategyType defines the available chunking strategies.
type ChunkingStrategyType string

const (
	StrategyFunction  ChunkingStrategyType = "function"
	StrategyClass     ChunkingStrategyType = "class"
	StrategyModule    ChunkingStrategyType = "module"
	StrategyFile      ChunkingStrategyType = "file"
	StrategySizeBased ChunkingStrategyType = "size_based"
	StrategyHybrid    ChunkingStrategyType = "hybrid"
	StrategyAdaptive  ChunkingStrategyType = "adaptive"
	StrategyHierarchy ChunkingStrategyType = "hierarchy"
)

// ChunkType defines the type of content in a chunk.
type ChunkType string

const (
	ChunkFunction      ChunkType = "function"
	ChunkClass         ChunkType = "class"
	ChunkModule        ChunkType = "module"
	ChunkInterface     ChunkType = "interface"
	ChunkStruct        ChunkType = "struct"
	ChunkEnum          ChunkType = "enum"
	ChunkVariable      ChunkType = "variable"
	ChunkConstant      ChunkType = "constant"
	ChunkMixed         ChunkType = "mixed"
	ChunkFragment      ChunkType = "fragment"
	ChunkDocumentation ChunkType = "documentation"
	ChunkComment       ChunkType = "comment"
	ChunkImport        ChunkType = "import"
)

// BoundaryType defines types of semantic boundaries for chunking.
type BoundaryType string

const (
	BoundaryFunction    BoundaryType = "function"
	BoundaryClass       BoundaryType = "class"
	BoundaryModule      BoundaryType = "module"
	BoundaryInterface   BoundaryType = "interface"
	BoundaryStatement   BoundaryType = "statement"
	BoundaryBlock       BoundaryType = "block"
	BoundaryComment     BoundaryType = "comment"
	BoundaryImport      BoundaryType = "import"
	BoundaryDeclaration BoundaryType = "declaration"
)

// PreservedContext contains context information preserved with chunks.
type PreservedContext struct {
	PrecedingContext   string                 `json:"preceding_context,omitempty"`
	FollowingContext   string                 `json:"following_context,omitempty"`
	ImportStatements   []ImportDeclaration    `json:"import_statements,omitempty"`
	TypeDefinitions    []TypeReference        `json:"type_definitions,omitempty"`
	UsedVariables      []VariableReference    `json:"used_variables,omitempty"`
	CalledFunctions    []FunctionCall         `json:"called_functions,omitempty"`
	DocumentationLinks []DocumentationLink    `json:"documentation_links,omitempty"`
	SemanticRelations  []SemanticRelation     `json:"semantic_relations,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// Supporting types for chunking functionality

type ChunkSizeRecommendation struct {
	OptimalSize     int      `json:"optimal_size"`
	MinSize         int      `json:"min_size"`
	MaxSize         int      `json:"max_size"`
	Confidence      float64  `json:"confidence"`
	Reasoning       string   `json:"reasoning"`
	LanguageFactors []string `json:"language_factors,omitempty"`
}

type ChunkValidationResult struct {
	ChunkID         string                 `json:"chunk_id"`
	IsValid         bool                   `json:"is_valid"`
	ValidationScore float64                `json:"validation_score"`
	Issues          []ChunkValidationIssue `json:"issues,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type ChunkValidationIssue struct {
	Type       ValidationIssueType `json:"type"`
	Severity   IssueSeverity       `json:"severity"`
	Message    string              `json:"message"`
	StartByte  uint32              `json:"start_byte,omitempty"`
	EndByte    uint32              `json:"end_byte,omitempty"`
	Suggestion string              `json:"suggestion,omitempty"`
}

type ValidationIssueType string

const (
	IssueIncompleteBoundary    ValidationIssueType = "incomplete_boundary"
	IssueMissingContext        ValidationIssueType = "missing_context"
	IssueBrokenDependency      ValidationIssueType = "broken_dependency"
	IssueSizeViolation         ValidationIssueType = "size_violation"
	IssueSemanticInconsistency ValidationIssueType = "semantic_inconsistency"
	IssuePoorCohesion          ValidationIssueType = "poor_cohesion"
	IssueHighComplexity        ValidationIssueType = "high_complexity"
)

type IssueSeverity string

const (
	SeverityLow      IssueSeverity = "low"
	SeverityMedium   IssueSeverity = "medium"
	SeverityHigh     IssueSeverity = "high"
	SeverityCritical IssueSeverity = "critical"
)

type ChunkDependency struct {
	Type         DependencyType `json:"type"`
	TargetChunk  string         `json:"target_chunk,omitempty"`
	TargetSymbol string         `json:"target_symbol"`
	Relationship string         `json:"relationship"`
	Strength     float64        `json:"strength"`
	IsResolved   bool           `json:"is_resolved"`
}

type DependencyType string

const (
	DependencyImport    DependencyType = "import"
	DependencyCall      DependencyType = "call"
	DependencyRef       DependencyType = "reference"
	DependencyInherit   DependencyType = "inherit"
	DependencyCompose   DependencyType = "compose"
	DependencyInterface DependencyType = "interface"
)

type ChunkQualityMetrics struct {
	CohesionScore        float64 `json:"cohesion_score"`
	CouplingScore        float64 `json:"coupling_score"`
	ComplexityScore      float64 `json:"complexity_score"`
	CompletenessScore    float64 `json:"completeness_score"`
	ReadabilityScore     float64 `json:"readability_score"`
	MaintainabilityScore float64 `json:"maintainability_score"`
	OverallQuality       float64 `json:"overall_quality"`
}

type ChunkRelation struct {
	RelatedChunkID string                 `json:"related_chunk_id"`
	RelationType   RelationType           `json:"relation_type"`
	Strength       float64                `json:"strength"`
	Description    string                 `json:"description,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type RelationType string

const (
	RelationParent    RelationType = "parent"
	RelationChild     RelationType = "child"
	RelationSibling   RelationType = "sibling"
	RelationDependsOn RelationType = "depends_on"
	RelationUsedBy    RelationType = "used_by"
	RelationSimilar   RelationType = "similar"
	RelationComposed  RelationType = "composed"
	RelationInherits  RelationType = "inherits"
)

type ChunkSize struct {
	Bytes      int `json:"bytes"`
	Lines      int `json:"lines"`
	Tokens     int `json:"tokens,omitempty"`
	Characters int `json:"characters"`
	Constructs int `json:"constructs"`
}

type VariableReference struct {
	Name            string `json:"name"`
	Type            string `json:"type,omitempty"`
	Scope           string `json:"scope"`
	DeclarationByte uint32 `json:"declaration_byte,omitempty"`
}

type DocumentationLink struct {
	Type        string `json:"type"`
	Target      string `json:"target"`
	Description string `json:"description,omitempty"`
}

type SemanticRelation struct {
	Type        string  `json:"type"`
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Strength    float64 `json:"strength"`
	Description string  `json:"description,omitempty"`
}

type ChunkingPerformanceHints struct {
	MaxConcurrency     int  `json:"max_concurrency,omitempty"`
	EnableCaching      bool `json:"enable_caching"`
	StreamProcessing   bool `json:"stream_processing"`
	MemoryOptimization bool `json:"memory_optimization"`
	LazyEvaluation     bool `json:"lazy_evaluation"`
}
