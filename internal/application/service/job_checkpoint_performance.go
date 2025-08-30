package service

import (
	"codechunking/internal/domain/entity"
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CheckpointPool provides object pooling for checkpoint data structures
// to reduce memory allocations and garbage collection overhead.
type CheckpointPool struct {
	checkpointPool    *sync.Pool
	progressPool      *sync.Pool
	metadataPool      *sync.Pool
	resumePointPool   *sync.Pool
	resourceUsagePool *sync.Pool
}

// NewCheckpointPool creates a new checkpoint object pool.
func NewCheckpointPool() *CheckpointPool {
	return &CheckpointPool{
		checkpointPool: &sync.Pool{
			New: func() interface{} {
				return &JobCheckpoint{
					ProcessedFiles:  make([]string, 0, 16),
					CompletedChunks: make([]ChunkReference, 0, 64),
				}
			},
		},
		progressPool: &sync.Pool{
			New: func() interface{} {
				return &JobProgress{
					ProcessedFiles:    make([]string, 0, 16),
					CompletedChunks:   make([]ChunkReference, 0, 64),
					ThroughputMetrics: make(map[string]float64),
				}
			},
		},
		metadataPool: &sync.Pool{
			New: func() interface{} {
				return &CheckpointMetadata{
					ValidationChecks: make(map[string]bool),
					CustomProperties: make(map[string]interface{}),
				}
			},
		},
		resumePointPool: &sync.Pool{
			New: func() interface{} {
				return &ResumePoint{
					ProcessingFlags: make(map[string]interface{}),
					Dependencies:    make([]string, 0, 8),
				}
			},
		},
		resourceUsagePool: &sync.Pool{
			New: func() interface{} {
				return &ResourceUsage{}
			},
		},
	}
}

// GetCheckpoint retrieves a checkpoint from the pool.
func (p *CheckpointPool) GetCheckpoint() *JobCheckpoint {
	raw := p.checkpointPool.Get()
	if raw == nil {
		panic("checkpoint pool returned nil - pool not properly initialized")
	}
	cp, ok := raw.(*JobCheckpoint)
	if !ok {
		panic("checkpoint pool returned wrong type")
	}
	p.resetCheckpoint(cp)
	return cp
}

// PutCheckpoint returns a checkpoint to the pool.
func (p *CheckpointPool) PutCheckpoint(cp *JobCheckpoint) {
	if cp != nil {
		p.checkpointPool.Put(cp)
	}
}

// GetProgress retrieves a job progress from the pool.
func (p *CheckpointPool) GetProgress() *JobProgress {
	raw := p.progressPool.Get()
	if raw == nil {
		panic("progress pool returned nil - pool not properly initialized")
	}
	progress, ok := raw.(*JobProgress)
	if !ok {
		panic("progress pool returned wrong type")
	}
	p.resetProgress(progress)
	return progress
}

// PutProgress returns a job progress to the pool.
func (p *CheckpointPool) PutProgress(progress *JobProgress) {
	if progress != nil {
		p.progressPool.Put(progress)
	}
}

// GetMetadata retrieves metadata from the pool.
func (p *CheckpointPool) GetMetadata() *CheckpointMetadata {
	raw := p.metadataPool.Get()
	if raw == nil {
		panic("metadata pool returned nil - pool not properly initialized")
	}
	metadata, ok := raw.(*CheckpointMetadata)
	if !ok {
		panic("metadata pool returned wrong type")
	}
	p.resetMetadata(metadata)
	return metadata
}

// PutMetadata returns metadata to the pool.
func (p *CheckpointPool) PutMetadata(metadata *CheckpointMetadata) {
	if metadata != nil {
		p.metadataPool.Put(metadata)
	}
}

// GetResumePoint retrieves a resume point from the pool.
func (p *CheckpointPool) GetResumePoint() *ResumePoint {
	raw := p.resumePointPool.Get()
	if raw == nil {
		panic("resume point pool returned nil - pool not properly initialized")
	}
	rp, ok := raw.(*ResumePoint)
	if !ok {
		panic("resume point pool returned wrong type")
	}
	p.resetResumePoint(rp)
	return rp
}

// PutResumePoint returns a resume point to the pool.
func (p *CheckpointPool) PutResumePoint(rp *ResumePoint) {
	if rp != nil {
		p.resumePointPool.Put(rp)
	}
}

// GetResourceUsage retrieves resource usage from the pool.
func (p *CheckpointPool) GetResourceUsage() *ResourceUsage {
	raw := p.resourceUsagePool.Get()
	if raw == nil {
		panic("resource usage pool returned nil - pool not properly initialized")
	}
	usage, ok := raw.(*ResourceUsage)
	if !ok {
		panic("resource usage pool returned wrong type")
	}
	p.resetResourceUsage(usage)
	return usage
}

// PutResourceUsage returns resource usage to the pool.
func (p *CheckpointPool) PutResourceUsage(usage *ResourceUsage) {
	if usage != nil {
		p.resourceUsagePool.Put(usage)
	}
}

// resetCheckpoint clears checkpoint data for reuse.
func (p *CheckpointPool) resetCheckpoint(cp *JobCheckpoint) {
	cp.ID = uuid.Nil
	cp.JobID = uuid.Nil
	cp.Stage = ""
	cp.ResumePoint = nil
	cp.Metadata = nil
	cp.ProcessedFiles = cp.ProcessedFiles[:0]
	cp.CompletedChunks = cp.CompletedChunks[:0]
	cp.FailureContext = nil
	cp.CreatedAt = time.Time{}
	cp.UpdatedAt = time.Time{}
	cp.ValidatedAt = nil
	cp.ChecksumHash = ""
	cp.CompressedData = nil
	cp.IsCorrupted = false
}

// resetProgress clears progress data for reuse.
func (p *CheckpointPool) resetProgress(progress *JobProgress) {
	progress.FilesProcessed = 0
	progress.ChunksGenerated = 0
	progress.EmbeddingsCreated = 0
	progress.BytesProcessed = 0
	progress.CurrentFile = ""
	progress.Stage = ""
	progress.ProcessedFiles = progress.ProcessedFiles[:0]
	progress.CompletedChunks = progress.CompletedChunks[:0]
	progress.ErrorCount = 0
	progress.LastError = nil
	progress.StartTime = time.Time{}
	progress.StageStartTime = time.Time{}
	progress.EstimatedCompletion = nil
	for k := range progress.ThroughputMetrics {
		delete(progress.ThroughputMetrics, k)
	}
	progress.ResourceUsage = nil
}

// resetMetadata clears metadata for reuse.
func (p *CheckpointPool) resetMetadata(metadata *CheckpointMetadata) {
	metadata.Version = ""
	metadata.CheckpointType = ""
	metadata.Granularity = ""
	metadata.WorkerID = ""
	metadata.WorkerVersion = ""
	metadata.CompressionType = ""
	metadata.EncryptionEnabled = false
	metadata.DataSize = 0
	for k := range metadata.ValidationChecks {
		delete(metadata.ValidationChecks, k)
	}
	for k := range metadata.CustomProperties {
		delete(metadata.CustomProperties, k)
	}
}

// resetResumePoint clears resume point for reuse.
func (p *CheckpointPool) resetResumePoint(rp *ResumePoint) {
	rp.Stage = ""
	rp.CurrentFile = ""
	rp.FileIndex = 0
	rp.ChunkIndex = 0
	rp.ByteOffset = 0
	rp.LastCommitHash = ""
	for k := range rp.ProcessingFlags {
		delete(rp.ProcessingFlags, k)
	}
	rp.Dependencies = rp.Dependencies[:0]
}

// resetResourceUsage clears resource usage for reuse.
func (p *CheckpointPool) resetResourceUsage(usage *ResourceUsage) {
	usage.CPUUsagePercent = 0
	usage.MemoryUsageMB = 0
	usage.DiskUsageMB = 0
	usage.NetworkBytesIn = 0
	usage.NetworkBytesOut = 0
	usage.DatabaseQueries = 0
	usage.CacheHitRatio = 0
}

// CheckpointCache provides LRU cache for frequently accessed checkpoints.
type CheckpointCache struct {
	mu       sync.RWMutex
	cache    map[uuid.UUID]*cacheEntry
	maxSize  int
	head     *cacheEntry
	tail     *cacheEntry
	hitCount int64
	getCount int64
}

type cacheEntry struct {
	key        uuid.UUID
	checkpoint *JobCheckpoint
	prev       *cacheEntry
	next       *cacheEntry
	accessTime time.Time
}

// NewCheckpointCache creates a new LRU cache for checkpoints.
func NewCheckpointCache(maxSize int) *CheckpointCache {
	cache := &CheckpointCache{
		cache:   make(map[uuid.UUID]*cacheEntry, maxSize),
		maxSize: maxSize,
	}
	// Initialize doubly linked list
	cache.head = &cacheEntry{}
	cache.tail = &cacheEntry{}
	cache.head.next = cache.tail
	cache.tail.prev = cache.head
	return cache
}

// Get retrieves a checkpoint from the cache.
func (c *CheckpointCache) Get(jobID uuid.UUID) (*JobCheckpoint, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.getCount++
	entry, exists := c.cache[jobID]
	if !exists {
		return nil, false
	}

	c.hitCount++
	// Move to front (most recently used)
	c.moveToFront(entry)
	entry.accessTime = time.Now()
	return entry.checkpoint, true
}

// Put stores a checkpoint in the cache.
func (c *CheckpointCache) Put(jobID uuid.UUID, checkpoint *JobCheckpoint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.cache[jobID]; exists {
		// Update existing entry
		entry.checkpoint = checkpoint
		entry.accessTime = time.Now()
		c.moveToFront(entry)
		return
	}

	// Create new entry
	entry := &cacheEntry{
		key:        jobID,
		checkpoint: checkpoint,
		accessTime: time.Now(),
	}

	c.cache[jobID] = entry
	c.addToFront(entry)

	// Evict if necessary
	if len(c.cache) > c.maxSize {
		c.evictLRU()
	}
}

// Remove removes a checkpoint from the cache.
func (c *CheckpointCache) Remove(jobID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.cache[jobID]; exists {
		delete(c.cache, jobID)
		c.removeFromList(entry)
	}
}

// Clear removes all checkpoints from the cache.
func (c *CheckpointCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[uuid.UUID]*cacheEntry, c.maxSize)
	c.head.next = c.tail
	c.tail.prev = c.head
	c.hitCount = 0
	c.getCount = 0
}

