package handler

import (
	"codechunking/internal/application/dto"
	"context"
	"errors"
	"testing"
	"time"

	domain_errors "codechunking/internal/domain/errors/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock indexing job application services for testing.
type MockCreateIndexingJobService struct {
	mock.Mock
}

func (m *MockCreateIndexingJobService) CreateIndexingJob(
	ctx context.Context,
	request dto.CreateIndexingJobRequest,
) (*dto.IndexingJobResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.IndexingJobResponse), args.Error(1)
}

type MockGetIndexingJobService struct {
	mock.Mock
}

func (m *MockGetIndexingJobService) GetIndexingJob(
	ctx context.Context,
	repositoryID, jobID uuid.UUID,
) (*dto.IndexingJobResponse, error) {
	args := m.Called(ctx, repositoryID, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.IndexingJobResponse), args.Error(1)
}

type MockUpdateIndexingJobService struct {
	mock.Mock
}

func (m *MockUpdateIndexingJobService) UpdateIndexingJob(
	ctx context.Context,
	jobID uuid.UUID,
	request dto.UpdateIndexingJobRequest,
) (*dto.IndexingJobResponse, error) {
	args := m.Called(ctx, jobID, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.IndexingJobResponse), args.Error(1)
}

type MockListIndexingJobsService struct {
	mock.Mock
}

func (m *MockListIndexingJobsService) ListIndexingJobs(
	ctx context.Context,
	repositoryID uuid.UUID,
	query dto.IndexingJobListQuery,
) (*dto.IndexingJobListResponse, error) {
	args := m.Called(ctx, repositoryID, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.IndexingJobListResponse), args.Error(1)
}

func TestCreateIndexingJobCommandHandler_Handle_Success(t *testing.T) {
	// Arrange
	mockService := new(MockCreateIndexingJobService)
	handler := NewCreateIndexingJobCommandHandler(mockService)

	repositoryID := uuid.New()
	command := CreateIndexingJobCommand{
		RepositoryID: repositoryID,
	}

	expectedRequest := dto.CreateIndexingJobRequest{
		RepositoryID: repositoryID,
	}

	expectedResponse := &dto.IndexingJobResponse{
		ID:           uuid.New(),
		RepositoryID: repositoryID,
		Status:       "pending",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	mockService.On("CreateIndexingJob", mock.Anything, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResponse.ID, result.ID)
	assert.Equal(t, expectedResponse.RepositoryID, result.RepositoryID)
	assert.Equal(t, "pending", result.Status)

	mockService.AssertExpectations(t)
}

func TestCreateIndexingJobCommandHandler_Handle_RepositoryNotFound(t *testing.T) {
	// Arrange
	mockService := new(MockCreateIndexingJobService)
	handler := NewCreateIndexingJobCommandHandler(mockService)

	repositoryID := uuid.New()
	command := CreateIndexingJobCommand{
		RepositoryID: repositoryID,
	}

	expectedRequest := dto.CreateIndexingJobRequest{
		RepositoryID: repositoryID,
	}

	mockService.On("CreateIndexingJob", mock.Anything, expectedRequest).Return(nil, domain_errors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound)

	mockService.AssertExpectations(t)
}

func TestCreateIndexingJobCommandHandler_Handle_RepositoryProcessing(t *testing.T) {
	// Arrange
	mockService := new(MockCreateIndexingJobService)
	handler := NewCreateIndexingJobCommandHandler(mockService)

	repositoryID := uuid.New()
	command := CreateIndexingJobCommand{
		RepositoryID: repositoryID,
	}

	expectedRequest := dto.CreateIndexingJobRequest{
		RepositoryID: repositoryID,
	}

	mockService.On("CreateIndexingJob", mock.Anything, expectedRequest).
		Return(nil, domain_errors.ErrRepositoryProcessing)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryProcessing)

	mockService.AssertExpectations(t)
}

func TestCreateIndexingJobCommandHandler_Handle_ValidationError(t *testing.T) {
	// Arrange
	mockService := new(MockCreateIndexingJobService)
	handler := NewCreateIndexingJobCommandHandler(mockService)

	command := CreateIndexingJobCommand{
		RepositoryID: uuid.Nil, // Invalid UUID
	}

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation failed")

	mockService.AssertNotCalled(t, "CreateIndexingJob")
}

