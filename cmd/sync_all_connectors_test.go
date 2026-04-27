package cmd

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/config"
	"codechunking/internal/port/inbound"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mockConnectorSvc is a local test double for inbound.ConnectorService.
type mockConnectorSvc struct {
	mu sync.Mutex

	ListConnectorsFunc func(ctx context.Context, query dto.ConnectorListQuery) (*dto.ConnectorListResponse, error)
	SyncConnectorFunc  func(ctx context.Context, id uuid.UUID) (*dto.SyncConnectorResponse, error)

	listCalls []dto.ConnectorListQuery
	syncCalls []uuid.UUID
}

func (m *mockConnectorSvc) GetConnector(_ context.Context, _ uuid.UUID) (*dto.ConnectorResponse, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockConnectorSvc) ListConnectors(ctx context.Context, query dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
	m.mu.Lock()
	m.listCalls = append(m.listCalls, query)
	m.mu.Unlock()
	if m.ListConnectorsFunc != nil {
		return m.ListConnectorsFunc(ctx, query)
	}
	return &dto.ConnectorListResponse{}, nil
}

func (m *mockConnectorSvc) SyncConnector(ctx context.Context, id uuid.UUID) (*dto.SyncConnectorResponse, error) {
	m.mu.Lock()
	m.syncCalls = append(m.syncCalls, id)
	m.mu.Unlock()
	if m.SyncConnectorFunc != nil {
		return m.SyncConnectorFunc(ctx, id)
	}
	return &dto.SyncConnectorResponse{}, nil
}

func (m *mockConnectorSvc) listCallsCopy() []dto.ConnectorListQuery {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]dto.ConnectorListQuery, len(m.listCalls))
	copy(out, m.listCalls)
	return out
}

func (m *mockConnectorSvc) syncCallsCopy() []uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]uuid.UUID, len(m.syncCalls))
	copy(out, m.syncCalls)
	return out
}

var _ inbound.ConnectorService = (*mockConnectorSvc)(nil)

// awaitDone blocks until the done channel closes or 5 s elapses.
func awaitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SyncAllConnectors done channel did not close within 5 s")
	}
}

// newSF builds a minimal ServiceFactory suitable for unit tests (no real DB/NATS).
func newSF() *ServiceFactory {
	return NewServiceFactory(&config.Config{})
}

// makeConnectors returns a slice of ConnectorResponse stubs with distinct UUIDs.
func makeConnectors(n int) []dto.ConnectorResponse {
	out := make([]dto.ConnectorResponse, n)
	for i := range n {
		out[i] = dto.ConnectorResponse{ID: uuid.New()}
	}
	return out
}

// pagedResponse builds a ConnectorListResponse for a given page of connectors.
func pagedResponse(connectors []dto.ConnectorResponse, hasMore bool) *dto.ConnectorListResponse {
	return &dto.ConnectorListResponse{
		Connectors: connectors,
		Pagination: dto.PaginationResponse{HasMore: hasMore},
	}
}

func TestSyncAllConnectors_PaginatesAcrossThreePages(t *testing.T) {
	t.Parallel()

	page0 := makeConnectors(100) // offset 0, HasMore true
	page1 := makeConnectors(100) // offset 100, HasMore true
	page2 := makeConnectors(50)  // offset 200, HasMore false

	mock := &mockConnectorSvc{
		ListConnectorsFunc: func(_ context.Context, q dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
			switch q.Offset {
			case 0:
				return pagedResponse(page0, true), nil
			case 100:
				return pagedResponse(page1, true), nil
			case 200:
				return pagedResponse(page2, false), nil
			default:
				return pagedResponse(nil, false), nil
			}
		},
		SyncConnectorFunc: func(_ context.Context, _ uuid.UUID) (*dto.SyncConnectorResponse, error) {
			return &dto.SyncConnectorResponse{}, nil
		},
	}

	done := newSF().SyncAllConnectors(context.Background(), mock)
	awaitDone(t, done)

	listCalls := mock.listCallsCopy()
	if len(listCalls) != 3 {
		t.Fatalf("expected 3 ListConnectors calls, got %d", len(listCalls))
	}
	if listCalls[0].Offset != 0 || listCalls[0].Limit != dto.MaxLimitValue {
		t.Errorf("page 0 query mismatch: %+v", listCalls[0])
	}
	if listCalls[1].Offset != 100 || listCalls[1].Limit != dto.MaxLimitValue {
		t.Errorf("page 1 query mismatch: %+v", listCalls[1])
	}
	if listCalls[2].Offset != 200 || listCalls[2].Limit != dto.MaxLimitValue {
		t.Errorf("page 2 query mismatch: %+v", listCalls[2])
	}

	syncCalls := mock.syncCallsCopy()
	if len(syncCalls) != 250 {
		t.Errorf("expected 250 SyncConnector calls, got %d", len(syncCalls))
	}
}

func TestSyncAllConnectors_EmptyFirstPageIsCleanExit(t *testing.T) {
	t.Parallel()

	mock := &mockConnectorSvc{
		ListConnectorsFunc: func(_ context.Context, _ dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
			return pagedResponse(nil, false), nil
		},
	}

	done := newSF().SyncAllConnectors(context.Background(), mock)
	awaitDone(t, done)

	if calls := mock.syncCallsCopy(); len(calls) != 0 {
		t.Errorf("expected 0 SyncConnector calls, got %d", len(calls))
	}
}

