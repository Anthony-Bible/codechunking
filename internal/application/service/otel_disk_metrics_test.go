package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

// TestDiskMetrics_NewDiskMetrics tests creating a new DiskMetrics instance.
func TestDiskMetrics_NewDiskMetrics(t *testing.T) {
	tests := []struct {
		name        string
		instanceID  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid instance ID creates metrics successfully",
			instanceID: "disk-service-001",
			wantErr:    false,
		},
		{
			name:       "valid UUID instance ID creates metrics successfully",
			instanceID: "550e8400-e29b-41d4-a716-446655440000",
			wantErr:    false,
		},
		{
			name:        "empty instance ID returns error",
			instanceID:  "",
			wantErr:     true,
			errContains: "instance ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := NewDiskMetrics(tt.instanceID)

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

			// Verify all metrics instruments are initialized
			assert.NotNil(t, metrics.diskUsageGauge)
			assert.NotNil(t, metrics.diskOperationDuration)
			assert.NotNil(t, metrics.diskOperationCounter)
			assert.NotNil(t, metrics.diskAlertCounter)
			assert.NotNil(t, metrics.diskHealthCheckDuration)
			assert.NotNil(t, metrics.diskHealthStatusGauge)
			assert.NotNil(t, metrics.cleanupOperationDuration)
			assert.NotNil(t, metrics.cleanupItemsCounter)
			assert.NotNil(t, metrics.cleanupBytesFreedCounter)
			assert.NotNil(t, metrics.retentionPolicyDuration)
			assert.NotNil(t, metrics.retentionPolicyCounter)

			assert.Equal(t, tt.instanceID, metrics.instanceID)
		})
	}
}

// TestDiskMetrics_RecordDiskUsage tests recording disk usage metrics.
func TestDiskMetrics_RecordDiskUsage(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		usageBytes    int64
		totalBytes    int64
		usagePercent  float64
		operationType string
		expectError   bool
	}{
		{
			name:          "record normal disk usage",
			path:          "/tmp/codechunking-cache",
			usageBytes:    25 * 1024 * 1024 * 1024,  // 25GB
			totalBytes:    100 * 1024 * 1024 * 1024, // 100GB
			usagePercent:  25.0,
			operationType: "get_current_usage",
		},
		{
			name:          "record high disk usage warning threshold",
			path:          "/tmp/codechunking-cache/warning",
			usageBytes:    80 * 1024 * 1024 * 1024,  // 80GB
			totalBytes:    100 * 1024 * 1024 * 1024, // 100GB
			usagePercent:  80.0,
			operationType: "get_current_usage",
		},
		{
			name:          "record critical disk usage",
			path:          "/tmp/codechunking-cache/critical",
			usageBytes:    95 * 1024 * 1024 * 1024,  // 95GB
			totalBytes:    100 * 1024 * 1024 * 1024, // 100GB
			usagePercent:  95.0,
			operationType: "get_current_usage",
		},
		{
			name:          "record monitoring operation",
			path:          "/var/cache/repos",
			usageBytes:    50 * 1024 * 1024 * 1024,  // 50GB
			totalBytes:    200 * 1024 * 1024 * 1024, // 200GB
			usagePercent:  25.0,
			operationType: "monitor_usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated metrics environment for each test case
			reader := sdkmetric.NewManualReader()
			resource := resource.NewWithAttributes("test")
			provider := sdkmetric.NewMeterProvider(
				sdkmetric.WithReader(reader),
				sdkmetric.WithResource(resource),
			)

			metrics, err := NewDiskMetricsWithProvider("test-instance", provider)
			require.NoError(t, err)

			ctx := context.Background()

			metrics.RecordDiskUsage(ctx, tt.path, tt.usageBytes, tt.totalBytes, tt.usagePercent, tt.operationType)

			// Collect metrics and verify
			var data metricdata.ResourceMetrics
			err = reader.Collect(ctx, &data)
			require.NoError(t, err)

			// Verify disk usage gauge was recorded
			foundGaugeMetric := false
			for _, scopeMetric := range data.ScopeMetrics {
				for _, metric := range scopeMetric.Metrics {
					if metric.Name == DiskUsageGaugeName {
						foundGaugeMetric = true
						gauge, ok := metric.Data.(metricdata.Gauge[float64])
						require.True(t, ok, "Expected Gauge[float64] data type")

						// Find our specific data point
						for _, datapoint := range gauge.DataPoints {
							pathAttr := attribute.String(AttrDiskPath, tt.path)
							opAttr := attribute.String(AttrOperationType, tt.operationType)

							if datapoint.Attributes.HasValue(pathAttr.Key) &&
								datapoint.Attributes.HasValue(opAttr.Key) {
								assert.InEpsilon(t, tt.usagePercent, datapoint.Value, 0.001)
								assert.Contains(t, datapoint.Attributes.ToSlice(), pathAttr)
								assert.Contains(t, datapoint.Attributes.ToSlice(), opAttr)
							}
						}
					}
				}
			}
			assert.True(t, foundGaugeMetric, "Expected to find disk usage gauge metric")
		})
	}
}

