package api

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_GetHealth_WithNATSEnhancedResponse(t *testing.T) {
	tests := []struct {
		name             string
		mockSetup        func(*testutil.MockHealthService)
		expectedStatus   int
		validateResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "healthy_nats_returns_detailed_metrics_in_response",
			mockSetup: func(mock *testutil.MockHealthService) {
				natsDetails := dto.NATSHealthDetails{
					Connected:        true,
					Uptime:           "3h45m12s",
					Reconnects:       2,
					LastError:        "",
					JetStreamEnabled: true,
					CircuitBreaker:   "closed",
					MessageMetrics: dto.NATSMessageMetrics{
						PublishedCount: 15674,
						FailedCount:    8,
						AverageLatency: "1.8ms",
					},
					ServerInfo: map[string]interface{}{
						"version":     "2.9.0",
						"max_payload": 1048576,
						"client_id":   12345,
						"server_name": "nats-server-1",
					},
				}

				response := dto.HealthResponse{
					Status:    "healthy",
					Timestamp: time.Now(),
					Version:   "1.0.0",
					Dependencies: map[string]dto.DependencyStatus{
						"database": {
							Status:       "healthy",
							Message:      "Connected to PostgreSQL",
							ResponseTime: "3ms",
						},
						"nats": {
							Status:       "healthy",
							Message:      "Connected to NATS server",
							ResponseTime: "2ms",
							Details: map[string]interface{}{
								"nats_health": natsDetails,
							},
						},
					},
				}
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "healthy", response.Status)
				assert.NotNil(t, response.Dependencies)

				// Verify NATS dependency structure
				natsStatus, exists := response.Dependencies["nats"]
				require.True(t, exists, "NATS dependency should exist")
				assert.Equal(t, "healthy", natsStatus.Status)
				assert.Equal(t, "2ms", natsStatus.ResponseTime)

				// Verify detailed NATS health information in JSON response
				require.NotNil(t, natsStatus.Details)
				natsHealthRaw, exists := natsStatus.Details["nats_health"]
				require.True(t, exists, "nats_health should exist in details")

				// Parse the NATS health details from the JSON response
				natsHealthMap, ok := natsHealthRaw.(map[string]interface{})
				require.True(t, ok, "nats_health should be a map in JSON")

				// Verify connection status
				connected, ok := natsHealthMap["connected"].(bool)
				require.True(t, ok)
				assert.True(t, connected)

				// Verify uptime
				uptime, ok := natsHealthMap["uptime"].(string)
				require.True(t, ok)
				assert.Equal(t, "3h45m12s", uptime)

				// Verify reconnects
				reconnects, ok := natsHealthMap["reconnects"].(float64) // JSON numbers are float64
				require.True(t, ok)
				assert.InDelta(t, float64(2), reconnects, 0)

				// Verify JetStream status
				jetstream, ok := natsHealthMap["jetstream_enabled"].(bool)
				require.True(t, ok)
				assert.True(t, jetstream)

				// Verify circuit breaker
				circuitBreaker, ok := natsHealthMap["circuit_breaker"].(string)
				require.True(t, ok)
				assert.Equal(t, "closed", circuitBreaker)

				// Verify message metrics
				metricsRaw, ok := natsHealthMap["message_metrics"]
				require.True(t, ok)
				metrics, ok := metricsRaw.(map[string]interface{})
				require.True(t, ok)

				publishedCount, ok := metrics["published_count"].(float64)
				require.True(t, ok)
				assert.InDelta(t, float64(15674), publishedCount, 0)

				failedCount, ok := metrics["failed_count"].(float64)
				require.True(t, ok)
				assert.InDelta(t, float64(8), failedCount, 0)

				avgLatency, ok := metrics["average_latency"].(string)
				require.True(t, ok)
				assert.Equal(t, "1.8ms", avgLatency)

				// Verify server info
				serverInfoRaw, ok := natsHealthMap["server_info"]
				require.True(t, ok)
				serverInfo, ok := serverInfoRaw.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "2.9.0", serverInfo["version"].(string))
				assert.Equal(t, "nats-server-1", serverInfo["server_name"].(string))
			},
		},
		{
			name: "unhealthy_nats_returns_503_with_error_details",
			mockSetup: func(mock *testutil.MockHealthService) {
				natsDetails := dto.NATSHealthDetails{
					Connected:        false,
					Uptime:           "0s",
					Reconnects:       12,
					LastError:        "nats: connection failed",
					JetStreamEnabled: false,
					CircuitBreaker:   "open",
					MessageMetrics: dto.NATSMessageMetrics{
						PublishedCount: 1000,
						FailedCount:    500,
						AverageLatency: "0ms",
					},
				}

				response := dto.HealthResponse{
					Status:    "unhealthy",
					Timestamp: time.Now(),
					Version:   "1.0.0",
					Dependencies: map[string]dto.DependencyStatus{
						"database": {
							Status:  "unhealthy",
							Message: "Database connection failed",
						},
						"nats": {
							Status:       "unhealthy",
							Message:      "NATS server disconnected",
							ResponseTime: "timeout",
							Details: map[string]interface{}{
								"nats_health": natsDetails,
							},
						},
					},
				}
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusServiceUnavailable,
			validateResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "unhealthy", response.Status)

				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Equal(t, "timeout", natsStatus.ResponseTime)

				// Verify error details are present
				natsHealthRaw := natsStatus.Details["nats_health"]
				natsHealthMap := natsHealthRaw.(map[string]interface{})

				assert.False(t, natsHealthMap["connected"].(bool))
				assert.Equal(t, "nats: connection failed", natsHealthMap["last_error"].(string))
				assert.Equal(t, "open", natsHealthMap["circuit_breaker"].(string))
				assert.False(t, natsHealthMap["jetstream_enabled"].(bool))

				// High failure count should be visible
				metrics := natsHealthMap["message_metrics"].(map[string]interface{})
				assert.InDelta(t, float64(500), metrics["failed_count"].(float64), 0)
			},
		},
		{
			name: "degraded_nats_returns_200_with_warning_details",
			mockSetup: func(mock *testutil.MockHealthService) {
				natsDetails := dto.NATSHealthDetails{
					Connected:        true,
					Uptime:           "5m30s",
					Reconnects:       8, // High reconnect count indicates instability
					LastError:        "temporary network issue",
					JetStreamEnabled: true,
					CircuitBreaker:   "half-open", // Recovering from failures
					MessageMetrics: dto.NATSMessageMetrics{
						PublishedCount: 2500,
						FailedCount:    75,    // Some failures but not critical
						AverageLatency: "8ms", // Higher than normal
					},
				}

				response := dto.HealthResponse{
					Status:    "degraded",
					Timestamp: time.Now(),
					Version:   "1.0.0",
					Dependencies: map[string]dto.DependencyStatus{
						"database": {
							Status: "healthy",
						},
						"nats": {
							Status:       "unhealthy",
							Message:      "NATS connection unstable - high reconnect rate",
							ResponseTime: "8ms",
							Details: map[string]interface{}{
								"nats_health": natsDetails,
							},
						},
					},
				}
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusOK, // Degraded still returns 200
			validateResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "degraded", response.Status)

				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "unhealthy", natsStatus.Status)
				assert.Contains(t, natsStatus.Message, "unstable")

				natsHealthMap := natsStatus.Details["nats_health"].(map[string]interface{})
				assert.InDelta(t, float64(8), natsHealthMap["reconnects"].(float64), 0)
				assert.Equal(t, "half-open", natsHealthMap["circuit_breaker"].(string))
			},
		},
		{
			name: "nats_timeout_during_health_check_handled_gracefully",
			mockSetup: func(mock *testutil.MockHealthService) {
				// Simulate a timeout scenario during health check
				response := dto.HealthResponse{
					Status:    "degraded",
					Timestamp: time.Now(),
					Version:   "1.0.0",
					Dependencies: map[string]dto.DependencyStatus{
						"database": {
							Status: "healthy",
						},
						"nats": {
							Status:       "unhealthy",
							Message:      "Health check timeout",
							ResponseTime: "timeout",
							Details: map[string]interface{}{
								"error": "health check exceeded maximum timeout of 5s",
							},
						},
					},
				}
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				natsStatus := response.Dependencies["nats"]
				assert.Equal(t, "timeout", natsStatus.ResponseTime)
				assert.Contains(t, natsStatus.Message, "timeout")

				// Error details should be present
				errorMsg, exists := natsStatus.Details["error"]
				require.True(t, exists)
				assert.Contains(t, errorMsg.(string), "timeout")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because the current health handler doesn't support
			// the enhanced NATS health response structure
			mockHealthService := testutil.NewMockHealthService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockHealthService)

			handler := NewHealthHandler(mockHealthService, mockErrorHandler)

			req := testutil.CreateRequest(http.MethodGet, "/health")
			recorder := httptest.NewRecorder()

			handler.GetHealth(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)
			tt.validateResponse(t, recorder)
		})
	}
}

