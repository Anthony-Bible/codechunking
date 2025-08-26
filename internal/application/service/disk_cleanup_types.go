package service

import (
	"time"
)

// CleanupStrategyType represents different cleanup strategies.
type CleanupStrategyType int

const (
	CleanupStrategyLRU CleanupStrategyType = iota
	CleanupStrategyTTL
	CleanupStrategySizeBased
	CleanupStrategyHybrid
	CleanupStrategySmart
)

// LRUCleanupConfig holds configuration for LRU cleanup.
type LRUCleanupConfig struct {
	Path              string   `json:"path"`
	MaxItems          int      `json:"max_items"`
	MinFreeSpaceBytes int64    `json:"min_free_space_bytes"`
	DryRun            bool     `json:"dry_run"`
	PreservePriority  bool     `json:"preserve_priority"`
	AccessWeight      float64  `json:"access_weight"`
	AgeWeight         float64  `json:"age_weight"`
	SizeWeight        float64  `json:"size_weight"`
	ExcludePatterns   []string `json:"exclude_patterns"`
}

// TTLCleanupConfig holds configuration for TTL cleanup.
type TTLCleanupConfig struct {
	Path             string        `json:"path"`
	MaxAge           time.Duration `json:"max_age"`
	MaxIdleTime      time.Duration `json:"max_idle_time"`
	DryRun           bool          `json:"dry_run"`
	GracePeriod      time.Duration `json:"grace_period"`
	PreservePinned   bool          `json:"preserve_pinned"`
	CleanupBatchSize int           `json:"cleanup_batch_size"`
	VerifyIntegrity  bool          `json:"verify_integrity"`
}

// SizeBasedCleanupConfig holds configuration for size-based cleanup.
type SizeBasedCleanupConfig struct {
	Path                 string `json:"path"`
	TargetSizeBytes      int64  `json:"target_size_bytes"`
	MaxSizeBytes         int64  `json:"max_size_bytes"`
	DryRun               bool   `json:"dry_run"`
	Strategy             string `json:"strategy"` // "largest_first", "oldest_first", "least_used_first"
	PreserveCritical     bool   `json:"preserve_critical"`
	CompressionEnabled   bool   `json:"compression_enabled"`
	DeduplicationEnabled bool   `json:"deduplication_enabled"`
}

// HybridCleanupConfig combines multiple cleanup strategies.
type HybridCleanupConfig struct {
	Path            string                          `json:"path"`
	LRUConfig       *LRUCleanupConfig               `json:"lru_config"`
	TTLConfig       *TTLCleanupConfig               `json:"ttl_config"`
	SizeConfig      *SizeBasedCleanupConfig         `json:"size_config"`
	StrategyWeights map[CleanupStrategyType]float64 `json:"strategy_weights"`
	DryRun          bool                            `json:"dry_run"`
}

// SmartCleanupConfig uses machine learning to optimize cleanup.
type SmartCleanupConfig struct {
	Path              string         `json:"path"`
	TargetFreeSpaceGB int64          `json:"target_free_space_gb"`
	PredictionWindow  time.Duration  `json:"prediction_window"`
	LearningEnabled   bool           `json:"learning_enabled"`
	ModelType         string         `json:"model_type"`
	DryRun            bool           `json:"dry_run"`
	UsagePatterns     []UsagePattern `json:"usage_patterns"`
	OptimizationGoals []string       `json:"optimization_goals"`
}

// UsagePattern represents repository usage patterns.
type UsagePattern struct {
	Pattern    string    `json:"pattern"`
	Frequency  int       `json:"frequency"`
	LastSeen   time.Time `json:"last_seen"`
	Importance float64   `json:"importance"`
}

// CleanupCandidate represents a repository candidate for cleanup.
type CleanupCandidate struct {
	RepositoryURL    string    `json:"repository_url"`
	Path             string    `json:"path"`
	SizeBytes        int64     `json:"size_bytes"`
	LastAccessed     time.Time `json:"last_accessed"`
	AccessCount      int64     `json:"access_count"`
	CreatedAt        time.Time `json:"created_at"`
	Priority         int       `json:"priority"`
	Score            float64   `json:"score"`
	Reason           string    `json:"reason"`
	SafeToDelete     bool      `json:"safe_to_delete"`
	EstimatedSavings int64     `json:"estimated_savings"`
}

