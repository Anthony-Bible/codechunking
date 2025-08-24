package outbound

import (
	"context"
	"time"
)

// GitClient defines the interface for git operations.
type GitClient interface {
	Clone(ctx context.Context, repoURL, targetPath string) error
	GetCommitHash(ctx context.Context, repoPath string) (string, error)
	GetBranch(ctx context.Context, repoPath string) (string, error)
}

// CodeParser defines the interface for parsing code.
type CodeParser interface {
	ParseDirectory(ctx context.Context, dirPath string, config CodeParsingConfig) ([]CodeChunk, error)
}

// EmbeddingGenerator defines the interface for generating embeddings.
type EmbeddingGenerator interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float64, error)
}

// CodeParsingConfig holds configuration for code parsing.
type CodeParsingConfig struct {
	ChunkSizeBytes   int
	MaxFileSizeBytes int64
	FileFilters      []string
	IncludeTests     bool
	ExcludeVendor    bool
}

// CodeChunk represents a parsed code chunk.
type CodeChunk struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	Size      int       `json:"size"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}
