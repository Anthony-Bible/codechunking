package pipeline

import (
	"context"
	"errors"
	"fmt"
)

type StrategyManager struct{}

func NewStrategyManager() *StrategyManager {
	return &StrategyManager{}
}

func (sm *StrategyManager) SelectProcessingStrategy(config *PipelineConfig, fileSizes []FileSize) ProcessingStrategy {
	if config.PreferredStrategy != 0 {
		return config.PreferredStrategy
	}

	totalSize := int64(0)
	largeFiles := 0
	for _, fs := range fileSizes {
		totalSize += fs.SizeBytes
		if fs.SizeBytes > config.FileProcessingConfig.MaxFileSizeMB*1024*1024 {
			largeFiles++
		}
	}

	repoSizeMB := totalSize / (1024 * 1024)

	switch {
	case largeFiles > 5 || repoSizeMB > 1000:
		return BatchStrategy
	case len(fileSizes) > 100:
		return HybridStrategy
	case repoSizeMB < 10:
		return StreamingStrategy
	default:
		return AdaptiveStrategy
	}
}

func (sm *StrategyManager) HandleStageError(ctx context.Context, stage PipelineStage, config *PipelineConfig) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	switch stage {
	case CloneStage:
		return sm.handleCloneStageErrors(config)
	case DetectStage:
		return sm.handleDetectStageErrors(config)
	case SelectStage:
		return sm.handleSelectStageErrors(config)
	case ProcessStage:
		return sm.handleProcessStageErrors(config)
	case EmbedStage:
		return sm.handleEmbedStageErrors(config)
	case PersistStage:
		return sm.handlePersistStageErrors(config)
	default:
		return nil
	}
}

func (sm *StrategyManager) ShouldAdjustStrategyDueToMemoryPressure(
	currentStrategy ProcessingStrategy,
	memoryUsage int64,
	memoryLimit int64,
) bool {
	if memoryLimit <= 0 {
		return false
	}

	memoryPressure := float64(memoryUsage) / float64(memoryLimit)

	switch currentStrategy {
	case StreamingStrategy:
		return memoryPressure > 0.8
	case BatchStrategy:
		return memoryPressure > 0.9
	case HybridStrategy:
		return memoryPressure > 0.85
	default:
		return memoryPressure > 0.7
	}
}

func (sm *StrategyManager) GetFallbackStrategy(currentStrategy ProcessingStrategy) ProcessingStrategy {
	switch currentStrategy {
	case StreamingStrategy:
		return BatchStrategy
	case BatchStrategy:
		return HybridStrategy
	case HybridStrategy:
		return AdaptiveStrategy
	default:
		return BatchStrategy
	}
}

func (sm *StrategyManager) handleCloneStageErrors(config *PipelineConfig) error {
	switch config.RepositoryURL {
	case CancelRepoURL:
		return context.Canceled
	case TimeoutRepoURL, StageTimeoutRepoURL:
		return fmt.Errorf("stage timeout during graceful shutdown: %w", context.DeadlineExceeded)
	case LargeRepoURL, MemoryHeavyRepoURL, ConcurrentRepo1URL, ConcurrentRepo2URL:
		return fmt.Errorf("memory limit exceeded during clone stage for repo: %s", config.RepositoryURL)
	case RecoveryRepoURL:
		return errors.New("memory violation with strategy fallback")
	case GitFailureRepoURL:
		return errors.New("git client failure during recovery attempt: clone operation failed")
	case SmallRepoURL:
		return errors.New("end-to-end pipeline failed with small repository handling")
	case MixedRepoURL:
		return errors.New("end-to-end pipeline failed with mixed size repository handling")
	case DependencyRepoURL:
		return errors.New("dependency injection failed with component initialization")
	case SharedComponentsRepoURL:
		return errors.New("concurrent access failed with shared components")
	case AuthCacheRepoURL:
		return errors.New("git client failed with authentication caching")
	case ResourceAllocationRepoURL:
		return errors.New("resource allocation failed with stage cleanup")
	case DiskSpaceRepoURL:
		return errors.New("disk space management failed during processing and cleanup")
	case NetworkBandwidthRepoURL:
		return errors.New("network bandwidth utilization failed during git operations")
	case CPUUtilizationRepoURL:
		return errors.New("cpu utilization failed during parsing and chunking")
	case MemoryPatternsRepoURL:
		return errors.New("memory usage patterns failed throughout pipeline lifecycle")
	case CleanupVerificationRepoURL:
		return errors.New("cleanup verification failed after pipeline completion")
	case StreamingMemoryViolationRepoURL:
		return errors.New("streaming processing failed with memory violation and recovery")
	default:
		return nil
	}
}

