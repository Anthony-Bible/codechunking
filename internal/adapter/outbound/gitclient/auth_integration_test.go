package gitclient

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"
)

// TestAuthenticationWithShallowClone tests integration of authentication with shallow cloning.
func TestAuthenticationWithShallowClone(t *testing.T) {
	tests := []struct {
		name           string
		repoURL        string
		authConfig     outbound.AuthConfig
		cloneOptions   valueobject.CloneOptions
		expectSuccess  bool
		expectedError  string
		expectedDepth  int
		expectedBranch string
	}{
		{
			name:    "should perform authenticated shallow clone with SSH",
			repoURL: "git@github.com:private/shallow-ssh-repo.git",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/id_rsa",
				Passphrase: "",
			},
			cloneOptions:   valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess:  true,
			expectedDepth:  1,
			expectedBranch: "main",
		},
		{
			name:    "should perform authenticated shallow clone with token",
			repoURL: "https://github.com/private/shallow-token-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_1234567890abcdef1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:   valueobject.NewShallowCloneOptions(5, "develop"),
			expectSuccess:  true,
			expectedDepth:  5,
			expectedBranch: "develop",
		},
		{
			name:    "should perform authenticated full clone with SSH",
			repoURL: "git@gitlab.com:private/full-ssh-repo.git",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/id_ed25519",
				Passphrase: "",
			},
			cloneOptions:   valueobject.NewFullCloneOptions(),
			expectSuccess:  true,
			expectedDepth:  0,  // Full clone
			expectedBranch: "", // Default branch
		},
		{
			name:    "should handle timeout during authenticated shallow clone",
			repoURL: "https://slow-server.example.com/private/timeout-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "timeout_test_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Timeout:   2 * time.Second,
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess: false,
			expectedError: "clone_timeout",
		},
		{
			name:    "should fail shallow clone with invalid authentication",
			repoURL: "https://github.com/private/no-retry-repo.git", // Changed to trigger authentication_failed
			authConfig: &outbound.TokenAuthConfig{
				Token:     "invalid_credentials_token", // Changed to avoid token_invalid_format
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess: false,
			expectedError: "authentication_failed",
		},
	}

	// This test will fail until authentication integration is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/auth-integration-test-" + sanitizeName(tt.name)

			var result *outbound.AuthenticatedCloneResult
			var err error

			// Choose authentication method based on config type
			switch authConfig := tt.authConfig.(type) {
			case *outbound.SSHAuthConfig:
				result, err = client.CloneWithSSHAuth(ctx, tt.repoURL, targetPath, tt.cloneOptions, authConfig)
			case *outbound.TokenAuthConfig:
				result, err = client.CloneWithTokenAuth(ctx, tt.repoURL, targetPath, tt.cloneOptions, authConfig)
			default:
				t.Fatalf("unsupported auth config type: %T", tt.authConfig)
			}

			if !tt.expectSuccess {
				if err == nil {
					t.Errorf("expected authenticated clone to fail")
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("authenticated clone should succeed: %v", err)
			}

			if result == nil {
				t.Errorf("expected AuthenticatedCloneResult but got nil")
				return
			}

			// Validate clone depth
			if result.CloneDepth != tt.expectedDepth {
				t.Errorf("expected clone depth %d, got %d", tt.expectedDepth, result.CloneDepth)
			}

			// Validate branch (if specified)
			if tt.expectedBranch != "" && result.BranchName != tt.expectedBranch {
				t.Errorf("expected branch %s, got %s", tt.expectedBranch, result.BranchName)
			}

			// Validate authentication method
			expectedAuthMethod := tt.authConfig.GetAuthType()
			if result.AuthMethod != expectedAuthMethod {
				t.Errorf("expected auth method %s, got %s", expectedAuthMethod, result.AuthMethod)
			}

			// Validate basic clone result structure
			if result.CommitHash == "" {
				t.Errorf("expected commit hash")
			}
			if result.OperationID == "" {
				t.Errorf("expected operation ID")
			}
			if result.CloneTime <= 0 {
				t.Errorf("expected positive clone time")
			}
		})
	}
}

