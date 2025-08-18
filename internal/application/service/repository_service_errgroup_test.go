package service

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ContextAwareMockRepo provides a mock that properly handles context cancellation.
type ContextAwareMockRepo struct {
	mock.Mock

	callCount *int64
}

func (m *ContextAwareMockRepo) ExistsByNormalizedURL(ctx context.Context, _ valueobject.RepositoryURL) (bool, error) {
	atomic.AddInt64(m.callCount, 1)

	// Check if context is already cancelled
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// Simulate work and check for context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(50 * time.Millisecond):
		// Normal processing - check context one more time
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, nil // No duplicate found, no error
	}
}

// Implement other required methods to satisfy the interface.
func (m *ContextAwareMockRepo) FindByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *ContextAwareMockRepo) Save(ctx context.Context, repository *entity.Repository) error {
	args := m.Called(ctx, repository)
	return args.Error(0)
}

func (m *ContextAwareMockRepo) Update(ctx context.Context, repository *entity.Repository) error {
	args := m.Called(ctx, repository)
	return args.Error(0)
}

func (m *ContextAwareMockRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Repository, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *ContextAwareMockRepo) FindAll(
	ctx context.Context,
	filters outbound.RepositoryFilters,
) ([]*entity.Repository, int, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*entity.Repository), args.Get(1).(int), args.Error(2)
}

func (m *ContextAwareMockRepo) Exists(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

func (m *ContextAwareMockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ContextAwareMockRepo) FindByURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	return args.Get(0).(*entity.Repository), args.Error(1)
}

// TestBatchCheckDuplicates_ContextCancellation tests that the errgroup-based implementation
// properly handles context cancellation and terminates early.
func TestBatchCheckDuplicates_ContextCancellation(t *testing.T) {
	// Setup
	ctx := context.Background()
	callCount := int64(0)
	mockRepo := &ContextAwareMockRepo{callCount: &callCount}
	service := NewPerformantDuplicateDetectionService(mockRepo)

	urls := []string{
		"https://github.com/owner/repo1",
		"https://github.com/owner/repo2",
		"https://github.com/owner/repo3",
		"https://github.com/owner/repo4",
		"https://github.com/owner/repo5",
	}

	// Create a context that will be cancelled after a short delay
	cancelCtx, cancel := context.WithCancel(ctx)

	// Cancel context after 25ms - should interrupt processing during the first URL operations
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	// Execute
	start := time.Now()
	results, err := service.BatchCheckDuplicates(cancelCtx, urls)
	elapsed := time.Since(start)

	// Verify - errgroup should propagate context cancellation error
	require.Error(t, err, "Should return error when context is cancelled")
	require.ErrorIs(t, err, context.Canceled, "Error should be context.Canceled")

	// Should terminate faster than processing all URLs sequentially (5 * 50ms = 250ms)
	assert.Less(t, elapsed, 200*time.Millisecond, "Should terminate early when context is cancelled")

	// errgroup implementation should return early with partial or empty results
	t.Logf("Processing time: %v, calls made: %d", elapsed, atomic.LoadInt64(&callCount))

	// With errgroup, we expect fewer results or an error due to early termination
	if results != nil {
		assert.LessOrEqual(t, len(results), len(urls), "Should not process more URLs than provided")
	}
}

