package api_test

import (
	"codechunking/internal/adapter/inbound/api"
	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"
	domainerrors "codechunking/internal/domain/errors/domain"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockConnectorService is a local mock for inbound.ConnectorService (read-only).
type MockConnectorService struct {
	mu sync.RWMutex

	GetConnectorFunc   func(ctx context.Context, id uuid.UUID) (*dto.ConnectorResponse, error)
	ListConnectorsFunc func(ctx context.Context, query dto.ConnectorListQuery) (*dto.ConnectorListResponse, error)
	SyncConnectorFunc  func(ctx context.Context, id uuid.UUID) (*dto.SyncConnectorResponse, error)

	GetConnectorCalls   []getConnectorCall
	ListConnectorsCalls []listConnectorsCall
	SyncConnectorCalls  []syncConnectorCall
}

type getConnectorCall struct {
	Ctx context.Context
	ID  uuid.UUID
}

type listConnectorsCall struct {
	Ctx   context.Context
	Query dto.ConnectorListQuery
}

type syncConnectorCall struct {
	Ctx context.Context
	ID  uuid.UUID
}

func newMockConnectorService() *MockConnectorService {
	return &MockConnectorService{}
}

func (m *MockConnectorService) GetConnector(ctx context.Context, id uuid.UUID) (*dto.ConnectorResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetConnectorCalls = append(m.GetConnectorCalls, getConnectorCall{Ctx: ctx, ID: id})
	if m.GetConnectorFunc != nil {
		return m.GetConnectorFunc(ctx, id)
	}
	return nil, errors.New("mock not configured")
}

func (m *MockConnectorService) ListConnectors(
	ctx context.Context,
	query dto.ConnectorListQuery,
) (*dto.ConnectorListResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListConnectorsCalls = append(m.ListConnectorsCalls, listConnectorsCall{Ctx: ctx, Query: query})
	if m.ListConnectorsFunc != nil {
		return m.ListConnectorsFunc(ctx, query)
	}
	return nil, errors.New("mock not configured")
}

func (m *MockConnectorService) SyncConnector(
	ctx context.Context,
	id uuid.UUID,
) (*dto.SyncConnectorResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SyncConnectorCalls = append(m.SyncConnectorCalls, syncConnectorCall{Ctx: ctx, ID: id})
	if m.SyncConnectorFunc != nil {
		return m.SyncConnectorFunc(ctx, id)
	}
	return nil, errors.New("mock not configured")
}

func newTestConnectorID() uuid.UUID {
	return uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
}

func newConnectorResponse(id uuid.UUID, name, status string) dto.ConnectorResponse {
	now := time.Now()
	return dto.ConnectorResponse{
		ID:              id,
		Name:            name,
		ConnectorType:   "gitlab",
		BaseURL:         "https://gitlab.com",
		Status:          status,
		RepositoryCount: 0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// =============================================================================
// GET /connectors
// =============================================================================

func TestConnectorHandler_ListConnectors(t *testing.T) {
	tests := []struct {
		name           string
		mockSetup      func(*MockConnectorService)
		expectedStatus int
		validateFunc   func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "successful_list_returns_200",
			mockSetup: func(m *MockConnectorService) {
				resp := &dto.ConnectorListResponse{
					Connectors: []dto.ConnectorResponse{
						newConnectorResponse(uuid.New(), "connector-1", "active"),
						newConnectorResponse(uuid.New(), "connector-2", "active"),
					},
					Pagination: dto.PaginationResponse{Limit: 20, Offset: 0, Total: 2, HasMore: false},
				}
				m.ListConnectorsFunc = func(_ context.Context, _ dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
					return resp, nil
				}
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

				var response dto.ConnectorListResponse
				err := testutil.ParseJSONResponse(rec, &response)
				require.NoError(t, err)

				assert.Len(t, response.Connectors, 2)
				assert.Equal(t, 2, response.Pagination.Total)
			},
		},
		{
			name: "empty_list_returns_200",
			mockSetup: func(m *MockConnectorService) {
				resp := &dto.ConnectorListResponse{
					Connectors: []dto.ConnectorResponse{},
					Pagination: dto.PaginationResponse{Limit: 20, Offset: 0, Total: 0, HasMore: false},
				}
				m.ListConnectorsFunc = func(_ context.Context, _ dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
					return resp, nil
				}
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response dto.ConnectorListResponse
				err := testutil.ParseJSONResponse(rec, &response)
				require.NoError(t, err)
				assert.Empty(t, response.Connectors)
			},
		},
		{
			name: "service_error_returns_500",
			mockSetup: func(m *MockConnectorService) {
				m.ListConnectorsFunc = func(_ context.Context, _ dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
					return nil, errors.New("database error")
				}
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockConnectorService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := api.NewConnectorHandler(mockService, mockErrorHandler)

			req := testutil.CreateRequest(http.MethodGet, "/connectors")
			recorder := httptest.NewRecorder()

			handler.ListConnectors(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

// =============================================================================
// GET /connectors/{id}
// =============================================================================

func TestConnectorHandler_GetConnector(t *testing.T) {
	tests := []struct {
		name           string
		connectorID    string
		mockSetup      func(*MockConnectorService)
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:        "successful_get_returns_200",
			connectorID: newTestConnectorID().String(),
			mockSetup: func(m *MockConnectorService) {
				resp := newConnectorResponse(newTestConnectorID(), "my-connector", "active")
				m.GetConnectorFunc = func(_ context.Context, _ uuid.UUID) (*dto.ConnectorResponse, error) {
					return &resp, nil
				}
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

				var response dto.ConnectorResponse
				err := testutil.ParseJSONResponse(rec, &response)
				require.NoError(t, err)

				assert.Equal(t, newTestConnectorID(), response.ID)
				assert.Equal(t, "my-connector", response.Name)
				assert.Equal(t, "active", response.Status)
			},
		},
		{
			name:           "missing_id_returns_400",
			connectorID:    "",
			mockSetup:      func(_ *MockConnectorService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_uuid_returns_400",
			connectorID:    "not-a-uuid",
			mockSetup:      func(_ *MockConnectorService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:        "not_found_returns_404",
			connectorID: newTestConnectorID().String(),
			mockSetup: func(m *MockConnectorService) {
				m.GetConnectorFunc = func(_ context.Context, _ uuid.UUID) (*dto.ConnectorResponse, error) {
					return nil, domainerrors.ErrConnectorNotFound
				}
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "service error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockConnectorService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := api.NewConnectorHandler(mockService, mockErrorHandler)

			pathParams := map[string]string{}
			if tt.connectorID != "" {
				pathParams["id"] = tt.connectorID
			}
			req := testutil.CreateRequestWithPathParams(http.MethodGet, "/connectors/"+tt.connectorID, pathParams)
			recorder := httptest.NewRecorder()

			handler.GetConnector(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

// =============================================================================
// POST /connectors/{id}/sync
// =============================================================================

func TestConnectorHandler_SyncConnector(t *testing.T) {
	tests := []struct {
		name           string
		connectorID    string
		mockSetup      func(*MockConnectorService)
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:        "successful_sync_returns_202",
			connectorID: newTestConnectorID().String(),
			mockSetup: func(m *MockConnectorService) {
				resp := &dto.SyncConnectorResponse{
					ConnectorID:       newTestConnectorID(),
					RepositoriesFound: 25,
					Message:           "sync triggered successfully",
				}
				m.SyncConnectorFunc = func(_ context.Context, _ uuid.UUID) (*dto.SyncConnectorResponse, error) {
					return resp, nil
				}
			},
			expectedStatus: http.StatusAccepted,
			validateFunc: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

				var response dto.SyncConnectorResponse
				err := testutil.ParseJSONResponse(rec, &response)
				require.NoError(t, err)

				assert.Equal(t, newTestConnectorID(), response.ConnectorID)
				assert.Equal(t, 25, response.RepositoriesFound)
				assert.NotEmpty(t, response.Message)
			},
		},
		{
			name:           "missing_id_returns_400",
			connectorID:    "",
			mockSetup:      func(_ *MockConnectorService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_uuid_returns_400",
			connectorID:    "bad-uuid",
			mockSetup:      func(_ *MockConnectorService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:        "not_found_returns_404",
			connectorID: newTestConnectorID().String(),
			mockSetup: func(m *MockConnectorService) {
				m.SyncConnectorFunc = func(_ context.Context, _ uuid.UUID) (*dto.SyncConnectorResponse, error) {
					return nil, domainerrors.ErrConnectorNotFound
				}
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "service error",
		},
		{
			name:        "already_syncing_returns_409",
			connectorID: newTestConnectorID().String(),
			mockSetup: func(m *MockConnectorService) {
				m.SyncConnectorFunc = func(_ context.Context, _ uuid.UUID) (*dto.SyncConnectorResponse, error) {
					return nil, domainerrors.ErrConnectorSyncing
				}
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "service error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockConnectorService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := api.NewConnectorHandler(mockService, mockErrorHandler)

			pathParams := map[string]string{}
			if tt.connectorID != "" {
				pathParams["id"] = tt.connectorID
			}
			req := testutil.CreateRequestWithPathParams(
				http.MethodPost,
				"/connectors/"+tt.connectorID+"/sync",
				pathParams,
			)
			recorder := httptest.NewRecorder()

			handler.SyncConnector(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}

			if tt.expectedStatus == http.StatusAccepted {
				assert.Len(t, mockService.SyncConnectorCalls, 1)
			}
		})
	}
}
