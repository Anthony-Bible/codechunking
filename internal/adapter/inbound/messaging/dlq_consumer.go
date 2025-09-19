package messaging

import (
	"codechunking/internal/config"
	"codechunking/internal/domain/messaging"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// DLQMessageHandler interface for handling DLQ messages.
type DLQMessageHandler interface {
	ProcessDLQMessage(ctx context.Context, dlqMessage messaging.DLQMessage) error
	AnalyzeDLQMessage(ctx context.Context, dlqMessage messaging.DLQMessage) (messaging.FailurePattern, error)
	GetDLQMessageHistory(ctx context.Context, messageID string) ([]messaging.DLQMessage, error)
}

// DLQRetryService interface for retrying DLQ messages.
type DLQRetryService interface {
	RetryMessage(ctx context.Context, dlqMessageID string) error
	BulkRetryMessages(ctx context.Context, messageIDs []string) error
	CanRetry(ctx context.Context, dlqMessage messaging.DLQMessage) bool
}

// DLQStatisticsCollector interface for collecting DLQ statistics.
type DLQStatisticsCollector interface {
	RecordDLQMessage(ctx context.Context, dlqMessage messaging.DLQMessage) error
	GetStatistics(ctx context.Context) (messaging.DLQStatistics, error)
	GetFailurePatterns(ctx context.Context) ([]messaging.FailurePattern, error)
}

// DLQConsumerConfig holds configuration for the DLQ consumer.
type DLQConsumerConfig struct {
	Subject              string
	DurableName          string
	AckWait              time.Duration
	MaxDeliver           int
	MaxAckPending        int
	ReplayPolicy         string
	DeliverPolicy        string
	ProcessingTimeout    time.Duration
	MaxProcessingWorkers int
	EnableAnalysis       bool
	EnableRetry          bool
}

// DLQConsumerHealth represents the health status of DLQ consumer.
type DLQConsumerHealth struct {
	IsRunning         bool  `json:"is_running"`
	IsConnected       bool  `json:"is_connected"`
	MessagesProcessed int64 `json:"messages_processed"`
	ProcessingErrors  int64 `json:"processing_errors"`
}

// DLQConsumer handles consuming messages from the dead letter queue.
type DLQConsumer struct {
	config         DLQConsumerConfig
	natsConfig     config.NATSConfig
	dlqHandler     DLQMessageHandler
	retryService   DLQRetryService
	statsCollector DLQStatisticsCollector
	running        bool
	connected      bool
}

// NewDLQConsumer creates a new DLQ consumer.
func NewDLQConsumer(
	config DLQConsumerConfig,
	natsConfig config.NATSConfig,
	dlqHandler DLQMessageHandler,
	retryService DLQRetryService,
	statsCollector DLQStatisticsCollector,
) (*DLQConsumer, error) {
	// Validate configuration
	if err := validateDLQConsumerConfig(config); err != nil {
		return nil, err
	}

	// Validate required dependencies
	if dlqHandler == nil {
		return nil, errors.New("DLQ message handler cannot be nil")
	}

	consumer := &DLQConsumer{
		config:         config,
		natsConfig:     natsConfig,
		dlqHandler:     dlqHandler,
		retryService:   retryService,
		statsCollector: statsCollector,
		running:        false,
		connected:      false,
	}

	return consumer, nil
}

// validateDLQConsumerConfig validates the DLQ consumer configuration.
func validateDLQConsumerConfig(config DLQConsumerConfig) error {
	if config.Subject == "" {
		return errors.New("subject cannot be empty")
	}
	if config.DurableName == "" {
		return errors.New("durable name cannot be empty")
	}
	if config.MaxProcessingWorkers <= 0 {
		return errors.New("max processing workers must be positive")
	}
	if config.ProcessingTimeout <= 0 {
		return errors.New("processing timeout must be positive")
	}
	return nil
}

// Start starts the DLQ consumer.
func (c *DLQConsumer) Start(ctx context.Context) error {
	if c.running {
		return errors.New("DLQ consumer is already running")
	}

	// Mark as running
	c.running = true
	c.connected = true

	return nil
}

// Stop stops the DLQ consumer.
func (c *DLQConsumer) Stop(ctx context.Context) error {
	if !c.running {
		return nil // Already stopped
	}

	// Mark as stopped
	c.running = false
	c.connected = false

	return nil
}

// Health returns the health status of the DLQ consumer.
func (c *DLQConsumer) Health() DLQConsumerHealth {
	return DLQConsumerHealth{
		IsRunning:         c.running,
		IsConnected:       c.connected,
		MessagesProcessed: 0, // Would track actual metrics in production
		ProcessingErrors:  0, // Would track actual metrics in production
	}
}

// GetStats returns DLQ consumer statistics.
func (c *DLQConsumer) GetStats(ctx context.Context) (messaging.DLQStatistics, error) {
	if c.statsCollector == nil {
		return messaging.DLQStatistics{}, errors.New("statistics collector not available")
	}
	return c.statsCollector.GetStatistics(ctx)
}

// ListMessages returns a list of messages currently in the DLQ.
func (c *DLQConsumer) ListMessages(ctx context.Context, offset int, limit int) ([]messaging.DLQMessage, error) {
	// In a real implementation, this would query the DLQ storage
	// For now, return empty list for GREEN phase
	return []messaging.DLQMessage{}, nil
}

// GetMessage retrieves a specific DLQ message by ID.
func (c *DLQConsumer) GetMessage(ctx context.Context, messageID string) (messaging.DLQMessage, error) {
	if messageID == "" {
		return messaging.DLQMessage{}, errors.New("message ID cannot be empty")
	}
	// In a real implementation, this would query the DLQ storage by ID
	// For now, return empty message for GREEN phase
	return messaging.DLQMessage{}, errors.New("message not found")
}

// RetryMessage attempts to retry a specific DLQ message.
func (c *DLQConsumer) RetryMessage(ctx context.Context, messageID string) error {
	if messageID == "" {
		return errors.New("message ID cannot be empty")
	}
	if c.retryService == nil {
		return errors.New("retry service not available")
	}
	return c.retryService.RetryMessage(ctx, messageID)
}

// RequeueMessage requeues a DLQ message back to the original processing queue.
func (c *DLQConsumer) RequeueMessage(ctx context.Context, messageID string) error {
	if messageID == "" {
		return errors.New("message ID cannot be empty")
	}
	// In a real implementation, this would move the message back to the original queue
	// For now, just delegate to retry service
	if c.retryService == nil {
		return errors.New("retry service not available")
	}
	return c.retryService.RetryMessage(ctx, messageID)
}

// DeleteMessage permanently deletes a DLQ message.
func (c *DLQConsumer) DeleteMessage(ctx context.Context, messageID string) error {
	if messageID == "" {
		return errors.New("message ID cannot be empty")
	}
	// In a real implementation, this would permanently remove the message from DLQ storage
	// For now, just return success for GREEN phase
	return nil
}

// GetFailurePatterns returns collected failure patterns.
func (c *DLQConsumer) GetFailurePatterns(ctx context.Context) ([]messaging.FailurePattern, error) {
	if c.statsCollector == nil {
		return nil, errors.New("statistics collector not available")
	}
	return c.statsCollector.GetFailurePatterns(ctx)
}

// handleDLQMessage processes a NATS message from the DLQ.
func (c *DLQConsumer) handleDLQMessage(msg *nats.Msg) error {
	if msg == nil {
		return errors.New("received nil message")
	}

	// Deserialize DLQ message
	var dlqMessage messaging.DLQMessage
	if err := json.Unmarshal(msg.Data, &dlqMessage); err != nil {
		return fmt.Errorf("failed to unmarshal DLQ message: %w", err)
	}

	// Process the DLQ message using the handler
	ctx := context.Background()
	if err := c.dlqHandler.ProcessDLQMessage(ctx, dlqMessage); err != nil {
		return fmt.Errorf("failed to process DLQ message: %w", err)
	}

	// Record statistics if collector is available
	if c.statsCollector != nil {
		_ = c.statsCollector.RecordDLQMessage(ctx, dlqMessage)
	}

	return nil
}

// AnalyzeDLQMessage analyzes a DLQ message for failure patterns.
func (c *DLQConsumer) AnalyzeDLQMessage(
	ctx context.Context,
	dlqMessage messaging.DLQMessage,
) (messaging.FailurePattern, error) {
	if c.dlqHandler == nil {
		return messaging.FailurePattern{}, errors.New("DLQ handler not available")
	}
	return c.dlqHandler.AnalyzeDLQMessage(ctx, dlqMessage)
}

// GetMessageHistory retrieves the history of a DLQ message.
func (c *DLQConsumer) GetMessageHistory(ctx context.Context, messageID string) ([]messaging.DLQMessage, error) {
	if messageID == "" {
		return nil, errors.New("message ID cannot be empty")
	}
	if c.dlqHandler == nil {
		return nil, errors.New("DLQ handler not available")
	}
	return c.dlqHandler.GetDLQMessageHistory(ctx, messageID)
}

// MaxWorkers returns the maximum number of processing workers configured.
func (c *DLQConsumer) MaxWorkers() int {
	return c.config.MaxProcessingWorkers
}

// BulkRetryMessages attempts to retry multiple DLQ messages.
func (c *DLQConsumer) BulkRetryMessages(ctx context.Context, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return errors.New("message IDs cannot be empty")
	}
	if c.retryService == nil {
		return errors.New("retry service not available")
	}
	return c.retryService.BulkRetryMessages(ctx, messageIDs)
}

// GetStatistics returns DLQ consumer statistics.
func (c *DLQConsumer) GetStatistics(ctx context.Context) (messaging.DLQStatistics, error) {
	if c.statsCollector == nil {
		return messaging.DLQStatistics{}, errors.New("statistics collector not available")
	}
	return c.statsCollector.GetStatistics(ctx)
}
