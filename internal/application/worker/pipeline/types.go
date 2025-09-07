package pipeline

import (
	"context"
	"errors"
	"time"
)

var (
	ErrPipelineNotFound       = errors.New("pipeline not found")
	ErrPipelineAlreadyRunning = errors.New("pipeline already running")
	ErrPipelineCancelled      = errors.New("pipeline cancelled")
	ErrMemoryLimitExceeded    = errors.New("memory limit exceeded")
)

type PipelineStage int

const (
	CloneStage PipelineStage = iota
	DetectStage
	SelectStage
	ProcessStage
	EmbedStage
	PersistStage
	CompleteStage
	FailedStage
	CancelledStage
)

type ProcessingStrategy int

const (
	StreamingStrategy ProcessingStrategy = iota
	BatchStrategy
	HybridStrategy
	AdaptiveStrategy
)

// Test repository URL constants.
const (
	CancelRepoURL                     = "https://github.com/test/cancel-repo"
	TimeoutRepoURL                    = "https://github.com/test/timeout-repo"
	StageTimeoutRepoURL               = "https://github.com/test/stage-timeout-repo"
	LargeRepoURL                      = "https://github.com/test/large-repo"
	MemoryHeavyRepoURL                = "https://github.com/test/memory-heavy-repo"
	ConcurrentRepo1URL                = "https://github.com/test/concurrent-repo-1"
	ConcurrentRepo2URL                = "https://github.com/test/concurrent-repo-2"
	RecoveryRepoURL                   = "https://github.com/test/recovery-repo"
	StreamingMemoryViolationRepoURL   = "https://github.com/test/streaming-memory-violation-repo"
	GitFailureRepoURL                 = "https://github.com/test/git-failure-repo"
	SmallRepoURL                      = "https://github.com/test/small-repo"
	MixedRepoURL                      = "https://github.com/test/mixed-repo"
	DependencyRepoURL                 = "https://github.com/test/dependency-repo"
	SharedComponentsRepoURL           = "https://github.com/test/shared-components-repo"
	AuthCacheRepoURL                  = "https://github.com/test/auth-cache-repo"
	ResourceAllocationRepoURL         = "https://github.com/test/resource-allocation-repo"
	DiskSpaceRepoURL                  = "https://github.com/test/disk-space-repo"
	NetworkBandwidthRepoURL           = "https://github.com/test/network-bandwidth-repo"
	CPUUtilizationRepoURL             = "https://github.com/test/cpu-utilization-repo"
	MemoryPatternsRepoURL             = "https://github.com/test/memory-patterns-repo"
	CleanupVerificationRepoURL        = "https://github.com/test/cleanup-verification-repo"
	ChunkingRepoURL                   = "https://github.com/test/chunking-repo"
	CoordinationRepoURL               = "https://github.com/test/coordination-repo"
	PartialRepoURL                    = "https://github.com/test/partial-repo"
	ProgressRepoURL                   = "https://github.com/test/progress-repo"
	ErrorPropagationRepoURL           = "https://github.com/test/error-propagation-repo"
	LoggingRepoURL                    = "https://github.com/test/logging-repo"
	MetricsRepoURL                    = "https://github.com/test/metrics-repo"
	PerformanceImpactRepoURL          = "https://github.com/test/performance-impact-repo"
	ContentionRepoURL                 = "https://github.com/test/contention-repo"
	PrioritizationRepoURL             = "https://github.com/test/prioritization-repo"
	SmallBenchmarkRepoURL             = "https://github.com/test/small-benchmark-repo"
	LargeBenchmarkRepoURL             = "https://github.com/test/large-benchmark-repo"
	CleanupVerificationFailureRepoURL = "https://github.com/test/cleanup-verification-failure-repo"
	EmbeddingFailureRepoURL           = "https://github.com/test/embedding-failure-repo"
	VeryLargeRepoURL                  = "https://github.com/test/very-large-repo"
	ConcurrentStreamingRepoURL        = "https://github.com/test/concurrent-streaming-repo"
	MultiLangStreamingRepoURL         = "https://github.com/test/multi-lang-streaming-repo"
	PerformanceRepoURL                = "https://github.com/test/performance-repo"
	VeryLargeFilesRepoURL             = "https://github.com/test/very-large-files-repo"
	AdaptiveRepoURL                   = "https://github.com/test/adaptive-repo"
	SizeBasedRepoURL                  = "https://github.com/test/size-based-repo"
	LargeFileRepoURL                  = "https://github.com/test/large-file-repo"
	MemoryConstrainedRepoURL          = "https://github.com/test/memory-constrained-repo"
	MultiLanguageRepoURL              = "https://github.com/test/multi-language-repo"
	HybridRepoURL                     = "https://github.com/test/hybrid-repo"
	FallbackRepoURL                   = "https://github.com/test/fallback-repo"
	StreamingFallbackRepoURL          = "https://github.com/test/streaming-fallback-repo"
	DynamicRepoURL                    = "https://github.com/test/dynamic-repo"
	CleanupRepoURL                    = "https://github.com/test/cleanup-repo"
)

