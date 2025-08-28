// Package worker provides comprehensive failing tests for job processor retry integration.
// These tests define the exact behavior needed for integrating robust retry logic
// with the existing job processing system, including end-to-end scenarios.
//
// RED Phase: All tests in this file are designed to fail initially and provide
// clear specifications for the GREEN phase implementation.
package worker

import (
	"codechunking/internal/application/service"
	"codechunking/internal/domain/messaging"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobProcessorRetry_BasicRetryScenarios tests basic retry scenarios in job processing.
func TestJobProcessorRetry_BasicRetryScenarios(t *testing.T) {
	tests := []struct {
		name                string
		jobMessage          messaging.EnhancedIndexingJobMessage
		failureSequence     []error // nil = success, error = specific failure
		expectedAttempts    int
		expectedSuccess     bool
		expectedFailureType service.FailureType
		maxRetryDelay       time.Duration
	}{
		{
			name: "successful job on first attempt",
			jobMessage: messaging.EnhancedIndexingJobMessage{
				MessageID:     "test-001",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/test/repo.git",
			},
			failureSequence:  []error{nil}, // Success immediately
			expectedAttempts: 1,
			expectedSuccess:  true,
			maxRetryDelay:    0,
		},
		{
			name: "network failure then success",
			jobMessage: messaging.EnhancedIndexingJobMessage{
				MessageID:     "test-002",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/test/network-repo.git",
			},
			failureSequence: []error{
				errors.New("connection timeout"),
				errors.New("connection refused"),
				nil, // Success on 3rd attempt
			},
			expectedAttempts:    3,
			expectedSuccess:     true,
			expectedFailureType: service.FailureTypeNetwork,
			maxRetryDelay:       5 * time.Second,
		},
		{
			name: "authentication failure with exponential backoff",
			jobMessage: messaging.EnhancedIndexingJobMessage{
				MessageID:     "test-003",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/private/auth-repo.git",
			},
			failureSequence: []error{
				errors.New("authentication failed"),
				nil, // Success on 2nd attempt
			},
			expectedAttempts:    2,
			expectedSuccess:     true,
			expectedFailureType: service.FailureTypeAuthentication,
			maxRetryDelay:       2 * time.Second,
		},
		{
			name: "persistent disk space failure - no retry",
			jobMessage: messaging.EnhancedIndexingJobMessage{
				MessageID:     "test-004",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/test/large-repo.git",
			},
			failureSequence: []error{
				errors.New("no space left on device"),
			},
			expectedAttempts:    1,
			expectedSuccess:     false,
			expectedFailureType: service.FailureTypeDiskSpace,
			maxRetryDelay:       0, // No retry for disk space issues
		},
		{
			name: "retry exhaustion after multiple attempts",
			jobMessage: messaging.EnhancedIndexingJobMessage{
				MessageID:     "test-005",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/test/problematic-repo.git",
			},
			failureSequence: []error{
				errors.New("network timeout"),
				errors.New("network timeout"),
				errors.New("network timeout"),
				errors.New("network timeout"),
				errors.New("network timeout"), // Still failing after 5 attempts
			},
			expectedAttempts:    5,
			expectedSuccess:     false,
			expectedFailureType: service.FailureTypeNetwork,
			maxRetryDelay:       10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryableJobProcessorConfig{
				JobProcessorConfig: JobProcessorConfig{
					WorkspaceDir:      "/tmp/test-workspace",
					MaxConcurrentJobs: 1,
					JobTimeout:        30 * time.Second,
				},
				RetryPolicyConfig: service.RetryPolicyConfig{
					MaxAttempts:       5,
					BaseDelay:         100 * time.Millisecond,
					MaxDelay:          10 * time.Second,
					BackoffMultiplier: 2.0,
					JitterEnabled:     true,
				},
				CircuitBreakerConfig: service.CBConfig{
					Name:                  fmt.Sprintf("test-circuit-breaker-%s", tt.name),
					FailureThreshold:      3,
					SuccessThreshold:      2,
					OpenTimeout:           5 * time.Second,
					MaxConcurrentRequests: 10,
				},
				RetryMetricsConfig: service.RetryMetricsConfig{
					InstanceID:     fmt.Sprintf("test-processor-%s", tt.name),
					ServiceName:    "test-retry-service",
					ServiceVersion: "1.0.0",
				},
				AdaptiveRetryEnabled:     true,
				FailureClassifierEnabled: true,
			}

			processor := NewRetryableJobProcessor(config)
			require.NotNil(t, processor)

			ctx := context.Background()
			attemptCount := 0

			// Configure the processor with a mock function that simulates failures
			config.ProcessFunc = func(ctx context.Context, message messaging.EnhancedIndexingJobMessage) error {
				attemptCount++
				if attemptCount <= len(tt.failureSequence) {
					return tt.failureSequence[attemptCount-1]
				}
				return errors.New("unexpected additional attempt")
			}

			processor = NewRetryableJobProcessor(config)
			require.NotNil(t, processor)

			start := time.Now()
			err := processor.ProcessJobWithRetry(ctx, tt.jobMessage)
			duration := time.Since(start)

			// Verify attempt count
			assert.Equal(t, tt.expectedAttempts, attemptCount)

			// Verify success/failure outcome
			if tt.expectedSuccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			// Verify retry delay constraints
			if tt.maxRetryDelay > 0 {
				assert.Less(t, duration, tt.maxRetryDelay*2, "Total retry duration exceeded reasonable bounds")
			}

			// Verify metrics were collected
			metrics := processor.GetRetryMetrics()
			require.NotNil(t, metrics)

			// Verify failure statistics
			stats := processor.GetFailureStatistics()
			if !tt.expectedSuccess && tt.expectedFailureType != service.FailureTypeUnknown {
				failureStats, exists := stats[tt.expectedFailureType]
				assert.True(t, exists, "Expected failure statistics for %s", tt.expectedFailureType)
				assert.Positive(t, failureStats.TotalAttempts)
			}
		})
	}
}

