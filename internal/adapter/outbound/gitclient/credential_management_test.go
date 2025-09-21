package gitclient

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"testing"
)

// TestCredentialEnvironmentVariables tests credential loading from environment variables.
func TestCredentialEnvironmentVariables(t *testing.T) {
	tests := []envCredentialTestCase{
		{
			name: "should load GitHub PAT from environment",
			envVars: map[string]string{
				"GITHUB_TOKEN":    "ghp_1234567890abcdef1234567890abcdef12345678",
				"GITHUB_USERNAME": "testuser",
			},
			repoURL:          "https://github.com/private/env-test-repo.git",
			expectedAuthType: "token",
			expectSuccess:    true,
		},
		{
			name: "should load GitLab PAT from environment",
			envVars: map[string]string{
				"GITLAB_TOKEN":    "glpat-xxxxxxxxxxxxxxxxxxxx",
				"GITLAB_USERNAME": "testuser",
			},
			repoURL:          "https://gitlab.com/private/env-test-repo.git",
			expectedAuthType: "token",
			expectSuccess:    true,
		},
		{
			name: "should load SSH key path from environment",
			envVars: map[string]string{
				"SSH_KEY_PATH":       "/home/user/.ssh/custom_key",
				"SSH_KEY_PASSPHRASE": "test-passphrase",
			},
			repoURL:          "git@github.com:private/ssh-env-repo.git",
			expectedAuthType: "ssh",
			expectSuccess:    true,
		},
		{
			name: "should prioritize specific provider tokens",
			envVars: map[string]string{
				"GITHUB_TOKEN": "ghp_github_token",
				"GITLAB_TOKEN": "glpat-gitlab_token",
				"GIT_TOKEN":    "generic_git_token",
			},
			repoURL:          "https://github.com/private/priority-test.git",
			expectedAuthType: "token",
			expectSuccess:    true,
		},
		{
			name: "should fall back to generic GIT_TOKEN",
			envVars: map[string]string{
				"GIT_TOKEN":    "generic_git_token",
				"GIT_USERNAME": "testuser",
			},
			repoURL:          "https://github.com/private/generic-token-test.git",
			expectedAuthType: "token",
			expectSuccess:    true,
		},
		{
			name: "should fail with invalid token format in environment",
			envVars: map[string]string{
				"GITHUB_TOKEN": "invalid-token-format",
			},
			repoURL:       "https://github.com/private/invalid-env-token.git",
			expectSuccess: false,
			expectedError: "env_token_invalid",
		},
		{
			name:          "should fail when no credentials in environment",
			envVars:       map[string]string{},
			repoURL:       "https://github.com/private/no-creds-repo.git",
			expectSuccess: false,
			expectedError: "env_credentials_not_found",
		},
	}

	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runEnvCredentialTest(t, client, tt)
		})
	}
}

// TestCredentialConfigurationFiles tests credential loading from configuration files.
func TestCredentialConfigurationFiles(t *testing.T) {
	tests := []configCredentialTestCase{
		{
			name: "should load credentials from YAML config",
			configContent: `
git:
  credentials:
    github:
      token: "ghp_1234567890abcdef1234567890abcdef12345678"
      username: "testuser"
      type: "pat"
    gitlab:
      token: "glpat-xxxxxxxxxxxxxxxxxxxx"
      username: "testuser"
      type: "pat"
`,
			configPath:       "/tmp/test-config.yaml",
			repoURL:          "https://github.com/private/config-test-repo.git",
			expectedAuthType: "token",
			expectSuccess:    true,
		},
		{
			name: "should load SSH credentials from config",
			configContent: `
git:
  credentials:
    ssh:
      key_path: "/home/user/.ssh/id_rsa"
      passphrase: "encrypted-passphrase"
      use_agent: false
`,
			configPath:       "/tmp/test-ssh-config.yaml",
			repoURL:          "git@github.com:private/ssh-config-repo.git",
			expectedAuthType: "ssh",
			expectSuccess:    true,
		},
		{
			name: "should load credentials from JSON config",
			configContent: `{
  "git": {
    "credentials": {
      "github": {
        "token": "ghp_json_token_1234567890abcdef12345678",
        "username": "jsonuser",
        "type": "pat"
      }
    }
  }
}`,
			configPath:       "/tmp/test-config.json",
			repoURL:          "https://github.com/private/json-config-test.git",
			expectedAuthType: "token",
			expectSuccess:    true,
		},
		{
			name: "should handle encrypted credentials in config",
			configContent: `
git:
  credentials:
    github:
      token: "encrypted:AES256:base64encodedtoken"
      username: "testuser"
      type: "pat"
      encryption_key_env: "CREDENTIAL_KEY"
`,
			configPath:       "/tmp/encrypted-config.yaml",
			repoURL:          "https://github.com/private/encrypted-config-test.git",
			expectedAuthType: "token",
			expectSuccess:    true,
		},
		{
			name: "should fail with malformed config",
			configContent: `
invalid-yaml-content:
  - missing: structure
`,
			configPath:    "/tmp/malformed-config.yaml",
			repoURL:       "https://github.com/private/malformed-config-test.git",
			expectSuccess: false,
			expectedError: "config_parse_error",
		},
		{
			name:          "should fail with non-existent config file",
			configContent: "",
			configPath:    "/tmp/nonexistent-config.yaml",
			repoURL:       "https://github.com/private/missing-config-test.git",
			expectSuccess: false,
			expectedError: "config_file_not_found",
		},
	}

	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runConfigCredentialTest(t, client, tt)
		})
	}
}

