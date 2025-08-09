package testutil

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
	"codechunking/internal/port/inbound"

	"github.com/google/uuid"
)

// Compile-time interface compliance checks
var (
	_ inbound.RepositoryService = (*MockRepositoryService)(nil)
	_ inbound.HealthService     = (*MockHealthService)(nil)
)

// MockRepositoryService implements inbound.RepositoryService for testing
type MockRepositoryService struct {
	mu sync.RWMutex

	// Method call expectations
	CreateRepositoryFunc  func(ctx context.Context, request dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error)
	GetRepositoryFunc     func(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error)
	ListRepositoriesFunc  func(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error)
	DeleteRepositoryFunc  func(ctx context.Context, id uuid.UUID) error
	GetRepositoryJobsFunc func(ctx context.Context, repositoryID uuid.UUID, query dto.IndexingJobListQuery) (*dto.IndexingJobListResponse, error)
	GetIndexingJobFunc    func(ctx context.Context, repositoryID, jobID uuid.UUID) (*dto.IndexingJobResponse, error)

	// Call tracking
	CreateRepositoryCalls  []CreateRepositoryCall
	GetRepositoryCalls     []GetRepositoryCall
	ListRepositoriesCalls  []ListRepositoriesCall
	DeleteRepositoryCalls  []DeleteRepositoryCall
	GetRepositoryJobsCalls []GetRepositoryJobsCall
	GetIndexingJobCalls    []GetIndexingJobCall
}

// Call structs for tracking method invocations
type CreateRepositoryCall struct {
	Ctx     context.Context
	Request dto.CreateRepositoryRequest
}

type GetRepositoryCall struct {
	Ctx context.Context
	ID  uuid.UUID
}

type ListRepositoriesCall struct {
	Ctx   context.Context
	Query dto.RepositoryListQuery
}

type DeleteRepositoryCall struct {
	Ctx context.Context
	ID  uuid.UUID
}

type GetRepositoryJobsCall struct {
	Ctx          context.Context
	RepositoryID uuid.UUID
	Query        dto.IndexingJobListQuery
}

type GetIndexingJobCall struct {
	Ctx          context.Context
	RepositoryID uuid.UUID
	JobID        uuid.UUID
}

// NewMockRepositoryService creates a new mock repository service
func NewMockRepositoryService() *MockRepositoryService {
	return &MockRepositoryService{}
}

func (m *MockRepositoryService) CreateRepository(ctx context.Context, request dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateRepositoryCalls = append(m.CreateRepositoryCalls, CreateRepositoryCall{
		Ctx:     ctx,
		Request: request,
	})

	if m.CreateRepositoryFunc != nil {
		return m.CreateRepositoryFunc(ctx, request)
	}

	return nil, errors.New("mock not configured")
}

func (m *MockRepositoryService) GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetRepositoryCalls = append(m.GetRepositoryCalls, GetRepositoryCall{
		Ctx: ctx,
		ID:  id,
	})

	if m.GetRepositoryFunc != nil {
		return m.GetRepositoryFunc(ctx, id)
	}

	return nil, errors.New("mock not configured")
}

func (m *MockRepositoryService) ListRepositories(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ListRepositoriesCalls = append(m.ListRepositoriesCalls, ListRepositoriesCall{
		Ctx:   ctx,
		Query: query,
	})

	if m.ListRepositoriesFunc != nil {
		return m.ListRepositoriesFunc(ctx, query)
	}

	return nil, errors.New("mock not configured")
}

func (m *MockRepositoryService) DeleteRepository(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeleteRepositoryCalls = append(m.DeleteRepositoryCalls, DeleteRepositoryCall{
		Ctx: ctx,
		ID:  id,
	})

	if m.DeleteRepositoryFunc != nil {
		return m.DeleteRepositoryFunc(ctx, id)
	}

	return errors.New("mock not configured")
}

func (m *MockRepositoryService) GetRepositoryJobs(ctx context.Context, repositoryID uuid.UUID, query dto.IndexingJobListQuery) (*dto.IndexingJobListResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetRepositoryJobsCalls = append(m.GetRepositoryJobsCalls, GetRepositoryJobsCall{
		Ctx:          ctx,
		RepositoryID: repositoryID,
		Query:        query,
	})

	if m.GetRepositoryJobsFunc != nil {
		return m.GetRepositoryJobsFunc(ctx, repositoryID, query)
	}

	return nil, errors.New("mock not configured")
}

func (m *MockRepositoryService) GetIndexingJob(ctx context.Context, repositoryID, jobID uuid.UUID) (*dto.IndexingJobResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetIndexingJobCalls = append(m.GetIndexingJobCalls, GetIndexingJobCall{
		Ctx:          ctx,
		RepositoryID: repositoryID,
		JobID:        jobID,
	})

	if m.GetIndexingJobFunc != nil {
		return m.GetIndexingJobFunc(ctx, repositoryID, jobID)
	}

	return nil, errors.New("mock not configured")
}

