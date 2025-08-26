package gitclient

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSSHKeyAuthentication tests SSH key-based authentication for private repositories.
func TestSSHKeyAuthentication(t *testing.T) {
	tests := []struct {
		name           string
		privateRepoURL string
		sshKeyPath     string
		keyPassphrase  string
		expectSuccess  bool
		expectedError  string
		cloneOptions   valueobject.CloneOptions
	}{
		{
			name:           "should authenticate with default SSH key (~/.ssh/id_rsa)",
			privateRepoURL: "git@github.com:private/test-repo.git",
			sshKeyPath:     "", // empty means use default
			keyPassphrase:  "",
			expectSuccess:  true,
			cloneOptions:   valueobject.NewShallowCloneOptions(1, "main"),
		},
		{
			name:           "should authenticate with default SSH key (~/.ssh/id_ed25519)",
			privateRepoURL: "git@github.com:private/test-repo-ed25519.git",
			sshKeyPath:     "", // empty means use default
			keyPassphrase:  "",
			expectSuccess:  true,
			cloneOptions:   valueobject.NewShallowCloneOptions(1, "main"),
		},
		{
			name:           "should authenticate with custom SSH key path",
			privateRepoURL: "git@gitlab.com:private/custom-key-repo.git",
			sshKeyPath:     "/home/user/.ssh/custom_key",
			keyPassphrase:  "",
			expectSuccess:  true,
			cloneOptions:   valueobject.NewFullCloneOptions(),
		},
		{
			name:           "should authenticate with passphrase-protected SSH key",
			privateRepoURL: "git@bitbucket.org:private/passphrase-repo.git",
			sshKeyPath:     "/home/user/.ssh/id_rsa_passphrase",
			keyPassphrase:  "test-passphrase-123",
			expectSuccess:  true,
			cloneOptions:   valueobject.NewShallowCloneOptions(5, "develop"),
		},
		{
			name:           "should fail with non-existent SSH key",
			privateRepoURL: "git@github.com:private/test-repo.git",
			sshKeyPath:     "/home/user/.ssh/nonexistent_key",
			keyPassphrase:  "",
			expectSuccess:  false,
			expectedError:  "ssh_key_not_found",
			cloneOptions:   valueobject.NewShallowCloneOptions(1, "main"),
		},
		{
			name:           "should fail with invalid SSH key format",
			privateRepoURL: "git@github.com:private/test-repo.git",
			sshKeyPath:     "/home/user/.ssh/invalid_key",
			keyPassphrase:  "",
			expectSuccess:  false,
			expectedError:  "ssh_key_invalid",
			cloneOptions:   valueobject.NewShallowCloneOptions(1, "main"),
		},
		{
			name:           "should fail with wrong passphrase",
			privateRepoURL: "git@github.com:private/test-repo.git",
			sshKeyPath:     "/home/user/.ssh/id_rsa_passphrase",
			keyPassphrase:  "wrong-passphrase",
			expectSuccess:  false,
			expectedError:  "ssh_passphrase_invalid",
			cloneOptions:   valueobject.NewShallowCloneOptions(1, "main"),
		},
		{
			name:           "should fail with SSH key lacking repository permissions",
			privateRepoURL: "git@github.com:private/no-access-repo.git",
			sshKeyPath:     "/home/user/.ssh/id_rsa",
			keyPassphrase:  "",
			expectSuccess:  false,
			expectedError:  "ssh_access_denied",
			cloneOptions:   valueobject.NewShallowCloneOptions(1, "main"),
		},
	}

	// This test will fail until SSH authentication is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/ssh-auth-test-" + tt.name

			// Configure SSH authentication
			authConfig := &outbound.SSHAuthConfig{
				KeyPath:    tt.sshKeyPath,
				Passphrase: tt.keyPassphrase,
			}

			result, err := client.CloneWithSSHAuth(ctx, tt.privateRepoURL, targetPath, tt.cloneOptions, authConfig)

			if !tt.expectSuccess {
				if err == nil {
					t.Errorf("expected authentication to fail for %s", tt.name)
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("SSH authentication should succeed: %v", err)
			}

			if result == nil {
				t.Errorf("expected CloneResult but got nil")
				return
			}

			// Validate that clone actually used SSH authentication
			if result.AuthMethod != "ssh" {
				t.Errorf("expected auth method 'ssh', got %s", result.AuthMethod)
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

// TestSSHKeyValidation tests SSH key validation before attempting authentication.
func TestSSHKeyValidation(t *testing.T) {
	tests := []struct {
		name          string
		keyPath       string
		keyContent    string
		passphrase    string
		expectValid   bool
		expectedError string
	}{
		{
			name:        "should validate RSA SSH key",
			keyPath:     "/tmp/test_rsa_key",
			keyContent:  generateMockRSAKey(),
			passphrase:  "",
			expectValid: true,
		},
		{
			name:        "should validate Ed25519 SSH key",
			keyPath:     "/tmp/test_ed25519_key",
			keyContent:  generateMockEd25519Key(),
			passphrase:  "",
			expectValid: true,
		},
		{
			name:        "should validate ECDSA SSH key",
			keyPath:     "/tmp/test_ecdsa_key",
			keyContent:  generateMockECDSAKey(),
			passphrase:  "",
			expectValid: true,
		},
		{
			name:        "should validate passphrase-protected SSH key",
			keyPath:     "/tmp/test_protected_key",
			keyContent:  generateMockProtectedRSAKey(),
			passphrase:  "test-passphrase",
			expectValid: true,
		},
		{
			name:          "should reject invalid SSH key format",
			keyPath:       "/tmp/invalid_key",
			keyContent:    "invalid-ssh-key-content",
			passphrase:    "",
			expectValid:   false,
			expectedError: "ssh_key_invalid",
		},
		{
			name:          "should reject SSH key with wrong passphrase",
			keyPath:       "/tmp/test_protected_key",
			keyContent:    generateMockProtectedRSAKey(),
			passphrase:    "wrong-passphrase",
			expectValid:   false,
			expectedError: "ssh_passphrase_invalid",
		},
		{
			name:          "should reject non-existent SSH key",
			keyPath:       "/tmp/nonexistent_key",
			keyContent:    "",
			passphrase:    "",
			expectValid:   false,
			expectedError: "ssh_key_not_found",
		},
	}

	// This test will fail until SSH key validation is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test key file if content is provided
			if tt.keyContent != "" {
				err := os.WriteFile(tt.keyPath, []byte(tt.keyContent), 0o600)
				if err != nil {
					t.Fatalf("failed to create test key file: %v", err)
				}
				defer os.Remove(tt.keyPath)
			}

			ctx := context.Background()
			authConfig := &outbound.SSHAuthConfig{
				KeyPath:    tt.keyPath,
				Passphrase: tt.passphrase,
			}

			isValid, err := client.ValidateSSHKey(ctx, authConfig)

			if !tt.expectValid {
				if err == nil {
					t.Errorf("expected validation to fail")
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				if isValid {
					t.Errorf("expected SSH key to be invalid")
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("SSH key validation should succeed: %v", err)
			}
			if !isValid {
				t.Errorf("SSH key should be valid")
			}
		})
	}
}

// TestSSHKeyDefaultLocations tests SSH key discovery in default locations.
func TestSSHKeyDefaultLocations(t *testing.T) {
	tests := []struct {
		name            string
		homeDir         string
		availableKeys   []string
		expectedKeyPath string
		expectedKeyType string
		expectFound     bool
	}{
		{
			name:            "should find id_rsa in default location",
			homeDir:         "/tmp/test-home-1",
			availableKeys:   []string{"id_rsa", "id_rsa.pub"},
			expectedKeyPath: "/tmp/test-home-1/.ssh/id_rsa",
			expectedKeyType: "rsa",
			expectFound:     true,
		},
		{
			name:            "should prefer id_ed25519 over id_rsa",
			homeDir:         "/tmp/test-home-2",
			availableKeys:   []string{"id_rsa", "id_rsa.pub", "id_ed25519", "id_ed25519.pub"},
			expectedKeyPath: "/tmp/test-home-2/.ssh/id_ed25519",
			expectedKeyType: "ed25519",
			expectFound:     true,
		},
		{
			name:            "should find id_ecdsa when others not available",
			homeDir:         "/tmp/test-home-3",
			availableKeys:   []string{"id_ecdsa", "id_ecdsa.pub"},
			expectedKeyPath: "/tmp/test-home-3/.ssh/id_ecdsa",
			expectedKeyType: "ecdsa",
			expectFound:     true,
		},
		{
			name:          "should return not found when no keys available",
			homeDir:       "/tmp/test-home-4",
			availableKeys: []string{},
			expectFound:   false,
		},
		{
			name:          "should ignore public keys only",
			homeDir:       "/tmp/test-home-5",
			availableKeys: []string{"id_rsa.pub", "id_ed25519.pub"},
			expectFound:   false,
		},
	}

	// This test will fail until SSH key discovery is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test home directory and SSH keys
			sshDir := filepath.Join(tt.homeDir, ".ssh")
			err := os.MkdirAll(sshDir, 0o755)
			if err != nil {
				t.Fatalf("failed to create SSH directory: %v", err)
			}
			defer os.RemoveAll(tt.homeDir)

			for _, keyName := range tt.availableKeys {
				keyPath := filepath.Join(sshDir, keyName)
				var keyContent string
				switch keyName {
				case "id_rsa":
					keyContent = generateMockRSAKey()
				case "id_ed25519":
					keyContent = generateMockEd25519Key()
				case "id_ecdsa":
					keyContent = generateMockECDSAKey()
				default:
					keyContent = "mock-public-key-content"
				}

				err := os.WriteFile(keyPath, []byte(keyContent), 0o600)
				if err != nil {
					t.Fatalf("failed to create test key %s: %v", keyName, err)
				}
			}

			ctx := context.Background()
			keyInfo, err := client.DiscoverSSHKeys(ctx, tt.homeDir)

			if !tt.expectFound {
				if err == nil && keyInfo != nil {
					t.Errorf("expected no SSH keys to be found")
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("SSH key discovery should succeed: %v", err)
			}
			if keyInfo == nil {
				t.Errorf("expected SSH key info but got nil")
				return
			}

			if keyInfo.KeyPath != tt.expectedKeyPath {
				t.Errorf("expected key path %s, got %s", tt.expectedKeyPath, keyInfo.KeyPath)
			}
			if keyInfo.KeyType != tt.expectedKeyType {
				t.Errorf("expected key type %s, got %s", tt.expectedKeyType, keyInfo.KeyType)
			}
			if !keyInfo.IsValid {
				t.Errorf("discovered key should be valid")
			}
		})
	}
}

