package memory

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrMemoryLimitExceeded     = errors.New("memory limit exceeded")
	ErrCriticalMemoryViolation = errors.New("critical memory violation")
	ErrNotMonitoring           = errors.New("not monitoring")
	ErrOperationNotFound       = errors.New("operation not found")
	ErrContextCancelled        = errors.New("context cancelled")
)

type MemoryViolationType int

const (
	MemoryViolationNone MemoryViolationType = iota
	MemoryViolationHard
	MemoryViolationCritical
)

type MemoryLimits struct {
	PerOperationLimit int64
	TotalProcessLimit int64
	AlertThreshold    float64
	CheckInterval     time.Duration
}

type MemoryUsage struct {
	OperationID string
	Current     int64
	Peak        int64
	Timestamp   time.Time
}

type MemoryViolation struct {
	OperationID   string
	CurrentUsage  int64
	Limit         int64
	ViolationType MemoryViolationType
	Timestamp     time.Time
}

type MemoryStats struct {
	CurrentUsage     int64
	PeakUsage        int64
	ActiveOperations int
	TotalUsage       int64
}

type MemoryLimitEnforcer struct {
	limits        MemoryLimits
	operations    map[string]*operationTracker
	mutex         sync.RWMutex
	trackerPool   sync.Pool
	alertThrottle map[string]*alertThrottle
	throttleMutex sync.RWMutex
}

type operationTracker struct {
	usage      MemoryUsage
	simulated  int64
	ctx        context.Context
	cancel     context.CancelFunc
	lastCheck  time.Time
	checkCount int64
}

type alertThrottle struct {
	lastAlert time.Time
	count     int64
}

func NewEnforcer(limits MemoryLimits) *MemoryLimitEnforcer {
	if limits.PerOperationLimit == 0 {
		limits.PerOperationLimit = 256 * 1024 * 1024
	}
	if limits.TotalProcessLimit == 0 {
		limits.TotalProcessLimit = 1024 * 1024 * 1024
	}
	if limits.AlertThreshold == 0 {
		limits.AlertThreshold = 0.8
	}
	if limits.CheckInterval == 0 {
		limits.CheckInterval = time.Second
	}

	e := &MemoryLimitEnforcer{
		limits:        limits,
		operations:    make(map[string]*operationTracker),
		alertThrottle: make(map[string]*alertThrottle),
	}

	e.trackerPool = sync.Pool{
		New: func() interface{} {
			return &operationTracker{}
		},
	}

	return e
}

func (e *MemoryLimitEnforcer) StartMonitoring(ctx context.Context, operationID string) error {
	if ctx.Err() != nil {
		slogger.Error(ctx, "context cancelled before monitoring started", slogger.Fields{
			"operation_id": operationID,
			"error":        ctx.Err().Error(),
		})
		return ErrContextCancelled
	}

	e.mutex.Lock()
	if _, exists := e.operations[operationID]; exists {
		e.mutex.Unlock()
		slogger.Debug(ctx, "operation already being monitored", slogger.Fields{
			"operation_id": operationID,
		})
		return nil
	}

	tracker := e.getTrackerFromPool()
	if tracker == nil {
		e.mutex.Unlock()
		slogger.Error(ctx, "failed to get tracker from pool", slogger.Fields{
			"operation_id": operationID,
		})
		return errors.New("failed to allocate operation tracker")
	}

	tracker.usage = MemoryUsage{
		OperationID: operationID,
		Current:     1024 * 1024,
		Peak:        1024 * 1024,
		Timestamp:   time.Now(),
	}
	tracker.ctx, tracker.cancel = context.WithCancel(ctx)
	tracker.lastCheck = time.Now()
	tracker.checkCount = 0

	e.operations[operationID] = tracker
	e.mutex.Unlock()

	slogger.Info(ctx, "started monitoring operation", slogger.Fields{
		"operation_id": operationID,
	})

	go e.monitorOperation(operationID, tracker)

	return nil
}

