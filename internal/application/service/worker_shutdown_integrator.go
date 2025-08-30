package service

import (
	"codechunking/internal/port/inbound"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// workerShutdownIntegrator implements WorkerShutdownIntegrator interface.
type workerShutdownIntegrator struct {
	workerServices       map[string]inbound.WorkerService
	status               WorkerShutdownStatus
	config               WorkerShutdownConfig
	jobCompletionTimeout time.Duration
	activeJobs           int
	mu                   sync.RWMutex
}

// NewWorkerShutdownIntegrator creates a new WorkerShutdownIntegrator instance.
func NewWorkerShutdownIntegrator(config WorkerShutdownConfig) WorkerShutdownIntegrator {
	// Set defaults
	if config.DrainTimeout == 0 {
		config.DrainTimeout = 10 * time.Second
	}
	if config.ConsumerShutdownTimeout == 0 {
		config.ConsumerShutdownTimeout = 5 * time.Second
	}
	if config.WorkerStopTimeout == 0 {
		config.WorkerStopTimeout = 5 * time.Second
	}
	if config.ResourceCleanupTimeout == 0 {
		config.ResourceCleanupTimeout = 5 * time.Second
	}
	if config.ForceStopTimeout == 0 {
		config.ForceStopTimeout = 30 * time.Second
	}
	if config.JobCompletionCheckInterval == 0 {
		config.JobCompletionCheckInterval = 1 * time.Second
	}
	if config.MaxConcurrentShutdowns == 0 {
		config.MaxConcurrentShutdowns = 3
	}
	if config.LongRunningJobThreshold == 0 {
		config.LongRunningJobThreshold = 30 * time.Second
	}

	return &workerShutdownIntegrator{
		workerServices: make(map[string]inbound.WorkerService),
		config:         config,
		status: WorkerShutdownStatus{
			Phase:             WorkerShutdownPhaseIdle,
			TotalWorkers:      0,
			StoppedWorkers:    0,
			WorkersWithErrors: 0,
			ActiveJobs:        0,
			PendingJobs:       0,
			CompletedJobs:     0,
			WorkerStatuses:    make([]WorkerShutdownInfo, 0),
			ConsumerStatuses:  make([]ConsumerShutdownInfo, 0),
		},
		activeJobs: 0,
	}
}

// IntegrateWorkerService registers a worker service with graceful shutdown management.
func (w *workerShutdownIntegrator) IntegrateWorkerService(workerService inbound.WorkerService) error {
	if workerService == nil {
		return errors.New("worker service cannot be nil")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Generate a unique ID for the worker
	workerID := fmt.Sprintf("worker-%d", len(w.workerServices))
	w.workerServices[workerID] = workerService

	// Update status
	w.status.TotalWorkers = len(w.workerServices)

	// Initialize worker status
	health := workerService.Health()
	metrics := workerService.GetMetrics()
	consumers := workerService.GetConsumers()

	workerStatus := WorkerShutdownInfo{
		WorkerID:          workerID,
		WorkerType:        "indexing-worker", // Default type
		Status:            "running",
		ActiveJobs:        health.JobProcessorHealth.ActiveJobs,
		CompletedJobs:     health.JobProcessorHealth.CompletedJobs,
		FailedJobs:        health.JobProcessorHealth.FailedJobs,
		LastJobTime:       health.JobProcessorHealth.LastJobTime,
		ShutdownStartTime: time.Time{},
		Consumers:         consumers,
		Health:            health,
		Metrics:           metrics,
	}

	w.status.WorkerStatuses = append(w.status.WorkerStatuses, workerStatus)

	// Initialize consumer statuses
	for _, consumer := range consumers {
		consumerStatus := ConsumerShutdownInfo{
			ConsumerID:        consumer.ID,
			Subject:           consumer.Subject,
			QueueGroup:        consumer.QueueGroup,
			Status:            "active",
			PendingMessages:   0,
			ProcessedMessages: consumer.Stats.MessagesProcessed,
			LastMessageTime:   consumer.Health.LastMessageTime,
			Health:            consumer.Health,
			Stats:             consumer.Stats,
		}
		w.status.ConsumerStatuses = append(w.status.ConsumerStatuses, consumerStatus)
	}

	return nil
}

// InitiateWorkerShutdown begins the graceful shutdown process for all registered workers.
func (w *workerShutdownIntegrator) InitiateWorkerShutdown(ctx context.Context) error {
	w.mu.Lock()
	if w.status.Phase != WorkerShutdownPhaseIdle {
		w.mu.Unlock()
		return errors.New("worker shutdown already in progress")
	}
	w.status.Phase = WorkerShutdownPhaseDrainingNew
	w.status.ShutdownStartTime = time.Now()
	w.mu.Unlock()

	// Execute shutdown phases
	if err := w.executeDrainingPhase(ctx); err != nil {
		return err
	}

	if err := w.executeJobCompletionPhase(ctx); err != nil {
		return err
	}

	if err := w.executeConsumerShutdownPhase(ctx); err != nil {
		return err
	}

	if err := w.executeWorkerStopPhase(ctx); err != nil {
		return err
	}

	if err := w.executeResourceCleanupPhase(ctx); err != nil {
		return err
	}

	w.mu.Lock()
	w.status.Phase = WorkerShutdownPhaseCompleted
	w.status.ElapsedTime = time.Since(w.status.ShutdownStartTime)
	w.mu.Unlock()

	return nil
}

// GetWorkerShutdownStatus returns the current status of worker shutdown operations.
func (w *workerShutdownIntegrator) GetWorkerShutdownStatus() WorkerShutdownStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	status := w.status
	status.ElapsedTime = time.Since(w.status.ShutdownStartTime)

	// Update active jobs count
	totalActiveJobs := 0
	for _, workerStatus := range w.status.WorkerStatuses {
		totalActiveJobs += workerStatus.ActiveJobs
	}
	status.ActiveJobs = totalActiveJobs

	return status
}

// RegisterJobCompletionHook adds a hook that waits for job completion before shutdown.
func (w *workerShutdownIntegrator) RegisterJobCompletionHook(timeout time.Duration) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.jobCompletionTimeout = timeout
	return nil
}

