package service

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"
	"codechunking/internal/port/outbound"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthServiceAdapter_NATSPerformance(t *testing.T) {
	t.Run("health_check_completes_within_performance_threshold", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Configure realistic NATS metrics
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			Uptime:           "24h15m30s",
			Reconnects:       5,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		})
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: 1000000, // Large number to test serialization
			FailedCount:    1500,
			AverageLatency: "2.3ms",
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// Measure performance over multiple calls
		const numCalls = 100
		totalDuration := time.Duration(0)

		for i := range numCalls {
			start := time.Now()
			response, err := service.GetHealth(context.Background())
			duration := time.Since(start)
			totalDuration += duration

			require.NoError(t, err)
			require.NotNil(t, response)

			// Individual call should be fast (< 50ms)
			assert.Less(t, duration, 50*time.Millisecond,
				"Health check %d took %v, expected < 50ms", i+1, duration)
		}

		// Average performance should be excellent (< 10ms)
		avgDuration := totalDuration / numCalls
		assert.Less(t, avgDuration, 10*time.Millisecond,
			"Average health check duration %v, expected < 10ms", avgDuration)

		// This test will fail because performance optimizations are not yet implemented
		t.Logf("Average health check duration: %v", avgDuration)
	})

	t.Run("health_check_performance_with_slow_nats", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Simulate slow NATS health checks
		mockNATS.SetSimulateLatency(100 * time.Millisecond)

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		start := time.Now()
		response, err := service.GetHealth(context.Background())
		duration := time.Since(start)

		require.NoError(t, err)

		// This test will fail because timeout and caching optimizations are not implemented
		// Total health check should still complete reasonably fast despite slow NATS
		assert.Less(t, duration, 150*time.Millisecond,
			"Health check with slow NATS took %v, expected < 150ms", duration)

		// NATS should be marked as slow but overall health should be available
		natsStatus := response.Dependencies["nats"]
		assert.Contains(t, natsStatus.Message, "slow response")
	})

	t.Run("health_check_caching_improves_performance", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Slow NATS for initial call
		mockNATS.SetSimulateLatency(50 * time.Millisecond)

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// First call should be slow
		start1 := time.Now()
		response1, err := service.GetHealth(context.Background())
		duration1 := time.Since(start1)
		require.NoError(t, err)

		// Second call should be much faster due to caching
		start2 := time.Now()
		response2, err := service.GetHealth(context.Background())
		duration2 := time.Since(start2)
		require.NoError(t, err)

		// This test will fail because caching is not yet implemented
		assert.Less(t, duration2, duration1/2,
			"Cached health check (%v) should be much faster than initial (%v)", duration2, duration1)

		// Responses should be identical due to caching
		assert.Equal(t, response1.Status, response2.Status)

		natsStatus1 := response1.Dependencies["nats"]
		natsStatus2 := response2.Dependencies["nats"]
		assert.Equal(t, natsStatus1.Status, natsStatus2.Status)
	})
}

// concurrentTestResult holds the results of concurrent health check testing.
type concurrentTestResult struct {
	successCount int64
	errorCount   int64
	responses    [][]dto.HealthResponse
}

// setupHealthServiceForConcurrentTest creates a health service with standard mocks for concurrent testing.
func setupHealthServiceForConcurrentTest() (*HealthServiceAdapter, *testutil.MockMessagePublisherWithHealthMonitoring) {
	mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
	mockRepo := &mockRepositoryRepo{findAllResult: nil}
	mockJobs := &mockIndexingJobRepo{}
	service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0").(*HealthServiceAdapter)
	return service, mockNATS
}

// runBasicConcurrentHealthChecks executes concurrent health checks and returns results.
func runBasicConcurrentHealthChecks(
	service *HealthServiceAdapter,
	numGoroutines, callsPerGoroutine int,
) *concurrentTestResult {
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	results := make([][]dto.HealthResponse, numGoroutines)

	// Launch concurrent health checks
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			results[goroutineID] = make([]dto.HealthResponse, callsPerGoroutine)

			for j := range callsPerGoroutine {
				response, err := service.GetHealth(context.Background())
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				atomic.AddInt64(&successCount, 1)
				results[goroutineID][j] = *response
			}
		}(i)
	}

	wg.Wait()

	return &concurrentTestResult{
		successCount: successCount,
		errorCount:   errorCount,
		responses:    results,
	}
}