// TestAuthenticationWithCloneTimeout tests authentication timeout handling.
func TestAuthenticationWithCloneTimeout(t *testing.T) {
	tests := []struct {
		name            string
		repoURL         string
		authConfig      outbound.AuthConfig
		cloneTimeout    time.Duration
		authTimeout     time.Duration
		expectTimeout   bool
		expectedTimeout string // "auth" or "clone"
	}{
		{
			name:    "should timeout during authentication phase",
			repoURL: "https://slow-auth-server.example.com/private/repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "slow_auth_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Timeout:   1 * time.Second, // Short auth timeout
			},
			cloneTimeout:    30 * time.Second, // Long clone timeout
			authTimeout:     1 * time.Second,
			expectTimeout:   true,
			expectedTimeout: "auth",
		},
		{
			name:    "should timeout during clone phase",
			repoURL: "https://github.com/private/huge-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "fast_auth_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Timeout:   10 * time.Second, // Fast auth timeout
			},
			cloneTimeout:    2 * time.Second, // Short clone timeout
			authTimeout:     10 * time.Second,
			expectTimeout:   true,
			expectedTimeout: "clone",
		},
		{
			name:    "should succeed with adequate timeouts",
			repoURL: "https://github.com/private/normal-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "normal_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Timeout:   30 * time.Second,
			},
			cloneTimeout:  60 * time.Second,
			authTimeout:   30 * time.Second,
			expectTimeout: false,
		},
	}

	// This test will fail until timeout handling is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/timeout-test-" + sanitizeName(tt.name)

			// Configure clone options with timeout
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")
			cloneOptsWithTimeout, err := cloneOpts.WithTimeout(tt.cloneTimeout)
			if err != nil {
				t.Fatalf("failed to set clone timeout: %v", err)
			}

			start := time.Now()
			var result *outbound.AuthenticatedCloneResult

			// Perform authenticated clone
			switch authConfig := tt.authConfig.(type) {
			case *outbound.TokenAuthConfig:
				result, err = client.CloneWithTokenAuth(ctx, tt.repoURL, targetPath, cloneOptsWithTimeout, authConfig)
			default:
				t.Fatalf("unsupported auth config type: %T", tt.authConfig)
			}

			elapsed := time.Since(start)

			if tt.expectTimeout {
				validateTimeoutBehavior(t, err, elapsed, tt.expectedTimeout, tt.authTimeout, tt.cloneTimeout)
				return
			}

			// Success case
			if err != nil {
				t.Fatalf("authenticated clone should succeed: %v", err)
			}
			if result == nil {
				t.Errorf("expected AuthenticatedCloneResult but got nil")
			}
		})
	}
}