func TestHealthHandler_GetHealth_NATSResponseHeaders(t *testing.T) {
	t.Run("health_response_includes_nats_specific_headers", func(t *testing.T) {
		mockHealthService := testutil.NewMockHealthService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		// This test will fail because enhanced headers are not yet implemented
		response := dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Dependencies: map[string]dto.DependencyStatus{
				"nats": {
					Status: "healthy",
					Details: map[string]interface{}{
						"nats_health": dto.NATSHealthDetails{
							Connected:        true,
							JetStreamEnabled: true,
							MessageMetrics: dto.NATSMessageMetrics{
								PublishedCount: 1000,
							},
						},
					},
				},
			},
		}
		mockHealthService.ExpectGetHealth(&response, nil)

		handler := NewHealthHandler(mockHealthService, mockErrorHandler)
		req := testutil.CreateRequest(http.MethodGet, "/health")
		recorder := httptest.NewRecorder()

		handler.GetHealth(recorder, req)

		// Verify enhanced health response headers
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

		// These headers should be added for monitoring systems
		assert.NotEmpty(t, recorder.Header().Get("X-Health-Check-Duration"))
		assert.NotEmpty(t, recorder.Header().Get("X-Nats-Connection-Status"))
		assert.NotEmpty(t, recorder.Header().Get("X-Jetstream-Enabled"))
	})
}

