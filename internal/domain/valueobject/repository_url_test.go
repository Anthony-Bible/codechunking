package valueobject

import (
	"testing"
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

func TestRepositoryURL_Equal(t *testing.T) {
	url1, _ := NewRepositoryURL("https://github.com/golang/go")
	url2, _ := NewRepositoryURL("https://github.com/golang/go")
	url3, _ := NewRepositoryURL("https://github.com/golang/tools")

	if !url1.Equal(url2) {
		t.Errorf("Equal URLs should be equal")
	}

	if url1.Equal(url3) {
		t.Errorf("Different URLs should not be equal")
	}
}
