package outbound

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"fmt"
	"time"
)

// SubmoduleDetector defines the interface for detecting and handling Git submodules.
// This interface supports submodule discovery, parsing .gitmodules files, and
// comprehensive submodule classification for the code processing pipeline.
type SubmoduleDetector interface {
	// DetectSubmodules discovers all submodules in a repository.
	DetectSubmodules(ctx context.Context, repositoryPath string) ([]valueobject.SubmoduleInfo, error)

	// ParseGitmodulesFile parses a .gitmodules file and returns submodule configurations.
	ParseGitmodulesFile(ctx context.Context, gitmodulesPath string) ([]valueobject.SubmoduleInfo, error)

	// IsSubmoduleDirectory determines if a directory is a Git submodule.
	IsSubmoduleDirectory(
		ctx context.Context,
		directoryPath string,
		repositoryRoot string,
	) (bool, *valueobject.SubmoduleInfo, error)

	// GetSubmoduleStatus retrieves the status of a specific submodule.
	GetSubmoduleStatus(
		ctx context.Context,
		submodulePath string,
		repositoryRoot string,
	) (valueobject.SubmoduleStatus, error)

	// ValidateSubmoduleConfiguration validates a submodule configuration.
	ValidateSubmoduleConfiguration(ctx context.Context, submodule valueobject.SubmoduleInfo) error
}

// EnhancedSubmoduleDetector extends SubmoduleDetector with advanced capabilities.
type EnhancedSubmoduleDetector interface {
	SubmoduleDetector

	// DetectNestedSubmodules discovers nested submodules (submodules within submodules).
	DetectNestedSubmodules(
		ctx context.Context,
		repositoryPath string,
		maxDepth int,
	) ([]valueobject.SubmoduleInfo, error)

	// ResolveSubmoduleURL resolves a submodule URL to its absolute form.
	ResolveSubmoduleURL(ctx context.Context, submoduleURL, repositoryRoot string) (string, error)

	// GetSubmoduleCommit retrieves the commit SHA that a submodule points to.
	GetSubmoduleCommit(ctx context.Context, submodulePath string, repositoryRoot string) (string, error)

	// IsSubmoduleActive determines if a submodule is active (initialized and configured).
	IsSubmoduleActive(ctx context.Context, submodulePath string, repositoryRoot string) (bool, error)

	// GetSubmoduleBranch retrieves the branch that a submodule is tracking.
	GetSubmoduleBranch(ctx context.Context, submodulePath string, repositoryRoot string) (string, error)

	// ValidateSubmoduleAccess checks if a submodule repository is accessible.
	ValidateSubmoduleAccess(ctx context.Context, submodule valueobject.SubmoduleInfo) (*SubmoduleAccessResult, error)
}

// ConfigurableSubmoduleDetector extends EnhancedSubmoduleDetector with configuration capabilities.
type ConfigurableSubmoduleDetector interface {
	EnhancedSubmoduleDetector

	// SetSubmodulePolicy sets the submodule processing policy.
	SetSubmodulePolicy(ctx context.Context, policy valueobject.SubmoduleUpdatePolicy) error

	// GetSubmodulePolicy returns the current submodule processing policy.
	GetSubmodulePolicy(ctx context.Context) (valueobject.SubmoduleUpdatePolicy, error)

	// SetMaxSubmoduleDepth sets the maximum depth for nested submodule detection.
	SetMaxSubmoduleDepth(ctx context.Context, maxDepth int) error

	// GetMaxSubmoduleDepth returns the current maximum submodule depth.
	GetMaxSubmoduleDepth(ctx context.Context) (int, error)

	// AddIgnoredSubmodule adds a submodule path to the ignore list.
	AddIgnoredSubmodule(ctx context.Context, submodulePath string) error

	// RemoveIgnoredSubmodule removes a submodule path from the ignore list.
	RemoveIgnoredSubmodule(ctx context.Context, submodulePath string) error

	// IsSubmoduleIgnored checks if a submodule is in the ignore list.
	IsSubmoduleIgnored(ctx context.Context, submodulePath string) (bool, error)
}

