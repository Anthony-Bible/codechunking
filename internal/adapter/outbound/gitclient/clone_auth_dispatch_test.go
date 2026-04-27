package gitclient

// TestCloneAuthDispatch verifies the dispatch contract of Clone() on
// AuthenticatedGitClientImpl.
//
// Contract under test:
//
//	Clone(ctx, repoURL, targetPath) must:
//	  1. Call LoadCredentialsFromEnvironment to detect credentials
//	  2. TokenAuthConfig found  → delegate to CloneWithTokenAuth (stub: returns nil)
//	  3. SSHAuthConfig found    → delegate to CloneWithSSHAuth   (stub: returns nil)
//	  4. env_credentials_not_found → plain unauthenticated clone (returns clone_failed for bad URLs)
//	  5. Any other error from LoadCredentialsFromEnvironment → propagate that error unchanged
//
// Detection strategy: CloneWithTokenAuth and CloneWithSSHAuth are stub
// implementations that return mock success without touching the network.
// A plain clone against a non-existent URL always produces a clone_failed error,
// so tests distinguish the dispatch path by whether Clone() returns nil or an error.
//
// t.Setenv and t.Parallel are mutually exclusive — these tests run sequentially.

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"os"
	"testing"
)

// ----------------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------------

// newDispatchTestClient returns the real AuthenticatedGitClientImpl so that we
// test through the actual struct rather than the interface — necessary to call
// LoadCredentialsFromEnvironment directly in priority tests.
func newDispatchTestClient(t *testing.T) *AuthenticatedGitClientImpl {
	t.Helper()
	return &AuthenticatedGitClientImpl{
		cacheManager:        NewCacheManager(),
		disabledProgressOps: make(map[string]bool),
		credentialStorage:   make(map[string]storedCredential),
		credentialCache:     make(map[string]*credentialCacheEntry),
		wipedMemoryHandles:  make(map[string]bool),
	}
}

// isCloneFailedError returns true when the error is a GitOperationError with
// type "clone_failed" — the error emitted by the plain-clone execution path.
func isCloneFailedError(err error) bool {
	var gitErr *outbound.GitOperationError
	return errors.As(err, &gitErr) && gitErr.Type == "clone_failed"
}

// isEnvCredentialLoadError returns true when the error carries the type that
// LoadCredentialsFromEnvironment returns for non-"not-found" failures such as
// "env_token_invalid".  Clone() must propagate these upward unchanged.
func isEnvCredentialLoadError(err error, expectedType string) bool {
	var gitErr *outbound.GitOperationError
	return errors.As(err, &gitErr) && gitErr.Type == expectedType
}

// clearAllAuthEnvVars removes every env variable that LoadCredentialsFromEnvironment
// consults so that each test starts from a known-clean baseline.
// Must be called before t.Setenv to avoid the parallel-conflict restriction.
func clearAllAuthEnvVars(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"GITHUB_TOKEN", "GITHUB_USERNAME",
		"GITLAB_TOKEN", "GITLAB_USERNAME",
		"GIT_TOKEN", "GIT_USERNAME",
		"SSH_KEY_PATH", "SSH_KEY_PASSPHRASE",
	} {
		t.Setenv(key, "")
	}
}

// ----------------------------------------------------------------------------
// TestCloneAuthDispatch — primary test suite
// ----------------------------------------------------------------------------

// TestCloneAuthDispatch_GitLabToken_DelegatesToCloneWithTokenAuth verifies that
// when GITLAB_TOKEN is set and the URL targets gitlab.com, Clone() delegates to
// CloneWithTokenAuth rather than executing a raw git clone.
//
// CloneWithTokenAuth is a stub that returns a mock success without touching the
// network.  The old Clone() implementation runs exec.Command("git", "clone", ...)
// which fails against non-existent URLs.
//
// Contract: Clone() with a valid GITLAB_TOKEN for a gitlab.com URL must return nil.
// Current failure: returns a clone_failed error (real git is invoked).
func TestCloneAuthDispatch_GitLabToken_DelegatesToCloneWithTokenAuth(t *testing.T) {
	clearAllAuthEnvVars(t)
	t.Setenv("GITLAB_TOKEN", "glpat-testtoken123456789")

	client := newDispatchTestClient(t)
	ctx := context.Background()
	targetPath := t.TempDir()

	err := client.Clone(ctx, "https://gitlab.com/nonexistent-org/nonexistent-repo.git", targetPath)
	// After refactor: CloneWithTokenAuth stub returns success → Clone returns nil.
	// Current (old) implementation: real git clone fails → returns clone_failed error.
	if err != nil {
		t.Errorf(
			"Clone() with valid GITLAB_TOKEN must delegate to CloneWithTokenAuth (stub success), "+
				"but got error: %v — this confirms Clone() is still running real git instead of dispatching",
			err,
		)
	}
}

