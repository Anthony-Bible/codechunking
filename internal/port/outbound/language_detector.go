package outbound

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"fmt"
	"io"
	"time"
)

// LanguageDetector defines the interface for detecting programming languages from files.
// This interface supports multiple detection strategies and provides comprehensive
// language identification capabilities for the code processing pipeline.
type LanguageDetector interface {
	// DetectFromFilePath detects language based on file path and extension.
	DetectFromFilePath(ctx context.Context, filePath string) (valueobject.Language, error)

	// DetectFromContent detects language based on file content analysis.
	DetectFromContent(ctx context.Context, content []byte, hint string) (valueobject.Language, error)

	// DetectFromReader detects language from a reader interface.
	DetectFromReader(ctx context.Context, reader io.Reader, filename string) (valueobject.Language, error)

	// DetectMultipleLanguages detects all languages present in a file (e.g., HTML with embedded JS/CSS).
	DetectMultipleLanguages(ctx context.Context, content []byte, filename string) ([]valueobject.Language, error)

	// DetectBatch performs batch language detection for multiple files efficiently.
	DetectBatch(ctx context.Context, files []FileInfo) ([]DetectionResult, error)
}

// EnhancedLanguageDetector extends LanguageDetector with advanced capabilities.
type EnhancedLanguageDetector interface {
	LanguageDetector

	// DetectWithHeuristics uses advanced heuristics for ambiguous files.
	DetectWithHeuristics(
		ctx context.Context,
		content []byte,
		filename string,
		options HeuristicOptions,
	) (valueobject.Language, error)

	// DetectFromShebang detects language from shebang lines in scripts.
	DetectFromShebang(ctx context.Context, content []byte) (valueobject.Language, error)

	// DetectBinaryFile determines if a file is binary and should be skipped.
	DetectBinaryFile(ctx context.Context, content []byte, filename string) (bool, error)

	// ValidateLanguageSupport checks if a detected language is supported for processing.
	ValidateLanguageSupport(ctx context.Context, language valueobject.Language) (bool, error)

	// GetSupportedLanguages returns all languages supported by the detector.
	GetSupportedLanguages(ctx context.Context) ([]valueobject.Language, error)

	// GetLanguageStatistics returns statistics about language detection performance.
	GetLanguageStatistics(ctx context.Context) (*DetectionStatistics, error)
}

// ConfigurableLanguageDetector extends EnhancedLanguageDetector with configuration capabilities.
type ConfigurableLanguageDetector interface {
	EnhancedLanguageDetector

	// UpdateConfiguration updates the detector configuration at runtime.
	UpdateConfiguration(ctx context.Context, config *DetectionConfig) error

	// GetConfiguration returns the current detector configuration.
	GetConfiguration(ctx context.Context) (*DetectionConfig, error)

	// AddCustomLanguage adds a custom language definition to the detector.
	AddCustomLanguage(ctx context.Context, definition *CustomLanguageDefinition) error

	// RemoveCustomLanguage removes a custom language definition.
	RemoveCustomLanguage(ctx context.Context, languageName string) error

	// ValidateConfiguration validates a detection configuration.
	ValidateConfiguration(ctx context.Context, config *DetectionConfig) error
}

// PerformanceOptimizedDetector extends ConfigurableLanguageDetector with performance features.
type PerformanceOptimizedDetector interface {
	ConfigurableLanguageDetector

	// DetectWithCache uses caching to improve detection performance.
	DetectWithCache(
		ctx context.Context,
		content []byte,
		filename string,
		cacheOptions *CacheOptions,
	) (valueobject.Language, error)

	// PrewarmCache prewarms the detection cache with common patterns.
	PrewarmCache(ctx context.Context, patterns []string) error

	// ClearCache clears the detection cache.
	ClearCache(ctx context.Context) error

	// GetCacheStatistics returns cache performance statistics.
	GetCacheStatistics(ctx context.Context) (*DetectionCacheStatistics, error)

	// DetectConcurrently performs concurrent detection for improved performance.
	DetectConcurrently(ctx context.Context, files []FileInfo, concurrency int) ([]DetectionResult, error)
}

// ObservableLanguageDetector extends PerformanceOptimizedDetector with observability features.
type ObservableLanguageDetector interface {
	PerformanceOptimizedDetector

	// RecordMetrics records detection metrics for observability.
	RecordMetrics(ctx context.Context, result *DetectionResult) error

	// GetHealthStatus returns the health status of the detector.
	GetHealthStatus(ctx context.Context) (*DetectorHealthStatus, error)

	// EnableTracing enables distributed tracing for detection operations.
	EnableTracing(ctx context.Context, enabled bool) error

	// GetDetectionMetrics returns comprehensive detection metrics.
	GetDetectionMetrics(ctx context.Context, timeRange TimeRange) (*DetectionMetrics, error)
}