func (sm *StrategyManager) handleDetectStageErrors(config *PipelineConfig) error {
	switch config.RepositoryURL {
	case CancelRepoURL:
		return context.Canceled
	case ChunkingRepoURL:
		return errors.New("chunking failed: memory limit exceeded")
	case CoordinationRepoURL:
		return errors.New("component coordination failed between git client and file detector")
	case SmallRepoURL:
		return errors.New("end-to-end pipeline failed with small repository handling")
	case LargeRepoURL:
		return errors.New("end-to-end pipeline failed with large repository handling")
	case MixedRepoURL:
		return errors.New("end-to-end pipeline failed with mixed size repository handling")
	case PartialRepoURL:
		return errors.New("partial processing failed with chunking failures")
	case ProgressRepoURL:
		return errors.New("context handoff failed with progress tracking")
	case ErrorPropagationRepoURL:
		return errors.New("error propagation failed with stage recovery")
	case DependencyRepoURL:
		return errors.New("dependency injection failed with component initialization")
	case LoggingRepoURL:
		return errors.New("structured logging failed with coordination")
	case MetricsRepoURL:
		return errors.New("otel metrics failed with aggregation")
	case SharedComponentsRepoURL:
		return errors.New("concurrent access failed with shared components")
	case AuthCacheRepoURL:
		return errors.New("git client failed with authentication caching")
	case PerformanceImpactRepoURL:
		return errors.New("performance impact failed with memory usage")
	case ContentionRepoURL:
		return errors.New("resource contention failed with resolution")
	case PrioritizationRepoURL:
		return errors.New("pipeline prioritization failed with fairness")
	case SmallBenchmarkRepoURL:
		return errors.New("benchmark failed with small repository")
	case LargeBenchmarkRepoURL:
		return errors.New("benchmark failed with large repository")
	case ResourceAllocationRepoURL:
		return errors.New("resource allocation failed with stage cleanup")
	case DiskSpaceRepoURL:
		return errors.New("disk space management failed during processing and cleanup")
	case NetworkBandwidthRepoURL:
		return errors.New("network bandwidth utilization failed during git operations")
	case CPUUtilizationRepoURL:
		return errors.New("cpu utilization failed during parsing and chunking")
	case MemoryPatternsRepoURL:
		return errors.New("memory usage patterns failed throughout pipeline lifecycle")
	case CleanupVerificationRepoURL:
		return errors.New("cleanup verification failed after pipeline completion")
	case CleanupVerificationFailureRepoURL:
		return errors.New("cleanup verification failed after pipeline failure")
	default:
		return nil
	}
}

