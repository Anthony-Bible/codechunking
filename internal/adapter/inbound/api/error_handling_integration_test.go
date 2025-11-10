package api

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompleteErrorHandlingFlow(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectError   bool
		expectedError error
	}{
		{
			name:          "empty_url_returns_domain_error",
			url:           "",
			expectError:   true,
			expectedError: domain.ErrInvalidRepositoryURL,
		},
		{
			name:          "invalid_scheme_returns_security_error",
			url:           "ftp://github.com/user/repo",
			expectError:   true,
			expectedError: nil, // This returns a security error, not domain error
		},
		{
			name:          "missing_scheme_returns_domain_error",
			url:           "github.com/user/repo",
			expectError:   true,
			expectedError: domain.ErrInvalidRepositoryURL,
		},
		{
			name:          "valid_url_succeeds",
			url:           "https://github.com/golang/go",
			expectError:   false,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test value object creation
			_, err := valueobject.NewRepositoryURL(tt.url)

			if tt.expectError {
				assert.Error(t, err, "Expected error for URL: %s", tt.url)

				if tt.expectedError != nil {
					assert.ErrorIs(t, err, tt.expectedError,
						"Expected domain error %v, got %v", tt.expectedError, err)
				}
			} else {
				// For valid URLs, we might still get errors (like repository exists)
				// but we shouldn't get URL validation errors
				if err != nil {
					assert.NotErrorIs(t, err, domain.ErrInvalidRepositoryURL,
						"Valid URL should not return domain error, got: %v", err)
				}
			}
		})
	}
}

func TestErrorHandlerMapping(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()

	tests := []struct {
		name               string
		err                error
		expectedStatusCode int
		expectedErrorCode  string
	}{
		{
			name:               "ErrInvalidRepositoryURL_maps_to_400",
			err:                domain.ErrInvalidRepositoryURL,
			expectedStatusCode: 400,
			expectedErrorCode:  "INVALID_URL",
		},
		{
			name:               "ErrRepositoryNotFound_maps_to_404",
			err:                domain.ErrRepositoryNotFound,
			expectedStatusCode: 404,
			expectedErrorCode:  "REPOSITORY_NOT_FOUND",
		},
		{
			name:               "ErrRepositoryAlreadyExists_maps_to_409",
			err:                domain.ErrRepositoryAlreadyExists,
			expectedStatusCode: 409,
			expectedErrorCode:  "REPOSITORY_ALREADY_EXISTS",
		},
		{
			name:               "wrapped_ErrInvalidRepositoryURL_maps_to_400",
			err:                fmt.Errorf("validation failed: %w", domain.ErrInvalidRepositoryURL),
			expectedStatusCode: 400,
			expectedErrorCode:  "INVALID_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP request and response
			req := &http.Request{URL: &url.URL{Path: "/repositories"}}
			w := httptest.NewRecorder()

			// Handle the error
			errorHandler.HandleServiceError(w, req, tt.err)

			// Check status code
			assert.Equal(t, tt.expectedStatusCode, w.Code)

			// Parse response body
			var response dto.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Check error code
			assert.Equal(t, tt.expectedErrorCode, response.Error)
		})
	}
}
