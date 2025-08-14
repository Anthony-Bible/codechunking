package handler

import (
	"context"
	"errors"
	"testing"

	"codechunking/internal/application/dto"
	domain_errors "codechunking/internal/domain/errors/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock application service for testing.
type MockCreateRepositoryService struct {
	mock.Mock
}

func (m *MockCreateRepositoryService) CreateRepository(
	ctx context.Context,
	request dto.CreateRepositoryRequest,
) (*dto.RepositoryResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.RepositoryResponse), args.Error(1)
}

type MockGetRepositoryService struct {
	mock.Mock
}

func (m *MockGetRepositoryService) GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.RepositoryResponse), args.Error(1)
}

type MockUpdateRepositoryService struct {
	mock.Mock
}

func (m *MockUpdateRepositoryService) UpdateRepository(
	ctx context.Context,
	id uuid.UUID,
	request dto.UpdateRepositoryRequest,
) (*dto.RepositoryResponse, error) {
	args := m.Called(ctx, id, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.RepositoryResponse), args.Error(1)
}

type MockDeleteRepositoryService struct {
	mock.Mock
}

func (m *MockDeleteRepositoryService) DeleteRepository(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockListRepositoriesService struct {
	mock.Mock
}

func (m *MockListRepositoriesService) ListRepositories(
	ctx context.Context,
	query dto.RepositoryListQuery,
) (*dto.RepositoryListResponse, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.RepositoryListResponse), args.Error(1)
}

func TestCreateRepositoryCommandHandler_Handle_Success(t *testing.T) {
	// Arrange
	mockService := new(MockCreateRepositoryService)
	handler := NewCreateRepositoryCommandHandler(mockService)

	command := CreateRepositoryCommand{
		URL:           "https://github.com/golang/go",
		Name:          "golang/go",
		Description:   stringPtr("The Go programming language"),
		DefaultBranch: stringPtr("master"),
	}

	expectedRequest := dto.CreateRepositoryRequest{
		URL:           "https://github.com/golang/go",
		Name:          "golang/go",
		Description:   stringPtr("The Go programming language"),
		DefaultBranch: stringPtr("master"),
	}

	expectedResponse := &dto.RepositoryResponse{
		ID:          uuid.New(),
		URL:         "https://github.com/golang/go",
		Name:        "golang/go",
		Description: stringPtr("The Go programming language"),
		Status:      "pending",
	}

	mockService.On("CreateRepository", mock.Anything, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResponse.ID, result.ID)
	assert.Equal(t, expectedResponse.URL, result.URL)
	assert.Equal(t, expectedResponse.Name, result.Name)
	assert.Equal(t, expectedResponse.Status, result.Status)

	mockService.AssertExpectations(t)
}

func TestCreateRepositoryCommandHandler_Handle_ValidationError(t *testing.T) {
	// Arrange
	mockService := new(MockCreateRepositoryService)
	handler := NewCreateRepositoryCommandHandler(mockService)

	command := CreateRepositoryCommand{
		URL: "", // Empty URL should fail validation
	}

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation failed")

	mockService.AssertNotCalled(t, "CreateRepository")
}

func TestCreateRepositoryCommandHandler_Handle_ServiceError(t *testing.T) {
	// Arrange
	mockService := new(MockCreateRepositoryService)
	handler := NewCreateRepositoryCommandHandler(mockService)

	command := CreateRepositoryCommand{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}

	expectedRequest := dto.CreateRepositoryRequest{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}

	serviceError := domain_errors.ErrRepositoryAlreadyExists
	mockService.On("CreateRepository", mock.Anything, expectedRequest).Return(nil, serviceError)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, serviceError)

	mockService.AssertExpectations(t)
}

func TestGetRepositoryQueryHandler_Handle_Success(t *testing.T) {
	// Arrange
	mockService := new(MockGetRepositoryService)
	handler := NewGetRepositoryQueryHandler(mockService)

	repositoryID := uuid.New()
	query := GetRepositoryQuery{
		ID: repositoryID,
	}

	expectedResponse := &dto.RepositoryResponse{
		ID:     repositoryID,
		URL:    "https://github.com/golang/go",
		Name:   "golang/go",
		Status: "completed",
	}

	mockService.On("GetRepository", mock.Anything, repositoryID).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResponse.ID, result.ID)
	assert.Equal(t, expectedResponse.URL, result.URL)
	assert.Equal(t, expectedResponse.Name, result.Name)
	assert.Equal(t, expectedResponse.Status, result.Status)

	mockService.AssertExpectations(t)
}

func TestGetRepositoryQueryHandler_Handle_NotFound(t *testing.T) {
	// Arrange
	mockService := new(MockGetRepositoryService)
	handler := NewGetRepositoryQueryHandler(mockService)

	repositoryID := uuid.New()
	query := GetRepositoryQuery{
		ID: repositoryID,
	}

	mockService.On("GetRepository", mock.Anything, repositoryID).Return(nil, domain_errors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound)

	mockService.AssertExpectations(t)
}

func TestUpdateRepositoryCommandHandler_Handle_Success(t *testing.T) {
	// Arrange
	mockService := new(MockUpdateRepositoryService)
	handler := NewUpdateRepositoryCommandHandler(mockService)

	repositoryID := uuid.New()
	command := UpdateRepositoryCommand{
		ID:            repositoryID,
		Name:          stringPtr("Updated Name"),
		Description:   stringPtr("Updated description"),
		DefaultBranch: stringPtr("main"),
	}

	expectedRequest := dto.UpdateRepositoryRequest{
		Name:          stringPtr("Updated Name"),
		Description:   stringPtr("Updated description"),
		DefaultBranch: stringPtr("main"),
	}

	expectedResponse := &dto.RepositoryResponse{
		ID:            repositoryID,
		URL:           "https://github.com/golang/go",
		Name:          "Updated Name",
		Description:   stringPtr("Updated description"),
		DefaultBranch: stringPtr("main"),
		Status:        "pending",
	}

	mockService.On("UpdateRepository", mock.Anything, repositoryID, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResponse.ID, result.ID)
	assert.Equal(t, "Updated Name", result.Name)
	assert.Equal(t, "Updated description", *result.Description)
	assert.Equal(t, "main", *result.DefaultBranch)

	mockService.AssertExpectations(t)
}

