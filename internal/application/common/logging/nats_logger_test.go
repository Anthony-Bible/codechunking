package logging

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNATSLogger_ConnectionEvents tests NATS connection event logging
func TestNATSLogger_ConnectionEvents(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	natsLogger := logger.WithComponent("nats-client")
	ctx := WithCorrelationID(context.Background(), "nats-connection-123")

	tests := []struct {
		name      string
		event     NATSConnectionEvent
		logLevel  string
		wantError bool
	}{
		{
			name: "successful connection",
			event: NATSConnectionEvent{
				Type:         "CONNECTED",
				ServerURL:    "nats://localhost:4222",
				ConnectionID: "conn-123",
				AttemptCount: 1,
				Duration:     time.Millisecond * 50,
				Success:      true,
			},
			logLevel:  "INFO",
			wantError: false,
		},
		{
			name: "connection failure",
			event: NATSConnectionEvent{
				Type:         "CONNECTION_FAILED",
				ServerURL:    "nats://localhost:4222",
				ConnectionID: "conn-124",
				AttemptCount: 3,
				Duration:     time.Second * 2,
				Success:      false,
				Error:        errors.New("connection refused"),
			},
			logLevel:  "ERROR",
			wantError: false,
		},
		{
			name: "disconnection",
			event: NATSConnectionEvent{
				Type:         "DISCONNECTED",
				ServerURL:    "nats://localhost:4222",
				ConnectionID: "conn-123",
				Success:      true,
				Reason:       "client_disconnect",
			},
			logLevel:  "INFO",
			wantError: false,
		},
		{
			name: "reconnection attempt",
			event: NATSConnectionEvent{
				Type:         "RECONNECTING",
				ServerURL:    "nats://localhost:4222",
				ConnectionID: "conn-125",
				AttemptCount: 2,
				Success:      false,
			},
			logLevel:  "WARN",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			natsLogger.LogNATSConnectionEvent(ctx, tt.event)

			output := getLoggerOutput(natsLogger)
			assert.NotEmpty(t, output, "Expected NATS connection event to be logged")

			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			// Verify log structure
			assert.Equal(t, tt.logLevel, logEntry.Level)
			assert.Equal(t, "nats-client", logEntry.Component)
			assert.Equal(t, "nats_connection", logEntry.Operation)
			assert.Equal(t, "nats-connection-123", logEntry.CorrelationID)

			// Verify NATS-specific metadata
			assert.Equal(t, tt.event.Type, logEntry.Metadata["event_type"])
			assert.Equal(t, tt.event.ServerURL, logEntry.Metadata["server_url"])
			assert.Equal(t, tt.event.ConnectionID, logEntry.Metadata["connection_id"])
			assert.Equal(t, float64(tt.event.AttemptCount), logEntry.Metadata["attempt_count"])
			assert.Equal(t, tt.event.Success, logEntry.Metadata["success"])

			if tt.event.Duration > 0 {
				assert.Contains(t, logEntry.Metadata, "duration")
			}

			if tt.event.Error != nil {
				assert.Equal(t, tt.event.Error.Error(), logEntry.Error)
			}

			if tt.event.Reason != "" {
				assert.Equal(t, tt.event.Reason, logEntry.Metadata["reason"])
			}
		})
	}
}