// TestCredentialPrecedence tests the precedence order of credential sources.
func TestCredentialPrecedence(t *testing.T) {
	tests := []precedenceTestCase{
		{
			name:           "explicit credentials should override everything",
			envToken:       "env_token",
			configToken:    "config_token",
			explicitToken:  "explicit_token",
			expectedToken:  "explicit_token",
			expectedSource: "explicit",
		},
		{
			name:           "environment should override config when no explicit",
			envToken:       "env_token",
			configToken:    "config_token",
			explicitToken:  "",
			expectedToken:  "env_token",
			expectedSource: "environment",
		},
		{
			name:           "config should be used when no env or explicit",
			envToken:       "",
			configToken:    "config_token",
			explicitToken:  "",
			expectedToken:  "config_token",
			expectedSource: "config",
		},
		{
			name:           "should handle empty credentials gracefully",
			envToken:       "",
			configToken:    "",
			explicitToken:  "",
			expectedToken:  "",
			expectedSource: "none",
		},
	}

	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPrecedenceTest(t, client, tt)
		})
	}
}

// TestSecureCredentialStorage tests secure storage and retrieval of credentials.
func TestSecureCredentialStorage(t *testing.T) {
	tests := []secureStorageTestCase{
		{
			name:            "should store token securely in memory",
			credentialType:  "token",
			sensitiveData:   "ghp_1234567890abcdef1234567890abcdef12345678",
			storageMethod:   "memory",
			expectEncrypted: true,
			expectSuccess:   true,
		},
		{
			name:            "should store SSH passphrase securely",
			credentialType:  "ssh_passphrase",
			sensitiveData:   "my-secret-passphrase-123",
			storageMethod:   "memory",
			expectEncrypted: true,
			expectSuccess:   true,
		},
		{
			name:            "should store credentials in keyring",
			credentialType:  "token",
			sensitiveData:   "secure-keyring-token",
			storageMethod:   "keyring",
			expectEncrypted: true,
			expectSuccess:   true,
		},
		{
			name:            "should handle keyring unavailable",
			credentialType:  "token",
			sensitiveData:   "fallback-token",
			storageMethod:   "keyring_unavailable",
			expectEncrypted: false,
			expectSuccess:   false,
			expectedError:   "keyring_unavailable",
		},
	}

	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSecureStorageTest(t, client, tt)
		})
	}
}

