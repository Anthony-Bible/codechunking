package service

import (
	"context"
	"testing"
	"time"
)

// DiskCleanupStrategyService interface for managing cleanup strategies.
type DiskCleanupStrategyService interface {
	// Strategy execution
	ExecuteLRUCleanup(ctx context.Context, config *LRUCleanupConfig) (*CleanupResult, error)
	ExecuteTTLCleanup(ctx context.Context, config *TTLCleanupConfig) (*CleanupResult, error)
	ExecuteSizeBasedCleanup(ctx context.Context, config *SizeBasedCleanupConfig) (*CleanupResult, error)

	// Strategy analysis
	AnalyzeLRUCandidates(ctx context.Context, path string, maxItems int) ([]*CleanupCandidate, error)
	AnalyzeTTLCandidates(ctx context.Context, path string, ttl time.Duration) ([]*CleanupCandidate, error)
	AnalyzeSizeCandidates(ctx context.Context, path string, targetSizeBytes int64) ([]*CleanupCandidate, error)

	// Strategy comparison and selection
	RecommendCleanupStrategy(ctx context.Context, path string, pressure *DiskPressure) (*StrategyRecommendation, error)
	CompareStrategies(ctx context.Context, path string, strategies []CleanupStrategyType) (*StrategyComparison, error)

	// Advanced cleanup operations
	ExecuteHybridCleanup(ctx context.Context, config *HybridCleanupConfig) (*CleanupResult, error)
	ExecuteSmartCleanup(ctx context.Context, config *SmartCleanupConfig) (*CleanupResult, error)
}

// ============================================================================
// FAILING TESTS FOR CLEANUP STRATEGIES
// ============================================================================

func TestDiskCleanupStrategyService_ExecuteLRUCleanup(t *testing.T) {
	tests := []struct {
		name           string
		config         *LRUCleanupConfig
		expectedResult *CleanupResult
		expectError    bool
	}{
		{
			name: "execute LRU cleanup with basic configuration",
			config: &LRUCleanupConfig{
				Path:              "/tmp/codechunking-cache",
				MaxItems:          100,
				MinFreeSpaceBytes: 10 * 1024 * 1024 * 1024, // 10GB
				DryRun:            false,
				PreservePriority:  true,
				AccessWeight:      0.4,
				AgeWeight:         0.3,
				SizeWeight:        0.3,
			},
			expectedResult: &CleanupResult{
				Strategy:        CleanupStrategyLRU,
				TotalCandidates: 150,
				CleanedItems:    50,
				BytesFreed:      5 * 1024 * 1024 * 1024, // 5GB
				DryRun:          false,
			},
			expectError: false,
		},
		{
			name: "execute LRU cleanup in dry run mode",
			config: &LRUCleanupConfig{
				Path:              "/tmp/codechunking-cache",
				MaxItems:          50,
				MinFreeSpaceBytes: 5 * 1024 * 1024 * 1024, // 5GB
				DryRun:            true,
			},
			expectedResult: &CleanupResult{
				Strategy:        CleanupStrategyLRU,
				TotalCandidates: 80,
				CleanedItems:    0, // No actual cleanup in dry run
				BytesFreed:      0,
				DryRun:          true,
			},
			expectError: false,
		},
		{
			name: "execute LRU cleanup with invalid configuration",
			config: &LRUCleanupConfig{
				Path:     "", // Invalid empty path
				MaxItems: -1, // Invalid negative max items
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testExecuteLRUCleanup(t, tt.config, tt.expectedResult, tt.expectError)
		})
	}
}

func testExecuteLRUCleanup(t *testing.T, config *LRUCleanupConfig, expected *CleanupResult, expectError bool) {
	// Create concrete implementation for GREEN phase
	service := NewDefaultDiskCleanupStrategyService()
	ctx := context.Background()

	result, err := service.ExecuteLRUCleanup(ctx, config)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateCleanupResult(t, result, expected)
}

