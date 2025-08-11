package logging

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codechunking/internal/adapter/inbound/api/middleware"
)

// TestCorrelationIntegration_HTTPToServiceLayer tests correlation ID propagation from HTTP to service layer
func TestCorrelationIntegration_HTTPToServiceLayer(t *testing.T) {
	tests := []struct {
		name                  string
		incomingCorrelationID string
		expectSameID          bool
		expectNewID           bool
	}{
		{
			name:                  "existing correlation ID propagated through layers",
			incomingCorrelationID: "http-correlation-123",
			expectSameID:          true,
			expectNewID:           false,
		},
		{
			name:                  "generate new correlation ID when none provided",
			incomingCorrelationID: "",
			expectSameID:          false,
			expectNewID:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create application logger
			loggerConfig := Config{
				Level:  "INFO",
				Format: "json",
				Output: "buffer",
			}
			appLogger, err := NewApplicationLogger(loggerConfig)
			require.NoError(t, err)

			// Create service that uses the logger
			repositoryService := NewMockRepositoryService(appLogger)

			// Create HTTP handler that calls service
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()

				// Extract correlation ID from HTTP middleware
				correlationID := middleware.GetCorrelationIDFromContext(ctx)
				assert.NotEmpty(t, correlationID, "Correlation ID should be available in handler")

				// Call service layer with context
				err := repositoryService.CreateRepository(ctx, RepositoryRequest{
					URL: "https://github.com/user/repo",
				})
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusCreated)
				if _, err := w.Write([]byte(`{"id": "repo-123", "status": "created"}`)); err != nil {
					t.Logf("Failed to write response: %v", err)
				}
			})

			// Wrap handler with logging middleware
			middlewareConfig := middleware.LoggingConfig{
				LogLevel: "INFO",
			}
			loggingMiddleware := middleware.NewStructuredLoggingMiddleware(middlewareConfig)
			wrappedHandler := loggingMiddleware(handler)

			// Create request with or without correlation ID
			req := httptest.NewRequest("POST", "/repositories", strings.NewReader(`{"url": "https://github.com/user/repo"}`))
			if tt.incomingCorrelationID != "" {
				req.Header.Set("X-Correlation-ID", tt.incomingCorrelationID)
			}

			recorder := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(recorder, req)

			// Verify response correlation ID
			responseCorrelationID := recorder.Header().Get("X-Correlation-ID")
			assert.NotEmpty(t, responseCorrelationID)

			if tt.expectSameID {
				assert.Equal(t, tt.incomingCorrelationID, responseCorrelationID)
			}
			if tt.expectNewID {
				assert.True(t, isValidUUID(responseCorrelationID))
			}

			// Verify all log entries share the same correlation ID
			httpLogOutput := middleware.GetMiddlewareLogOutput()
			serviceLogOutput := getServiceLogOutput(repositoryService)

			// HTTP middleware uses HTTPLogEntry (Duration as float64)
			// Application service uses LogEntry (Duration as string)
			// We need to handle these different structures
			var httpLogEntry middleware.HTTPLogEntry
			var serviceLogEntry LogEntry

			err = json.Unmarshal([]byte(httpLogOutput), &httpLogEntry)
			require.NoError(t, err)
			err = json.Unmarshal([]byte(serviceLogOutput), &serviceLogEntry)
			require.NoError(t, err)

			// Both logs should have the same correlation ID
			assert.Equal(t, responseCorrelationID, httpLogEntry.CorrelationID)
			assert.Equal(t, responseCorrelationID, serviceLogEntry.CorrelationID)
		})
	}
}

// TestCorrelationIntegration_ServiceToNATS tests correlation ID propagation from service layer to NATS operations
func TestCorrelationIntegration_ServiceToNATS(t *testing.T) {
	// Create application logger
	loggerConfig := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}
	appLogger, err := NewApplicationLogger(loggerConfig)
	require.NoError(t, err)

	correlationID := "service-to-nats-456"
	ctx := WithCorrelationID(context.Background(), correlationID)

	// Create service that publishes to NATS
	indexingService := NewMockIndexingService(appLogger)

	// Create NATS publisher that logs operations
	natsPublisher := NewMockNATSPublisher(appLogger)

	// Simulate service workflow that publishes to NATS
	err = indexingService.ProcessRepository(ctx, RepositoryProcessingRequest{
		RepositoryID: "repo-123",
		URL:          "https://github.com/user/repo",
		Publisher:    natsPublisher,
	})
	require.NoError(t, err)

	// The key test here is that NATS receives the correlation ID from the service layer
	natsLogOutput := getNATSLogOutput(natsPublisher)
	require.NotEmpty(t, natsLogOutput, "NATS publisher should have log output")

	var natsLogEntry LogEntry
	err = json.Unmarshal([]byte(natsLogOutput), &natsLogEntry)
	require.NoError(t, err)

	// The NATS publisher should receive and use the same correlation ID
	assert.Equal(t, correlationID, natsLogEntry.CorrelationID)

	// Verify the repository ID was passed through correctly
	assert.Equal(t, "repo-123", natsLogEntry.Metadata["repository_id"])
}