// TestCredentialCaching tests caching of validated credentials.
func TestCredentialCaching(t *testing.T) {
	tests := []struct {
		name           string
		repoURL        string
		cacheDuration  string
		expectCacheHit bool
		expectSuccess  bool
	}{
		{
			name:           "should cache validated credentials",
			repoURL:        "https://github.com/private/cache-test-repo.git",
			cacheDuration:  "5m",
			expectCacheHit: false, // First call
			expectSuccess:  true,
		},
		{
			name:           "should use cached credentials on second call",
			repoURL:        "https://github.com/private/cache-test-repo.git",
			cacheDuration:  "5m",
			expectCacheHit: true, // Second call
			expectSuccess:  true,
		},
		{
			name:           "should not cache credentials with short duration",
			repoURL:        "https://github.com/private/short-cache-repo.git",
			cacheDuration:  "1s",
			expectCacheHit: false,
			expectSuccess:  true,
		},
	}

	// This test will fail until credential caching is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/cache-test-" + sanitizeName(tt.name)

			authConfig := &outbound.TokenAuthConfig{
				Token:     "ghp_cache_token_1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "cacheuser",
			}

			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			// Configure credential caching
			cacheConfig := &outbound.CredentialCacheConfig{
				Duration: tt.cacheDuration,
				Enabled:  true,
			}

			result, cacheHit, err := client.CloneWithCachedAuth(
				ctx,
				tt.repoURL,
				targetPath,
				cloneOpts,
				authConfig,
				cacheConfig,
			)

			if !tt.expectSuccess {
				if err == nil {
					t.Errorf("expected cached authentication to fail")
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("cached authentication should succeed: %v", err)
			}

			if cacheHit != tt.expectCacheHit {
				t.Errorf("expected cache hit %v, got %v", tt.expectCacheHit, cacheHit)
			}

			if result == nil {
				t.Errorf("expected clone result but got nil")
			}
		})
	}
}

// TestCredentialRotation tests automatic credential rotation.
func TestCredentialRotation(t *testing.T) {
	tests := []rotationTestCase{
		{
			name:            "should rotate OAuth tokens when enabled",
			tokenType:       "oauth",
			provider:        "github",
			rotationEnabled: true,
			expectRotation:  true,
			expectSuccess:   true,
		},
		{
			name:            "should not rotate PAT tokens",
			tokenType:       "pat",
			provider:        "github",
			rotationEnabled: true,
			expectRotation:  false,
			expectSuccess:   true,
		},
		{
			name:            "should skip rotation when disabled",
			tokenType:       "oauth",
			provider:        "gitlab",
			rotationEnabled: false,
			expectRotation:  false,
			expectSuccess:   true,
		},
	}

	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runRotationTest(t, client, tt)
		})
	}
}

// Helper functions to reduce cognitive complexity

type envCredentialTestCase struct {
	name             string
	envVars          map[string]string
	repoURL          string
	expectedAuthType string
	expectSuccess    bool
	expectedError    string
}

func runEnvCredentialTest(t *testing.T, client outbound.AuthenticatedGitClient, tt envCredentialTestCase) {
	// Setup environment variables
	for key, value := range tt.envVars {
		t.Setenv(key, value)
	}

	ctx := context.Background()
	targetPath := "/tmp/env-cred-test-" + sanitizeName(tt.name)

	// Load credentials from environment
	authConfig, err := client.LoadCredentialsFromEnvironment(ctx, tt.repoURL)

	if !tt.expectSuccess {
		if err == nil {
			t.Errorf("expected credential loading to fail")
			return
		}
		if !isGitOperationError(err, tt.expectedError) {
			t.Errorf("expected error type %s, got %v", tt.expectedError, err)
		}
		return
	}

	// Success case validations
	if err != nil {
		t.Fatalf("credential loading should succeed: %v", err)
	}

	if authConfig == nil {
		t.Errorf("expected auth config but got nil")
		return
	}

	if authConfig.GetAuthType() != tt.expectedAuthType {
		t.Errorf("expected auth type %s, got %s", tt.expectedAuthType, authConfig.GetAuthType())
	}

	// Test that credentials work for cloning
	testCredentialsForCloning(t, client, authConfig, tt.repoURL, targetPath, tt.expectedAuthType)
}