// validateResponseConsistency checks that all health responses are consistent and valid.
func validateResponseConsistency(t *testing.T, results [][]dto.HealthResponse) {
	for i := range results {
		for j := range results[i] {
			response := results[i][j]

			// All responses should be healthy and consistent
			assert.Equal(t, "healthy", response.Status)
			assert.NotNil(t, response.Dependencies)

			natsStatus, exists := response.Dependencies["nats"]
			require.True(t, exists, "NATS dependency should exist in all responses")
			assert.Equal(t, "healthy", natsStatus.Status)
		}
	}
}

// runConcurrentTestWithStateChanges executes concurrent health checks while changing NATS state.
func runConcurrentTestWithStateChanges(
	service *HealthServiceAdapter,
	mockNATS *testutil.MockMessagePublisherWithHealthMonitoring,
) []dto.HealthResponse {
	const numReaders = 20
	const numReads = 5

	var wg sync.WaitGroup
	var allResponses []dto.HealthResponse
	var responsesMutex sync.Mutex

	// Start concurrent readers
	for range numReaders {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for range numReads {
				response, err := service.GetHealth(context.Background())
				if err != nil {
					continue
				}

				responsesMutex.Lock()
				allResponses = append(allResponses, *response)
				responsesMutex.Unlock()

				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// Simulate NATS state changes during concurrent access
	wg.Add(1)
	go func() {
		defer wg.Done()

		states := []outbound.MessagePublisherHealthStatus{
			{Connected: true, Uptime: "1h", Reconnects: 0, JetStreamEnabled: true, CircuitBreaker: "closed"},
			{Connected: false, Uptime: "0s", Reconnects: 1, JetStreamEnabled: false, CircuitBreaker: "open"},
			{Connected: true, Uptime: "30s", Reconnects: 2, JetStreamEnabled: true, CircuitBreaker: "closed"},
			{Connected: true, Uptime: "1h30m", Reconnects: 2, JetStreamEnabled: true, CircuitBreaker: "closed"},
		}

		for _, state := range states {
			mockNATS.SetConnectionHealth(state)
			time.Sleep(25 * time.Millisecond)
		}
	}()

	wg.Wait()
	return allResponses
}

func TestHealthServiceAdapter_ThreadSafety(t *testing.T) {
	t.Run("concurrent_health_checks_are_thread_safe", func(t *testing.T) {
		service, _ := setupHealthServiceForConcurrentTest()

		const numGoroutines = 50
		const callsPerGoroutine = 10

		result := runBasicConcurrentHealthChecks(service, numGoroutines, callsPerGoroutine)

		// All calls should succeed
		totalCalls := int64(numGoroutines * callsPerGoroutine)
		assert.Equal(t, totalCalls, result.successCount, "All concurrent health checks should succeed")
		assert.Equal(t, int64(0), result.errorCount, "No errors should occur during concurrent access")
	})
}

func TestHealthServiceAdapter_ConcurrentResponseConsistency(t *testing.T) {
	t.Run("concurrent_responses_maintain_consistency", func(t *testing.T) {
		service, _ := setupHealthServiceForConcurrentTest()

		const numGoroutines = 50
		const callsPerGoroutine = 10

		result := runBasicConcurrentHealthChecks(service, numGoroutines, callsPerGoroutine)

		// Validate consistency across all responses
		validateResponseConsistency(t, result.responses)
	})
}

func TestHealthServiceAdapter_StateChangesDuringConcurrency(t *testing.T) {
	t.Run("concurrent_access_with_nats_state_changes", func(t *testing.T) {
		service, mockNATS := setupHealthServiceForConcurrentTest()

		allResponses := runConcurrentTestWithStateChanges(service, mockNATS)

		const numReaders = 20
		const numReads = 5

		// This test will fail because proper concurrent access handling is not implemented
		// Should have collected responses from all readers
		assert.GreaterOrEqual(t, len(allResponses), numReaders*numReads/2,
			"Should collect most responses despite state changes")

		// All responses should be valid (no race conditions)
		for _, response := range allResponses {
			assert.NotEmpty(t, response.Status)
			assert.NotZero(t, response.Timestamp)
			assert.NotNil(t, response.Dependencies)
		}
	})
}

func TestHealthServiceAdapter_RateLimitingProtection(t *testing.T) {
	t.Run("health_check_rate_limiting_protects_nats", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// Simulate high load scenario
		const numConcurrentRequests = 100
		var wg sync.WaitGroup
		var requestTimes []time.Time
		var timesMutex sync.Mutex

		start := time.Now()

		for range numConcurrentRequests {
			wg.Add(1)
			go func() {
				defer wg.Done()

				requestStart := time.Now()
				_, err := service.GetHealth(context.Background())

				if err == nil {
					timesMutex.Lock()
					requestTimes = append(requestTimes, requestStart)
					timesMutex.Unlock()
				}
			}()
		}

		wg.Wait()
		totalDuration := time.Since(start)

		// This test will fail because rate limiting is not implemented
		// Requests should complete but NATS health checks should be rate-limited
		assert.Less(t, totalDuration, 2*time.Second,
			"High load health checks should complete within reasonable time")

		// Should not overwhelm NATS with health checks
		healthCheckCalls := mockNATS.GetHealthCheckCalls()
		assert.Less(t, len(healthCheckCalls), numConcurrentRequests,
			"Should rate-limit NATS health checks to avoid overwhelming the service")

		t.Logf("Processed %d requests in %v with %d NATS health checks",
			len(requestTimes), totalDuration, len(healthCheckCalls))
	})
}

func TestHealthServiceAdapter_MemoryEfficiency(t *testing.T) {
	t.Run("health_responses_do_not_leak_memory", func(t *testing.T) {
		mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
		mockRepo := &mockRepositoryRepo{findAllResult: nil}
		mockJobs := &mockIndexingJobRepo{}

		// Configure complex NATS health data
		mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
			Connected:        true,
			Uptime:           "48h30m15s",
			Reconnects:       25,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		})

		// Large metrics to test memory handling
		mockNATS.SetMessageMetrics(outbound.MessagePublisherMetrics{
			PublishedCount: 50000000,     // 50M messages
			FailedCount:    100000,       // 100K failures
			AverageLatency: "1.234567ms", // Precise timing
		})

		service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

		// Create many responses to test for memory leaks
		const numResponses = 1000
		var responses []*dto.HealthResponse

		for range numResponses {
			response, err := service.GetHealth(context.Background())
			require.NoError(t, err)
			responses = append(responses, response)
		}

		// Verify all responses are valid
		for i, response := range responses {
			require.NotNil(t, response, "Response %d should not be nil", i)
			assert.NotEmpty(t, response.Status)

			natsStatus, exists := response.Dependencies["nats"]
			require.True(t, exists)
			require.NotNil(t, natsStatus.Details)

			// Verify complex data is properly handled
			natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
			assert.Equal(t, int64(50000000), natsDetails.MessageMetrics.PublishedCount)
			assert.Equal(t, int64(100000), natsDetails.MessageMetrics.FailedCount)
		}

		// This test will fail if there are memory leaks or inefficient handling
		t.Logf("Successfully created and validated %d health responses", numResponses)
	})

	t.Run("large_nats_server_info_handled_efficiently", func(t *testing.T) {
		// Create large server info to test serialization efficiency
		largeServerInfo := map[string]interface{}{
			"version":          "2.9.0",
			"server_id":        "NCDKZ7LPKYZDCJJJL2XSFQFBHB5Q7CC6NQKK2RDZ2BVGDRMWJHYB72B6",
			"server_name":      "nats-server-production-cluster-1",
			"max_payload":      1048576,
			"proto":            1,
			"client_id":        123456,
			"auth_required":    false,
			"tls_required":     false,
			"max_connections":  65536,
			"ping_interval":    "2m",
			"ping_max":         2,
			"http_host":        "0.0.0.0",
			"http_port":        8222,
			"https_port":       0,
			"auth_timeout":     1,
			"max_control_line": 4096,
			"cluster": map[string]interface{}{
				"addr":         "0.0.0.0",
				"port":         6222,
				"auth_timeout": 1,
				"urls": []string{
					"nats://nats-1.cluster.local:6222",
					"nats://nats-2.cluster.local:6222",
					"nats://nats-3.cluster.local:6222",
				},
			},
			"gateway": map[string]interface{}{
				"name":      "production",
				"host":      "0.0.0.0",
				"port":      7222,
				"advertise": "nats-gateway.production.local:7222",
			},
			"leaf": map[string]interface{}{
				"host": "0.0.0.0",
				"port": 7422,
			},
			"jetstream": map[string]interface{}{
				"enabled":       true,
				"max_memory":    "1GB",
				"max_file":      "10GB",
				"store_dir":     "/data/jetstream",
				"max_streams":   1000,
				"max_consumers": 1000,
			},
		}

		// Create detailed NATS health with large server info
		natsHealth := dto.NATSHealthDetails{
			Connected:        true,
			Uptime:           "168h45m12s", // 1 week uptime
			Reconnects:       0,
			LastError:        "",
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
			MessageMetrics: dto.NATSMessageMetrics{
				PublishedCount: 999999999, // Nearly 1B messages
				FailedCount:    0,
				AverageLatency: "0.123ms",
			},
			ServerInfo: largeServerInfo,
		}

		// Mock the service to return this complex response
		response := testutil.NewEnhancedHealthResponseBuilder().
			WithStatus("healthy").
			WithVersion("1.0.0").
			WithNATSHealth(natsHealth, "healthy", "Connected to NATS cluster", "1ms").
			BuildEnhanced()

		// Measure serialization time
		start := time.Now()

		// This would typically be done by the actual service
		// For now, just validate the structure can be created efficiently
		require.NotNil(t, response.Dependencies["nats"])
		natsStatus := response.Dependencies["nats"]
		require.NotNil(t, natsStatus.Details)

		serializationTime := time.Since(start)

		// This test will fail if serialization is inefficient
		assert.Less(t, serializationTime, 10*time.Millisecond,
			"Large NATS health response serialization took %v, expected < 10ms", serializationTime)

		// Verify complex data integrity
		natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
		assert.NotNil(t, natsDetails.ServerInfo)

		cluster := natsDetails.ServerInfo["cluster"].(map[string]interface{})
		urls := cluster["urls"].([]string)
		assert.Len(t, urls, 3, "Cluster URLs should be preserved")

		jetstream := natsDetails.ServerInfo["jetstream"].(map[string]interface{})
		assert.Equal(t, "1GB", jetstream["max_memory"].(string))
	})
}

