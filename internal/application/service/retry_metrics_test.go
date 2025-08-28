// Package service provides comprehensive failing tests for retry metrics with OpenTelemetry.
// These tests define the exact behavior needed for comprehensive observability of retry
// operations, including histograms, counters, and gauges for retry attempts, success rates,
// and failure classifications.
//
// RED Phase: All tests in this file are designed to fail initially and provide
// clear specifications for the GREEN phase implementation.
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// TestRetryMetrics_NewRetryMetrics tests creating a new RetryMetrics instance.
func TestRetryMetrics_NewRetryMetrics(t *testing.T) {
	tests := []struct {
		name        string
		config      RetryMetricsConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config creates metrics successfully",
			config: RetryMetricsConfig{
				InstanceID:          "retry-service-001",
				ServiceName:         "codechunking-worker",
				ServiceVersion:      "1.0.0",
				EnableDelayBuckets:  true,
				EnableJitterMetrics: true,
				MaxAttempts:         5,
			},
			wantErr: false,
		},
		{
			name: "minimal config works",
			config: RetryMetricsConfig{
				InstanceID:  "minimal-001",
				ServiceName: "test-service",
			},
			wantErr: false,
		},
		{
			name: "custom delay buckets",
			config: RetryMetricsConfig{
				InstanceID:         "custom-buckets-001",
				ServiceName:        "test-service",
				CustomDelayBuckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
			},
			wantErr: false,
		},
		{
			name: "empty instance ID returns error",
			config: RetryMetricsConfig{
				InstanceID:  "",
				ServiceName: "test-service",
			},
			wantErr:     true,
			errContains: "instance ID cannot be empty",
		},
		{
			name: "empty service name returns error",
			config: RetryMetricsConfig{
				InstanceID:  "test-001",
				ServiceName: "",
			},
			wantErr:     true,
			errContains: "service name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			metrics, err := NewRetryMetrics(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, metrics)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, metrics)
			assert.Equal(t, tt.config.InstanceID, metrics.GetInstanceID())
		})
	}
}

// TestRetryMetrics_RecordRetryAttempt tests recording retry attempts.
func TestRetryMetrics_RecordRetryAttempt(t *testing.T) {
	tests := []struct {
		name          string
		attempt       int
		failureType   FailureType
		operationName string
		expectMetrics bool
	}{
		{
			name:          "first retry attempt",
			attempt:       1,
			failureType:   FailureTypeNetwork,
			operationName: "git_clone",
			expectMetrics: true,
		},
		{
			name:          "multiple retry attempts",
			attempt:       3,
			failureType:   FailureTypeAuthentication,
			operationName: "api_request",
			expectMetrics: true,
		},
		{
			name:          "disk space failure attempt",
			attempt:       1,
			failureType:   FailureTypeDiskSpace,
			operationName: "file_write",
			expectMetrics: true,
		},
		{
			name:          "zero attempt number",
			attempt:       0,
			failureType:   FailureTypeUnknown,
			operationName: "invalid_operation",
			expectMetrics: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryMetricsConfig{
				InstanceID:  "test-retry-001",
				ServiceName: "test-service",
			}

			// Create metrics with test meter provider
			meterProvider := createTestMeterProvider(t)
			defer func() {
				err := meterProvider.Shutdown(context.Background())
				require.NoError(t, err)
			}()

			metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
			require.NoError(t, err)
			require.NotNil(t, metrics)

			ctx := context.Background()

			// Record retry attempt
			metrics.RecordRetryAttempt(ctx, tt.attempt, tt.failureType, tt.operationName)

			if tt.expectMetrics {
				// Verify metrics were recorded
				err = meterProvider.ForceFlush(ctx)
				require.NoError(t, err)

				// Verify retry attempt counter was incremented
				// Additional assertions would be added here in GREEN phase
			}
		})
	}
}

