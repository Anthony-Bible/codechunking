package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	domainerrors "codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockGitProvider is a testify mock for outbound.GitProvider.
type MockGitProvider struct {
	mock.Mock
}

func (m *MockGitProvider) ListRepositories(ctx context.Context, connector *entity.Connector) ([]outbound.GitProviderRepository, error) {
	args := m.Called(ctx, connector)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]outbound.GitProviderRepository), args.Error(1)
}

func (m *MockGitProvider) ValidateCredentials(ctx context.Context, connector *entity.Connector) error {
	args := m.Called(ctx, connector)
	return args.Error(0)
}

// MockSyncRepoRepository is a testify mock for outbound.RepositoryRepository.
type MockSyncRepoRepository struct {
	mock.Mock
}

func (m *MockSyncRepoRepository) Save(ctx context.Context, repo *entity.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockSyncRepoRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *MockSyncRepoRepository) FindByURL(ctx context.Context, url valueobject.RepositoryURL) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *MockSyncRepoRepository) FindAll(ctx context.Context, filters outbound.RepositoryFilters) ([]*entity.Repository, int, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*entity.Repository), args.Int(1), args.Error(2)
}

func (m *MockSyncRepoRepository) Update(ctx context.Context, repo *entity.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockSyncRepoRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSyncRepoRepository) Exists(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

func (m *MockSyncRepoRepository) ExistsByNormalizedURL(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

func (m *MockSyncRepoRepository) FindByNormalizedURL(ctx context.Context, url valueobject.RepositoryURL) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

// MockSyncIndexingJobRepo is a testify mock for outbound.IndexingJobRepository.
type MockSyncIndexingJobRepo struct {
	mock.Mock
}

func (m *MockSyncIndexingJobRepo) Save(ctx context.Context, job *entity.IndexingJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockSyncIndexingJobRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.IndexingJob, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.IndexingJob), args.Error(1)
}

func (m *MockSyncIndexingJobRepo) FindByRepositoryID(ctx context.Context, repoID uuid.UUID, filters outbound.IndexingJobFilters) ([]*entity.IndexingJob, int, error) {
	args := m.Called(ctx, repoID, filters)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*entity.IndexingJob), args.Int(1), args.Error(2)
}

func (m *MockSyncIndexingJobRepo) Update(ctx context.Context, job *entity.IndexingJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockSyncIndexingJobRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockSyncMessagePublisher is a testify mock for outbound.MessagePublisher.
type MockSyncMessagePublisher struct {
	mock.Mock
}

func (m *MockSyncMessagePublisher) PublishIndexingJob(ctx context.Context, indexingJobID, repositoryID uuid.UUID, repositoryURL string) error {
	args := m.Called(ctx, indexingJobID, repositoryID, repositoryURL)
	return args.Error(0)
}

var errInternalTest = errors.New("internal test error")

// MockConnectorRepository is a testify mock for outbound.ConnectorRepository.
type MockConnectorRepository struct {
	mock.Mock
}

func (m *MockConnectorRepository) Save(ctx context.Context, connector *entity.Connector) error {
	args := m.Called(ctx, connector)
	return args.Error(0)
}

func (m *MockConnectorRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Connector, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Connector), args.Error(1)
}

func (m *MockConnectorRepository) FindByName(ctx context.Context, name string) (*entity.Connector, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Connector), args.Error(1)
}

func (m *MockConnectorRepository) FindAll(
	ctx context.Context,
	filters outbound.ConnectorFilters,
) ([]*entity.Connector, int, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*entity.Connector), args.Int(1), args.Error(2)
}

func (m *MockConnectorRepository) Update(ctx context.Context, connector *entity.Connector) error {
	args := m.Called(ctx, connector)
	return args.Error(0)
}

func (m *MockConnectorRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConnectorRepository) Exists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

// silentConnectorLogger sets up a silent logger for tests.
func silentConnectorLogger(t *testing.T) {
	t.Helper()
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR",
		Format: "json",
		Output: "buffer",
	})
	require.NoError(t, err)
	slogger.SetGlobalLogger(silentLogger)
}

// buildTestConnector creates a test connector entity in active status.
func buildTestConnector(t *testing.T, id uuid.UUID, name string) *entity.Connector {
	t.Helper()
	now := time.Now()
	return entity.RestoreConnector(
		id,
		name,
		valueobject.ConnectorTypeGitLab,
		"https://gitlab.com",
		nil,
		valueobject.ConnectorStatusActive,
		5,
		nil,
		now,
		now,
		[]string{},
		[]string{},
	)
}

// =============================================================================
// GetConnector tests
// =============================================================================

func TestGetConnectorService_GetConnector_Success(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewGetConnectorService(mockRepo)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "my-connector")

	mockRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)

	ctx := context.Background()
	response, err := service.GetConnector(ctx, connectorID)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, connectorID, response.ID)
	assert.Equal(t, "my-connector", response.Name)

	mockRepo.AssertExpectations(t)
}

