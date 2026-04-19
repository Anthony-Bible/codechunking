package zoekt

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Indexer implements the ZoektIndexer outbound port using zoekt-git-index CLI.
type Indexer struct {
	gitIndexPath string
	indexDir     string
	timeout      time.Duration
}

// IndexerConfig holds configuration for the Zoekt indexer.
type IndexerConfig struct {
	GitIndexPath string        // Path to zoekt-git-index binary
	IndexDir     string        // Shard output directory
	Timeout      time.Duration // Indexing timeout
}

// NewIndexer creates a new Zoekt indexer instance.
func NewIndexer(cfg IndexerConfig) *Indexer {
	return &Indexer{
		gitIndexPath: cfg.GitIndexPath,
		indexDir:     cfg.IndexDir,
		timeout:      cfg.Timeout,
	}
}

// Index creates or updates a Zoekt index for a repository.
func (i *Indexer) Index(ctx context.Context, config *outbound.ZoektRepositoryConfig) (*outbound.ZoektIndexResult, error) {
	startTime := time.Now()

	slogger.Info(ctx, "Starting Zoekt indexing", slogger.Fields{
		"repository":  config.Name,
		"path":        config.Path,
		"commit_hash": config.CommitHash,
		"branch":      config.Branch,
		"index_dir":   config.IndexDir,
	})

	if i.gitIndexPath == "" {
		return nil, errors.New("git_index_path not configured")
	}

	args := i.buildIndexArgs(config)

	effectiveTimeout := config.Timeout
	if effectiveTimeout <= 0 {
		effectiveTimeout = i.timeout
	}

	indexCtx := ctx
	if effectiveTimeout > 0 {
		var cancel context.CancelFunc
		indexCtx, cancel = context.WithTimeout(ctx, effectiveTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(indexCtx, i.gitIndexPath, args...) //nolint:gosec // gitIndexPath is operator-configured, args are derived from validated repository config
	output, err := cmd.CombinedOutput()
	if err != nil {
		slogger.Error(ctx, "Zoekt indexing failed", slogger.Fields{
			"repository": config.Name,
			"error":      err.Error(),
			"output":     string(output),
		})
		return nil, fmt.Errorf("zoekt-git-index failed: %w: %s", err, string(output))
	}

	slogger.Info(ctx, "Zoekt indexing completed", slogger.Fields{
		"repository":  config.Name,
		"duration_ms": time.Since(startTime).Milliseconds(),
		"output":      string(output),
	})

	result := i.parseIndexOutput(string(output))

	// zoekt-git-index runs incrementally by default: if the shard is already up-to-date
	// it exits successfully with no output. Fall back to disk to get actual shard count.
	i.fillMissingShardStats(ctx, config.Name, result)

	return &outbound.ZoektIndexResult{
		FileCount:     result.FileCount,
		ShardCount:    result.ShardCount,
		DocumentCount: result.FileCount,
		BytesIndexed:  result.BytesIndexed,
		Duration:      time.Since(startTime),
		ShardPaths:    result.ShardPaths,
	}, nil
}

// CheckIndexStatus checks the indexing status of a repository.
// The commitHash parameter is reserved for future use (e.g., verifying the indexed version).
func (i *Indexer) CheckIndexStatus(ctx context.Context, repoName, commitHash string) (*outbound.ZoektIndexStatus, error) {
	matches, err := i.findShards(repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to check for shards: %w", err)
	}

	if len(matches) == 0 {
		return &outbound.ZoektIndexStatus{
			Exists: false,
		}, nil
	}

	var totalIndexSize int64
	var latestInfo os.FileInfo
	for _, shardPath := range matches {
		info, err := os.Stat(shardPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat shard %s: %w", shardPath, err)
		}
		totalIndexSize += info.Size()
		if latestInfo == nil || info.ModTime().After(latestInfo.ModTime()) {
			latestInfo = info
		}
	}

	return &outbound.ZoektIndexStatus{
		Exists:     true,
		IndexedAt:  getModTime(latestInfo),
		ShardCount: len(matches),
		IndexSize:  totalIndexSize,
	}, nil
}

// DeleteRepository removes the index for a repository.
func (i *Indexer) DeleteRepository(ctx context.Context, repoName string) error {
	slogger.Info(ctx, "Deleting Zoekt index", slogger.Fields{
		"repository": repoName,
	})

	matches, err := i.findShards(repoName)
	if err != nil {
		return fmt.Errorf("failed to find shards: %w", err)
	}

	if len(matches) == 0 {
		slogger.Debug(ctx, "No shards found to delete", slogger.Fields{
			"repository": repoName,
		})
		return nil
	}

	var deleteErrors []string
	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			deleteErrors = append(deleteErrors, fmt.Sprintf("'%s': %v", match, err))
		}
	}

	if len(deleteErrors) > 0 {
		return fmt.Errorf("failed to delete some shards: %v", deleteErrors)
	}

	slogger.Info(ctx, "Zoekt index deleted", slogger.Fields{
		"repository": repoName,
		"shards":     len(matches),
	})

	return nil
}