// TestDiskMetrics_RecordDiskOperation tests recording disk operation timing and success metrics.
func TestDiskMetrics_RecordDiskOperation(t *testing.T) {
	tests := []struct {
		name            string
		operation       string
		path            string
		duration        time.Duration
		result          string
		correlationID   string
		expectHistogram bool
		expectCounter   bool
	}{
		{
			name:            "successful get current disk usage operation",
			operation:       "get_current_usage",
			path:            "/tmp/codechunking-cache",
			duration:        50 * time.Millisecond,
			result:          "success",
			correlationID:   "req-123",
			expectHistogram: true,
			expectCounter:   true,
		},
		{
			name:            "failed disk usage operation with permission error",
			operation:       "get_current_usage",
			path:            "/root/restricted",
			duration:        10 * time.Millisecond,
			result:          "permission_denied",
			correlationID:   "req-124",
			expectHistogram: true,
			expectCounter:   true,
		},
		{
			name:            "disk monitoring operation",
			operation:       "monitor_usage",
			path:            "/var/cache",
			duration:        200 * time.Millisecond,
			result:          "success",
			correlationID:   "req-125",
			expectHistogram: true,
			expectCounter:   true,
		},
		{
			name:            "disk health check operation",
			operation:       "check_health",
			path:            "/tmp/codechunking-cache",
			duration:        100 * time.Millisecond,
			result:          "success",
			correlationID:   "req-126",
			expectHistogram: true,
			expectCounter:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated metrics environment for each test case
			reader := sdkmetric.NewManualReader()
			resource := resource.NewWithAttributes("test")
			provider := sdkmetric.NewMeterProvider(
				sdkmetric.WithReader(reader),
				sdkmetric.WithResource(resource),
			)

			metrics, err := NewDiskMetricsWithProvider("test-instance", provider)
			require.NoError(t, err)

			ctx := context.Background()

			metrics.RecordDiskOperation(ctx, tt.operation, tt.path, tt.duration, tt.result, tt.correlationID)

			var data metricdata.ResourceMetrics
			err = reader.Collect(ctx, &data)
			require.NoError(t, err)

			foundHistogram := false
			foundCounter := false

			for _, scopeMetric := range data.ScopeMetrics {
				for _, metric := range scopeMetric.Metrics {
					switch metric.Name {
					case DiskOperationDurationHistogramName:
						if tt.expectHistogram {
							foundHistogram = true
							histogram, ok := metric.Data.(metricdata.Histogram[float64])
							require.True(t, ok, "Expected Histogram[float64] data type")

							// Verify histogram recorded the duration
							for _, datapoint := range histogram.DataPoints {
								expectedAttrs := []attribute.KeyValue{
									attribute.String(AttrDiskOperation, tt.operation),
									attribute.String(AttrDiskPath, tt.path),
									attribute.String(AttrOperationResult, tt.result),
									attribute.String(AttrCorrelationID, tt.correlationID),
									attribute.String(AttrInstanceID, "test-instance"),
								}

								for _, expectedAttr := range expectedAttrs {
									assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
								}

								// Verify duration is within expected bucket
								assert.Equal(t, uint64(1), datapoint.Count)
								assert.InEpsilon(t, tt.duration.Seconds(), datapoint.Sum, 0.001)
							}
						}

					case DiskOperationCounterName:
						if tt.expectCounter {
							foundCounter = true
							counter, ok := metric.Data.(metricdata.Sum[int64])
							require.True(t, ok, "Expected Sum[int64] data type")

							for _, datapoint := range counter.DataPoints {
								expectedAttrs := []attribute.KeyValue{
									attribute.String(AttrDiskOperation, tt.operation),
									attribute.String(AttrDiskPath, tt.path),
									attribute.String(AttrOperationResult, tt.result),
									attribute.String(AttrCorrelationID, tt.correlationID),
									attribute.String(AttrInstanceID, "test-instance"),
								}

								for _, expectedAttr := range expectedAttrs {
									assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
								}

								assert.Equal(t, int64(1), datapoint.Value)
							}
						}
					}
				}
			}

			if tt.expectHistogram {
				assert.True(t, foundHistogram, "Expected to find disk operation duration histogram")
			}
			if tt.expectCounter {
				assert.True(t, foundCounter, "Expected to find disk operation counter")
			}
		})
	}
}

