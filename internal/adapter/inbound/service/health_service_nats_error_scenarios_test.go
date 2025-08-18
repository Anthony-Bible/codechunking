package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"
	"codechunking/internal/port/outbound"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthServiceAdapter_NATSErrorScenarios(t *testing.T) {
	tests := []struct {
		name             string
		setupMocks       func(*testutil.MockMessagePublisherWithHealthMonitoring)
		expectedStatus   string
		validateResponse func(t *testing.T, response *dto.HealthResponse)
		expectError      bool
	}{
		{
			name: "nats_connection_timeout_during_health_check",
			setupMocks: func(nats *testutil.MockMessagePublisherWithHealthMonitoring) {
				// Simulate slow health check that times out
				nats.SetSimulateLatency(6 * time.Second) // Longer than typical timeout
				nats.SetErrorMode("disconnected")
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus, exists := response.Dependencies["nats"]
				require.True(t, exists)
				assert.Equal(t, "unhealthy", natsStatus.Status)

				// Should indicate timeout in response
				assert.Contains(t, natsStatus.Message, "timeout")
				assert.Equal(t, "timeout", natsStatus.ResponseTime)

				// Details should show connection issues
				if natsStatus.Details != nil {
					if errorMsg, hasError := natsStatus.Details["error"]; hasError {
						assert.Contains(t, errorMsg.(string), "timeout")
					}
				}
			},
		},
		{
			name: "nats_server_unreachable_returns_connection_error",
			setupMocks: func(nats *testutil.MockMessagePublisherWithHealthMonitoring) {
				nats.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
					Connected:        false,
					LastError:        "dial tcp 127.0.0.1:4222: connect: connection refused",
					Uptime:           "0s",
					Reconnects:       0,
					JetStreamEnabled: false,
					CircuitBreaker:   "open",
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "connection refused")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.False(t, natsDetails.Connected)
				assert.Contains(t, natsDetails.LastError, "connection refused")
				assert.False(t, natsDetails.JetStreamEnabled)
			},
		},
		{
			name: "nats_frequent_disconnections_indicate_network_instability",
			setupMocks: func(nats *testutil.MockMessagePublisherWithHealthMonitoring) {
				nats.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					LastError:        "previous connection lost",
					Uptime:           "2m30s",
					Reconnects:       20, // Very high reconnection rate
					JetStreamEnabled: true,
					CircuitBreaker:   "half-open",
				})
				nats.SetMessageMetrics(outbound.MessagePublisherMetrics{
					PublishedCount: 500,
					FailedCount:    150,    // High failure rate due to instability
					AverageLatency: "45ms", // High latency due to network issues
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "unstable")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.True(t, natsDetails.Connected) // Still connected but unstable
				assert.Equal(t, 20, natsDetails.Reconnects)
				assert.Equal(t, "half-open", natsDetails.CircuitBreaker)
				assert.Equal(t, int64(150), natsDetails.MessageMetrics.FailedCount)
			},
		},
		{
			name: "jetstream_not_available_affects_service_health",
			setupMocks: func(nats *testutil.MockMessagePublisherWithHealthMonitoring) {
				nats.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					LastError:        "JetStream not enabled on server",
					Uptime:           "1h",
					Reconnects:       0,
					JetStreamEnabled: false, // Critical for this service
					CircuitBreaker:   "closed",
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "JetStream not available")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.True(t, natsDetails.Connected)         // Core NATS works
				assert.False(t, natsDetails.JetStreamEnabled) // But JetStream doesn't
			},
		},
		{
			name: "circuit_breaker_open_prevents_message_publishing",
			setupMocks: func(nats *testutil.MockMessagePublisherWithHealthMonitoring) {
				nats.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					LastError:        "too many consecutive failures",
					Uptime:           "30m",
					Reconnects:       2,
					JetStreamEnabled: true,
					CircuitBreaker:   "open", // Circuit breaker protecting the system
				})
				nats.SetMessageMetrics(outbound.MessagePublisherMetrics{
					PublishedCount: 1000,
					FailedCount:    600,   // Very high failure rate triggered circuit breaker
					AverageLatency: "0ms", // No recent attempts due to circuit breaker
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "circuit breaker open")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.Equal(t, "open", natsDetails.CircuitBreaker)
				assert.Equal(t, int64(600), natsDetails.MessageMetrics.FailedCount)
			},
		},
		{
			name: "authentication_failure_prevents_nats_connection",
			setupMocks: func(nats *testutil.MockMessagePublisherWithHealthMonitoring) {
				nats.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
					Connected:        false,
					LastError:        "nats: Authorization Violation",
					Uptime:           "0s",
					Reconnects:       0,
					JetStreamEnabled: false,
					CircuitBreaker:   "open",
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "Authorization Violation")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.Contains(t, natsDetails.LastError, "Authorization Violation")
			},
		},
		{
			name: "nats_server_version_incompatibility",
			setupMocks: func(nats *testutil.MockMessagePublisherWithHealthMonitoring) {
				nats.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					LastError:        "protocol version mismatch",
					Uptime:           "5m",
					Reconnects:       1,
					JetStreamEnabled: false, // Old version doesn't support JetStream
					CircuitBreaker:   "closed",
				})
				// Add server info indicating old version
				serverInfo := map[string]interface{}{
					"version":    "1.4.0", // Old version
					"proto":      1,       // Old protocol
					"git_commit": "abc123",
				}
				natsHealth := dto.NATSHealthDetails{
					Connected:        true,
					Uptime:           "5m",
					Reconnects:       1,
					LastError:        "protocol version mismatch",
					JetStreamEnabled: false,
					CircuitBreaker:   "closed",
					MessageMetrics: dto.NATSMessageMetrics{
						PublishedCount: 0,   // Can't publish due to compatibility
						FailedCount:    100, // All attempts fail
						AverageLatency: "0ms",
					},
					ServerInfo: serverInfo,
				}

				// Use the builder to create a proper response
				_ = testutil.NewEnhancedHealthResponseBuilder().
					WithStatus("degraded").
					WithVersion("1.0.0").
					WithNATSHealth(natsHealth, "unhealthy", "NATS server version incompatible", "5ms").
					BuildEnhanced()

				// Mock the health service to return this response
				// This would need to be handled by the actual test setup
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "incompatible")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.Equal(t, int64(0), natsDetails.MessageMetrics.PublishedCount)
				assert.Equal(t, int64(100), natsDetails.MessageMetrics.FailedCount)

				if natsDetails.ServerInfo != nil {
					version, exists := natsDetails.ServerInfo["version"]
					if exists {
						assert.Equal(t, "1.4.0", version.(string))
					}
				}
			},
		},
		{
			name: "nats_memory_pressure_affects_performance",
			setupMocks: func(nats *testutil.MockMessagePublisherWithHealthMonitoring) {
				nats.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
					Connected:        true,
					LastError:        "slow consumer detected",
					Uptime:           "2h",
					Reconnects:       0,
					JetStreamEnabled: true,
					CircuitBreaker:   "half-open", // Intermittent issues
				})
				nats.SetMessageMetrics(outbound.MessagePublisherMetrics{
					PublishedCount: 10000,
					FailedCount:    500,
					AverageLatency: "150ms", // Very slow due to memory pressure
				})
			},
			expectedStatus: "degraded",
			validateResponse: func(t *testing.T, response *dto.HealthResponse) {
				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "performance degraded")

				natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
				assert.Contains(t, natsDetails.LastError, "slow consumer")
				assert.Equal(t, "150ms", natsDetails.MessageMetrics.AverageLatency)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
			mockRepo := &mockRepositoryRepo{findAllResult: nil} // Database healthy
			mockJobs := &mockIndexingJobRepo{}

			tt.setupMocks(mockNATS)

			// This test will fail because enhanced NATS error handling is not implemented
			service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

			// Execute with timeout to handle slow operations
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			response, err := service.GetHealth(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, response)
				assert.Equal(t, tt.expectedStatus, response.Status)
				tt.validateResponse(t, response)
			}
		})
	}
}

