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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMessagePublisherWithHealth combines MessagePublisher and MessagePublisherHealth
type MockMessagePublisherWithHealth struct {
	// MessagePublisher methods
	publishFunc func(ctx context.Context, repositoryID, repositoryURL string) error

	// Health monitoring methods
	healthStatus outbound.MessagePublisherHealthStatus
	metrics      outbound.MessagePublisherMetrics

	// Test control
	simulateError bool
}

func NewMockMessagePublisherWithHealth() *MockMessagePublisherWithHealth {
	return &MockMessagePublisherWithHealth{
		healthStatus: outbound.MessagePublisherHealthStatus{
			Connected:        true,
			Uptime:           "1h30m",
			Reconnects:       0,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		},
		metrics: outbound.MessagePublisherMetrics{
			PublishedCount: 1234,
			FailedCount:    0,
			AverageLatency: "2ms",
		},
	}
}

func (m *MockMessagePublisherWithHealth) PublishIndexingJob(ctx context.Context, repositoryID uuid.UUID, repositoryURL string) error {
	if m.publishFunc != nil {
		return m.publishFunc(ctx, repositoryID.String(), repositoryURL)
	}
	if m.simulateError {
		return errors.New("message publishing failed")
	}
	return nil
}

func (m *MockMessagePublisherWithHealth) GetConnectionHealth() outbound.MessagePublisherHealthStatus {
	return m.healthStatus
}

func (m *MockMessagePublisherWithHealth) GetMessageMetrics() outbound.MessagePublisherMetrics {
	return m.metrics
}

func (m *MockMessagePublisherWithHealth) SetHealthStatus(status outbound.MessagePublisherHealthStatus) {
	m.healthStatus = status
}

func (m *MockMessagePublisherWithHealth) SetMetrics(metrics outbound.MessagePublisherMetrics) {
	m.metrics = metrics
}

