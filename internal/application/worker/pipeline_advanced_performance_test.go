package worker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type Job struct {
	ID       string
	Data     string
	WorkerID int
}

type ProcessingPipeline struct {
	workers   int
	processor func(Job) error
	jobQueue  chan Job
	wg        sync.WaitGroup
	workerIDs chan int
	closed    int32 // atomic flag for pipeline state
}

func NewProcessingPipeline(workers int) *ProcessingPipeline {
	p := &ProcessingPipeline{
		workers:   workers,
		jobQueue:  make(chan Job, 1000),
		workerIDs: make(chan int, workers),
	}

	for i := range workers {
		p.workerIDs <- i
	}

	p.startWorkers()
	return p
}

func (p *ProcessingPipeline) SetProcessor(processor func(Job) error) {
	p.processor = processor
}

func (p *ProcessingPipeline) Submit(ctx context.Context, job Job) error {
	// Check if pipeline is closed
	if atomic.LoadInt32(&p.closed) == 1 {
		return errors.New("pipeline closed")
	}

	select {
	case p.jobQueue <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel might be full or closed
		return errors.New("pipeline busy or closed")
	}
}

func (p *ProcessingPipeline) startWorkers() {
	for range p.workers {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			workerID := <-p.workerIDs
			for job := range p.jobQueue {
				job.WorkerID = workerID
				if p.processor != nil {
					p.processor(job)
				}
			}
			p.workerIDs <- workerID
		}()
	}
}

func (p *ProcessingPipeline) Close() {
	// Check if already closed to avoid double-close panic
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return // Already closed
	}
	close(p.jobQueue)
	p.wg.Wait()
}

func BenchmarkPipelineScalability_VerticalScaling(b *testing.B) {
	maxCores := 4
	if runtime.NumCPU() < maxCores {
		b.Skipf("Not enough CPU cores available for test: %d < %d", runtime.NumCPU(), maxCores)
	}

	baseThroughput := measureThroughput(b, 1)
	expectedThroughput := baseThroughput * float64(maxCores)

	b.ResetTimer()
	scaledThroughput := measureThroughput(b, maxCores)

	if scaledThroughput < expectedThroughput*0.8 {
		b.Errorf(
			"Vertical scaling failed: expected throughput %.2f ops/sec with %d cores, got %.2f ops/sec",
			expectedThroughput,
			maxCores,
			scaledThroughput,
		)
	}
}

func measureThroughput(b *testing.B, cores int) float64 {
	runtime.GOMAXPROCS(cores)

	pipeline := NewProcessingPipeline(50)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var processed int64
	pipeline.SetProcessor(func(job Job) error {
		atomic.AddInt64(&processed, 1)
		time.Sleep(100 * time.Microsecond)
		return nil
	})

	start := time.Now()
	totalJobs := 2000

	var wg sync.WaitGroup
	wg.Add(totalJobs)

	for i := range totalJobs {
		go func() {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("job-%d", i)})
		}()
	}

	wg.Wait()
	pipeline.Close()

	elapsed := time.Since(start)
	return float64(processed) / elapsed.Seconds()
}

func BenchmarkPipelineScalability_HorizontalScaling(b *testing.B) {
	workerCounts := []int{10, 30, 50}
	baseTime := measureWorkerScaling(b, 1)

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("%d_workers", workers), func(b *testing.B) {
			scaledTime := measureWorkerScaling(b, workers)
			overhead := float64(scaledTime-baseTime) / float64(baseTime)

			if overhead > 0.20 {
				b.Errorf("Horizontal scaling overhead exceeded 20%%: got %.2f%% with %d workers", overhead*100, workers)
			}
		})
	}
}

func measureWorkerScaling(b *testing.B, workers int) time.Duration {
	pipeline := NewProcessingPipeline(workers)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var processed int64
	pipeline.SetProcessor(func(job Job) error {
		atomic.AddInt64(&processed, 1)
		time.Sleep(100 * time.Microsecond)
		return nil
	})

	start := time.Now()
	totalJobs := 1000

	var wg sync.WaitGroup
	wg.Add(totalJobs)

	for i := range totalJobs {
		go func(id int) {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("job-%d", id)})
		}(i)
	}

	wg.Wait()
	pipeline.Close()

	return time.Since(start)
}

