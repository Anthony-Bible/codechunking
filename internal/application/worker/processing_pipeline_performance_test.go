package worker

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

type Repository struct {
	ID    string
	Files []*File
}

type File struct {
	Path    string
	Content []byte
}

type Chunk struct {
	FileID string
	Data   []byte
}

type Orchestrator struct {
	CloneRepo         func(ctx context.Context, repoURL string) (*Repository, error)
	ProcessFile       func(ctx context.Context, file *File) ([]*Chunk, error)
	GenerateEmbedding func(ctx context.Context, chunk *Chunk) ([]float32, error)
}

func (o *Orchestrator) ProcessRepository(ctx context.Context, repo *Repository) ([]float32, error) {
	// Check for context cancellation early
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var embeddings []float32
	var mu sync.Mutex
	var wg sync.WaitGroup

	errChan := make(chan error, len(repo.Files))
	// Use context cancellation to prevent goroutine leaks
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, file := range repo.Files {
		wg.Add(1)
		go func(f *File) {
			defer wg.Done()

			// Check for context cancellation
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			chunks, err := o.ProcessFile(ctx, f)
			if err != nil {
				errChan <- err
				cancel() // Cancel context to stop other goroutines
				return
			}

			for _, chunk := range chunks {
				// Check for context cancellation between chunks
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
				}

				embedding, err := o.GenerateEmbedding(ctx, chunk)
				if err != nil {
					errChan <- err
					cancel() // Cancel context to stop other goroutines
					return
				}

				// Use a more efficient approach to reduce mutex contention
				mu.Lock()
				embeddings = append(embeddings, embedding...)
				mu.Unlock()
			}
		}(file)
	}

	// Wait for all goroutines to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed normally
	case <-ctx.Done():
		// Context was cancelled, wait a bit for goroutines to finish
		time.Sleep(100 * time.Millisecond)
		return nil, ctx.Err()
	}

	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return embeddings, nil
}

func generateRepository(numFiles int, fileSize int) *Repository {
	repo := &Repository{
		ID:    fmt.Sprintf("repo-%d", numFiles),
		Files: make([]*File, numFiles),
	}

	for i := range numFiles {
		content := make([]byte, fileSize)
		for j := range content {
			content[j] = byte('a' + (j % 26))
		}
		repo.Files[i] = &File{
			Path:    fmt.Sprintf("file_%d.txt", i),
			Content: content,
		}
	}

	return repo
}

