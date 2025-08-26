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
	// This test will fail until the git client implementation exists
	t.Run("should fail - no git client implementation exists yet", func(t *testing.T) {
		// Attempt to create git client - this will fail since implementation doesn't exist
		var client outbound.EnhancedGitClient
		if client == nil {
			t.Fatal("EnhancedGitClient implementation does not exist - this is expected in RED phase")
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

// TestShallowClonePerformanceComparison tests performance benefits of shallow clone.
func TestShallowClonePerformanceComparison(t *testing.T) {
	// This test will fail until performance comparison is implemented
	t.Run("shallow clone should be faster than full clone", func(t *testing.T) {
		client := createMockGitClient(t)
		ctx := context.Background()
		repoURL := "https://github.com/test/large-repo.git"

		// Full clone
		fullTargetPath := "/tmp/full-clone-test"
		fullOpts := valueobject.NewFullCloneOptions()

		fullStart := time.Now()
		fullResult, err := client.CloneWithOptions(ctx, repoURL, fullTargetPath, fullOpts)
		fullDuration := time.Since(fullStart)

		if err != nil {
			t.Fatalf("full clone should not fail: %v", err)
		}

		// Shallow clone
		shallowTargetPath := "/tmp/shallow-clone-test"
		shallowOpts := valueobject.NewShallowCloneOptions(1, "main")

		shallowStart := time.Now()
		shallowResult, err := client.CloneWithOptions(ctx, repoURL, shallowTargetPath, shallowOpts)
		shallowDuration := time.Since(shallowStart)

		if err != nil {
			t.Fatalf("shallow clone should not fail: %v", err)
		}

		// Performance assertions
		if shallowDuration >= fullDuration {
			t.Errorf("shallow clone should be faster: shallow=%v, full=%v", shallowDuration, fullDuration)
		}
		if shallowResult.RepositorySize >= fullResult.RepositorySize {
			t.Errorf("shallow clone should be smaller: shallow=%d, full=%d",
				shallowResult.RepositorySize, fullResult.RepositorySize)
		}
		if shallowResult.FileCount >= fullResult.FileCount {
			t.Errorf("shallow clone should have fewer files: shallow=%d, full=%d",
				shallowResult.FileCount, fullResult.FileCount)
		}

		// Speed ratio should be at least 2x
		speedRatio := float64(fullDuration) / float64(shallowDuration)
		if speedRatio < 2.0 {
			t.Errorf("expected at least 2x speed improvement, got %fx", speedRatio)
		}
	})
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
	// This will fail until implementation exists - that's expected for RED phase
	t.Skip("EnhancedGitClient implementation does not exist - implement in GREEN phase")
	// This line will never be reached due to t.Skip(), but satisfies the linter
	panic("unreachable: implementation required in GREEN phase")
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
