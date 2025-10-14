package repository

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/service"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"
)

// Constants for default values.
const (
	defaultChunkType     = "fragment"
	defaultEntityName    = ""
	defaultParentEntity  = ""
	defaultQualifiedName = ""
	defaultSignature     = ""
	defaultVisibility    = ""
)

// SQL query constants.
const (
	insertChunkQuery = `
		INSERT INTO codechunking.code_chunks (
			id, repository_id, file_path, chunk_type, content, language,
			start_line, end_line, entity_name, parent_entity, content_hash, metadata,
			qualified_name, signature, visibility
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (repository_id, file_path, content_hash) DO NOTHING
	`

	insertPartitionedEmbeddingQuery = `
		INSERT INTO codechunking.embeddings_partitioned (
			id, chunk_id, repository_id, embedding, model_version
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (chunk_id, model_version, repository_id)
		DO UPDATE SET
			embedding = EXCLUDED.embedding,
			created_at = CURRENT_TIMESTAMP
	`

	insertRegularEmbeddingQuery = `
		INSERT INTO codechunking.embeddings (
			id, chunk_id, embedding, model_version
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (chunk_id, model_version)
		DO UPDATE SET
			embedding = EXCLUDED.embedding,
			created_at = CURRENT_TIMESTAMP
	`
)

// PostgreSQLChunkRepository implements the ChunkStorageRepository interface.
// It provides operations for both code chunks and embeddings with support for
// both regular and partitioned embeddings tables.
type PostgreSQLChunkRepository struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLChunkRepository creates a new PostgreSQL chunk repository.
func NewPostgreSQLChunkRepository(pool *pgxpool.Pool) *PostgreSQLChunkRepository {
	return &PostgreSQLChunkRepository{
		pool: pool,
	}
}

