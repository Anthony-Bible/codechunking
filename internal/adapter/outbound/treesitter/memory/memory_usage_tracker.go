package memory

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type MemoryPressureLevel int

const (
	MemoryPressureLow MemoryPressureLevel = iota
	MemoryPressureCritical
)

type MemoryAlertType int

const (
	MemoryAlertLimitExceeded MemoryAlertType = iota
	MemoryAlertSpikeDetected
	MemoryAlertPotentialLeak
)

type TrackerConfig struct {
	OTELMeter           metric.Meter
	ReportingInterval   time.Duration
	PressureThreshold   float64
	EnableGCTracking    bool
	EnableLeakDetection bool
	MetricLabels        []attribute.KeyValue
}

type MemoryBreakdown struct {
	ByOperation map[string]int64
	ByFileSize  map[string]int64
	TotalActive int64
	PeakUsage   int64
	Timestamp   time.Time
}

type MemoryAlert struct {
	OperationID  string
	AlertType    MemoryAlertType
	CurrentUsage int64
	Threshold    int64
	Timestamp    time.Time
	Metadata     map[string]interface{}
}

type MemoryUsageTracker struct {
	meter               metric.Meter
	config              TrackerConfig
	memoryUsageHist     metric.Int64Histogram
	activeMemoryGauge   metric.Int64Gauge
	memoryPressureGauge metric.Float64Gauge
	gcEventsCounter     metric.Int64Counter
	memoryLeakCounter   metric.Int64Counter
	alertsCounter       metric.Int64Counter

	operationMemory map[string]*atomic.Int64
	activeMemory    map[string]*atomic.Int64
	fileSizeMemory  map[string]*atomic.Int64
	totalActive     *atomic.Int64
	peakUsage       *atomic.Int64
	previousUsage   *atomic.Int64
	mutex           sync.RWMutex
	stopChan        chan struct{}
	otelAvailable   *atomic.Bool
	attrPool        *sync.Pool
}

func NewMemoryUsageTracker(ctx context.Context, config TrackerConfig) (*MemoryUsageTracker, error) {
	tracker := &MemoryUsageTracker{
		meter:           config.OTELMeter,
		config:          config,
		operationMemory: make(map[string]*atomic.Int64),
		activeMemory:    make(map[string]*atomic.Int64),
		fileSizeMemory:  make(map[string]*atomic.Int64),
		totalActive:     &atomic.Int64{},
		peakUsage:       &atomic.Int64{},
		previousUsage:   &atomic.Int64{},
		stopChan:        make(chan struct{}),
		otelAvailable:   &atomic.Bool{},
		attrPool: &sync.Pool{
			New: func() interface{} {
				attrs := make([]attribute.KeyValue, 0, 10)
				return &attrs
			},
		},
	}

	tracker.otelAvailable.Store(config.OTELMeter != nil)

	if tracker.otelAvailable.Load() {
		if err := tracker.initOTELMetrics(); err != nil {
			tracker.otelAvailable.Store(false)
			slogger.Error(ctx, "Failed to initialize OTEL metrics", slogger.Fields{"error": err})
		}
	}

	return tracker, nil
}

func (t *MemoryUsageTracker) initOTELMetrics() error {
	var err error

	t.memoryUsageHist, err = t.meter.Int64Histogram("memory.usage.bytes")
	if err != nil {
		return err
	}

	t.activeMemoryGauge, err = t.meter.Int64Gauge("memory.active.bytes")
	if err != nil {
		return err
	}

	t.memoryPressureGauge, err = t.meter.Float64Gauge("memory.pressure.level")
	if err != nil {
		return err
	}

	t.gcEventsCounter, err = t.meter.Int64Counter("memory.gc.events")
	if err != nil {
		return err
	}

	t.memoryLeakCounter, err = t.meter.Int64Counter("memory.leaks.detected")
	if err != nil {
		return err
	}

	t.alertsCounter, err = t.meter.Int64Counter("memory.alerts.generated")
	if err != nil {
		return err
	}

	return nil
}

func (t *MemoryUsageTracker) RecordMemoryUsage(ctx context.Context, operationID string, bytes int64) error {
	t.mutex.Lock()
	if _, exists := t.operationMemory[operationID]; !exists {
		t.operationMemory[operationID] = &atomic.Int64{}
		t.activeMemory[operationID] = &atomic.Int64{}
		t.fileSizeMemory[operationID] = &atomic.Int64{}
	}
	t.operationMemory[operationID].Add(bytes)
	currentOpUsage := t.operationMemory[operationID].Load()
	peak := t.peakUsage.Load()
	if currentOpUsage > peak {
		t.peakUsage.CompareAndSwap(peak, currentOpUsage)
	}
	t.fileSizeMemory[operationID].Add(bytes)
	t.activeMemory[operationID].Store(t.operationMemory[operationID].Load())
	t.updateTotalActive()
	t.mutex.Unlock()

	if t.otelAvailable.Load() {
		attrs := t.getAttributes(operationID)
		t.memoryUsageHist.Record(ctx, bytes, metric.WithAttributes(attrs...))
		t.putAttributes(attrs)
	}

	return nil
}

