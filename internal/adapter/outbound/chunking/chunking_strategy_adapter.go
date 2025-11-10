package chunking

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ChunkingStrategyAdapter implements the CodeChunkingStrategy interface with production-ready
// algorithms for intelligent code chunking with context preservation.
type ChunkingStrategyAdapter struct {
	functionChunker  *FunctionChunker
	classChunker     *ClassChunker
	sizeBasedChunker *SizeBasedChunker
	contextPreserver *ContextPreserver
	qualityAnalyzer  *QualityAnalyzer

	// Metrics
	chunkingDuration metric.Float64Histogram
	chunksCreated    metric.Int64Counter
	qualityScores    metric.Float64Histogram
	chunkingErrors   metric.Int64Counter
}

// NewChunkingStrategyAdapter creates a new production-ready chunking strategy adapter
// with all necessary components initialized.
func NewChunkingStrategyAdapter() (*ChunkingStrategyAdapter, error) {
	meter := otel.Meter("codechunking/chunking_strategy")

	chunkingDuration, err := meter.Float64Histogram(
		"chunking_duration_seconds",
		metric.WithDescription("Duration of chunking operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunking duration metric: %w", err)
	}

	chunksCreated, err := meter.Int64Counter(
		"chunks_created_total",
		metric.WithDescription("Total number of chunks created"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunks created metric: %w", err)
	}

	qualityScores, err := meter.Float64Histogram(
		"chunk_quality_scores",
		metric.WithDescription("Quality scores of created chunks"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create quality scores metric: %w", err)
	}

	chunkingErrors, err := meter.Int64Counter(
		"chunking_errors_total",
		metric.WithDescription("Total number of chunking errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunking errors metric: %w", err)
	}

	return &ChunkingStrategyAdapter{
		functionChunker:  NewFunctionChunker(),
		classChunker:     NewClassChunker(),
		sizeBasedChunker: NewSizeBasedChunker(),
		contextPreserver: NewContextPreserver(),
		qualityAnalyzer:  NewQualityAnalyzer(),

		chunkingDuration: chunkingDuration,
		chunksCreated:    chunksCreated,
		qualityScores:    qualityScores,
		chunkingErrors:   chunkingErrors,
	}, nil
}

// ChunkCode applies the specified chunking strategy to create optimized code chunks
// with preserved context and intelligent boundaries.
func (c *ChunkingStrategyAdapter) ChunkCode(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	startTime := time.Now()

	slogger.Info(ctx, "Starting code chunking", slogger.Fields{
		"strategy":         string(config.Strategy),
		"semantic_chunks":  len(semanticChunks),
		"context_strategy": string(config.ContextPreservation),
	})

	defer func() {
		duration := time.Since(startTime).Seconds()
		c.chunkingDuration.Record(ctx, duration, metric.WithAttributes(
			attribute.String("strategy", string(config.Strategy)),
		))
	}()

	if len(semanticChunks) == 0 {
		slogger.Info(ctx, "No semantic chunks provided, returning empty result", slogger.Fields{})
		return []outbound.EnhancedCodeChunk{}, nil
	}

	// Validate configuration
	if err := c.validateConfiguration(config); err != nil {
		c.chunkingErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("error_type", "invalid_configuration"),
		))
		return nil, fmt.Errorf("invalid chunking configuration: %w", err)
	}

	var chunks []outbound.EnhancedCodeChunk
	var err error

	// Apply the specified chunking strategy
	switch config.Strategy {
	case outbound.StrategyFunction:
		chunks, err = c.functionChunker.ChunkByFunction(ctx, semanticChunks, config)
	case outbound.StrategyClass:
		chunks, err = c.classChunker.ChunkByClass(ctx, semanticChunks, config)
	case outbound.StrategySizeBased:
		chunks, err = c.sizeBasedChunker.ChunkBySize(ctx, semanticChunks, config)
	case outbound.StrategyHybrid:
		chunks, err = c.chunkByHybrid(ctx, semanticChunks, config)
	case outbound.StrategyAdaptive:
		chunks, err = c.chunkByAdaptive(ctx, semanticChunks, config)
	default:
		err = fmt.Errorf("unsupported chunking strategy: %s", config.Strategy)
	}

	if err != nil {
		c.chunkingErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("error_type", "chunking_failed"),
			attribute.String("strategy", string(config.Strategy)),
		))
		return nil, fmt.Errorf("chunking failed with strategy %s: %w", config.Strategy, err)
	}

	// Apply context preservation if requested
	if config.ContextPreservation != outbound.PreserveMinimal {
		chunks, err = c.contextPreserver.PreserveContext(ctx, chunks, config.ContextPreservation)
		if err != nil {
			c.chunkingErrors.Add(ctx, 1, metric.WithAttributes(
				attribute.String("error_type", "context_preservation_failed"),
			))
			return nil, fmt.Errorf("context preservation failed: %w", err)
		}
	}

	// Calculate quality metrics for all chunks
	for i := range chunks {
		metrics, err := c.qualityAnalyzer.CalculateQualityMetrics(ctx, &chunks[i])
		if err != nil {
			slogger.Error(ctx, "Failed to calculate quality metrics for chunk", slogger.Fields{
				"chunk_id": chunks[i].ID,
				"error":    err.Error(),
			})
			continue
		}
		chunks[i].QualityMetrics = *metrics
		c.qualityScores.Record(ctx, metrics.OverallQuality)
	}

	// Record success metrics
	c.chunksCreated.Add(ctx, int64(len(chunks)), metric.WithAttributes(
		attribute.String("strategy", string(config.Strategy)),
	))

	slogger.Info(ctx, "Code chunking completed successfully", slogger.Fields{
		"strategy":       string(config.Strategy),
		"chunks_created": len(chunks),
		"duration_ms":    time.Since(startTime).Milliseconds(),
	})

	return chunks, nil
}

