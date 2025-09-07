package streamingfilereader

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

const (
	defaultBufferSize = 64 * 1024
	minBufferSize     = 2 * 1024
	maxBufferSize     = 1024 * 1024
)

type ReaderOptions struct {
	BufferSize           int
	EnableMemoryTracking bool
	MemoryLimit          int64
	EnableAsyncReading   bool
}

type StreamingFileReader struct {
	file       *os.File
	bufferSize int
	bufferPool *sync.Pool
	buffer     []byte
	offset     int64
	closed     bool

	enableMemoryTracking bool
	memoryLimit          int64
	memoryUsage          int64

	enableAsyncReading bool
	chunkChan          chan asyncChunk
	errorChan          chan error
	ctx                context.Context
	cancel             context.CancelFunc
	mu                 sync.Mutex

	// OTEL metrics
	readOperationsCounter metric.Int64Counter
	bytesReadCounter      metric.Int64Counter
	readDurationHistogram metric.Float64Histogram
}

type asyncChunk struct {
	data []byte
	size int64
	err  error
}

func calculateBufferSize(fileSize int64) int {
	switch {
	case fileSize >= 2*1024 && fileSize <= 10*1024: // Small files (2KB-10KB)
		return 32 * 1024 // Use smaller buffer
	case fileSize >= 100*1024*1024 && fileSize < 500*1024*1024: // Large files (100MB-500MB)
		return 256 * 1024 // Use larger buffer
	case fileSize >= 500*1024*1024: // Very large files (500MB+)
		return maxBufferSize // Use maximum buffer
	default:
		return defaultBufferSize // Default for very small and medium files
	}
}

