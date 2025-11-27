package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
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
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	repositoryURL string,
) error {
	args := m.Called(ctx, indexingJobID, repositoryID, repositoryURL)
	return args.Error(0)
}

func TestCreateRepositoryService_CreateRepository_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockJobRepo := new(MockIndexingJobRepository)
	mockPublisher := new(MockMessagePublisher)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL:           "https://github.com/golang/go",
		Name:          "golang/go",
		Description:   stringPtr("The Go programming language"),
		DefaultBranch: stringPtr("master"),
	}

	// Mock expectations
	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
	mockPublisher.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID"), "https://github.com/golang/go.git").
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
	mockJobRepo := new(MockIndexingJobRepository)
	mockPublisher := new(MockMessagePublisher)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

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
	mockJobRepo := new(MockIndexingJobRepository)
	mockPublisher := new(MockMessagePublisher)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

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
	require.ErrorIs(t, err, domainerrors.ErrRepositoryAlreadyExists)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Save")
	mockPublisher.AssertNotCalled(t, "PublishIndexingJob")
}

func TestCreateRepositoryService_CreateRepository_RepositoryExistsCheckFails(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)
	mockJobRepo := new(MockIndexingJobRepository)
	mockPublisher := new(MockMessagePublisher)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

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
	mockJobRepo := new(MockIndexingJobRepository)
	mockPublisher := new(MockMessagePublisher)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

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
	mockJobRepo := new(MockIndexingJobRepository)
	mockPublisher := new(MockMessagePublisher)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}

	publishError := errors.New("message queue unavailable")
	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
	mockPublisher.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID"), "https://github.com/golang/go.git").
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
	mockJobRepo := new(MockIndexingJobRepository)
	mockPublisher := new(MockMessagePublisher)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewCreateRepositoryService(mockRepo, mockJobRepo, mockPublisher)

	request := dto.CreateRepositoryRequest{
		URL: "https://github.com/golang/go",
		// Name is empty - should be auto-generated from URL
	}

	mockRepo.On("Exists", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
	mockPublisher.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID"), "https://github.com/golang/go.git").
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

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewGetRepositoryService(mockRepo)

	repositoryID := uuid.New()
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domainerrors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	response, err := service.GetRepository(ctx, repositoryID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
}

func TestGetRepositoryService_GetRepository_DatabaseError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewUpdateRepositoryService(mockRepo)

	repositoryID := uuid.New()
	request := dto.UpdateRepositoryRequest{
		Name: stringPtr("Updated Name"),
	}

	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domainerrors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	response, err := service.UpdateRepository(ctx, repositoryID, request)

	// Assert
	require.Error(t, err)
	assert.Nil(t, response)
	require.ErrorIs(t, err, domainerrors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestUpdateRepositoryService_UpdateRepository_CannotUpdateProcessingRepository(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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
	require.ErrorIs(t, err, domainerrors.ErrRepositoryProcessing)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestDeleteRepositoryService_DeleteRepository_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, loggerErr := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, loggerErr)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, loggerErr := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, loggerErr)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	service := NewDeleteRepositoryService(mockRepo)

	repositoryID := uuid.New()
	mockRepo.On("FindByID", mock.Anything, repositoryID).Return(nil, domainerrors.ErrRepositoryNotFound)

	ctx := context.Background()

	// Act
	err := service.DeleteRepository(ctx, repositoryID)

	// Assert
	require.Error(t, err)
	require.ErrorIs(t, err, domainerrors.ErrRepositoryNotFound)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestDeleteRepositoryService_DeleteRepository_CannotDeleteProcessingRepository(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, loggerErr := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, loggerErr)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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
	require.ErrorIs(t, err, domainerrors.ErrRepositoryProcessing)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestListRepositoriesService_ListRepositories_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepositoryRepository)

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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

	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

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

