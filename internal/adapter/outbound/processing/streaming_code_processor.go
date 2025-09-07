package streamingcodeprocessor

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/application/worker/pipeline"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Interfaces.
type StreamingFileReader interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Close() error
}

type MemoryLimitEnforcer interface {
	EnforceLimit(ctx context.Context, limit int64) error
}

type LargeFileDetector interface {
	DetectFileSizeCategory(size int64) treesitter.FileSizeCategory
	SelectProcessingStrategy(
		category treesitter.FileSizeCategory,
		lang valueobject.Language,
		fileSize int64,
	) treesitter.ProcessingStrategy
}

type MemoryUsageTracker interface {
	TrackUsage(ctx context.Context, usage int64) error
	GetCurrentUsage() int64
}

type TreeSitterParser interface {
	Parse(ctx context.Context, content []byte) ([]*SyntaxNode, error)
}

type SyntaxNode struct {
	NodeType string
	Content  string
}

type Chunker interface {
	Chunk(ctx context.Context, nodes []*SyntaxNode) ([]outbound.EnhancedCodeChunk, error)
}

// StreamingCodeProcessor implementation.
type StreamingCodeProcessor struct {
	reader   StreamingFileReader
	enforcer MemoryLimitEnforcer
	detector LargeFileDetector
	tracker  MemoryUsageTracker

	parsers  map[string]TreeSitterParser
	chunkers map[outbound.ChunkingStrategyType]Chunker

	metrics *outbound.ProcessingMetrics
	mutex   sync.RWMutex
	closed  bool

	bufferPool sync.Pool

	// Resilience components
	errorClassifier *pipeline.ProcessingErrorClassifier
	circuitBreaker  *pipeline.ProcessingCircuitBreaker
	retryManager    *pipeline.ProcessingRetryManager
	fallbackManager *pipeline.ProcessingFallbackManager
	logger          *slog.Logger
}

func NewStreamingCodeProcessor(
	reader StreamingFileReader,
	enforcer MemoryLimitEnforcer,
	detector LargeFileDetector,
	tracker MemoryUsageTracker,
) *StreamingCodeProcessor {
	// Initialize resilience components
	errorClassifier := &pipeline.ProcessingErrorClassifier{}

	circuitBreaker := pipeline.NewProcessingCircuitBreaker(
		"streaming_code_processor",
		5, // failure threshold
		3, // success threshold
		30*time.Second,
		nil, // meter - would be provided in real implementation
	)

	retryConfig := pipeline.ProcessingRetryConfig{
		MaxRetries:  3,
		Strategy:    pipeline.ProcessingExponentialBackoff,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Jitter:      true,
		RetryBudget: 100,
	}
	retryManager := pipeline.NewProcessingRetryManager(retryConfig)

	logger := slog.Default()
	fallbackManager := pipeline.NewProcessingFallbackManager(logger)

	return &StreamingCodeProcessor{
		reader:   reader,
		enforcer: enforcer,
		detector: detector,
		tracker:  tracker,
		parsers:  make(map[string]TreeSitterParser),
		chunkers: make(map[outbound.ChunkingStrategyType]Chunker),
		metrics: &outbound.ProcessingMetrics{
			FilesProcessed:        0,
			BytesProcessed:        0,
			ChunksGenerated:       0,
			AverageProcessingTime: 0,
			MemoryPeakUsage:       0,
			MemoryCurrentUsage:    0,
			ErrorCount:            0,
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 0, 1024)
				return &buf
			},
		},
		errorClassifier: errorClassifier,
		circuitBreaker:  circuitBreaker,
		retryManager:    retryManager,
		fallbackManager: fallbackManager,
		logger:          logger,
	}
}