// TestCloneAuthDispatch_GitLabToken_WrongHost_FallsThroughToPlainClone verifies
// that when GITLAB_TOKEN is set but the URL targets github.com (not gitlab.com),
// LoadCredentialsFromEnvironment returns env_credentials_not_found for that URL,
// and Clone() falls through to a plain (unauthenticated) git clone.
//
// A plain git clone against a non-existent URL returns a clone_failed error.
// The token must NOT be applied cross-host.
func TestCloneAuthDispatch_GitLabToken_WrongHost_FallsThroughToPlainClone(t *testing.T) {
	clearAllAuthEnvVars(t)
	t.Setenv("GITLAB_TOKEN", "glpat-testtoken123456789")
	// Intentionally do NOT set GITHUB_TOKEN.

	client := newDispatchTestClient(t)
	ctx := context.Background()
	targetPath := t.TempDir()

	err := client.Clone(ctx, "https://github.com/nonexistent-org/nonexistent-repo.git", targetPath)

	// Must fail — no credentials for github.com, plain git clone fails.
	if err == nil {
		t.Fatal("expected Clone to fail for github.com with only GITLAB_TOKEN set, got nil error")
	}

	// Must be a plain clone_failed error — GITLAB_TOKEN must not have been applied.
	if !isCloneFailedError(err) {
		var gitErr *outbound.GitOperationError
		if errors.As(err, &gitErr) {
			t.Errorf(
				"expected clone_failed for unauthenticated github.com URL, got error type %q — "+
					"GITLAB_TOKEN must not be applied to github.com URLs; "+
					"LoadCredentialsFromEnvironment scopes it to gitlab.com only",
				gitErr.Type,
			)
		} else {
			t.Errorf("expected *outbound.GitOperationError with type clone_failed, got %T: %v", err, err)
		}
	}
}

// TestCloneAuthDispatch_GitHubToken_DelegatesToCloneWithTokenAuth verifies that
// GITHUB_TOKEN for a github.com URL causes Clone() to delegate to CloneWithTokenAuth.
//
// Contract: Clone() with a valid GITHUB_TOKEN for github.com must return nil (stub success).
// Current failure: returns a clone_failed error (real git is invoked).
func TestCloneAuthDispatch_GitHubToken_DelegatesToCloneWithTokenAuth(t *testing.T) {
	clearAllAuthEnvVars(t)
	t.Setenv("GITHUB_TOKEN", "ghp_testtoken1234567890abcdef12345678")

	client := newDispatchTestClient(t)
	ctx := context.Background()
	targetPath := t.TempDir()

	err := client.Clone(ctx, "https://github.com/nonexistent-org/nonexistent-repo.git", targetPath)
	// After refactor: stub success → nil.
	// Current: real git clone fails.
	if err != nil {
		t.Errorf(
			"Clone() with valid GITHUB_TOKEN must delegate to CloneWithTokenAuth (stub success), "+
				"but got error: %v",
			err,
		)
	}
}

// TestCloneAuthDispatch_GenericGitToken_DelegatesToCloneWithTokenAuth verifies
// that GIT_TOKEN (the provider-agnostic fallback) causes Clone() to delegate to
// CloneWithTokenAuth for a non-github/gitlab URL.
//
// Contract: Clone() with a valid GIT_TOKEN must return nil (stub success).
// Current failure: returns a clone_failed error (real git is invoked).
func TestCloneAuthDispatch_GenericGitToken_DelegatesToCloneWithTokenAuth(t *testing.T) {
	clearAllAuthEnvVars(t)
	// Use a URL that won't match github.com or gitlab.com so only GIT_TOKEN fires.
	t.Setenv("GIT_TOKEN", "generic-valid-token-abc123")

	client := newDispatchTestClient(t)
	ctx := context.Background()
	targetPath := t.TempDir()

	err := client.Clone(ctx, "https://bitbucket.org/nonexistent-org/nonexistent-repo.git", targetPath)
	// After refactor: stub success → nil.
	// Current: real git clone fails.
	if err != nil {
		t.Errorf(
			"Clone() with valid GIT_TOKEN must delegate to CloneWithTokenAuth (stub success), "+
				"but got error: %v",
			err,
		)
	}
}

