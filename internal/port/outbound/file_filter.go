package outbound

import (
	"context"
	"fmt"
	"time"
)

// FileFilter defines the interface for filtering files during repository processing.
// This interface supports binary file detection, gitignore pattern matching, and
// comprehensive file classification for the code processing pipeline.
type FileFilter interface {
	// ShouldProcessFile determines if a file should be processed based on filtering rules.
	ShouldProcessFile(ctx context.Context, filePath string, fileInfo FileInfo) (FilterDecision, error)

	// DetectBinaryFile determines if a file is binary and should be excluded.
	DetectBinaryFile(ctx context.Context, filePath string, content []byte) (bool, error)

	// DetectBinaryFromPath determines if a file is binary based on extension only.
	DetectBinaryFromPath(ctx context.Context, filePath string) (bool, error)

	// MatchesGitignorePatterns checks if a file matches any gitignore patterns.
	MatchesGitignorePatterns(ctx context.Context, filePath string, repoPath string) (bool, error)

	// LoadGitignorePatterns loads gitignore patterns from repository.
	LoadGitignorePatterns(ctx context.Context, repoPath string) ([]GitignorePattern, error)

	// FilterFilesBatch performs batch file filtering for multiple files efficiently.
	FilterFilesBatch(ctx context.Context, files []FileInfo, repoPath string) ([]FilterResult, error)
}

// EnhancedFileFilter extends FileFilter with advanced filtering capabilities.
type EnhancedFileFilter interface {
	FileFilter

	// DetectBinaryWithHeuristics uses advanced heuristics for binary detection.
	DetectBinaryWithHeuristics(
		ctx context.Context,
		filePath string,
		content []byte,
		options BinaryDetectionOptions,
	) (BinaryDetectionResult, error)

	// ParseGitignoreFile parses a single gitignore file and returns patterns.
	ParseGitignoreFile(ctx context.Context, gitignorePath string) ([]GitignorePattern, error)

	// TestGitignorePattern tests if a pattern matches a given file path.
	TestGitignorePattern(ctx context.Context, pattern GitignorePattern, filePath string) (bool, error)

	// GetFilterStatistics returns filtering performance statistics.
	GetFilterStatistics(ctx context.Context) (*FilterStatistics, error)

	// ValidateFilterConfiguration validates the current filter configuration.
	ValidateFilterConfiguration(ctx context.Context, config *FilterConfig) error

	// UpdateFilterConfiguration updates filter configuration at runtime.
	UpdateFilterConfiguration(ctx context.Context, config *FilterConfig) error
}

// ConfigurableFileFilter extends EnhancedFileFilter with configuration capabilities.
type ConfigurableFileFilter interface {
	EnhancedFileFilter

	// AddCustomBinaryExtension adds a custom binary file extension.
	AddCustomBinaryExtension(ctx context.Context, extension string) error

	// RemoveCustomBinaryExtension removes a custom binary file extension.
	RemoveCustomBinaryExtension(ctx context.Context, extension string) error

	// AddCustomGitignorePattern adds a custom gitignore pattern.
	AddCustomGitignorePattern(ctx context.Context, pattern string, repoPath string) error

	// RemoveCustomGitignorePattern removes a custom gitignore pattern.
	RemoveCustomGitignorePattern(ctx context.Context, pattern string, repoPath string) error

	// GetSupportedBinaryExtensions returns all supported binary file extensions.
	GetSupportedBinaryExtensions(ctx context.Context) ([]string, error)

	// SetBinaryDetectionThreshold sets the threshold for binary content detection.
	SetBinaryDetectionThreshold(ctx context.Context, threshold float64) error
}

// PerformanceOptimizedFilter extends ConfigurableFileFilter with performance features.
type PerformanceOptimizedFilter interface {
	ConfigurableFileFilter

	// FilterWithCache uses caching to improve filtering performance.
	FilterWithCache(
		ctx context.Context,
		files []FileInfo,
		repoPath string,
		cacheOptions *FilterCacheOptions,
	) ([]FilterResult, error)

	// PrewarmFilterCache prewarms the filtering cache with common patterns.
	PrewarmFilterCache(ctx context.Context, repoPath string) error

	// ClearFilterCache clears the filtering cache.
	ClearFilterCache(ctx context.Context) error

	// GetCacheStatistics returns cache performance statistics.
	GetCacheStatistics(ctx context.Context) (*FilterCacheStatistics, error)

	// FilterConcurrently performs concurrent filtering for improved performance.
	FilterConcurrently(ctx context.Context, files []FileInfo, repoPath string, concurrency int) ([]FilterResult, error)
}

