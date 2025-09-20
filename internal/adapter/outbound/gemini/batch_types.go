package gemini

import (
	"codechunking/internal/port/outbound"
	"context"
	"time"
)

// BatchAnalyzer interface defines the contract for batch size analysis.
type BatchAnalyzer interface {
	// AnalyzeBatchSizes performs comprehensive batch size analysis
	AnalyzeBatchSizes(ctx context.Context, config *BatchAnalysisConfig) (*BatchAnalysisResult, error)

	// AnalyzeCosts analyzes cost implications of different batch sizes
	AnalyzeCosts(ctx context.Context, config *BatchAnalysisConfig) (*CostMetrics, error)

	// AnalyzeLatency analyzes latency patterns for different batch sizes
	AnalyzeLatency(ctx context.Context, config *BatchAnalysisConfig) (*LatencyMetrics, error)

	// BenchmarkPerformance conducts performance benchmarks for different batch sizes
	BenchmarkPerformance(ctx context.Context, config *BatchAnalysisConfig) (*PerformanceMetrics, error)

	// RecommendOptimalBatchSize recommends optimal batch size based on constraints
	RecommendOptimalBatchSize(ctx context.Context, constraints *OptimizationConstraints) (*BatchRecommendation, error)

	// PredictPerformance predicts performance metrics for a given batch size
	PredictPerformance(ctx context.Context, batchSize int, config *BatchAnalysisConfig) (*PerformancePrediction, error)
}

// BatchAnalysisConfig configures batch size analysis parameters.
type BatchAnalysisConfig struct {
	BatchSizes        []int                     `json:"batch_sizes"`        // Batch sizes to test (e.g., 1, 5, 10, 25, 50, 100)
	TestTexts         []string                  `json:"test_texts"`         // Sample texts for analysis
	CostPerToken      float64                   `json:"cost_per_token"`     // Cost per token for calculations
	CostPerRequest    float64                   `json:"cost_per_request"`   // Base cost per API request
	MaxLatency        time.Duration             `json:"max_latency"`        // Maximum acceptable latency
	MaxCostPerToken   float64                   `json:"max_cost_per_token"` // Maximum acceptable cost per token
	ConcurrencyLevels []int                     `json:"concurrency_levels"` // Concurrency levels to test
	MemoryLimits      []int64                   `json:"memory_limits"`      // Memory limits to test (bytes)
	TargetThroughput  float64                   `json:"target_throughput"`  // Target throughput (requests/second)
	OptimizationGoals []OptimizationGoal        `json:"optimization_goals"` // Optimization goals
	EmbeddingOptions  outbound.EmbeddingOptions `json:"embedding_options"`  // Embedding options to use
	TestDuration      time.Duration             `json:"test_duration"`      // Duration for benchmark tests
	WarmupDuration    time.Duration             `json:"warmup_duration"`    // Warmup duration before measurements
	NetworkDelay      time.Duration             `json:"network_delay"`      // Simulated network delay per request
}

// OptimizationGoal defines different optimization targets.
type OptimizationGoal string

const (
	OptimizeMinCost       OptimizationGoal = "min_cost"       // Minimize total cost
	OptimizeMinLatency    OptimizationGoal = "min_latency"    // Minimize latency
	OptimizeMaxThroughput OptimizationGoal = "max_throughput" // Maximize throughput
	OptimizeBalanced      OptimizationGoal = "balanced"       // Balance cost and performance
)

// OptimizationConstraints defines constraints for batch size optimization.
type OptimizationConstraints struct {
	MaxCost             *float64             `json:"max_cost,omitempty"`         // Maximum cost constraint
	MaxLatency          *time.Duration       `json:"max_latency,omitempty"`      // Maximum latency constraint
	MinThroughput       *float64             `json:"min_throughput,omitempty"`   // Minimum throughput constraint
	MaxMemoryUsage      *int64               `json:"max_memory_usage,omitempty"` // Maximum memory constraint
	OptimizationGoal    OptimizationGoal     `json:"optimization_goal"`          // Primary optimization goal
	TextCharacteristics *TextCharacteristics `json:"text_characteristics"`       // Characteristics of text to process
	ScenarioType        ScenarioType         `json:"scenario_type"`              // Type of use case scenario
}

