package service

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	domain_errors "codechunking/internal/domain/errors/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestApplicationServiceInterfaces_MustExist verifies that all required service interfaces exist
// These tests will fail until the actual service implementations are created.
func TestApplicationServiceInterfaces_MustExist(t *testing.T) {
	t.Run("CreateRepositoryService interface must exist", func(t *testing.T) {
		// This test will fail until NewCreateRepositoryService function exists
		mockRepo := new(MockRepositoryRepository)
		mockPublisher := new(MockMessagePublisher)

		// This line will cause compilation error until service is implemented
		service := NewCreateRepositoryService(mockRepo, mockPublisher)
		assert.NotNil(t, service)
	})

	t.Run("GetRepositoryService interface must exist", func(t *testing.T) {
		mockRepo := new(MockRepositoryRepository)
		service := NewGetRepositoryService(mockRepo)
		assert.NotNil(t, service)
	})

	t.Run("UpdateRepositoryService interface must exist", func(t *testing.T) {
		mockRepo := new(MockRepositoryRepository)
		service := NewUpdateRepositoryService(mockRepo)
		assert.NotNil(t, service)
	})

	t.Run("DeleteRepositoryService interface must exist", func(t *testing.T) {
		mockRepo := new(MockRepositoryRepository)
		service := NewDeleteRepositoryService(mockRepo)
		assert.NotNil(t, service)
	})

	t.Run("ListRepositoriesService interface must exist", func(t *testing.T) {
		mockRepo := new(MockRepositoryRepository)
		service := NewListRepositoriesService(mockRepo)
		assert.NotNil(t, service)
	})

	t.Run("CreateIndexingJobService interface must exist", func(t *testing.T) {
		mockJobRepo := new(MockIndexingJobRepository)
		mockRepo := new(MockRepositoryRepository)
		mockPublisher := new(MockMessagePublisher)
		service := NewCreateIndexingJobService(mockJobRepo, mockRepo, mockPublisher)
		assert.NotNil(t, service)
	})

	t.Run("GetIndexingJobService interface must exist", func(t *testing.T) {
		mockJobRepo := new(MockIndexingJobRepository)
		mockRepo := new(MockRepositoryRepository)
		service := NewGetIndexingJobService(mockJobRepo, mockRepo)
		assert.NotNil(t, service)
	})

	t.Run("UpdateIndexingJobService interface must exist", func(t *testing.T) {
		mockJobRepo := new(MockIndexingJobRepository)
		service := NewUpdateIndexingJobService(mockJobRepo)
		assert.NotNil(t, service)
	})

	t.Run("ListIndexingJobsService interface must exist", func(t *testing.T) {
		mockJobRepo := new(MockIndexingJobRepository)
		mockRepo := new(MockRepositoryRepository)
		service := NewListIndexingJobsService(mockJobRepo, mockRepo)
		assert.NotNil(t, service)
	})
}

