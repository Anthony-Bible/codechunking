package service

import (
	"sync"
	"time"
)

// RetryContext holds context information for a retry operation.
type RetryContext struct {
	OperationName string
	StartTime     time.Time
	Attempt       int
	LastError     error
	TotalDelay    time.Duration
	FailureType   FailureType
}

// Reset resets the retry context for reuse.
func (rc *RetryContext) Reset() {
	rc.OperationName = ""
	rc.StartTime = time.Time{}
	rc.Attempt = 0
	rc.LastError = nil
	rc.TotalDelay = 0
	rc.FailureType = FailureTypeUnknown
}

// RetryContextPool provides efficient pooling of RetryContext objects to reduce memory allocations.
type RetryContextPool struct {
	pool sync.Pool
}

// NewRetryContextPool creates a new retry context pool.
func NewRetryContextPool() *RetryContextPool {
	return &RetryContextPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &RetryContext{}
			},
		},
	}
}

// Get retrieves a retry context from the pool.
func (p *RetryContextPool) Get() *RetryContext {
	ctx, ok := p.pool.Get().(*RetryContext)
	if !ok {
		// This should never happen with our pool setup, but handle gracefully
		return &RetryContext{}
	}
	return ctx
}

// Put returns a retry context to the pool after resetting it.
func (p *RetryContextPool) Put(ctx *RetryContext) {
	if ctx != nil {
		ctx.Reset()
		p.pool.Put(ctx)
	}
}

// GetWithOperation retrieves a retry context from the pool and initializes it.
func (p *RetryContextPool) GetWithOperation(operationName string) *RetryContext {
	ctx := p.Get()
	ctx.OperationName = operationName
	ctx.StartTime = time.Now()
	return ctx
}

// PoolManager manages retry context pools without using global variables.
type PoolManager struct {
	pool *RetryContextPool
}

// NewPoolManager creates a new pool manager.
func NewPoolManager() *PoolManager {
	return &PoolManager{
		pool: NewRetryContextPool(),
	}
}

// GetContext gets a retry context from the managed pool.
func (pm *PoolManager) GetContext() *RetryContext {
	return pm.pool.Get()
}

// PutContext returns a retry context to the managed pool.
func (pm *PoolManager) PutContext(ctx *RetryContext) {
	pm.pool.Put(ctx)
}

// GetContextWithOperation gets a retry context from the managed pool with operation name.
func (pm *PoolManager) GetContextWithOperation(operationName string) *RetryContext {
	return pm.pool.GetWithOperation(operationName)
}

// WithContext executes a function with a pooled retry context.
func (pm *PoolManager) WithContext(operationName string, fn func(*RetryContext) error) error {
	retryCtx := pm.GetContextWithOperation(operationName)
	defer pm.PutContext(retryCtx)
	return fn(retryCtx)
}