// TextCharacteristics describes the characteristics of text being processed.
type TextCharacteristics struct {
	AverageLength   int     `json:"average_length"`   // Average text length
	LengthVariation float64 `json:"length_variation"` // Coefficient of variation in length
	Language        string  `json:"language"`         // Primary language
	ContentType     string  `json:"content_type"`     // Type of content (code, docs, etc.)
	ComplexityScore float64 `json:"complexity_score"` // Text complexity score
}

// ScenarioType defines different use case scenarios.
type ScenarioType string

const (
	ScenarioRealTime    ScenarioType = "real_time"   // Real-time processing needs
	ScenarioBatch       ScenarioType = "batch"       // Batch processing scenarios
	ScenarioInteractive ScenarioType = "interactive" // Interactive user experiences
	ScenarioBackground  ScenarioType = "background"  // Background processing
)

// BatchAnalysisResult contains the results of batch size analysis.
type BatchAnalysisResult struct {
	Config             *BatchAnalysisConfig     `json:"config"`              // Analysis configuration used
	CostMetrics        *CostMetrics             `json:"cost_metrics"`        // Cost analysis results
	LatencyMetrics     *LatencyMetrics          `json:"latency_metrics"`     // Latency analysis results
	PerformanceMetrics *PerformanceMetrics      `json:"performance_metrics"` // Performance benchmark results
	OptimalBatchSizes  map[OptimizationGoal]int `json:"optimal_batch_sizes"` // Optimal batch sizes by goal
	Recommendations    []*BatchRecommendation   `json:"recommendations"`     // Batch size recommendations
	TestSummary        *TestSummary             `json:"test_summary"`        // Summary of all tests
	GeneratedAt        time.Time                `json:"generated_at"`        // When analysis was performed
	AnalysisDuration   time.Duration            `json:"analysis_duration"`   // Total time for analysis
}

// CostMetrics contains cost analysis results for different batch sizes.
type CostMetrics struct {
	CostPerBatchSize  map[int]*BatchCostResult `json:"cost_per_batch_size"` // Cost results by batch size
	TotalRequestCosts map[int]float64          `json:"total_request_costs"` // Total request costs by batch size
	CostPerToken      map[int]float64          `json:"cost_per_token"`      // Cost per token by batch size
	CostEfficiency    map[int]float64          `json:"cost_efficiency"`     // Cost efficiency score by batch size
	OptimalCostBatch  int                      `json:"optimal_cost_batch"`  // Most cost-efficient batch size
	CostSavings       map[int]float64          `json:"cost_savings"`        // Cost savings vs batch size 1
}

// BatchCostResult contains detailed cost analysis for a specific batch size.
type BatchCostResult struct {
	BatchSize       int     `json:"batch_size"`       // Batch size analyzed
	RequestCount    int     `json:"request_count"`    // Number of API requests needed
	TotalTokens     int     `json:"total_tokens"`     // Total tokens processed
	BaseCost        float64 `json:"base_cost"`        // Base request costs
	TokenCost       float64 `json:"token_cost"`       // Token processing costs
	TotalCost       float64 `json:"total_cost"`       // Total cost for this batch size
	CostPerItem     float64 `json:"cost_per_item"`    // Cost per individual item
	CostPerToken    float64 `json:"cost_per_token"`   // Effective cost per token
	EfficiencyScore float64 `json:"efficiency_score"` // Cost efficiency score (0-1)
}

// LatencyMetrics contains latency analysis results for different batch sizes.
type LatencyMetrics struct {
	LatencyPerBatchSize map[int]*BatchLatencyResult `json:"latency_per_batch_size"` // Latency results by batch size
	SequentialLatency   map[int]time.Duration       `json:"sequential_latency"`     // Sequential processing latency
	ParallelLatency     map[int]time.Duration       `json:"parallel_latency"`       // Parallel processing latency
	NetworkOverhead     map[int]time.Duration       `json:"network_overhead"`       // Network overhead by batch size
	OptimalLatencyBatch int                         `json:"optimal_latency_batch"`  // Lowest latency batch size
	LatencyReduction    map[int]float64             `json:"latency_reduction"`      // Latency reduction vs batch size 1
}