func TestDiskCleanupStrategyService_ExecuteTTLCleanup(t *testing.T) {
	tests := []struct {
		name           string
		config         *TTLCleanupConfig
		expectedResult *CleanupResult
		expectError    bool
	}{
		{
			name: "execute TTL cleanup with 7-day expiration",
			config: &TTLCleanupConfig{
				Path:             "/tmp/codechunking-cache",
				MaxAge:           7 * 24 * time.Hour,
				MaxIdleTime:      3 * 24 * time.Hour,
				DryRun:           false,
				GracePeriod:      1 * time.Hour,
				PreservePinned:   true,
				CleanupBatchSize: 10,
				VerifyIntegrity:  true,
			},
			expectedResult: &CleanupResult{
				Strategy:        CleanupStrategyTTL,
				TotalCandidates: 120,
				CleanedItems:    75,
				BytesFreed:      8 * 1024 * 1024 * 1024, // 8GB
				DryRun:          false,
			},
			expectError: false,
		},
		{
			name: "execute TTL cleanup with aggressive expiration",
			config: &TTLCleanupConfig{
				Path:             "/tmp/codechunking-cache",
				MaxAge:           24 * time.Hour, // Very aggressive
				MaxIdleTime:      6 * time.Hour,
				DryRun:           false,
				PreservePinned:   false,
				CleanupBatchSize: 50,
			},
			expectedResult: &CleanupResult{
				Strategy:        CleanupStrategyTTL,
				TotalCandidates: 200,
				CleanedItems:    150,
				BytesFreed:      15 * 1024 * 1024 * 1024, // 15GB
			},
			expectError: false,
		},
		{
			name: "execute TTL cleanup with zero max age should error",
			config: &TTLCleanupConfig{
				Path:   "/tmp/codechunking-cache",
				MaxAge: 0, // Invalid
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testExecuteTTLCleanup(t, tt.config, tt.expectedResult, tt.expectError)
		})
	}
}

func testExecuteTTLCleanup(t *testing.T, config *TTLCleanupConfig, expected *CleanupResult, expectError bool) {
	service := NewDefaultDiskCleanupStrategyService()
	ctx := context.Background()

	result, err := service.ExecuteTTLCleanup(ctx, config)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateCleanupResult(t, result, expected)
}

func TestDiskCleanupStrategyService_ExecuteSizeBasedCleanup(t *testing.T) {
	tests := []struct {
		name           string
		config         *SizeBasedCleanupConfig
		expectedResult *CleanupResult
		expectError    bool
	}{
		{
			name: "execute size-based cleanup with largest first strategy",
			config: &SizeBasedCleanupConfig{
				Path:                 "/tmp/codechunking-cache",
				TargetSizeBytes:      50 * 1024 * 1024 * 1024,  // 50GB
				MaxSizeBytes:         100 * 1024 * 1024 * 1024, // 100GB
				DryRun:               false,
				Strategy:             "largest_first",
				PreserveCritical:     true,
				CompressionEnabled:   true,
				DeduplicationEnabled: true,
			},
			expectedResult: &CleanupResult{
				Strategy:        CleanupStrategySizeBased,
				TotalCandidates: 180,
				CleanedItems:    90,
				BytesFreed:      45 * 1024 * 1024 * 1024, // 45GB
				DryRun:          false,
			},
			expectError: false,
		},
		{
			name: "execute size-based cleanup with oldest first strategy",
			config: &SizeBasedCleanupConfig{
				Path:             "/tmp/codechunking-cache",
				TargetSizeBytes:  30 * 1024 * 1024 * 1024, // 30GB
				MaxSizeBytes:     80 * 1024 * 1024 * 1024, // 80GB
				Strategy:         "oldest_first",
				PreserveCritical: true,
			},
			expectedResult: &CleanupResult{
				Strategy:     CleanupStrategySizeBased,
				CleanedItems: 120,
				BytesFreed:   35 * 1024 * 1024 * 1024, // 35GB
			},
			expectError: false,
		},
		{
			name: "execute size-based cleanup with invalid target size",
			config: &SizeBasedCleanupConfig{
				Path:            "/tmp/codechunking-cache",
				TargetSizeBytes: -1, // Invalid negative size
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testExecuteSizeBasedCleanup(t, tt.config, tt.expectedResult, tt.expectError)
		})
	}
}

func testExecuteSizeBasedCleanup(
	t *testing.T,
	config *SizeBasedCleanupConfig,
	expected *CleanupResult,
	expectError bool,
) {
	service := NewDefaultDiskCleanupStrategyService()
	ctx := context.Background()

	result, err := service.ExecuteSizeBasedCleanup(ctx, config)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateCleanupResult(t, result, expected)
}