// TestCloneAuthDispatch_SSHKeyPath_DelegatesToCloneWithSSHAuth verifies that
// when SSH_KEY_PATH is set and no token vars are present, Clone() dispatches to
// CloneWithSSHAuth rather than attempting a plain git clone.
//
// CloneWithSSHAuth is a stub that returns mock success without network access.
//
// Contract: Clone() with SSH_KEY_PATH set must return nil (stub success).
// Current failure: Clone() ignores SSH_KEY_PATH entirely; plain git clone runs and fails.
func TestCloneAuthDispatch_SSHKeyPath_DelegatesToCloneWithSSHAuth(t *testing.T) {
	clearAllAuthEnvVars(t)

	// Create a real file so LoadCredentialsFromEnvironment finds it and returns SSHAuthConfig.
	keyFile, createErr := os.CreateTemp(t.TempDir(), "test_ssh_key_*")
	if createErr != nil {
		t.Fatalf("failed to create temp SSH key file: %v", createErr)
	}
	keyPath := keyFile.Name()
	keyFile.Close()
	t.Cleanup(func() { os.Remove(keyPath) })

	t.Setenv("SSH_KEY_PATH", keyPath)

	client := newDispatchTestClient(t)
	ctx := context.Background()
	targetPath := t.TempDir()

	err := client.Clone(ctx, "git@nonexistent.example.com:org/repo.git", targetPath)
	// After refactor: CloneWithSSHAuth stub returns success → Clone returns nil.
	// Current: Clone() ignores SSH_KEY_PATH; real git clone runs and fails.
	if err != nil {
		t.Errorf(
			"Clone() with SSH_KEY_PATH set must delegate to CloneWithSSHAuth (stub success), "+
				"but got error: %v — Clone() is not dispatching to CloneWithSSHAuth",
			err,
		)
	}
}

// TestCloneAuthDispatch_NoCredentials_PlainCloneFails verifies the no-credentials
// case: when no auth env vars are set, LoadCredentialsFromEnvironment returns
// env_credentials_not_found and Clone() must fall through to a plain git clone.
//
// A plain git clone against a non-existent URL must return a clone_failed error.
// This test should pass both before and after the refactor — it validates the
// fall-through behavior is preserved.
func TestCloneAuthDispatch_NoCredentials_PlainCloneFails(t *testing.T) {
	clearAllAuthEnvVars(t)

	client := newDispatchTestClient(t)
	ctx := context.Background()
	targetPath := t.TempDir()

	err := client.Clone(ctx, "https://github.com/nonexistent-org/nonexistent-repo.git", targetPath)

	// Must fail — no credentials, no network, plain git clone.
	if err == nil {
		t.Fatal("expected Clone to fail without credentials, got nil error")
	}

	// Must be a clone_failed error — env_credentials_not_found must NOT be
	// propagated; it is a sentinel that means "proceed without auth".
	if !isCloneFailedError(err) {
		var gitErr *outbound.GitOperationError
		if errors.As(err, &gitErr) {
			t.Errorf(
				"expected clone_failed for plain clone path, got error type %q — "+
					"env_credentials_not_found must trigger fall-through to plain clone, not be returned",
				gitErr.Type,
			)
		} else {
			t.Errorf("expected *outbound.GitOperationError with type clone_failed, got %T: %v", err, err)
		}
	}
}

// TestCloneAuthDispatch_InvalidTokenFormat_PropagatesError verifies that when
// LoadCredentialsFromEnvironment returns a non-"env_credentials_not_found" error
// (e.g., "env_token_invalid" for a malformed token), Clone() must propagate that
// error upward immediately instead of falling through to a plain clone.
//
// Current failure: Clone() ignores LoadCredentialsFromEnvironment entirely,
// runs a real git clone, and returns clone_failed instead of env_token_invalid.
func TestCloneAuthDispatch_InvalidTokenFormat_PropagatesError(t *testing.T) {
	clearAllAuthEnvVars(t)
	// "invalid-token-format" is the sentinel value that causes LoadCredentialsFromEnvironment
	// to return an env_token_invalid error (see the InvalidTokenFormat constant).
	t.Setenv("GIT_TOKEN", InvalidTokenFormat)

	client := newDispatchTestClient(t)
	ctx := context.Background()
	targetPath := t.TempDir()

	err := client.Clone(ctx, "https://github.com/nonexistent-org/nonexistent-repo.git", targetPath)

	if err == nil {
		t.Fatal("expected Clone to propagate token-invalid error, got nil")
	}

	// The error must be "env_token_invalid", not "clone_failed".
	// "clone_failed" here means Clone silently fell through to a plain git clone
	// despite encountering a real credential format error — that is wrong behavior.
	if !isEnvCredentialLoadError(err, "env_token_invalid") {
		var gitErr *outbound.GitOperationError
		if errors.As(err, &gitErr) {
			t.Errorf(
				"Clone() must propagate env_token_invalid from LoadCredentialsFromEnvironment, "+
					"got %q — non-not-found credential errors must be returned immediately, "+
					"not silently fallen through to plain clone",
				gitErr.Type,
			)
		} else {
			t.Errorf(
				"expected *outbound.GitOperationError with type env_token_invalid, got %T: %v",
				err, err,
			)
		}
	}
}

