package gitclient

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"
)

// TestTokenAuthentication tests token-based authentication for private repositories.
func TestTokenAuthentication(t *testing.T) {
	tests := []struct {
		name           string
		privateRepoURL string
		authConfig     *outbound.TokenAuthConfig
		cloneOptions   valueobject.CloneOptions
		expectSuccess  bool
		expectedError  string
	}{
		{
			name:           "should authenticate with GitHub PAT",
			privateRepoURL: "https://github.com/private/test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_1234567890abcdef1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Scopes:    []string{"repo", "read:org"},
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess: true,
		},
		{
			name:           "should authenticate with GitLab PAT",
			privateRepoURL: "https://gitlab.com/private/test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "glpat-xxxxxxxxxxxxxxxxxxxx",
				TokenType: "pat",
				Provider:  "gitlab",
				Username:  "testuser",
				Scopes:    []string{"read_repository"},
			},
			cloneOptions:  valueobject.NewFullCloneOptions(),
			expectSuccess: true,
		},
		{
			name:           "should authenticate with Bitbucket app password",
			privateRepoURL: "https://bitbucket.org/private/test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ATBB-1234567890abcdef1234567890abcdef12345678",
				TokenType: "app_password",
				Provider:  "bitbucket",
				Username:  "testuser",
				Scopes:    []string{"repositories:read"},
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(5, "develop"),
			expectSuccess: true,
		},
		{
			name:           "should authenticate with GitHub OAuth token",
			privateRepoURL: "https://github.com/private/oauth-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "gho_1234567890abcdef1234567890abcdef12345678",
				TokenType: "oauth",
				Provider:  "github",
				Scopes:    []string{"repo", "user"},
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess: true,
		},
		{
			name:           "should fail with invalid GitHub PAT format",
			privateRepoURL: "https://github.com/private/test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "invalid-token-format",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess: false,
			expectedError: "token_invalid_format",
		},
		{
			name:           "should fail with expired token",
			privateRepoURL: "https://github.com/private/test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_expiredtoken1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess: false,
			expectedError: "token_expired",
		},
		{
			name:           "should fail with insufficient token permissions",
			privateRepoURL: "https://github.com/private/restricted-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_limitedtoken1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Scopes:    []string{"public_repo"}, // insufficient for private repo
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess: false,
			expectedError: "token_insufficient_permissions",
		},
		{
			name:           "should fail with revoked token",
			privateRepoURL: "https://github.com/private/test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_revokedtoken1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectSuccess: false,
			expectedError: "token_revoked",
		},
	}

	// This test will fail until token authentication is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/token-auth-test-" + sanitizeName(tt.name)

			result, err := client.CloneWithTokenAuth(ctx, tt.privateRepoURL, targetPath, tt.cloneOptions, tt.authConfig)

			if !tt.expectSuccess {
				if err == nil {
					t.Errorf("expected token authentication to fail for %s", tt.name)
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("token authentication should succeed: %v", err)
			}

			if result == nil {
				t.Errorf("expected AuthenticatedCloneResult but got nil")
				return
			}

			// Validate that clone actually used token authentication
			if result.AuthMethod != "token" {
				t.Errorf("expected auth method 'token', got %s", result.AuthMethod)
			}

			if result.AuthProvider != tt.authConfig.Provider {
				t.Errorf("expected auth provider %s, got %s", tt.authConfig.Provider, result.AuthProvider)
			}

			// Validate token scopes were preserved
			if len(tt.authConfig.Scopes) > 0 && len(result.TokenScopes) == 0 {
				t.Errorf("expected token scopes to be preserved")
			}

			// Validate clone options were respected
			if tt.cloneOptions.IsShallowClone() && result.CloneDepth != tt.cloneOptions.Depth() {
				t.Errorf("expected clone depth %d, got %d", tt.cloneOptions.Depth(), result.CloneDepth)
			}

			// Validate basic clone result structure
			if result.CommitHash == "" {
				t.Errorf("expected commit hash")
			}
			if result.BranchName == "" {
				t.Errorf("expected branch name")
			}
			if result.OperationID == "" {
				t.Errorf("expected operation ID")
			}
		})
	}
}

