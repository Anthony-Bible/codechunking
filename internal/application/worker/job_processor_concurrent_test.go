package worker

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/messaging"
	"codechunking/internal/domain/valueobject"
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

// ---------------------------------------------------------------------------
// helpers shared across this file
// ---------------------------------------------------------------------------

// makePendingRepo returns a Repository in its initial pending state.
func makePendingRepo(t *testing.T, rawURL string) *entity.Repository {
	t.Helper()
	repoURL, err := valueobject.NewRepositoryURL(rawURL)
	require.NoError(t, err)
	return entity.NewRepository(repoURL, "test-repo", nil, nil)
}

// makeProcessingRepo returns a Repository that has been transitioned through
// pending → cloning → processing, which is the state the pipeline expects when
// it reaches the indexing step.
func makeProcessingRepo(t *testing.T, rawURL string) *entity.Repository {
	t.Helper()
	repo := makePendingRepo(t, rawURL)
	require.NoError(t, repo.UpdateStatus(valueobject.RepositoryStatusCloning))
	require.NoError(t, repo.UpdateStatus(valueobject.RepositoryStatusProcessing))
	return repo
}

// concurrentTestProcessorConfig is the minimal JobProcessorConfig used by
// processor factory helpers in this file.
func concurrentTestProcessorConfig() JobProcessorConfig {
	return JobProcessorConfig{
		WorkspaceDir:      "/tmp/codechunking-concurrent-tests",
		MaxConcurrentJobs: 1,
		JobTimeout:        30 * time.Second,
	}
}

// buildConcurrentProcessor constructs a DefaultJobProcessor with Zoekt enabled,
// wiring in the supplied mocks and embedding function.
func buildConcurrentProcessor(
	repoRepo *MockRepositoryRepository,
	gitClient *MockEnhancedGitClient,
	parser *MockCodeParser,
	zoektIndexer outbound.ZoektIndexer,
	embFn func(ctx context.Context, jobID, repoID uuid.UUID, chunks []outbound.CodeChunk) error,
) *DefaultJobProcessor {
	return &DefaultJobProcessor{
		config:          concurrentTestProcessorConfig(),
		repositoryRepo:  repoRepo,
		indexingJobRepo: newTestMockJobRepo(),
		gitClient:       gitClient,
		codeParser:      parser,
		embeddingFn:     embFn,
		zoektIndexer:    zoektIndexer,
		zoektConfig:     enabledZoektConfig(),
		activeJobs:      make(map[string]*JobExecution),
		semaphore:       make(chan struct{}, 1),
	}
}

// newExecution creates a minimal JobExecution for use in pipeline tests.
func newExecution(repoID uuid.UUID) *JobExecution {
	return &JobExecution{
		JobID:        uuid.New(),
		RepositoryID: repoID,
		StartTime:    time.Now(),
		Status:       jobStatusRunning,
		Progress:     &JobProgress{},
	}
}

// seqRepoFindByID wires n sequential Once() returns to the mock so that each
// pipeline-internal FindByID call gets the right entity at the right stage.
func seqRepoFindByID(m *MockRepositoryRepository, repoID uuid.UUID, repos ...*entity.Repository) {
	for _, r := range repos {
		m.On("FindByID", mock.Anything, repoID).Return(r, nil).Once()
	}
}

// TestExecuteJobPipeline_NormalisesRepoNameForZoekt asserts that executeJobPipeline
// passes the normalised "host/owner/repo" form as repoName to Zoekt, not the raw URL.

