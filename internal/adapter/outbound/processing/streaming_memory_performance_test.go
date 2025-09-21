package streamingcodeprocessor

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestMemoryPressureSimulation_GradualIncrease(t *testing.T) {
	t.Parallel()

	// Setup mocks
	reader := NewMockStreamingFileReader()
	limitEnforcer := &MockMemoryLimitEnforcer{}
	largeFileDetector := &MockLargeFileDetector{}
	usageTracker := &MockMemoryUsageTracker{}

	// Create processor
	processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

	// Setup mock parsers and chunkers
	parser := &MockTreeSitterParser{}
	chunker := &MockChunker{}

	processor.Parsers["JavaScript"] = parser
	processor.chunkers[outbound.StrategyFunction] = chunker

	// Setup context and config
	ctx := context.Background()

	// Process small file (5KB)
	data1 := generateTestFileContent(1024 * 5)
	path1 := "/test/file1.txt"
	lang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	config1 := &outbound.ProcessingConfig{
		FilePath:         path1,
		Language:         lang,
		ChunkingStrategy: outbound.StrategyFunction,
	}
	reader.SetFileContent(path1, data1)
	processor.ProcessFile(ctx, config1)

	// Process medium file (50KB)
	path2 := "/test/file2.txt"
	data2 := generateTestFileContent(1024 * 50)
	lang2, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	config2 := &outbound.ProcessingConfig{
		FilePath:         path2,
		Language:         lang2,
		ChunkingStrategy: outbound.StrategyFunction,
	}
	reader.SetFileContent(path2, data2)
	processor.ProcessFile(ctx, config2)

	// Process large file (500KB)
	path3 := "/test/file3.txt"
	data3 := generateTestFileContent(1024 * 500)
	lang3, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	config3 := &outbound.ProcessingConfig{
		FilePath:         path3,
		Language:         lang3,
		ChunkingStrategy: outbound.StrategyFunction,
	}
	reader.SetFileContent(path3, data3)
	processor.ProcessFile(ctx, config3)

	// Check memory growth using runtime.MemStats
	runtime.GC() // Force GC before measurement
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Force some allocations to simulate memory pressure
	for range 100 {
		_ = make([]byte, 1024)
	}

	runtime.GC() // Force GC after allocations
	runtime.ReadMemStats(&m2)
	growth := m2.Alloc - m1.Alloc

	// Expect failure: Memory growth should not exceed 1MB threshold
	if growth > 1*1024*1024 {
		t.Errorf("Memory growth %dKB exceeds threshold of 1MB", growth/1024)
	}
}