func TestHealthHandler_GetHealth_PerformanceWithNATSMetrics(t *testing.T) {
	t.Run("health_endpoint_response_time_under_threshold_with_nats_metrics", func(t *testing.T) {
		mockHealthService := testutil.NewMockHealthService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		// Complex NATS health response with lots of metrics
		natsDetails := dto.NATSHealthDetails{
			Connected:        true,
			Uptime:           "72h15m45s",
			Reconnects:       156,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
			MessageMetrics: dto.NATSMessageMetrics{
				PublishedCount: 999999,
				FailedCount:    1234,
				AverageLatency: "2.1ms",
			},
			ServerInfo: map[string]interface{}{
				"version":        "2.9.0",
				"max_payload":    1048576,
				"client_id":      54321,
				"server_name":    "nats-cluster-1",
				"cluster":        "production",
				"connections":    500,
				"total_msgs":     50000000,
				"total_bytes":    "15.2GB",
				"slow_consumers": 2,
			},
		}

		response := dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Dependencies: map[string]dto.DependencyStatus{
				"nats": {
					Status: "healthy",
					Details: map[string]interface{}{
						"nats_health": natsDetails,
					},
				},
			},
		}
		mockHealthService.ExpectGetHealth(&response, nil)

		handler := NewHealthHandler(mockHealthService, mockErrorHandler)
		req := testutil.CreateRequest(http.MethodGet, "/health")
		recorder := httptest.NewRecorder()

		start := time.Now()
		handler.GetHealth(recorder, req)
		duration := time.Since(start)

		// Health endpoint should respond quickly even with detailed metrics
		assert.Less(t, duration, 100*time.Millisecond,
			"Health check should complete within 100ms, took %v", duration)

		assert.Equal(t, http.StatusOK, recorder.Code)

		// Verify the large response is properly serialized
		var actualResponse dto.HealthResponse
		err := testutil.ParseJSONResponse(recorder, &actualResponse)
		require.NoError(t, err)

		natsStatus := actualResponse.Dependencies["nats"]
		natsHealthMap := natsStatus.Details["nats_health"].(map[string]interface{})

		// Verify complex server info is preserved
		serverInfo := natsHealthMap["server_info"].(map[string]interface{})
		assert.Equal(t, "production", serverInfo["cluster"].(string))
		assert.InDelta(t, float64(500), serverInfo["connections"].(float64), 0)
		assert.Equal(t, "15.2GB", serverInfo["total_bytes"].(string))
	})
}