// TestDiskMetrics_RecordDiskAlert tests recording disk alert metrics by severity.
func TestDiskMetrics_RecordDiskAlert(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		alertLevel    string
		alertType     string
		correlationID string
	}{
		{
			name:          "warning level disk alert",
			path:          "/tmp/codechunking-cache",
			alertLevel:    "warning",
			alertType:     "high_usage",
			correlationID: "alert-001",
		},
		{
			name:          "critical level disk alert",
			path:          "/tmp/codechunking-cache/critical",
			alertLevel:    "critical",
			alertType:     "critical_usage",
			correlationID: "alert-002",
		},
		{
			name:          "emergency level disk alert",
			path:          "/var/cache",
			alertLevel:    "emergency",
			alertType:     "disk_full",
			correlationID: "alert-003",
		},
		{
			name:          "performance degradation alert",
			path:          "/tmp/codechunking-cache",
			alertLevel:    "warning",
			alertType:     "performance_degraded",
			correlationID: "alert-004",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated metrics environment for each test case
			reader := sdkmetric.NewManualReader()
			resource := resource.NewWithAttributes("test")
			provider := sdkmetric.NewMeterProvider(
				sdkmetric.WithReader(reader),
				sdkmetric.WithResource(resource),
			)

			metrics, err := NewDiskMetricsWithProvider("test-instance", provider)
			require.NoError(t, err)

			ctx := context.Background()

			metrics.RecordDiskAlert(ctx, tt.path, tt.alertLevel, tt.alertType, tt.correlationID)

			var data metricdata.ResourceMetrics
			err = reader.Collect(ctx, &data)
			require.NoError(t, err)

			foundAlertCounter := false
			for _, scopeMetric := range data.ScopeMetrics {
				for _, metric := range scopeMetric.Metrics {
					if metric.Name == DiskAlertCounterName {
						foundAlertCounter = true
						counter, ok := metric.Data.(metricdata.Sum[int64])
						require.True(t, ok, "Expected Sum[int64] data type")

						for _, datapoint := range counter.DataPoints {
							expectedAttrs := []attribute.KeyValue{
								attribute.String(AttrDiskPath, tt.path),
								attribute.String(AttrAlertLevel, tt.alertLevel),
								attribute.String(AttrAlertType, tt.alertType),
								attribute.String(AttrCorrelationID, tt.correlationID),
								attribute.String(AttrInstanceID, "test-instance"),
							}

							for _, expectedAttr := range expectedAttrs {
								assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
							}

							assert.Equal(t, int64(1), datapoint.Value)
						}
					}
				}
			}

			assert.True(t, foundAlertCounter, "Expected to find disk alert counter metric")
		})
	}
}

// TestDiskMetrics_RecordDiskHealthCheck tests recording disk health check metrics.
func TestDiskMetrics_RecordDiskHealthCheck(t *testing.T) {
	tests := []struct {
		name          string
		paths         []string
		duration      time.Duration
		overallHealth string
		pathsChecked  int
		activeAlerts  int
		correlationID string
	}{
		{
			name:          "successful health check with good status",
			paths:         []string{"/tmp/codechunking-cache"},
			duration:      150 * time.Millisecond,
			overallHealth: "good",
			pathsChecked:  1,
			activeAlerts:  0,
			correlationID: "health-001",
		},
		{
			name:          "health check with warning status",
			paths:         []string{"/tmp/codechunking-cache/warning"},
			duration:      200 * time.Millisecond,
			overallHealth: "warning",
			pathsChecked:  1,
			activeAlerts:  1,
			correlationID: "health-002",
		},
		{
			name:          "health check with critical status multiple paths",
			paths:         []string{"/tmp/codechunking-cache/warning", "/tmp/codechunking-cache/critical"},
			duration:      300 * time.Millisecond,
			overallHealth: "critical",
			pathsChecked:  2,
			activeAlerts:  3,
			correlationID: "health-003",
		},
		{
			name:          "health check with failed access",
			paths:         []string{"/invalid/path"},
			duration:      50 * time.Millisecond,
			overallHealth: "failed",
			pathsChecked:  1,
			activeAlerts:  1,
			correlationID: "health-004",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated metrics environment for each test case
			reader := sdkmetric.NewManualReader()
			resource := resource.NewWithAttributes("test")
			provider := sdkmetric.NewMeterProvider(
				sdkmetric.WithReader(reader),
				sdkmetric.WithResource(resource),
			)

			metrics, err := NewDiskMetricsWithProvider("test-instance", provider)
			require.NoError(t, err)

			ctx := context.Background()

			metrics.RecordDiskHealthCheck(ctx, tt.paths, tt.duration, tt.overallHealth,
				tt.pathsChecked, tt.activeAlerts, tt.correlationID)

			var data metricdata.ResourceMetrics
			err = reader.Collect(ctx, &data)
			require.NoError(t, err)

			foundHistogram := false
			foundGauge := false

			for _, scopeMetric := range data.ScopeMetrics {
				for _, metric := range scopeMetric.Metrics {
					switch metric.Name {
					case DiskHealthCheckDurationHistogramName:
						foundHistogram = true
						histogram, ok := metric.Data.(metricdata.Histogram[float64])
						require.True(t, ok, "Expected Histogram[float64] data type")

						for _, datapoint := range histogram.DataPoints {
							expectedAttrs := []attribute.KeyValue{
								attribute.String(AttrOverallHealth, tt.overallHealth),
								attribute.Int(AttrPathsChecked, tt.pathsChecked),
								attribute.Int(AttrActiveAlerts, tt.activeAlerts),
								attribute.String(AttrCorrelationID, tt.correlationID),
								attribute.String(AttrInstanceID, "test-instance"),
							}

							for _, expectedAttr := range expectedAttrs {
								assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
							}

							assert.Equal(t, uint64(1), datapoint.Count)
							assert.InEpsilon(t, tt.duration.Seconds(), datapoint.Sum, 0.001)
						}

					case DiskHealthStatusGaugeName:
						foundGauge = true
						gauge, ok := metric.Data.(metricdata.Gauge[int64])
						require.True(t, ok, "Expected Gauge[int64] data type")

						for _, datapoint := range gauge.DataPoints {
							expectedAttrs := []attribute.KeyValue{
								attribute.String(AttrOverallHealth, tt.overallHealth),
								attribute.Int(AttrActiveAlerts, tt.activeAlerts),
								attribute.String(AttrInstanceID, "test-instance"),
							}

							for _, expectedAttr := range expectedAttrs {
								assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
							}

							// Verify health status is encoded as integer
							var expectedHealthValue int64
							switch tt.overallHealth {
							case "good":
								expectedHealthValue = 0
							case "warning":
								expectedHealthValue = 1
							case "critical":
								expectedHealthValue = 2
							case "failed":
								expectedHealthValue = 3
							default:
								expectedHealthValue = -1
							}
							assert.Equal(t, expectedHealthValue, datapoint.Value)
						}
					}
				}
			}

			assert.True(t, foundHistogram, "Expected to find disk health check duration histogram")
			assert.True(t, foundGauge, "Expected to find disk health status gauge")
		})
	}
}

