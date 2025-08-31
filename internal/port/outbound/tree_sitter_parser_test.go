package outbound

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TreeSitterParser defines the interface for tree-sitter based parsing operations.
// This is a RED PHASE test that defines expected tree-sitter parser interface behavior.
type TreeSitterParser interface {
	// ParseSource parses source code and returns a parse tree
	ParseSource(
		ctx context.Context,
		language valueobject.Language,
		source []byte,
		options ParseOptions,
	) (*ParseResult, error)

	// ParseFile parses a source file and returns a parse tree
	ParseFile(
		ctx context.Context,
		language valueobject.Language,
		filepath string,
		options ParseOptions,
	) (*ParseResult, error)

	// ParseWithTimeout parses source code with a specified timeout
	ParseWithTimeout(
		ctx context.Context,
		language valueobject.Language,
		source []byte,
		timeout time.Duration,
		options ParseOptions,
	) (*ParseResult, error)

	// GetSupportedLanguages returns all languages supported by this parser
	GetSupportedLanguages(ctx context.Context) ([]valueobject.Language, error)

	// ValidateGrammar validates that a grammar is properly loaded for a language
	ValidateGrammar(ctx context.Context, language valueobject.Language) error

	// GetParserVersion returns the tree-sitter parser version information
	GetParserVersion(ctx context.Context) (*ParserVersionInfo, error)
}

// EnhancedTreeSitterParser extends TreeSitterParser with advanced capabilities.
type EnhancedTreeSitterParser interface {
	TreeSitterParser

	// ParseIncremental performs incremental parsing on existing parse trees
	ParseIncremental(ctx context.Context, oldTree *ParseTree, newSource []byte, changes []Edit) (*ParseResult, error)

	// ParseWithRecovery attempts to recover from parsing errors
	ParseWithRecovery(
		ctx context.Context,
		language valueobject.Language,
		source []byte,
		recoveryOptions RecoveryOptions,
	) (*ParseResult, error)

	// GetNodeAtCursor returns the node at a specific cursor position
	GetNodeAtCursor(ctx context.Context, tree *ParseTree, cursor TreeCursor) (*ParseNode, error)

	// QueryTree executes a tree-sitter query against a parse tree
	QueryTree(ctx context.Context, tree *ParseTree, query string) (*QueryResult, error)

	// ExtractSyntaxHighlighting extracts syntax highlighting information
	ExtractSyntaxHighlighting(ctx context.Context, tree *ParseTree) (*SyntaxHighlighting, error)
}

// ConfigurableTreeSitterParser extends EnhancedTreeSitterParser with configuration capabilities.
type ConfigurableTreeSitterParser interface {
	EnhancedTreeSitterParser

	// UpdateConfiguration updates parser configuration at runtime
	UpdateConfiguration(ctx context.Context, config *ParserConfiguration) error

	// GetConfiguration returns current parser configuration
	GetConfiguration(ctx context.Context) (*ParserConfiguration, error)

	// LoadCustomGrammar loads a custom grammar for a language
	LoadCustomGrammar(ctx context.Context, language valueobject.Language, grammarPath string) error

	// UnloadGrammar unloads a grammar for a language
	UnloadGrammar(ctx context.Context, language valueobject.Language) error

	// SetMemoryLimit sets memory limits for parsing operations
	SetMemoryLimit(ctx context.Context, limit int64) error
}

// PerformanceOptimizedParser extends ConfigurableTreeSitterParser with performance features.
type PerformanceOptimizedParser interface {
	ConfigurableTreeSitterParser

	// ParseConcurrently parses multiple sources concurrently
	ParseConcurrently(ctx context.Context, requests []ParseRequest) ([]*ParseResult, error)

	// GetParserPool returns a pool of parser instances for concurrent use
	GetParserPool(ctx context.Context) (*ParserPool, error)

	// WarmupParser preloads grammars and prepares parser for optimal performance
	WarmupParser(ctx context.Context, languages []valueobject.Language) error

	// GetPerformanceMetrics returns detailed performance metrics
	GetPerformanceMetrics(ctx context.Context) (*ParserPerformanceMetrics, error)

	// EnableProfiling enables or disables performance profiling
	EnableProfiling(ctx context.Context, enabled bool) error
}