// Stats returns cache statistics.
func (c *CheckpointCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hitRate := float64(0)
	if c.getCount > 0 {
		hitRate = float64(c.hitCount) / float64(c.getCount)
	}

	return CacheStats{
		Size:     len(c.cache),
		MaxSize:  c.maxSize,
		HitCount: c.hitCount,
		GetCount: c.getCount,
		HitRate:  hitRate,
	}
}

// CacheStats contains cache performance statistics.
type CacheStats struct {
	Size     int
	MaxSize  int
	HitCount int64
	GetCount int64
	HitRate  float64
}

// moveToFront moves an entry to the front of the LRU list.
func (c *CheckpointCache) moveToFront(entry *cacheEntry) {
	c.removeFromList(entry)
	c.addToFront(entry)
}

// addToFront adds an entry to the front of the LRU list.
func (c *CheckpointCache) addToFront(entry *cacheEntry) {
	entry.prev = c.head
	entry.next = c.head.next
	c.head.next.prev = entry
	c.head.next = entry
}

// removeFromList removes an entry from the LRU list.
func (c *CheckpointCache) removeFromList(entry *cacheEntry) {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
}

// evictLRU removes the least recently used entry.
func (c *CheckpointCache) evictLRU() {
	lru := c.tail.prev
	if lru != c.head {
		delete(c.cache, lru.key)
		c.removeFromList(lru)
	}
}