// TestNATSLogger_MessagePublishing tests NATS message publishing logging
func TestNATSLogger_MessagePublishing(t *testing.T) {
	config := Config{
		Level:  "DEBUG",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	natsLogger := logger.WithComponent("nats-publisher")
	ctx := WithCorrelationID(context.Background(), "publish-correlation-456")

	tests := []struct {
		name     string
		event    NATSPublishEvent
		logLevel string
	}{
		{
			name: "successful message publish",
			event: NATSPublishEvent{
				Subject:        "INDEXING.jobs",
				MessageID:      "msg-123",
				MessageSize:    1024,
				QueueName:      "indexing_queue",
				Duration:       time.Millisecond * 2,
				Success:        true,
				DeliveryPolicy: "all",
				Retries:        0,
			},
			logLevel: "DEBUG",
		},
		{
			name: "failed message publish",
			event: NATSPublishEvent{
				Subject:     "INDEXING.jobs",
				MessageID:   "msg-124",
				MessageSize: 2048,
				QueueName:   "indexing_queue",
				Duration:    time.Millisecond * 100,
				Success:     false,
				Error:       errors.New("no responders available"),
				Retries:     3,
			},
			logLevel: "ERROR",
		},
		{
			name: "large message publish",
			event: NATSPublishEvent{
				Subject:        "INDEXING.large",
				MessageID:      "msg-125",
				MessageSize:    1048576, // 1MB
				Duration:       time.Millisecond * 50,
				Success:        true,
				DeliveryPolicy: "new",
				Retries:        0,
			},
			logLevel: "INFO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			natsLogger.LogNATSPublishEvent(ctx, tt.event)

			output := getLoggerOutput(natsLogger)
			assert.NotEmpty(t, output, "Expected NATS publish event to be logged")

			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			// Verify log structure
			assert.Equal(t, tt.logLevel, logEntry.Level)
			assert.Equal(t, "nats-publisher", logEntry.Component)
			assert.Equal(t, "nats_publish", logEntry.Operation)
			assert.Equal(t, "publish-correlation-456", logEntry.CorrelationID)

			// Verify NATS publish-specific metadata
			assert.Equal(t, tt.event.Subject, logEntry.Metadata["subject"])
			assert.Equal(t, tt.event.MessageID, logEntry.Metadata["message_id"])
			assert.Equal(t, float64(tt.event.MessageSize), logEntry.Metadata["message_size"])
			assert.Equal(t, tt.event.Success, logEntry.Metadata["success"])
			assert.Contains(t, logEntry.Metadata, "duration")
			assert.Equal(t, float64(tt.event.Retries), logEntry.Metadata["retries"])

			if tt.event.QueueName != "" {
				assert.Equal(t, tt.event.QueueName, logEntry.Metadata["queue_name"])
			}

			if tt.event.DeliveryPolicy != "" {
				assert.Equal(t, tt.event.DeliveryPolicy, logEntry.Metadata["delivery_policy"])
			}

			if tt.event.Error != nil {
				assert.Equal(t, tt.event.Error.Error(), logEntry.Error)
			}
		})
	}
}

// TestNATSLogger_MessageConsumption tests NATS message consumption logging
func TestNATSLogger_MessageConsumption(t *testing.T) {
	config := Config{
		Level:  "DEBUG",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	natsLogger := logger.WithComponent("nats-consumer")
	ctx := WithCorrelationID(context.Background(), "consume-correlation-789")

	tests := []struct {
		name     string
		event    NATSConsumeEvent
		logLevel string
	}{
		{
			name: "successful message consumption",
			event: NATSConsumeEvent{
				Subject:         "INDEXING.jobs",
				MessageID:       "msg-123",
				MessageSize:     1024,
				ProcessingTime:  time.Millisecond * 150,
				AckTime:         time.Millisecond * 2,
				Success:         true,
				ConsumerGroup:   "indexing-workers",
				DeliveryAttempt: 1,
				QueuedTime:      time.Minute * 2,
			},
			logLevel: "DEBUG",
		},
		{
			name: "failed message processing",
			event: NATSConsumeEvent{
				Subject:         "INDEXING.jobs",
				MessageID:       "msg-124",
				MessageSize:     2048,
				ProcessingTime:  time.Millisecond * 300,
				Success:         false,
				Error:           errors.New("processing failed"),
				ConsumerGroup:   "indexing-workers",
				DeliveryAttempt: 2,
				QueuedTime:      time.Minute * 5,
				WillRetry:       true,
				NextRetryAt:     time.Now().Add(time.Minute * 5),
			},
			logLevel: "ERROR",
		},
		{
			name: "message rejected",
			event: NATSConsumeEvent{
				Subject:         "INDEXING.jobs",
				MessageID:       "msg-125",
				MessageSize:     512,
				ProcessingTime:  time.Millisecond * 10,
				Success:         false,
				Error:           errors.New("invalid message format"),
				ConsumerGroup:   "indexing-workers",
				DeliveryAttempt: 1,
				QueuedTime:      time.Second * 30,
				WillRetry:       false,
				Rejected:        true,
			},
			logLevel: "WARN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			natsLogger.LogNATSConsumeEvent(ctx, tt.event)

			output := getLoggerOutput(natsLogger)
			assert.NotEmpty(t, output, "Expected NATS consume event to be logged")

			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			// Verify log structure
			assert.Equal(t, tt.logLevel, logEntry.Level)
			assert.Equal(t, "nats-consumer", logEntry.Component)
			assert.Equal(t, "nats_consume", logEntry.Operation)
			assert.Equal(t, "consume-correlation-789", logEntry.CorrelationID)

			// Verify NATS consume-specific metadata
			assert.Equal(t, tt.event.Subject, logEntry.Metadata["subject"])
			assert.Equal(t, tt.event.MessageID, logEntry.Metadata["message_id"])
			assert.Equal(t, float64(tt.event.MessageSize), logEntry.Metadata["message_size"])
			assert.Equal(t, tt.event.Success, logEntry.Metadata["success"])
			assert.Contains(t, logEntry.Metadata, "processing_time")
			assert.Equal(t, tt.event.ConsumerGroup, logEntry.Metadata["consumer_group"])
			assert.Equal(t, float64(tt.event.DeliveryAttempt), logEntry.Metadata["delivery_attempt"])
			assert.Contains(t, logEntry.Metadata, "queued_time")

			if tt.event.AckTime > 0 {
				assert.Contains(t, logEntry.Metadata, "ack_time")
			}

			if tt.event.Error != nil {
				assert.Equal(t, tt.event.Error.Error(), logEntry.Error)
			}

			if tt.event.WillRetry {
				assert.Equal(t, true, logEntry.Metadata["will_retry"])
				assert.Contains(t, logEntry.Metadata, "next_retry_at")
			}

			if tt.event.Rejected {
				assert.Equal(t, true, logEntry.Metadata["rejected"])
			}
		})
	}
}

