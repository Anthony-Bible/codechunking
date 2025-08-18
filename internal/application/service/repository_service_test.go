package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

	domain_errors "codechunking/internal/domain/errors/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock repository for testing.
type MockRepositoryRepository struct {
	mock.Mock
}

func (m *MockRepositoryRepository) Save(ctx context.Context, repository *entity.Repository) error {
	args := m.Called(ctx, repository)
	return args.Error(0)
}

func (m *MockRepositoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *MockRepositoryRepository) FindByURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *MockRepositoryRepository) FindAll(
	ctx context.Context,
	filters outbound.RepositoryFilters,
) ([]*entity.Repository, int, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*entity.Repository), args.Int(1), args.Error(2)
}

func (m *MockRepositoryRepository) Update(ctx context.Context, repository *entity.Repository) error {
	args := m.Called(ctx, repository)
	return args.Error(0)
}

func (m *MockRepositoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepositoryRepository) Exists(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepositoryRepository) ExistsByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepositoryRepository) FindByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

// Mock message publisher for testing.
type MockMessagePublisher struct {
	mock.Mock
}

func (m *MockMessagePublisher) PublishIndexingJob(
	ctx context.Context,
	repositoryID uuid.UUID,
	repositoryURL string,
) error {
	args := m.Called(ctx, repositoryID, repositoryURL)
	return args.Error(0)
}

func TestCreateRepositoryService_CreateRepository_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateRepositoryService(mockRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL:           "https://github.com/golang/go",
		Name:          "golang/go",
		Description:   stringPtr("The Go programming language"),
		DefaultBranch: stringPtr("master"),
	}

	// Mock expectations
	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
	mockPublisher.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), "https://github.com/golang/go").
		Return(nil)

	ctx := context.Background()

	// Act
	response, err := service.CreateRepository(ctx, request)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "https://github.com/golang/go", response.URL)
	assert.Equal(t, "golang/go", response.Name)
	assert.Equal(t, "The Go programming language", *response.Description)
	assert.Equal(t, "master", *response.DefaultBranch)
	assert.Equal(t, "pending", response.Status)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, 0, response.TotalFiles)
	assert.Equal(t, 0, response.TotalChunks)
	assert.NotZero(t, response.CreatedAt)
	assert.NotZero(t, response.UpdatedAt)

	mockRepo.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestCreateRepositoryService_CreateRepository_InvalidURL(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateRepositoryService(mockRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL:  "invalid-url",
		Name: "test-repo",
	}

	ctx := context.Background()

	// Act
	response, err := service.CreateRepository(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid repository URL")

	// No repository calls should be made for invalid input
	mockRepo.AssertNotCalled(t, "Exists")
	mockRepo.AssertNotCalled(t, "Save")
	mockPublisher.AssertNotCalled(t, "PublishIndexingJob")
}

func TestCreateRepositoryService_CreateRepository_RepositoryAlreadyExists(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateRepositoryService(mockRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}

	// Mock expectations
	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(true, nil)

	ctx := context.Background()

	// Act
	response, err := service.CreateRepository(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryAlreadyExists)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Save")
	mockPublisher.AssertNotCalled(t, "PublishIndexingJob")
}

func TestCreateRepositoryService_CreateRepository_RepositoryExistsCheckFails(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateRepositoryService(mockRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}

	dbError := errors.New("database connection failed")
	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, dbError)

	ctx := context.Background()

	// Act
	response, err := service.CreateRepository(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to check if repository exists")
	require.ErrorIs(t, err, dbError)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Save")
	mockPublisher.AssertNotCalled(t, "PublishIndexingJob")
}

func TestCreateRepositoryService_CreateRepository_SaveFails(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateRepositoryService(mockRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}

	saveError := errors.New("database save failed")
	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(saveError)

	ctx := context.Background()

	// Act
	response, err := service.CreateRepository(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to save repository")
	require.ErrorIs(t, err, saveError)

	mockRepo.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "PublishIndexingJob")
}

func TestCreateRepositoryService_CreateRepository_PublishJobFails(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateRepositoryService(mockRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}

	publishError := errors.New("message queue unavailable")
	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
	mockPublisher.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), "https://github.com/golang/go").
		Return(publishError)

	ctx := context.Background()

	// Act
	response, err := service.CreateRepository(ctx, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to publish indexing job")
	require.ErrorIs(t, err, publishError)

	mockRepo.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestCreateRepositoryService_CreateRepository_AutoGeneratesNameFromURL(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockPublisher := new(MockMessagePublisher)
	service := NewCreateRepositoryService(mockRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL: "https://github.com/golang/go",
		// Name is empty - should be auto-generated from URL
	}

	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
	mockPublisher.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), "https://github.com/golang/go").
		Return(nil)

	ctx := context.Background()

	// Act
	response, err := service.CreateRepository(ctx, request)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "golang/go", response.Name) // Should be auto-generated from URL

	mockRepo.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestGetRepositoryService_GetRepository_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewGetRepositoryService(mockRepo)

	repositoryID := uuid.New()
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

	repository := entity.RestoreRepository(
		repositoryID,
		testURL,
		"golang/go",
		stringPtr("The Go programming language"),
		stringPtr("master"),
		timePtr(time.Now().Add(-time.Hour)),
		stringPtr("abc123def456"),
		1250,
		5000,
		valueobject.RepositoryStatusCompleted,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)

	ctx := context.Background()

	// Act
	response, err := service.GetRepository(ctx, repositoryID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, repositoryID, response.ID)
	assert.Equal(t, "https://github.com/golang/go", response.URL)
	assert.Equal(t, "golang/go", response.Name)
	assert.Equal(t, "The Go programming language", *response.Description)
	assert.Equal(t, "master", *response.DefaultBranch)
	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, 1250, response.TotalFiles)
	assert.Equal(t, 5000, response.TotalChunks)
	assert.Equal(t, "abc123def456", *response.LastCommitHash)
	assert.NotNil(t, response.LastIndexedAt)

	mockRepo.AssertExpectations(t)
}

