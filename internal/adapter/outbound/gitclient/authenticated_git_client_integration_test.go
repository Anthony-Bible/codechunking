//go:build integration
// +build integration

package gitclient

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants for real git repositories.
const (
	// Small, stable test repository that's publicly accessible.
	TestRepoURL = "https://github.com/octocat/Hello-World.git"

	// Repository with known non-"main" default branch for testing.
	TestRepoURLWithMasterBranch = "https://github.com/microsoft/vscode-go.git"

	// Small repository for shallow clone testing.
	SmallTestRepoURL = "https://github.com/octocat/Spoon-Knife.git"

	// Invalid URLs for error testing.
	InvalidRepoURL     = "https://github.com/nonexistent/repo.git"
	MalformedRepoURL   = "not-a-valid-url"
	UnreachableRepoURL = "https://unreachable-server.example.com/repo.git"

	// Git hash format regex.
	GitHashRegex = "^[a-f0-9]{40}$"
)

// TestClone_RealGitOperations tests that Clone() performs actual git clone operations
// instead of just logging simulation messages.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - Current Clone() only logs "Simulating git clone" and returns nil
// - No actual directory creation or git clone happens
// - No files are actually downloaded from the repository.
func TestClone_RealGitOperations(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		wantErr bool
		errType string
	}{
		{
			name:    "Clone public repository successfully",
			repoURL: TestRepoURL,
			wantErr: false,
		},
		{
			name:    "Clone small fork repository successfully",
			repoURL: SmallTestRepoURL,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAuthenticatedGitClient()
			ctx := context.Background()

			// Create temporary target directory
			tempDir := t.TempDir()
			targetPath := filepath.Join(tempDir, "cloned-repo")

			// Perform clone operation
			err := client.Clone(ctx, tt.repoURL, targetPath)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// THESE ASSERTIONS WILL FAIL WITH CURRENT MOCK IMPLEMENTATION
			// They define the expected behavior for real git clone operations:

			// 1. Target directory should be created
			assert.DirExists(t, targetPath, "Clone should create target directory")

			// 2. .git directory should exist (indicating a real git clone)
			gitDir := filepath.Join(targetPath, ".git")
			assert.DirExists(t, gitDir, "Clone should create .git directory")

			// 3. Repository files should be present (README is common)
			readmePattern := filepath.Join(targetPath, "README*")
			matches, err := filepath.Glob(readmePattern)
			require.NoError(t, err)
			assert.NotEmpty(t, matches, "Clone should download repository files (README)")

			// 4. .git directory should contain standard git structures
			gitObjectsDir := filepath.Join(gitDir, "objects")
			assert.DirExists(t, gitObjectsDir, ".git directory should contain objects folder")

			gitRefsDir := filepath.Join(gitDir, "refs")
			assert.DirExists(t, gitRefsDir, ".git directory should contain refs folder")

			gitHeadFile := filepath.Join(gitDir, "HEAD")
			assert.FileExists(t, gitHeadFile, ".git directory should contain HEAD file")
		})
	}
}

// TestGetCommitHash_RealGitMetadata tests that GetCommitHash() returns actual commit hashes
// from cloned repositories instead of mock values.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - Current GetCommitHash() always returns mock "abc123def456789"
// - No actual git metadata is read from repository.
func TestGetCommitHash_RealGitMetadata(t *testing.T) {
	client := NewAuthenticatedGitClient()
	ctx := context.Background()

	// Create temporary directory and clone a real repository
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "repo-for-hash-test")

	// First clone the repository (this will fail with current implementation)
	err := client.Clone(ctx, TestRepoURL, targetPath)
	require.NoError(t, err)

	// Now get the commit hash from the cloned repository
	commitHash, err := client.GetCommitHash(ctx, targetPath)
	require.NoError(t, err)

	// THESE ASSERTIONS WILL FAIL WITH CURRENT MOCK IMPLEMENTATION
	// They define the expected behavior for real git metadata extraction:

	// 1. Commit hash should be a valid git hash format (40 hex characters)
	assert.Regexp(t, GitHashRegex, commitHash,
		"Commit hash should be 40 character hexadecimal string")

	// 2. Should NOT be the mock value
	assert.NotEqual(t, "abc123def456789", commitHash,
		"Should return real commit hash, not mock value")

	// 3. Should be a non-empty string
	assert.NotEmpty(t, commitHash, "Commit hash should not be empty")

	// 4. Should be exactly 40 characters
	assert.Len(t, commitHash, 40, "Git commit hash should be exactly 40 characters")

	// 5. Should contain only valid hex characters
	assert.True(t, isValidHexString(commitHash),
		"Commit hash should contain only valid hexadecimal characters (0-9, a-f)")
}