func (t *MemoryUsageTracker) TrackActiveMemory(ctx context.Context, operationID string, bytes int64) error {
	t.mutex.Lock()
	if _, exists := t.activeMemory[operationID]; !exists {
		t.activeMemory[operationID] = &atomic.Int64{}
	}
	t.activeMemory[operationID].Store(bytes)
	t.updateTotalActive()
	t.mutex.Unlock()

	if t.otelAvailable.Load() {
		t.activeMemoryGauge.Record(ctx, t.totalActive.Load(), metric.WithAttributes(t.config.MetricLabels...))
	}

	return nil
}

func (t *MemoryUsageTracker) MonitorMemoryPressure(ctx context.Context) (MemoryPressureLevel, error) {
	currentUsage := t.totalActive.Load()

	pressureLevel := MemoryPressureLow
	if t.config.PressureThreshold > 0 && float64(currentUsage) > t.config.PressureThreshold {
		pressureLevel = MemoryPressureCritical
	}

	if t.otelAvailable.Load() {
		t.memoryPressureGauge.Record(ctx, float64(pressureLevel), metric.WithAttributes(t.config.MetricLabels...))
	}

	return pressureLevel, nil
}

func (t *MemoryUsageTracker) ReportMemoryBreakdown() (MemoryBreakdown, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	byOperation := make(map[string]int64)
	byFileSize := make(map[string]int64)

	for k, v := range t.operationMemory {
		parts := strings.Split(k, "-")
		if len(parts) > 0 {
			opType := parts[0]
			byOperation[opType] += v.Load()
		} else {
			byOperation[k] = v.Load()
		}
	}

	for k, v := range t.fileSizeMemory {
		parts := strings.Split(k, "-")
		if len(parts) > 0 {
			opType := parts[0]
			byFileSize[opType] += v.Load()
		} else {
			byFileSize[k] = v.Load()
		}
	}

	return MemoryBreakdown{
		ByOperation: byOperation,
		ByFileSize:  byFileSize,
		TotalActive: t.totalActive.Load(),
		PeakUsage:   t.peakUsage.Load(),
		Timestamp:   time.Now(),
	}, nil
}

func (t *MemoryUsageTracker) TrackMemoryCleanup(ctx context.Context, operationID string, bytesReclaimed int64) error {
	t.mutex.Lock()
	if mem, exists := t.activeMemory[operationID]; exists {
		current := mem.Load()
		newValue := current - bytesReclaimed
		if newValue < 0 {
			newValue = 0
		}
		mem.Store(newValue)
		t.updateTotalActive()
	}
	t.mutex.Unlock()

	if t.otelAvailable.Load() && t.config.EnableGCTracking {
		attrs := t.getAttributes(operationID)
		t.gcEventsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		t.putAttributes(attrs)
	}

	return nil
}

func (t *MemoryUsageTracker) GenerateMemoryAlert(ctx context.Context, alert MemoryAlert) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.otelAvailable.Load() {
		attrs := t.getAlertAttributes(alert.AlertType.String())
		t.alertsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		t.putAttributes(attrs)
	}

	return nil
}

func (t *MemoryUsageTracker) StartPeriodicReporting(ctx context.Context, interval time.Duration) error {
	if interval == 0 {
		interval = t.config.ReportingInterval
	}
	if interval == 0 {
		interval = time.Second
	}

	t.stopChan = make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t.detectMemorySpikes(ctx)
				if t.config.EnableLeakDetection {
					t.detectMemoryLeaks(ctx)
				}
			case <-ctx.Done():
				close(t.stopChan)
				return
			case <-t.stopChan:
				return
			}
		}
	}()

	return nil
}

func (t *MemoryUsageTracker) StopPeriodicReporting() error {
	close(t.stopChan)
	return nil
}

func (mat MemoryAlertType) String() string {
	switch mat {
	case MemoryAlertLimitExceeded:
		return "limit_exceeded"
	case MemoryAlertSpikeDetected:
		return "spike_detected"
	case MemoryAlertPotentialLeak:
		return "potential_leak"
	default:
		return "unknown"
	}
}