func (e *MemoryLimitEnforcer) handleViolationCheck(
	ctx context.Context,
	operationID string,
	tracker *operationTracker,
) (bool, error) {
	usage, err := e.CheckMemoryUsageByOperation(operationID)
	if err != nil {
		slogger.Error(ctx, "failed to check memory usage", slogger.Fields{
			"operation_id": operationID,
			"error":        err.Error(),
		})
		return false, err
	}

	tracker.checkCount++

	violation := e.detectViolation(usage)
	if violation.ViolationType != MemoryViolationNone {
		if handleErr := e.HandleMemoryViolation(ctx, violation); handleErr != nil {
			slogger.Error(ctx, "memory violation handled", slogger.Fields{
				"operation_id":   violation.OperationID,
				"current_usage":  formatBytes(violation.CurrentUsage),
				"limit":          formatBytes(violation.Limit),
				"violation_type": violation.ViolationType,
				"error":          handleErr.Error(),
			})
		}
		return true, nil
	}
	return false, nil
}

func (e *MemoryLimitEnforcer) handleMemoryPressure(ctx context.Context, operationID string) {
	usage, err := e.CheckMemoryUsageByOperation(operationID)
	if err != nil {
		slogger.Error(ctx, "failed to check memory usage for alert", slogger.Fields{
			"operation_id": operationID,
			"error":        err.Error(),
		})
		return
	}

	alert := e.generateAlert(usage)
	if isUnderPressure, ok := alert.Metadata["is_under_pressure"].(bool); ok && isUnderPressure {
		if e.shouldSendAlert(operationID) {
			usagePercentage, _ := alert.Metadata["usage_percentage"].(float64)
			slogger.Warn(ctx, "memory pressure detected", slogger.Fields{
				"operation_id":     alert.OperationID,
				"current_usage":    formatBytes(alert.CurrentUsage),
				"threshold":        formatBytes(alert.Threshold),
				"usage_percentage": usagePercentage,
			})
		}
	}
}

func (e *MemoryLimitEnforcer) adjustMonitoringInterval(
	ticker *time.Ticker,
	currentInterval *time.Duration,
	baseInterval time.Duration,
	maxInterval time.Duration,
	backoffFactor time.Duration,
) {
	if e.isHighLoad() {
		newInterval := time.Duration(
			math.Min(float64(int64(*currentInterval)*int64(backoffFactor)), float64(maxInterval)),
		)
		*currentInterval = newInterval
	} else if *currentInterval > baseInterval {
		newInterval := time.Duration(math.Max(float64(int64(*currentInterval)/int64(backoffFactor)), float64(baseInterval)))
		*currentInterval = newInterval
	}

	ticker.Reset(*currentInterval)
}

func (e *MemoryLimitEnforcer) monitorOperation(operationID string, tracker *operationTracker) {
	baseInterval := e.limits.CheckInterval
	currentInterval := baseInterval
	maxInterval := baseInterval * 64
	backoffFactor := time.Duration(2)

	defer func() {
		e.mutex.Lock()
		delete(e.operations, operationID)
		e.mutex.Unlock()

		e.throttleMutex.Lock()
		delete(e.alertThrottle, operationID)
		e.throttleMutex.Unlock()

		e.returnTrackerToPool(tracker)
	}()

	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	for {
		if tracker == nil {
			slogger.Error(context.Background(), "tracker is nil in monitoring loop", slogger.Fields{
				"operation_id": operationID,
			})
			return
		}

		trackerCtx := tracker.ctx
		if trackerCtx == nil {
			slogger.Error(context.Background(), "tracker context is nil in monitoring loop", slogger.Fields{
				"operation_id": operationID,
			})
			return
		}

		select {
		case <-trackerCtx.Done():
			slogger.Info(trackerCtx, "monitoring stopped for operation", slogger.Fields{
				"operation_id": operationID,
			})
			return
		case <-ticker.C:
			violationDetected, err := e.handleViolationCheck(trackerCtx, operationID, tracker)
			if err != nil {
				continue
			}
			if violationDetected {
				return
			}

			e.handleMemoryPressure(trackerCtx, operationID)

			e.adjustMonitoringInterval(ticker, &currentInterval, baseInterval, maxInterval, backoffFactor)
			tracker.lastCheck = time.Now()
		}
	}
}

func (e *MemoryLimitEnforcer) CheckMemoryUsage() (MemoryUsage, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if len(e.operations) == 0 {
		return MemoryUsage{}, ErrNotMonitoring
	}

	var totalUsage int64
	var peakUsage int64
	var currentUsage int64
	var firstOperationID string

	first := true
	for _, tracker := range e.operations {
		if first {
			firstOperationID = tracker.usage.OperationID
			first = false
		}
		usage := atomic.LoadInt64(&tracker.usage.Current)
		simulated := atomic.LoadInt64(&tracker.simulated)
		if simulated > 0 {
			usage = simulated
		}
		currentUsage = usage
		totalUsage += usage
		currentPeak := atomic.LoadInt64(&tracker.usage.Peak)
		if usage > currentPeak {
			atomic.StoreInt64(&tracker.usage.Peak, usage)
		}
		if usage > peakUsage {
			peakUsage = usage
		}
	}

	return MemoryUsage{
		OperationID: firstOperationID,
		Current:     currentUsage,
		Peak:        peakUsage,
		Timestamp:   time.Now(),
	}, nil
}

