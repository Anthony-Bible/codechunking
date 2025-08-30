package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// resourceCleanupManager implements ResourceCleanupManager interface.
type resourceCleanupManager struct {
	config            ResourceCleanupConfig
	resources         map[string]CleanupResource
	resourceOrder     []string
	customOrder       []string
	status            []ResourceStatus
	metrics           ResourceCleanupMetrics
	isCleanupComplete bool
	cleanupInProgress bool
	mu                sync.RWMutex
}

// NewResourceCleanupManager creates a new ResourceCleanupManager instance.
func NewResourceCleanupManager(config ResourceCleanupConfig) ResourceCleanupManager {
	// Set defaults
	if config.CleanupTimeout == 0 {
		config.CleanupTimeout = 30 * time.Second
	}
	if config.ResourceTimeout == 0 {
		config.ResourceTimeout = 10 * time.Second
	}
	if config.MaxConcurrentCleanups == 0 {
		config.MaxConcurrentCleanups = 3
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.ForceKillTimeout == 0 {
		config.ForceKillTimeout = 5 * time.Second
	}

	return &resourceCleanupManager{
		config:        config,
		resources:     make(map[string]CleanupResource),
		resourceOrder: make([]string, 0),
		customOrder:   make([]string, 0),
		status:        make([]ResourceStatus, 0),
		metrics: ResourceCleanupMetrics{
			ResourceTypeMetrics: make(map[ResourceType]TypeMetrics),
		},
		isCleanupComplete: false,
		cleanupInProgress: false,
	}
}

// RegisterResource registers a resource that needs cleanup during shutdown.
func (r *resourceCleanupManager) RegisterResource(name string, resource CleanupResource) error {
	if name == "" {
		return errors.New("resource name cannot be empty")
	}
	if resource == nil {
		return errors.New("resource cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.resources[name]; exists {
		return fmt.Errorf("resource already registered: %s", name)
	}

	r.resources[name] = resource
	r.resourceOrder = append(r.resourceOrder, name)

	// Initialize status
	r.status = append(r.status, ResourceStatus{
		Name:       name,
		Type:       resource.GetResourceType(),
		Status:     "registered",
		IsHealthy:  resource.IsHealthy(context.Background()),
		RetryCount: 0,
	})

	// Update metrics
	r.metrics.TotalResources = len(r.resources)

	return nil
}

// UnregisterResource removes a resource from cleanup management.
func (r *resourceCleanupManager) UnregisterResource(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.resources[name]; !exists {
		return fmt.Errorf("resource not found: %s", name)
	}

	delete(r.resources, name)

	// Remove from order
	for i, n := range r.resourceOrder {
		if n == name {
			r.resourceOrder = append(r.resourceOrder[:i], r.resourceOrder[i+1:]...)
			break
		}
	}

	// Remove from status
	for i, status := range r.status {
		if status.Name == name {
			r.status = append(r.status[:i], r.status[i+1:]...)
			break
		}
	}

	r.metrics.TotalResources = len(r.resources)
	return nil
}

// CleanupAll performs cleanup of all registered resources with timeout.
func (r *resourceCleanupManager) CleanupAll(ctx context.Context) error {
	r.mu.Lock()
	if r.cleanupInProgress {
		r.mu.Unlock()
		return errors.New("cleanup already in progress")
	}
	r.cleanupInProgress = true
	r.metrics.LastCleanupTime = time.Now()
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.cleanupInProgress = false
		r.mu.Unlock()
	}()

	// Create timeout context
	cleanupCtx, cancel := context.WithTimeout(ctx, r.config.CleanupTimeout)
	defer cancel()

	// Determine cleanup order
	order := r.getCleanupOrder()

	// Group resources by concurrent capability
	concurrentResources := make([]string, 0)
	sequentialResources := make([]string, 0)

	for _, name := range order {
		resource := r.resources[name]
		if resource.CanCleanupConcurrently() {
			concurrentResources = append(concurrentResources, name)
		} else {
			sequentialResources = append(sequentialResources, name)
		}
	}

	// Clean up concurrent resources first
	if len(concurrentResources) > 0 {
		if err := r.cleanupConcurrent(cleanupCtx, concurrentResources); err != nil {
			return err
		}
	}

	// Clean up sequential resources
	if len(sequentialResources) > 0 {
		if err := r.cleanupSequential(cleanupCtx, sequentialResources); err != nil {
			return err
		}
	}

	r.mu.Lock()
	r.isCleanupComplete = true
	r.mu.Unlock()

	return nil
}

// CleanupResource performs cleanup of a specific resource.
func (r *resourceCleanupManager) CleanupResource(ctx context.Context, name string) error {
	r.mu.RLock()
	resource, exists := r.resources[name]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("resource not found: %s", name)
	}

	return r.cleanupSingleResource(ctx, name, resource)
}

// GetResourceStatus returns the status of all managed resources.
func (r *resourceCleanupManager) GetResourceStatus() []ResourceStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid race conditions
	status := make([]ResourceStatus, len(r.status))
	copy(status, r.status)
	return status
}

// GetCleanupMetrics returns metrics about resource cleanup performance.
func (r *resourceCleanupManager) GetCleanupMetrics() ResourceCleanupMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := r.metrics

	// Calculate cleanup metrics
	metrics.CleanedResources = 0
	metrics.FailedResources = 0

	for _, status := range r.status {
		switch status.Status {
		case "cleaned":
			metrics.CleanedResources++
		case string(ShutdownPhaseFailed):
			metrics.FailedResources++
		}
	}

	return metrics
}