func TestHealthServiceAdapter_NATSReconnectionScenarios(t *testing.T) {
	t.Run("nats_reconnection_during_health_check_handled_gracefully", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// Simulate NATS being initially disconnected
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        false,
			LastError:        "connection lost",
			Uptime:           "0s",
			Reconnects:       5,
			JetStreamEnabled: false,
			CircuitBreaker:   "open",
		})

		// First health check should show unhealthy
		response1, err := service.GetHealth(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "degraded", response1.Status)

		natsStatus1 := response1.Dependencies["nats"]
		assert.Equal(t, "unhealthy", natsStatus1.Status)

		// Simulate NATS reconnection
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			LastError:        "",       // Error cleared
			Uptime:           "30s",    // Recently reconnected
			Reconnects:       6,        // Incremented reconnect count
			JetStreamEnabled: true,     // Now available
			CircuitBreaker:   "closed", // Circuit breaker reset
		})

		// Clear cache to ensure fresh health check reads the updated mock state
		if clearableService, ok := service.(*HealthServiceAdapter); ok {
			clearableService.ClearCache()
		}

		// Second health check should show recovery
		response2, err := service.GetHealth(context.Background())
		require.NoError(t, err)

		// This test will fail because the service doesn't yet handle
		// NATS reconnection status properly
		assert.Equal(t, "healthy", response2.Status)

		natsStatus2 := response2.Dependencies["nats"]
		assert.Equal(t, "healthy", natsStatus2.Status)

		// Should track the reconnection event
		natsDetails := natsStatus2.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, 6, natsDetails.Reconnects)
		assert.Equal(t, "30s", natsDetails.Uptime)
	})

	t.Run("repeated_disconnections_trigger_circuit_breaker", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// Simulate multiple rapid disconnections
		disconnectionEvents := []outbound.MessagePublisherHealthStatus{
			{Connected: false, Reconnects: 1, LastError: "network error", CircuitBreaker: "closed"},
			{Connected: true, Reconnects: 2, LastError: "recovered", CircuitBreaker: "closed"},
			{Connected: false, Reconnects: 3, LastError: "network error", CircuitBreaker: "closed"},
			{Connected: true, Reconnects: 4, LastError: "recovered", CircuitBreaker: "half-open"},
			{Connected: false, Reconnects: 5, LastError: "network error", CircuitBreaker: "half-open"},
			{Connected: false, Reconnects: 6, LastError: "too many failures", CircuitBreaker: "open"},
		}

		var lastResponse *dto.HealthResponse
		for i, event := range disconnectionEvents {
			mockNATS.SetConnectionHealth(event)

			// Clear cache to ensure fresh health check reads the updated mock state
			if clearableService, ok := service.(*HealthServiceAdapter); ok {
				clearableService.ClearCache()
			}

			response, err := service.GetHealth(context.Background())
			require.NoError(t, err, "Health check %d should not error", i+1)

			lastResponse = response
		}

		// Final state should show circuit breaker protection
		natsStatus := lastResponse.Dependencies["nats"]
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "circuit breaker")

		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, "open", natsDetails.CircuitBreaker)
		assert.Equal(t, 6, natsDetails.Reconnects)
	})
}