// TestSSHAgentIntegration tests integration with SSH agent for key management.
func TestSSHAgentIntegration(t *testing.T) {
	tests := []struct {
		name            string
		agentSocketPath string
		keysInAgent     []string
		repositoryURL   string
		expectAgentUsed bool
		expectSuccess   bool
		expectedError   string
	}{
		{
			name:            "should use SSH agent when available",
			agentSocketPath: "/tmp/ssh-agent-socket",
			keysInAgent:     []string{"rsa-key", "ed25519-key"},
			repositoryURL:   "git@github.com:private/agent-repo.git",
			expectAgentUsed: true,
			expectSuccess:   true,
		},
		{
			name:            "should fallback to key file when agent unavailable",
			agentSocketPath: "",
			keysInAgent:     []string{},
			repositoryURL:   "git@github.com:private/fallback-repo.git",
			expectAgentUsed: false,
			expectSuccess:   true,
		},
		{
			name:            "should fail when agent has no suitable keys",
			agentSocketPath: "/tmp/ssh-agent-socket",
			keysInAgent:     []string{},
			repositoryURL:   "git@github.com:private/no-keys-repo.git",
			expectAgentUsed: true,
			expectSuccess:   false,
			expectedError:   "ssh_agent_no_keys",
		},
		{
			name:            "should fail when agent socket is invalid",
			agentSocketPath: "/invalid/socket/path",
			keysInAgent:     []string{"rsa-key"},
			repositoryURL:   "git@github.com:private/invalid-agent-repo.git",
			expectAgentUsed: false,
			expectSuccess:   false,
			expectedError:   "ssh_agent_unavailable",
		},
	}

	// This test will fail until SSH agent integration is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/ssh-agent-test-" + tt.name

			// Configure SSH agent environment
			if tt.agentSocketPath != "" {
				t.Setenv("SSH_AUTH_SOCK", tt.agentSocketPath)
			} else {
				// Use t.Setenv with empty value to ensure restoration after test
				t.Setenv("SSH_AUTH_SOCK", "")
			}

			// Mock SSH agent with keys
			if tt.agentSocketPath != "" && len(tt.keysInAgent) > 0 {
				setupMockSSHAgent(t, tt.agentSocketPath, tt.keysInAgent)
				defer cleanupMockSSHAgent(tt.agentSocketPath)
			}

			authConfig := &outbound.SSHAuthConfig{
				UseSSHAgent: true,
			}
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			result, err := client.CloneWithSSHAuth(ctx, tt.repositoryURL, targetPath, cloneOpts, authConfig)

			if !tt.expectSuccess {
				if err == nil {
					t.Errorf("expected SSH agent authentication to fail")
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("SSH agent authentication should succeed: %v", err)
			}

			if result.AuthMethod != "ssh" {
				t.Errorf("expected auth method 'ssh', got %s", result.AuthMethod)
			}

			if tt.expectAgentUsed && !result.UsedSSHAgent {
				t.Errorf("expected SSH agent to be used")
			}
			if !tt.expectAgentUsed && result.UsedSSHAgent {
				t.Errorf("expected SSH agent not to be used")
			}
		})
	}
}

