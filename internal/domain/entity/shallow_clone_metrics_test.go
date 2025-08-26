package entity

import (
	"codechunking/internal/domain/valueobject"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Test helper functions to reduce cognitive complexity

func createTestMetrics(t *testing.T, repoURL string, opts valueobject.CloneOptions) *GitCloneMetrics {
	t.Helper()
	repoID := uuid.New()
	metrics, err := NewGitCloneMetrics(repoID, repoURL, opts)
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}
	return metrics
}

func simulateCloneOperation(t *testing.T, metrics *GitCloneMetrics, duration time.Duration, size int64, fileCount int) {
	t.Helper()
	if err := metrics.StartClone(); err != nil {
		t.Fatalf("failed to start clone: %v", err)
	}
	if err := metrics.CompleteClone(duration, size, fileCount); err != nil {
		t.Fatalf("failed to complete clone: %v", err)
	}
}

func simulateCloneFailure(t *testing.T, metrics *GitCloneMetrics, phase, errorMessage string) {
	t.Helper()
	if err := metrics.StartClone(); err != nil {
		t.Fatalf("failed to start clone: %v", err)
	}
	if err := metrics.UpdateProgress(0, 100*1024*1024, 0, phase); err != nil {
		t.Fatalf("failed to update progress: %v", err)
	}
	if err := metrics.FailClone(errorMessage); err != nil {
		t.Fatalf("failed to fail clone: %v", err)
	}
}

func validateCloneComparison(
	t *testing.T,
	comparison *MetricsComparison,
	expectedSpeedRatio, expectedSizeRatio float64,
	expectedWinner string,
) {
	t.Helper()
	if comparison.SpeedRatio < expectedSpeedRatio {
		t.Errorf("expected speed ratio >= %f, got %f", expectedSpeedRatio, comparison.SpeedRatio)
	}
	if comparison.SizeRatio < expectedSizeRatio {
		t.Errorf("expected size ratio >= %f, got %f", expectedSizeRatio, comparison.SizeRatio)
	}
	if comparison.Winner != expectedWinner {
		t.Errorf("expected winner %s, got %s", expectedWinner, comparison.Winner)
	}
	if len(comparison.Improvements) == 0 {
		t.Errorf("expected improvement suggestions")
	}
}

func validateCloneOptions(
	t *testing.T,
	metrics *GitCloneMetrics,
	expectedDepth int,
	expectedBranch string,
	shouldBeShallow, shouldBeSingleBranch bool,
) {
	t.Helper()
	opts := metrics.CloneOptions()
	if opts.Depth() != expectedDepth {
		t.Errorf("expected depth %d, got %d", expectedDepth, opts.Depth())
	}
	if opts.Branch() != expectedBranch {
		t.Errorf("expected branch %s, got %s", expectedBranch, opts.Branch())
	}
	if opts.IsShallowClone() != shouldBeShallow {
		t.Errorf("expected shallow clone %t, got %t", shouldBeShallow, opts.IsShallowClone())
	}
	if opts.IsSingleBranch() != shouldBeSingleBranch {
		t.Errorf("expected single branch %t, got %t", shouldBeSingleBranch, opts.IsSingleBranch())
	}
}

func validateMetricsState(
	t *testing.T,
	metrics *GitCloneMetrics,
	expectedDuration time.Duration,
	expectedSize int64,
	expectedFiles int,
) {
	t.Helper()
	if metrics.CloneDuration() != expectedDuration {
		t.Errorf("expected duration %v, got %v", expectedDuration, metrics.CloneDuration())
	}
	if metrics.RepositorySize() != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, metrics.RepositorySize())
	}
	if metrics.FileCount() != expectedFiles {
		t.Errorf("expected file count %d, got %d", expectedFiles, metrics.FileCount())
	}
}

// TestGitCloneMetrics_ShallowCloneVsFullClone tests performance comparison between shallow and full clones.
func TestGitCloneMetrics_ShallowCloneVsFullClone(t *testing.T) {
	// This test will fail until shallow clone functionality is fully implemented
	t.Run("should demonstrate shallow clone performance advantages", func(t *testing.T) {
		repoURL := "https://github.com/large/repository.git"

		// Create and simulate full clone
		fullCloneOpts := valueobject.NewFullCloneOptions()
		fullMetrics := createTestMetrics(t, repoURL, fullCloneOpts)
		simulateCloneOperation(t, fullMetrics, 5*time.Minute, int64(500*1024*1024), 10000)

		// Create and simulate shallow clone
		shallowOpts := valueobject.NewShallowCloneOptions(1, "main")
		shallowMetrics := createTestMetrics(t, repoURL, shallowOpts)
		simulateCloneOperation(t, shallowMetrics, 30*time.Second, int64(50*1024*1024), 1000)

		// Compare performance
		comparison, err := shallowMetrics.CompareTo(fullMetrics)
		if err != nil {
			t.Fatalf("failed to compare metrics: %v", err)
		}

		// Validate comparison results
		validateCloneComparison(t, comparison, 5.0, 5.0, "current")
	})
}

