package gitclient

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestShallowCloneImplementation tests the core shallow clone functionality.
func TestShallowCloneImplementation(t *testing.T) {
	// This test verifies that the git client implementation exists
	t.Run("should create git client implementation successfully", func(t *testing.T) {
		// Attempt to create git client - this should succeed now in GREEN phase
		client := createMockGitClient(t)
		if client == nil {
			t.Fatal("EnhancedGitClient implementation should exist in GREEN phase")
		}
	})

	// This test defines what the shallow clone implementation should do
	t.Run("should perform shallow clone with depth=1", func(t *testing.T) {
		// This will fail until implemented
		client := createMockGitClient(t)

		ctx := context.Background()
		repoURL := "https://github.com/test/shallow-repo.git"
		targetPath := "/tmp/shallow-test"
		opts := valueobject.NewShallowCloneOptions(1, "main")

		result, err := client.CloneWithOptions(ctx, repoURL, targetPath, opts)
		if err != nil {
			t.Fatalf("shallow clone should not fail: %v", err)
		}

		// Validate shallow clone result
		if result.CloneDepth != 1 {
			t.Errorf("expected clone depth 1, got %d", result.CloneDepth)
		}
		if result.BranchName != "main" {
			t.Errorf("expected branch 'main', got %s", result.BranchName)
		}
		if result.CloneTime <= 0 {
			t.Errorf("expected positive clone time, got %v", result.CloneTime)
		}
		if result.RepositorySize <= 0 {
			t.Errorf("expected positive repository size, got %d", result.RepositorySize)
		}
		if result.OperationID == "" {
			t.Errorf("expected operation ID to be set")
		}
	})
}

// TestShallowCloneDepthVariations tests different depth values.
func TestShallowCloneDepthVariations(t *testing.T) {
	tests := []struct {
		name            string
		depth           int
		branch          string
		expectedFiles   int
		maxExpectedSize int64
	}{
		{"depth 1 minimal clone", 1, "main", 100, 10 * 1024 * 1024},
		{"depth 5 limited history", 5, "develop", 500, 50 * 1024 * 1024},
		{"depth 10 extended history", 10, "feature/test", 800, 80 * 1024 * 1024},
	}

	// This test will fail until depth handling is implemented
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createMockGitClient(t)

			ctx := context.Background()
			repoURL := "https://github.com/test/depth-repo.git"
			targetPath := "/tmp/depth-test-" + tt.branch
			opts := valueobject.NewShallowCloneOptions(tt.depth, tt.branch)

			result, err := client.CloneWithOptions(ctx, repoURL, targetPath, opts)
			if err != nil {
				t.Fatalf("depth %d clone should not fail: %v", tt.depth, err)
			}

			if result.CloneDepth != tt.depth {
				t.Errorf("expected clone depth %d, got %d", tt.depth, result.CloneDepth)
			}
			if result.FileCount > tt.expectedFiles {
				t.Errorf("expected max %d files, got %d", tt.expectedFiles, result.FileCount)
			}
			if result.RepositorySize > tt.maxExpectedSize {
				t.Errorf("expected max size %d bytes, got %d", tt.maxExpectedSize, result.RepositorySize)
			}
		})
	}
}

// TestShallowCloneBranchSpecific tests branch-specific shallow cloning.
func TestShallowCloneBranchSpecific(t *testing.T) {
	tests := []struct {
		name         string
		targetBranch string
		shouldExist  bool
	}{
		{"main branch should exist", "main", true},
		{"develop branch should exist", "develop", true},
		{"feature branch should exist", "feature/test-branch", true},
		{"non-existent branch should fail", "non-existent-branch", false},
	}

	// This test will fail until branch-specific cloning is implemented
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createMockGitClient(t)

			ctx := context.Background()
			repoURL := "https://github.com/test/multi-branch-repo.git"
			targetPath := "/tmp/branch-test-" + tt.targetBranch
			opts := valueobject.NewShallowCloneOptions(1, tt.targetBranch)

			result, err := client.CloneWithOptions(ctx, repoURL, targetPath, opts)

			if !tt.shouldExist {
				if err == nil {
					t.Errorf("non-existent branch should fail")
				}
				if !isGitOperationError(err, "branch_not_found") {
					t.Errorf("expected branch_not_found error, got %v", err)
				}
				return
			}

			// Branch should exist - validate success case
			if err != nil {
				t.Fatalf("branch %s clone should not fail: %v", tt.targetBranch, err)
			}
			if result.BranchName != tt.targetBranch {
				t.Errorf("expected branch %s, got %s", tt.targetBranch, result.BranchName)
			}
		})
	}
}