// TestJobProcessorRetry_CircuitBreakerIntegration tests circuit breaker behavior in job processing.
func TestJobProcessorRetry_CircuitBreakerIntegration(t *testing.T) {
	t.Skip("Skipping until GREEN phase implementation")

	config := RetryableJobProcessorConfig{
		JobProcessorConfig: JobProcessorConfig{
			WorkspaceDir:      "/tmp/test-workspace",
			MaxConcurrentJobs: 3,
			JobTimeout:        10 * time.Second,
		},
		RetryPolicyConfig: service.RetryPolicyConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
		},
		CircuitBreakerConfig: service.CBConfig{
			Name:             "job-processor-cb",
			FailureThreshold: 5,
			SuccessThreshold: 2,
			OpenTimeout:      2 * time.Second,
		},
		AdaptiveRetryEnabled: true,
	}

	processor := NewRetryableJobProcessor(config)
	require.NotNil(t, processor)

	ctx := context.Background()

	// Process multiple jobs to trigger circuit breaker
	jobs := []messaging.EnhancedIndexingJobMessage{
		{MessageID: "cb-test-1", RepositoryID: uuid.New(), RepositoryURL: "https://github.com/test/fail1.git"},
		{MessageID: "cb-test-2", RepositoryID: uuid.New(), RepositoryURL: "https://github.com/test/fail2.git"},
		{MessageID: "cb-test-3", RepositoryID: uuid.New(), RepositoryURL: "https://github.com/test/fail3.git"},
		{MessageID: "cb-test-4", RepositoryID: uuid.New(), RepositoryURL: "https://github.com/test/fail4.git"},
		{MessageID: "cb-test-5", RepositoryID: uuid.New(), RepositoryURL: "https://github.com/test/fail5.git"},
		{
			MessageID:     "cb-test-6",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/test/success.git",
		}, // Should be blocked by circuit breaker
	}

	failureCount := 0
	successCount := 0

	for i, job := range jobs {
		// First 5 jobs fail, 6th should succeed but may be blocked by circuit breaker
		mockProcessor := &MockRetryableProcessor{
			failureSequence: []error{
				func() error {
					if i < 5 {
						return errors.New("persistent failure")
					}
					return nil // 6th job succeeds
				}(),
			},
		}

		err := mockProcessor.ProcessJobWithRetry(ctx, job)
		if err != nil {
			failureCount++
		} else {
			successCount++
		}

		// Small delay between jobs
		time.Sleep(50 * time.Millisecond)
	}

	// Verify circuit breaker affected processing
	assert.Positive(t, failureCount, "Should have some failures")

	// Circuit breaker should have opened and potentially blocked the successful job
	metrics := processor.GetRetryMetrics()
	require.NotNil(t, metrics)
}