// ObservableFileFilter extends PerformanceOptimizedFilter with observability features.
type ObservableFileFilter interface {
	PerformanceOptimizedFilter

	// RecordFilterMetrics records filtering metrics for observability.
	RecordFilterMetrics(ctx context.Context, result *FilterResult) error

	// GetHealthStatus returns the health status of the filter.
	GetHealthStatus(ctx context.Context) (*FilterHealthStatus, error)

	// EnableTracing enables distributed tracing for filtering operations.
	EnableTracing(ctx context.Context, enabled bool) error

	// GetFilteringMetrics returns comprehensive filtering metrics.
	GetFilteringMetrics(ctx context.Context, timeRange TimeRange) (*FilteringMetrics, error)
}

// FilterDecision represents the decision made by the file filter.
type FilterDecision struct {
	ShouldProcess   bool                   `json:"should_process"`
	IsBinary        bool                   `json:"is_binary"`
	IsGitIgnored    bool                   `json:"is_git_ignored"`
	FilterReason    string                 `json:"filter_reason"`
	Confidence      float64                `json:"confidence"`
	ProcessingTime  time.Duration          `json:"processing_time"`
	MatchedPatterns []string               `json:"matched_patterns,omitempty"`
	DetectionMethod string                 `json:"detection_method"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// FilterResult represents the result of file filtering operations.
type FilterResult struct {
	FileInfo       FileInfo               `json:"file_info"`
	Decision       FilterDecision         `json:"decision"`
	Error          error                  `json:"error,omitempty"`
	ProcessingTime time.Duration          `json:"processing_time"`
	CacheHit       bool                   `json:"cache_hit"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// GitignorePattern represents a gitignore pattern with metadata.
type GitignorePattern struct {
	Pattern         string    `json:"pattern"`
	IsNegation      bool      `json:"is_negation"`
	IsDirectory     bool      `json:"is_directory"`
	SourceFile      string    `json:"source_file"`
	LineNumber      int       `json:"line_number"`
	Priority        int       `json:"priority"`
	CompiledPattern *Matcher  `json:"-"` // Compiled pattern matcher
	ParsedAt        time.Time `json:"parsed_at"`
}

// Matcher represents a compiled pattern matcher (interface for flexibility).
type Matcher interface {
	Match(path string) bool
	String() string
}

// BinaryDetectionOptions configures binary file detection behavior.
type BinaryDetectionOptions struct {
	EnableExtensionCheck    bool    `json:"enable_extension_check"`
	EnableContentAnalysis   bool    `json:"enable_content_analysis"`
	EnableMagicByteCheck    bool    `json:"enable_magic_byte_check"`
	EnableHeuristicAnalysis bool    `json:"enable_heuristic_analysis"`
	MaxSampleSize           int     `json:"max_sample_size"`
	BinaryThreshold         float64 `json:"binary_threshold"`
	UseFileSize             bool    `json:"use_file_size"`
	MaxFileSize             int64   `json:"max_file_size"`
}

// BinaryDetectionResult represents the result of binary file detection.
type BinaryDetectionResult struct {
	IsBinary        bool                   `json:"is_binary"`
	Confidence      float64                `json:"confidence"`
	DetectionMethod string                 `json:"detection_method"`
	Reasons         []string               `json:"reasons"`
	SampleSize      int                    `json:"sample_size"`
	ProcessingTime  time.Duration          `json:"processing_time"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// FilterConfig configures the file filtering behavior.
type FilterConfig struct {
	// Binary detection settings
	EnableBinaryDetection  bool     `json:"enable_binary_detection"`
	BinaryExtensions       []string `json:"binary_extensions"`
	CustomBinaryExtensions []string `json:"custom_binary_extensions"`
	BinaryDetectionMethod  string   `json:"binary_detection_method"` // "extension", "content", "heuristic", "all"
	BinaryThreshold        float64  `json:"binary_threshold"`
	MaxBinarySampleSize    int      `json:"max_binary_sample_size"`
	MaxFileSize            int64    `json:"max_file_size"`

	// Gitignore settings
	EnableGitignore        bool     `json:"enable_gitignore"`
	GitignoreFiles         []string `json:"gitignore_files"` // e.g., ".gitignore", ".dockerignore"
	CustomIgnorePatterns   []string `json:"custom_ignore_patterns"`
	IgnoreNestedGitignore  bool     `json:"ignore_nested_gitignore"`
	CaseSensitivePatterns  bool     `json:"case_sensitive_patterns"`
	EnableNegationPatterns bool     `json:"enable_negation_patterns"`

	// Performance settings
	EnableCaching     bool          `json:"enable_caching"`
	CacheTTL          time.Duration `json:"cache_ttl"`
	CacheMaxSize      int           `json:"cache_max_size"`
	ConcurrentWorkers int           `json:"concurrent_workers"`
	ProcessingTimeout time.Duration `json:"processing_timeout"`

	// Observability settings
	EnableMetrics         bool    `json:"enable_metrics"`
	EnableTracing         bool    `json:"enable_tracing"`
	DetailedLogging       bool    `json:"detailed_logging"`
	MetricsCollectionRate float64 `json:"metrics_collection_rate"`

	// Filter strategy
	FilterStrategy        string   `json:"filter_strategy"` // "strict", "lenient", "custom"
	AllowedFileTypes      []string `json:"allowed_file_types,omitempty"`
	BlockedFileTypes      []string `json:"blocked_file_types,omitempty"`
	MinFileSize           int64    `json:"min_file_size"`
	MaxFilesPerRepository int      `json:"max_files_per_repository"`
}

// FilterStatistics contains statistics about filtering performance.
type FilterStatistics struct {
	TotalFilesProcessed      int64            `json:"total_files_processed"`
	TotalFilesFiltered       int64            `json:"total_files_filtered"`
	BinaryFilesDetected      int64            `json:"binary_files_detected"`
	GitIgnoredFiles          int64            `json:"git_ignored_files"`
	AverageProcessingTime    time.Duration    `json:"average_processing_time"`
	FilterMethodDistribution map[string]int64 `json:"filter_method_distribution"`
	ExtensionDistribution    map[string]int64 `json:"extension_distribution"`
	ErrorDistribution        map[string]int64 `json:"error_distribution"`
	LastResetTime            time.Time        `json:"last_reset_time"`
	CacheHitRate             float64          `json:"cache_hit_rate"`
}

// FilterCacheOptions configures caching behavior for file filtering.
type FilterCacheOptions struct {
	Enabled            bool          `json:"enabled"`
	TTL                time.Duration `json:"ttl"`
	UseContentHash     bool          `json:"use_content_hash"`
	UseFileStat        bool          `json:"use_file_stat"`
	MaxEntries         int           `json:"max_entries"`
	EvictionPolicy     string        `json:"eviction_policy"` // "lru", "lfu", "ttl"
	CacheByRepository  bool          `json:"cache_by_repository"`
	InvalidateOnChange bool          `json:"invalidate_on_change"`
}

// FilterCacheStatistics contains cache performance metrics.
type FilterCacheStatistics struct {
	Hits             int64         `json:"hits"`
	Misses           int64         `json:"misses"`
	Evictions        int64         `json:"evictions"`
	Size             int           `json:"size"`
	MaxSize          int           `json:"max_size"`
	HitRatio         float64       `json:"hit_ratio"`
	AverageHitTime   time.Duration `json:"average_hit_time"`
	AverageMissTime  time.Duration `json:"average_miss_time"`
	LastResetTime    time.Time     `json:"last_reset_time"`
	MemoryUsageBytes int64         `json:"memory_usage_bytes"`
}

// FilterHealthStatus represents the health status of the file filter.
type FilterHealthStatus struct {
	Healthy            bool                `json:"healthy"`
	Status             string              `json:"status"`
	LastHealthCheck    time.Time           `json:"last_health_check"`
	Uptime             time.Duration       `json:"uptime"`
	ConfigurationValid bool                `json:"configuration_valid"`
	CacheHealthy       bool                `json:"cache_healthy"`
	ProcessedFiles     int64               `json:"processed_files"`
	ActiveWorkers      int                 `json:"active_workers"`
	QueueLength        int                 `json:"queue_length"`
	RecentErrors       []string            `json:"recent_errors"`
	PerformanceMetrics *PerformanceMetrics `json:"performance_metrics"`
	ResourceUsage      *ResourceUsage      `json:"resource_usage"`
}

// FilteringMetrics contains comprehensive filtering metrics over a time range.
type FilteringMetrics struct {
	TimeRange              TimeRange               `json:"time_range"`
	TotalOperations        int64                   `json:"total_operations"`
	SuccessfulOperations   int64                   `json:"successful_operations"`
	FailedOperations       int64                   `json:"failed_operations"`
	AverageLatency         time.Duration           `json:"average_latency"`
	ThroughputPerSecond    float64                 `json:"throughput_per_second"`
	FilterTypeBreakdown    map[string]int64        `json:"filter_type_breakdown"`
	ExtensionBreakdown     map[string]int64        `json:"extension_breakdown"`
	ErrorBreakdown         map[string]int64        `json:"error_breakdown"`
	PerformancePercentiles *PerformancePercentiles `json:"performance_percentiles"`
	CacheEffectiveness     *CacheEffectiveness     `json:"cache_effectiveness"`
}

// CacheEffectiveness contains cache performance data.
type CacheEffectiveness struct {
	HitRate            float64       `json:"hit_rate"`
	MissRate           float64       `json:"miss_rate"`
	EvictionRate       float64       `json:"eviction_rate"`
	AverageHitLatency  time.Duration `json:"average_hit_latency"`
	AverageMissLatency time.Duration `json:"average_miss_latency"`
	MemoryEfficiency   float64       `json:"memory_efficiency"`
}

// GetDefaultBinaryExtensions returns the default set of binary file extensions.
func GetDefaultBinaryExtensions() []string {
	return []string{
		// Executables
		".exe", ".dll", ".so", ".dylib", ".bin", ".obj", ".o", ".class", ".jar",

		// Archives
		".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".war", ".ear",

		// Images
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tga", ".ico", ".svg",
		".webp", ".avif", ".heic", ".raw", ".psd", ".ai", ".eps",

		// Audio/Video
		".mp3", ".wav", ".flac", ".ogg", ".aac", ".m4a", ".wma",
		".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v",

		// Documents
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp",

		// Fonts
		".ttf", ".otf", ".woff", ".woff2", ".eot",

		// Database
		".db", ".sqlite", ".sqlite3", ".mdb", ".accdb",

		// Other binary formats
		".wasm", ".deb", ".rpm", ".dmg", ".iso", ".img", ".vhd", ".vmdk",
	}
}

// GetDefaultGitignoreFiles returns the default set of gitignore file names.
func GetDefaultGitignoreFiles() []string {
	return []string{
		".gitignore",
		".dockerignore",
		".npmignore",
		".eslintignore",
		".prettierignore",
	}
}

// FilterError represents errors that can occur during file filtering.
type FilterError struct {
	Type       string    `json:"type"`
	Message    string    `json:"message"`
	FilePath   string    `json:"file_path,omitempty"`
	Pattern    string    `json:"pattern,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	StackTrace string    `json:"stack_trace,omitempty"`
	Cause      error     `json:"-"` // Original error
}

// Error implements the error interface.
func (e *FilterError) Error() string {
	if e.FilePath != "" {
		return fmt.Sprintf("%s: %s (file: %s)", e.Type, e.Message, e.FilePath)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause error.
func (e *FilterError) Unwrap() error {
	return e.Cause
}

// Common filter error types (additional to language_detector error types).
const (
	ErrorTypeInvalidPattern      = "InvalidPattern"
	ErrorTypeFileAccess          = "FileAccess"
	ErrorTypeConfigurationError  = "ConfigurationError"
	ErrorTypeBinaryDetection     = "BinaryDetection"
	ErrorTypeGitignoreProcessing = "GitignoreProcessing"
	// Note: ErrorTypeTimeout, ErrorTypeCache, ErrorTypeInternal are defined in language_detector.go.
)

// Binary detection methods.
const (
	BinaryDetectionMethodExtension = "extension"
	BinaryDetectionMethodContent   = "content"
	BinaryDetectionMethodMagicByte = "magic_byte"
	BinaryDetectionMethodHeuristic = "heuristic"
	BinaryDetectionMethodCombined  = "combined"
)

// Filter strategies.
const (
	FilterStrategyStrict  = "strict"  // Block anything that might be binary or ignored
	FilterStrategyLenient = "lenient" // Allow more files, be conservative with filtering
	FilterStrategyCustom  = "custom"  // Use custom configuration
)