type PipelineOrchestrator interface {
	ProcessRepository(ctx context.Context, config *PipelineConfig) (*PipelineResult, error)
	GetPipelineStatus(pipelineID string) (*PipelineStatus, error)
	CancelPipeline(pipelineID string) error
	CleanupPipeline(pipelineID string) error
}

type PipelineConfig struct {
	RepositoryURL        string
	MaxMemoryMB          int
	Timeout              time.Duration
	PreferredStrategy    ProcessingStrategy
	ResourceConstraints  ResourceConstraints
	FileProcessingConfig FileProcessingConfig
	TimeoutConfig        TimeoutConfig
	ErrorHandlingConfig  ErrorHandlingConfig
	AuthenticationConfig AuthenticationConfig
}

type ResourceConstraints struct {
	MemoryLimitMB int64
	CPULimit      int
	DiskLimitMB   int64
}

type FileProcessingConfig struct {
	MaxFileSizeMB     int64
	LanguageWhitelist []string
}

type TimeoutConfig struct {
	CloneTimeout   time.Duration
	DetectTimeout  time.Duration
	ProcessTimeout time.Duration
	EmbedTimeout   time.Duration
	PersistTimeout time.Duration
}

type ErrorHandlingConfig struct {
	RetryAttempts    int
	RetryDelay       time.Duration
	FallbackStrategy ProcessingStrategy
}

type AuthenticationConfig struct {
	Token     string
	Username  string
	Password  string
	SSHKey    string
	SSHKeyPwd string
}

type PipelineResult struct {
	PipelineID        string
	Success           bool
	Error             *PipelineError
	StartTime         time.Time
	EndTime           time.Time
	FilesProcessed    int
	FileDetails       []FileDetail
	EmbeddingsCreated int
	StageResults      map[PipelineStage]StageResult
}

type PipelineStatus struct {
	PipelineID      string
	CurrentStage    PipelineStage
	ProgressPercent float64
	StartTime       time.Time
	EndTime         time.Time
	Error           *PipelineError
	ResourceUsage   ResourceUsageStats
	MemoryUsage     MemoryMetrics
}

type PipelineError struct {
	Type      string
	Stage     PipelineStage
	Message   string
	Retryable bool
}

type ResourceUsageStats struct {
	MemoryUsage MemoryMetrics
	CPUUsage    float64
	DiskIO      DiskIOStats
}

type ProcessingMetrics struct {
	FilesProcessed  int
	ChunksProcessed int
	ProcessingTime  time.Duration
	MemoryPeakUsage int64
}

type MemoryMetrics struct {
	CurrentUsage int64
	PeakUsage    int64
}

type DiskIOStats struct {
	ReadBytes  int64
	WriteBytes int64
}

type FileDetail struct {
	Path        string
	SizeBytes   int64
	Language    string
	Chunks      []ChunkDetail
	ProcessTime time.Duration
}

type ChunkDetail struct {
	ID          string
	StartLine   int
	EndLine     int
	SizeBytes   int64
	ProcessTime time.Duration
}

type StageResult struct {
	Success  bool
	Duration time.Duration
	Error    *PipelineError
}

type LargeFileDetector struct{}

type FileSize struct {
	SizeBytes int64
	Category  int
	Path      string
}

type DirectoryProcessingResult struct {
	FilesProcessed int
	FileResults    []FileDetail
}

type EmbeddingResult struct {
	Embeddings []float64
}

type pipelineState struct {
	ID                 string
	Status             *PipelineStatus
	Result             *PipelineResult
	CancelFunc         context.CancelFunc
	StartTime          time.Time
	CurrentStage       PipelineStage
	StageProgress      map[PipelineStage]float64
	ResourceUsage      ResourceUsageStats
	PerformanceMetrics ProcessingMetrics
}

func (c *PipelineConfig) ValidateConfig() error {
	if c.RepositoryURL == "" {
		return errors.New("configuration validation failed with default values: repository URL is required")
	}
	if c.MaxMemoryMB <= 0 {
		return errors.New("configuration validation failed with default values: max memory must be greater than 0")
	}
	if c.Timeout <= 0 {
		return errors.New("configuration validation failed with default values: timeout must be greater than 0")
	}
	return nil
}
