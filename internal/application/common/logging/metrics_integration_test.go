package logging

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsIntegration_PrometheusStyleMetrics tests Prometheus-style metrics integration
func TestMetricsIntegration_PrometheusStyleMetrics(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
		MetricsConfig: MetricsConfig{
			EnablePrometheusIntegration: true,
			MetricsNamespace:            "codechunking",
			MetricsSubsystem:            "api",
		},
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := WithCorrelationID(context.Background(), "metrics-test-123")

	tests := []struct {
		name       string
		metricType string
		metricName string
		value      float64
		labels     map[string]string
		operation  string
	}{
		{
			name:       "HTTP request counter",
			metricType: "counter",
			metricName: "http_requests_total",
			value:      1,
			labels: map[string]string{
				"method": "POST",
				"path":   "/repositories",
				"status": "201",
			},
			operation: "increment_counter",
		},
		{
			name:       "Request duration histogram",
			metricType: "histogram",
			metricName: "http_request_duration_seconds",
			value:      0.150, // 150ms
			labels: map[string]string{
				"method": "POST",
				"path":   "/repositories",
			},
			operation: "observe_histogram",
		},
		{
			name:       "Active connections gauge",
			metricType: "gauge",
			metricName: "active_connections",
			value:      25,
			labels: map[string]string{
				"connection_type": "http",
			},
			operation: "set_gauge",
		},
		{
			name:       "Repository processing summary",
			metricType: "summary",
			metricName: "repository_processing_duration_seconds",
			value:      2.5,
			labels: map[string]string{
				"repository_type": "github",
				"processing_type": "indexing",
			},
			operation: "observe_summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := PrometheusMetric{
				Type:      tt.metricType,
				Name:      tt.metricName,
				Value:     tt.value,
				Labels:    tt.labels,
				Operation: tt.operation,
				Timestamp: time.Now(),
			}

			logger.LogMetric(ctx, metric)

			// Verify structured log was created
			output := getLoggerOutput(logger)
			assert.NotEmpty(t, output, "Expected metrics log output")

			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			// Verify log structure
			assert.Equal(t, "INFO", logEntry.Level)
			assert.Equal(t, "metrics-test-123", logEntry.CorrelationID)
			assert.Equal(t, "metric_logged", logEntry.Operation)
			assert.Contains(t, logEntry.Message, "Metric recorded")

			// Verify metric metadata
			assert.Equal(t, tt.metricType, logEntry.Metadata["metric_type"])
			assert.Equal(t, tt.metricName, logEntry.Metadata["metric_name"])
			assert.Equal(t, tt.value, logEntry.Metadata["metric_value"])
			assert.Equal(t, tt.operation, logEntry.Metadata["operation"])

			// Verify labels
			assert.Contains(t, logEntry.Metadata, "labels")
			labels := logEntry.Metadata["labels"].(map[string]interface{})
			for key, expectedValue := range tt.labels {
				assert.Equal(t, expectedValue, labels[key])
			}

			// Verify Prometheus format integration
			assert.Contains(t, logEntry.Metadata, "prometheus_format")
			prometheusFormat := logEntry.Metadata["prometheus_format"].(string)
			assert.Contains(t, prometheusFormat, tt.metricName)
		})
	}
}