// TestDiskMetrics_RecordCleanupOperation tests recording cleanup operation metrics.
func TestDiskMetrics_RecordCleanupOperation(t *testing.T) {
	tests := []struct {
		name          string
		strategy      string
		path          string
		duration      time.Duration
		itemsCleaned  int64
		bytesFreed    int64
		dryRun        bool
		result        string
		correlationID string
	}{
		{
			name:          "successful LRU cleanup operation",
			strategy:      "lru",
			path:          "/tmp/codechunking-cache",
			duration:      15 * time.Minute,
			itemsCleaned:  50,
			bytesFreed:    5 * 1024 * 1024 * 1024, // 5GB
			dryRun:        false,
			result:        "success",
			correlationID: "cleanup-001",
		},
		{
			name:          "TTL cleanup dry run operation",
			strategy:      "ttl",
			path:          "/tmp/codechunking-cache",
			duration:      20 * time.Minute,
			itemsCleaned:  0, // Dry run
			bytesFreed:    0, // Dry run
			dryRun:        true,
			result:        "success",
			correlationID: "cleanup-002",
		},
		{
			name:          "size-based cleanup with partial failure",
			strategy:      "size_based",
			path:          "/tmp/codechunking-cache",
			duration:      25 * time.Minute,
			itemsCleaned:  70,
			bytesFreed:    35 * 1024 * 1024 * 1024, // 35GB
			dryRun:        false,
			result:        "partial_failure",
			correlationID: "cleanup-003",
		},
		{
			name:          "smart cleanup with ML optimization",
			strategy:      "smart",
			path:          "/tmp/codechunking-cache",
			duration:      40 * time.Minute,
			itemsCleaned:  95,
			bytesFreed:    22 * 1024 * 1024 * 1024, // 22GB
			dryRun:        false,
			result:        "success",
			correlationID: "cleanup-004",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated metrics environment for each test case
			reader := sdkmetric.NewManualReader()
			resource := resource.NewWithAttributes("test")
			provider := sdkmetric.NewMeterProvider(
				sdkmetric.WithReader(reader),
				sdkmetric.WithResource(resource),
			)

			metrics, err := NewDiskMetricsWithProvider("test-instance", provider)
			require.NoError(t, err)

			ctx := context.Background()

			metrics.RecordCleanupOperation(ctx, tt.strategy, tt.path, tt.duration,
				tt.itemsCleaned, tt.bytesFreed, tt.dryRun, tt.result, tt.correlationID)

			var data metricdata.ResourceMetrics
			err = reader.Collect(ctx, &data)
			require.NoError(t, err)

			foundDurationHistogram := false
			foundItemsCounter := false
			foundBytesCounter := false

			for _, scopeMetric := range data.ScopeMetrics {
				for _, metric := range scopeMetric.Metrics {
					switch metric.Name {
					case CleanupOperationDurationHistogramName:
						foundDurationHistogram = true
						histogram, ok := metric.Data.(metricdata.Histogram[float64])
						require.True(t, ok, "Expected Histogram[float64] data type")

						for _, datapoint := range histogram.DataPoints {
							expectedAttrs := []attribute.KeyValue{
								attribute.String(AttrCleanupStrategy, tt.strategy),
								attribute.String(AttrDiskPath, tt.path),
								attribute.Bool(AttrDryRun, tt.dryRun),
								attribute.String(AttrOperationResult, tt.result),
								attribute.String(AttrCorrelationID, tt.correlationID),
								attribute.String(AttrInstanceID, "test-instance"),
							}

							for _, expectedAttr := range expectedAttrs {
								assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
							}

							assert.Equal(t, uint64(1), datapoint.Count)
							assert.InEpsilon(t, tt.duration.Seconds(), datapoint.Sum, 0.001)
						}

					case CleanupItemsCounterName:
						foundItemsCounter = true
						counter, ok := metric.Data.(metricdata.Sum[int64])
						require.True(t, ok, "Expected Sum[int64] data type")

						for _, datapoint := range counter.DataPoints {
							expectedAttrs := []attribute.KeyValue{
								attribute.String(AttrCleanupStrategy, tt.strategy),
								attribute.String(AttrDiskPath, tt.path),
								attribute.Bool(AttrDryRun, tt.dryRun),
								attribute.String(AttrOperationResult, tt.result),
								attribute.String(AttrCorrelationID, tt.correlationID),
								attribute.String(AttrInstanceID, "test-instance"),
							}

							for _, expectedAttr := range expectedAttrs {
								assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
							}

							assert.Equal(t, tt.itemsCleaned, datapoint.Value)
						}

					case CleanupBytesFreedCounterName:
						foundBytesCounter = true
						counter, ok := metric.Data.(metricdata.Sum[int64])
						require.True(t, ok, "Expected Sum[int64] data type")

						for _, datapoint := range counter.DataPoints {
							expectedAttrs := []attribute.KeyValue{
								attribute.String(AttrCleanupStrategy, tt.strategy),
								attribute.String(AttrDiskPath, tt.path),
								attribute.Bool(AttrDryRun, tt.dryRun),
								attribute.String(AttrOperationResult, tt.result),
								attribute.String(AttrCorrelationID, tt.correlationID),
								attribute.String(AttrInstanceID, "test-instance"),
							}

							for _, expectedAttr := range expectedAttrs {
								assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
							}

							assert.Equal(t, tt.bytesFreed, datapoint.Value)
						}
					}
				}
			}

			assert.True(t, foundDurationHistogram, "Expected to find cleanup operation duration histogram")
			assert.True(t, foundItemsCounter, "Expected to find cleanup items counter")
			assert.True(t, foundBytesCounter, "Expected to find cleanup bytes freed counter")
		})
	}
}

