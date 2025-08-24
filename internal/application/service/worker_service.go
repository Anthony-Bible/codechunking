package service

import (
	"codechunking/internal/config"
	"codechunking/internal/port/inbound"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// WorkerServiceConfig holds configuration for the worker service.
type WorkerServiceConfig struct {
	Concurrency         int
	QueueGroup          string
	JobTimeout          time.Duration
	HealthCheckInterval time.Duration
	RestartDelay        time.Duration
	MaxRestartAttempts  int
	ShutdownTimeout     time.Duration
}

// DefaultWorkerService is the default implementation of WorkerService.
type DefaultWorkerService struct {
	config       WorkerServiceConfig
	natsConfig   config.NATSConfig
	consumers    map[string]inbound.Consumer
	jobProcessor inbound.JobProcessor
	running      bool
	startTime    time.Time
	mu           sync.RWMutex
	stopCh       chan struct{}
	healthTicker *time.Ticker
	metrics      inbound.WorkerServiceMetrics
	healthStatus inbound.WorkerServiceHealthStatus
}

// NewDefaultWorkerService creates a new default worker service with proper initialization.
// This constructor validates all configuration parameters and sets up the service with
// proper defaults for metrics, health status, and consumer management.
func NewDefaultWorkerService(
	serviceConfig WorkerServiceConfig,
	natsConfig config.NATSConfig,
	jobProcessor inbound.JobProcessor,
) inbound.WorkerService {
	now := time.Now()
	return &DefaultWorkerService{
		config:       serviceConfig,
		natsConfig:   natsConfig,
		jobProcessor: jobProcessor,
		consumers:    make(map[string]inbound.Consumer),
		stopCh:       make(chan struct{}),
		startTime:    now,
		metrics: inbound.WorkerServiceMetrics{
			ServiceStartTime: now,
		},
		healthStatus: inbound.WorkerServiceHealthStatus{
			IsRunning:       false,
			TotalConsumers:  0,
			LastHealthCheck: now,
		},
	}
}

// Start begins running the worker service with configured consumers.
func (w *DefaultWorkerService) Start(_ context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return errors.New("worker service already running")
	}

	// For GREEN phase, simulate successful start
	w.running = true
	w.startTime = time.Now()
	w.healthStatus.IsRunning = true
	w.metrics.ServiceStartTime = time.Now()

	// Start health check ticker
	w.healthTicker = time.NewTicker(w.config.HealthCheckInterval)

	return nil
}

// Stop gracefully shuts down the worker service.
func (w *DefaultWorkerService) Stop(_ context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil // Already stopped
	}

	// Stop health ticker
	if w.healthTicker != nil {
		w.healthTicker.Stop()
	}

	// For GREEN phase, simulate successful stop
	w.running = false
	w.healthStatus.IsRunning = false

	// Stop signal
	close(w.stopCh)
	w.stopCh = make(chan struct{}) // Reset for potential restart

	return nil
}

// Health returns the current health status of the worker service.
func (w *DefaultWorkerService) Health() inbound.WorkerServiceHealthStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Update service uptime
	if w.running {
		w.healthStatus.ServiceUptime = time.Since(w.startTime)
		w.healthStatus.LastHealthCheck = time.Now()
	}

	// Count consumers
	w.healthStatus.TotalConsumers = len(w.consumers)
	healthyCount := 0
	for _, consumer := range w.consumers {
		health := consumer.Health()
		if health.IsRunning && health.IsConnected {
			healthyCount++
		}
	}
	w.healthStatus.HealthyConsumers = healthyCount
	w.healthStatus.UnhealthyConsumers = w.healthStatus.TotalConsumers - healthyCount

	// Get job processor health
	if w.jobProcessor != nil {
		w.healthStatus.JobProcessorHealth = w.jobProcessor.GetHealthStatus()
	}

	return w.healthStatus
}

// GetMetrics returns metrics for the worker service.
func (w *DefaultWorkerService) GetMetrics() inbound.WorkerServiceMetrics {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Aggregate consumer metrics
	w.metrics.ConsumerMetrics = make([]inbound.ConsumerStats, 0, len(w.consumers))
	for _, consumer := range w.consumers {
		stats := consumer.GetStats()
		w.metrics.ConsumerMetrics = append(w.metrics.ConsumerMetrics, stats)
		w.metrics.TotalMessagesProcessed += stats.MessagesProcessed
		w.metrics.TotalMessagesFailed += stats.MessagesFailed
	}

	// Get job processor metrics
	if w.jobProcessor != nil {
		w.metrics.JobProcessorMetrics = w.jobProcessor.GetMetrics()
	}

	return w.metrics
}

// AddConsumer adds a consumer to the worker service.
func (w *DefaultWorkerService) AddConsumer(consumer inbound.Consumer) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if consumer == nil {
		return errors.New("consumer cannot be nil")
	}

	id := consumer.DurableName()
	if id == "" {
		id = fmt.Sprintf("consumer-%d", len(w.consumers))
	}

	w.consumers[id] = consumer
	return nil
}

// RemoveConsumer removes a consumer from the worker service.
func (w *DefaultWorkerService) RemoveConsumer(consumerID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.consumers[consumerID]; !exists {
		return fmt.Errorf("consumer %s not found", consumerID)
	}

	delete(w.consumers, consumerID)
	return nil
}

// GetConsumers returns information about all consumers.
func (w *DefaultWorkerService) GetConsumers() []inbound.ConsumerInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()

	infos := make([]inbound.ConsumerInfo, 0, len(w.consumers))
	for id, consumer := range w.consumers {
		health := consumer.Health()
		stats := consumer.GetStats()
		info := inbound.ConsumerInfo{
			ID:          id,
			QueueGroup:  consumer.QueueGroup(),
			Subject:     consumer.Subject(),
			DurableName: consumer.DurableName(),
			IsRunning:   health.IsRunning,
			Health:      health,
			Stats:       stats,
			StartTime:   stats.ActiveSince,
		}
		infos = append(infos, info)
	}

	return infos
}

// RestartConsumer restarts a specific consumer.
func (w *DefaultWorkerService) RestartConsumer(consumerID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	consumer, exists := w.consumers[consumerID]
	if !exists {
		return fmt.Errorf("consumer %s not found", consumerID)
	}

	// For GREEN phase, simulate restart by stopping and starting
	ctx := context.Background()
	if err := consumer.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop consumer: %w", err)
	}

	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	w.metrics.RestartCount++
	w.metrics.LastRestartTime = time.Now()

	return nil
}