// TestMetricsIntegration_ApplicationMetrics tests application-level metrics logging
func TestMetricsIntegration_ApplicationMetrics(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
		MetricsConfig: MetricsConfig{
			EnableApplicationMetrics: true,
			MetricsNamespace:         "codechunking",
		},
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := WithCorrelationID(context.Background(), "app-metrics-456")

	tests := []struct {
		name           string
		metricsData    ApplicationMetrics
		expectedFields []string
	}{
		{
			name: "repository indexing metrics",
			metricsData: ApplicationMetrics{
				Category:   "indexing",
				Operation:  "repository_processing",
				Throughput: 15.5, // repositories per minute
				ErrorRate:  0.02, // 2% error rate
				Latency: LatencyMetrics{
					P50: time.Millisecond * 150,
					P95: time.Millisecond * 500,
					P99: time.Millisecond * 1200,
					Max: time.Millisecond * 2500,
				},
				ResourceUsage: ResourceMetrics{
					CPUPercent:  45.2,
					MemoryMB:    256,
					DiskUsageMB: 1024,
					NetworkKBPS: 128,
				},
				BusinessMetrics: map[string]interface{}{
					"repositories_processed": 150,
					"chunks_created":         12500,
					"embeddings_generated":   12500,
					"total_files_processed":  3200,
				},
				Duration: time.Minute * 5,
			},
			expectedFields: []string{"category", "operation", "throughput", "error_rate", "latency", "resource_usage", "business_metrics", "duration"},
		},
		{
			name: "NATS messaging metrics",
			metricsData: ApplicationMetrics{
				Category:   "messaging",
				Operation:  "nats_operations",
				Throughput: 500.0, // messages per second
				ErrorRate:  0.005, // 0.5% error rate
				Latency: LatencyMetrics{
					P50: time.Millisecond * 2,
					P95: time.Millisecond * 8,
					P99: time.Millisecond * 20,
					Max: time.Millisecond * 150,
				},
				ResourceUsage: ResourceMetrics{
					CPUPercent:  12.5,
					MemoryMB:    64,
					NetworkKBPS: 512,
				},
				BusinessMetrics: map[string]interface{}{
					"messages_published":  25000,
					"messages_consumed":   24800,
					"message_queue_depth": 15,
					"active_consumers":    8,
				},
				Duration: time.Minute * 10,
			},
			expectedFields: []string{"category", "operation", "throughput", "error_rate", "latency", "resource_usage", "business_metrics"},
		},
		{
			name: "API performance metrics",
			metricsData: ApplicationMetrics{
				Category:   "api",
				Operation:  "http_requests",
				Throughput: 125.0, // requests per second
				ErrorRate:  0.03,  // 3% error rate
				Latency: LatencyMetrics{
					P50: time.Millisecond * 50,
					P95: time.Millisecond * 200,
					P99: time.Millisecond * 500,
					Max: time.Second * 2,
				},
				ResourceUsage: ResourceMetrics{
					CPUPercent:  25.8,
					MemoryMB:    128,
					NetworkKBPS: 256,
				},
				BusinessMetrics: map[string]interface{}{
					"total_requests":      75000,
					"successful_requests": 72750,
					"failed_requests":     2250,
					"unique_users":        1250,
				},
				Duration: time.Minute * 10,
			},
			expectedFields: []string{"category", "operation", "throughput", "error_rate", "latency", "resource_usage", "business_metrics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.LogApplicationMetrics(ctx, tt.metricsData)

			// Verify structured log was created
			output := getLoggerOutput(logger)
			assert.NotEmpty(t, output, "Expected application metrics log output")

			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			// Verify log structure
			assert.Equal(t, "INFO", logEntry.Level)
			assert.Equal(t, "app-metrics-456", logEntry.CorrelationID)
			assert.Equal(t, "application_metrics", logEntry.Operation)
			assert.Contains(t, logEntry.Message, "Application metrics")

			// Verify all expected fields are present
			for _, field := range tt.expectedFields {
				assert.Contains(t, logEntry.Metadata, field, "Expected field %s in metadata", field)
			}

			// Verify specific metric values
			assert.Equal(t, tt.metricsData.Category, logEntry.Metadata["category"])
			assert.Equal(t, tt.metricsData.Operation, logEntry.Metadata["operation"])
			assert.Equal(t, tt.metricsData.Throughput, logEntry.Metadata["throughput"])
			assert.Equal(t, tt.metricsData.ErrorRate, logEntry.Metadata["error_rate"])

			// Verify latency metrics structure
			latency := logEntry.Metadata["latency"].(map[string]interface{})
			assert.Contains(t, latency, "p50")
			assert.Contains(t, latency, "p95")
			assert.Contains(t, latency, "p99")
			assert.Contains(t, latency, "max")

			// Verify resource usage structure
			resourceUsage := logEntry.Metadata["resource_usage"].(map[string]interface{})
			assert.Contains(t, resourceUsage, "cpu_percent")
			assert.Contains(t, resourceUsage, "memory_mb")

			// Verify business metrics are present
			businessMetrics := logEntry.Metadata["business_metrics"].(map[string]interface{})
			for key := range tt.metricsData.BusinessMetrics {
				assert.Contains(t, businessMetrics, key)
			}
		})
	}
}

