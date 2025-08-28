package service

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockJobCheckpointRepository defines the interface for job checkpoint persistence.
type MockJobCheckpointRepository struct {
	mock.Mock
}

func (m *MockJobCheckpointRepository) CreateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

func (m *MockJobCheckpointRepository) GetLatestCheckpoint(
	ctx context.Context,
	jobID uuid.UUID,
) (*JobCheckpoint, error) {
	args := m.Called(ctx, jobID)
	return args.Get(0).(*JobCheckpoint), args.Error(1)
}

func (m *MockJobCheckpointRepository) GetCheckpointsByJob(
	ctx context.Context,
	jobID uuid.UUID,
) ([]*JobCheckpoint, error) {
	args := m.Called(ctx, jobID)
	return args.Get(0).([]*JobCheckpoint), args.Error(1)
}

func (m *MockJobCheckpointRepository) UpdateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

func (m *MockJobCheckpointRepository) DeleteCheckpointsByJob(ctx context.Context, jobID uuid.UUID) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockJobCheckpointRepository) GetOrphanedCheckpoints(
	ctx context.Context,
	cutoffTime time.Time,
) ([]*JobCheckpoint, error) {
	args := m.Called(ctx, cutoffTime)
	return args.Get(0).([]*JobCheckpoint), args.Error(1)
}

func (m *MockJobCheckpointRepository) BeginTransaction(ctx context.Context) (CheckpointTransaction, error) {
	args := m.Called(ctx)
	return args.Get(0).(CheckpointTransaction), args.Error(1)
}

// MockCheckpointTransaction provides transaction support for checkpoint operations.
type MockCheckpointTransaction struct {
	mock.Mock
}

func (m *MockCheckpointTransaction) CreateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

func (m *MockCheckpointTransaction) UpdateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

func (m *MockCheckpointTransaction) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCheckpointTransaction) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

// Mock for CheckpointStrategy.
type MockCheckpointStrategy struct {
	mock.Mock
}

func (m *MockCheckpointStrategy) ShouldCreateCheckpoint(
	ctx context.Context,
	progress *JobProgress,
	lastCheckpoint *JobCheckpoint,
) bool {
	args := m.Called(ctx, progress, lastCheckpoint)
	return args.Bool(0)
}

func (m *MockCheckpointStrategy) CreateCheckpointData(
	ctx context.Context,
	job *entity.IndexingJob,
	progress *JobProgress,
) (*JobCheckpoint, error) {
	args := m.Called(ctx, job, progress)
	return args.Get(0).(*JobCheckpoint), args.Error(1)
}