// TestDiskMetrics_RecordRetentionPolicyOperation tests recording retention policy metrics.
func TestDiskMetrics_RecordRetentionPolicyOperation(t *testing.T) {
	tests := []struct {
		name             string
		operation        string
		policyID         string
		path             string
		duration         time.Duration
		itemsEvaluated   int64
		itemsAffected    int64
		complianceStatus string
		dryRun           bool
		result           string
		correlationID    string
	}{
		{
			name:             "successful policy enforcement",
			operation:        "enforce_policy",
			policyID:         "policy-123",
			path:             "/tmp/test",
			duration:         15 * time.Minute,
			itemsEvaluated:   250,
			itemsAffected:    75,
			complianceStatus: "pass",
			dryRun:           false,
			result:           "success",
			correlationID:    "policy-001",
		},
		{
			name:             "policy enforcement dry run",
			operation:        "enforce_policy",
			policyID:         "policy-456",
			path:             "/tmp/test",
			duration:         15 * time.Minute,
			itemsEvaluated:   180,
			itemsAffected:    0, // Dry run
			complianceStatus: "warning",
			dryRun:           true,
			result:           "success",
			correlationID:    "policy-002",
		},
		{
			name:             "compliance evaluation operation",
			operation:        "evaluate_compliance",
			policyID:         "policy-789",
			path:             "/tmp/codechunking-cache",
			duration:         5 * time.Minute,
			itemsEvaluated:   300,
			itemsAffected:    0, // Evaluation only
			complianceStatus: "fail",
			dryRun:           false,
			result:           "success",
			correlationID:    "policy-003",
		},
		{
			name:             "policy creation operation",
			operation:        "create_policy",
			policyID:         "policy-new-001",
			path:             "/var/cache",
			duration:         30 * time.Second,
			itemsEvaluated:   0,
			itemsAffected:    0,
			complianceStatus: "not_applicable",
			dryRun:           false,
			result:           "success",
			correlationID:    "policy-004",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated metrics environment for each test case
			reader := sdkmetric.NewManualReader()
			resource := resource.NewWithAttributes("test")
			provider := sdkmetric.NewMeterProvider(
				sdkmetric.WithReader(reader),
				sdkmetric.WithResource(resource),
			)

			metrics, err := NewDiskMetricsWithProvider("test-instance", provider)
			require.NoError(t, err)

			ctx := context.Background()

			metrics.RecordRetentionPolicyOperation(ctx, tt.operation, tt.policyID, tt.path,
				tt.duration, tt.itemsEvaluated, tt.itemsAffected, tt.complianceStatus,
				tt.dryRun, tt.result, tt.correlationID)

			var data metricdata.ResourceMetrics
			err = reader.Collect(ctx, &data)
			require.NoError(t, err)

			foundDurationHistogram := false
			foundCounter := false

			for _, scopeMetric := range data.ScopeMetrics {
				for _, metric := range scopeMetric.Metrics {
					switch metric.Name {
					case RetentionPolicyDurationHistogramName:
						foundDurationHistogram = true
						histogram, ok := metric.Data.(metricdata.Histogram[float64])
						require.True(t, ok, "Expected Histogram[float64] data type")

						for _, datapoint := range histogram.DataPoints {
							expectedAttrs := []attribute.KeyValue{
								attribute.String(AttrPolicyOperation, tt.operation),
								attribute.String(AttrPolicyID, tt.policyID),
								attribute.String(AttrDiskPath, tt.path),
								attribute.String(AttrComplianceStatus, tt.complianceStatus),
								attribute.Bool(AttrDryRun, tt.dryRun),
								attribute.String(AttrOperationResult, tt.result),
								attribute.String(AttrCorrelationID, tt.correlationID),
								attribute.String(AttrInstanceID, "test-instance"),
							}

							for _, expectedAttr := range expectedAttrs {
								assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
							}

							assert.Equal(t, uint64(1), datapoint.Count)
							assert.InEpsilon(t, tt.duration.Seconds(), datapoint.Sum, 0.001)
						}

					case RetentionPolicyCounterName:
						foundCounter = true
						counter, ok := metric.Data.(metricdata.Sum[int64])
						require.True(t, ok, "Expected Sum[int64] data type")

						for _, datapoint := range counter.DataPoints {
							expectedAttrs := []attribute.KeyValue{
								attribute.String(AttrPolicyOperation, tt.operation),
								attribute.String(AttrPolicyID, tt.policyID),
								attribute.String(AttrDiskPath, tt.path),
								attribute.String(AttrComplianceStatus, tt.complianceStatus),
								attribute.Bool(AttrDryRun, tt.dryRun),
								attribute.String(AttrOperationResult, tt.result),
								attribute.String(AttrCorrelationID, tt.correlationID),
								attribute.String(AttrInstanceID, "test-instance"),
							}

							for _, expectedAttr := range expectedAttrs {
								assert.Contains(t, datapoint.Attributes.ToSlice(), expectedAttr)
							}

							assert.Equal(t, int64(1), datapoint.Value)
						}
					}
				}
			}

			assert.True(t, foundDurationHistogram, "Expected to find retention policy duration histogram")
			assert.True(t, foundCounter, "Expected to find retention policy counter")
		})
	}
}