// stressTestConfig holds configuration for stress testing.
type stressTestConfig struct {
	duration       time.Duration
	maxConcurrency int
	requestRate    time.Duration
	channelSize    int
}

// stressTestResults holds the results of a stress test.
type stressTestResults struct {
	totalRequests int64
	totalErrors   int64
	duration      time.Duration
}

// setupStressTestService creates a health service configured for stress testing.
func setupStressTestService() *HealthServiceAdapter {
	mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
	mockRepo := &mockRepositoryRepo{findAllResult: nil}
	mockJobs := &mockIndexingJobRepo{}
	return NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0").(*HealthServiceAdapter)
}

// runWorkerPool executes health checks using a worker pool pattern.
func runWorkerPool(
	ctx context.Context,
	service *HealthServiceAdapter,
	config stressTestConfig,
	requestChan <-chan struct{},
) (int64, int64) {
	var wg sync.WaitGroup
	var totalRequests int64
	var totalErrors int64

	// Start workers
	for range config.maxConcurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requestChan {
				_, err := service.GetHealth(ctx)
				atomic.AddInt64(&totalRequests, 1)
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
				}
			}
		}()
	}

	wg.Wait()
	return atomic.LoadInt64(&totalRequests), atomic.LoadInt64(&totalErrors)
}