// TestBatchCheckDuplicates_EarlyReturnOnAllInvalidURLs tests that when all URLs are invalid,
// the errgroup implementation returns early without making database calls
// This test will FAIL with current implementation that processes all URLs regardless.
func TestBatchCheckDuplicates_EarlyReturnOnAllInvalidURLs(t *testing.T) {
	// Setup
	ctx := context.Background()
	mockRepo := &MockRepositoryRepositoryWithNormalization{}
	service := NewPerformantDuplicateDetectionService(mockRepo)

	// All invalid URLs that will fail validation
	invalidURLs := []string{
		"not-a-url",
		"ftp://invalid.com",
		"",
		"://malformed",
		"http://",
	}

	// Mock should NOT be called for database operations since URLs are invalid
	// The errgroup implementation should detect this early and avoid database calls
	callCount := int64(0)
	mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(
		false, // Return false for exists
		nil,   // No error
	).Run(func(_ mock.Arguments) {
		atomic.AddInt64(&callCount, 1)
	}).Maybe() // Maybe() because we expect NO calls in the optimized version

	// Execute
	start := time.Now()
	results, err := service.BatchCheckDuplicates(ctx, invalidURLs)
	elapsed := time.Since(start)

	// Verify
	require.NoError(t, err, "Should not return error for invalid URLs, just mark them as errors in results")
	require.Len(t, results, len(invalidURLs), "Should return results for all provided URLs")

	// Verify all results have errors (URL validation failures)
	for i, result := range results {
		assert.Equal(t, invalidURLs[i], result.URL, "Should preserve original URL")
		require.Error(t, result.Error, "Should have validation error for invalid URL")
		assert.False(t, result.IsDuplicate, "Invalid URLs should not be marked as duplicates")
		assert.Empty(t, result.NormalizedURL, "Invalid URLs should not have normalized URL")
		assert.Nil(t, result.ExistingRepository, "Invalid URLs should not have existing repository")
		assert.Greater(t, result.ProcessingTime, time.Duration(0), "Should track processing time")
	}

	// Early return optimization: should not call database for invalid URLs
	assert.Equal(t, int64(0), atomic.LoadInt64(&callCount),
		"Should not make database calls when all URLs are invalid (early return optimization)")

	// Should be fast since no database calls are made
	assert.Less(t, elapsed, 50*time.Millisecond,
		"Should be fast with early return when all URLs are invalid")
}

// TestBatchCheckDuplicates_ErrorPropagation tests that the errgroup implementation
// properly propagates the first error encountered during batch processing.
//
// Note: When database operations complete very quickly (microseconds), all workers
// may complete before errgroup context cancellation can take effect. This is normal
// and expected behavior - errgroup ensures the first error is returned, but doesn't
// retroactively cancel already-completed work.
func TestBatchCheckDuplicates_ErrorPropagation(t *testing.T) {
	// Setup
	ctx := context.Background()
	mockRepo := &MockRepositoryRepositoryWithNormalization{}
	service := NewPerformantDuplicateDetectionService(mockRepo)

	urls := []string{
		"https://github.com/owner/repo1",
		"https://github.com/owner/repo2",
		"https://github.com/owner/repo3",
		"https://github.com/owner/repo4",
	}

	databaseError := errors.New("database connection failed")
	processedCount := int64(0)

	// Setup mock to fail for all URLs to simulate database failure
	mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(
		false,         // Always return false for exists
		databaseError, // Return error for all calls to simulate failure
	).Run(func(_ mock.Arguments) {
		atomic.AddInt64(&processedCount, 1)
	})

	// Execute
	start := time.Now()
	results, err := service.BatchCheckDuplicates(ctx, urls)
	elapsed := time.Since(start)

	// Verify - errgroup should propagate first error
	require.Error(t, err, "Should return the first database error")
	if err != nil {
		assert.Contains(t, err.Error(), "database connection failed", "Should contain original database error")
	}

	finalCount := atomic.LoadInt64(&processedCount)
	t.Logf("Database calls made: %d, processing time: %v", finalCount, elapsed)

	// For fast-completing operations, errgroup may process all URLs before
	// context cancellation takes effect. This is expected behavior.
	assert.LessOrEqual(t, finalCount, int64(len(urls)),
		"Should not process more URLs than provided")
	assert.Positive(t, finalCount,
		"Should process at least one URL before encountering error")

	// Should terminate reasonably quickly since database operations fail fast
	assert.Less(t, elapsed, 200*time.Millisecond,
		"Should terminate quickly when database errors occur")

	// errgroup implementation should return error instead of partial results
	assert.Nil(t, results, "Should return nil results when errgroup encounters error")
}

