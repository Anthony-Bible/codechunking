package treesitter

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLargeFileDetector_DetectFileSize_SmallFiles(t *testing.T) {
	tempDir := t.TempDir()

	smallFile := filepath.Join(tempDir, "small.txt")
	err := os.WriteFile(smallFile, make([]byte, 1024*1024), 0o644) // 1MB file
	require.NoError(t, err)

	detector := NewDetector(DetectorConfig{})
	fileSize, err := detector.DetectFileSize(smallFile)
	require.NoError(t, err)

	assert.Equal(t, int64(1024*1024), fileSize.SizeBytes)
	assert.Equal(t, FileSizeSmall, fileSize.Category)
	assert.Equal(t, smallFile, fileSize.Path)
}

func TestLargeFileDetector_DetectFileSize_LargeFiles(t *testing.T) {
	tempDir := t.TempDir()

	largeFile := filepath.Join(tempDir, "large.txt")
	err := os.WriteFile(largeFile, make([]byte, 75*1024*1024), 0o644) // 75MB file
	require.NoError(t, err)

	detector := NewDetector(DetectorConfig{})
	fileSize, err := detector.DetectFileSize(largeFile)
	require.NoError(t, err)

	assert.Equal(t, int64(75*1024*1024), fileSize.SizeBytes)
	assert.Equal(t, FileSizeLarge, fileSize.Category)
	assert.Equal(t, largeFile, fileSize.Path)
}

func TestLargeFileDetector_DetectFileSize_VeryLargeFiles(t *testing.T) {
	tempDir := t.TempDir()

	veryLargeFile := filepath.Join(tempDir, "very_large.txt")
	err := os.WriteFile(veryLargeFile, make([]byte, 150*1024*1024), 0o644) // 150MB file
	require.NoError(t, err)

	detector := NewDetector(DetectorConfig{})
	fileSize, err := detector.DetectFileSize(veryLargeFile)
	require.NoError(t, err)

	assert.Equal(t, int64(150*1024*1024), fileSize.SizeBytes)
	assert.Equal(t, FileSizeVeryLarge, fileSize.Category)
	assert.Equal(t, veryLargeFile, fileSize.Path)
}

func TestLargeFileDetector_SelectStrategy_BasedOnSize(t *testing.T) {
	detector := NewDetector(DetectorConfig{})

	smallFile := FileSize{SizeBytes: 1024 * 1024, Category: FileSizeSmall, Path: "small.txt"}
	largeFile := FileSize{SizeBytes: 75 * 1024 * 1024, Category: FileSizeLarge, Path: "large.txt"}
	veryLargeFile := FileSize{SizeBytes: 150 * 1024 * 1024, Category: FileSizeVeryLarge, Path: "very_large.txt"}

	memoryConstraints := MemoryConstraints{
		AvailableMemory:      1024 * 1024 * 1024,
		MaxPerOperation:      512 * 1024 * 1024,
		SystemMemoryPressure: 0.3,
	}

	strategy, err := detector.SelectProcessingStrategy(smallFile, memoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyMemory, strategy)

	strategy, err = detector.SelectProcessingStrategy(largeFile, memoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyChunked, strategy)

	strategy, err = detector.SelectProcessingStrategy(veryLargeFile, memoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyStreaming, strategy)
}

func TestLargeFileDetector_SelectStrategy_BasedOnMemory(t *testing.T) {
	detector := NewDetector(DetectorConfig{})

	file := FileSize{SizeBytes: 30 * 1024 * 1024, Category: FileSizeSmall, Path: "file.txt"}

	// Low available memory should force streaming strategy
	lowMemoryConstraints := MemoryConstraints{
		AvailableMemory:      10 * 1024 * 1024,
		MaxPerOperation:      5 * 1024 * 1024,
		SystemMemoryPressure: 0.8,
	}

	strategy, err := detector.SelectProcessingStrategy(file, lowMemoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyStreaming, strategy)

	// High available memory should allow memory strategy
	highMemoryConstraints := MemoryConstraints{
		AvailableMemory:      1024 * 1024 * 1024,
		MaxPerOperation:      512 * 1024 * 1024,
		SystemMemoryPressure: 0.1,
	}

	strategy, err = detector.SelectProcessingStrategy(file, highMemoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyMemory, strategy)
}

func TestLargeFileDetector_FallbackStrategy_WhenMemoryExceeded(t *testing.T) {
	detector := NewDetector(DetectorConfig{})

	file := FileSize{SizeBytes: 200 * 1024 * 1024, Category: FileSizeVeryLarge, Path: "file.txt"}

	// Very low memory should trigger fallback strategy
	veryLowMemoryConstraints := MemoryConstraints{
		AvailableMemory:      1 * 1024 * 1024,
		MaxPerOperation:      512 * 1024,
		SystemMemoryPressure: 0.95,
	}

	strategy, err := detector.SelectProcessingStrategy(file, veryLowMemoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyFallback, strategy)
}