// TestGitCloneMetrics_ShallowCloneDepthVariations tests different depth values for shallow clones.
func TestGitCloneMetrics_ShallowCloneDepthVariations(t *testing.T) {
	tests := []struct {
		name             string
		depth            int
		branch           string
		expectedDuration time.Duration
		expectedSize     int64
		expectedFiles    int
	}{
		{"depth 1 should be fastest and smallest", 1, "main", 15 * time.Second, 10 * 1024 * 1024, 200},
		{"depth 5 should be moderate speed and size", 5, "main", 45 * time.Second, 30 * 1024 * 1024, 600},
		{"depth 10 should be slower but still efficient", 10, "develop", 90 * time.Second, 50 * 1024 * 1024, 1000},
		{"depth 50 should be much slower", 50, "feature/large-feature", 300 * time.Second, 150 * 1024 * 1024, 3000},
	}

	// This test will fail until shallow clone depth handling is implemented
	repoURL := "https://github.com/test/depth-repository.git"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := valueobject.NewShallowCloneOptions(tt.depth, tt.branch)
			metrics := createTestMetrics(t, repoURL, opts)
			simulateCloneOperation(t, metrics, tt.expectedDuration, tt.expectedSize, tt.expectedFiles)

			validateMetricsState(t, metrics, tt.expectedDuration, tt.expectedSize, tt.expectedFiles)
			validateCloneOptions(t, metrics, tt.depth, tt.branch, true, true)
		})
	}
}

// TestGitCloneMetrics_ShallowCloneSingleBranchOptimization tests single branch optimization.
func TestGitCloneMetrics_ShallowCloneSingleBranchOptimization(t *testing.T) {
	// This test will fail until single branch optimization is implemented
	t.Run("should demonstrate single branch performance benefits", func(t *testing.T) {
		repoURL := "https://github.com/multi-branch/repository.git"

		// Create metrics for non-single branch shallow clone
		nonSingleBranchOpts, err := valueobject.NewCloneOptions(1, "", true)
		if err != nil {
			t.Fatalf("failed to create non-single branch options: %v", err)
		}
		nonSingleBranchMetrics := createTestMetrics(t, repoURL, nonSingleBranchOpts)
		simulateCloneOperation(t, nonSingleBranchMetrics, 60*time.Second, int64(80*1024*1024), 1500)

		// Create metrics for single branch shallow clone
		singleBranchOpts := valueobject.NewShallowCloneOptions(1, "main")
		singleBranchMetrics := createTestMetrics(t, repoURL, singleBranchOpts)
		simulateCloneOperation(t, singleBranchMetrics, 20*time.Second, int64(25*1024*1024), 500)

		// Compare performance
		comparison, err := singleBranchMetrics.CompareTo(nonSingleBranchMetrics)
		if err != nil {
			t.Fatalf("failed to compare metrics: %v", err)
		}

		validateCloneComparison(t, comparison, 2.0, 2.0, "current")
		validateCloneOptions(t, singleBranchMetrics, 1, "main", true, true)
	})
}

type progressUpdate struct {
	phase          string
	bytesReceived  int64
	totalBytes     int64
	filesProcessed int
}

func simulateProgressUpdates(t *testing.T, metrics *GitCloneMetrics, updates []progressUpdate) {
	t.Helper()
	for i, update := range updates {
		err := metrics.UpdateProgress(update.bytesReceived, update.totalBytes, update.filesProcessed, update.phase)
		if err != nil {
			t.Fatalf("failed to update progress at step %d: %v", i, err)
		}
		validateProgressState(t, metrics, update, i)
	}
}

func validateProgressState(t *testing.T, metrics *GitCloneMetrics, expected progressUpdate, step int) {
	t.Helper()
	if metrics.BytesReceived() != expected.bytesReceived {
		t.Errorf("step %d: expected bytes received %d, got %d", step, expected.bytesReceived, metrics.BytesReceived())
	}
	if metrics.TotalBytes() != expected.totalBytes {
		t.Errorf("step %d: expected total bytes %d, got %d", step, expected.totalBytes, metrics.TotalBytes())
	}
	if metrics.FilesProcessed() != expected.filesProcessed {
		t.Errorf(
			"step %d: expected files processed %d, got %d",
			step,
			expected.filesProcessed,
			metrics.FilesProcessed(),
		)
	}
	if metrics.CurrentPhase() != expected.phase {
		t.Errorf("step %d: expected phase %s, got %s", step, expected.phase, metrics.CurrentPhase())
	}
}