// ObservableSubmoduleDetector extends ConfigurableSubmoduleDetector with observability features.
type ObservableSubmoduleDetector interface {
	ConfigurableSubmoduleDetector

	// RecordSubmoduleMetrics records submodule detection metrics for observability.
	RecordSubmoduleMetrics(ctx context.Context, result *SubmoduleDetectionResult) error

	// GetSubmoduleStatistics returns submodule detection statistics.
	GetSubmoduleStatistics(ctx context.Context) (*SubmoduleStatistics, error)

	// GetHealthStatus returns the health status of the submodule detector.
	GetHealthStatus(ctx context.Context) (*SubmoduleHealthStatus, error)

	// EnableTracing enables distributed tracing for submodule operations.
	EnableTracing(ctx context.Context, enabled bool) error

	// GetSubmoduleMetrics returns comprehensive submodule metrics.
	GetSubmoduleMetrics(ctx context.Context, timeRange TimeRange) (*SubmoduleMetrics, error)
}

// SubmoduleDetectionResult represents the result of submodule detection operations.
type SubmoduleDetectionResult struct {
	RepositoryPath   string                      `json:"repository_path"`
	Submodules       []valueobject.SubmoduleInfo `json:"submodules"`
	NestedSubmodules []valueobject.SubmoduleInfo `json:"nested_submodules"`
	TotalFound       int                         `json:"total_found"`
	ActiveSubmodules int                         `json:"active_submodules"`
	ProcessingTime   time.Duration               `json:"processing_time"`
	DetectionMethod  string                      `json:"detection_method"`
	Errors           []SubmoduleDetectionError   `json:"errors,omitempty"`
	Metadata         map[string]interface{}      `json:"metadata,omitempty"`
}

// SubmoduleAccessResult represents the result of submodule access validation.
type SubmoduleAccessResult struct {
	Submodule    valueobject.SubmoduleInfo `json:"submodule"`
	IsAccessible bool                      `json:"is_accessible"`
	AccessMethod string                    `json:"access_method"` // "https", "ssh", "file", etc.
	RequiresAuth bool                      `json:"requires_auth"`
	ResponseTime time.Duration             `json:"response_time"`
	Error        *SubmoduleDetectionError  `json:"error,omitempty"`
	Metadata     map[string]interface{}    `json:"metadata,omitempty"`
}

// SubmoduleStatistics contains statistics about submodule detection performance.
type SubmoduleStatistics struct {
	TotalRepositoriesProcessed int64            `json:"total_repositories_processed"`
	TotalSubmodulesDetected    int64            `json:"total_submodules_detected"`
	ActiveSubmodules           int64            `json:"active_submodules"`
	NestedSubmodules           int64            `json:"nested_submodules"`
	BrokenSubmodules           int64            `json:"broken_submodules"`
	InaccessibleSubmodules     int64            `json:"inaccessible_submodules"`
	AverageProcessingTime      time.Duration    `json:"average_processing_time"`
	DepthDistribution          map[int]int64    `json:"depth_distribution"`
	URLSchemeDistribution      map[string]int64 `json:"url_scheme_distribution"`
	ErrorDistribution          map[string]int64 `json:"error_distribution"`
	LastResetTime              time.Time        `json:"last_reset_time"`
	CacheHitRate               float64          `json:"cache_hit_rate"`
}

// SubmoduleHealthStatus represents the health status of the submodule detector.
type SubmoduleHealthStatus struct {
	Healthy               bool                `json:"healthy"`
	Status                string              `json:"status"`
	LastHealthCheck       time.Time           `json:"last_health_check"`
	Uptime                time.Duration       `json:"uptime"`
	ConfigurationValid    bool                `json:"configuration_valid"`
	CacheHealthy          bool                `json:"cache_healthy"`
	ProcessedRepositories int64               `json:"processed_repositories"`
	ActiveDetectors       int                 `json:"active_detectors"`
	QueueLength           int                 `json:"queue_length"`
	RecentErrors          []string            `json:"recent_errors"`
	PerformanceMetrics    *PerformanceMetrics `json:"performance_metrics"`
	ResourceUsage         *ResourceUsage      `json:"resource_usage"`
}

// SubmoduleMetrics contains comprehensive submodule metrics over a time range.
type SubmoduleMetrics struct {
	TimeRange                TimeRange               `json:"time_range"`
	TotalOperations          int64                   `json:"total_operations"`
	SuccessfulOperations     int64                   `json:"successful_operations"`
	FailedOperations         int64                   `json:"failed_operations"`
	AverageLatency           time.Duration           `json:"average_latency"`
	ThroughputPerSecond      float64                 `json:"throughput_per_second"`
	DetectionMethodBreakdown map[string]int64        `json:"detection_method_breakdown"`
	SubmoduleStatusBreakdown map[string]int64        `json:"submodule_status_breakdown"`
	ErrorBreakdown           map[string]int64        `json:"error_breakdown"`
	PerformancePercentiles   *PerformancePercentiles `json:"performance_percentiles"`
	CacheEffectiveness       *CacheEffectiveness     `json:"cache_effectiveness"`
}