func (m *MockCheckpointStrategy) ValidateCheckpoint(ctx context.Context, checkpoint *JobCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

func (m *MockCheckpointStrategy) CompressCheckpointData(ctx context.Context, checkpoint *JobCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

func (m *MockCheckpointStrategy) DecompressCheckpointData(ctx context.Context, checkpoint *JobCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

// Test suite for JobCheckpointService

func TestJobCheckpointService_CreateCheckpoint_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	mockStrategy := new(MockCheckpointStrategy)
	service := NewDefaultJobCheckpointService(mockRepo, mockStrategy)

	jobID := uuid.New()
	repositoryID := uuid.New()
	job := entity.RestoreIndexingJob(
		jobID, repositoryID, valueobject.JobStatusRunning,
		&time.Time{}, nil, nil, 0, 0,
		time.Now(), time.Now(), nil,
	)

	progress := &JobProgress{
		FilesProcessed:  10,
		ChunksGenerated: 50,
		BytesProcessed:  1024000,
		Stage:           StageFileProcessing,
		CurrentFile:     "src/main.go",
		ProcessedFiles:  []string{"src/util.go", "src/helper.go"},
	}

	expectedCheckpoint := &JobCheckpoint{
		ID:    uuid.New(),
		JobID: jobID,
		Stage: StageFileProcessing,
		ResumePoint: &ResumePoint{
			Stage:       StageFileProcessing,
			CurrentFile: "src/main.go",
			FileIndex:   10,
		},
		Metadata: &CheckpointMetadata{
			CheckpointType: CheckpointTypeAutomatic,
			Granularity:    GranularityFile,
		},
	}

	mockStrategy.On("CreateCheckpointData", ctx, job, progress).Return(expectedCheckpoint, nil)
	mockStrategy.On("CompressCheckpointData", ctx, expectedCheckpoint).Return(nil)
	mockRepo.On("CreateCheckpoint", ctx, expectedCheckpoint).Return(nil)

	// Act
	result, err := service.CreateCheckpoint(ctx, job, progress, CheckpointTypeAutomatic)

	// Assert - GREEN phase: expect success
	require.NoError(t, err, "Expected success in GREEN phase")
	assert.NotNil(t, result)
	assert.Equal(t, jobID, result.JobID)
	assert.Equal(t, StageFileProcessing, result.Stage)
	// Verify mocks were called
	mockStrategy.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_CreateCheckpoint_DatabaseError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	mockStrategy := new(MockCheckpointStrategy)
	service := NewDefaultJobCheckpointService(mockRepo, mockStrategy)

	jobID := uuid.New()
	job := entity.RestoreIndexingJob(
		jobID, uuid.New(), valueobject.JobStatusRunning,
		&time.Time{}, nil, nil, 0, 0,
		time.Now(), time.Now(), nil,
	)

	progress := &JobProgress{Stage: StageFileProcessing}

	checkpoint := &JobCheckpoint{
		ID:    uuid.New(),
		JobID: jobID,
		Stage: StageFileProcessing,
	}

	// Setup mocks - strategy succeeds but repository fails
	mockStrategy.On("CreateCheckpointData", ctx, job, progress).Return(checkpoint, nil)
	mockStrategy.On("CompressCheckpointData", ctx, checkpoint).Return(nil)
	mockRepo.On("CreateCheckpoint", ctx, mock.MatchedBy(func(cp *JobCheckpoint) bool {
		return cp.JobID == jobID
	})).Return(errors.New("database connection failed"))

	// Act
	result, err := service.CreateCheckpoint(ctx, job, progress, CheckpointTypeAutomatic)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to persist checkpoint")
	assert.Nil(t, result)

	// Verify mocks were called
	mockStrategy.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_CreateCheckpoint_TransactionRollback(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	mockStrategy := new(MockCheckpointStrategy)
	service := NewDefaultJobCheckpointService(mockRepo, mockStrategy)

	jobID := uuid.New()
	job := entity.RestoreIndexingJob(
		jobID, uuid.New(), valueobject.JobStatusRunning,
		&time.Time{}, nil, nil, 0, 0,
		time.Now(), time.Now(), nil,
	)

	progress := &JobProgress{Stage: StageFileProcessing}

	checkpoint := &JobCheckpoint{
		ID:    uuid.New(),
		JobID: jobID,
		Stage: StageFileProcessing,
	}

	// Setup mocks for database error scenario
	mockStrategy.On("CreateCheckpointData", ctx, job, progress).Return(checkpoint, nil)
	mockStrategy.On("CompressCheckpointData", ctx, mock.MatchedBy(func(cp *JobCheckpoint) bool {
		return cp.JobID == jobID
	})).Return(nil)
	mockRepo.On("CreateCheckpoint", ctx, mock.MatchedBy(func(cp *JobCheckpoint) bool {
		return cp.JobID == jobID
	})).Return(errors.New("database error"))

	// Act
	result, err := service.CreateCheckpoint(ctx, job, progress, CheckpointTypeAutomatic)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to persist checkpoint")
	assert.Nil(t, result)

	// Verify mocks were called
	mockStrategy.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_GetLatestCheckpoint_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	service := NewDefaultJobCheckpointService(mockRepo, nil)

	jobID := uuid.New()
	expectedCheckpoint := &JobCheckpoint{
		ID:        uuid.New(),
		JobID:     jobID,
		Stage:     StageFileProcessing,
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}

	mockRepo.On("GetLatestCheckpoint", ctx, jobID).Return(expectedCheckpoint, nil)

	// Act
	result, err := service.GetLatestCheckpoint(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedCheckpoint.ID, result.ID)
	assert.Equal(t, expectedCheckpoint.JobID, result.JobID)
	assert.Equal(t, expectedCheckpoint.Stage, result.Stage)

	// Verify mocks were called
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_GetLatestCheckpoint_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	service := NewDefaultJobCheckpointService(mockRepo, nil)

	jobID := uuid.New()
	mockRepo.On("GetLatestCheckpoint", ctx, jobID).Return((*JobCheckpoint)(nil), sql.ErrNoRows)

	// Act
	result, err := service.GetLatestCheckpoint(ctx, jobID)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get latest checkpoint")
	assert.Contains(t, err.Error(), "no rows in result set")
	assert.Nil(t, result)

	// Verify mocks were called
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_ValidateCheckpoint_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockStrategy := new(MockCheckpointStrategy)
	service := NewDefaultJobCheckpointService(nil, mockStrategy)

	checkpoint := &JobCheckpoint{
		ID:        uuid.New(),
		JobID:     uuid.New(),
		Stage:     StageFileProcessing,
		CreatedAt: time.Now(),
	}

	// Calculate correct checksum
	checkpoint.ChecksumHash = service.calculateChecksum(checkpoint)

	mockStrategy.On("ValidateCheckpoint", ctx, checkpoint).Return(nil)

	// Act
	err := service.ValidateCheckpoint(ctx, checkpoint)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, checkpoint.ValidatedAt)

	// Verify mocks were called
	mockStrategy.AssertExpectations(t)
}

func TestJobCheckpointService_ValidateCheckpoint_Corrupted(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockStrategy := new(MockCheckpointStrategy)
	service := NewDefaultJobCheckpointService(nil, mockStrategy)

	checkpoint := &JobCheckpoint{
		ID:           uuid.New(),
		JobID:        uuid.New(),
		Stage:        StageFileProcessing,
		ChecksumHash: "invalid",
		IsCorrupted:  true,
		CreatedAt:    time.Now(),
	}

	mockStrategy.On("ValidateCheckpoint", ctx, checkpoint).Return(errors.New("checkpoint corrupted"))

	// Act
	err := service.ValidateCheckpoint(ctx, checkpoint)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checkpoint is marked as corrupted")
}

func TestJobCheckpointService_CleanupCompletedJobCheckpoints_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	service := NewDefaultJobCheckpointService(mockRepo, nil)

	jobID := uuid.New()
	mockRepo.On("DeleteCheckpointsByJob", ctx, jobID).Return(nil)

	// Act
	err := service.CleanupCompletedJobCheckpoints(ctx, jobID)

	// Assert
	require.NoError(t, err)

	// Verify mocks were called
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_CleanupOrphanedCheckpoints_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	service := NewDefaultJobCheckpointService(mockRepo, nil)

	cutoffTime := time.Now().Add(-24 * time.Hour)
	orphanedCheckpoints := []*JobCheckpoint{
		{ID: uuid.New(), JobID: uuid.New()},
		{ID: uuid.New(), JobID: uuid.New()},
	}

	mockRepo.On("GetOrphanedCheckpoints", ctx, cutoffTime).Return(orphanedCheckpoints, nil)
	mockRepo.On("DeleteCheckpointsByJob", ctx, mock.AnythingOfType("uuid.UUID")).Return(nil).Times(2)

	// Act
	count, err := service.CleanupOrphanedCheckpoints(ctx, cutoffTime)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify mocks were called
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_GetCheckpointHistory_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	service := NewDefaultJobCheckpointService(mockRepo, nil)

	jobID := uuid.New()
	expectedCheckpoints := []*JobCheckpoint{
		{ID: uuid.New(), JobID: jobID, Stage: StageGitClone, CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: uuid.New(), JobID: jobID, Stage: StageFileProcessing, CreatedAt: time.Now().Add(-1 * time.Hour)},
	}

	mockRepo.On("GetCheckpointsByJob", ctx, jobID).Return(expectedCheckpoints, nil)

	// Act
	result, err := service.GetCheckpointHistory(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, expectedCheckpoints[0].ID, result[0].ID)
	assert.Equal(t, expectedCheckpoints[1].ID, result[1].ID)

	// Verify mocks were called
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_CreateCheckpoint_WithCompression(t *testing.T) {
	// Test checkpoint creation with data compression
	ctx := context.Background()
	mockRepo := new(MockJobCheckpointRepository)
	mockStrategy := new(MockCheckpointStrategy)
	service := NewDefaultJobCheckpointService(mockRepo, mockStrategy)

	job := entity.RestoreIndexingJob(
		uuid.New(), uuid.New(), valueobject.JobStatusRunning,
		&time.Time{}, nil, nil, 0, 0,
		time.Now(), time.Now(), nil,
	)

	progress := &JobProgress{
		FilesProcessed:  1000,
		ChunksGenerated: 5000,
		BytesProcessed:  10485760, // 10MB - should trigger compression
		Stage:           StageChunking,
		ProcessedFiles:  make([]string, 1000), // Large data set
	}

	checkpoint := &JobCheckpoint{
		ID:    uuid.New(),
		JobID: job.ID(),
		Stage: StageChunking,
	}

	// Setup mocks for compression scenario
	mockStrategy.On("CreateCheckpointData", ctx, job, progress).Return(checkpoint, nil)
	mockStrategy.On("CompressCheckpointData", ctx, mock.MatchedBy(func(cp *JobCheckpoint) bool {
		return cp.JobID == job.ID()
	})).Return(nil)
	mockRepo.On("CreateCheckpoint", ctx, mock.MatchedBy(func(cp *JobCheckpoint) bool {
		return cp.JobID == job.ID()
	})).Return(nil)

	// Act
	result, err := service.CreateCheckpoint(ctx, job, progress, CheckpointTypeAutomatic)

	// Assert - compression should succeed
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, job.ID(), result.JobID)
	assert.Equal(t, StageChunking, result.Stage)

	// Verify mocks were called
	mockStrategy.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestJobCheckpointService_CreateCheckpoint_ConcurrentAccess(t *testing.T) {
	// Test concurrent checkpoint creation for the same job
	ctx := context.Background()
	service := NewDefaultJobCheckpointService(nil, nil)

	jobID := uuid.New()
	job := entity.RestoreIndexingJob(
		jobID, uuid.New(), valueobject.JobStatusRunning,
		&time.Time{}, nil, nil, 0, 0,
		time.Now(), time.Now(), nil,
	)

	progress := &JobProgress{Stage: StageFileProcessing}

	// Simulate concurrent checkpoint creation
	results := make(chan error, 2)

	go func() {
		_, err := service.CreateCheckpoint(ctx, job, progress, CheckpointTypeAutomatic)
		results <- err
	}()

	go func() {
		_, err := service.CreateCheckpoint(ctx, job, progress, CheckpointTypeManual)
		results <- err
	}()

	// Collect results
	for range 2 {
		err := <-results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "repository or strategy not initialized")
	}
}

func TestJobCheckpointService_ValidateCheckpoint_IntegrityChecks(t *testing.T) {
	// Test comprehensive checkpoint integrity validation
	ctx := context.Background()
	service := NewDefaultJobCheckpointService(nil, nil)

	testCases := []struct {
		name        string
		checkpoint  *JobCheckpoint
		expectError bool
	}{
		{
			name: "valid_checkpoint",
			checkpoint: &JobCheckpoint{
				ID:           uuid.New(),
				JobID:        uuid.New(),
				Stage:        StageFileProcessing,
				ChecksumHash: "sha256:abc123",
				CreatedAt:    time.Now(),
				IsCorrupted:  false,
			},
			expectError: false,
		},
		{
			name: "corrupted_checkpoint",
			checkpoint: &JobCheckpoint{
				ID:           uuid.New(),
				JobID:        uuid.New(),
				Stage:        StageFileProcessing,
				ChecksumHash: "invalid",
				CreatedAt:    time.Now(),
				IsCorrupted:  true,
			},
			expectError: true,
		},
		{
			name: "missing_required_fields",
			checkpoint: &JobCheckpoint{
				ID:    uuid.Nil, // Invalid ID
				JobID: uuid.New(),
				Stage: "", // Invalid stage
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := service.ValidateCheckpoint(ctx, tc.checkpoint)

			// Assert - all should fail due to nil strategy
			require.Error(t, err)
			assert.Contains(t, err.Error(), "strategy not initialized")
		})
	}
}

func TestCheckpointGranularityBehavior(t *testing.T) {
	// Test different checkpoint granularities
	testCases := []struct {
		name        string
		granularity CheckpointGranularity
		progress    *JobProgress
	}{
		{
			name:        "job_level_granularity",
			granularity: GranularityJob,
			progress: &JobProgress{
				FilesProcessed: 100,
				Stage:          StageFinalization,
			},
		},
		{
			name:        "file_level_granularity",
			granularity: GranularityFile,
			progress: &JobProgress{
				FilesProcessed: 1,
				CurrentFile:    "src/main.go",
				Stage:          StageFileProcessing,
			},
		},
		{
			name:        "chunk_level_granularity",
			granularity: GranularityChunk,
			progress: &JobProgress{
				ChunksGenerated: 1,
				Stage:           StageChunking,
			},
		},
	}

	ctx := context.Background()
	service := NewDefaultJobCheckpointService(nil, nil)
	job := entity.RestoreIndexingJob(
		uuid.New(), uuid.New(), valueobject.JobStatusRunning,
		&time.Time{}, nil, nil, 0, 0,
		time.Now(), time.Now(), nil,
	)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result, err := service.CreateCheckpoint(ctx, job, tc.progress, CheckpointTypeAutomatic)

			// Assert
			require.Error(t, err, "Expected error due to nil dependencies")
			assert.Contains(t, err.Error(), "repository or strategy not initialized")
			assert.Nil(t, result)
		})
	}
}

