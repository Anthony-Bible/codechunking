package config

import (
	"codechunking/internal/domain/valueobject"
	"testing"
	"time"
)

// TestGitConfig_ShallowCloneDecisionMaking tests automatic shallow clone decisions.
func TestGitConfig_ShallowCloneDecisionMaking(t *testing.T) {
	// This test will fail until shallow clone decision logic is implemented
	tests := []struct {
		name                      string
		repositorySizeMB          int64
		shallowCloneThresholdMB   int64
		autoSelectStrategy        bool
		expectedUseShallowClone   bool
		expectedRecommendedDepth  int
		expectedTimeoutMultiplier float64
	}{
		{
			name:                      "small repo should use full clone by default",
			repositorySizeMB:          50,
			shallowCloneThresholdMB:   100,
			autoSelectStrategy:        true,
			expectedUseShallowClone:   false,
			expectedRecommendedDepth:  0,
			expectedTimeoutMultiplier: 1.0,
		},
		{
			name:                      "large repo should use shallow clone",
			repositorySizeMB:          250,
			shallowCloneThresholdMB:   100,
			autoSelectStrategy:        true,
			expectedUseShallowClone:   true,
			expectedRecommendedDepth:  1,
			expectedTimeoutMultiplier: 2.5,
		},
		{
			name:                      "very large repo should use shallow clone with adjusted timeout",
			repositorySizeMB:          1000,
			shallowCloneThresholdMB:   100,
			autoSelectStrategy:        true,
			expectedUseShallowClone:   true,
			expectedRecommendedDepth:  1,
			expectedTimeoutMultiplier: 10.0,
		},
		{
			name:                      "auto-select disabled should always use shallow",
			repositorySizeMB:          10,
			shallowCloneThresholdMB:   100,
			autoSelectStrategy:        false,
			expectedUseShallowClone:   true,
			expectedRecommendedDepth:  1,
			expectedTimeoutMultiplier: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			config := &GitConfig{
				DefaultDepth:                DefaultDepthValue,
				ShallowCloneThresholdMB:     tt.shallowCloneThresholdMB,
				DefaultTimeout:              DefaultTimeoutMinutes * time.Minute,
				MaxConcurrentClones:         DefaultMaxConcurrentClones,
				RetryAttempts:               DefaultRetryAttempts,
				RetryBackoffDuration:        DefaultRetryBackoffSeconds * time.Second,
				EnableProgressTracking:      true,
				EnablePerformanceMonitoring: true,
				AutoSelectStrategy:          tt.autoSelectStrategy,
				WorkspaceCleanupEnabled:     true,
				WorkspaceCleanupInterval:    DefaultWorkspaceCleanupHours * time.Hour,
			}

			// Test shallow clone decision
			shouldUseShallow := config.ShouldUseShallowClone(tt.repositorySizeMB)
			if shouldUseShallow != tt.expectedUseShallowClone {
				t.Errorf("expected shallow clone decision %t, got %t",
					tt.expectedUseShallowClone, shouldUseShallow)
			}

			// Test default clone options
			opts := config.GetDefaultCloneOptions(tt.repositorySizeMB)
			if opts.IsShallowClone() != tt.expectedUseShallowClone {
				t.Errorf("expected clone options shallow %t, got %t",
					tt.expectedUseShallowClone, opts.IsShallowClone())
			}

			if tt.expectedUseShallowClone {
				if opts.Depth() != tt.expectedRecommendedDepth {
					t.Errorf("expected depth %d, got %d",
						tt.expectedRecommendedDepth, opts.Depth())
				}
			}

			// Test timeout calculation
			calculatedTimeout := config.GetCloneTimeout(tt.repositorySizeMB)
			expectedTimeout := time.Duration(float64(config.DefaultTimeout) * tt.expectedTimeoutMultiplier)
			maxTimeout := 180 * time.Minute
			if expectedTimeout > maxTimeout {
				expectedTimeout = maxTimeout
			}

			if calculatedTimeout != expectedTimeout {
				t.Errorf("expected timeout %v, got %v", expectedTimeout, calculatedTimeout)
			}
		})
	}
}