// GetOptimalChunkSize calculates the optimal chunk size based on content and language characteristics.
func (c *ChunkingStrategyAdapter) GetOptimalChunkSize(
	ctx context.Context,
	language valueobject.Language,
	contentSize int,
) (outbound.ChunkSizeRecommendation, error) {
	slogger.Debug(ctx, "Calculating optimal chunk size", slogger.Fields{
		"language":     language.Name(),
		"content_size": contentSize,
	})

	if contentSize <= 0 {
		return outbound.ChunkSizeRecommendation{}, fmt.Errorf("invalid content size: %d", contentSize)
	}

	// Language-specific size calculations
	var baseSize, minSize, maxSize int
	var confidence float64
	var factors []string

	switch language.Name() {
	case valueobject.LanguageGo:
		baseSize = c.calculateGoOptimalSize(contentSize)
		minSize = 200
		maxSize = 2000
		confidence = 0.9
		factors = []string{"function_boundaries", "package_structure", "interface_definitions"}

	case valueobject.LanguagePython:
		baseSize = c.calculatePythonOptimalSize(contentSize)
		minSize = 150
		maxSize = 1800
		confidence = 0.85
		factors = []string{"class_structure", "function_definitions", "import_statements"}

	case valueobject.LanguageJavaScript, valueobject.LanguageTypeScript:
		baseSize = c.calculateJSOptimalSize(contentSize)
		minSize = 180
		maxSize = 1600
		confidence = 0.8
		factors = []string{"function_expressions", "class_declarations", "module_exports"}

	default:
		// Default algorithm for unknown languages
		baseSize = c.calculateDefaultOptimalSize(contentSize)
		minSize = 200
		maxSize = 1500
		confidence = 0.7
		factors = []string{"line_breaks", "indentation", "syntax_boundaries"}
	}

	// Ensure constraints are respected
	if baseSize < minSize {
		baseSize = minSize
	}
	if baseSize > maxSize {
		baseSize = maxSize
	}

	reasoning := fmt.Sprintf("Calculated based on %s characteristics and content size analysis", language)

	return outbound.ChunkSizeRecommendation{
		OptimalSize:     baseSize,
		MinSize:         minSize,
		MaxSize:         maxSize,
		Confidence:      confidence,
		Reasoning:       reasoning,
		LanguageFactors: factors,
	}, nil
}