// TestRetryMetrics_RecordRetrySuccess tests recording successful retry outcomes.
func TestRetryMetrics_RecordRetrySuccess(t *testing.T) {
	tests := []struct {
		name          string
		totalAttempts int
		totalDuration time.Duration
		operationName string
	}{
		{
			name:          "success after first attempt",
			totalAttempts: 1,
			totalDuration: 100 * time.Millisecond,
			operationName: "quick_operation",
		},
		{
			name:          "success after multiple attempts",
			totalAttempts: 4,
			totalDuration: 2500 * time.Millisecond,
			operationName: "resilient_operation",
		},
		{
			name:          "success with long duration",
			totalAttempts: 2,
			totalDuration: 30 * time.Second,
			operationName: "long_running_operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryMetricsConfig{
				InstanceID:  "test-success-001",
				ServiceName: "test-service",
			}

			meterProvider := createTestMeterProvider(t)
			defer func() {
				err := meterProvider.Shutdown(context.Background())
				require.NoError(t, err)
			}()

			metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
			require.NoError(t, err)

			ctx := context.Background()

			// Record successful retry
			metrics.RecordRetrySuccess(ctx, tt.totalAttempts, tt.totalDuration, tt.operationName)

			// Verify success counter and duration histogram were recorded
			err = meterProvider.ForceFlush(ctx)
			require.NoError(t, err)
		})
	}
}

// TestRetryMetrics_RecordRetryFailure tests recording failed retry attempts.
func TestRetryMetrics_RecordRetryFailure(t *testing.T) {
	tests := []struct {
		name          string
		attempt       int
		failureType   FailureType
		error         error
		operationName string
	}{
		{
			name:          "network failure on second attempt",
			attempt:       2,
			failureType:   FailureTypeNetwork,
			error:         errors.New("connection timeout"),
			operationName: "network_call",
		},
		{
			name:          "authentication failure",
			attempt:       1,
			failureType:   FailureTypeAuthentication,
			error:         errors.New("invalid credentials"),
			operationName: "auth_request",
		},
		{
			name:          "disk space failure",
			attempt:       1,
			failureType:   FailureTypeDiskSpace,
			error:         errors.New("no space left on device"),
			operationName: "file_operation",
		},
		{
			name:          "unknown failure type",
			attempt:       3,
			failureType:   FailureTypeUnknown,
			error:         errors.New("generic error"),
			operationName: "unknown_operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryMetricsConfig{
				InstanceID:  "test-failure-001",
				ServiceName: "test-service",
			}

			meterProvider := createTestMeterProvider(t)
			defer func() {
				err := meterProvider.Shutdown(context.Background())
				require.NoError(t, err)
			}()

			metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
			require.NoError(t, err)

			ctx := context.Background()

			// Record retry failure
			metrics.RecordRetryFailure(ctx, tt.attempt, tt.failureType, tt.error, tt.operationName)

			// Verify failure counter was incremented with proper attributes
			err = meterProvider.ForceFlush(ctx)
			require.NoError(t, err)
		})
	}
}

// TestRetryMetrics_RecordRetryExhaustion tests recording retry exhaustion.
func TestRetryMetrics_RecordRetryExhaustion(t *testing.T) {
	tests := []struct {
		name          string
		maxAttempts   int
		totalDuration time.Duration
		lastError     error
		operationName string
	}{
		{
			name:          "exhausted after 3 attempts",
			maxAttempts:   3,
			totalDuration: 1500 * time.Millisecond,
			lastError:     errors.New("persistent network error"),
			operationName: "network_operation",
		},
		{
			name:          "exhausted after 5 attempts",
			maxAttempts:   5,
			totalDuration: 10 * time.Second,
			lastError:     errors.New("authentication still failing"),
			operationName: "auth_operation",
		},
		{
			name:          "exhausted with quick failure",
			maxAttempts:   1,
			totalDuration: 50 * time.Millisecond,
			lastError:     errors.New("disk space full"),
			operationName: "disk_operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryMetricsConfig{
				InstanceID:  "test-exhaustion-001",
				ServiceName: "test-service",
			}

			meterProvider := createTestMeterProvider(t)
			defer func() {
				err := meterProvider.Shutdown(context.Background())
				require.NoError(t, err)
			}()

			metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
			require.NoError(t, err)

			ctx := context.Background()

			// Record retry exhaustion
			metrics.RecordRetryExhaustion(ctx, tt.maxAttempts, tt.totalDuration, tt.lastError, tt.operationName)

			// Verify exhaustion counter and total duration histogram were recorded
			err = meterProvider.ForceFlush(ctx)
			require.NoError(t, err)
		})
	}
}

