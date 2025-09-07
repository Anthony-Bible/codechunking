package streamingcodeprocessor

import (
	"context"
	"runtime"
	"sync"
	"time"
)

type ProcessingPerformanceMetrics struct {
	TotalFilesProcessed   int64
	TotalBytesProcessed   int64
	TotalProcessingTime   time.Duration
	AverageProcessingTime time.Duration
	ThroughputMBPerSec    float64
	ThroughputFilesPerSec float64
}

type MemoryUsageStats struct {
	Alloc         uint64
	Sys           uint64
	GCCPUFraction float64
}

type PerformanceMonitor struct {
	mu       sync.Mutex
	memStats runtime.MemStats

	metrics ProcessingPerformanceMetrics
}

func (pm *PerformanceMonitor) StartMonitoring(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pm.mu.Lock()
				runtime.ReadMemStats(&pm.memStats)
				pm.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (pm *PerformanceMonitor) RecordProcessingMetrics(fileSize int64, duration time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.metrics.TotalFilesProcessed++
	pm.metrics.TotalBytesProcessed += fileSize
	pm.metrics.TotalProcessingTime += duration

	if pm.metrics.TotalFilesProcessed > 0 {
		pm.metrics.AverageProcessingTime = pm.metrics.TotalProcessingTime / time.Duration(
			pm.metrics.TotalFilesProcessed,
		)
	}

	pm.metrics.ThroughputMBPerSec = pm.CalculateThroughput()
	pm.metrics.ThroughputFilesPerSec = float64(
		pm.metrics.TotalFilesProcessed,
	) / pm.metrics.TotalProcessingTime.Seconds()
}

func (pm *PerformanceMonitor) GetCurrentMetrics() *ProcessingPerformanceMetrics {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return &pm.metrics
}

func (pm *PerformanceMonitor) CalculateThroughput() float64 {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.metrics.TotalProcessingTime.Seconds() == 0 {
		return 0
	}

	return float64(pm.metrics.TotalBytesProcessed) / (pm.metrics.TotalProcessingTime.Seconds() * 1024 * 1024)
}

func (pm *PerformanceMonitor) DetectMemoryPressure() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.memStats.GCCPUFraction > 0.5
}

func (pm *PerformanceMonitor) GetMemoryUsageStats() *MemoryUsageStats {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return &MemoryUsageStats{
		Alloc:         pm.memStats.Alloc,
		Sys:           pm.memStats.Sys,
		GCCPUFraction: pm.memStats.GCCPUFraction,
	}
}