func (t *MemoryUsageTracker) TrackFileSizeCorrelation(
	ctx context.Context,
	operationID string,
	fileSize int64,
	memoryUsed int64,
) error {
	t.mutex.Lock()
	if _, exists := t.fileSizeMemory[operationID]; !exists {
		t.fileSizeMemory[operationID] = &atomic.Int64{}
	}
	t.fileSizeMemory[operationID].Add(memoryUsed)
	t.mutex.Unlock()

	if t.otelAvailable.Load() {
		attrs := t.getAttributes(operationID)
		attrs = append(attrs, attribute.Int64("file.size.bytes", fileSize))
		t.memoryUsageHist.Record(ctx, memoryUsed, metric.WithAttributes(attrs...))
		t.putAttributes(attrs)
	}

	return nil
}

func (t *MemoryUsageTracker) detectMemorySpikes(ctx context.Context) {
	currentUsage := t.totalActive.Load()
	previousUsage := t.previousUsage.Load()
	t.previousUsage.Store(currentUsage)

	if previousUsage > 0 {
		increase := float64(currentUsage-previousUsage) / float64(previousUsage)
		if increase > 0.5 {
			alert := MemoryAlert{
				AlertType:    MemoryAlertSpikeDetected,
				CurrentUsage: currentUsage,
				Timestamp:    time.Now(),
				Metadata: map[string]interface{}{
					"increase_percentage": increase * 100,
				},
			}
			if err := t.GenerateMemoryAlert(ctx, alert); err != nil {
				slogger.Error(ctx, "Failed to generate memory spike alert", slogger.Fields{"error": err})
			}
		}
	}
}

func (t *MemoryUsageTracker) detectMemoryLeaks(ctx context.Context) {
	t.mutex.RLock()
	longRunningOps := make([]string, 0, len(t.activeMemory))
	for opID, usage := range t.activeMemory {
		if usage.Load() > 1000000 {
			longRunningOps = append(longRunningOps, opID)
		}
	}
	t.mutex.RUnlock()

	if len(longRunningOps) > 0 {
		if t.otelAvailable.Load() {
			t.memoryLeakCounter.Add(ctx, int64(len(longRunningOps)), metric.WithAttributes(t.config.MetricLabels...))
		}

		for _, opID := range longRunningOps {
			alert := MemoryAlert{
				OperationID:  opID,
				AlertType:    MemoryAlertPotentialLeak,
				CurrentUsage: t.activeMemory[opID].Load(),
				Timestamp:    time.Now(),
				Metadata: map[string]interface{}{
					"threshold_mb": 1,
				},
			}
			if err := t.GenerateMemoryAlert(ctx, alert); err != nil {
				slogger.Error(ctx, "Failed to generate memory leak alert", slogger.Fields{
					"operation_id": opID,
					"error":        err,
				})
			}
		}
	}
}

func (t *MemoryUsageTracker) updateTotalActive() {
	var total int64
	for _, v := range t.activeMemory {
		total += v.Load()
	}
	t.totalActive.Store(total)
}

func (t *MemoryUsageTracker) getAttributes(operationID string) []attribute.KeyValue {
	attrsPtr, ok := t.attrPool.Get().(*[]attribute.KeyValue)
	if !ok || attrsPtr == nil {
		attrs := make([]attribute.KeyValue, 0, 10)
		attrsPtr = &attrs
	}

	attrs := *attrsPtr
	capacity := len(t.config.MetricLabels) + 1
	if capacity > cap(attrs) {
		attrs = make([]attribute.KeyValue, capacity)
	} else {
		attrs = attrs[:capacity]
	}

	copy(attrs, t.config.MetricLabels)
	attrs[len(t.config.MetricLabels)] = attribute.String("operation.id", operationID)
	return attrs
}

func (t *MemoryUsageTracker) getAlertAttributes(alertType string) []attribute.KeyValue {
	attrsPtr, ok := t.attrPool.Get().(*[]attribute.KeyValue)
	if !ok || attrsPtr == nil {
		attrs := make([]attribute.KeyValue, 0, 10)
		attrsPtr = &attrs
	}

	attrs := *attrsPtr
	capacity := len(t.config.MetricLabels) + 1
	if capacity > cap(attrs) {
		attrs = make([]attribute.KeyValue, capacity)
	} else {
		attrs = attrs[:capacity]
	}

	copy(attrs, t.config.MetricLabels)
	attrs[len(t.config.MetricLabels)] = attribute.String("alert.type", alertType)
	return attrs
}

func (t *MemoryUsageTracker) putAttributes(attrs []attribute.KeyValue) {
	if len(attrs) > 0 {
		// Clear the slice elements and reset length to avoid memory leaks
		for i := range attrs {
			attrs[i] = attribute.KeyValue{}
		}
		attrs = attrs[:0]
		t.attrPool.Put(&attrs)
	}
}