func TestExecuteJobPipeline_NormalisesRepoNameForZoekt(t *testing.T) {
	repoID := uuid.New()
	const rawURL = "https://github.com/example/myrepo.git"
	const wantRepoName = "github.com/example/myrepo"

	// The pipeline calls FindByID at multiple internal transition points.
	// We provide fresh entity copies for each call so domain state-machine
	// invariants are never violated.
	pending1 := makePendingRepo(t, rawURL)
	pending2 := makePendingRepo(t, rawURL)
	cloning1 := makePendingRepo(t, rawURL)
	require.NoError(t, cloning1.UpdateStatus(valueobject.RepositoryStatusCloning))
	cloning2 := makePendingRepo(t, rawURL)
	require.NoError(t, cloning2.UpdateStatus(valueobject.RepositoryStatusCloning))
	processing := makePendingRepo(t, rawURL)
	require.NoError(t, processing.UpdateStatus(valueobject.RepositoryStatusCloning))
	require.NoError(t, processing.UpdateStatus(valueobject.RepositoryStatusProcessing))

	mockRepoRepo := &MockRepositoryRepository{}
	seqRepoFindByID(mockRepoRepo, repoID, pending1, pending2, cloning1, cloning2, processing)
	// finalizeWithConcurrencyResult needs one more FindByID for the completed repo.
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(processing, nil).Maybe()
	mockRepoRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockGitClient.On("GetCommitHash", mock.Anything, mock.Anything).Return("abc123", nil)

	mockParser := &MockCodeParser{}
	mockParser.On("ParseDirectory", mock.Anything, mock.Anything, mock.Anything).
		Return([]outbound.CodeChunk{{ID: "c1", Content: "test", FilePath: "main.go"}}, nil)

	var capturedRepoName string
	mockZoekt := &MockZoektIndexer{}
	mockZoekt.On("Index", mock.Anything, mock.MatchedBy(func(cfg *outbound.ZoektRepositoryConfig) bool {
		capturedRepoName = cfg.Name
		return true
	})).Return(&outbound.ZoektIndexResult{FileCount: 1, ShardCount: 1}, nil)

	p := buildConcurrentProcessor(
		mockRepoRepo, mockGitClient, mockParser, mockZoekt,
		func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ []outbound.CodeChunk) error { return nil },
	)

	msg := messaging.EnhancedIndexingJobMessage{
		MessageID:     "url-normalise-test",
		RepositoryID:  repoID,
		RepositoryURL: rawURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _, err := p.executeJobPipeline(ctx, msg, "/tmp/codechunking-concurrent-tests/url-normalise", newExecution(repoID))

	require.NoError(t, err)

	assert.Equal(t, wantRepoName, capturedRepoName,
		"repoName passed to Zoekt must be the normalised 'host/owner/repo' form, not the raw URL")

	mockZoekt.AssertExpectations(t)
}

// TestFinalizeWithConcurrencyResult_UsesResultCommitHash asserts that
// finalizeWithConcurrencyResult persists the commit hash from ConcurrencyResult
// rather than the hardcoded "unknown" sentinel.
func TestFinalizeWithConcurrencyResult_UsesResultCommitHash(t *testing.T) {
	const realHash = "deadbeef99887766"
	repoID := uuid.New()

	repo := makeProcessingRepo(t, "https://github.com/example/myapp.git")

	mockRepoRepo := &MockRepositoryRepository{}
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(repo, nil).Once()

	var capturedHash string
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		if h := r.LastCommitHash(); h != nil {
			capturedHash = *h
		}
		return true
	})).Return(nil).Once()

	p := &DefaultJobProcessor{repositoryRepo: mockRepoRepo}

	result := &ConcurrencyResult{
		ZoektResult:     &outbound.ZoektIndexResult{FileCount: 5, ShardCount: 2},
		ChunksProcessed: 10,
		CommitHash:      realHash,
	}

	p.finalizeWithConcurrencyResult(context.Background(), repoID, result, 5, 10)

	assert.Equal(t, realHash, capturedHash,
		"finalizeWithConcurrencyResult must use result.CommitHash, not the 'unknown' sentinel")

	mockRepoRepo.AssertExpectations(t)
}

// TestFinalizeWithConcurrencyResult_OneEngineSucceeds asserts that when Zoekt
// succeeds but embedding fails, the repository is still marked completed overall
// and per-engine statuses reflect each engine's outcome.