func TestGetConnectorService_GetConnector_NotFound(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewGetConnectorService(mockRepo)

	connectorID := uuid.New()
	mockRepo.On("FindByID", mock.Anything, connectorID).Return(nil, domainerrors.ErrConnectorNotFound)

	ctx := context.Background()
	response, err := service.GetConnector(ctx, connectorID)

	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, domainerrors.ErrConnectorNotFound)

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// ListConnectors tests
// =============================================================================

func TestListConnectorsService_ListConnectors_Success(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewListConnectorsService(mockRepo)

	connectors := []*entity.Connector{
		buildTestConnector(t, uuid.New(), "connector-1"),
		buildTestConnector(t, uuid.New(), "connector-2"),
	}

	query := dto.DefaultConnectorListQuery()
	mockRepo.On("FindAll", mock.Anything, mock.AnythingOfType("outbound.ConnectorFilters")).
		Return(connectors, 2, nil)

	ctx := context.Background()
	response, err := service.ListConnectors(ctx, query)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Len(t, response.Connectors, 2)
	assert.Equal(t, 2, response.Pagination.Total)

	mockRepo.AssertExpectations(t)
}

func TestListConnectorsService_ListConnectors_Empty(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewListConnectorsService(mockRepo)

	query := dto.DefaultConnectorListQuery()
	mockRepo.On("FindAll", mock.Anything, mock.AnythingOfType("outbound.ConnectorFilters")).
		Return([]*entity.Connector{}, 0, nil)

	ctx := context.Background()
	response, err := service.ListConnectors(ctx, query)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Empty(t, response.Connectors)
	assert.Equal(t, 0, response.Pagination.Total)

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// SyncConnector tests
// =============================================================================

// buildSyncService is a helper that constructs a SyncConnectorService with all deps.
func buildSyncService(
	connectorRepo *MockConnectorRepository,
	gitProvider *MockGitProvider,
	repoRepo *MockSyncRepoRepository,
	jobRepo *MockSyncIndexingJobRepo,
	publisher *MockSyncMessagePublisher,
) *SyncConnectorService {
	return NewSyncConnectorService(connectorRepo, gitProvider, repoRepo, jobRepo, publisher)
}

func TestSyncConnectorService_SyncConnector_Success(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	mockGit := new(MockGitProvider)
	service := buildSyncService(mockRepo, mockGit, nil, nil, nil)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "my-connector")

	mockRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)
	mockGit.On("ListRepositories", mock.Anything, connector).Return([]outbound.GitProviderRepository{}, nil)

	ctx := context.Background()
	response, err := service.SyncConnector(ctx, connectorID)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, connectorID, response.ConnectorID)
	assert.NotEmpty(t, response.Message)

	mockRepo.AssertExpectations(t)
	mockGit.AssertExpectations(t)
}

func TestSyncConnectorService_SyncConnector_NotFound(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := buildSyncService(mockRepo, nil, nil, nil, nil)

	connectorID := uuid.New()
	mockRepo.On("FindByID", mock.Anything, connectorID).Return(nil, domainerrors.ErrConnectorNotFound)

	ctx := context.Background()
	response, err := service.SyncConnector(ctx, connectorID)

	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, domainerrors.ErrConnectorNotFound)

	mockRepo.AssertExpectations(t)
}

func TestSyncConnectorService_SyncConnector_AlreadySyncing(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := buildSyncService(mockRepo, nil, nil, nil, nil)

	connectorID := uuid.New()
	now := time.Now()
	syncingConnector := entity.RestoreConnector(
		connectorID,
		"syncing-connector",
		valueobject.ConnectorTypeGitLab,
		"https://gitlab.com",
		nil,
		valueobject.ConnectorStatusSyncing,
		0,
		nil,
		now,
		now,
		[]string{},
		[]string{},
	)

	mockRepo.On("FindByID", mock.Anything, connectorID).Return(syncingConnector, nil)

	ctx := context.Background()
	response, err := service.SyncConnector(ctx, connectorID)

	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, domainerrors.ErrConnectorSyncing)

	mockRepo.AssertExpectations(t)
}