func TestSyncAllConnectors_RespectsShutdownContextMidIteration(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	var listCallCount int32
	mock := &mockConnectorSvc{
		ListConnectorsFunc: func(_ context.Context, q dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
			n := atomic.AddInt32(&listCallCount, 1)
			if n == 1 {
				// First page returns data + HasMore; cancel ctx before second page.
				cancel()
				return pagedResponse(makeConnectors(10), true), nil
			}
			return pagedResponse(makeConnectors(10), false), nil
		},
		SyncConnectorFunc: func(_ context.Context, _ uuid.UUID) (*dto.SyncConnectorResponse, error) {
			return &dto.SyncConnectorResponse{}, nil
		},
	}

	done := newSF().SyncAllConnectors(ctx, mock)
	awaitDone(t, done)

	// After ctx cancel, no further ListConnectors calls must be issued.
	if n := atomic.LoadInt32(&listCallCount); n > 1 {
		t.Errorf("expected at most 1 ListConnectors call after cancel, got %d", n)
	}
}

func TestSyncAllConnectors_BoundedConcurrency(t *testing.T) {
	t.Parallel()

	const connectorCount = 20

	// allBlocked is closed once syncConnectorConcurrency workers are simultaneously
	// blocked inside SyncConnector, proving the semaphore is saturated.
	allBlocked := make(chan struct{})
	var blockedOnce sync.Once
	release := make(chan struct{})

	var inflight int32
	var peakInflight int32

	mock := &mockConnectorSvc{
		ListConnectorsFunc: func(_ context.Context, _ dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
			return pagedResponse(makeConnectors(connectorCount), false), nil
		},
		SyncConnectorFunc: func(_ context.Context, _ uuid.UUID) (*dto.SyncConnectorResponse, error) {
			cur := atomic.AddInt32(&inflight, 1)
			for {
				peak := atomic.LoadInt32(&peakInflight)
				if cur <= peak || atomic.CompareAndSwapInt32(&peakInflight, peak, cur) {
					break
				}
			}
			// Signal once the semaphore is fully saturated so the test can
			// unblock workers without relying on a fixed sleep duration.
			if atomic.LoadInt32(&inflight) >= int32(syncConnectorConcurrency) {
				blockedOnce.Do(func() { close(allBlocked) })
			}
			<-release
			atomic.AddInt32(&inflight, -1)
			return &dto.SyncConnectorResponse{}, nil
		},
	}

	done := newSF().SyncAllConnectors(context.Background(), mock)

	// Wait until the semaphore is fully saturated, then release all workers.
	select {
	case <-allBlocked:
	case <-time.After(5 * time.Second):
		t.Fatal("workers did not saturate the semaphore within 5 s")
	}
	close(release)

	awaitDone(t, done)

	if peak := atomic.LoadInt32(&peakInflight); peak > syncConnectorConcurrency {
		t.Errorf("peak concurrency %d exceeded syncConnectorConcurrency=%d", peak, syncConnectorConcurrency)
	}
}

func TestSyncAllConnectors_PerConnectorErrorLoggedSiblingsComplete(t *testing.T) {
	t.Parallel()

	ids := makeConnectors(5)
	failID := ids[2].ID

	mock := &mockConnectorSvc{
		ListConnectorsFunc: func(_ context.Context, _ dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
			return pagedResponse(ids, false), nil
		},
		SyncConnectorFunc: func(_ context.Context, id uuid.UUID) (*dto.SyncConnectorResponse, error) {
			if id == failID {
				return nil, errors.New("simulated sync failure")
			}
			return &dto.SyncConnectorResponse{}, nil
		},
	}

	done := newSF().SyncAllConnectors(context.Background(), mock)
	awaitDone(t, done)

	if calls := mock.syncCallsCopy(); len(calls) != 5 {
		t.Errorf("expected 5 SyncConnector calls (including the failing one), got %d", len(calls))
	}
}

func TestSyncAllConnectors_ListErrorOnFirstPageAbortsCleanly(t *testing.T) {
	t.Parallel()

	mock := &mockConnectorSvc{
		ListConnectorsFunc: func(_ context.Context, _ dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
			return nil, errors.New("db unavailable")
		},
	}

	done := newSF().SyncAllConnectors(context.Background(), mock)
	awaitDone(t, done)

	if calls := mock.syncCallsCopy(); len(calls) != 0 {
		t.Errorf("expected 0 SyncConnector calls after list error, got %d", len(calls))
	}
}

func TestSyncAllConnectors_ListErrorOnLaterPageAbortsCleanly(t *testing.T) {
	t.Parallel()

	mock := &mockConnectorSvc{
		ListConnectorsFunc: func(_ context.Context, q dto.ConnectorListQuery) (*dto.ConnectorListResponse, error) {
			if q.Offset == 0 {
				return pagedResponse(makeConnectors(dto.MaxLimitValue), true), nil
			}
			return nil, errors.New("db intermittent error")
		},
		SyncConnectorFunc: func(_ context.Context, _ uuid.UUID) (*dto.SyncConnectorResponse, error) {
			return &dto.SyncConnectorResponse{}, nil
		},
	}

	done := newSF().SyncAllConnectors(context.Background(), mock)
	awaitDone(t, done)

	// Page 0 connectors dispatched; page 1 never fetched due to error.
	listCalls := mock.listCallsCopy()
	if len(listCalls) != 2 {
		t.Errorf("expected exactly 2 ListConnectors calls, got %d", len(listCalls))
	}
	// Syncs for page 0 complete; none from the errored page.
	syncCalls := mock.syncCallsCopy()
	if len(syncCalls) != dto.MaxLimitValue {
		t.Errorf("expected %d SyncConnector calls (page 0 only), got %d", dto.MaxLimitValue, len(syncCalls))
	}
}