func (p *StreamingCodeProcessor) processFileValidation(ctx context.Context) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		errCtx := pipeline.NewProcessingErrorContext("StreamingCodeProcessor", "processFileValidation", nil)
		return pipeline.CreateEnhancedPipelineError(
			errCtx,
			pipeline.ProcessingContextCancelled,
			pipeline.ProcessingTransientError,
			"context cancelled during file validation",
			false,
			ctx.Err(),
		)
	default:
	}

	// Check if processor is closed
	p.mutex.RLock()
	if p.closed {
		p.mutex.RUnlock()
		errCtx := pipeline.NewProcessingErrorContext("StreamingCodeProcessor", "processFileValidation", nil)
		return pipeline.CreateEnhancedPipelineError(
			errCtx,
			pipeline.ProcessingInvalidConfiguration,
			pipeline.ProcessingPermanentError,
			"processor is closed",
			false,
			errors.New("processor is closed"),
		)
	}
	p.mutex.RUnlock()

	return nil
}

func (p *StreamingCodeProcessor) processFileContent(
	ctx context.Context,
	config *outbound.ProcessingConfig,
) ([]byte, valueobject.Language, error) {
	var content []byte
	lang := config.Language

	err := p.circuitBreaker.Execute(ctx, func() error {
		return p.retryManager.ExecuteWithRetry(ctx, func() error {
			// Track memory usage
			if err := p.tracker.TrackUsage(ctx, 1000000); err != nil {
				errCtx := pipeline.NewProcessingErrorContext(
					"StreamingCodeProcessor",
					"processFileContent",
					map[string]interface{}{
						"file_path": config.FilePath,
					},
				)
				return pipeline.CreateEnhancedPipelineError(
					errCtx,
					pipeline.ProcessingMemoryLimitExceededError,
					pipeline.ProcessingResourceError,
					"memory tracking failed",
					true,
					err,
				)
			}

			// Read file content
			var readErr error
			content, readErr = p.reader.ReadFile(ctx, config.FilePath)
			if readErr != nil {
				errCtx := pipeline.NewProcessingErrorContext(
					"StreamingCodeProcessor",
					"processFileContent",
					map[string]interface{}{
						"file_path": config.FilePath,
					},
				)
				return pipeline.CreateEnhancedPipelineError(
					errCtx,
					pipeline.ProcessingComponentFailure,
					pipeline.ProcessingTransientError,
					"failed to read file content",
					true,
					readErr,
				)
			}

			// Check if content is binary and adjust language if needed
			if isBinaryContent(content) {
				lang, _ = valueobject.NewLanguage(valueobject.LanguageUnknown)
			}

			return nil
		})
	})
	if err != nil {
		return nil, lang, err
	}

	return content, lang, nil
}

func (p *StreamingCodeProcessor) processFileParsing(
	ctx context.Context,
	content []byte,
	lang valueobject.Language,
) ([]*SyntaxNode, outbound.ProcessingStrategy, error) {
	var nodes []*SyntaxNode
	var strategy outbound.ProcessingStrategy
	var parseErr error

	err := p.circuitBreaker.Execute(ctx, func() error {
		return p.retryManager.ExecuteWithRetry(ctx, func() error {
			// Detect file size category and select processing strategy
			category := p.detector.DetectFileSizeCategory(int64(len(content)))
			tsStrategy := p.detector.SelectProcessingStrategy(category, lang, int64(len(content)))

			switch tsStrategy {
			case treesitter.ProcessingStrategyMemory:
				strategy = outbound.ProcessingStrategyMemory
			case treesitter.ProcessingStrategyStreaming:
				strategy = outbound.ProcessingStrategyStreaming
			case treesitter.ProcessingStrategyChunked:
				strategy = outbound.ProcessingStrategyChunked
			case treesitter.ProcessingStrategyFallback:
				strategy = outbound.ProcessingStrategyFallback
			default:
				strategy = outbound.ProcessingStrategyFallback
			}

			// Get parser for language
			parser, ok := p.parsers[lang.String()]
			if !ok {
				errCtx := pipeline.NewProcessingErrorContext(
					"StreamingCodeProcessor",
					"processFileParsing",
					map[string]interface{}{
						"language": lang.String(),
					},
				)
				return pipeline.CreateEnhancedPipelineError(
					errCtx,
					pipeline.ProcessingDependencyUnavailable,
					pipeline.ProcessingPermanentError,
					"unsupported language parser",
					false,
					errors.New("unsupported language"),
				)
			}

			// Parse content
			nodes, parseErr = parser.Parse(ctx, content)
			if parseErr != nil {
				errCtx := pipeline.NewProcessingErrorContext(
					"StreamingCodeProcessor",
					"processFileParsing",
					map[string]interface{}{
						"language":       lang.String(),
						"content_length": len(content),
					},
				)
				return pipeline.CreateEnhancedPipelineError(
					errCtx,
					pipeline.ProcessingDataCorruption,
					pipeline.ProcessingTransientError,
					"failed to parse content",
					true,
					parseErr,
				)
			}

			return nil
		})
	})
	if err != nil {
		return nil, strategy, err
	}

	return nodes, strategy, nil
}