func TestLargeFileDetector_ConfigureThresholds_CustomSizes(t *testing.T) {
	config := DetectorConfig{
		SmallFileThreshold: 10 * 1024 * 1024,  // 10MB
		LargeFileThreshold: 200 * 1024 * 1024, // 200MB
		MemoryLimitPerFile: 512 * 1024 * 1024, // 512MB
		EnableFallback:     true,
	}

	detector := NewDetector(config)

	// File size between custom thresholds should be classified as Large
	file := FileSize{SizeBytes: 100 * 1024 * 1024, Category: FileSizeLarge, Path: "custom.txt"}

	memoryConstraints := MemoryConstraints{
		AvailableMemory:      1024 * 1024 * 1024,
		MaxPerOperation:      768 * 1024 * 1024,
		SystemMemoryPressure: 0.2,
	}

	strategy, err := detector.SelectProcessingStrategy(file, memoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyChunked, strategy)
}

func TestLargeFileDetector_ProcessingMode_Streaming(t *testing.T) {
	tempDir := t.TempDir()

	streamingFile := filepath.Join(tempDir, "streaming.txt")
	err := os.WriteFile(streamingFile, make([]byte, 200*1024*1024), 0o644) // 200MB file
	require.NoError(t, err)

	detector := NewDetector(DetectorConfig{})
	fileSize, err := detector.DetectFileSize(streamingFile)
	require.NoError(t, err)

	memoryConstraints := MemoryConstraints{
		AvailableMemory:      50 * 1024 * 1024,
		MaxPerOperation:      25 * 1024 * 1024,
		SystemMemoryPressure: 0.9,
	}

	strategy, err := detector.SelectProcessingStrategy(fileSize, memoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyStreaming, strategy)

	// Validate that streaming strategy is appropriate for this file
	err = detector.ValidateStrategy(strategy, fileSize)
	assert.NoError(t, err)
}

func TestLargeFileDetector_ProcessingMode_Chunked(t *testing.T) {
	tempDir := t.TempDir()

	chunkedFile := filepath.Join(tempDir, "chunked.txt")
	err := os.WriteFile(chunkedFile, make([]byte, 75*1024*1024), 0o644) // 75MB file
	require.NoError(t, err)

	detector := NewDetector(DetectorConfig{})
	fileSize, err := detector.DetectFileSize(chunkedFile)
	require.NoError(t, err)

	memoryConstraints := MemoryConstraints{
		AvailableMemory:      512 * 1024 * 1024,
		MaxPerOperation:      128 * 1024 * 1024,
		SystemMemoryPressure: 0.4,
	}

	strategy, err := detector.SelectProcessingStrategy(fileSize, memoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyChunked, strategy)

	// Validate that chunked strategy is appropriate for this file
	err = detector.ValidateStrategy(strategy, fileSize)
	assert.NoError(t, err)
}

func TestLargeFileDetector_ProcessingMode_Memory(t *testing.T) {
	tempDir := t.TempDir()

	memoryFile := filepath.Join(tempDir, "memory.txt")
	err := os.WriteFile(memoryFile, make([]byte, 10*1024*1024), 0o644) // 10MB file
	require.NoError(t, err)

	detector := NewDetector(DetectorConfig{})
	fileSize, err := detector.DetectFileSize(memoryFile)
	require.NoError(t, err)

	memoryConstraints := MemoryConstraints{
		AvailableMemory:      1024 * 1024 * 1024,
		MaxPerOperation:      512 * 1024 * 1024,
		SystemMemoryPressure: 0.1,
	}

	strategy, err := detector.SelectProcessingStrategy(fileSize, memoryConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyMemory, strategy)

	// Validate that memory strategy is appropriate for this file
	err = detector.ValidateStrategy(strategy, fileSize)
	assert.NoError(t, err)
}

func TestLargeFileDetector_StrategyFailure_GracefulFallback(t *testing.T) {
	detector := NewDetector(DetectorConfig{})

	// Simulate a strategy failure
	failedStrategy := ProcessingStrategyChunked
	failureReason := errors.New("chunked processing failed due to disk I/O error")

	fallbackStrategy, err := detector.GetFallbackStrategy(failedStrategy, failureReason)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyStreaming, fallbackStrategy)

	// If streaming also fails, should fallback to minimal processing
	failedStrategy = ProcessingStrategyStreaming
	failureReason = errors.New("streaming failed due to network error")

	fallbackStrategy, err = detector.GetFallbackStrategy(failedStrategy, failureReason)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyFallback, fallbackStrategy)
}

func TestLargeFileDetector_SystemResources_AdaptiveStrategy(t *testing.T) {
	detector := NewDetector(DetectorConfig{})

	file := FileSize{SizeBytes: 60 * 1024 * 1024, Category: FileSizeLarge, Path: "adaptive.txt"}

	// High memory pressure should force streaming even for large files
	highPressureConstraints := MemoryConstraints{
		AvailableMemory:      256 * 1024 * 1024,
		MaxPerOperation:      64 * 1024 * 1024,
		SystemMemoryPressure: 0.85,
	}

	strategy, err := detector.SelectProcessingStrategy(file, highPressureConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyStreaming, strategy)

	// Low memory pressure should allow chunked processing
	lowPressureConstraints := MemoryConstraints{
		AvailableMemory:      1024 * 1024 * 1024,
		MaxPerOperation:      256 * 1024 * 1024,
		SystemMemoryPressure: 0.15,
	}

	strategy, err = detector.SelectProcessingStrategy(file, lowPressureConstraints)
	require.NoError(t, err)
	assert.Equal(t, ProcessingStrategyChunked, strategy)
}
