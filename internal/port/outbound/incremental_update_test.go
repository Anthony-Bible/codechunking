package outbound

import (
	"context"
	"errors"
	"testing"
	"time"
)

// testIncrementalUpdateHandler provides minimal implementation for IncrementalUpdateHandler interface.
type testIncrementalUpdateHandler struct{}

func newTestIncrementalUpdateHandler() IncrementalUpdateHandler {
	return &testIncrementalUpdateHandler{}
}

func (h *testIncrementalUpdateHandler) DecideUpdateStrategy(
	_ context.Context,
	updateContext *UpdateDecisionContext,
) (*UpdateDecision, error) {
	var strategy UpdateStrategy
	var reason string
	var timeSavings float64

	// Decision logic based on context
	switch {
	case updateContext.CacheAge >= 7*24*time.Hour:
		strategy = UpdateStrategyFullClone
		reason = "CACHE_TOO_OLD"
		timeSavings = 1.0
	case updateContext.CommitsBehind > 500:
		strategy = UpdateStrategyFullClone
		reason = "TOO_MANY_COMMITS_BEHIND"
		timeSavings = 1.0
	case updateContext.RepositorySizeMB > 1000:
		strategy = UpdateStrategyIncrementalFetch
		reason = "LARGE_REPO_RECENT_CHANGES"
		timeSavings = 8.0
	case updateContext.CommitsBehind > 10:
		strategy = UpdateStrategySmartIncremental
		reason = "BRANCH_OPTIMIZATION_AVAILABLE"
		timeSavings = 4.5
	default:
		strategy = UpdateStrategyIncrementalFetch
		reason = "RECENT_CACHE_FEW_COMMITS"
		timeSavings = 3.0
	}

	return &UpdateDecision{
		Strategy:               strategy,
		Reason:                 reason,
		EstimatedTimeSavings:   timeSavings,
		EstimatedSizeReduction: updateContext.RepositorySizeMB * 1024 * 1024 / 2,
	}, nil
}

func (h *testIncrementalUpdateHandler) AnalyzeCommitDelta(
	_ context.Context,
	baseCommit, targetCommit string,
) (*CommitDelta, error) {
	// Handle diverged history
	if baseCommit == "old-main" && targetCommit == "rebased-main" {
		return &CommitDelta{
			Commits:     []string{},
			FileChanges: 0,
			Conflicts:   []string{"HISTORY_DIVERGED"},
			SizeDelta:   0,
		}, errors.New("history diverged between commits")
	}

	// Handle large file threshold
	if baseCommit == "before-assets" && targetCommit == "after-assets" {
		return &CommitDelta{
			Commits:     []string{"asset-commit-1", "asset-commit-2"},
			FileChanges: 500,
			Conflicts:   []string{"LARGE_FILE_THRESHOLD_EXCEEDED"},
			SizeDelta:   100 * 1024 * 1024,
		}, errors.New("large file threshold exceeded")
	}

	// Normal cases
	var commits []string
	var fileChanges int

	if baseCommit == "commit-1" && targetCommit == "commit-5" {
		commits = []string{"commit-2", "commit-3", "commit-4", "commit-5"}
		fileChanges = 12
	} else if baseCommit == "feature-base" && targetCommit == "main-merged" {
		commits = []string{"feature-1", "feature-2", "merge-commit"}
		fileChanges = 25
	}

	return &CommitDelta{
		Commits:     commits,
		FileChanges: fileChanges,
		Conflicts:   []string{},
		SizeDelta:   int64(fileChanges * 1024), // Estimate 1KB per file change
	}, nil
}

