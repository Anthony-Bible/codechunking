package streamingcodeprocessor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type MetricsCollector struct {
	metricCounter   int64
	processingTimes []time.Duration
	timesMutex      sync.RWMutex
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		processingTimes: make([]time.Duration, 0),
	}
}

func (mc *MetricsCollector) Collect() {
	atomic.AddInt64(&mc.metricCounter, 1)

	start := time.Now()
	// Simulate realistic lightweight metric collection work instead of sleep
	_ = atomic.LoadInt64(&mc.metricCounter)
	duration := time.Since(start)

	mc.timesMutex.Lock()
	mc.processingTimes = append(mc.processingTimes, duration)
	mc.timesMutex.Unlock()
}

func (mc *MetricsCollector) CollectLightweight() {
	atomic.AddInt64(&mc.metricCounter, 1)
}

func (mc *MetricsCollector) GetCounter() int64 {
	return atomic.LoadInt64(&mc.metricCounter)
}

func (mc *MetricsCollector) GetProcessingTimes() []time.Duration {
	mc.timesMutex.RLock()
	defer mc.timesMutex.RUnlock()

	result := make([]time.Duration, len(mc.processingTimes))
	copy(result, mc.processingTimes)
	return result
}

func (mc *MetricsCollector) Reset() {
	atomic.StoreInt64(&mc.metricCounter, 0)
	mc.timesMutex.Lock()
	mc.processingTimes = mc.processingTimes[:0]
	mc.timesMutex.Unlock()
}

func BenchmarkOTELMetricsPerformance_HighThroughput(b *testing.B) {
	mc := NewMetricsCollector()

	b.ResetTimer()
	start := time.Now()

	for range b.N {
		mc.Collect()
	}

	duration := time.Since(start)
	rate := float64(b.N) / duration.Seconds()

	b.ReportMetric(rate, "ops/sec")
}

func BenchmarkPerformanceAnalytics_RealtimeAnalysis(b *testing.B) {
	sample := PerformanceSample{Value: 100.0, Timestamp: time.Now()}

	b.ResetTimer()
	var totalDuration time.Duration
	for range b.N {
		start := time.Now()
		analyzePerformance(sample)
		totalDuration += time.Since(start)
	}

	avgDuration := totalDuration / time.Duration(b.N)
	b.ReportMetric(float64(avgDuration.Microseconds()), "avg_latency_μs")
}

func BenchmarkPerformanceAlerting_AlertProcessing(b *testing.B) {
	alerts := make([]Alert, 1000)
	for i := range alerts {
		alerts[i] = Alert{ID: i, Message: "test alert"}
	}

	b.ResetTimer()
	start := time.Now()

	for _, alert := range alerts {
		processAlert(alert)
	}

	duration := time.Since(start)
	rate := float64(len(alerts)) / duration.Seconds()

	b.ReportMetric(rate, "alerts/sec")
}

func BenchmarkObservability_EndToEndLatency(b *testing.B) {
	event := ObservabilityEvent{Timestamp: time.Now(), Data: "test data"}

	b.ResetTimer()
	var totalDuration time.Duration
	for range b.N {
		start := time.Now()
		processEventToEndToEnd(event)
		totalDuration += time.Since(start)
	}

	avgDuration := totalDuration / time.Duration(b.N)
	b.ReportMetric(float64(avgDuration.Microseconds()), "e2e_latency_μs")
}

// Mock implementations.
type PerformanceSample struct {
	Value     float64
	Timestamp time.Time
}

type Alert struct {
	ID      int
	Message string
}

type ObservabilityEvent struct {
	Timestamp time.Time
	Data      string
}

func analyzePerformance(sample PerformanceSample) {
	// Fast computational work
	_ = sample.Value * 2
}

func processAlert(alert Alert) {
	// Fast computational work
	_ = alert.ID + 1
}

func processEventToEndToEnd(event ObservabilityEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	// Simulate some processing with context checking
	for i := range 100 {
		select {
		case <-ctx.Done():
			return
		default:
			_ = float64(len(event.Data)) * float64(i)
		}
	}
}