func TestUpdateRepositoryCommandHandler_Handle_ValidationError(t *testing.T) {
	// Arrange
	mockService := new(MockUpdateRepositoryService)
	handler := NewUpdateRepositoryCommandHandler(mockService)

	repositoryID := uuid.New()
	command := UpdateRepositoryCommand{
		ID:   repositoryID,
		Name: stringPtr(string(make([]byte, 300))), // Name too long
	}

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, command)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation failed")

	mockService.AssertNotCalled(t, "UpdateRepository")
}

func TestDeleteRepositoryCommandHandler_Handle_Success(t *testing.T) {
	// Arrange
	mockService := new(MockDeleteRepositoryService)
	handler := NewDeleteRepositoryCommandHandler(mockService)

	repositoryID := uuid.New()
	command := DeleteRepositoryCommand{
		ID: repositoryID,
	}

	mockService.On("DeleteRepository", mock.Anything, repositoryID).Return(nil)

	ctx := context.Background()

	// Act
	err := handler.Handle(ctx, command)

	// Assert
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}

func TestDeleteRepositoryCommandHandler_Handle_NotFound(t *testing.T) {
	// Arrange
	mockService := new(MockDeleteRepositoryService)
	handler := NewDeleteRepositoryCommandHandler(mockService)

	repositoryID := uuid.New()
	command := DeleteRepositoryCommand{
		ID: repositoryID,
	}

	mockService.On("DeleteRepository", mock.Anything, repositoryID).Return(domain_errors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	err := handler.Handle(ctx, command)

	// Assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound)

	mockService.AssertExpectations(t)
}

func TestListRepositoriesQueryHandler_Handle_Success(t *testing.T) {
	// Arrange
	mockService := new(MockListRepositoriesService)
	handler := NewListRepositoriesQueryHandler(mockService)

	query := ListRepositoriesQuery{
		Status: "completed",
		Limit:  20,
		Offset: 0,
		Sort:   "created_at:desc",
	}

	expectedRequest := dto.RepositoryListQuery{
		Status: "completed",
		Limit:  20,
		Offset: 0,
		Sort:   "created_at:desc",
	}

	expectedResponse := &dto.RepositoryListResponse{
		Repositories: []dto.RepositoryResponse{
			{
				ID:     uuid.New(),
				URL:    "https://github.com/golang/go",
				Name:   "golang/go",
				Status: "completed",
			},
		},
		Pagination: dto.PaginationResponse{
			Limit:   20,
			Offset:  0,
			Total:   1,
			HasMore: false,
		},
	}

	mockService.On("ListRepositories", mock.Anything, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Repositories, 1)
	assert.Equal(t, "golang/go", result.Repositories[0].Name)
	assert.Equal(t, "completed", result.Repositories[0].Status)
	assert.Equal(t, 20, result.Pagination.Limit)
	assert.Equal(t, 1, result.Pagination.Total)

	mockService.AssertExpectations(t)
}

func TestListRepositoriesQueryHandler_Handle_ValidationError(t *testing.T) {
	// Arrange
	mockService := new(MockListRepositoriesService)
	handler := NewListRepositoriesQueryHandler(mockService)

	query := ListRepositoriesQuery{
		Status: "invalid-status", // Invalid status
		Limit:  20,
		Offset: 0,
	}

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation failed")

	mockService.AssertNotCalled(t, "ListRepositories")
}

func TestListRepositoriesQueryHandler_Handle_DefaultValues(t *testing.T) {
	// Arrange
	mockService := new(MockListRepositoriesService)
	handler := NewListRepositoriesQueryHandler(mockService)

	query := ListRepositoriesQuery{
		// No values set - should use defaults
	}

	expectedRequest := dto.RepositoryListQuery{
		Status: "",                // Empty status means no filter
		Limit:  20,                // Default limit
		Offset: 0,                 // Default offset
		Sort:   "created_at:desc", // Default sort
	}

	expectedResponse := &dto.RepositoryListResponse{
		Repositories: []dto.RepositoryResponse{},
		Pagination: dto.PaginationResponse{
			Limit:   20,
			Offset:  0,
			Total:   0,
			HasMore: false,
		},
	}

	mockService.On("ListRepositories", mock.Anything, expectedRequest).Return(expectedResponse, nil)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 20, result.Pagination.Limit)
	assert.Equal(t, 0, result.Pagination.Offset)

	mockService.AssertExpectations(t)
}

func TestListRepositoriesQueryHandler_Handle_DatabaseError(t *testing.T) {
	// Arrange
	mockService := new(MockListRepositoriesService)
	handler := NewListRepositoriesQueryHandler(mockService)

	query := ListRepositoriesQuery{
		Limit:  10,
		Offset: 0,
	}

	expectedRequest := dto.RepositoryListQuery{
		Status: "",
		Limit:  10,
		Offset: 0,
		Sort:   "created_at:desc",
	}

	dbError := errors.New("database connection failed")
	mockService.On("ListRepositories", mock.Anything, expectedRequest).Return(nil, dbError)

	ctx := context.Background()

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, dbError)

	mockService.AssertExpectations(t)
}

// Helper function.
func stringPtr(s string) *string {
	return &s
}