// TestGitConfig_ShallowCloneConfigurationValidation tests configuration validation.
func TestGitConfig_ShallowCloneConfigurationValidation(t *testing.T) {
	// This test will fail until comprehensive validation is implemented
	tests := []struct {
		name        string
		config      GitConfig
		shouldFail  bool
		errorSubstr string
	}{
		{
			name: "valid configuration should pass",
			config: GitConfig{
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
			shouldFail: false,
		},
		{
			name: "negative default depth should fail",
			config: GitConfig{
				DefaultDepth:             -1,
				ShallowCloneThresholdMB:  100,
				DefaultTimeout:           30 * time.Minute,
				MaxConcurrentClones:      3,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			shouldFail:  true,
			errorSubstr: "default_depth cannot be negative",
		},
		{
			name: "negative threshold should fail",
			config: GitConfig{
				DefaultDepth:             1,
				ShallowCloneThresholdMB:  -100,
				DefaultTimeout:           30 * time.Minute,
				MaxConcurrentClones:      3,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			shouldFail:  true,
			errorSubstr: "shallow_clone_threshold_mb cannot be negative",
		},
		{
			name: "zero timeout should fail",
			config: GitConfig{
				DefaultDepth:             1,
				ShallowCloneThresholdMB:  100,
				DefaultTimeout:           0,
				MaxConcurrentClones:      3,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			shouldFail:  true,
			errorSubstr: "default_timeout must be positive",
		},
		{
			name: "zero concurrent clones should fail",
			config: GitConfig{
				DefaultDepth:             1,
				ShallowCloneThresholdMB:  100,
				DefaultTimeout:           30 * time.Minute,
				MaxConcurrentClones:      0,
				RetryAttempts:            2,
				RetryBackoffDuration:     5 * time.Second,
				WorkspaceCleanupInterval: 24 * time.Hour,
			},
			shouldFail:  true,
			errorSubstr: "max_concurrent_clones must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.shouldFail {
				if err == nil {
					t.Errorf("expected validation to fail")
					return
				}
				if tt.errorSubstr != "" && !containsString(err.Error(), tt.errorSubstr) {
					t.Errorf("expected error to contain %q, got: %v", tt.errorSubstr, err)
				}
			} else if err != nil {
				t.Errorf("expected validation to pass, got: %v", err)
			}
		})
	}
}

// TestGitConfig_ShallowCloneYAMLConfiguration tests YAML configuration parsing.
func TestGitConfig_ShallowCloneYAMLConfiguration(t *testing.T) {
	// This test will fail until YAML parsing handles all shallow clone options
	tests := []struct {
		name           string
		yamlConfig     string
		expectedConfig GitConfig
		shouldFail     bool
	}{
		{
			name: "complete shallow clone configuration",
			yamlConfig: `
git:
  default_depth: 5
  shallow_clone_threshold_mb: 200
  default_timeout: "45m"
  max_concurrent_clones: 5
  retry_attempts: 3
  retry_backoff_duration: "10s"
  enable_progress_tracking: true
  enable_performance_monitoring: true
  auto_select_strategy: false
  workspace_cleanup_enabled: true
  workspace_cleanup_interval: "48h"`,
			expectedConfig: GitConfig{
				DefaultDepth:                5,
				ShallowCloneThresholdMB:     200,
				DefaultTimeout:              45 * time.Minute,
				MaxConcurrentClones:         5,
				RetryAttempts:               3,
				RetryBackoffDuration:        10 * time.Second,
				EnableProgressTracking:      true,
				EnablePerformanceMonitoring: true,
				AutoSelectStrategy:          false,
				WorkspaceCleanupEnabled:     true,
				WorkspaceCleanupInterval:    48 * time.Hour,
			},
		},
		{
			name: "minimal shallow clone configuration with defaults",
			yamlConfig: `
git:
  shallow_clone_threshold_mb: 150`,
			expectedConfig: GitConfig{
				DefaultDepth:                DefaultDepthValue,
				ShallowCloneThresholdMB:     150,
				DefaultTimeout:              DefaultTimeoutMinutes * time.Minute,
				MaxConcurrentClones:         DefaultMaxConcurrentClones,
				RetryAttempts:               DefaultRetryAttempts,
				RetryBackoffDuration:        DefaultRetryBackoffSeconds * time.Second,
				EnableProgressTracking:      true,
				EnablePerformanceMonitoring: true,
				AutoSelectStrategy:          true,
				WorkspaceCleanupEnabled:     true,
				WorkspaceCleanupInterval:    DefaultWorkspaceCleanupHours * time.Hour,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseGitConfigFromYAML(tt.yamlConfig)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("expected YAML parsing to fail")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected YAML parsing error: %v", err)
			}

			// Validate parsed configuration matches expected
			validateGitConfigFields(t, config, tt.expectedConfig)
		})
	}
}

// TestGitConfig_ShallowClonePerformanceSettings tests performance-related settings.
func TestGitConfig_ShallowClonePerformanceSettings(t *testing.T) {
	// This test will fail until performance configuration is fully implemented
	t.Run("should provide performance monitoring configuration", func(t *testing.T) {
		config := &GitConfig{
			EnableProgressTracking:      true,
			EnablePerformanceMonitoring: false,
			MaxConcurrentClones:         10,
		}

		if !config.IsProgressTrackingEnabled() {
			t.Errorf("expected progress tracking to be enabled")
		}

		if config.IsPerformanceMonitoringEnabled() {
			t.Errorf("expected performance monitoring to be disabled")
		}

		// Test retry configuration
		retryConfig := config.GetRetryConfig()
		if retryConfig.MaxAttempts != config.RetryAttempts {
			t.Errorf("expected retry attempts %d, got %d",
				config.RetryAttempts, retryConfig.MaxAttempts)
		}
		if retryConfig.BackoffMultiplier != ExponentialBackoffMultiplier {
			t.Errorf("expected backoff multiplier %f, got %f",
				ExponentialBackoffMultiplier, retryConfig.BackoffMultiplier)
		}

		// Test workspace configuration
		workspaceConfig := config.GetWorkspaceConfig()
		if workspaceConfig.CleanupEnabled != config.WorkspaceCleanupEnabled {
			t.Errorf("expected cleanup enabled %t, got %t",
				config.WorkspaceCleanupEnabled, workspaceConfig.CleanupEnabled)
		}
	})
}

// TestGitConfig_ShallowCloneIntegrationWithCloneOptions tests integration with clone options.
func TestGitConfig_ShallowCloneIntegrationWithCloneOptions(t *testing.T) {
	// This test will fail until full integration is implemented
	t.Run("should create appropriate clone options for different scenarios", func(t *testing.T) {
		config := &GitConfig{
			DefaultDepth:            1,
			ShallowCloneThresholdMB: 100,
			DefaultTimeout:          30 * time.Minute,
			AutoSelectStrategy:      true,
		}

		scenarios := []integrationTestScenario{
			{"tiny repo", 10, false, 0, 30 * time.Minute},
			{"medium repo", 150, true, 1, 45 * time.Minute},
			{"large repo", 500, true, 1, 150 * time.Minute},
			{"huge repo", 2000, true, 1, 180 * time.Minute}, // Capped at max
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				opts := config.GetDefaultCloneOptions(scenario.repositorySizeMB)
				validateCloneOptionsBasics(t, opts, scenario)
				validateGitArgsForShallowClone(t, opts.ToGitArgs(), scenario.expectedShallow)
			})
		}
	})
}