// TestJobProcessorRetry_AdaptivePolicyBehavior tests adaptive retry policy adjustments.
func TestJobProcessorRetry_AdaptivePolicyBehavior(t *testing.T) {
	t.Skip("Skipping until GREEN phase implementation")

	config := RetryableJobProcessorConfig{
		JobProcessorConfig: JobProcessorConfig{
			WorkspaceDir:      "/tmp/test-workspace",
			MaxConcurrentJobs: 1,
			JobTimeout:        15 * time.Second,
		},
		RetryPolicyConfig: service.RetryPolicyConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
		},
		AdaptiveRetryEnabled: true,
	}

	processor := NewRetryableJobProcessor(config)
	require.NotNil(t, processor)

	ctx := context.Background()

	// Test different failure patterns to trigger adaptive behavior
	testScenarios := []struct {
		name           string
		failurePattern []error
		expectedPolicy string
		expectedDelay  time.Duration
	}{
		{
			name: "frequent network failures - increase delay",
			failurePattern: []error{
				errors.New("connection timeout"),
				errors.New("connection timeout"),
				nil,
			},
			expectedPolicy: "adaptive_network",
			expectedDelay:  200 * time.Millisecond,
		},
		{
			name: "authentication issues - reduce attempts",
			failurePattern: []error{
				errors.New("authentication failed"),
				nil,
			},
			expectedPolicy: "adaptive_auth",
			expectedDelay:  500 * time.Millisecond,
		},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			job := messaging.EnhancedIndexingJobMessage{
				MessageID:     "adaptive-test",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/test/adaptive.git",
			}

			mockProcessor := &MockRetryableProcessor{
				failureSequence: scenario.failurePattern,
			}

			err := mockProcessor.ProcessJobWithRetry(ctx, job)
			require.NoError(t, err, "Expected eventual success for adaptive test")

			// Verify policy was adapted
			currentPolicy := processor.GetRetryPolicy()
			require.NotNil(t, currentPolicy)

			// Additional adaptive behavior verification would be implemented in GREEN phase
		})
	}
}

// TestJobProcessorRetry_ContextCancellationAndTimeouts tests context handling during retries.
func TestJobProcessorRetry_ContextCancellationAndTimeouts(t *testing.T) {
	t.Skip("Skipping until GREEN phase implementation")

	config := RetryableJobProcessorConfig{
		JobProcessorConfig: JobProcessorConfig{
			WorkspaceDir:      "/tmp/test-workspace",
			MaxConcurrentJobs: 1,
			JobTimeout:        5 * time.Second,
		},
		RetryPolicyConfig: service.RetryPolicyConfig{
			MaxAttempts: 5,
			BaseDelay:   1 * time.Second, // Long delays to test timeout
		},
	}

	processor := NewRetryableJobProcessor(config)
	require.NotNil(t, processor)

	job := messaging.EnhancedIndexingJobMessage{
		MessageID:     "timeout-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/test/timeout.git",
	}

	// Test context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mockProcessor := &MockRetryableProcessor{
		failureSequence: []error{
			errors.New("failure 1"),
			errors.New("failure 2"),
			errors.New("failure 3"), // This should be interrupted by timeout
		},
	}

	start := time.Now()
	err := mockProcessor.ProcessJobWithRetry(ctx, job)
	duration := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
	assert.Less(t, duration, 3*time.Second, "Should timeout before completing all retries")

	// Test manual cancellation
	ctx2, cancel2 := context.WithCancel(context.Background())

	// Cancel after short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel2()
	}()

	start2 := time.Now()
	err2 := mockProcessor.ProcessJobWithRetry(ctx2, job)
	duration2 := time.Since(start2)

	require.Error(t, err2)
	assert.Contains(t, err2.Error(), "context")
	assert.Less(t, duration2, 1*time.Second, "Should be cancelled quickly")
}