// TestListRepositoriesService_ListRepositories_FilterByName tests name filtering functionality.
func TestListRepositoriesService_ListRepositories_FilterByName(t *testing.T) {
	// Set up silent logger for tests to avoid logging side effects
	silentLogger, loggerErr := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, loggerErr)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	tests := []struct {
		name           string
		nameFilter     string
		expectedFilter string
		setupRepos     func() []*entity.Repository
		expectedCount  int
	}{
		{
			name:           "Filter_By_Exact_Name_Match",
			nameFilter:     "golang/go",
			expectedFilter: "golang/go",
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
				return []*entity.Repository{
					entity.RestoreRepository(
						uuid.New(),
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
					),
				}
			},
			expectedCount: 1,
		},
		{
			name:           "Filter_By_Partial_Name_Match",
			nameFilter:     "golang",
			expectedFilter: "golang",
			setupRepos: func() []*entity.Repository {
				testURL1, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
				testURL2, _ := valueobject.NewRepositoryURL("https://github.com/golang/tools")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL1, "golang/go", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
					entity.RestoreRepository(uuid.New(), testURL2, "golang/tools", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 2,
		},
		{
			name:           "Filter_By_Name_Case_Insensitive",
			nameFilter:     "KUBERNETES",
			expectedFilter: "KUBERNETES",
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/kubernetes/kubernetes")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL, "kubernetes/kubernetes", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 1,
		},
		{
			name:           "Filter_By_Name_No_Matches",
			nameFilter:     "nonexistent",
			expectedFilter: "nonexistent",
			setupRepos: func() []*entity.Repository {
				return []*entity.Repository{}
			},
			expectedCount: 0,
		},
		{
			name:           "Filter_By_Name_With_Special_Characters",
			nameFilter:     "repo-name_v2",
			expectedFilter: "repo-name_v2",
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/org/repo-name_v2")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL, "org/repo-name_v2", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := new(MockRepositoryRepository)
			service := NewListRepositoriesService(mockRepo)

			query := dto.RepositoryListQuery{
				Name:   tt.nameFilter,
				Limit:  20,
				Offset: 0,
				Sort:   "created_at:desc",
			}

			repos := tt.setupRepos()
			expectedFilters := outbound.RepositoryFilters{
				Name:   tt.expectedFilter,
				Limit:  20,
				Offset: 0,
				Sort:   "created_at:desc",
			}

			mockRepo.On("FindAll", mock.Anything, expectedFilters).Return(repos, len(repos), nil)

			ctx := context.Background()

			// Act
			response, err := service.ListRepositories(ctx, query)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, response)
			assert.Len(t, response.Repositories, tt.expectedCount)

			mockRepo.AssertExpectations(t)
		})
	}
}