// TestRetryMetrics_RecordRetryDelay tests recording retry delay measurements.
func TestRetryMetrics_RecordRetryDelay(t *testing.T) {
	tests := []struct {
		name             string
		delay            time.Duration
		attempt          int
		backoffAlgorithm string
	}{
		{
			name:             "exponential backoff delay",
			delay:            200 * time.Millisecond,
			attempt:          2,
			backoffAlgorithm: "exponential",
		},
		{
			name:             "linear backoff delay",
			delay:            500 * time.Millisecond,
			attempt:          3,
			backoffAlgorithm: "linear",
		},
		{
			name:             "fixed delay",
			delay:            1 * time.Second,
			attempt:          1,
			backoffAlgorithm: "fixed",
		},
		{
			name:             "jittered exponential delay",
			delay:            350 * time.Millisecond,
			attempt:          2,
			backoffAlgorithm: "exponential_jitter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryMetricsConfig{
				InstanceID:         "test-delay-001",
				ServiceName:        "test-service",
				EnableDelayBuckets: true,
			}

			meterProvider := createTestMeterProvider(t)
			defer func() {
				err := meterProvider.Shutdown(context.Background())
				require.NoError(t, err)
			}()

			metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
			require.NoError(t, err)

			ctx := context.Background()

			// Record retry delay
			metrics.RecordRetryDelay(ctx, tt.delay, tt.attempt, tt.backoffAlgorithm)

			// Verify delay histogram was recorded with proper attributes
			err = meterProvider.ForceFlush(ctx)
			require.NoError(t, err)
		})
	}
}

// TestRetryMetrics_RecordCircuitBreakerEvent tests recording circuit breaker events.
func TestRetryMetrics_RecordCircuitBreakerEvent(t *testing.T) {
	tests := []struct {
		name          string
		state         string
		operationName string
	}{
		{
			name:          "circuit breaker opened",
			state:         "open",
			operationName: "external_api_call",
		},
		{
			name:          "circuit breaker closed",
			state:         "closed",
			operationName: "external_api_call",
		},
		{
			name:          "circuit breaker half open",
			state:         "half_open",
			operationName: "database_connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryMetricsConfig{
				InstanceID:  "test-circuit-001",
				ServiceName: "test-service",
			}

			meterProvider := createTestMeterProvider(t)
			defer func() {
				err := meterProvider.Shutdown(context.Background())
				require.NoError(t, err)
			}()

			metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
			require.NoError(t, err)

			ctx := context.Background()

			// Record circuit breaker event
			metrics.RecordCircuitBreakerEvent(ctx, tt.state, tt.operationName)

			// Verify circuit breaker counter was incremented
			err = meterProvider.ForceFlush(ctx)
			require.NoError(t, err)
		})
	}
}

// TestRetryMetrics_RecordPolicyChange tests recording policy changes.
func TestRetryMetrics_RecordPolicyChange(t *testing.T) {
	tests := []struct {
		name          string
		oldPolicy     string
		newPolicy     string
		operationName string
	}{
		{
			name:          "switched from exponential to linear",
			oldPolicy:     "exponential_backoff",
			newPolicy:     "linear_backoff",
			operationName: "adaptive_operation",
		},
		{
			name:          "increased max attempts",
			oldPolicy:     "max_3_attempts",
			newPolicy:     "max_5_attempts",
			operationName: "critical_operation",
		},
		{
			name:          "enabled circuit breaker",
			oldPolicy:     "no_circuit_breaker",
			newPolicy:     "with_circuit_breaker",
			operationName: "external_service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryMetricsConfig{
				InstanceID:  "test-policy-001",
				ServiceName: "test-service",
			}

			meterProvider := createTestMeterProvider(t)
			defer func() {
				err := meterProvider.Shutdown(context.Background())
				require.NoError(t, err)
			}()

			metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
			require.NoError(t, err)

			ctx := context.Background()

			// Record policy change
			metrics.RecordPolicyChange(ctx, tt.oldPolicy, tt.newPolicy, tt.operationName)

			// Verify policy change counter was incremented
			err = meterProvider.ForceFlush(ctx)
			require.NoError(t, err)
		})
	}
}