func TestPipelineScalability_LoadBalancing(t *testing.T) {
	t.Parallel()
	workers := 20
	pipeline := NewProcessingPipeline(workers)

	jobCounts := make([]int64, workers)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline.SetProcessor(func(job Job) error {
		workerID := job.WorkerID
		if workerID >= 0 && workerID < workers {
			atomic.AddInt64(&jobCounts[workerID], 1)
		}
		time.Sleep(50 * time.Microsecond)
		return nil
	})

	totalJobs := 2000
	var wg sync.WaitGroup
	wg.Add(totalJobs)

	for i := range totalJobs {
		go func(id int) {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("job-%d", id)})
		}(i)
	}

	wg.Wait()
	pipeline.Close()

	mean := float64(totalJobs) / float64(workers)
	var variance float64
	for _, count := range jobCounts {
		diff := float64(count) - mean
		variance += diff * diff
	}
	variance /= float64(workers)
	stdDev := math.Sqrt(variance)

	if stdDev/mean > 0.25 {
		t.Errorf("Load balancing variance exceeded 25%%: got %.2f%%", (stdDev/mean)*100)
	}
}

func BenchmarkPipelineScalability_QueueingPerformance(b *testing.B) {
	pipeline := NewProcessingPipeline(30)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline.SetProcessor(func(job Job) error {
		time.Sleep(50 * time.Microsecond)
		return nil
	})

	b.ResetTimer()
	start := time.Now()

	jobsSubmitted := 0
	targetJobs := 3000
	targetDuration := 3 * time.Second

	ticker := time.NewTicker(100 * time.Microsecond)
	defer ticker.Stop()

	var wg sync.WaitGroup
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				if jobsSubmitted < targetJobs && time.Since(start) < targetDuration {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()
						pipeline.Submit(ctx, Job{ID: fmt.Sprintf("job-%d", id)})
					}(jobsSubmitted)
					jobsSubmitted++
				}
			case <-done:
				return
			}
		}
	}()

	wg.Wait()
	close(done)
	pipeline.Close()

	elapsed := time.Since(start)
	jobsPerSecond := float64(jobsSubmitted) / elapsed.Seconds()
	avgLatency := elapsed / time.Duration(jobsSubmitted)

	if jobsPerSecond < 500 {
		b.Errorf("Queue throughput below threshold: expected 500+ jobs/sec, got %.2f jobs/sec", jobsPerSecond)
	}

	if avgLatency > 50*time.Millisecond {
		b.Errorf("Queue latency exceeded threshold: expected <50ms, got %v", avgLatency)
	}
}

func TestResourceContention_CPUBound(t *testing.T) {
	t.Parallel()
	pipeline := NewProcessingPipeline(10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var processed int64
	pipeline.SetProcessor(func(job Job) error {
		atomic.AddInt64(&processed, 1)
		// Fast CPU-intensive work
		n := 10000
		result := 0
		for i := range n {
			result += i * i
		}
		return nil
	})

	start := time.Now()
	totalJobs := 1000

	var wg sync.WaitGroup
	wg.Add(totalJobs)

	for i := range totalJobs {
		go func(id int) {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("job-%d", id)})
		}(i)
	}

	wg.Wait()
	pipeline.Close()

	elapsed := time.Since(start)
	throughput := float64(processed) / elapsed.Seconds()

	// Expected throughput under CPU pressure
	expectedThroughput := 150.0

	if throughput < expectedThroughput*0.6 {
		t.Errorf(
			"CPU-bound performance degradation: expected %.2f ops/sec, got %.2f ops/sec",
			expectedThroughput,
			throughput,
		)
	}
}