// TestGetBranch_RealBranchDetection tests that GetBranch() returns actual branch names
// from cloned repositories instead of mock values.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - Current GetBranch() always returns mock "main"
// - No actual git branch detection is performed.
func TestGetBranch_RealBranchDetection(t *testing.T) {
	tests := []struct {
		name             string
		repoURL          string
		expectedBranches []string // Multiple possible branches since repos can change
	}{
		{
			name:             "Detect branch from GitHub test repo",
			repoURL:          TestRepoURL,
			expectedBranches: []string{"main", "master"}, // Either could be default
		},
		{
			name:             "Detect branch from repository with master branch",
			repoURL:          TestRepoURLWithMasterBranch,
			expectedBranches: []string{"main", "master", "dev", "develop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAuthenticatedGitClient()
			ctx := context.Background()

			// Create temporary directory and clone repository
			tempDir := t.TempDir()
			targetPath := filepath.Join(tempDir, "repo-for-branch-test")

			// Clone the repository (will fail with current mock implementation)
			err := client.Clone(ctx, tt.repoURL, targetPath)
			require.NoError(t, err)

			// Get the branch name from cloned repository
			branchName, err := client.GetBranch(ctx, targetPath)
			require.NoError(t, err)

			// THESE ASSERTIONS WILL FAIL WITH CURRENT MOCK IMPLEMENTATION
			// They define the expected behavior for real branch detection:

			// 1. Should NOT always return "main" (mock behavior)
			// Allow for different possible branch names since repos can vary
			assert.Contains(t, tt.expectedBranches, branchName,
				"Should return actual branch name, one of: %v, got: %s", tt.expectedBranches, branchName)

			// 2. Should be a valid branch name (non-empty, no special chars)
			assert.NotEmpty(t, branchName, "Branch name should not be empty")
			assert.NotContains(t, branchName, " ", "Branch name should not contain spaces")
			assert.NotContains(t, branchName, "\n", "Branch name should not contain newlines")

			// 3. Should be a reasonable length for git branch names
			assert.True(t, len(branchName) > 0 && len(branchName) < 256,
				"Branch name should be reasonable length (1-255 chars)")
		})
	}
}

// TestCloneWithOptions_RealShallowAndBranchOperations tests that CloneWithOptions()
// performs actual shallow cloning and branch selection instead of returning mock data.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - Current CloneWithOptions() returns mock CloneResult data
// - No actual shallow cloning or branch selection is performed
// - Mock commit hash "abc123def456789" is always returned.
func TestCloneWithOptions_RealShallowAndBranchOperations(t *testing.T) {
	tests := []struct {
		name        string
		repoURL     string
		options     valueobject.CloneOptions
		expectFiles bool
	}{
		{
			name:    "Shallow clone with depth 1",
			repoURL: TestRepoURL,
			options: func() valueobject.CloneOptions {
				opts, err := valueobject.NewCloneOptions(1, "main", true)
				if err != nil {
					panic(err)
				}
				return opts
			}(),
			expectFiles: true,
		},
		{
			name:    "Regular clone with specific branch",
			repoURL: SmallTestRepoURL,
			options: func() valueobject.CloneOptions {
				opts, err := valueobject.NewCloneOptions(0, "main", false)
				if err != nil {
					panic(err)
				}
				return opts
			}(),
			expectFiles: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAuthenticatedGitClient()
			ctx := context.Background()

			// Create temporary target directory
			tempDir := t.TempDir()
			targetPath := filepath.Join(tempDir, "cloned-with-options")

			// Perform clone with options
			result, err := client.CloneWithOptions(ctx, tt.repoURL, targetPath, tt.options)
			require.NoError(t, err)
			require.NotNil(t, result)

			// THESE ASSERTIONS WILL FAIL WITH CURRENT MOCK IMPLEMENTATION
			// They define the expected behavior for real CloneWithOptions:

			// 1. Should create actual directory and files
			if tt.expectFiles {
				assert.DirExists(t, targetPath, "CloneWithOptions should create target directory")
				assert.DirExists(t, filepath.Join(targetPath, ".git"), "Should create .git directory")
			}

			// 2. Should return real commit hash, not mock
			assert.Regexp(t, GitHashRegex, result.CommitHash,
				"Should return real commit hash format")
			assert.NotEqual(t, "abc123def456789", result.CommitHash,
				"Should not return mock commit hash")

			// 3. Should return actual branch name based on clone operation
			if tt.options.Branch() != "" {
				assert.Equal(t, tt.options.Branch(), result.BranchName,
					"Result should reflect actually cloned branch")
			}
			assert.NotEmpty(t, result.BranchName, "Branch name should not be empty")

			// 4. Clone time should be realistic, not always mock 100ms
			assert.Positive(t, result.CloneTime, "Clone time should be positive")
			assert.Less(t, result.CloneTime, 5*time.Minute, "Clone time should be reasonable")

			// 5. Repository size should be realistic, not always mock 1024000
			assert.Positive(t, result.RepositorySize, "Repository size should be positive")

			// 6. For shallow clones, verify depth is respected
			if tt.options.IsShallowClone() && tt.expectFiles {
				assert.Equal(t, tt.options.Depth(), result.CloneDepth,
					"Result should reflect actual clone depth")
			}
		})
	}
}