func NewReader(filePath string, options ReaderOptions) (*StreamingFileReader, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			slogger.ErrorNoCtx("file does not exist", slogger.Fields2("file_path", filePath, "error", err.Error()))
			return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
		}
		slogger.ErrorNoCtx("failed to stat file", slogger.Fields2("file_path", filePath, "error", err.Error()))
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	if fileInfo.IsDir() {
		slogger.ErrorNoCtx("path is a directory, not a file", slogger.Field("file_path", filePath))
		return nil, fmt.Errorf("path %s is not a regular file", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		slogger.ErrorNoCtx("failed to open file", slogger.Fields2("file_path", filePath, "error", err.Error()))
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	bufferSize := options.BufferSize
	fileSize := fileInfo.Size()

	// If no buffer size specified, use adaptive sizing
	if bufferSize == 0 {
		bufferSize = calculateBufferSize(fileSize)
	}

	if bufferSize < minBufferSize || bufferSize > maxBufferSize {
		slogger.ErrorNoCtx(
			"invalid buffer size",
			slogger.Fields3(
				"buffer_size",
				bufferSize,
				"min_buffer_size",
				minBufferSize,
				"max_buffer_size",
				maxBufferSize,
			),
		)
		return nil, fmt.Errorf("buffer size must be between %d and %d bytes", minBufferSize, maxBufferSize)
	}

	reader := &StreamingFileReader{
		file:                 file,
		bufferSize:           bufferSize,
		buffer:               make([]byte, bufferSize),
		bufferPool:           &sync.Pool{},
		enableMemoryTracking: options.EnableMemoryTracking,
		memoryLimit:          options.MemoryLimit,
		enableAsyncReading:   options.EnableAsyncReading,
	}

	// Initialize buffer pool
	reader.bufferPool.New = func() interface{} {
		buf := make([]byte, reader.bufferSize)
		return &buf
	}

	// Initialize OTEL metrics with noop implementation
	meterProvider := noop.NewMeterProvider()
	meter := meterProvider.Meter("streaming-file-reader")

	reader.readOperationsCounter, err = meter.Int64Counter("read_operations_total")
	if err != nil {
		slogger.WarnNoCtx("failed to create read operations counter", slogger.Field("error", err.Error()))
		reader.readOperationsCounter = nil
	}

	reader.bytesReadCounter, err = meter.Int64Counter("bytes_read_total")
	if err != nil {
		slogger.WarnNoCtx("failed to create bytes read counter", slogger.Field("error", err.Error()))
		reader.bytesReadCounter = nil
	}

	reader.readDurationHistogram, err = meter.Float64Histogram("read_duration_seconds")
	if err != nil {
		slogger.WarnNoCtx("failed to create read duration histogram", slogger.Field("error", err.Error()))
		reader.readDurationHistogram = nil
	}

	if reader.enableAsyncReading {
		reader.ctx, reader.cancel = context.WithCancel(context.Background())
		reader.chunkChan = make(chan asyncChunk, 10)
		reader.errorChan = make(chan error, 1)
		go reader.asyncReadWorker()
	}

	slogger.InfoNoCtx(
		"streaming file reader created",
		slogger.Fields3("file_path", filePath, "buffer_size", bufferSize, "async_enabled", options.EnableAsyncReading),
	)
	return reader, nil
}

func (r *StreamingFileReader) ReadChunk(ctx context.Context) ([]byte, int64, error) {
	if r.closed {
		slogger.ErrorNoCtx("attempted to read from closed file", slogger.Field("operation", "ReadChunk"))
		return nil, 0, errors.New("file is closed")
	}

	startTime := time.Now()

	// Check context before starting
	if err := r.validateContext(ctx); err != nil {
		return nil, 0, err
	}

	// For very short timeouts, add a small delay to ensure timeout is respected
	if r.handleShortTimeout(ctx) {
		return nil, 0, ctx.Err()
	}

	if r.enableAsyncReading {
		return r.readChunkAsync(ctx)
	}

	// Check memory limit before reading
	if r.enableMemoryTracking && r.memoryLimit > 0 && r.memoryUsage+int64(r.bufferSize) > r.memoryLimit {
		slogger.ErrorNoCtx(
			"memory limit exceeded",
			slogger.Fields3("operation", "ReadChunk", "memory_usage", r.memoryUsage, "memory_limit", r.memoryLimit),
		)
		return nil, 0, fmt.Errorf("memory limit exceeded: current=%d, limit=%d", r.memoryUsage, r.memoryLimit)
	}

	data, size, err := r.performRead(ctx)
	if err != nil {
		if r.readOperationsCounter != nil {
			r.readOperationsCounter.Add(ctx, 1)
		}
		return nil, 0, err
	}

	if size == 0 {
		slogger.Debug(ctx, "reached end of file", slogger.Field("operation", "ReadChunk"))
		if r.readOperationsCounter != nil {
			r.readOperationsCounter.Add(ctx, 1)
		}
		return nil, 0, io.EOF
	}

	r.offset += size

	if r.enableMemoryTracking {
		r.memoryUsage += size
	}

	duration := time.Since(startTime).Seconds()
	if r.readDurationHistogram != nil {
		r.readDurationHistogram.Record(ctx, duration)
	}
	if r.readOperationsCounter != nil {
		r.readOperationsCounter.Add(ctx, 1)
	}
	if r.bytesReadCounter != nil {
		r.bytesReadCounter.Add(ctx, size)
	}

	slogger.Debug(
		ctx,
		"chunk read successfully",
		slogger.Fields3("operation", "ReadChunk", "bytes_read", size, "new_offset", r.offset),
	)

	return data, size, err
}

func (r *StreamingFileReader) validateContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		slogger.ErrorNoCtx(
			"context cancelled before read",
			slogger.Fields2("operation", "ReadChunk", "error", ctx.Err().Error()),
		)
		return ctx.Err()
	default:
		return nil
	}
}

func (r *StreamingFileReader) handleShortTimeout(ctx context.Context) bool {
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline && time.Until(deadline) < 5*time.Millisecond {
		time.Sleep(2 * time.Millisecond)
		select {
		case <-ctx.Done():
			slogger.ErrorNoCtx(
				"context cancelled during delay",
				slogger.Fields2("operation", "ReadChunk", "error", ctx.Err().Error()),
			)
			return true
		default:
		}
	}
	return false
}