func TestFinalizeWithConcurrencyResult_OneEngineSucceeds(t *testing.T) {
	repoID := uuid.New()
	repo := makeProcessingRepo(t, "https://github.com/example/partial-success.git")

	mockRepoRepo := &MockRepositoryRepository{}
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(repo, nil).Once()

	var finalRepo *entity.Repository
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		finalRepo = r
		return true
	})).Return(nil).Once()

	p := &DefaultJobProcessor{repositoryRepo: mockRepoRepo}

	result := &ConcurrencyResult{
		ZoektResult:     &outbound.ZoektIndexResult{FileCount: 7, ShardCount: 1},
		EmbeddingErr:    errors.New("embedding generation failed"),
		ChunksProcessed: 15,
	}

	p.finalizeWithConcurrencyResult(context.Background(), repoID, result, 7, 15)

	require.NotNil(t, finalRepo, "repository must be persisted")
	assert.Equal(t, valueobject.RepositoryStatusCompleted, finalRepo.Status(),
		"overall status must be completed when at least one engine succeeded")
	assert.Equal(t, valueobject.ZoektIndexStatusCompleted, finalRepo.ZoektIndexStatus(),
		"Zoekt engine status must be completed because Zoekt succeeded")
	assert.Equal(t, valueobject.EmbeddingIndexStatusFailed, finalRepo.EmbeddingIndexStatus(),
		"embedding engine status must be failed because embedding errored")

	mockRepoRepo.AssertExpectations(t)
}

// TestFinalizeWithConcurrencyResult_BothSucceed asserts that when both engines
// succeed both statuses are completed and the real commit hash is persisted.

func TestFinalizeWithConcurrencyResult_BothSucceed(t *testing.T) {
	repoID := uuid.New()
	repo := makeProcessingRepo(t, "https://github.com/example/both-succeed.git")

	mockRepoRepo := &MockRepositoryRepository{}
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(repo, nil).Once()

	var finalRepo *entity.Repository
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		finalRepo = r
		return true
	})).Return(nil).Once()

	p := &DefaultJobProcessor{repositoryRepo: mockRepoRepo}

	result := &ConcurrencyResult{
		ZoektResult:     &outbound.ZoektIndexResult{FileCount: 10, ShardCount: 3},
		ChunksProcessed: 20,
		CommitHash:      "cafebabe12345678",
	}

	p.finalizeWithConcurrencyResult(context.Background(), repoID, result, 10, 20)

	require.NotNil(t, finalRepo, "repository must be persisted")
	assert.Equal(t, valueobject.RepositoryStatusCompleted, finalRepo.Status(),
		"overall status must be completed when both engines succeed")
	assert.Equal(t, valueobject.ZoektIndexStatusCompleted, finalRepo.ZoektIndexStatus(),
		"Zoekt engine status must be completed")
	assert.Equal(t, valueobject.EmbeddingIndexStatusCompleted, finalRepo.EmbeddingIndexStatus(),
		"embedding engine status must be completed")
	require.NotNil(t, finalRepo.LastCommitHash())
	assert.Equal(t, "cafebabe12345678", *finalRepo.LastCommitHash(),
		"MarkIndexingCompleted must receive the real commit hash from ConcurrencyResult")

	mockRepoRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Fix 4 (pipeline smoke test — both-fail branch)
//
// TestExecuteJobPipeline_ConcurrentBothFail asserts that when both engines fail
// executeJobPipeline returns an error and the repository is marked failed.