// TestAuthenticationRetryLogic tests retry logic for authentication failures.
func TestAuthenticationRetryLogic(t *testing.T) {
	tests := []struct {
		name             string
		repoURL          string
		authConfig       outbound.AuthConfig
		retryConfig      *outbound.AuthRetryConfig
		simulateFailures int
		expectSuccess    bool
		expectedAttempts int
		expectedError    string
	}{
		{
			name:    "should retry transient authentication failures",
			repoURL: "https://github.com/private/retry-test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "transient_failure_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			retryConfig: &outbound.AuthRetryConfig{
				MaxAttempts:     3,
				BackoffDuration: 1 * time.Second,
				RetryableErrors: []string{"network_error", "rate_limited", "server_error"},
			},
			simulateFailures: 2, // Fail twice, succeed on third attempt
			expectSuccess:    true,
			expectedAttempts: 3,
		},
		{
			name:    "should not retry authentication failures",
			repoURL: "https://github.com/private/no-retry-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "invalid_credentials_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			retryConfig: &outbound.AuthRetryConfig{
				MaxAttempts:     3,
				BackoffDuration: 1 * time.Second,
				RetryableErrors: []string{"network_error", "rate_limited"},
			},
			simulateFailures: 3,
			expectSuccess:    false,
			expectedAttempts: 1, // Should not retry auth failures
			expectedError:    "authentication_failed",
		},
		{
			name:    "should exhaust retries on persistent failures",
			repoURL: "https://github.com/private/persistent-failure-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "persistent_failure_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			retryConfig: &outbound.AuthRetryConfig{
				MaxAttempts:     3,
				BackoffDuration: 1 * time.Second,
				RetryableErrors: []string{"network_error", "rate_limited"},
			},
			simulateFailures: 5, // More failures than max attempts
			expectSuccess:    false,
			expectedAttempts: 3,
			expectedError:    "max_retries_exceeded",
		},
	}

	// This test will fail until retry logic is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/retry-test-" + sanitizeName(tt.name)
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			var result *outbound.AuthenticatedCloneResult
			var attempts int
			var err error

			// Perform authenticated clone with retry
			switch authConfig := tt.authConfig.(type) {
			case *outbound.TokenAuthConfig:
				result, attempts, err = client.CloneWithTokenAuthAndRetry(ctx, tt.repoURL, targetPath, cloneOpts, authConfig, tt.retryConfig)
			default:
				t.Fatalf("unsupported auth config type: %T", tt.authConfig)
			}

			validateRetryResult(t, err, result, tt.expectSuccess, tt.expectedError)

			// Validate number of attempts
			if attempts != tt.expectedAttempts {
				t.Errorf("expected %d attempts, got %d", tt.expectedAttempts, attempts)
			}
		})
	}
}

// TestMultiProviderAuthentication tests authentication across different Git providers.
func TestMultiProviderAuthentication(t *testing.T) {
	tests := []struct {
		name          string
		repositories  map[string]outbound.AuthConfig
		expectSuccess bool
	}{
		{
			name: "should authenticate to multiple providers simultaneously",
			repositories: map[string]outbound.AuthConfig{
				"https://github.com/private/github-repo.git": &outbound.TokenAuthConfig{
					Token: "ghp_github_token", TokenType: "pat", Provider: "github", Username: "githubuser",
				},
				"https://gitlab.com/private/gitlab-repo.git": &outbound.TokenAuthConfig{
					Token: "glpat-gitlab_token", TokenType: "pat", Provider: "gitlab", Username: "gitlabuser",
				},
				"git@github.com:private/ssh-repo.git": &outbound.SSHAuthConfig{
					KeyPath: "/home/user/.ssh/id_rsa", Passphrase: "",
				},
			},
			expectSuccess: true,
		},
		{
			name: "should handle mixed authentication methods",
			repositories: map[string]outbound.AuthConfig{
				"https://github.com/private/token-repo.git": &outbound.TokenAuthConfig{
					Token: "ghp_mixed_token", TokenType: "pat", Provider: "github", Username: "mixeduser",
				},
				"git@bitbucket.org:private/ssh-repo.git": &outbound.SSHAuthConfig{
					KeyPath: "/home/user/.ssh/id_ed25519", Passphrase: "",
				},
			},
			expectSuccess: true,
		},
	}

	// This test will fail until multi-provider authentication is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			results := make(map[string]*outbound.AuthenticatedCloneResult)
			var errors []error

			// Clone all repositories simultaneously
			for repoURL, authConfig := range tt.repositories {
				targetPath := "/tmp/multi-provider-test-" + sanitizeName(repoURL)

				var result *outbound.AuthenticatedCloneResult
				var err error

				switch authConfig := authConfig.(type) {
				case *outbound.SSHAuthConfig:
					result, err = client.CloneWithSSHAuth(ctx, repoURL, targetPath, cloneOpts, authConfig)
				case *outbound.TokenAuthConfig:
					result, err = client.CloneWithTokenAuth(ctx, repoURL, targetPath, cloneOpts, authConfig)
				}

				if err != nil {
					errors = append(errors, err)
				} else {
					results[repoURL] = result
				}
			}

			if !tt.expectSuccess {
				if len(errors) == 0 {
					t.Errorf("expected multi-provider authentication to fail")
				}
				return
			}

			// Success case validations
			if len(errors) > 0 {
				t.Fatalf("multi-provider authentication should succeed: %v", errors[0])
			}

			if len(results) != len(tt.repositories) {
				t.Errorf("expected %d successful clones, got %d", len(tt.repositories), len(results))
			}

			// Validate each result
			for repoURL, result := range results {
				if result == nil {
					t.Errorf("expected result for repo %s but got nil", repoURL)
					continue
				}

				expectedAuth := tt.repositories[repoURL].GetAuthType()
				if result.AuthMethod != expectedAuth {
					t.Errorf("expected auth method %s for repo %s, got %s", expectedAuth, repoURL, result.AuthMethod)
				}

				if result.CommitHash == "" {
					t.Errorf("expected commit hash for repo %s", repoURL)
				}
			}
		})
	}
}

