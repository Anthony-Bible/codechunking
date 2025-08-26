package config

import (
	"codechunking/internal/domain/valueobject"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestNewGitConfig(t *testing.T) {
	tests := []struct {
		name           string
		configData     map[string]interface{}
		expectError    bool
		expectedConfig GitConfig
	}{
		{
			name: "should create default git config",
			configData: map[string]interface{}{
				"git": map[string]interface{}{},
			},
			expectError: false,
			expectedConfig: GitConfig{
				DefaultDepth:                1,
				ShallowCloneThresholdMB:     100,
				DefaultTimeout:              30 * time.Minute,
				MaxConcurrentClones:         3,
				RetryAttempts:               2,
				RetryBackoffDuration:        5 * time.Second,
				EnableProgressTracking:      true,
				EnablePerformanceMonitoring: true,
				AutoSelectStrategy:          true,
				WorkspaceCleanupEnabled:     true,
				WorkspaceCleanupInterval:    24 * time.Hour,
			},
		},
		{
			name: "should create git config with custom settings",
			configData: map[string]interface{}{
				"git": map[string]interface{}{
					"default_depth":                 5,
					"shallow_clone_threshold_mb":    50,
					"default_timeout":               "15m",
					"max_concurrent_clones":         5,
					"retry_attempts":                3,
					"retry_backoff_duration":        "10s",
					"enable_progress_tracking":      false,
					"enable_performance_monitoring": false,
					"auto_select_strategy":          false,
					"workspace_cleanup_enabled":     false,
					"workspace_cleanup_interval":    "48h",
				},
			},
			expectError: false,
			expectedConfig: GitConfig{
				DefaultDepth:                5,
				ShallowCloneThresholdMB:     50,
				DefaultTimeout:              15 * time.Minute,
				MaxConcurrentClones:         5,
				RetryAttempts:               3,
				RetryBackoffDuration:        10 * time.Second,
				EnableProgressTracking:      false,
				EnablePerformanceMonitoring: false,
				AutoSelectStrategy:          false,
				WorkspaceCleanupEnabled:     false,
				WorkspaceCleanupInterval:    48 * time.Hour,
			},
		},
		{
			name: "should reject invalid depth",
			configData: map[string]interface{}{
				"git": map[string]interface{}{
					"default_depth": -1,
				},
			},
			expectError: true,
		},
		{
			name: "should reject invalid threshold",
			configData: map[string]interface{}{
				"git": map[string]interface{}{
					"shallow_clone_threshold_mb": -10,
				},
			},
			expectError: true,
		},
		{
			name: "should reject invalid timeout",
			configData: map[string]interface{}{
				"git": map[string]interface{}{
					"default_timeout": "invalid",
				},
			},
			expectError: true,
		},
		{
			name: "should reject zero concurrent clones",
			configData: map[string]interface{}{
				"git": map[string]interface{}{
					"max_concurrent_clones": 0,
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up viper with test data
			v := viper.New()
			for key, value := range tt.configData {
				v.Set(key, value)
			}

			config, err := NewGitConfig(v)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if config != nil {
					t.Errorf("expected nil config but got %v", config)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Errorf("expected GitConfig but got nil")
				return
			}

			// Validate configuration values
			if config.DefaultDepth != tt.expectedConfig.DefaultDepth {
				t.Errorf("expected default depth %d but got %d", tt.expectedConfig.DefaultDepth, config.DefaultDepth)
			}
			if config.ShallowCloneThresholdMB != tt.expectedConfig.ShallowCloneThresholdMB {
				t.Errorf(
					"expected threshold %d but got %d",
					tt.expectedConfig.ShallowCloneThresholdMB,
					config.ShallowCloneThresholdMB,
				)
			}
			if config.DefaultTimeout != tt.expectedConfig.DefaultTimeout {
				t.Errorf("expected timeout %v but got %v", tt.expectedConfig.DefaultTimeout, config.DefaultTimeout)
			}
			if config.MaxConcurrentClones != tt.expectedConfig.MaxConcurrentClones {
				t.Errorf(
					"expected max concurrent %d but got %d",
					tt.expectedConfig.MaxConcurrentClones,
					config.MaxConcurrentClones,
				)
			}
		})
	}
}

func TestGitConfig_GetDefaultCloneOptions(t *testing.T) {
	tests := []struct {
		name         string
		config       GitConfig
		repositoryMB int64
		want         valueobject.CloneOptions
	}{
		{
			name: "should return shallow clone options for large repository",
			config: GitConfig{
				DefaultDepth:            1,
				ShallowCloneThresholdMB: 100,
				DefaultTimeout:          30 * time.Minute,
				AutoSelectStrategy:      true,
			},
			repositoryMB: 150, // Above threshold
			want: func() valueobject.CloneOptions {
				opts := valueobject.NewShallowCloneOptions(1, "")
				// Multiplier = 150/100 = 1.5, so timeout = 30min * 1.5 = 45min
				optsWithTimeout, _ := opts.WithTimeout(45 * time.Minute)
				return optsWithTimeout
			}(),
		},
		{
			name: "should return full clone options for small repository",
			config: GitConfig{
				DefaultDepth:            1,
				ShallowCloneThresholdMB: 100,
				DefaultTimeout:          30 * time.Minute,
				AutoSelectStrategy:      true,
			},
			repositoryMB: 50, // Below threshold
			want:         valueobject.NewFullCloneOptions(),
		},
		{
			name: "should return shallow clone when auto-select is disabled",
			config: GitConfig{
				DefaultDepth:            5,
				ShallowCloneThresholdMB: 100,
				DefaultTimeout:          15 * time.Minute,
				AutoSelectStrategy:      false, // Always use shallow
			},
			repositoryMB: 50, // Below threshold but auto-select disabled
			want: func() valueobject.CloneOptions {
				opts := valueobject.NewShallowCloneOptions(5, "")
				// Multiplier = max(50/100, 1.0) = 1.0, so timeout = 15min * 1.0 = 15min
				optsWithTimeout, _ := opts.WithTimeout(15 * time.Minute)
				return optsWithTimeout
			}(),
		},
		{
			name: "should apply custom timeout to clone options",
			config: GitConfig{
				DefaultDepth:            1,
				ShallowCloneThresholdMB: 100,
				DefaultTimeout:          10 * time.Minute,
				AutoSelectStrategy:      true,
			},
			repositoryMB: 150,
			want: func() valueobject.CloneOptions {
				opts := valueobject.NewShallowCloneOptions(1, "")
				// Multiplier = 150/100 = 1.5, so timeout = 10min * 1.5 = 15min
				optsWithTimeout, _ := opts.WithTimeout(15 * time.Minute)
				return optsWithTimeout
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetDefaultCloneOptions(tt.repositoryMB)

			if !got.Equal(tt.want) {
				t.Errorf("GetDefaultCloneOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitConfig_ShouldUseShallowClone(t *testing.T) {
	tests := []struct {
		name         string
		config       GitConfig
		repositoryMB int64
		want         bool
	}{
		{
			name: "should use shallow clone for large repository with auto-select",
			config: GitConfig{
				ShallowCloneThresholdMB: 100,
				AutoSelectStrategy:      true,
			},
			repositoryMB: 150,
			want:         true,
		},
		{
			name: "should not use shallow clone for small repository with auto-select",
			config: GitConfig{
				ShallowCloneThresholdMB: 100,
				AutoSelectStrategy:      true,
			},
			repositoryMB: 50,
			want:         false,
		},
		{
			name: "should always use shallow clone when auto-select disabled",
			config: GitConfig{
				ShallowCloneThresholdMB: 100,
				AutoSelectStrategy:      false,
			},
			repositoryMB: 50, // Below threshold
			want:         true,
		},
		{
			name: "should handle edge case at threshold",
			config: GitConfig{
				ShallowCloneThresholdMB: 100,
				AutoSelectStrategy:      true,
			},
			repositoryMB: 100,   // Exactly at threshold
			want:         false, // Should use full clone
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ShouldUseShallowClone(tt.repositoryMB)

			if got != tt.want {
				t.Errorf("ShouldUseShallowClone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitConfig_GetCloneTimeout(t *testing.T) {
	tests := []struct {
		name         string
		config       GitConfig
		repositoryMB int64
		want         time.Duration
	}{
		{
			name: "should return default timeout for small repository",
			config: GitConfig{
				DefaultTimeout:          30 * time.Minute,
				ShallowCloneThresholdMB: 100,
			},
			repositoryMB: 50,
			want:         30 * time.Minute,
		},
		{
			name: "should return extended timeout for large repository",
			config: GitConfig{
				DefaultTimeout:          30 * time.Minute,
				ShallowCloneThresholdMB: 100,
			},
			repositoryMB: 500,               // 5x threshold
			want:         150 * time.Minute, // 5x default timeout
		},
		{
			name: "should cap timeout at maximum",
			config: GitConfig{
				DefaultTimeout:          30 * time.Minute,
				ShallowCloneThresholdMB: 100,
			},
			repositoryMB: 2000,              // Very large
			want:         180 * time.Minute, // Capped at 3 hours
		},
		{
			name: "should handle zero repository size",
			config: GitConfig{
				DefaultTimeout:          30 * time.Minute,
				ShallowCloneThresholdMB: 100,
			},
			repositoryMB: 0,
			want:         30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetCloneTimeout(tt.repositoryMB)

			if got != tt.want {
				t.Errorf("GetCloneTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitConfig_GetRetryConfig(t *testing.T) {
	config := GitConfig{
		RetryAttempts:        3,
		RetryBackoffDuration: 5 * time.Second,
	}

	retryConfig := config.GetRetryConfig()

	if retryConfig.MaxAttempts != 3 {
		t.Errorf("expected max attempts 3 but got %d", retryConfig.MaxAttempts)
	}
	if retryConfig.BackoffDuration != 5*time.Second {
		t.Errorf("expected backoff duration 5s but got %v", retryConfig.BackoffDuration)
	}
	if retryConfig.BackoffMultiplier != 2.0 {
		t.Errorf("expected backoff multiplier 2.0 but got %f", retryConfig.BackoffMultiplier)
	}
}

func TestGitConfig_IsPerformanceMonitoringEnabled(t *testing.T) {
	tests := []struct {
		name   string
		config GitConfig
		want   bool
	}{
		{
			name: "should return true when monitoring enabled",
			config: GitConfig{
				EnablePerformanceMonitoring: true,
			},
			want: true,
		},
		{
			name: "should return false when monitoring disabled",
			config: GitConfig{
				EnablePerformanceMonitoring: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.IsPerformanceMonitoringEnabled()

			if got != tt.want {
				t.Errorf("IsPerformanceMonitoringEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitConfig_IsProgressTrackingEnabled(t *testing.T) {
	tests := []struct {
		name   string
		config GitConfig
		want   bool
	}{
		{
			name: "should return true when progress tracking enabled",
			config: GitConfig{
				EnableProgressTracking: true,
			},
			want: true,
		},
		{
			name: "should return false when progress tracking disabled",
			config: GitConfig{
				EnableProgressTracking: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.IsProgressTrackingEnabled()

			if got != tt.want {
				t.Errorf("IsProgressTrackingEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitConfig_GetWorkspaceConfig(t *testing.T) {
	config := GitConfig{
		WorkspaceCleanupEnabled:  true,
		WorkspaceCleanupInterval: 24 * time.Hour,
	}

	workspaceConfig := config.GetWorkspaceConfig()

	if !workspaceConfig.CleanupEnabled {
		t.Errorf("expected cleanup enabled but got false")
	}
	if workspaceConfig.CleanupInterval != 24*time.Hour {
		t.Errorf("expected cleanup interval 24h but got %v", workspaceConfig.CleanupInterval)
	}
}

func TestGitConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      GitConfig
		expectError bool
	}{
		{
			name: "should validate valid config",
			config: GitConfig{
				DefaultDepth:             1,
				ShallowCloneThresholdMB:  100,
				DefaultTimeout:           30 * time.Minute,
				MaxConcurrentClones:      3,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			expectError: false,
		},
		{
			name: "should reject negative default depth",
			config: GitConfig{
				DefaultDepth:             -1,
				ShallowCloneThresholdMB:  100,
				DefaultTimeout:           30 * time.Minute,
				MaxConcurrentClones:      3,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			expectError: true,
		},
		{
			name: "should reject negative threshold",
			config: GitConfig{
				DefaultDepth:             1,
				ShallowCloneThresholdMB:  -100,
				DefaultTimeout:           30 * time.Minute,
				MaxConcurrentClones:      3,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			expectError: true,
		},
		{
			name: "should reject zero timeout",
			config: GitConfig{
				DefaultDepth:             1,
				ShallowCloneThresholdMB:  100,
				DefaultTimeout:           0,
				MaxConcurrentClones:      3,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			expectError: true,
		},
		{
			name: "should reject zero max concurrent clones",
			config: GitConfig{
				DefaultDepth:             1,
				ShallowCloneThresholdMB:  100,
				DefaultTimeout:           30 * time.Minute,
				MaxConcurrentClones:      0,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGitConfig_Integration_WithViper(t *testing.T) {
	// This test will fail until proper Viper integration is implemented
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("test-config")

	// Set test configuration
	configYAML := `
git:
  default_depth: 1
  shallow_clone_threshold_mb: 50
  default_timeout: "20m"
  max_concurrent_clones: 4
  retry_attempts: 3
  retry_backoff_duration: "10s"
  enable_progress_tracking: true
  enable_performance_monitoring: true
  auto_select_strategy: true
  workspace_cleanup_enabled: true
  workspace_cleanup_interval: "12h"
`

	// This should fail until GitConfig is properly implemented
	config, err := ParseGitConfigFromYAML(configYAML)
	if err != nil {
		t.Errorf("ParseGitConfigFromYAML not implemented: %v", err)
	}
	if config == nil {
		t.Errorf("expected GitConfig but got nil")
	}
}
