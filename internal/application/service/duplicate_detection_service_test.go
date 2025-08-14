package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	domain_errors "codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRepositoryRepositoryWithNormalization extends the mock to support normalized URL checking.
type MockRepositoryRepositoryWithNormalization struct {
	outbound.RepositoryRepository
	mock.Mock
}

// ExistsByNormalizedURL checks if a repository exists using normalized URL comparison
// This method doesn't exist yet - test should FAIL.
func (m *MockRepositoryRepositoryWithNormalization) ExistsByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

// FindByNormalizedURL finds a repository using normalized URL comparison
// This method doesn't exist yet - test should FAIL.
func (m *MockRepositoryRepositoryWithNormalization) FindByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

// Mock implementation methods from the original interface.
func (m *MockRepositoryRepositoryWithNormalization) Save(ctx context.Context, repo *entity.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockRepositoryRepositoryWithNormalization) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*entity.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *MockRepositoryRepositoryWithNormalization) FindByURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *MockRepositoryRepositoryWithNormalization) FindAll(
	ctx context.Context,
	filters outbound.RepositoryFilters,
) ([]*entity.Repository, int, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*entity.Repository), args.Int(1), args.Error(2)
}

func (m *MockRepositoryRepositoryWithNormalization) Update(ctx context.Context, repo *entity.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockRepositoryRepositoryWithNormalization) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepositoryRepositoryWithNormalization) Exists(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

// TestCreateRepositoryService_DetectDuplicatesByNormalizedURL tests duplicate detection using normalized URLs
// This test will FAIL initially as normalized duplicate detection doesn't exist yet.
func TestCreateRepositoryService_DetectDuplicatesByNormalizedURL(t *testing.T) {
	tests := []struct {
		name                  string
		existingURL           string
		newURL                string
		shouldDetectDuplicate bool
		expectedError         error
		description           string
	}{
		{
			name:                  "detect_duplicate_with_git_suffix_variation",
			existingURL:           "https://github.com/owner/repo",
			newURL:                "https://github.com/owner/repo.git",
			shouldDetectDuplicate: true,
			expectedError:         domain_errors.ErrRepositoryAlreadyExists,
			description:           "Should detect duplicate when new URL has .git suffix but existing doesn't",
		},
		{
			name:                  "detect_duplicate_with_case_variation",
			existingURL:           "https://github.com/owner/repo",
			newURL:                "https://GitHub.com/owner/repo",
			shouldDetectDuplicate: true,
			expectedError:         domain_errors.ErrRepositoryAlreadyExists,
			description:           "Should detect duplicate when hostname case differs",
		},
		{
			name:                  "detect_duplicate_with_protocol_variation",
			existingURL:           "https://github.com/owner/repo",
			newURL:                "http://github.com/owner/repo",
			shouldDetectDuplicate: true,
			expectedError:         domain_errors.ErrRepositoryAlreadyExists,
			description:           "Should detect duplicate when protocol differs (http vs https)",
		},
		{
			name:                  "detect_duplicate_with_trailing_slash_variation",
			existingURL:           "https://github.com/owner/repo",
			newURL:                "https://github.com/owner/repo/",
			shouldDetectDuplicate: true,
			expectedError:         domain_errors.ErrRepositoryAlreadyExists,
			description:           "Should detect duplicate when new URL has trailing slash",
		},
		{
			name:                  "detect_duplicate_with_query_parameters",
			existingURL:           "https://github.com/owner/repo",
			newURL:                "https://github.com/owner/repo?branch=main",
			shouldDetectDuplicate: true,
			expectedError:         domain_errors.ErrRepositoryAlreadyExists,
			description:           "Should detect duplicate when new URL has query parameters",
		},
		{
			name:                  "detect_duplicate_with_fragment_identifier",
			existingURL:           "https://github.com/owner/repo",
			newURL:                "https://github.com/owner/repo#readme",
			shouldDetectDuplicate: true,
			expectedError:         domain_errors.ErrRepositoryAlreadyExists,
			description:           "Should detect duplicate when new URL has fragment identifier",
		},
		{
			name:                  "detect_duplicate_complex_variation",
			existingURL:           "https://github.com/owner/repo",
			newURL:                "HTTP://GitHub.COM/owner/repo.git/?branch=main#readme///",
			shouldDetectDuplicate: true,
			expectedError:         domain_errors.ErrRepositoryAlreadyExists,
			description:           "Should detect duplicate with complex URL variations",
		},
		{
			name:                  "allow_different_repositories_same_owner",
			existingURL:           "https://github.com/owner/repo1",
			newURL:                "https://github.com/owner/repo2",
			shouldDetectDuplicate: false,
			expectedError:         nil,
			description:           "Should allow different repositories from same owner",
		},
		{
			name:                  "allow_same_repo_name_different_owner",
			existingURL:           "https://github.com/owner1/repo",
			newURL:                "https://github.com/owner2/repo",
			shouldDetectDuplicate: false,
			expectedError:         nil,
			description:           "Should allow same repository name with different owner",
		},
		{
			name:                  "allow_different_hosting_providers",
			existingURL:           "https://github.com/owner/repo",
			newURL:                "https://gitlab.com/owner/repo",
			shouldDetectDuplicate: false,
			expectedError:         nil,
			description:           "Should allow same repo name on different hosting providers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctx := context.Background()
			mockRepo := &MockRepositoryRepositoryWithNormalization{}
			mockPublisher := &MockMessagePublisher{}

			// Create service with enhanced duplicate detection - this will FAIL as the service doesn't use normalization yet
			service := NewCreateRepositoryServiceWithNormalizedDuplicateDetection(mockRepo, mockPublisher)

			// Setup mocks based on expected behavior
			if tt.shouldDetectDuplicate {
				// Mock should return true for duplicate check (the service uses regular Exists)
				mockRepo.On("Exists", ctx, mock.AnythingOfType("valueobject.RepositoryURL")).Return(true, nil)
			} else {
				// Mock should return false (no duplicate found)
				mockRepo.On("Exists", ctx, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
				// If no duplicate, expect save and publish calls
				mockRepo.On("Save", ctx, mock.AnythingOfType("*entity.Repository")).Return(nil)
				mockPublisher.On("PublishIndexingJob", ctx, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).Return(nil)
			}

			// Execute
			request := dto.CreateRepositoryRequest{
				URL:         tt.newURL,
				Name:        "test-repo",
				Description: stringPtr("Test repository"),
			}

			response, err := service.CreateRepository(ctx, request)

			// Verify
			if tt.shouldDetectDuplicate {
				assert.Error(t, err, tt.description)
				assert.Equal(t, tt.expectedError, err, tt.description)
				assert.Nil(t, response, "Response should be nil when duplicate is detected")
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, response, "Response should not be nil when repository is created successfully")
				assert.Equal(t, "test-repo", response.Name, "Response should contain correct repository name")
			}

			// Verify mock expectations
			mockRepo.AssertExpectations(t)
			mockPublisher.AssertExpectations(t)
		})
	}
}

