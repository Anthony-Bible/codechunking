package outbound

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"fmt"
	"time"
)

// SymlinkResolver defines the interface for detecting and resolving symbolic links.
// This interface supports symlink discovery, resolution, circular detection, and
// comprehensive symlink classification for the code processing pipeline.
type SymlinkResolver interface {
	// DetectSymlinks discovers all symbolic links in a directory tree.
	DetectSymlinks(ctx context.Context, directoryPath string) ([]valueobject.SymlinkInfo, error)

	// IsSymlink determines if a given path is a symbolic link.
	IsSymlink(ctx context.Context, filePath string) (bool, error)

	// ResolveSymlink resolves a symbolic link to its target path.
	ResolveSymlink(ctx context.Context, symlinkPath string) (*valueobject.SymlinkInfo, error)

	// GetSymlinkTarget retrieves the target path of a symbolic link.
	GetSymlinkTarget(ctx context.Context, symlinkPath string) (string, error)

	// ValidateSymlinkTarget checks if a symlink target exists and is accessible.
	ValidateSymlinkTarget(ctx context.Context, symlink valueobject.SymlinkInfo) (*SymlinkValidationResult, error)
}

// EnhancedSymlinkResolver extends SymlinkResolver with advanced capabilities.
type EnhancedSymlinkResolver interface {
	SymlinkResolver

	// ResolveSymlinkChain fully resolves a chain of symbolic links.
	ResolveSymlinkChain(ctx context.Context, symlinkPath string, maxDepth int) (*valueobject.SymlinkInfo, error)

	// DetectCircularSymlinks detects circular references in symbolic links.
	DetectCircularSymlinks(ctx context.Context, symlinkPath string) (bool, []string, error)

	// ClassifySymlinkScope determines if a symlink points inside or outside the repository.
	ClassifySymlinkScope(
		ctx context.Context,
		symlink valueobject.SymlinkInfo,
		repositoryRoot string,
	) (valueobject.SymlinkScope, error)

	// GetSymlinkType determines the type of the symlink target (file, directory, broken).
	GetSymlinkType(ctx context.Context, symlink valueobject.SymlinkInfo) (valueobject.SymlinkType, error)

	// ResolveRelativeSymlink resolves a relative symbolic link within a repository context.
	ResolveRelativeSymlink(ctx context.Context, symlinkPath, repositoryRoot string) (*valueobject.SymlinkInfo, error)

	// ValidateSymlinkSecurity checks if following a symlink would violate security constraints.
	ValidateSymlinkSecurity(
		ctx context.Context,
		symlink valueobject.SymlinkInfo,
		repositoryRoot string,
	) (*SymlinkSecurityResult, error)
}

// ConfigurableSymlinkResolver extends EnhancedSymlinkResolver with configuration capabilities.
type ConfigurableSymlinkResolver interface {
	EnhancedSymlinkResolver

	// SetSymlinkPolicy sets the symbolic link processing policy.
	SetSymlinkPolicy(ctx context.Context, policy valueobject.SymlinkResolutionPolicy) error

	// GetSymlinkPolicy returns the current symbolic link processing policy.
	GetSymlinkPolicy(ctx context.Context) (valueobject.SymlinkResolutionPolicy, error)

	// SetMaxSymlinkDepth sets the maximum depth for symlink chain resolution.
	SetMaxSymlinkDepth(ctx context.Context, maxDepth int) error

	// GetMaxSymlinkDepth returns the current maximum symlink resolution depth.
	GetMaxSymlinkDepth(ctx context.Context) (int, error)

	// AddIgnoredSymlinkPattern adds a pattern to ignore certain symbolic links.
	AddIgnoredSymlinkPattern(ctx context.Context, pattern string) error

	// RemoveIgnoredSymlinkPattern removes a pattern from the ignore list.
	RemoveIgnoredSymlinkPattern(ctx context.Context, pattern string) error

	// IsSymlinkIgnored checks if a symlink matches any ignore patterns.
	IsSymlinkIgnored(ctx context.Context, symlinkPath string) (bool, error)

	// SetAllowExternalSymlinks controls whether symlinks outside the repository are allowed.
	SetAllowExternalSymlinks(ctx context.Context, allow bool) error

	// GetAllowExternalSymlinks returns whether external symlinks are allowed.
	GetAllowExternalSymlinks(ctx context.Context) (bool, error)
}