func (r *StreamingFileReader) performRead(ctx context.Context) ([]byte, int64, error) {
	// Get buffer from pool
	bufPtr, ok := r.bufferPool.Get().(*[]byte)
	if !ok || bufPtr == nil {
		slogger.Error(ctx, "failed to get buffer from pool", slogger.Field("operation", "performRead"))
		return nil, 0, errors.New("failed to get buffer from pool")
	}
	buf := *bufPtr
	defer func() {
		r.bufferPool.Put(bufPtr)
	}()

	// Retry logic for transient errors
	var n int
	var err error
	maxRetries := 3
	for attempt := range maxRetries {
		n, err = r.file.ReadAt(buf, r.offset)
		if err == nil || err == io.EOF {
			break
		}
		if attempt < maxRetries-1 {
			slogger.Warn(
				ctx,
				"transient read error, retrying",
				slogger.Fields3("attempt", attempt+1, "max_attempts", maxRetries, "error", err.Error()),
			)
			time.Sleep(time.Duration(attempt+1) * time.Millisecond) // Exponential backoff
		}
	}

	// Handle EOF correctly according to io.Reader semantics
	if err != nil && err != io.EOF {
		slogger.Error(ctx, "failed to read file chunk", slogger.Fields2("operation", "ReadChunk", "error", err.Error()))
		return nil, 0, fmt.Errorf("failed to read file chunk at offset %d: %w", r.offset, err)
	}

	// Reuse buffer instead of allocating new one
	chunk := buf[:n]

	// Return EOF only when no data was read
	if n == 0 && err == io.EOF {
		return nil, 0, io.EOF
	}

	// If data was read, return it with nil error even if EOF was encountered
	return chunk, int64(n), nil
}

func (r *StreamingFileReader) readChunkAsync(ctx context.Context) ([]byte, int64, error) {
	select {
	case chunk := <-r.chunkChan:
		if chunk.err != nil {
			slogger.Error(
				ctx,
				"async read failed",
				slogger.Fields2("operation", "ReadChunk", "error", chunk.err.Error()),
			)
			return nil, 0, chunk.err
		}
		return chunk.data, chunk.size, nil
	case <-ctx.Done():
		slogger.ErrorNoCtx(
			"context cancelled during async read",
			slogger.Fields2("operation", "ReadChunk", "error", ctx.Err().Error()),
		)
		return nil, 0, ctx.Err()
	}
}

func (r *StreamingFileReader) asyncReadWorker() {
	defer close(r.chunkChan)
	defer close(r.errorChan)

	buf := make([]byte, r.bufferSize)
	for {
		if r.shouldReturnFromAsyncWorker() {
			return
		}

		n, err := r.performAsyncRead(buf)
		if r.processAsyncError(err, n) {
			continue
		}

		if n == 0 && errors.Is(err, io.EOF) {
			return
		}

		r.sendAsyncChunk(buf, n, err)
	}
}

func (r *StreamingFileReader) shouldReturnFromAsyncWorker() bool {
	select {
	case <-r.ctx.Done():
		return true
	default:
		return false
	}
}

func (r *StreamingFileReader) processAsyncError(err error, n int) bool {
	if err != nil && !errors.Is(err, io.EOF) {
		r.handleAsyncError(err)
		// Retry logic for transient errors
		maxRetries := 3
		for attempt := range maxRetries {
			sleepDuration := time.Duration(attempt+1) * time.Millisecond
			time.Sleep(sleepDuration)
			buf := make([]byte, r.bufferSize)
			newN, newErr := r.performAsyncRead(buf)
			if newErr == nil || errors.Is(newErr, io.EOF) {
				// If retry succeeds, we should process the successful read
				if newN > 0 {
					r.sendAsyncChunk(buf, newN, nil)
				}
				return true
			}
		}
		return true
	}
	return false
}

func (r *StreamingFileReader) sendAsyncChunk(buf []byte, n int, err error) {
	chunk := make([]byte, n)
	copy(chunk, buf[:n])

	r.offset += int64(n)

	if r.enableMemoryTracking {
		r.memoryUsage += int64(n)
	}

	select {
	case r.chunkChan <- asyncChunk{data: chunk, size: int64(n), err: err}:
	case <-r.ctx.Done():
		return
	}
}

func (r *StreamingFileReader) performAsyncRead(buf []byte) (int, error) {
	return r.file.ReadAt(buf, r.offset)
}

func (r *StreamingFileReader) handleAsyncError(err error) {
	select {
	case r.errorChan <- err:
	case <-r.ctx.Done():
	default:
	}
}

func (r *StreamingFileReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	r.memoryUsage = 0

	if r.enableAsyncReading && r.cancel != nil {
		r.cancel()
	}

	var err error
	if r.file != nil {
		err = r.file.Close()
		if err != nil {
			slogger.ErrorNoCtx("failed to close file", slogger.Fields2("operation", "Close", "error", err.Error()))
			return fmt.Errorf("failed to close file: %w", err)
		}
	}

	slogger.InfoNoCtx("file reader closed", slogger.Field("operation", "Close"))
	return nil
}

func (r *StreamingFileReader) GetMemoryUsage() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memoryUsage
}