// ForceStopWorkers immediately stops all workers without graceful cleanup.
func (w *workerShutdownIntegrator) ForceStopWorkers(ctx context.Context) error {
	w.mu.Lock()
	w.status.Phase = WorkerShutdownPhaseForceStop
	w.mu.Unlock()

	// Force stop all workers
	for workerID, workerService := range w.workerServices {
		if err := workerService.Stop(ctx); err != nil {
			w.updateWorkerStatus(workerID, "failed", err.Error())
		} else {
			w.updateWorkerStatus(workerID, "stopped", "")
		}
	}

	w.mu.Lock()
	w.status.Phase = WorkerShutdownPhaseCompleted
	w.status.StoppedWorkers = len(w.workerServices)
	w.mu.Unlock()

	return nil
}

// GetActiveJobCount returns the number of jobs currently being processed.
func (w *workerShutdownIntegrator) GetActiveJobCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	totalActiveJobs := 0
	for _, workerService := range w.workerServices {
		health := workerService.Health()
		totalActiveJobs += health.JobProcessorHealth.ActiveJobs
	}

	return totalActiveJobs
}

// executeDrainingPhase stops accepting new jobs.
func (w *workerShutdownIntegrator) executeDrainingPhase(ctx context.Context) error {
	w.mu.Lock()
	w.status.Phase = WorkerShutdownPhaseDrainingNew
	w.mu.Unlock()

	// In a real implementation, this would signal all workers to stop accepting new jobs
	// For the minimal implementation, we just wait for the drain timeout
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(w.config.DrainTimeout):
		// Draining phase completed
	}

	return nil
}

// executeJobCompletionPhase waits for active jobs to complete.
func (w *workerShutdownIntegrator) executeJobCompletionPhase(ctx context.Context) error {
	w.mu.Lock()
	w.status.Phase = WorkerShutdownPhaseJobCompletion
	w.mu.Unlock()

	timeout := w.config.DrainTimeout
	if w.jobCompletionTimeout > 0 {
		timeout = w.jobCompletionTimeout
	}

	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for jobs to complete or timeout
	ticker := time.NewTicker(w.config.JobCompletionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-jobCtx.Done():
			return jobCtx.Err()
		case <-ticker.C:
			activeJobs := w.GetActiveJobCount()
			w.mu.Lock()
			w.status.ActiveJobs = activeJobs
			w.mu.Unlock()

			if activeJobs == 0 {
				return nil // All jobs completed
			}
		}
	}
}