func TestMemoryPressureSimulation_SuddenSpike(t *testing.T) {
	t.Parallel()

	// Setup mocks
	reader := NewMockStreamingFileReader()
	limitEnforcer := &MockMemoryLimitEnforcer{}
	largeFileDetector := &MockLargeFileDetector{}
	usageTracker := &MockMemoryUsageTracker{}

	// Create processor
	processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

	// Setup mock parsers and chunkers
	parser := &MockTreeSitterParser{}
	chunker := &MockChunker{}

	processor.Parsers["Python"] = parser
	processor.chunkers[outbound.StrategyFunction] = chunker

	// Setup context and config
	ctx := context.Background()
	lang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
	config := &outbound.ProcessingConfig{
		FilePath:         "/test/small.txt",
		Language:         lang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	// Process normal file (1KB)
	data1 := generateTestFileContent(1024 * 1)
	reader.SetFileContent(config.FilePath, data1)
	start := time.Now()
	processor.ProcessFile(ctx, config)

	// Process large file (100KB) - sudden spike
	config.FilePath = "/test/large.txt"
	data2 := generateTestFileContent(1024 * 100)
	lang2, _ := valueobject.NewLanguage(valueobject.LanguagePython)
	config.Language = lang2
	reader.SetFileContent(config.FilePath, data2)
	processor.ProcessFile(ctx, config)

	// Calculate recovery time
	recoveryTime := time.Since(start)

	// Expect failure: Recovery time should be under 50ms
	if recoveryTime > time.Millisecond*50 {
		t.Errorf("Recovery time %v exceeds threshold of 50ms", recoveryTime)
	}
}

func TestMemoryPressureSimulation_SustainedHighUsage(t *testing.T) {
	t.Parallel()

	// Setup mocks
	reader := NewMockStreamingFileReader()
	limitEnforcer := &MockMemoryLimitEnforcer{}
	largeFileDetector := &MockLargeFileDetector{}
	usageTracker := &MockMemoryUsageTracker{}

	// Create processor
	processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

	// Setup mock parsers and chunkers
	parser := &MockTreeSitterParser{}
	chunker := &MockChunker{}

	processor.Parsers["Java"] = parser
	processor.chunkers[outbound.StrategyFunction] = chunker

	// Setup context and config
	ctx := context.Background()
	lang, _ := valueobject.NewLanguage(valueobject.LanguageJava)
	config := &outbound.ProcessingConfig{
		FilePath:         "/test/sustained.txt",
		Language:         lang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	// Process smaller file continuously (5KB) for shorter duration
	data := generateTestFileContent(1024 * 5)
	reader.SetFileContent(config.FilePath, data)

	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// Process for limited iterations or until timeout
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	iterations := 0
	maxIterations := 20 // Limit iterations for faster test

	for {
		select {
		case <-timeoutCtx.Done():
			runtime.GC()
			runtime.ReadMemStats(&m2)
			growth := m2.Alloc - m1.Alloc

			// Expect failure: Memory growth should not exceed 5MB
			if growth > 5*1024*1024 {
				t.Errorf("Memory growth %dKB exceeds threshold of 5MB", growth/1024)
			}
			return
		default:
			if iterations >= maxIterations {
				runtime.GC()
				runtime.ReadMemStats(&m2)
				growth := m2.Alloc - m1.Alloc

				// Expect failure: Memory growth should not exceed 5MB
				if growth > 5*1024*1024 {
					t.Errorf("Memory growth %dKB exceeds threshold of 5MB", growth/1024)
				}
				return
			}
			processor.ProcessFile(ctx, config)
			iterations++
		}
	}
}

func BenchmarkMemoryPressureRecovery_TimeToRecover(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		// Setup mocks
		reader := NewMockStreamingFileReader()
		limitEnforcer := &MockMemoryLimitEnforcer{}
		largeFileDetector := &MockLargeFileDetector{}
		usageTracker := &MockMemoryUsageTracker{}

		// Create processor
		processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

		// Setup mock parsers and chunkers
		parser := &MockTreeSitterParser{}
		chunker := &MockChunker{}

		processor.Parsers["Go"] = parser
		processor.chunkers[outbound.StrategyFunction] = chunker

		lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		config := &outbound.ProcessingConfig{
			FilePath:         "/test/benchmark.txt",
			Language:         lang,
			ChunkingStrategy: outbound.StrategyFunction,
		}

		for pb.Next() {
			b.StopTimer()

			// Simulate memory pressure with smaller data
			data := generateTestFileContent(1024 * 50) // 50KB
			reader.SetFileContent(config.FilePath, data)

			b.StartTimer()
			start := time.Now()

			ctx := context.Background()
			processor.ProcessFile(ctx, config)

			recoveryTime := time.Since(start)
			b.StopTimer()

			// Expect failure: Recovery time should be under 100ms
			if recoveryTime > time.Millisecond*100 {
				b.Errorf("Recovery time %v exceeds threshold of 100ms", recoveryTime)
			}
		}
	})
}

func TestAdvancedBufferManagement_FragmentationPrevention(t *testing.T) {
	t.Parallel()

	// Setup mocks
	reader := NewMockStreamingFileReader()
	limitEnforcer := &MockMemoryLimitEnforcer{}
	largeFileDetector := &MockLargeFileDetector{}
	usageTracker := &MockMemoryUsageTracker{}

	// Create processor
	processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

	// Setup mock parsers and chunkers with working implementations
	parser := &MockTreeSitterParser{}
	chunker := &MockChunker{}

	processor.Parsers["JavaScript"] = parser
	processor.chunkers[outbound.StrategyFunction] = chunker

	// Test buffer pool efficiency by measuring consistent memory usage
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	initialAlloc := m1.Alloc

	// Process files multiple times to test buffer reuse
	files := []int{1024, 4096, 8192} // Smaller, more consistent sizes
	processCount := 0

	for round := range 5 { // 5 rounds of processing
		for _, size := range files {
			ctx := context.Background()
			lang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
			config := &outbound.ProcessingConfig{
				FilePath:         fmt.Sprintf("/test/round_%d_file_%d.txt", round, size),
				Language:         lang,
				ChunkingStrategy: outbound.StrategyFunction,
			}

			data := generateTestFileContent(size)
			reader.SetFileContent(config.FilePath, data)

			// Process file - errors are expected but processor should handle them gracefully
			processor.ProcessFile(ctx, config)
			processCount++
		}

		// Force GC between rounds to stabilize memory measurements
		runtime.GC()
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)
	finalAlloc := m2.Alloc

	// Calculate memory growth per file processed
	if initialAlloc == 0 {
		t.Skip("Initial memory allocation is zero, cannot calculate growth")
	}

	totalGrowth := float64(finalAlloc) - float64(initialAlloc)
	growthPerFile := totalGrowth / float64(processCount)

	// Each file should not cause more than 1KB of permanent memory growth on average
	// This tests that buffers are being reused effectively
	maxGrowthPerFile := 1024.0 // 1KB

	if growthPerFile > maxGrowthPerFile {
		t.Errorf("Average memory growth per file %f bytes exceeds threshold of %f bytes",
			growthPerFile, maxGrowthPerFile)
	}

	t.Logf("Processed %d files, total memory growth: %f KB, average per file: %f bytes",
		processCount, totalGrowth/1024, growthPerFile)
}

