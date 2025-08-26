package service

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"time"

	domainerrors "codechunking/internal/domain/errors/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock IndexingJob repository for testing.
type MockIndexingJobRepository struct {
	mock.Mock
}

func (m *MockIndexingJobRepository) Save(ctx context.Context, job *entity.IndexingJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockIndexingJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.IndexingJob, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.IndexingJob), args.Error(1)
}

func (m *MockIndexingJobRepository) FindByRepositoryID(
	ctx context.Context,
	repositoryID uuid.UUID,
	filters outbound.IndexingJobFilters,
) ([]*entity.IndexingJob, int, error) {
	args := m.Called(ctx, repositoryID, filters)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*entity.IndexingJob), args.Int(1), args.Error(2)
}

func (m *MockIndexingJobRepository) Update(ctx context.Context, job *entity.IndexingJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockIndexingJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestCreateIndexingJobService_CreateIndexingJob_Success(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateIndexingJobService(mockJobRepo, mockRepo, mockPublisher)

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
		valueobject.RepositoryStatusPending,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	request := dto.CreateIndexingJobRequest{
		RepositoryID: repositoryID,
	}

	// Mock expectations
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
	mockPublisher.On("PublishIndexingJob", mock.Anything, repositoryID, "https://github.com/golang/go").Return(nil)

	ctx := context.Background()

	// Act
	response, err := service.CreateIndexingJob(ctx, request)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, repositoryID, response.RepositoryID)
	assert.Equal(t, "pending", response.Status)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, 0, response.FilesProcessed)
	assert.Equal(t, 0, response.ChunksCreated)
	assert.NotZero(t, response.CreatedAt)
	assert.NotZero(t, response.UpdatedAt)
	assert.Nil(t, response.StartedAt)
	assert.Nil(t, response.CompletedAt)
	assert.Nil(t, response.ErrorMessage)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestCreateIndexingJobService_CreateIndexingJob_RepositoryNotFound(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateIndexingJobService(mockJobRepo, mockRepo, mockPublisher)

	repositoryID := uuid.New()
	request := dto.CreateIndexingJobRequest{
		RepositoryID: repositoryID,
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domainerrors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	response, err := service.CreateIndexingJob(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertNotCalled(t, "Save")
	mockPublisher.AssertNotCalled(t, "PublishIndexingJob")
}

func TestCreateIndexingJobService_CreateIndexingJob_RepositoryNotEligible(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateIndexingJobService(mockJobRepo, mockRepo, mockPublisher)

	repositoryID := uuid.New()
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

	// Repository already processing
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
		valueobject.RepositoryStatusProcessing,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	request := dto.CreateIndexingJobRequest{
		RepositoryID: repositoryID,
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)

	ctx := context.Background()

	// Act
	response, err := service.CreateIndexingJob(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrRepositoryProcessing)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertNotCalled(t, "Save")
	mockPublisher.AssertNotCalled(t, "PublishIndexingJob")
}

func TestCreateIndexingJobService_CreateIndexingJob_SaveJobFails(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateIndexingJobService(mockJobRepo, mockRepo, mockPublisher)

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
		valueobject.RepositoryStatusPending,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	request := dto.CreateIndexingJobRequest{
		RepositoryID: repositoryID,
	}

	saveError := errors.New("database save failed")
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(saveError)

	ctx := context.Background()

	// Act
	response, err := service.CreateIndexingJob(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to save indexing job")
	require.ErrorIs(t, err, saveError)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "PublishIndexingJob")
}

func TestCreateIndexingJobService_CreateIndexingJob_PublishJobFails(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateIndexingJobService(mockJobRepo, mockRepo, mockPublisher)

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
		valueobject.RepositoryStatusPending,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	request := dto.CreateIndexingJobRequest{
		RepositoryID: repositoryID,
	}

	publishError := errors.New("message queue unavailable")
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
	mockPublisher.On("PublishIndexingJob", mock.Anything, repositoryID, "https://github.com/golang/go").
		Return(publishError)

	ctx := context.Background()

	// Act
	response, err := service.CreateIndexingJob(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to publish indexing job")
	require.ErrorIs(t, err, publishError)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestGetIndexingJobService_GetIndexingJob_Success(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	service := NewGetIndexingJobService(mockJobRepo, mockRepo)

	repositoryID := uuid.New()
	jobID := uuid.New()

	// Verify repository exists
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
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
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
		1250,
		5000,
		time.Now().Add(-3*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("FindByID", mock.Anything, jobID).Return(job, nil)

	ctx := context.Background()

	// Act
	response, err := service.GetIndexingJob(ctx, repositoryID, jobID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, jobID, response.ID)
	assert.Equal(t, repositoryID, response.RepositoryID)
	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, 1250, response.FilesProcessed)
	assert.Equal(t, 5000, response.ChunksCreated)
	assert.NotNil(t, response.StartedAt)
	assert.NotNil(t, response.CompletedAt)
	assert.Nil(t, response.ErrorMessage)
	assert.NotEmpty(t, response.Duration) // Should have duration string

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
}

func TestGetIndexingJobService_GetIndexingJob_RepositoryNotFound(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	service := NewGetIndexingJobService(mockJobRepo, mockRepo)

	repositoryID := uuid.New()
	jobID := uuid.New()

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domainerrors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	response, err := service.GetIndexingJob(ctx, repositoryID, jobID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertNotCalled(t, "FindByID")
}

func TestGetIndexingJobService_GetIndexingJob_JobNotFound(t *testing.T) {
	// Arrange
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
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("FindByID", mock.Anything, jobID).Return(nil, domainerrors.ErrJobNotFound)

	ctx := context.Background()

	// Act
	response, err := service.GetIndexingJob(ctx, repositoryID, jobID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrJobNotFound)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
}

func TestGetIndexingJobService_GetIndexingJob_JobBelongsToDifferentRepository(t *testing.T) {
	// Arrange
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
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	// Job belongs to a different repository
	job := entity.RestoreIndexingJob(
		jobID,
		otherRepositoryID, // Different repository ID
		valueobject.JobStatusCompleted,
		nil,
		nil,
		nil,
		0,
		0,
		time.Now().Add(-3*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("FindByID", mock.Anything, jobID).Return(job, nil)

	ctx := context.Background()

	// Act
	response, err := service.GetIndexingJob(ctx, repositoryID, jobID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrJobNotFound)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
}

func TestUpdateIndexingJobService_UpdateIndexingJob_StartJob(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	service := NewUpdateIndexingJobService(mockJobRepo)

	jobID := uuid.New()
	repositoryID := uuid.New()

	job := entity.RestoreIndexingJob(
		jobID,
		repositoryID,
		valueobject.JobStatusPending,
		nil,
		nil,
		nil,
		0,
		0,
		time.Now().Add(-time.Hour),
		time.Now().Add(-30*time.Minute),
		nil,
	)

	request := dto.UpdateIndexingJobRequest{
		Status: "running",
	}

	mockJobRepo.On("FindByID", mock.Anything, jobID).Return(job, nil)
	mockJobRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)

	ctx := context.Background()

	// Act
	response, err := service.UpdateIndexingJob(ctx, jobID, request)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, jobID, response.ID)
	assert.Equal(t, "running", response.Status)
	assert.NotNil(t, response.StartedAt)
	assert.Nil(t, response.CompletedAt)

	mockJobRepo.AssertExpectations(t)
}

func TestUpdateIndexingJobService_UpdateIndexingJob_CompleteJob(t *testing.T) {
	// Arrange
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
		500,
		2000,
		time.Now().Add(-2*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	request := dto.UpdateIndexingJobRequest{
		Status:         "completed",
		FilesProcessed: intPtr(1250),
		ChunksCreated:  intPtr(5000),
	}

	mockJobRepo.On("FindByID", mock.Anything, jobID).Return(job, nil)
	mockJobRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)

	ctx := context.Background()

	// Act
	response, err := service.UpdateIndexingJob(ctx, jobID, request)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, jobID, response.ID)
	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, 1250, response.FilesProcessed)
	assert.Equal(t, 5000, response.ChunksCreated)
	assert.NotNil(t, response.StartedAt)
	assert.NotNil(t, response.CompletedAt)
	assert.Nil(t, response.ErrorMessage)

	mockJobRepo.AssertExpectations(t)
}

func TestUpdateIndexingJobService_UpdateIndexingJob_FailJob(t *testing.T) {
	// Arrange
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
		time.Now().Add(-time.Hour),
		nil,
	)

	errorMessage := "Failed to process repository: connection timeout"
	request := dto.UpdateIndexingJobRequest{
		Status:       "failed",
		ErrorMessage: &errorMessage,
	}

	mockJobRepo.On("FindByID", mock.Anything, jobID).Return(job, nil)
	mockJobRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)

	ctx := context.Background()

	// Act
	response, err := service.UpdateIndexingJob(ctx, jobID, request)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, jobID, response.ID)
	assert.Equal(t, "failed", response.Status)
	assert.Equal(t, errorMessage, *response.ErrorMessage)
	assert.NotNil(t, response.CompletedAt)

	mockJobRepo.AssertExpectations(t)
}