// ObservableTreeSitterParser extends PerformanceOptimizedParser with observability features.
type ObservableTreeSitterParser interface {
	PerformanceOptimizedParser

	// RecordMetrics records parsing metrics for observability
	RecordMetrics(ctx context.Context, result *ParseResult) error

	// GetHealthStatus returns health status of the parser
	GetHealthStatus(ctx context.Context) (*ParserHealthStatus, error)

	// EnableTracing enables distributed tracing for parsing operations
	EnableTracing(ctx context.Context, enabled bool) error

	// GetDetailedMetrics returns comprehensive parsing metrics over time
	GetDetailedMetrics(ctx context.Context, timeRange TimeRange) (*DetailedParsingMetrics, error)
}

// Supporting types for the parser interface

// ParseOptions configures parsing behavior.
type ParseOptions struct {
	IncludeComments   bool                   `json:"include_comments"`
	IncludeWhitespace bool                   `json:"include_whitespace"`
	MaxDepth          int                    `json:"max_depth"`
	MaxNodes          int                    `json:"max_nodes"`
	MaxErrors         int                    `json:"max_errors"`
	TimeoutMs         int                    `json:"timeout_ms"`
	RecoveryMode      bool                   `json:"recovery_mode"`
	ValidateUTF8      bool                   `json:"validate_utf8"`
	CustomFeatures    map[string]interface{} `json:"custom_features,omitempty"`
	PerformanceHints  *PerformanceHints      `json:"performance_hints,omitempty"`
	ErrorHandling     *ErrorHandlingOptions  `json:"error_handling,omitempty"`
}