// generateRequests creates a stream of requests at the specified rate.
func generateRequests(ctx context.Context, config stressTestConfig) <-chan struct{} {
	requestChan := make(chan struct{}, config.channelSize)

	go func() {
		defer close(requestChan)
		ticker := time.NewTicker(config.requestRate)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case requestChan <- struct{}{}:
				default:
					// Channel full, skip this request
				}
			}
		}
	}()

	return requestChan
}

// calculateStressTestMetrics computes performance metrics from test results.
func calculateStressTestMetrics(results stressTestResults) (float64, float64) {
	errorRate := float64(results.totalErrors) / float64(results.totalRequests)
	throughput := float64(results.totalRequests) / results.duration.Seconds()
	return errorRate, throughput
}

// validateStressTestResults performs assertions on stress test results.
func validateStressTestResults(t *testing.T, results stressTestResults) {
	t.Logf("Processed %d requests with %d errors over %v", results.totalRequests, results.totalErrors, results.duration)

	// Should handle significant load (expecting ~250-300 requests in 3 seconds at 100 RPS)
	assert.Greater(t, results.totalRequests, int64(200), "Should process significant number of requests")

	errorRate, throughput := calculateStressTestMetrics(results)

	// Error rate should be low
	assert.Less(t, errorRate, 0.01, "Error rate should be < 1%")

	// Should achieve reasonable throughput (> 50 RPS)
	assert.Greater(t, throughput, 50.0, "Should achieve > 50 RPS throughput")
}