// TestNATSLogger_JetStreamOperations tests JetStream-specific logging
func TestNATSLogger_JetStreamOperations(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	natsLogger := logger.WithComponent("jetstream-client")
	ctx := WithCorrelationID(context.Background(), "jetstream-correlation-101")

	tests := []struct {
		name     string
		event    JetStreamEvent
		logLevel string
	}{
		{
			name: "stream creation",
			event: JetStreamEvent{
				Operation:  "create_stream",
				StreamName: "INDEXING",
				Success:    true,
				Duration:   time.Millisecond * 25,
				StreamConfig: map[string]interface{}{
					"subjects":  []string{"INDEXING.*"},
					"retention": "limits",
					"max_msgs":  1000000,
					"max_age":   "24h",
				},
			},
			logLevel: "INFO",
		},
		{
			name: "consumer creation",
			event: JetStreamEvent{
				Operation:    "create_consumer",
				StreamName:   "INDEXING",
				ConsumerName: "indexing-workers",
				Success:      true,
				Duration:     time.Millisecond * 15,
				ConsumerConfig: map[string]interface{}{
					"durable_name":   "indexing-workers",
					"deliver_policy": "all",
					"ack_policy":     "explicit",
					"max_deliver":    3,
				},
			},
			logLevel: "INFO",
		},
		{
			name: "stream info query",
			event: JetStreamEvent{
				Operation:  "stream_info",
				StreamName: "INDEXING",
				Success:    true,
				Duration:   time.Millisecond * 5,
				StreamInfo: &StreamInfo{
					Messages:  12500,
					Bytes:     25600000,
					Consumers: 2,
					Subjects:  []string{"INDEXING.jobs", "INDEXING.results"},
				},
			},
			logLevel: "DEBUG",
		},
		{
			name: "failed stream operation",
			event: JetStreamEvent{
				Operation:  "delete_stream",
				StreamName: "NONEXISTENT",
				Success:    false,
				Duration:   time.Millisecond * 10,
				Error:      errors.New("stream not found"),
			},
			logLevel: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			natsLogger.LogJetStreamEvent(ctx, tt.event)

			output := getLoggerOutput(natsLogger)
			assert.NotEmpty(t, output, "Expected JetStream event to be logged")

			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			// Verify log structure
			assert.Equal(t, tt.logLevel, logEntry.Level)
			assert.Equal(t, "jetstream-client", logEntry.Component)
			assert.Equal(t, "jetstream_operation", logEntry.Operation)
			assert.Equal(t, "jetstream-correlation-101", logEntry.CorrelationID)

			// Verify JetStream-specific metadata
			assert.Equal(t, tt.event.Operation, logEntry.Metadata["js_operation"])
			assert.Equal(t, tt.event.StreamName, logEntry.Metadata["stream_name"])
			assert.Equal(t, tt.event.Success, logEntry.Metadata["success"])
			assert.Contains(t, logEntry.Metadata, "duration")

			if tt.event.ConsumerName != "" {
				assert.Equal(t, tt.event.ConsumerName, logEntry.Metadata["consumer_name"])
			}

			if tt.event.StreamConfig != nil {
				assert.Contains(t, logEntry.Metadata, "stream_config")
			}

			if tt.event.ConsumerConfig != nil {
				assert.Contains(t, logEntry.Metadata, "consumer_config")
			}

			if tt.event.StreamInfo != nil {
				assert.Contains(t, logEntry.Metadata, "stream_info")
				streamInfo := logEntry.Metadata["stream_info"].(map[string]interface{})
				assert.Equal(t, float64(tt.event.StreamInfo.Messages), streamInfo["messages"])
				assert.Equal(t, float64(tt.event.StreamInfo.Bytes), streamInfo["bytes"])
				assert.Equal(t, float64(tt.event.StreamInfo.Consumers), streamInfo["consumers"])
			}

			if tt.event.Error != nil {
				assert.Equal(t, tt.event.Error.Error(), logEntry.Error)
			}
		})
	}
}