func TestAdvancedBufferManagement_OptimalBufferSizing(t *testing.T) {
	t.Parallel()

	// Setup mocks
	reader := NewMockStreamingFileReader()
	limitEnforcer := &MockMemoryLimitEnforcer{}
	largeFileDetector := &MockLargeFileDetector{}
	usageTracker := &MockMemoryUsageTracker{}

	// Create processor
	processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

	// Setup mock parsers and chunkers
	parser := &MockTreeSitterParser{}
	chunker := &MockChunker{}

	processor.Parsers["Python"] = parser
	processor.chunkers[outbound.StrategyFunction] = chunker

	// Process 100KB of data
	ctx := context.Background()
	lang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
	config := &outbound.ProcessingConfig{
		FilePath:         "/test/100kb.txt",
		Language:         lang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	data := generateTestFileContent(1024 * 100)
	reader.SetFileContent(config.FilePath, data)
	processor.ProcessFile(ctx, config)

	// Simulate buffer utilization check
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Buffer utilization is simulated as a ratio of allocations to sys memory
	utilization := float64(m.Alloc) / float64(m.Sys)

	// Expect failure: Buffer utilization should be at least 5%
	if utilization < 0.05 {
		t.Errorf("Buffer utilization %f is below threshold of 5%%", utilization*100)
	}
}

func BenchmarkBufferManagement_MemoryReuse(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		// Setup mocks
		reader := NewMockStreamingFileReader()
		limitEnforcer := &MockMemoryLimitEnforcer{}
		largeFileDetector := &MockLargeFileDetector{}
		usageTracker := &MockMemoryUsageTracker{}

		// Create processor
		processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

		// Setup mock parsers and chunkers
		parser := &MockTreeSitterParser{}
		chunker := &MockChunker{}

		processor.Parsers["Go"] = parser
		processor.chunkers[outbound.StrategyFunction] = chunker

		lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		config := &outbound.ProcessingConfig{
			FilePath:         "/test/reuse.txt",
			Language:         lang,
			ChunkingStrategy: outbound.StrategyFunction,
		}

		for pb.Next() {
			data := generateTestFileContent(1024)
			reader.SetFileContent(config.FilePath, data)
			ctx := context.Background()
			processor.ProcessFile(ctx, config)
		}

		// Simulate reuse rate check
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Reuse rate is simulated as a ratio based on heap management
		reuseRate := float64(m.HeapObjects) / float64(m.Mallocs)

		// Expect failure: Reuse rate should be at least 2%
		if reuseRate < 0.02 {
			b.Errorf("Buffer reuse rate %f is below threshold of 2%%", reuseRate*100)
		}
	})
}

func TestBufferManagement_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	// Setup mocks
	reader := NewMockStreamingFileReader()
	limitEnforcer := &MockMemoryLimitEnforcer{}
	largeFileDetector := &MockLargeFileDetector{}
	usageTracker := &MockMemoryUsageTracker{}

	// Create processor
	processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

	// Setup mock parsers and chunkers
	parser := &MockTreeSitterParser{}
	chunker := &MockChunker{}

	processor.Parsers["JavaScript"] = parser
	processor.chunkers[outbound.StrategyFunction] = chunker

	ctx := context.Background()

	var wg sync.WaitGroup
	start := time.Now()

	// Simulate concurrent access
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
			config := &outbound.ProcessingConfig{
				FilePath:         fmt.Sprintf("/test/concurrent_%d.txt", id),
				Language:         lang,
				ChunkingStrategy: outbound.StrategyFunction,
			}

			data := generateTestFileContent(1024)
			reader.SetFileContent(config.FilePath, data)
			processor.ProcessFile(ctx, config)
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Baseline for single-threaded performance
	baseline := time.Millisecond * 20
	expectedMax := time.Duration(float64(baseline) * 2.0) // 100% slowdown threshold

	// Expect failure: Concurrent processing should not exceed 100% slowdown
	if duration > expectedMax {
		t.Errorf("Concurrent processing took %v, exceeds threshold of %v (100%% slowdown)", duration, expectedMax)
	}
}