// TestSSHAuthenticationTimeout tests timeout handling in SSH authentication.
func TestSSHAuthenticationTimeout(t *testing.T) {
	tests := []struct {
		name         string
		repoURL      string
		timeout      time.Duration
		expectError  bool
		expectedType string
	}{
		{
			name:         "should respect short timeout for slow SSH connection",
			repoURL:      "git@slow-server.example.com:private/timeout-repo.git",
			timeout:      1 * time.Second,
			expectError:  true,
			expectedType: "ssh_timeout",
		},
		{
			name:        "should succeed with reasonable timeout",
			repoURL:     "git@github.com:private/quick-repo.git",
			timeout:     30 * time.Second,
			expectError: false,
		},
		{
			name:         "should handle connection timeout gracefully",
			repoURL:      "git@unreachable.example.com:private/unreachable-repo.git",
			timeout:      5 * time.Second,
			expectError:  true,
			expectedType: "ssh_connection_timeout",
		},
	}

	// This test will fail until SSH timeout handling is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			targetPath := "/tmp/ssh-timeout-test-" + tt.name

			authConfig := &outbound.SSHAuthConfig{
				KeyPath: "/home/user/.ssh/id_rsa",
				Timeout: tt.timeout,
			}
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			start := time.Now()
			result, err := client.CloneWithSSHAuth(ctx, tt.repoURL, targetPath, cloneOpts, authConfig)
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
				t.Fatalf("SSH authentication with timeout should succeed: %v", err)
			}
			if result == nil {
				t.Errorf("expected CloneResult but got nil")
			}
		})
	}
}

// Helper functions for SSH authentication tests

func createMockAuthenticatedGitClient(t *testing.T) outbound.AuthenticatedGitClient {
	t.Helper()
	// GREEN phase implementation - return our actual client
	return NewAuthenticatedGitClient()
}

func generateMockRSAKey() string {
	return `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAzMockRSAPrivateKeyContentHere...
-----END RSA PRIVATE KEY-----`
}

func generateMockEd25519Key() string {
	return `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
-----END OPENSSH PRIVATE KEY-----`
}

func generateMockECDSAKey() string {
	return `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIAMockECDSAPrivateKeyContentHere...
-----END EC PRIVATE KEY-----`
}

func generateMockProtectedRSAKey() string {
	return `-----BEGIN RSA PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-128-CBC,MockEncryptedKeyContentHere...
-----END RSA PRIVATE KEY-----`
}

func setupMockSSHAgent(t *testing.T, _ string, _ []string) {
	t.Helper()
	// This would setup a mock SSH agent socket for testing
	// Implementation details would be in the GREEN phase
}

func cleanupMockSSHAgent(_ string) {
	// This would cleanup the mock SSH agent
	// Implementation details would be in the GREEN phase
}