// CleanupResult contains the results of a cleanup operation.
type CleanupResult struct {
	Strategy          CleanupStrategyType `json:"strategy"`
	TotalCandidates   int                 `json:"total_candidates"`
	CleanedItems      int                 `json:"cleaned_items"`
	BytesFreed        int64               `json:"bytes_freed"`
	Duration          time.Duration       `json:"duration"`
	ErrorsEncountered []CleanupError      `json:"errors_encountered"`
	DryRun            bool                `json:"dry_run"`
	ItemsProcessed    []*ProcessedItem    `json:"items_processed"`
	Performance       *CleanupPerformance `json:"performance"`
}

// CleanupError represents an error during cleanup.
type CleanupError struct {
	ItemPath    string `json:"item_path"`
	ErrorType   string `json:"error_type"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

// ProcessedItem represents an item that was processed during cleanup.
type ProcessedItem struct {
	Path        string        `json:"path"`
	Action      string        `json:"action"` // "deleted", "compressed", "moved", "skipped"
	SizeFreed   int64         `json:"size_freed"`
	Reason      string        `json:"reason"`
	Success     bool          `json:"success"`
	ProcessTime time.Duration `json:"process_time"`
}

// CleanupPerformance contains performance metrics for cleanup.
type CleanupPerformance struct {
	ThroughputMBps  float64 `json:"throughput_mbps"`
	ItemsPerSecond  float64 `json:"items_per_second"`
	CPUUtilization  float64 `json:"cpu_utilization"`
	IOUtilization   float64 `json:"io_utilization"`
	ParallelWorkers int     `json:"parallel_workers"`
}

// DiskPressure represents current disk pressure conditions.
type DiskPressure struct {
	CurrentUsagePercent float64       `json:"current_usage_percent"`
	FreeSpaceBytes      int64         `json:"free_space_bytes"`
	GrowthRatePerDay    float64       `json:"growth_rate_per_day"`
	TimeToFull          time.Duration `json:"time_to_full"`
	Severity            PressureLevel `json:"severity"`
}

// PressureLevel represents disk pressure severity.
type PressureLevel int

const (
	PressureLow PressureLevel = iota
	PressureMedium
	PressureHigh
	PressureCritical
)

// StrategyRecommendation contains cleanup strategy recommendations.
type StrategyRecommendation struct {
	RecommendedStrategy   CleanupStrategyType `json:"recommended_strategy"`
	Confidence            float64             `json:"confidence"`
	Reasoning             []string            `json:"reasoning"`
	EstimatedFreed        int64               `json:"estimated_freed"`
	EstimatedDuration     time.Duration       `json:"estimated_duration"`
	RiskLevel             string              `json:"risk_level"`
	AlternativeStrategies []*StrategyOption   `json:"alternative_strategies"`
}

// StrategyOption represents an alternative cleanup strategy.
type StrategyOption struct {
	Strategy       CleanupStrategyType `json:"strategy"`
	Score          float64             `json:"score"`
	Pros           []string            `json:"pros"`
	Cons           []string            `json:"cons"`
	EstimatedFreed int64               `json:"estimated_freed"`
}

// StrategyComparison compares different cleanup strategies.
type StrategyComparison struct {
	Strategies         []CleanupStrategyType `json:"strategies"`
	ComparisonResults  []*StrategyResult     `json:"comparison_results"`
	BestStrategy       CleanupStrategyType   `json:"best_strategy"`
	ComparisonCriteria []string              `json:"comparison_criteria"`
}

// StrategyResult contains results for a specific strategy comparison.
type StrategyResult struct {
	Strategy          CleanupStrategyType `json:"strategy"`
	Score             float64             `json:"score"`
	EstimatedFreedMB  int64               `json:"estimated_freed_mb"`
	EstimatedDuration time.Duration       `json:"estimated_duration"`
	RiskAssessment    string              `json:"risk_assessment"`
	Effectiveness     float64             `json:"effectiveness"`
	Efficiency        float64             `json:"efficiency"`
}