func (e *MemoryLimitEnforcer) CheckMemoryUsageByOperation(operationID string) (MemoryUsage, error) {
	e.mutex.RLock()
	tracker, exists := e.operations[operationID]
	e.mutex.RUnlock()

	if !exists {
		return MemoryUsage{}, ErrOperationNotFound
	}

	usage := tracker.usage
	current := atomic.LoadInt64(&tracker.usage.Current)
	simulated := atomic.LoadInt64(&tracker.simulated)

	if simulated > 0 {
		usage.Current = simulated
		currentPeak := atomic.LoadInt64(&tracker.usage.Peak)
		if simulated > currentPeak {
			atomic.StoreInt64(&tracker.usage.Peak, simulated)
		}
	} else {
		usage.Current = current
	}

	usage.Timestamp = time.Now()
	usage.OperationID = operationID

	return usage, nil
}

func (e *MemoryLimitEnforcer) simulateMemoryUsage(bytes int64) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	for _, tracker := range e.operations {
		atomic.StoreInt64(&tracker.simulated, bytes)
	}
}

func (e *MemoryLimitEnforcer) detectViolation(usage MemoryUsage) MemoryViolation {
	currentUsage := usage.Current
	limit := e.limits.PerOperationLimit

	if currentUsage > limit {
		return MemoryViolation{
			OperationID:   usage.OperationID,
			CurrentUsage:  currentUsage,
			Limit:         limit,
			ViolationType: MemoryViolationHard,
			Timestamp:     time.Now(),
		}
	}

	if currentUsage > int64(float64(limit)*0.95) {
		return MemoryViolation{
			OperationID:   usage.OperationID,
			CurrentUsage:  currentUsage,
			Limit:         limit,
			ViolationType: MemoryViolationCritical,
			Timestamp:     time.Now(),
		}
	}

	return MemoryViolation{
		OperationID:   usage.OperationID,
		ViolationType: MemoryViolationNone,
	}
}

func (e *MemoryLimitEnforcer) HandleMemoryViolation(ctx context.Context, violation MemoryViolation) error {
	e.mutex.Lock()
	if tracker, exists := e.operations[violation.OperationID]; exists {
		tracker.cancel()
		delete(e.operations, violation.OperationID)
	}
	e.mutex.Unlock()

	switch violation.ViolationType {
	case MemoryViolationHard:
		err := fmt.Errorf(
			"memory limit exceeded: operation %s using %s of limit %s: %w",
			violation.OperationID,
			formatBytes(violation.CurrentUsage),
			formatBytes(violation.Limit),
			ErrMemoryLimitExceeded,
		)
		slogger.Error(ctx, "hard memory violation detected", slogger.Fields{
			"operation_id":   violation.OperationID,
			"current_usage":  violation.CurrentUsage,
			"limit":          violation.Limit,
			"violation_type": "hard",
		})
		return err
	case MemoryViolationCritical:
		err := fmt.Errorf(
			"critical memory violation: operation %s using %s of limit %s: %w",
			violation.OperationID,
			formatBytes(violation.CurrentUsage),
			formatBytes(violation.Limit),
			ErrMemoryLimitExceeded,
		)
		slogger.Error(ctx, "critical memory violation detected", slogger.Fields{
			"operation_id":   violation.OperationID,
			"current_usage":  violation.CurrentUsage,
			"limit":          violation.Limit,
			"violation_type": "critical",
		})
		return err
	case MemoryViolationNone:
		return nil
	default:
		return nil
	}
}

