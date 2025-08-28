package language

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// BatchProcessor handles concurrent language detection for multiple files.
type BatchProcessor struct {
	detector   *Detector
	maxWorkers int
	batchSize  int
	tracer     trace.Tracer

	// Metrics
	batchCounter      metric.Int64Counter
	workerUtilization metric.Float64Gauge
	queueLength       metric.Int64Gauge
	throughputMeter   metric.Float64Counter
}

// NewBatchProcessor creates a new batch processor for language detection.
func NewBatchProcessor(detector *Detector) *BatchProcessor {
	maxWorkers := runtime.NumCPU()
	if maxWorkers > 10 {
		maxWorkers = 10 // Cap at 10 workers for reasonable resource usage
	}

	tracer := otel.Tracer("language-detection-batch")
	meter := otel.Meter("language-detection-batch")

	batchCounter, _ := meter.Int64Counter(
		"batch_operations_total",
		metric.WithDescription("Total number of batch detection operations"),
	)

	workerUtilization, _ := meter.Float64Gauge(
		"worker_utilization_ratio",
		metric.WithDescription("Ratio of active workers to total workers"),
	)

	queueLength, _ := meter.Int64Gauge(
		"queue_length",
		metric.WithDescription("Number of jobs waiting in the processing queue"),
	)

	throughputMeter, _ := meter.Float64Counter(
		"files_processed_total",
		metric.WithDescription("Total number of files processed"),
	)

	bp := &BatchProcessor{
		detector:          detector,
		maxWorkers:        maxWorkers,
		batchSize:         50, // Process files in batches of 50
		tracer:            tracer,
		batchCounter:      batchCounter,
		workerUtilization: workerUtilization,
		queueLength:       queueLength,
		throughputMeter:   throughputMeter,
	}

	return bp
}

// ProcessConcurrently processes files concurrently using worker goroutines.
func (bp *BatchProcessor) ProcessConcurrently(
	ctx context.Context,
	files []outbound.FileInfo,
	concurrency int,
) ([]outbound.DetectionResult, error) {
	ctx, span := bp.tracer.Start(ctx, "ProcessConcurrently")
	defer span.End()

	if concurrency <= 0 {
		concurrency = bp.maxWorkers
	}

	span.SetAttributes(
		attribute.Int("total_files", len(files)),
		attribute.Int("concurrency", concurrency),
	)

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		filesPerSecond := float64(len(files)) / duration.Seconds()
		bp.throughputMeter.Add(ctx, float64(len(files)))

		slogger.Info(ctx, "Batch processing completed", slogger.Fields{
			"total_files":      len(files),
			"duration_ms":      duration.Milliseconds(),
			"files_per_second": filesPerSecond,
			"concurrency_used": concurrency,
		})
	}()

	bp.batchCounter.Add(ctx, 1)

	// If small batch, process directly without complex orchestration
	if len(files) <= bp.batchSize {
		return bp.processSmallBatch(ctx, files)
	}

	return bp.processLargeBatch(ctx, files, concurrency)
}

// processSmallBatch handles small batches efficiently without worker pools.
func (bp *BatchProcessor) processSmallBatch(
	ctx context.Context,
	files []outbound.FileInfo,
) ([]outbound.DetectionResult, error) {
	ctx, span := bp.tracer.Start(ctx, "processSmallBatch")
	defer span.End()

	results := make([]outbound.DetectionResult, 0, len(files))

	for _, file := range files {
		result := bp.detector.createDetectionResult(ctx, file, time.Now())
		results = append(results, result)
	}

	return results, nil
}

// processLargeBatch handles large batches with worker pool and concurrency control.
func (bp *BatchProcessor) processLargeBatch(
	ctx context.Context,
	files []outbound.FileInfo,
	concurrency int,
) ([]outbound.DetectionResult, error) {
	ctx, span := bp.tracer.Start(ctx, "processLargeBatch")
	defer span.End()

	// Split files into chunks for processing
	chunks := bp.chunkFiles(files, bp.batchSize)
	results := make([]outbound.DetectionResult, 0, len(files))
	resultsChan := make(chan []outbound.DetectionResult, len(chunks))
	errorsChan := make(chan error, len(chunks))

	// Start worker pool
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	activeWorkers := 0
	for _, chunk := range chunks {
		wg.Add(1)

		go func(chunkFiles []outbound.FileInfo) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			activeWorkers++
			bp.workerUtilization.Record(ctx, float64(activeWorkers)/float64(concurrency))

			defer func() {
				<-semaphore // Release semaphore
				activeWorkers--
			}()

			chunkResults := make([]outbound.DetectionResult, 0, len(chunkFiles))
			for _, file := range chunkFiles {
				result := bp.detector.createDetectionResult(ctx, file, time.Now())
				chunkResults = append(chunkResults, result)
			}

			select {
			case resultsChan <- chunkResults:
			case <-ctx.Done():
				errorsChan <- ctx.Err()
				return
			}
		}(chunk)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	// Collect results
	for range chunks {
		select {
		case chunkResults := <-resultsChan:
			results = append(results, chunkResults...)
		case err := <-errorsChan:
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return results, nil
}

// chunkFiles splits files into smaller chunks for batch processing.
func (bp *BatchProcessor) chunkFiles(files []outbound.FileInfo, chunkSize int) [][]outbound.FileInfo {
	if chunkSize <= 0 {
		chunkSize = bp.batchSize
	}

	chunks := make([][]outbound.FileInfo, 0, (len(files)+chunkSize-1)/chunkSize)

	for i := 0; i < len(files); i += chunkSize {
		end := i + chunkSize
		if end > len(files) {
			end = len(files)
		}
		chunks = append(chunks, files[i:end])
	}

	return chunks
}

// GetMetrics returns current batch processing metrics.
func (bp *BatchProcessor) GetMetrics(_ context.Context) map[string]interface{} {
	return map[string]interface{}{
		"max_workers": bp.maxWorkers,
		"batch_size":  bp.batchSize,
	}
}