func TestExecuteJobPipeline_ConcurrentBothFail(t *testing.T) {
	repoID := uuid.New()
	const rawURL = "https://github.com/example/both-fail.git"

	// Provide fresh copies for each internal FindByID call.
	pending1 := makePendingRepo(t, rawURL)
	pending2 := makePendingRepo(t, rawURL)
	cloning1 := makePendingRepo(t, rawURL)
	require.NoError(t, cloning1.UpdateStatus(valueobject.RepositoryStatusCloning))
	cloning2 := makePendingRepo(t, rawURL)
	require.NoError(t, cloning2.UpdateStatus(valueobject.RepositoryStatusCloning))
	processing := makePendingRepo(t, rawURL)
	require.NoError(t, processing.UpdateStatus(valueobject.RepositoryStatusCloning))
	require.NoError(t, processing.UpdateStatus(valueobject.RepositoryStatusProcessing))

	mockRepoRepo := &MockRepositoryRepository{}
	seqRepoFindByID(mockRepoRepo, repoID, pending1, pending2, cloning1, cloning2, processing)
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(processing, nil).Maybe()
	mockRepoRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockGitClient.On("GetCommitHash", mock.Anything, mock.Anything).Return("abc123", nil)

	mockParser := &MockCodeParser{}
	mockParser.On("ParseDirectory", mock.Anything, mock.Anything, mock.Anything).
		Return([]outbound.CodeChunk{{ID: "c1", Content: "func Foo(){}", FilePath: "foo.go"}}, nil)

	zoektErr := errors.New("zoekt failed completely")
	embErr := errors.New("embedding failed completely")

	mockZoekt := &MockZoektIndexer{}
	mockZoekt.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
		Return((*outbound.ZoektIndexResult)(nil), zoektErr)

	p := buildConcurrentProcessor(
		mockRepoRepo, mockGitClient, mockParser, mockZoekt,
		func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ []outbound.CodeChunk) error { return embErr },
	)

	msg := messaging.EnhancedIndexingJobMessage{
		MessageID:     "both-fail-test",
		RepositoryID:  repoID,
		RepositoryURL: rawURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	chunks, concResult, err := p.executeJobPipeline(
		ctx, msg, "/tmp/codechunking-concurrent-tests/workspace-both-fail", newExecution(repoID))

	assert.Error(t, err, "executeJobPipeline must return an error when both engines fail")
	assert.Nil(t, chunks, "chunks must be nil on the concurrent both-fail path")
	assert.Nil(t, concResult, "ConcurrencyResult must be nil on the both-fail path")
	assert.Contains(t, err.Error(), "both indexing engines failed",
		"error message must indicate that both engines failed")

	mockZoekt.AssertExpectations(t)
}

// TestExecuteJobPipeline_LogsWarnOnCommitHashError asserts that a GetCommitHash
// failure does NOT abort the pipeline — the processor falls back to unknownCommitHash
// and continues. The warning log is emitted but not captured here.

func TestExecuteJobPipeline_LogsWarnOnCommitHashError(t *testing.T) {
	repoID := uuid.New()
	const rawURL = "https://github.com/example/hash-err.git"

	pending1 := makePendingRepo(t, rawURL)
	pending2 := makePendingRepo(t, rawURL)
	cloning1 := makePendingRepo(t, rawURL)
	require.NoError(t, cloning1.UpdateStatus(valueobject.RepositoryStatusCloning))
	cloning2 := makePendingRepo(t, rawURL)
	require.NoError(t, cloning2.UpdateStatus(valueobject.RepositoryStatusCloning))
	processing := makePendingRepo(t, rawURL)
	require.NoError(t, processing.UpdateStatus(valueobject.RepositoryStatusCloning))
	require.NoError(t, processing.UpdateStatus(valueobject.RepositoryStatusProcessing))

	mockRepoRepo := &MockRepositoryRepository{}
	seqRepoFindByID(mockRepoRepo, repoID, pending1, pending2, cloning1, cloning2, processing)
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(processing, nil).Maybe()
	mockRepoRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// GetCommitHash returns an error — the processor must fall back gracefully.
	mockGitClient.On("GetCommitHash", mock.Anything, mock.Anything).
		Return("", errors.New("git rev-parse HEAD: exit status 128"))

	mockParser := &MockCodeParser{}
	mockParser.On("ParseDirectory", mock.Anything, mock.Anything, mock.Anything).
		Return([]outbound.CodeChunk{{ID: "c1", Content: "test", FilePath: "main.go"}}, nil)

	mockZoekt := &MockZoektIndexer{}
	mockZoekt.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
		Return(&outbound.ZoektIndexResult{FileCount: 1, ShardCount: 1}, nil)

	p := buildConcurrentProcessor(
		mockRepoRepo, mockGitClient, mockParser, mockZoekt,
		func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ []outbound.CodeChunk) error { return nil },
	)

	msg := messaging.EnhancedIndexingJobMessage{
		MessageID:     "commit-hash-err-test",
		RepositoryID:  repoID,
		RepositoryURL: rawURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	chunks, concResult, err := p.executeJobPipeline(
		ctx, msg, "/tmp/codechunking-concurrent-tests/workspace-hash-err", newExecution(repoID))

	// The pipeline must NOT abort just because GetCommitHash failed.
	assert.NoError(t, err,
		"executeJobPipeline must not fail when GetCommitHash errors; it must fall back to unknownCommitHash and log a warning")
	assert.NotNil(t, chunks,
		"chunks must be populated — the pipeline must continue after a GetCommitHash error")
	assert.NotNil(t, concResult,
		"ConcurrencyResult must be populated — concurrent indexing must still run after a GetCommitHash error")

	// Verify GetCommitHash was actually called (not short-circuited before the call).
	mockGitClient.AssertCalled(t, "GetCommitHash", mock.Anything, mock.Anything)
	// Verify Zoekt was still called (execution continued past the error).
	mockZoekt.AssertExpectations(t)
}

