package treesitter

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"context"
	"fmt"
	"sync"
	"time"

	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// BaseTreeSitterParser provides a foundation for tree-sitter parser implementations
// following the hexagonal architecture pattern and production best practices.
type BaseTreeSitterParser struct {
	parsers            map[string]*tree_sitter.Parser   // Language to parser mapping
	languages          map[string]*tree_sitter.Language // Language to grammar mapping
	supportedLanguages []valueobject.Language
	mu                 sync.RWMutex
	metrics            *ParserMetrics
	config             ParserConfig
}

// ParserConfig holds configuration for the tree-sitter parser.
type ParserConfig struct {
	MaxSourceSize  int64         // Maximum source code size in bytes
	DefaultTimeout time.Duration // Default parsing timeout
	CacheSize      int           // Parser cache size
	EnableMetrics  bool          // Enable OTEL metrics
	EnableLogging  bool          // Enable structured logging
	LogLevel       string        // Log level for parser operations
	MaxConcurrency int           // Maximum concurrent parsing operations
}

// ParserMetrics holds OTEL metrics for parser operations.
type ParserMetrics struct {
	parseOperationsTotal    metric.Int64Counter
	parseDurationHistogram  metric.Float64Histogram
	parseErrorsTotal        metric.Int64Counter
	parserCacheHits         metric.Int64Counter
	parserCacheMisses       metric.Int64Counter
	activeParsingOperations metric.Int64Gauge
}

// ParserVersionInfo provides version information about the tree-sitter parser.
type ParserVersionInfo struct {
	TreeSitterVersion  string                 `json:"tree_sitter_version"`
	ParserVersion      string                 `json:"parser_version"`
	SupportedLanguages []LanguageVersionInfo  `json:"supported_languages"`
	BuildInfo          map[string]interface{} `json:"build_info"`
	CreatedAt          time.Time              `json:"created_at"`
}

// LanguageVersionInfo provides version information for a specific language grammar.
type LanguageVersionInfo struct {
	Name           string    `json:"name"`
	GrammarVersion string    `json:"grammar_version"`
	ParserVersion  string    `json:"parser_version"`
	LoadedAt       time.Time `json:"loaded_at"`
}

// NewBaseTreeSitterParser creates a new base tree-sitter parser with production features.
func NewBaseTreeSitterParser(ctx context.Context, config ParserConfig) (*BaseTreeSitterParser, error) {
	if config.MaxSourceSize <= 0 {
		config.MaxSourceSize = DefaultMaxSourceSize
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = DefaultParserTimeout
	}
	if config.CacheSize <= 0 {
		config.CacheSize = DefaultCacheSize
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = DefaultMaxConcurrency
	}

	var metrics *ParserMetrics
	var err error
	if config.EnableMetrics {
		metrics, err = initParserMetrics()
		if err != nil {
			slogger.Warn(ctx, "Failed to initialize parser metrics", slogger.Fields{
				"error": err.Error(),
			})
		}
	}

	parser := &BaseTreeSitterParser{
		parsers:            make(map[string]*tree_sitter.Parser),
		languages:          make(map[string]*tree_sitter.Language),
		supportedLanguages: make([]valueobject.Language, 0),
		metrics:            metrics,
		config:             config,
	}

	slogger.Info(ctx, "BaseTreeSitterParser initialized", slogger.Fields{
		"max_source_size": config.MaxSourceSize,
		"default_timeout": config.DefaultTimeout.String(),
		"cache_size":      config.CacheSize,
		"max_concurrency": config.MaxConcurrency,
		"metrics_enabled": config.EnableMetrics,
	})

	return parser, nil
}

// AddLanguageSupport adds support for a specific language with its grammar
// This is called by concrete implementations to register language support.
func (p *BaseTreeSitterParser) AddLanguageSupport(
	ctx context.Context,
	lang valueobject.Language,
	grammar *tree_sitter.Language,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	langName := lang.Name()

	// Create and configure parser
	parser := tree_sitter.NewParser()
	if parser == nil {
		return fmt.Errorf("failed to create parser for language %s", langName)
	}

	if !parser.SetLanguage(grammar) {
		return fmt.Errorf("failed to set language %s", langName)
	}

	p.parsers[langName] = parser
	p.languages[langName] = grammar
	p.supportedLanguages = append(p.supportedLanguages, lang)

	slogger.Info(ctx, "Added language support", slogger.Fields{
		"language":   langName,
		"extensions": lang.Extensions(),
	})

	return nil
}

// GetSupportedLanguages returns all languages supported by this parser.
func (p *BaseTreeSitterParser) GetSupportedLanguages(ctx context.Context) ([]valueobject.Language, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.config.EnableLogging {
		slogger.Debug(ctx, "Getting supported languages", slogger.Fields{
			"language_count": len(p.supportedLanguages),
		})
	}

	return p.supportedLanguages, nil
}

// ValidateGrammar validates that a grammar is properly loaded for a language.
func (p *BaseTreeSitterParser) ValidateGrammar(ctx context.Context, language valueobject.Language) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	langName := language.Name()
	grammar, exists := p.languages[langName]
	if !exists {
		return fmt.Errorf("language %s is not supported", langName)
	}

	if grammar == nil {
		return fmt.Errorf("grammar for language %s is not loaded", langName)
	}

	slogger.Debug(ctx, "Grammar validation successful", slogger.Fields{
		"language": langName,
	})

	return nil
}