func TestSyncConnector_FetchesAndCreatesRepositories(t *testing.T) {
	silentConnectorLogger(t)

	mockConnRepo := new(MockConnectorRepository)
	mockGit := new(MockGitProvider)
	mockRepoRepo := new(MockSyncRepoRepository)
	mockJobRepo := new(MockSyncIndexingJobRepo)
	mockPub := new(MockSyncMessagePublisher)
	svc := buildSyncService(mockConnRepo, mockGit, mockRepoRepo, mockJobRepo, mockPub)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "gl-connector")

	providerRepos := []outbound.GitProviderRepository{
		{Name: "repo-a", URL: "https://gitlab.com/group/repo-a.git", Description: "desc-a", DefaultBranch: "main"},
		{Name: "repo-b", URL: "https://gitlab.com/group/repo-b.git", Description: "desc-b", DefaultBranch: "main"},
	}

	mockConnRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockConnRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)
	mockGit.On("ListRepositories", mock.Anything, connector).Return(providerRepos, nil)
	mockRepoRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepoRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
	mockPub.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).Return(nil)

	ctx := context.Background()
	resp, err := svc.SyncConnector(ctx, connectorID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, connectorID, resp.ConnectorID)
	assert.Equal(t, 2, resp.RepositoriesFound)
	assert.Equal(t, "sync completed", resp.Message)

	mockConnRepo.AssertExpectations(t)
	mockGit.AssertExpectations(t)
	mockRepoRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
	mockPub.AssertExpectations(t)
}

func TestSyncConnector_SkipsDuplicates(t *testing.T) {
	silentConnectorLogger(t)

	mockConnRepo := new(MockConnectorRepository)
	mockGit := new(MockGitProvider)
	mockRepoRepo := new(MockSyncRepoRepository)
	mockJobRepo := new(MockSyncIndexingJobRepo)
	mockPub := new(MockSyncMessagePublisher)
	svc := buildSyncService(mockConnRepo, mockGit, mockRepoRepo, mockJobRepo, mockPub)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "gl-connector")

	providerRepos := []outbound.GitProviderRepository{
		{Name: "repo-a", URL: "https://gitlab.com/group/repo-a.git", DefaultBranch: "main"},
		{Name: "repo-b", URL: "https://gitlab.com/group/repo-b.git", DefaultBranch: "main"},
		{Name: "repo-c", URL: "https://gitlab.com/group/repo-c.git", DefaultBranch: "main"},
	}

	mockConnRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockConnRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)
	mockGit.On("ListRepositories", mock.Anything, connector).Return(providerRepos, nil)
	// repo-b already exists; URLs are normalized (no .git suffix)
	mockRepoRepo.On("ExistsByNormalizedURL", mock.Anything, mock.MatchedBy(func(u valueobject.RepositoryURL) bool {
		return u.String() == "https://gitlab.com/group/repo-a"
	})).Return(false, nil)
	mockRepoRepo.On("ExistsByNormalizedURL", mock.Anything, mock.MatchedBy(func(u valueobject.RepositoryURL) bool {
		return u.String() == "https://gitlab.com/group/repo-b"
	})).Return(true, nil)
	mockRepoRepo.On("ExistsByNormalizedURL", mock.Anything, mock.MatchedBy(func(u valueobject.RepositoryURL) bool {
		return u.String() == "https://gitlab.com/group/repo-c"
	})).Return(false, nil)
	// repo-b exists with no pending jobs (already indexed) — republishPendingJob finds nothing
	repoBID := uuid.New()
	repoBURL, _ := valueobject.NewRepositoryURL("https://gitlab.com/group/repo-b.git")
	repoBEntity := entity.RestoreRepository(repoBID, repoBURL, "repo-b", nil, nil, nil, nil, 0, 0, valueobject.RepositoryStatusCompleted, valueobject.ZoektIndexStatusPending, valueobject.EmbeddingIndexStatusCompleted, nil, 0, nil, time.Now(), time.Now(), nil)
	mockRepoRepo.On("FindByNormalizedURL", mock.Anything, mock.MatchedBy(func(u valueobject.RepositoryURL) bool {
		return u.String() == "https://gitlab.com/group/repo-b"
	})).Return(repoBEntity, nil)
	mockJobRepo.On("FindByRepositoryID", mock.Anything, repoBID, mock.AnythingOfType("outbound.IndexingJobFilters")).Return([]*entity.IndexingJob{}, 0, nil)
	mockRepoRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Repository")).Return(nil)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
	mockPub.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).Return(nil)

	ctx := context.Background()
	resp, err := svc.SyncConnector(ctx, connectorID)

	require.NoError(t, err)
	assert.Equal(t, 3, resp.RepositoriesFound)

	mockConnRepo.AssertExpectations(t)
	mockGit.AssertExpectations(t)
	mockRepoRepo.AssertExpectations(t)
}

func TestSyncConnector_ListRepositoriesFails(t *testing.T) {
	silentConnectorLogger(t)

	mockConnRepo := new(MockConnectorRepository)
	mockGit := new(MockGitProvider)
	svc := buildSyncService(mockConnRepo, mockGit, nil, nil, nil)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "gl-connector")

	mockConnRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockConnRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)
	mockGit.On("ListRepositories", mock.Anything, connector).Return(nil, errInternalTest)

	ctx := context.Background()
	resp, err := svc.SyncConnector(ctx, connectorID)

	require.Error(t, err)
	assert.Nil(t, resp)

	mockConnRepo.AssertExpectations(t)
	mockGit.AssertExpectations(t)
}