// TestBatchCheckDuplicates_BoundedWorkerPool tests that the errgroup implementation
// respects worker pool limits and doesn't exceed concurrency bounds
// This test will FAIL with current semaphore implementation details.
func TestBatchCheckDuplicates_BoundedWorkerPool(t *testing.T) {
	// Setup
	ctx := context.Background()
	mockRepo := &MockRepositoryRepositoryWithNormalization{}
	service := NewPerformantDuplicateDetectionService(mockRepo)

	// Create enough URLs to test concurrency limits
	urls := make([]string, 20)
	for i := range 20 {
		urls[i] = "https://github.com/owner/repo" + string(rune('A'+i))
	}

	concurrentCalls := int64(0)
	maxConcurrentCalls := int64(0)

	// Mock tracks concurrent calls to verify bounded worker pool
	mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.AnythingOfType("valueobject.RepositoryURL")).Return(
		false, // Return false for exists
		nil,   // No error
	).Run(func(_ mock.Arguments) {
		current := atomic.AddInt64(&concurrentCalls, 1)

		// Track maximum concurrent calls
		for {
			currentMax := atomic.LoadInt64(&maxConcurrentCalls)
			if current <= currentMax || atomic.CompareAndSwapInt64(&maxConcurrentCalls, currentMax, current) {
				break
			}
		}

		// Simulate work
		time.Sleep(10 * time.Millisecond)

		atomic.AddInt64(&concurrentCalls, -1)
	})

	// Execute
	start := time.Now()
	results, err := service.BatchCheckDuplicates(ctx, urls)
	elapsed := time.Since(start)

	// Verify
	require.NoError(t, err, "Should not return error for successful processing")
	require.Len(t, results, len(urls), "Should return results for all URLs")

	// Verify bounded worker pool - errgroup should limit concurrency
	// Current implementation uses semaphore with limit of 10
	// errgroup implementation should also respect this limit
	maxConcurrent := atomic.LoadInt64(&maxConcurrentCalls)
	t.Logf("Max concurrent calls: %d, total time: %v", maxConcurrent, elapsed)

	// Should not exceed the worker pool limit (expected to be 10)
	assert.LessOrEqual(t, maxConcurrent, int64(10),
		"Should not exceed bounded worker pool limit")
	assert.Greater(t, maxConcurrent, int64(1),
		"Should use concurrent processing")

	// Verify all results are correct
	for i, result := range results {
		assert.Equal(t, urls[i], result.URL, "Should preserve original URL")
		require.NoError(t, result.Error, "Should not have error for valid URLs")
		assert.False(t, result.IsDuplicate, "Mock returns false for duplicates")
		assert.NotEmpty(t, result.NormalizedURL, "Should have normalized URL")
		assert.Nil(t, result.ExistingRepository, "No existing repository in this test")
		assert.Greater(t, result.ProcessingTime, time.Duration(0), "Should track processing time")
	}
}

// TestBatchCheckDuplicates_MaintainsSameSuccessfulBehavior tests that the errgroup
// implementation maintains exact same behavior for successful cases
// This test should PASS with both implementations.
func TestBatchCheckDuplicates_MaintainsSameSuccessfulBehavior(t *testing.T) {
	// Setup
	ctx := context.Background()
	mockRepo := &MockRepositoryRepositoryWithNormalization{}
	service := NewPerformantDuplicateDetectionService(mockRepo)

	urls := []string{
		"https://github.com/owner/repo1", // Not duplicate
		"https://github.com/owner/repo2", // Duplicate
		"https://github.com/owner/repo3", // Not duplicate
		"invalid-url",                    // Invalid URL
	}

	// Create existing repository for duplicate test
	existingRepo := createTestRepository("https://github.com/owner/repo2")

	// Setup mocks for each URL
	mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.MatchedBy(func(url valueobject.RepositoryURL) bool {
		return url.String() == "https://github.com/owner/repo1"
	})).Return(false, nil)

	mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.MatchedBy(func(url valueobject.RepositoryURL) bool {
		return url.String() == "https://github.com/owner/repo2"
	})).Return(true, nil)

	mockRepo.On("FindByNormalizedURL", mock.Anything, mock.MatchedBy(func(url valueobject.RepositoryURL) bool {
		return url.String() == "https://github.com/owner/repo2"
	})).Return(existingRepo, nil)

	mockRepo.On("ExistsByNormalizedURL", mock.Anything, mock.MatchedBy(func(url valueobject.RepositoryURL) bool {
		return url.String() == "https://github.com/owner/repo3"
	})).Return(false, nil)

	// Execute
	results, err := service.BatchCheckDuplicates(ctx, urls)

	// Verify - should maintain exact same behavior as current implementation
	require.NoError(t, err, "Should not return error for mixed valid/invalid URLs")
	require.Len(t, results, len(urls), "Should return results for all URLs")

	// First URL - not duplicate
	assert.Equal(t, urls[0], results[0].URL)
	require.NoError(t, results[0].Error)
	assert.False(t, results[0].IsDuplicate)
	assert.Equal(t, "https://github.com/owner/repo1", results[0].NormalizedURL)
	assert.Nil(t, results[0].ExistingRepository)
	assert.Greater(t, results[0].ProcessingTime, time.Duration(0))

	// Second URL - duplicate
	assert.Equal(t, urls[1], results[1].URL)
	require.NoError(t, results[1].Error)
	assert.True(t, results[1].IsDuplicate)
	assert.Equal(t, "https://github.com/owner/repo2", results[1].NormalizedURL)
	assert.Equal(t, existingRepo, results[1].ExistingRepository)
	assert.Greater(t, results[1].ProcessingTime, time.Duration(0))

	// Third URL - not duplicate
	assert.Equal(t, urls[2], results[2].URL)
	require.NoError(t, results[2].Error)
	assert.False(t, results[2].IsDuplicate)
	assert.Equal(t, "https://github.com/owner/repo3", results[2].NormalizedURL)
	assert.Nil(t, results[2].ExistingRepository)
	assert.Greater(t, results[2].ProcessingTime, time.Duration(0))

	// Fourth URL - invalid
	assert.Equal(t, urls[3], results[3].URL)
	require.Error(t, results[3].Error)
	assert.False(t, results[3].IsDuplicate)
	assert.Empty(t, results[3].NormalizedURL)
	assert.Nil(t, results[3].ExistingRepository)
	assert.Greater(t, results[3].ProcessingTime, time.Duration(0))

	// Verify all mocks were called appropriately
	mockRepo.AssertExpectations(t)
}

