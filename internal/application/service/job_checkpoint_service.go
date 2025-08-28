package service

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/messaging"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JobCheckpoint represents a processing checkpoint for a job.
type JobCheckpoint struct {
	ID              uuid.UUID
	JobID           uuid.UUID
	Stage           ProcessingStage
	ResumePoint     *ResumePoint
	Metadata        *CheckpointMetadata
	ProcessedFiles  []string
	CompletedChunks []ChunkReference
	FailureContext  *FailureContext
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ValidatedAt     *time.Time
	ChecksumHash    string
	CompressedData  []byte
	IsCorrupted     bool
}

// ProcessingStage defines the stages of job processing.
type ProcessingStage string

const (
	StageInitialization ProcessingStage = "initialization"
	StageGitClone       ProcessingStage = "git_clone"
	StageFileDiscovery  ProcessingStage = "file_discovery"
	StageFileProcessing ProcessingStage = "file_processing"
	StageChunking       ProcessingStage = "chunking"
	StageEmbedding      ProcessingStage = "embedding"
	StageFinalization   ProcessingStage = "finalization"
)

// ResumePoint contains information about where to resume processing.
type ResumePoint struct {
	Stage           ProcessingStage
	CurrentFile     string
	FileIndex       int
	ChunkIndex      int
	ByteOffset      int64
	LastCommitHash  string
	ProcessingFlags map[string]interface{}
	Dependencies    []string
}

// CheckpointMetadata contains metadata about the checkpoint.
type CheckpointMetadata struct {
	Version           string
	CheckpointType    CheckpointType
	Granularity       CheckpointGranularity
	WorkerID          string
	WorkerVersion     string
	CompressionType   string
	EncryptionEnabled bool
	DataSize          int64
	ValidationChecks  map[string]bool
	CustomProperties  map[string]interface{}
}

// CheckpointType defines the type of checkpoint.
type CheckpointType string

const (
	CheckpointTypeAutomatic CheckpointType = "automatic"
	CheckpointTypeManual    CheckpointType = "manual"
	CheckpointTypeFailure   CheckpointType = "failure"
	CheckpointTypeRetry     CheckpointType = "retry"
)

// CheckpointGranularity defines the granularity of checkpoints.
type CheckpointGranularity string

const (
	GranularityJob   CheckpointGranularity = "job"
	GranularityStage CheckpointGranularity = "stage"
	GranularityFile  CheckpointGranularity = "file"
	GranularityChunk CheckpointGranularity = "chunk"
	GranularityBatch CheckpointGranularity = "batch"
)

// ChunkReference represents a reference to a processed chunk.
type ChunkReference struct {
	ID       uuid.UUID
	FileID   uuid.UUID
	Index    int
	Checksum string
	Size     int64
}

// FailureContext contains information about failures that occurred.
type FailureContext struct {
	FailureType   messaging.FailureType
	ErrorMessage  string
	ErrorCode     string
	StackTrace    string
	RecoveryHints []string
	RetryCount    int
	LastRetryAt   *time.Time
	IsCritical    bool
	AffectedFiles []string
	SystemContext map[string]interface{}
}

// CheckpointTransaction defines transaction interface for checkpoint operations.
type CheckpointTransaction interface {
	CreateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error
	UpdateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error
	Commit() error
	Rollback() error
}

// JobCheckpointRepository defines the interface for checkpoint persistence.
type JobCheckpointRepository interface {
	CreateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error
	GetLatestCheckpoint(ctx context.Context, jobID uuid.UUID) (*JobCheckpoint, error)
	GetCheckpointsByJob(ctx context.Context, jobID uuid.UUID) ([]*JobCheckpoint, error)
	UpdateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error
	DeleteCheckpointsByJob(ctx context.Context, jobID uuid.UUID) error
	GetOrphanedCheckpoints(ctx context.Context, cutoffTime time.Time) ([]*JobCheckpoint, error)
	BeginTransaction(ctx context.Context) (CheckpointTransaction, error)
}