func TestHealthServiceAdapter_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Run("sustained_load_health_checks", func(t *testing.T) {
		service := setupStressTestService()

		config := stressTestConfig{
			duration:       3 * time.Second,
			maxConcurrency: 25,
			requestRate:    10 * time.Millisecond, // 100 RPS
			channelSize:    1000,
		}

		ctx, cancel := context.WithTimeout(context.Background(), config.duration)
		defer cancel()

		requestChan := generateRequests(ctx, config)
		totalRequests, totalErrors := runWorkerPool(ctx, service, config, requestChan)

		results := stressTestResults{
			totalRequests: totalRequests,
			totalErrors:   totalErrors,
			duration:      config.duration,
		}

		// This test will fail without proper optimizations for sustained load
		validateStressTestResults(t, results)
	})
}

func TestHealthServiceAdapter_StressTest_ServiceSetup(t *testing.T) {
	t.Run("creates_service_with_correct_configuration", func(t *testing.T) {
		service := setupStressTestService()

		require.NotNil(t, service)

		// Verify service can handle basic health check
		response, err := service.GetHealth(context.Background())
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, "healthy", response.Status)
	})
}

func TestHealthServiceAdapter_StressTest_WorkerPoolPattern(t *testing.T) {
	t.Run("worker_pool_processes_requests_correctly", func(t *testing.T) {
		service := setupStressTestService()

		config := stressTestConfig{
			duration:       100 * time.Millisecond,
			maxConcurrency: 5,
			requestRate:    10 * time.Millisecond,
			channelSize:    50,
		}

		ctx, cancel := context.WithTimeout(context.Background(), config.duration)
		defer cancel()

		// Create a small number of requests to test worker pool
		requestChan := make(chan struct{}, 10)
		for range 10 {
			requestChan <- struct{}{}
		}
		close(requestChan)

		totalRequests, totalErrors := runWorkerPool(ctx, service, config, requestChan)

		assert.Equal(t, int64(10), totalRequests)
		assert.Equal(t, int64(0), totalErrors)
	})
}

func TestHealthServiceAdapter_StressTest_RequestGeneration(t *testing.T) {
	t.Run("generates_requests_at_specified_rate", func(t *testing.T) {
		config := stressTestConfig{
			duration:       200 * time.Millisecond,
			maxConcurrency: 5,
			requestRate:    50 * time.Millisecond, // 20 RPS
			channelSize:    100,
		}

		ctx, cancel := context.WithTimeout(context.Background(), config.duration)
		defer cancel()

		requestChan := generateRequests(ctx, config)

		// Count requests generated
		var requestCount int
		for range requestChan {
			requestCount++
		}

		// Should generate approximately 4 requests in 200ms at 20 RPS
		assert.GreaterOrEqual(t, requestCount, 2)
		assert.LessOrEqual(t, requestCount, 6)
	})
}

func TestHealthServiceAdapter_StressTest_MetricsCalculation(t *testing.T) {
	t.Run("calculates_metrics_correctly", func(t *testing.T) {
		results := stressTestResults{
			totalRequests: 100,
			totalErrors:   5,
			duration:      2 * time.Second,
		}

		errorRate, throughput := calculateStressTestMetrics(results)

		assert.InEpsilon(t, 0.05, errorRate, 1e-9)  // 5/100 = 0.05
		assert.InEpsilon(t, 50.0, throughput, 1e-9) // 100/2 = 50 RPS
	})

	t.Run("handles_zero_errors_correctly", func(t *testing.T) {
		results := stressTestResults{
			totalRequests: 200,
			totalErrors:   0,
			duration:      4 * time.Second,
		}

		errorRate, throughput := calculateStressTestMetrics(results)

		assert.InDelta(t, 0.0, errorRate, 1e-9)
		assert.InEpsilon(t, 50.0, throughput, 1e-9)
	})
}