func TestDiskCleanupStrategyService_AnalyzeLRUCandidates(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		maxItems           int
		expectedCandidates int
		expectError        bool
	}{
		{
			name:               "analyze LRU candidates for cache directory",
			path:               "/tmp/codechunking-cache",
			maxItems:           100,
			expectedCandidates: 150, // More than maxItems to show cleanup needed
			expectError:        false,
		},
		{
			name:               "analyze LRU candidates with small max items",
			path:               "/tmp/codechunking-cache",
			maxItems:           10,
			expectedCandidates: 50, // Many candidates for cleanup
			expectError:        false,
		},
		{
			name:        "analyze LRU candidates for invalid path",
			path:        "/invalid/path",
			maxItems:    100,
			expectError: true,
		},
		{
			name:        "analyze LRU candidates with negative max items",
			path:        "/tmp/codechunking-cache",
			maxItems:    -1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAnalyzeLRUCandidates(t, tt.path, tt.maxItems, tt.expectedCandidates, tt.expectError)
		})
	}
}

func testAnalyzeLRUCandidates(t *testing.T, path string, maxItems, expectedCount int, expectError bool) {
	service := NewDefaultDiskCleanupStrategyService()
	ctx := context.Background()

	candidates, err := service.AnalyzeLRUCandidates(ctx, path, maxItems)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(candidates) != expectedCount {
		t.Errorf("Expected %d candidates, got %d", expectedCount, len(candidates))
	}

	validateCleanupCandidates(t, candidates)
}

func TestDiskCleanupStrategyService_RecommendCleanupStrategy(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		pressure           *DiskPressure
		expectedStrategy   CleanupStrategyType
		expectedConfidence float64
		expectError        bool
	}{
		{
			name: "recommend strategy for low disk pressure",
			path: "/tmp/codechunking-cache",
			pressure: &DiskPressure{
				CurrentUsagePercent: 60.0,
				FreeSpaceBytes:      40 * 1024 * 1024 * 1024, // 40GB
				GrowthRatePerDay:    1.5,
				TimeToFull:          30 * 24 * time.Hour,
				Severity:            PressureLow,
			},
			expectedStrategy:   CleanupStrategyTTL, // Gentle cleanup
			expectedConfidence: 0.8,
			expectError:        false,
		},
		{
			name: "recommend strategy for high disk pressure",
			path: "/tmp/codechunking-cache",
			pressure: &DiskPressure{
				CurrentUsagePercent: 85.0,
				FreeSpaceBytes:      5 * 1024 * 1024 * 1024, // 5GB
				GrowthRatePerDay:    5.0,
				TimeToFull:          2 * 24 * time.Hour,
				Severity:            PressureHigh,
			},
			expectedStrategy:   CleanupStrategySizeBased, // Aggressive cleanup
			expectedConfidence: 0.95,
			expectError:        false,
		},
		{
			name: "recommend strategy for critical disk pressure",
			path: "/tmp/codechunking-cache",
			pressure: &DiskPressure{
				CurrentUsagePercent: 95.0,
				FreeSpaceBytes:      1 * 1024 * 1024 * 1024, // 1GB
				GrowthRatePerDay:    10.0,
				TimeToFull:          6 * time.Hour,
				Severity:            PressureCritical,
			},
			expectedStrategy:   CleanupStrategySmart, // AI-optimized cleanup
			expectedConfidence: 0.98,
			expectError:        false,
		},
		{
			name:        "recommend strategy with invalid pressure data",
			path:        "/tmp/codechunking-cache",
			pressure:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRecommendCleanupStrategy(
				t,
				tt.path,
				tt.pressure,
				tt.expectedStrategy,
				tt.expectedConfidence,
				tt.expectError,
			)
		})
	}
}

func testRecommendCleanupStrategy(
	t *testing.T,
	path string,
	pressure *DiskPressure,
	expectedStrategy CleanupStrategyType,
	expectedConfidence float64,
	expectError bool,
) {
	service := NewDefaultDiskCleanupStrategyService()
	ctx := context.Background()

	recommendation, err := service.RecommendCleanupStrategy(ctx, path, pressure)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateStrategyRecommendation(t, recommendation, expectedStrategy, expectedConfidence)
}