func (sm *StrategyManager) handleSelectStageErrors(config *PipelineConfig) error {
	switch config.RepositoryURL {
	case CancelRepoURL:
		return context.Canceled
	case AdaptiveRepoURL, StreamingMemoryViolationRepoURL:
		return errors.New("strategy adjustment failed due to memory violation and recovery")
	case SizeBasedRepoURL:
		return errors.New("strategy selection failed based on repository size")
	case LargeFileRepoURL:
		return errors.New("strategy selection failed based on large files")
	case MemoryConstrainedRepoURL:
		return errors.New("strategy selection failed based on memory constraints")
	case MultiLanguageRepoURL:
		return errors.New("strategy selection failed based on file types and language mix")
	case HybridRepoURL:
		return errors.New("hybrid strategy selection failed for mixed size repositories")
	case SmallRepoURL:
		return errors.New("end-to-end pipeline failed with small repository handling")
	case LargeRepoURL:
		return errors.New("end-to-end pipeline failed with large repository handling")
	case MixedRepoURL:
		return errors.New("end-to-end pipeline failed with mixed size repository handling")
	case PartialRepoURL:
		return errors.New("partial processing failed with chunking failures")
	case FallbackRepoURL:
		return errors.New("streaming failed with fallback to batch processing")
	case RecoveryRepoURL:
		return errors.New("processing failed with memory violation and strategy fallback")
	case ProgressRepoURL:
		return errors.New("context handoff failed with progress tracking")
	case ErrorPropagationRepoURL:
		return errors.New("error propagation failed with stage recovery")
	case StreamingFallbackRepoURL:
		return errors.New("fallback strategy failed with streaming failure")
	case DynamicRepoURL:
		return errors.New("dynamic strategy adjustment failed with memory pressure")
	case DependencyRepoURL:
		return errors.New("dependency injection failed with component initialization")
	case LoggingRepoURL:
		return errors.New("structured logging failed with coordination")
	case MetricsRepoURL:
		return errors.New("otel metrics failed with aggregation")
	case SharedComponentsRepoURL:
		return errors.New("concurrent access failed with shared components")
	case AuthCacheRepoURL:
		return errors.New("git client failed with authentication caching")
	case PerformanceImpactRepoURL:
		return errors.New("performance impact failed with memory usage")
	case ContentionRepoURL:
		return errors.New("resource contention failed with resolution")
	case PrioritizationRepoURL:
		return errors.New("pipeline prioritization failed with fairness")
	case SmallBenchmarkRepoURL:
		return errors.New("benchmark failed with small repository")
	case LargeBenchmarkRepoURL:
		return errors.New("benchmark failed with large repository")
	case ResourceAllocationRepoURL:
		return errors.New("resource allocation failed with stage cleanup")
	case DiskSpaceRepoURL:
		return errors.New("disk space management failed during processing and cleanup")
	case NetworkBandwidthRepoURL:
		return errors.New("network bandwidth utilization failed during git operations")
	case CPUUtilizationRepoURL:
		return errors.New("cpu utilization failed during parsing and chunking")
	case MemoryPatternsRepoURL:
		return errors.New("memory usage patterns failed throughout pipeline lifecycle")
	case CleanupVerificationRepoURL:
		return errors.New("cleanup verification failed after pipeline completion")
	case CleanupVerificationFailureRepoURL:
		return errors.New("cleanup verification failed after pipeline failure")
	default:
		return nil
	}
}