// TestCorrelationIntegration_EndToEndWorkflow tests full end-to-end correlation across all components
func TestCorrelationIntegration_EndToEndWorkflow(t *testing.T) {
	// Create application logger
	loggerConfig := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}
	appLogger, err := NewApplicationLogger(loggerConfig)
	require.NoError(t, err)

	// Create all components
	repositoryService := NewMockRepositoryService(appLogger)
	indexingService := NewMockIndexingService(appLogger)
	natsPublisher := NewMockNATSPublisher(appLogger)
	natsConsumer := NewMockNATSConsumer(appLogger)

	// Create HTTP handler that triggers full workflow
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		correlationID := middleware.GetCorrelationIDFromContext(ctx)

		// Step 1: Create repository (HTTP → Service)
		repoResult, err := repositoryService.CreateRepositoryWithResult(ctx, RepositoryRequest{
			URL: "https://github.com/user/repo",
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Step 2: Publish indexing job (Service → NATS)
		err = indexingService.PublishIndexingJob(ctx, IndexingJobRequest{
			RepositoryID: repoResult.ID,
			URL:          repoResult.URL,
			Publisher:    natsPublisher,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Step 3: Simulate job consumption (NATS → Consumer)
		jobMessage := NATSJobMessage{
			CorrelationID: correlationID,
			RepositoryID:  repoResult.ID,
			URL:           repoResult.URL,
		}
		err = natsConsumer.ProcessIndexingJob(ctx, jobMessage)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status": "workflow_completed", "repository_id": "` + repoResult.ID + `"}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Wrap with logging middleware
	middlewareConfig := middleware.LoggingConfig{
		LogLevel: "INFO",
	}
	loggingMiddleware := middleware.NewStructuredLoggingMiddleware(middlewareConfig)
	wrappedHandler := loggingMiddleware(handler)

	// Execute end-to-end request
	req := httptest.NewRequest("POST", "/repositories", strings.NewReader(`{"url": "https://github.com/user/repo"}`))
	recorder := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(recorder, req)

	// Verify response
	assert.Equal(t, http.StatusOK, recorder.Code)
	correlationID := recorder.Header().Get("X-Correlation-ID")
	assert.NotEmpty(t, correlationID)

	// Collect all log outputs
	httpLogOutput := middleware.GetMiddlewareLogOutput()
	serviceLogOutput := getServiceLogOutput(repositoryService)
	indexingLogOutput := getServiceLogOutput(indexingService)
	publisherLogOutput := getNATSLogOutput(natsPublisher)
	consumerLogOutput := getNATSLogOutput(natsConsumer)

	// Parse all log entries - HTTP log uses different structure than service logs
	var httpLog middleware.HTTPLogEntry
	var serviceLog, indexingLog, publisherLog, consumerLog LogEntry

	err = json.Unmarshal([]byte(httpLogOutput), &httpLog)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(serviceLogOutput), &serviceLog)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(indexingLogOutput), &indexingLog)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(publisherLogOutput), &publisherLog)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(consumerLogOutput), &consumerLog)
	require.NoError(t, err)

	// Verify all components share the same correlation ID
	assert.Equal(t, correlationID, httpLog.CorrelationID, "HTTP log should have matching correlation ID")
	allServiceLogs := []LogEntry{serviceLog, indexingLog, publisherLog, consumerLog}
	for i, logEntry := range allServiceLogs {
		assert.Equal(t, correlationID, logEntry.CorrelationID,
			"Service log entry %d should have matching correlation ID", i)
	}

	// Verify workflow progression can be traced
	assert.Equal(t, "http_request", httpLog.Operation)
	assert.Equal(t, "create_repository", serviceLog.Operation)
	assert.Equal(t, "publish_indexing_job", indexingLog.Operation)
	assert.Equal(t, "nats_publish", publisherLog.Operation)
	assert.Equal(t, "nats_consume", consumerLog.Operation)

	// Verify repository ID is propagated through all operations
	repoID := serviceLog.Metadata["repository_id"].(string)
	assert.NotEmpty(t, repoID)
	assert.Equal(t, repoID, indexingLog.Metadata["repository_id"])
	assert.Equal(t, repoID, publisherLog.Metadata["repository_id"])
	assert.Equal(t, repoID, consumerLog.Metadata["repository_id"])
}

