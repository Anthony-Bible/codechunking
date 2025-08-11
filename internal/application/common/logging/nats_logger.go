package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// NATS-specific event structures
type NATSConnectionEvent struct {
	Type         string // CONNECTED, DISCONNECTED, RECONNECTING, CONNECTION_FAILED
	ServerURL    string
	ConnectionID string
	AttemptCount int
	Duration     time.Duration
	Success      bool
	Error        error
	Reason       string // For disconnections
}

type NATSPublishEvent struct {
	Subject        string
	MessageID      string
	MessageSize    int64
	QueueName      string
	Duration       time.Duration
	Success        bool
	Error          error
	DeliveryPolicy string
	Retries        int
}

type NATSConsumeEvent struct {
	Subject         string
	MessageID       string
	MessageSize     int64
	ProcessingTime  time.Duration
	AckTime         time.Duration
	Success         bool
	Error           error
	ConsumerGroup   string
	DeliveryAttempt int
	QueuedTime      time.Duration
	WillRetry       bool
	NextRetryAt     time.Time
	Rejected        bool
}

type JetStreamEvent struct {
	Operation      string
	StreamName     string
	ConsumerName   string
	Success        bool
	Duration       time.Duration
	Error          error
	StreamConfig   map[string]interface{}
	ConsumerConfig map[string]interface{}
	StreamInfo     *StreamInfo
}

type StreamInfo struct {
	Messages  uint64
	Bytes     uint64
	Consumers int
	Subjects  []string
}

type NATSPerformanceMetrics struct {
	PublishRate     float64
	ConsumeRate     float64
	AverageLatency  time.Duration
	MaxLatency      time.Duration
	ErrorRate       float64
	TotalMessages   uint64
	TotalErrors     uint64
	ConnectionCount int
	ActiveConsumers int
	QueueDepth      int
	MemoryUsage     string
	TimeWindow      time.Duration
}

// NATSApplicationLogger extends ApplicationLogger with NATS operations
type NATSApplicationLogger interface {
	ApplicationLogger
	LogNATSConnectionEvent(ctx context.Context, event NATSConnectionEvent)
	LogNATSPublishEvent(ctx context.Context, event NATSPublishEvent)
	LogNATSConsumeEvent(ctx context.Context, event NATSConsumeEvent)
	LogJetStreamEvent(ctx context.Context, event JetStreamEvent)
	LogNATSPerformanceMetrics(ctx context.Context, metrics NATSPerformanceMetrics)
}

// NATS logger implementation using composition
type natsApplicationLogger struct {
	ApplicationLogger
}

// NewNATSApplicationLogger creates a NATS-enabled application logger
func NewNATSApplicationLogger(base ApplicationLogger) NATSApplicationLogger {
	return &natsApplicationLogger{
		ApplicationLogger: base,
	}
}

// LogNATSConnectionEvent logs NATS connection events
func (n *natsApplicationLogger) LogNATSConnectionEvent(ctx context.Context, event NATSConnectionEvent) {
	fields := Fields{
		"event_type":    event.Type,
		"server_url":    event.ServerURL,
		"connection_id": event.ConnectionID,
		"attempt_count": event.AttemptCount,
		"success":       event.Success,
	}

	if event.Duration > 0 {
		fields["duration"] = event.Duration.String()
	}
	if event.Reason != "" {
		fields["reason"] = event.Reason
	}

	message := fmt.Sprintf("NATS connection event: %s", event.Type)
	operation := "nats_connection"

	// Set the operation in the logger
	if appLogger, ok := n.ApplicationLogger.(*applicationLoggerImpl); ok {
		originalComponent := appLogger.component
		defer func() { appLogger.component = originalComponent }()

		// Create a new log entry with operation
		correlationID := getOrGenerateCorrelationID(ctx)
		entry := LogEntry{
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			CorrelationID: correlationID,
			Component:     appLogger.component,
			Operation:     operation,
			Metadata:      make(map[string]interface{}),
			Context:       make(map[string]interface{}),
		}

		// Add fields to metadata
		for key, value := range fields {
			entry.Metadata[key] = value
		}

		// Determine log level based on success/error
		var level string
		var errorStr string
		if !event.Success && event.Error != nil {
			level = "ERROR"
			errorStr = event.Error.Error()
			message = fmt.Sprintf("NATS connection failed: %s", event.Type)
		} else if event.Type == "RECONNECTING" {
			level = "WARN"
			message = fmt.Sprintf("NATS reconnecting: %s", event.Type)
		} else {
			level = "INFO"
		}

		entry.Level = level
		entry.Message = message
		entry.Error = errorStr

		// Output log entry
		if appLogger.config.Format == "json" {
			jsonData, _ := json.Marshal(entry)
			appLogger.logger.Println(string(jsonData))
		}
	}
}

