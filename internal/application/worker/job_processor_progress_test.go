package worker

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockBatchProgressRepository mocks the batch progress repository interface.
type MockBatchProgressRepository struct {
	mock.Mock
}

func (m *MockBatchProgressRepository) Save(ctx context.Context, progress *entity.BatchJobProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockBatchProgressRepository) GetByJobID(
	ctx context.Context,
	jobID uuid.UUID,
) ([]*entity.BatchJobProgress, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.BatchJobProgress), args.Error(1)
}

func (m *MockBatchProgressRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*entity.BatchJobProgress, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.BatchJobProgress), args.Error(1)
}

func (m *MockBatchProgressRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status string,
) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockBatchProgressRepository) GetNextRetryBatch(
	ctx context.Context,
) (*entity.BatchJobProgress, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.BatchJobProgress), args.Error(1)
}

func (m *MockBatchProgressRepository) MarkCompleted(
	ctx context.Context,
	id uuid.UUID,
	chunksProcessed int,
) error {
	args := m.Called(ctx, id, chunksProcessed)
	return args.Error(0)
}

func (m *MockBatchProgressRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockBatchProgressRepository) GetPendingGeminiBatches(
	ctx context.Context,
) ([]*entity.BatchJobProgress, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.BatchJobProgress), args.Error(1)
}

func (m *MockBatchProgressRepository) GetPendingSubmissionBatch(
	ctx context.Context,
) (*entity.BatchJobProgress, error) {
	args := m.Called(ctx)

	// Handle nil return
	if args.Get(0) == nil {
		// Check if args[1] is a function or error
		if args.Get(1) == nil {
			return nil, nil
		}
		// Try to get as function first
		if fn, ok := args.Get(1).(func(context.Context) error); ok {
			return nil, fn(ctx)
		}
		return nil, args.Error(1)
	}

	// Handle function return
	if fn, ok := args.Get(0).(func(context.Context) *entity.BatchJobProgress); ok {
		result := fn(ctx)
		// Check second return value
		if args.Get(1) == nil {
			return result, nil
		}
		if errFn, ok := args.Get(1).(func(context.Context) error); ok {
			return result, errFn(ctx)
		}
		return result, args.Error(1)
	}

	// Handle direct value return
	return args.Get(0).(*entity.BatchJobProgress), args.Error(1)
}

func (m *MockBatchProgressRepository) GetPendingSubmissionCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

// TestSaveBatchProgress_Success tests successfully saving batch progress with completed status.
func TestSaveBatchProgress_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()
	batchNumber := 1
	totalBatches := 5
	chunksProcessed := 100
	status := "completed"

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Expect Save to be called with a BatchJobProgress entity
	mockRepo.On("Save", ctx, mock.MatchedBy(func(progress *entity.BatchJobProgress) bool {
		return progress.RepositoryID() != nil && *progress.RepositoryID() == repositoryID &&
			progress.IndexingJobID() == jobID &&
			progress.BatchNumber() == batchNumber &&
			progress.TotalBatches() == totalBatches &&
			progress.ChunksProcessed() == chunksProcessed &&
			progress.Status() == status
	})).Return(nil)

	// Act
	err := processor.saveBatchProgress(ctx, repositoryID, jobID, batchNumber, totalBatches, chunksProcessed, status)

	// Assert
	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestSaveBatchProgress_PendingStatus tests saving batch progress with pending status.
func TestSaveBatchProgress_PendingStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()
	batchNumber := 3
	totalBatches := 10
	chunksProcessed := 0
	status := "pending"

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Expect Save to be called with correct pending status
	mockRepo.On("Save", ctx, mock.MatchedBy(func(progress *entity.BatchJobProgress) bool {
		// Verify all fields are saved correctly for pending status
		return progress.RepositoryID() != nil && *progress.RepositoryID() == repositoryID &&
			progress.IndexingJobID() == jobID &&
			progress.BatchNumber() == batchNumber &&
			progress.TotalBatches() == totalBatches &&
			progress.ChunksProcessed() == chunksProcessed &&
			progress.Status() == status &&
			progress.RetryCount() == 0 &&
			progress.ErrorMessage() == nil
	})).Return(nil)

	// Act
	err := processor.saveBatchProgress(ctx, repositoryID, jobID, batchNumber, totalBatches, chunksProcessed, status)

	// Assert
	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestSaveBatchProgress_RepositoryError tests error propagation from repository.
func TestSaveBatchProgress_RepositoryError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()
	batchNumber := 2
	totalBatches := 5
	chunksProcessed := 50
	status := "completed"
	expectedErr := errors.New("database connection failed")

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Repository returns error
	mockRepo.On("Save", ctx, mock.AnythingOfType("*entity.BatchJobProgress")).Return(expectedErr)

	// Act
	err := processor.saveBatchProgress(ctx, repositoryID, jobID, batchNumber, totalBatches, chunksProcessed, status)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database connection failed")
	mockRepo.AssertExpectations(t)
}