// TestRepositoryServiceBehaviorSpecification defines the expected behavior of repository services.
func TestRepositoryServiceBehaviorSpecification(t *testing.T) {
	t.Run("CreateRepositoryService behavior specification", func(t *testing.T) {
		// Test case: Auto-generate name from URL if not provided
		t.Run("should auto-generate name from URL when name is empty", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			mockPublisher := new(MockMessagePublisher)
			service := NewCreateRepositoryService(mockRepo, mockPublisher)

			request := dto.CreateRepositoryRequest{
				URL: "https://github.com/golang/go",
				// Name is empty - should be generated
			}

			mockRepo.On("Exists", context.Background(), mock.AnythingOfType("valueobject.RepositoryURL")).
				Return(false, nil)
			mockRepo.On("Save", context.Background(), mock.AnythingOfType("*entity.Repository")).Return(nil)
			mockPublisher.On("PublishIndexingJob", context.Background(), mock.AnythingOfType("uuid.UUID"), "https://github.com/golang/go").
				Return(nil)

			response, err := service.CreateRepository(context.Background(), request)

			require.NoError(t, err)
			assert.Equal(t, "golang/go", response.Name) // Should be auto-generated from URL
		})

		// Test case: URL normalization
		t.Run("should normalize repository URL by removing .git suffix", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			mockPublisher := new(MockMessagePublisher)
			service := NewCreateRepositoryService(mockRepo, mockPublisher)

			request := dto.CreateRepositoryRequest{
				URL:  "https://github.com/golang/go.git", // With .git suffix
				Name: "golang/go",
			}

			mockRepo.On("Exists", context.Background(), mock.AnythingOfType("valueobject.RepositoryURL")).
				Return(false, nil)
			mockRepo.On("Save", context.Background(), mock.AnythingOfType("*entity.Repository")).Return(nil)
			mockPublisher.On("PublishIndexingJob", context.Background(), mock.AnythingOfType("uuid.UUID"), "https://github.com/golang/go").
				Return(nil)

			response, err := service.CreateRepository(context.Background(), request)

			require.NoError(t, err)
			assert.Equal(t, "https://github.com/golang/go", response.URL) // .git should be removed
		})

		// Test case: Business rule validation
		t.Run("should enforce repository URL validation rules", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			mockPublisher := new(MockMessagePublisher)
			service := NewCreateRepositoryService(mockRepo, mockPublisher)

			invalidURLs := []string{
				"not-a-url",
				"https://unsupported-host.com/owner/repo",
				"https://github.com/only-owner", // Missing repo name
				"ftp://github.com/owner/repo",   // Wrong scheme
			}

			for _, url := range invalidURLs {
				request := dto.CreateRepositoryRequest{
					URL:  url,
					Name: "test",
				}

				response, err := service.CreateRepository(context.Background(), request)
				require.Error(t, err, "URL %s should be invalid", url)
				assert.Nil(t, response)
			}
		})
	})

	t.Run("UpdateRepositoryService behavior specification", func(t *testing.T) {
		// Test case: Business rule - cannot update processing repository
		t.Run("should not allow updates to repository that is being processed", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			service := NewUpdateRepositoryService(mockRepo)

			repositoryID := uuid.New()
			testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

			processingRepository := entity.RestoreRepository(
				repositoryID,
				testURL,
				"golang/go",
				nil,
				nil,
				nil,
				nil,
				0,
				0,
				valueobject.RepositoryStatusProcessing,
				time.Now().Add(-time.Hour),
				time.Now().Add(-30*time.Minute),
				nil,
			)

			mockRepo.On("FindByID", context.Background(), repositoryID).Return(processingRepository, nil)

			request := dto.UpdateRepositoryRequest{
				Name: stringPtr("New Name"),
			}

			response, err := service.UpdateRepository(context.Background(), repositoryID, request)
			require.Error(t, err)
			assert.Nil(t, response)
			require.ErrorIs(t, err, domain_errors.ErrRepositoryProcessing)
		})

		// Test case: Partial updates
		t.Run("should support partial updates without affecting unspecified fields", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			service := NewUpdateRepositoryService(mockRepo)

			repositoryID := uuid.New()
			testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

			repository := entity.RestoreRepository(
				repositoryID,
				testURL,
				"golang/go",
				stringPtr("Original description"),
				stringPtr("main"),
				nil,
				nil,
				0,
				0,
				valueobject.RepositoryStatusPending,
				time.Now().Add(-time.Hour),
				time.Now().Add(-30*time.Minute),
				nil,
			)

			mockRepo.On("FindByID", context.Background(), repositoryID).Return(repository, nil)
			mockRepo.On("Update", context.Background(), mock.AnythingOfType("*entity.Repository")).Return(nil)

			// Only update name, leave description and branch unchanged
			request := dto.UpdateRepositoryRequest{
				Name: stringPtr("New Name"),
			}

			response, err := service.UpdateRepository(context.Background(), repositoryID, request)
			require.NoError(t, err)
			assert.Equal(t, "New Name", response.Name)
			assert.Equal(t, "Original description", *response.Description) // Should remain unchanged
			assert.Equal(t, "main", *response.DefaultBranch)               // Should remain unchanged
		})
	})

	t.Run("DeleteRepositoryService behavior specification", func(t *testing.T) {
		// Test case: Business rule - cannot delete processing repository
		t.Run("should not allow deletion of repository that is being processed", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			service := NewDeleteRepositoryService(mockRepo)

			repositoryID := uuid.New()
			testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

			processingRepository := entity.RestoreRepository(
				repositoryID,
				testURL,
				"golang/go",
				nil,
				nil,
				nil,
				nil,
				0,
				0,
				valueobject.RepositoryStatusProcessing,
				time.Now().Add(-time.Hour),
				time.Now().Add(-30*time.Minute),
				nil,
			)

			mockRepo.On("FindByID", context.Background(), repositoryID).Return(processingRepository, nil)

			err := service.DeleteRepository(context.Background(), repositoryID)
			require.Error(t, err)
			require.ErrorIs(t, err, domain_errors.ErrRepositoryProcessing)
		})

		// Test case: Soft delete behavior
		t.Run("should perform soft delete by archiving repository", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			service := NewDeleteRepositoryService(mockRepo)

			repositoryID := uuid.New()
			testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

			repository := entity.RestoreRepository(
				repositoryID,
				testURL,
				"golang/go",
				nil,
				nil,
				nil,
				nil,
				0,
				0,
				valueobject.RepositoryStatusCompleted,
				time.Now().Add(-time.Hour),
				time.Now().Add(-30*time.Minute),
				nil,
			)

			mockRepo.On("FindByID", context.Background(), repositoryID).Return(repository, nil)
			mockRepo.On("Update", context.Background(), mock.AnythingOfType("*entity.Repository")).
				Run(func(args mock.Arguments) {
					// Verify that the repository was archived
					repo := args.Get(1).(*entity.Repository)
					assert.True(t, repo.IsDeleted())
					assert.Equal(t, valueobject.RepositoryStatusArchived, repo.Status())
				}).
				Return(nil)

			err := service.DeleteRepository(context.Background(), repositoryID)
			require.NoError(t, err)
		})
	})

	t.Run("ListRepositoriesService behavior specification", func(t *testing.T) {
		// Test case: Status filter validation
		t.Run("should validate status filter values", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			service := NewListRepositoriesService(mockRepo)

			invalidStatuses := []string{
				"invalid-status",
				"PENDING", // Wrong case
				"running", // Wrong status (this is for jobs, not repositories)
			}

			for _, status := range invalidStatuses {
				query := dto.RepositoryListQuery{
					Status: status,
					Limit:  20,
					Offset: 0,
				}

				response, err := service.ListRepositories(context.Background(), query)
				require.Error(t, err, "Status %s should be invalid", status)
				assert.Nil(t, response)
			}
		})

		// Test case: Pagination calculation
		t.Run("should correctly calculate HasMore pagination flag", func(t *testing.T) {
			mockRepo := new(MockRepositoryRepository)
			service := NewListRepositoriesService(mockRepo)

			query := dto.RepositoryListQuery{
				Limit:  10,
				Offset: 20,
			}

			// Mock return: total=35, so 20+10=30 < 35, HasMore should be true
			mockRepo.On("FindAll", context.Background(), mock.AnythingOfType("outbound.RepositoryFilters")).
				Return([]*entity.Repository{}, 35, nil)

			response, err := service.ListRepositories(context.Background(), query)
			require.NoError(t, err)
			assert.True(t, response.Pagination.HasMore)
			assert.Equal(t, 35, response.Pagination.Total)
		})
	})
}

