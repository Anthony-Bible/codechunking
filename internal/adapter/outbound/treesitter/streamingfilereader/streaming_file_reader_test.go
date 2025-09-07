package streamingfilereader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamingFileReader_NewReader_ConfigurableBufferSizes(t *testing.T) {
	t.Run("default buffer size", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{})
		require.NoError(t, err)
		require.NotNil(t, reader)
		assert.Equal(t, int64(64*1024), int64(cap(reader.buffer)))
	})

	t.Run("custom buffer size within limits", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 4 * 1024})
		require.NoError(t, err)
		require.NotNil(t, reader)
		assert.Equal(t, int64(4*1024), int64(cap(reader.buffer)))

		reader2, err := NewReader(tempFile, ReaderOptions{BufferSize: 1024 * 1024})
		require.NoError(t, err)
		require.NotNil(t, reader2)
		assert.Equal(t, int64(1024*1024), int64(cap(reader2.buffer)))
	})

	t.Run("buffer size below minimum", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 1024})
		assert.Error(t, err)
		assert.Nil(t, reader)
		assert.Contains(t, err.Error(), "buffer size must be between")
	})

	t.Run("buffer size above maximum", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 2 * 1024 * 1024})
		assert.Error(t, err)
		assert.Nil(t, reader)
		assert.Contains(t, err.Error(), "buffer size must be between")
	})

	t.Run("non-existent file", func(t *testing.T) {
		reader, err := NewReader("/non/existent/file.txt", ReaderOptions{})
		assert.Error(t, err)
		assert.Nil(t, reader)
		assert.Contains(t, err.Error(), "failed to open file")
	})
}

func TestStreamingFileReader_ReadChunk_SmallFiles(t *testing.T) {
	t.Run("read entire small file in one chunk", func(t *testing.T) {
		content := make([]byte, 10*1024)
		for i := range content {
			content[i] = byte(i % 256)
		}
		tempFile := createTempFileWithContent(t, content)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 64 * 1024})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()
		data, bytesRead, err := reader.ReadChunk(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(len(content)), bytesRead)
		assert.Equal(t, content, data)

		// Second read should return EOF
		data2, bytesRead2, err := reader.ReadChunk(ctx)
		assert.Error(t, err)
		assert.Equal(t, int64(0), bytesRead2)
		assert.Nil(t, data2)
		assert.Contains(t, err.Error(), "EOF")
	})

	t.Run("read small file in multiple chunks", func(t *testing.T) {
		content := make([]byte, 10*1024)
		for i := range content {
			content[i] = byte(i % 256)
		}
		tempFile := createTempFileWithContent(t, content)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 4 * 1024})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()

		// First chunk
		data1, bytesRead1, err := reader.ReadChunk(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(4*1024), bytesRead1)
		assert.Equal(t, content[:4*1024], data1)

		// Second chunk
		data2, bytesRead2, err := reader.ReadChunk(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(4*1024), bytesRead2)
		assert.Equal(t, content[4*1024:8*1024], data2)

		// Third chunk (partial)
		data3, bytesRead3, err := reader.ReadChunk(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2*1024), bytesRead3)
		assert.Equal(t, content[8*1024:], data3)

		// Fourth chunk should return EOF
		data4, bytesRead4, err := reader.ReadChunk(ctx)
		assert.Error(t, err)
		assert.Equal(t, int64(0), bytesRead4)
		assert.Nil(t, data4)
		assert.Contains(t, err.Error(), "EOF")
	})
}

func TestStreamingFileReader_ReadChunk_LargeFiles(t *testing.T) {
	t.Run("read 50MB file with default buffer", func(t *testing.T) {
		tempFile := createTempFile(t, 50*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()
		var totalBytes int64
		chunkCount := 0

		for {
			data, bytesRead, err := reader.ReadChunk(ctx)
			if err != nil {
				assert.Contains(t, err.Error(), "EOF")
				break
			}
			require.NotNil(t, data)
			assert.Equal(t, int64(64*1024), bytesRead)
			totalBytes += bytesRead
			chunkCount++
		}

		assert.Equal(t, int64(50*1024*1024), totalBytes)
		assert.Equal(t, 800, chunkCount)
	})

	t.Run("read 100MB file with custom buffer", func(t *testing.T) {
		tempFile := createTempFile(t, 100*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 256 * 1024})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()
		var totalBytes int64
		chunkCount := 0

		for {
			data, bytesRead, err := reader.ReadChunk(ctx)
			if err != nil {
				assert.Contains(t, err.Error(), "EOF")
				break
			}
			require.NotNil(t, data)
			assert.Equal(t, int64(256*1024), bytesRead)
			totalBytes += bytesRead
			chunkCount++
		}

		assert.Equal(t, int64(100*1024*1024), totalBytes)
		assert.Equal(t, 400, chunkCount)
	})
}