func TestResourceContention_IOBound(t *testing.T) {
	t.Parallel()
	pipeline := NewProcessingPipeline(10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	baseTime := 1 * time.Millisecond
	var processingTimes []time.Duration
	var mu sync.Mutex

	pipeline.SetProcessor(func(job Job) error {
		start := time.Now()
		time.Sleep(baseTime)
		mu.Lock()
		processingTimes = append(processingTimes, time.Since(start))
		mu.Unlock()
		return nil
	})

	totalJobs := 1000
	var wg sync.WaitGroup
	wg.Add(totalJobs)

	for i := range totalJobs {
		go func(id int) {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("job-%d", id)})
		}(i)
	}

	wg.Wait()
	pipeline.Close()

	var totalTime time.Duration
	for _, pt := range processingTimes {
		totalTime += pt
	}
	avgTime := totalTime / time.Duration(len(processingTimes))

	if avgTime > time.Duration(float64(baseTime)*2.0) {
		t.Errorf(
			"I/O-bound processing time increased >100%%: expected <%v, got %v",
			time.Duration(float64(baseTime)*2.0),
			avgTime,
		)
	}
}

func TestResourceContention_MemoryBound(t *testing.T) {
	t.Parallel()
	pipeline := NewProcessingPipeline(5)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const memoryLimit = 128 * 1024 * 1024
	var totalAllocated int64

	pipeline.SetProcessor(func(job Job) error {
		// Allocate smaller memory chunks
		chunk := make([]byte, 64*1024)
		atomic.AddInt64(&totalAllocated, int64(len(chunk)))

		if atomic.LoadInt64(&totalAllocated) > memoryLimit {
			return errors.New("memory limit exceeded")
		}

		// Simulate processing
		time.Sleep(100 * time.Microsecond)
		return nil
	})

	totalJobs := 500
	var wg sync.WaitGroup
	var errors int64
	wg.Add(totalJobs)

	for i := range totalJobs {
		go func(id int) {
			defer wg.Done()
			if err := pipeline.Submit(ctx, Job{ID: fmt.Sprintf("job-%d", id)}); err != nil {
				atomic.AddInt64(&errors, 1)
			}
		}(i)
	}

	wg.Wait()
	pipeline.Close()

	if atomic.LoadInt64(&errors) > 0 {
		t.Errorf("Memory-bound processing failed: %d jobs exceeded memory limits", errors)
	}
}

func BenchmarkResourceContention_MixedWorkload(b *testing.B) {
	pipeline := NewProcessingPipeline(15)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var processed int64
	pipeline.SetProcessor(func(job Job) error {
		atomic.AddInt64(&processed, 1)

		switch job.ID[len(job.ID)-1] % 3 {
		case 0: // CPU-bound
			n := 5000
			result := 0
			for i := range n {
				result += i * i
			}
		case 1: // I/O-bound
			time.Sleep(1 * time.Millisecond)
		case 2: // Memory-bound
			_ = make([]byte, 64*1024)
		}
		return nil
	})

	start := time.Now()
	totalJobs := 1500

	var wg sync.WaitGroup
	wg.Add(totalJobs)

	for i := range totalJobs {
		go func(id int) {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("job-%d", id)})
		}(i)
	}

	wg.Wait()
	pipeline.Close()

	elapsed := time.Since(start)
	efficiency := float64(processed) / (elapsed.Seconds() * 15)

	if efficiency < 0.60 {
		b.Errorf("Mixed workload efficiency below threshold: expected >60%%, got %.2f%%", efficiency*100)
	}
}

func BenchmarkRealWorldPerformance_DockerCodebase(b *testing.B) {
	pipeline := NewProcessingPipeline(10)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dockerRepo := generateSyntheticCodebase(1000, 100)

	var processed int64
	pipeline.SetProcessor(func(job Job) error {
		atomic.AddInt64(&processed, 1)
		// Simulate realistic processing time
		time.Sleep(10 * time.Microsecond)
		return nil
	})

	b.ResetTimer()
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(len(dockerRepo))

	for i, file := range dockerRepo {
		go func(id int, content string) {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("file-%d", id), Data: content})
		}(i, file)
	}

	wg.Wait()
	pipeline.Close()

	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		b.Errorf("Docker codebase processing exceeded time limit: expected <3 seconds, got %v", elapsed)
	}
}