// ValidateChunkBoundaries ensures chunk boundaries preserve semantic integrity.
func (c *ChunkingStrategyAdapter) ValidateChunkBoundaries(
	ctx context.Context,
	chunks []outbound.EnhancedCodeChunk,
) ([]outbound.ChunkValidationResult, error) {
	slogger.Debug(ctx, "Validating chunk boundaries", slogger.Fields{
		"chunk_count": len(chunks),
	})

	if len(chunks) == 0 {
		return []outbound.ChunkValidationResult{}, nil
	}

	results := make([]outbound.ChunkValidationResult, len(chunks))

	for i, chunk := range chunks {
		result := outbound.ChunkValidationResult{
			ChunkID: chunk.ID,
			IsValid: true,
			Issues:  []outbound.ChunkValidationIssue{},
		}

		// Validate semantic boundaries
		c.validateSemanticBoundaries(ctx, &chunk, &result)

		// Validate size constraints
		c.validateSizeConstraints(ctx, &chunk, &result)

		// Validate context completeness
		c.validateContextCompleteness(ctx, &chunk, &result)

		// Calculate overall validation score
		result.ValidationScore = c.calculateValidationScore(result)

		results[i] = result
	}

	return results, nil
}

// GetSupportedStrategies returns the chunking strategies supported by this adapter.
func (c *ChunkingStrategyAdapter) GetSupportedStrategies(ctx context.Context) ([]outbound.ChunkingStrategyType, error) {
	strategies := []outbound.ChunkingStrategyType{
		outbound.StrategyFunction,
		outbound.StrategyClass,
		outbound.StrategySizeBased,
		outbound.StrategyHybrid,
		outbound.StrategyAdaptive,
	}

	slogger.Debug(ctx, "Returning supported chunking strategies", slogger.Fields{
		"strategy_count": len(strategies),
	})

	return strategies, nil
}

// PreserveContext applies context preservation strategies to maintain semantic relationships.
func (c *ChunkingStrategyAdapter) PreserveContext(
	ctx context.Context,
	chunks []outbound.EnhancedCodeChunk,
	strategy outbound.ContextPreservationStrategy,
) ([]outbound.EnhancedCodeChunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	slogger.Info(ctx, "Preserving context for chunks", slogger.Fields{
		"chunk_count": len(chunks),
		"strategy":    string(strategy),
	})

	return c.contextPreserver.PreserveContext(ctx, chunks, strategy)
}

// Helper methods for internal chunking algorithms

func (c *ChunkingStrategyAdapter) chunkByHybrid(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	// Combine function and class-based chunking with size constraints
	functionChunks, err := c.functionChunker.ChunkByFunction(ctx, semanticChunks, config)
	if err != nil {
		return nil, fmt.Errorf("function chunking failed in hybrid mode: %w", err)
	}

	// Apply size-based refinement
	refinedChunks, err := c.sizeBasedChunker.RefineChunks(ctx, functionChunks, config)
	if err != nil {
		return nil, fmt.Errorf("size-based refinement failed in hybrid mode: %w", err)
	}

	return refinedChunks, nil
}

func (c *ChunkingStrategyAdapter) chunkByAdaptive(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	// Analyze content characteristics to choose optimal strategy
	strategy := c.analyzeOptimalStrategy(ctx, semanticChunks)

	// Create new config with the adaptive strategy
	adaptedConfig := config
	adaptedConfig.Strategy = strategy

	slogger.Info(ctx, "Adaptive chunking selected strategy", slogger.Fields{
		"selected_strategy": string(strategy),
		"chunk_count":       len(semanticChunks),
	})

	// Apply the selected strategy
	return c.ChunkCode(ctx, semanticChunks, adaptedConfig)
}

func (c *ChunkingStrategyAdapter) analyzeOptimalStrategy(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
) outbound.ChunkingStrategyType {
	// Analyze semantic chunk characteristics
	functionCount := 0
	classCount := 0
	totalSize := 0

	for _, chunk := range semanticChunks {
		totalSize += len(chunk.Content)
		switch chunk.Type {
		case outbound.ConstructFunction, outbound.ConstructMethod:
			functionCount++
		case outbound.ConstructClass, outbound.ConstructStruct:
			classCount++
		}
	}

	avgChunkSize := totalSize / max(len(semanticChunks), 1)

	// Decision logic for strategy selection
	if classCount > functionCount {
		return outbound.StrategyClass
	}
	if functionCount > 0 && avgChunkSize < 1000 {
		return outbound.StrategyFunction
	}
	if avgChunkSize > 2000 {
		return outbound.StrategySizeBased
	}

	// Default to hybrid for balanced content
	return outbound.StrategyHybrid
}