func TestHealthServiceAdapter_GetHealth_WithNATSHealthIntegration(t *testing.T) {
	tests := []struct {
		name             string
		setupMocks       func(*MockMessagePublisherWithHealth, *mockRepositoryRepo, *mockIndexingJobRepo)
		expectedStatus   string
		validateResponse func(t *testing.T, response *dto.HealthResponse)
	}{
		{
			name: "healthy_nats_connection_includes_detailed_metrics",
			setupMocks: func(nats *MockMessagePublisherWithHealth, repo *mockRepositoryRepo, jobs *mockIndexingJobRepo) {
				// Database is healthy
				repo.findAllResult = nil
				// NATS is healthy with good metrics
				nats.SetHealthStatus(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					Uptime:           "2h15m30s",
					Reconnects:       3,
					JetStreamEnabled: true,
					CircuitBreaker:   "closed",
				})
				nats.SetMetrics(outbound.MessagePublisherMetrics{
					PublishedCount: 5678,
					FailedCount:    12,
					AverageLatency: "3.5ms",
				})
			},
			expectedStatus: "healthy",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				require.NotNil(t, response.Dependencies)

				natsStatus, exists := response.Dependencies["nats"]
				require.True(t, exists, "NATS dependency should exist in health response")
				assert.Equal(t, "healthy", natsStatus.Status)

				// Verify detailed NATS health information
				require.NotNil(t, natsStatus.Details)
				natsDetails, ok := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				require.True(t, ok, "NATS details should be of type NATSHealthDetails")

				assert.True(t, natsDetails.Connected)
				assert.Equal(t, "2h15m30s", natsDetails.Uptime)
				assert.Equal(t, 3, natsDetails.Reconnects)
				assert.Equal(t, "", natsDetails.LastError)
				assert.True(t, natsDetails.JetStreamEnabled)
				assert.Equal(t, "closed", natsDetails.CircuitBreaker)

				// Verify message metrics
				assert.Equal(t, int64(5678), natsDetails.MessageMetrics.PublishedCount)
				assert.Equal(t, int64(12), natsDetails.MessageMetrics.FailedCount)
				assert.Equal(t, "3.5ms", natsDetails.MessageMetrics.AverageLatency)
			},
		},
		{
			name: "disconnected_nats_returns_degraded_status",
			setupMocks: func(nats *MockMessagePublisherWithHealth, repo *mockRepositoryRepo, jobs *mockIndexingJobRepo) {
				// Database is healthy
				repo.findAllResult = nil
				// NATS is disconnected
				nats.SetHealthStatus(outbound.MessagePublisherHealthStatus{
					Connected:        false,
					LastError:        "connection lost",
					Uptime:           "0s",
					Reconnects:       5,
					JetStreamEnabled: false,
					CircuitBreaker:   "open",
				})
				nats.SetMetrics(outbound.MessagePublisherMetrics{
					PublishedCount: 1000,
					FailedCount:    50,
					AverageLatency: "0ms",
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.False(t, natsDetails.Connected)
				assert.Equal(t, "connection lost", natsDetails.LastError)
				assert.Equal(t, "open", natsDetails.CircuitBreaker)
				assert.False(t, natsDetails.JetStreamEnabled)
			},
		},
		{
			name: "nats_circuit_breaker_open_shows_degraded_health",
			setupMocks: func(nats *MockMessagePublisherWithHealth, repo *mockRepositoryRepo, jobs *mockIndexingJobRepo) {
				repo.findAllResult = nil
				nats.SetHealthStatus(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					LastError:        "too many failures",
					Uptime:           "45m",
					Reconnects:       1,
					JetStreamEnabled: true,
					CircuitBreaker:   "open",
				})
				nats.SetMetrics(outbound.MessagePublisherMetrics{
					PublishedCount: 800,
					FailedCount:    200,    // High failure rate
					AverageLatency: "15ms", // Higher latency
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "circuit breaker open")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.Equal(t, "open", natsDetails.CircuitBreaker)
				assert.Equal(t, int64(200), natsDetails.MessageMetrics.FailedCount)
			},
		},
		{
			name: "high_reconnection_count_indicates_unstable_connection",
			setupMocks: func(nats *MockMessagePublisherWithHealth, repo *mockRepositoryRepo, jobs *mockIndexingJobRepo) {
				repo.findAllResult = nil
				nats.SetHealthStatus(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					Uptime:           "10m",
					Reconnects:       15, // Very high reconnection count
					JetStreamEnabled: true,
					CircuitBreaker:   "closed",
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "unstable connection")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.Equal(t, 15, natsDetails.Reconnects)
			},
		},
		{
			name: "jetstream_disabled_affects_health_status",
			setupMocks: func(nats *MockMessagePublisherWithHealth, repo *mockRepositoryRepo, jobs *mockIndexingJobRepo) {
				repo.findAllResult = nil
				nats.SetHealthStatus(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					Uptime:           "1h",
					Reconnects:       0,
					JetStreamEnabled: false, // JetStream not available
					CircuitBreaker:   "closed",
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "JetStream not available")
			},
		},
		{
			name: "both_database_and_nats_unhealthy_returns_unhealthy_status",
			setupMocks: func(nats *MockMessagePublisherWithHealth, repo *mockRepositoryRepo, jobs *mockIndexingJobRepo) {
				// Database is unhealthy
				repo.findAllResult = errors.New("database connection failed")
				// NATS is also unhealthy
				nats.SetHealthStatus(outbound.MessagePublisherHealthStatus{
					Connected:        false,
					LastError:        "server unreachable",
					JetStreamEnabled: false,
					CircuitBreaker:   "open",
				})
			},
			expectedStatus: "unhealthy",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				dbStatus := response.Dependencies["database"]
				assert.Equal(t, "unhealthy", dbStatus.Status)

				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockNATS := NewMockMessagePublisherWithHealth()
			mockRepo := &mockRepositoryRepo{}
			mockJobs := &mockIndexingJobRepo{}

			tt.setupMocks(mockNATS, mockRepo, mockJobs)

			// This test will fail because the HealthServiceAdapter doesn't yet implement
			// the enhanced NATS health monitoring integration
			service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

			response, err := service.GetHealth(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, response.Status)
			tt.validateResponse(t, response)
		})
	}
}

func TestHealthServiceAdapter_GetHealth_NATSResponseTimeTracking(t *testing.T) {
	t.Run("health_check_includes_nats_response_time", func(t *testing.T) {
		mockNATS := NewMockMessagePublisherWithHealth()
		mockRepo := &mockRepositoryRepo{}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		start := time.Now()
		response, err := service.GetHealth(context.Background())
		duration := time.Since(start)

		require.NoError(t, err)

		natsStatus := response.Dependencies["nats"]
		require.NotEmpty(t, natsStatus.ResponseTime)

		// Response time should be reasonable and formatted correctly
		assert.Regexp(t, `^\d+(\.\d+)?ms$`, natsStatus.ResponseTime)

		// Actual response time should be less than test duration
		// This ensures we're measuring the actual health check time
		assert.True(t, duration.Milliseconds() > 0)
	})
}