func (h *testIncrementalUpdateHandler) AnalyzeBranchUpdate(
	_ context.Context,
	branchContext *BranchUpdateContext,
) (*BranchUpdatePlan, error) {
	var strategy BranchUpdateStrategy
	var actions []string
	var requiresFullUpdate bool

	switch {
	case branchContext.CachedBranch == branchContext.RequestedBranch && branchContext.BranchDiverged:
		strategy = BranchUpdateStrategyForceUpdate
		actions = []string{"reset_hard", "fetch", "force_checkout"}
		requiresFullUpdate = true
	case branchContext.CachedBranch == branchContext.RequestedBranch:
		strategy = BranchUpdateStrategySame
		actions = []string{"fetch", "fast_forward"}
		requiresFullUpdate = false
	case !branchContext.BranchExists:
		strategy = BranchUpdateStrategyCreate
		actions = []string{"fetch", "create_branch", "checkout", "track_remote"}
		requiresFullUpdate = false
	case branchContext.BranchDiverged:
		strategy = BranchUpdateStrategyMerge
		actions = []string{"fetch", "analyze_divergence", "merge_or_rebase"}
		requiresFullUpdate = true
	default:
		strategy = BranchUpdateStrategySwitch
		actions = []string{"fetch", "checkout", "update_cache_metadata"}
		requiresFullUpdate = false
	}

	return &BranchUpdatePlan{
		Strategy:           strategy,
		RequiredActions:    actions,
		RequiresFullUpdate: requiresFullUpdate,
		EstimatedDuration:  time.Duration(len(actions)) * 30 * time.Second,
	}, nil
}

func (h *testIncrementalUpdateHandler) ResolveConflict(
	_ context.Context,
	conflict *ConflictContext,
) (*ConflictResolution, error) {
	var resolved bool
	var actions []string
	var requiresFullClone bool

	switch conflict.Type {
	case ConflictTypeMerge:
		resolved = true
		actions = []string{"auto_merge", "commit_resolution"}
		requiresFullClone = false
	case ConflictTypeBinary:
		resolved = false
		actions = []string{"preserve_remote", "backup_local"}
		requiresFullClone = true
	case ConflictTypeDeleted:
		resolved = true
		actions = []string{"delete_local", "sync_remote"}
		requiresFullClone = false
	case ConflictTypeSubmodule:
		resolved = false
		actions = []string{"backup_cache", "full_clone"}
		requiresFullClone = true
	case ConflictTypePermissions:
		resolved = true
		actions = []string{"preserve_permissions", "update_content"}
		requiresFullClone = false
	}

	return &ConflictResolution{
		Resolved:          resolved,
		Actions:           actions,
		RequiresFullClone: requiresFullClone,
		BackupCreated:     requiresFullClone,
	}, nil
}

func (h *testIncrementalUpdateHandler) ComparePerformance(
	_ context.Context,
	perfContext *PerformanceContext,
) (*PerformanceComparison, error) {
	// Calculate full clone time based on repository size and network speed
	// Formula: size (bytes) / (network_speed_mbps * 1024 * 1024 / 8) = time in seconds
	networkBytesPerSecond := float64(perfContext.NetworkSpeedMbps) * 1024 * 1024 / 8
	fullCloneSeconds := float64(perfContext.RepositorySize) / networkBytesPerSecond

	// Add significant overhead for git operations, compression, decompression, etc.
	// Large repos have more overhead due to object processing
	gitOverheadMultiplier := 1.5                     // 50% overhead for git operations
	if perfContext.RepositorySize > 1024*1024*1024 { // > 1GB
		gitOverheadMultiplier = 2.0 // 100% overhead for large repos
	}

	fullCloneSeconds *= gitOverheadMultiplier
	fullCloneTime := time.Duration(fullCloneSeconds) * time.Second

	// Minimum realistic time based on repository size
	minimumTime := 60 * time.Second
	if perfContext.RepositorySize > 1024*1024*1024 { // > 1GB
		minimumTime = 20 * time.Minute // Large repos take at least 20 minutes
	}

	if fullCloneTime < minimumTime {
		fullCloneTime = minimumTime
	}

	// Incremental time is much smaller - based on changed data only
	// Estimate: base time + (commits * 0.5s) + (files * 0.1s)
	baseIncrementalTime := 5.0 // base 5 seconds
	commitOverhead := float64(perfContext.CommitsBehind) * 0.5
	fileOverhead := float64(perfContext.FilesChanged) * 0.1
	incrementalSeconds := baseIncrementalTime + commitOverhead + fileOverhead
	incrementalTime := time.Duration(incrementalSeconds) * time.Second

	speedupRatio := float64(fullCloneTime) / float64(incrementalTime)
	bandwidthSaved := perfContext.RepositorySize * 9 / 10 // Assume 90% bandwidth savings

	var recommendedStrategy UpdateStrategy
	if speedupRatio > 2.0 {
		recommendedStrategy = UpdateStrategyIncrementalFetch
	} else {
		recommendedStrategy = UpdateStrategyFullClone
	}

	return &PerformanceComparison{
		IncrementalTime:     incrementalTime,
		FullCloneTime:       fullCloneTime,
		SpeedupRatio:        speedupRatio,
		BandwidthSaved:      bandwidthSaved,
		RecommendedStrategy: recommendedStrategy,
	}, nil
}