// PerformanceOptimizedCheckpointService extends DefaultJobCheckpointService
// with performance optimizations like object pooling and caching.
type PerformanceOptimizedCheckpointService struct {
	*DefaultJobCheckpointService

	pool  *CheckpointPool
	cache *CheckpointCache
}

// NewPerformanceOptimizedCheckpointService creates a performance-optimized checkpoint service.
func NewPerformanceOptimizedCheckpointService(
	repository JobCheckpointRepository,
	strategy CheckpointStrategy,
	cacheSize int,
) *PerformanceOptimizedCheckpointService {
	return &PerformanceOptimizedCheckpointService{
		DefaultJobCheckpointService: NewDefaultJobCheckpointService(repository, strategy),
		pool:                        NewCheckpointPool(),
		cache:                       NewCheckpointCache(cacheSize),
	}
}

// GetLatestCheckpoint retrieves the latest checkpoint with caching.
func (s *PerformanceOptimizedCheckpointService) GetLatestCheckpoint(
	ctx context.Context,
	jobID uuid.UUID,
) (*JobCheckpoint, error) {
	// Check cache first
	if cached, found := s.cache.Get(jobID); found {
		// Return a copy to prevent cache corruption
		return s.copyCheckpoint(cached), nil
	}

	// Cache miss - fetch from repository
	checkpoint, err := s.DefaultJobCheckpointService.GetLatestCheckpoint(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cache.Put(jobID, checkpoint)
	return checkpoint, nil
}

// CreateCheckpoint creates a checkpoint using object pooling for better performance.
func (s *PerformanceOptimizedCheckpointService) CreateCheckpoint(
	ctx context.Context,
	job *entity.IndexingJob,
	progress *JobProgress,
	checkpointType CheckpointType,
) (*JobCheckpoint, error) {
	// Create checkpoint using the parent implementation
	checkpoint, err := s.DefaultJobCheckpointService.CreateCheckpoint(ctx, job, progress, checkpointType)
	if err != nil {
		return nil, err
	}

	// Cache the created checkpoint
	s.cache.Put(checkpoint.JobID, checkpoint)
	return checkpoint, nil
}

// copyCheckpoint creates a deep copy of a checkpoint to prevent cache corruption.
func (s *PerformanceOptimizedCheckpointService) copyCheckpoint(original *JobCheckpoint) *JobCheckpoint {
	// Get a checkpoint from the pool
	copied := s.pool.GetCheckpoint()

	// Copy basic fields
	s.copyBasicFields(copied, original)
	s.copyDataSlices(copied, original)
	s.copyResumePoint(copied, original)
	s.copyMetadata(copied, original)
	s.copyFailureContext(copied, original)

	return copied
}

// copyBasicFields copies the basic fields of a checkpoint.
func (s *PerformanceOptimizedCheckpointService) copyBasicFields(copied, original *JobCheckpoint) {
	copied.ID = original.ID
	copied.JobID = original.JobID
	copied.Stage = original.Stage
	copied.CreatedAt = original.CreatedAt
	copied.UpdatedAt = original.UpdatedAt
	copied.ChecksumHash = original.ChecksumHash
	copied.IsCorrupted = original.IsCorrupted

	// Copy ValidatedAt if it exists
	if original.ValidatedAt != nil {
		validatedAt := *original.ValidatedAt
		copied.ValidatedAt = &validatedAt
	}
}

// copyDataSlices copies the data slices of a checkpoint.
func (s *PerformanceOptimizedCheckpointService) copyDataSlices(copied, original *JobCheckpoint) {
	// Copy CompressedData
	if len(original.CompressedData) > 0 {
		copied.CompressedData = make([]byte, len(original.CompressedData))
		copy(copied.CompressedData, original.CompressedData)
	}

	// Copy ProcessedFiles
	if len(original.ProcessedFiles) > 0 {
		copied.ProcessedFiles = make([]string, len(original.ProcessedFiles))
		copy(copied.ProcessedFiles, original.ProcessedFiles)
	}

	// Copy CompletedChunks
	if len(original.CompletedChunks) > 0 {
		copied.CompletedChunks = make([]ChunkReference, len(original.CompletedChunks))
		copy(copied.CompletedChunks, original.CompletedChunks)
	}
}

// copyResumePoint copies the resume point of a checkpoint.
func (s *PerformanceOptimizedCheckpointService) copyResumePoint(copied, original *JobCheckpoint) {
	if original.ResumePoint == nil {
		return
	}

	copied.ResumePoint = s.pool.GetResumePoint()
	copied.ResumePoint.Stage = original.ResumePoint.Stage
	copied.ResumePoint.CurrentFile = original.ResumePoint.CurrentFile
	copied.ResumePoint.FileIndex = original.ResumePoint.FileIndex
	copied.ResumePoint.ChunkIndex = original.ResumePoint.ChunkIndex
	copied.ResumePoint.ByteOffset = original.ResumePoint.ByteOffset
	copied.ResumePoint.LastCommitHash = original.ResumePoint.LastCommitHash

	// Copy ProcessingFlags map
	for k, v := range original.ResumePoint.ProcessingFlags {
		copied.ResumePoint.ProcessingFlags[k] = v
	}

	// Copy Dependencies slice
	if len(original.ResumePoint.Dependencies) > 0 {
		copied.ResumePoint.Dependencies = make([]string, len(original.ResumePoint.Dependencies))
		copy(copied.ResumePoint.Dependencies, original.ResumePoint.Dependencies)
	}
}

// copyMetadata copies the metadata of a checkpoint.
func (s *PerformanceOptimizedCheckpointService) copyMetadata(copied, original *JobCheckpoint) {
	if original.Metadata == nil {
		return
	}

	copied.Metadata = s.pool.GetMetadata()
	copied.Metadata.Version = original.Metadata.Version
	copied.Metadata.CheckpointType = original.Metadata.CheckpointType
	copied.Metadata.Granularity = original.Metadata.Granularity
	copied.Metadata.WorkerID = original.Metadata.WorkerID
	copied.Metadata.WorkerVersion = original.Metadata.WorkerVersion
	copied.Metadata.CompressionType = original.Metadata.CompressionType
	copied.Metadata.EncryptionEnabled = original.Metadata.EncryptionEnabled
	copied.Metadata.DataSize = original.Metadata.DataSize

	// Copy ValidationChecks map
	for k, v := range original.Metadata.ValidationChecks {
		copied.Metadata.ValidationChecks[k] = v
	}

	// Copy CustomProperties map
	for k, v := range original.Metadata.CustomProperties {
		copied.Metadata.CustomProperties[k] = v
	}
}

// copyFailureContext copies the failure context of a checkpoint.
func (s *PerformanceOptimizedCheckpointService) copyFailureContext(copied, original *JobCheckpoint) {
	if original.FailureContext == nil {
		return
	}

	copied.FailureContext = &FailureContext{
		FailureType:  original.FailureContext.FailureType,
		ErrorMessage: original.FailureContext.ErrorMessage,
		ErrorCode:    original.FailureContext.ErrorCode,
		StackTrace:   original.FailureContext.StackTrace,
		RetryCount:   original.FailureContext.RetryCount,
		IsCritical:   original.FailureContext.IsCritical,
	}

	// Copy LastRetryAt if it exists
	if original.FailureContext.LastRetryAt != nil {
		lastRetryAt := *original.FailureContext.LastRetryAt
		copied.FailureContext.LastRetryAt = &lastRetryAt
	}

	s.copyFailureContextSlices(copied, original)
	s.copyFailureContextMaps(copied, original)
}

// copyFailureContextSlices copies the slice fields of failure context.
func (s *PerformanceOptimizedCheckpointService) copyFailureContextSlices(copied, original *JobCheckpoint) {
	// Copy RecoveryHints slice
	if len(original.FailureContext.RecoveryHints) > 0 {
		copied.FailureContext.RecoveryHints = make([]string, len(original.FailureContext.RecoveryHints))
		copy(copied.FailureContext.RecoveryHints, original.FailureContext.RecoveryHints)
	}

	// Copy AffectedFiles slice
	if len(original.FailureContext.AffectedFiles) > 0 {
		copied.FailureContext.AffectedFiles = make([]string, len(original.FailureContext.AffectedFiles))
		copy(copied.FailureContext.AffectedFiles, original.FailureContext.AffectedFiles)
	}
}

// copyFailureContextMaps copies the map fields of failure context.
func (s *PerformanceOptimizedCheckpointService) copyFailureContextMaps(copied, original *JobCheckpoint) {
	// Copy SystemContext map
	if len(original.FailureContext.SystemContext) > 0 {
		copied.FailureContext.SystemContext = make(map[string]interface{})
		for k, v := range original.FailureContext.SystemContext {
			copied.FailureContext.SystemContext[k] = v
		}
	}
}

// ReleaseCheckpoint returns checkpoint resources to the pool.
func (s *PerformanceOptimizedCheckpointService) ReleaseCheckpoint(checkpoint *JobCheckpoint) {
	if checkpoint == nil {
		return
	}

	// Return nested objects to their respective pools
	if checkpoint.ResumePoint != nil {
		s.pool.PutResumePoint(checkpoint.ResumePoint)
		checkpoint.ResumePoint = nil
	}

	if checkpoint.Metadata != nil {
		s.pool.PutMetadata(checkpoint.Metadata)
		checkpoint.Metadata = nil
	}

	// Return the main checkpoint to the pool
	s.pool.PutCheckpoint(checkpoint)
}

// GetCacheStats returns cache performance statistics.
func (s *PerformanceOptimizedCheckpointService) GetCacheStats() CacheStats {
	return s.cache.Stats()
}

// ClearCache clears the checkpoint cache.
func (s *PerformanceOptimizedCheckpointService) ClearCache() {
	s.cache.Clear()
}