// TestCreateRepositoryService_NormalizedDuplicateDetectionWithDatabaseError tests error handling in normalized duplicate detection
// This test will FAIL initially as normalized duplicate detection doesn't exist yet.
func TestCreateRepositoryService_NormalizedDuplicateDetectionWithDatabaseError(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockRepositoryRepositoryWithNormalization{}
	mockPublisher := &MockMessagePublisher{}

	// Create service with enhanced duplicate detection - this will FAIL as the service doesn't exist yet
	service := NewCreateRepositoryServiceWithNormalizedDuplicateDetection(mockRepo, mockPublisher)

	// Setup mock to return database error
	expectedError := errors.New("database connection failed")
	mockRepo.On("Exists", ctx, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, expectedError)

	// Execute
	request := dto.CreateRepositoryRequest{
		URL:         "https://github.com/owner/repo",
		Name:        "test-repo",
		Description: stringPtr("Test repository"),
	}

	response, err := service.CreateRepository(ctx, request)

	// Verify
	assert.Error(t, err, "Should return error when database check fails")
	assert.Nil(t, response, "Response should be nil when error occurs")
	assert.Contains(t, err.Error(), "database connection failed", "Error should contain original database error")

	// Verify mock expectations
	mockRepo.AssertExpectations(t)
}

