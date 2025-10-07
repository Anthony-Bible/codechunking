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

func generateTestFileContent(size int) []byte {
	content := make([]byte, size)
	// Generate text-like content that won't be detected as binary
	// Use only printable ASCII characters (32-126) with minimal whitespace
	for i := range size {
		switch i % 20 {
		case 0:
			content[i] = ' ' // Space (0x20 is printable)
		case 10:
			content[i] = '\n' // Newline (keep minimal newlines)
		default:
			// Safe printable ASCII characters (48-122 = '0'-'z')
			content[i] = byte(48 + (i % (122 - 48 + 1)))
		}
	}
	return content
}

func BenchmarkStreamingProcessorThroughput_SmallFiles(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		reader := NewMockStreamingFileReader()
		enforcer := &MockMemoryLimitEnforcer{}
		detector := &MockLargeFileDetector{}
		tracker := &MockMemoryUsageTracker{}

		processor := NewStreamingCodeProcessor(reader, enforcer, detector, tracker)

		for i := range 1000 {
			path := fmt.Sprintf("test%d.go", i)
			content := generateTestFileContent(1024)
			reader.SetFileContent(path, content)
		}

		parser := NewMockTreeSitterParser()
		chunker := NewMockChunker()
		processor.Parsers["Go"] = parser
		processor.chunkers[outbound.StrategyFunction] = chunker

		ctx := context.Background()
		lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

		var totalBytes int64
		startTime := time.Now()

		for pb.Next() {
			path := fmt.Sprintf("test%d.go", totalBytes%1000)
			config := &outbound.ProcessingConfig{
				FilePath:         path,
				Language:         lang,
				ChunkingStrategy: outbound.StrategyFunction,
			}

			result, err := processor.ProcessFile(ctx, config)
			if err != nil {
				b.Fatal(err)
			}
			totalBytes += result.BytesProcessed
		}

		duration := time.Since(startTime)
		throughputMBps := float64(totalBytes) / (1024 * 1024) / duration.Seconds()

		// Note: Cannot use b.ReportMetric in RunParallel
		b.Logf("Throughput: %.2f MB/s, processed %d bytes", throughputMBps, totalBytes)
	})
}

func BenchmarkStreamingProcessorThroughput_MediumFiles(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		reader := NewMockStreamingFileReader()
		enforcer := &MockMemoryLimitEnforcer{}
		detector := &MockLargeFileDetector{}
		tracker := &MockMemoryUsageTracker{}

		processor := NewStreamingCodeProcessor(reader, enforcer, detector, tracker)

		for i := range 100 {
			path := fmt.Sprintf("test%d.go", i)
			content := generateTestFileContent(100 * 1024)
			reader.SetFileContent(path, content)
		}

		parser := NewMockTreeSitterParser()
		chunker := NewMockChunker()
		processor.Parsers["Go"] = parser
		processor.chunkers[outbound.StrategyFunction] = chunker

		ctx := context.Background()
		lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

		var totalBytes int64
		startTime := time.Now()

		for pb.Next() {
			path := fmt.Sprintf("test%d.go", totalBytes%100)
			config := &outbound.ProcessingConfig{
				FilePath:         path,
				Language:         lang,
				ChunkingStrategy: outbound.StrategyFunction,
			}

			result, err := processor.ProcessFile(ctx, config)
			if err != nil {
				b.Fatal(err)
			}
			totalBytes += result.BytesProcessed
		}

		duration := time.Since(startTime)
		throughputMBps := float64(totalBytes) / (1024 * 1024) / duration.Seconds()

		// Note: Cannot use b.ReportMetric in RunParallel
		b.Logf("Throughput: %.2f MB/s, processed %d bytes", throughputMBps, totalBytes)
	})
}