func TestGetRepositoryService_GetRepository_NotFound(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewGetRepositoryService(mockRepo)

	repositoryID := uuid.New()
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domain_errors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	response, err := service.GetRepository(ctx, repositoryID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
}

func TestGetRepositoryService_GetRepository_DatabaseError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewGetRepositoryService(mockRepo)

	repositoryID := uuid.New()
	dbError := errors.New("database connection failed")
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, dbError)

	ctx := context.Background()

	// Act
	response, err := service.GetRepository(ctx, repositoryID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to retrieve repository")
	require.ErrorIs(t, err, dbError)

	mockRepo.AssertExpectations(t)
}

func TestUpdateRepositoryService_UpdateRepository_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewUpdateRepositoryService(mockRepo)

	repositoryID := uuid.New()
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

	repository := entity.RestoreRepository(
		repositoryID,
		testURL,
		"golang/go",
		stringPtr("Old description"),
		stringPtr("main"),
		nil,
		nil,
		0,
		0,
		valueobject.RepositoryStatusPending,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	request := dto.UpdateRepositoryRequest{
		Name:          stringPtr("Updated Name"),
		Description:   stringPtr("Updated description"),
		DefaultBranch: stringPtr("master"),
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)

	ctx := context.Background()

	// Act
	response, err := service.UpdateRepository(ctx, repositoryID, request)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, repositoryID, response.ID)
	assert.Equal(t, "Updated Name", response.Name)
	assert.Equal(t, "Updated description", *response.Description)
	assert.Equal(t, "master", *response.DefaultBranch)

	mockRepo.AssertExpectations(t)
}

