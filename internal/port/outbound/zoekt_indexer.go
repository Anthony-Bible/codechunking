package outbound

import (
	"context"
	"time"
)

// ZoektIndexer defines the outbound port for Zoekt index management.
type ZoektIndexer interface {
	// Index creates or updates a Zoekt index for a repository.
	Index(ctx context.Context, config *ZoektRepositoryConfig) (*ZoektIndexResult, error)

	// CheckIndexStatus checks the indexing status of a repository.
	CheckIndexStatus(ctx context.Context, repoName, commitHash string) (*ZoektIndexStatus, error)

	// DeleteRepository removes the index for a repository.
	DeleteRepository(ctx context.Context, repoName string) error
}

// ZoektRepositoryConfig contains configuration for indexing a repository.
type ZoektRepositoryConfig struct {
	// Name is the repository name (e.g., github.com/user/repo).
	Name string `json:"name,omitempty"`

	// Path is the local filesystem path to the repository clone.
	Path string `json:"path,omitempty"`

	// CommitHash is the commit hash to index.
	CommitHash string `json:"commit_hash,omitempty"`

	// Branch is the branch name.
	Branch string `json:"branch,omitempty"`

	// IndexDir is the directory where Zoekt shards will be written.
	IndexDir string `json:"index_dir,omitempty"`

	// Priority controls indexing priority (higher = more important).
	Priority float64 `json:"priority,omitempty"`

	// Languages are the languages to index (empty = all).
	Languages []string `json:"languages,omitempty"`

	// MaxFileSize is the maximum file size to index in bytes.
	MaxFileSize int64 `json:"max_file_size,omitempty"`

	// IncludeTests controls whether test files are included.
	IncludeTests bool `json:"include_tests,omitempty"`

	// IncludeVendor controls whether vendor directories are included.
	IncludeVendor bool `json:"include_vendor,omitempty"`

	// Timeout for the indexing operation.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// ZoektIndexResult contains results from a Zoekt indexing operation.
type ZoektIndexResult struct {
	// FileCount is the number of files indexed.
	FileCount int `json:"file_count,omitempty"`

	// ShardCount is the number of shards created.
	ShardCount int `json:"shard_count,omitempty"`

	// DocumentCount is the number of documents indexed.
	DocumentCount int `json:"document_count,omitempty"`

	// BytesIndexed is the number of bytes indexed.
	BytesIndexed int64 `json:"bytes_indexed,omitempty"`

	// Duration is the time taken to index.
	Duration time.Duration `json:"duration,omitempty"`

	// ShardPaths are the paths to created shards.
	ShardPaths []string `json:"shard_paths,omitempty"`
}

// ZoektIndexStatus represents the indexing status of a repository.
type ZoektIndexStatus struct {
	// Exists indicates if shards exist for the repository.
	Exists bool `json:"exists,omitempty"`

	// IndexedAt is when the repository was indexed.
	IndexedAt *time.Time `json:"indexed_at,omitempty"`

	// FileCount is the number of files in the index.
	FileCount int `json:"file_count,omitempty"`

	// ShardCount is the number of shards.
	ShardCount int `json:"shard_count,omitempty"`

	// CommitHash is the commit hash at time of indexing.
	CommitHash string `json:"commit_hash,omitempty"`

	// Branch is the indexed branch.
	Branch string `json:"branch,omitempty"`

	// IndexSize is the size of the index in bytes.
	IndexSize int64 `json:"index_size,omitempty"`
}