// TestTokenValidation tests token validation before attempting authentication.
func TestTokenValidation(t *testing.T) {
	tests := []struct {
		name          string
		authConfig    *outbound.TokenAuthConfig
		expectValid   bool
		expectedError string
	}{
		{
			name: "should validate GitHub PAT format",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_1234567890abcdef1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			expectValid: true,
		},
		{
			name: "should validate GitLab PAT format",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "glpat-xxxxxxxxxxxxxxxxxxxx",
				TokenType: "pat",
				Provider:  "gitlab",
				Username:  "testuser",
			},
			expectValid: true,
		},
		{
			name: "should validate GitHub OAuth token format",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "gho_1234567890abcdef1234567890abcdef12345678",
				TokenType: "oauth",
				Provider:  "github",
			},
			expectValid: true,
		},
		{
			name: "should validate GitHub App token format",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghs_1234567890abcdef1234567890abcdef12345678",
				TokenType: "github_app",
				Provider:  "github",
			},
			expectValid: true,
		},
		{
			name: "should reject empty token",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			expectValid:   false,
			expectedError: "token_empty",
		},
		{
			name: "should reject invalid GitHub PAT format",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "invalid-github-pat",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			expectValid:   false,
			expectedError: "token_invalid_format",
		},
		{
			name: "should reject unsupported provider",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "some-token",
				TokenType: "pat",
				Provider:  "unsupported-provider",
				Username:  "testuser",
			},
			expectValid:   false,
			expectedError: "provider_unsupported",
		},
		{
			name: "should reject malformed GitLab token",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "invalid-gitlab-token",
				TokenType: "pat",
				Provider:  "gitlab",
				Username:  "testuser",
			},
			expectValid:   false,
			expectedError: "token_invalid_format",
		},
	}

	// This test will fail until token validation is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			isValid, err := client.ValidateToken(ctx, tt.authConfig)

			if !tt.expectValid {
				if err == nil {
					t.Errorf("expected token validation to fail")
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				if isValid {
					t.Errorf("expected token to be invalid")
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("token validation should succeed: %v", err)
			}
			if !isValid {
				t.Errorf("token should be valid")
			}
		})
	}
}

// TestTokenScopeValidation tests validation of token scopes for different operations.
func TestTokenScopeValidation(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		operation      string
		requiredScopes []string
		tokenScopes    []string
		expectValid    bool
	}{
		{
			name:           "GitHub: repo scope should allow private repository access",
			provider:       "github",
			operation:      "clone_private_repo",
			requiredScopes: []string{"repo"},
			tokenScopes:    []string{"repo", "user"},
			expectValid:    true,
		},
		{
			name:           "GitHub: public_repo scope should not allow private repository access",
			provider:       "github",
			operation:      "clone_private_repo",
			requiredScopes: []string{"repo"},
			tokenScopes:    []string{"public_repo", "user"},
			expectValid:    false,
		},
		{
			name:           "GitLab: read_repository scope should allow cloning",
			provider:       "gitlab",
			operation:      "clone_private_repo",
			requiredScopes: []string{"read_repository"},
			tokenScopes:    []string{"read_repository", "read_user"},
			expectValid:    true,
		},
		{
			name:           "GitLab: read_user scope should not allow repository access",
			provider:       "gitlab",
			operation:      "clone_private_repo",
			requiredScopes: []string{"read_repository"},
			tokenScopes:    []string{"read_user"},
			expectValid:    false,
		},
		{
			name:           "Bitbucket: repositories:read scope should allow cloning",
			provider:       "bitbucket",
			operation:      "clone_private_repo",
			requiredScopes: []string{"repositories:read"},
			tokenScopes:    []string{"repositories:read"},
			expectValid:    true,
		},
		{
			name:           "should handle multiple required scopes",
			provider:       "github",
			operation:      "clone_with_org_access",
			requiredScopes: []string{"repo", "read:org"},
			tokenScopes:    []string{"repo", "read:org", "user"},
			expectValid:    true,
		},
		{
			name:           "should fail when missing one required scope",
			provider:       "github",
			operation:      "clone_with_org_access",
			requiredScopes: []string{"repo", "read:org"},
			tokenScopes:    []string{"repo", "user"},
			expectValid:    false,
		},
	}

	// This test will fail until scope validation is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			authConfig := &outbound.TokenAuthConfig{
				Token:     "test-token",
				TokenType: "pat",
				Provider:  tt.provider,
				Username:  "testuser",
				Scopes:    tt.tokenScopes,
			}

			isValid, err := client.ValidateTokenScopes(ctx, authConfig, tt.requiredScopes)
			if err != nil {
				t.Fatalf("scope validation should not error: %v", err)
			}

			if isValid != tt.expectValid {
				t.Errorf("expected scope validation result %v, got %v", tt.expectValid, isValid)
			}
		})
	}
}