// TestMetricsIntegration_HealthMetrics tests system health metrics logging
func TestMetricsIntegration_HealthMetrics(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
		MetricsConfig: MetricsConfig{
			EnableHealthMetrics: true,
			HealthCheckInterval: time.Second * 30,
		},
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := WithCorrelationID(context.Background(), "health-metrics-789")

	tests := []struct {
		name          string
		healthData    HealthMetrics
		expectedLevel string
	}{
		{
			name: "healthy system state",
			healthData: HealthMetrics{
				Overall: HealthStatus{
					Status:    "healthy",
					Message:   "All systems operational",
					CheckTime: time.Now(),
				},
				Database: HealthStatus{
					Status:       "healthy",
					Message:      "Connection pool healthy",
					CheckTime:    time.Now(),
					ResponseTime: time.Millisecond * 5,
					Details: map[string]interface{}{
						"active_connections": 8,
						"max_connections":    20,
						"query_avg_time":     "2.5ms",
					},
				},
				NATS: HealthStatus{
					Status:       "healthy",
					Message:      "JetStream operational",
					CheckTime:    time.Now(),
					ResponseTime: time.Millisecond * 1,
					Details: map[string]interface{}{
						"connected":        true,
						"streams_active":   2,
						"consumers_active": 5,
						"messages_pending": 12,
					},
				},
				ExternalAPIs: []ExternalAPIHealth{
					{
						Name:         "GitHub API",
						Status:       "healthy",
						ResponseTime: time.Millisecond * 150,
						LastCheck:    time.Now(),
					},
					{
						Name:         "Google Gemini API",
						Status:       "healthy",
						ResponseTime: time.Millisecond * 300,
						LastCheck:    time.Now(),
					},
				},
				SystemResources: SystemResourceMetrics{
					CPUUsage:       15.2,
					MemoryUsage:    68.5,
					DiskUsage:      45.0,
					NetworkLatency: time.Millisecond * 25,
					LoadAverage:    1.2,
				},
			},
			expectedLevel: "INFO",
		},
		{
			name: "degraded system state",
			healthData: HealthMetrics{
				Overall: HealthStatus{
					Status:    "degraded",
					Message:   "Some components experiencing issues",
					CheckTime: time.Now(),
				},
				Database: HealthStatus{
					Status:       "healthy",
					Message:      "Connection pool healthy",
					CheckTime:    time.Now(),
					ResponseTime: time.Millisecond * 8,
				},
				NATS: HealthStatus{
					Status:       "degraded",
					Message:      "High message queue depth",
					CheckTime:    time.Now(),
					ResponseTime: time.Millisecond * 5,
					Details: map[string]interface{}{
						"connected":        true,
						"streams_active":   2,
						"consumers_active": 5,
						"messages_pending": 1500, // High queue depth
					},
				},
				ExternalAPIs: []ExternalAPIHealth{
					{
						Name:         "GitHub API",
						Status:       "degraded",
						ResponseTime: time.Millisecond * 800, // Slow response
						LastCheck:    time.Now(),
						Error:        "Rate limit approaching",
					},
				},
				SystemResources: SystemResourceMetrics{
					CPUUsage:    75.8, // High CPU
					MemoryUsage: 88.2, // High memory
					DiskUsage:   45.0,
					LoadAverage: 4.2, // High load
				},
			},
			expectedLevel: "WARN",
		},
		{
			name: "unhealthy system state",
			healthData: HealthMetrics{
				Overall: HealthStatus{
					Status:    "unhealthy",
					Message:   "Critical system failures detected",
					CheckTime: time.Now(),
				},
				Database: HealthStatus{
					Status:       "unhealthy",
					Message:      "Connection pool exhausted",
					CheckTime:    time.Now(),
					ResponseTime: time.Second * 30,
					Error:        "Max connections reached",
				},
				NATS: HealthStatus{
					Status:    "unhealthy",
					Message:   "Connection lost",
					CheckTime: time.Now(),
					Error:     "Unable to connect to NATS server",
				},
				SystemResources: SystemResourceMetrics{
					CPUUsage:    95.5, // Critical CPU
					MemoryUsage: 98.2, // Critical memory
					DiskUsage:   95.0, // Critical disk
					LoadAverage: 12.5, // Critical load
				},
			},
			expectedLevel: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.LogHealthMetrics(ctx, tt.healthData)

			// Verify structured log was created
			output := getLoggerOutput(logger)
			assert.NotEmpty(t, output, "Expected health metrics log output")

			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			// Verify log level matches health status
			assert.Equal(t, tt.expectedLevel, logEntry.Level)
			assert.Equal(t, "health-metrics-789", logEntry.CorrelationID)
			assert.Equal(t, "health_check", logEntry.Operation)

			// Verify health status in metadata
			assert.Equal(t, tt.healthData.Overall.Status, logEntry.Metadata["overall_status"])
			assert.Equal(t, tt.healthData.Overall.Message, logEntry.Metadata["overall_message"])

			// Verify component health details
			assert.Contains(t, logEntry.Metadata, "database")
			assert.Contains(t, logEntry.Metadata, "nats")
			assert.Contains(t, logEntry.Metadata, "system_resources")

			// Verify external APIs health if present
			if len(tt.healthData.ExternalAPIs) > 0 {
				assert.Contains(t, logEntry.Metadata, "external_apis")
				externalAPIs := logEntry.Metadata["external_apis"].([]interface{})
				assert.Len(t, externalAPIs, len(tt.healthData.ExternalAPIs))
			}

			// Verify system resource metrics
			systemResources := logEntry.Metadata["system_resources"].(map[string]interface{})
			assert.Contains(t, systemResources, "cpu_usage")
			assert.Contains(t, systemResources, "memory_usage")
			assert.Contains(t, systemResources, "disk_usage")
		})
	}
}

