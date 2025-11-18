// Package outbound defines the outbound ports (interfaces) for external dependencies.
package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// VectorStorageRepository defines the outbound port for vector embeddings persistence.
// This interface provides efficient storage and retrieval of vector embeddings with
// support for bulk operations, transactions, and duplicate detection.
type VectorStorageRepository interface {
	// BulkInsertEmbeddings inserts multiple embeddings efficiently using batch operations.
	// Supports both regular and partitioned tables with automatic table selection.
	BulkInsertEmbeddings(
		ctx context.Context,
		embeddings []VectorEmbedding,
		options BulkInsertOptions,
	) (*BulkInsertResult, error)

	// UpsertEmbedding inserts or updates a single embedding with conflict resolution.
	// Uses ON CONFLICT DO UPDATE for handling duplicates based on (chunk_id, model_version).
	UpsertEmbedding(ctx context.Context, embedding VectorEmbedding, options UpsertOptions) error

	// FindEmbeddingByChunkID retrieves an embedding by chunk ID and model version.
	FindEmbeddingByChunkID(ctx context.Context, chunkID uuid.UUID, modelVersion string) (*VectorEmbedding, error)

	// FindEmbeddingsByRepositoryID retrieves all embeddings for a specific repository.
	// Automatically queries the appropriate table (regular or partitioned) based on options.
	FindEmbeddingsByRepositoryID(
		ctx context.Context,
		repositoryID uuid.UUID,
		filters EmbeddingFilters,
	) ([]VectorEmbedding, error)

	// DeleteEmbeddingsByChunkIDs performs bulk deletion of embeddings by chunk IDs.
	// Supports transaction management for atomic operations.
	DeleteEmbeddingsByChunkIDs(
		ctx context.Context,
		chunkIDs []uuid.UUID,
		options DeleteOptions,
	) (*BulkDeleteResult, error)

	// DeleteEmbeddingsByRepositoryID deletes all embeddings for a repository.
	// Used during repository cleanup and reindexing operations.
	DeleteEmbeddingsByRepositoryID(
		ctx context.Context,
		repositoryID uuid.UUID,
		options DeleteOptions,
	) (*BulkDeleteResult, error)

	// VectorSimilaritySearch performs similarity search using vector operations.
	// Supports configurable similarity metrics and result limiting.
	VectorSimilaritySearch(
		ctx context.Context,
		queryVector []float64,
		options SimilaritySearchOptions,
	) ([]VectorSimilarityResult, error)

	// GetStorageStatistics returns storage statistics for monitoring and optimization.
	GetStorageStatistics(ctx context.Context, options StatisticsOptions) (*StorageStatistics, error)

	// BeginTransaction starts a new transaction for atomic vector operations.
	BeginTransaction(ctx context.Context) (VectorTransaction, error)
}

// VectorTransaction provides transaction management for vector operations.
type VectorTransaction interface {
	// BulkInsertEmbeddings performs bulk insert within transaction context.
	BulkInsertEmbeddings(
		ctx context.Context,
		embeddings []VectorEmbedding,
		options BulkInsertOptions,
	) (*BulkInsertResult, error)

	// UpsertEmbedding performs upsert within transaction context.
	UpsertEmbedding(ctx context.Context, embedding VectorEmbedding, options UpsertOptions) error

	// DeleteEmbeddingsByChunkIDs performs bulk delete within transaction context.
	DeleteEmbeddingsByChunkIDs(
		ctx context.Context,
		chunkIDs []uuid.UUID,
		options DeleteOptions,
	) (*BulkDeleteResult, error)

	// Commit commits the transaction and makes all changes permanent.
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction and undoes all changes.
	Rollback(ctx context.Context) error
}

