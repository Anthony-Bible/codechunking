package chunking

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Phase 4.3 Step 4: Large function size-based chunking implementation

// LargeFunctionChunker handles intelligent splitting of very large functions while preserving
// semantic boundaries and context.
type LargeFunctionChunker struct {
	parserFactory TreeSitterParserFactory
	options       SizeChunkingOptions
}

// SizeChunkingOptions configures size-based chunking behavior for large functions.
type SizeChunkingOptions struct {
	OptimalThreshold int // Default: 10KB
	MaxThreshold     int // Default: 50KB
	MinChunkSize     int // Minimum size for split chunks
}

// TreeSitterParserFactory interface for creating Tree-sitter parsers.
type TreeSitterParserFactory interface {
	CreateParser(language valueobject.Language) (TreeSitterParser, error)
	GetSupportedLanguages() []valueobject.Language
}

// TreeSitterParser interface for Tree-sitter parsing operations.
type TreeSitterParser interface {
	Parse(ctx context.Context, content string) (*valueobject.ParseTree, error)
	IdentifyBoundaries(ctx context.Context, parseTree *valueobject.ParseTree) ([]SemanticBoundary, error)
	ExtractContext(ctx context.Context, parseTree *valueobject.ParseTree) ([]ContextElement, error)
}

// SemanticBoundary represents a logical split point within a function.
type SemanticBoundary struct {
	Type      outbound.BoundaryType `json:"type"`
	Position  BoundaryPosition      `json:"position"`
	CanSplit  bool                  `json:"can_split"`
	Reason    string                `json:"reason"`
	StartByte uint32                `json:"start_byte"`
	EndByte   uint32                `json:"end_byte"`
}

// BoundaryPosition represents specific positions for different boundary types.
type BoundaryPosition string

const (
	StatementComplete   BoundaryPosition = "statement_complete"
	IfElseBlock         BoundaryPosition = "if_else_block"
	LoopBlock           BoundaryPosition = "loop_block"
	TryCatchBlock       BoundaryPosition = "try_catch_block"
	CommentBlock        BoundaryPosition = "comment_block"
	VariableDeclaration BoundaryPosition = "variable_declaration"
	ClassDefinition     BoundaryPosition = "class_definition"
	FunctionDefinition  BoundaryPosition = "function_definition"
	WithBlock           BoundaryPosition = "with_block"
	ArrowFunction       BoundaryPosition = "arrow_function"
	AsyncAwaitBlock     BoundaryPosition = "async_await_block"
	PromiseChain        BoundaryPosition = "promise_chain"
	ReturnStatement     BoundaryPosition = "return_statement"
)

// ContextElement represents a context element that needs to be preserved.
type ContextElement struct {
	Type     ContextElementType `json:"type"`
	Content  string             `json:"content"`
	Required bool               `json:"required"`
	Reason   string             `json:"reason"`
}

// ContextElementType represents different types of context.
type ContextElementType string

const (
	ContextImport              ContextElementType = "import"
	ContextTypeDefinition      ContextElementType = "type_definition"
	ContextSignature           ContextElementType = "signature"
	ContextDocumentation       ContextElementType = "documentation"
	ContextVariableDeclaration ContextElementType = "variable_declaration"
)

// SplitRecommendation represents a recommendation for splitting a function.
type SplitRecommendation struct {
	EstimatedChunks int `json:"estimated_chunks"`
	MaxChunkSize    int `json:"max_chunk_size"`
}

// Expected error types that should be implemented.
var (
	ErrUnparseableFunction  = errors.New("function cannot be parsed")
	ErrCircularDependencies = errors.New("circular dependencies detected")
)

