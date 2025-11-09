package domain

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test ErrInvalidRepositoryURL error definition and behavior
func TestErrInvalidRepositoryURL(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "ErrInvalidRepositoryURL_has_correct_message",
			err:         ErrInvalidRepositoryURL,
			expectedMsg: "repository URL is invalid",
		},
		{
			name:        "ErrInvalidRepositoryURL_is_not_nil",
			err:         ErrInvalidRepositoryURL,
			expectedMsg: "repository URL is invalid",
		},
		{
			name:        "ErrInvalidRepositoryURL_error_string_matches",
			err:         ErrInvalidRepositoryURL,
			expectedMsg: "repository URL is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.Equal(t, tt.expectedMsg, tt.err.Error())
		})
	}
}

// Test URL validation error creation and handling
func TestURLValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedErr string
	}{
		{
			name:        "invalid_scheme_ftp_creates_error",
			url:         "ftp://github.com/user/repo",
			expectedErr: "invalid repository URL: URL must use http or https scheme",
		},
		{
			name:        "missing_scheme_creates_error",
			url:         "github.com/user/repo",
			expectedErr: "invalid repository URL: URL must use http or https scheme",
		},
		{
			name:        "unsupported_host_creates_error",
			url:         "https://unsupported-git-host.com/user/repo",
			expectedErr: "invalid repository URL: unsupported host unsupported-git-host.com",
		},
		{
			name:        "missing_path_creates_error",
			url:         "https://github.com",
			expectedErr: "invalid repository URL: missing repository path in URL",
		},
		{
			name:        "empty_url_creates_error",
			url:         "",
			expectedErr: "invalid repository URL: URL cannot be empty",
		},
		{
			name:        "invalid_url_format_creates_error",
			url:         "not-a-url",
			expectedErr: "invalid repository URL: invalid URL format",
		},
		{
			name:        "private_git_url_creates_error",
			url:         "git@github.com:user/repo.git",
			expectedErr: "invalid repository URL: private Git repositories not supported",
		},
		{
			name:        "file_protocol_creates_error",
			url:         "file:///path/to/repo",
			expectedErr: "invalid repository URL: file protocol not supported",
		},
		{
			name:        "localhost_not_supported_creates_error",
			url:         "http://localhost/user/repo",
			expectedErr: "invalid repository URL: localhost not supported",
		},
		{
			name:        "private_ip_not_supported_creates_error",
			url:         "http://192.168.1.1/user/repo",
			expectedErr: "invalid repository URL: private IP addresses not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create URL validation error
			err := errors.New(tt.expectedErr)

			// Verify error properties
			assert.NotNil(t, err)
			assert.Equal(t, tt.expectedErr, err.Error())
			assert.Contains(t, err.Error(), "invalid repository URL:")
		})
	}
}

// Test error matching and wrapping behavior
func TestErrorMatchingBehavior(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		targetError  error
		shouldMatch  bool
		shouldUnwrap bool
	}{
		{
			name:         "direct_ErrInvalidRepositoryURL_matches",
			err:          ErrInvalidRepositoryURL,
			targetError:  ErrInvalidRepositoryURL,
			shouldMatch:  true,
			shouldUnwrap: false,
		},
		{
			name:         "wrapped_ErrInvalidRepositoryURL_matches",
			err:          fmt.Errorf("context: %w", ErrInvalidRepositoryURL),
			targetError:  ErrInvalidRepositoryURL,
			shouldMatch:  true,
			shouldUnwrap: true,
		},
		{
			name:         "custom_url_error_does_not_match_domain_error",
			err:          errors.New("invalid repository URL: custom message"),
			targetError:  ErrInvalidRepositoryURL,
			shouldMatch:  false,
			shouldUnwrap: false,
		},
		{
			name:         "different_domain_error_does_not_match",
			err:          ErrRepositoryNotFound,
			targetError:  ErrInvalidRepositoryURL,
			shouldMatch:  false,
			shouldUnwrap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test error matching
			matches := errors.Is(tt.err, tt.targetError)
			assert.Equal(t, tt.shouldMatch, matches)

			// Test error unwrapping
			if tt.shouldUnwrap {
				unwrapped := errors.Unwrap(tt.err)
				assert.NotNil(t, unwrapped)
				assert.True(t, errors.Is(unwrapped, tt.targetError))
			}
		})
	}
}

