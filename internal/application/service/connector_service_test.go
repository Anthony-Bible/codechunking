package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	domainerrors "codechunking/internal/domain/errors/domain"
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
// It saves the previous logger and restores it on test cleanup to avoid
// interfering with other tests that rely on a non-nil global logger.
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
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
		valueobject.ConnectorStatusActive,
		5,
		nil,
		now,
		now,
	)
}

// =============================================================================
// CreateConnector tests
// =============================================================================

func TestCreateConnectorService_CreateConnector_Success(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewCreateConnectorService(mockRepo)

	req := dto.CreateConnectorRequest{
		Name:          "my-org-connector",
		ConnectorType: "github_org",
		BaseURL:       "https://api.github.com",
	}

	mockRepo.On("Exists", mock.Anything, "my-org-connector").Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)

	ctx := context.Background()
	response, err := service.CreateConnector(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "my-org-connector", response.Name)
	assert.Equal(t, "github_org", response.ConnectorType)
	assert.Equal(t, "pending", response.Status)
	assert.NotEqual(t, uuid.Nil, response.ID)

	mockRepo.AssertExpectations(t)
}

func TestCreateConnectorService_CreateConnector_DuplicateName(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewCreateConnectorService(mockRepo)

	req := dto.CreateConnectorRequest{
		Name:          "existing-connector",
		ConnectorType: "github_org",
		BaseURL:       "https://api.github.com",
	}

	mockRepo.On("Exists", mock.Anything, "existing-connector").Return(true, nil)

	ctx := context.Background()
	response, err := service.CreateConnector(ctx, req)

	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, domainerrors.ErrConnectorAlreadyExists)

	mockRepo.AssertExpectations(t)
}

func TestCreateConnectorService_CreateConnector_RepositoryError(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewCreateConnectorService(mockRepo)

	req := dto.CreateConnectorRequest{
		Name:          "new-connector",
		ConnectorType: "gitlab_group",
		BaseURL:       "https://gitlab.com",
	}

	mockRepo.On("Exists", mock.Anything, "new-connector").Return(false, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Connector")).
		Return(errInternalTest)

	ctx := context.Background()
	_, err := service.CreateConnector(ctx, req)

	require.Error(t, err)
	mockRepo.AssertExpectations(t)
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
// DeleteConnector tests
// =============================================================================

func TestDeleteConnectorService_DeleteConnector_Success(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewDeleteConnectorService(mockRepo)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "to-delete")

	mockRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockRepo.On("Delete", mock.Anything, connectorID).Return(nil)

	ctx := context.Background()
	err := service.DeleteConnector(ctx, connectorID)

	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteConnectorService_DeleteConnector_NotFound(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewDeleteConnectorService(mockRepo)

	connectorID := uuid.New()
	mockRepo.On("FindByID", mock.Anything, connectorID).Return(nil, domainerrors.ErrConnectorNotFound)

	ctx := context.Background()
	err := service.DeleteConnector(ctx, connectorID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrConnectorNotFound)

	mockRepo.AssertExpectations(t)
}

func TestDeleteConnectorService_DeleteConnector_WhileSyncing(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewDeleteConnectorService(mockRepo)

	connectorID := uuid.New()
	now := time.Now()
	syncingConnector := entity.RestoreConnector(
		connectorID,
		"syncing-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
		valueobject.ConnectorStatusSyncing,
		0,
		nil,
		now,
		now,
	)

	mockRepo.On("FindByID", mock.Anything, connectorID).Return(syncingConnector, nil)

	ctx := context.Background()
	err := service.DeleteConnector(ctx, connectorID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainerrors.ErrConnectorSyncing)

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// SyncConnector tests
// =============================================================================

func TestSyncConnectorService_SyncConnector_Success(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewSyncConnectorService(mockRepo)

	connectorID := uuid.New()
	connector := buildTestConnector(t, connectorID, "my-connector")

	mockRepo.On("FindByID", mock.Anything, connectorID).Return(connector, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Connector")).Return(nil)

	ctx := context.Background()
	response, err := service.SyncConnector(ctx, connectorID)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, connectorID, response.ConnectorID)
	assert.NotEmpty(t, response.Message)

	mockRepo.AssertExpectations(t)
}

func TestSyncConnectorService_SyncConnector_NotFound(t *testing.T) {
	silentConnectorLogger(t)

	mockRepo := new(MockConnectorRepository)
	service := NewSyncConnectorService(mockRepo)

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
	service := NewSyncConnectorService(mockRepo)

	connectorID := uuid.New()
	now := time.Now()
	syncingConnector := entity.RestoreConnector(
		connectorID,
		"syncing-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
		valueobject.ConnectorStatusSyncing,
		0,
		nil,
		now,
		now,
	)

	mockRepo.On("FindByID", mock.Anything, connectorID).Return(syncingConnector, nil)

	ctx := context.Background()
	response, err := service.SyncConnector(ctx, connectorID)

	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, domainerrors.ErrConnectorSyncing)

	mockRepo.AssertExpectations(t)
}