// ParseResult represents the result of a parsing operation.
type ParseResult struct {
	Success    bool                   `json:"success"`
	ParseTree  *ParseTree             `json:"parse_tree,omitempty"`
	Errors     []ParseError           `json:"errors,omitempty"`
	Warnings   []ParseWarning         `json:"warnings,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Statistics *ParsingStatistics     `json:"statistics,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ParseTree represents a tree-sitter parse tree.
type ParseTree struct {
	Language       valueobject.Language `json:"language"`
	RootNode       *ParseNode           `json:"root_node"`
	Source         []byte               `json:"-"`
	CreatedAt      time.Time            `json:"created_at"`
	TreeSitterTree interface{}          `json:"-"` // Actual tree-sitter tree object
}

// ParseNode represents a node in the parse tree.
type ParseNode struct {
	Type       string       `json:"type"`
	StartByte  uint32       `json:"start_byte"`
	EndByte    uint32       `json:"end_byte"`
	StartPoint Point        `json:"start_point"`
	EndPoint   Point        `json:"end_point"`
	Children   []*ParseNode `json:"children,omitempty"`
	FieldName  string       `json:"field_name,omitempty"`
	IsNamed    bool         `json:"is_named"`
	IsError    bool         `json:"is_error"`
	IsMissing  bool         `json:"is_missing"`
	HasChanges bool         `json:"has_changes"`
	HasError   bool         `json:"has_error"`
	Text       string       `json:"text,omitempty"`
}

// Point represents a position in source code.
type Point struct {
	Row    uint32 `json:"row"`
	Column uint32 `json:"column"`
}

// Edit represents a change to source code for incremental parsing.
type Edit struct {
	StartByte   uint32 `json:"start_byte"`
	OldEndByte  uint32 `json:"old_end_byte"`
	NewEndByte  uint32 `json:"new_end_byte"`
	StartPoint  Point  `json:"start_point"`
	OldEndPoint Point  `json:"old_end_point"`
	NewEndPoint Point  `json:"new_end_point"`
}

// TreeCursor represents a cursor for traversing parse trees.
type TreeCursor struct {
	Node      *ParseNode `json:"node"`
	FieldName string     `json:"field_name,omitempty"`
	FieldID   uint16     `json:"field_id"`
	Depth     uint32     `json:"depth"`
}

// QueryResult represents the result of a tree-sitter query.
type QueryResult struct {
	Matches     []*QueryMatch    `json:"matches"`
	Captures    []*QueryCapture  `json:"captures"`
	Duration    time.Duration    `json:"duration"`
	QueryString string           `json:"query_string"`
	Statistics  *QueryStatistics `json:"statistics,omitempty"`
}

// QueryMatch represents a query match.
type QueryMatch struct {
	ID           uint32          `json:"id"`
	PatternIndex uint16          `json:"pattern_index"`
	Captures     []*QueryCapture `json:"captures"`
}

// QueryCapture represents a captured node from a query.
type QueryCapture struct {
	Node  *ParseNode `json:"node"`
	Index uint32     `json:"index"`
	Name  string     `json:"name"`
}

// Additional supporting types

type RecoveryOptions struct {
	MaxRetries         int      `json:"max_retries"`
	BacktrackLimit     int      `json:"backtrack_limit"`
	ErrorTolerance     float64  `json:"error_tolerance"`
	RecoveryStrategies []string `json:"recovery_strategies"`
}

type ParserVersionInfo struct {
	TreeSitterVersion string                 `json:"tree_sitter_version"`
	GrammarVersions   map[string]string      `json:"grammar_versions"`
	BuildInfo         map[string]interface{} `json:"build_info"`
	Capabilities      []string               `json:"capabilities"`
}

type ParserConfiguration struct {
	DefaultTimeout   time.Duration          `json:"default_timeout"`
	MaxMemory        int64                  `json:"max_memory"`
	ConcurrencyLimit int                    `json:"concurrency_limit"`
	CacheEnabled     bool                   `json:"cache_enabled"`
	CacheSize        int                    `json:"cache_size"`
	MetricsEnabled   bool                   `json:"metrics_enabled"`
	TracingEnabled   bool                   `json:"tracing_enabled"`
	CustomOptions    map[string]interface{} `json:"custom_options,omitempty"`
}

type ParseRequest struct {
	ID       string               `json:"id"`
	Language valueobject.Language `json:"language"`
	Source   []byte               `json:"source"`
	Options  ParseOptions         `json:"options"`
	Priority int                  `json:"priority"`
}

type ParserPool struct {
	Size      int       `json:"size"`
	Available int       `json:"available"`
	InUse     int       `json:"in_use"`
	Created   time.Time `json:"created"`
	Languages []string  `json:"languages"`
}

type PerformanceHints struct {
	OptimizeForSpeed  bool   `json:"optimize_for_speed"`
	OptimizeForMemory bool   `json:"optimize_for_memory"`
	PreferIncremental bool   `json:"prefer_incremental"`
	CacheStrategy     string `json:"cache_strategy"`
}

type ErrorHandlingOptions struct {
	StopOnFirstError   bool     `json:"stop_on_first_error"`
	MaxErrors          int      `json:"max_errors"`
	IgnoreTypes        []string `json:"ignore_types,omitempty"`
	RecoveryStrategies []string `json:"recovery_strategies,omitempty"`
}

// Additional missing types for tests.
type ParserPerformanceMetrics struct {
	TotalParses         int64         `json:"total_parses"`
	AverageParseTime    time.Duration `json:"average_parse_time"`
	MemoryUsage         int64         `json:"memory_usage"`
	CacheHitRate        float64       `json:"cache_hit_rate"`
	ErrorRate           float64       `json:"error_rate"`
	ThroughputPerSecond int64         `json:"throughput_per_second"`
	LastUpdated         time.Time     `json:"last_updated"`
}

type ParserHealthStatus struct {
	IsHealthy     bool              `json:"is_healthy"`
	Status        string            `json:"status"`
	LastChecked   time.Time         `json:"last_checked"`
	Issues        []string          `json:"issues,omitempty"`
	Metrics       map[string]int64  `json:"metrics"`
	Configuration map[string]string `json:"configuration"`
	Uptime        time.Duration     `json:"uptime"`
}

type DetailedParsingMetrics struct {
	TimeRange       TimeRange                      `json:"time_range"`
	TotalOperations int64                          `json:"total_operations"`
	SuccessRate     float64                        `json:"success_rate"`
	LanguageStats   map[string]*LanguageStatistics `json:"language_stats"`
	ErrorBreakdown  map[string]int64               `json:"error_breakdown"`
	PerformanceData *PerformanceData               `json:"performance_data"`
	Trends          *MetricsTrends                 `json:"trends,omitempty"`
}

type QueryStatistics struct {
	TotalQueries    int64         `json:"total_queries"`
	QueryDuration   time.Duration `json:"query_duration"`
	MatchesFound    int           `json:"matches_found"`
	CapturesFound   int           `json:"captures_found"`
	QueryComplexity int           `json:"query_complexity"`
	CacheHits       int64         `json:"cache_hits"`
	CacheMisses     int64         `json:"cache_misses"`
}

type LanguageStatistics struct {
	ParseCount       int64         `json:"parse_count"`
	AverageTime      time.Duration `json:"average_time"`
	ErrorCount       int64         `json:"error_count"`
	SuccessRate      float64       `json:"success_rate"`
	LargestFileSize  int64         `json:"largest_file_size"`
	SmallestFileSize int64         `json:"smallest_file_size"`
}

type PerformanceData struct {
	MinParseTime    time.Duration `json:"min_parse_time"`
	MaxParseTime    time.Duration `json:"max_parse_time"`
	MedianParseTime time.Duration `json:"median_parse_time"`
	P95ParseTime    time.Duration `json:"p95_parse_time"`
	P99ParseTime    time.Duration `json:"p99_parse_time"`
	MemoryPeak      int64         `json:"memory_peak"`
	MemoryAverage   int64         `json:"memory_average"`
}

type MetricsTrends struct {
	PerformanceTrend string  `json:"performance_trend"` // "improving", "degrading", "stable"
	ErrorRateTrend   string  `json:"error_rate_trend"`
	ThroughputTrend  string  `json:"throughput_trend"`
	Confidence       float64 `json:"confidence"`
}

// ParseError represents an error that occurred during parsing.
type ParseError struct {
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	Position    Point     `json:"position,omitempty"`
	ByteOffset  uint32    `json:"byte_offset,omitempty"`
	Severity    string    `json:"severity"`
	Code        string    `json:"code,omitempty"`
	Recoverable bool      `json:"recoverable"`
	Context     string    `json:"context,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// ParseWarning represents a warning that occurred during parsing.
type ParseWarning struct {
	Type       string    `json:"type"`
	Message    string    `json:"message"`
	Position   Point     `json:"position,omitempty"`
	ByteOffset uint32    `json:"byte_offset,omitempty"`
	Code       string    `json:"code,omitempty"`
	Context    string    `json:"context,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type ParsingStatistics struct {
	TotalNodes     int           `json:"total_nodes"`
	ErrorNodes     int           `json:"error_nodes"`
	MissingNodes   int           `json:"missing_nodes"`
	MaxDepth       int           `json:"max_depth"`
	ParseDuration  time.Duration `json:"parse_duration"`
	MemoryUsed     int64         `json:"memory_used"`
	BytesProcessed int64         `json:"bytes_processed"`
	LinesProcessed int           `json:"lines_processed"`
	TreeSizeBytes  int64         `json:"tree_size_bytes"`
}

type SyntaxHighlighting struct {
	Tokens      []*HighlightToken `json:"tokens"`
	Scopes      []*HighlightScope `json:"scopes"`
	Duration    time.Duration     `json:"duration"`
	TotalTokens int               `json:"total_tokens"`
}

type HighlightToken struct {
	StartByte uint32   `json:"start_byte"`
	EndByte   uint32   `json:"end_byte"`
	TokenType string   `json:"token_type"`
	Scope     string   `json:"scope"`
	Modifiers []string `json:"modifiers,omitempty"`
}

type HighlightScope struct {
	Name      string            `json:"name"`
	StartByte uint32            `json:"start_byte"`
	EndByte   uint32            `json:"end_byte"`
	Children  []*HighlightScope `json:"children,omitempty"`
}

// TestTreeSitterParser_BasicParsing tests basic parsing operations.
// This is a RED PHASE test that defines expected behavior for basic parsing.
func TestTreeSitterParser_BasicParsing(t *testing.T) {
	tests := []struct {
		name          string
		language      valueobject.Language
		source        []byte
		options       ParseOptions
		expectSuccess bool
		expectError   bool
		errorMsg      string
		validate      func(*ParseResult) bool
	}{
		{
			name: "parse simple Go source",
			language: func() valueobject.Language {
				lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
				return lang
			}(),
			source: []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`),
			options: ParseOptions{
				IncludeComments:   false,
				IncludeWhitespace: false,
				MaxDepth:          100,
				TimeoutMs:         5000,
				RecoveryMode:      false,
			},
			expectSuccess: true,
			expectError:   false,
			validate: func(result *ParseResult) bool {
				return result.Success &&
					result.ParseTree != nil &&
					result.ParseTree.RootNode.Type == "source_file" &&
					len(result.Errors) == 0
			},
		},
		{
			name: "parse Python source with classes",
			language: func() valueobject.Language {
				lang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
				return lang
			}(),
			source: []byte(`class Calculator:
    def __init__(self):
        self.value = 0
    
    def add(self, x):
        self.value += x
        return self.value`),
			options: ParseOptions{
				IncludeComments:   true,
				IncludeWhitespace: false,
				MaxDepth:          50,
				TimeoutMs:         3000,
				RecoveryMode:      false,
			},
			expectSuccess: true,
			expectError:   false,
			validate: func(result *ParseResult) bool {
				return result.Success &&
					result.ParseTree != nil &&
					result.ParseTree.RootNode.Type == "module" &&
					result.Statistics.TotalNodes > 10
			},
		},
		{
			name: "parse JavaScript with syntax error",
			language: func() valueobject.Language {
				lang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
				return lang
			}(),
			source: []byte(`function test() {
    console.log("missing closing brace"`),
			options: ParseOptions{
				RecoveryMode: true,
				MaxErrors:    10,
				TimeoutMs:    2000,
			},
			expectSuccess: false,
			expectError:   false, // No error, but parsing fails
			validate: func(result *ParseResult) bool {
				return !result.Success &&
					result.ParseTree != nil && // Still creates a tree with errors
					len(result.Errors) > 0 &&
					result.Statistics.ErrorNodes > 0
			},
		},
		{
			name: "unsupported language should fail",
			language: func() valueobject.Language {
				lang, _ := valueobject.NewLanguage("UnsupportedLanguage")
				return lang
			}(),
			source: []byte(`some code`),
			options: ParseOptions{
				TimeoutMs: 1000,
			},
			expectSuccess: false,
			expectError:   true,
			errorMsg:      "unsupported language",
		},
		{
			name: "empty source should handle gracefully",
			language: func() valueobject.Language {
				lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
				return lang
			}(),
			source:        []byte{},
			options:       ParseOptions{},
			expectSuccess: true,
			expectError:   false,
			validate: func(result *ParseResult) bool {
				return result.Success &&
					result.ParseTree != nil &&
					result.Statistics.TotalNodes >= 1 // At least root node
			},
		},
		{
			name: "timeout should be enforced",
			language: func() valueobject.Language {
				lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
				return lang
			}(),
			source: []byte(`package main
// Very large file that would take time to parse...
` + string(make([]byte, 100000))),
			options: ParseOptions{
				TimeoutMs: 1, // Very short timeout
			},
			expectSuccess: false,
			expectError:   true,
			errorMsg:      "parsing timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test assumes we have a TreeSitterParser implementation
			// In the RED phase, we're defining the expected interface behavior
			// Skip test as no actual implementation is injected yet
			// This is a red-phase test that should be updated once implementation is available
			t.Skip(
				"TreeSitterParser implementation not available in this context - skipping port test during refactoring",
			)
		})
	}
}

// TestTreeSitterParser_AdvancedFeatures tests advanced parsing features.
// This is a RED PHASE test that defines expected behavior for advanced features.
func TestTreeSitterParser_AdvancedFeatures(t *testing.T) {
	t.Run("incremental parsing", func(t *testing.T) {
		// Skip test as no actual implementation is injected yet
		// This is a red-phase test that should be updated once implementation is available
		t.Skip("EnhancedTreeSitterParser implementation not available - skipping port test during refactoring")
	})

	t.Run("tree queries", func(t *testing.T) {
		// Skip test as no actual implementation is injected yet
		t.Skip("EnhancedTreeSitterParser implementation not available - skipping port test during refactoring")
	})

	t.Run("syntax highlighting extraction", func(t *testing.T) {
		// Skip test as no actual implementation is injected yet
		t.Skip("EnhancedTreeSitterParser implementation not available - skipping port test during refactoring")
	})
}

// TestTreeSitterParser_ErrorHandling tests error handling and recovery.
// This is a RED PHASE test that defines expected error handling behavior.
func TestTreeSitterParser_ErrorHandling(t *testing.T) {
	// Skip this entire test suite as no actual implementation is available
	// This is a red-phase test that should be updated once implementation is available
	t.Skip(
		"TreeSitterParser implementation not available for error handling tests - skipping port test during refactoring",
	)
}

// TestTreeSitterParser_PerformanceRequirements tests performance requirements.
// This is a RED PHASE test that defines expected performance behavior.
func TestTreeSitterParser_PerformanceRequirements(t *testing.T) {
	// Skip this entire test suite as no actual implementation is available
	// This is a red-phase test that should be updated once implementation is available
	t.Skip(
		"TreeSitterParser implementation not available for performance tests - skipping port test during refactoring",
	)

	t.Run("parsing performance benchmarks", func(t *testing.T) {
		// Define performance requirements that the implementation must meet
		requirements := map[string]struct {
			maxDuration time.Duration
			maxMemoryMB int64
			sourceSize  int
			description string
		}{
			"small_file": {
				maxDuration: 10 * time.Millisecond,
				maxMemoryMB: 5,
				sourceSize:  1000, // 1KB
				description: "Small Go source file",
			},
			"medium_file": {
				maxDuration: 100 * time.Millisecond,
				maxMemoryMB: 20,
				sourceSize:  100000, // 100KB
				description: "Medium Go source file",
			},
			"large_file": {
				maxDuration: 1 * time.Second,
				maxMemoryMB: 100,
				sourceSize:  1000000, // 1MB
				description: "Large Go source file",
			},
		}

		for name, req := range requirements {
			t.Run(name, func(t *testing.T) {
				// This test defines the performance requirements
				// The actual implementation will need to meet these benchmarks

				ctx := context.Background()
				var parser TreeSitterParser // Would be injected

				language, _ := valueobject.NewLanguage(valueobject.LanguageGo)
				source := generateGoSource(req.sourceSize)
				options := ParseOptions{
					TimeoutMs: int(req.maxDuration.Milliseconds()),
				}

				start := time.Now()
				result, err := parser.ParseSource(ctx, language, source, options)
				duration := time.Since(start)

				// Performance assertions
				require.NoError(t, err, "Parsing should succeed for %s", req.description)
				require.NotNil(t, result, "Result should not be nil")

				assert.Less(t, duration, req.maxDuration,
					"Parsing %s should complete within %v", req.description, req.maxDuration)

				if result.Statistics != nil {
					memoryUsedMB := result.Statistics.MemoryUsed / (1024 * 1024)
					assert.Less(t, memoryUsedMB, req.maxMemoryMB,
						"Memory usage for %s should be less than %d MB", req.description, req.maxMemoryMB)
				}

				t.Logf("Performance for %s: Duration=%v, Expected=<%v",
					req.description, duration, req.maxDuration)
			})
		}
	})
}

// Helper function to generate Go source code of specified size.
func generateGoSource(size int) []byte {
	template := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

`

	result := make([]byte, 0, size)
	for len(result) < size {
		remaining := size - len(result)
		if remaining < len(template) {
			result = append(result, template[:remaining]...)
		} else {
			result = append(result, template...)
		}
	}

	return result
}