func TestHealthServiceAdapter_NATSTimeoutHandling(t *testing.T) {
	t.Run("health_check_timeout_returns_partial_status", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Configure NATS to be very slow
		mockNATS.SetSimulateLatency(2 * time.Second)

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// Use a short timeout to force timeout scenario
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		response, err := service.GetHealth(ctx)

		// This test will fail because timeout handling is not yet implemented
		// The service should return partial health status when NATS check times out
		require.NoError(t, err) // Should not error, but return degraded status
		assert.Equal(t, "degraded", response.Status)

		// Should indicate timeout in NATS status
		natsStatus, exists := response.Dependencies["nats"]
		require.True(t, exists)
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "timeout")
		assert.Equal(t, "timeout", natsStatus.ResponseTime)
	})

	t.Run("partial_nats_failure_does_not_block_database_health_check", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil} // Database is healthy
		mockJobs := &mockIndexingJobRepo{}

		// NATS will timeout
		mockNATS.SetSimulateLatency(5 * time.Second)

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		response, err := service.GetHealth(ctx)

		require.NoError(t, err)

		// Database should still be checked and healthy
		dbStatus, exists := response.Dependencies["database"]
		require.True(t, exists)
		assert.Equal(t, "healthy", dbStatus.Status)

		// NATS should show timeout
		natsStatus, exists := response.Dependencies["nats"]
		require.True(t, exists)
		assert.Equal(t, "unhealthy", natsStatus.Status)

		// Overall status should be degraded due to NATS timeout
		assert.Equal(t, "degraded", response.Status)
	})
}