func TestHealthHandler_GetHealth_ConcurrentNATSHealthChecks(t *testing.T) {
	t.Run("concurrent_health_requests_return_consistent_nats_data", func(t *testing.T) {
		mockHealthService := testutil.NewMockHealthService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		response := dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Dependencies: map[string]dto.DependencyStatus{
				"nats": {
					Status: "healthy",
					Details: map[string]interface{}{
						"nats_health": dto.NATSHealthDetails{
							Connected:        true,
							Uptime:           "1h30m",
							Reconnects:       3,
							JetStreamEnabled: true,
							CircuitBreaker:   "closed",
							MessageMetrics: dto.NATSMessageMetrics{
								PublishedCount: 5000,
								FailedCount:    10,
								AverageLatency: "2ms",
							},
						},
					},
				},
			},
		}
		mockHealthService.ExpectGetHealth(&response, nil)

		handler := NewHealthHandler(mockHealthService, mockErrorHandler)

		// Make concurrent requests
		const numRequests = 20
		responses := make([]*httptest.ResponseRecorder, numRequests)
		done := make(chan int, numRequests)

		for i := range numRequests {
			go func(index int) {
				defer func() { done <- index }()

				req := testutil.CreateRequest(http.MethodGet, "/health")
				responses[index] = httptest.NewRecorder()
				handler.GetHealth(responses[index], req)
			}(i)
		}

		// Wait for all requests to complete
		for range numRequests {
			<-done
		}

		// All responses should be successful and consistent
		for i := range numRequests {
			assert.Equal(t, http.StatusOK, responses[i].Code)

			var healthResp dto.HealthResponse
			err := testutil.ParseJSONResponse(responses[i], &healthResp)
			require.NoError(t, err)

			// NATS data should be consistent across all responses
			natsStatus := healthResp.Dependencies["nats"]
			assert.Equal(t, "healthy", natsStatus.Status)

			natsHealthMap := natsStatus.Details["nats_health"].(map[string]interface{})
			assert.True(t, natsHealthMap["connected"].(bool))
			assert.Equal(t, "1h30m", natsHealthMap["uptime"].(string))
			assert.InDelta(t, float64(3), natsHealthMap["reconnects"].(float64), 0)
		}
	})
}

func TestHealthHandler_GetHealth_NATSErrorPropagation(t *testing.T) {
	t.Run("nats_health_service_errors_are_properly_handled", func(t *testing.T) {
		mockHealthService := testutil.NewMockHealthService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		// Simulate health service returning an error due to NATS issues
		natsError := errors.New("NATS health check failed: connection timeout")
		mockHealthService.ExpectGetHealth(nil, natsError)

		handler := NewHealthHandler(mockHealthService, mockErrorHandler)
		req := testutil.CreateRequest(http.MethodGet, "/health")
		recorder := httptest.NewRecorder()

		handler.GetHealth(recorder, req)

		// Error handler should be called
		assert.Len(t, mockErrorHandler.HandleServiceErrorCalls, 1)

		call := mockErrorHandler.HandleServiceErrorCalls[0]
		assert.Equal(t, natsError, call.Error)
		assert.Contains(t, call.Error.Error(), "NATS health check failed")
	})
}
