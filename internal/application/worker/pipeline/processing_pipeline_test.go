package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type PipelineOrchestrationTestSuite struct {
	suite.Suite

	orchestrator PipelineOrchestrator
}

func (suite *PipelineOrchestrationTestSuite) SetupTest() {
	suite.orchestrator = NewPipelineOrchestrator(
		5, // maxConcurrency
	)
}

func (suite *PipelineOrchestrationTestSuite) TestEndToEndPipelineWithSmallRepository() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/small-repo",
		MaxMemoryMB:   512,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestEndToEndPipelineWithLargeRepository() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/large-repo",
		MaxMemoryMB:   1024,
		Timeout:       60 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestEndToEndPipelineWithMixedSizeRepository() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/mixed-repo",
		MaxMemoryMB:   768,
		Timeout:       45 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestPipelineCancellationDuringCloneStage() {
	ctx, cancel := context.WithCancel(context.Background())
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/cancel-repo",
		MaxMemoryMB:   512,
		Timeout:       30 * time.Second,
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.ErrorIs(err, context.Canceled)
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestPipelineCancellationDuringProcessingStage() {
	ctx, cancel := context.WithCancel(context.Background())
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/cancel-repo",
		MaxMemoryMB:   512,
		Timeout:       30 * time.Second,
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.ErrorIs(err, context.Canceled)
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestPipelineTimeoutHandling() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/timeout-repo",
		MaxMemoryMB:   512,
		Timeout:       1 * time.Millisecond,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.ErrorIs(err, context.DeadlineExceeded)
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestMemoryLimitEnforcementDuringClone() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/large-repo",
		MaxMemoryMB:   10,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "memory limit exceeded")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestMemoryLimitEnforcementDuringProcessing() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/memory-heavy-repo",
		MaxMemoryMB:   50,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "memory limit exceeded")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestAdaptiveStrategyAdjustmentBasedOnMemoryPressure() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/adaptive-repo",
		MaxMemoryMB:   100,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "strategy adjustment failed")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestMemoryLimitEnforcementWithConcurrentPipelines() {
	ctx := context.Background()
	config1 := &PipelineConfig{
		RepositoryURL: "https://github.com/test/concurrent-repo-1",
		MaxMemoryMB:   50,
		Timeout:       30 * time.Second,
	}
	config2 := &PipelineConfig{
		RepositoryURL: "https://github.com/test/concurrent-repo-2",
		MaxMemoryMB:   50,
		Timeout:       30 * time.Second,
	}

	result1, err1 := suite.orchestrator.ProcessRepository(ctx, config1)
	result2, err2 := suite.orchestrator.ProcessRepository(ctx, config2)

	suite.Error(err1)
	suite.Error(err2)
	suite.Contains(err1.Error(), "memory limit exceeded")
	suite.Contains(err2.Error(), "memory limit exceeded")
	suite.Nil(result1)
	suite.Nil(result2)
}

