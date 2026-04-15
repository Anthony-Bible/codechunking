package outbound

import (
	"context"
	"time"
)

// ZoektSearcher defines the outbound port for Zoekt full-text search.
type ZoektSearcher interface {
	// Search performs a full-text search query against Zoekt indexes.
	Search(ctx context.Context, query string, opts ZoektSearchOptions) (*ZoektSearchResult, error)

	// List lists indexed repositories matching a query.
	List(ctx context.Context, query string, opts ZoektListOptions) (*ZoektListResult, error)

	// CheckHealth checks if the Zoekt webserver is healthy and accessible.
	CheckHealth(ctx context.Context) (*ZoektHealthStatus, error)
}

// ZoektSearchOptions configures Zoekt search behavior.
type ZoektSearchOptions struct {
	// MaxTotalResults limits the total number of file matches returned.
	MaxTotalResults int `json:"max_total_results,omitempty"`

	// MaxMatchPerFile limits matches per file.
	MaxMatchPerFile int `json:"max_match_per_file,omitempty"`

	// ContextLines is the number of lines surrounding matches.
	ContextLines int `json:"context_lines,omitempty"`

	// Timeout for the search request.
	Timeout time.Duration `json:"timeout,omitempty"`

	// Repositories limits search to specific repositories.
	Repos []string `json:"repos,omitempty"`

	// ExcludeRepos excludes specific repositories from search.
	Exclude []string `json:"exclude,omitempty"`

	// Branch limits search to a specific branch.
	Branch string `json:"branch,omitempty"`

	// Languages limits search to specific languages.
	Lang string `json:"lang,omitempty"`

	// ChunkMatches enables chunk-based matching syntax.
	ChunkMatches bool `json:"chunk_matches,omitempty"`
}

// ZoektSearchResult contains search results from Zoekt.
type ZoektSearchResult struct {
	// FileMatches contains files with matching content.
	FileMatches []ZoektFileMatch `json:"file_matches,omitempty"`

	// TotalCount is the total number of matches across all files.
	TotalCount int `json:"total_count,omitempty"`

	// Duration is the time taken to execute the search.
	Duration time.Duration `json:"duration,omitempty"`

	// Stats contains search statistics.
	Stats ZoektSearchStats `json:"stats,omitempty"`
}

// ZoektFileMatch represents a file containing search matches.
type ZoektFileMatch struct {
	// Repository is the repository containing the file.
	Repository string `json:"repository,omitempty"`

	// FileName is the relative path to the file.
	FileName string `json:"file_name,omitempty"`

	// Branch is the branch the file was indexed from.
	Branch string `json:"branch,omitempty"`

	// CommitHash is the commit hash at indexing time.
	CommitHash string `json:"commit_hash,omitempty"`

	// Language is the detected programming language.
	Language string `json:"language,omitempty"`

	// LineMatches contains line-level matches within the file.
	LineMatches []ZoektLineMatch `json:"line_matches,omitempty"`

	// ChunkMatches contains chunk-level matches if enabled.
	ChunkMatches []ZoektChunkMatch `json:"chunk_matches,omitempty"`

	// Rank indicates ranking in results.
	Rank int `json:"rank,omitempty"`

	// Score is the match score.
	Score float64 `json:"score,omitempty"`
}

// ZoektLineMatch represents a single line matching the query.
type ZoektLineMatch struct {
	// LineNumber is the 1-based line number.
	LineNumber int `json:"line_number,omitempty"`

	// LineContent is the full line text.
	LineContent string `json:"line_content,omitempty"`

	// Before is context before the match.
	Before []string `json:"before,omitempty"`

	// After is context after the match.
	After []string `json:"after,omitempty"`

	// Score for this line match.
	Score float64 `json:"score,omitempty"`
}

// ZoektChunkMatch represents a chunk-level match.
type ZoektChunkMatch struct {
	// ChunkID is the identifier for the chunk.
	ChunkID string `json:"chunk_id,omitempty"`

	// ChunkType is the type of code chunk (e.g., function, class).
	ChunkType string `json:"chunk_type,omitempty"`

	// Context is the full chunk content.
	Context string `json:"context,omitempty"`

	// StartLine is the starting line number of the chunk.
	StartLine int `json:"start_line,omitempty"`

	// EndLine is the ending line number of the chunk.
	EndLine int `json:"end_line,omitempty"`

	// Score for this chunk match.
	Score float64 `json:"score,omitempty"`
}

// ZoektSearchStats contains search statistics.
type ZoektSearchStats struct {
	// FilesSearched is the number of files examined.
	FilesSearched int `json:"files_searched,omitempty"`

	// FilesConsidered is the number of files after filtering.
	FilesConsidered int `json:"files_considered,omitempty"`

	// BytesSearched is the number of bytes examined.
	BytesSearched int64 `json:"bytes_searched,omitempty"`

	// ShardsSearched is the number of Zoekt shards queried.
	ShardsSearched int `json:"shards_searched,omitempty"`

	// DocumentCount is the number of documents in the index.
	DocumentCount int `json:"document_count,omitempty"`

	// IndexBytes is the size of the index in bytes.
	IndexBytes int64 `json:"index_bytes,omitempty"`
}

// ZoektListOptions configures repository listing.
type ZoektListOptions struct {
	// Query filters repositories.
	Query string `json:"query,omitempty"`

	// MaxResults limits the number of repositories returned.
	MaxResults int `json:"max_results,omitempty"`

	// Timeout for the list request.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// ZoektListResult contains repository listing results.
type ZoektListResult struct {
	// Repositories matching the query.
	Repositories []ZoektRepoInfo `json:"repositories,omitempty"`

	// TotalCount is the total number of matching repositories.
	TotalCount int `json:"total_count,omitempty"`

	// Duration is time taken to execute the list.
	Duration time.Duration `json:"duration,omitempty"`
}

// ZoektRepoInfo contains information about a repository.
type ZoektRepoInfo struct {
	// Name is the repository name (e.g., github.com/user/repo).
	Name string `json:"name,omitempty"`

	// Branch is the indexed branch.
	Branch string `json:"branch,omitempty"`

	// CommitHash is the committed at indexing time.
	CommitHash string `json:"commit_hash,omitempty"`

	// FilesCount is the number of files indexed.
	FilesCount int `json:"files_count,omitempty"`

	// ShardCount is the number of shards for this repo.
	ShardCount int `json:"shard_count,omitempty"`

	// IndexTime is when the repo was indexed.
	IndexTime time.Time `json:"index_time,omitempty"`
}

// ZoektHealthStatus represents the health status of the Zoekt webserver.
type ZoektHealthStatus struct {
	// Healthy indicates if the server is healthy.
	Healthy bool `json:"healthy,omitempty"`

	// Version is the Zoekt webserver version.
	Version string `json:"version,omitempty"`

	// Uptime is the server uptime.
	Uptime time.Duration `json:"uptime,omitempty"`

	// IndexStats contains aggregate index statistics.
	IndexStats *ZoektIndexStats `json:"index_stats,omitempty"`

	// Error contains any error details if unhealthy.
	Error string `json:"error,omitempty"`
}

// ZoektIndexStats provides aggregate statistics about Zoekt indexes.
type ZoektIndexStats struct {
	// ShardCount is the total number of shards.
	ShardCount int `json:"shard_count,omitempty"`

	// DocumentCount is the total number of documents indexed.
	DocumentCount int `json:"document_count,omitempty"`

	// IndexBytes is the total size of indexes in bytes.
	IndexBytes int64 `json:"index_bytes,omitempty"`

	// Repositories is the number of repositories indexed.
	Repositories int `json:"repos,omitempty"`
}
