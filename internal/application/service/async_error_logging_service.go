package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// asyncErrorLoggingService implements NonBlockingErrorLogger interface.
type asyncErrorLoggingService struct {
	logger         logging.ApplicationLogger
	errorChan      chan *errorRequest
	stopChan       chan struct{}
	doneChan       chan struct{}
	isRunning      int32 // atomic
	processedCount int64 // atomic
	droppedCount   int64 // atomic
	bufferSize     int
	wg             sync.WaitGroup
}

// errorRequest represents an error to be processed asynchronously.
type errorRequest struct {
	ctx       context.Context
	err       error
	component string
	severity  *valueobject.ErrorSeverity
}

// NewNonBlockingErrorLoggingService creates a new async error logging service.
func NewNonBlockingErrorLoggingService(logger logging.ApplicationLogger, bufferSize int) NonBlockingErrorLogger {
	service := &asyncErrorLoggingService{
		logger:     logger,
		errorChan:  make(chan *errorRequest, bufferSize),
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		bufferSize: bufferSize,
	}

	// Start the background processing goroutine
	atomic.StoreInt32(&service.isRunning, 1)
	service.wg.Add(1)
	go service.processErrors()

	return service
}

// LogAndClassifyError processes an error asynchronously without blocking.
func (s *asyncErrorLoggingService) LogAndClassifyError(
	ctx context.Context,
	err error,
	component string,
	severity *valueobject.ErrorSeverity,
) error {
	if !s.IsRunning() {
		return errors.New("error logging service is not running")
	}

	request := &errorRequest{
		ctx:       ctx,
		err:       err,
		component: component,
		severity:  severity,
	}

	// Try to send non-blocking
	select {
	case s.errorChan <- request:
		return nil // Successfully queued
	default:
		// Buffer is full
		droppedCount := atomic.AddInt64(&s.droppedCount, 1)

		// Log warning about dropped errors periodically (every 10th drop)
		if droppedCount%10 == 1 {
			s.logger.Warn(ctx, "Dropping errors due to buffer overflow", logging.Fields{
				"dropped_errors": droppedCount,
				"buffer_size":    s.bufferSize,
				"component":      component,
			})
		}

		return errors.New("error buffer full, dropping error")
	}
}

// processErrors runs in a background goroutine to process error requests.
func (s *asyncErrorLoggingService) processErrors() {
	defer s.wg.Done()
	defer close(s.doneChan)

	for {
		select {
		case <-s.stopChan:
			// Drain remaining errors before stopping
			s.drainRemainingErrors()
			return
		case request := <-s.errorChan:
			s.processErrorRequest(request)
		}
	}
}

// drainRemainingErrors processes any remaining errors in the channel during shutdown.
func (s *asyncErrorLoggingService) drainRemainingErrors() {
	for {
		select {
		case request := <-s.errorChan:
			s.processErrorRequest(request)
		default:
			// No more errors to process
			return
		}
	}
}

// processErrorRequest processes a single error request.
func (s *asyncErrorLoggingService) processErrorRequest(request *errorRequest) {
	// Create classified error
	classifiedError, err := entity.NewClassifiedError(
		request.ctx,
		request.err,
		request.severity,
		"async_error",
		request.err.Error(),
		map[string]interface{}{
			"component": request.component,
		},
	)

	if err == nil {
		// Log the classified error
		fields := logging.Fields{
			"error_id":   classifiedError.ID(),
			"component":  request.component,
			"severity":   request.severity.String(),
			"error_code": classifiedError.ErrorCode(),
			"timestamp":  classifiedError.Timestamp(),
		}

		s.logger.Error(request.ctx, classifiedError.Message(), fields)
		atomic.AddInt64(&s.processedCount, 1)
	}
}

// Flush forces processing of all queued errors. May block until complete.
func (s *asyncErrorLoggingService) Flush() error {
	if !s.IsRunning() {
		return nil // Already shut down
	}

	// Wait for queue to be empty
	for s.GetQueueSize() > 0 {
		time.Sleep(time.Millisecond)
	}

	return nil
}

// ShutdownWithContext gracefully shuts down the service, processing remaining errors.
func (s *asyncErrorLoggingService) ShutdownWithContext(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.isRunning, 1, 0) {
		// Already shut down
		return nil
	}

	// Signal stop
	close(s.stopChan)

	// Wait for processing to complete or context timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil // Graceful shutdown completed
	case <-ctx.Done():
		return fmt.Errorf("shutdown timeout: %w", ctx.Err())
	}
}

// GetQueueSize returns current number of errors waiting to be processed.
func (s *asyncErrorLoggingService) GetQueueSize() int {
	return len(s.errorChan)
}

// GetProcessedCount returns total number of errors processed since startup.
func (s *asyncErrorLoggingService) GetProcessedCount() int64 {
	return atomic.LoadInt64(&s.processedCount)
}

// GetDroppedCount returns total number of errors dropped due to buffer overflow.
func (s *asyncErrorLoggingService) GetDroppedCount() int64 {
	return atomic.LoadInt64(&s.droppedCount)
}

// IsRunning returns true if the service is currently processing errors.
func (s *asyncErrorLoggingService) IsRunning() bool {
	return atomic.LoadInt32(&s.isRunning) == 1
}

// GetQueueStatus returns the current status of the error queue.
func (s *asyncErrorLoggingService) GetQueueStatus() (int, int) {
	capacity := s.bufferSize
	used := len(s.errorChan)
	return capacity, used
}

// LogError logs a classified error in a non-blocking manner.
func (s *asyncErrorLoggingService) LogError(err *entity.ClassifiedError) bool {
	if err == nil {
		return false
	}

	// Create error request
	request := &errorRequest{
		ctx:       context.Background(),
		err:       err.OriginalError(),
		component: err.Component(),
		severity:  err.Severity(),
	}

	select {
	case s.errorChan <- request:
		return true
	default:
		atomic.AddInt64(&s.droppedCount, 1)
		return false
	}
}

// Shutdown gracefully shuts down the error logger (interface method).
func (s *asyncErrorLoggingService) Shutdown() error {
	return s.ShutdownWithContext(context.Background())
}