// SubmoduleDetectionError represents errors that can occur during submodule detection.
type SubmoduleDetectionError struct {
	Type       string                     `json:"type"`
	Message    string                     `json:"message"`
	Submodule  *valueobject.SubmoduleInfo `json:"submodule,omitempty"`
	FilePath   string                     `json:"file_path,omitempty"`
	Timestamp  time.Time                  `json:"timestamp"`
	StackTrace string                     `json:"stack_trace,omitempty"`
	Cause      error                      `json:"-"` // Original error
}

// Error implements the error interface.
func (e *SubmoduleDetectionError) Error() string {
	if e.FilePath != "" {
		return fmt.Sprintf("%s: %s (file: %s)", e.Type, e.Message, e.FilePath)
	}
	if e.Submodule != nil {
		return fmt.Sprintf("%s: %s (submodule: %s)", e.Type, e.Message, e.Submodule.Path())
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause error.
func (e *SubmoduleDetectionError) Unwrap() error {
	return e.Cause
}

// Common submodule detection error types.
const (
	ErrorTypeGitmodulesNotFound    = "GitmodulesNotFound"
	ErrorTypeGitmodulesParsing     = "GitmodulesParsing"
	ErrorTypeSubmoduleNotFound     = "SubmoduleNotFound"
	ErrorTypeSubmoduleInaccessible = "SubmoduleInaccessible"
	ErrorTypeSubmoduleInvalid      = "SubmoduleInvalid"
	ErrorTypeSubmoduleCircular     = "SubmoduleCircular"
	ErrorTypeSubmoduleDepthLimit   = "SubmoduleDepthLimit"
	ErrorTypeSubmoduleGitError     = "SubmoduleGitError"
)

// Submodule detection methods.
const (
	SubmoduleDetectionMethodGitmodules = "gitmodules"
	SubmoduleDetectionMethodGitCommand = "git_command"
	SubmoduleDetectionMethodFileSystem = "filesystem"
	SubmoduleDetectionMethodCombined   = "combined"
)

// Common helper functions and utilities.

// GetDefaultSubmoduleDetectionConfig returns the default submodule detection configuration.
func GetDefaultSubmoduleDetectionConfig() *SubmoduleDetectionConfig {
	return &SubmoduleDetectionConfig{
		Policy:            valueobject.SubmoduleUpdatePolicyShallow,
		MaxDepth:          3,
		DetectionMethod:   SubmoduleDetectionMethodCombined,
		EnableCaching:     true,
		CacheTTL:          15 * time.Minute,
		ProcessingTimeout: 30 * time.Second,
		EnableMetrics:     true,
		EnableTracing:     false,
		IgnorePatterns:    []string{},
	}
}

// SubmoduleDetectionConfig configures submodule detection behavior.
type SubmoduleDetectionConfig struct {
	// Processing settings
	Policy            valueobject.SubmoduleUpdatePolicy `json:"policy"`
	MaxDepth          int                               `json:"max_depth"`
	DetectionMethod   string                            `json:"detection_method"`
	ProcessingTimeout time.Duration                     `json:"processing_timeout"`

	// Filtering settings
	IgnorePatterns       []string `json:"ignore_patterns"`
	IgnoredSubmodules    []string `json:"ignored_submodules"`
	OnlyActiveSubmodules bool     `json:"only_active_submodules"`

	// Performance settings
	EnableCaching     bool          `json:"enable_caching"`
	CacheTTL          time.Duration `json:"cache_ttl"`
	CacheMaxSize      int           `json:"cache_max_size"`
	ConcurrentWorkers int           `json:"concurrent_workers"`

	// Observability settings
	EnableMetrics         bool    `json:"enable_metrics"`
	EnableTracing         bool    `json:"enable_tracing"`
	DetailedLogging       bool    `json:"detailed_logging"`
	MetricsCollectionRate float64 `json:"metrics_collection_rate"`
}