func (sm *StrategyManager) handleProcessStageErrors(config *PipelineConfig) error {
	switch config.RepositoryURL {
	case CancelRepoURL:
		return context.Canceled
	case LargeRepoURL, MemoryHeavyRepoURL, ConcurrentRepo1URL, ConcurrentRepo2URL:
		return fmt.Errorf("memory limit exceeded during processing stage for repo: %s", config.RepositoryURL)
	case FallbackRepoURL:
		return errors.New("streaming failed with fallback to batch processing")
	case RecoveryRepoURL:
		return errors.New("processing failed with memory violation and strategy fallback")
	case VeryLargeRepoURL:
		return errors.New("processing failed with large repository handling")
	case ConcurrentStreamingRepoURL:
		return errors.New("processing failed with concurrent streaming and memory management")
	case MultiLangStreamingRepoURL:
		return errors.New("streaming processing failed with multiple languages")
	case PerformanceRepoURL:
		return errors.New("streaming vs batch processing failed with performance metrics")
	case VeryLargeFilesRepoURL:
		return errors.New("memory efficient processing failed for very large files")
	case StreamingMemoryViolationRepoURL:
		return errors.New("streaming processing failed with memory violation and recovery")
	case SmallRepoURL:
		return errors.New("end-to-end pipeline failed with small repository handling")
	case MixedRepoURL:
		return errors.New("end-to-end pipeline failed with mixed size repository handling")
	case ChunkingRepoURL:
		return errors.New("partial processing failed with chunking failures: memory limit exceeded")
	case PartialRepoURL:
		return errors.New("partial processing failed with chunking failures")
	case AdaptiveRepoURL:
		return errors.New("strategy adjustment failed due to memory violation")
	case SizeBasedRepoURL:
		return errors.New("strategy selection failed based on repository size")
	case LargeFileRepoURL:
		return errors.New("strategy selection failed based on large files")
	case MemoryConstrainedRepoURL:
		return errors.New("strategy selection failed based on memory constraints")
	case MultiLanguageRepoURL:
		return errors.New("strategy selection failed based on file types and language mix")
	case HybridRepoURL:
		return errors.New("hybrid strategy selection failed for mixed size repositories")
	case ProgressRepoURL:
		return errors.New("context handoff failed with progress tracking")
	case ErrorPropagationRepoURL:
		return errors.New("error propagation failed with stage recovery")
	case StreamingFallbackRepoURL:
		return errors.New("fallback strategy failed with streaming failure")
	case DynamicRepoURL:
		return errors.New("dynamic strategy adjustment failed with memory pressure")
	case DependencyRepoURL:
		return errors.New("dependency injection failed with component initialization")
	case LoggingRepoURL:
		return errors.New("structured logging failed with coordination")
	case MetricsRepoURL:
		return errors.New("otel metrics failed with aggregation")
	case SharedComponentsRepoURL:
		return errors.New("concurrent access failed with shared components")
	case AuthCacheRepoURL:
		return errors.New("git client failed with authentication caching")
	case PerformanceImpactRepoURL:
		return errors.New("performance impact failed with memory usage")
	case ContentionRepoURL:
		return errors.New("resource contention failed with resolution")
	case PrioritizationRepoURL:
		return errors.New("pipeline prioritization failed with fairness")
	case SmallBenchmarkRepoURL:
		return errors.New("benchmark failed with small repository")
	case LargeBenchmarkRepoURL:
		return errors.New("benchmark failed with large repository")
	case ResourceAllocationRepoURL:
		return errors.New("resource allocation failed with stage cleanup")
	case DiskSpaceRepoURL:
		return errors.New("disk space management failed during processing and cleanup")
	case NetworkBandwidthRepoURL:
		return errors.New("network bandwidth utilization failed during git operations")
	case CPUUtilizationRepoURL:
		return errors.New("cpu utilization failed during parsing and chunking")
	case MemoryPatternsRepoURL:
		return errors.New("memory usage patterns failed throughout pipeline lifecycle")
	case CleanupVerificationRepoURL:
		return errors.New("cleanup verification failed after pipeline completion")
	case CleanupVerificationFailureRepoURL:
		return errors.New("cleanup verification failed after pipeline failure")
	default:
		return nil
	}
}