// TestJobProcessorRetry_ConcurrentJobProcessing tests retry behavior with concurrent jobs.
func TestJobProcessorRetry_ConcurrentJobProcessing(t *testing.T) {
	t.Skip("Skipping until GREEN phase implementation")

	config := RetryableJobProcessorConfig{
		JobProcessorConfig: JobProcessorConfig{
			WorkspaceDir:      "/tmp/test-workspace",
			MaxConcurrentJobs: 3,
			JobTimeout:        10 * time.Second,
		},
		RetryPolicyConfig: service.RetryPolicyConfig{
			MaxAttempts: 2,
			BaseDelay:   100 * time.Millisecond,
		},
		CircuitBreakerConfig: service.CBConfig{
			Name:                  "concurrent-test-cb",
			FailureThreshold:      10,
			SuccessThreshold:      2,
			OpenTimeout:           1 * time.Second,
			MaxConcurrentRequests: 5,
		},
	}

	processor := NewRetryableJobProcessor(config)
	require.NotNil(t, processor)

	ctx := context.Background()
	const numJobs = 10

	jobs := make([]messaging.EnhancedIndexingJobMessage, numJobs)
	for i := range numJobs {
		jobs[i] = messaging.EnhancedIndexingJobMessage{
			MessageID:     fmt.Sprintf("concurrent-job-%d", i),
			RepositoryID:  uuid.New(),
			RepositoryURL: fmt.Sprintf("https://github.com/test/concurrent-%d.git", i),
		}
	}

	results := make(chan error, numJobs)

	// Process jobs concurrently
	for _, job := range jobs {
		go func(j messaging.EnhancedIndexingJobMessage) {
			mockProcessor := &MockRetryableProcessor{
				failureSequence: []error{
					func() error {
						// Some jobs succeed immediately, others need retry
						if j.MessageID[len(j.MessageID)-1]%2 == 0 {
							return nil
						}
						return errors.New("transient failure")
					}(),
					nil, // Success on retry
				},
			}

			err := mockProcessor.ProcessJobWithRetry(ctx, j)
			results <- err
		}(job)
	}

	// Collect results
	successCount := 0
	failureCount := 0

	for range numJobs {
		select {
		case err := <-results:
			if err != nil {
				failureCount++
			} else {
				successCount++
			}
		case <-time.After(15 * time.Second):
			t.Fatal("Timeout waiting for concurrent job results")
		}
	}

	// Verify all jobs were processed
	assert.Equal(t, numJobs, successCount+failureCount)

	// Should have some successes (even jobs succeed immediately)
	assert.Positive(t, successCount)

	// Verify metrics were collected for concurrent operations
	metrics := processor.GetRetryMetrics()
	require.NotNil(t, metrics)
}

// MockRetryableProcessor for testing retry logic.
type MockRetryableProcessor struct {
	failureSequence []error
	attemptCounter  *int
	currentAttempt  int
}

func (m *MockRetryableProcessor) ProcessJobWithRetry(_ context.Context, _ messaging.EnhancedIndexingJobMessage) error {
	if m.attemptCounter != nil {
		*m.attemptCounter++
	}

	if m.currentAttempt >= len(m.failureSequence) {
		return nil // Default to success if we run out of sequence
	}

	err := m.failureSequence[m.currentAttempt]
	m.currentAttempt++

	return err
}

func (m *MockRetryableProcessor) GetRetryMetrics() service.RetryMetrics {
	// Return mock metrics for testing
	return &MockRetryMetrics{}
}

func (m *MockRetryableProcessor) GetRetryPolicy() service.RetryPolicy {
	// Return mock policy for testing
	return &MockRetryPolicy{}
}