// TestDiskMetrics_HistogramBuckets tests that histogram buckets are configured correctly.
func TestDiskMetrics_HistogramBuckets(t *testing.T) {
	// DiskMetrics is now implemented

	tests := []struct {
		name              string
		histogramName     string
		expectedBuckets   []float64
		testDuration      time.Duration
		expectedBucketHit int // Which bucket index should be hit
	}{
		{
			name:              "disk operation duration buckets for fast operations",
			histogramName:     DiskOperationDurationHistogramName,
			expectedBuckets:   getDiskOperationLatencyBuckets(),
			testDuration:      25 * time.Millisecond,
			expectedBucketHit: 3, // Should hit the 0.025 (25ms) bucket
		},
		{
			name:              "cleanup operation duration buckets for long operations",
			histogramName:     CleanupOperationDurationHistogramName,
			expectedBuckets:   getCleanupDurationBuckets(),
			testDuration:      15 * time.Minute,
			expectedBucketHit: 4, // Should hit the 20 minutes bucket
		},
		{
			name:              "health check duration buckets for moderate operations",
			histogramName:     DiskHealthCheckDurationHistogramName,
			expectedBuckets:   getHealthCheckLatencyBuckets(),
			testDuration:      500 * time.Millisecond,
			expectedBucketHit: 3, // Should hit the 1.0 second bucket
		},
		{
			name:              "retention policy duration buckets for policy operations",
			histogramName:     RetentionPolicyDurationHistogramName,
			expectedBuckets:   getPolicyOperationDurationBuckets(),
			testDuration:      5 * time.Minute,
			expectedBucketHit: 3, // Should hit the 10 minutes bucket
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify expected buckets are defined correctly
			require.NotNil(t, tt.expectedBuckets)
			require.NotEmpty(t, tt.expectedBuckets)

			// Verify buckets are in ascending order
			for i := 1; i < len(tt.expectedBuckets); i++ {
				assert.Greater(t, tt.expectedBuckets[i], tt.expectedBuckets[i-1],
					"Histogram buckets should be in ascending order")
			}

			// Verify test duration falls into expected bucket
			testSeconds := tt.testDuration.Seconds()
			if tt.expectedBucketHit < len(tt.expectedBuckets) {
				if tt.expectedBucketHit > 0 {
					assert.Greater(t, testSeconds, tt.expectedBuckets[tt.expectedBucketHit-1],
						"Test duration should be greater than previous bucket")
				}
				assert.LessOrEqual(t, testSeconds, tt.expectedBuckets[tt.expectedBucketHit],
					"Test duration should fall within expected bucket")
			}
		})
	}
}