// TestTokenProviderIntegration tests integration with different Git providers.
func TestTokenProviderIntegration(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		repoURL       string
		tokenFormat   string
		expectSuccess bool
		expectedError string
	}{
		{
			name:          "should work with GitHub.com repositories",
			provider:      "github",
			repoURL:       "https://github.com/private/test-repo.git",
			tokenFormat:   "ghp_1234567890abcdef1234567890abcdef12345678",
			expectSuccess: true,
		},
		{
			name:          "should work with GitHub Enterprise repositories",
			provider:      "github",
			repoURL:       "https://github.enterprise.com/private/test-repo.git",
			tokenFormat:   "ghp_1234567890abcdef1234567890abcdef12345678",
			expectSuccess: true,
		},
		{
			name:          "should work with GitLab.com repositories",
			provider:      "gitlab",
			repoURL:       "https://gitlab.com/private/test-repo.git",
			tokenFormat:   "glpat-xxxxxxxxxxxxxxxxxxxx",
			expectSuccess: true,
		},
		{
			name:          "should work with self-hosted GitLab",
			provider:      "gitlab",
			repoURL:       "https://gitlab.company.com/private/test-repo.git",
			tokenFormat:   "glpat-xxxxxxxxxxxxxxxxxxxx",
			expectSuccess: true,
		},
		{
			name:          "should work with Bitbucket.org repositories",
			provider:      "bitbucket",
			repoURL:       "https://bitbucket.org/private/test-repo.git",
			tokenFormat:   "ATBB-1234567890abcdef1234567890abcdef12345678",
			expectSuccess: true,
		},
		{
			name:          "should fail with mismatched provider and URL",
			provider:      "github",
			repoURL:       "https://gitlab.com/private/test-repo.git",
			tokenFormat:   "ghp_1234567890abcdef1234567890abcdef12345678",
			expectSuccess: false,
			expectedError: "provider_url_mismatch",
		},
		{
			name:          "should fail with unsupported provider",
			provider:      "unsupported",
			repoURL:       "https://unsupported.com/private/test-repo.git",
			tokenFormat:   "some-token",
			expectSuccess: false,
			expectedError: "provider_unsupported",
		},
	}

	// This test will fail until provider integration is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/provider-test-" + sanitizeName(tt.name)

			authConfig := &outbound.TokenAuthConfig{
				Token:     tt.tokenFormat,
				TokenType: "pat",
				Provider:  tt.provider,
				Username:  "testuser",
			}
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			result, err := client.CloneWithTokenAuth(ctx, tt.repoURL, targetPath, cloneOpts, authConfig)

			if !tt.expectSuccess {
				if err == nil {
					t.Errorf("expected provider integration to fail")
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("provider integration should succeed: %v", err)
			}

			if result.AuthProvider != tt.provider {
				t.Errorf("expected auth provider %s, got %s", tt.provider, result.AuthProvider)
			}
		})
	}
}