// FindChunksByIDs retrieves chunks by their IDs with repository information.
func (r *PostgreSQLChunkRepository) FindChunksByIDs(
	ctx context.Context,
	chunkIDs []uuid.UUID,
) ([]service.ChunkInfo, error) {
	if len(chunkIDs) == 0 {
		return []service.ChunkInfo{}, nil
	}

	slogger.Debug(ctx, "Finding chunks by IDs", slogger.Field("chunk_ids_count", len(chunkIDs)))

	query := `
		SELECT
			c.id,
			c.content,
			c.file_path,
			c.language,
			c.start_line,
			c.end_line,
			c.chunk_type,
			c.entity_name,
			c.parent_entity,
			c.qualified_name,
			c.signature,
			c.visibility,
			r.id,
			r.name,
			r.url
		FROM codechunking.code_chunks c
		JOIN codechunking.repositories r ON c.repository_id = r.id
		WHERE c.id = ANY($1) AND c.deleted_at IS NULL AND r.deleted_at IS NULL
	`

	rows, err := r.pool.Query(ctx, query, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	var chunks []service.ChunkInfo
	foundChunkIDs := make(map[uuid.UUID]bool)

	for rows.Next() {
		var chunk service.ChunkInfo

		err := rows.Scan(
			&chunk.ChunkID,
			&chunk.Content,
			&chunk.FilePath,
			&chunk.Language,
			&chunk.StartLine,
			&chunk.EndLine,
			&chunk.Type,
			&chunk.EntityName,
			&chunk.ParentEntity,
			&chunk.QualifiedName,
			&chunk.Signature,
			&chunk.Visibility,
			&chunk.Repository.ID,
			&chunk.Repository.Name,
			&chunk.Repository.URL,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chunk row: %w", err)
		}

		chunks = append(chunks, chunk)
		foundChunkIDs[chunk.ChunkID] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chunk rows: %w", err)
	}

	// Log warning if requested chunk IDs were not found (indicates orphaned embeddings)
	missingCount := len(chunkIDs) - len(chunks)
	if missingCount > 0 {
		// Find which specific chunk IDs are missing
		missingIDs := make([]string, 0, missingCount)
		for _, id := range chunkIDs {
			if !foundChunkIDs[id] {
				missingIDs = append(missingIDs, id.String())
			}
		}

		slogger.Warn(ctx, "Some chunk IDs were not found - possible orphaned embeddings", slogger.Fields{
			"requested_count": len(chunkIDs),
			"found_count":     len(chunks),
			"missing_count":   missingCount,
			"missing_ids":     missingIDs,
		})
	} else {
		slogger.Debug(ctx, "All requested chunks found successfully", slogger.Field("chunks_count", len(chunks)))
	}

	return chunks, nil
}

// ============================================================================
// ChunkRepository Interface Implementation
// ============================================================================

// SaveChunk stores a code chunk in the database.
func (r *PostgreSQLChunkRepository) SaveChunk(ctx context.Context, chunk *outbound.CodeChunk) error {
	query := insertChunkQuery

	// Parse chunk ID as UUID
	chunkID, err := uuid.Parse(chunk.ID)
	if err != nil {
		slogger.Error(ctx, "Invalid chunk ID format", slogger.Fields2(
			"chunk_id", chunk.ID,
			"error", err.Error(),
		))
		return fmt.Errorf("invalid chunk ID format: %w", err)
	}

	// Use chunk type information or defaults
	entityName := chunk.EntityName
	if entityName == "" {
		entityName = defaultEntityName
	}

	parentEntity := chunk.ParentEntity
	if parentEntity == "" {
		parentEntity = defaultParentEntity
	}

	chunkType := chunk.Type
	if chunkType == "" {
		chunkType = defaultChunkType
	}

	qualifiedName := chunk.QualifiedName
	if qualifiedName == "" {
		qualifiedName = defaultQualifiedName
	}

	signature := chunk.Signature
	if signature == "" {
		signature = defaultSignature
	}

	visibility := chunk.Visibility
	if visibility == "" {
		visibility = defaultVisibility
	}

	// Validate repository ID is provided
	repositoryID := chunk.RepositoryID
	if repositoryID == uuid.Nil {
		slogger.Error(ctx, "Missing repository_id for chunk save", slogger.Fields2(
			"chunk_id", chunk.ID,
			"file_path", chunk.FilePath,
		))
		return errors.New("repository_id is required to save chunk")
	}

	_, err = r.pool.Exec(ctx, query,
		chunkID,
		repositoryID,
		chunk.FilePath,
		chunkType,
		chunk.Content,
		chunk.Language,
		chunk.StartLine,
		chunk.EndLine,
		entityName,
		parentEntity,
		chunk.Hash,
		nil, // metadata
		qualifiedName,
		signature,
		visibility,
	)
	if err != nil {
		slogger.Error(ctx, "Failed to save chunk", slogger.Fields{
			"chunk_id":      chunk.ID,
			"file_path":     chunk.FilePath,
			"repository_id": repositoryID.String(),
			"error":         err.Error(),
		})
		return fmt.Errorf("failed to save chunk: %w", err)
	}

	slogger.Debug(ctx, "Chunk saved successfully", slogger.Fields2(
		"chunk_id", chunk.ID,
		"file_path", chunk.FilePath,
	))

	return nil
}

// SaveChunks stores multiple code chunks in a batch operation.
func (r *PostgreSQLChunkRepository) SaveChunks(ctx context.Context, chunks []outbound.CodeChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		slogger.Error(ctx, "Failed to begin transaction for batch chunk save", slogger.Field("error", err.Error()))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slogger.Warn(ctx, "Failed to rollback transaction", slogger.Field("error", err.Error()))
		}
	}()

	query := insertChunkQuery

	for _, chunk := range chunks {
		chunkID, err := uuid.Parse(chunk.ID)
		if err != nil {
			slogger.Error(ctx, "Invalid chunk ID in batch", slogger.Fields2(
				"chunk_id", chunk.ID,
				"error", err.Error(),
			))
			return fmt.Errorf("invalid chunk ID format: %w", err)
		}

		// Use chunk type information or defaults
		entityName := chunk.EntityName
		if entityName == "" {
			entityName = defaultEntityName
		}

		parentEntity := chunk.ParentEntity
		if parentEntity == "" {
			parentEntity = defaultParentEntity
		}

		chunkType := chunk.Type
		if chunkType == "" {
			chunkType = defaultChunkType
		}

		qualifiedName := chunk.QualifiedName
		if qualifiedName == "" {
			qualifiedName = defaultQualifiedName
		}

		signature := chunk.Signature
		if signature == "" {
			signature = defaultSignature
		}

		visibility := chunk.Visibility
		if visibility == "" {
			visibility = defaultVisibility
		}

		repositoryID := chunk.RepositoryID
		if repositoryID == uuid.Nil {
			slogger.Error(ctx, "Missing repository_id for chunk batch save", slogger.Fields2(
				"chunk_id", chunk.ID,
				"file_path", chunk.FilePath,
			))
			return errors.New("repository_id is required to save chunk in batch")
		}

		_, err = tx.Exec(ctx, query,
			chunkID,
			repositoryID,
			chunk.FilePath,
			chunkType,
			chunk.Content,
			chunk.Language,
			chunk.StartLine,
			chunk.EndLine,
			entityName,
			parentEntity,
			chunk.Hash,
			nil, // metadata
			qualifiedName,
			signature,
			visibility,
		)
		if err != nil {
			slogger.Error(ctx, "Failed to save chunk in batch", slogger.Fields{
				"chunk_id":      chunk.ID,
				"file_path":     chunk.FilePath,
				"repository_id": repositoryID.String(),
				"error":         err.Error(),
			})
			return fmt.Errorf("failed to save chunk in batch: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slogger.Error(ctx, "Failed to commit batch chunk save", slogger.Fields2(
			"chunk_count", len(chunks),
			"error", err.Error(),
		))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slogger.Info(ctx, "Batch chunks saved successfully", slogger.Field(
		"chunk_count", len(chunks),
	))

	return nil
}

// GetChunk retrieves a chunk by ID.
func (r *PostgreSQLChunkRepository) GetChunk(ctx context.Context, id uuid.UUID) (*outbound.CodeChunk, error) {
	query := `
		SELECT id, file_path, start_line, end_line, content, language, content_hash, created_at,
		       chunk_type, entity_name, parent_entity, qualified_name, signature, visibility
		FROM codechunking.code_chunks
		WHERE id = $1 AND deleted_at IS NULL
	`

	var chunk outbound.CodeChunk
	var createdAt time.Time

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&chunk.ID,
		&chunk.FilePath,
		&chunk.StartLine,
		&chunk.EndLine,
		&chunk.Content,
		&chunk.Language,
		&chunk.Hash,
		&createdAt,
		&chunk.Type,
		&chunk.EntityName,
		&chunk.ParentEntity,
		&chunk.QualifiedName,
		&chunk.Signature,
		&chunk.Visibility,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slogger.Debug(ctx, "Chunk not found", slogger.Field("chunk_id", id.String()))
			return nil, errors.New("chunk not found")
		}
		slogger.Error(ctx, "Failed to get chunk", slogger.Fields2(
			"chunk_id", id.String(),
			"error", err.Error(),
		))
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	chunk.CreatedAt = createdAt
	chunk.Size = len(chunk.Content)

	slogger.Debug(ctx, "Chunk retrieved successfully", slogger.Field("chunk_id", id.String()))
	return &chunk, nil
}

