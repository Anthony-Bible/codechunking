package config

import (
	"codechunking/internal/domain/valueobject"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Default configuration constants.
const (
	DefaultDepthValue              = 1
	DefaultShallowCloneThresholdMB = 100
	DefaultTimeoutMinutes          = 30
	DefaultMaxConcurrentClones     = 3
	DefaultRetryAttempts           = 2
	DefaultRetryBackoffSeconds     = 5
	DefaultWorkspaceCleanupHours   = 24
	MaxTimeoutMinutes              = 180
	ExponentialBackoffMultiplier   = 2.0
)

// GitConfig holds configuration settings for git operations.
type GitConfig struct {
	DefaultDepth                int           `mapstructure:"default_depth"                 yaml:"default_depth"`
	ShallowCloneThresholdMB     int64         `mapstructure:"shallow_clone_threshold_mb"    yaml:"shallow_clone_threshold_mb"`
	DefaultTimeout              time.Duration `mapstructure:"default_timeout"               yaml:"default_timeout"`
	MaxConcurrentClones         int           `mapstructure:"max_concurrent_clones"         yaml:"max_concurrent_clones"`
	RetryAttempts               int           `mapstructure:"retry_attempts"                yaml:"retry_attempts"`
	RetryBackoffDuration        time.Duration `mapstructure:"retry_backoff_duration"        yaml:"retry_backoff_duration"`
	EnableProgressTracking      bool          `mapstructure:"enable_progress_tracking"      yaml:"enable_progress_tracking"`
	EnablePerformanceMonitoring bool          `mapstructure:"enable_performance_monitoring" yaml:"enable_performance_monitoring"`
	AutoSelectStrategy          bool          `mapstructure:"auto_select_strategy"          yaml:"auto_select_strategy"`
	WorkspaceCleanupEnabled     bool          `mapstructure:"workspace_cleanup_enabled"     yaml:"workspace_cleanup_enabled"`
	WorkspaceCleanupInterval    time.Duration `mapstructure:"workspace_cleanup_interval"    yaml:"workspace_cleanup_interval"`
}

// RetryConfig represents retry configuration for git operations.
type RetryConfig struct {
	MaxAttempts       int           `json:"max_attempts"`
	BackoffDuration   time.Duration `json:"backoff_duration"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
}

// WorkspaceConfig represents workspace management configuration.
type WorkspaceConfig struct {
	CleanupEnabled  bool          `json:"cleanup_enabled"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// NewGitConfig creates a new GitConfig from viper configuration.
func NewGitConfig(v *viper.Viper) (*GitConfig, error) {
	config := &GitConfig{
		// Set defaults
		DefaultDepth:                DefaultDepthValue,
		ShallowCloneThresholdMB:     DefaultShallowCloneThresholdMB,
		DefaultTimeout:              DefaultTimeoutMinutes * time.Minute,
		MaxConcurrentClones:         DefaultMaxConcurrentClones,
		RetryAttempts:               DefaultRetryAttempts,
		RetryBackoffDuration:        DefaultRetryBackoffSeconds * time.Second,
		EnableProgressTracking:      true,
		EnablePerformanceMonitoring: true,
		AutoSelectStrategy:          true,
		WorkspaceCleanupEnabled:     true,
		WorkspaceCleanupInterval:    DefaultWorkspaceCleanupHours * time.Hour,
	}

	// Unmarshal git configuration from viper
	if v.IsSet("git") {
		if err := v.UnmarshalKey("git", config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal git config: %w", err)
		}
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid git configuration: %w", err)
	}

	return config, nil
}

// ParseGitConfigFromYAML parses git configuration from YAML string.
func ParseGitConfigFromYAML(yamlContent string) (*GitConfig, error) {
	var configData map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &configData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	v := viper.New()
	for key, value := range configData {
		v.Set(key, value)
	}

	return NewGitConfig(v)
}

// GetDefaultCloneOptions returns default clone options based on repository size.
func (c *GitConfig) GetDefaultCloneOptions(repositoryMB int64) valueobject.CloneOptions {
	var opts valueobject.CloneOptions

	if c.ShouldUseShallowClone(repositoryMB) {
		opts = valueobject.NewShallowCloneOptions(c.DefaultDepth, "")
	} else {
		opts = valueobject.NewFullCloneOptions()
	}

	// Apply timeout
	timeout := c.GetCloneTimeout(repositoryMB)
	optsWithTimeout, err := opts.WithTimeout(timeout)
	if err != nil {
		// Fallback to original options if timeout setting fails
		return opts
	}

	return optsWithTimeout
}

// ShouldUseShallowClone determines if shallow clone should be used based on repository size.
func (c *GitConfig) ShouldUseShallowClone(repositoryMB int64) bool {
	if !c.AutoSelectStrategy {
		return true // Always use shallow clone when auto-select is disabled
	}

	return repositoryMB > c.ShallowCloneThresholdMB
}

// GetCloneTimeout calculates appropriate timeout based on repository size.
func (c *GitConfig) GetCloneTimeout(repositoryMB int64) time.Duration {
	if repositoryMB == 0 {
		return c.DefaultTimeout
	}

	// Calculate timeout multiplier based on size relative to threshold
	multiplier := float64(repositoryMB) / float64(c.ShallowCloneThresholdMB)
	if multiplier < 1.0 {
		multiplier = 1.0
	}

	timeout := time.Duration(float64(c.DefaultTimeout) * multiplier)

	// Cap at 3 hours maximum
	maxTimeout := 180 * time.Minute
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	return timeout
}

// GetRetryConfig returns the retry configuration.
func (c *GitConfig) GetRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       c.RetryAttempts,
		BackoffDuration:   c.RetryBackoffDuration,
		BackoffMultiplier: 2.0, // Fixed exponential backoff
	}
}

// IsPerformanceMonitoringEnabled returns whether performance monitoring is enabled.
func (c *GitConfig) IsPerformanceMonitoringEnabled() bool {
	return c.EnablePerformanceMonitoring
}

// IsProgressTrackingEnabled returns whether progress tracking is enabled.
func (c *GitConfig) IsProgressTrackingEnabled() bool {
	return c.EnableProgressTracking
}

// GetWorkspaceConfig returns the workspace configuration.
func (c *GitConfig) GetWorkspaceConfig() WorkspaceConfig {
	return WorkspaceConfig{
		CleanupEnabled:  c.WorkspaceCleanupEnabled,
		CleanupInterval: c.WorkspaceCleanupInterval,
	}
}

// Validate validates the git configuration.
func (c *GitConfig) Validate() error {
	var errors []string

	if c.DefaultDepth < 0 {
		errors = append(errors, "default_depth cannot be negative")
	}

	if c.ShallowCloneThresholdMB < 0 {
		errors = append(errors, "shallow_clone_threshold_mb cannot be negative")
	}

	if c.DefaultTimeout <= 0 {
		errors = append(errors, "default_timeout must be positive")
	}

	if c.MaxConcurrentClones <= 0 {
		errors = append(errors, "max_concurrent_clones must be positive")
	}

	if c.RetryAttempts < 0 {
		errors = append(errors, "retry_attempts cannot be negative")
	}

	if c.RetryBackoffDuration < 0 {
		errors = append(errors, "retry_backoff_duration cannot be negative")
	}

	if c.WorkspaceCleanupInterval <= 0 {
		errors = append(errors, "workspace_cleanup_interval must be positive")
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}
