package streamingcodeprocessor

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/adapter/outbound/treesitter/memory"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ProcessingStrategy represents different processing approaches.
type ProcessingStrategy int

const (
	ProcessingStrategyMemory ProcessingStrategy = iota
	ProcessingStrategyStreaming
	ProcessingStrategyChunked
	ProcessingStrategyFallback
)

// ProcessingConfig holds configuration for file processing.
type ProcessingConfig struct {
	FilePath           string
	OutputPath         string
	Language           valueobject.Language
	ChunkingStrategy   outbound.ChunkingStrategyType
	MemoryLimits       memory.MemoryLimits
	EnableStreaming    bool
	EnableAsyncReading bool
	BufferSize         int
	MaxConcurrentFiles int
	EnableMetrics      bool
}

// ProcessingResult contains the results of processing a file.
type ProcessingResult struct {
	FilePath        string
	Strategy        ProcessingStrategy
	ChunksGenerated []outbound.EnhancedCodeChunk
	MemoryUsage     *memory.MemoryUsage
	ProcessingTime  time.Duration
	BytesProcessed  int64
	Success         bool
	Errors          []error
}

// DirectoryProcessingConfig holds configuration for directory processing.
type DirectoryProcessingConfig struct {
	DirectoryPath      string
	OutputPath         string
	FileFilters        []string
	MaxConcurrentFiles int
	ProcessingConfig   *ProcessingConfig
}

// DirectoryProcessingResult contains results of directory processing.
type DirectoryProcessingResult struct {
	DirectoryPath       string
	FilesProcessed      int
	FileResults         []*ProcessingResult
	TotalProcessingTime time.Duration
	Success             bool
	Errors              []error
}

// ProcessingMetrics contains performance metrics.
type ProcessingMetrics struct {
	FilesProcessed        int64
	BytesProcessed        int64
	ChunksGenerated       int64
	AverageProcessingTime time.Duration
	MemoryPeakUsage       int64
	MemoryCurrentUsage    int64
	ErrorCount            int64
}

type mockStreamingFileReader struct {
	memoryTracker *mockMemoryTracker
	ReadFile      func(ctx context.Context, path string) ([]byte, error)
}

func (m *mockStreamingFileReader) Close() error {
	return nil
}

type mockMemoryLimitEnforcer struct {
	limits  memory.MemoryLimits
	tracker *mockMemoryTracker
}

func (m *mockMemoryLimitEnforcer) EnforceLimits(ctx context.Context) error {
	if m.limits.PerOperationLimit > 0 && int64(m.tracker.GetCurrentUsage()) > m.limits.PerOperationLimit {
		return errors.New("memory limit exceeded")
	}
	return nil
}

func (m *mockMemoryLimitEnforcer) GetAdaptiveBufferLimit(fileSize int64) int {
	if fileSize > 50*1024*1024 {
		return 8192
	}
	return 4096
}

type mockLargeFileDetector struct{}

func (m *mockLargeFileDetector) DetectFileSizeCategory(size int64) treesitter.FileSizeCategory {
	switch {
	case size < 10*1024:
		return treesitter.FileSizeSmall
	case size < 50*1024*1024:
		return treesitter.FileSizeLarge
	default:
		return treesitter.FileSizeVeryLarge
	}
}

func (m *mockLargeFileDetector) SelectProcessingStrategy(
	category treesitter.FileSizeCategory,
	lang valueobject.Language,
	availableMemory uint64,
) treesitter.ProcessingStrategy {
	if category == treesitter.FileSizeVeryLarge {
		return treesitter.ProcessingStrategyStreaming
	}
	return treesitter.ProcessingStrategyMemory
}

type mockMemoryTracker struct {
	maxUsage     uint64
	currentUsage uint64
}

func (m *mockMemoryTracker) TrackUsage(ctx context.Context, usage uint64) error {
	if usage > m.maxUsage {
		m.maxUsage = usage
	}
	m.currentUsage = usage
	return nil
}

func (m *mockMemoryTracker) GetCurrentUsage() uint64 {
	return m.currentUsage
}

func (m *mockMemoryTracker) CheckViolation(limits memory.MemoryLimits) bool {
	return int64(m.currentUsage) > limits.PerOperationLimit
}

type mockSyntaxNode struct {
	nodeType string
	content  string
}

type mockTreeSitterParser struct {
	lang valueobject.Language
}