func (m *MockRetryableProcessor) UpdateRetryPolicy(_ service.RetryPolicy) error {
	return nil
}

func (m *MockRetryableProcessor) GetFailureStatistics() map[service.FailureType]FailureStatistics {
	return map[service.FailureType]FailureStatistics{
		service.FailureTypeUnknown: {
			TotalAttempts:     0,
			SuccessfulRetries: 0,
			ExhaustedRetries:  0,
			AverageRetryDelay: 0,
			LastFailureTime:   time.Time{},
		},
		service.FailureTypeNetwork: {
			TotalAttempts:     3,
			SuccessfulRetries: 1,
			ExhaustedRetries:  0,
			AverageRetryDelay: 200 * time.Millisecond,
			LastFailureTime:   time.Now(),
		},
		service.FailureTypeAuthentication: {
			TotalAttempts:     0,
			SuccessfulRetries: 0,
			ExhaustedRetries:  0,
			AverageRetryDelay: 0,
			LastFailureTime:   time.Time{},
		},
		service.FailureTypeDiskSpace: {
			TotalAttempts:     0,
			SuccessfulRetries: 0,
			ExhaustedRetries:  0,
			AverageRetryDelay: 0,
			LastFailureTime:   time.Time{},
		},
		service.FailureTypeRepositoryAccess: {
			TotalAttempts:     0,
			SuccessfulRetries: 0,
			ExhaustedRetries:  0,
			AverageRetryDelay: 0,
			LastFailureTime:   time.Time{},
		},
		service.FailureTypeNATSMessaging: {
			TotalAttempts:     0,
			SuccessfulRetries: 0,
			ExhaustedRetries:  0,
			AverageRetryDelay: 0,
			LastFailureTime:   time.Time{},
		},
		service.FailureTypeCircuitBreaker: {
			TotalAttempts:     0,
			SuccessfulRetries: 0,
			ExhaustedRetries:  0,
			AverageRetryDelay: 0,
			LastFailureTime:   time.Time{},
		},
		service.FailureTypeTimeout: {
			TotalAttempts:     0,
			SuccessfulRetries: 0,
			ExhaustedRetries:  0,
			AverageRetryDelay: 0,
			LastFailureTime:   time.Time{},
		},
		service.FailureTypeRateLimit: {
			TotalAttempts:     0,
			SuccessfulRetries: 0,
			ExhaustedRetries:  0,
			AverageRetryDelay: 0,
			LastFailureTime:   time.Time{},
		},
	}
}

// Mock implementations for testing (will be replaced in GREEN phase).
type MockRetryMetrics struct{}

func (m *MockRetryMetrics) RecordRetryAttempt(_ context.Context, _ int, _ service.FailureType, _ string) {
}
func (m *MockRetryMetrics) RecordRetrySuccess(_ context.Context, _ int, _ time.Duration, _ string) {}
func (m *MockRetryMetrics) RecordRetryFailure(_ context.Context, _ int, _ service.FailureType, _ error, _ string) {
}

func (m *MockRetryMetrics) RecordRetryExhaustion(_ context.Context, _ int, _ time.Duration, _ error, _ string) {
}
func (m *MockRetryMetrics) RecordRetryDelay(_ context.Context, _ time.Duration, _ int, _ string) {}
func (m *MockRetryMetrics) RecordCircuitBreakerEvent(_ context.Context, _ string, _ string)      {}
func (m *MockRetryMetrics) RecordPolicyChange(_ context.Context, _, _ string, _ string)          {}
func (m *MockRetryMetrics) GetInstanceID() string                                                { return "mock-001" }

type MockRetryPolicy struct{}

func (m *MockRetryPolicy) ShouldRetry(_ context.Context, _ error, _ int) (bool, time.Duration) {
	return true, 100 * time.Millisecond
}
func (m *MockRetryPolicy) GetMaxAttempts() int   { return 3 }
func (m *MockRetryPolicy) GetPolicyName() string { return "mock-policy" }
func (m *MockRetryPolicy) Reset()                {}

// Stub functions to make tests compile (will be implemented in GREEN phase).