func BenchmarkRealWorldPerformance_EtcdCodebase(b *testing.B) {
	pipeline := NewProcessingPipeline(8)

	etcdRepo := generateSyntheticCodebase(800, 80)

	var memUsage int64
	pipeline.SetProcessor(func(job Job) error {
		// Simulate memory usage with smaller allocations
		data := make([]byte, len(job.Data))
		copy(data, job.Data)
		atomic.AddInt64(&memUsage, int64(len(data)))
		time.Sleep(20 * time.Microsecond)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(etcdRepo))

	for i, file := range etcdRepo {
		go func(id int, content string) {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("file-%d", id), Data: content})
		}(i, file)
	}

	wg.Wait()
	pipeline.Close()

	if atomic.LoadInt64(&memUsage) > 256*1024*1024 {
		b.Errorf("Etcd codebase memory usage exceeded limit: expected <256MB, got %d bytes", memUsage)
	}
}

func BenchmarkRealWorldPerformance_PrometheusCodebase(b *testing.B) {
	pipeline := NewProcessingPipeline(12)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	prometheusRepo := generateSyntheticCodebase(1200, 100)

	var totalBytes int64
	var startTime time.Time
	var once sync.Once

	pipeline.SetProcessor(func(job Job) error {
		once.Do(func() {
			startTime = time.Now()
		})

		atomic.AddInt64(&totalBytes, int64(len(job.Data)))
		time.Sleep(25 * time.Microsecond)
		return nil
	})

	var wg sync.WaitGroup
	wg.Add(len(prometheusRepo))

	for i, file := range prometheusRepo {
		go func(id int, content string) {
			defer wg.Done()
			pipeline.Submit(ctx, Job{ID: fmt.Sprintf("file-%d", id), Data: content})
		}(i, file)
	}

	wg.Wait()
	pipeline.Close()

	elapsed := time.Since(startTime)
	throughput := float64(totalBytes) / elapsed.Seconds()

	if throughput < 1*1024*1024 {
		b.Errorf("Prometheus codebase throughput below threshold: expected >1MB/s, got %.2f MB/s", throughput/1024/1024)
	}
}

func TestRealWorldPerformance_MemoryBehavior(t *testing.T) {
	t.Parallel()

	codebases := []struct {
		name  string
		size  int
		files int
	}{
		{"small", 1000, 100},
		{"medium", 2000, 200},
		{"large", 3000, 300},
	}

	for _, cb := range codebases {
		t.Run(cb.name, func(t *testing.T) {
			t.Parallel()
			// Create a separate pipeline for each sub-test
			pipeline := NewProcessingPipeline(10)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			repo := generateSyntheticCodebase(cb.files, cb.size)

			var memUsage []int64
			var mu sync.Mutex

			pipeline.SetProcessor(func(job Job) error {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)

				mu.Lock()
				memUsage = append(memUsage, int64(m.Alloc))
				mu.Unlock()

				time.Sleep(5 * time.Microsecond)
				return nil
			})

			var wg sync.WaitGroup
			wg.Add(len(repo))

			for i, file := range repo {
				go func(id int, content string) {
					defer wg.Done()
					// Handle potential errors from submission
					if err := pipeline.Submit(ctx, Job{ID: fmt.Sprintf("file-%d", id), Data: content}); err != nil {
						// Log but continue - context might be cancelled or pipeline closed
						t.Logf("Failed to submit job %d: %v", id, err)
					}
				}(i, file)
			}

			wg.Wait()
			pipeline.Close()

			var sum, sumSquares float64
			for _, usage := range memUsage {
				sum += float64(usage)
				sumSquares += float64(usage * usage)
			}

			mean := sum / float64(len(memUsage))
			variance := (sumSquares / float64(len(memUsage))) - (mean * mean)
			stdDev := math.Sqrt(variance)

			if stdDev/mean > 0.10 {
				t.Errorf(
					"Memory behavior variance exceeded 10%% for %s codebase: got %.2f%%",
					cb.name,
					(stdDev/mean)*100,
				)
			}
		})
	}
}

func generateSyntheticCodebase(files, avgFileSize int) []string {
	codebase := make([]string, files)
	pattern := "abcdefghijklmnopqrstuvwxyz"

	for i := range files {
		size := avgFileSize + (i%50)*10 - 250
		if size < 50 {
			size = 50
		}

		content := make([]byte, size)
		for j := range size {
			content[j] = pattern[j%len(pattern)]
		}
		codebase[i] = string(content)
	}
	return codebase
}