// TestCloneAuthDispatch_CredentialDispatchPriority verifies that provider-specific
// tokens take priority over generic GIT_TOKEN when multiple env vars are set,
// and that Clone() dispatches through the highest-priority credential.
//
// For a github.com URL with both GITHUB_TOKEN and GIT_TOKEN set:
//   - LoadCredentialsFromEnvironment must return the GITHUB_TOKEN config (provider=github)
//   - Clone() must dispatch to CloneWithTokenAuth with that config
//   - Result: stub success → Clone returns nil
//
// This test confirms the priority contract of LoadCredentialsFromEnvironment is
// respected by Clone()'s dispatch logic.
func TestCloneAuthDispatch_CredentialDispatchPriority(t *testing.T) {
	clearAllAuthEnvVars(t)
	t.Setenv("GITHUB_TOKEN", "ghp_prioritytoken123456789abcdef123456")
	t.Setenv("GIT_TOKEN", "generic-lower-priority-token")

	client := newDispatchTestClient(t)
	ctx := context.Background()

	// First verify that LoadCredentialsFromEnvironment itself returns the correct
	// priority — if this assertion fails the dispatch test would be misleading.
	resolved, credErr := client.LoadCredentialsFromEnvironment(
		ctx,
		"https://github.com/nonexistent-org/nonexistent-repo.git",
	)
	if credErr != nil {
		t.Fatalf(
			"LoadCredentialsFromEnvironment must return GITHUB_TOKEN for github.com URL, got error: %v",
			credErr,
		)
	}
	tokenCfg, ok := resolved.(*outbound.TokenAuthConfig)
	if !ok {
		t.Fatalf("expected *outbound.TokenAuthConfig from LoadCredentialsFromEnvironment, got %T", resolved)
	}
	if tokenCfg.Provider != "github" {
		t.Errorf(
			"LoadCredentialsFromEnvironment must return provider 'github' for github.com URL "+
				"when GITHUB_TOKEN is set, got %q — this is a prerequisite for dispatch priority",
			tokenCfg.Provider,
		)
	}

	// Now verify Clone() dispatches through the highest-priority credential.
	// After refactor: CloneWithTokenAuth stub returns success → Clone returns nil.
	// Current: real git clone runs and returns an error.
	targetPath := t.TempDir()
	err := client.Clone(ctx, "https://github.com/nonexistent-org/nonexistent-repo.git", targetPath)
	if err != nil {
		t.Errorf(
			"Clone() must delegate to CloneWithTokenAuth (stub success) when GITHUB_TOKEN is set, "+
				"but got error: %v — Clone() is not dispatching through LoadCredentialsFromEnvironment",
			err,
		)
	}
}

// TestCloneAuthDispatch_TokenAuthConfig_OldInlineInjectionAbsent is a direct
// regression test for the specific old behavior being replaced.
//
// The old Clone() implementation:
//  1. Reads GITLAB_TOKEN via os.Getenv directly
//  2. Builds a cmd.Env with GIT_CONFIG_COUNT/GIT_CONFIG_KEY_0/GIT_CONFIG_VALUE_0
//  3. Runs exec.Command("git", "clone", ...) with those env vars on the child process
//
// The new Clone() must NOT do step 2-3 itself — it must delegate to CloneWithTokenAuth.
//
// Observable difference: old path fails with "clone_failed" (real git runs);
// new path succeeds with nil (CloneWithTokenAuth stub returns mock result).
func TestCloneAuthDispatch_TokenAuthConfig_OldInlineInjectionAbsent(t *testing.T) {
	clearAllAuthEnvVars(t)
	t.Setenv("GITLAB_TOKEN", "glpat-refactortest123456789")

	client := newDispatchTestClient(t)
	ctx := context.Background()
	targetPath := t.TempDir()

	err := client.Clone(ctx, "https://gitlab.com/nonexistent-org/nonexistent-repo.git", targetPath)
	// After refactor: Clone → LoadCredentialsFromEnvironment → TokenAuthConfig
	//               → CloneWithTokenAuth(stub) → returns nil
	//
	// Current (before refactor): Clone → os.Getenv("GITLAB_TOKEN") → injects
	//   GIT_CONFIG_* onto cmd.Env → exec.Command git clone → fails → clone_failed error
	if err != nil {
		t.Errorf(
			"Clone() with GITLAB_TOKEN must delegate to CloneWithTokenAuth (stub success, nil error), "+
				"got: %v — this confirms the old inline GIT_CONFIG injection path is still active "+
				"instead of dispatching through LoadCredentialsFromEnvironment",
			err,
		)
	}
}