// TestGitCloneMetrics_ShallowCloneProgressTracking tests progress tracking during shallow clone.
func TestGitCloneMetrics_ShallowCloneProgressTracking(t *testing.T) {
	// This test will fail until progress tracking is implemented
	t.Run("should track progress during shallow clone operation", func(t *testing.T) {
		repoURL := "https://github.com/progress/repository.git"
		opts := valueobject.NewShallowCloneOptions(1, "main")
		metrics := createTestMetrics(t, repoURL, opts)

		// Start clone
		if err := metrics.StartClone(); err != nil {
			t.Fatalf("failed to start clone: %v", err)
		}

		// Define progress updates
		updates := []progressUpdate{
			{"initializing", 0, 50 * 1024 * 1024, 0},
			{"receiving objects", 10 * 1024 * 1024, 50 * 1024 * 1024, 100},
			{"receiving objects", 25 * 1024 * 1024, 50 * 1024 * 1024, 250},
			{"receiving objects", 40 * 1024 * 1024, 50 * 1024 * 1024, 400},
			{"resolving deltas", 50 * 1024 * 1024, 50 * 1024 * 1024, 500},
			{"checking out files", 50 * 1024 * 1024, 50 * 1024 * 1024, 500},
		}

		simulateProgressUpdates(t, metrics, updates)

		// Complete clone and validate final state
		finalDuration, finalSize, finalFiles := 30*time.Second, int64(50*1024*1024), 500
		if err := metrics.CompleteClone(finalDuration, finalSize, finalFiles); err != nil {
			t.Fatalf("failed to complete clone: %v", err)
		}

		if metrics.Status() != CloneMetricsStatusCompleted {
			t.Errorf("expected status completed, got %s", metrics.Status())
		}
		validateMetricsState(t, metrics, finalDuration, finalSize, finalFiles)
	})
}

func validateFailureState(t *testing.T, metrics *GitCloneMetrics, expectedError, expectedPhase string) {
	t.Helper()
	if metrics.Status() != CloneMetricsStatusFailed {
		t.Errorf("expected status failed, got %s", metrics.Status())
	}
	if metrics.ErrorMessage() == nil {
		t.Errorf("expected error message, got nil")
	} else if *metrics.ErrorMessage() != expectedError {
		t.Errorf("expected error message %s, got %s", expectedError, *metrics.ErrorMessage())
	}
	if metrics.CurrentPhase() != expectedPhase {
		t.Errorf("expected phase %s, got %s", expectedPhase, metrics.CurrentPhase())
	}
}

func validateFailedCloneOperations(t *testing.T, metrics *GitCloneMetrics) {
	t.Helper()
	if _, err := metrics.CalculateEfficiency(); err == nil {
		t.Errorf("expected error when calculating efficiency on failed clone")
	}
	if _, err := metrics.GetPerformanceInsights(); err == nil {
		t.Errorf("expected error when getting insights on failed clone")
	}
}

// TestGitCloneMetrics_ShallowCloneFailureHandling tests error handling for shallow clone failures.
func TestGitCloneMetrics_ShallowCloneFailureHandling(t *testing.T) {
	tests := []struct {
		name         string
		errorMessage string
		failurePhase string
	}{
		{"should handle shallow clone depth not supported", "server does not support shallow clone", "initializing"},
		{
			"should handle branch not found in shallow clone",
			"branch 'non-existent' not found in shallow clone",
			"receiving objects",
		},
		{
			"should handle network failure during shallow clone",
			"network error during shallow clone operation",
			"receiving objects",
		},
		{
			"should handle disk space error during shallow clone",
			"insufficient disk space for shallow clone",
			"checking out files",
		},
	}

	// This test will fail until error handling is implemented
	repoURL := "https://github.com/failure/repository.git"
	opts := valueobject.NewShallowCloneOptions(1, "main")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := createTestMetrics(t, repoURL, opts)
			simulateCloneFailure(t, metrics, tt.failurePhase, tt.errorMessage)
			validateFailureState(t, metrics, tt.errorMessage, tt.failurePhase)
			validateFailedCloneOperations(t, metrics)
		})
	}
}

type performanceTestCase struct {
	name                    string
	opts                    valueobject.CloneOptions
	duration                time.Duration
	size                    int64
	files                   int
	expectedScore           float64
	expectedStrategy        string
	expectedSuggestionCount int
}

func validatePerformanceInsights(t *testing.T, insights *PerformanceInsights, tc performanceTestCase) {
	t.Helper()
	if insights.PerformanceScore != tc.expectedScore {
		t.Errorf("expected performance score %f, got %f", tc.expectedScore, insights.PerformanceScore)
	}
	if insights.RecommendedStrategy != tc.expectedStrategy {
		t.Errorf("expected strategy %s, got %s", tc.expectedStrategy, insights.RecommendedStrategy)
	}
	if len(insights.OptimizationSuggestions) != tc.expectedSuggestionCount {
		t.Errorf("expected %d suggestions, got %d", tc.expectedSuggestionCount, len(insights.OptimizationSuggestions))
	}
}