func TestHealthServiceAdapter_NATSVersionIncompatibility(t *testing.T) {
	t.Run("protocol_version_mismatch_detected_as_incompatible", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Setup NATS with protocol version mismatch error
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			LastError:        "protocol version mismatch",
			Uptime:           "1m",
			Reconnects:       1,
			JetStreamEnabled: false,
			CircuitBreaker:   "closed",
		})
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: 0,
			FailedCount:    50,
			AverageLatency: "0ms",
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "degraded", response.Status)

		// Validate that version incompatibility is properly detected
		natsStatus := response.Dependencies["nats"]
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "incompatible")

		// Verify the protocol version mismatch is captured in details
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, "protocol version mismatch", natsDetails.LastError)
		assert.Equal(t, int64(0), natsDetails.MessageMetrics.PublishedCount)
		assert.Equal(t, int64(100), natsDetails.MessageMetrics.FailedCount) // Should be adjusted due to incompatibility
	})

	t.Run("version_mismatch_error_detected_as_incompatible", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Setup NATS with version mismatch error (alternative error format)
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        false,
			LastError:        "nats: server version mismatch detected",
			Uptime:           "0s",
			Reconnects:       0,
			JetStreamEnabled: false,
			CircuitBreaker:   "open",
		})
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: 0,
			FailedCount:    25,
			AverageLatency: "0ms",
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "degraded", response.Status)

		// Validate that version mismatch is properly detected
		natsStatus := response.Dependencies["nats"]
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "incompatible")

		// Verify the version mismatch is captured in details
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Contains(t, natsDetails.LastError, "version mismatch")
		assert.Equal(t, int64(0), natsDetails.MessageMetrics.PublishedCount)
		assert.Equal(t, int64(100), natsDetails.MessageMetrics.FailedCount) // Should be adjusted due to incompatibility
	})

	t.Run("case_insensitive_version_incompatibility_detection", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Setup NATS with mixed case version error
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			LastError:        "NATS Protocol Version Mismatch Error",
			Uptime:           "30s",
			Reconnects:       0,
			JetStreamEnabled: false,
			CircuitBreaker:   "closed",
		})
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: 10,
			FailedCount:    90,
			AverageLatency: "5ms",
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "degraded", response.Status)

		// Validate that mixed case version error is properly detected
		natsStatus := response.Dependencies["nats"]
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "incompatible")
		assert.Equal(t, "NATS server version incompatible", natsStatus.Message)

		// Verify the mixed case error is captured in details
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Contains(t, strings.ToLower(natsDetails.LastError), "protocol version")
		assert.Equal(t, int64(0), natsDetails.MessageMetrics.PublishedCount) // Should be zeroed due to incompatibility
		assert.Equal(
			t,
			int64(100),
			natsDetails.MessageMetrics.FailedCount,
		) // Should be adjusted due to incompatibility
	})
}