// Helper functions

type integrationTestScenario struct {
	name               string
	repositorySizeMB   int64
	expectedShallow    bool
	expectedDepth      int
	expectedMinTimeout time.Duration
}

func validateCloneOptionsBasics(t *testing.T, opts valueobject.CloneOptions, scenario integrationTestScenario) {
	t.Helper()
	if opts.IsShallowClone() != scenario.expectedShallow {
		t.Errorf("expected shallow %t, got %t", scenario.expectedShallow, opts.IsShallowClone())
	}
	if opts.Depth() != scenario.expectedDepth {
		t.Errorf("expected depth %d, got %d", scenario.expectedDepth, opts.Depth())
	}
	if opts.Timeout() < scenario.expectedMinTimeout {
		t.Errorf("expected timeout >= %v, got %v", scenario.expectedMinTimeout, opts.Timeout())
	}
}

func validateGitArgsForShallowClone(t *testing.T, gitArgs []string, expectedShallow bool) {
	t.Helper()
	if expectedShallow {
		if !containsString(gitArgs, "--depth=1") {
			t.Errorf("expected git args to contain --depth=1, got: %v", gitArgs)
		}
		if !containsString(gitArgs, "--single-branch") {
			t.Errorf("expected git args to contain --single-branch, got: %v", gitArgs)
		}
	} else if containsString(gitArgs, "--depth") {
		t.Errorf("expected git args not to contain --depth, got: %v", gitArgs)
	}
}

func validateGitConfigFields(t *testing.T, actual *GitConfig, expected GitConfig) {
	t.Helper()

	if actual.DefaultDepth != expected.DefaultDepth {
		t.Errorf("expected DefaultDepth %d, got %d", expected.DefaultDepth, actual.DefaultDepth)
	}
	if actual.ShallowCloneThresholdMB != expected.ShallowCloneThresholdMB {
		t.Errorf("expected ShallowCloneThresholdMB %d, got %d",
			expected.ShallowCloneThresholdMB, actual.ShallowCloneThresholdMB)
	}
	if actual.DefaultTimeout != expected.DefaultTimeout {
		t.Errorf("expected DefaultTimeout %v, got %v", expected.DefaultTimeout, actual.DefaultTimeout)
	}
	if actual.AutoSelectStrategy != expected.AutoSelectStrategy {
		t.Errorf("expected AutoSelectStrategy %t, got %t",
			expected.AutoSelectStrategy, actual.AutoSelectStrategy)
	}
}

func containsString(slice interface{}, target string) bool {
	switch s := slice.(type) {
	case []string:
		for _, item := range s {
			if item == target {
				return true
			}
		}
	case string:
		return len(s) > 0 && len(target) > 0 &&
			(s == target || (len(s) > len(target) && findSubstring(s, target)))
	}
	return false
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