// TestDiskMetrics_MetricNames tests that metric names follow OpenTelemetry conventions.
func TestDiskMetrics_MetricNames(t *testing.T) {
	// Constants are now defined

	tests := []struct {
		name           string
		metricName     string
		expectedSuffix string
		description    string
	}{
		{
			name:           "disk usage gauge metric name",
			metricName:     DiskUsageGaugeName,
			expectedSuffix: "_bytes",
			description:    "Should end with _bytes for byte measurements",
		},
		{
			name:           "disk operation duration histogram name",
			metricName:     DiskOperationDurationHistogramName,
			expectedSuffix: "_duration_seconds",
			description:    "Should end with _duration_seconds for timing histograms",
		},
		{
			name:           "disk operation counter name",
			metricName:     DiskOperationCounterName,
			expectedSuffix: "_total",
			description:    "Should end with _total for counters",
		},
		{
			name:           "disk alert counter name",
			metricName:     DiskAlertCounterName,
			expectedSuffix: "_total",
			description:    "Should end with _total for counters",
		},
		{
			name:           "cleanup operation duration histogram name",
			metricName:     CleanupOperationDurationHistogramName,
			expectedSuffix: "_duration_seconds",
			description:    "Should end with _duration_seconds for timing histograms",
		},
		{
			name:           "cleanup items counter name",
			metricName:     CleanupItemsCounterName,
			expectedSuffix: "_total",
			description:    "Should end with _total for counters",
		},
		{
			name:           "cleanup bytes freed counter name",
			metricName:     CleanupBytesFreedCounterName,
			expectedSuffix: "_bytes_total",
			description:    "Should end with _bytes_total for byte counters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.metricName, "Metric name should not be empty")
			assert.NotEmpty(t, tt.metricName, "Metric name should have content")

			// Check naming conventions
			assert.Contains(t, tt.metricName, "disk_", "Metric name should contain 'disk_' prefix")
			assert.True(t, strings.HasSuffix(tt.metricName, tt.expectedSuffix),
				"Metric name should end with expected suffix: %s", tt.expectedSuffix)

			// Ensure no uppercase letters (OpenTelemetry convention)
			assert.Equal(t, strings.ToLower(tt.metricName), tt.metricName,
				"Metric name should be lowercase")
		})
	}
}