// FileInfo represents information about a file to be processed.
type FileInfo struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	IsDirectory bool      `json:"is_directory"`
	IsSymlink   bool      `json:"is_symlink"`
	Permissions string    `json:"permissions"`
	ContentHash string    `json:"content_hash,omitempty"`
	GitIgnored  bool      `json:"git_ignored"`
	IsBinary    *bool     `json:"is_binary,omitempty"` // Cached binary detection result
}

// DetectionResult represents the result of language detection.
type DetectionResult struct {
	FileInfo       FileInfo               `json:"file_info"`
	Language       valueobject.Language   `json:"language"`
	SecondaryLangs []valueobject.Language `json:"secondary_languages,omitempty"`
	Confidence     float64                `json:"confidence"`
	DetectionTime  time.Duration          `json:"detection_time"`
	Method         string                 `json:"method"`
	Error          error                  `json:"error,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// HeuristicOptions configures heuristic-based language detection.
type HeuristicOptions struct {
	MaxSampleSize    int           `json:"max_sample_size"`   // Maximum bytes to analyze for heuristics
	MinConfidence    float64       `json:"min_confidence"`    // Minimum confidence threshold
	EnableKeywords   bool          `json:"enable_keywords"`   // Use keyword-based detection
	EnableSyntax     bool          `json:"enable_syntax"`     // Use syntax pattern detection
	EnableStatistics bool          `json:"enable_statistics"` // Use statistical analysis
	Timeout          time.Duration `json:"timeout"`           // Maximum time for heuristic analysis
}

// DetectionConfig configures the language detection behavior.
type DetectionConfig struct {
	// Core detection settings
	EnableExtensionDetection bool `json:"enable_extension_detection"`
	EnableContentDetection   bool `json:"enable_content_detection"`
	EnableShebangDetection   bool `json:"enable_shebang_detection"`
	EnableHeuristicDetection bool `json:"enable_heuristic_detection"`

	// Performance settings
	MaxFileSize          int64         `json:"max_file_size"`
	MaxContentSampleSize int           `json:"max_content_sample_size"`
	DetectionTimeout     time.Duration `json:"detection_timeout"`
	ConcurrentWorkers    int           `json:"concurrent_workers"`

	// Confidence thresholds
	ExtensionConfidence  float64 `json:"extension_confidence"`
	ContentConfidence    float64 `json:"content_confidence"`
	ShebangConfidence    float64 `json:"shebang_confidence"`
	HeuristicConfidence  float64 `json:"heuristic_confidence"`
	MinOverallConfidence float64 `json:"min_overall_confidence"`

	// Language-specific settings
	SupportedLanguages []string                    `json:"supported_languages"`
	LanguagePriorities map[string]int              `json:"language_priorities"`
	CustomLanguages    []*CustomLanguageDefinition `json:"custom_languages"`

	// Binary file detection
	BinaryDetectionEnabled bool    `json:"binary_detection_enabled"`
	BinarySampleSize       int     `json:"binary_sample_size"`
	BinaryThreshold        float64 `json:"binary_threshold"`

	// Caching settings
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"`
	CacheMaxSize int           `json:"cache_max_size"`

	// Observability settings
	MetricsEnabled  bool `json:"metrics_enabled"`
	TracingEnabled  bool `json:"tracing_enabled"`
	DetailedLogging bool `json:"detailed_logging"`
}

// CustomLanguageDefinition defines a custom language for detection.
type CustomLanguageDefinition struct {
	Name            string                   `json:"name"`
	Aliases         []string                 `json:"aliases"`
	Extensions      []string                 `json:"extensions"`
	Type            valueobject.LanguageType `json:"type"`
	Patterns        []string                 `json:"patterns"`         // Regex patterns for content detection
	Keywords        []string                 `json:"keywords"`         // Language keywords
	Shebangs        []string                 `json:"shebangs"`         // Shebang patterns
	CommentPatterns []string                 `json:"comment_patterns"` // Comment syntax patterns
	Priority        int                      `json:"priority"`         // Detection priority (higher = preferred)
	Confidence      float64                  `json:"confidence"`       // Base confidence score
	Enabled         bool                     `json:"enabled"`
}

// DetectionStatistics contains statistics about detection performance.
type DetectionStatistics struct {
	TotalDetections      int64            `json:"total_detections"`
	SuccessfulDetections int64            `json:"successful_detections"`
	FailedDetections     int64            `json:"failed_detections"`
	AverageDetectionTime time.Duration    `json:"average_detection_time"`
	LanguageDistribution map[string]int64 `json:"language_distribution"`
	MethodDistribution   map[string]int64 `json:"method_distribution"`
	ConfidenceHistogram  map[string]int64 `json:"confidence_histogram"`
	ErrorDistribution    map[string]int64 `json:"error_distribution"`
	LastResetTime        time.Time        `json:"last_reset_time"`
}