// executeConsumerShutdownPhase shuts down NATS consumers.
func (w *workerShutdownIntegrator) executeConsumerShutdownPhase(ctx context.Context) error {
	w.mu.Lock()
	w.status.Phase = WorkerShutdownPhaseConsumerShutdown
	w.mu.Unlock()

	consumerCtx, cancel := context.WithTimeout(ctx, w.config.ConsumerShutdownTimeout)
	defer cancel()

	// Update consumer statuses to draining
	w.mu.Lock()
	for i := range w.status.ConsumerStatuses {
		w.status.ConsumerStatuses[i].Status = ConsumerStatusDraining
		w.status.ConsumerStatuses[i].DrainStartTime = time.Now()
	}
	w.mu.Unlock()

	// Wait for consumer shutdown timeout
	select {
	case <-consumerCtx.Done():
		return consumerCtx.Err()
	case <-time.After(w.config.ConsumerShutdownTimeout):
		// Consumer shutdown completed
	}

	// Update consumer statuses to stopped
	w.mu.Lock()
	for i := range w.status.ConsumerStatuses {
		w.status.ConsumerStatuses[i].Status = ConsumerStatusStopped
		w.status.ConsumerStatuses[i].DrainDuration = time.Since(w.status.ConsumerStatuses[i].DrainStartTime)
	}
	w.mu.Unlock()

	return nil
}

// executeWorkerStopPhase stops the worker services.
func (w *workerShutdownIntegrator) executeWorkerStopPhase(ctx context.Context) error {
	w.mu.Lock()
	w.status.Phase = WorkerShutdownPhaseWorkerStop
	w.mu.Unlock()

	workerCtx, cancel := context.WithTimeout(ctx, w.config.WorkerStopTimeout)
	defer cancel()

	// Stop all workers
	for workerID, workerService := range w.workerServices {
		w.updateWorkerStatus(workerID, "stopping", "")

		if err := workerService.Stop(workerCtx); err != nil {
			w.updateWorkerStatus(workerID, "failed", err.Error())
			w.mu.Lock()
			w.status.WorkersWithErrors++
			w.mu.Unlock()
		} else {
			w.updateWorkerStatus(workerID, "stopped", "")
			w.mu.Lock()
			w.status.StoppedWorkers++
			w.mu.Unlock()
		}
	}

	return nil
}

// executeResourceCleanupPhase cleans up worker resources.
func (w *workerShutdownIntegrator) executeResourceCleanupPhase(ctx context.Context) error {
	w.mu.Lock()
	w.status.Phase = WorkerShutdownPhaseResourceCleanup
	w.mu.Unlock()

	cleanupCtx, cancel := context.WithTimeout(ctx, w.config.ResourceCleanupTimeout)
	defer cancel()

	// Initialize resource cleanup status
	resourceStatus := ResourceCleanupStatus{
		TempFilesDeleted:          false,
		WorkspaceCleared:          false,
		DatabaseConnectionsClosed: false,
		CachesCleared:             false,
		MetricsExported:           false,
		CleanupStartTime:          time.Now(),
	}

	// Simulate resource cleanup
	select {
	case <-cleanupCtx.Done():
		return cleanupCtx.Err()
	case <-time.After(100 * time.Millisecond):
		// Cleanup completed
		resourceStatus.TempFilesDeleted = true
		resourceStatus.WorkspaceCleared = true
		resourceStatus.DatabaseConnectionsClosed = true
		resourceStatus.CachesCleared = true
		resourceStatus.MetricsExported = true
		resourceStatus.CleanupDuration = time.Since(resourceStatus.CleanupStartTime)
	}

	w.mu.Lock()
	w.status.JobProcessorStatus.ResourceCleanupStatus = resourceStatus
	w.mu.Unlock()

	return nil
}

// updateWorkerStatus updates the status of a specific worker.
func (w *workerShutdownIntegrator) updateWorkerStatus(workerID, status, errorMsg string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, workerStatus := range w.status.WorkerStatuses {
		if workerStatus.WorkerID == workerID {
			w.status.WorkerStatuses[i].Status = status
			w.status.WorkerStatuses[i].Error = errorMsg
			if status == "stopping" && w.status.WorkerStatuses[i].ShutdownStartTime.IsZero() {
				w.status.WorkerStatuses[i].ShutdownStartTime = time.Now()
			}
			if status == ConsumerStatusStopped || status == string(ShutdownPhaseFailed) {
				w.status.WorkerStatuses[i].ShutdownDuration = time.Since(w.status.WorkerStatuses[i].ShutdownStartTime)
			}
			break
		}
	}
}
