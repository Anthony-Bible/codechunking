package worker

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"testing/synctest"
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

func processorWithZoektAndEmbedFn(
	indexer outbound.ZoektIndexer,
	cfg config.ZoektConfig,
	embFn func(ctx context.Context, jobID, repoID uuid.UUID, chunks []outbound.CodeChunk) error,
) *DefaultJobProcessor {
	p := processorWithZoekt(indexer, cfg)
	p.embeddingFn = embFn
	return p
}

// makeChunks creates n minimal CodeChunk values for use in tests.
func makeChunks(n int) []outbound.CodeChunk {
	return make([]outbound.CodeChunk, n)
}

// ---------------------------------------------------------------------------
// shouldRunConcurrentIndexing
// ---------------------------------------------------------------------------

func TestShouldRunConcurrentIndexing(t *testing.T) {
	tests := []struct {
		name    string
		indexer outbound.ZoektIndexer
		cfg     config.ZoektConfig
		want    bool
	}{
		{"nil indexer, config enabled", nil, enabledZoektConfig(), false},
		{"indexer set, config disabled", &MockZoektIndexer{}, disabledZoektConfig(), false},
		{"indexer set, config enabled", &MockZoektIndexer{}, enabledZoektConfig(), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := processorWithZoekt(tc.indexer, tc.cfg)
			assert.Equal(t, tc.want, p.shouldRunConcurrentIndexing())
		})
	}
}

// ---------------------------------------------------------------------------
// runConcurrentIndexing — outcome tests
// ---------------------------------------------------------------------------

func TestRunConcurrentIndexing(t *testing.T) {
	var (
		someZoektErr = errors.New("zoekt indexing failed")
		someEmbErr   = errors.New("embedding generation failed")
		zoektResult  = &outbound.ZoektIndexResult{FileCount: 5, ShardCount: 1}
	)

	tests := []struct {
		name              string
		zoektResult       *outbound.ZoektIndexResult
		zoektErr          error
		embErr            error
		chunkCount        int
		wantBothSucceeded bool
		wantBothFailed    bool
		wantAnySucceeded  bool
	}{
		{
			name:              "both succeed",
			zoektResult:       zoektResult,
			chunkCount:        10,
			wantBothSucceeded: true,
			wantAnySucceeded:  true,
		},
		{
			name:             "zoekt fails, embedding succeeds",
			zoektErr:         someZoektErr,
			chunkCount:       5,
			wantAnySucceeded: true,
		},
		{
			name:             "embedding fails, zoekt succeeds",
			zoektResult:      &outbound.ZoektIndexResult{FileCount: 3, ShardCount: 1},
			embErr:           someEmbErr,
			chunkCount:       7,
			wantAnySucceeded: true,
		},
		{
			name:           "both fail",
			zoektErr:       someZoektErr,
			embErr:         someEmbErr,
			wantBothFailed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			indexer := &MockZoektIndexer{}
			indexer.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
				Return(tc.zoektResult, tc.zoektErr)

			p := processorWithZoektAndEmbedFn(indexer, enabledZoektConfig(), func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ []outbound.CodeChunk) error {
				return tc.embErr
			})

			result := p.runConcurrentIndexing(
				context.Background(), "repo-name", "/workspace", "abc123",
				makeChunks(tc.chunkCount), uuid.Nil, uuid.Nil,
			)

			if tc.zoektErr != nil {
				assert.ErrorIs(t, result.ZoektErr, tc.zoektErr)
			} else {
				assert.NoError(t, result.ZoektErr)
			}
			if tc.embErr != nil {
				assert.ErrorIs(t, result.EmbeddingErr, tc.embErr)
			} else {
				assert.NoError(t, result.EmbeddingErr)
			}

			assert.Equal(t, tc.wantBothSucceeded, result.BothSucceeded())
			assert.Equal(t, tc.wantBothFailed, result.BothFailed())
			assert.Equal(t, tc.wantAnySucceeded, result.AnySucceeded())
			assert.Equal(t, tc.chunkCount, result.ChunksProcessed)

			if tc.zoektResult != nil {
				assert.Equal(t, tc.zoektResult, result.ZoektResult)
			}

			indexer.AssertExpectations(t)
		})
	}
}

// TestRunConcurrentIndexing_DurationsRecorded verifies that both durations are
// populated (greater than zero) after a successful run. Kept standalone because
// it tests timing behaviour rather than outcome logic.
func TestRunConcurrentIndexing_DurationsRecorded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		indexer := &MockZoektIndexer{}
		indexer.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
			After(1*time.Millisecond).
			Return(&outbound.ZoektIndexResult{}, nil)

		p := processorWithZoektAndEmbedFn(indexer, enabledZoektConfig(), func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ []outbound.CodeChunk) error {
			time.Sleep(1 * time.Millisecond)
			return nil
		})

		result := p.runConcurrentIndexing(
			context.Background(), "repo-name", "/workspace", "abc123",
			makeChunks(1), uuid.Nil, uuid.Nil,
		)

		assert.Greater(t, result.ZoektDuration, time.Duration(0), "ZoektDuration should be > 0")
		assert.Greater(t, result.EmbeddingDuration, time.Duration(0), "EmbeddingDuration should be > 0")
		indexer.AssertExpectations(t)
	})
}