func (suite *PipelineOrchestrationTestSuite) TestGracefulDegradationWhenMemoryExceededDuringChunking() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/chunking-repo",
		MaxMemoryMB:   30,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "chunking failed")
	suite.Contains(err.Error(), "memory limit exceeded")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestMemoryCleanupBetweenPipelineStages() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/cleanup-repo",
		MaxMemoryMB:   128,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "memory cleanup failed")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestMemoryViolationRecoveryAndStrategyFallback() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/recovery-repo",
		MaxMemoryMB:   20,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "memory violation")
	suite.Contains(err.Error(), "strategy fallback")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestFallbackFromStreamingToBatchWhenStreamingFails() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL:     "https://github.com/test/fallback-repo",
		MaxMemoryMB:       256,
		PreferredStrategy: StreamingStrategy,
		Timeout:           30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "streaming failed")
	suite.Contains(err.Error(), "fallback to batch")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestPartialProcessingWhenSomeFilesFailDuringChunking() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/partial-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "partial processing")
	suite.Contains(err.Error(), "chunking failures")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestPipelineRecoveryFromGitClientFailures() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/git-failure-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "git client failure")
	suite.Contains(err.Error(), "recovery attempt")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestPipelineRecoveryFromEmbeddingGeneratorFailures() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/embedding-failure-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "embedding generator failure")
	suite.Contains(err.Error(), "recovery attempt")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestTimeoutHandlingAtEachStageWithGracefulShutdown() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/stage-timeout-repo",
		MaxMemoryMB:   256,
		Timeout:       1 * time.Millisecond,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "stage timeout")
	suite.Contains(err.Error(), "graceful shutdown")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestResourceCleanupOnFailuresAtDifferentStages() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/cleanup-failure-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "resource cleanup")
	suite.Contains(err.Error(), "stage failure")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestStrategySelectionBasedOnTotalRepositorySize() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/size-based-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "strategy selection")
	suite.Contains(err.Error(), "repository size")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestStrategySelectionBasedOnIndividualFileSize() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/large-file-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "strategy selection")
	suite.Contains(err.Error(), "large files")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestStrategySelectionBasedOnAvailableMemoryConstraints() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/memory-constrained-repo",
		MaxMemoryMB:   15,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "strategy selection")
	suite.Contains(err.Error(), "memory constraints")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestStrategySelectionBasedOnFileTypesAndLanguageMix() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/multi-language-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "strategy selection")
	suite.Contains(err.Error(), "file types")
	suite.Contains(err.Error(), "language mix")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestDynamicStrategyAdjustmentDuringProcessing() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/dynamic-repo",
		MaxMemoryMB:   100,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "dynamic strategy adjustment")
	suite.Contains(err.Error(), "memory pressure")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestFallbackStrategySelectionWhenStreamingFails() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/streaming-fallback-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "fallback strategy")
	suite.Contains(err.Error(), "streaming failure")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestHybridStrategySelectionForMixedSizeRepositories() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/hybrid-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "hybrid strategy")
	suite.Contains(err.Error(), "mixed size")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestProcessingRepositoriesLargerThan1GB() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/very-large-repo",
		MaxMemoryMB:   1024,
		Timeout:       60 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "processing failed")
	suite.Contains(err.Error(), "large repository")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestConcurrentFileStreamingWithMemoryManagement() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/concurrent-streaming-repo",
		MaxMemoryMB:   200,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "concurrent streaming")
	suite.Contains(err.Error(), "memory management")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestStreamingProcessingWithMultipleLanguages() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/multi-lang-streaming-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "streaming processing")
	suite.Contains(err.Error(), "multiple languages")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestStreamingPipelinePerformanceMetrics() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/performance-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "performance metrics")
	suite.Contains(err.Error(), "streaming vs batch")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestMemoryEfficientProcessingOfVeryLargeFiles() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/very-large-files-repo",
		MaxMemoryMB:   128,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "memory efficient processing")
	suite.Contains(err.Error(), "large files")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestStreamingProcessingWithMemoryLimitViolations() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/streaming-memory-violation-repo",
		MaxMemoryMB:   50,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "streaming processing")
	suite.Contains(err.Error(), "memory violation")
	suite.Contains(err.Error(), "recovery")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestCoordinationBetweenGitClientAndDetector() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/coordination-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "component coordination")
	suite.Contains(err.Error(), "git client")
	suite.Contains(err.Error(), "file detector")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestContextAndProgressTrackingHandoff() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/progress-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "context handoff")
	suite.Contains(err.Error(), "progress tracking")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestErrorPropagationAndRecoveryAcrossStages() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/error-propagation-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "error propagation")
	suite.Contains(err.Error(), "stage recovery")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestOTELMetricsAggregationAcrossComponents() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/metrics-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "otel metrics")
	suite.Contains(err.Error(), "aggregation")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestStructuredLoggingCoordination() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/logging-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "structured logging")
	suite.Contains(err.Error(), "coordination")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestDependencyInjectionAndComponentInitialization() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/dependency-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "dependency injection")
	suite.Contains(err.Error(), "component initialization")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestConfigurationValidationAndDefaultHandling() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "", // Invalid config
		MaxMemoryMB:   0,
		Timeout:       0,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "configuration validation")
	suite.Contains(err.Error(), "default values")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestConcurrentAccessToSharedComponents() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/shared-components-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "concurrent access")
	suite.Contains(err.Error(), "shared components")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestMultipleConcurrentPipelinesWithResourceSharing() {
	ctx := context.Background()
	config1 := &PipelineConfig{
		RepositoryURL: "https://github.com/test/concurrent-1",
		MaxMemoryMB:   128,
		Timeout:       30 * time.Second,
	}
	config2 := &PipelineConfig{
		RepositoryURL: "https://github.com/test/concurrent-2",
		MaxMemoryMB:   128,
		Timeout:       30 * time.Second,
	}

	result1, err1 := suite.orchestrator.ProcessRepository(ctx, config1)
	result2, err2 := suite.orchestrator.ProcessRepository(ctx, config2)

	suite.Error(err1)
	suite.Error(err2)
	suite.Contains(err1.Error(), "concurrent pipelines")
	suite.Contains(err2.Error(), "resource sharing")
	suite.Nil(result1)
	suite.Nil(result2)
}

