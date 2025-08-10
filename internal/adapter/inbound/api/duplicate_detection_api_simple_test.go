package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codechunking/internal/application/dto"
	domain_errors "codechunking/internal/domain/errors/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestRepositoryHandler_CreateRepository_DuplicateDetection tests HTTP 409 Conflict responses for duplicates
func TestRepositoryHandler_CreateRepository_DuplicateDetection(t *testing.T) {
	tests := []struct {
		name                 string
		requestURL           string
		serviceError         error
		expectedStatusCode   int
		expectedErrorCode    string
		expectedErrorMessage string
		description          string
	}{
		{
			name:                 "returns_409_for_exact_duplicate",
			requestURL:           "https://github.com/owner/repo",
			serviceError:         domain_errors.ErrRepositoryAlreadyExists,
			expectedStatusCode:   http.StatusConflict,
			expectedErrorCode:    "REPOSITORY_ALREADY_EXISTS",
			expectedErrorMessage: "Repository already exists",
			description:          "Should return 409 Conflict when exact duplicate is detected",
		},
		{
			name:                 "returns_409_for_normalized_duplicate_with_git_suffix",
			requestURL:           "https://github.com/owner/repo.git",
			serviceError:         domain_errors.ErrRepositoryAlreadyExists,
			expectedStatusCode:   http.StatusConflict,
			expectedErrorCode:    "REPOSITORY_ALREADY_EXISTS",
			expectedErrorMessage: "Repository already exists",
			description:          "Should return 409 Conflict when duplicate detected via normalization (git suffix)",
		},
		{
			name:                 "returns_409_for_normalized_duplicate_with_case_difference",
			requestURL:           "https://GitHub.com/owner/repo",
			serviceError:         domain_errors.ErrRepositoryAlreadyExists,
			expectedStatusCode:   http.StatusConflict,
			expectedErrorCode:    "REPOSITORY_ALREADY_EXISTS",
			expectedErrorMessage: "Repository already exists",
			description:          "Should return 409 Conflict when duplicate detected via normalization (case difference)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &MockRepositoryService{}
			errorHandler := NewDefaultErrorHandler()
			handler := NewRepositoryHandler(mockService, errorHandler)

			// Setup mock to return the specified service error
			mockService.On("CreateRepository", mock.Anything, mock.AnythingOfType("dto.CreateRepositoryRequest")).
				Return(nil, tt.serviceError)

			// Create request
			desc := "Test repository"
			requestBody := dto.CreateRepositoryRequest{
				URL:         tt.requestURL,
				Name:        "test-repo",
				Description: &desc,
			}
			bodyBytes, err := json.Marshal(requestBody)
			require.NoError(t, err, "Should be able to marshal request body")

			req := httptest.NewRequest(http.MethodPost, "/repositories", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Execute
			w := httptest.NewRecorder()
			handler.CreateRepository(w, req)

			// Verify status code
			assert.Equal(t, tt.expectedStatusCode, w.Code, tt.description)

			// Verify response body structure
			var response dto.ErrorResponse
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err, "Should be able to unmarshal error response")

			assert.Equal(t, tt.expectedErrorCode, response.Error, "Error code should match expected")
			assert.Contains(t, response.Message, tt.expectedErrorMessage, "Error message should contain expected text")

			// Verify Content-Type header
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "Should return JSON content type")

			// Verify mock expectations
			mockService.AssertExpectations(t)
		})
	}
}

// MockRepositoryService implements the RepositoryService interface for testing
type MockRepositoryService struct {
	mock.Mock
}

func (m *MockRepositoryService) CreateRepository(ctx context.Context, request dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.RepositoryResponse), args.Error(1)
}

func (m *MockRepositoryService) GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.RepositoryResponse), args.Error(1)
}

func (m *MockRepositoryService) ListRepositories(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.RepositoryListResponse), args.Error(1)
}

func (m *MockRepositoryService) DeleteRepository(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepositoryService) GetRepositoryJobs(ctx context.Context, repositoryID uuid.UUID, query dto.IndexingJobListQuery) (*dto.IndexingJobListResponse, error) {
	args := m.Called(ctx, repositoryID, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.IndexingJobListResponse), args.Error(1)
}

func (m *MockRepositoryService) GetIndexingJob(ctx context.Context, repositoryID, jobID uuid.UUID) (*dto.IndexingJobResponse, error) {
	args := m.Called(ctx, repositoryID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.IndexingJobResponse), args.Error(1)
}

// Helper function - using stringPtr defined elsewhere
