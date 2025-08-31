package valueobject

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ParseResult represents the result of a parsing operation as a value object.
type ParseResult struct {
	success      bool
	parseTree    *ParseTree
	errors       []ParseError
	warnings     []ParseWarning
	duration     time.Duration
	statistics   ParseStatistics
	metadata     map[string]interface{}
	parseOptions ParseOptions
	createdAt    time.Time
	mu           sync.RWMutex
	metrics      *parseResultMetrics
}

// parseResultMetrics holds OTEL metrics for ParseResult operations.
type parseResultMetrics struct {
	parseResultCounter     metric.Int64Counter
	parseSuccessRate       metric.Float64Histogram
	errorCountHistogram    metric.Int64Histogram
	warningCountHistogram  metric.Int64Histogram
	parseDurationHistogram metric.Float64Histogram
}

// ParseError represents an error that occurred during parsing.
type ParseError struct {
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	Position    Position  `json:"position,omitempty"`
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
	Position   Position  `json:"position,omitempty"`
	ByteOffset uint32    `json:"byte_offset,omitempty"`
	Code       string    `json:"code,omitempty"`
	Context    string    `json:"context,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// ParseStatistics contains statistics about the parsing operation.
type ParseStatistics struct {
	TotalNodes       int           `json:"total_nodes"`
	ErrorNodes       int           `json:"error_nodes"`
	MaxDepth         int           `json:"max_depth"`
	ParseDuration    time.Duration `json:"parse_duration"`
	MemoryUsed       int64         `json:"memory_used"`
	BytesProcessed   int64         `json:"bytes_processed"`
	LinesProcessed   int           `json:"lines_processed"`
	TokensProcessed  int           `json:"tokens_processed"`
	RecoveredErrors  int           `json:"recovered_errors"`
	PerformanceScore float64       `json:"performance_score"`
}

// ParseOptions configures the parsing operation.
type ParseOptions struct {
	Language           Language               `json:"language"`
	RecoveryMode       bool                   `json:"recovery_mode"`
	IncludeComments    bool                   `json:"include_comments"`
	IncludeWhitespace  bool                   `json:"include_whitespace"`
	MaxErrorCount      int                    `json:"max_error_count"`
	MaxDepth           int                    `json:"max_depth"`
	TimeoutDuration    time.Duration          `json:"timeout_duration"`
	CustomGrammarRules map[string]interface{} `json:"custom_grammar_rules,omitempty"`
	PerformanceOptions *PerformanceOptions    `json:"performance_options,omitempty"`
	ValidationOptions  *ValidationOptions     `json:"validation_options,omitempty"`
	OutputOptions      *OutputOptions         `json:"output_options,omitempty"`
}

// PerformanceOptions configures performance-related parsing settings.
type PerformanceOptions struct {
	EnableCaching      bool          `json:"enable_caching"`
	CacheSize          int           `json:"cache_size"`
	ParallelProcessing bool          `json:"parallel_processing"`
	WorkerCount        int           `json:"worker_count"`
	MemoryLimit        int64         `json:"memory_limit"`
	CPULimit           time.Duration `json:"cpu_limit"`
}

// ValidationOptions configures validation during parsing.
type ValidationOptions struct {
	StrictMode        bool     `json:"strict_mode"`
	ValidateStructure bool     `json:"validate_structure"`
	ValidateSemantics bool     `json:"validate_semantics"`
	ValidateSyntax    bool     `json:"validate_syntax"`
	IgnoreWarnings    bool     `json:"ignore_warnings"`
	RequiredFeatures  []string `json:"required_features,omitempty"`
	ForbiddenFeatures []string `json:"forbidden_features,omitempty"`
}

// OutputOptions configures the output format and content.
type OutputOptions struct {
	IncludeSourceText bool     `json:"include_source_text"`
	IncludePositions  bool     `json:"include_positions"`
	IncludeMetadata   bool     `json:"include_metadata"`
	CompressOutput    bool     `json:"compress_output"`
	OutputFormat      string   `json:"output_format"`
	ExportFormats     []string `json:"export_formats,omitempty"`
}

// NewParseResult creates a new ParseResult with validation.
func NewParseResult(
	ctx context.Context,
	success bool,
	parseTree *ParseTree,
	parseErrors []ParseError,
	warnings []ParseWarning,
	duration time.Duration,
	options ParseOptions,
) (*ParseResult, error) {
	if duration < 0 {
		slogger.Error(ctx, "Invalid parse duration", slogger.Fields{
			"duration": duration.String(),
			"language": options.Language.Name(),
		})
		return nil, errors.New("parse duration cannot be negative")
	}

	if success && parseTree == nil {
		slogger.Error(ctx, "Successful parse result missing parse tree", slogger.Fields{
			"language": options.Language.Name(),
		})
		return nil, errors.New("successful parse result must have a parse tree")
	}

	if !success && len(parseErrors) == 0 {
		slogger.Error(ctx, "Failed parse result missing errors", slogger.Fields{
			"language": options.Language.Name(),
		})
		return nil, errors.New("failed parse result must have at least one error")
	}

	// Initialize metrics
	metrics, err := initParseResultMetrics()
	if err != nil {
		slogger.Warn(ctx, "Failed to initialize parse result metrics", slogger.Fields{
			"error":    err.Error(),
			"language": options.Language.Name(),
		})
	}

	// Calculate statistics
	var stats ParseStatistics
	if parseTree != nil {
		stats.TotalNodes = parseTree.GetTotalNodeCount()
		stats.MaxDepth = parseTree.GetTreeDepth()
		stats.ParseDuration = duration

		// Count error nodes
		if hasErrors, _ := parseTree.HasSyntaxErrors(); hasErrors {
			stats.ErrorNodes = countErrorNodes(parseTree.RootNode())
		}
	}

	stats.RecoveredErrors = len(parseErrors)

	result := ParseResult{
		success:      success,
		parseTree:    parseTree,
		errors:       parseErrors,
		warnings:     warnings,
		duration:     duration,
		statistics:   stats,
		metadata:     make(map[string]interface{}),
		parseOptions: options,
		createdAt:    time.Now(),
		metrics:      metrics,
	}

	// Record metrics
	if metrics != nil {
		metrics.recordParseResult(ctx, options.Language.Name(), success, len(parseErrors), len(warnings), duration)
	}

	slogger.Info(ctx, "ParseResult created", slogger.Fields{
		"success":       success,
		"language":      options.Language.Name(),
		"duration":      duration.String(),
		"error_count":   len(parseErrors),
		"warning_count": len(warnings),
		"node_count":    stats.TotalNodes,
	})

	return &result, nil
}

// countErrorNodes recursively counts error nodes in the parse tree.
func countErrorNodes(node *ParseNode) int {
	if node == nil {
		return 0
	}

	count := 0
	if node.IsErrorNode() {
		count++
	}

	for _, child := range node.Children {
		count += countErrorNodes(child)
	}

	return count
}

// IsSuccessful returns true if the parsing operation was successful.
func (pr *ParseResult) IsSuccessful() bool {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.success
}

// Duration returns the time taken for the parsing operation.
func (pr *ParseResult) Duration() time.Duration {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.duration
}

// ErrorCount returns the number of errors encountered.
func (pr *ParseResult) ErrorCount() int {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return len(pr.errors)
}

// WarningCount returns the number of warnings encountered.
func (pr *ParseResult) WarningCount() int {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return len(pr.warnings)
}

// HasErrors returns true if there are parsing errors.
func (pr *ParseResult) HasErrors() bool {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return len(pr.errors) > 0
}

// HasWarnings returns true if there are parsing warnings.
func (pr *ParseResult) HasWarnings() bool {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return len(pr.warnings) > 0
}

// ParseTree returns the parse tree if parsing was successful.
func (pr *ParseResult) ParseTree() *ParseTree {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.parseTree
}

// GetStatistics returns the parsing statistics.
func (pr *ParseResult) GetStatistics() ParseStatistics {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.statistics
}

// GetErrorsByType returns errors of a specific type.
func (pr *ParseResult) GetErrorsByType(errorType string) []ParseError {
	var result []ParseError
	for _, err := range pr.errors {
		if err.Type == errorType {
			result = append(result, err)
		}
	}
	return result
}

// GetErrorsByPosition returns errors at a specific position.
func (pr *ParseResult) GetErrorsByPosition(pos Position) []ParseError {
	var result []ParseError
	for _, err := range pr.errors {
		if err.Position.Row == pos.Row && err.Position.Column == pos.Column {
			result = append(result, err)
		}
	}
	return result
}

// GetRecoverableErrors returns only recoverable errors.
func (pr *ParseResult) GetRecoverableErrors() []ParseError {
	var result []ParseError
	for _, err := range pr.errors {
		if err.Recoverable {
			result = append(result, err)
		}
	}
	return result
}

// GetCriticalErrors returns only critical (non-recoverable) errors.
func (pr *ParseResult) GetCriticalErrors() []ParseError {
	var result []ParseError
	for _, err := range pr.errors {
		if !err.Recoverable {
			result = append(result, err)
		}
	}
	return result
}

// FormatErrors returns a formatted string of all errors.
func (pr *ParseResult) FormatErrors() string {
	if len(pr.errors) == 0 {
		return "No errors"
	}

	var formatted strings.Builder
	formatted.WriteString("Parse Errors:\n")
	for i, err := range pr.errors {
		formatted.WriteString(fmt.Sprintf("%d. %s: %s (Line: %d, Column: %d)\n",
			i+1, err.Type, err.Message, err.Position.Row, err.Position.Column))
	}
	return formatted.String()
}

// ToJSON serializes the parse result to JSON.
func (pr *ParseResult) ToJSON() (string, error) {
	return fmt.Sprintf(`{
  "success": %t,
  "language": "%s",
  "statistics": {
    "total_nodes": %d,
    "error_nodes": %d,
    "max_depth": %d
  },
  "duration": "%s",
  "error_count": %d,
  "warning_count": %d
}`, pr.success, pr.parseOptions.Language.Name(), pr.statistics.TotalNodes, pr.statistics.ErrorNodes,
		pr.statistics.MaxDepth, pr.duration.String(), len(pr.errors), len(pr.warnings)), nil
}

// ExportSummary exports a summary report of the parse result.
func (pr *ParseResult) ExportSummary() (string, error) {
	var summary strings.Builder
	summary.WriteString("Parse Result Summary\n")
	summary.WriteString("===================\n")
	summary.WriteString(fmt.Sprintf("Language: %s\n", pr.parseOptions.Language.Name()))

	if pr.success {
		summary.WriteString("Status: Success\n")
	} else {
		summary.WriteString("Status: Failed\n")
	}

	summary.WriteString(fmt.Sprintf("Duration: %v\n", pr.duration))
	summary.WriteString(fmt.Sprintf("Errors: %d\n", len(pr.errors)))
	summary.WriteString(fmt.Sprintf("Warnings: %d\n", len(pr.warnings)))

	return summary.String(), nil
}

// ExportDetailedReport exports a detailed report of the parse result.
func (pr *ParseResult) ExportDetailedReport() (string, error) {
	var report strings.Builder
	report.WriteString("Detailed Parse Report\n")
	report.WriteString("====================\n")

	report.WriteString("Parse Tree Structure\n")
	report.WriteString("-------------------\n")
	if pr.parseTree != nil {
		report.WriteString(fmt.Sprintf("Total Nodes: %d\n", pr.statistics.TotalNodes))
		report.WriteString(fmt.Sprintf("Max Depth: %d\n", pr.statistics.MaxDepth))
	}

	report.WriteString("\nPerformance Statistics\n")
	report.WriteString("---------------------\n")
	report.WriteString(fmt.Sprintf("Parse Duration: %v\n", pr.statistics.ParseDuration))
	report.WriteString(fmt.Sprintf("Memory Used: %d bytes\n", pr.statistics.MemoryUsed))

	return report.String(), nil
}

// initParseResultMetrics initializes OTEL metrics for ParseResult operations.
func initParseResultMetrics() (*parseResultMetrics, error) {
	meter := otel.Meter("codechunking/parse_result")

	parseResultCounter, err := meter.Int64Counter(
		"parse_result_operations_total",
		metric.WithDescription("Total number of parse result operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse result counter: %w", err)
	}

	successRate, err := meter.Float64Histogram(
		"parse_result_success_rate",
		metric.WithDescription("Success rate of parse operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create success rate histogram: %w", err)
	}

	errorCountHist, err := meter.Int64Histogram(
		"parse_result_error_count",
		metric.WithDescription("Number of parse errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create error count histogram: %w", err)
	}

	warningCountHist, err := meter.Int64Histogram(
		"parse_result_warning_count",
		metric.WithDescription("Number of parse warnings"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create warning count histogram: %w", err)
	}

	durationHist, err := meter.Float64Histogram(
		"parse_result_duration_seconds",
		metric.WithDescription("Duration of parse operations in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create duration histogram: %w", err)
	}

	return &parseResultMetrics{
		parseResultCounter:     parseResultCounter,
		parseSuccessRate:       successRate,
		errorCountHistogram:    errorCountHist,
		warningCountHistogram:  warningCountHist,
		parseDurationHistogram: durationHist,
	}, nil
}

// recordParseResult records metrics for a parse result operation.
func (m *parseResultMetrics) recordParseResult(
	ctx context.Context,
	language string,
	success bool,
	errorCount, warningCount int,
	duration time.Duration,
) {
	attrs := []attribute.KeyValue{
		attribute.String("language", language),
		attribute.Bool("success", success),
	}

	m.parseResultCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	var successValue float64
	if success {
		successValue = 1.0
	}
	m.parseSuccessRate.Record(ctx, successValue, metric.WithAttributes(attrs...))

	m.errorCountHistogram.Record(ctx, int64(errorCount), metric.WithAttributes(attrs...))
	m.warningCountHistogram.Record(ctx, int64(warningCount), metric.WithAttributes(attrs...))
	m.parseDurationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// CreatedAt returns when the parse result was created.
func (pr *ParseResult) CreatedAt() time.Time {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.createdAt
}