func TestHealthServiceAdapter_GetHealth_NATSHealthCaching(t *testing.T) {
	t.Run("health_checks_are_cached_to_avoid_expensive_operations", func(t *testing.T) {
		mockNATS := NewMockMessagePublisherWithHealth()
		mockRepo := &mockRepositoryRepo{}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// First call should establish cache
		response1, err := service.GetHealth(context.Background())
		require.NoError(t, err)

		// Change NATS status
		mockNATS.SetHealthStatus(outbound.MessagePublisherHealthStatus{
			Connected:        false,
			LastError:        "connection lost",
			JetStreamEnabled: false,
			CircuitBreaker:   "open",
		})

		// Second call within cache window should return cached result
		response2, err := service.GetHealth(context.Background())
		require.NoError(t, err)

		// This test will fail because caching is not yet implemented
		// The responses should be identical due to caching
		nats1 := response1.Dependencies["nats"]
		nats2 := response2.Dependencies["nats"]
		assert.Equal(t, nats1.Status, nats2.Status)
		assert.Equal(t, nats1.Message, nats2.Message)
	})
}

func TestHealthServiceAdapter_GetHealth_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent_health_checks_are_handled_safely", func(t *testing.T) {
		mockNATS := NewMockMessagePublisherWithHealth()
		mockRepo := &mockRepositoryRepo{}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// Start multiple concurrent health checks
		const numGoroutines = 10
		responses := make([]*dto.HealthResponse, numGoroutines)
		errors := make([]error, numGoroutines)

		done := make(chan int, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				defer func() { done <- index }()
				responses[index], errors[index] = service.GetHealth(context.Background())
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// All responses should be successful and consistent
		for i := 0; i < numGoroutines; i++ {
			require.NoError(t, errors[i])
			require.NotNil(t, responses[i])
			assert.Equal(t, "healthy", responses[i].Status)

			// NATS status should be present in all responses
			natsStatus, exists := responses[i].Dependencies["nats"]
			assert.True(t, exists)
			assert.Equal(t, "healthy", natsStatus.Status)
		}
	})
}

// Mock implementations for testing
type mockRepositoryRepo struct {
	findAllResult error
}

func (m *mockRepositoryRepo) Save(ctx context.Context, repository *entity.Repository) error {
	return nil
}

func (m *mockRepositoryRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Repository, error) {
	return nil, nil
}

func (m *mockRepositoryRepo) FindByURL(ctx context.Context, url valueobject.RepositoryURL) (*entity.Repository, error) {
	return nil, nil
}

func (m *mockRepositoryRepo) FindAll(ctx context.Context, filters outbound.RepositoryFilters) ([]*entity.Repository, int, error) {
	if m.findAllResult != nil {
		return nil, 0, m.findAllResult
	}
	return []*entity.Repository{}, 0, nil
}

func (m *mockRepositoryRepo) Update(ctx context.Context, repository *entity.Repository) error {
	return nil
}

func (m *mockRepositoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockRepositoryRepo) Exists(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	return false, nil
}

func (m *mockRepositoryRepo) ExistsByNormalizedURL(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	return false, nil
}

func (m *mockRepositoryRepo) FindByNormalizedURL(ctx context.Context, url valueobject.RepositoryURL) (*entity.Repository, error) {
	return nil, nil
}

type mockIndexingJobRepo struct{}

func (m *mockIndexingJobRepo) Save(ctx context.Context, job *entity.IndexingJob) error {
	return nil
}

func (m *mockIndexingJobRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.IndexingJob, error) {
	return nil, nil
}

func (m *mockIndexingJobRepo) FindByRepositoryID(ctx context.Context, repositoryID uuid.UUID, filters outbound.IndexingJobFilters) ([]*entity.IndexingJob, int, error) {
	return []*entity.IndexingJob{}, 0, nil
}

func (m *mockIndexingJobRepo) Update(ctx context.Context, job *entity.IndexingJob) error {
	return nil
}

func (m *mockIndexingJobRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}