// TestCreateRepositoryService_FindExistingRepositoryByNormalizedURL tests finding existing repositories with normalized URL
// This test will FAIL initially as the functionality doesn't exist yet.
func TestCreateRepositoryService_FindExistingRepositoryByNormalizedURL(t *testing.T) {
	tests := []struct {
		name        string
		searchURL   string
		existingURL string
		shouldFind  bool
		description string
	}{
		{
			name:        "find_by_normalized_url_with_git_suffix",
			searchURL:   "https://github.com/owner/repo.git",
			existingURL: "https://github.com/owner/repo",
			shouldFind:  true,
			description: "Should find existing repository when searching with .git suffix",
		},
		{
			name:        "find_by_normalized_url_with_case_difference",
			searchURL:   "https://GitHub.com/owner/repo",
			existingURL: "https://github.com/owner/repo",
			shouldFind:  true,
			description: "Should find existing repository when searching with different case",
		},
		{
			name:        "find_by_normalized_url_with_trailing_slash",
			searchURL:   "https://github.com/owner/repo/",
			existingURL: "https://github.com/owner/repo",
			shouldFind:  true,
			description: "Should find existing repository when searching with trailing slash",
		},
		{
			name:        "not_find_different_repository",
			searchURL:   "https://github.com/owner/repo2",
			existingURL: "https://github.com/owner/repo1",
			shouldFind:  false,
			description: "Should not find different repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockRepo := &MockRepositoryRepositoryWithNormalization{}

			// This method doesn't exist yet - test should FAIL
			service := NewRepositoryFinderServiceWithNormalization(mockRepo)

			// Setup mocks
			if tt.shouldFind {
				existingRepo := createTestRepository(tt.existingURL)
				mockRepo.On("FindByNormalizedURL", ctx, mock.AnythingOfType("valueobject.RepositoryURL")).
					Return(existingRepo, nil)
			} else {
				mockRepo.On("FindByNormalizedURL", ctx, mock.AnythingOfType("valueobject.RepositoryURL")).Return(nil, nil)
			}

			// Execute - this method doesn't exist yet
			foundRepo, err := service.FindByNormalizedURL(ctx, tt.searchURL)

			// Verify
			require.NoError(t, err, "Should not return error for valid search")

			if tt.shouldFind {
				assert.NotNil(t, foundRepo, tt.description)
				assert.Contains(
					t,
					foundRepo.URL().String(),
					"github.com/owner/repo",
					"Found repository should match expected repository",
				)
			} else {
				assert.Nil(t, foundRepo, tt.description)
			}

			// Verify mock expectations
			mockRepo.AssertExpectations(t)
		})
	}
}

// TestDuplicateDetectionPerformance tests performance of normalized duplicate detection
// This test will FAIL initially as the performance-optimized methods don't exist yet.
func TestDuplicateDetectionPerformance(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockRepositoryRepositoryWithNormalization{}

	// This service doesn't exist yet - test should FAIL
	service := NewPerformantDuplicateDetectionService(mockRepo)

	// Setup mock for performance test
	// Use mock.Anything for context to support errgroup context passing
	mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).
		Return(false, nil)

	urls := []string{
		"https://github.com/owner/repo1",
		"https://github.com/owner/repo2.git",
		"https://GitHub.com/owner/repo3/",
		"http://github.com/owner/repo4?branch=main",
		"https://gitlab.com/owner/repo5#readme",
	}

	// This method doesn't exist yet - test should FAIL
	start := time.Now()
	results, err := service.BatchCheckDuplicates(ctx, urls)
	elapsed := time.Since(start)

	// Verify
	require.NoError(t, err, "Batch duplicate check should not return error")
	assert.Len(t, results, len(urls), "Should return results for all URLs")
	assert.Less(t, elapsed, 100*time.Millisecond, "Batch operation should be fast")

	// All URLs should be unique (no duplicates in this test)
	for i, result := range results {
		assert.False(t, result.IsDuplicate, "URL %s should not be detected as duplicate", urls[i])
		assert.Empty(t, result.ExistingRepository, "No existing repository should be found")
	}

	// Verify mock was called for each URL
	mockRepo.AssertNumberOfCalls(t, "ExistsByNormalizedURL", len(urls))
}

// Helper functions and types

// MockMessagePublisher is defined in repository_service_test.go to avoid duplication

// stringPtr is defined in repository_service_test.go to avoid duplication

// createTestRepository creates a test repository for mocking.
func createTestRepository(url string) *entity.Repository {
	repoURL, _ := valueobject.NewRepositoryURL(url)
	return entity.NewRepository(repoURL, "test-repo", stringPtr("Test repository"), stringPtr("main"))
}

// Test service interfaces are now implemented in repository_service.go