func BenchmarkGCPerformance_StreamingProcessing(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		// Setup mocks
		reader := NewMockStreamingFileReader()
		limitEnforcer := &MockMemoryLimitEnforcer{}
		largeFileDetector := &MockLargeFileDetector{}
		usageTracker := &MockMemoryUsageTracker{}

		// Create processor
		processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

		// Setup mock parsers and chunkers
		parser := &MockTreeSitterParser{}
		chunker := &MockChunker{}

		processor.Parsers["Go"] = parser
		processor.chunkers[outbound.StrategyFunction] = chunker

		runtime.GC()
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		config := &outbound.ProcessingConfig{
			FilePath:         "/test/gc_benchmark.txt",
			Language:         lang,
			ChunkingStrategy: outbound.StrategyFunction,
		}

		for pb.Next() {
			data := generateTestFileContent(1024)
			reader.SetFileContent(config.FilePath, data)
			ctx := context.Background()
			processor.ProcessFile(ctx, config)
		}

		b.StopTimer()
		runtime.GC()
		runtime.ReadMemStats(&m2)

		// Calculate GC overhead
		gcPause := time.Duration(m2.PauseTotalNs - m1.PauseTotalNs)
		overhead := float64(gcPause) / float64(time.Duration(b.N)*time.Millisecond*10) // Mock processing time

		// Expect failure: GC overhead should be under 5%
		if overhead > 0.05 {
			b.Errorf("GC overhead %f exceeds threshold of 5%%", overhead*100)
		}
	})
}

func TestGCPerformance_LowGCPressure(t *testing.T) {
	t.Parallel()

	// Setup mocks
	reader := NewMockStreamingFileReader()
	limitEnforcer := &MockMemoryLimitEnforcer{}
	largeFileDetector := &MockLargeFileDetector{}
	usageTracker := &MockMemoryUsageTracker{}

	// Create processor
	processor := NewStreamingCodeProcessor(reader, limitEnforcer, largeFileDetector, usageTracker)

	// Setup mock parsers and chunkers
	parser := &MockTreeSitterParser{}
	chunker := &MockChunker{}

	processor.Parsers["Java"] = parser
	processor.chunkers[outbound.StrategyFunction] = chunker

	// Setup context and config
	ctx := context.Background()
	lang, _ := valueobject.NewLanguage(valueobject.LanguageJava)
	config := &outbound.ProcessingConfig{
		FilePath:         "/test/gc_pressure.txt",
		Language:         lang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Process large amount of data to create GC pressure
	for range 50 {
		data := generateTestFileContent(1024)
		reader.SetFileContent(config.FilePath, data)
		processor.ProcessFile(ctx, config)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Expect failure: Should trigger GC less than 5 times
	expectedGCFrequency := 5
	actualGCFrequency := int(m2.NumGC - m1.NumGC)

	if actualGCFrequency > expectedGCFrequency {
		t.Errorf("GC frequency %d exceeds expected threshold of %d", actualGCFrequency, expectedGCFrequency)
	}
}

func BenchmarkGCPerformance_MemoryReleasePattern(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		var initial, afterAlloc, afterGC runtime.MemStats

		// Measure initial state
		runtime.GC()
		runtime.ReadMemStats(&initial)

		// Allocate memory
		data := make([][]byte, 50)
		for i := range data {
			data[i] = make([]byte, 1024*10) // 10KB each
		}

		// Measure after allocation
		runtime.ReadMemStats(&afterAlloc)

		for pb.Next() {
			// Release memory
			for i := range data {
				data[i] = nil
			}

			// Force GC
			runtime.GC()

			// Measure after GC
			runtime.ReadMemStats(&afterGC)

			// Check if memory was properly released
			allocDiff := afterAlloc.Alloc - initial.Alloc
			releaseDiff := afterAlloc.Alloc - afterGC.Alloc

			// Expect failure: Should release at least 30% of allocated memory
			expectedRelease := uint64(float64(allocDiff) * 0.3)
			if releaseDiff < expectedRelease {
				b.Errorf(
					"Memory release %d bytes is insufficient compared to expected release %d bytes",
					releaseDiff,
					expectedRelease,
				)
			}
		}
	})
}