// TestResumeFromLastBatch_NoPreviousProgress tests resuming when no batches exist.
func TestResumeFromLastBatch_NoPreviousProgress(t *testing.T) {
	// Arrange
	ctx := context.Background()
	jobID := uuid.New()

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Repository returns empty slice (no batches found)
	mockRepo.On("GetByJobID", ctx, jobID).Return([]*entity.BatchJobProgress{}, nil)

	// Act
	lastBatch, err := processor.resumeFromLastBatch(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, lastBatch, "Should return 0 when no previous progress exists")
	mockRepo.AssertExpectations(t)
}

// TestResumeFromLastBatch_SomeCompleted tests resuming from partially completed batches.
func TestResumeFromLastBatch_SomeCompleted(t *testing.T) {
	// Arrange
	ctx := context.Background()
	jobID := uuid.New()

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Create batch progress records:
	// Batch 1: completed
	// Batch 2: completed
	// Batch 3: pending
	// Batch 4: pending
	batches := []*entity.BatchJobProgress{
		createBatchProgress(jobID, 1, 5, 100, "completed"),
		createBatchProgress(jobID, 2, 5, 100, "completed"),
		createBatchProgress(jobID, 3, 5, 0, "pending"),
		createBatchProgress(jobID, 4, 5, 0, "pending"),
	}

	mockRepo.On("GetByJobID", ctx, jobID).Return(batches, nil)

	// Act
	lastBatch, err := processor.resumeFromLastBatch(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, lastBatch, "Should return last completed batch number (2)")
	mockRepo.AssertExpectations(t)
}

// TestResumeFromLastBatch_AllCompleted tests resuming when all batches are completed.
func TestResumeFromLastBatch_AllCompleted(t *testing.T) {
	// Arrange
	ctx := context.Background()
	jobID := uuid.New()

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// All batches completed (1-5)
	batches := []*entity.BatchJobProgress{
		createBatchProgress(jobID, 1, 5, 100, "completed"),
		createBatchProgress(jobID, 2, 5, 100, "completed"),
		createBatchProgress(jobID, 3, 5, 100, "completed"),
		createBatchProgress(jobID, 4, 5, 100, "completed"),
		createBatchProgress(jobID, 5, 5, 100, "completed"),
	}

	mockRepo.On("GetByJobID", ctx, jobID).Return(batches, nil)

	// Act
	lastBatch, err := processor.resumeFromLastBatch(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 5, lastBatch, "Should return last batch number when all completed")
	mockRepo.AssertExpectations(t)
}

// TestResumeFromLastBatch_PartialWithFailed tests resuming with mixed completed/failed batches.
func TestResumeFromLastBatch_PartialWithFailed(t *testing.T) {
	// Arrange
	ctx := context.Background()
	jobID := uuid.New()

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Batches with mixed status:
	// Batch 1: completed
	// Batch 2: failed
	// Batch 3: pending
	batches := []*entity.BatchJobProgress{
		createBatchProgress(jobID, 1, 5, 100, "completed"),
		createBatchProgress(jobID, 2, 5, 50, "failed"),
		createBatchProgress(jobID, 3, 5, 0, "pending"),
	}

	mockRepo.On("GetByJobID", ctx, jobID).Return(batches, nil)

	// Act
	lastBatch, err := processor.resumeFromLastBatch(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, lastBatch, "Should only count completed batches, not failed ones")
	mockRepo.AssertExpectations(t)
}

// TestResumeFromLastBatch_RepositoryError tests error handling when repository fails.
func TestResumeFromLastBatch_RepositoryError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	jobID := uuid.New()
	expectedErr := errors.New("database query failed")

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Repository returns error
	mockRepo.On("GetByJobID", ctx, jobID).Return(nil, expectedErr)

	// Act
	lastBatch, err := processor.resumeFromLastBatch(ctx, jobID)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0, lastBatch, "Should return 0 on error")
	assert.Contains(t, err.Error(), "database query failed")
	mockRepo.AssertExpectations(t)
}

// TestResumeFromLastBatch_OnlyFailedBatches tests resuming when only failed batches exist.
func TestResumeFromLastBatch_OnlyFailedBatches(t *testing.T) {
	// Arrange
	ctx := context.Background()
	jobID := uuid.New()

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// All batches failed
	batches := []*entity.BatchJobProgress{
		createBatchProgress(jobID, 1, 3, 0, "failed"),
		createBatchProgress(jobID, 2, 3, 0, "failed"),
		createBatchProgress(jobID, 3, 3, 0, "failed"),
	}

	mockRepo.On("GetByJobID", ctx, jobID).Return(batches, nil)

	// Act
	lastBatch, err := processor.resumeFromLastBatch(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, lastBatch, "Should return 0 when no completed batches exist")
	mockRepo.AssertExpectations(t)
}