// ObservableSymlinkResolver extends ConfigurableSymlinkResolver with observability features.
type ObservableSymlinkResolver interface {
	ConfigurableSymlinkResolver

	// RecordSymlinkMetrics records symlink resolution metrics for observability.
	RecordSymlinkMetrics(ctx context.Context, result *SymlinkResolutionResult) error

	// GetSymlinkStatistics returns symlink resolution statistics.
	GetSymlinkStatistics(ctx context.Context) (*SymlinkStatistics, error)

	// GetHealthStatus returns the health status of the symlink resolver.
	GetHealthStatus(ctx context.Context) (*SymlinkHealthStatus, error)

	// EnableTracing enables distributed tracing for symlink operations.
	EnableTracing(ctx context.Context, enabled bool) error

	// GetSymlinkMetrics returns comprehensive symlink metrics.
	GetSymlinkMetrics(ctx context.Context, timeRange TimeRange) (*SymlinkMetrics, error)
}

// SymlinkResolutionResult represents the result of symlink resolution operations.
type SymlinkResolutionResult struct {
	OriginalPath     string                    `json:"original_path"`
	ResolvedSymlinks []valueobject.SymlinkInfo `json:"resolved_symlinks"`
	FinalTarget      string                    `json:"final_target"`
	IsResolved       bool                      `json:"is_resolved"`
	IsBroken         bool                      `json:"is_broken"`
	IsCircular       bool                      `json:"is_circular"`
	ChainLength      int                       `json:"chain_length"`
	ProcessingTime   time.Duration             `json:"processing_time"`
	ResolutionMethod string                    `json:"resolution_method"`
	Errors           []SymlinkResolutionError  `json:"errors,omitempty"`
	Metadata         map[string]interface{}    `json:"metadata,omitempty"`
}

// SymlinkValidationResult represents the result of symlink target validation.
type SymlinkValidationResult struct {
	Symlink      valueobject.SymlinkInfo `json:"symlink"`
	TargetExists bool                    `json:"target_exists"`
	TargetType   valueobject.SymlinkType `json:"target_type"`
	IsAccessible bool                    `json:"is_accessible"`
	Permissions  string                  `json:"permissions"`
	ResponseTime time.Duration           `json:"response_time"`
	Error        *SymlinkResolutionError `json:"error,omitempty"`
	Metadata     map[string]interface{}  `json:"metadata,omitempty"`
}

// SymlinkSecurityResult represents the result of symlink security validation.
type SymlinkSecurityResult struct {
	Symlink               valueobject.SymlinkInfo `json:"symlink"`
	IsSecure              bool                    `json:"is_secure"`
	SecurityViolations    []string                `json:"security_violations"`
	EscapesRepository     bool                    `json:"escapes_repository"`
	PointsToSensitivePath bool                    `json:"points_to_sensitive_path"`
	ExceedsDepthLimit     bool                    `json:"exceeds_depth_limit"`
	HasCircularReference  bool                    `json:"has_circular_reference"`
	RecommendedAction     string                  `json:"recommended_action"`
	RiskLevel             string                  `json:"risk_level"` // "low", "medium", "high"
	Metadata              map[string]interface{}  `json:"metadata,omitempty"`
}

// SymlinkStatistics contains statistics about symlink resolution performance.
type SymlinkStatistics struct {
	TotalDirectoriesScanned int64            `json:"total_directories_scanned"`
	TotalSymlinksDetected   int64            `json:"total_symlinks_detected"`
	ResolvedSymlinks        int64            `json:"resolved_symlinks"`
	BrokenSymlinks          int64            `json:"broken_symlinks"`
	CircularSymlinks        int64            `json:"circular_symlinks"`
	ExternalSymlinks        int64            `json:"external_symlinks"`
	IgnoredSymlinks         int64            `json:"ignored_symlinks"`
	AverageProcessingTime   time.Duration    `json:"average_processing_time"`
	ChainLengthDistribution map[int]int64    `json:"chain_length_distribution"`
	TypeDistribution        map[string]int64 `json:"type_distribution"`
	ScopeDistribution       map[string]int64 `json:"scope_distribution"`
	ErrorDistribution       map[string]int64 `json:"error_distribution"`
	LastResetTime           time.Time        `json:"last_reset_time"`
	CacheHitRate            float64          `json:"cache_hit_rate"`
}

// SymlinkHealthStatus represents the health status of the symlink resolver.
type SymlinkHealthStatus struct {
	Healthy              bool                `json:"healthy"`
	Status               string              `json:"status"`
	LastHealthCheck      time.Time           `json:"last_health_check"`
	Uptime               time.Duration       `json:"uptime"`
	ConfigurationValid   bool                `json:"configuration_valid"`
	CacheHealthy         bool                `json:"cache_healthy"`
	ProcessedDirectories int64               `json:"processed_directories"`
	ActiveResolvers      int                 `json:"active_resolvers"`
	QueueLength          int                 `json:"queue_length"`
	RecentErrors         []string            `json:"recent_errors"`
	PerformanceMetrics   *PerformanceMetrics `json:"performance_metrics"`
	ResourceUsage        *ResourceUsage      `json:"resource_usage"`
}