// TestCommitHashPropagation_RuntimeContract verifies the end-to-end commit hash
// flow: runConcurrentIndexing → ConcurrencyResult → finalizeWithConcurrencyResult
// → persisted repository. The stored hash must not be the "unknown" sentinel.

func TestCommitHashPropagation_RuntimeContract(t *testing.T) {
	repoID := uuid.New()

	indexer := &MockZoektIndexer{}
	indexer.On("Index", mock.Anything, mock.AnythingOfType("*outbound.ZoektRepositoryConfig")).
		Return(&outbound.ZoektIndexResult{FileCount: 3, ShardCount: 1}, nil)

	pIndexing := processorWithZoektAndEmbedFn(indexer, enabledZoektConfig(),
		func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ []outbound.CodeChunk) error { return nil })

	result := pIndexing.runConcurrentIndexing(
		context.Background(),
		"github.com/example/propagation-test",
		"/workspace",
		"feedface12345678",
		makeChunks(5),
		uuid.New(),
		repoID,
	)

	repo := makeProcessingRepo(t, "https://github.com/example/propagation-test.git")

	mockRepoRepo := &MockRepositoryRepository{}
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(repo, nil).Once()

	var storedHash string
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		if h := r.LastCommitHash(); h != nil {
			storedHash = *h
		}
		return true
	})).Return(nil).Once()

	pFinalize := &DefaultJobProcessor{repositoryRepo: mockRepoRepo}
	pFinalize.finalizeWithConcurrencyResult(context.Background(), repoID, &result, 3, 5)

	assert.Equal(t, "feedface12345678", storedHash,
		"commit hash must flow end-to-end from runConcurrentIndexing through ConcurrencyResult into the persisted repository; 'unknown' sentinel must not be used")

	indexer.AssertExpectations(t)
	mockRepoRepo.AssertExpectations(t)
}

// TestFinalizeWithConcurrencyResult_NonPendingEntryState verifies that when the
// Zoekt engine starts in Failed state (e.g. a previous run failed) and Zoekt
// succeeds this time, finalizeWithConcurrencyResult correctly walks the
// Failed → Pending → Indexing → Completed path and persists a consistent record.
func TestFinalizeWithConcurrencyResult_NonPendingEntryState(t *testing.T) {
	repoID := uuid.New()

	repo := makeProcessingRepo(t, "https://github.com/example/retry-zoekt.git")
	// Simulate a prior failed Zoekt run: Pending → Failed (valid direct transition).
	require.NoError(t, repo.UpdateZoektStatus(valueobject.ZoektIndexStatusFailed, 0, nil))

	mockRepoRepo := &MockRepositoryRepository{}
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(repo, nil).Once()

	var finalRepo *entity.Repository
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		finalRepo = r
		return true
	})).Return(nil).Once()

	p := &DefaultJobProcessor{repositoryRepo: mockRepoRepo}

	result := &ConcurrencyResult{
		ZoektResult:     &outbound.ZoektIndexResult{FileCount: 4, ShardCount: 1},
		CommitHash:      "abc123retry",
		ChunksProcessed: 8,
	}

	p.finalizeWithConcurrencyResult(context.Background(), repoID, result, 4, 8)

	require.NotNil(t, finalRepo, "repository must be persisted")
	assert.Equal(t, valueobject.ZoektIndexStatusCompleted, finalRepo.ZoektIndexStatus(),
		"Zoekt must reach Completed even when entry state was Failed")
	assert.Equal(t, valueobject.EmbeddingIndexStatusCompleted, finalRepo.EmbeddingIndexStatus(),
		"embedding must also be Completed (succeeded)")
	assert.Equal(t, valueobject.RepositoryStatusCompleted, finalRepo.Status())

	mockRepoRepo.AssertExpectations(t)
}

