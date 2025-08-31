package service

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// SemanticAnalysisService provides domain logic for semantic code analysis.
// It orchestrates semantic traversal operations and applies business rules.
type SemanticAnalysisService struct {
	traverser outbound.SemanticTraverser
}

// NewSemanticAnalysisService creates a new semantic analysis domain service.
func NewSemanticAnalysisService(traverser outbound.SemanticTraverser) *SemanticAnalysisService {
	return &SemanticAnalysisService{
		traverser: traverser,
	}
}

// AnalyzeCodeSemantics performs comprehensive semantic analysis on a parse tree.
// This is the main domain method that applies business rules and coordinates
// different types of semantic extraction.
func (s *SemanticAnalysisService) AnalyzeCodeSemantics(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticAnalysisOptions,
) (*SemanticAnalysisResult, error) {
	slogger.Info(ctx, "Starting semantic code analysis", slogger.Fields{
		"language":        parseTree.Language().Name(),
		"source_length":   len(parseTree.Source()),
		"include_private": options.IncludePrivate,
		"max_depth":       options.MaxDepth,
	})

	startTime := time.Now()

	// Apply domain validation rules
	if err := s.validateAnalysisRequest(ctx, parseTree, options); err != nil {
		slogger.Error(ctx, "Semantic analysis validation failed", slogger.Fields{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("semantic analysis validation failed: %w", err)
	}

	// Convert domain options to port options
	extractionOptions := s.convertToExtractionOptions(options)

	// Extract different types of semantic constructs
	result := &SemanticAnalysisResult{
		Language:   parseTree.Language(),
		SourceHash: s.calculateSourceHash(parseTree.Source()),
		Chunks:     make([]outbound.SemanticCodeChunk, 0),
		Statistics: &AnalysisStatistics{},
		StartTime:  startTime,
		Options:    options,
	}

	// Extract functions if requested
	if options.IncludeFunctions {
		functions, err := s.traverser.ExtractFunctions(ctx, parseTree, extractionOptions)
		if err != nil {
			slogger.Error(ctx, "Function extraction failed", slogger.Fields{
				"error": err.Error(),
			})
			return nil, fmt.Errorf("function extraction failed: %w", err)
		}
		result.Chunks = append(result.Chunks, functions...)
		result.Statistics.FunctionCount = len(functions)
		slogger.Debug(ctx, "Functions extracted successfully", slogger.Fields{
			"count": len(functions),
		})
	}

	// Extract classes if requested
	if options.IncludeClasses {
		classes, err := s.traverser.ExtractClasses(ctx, parseTree, extractionOptions)
		if err != nil {
			slogger.Error(ctx, "Class extraction failed", slogger.Fields{
				"error": err.Error(),
			})
			return nil, fmt.Errorf("class extraction failed: %w", err)
		}
		result.Chunks = append(result.Chunks, classes...)
		result.Statistics.ClassCount = len(classes)
		slogger.Debug(ctx, "Classes extracted successfully", slogger.Fields{
			"count": len(classes),
		})
	}

	// Extract modules if requested
	if options.IncludeModules {
		modules, err := s.traverser.ExtractModules(ctx, parseTree, extractionOptions)
		if err != nil {
			slogger.Error(ctx, "Module extraction failed", slogger.Fields{
				"error": err.Error(),
			})
			return nil, fmt.Errorf("module extraction failed: %w", err)
		}
		result.Chunks = append(result.Chunks, modules...)
		result.Statistics.ModuleCount = len(modules)
		slogger.Debug(ctx, "Modules extracted successfully", slogger.Fields{
			"count": len(modules),
		})
	}

	// Apply business rules for chunk filtering and processing
	result.Chunks = s.applyBusinessRules(ctx, result.Chunks, options)

	// Calculate final statistics
	result.Duration = time.Since(startTime)
	result.Statistics.TotalChunks = len(result.Chunks)
	result.Statistics.ChunksByType = s.calculateChunksByType(result.Chunks)
	result.Statistics.AverageChunkSize = s.calculateAverageChunkSize(result.Chunks)

	slogger.Info(ctx, "Semantic analysis completed successfully", slogger.Fields{
		"total_chunks":   result.Statistics.TotalChunks,
		"function_count": result.Statistics.FunctionCount,
		"class_count":    result.Statistics.ClassCount,
		"module_count":   result.Statistics.ModuleCount,
		"duration_ms":    result.Duration.Milliseconds(),
	})

	return result, nil
}

// validateAnalysisRequest applies domain validation rules to the analysis request.
func (s *SemanticAnalysisService) validateAnalysisRequest(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticAnalysisOptions,
) error {
	if parseTree == nil {
		return errors.New("parse tree cannot be nil")
	}

	if options.MaxDepth < 0 {
		return fmt.Errorf("max depth cannot be negative: %d", options.MaxDepth)
	}

	if len(parseTree.Source()) == 0 {
		slogger.Warn(ctx, "Empty source code provided for semantic analysis", slogger.Fields{
			"language": parseTree.Language().Name(),
		})
	}

	// Business rule: Enforce reasonable limits
	const maxSourceSize = 10 * 1024 * 1024 // 10MB
	if len(parseTree.Source()) > maxSourceSize {
		return fmt.Errorf("source code too large: %d bytes exceeds limit of %d bytes",
			len(parseTree.Source()), maxSourceSize)
	}

	return nil
}

// convertToExtractionOptions converts domain options to port layer options.
func (s *SemanticAnalysisService) convertToExtractionOptions(
	options SemanticAnalysisOptions,
) outbound.SemanticExtractionOptions {
	return outbound.SemanticExtractionOptions{
		IncludePrivate:       options.IncludePrivate,
		IncludeComments:      options.IncludeComments,
		IncludeDocumentation: options.IncludeDocumentation,
		IncludeTypeInfo:      options.IncludeTypeInfo,
		MaxDepth:             options.MaxDepth,
		NamePatterns:         options.NamePatterns,
		ExcludePatterns:      options.ExcludePatterns,
		PreservationStrategy: outbound.PreserveModerate,
		ChunkingStrategy:     outbound.ChunkByFunction,
	}
}

// applyBusinessRules applies domain-specific business rules to filter and process chunks.
func (s *SemanticAnalysisService) applyBusinessRules(
	ctx context.Context,
	chunks []outbound.SemanticCodeChunk,
	options SemanticAnalysisOptions,
) []outbound.SemanticCodeChunk {
	processed := make([]outbound.SemanticCodeChunk, 0, len(chunks))

	for _, chunk := range chunks {
		// Business rule: Filter by minimum size if specified
		if options.MinChunkSize > 0 && len(chunk.Content) < options.MinChunkSize {
			slogger.Debug(ctx, "Chunk filtered due to minimum size rule", slogger.Fields{
				"chunk_id":     chunk.ID,
				"chunk_size":   len(chunk.Content),
				"min_required": options.MinChunkSize,
			})
			continue
		}

		// Business rule: Skip empty or trivial chunks
		if len(chunk.Content) == 0 || len(chunk.Name) == 0 {
			slogger.Debug(ctx, "Skipping empty or trivial chunk", slogger.Fields{
				"chunk_id": chunk.ID,
			})
			continue
		}

		// Business rule: Apply quality scoring
		chunk.Metadata = s.enhanceChunkMetadata(ctx, chunk)

		processed = append(processed, chunk)
	}

	slogger.Debug(ctx, "Business rules applied to chunks", slogger.Fields{
		"original_count": len(chunks),
		"filtered_count": len(processed),
	})

	return processed
}

// enhanceChunkMetadata adds domain-specific metadata to chunks.
func (s *SemanticAnalysisService) enhanceChunkMetadata(
	_ context.Context,
	chunk outbound.SemanticCodeChunk,
) map[string]interface{} {
	metadata := make(map[string]interface{})
	if chunk.Metadata != nil {
		metadata = chunk.Metadata
	}

	// Add quality metrics
	metadata["content_length"] = len(chunk.Content)
	metadata["has_documentation"] = len(chunk.Documentation) > 0
	metadata["complexity_score"] = s.calculateComplexityScore(chunk)
	metadata["semantic_depth"] = s.calculateSemanticDepth(chunk)

	return metadata
}

// calculateComplexityScore calculates a simple complexity score for a chunk.
func (s *SemanticAnalysisService) calculateComplexityScore(chunk outbound.SemanticCodeChunk) float64 {
	score := 1.0

	// Factor in content length
	if len(chunk.Content) > 1000 {
		score += 0.5
	}

	// Factor in child chunks
	score += float64(len(chunk.ChildChunks)) * 0.2

	// Factor in parameters
	score += float64(len(chunk.Parameters)) * 0.1

	return score
}

// calculateSemanticDepth calculates the semantic nesting depth of a chunk.
func (s *SemanticAnalysisService) calculateSemanticDepth(chunk outbound.SemanticCodeChunk) int {
	if len(chunk.ChildChunks) == 0 {
		return 1
	}

	maxDepth := 0
	for _, child := range chunk.ChildChunks {
		depth := s.calculateSemanticDepth(child)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth + 1
}

// calculateSourceHash generates a hash for the source code.
func (s *SemanticAnalysisService) calculateSourceHash(source []byte) string {
	hash := sha256.Sum256(source)
	return hex.EncodeToString(hash[:])[:16]
}

// calculateChunksByType counts chunks by their semantic construct type.
func (s *SemanticAnalysisService) calculateChunksByType(
	chunks []outbound.SemanticCodeChunk,
) map[outbound.SemanticConstructType]int {
	counts := make(map[outbound.SemanticConstructType]int)

	for _, chunk := range chunks {
		counts[chunk.Type]++
	}

	return counts
}

// calculateAverageChunkSize calculates the average size of chunks.
func (s *SemanticAnalysisService) calculateAverageChunkSize(chunks []outbound.SemanticCodeChunk) int {
	if len(chunks) == 0 {
		return 0
	}

	total := 0
	for _, chunk := range chunks {
		total += len(chunk.Content)
	}

	return total / len(chunks)
}

// Domain types for semantic analysis

// SemanticAnalysisOptions configures semantic analysis behavior at the domain level.
type SemanticAnalysisOptions struct {
	IncludeFunctions     bool     `json:"include_functions"`
	IncludeClasses       bool     `json:"include_classes"`
	IncludeModules       bool     `json:"include_modules"`
	IncludePrivate       bool     `json:"include_private"`
	IncludeComments      bool     `json:"include_comments"`
	IncludeDocumentation bool     `json:"include_documentation"`
	IncludeTypeInfo      bool     `json:"include_type_info"`
	MaxDepth             int      `json:"max_depth"`
	MinChunkSize         int      `json:"min_chunk_size"`
	NamePatterns         []string `json:"name_patterns,omitempty"`
	ExcludePatterns      []string `json:"exclude_patterns,omitempty"`
}

// SemanticAnalysisResult contains the results of semantic analysis.
type SemanticAnalysisResult struct {
	Language   valueobject.Language         `json:"language"`
	SourceHash string                       `json:"source_hash"`
	Chunks     []outbound.SemanticCodeChunk `json:"chunks"`
	Statistics *AnalysisStatistics          `json:"statistics"`
	StartTime  time.Time                    `json:"start_time"`
	Duration   time.Duration                `json:"duration"`
	Options    SemanticAnalysisOptions      `json:"options"`
}

// AnalysisStatistics provides statistics about the semantic analysis.
type AnalysisStatistics struct {
	TotalChunks      int                                    `json:"total_chunks"`
	FunctionCount    int                                    `json:"function_count"`
	ClassCount       int                                    `json:"class_count"`
	ModuleCount      int                                    `json:"module_count"`
	ChunksByType     map[outbound.SemanticConstructType]int `json:"chunks_by_type"`
	AverageChunkSize int                                    `json:"average_chunk_size"`
}
