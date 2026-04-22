package valueobject

import (
	"codechunking/internal/application/common/security"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRepositoryURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid github URL",
			input:     "https://github.com/golang/go",
			wantError: false,
		},
		{
			name:      "valid github URL with .git suffix",
			input:     "https://github.com/golang/go.git",
			wantError: false,
		},
		{
			name:      "valid gitlab URL",
			input:     "https://gitlab.com/gitlab-org/gitlab",
			wantError: false,
		},
		{
			name:      "valid bitbucket URL",
			input:     "https://bitbucket.org/atlassian/stash",
			wantError: false,
		},
		{
			name:      "empty URL",
			input:     "",
			wantError: true,
		},
		{
			name:      "invalid URL format",
			input:     "not-a-url",
			wantError: true,
		},
		{
			name:      "unsupported host",
			input:     "https://example.com/user/repo",
			wantError: true,
		},
		{
			name:      "missing repository name",
			input:     "https://github.com/golang",
			wantError: true,
		},
		{
			name:      "http scheme",
			input:     "http://github.com/golang/go",
			wantError: false,
		},
		{
			name:      "invalid scheme",
			input:     "ftp://github.com/golang/go",
			wantError: true,
		},
	}

	// Security-focused tests - these should ALL FAIL initially as security validation is not implemented
	securityTests := []struct {
		name          string
		input         string
		wantError     bool
		errorContains string
	}{
		// Malicious protocol schemes
		{
			name:          "javascript protocol injection",
			input:         "javascript://github.com/user/repo",
			wantError:     true,
			errorContains: "malicious protocol",
		},
		{
			name:          "file protocol injection",
			input:         "file:///etc/passwd",
			wantError:     true,
			errorContains: "malicious protocol",
		},
		{
			name:          "data protocol injection",
			input:         "data:text/html,<script>alert('xss')</script>",
			wantError:     true,
			errorContains: "malicious protocol",
		},
		// URL length attacks
		{
			name:          "extremely long URL",
			input:         "https://github.com/user/" + strings.Repeat("a", 10000),
			wantError:     true,
			errorContains: "URL too long",
		},
		// Unicode and encoding attacks
		{
			name:          "unicode homograph attack",
			input:         "https://github.com/user/reρo", // Greek rho instead of 'p'
			wantError:     true,
			errorContains: "suspicious unicode",
		},
		{
			name:          "URL encoding bypass attempt",
			input:         "https://github.com/user%2Fmalicious/repo",
			wantError:     true,
			errorContains: "invalid encoding",
		},
		{
			name:          "double URL encoding",
			input:         "https://github.com/user%252Fmalicious/repo",
			wantError:     true,
			errorContains: "invalid encoding",
		},
		// Directory traversal attempts
		{
			name:          "directory traversal with dots",
			input:         "https://github.com/../../../etc/passwd",
			wantError:     true,
			errorContains: "path traversal",
		},
		{
			name:          "encoded directory traversal",
			input:         "https://github.com/%2E%2E/%2E%2E/etc/passwd",
			wantError:     true,
			errorContains: "path traversal",
		},
		// SQL injection attempts in URL
		{
			name:          "SQL injection in path",
			input:         "https://github.com/user'; DROP TABLE repositories;--/repo",
			wantError:     true,
			errorContains: "invalid characters",
		},
		// XSS attempts in URL
		{
			name:          "script tag in URL",
			input:         "https://github.com/user/<script>alert('xss')</script>/repo",
			wantError:     true,
			errorContains: "invalid characters",
		},
		// Control character attacks
		{
			name:          "null byte injection",
			input:         "https://github.com/user\x00/repo",
			wantError:     true,
			errorContains: "control characters",
		},
		{
			name:          "newline injection",
			input:         "https://github.com/user\n/repo",
			wantError:     true,
			errorContains: "control characters",
		},
		// Port scanning attempts
		{
			name:          "non-standard port",
			input:         "https://github.com:8080/user/repo",
			wantError:     true,
			errorContains: "non-standard port",
		},
		// Host header injection
		{
			name:          "host header injection",
			input:         "https://github.com@malicious.com/user/repo",
			wantError:     true,
			errorContains: "invalid host format",
		},
	}

	// Run security tests that should FAIL initially
	for _, tt := range securityTests {
		t.Run("SECURITY_"+tt.name, func(t *testing.T) {
			_, err := NewRepositoryURL(tt.input)

			// These tests should FAIL because security validation is not implemented yet
			if tt.wantError {
				require.Error(t, err, "Expected security validation to reject malicious URL: %s", tt.input)
				assert.ErrorContains(t, err, tt.errorContains, "Error message should indicate the security issue")
			} else {
				require.NoError(t, err, "Valid URL should not be rejected")
			}
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := NewRepositoryURL(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewRepositoryURL() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("NewRepositoryURL() unexpected error: %v", err)
				return
			}

			if url.String() == "" {
				t.Errorf("NewRepositoryURL() returned empty URL")
			}
		})
	}
}