// BatchLatencyResult contains detailed latency analysis for a specific batch size.
type BatchLatencyResult struct {
	BatchSize           int           `json:"batch_size"`            // Batch size analyzed
	MeanLatency         time.Duration `json:"mean_latency"`          // Mean request latency
	MedianLatency       time.Duration `json:"median_latency"`        // Median request latency
	P95Latency          time.Duration `json:"p95_latency"`           // 95th percentile latency
	P99Latency          time.Duration `json:"p99_latency"`           // 99th percentile latency
	MinLatency          time.Duration `json:"min_latency"`           // Minimum observed latency
	MaxLatency          time.Duration `json:"max_latency"`           // Maximum observed latency
	StandardDeviation   time.Duration `json:"standard_deviation"`    // Latency standard deviation
	NetworkRoundTrips   int           `json:"network_round_trips"`   // Number of network round trips
	TotalProcessingTime time.Duration `json:"total_processing_time"` // Total time to process all items
	LatencyPerItem      time.Duration `json:"latency_per_item"`      // Average latency per item
}

// PerformanceMetrics contains performance benchmark results.
type PerformanceMetrics struct {
	ThroughputPerBatchSize  map[int]*ThroughputResult    `json:"throughput_per_batch_size"`   // Throughput by batch size
	MemoryUsagePerBatchSize map[int]*MemoryUsageResult   `json:"memory_usage_per_batch_size"` // Memory usage by batch size
	ConcurrentPerformance   map[int]*ConcurrencyResult   `json:"concurrent_performance"`      // Concurrent processing results
	BottleneckAnalysis      *BottleneckAnalysis          `json:"bottleneck_analysis"`         // Bottleneck identification
	OptimalThroughputBatch  int                          `json:"optimal_throughput_batch"`    // Best throughput batch size
	ResourceUtilization     map[int]*ResourceUtilization `json:"resource_utilization"`        // Resource utilization stats
}

// ThroughputResult contains throughput analysis for a specific batch size.
type ThroughputResult struct {
	BatchSize           int     `json:"batch_size"`           // Batch size analyzed
	RequestsPerSecond   float64 `json:"requests_per_second"`  // API requests per second
	ItemsPerSecond      float64 `json:"items_per_second"`     // Individual items per second
	TokensPerSecond     float64 `json:"tokens_per_second"`    // Tokens processed per second
	ThroughputScore     float64 `json:"throughput_score"`     // Throughput efficiency score
	SustainedThroughput float64 `json:"sustained_throughput"` // Sustained throughput over time
}

// MemoryUsageResult contains memory usage analysis for a specific batch size.
type MemoryUsageResult struct {
	BatchSize          int     `json:"batch_size"`           // Batch size analyzed
	PeakMemoryUsage    int64   `json:"peak_memory_usage"`    // Peak memory usage (bytes)
	AverageMemoryUsage int64   `json:"average_memory_usage"` // Average memory usage (bytes)
	MemoryPerItem      int64   `json:"memory_per_item"`      // Memory per item (bytes)
	MemoryEfficiency   float64 `json:"memory_efficiency"`    // Memory efficiency score
	GCPressure         float64 `json:"gc_pressure"`          // Garbage collection pressure
}

// ConcurrencyResult contains concurrent processing analysis.
type ConcurrencyResult struct {
	BatchSize          int     `json:"batch_size"`          // Batch size analyzed
	ConcurrencyLevel   int     `json:"concurrency_level"`   // Number of concurrent operations
	TotalThroughput    float64 `json:"total_throughput"`    // Total throughput achieved
	EfficiencyRatio    float64 `json:"efficiency_ratio"`    // Efficiency vs sequential
	ContentionScore    float64 `json:"contention_score"`    // Resource contention score
	OptimalConcurrency int     `json:"optimal_concurrency"` // Optimal concurrency level
}

// BottleneckAnalysis identifies performance bottlenecks.
type BottleneckAnalysis struct {
	PrimaryBottleneck   string             `json:"primary_bottleneck"`    // Primary bottleneck (network, CPU, memory)
	BottleneckScores    map[string]float64 `json:"bottleneck_scores"`     // Bottleneck impact scores
	NetworkBoundBatches []int              `json:"network_bound_batches"` // Network-bound batch sizes
	CPUBoundBatches     []int              `json:"cpu_bound_batches"`     // CPU-bound batch sizes
	MemoryBoundBatches  []int              `json:"memory_bound_batches"`  // Memory-bound batch sizes
	Recommendations     []string           `json:"recommendations"`       // Performance recommendations
}