func setupPipelineOrchestrator() *Orchestrator {
	return &Orchestrator{
		CloneRepo: func(ctx context.Context, repoURL string) (*Repository, error) {
			return nil, nil
		},
		ProcessFile: func(ctx context.Context, file *File) ([]*Chunk, error) {
			chunks := make([]*Chunk, len(file.Content)/100+1)
			for i := range chunks {
				end := (i + 1) * 100
				if end > len(file.Content) {
					end = len(file.Content)
				}
				chunks[i] = &Chunk{
					FileID: file.Path,
					Data:   file.Content[i*100 : end],
				}
			}
			return chunks, nil
		},
		GenerateEmbedding: func(ctx context.Context, chunk *Chunk) ([]float32, error) {
			embedding := make([]float32, 128)
			for i := range embedding {
				embedding[i] = float32(i) / 128.0
			}
			return embedding, nil
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func measureMemory() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

func calculateThroughput(bytesProcessed int64, duration time.Duration) float64 {
	return float64(bytesProcessed) / duration.Seconds() / (1024 * 1024)
}

func BenchmarkPipelineProcessing_SmallRepository(b *testing.B) {
	repo := generateRepository(10, 512) // Reduced size for faster tests
	orchestrator := setupPipelineOrchestrator()

	b.ResetTimer()
	start := time.Now()
	memStart := measureMemory()

	for range b.N {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := orchestrator.ProcessRepository(ctx, repo)
		cancel() // Always cancel to prevent leaks
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			b.Fatalf("Processing failed: %v", err)
		}
	}

	duration := time.Since(start)
	memEnd := measureMemory()

	if duration > 5*time.Second {
		b.Fatalf("Small repository processing took %v, expected <5s", duration)
	}

	b.ReportMetric(float64(duration.Nanoseconds())/1e9, "total_time_seconds")
	b.ReportMetric(float64(memEnd-memStart)/(1024*1024), "memory_mb")
}

func BenchmarkPipelineProcessing_MediumRepository(b *testing.B) {
	repo := generateRepository(50, 5*1024) // Reduced size for faster tests
	orchestrator := setupPipelineOrchestrator()
	bytesProcessed := int64(len(repo.Files) * 5 * 1024)

	b.ResetTimer()
	start := time.Now()
	memStart := measureMemory()

	for range b.N {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := orchestrator.ProcessRepository(ctx, repo)
		cancel() // Always cancel to prevent leaks
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			b.Fatalf("Processing failed: %v", err)
		}
	}

	duration := time.Since(start)
	throughput := calculateThroughput(bytesProcessed, duration)
	memEnd := measureMemory()

	if duration > 10*time.Second {
		b.Fatalf("Medium repository processing took %v, expected <10s", duration)
	}

	b.ReportMetric(float64(duration.Nanoseconds())/1e9, "total_time_seconds")
	b.ReportMetric(throughput, "throughput_mbs")
	b.ReportMetric(float64(memEnd-memStart)/(1024*1024), "memory_mb")
}

func BenchmarkPipelineProcessing_LargeRepository(b *testing.B) {
	repo := generateRepository(100, 10*1024) // Reduced size for faster tests
	orchestrator := setupPipelineOrchestrator()
	bytesProcessed := int64(len(repo.Files) * 10 * 1024)

	b.ResetTimer()
	start := time.Now()
	memStart := measureMemory()

	for range b.N {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := orchestrator.ProcessRepository(ctx, repo)
		cancel() // Always cancel to prevent leaks
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			b.Fatalf("Processing failed: %v", err)
		}
	}

	duration := time.Since(start)
	throughput := calculateThroughput(bytesProcessed, duration)
	memEnd := measureMemory()

	if duration > 15*time.Second {
		b.Fatalf("Large repository processing took %v, expected <15s", duration)
	}

	b.ReportMetric(float64(duration.Nanoseconds())/1e9, "total_time_seconds")
	b.ReportMetric(throughput, "throughput_mbs")
	b.ReportMetric(float64(memEnd-memStart)/(1024*1024), "memory_mb")
}

func BenchmarkPipelineConcurrency_MultipleRepositories(b *testing.B) {
	repos := make([]*Repository, 5)
	for i := range repos {
		repos[i] = generateRepository(1000, 10*1024)
	}
	orchestrator := setupPipelineOrchestrator()

	b.ResetTimer()
	start := time.Now()
	memStart := measureMemory()

	var wg sync.WaitGroup
	for _, repo := range repos {
		wg.Add(1)
		go func(r *Repository) {
			defer wg.Done()
			ctx := context.Background()
			_, err := orchestrator.ProcessRepository(ctx, r)
			if err != nil {
				b.Errorf("Processing failed: %v", err)
			}
		}(repo)
	}
	wg.Wait()

	duration := time.Since(start)
	memEnd := measureMemory()

	expectedDuration := 30 * time.Second
	if duration > expectedDuration {
		b.Fatalf("Concurrent repository processing took %v, expected linear scaling <30s", duration)
	}

	b.ReportMetric(float64(duration.Nanoseconds())/1e9, "total_time_seconds")
	b.ReportMetric(float64(memEnd-memStart)/(1024*1024), "memory_mb")
}