func testCredentialsForCloning(
	t *testing.T,
	client outbound.AuthenticatedGitClient,
	authConfig outbound.AuthConfig,
	repoURL, targetPath, expectedAuthType string,
) {
	cloneOpts := valueobject.NewShallowCloneOptions(1, "main")
	var result *outbound.AuthenticatedCloneResult
	var err error

	if expectedAuthType == "ssh" {
		sshConfig := authConfig.(*outbound.SSHAuthConfig)
		result, err = client.CloneWithSSHAuth(context.Background(), repoURL, targetPath, cloneOpts, sshConfig)
	} else {
		tokenConfig := authConfig.(*outbound.TokenAuthConfig)
		result, err = client.CloneWithTokenAuth(context.Background(), repoURL, targetPath, cloneOpts, tokenConfig)
	}

	if err != nil {
		t.Fatalf("clone with environment credentials should succeed: %v", err)
	}

	if result.AuthMethod != expectedAuthType {
		t.Errorf("expected auth method %s, got %s", expectedAuthType, result.AuthMethod)
	}
}

type configCredentialTestCase struct {
	name             string
	configContent    string
	configPath       string
	repoURL          string
	expectedAuthType string
	expectSuccess    bool
	expectedError    string
}

func runConfigCredentialTest(t *testing.T, client outbound.AuthenticatedGitClient, tt configCredentialTestCase) {
	// Create config file if content provided
	if tt.configContent != "" {
		err := os.WriteFile(tt.configPath, []byte(tt.configContent), 0o600)
		if err != nil {
			t.Fatalf("failed to create config file: %v", err)
		}
		defer os.Remove(tt.configPath)
	}

	// Setup encryption key for encrypted config test
	if tt.name == "should handle encrypted credentials in config" {
		t.Setenv("CREDENTIAL_KEY", "test-encryption-key-32-bytes-long")
	}

	ctx := context.Background()

	// Load credentials from config file
	authConfig, err := client.LoadCredentialsFromFile(ctx, tt.configPath, tt.repoURL)

	if !tt.expectSuccess {
		if err == nil {
			t.Errorf("expected credential loading to fail")
			return
		}
		if !isGitOperationError(err, tt.expectedError) {
			t.Errorf("expected error type %s, got %v", tt.expectedError, err)
		}
		return
	}

	if err != nil {
		t.Fatalf("credential loading should succeed: %v", err)
	}

	if authConfig == nil {
		t.Errorf("expected auth config but got nil")
		return
	}

	if authConfig.GetAuthType() != tt.expectedAuthType {
		t.Errorf("expected auth type %s, got %s", tt.expectedAuthType, authConfig.GetAuthType())
	}

	// Validate that credentials are properly parsed
	validateParsedCredentials(t, authConfig)
}

func validateParsedCredentials(t *testing.T, authConfig outbound.AuthConfig) {
	if tokenConfig, ok := authConfig.(*outbound.TokenAuthConfig); ok {
		if tokenConfig.Token == "" {
			t.Errorf("expected token to be loaded from config")
		}
		if tokenConfig.Username == "" {
			t.Errorf("expected username to be loaded from config")
		}
	}
}

type precedenceTestCase struct {
	name           string
	envToken       string
	configToken    string
	explicitToken  string
	expectedToken  string
	expectedSource string
}

func runPrecedenceTest(t *testing.T, client outbound.AuthenticatedGitClient, tt precedenceTestCase) {
	ctx := context.Background()
	repoURL := "https://github.com/private/precedence-test.git"

	// Setup environment if provided
	if tt.envToken != "" {
		t.Setenv("GITHUB_TOKEN", tt.envToken)
	}

	// Setup config file if provided
	var configPath string
	if tt.configToken != "" {
		configPath = "/tmp/precedence-config.yaml"
		configContent := `
git:
  credentials:
    github:
      token: "` + tt.configToken + `"
      username: "testuser"
      type: "pat"
`
		err := os.WriteFile(configPath, []byte(configContent), 0o600)
		if err != nil {
			t.Fatalf("failed to create config file: %v", err)
		}
		defer os.Remove(configPath)
	}

	// Setup explicit credentials if provided
	var explicitConfig outbound.AuthConfig
	if tt.explicitToken != "" {
		explicitConfig = &outbound.TokenAuthConfig{
			Token:     tt.explicitToken,
			TokenType: "pat",
			Provider:  "github",
			Username:  "testuser",
		}
	}

	// Test credential precedence
	finalConfig, source, err := client.ResolveCredentials(ctx, repoURL, explicitConfig, configPath)

	if tt.expectedSource == "none" {
		if err == nil {
			t.Errorf("expected credential resolution to fail when no credentials")
		}
		return
	}

	if err != nil {
		t.Fatalf("credential resolution should succeed: %v", err)
	}

	if finalConfig == nil {
		t.Errorf("expected final config but got nil")
		return
	}

	tokenConfig, ok := finalConfig.(*outbound.TokenAuthConfig)
	if !ok {
		t.Errorf("expected TokenAuthConfig but got %T", finalConfig)
		return
	}
	if tokenConfig == nil {
		t.Errorf("tokenConfig is nil after type assertion")
		return
	}
	if tokenConfig.Token != tt.expectedToken {
		t.Errorf("expected token %s, got %s", tt.expectedToken, tokenConfig.Token)
	}

	if source != tt.expectedSource {
		t.Errorf("expected source %s, got %s", tt.expectedSource, source)
	}
}

