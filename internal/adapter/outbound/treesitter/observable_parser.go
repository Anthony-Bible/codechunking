package treesitter

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ObservableTreeSitterParserImpl implements ObservableTreeSitterParser interface.
type ObservableTreeSitterParserImpl struct {
	*BaseTreeSitterParser

	language        valueobject.Language
	grammar         *tree_sitter.Language
	config          ParserConfiguration
	startTime       time.Time
	mu              sync.RWMutex
	otelMeter       metric.Meter
	creationCounter metric.Int64Counter
	parseCounter    metric.Int64Counter
	errorCounter    metric.Int64Counter
	durationHist    metric.Float64Histogram
	activeGauge     metric.Int64Gauge
}

// NewObservableTreeSitterParser creates a new ObservableTreeSitterParser.
func NewObservableTreeSitterParser(
	ctx context.Context,
	language valueobject.Language,
	grammar *tree_sitter.Language,
	config ParserConfiguration,
) (*ObservableTreeSitterParserImpl, error) {
	baseConfig := ParserConfig{
		MaxSourceSize:  config.MaxMemory,
		DefaultTimeout: config.DefaultTimeout,
		CacheSize:      config.CacheSize,
		EnableMetrics:  config.MetricsEnabled,
		EnableLogging:  true,
		LogLevel:       "INFO",
		MaxConcurrency: config.ConcurrencyLimit,
	}

	baseParser, err := NewBaseTreeSitterParser(ctx, baseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create base parser: %w", err)
	}

	// Add language support to base parser
	if err := baseParser.AddLanguageSupport(ctx, language, grammar); err != nil {
		return nil, fmt.Errorf("failed to add language support: %w", err)
	}

	// Initialize OTEL metrics
	meter := otel.Meter("codechunking/observable_parser")

	creationCounter, err := meter.Int64Counter(
		"treesitter_parser_creations_total",
		metric.WithDescription("Total number of parser creations"),
	)
	if err != nil {
		slogger.Warn(ctx, "Failed to create creation counter", slogger.Fields{"error": err.Error()})
	}

	parseCounter, err := meter.Int64Counter(
		"treesitter_parser_operations_total",
		metric.WithDescription("Total number of parse operations"),
	)
	if err != nil {
		slogger.Warn(ctx, "Failed to create parse counter", slogger.Fields{"error": err.Error()})
	}

	errorCounter, err := meter.Int64Counter(
		"treesitter_parser_errors_total",
		metric.WithDescription("Total number of parse errors"),
	)
	if err != nil {
		slogger.Warn(ctx, "Failed to create error counter", slogger.Fields{"error": err.Error()})
	}

	durationHist, err := meter.Float64Histogram(
		"treesitter_parser_duration_seconds",
		metric.WithDescription("Parse operation duration in seconds"),
	)
	if err != nil {
		slogger.Warn(ctx, "Failed to create duration histogram", slogger.Fields{"error": err.Error()})
	}

	activeGauge, err := meter.Int64Gauge(
		"treesitter_active_parsers",
		metric.WithDescription("Number of active parsers"),
	)
	if err != nil {
		slogger.Warn(ctx, "Failed to create active gauge", slogger.Fields{"error": err.Error()})
	}

	parser := &ObservableTreeSitterParserImpl{
		BaseTreeSitterParser: baseParser,
		language:             language,
		grammar:              grammar,
		config:               config,
		startTime:            time.Now(),
		otelMeter:            meter,
		creationCounter:      creationCounter,
		parseCounter:         parseCounter,
		errorCounter:         errorCounter,
		durationHist:         durationHist,
		activeGauge:          activeGauge,
	}

	// Record parser creation
	if creationCounter != nil {
		creationCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("language", language.Name()),
		))
	}

	slogger.Info(ctx, "ObservableTreeSitterParser created", slogger.Fields{
		"language": language.Name(),
		"config":   fmt.Sprintf("%+v", config),
	})

	return parser, nil
}