// NewLargeFunctionChunker creates a new large function chunker with the specified parser factory and options.
func NewLargeFunctionChunker(parserFactory TreeSitterParserFactory, options SizeChunkingOptions) *LargeFunctionChunker {
	// Set default options if not provided
	if options.OptimalThreshold == 0 {
		options.OptimalThreshold = 10 * 1024 // 10KB default
	}
	if options.MaxThreshold == 0 {
		options.MaxThreshold = 50 * 1024 // 50KB max
	}
	if options.MinChunkSize == 0 {
		options.MinChunkSize = 1 * 1024 // 1KB min
	}

	return &LargeFunctionChunker{
		parserFactory: parserFactory,
		options:       options,
	}
}

// NewLargeFunctionChunkerWithDefaults creates a new large function chunker with default options for testing.
func NewLargeFunctionChunkerWithDefaults() *LargeFunctionChunker {
	return &LargeFunctionChunker{
		parserFactory: &MockTreeSitterParserFactory{},
		options: SizeChunkingOptions{
			OptimalThreshold: 10 * 1024, // 10KB
			MaxThreshold:     50 * 1024, // 50KB
			MinChunkSize:     1 * 1024,  // 1KB
		},
	}
}

// ChunkCode implements the main chunking interface for large functions.
func (lfc *LargeFunctionChunker) ChunkCode(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Info(ctx, "Starting large function size-based chunking", slogger.Fields{
		"chunks_count":      len(semanticChunks),
		"optimal_threshold": lfc.options.OptimalThreshold,
		"max_threshold":     lfc.options.MaxThreshold,
	})

	var enhancedChunks []outbound.EnhancedCodeChunk

	for _, chunk := range semanticChunks {
		// Check if function exceeds size thresholds
		shouldSplit, recommendation, err := lfc.ShouldSplitFunction(ctx, chunk, config)
		if err != nil {
			slogger.Error(ctx, "Error determining if function should be split", slogger.Fields{
				"function": chunk.Name,
				"error":    err.Error(),
			})
			continue
		}

		chunksToAdd, err := lfc.processChunkBasedOnSize(ctx, chunk, shouldSplit, config)
		if err != nil {
			return nil, fmt.Errorf("failed to process chunk %s: %w", chunk.Name, err)
		}

		// Add overlap between split chunks if configured and multiple chunks were created
		if config.OverlapSize > 0 && len(chunksToAdd) > 1 {
			lfc.addSplitChunkOverlap(ctx, chunksToAdd, config)
		}

		enhancedChunks = append(enhancedChunks, chunksToAdd...)

		slogger.Debug(ctx, "Function processing completed", slogger.Fields{
			"function":         chunk.Name,
			"should_split":     shouldSplit,
			"estimated_chunks": recommendation.EstimatedChunks,
			"original_size":    len(chunk.Content),
		})
	}

	slogger.Info(ctx, "Large function size-based chunking completed", slogger.Fields{
		"input_chunks":  len(semanticChunks),
		"output_chunks": len(enhancedChunks),
	})

	return enhancedChunks, nil
}

// ShouldSplitFunction determines if a function should be split based on size thresholds.
func (lfc *LargeFunctionChunker) ShouldSplitFunction(
	ctx context.Context,
	chunk outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) (bool, *SplitRecommendation, error) {
	functionSize := len(chunk.Content)

	// Use config max size if provided, otherwise use our optimal threshold
	threshold := lfc.options.OptimalThreshold
	if config.MaxChunkSize > 0 && config.MaxChunkSize < lfc.options.OptimalThreshold {
		threshold = config.MaxChunkSize
	}

	shouldSplit := functionSize > threshold

	recommendation := &SplitRecommendation{
		MaxChunkSize: threshold,
	}

	if shouldSplit {
		// Estimate number of chunks needed
		estimatedChunks := (functionSize + threshold - 1) / threshold // Ceiling division
		if estimatedChunks < 2 {
			estimatedChunks = 2
		}
		recommendation.EstimatedChunks = estimatedChunks
	} else {
		recommendation.EstimatedChunks = 1
	}

	slogger.Debug(ctx, "Size-based split analysis", slogger.Fields{
		"function":         chunk.Name,
		"size":             functionSize,
		"threshold":        threshold,
		"should_split":     shouldSplit,
		"estimated_chunks": recommendation.EstimatedChunks,
	})

	return shouldSplit, recommendation, nil
}