// TestNATSLogger_PerformanceMetrics tests NATS performance metrics logging
func TestNATSLogger_PerformanceMetrics(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	natsLogger := logger.WithComponent("nats-metrics")
	ctx := WithCorrelationID(context.Background(), "metrics-correlation-202")

	performanceMetrics := NATSPerformanceMetrics{
		PublishRate:     1500.5,                  // messages per second
		ConsumeRate:     1450.2,                  // messages per second
		AverageLatency:  time.Microsecond * 2500, // 2.5ms
		MaxLatency:      time.Millisecond * 25,
		ErrorRate:       0.02, // 2% error rate
		TotalMessages:   125000,
		TotalErrors:     2500,
		ConnectionCount: 5,
		ActiveConsumers: 8,
		QueueDepth:      150,
		MemoryUsage:     "128MB",
		TimeWindow:      time.Minute * 5,
	}

	natsLogger.LogNATSPerformanceMetrics(ctx, performanceMetrics)

	output := getLoggerOutput(natsLogger)
	assert.NotEmpty(t, output, "Expected NATS performance metrics to be logged")

	var logEntry LogEntry
	err = json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	// Verify log structure
	assert.Equal(t, "INFO", logEntry.Level)
	assert.Equal(t, "nats-metrics", logEntry.Component)
	assert.Equal(t, "nats_performance", logEntry.Operation)
	assert.Equal(t, "metrics-correlation-202", logEntry.CorrelationID)
	assert.Contains(t, logEntry.Message, "NATS performance metrics")

	// Verify performance metrics
	assert.Equal(t, 1500.5, logEntry.Metadata["publish_rate"])
	assert.Equal(t, 1450.2, logEntry.Metadata["consume_rate"])
	assert.Contains(t, logEntry.Metadata, "average_latency")
	assert.Contains(t, logEntry.Metadata, "max_latency")
	assert.Equal(t, 0.02, logEntry.Metadata["error_rate"])
	assert.Equal(t, float64(125000), logEntry.Metadata["total_messages"])
	assert.Equal(t, float64(2500), logEntry.Metadata["total_errors"])
	assert.Equal(t, float64(5), logEntry.Metadata["connection_count"])
	assert.Equal(t, float64(8), logEntry.Metadata["active_consumers"])
	assert.Equal(t, float64(150), logEntry.Metadata["queue_depth"])
	assert.Equal(t, "128MB", logEntry.Metadata["memory_usage"])
	assert.Contains(t, logEntry.Metadata, "time_window")
}

// TestNATSLogger_ErrorCorrelation tests error correlation across NATS operations
func TestNATSLogger_ErrorCorrelation(t *testing.T) {
	config := Config{
		Level:  "ERROR",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	correlationID := "error-correlation-999"
	ctx := WithCorrelationID(context.Background(), correlationID)

	// Test cascading errors across NATS operations
	publishLogger := logger.WithComponent("nats-publisher")
	consumerLogger := logger.WithComponent("nats-consumer")

	// Simulate publish failure
	publishEvent := NATSPublishEvent{
		Subject:     "INDEXING.jobs",
		MessageID:   "msg-error-001",
		MessageSize: 1024,
		Success:     false,
		Error:       errors.New("connection lost during publish"),
		Duration:    time.Millisecond * 500,
		Retries:     3,
	}

	publishLogger.LogNATSPublishEvent(ctx, publishEvent)

	// Simulate related consumer error
	consumeEvent := NATSConsumeEvent{
		Subject:         "INDEXING.jobs",
		MessageID:       "msg-error-001",
		MessageSize:     1024,
		Success:         false,
		Error:           errors.New("message processing timeout"),
		ProcessingTime:  time.Second * 30,
		ConsumerGroup:   "indexing-workers",
		DeliveryAttempt: 3,
		WillRetry:       false,
		Rejected:        true,
	}

	consumerLogger.LogNATSConsumeEvent(ctx, consumeEvent)

	// Verify both logs share same correlation ID
	publishOutput := getLoggerOutput(publishLogger)
	consumeOutput := getLoggerOutput(consumerLogger)

	var publishLogEntry, consumeLogEntry LogEntry
	err = json.Unmarshal([]byte(publishOutput), &publishLogEntry)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(consumeOutput), &consumeLogEntry)
	require.NoError(t, err)

	// Both errors should have same correlation ID for tracing
	assert.Equal(t, correlationID, publishLogEntry.CorrelationID)
	assert.Equal(t, correlationID, consumeLogEntry.CorrelationID)
	assert.Equal(t, "msg-error-001", publishLogEntry.Metadata["message_id"])
	assert.Equal(t, "msg-error-001", consumeLogEntry.Metadata["message_id"])
}

// Test helper - structures are now implemented in nats_logger.go