// TestBatchCheckDuplicates_EmptyURLSlice tests edge case behavior
// This should pass with both implementations.
func TestBatchCheckDuplicates_EmptyURLSlice(t *testing.T) {
	// Setup
	ctx := context.Background()
	mockRepo := &MockRepositoryRepositoryWithNormalization{}
	service := NewPerformantDuplicateDetectionService(mockRepo)

	// Execute
	results, err := service.BatchCheckDuplicates(ctx, []string{})

	// Verify
	require.NoError(t, err, "Should not return error for empty slice")
	require.Empty(t, results, "Should return empty results for empty input")

	// Should not make any database calls
	mockRepo.AssertNotCalled(t, "ExistsByNormalizedURL")
	mockRepo.AssertNotCalled(t, "FindByNormalizedURL")
}

// TestBatchCheckDuplicates_ContextDeadline tests behavior with context deadline.
// Uses ContextAwareMockRepo which properly simulates slow operations that are
// interrupted by context deadline, causing errgroup to return a deadline exceeded error.
func TestBatchCheckDuplicates_ContextDeadline(t *testing.T) {
	// Setup - use the context-aware mock that supports timeout simulation
	ctx := context.Background()
	callCount := int64(0)
	mockRepo := &ContextAwareMockRepo{callCount: &callCount}
	service := NewPerformantDuplicateDetectionService(mockRepo)

	urls := []string{
		"https://github.com/owner/repo1",
		"https://github.com/owner/repo2",
		"https://github.com/owner/repo3",
	}

	// Create context with very short deadline - shorter than the 50ms delay in ContextAwareMockRepo
	deadlineCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()

	// Execute
	start := time.Now()
	results, err := service.BatchCheckDuplicates(deadlineCtx, urls)
	elapsed := time.Since(start)

	// Verify - errgroup should respect context deadline
	require.Error(t, err, "Should return error when context deadline is exceeded")
	require.ErrorIs(t, err, context.DeadlineExceeded, "Error should be context.DeadlineExceeded")

	// Should terminate quickly due to deadline (around 20ms, not 50ms)
	assert.Less(t, elapsed, 100*time.Millisecond, "Should terminate when context deadline is exceeded")
	assert.GreaterOrEqual(t, elapsed, 15*time.Millisecond, "Should take at least near the deadline duration")

	// The ContextAwareMockRepo simulates 50ms operations, but context deadline is 20ms
	// so not all operations should complete
	finalCallCount := atomic.LoadInt64(&callCount)
	t.Logf("Processing time: %v, calls made: %d", elapsed, finalCallCount)

	// errgroup should return error, not partial results
	assert.Nil(t, results, "Should return nil results when context deadline is exceeded")
}

// MockRepositoryRepositoryWithNormalization is already defined in duplicate_detection_service_test.go
// We'll reuse that definition to avoid duplication
// stringPtr and createTestRepository helper functions are already defined in other test files
