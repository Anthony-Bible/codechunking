package service

import (
	"sync"
	"time"
)

// ShutdownContextPool provides object pooling for shutdown-related structures
// to reduce memory allocations and GC pressure during shutdown operations.
type ShutdownContextPool struct {
	componentStatusPool sync.Pool
	metricsPool         sync.Pool
}

// NewShutdownContextPool creates a new object pool for shutdown operations.
func NewShutdownContextPool() *ShutdownContextPool {
	return &ShutdownContextPool{
		componentStatusPool: sync.Pool{
			New: func() interface{} {
				return &ComponentShutdownStatus{}
			},
		},
		metricsPool: sync.Pool{
			New: func() interface{} {
				return &ComponentShutdownMetrics{}
			},
		},
	}
}

// GetComponentStatus retrieves a ComponentShutdownStatus from the pool.
func (p *ShutdownContextPool) GetComponentStatus() *ComponentShutdownStatus {
	obj := p.componentStatusPool.Get()
	status, ok := obj.(*ComponentShutdownStatus)
	if !ok {
		status = &ComponentShutdownStatus{}
	}
	// Reset the status to clean state
	status.Name = ""
	status.Status = ""
	status.StartTime = time.Now()
	status.Duration = 0
	status.Error = ""
	status.ForceKilled = false
	return status
}

// PutComponentStatus returns a ComponentShutdownStatus to the pool.
func (p *ShutdownContextPool) PutComponentStatus(status *ComponentShutdownStatus) {
	if status != nil {
		p.componentStatusPool.Put(status)
	}
}

// GetMetrics retrieves a ComponentShutdownMetrics from the pool.
func (p *ShutdownContextPool) GetMetrics() *ComponentShutdownMetrics {
	obj := p.metricsPool.Get()
	metrics, ok := obj.(*ComponentShutdownMetrics)
	if !ok {
		metrics = &ComponentShutdownMetrics{}
	}
	// Reset the metrics to clean state
	metrics.Name = ""
	metrics.TotalShutdowns = 0
	metrics.SuccessfulShutdowns = 0
	metrics.FailedShutdowns = 0
	metrics.AverageTime = 0
	metrics.MaxTime = 0
	metrics.LastError = ""
	return metrics
}

// PutMetrics returns a ComponentShutdownMetrics to the pool.
func (p *ShutdownContextPool) PutMetrics(metrics *ComponentShutdownMetrics) {
	if metrics != nil {
		p.metricsPool.Put(metrics)
	}
}