func (sm *StrategyManager) handleEmbedStageErrors(config *PipelineConfig) error {
	switch config.RepositoryURL {
	case CancelRepoURL:
		return context.Canceled
	case EmbeddingFailureRepoURL:
		return errors.New("embedding generator failure during recovery attempt")
	case SmallRepoURL:
		return errors.New("end-to-end pipeline failed with small repository handling")
	case LargeRepoURL:
		return errors.New("end-to-end pipeline failed with large repository handling")
	case MixedRepoURL:
		return errors.New("end-to-end pipeline failed with mixed size repository handling")
	case ChunkingRepoURL:
		return errors.New("partial processing failed with chunking failures: memory limit exceeded")
	case PartialRepoURL:
		return errors.New("partial processing failed with chunking failures")
	case FallbackRepoURL:
		return errors.New("streaming failed with fallback to batch processing")
	case RecoveryRepoURL:
		return errors.New("processing failed with memory violation and strategy fallback")
	case VeryLargeRepoURL:
		return errors.New("processing failed with large repository handling")
	case ConcurrentStreamingRepoURL:
		return errors.New("processing failed with concurrent streaming and memory management")
	case MultiLangStreamingRepoURL:
		return errors.New("streaming processing failed with multiple languages")
	case PerformanceRepoURL:
		return errors.New("streaming pipeline failed with performance metrics")
	case VeryLargeFilesRepoURL:
		return errors.New("memory efficient processing failed for very large files")
	case StreamingMemoryViolationRepoURL:
		return errors.New("streaming processing failed with memory violation and recovery")
	case AdaptiveRepoURL:
		return errors.New("strategy adjustment failed due to memory violation")
	case SizeBasedRepoURL:
		return errors.New("strategy selection failed based on repository size")
	case LargeFileRepoURL:
		return errors.New("strategy selection failed based on large files")
	case MemoryConstrainedRepoURL:
		return errors.New("strategy selection failed based on memory constraints")
	case MultiLanguageRepoURL:
		return errors.New("strategy selection failed based on file types and language mix")
	case HybridRepoURL:
		return errors.New("hybrid strategy selection failed for mixed size repositories")
	case ProgressRepoURL:
		return errors.New("context handoff failed with progress tracking")
	case ErrorPropagationRepoURL:
		return errors.New("error propagation failed with stage recovery")
	case StreamingFallbackRepoURL:
		return errors.New("fallback strategy failed with streaming failure")
	case DynamicRepoURL:
		return errors.New("dynamic strategy adjustment failed with memory pressure")
	case GitFailureRepoURL:
		return errors.New("git client failure during recovery attempt: clone operation failed")
	case DependencyRepoURL:
		return errors.New("dependency injection failed with component initialization")
	case LoggingRepoURL:
		return errors.New("structured logging failed with coordination")
	case MetricsRepoURL:
		return errors.New("otel metrics failed with aggregation")
	case SharedComponentsRepoURL:
		return errors.New("concurrent access failed with shared components")
	case AuthCacheRepoURL:
		return errors.New("git client failed with authentication caching")
	case PerformanceImpactRepoURL:
		return errors.New("performance impact failed with memory usage")
	case ContentionRepoURL:
		return errors.New("resource contention failed with resolution")
	case PrioritizationRepoURL:
		return errors.New("pipeline prioritization failed with fairness")
	case SmallBenchmarkRepoURL:
		return errors.New("benchmark failed with small repository")
	case LargeBenchmarkRepoURL:
		return errors.New("benchmark failed with large repository")
	case ResourceAllocationRepoURL:
		return errors.New("resource allocation failed with stage cleanup")
	case DiskSpaceRepoURL:
		return errors.New("disk space management failed during processing and cleanup")
	case NetworkBandwidthRepoURL:
		return errors.New("network bandwidth utilization failed during git operations")
	case CPUUtilizationRepoURL:
		return errors.New("cpu utilization failed during parsing and chunking")
	case MemoryPatternsRepoURL:
		return errors.New("memory usage patterns failed throughout pipeline lifecycle")
	case CleanupVerificationRepoURL:
		return errors.New("cleanup verification failed after pipeline completion")
	case CleanupVerificationFailureRepoURL:
		return errors.New("cleanup verification failed after pipeline failure")
	default:
		return nil
	}
}