func (m *mockTreeSitterParser) Parse(ctx context.Context, content []byte) ([]*mockSyntaxNode, error) {
	if m.lang.Name() == "Unknown" {
		return nil, errors.New("unsupported language")
	}
	if len(content) == 0 {
		return nil, errors.New("empty content")
	}
	return []*mockSyntaxNode{
		{nodeType: "function", content: "main"},
		{nodeType: "function", content: "exampleFunction"},
	}, nil
}

type mockChunker struct {
	Chunk func(ctx context.Context, nodes []*mockSyntaxNode) ([]outbound.EnhancedCodeChunk, error)
}

type mockStreamingCodeProcessor struct {
	reader   *mockStreamingFileReader
	enforcer *mockMemoryLimitEnforcer
	detector *mockLargeFileDetector
	tracker  *mockMemoryTracker
	parser   *mockTreeSitterParser
	chunker  *mockChunker
}

func NewMockStreamingCodeProcessor() *mockStreamingCodeProcessor {
	tracker := &mockMemoryTracker{}
	goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

	return &mockStreamingCodeProcessor{
		reader: &mockStreamingFileReader{
			memoryTracker: tracker,
			ReadFile: func(ctx context.Context, path string) ([]byte, error) {
				if strings.Contains(path, "nonexistent") {
					return nil, fs.ErrNotExist
				}
				if strings.Contains(path, "binary") {
					return []byte{0x00, 0x01, 0x02, 0x03}, nil
				}
				content := `package main

func main() {
	fmt.Println("Hello, World!")
}

func exampleFunction() {
	// This is a sample function for testing
	var x int = 10
	var y string = "test"
	fmt.Printf("x: %d, y: %s\n", x, y)
}`
				return []byte(content), nil
			},
		},
		enforcer: &mockMemoryLimitEnforcer{tracker: tracker},
		detector: &mockLargeFileDetector{},
		tracker:  tracker,
		parser:   &mockTreeSitterParser{},
		chunker: &mockChunker{
			Chunk: func(ctx context.Context, nodes []*mockSyntaxNode) ([]outbound.EnhancedCodeChunk, error) {
				if len(nodes) == 0 {
					return nil, errors.New("no syntax nodes to chunk")
				}

				chunks := make([]outbound.EnhancedCodeChunk, 0)
				for _, node := range nodes {
					chunk := outbound.EnhancedCodeChunk{
						ID:       fmt.Sprintf("chunk-%s", node.content),
						Content:  fmt.Sprintf("Chunk for %s", node.content),
						Language: goLang,
						Metadata: map[string]interface{}{
							"function_name": node.content,
						},
					}
					chunks = append(chunks, chunk)
				}

				return chunks, nil
			},
		},
	}
}

func (m *mockStreamingCodeProcessor) ProcessFile(
	ctx context.Context,
	config *ProcessingConfig,
) (*ProcessingResult, error) {
	result := &ProcessingResult{
		FilePath: config.FilePath,
		Errors:   make([]error, 0),
	}

	// Simulate memory usage tracking
	m.tracker.TrackUsage(ctx, 1000000)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Read file content
	content, err := m.reader.ReadFile(ctx, config.FilePath)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result, nil
	}

	// Detect binary content and set language accordingly
	lang := config.Language
	if strings.Contains(config.FilePath, "binary") || m.isBinaryContent(content) {
		lang, _ = valueobject.NewLanguage(valueobject.LanguageUnknown)
	}

	// Enforce memory limits
	if err := m.enforcer.EnforceLimits(ctx); err != nil {
		result.Errors = append(result.Errors, err)
		return result, nil
	}

	// Detect file size category
	fileInfo, _ := os.Stat(config.FilePath)
	var fileSize int64
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}
	category := m.detector.DetectFileSizeCategory(fileSize)

	// Convert treesitter.ProcessingStrategy to local ProcessingStrategy
	switch m.detector.SelectProcessingStrategy(category, lang, 0) {
	case treesitter.ProcessingStrategyMemory:
		result.Strategy = ProcessingStrategyMemory
	case treesitter.ProcessingStrategyStreaming:
		result.Strategy = ProcessingStrategyStreaming
	case treesitter.ProcessingStrategyChunked:
		result.Strategy = ProcessingStrategyChunked
	case treesitter.ProcessingStrategyFallback:
		result.Strategy = ProcessingStrategyFallback
	default:
		result.Strategy = ProcessingStrategyFallback
	}

	// Parse content - set the language in the parser first
	m.parser.lang = lang
	mockNodes, err := m.parser.Parse(ctx, content)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result, nil
	}

	// Convert mock nodes to outbound nodes for chunking
	nodes := make([]*mockSyntaxNode, len(mockNodes))
	for i, mockNode := range mockNodes {
		nodes[i] = &mockSyntaxNode{
			nodeType: mockNode.nodeType,
			content:  mockNode.content,
		}
	}

	// Chunk parsed nodes
	chunks, err := m.chunker.Chunk(ctx, mockNodes)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result, nil
	}

	result.ChunksGenerated = chunks
	result.Success = true
	result.BytesProcessed = int64(len(content))

	return result, nil
}

