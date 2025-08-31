package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"time"

	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

// ParseTree represents a tree-sitter parse tree for the adapter layer.
type ParseTree struct {
	Language       string            `json:"language"`
	RootNode       *ParseNode        `json:"root_node"`
	Source         string            `json:"source"`
	CreatedAt      time.Time         `json:"created_at"`
	TreeSitterTree *tree_sitter.Tree `json:"-"` // Raw tree-sitter tree (not serialized)
}

// ParseNode represents a node in the tree-sitter parse tree.
type ParseNode struct {
	Type       string       `json:"type"`
	StartByte  uint32       `json:"start_byte"`
	EndByte    uint32       `json:"end_byte"`
	StartPoint Point        `json:"start_point"`
	EndPoint   Point        `json:"end_point"`
	Children   []*ParseNode `json:"children"`
	FieldName  string       `json:"field_name"`
	IsNamed    bool         `json:"is_named"`
	IsError    bool         `json:"is_error"`
	IsMissing  bool         `json:"is_missing"`
	HasChanges bool         `json:"has_changes"`
	HasError   bool         `json:"has_error"`
	Text       string       `json:"text"`
}

// Point represents a position in the source code.
type Point struct {
	Row    uint32 `json:"row"`
	Column uint32 `json:"column"`
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

// ParseOptions configures parsing behavior.
type ParseOptions struct {
	MaxSourceSize      int64         `json:"max_source_size"`
	Timeout            time.Duration `json:"timeout"`
	EnableRecovery     bool          `json:"enable_recovery"`
	RecoveryMode       bool          `json:"recovery_mode"`
	EnableStatistics   bool          `json:"enable_statistics"`
	EnableErrorLogging bool          `json:"enable_error_logging"`
	Language           string        `json:"language"`
	FilePath           string        `json:"file_path"`
	IncludeComments    bool          `json:"include_comments"`
	IncludeWhitespace  bool          `json:"include_whitespace"`
	MaxDepth           int           `json:"max_depth"`
	MaxErrors          int           `json:"max_errors"`
	TimeoutMs          int           `json:"timeout_ms"`
	EnableCaching      bool          `json:"enable_caching"`
}

// ParseError represents a parsing error.
type ParseError struct {
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	StartByte   uint32    `json:"start_byte,omitempty"`
	EndByte     uint32    `json:"end_byte,omitempty"`
	StartPoint  *Point    `json:"start_point,omitempty"`
	EndPoint    *Point    `json:"end_point,omitempty"`
	Severity    string    `json:"severity"`
	Code        string    `json:"code,omitempty"`
	Recoverable bool      `json:"recoverable"`
	Context     string    `json:"context,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// ParseWarning represents a parsing warning.
type ParseWarning struct {
	Type       string    `json:"type"`
	Message    string    `json:"message"`
	StartByte  uint32    `json:"start_byte,omitempty"`
	EndByte    uint32    `json:"end_byte,omitempty"`
	StartPoint *Point    `json:"start_point,omitempty"`
	EndPoint   *Point    `json:"end_point,omitempty"`
	Code       string    `json:"code,omitempty"`
	Context    string    `json:"context,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// ParsingStatistics contains statistics about the parsing operation.
type ParsingStatistics struct {
	TotalNodes     uint32        `json:"total_nodes"`
	ErrorNodes     uint32        `json:"error_nodes"`
	MissingNodes   uint32        `json:"missing_nodes"`
	MaxDepth       uint32        `json:"max_depth"`
	ParseDuration  time.Duration `json:"parse_duration"`
	MemoryUsed     uint64        `json:"memory_used"`
	BytesProcessed uint64        `json:"bytes_processed"`
	LinesProcessed uint32        `json:"lines_processed"`
	TreeSizeBytes  uint64        `json:"tree_size_bytes"`
}

// ParserConfiguration represents parser configuration.
type ParserConfiguration struct {
	Language           string        `json:"language"`
	MaxSourceSize      int64         `json:"max_source_size"`
	DefaultTimeout     time.Duration `json:"default_timeout"`
	MaxMemory          int64         `json:"max_memory"`
	EnableMetrics      bool          `json:"enable_metrics"`
	MetricsEnabled     bool          `json:"metrics_enabled"`
	EnableLogging      bool          `json:"enable_logging"`
	LogLevel           string        `json:"log_level"`
	MaxConcurrency     int           `json:"max_concurrency"`
	ConcurrencyLimit   int           `json:"concurrency_limit"`
	CacheSize          int           `json:"cache_size"`
	CacheEnabled       bool          `json:"cache_enabled"`
	EnableRecovery     bool          `json:"enable_recovery"`
	EnableStatistics   bool          `json:"enable_statistics"`
	TracingEnabled     bool          `json:"tracing_enabled"`
	PerformanceProfile string        `json:"performance_profile"`
}

// FactoryConfiguration represents factory configuration.
type FactoryConfiguration struct {
	MaxCachedParsers    int                    `json:"max_cached_parsers"`
	ParserTimeout       time.Duration          `json:"parser_timeout"`
	ConcurrencyLimit    int                    `json:"concurrency_limit"`
	CacheEvictionPolicy string                 `json:"cache_eviction_policy"`
	HealthCheckInterval time.Duration          `json:"health_check_interval"`
	EnableMetrics       bool                   `json:"enable_metrics"`
	EnableTracing       bool                   `json:"enable_tracing"`
	EnableLogging       bool                   `json:"enable_logging"`
	LogLevel            string                 `json:"log_level"`
	SupportedLanguages  []string               `json:"supported_languages"`
	DefaultLanguage     string                 `json:"default_language"`
	ParserPoolSize      int                    `json:"parser_pool_size"`
	WarmupEnabled       bool                   `json:"warmup_enabled"`
	EnableAutoCleanup   bool                   `json:"enable_auto_cleanup"`
	DefaultParserConfig ParserConfiguration    `json:"default_parser_config"`
	LanguageConfigs     map[string]interface{} `json:"language_configs,omitempty"`
	ResourceLimits      map[string]interface{} `json:"resource_limits"`
}

// ParserPool represents a pool of parsers.
type ParserPool struct {
	Size               int           `json:"size"`
	ActiveParsers      int           `json:"active_parsers"`
	IdleParsers        int           `json:"idle_parsers"`
	MaxConcurrency     int           `json:"max_concurrency"`
	TotalRequests      uint64        `json:"total_requests"`
	ActiveRequests     int           `json:"active_requests"`
	QueuedRequests     int           `json:"queued_requests"`
	AverageWaitTime    time.Duration `json:"average_wait_time"`
	PeakConcurrency    int           `json:"peak_concurrency"`
	HealthStatus       string        `json:"health_status"`
	LastHealthCheck    time.Time     `json:"last_health_check"`
	PoolCreatedAt      time.Time     `json:"pool_created_at"`
	SupportedLanguages []string      `json:"supported_languages"`
	// Fields expected by observable_parser.go
	MaxSize          int                   `json:"max_size"`
	CurrentSize      int                   `json:"current_size"`
	AvailableParsers int                   `json:"available_parsers"`
	InUseParsers     int                   `json:"in_use_parsers"`
	Languages        []string              `json:"languages"`
	CreatedAt        time.Time             `json:"created_at"`
	LastAccessed     time.Time             `json:"last_accessed"`
	PoolStatistics   *ParserPoolStatistics `json:"pool_statistics,omitempty"`
	Configuration    *FactoryConfiguration `json:"configuration,omitempty"`
}

// ParserPoolStatistics holds detailed statistics about parser pool usage.
type ParserPoolStatistics struct {
	TotalCreated        int64         `json:"total_created"`
	TotalDestroyed      int64         `json:"total_destroyed"`
	TotalCheckouts      int64         `json:"total_checkouts"`
	TotalReturns        int64         `json:"total_returns"`
	AverageCheckoutTime time.Duration `json:"average_checkout_time"`
	PeakUsage           int           `json:"peak_usage"`
	CacheHitRate        float64       `json:"cache_hit_rate"`
	ErrorRate           float64       `json:"error_rate"`
}

// Additional types needed by the production code

// Edit represents an edit operation for incremental parsing.
type Edit struct {
	StartByte   uint32 `json:"start_byte"`
	OldEndByte  uint32 `json:"old_end_byte"`
	NewEndByte  uint32 `json:"new_end_byte"`
	StartPoint  Point  `json:"start_point"`
	OldEndPoint Point  `json:"old_end_point"`
	NewEndPoint Point  `json:"new_end_point"`
}

// QueryResult represents the result of a tree-sitter query.
type QueryResult struct {
	Matches     []*QueryMatch    `json:"matches"`
	Captures    []*QueryCapture  `json:"captures"`
	Duration    time.Duration    `json:"duration"`
	QueryString string           `json:"query_string"`
	Statistics  *QueryStatistics `json:"statistics,omitempty"`
}

// QueryMatch represents a single query match.
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

// QueryStatistics holds statistics about query operations.
type QueryStatistics struct {
	TotalQueries    int64         `json:"total_queries"`
	QueryDuration   time.Duration `json:"query_duration"`
	MatchesFound    int           `json:"matches_found"`
	CapturesFound   int           `json:"captures_found"`
	QueryComplexity int           `json:"query_complexity"`
	CacheHits       int64         `json:"cache_hits"`
	CacheMisses     int64         `json:"cache_misses"`
}

// ParseRequest represents a parsing request.
type ParseRequest struct {
	ID       string       `json:"id"`
	Language string       `json:"language"`
	Source   []byte       `json:"source"`
	Options  ParseOptions `json:"options"`
	Priority int          `json:"priority"`
}

// ParserPerformanceMetrics holds performance metrics for a parser.
type ParserPerformanceMetrics struct {
	TotalParses         int64         `json:"total_parses"`
	AverageParseTime    time.Duration `json:"average_parse_time"`
	MemoryUsage         int64         `json:"memory_usage"`
	CacheHitRate        float64       `json:"cache_hit_rate"`
	ErrorRate           float64       `json:"error_rate"`
	ThroughputPerSecond int64         `json:"throughput_per_second"`
	LastUpdated         time.Time     `json:"last_updated"`
}

// ParserHealthStatus represents the health status of a parser.
type ParserHealthStatus struct {
	IsHealthy     bool              `json:"is_healthy"`
	Status        string            `json:"status"`
	LastChecked   time.Time         `json:"last_checked"`
	Issues        []string          `json:"issues,omitempty"`
	Metrics       map[string]int64  `json:"metrics"`
	Configuration map[string]string `json:"configuration"`
	Uptime        time.Duration     `json:"uptime"`
}

// TimeRange represents a time range for metrics.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// DetailedParsingMetrics holds detailed parsing metrics.
type DetailedParsingMetrics struct {
	TimeRange       TimeRange                      `json:"time_range"`
	TotalOperations int64                          `json:"total_operations"`
	SuccessRate     float64                        `json:"success_rate"`
	LanguageStats   map[string]*LanguageStatistics `json:"language_stats"`
	ErrorBreakdown  map[string]int64               `json:"error_breakdown"`
	PerformanceData *PerformanceData               `json:"performance_data"`
	Trends          *MetricsTrends                 `json:"trends,omitempty"`
}

// LanguageStatistics holds statistics for a specific language.
type LanguageStatistics struct {
	ParseCount       int64         `json:"parse_count"`
	AverageTime      time.Duration `json:"average_time"`
	ErrorCount       int64         `json:"error_count"`
	SuccessRate      float64       `json:"success_rate"`
	LargestFileSize  int64         `json:"largest_file_size"`
	SmallestFileSize int64         `json:"smallest_file_size"`
}

// PerformanceData holds performance statistics.
type PerformanceData struct {
	MinParseTime    time.Duration `json:"min_parse_time"`
	MaxParseTime    time.Duration `json:"max_parse_time"`
	MedianParseTime time.Duration `json:"median_parse_time"`
	P95ParseTime    time.Duration `json:"p95_parse_time"`
	P99ParseTime    time.Duration `json:"p99_parse_time"`
	MemoryPeak      int64         `json:"memory_peak"`
	MemoryAverage   int64         `json:"memory_average"`
}

// MetricsTrends holds trend information for metrics.
type MetricsTrends struct {
	PerformanceTrend string  `json:"performance_trend"`
	ErrorRateTrend   string  `json:"error_rate_trend"`
	ThroughputTrend  string  `json:"throughput_trend"`
	Confidence       float64 `json:"confidence"`
}

// FactoryParserVersionInfo holds version information for the parser factory.
type FactoryParserVersionInfo struct {
	TreeSitterVersion  string                       `json:"tree_sitter_version"`
	ParserVersion      string                       `json:"parser_version"`
	SupportedLanguages []FactoryLanguageVersionInfo `json:"supported_languages"`
	BuildInfo          map[string]interface{}       `json:"build_info"`
	CreatedAt          time.Time                    `json:"created_at"`
}

// FactoryLanguageVersionInfo holds version information for a specific language in the factory.
type FactoryLanguageVersionInfo struct {
	Name           string    `json:"name"`
	GrammarVersion string    `json:"grammar_version"`
	ParserVersion  string    `json:"parser_version"`
	LoadedAt       time.Time `json:"loaded_at"`
}

// ObservableTreeSitterParser defines the interface for observable parsers.
type ObservableTreeSitterParser interface {
	// Parse parses source code and returns a parse tree
	Parse(ctx context.Context, source []byte) (*ParseResult, error)

	// GetLanguage returns the language this parser handles
	GetLanguage() string

	// Close cleans up parser resources
	Close() error

	// Additional methods needed by the implementation
	ParseSource(
		ctx context.Context,
		language valueobject.Language,
		source []byte,
		options ParseOptions,
	) (*ParseResult, error)
}

// FactoryHealthStatus represents the health status of the parser factory.
type FactoryHealthStatus struct {
	IsHealthy          bool              `json:"is_healthy"`
	Status             string            `json:"status"`
	LastChecked        time.Time         `json:"last_checked"`
	Issues             []string          `json:"issues,omitempty"`
	ParserPoolHealth   *ParserPoolHealth `json:"parser_pool_health,omitempty"`
	SupportedLanguages int               `json:"supported_languages"`
	ActiveParsers      int               `json:"active_parsers"`
	MemoryUsage        int64             `json:"memory_usage"`
	ConfigurationValid bool              `json:"configuration_valid"`
}

// ParserPoolHealth represents health status of the parser pool.
type ParserPoolHealth struct {
	PoolSize         int       `json:"pool_size"`
	HealthyParsers   int       `json:"healthy_parsers"`
	UnhealthyParsers int       `json:"unhealthy_parsers"`
	LastPoolCleanup  time.Time `json:"last_pool_cleanup"`
}