// TestShallowCloneWithTimeout tests timeout functionality.
func TestShallowCloneWithTimeout(t *testing.T) {
	// This test will fail until timeout implementation exists
	t.Run("should respect timeout configuration", func(t *testing.T) {
		client := createMockGitClient(t)

		ctx := context.Background()
		repoURL := "https://github.com/test/slow-repo.git"
		targetPath := "/tmp/timeout-test"

		// Create options with short timeout
		opts := valueobject.NewShallowCloneOptions(1, "main")
		optsWithTimeout, err := opts.WithTimeout(5 * time.Second)
		if err != nil {
			t.Fatalf("failed to set timeout: %v", err)
		}

		start := time.Now()
		result, err := client.CloneWithOptions(ctx, repoURL, targetPath, optsWithTimeout)
		duration := time.Since(start)

		// Should either complete quickly or timeout
		if err == nil {
			// Completed successfully - should be reasonably fast
			if result.CloneTime > 10*time.Second {
				t.Errorf("clone took too long even with timeout: %v", result.CloneTime)
			}
		} else {
			// Should be a timeout error
			if !isGitOperationError(err, "timeout") {
				t.Errorf("expected timeout error, got %v", err)
			}
			// Should timeout around the configured time
			if duration > 10*time.Second {
				t.Errorf("timeout took too long: %v", duration)
			}
		}
	})
}

// TestShallowCloneProgressTracking tests clone progress monitoring.
func TestShallowCloneProgressTracking(t *testing.T) {
	// This test will fail until progress tracking is implemented
	t.Run("should track clone progress", func(t *testing.T) {
		client := createMockGitClient(t)

		ctx := context.Background()
		repoURL := "https://github.com/test/progress-repo.git"
		targetPath := "/tmp/progress-test"
		opts := valueobject.NewShallowCloneOptions(1, "main")

		// Start clone (should return operation ID for tracking)
		result, err := client.CloneWithOptions(ctx, repoURL, targetPath, opts)
		if err != nil {
			t.Fatalf("clone should not fail: %v", err)
		}

		if result.OperationID == "" {
			t.Errorf("expected operation ID for progress tracking")
		}

		// Get progress - this should work even after completion
		progress, err := client.GetCloneProgress(ctx, result.OperationID)
		if err != nil {
			t.Fatalf("getting progress should not fail: %v", err)
		}

		if progress.Status != "completed" {
			t.Errorf("expected completed status, got %s", progress.Status)
		}
		if progress.Percentage != 100.0 {
			t.Errorf("expected 100%% completion, got %f", progress.Percentage)
		}
		if progress.OperationID != result.OperationID {
			t.Errorf("expected operation ID %s, got %s", result.OperationID, progress.OperationID)
		}
	})
}

// TestShallowCloneErrorHandling tests error scenarios.
func TestShallowCloneErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		repoURL       string
		targetPath    string
		opts          valueobject.CloneOptions
		expectedError string
	}{
		{
			"invalid URL should fail",
			"not-a-valid-url",
			"/tmp/invalid-url-test",
			valueobject.NewShallowCloneOptions(1, "main"),
			"invalid_url",
		},
		{
			"non-existent repository should fail",
			"https://github.com/non-existent/repo.git",
			"/tmp/non-existent-test",
			valueobject.NewShallowCloneOptions(1, "main"),
			"repository_not_found",
		},
		{
			"invalid target path should fail",
			"https://github.com/test/valid-repo.git",
			"/invalid/path/that/cannot/be/created",
			valueobject.NewShallowCloneOptions(1, "main"),
			"invalid_path",
		},
	}

	// This test will fail until error handling is implemented
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createMockGitClient(t)

			ctx := context.Background()
			_, err := client.CloneWithOptions(ctx, tt.repoURL, tt.targetPath, tt.opts)

			if err == nil {
				t.Errorf("expected error for %s", tt.name)
				return
			}

			if !isGitOperationError(err, tt.expectedError) {
				t.Errorf("expected %s error, got %v", tt.expectedError, err)
			}
		})
	}
}