func TestGetIndexingJobQueryHandler_Handle_Success(t *testing.T) {
	// Arrange
	mockService := new(MockGetIndexingJobService)
	handler := NewGetIndexingJobQueryHandler(mockService)

	repositoryID := uuid.New()
	jobID := uuid.New()
	query := GetIndexingJobQuery{
		RepositoryID: repositoryID,
		JobID:        jobID,
	}

	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-time.Hour)

	expectedResponse := &dto.IndexingJobResponse{
		ID:             jobID,
		RepositoryID:   repositoryID,
		Status:         "completed",
		StartedAt:      &startTime,
		CompletedAt:    &endTime,
		FilesProcessed: 1250,
		ChunksCreated:  5000,
		Duration:       stringPtr("1h0m0s"),
	}

	mockService.On("GetIndexingJob", mock.Anything, repositoryID, jobID).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, jobID, result.ID)
	assert.Equal(t, repositoryID, result.RepositoryID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 1250, result.FilesProcessed)
	assert.Equal(t, 5000, result.ChunksCreated)
	assert.NotNil(t, result.Duration)

	mockService.AssertExpectations(t)
}

func TestGetIndexingJobQueryHandler_Handle_JobNotFound(t *testing.T) {
	// Arrange
	mockService := new(MockGetIndexingJobService)
	handler := NewGetIndexingJobQueryHandler(mockService)

	repositoryID := uuid.New()
	jobID := uuid.New()
	query := GetIndexingJobQuery{
		RepositoryID: repositoryID,
		JobID:        jobID,
	}

	mockService.On("GetIndexingJob", mock.Anything, repositoryID, jobID).Return(nil, domain_errors.ErrJobNotFound)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, domain_errors.ErrJobNotFound)

	mockService.AssertExpectations(t)
}

func TestGetIndexingJobQueryHandler_Handle_RepositoryNotFound(t *testing.T) {
	// Arrange
	mockService := new(MockGetIndexingJobService)
	handler := NewGetIndexingJobQueryHandler(mockService)

	repositoryID := uuid.New()
	jobID := uuid.New()
	query := GetIndexingJobQuery{
		RepositoryID: repositoryID,
		JobID:        jobID,
	}

	mockService.On("GetIndexingJob", mock.Anything, repositoryID, jobID).
		Return(nil, domain_errors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound)

	mockService.AssertExpectations(t)
}

func TestUpdateIndexingJobCommandHandler_Handle_StartJob(t *testing.T) {
	// Arrange
	mockService := new(MockUpdateIndexingJobService)
	handler := NewUpdateIndexingJobCommandHandler(mockService)

	jobID := uuid.New()
	command := UpdateIndexingJobCommand{
		JobID:  jobID,
		Status: "running",
	}

	expectedRequest := dto.UpdateIndexingJobRequest{
		Status: "running",
	}

	startTime := time.Now()
	expectedResponse := &dto.IndexingJobResponse{
		ID:        jobID,
		Status:    "running",
		StartedAt: &startTime,
	}

	mockService.On("UpdateIndexingJob", mock.Anything, jobID, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, jobID, result.ID)
	assert.Equal(t, "running", result.Status)
	assert.NotNil(t, result.StartedAt)

	mockService.AssertExpectations(t)
}

func TestUpdateIndexingJobCommandHandler_Handle_CompleteJob(t *testing.T) {
	// Arrange
	mockService := new(MockUpdateIndexingJobService)
	handler := NewUpdateIndexingJobCommandHandler(mockService)

	jobID := uuid.New()
	command := UpdateIndexingJobCommand{
		JobID:          jobID,
		Status:         "completed",
		FilesProcessed: intPtr(1250),
		ChunksCreated:  intPtr(5000),
	}

	expectedRequest := dto.UpdateIndexingJobRequest{
		Status:         "completed",
		FilesProcessed: intPtr(1250),
		ChunksCreated:  intPtr(5000),
	}

	endTime := time.Now()
	expectedResponse := &dto.IndexingJobResponse{
		ID:             jobID,
		Status:         "completed",
		CompletedAt:    &endTime,
		FilesProcessed: 1250,
		ChunksCreated:  5000,
	}

	mockService.On("UpdateIndexingJob", mock.Anything, jobID, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, jobID, result.ID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 1250, result.FilesProcessed)
	assert.Equal(t, 5000, result.ChunksCreated)
	assert.NotNil(t, result.CompletedAt)

	mockService.AssertExpectations(t)
}

