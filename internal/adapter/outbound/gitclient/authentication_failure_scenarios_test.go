package gitclient

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"
)

// TestTokenAuthenticationFailureScenarios tests various token authentication failure scenarios
// These tests define the expected behavior for different types of authentication failures
// and will initially fail until the authentication logic is properly implemented.
func TestTokenAuthenticationFailureScenarios(t *testing.T) {
	tests := []struct {
		name          string
		repoURL       string
		authConfig    *outbound.TokenAuthConfig
		cloneOptions  valueobject.CloneOptions
		expectedError string
		description   string
	}{
		// ===== TOKEN FORMAT VALIDATION FAILURES =====
		{
			name:    "should fail with token_invalid_format for malformed token",
			repoURL: "https://github.com/private/format-test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "invalid_token", // This triggers token_invalid_format
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_invalid_format",
			description:   "Token validation should detect invalid format and return token_invalid_format error",
		},
		{
			name:    "should fail with token_invalid_format for invalid-token-format token",
			repoURL: "https://github.com/private/another-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "invalid-token-format", // This also triggers token_invalid_format
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_invalid_format",
			description:   "Token validation should detect invalid-token-format and return token_invalid_format error",
		},

		// ===== AUTHENTICATION FAILURES =====
		{
			name:    "should fail with authentication_failed for invalid credentials on no-retry repo",
			repoURL: "https://github.com/private/no-retry-repo.git", // Contains "no-retry-repo" to trigger authentication_failed
			authConfig: &outbound.TokenAuthConfig{
				Token:     "some_invalid_credentials_token", // Any token that doesn't match other patterns
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "authentication_failed",
			description:   "Authentication should fail with authentication_failed for repos containing 'no-retry-repo'",
		},
		{
			name:    "should fail with authentication_failed for wrong credentials on no-retry endpoint",
			repoURL: "https://gitlab.com/private/project-no-retry-repo.git", // Contains "no-retry-repo"
			authConfig: &outbound.TokenAuthConfig{
				Token:     "wrong_credentials",
				TokenType: "pat",
				Provider:  "gitlab",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "authentication_failed",
			description:   "Authentication should fail with authentication_failed for wrong credentials on no-retry repos",
		},

		// ===== TOKEN EXPIRATION FAILURES =====
		{
			name:    "should fail with token_expired for expired token",
			repoURL: "https://github.com/private/test-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "expiredtoken_abc123", // Contains "expiredtoken" to trigger token_expired
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_expired",
			description:   "Token validation should detect expired tokens and return token_expired error",
		},
		{
			name:    "should fail with token_expired for time-based expired token",
			repoURL: "https://bitbucket.org/private/workspace-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "my_expiredtoken_xyz789", // Contains "expiredtoken"
				TokenType: "pat",
				Provider:  "bitbucket",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_expired",
			description:   "Token validation should detect time-based expiration and return token_expired error",
		},

		// ===== TOKEN REVOCATION FAILURES =====
		{
			name:    "should fail with token_revoked for revoked token",
			repoURL: "https://github.com/private/secure-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "revokedtoken_def456", // Contains "revokedtoken" to trigger token_revoked
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_revoked",
			description:   "Token validation should detect revoked tokens and return token_revoked error",
		},
		{
			name:    "should fail with token_revoked for administratively revoked token",
			repoURL: "https://gitlab.com/private/enterprise-repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "admin_revokedtoken_ghi789", // Contains "revokedtoken"
				TokenType: "pat",
				Provider:  "gitlab",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_revoked",
			description:   "Token validation should detect administratively revoked tokens",
		},

		// ===== PERMISSION FAILURES =====
		{
			name:    "should fail with token_insufficient_permissions for limited token on restricted repo",
			repoURL: "https://github.com/private/restricted-repo.git", // Contains "restricted-repo"
			authConfig: &outbound.TokenAuthConfig{
				Token:     "limitedtoken_permissions", // Contains "limitedtoken" + repo contains "restricted-repo"
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_insufficient_permissions",
			description:   "Token validation should detect insufficient permissions for restricted repositories",
		},
		{
			name:    "should fail with token_insufficient_permissions for repo with insufficient permissions",
			repoURL: "https://github.com/private/insufficient-permissions-repo.git", // Contains "insufficient-permissions"
			authConfig: &outbound.TokenAuthConfig{
				Token:     "valid_token_format",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_insufficient_permissions",
			description:   "Authentication should fail with insufficient permissions for protected repositories",
		},

		// ===== TIMEOUT SCENARIOS =====
		{
			name:    "should fail with clone_timeout for slow server timeout",
			repoURL: "https://slow-server.example.com/private/timeout-repo.git", // Contains both "slow-server" and "timeout-repo"
			authConfig: &outbound.TokenAuthConfig{
				Token:     "valid_github_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Timeout:   1 * time.Second,
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "clone_timeout",
			description:   "Clone operations should timeout when server response is too slow",
		},
		{
			name:    "should fail with auth_timeout for slow authentication server",
			repoURL: "https://slow-auth-server.example.com/private/repo.git", // Contains "slow-auth-server"
			authConfig: &outbound.TokenAuthConfig{
				Token:     "auth_test_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
				Timeout:   1 * time.Second,
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "auth_timeout",
			description:   "Authentication should timeout when auth server response is too slow",
		},

		// ===== PROVIDER VALIDATION FAILURES =====
		{
			name:    "should fail with provider_unsupported for unsupported provider",
			repoURL: "https://some-git-host.example.com/private/repo.git",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "valid_token_format",
				TokenType: "pat",
				Provider:  "unsupported", // This triggers provider_unsupported
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "provider_unsupported",
			description:   "Authentication should fail with unsupported provider error for unknown providers",
		},
		{
			name:    "should fail with provider_url_mismatch for GitHub provider with GitLab URL",
			repoURL: "https://gitlab.com/private/project.git", // GitLab URL
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_valid_github_token",
				TokenType: "pat",
				Provider:  "github", // GitHub provider with GitLab URL = mismatch
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "provider_url_mismatch",
			description:   "Authentication should fail when provider type doesn't match repository URL domain",
		},
		{
			name:    "should fail with provider_url_mismatch for GitLab provider with GitHub URL",
			repoURL: "https://github.com/private/repo.git", // GitHub URL
			authConfig: &outbound.TokenAuthConfig{
				Token:     "glpat-valid_gitlab_token",
				TokenType: "pat",
				Provider:  "gitlab", // GitLab provider with GitHub URL = mismatch
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "provider_url_mismatch",
			description:   "Authentication should fail when GitLab provider is used with GitHub URLs",
		},

		// ===== RATE LIMITING FAILURES =====
		{
			name:    "should fail with token_rate_limited for rate limited requests",
			repoURL: "https://github.com/private/rate-limited-repo.git", // Contains "rate-limited"
			authConfig: &outbound.TokenAuthConfig{
				Token:     "active_token_123",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "token_rate_limited",
			description:   "Authentication should fail with rate limiting error when API limits are exceeded",
		},
	}

	// This test will fail until authentication failure handling is properly implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/auth-failure-test-" + sanitizeName(tt.name)

			result, err := client.CloneWithTokenAuth(ctx, tt.repoURL, targetPath, tt.cloneOptions, tt.authConfig)

			// Test should fail - we expect an error
			if err == nil {
				t.Errorf(
					"Expected authentication to fail with error type %s, but operation succeeded",
					tt.expectedError,
				)
				return
			}

			// Verify we get the expected error type
			if !isGitOperationError(err, tt.expectedError) {
				t.Errorf("Expected error type %s, got %v - %s", tt.expectedError, err, tt.description)
			}

			// Result should be nil on error
			if result != nil {
				t.Errorf("Expected nil result on authentication failure, got %v", result)
			}
		})
	}
}

// TestSSHAuthenticationFailureScenarios tests various SSH authentication failure scenarios.
func TestSSHAuthenticationFailureScenarios(t *testing.T) {
	tests := []struct {
		name          string
		repoURL       string
		authConfig    *outbound.SSHAuthConfig
		cloneOptions  valueobject.CloneOptions
		expectedError string
		description   string
	}{
		// ===== SSH KEY NOT FOUND =====
		{
			name:    "should fail with ssh_key_not_found for nonexistent key file",
			repoURL: "git@github.com:private/ssh-test-repo.git",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/nonexistent_key", // This triggers ssh_key_not_found
				Passphrase: "",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "ssh_key_not_found",
			description:   "SSH authentication should fail when key file does not exist",
		},

		// ===== SSH KEY INVALID =====
		{
			name:    "should fail with ssh_key_invalid for invalid key format",
			repoURL: "git@gitlab.com:private/ssh-test-repo.git",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/invalid_key", // This triggers ssh_key_invalid
				Passphrase: "",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "ssh_key_invalid",
			description:   "SSH authentication should fail when key file has invalid format",
		},

		// ===== SSH PASSPHRASE INVALID =====
		{
			name:    "should fail with ssh_passphrase_invalid for wrong passphrase",
			repoURL: "git@bitbucket.org:private/encrypted-key-repo.git",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/id_rsa",
				Passphrase: "wrong-passphrase", // This triggers ssh_passphrase_invalid
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "ssh_passphrase_invalid",
			description:   "SSH authentication should fail when wrong passphrase is provided for encrypted key",
		},

		// ===== SSH ACCESS DENIED =====
		{
			name:    "should fail with ssh_access_denied for repository without permissions",
			repoURL: "git@github.com:private/no-access-repo.git", // Contains "no-access-repo"
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/id_rsa",
				Passphrase: "",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "ssh_access_denied",
			description:   "SSH authentication should fail when key lacks repository permissions",
		},

		// ===== SSH CONNECTION TIMEOUT =====
		{
			name:    "should fail with ssh_timeout for slow server",
			repoURL: "git@slow-server.example.com:private/repo.git", // Contains "slow-server"
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/id_rsa",
				Passphrase: "",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "ssh_timeout",
			description:   "SSH authentication should timeout when server is slow to respond",
		},
		{
			name:    "should fail with ssh_connection_timeout for unreachable server",
			repoURL: "git@unreachable.example.com:private/repo.git", // Contains "unreachable"
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/id_rsa",
				Passphrase: "",
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "ssh_connection_timeout",
			description:   "SSH authentication should timeout when server is unreachable",
		},

		// ===== SSH AGENT FAILURES =====
		{
			name:    "should fail with ssh_agent_unavailable for invalid agent socket",
			repoURL: "git@github.com:private/agent-test-repo.git",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:     "",
				Passphrase:  "",
				UseSSHAgent: true, // Will check SSH_AUTH_SOCK environment variable
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "ssh_agent_unavailable",
			description:   "SSH authentication should fail when SSH agent socket is invalid or unavailable",
		},
		{
			name:    "should fail with ssh_agent_no_keys for agent without suitable keys",
			repoURL: "git@github.com:private/no-keys-repo.git", // Contains "no-keys-repo"
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:     "",
				Passphrase:  "",
				UseSSHAgent: true,
			},
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectedError: "ssh_agent_no_keys",
			description:   "SSH authentication should fail when SSH agent has no suitable keys for the repository",
		},
	}

	// This test will fail until SSH authentication failure handling is properly implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/ssh-failure-test-" + sanitizeName(tt.name)

			result, err := client.CloneWithSSHAuth(ctx, tt.repoURL, targetPath, tt.cloneOptions, tt.authConfig)

			// Test should fail - we expect an error
			if err == nil {
				t.Errorf(
					"Expected SSH authentication to fail with error type %s, but operation succeeded",
					tt.expectedError,
				)
				return
			}

			// Verify we get the expected error type
			if !isGitOperationError(err, tt.expectedError) {
				t.Errorf("Expected error type %s, got %v - %s", tt.expectedError, err, tt.description)
			}

			// Result should be nil on error
			if result != nil {
				t.Errorf("Expected nil result on SSH authentication failure, got %v", result)
			}
		})
	}
}

// TestAuthenticationErrorDistinction ensures we can properly distinguish between different error types.
func TestAuthenticationErrorDistinction(t *testing.T) {
	tests := []struct {
		name          string
		scenario      string
		repoURL       string
		token         string
		expectedError string
		description   string
	}{
		{
			name:          "distinguish between token format validation and authentication failure",
			scenario:      "format_vs_auth",
			repoURL:       "https://github.com/private/format-test-repo.git", // Regular repo
			token:         "invalid_token",                                   // Triggers format error
			expectedError: "token_invalid_format",
			description:   "Should return format error, not authentication error, for malformed tokens",
		},
		{
			name:          "distinguish between expired token and revoked token",
			scenario:      "expired_vs_revoked",
			repoURL:       "https://github.com/private/test-repo.git",
			token:         "expiredtoken_123", // Triggers expired error
			expectedError: "token_expired",
			description:   "Should return expired error, not revoked error, for time-expired tokens",
		},
		{
			name:          "distinguish between authentication failure and permissions",
			scenario:      "auth_vs_permissions",
			repoURL:       "https://github.com/private/no-retry-repo.git", // Triggers auth failure
			token:         "valid_format_token",                           // Valid format
			expectedError: "authentication_failed",
			description:   "Should return authentication failure for invalid credentials on no-retry repos",
		},
		{
			name:          "distinguish between provider mismatch and unsupported provider",
			scenario:      "mismatch_vs_unsupported",
			repoURL:       "https://gitlab.com/private/repo.git", // GitLab URL
			token:         "github_token_123",                    // Valid format, GitHub provider
			expectedError: "provider_url_mismatch",
			description:   "Should return provider mismatch, not unsupported, for valid providers with wrong URLs",
		},
	}

	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/error-distinction-test-" + sanitizeName(tt.name)
			cloneOptions := valueobject.NewShallowCloneOptions(1, "main")

			// For mismatch scenario, intentionally use github provider with GitLab URL
			provider := "github"

			authConfig := &outbound.TokenAuthConfig{
				Token:     tt.token,
				TokenType: "pat",
				Provider:  provider,
				Username:  "testuser",
			}

			result, err := client.CloneWithTokenAuth(ctx, tt.repoURL, targetPath, cloneOptions, authConfig)

			// Test should fail - we expect an error
			if err == nil {
				t.Errorf(
					"Expected authentication to fail with error type %s, but operation succeeded",
					tt.expectedError,
				)
				return
			}

			// Verify we get the exact expected error type
			if !isGitOperationError(err, tt.expectedError) {
				t.Errorf("Expected error type %s, got %v - %s", tt.expectedError, err, tt.description)
			}

			// Result should be nil on error
			if result != nil {
				t.Errorf("Expected nil result on authentication failure, got %v", result)
			}
		})
	}
}

// sanitizeName creates a safe file name from test name.
func sanitizeName(name string) string {
	// Simple sanitization for file paths
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result += string(r)
		} else if r == ' ' {
			result += "-"
		}
	}
	return result
}