func TestHealthServiceAdapter_CheckVersionIncompatibilityDirect(t *testing.T) {
	t.Run("checkVersionIncompatibility_detects_protocol_version_mismatch", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0").(*HealthServiceAdapter)

		// Test protocol version mismatch detection
		healthStatus := outbound.MessagePublisherHealthStatus{
			Connected:        true,
			LastError:        "protocol version mismatch",
			Uptime:           "1m",
			Reconnects:       1,
			JetStreamEnabled: true, // This should not affect version check
			CircuitBreaker:   "closed",
		}

		status, message, handled := service.checkVersionIncompatibility(healthStatus)

		// This should detect the version incompatibility
		assert.True(t, handled, "Version incompatibility should be handled")
		assert.Equal(t, dto.DependencyStatusUnhealthy, status)
		assert.Contains(t, message, "incompatible")
		assert.Equal(t, "NATS server version incompatible", message)
	})

	t.Run("checkVersionIncompatibility_detects_version_mismatch", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0").(*HealthServiceAdapter)

		// Test version mismatch detection
		healthStatus := outbound.MessagePublisherHealthStatus{
			Connected:        false,
			LastError:        "server version mismatch detected",
			Uptime:           "0s",
			Reconnects:       0,
			JetStreamEnabled: false,
			CircuitBreaker:   "open", // This should not affect version check
		}

		status, message, handled := service.checkVersionIncompatibility(healthStatus)

		// This should detect the version incompatibility
		assert.True(t, handled, "Version incompatibility should be handled")
		assert.Equal(t, dto.DependencyStatusUnhealthy, status)
		assert.Contains(t, message, "incompatible")
		assert.Equal(t, "NATS server version incompatible", message)
	})

	t.Run("checkVersionIncompatibility_case_insensitive", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0").(*HealthServiceAdapter)

		// Test case insensitive detection
		healthStatus := outbound.MessagePublisherHealthStatus{
			Connected:        true,
			LastError:        "PROTOCOL VERSION MISMATCH ERROR",
			Uptime:           "30s",
			Reconnects:       0,
			JetStreamEnabled: false,
			CircuitBreaker:   "closed",
		}

		status, message, handled := service.checkVersionIncompatibility(healthStatus)

		// This should detect the version incompatibility regardless of case
		assert.True(t, handled, "Version incompatibility should be handled")
		assert.Equal(t, dto.DependencyStatusUnhealthy, status)
		assert.Contains(t, message, "incompatible")
		assert.Equal(t, "NATS server version incompatible", message)
	})

	t.Run("checkVersionIncompatibility_no_version_error", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0").(*HealthServiceAdapter)

		// Test that non-version errors are not handled
		healthStatus := outbound.MessagePublisherHealthStatus{
			Connected:        false,
			LastError:        "connection refused",
			Uptime:           "0s",
			Reconnects:       5,
			JetStreamEnabled: false,
			CircuitBreaker:   "open",
		}

		status, message, handled := service.checkVersionIncompatibility(healthStatus)

		// This should NOT be handled by version incompatibility check
		assert.False(t, handled, "Non-version error should not be handled by version check")
		assert.Equal(t, dto.DependencyStatusHealthy, status)
		assert.Empty(t, message)
	})
}