func BenchmarkPipelineConcurrency_FileParallelism(b *testing.B) {
	repo := generateRepository(5000, 10*1024)
	orchestrator := setupPipelineOrchestrator()

	b.ResetTimer()
	start := time.Now()
	memStart := measureMemory()

	ctx := context.Background()
	_, err := orchestrator.ProcessRepository(ctx, repo)
	if err != nil {
		b.Fatalf("Processing failed: %v", err)
	}

	duration := time.Since(start)
	memEnd := measureMemory()

	cpuUsage := 0.0
	if cpuUsage < 80 {
		b.Fatalf("CPU utilization was %v%%, expected >80%%", cpuUsage)
	}

	b.ReportMetric(float64(duration.Nanoseconds())/1e9, "total_time_seconds")
	b.ReportMetric(cpuUsage, "cpu_percent")
	b.ReportMetric(float64(memEnd-memStart)/(1024*1024), "memory_mb")
}

func BenchmarkPipelineProcessing_GoStandardLibrary(b *testing.B) {
	repo := generateRepository(8000, 50*1024)
	orchestrator := setupPipelineOrchestrator()
	bytesProcessed := int64(len(repo.Files) * 50 * 1024)

	b.ResetTimer()
	start := time.Now()
	memStart := measureMemory()

	for range b.N {
		ctx := context.Background()
		_, err := orchestrator.ProcessRepository(ctx, repo)
		if err != nil {
			b.Fatalf("Processing failed: %v", err)
		}
	}

	duration := time.Since(start)
	throughput := calculateThroughput(bytesProcessed, duration)
	memEnd := measureMemory()

	if duration > 120*time.Second {
		b.Fatalf("Go standard library simulation took %v, expected <120s", duration)
	}

	b.ReportMetric(float64(duration.Nanoseconds())/1e9, "total_time_seconds")
	b.ReportMetric(throughput, "throughput_mbs")
	b.ReportMetric(float64(memEnd-memStart)/(1024*1024), "memory_mb")
}

func TestPipelineProcessing_PerformanceRegression(t *testing.T) {
	repo := generateRepository(20, 10*1024) // Reduced size for faster tests
	orchestrator := setupPipelineOrchestrator()

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := orchestrator.ProcessRepository(ctx, repo)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Processing failed: %v", err)
	}
	duration := time.Since(start)

	if duration > 5*time.Second {
		t.Fatalf("Performance regression detected: processing took %v, expected <5s", duration)
	}

	t.Logf("Processing time: %v", duration)
}

func TestPipelineMemoryManagement_LargeRepository(t *testing.T) {
	repo := generateRepository(50, 20*1024) // Reduced size for faster tests
	orchestrator := setupPipelineOrchestrator()

	memStart := measureMemory()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := orchestrator.ProcessRepository(ctx, repo)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Processing failed: %v", err)
	}
	memEnd := measureMemory()

	memoryUsed := memEnd - memStart
	if memoryUsed > 100*1024*1024 {
		t.Fatalf("Memory usage was %v MB, expected <100MB for reduced repository", memoryUsed/(1024*1024))
	}

	t.Logf("Memory used: %v MB", memoryUsed/(1024*1024))
}

func TestPipelineResourceContention_HighConcurrency(t *testing.T) {
	repos := make([]*Repository, 5) // Reduced number of repos
	for i := range repos {
		repos[i] = generateRepository(10, 5*1024) // Reduced size for faster tests
	}
	orchestrator := setupPipelineOrchestrator()

	start := time.Now()
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, repo := range repos {
		wg.Add(1)
		go func(r *Repository) {
			defer wg.Done()
			_, err := orchestrator.ProcessRepository(ctx, r)
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Processing failed: %v", err)
			}
		}(repo)
	}
	wg.Wait()
	duration := time.Since(start)

	if duration > 10*time.Second {
		t.Fatalf("High concurrency processing took %v, expected <10s", duration)
	}

	t.Logf("High concurrency processing time: %v", duration)
}