// TestResumeFromLastBatch_ProcessingStatus tests handling of in-progress batches.
func TestResumeFromLastBatch_ProcessingStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	jobID := uuid.New()

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Batches with processing status:
	// Batch 1: completed
	// Batch 2: processing (should not count as completed)
	// Batch 3: pending
	batches := []*entity.BatchJobProgress{
		createBatchProgress(jobID, 1, 5, 100, "completed"),
		createBatchProgress(jobID, 2, 5, 50, "processing"),
		createBatchProgress(jobID, 3, 5, 0, "pending"),
	}

	mockRepo.On("GetByJobID", ctx, jobID).Return(batches, nil)

	// Act
	lastBatch, err := processor.resumeFromLastBatch(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, lastBatch, "Should not count processing batches as completed")
	mockRepo.AssertExpectations(t)
}

// TestResumeFromLastBatch_OutOfOrderBatches tests handling batches not in sequential order.
func TestResumeFromLastBatch_OutOfOrderBatches(t *testing.T) {
	// Arrange
	ctx := context.Background()
	jobID := uuid.New()

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	// Batches returned out of order (repository should order them, but test robustness):
	// Batch 3: pending
	// Batch 1: completed
	// Batch 2: completed
	// Should still find last completed as batch 2
	batches := []*entity.BatchJobProgress{
		createBatchProgress(jobID, 3, 5, 0, "pending"),
		createBatchProgress(jobID, 1, 5, 100, "completed"),
		createBatchProgress(jobID, 2, 5, 100, "completed"),
	}

	mockRepo.On("GetByJobID", ctx, jobID).Return(batches, nil)

	// Act
	lastBatch, err := processor.resumeFromLastBatch(ctx, jobID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, lastBatch, "Should handle out-of-order batches correctly")
	mockRepo.AssertExpectations(t)
}

// TestSaveBatchProgress_EntityConstruction tests proper entity construction.
func TestSaveBatchProgress_EntityConstruction(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()
	batchNumber := 7
	totalBatches := 15
	chunksProcessed := 250
	status := "completed"

	mockRepo := new(MockBatchProgressRepository)
	processor := &DefaultJobProcessor{
		batchProgressRepo: mockRepo,
	}

	var capturedProgress *entity.BatchJobProgress
	mockRepo.On("Save", ctx, mock.AnythingOfType("*entity.BatchJobProgress")).Run(func(args mock.Arguments) {
		capturedProgress = args.Get(1).(*entity.BatchJobProgress)
	}).Return(nil)

	// Act
	err := processor.saveBatchProgress(ctx, repositoryID, jobID, batchNumber, totalBatches, chunksProcessed, status)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, capturedProgress, "Should capture progress entity")

	// Verify entity was constructed using NewBatchJobProgress
	assert.NotNil(t, capturedProgress.RepositoryID(), "Should have repository ID")
	assert.Equal(t, repositoryID, *capturedProgress.RepositoryID(), "Should have correct repository ID")
	assert.Equal(t, jobID, capturedProgress.IndexingJobID())
	assert.Equal(t, batchNumber, capturedProgress.BatchNumber())
	assert.Equal(t, totalBatches, capturedProgress.TotalBatches())
	assert.Equal(t, chunksProcessed, capturedProgress.ChunksProcessed())
	assert.Equal(t, status, capturedProgress.Status())
	assert.NotEqual(t, uuid.Nil, capturedProgress.ID(), "Should have valid UUID")
	assert.False(t, capturedProgress.CreatedAt().IsZero(), "Should have creation timestamp")
	assert.False(t, capturedProgress.UpdatedAt().IsZero(), "Should have update timestamp")

	mockRepo.AssertExpectations(t)
}

// Helper function to create batch progress entities for testing.
func createBatchProgress(
	jobID uuid.UUID,
	batchNumber int,
	totalBatches int,
	chunksProcessed int,
	status string,
) *entity.BatchJobProgress {
	now := time.Now()
	return entity.RestoreBatchJobProgress(
		uuid.New(),
		nil, // repositoryID (nil for test purposes)
		jobID,
		batchNumber,
		totalBatches,
		chunksProcessed,
		status,
		0,
		nil, // nextRetryAt
		nil, // errorMessage
		nil, // geminiBatchJobID
		nil, // geminiFileURI
		now,
		now,
		nil, // batchRequestData
		0,   // submissionAttempts
		nil, // nextSubmissionAt
	)
}

// Ensure DefaultJobProcessor has batchProgressRepo field (will fail until added).
var _ outbound.BatchProgressRepository = (*MockBatchProgressRepository)(nil)