// VectorEmbedding represents a stored vector embedding with metadata.
type VectorEmbedding struct {
	ID           uuid.UUID  `json:"id"`
	ChunkID      uuid.UUID  `json:"chunk_id"`
	RepositoryID uuid.UUID  `json:"repository_id"` // Required for partitioned tables
	Embedding    []float64  `json:"embedding"`     // 768-dimensional vector for Gemini gemini-embedding-001
	ModelVersion string     `json:"model_version"` // e.g., "gemini-embedding-001"
	CreatedAt    time.Time  `json:"created_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	// Metadata fields populated from code_chunks table join (optional)
	Language  string `json:"language,omitempty"`   // Programming language (e.g., "go", "python")
	ChunkType string `json:"chunk_type,omitempty"` // Type of code chunk (e.g., "function", "class")
	FilePath  string `json:"file_path,omitempty"`  // File path where chunk is located
}

// BulkInsertOptions configures bulk insertion behavior.
type BulkInsertOptions struct {
	BatchSize           int                 `json:"batch_size"`            // Number of embeddings per batch (default: 1000)
	UsePartitionedTable bool                `json:"use_partitioned_table"` // Whether to use partitioned table
	OnConflictAction    ConflictAction      `json:"on_conflict_action"`    // Action to take on conflicts
	TransactionBehavior TransactionBehavior `json:"transaction_behavior"`  // Transaction management behavior
	ValidateVectorDims  bool                `json:"validate_vector_dims"`  // Validate vector dimensions before insert
	EnableMetrics       bool                `json:"enable_metrics"`        // Enable performance metrics collection
	Timeout             time.Duration       `json:"timeout"`               // Operation timeout
}

// UpsertOptions configures upsert behavior.
type UpsertOptions struct {
	UsePartitionedTable bool                    `json:"use_partitioned_table"` // Whether to use partitioned table
	UpdateStrategy      EmbeddingUpdateStrategy `json:"update_strategy"`       // Strategy for handling updates
	ValidateVectorDims  bool                    `json:"validate_vector_dims"`  // Validate vector dimensions
	Timeout             time.Duration           `json:"timeout"`               // Operation timeout
}

// EmbeddingFilters provides filtering options for embedding queries.
type EmbeddingFilters struct {
	ModelVersions       []string      `json:"model_versions,omitempty"` // Filter by model versions
	CreatedAfter        *time.Time    `json:"created_after,omitempty"`  // Filter by creation date
	CreatedBefore       *time.Time    `json:"created_before,omitempty"` // Filter by creation date
	ChunkIDs            []uuid.UUID   `json:"chunk_ids,omitempty"`      // Filter by specific chunk IDs
	IncludeDeleted      bool          `json:"include_deleted"`          // Include soft-deleted embeddings
	Limit               int           `json:"limit"`                    // Maximum number of results
	Offset              int           `json:"offset"`                   // Pagination offset
	UsePartitionedTable bool          `json:"use_partitioned_table"`    // Query partitioned table
	Timeout             time.Duration `json:"timeout"`                  // Query timeout
}

// DeleteOptions configures deletion behavior.
type DeleteOptions struct {
	UsePartitionedTable bool                `json:"use_partitioned_table"` // Whether to use partitioned table
	SoftDelete          bool                `json:"soft_delete"`           // Use soft delete (set deleted_at) vs hard delete
	TransactionBehavior TransactionBehavior `json:"transaction_behavior"`  // Transaction management behavior
	BatchSize           int                 `json:"batch_size"`            // Batch size for bulk operations
	Timeout             time.Duration       `json:"timeout"`               // Operation timeout
}

// SimilaritySearchOptions configures vector similarity search.
type SimilaritySearchOptions struct {
	UsePartitionedTable bool              `json:"use_partitioned_table"`     // Whether to use partitioned table
	SimilarityMetric    SimilarityMetric  `json:"similarity_metric"`         // Similarity metric to use
	MaxResults          int               `json:"max_results"`               // Maximum number of results
	MinSimilarity       float64           `json:"min_similarity"`            // Minimum similarity threshold
	RepositoryIDs       []uuid.UUID       `json:"repository_ids"`            // Filter by specific repositories
	ModelVersions       []string          `json:"model_versions"`            // Filter by model versions
	Languages           []string          `json:"languages,omitempty"`       // Filter by programming languages (requires metadata columns)
	ChunkTypes          []string          `json:"chunk_types,omitempty"`     // Filter by chunk types (requires metadata columns)
	FileExtensions      []string          `json:"file_extensions,omitempty"` // Filter by file extensions (requires metadata columns)
	EfSearch            int               `json:"ef_search"`                 // HNSW search parameter for recall/speed tradeoff
	IncludeDeleted      bool              `json:"include_deleted"`           // Include soft-deleted embeddings
	IterativeScanMode   IterativeScanMode `json:"iterative_scan_mode"`       // pgvector 0.8.0+ iterative scan mode
	Timeout             time.Duration     `json:"timeout"`                   // Search timeout
}

// StatisticsOptions configures storage statistics retrieval.
type StatisticsOptions struct {
	IncludePartitioned bool `json:"include_partitioned"` // Include partitioned table statistics
	IncludeRegular     bool `json:"include_regular"`     // Include regular table statistics
	IncludeIndexStats  bool `json:"include_index_stats"` // Include index statistics
	IncludePartitions  bool `json:"include_partitions"`  // Include per-partition statistics
}

// BulkInsertResult provides results and metrics from bulk insertion operations.
type BulkInsertResult struct {
	InsertedCount    int           `json:"inserted_count"`     // Number of embeddings successfully inserted
	UpdatedCount     int           `json:"updated_count"`      // Number of embeddings updated (on conflict)
	SkippedCount     int           `json:"skipped_count"`      // Number of embeddings skipped
	FailedCount      int           `json:"failed_count"`       // Number of embeddings that failed
	Duration         time.Duration `json:"duration"`           // Total operation duration
	BatchCount       int           `json:"batch_count"`        // Number of batches processed
	AverageBatchTime time.Duration `json:"average_batch_time"` // Average time per batch
	ErrorDetails     []BatchError  `json:"error_details"`      // Details of any errors encountered
}

// BulkDeleteResult provides results from bulk deletion operations.
type BulkDeleteResult struct {
	DeletedCount     int           `json:"deleted_count"`      // Number of embeddings deleted
	SkippedCount     int           `json:"skipped_count"`      // Number of embeddings skipped (not found)
	Duration         time.Duration `json:"duration"`           // Total operation duration
	BatchCount       int           `json:"batch_count"`        // Number of batches processed
	AverageBatchTime time.Duration `json:"average_batch_time"` // Average time per batch
}

// VectorSimilarityResult represents a result from vector similarity search.
type VectorSimilarityResult struct {
	Embedding  VectorEmbedding `json:"embedding"`  // The matching embedding
	Similarity float64         `json:"similarity"` // Similarity score (0.0 to 1.0)
	Distance   float64         `json:"distance"`   // Distance metric value
	Rank       int             `json:"rank"`       // Result rank (1-based)
}

// StorageStatistics provides comprehensive storage statistics.
type StorageStatistics struct {
	RegularTable     *TableStatistics      `json:"regular_table,omitempty"`     // Regular embeddings table stats
	PartitionedTable *TableStatistics      `json:"partitioned_table,omitempty"` // Partitioned table stats
	Partitions       []PartitionStatistics `json:"partitions,omitempty"`        // Per-partition statistics
	TotalEmbeddings  int64                 `json:"total_embeddings"`            // Total embeddings across all tables
	TotalSize        int64                 `json:"total_size"`                  // Total storage size in bytes
	LastUpdated      time.Time             `json:"last_updated"`                // When statistics were last updated
}

// TableStatistics provides statistics for a single table.
type TableStatistics struct {
	TableName    string     `json:"table_name"`    // Name of the table
	RowCount     int64      `json:"row_count"`     // Number of rows
	TableSize    int64      `json:"table_size"`    // Table size in bytes
	IndexSize    int64      `json:"index_size"`    // Index size in bytes
	LastVacuum   *time.Time `json:"last_vacuum"`   // Last vacuum time
	LastAnalyze  *time.Time `json:"last_analyze"`  // Last analyze time
	SeqScans     int64      `json:"seq_scans"`     // Number of sequential scans
	IndexScans   int64      `json:"index_scans"`   // Number of index scans
	TupleInserts int64      `json:"tuple_inserts"` // Number of tuple inserts
	TupleUpdates int64      `json:"tuple_updates"` // Number of tuple updates
	TupleDeletes int64      `json:"tuple_deletes"` // Number of tuple deletes
}

// PartitionStatistics provides statistics for individual partitions.
type PartitionStatistics struct {
	PartitionName string     `json:"partition_name"` // Name of the partition
	RowCount      int64      `json:"row_count"`      // Number of rows in partition
	TableSize     int64      `json:"table_size"`     // Partition size in bytes
	IndexSize     int64      `json:"index_size"`     // Index size in bytes
	LastVacuum    *time.Time `json:"last_vacuum"`    // Last vacuum time
	LastAnalyze   *time.Time `json:"last_analyze"`   // Last analyze time
}

// BatchError provides details about batch operation errors.
type BatchError struct {
	BatchIndex  int    `json:"batch_index"` // Index of the failed batch
	ErrorType   string `json:"error_type"`  // Type of error
	ErrorCode   string `json:"error_code"`  // Database error code
	Message     string `json:"message"`     // Error message
	RetryCount  int    `json:"retry_count"` // Number of retries attempted
	Recoverable bool   `json:"recoverable"` // Whether the error is recoverable
}

// ConflictAction defines actions to take on insert conflicts.
type ConflictAction string

const (
	ConflictActionDoNothing ConflictAction = "do_nothing" // Skip conflicting records
	ConflictActionUpdate    ConflictAction = "update"     // Update existing records
	ConflictActionError     ConflictAction = "error"      // Return error on conflict
)

// TransactionBehavior defines transaction management behavior.
type TransactionBehavior string

const (
	TransactionBehaviorAuto     TransactionBehavior = "auto"     // Automatic transaction per batch
	TransactionBehaviorManual   TransactionBehavior = "manual"   // Manual transaction management
	TransactionBehaviorDisabled TransactionBehavior = "disabled" // No transaction management
)

// EmbeddingUpdateStrategy defines strategies for handling embedding updates.
type EmbeddingUpdateStrategy string

const (
	EmbeddingUpdateStrategyReplace   EmbeddingUpdateStrategy = "replace"   // Replace entire embedding
	EmbeddingUpdateStrategyMerge     EmbeddingUpdateStrategy = "merge"     // Merge with existing data
	EmbeddingUpdateStrategyVersioned EmbeddingUpdateStrategy = "versioned" // Create new version
)

// SimilarityMetric defines vector similarity metrics.
type SimilarityMetric string

const (
	SimilarityMetricCosine     SimilarityMetric = "cosine"    // Cosine similarity
	SimilarityMetricEuclidean  SimilarityMetric = "euclidean" // Euclidean distance
	SimilarityMetricDotProduct SimilarityMetric = "dot"       // Dot product similarity
)

// IterativeScanMode defines pgvector 0.8.0+ iterative scan modes for HNSW indexes.
// Iterative scanning automatically expands search scope when filters reduce result count,
// ensuring the requested number of results are returned even with selective filters.
type IterativeScanMode string

const (
	// IterativeScanOff disables iterative scanning (default pgvector behavior).
	// May return fewer results than requested when filters are highly selective.
	IterativeScanOff IterativeScanMode = "off"

	// IterativeScanStrictOrder enables iterative scanning with strict distance ordering.
	// Guarantees exact ordering but may be slower for highly selective filters.
	IterativeScanStrictOrder IterativeScanMode = "strict_order"

	// IterativeScanRelaxedOrder enables iterative scanning with relaxed ordering.
	// Better performance and recall than strict_order, with approximate ordering.
	// Recommended for most use cases where slight ordering differences are acceptable.
	IterativeScanRelaxedOrder IterativeScanMode = "relaxed_order"
)
