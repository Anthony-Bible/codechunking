package service

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"time"
)

// gracefulShutdownService implements the GracefulShutdownService interface.
type gracefulShutdownService struct {
	config            GracefulShutdownConfig
	shutdownHooks     []shutdownHookInfo
	status            ShutdownStatus
	metrics           ShutdownMetrics
	signalChan        chan os.Signal
	isShuttingDown    bool
	mu                sync.RWMutex
	componentStatuses []ComponentShutdownStatus
	signalReceived    os.Signal
	// Performance and observability enhancements
	observability    *ShutdownObservabilityManager
	metricsCollector *ShutdownMetricsCollector
}

type shutdownHookInfo struct {
	name string
	hook ShutdownHookFunc
}

// NewGracefulShutdownService creates a new graceful shutdown service.
func NewGracefulShutdownService(config GracefulShutdownConfig) GracefulShutdownService {
	return &gracefulShutdownService{
		config:         config,
		shutdownHooks:  make([]shutdownHookInfo, 0),
		signalChan:     make(chan os.Signal, 1),
		isShuttingDown: false,
		status: ShutdownStatus{
			IsShuttingDown:    false,
			ShutdownPhase:     ShutdownPhaseIdle,
			ComponentsTotal:   0,
			ComponentsClosed:  0,
			ComponentsFailed:  0,
			ComponentStatuses: make([]ComponentShutdownStatus, 0),
		},
		metrics: ShutdownMetrics{
			SignalCounts:     make(map[string]int64),
			ComponentMetrics: make([]ComponentShutdownMetrics, 0),
		},
		componentStatuses: make([]ComponentShutdownStatus, 0),
		// Initialize observability and metrics collection
		observability:    NewShutdownObservabilityManager(),
		metricsCollector: NewShutdownMetricsCollector(config.MetricsEnabled),
	}
}

// Start initializes signal handlers and begins monitoring for shutdown signals.
func (g *gracefulShutdownService) Start(_ context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.config.SignalHandlingEnabled {
		signal.Notify(g.signalChan, g.config.Signals...)
		go g.handleSignals()
	}

	return nil
}

// Shutdown initiates the graceful shutdown process.
func (g *gracefulShutdownService) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	if g.isShuttingDown {
		g.mu.Unlock()
		return errors.New("shutdown already in progress")
	}
	g.isShuttingDown = true
	g.status.IsShuttingDown = true
	g.status.StartTime = time.Now()
	g.status.ComponentsTotal = len(g.shutdownHooks)
	g.mu.Unlock()

	// Create component statuses
	g.initializeComponentStatuses()

	// Execute shutdown phases
	if err := g.executeDrainingPhase(ctx); err != nil {
		return g.handleShutdownError(err)
	}

	if err := g.executeJobCompletionPhase(ctx); err != nil {
		return g.handleShutdownError(err)
	}

	if err := g.executeResourceCleanupPhase(ctx); err != nil {
		return g.handleShutdownError(err)
	}

	g.completeShutdown()
	return nil
}

// RegisterShutdownHook adds a component that needs cleanup during shutdown.
func (g *gracefulShutdownService) RegisterShutdownHook(name string, shutdownFunc ShutdownHookFunc) error {
	if name == "" {
		return errors.New("hook name cannot be empty")
	}
	if shutdownFunc == nil {
		return errors.New("hook function cannot be nil")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Check for duplicate names
	for _, hook := range g.shutdownHooks {
		if hook.name == name {
			return errors.New("hook already registered")
		}
	}

	g.shutdownHooks = append(g.shutdownHooks, shutdownHookInfo{
		name: name,
		hook: shutdownFunc,
	})

	return nil
}

// GetShutdownStatus returns current status of shutdown process.
func (g *gracefulShutdownService) GetShutdownStatus() ShutdownStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	status := g.status
	status.ElapsedTime = time.Since(status.StartTime)
	status.TimeoutRemaining = g.config.GracefulTimeout - status.ElapsedTime
	status.ComponentStatuses = make([]ComponentShutdownStatus, len(g.componentStatuses))
	copy(status.ComponentStatuses, g.componentStatuses)
	status.SignalReceived = g.signalReceived

	return status
}

// GetShutdownMetrics returns metrics about shutdown performance and health.
func (g *gracefulShutdownService) GetShutdownMetrics() ShutdownMetrics {
	g.mu.RLock()
	defer g.mu.RUnlock()

	metrics := g.metrics
	if g.status.IsShuttingDown {
		metrics.LastShutdownDuration = time.Since(g.status.StartTime)
	}

	return metrics
}

// IsShuttingDown returns true if shutdown process has been initiated.
func (g *gracefulShutdownService) IsShuttingDown() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.isShuttingDown
}

// ForceShutdown immediately terminates all components without graceful cleanup.
func (g *gracefulShutdownService) ForceShutdown(_ context.Context) error {
	g.mu.Lock()
	g.isShuttingDown = true
	g.status.IsShuttingDown = true
	g.status.ShutdownPhase = ShutdownPhaseForceClose
	if g.status.StartTime.IsZero() {
		g.status.StartTime = time.Now()
	}
	g.mu.Unlock()

	g.updateMetrics(false, true)
	return nil
}