// TestMetricsIntegration_CustomMetrics tests custom business metrics logging
func TestMetricsIntegration_CustomMetrics(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
		MetricsConfig: MetricsConfig{
			EnableCustomMetrics: true,
		},
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := WithCorrelationID(context.Background(), "custom-metrics-101")

	tests := []struct {
		name           string
		customMetrics  CustomMetrics
		expectedFields []string
	}{
		{
			name: "repository processing metrics",
			customMetrics: CustomMetrics{
				Namespace: "business",
				Category:  "repository_processing",
				Metrics: map[string]interface{}{
					"repositories_indexed_today":   150,
					"total_code_lines_processed":   125000,
					"average_files_per_repository": 25.5,
					"embedding_generation_rate":    85.2, // per minute
					"duplicate_detection_accuracy": 98.5, // percentage
					"processing_cost_usd":          12.45,
				},
				Dimensions: map[string]string{
					"environment":     "production",
					"data_center":     "us-east-1",
					"processing_type": "full_index",
					"language_mix":    "multi_language",
				},
				Timestamp: time.Now(),
			},
			expectedFields: []string{"namespace", "category", "metrics", "dimensions", "timestamp"},
		},
		{
			name: "user engagement metrics",
			customMetrics: CustomMetrics{
				Namespace: "product",
				Category:  "user_engagement",
				Metrics: map[string]interface{}{
					"daily_active_users":       1250,
					"search_queries_per_user":  15.8,
					"average_session_duration": time.Minute * 12,
					"search_success_rate":      92.3, // percentage
					"user_retention_7day":      78.5, // percentage
					"premium_conversion_rate":  4.2,  // percentage
				},
				Dimensions: map[string]string{
					"environment":   "production",
					"user_segment":  "all_users",
					"feature_flags": "search_v2_enabled",
				},
				Timestamp: time.Now(),
			},
			expectedFields: []string{"namespace", "category", "metrics", "dimensions"},
		},
		{
			name: "system performance metrics",
			customMetrics: CustomMetrics{
				Namespace: "infrastructure",
				Category:  "performance",
				Metrics: map[string]interface{}{
					"cache_hit_rate":              95.2,                    // percentage
					"database_query_avg_time":     time.Microsecond * 3500, // 3.5ms
					"api_response_time_p95":       time.Millisecond * 200,
					"background_job_success_rate": 99.1, // percentage
					"storage_efficiency":          87.3, // percentage
					"network_utilization":         45.8, // percentage
				},
				Dimensions: map[string]string{
					"environment": "production",
					"region":      "us-east-1",
					"cluster":     "api-cluster-1",
				},
				Timestamp: time.Now(),
			},
			expectedFields: []string{"namespace", "category", "metrics", "dimensions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.LogCustomMetrics(ctx, tt.customMetrics)

			// Verify structured log was created
			output := getLoggerOutput(logger)
			assert.NotEmpty(t, output, "Expected custom metrics log output")

			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			// Verify log structure
			assert.Equal(t, "INFO", logEntry.Level)
			assert.Equal(t, "custom-metrics-101", logEntry.CorrelationID)
			assert.Equal(t, "custom_metrics", logEntry.Operation)
			assert.Contains(t, logEntry.Message, "Custom metrics")

			// Verify all expected fields are present
			for _, field := range tt.expectedFields {
				assert.Contains(t, logEntry.Metadata, field, "Expected field %s in metadata", field)
			}

			// Verify specific values
			assert.Equal(t, tt.customMetrics.Namespace, logEntry.Metadata["namespace"])
			assert.Equal(t, tt.customMetrics.Category, logEntry.Metadata["category"])

			// Verify metrics structure
			metrics := logEntry.Metadata["metrics"].(map[string]interface{})
			for key := range tt.customMetrics.Metrics {
				assert.Contains(t, metrics, key, "Expected metric %s in metrics data", key)
			}

			// Verify dimensions structure
			dimensions := logEntry.Metadata["dimensions"].(map[string]interface{})
			for key, expectedValue := range tt.customMetrics.Dimensions {
				assert.Equal(t, expectedValue, dimensions[key], "Expected dimension %s to match", key)
			}
		})
	}
}