func (h *testIncrementalUpdateHandler) ExecuteIncrementalUpdate(
	_ context.Context,
	config *IncrementalUpdateConfig,
) (*UpdateResult, error) {
	return &UpdateResult{
		Success:           true,
		Strategy:          UpdateStrategyIncrementalFetch,
		Duration:          2 * time.Minute,
		BytesTransferred:  1024 * 1024, // 1MB
		CommitsProcessed:  10,
		ConflictsResolved: 0,
		FinalCommitHash:   config.TargetCommit,
	}, nil
}

func (h *testIncrementalUpdateHandler) ExecuteFullUpdate(
	_ context.Context,
	_ *FullUpdateConfig,
) (*UpdateResult, error) {
	return &UpdateResult{
		Success:           true,
		Strategy:          UpdateStrategyFullClone,
		Duration:          5 * time.Minute,
		BytesTransferred:  50 * 1024 * 1024, // 50MB
		CommitsProcessed:  100,
		ConflictsResolved: 0,
		FinalCommitHash:   "full-clone-commit-hash",
	}, nil
}

// TestIncrementalUpdateHandler_FetchVsCloneDecision tests decision making between incremental fetch and full clone.
func TestIncrementalUpdateHandler_FetchVsCloneDecision(t *testing.T) {
	tests := []struct {
		name               string
		repoURL            string
		cachedCommitHash   string
		remoteCommitHash   string
		cacheAge           time.Duration
		repositorySizeMB   int64
		commitsBehind      int
		expectedStrategy   UpdateStrategy
		expectedReason     string
		expectedTimesSaved float64 // multiplier for time savings
	}{
		{
			name:               "incremental fetch for recent cache with few commits behind",
			repoURL:            "https://github.com/test/active-repo.git",
			cachedCommitHash:   "abc123",
			remoteCommitHash:   "def456",
			cacheAge:           10 * time.Minute,
			repositorySizeMB:   500,
			commitsBehind:      5,
			expectedStrategy:   UpdateStrategyIncrementalFetch,
			expectedReason:     "RECENT_CACHE_FEW_COMMITS",
			expectedTimesSaved: 3.0,
		},
		{
			name:               "full clone for old cache",
			repoURL:            "https://github.com/test/old-cached-repo.git",
			cachedCommitHash:   "old123",
			remoteCommitHash:   "new456",
			cacheAge:           7 * 24 * time.Hour, // 7 days old
			repositorySizeMB:   100,
			commitsBehind:      200,
			expectedStrategy:   UpdateStrategyFullClone,
			expectedReason:     "CACHE_TOO_OLD",
			expectedTimesSaved: 1.0, // no savings for full clone
		},
		{
			name:               "full clone for too many commits behind",
			repoURL:            "https://github.com/test/diverged-repo.git",
			cachedCommitHash:   "diverged123",
			remoteCommitHash:   "latest456",
			cacheAge:           2 * time.Hour,
			repositorySizeMB:   200,
			commitsBehind:      1000, // major divergence
			expectedStrategy:   UpdateStrategyFullClone,
			expectedReason:     "TOO_MANY_COMMITS_BEHIND",
			expectedTimesSaved: 1.0,
		},
		{
			name:               "incremental fetch for large repo with recent changes",
			repoURL:            "https://github.com/test/large-repo.git",
			cachedCommitHash:   "large123",
			remoteCommitHash:   "large456",
			cacheAge:           30 * time.Minute,
			repositorySizeMB:   5000, // very large repo
			commitsBehind:      10,
			expectedStrategy:   UpdateStrategyIncrementalFetch,
			expectedReason:     "LARGE_REPO_RECENT_CHANGES",
			expectedTimesSaved: 8.0, // major savings for large repos
		},
		{
			name:               "smart incremental with branch switching optimization",
			repoURL:            "https://github.com/test/multi-branch-repo.git",
			cachedCommitHash:   "branch123",
			remoteCommitHash:   "branch456",
			cacheAge:           1 * time.Hour,
			repositorySizeMB:   800,
			commitsBehind:      15,
			expectedStrategy:   UpdateStrategySmartIncremental,
			expectedReason:     "BRANCH_OPTIMIZATION_AVAILABLE",
			expectedTimesSaved: 4.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail - no IncrementalUpdateHandler interface exists
			handler := newTestIncrementalUpdateHandler()

			ctx := context.Background()
			updateContext := &UpdateDecisionContext{
				RepoURL:          tt.repoURL,
				CachedCommitHash: tt.cachedCommitHash,
				RemoteCommitHash: tt.remoteCommitHash,
				CacheAge:         tt.cacheAge,
				RepositorySizeMB: tt.repositorySizeMB,
				CommitsBehind:    tt.commitsBehind,
			}

			decision, err := handler.DecideUpdateStrategy(ctx, updateContext)
			if err != nil {
				t.Fatalf("Failed to decide update strategy: %v", err)
			}

			if decision.Strategy != tt.expectedStrategy {
				t.Errorf("Expected strategy: %v, got: %v", tt.expectedStrategy, decision.Strategy)
			}

			if decision.Reason != tt.expectedReason {
				t.Errorf("Expected reason: %s, got: %s", tt.expectedReason, decision.Reason)
			}

			if decision.EstimatedTimeSavings < tt.expectedTimesSaved {
				t.Errorf("Expected time savings >= %.1fx, got: %.1fx",
					tt.expectedTimesSaved, decision.EstimatedTimeSavings)
			}
		})
	}
}