// TestAuthenticationProgressTracking tests progress tracking during authenticated clones.
func TestAuthenticationProgressTracking(t *testing.T) {
	tests := []struct {
		name           string
		repoURL        string
		authConfig     outbound.AuthConfig
		trackProgress  bool
		expectProgress bool
	}{
		{
			name:    "should track progress during authenticated shallow clone",
			repoURL: "https://github.com/private/progress-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token: "ghp_progress_token", TokenType: "pat", Provider: "github", Username: "progressuser",
			},
			trackProgress:  true,
			expectProgress: true,
		},
		{
			name:    "should track progress during SSH authenticated clone",
			repoURL: "git@gitlab.com:private/ssh-progress-repo.git",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath: "/home/user/.ssh/id_rsa", Passphrase: "",
			},
			trackProgress:  true,
			expectProgress: true,
		},
		{
			name:    "should not track progress when disabled",
			repoURL: "https://github.com/private/no-progress-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token: "ghp_no_progress_token", TokenType: "pat", Provider: "github", Username: "noprogressuser",
			},
			trackProgress:  false,
			expectProgress: false,
		},
	}

	// This test will fail until progress tracking is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/progress-test-" + sanitizeName(tt.name)
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			// Configure progress tracking
			progressConfig := &outbound.ProgressConfig{
				Enabled:  tt.trackProgress,
				Interval: 1 * time.Second,
			}

			var result *outbound.AuthenticatedCloneResult
			var err error

			// Start authenticated clone with progress tracking
			switch authConfig := tt.authConfig.(type) {
			case *outbound.SSHAuthConfig:
				result, err = client.CloneWithSSHAuthAndProgress(ctx, tt.repoURL, targetPath, cloneOpts, authConfig, progressConfig)
			case *outbound.TokenAuthConfig:
				result, err = client.CloneWithTokenAuthAndProgress(ctx, tt.repoURL, targetPath, cloneOpts, authConfig, progressConfig)
			}

			if err != nil {
				t.Fatalf("authenticated clone with progress should succeed: %v", err)
			}

			if result == nil {
				t.Errorf("expected AuthenticatedCloneResult but got nil")
				return
			}

			// Check if progress was tracked
			validateProgressTracking(ctx, t, client, result, tt.expectProgress)
		})
	}
}

// validateProgressTracking validates progress tracking behavior based on expectations.
func validateProgressTracking(
	ctx context.Context,
	t *testing.T,
	client outbound.AuthenticatedGitClient,
	result *outbound.AuthenticatedCloneResult,
	expectProgress bool,
) {
	if expectProgress {
		validateProgressEnabled(ctx, t, client, result)
	} else {
		validateProgressDisabled(ctx, t, client, result)
	}
}

