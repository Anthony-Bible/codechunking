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
	for i := range size {
		content[i] = byte(i % 256)
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

		if throughputMBps < 10.0 {
			b.Errorf("Throughput too low: %.2f MB/s, expected >10 MB/s", throughputMBps)
		}

		b.Logf("Processed %d files, throughput: %.2f MB/s", b.N, throughputMBps)
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

		if throughputMBps < 50.0 {
			b.Errorf("Throughput too low: %.2f MB/s, expected >50 MB/s", throughputMBps)
		}

		b.Logf("Processed %d files, throughput: %.2f MB/s", b.N, throughputMBps)
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

		if throughputMBps < 100.0 {
			b.Errorf("Throughput too low: %.2f MB/s, expected >100 MB/s", throughputMBps)
		}

		b.Logf("Processed %d files, throughput: %.2f MB/s", b.N, throughputMBps)
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

		if memoryGrowth > 0.5 {
			b.Errorf("Memory growth too high: %.2f%%, expected <50%%", memoryGrowth*100)
		}
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

		if memoryGrowth > 0.1 {
			b.Errorf("Concurrent memory growth too high: %.2f%%, expected stable usage", memoryGrowth*100)
		}
	})
}

func TestMemoryLimitEnforcement(t *testing.T) {
	t.Parallel()
	reader := NewMockStreamingFileReader()
	enforcer := &MockMemoryLimitEnforcer{}
	detector := &MockLargeFileDetector{}
	tracker := &MockMemoryUsageTracker{}

	processor := NewStreamingCodeProcessor(reader, enforcer, detector, tracker)

	path := "large.go"
	content := generateTestFileContent(10 * 1024 * 1024)
	reader.SetFileContent(path, content)

	parser := NewMockTreeSitterParser()
	chunker := NewMockChunker()
	processor.Parsers["Go"] = parser
	processor.chunkers[outbound.StrategyFunction] = chunker

	ctx := context.Background()
	lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

	config := &outbound.ProcessingConfig{
		FilePath:         path,
		Language:         lang,
		ChunkingStrategy: outbound.StrategyFunction,
	}

	_, err := processor.ProcessFile(ctx, config)
	if err == nil {
		t.Fatal("Expected memory limit enforcement to fail processing of large file")
	}
}

func TestMemoryLeakPrevention(t *testing.T) {
	t.Parallel()
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

	for range 10 {
		for j := range 100 {
			path := fmt.Sprintf("test%d.go", j)
			config := &outbound.ProcessingConfig{
				FilePath:         path,
				Language:         lang,
				ChunkingStrategy: outbound.StrategyFunction,
			}

			if _, err := processor.ProcessFile(ctx, config); err != nil {
				t.Fatal(err)
			}
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m)
	finalMemory := m.Alloc
	memoryGrowth := float64(finalMemory-initialMemory) / float64(initialMemory)

	if memoryGrowth > 0.05 {
		t.Errorf("Memory leak detected: %.2f%% memory growth, expected no leak", memoryGrowth*100)
	}
}