// SplitLargeFunction splits a large function into smaller semantic chunks.
func (lfc *LargeFunctionChunker) SplitLargeFunction(
	ctx context.Context,
	chunk outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	// Identify semantic boundaries using Tree-sitter
	boundaries, err := lfc.IdentifySemanticBoundaries(ctx, chunk)
	if err != nil {
		slogger.Error(ctx, "Failed to identify semantic boundaries", slogger.Fields{
			"function": chunk.Name,
			"error":    err.Error(),
		})
		return nil, fmt.Errorf("failed to identify boundaries for %s: %w", chunk.Name, err)
	}

	// Extract context for preservation
	contextElements := lfc.extractContextElements(ctx, chunk)

	// Split function at semantic boundaries
	splitChunks, err := lfc.splitAtBoundaries(ctx, chunk, boundaries, contextElements, config)
	if err != nil {
		return nil, fmt.Errorf("failed to split at boundaries for %s: %w", chunk.Name, err)
	}

	// Validate quality of split chunks
	validatedChunks := lfc.validateQualityOfSplitChunks(ctx, splitChunks, config)

	slogger.Info(ctx, "Function split successfully", slogger.Fields{
		"function":     chunk.Name,
		"boundaries":   len(boundaries),
		"split_chunks": len(validatedChunks),
	})

	return validatedChunks, nil
}

// IdentifySemanticBoundaries uses Tree-sitter to identify logical split points.
func (lfc *LargeFunctionChunker) IdentifySemanticBoundaries(
	ctx context.Context,
	chunk outbound.SemanticCodeChunk,
) ([]SemanticBoundary, error) {
	parser, err := lfc.parserFactory.CreateParser(chunk.Language)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser for %s: %w", chunk.Language.Name(), err)
	}

	parseTree, err := parser.Parse(ctx, chunk.Content)
	if err != nil {
		return nil, ErrUnparseableFunction
	}

	boundaries, err := parser.IdentifyBoundaries(ctx, parseTree)
	if err != nil {
		return nil, fmt.Errorf("failed to identify boundaries: %w", err)
	}

	return boundaries, nil
}

// extractContextElements extracts context that needs to be preserved across chunks.
func (lfc *LargeFunctionChunker) extractContextElements(
	ctx context.Context,
	chunk outbound.SemanticCodeChunk,
) []ContextElement {
	parser, err := lfc.parserFactory.CreateParser(chunk.Language)
	if err != nil {
		return []ContextElement{} // Degraded mode - no context
	}

	parseTree, err := parser.Parse(ctx, chunk.Content)
	if err != nil {
		return []ContextElement{} // Degraded mode
	}

	contextElements, err := parser.ExtractContext(ctx, parseTree)
	if err != nil {
		return []ContextElement{} // Degraded mode
	}

	return contextElements
}

// splitAtBoundaries splits the function content at semantic boundaries.
func (lfc *LargeFunctionChunker) splitAtBoundaries(
	ctx context.Context,
	originalChunk outbound.SemanticCodeChunk,
	boundaries []SemanticBoundary,
	contextElements []ContextElement,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	if len(boundaries) == 0 {
		return lfc.splitBySize(ctx, originalChunk, config)
	}

	validBoundaries := lfc.filterValidBoundaries(boundaries)
	if len(validBoundaries) == 0 {
		return lfc.splitBySize(ctx, originalChunk, config)
	}

	chunks := lfc.createChunksFromBoundaries(ctx, originalChunk, validBoundaries, contextElements, config)
	chunks = lfc.handleRemainingContent(ctx, originalChunk, validBoundaries, contextElements, config, chunks)

	return chunks, nil
}