func TestStreamingFileReader_ReadChunk_VeryLargeFiles(t *testing.T) {
	t.Run("read 200MB file with adaptive buffer strategy", func(t *testing.T) {
		tempFile := createTempFile(t, 200*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 512 * 1024})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()
		var totalBytes int64
		chunkCount := 0

		for {
			data, bytesRead, err := reader.ReadChunk(ctx)
			if err != nil {
				assert.Contains(t, err.Error(), "EOF")
				break
			}
			require.NotNil(t, data)
			assert.Equal(t, int64(512*1024), bytesRead)
			totalBytes += bytesRead
			chunkCount++
		}

		assert.Equal(t, int64(200*1024*1024), totalBytes)
		assert.Equal(t, 400, chunkCount)
	})

	t.Run("read 500MB file with memory tracking enabled", func(t *testing.T) {
		tempFile := createTempFile(t, 500*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{
			BufferSize:           1024 * 1024,
			EnableMemoryTracking: true,
		})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()
		var totalBytes int64
		chunkCount := 0

		for {
			data, bytesRead, err := reader.ReadChunk(ctx)
			if err != nil {
				assert.Contains(t, err.Error(), "EOF")
				break
			}
			require.NotNil(t, data)
			assert.Equal(t, int64(1024*1024), bytesRead)
			totalBytes += bytesRead
			chunkCount++
		}

		assert.Equal(t, int64(500*1024*1024), totalBytes)
		assert.Equal(t, 500, chunkCount)
	})
}

func TestStreamingFileReader_MemoryTracking_DuringReading(t *testing.T) {
	t.Run("memory usage increases with each chunk read", func(t *testing.T) {
		content := make([]byte, 100*1024)
		for i := range content {
			content[i] = byte(i % 256)
		}
		tempFile := createTempFileWithContent(t, content)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{
			BufferSize:           32 * 1024,
			EnableMemoryTracking: true,
		})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()

		// Initial memory usage should be 0
		assert.Equal(t, int64(0), reader.GetMemoryUsage())

		// First chunk
		_, _, err = reader.ReadChunk(ctx)
		require.NoError(t, err)
		mem1 := reader.GetMemoryUsage()
		assert.Positive(t, mem1)

		// Second chunk
		_, _, err = reader.ReadChunk(ctx)
		require.NoError(t, err)
		mem2 := reader.GetMemoryUsage()
		assert.Greater(t, mem2, mem1)

		// Third chunk
		_, _, err = reader.ReadChunk(ctx)
		require.NoError(t, err)
		mem3 := reader.GetMemoryUsage()
		assert.Greater(t, mem3, mem2)

		// Fourth chunk
		_, _, err = reader.ReadChunk(ctx)
		require.NoError(t, err)
		mem4 := reader.GetMemoryUsage()
		assert.Greater(t, mem4, mem3)
	})

	t.Run("memory usage reset after file close", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{
			BufferSize:           64 * 1024,
			EnableMemoryTracking: true,
		})
		require.NoError(t, err)

		ctx := context.Background()
		_, _, err = reader.ReadChunk(ctx)
		require.NoError(t, err)
		assert.Positive(t, reader.GetMemoryUsage())

		err = reader.Close()
		require.NoError(t, err)
		assert.Equal(t, int64(0), reader.GetMemoryUsage())
	})
}

func TestStreamingFileReader_ResourceCleanup_FileHandles(t *testing.T) {
	t.Run("file handle properly closed", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{})
		require.NoError(t, err)
		require.NotNil(t, reader.file)

		// Verify file is initially accessible
		_, err = os.Stat(tempFile)
		assert.NoError(t, err)

		err = reader.Close()
		require.NoError(t, err)

		// After closing, file operations should still work (file handle released, not deleted)
		_, err = os.Stat(tempFile)
		assert.NoError(t, err)
	})

	t.Run("multiple close calls should not panic", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{})
		require.NoError(t, err)

		err = reader.Close()
		assert.NoError(t, err)

		err = reader.Close()
		assert.NoError(t, err)
	})
}

