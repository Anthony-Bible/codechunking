package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CodeChunk represents a parsed code chunk.
type CodeChunk struct {
	ID           string    `json:"id"`
	RepositoryID uuid.UUID `json:"repository_id,omitempty"`
	FilePath     string    `json:"file_path"`
	StartLine    int       `json:"start_line"`
	EndLine      int       `json:"end_line"`
	Content      string    `json:"content"`
	Language     string    `json:"language"`
	Size         int       `json:"size"`
	Hash         string    `json:"hash"`
	CreatedAt    time.Time `json:"created_at"`
	// Enhanced type information from semantic analysis
	Type          string `json:"type,omitempty"`           // Semantic construct type (function, class, method, interface, etc.)
	EntityName    string `json:"entity_name,omitempty"`    // Name of the entity (function name, class name, etc.)
	ParentEntity  string `json:"parent_entity,omitempty"`  // Parent entity name (class for method, namespace for class, etc.)
	QualifiedName string `json:"qualified_name,omitempty"` // Fully qualified name
	Signature     string `json:"signature,omitempty"`      // Function/method signature
	Visibility    string `json:"visibility,omitempty"`     // Visibility modifier (public, private, protected)
}

// ChunkRepository defines operations for storing and retrieving code chunks.
type ChunkRepository interface {
	// SaveChunk stores a code chunk in the database.
	SaveChunk(ctx context.Context, chunk *CodeChunk) error

	// SaveChunks stores multiple code chunks in a batch operation.
	SaveChunks(ctx context.Context, chunks []CodeChunk) error

	// FindOrCreateChunks saves chunks and returns the actual chunk IDs (existing or new).
	// For chunks that already exist (same repo/path/hash), returns the existing chunk with its ID.
	// This ensures batch embedding requests use persisted chunk IDs, preventing FK constraint violations.
	FindOrCreateChunks(ctx context.Context, chunks []CodeChunk) ([]CodeChunk, error)

	// GetChunk retrieves a chunk by ID.
	GetChunk(ctx context.Context, id uuid.UUID) (*CodeChunk, error)

	// GetChunksForRepository retrieves all chunks for a repository.
	GetChunksForRepository(ctx context.Context, repositoryID uuid.UUID) ([]CodeChunk, error)

	// DeleteChunksForRepository deletes all chunks for a repository.
	DeleteChunksForRepository(ctx context.Context, repositoryID uuid.UUID) error

	// CountChunksForRepository returns the number of chunks for a repository.
	CountChunksForRepository(ctx context.Context, repositoryID uuid.UUID) (int, error)
}

// EmbeddingRepository defines operations for storing and retrieving embeddings.
type EmbeddingRepository interface {
	// SaveEmbedding stores an embedding with its associated chunk.
	SaveEmbedding(ctx context.Context, embedding *Embedding) error

	// SaveEmbeddings stores multiple embeddings in a batch operation.
	SaveEmbeddings(ctx context.Context, embeddings []Embedding) error

	// GetEmbedding retrieves an embedding by chunk ID.
	GetEmbedding(ctx context.Context, chunkID uuid.UUID) (*Embedding, error)

	// GetEmbeddingsForRepository retrieves all embeddings for a repository.
	GetEmbeddingsForRepository(ctx context.Context, repositoryID uuid.UUID) ([]Embedding, error)

	// DeleteEmbeddingsForRepository deletes all embeddings for a repository.
	DeleteEmbeddingsForRepository(ctx context.Context, repositoryID uuid.UUID) error

	// SearchSimilar performs vector similarity search.
	SearchSimilar(ctx context.Context, query []float64, limit int, threshold float64) ([]EmbeddingSearchResult, error)
}

// Embedding represents a vector embedding for a code chunk.
type Embedding struct {
	ID           uuid.UUID `json:"id"`
	ChunkID      uuid.UUID `json:"chunk_id"`
	RepositoryID uuid.UUID `json:"repository_id"` // Added for partitioned storage
	Vector       []float64 `json:"vector"`
	ModelVersion string    `json:"model_version"`
	CreatedAt    string    `json:"created_at"`
}

// EmbeddingSearchResult represents a search result with similarity score.
type EmbeddingSearchResult struct {
	ChunkID    uuid.UUID  `json:"chunk_id"`
	Similarity float64    `json:"similarity"`
	Chunk      *CodeChunk `json:"chunk,omitempty"`
}

// ChunkStorageRepository combines chunk and embedding storage operations.
// This provides a simpler interface for the job processor compared to the full VectorStorageRepository.
type ChunkStorageRepository interface {
	ChunkRepository
	EmbeddingRepository

	// SaveChunkWithEmbedding stores both chunk and embedding in a single transaction.
	SaveChunkWithEmbedding(ctx context.Context, chunk *CodeChunk, embedding *Embedding) error

	// SaveChunksWithEmbeddings stores multiple chunks and embeddings in a single transaction.
	SaveChunksWithEmbeddings(ctx context.Context, chunks []CodeChunk, embeddings []Embedding) error
}