// splitBySize performs simple size-based splitting when no semantic boundaries are available.
func (lfc *LargeFunctionChunker) splitBySize(
	ctx context.Context,
	chunk outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	content := chunk.Content
	chunkSize := lfc.options.OptimalThreshold
	if config.MaxChunkSize > 0 && config.MaxChunkSize < chunkSize {
		chunkSize = config.MaxChunkSize
	}

	var chunks []outbound.EnhancedCodeChunk

	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		chunkContent := content[i:end]
		chunkIndex := i / chunkSize

		// Safe conversion with bounds checking
		var startByteVal, endByteVal uint32
		if i < 0 || i > int(^uint32(0)) {
			startByteVal = 0
		} else {
			startByteVal = uint32(i)
		}
		if end < 0 || end > int(^uint32(0)) {
			endByteVal = ^uint32(0) // Max uint32 value
		} else {
			endByteVal = uint32(end)
		}

		enhancedChunk := lfc.createEnhancedChunkFromSplit(
			ctx,
			chunk,
			chunkContent,
			startByteVal,
			endByteVal,
			chunkIndex,
			config,
		)

		chunks = append(chunks, *enhancedChunk)
	}

	return chunks, nil
}

// filterValidBoundaries filters boundaries that can be split.
func (lfc *LargeFunctionChunker) filterValidBoundaries(boundaries []SemanticBoundary) []SemanticBoundary {
	validBoundaries := []SemanticBoundary{}
	for _, boundary := range boundaries {
		if boundary.CanSplit {
			validBoundaries = append(validBoundaries, boundary)
		}
	}
	return validBoundaries
}

// createChunksFromBoundaries creates chunks based on semantic boundaries.
func (lfc *LargeFunctionChunker) createChunksFromBoundaries(
	ctx context.Context,
	originalChunk outbound.SemanticCodeChunk,
	validBoundaries []SemanticBoundary,
	contextElements []ContextElement,
	config outbound.ChunkingConfiguration,
) []outbound.EnhancedCodeChunk {
	var chunks []outbound.EnhancedCodeChunk
	content := originalChunk.Content
	startIdx := 0

	for i, boundary := range validBoundaries {
		endIdx := int(boundary.EndByte)
		if endIdx > len(content) {
			endIdx = len(content)
		}

		chunk := lfc.createChunkFromBoundary(
			ctx, originalChunk, content, contextElements,
			startIdx, endIdx, i, config,
		)
		chunks = append(chunks, *chunk)
		startIdx = endIdx
	}

	return chunks
}

// createChunkFromBoundary creates a single chunk from a boundary segment.
func (lfc *LargeFunctionChunker) createChunkFromBoundary(
	ctx context.Context,
	originalChunk outbound.SemanticCodeChunk,
	content string,
	contextElements []ContextElement,
	startIdx, endIdx, index int,
	config outbound.ChunkingConfiguration,
) *outbound.EnhancedCodeChunk {
	chunkContent := content[startIdx:endIdx]
	preservedContent := lfc.preserveContextInSplitFunctions(chunkContent, contextElements, index == 0)

	// Safe conversion with bounds checking
	startByteVal := lfc.safeIntToUint32(startIdx)
	endByteVal := lfc.safeIntToUint32(endIdx)

	return lfc.createEnhancedChunkFromSplit(
		ctx, originalChunk, preservedContent,
		startByteVal, endByteVal, index, config,
	)
}