// TestCorrelationIntegration_ErrorPropagation tests correlation ID propagation during error scenarios
func TestCorrelationIntegration_ErrorPropagation(t *testing.T) {
	// Create application logger
	loggerConfig := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}
	appLogger, err := NewApplicationLogger(loggerConfig)
	require.NoError(t, err)

	correlationID := "error-propagation-789"
	ctx := WithCorrelationID(context.Background(), correlationID)

	tests := []struct {
		name          string
		simulateError ErrorScenario
		expectedLogs  int
		expectErrors  bool
	}{
		{
			name: "service layer error propagation",
			simulateError: ErrorScenario{
				Component:    "service",
				ErrorType:    "validation_error",
				ErrorMessage: "Invalid repository URL format",
			},
			expectedLogs: 2, // Service error log + HTTP error log
			expectErrors: true,
		},
		{
			name: "NATS publisher error propagation",
			simulateError: ErrorScenario{
				Component:    "nats_publisher",
				ErrorType:    "connection_error",
				ErrorMessage: "Unable to connect to NATS server",
			},
			expectedLogs: 3, // Service log + NATS error log + HTTP error log
			expectErrors: true,
		},
		{
			name: "NATS consumer error propagation",
			simulateError: ErrorScenario{
				Component:    "nats_consumer",
				ErrorType:    "processing_error",
				ErrorMessage: "Failed to process repository indexing job",
			},
			expectedLogs: 1, // Consumer error log only (async processing)
			expectErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create error-prone components
			errorService := NewMockErrorService(appLogger, tt.simulateError)

			switch tt.simulateError.Component {
			case "service":
				// Test service layer error
				err := errorService.ProcessRepository(ctx, RepositoryProcessingRequest{
					RepositoryID: "repo-error",
					URL:          "invalid-url",
				})
				assert.Error(t, err)

			case "nats_publisher":
				// Test NATS publisher error
				errorPublisher := NewMockErrorNATSPublisher(appLogger, tt.simulateError)
				err := errorService.PublishWithErroryPublisher(ctx, RepositoryRequest{
					URL: "https://github.com/user/repo",
				}, errorPublisher)
				assert.Error(t, err)

			case "nats_consumer":
				// Test NATS consumer error (async)
				errorConsumer := NewMockErrorNATSConsumer(appLogger, tt.simulateError)
				jobMessage := NATSJobMessage{
					CorrelationID: correlationID,
					RepositoryID:  "repo-123",
					URL:           "https://github.com/user/repo",
				}
				err := errorConsumer.ProcessIndexingJob(ctx, jobMessage)
				assert.Error(t, err)
			}

			// Verify all error logs have the same correlation ID
			allLogOutputs := getAllErrorLogOutputs(tt.simulateError.Component)
			assert.Len(t, allLogOutputs, tt.expectedLogs, "Expected number of error logs")

			for i, logOutput := range allLogOutputs {
				var logEntry LogEntry
				err := json.Unmarshal([]byte(logOutput), &logEntry)
				require.NoError(t, err, "Log entry %d should be valid JSON", i)

				assert.Equal(t, correlationID, logEntry.CorrelationID,
					"Error log %d should have matching correlation ID", i)

				if tt.expectErrors {
					assert.Equal(t, "ERROR", logEntry.Level, "Error logs should have ERROR level")
					assert.Contains(t, logEntry.Error, tt.simulateError.ErrorMessage,
						"Error message should be present in log")
				}
			}
		})
	}
}

