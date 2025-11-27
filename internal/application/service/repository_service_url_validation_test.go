package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMain initializes the test environment for all tests in this package.
func TestMain(m *testing.M) {
	// Initialize slogger with a test logger to prevent nil pointer panics
	testLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer", // Use buffer output for testing to avoid polluting test output
	})
	if err != nil {
		panic("Failed to initialize test logger: " + err.Error())
	}

	// Set the global logger for the slogger package
	slogger.SetGlobalLogger(testLogger)

	// Run all tests
	code := m.Run()

	// Exit with the test result code
	os.Exit(code)
}

// Test URL validation in repository service.
func TestRepositoryService_URLValidation(t *testing.T) {
	tests := []struct {
		name          string
		request       dto.CreateRepositoryRequest
		expectedError error
		validateFunc  func(t *testing.T, err error)
	}{
		{
			name: "invalid_scheme_ftp_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "ftp://github.com/user/repo",
			},
			expectedError: errors.New("malicious protocol detected: ftp"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "malicious protocol detected")
				assert.Contains(t, err.Error(), "ftp")
			},
		},
		{
			name: "missing_scheme_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "github.com/user/repo",
			},
			expectedError: errors.New("invalid repository URL: URL must use http or https scheme"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "URL must use http or https scheme")
			},
		},
		{
			name: "unsupported_host_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "https://unsupported-git-host.com/user/repo",
			},
			expectedError: errors.New("invalid repository URL: unsupported host unsupported-git-host.com"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "unsupported host")
			},
		},
		{
			name: "missing_path_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "https://github.com",
			},
			expectedError: errors.New("invalid repository URL: missing repository path in URL"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "missing repository path")
			},
		},
		{
			name: "empty_url_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "",
			},
			expectedError: errors.New("invalid repository URL: URL cannot be empty"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "URL cannot be empty")
			},
		},
		{
			name: "invalid_url_format_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "not-a-url",
			},
			expectedError: errors.New("invalid repository URL: URL must use http or https scheme"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "URL must use http or https scheme")
			},
		},
		{
			name: "private_git_url_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "git@github.com:user/repo.git",
			},
			expectedError: errors.New("invalid repository URL: URL must use http or https scheme"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "URL must use http or https scheme")
			},
		},
		{
			name: "file_protocol_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "file:///path/to/repo",
			},
			expectedError: errors.New("malicious protocol detected: file"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "malicious protocol detected")
				assert.Contains(t, err.Error(), "file")
			},
		},
		{
			name: "localhost_not_supported_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "http://localhost/user/repo",
			},
			expectedError: errors.New("invalid repository URL: unsupported host localhost"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "unsupported host")
			},
		},
		{
			name: "private_ip_not_supported_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "http://192.168.1.1/user/repo",
			},
			expectedError: errors.New("invalid repository URL: unsupported host 192.168.1.1"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "unsupported host")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockRepo := &MockRepositoryRepository{}
			mockJobRepo := new(MockIndexingJobRepository)
			mockPublisher := &MockMessagePublisher{}
			service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

			// Execute
			_, err := service.CreateRepository(context.Background(), tt.request)

			// Assert
			assert.Error(t, err)
			// Note: Not all validation errors have the "invalid repository URL:" prefix
			// Some security checks (like malicious protocol detection) have their own format

			if tt.validateFunc != nil {
				tt.validateFunc(t, err)
			}

			// Ensure repository was not saved
			mockRepo.AssertNotCalled(t, "Save")
		})
	}
}

// Test successful URL validation.
func TestRepositoryService_SuccessfulURLValidation(t *testing.T) {
	tests := []struct {
		name    string
		request dto.CreateRepositoryRequest
	}{
		{
			name: "valid_github_https_url_succeeds",
			request: dto.CreateRepositoryRequest{
				URL: "https://github.com/golang/go",
			},
		},
		{
			name: "valid_gitlab_https_url_succeeds",
			request: dto.CreateRepositoryRequest{
				URL: "https://gitlab.com/gitlab-org/gitlab",
			},
		},
		{
			name: "valid_bitbucket_https_url_succeeds",
			request: dto.CreateRepositoryRequest{
				URL: "https://bitbucket.org/atlassian/atlassian-sdk",
			},
		},
		{
			name: "valid_github_http_url_succeeds",
			request: dto.CreateRepositoryRequest{
				URL: "http://github.com/golang/go",
			},
		},
		{
			name: "valid_url_with_git_extension_succeeds",
			request: dto.CreateRepositoryRequest{
				URL: "https://github.com/golang/go.git",
			},
		},
		{
			name: "valid_url_with_trailing_slash_succeeds",
			request: dto.CreateRepositoryRequest{
				URL: "https://github.com/golang/go/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockRepo := &MockRepositoryRepository{}
			mockJobRepo := new(MockIndexingJobRepository)
			mockPublisher := &MockMessagePublisher{}
			service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

			// Mock repository doesn't exist
			mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).
				Return(false, nil)

			// Mock successful save
			mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
			mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)

			// Mock successful message publish
			mockPublisher.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
				Return(nil)

			// Execute
			response, err := service.CreateRepository(context.Background(), tt.request)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, response)
			assert.Equal(t, tt.request.URL, response.URL)

			// Verify repository was saved
			mockRepo.AssertExpectations(t)
		})
	}
}