func TestUpdateRepositoryService_UpdateRepository_NotFound(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewUpdateRepositoryService(mockRepo)

	repositoryID := uuid.New()
	request := dto.UpdateRepositoryRequest{
		Name: stringPtr("Updated Name"),
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domain_errors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	response, err := service.UpdateRepository(ctx, repositoryID, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestUpdateRepositoryService_UpdateRepository_CannotUpdateProcessingRepository(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewUpdateRepositoryService(mockRepo)

	repositoryID := uuid.New()
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")

	// Repository in processing state
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

	request := dto.UpdateRepositoryRequest{
		Name: stringPtr("Updated Name"),
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)

	ctx := context.Background()

	// Act
	response, err := service.UpdateRepository(ctx, repositoryID, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryProcessing)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestDeleteRepositoryService_DeleteRepository_Success(t *testing.T) {
	// Arrange
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
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)

	ctx := context.Background()

	// Act
	err := service.DeleteRepository(ctx, repositoryID)

	// Assert
	require.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestDeleteRepositoryService_DeleteRepository_NotFound(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewDeleteRepositoryService(mockRepo)

	repositoryID := uuid.New()
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domain_errors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	err := service.DeleteRepository(ctx, repositoryID)

	// Assert
	require.Error(t, err)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestDeleteRepositoryService_DeleteRepository_CannotDeleteProcessingRepository(t *testing.T) {
	// Arrange
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
		valueobject.RepositoryStatusProcessing,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(repository, nil)

	ctx := context.Background()

	// Act
	err := service.DeleteRepository(ctx, repositoryID)

	// Assert
	require.Error(t, err)
	require.ErrorIs(t, err, domain_errors.ErrRepositoryProcessing)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestListRepositoriesService_ListRepositories_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewListRepositoriesService(mockRepo)

	testURL1, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
	testURL2, _ := valueobject.NewRepositoryURL("https://github.com/kubernetes/kubernetes")

	repositories := []*entity.Repository{
		entity.RestoreRepository(
			uuid.New(),
			testURL1,
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
		),
		entity.RestoreRepository(
			uuid.New(),
			testURL2,
			"kubernetes/kubernetes",
			nil,
			nil,
			nil,
			nil,
			0,
			0,
			valueobject.RepositoryStatusPending,
			time.Now().Add(-12*time.Hour),
			time.Now().Add(-30*time.Minute),
			nil,
		),
	}

	query := dto.RepositoryListQuery{
		Status: "completed",
		Limit:  20,
		Offset: 0,
		Sort:   "created_at:desc",
	}

	completedStatus := valueobject.RepositoryStatusCompleted
	expectedFilters := outbound.RepositoryFilters{
		Status: &completedStatus,
		Limit:  20,
		Offset: 0,
		Sort:   "created_at:desc",
	}

	mockRepo.On("FindAll", mock.Anything, expectedFilters).Return(repositories, 2, nil)

	ctx := context.Background()

	// Act
	response, err := service.ListRepositories(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Repositories, 2)
	assert.Equal(t, "golang/go", response.Repositories[0].Name)
	assert.Equal(t, "kubernetes/kubernetes", response.Repositories[1].Name)
	assert.Equal(t, 20, response.Pagination.Limit)
	assert.Equal(t, 0, response.Pagination.Offset)
	assert.Equal(t, 2, response.Pagination.Total)
	assert.False(t, response.Pagination.HasMore)

	mockRepo.AssertExpectations(t)
}

func TestListRepositoriesService_ListRepositories_WithPagination(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewListRepositoriesService(mockRepo)

	query := dto.RepositoryListQuery{
		Limit:  10,
		Offset: 20,
	}

	expectedFilters := outbound.RepositoryFilters{
		Limit:  10,
		Offset: 20,
		Sort:   "created_at:desc", // Default sort
	}

	mockRepo.On("FindAll", mock.Anything, expectedFilters).Return([]*entity.Repository{}, 50, nil)

	ctx := context.Background()

	// Act
	response, err := service.ListRepositories(ctx, query)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 10, response.Pagination.Limit)
	assert.Equal(t, 20, response.Pagination.Offset)
	assert.Equal(t, 50, response.Pagination.Total)
	assert.True(t, response.Pagination.HasMore) // 20 + 10 < 50

	mockRepo.AssertExpectations(t)
}

func TestListRepositoriesService_ListRepositories_InvalidStatus(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewListRepositoriesService(mockRepo)

	query := dto.RepositoryListQuery{
		Status: "invalid-status",
	}

	ctx := context.Background()

	// Act
	response, err := service.ListRepositories(ctx, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid repository status")

	mockRepo.AssertNotCalled(t, "FindAll")
}

func TestListRepositoriesService_ListRepositories_DatabaseError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	service := NewListRepositoriesService(mockRepo)

	query := dto.RepositoryListQuery{
		Limit:  20,
		Offset: 0,
	}

	dbError := errors.New("database connection failed")
	mockRepo.On("FindAll", mock.Anything, mock.AnythingOfType("outbound.RepositoryFilters")).Return(nil, 0, dbError)

	ctx := context.Background()

	// Act
	response, err := service.ListRepositories(ctx, query)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to retrieve repositories")
	require.ErrorIs(t, err, dbError)

	mockRepo.AssertExpectations(t)
}

// Helper functions.
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