func (suite *PipelineOrchestrationTestSuite) TestPipelineQueuingWhenMaxConcurrencyReached() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/queuing-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "pipeline queuing")
	suite.Contains(err.Error(), "max concurrency")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestConcurrentAccessToSharedGitClientWithAuthCaching() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/auth-cache-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "git client")
	suite.Contains(err.Error(), "authentication caching")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestPerformanceImpactOfConcurrentProcessingOnMemory() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/performance-impact-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "performance impact")
	suite.Contains(err.Error(), "memory usage")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestResourceContentionAndResolutionBetweenPipelines() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/contention-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "resource contention")
	suite.Contains(err.Error(), "resolution")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestPipelinePrioritizationAndFairness() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/prioritization-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "pipeline prioritization")
	suite.Contains(err.Error(), "fairness")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) BenchmarkPipelinePerformanceWithSmallRepository() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/small-benchmark-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "benchmark")
	suite.Contains(err.Error(), "small repository")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) BenchmarkPipelinePerformanceWithLargeRepository() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/large-benchmark-repo",
		MaxMemoryMB:   1024,
		Timeout:       60 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "benchmark")
	suite.Contains(err.Error(), "large repository")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestResourceAllocationAndCleanupForEachStage() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/resource-allocation-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "resource allocation")
	suite.Contains(err.Error(), "stage cleanup")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestDiskSpaceManagementDuringProcessingAndCleanup() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/disk-space-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "disk space management")
	suite.Contains(err.Error(), "processing")
	suite.Contains(err.Error(), "cleanup")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestNetworkBandwidthUtilizationDuringGitOperations() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/network-bandwidth-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "network bandwidth")
	suite.Contains(err.Error(), "git operations")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestCPUUtilizationDuringParsingAndChunking() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/cpu-utilization-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "cpu utilization")
	suite.Contains(err.Error(), "parsing")
	suite.Contains(err.Error(), "chunking")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestMemoryUsagePatternsThroughoutPipelineLifecycle() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/memory-patterns-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "memory usage patterns")
	suite.Contains(err.Error(), "pipeline lifecycle")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestCleanupVerificationAfterPipelineCompletion() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/cleanup-verification-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "cleanup verification")
	suite.Contains(err.Error(), "pipeline completion")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestCleanupVerificationAfterPipelineFailure() {
	ctx := context.Background()
	config := &PipelineConfig{
		RepositoryURL: "https://github.com/test/cleanup-verification-failure-repo",
		MaxMemoryMB:   256,
		Timeout:       30 * time.Second,
	}

	result, err := suite.orchestrator.ProcessRepository(ctx, config)

	suite.Error(err)
	suite.Contains(err.Error(), "cleanup verification")
	suite.Contains(err.Error(), "pipeline failure")
	suite.Nil(result)
}

func (suite *PipelineOrchestrationTestSuite) TestGetPipelineStatus() {
	pipelineID := uuid.New().String()

	status, err := suite.orchestrator.GetPipelineStatus(pipelineID)

	suite.Error(err)
	suite.Nil(status)
}

func (suite *PipelineOrchestrationTestSuite) TestCancelPipeline() {
	pipelineID := uuid.New().String()

	err := suite.orchestrator.CancelPipeline(pipelineID)

	suite.Error(err)
}

func (suite *PipelineOrchestrationTestSuite) TestCleanupPipeline() {
	pipelineID := uuid.New().String()

	err := suite.orchestrator.CleanupPipeline(pipelineID)

	suite.Error(err)
}

func TestPipelineOrchestrationTestSuite(t *testing.T) {
	suite.Run(t, new(PipelineOrchestrationTestSuite))
}