// CacheOptions configures caching behavior for language detection.
type CacheOptions struct {
	Enabled        bool          `json:"enabled"`
	TTL            time.Duration `json:"ttl"`
	UseContentHash bool          `json:"use_content_hash"` // Cache based on content hash vs filename
	UseFileStat    bool          `json:"use_file_stat"`    // Include file modification time in cache key
	MaxEntries     int           `json:"max_entries"`
	EvictionPolicy string        `json:"eviction_policy"` // "lru", "lfu", "ttl"
}

// DetectionCacheStatistics contains cache performance metrics.
type DetectionCacheStatistics struct {
	Hits            int64         `json:"hits"`
	Misses          int64         `json:"misses"`
	Evictions       int64         `json:"evictions"`
	Size            int           `json:"size"`
	MaxSize         int           `json:"max_size"`
	HitRatio        float64       `json:"hit_ratio"`
	AverageHitTime  time.Duration `json:"average_hit_time"`
	AverageMissTime time.Duration `json:"average_miss_time"`
	LastResetTime   time.Time     `json:"last_reset_time"`
}

// DetectorHealthStatus represents the health status of the language detector.
type DetectorHealthStatus struct {
	Healthy            bool                `json:"healthy"`
	Status             string              `json:"status"`
	LastHealthCheck    time.Time           `json:"last_health_check"`
	Uptime             time.Duration       `json:"uptime"`
	ConfigurationValid bool                `json:"configuration_valid"`
	CacheHealthy       bool                `json:"cache_healthy"`
	SupportedLanguages int                 `json:"supported_languages"`
	CustomLanguages    int                 `json:"custom_languages"`
	RecentErrors       []string            `json:"recent_errors"`
	PerformanceMetrics *PerformanceMetrics `json:"performance_metrics"`
	ResourceUsage      *ResourceUsage      `json:"resource_usage"`
}

// PerformanceMetrics contains performance-related metrics.
type PerformanceMetrics struct {
	AverageDetectionTime time.Duration `json:"average_detection_time"`
	P95DetectionTime     time.Duration `json:"p95_detection_time"`
	P99DetectionTime     time.Duration `json:"p99_detection_time"`
	DetectionsPerSecond  float64       `json:"detections_per_second"`
	ConcurrentDetections int           `json:"concurrent_detections"`
	QueueLength          int           `json:"queue_length"`
}

// ResourceUsage contains resource utilization metrics.
type ResourceUsage struct {
	MemoryUsageMB   float64 `json:"memory_usage_mb"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	GoroutinesCount int     `json:"goroutines_count"`
	OpenFilesCount  int     `json:"open_files_count"`
	CacheSize       int64   `json:"cache_size"`
}

// TimeRange represents a time range for metrics queries.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// DetectionMetrics contains comprehensive detection metrics over a time range.
type DetectionMetrics struct {
	TimeRange              TimeRange               `json:"time_range"`
	TotalOperations        int64                   `json:"total_operations"`
	SuccessfulOperations   int64                   `json:"successful_operations"`
	FailedOperations       int64                   `json:"failed_operations"`
	AverageLatency         time.Duration           `json:"average_latency"`
	ThroughputPerSecond    float64                 `json:"throughput_per_second"`
	LanguageBreakdown      map[string]int64        `json:"language_breakdown"`
	MethodBreakdown        map[string]int64        `json:"method_breakdown"`
	ErrorBreakdown         map[string]int64        `json:"error_breakdown"`
	ConfidenceDistribution map[string]int64        `json:"confidence_distribution"`
	PerformancePercentiles *PerformancePercentiles `json:"performance_percentiles"`
}

// PerformancePercentiles contains latency percentile data.
type PerformancePercentiles struct {
	P50 time.Duration `json:"p50"`
	P90 time.Duration `json:"p90"`
	P95 time.Duration `json:"p95"`
	P99 time.Duration `json:"p99"`
}

// DetectionError represents errors that can occur during language detection.
type DetectionError struct {
	Type       string    `json:"type"`
	Message    string    `json:"message"`
	FilePath   string    `json:"file_path,omitempty"`
	Language   string    `json:"language,omitempty"`
	Confidence float64   `json:"confidence,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	StackTrace string    `json:"stack_trace,omitempty"`
	Cause      error     `json:"-"` // Original error
}

// Error implements the error interface.
func (e *DetectionError) Error() string {
	if e.FilePath != "" {
		return fmt.Sprintf("%s: %s (file: %s)", e.Type, e.Message, e.FilePath)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause error.
func (e *DetectionError) Unwrap() error {
	return e.Cause
}

// Common detection error types.
const (
	ErrorTypeInvalidFile       = "InvalidFile"
	ErrorTypeUnsupportedFormat = "UnsupportedFormat"
	ErrorTypeBinaryFile        = "BinaryFile"
	ErrorTypeTimeout           = "Timeout"
	ErrorTypeConfiguration     = "Configuration"
	ErrorTypeCache             = "Cache"
	ErrorTypeInternal          = "Internal"
)