// GetChunksForRepository retrieves all chunks for a repository.
func (r *PostgreSQLChunkRepository) GetChunksForRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
) ([]outbound.CodeChunk, error) {
	query := `
		SELECT id, file_path, start_line, end_line, content, language, content_hash, created_at,
		       chunk_type, entity_name, parent_entity, qualified_name, signature, visibility
		FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
		ORDER BY file_path, start_line
	`

	rows, err := r.pool.Query(ctx, query, repositoryID)
	if err != nil {
		slogger.Error(ctx, "Failed to query chunks for repository", slogger.Fields2(
			"repository_id", repositoryID.String(),
			"error", err.Error(),
		))
		return nil, fmt.Errorf("failed to query chunks for repository: %w", err)
	}
	defer rows.Close()

	var chunks []outbound.CodeChunk
	for rows.Next() {
		var chunk outbound.CodeChunk
		var createdAt time.Time

		err := rows.Scan(
			&chunk.ID,
			&chunk.FilePath,
			&chunk.StartLine,
			&chunk.EndLine,
			&chunk.Content,
			&chunk.Language,
			&chunk.Hash,
			&createdAt,
			&chunk.Type,
			&chunk.EntityName,
			&chunk.ParentEntity,
			&chunk.QualifiedName,
			&chunk.Signature,
			&chunk.Visibility,
		)
		if err != nil {
			slogger.Error(ctx, "Failed to scan chunk row", slogger.Fields2(
				"repository_id", repositoryID.String(),
				"error", err.Error(),
			))
			return nil, fmt.Errorf("failed to scan chunk row: %w", err)
		}

		chunk.CreatedAt = createdAt
		chunk.Size = len(chunk.Content)
		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		slogger.Error(ctx, "Error iterating chunk rows", slogger.Fields2(
			"repository_id", repositoryID.String(),
			"error", err.Error(),
		))
		return nil, fmt.Errorf("error iterating chunk rows: %w", err)
	}

	slogger.Info(ctx, "Retrieved chunks for repository", slogger.Fields2(
		"repository_id", repositoryID.String(),
		"chunk_count", len(chunks),
	))

	return chunks, nil
}

// DeleteChunksForRepository deletes all chunks for a repository.
func (r *PostgreSQLChunkRepository) DeleteChunksForRepository(ctx context.Context, repositoryID uuid.UUID) error {
	query := `
		UPDATE codechunking.code_chunks
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE repository_id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, repositoryID)
	if err != nil {
		slogger.Error(ctx, "Failed to delete chunks for repository", slogger.Fields2(
			"repository_id", repositoryID.String(),
			"error", err.Error(),
		))
		return fmt.Errorf("failed to delete chunks for repository: %w", err)
	}

	deletedCount := result.RowsAffected()
	slogger.Info(ctx, "Deleted chunks for repository", slogger.Fields2(
		"repository_id", repositoryID.String(),
		"deleted_count", deletedCount,
	))

	return nil
}

// CountChunksForRepository returns the number of chunks for a repository.
func (r *PostgreSQLChunkRepository) CountChunksForRepository(ctx context.Context, repositoryID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
	`

	var count int
	err := r.pool.QueryRow(ctx, query, repositoryID).Scan(&count)
	if err != nil {
		slogger.Error(ctx, "Failed to count chunks for repository", slogger.Fields2(
			"repository_id", repositoryID.String(),
			"error", err.Error(),
		))
		return 0, fmt.Errorf("failed to count chunks for repository: %w", err)
	}

	slogger.Debug(ctx, "Counted chunks for repository", slogger.Fields2(
		"repository_id", repositoryID.String(),
		"count", count,
	))

	return count, nil
}

// ============================================================================
// EmbeddingRepository Interface Implementation
// ============================================================================

// SaveEmbedding stores an embedding with its associated chunk.
func (r *PostgreSQLChunkRepository) SaveEmbedding(ctx context.Context, embedding *outbound.Embedding) error {
	// Try partitioned table first for better performance, fallback to regular table
	err := r.saveEmbeddingToPartitioned(ctx, embedding)
	if err != nil {
		slogger.Warn(ctx, "Failed to save to partitioned table, falling back to regular table", slogger.Fields2(
			"embedding_id", embedding.ID.String(),
			"error", err.Error(),
		))
		return r.saveEmbeddingToRegular(ctx, embedding)
	}
	return nil
}

// saveEmbeddingToPartitioned saves embedding to the partitioned table.
func (r *PostgreSQLChunkRepository) saveEmbeddingToPartitioned(
	ctx context.Context,
	embedding *outbound.Embedding,
) error {
	query := insertPartitionedEmbeddingQuery

	_, err := r.pool.Exec(ctx, query,
		embedding.ID,
		embedding.ChunkID,
		embedding.RepositoryID,
		VectorToString(embedding.Vector),
		embedding.ModelVersion,
	)
	if err != nil {
		slogger.Error(ctx, "Failed to save embedding to partitioned table", slogger.Fields3(
			"embedding_id", embedding.ID.String(),
			"chunk_id", embedding.ChunkID.String(),
			"error", err.Error(),
		))
		return fmt.Errorf("failed to save embedding to partitioned table: %w", err)
	}

	slogger.Debug(ctx, "Embedding saved to partitioned table successfully", slogger.Fields2(
		"embedding_id", embedding.ID.String(),
		"chunk_id", embedding.ChunkID.String(),
	))

	return nil
}