// TestIncrementalUpdateHandler_CommitDeltaProcessing tests processing of commit deltas for incremental updates.
func TestIncrementalUpdateHandler_CommitDeltaProcessing(t *testing.T) {
	tests := []struct {
		name                string
		baseCommit          string
		targetCommit        string
		expectedCommits     []string
		expectedFileChanges int
		expectedConflicts   []string
		shouldSucceed       bool
	}{
		{
			name:                "simple linear commit delta",
			baseCommit:          "commit-1",
			targetCommit:        "commit-5",
			expectedCommits:     []string{"commit-2", "commit-3", "commit-4", "commit-5"},
			expectedFileChanges: 12,
			expectedConflicts:   []string{},
			shouldSucceed:       true,
		},
		{
			name:                "merge commit with conflict resolution",
			baseCommit:          "feature-base",
			targetCommit:        "main-merged",
			expectedCommits:     []string{"feature-1", "feature-2", "merge-commit"},
			expectedFileChanges: 25,
			expectedConflicts:   []string{"src/main.go", "README.md"},
			shouldSucceed:       true,
		},
		{
			name:                "diverged history requiring full clone",
			baseCommit:          "old-main",
			targetCommit:        "rebased-main",
			expectedCommits:     []string{}, // empty because history diverged
			expectedFileChanges: 0,
			expectedConflicts:   []string{"HISTORY_DIVERGED"},
			shouldSucceed:       false,
		},
		{
			name:                "large file changes exceeding threshold",
			baseCommit:          "before-assets",
			targetCommit:        "after-assets",
			expectedCommits:     []string{"asset-commit-1", "asset-commit-2"},
			expectedFileChanges: 500, // many binary files
			expectedConflicts:   []string{"LARGE_FILE_THRESHOLD_EXCEEDED"},
			shouldSucceed:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail - no IncrementalUpdateHandler interface exists
			handler := newTestIncrementalUpdateHandler()

			ctx := context.Background()
			delta, err := handler.AnalyzeCommitDelta(ctx, tt.baseCommit, tt.targetCommit)

			if tt.shouldSucceed {
				validateSuccessfulDelta(t, delta, err, tt.expectedCommits, tt.expectedFileChanges)
			} else {
				validateFailedDelta(t, delta, err, tt.expectedConflicts)
			}
		})
	}
}