// SymlinkMetrics contains comprehensive symlink metrics over a time range.
type SymlinkMetrics struct {
	TimeRange                 TimeRange               `json:"time_range"`
	TotalOperations           int64                   `json:"total_operations"`
	SuccessfulOperations      int64                   `json:"successful_operations"`
	FailedOperations          int64                   `json:"failed_operations"`
	AverageLatency            time.Duration           `json:"average_latency"`
	ThroughputPerSecond       float64                 `json:"throughput_per_second"`
	ResolutionMethodBreakdown map[string]int64        `json:"resolution_method_breakdown"`
	SymlinkTypeBreakdown      map[string]int64        `json:"symlink_type_breakdown"`
	ErrorBreakdown            map[string]int64        `json:"error_breakdown"`
	PerformancePercentiles    *PerformancePercentiles `json:"performance_percentiles"`
	CacheEffectiveness        *CacheEffectiveness     `json:"cache_effectiveness"`
}

// SymlinkResolutionError represents errors that can occur during symlink resolution.
type SymlinkResolutionError struct {
	Type       string                   `json:"type"`
	Message    string                   `json:"message"`
	Symlink    *valueobject.SymlinkInfo `json:"symlink,omitempty"`
	FilePath   string                   `json:"file_path,omitempty"`
	Timestamp  time.Time                `json:"timestamp"`
	StackTrace string                   `json:"stack_trace,omitempty"`
	Cause      error                    `json:"-"` // Original error
}

// Error implements the error interface.
func (e *SymlinkResolutionError) Error() string {
	if e.FilePath != "" {
		return fmt.Sprintf("%s: %s (file: %s)", e.Type, e.Message, e.FilePath)
	}
	if e.Symlink != nil {
		return fmt.Sprintf("%s: %s (symlink: %s)", e.Type, e.Message, e.Symlink.Path())
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause error.
func (e *SymlinkResolutionError) Unwrap() error {
	return e.Cause
}

// Common symlink resolution error types.
const (
	ErrorTypeSymlinkNotFound   = "SymlinkNotFound"
	ErrorTypeSymlinkBroken     = "SymlinkBroken"
	ErrorTypeSymlinkCircular   = "SymlinkCircular"
	ErrorTypeSymlinkDepthLimit = "SymlinkDepthLimit"
	ErrorTypeSymlinkPermission = "SymlinkPermission"
	ErrorTypeSymlinkSecurity   = "SymlinkSecurity"
	ErrorTypeSymlinkExternal   = "SymlinkExternal"
	ErrorTypeSymlinkFileSystem = "SymlinkFileSystem"
)

// Symlink resolution methods.
const (
	SymlinkResolutionMethodDirect    = "direct"
	SymlinkResolutionMethodRecursive = "recursive"
	SymlinkResolutionMethodCached    = "cached"
	SymlinkResolutionMethodSystem    = "system"
)

// Security risk levels.
const (
	SymlinkRiskLevelLow    = "low"
	SymlinkRiskLevelMedium = "medium"
	SymlinkRiskLevelHigh   = "high"
)

// Recommended actions for security violations.
const (
	SymlinkActionAllow  = "allow"
	SymlinkActionWarn   = "warn"
	SymlinkActionIgnore = "ignore"
	SymlinkActionBlock  = "block"
)

// Common helper functions and utilities.

// GetDefaultSymlinkResolutionConfig returns the default symlink resolution configuration.
func GetDefaultSymlinkResolutionConfig() *SymlinkResolutionConfig {
	return &SymlinkResolutionConfig{
		Policy:                valueobject.SymlinkResolutionPolicyResolve,
		MaxDepth:              10,
		ResolutionMethod:      SymlinkResolutionMethodRecursive,
		AllowExternalSymlinks: false,
		EnableSecurity:        true,
		EnableCaching:         true,
		CacheTTL:              10 * time.Minute,
		ProcessingTimeout:     15 * time.Second,
		EnableMetrics:         true,
		EnableTracing:         false,
		IgnorePatterns:        []string{},
	}
}

// SymlinkResolutionConfig configures symlink resolution behavior.
type SymlinkResolutionConfig struct {
	// Processing settings
	Policy            valueobject.SymlinkResolutionPolicy `json:"policy"`
	MaxDepth          int                                 `json:"max_depth"`
	ResolutionMethod  string                              `json:"resolution_method"`
	ProcessingTimeout time.Duration                       `json:"processing_timeout"`

	// Security settings
	AllowExternalSymlinks bool     `json:"allow_external_symlinks"`
	EnableSecurity        bool     `json:"enable_security"`
	SensitivePaths        []string `json:"sensitive_paths"`
	BlockedPaths          []string `json:"blocked_paths"`

	// Filtering settings
	IgnorePatterns    []string `json:"ignore_patterns"`
	IgnoredSymlinks   []string `json:"ignored_symlinks"`
	OnlyValidSymlinks bool     `json:"only_valid_symlinks"`

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