func TestStreamingFileReader_ContextCancellation_GracefulStop(t *testing.T) {
	t.Run("context cancellation stops reading", func(t *testing.T) {
		tempFile := createTempFile(t, 10*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 64 * 1024})
		require.NoError(t, err)
		defer reader.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		data, bytesRead, err := reader.ReadChunk(ctx)
		assert.Error(t, err)
		assert.Equal(t, int64(0), bytesRead)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("timeout context stops reading", func(t *testing.T) {
		tempFile := createTempFile(t, 50*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{BufferSize: 64 * 1024})
		require.NoError(t, err)
		defer reader.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		data, bytesRead, err := reader.ReadChunk(ctx)
		assert.Error(t, err)
		assert.Equal(t, int64(0), bytesRead)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestStreamingFileReader_ErrorHandling_FileAccess(t *testing.T) {
	t.Run("read from closed file returns error", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{})
		require.NoError(t, err)

		err = reader.Close()
		require.NoError(t, err)

		ctx := context.Background()
		data, bytesRead, err := reader.ReadChunk(ctx)
		assert.Error(t, err)
		assert.Equal(t, int64(0), bytesRead)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "file is closed")
	})

	t.Run("read from non-existent file returns error", func(t *testing.T) {
		reader, err := NewReader("/non/existent/file.txt", ReaderOptions{})
		assert.Error(t, err)
		assert.Nil(t, reader)
	})

	t.Run("read from directory returns error", func(t *testing.T) {
		tempDir := t.TempDir()

		reader, err := NewReader(tempDir, ReaderOptions{})
		assert.Error(t, err)
		assert.Nil(t, reader)
		assert.Contains(t, err.Error(), "not a regular file")
	})

	t.Run("read from unreadable file returns error", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		// Make file unreadable
		err := os.Chmod(tempFile, 0o200)
		require.NoError(t, err)

		reader, err := NewReader(tempFile, ReaderOptions{})
		assert.Error(t, err)
		assert.Nil(t, reader)
	})
}

func TestStreamingFileReader_ErrorHandling_MemoryLimits(t *testing.T) {
	t.Run("memory limit exceeded during reading", func(t *testing.T) {
		tempFile := createTempFile(t, 10*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{
			BufferSize:           64 * 1024,
			MemoryLimit:          10 * 1024,
			EnableMemoryTracking: true,
		})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()
		data, bytesRead, err := reader.ReadChunk(ctx)
		assert.Error(t, err)
		assert.Equal(t, int64(0), bytesRead)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "memory limit exceeded")
	})

	t.Run("memory limit of zero disables limit checking", func(t *testing.T) {
		tempFile := createTempFile(t, 1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{
			BufferSize:           64 * 1024,
			MemoryLimit:          0,
			EnableMemoryTracking: true,
		})
		require.NoError(t, err)
		defer reader.Close()

		ctx := context.Background()
		data, bytesRead, err := reader.ReadChunk(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(1024), bytesRead)
		assert.NotNil(t, data)
	})
}

func TestStreamingFileReader_BufferStrategy_AdaptiveSize(t *testing.T) {
	t.Run("small files use smaller buffer", func(t *testing.T) {
		tempFile := createTempFile(t, 10*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{})
		require.NoError(t, err)
		defer reader.Close()

		// For small files, buffer size should be smaller than default
		assert.LessOrEqual(t, int64(cap(reader.buffer)), int64(32*1024))
	})

	t.Run("large files use larger buffer", func(t *testing.T) {
		tempFile := createTempFile(t, 100*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{})
		require.NoError(t, err)
		defer reader.Close()

		// For large files, buffer size should be larger than default
		assert.GreaterOrEqual(t, int64(cap(reader.buffer)), int64(256*1024))
	})

	t.Run("very large files use maximum buffer", func(t *testing.T) {
		tempFile := createTempFile(t, 500*1024*1024)
		defer os.Remove(tempFile)

		reader, err := NewReader(tempFile, ReaderOptions{})
		require.NoError(t, err)
		defer reader.Close()

		// For very large files, buffer size should be at maximum
		assert.Equal(t, int64(1024*1024), int64(cap(reader.buffer)))
	})
}

// Helper functions.
func createTempFile(t *testing.T, size int64) string {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "testfile.txt")

	file, err := os.Create(tempFile)
	require.NoError(t, err)

	// Fill file with data
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	var written int64
	for written < size {
		toWrite := size - written
		if toWrite > 1024 {
			toWrite = 1024
		}
		n, err := file.Write(data[:toWrite])
		require.NoError(t, err)
		written += int64(n)
	}

	err = file.Close()
	require.NoError(t, err)

	return tempFile
}

func createTempFileWithContent(t *testing.T, content []byte) string {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "testfile.txt")

	err := os.WriteFile(tempFile, content, 0o644)
	require.NoError(t, err)

	return tempFile
}