// GetParserVersion returns version information about the tree-sitter parser.
func (p *BaseTreeSitterParser) GetParserVersion(ctx context.Context) (*ParserVersionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	langVersions := make([]LanguageVersionInfo, 0, len(p.supportedLanguages))
	for _, lang := range p.supportedLanguages {
		langVersions = append(langVersions, LanguageVersionInfo{
			Name:           lang.Name(),
			GrammarVersion: "unknown", // Would be set by concrete implementations
			ParserVersion:  "unknown", // Would be set by concrete implementations
			LoadedAt:       time.Now(),
		})
	}

	versionInfo := &ParserVersionInfo{
		TreeSitterVersion:  "github.com/alexaandru/go-tree-sitter-bare",
		ParserVersion:      "1.0.0", // Base parser version
		SupportedLanguages: langVersions,
		BuildInfo: map[string]interface{}{
			"go_version":      "go1.24.6",
			"build_time":      time.Now(),
			"cache_size":      p.config.CacheSize,
			"max_concurrency": p.config.MaxConcurrency,
		},
		CreatedAt: time.Now(),
	}

	slogger.Info(ctx, "Retrieved parser version info", slogger.Fields{
		"tree_sitter_version": versionInfo.TreeSitterVersion,
		"supported_languages": len(versionInfo.SupportedLanguages),
	})

	return versionInfo, nil
}

// Cleanup releases all parser resources.
func (p *BaseTreeSitterParser) Cleanup(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	slogger.Info(ctx, "Cleaning up parser resources", slogger.Fields{
		"parser_count":   len(p.parsers),
		"language_count": len(p.languages),
	})

	// Clean up all parsers
	for langName, parser := range p.parsers {
		if parser != nil {
			// Note: go-tree-sitter-bare parsers don't require explicit cleanup
			slogger.Debug(ctx, "Released parser", slogger.Fields{
				"language": langName,
			})
		}
	}

	// Clear maps
	p.parsers = make(map[string]*tree_sitter.Parser)
	p.languages = make(map[string]*tree_sitter.Language)
	p.supportedLanguages = make([]valueobject.Language, 0)

	slogger.Info(ctx, "Parser cleanup completed", slogger.Fields{})

	return nil
}

// initParserMetrics initializes OTEL metrics for parser operations.
func initParserMetrics() (*ParserMetrics, error) {
	meter := otel.Meter("codechunking/treesitter_parser")

	parseOpsTotal, err := meter.Int64Counter(
		"treesitter_parse_operations_total",
		metric.WithDescription("Total number of tree-sitter parse operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse operations counter: %w", err)
	}

	parseDurationHist, err := meter.Float64Histogram(
		"treesitter_parse_duration_seconds",
		metric.WithDescription("Duration of tree-sitter parse operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse duration histogram: %w", err)
	}

	parseErrorsTotal, err := meter.Int64Counter(
		"treesitter_parse_errors_total",
		metric.WithDescription("Total number of tree-sitter parse errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse errors counter: %w", err)
	}

	cacheHits, err := meter.Int64Counter(
		"treesitter_parser_cache_hits_total",
		metric.WithDescription("Total number of parser cache hits"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache hits counter: %w", err)
	}

	cacheMisses, err := meter.Int64Counter(
		"treesitter_parser_cache_misses_total",
		metric.WithDescription("Total number of parser cache misses"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache misses counter: %w", err)
	}

	activeOpsGauge, err := meter.Int64Gauge(
		"treesitter_active_parsing_operations",
		metric.WithDescription("Number of active parsing operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create active operations gauge: %w", err)
	}

	return &ParserMetrics{
		parseOperationsTotal:    parseOpsTotal,
		parseDurationHistogram:  parseDurationHist,
		parseErrorsTotal:        parseErrorsTotal,
		parserCacheHits:         cacheHits,
		parserCacheMisses:       cacheMisses,
		activeParsingOperations: activeOpsGauge,
	}, nil
}

// RecordParseOperation records metrics for a parse operation.
// This method is public so concrete parser implementations can use it.
func (m *ParserMetrics) RecordParseOperation(
	ctx context.Context,
	language string,
	success bool,
	duration time.Duration,
) {
	if m == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("language", language),
		attribute.Bool("success", success),
	}

	m.parseOperationsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.parseDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if !success {
		m.parseErrorsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// GetMetrics returns the metrics instance for concrete implementations to use.
func (p *BaseTreeSitterParser) GetMetrics() *ParserMetrics {
	return p.metrics
}

// DefaultParserConfig returns a default configuration for the base parser.
func DefaultParserConfig() ParserConfig {
	return ParserConfig{
		MaxSourceSize:  10 * 1024 * 1024, // 10MB
		DefaultTimeout: 30 * time.Second, // 30 seconds
		CacheSize:      100,              // 100 parsers
		EnableMetrics:  true,
		EnableLogging:  true,
		LogLevel:       "INFO",
		MaxConcurrency: 10,
	}
}