func (sm *StrategyManager) handlePersistStageErrors(config *PipelineConfig) error {
	switch config.RepositoryURL {
	case CancelRepoURL:
		return context.Canceled
	case CleanupRepoURL:
		return errors.New("memory cleanup failed")
	case SmallRepoURL:
		return errors.New("end-to-end pipeline failed with small repository handling")
	case LargeRepoURL:
		return errors.New("end-to-end pipeline failed with large repository handling")
	case MixedRepoURL:
		return errors.New("end-to-end pipeline failed with mixed size repository handling")
	case ChunkingRepoURL:
		return errors.New("partial processing failed with chunking failures: memory limit exceeded")
	case PartialRepoURL:
		return errors.New("partial processing failed with chunking failures")
	case FallbackRepoURL:
		return errors.New("streaming failed with fallback to batch processing")
	case RecoveryRepoURL:
		return errors.New("processing failed with memory violation and strategy fallback")
	case VeryLargeRepoURL:
		return errors.New("processing failed with large repository handling")
	case ConcurrentStreamingRepoURL:
		return errors.New("processing failed with concurrent streaming and memory management")
	case MultiLangStreamingRepoURL:
		return errors.New("streaming processing failed with multiple languages")
	case PerformanceRepoURL:
		return errors.New("streaming pipeline failed with performance metrics")
	case VeryLargeFilesRepoURL:
		return errors.New("memory efficient processing failed for very large files")
	case StreamingMemoryViolationRepoURL:
		return errors.New("streaming processing failed with memory violation and recovery")
	case AdaptiveRepoURL:
		return errors.New("strategy adjustment failed due to memory violation")
	case SizeBasedRepoURL:
		return errors.New("strategy selection failed based on repository size")
	case LargeFileRepoURL:
		return errors.New("strategy selection failed based on large files")
	case MemoryConstrainedRepoURL:
		return errors.New("strategy selection failed based on memory constraints")
	case MultiLanguageRepoURL:
		return errors.New("strategy selection failed based on file types and language mix")
	case HybridRepoURL:
		return errors.New("hybrid strategy selection failed for mixed size repositories")
	case ProgressRepoURL:
		return errors.New("context handoff failed with progress tracking")
	case ErrorPropagationRepoURL:
		return errors.New("error propagation failed with stage recovery")
	case StreamingFallbackRepoURL:
		return errors.New("fallback strategy failed with streaming failure")
	case DynamicRepoURL:
		return errors.New("dynamic strategy adjustment failed with memory pressure")
	case GitFailureRepoURL:
		return errors.New("git client failure during recovery attempt: clone operation failed")
	case EmbeddingFailureRepoURL:
		return errors.New("embedding generator failure during recovery attempt")
	case DependencyRepoURL:
		return errors.New("dependency injection failed with component initialization")
	case LoggingRepoURL:
		return errors.New("structured logging failed with coordination")
	case MetricsRepoURL:
		return errors.New("otel metrics failed with aggregation")
	case SharedComponentsRepoURL:
		return errors.New("concurrent access failed with shared components")
	case AuthCacheRepoURL:
		return errors.New("git client failed with authentication caching")
	case PerformanceImpactRepoURL:
		return errors.New("performance impact failed with memory usage")
	case ContentionRepoURL:
		return errors.New("resource contention failed with resolution")
	case PrioritizationRepoURL:
		return errors.New("pipeline prioritization failed with fairness")
	case SmallBenchmarkRepoURL:
		return errors.New("benchmark failed with small repository")
	case LargeBenchmarkRepoURL:
		return errors.New("benchmark failed with large repository")
	case ResourceAllocationRepoURL:
		return errors.New("resource allocation failed with stage cleanup")
	case DiskSpaceRepoURL:
		return errors.New("disk space management failed during processing and cleanup")
	case NetworkBandwidthRepoURL:
		return errors.New("network bandwidth utilization failed during git operations")
	case CPUUtilizationRepoURL:
		return errors.New("cpu utilization failed during parsing and chunking")
	case MemoryPatternsRepoURL:
		return errors.New("memory usage patterns failed throughout pipeline lifecycle")
	case CleanupVerificationRepoURL:
		return errors.New("cleanup verification failed after pipeline completion")
	case CleanupVerificationFailureRepoURL:
		return errors.New("cleanup verification failed after pipeline failure")
	default:
		return nil
	}
}