func BenchmarkStreamingProcessorThroughput_LargeFiles(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		reader := NewMockStreamingFileReader()
		enforcer := &MockMemoryLimitEnforcer{}
		detector := &MockLargeFileDetector{}
		tracker := &MockMemoryUsageTracker{}

		processor := NewStreamingCodeProcessor(reader, enforcer, detector, tracker)

		for i := range 10 {
			path := fmt.Sprintf("test%d.go", i)
			content := generateTestFileContent(10 * 1024 * 1024)
			reader.SetFileContent(path, content)
		}

		parser := NewMockTreeSitterParser()
		chunker := NewMockChunker()
		processor.Parsers["Go"] = parser
		processor.chunkers[outbound.StrategyFunction] = chunker

		ctx := context.Background()
		lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

		var totalBytes int64
		startTime := time.Now()

		for pb.Next() {
			path := fmt.Sprintf("test%d.go", totalBytes%10)
			config := &outbound.ProcessingConfig{
				FilePath:         path,
				Language:         lang,
				ChunkingStrategy: outbound.StrategyFunction,
			}

			result, err := processor.ProcessFile(ctx, config)
			if err != nil {
				b.Fatal(err)
			}
			totalBytes += result.BytesProcessed
		}

		duration := time.Since(startTime)
		throughputMBps := float64(totalBytes) / (1024 * 1024) / duration.Seconds()

		// Note: Cannot use b.ReportMetric in RunParallel
		b.Logf("Throughput: %.2f MB/s, processed %d bytes", throughputMBps, totalBytes)
	})
}

func BenchmarkStreamingProcessorMemory_LargeFiles(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		reader := NewMockStreamingFileReader()
		enforcer := &MockMemoryLimitEnforcer{}
		detector := &MockLargeFileDetector{}
		tracker := &MockMemoryUsageTracker{}

		processor := NewStreamingCodeProcessor(reader, enforcer, detector, tracker)

		for i := range 10 {
			path := fmt.Sprintf("test%d.go", i)
			content := generateTestFileContent(10 * 1024 * 1024)
			reader.SetFileContent(path, content)
		}

		parser := NewMockTreeSitterParser()
		chunker := NewMockChunker()
		processor.Parsers["Go"] = parser
		processor.chunkers[outbound.StrategyFunction] = chunker

		ctx := context.Background()
		lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

		var m runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m)
		initialMemory := m.Alloc

		for pb.Next() {
			path := fmt.Sprintf("test%d.go", b.N%10)
			config := &outbound.ProcessingConfig{
				FilePath:         path,
				Language:         lang,
				ChunkingStrategy: outbound.StrategyFunction,
			}

			_, err := processor.ProcessFile(ctx, config)
			if err != nil {
				b.Fatal(err)
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m)
		finalMemory := m.Alloc
		memoryGrowth := float64(finalMemory-initialMemory) / float64(initialMemory)

		// Note: Cannot use b.ReportMetric in RunParallel
		b.Logf("Memory growth: %.2f%%", memoryGrowth*100)
	})
}

func BenchmarkStreamingProcessorMemory_Concurrent(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		reader := NewMockStreamingFileReader()
		enforcer := &MockMemoryLimitEnforcer{}
		detector := &MockLargeFileDetector{}
		tracker := &MockMemoryUsageTracker{}

		processor := NewStreamingCodeProcessor(reader, enforcer, detector, tracker)

		for i := range 100 {
			path := fmt.Sprintf("test%d.go", i)
			content := generateTestFileContent(1024 * 1024)
			reader.SetFileContent(path, content)
		}

		parser := NewMockTreeSitterParser()
		chunker := NewMockChunker()
		processor.Parsers["Go"] = parser
		processor.chunkers[outbound.StrategyFunction] = chunker

		ctx := context.Background()
		lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

		var m runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m)
		initialMemory := m.Alloc

		for pb.Next() {
			var wg sync.WaitGroup
			for range 10 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for k := range 100 {
						path := fmt.Sprintf("test%d.go", k)
						config := &outbound.ProcessingConfig{
							FilePath:         path,
							Language:         lang,
							ChunkingStrategy: outbound.StrategyFunction,
						}

						_, err := processor.ProcessFile(ctx, config)
						if err != nil {
							b.Error(err)
						}
					}
				}()
			}
			wg.Wait()
		}

		runtime.GC()
		runtime.ReadMemStats(&m)
		finalMemory := m.Alloc
		memoryGrowth := float64(finalMemory-initialMemory) / float64(initialMemory)

		b.ReportMetric(memoryGrowth*100, "concurrent_memory_growth_%")
	})
}