// handleRemainingContent handles any remaining content after boundary-based splitting.
func (lfc *LargeFunctionChunker) handleRemainingContent(
	ctx context.Context,
	originalChunk outbound.SemanticCodeChunk,
	validBoundaries []SemanticBoundary,
	contextElements []ContextElement,
	config outbound.ChunkingConfiguration,
	chunks []outbound.EnhancedCodeChunk,
) []outbound.EnhancedCodeChunk {
	content := originalChunk.Content
	startIdx := 0
	if len(validBoundaries) > 0 {
		lastBoundary := validBoundaries[len(validBoundaries)-1]
		startIdx = int(lastBoundary.EndByte)
		if startIdx > len(content) {
			startIdx = len(content)
		}
	}

	// Extract remaining content if any
	var remainingContent string
	if startIdx < len(content) {
		remainingContent = content[startIdx:]
	}

	// Process remaining content if it exists
	if remainingContent == "" {
		return chunks
	}

	// Process remaining content
	chunkSize := lfc.getEffectiveChunkSize(config)
	if lfc.shouldSplitBySize(remainingContent, chunkSize) {
		return lfc.processLargeRemainingContent(
			ctx,
			originalChunk,
			content,
			startIdx,
			validBoundaries,
			contextElements,
			config,
			chunks,
			remainingContent,
		)
	}

	// Add small remaining content as single chunk
	chunk := lfc.createRemainingContentChunk(
		ctx,
		originalChunk,
		content,
		startIdx,
		validBoundaries,
		contextElements,
		config,
		remainingContent,
	)
	chunks = append(chunks, *chunk)

	return chunks
}

// getEffectiveChunkSize returns the effective chunk size based on config and options.
func (lfc *LargeFunctionChunker) getEffectiveChunkSize(config outbound.ChunkingConfiguration) int {
	maxChunkSize := config.MaxChunkSize
	if maxChunkSize == 0 {
		maxChunkSize = lfc.options.OptimalThreshold
	}
	return maxChunkSize
}

// shouldSplitBySize determines if content should be split based on its size vs chunk size.
func (lfc *LargeFunctionChunker) shouldSplitBySize(content string, chunkSize int) bool {
	return len(content) > chunkSize
}

// processLargeRemainingContent handles large remaining content by attempting to split it.
func (lfc *LargeFunctionChunker) processLargeRemainingContent(
	ctx context.Context,
	originalChunk outbound.SemanticCodeChunk,
	content string,
	startIdx int,
	validBoundaries []SemanticBoundary,
	contextElements []ContextElement,
	config outbound.ChunkingConfiguration,
	chunks []outbound.EnhancedCodeChunk,
	remainingContent string,
) []outbound.EnhancedCodeChunk {
	remainingChunkData := outbound.SemanticCodeChunk{
		Name:          originalChunk.Name + "_remaining",
		Type:          originalChunk.Type,
		Language:      originalChunk.Language,
		Content:       remainingContent,
		StartByte:     lfc.safeIntToUint32(startIdx),
		EndByte:       lfc.safeIntToUint32(len(content)),
		StartPosition: originalChunk.StartPosition,
		EndPosition:   originalChunk.EndPosition,
		Documentation: originalChunk.Documentation,
	}

	splitRemaining, err := lfc.splitBySize(ctx, remainingChunkData, config)
	if err != nil {
		// Fallback to single chunk if split fails
		chunk := lfc.createRemainingContentChunk(
			ctx,
			originalChunk,
			content,
			startIdx,
			validBoundaries,
			contextElements,
			config,
			remainingContent,
		)
		chunks = append(chunks, *chunk)
	} else {
		chunks = append(chunks, splitRemaining...)
	}

	return chunks
}

// createRemainingContentChunk creates a chunk from remaining content with preserved context.
func (lfc *LargeFunctionChunker) createRemainingContentChunk(
	ctx context.Context,
	originalChunk outbound.SemanticCodeChunk,
	content string,
	startIdx int,
	validBoundaries []SemanticBoundary,
	contextElements []ContextElement,
	config outbound.ChunkingConfiguration,
	remainingContent string,
) *outbound.EnhancedCodeChunk {
	preservedContent := lfc.preserveContextInSplitFunctions(remainingContent, contextElements, false)
	return lfc.createEnhancedChunkFromSplit(
		ctx,
		originalChunk,
		preservedContent,
		lfc.safeIntToUint32(startIdx),
		lfc.safeIntToUint32(len(content)),
		len(validBoundaries),
		config,
	)
}

// safeIntToUint32 safely converts int to uint32 with bounds checking.
func (lfc *LargeFunctionChunker) safeIntToUint32(value int) uint32 {
	if value < 0 || value > int(^uint32(0)) {
		if value < 0 {
			return 0
		}
		return ^uint32(0) // Max uint32 value
	}
	return uint32(value)
}