// Helper methods for setting up mock expectations
func (m *MockRepositoryService) ExpectCreateRepository(response *dto.RepositoryResponse, err error) {
	m.CreateRepositoryFunc = func(ctx context.Context, request dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
		return response, err
	}
}

func (m *MockRepositoryService) ExpectGetRepository(response *dto.RepositoryResponse, err error) {
	m.GetRepositoryFunc = func(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error) {
		return response, err
	}
}

func (m *MockRepositoryService) ExpectListRepositories(response *dto.RepositoryListResponse, err error) {
	m.ListRepositoriesFunc = func(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error) {
		return response, err
	}
}

func (m *MockRepositoryService) ExpectDeleteRepository(err error) {
	m.DeleteRepositoryFunc = func(ctx context.Context, id uuid.UUID) error {
		return err
	}
}

func (m *MockRepositoryService) ExpectGetRepositoryJobs(response *dto.IndexingJobListResponse, err error) {
	m.GetRepositoryJobsFunc = func(ctx context.Context, repositoryID uuid.UUID, query dto.IndexingJobListQuery) (*dto.IndexingJobListResponse, error) {
		return response, err
	}
}

func (m *MockRepositoryService) ExpectGetIndexingJob(response *dto.IndexingJobResponse, err error) {
	m.GetIndexingJobFunc = func(ctx context.Context, repositoryID, jobID uuid.UUID) (*dto.IndexingJobResponse, error) {
		return response, err
	}
}

// Reset clears all call tracking
func (m *MockRepositoryService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateRepositoryCalls = nil
	m.GetRepositoryCalls = nil
	m.ListRepositoriesCalls = nil
	m.DeleteRepositoryCalls = nil
	m.GetRepositoryJobsCalls = nil
	m.GetIndexingJobCalls = nil
}

// MockHealthService implements inbound.HealthService for testing
type MockHealthService struct {
	mu sync.RWMutex

	GetHealthFunc  func(ctx context.Context) (*dto.HealthResponse, error)
	GetHealthCalls []GetHealthCall
}

type GetHealthCall struct {
	Ctx context.Context
}

func NewMockHealthService() *MockHealthService {
	return &MockHealthService{}
}

func (m *MockHealthService) GetHealth(ctx context.Context) (*dto.HealthResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetHealthCalls = append(m.GetHealthCalls, GetHealthCall{Ctx: ctx})

	if m.GetHealthFunc != nil {
		return m.GetHealthFunc(ctx)
	}

	return nil, errors.New("mock not configured")
}

func (m *MockHealthService) ExpectGetHealth(response *dto.HealthResponse, err error) {
	m.GetHealthFunc = func(ctx context.Context) (*dto.HealthResponse, error) {
		return response, err
	}
}

func (m *MockHealthService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetHealthCalls = nil
}

// MockErrorHandler implements ErrorHandler for testing
type MockErrorHandler struct {
	mu sync.RWMutex

	HandleValidationErrorFunc func(w http.ResponseWriter, r *http.Request, err error)
	HandleServiceErrorFunc    func(w http.ResponseWriter, r *http.Request, err error)

	HandleValidationErrorCalls []HandleValidationErrorCall
	HandleServiceErrorCalls    []HandleServiceErrorCall
}

type HandleValidationErrorCall struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Error          error
}

type HandleServiceErrorCall struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Error          error
}

func NewMockErrorHandler() *MockErrorHandler {
	return &MockErrorHandler{}
}

func (m *MockErrorHandler) HandleValidationError(w http.ResponseWriter, r *http.Request, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.HandleValidationErrorCalls = append(m.HandleValidationErrorCalls, HandleValidationErrorCall{
		ResponseWriter: w,
		Request:        r,
		Error:          err,
	})

	if m.HandleValidationErrorFunc != nil {
		m.HandleValidationErrorFunc(w, r, err)
		return
	}

	// Default mock behavior - return generic validation error
	http.Error(w, "validation error", http.StatusBadRequest)
}

func (m *MockErrorHandler) HandleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.HandleServiceErrorCalls = append(m.HandleServiceErrorCalls, HandleServiceErrorCall{
		ResponseWriter: w,
		Request:        r,
		Error:          err,
	})

	if m.HandleServiceErrorFunc != nil {
		m.HandleServiceErrorFunc(w, r, err)
		return
	}

	// Default mock behavior - map domain errors to proper HTTP status codes
	switch {
	case errors.Is(err, domain.ErrRepositoryNotFound):
		http.Error(w, "service error", http.StatusNotFound)
	case errors.Is(err, domain.ErrRepositoryAlreadyExists):
		http.Error(w, "service error", http.StatusConflict)
	case errors.Is(err, domain.ErrRepositoryProcessing):
		http.Error(w, "service error", http.StatusConflict)
	case errors.Is(err, domain.ErrJobNotFound):
		http.Error(w, "service error", http.StatusNotFound)
	default:
		http.Error(w, "service error", http.StatusInternalServerError)
	}
}

func (m *MockErrorHandler) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.HandleValidationErrorCalls = nil
	m.HandleServiceErrorCalls = nil
}