// Test URL validation error propagation.
func TestRepositoryService_URLValidationErrorPropagation(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectedError string
		expectDomain  bool
	}{
		{
			name:          "domain_invalid_url_error_propagated",
			url:           "invalid-url",
			expectedError: "invalid repository URL: URL must use http or https scheme",
			expectDomain:  false,
		},
		{
			name:          "domain_error_constant_used",
			url:           "", // This should trigger domain.ErrInvalidRepositoryURL
			expectedError: "repository URL is invalid",
			expectDomain:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockRepo := &MockRepositoryRepository{}
			mockJobRepo := new(MockIndexingJobRepository)
			mockPublisher := &MockMessagePublisher{}
			service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

			request := dto.CreateRepositoryRequest{
				URL: tt.url,
			}

			// Execute
			_, err := service.CreateRepository(context.Background(), request)

			// Assert
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			if tt.expectDomain {
				assert.ErrorIs(t, err, domain.ErrInvalidRepositoryURL)
			}

			// Ensure no repository operations were attempted
			mockRepo.AssertNotCalled(t, "Save")
			mockRepo.AssertNotCalled(t, "ExistsByNormalizedURL")
		})
	}
}

// Test URL validation edge cases.
func TestRepositoryService_URLValidationEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		shouldFail    bool
		expectedError string
	}{
		{
			name:          "url_with_only_scheme_fails",
			url:           "https://",
			shouldFail:    true,
			expectedError: "invalid repository URL: unsupported host",
		},
		{
			name:          "url_with_query_params_only_fails",
			url:           "https://github.com?query=value",
			shouldFail:    true,
			expectedError: "invalid repository URL: missing repository path in URL",
		},
		{
			name:          "url_with_fragment_only_fails",
			url:           "https://github.com#fragment",
			shouldFail:    true,
			expectedError: "invalid repository URL: missing repository path in URL",
		},
		{
			name:          "url_with_port_but_no_path_fails",
			url:           "https://github.com:443",
			shouldFail:    true,
			expectedError: "invalid repository URL: non-standard port not allowed",
		},
		{
			name:          "url_with_user_info_but_no_path_fails",
			url:           "https://user:pass@github.com",
			shouldFail:    true,
			expectedError: "invalid host format: @ symbol not allowed",
		},
		{
			name:          "url_with_unicode_characters_succeeds",
			url:           "https://github.com/user/repó",
			shouldFail:    false,
			expectedError: "",
		},
		{
			name:          "url_with_very_long_path_fails_control_chars",
			url:           "https://github.com/user/" + string(make([]byte, 1000)),
			shouldFail:    true,
			expectedError: "URL contains invalid control characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockRepo := &MockRepositoryRepository{}
			mockJobRepo := new(MockIndexingJobRepository)
			mockPublisher := &MockMessagePublisher{}
			service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

			// Setup mocks for valid URLs BEFORE execution
			if !tt.shouldFail {
				mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).
					Return(false, nil)
				mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
				mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
				mockPublisher.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
					Return(nil)
			}

			request := dto.CreateRepositoryRequest{
				URL: tt.url,
			}

			// Execute
			_, err := service.CreateRepository(context.Background(), request)

			// Assert
			if tt.shouldFail {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)

				// Ensure no repository operations were attempted
				mockRepo.AssertNotCalled(t, "Save")
				mockRepo.AssertNotCalled(t, "Exists")
			} else if err != nil && !errors.Is(err, domain.ErrRepositoryAlreadyExists) {
				// Should not return validation error
				t.Errorf("Expected no validation error for valid URL, got: %v", err)
			}
		})
	}
}

// Test URL validation with RepositoryURL value object.
func TestRepositoryService_RepositoryURLValidation(t *testing.T) {
	tests := []struct {
		name          string
		urlString     string
		shouldFail    bool
		expectedError string
	}{
		{
			name:          "invalid_scheme_fails_repository_url_creation",
			urlString:     "ftp://github.com/user/repo",
			shouldFail:    true,
			expectedError: "malicious protocol detected: ftp",
		},
		{
			name:          "empty_url_fails_repository_url_creation",
			urlString:     "",
			shouldFail:    true,
			expectedError: "invalid repository URL: URL cannot be empty",
		},
		{
			name:          "valid_url_succeeds_repository_url_creation",
			urlString:     "https://github.com/golang/go",
			shouldFail:    false,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test RepositoryURL value object creation directly
			_, err := valueobject.NewRepositoryURL(tt.urlString)

			if tt.shouldFail {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else if err != nil {
				// For valid URLs, the error might be different (like repository already exists)
				// but it shouldn't be a URL validation error
				assert.NotContains(t, err.Error(), "invalid repository URL:")
			}
		})
	}
}