// ResourceUtilization contains resource utilization statistics.
type ResourceUtilization struct {
	BatchSize          int     `json:"batch_size"`          // Batch size analyzed
	CPUUtilization     float64 `json:"cpu_utilization"`     // CPU utilization percentage
	MemoryUtilization  float64 `json:"memory_utilization"`  // Memory utilization percentage
	NetworkUtilization float64 `json:"network_utilization"` // Network utilization percentage
	UtilizationScore   float64 `json:"utilization_score"`   // Overall utilization efficiency
}

// BatchRecommendation provides batch size recommendations for specific scenarios.
type BatchRecommendation struct {
	Scenario           string                   `json:"scenario"`            // Use case scenario
	RecommendedBatch   int                      `json:"recommended_batch"`   // Recommended batch size
	OptimizationGoal   OptimizationGoal         `json:"optimization_goal"`   // Primary optimization goal
	ExpectedCost       float64                  `json:"expected_cost"`       // Expected cost per 1000 items
	ExpectedLatency    time.Duration            `json:"expected_latency"`    // Expected latency per batch
	ExpectedThroughput float64                  `json:"expected_throughput"` // Expected throughput
	Confidence         float64                  `json:"confidence"`          // Confidence score (0-1)
	Constraints        []string                 `json:"constraints"`         // Constraints considered
	AlternativeOptions []AlternativeBatchOption `json:"alternative_options"` // Alternative batch size options
}

// AlternativeBatchOption represents an alternative batch size option.
type AlternativeBatchOption struct {
	BatchSize           int           `json:"batch_size"`           // Alternative batch size
	TradeoffDescription string        `json:"tradeoff_description"` // Description of tradeoffs
	CostDifference      float64       `json:"cost_difference"`      // Cost difference vs recommended
	LatencyDifference   time.Duration `json:"latency_difference"`   // Latency difference vs recommended
}

// TestSummary provides a summary of all batch analysis tests.
type TestSummary struct {
	TotalTestsRun     int           `json:"total_tests_run"`     // Total number of tests executed
	TestsSucceeded    int           `json:"tests_succeeded"`     // Number of successful tests
	TestsFailed       int           `json:"tests_failed"`        // Number of failed tests
	BatchSizesTested  []int         `json:"batch_sizes_tested"`  // All batch sizes tested
	TotalItemsTested  int           `json:"total_items_tested"`  // Total items processed
	TotalTokensTested int           `json:"total_tokens_tested"` // Total tokens processed
	TestExecutionTime time.Duration `json:"test_execution_time"` // Total test execution time
	ErrorsSummary     []TestError   `json:"errors_summary"`      // Summary of errors encountered
}

// TestError represents an error encountered during testing.
type TestError struct {
	BatchSize    int    `json:"batch_size"`    // Batch size when error occurred
	ErrorType    string `json:"error_type"`    // Type of error
	ErrorMessage string `json:"error_message"` // Error message
	Occurrence   int    `json:"occurrence"`    // Number of times this error occurred
}

// PerformancePrediction contains predicted performance metrics.
type PerformancePrediction struct {
	BatchSize           int                 `json:"batch_size"`           // Predicted batch size
	PredictedCost       float64             `json:"predicted_cost"`       // Predicted cost
	PredictedLatency    time.Duration       `json:"predicted_latency"`    // Predicted latency
	PredictedThroughput float64             `json:"predicted_throughput"` // Predicted throughput
	ConfidenceInterval  *ConfidenceInterval `json:"confidence_interval"`  // Confidence intervals
	ModelAccuracy       float64             `json:"model_accuracy"`       // Prediction model accuracy
}

// ConfidenceInterval represents confidence intervals for predictions.
type ConfidenceInterval struct {
	CostRange       [2]float64       `json:"cost_range"`       // Cost confidence range
	LatencyRange    [2]time.Duration `json:"latency_range"`    // Latency confidence range
	ThroughputRange [2]float64       `json:"throughput_range"` // Throughput confidence range
	ConfidenceLevel float64          `json:"confidence_level"` // Confidence level (e.g., 0.95)
}