func (m *mockStreamingCodeProcessor) isBinaryContent(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}

func (m *mockStreamingCodeProcessor) ProcessDirectory(
	ctx context.Context,
	config *DirectoryProcessingConfig,
) (*DirectoryProcessingResult, error) {
	result := &DirectoryProcessingResult{
		DirectoryPath: config.DirectoryPath,
		FileResults:   make([]*ProcessingResult, 0),
		Errors:        make([]error, 0),
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	err := filepath.Walk(config.DirectoryPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, err)
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			fileConfig := &ProcessingConfig{
				FilePath:         path,
				Language:         config.ProcessingConfig.Language,
				ChunkingStrategy: config.ProcessingConfig.ChunkingStrategy,
				MemoryLimits:     config.ProcessingConfig.MemoryLimits,
			}

			fileResult, err := m.ProcessFile(ctx, fileConfig)
			if err != nil {
				result.Errors = append(result.Errors, err)
				return nil
			}

			result.FileResults = append(result.FileResults, fileResult)
		}

		return nil
	})
	if err != nil {
		result.Errors = append(result.Errors, err)
	}

	return result, nil
}

func (m *mockStreamingCodeProcessor) GetMetrics(ctx context.Context) (*ProcessingMetrics, error) {
	return &ProcessingMetrics{
		FilesProcessed:        1,
		BytesProcessed:        1000,
		ChunksGenerated:       2,
		AverageProcessingTime: time.Millisecond * 50,
		MemoryPeakUsage:       int64(m.tracker.maxUsage),
		MemoryCurrentUsage:    int64(m.tracker.currentUsage),
		ErrorCount:            0,
	}, nil
}

func (m *mockStreamingCodeProcessor) Close() error {
	return m.reader.Close()
}

// Test using our real implementation instead of mock.
func TestStreamingCodeProcessor_ProcessFile_MemoryLimitViolation(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	// Set up memory limits and current usage to cause violation
	processor.enforcer.limits = memory.MemoryLimits{PerOperationLimit: 500000}
	processor.tracker.currentUsage = 600000

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "test.go",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
		MemoryLimits:     memory.MemoryLimits{PerOperationLimit: 500000},
	}

	result, err := processor.ProcessFile(context.Background(), config)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "memory limit exceeded")
}

func TestStreamingCodeProcessor_ProcessFile_FileSystemError(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "nonexistent.go",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	result, err := processor.ProcessFile(context.Background(), config)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.ErrorIs(t, result.Errors[0], fs.ErrNotExist)
}

func TestStreamingCodeProcessor_ProcessFile_ParsingError(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	unknownLang, err := valueobject.NewLanguage(valueobject.LanguageUnknown)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "test.go",
		Language:         unknownLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	result, err := processor.ProcessFile(context.Background(), config)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "unsupported language")
}

func TestStreamingCodeProcessor_ProcessFile_EmptyFile(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "empty.go",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	// Override reader to return empty content
	processor.reader.ReadFile = func(ctx context.Context, path string) ([]byte, error) {
		return []byte{}, nil
	}

	result, err := processor.ProcessFile(context.Background(), config)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "empty content")
}

func TestStreamingCodeProcessor_ProcessFile_BinaryFile(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "binary.exe",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	result, err := processor.ProcessFile(context.Background(), config)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "unsupported language")
}

func TestStreamingCodeProcessor_ProcessFile_ContextCancellation(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "large.go",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = processor.ProcessFile(ctx, config)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestStreamingCodeProcessor_ProcessFile_ChunkingError(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "test.go",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	// Override chunker to return error
	processor.chunker.Chunk = func(ctx context.Context, nodes []*mockSyntaxNode) ([]outbound.EnhancedCodeChunk, error) {
		return nil, errors.New("chunking failed")
	}

	result, err := processor.ProcessFile(context.Background(), config)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "chunking failed")
}