func TestSyncConnector_PartialFailure(t *testing.T) {
	silentConnectorLogger(t)

	mockConnRepo := new(MockConnectorRepository)
	mockGit := new(MockGitProvider)
	mockRepoRepo := new(MockSyncRepoRepository)
	mockJobRepo := new(MockSyncIndexingJobRepo)
	mockPub := new(MockSyncMessagePublisher)
	svc := buildSyncService(mockConnRepo, mockGit, mockRepoRepo, mockJobRepo, mockPub)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "gl-connector")

	providerRepos := []outbound.GitProviderRepository{
		{Name: "repo-ok", URL: "https://gitlab.com/group/repo-ok.git", DefaultBranch: "main"},
		{Name: "repo-fail", URL: "https://gitlab.com/group/repo-fail.git", DefaultBranch: "main"},
	}

	mockConnRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockConnRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)
	mockGit.On("ListRepositories", mock.Anything, connector).Return(providerRepos, nil)
	mockRepoRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(false, nil)
	mockRepoRepo.On("Save", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		return r.Name() == "repo-ok"
	})).Return(nil)
	mockRepoRepo.On("Save", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		return r.Name() == "repo-fail"
	})).Return(errInternalTest)
	mockJobRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
	mockPub.On("PublishIndexingJob", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).Return(nil)

	ctx := context.Background()
	resp, err := svc.SyncConnector(ctx, connectorID)

	require.NoError(t, err)
	assert.Equal(t, 2, resp.RepositoriesFound)

	mockConnRepo.AssertExpectations(t)
}

func TestSyncConnector_AllReposExist(t *testing.T) {
	silentConnectorLogger(t)

	mockConnRepo := new(MockConnectorRepository)
	mockGit := new(MockGitProvider)
	mockRepoRepo := new(MockSyncRepoRepository)
	mockJobRepo := new(MockSyncIndexingJobRepo)
	svc := buildSyncService(mockConnRepo, mockGit, mockRepoRepo, mockJobRepo, nil)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "gl-connector")

	providerRepos := []outbound.GitProviderRepository{
		{Name: "repo-a", URL: "https://gitlab.com/group/repo-a.git", DefaultBranch: "main"},
		{Name: "repo-b", URL: "https://gitlab.com/group/repo-b.git", DefaultBranch: "main"},
	}

	mockConnRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockConnRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)
	mockGit.On("ListRepositories", mock.Anything, connector).Return(providerRepos, nil)
	mockRepoRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(true, nil)

	// Both repos exist and are already indexed (no pending jobs) — nothing to re-publish
	for _, pr := range providerRepos {
		repoID := uuid.New()
		repoURL, _ := valueobject.NewRepositoryURL(pr.URL)
		repoEntity := entity.RestoreRepository(repoID, repoURL, pr.Name, nil, nil, nil, nil, 0, 0, valueobject.RepositoryStatusCompleted, valueobject.ZoektIndexStatusPending, valueobject.EmbeddingIndexStatusCompleted, nil, 0, nil, time.Now(), time.Now(), nil)
		mockRepoRepo.On("FindByNormalizedURL", mock.Anything, mock.MatchedBy(func(u valueobject.RepositoryURL) bool {
			return u.String() == repoURL.String()
		})).Return(repoEntity, nil)
		mockJobRepo.On("FindByRepositoryID", mock.Anything, repoID, mock.AnythingOfType("outbound.IndexingJobFilters")).Return([]*entity.IndexingJob{}, 0, nil)
	}

	ctx := context.Background()
	resp, err := svc.SyncConnector(ctx, connectorID)

	require.NoError(t, err)
	assert.Equal(t, 2, resp.RepositoriesFound)

	mockConnRepo.AssertExpectations(t)
	mockGit.AssertExpectations(t)
	mockRepoRepo.AssertExpectations(t)
	mockJobRepo.AssertExpectations(t)
}

func TestSyncConnector_NoReposReturned(t *testing.T) {
	silentConnectorLogger(t)

	mockConnRepo := new(MockConnectorRepository)
	mockGit := new(MockGitProvider)
	svc := buildSyncService(mockConnRepo, mockGit, nil, nil, nil)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "gl-connector")

	mockConnRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockConnRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)
	mockGit.On("ListRepositories", mock.Anything, connector).Return([]outbound.GitProviderRepository{}, nil)

	ctx := context.Background()
	resp, err := svc.SyncConnector(ctx, connectorID)

	require.NoError(t, err)
	assert.Equal(t, 0, resp.RepositoriesFound)

	mockConnRepo.AssertExpectations(t)
	mockGit.AssertExpectations(t)
}