// saveEmbeddingToRegular saves embedding to the regular table.
func (r *PostgreSQLChunkRepository) saveEmbeddingToRegular(ctx context.Context, embedding *outbound.Embedding) error {
	query := `
		INSERT INTO codechunking.embeddings (
			id, chunk_id, embedding, model_version
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (chunk_id, model_version)
		DO UPDATE SET
			embedding = EXCLUDED.embedding,
			created_at = CURRENT_TIMESTAMP
	`

	_, err := r.pool.Exec(ctx, query,
		embedding.ID,
		embedding.ChunkID,
		VectorToString(embedding.Vector),
		embedding.ModelVersion,
	)
	if err != nil {
		slogger.Error(ctx, "Failed to save embedding to regular table", slogger.Fields3(
			"embedding_id", embedding.ID.String(),
			"chunk_id", embedding.ChunkID.String(),
			"error", err.Error(),
		))
		return fmt.Errorf("failed to save embedding to regular table: %w", err)
	}

	slogger.Debug(ctx, "Embedding saved to regular table successfully", slogger.Fields2(
		"embedding_id", embedding.ID.String(),
		"chunk_id", embedding.ChunkID.String(),
	))

	return nil
}

// SaveEmbeddings stores multiple embeddings in a batch operation.
func (r *PostgreSQLChunkRepository) SaveEmbeddings(ctx context.Context, embeddings []outbound.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		slogger.Error(ctx, "Failed to begin transaction for batch embedding save", slogger.Field("error", err.Error()))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slogger.Warn(ctx, "Failed to rollback transaction", slogger.Field("error", err.Error()))
		}
	}()

	// Try partitioned table first
	err = r.saveEmbeddingsBatchToPartitioned(ctx, tx, embeddings)
	if err != nil {
		slogger.Warn(
			ctx,
			"Failed to save batch to partitioned table, falling back to regular table",
			slogger.Field("error", err.Error()),
		)
		// Rollback and retry with regular table
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			slogger.Error(ctx, "Failed to rollback transaction for retry", slogger.Field("error", rbErr.Error()))
		}

		return r.saveEmbeddingsBatchToRegular(ctx, embeddings)
	}

	if err := tx.Commit(ctx); err != nil {
		slogger.Error(ctx, "Failed to commit batch embedding save", slogger.Fields2(
			"embedding_count", len(embeddings),
			"error", err.Error(),
		))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slogger.Info(ctx, "Batch embeddings saved successfully", slogger.Field("embedding_count", len(embeddings)))
	return nil
}

// saveEmbeddingsBatchToPartitioned saves embeddings batch to partitioned table.
func (r *PostgreSQLChunkRepository) saveEmbeddingsBatchToPartitioned(
	ctx context.Context,
	tx pgx.Tx,
	embeddings []outbound.Embedding,
) error {
	query := insertPartitionedEmbeddingQuery

	for _, embedding := range embeddings {
		_, err := tx.Exec(ctx, query,
			embedding.ID,
			embedding.ChunkID,
			embedding.RepositoryID,
			VectorToString(embedding.Vector),
			embedding.ModelVersion,
		)
		if err != nil {
			return fmt.Errorf("failed to save embedding in batch to partitioned table: %w", err)
		}
	}

	return nil
}

// saveEmbeddingsBatchToRegular saves embeddings batch to regular table.
func (r *PostgreSQLChunkRepository) saveEmbeddingsBatchToRegular(
	ctx context.Context,
	embeddings []outbound.Embedding,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for regular table batch: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slogger.Warn(ctx, "Failed to rollback regular table batch transaction", slogger.Field("error", err.Error()))
		}
	}()

	query := `
		INSERT INTO codechunking.embeddings (
			id, chunk_id, embedding, model_version
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (chunk_id, model_version)
		DO UPDATE SET
			embedding = EXCLUDED.embedding,
			created_at = CURRENT_TIMESTAMP
	`

	for _, embedding := range embeddings {
		_, err := tx.Exec(ctx, query,
			embedding.ID,
			embedding.ChunkID,
			VectorToString(embedding.Vector),
			embedding.ModelVersion,
		)
		if err != nil {
			return fmt.Errorf("failed to save embedding in batch to regular table: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit regular table batch transaction: %w", err)
	}

	return nil
}

// GetEmbedding retrieves an embedding by chunk ID.
func (r *PostgreSQLChunkRepository) GetEmbedding(ctx context.Context, chunkID uuid.UUID) (*outbound.Embedding, error) {
	// Try partitioned table first
	embedding, err := r.getEmbeddingFromPartitioned(ctx, chunkID)
	if err == nil {
		return embedding, nil
	}

	// If not found in partitioned table, try regular table
	embedding, err = r.getEmbeddingFromRegular(ctx, chunkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slogger.Debug(ctx, "Embedding not found", slogger.Field("chunk_id", chunkID.String()))
			return nil, errors.New("embedding not found")
		}
		return nil, err
	}

	return embedding, nil
}