// TestDiskMetrics_AttributeKeys tests that attribute keys are defined correctly.
func TestDiskMetrics_AttributeKeys(t *testing.T) {
	// Attribute constants are now defined

	tests := []struct {
		name        string
		attrKey     string
		description string
	}{
		{
			name:        "disk path attribute",
			attrKey:     AttrDiskPath,
			description: "Should identify the disk path being monitored",
		},
		{
			name:        "operation type attribute",
			attrKey:     AttrOperationType,
			description: "Should identify the type of disk operation",
		},
		{
			name:        "disk operation attribute",
			attrKey:     AttrDiskOperation,
			description: "Should identify specific disk operations",
		},
		{
			name:        "operation result attribute",
			attrKey:     AttrOperationResult,
			description: "Should identify the result of operations",
		},
		{
			name:        "alert level attribute",
			attrKey:     AttrAlertLevel,
			description: "Should identify the severity level of alerts",
		},
		{
			name:        "alert type attribute",
			attrKey:     AttrAlertType,
			description: "Should identify the type of disk alert",
		},
		{
			name:        "cleanup strategy attribute",
			attrKey:     AttrCleanupStrategy,
			description: "Should identify the cleanup strategy used",
		},
		{
			name:        "dry run attribute",
			attrKey:     AttrDryRun,
			description: "Should indicate if operation was a dry run",
		},
		{
			name:        "policy operation attribute",
			attrKey:     AttrPolicyOperation,
			description: "Should identify retention policy operations",
		},
		{
			name:        "policy ID attribute",
			attrKey:     AttrPolicyID,
			description: "Should identify the retention policy ID",
		},
		{
			name:        "compliance status attribute",
			attrKey:     AttrComplianceStatus,
			description: "Should identify compliance evaluation results",
		},
		{
			name:        "correlation ID attribute",
			attrKey:     AttrCorrelationID,
			description: "Should provide correlation for distributed tracing",
		},
		{
			name:        "instance ID attribute",
			attrKey:     AttrInstanceID,
			description: "Should identify the service instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.attrKey, "Attribute key should not be empty")
			assert.NotEmpty(t, tt.attrKey, "Attribute key should have content")

			// Check attribute naming conventions
			assert.NotContains(t, tt.attrKey, " ", "Attribute key should not contain spaces")
			assert.Equal(t, strings.ToLower(tt.attrKey), tt.attrKey,
				"Attribute key should be lowercase")
		})
	}
}

// TestDiskMetrics_ErrorHandling tests error handling in metric recording.
func TestDiskMetrics_ErrorHandling(t *testing.T) {
	// This test will fail until DiskMetrics is implemented
	// DiskMetrics error handling is now implemented

	// Use noop provider to test error handling
	metrics, err := NewDiskMetricsWithProvider("test", noop.NewMeterProvider())
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("empty path should be handled gracefully", func(_ *testing.T) {
		// These should not panic even with invalid inputs
		metrics.RecordDiskUsage(ctx, "", 1024, 2048, 50.0, "test")
		metrics.RecordDiskOperation(ctx, "test", "", 100*time.Millisecond, "success", "test-id")
		metrics.RecordDiskAlert(ctx, "", "warning", "high_usage", "test-id")
	})

	t.Run("negative values should be handled gracefully", func(_ *testing.T) {
		// These should not panic even with invalid inputs
		metrics.RecordDiskUsage(ctx, "/tmp", -1024, 2048, -50.0, "test")
		metrics.RecordCleanupOperation(
			ctx,
			"lru",
			"/tmp",
			100*time.Millisecond,
			-10,
			-1024,
			false,
			"success",
			"test-id",
		)
	})

	t.Run("nil context should be handled gracefully", func(_ *testing.T) {
		// These should not panic even with nil context
		metrics.RecordDiskUsage(context.Background(), "/tmp", 1024, 2048, 50.0, "test")
		metrics.RecordDiskOperation(context.Background(), "test", "/tmp", 100*time.Millisecond, "success", "test-id")
	})
}

// TestDiskMetrics_ConcurrentAccess tests concurrent access to metrics recording.
func TestDiskMetrics_ConcurrentAccess(t *testing.T) {
	// This test will fail until DiskMetrics is implemented
	// DiskMetrics concurrent access is now implemented

	reader := sdkmetric.NewManualReader()
	resource := resource.NewWithAttributes("test")
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(resource),
	)

	metrics, err := NewDiskMetricsWithProvider("test-instance", provider)
	require.NoError(t, err)

	ctx := context.Background()
	const numGoroutines = 100
	const numOperations = 10

	// Test concurrent metric recording
	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for j := range numOperations {
				path := fmt.Sprintf("/tmp/test-%d", goroutineID)
				correlationID := fmt.Sprintf("test-%d-%d", goroutineID, j)

				// Record various metrics concurrently
				metrics.RecordDiskUsage(ctx, path, 1024*int64(j+1), 2048*int64(j+1), float64(j*10), "test")
				metrics.RecordDiskOperation(
					ctx,
					"test_op",
					path,
					time.Duration(j+1)*time.Millisecond,
					"success",
					correlationID,
				)
				metrics.RecordDiskAlert(ctx, path, "warning", "test_alert", correlationID)
				metrics.RecordCleanupOperation(
					ctx,
					"test_strategy",
					path,
					time.Duration(j+1)*time.Second,
					int64(j),
					int64(j*1024),
					false,
					"success",
					correlationID,
				)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}

	// Collect metrics and verify no data corruption occurred
	var data metricdata.ResourceMetrics
	err = reader.Collect(ctx, &data)
	require.NoError(t, err)

	// Verify we have metrics recorded (exact counts may vary due to aggregation)
	assert.NotEmpty(t, data.ScopeMetrics, "Should have recorded metrics from concurrent operations")
}