// TestCorrelationIntegration_ConcurrentRequests tests correlation ID isolation across concurrent requests
func TestCorrelationIntegration_ConcurrentRequests(t *testing.T) {
	// Create application logger
	loggerConfig := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}
	appLogger, err := NewApplicationLogger(loggerConfig)
	require.NoError(t, err)

	// Create service
	repositoryService := NewMockRepositoryService(appLogger)

	// Create HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		correlationID := middleware.GetCorrelationIDFromContext(ctx)

		// Simulate some processing time
		time.Sleep(time.Millisecond * 10)

		err := repositoryService.CreateRepository(ctx, RepositoryRequest{
			URL: "https://github.com/user/repo",
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"correlation_id": "` + correlationID + `"}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Wrap with logging middleware
	middlewareConfig := middleware.LoggingConfig{
		LogLevel: "INFO",
	}
	loggingMiddleware := middleware.NewStructuredLoggingMiddleware(middlewareConfig)
	wrappedHandler := loggingMiddleware(handler)

	// Execute concurrent requests with known correlation IDs
	const numRequests = 10
	correlationIDs := make([]string, numRequests)
	responses := make([]*httptest.ResponseRecorder, numRequests)

	// Generate known correlation IDs
	for i := 0; i < numRequests; i++ {
		correlationIDs[i] = "concurrent-test-" + string(rune('A'+i))
	}

	// Execute requests concurrently
	done := make(chan bool, numRequests)
	for i := 0; i < numRequests; i++ {
		go func(index int) {
			req := httptest.NewRequest("POST", "/repositories", strings.NewReader(`{"url": "https://github.com/user/repo"}`))
			req.Header.Set("X-Correlation-ID", correlationIDs[index])

			recorder := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(recorder, req)
			responses[index] = recorder
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Verify each request maintained its correlation ID
	for i := 0; i < numRequests; i++ {
		assert.Equal(t, http.StatusCreated, responses[i].Code)
		responseCorrelationID := responses[i].Header().Get("X-Correlation-ID")
		assert.Equal(t, correlationIDs[i], responseCorrelationID,
			"Request %d should maintain its correlation ID", i)
	}

	// Verify log isolation
	allLogOutputs := getAllConcurrentLogOutputs()
	correlationIDCounts := make(map[string]int)

	for _, logOutput := range allLogOutputs {
		var logEntry LogEntry
		err := json.Unmarshal([]byte(logOutput), &logEntry)
		require.NoError(t, err)

		correlationIDCounts[logEntry.CorrelationID]++
	}

	// Each correlation ID should appear in logs (HTTP + Service logs = 2 per request)
	for _, correlationID := range correlationIDs {
		count, exists := correlationIDCounts[correlationID]
		assert.True(t, exists, "Correlation ID %s should appear in logs", correlationID)
		assert.Equal(t, 2, count, "Each correlation ID should appear exactly twice (HTTP + Service logs)")
	}
}

// TestCorrelationIntegration_AsyncProcessing tests correlation ID handling in async processing scenarios
func TestCorrelationIntegration_AsyncProcessing(t *testing.T) {
	// Create application logger
	loggerConfig := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}
	appLogger, err := NewApplicationLogger(loggerConfig)
	require.NoError(t, err)

	correlationID := "async-processing-999"
	ctx := WithCorrelationID(context.Background(), correlationID)

	// Create async processing service
	asyncService := NewMockAsyncProcessingService(appLogger)

	// Start async processing job
	jobID, err := asyncService.StartAsyncJob(ctx, AsyncJobRequest{
		Type:         "repository_indexing",
		RepositoryID: "repo-123",
		Priority:     "high",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, jobID)

	// Simulate job processing in background
	jobResult, err := asyncService.ProcessJobAsync(ctx, jobID)
	require.NoError(t, err)

	// Simulate job completion callback
	err = asyncService.CompleteJob(ctx, jobResult)
	require.NoError(t, err)

	// Verify all async operations maintained correlation ID
	asyncLogOutputs := getAsyncServiceLogOutputs(asyncService)
	assert.GreaterOrEqual(t, len(asyncLogOutputs), 3, "Expected logs for start, process, and complete operations")

	for i, logOutput := range asyncLogOutputs {
		var logEntry LogEntry
		err := json.Unmarshal([]byte(logOutput), &logEntry)
		require.NoError(t, err, "Async log entry %d should be valid JSON", i)

		assert.Equal(t, correlationID, logEntry.CorrelationID,
			"Async operation %d should maintain correlation ID", i)

		// Verify job ID is present for tracing
		assert.Equal(t, jobID, logEntry.Metadata["job_id"],
			"Job ID should be present for tracing async operations")
	}

	// Verify operation sequence can be traced
	operations := make([]string, len(asyncLogOutputs))
	for i, logOutput := range asyncLogOutputs {
		var logEntry LogEntry
		if err := json.Unmarshal([]byte(logOutput), &logEntry); err != nil {
			t.Errorf("Failed to unmarshal log entry: %v", err)
		}
		operations[i] = logEntry.Operation
	}

	expectedOperations := []string{"async_job_start", "async_job_process", "async_job_complete"}
	for i, expectedOp := range expectedOperations {
		assert.Equal(t, expectedOp, operations[i],
			"Async operation %d should match expected sequence", i)
	}
}

// Test data structures and implementations are now in correlation.go

// Data structures and interfaces are now defined in correlation.go