func TestUpdateIndexingJobCommandHandler_Handle_FailJob(t *testing.T) {
	// Arrange
	mockService := new(MockUpdateIndexingJobService)
	handler := NewUpdateIndexingJobCommandHandler(mockService)

	jobID := uuid.New()
	errorMessage := "Processing failed: connection timeout"
	command := UpdateIndexingJobCommand{
		JobID:        jobID,
		Status:       "failed",
		ErrorMessage: &errorMessage,
	}

	expectedRequest := dto.UpdateIndexingJobRequest{
		Status:       "failed",
		ErrorMessage: &errorMessage,
	}

	endTime := time.Now()
	expectedResponse := &dto.IndexingJobResponse{
		ID:           jobID,
		Status:       "failed",
		CompletedAt:  &endTime,
		ErrorMessage: &errorMessage,
	}

	mockService.On("UpdateIndexingJob", mock.Anything, jobID, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, jobID, result.ID)
	assert.Equal(t, "failed", result.Status)
	assert.Equal(t, errorMessage, *result.ErrorMessage)
	assert.NotNil(t, result.CompletedAt)

	mockService.AssertExpectations(t)
}

func TestUpdateIndexingJobCommandHandler_Handle_ValidationError(t *testing.T) {
	// Arrange
	mockService := new(MockUpdateIndexingJobService)
	handler := NewUpdateIndexingJobCommandHandler(mockService)

	jobID := uuid.New()
	command := UpdateIndexingJobCommand{
		JobID:  jobID,
		Status: "invalid-status", // Invalid status
	}

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation failed")

	mockService.AssertNotCalled(t, "UpdateIndexingJob")
}

func TestUpdateIndexingJobCommandHandler_Handle_JobNotFound(t *testing.T) {
	// Arrange
	mockService := new(MockUpdateIndexingJobService)
	handler := NewUpdateIndexingJobCommandHandler(mockService)

	jobID := uuid.New()
	command := UpdateIndexingJobCommand{
		JobID:  jobID,
		Status: "running",
	}

	expectedRequest := dto.UpdateIndexingJobRequest{
		Status: "running",
	}

	mockService.On("UpdateIndexingJob", mock.Anything, jobID, expectedRequest).Return(nil, domain_errors.ErrJobNotFound)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, domain_errors.ErrJobNotFound)

	mockService.AssertExpectations(t)
}

func TestListIndexingJobsQueryHandler_Handle_Success(t *testing.T) {
	// Arrange
	mockService := new(MockListIndexingJobsService)
	handler := NewListIndexingJobsQueryHandler(mockService)

	repositoryID := uuid.New()
	query := ListIndexingJobsQuery{
		RepositoryID: repositoryID,
		Limit:        10,
		Offset:       0,
	}

	expectedRequest := dto.IndexingJobListQuery{
		Limit:  10,
		Offset: 0,
	}

	job1 := dto.IndexingJobResponse{
		ID:             uuid.New(),
		RepositoryID:   repositoryID,
		Status:         "completed",
		FilesProcessed: 1250,
		ChunksCreated:  5000,
	}

	job2 := dto.IndexingJobResponse{
		ID:           uuid.New(),
		RepositoryID: repositoryID,
		Status:       "pending",
	}

	expectedResponse := &dto.IndexingJobListResponse{
		Jobs: []dto.IndexingJobResponse{job1, job2},
		Pagination: dto.PaginationResponse{
			Limit:   10,
			Offset:  0,
			Total:   2,
			HasMore: false,
		},
	}

	mockService.On("ListIndexingJobs", mock.Anything, repositoryID, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Jobs, 2)
	assert.Equal(t, "completed", result.Jobs[0].Status)
	assert.Equal(t, "pending", result.Jobs[1].Status)
	assert.Equal(t, 10, result.Pagination.Limit)
	assert.Equal(t, 2, result.Pagination.Total)
	assert.False(t, result.Pagination.HasMore)

	mockService.AssertExpectations(t)
}