func validateEfficiencyMetrics(t *testing.T, efficiency *CloneEfficiency) {
	t.Helper()
	if efficiency.BytesPerSecond <= 0 {
		t.Errorf("expected positive bytes per second, got %f", efficiency.BytesPerSecond)
	}
	if efficiency.FilesPerSecond <= 0 {
		t.Errorf("expected positive files per second, got %f", efficiency.FilesPerSecond)
	}
}

// TestGitCloneMetrics_ShallowClonePerformanceInsights tests performance analysis for shallow clones.
func TestGitCloneMetrics_ShallowClonePerformanceInsights(t *testing.T) {
	// This test will fail until performance insights are implemented
	t.Run("should generate appropriate insights for shallow clone performance", func(t *testing.T) {
		repoURL := "https://github.com/insights/repository.git"

		tests := []performanceTestCase{
			{
				name: "fast shallow clone should get high score", opts: valueobject.NewShallowCloneOptions(1, "main"),
				duration: 10 * time.Second, size: 20 * 1024 * 1024, files: 400,
				expectedScore: 90.0, expectedStrategy: "current-strategy-optimal", expectedSuggestionCount: 1,
			},
			{
				name: "slow shallow clone should suggest optimization", opts: valueobject.NewShallowCloneOptions(10, "develop"),
				duration: 120 * time.Second, size: 100 * 1024 * 1024, files: 2000,
				expectedScore: 50.0, expectedStrategy: "optimize-shallow-clone", expectedSuggestionCount: 2,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				metrics := createTestMetrics(t, repoURL, tc.opts)
				simulateCloneOperation(t, metrics, tc.duration, tc.size, tc.files)

				insights, err := metrics.GetPerformanceInsights()
				if err != nil {
					t.Fatalf("failed to get performance insights: %v", err)
				}
				validatePerformanceInsights(t, insights, tc)

				efficiency, err := metrics.CalculateEfficiency()
				if err != nil {
					t.Fatalf("failed to calculate efficiency: %v", err)
				}
				validateEfficiencyMetrics(t, efficiency)
			})
		}
	})
}

func createTimeoutCloneOptions(t *testing.T, timeout time.Duration) valueobject.CloneOptions {
	t.Helper()
	opts := valueobject.NewShallowCloneOptions(1, "main")
	optsWithTimeout, err := opts.WithTimeout(timeout)
	if err != nil {
		t.Fatalf("failed to set timeout: %v", err)
	}
	return optsWithTimeout
}

func simulateTimeoutClone(t *testing.T, metrics *GitCloneMetrics, timeoutError string) {
	t.Helper()
	if err := metrics.StartClone(); err != nil {
		t.Fatalf("failed to start clone: %v", err)
	}
	if err := metrics.UpdateProgress(10*1024*1024, 100*1024*1024, 200, "receiving objects"); err != nil {
		t.Fatalf("failed to update progress: %v", err)
	}
	if err := metrics.FailClone(timeoutError); err != nil {
		t.Fatalf("failed to fail clone: %v", err)
	}
}

func validateTimeoutFailure(
	t *testing.T,
	metrics *GitCloneMetrics,
	expectedTimeout time.Duration,
	timeoutError string,
) {
	t.Helper()
	if metrics.Status() != CloneMetricsStatusFailed {
		t.Errorf("expected status failed, got %s", metrics.Status())
	}
	if metrics.ErrorMessage() == nil || *metrics.ErrorMessage() != timeoutError {
		t.Errorf("expected timeout error message")
	}
	if metrics.BytesReceived() != 10*1024*1024 {
		t.Errorf("expected partial bytes received to be tracked")
	}
	if metrics.CurrentPhase() != "receiving objects" {
		t.Errorf("expected current phase to be preserved")
	}
	if metrics.CloneOptions().Timeout() != expectedTimeout {
		t.Errorf("expected timeout to be preserved in clone options")
	}
}

// TestGitCloneMetrics_ShallowCloneTimeoutHandling tests timeout handling in shallow clones.
func TestGitCloneMetrics_ShallowCloneTimeoutHandling(t *testing.T) {
	// This test will fail until timeout handling is implemented
	t.Run("should handle timeout during shallow clone operation", func(t *testing.T) {
		repoURL := "https://github.com/slow/repository.git"
		timeout := 30 * time.Second
		timeoutError := "clone operation timed out after 30s"

		opts := createTimeoutCloneOptions(t, timeout)
		metrics := createTestMetrics(t, repoURL, opts)
		simulateTimeoutClone(t, metrics, timeoutError)
		validateTimeoutFailure(t, metrics, timeout, timeoutError)
	})
}