// preserveContextInSplitFunctions adds necessary context to split function chunks.
func (lfc *LargeFunctionChunker) preserveContextInSplitFunctions(
	chunkContent string,
	contextElements []ContextElement,
	isFirstChunk bool,
) string {
	var contextBuilder strings.Builder

	// Add required context elements
	for _, element := range contextElements {
		if element.Required || (isFirstChunk && element.Type == ContextSignature) {
			contextBuilder.WriteString(element.Content)
			contextBuilder.WriteString("\n")
		}
	}

	// Add the original chunk content
	contextBuilder.WriteString(chunkContent)

	return contextBuilder.String()
}

// validateQualityOfSplitChunks ensures split chunks meet quality thresholds.
func (lfc *LargeFunctionChunker) validateQualityOfSplitChunks(
	ctx context.Context,
	chunks []outbound.EnhancedCodeChunk,
	config outbound.ChunkingConfiguration,
) []outbound.EnhancedCodeChunk {
	validatedChunks := make([]outbound.EnhancedCodeChunk, len(chunks))

	for i, chunk := range chunks {
		// Calculate quality metrics
		cohesionScore := lfc.calculateCohesionScore(chunk)
		complexityScore := lfc.calculateComplexityScore(chunk)

		// Update chunk with quality metrics
		validatedChunk := chunk
		validatedChunk.CohesionScore = cohesionScore
		validatedChunk.ComplexityScore = complexityScore
		validatedChunk.QualityMetrics = outbound.ChunkQualityMetrics{
			CohesionScore:     cohesionScore,
			ComplexityScore:   complexityScore,
			CompletenessScore: 0.9, // Basic assumption for split chunks
			OverallQuality:    (cohesionScore + (1.0 - complexityScore) + 0.9) / 3.0,
		}

		// Check if chunk meets quality threshold
		if validatedChunk.QualityMetrics.OverallQuality < config.QualityThreshold {
			slogger.Warn(ctx, "Split chunk quality below threshold", slogger.Fields{
				"chunk_index": i,
				"quality":     validatedChunk.QualityMetrics.OverallQuality,
				"threshold":   config.QualityThreshold,
				"cohesion":    cohesionScore,
				"complexity":  complexityScore,
			})
		}

		validatedChunks[i] = validatedChunk
	}

	return validatedChunks
}

// processChunkBasedOnSize handles chunk processing based on whether it should be split or not.
func (lfc *LargeFunctionChunker) processChunkBasedOnSize(
	ctx context.Context,
	chunk outbound.SemanticCodeChunk,
	shouldSplit bool,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	if shouldSplit {
		// Split large function into smaller chunks
		splitChunks, err := lfc.SplitLargeFunction(ctx, chunk, config)
		if err != nil {
			slogger.Error(ctx, "Error splitting large function", slogger.Fields{
				"function": chunk.Name,
				"error":    err.Error(),
			})
			// Fallback to single chunk
			fallbackChunk, fallbackErr := lfc.createFallbackChunk(ctx, chunk, config)
			if fallbackErr != nil {
				return nil, fmt.Errorf("failed to create fallback chunk for %s: %w", chunk.Name, fallbackErr)
			}
			return []outbound.EnhancedCodeChunk{*fallbackChunk}, nil
		}
		return splitChunks, nil
	}

	// Function is small enough, create single chunk
	singleChunk, err := lfc.createSingleChunk(ctx, chunk, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create single chunk for %s: %w", chunk.Name, err)
	}
	return []outbound.EnhancedCodeChunk{*singleChunk}, nil
}

// Helper methods for chunk creation and metrics calculation