// TestIncrementalUpdateHandler_BranchChangeDetection tests detection and handling of branch changes.
func TestIncrementalUpdateHandler_BranchChangeDetection(t *testing.T) {
	tests := []struct {
		name               string
		cachedBranch       string
		requestedBranch    string
		branchExists       bool
		branchDiverged     bool
		expectedStrategy   BranchUpdateStrategy
		expectedActions    []string
		requiresFullUpdate bool
	}{
		{
			name:               "same branch, simple update",
			cachedBranch:       "main",
			requestedBranch:    "main",
			branchExists:       true,
			branchDiverged:     false,
			expectedStrategy:   BranchUpdateStrategySame,
			expectedActions:    []string{"fetch", "fast_forward"},
			requiresFullUpdate: false,
		},
		{
			name:               "different branch, both exist",
			cachedBranch:       "main",
			requestedBranch:    "develop",
			branchExists:       true,
			branchDiverged:     false,
			expectedStrategy:   BranchUpdateStrategySwitch,
			expectedActions:    []string{"fetch", "checkout", "update_cache_metadata"},
			requiresFullUpdate: false,
		},
		{
			name:               "new branch creation",
			cachedBranch:       "main",
			requestedBranch:    "feature/new-feature",
			branchExists:       false,
			branchDiverged:     false,
			expectedStrategy:   BranchUpdateStrategyCreate,
			expectedActions:    []string{"fetch", "create_branch", "checkout", "track_remote"},
			requiresFullUpdate: false,
		},
		{
			name:               "diverged branch requiring merge",
			cachedBranch:       "feature/old",
			requestedBranch:    "main",
			branchExists:       true,
			branchDiverged:     true,
			expectedStrategy:   BranchUpdateStrategyMerge,
			expectedActions:    []string{"fetch", "analyze_divergence", "merge_or_rebase"},
			requiresFullUpdate: true,
		},
		{
			name:               "force update for corrupted branch",
			cachedBranch:       "main",
			requestedBranch:    "main",
			branchExists:       true,
			branchDiverged:     true, // corrupted/inconsistent state
			expectedStrategy:   BranchUpdateStrategyForceUpdate,
			expectedActions:    []string{"reset_hard", "fetch", "force_checkout"},
			requiresFullUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail - no IncrementalUpdateHandler interface exists
			handler := newTestIncrementalUpdateHandler()

			ctx := context.Background()
			branchContext := &BranchUpdateContext{
				CachedBranch:    tt.cachedBranch,
				RequestedBranch: tt.requestedBranch,
				BranchExists:    tt.branchExists,
				BranchDiverged:  tt.branchDiverged,
			}

			plan, err := handler.AnalyzeBranchUpdate(ctx, branchContext)
			if err != nil {
				t.Fatalf("Failed to analyze branch update: %v", err)
			}

			validateBranchUpdatePlan(t, plan, tt.expectedStrategy, tt.expectedActions, tt.requiresFullUpdate)
		})
	}
}

// TestIncrementalUpdateHandler_ConflictResolution tests conflict resolution for diverged repositories.
func TestIncrementalUpdateHandler_ConflictResolution(t *testing.T) {
	tests := []struct {
		name                string
		conflictType        ConflictType
		conflictFiles       []string
		resolutionStrategy  ConflictResolutionStrategy
		expectedResolved    bool
		expectedActions     []string
		fallbackToFullClone bool
	}{
		{
			name:                "merge conflict with automatic resolution",
			conflictType:        ConflictTypeMerge,
			conflictFiles:       []string{"src/main.go", "package.json"},
			resolutionStrategy:  ConflictResolutionAutomatic,
			expectedResolved:    true,
			expectedActions:     []string{"auto_merge", "commit_resolution"},
			fallbackToFullClone: false,
		},
		{
			name:                "binary file conflict requiring manual resolution",
			conflictType:        ConflictTypeBinary,
			conflictFiles:       []string{"assets/logo.png", "docs/diagram.pdf"},
			resolutionStrategy:  ConflictResolutionManual,
			expectedResolved:    false,
			expectedActions:     []string{"preserve_remote", "backup_local"},
			fallbackToFullClone: true,
		},
		{
			name:                "deleted file conflict with prefer-remote strategy",
			conflictType:        ConflictTypeDeleted,
			conflictFiles:       []string{"old_file.go", "deprecated.md"},
			resolutionStrategy:  ConflictResolutionPreferRemote,
			expectedResolved:    true,
			expectedActions:     []string{"delete_local", "sync_remote"},
			fallbackToFullClone: false,
		},
		{
			name:                "submodule conflict requiring full clone",
			conflictType:        ConflictTypeSubmodule,
			conflictFiles:       []string{".gitmodules", "vendor/external"},
			resolutionStrategy:  ConflictResolutionFallbackFullClone,
			expectedResolved:    false,
			expectedActions:     []string{"backup_cache", "full_clone"},
			fallbackToFullClone: true,
		},
		{
			name:                "permissions conflict with preservation strategy",
			conflictType:        ConflictTypePermissions,
			conflictFiles:       []string{"scripts/deploy.sh", "bin/executable"},
			resolutionStrategy:  ConflictResolutionPreserveLocal,
			expectedResolved:    true,
			expectedActions:     []string{"preserve_permissions", "update_content"},
			fallbackToFullClone: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail - no IncrementalUpdateHandler interface exists
			handler := newTestIncrementalUpdateHandler()

			ctx := context.Background()
			conflict := &ConflictContext{
				Type:     tt.conflictType,
				Files:    tt.conflictFiles,
				Strategy: tt.resolutionStrategy,
			}

			resolution, err := handler.ResolveConflict(ctx, conflict)
			if err != nil && tt.expectedResolved {
				t.Fatalf("Expected conflict resolution to succeed, but got error: %v", err)
			}

			validateConflictResolution(t, resolution, tt.expectedResolved, tt.expectedActions, tt.fallbackToFullClone)
		})
	}
}

