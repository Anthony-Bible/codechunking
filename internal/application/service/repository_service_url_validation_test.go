package service

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test URL validation in repository service
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
			expectedError: errors.New("invalid repository URL: URL must use http or https scheme"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "URL must use http or https scheme")
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
			expectedError: errors.New("invalid repository URL: invalid URL format"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "invalid URL format")
			},
		},
		{
			name: "private_git_url_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "git@github.com:user/repo.git",
			},
			expectedError: errors.New("invalid repository URL: private Git repositories not supported"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "private Git repositories not supported")
			},
		},
		{
			name: "file_protocol_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "file:///path/to/repo",
			},
			expectedError: errors.New("invalid repository URL: file protocol not supported"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "file protocol not supported")
			},
		},
		{
			name: "localhost_not_supported_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "http://localhost/user/repo",
			},
			expectedError: errors.New("invalid repository URL: localhost not supported"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "localhost not supported")
			},
		},
		{
			name: "private_ip_not_supported_returns_error",
			request: dto.CreateRepositoryRequest{
				URL: "http://192.168.1.1/user/repo",
			},
			expectedError: errors.New("invalid repository URL: private IP addresses not supported"),
			validateFunc: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid repository URL:")
				assert.Contains(t, err.Error(), "private IP addresses not supported")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockRepo := &MockRepositoryRepository{}
			mockPublisher := &MockMessagePublisher{}
			service := NewCreateRepositoryService(mockRepo, mockPublisher)

			// Execute
			_, err := service.CreateRepository(context.Background(), tt.request)

			// Assert
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid repository URL:")

			if tt.validateFunc != nil {
				tt.validateFunc(t, err)
			}

			// Ensure repository was not saved
			mockRepo.AssertNotCalled(t, "Save")
		})
	}
}

// Test successful URL validation
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
			mockPublisher := &MockMessagePublisher{}
			service := NewCreateRepositoryService(mockRepo, mockPublisher)

			// Mock repository doesn't exist
			mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)

			// Mock successful save
			mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)

			// Mock successful message publish
			mockPublisher.On("Publish", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

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

// Test URL validation error propagation
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
			expectedError: "invalid repository URL: invalid URL format",
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
			mockPublisher := &MockMessagePublisher{}
			service := NewCreateRepositoryService(mockRepo, mockPublisher)

			request := dto.CreateRepositoryRequest{
				URL: tt.url,
			}

			// Execute
			_, err := service.CreateRepository(context.Background(), request)

			// Assert
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			if tt.expectDomain {
				assert.True(t, errors.Is(err, domain.ErrInvalidRepositoryURL))
			}

			// Ensure no repository operations were attempted
			mockRepo.AssertNotCalled(t, "Save")
			mockRepo.AssertNotCalled(t, "ExistsByNormalizedURL")
		})
	}
}

// Test URL validation edge cases
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
			expectedError: "invalid repository URL: missing repository path in URL",
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
			expectedError: "invalid repository URL: missing repository path in URL",
		},
		{
			name:          "url_with_user_info_but_no_path_fails",
			url:           "https://user:pass@github.com",
			shouldFail:    true,
			expectedError: "invalid repository URL: missing repository path in URL",
		},
		{
			name:          "url_with_unicode_characters_succeeds",
			url:           "https://github.com/user/repó",
			shouldFail:    false,
			expectedError: "",
		},
		{
			name:          "url_with_very_long_path_succeeds",
			url:           "https://github.com/user/" + string(make([]byte, 1000)),
			shouldFail:    false,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockRepo := &MockRepositoryRepository{}
			mockPublisher := &MockMessagePublisher{}
			service := NewCreateRepositoryService(mockRepo, mockPublisher)

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
				mockRepo.AssertNotCalled(t, "ExistsByNormalizedURL")
			} else {
				// For valid URLs, mock the dependencies
				mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
				mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
				mockPublisher.On("Publish", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil)

				// Should not return validation error
				if err != nil && !errors.Is(err, domain.ErrRepositoryAlreadyExists) {
					t.Errorf("Expected no validation error for valid URL, got: %v", err)
				}
			}
		})
	}
}

// Test URL validation with RepositoryURL value object
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
			expectedError: "invalid repository URL: URL must use http or https scheme",
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
			} else {
				// For valid URLs, the error might be different (like repository already exists)
				// but it shouldn't be a URL validation error
				if err != nil {
					assert.NotContains(t, err.Error(), "invalid repository URL:")
				}
			}
		})
	}
}

// Test error handling consistency
func TestRepositoryService_ErrorHandlingConsistency(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockRepositoryRepository, *MockMessagePublisher)
		request        dto.CreateRepositoryRequest
		expectedError  error
		expectedStatus string
	}{
		{
			name: "url_validation_error_before_repository_check",
			setupMock: func(mockRepo *MockRepositoryRepository, mockPub *MockMessagePublisher) {
				// No mocks should be called for URL validation errors
			},
			request: dto.CreateRepositoryRequest{
				URL: "invalid-url",
			},
			expectedError:  errors.New("invalid repository URL: invalid URL format"),
			expectedStatus: "INVALID_URL",
		},
		{
			name: "repository_exists_error_after_url_validation",
			setupMock: func(mockRepo *MockRepositoryRepository, mockPub *MockMessagePublisher) {
				mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(true, nil)
			},
			request: dto.CreateRepositoryRequest{
				URL: "https://github.com/golang/go",
			},
			expectedError:  domain.ErrRepositoryAlreadyExists,
			expectedStatus: "REPOSITORY_ALREADY_EXISTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockRepo := &MockRepositoryRepository{}
			mockPublisher := &MockMessagePublisher{}
			tt.setupMock(mockRepo, mockPublisher)

			service := NewCreateRepositoryService(mockRepo, mockPublisher)

			// Execute
			_, err := service.CreateRepository(context.Background(), tt.request)

			// Assert
			assert.Error(t, err)

			if tt.expectedError != nil {
				if errors.Is(tt.expectedError, domain.ErrRepositoryAlreadyExists) {
					assert.True(t, errors.Is(err, domain.ErrRepositoryAlreadyExists))
				} else {
					assert.Contains(t, err.Error(), tt.expectedError.Error())
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