func TestDiskCleanupStrategyService_ExecuteHybridCleanup(t *testing.T) {
	tests := []struct {
		name           string
		config         *HybridCleanupConfig
		expectedResult *CleanupResult
		expectError    bool
	}{
		{
			name: "execute hybrid cleanup combining LRU and TTL strategies",
			config: &HybridCleanupConfig{
				Path: "/tmp/codechunking-cache",
				LRUConfig: &LRUCleanupConfig{
					MaxItems:     50,
					AccessWeight: 0.6,
					AgeWeight:    0.4,
				},
				TTLConfig: &TTLCleanupConfig{
					MaxAge:         7 * 24 * time.Hour,
					MaxIdleTime:    2 * 24 * time.Hour,
					PreservePinned: true,
				},
				StrategyWeights: map[CleanupStrategyType]float64{
					CleanupStrategyLRU:       0.6,
					CleanupStrategyTTL:       0.4,
					CleanupStrategySizeBased: 0.0,
					CleanupStrategyHybrid:    0.0,
					CleanupStrategySmart:     0.0,
				},
				DryRun: false,
			},
			expectedResult: &CleanupResult{
				Strategy:        CleanupStrategyHybrid,
				TotalCandidates: 200,
				CleanedItems:    85,
				BytesFreed:      12 * 1024 * 1024 * 1024, // 12GB
				DryRun:          false,
			},
			expectError: false,
		},
		{
			name: "execute hybrid cleanup with all three strategies",
			config: &HybridCleanupConfig{
				Path: "/tmp/codechunking-cache",
				LRUConfig: &LRUCleanupConfig{
					MaxItems: 30,
				},
				TTLConfig: &TTLCleanupConfig{
					MaxAge: 5 * 24 * time.Hour,
				},
				SizeConfig: &SizeBasedCleanupConfig{
					TargetSizeBytes: 20 * 1024 * 1024 * 1024, // 20GB
					Strategy:        "largest_first",
				},
				StrategyWeights: map[CleanupStrategyType]float64{
					CleanupStrategyLRU:       0.4,
					CleanupStrategyTTL:       0.3,
					CleanupStrategySizeBased: 0.3,
					CleanupStrategyHybrid:    0.0,
					CleanupStrategySmart:     0.0,
				},
			},
			expectedResult: &CleanupResult{
				Strategy:     CleanupStrategyHybrid,
				CleanedItems: 120,
				BytesFreed:   18 * 1024 * 1024 * 1024, // 18GB
			},
			expectError: false,
		},
		{
			name: "execute hybrid cleanup with invalid configuration",
			config: &HybridCleanupConfig{
				Path:            "",  // Invalid empty path
				StrategyWeights: nil, // No strategies defined
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testExecuteHybridCleanup(t, tt.config, tt.expectedResult, tt.expectError)
		})
	}
}

func testExecuteHybridCleanup(t *testing.T, config *HybridCleanupConfig, expected *CleanupResult, expectError bool) {
	service := NewDefaultDiskCleanupStrategyService()
	ctx := context.Background()

	result, err := service.ExecuteHybridCleanup(ctx, config)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateCleanupResult(t, result, expected)
}

func TestDiskCleanupStrategyService_ExecuteSmartCleanup(t *testing.T) {
	tests := []struct {
		name           string
		config         *SmartCleanupConfig
		expectedResult *CleanupResult
		expectError    bool
	}{
		{
			name: "execute smart cleanup with machine learning optimization",
			config: &SmartCleanupConfig{
				Path:              "/tmp/codechunking-cache",
				TargetFreeSpaceGB: 20,
				PredictionWindow:  7 * 24 * time.Hour,
				LearningEnabled:   true,
				ModelType:         "random_forest",
				DryRun:            false,
				UsagePatterns: []UsagePattern{
					{Pattern: "daily_build", Frequency: 50, Importance: 0.9},
					{Pattern: "weekend_cleanup", Frequency: 10, Importance: 0.3},
				},
				OptimizationGoals: []string{"maximize_space", "minimize_disruption", "preserve_performance"},
			},
			expectedResult: &CleanupResult{
				Strategy:        CleanupStrategySmart,
				TotalCandidates: 250,
				CleanedItems:    95,
				BytesFreed:      22 * 1024 * 1024 * 1024, // 22GB (exceeds target due to optimization)
				DryRun:          false,
			},
			expectError: false,
		},
		{
			name: "execute smart cleanup without learning",
			config: &SmartCleanupConfig{
				Path:              "/tmp/codechunking-cache",
				TargetFreeSpaceGB: 15,
				LearningEnabled:   false,
				ModelType:         "heuristic",
				OptimizationGoals: []string{"maximize_space"},
			},
			expectedResult: &CleanupResult{
				Strategy:     CleanupStrategySmart,
				CleanedItems: 70,
				BytesFreed:   16 * 1024 * 1024 * 1024, // 16GB
			},
			expectError: false,
		},
		{
			name: "execute smart cleanup with invalid model type",
			config: &SmartCleanupConfig{
				Path:            "/tmp/codechunking-cache",
				ModelType:       "invalid_model",
				LearningEnabled: true,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testExecuteSmartCleanup(t, tt.config, tt.expectedResult, tt.expectError)
		})
	}
}