// TestListRepositoriesService_ListRepositories_FilterByURL tests URL filtering functionality.
func TestListRepositoriesService_ListRepositories_FilterByURL(t *testing.T) {
	// Set up silent logger for tests to avoid logging side effects
	silentLogger, loggerErr := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, loggerErr)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)

	tests := []struct {
		name           string
		urlFilter      string
		expectedFilter string
		setupRepos     func() []*entity.Repository
		expectedCount  int
	}{
		{
			name:           "Filter_By_Full_URL",
			urlFilter:      "https://github.com/golang/go",
			expectedFilter: "https://github.com/golang/go",
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL, "golang/go", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 1,
		},
		{
			name:           "Filter_By_Partial_URL_Domain",
			urlFilter:      "github.com",
			expectedFilter: "github.com",
			setupRepos: func() []*entity.Repository {
				testURL1, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
				testURL2, _ := valueobject.NewRepositoryURL("https://github.com/kubernetes/kubernetes")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL1, "golang/go", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
					entity.RestoreRepository(uuid.New(), testURL2, "kubernetes/kubernetes", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 2,
		},
		{
			name:           "Filter_By_Partial_URL_Organization",
			urlFilter:      "golang",
			expectedFilter: "golang",
			setupRepos: func() []*entity.Repository {
				testURL1, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
				testURL2, _ := valueobject.NewRepositoryURL("https://github.com/golang/tools")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL1, "golang/go", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
					entity.RestoreRepository(uuid.New(), testURL2, "golang/tools", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 2,
		},
		{
			name:           "Filter_By_URL_Case_Insensitive",
			urlFilter:      "GITHUB.COM/DOCKER",
			expectedFilter: "GITHUB.COM/DOCKER",
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/docker/docker")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL, "docker/docker", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 1,
		},
		{
			name:           "Filter_By_URL_No_Matches",
			urlFilter:      "bitbucket.org",
			expectedFilter: "bitbucket.org",
			setupRepos: func() []*entity.Repository {
				return []*entity.Repository{}
			},
			expectedCount: 0,
		},
		{
			name:           "Filter_By_URL_With_Git_Extension",
			urlFilter:      ".git",
			expectedFilter: ".git",
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/example/repo.git")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL, "example/repo", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := new(MockRepositoryRepository)
			service := NewListRepositoriesService(mockRepo)

			query := dto.RepositoryListQuery{
				URL:    tt.urlFilter,
				Limit:  20,
				Offset: 0,
				Sort:   "created_at:desc",
			}

			repos := tt.setupRepos()
			expectedFilters := outbound.RepositoryFilters{
				URL:    tt.expectedFilter,
				Limit:  20,
				Offset: 0,
				Sort:   "created_at:desc",
			}

			mockRepo.On("FindAll", mock.Anything, expectedFilters).Return(repos, len(repos), nil)

			ctx := context.Background()

			// Act
			response, err := service.ListRepositories(ctx, query)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, response)
			assert.Len(t, response.Repositories, tt.expectedCount)

			mockRepo.AssertExpectations(t)
		})
	}
}

// TestListRepositoriesService_ListRepositories_CombinedFilters tests combined filtering scenarios.
func TestListRepositoriesService_ListRepositories_CombinedFilters(t *testing.T) {
	// Set up silent logger for tests to avoid logging side effects
	silentLogger, loggerErr := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, loggerErr)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)
	tests := []struct {
		name           string
		query          dto.RepositoryListQuery
		setupRepos     func() []*entity.Repository
		expectedCount  int
		expectedFilter outbound.RepositoryFilters
	}{
		{
			name: "Filter_By_Name_And_URL",
			query: dto.RepositoryListQuery{
				Name:   "golang",
				URL:    "github.com",
				Limit:  20,
				Offset: 0,
				Sort:   "created_at:desc",
			},
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL, "golang/go", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 1,
			expectedFilter: outbound.RepositoryFilters{
				Name:   "golang",
				URL:    "github.com",
				Limit:  20,
				Offset: 0,
				Sort:   "created_at:desc",
			},
		},
		{
			name: "Filter_By_Name_URL_And_Status",
			query: dto.RepositoryListQuery{
				Status: "completed",
				Name:   "kubernetes",
				URL:    "github.com",
				Limit:  20,
				Offset: 0,
				Sort:   "name:asc",
			},
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/kubernetes/kubernetes")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL, "kubernetes/kubernetes", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 1,
			expectedFilter: outbound.RepositoryFilters{
				Status: func() *valueobject.RepositoryStatus {
					s := valueobject.RepositoryStatusCompleted
					return &s
				}(),
				Name:   "kubernetes",
				URL:    "github.com",
				Limit:  20,
				Offset: 0,
				Sort:   "name:asc",
			},
		},
		{
			name: "Filter_Returns_No_Results_When_Criteria_Dont_Match",
			query: dto.RepositoryListQuery{
				Name:   "golang",
				URL:    "bitbucket.org",
				Limit:  20,
				Offset: 0,
				Sort:   "created_at:desc",
			},
			setupRepos: func() []*entity.Repository {
				return []*entity.Repository{}
			},
			expectedCount: 0,
			expectedFilter: outbound.RepositoryFilters{
				Name:   "golang",
				URL:    "bitbucket.org",
				Limit:  20,
				Offset: 0,
				Sort:   "created_at:desc",
			},
		},
		{
			name: "Filter_With_Pagination_And_Name_URL",
			query: dto.RepositoryListQuery{
				Name:   "prometheus",
				URL:    "github.com",
				Limit:  10,
				Offset: 5,
				Sort:   "updated_at:desc",
			},
			setupRepos: func() []*entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/prometheus/prometheus")
				return []*entity.Repository{
					entity.RestoreRepository(uuid.New(), testURL, "prometheus/prometheus", nil, nil, nil, nil, 0, 0,
						valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
				}
			},
			expectedCount: 1,
			expectedFilter: outbound.RepositoryFilters{
				Name:   "prometheus",
				URL:    "github.com",
				Limit:  10,
				Offset: 5,
				Sort:   "updated_at:desc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := new(MockRepositoryRepository)
			service := NewListRepositoriesService(mockRepo)

			repos := tt.setupRepos()
			mockRepo.On("FindAll", mock.Anything, tt.expectedFilter).Return(repos, len(repos), nil)

			ctx := context.Background()

			// Act
			response, err := service.ListRepositories(ctx, tt.query)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, response)
			assert.Len(t, response.Repositories, tt.expectedCount)

			mockRepo.AssertExpectations(t)
		})
	}
}

