package service

import (
	"codechunking/internal/domain/entity"
	"sync"
	"unsafe"
)

// circularErrorBuffer implements CircularErrorBuffer interface with thread-safe circular behavior.
type circularErrorBuffer struct {
	buffer   []*entity.ClassifiedError
	capacity int
	size     int
	head     int // Points to the next position to write
	tail     int // Points to the oldest element
	mu       sync.RWMutex
}

// NewCircularErrorBuffer creates a new circular error buffer with specified capacity.
func NewCircularErrorBuffer(capacity int) CircularErrorBuffer {
	if capacity <= 0 {
		return nil
	}

	return &circularErrorBuffer{
		buffer:   make([]*entity.ClassifiedError, capacity),
		capacity: capacity,
		size:     0,
		head:     0,
		tail:     0,
	}
}

// Add adds an error to the buffer. Returns false if buffer is nil or error is nil.
func (b *circularErrorBuffer) Add(err *entity.ClassifiedError) bool {
	if err == nil {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer[b.head] = err
	b.head = (b.head + 1) % b.capacity

	if b.size < b.capacity {
		b.size++
	} else {
		// Buffer is full, move tail forward (overwrite oldest)
		b.tail = (b.tail + 1) % b.capacity
	}

	return true
}

// Size returns the current number of errors in the buffer.
func (b *circularErrorBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size
}

// Capacity returns the maximum number of errors the buffer can hold.
func (b *circularErrorBuffer) Capacity() int {
	return b.capacity
}

// IsFull returns true when Size() equals Capacity().
func (b *circularErrorBuffer) IsFull() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size == b.capacity
}

// IsEmpty returns true when Size() equals 0.
func (b *circularErrorBuffer) IsEmpty() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.size == 0
}

// GetAll returns all errors in the buffer in order from oldest to newest.
func (b *circularErrorBuffer) GetAll() []*entity.ClassifiedError {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.size == 0 {
		return []*entity.ClassifiedError{}
	}

	result := make([]*entity.ClassifiedError, b.size)
	idx := 0

	// Start from tail (oldest) and read size elements
	for i := range b.size {
		pos := (b.tail + i) % b.capacity
		result[idx] = b.buffer[pos]
		idx++
	}

	return result
}

// GetBatch retrieves and removes up to 'count' errors from the buffer.
func (b *circularErrorBuffer) GetBatch(count int) []*entity.ClassifiedError {
	if count <= 0 {
		return []*entity.ClassifiedError{}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.size == 0 {
		return []*entity.ClassifiedError{}
	}

	// Get min(count, size) elements
	actualCount := count
	if actualCount > b.size {
		actualCount = b.size
	}

	result := make([]*entity.ClassifiedError, actualCount)

	// Get elements from tail (oldest first)
	for i := range actualCount {
		pos := (b.tail + i) % b.capacity
		result[i] = b.buffer[pos]
		b.buffer[pos] = nil // Clear reference to help GC
	}

	// Update tail and size
	b.tail = (b.tail + actualCount) % b.capacity
	b.size -= actualCount

	return result
}

// Clear removes all errors from the buffer and resets Size() to 0.
func (b *circularErrorBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Clear all references to help GC
	for i := range b.capacity {
		b.buffer[i] = nil
	}

	b.size = 0
	b.head = 0
	b.tail = 0
}

// GetMemoryUsage returns the approximate memory usage in bytes.
func (b *circularErrorBuffer) GetMemoryUsage() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Base memory for the struct and slice header
	baseMemory := int64(unsafe.Sizeof(*b)) + int64(unsafe.Sizeof(uintptr(0)))*int64(b.capacity)

	// Estimate memory for stored errors
	errorMemory := int64(0)
	for i := range b.size {
		pos := (b.tail + i) % b.capacity
		if b.buffer[pos] != nil {
			// Rough estimate: pointer + estimated error object size
			errorMemory += int64(unsafe.Sizeof(uintptr(0))) + estimateErrorSize(b.buffer[pos])
		}
	}

	return baseMemory + errorMemory
}

// estimateErrorSize provides a rough estimate of ClassifiedError memory usage.
func estimateErrorSize(err *entity.ClassifiedError) int64 {
	if err == nil {
		return 0
	}
	// Conservative estimate: 1KB per error (includes message, metadata, etc.)
	return 1024
}

// GetOldest returns the oldest error without removing it.
func (b *circularErrorBuffer) GetOldest() *entity.ClassifiedError {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.size == 0 {
		return nil
	}

	return b.buffer[b.tail]
}

// GetNewest returns the newest error without removing it.
func (b *circularErrorBuffer) GetNewest() *entity.ClassifiedError {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.size == 0 {
		return nil
	}

	// Newest is at head-1 (wrapped around)
	newestPos := (b.head - 1 + b.capacity) % b.capacity
	return b.buffer[newestPos]
}

// Clone creates a deep copy of the buffer with the same contents and configuration.
func (b *circularErrorBuffer) Clone() CircularErrorBuffer {
	b.mu.RLock()
	defer b.mu.RUnlock()

	clone := &circularErrorBuffer{
		buffer:   make([]*entity.ClassifiedError, b.capacity),
		capacity: b.capacity,
		size:     b.size,
		head:     b.head,
		tail:     b.tail,
	}

	// Deep copy the buffer contents
	copy(clone.buffer, b.buffer)

	return clone
}

// GetMemoryEstimate returns the estimated memory usage of the buffer.
func (b *circularErrorBuffer) GetMemoryEstimate() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Estimate memory usage:
	// - Fixed overhead for the struct
	// - Size of buffer slice (cap * pointer size)
	// - Estimated size of stored error messages
	const structOverhead = 64 // bytes for struct fields and mutex
	const pointerSize = 8     // bytes per pointer on 64-bit systems

	bufferMemory := int64(cap(b.buffer) * pointerSize)

	// Estimate average error message size (rough estimate)
	var messageMemory int64
	count := b.size
	if count > 0 {
		const averageErrorSize = 256 // estimated average bytes per error
		messageMemory = int64(count) * averageErrorSize
	}

	return structOverhead + bufferMemory + messageMemory
}