// handleSignals processes incoming OS signals.
func (g *gracefulShutdownService) handleSignals() {
	for sig := range g.signalChan {
		if g.isSignalRegistered(sig) {
			g.mu.Lock()
			g.signalReceived = sig
			g.metrics.SignalCounts[sig.String()]++
			g.mu.Unlock()

			// Start shutdown process
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), g.config.GracefulTimeout)
				defer cancel()
				if err := g.Shutdown(ctx); err != nil {
					// Log error but don't block signal handling
					if g.observability != nil {
						g.observability.LogForceShutdown(ctx, "signal handler shutdown failed: "+err.Error())
					}
				}
			}()
			break
		}
	}
}

// isSignalRegistered checks if the signal is in the configured signals list.
func (g *gracefulShutdownService) isSignalRegistered(sig os.Signal) bool {
	for _, configuredSig := range g.config.Signals {
		if sig == configuredSig {
			return true
		}
	}
	return false
}

// initializeComponentStatuses creates initial status entries for all components.
func (g *gracefulShutdownService) initializeComponentStatuses() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.componentStatuses = make([]ComponentShutdownStatus, len(g.shutdownHooks))
	for i, hook := range g.shutdownHooks {
		g.componentStatuses[i] = ComponentShutdownStatus{
			Name:        hook.name,
			Status:      "pending",
			StartTime:   time.Now(),
			Duration:    0,
			Error:       "",
			ForceKilled: false,
		}
	}
	g.status.ComponentStatuses = g.componentStatuses
}

// executeDrainingPhase executes the draining phase of shutdown.
func (g *gracefulShutdownService) executeDrainingPhase(ctx context.Context) error {
	g.updatePhase(ShutdownPhaseDraining)

	drainCtx, cancel := context.WithTimeout(ctx, g.config.DrainTimeout)
	defer cancel()

	// Wait for drain timeout or context cancellation
	select {
	case <-drainCtx.Done():
		return drainCtx.Err()
	case <-time.After(g.config.DrainTimeout):
		// Draining phase completed
	}

	return nil
}

// executeJobCompletionPhase executes the job completion phase.
func (g *gracefulShutdownService) executeJobCompletionPhase(ctx context.Context) error {
	g.updatePhase(ShutdownPhaseJobCompletion)

	jobCtx, cancel := context.WithTimeout(ctx, g.config.JobCompletionTimeout)
	defer cancel()

	select {
	case <-jobCtx.Done():
		return jobCtx.Err()
	case <-time.After(g.config.JobCompletionTimeout):
		// Job completion phase completed
	}

	return nil
}

// executeResourceCleanupPhase executes the resource cleanup phase.
func (g *gracefulShutdownService) executeResourceCleanupPhase(ctx context.Context) error {
	g.updatePhase(ShutdownPhaseResourceCleanup)

	cleanupCtx, cancel := context.WithTimeout(ctx, g.config.ResourceCleanupTimeout)
	defer cancel()

	// Execute shutdown hooks in reverse order (LIFO)
	for i := len(g.shutdownHooks) - 1; i >= 0; i-- {
		hook := g.shutdownHooks[i]
		g.updateComponentStatus(i, "in_progress", "", false)

		err := hook.hook(cleanupCtx)
		if err != nil {
			g.updateComponentStatus(i, "failed", err.Error(), false)
			g.mu.Lock()
			g.status.ComponentsFailed++
			g.mu.Unlock()
		} else {
			g.updateComponentStatus(i, "completed", "", false)
			g.mu.Lock()
			g.status.ComponentsClosed++
			g.mu.Unlock()
		}

		// Check for timeout
		select {
		case <-cleanupCtx.Done():
			return cleanupCtx.Err()
		default:
		}
	}

	return nil
}

// updatePhase updates the current shutdown phase.
func (g *gracefulShutdownService) updatePhase(phase ShutdownPhase) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.status.ShutdownPhase = phase
}

// updateComponentStatus updates the status of a specific component.
func (g *gracefulShutdownService) updateComponentStatus(index int, status, errorMsg string, forceKilled bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if index < len(g.componentStatuses) {
		g.componentStatuses[index].Status = status
		g.componentStatuses[index].Duration = time.Since(g.componentStatuses[index].StartTime)
		g.componentStatuses[index].Error = errorMsg
		g.componentStatuses[index].ForceKilled = forceKilled
	}
}

// handleShutdownError handles errors during shutdown process.
func (g *gracefulShutdownService) handleShutdownError(err error) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.status.LastError = err.Error()
	g.status.ShutdownPhase = ShutdownPhaseFailed
	g.updateMetrics(false, false)

	return err
}

// completeShutdown marks shutdown as completed.
func (g *gracefulShutdownService) completeShutdown() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.status.ShutdownPhase = ShutdownPhaseCompleted
	g.updateMetrics(true, false)
}

// updateMetrics updates shutdown metrics.
func (g *gracefulShutdownService) updateMetrics(success, force bool) {
	g.metrics.TotalShutdowns++

	if success {
		g.metrics.SuccessfulShutdowns++
	} else {
		g.metrics.FailedShutdowns++
	}

	if force {
		g.metrics.ForceShutdowns++
	}

	if g.status.StartTime.IsZero() {
		g.status.StartTime = time.Now()
	}

	duration := time.Since(g.status.StartTime)
	g.metrics.LastShutdownDuration = duration

	// Calculate average shutdown time
	if g.metrics.TotalShutdowns > 0 {
		totalTime := time.Duration(g.metrics.AverageShutdownTime.Nanoseconds() * (g.metrics.TotalShutdowns - 1))
		totalTime += duration
		g.metrics.AverageShutdownTime = totalTime / time.Duration(g.metrics.TotalShutdowns)
	}
}