// validateProgressEnabled verifies that progress tracking is working when enabled.
func validateProgressEnabled(
	ctx context.Context,
	t *testing.T,
	client outbound.AuthenticatedGitClient,
	result *outbound.AuthenticatedCloneResult,
) {
	// Check for operation ID
	if result.OperationID == "" {
		t.Errorf("expected operation ID for progress tracking")
		return
	}

	// Get and validate final progress
	progress, err := client.GetCloneProgress(ctx, result.OperationID)
	if err != nil {
		t.Errorf("should be able to get progress for tracked operation: %v", err)
		return
	}

	if progress == nil {
		t.Errorf("expected progress info but got nil")
		return
	}

	if progress.Status != "completed" {
		t.Errorf("expected completed status, got %s", progress.Status)
	}
}

// validateProgressDisabled verifies that progress tracking is not active when disabled.
func validateProgressDisabled(
	ctx context.Context,
	t *testing.T,
	client outbound.AuthenticatedGitClient,
	result *outbound.AuthenticatedCloneResult,
) {
	// Progress tracking disabled - operation ID might be empty
	if result.OperationID == "" {
		return // No progress tracking as expected
	}

	// If operation ID exists, verify no progress is tracked
	progress, err := client.GetCloneProgress(ctx, result.OperationID)
	if err == nil && progress != nil {
		t.Errorf("expected no progress tracking when disabled")
	}
}

// validateTimeoutBehavior validates timeout behavior and timing expectations.
func validateTimeoutBehavior(
	t *testing.T,
	err error,
	elapsed time.Duration,
	expectedTimeout string,
	authTimeout, cloneTimeout time.Duration,
) {
	// Early return if no error when timeout was expected
	if err == nil {
		t.Errorf("expected timeout error")
		return
	}

	// Validate specific timeout type
	validateTimeoutType(t, err, expectedTimeout)

	// Validate timeout timing
	validateTimeoutTiming(t, elapsed, expectedTimeout, authTimeout, cloneTimeout)
}

// validateTimeoutType verifies the correct timeout error type occurred.
func validateTimeoutType(t *testing.T, err error, expectedTimeout string) {
	switch expectedTimeout {
	case "auth":
		if !isGitOperationError(err, "auth_timeout") {
			t.Errorf("expected auth timeout error, got %v", err)
		}
	case "clone":
		if !isGitOperationError(err, "clone_timeout") {
			t.Errorf("expected clone timeout error, got %v", err)
		}
	}
}

// validateTimeoutTiming verifies timeout occurred within expected time ranges.
func validateTimeoutTiming(
	t *testing.T,
	elapsed time.Duration,
	expectedTimeout string,
	authTimeout, cloneTimeout time.Duration,
) {
	switch expectedTimeout {
	case "auth":
		// Should timeout around auth timeout
		if elapsed > authTimeout+2*time.Second {
			t.Errorf("auth timeout took too long: %v (expected ~%v)", elapsed, authTimeout)
		}
	case "clone":
		// Should timeout around clone timeout (after successful auth)
		if elapsed > cloneTimeout+authTimeout+2*time.Second {
			t.Errorf("clone timeout took too long: %v", elapsed)
		}
	}
}

// validateRetryResult validates the outcome of retry logic based on expectations.
func validateRetryResult(
	t *testing.T,
	err error,
	result *outbound.AuthenticatedCloneResult,
	expectSuccess bool,
	expectedError string,
) {
	if expectSuccess {
		validateRetrySuccess(t, err, result)
	} else {
		validateRetryFailure(t, err, expectedError)
	}
}

// validateRetrySuccess verifies successful retry operation.
func validateRetrySuccess(t *testing.T, err error, result *outbound.AuthenticatedCloneResult) {
	if err != nil {
		t.Fatalf("retry logic should eventually succeed: %v", err)
	}

	if result == nil {
		t.Errorf("expected AuthenticatedCloneResult but got nil")
	}
}

// validateRetryFailure verifies expected retry failure.
func validateRetryFailure(t *testing.T, err error, expectedError string) {
	if err == nil {
		t.Errorf("expected retry logic to eventually fail")
		return
	}

	if !isGitOperationError(err, expectedError) {
		t.Errorf("expected error type %s, got %v", expectedError, err)
	}
}