// TestIndexingJobServiceBehaviorSpecification defines the expected behavior of indexing job services.
func TestIndexingJobServiceBehaviorSpecification(t *testing.T) {
	t.Run("CreateIndexingJobService behavior specification", func(t *testing.T) {
		// Test case: Business rule - repository must be in eligible state
		t.Run("should only allow job creation for repositories in eligible states", func(t *testing.T) {
			mockJobRepo := new(MockIndexingJobRepository)
			mockRepo := new(MockRepositoryRepository)
			mockPublisher := new(MockMessagePublisher)
			service := NewCreateIndexingJobService(mockJobRepo, mockRepo, mockPublisher)

			repositoryID := uuid.New()
			testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

			ineligibleStatuses := []valueobject.RepositoryStatus{
				valueobject.RepositoryStatusProcessing, // Already being processed
				valueobject.RepositoryStatusCloning,    // Already being processed
			}

			for _, status := range ineligibleStatuses {
				repository := entity.RestoreRepository(
					repositoryID,
					testURL,
					"golang/go",
					nil,
					nil,
					nil,
					nil,
					0,
					0,
					status,
					time.Now().Add(-time.Hour),
					time.Now().Add(-30*time.Minute),
					nil,
				)

				mockRepo.On("FindByID", context.Background(), repositoryID).Return(repository, nil).Once()

				request := dto.CreateIndexingJobRequest{
					RepositoryID: repositoryID,
				}

				response, err := service.CreateIndexingJob(context.Background(), request)
				require.Error(t, err, "Should not allow job creation for status %s", status)
				assert.Nil(t, response)
				require.ErrorIs(t, err, domain_errors.ErrRepositoryProcessing)
			}
		})
	})

	t.Run("UpdateIndexingJobService behavior specification", func(t *testing.T) {
		// Test case: Status transition validation
		t.Run("should enforce valid status transitions", func(t *testing.T) {
			mockJobRepo := new(MockIndexingJobRepository)
			service := NewUpdateIndexingJobService(mockJobRepo)

			jobID := uuid.New()
			repositoryID := uuid.New()

			// Job is already completed
			job := entity.RestoreIndexingJob(
				jobID,
				repositoryID,
				valueobject.JobStatusCompleted,
				timePtr(time.Now().Add(-2*time.Hour)),
				timePtr(time.Now().Add(-time.Hour)),
				nil,
				1000,
				4000,
				time.Now().Add(-3*time.Hour),
				time.Now().Add(-time.Hour),
				nil,
			)

			mockJobRepo.On("FindByID", context.Background(), jobID).Return(job, nil)

			// Try to transition from completed to running (invalid)
			request := dto.UpdateIndexingJobRequest{
				Status: "running",
			}

			response, err := service.UpdateIndexingJob(context.Background(), jobID, request)
			require.Error(t, err)
			assert.Nil(t, response)
			assert.Contains(t, err.Error(), "invalid status transition")
		})

		// Test case: Progress tracking
		t.Run("should update progress without changing status", func(t *testing.T) {
			mockJobRepo := new(MockIndexingJobRepository)
			service := NewUpdateIndexingJobService(mockJobRepo)

			jobID := uuid.New()
			repositoryID := uuid.New()
			startTime := time.Now().Add(-time.Hour)

			job := entity.RestoreIndexingJob(
				jobID,
				repositoryID,
				valueobject.JobStatusRunning,
				&startTime,
				nil,
				nil,
				100,
				400,
				time.Now().Add(-2*time.Hour),
				time.Now().Add(-30*time.Minute),
				nil,
			)

			mockJobRepo.On("FindByID", context.Background(), jobID).Return(job, nil)
			mockJobRepo.On("Update", context.Background(), mock.AnythingOfType("*entity.IndexingJob")).Return(nil)

			// Update progress only, keep same status
			request := dto.UpdateIndexingJobRequest{
				Status:         "running", // Same status
				FilesProcessed: intPtr(250),
				ChunksCreated:  intPtr(1000),
			}

			response, err := service.UpdateIndexingJob(context.Background(), jobID, request)
			require.NoError(t, err)
			assert.Equal(t, "running", response.Status) // Status unchanged
			assert.Equal(t, 250, response.FilesProcessed)
			assert.Equal(t, 1000, response.ChunksCreated)
		})
	})

	t.Run("GetIndexingJobService behavior specification", func(t *testing.T) {
		// Test case: Job ownership validation
		t.Run("should verify job belongs to specified repository", func(t *testing.T) {
			mockJobRepo := new(MockIndexingJobRepository)
			mockRepo := new(MockRepositoryRepository)
			service := NewGetIndexingJobService(mockJobRepo, mockRepo)

			repositoryID := uuid.New()
			otherRepositoryID := uuid.New()
			jobID := uuid.New()

			testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
			repository := entity.RestoreRepository(
				repositoryID,
				testURL,
				"golang/go",
				nil,
				nil,
				nil,
				nil,
				0,
				0,
				valueobject.RepositoryStatusCompleted,
				time.Now().Add(-time.Hour),
				time.Now().Add(-30*time.Minute),
				nil,
			)

			// Job belongs to different repository
			job := entity.RestoreIndexingJob(
				jobID,
				otherRepositoryID, // Different repository
				valueobject.JobStatusCompleted,
				timePtr(time.Now().Add(-2*time.Hour)),
				timePtr(time.Now().Add(-time.Hour)),
				nil,
				1000,
				4000,
				time.Now().Add(-3*time.Hour),
				time.Now().Add(-time.Hour),
				nil,
			)

			mockRepo.On("FindByID", context.Background(), repositoryID).Return(repository, nil)
			mockJobRepo.On("FindByID", context.Background(), jobID).Return(job, nil)

			response, err := service.GetIndexingJob(context.Background(), repositoryID, jobID)
			require.Error(t, err)
			assert.Nil(t, response)
			require.ErrorIs(t, err, domain_errors.ErrJobNotFound)
		})

		// Test case: Duration calculation
		t.Run("should include human-readable duration for completed jobs", func(t *testing.T) {
			mockJobRepo := new(MockIndexingJobRepository)
			mockRepo := new(MockRepositoryRepository)
			service := NewGetIndexingJobService(mockJobRepo, mockRepo)

			repositoryID := uuid.New()
			jobID := uuid.New()

			testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
			repository := entity.RestoreRepository(
				repositoryID,
				testURL,
				"golang/go",
				nil,
				nil,
				nil,
				nil,
				0,
				0,
				valueobject.RepositoryStatusCompleted,
				time.Now().Add(-time.Hour),
				time.Now().Add(-30*time.Minute),
				nil,
			)

			startTime := time.Now().Add(-2 * time.Hour)
			endTime := time.Now().Add(-time.Hour)

			job := entity.RestoreIndexingJob(
				jobID,
				repositoryID,
				valueobject.JobStatusCompleted,
				&startTime,
				&endTime,
				nil,
				1000,
				4000,
				time.Now().Add(-3*time.Hour),
				time.Now().Add(-time.Hour),
				nil,
			)

			mockRepo.On("FindByID", context.Background(), repositoryID).Return(repository, nil)
			mockJobRepo.On("FindByID", context.Background(), jobID).Return(job, nil)

			response, err := service.GetIndexingJob(context.Background(), repositoryID, jobID)
			require.NoError(t, err)
			assert.NotNil(t, response.Duration)
			assert.Contains(t, *response.Duration, "h") // Should contain hour indicator
		})
	})
}

// TestServiceErrorHandlingSpecification defines expected error handling behavior.
func TestServiceErrorHandlingSpecification(t *testing.T) {
	t.Run("should wrap repository layer errors with context", func(t *testing.T) {
		mockRepo := new(MockRepositoryRepository)
		service := NewGetRepositoryService(mockRepo)

		repositoryID := uuid.New()
		underlyingError := assert.AnError // Generic database error

		mockRepo.On("FindByID", context.Background(), repositoryID).Return(nil, underlyingError)

		response, err := service.GetRepository(context.Background(), repositoryID)
		require.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to retrieve repository") // Contextual error message
		require.ErrorIs(t, err, underlyingError)                          // Original error preserved
	})

	t.Run("should preserve domain error semantics", func(t *testing.T) {
		mockRepo := new(MockRepositoryRepository)
		service := NewGetRepositoryService(mockRepo)

		repositoryID := uuid.New()
		mockRepo.On("FindByID", context.Background(), repositoryID).Return(nil, domain_errors.ErrRepositoryNotFound)

		response, err := service.GetRepository(context.Background(), repositoryID)
		require.Error(t, err)
		assert.Nil(t, response)
		require.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound) // Domain error preserved for API layer
	})
}