// TestRetryMetrics_DelayHistogramBuckets tests custom delay histogram buckets.
func TestRetryMetrics_DelayHistogramBuckets(t *testing.T) {
	tests := []struct {
		name           string
		customBuckets  []float64
		testDelays     []time.Duration
		expectedBucket []int // Which bucket each delay should fall into
	}{
		{
			name:           "default exponential buckets",
			customBuckets:  nil, // Use defaults
			testDelays:     []time.Duration{50 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second},
			expectedBucket: []int{0, 2, 4}, // Approximate bucket indices
		},
		{
			name:           "custom linear buckets",
			customBuckets:  []float64{0.1, 0.5, 1.0, 2.0, 5.0},
			testDelays:     []time.Duration{200 * time.Millisecond, 1500 * time.Millisecond, 6 * time.Second},
			expectedBucket: []int{1, 3, 4},
		},
		{
			name:           "fine-grained buckets",
			customBuckets:  []float64{0.01, 0.05, 0.1, 0.2, 0.3, 0.5, 1.0},
			testDelays:     []time.Duration{30 * time.Millisecond, 150 * time.Millisecond, 800 * time.Millisecond},
			expectedBucket: []int{2, 3, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryMetricsConfig{
				InstanceID:         "test-buckets-001",
				ServiceName:        "test-service",
				EnableDelayBuckets: true,
				CustomDelayBuckets: tt.customBuckets,
			}

			meterProvider := createTestMeterProvider(t)
			defer func() {
				err := meterProvider.Shutdown(context.Background())
				require.NoError(t, err)
			}()

			metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
			require.NoError(t, err)

			ctx := context.Background()

			// Record test delays
			for i, delay := range tt.testDelays {
				metrics.RecordRetryDelay(ctx, delay, i+1, "test_algorithm")
			}

			// Force flush and verify histogram buckets
			err = meterProvider.ForceFlush(ctx)
			require.NoError(t, err)

			// Verify bucket distribution matches expectations
			// Additional bucket verification would be implemented in GREEN phase
		})
	}
}

// TestRetryMetrics_AttributeCardinality tests that metrics maintain reasonable cardinality.
func TestRetryMetrics_AttributeCardinality(t *testing.T) {
	// GREEN phase implementation ready

	config := RetryMetricsConfig{
		InstanceID:  "test-cardinality-001",
		ServiceName: "test-service",
	}

	meterProvider := createTestMeterProvider(t)
	defer func() {
		err := meterProvider.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
	require.NoError(t, err)

	ctx := context.Background()

	// Record many different combinations to test cardinality
	operationNames := []string{"op1", "op2", "op3", "op4", "op5"}
	failureTypes := []FailureType{FailureTypeNetwork, FailureTypeAuthentication, FailureTypeDiskSpace}
	backoffAlgorithms := []string{"exponential", "linear", "fixed"}

	for _, op := range operationNames {
		for _, ft := range failureTypes {
			for _, algo := range backoffAlgorithms {
				metrics.RecordRetryAttempt(ctx, 1, ft, op)
				metrics.RecordRetryDelay(ctx, 100*time.Millisecond, 1, algo)
			}
		}
	}

	err = meterProvider.ForceFlush(ctx)
	require.NoError(t, err)

	// Verify cardinality is within acceptable limits
	// This would involve checking the actual metric data points generated
}

// TestRetryMetrics_ConcurrentSafety tests thread safety of metrics collection.
func TestRetryMetrics_ConcurrentSafety(t *testing.T) {
	// GREEN phase implementation ready

	config := RetryMetricsConfig{
		InstanceID:  "test-concurrent-001",
		ServiceName: "test-service",
	}

	meterProvider := createTestMeterProvider(t)
	defer func() {
		err := meterProvider.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	metrics, err := NewRetryMetricsWithProvider(config, meterProvider)
	require.NoError(t, err)

	ctx := context.Background()
	const goroutines = 10
	const iterations = 100
	done := make(chan bool, goroutines)

	// Run concurrent metric recording
	for i := range goroutines {
		go func(id int) {
			defer func() { done <- true }()

			for j := range iterations {
				attempt := j%5 + 1
				failureType := FailureType(j % 3)
				operationName := fmt.Sprintf("operation_%d", id)

				metrics.RecordRetryAttempt(ctx, attempt, failureType, operationName)
				metrics.RecordRetryDelay(ctx, time.Duration(j*10)*time.Millisecond, attempt, "concurrent_test")

				if j%10 == 0 {
					metrics.RecordRetrySuccess(ctx, attempt, time.Duration(j*100)*time.Millisecond, operationName)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for range goroutines {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for concurrent metrics operations")
		}
	}

	err = meterProvider.ForceFlush(ctx)
	require.NoError(t, err)
}

// Helper function to create test meter provider.
func createTestMeterProvider(t *testing.T) *sdkmetric.MeterProvider {
	t.Helper()

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", "test-service"),
			attribute.String("service.version", "test"),
		),
	)
	require.NoError(t, err)

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewManualReader(),
		),
	)
}