func TestListIndexingJobsQueryHandler_Handle_RepositoryNotFound(t *testing.T) {
	// Arrange
	mockService := new(MockListIndexingJobsService)
	handler := NewListIndexingJobsQueryHandler(mockService)

	repositoryID := uuid.New()
	query := ListIndexingJobsQuery{
		RepositoryID: repositoryID,
		Limit:        10,
		Offset:       0,
	}

	expectedRequest := dto.IndexingJobListQuery{
		Limit:  10,
		Offset: 0,
	}

	mockService.On("ListIndexingJobs", mock.Anything, repositoryID, expectedRequest).
		Return(nil, domain_errors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound)

	mockService.AssertExpectations(t)
}

func TestListIndexingJobsQueryHandler_Handle_DefaultValues(t *testing.T) {
	// Arrange
	mockService := new(MockListIndexingJobsService)
	handler := NewListIndexingJobsQueryHandler(mockService)

	repositoryID := uuid.New()
	query := ListIndexingJobsQuery{
		RepositoryID: repositoryID,
		// No limit/offset set - should use defaults
	}

	expectedRequest := dto.IndexingJobListQuery{
		Limit:  10, // Default limit
		Offset: 0,  // Default offset
	}

	expectedResponse := &dto.IndexingJobListResponse{
		Jobs: []dto.IndexingJobResponse{},
		Pagination: dto.PaginationResponse{
			Limit:   10,
			Offset:  0,
			Total:   0,
			HasMore: false,
		},
	}

	mockService.On("ListIndexingJobs", mock.Anything, repositoryID, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 10, result.Pagination.Limit)
	assert.Equal(t, 0, result.Pagination.Offset)

	mockService.AssertExpectations(t)
}

func TestListIndexingJobsQueryHandler_Handle_WithPagination(t *testing.T) {
	// Arrange
	mockService := new(MockListIndexingJobsService)
	handler := NewListIndexingJobsQueryHandler(mockService)

	repositoryID := uuid.New()
	query := ListIndexingJobsQuery{
		RepositoryID: repositoryID,
		Limit:        5,
		Offset:       10,
	}

	expectedRequest := dto.IndexingJobListQuery{
		Limit:  5,
		Offset: 10,
	}

	expectedResponse := &dto.IndexingJobListResponse{
		Jobs: []dto.IndexingJobResponse{},
		Pagination: dto.PaginationResponse{
			Limit:   5,
			Offset:  10,
			Total:   25,
			HasMore: true,
		},
	}

	mockService.On("ListIndexingJobs", mock.Anything, repositoryID, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 5, result.Pagination.Limit)
	assert.Equal(t, 10, result.Pagination.Offset)
	assert.Equal(t, 25, result.Pagination.Total)
	assert.True(t, result.Pagination.HasMore)

	mockService.AssertExpectations(t)
}

func TestListIndexingJobsQueryHandler_Handle_ValidationError(t *testing.T) {
	// Arrange
	mockService := new(MockListIndexingJobsService)
	handler := NewListIndexingJobsQueryHandler(mockService)

	repositoryID := uuid.New()
	query := ListIndexingJobsQuery{
		RepositoryID: repositoryID,
		Limit:        100, // Exceeds maximum limit
		Offset:       0,
	}

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation failed")

	mockService.AssertNotCalled(t, "ListIndexingJobs")
}

func TestListIndexingJobsQueryHandler_Handle_DatabaseError(t *testing.T) {
	// Arrange
	mockService := new(MockListIndexingJobsService)
	handler := NewListIndexingJobsQueryHandler(mockService)

	repositoryID := uuid.New()
	query := ListIndexingJobsQuery{
		RepositoryID: repositoryID,
		Limit:        10,
		Offset:       0,
	}

	expectedRequest := dto.IndexingJobListQuery{
		Limit:  10,
		Offset: 0,
	}

	dbError := errors.New("database connection failed")
	mockService.On("ListIndexingJobs", mock.Anything, repositoryID, expectedRequest).Return(nil, dbError)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, dbError)

	mockService.AssertExpectations(t)
}

// Helper functions.
func intPtr(i int) *int {
	return &i
}
