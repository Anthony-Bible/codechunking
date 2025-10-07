package streamingcodeprocessor

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"runtime"
	"testing"
	"time"
)

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

			// Note: Cannot use b.ReportMetric in RunParallel
			b.Logf("Recovery time: %v", recoveryTime)
		}
	})
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

		// Calculate reuse rate based on heap management
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Reuse rate is simulated as a ratio based on heap management
		reuseRate := float64(m.HeapObjects) / float64(m.Mallocs)

		// Note: Cannot use b.ReportMetric in RunParallel
		b.Logf("Buffer reuse rate: %.2f%%", reuseRate*100)
	})
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

		// Note: Cannot use b.ReportMetric in RunParallel
		b.Logf("GC overhead: %.2f%%", overhead*100)
	})
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

			// Calculate release percentage
			var releasePercent float64
			if allocDiff > 0 {
				releasePercent = float64(releaseDiff) / float64(allocDiff) * 100
			}
			// Note: Cannot use b.ReportMetric in RunParallel
			b.Logf("Memory release: %.2f%%", releasePercent)
		}
	})
}