type secureStorageTestCase struct {
	name            string
	credentialType  string
	sensitiveData   string
	storageMethod   string
	expectEncrypted bool
	expectSuccess   bool
	expectedError   string
}

func runSecureStorageTest(t *testing.T, client outbound.AuthenticatedGitClient, tt secureStorageTestCase) {
	ctx := context.Background()

	// Store credential securely
	credentialID, err := client.StoreCredentialSecurely(
		ctx,
		tt.credentialType,
		tt.sensitiveData,
		tt.storageMethod,
	)

	if !tt.expectSuccess {
		if err == nil {
			t.Errorf("expected secure storage to fail")
			return
		}
		if !isGitOperationError(err, tt.expectedError) {
			t.Errorf("expected error type %s, got %v", tt.expectedError, err)
		}
		return
	}

	// Success case validations
	if err != nil {
		t.Fatalf("secure storage should succeed: %v", err)
	}

	if credentialID == "" {
		t.Errorf("expected credential ID but got empty string")
	}

	// Retrieve credential
	retrievedData, err := client.RetrieveCredentialSecurely(ctx, credentialID)
	if err != nil {
		t.Fatalf("credential retrieval should succeed: %v", err)
	}

	if retrievedData != tt.sensitiveData {
		t.Errorf("expected retrieved data %s, got %s", tt.sensitiveData, retrievedData)
	}

	// Verify credential is encrypted in storage
	isEncrypted, err := client.IsCredentialEncrypted(ctx, credentialID)
	if err != nil {
		t.Fatalf("encryption check should succeed: %v", err)
	}

	if isEncrypted != tt.expectEncrypted {
		t.Errorf("expected encryption status %v, got %v", tt.expectEncrypted, isEncrypted)
	}

	// Clean up
	err = client.DeleteCredentialSecurely(ctx, credentialID)
	if err != nil {
		t.Fatalf("credential deletion should succeed: %v", err)
	}
}

type rotationTestCase struct {
	name            string
	tokenType       string
	provider        string
	rotationEnabled bool
	expectRotation  bool
	expectSuccess   bool
}

func runRotationTest(t *testing.T, client outbound.AuthenticatedGitClient, tt rotationTestCase) {
	ctx := context.Background()

	authConfig := &outbound.TokenAuthConfig{
		Token:     "expiring-token-12345",
		TokenType: tt.tokenType,
		Provider:  tt.provider,
		Username:  "rotationuser",
	}

	rotationConfig := &outbound.CredentialRotationConfig{
		Enabled:           tt.rotationEnabled,
		RotationThreshold: "24h",
		RefreshToken:      "refresh-token-for-oauth",
	}

	newConfig, rotated, err := client.RotateCredentials(ctx, authConfig, rotationConfig)

	if !tt.expectSuccess {
		if err == nil {
			t.Errorf("expected credential rotation to fail")
		}
		return
	}

	// Success case validations
	if err != nil {
		t.Fatalf("credential rotation should succeed: %v", err)
	}

	if rotated != tt.expectRotation {
		t.Errorf("expected rotation %v, got %v", tt.expectRotation, rotated)
	}

	if tt.expectRotation && newConfig.Token == authConfig.Token {
		t.Errorf("expected new token after rotation")
	}

	if !tt.expectRotation && newConfig.Token != authConfig.Token {
		t.Errorf("expected token to remain unchanged when rotation disabled")
	}
}