// TestTokenAuthenticationTimeout tests timeout handling in token authentication.
func TestTokenAuthenticationTimeout(t *testing.T) {
	tests := []struct {
		name         string
		repoURL      string
		timeout      time.Duration
		expectError  bool
		expectedType string
	}{
		{
			name:         "should respect short timeout for slow token validation",
			repoURL:      "https://slow-api.github.com/private/timeout-repo.git",
			timeout:      1 * time.Second,
			expectError:  true,
			expectedType: "token_timeout",
		},
		{
			name:        "should succeed with reasonable timeout",
			repoURL:     "https://github.com/private/quick-repo.git",
			timeout:     30 * time.Second,
			expectError: false,
		},
		{
			name:         "should handle API rate limiting gracefully",
			repoURL:      "https://api.github.com/private/rate-limited-repo.git",
			timeout:      5 * time.Second,
			expectError:  true,
			expectedType: "token_rate_limited",
		},
	}

	// This test will fail until token timeout handling is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/token-timeout-test-" + sanitizeName(tt.name)

			authConfig := &outbound.TokenAuthConfig{
				Token:     "ghp_1234567890abcdef1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Timeout:   tt.timeout,
			}
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			start := time.Now()
			result, err := client.CloneWithTokenAuth(ctx, tt.repoURL, targetPath, cloneOpts, authConfig)
			elapsed := time.Since(start)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected timeout error")
					return
				}
				if !isGitOperationError(err, tt.expectedType) {
					t.Errorf("expected error type %s, got %v", tt.expectedType, err)
				}
				// Timeout should occur roughly within the specified time
				if elapsed > tt.timeout+2*time.Second {
					t.Errorf("timeout took too long: %v (expected ~%v)", elapsed, tt.timeout)
				}
				return
			}

			// Success case
			if err != nil {
				t.Fatalf("token authentication with timeout should succeed: %v", err)
			}
			if result == nil {
				t.Errorf("expected AuthenticatedCloneResult but got nil")
			}
		})
	}
}

// TestTokenRefresh tests automatic token refresh functionality.
func TestTokenRefresh(t *testing.T) {
	tests := []struct {
		name            string
		tokenType       string
		provider        string
		hasRefreshToken bool
		expectRefresh   bool
		expectSuccess   bool
	}{
		{
			name:            "should not attempt refresh for PAT tokens",
			tokenType:       "pat",
			provider:        "github",
			hasRefreshToken: false,
			expectRefresh:   false,
			expectSuccess:   true,
		},
		{
			name:            "should attempt refresh for OAuth tokens",
			tokenType:       "oauth",
			provider:        "github",
			hasRefreshToken: true,
			expectRefresh:   true,
			expectSuccess:   true,
		},
		{
			name:            "should handle refresh failure gracefully",
			tokenType:       "oauth",
			provider:        "gitlab",
			hasRefreshToken: true,
			expectRefresh:   true,
			expectSuccess:   false,
		},
	}

	// This test will fail until token refresh is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/token-refresh-test-" + sanitizeName(tt.name)

			authConfig := &outbound.TokenAuthConfig{
				Token:     "expired-token",
				TokenType: tt.tokenType,
				Provider:  tt.provider,
				Username:  "testuser",
			}

			if tt.hasRefreshToken {
				// In real implementation, refresh token would be stored securely
				authConfig.Token = "refreshable-token"
			}

			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")
			repoURL := "https://github.com/private/refresh-test-repo.git"

			result, err := client.CloneWithTokenAuth(ctx, repoURL, targetPath, cloneOpts, authConfig)

			if !tt.expectSuccess {
				if err == nil {
					t.Errorf("expected token refresh to fail")
					return
				}
				return
			}

			// Success case
			if err != nil {
				t.Fatalf("token refresh should succeed: %v", err)
			}

			// Validate that refresh was attempted if expected
			if tt.expectRefresh && result.AuthMethod != "token" {
				t.Errorf("expected token authentication to be used")
			}
		})
	}
}

// Helper functions for token authentication tests
