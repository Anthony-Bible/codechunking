package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// natsShutdownManager implements NATSShutdownManager interface.
type natsShutdownManager struct {
	consumers map[string]NATSConsumer
	status    []NATSConsumerStatus
	metrics   NATSShutdownMetrics
	mu        sync.RWMutex
}

// NewNATSShutdownManager creates a new NATSShutdownManager instance.
func NewNATSShutdownManager() NATSShutdownManager {
	return &natsShutdownManager{
		consumers: make(map[string]NATSConsumer),
		status:    make([]NATSConsumerStatus, 0),
		metrics: NATSShutdownMetrics{
			ConsumerMetrics: make(map[string]NATSConsumerMetrics),
		},
	}
}

// RegisterConsumer registers a NATS consumer for shutdown management.
func (n *natsShutdownManager) RegisterConsumer(name string, consumer NATSConsumer) error {
	if name == "" {
		return errors.New("consumer name cannot be empty")
	}
	if consumer == nil {
		return errors.New("consumer cannot be nil")
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.consumers[name]; exists {
		return fmt.Errorf("consumer already registered: %s", name)
	}

	n.consumers[name] = consumer

	// Initialize status
	info := consumer.GetConsumerInfo()
	status := NATSConsumerStatus{
		Name:              name,
		Subject:           info.Subject,
		QueueGroup:        info.QueueGroup,
		Status:            "active",
		PendingMessages:   consumer.GetPendingMessages(),
		ProcessedMessages: consumer.GetProcessedMessages(),
		LastMessageTime:   time.Now(),
		IsConnected:       consumer.IsConnected(),
	}

	n.status = append(n.status, status)
	n.metrics.TotalConsumers = len(n.consumers)

	return nil
}

// DrainAllConsumers gracefully drains all registered NATS consumers.
func (n *natsShutdownManager) DrainAllConsumers(ctx context.Context) error {
	n.mu.RLock()
	consumers := make(map[string]NATSConsumer)
	for name, consumer := range n.consumers {
		consumers[name] = consumer
	}
	n.mu.RUnlock()

	for name, consumer := range consumers {
		n.updateConsumerStatus(name, "draining", "")

		drainCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := consumer.Drain(drainCtx, 10*time.Second); err != nil {
			cancel()
			n.updateConsumerStatus(name, "error", err.Error())
			n.mu.Lock()
			n.metrics.FailedDrains++
			n.mu.Unlock()
			continue
		}
		cancel()

		n.updateConsumerStatus(name, "drained", "")
		n.mu.Lock()
		n.metrics.SuccessfulDrains++
		n.mu.Unlock()
	}

	return nil
}

// StopConsumers stops all consumers without draining.
func (n *natsShutdownManager) StopConsumers(ctx context.Context) error {
	n.mu.RLock()
	consumers := make(map[string]NATSConsumer)
	for name, consumer := range n.consumers {
		consumers[name] = consumer
	}
	n.mu.RUnlock()

	for name, consumer := range consumers {
		n.updateConsumerStatus(name, "stopping", "")

		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := consumer.Stop(stopCtx); err != nil {
			cancel()
			n.updateConsumerStatus(name, "error", err.Error())
			continue
		}
		cancel()

		n.updateConsumerStatus(name, "stopped", "")
	}

	return nil
}

// FlushPendingMessages ensures all pending messages are processed or acknowledged.
func (n *natsShutdownManager) FlushPendingMessages(ctx context.Context) error {
	n.mu.RLock()
	consumers := make(map[string]NATSConsumer)
	for name, consumer := range n.consumers {
		consumers[name] = consumer
	}
	n.mu.RUnlock()

	for name, consumer := range consumers {
		flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := consumer.FlushPending(flushCtx); err != nil {
			cancel()
			n.updateConsumerStatus(name, "error", err.Error())
			continue
		}
		cancel()

		// Update metrics
		n.mu.Lock()
		if consumerMetrics, exists := n.metrics.ConsumerMetrics[name]; exists {
			consumerMetrics.MessagesDrained = consumer.GetProcessedMessages()
			n.metrics.ConsumerMetrics[name] = consumerMetrics
		}
		n.metrics.TotalMessagesDrained += consumer.GetProcessedMessages()
		n.mu.Unlock()
	}

	return nil
}

// CloseConnections closes all NATS connections.
func (n *natsShutdownManager) CloseConnections(ctx context.Context) error {
	n.mu.RLock()
	consumers := make(map[string]NATSConsumer)
	for name, consumer := range n.consumers {
		consumers[name] = consumer
	}
	n.mu.RUnlock()

	for name, consumer := range consumers {
		n.updateConsumerStatus(name, "closing", "")

		if err := consumer.Close(); err != nil {
			n.updateConsumerStatus(name, "error", err.Error())
			n.mu.Lock()
			n.metrics.ForceStopCount++
			n.mu.Unlock()
			continue
		}

		n.updateConsumerStatus(name, "closed", "")
		n.mu.Lock()
		n.metrics.SuccessfulDrains++
		n.mu.Unlock()
	}

	n.mu.Lock()
	n.metrics.LastShutdownTime = time.Now()
	n.mu.Unlock()

	return nil
}

// GetConsumerStatus returns the current status of all consumers.
func (n *natsShutdownManager) GetConsumerStatus() []NATSConsumerStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Create a copy to avoid race conditions
	status := make([]NATSConsumerStatus, len(n.status))
	copy(status, n.status)
	return status
}

// GetShutdownMetrics returns metrics about NATS shutdown performance.
func (n *natsShutdownManager) GetShutdownMetrics() NATSShutdownMetrics {
	n.mu.RLock()
	defer n.mu.RUnlock()

	metrics := n.metrics

	// Calculate average drain time
	if n.metrics.SuccessfulDrains > 0 {
		totalDrainTime := time.Duration(0)
		count := 0
		for _, consumerMetrics := range n.metrics.ConsumerMetrics {
			if consumerMetrics.DrainTime > 0 {
				totalDrainTime += consumerMetrics.DrainTime
				count++
			}
		}
		if count > 0 {
			metrics.AverageDrainTime = totalDrainTime / time.Duration(count)
		}
	}

	return metrics
}

// updateConsumerStatus updates the status of a specific consumer.
func (n *natsShutdownManager) updateConsumerStatus(name, status, errorMsg string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, consumerStatus := range n.status {
		if consumerStatus.Name == name {
			n.status[i].Status = status
			n.status[i].Error = errorMsg

			if status == "draining" && n.status[i].DrainStartTime.IsZero() {
				n.status[i].DrainStartTime = time.Now()
			}

			if (status == "drained" || status == "stopped" || status == "closed") &&
				!n.status[i].DrainStartTime.IsZero() {
				n.status[i].DrainDuration = time.Since(n.status[i].DrainStartTime)

				// Update consumer metrics
				consumerMetrics, exists := n.metrics.ConsumerMetrics[name]
				if !exists {
					consumerMetrics = NATSConsumerMetrics{Name: name}
				}
				consumerMetrics.DrainTime = n.status[i].DrainDuration
				consumerMetrics.DrainAttempts++
				if status != "error" {
					consumerMetrics.SuccessfulDrains++
				}
				n.metrics.ConsumerMetrics[name] = consumerMetrics
			}

			break
		}
	}
}