// TestListRepositoriesService_ListRepositories_EdgeCases tests edge case scenarios for filtering.
func TestListRepositoriesService_ListRepositories_EdgeCases(t *testing.T) {
	// Set up silent logger for tests to avoid logging side effects
	silentLogger, loggerErr := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, loggerErr)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)
	t.Run("Empty_Name_Filter_Returns_All_Repositories", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepositoryRepository)
		service := NewListRepositoriesService(mockRepo)

		testURL1, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
		testURL2, _ := valueobject.NewRepositoryURL("https://github.com/kubernetes/kubernetes")

		repositories := []*entity.Repository{
			entity.RestoreRepository(uuid.New(), testURL1, "golang/go", nil, nil, nil, nil, 0, 0,
				valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
			entity.RestoreRepository(uuid.New(), testURL2, "kubernetes/kubernetes", nil, nil, nil, nil, 0, 0,
				valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
		}

		query := dto.RepositoryListQuery{
			Name:   "",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		expectedFilters := outbound.RepositoryFilters{
			Name:   "",
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

		mockRepo.AssertExpectations(t)
	})

	t.Run("Empty_URL_Filter_Returns_All_Repositories", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepositoryRepository)
		service := NewListRepositoriesService(mockRepo)

		testURL1, _ := valueobject.NewRepositoryURL("https://github.com/golang/go")
		testURL2, _ := valueobject.NewRepositoryURL("https://bitbucket.org/example/repo")

		repositories := []*entity.Repository{
			entity.RestoreRepository(uuid.New(), testURL1, "golang/go", nil, nil, nil, nil, 0, 0,
				valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
			entity.RestoreRepository(uuid.New(), testURL2, "example/repo", nil, nil, nil, nil, 0, 0,
				valueobject.RepositoryStatusCompleted, time.Now(), time.Now(), nil),
		}

		query := dto.RepositoryListQuery{
			URL:    "",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		expectedFilters := outbound.RepositoryFilters{
			URL:    "",
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

		mockRepo.AssertExpectations(t)
	})

	t.Run("Whitespace_Only_Name_Filter_Treated_As_Search_Term", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepositoryRepository)
		service := NewListRepositoriesService(mockRepo)

		query := dto.RepositoryListQuery{
			Name:   "   ",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		expectedFilters := outbound.RepositoryFilters{
			Name:   "   ",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		mockRepo.On("FindAll", mock.Anything, expectedFilters).Return([]*entity.Repository{}, 0, nil)

		ctx := context.Background()

		// Act
		response, err := service.ListRepositories(ctx, query)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Empty(t, response.Repositories)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Whitespace_Only_URL_Filter_Treated_As_Search_Term", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepositoryRepository)
		service := NewListRepositoriesService(mockRepo)

		query := dto.RepositoryListQuery{
			URL:    "   ",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		expectedFilters := outbound.RepositoryFilters{
			URL:    "   ",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		mockRepo.On("FindAll", mock.Anything, expectedFilters).Return([]*entity.Repository{}, 0, nil)

		ctx := context.Background()

		// Act
		response, err := service.ListRepositories(ctx, query)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Empty(t, response.Repositories)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Special_Characters_In_Name_Filter", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepositoryRepository)
		service := NewListRepositoriesService(mockRepo)

		testURL, _ := valueobject.NewRepositoryURL("https://github.com/org/repo-name_v2.0")

		repository := entity.RestoreRepository(
			uuid.New(),
			testURL,
			"org/repo-name_v2.0",
			nil,
			nil,
			nil,
			nil,
			0,
			0,
			valueobject.RepositoryStatusCompleted,
			time.Now(),
			time.Now(),
			nil,
		)

		query := dto.RepositoryListQuery{
			Name:   "repo-name_v2.0",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		expectedFilters := outbound.RepositoryFilters{
			Name:   "repo-name_v2.0",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		mockRepo.On("FindAll", mock.Anything, expectedFilters).Return([]*entity.Repository{repository}, 1, nil)

		ctx := context.Background()

		// Act
		response, err := service.ListRepositories(ctx, query)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Len(t, response.Repositories, 1)
		assert.Equal(t, "org/repo-name_v2.0", response.Repositories[0].Name)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Special_Characters_In_URL_Filter", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepositoryRepository)
		service := NewListRepositoriesService(mockRepo)

		testURL, _ := valueobject.NewRepositoryURL("https://github.com/org/repo-name_v2.0.git")

		repository := entity.RestoreRepository(
			uuid.New(),
			testURL,
			"org/repo-name_v2.0",
			nil,
			nil,
			nil,
			nil,
			0,
			0,
			valueobject.RepositoryStatusCompleted,
			time.Now(),
			time.Now(),
			nil,
		)

		query := dto.RepositoryListQuery{
			URL:    "repo-name_v2.0.git",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		expectedFilters := outbound.RepositoryFilters{
			URL:    "repo-name_v2.0.git",
			Limit:  20,
			Offset: 0,
			Sort:   "created_at:desc",
		}

		mockRepo.On("FindAll", mock.Anything, expectedFilters).Return([]*entity.Repository{repository}, 1, nil)

		ctx := context.Background()

		// Act
		response, err := service.ListRepositories(ctx, query)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Len(t, response.Repositories, 1)

		mockRepo.AssertExpectations(t)
	})
}

// Helper functions.
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