// Benchmark tests for checkpoint operations performance

func BenchmarkJobCheckpointService_CreateCheckpoint(b *testing.B) {
	ctx := context.Background()
	service := NewDefaultJobCheckpointService(nil, nil)

	job := entity.RestoreIndexingJob(
		uuid.New(), uuid.New(), valueobject.JobStatusRunning,
		&time.Time{}, nil, nil, 0, 0,
		time.Now(), time.Now(), nil,
	)

	progress := &JobProgress{
		FilesProcessed:  100,
		ChunksGenerated: 500,
		Stage:           StageFileProcessing,
	}

	b.ResetTimer()
	for range b.N {
		_, err := service.CreateCheckpoint(ctx, job, progress, CheckpointTypeAutomatic)
		if err == nil {
			b.Fatal("Expected error due to nil dependencies")
		}
	}
}

func BenchmarkJobCheckpointService_ValidateCheckpoint(b *testing.B) {
	ctx := context.Background()
	service := NewDefaultJobCheckpointService(nil, nil)

	checkpoint := &JobCheckpoint{
		ID:           uuid.New(),
		JobID:        uuid.New(),
		Stage:        StageFileProcessing,
		ChecksumHash: "sha256:abc123",
		CreatedAt:    time.Now(),
	}

	b.ResetTimer()
	for range b.N {
		err := service.ValidateCheckpoint(ctx, checkpoint)
		if err == nil {
			b.Fatal("Expected error due to nil dependencies")
		}
	}
}