// CheckpointStrategy defines how checkpoints are created and managed.
type CheckpointStrategy interface {
	ShouldCreateCheckpoint(ctx context.Context, progress *JobProgress, lastCheckpoint *JobCheckpoint) bool
	CreateCheckpointData(ctx context.Context, job *entity.IndexingJob, progress *JobProgress) (*JobCheckpoint, error)
	ValidateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error
	CompressCheckpointData(ctx context.Context, checkpoint *JobCheckpoint) error
	DecompressCheckpointData(ctx context.Context, checkpoint *JobCheckpoint) error
}

// JobCheckpointService provides checkpoint management functionality.
type JobCheckpointService interface {
	CreateCheckpoint(
		ctx context.Context,
		job *entity.IndexingJob,
		progress *JobProgress,
		checkpointType CheckpointType,
	) (*JobCheckpoint, error)
	GetLatestCheckpoint(ctx context.Context, jobID uuid.UUID) (*JobCheckpoint, error)
	ValidateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error
	CleanupCompletedJobCheckpoints(ctx context.Context, jobID uuid.UUID) error
	CleanupOrphanedCheckpoints(ctx context.Context, cutoffTime time.Time) (int, error)
	GetCheckpointHistory(ctx context.Context, jobID uuid.UUID) ([]*JobCheckpoint, error)
}

// JobProgress tracks detailed job processing progress (enhanced version).
type JobProgress struct {
	FilesProcessed      int
	ChunksGenerated     int
	EmbeddingsCreated   int
	BytesProcessed      int64
	CurrentFile         string
	Stage               ProcessingStage
	ProcessedFiles      []string
	CompletedChunks     []ChunkReference
	ErrorCount          int
	LastError           *FailureContext
	StartTime           time.Time
	StageStartTime      time.Time
	EstimatedCompletion *time.Time
	ThroughputMetrics   map[string]float64
	ResourceUsage       *ResourceUsage
}

// ResourceUsage tracks resource consumption during processing.
type ResourceUsage struct {
	CPUUsagePercent float64
	MemoryUsageMB   int64
	DiskUsageMB     int64
	NetworkBytesIn  int64
	NetworkBytesOut int64
	DatabaseQueries int
	CacheHitRatio   float64
}

// DefaultJobCheckpointService implements JobCheckpointService.
type DefaultJobCheckpointService struct {
	repository JobCheckpointRepository
	strategy   CheckpointStrategy
}

// NewDefaultJobCheckpointService creates a new DefaultJobCheckpointService.
func NewDefaultJobCheckpointService(
	repository JobCheckpointRepository,
	strategy CheckpointStrategy,
) *DefaultJobCheckpointService {
	return &DefaultJobCheckpointService{
		repository: repository,
		strategy:   strategy,
	}
}

// CreateCheckpoint creates a new checkpoint for a job.
func (s *DefaultJobCheckpointService) CreateCheckpoint(
	ctx context.Context,
	job *entity.IndexingJob,
	progress *JobProgress,
	checkpointType CheckpointType,
) (*JobCheckpoint, error) {
	if s.repository == nil || s.strategy == nil {
		return nil, errors.New("repository or strategy not initialized")
	}

	// Create checkpoint data using strategy
	checkpoint, err := s.strategy.CreateCheckpointData(ctx, job, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to create checkpoint data: %w", err)
	}

	// Set checkpoint type and generate ID
	checkpoint.ID = uuid.New()
	if checkpoint.Metadata == nil {
		checkpoint.Metadata = &CheckpointMetadata{}
	}
	checkpoint.Metadata.CheckpointType = checkpointType
	checkpoint.CreatedAt = time.Now()
	checkpoint.UpdatedAt = checkpoint.CreatedAt

	// Calculate checksum
	checkpoint.ChecksumHash = s.calculateChecksum(checkpoint)

	// Compress checkpoint data if needed
	if err := s.strategy.CompressCheckpointData(ctx, checkpoint); err != nil {
		return nil, fmt.Errorf("failed to compress checkpoint data: %w", err)
	}

	// Persist to repository
	if err := s.repository.CreateCheckpoint(ctx, checkpoint); err != nil {
		return nil, fmt.Errorf("failed to persist checkpoint: %w", err)
	}

	return checkpoint, nil
}