// Validation helper methods

func (c *ChunkingStrategyAdapter) validateConfiguration(config outbound.ChunkingConfiguration) error {
	if config.MaxChunkSize < config.MinChunkSize {
		return fmt.Errorf("max chunk size (%d) cannot be less than min chunk size (%d)",
			config.MaxChunkSize, config.MinChunkSize)
	}
	if config.QualityThreshold < 0 || config.QualityThreshold > 1 {
		return fmt.Errorf("quality threshold must be between 0 and 1, got %f", config.QualityThreshold)
	}
	if config.OverlapSize < 0 {
		return fmt.Errorf("overlap size must be non-negative, got %d", config.OverlapSize)
	}
	if config.OverlapSize >= config.MaxChunkSize {
		return fmt.Errorf("overlap size (%d) must be less than max chunk size (%d)",
			config.OverlapSize, config.MaxChunkSize)
	}
	return nil
}

func (c *ChunkingStrategyAdapter) validateSemanticBoundaries(
	ctx context.Context,
	chunk *outbound.EnhancedCodeChunk,
	result *outbound.ChunkValidationResult,
) {
	// Check for incomplete constructs
	if chunk.EndByte <= chunk.StartByte {
		result.Issues = append(result.Issues, outbound.ChunkValidationIssue{
			Type:     outbound.IssueIncompleteBoundary,
			Severity: outbound.SeverityCritical,
			Message:  "Invalid byte boundaries: end byte must be greater than start byte",
		})
		result.IsValid = false
	}
}

func (c *ChunkingStrategyAdapter) validateSizeConstraints(
	ctx context.Context,
	chunk *outbound.EnhancedCodeChunk,
	result *outbound.ChunkValidationResult,
) {
	chunkSize := chunk.Size.Bytes
	if chunkSize == 0 {
		result.Issues = append(result.Issues, outbound.ChunkValidationIssue{
			Type:     outbound.IssueSizeViolation,
			Severity: outbound.SeverityHigh,
			Message:  "Chunk has zero size",
		})
	}
}

func (c *ChunkingStrategyAdapter) validateContextCompleteness(
	ctx context.Context,
	chunk *outbound.EnhancedCodeChunk,
	result *outbound.ChunkValidationResult,
) {
	// Check if chunk has minimal context preservation
	if len(chunk.PreservedContext.ImportStatements) == 0 && len(chunk.Dependencies) > 0 {
		result.Issues = append(result.Issues, outbound.ChunkValidationIssue{
			Type:     outbound.IssueMissingContext,
			Severity: outbound.SeverityMedium,
			Message:  "Chunk has dependencies but no import context",
		})
	}
}

func (c *ChunkingStrategyAdapter) calculateValidationScore(result outbound.ChunkValidationResult) float64 {
	if !result.IsValid {
		return 0.0
	}

	score := 1.0
	for _, issue := range result.Issues {
		switch issue.Severity {
		case outbound.SeverityCritical:
			score -= 0.5
		case outbound.SeverityHigh:
			score -= 0.3
		case outbound.SeverityMedium:
			score -= 0.2
		case outbound.SeverityLow:
			score -= 0.1
		}
	}

	if score < 0.0 {
		return 0.0
	}
	return score
}

// Language-specific size calculation methods

func (c *ChunkingStrategyAdapter) calculateGoOptimalSize(contentSize int) int {
	// Go-specific heuristics: prefer function boundaries, account for interfaces
	return int(float64(contentSize) * 0.3)
}

func (c *ChunkingStrategyAdapter) calculatePythonOptimalSize(contentSize int) int {
	// Python-specific heuristics: class and function structure
	return int(float64(contentSize) * 0.35)
}

func (c *ChunkingStrategyAdapter) calculateJSOptimalSize(contentSize int) int {
	// JavaScript/TypeScript-specific heuristics: modules and closures
	return int(float64(contentSize) * 0.25)
}

func (c *ChunkingStrategyAdapter) calculateDefaultOptimalSize(contentSize int) int {
	// Default algorithm for unknown languages
	return int(float64(contentSize) * 0.4)
}

// Helper function for max (Go 1.18+ style).
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
