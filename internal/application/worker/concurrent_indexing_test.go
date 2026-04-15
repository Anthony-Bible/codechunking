package worker

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// MockZoektIndexer — testify mock implementing outbound.ZoektIndexer
// ---------------------------------------------------------------------------

type MockZoektIndexer struct {
	mock.Mock
}

func (m *MockZoektIndexer) Index(ctx context.Context, cfg *outbound.ZoektRepositoryConfig) (*outbound.ZoektIndexResult, error) {
	args := m.Called(ctx, cfg)
	result, _ := args.Get(0).(*outbound.ZoektIndexResult)
	return result, args.Error(1)
}

func (m *MockZoektIndexer) CheckIndexStatus(ctx context.Context, repoName, commitHash string) (*outbound.ZoektIndexStatus, error) {
	args := m.Called(ctx, repoName, commitHash)
	status, _ := args.Get(0).(*outbound.ZoektIndexStatus)
	return status, args.Error(1)
}

func (m *MockZoektIndexer) DeleteRepository(ctx context.Context, repoName string) error {
	args := m.Called(ctx, repoName)
	return args.Error(0)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func enabledZoektConfig() config.ZoektConfig {
	return config.ZoektConfig{
		ConcurrentIndexing: config.ZoektConcurrentIndexingConfig{
			Enabled: true,
		},
		Indexing: config.ZoektIndexingConfig{
			IndexDir: "/tmp/zoekt-index",
		},
	}
}

func disabledZoektConfig() config.ZoektConfig {
	return config.ZoektConfig{
		ConcurrentIndexing: config.ZoektConcurrentIndexingConfig{
			Enabled: false,
		},
	}
}

func processorWithZoekt(indexer outbound.ZoektIndexer, cfg config.ZoektConfig) *DefaultJobProcessor {
	p := &DefaultJobProcessor{}
	p.zoektIndexer = indexer
	p.zoektConfig = cfg
	return p
}

// ---------------------------------------------------------------------------
// shouldRunConcurrentIndexing
// ---------------------------------------------------------------------------

// TestShouldRunConcurrentIndexing_NilIndexer verifies that concurrent indexing
// is disabled when the zoektIndexer field is nil, regardless of config.
func TestShouldRunConcurrentIndexing_NilIndexer(t *testing.T) {
	p := processorWithZoekt(nil, enabledZoektConfig())

	got := p.shouldRunConcurrentIndexing()

	assert.False(t, got, "shouldRunConcurrentIndexing should be false when indexer is nil")
}

// TestShouldRunConcurrentIndexing_ConfigDisabled verifies that concurrent indexing
// is disabled when config.ConcurrentIndexing.Enabled is false.
func TestShouldRunConcurrentIndexing_ConfigDisabled(t *testing.T) {
	indexer := &MockZoektIndexer{}
	p := processorWithZoekt(indexer, disabledZoektConfig())

	got := p.shouldRunConcurrentIndexing()

	assert.False(t, got, "shouldRunConcurrentIndexing should be false when config is disabled")
}

// TestShouldRunConcurrentIndexing_EnabledAndIndexerSet verifies that concurrent
// indexing is enabled when both the indexer is non-nil and config is enabled.
func TestShouldRunConcurrentIndexing_EnabledAndIndexerSet(t *testing.T) {
	indexer := &MockZoektIndexer{}
	p := processorWithZoekt(indexer, enabledZoektConfig())

	got := p.shouldRunConcurrentIndexing()

	assert.True(t, got, "shouldRunConcurrentIndexing should be true when indexer set and config enabled")
}

// ---------------------------------------------------------------------------
// runConcurrentIndexing
// ---------------------------------------------------------------------------

// mockGenerateEmbeddings is a helper that injects an embeddingFn into a processor
// so we can control the embedding side of runConcurrentIndexing in tests.
// The real generateEmbeddings requires real repos; we stub it via the function field.
func processorWithZoektAndEmbedFn(
	indexer outbound.ZoektIndexer,
	cfg config.ZoektConfig,
	embFn func(ctx context.Context) error,
) *DefaultJobProcessor {
	p := processorWithZoekt(indexer, cfg)
	p.embeddingFnOverride = embFn
	return p
}

// makeChunks creates n minimal CodeChunk values for use in tests.
func makeChunks(n int) []outbound.CodeChunk {
	chunks := make([]outbound.CodeChunk, n)
	return chunks
}

// TestRunConcurrentIndexing_BothSucceed verifies that when both Zoekt and
// embedding succeed, the result has no errors and both were called.
func TestRunConcurrentIndexing_BothSucceed(t *testing.T) {
	zoektResult := &outbound.ZoektIndexResult{FileCount: 5, ShardCount: 1}
	indexer := &MockZoektIndexer{}
	indexer.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
		Return(zoektResult, nil)

	embCalled := false
	p := processorWithZoektAndEmbedFn(indexer, enabledZoektConfig(), func(_ context.Context) error {
		embCalled = true
		return nil
	})

	chunks := makeChunks(10)
	result := p.runConcurrentIndexing(context.Background(), "repo-name", "/workspace", "abc123", chunks, uuid.Nil, uuid.Nil)

	assert.NoError(t, result.ZoektErr)
	assert.NoError(t, result.EmbeddingErr)
	assert.True(t, result.BothSucceeded())
	assert.True(t, embCalled, "embedding function should have been called")
	assert.Equal(t, zoektResult, result.ZoektResult)
	assert.Equal(t, 10, result.ChunksProcessed)
	indexer.AssertExpectations(t)
}

// TestRunConcurrentIndexing_ZoektFails_EmbeddingSucceeds verifies that when
// Zoekt fails but embedding succeeds, AnySucceeded is true and ZoektErr is set.
func TestRunConcurrentIndexing_ZoektFails_EmbeddingSucceeds(t *testing.T) {
	zoektErr := errors.New("zoekt indexing failed")
	indexer := &MockZoektIndexer{}
	indexer.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
		Return((*outbound.ZoektIndexResult)(nil), zoektErr)

	p := processorWithZoektAndEmbedFn(indexer, enabledZoektConfig(), func(_ context.Context) error {
		return nil
	})

	result := p.runConcurrentIndexing(context.Background(), "repo-name", "/workspace", "abc123", makeChunks(5), uuid.Nil, uuid.Nil)

	assert.ErrorIs(t, result.ZoektErr, zoektErr)
	assert.NoError(t, result.EmbeddingErr)
	assert.True(t, result.AnySucceeded())
	assert.False(t, result.BothFailed())
	indexer.AssertExpectations(t)
}

// TestRunConcurrentIndexing_EmbeddingFails_ZoektSucceeds verifies that when
// embedding fails but Zoekt succeeds, AnySucceeded is true and EmbeddingErr is set.
func TestRunConcurrentIndexing_EmbeddingFails_ZoektSucceeds(t *testing.T) {
	embErr := errors.New("embedding generation failed")
	zoektResult := &outbound.ZoektIndexResult{FileCount: 3, ShardCount: 1}
	indexer := &MockZoektIndexer{}
	indexer.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
		Return(zoektResult, nil)

	p := processorWithZoektAndEmbedFn(indexer, enabledZoektConfig(), func(_ context.Context) error {
		return embErr
	})

	result := p.runConcurrentIndexing(context.Background(), "repo-name", "/workspace", "abc123", makeChunks(7), uuid.Nil, uuid.Nil)

	assert.NoError(t, result.ZoektErr)
	assert.ErrorIs(t, result.EmbeddingErr, embErr)
	assert.True(t, result.AnySucceeded())
	assert.False(t, result.BothFailed())
	indexer.AssertExpectations(t)
}

// TestRunConcurrentIndexing_BothFail verifies that when both engines fail,
// BothFailed returns true.
func TestRunConcurrentIndexing_BothFail(t *testing.T) {
	zoektErr := errors.New("zoekt failed")
	embErr := errors.New("embedding failed")

	indexer := &MockZoektIndexer{}
	indexer.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
		Return((*outbound.ZoektIndexResult)(nil), zoektErr)

	p := processorWithZoektAndEmbedFn(indexer, enabledZoektConfig(), func(_ context.Context) error {
		return embErr
	})

	result := p.runConcurrentIndexing(context.Background(), "repo-name", "/workspace", "abc123", nil, uuid.Nil, uuid.Nil)

	assert.ErrorIs(t, result.ZoektErr, zoektErr)
	assert.ErrorIs(t, result.EmbeddingErr, embErr)
	assert.True(t, result.BothFailed())
	assert.False(t, result.AnySucceeded())
	indexer.AssertExpectations(t)
}

// TestRunConcurrentIndexing_DurationsRecorded verifies that both durations
// are populated (greater than zero) after a successful run.
func TestRunConcurrentIndexing_DurationsRecorded(t *testing.T) {
	indexer := &MockZoektIndexer{}
	indexer.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
		After(1 * time.Millisecond).
		Return(&outbound.ZoektIndexResult{}, nil)

	p := processorWithZoektAndEmbedFn(indexer, enabledZoektConfig(), func(_ context.Context) error {
		time.Sleep(1 * time.Millisecond)
		return nil
	})

	result := p.runConcurrentIndexing(context.Background(), "repo-name", "/workspace", "abc123", makeChunks(1), uuid.Nil, uuid.Nil)

	assert.Greater(t, result.ZoektDuration, time.Duration(0), "ZoektDuration should be > 0")
	assert.Greater(t, result.EmbeddingDuration, time.Duration(0), "EmbeddingDuration should be > 0")
	indexer.AssertExpectations(t)
}