func TestUpdateIndexingJobService_UpdateIndexingJob_JobNotFound(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	service := NewUpdateIndexingJobService(mockJobRepo)

	jobID := uuid.New()
	request := dto.UpdateIndexingJobRequest{
		Status: "running",
	}

	mockJobRepo.On("FindByID", mock.Anything, jobID).Return(nil, domainerrors.ErrJobNotFound)

	ctx := context.Background()

	// Act
	response, err := service.UpdateIndexingJob(ctx, jobID, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrJobNotFound)

	mockJobRepo.AssertExpectations(t)
	mockJobRepo.AssertNotCalled(t, "Update")
}

func TestUpdateIndexingJobService_UpdateIndexingJob_InvalidStatusTransition(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	service := NewUpdateIndexingJobService(mockJobRepo)

	jobID := uuid.New()
	repositoryID := uuid.New()

	// Job is already completed
	completedTime := time.Now().Add(-time.Hour)
	job := entity.RestoreIndexingJob(
		jobID,
		repositoryID,
		valueobject.JobStatusCompleted,
		&completedTime,
		&completedTime,
		nil,
		1000,
		4000,
		time.Now().Add(-2*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	request := dto.UpdateIndexingJobRequest{
		Status: "running", // Cannot transition from completed to running
	}

	mockJobRepo.On("FindByID", mock.Anything, jobID).Return(job, nil)

	ctx := context.Background()

	// Act
	response, err := service.UpdateIndexingJob(ctx, jobID, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid status transition")

	mockJobRepo.AssertExpectations(t)
	mockJobRepo.AssertNotCalled(t, "Update")
}

func TestListIndexingJobsService_ListIndexingJobs_Success(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	service := NewListIndexingJobsService(mockJobRepo, mockRepo)

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
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	job1 := entity.RestoreIndexingJob(
		uuid.New(),
		repositoryID,
		valueobject.JobStatusCompleted,
		timePtr(time.Now().Add(-2*time.Hour)),
		timePtr(time.Now().Add(-time.Hour)),
		nil,
		1250,
		5000,
		time.Now().Add(-3*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	job2 := entity.RestoreIndexingJob(
		uuid.New(),
		repositoryID,
		valueobject.JobStatusPending,
		nil,
		nil,
		nil,
		0,
		0,
		time.Now().Add(-30*time.Minute),
		time.Now().Add(-30*time.Minute),
		nil,
	)

	jobs := []*entity.IndexingJob{job1, job2}

	query := dto.IndexingJobListQuery{
		Limit:  10,
		Offset: 0,
	}

	expectedFilters := outbound.IndexingJobFilters{
		Limit:  10,
		Offset: 0,
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("FindByRepositoryID", mock.Anything, repositoryID, expectedFilters).Return(jobs, 2, nil)

	ctx := context.Background()

	// Act
	response, err := service.ListIndexingJobs(ctx, repositoryID, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Jobs, 2)
	assert.Equal(t, "completed", response.Jobs[0].Status)
	assert.Equal(t, "pending", response.Jobs[1].Status)
	assert.Equal(t, 10, response.Pagination.Limit)
	assert.Equal(t, 0, response.Pagination.Offset)
	assert.Equal(t, 2, response.Pagination.Total)
	assert.False(t, response.Pagination.HasMore)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
}

func TestListIndexingJobsService_ListIndexingJobs_RepositoryNotFound(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	service := NewListIndexingJobsService(mockJobRepo, mockRepo)

	repositoryID := uuid.New()
	query := dto.IndexingJobListQuery{
		Limit:  10,
		Offset: 0,
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domainerrors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	response, err := service.ListIndexingJobs(ctx, repositoryID, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertNotCalled(t, "FindByRepositoryID")
}

func TestListIndexingJobsService_ListIndexingJobs_WithPagination(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	service := NewListIndexingJobsService(mockJobRepo, mockRepo)

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
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	query := dto.IndexingJobListQuery{
		Limit:  5,
		Offset: 10,
	}

	expectedFilters := outbound.IndexingJobFilters{
		Limit:  5,
		Offset: 10,
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("FindByRepositoryID", mock.Anything, repositoryID, expectedFilters).
		Return([]*entity.IndexingJob{}, 25, nil)

	ctx := context.Background()

	// Act
	response, err := service.ListIndexingJobs(ctx, repositoryID, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 5, response.Pagination.Limit)
	assert.Equal(t, 10, response.Pagination.Offset)
	assert.Equal(t, 25, response.Pagination.Total)
	assert.True(t, response.Pagination.HasMore) // 10 + 5 < 25

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
}

func TestListIndexingJobsService_ListIndexingJobs_DatabaseError(t *testing.T) {
	// Arrange
	mockJobRepo := new(MockIndexingJobRepository)
	mockRepo := new(MockRepositoryRepository)
	service := NewListIndexingJobsService(mockJobRepo, mockRepo)

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
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	query := dto.IndexingJobListQuery{
		Limit:  10,
		Offset: 0,
	}

	dbError := errors.New("database connection failed")
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockJobRepo.On("FindByRepositoryID", mock.Anything, repositoryID, mock.AnythingOfType("outbound.IndexingJobFilters")).
		Return(nil, 0, dbError)

	ctx := context.Background()

	// Act
	response, err := service.ListIndexingJobs(ctx, repositoryID, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to retrieve indexing jobs")
	require.ErrorIs(t, err, dbError)

	mockRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
}

// Helper function.
func intPtr(i int) *int {
	return &i
}