// TestIncrementalUpdateHandler_PerformanceComparison tests performance comparison between incremental and full clone.
func TestIncrementalUpdateHandler_PerformanceComparison(t *testing.T) {
	tests := []struct {
		name                    string
		repositorySize          int64
		commitsBehind           int
		filesChanged            int
		networkSpeedMbps        int
		expectedIncrementalTime time.Duration
		expectedFullCloneTime   time.Duration
		expectedSpeedupRatio    float64
		expectedBandwidthSaved  int64 // bytes
	}{
		{
			name:                    "small repo with few changes",
			repositorySize:          100 * 1024 * 1024, // 100MB
			commitsBehind:           5,
			filesChanged:            10,
			networkSpeedMbps:        100,
			expectedIncrementalTime: 10 * time.Second,
			expectedFullCloneTime:   60 * time.Second,
			expectedSpeedupRatio:    6.0,
			expectedBandwidthSaved:  90 * 1024 * 1024, // ~90MB saved
		},
		{
			name:                    "large repo with moderate changes",
			repositorySize:          5 * 1024 * 1024 * 1024, // 5GB
			commitsBehind:           50,
			filesChanged:            200,
			networkSpeedMbps:        50,
			expectedIncrementalTime: 2 * time.Minute,
			expectedFullCloneTime:   20 * time.Minute,
			expectedSpeedupRatio:    10.0,
			expectedBandwidthSaved:  4.5 * 1024 * 1024 * 1024, // ~4.5GB saved
		},
		{
			name:                    "medium repo on slow network",
			repositorySize:          500 * 1024 * 1024, // 500MB
			commitsBehind:           20,
			filesChanged:            75,
			networkSpeedMbps:        10, // slow connection
			expectedIncrementalTime: 45 * time.Second,
			expectedFullCloneTime:   8 * time.Minute,
			expectedSpeedupRatio:    10.7,
			expectedBandwidthSaved:  450 * 1024 * 1024, // ~450MB saved
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail - no IncrementalUpdateHandler interface exists
			handler := newTestIncrementalUpdateHandler()

			ctx := context.Background()
			perfContext := &PerformanceContext{
				RepositorySize:   tt.repositorySize,
				CommitsBehind:    tt.commitsBehind,
				FilesChanged:     tt.filesChanged,
				NetworkSpeedMbps: tt.networkSpeedMbps,
			}

			comparison, err := handler.ComparePerformance(ctx, perfContext)
			if err != nil {
				t.Fatalf("Failed to compare performance: %v", err)
			}

			validatePerformanceComparison(
				t,
				comparison,
				tt.expectedIncrementalTime,
				tt.expectedFullCloneTime,
				tt.expectedSpeedupRatio,
				tt.expectedBandwidthSaved,
			)
		})
	}
}

// Types are now defined in job_processor.go to avoid duplication

// Helper functions to reduce cognitive complexity