// getEmbeddingFromPartitioned retrieves embedding from partitioned table.
func (r *PostgreSQLChunkRepository) getEmbeddingFromPartitioned(
	ctx context.Context,
	chunkID uuid.UUID,
) (*outbound.Embedding, error) {
	query := `
		SELECT id, chunk_id, repository_id, embedding, model_version, created_at
		FROM codechunking.embeddings_partitioned
		WHERE chunk_id = $1 AND deleted_at IS NULL
		LIMIT 1
	`

	var embedding outbound.Embedding
	var vector pq.Float64Array
	var createdAt time.Time

	err := r.pool.QueryRow(ctx, query, chunkID).Scan(
		&embedding.ID,
		&embedding.ChunkID,
		&embedding.RepositoryID,
		&vector,
		&embedding.ModelVersion,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	embedding.Vector = []float64(vector)
	embedding.CreatedAt = createdAt.Format(time.RFC3339)

	return &embedding, nil
}

// getEmbeddingFromRegular retrieves embedding from regular table.
func (r *PostgreSQLChunkRepository) getEmbeddingFromRegular(
	ctx context.Context,
	chunkID uuid.UUID,
) (*outbound.Embedding, error) {
	query := `
		SELECT id, chunk_id, embedding, model_version, created_at
		FROM codechunking.embeddings
		WHERE chunk_id = $1 AND deleted_at IS NULL
		LIMIT 1
	`

	var embedding outbound.Embedding
	var vector pq.Float64Array
	var createdAt time.Time

	err := r.pool.QueryRow(ctx, query, chunkID).Scan(
		&embedding.ID,
		&embedding.ChunkID,
		&vector,
		&embedding.ModelVersion,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	embedding.Vector = []float64(vector)
	embedding.CreatedAt = createdAt.Format(time.RFC3339)
	// Regular table doesn't have repository_id, so leave it as zero value

	return &embedding, nil
}

// GetEmbeddingsForRepository retrieves all embeddings for a repository.
func (r *PostgreSQLChunkRepository) GetEmbeddingsForRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
) ([]outbound.Embedding, error) {
	// Try partitioned table first (has repository_id)
	embeddings, err := r.getEmbeddingsForRepositoryFromPartitioned(ctx, repositoryID)
	if err == nil && len(embeddings) > 0 {
		return embeddings, nil
	}

	// Fallback to regular table (need to join with chunks to get repository_id)
	embeddings, err = r.getEmbeddingsForRepositoryFromRegular(ctx, repositoryID)
	if err != nil {
		slogger.Error(ctx, "Failed to get embeddings for repository", slogger.Fields2(
			"repository_id", repositoryID.String(),
			"error", err.Error(),
		))
		return nil, fmt.Errorf("failed to get embeddings for repository: %w", err)
	}

	return embeddings, nil
}

// getEmbeddingsForRepositoryFromPartitioned gets embeddings from partitioned table.
func (r *PostgreSQLChunkRepository) getEmbeddingsForRepositoryFromPartitioned(
	ctx context.Context,
	repositoryID uuid.UUID,
) ([]outbound.Embedding, error) {
	query := `
		SELECT id, chunk_id, repository_id, embedding, model_version, created_at
		FROM codechunking.embeddings_partitioned
		WHERE repository_id = $1 AND deleted_at IS NULL
		ORDER BY created_at
	`

	rows, err := r.pool.Query(ctx, query, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to query partitioned embeddings: %w", err)
	}
	defer rows.Close()

	var embeddings []outbound.Embedding
	for rows.Next() {
		var embedding outbound.Embedding
		var vector pq.Float64Array
		var createdAt time.Time

		err := rows.Scan(
			&embedding.ID,
			&embedding.ChunkID,
			&embedding.RepositoryID,
			&vector,
			&embedding.ModelVersion,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan partitioned embedding row: %w", err)
		}

		embedding.Vector = []float64(vector)
		embedding.CreatedAt = createdAt.Format(time.RFC3339)
		embeddings = append(embeddings, embedding)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating partitioned embedding rows: %w", err)
	}

	return embeddings, nil
}

// getEmbeddingsForRepositoryFromRegular gets embeddings from regular table via join.
func (r *PostgreSQLChunkRepository) getEmbeddingsForRepositoryFromRegular(
	ctx context.Context,
	repositoryID uuid.UUID,
) ([]outbound.Embedding, error) {
	query := `
		SELECT e.id, e.chunk_id, e.embedding, e.model_version, e.created_at
		FROM codechunking.embeddings e
		JOIN codechunking.code_chunks c ON e.chunk_id = c.id
		WHERE c.repository_id = $1 AND e.deleted_at IS NULL AND c.deleted_at IS NULL
		ORDER BY e.created_at
	`

	rows, err := r.pool.Query(ctx, query, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to query regular embeddings: %w", err)
	}
	defer rows.Close()

	var embeddings []outbound.Embedding
	for rows.Next() {
		var embedding outbound.Embedding
		var vector pq.Float64Array
		var createdAt time.Time

		err := rows.Scan(
			&embedding.ID,
			&embedding.ChunkID,
			&vector,
			&embedding.ModelVersion,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan regular embedding row: %w", err)
		}

		embedding.Vector = []float64(vector)
		embedding.CreatedAt = createdAt.Format(time.RFC3339)
		embedding.RepositoryID = repositoryID // Set from parameter since regular table doesn't have it
		embeddings = append(embeddings, embedding)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating regular embedding rows: %w", err)
	}

	return embeddings, nil
}

// DeleteEmbeddingsForRepository deletes all embeddings for a repository.
func (r *PostgreSQLChunkRepository) DeleteEmbeddingsForRepository(ctx context.Context, repositoryID uuid.UUID) error {
	// Delete from both tables to be safe
	var deletedCount int64

	// Delete from partitioned table
	partitionedQuery := `
		UPDATE codechunking.embeddings_partitioned
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE repository_id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, partitionedQuery, repositoryID)
	if err != nil {
		slogger.Warn(ctx, "Failed to delete from partitioned embeddings table", slogger.Fields2(
			"repository_id", repositoryID.String(),
			"error", err.Error(),
		))
	} else {
		deletedCount += result.RowsAffected()
	}

	// Delete from regular table (use JOIN to find matching embeddings)
	regularQuery := `
		UPDATE codechunking.embeddings e
		SET deleted_at = CURRENT_TIMESTAMP
		FROM codechunking.code_chunks c
		WHERE e.chunk_id = c.id AND c.repository_id = $1 AND e.deleted_at IS NULL
	`

	result, err = r.pool.Exec(ctx, regularQuery, repositoryID)
	if err != nil {
		slogger.Error(ctx, "Failed to delete embeddings from regular table", slogger.Fields2(
			"repository_id", repositoryID.String(),
			"error", err.Error(),
		))
		return fmt.Errorf("failed to delete embeddings from regular table: %w", err)
	}

	deletedCount += result.RowsAffected()

	slogger.Info(ctx, "Deleted embeddings for repository", slogger.Fields2(
		"repository_id", repositoryID.String(),
		"deleted_count", deletedCount,
	))

	return nil
}

// SearchSimilar performs vector similarity search.
func (r *PostgreSQLChunkRepository) SearchSimilar(
	ctx context.Context,
	query []float64,
	limit int,
	threshold float64,
) ([]outbound.EmbeddingSearchResult, error) {
	// Try partitioned table first for better performance
	results, err := r.searchSimilarInPartitioned(ctx, query, limit, threshold)
	if err == nil && len(results) > 0 {
		return results, nil
	}

	// Fallback to regular table
	results, err = r.searchSimilarInRegular(ctx, query, limit, threshold)
	if err != nil {
		slogger.Error(ctx, "Failed to perform similarity search", slogger.Fields3(
			"query_length", len(query),
			"limit", limit,
			"error", err.Error(),
		))
		return nil, fmt.Errorf("failed to perform similarity search: %w", err)
	}

	slogger.Debug(ctx, "Similarity search completed", slogger.Fields3(
		"query_length", len(query),
		"limit", limit,
		"results_count", len(results),
	))

	return results, nil
}

// searchSimilarInPartitioned performs similarity search in partitioned table.
func (r *PostgreSQLChunkRepository) searchSimilarInPartitioned(
	ctx context.Context,
	query []float64,
	limit int,
	threshold float64,
) ([]outbound.EmbeddingSearchResult, error) {
	sqlQuery := `
		SELECT
			ep.chunk_id,
			1 - (ep.embedding <=> $1::vector) as similarity,
			c.id,
			c.file_path,
			c.start_line,
			c.end_line,
			c.content,
			c.language,
			c.content_hash,
			c.created_at
		FROM codechunking.embeddings_partitioned ep
		JOIN codechunking.code_chunks c ON ep.chunk_id = c.id
		WHERE (1 - (ep.embedding <=> $1::vector)) > $2
			AND ep.deleted_at IS NULL
			AND c.deleted_at IS NULL
		ORDER BY similarity DESC
		LIMIT $3
	`

	return r.executeSimilaritySearch(ctx, sqlQuery, query, threshold, limit, "partitioned")
}

// searchSimilarInRegular performs similarity search in regular table.
func (r *PostgreSQLChunkRepository) searchSimilarInRegular(
	ctx context.Context,
	query []float64,
	limit int,
	threshold float64,
) ([]outbound.EmbeddingSearchResult, error) {
	sqlQuery := `
		SELECT
			e.chunk_id,
			1 - (e.embedding <=> $1::vector) as similarity,
			c.id,
			c.file_path,
			c.start_line,
			c.end_line,
			c.content,
			c.language,
			c.content_hash,
			c.created_at
		FROM codechunking.embeddings e
		JOIN codechunking.code_chunks c ON e.chunk_id = c.id
		WHERE (1 - (e.embedding <=> $1::vector)) > $2
			AND e.deleted_at IS NULL
			AND c.deleted_at IS NULL
		ORDER BY similarity DESC
		LIMIT $3
	`

	return r.executeSimilaritySearch(ctx, sqlQuery, query, threshold, limit, "regular")
}

// executeSimilaritySearch executes the similarity search query and processes results.
func (r *PostgreSQLChunkRepository) executeSimilaritySearch(
	ctx context.Context,
	sqlQuery string,
	query []float64,
	threshold float64,
	limit int,
	tableType string,
) ([]outbound.EmbeddingSearchResult, error) {
	rows, err := r.pool.Query(ctx, sqlQuery, VectorToString(query), threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute %s similarity search: %w", tableType, err)
	}
	defer rows.Close()

	var results []outbound.EmbeddingSearchResult
	for rows.Next() {
		var result outbound.EmbeddingSearchResult
		var chunk outbound.CodeChunk
		var createdAt time.Time

		err := rows.Scan(
			&result.ChunkID,
			&result.Similarity,
			&chunk.ID,
			&chunk.FilePath,
			&chunk.StartLine,
			&chunk.EndLine,
			&chunk.Content,
			&chunk.Language,
			&chunk.Hash,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s similarity search result: %w", tableType, err)
		}

		chunk.CreatedAt = createdAt
		chunk.Size = len(chunk.Content)
		result.Chunk = &chunk

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating %s similarity search results: %w", tableType, err)
	}

	return results, nil
}

// ============================================================================
// ChunkStorageRepository Interface Implementation (Transactional Methods)
// ============================================================================

// chunkFieldsWithDefaults holds chunk fields with defaults applied.
type chunkFieldsWithDefaults struct {
	EntityName    string
	ParentEntity  string
	ChunkType     string
	QualifiedName string
	Signature     string
	Visibility    string
	RepositoryID  uuid.UUID
}

// prepareChunkFields applies default values to chunk fields.
func prepareChunkFields(chunk *outbound.CodeChunk, embedding *outbound.Embedding) (chunkFieldsWithDefaults, error) {
	fields := chunkFieldsWithDefaults{
		EntityName:    chunk.EntityName,
		ParentEntity:  chunk.ParentEntity,
		ChunkType:     chunk.Type,
		QualifiedName: chunk.QualifiedName,
		Signature:     chunk.Signature,
		Visibility:    chunk.Visibility,
	}

	// Apply defaults
	if fields.EntityName == "" {
		fields.EntityName = defaultEntityName
	}
	if fields.ParentEntity == "" {
		fields.ParentEntity = defaultParentEntity
	}
	if fields.ChunkType == "" {
		fields.ChunkType = defaultChunkType
	}
	if fields.QualifiedName == "" {
		fields.QualifiedName = defaultQualifiedName
	}
	if fields.Signature == "" {
		fields.Signature = defaultSignature
	}
	if fields.Visibility == "" {
		fields.Visibility = defaultVisibility
	}

	// Determine repository ID
	fields.RepositoryID = chunk.RepositoryID
	if fields.RepositoryID == uuid.Nil {
		fields.RepositoryID = embedding.RepositoryID
	}
	if fields.RepositoryID == uuid.Nil {
		return fields, errors.New("repository_id is required")
	}

	// Ensure embedding has repository ID for partitioned table
	if embedding.RepositoryID == uuid.Nil {
		embedding.RepositoryID = fields.RepositoryID
	}

	return fields, nil
}

// SaveChunkWithEmbedding stores both chunk and embedding in a single transaction.
func (r *PostgreSQLChunkRepository) SaveChunkWithEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	embedding *outbound.Embedding,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		slogger.Error(ctx, "Failed to begin transaction for chunk with embedding", slogger.Field("error", err.Error()))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slogger.Warn(ctx, "Failed to rollback transaction", slogger.Field("error", err.Error()))
		}
	}()

	chunkID, err := uuid.Parse(chunk.ID)
	if err != nil {
		slogger.Error(ctx, "Invalid chunk ID in transactional save", slogger.Fields2(
			"chunk_id", chunk.ID,
			"error", err.Error(),
		))
		return fmt.Errorf("invalid chunk ID format: %w", err)
	}

	fields, err := prepareChunkFields(chunk, embedding)
	if err != nil {
		slogger.Error(ctx, "Missing repository_id for transactional chunk save", slogger.Fields{
			"chunk_id":     chunk.ID,
			"embedding_id": embedding.ID.String(),
		})
		return fmt.Errorf("failed to prepare chunk fields: %w", err)
	}

	// Save chunk
	_, err = tx.Exec(ctx, insertChunkQuery,
		chunkID,
		fields.RepositoryID,
		chunk.FilePath,
		fields.ChunkType,
		chunk.Content,
		chunk.Language,
		chunk.StartLine,
		chunk.EndLine,
		fields.EntityName,
		fields.ParentEntity,
		chunk.Hash,
		nil, // metadata
		fields.QualifiedName,
		fields.Signature,
		fields.Visibility,
	)
	if err != nil {
		slogger.Error(ctx, "Failed to save chunk in transaction", slogger.Fields{
			"chunk_id":      chunk.ID,
			"repository_id": fields.RepositoryID.String(),
			"error":         err.Error(),
		})
		return fmt.Errorf("failed to save chunk in transaction: %w", err)
	}

	// Save embedding (try partitioned, fallback to regular)
	if err := r.saveEmbeddingInTx(ctx, tx, embedding, chunk.ID, fields.RepositoryID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		slogger.Error(ctx, "Failed to commit chunk with embedding transaction", slogger.Fields2(
			"chunk_id", chunk.ID,
			"error", err.Error(),
		))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slogger.Debug(ctx, "Chunk with embedding saved successfully", slogger.Fields{
		"chunk_id":      chunk.ID,
		"repository_id": fields.RepositoryID.String(),
	})
	return nil
}

// saveEmbeddingInTx saves embedding in a transaction, trying partitioned table first.
func (r *PostgreSQLChunkRepository) saveEmbeddingInTx(
	ctx context.Context,
	tx pgx.Tx,
	embedding *outbound.Embedding,
	chunkID string,
	repositoryID uuid.UUID,
) error {
	// Try partitioned table
	_, err := tx.Exec(ctx, insertPartitionedEmbeddingQuery,
		embedding.ID,
		embedding.ChunkID,
		embedding.RepositoryID,
		VectorToString(embedding.Vector),
		embedding.ModelVersion,
	)
	if err == nil {
		return nil
	}

	// Fallback to regular table
	slogger.Warn(
		ctx,
		"Failed to save embedding to partitioned table in transaction, trying regular table",
		slogger.Field("error", err.Error()),
	)

	_, err = tx.Exec(ctx, insertRegularEmbeddingQuery,
		embedding.ID,
		embedding.ChunkID,
		VectorToString(embedding.Vector),
		embedding.ModelVersion,
	)
	if err != nil {
		slogger.Error(ctx, "Failed to save embedding to regular table in transaction", slogger.Fields{
			"chunk_id":      chunkID,
			"repository_id": repositoryID.String(),
			"error":         err.Error(),
		})
		return fmt.Errorf("failed to save embedding in transaction: %w", err)
	}

	return nil
}

// SaveChunksWithEmbeddings stores multiple chunks and embeddings in a single transaction.
func (r *PostgreSQLChunkRepository) SaveChunksWithEmbeddings(
	ctx context.Context,
	chunks []outbound.CodeChunk,
	embeddings []outbound.Embedding,
) error {
	if len(chunks) == 0 || len(embeddings) == 0 {
		return nil
	}

	if len(chunks) != len(embeddings) {
		return fmt.Errorf(
			"chunks and embeddings count mismatch: %d chunks, %d embeddings",
			len(chunks),
			len(embeddings),
		)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		slogger.Error(
			ctx,
			"Failed to begin transaction for batch chunk with embeddings",
			slogger.Field("error", err.Error()),
		)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slogger.Warn(ctx, "Failed to rollback batch transaction", slogger.Field("error", err.Error()))
		}
	}()

	// Save all chunks first
	chunkQuery := insertChunkQuery

	for i, chunk := range chunks {
		chunkID, err := uuid.Parse(chunk.ID)
		if err != nil {
			slogger.Error(ctx, "Invalid chunk ID in batch transactional save", slogger.Fields2(
				"chunk_id", chunk.ID,
				"error", err.Error(),
			))
			return fmt.Errorf("invalid chunk ID format: %w", err)
		}

		// Use chunk type information or defaults
		entityName := chunk.EntityName
		if entityName == "" {
			entityName = defaultEntityName
		}

		parentEntity := chunk.ParentEntity
		if parentEntity == "" {
			parentEntity = defaultParentEntity
		}

		chunkType := chunk.Type
		if chunkType == "" {
			chunkType = defaultChunkType
		}

		qualifiedName := chunk.QualifiedName
		if qualifiedName == "" {
			qualifiedName = defaultQualifiedName
		}

		signature := chunk.Signature
		if signature == "" {
			signature = defaultSignature
		}

		visibility := chunk.Visibility
		if visibility == "" {
			visibility = defaultVisibility
		}

		// Prefer chunk.RepositoryID; fallback to embedding.RepositoryID
		repositoryID := chunks[i].RepositoryID
		if repositoryID == uuid.Nil {
			repositoryID = embeddings[i].RepositoryID
		}
		if repositoryID == uuid.Nil {
			slogger.Error(ctx, "Missing repository_id for batch transactional chunk save", slogger.Fields{
				"chunk_id":    chunk.ID,
				"batch_index": i,
			})
			return errors.New("repository_id is required to save chunk in batch transaction")
		}
		// Ensure embedding has repository id for partitioned table path
		if embeddings[i].RepositoryID == uuid.Nil {
			embeddings[i].RepositoryID = repositoryID
		}

		_, err = tx.Exec(ctx, chunkQuery,
			chunkID,
			repositoryID,
			chunk.FilePath,
			chunkType,
			chunk.Content,
			chunk.Language,
			chunk.StartLine,
			chunk.EndLine,
			entityName,
			parentEntity,
			chunk.Hash,
			nil, // metadata
			qualifiedName,
			signature,
			visibility,
		)
		if err != nil {
			slogger.Error(ctx, "Failed to save chunk in batch transaction", slogger.Fields{
				"chunk_id":      chunk.ID,
				"batch_index":   i,
				"repository_id": repositoryID.String(),
				"error":         err.Error(),
			})
			return fmt.Errorf("failed to save chunk in batch transaction: %w", err)
		}
	}

	// Try to save all embeddings to partitioned table
	usePartitioned := true
	embeddingQuery := `
		INSERT INTO codechunking.embeddings_partitioned (
			id, chunk_id, repository_id, embedding, model_version
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (chunk_id, model_version, repository_id)
		DO UPDATE SET
			embedding = EXCLUDED.embedding,
			created_at = CURRENT_TIMESTAMP
	`

	for i, embedding := range embeddings {
		_, err = tx.Exec(ctx, embeddingQuery,
			embedding.ID,
			embedding.ChunkID,
			embedding.RepositoryID,
			VectorToString(embedding.Vector),
			embedding.ModelVersion,
		)
		if err != nil {
			usePartitioned = false
			slogger.Warn(
				ctx,
				"Failed to save embedding to partitioned table in batch, switching to regular table",
				slogger.Fields2(
					"batch_index", i,
					"error", err.Error(),
				),
			)
			break
		}
	}

	// If partitioned failed, save all embeddings to regular table
	if !usePartitioned {
		regularEmbeddingQuery := insertRegularEmbeddingQuery

		for i, embedding := range embeddings {
			_, err = tx.Exec(ctx, regularEmbeddingQuery,
				embedding.ID,
				embedding.ChunkID,
				VectorToString(embedding.Vector),
				embedding.ModelVersion,
			)
			if err != nil {
				slogger.Error(ctx, "Failed to save embedding to regular table in batch transaction", slogger.Fields{
					"chunk_id":      chunks[i].ID,
					"batch_index":   i,
					"repository_id": embeddings[i].RepositoryID.String(),
					"error":         err.Error(),
				})
				return fmt.Errorf("failed to save embedding in batch transaction: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slogger.Error(ctx, "Failed to commit batch chunks with embeddings transaction", slogger.Fields2(
			"chunk_count", len(chunks),
			"error", err.Error(),
		))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slogger.Info(ctx, "Batch chunks with embeddings saved successfully", slogger.Fields2(
		"chunk_count", len(chunks),
		"embedding_count", len(embeddings),
	))
	return nil
}