// TestMetricsIntegration_MetricsAggregation tests metrics aggregation and batching
func TestMetricsIntegration_MetricsAggregation(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
		MetricsConfig: MetricsConfig{
			EnableAggregation: true,
			AggregationWindow: time.Minute * 5,
			BatchSize:         100,
			EnableCompression: true,
		},
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := WithCorrelationID(context.Background(), "aggregation-test-202")

	// Simulate multiple metric events that should be aggregated
	metricEvents := []PrometheusMetric{
		{Type: "counter", Name: "http_requests_total", Value: 1, Labels: map[string]string{"method": "GET", "status": "200"}},
		{Type: "counter", Name: "http_requests_total", Value: 1, Labels: map[string]string{"method": "GET", "status": "200"}},
		{Type: "counter", Name: "http_requests_total", Value: 1, Labels: map[string]string{"method": "POST", "status": "201"}},
		{Type: "histogram", Name: "request_duration", Value: 0.1, Labels: map[string]string{"method": "GET"}},
		{Type: "histogram", Name: "request_duration", Value: 0.15, Labels: map[string]string{"method": "GET"}},
		{Type: "histogram", Name: "request_duration", Value: 0.2, Labels: map[string]string{"method": "POST"}},
	}

	// Log all metrics within aggregation window
	for _, metric := range metricEvents {
		logger.LogMetric(ctx, metric)
	}

	// Trigger aggregation flush
	logger.FlushAggregatedMetrics(ctx)

	// Verify aggregated log output
	output := getLoggerOutput(logger)
	assert.NotEmpty(t, output, "Expected aggregated metrics log output")

	var logEntry LogEntry
	err = json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	// Verify aggregation structure
	assert.Equal(t, "INFO", logEntry.Level)
	assert.Equal(t, "aggregation-test-202", logEntry.CorrelationID)
	assert.Equal(t, "aggregated_metrics", logEntry.Operation)
	assert.Contains(t, logEntry.Message, "Aggregated metrics")

	// Verify aggregated data structure
	assert.Contains(t, logEntry.Metadata, "aggregation_window")
	assert.Contains(t, logEntry.Metadata, "total_events")
	assert.Contains(t, logEntry.Metadata, "unique_metrics")
	assert.Contains(t, logEntry.Metadata, "aggregated_counters")
	assert.Contains(t, logEntry.Metadata, "histogram_summaries")

	// Verify counter aggregation
	counters := logEntry.Metadata["aggregated_counters"].(map[string]interface{})
	assert.Contains(t, counters, "http_requests_total")

	// Verify histogram aggregation
	histograms := logEntry.Metadata["histogram_summaries"].(map[string]interface{})
	assert.Contains(t, histograms, "request_duration")
}

// Test helper - structures are now implemented in metrics_integration.go