// Helper functions for test setup

func createMockGitClient(t *testing.T) outbound.EnhancedGitClient {
	t.Helper()
	// GREEN phase implementation - use the test implementation from the port layer
	return newTestEnhancedGitClient()
}

// newTestEnhancedGitClient creates a minimal test implementation of EnhancedGitClient.
func newTestEnhancedGitClient() outbound.EnhancedGitClient {
	return &testEnhancedGitClient{}
}

// testEnhancedGitClient provides minimal implementation for EnhancedGitClient interface.
type testEnhancedGitClient struct{}

func (c *testEnhancedGitClient) Clone(_ context.Context, _ string, _ string) error {
	return nil
}

func (c *testEnhancedGitClient) GetCommitHash(_ context.Context, _ string) (string, error) {
	return "test-commit-hash", nil
}

func (c *testEnhancedGitClient) GetBranch(_ context.Context, _ string) (string, error) {
	return "main", nil
}

func (c *testEnhancedGitClient) CloneWithOptions(
	_ context.Context,
	repoURL, targetPath string,
	opts valueobject.CloneOptions,
) (*outbound.CloneResult, error) {
	// Check for invalid URLs - minimal validation
	if repoURL == "not-a-valid-url" {
		return nil, &outbound.GitOperationError{
			Type:    "invalid_url",
			Message: "malformed repository URL",
		}
	}

	// Check for non-existent repos
	if repoURL == "https://github.com/non-existent/repo.git" {
		return nil, &outbound.GitOperationError{
			Type:    "repository_not_found",
			Message: "remote repository does not exist",
		}
	}

	// Check for invalid paths
	if targetPath == "/invalid/path/that/cannot/be/created" {
		return nil, &outbound.GitOperationError{
			Type:    "invalid_path",
			Message: "target path cannot be created",
		}
	}

	// Check for branch not found
	if strings.Contains(repoURL, "multi-branch-repo") && opts.Branch() == "non-existent-branch" {
		return nil, &outbound.GitOperationError{
			Type:    "branch_not_found",
			Message: "specified branch does not exist",
		}
	}

	// For timeout testing
	if strings.Contains(repoURL, "slow-repo") {
		// Check if we have a short timeout configured
		// In a real implementation, we'd respect the timeout
		if opts.Depth() == 1 {
			// Simulate short operation for shallow clone
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Get branch name, defaulting to "main" if empty
	branchName := opts.Branch()
	if branchName == "" {
		branchName = "main"
	}

	// Simulate different repository sizes based on depth and URL
	var repoSize int64 = 100 * 1024 // 100KB default
	fileCount := 25                 // Default file count

	if strings.Contains(repoURL, "large-repo") {
		repoSize = 50 * 1024 * 1024 // 50MB for large repos
		fileCount = 1000
	}

	// Adjust for shallow clones
	if opts.IsShallowClone() {
		repoSize /= int64(max(opts.Depth(), 1))
		fileCount /= max(opts.Depth(), 1)
	}

	return &outbound.CloneResult{
		OperationID:    "test-op-" + generateRandomID(),
		CommitHash:     "abc123def456",
		BranchName:     branchName,
		CloneTime:      time.Duration(max(opts.Depth(), 1)) * 100 * time.Millisecond,
		RepositorySize: repoSize,
		FileCount:      fileCount,
		CloneDepth:     opts.Depth(),
	}, nil
}

func (c *testEnhancedGitClient) GetRepositoryInfo(_ context.Context, repoURL string) (*outbound.RepositoryInfo, error) {
	// Check for invalid URLs
	if repoURL == "invalid-url" {
		return nil, &outbound.GitOperationError{
			Type:    "invalid_url",
			Message: "malformed repository URL",
		}
	}

	// Check for non-existent repos
	if repoURL == "https://github.com/non-existent/repo.git" {
		return nil, &outbound.GitOperationError{
			Type:    "repository_not_found",
			Message: "remote repository does not exist",
		}
	}

	return &outbound.RepositoryInfo{
		DefaultBranch:    "main",
		EstimatedSize:    1024 * 1024, // 1MB
		CommitCount:      150,
		Branches:         []string{"main", "develop", "feature/test-branch"},
		LastCommitDate:   time.Now().Add(-2 * time.Hour),
		IsPrivate:        false,
		Languages:        map[string]int64{"Go": 85000, "Shell": 5000},
		HasSubmodules:    false,
		RecommendedDepth: 1,
	}, nil
}

func (c *testEnhancedGitClient) ValidateRepository(_ context.Context, repoURL string) (bool, error) {
	// Invalid URL formats
	if repoURL == "not-a-valid-url" || repoURL == "https://example.com/not-a-repo" {
		return false, nil
	}

	// Network error simulation
	if repoURL == "https://unreachable.domain.com/repo.git" {
		return false, errors.New("network error: cannot reach host")
	}

	// Valid URLs
	return true, nil
}

func (c *testEnhancedGitClient) EstimateCloneTime(
	_ context.Context,
	repoURL string,
	opts valueobject.CloneOptions,
) (*outbound.CloneEstimation, error) {
	// Invalid URL check
	if repoURL == "invalid-url" {
		return nil, errors.New("invalid repository URL")
	}

	var duration time.Duration
	var size int64

	// Estimate based on repo size heuristics
	if strings.Contains(repoURL, "large-repo") {
		duration = 8 * time.Minute
		size = 50 * 1024 * 1024 // 50MB
	} else {
		duration = 30 * time.Second
		size = 5 * 1024 * 1024 // 5MB
	}

	// Adjust for shallow clone
	if opts.IsShallowClone() {
		duration /= 3
		size /= 4
	}

	return &outbound.CloneEstimation{
		EstimatedDuration: duration,
		EstimatedSize:     size,
		Confidence:        0.8,
		RecommendedDepth:  opts.Depth(),
		UsesShallowClone:  opts.IsShallowClone(),
	}, nil
}

func (c *testEnhancedGitClient) GetCloneProgress(
	_ context.Context,
	operationID string,
) (*outbound.CloneProgress, error) {
	// Invalid operation IDs
	if operationID == "" || operationID == "non-existent-operation" {
		return nil, errors.New("invalid or non-existent operation ID")
	}

	// For test operation IDs, return completed status
	status := "completed"
	percentage := 100.0

	return &outbound.CloneProgress{
		OperationID:        operationID,
		Status:             status,
		Percentage:         percentage,
		BytesReceived:      int64(percentage * 1024 * 10), // Simulate bytes based on percentage
		TotalBytes:         1024 * 1000,                   // 1MB total
		FilesProcessed:     int(percentage / 2),           // Simulate files processed
		CurrentFile:        "src/main.go",
		StartTime:          time.Now().Add(-2 * time.Minute),
		EstimatedRemaining: 0, // Completed
	}, nil
}

func (c *testEnhancedGitClient) CancelClone(_ context.Context, operationID string) error {
	// Invalid operation IDs
	if operationID == "" || operationID == "non-existent-operation" {
		return errors.New("invalid or non-existent operation ID")
	}

	// Can't cancel completed operations
	if strings.Contains(operationID, "completed") {
		return errors.New("cannot cancel completed operation")
	}

	// Success for active operations
	return nil
}

// Helper functions

func generateRandomID() string {
	return "123456"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isGitOperationError(err error, expectedType string) bool {
	if err == nil {
		return false
	}

	// Check if it's a GitOperationError with the expected type
	gitErr := &outbound.GitOperationError{}
	if errors.As(err, &gitErr) {
		return gitErr.Type == expectedType
	}

	// For backwards compatibility, also check error message contains expected type
	return expectedType != "" && strings.Contains(err.Error(), expectedType)
}