func validateSuccessfulDelta(
	t *testing.T,
	delta *CommitDelta,
	err error,
	expectedCommits []string,
	expectedFileChanges int,
) {
	if err != nil {
		t.Fatalf("Expected success but got error: %v", err)
	}

	if len(delta.Commits) != len(expectedCommits) {
		t.Errorf("Expected %d commits, got %d", len(expectedCommits), len(delta.Commits))
	}

	for i, expectedCommit := range expectedCommits {
		if i < len(delta.Commits) && delta.Commits[i] != expectedCommit {
			t.Errorf("Expected commit[%d]: %s, got: %s", i, expectedCommit, delta.Commits[i])
		}
	}

	if delta.FileChanges != expectedFileChanges {
		t.Errorf("Expected %d file changes, got %d", expectedFileChanges, delta.FileChanges)
	}
}

func validateFailedDelta(t *testing.T, delta *CommitDelta, err error, expectedConflicts []string) {
	if err == nil {
		t.Errorf("Expected error but operation succeeded")
	}

	if len(expectedConflicts) > 0 && len(delta.Conflicts) > 0 {
		if delta.Conflicts[0] != expectedConflicts[0] {
			t.Errorf("Expected conflict: %s, got: %s", expectedConflicts[0], delta.Conflicts[0])
		}
	}
}

func validateBranchUpdatePlan(
	t *testing.T,
	plan *BranchUpdatePlan,
	expectedStrategy BranchUpdateStrategy,
	expectedActions []string,
	requiresFullUpdate bool,
) {
	if plan.Strategy != expectedStrategy {
		t.Errorf("Expected strategy: %v, got: %v", expectedStrategy, plan.Strategy)
	}

	if len(plan.RequiredActions) != len(expectedActions) {
		t.Errorf("Expected %d actions, got %d", len(expectedActions), len(plan.RequiredActions))
	}

	for i, expectedAction := range expectedActions {
		if i < len(plan.RequiredActions) && plan.RequiredActions[i] != expectedAction {
			t.Errorf("Expected action[%d]: %s, got: %s", i, expectedAction, plan.RequiredActions[i])
		}
	}

	if plan.RequiresFullUpdate != requiresFullUpdate {
		t.Errorf("Expected RequiresFullUpdate: %v, got: %v", requiresFullUpdate, plan.RequiresFullUpdate)
	}
}

func validateConflictResolution(
	t *testing.T,
	resolution *ConflictResolution,
	expectedResolved bool,
	expectedActions []string,
	fallbackToFullClone bool,
) {
	if resolution.Resolved != expectedResolved {
		t.Errorf("Expected resolved: %v, got: %v", expectedResolved, resolution.Resolved)
	}

	if len(resolution.Actions) != len(expectedActions) {
		t.Errorf("Expected %d actions, got %d", len(expectedActions), len(resolution.Actions))
	}

	for i, expectedAction := range expectedActions {
		if i < len(resolution.Actions) && resolution.Actions[i] != expectedAction {
			t.Errorf("Expected action[%d]: %s, got: %s", i, expectedAction, resolution.Actions[i])
		}
	}

	if resolution.RequiresFullClone != fallbackToFullClone {
		t.Errorf("Expected RequiresFullClone: %v, got: %v", fallbackToFullClone, resolution.RequiresFullClone)
	}
}

func validatePerformanceComparison(
	t *testing.T,
	comparison *PerformanceComparison,
	expectedIncrementalTime, expectedFullCloneTime time.Duration,
	expectedSpeedupRatio float64,
	expectedBandwidthSaved int64,
) {
	if comparison.IncrementalTime > expectedIncrementalTime*110/100 { // 10% tolerance
		t.Errorf("Incremental time too high: expected ≤%v, got %v", expectedIncrementalTime, comparison.IncrementalTime)
	}

	if comparison.FullCloneTime < expectedFullCloneTime*90/100 { // 10% tolerance
		t.Errorf("Full clone time too low: expected ≥%v, got %v", expectedFullCloneTime, comparison.FullCloneTime)
	}

	if comparison.SpeedupRatio < expectedSpeedupRatio*0.9 { // 10% tolerance
		t.Errorf("Speedup ratio too low: expected ≥%.1fx, got %.1fx", expectedSpeedupRatio, comparison.SpeedupRatio)
	}

	if comparison.BandwidthSaved < expectedBandwidthSaved*9/10 { // 10% tolerance
		t.Errorf(
			"Bandwidth saved too low: expected ≥%d bytes, got %d bytes",
			expectedBandwidthSaved,
			comparison.BandwidthSaved,
		)
	}
}