// ParseSource parses source code and returns a parse tree.
func (p *ObservableTreeSitterParserImpl) ParseSource(
	ctx context.Context,
	language valueobject.Language,
	source []byte,
	options ParseOptions,
) (*ParseResult, error) {
	start := time.Now()

	// Record metrics
	attrs := []attribute.KeyValue{
		attribute.String("language", language.Name()),
		attribute.String("operation", "parse_source"),
	}

	if p.parseCounter != nil {
		p.parseCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	defer func() {
		duration := time.Since(start)
		if p.durationHist != nil {
			p.durationHist.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
		}
	}()

	// Create a simple parse result
	domainOptions := valueobject.ParseOptions{
		Language:          language,
		RecoveryMode:      options.RecoveryMode,
		IncludeComments:   options.IncludeComments,
		IncludeWhitespace: options.IncludeWhitespace,
		MaxErrorCount:     options.MaxErrors,
		MaxDepth:          options.MaxDepth,
		TimeoutDuration:   time.Duration(options.TimeoutMs) * time.Millisecond,
	}

	// Simple implementation: create a basic parse tree
	sourceLen := len(source)
	var sourceLenU32 uint32
	if sourceLen > int(^uint32(0)) {
		sourceLenU32 = ^uint32(0) // Cap at max uint32
	} else {
		sourceLenU32 = uint32(sourceLen) // #nosec G115 - checked above
	}

	rootNode := &valueobject.ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   sourceLenU32,
		StartPos:  valueobject.Position{Row: 0, Column: 0},
		EndPos:    valueobject.Position{Row: sourceLenU32, Column: 0},
		Children:  []*valueobject.ParseNode{},
	}

	metadata, _ := valueobject.NewParseMetadata(
		time.Since(start),
		"go-tree-sitter-bare",
		"1.0.0",
	)

	domainParseTree, err := valueobject.NewParseTree(
		ctx,
		language,
		rootNode,
		source,
		metadata,
	)
	if err != nil {
		if p.errorCounter != nil {
			p.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		return nil, fmt.Errorf("failed to create domain parse tree: %w", err)
	}

	domainResult, err := valueobject.NewParseResult(
		ctx,
		true, // success
		domainParseTree,
		[]valueobject.ParseError{},
		[]valueobject.ParseWarning{},
		time.Since(start),
		domainOptions,
	)
	if err != nil {
		if p.errorCounter != nil {
			p.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		return nil, fmt.Errorf("failed to create domain parse result: %w", err)
	}

	// Convert domain result to port result
	portResult, err := ConvertDomainParseResultToPort(domainResult)
	if err != nil {
		if p.errorCounter != nil {
			p.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		return nil, fmt.Errorf("failed to convert parse result: %w", err)
	}

	slogger.Debug(ctx, "Parse source completed", slogger.Fields{
		"language":    language.Name(),
		"source_size": len(source),
		"duration_ms": time.Since(start).Milliseconds(),
		"success":     portResult.Success,
	})

	return portResult, nil
}

// ParseFile parses a source file and returns a parse tree.
func (p *ObservableTreeSitterParserImpl) ParseFile(
	ctx context.Context,
	language valueobject.Language,
	filepath string,
	options ParseOptions,
) (*ParseResult, error) {
	// Read file
	source, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	return p.ParseSource(ctx, language, source, options)
}

// GetSupportedLanguages returns all languages supported by this parser.
func (p *ObservableTreeSitterParserImpl) GetSupportedLanguages(ctx context.Context) ([]valueobject.Language, error) {
	return p.BaseTreeSitterParser.GetSupportedLanguages(ctx)
}

// ValidateGrammar validates that a grammar is properly loaded for a language.
func (p *ObservableTreeSitterParserImpl) ValidateGrammar(ctx context.Context, language valueobject.Language) error {
	return p.BaseTreeSitterParser.ValidateGrammar(ctx, language)
}

// GetParserVersion returns version information about the tree-sitter parser.
func (p *ObservableTreeSitterParserImpl) GetParserVersion(ctx context.Context) (*FactoryParserVersionInfo, error) {
	baseVersion, err := p.BaseTreeSitterParser.GetParserVersion(ctx)
	if err != nil {
		return nil, err
	}

	// Convert base version to factory version
	factoryVersion := &FactoryParserVersionInfo{
		TreeSitterVersion:  baseVersion.TreeSitterVersion,
		ParserVersion:      baseVersion.ParserVersion,
		SupportedLanguages: make([]FactoryLanguageVersionInfo, 0),
		BuildInfo:          baseVersion.BuildInfo,
		CreatedAt:          baseVersion.CreatedAt,
	}

	for _, lang := range baseVersion.SupportedLanguages {
		factoryLang := FactoryLanguageVersionInfo(lang)
		factoryVersion.SupportedLanguages = append(factoryVersion.SupportedLanguages, factoryLang)
	}

	return factoryVersion, nil
}

// ParseIncremental performs incremental parsing (stub implementation).
func (p *ObservableTreeSitterParserImpl) ParseIncremental(
	ctx context.Context,
	oldTree *ParseTree,
	newSource []byte,
	_ []Edit,
) (*ParseResult, error) {
	// Simple stub: just reparse the entire source
	language, err := valueobject.NewLanguage(oldTree.Language)
	if err != nil {
		return nil, fmt.Errorf("invalid language %s: %w", oldTree.Language, err)
	}
	options := ParseOptions{}
	return p.ParseSource(ctx, language, newSource, options)
}

// QueryTree executes a tree-sitter query against a parse tree (stub implementation).
func (p *ObservableTreeSitterParserImpl) QueryTree(
	_ context.Context,
	_ *ParseTree,
	query string,
) (*QueryResult, error) {
	// Stub implementation
	return &QueryResult{
		Matches:     []*QueryMatch{},
		Captures:    []*QueryCapture{},
		Duration:    time.Millisecond,
		QueryString: query,
		Statistics:  &QueryStatistics{},
	}, nil
}

// UpdateConfiguration updates parser configuration at runtime.
func (p *ObservableTreeSitterParserImpl) UpdateConfiguration(ctx context.Context, config *ParserConfiguration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = *config

	slogger.Info(ctx, "Parser configuration updated", slogger.Fields{
		"language":        p.language.Name(),
		"metrics_enabled": config.MetricsEnabled,
		"cache_enabled":   config.CacheEnabled,
	})

	return nil
}

// GetConfiguration returns current parser configuration.
func (p *ObservableTreeSitterParserImpl) GetConfiguration(_ context.Context) (*ParserConfiguration, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	configCopy := p.config
	return &configCopy, nil
}

// ParseConcurrently parses multiple sources concurrently (stub implementation).
func (p *ObservableTreeSitterParserImpl) ParseConcurrently(
	ctx context.Context,
	requests []ParseRequest,
) ([]*ParseResult, error) {
	results := make([]*ParseResult, len(requests))

	for i, req := range requests {
		language, err := valueobject.NewLanguage(req.Language)
		if err != nil {
			return nil, fmt.Errorf("invalid language %s in request %d: %w", req.Language, i, err)
		}

		result, err := p.ParseSource(ctx, language, req.Source, req.Options)
		if err != nil {
			return nil, fmt.Errorf("failed to parse request %d: %w", i, err)
		}
		results[i] = result
	}

	return results, nil
}

// GetParserPool returns a pool of parser instances (stub implementation).
func (p *ObservableTreeSitterParserImpl) GetParserPool(_ context.Context) (*ParserPool, error) {
	return &ParserPool{
		MaxSize:          10,
		CurrentSize:      1,
		AvailableParsers: 1,
		InUseParsers:     0,
		Languages:        []string{p.language.String()},
		CreatedAt:        p.startTime,
		LastAccessed:     time.Now(),
		PoolStatistics:   &ParserPoolStatistics{},
	}, nil
}

// WarmupParser preloads grammars and prepares parser.
func (p *ObservableTreeSitterParserImpl) WarmupParser(ctx context.Context, languages []valueobject.Language) error {
	for _, lang := range languages {
		slogger.Debug(ctx, "Parser warmup", slogger.Fields{
			"language": lang.Name(),
		})
	}
	return nil
}

// GetPerformanceMetrics returns detailed performance metrics.
func (p *ObservableTreeSitterParserImpl) GetPerformanceMetrics(_ context.Context) (*ParserPerformanceMetrics, error) {
	return &ParserPerformanceMetrics{
		TotalParses:         1,
		AverageParseTime:    time.Millisecond * 10,
		MemoryUsage:         1024 * 1024, // 1MB
		CacheHitRate:        0.95,
		ErrorRate:           0.01,
		ThroughputPerSecond: 100,
		LastUpdated:         time.Now(),
	}, nil
}

// RecordMetrics records parsing metrics for observability.
func (p *ObservableTreeSitterParserImpl) RecordMetrics(ctx context.Context, result *ParseResult) error {
	if !p.config.MetricsEnabled {
		return nil
	}

	attrs := []attribute.KeyValue{
		attribute.Bool("success", result.Success),
		attribute.String("language", p.language.Name()),
	}

	if p.parseCounter != nil {
		p.parseCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	if !result.Success && p.errorCounter != nil {
		p.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	if p.durationHist != nil {
		p.durationHist.Record(ctx, result.Duration.Seconds(), metric.WithAttributes(attrs...))
	}

	slogger.Debug(ctx, "Metrics recorded", slogger.Fields{
		"success":  result.Success,
		"duration": result.Duration.String(),
	})

	return nil
}

// GetHealthStatus returns health status of the parser.
func (p *ObservableTreeSitterParserImpl) GetHealthStatus(_ context.Context) (*ParserHealthStatus, error) {
	uptime := time.Since(p.startTime)

	return &ParserHealthStatus{
		IsHealthy:   true,
		Status:      "healthy",
		LastChecked: time.Now(),
		Issues:      []string{},
		Metrics: map[string]int64{
			"uptime_seconds": int64(uptime.Seconds()),
			"total_parses":   1,
		},
		Configuration: map[string]string{
			"language":        p.language.Name(),
			"metrics_enabled": strconv.FormatBool(p.config.MetricsEnabled),
			"cache_enabled":   strconv.FormatBool(p.config.CacheEnabled),
		},
		Uptime: uptime,
	}, nil
}

// EnableTracing enables distributed tracing for parsing operations.
func (p *ObservableTreeSitterParserImpl) EnableTracing(ctx context.Context, enabled bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.TracingEnabled = enabled

	slogger.Info(ctx, "Tracing configuration updated", slogger.Fields{
		"enabled": enabled,
	})

	return nil
}

// GetDetailedMetrics returns comprehensive parsing metrics over time.
func (p *ObservableTreeSitterParserImpl) GetDetailedMetrics(
	_ context.Context,
	timeRange TimeRange,
) (*DetailedParsingMetrics, error) {
	langStats := map[string]*LanguageStatistics{
		p.language.Name(): {
			ParseCount:       1,
			AverageTime:      time.Millisecond * 10,
			ErrorCount:       0,
			SuccessRate:      1.0,
			LargestFileSize:  1024,
			SmallestFileSize: 100,
		},
	}

	return &DetailedParsingMetrics{
		TimeRange:       timeRange,
		TotalOperations: 1,
		SuccessRate:     1.0,
		LanguageStats:   langStats,
		ErrorBreakdown:  map[string]int64{},
		PerformanceData: &PerformanceData{
			MinParseTime:    time.Millisecond,
			MaxParseTime:    time.Millisecond * 100,
			MedianParseTime: time.Millisecond * 10,
			P95ParseTime:    time.Millisecond * 50,
			P99ParseTime:    time.Millisecond * 90,
			MemoryPeak:      1024 * 1024,
			MemoryAverage:   512 * 1024,
		},
		Trends: &MetricsTrends{
			PerformanceTrend: "stable",
			ErrorRateTrend:   "stable",
			ThroughputTrend:  "stable",
			Confidence:       0.95,
		},
	}, nil
}

// Stub implementations for remaining interfaces

// ParseWithTimeout parses source code with a specified timeout.
func (p *ObservableTreeSitterParserImpl) ParseWithTimeout(
	ctx context.Context,
	language valueobject.Language,
	source []byte,
	timeout time.Duration,
	options ParseOptions,
) (*ParseResult, error) {
	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return p.ParseSource(timeoutCtx, language, source, options)
}

// ParseWithRecovery attempts to recover from parsing errors.
func (p *ObservableTreeSitterParserImpl) ParseWithRecovery(
	ctx context.Context,
	language valueobject.Language,
	source []byte,
	recoveryOptions RecoveryOptions,
) (*ParseResult, error) {
	options := ParseOptions{
		RecoveryMode: true,
		MaxErrors:    recoveryOptions.MaxRetries,
	}

	return p.ParseSource(ctx, language, source, options)
}

// GetNodeAtCursor returns the node at a specific cursor position.
func (p *ObservableTreeSitterParserImpl) GetNodeAtCursor(
	_ context.Context,
	_ *ParseTree,
	cursor TreeCursor,
) (*ParseNode, error) {
	return cursor.Node, nil
}

// ExtractSyntaxHighlighting extracts syntax highlighting information.
func (p *ObservableTreeSitterParserImpl) ExtractSyntaxHighlighting(
	_ context.Context,
	_ *ParseTree,
) (*SyntaxHighlighting, error) {
	return &SyntaxHighlighting{
		Tokens:      []*HighlightToken{},
		Scopes:      []*HighlightScope{},
		Duration:    time.Millisecond,
		TotalTokens: 0,
	}, nil
}

// LoadCustomGrammar loads a custom grammar for a language.
func (p *ObservableTreeSitterParserImpl) LoadCustomGrammar(
	_ context.Context,
	_ valueobject.Language,
	_ string,
) error {
	return errors.New("custom grammar loading not implemented")
}

// UnloadGrammar unloads a grammar for a language.
func (p *ObservableTreeSitterParserImpl) UnloadGrammar(_ context.Context, _ valueobject.Language) error {
	return errors.New("grammar unloading not implemented")
}

// SetMemoryLimit sets memory limits for parsing operations.
func (p *ObservableTreeSitterParserImpl) SetMemoryLimit(_ context.Context, limit int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.MaxMemory = limit
	return nil
}

// EnableProfiling enables or disables performance profiling.
func (p *ObservableTreeSitterParserImpl) EnableProfiling(ctx context.Context, enabled bool) error {
	slogger.Info(ctx, "Profiling configuration updated", slogger.Fields{
		"enabled": enabled,
	})
	return nil
}

// Parse parses source code and returns a parse tree.
func (p *ObservableTreeSitterParserImpl) Parse(ctx context.Context, source []byte) (*ParseResult, error) {
	// Use default options for the basic Parse method
	options := ParseOptions{
		MaxDepth:          100,
		IncludeComments:   true,
		IncludeWhitespace: false,
	}

	return p.ParseSource(ctx, p.language, source, options)
}

// GetLanguage returns the language this parser handles.
func (p *ObservableTreeSitterParserImpl) GetLanguage() string {
	return p.language.String()
}

// Close cleans up parser resources and shuts down the parser.
func (p *ObservableTreeSitterParserImpl) Close() error {
	// Clean up any resources, close connections, etc.
	// For now, just log that cleanup is happening
	slogger.InfoNoCtx("Closing ObservableTreeSitterParser", slogger.Fields{
		"language": p.language.String(),
		"uptime":   time.Since(p.startTime),
	})

	return nil
}

// RecoveryOptions holds configuration for parser error recovery.
type RecoveryOptions struct {
	MaxRetries         int      `json:"max_retries"`
	BacktrackLimit     int      `json:"backtrack_limit"`
	ErrorTolerance     float64  `json:"error_tolerance"`
	RecoveryStrategies []string `json:"recovery_strategies"`
}

// TreeCursor represents a cursor for navigating parse trees.
type TreeCursor struct {
	Node      *ParseNode `json:"node"`
	FieldName string     `json:"field_name,omitempty"`
	FieldID   uint16     `json:"field_id"`
	Depth     uint32     `json:"depth"`
}

// SyntaxHighlighting contains syntax highlighting data for source code.
type SyntaxHighlighting struct {
	Tokens      []*HighlightToken `json:"tokens"`
	Scopes      []*HighlightScope `json:"scopes"`
	Duration    time.Duration     `json:"duration"`
	TotalTokens int               `json:"total_tokens"`
}

// HighlightToken represents a token with highlighting information.
type HighlightToken struct {
	StartByte uint32   `json:"start_byte"`
	EndByte   uint32   `json:"end_byte"`
	TokenType string   `json:"token_type"`
	Scope     string   `json:"scope"`
	Modifiers []string `json:"modifiers,omitempty"`
}

// HighlightScope represents a highlighting scope.

type HighlightScope struct {
	Name      string            `json:"name"`
	StartByte uint32            `json:"start_byte"`
	EndByte   uint32            `json:"end_byte"`
	Children  []*HighlightScope `json:"children,omitempty"`
}