func (e *MemoryLimitEnforcer) CalculateAdaptiveLimit(fileSize int64) int64 {
	baseLimit := e.limits.PerOperationLimit
	targetMedium := int64(384 * 1024 * 1024)
	maxLimit := e.limits.TotalProcessLimit

	if fileSize <= 10*1024 {
		return baseLimit
	}

	if fileSize > 10*1024 && fileSize <= 1024*1024 {
		increase := (fileSize - 10*1024) * (targetMedium - baseLimit) / (1024*1024 - 10*1024)
		return baseLimit + increase
	}

	if fileSize > 1024*1024 && fileSize < 100*1024*1024 {
		reduction := (fileSize - 1024*1024) * (targetMedium - baseLimit) / (99 * 1024 * 1024)
		return targetMedium - reduction
	}

	return maxLimit
}

func (e *MemoryLimitEnforcer) GetAdaptiveLimit(fileSize int64) int64 {
	return e.CalculateAdaptiveLimit(fileSize)
}

func (e *MemoryLimitEnforcer) generateAlert(usage MemoryUsage) MemoryAlert {
	limit := e.limits.PerOperationLimit
	percentage := math.Round((float64(usage.Current)/float64(limit))*1000000) / 1000000
	pressure := math.Round(percentage*100) >= math.Round(e.limits.AlertThreshold*100)

	return MemoryAlert{
		OperationID:  usage.OperationID,
		AlertType:    MemoryAlertLimitExceeded,
		CurrentUsage: usage.Current,
		Threshold:    e.limits.PerOperationLimit,
		Timestamp:    time.Now(),
		Metadata: map[string]interface{}{
			"is_under_pressure": pressure,
			"usage_percentage":  percentage,
		},
	}
}

func (e *MemoryLimitEnforcer) StopMonitoring(operationID string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	tracker, exists := e.operations[operationID]
	if !exists {
		return ErrOperationNotFound
	}

	tracker.cancel()
	delete(e.operations, operationID)

	e.throttleMutex.Lock()
	delete(e.alertThrottle, operationID)
	e.throttleMutex.Unlock()

	e.returnTrackerToPool(tracker)

	return nil
}

func (e *MemoryLimitEnforcer) GetMemoryStats() MemoryStats {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	var totalUsage, peakUsage int64
	count := len(e.operations)

	for _, tracker := range e.operations {
		current := atomic.LoadInt64(&tracker.usage.Current)
		simulated := atomic.LoadInt64(&tracker.simulated)
		usage := current
		if simulated > 0 {
			usage = simulated
		}
		totalUsage += usage
		if usage > peakUsage {
			peakUsage = usage
		}
	}

	return MemoryStats{
		CurrentUsage:     totalUsage,
		PeakUsage:        peakUsage,
		ActiveOperations: count,
		TotalUsage:       totalUsage,
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	value := float64(bytes) / float64(div)
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d%cB", int64(value), "KMGTPE"[exp])
	}
	return fmt.Sprintf("%.1f%cB", value, "KMGTPE"[exp])
}

func (e *MemoryLimitEnforcer) getTrackerFromPool() *operationTracker {
	obj := e.trackerPool.Get()
	if obj == nil {
		return nil
	}

	tracker, ok := obj.(*operationTracker)
	if !ok {
		slogger.Error(context.Background(), "invalid type from tracker pool", slogger.Fields{
			"expected_type": "*operationTracker",
			"actual_type":   fmt.Sprintf("%T", obj),
		})
		return nil
	}

	tracker.usage = MemoryUsage{}
	tracker.simulated = 0
	tracker.ctx = nil
	tracker.cancel = nil
	tracker.lastCheck = time.Time{}
	tracker.checkCount = 0
	return tracker
}

func (e *MemoryLimitEnforcer) returnTrackerToPool(tracker *operationTracker) {
	tracker.cancel = nil
	tracker.ctx = nil
	e.trackerPool.Put(tracker)
}

func (e *MemoryLimitEnforcer) shouldSendAlert(operationID string) bool {
	e.throttleMutex.Lock()
	defer e.throttleMutex.Unlock()

	now := time.Now()
	throttle, exists := e.alertThrottle[operationID]
	if !exists {
		e.alertThrottle[operationID] = &alertThrottle{
			lastAlert: now,
			count:     1,
		}
		return true
	}

	if now.Sub(throttle.lastAlert) > time.Minute {
		throttle.lastAlert = now
		throttle.count = 1
		return true
	}

	if throttle.count < 5 {
		throttle.count++
		throttle.lastAlert = now
		return true
	}

	return false
}

func (e *MemoryLimitEnforcer) isHighLoad() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	stats := e.GetMemoryStats()
	return stats.CurrentUsage > int64(float64(e.limits.TotalProcessLimit)*0.9)
}