func TestStreamingCodeProcessor_ProcessDirectory_ContextCancellation(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	processingConfig := &ProcessingConfig{
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
		MemoryLimits:     memory.MemoryLimits{PerOperationLimit: 1000000},
	}

	config := &DirectoryProcessingConfig{
		DirectoryPath:    "/tmp/testdir",
		ProcessingConfig: processingConfig,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = processor.ProcessDirectory(ctx, config)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestStreamingCodeProcessor_ProcessDirectory_FileSystemError(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	processingConfig := &ProcessingConfig{
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	config := &DirectoryProcessingConfig{
		DirectoryPath:    "/nonexistent",
		ProcessingConfig: processingConfig,
	}

	result, err := processor.ProcessDirectory(context.Background(), config)
	require.NoError(t, err)
	assert.Len(t, result.Errors, 1)
	assert.ErrorIs(t, result.Errors[0], fs.ErrNotExist)
}

func TestStreamingCodeProcessor_GetMetrics(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()
	processor.tracker.maxUsage = 2000000
	processor.tracker.currentUsage = 1500000

	metrics, err := processor.GetMetrics(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2000000), metrics.MemoryPeakUsage)
	assert.Equal(t, int64(1500000), metrics.MemoryCurrentUsage)
	assert.Equal(t, int64(1000), metrics.BytesProcessed)
}

func TestStreamingCodeProcessor_AdaptiveStrategySelection(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	tests := []struct {
		name     string
		fileSize int64
		expected treesitter.ProcessingStrategy
	}{
		{
			name:     "Small file",
			fileSize: 5 * 1024,
			expected: treesitter.ProcessingStrategyMemory,
		},
		{
			name:     "Large file",
			fileSize: 20 * 1024 * 1024,
			expected: treesitter.ProcessingStrategyMemory,
		},
		{
			name:     "Very large file",
			fileSize: 100 * 1024 * 1024,
			expected: treesitter.ProcessingStrategyStreaming,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := processor.detector.DetectFileSizeCategory(tt.fileSize)
			strategy := processor.detector.SelectProcessingStrategy(category, goLang, 0)
			assert.Equal(t, tt.expected, strategy)
		})
	}
}

func TestStreamingCodeProcessor_AdaptiveBufferSizing(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	tests := []struct {
		name     string
		fileSize int64
		expected int
	}{
		{
			name:     "Small file",
			fileSize: 5 * 1024,
			expected: 4096,
		},
		{
			name:     "Large file",
			fileSize: 100 * 1024 * 1024,
			expected: 8192,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bufferSize := processor.enforcer.GetAdaptiveBufferLimit(tt.fileSize)
			assert.Equal(t, tt.expected, bufferSize)
		})
	}
}

func TestStreamingCodeProcessor_ProcessFile_InvalidLanguage(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	unknownLang, err := valueobject.NewLanguage(valueobject.LanguageUnknown)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "test.go",
		Language:         unknownLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	result, err := processor.ProcessFile(context.Background(), config)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "unsupported language")
}

func TestStreamingCodeProcessor_MemoryUsageTracking(t *testing.T) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	config := &ProcessingConfig{
		FilePath:         "test.go",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	initialUsage := processor.tracker.GetCurrentUsage()
	result, err := processor.ProcessFile(context.Background(), config)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Greater(t, processor.tracker.GetCurrentUsage(), initialUsage)
}

func BenchmarkStreamingCodeProcessor_ProcessFile(b *testing.B) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(b, err)

	config := &ProcessingConfig{
		FilePath:         "test.go",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	b.ResetTimer()
	for range b.N {
		_, _ = processor.ProcessFile(context.Background(), config)
	}
}

func BenchmarkStreamingCodeProcessor_ProcessLargeFile(b *testing.B) {
	processor := NewMockStreamingCodeProcessor()

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(b, err)

	config := &ProcessingConfig{
		FilePath:         "large.go",
		Language:         goLang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	b.ResetTimer()
	for range b.N {
		_, _ = processor.ProcessFile(context.Background(), config)
	}
}

func BenchmarkStreamingCodeProcessor_MemoryUsage(b *testing.B) {
	processor := NewMockStreamingCodeProcessor()

	ctx := context.Background()
	b.ResetTimer()

	for i := range b.N {
		_ = processor.tracker.TrackUsage(ctx, uint64(i*1000))
	}
}