// findShards returns all shard file paths for a repository.
// zoekt-git-index encodes the repo name using URL percent-encoding (e.g. "/" → "%2F"),
// so we match both the encoded form and the legacy underscore form.
func (i *Indexer) findShards(repoName string) ([]string, error) {
	encodedName := strings.ReplaceAll(repoName, "/", "%2F")
	pattern := filepath.Join(i.indexDir, encodedName) + "*.zoekt"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return matches, nil
	}
	// Fallback: legacy underscore encoding used by older zoekt versions.
	legacyPattern := filepath.Join(i.indexDir, strings.ReplaceAll(repoName, "/", "_")) + "*.zoekt"
	return filepath.Glob(legacyPattern)
}

// buildIndexArgs constructs CLI arguments for zoekt-git-index.
// The repository name is derived by zoekt-git-index from the git remote URL
// (or the directory name when no remote is configured).
// indexDir falls back to i.indexDir when config.IndexDir is empty, ensuring
// that Index/CheckIndexStatus/DeleteRepository all operate on the same directory.
func (i *Indexer) buildIndexArgs(config *outbound.ZoektRepositoryConfig) []string {
	indexDir := config.IndexDir
	if indexDir == "" {
		indexDir = i.indexDir
	}

	args := []string{
		"-index", indexDir,
	}

	if config.Branch != "" {
		args = append(args, "-branches", config.Branch)
	}

	if config.MaxFileSize > 0 {
		args = append(args, "-file_limit", strconv.FormatInt(config.MaxFileSize, 10))
	}

	args = append(args, config.Path)

	return args
}

// parseIndexOutput extracts statistics from zoekt-git-index output.
// zoekt-git-index writes to stderr lines like:
//
//	2006/01/02 15:04:05 finished shard /path/to/file.zoekt: 12345 index bytes (overhead 2.8), 701 files processed
//
// Each "finished shard" line represents one shard.
func (i *Indexer) parseIndexOutput(output string) *outbound.ZoektIndexResult {
	result := &outbound.ZoektIndexResult{}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "finished shard") {
			continue
		}
		// Extract shard path (ends in .zoekt, appears before the colon after "finished shard")
		shardPath := extractShardPath(line)
		if shardPath != "" {
			result.ShardPaths = append(result.ShardPaths, shardPath)
			result.ShardCount++
		}
		// Extract file count: "..., 701 files processed"
		if idx := strings.Index(line, ", "); idx >= 0 {
			var fileCount int
			if n, _ := fmt.Sscanf(line[idx+2:], "%d files processed", &fileCount); n == 1 {
				result.FileCount += fileCount
			}
		}
		// Extract bytes indexed: after the colon "... .zoekt: 29941290 index bytes ..."
		if idx := strings.Index(line, ".zoekt:"); idx >= 0 {
			var bytesIndexed int64
			if n, _ := fmt.Sscanf(strings.TrimSpace(line[idx+7:]), "%d", &bytesIndexed); n == 1 {
				result.BytesIndexed += bytesIndexed
			}
		}
	}

	return result
}

// extractShardPath extracts the shard path from output line.
func extractShardPath(line string) string {
	fields := strings.Fields(line)
	for _, field := range fields {
		// The field may have a trailing colon (e.g. "file.zoekt:") in zoekt-git-index output.
		trimmed := strings.TrimSuffix(field, ":")
		if strings.HasSuffix(trimmed, ".zoekt") {
			return trimmed
		}
	}
	return ""
}

// fillMissingShardStats populates result from disk when zoekt-git-index produced no output
// (incremental mode skips indexing when the shard is already up-to-date).
func (i *Indexer) fillMissingShardStats(_ context.Context, repoName string, result *outbound.ZoektIndexResult) {
	if result.ShardCount != 0 {
		return
	}
	status, err := i.CheckIndexStatus(context.Background(), repoName, "")
	if err != nil || !status.Exists {
		return
	}
	result.ShardCount = status.ShardCount
	result.BytesIndexed = status.IndexSize
	if len(result.ShardPaths) == 0 {
		if paths, err := i.findShards(repoName); err == nil {
			result.ShardPaths = paths
		}
	}
}

// getModTime returns the modification time of a file.
func getModTime(info os.FileInfo) *time.Time {
	modTime := info.ModTime()
	return &modTime
}