func (lfc *LargeFunctionChunker) createEnhancedChunkFromSplit(
	ctx context.Context,
	originalChunk outbound.SemanticCodeChunk,
	content string,
	startByte, endByte uint32,
	index int,
	config outbound.ChunkingConfiguration,
) *outbound.EnhancedCodeChunk {
	chunkID := uuid.New().String()
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	// Calculate size metrics
	size := outbound.ChunkSize{
		Bytes:      len(content),
		Lines:      strings.Count(content, "\n") + 1,
		Characters: len(content),
		Constructs: 1, // Fragment of original construct
	}

	chunk := &outbound.EnhancedCodeChunk{
		ID:                 chunkID,
		SourceFile:         fmt.Sprintf("%s_split_%d", originalChunk.Name, index),
		Language:           originalChunk.Language,
		StartByte:          startByte,
		EndByte:            endByte,
		StartPosition:      originalChunk.StartPosition,
		EndPosition:        originalChunk.EndPosition,
		Content:            content,
		PreservedContext:   outbound.PreservedContext{},
		SemanticConstructs: []outbound.SemanticCodeChunk{originalChunk},
		Dependencies:       []outbound.ChunkDependency{},
		ChunkType:          outbound.ChunkFragment,
		ChunkingStrategy:   outbound.StrategySizeBased,
		ContextStrategy:    config.ContextPreservation,
		QualityMetrics:     outbound.ChunkQualityMetrics{},
		CreatedAt:          time.Now(),
		Hash:               contentHash,
		Size:               size,
		ComplexityScore:    0.5, // Default for fragments
		CohesionScore:      0.8, // Default for fragments
	}

	return chunk
}

func (lfc *LargeFunctionChunker) createSingleChunk(
	ctx context.Context,
	semanticChunk outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) (*outbound.EnhancedCodeChunk, error) {
	chunkID := uuid.New().String()
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(semanticChunk.Content)))

	size := outbound.ChunkSize{
		Bytes:      len(semanticChunk.Content),
		Lines:      strings.Count(semanticChunk.Content, "\n") + 1,
		Characters: len(semanticChunk.Content),
		Constructs: 1,
	}

	chunk := &outbound.EnhancedCodeChunk{
		ID:                 chunkID,
		SourceFile:         semanticChunk.Name,
		Language:           semanticChunk.Language,
		StartByte:          semanticChunk.StartByte,
		EndByte:            semanticChunk.EndByte,
		StartPosition:      semanticChunk.StartPosition,
		EndPosition:        semanticChunk.EndPosition,
		Content:            semanticChunk.Content,
		PreservedContext:   outbound.PreservedContext{},
		SemanticConstructs: []outbound.SemanticCodeChunk{semanticChunk},
		Dependencies:       []outbound.ChunkDependency{},
		ChunkType:          outbound.ChunkFunction,
		ChunkingStrategy:   outbound.StrategyFunction,
		ContextStrategy:    config.ContextPreservation,
		QualityMetrics:     outbound.ChunkQualityMetrics{},
		CreatedAt:          time.Now(),
		Hash:               contentHash,
		Size:               size,
		ComplexityScore:    0.3, // Low complexity for single functions
		CohesionScore:      1.0, // Perfect cohesion for single functions
	}

	return chunk, nil
}

func (lfc *LargeFunctionChunker) createFallbackChunk(
	ctx context.Context,
	semanticChunk outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) (*outbound.EnhancedCodeChunk, error) {
	// Create a fallback chunk when splitting fails
	return lfc.createSingleChunk(ctx, semanticChunk, config)
}

func (lfc *LargeFunctionChunker) calculateCohesionScore(chunk outbound.EnhancedCodeChunk) float64 {
	// Simple cohesion calculation for fragments
	contentLength := len(chunk.Content)
	switch {
	case contentLength < 100:
		return 0.9 // Small chunks are highly cohesive
	case contentLength < 1000:
		return 0.8 // Medium chunks are moderately cohesive
	default:
		return 0.7 // Large chunks are less cohesive
	}
}