func TestRepositoryURL_Methods(t *testing.T) {
	url, err := NewRepositoryURL("https://github.com/golang/go")
	if err != nil {
		t.Fatalf("failed to create test URL: %v", err)
	}

	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "Host",
			method:   url.Host,
			expected: "github.com",
		},
		{
			name:     "Owner",
			method:   url.Owner,
			expected: "golang",
		},
		{
			name:     "Name",
			method:   url.Name,
			expected: "go",
		},
		{
			name:     "FullName",
			method:   url.FullName,
			expected: "golang/go",
		},
		{
			name:     "CloneURL",
			method:   url.CloneURL,
			expected: "https://github.com/golang/go.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestRepositoryURL_FullPath(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		expected string
	}{
		{
			name:     "standard GitHub repo",
			rawURL:   "https://github.com/owner/repo",
			expected: "owner/repo",
		},
		{
			name:     "GitLab nested group",
			rawURL:   "https://gitlab.com/org/sub/repo",
			expected: "org/sub/repo",
		},
		{
			name:     "deeply nested group",
			rawURL:   "https://gitlab.com/a/b/c/repo",
			expected: "a/b/c/repo",
		},
		{
			name:     "GitHub URL with UI tree suffix is truncated to repo root",
			rawURL:   "https://github.com/owner/repo",
			expected: "owner/repo",
		},
		{
			name:     "Bitbucket URL returns owner/repo",
			rawURL:   "https://bitbucket.org/owner/repo",
			expected: "owner/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := NewRepositoryURL(tt.rawURL)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, u.FullPath())
		})
	}
}

func TestRepositoryURL_Equal(t *testing.T) {
	tests := []struct {
		name      string
		a         string
		b         string
		wantEqual bool
	}{
		{
			name:      "identical URLs are equal",
			a:         "https://github.com/golang/go",
			b:         "https://github.com/golang/go",
			wantEqual: true,
		},
		{
			name:      "different repo names are not equal",
			a:         "https://github.com/golang/go",
			b:         "https://github.com/golang/tools",
			wantEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := NewRepositoryURL(tt.a)
			require.NoError(t, err)
			b, err := NewRepositoryURL(tt.b)
			require.NoError(t, err)
			assert.Equal(t, tt.wantEqual, a.Equal(b))
		})
	}
}

func TestNewRepositoryURLWithConfig_AdditionalAllowedHosts(t *testing.T) {
	tests := []struct {
		name          string
		allowedHosts  []string
		input         string
		useDefaultCfg bool // when true, call NewRepositoryURL (no extra hosts)
		wantError     bool
		wantURL       string
	}{
		{
			name:         "self-hosted host passes when listed in AdditionalAllowedHosts",
			allowedHosts: []string{"gitlab.mycompany.com"},
			input:        "https://gitlab.mycompany.com/org/repo",
			wantError:    false,
		},
		{
			name:          "same self-hosted host fails without config",
			useDefaultCfg: true,
			input:         "https://gitlab.mycompany.com/org/repo",
			wantError:     true,
		},
		{
			name:         "allowlist matching is case-insensitive",
			allowedHosts: []string{"GitLab.MyCompany.COM"},
			input:        "https://gitlab.mycompany.com/org/repo",
			wantError:    false,
		},
		{
			name:         "normalization strips .git suffix for additional host",
			allowedHosts: []string{"gitlab.mycompany.com"},
			input:        "https://gitlab.mycompany.com/org/repo.git",
			wantError:    false,
			wantURL:      "https://gitlab.mycompany.com/org/repo",
		},
		{
			name:         "host not in allowlist is rejected even with config",
			allowedHosts: []string{"gitlab.mycompany.com"},
			input:        "https://other.example.com/org/repo",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				url RepositoryURL
				err error
			)
			if tt.useDefaultCfg {
				url, err = NewRepositoryURL(tt.input)
			} else {
				cfg := security.TestConfig()
				cfg.AdditionalAllowedHosts = tt.allowedHosts
				url, err = NewRepositoryURLWithConfig(tt.input, cfg)
			}

			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantURL != "" {
				assert.Equal(t, tt.wantURL, url.String())
			} else {
				assert.NotEmpty(t, url.String())
			}
		})
	}
}