// GetLatestCheckpoint retrieves the latest checkpoint for a job.
func (s *DefaultJobCheckpointService) GetLatestCheckpoint(
	ctx context.Context,
	jobID uuid.UUID,
) (*JobCheckpoint, error) {
	if s.repository == nil {
		return nil, errors.New("repository not initialized")
	}

	checkpoint, err := s.repository.GetLatestCheckpoint(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest checkpoint: %w", err)
	}

	// Decompress if needed
	if s.strategy != nil && len(checkpoint.CompressedData) > 0 {
		if err := s.strategy.DecompressCheckpointData(ctx, checkpoint); err != nil {
			return nil, fmt.Errorf("failed to decompress checkpoint data: %w", err)
		}
	}

	return checkpoint, nil
}

// ValidateCheckpoint validates a checkpoint for integrity and corruption.
func (s *DefaultJobCheckpointService) ValidateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error {
	if s.strategy == nil {
		return errors.New("strategy not initialized")
	}

	// Basic validation
	if checkpoint == nil {
		return errors.New("checkpoint is nil")
	}

	if checkpoint.JobID == uuid.Nil {
		return errors.New("checkpoint job ID is empty")
	}

	// Check for corruption
	if checkpoint.IsCorrupted {
		return errors.New("checkpoint is marked as corrupted")
	}

	// Validate checksum
	expectedChecksum := s.calculateChecksum(checkpoint)
	if checkpoint.ChecksumHash != expectedChecksum {
		checkpoint.IsCorrupted = true
		return errors.New("checkpoint checksum mismatch")
	}

	// Use strategy for detailed validation
	if err := s.strategy.ValidateCheckpoint(ctx, checkpoint); err != nil {
		return fmt.Errorf("strategy validation failed: %w", err)
	}

	// Update validation timestamp
	now := time.Now()
	checkpoint.ValidatedAt = &now

	return nil
}

// CleanupCompletedJobCheckpoints removes checkpoints for completed jobs.
func (s *DefaultJobCheckpointService) CleanupCompletedJobCheckpoints(ctx context.Context, jobID uuid.UUID) error {
	if s.repository == nil {
		return errors.New("repository not initialized")
	}

	return s.repository.DeleteCheckpointsByJob(ctx, jobID)
}

// CleanupOrphanedCheckpoints removes checkpoints for jobs that no longer exist.
func (s *DefaultJobCheckpointService) CleanupOrphanedCheckpoints(
	ctx context.Context,
	cutoffTime time.Time,
) (int, error) {
	if s.repository == nil {
		return 0, errors.New("repository not initialized")
	}

	orphanedCheckpoints, err := s.repository.GetOrphanedCheckpoints(ctx, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to get orphaned checkpoints: %w", err)
	}

	cleanedCount := 0
	for _, checkpoint := range orphanedCheckpoints {
		if err := s.repository.DeleteCheckpointsByJob(ctx, checkpoint.JobID); err != nil {
			// Continue cleaning other checkpoints even if one fails
			continue
		}
		cleanedCount++
	}

	return cleanedCount, nil
}

// GetCheckpointHistory retrieves all checkpoints for a job.
func (s *DefaultJobCheckpointService) GetCheckpointHistory(
	ctx context.Context,
	jobID uuid.UUID,
) ([]*JobCheckpoint, error) {
	if s.repository == nil {
		return nil, errors.New("repository not initialized")
	}

	checkpoints, err := s.repository.GetCheckpointsByJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint history: %w", err)
	}

	// Decompress each checkpoint if needed
	if s.strategy != nil {
		for _, checkpoint := range checkpoints {
			if len(checkpoint.CompressedData) > 0 {
				if err := s.strategy.DecompressCheckpointData(ctx, checkpoint); err != nil {
					// Mark as corrupted but continue
					checkpoint.IsCorrupted = true
				}
			}
		}
	}

	return checkpoints, nil
}

// calculateChecksum generates a checksum for checkpoint integrity verification.
func (s *DefaultJobCheckpointService) calculateChecksum(checkpoint *JobCheckpoint) string {
	h := sha256.New()
	h.Write([]byte(checkpoint.JobID.String()))
	h.Write([]byte(string(checkpoint.Stage)))
	if checkpoint.ResumePoint != nil {
		h.Write([]byte(checkpoint.ResumePoint.CurrentFile))
	}
	return hex.EncodeToString(h.Sum(nil))
}