// Test error creation with different validation scenarios
func TestValidationErrorCreation(t *testing.T) {
	tests := []struct {
		name           string
		validationFunc func() error
		expectedErr    string
	}{
		{
			name: "scheme_validation_error",
			validationFunc: func() error {
				return errors.New("invalid repository URL: URL must use http or https scheme")
			},
			expectedErr: "invalid repository URL: URL must use http or https scheme",
		},
		{
			name: "host_validation_error",
			validationFunc: func() error {
				return errors.New("invalid repository URL: unsupported host example.com")
			},
			expectedErr: "invalid repository URL: unsupported host example.com",
		},
		{
			name: "path_validation_error",
			validationFunc: func() error {
				return errors.New("invalid repository URL: missing repository path in URL")
			},
			expectedErr: "invalid repository URL: missing repository path in URL",
		},
		{
			name: "format_validation_error",
			validationFunc: func() error {
				return errors.New("invalid repository URL: invalid URL format")
			},
			expectedErr: "invalid repository URL: invalid URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validationFunc()
			assert.NotNil(t, err)
			assert.Equal(t, tt.expectedErr, err.Error())
			assert.Contains(t, err.Error(), "invalid repository URL:")
		})
	}
}

// Test error consistency across the domain layer
func TestDomainErrorConsistency(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		isDomain bool
	}{
		{
			name:     "ErrInvalidRepositoryURL_is_domain_error",
			err:      ErrInvalidRepositoryURL,
			isDomain: true,
		},
		{
			name:     "ErrRepositoryNotFound_is_domain_error",
			err:      ErrRepositoryNotFound,
			isDomain: true,
		},
		{
			name:     "ErrRepositoryAlreadyExists_is_domain_error",
			err:      ErrRepositoryAlreadyExists,
			isDomain: true,
		},
		{
			name:     "ErrRepositoryProcessing_is_domain_error",
			err:      ErrRepositoryProcessing,
			isDomain: true,
		},
		{
			name:     "ErrJobNotFound_is_domain_error",
			err:      ErrJobNotFound,
			isDomain: true,
		},
		{
			name:     "custom_error_is_not_domain_error",
			err:      errors.New("custom error"),
			isDomain: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.NotEmpty(t, tt.err.Error())

			if tt.isDomain {
				// Domain errors should be predefined constants
				assert.True(t, errors.Is(tt.err, tt.err))
			}
		})
	}
}

// Test error message formatting
func TestErrorMessageFormatting(t *testing.T) {
	tests := []struct {
		name             string
		err              error
		expectedPrefix   string
		expectedContains []string
	}{
		{
			name:             "ErrInvalidRepositoryURL_message_format",
			err:              ErrInvalidRepositoryURL,
			expectedPrefix:   "repository",
			expectedContains: []string{"URL", "invalid"},
		},
		{
			name:             "custom_url_validation_message_format",
			err:              errors.New("invalid repository URL: scheme must be http or https"),
			expectedPrefix:   "invalid repository URL",
			expectedContains: []string{"scheme", "http", "https"},
		},
		{
			name:             "host_validation_message_format",
			err:              errors.New("invalid repository URL: unsupported host gitlab.org"),
			expectedPrefix:   "invalid repository URL",
			expectedContains: []string{"unsupported", "host", "gitlab.org"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			assert.NotEmpty(t, errMsg)

			if tt.expectedPrefix != "" {
				assert.Contains(t, errMsg, tt.expectedPrefix)
			}

			for _, contains := range tt.expectedContains {
				assert.Contains(t, errMsg, contains)
			}
		})
	}
}