func TestHealthServiceAdapter_NATSMetricsValidation(t *testing.T) {
	t.Run("negative_published_count_causes_unhealthy_status", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Set connection as healthy but metrics as invalid
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		})

		// Configure negative published count - invalid metric
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: -5,     // Invalid negative count
			FailedCount:    0,      // Valid count
			AverageLatency: "10ms", // Valid latency
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")
		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		natsStatus := response.Dependencies["nats"]

		// Should be unhealthy due to invalid metrics despite good connection
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "invalid metrics")

		// Response should sanitize negative count to zero
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, int64(0), natsDetails.MessageMetrics.PublishedCount)
		assert.Equal(t, int64(0), natsDetails.MessageMetrics.FailedCount)
		assert.Equal(t, "10ms", natsDetails.MessageMetrics.AverageLatency)
	})

	t.Run("invalid_latency_format_causes_unhealthy_status", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Set connection as healthy
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		})

		// Configure invalid latency format
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: 100,       // Valid count
			FailedCount:    5,         // Valid count
			AverageLatency: "invalid", // Invalid latency format
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")
		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		natsStatus := response.Dependencies["nats"]

		// Should be unhealthy due to invalid latency format
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "invalid metrics")

		// Response should sanitize invalid latency to default
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, int64(100), natsDetails.MessageMetrics.PublishedCount)
		assert.Equal(t, int64(5), natsDetails.MessageMetrics.FailedCount)
		assert.Equal(t, "0s", natsDetails.MessageMetrics.AverageLatency) // Sanitized to valid default
	})

	t.Run("multiple_invalid_metrics_causes_unhealthy_status", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Set connection as healthy
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		})

		// Configure multiple invalid metrics
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: -10,        // Invalid negative count
			FailedCount:    -2,         // Invalid negative count
			AverageLatency: "not-time", // Invalid latency format
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")
		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		natsStatus := response.Dependencies["nats"]

		// Should be unhealthy due to multiple invalid metrics
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "invalid metrics")

		// All invalid values should be sanitized
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, int64(0), natsDetails.MessageMetrics.PublishedCount)
		assert.Equal(t, int64(0), natsDetails.MessageMetrics.FailedCount)
		assert.Equal(t, "0s", natsDetails.MessageMetrics.AverageLatency)
	})

	t.Run("metrics_validation_overrides_connection_status", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Set connection as perfectly healthy
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			Reconnects:       0,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
			Uptime:           "1h30m",
		})

		// But metrics are invalid
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: -1,
			FailedCount:    0,
			AverageLatency: "5ms",
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")
		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		natsStatus := response.Dependencies["nats"]

		// Metrics validation should override good connection status
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "invalid metrics")
		assert.NotContains(t, natsStatus.Message, "connected") // Should not mention good connection
	})

	t.Run("edge_case_empty_latency_string_is_invalid", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Set connection as healthy
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		})

		// Configure empty latency string - should be considered invalid
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: 50,
			FailedCount:    1,
			AverageLatency: "", // Empty string - invalid
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")
		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		natsStatus := response.Dependencies["nats"]

		// Should be unhealthy due to empty latency
		assert.Equal(t, "unhealthy", natsStatus.Status)
		assert.Contains(t, natsStatus.Message, "invalid metrics")

		// Empty latency should be sanitized to default
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, "0s", natsDetails.MessageMetrics.AverageLatency)
	})

	t.Run("valid_metrics_with_good_connection_remain_healthy", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Set connection as healthy
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		})

		// Configure completely valid metrics
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: 1000,
			FailedCount:    5,
			AverageLatency: "15ms",
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")
		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		natsStatus := response.Dependencies["nats"]

		// Should remain healthy with valid metrics
		assert.Equal(t, "healthy", natsStatus.Status)
		assert.Empty(t, natsStatus.Message) // No error message

		// Metrics should pass through unchanged
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, int64(1000), natsDetails.MessageMetrics.PublishedCount)
		assert.Equal(t, int64(5), natsDetails.MessageMetrics.FailedCount)
		assert.Equal(t, "15ms", natsDetails.MessageMetrics.AverageLatency)
	})

	t.Run("metrics_validation_works_when_connection_is_disconnected", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Set connection as disconnected
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:      false,
			LastError:      "connection failed",
			CircuitBreaker: "open",
		})

		// But also set invalid metrics
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: -1, // Invalid
			FailedCount:    0,
			AverageLatency: "invalid", // Invalid
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")
		response, err := service.GetHealth(context.Background())

		require.NoError(t, err)
		natsStatus := response.Dependencies["nats"]

		// Should be unhealthy, but we want to ensure invalid metrics are still sanitized
		// even when connection issues take priority
		assert.Equal(t, "unhealthy", natsStatus.Status)

		// Invalid metrics should still be sanitized in response
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.Equal(t, int64(0), natsDetails.MessageMetrics.PublishedCount)
		assert.Equal(t, "0s", natsDetails.MessageMetrics.AverageLatency)
	})
}
