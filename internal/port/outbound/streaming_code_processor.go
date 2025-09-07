package outbound

import (
	"codechunking/internal/adapter/outbound/treesitter/memory"
	"codechunking/internal/domain/valueobject"
	"context"
	"time"
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
	ChunkingStrategy   ChunkingStrategyType
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
	ChunksGenerated []EnhancedCodeChunk
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

// StreamingCodeProcessor defines the interface for streaming code processing.
type StreamingCodeProcessor interface {
	ProcessFile(ctx context.Context, config *ProcessingConfig) (*ProcessingResult, error)
	ProcessDirectory(ctx context.Context, config *DirectoryProcessingConfig) (*DirectoryProcessingResult, error)
	GetMetrics(ctx context.Context) (*ProcessingMetrics, error)
	Close() error
}