func (lfc *LargeFunctionChunker) calculateComplexityScore(chunk outbound.EnhancedCodeChunk) float64 {
	// Simple complexity calculation based on content length and constructs
	contentLength := len(chunk.Content)
	complexityFactor := float64(contentLength) / 1000.0 // Normalize to 1KB
	if complexityFactor > 1.0 {
		complexityFactor = 1.0
	}
	return complexityFactor * 0.5 // Scale to reasonable complexity
}

// addSplitChunkOverlap adds semantic overlap context between split chunks from a large function.
// This helps maintain continuity and context across split boundaries.
func (lfc *LargeFunctionChunker) addSplitChunkOverlap(
	ctx context.Context,
	chunks []outbound.EnhancedCodeChunk,
	config outbound.ChunkingConfiguration,
) {
	if len(chunks) <= 1 {
		return
	}

	// Add overlap from previous chunk to each subsequent chunk
	for i := 1; i < len(chunks); i++ {
		previousChunk := &chunks[i-1]
		currentChunk := &chunks[i]

		// Extract ending context from previous chunk
		overlapContext := lfc.extractSplitOverlapContext(previousChunk, config.OverlapSize)
		if overlapContext == "" {
			continue
		}

		// Add to preserved context as preceding context
		if currentChunk.PreservedContext.PrecedingContext == "" {
			currentChunk.PreservedContext.PrecedingContext = overlapContext
		} else {
			currentChunk.PreservedContext.PrecedingContext = overlapContext + "\n" + currentChunk.PreservedContext.PrecedingContext
		}

		slogger.Debug(ctx, "Added split chunk overlap", slogger.Fields{
			"chunk_id":     currentChunk.ID,
			"overlap_size": len(overlapContext),
		})
	}
}

// extractSplitOverlapContext extracts the ending portion of a chunk for overlap context.
// For split functions, we want to preserve the last few statements/lines as context.
func (lfc *LargeFunctionChunker) extractSplitOverlapContext(
	chunk *outbound.EnhancedCodeChunk,
	maxSize int,
) string {
	if chunk == nil || maxSize <= 0 || chunk.Content == "" {
		return ""
	}

	content := chunk.Content
	lines := strings.Split(content, "\n")

	// Extract last N lines that fit within maxSize
	var overlapLines []string
	currentSize := 0

	// Work backwards from the end
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		lineSize := len(line) + 1 // +1 for newline

		if currentSize+lineSize > maxSize {
			break
		}

		// Prepend to maintain order
		overlapLines = append([]string{line}, overlapLines...)
		currentSize += lineSize
	}

	if len(overlapLines) == 0 {
		return ""
	}

	return strings.Join(overlapLines, "\n")
}

// Mock implementation for basic testing.
type MockTreeSitterParserFactory struct{}

func (m *MockTreeSitterParserFactory) CreateParser(language valueobject.Language) (TreeSitterParser, error) {
	return &MockTreeSitterParser{language: language}, nil
}

func (m *MockTreeSitterParserFactory) GetSupportedLanguages() []valueobject.Language {
	go_lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
	python_lang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
	js_lang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	return []valueobject.Language{go_lang, python_lang, js_lang}
}

type MockTreeSitterParser struct {
	language valueobject.Language
}

func (m *MockTreeSitterParser) Parse(ctx context.Context, content string) (*valueobject.ParseTree, error) {
	// Mock parse tree creation
	return &valueobject.ParseTree{}, nil
}

func (m *MockTreeSitterParser) IdentifyBoundaries(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
) ([]SemanticBoundary, error) {
	// Return some basic boundaries for testing
	return []SemanticBoundary{
		{
			Type:      outbound.BoundaryStatement,
			Position:  StatementComplete,
			CanSplit:  true,
			Reason:    "Statement boundary",
			StartByte: 0,
			EndByte:   100,
		},
	}, nil
}

func (m *MockTreeSitterParser) ExtractContext(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
) ([]ContextElement, error) {
	// Return basic context for testing
	return []ContextElement{
		{
			Type:     ContextSignature,
			Content:  "func example()",
			Required: true,
			Reason:   "Function signature",
		},
	}, nil
}