// TestClone_ErrorHandling tests proper error handling for various failure scenarios
// instead of always succeeding with mock implementation.
//
// RED PHASE: Some of these tests may PASS with current mock implementation
// because it doesn't validate URLs, but they define expected error behavior.
func TestClone_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		timeout time.Duration
		wantErr bool
		errType string
	}{
		{
			name:    "Invalid repository URL should fail",
			repoURL: InvalidRepoURL,
			timeout: 30 * time.Second,
			wantErr: true,
			errType: "repository_not_found",
		},
		{
			name:    "Malformed URL should fail",
			repoURL: MalformedRepoURL,
			timeout: 30 * time.Second,
			wantErr: true,
			errType: "invalid_url",
		},
		{
			name:    "Unreachable server should fail",
			repoURL: UnreachableRepoURL,
			timeout: 5 * time.Second,
			wantErr: true,
			errType: "network_error",
		},
		{
			name:    "Context timeout should fail",
			repoURL: TestRepoURL,
			timeout: 1 * time.Nanosecond, // Very short timeout
			wantErr: true,
			errType: "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAuthenticatedGitClient()
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			tempDir := t.TempDir()
			targetPath := filepath.Join(tempDir, "should-fail")

			err := client.Clone(ctx, tt.repoURL, targetPath)

			if tt.wantErr {
				assert.Error(t, err, "Expected error for %s", tt.name)

				// Directory should not exist on failure
				assert.NoDirExists(t, targetPath, "Target directory should not exist on failure")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGetCommitHash_ErrorHandling tests error handling when getting commit hashes
// from invalid or non-existent repositories.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - Current GetCommitHash() ignores the path parameter and always returns mock hash
// - No validation of repository path is performed.
func TestGetCommitHash_ErrorHandling(t *testing.T) {
	client := NewAuthenticatedGitClient()
	ctx := context.Background()

	tests := []struct {
		name     string
		repoPath string
		wantErr  bool
	}{
		{
			name:     "Non-existent path should return error",
			repoPath: "/non/existent/path",
			wantErr:  true,
		},
		{
			name:     "Empty path should return error",
			repoPath: "",
			wantErr:  true,
		},
		{
			name:     "Path to regular file should return error",
			repoPath: "/etc/passwd", // Exists but not a git repo
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCommitHash(ctx, tt.repoPath)

			if tt.wantErr {
				// CURRENT IMPLEMENTATION WILL FAIL THIS ASSERTION
				// because it always returns mock hash without validation
				assert.Error(t, err, "Should return error for invalid repository path")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGetBranch_ErrorHandling tests error handling when getting branch names
// from invalid repositories.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - Current GetBranch() ignores path parameter and always returns "main"
// - No validation is performed.
func TestGetBranch_ErrorHandling(t *testing.T) {
	client := NewAuthenticatedGitClient()
	ctx := context.Background()

	tests := []struct {
		name     string
		repoPath string
		wantErr  bool
	}{
		{
			name:     "Non-existent repository path should return error",
			repoPath: "/completely/non/existent/path",
			wantErr:  true,
		},
		{
			name:     "Empty repository path should return error",
			repoPath: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetBranch(ctx, tt.repoPath)

			if tt.wantErr {
				// CURRENT IMPLEMENTATION WILL FAIL THIS ASSERTION
				// because it always returns "main" without validation
				assert.Error(t, err, "Should return error for invalid repository path")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCloneAndMetadataExtraction_EndToEndIntegration tests the complete workflow
// of cloning a repository and then extracting real metadata from it.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - Clone() doesn't actually clone anything
// - GetCommitHash() and GetBranch() return mock data
// - No real end-to-end git operations occur.
func TestCloneAndMetadataExtraction_EndToEndIntegration(t *testing.T) {
	client := NewAuthenticatedGitClient()
	ctx := context.Background()

	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "end-to-end-test")

	// Step 1: Clone a real repository
	err := client.Clone(ctx, TestRepoURL, targetPath)
	require.NoError(t, err)

	// Step 2: Extract commit hash from cloned repository
	commitHash, err := client.GetCommitHash(ctx, targetPath)
	require.NoError(t, err)

	// Step 3: Extract branch name from cloned repository
	branchName, err := client.GetBranch(ctx, targetPath)
	require.NoError(t, err)

	// Step 4: Verify all operations used real git data

	// Repository should exist with git structure
	assert.DirExists(t, targetPath, "Repository should be cloned")
	assert.DirExists(t, filepath.Join(targetPath, ".git"), "Git directory should exist")

	// Metadata should be real, not mock
	assert.Regexp(t, GitHashRegex, commitHash, "Should have real commit hash")
	assert.NotEqual(t, "abc123def456789", commitHash, "Should not be mock commit hash")
	assert.NotEmpty(t, branchName, "Should have real branch name")

	// Test with CloneWithOptions as well
	optionsTargetPath := filepath.Join(tempDir, "end-to-end-options-test")
	options, err := valueobject.NewCloneOptions(1, "", true)
	require.NoError(t, err)

	result, err := client.CloneWithOptions(ctx, TestRepoURL, optionsTargetPath, options)
	require.NoError(t, err)

	// CloneWithOptions should also return real data
	assert.Regexp(
		t,
		GitHashRegex,
		result.CommitHash,
		"CloneWithOptions should return real commit hash",
	)
	assert.NotEqual(t, "abc123def456789", result.CommitHash, "Should not return mock commit hash")
	assert.NotEmpty(t, result.BranchName, "Should return real branch name")
}

// Helper function to validate hexadecimal strings.
func isValidHexString(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// TestCloneWithOptions_ShallowCloneValidation tests that shallow cloning actually
// limits git history as expected.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - No actual shallow cloning is performed
// - Cannot verify git history depth.
func TestCloneWithOptions_ShallowCloneValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewAuthenticatedGitClient()
	ctx := context.Background()

	tempDir := t.TempDir()
	shallowPath := filepath.Join(tempDir, "shallow-test")

	// Clone with depth 1 (shallow)
	shallowOpts, err := valueobject.NewCloneOptions(1, "", true)
	require.NoError(t, err)
	result, err := client.CloneWithOptions(ctx, TestRepoURL, shallowPath, shallowOpts)
	require.NoError(t, err)

	// THESE ASSERTIONS DEFINE EXPECTED SHALLOW CLONE BEHAVIOR
	// They will fail with current mock implementation:

	// Should create actual repository
	assert.DirExists(t, shallowPath, "Shallow clone should create repository")
	assert.DirExists(t, filepath.Join(shallowPath, ".git"), "Should create .git directory")

	// Should return realistic clone depth
	assert.Equal(t, 1, result.CloneDepth, "Should reflect actual shallow clone depth")

	// Should return real commit hash
	assert.Regexp(t, GitHashRegex, result.CommitHash,
		"Shallow clone should return real commit hash")
	assert.NotEqual(t, "abc123def456789", result.CommitHash,
		"Should not return mock commit hash")
}

// TestCloneWithOptions_BranchSelection tests that specific branch selection
// actually clones the requested branch.
//
// RED PHASE: This test WILL FAIL with current implementation because:
// - No actual branch selection is performed
// - Mock implementation doesn't handle branch-specific cloning.
func TestCloneWithOptions_BranchSelection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Note: This test might need adjustment based on actual repository branches
	// Using a repository known to have multiple branches
	testCases := []struct {
		name            string
		repoURL         string
		requestedBranch string
	}{
		{
			name:            "Clone main branch specifically",
			repoURL:         TestRepoURL,
			requestedBranch: "main",
		},
		// Could add more cases if we find repos with known branch structures
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewAuthenticatedGitClient()
			ctx := context.Background()

			tempDir := t.TempDir()
			targetPath := filepath.Join(tempDir, "branch-test")

			// Clone specific branch
			options, err := valueobject.NewCloneOptions(0, tc.requestedBranch, false)
			require.NoError(t, err)
			result, err := client.CloneWithOptions(ctx, tc.repoURL, targetPath, options)
			require.NoError(t, err)

			// THESE ASSERTIONS DEFINE EXPECTED BRANCH SELECTION BEHAVIOR
			// They will fail with current mock implementation:

			// Should actually clone the repository
			assert.DirExists(t, targetPath, "Branch-specific clone should create repository")

			// Should return the requested branch name
			assert.Equal(t, tc.requestedBranch, result.BranchName,
				"Should clone and return the requested branch")

			// Should return real commit hash for that branch
			assert.Regexp(t, GitHashRegex, result.CommitHash,
				"Should return real commit hash for specified branch")
			assert.NotEqual(t, "abc123def456789", result.CommitHash,
				"Should not return mock commit hash")
		})
	}
}