func (p *StreamingCodeProcessor) processFileChunking(
	ctx context.Context,
	config *outbound.ProcessingConfig,
	nodes []*SyntaxNode,
) ([]outbound.EnhancedCodeChunk, error) {
	var chunks []outbound.EnhancedCodeChunk
	var chunkErr error

	err := p.circuitBreaker.Execute(ctx, func() error {
		return p.retryManager.ExecuteWithRetry(ctx, func() error {
			// Get chunker for strategy
			chunker, ok := p.chunkers[config.ChunkingStrategy]
			if !ok {
				errCtx := pipeline.NewProcessingErrorContext(
					"StreamingCodeProcessor",
					"processFileChunking",
					map[string]interface{}{
						"chunking_strategy": string(config.ChunkingStrategy),
					},
				)
				return pipeline.CreateEnhancedPipelineError(
					errCtx,
					pipeline.ProcessingDependencyUnavailable,
					pipeline.ProcessingPermanentError,
					"unsupported chunking strategy",
					false,
					errors.New("unsupported chunking strategy"),
				)
			}

			// Chunk parsed nodes
			chunks, chunkErr = chunker.Chunk(ctx, nodes)
			if chunkErr != nil {
				errCtx := pipeline.NewProcessingErrorContext(
					"StreamingCodeProcessor",
					"processFileChunking",
					map[string]interface{}{
						"chunking_strategy": string(config.ChunkingStrategy),
						"nodes_count":       len(nodes),
					},
				)
				return pipeline.CreateEnhancedPipelineError(
					errCtx,
					pipeline.ProcessingDataCorruption,
					pipeline.ProcessingTransientError,
					"failed to chunk parsed nodes",
					true,
					chunkErr,
				)
			}

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return chunks, nil
}

func (p *StreamingCodeProcessor) ProcessFile(
	ctx context.Context,
	config *outbound.ProcessingConfig,
) (*outbound.ProcessingResult, error) {
	startTime := time.Now()
	result := &outbound.ProcessingResult{
		FilePath: config.FilePath,
		Errors:   make([]error, 0),
	}

	// Initial validation
	if err := p.processFileValidation(ctx); err != nil {
		result.Errors = append(result.Errors, err)
		result.Success = false
		p.logger.ErrorContext(ctx, "File processing validation failed",
			"file_path", config.FilePath,
			"error", err.Error())
		return result, nil
	}

	// Read and process file content with fallback
	content, lang, err := p.processFileContent(ctx, config)
	if err != nil {
		result.Errors = append(result.Errors, err)
		result.Success = false
		result.Strategy = outbound.ProcessingStrategyFallback
		p.logger.ErrorContext(ctx, "File content processing failed",
			"file_path", config.FilePath,
			"error", err.Error())
		return result, nil
	}
	defer p.releaseBuffer(content)

	// Parse the content with fallback
	nodes, strategy, err := p.processFileParsing(ctx, content, lang)
	if err != nil {
		result.Errors = append(result.Errors, err)
		result.Success = false
		result.Strategy = strategy
		p.logger.ErrorContext(ctx, "File parsing failed",
			"file_path", config.FilePath,
			"strategy", strategy,
			"error", err.Error())
		return result, nil
	}
	defer p.releaseSyntaxNodes(nodes)

	// Chunk the parsed nodes with fallback
	chunks, err := p.processFileChunking(ctx, config, nodes)
	if err != nil {
		result.Errors = append(result.Errors, err)
		result.Success = false
		result.Strategy = strategy
		p.logger.ErrorContext(ctx, "File chunking failed",
			"file_path", config.FilePath,
			"strategy", strategy,
			"error", err.Error())
		return result, nil
	}
	defer p.releaseChunks(chunks)

	result.ChunksGenerated = chunks
	result.Success = true
	result.BytesProcessed = int64(len(content))
	result.ProcessingTime = time.Since(startTime)
	result.Strategy = strategy

	// Update metrics
	p.updateMetrics(result)

	return result, nil
}

func (p *StreamingCodeProcessor) ProcessDirectory(
	ctx context.Context,
	config *outbound.DirectoryProcessingConfig,
) (*outbound.DirectoryProcessingResult, error) {
	result := &outbound.DirectoryProcessingResult{
		DirectoryPath: config.DirectoryPath,
		FileResults:   make([]*outbound.ProcessingResult, 0),
		Errors:        make([]error, 0),
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		errCtx := pipeline.NewProcessingErrorContext(
			"StreamingCodeProcessor",
			"ProcessDirectory",
			map[string]interface{}{
				"directory_path": config.DirectoryPath,
			},
		)
		return nil, pipeline.CreateEnhancedPipelineError(
			errCtx,
			pipeline.ProcessingContextCancelled,
			pipeline.ProcessingTransientError,
			"context cancelled during directory processing",
			false,
			ctx.Err(),
		)
	default:
	}

	err := filepath.Walk(config.DirectoryPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			p.logger.ErrorContext(ctx, "Error walking directory",
				"path", path,
				"error", err.Error())
			result.Errors = append(result.Errors, fmt.Errorf("failed to access path %s: %w", path, err))
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			fileConfig := &outbound.ProcessingConfig{
				FilePath:         path,
				Language:         config.ProcessingConfig.Language,
				ChunkingStrategy: config.ProcessingConfig.ChunkingStrategy,
				MemoryLimits:     config.ProcessingConfig.MemoryLimits,
			}

			fileResult, err := p.ProcessFile(ctx, fileConfig)
			if err != nil {
				p.logger.ErrorContext(ctx, "File processing failed",
					"file_path", path,
					"error", err.Error())
				result.Errors = append(result.Errors, fmt.Errorf("failed to process file %s: %w", path, err))
				return nil
			}

			result.FileResults = append(result.FileResults, fileResult)
		}

		return nil
	})
	if err != nil {
		errCtx := pipeline.NewProcessingErrorContext(
			"StreamingCodeProcessor",
			"ProcessDirectory",
			map[string]interface{}{
				"directory_path": config.DirectoryPath,
			},
		)
		wrappedErr := pipeline.CreateEnhancedPipelineError(
			errCtx,
			pipeline.ProcessingComponentFailure,
			pipeline.ProcessingTransientError,
			"directory walk failed",
			true,
			err,
		)
		result.Errors = append(result.Errors, wrappedErr)
	}

	result.FilesProcessed = len(result.FileResults)
	result.Success = len(result.Errors) == 0

	return result, nil
}

func (p *StreamingCodeProcessor) GetMetrics(ctx context.Context) (*outbound.ProcessingMetrics, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.closed {
		errCtx := pipeline.NewProcessingErrorContext("StreamingCodeProcessor", "GetMetrics", nil)
		return nil, pipeline.CreateEnhancedPipelineError(
			errCtx,
			pipeline.ProcessingInvalidConfiguration,
			pipeline.ProcessingPermanentError,
			"processor is closed",
			false,
			errors.New("processor is closed"),
		)
	}

	currentUsage := p.tracker.GetCurrentUsage()
	memoryPeakUsage := currentUsage
	memoryCurrentUsage := currentUsage

	return &outbound.ProcessingMetrics{
		FilesProcessed:        p.metrics.FilesProcessed,
		BytesProcessed:        p.metrics.BytesProcessed,
		ChunksGenerated:       p.metrics.ChunksGenerated,
		AverageProcessingTime: p.metrics.AverageProcessingTime,
		MemoryPeakUsage:       memoryPeakUsage,
		MemoryCurrentUsage:    memoryCurrentUsage,
		ErrorCount:            p.metrics.ErrorCount,
	}, nil
}

func (p *StreamingCodeProcessor) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	return p.reader.Close()
}

// Helper functions for binary detection with early termination.
func hasNullBytes(content []byte) bool {
	sampleSize := 512
	if len(content) < sampleSize {
		sampleSize = len(content)
	}

	for i := range sampleSize {
		if content[i] == 0x00 {
			return true
		}
	}
	return false
}

func matchesBinarySignature(content []byte) bool {
	if len(content) < 4 {
		return false
	}

	binarySignatures := [][]byte{
		{0x7f, 0x45, 0x4c, 0x46}, // ELF
		{0xff, 0xd8, 0xff},       // JPEG
		{0x89, 0x50, 0x4e, 0x47}, // PNG
		{0x50, 0x4b, 0x03, 0x04}, // ZIP
		{0x25, 0x50, 0x44, 0x46}, // PDF
		{0x52, 0x61, 0x72, 0x21}, // RAR
	}

	for _, signature := range binarySignatures {
		if len(content) >= len(signature) {
			match := true
			for i, b := range signature {
				if content[i] != b {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func hasHighNonPrintableRatio(content []byte) bool {
	sampleSize := 1024
	if len(content) < sampleSize {
		sampleSize = len(content)
	}

	if sampleSize == 0 {
		return false
	}

	nonPrintableCount := 0
	for i := range sampleSize {
		b := content[i]
		if b < 0x20 || b > 0x7e {
			nonPrintableCount++
		}
	}

	return float64(nonPrintableCount)/float64(sampleSize) > 0.3
}

func isBinaryContent(content []byte) bool {
	return hasNullBytes(content) || matchesBinarySignature(content) || hasHighNonPrintableRatio(content)
}

// Memory management helpers.
func (p *StreamingCodeProcessor) releaseBuffer(buf []byte) {
	if cap(buf) <= 1024*1024 { // Only pool buffers up to 1MB
		p.bufferPool.Put(&buf)
	}
}

func (p *StreamingCodeProcessor) releaseSyntaxNodes(nodes []*SyntaxNode) {
	// In a real implementation, we might pool SyntaxNode slices
	// For now, we just let them be garbage collected
}

func (p *StreamingCodeProcessor) releaseChunks(chunks []outbound.EnhancedCodeChunk) {
	// In a real implementation, we might pool EnhancedCodeChunk slices
	// For now, we just let them be garbage collected
}

// Metrics update helper.
func (p *StreamingCodeProcessor) updateMetrics(result *outbound.ProcessingResult) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.FilesProcessed++
	p.metrics.BytesProcessed += result.BytesProcessed
	p.metrics.ChunksGenerated += int64(len(result.ChunksGenerated))

	// Update average processing time
	totalTime := p.metrics.AverageProcessingTime*time.Duration(p.metrics.FilesProcessed-1) + result.ProcessingTime
	p.metrics.AverageProcessingTime = totalTime / time.Duration(p.metrics.FilesProcessed)

	currentUsage := p.tracker.GetCurrentUsage()
	if currentUsage > p.metrics.MemoryPeakUsage {
		p.metrics.MemoryPeakUsage = currentUsage
	}
	p.metrics.MemoryCurrentUsage = currentUsage

	if !result.Success {
		p.metrics.ErrorCount++
	}
}

// Mock implementations for testing.
type MockStreamingFileReader struct {
	content map[string][]byte
}

func NewMockStreamingFileReader() *MockStreamingFileReader {
	return &MockStreamingFileReader{
		content: make(map[string][]byte),
	}
}

func (m *MockStreamingFileReader) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if content, ok := m.content[path]; ok {
		buf := make([]byte, len(content))
		copy(buf, content)
		return buf, nil
	}
	return nil, errors.New("file not found")
}

func (m *MockStreamingFileReader) Close() error {
	return nil
}

func (m *MockStreamingFileReader) SetFileContent(path string, content []byte) {
	m.content[path] = content
}

type MockMemoryLimitEnforcer struct{}

func (m *MockMemoryLimitEnforcer) EnforceLimit(ctx context.Context, limit int64) error {
	return nil
}

type MockLargeFileDetector struct{}

func (m *MockLargeFileDetector) DetectFileSizeCategory(size int64) treesitter.FileSizeCategory {
	if size > 1000000 {
		return treesitter.FileSizeVeryLarge
	}
	if size > 100000 {
		return treesitter.FileSizeLarge
	}
	return treesitter.FileSizeSmall
}

func (m *MockLargeFileDetector) SelectProcessingStrategy(
	category treesitter.FileSizeCategory,
	lang valueobject.Language,
	fileSize int64,
) treesitter.ProcessingStrategy {
	switch category {
	case treesitter.FileSizeVeryLarge:
		return treesitter.ProcessingStrategyStreaming
	case treesitter.FileSizeLarge:
		return treesitter.ProcessingStrategyChunked
	case treesitter.FileSizeSmall:
		return treesitter.ProcessingStrategyMemory
	default:
		return treesitter.ProcessingStrategyMemory
	}
}

type MockMemoryUsageTracker struct {
	currentUsage int64
	mutex        sync.RWMutex
}

func (m *MockMemoryUsageTracker) TrackUsage(ctx context.Context, usage int64) error {
	m.mutex.Lock()
	m.currentUsage += usage
	m.mutex.Unlock()
	return nil
}

func (m *MockMemoryUsageTracker) GetCurrentUsage() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentUsage
}

type MockTreeSitterParser struct {
	nodes []*SyntaxNode
}

func NewMockTreeSitterParser() *MockTreeSitterParser {
	return &MockTreeSitterParser{
		nodes: make([]*SyntaxNode, 0),
	}
}

func (m *MockTreeSitterParser) Parse(ctx context.Context, content []byte) ([]*SyntaxNode, error) {
	result := make([]*SyntaxNode, len(m.nodes))
	copy(result, m.nodes)
	return result, nil
}

func (m *MockTreeSitterParser) SetNodes(nodes []*SyntaxNode) {
	m.nodes = nodes
}

type MockChunker struct {
	chunks []outbound.EnhancedCodeChunk
}

func NewMockChunker() *MockChunker {
	return &MockChunker{
		chunks: make([]outbound.EnhancedCodeChunk, 0),
	}
}

func (m *MockChunker) Chunk(ctx context.Context, nodes []*SyntaxNode) ([]outbound.EnhancedCodeChunk, error) {
	result := make([]outbound.EnhancedCodeChunk, len(m.chunks))
	copy(result, m.chunks)
	return result, nil
}

func (m *MockChunker) SetChunks(chunks []outbound.EnhancedCodeChunk) {
	m.chunks = chunks
}