func testExecuteSmartCleanup(t *testing.T, config *SmartCleanupConfig, expected *CleanupResult, expectError bool) {
	service := NewDefaultDiskCleanupStrategyService()
	ctx := context.Background()

	result, err := service.ExecuteSmartCleanup(ctx, config)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateCleanupResult(t, result, expected)
}

// Helper functions for validation

func validateCleanupResult(t *testing.T, result, expected *CleanupResult) {
	if result == nil {
		t.Fatal("Expected cleanup result, got nil")
	}

	if expected == nil {
		return // No specific expectations
	}

	if result.Strategy != expected.Strategy {
		t.Errorf("Expected strategy %v, got %v", expected.Strategy, result.Strategy)
	}

	if expected.TotalCandidates > 0 && result.TotalCandidates != expected.TotalCandidates {
		t.Errorf("Expected %d total candidates, got %d", expected.TotalCandidates, result.TotalCandidates)
	}

	if expected.CleanedItems > 0 && result.CleanedItems != expected.CleanedItems {
		t.Errorf("Expected %d cleaned items, got %d", expected.CleanedItems, result.CleanedItems)
	}

	if expected.BytesFreed > 0 && result.BytesFreed != expected.BytesFreed {
		t.Errorf("Expected %d bytes freed, got %d", expected.BytesFreed, result.BytesFreed)
	}

	if result.DryRun != expected.DryRun {
		t.Errorf("Expected dry run %v, got %v", expected.DryRun, result.DryRun)
	}

	if result.Duration <= 0 && !result.DryRun {
		t.Error("Cleanup should have positive duration")
	}

	if result.Performance == nil {
		t.Error("Cleanup result should include performance metrics")
	}
}

func validateCleanupCandidates(t *testing.T, candidates []*CleanupCandidate) {
	for i, candidate := range candidates {
		if candidate.RepositoryURL == "" {
			t.Errorf("Candidate %d should have repository URL", i)
		}
		if candidate.Path == "" {
			t.Errorf("Candidate %d should have path", i)
		}
		if candidate.SizeBytes <= 0 {
			t.Errorf("Candidate %d should have positive size", i)
		}
		if candidate.Score < 0 || candidate.Score > 1 {
			t.Errorf("Candidate %d score should be between 0 and 1, got %f", i, candidate.Score)
		}
		if candidate.Reason == "" {
			t.Errorf("Candidate %d should have cleanup reason", i)
		}
	}
}

func validateStrategyRecommendation(
	t *testing.T,
	rec *StrategyRecommendation,
	expectedStrategy CleanupStrategyType,
	expectedConfidence float64,
) {
	if rec == nil {
		t.Fatal("Expected strategy recommendation, got nil")
	}

	if rec.RecommendedStrategy != expectedStrategy {
		t.Errorf("Expected strategy %v, got %v", expectedStrategy, rec.RecommendedStrategy)
	}

	if rec.Confidence < expectedConfidence {
		t.Errorf("Expected confidence at least %f, got %f", expectedConfidence, rec.Confidence)
	}

	if len(rec.Reasoning) == 0 {
		t.Error("Recommendation should include reasoning")
	}

	if rec.EstimatedFreed <= 0 {
		t.Error("Recommendation should estimate space freed")
	}

	if rec.EstimatedDuration <= 0 {
		t.Error("Recommendation should estimate duration")
	}

	if rec.RiskLevel == "" {
		t.Error("Recommendation should assess risk level")
	}

	if len(rec.AlternativeStrategies) == 0 {
		t.Error("Recommendation should provide alternative strategies")
	}
}