// TestFinalizeWithConcurrencyResult_PartialEntryState verifies that when the
// Zoekt engine starts in Partial state and Zoekt succeeds, the function correctly
// walks the Partial → Indexing → Completed path.
func TestFinalizeWithConcurrencyResult_PartialEntryState(t *testing.T) {
	repoID := uuid.New()

	repo := makeProcessingRepo(t, "https://github.com/example/partial-zoekt.git")
	// Simulate a prior partial Zoekt run: Pending → Indexing → Partial.
	require.NoError(t, repo.UpdateZoektStatus(valueobject.ZoektIndexStatusIndexing, 0, nil))
	require.NoError(t, repo.UpdateZoektStatus(valueobject.ZoektIndexStatusPartial, 1, nil))

	mockRepoRepo := &MockRepositoryRepository{}
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(repo, nil).Once()

	var finalRepo *entity.Repository
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		finalRepo = r
		return true
	})).Return(nil).Once()

	p := &DefaultJobProcessor{repositoryRepo: mockRepoRepo}

	result := &ConcurrencyResult{
		ZoektResult:     &outbound.ZoektIndexResult{FileCount: 6, ShardCount: 2},
		CommitHash:      "deadbeefpartial",
		ChunksProcessed: 12,
	}

	p.finalizeWithConcurrencyResult(context.Background(), repoID, result, 6, 12)

	require.NotNil(t, finalRepo, "repository must be persisted")
	assert.Equal(t, valueobject.ZoektIndexStatusCompleted, finalRepo.ZoektIndexStatus(),
		"Zoekt must reach Completed even when entry state was Partial")
	assert.Equal(t, valueobject.EmbeddingIndexStatusCompleted, finalRepo.EmbeddingIndexStatus())
	assert.Equal(t, valueobject.RepositoryStatusCompleted, finalRepo.Status())

	mockRepoRepo.AssertExpectations(t)
}

// TestFinalizeWithConcurrencyResult_AllEngineTransitionsFail verifies that when
// both ConcurrencyResult engines failed (BothFailed), finalizeWithConcurrencyResult
// calls markRepositoryFailed and does NOT persist the repository as Completed.
func TestFinalizeWithConcurrencyResult_AllEngineTransitionsFail(t *testing.T) {
	repoID := uuid.New()

	// makeProcessingRepo provides a valid repo so FindByID in markRepositoryFailed succeeds.
	repoForFinalize := makeProcessingRepo(t, "https://github.com/example/both-engines-fail.git")
	repoForFailed := makeProcessingRepo(t, "https://github.com/example/both-engines-fail.git")

	mockRepoRepo := &MockRepositoryRepository{}
	// BothFailed guard fires before FindByID, so only markRepositoryFailed → updateRepositoryStatus calls it.
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(repoForFailed, nil).Once()
	_ = repoForFinalize // unused; BothFailed short-circuits before the initial FindByID

	var updatedRepo *entity.Repository
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *entity.Repository) bool {
		updatedRepo = r
		return true
	})).Return(nil).Once()

	p := &DefaultJobProcessor{repositoryRepo: mockRepoRepo}

	result := &ConcurrencyResult{
		ZoektErr:     errors.New("zoekt failed"),
		EmbeddingErr: errors.New("embedding failed"),
	}

	p.finalizeWithConcurrencyResult(context.Background(), repoID, result, 0, 0)

	// markRepositoryFailed must have been called — verify via the persisted status.
	require.NotNil(t, updatedRepo, "markRepositoryFailed must trigger an Update call")
	assert.Equal(t, valueobject.RepositoryStatusFailed, updatedRepo.Status(),
		"repository must be marked Failed, not Completed, when both engines fail")

	mockRepoRepo.AssertExpectations(t)
}