// LogNATSPublishEvent logs NATS message publishing events
func (n *natsApplicationLogger) LogNATSPublishEvent(ctx context.Context, event NATSPublishEvent) {
	fields := Fields{
		"subject":      event.Subject,
		"message_id":   event.MessageID,
		"message_size": event.MessageSize,
		"success":      event.Success,
		"duration":     event.Duration.String(),
		"retries":      event.Retries,
	}

	if event.QueueName != "" {
		fields["queue_name"] = event.QueueName
	}
	if event.DeliveryPolicy != "" {
		fields["delivery_policy"] = event.DeliveryPolicy
	}

	operation := "nats_publish"
	message := "NATS message published"
	level := "DEBUG"

	if event.MessageSize > 1024*512 { // Large message threshold
		level = "INFO"
		message = "NATS large message published"
	}

	if !event.Success {
		level = "ERROR"
		message = "NATS message publish failed"
	}

	// Create structured log entry
	n.logNATSEntry(ctx, level, message, operation, fields, event.Error)
}

// LogNATSConsumeEvent logs NATS message consumption events
func (n *natsApplicationLogger) LogNATSConsumeEvent(ctx context.Context, event NATSConsumeEvent) {
	fields := Fields{
		"subject":          event.Subject,
		"message_id":       event.MessageID,
		"message_size":     event.MessageSize,
		"processing_time":  event.ProcessingTime.String(),
		"success":          event.Success,
		"consumer_group":   event.ConsumerGroup,
		"delivery_attempt": event.DeliveryAttempt,
		"queued_time":      event.QueuedTime.String(),
	}

	if event.AckTime > 0 {
		fields["ack_time"] = event.AckTime.String()
	}
	if event.WillRetry {
		fields["will_retry"] = true
		fields["next_retry_at"] = event.NextRetryAt.Format(time.RFC3339)
	}
	if event.Rejected {
		fields["rejected"] = true
	}

	operation := "nats_consume"
	message := "NATS message consumed"
	level := "DEBUG"

	if event.Rejected {
		level = "WARN"
		message = "NATS message rejected"
	} else if !event.Success {
		level = "ERROR"
		message = "NATS message processing failed"
	}

	n.logNATSEntry(ctx, level, message, operation, fields, event.Error)
}

// LogJetStreamEvent logs JetStream-specific operations
func (n *natsApplicationLogger) LogJetStreamEvent(ctx context.Context, event JetStreamEvent) {
	fields := Fields{
		"js_operation": event.Operation,
		"stream_name":  event.StreamName,
		"success":      event.Success,
		"duration":     event.Duration.String(),
	}

	if event.ConsumerName != "" {
		fields["consumer_name"] = event.ConsumerName
	}
	if event.StreamConfig != nil {
		fields["stream_config"] = event.StreamConfig
	}
	if event.ConsumerConfig != nil {
		fields["consumer_config"] = event.ConsumerConfig
	}
	if event.StreamInfo != nil {
		fields["stream_info"] = map[string]interface{}{
			"messages":  event.StreamInfo.Messages,
			"bytes":     event.StreamInfo.Bytes,
			"consumers": event.StreamInfo.Consumers,
			"subjects":  event.StreamInfo.Subjects,
		}
	}

	operation := "jetstream_operation"
	message := fmt.Sprintf("JetStream operation: %s", event.Operation)
	level := "INFO"

	if event.Operation == "stream_info" {
		level = "DEBUG"
	} else if !event.Success {
		level = "ERROR"
		message = fmt.Sprintf("JetStream operation failed: %s", event.Operation)
	}

	n.logNATSEntry(ctx, level, message, operation, fields, event.Error)
}

// LogNATSPerformanceMetrics logs NATS performance metrics
func (n *natsApplicationLogger) LogNATSPerformanceMetrics(ctx context.Context, metrics NATSPerformanceMetrics) {
	fields := Fields{
		"publish_rate":     metrics.PublishRate,
		"consume_rate":     metrics.ConsumeRate,
		"average_latency":  metrics.AverageLatency.String(),
		"max_latency":      metrics.MaxLatency.String(),
		"error_rate":       metrics.ErrorRate,
		"total_messages":   metrics.TotalMessages,
		"total_errors":     metrics.TotalErrors,
		"connection_count": metrics.ConnectionCount,
		"active_consumers": metrics.ActiveConsumers,
		"queue_depth":      metrics.QueueDepth,
		"memory_usage":     metrics.MemoryUsage,
		"time_window":      metrics.TimeWindow.String(),
	}

	operation := "nats_performance"
	message := "NATS performance metrics recorded"
	level := "INFO"

	n.logNATSEntry(ctx, level, message, operation, fields, nil)
}

// Helper function to create NATS log entries
func (n *natsApplicationLogger) logNATSEntry(ctx context.Context, level, message, operation string, fields Fields, err error) {
	if appLogger, ok := n.ApplicationLogger.(*applicationLoggerImpl); ok {
		correlationID := getOrGenerateCorrelationID(ctx)
		entry := LogEntry{
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			Level:         level,
			Message:       message,
			CorrelationID: correlationID,
			Component:     appLogger.component,
			Operation:     operation,
			Metadata:      make(map[string]interface{}),
			Context:       make(map[string]interface{}),
		}

		// Add fields to metadata
		for key, value := range fields {
			entry.Metadata[key] = value
		}

		// Add error if present
		if err != nil {
			entry.Error = err.Error()
		}

		// Add context information
		if requestID := getRequestIDFromContext(ctx); requestID != "" {
			entry.Context["request_id"] = requestID
		}

		// Output log entry
		if appLogger.config.Format == "json" {
			jsonData, _ := json.Marshal(entry)
			appLogger.logger.Println(string(jsonData))
		}
	}
}