// SetCleanupOrder sets the order in which resources should be cleaned up.
func (r *resourceCleanupManager) SetCleanupOrder(order []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate that all names exist
	for _, name := range order {
		if _, exists := r.resources[name]; !exists {
			return fmt.Errorf("resource not found in order: %s", name)
		}
	}

	r.customOrder = make([]string, len(order))
	copy(r.customOrder, order)

	return nil
}

// IsCleanupComplete returns true if all resources have been cleaned up.
func (r *resourceCleanupManager) IsCleanupComplete() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isCleanupComplete
}

// getCleanupOrder determines the order in which resources should be cleaned.
func (r *resourceCleanupManager) getCleanupOrder() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use custom order if set
	if len(r.customOrder) > 0 {
		return r.customOrder
	}

	// Use priority-based ordering
	type resourcePriority struct {
		name     string
		priority int
	}

	priorities := make([]resourcePriority, 0, len(r.resources))
	for name, resource := range r.resources {
		priorities = append(priorities, resourcePriority{
			name:     name,
			priority: resource.GetCleanupPriority(),
		})
	}

	// Sort by priority (higher priority first)
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].priority > priorities[j].priority
	})

	order := make([]string, len(priorities))
	for i, p := range priorities {
		order[i] = p.name
	}

	return order
}

// cleanupConcurrent cleans up resources concurrently.
func (r *resourceCleanupManager) cleanupConcurrent(ctx context.Context, names []string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(names))
	semaphore := make(chan struct{}, r.config.MaxConcurrentCleanups)

	for _, name := range names {
		wg.Add(1)
		go func(resourceName string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			resource := r.resources[resourceName]
			if err := r.cleanupSingleResource(ctx, resourceName, resource); err != nil {
				errChan <- fmt.Errorf("failed to cleanup %s: %w", resourceName, err)
			}
		}(name)
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("concurrent cleanup errors: %v", errors)
	}

	return nil
}

// cleanupSequential cleans up resources sequentially.
func (r *resourceCleanupManager) cleanupSequential(ctx context.Context, names []string) error {
	var errors []error
	for _, name := range names {
		resource := r.resources[name]
		if err := r.cleanupSingleResource(ctx, name, resource); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup %s: %w", name, err))
			// Continue with other resources even if one fails
			continue
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("sequential cleanup errors: %v", errors)
	}
	return nil
}

// cleanupSingleResource cleans up a single resource with retry logic.
func (r *resourceCleanupManager) cleanupSingleResource(
	ctx context.Context,
	name string,
	resource CleanupResource,
) error {
	r.updateResourceStatus(name, "cleaning", "", 0)

	resourceCtx, cancel := context.WithTimeout(ctx, r.config.ResourceTimeout)
	defer cancel()

	var lastError error
	maxRetries := r.config.MaxRetries
	if !r.config.RetryEnabled {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check health if enabled
		if r.config.HealthCheckEnabled {
			if !resource.IsHealthy(resourceCtx) {
				r.updateResourceStatus(name, string(ShutdownPhaseFailed), "resource unhealthy", attempt)
				return fmt.Errorf("resource %s is not healthy", name)
			}
		}

		// Attempt cleanup
		startTime := time.Now()
		err := resource.Cleanup(resourceCtx)
		duration := time.Since(startTime)

		if err == nil {
			// Success
			r.updateResourceStatus(name, "cleaned", "", attempt)
			r.updateTypeMetrics(resource.GetResourceType(), true, duration)
			return nil
		}

		lastError = err
		r.updateResourceStatus(name, "cleaning", err.Error(), attempt)

		// If this was the last attempt, mark as failed
		if attempt == maxRetries {
			r.updateResourceStatus(name, string(ShutdownPhaseFailed), err.Error(), attempt)
			r.updateTypeMetrics(resource.GetResourceType(), false, duration)
			break
		}

		// Wait before retry
		if r.config.RetryDelay > 0 {
			time.Sleep(r.config.RetryDelay)
		}
	}

	return lastError
}

// updateResourceStatus updates the status of a resource.
func (r *resourceCleanupManager) updateResourceStatus(name, status, errorMsg string, retryCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, resourceStatus := range r.status {
		if resourceStatus.Name == name {
			r.status[i].Status = status
			r.status[i].Error = errorMsg
			r.status[i].ForceKilled = false
			r.status[i].RetryCount = retryCount
			r.status[i].LastRetry = time.Now()
			if status == "cleaning" && r.status[i].StartTime.IsZero() {
				r.status[i].StartTime = time.Now()
			}
			if status == "cleaned" || status == string(ShutdownPhaseFailed) {
				r.status[i].Duration = time.Since(r.status[i].StartTime)
			}
			break
		}
	}
}

// updateTypeMetrics updates metrics for a resource type.
func (r *resourceCleanupManager) updateTypeMetrics(resourceType ResourceType, success bool, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	typeMetrics, exists := r.metrics.ResourceTypeMetrics[resourceType]
	if !exists {
		typeMetrics = TypeMetrics{}
	}

	typeMetrics.Count++
	if success {
		typeMetrics.SuccessCount++
	} else {
		typeMetrics.FailureCount++
	}

	// Update timing metrics
	totalTime := typeMetrics.AverageCleanupTime*time.Duration(typeMetrics.Count-1) + duration
	typeMetrics.AverageCleanupTime = totalTime / time.Duration(typeMetrics.Count)

	if duration > typeMetrics.MaxCleanupTime {
		typeMetrics.MaxCleanupTime = duration
	}

	r.metrics.ResourceTypeMetrics[resourceType] = typeMetrics
}
