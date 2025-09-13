package repository

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	embeddingsTable            = "codechunking.embeddings"
	embeddingsPartitionedTable = "codechunking.embeddings_partitioned"
)

// vectorToString converts a float64 slice to pgvector string format.
func vectorToString(vector []float64) string {
	if len(vector) == 0 {
		return "[]"
	}

	var sb strings.Builder
	sb.WriteByte('[')
	for i, val := range vector {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
	}
	sb.WriteByte(']')
	return sb.String()
}

// stringToVector converts a pgvector string format to float64 slice.
func stringToVector(vectorStr string) ([]float64, error) {
	if vectorStr == "" || vectorStr == "[]" {
		return []float64{}, nil
	}

	// Remove brackets
	vectorStr = strings.Trim(vectorStr, "[]")
	if vectorStr == "" {
		return []float64{}, nil
	}

	// Split by comma
	parts := strings.Split(vectorStr, ",")
	vector := make([]float64, len(parts))

	for i, part := range parts {
		val, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid vector element %q: %w", part, err)
		}
		vector[i] = val
	}

	return vector, nil
}

// PostgreSQLVectorStorageRepository implements the VectorStorageRepository interface.
type PostgreSQLVectorStorageRepository struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLVectorStorageRepository creates a new PostgreSQL vector storage repository.
func NewPostgreSQLVectorStorageRepository(pool *pgxpool.Pool) *PostgreSQLVectorStorageRepository {
	return &PostgreSQLVectorStorageRepository{
		pool: pool,
	}
}

// BulkInsertEmbeddings inserts multiple embeddings efficiently using batch operations.
func (r *PostgreSQLVectorStorageRepository) BulkInsertEmbeddings(
	ctx context.Context,
	embeddings []outbound.VectorEmbedding,
	options outbound.BulkInsertOptions,
) (*outbound.BulkInsertResult, error) {
	startTime := time.Now()

	// Validate options
	if options.BatchSize <= 0 {
		return nil, errors.New("batch size must be greater than 0")
	}

	// Handle empty embeddings
	if len(embeddings) == 0 {
		return &outbound.BulkInsertResult{
			InsertedCount:    0,
			UpdatedCount:     0,
			SkippedCount:     0,
			FailedCount:      0,
			Duration:         time.Since(startTime),
			BatchCount:       0,
			AverageBatchTime: 0,
			ErrorDetails:     []outbound.BatchError{},
		}, nil
	}

	// Validate vector dimensions if required
	if options.ValidateVectorDims {
		for _, embedding := range embeddings {
			if len(embedding.Embedding) != 768 {
				return nil, fmt.Errorf("invalid vector dimensions: expected 768, got %d", len(embedding.Embedding))
			}
		}
	}

	// Validate repository ID for partitioned table
	if options.UsePartitionedTable {
		for _, embedding := range embeddings {
			if embedding.RepositoryID == uuid.Nil {
				return nil, errors.New("repository_id is required for partitioned table")
			}
		}
	}

	qi := GetQueryInterface(ctx, r.pool)

	tableName := embeddingsTable
	if options.UsePartitionedTable {
		tableName = embeddingsPartitionedTable
	}

	var totalInserted, totalUpdated, totalSkipped, totalFailed int
	var errorDetails []outbound.BatchError
	batchCount := 0

	// Process in batches
	for i := 0; i < len(embeddings); i += options.BatchSize {
		end := i + options.BatchSize
		if end > len(embeddings) {
			end = len(embeddings)
		}

		batch := embeddings[i:end]
		batchCount++

		inserted, updated, skipped, failed, batchErrors := r.processBatch(
			ctx,
			qi,
			batch,
			tableName,
			options,
			batchCount,
		)
		totalInserted += inserted
		totalUpdated += updated
		totalSkipped += skipped
		totalFailed += failed
		errorDetails = append(errorDetails, batchErrors...)
	}

	duration := time.Since(startTime)
	avgBatchTime := duration
	if batchCount > 0 {
		avgBatchTime = duration / time.Duration(batchCount)
	}

	// Check if we have constraint violations with ConflictActionError
	if options.OnConflictAction == outbound.ConflictActionError && totalFailed > 0 {
		for _, errorDetail := range errorDetails {
			if errorDetail.ErrorType == "constraint_violation" {
				return nil, fmt.Errorf("constraint violation occurred: %s", errorDetail.Message)
			}
		}
	}

	return &outbound.BulkInsertResult{
		InsertedCount:    totalInserted,
		UpdatedCount:     totalUpdated,
		SkippedCount:     totalSkipped,
		FailedCount:      totalFailed,
		Duration:         duration,
		BatchCount:       batchCount,
		AverageBatchTime: avgBatchTime,
		ErrorDetails:     errorDetails,
	}, nil
}

// processBatch handles a single batch of embeddings.
func (r *PostgreSQLVectorStorageRepository) processBatch(
	ctx context.Context,
	qi QueryInterface,
	batch []outbound.VectorEmbedding,
	tableName string,
	options outbound.BulkInsertOptions,
	batchIndex int,
) (int, int, int, int, []outbound.BatchError) {
	var inserted, updated, skipped, failed int
	var errors []outbound.BatchError
	var columns string
	var placeholders string

	if options.UsePartitionedTable {
		columns = "(id, chunk_id, repository_id, embedding, model_version, created_at)"
		placeholders = "($1, $2, $3, $4, $5, $6)"
	} else {
		columns = "(id, chunk_id, embedding, model_version, created_at)"
		placeholders = "($1, $2, $3, $4, $5)"
	}

	var conflictClause string
	switch options.OnConflictAction {
	case outbound.ConflictActionDoNothing:
		if options.UsePartitionedTable {
			conflictClause = "ON CONFLICT (chunk_id, model_version, repository_id) DO NOTHING"
		} else {
			conflictClause = "ON CONFLICT (chunk_id, model_version) DO NOTHING"
		}
	case outbound.ConflictActionUpdate:
		if options.UsePartitionedTable {
			conflictClause = "ON CONFLICT (chunk_id, model_version, repository_id) DO UPDATE SET embedding = EXCLUDED.embedding, created_at = EXCLUDED.created_at"
		} else {
			conflictClause = "ON CONFLICT (chunk_id, model_version) DO UPDATE SET embedding = EXCLUDED.embedding, created_at = EXCLUDED.created_at"
		}
	case outbound.ConflictActionError:
		conflictClause = ""
	}

	// Build query with proper RETURNING clause for accurate tracking
	var returningClause string
	if options.OnConflictAction == outbound.ConflictActionUpdate {
		returningClause = " RETURNING (xmax = 0) AS inserted"
	}

	query := fmt.Sprintf("INSERT INTO %s %s VALUES %s", tableName, columns, placeholders)
	if conflictClause != "" {
		query += " " + conflictClause
	}
	query += returningClause

	for _, embedding := range batch {
		vectorStr := vectorToString(embedding.Embedding)
		var args []interface{}
		if options.UsePartitionedTable {
			args = []interface{}{
				embedding.ID,
				embedding.ChunkID,
				embedding.RepositoryID,
				vectorStr,
				embedding.ModelVersion,
				embedding.CreatedAt,
			}
		} else {
			args = []interface{}{
				embedding.ID,
				embedding.ChunkID,
				vectorStr,
				embedding.ModelVersion,
				embedding.CreatedAt,
			}
		}

		// Execute query and handle results based on conflict action
		if options.OnConflictAction == outbound.ConflictActionUpdate && returningClause != "" {
			inserted, updated, failed, errors = r.executeUpsertWithReturning(
				ctx, qi, query, args, batchIndex, inserted, updated, failed, errors,
			)
		} else {
			inserted, skipped, failed, errors = r.executeStandardQuery(
				ctx, qi, query, args, options, batchIndex, inserted, skipped, failed, errors,
			)
		}
	}

	return inserted, updated, skipped, failed, errors
}

// executeUpsertWithReturning handles upsert operations with RETURNING clause.
func (r *PostgreSQLVectorStorageRepository) executeUpsertWithReturning(
	ctx context.Context,
	qi QueryInterface,
	query string,
	args []interface{},
	batchIndex int,
	inserted, updated, failed int,
	errors []outbound.BatchError,
) (int, int, int, []outbound.BatchError) {
	var isInsert bool
	err := qi.QueryRow(ctx, query, args...).Scan(&isInsert)
	if err != nil {
		failed++
		errors = append(errors, outbound.BatchError{
			BatchIndex:  batchIndex,
			ErrorType:   "execution_error",
			ErrorCode:   "unknown",
			Message:     err.Error(),
			RetryCount:  0,
			Recoverable: false,
		})
		return inserted, updated, failed, errors
	}

	if isInsert {
		inserted++
	} else {
		updated++
	}
	return inserted, updated, failed, errors
}

// executeStandardQuery handles standard query execution without RETURNING clause.
func (r *PostgreSQLVectorStorageRepository) executeStandardQuery(
	ctx context.Context,
	qi QueryInterface,
	query string,
	args []interface{},
	options outbound.BulkInsertOptions,
	batchIndex int,
	inserted, skipped, failed int,
	errors []outbound.BatchError,
) (int, int, int, []outbound.BatchError) {
	result, err := qi.Exec(ctx, query, args...)
	if err != nil {
		if options.OnConflictAction == outbound.ConflictActionError && IsConstraintViolationError(err) {
			failed++
			errors = append(errors, outbound.BatchError{
				BatchIndex:  batchIndex,
				ErrorType:   "constraint_violation",
				ErrorCode:   "23505",
				Message:     err.Error(),
				RetryCount:  0,
				Recoverable: false,
			})
			return inserted, skipped, failed, errors
		}
		failed++
		errors = append(errors, outbound.BatchError{
			BatchIndex:  batchIndex,
			ErrorType:   "execution_error",
			ErrorCode:   "unknown",
			Message:     err.Error(),
			RetryCount:  0,
			Recoverable: false,
		})
		return inserted, skipped, failed, errors
	}

	rowsAffected := result.RowsAffected()
	switch options.OnConflictAction {
	case outbound.ConflictActionDoNothing:
		if rowsAffected == 0 {
			skipped++ // Conflict occurred, row was skipped
		} else {
			inserted++ // New row was inserted
		}
	case outbound.ConflictActionUpdate:
		// This case should not reach here as it's handled above with RETURNING
		if rowsAffected > 0 {
			inserted++
		} else {
			skipped++
		}
	case outbound.ConflictActionError:
		// This case should not reach here since errors are handled above
		if rowsAffected > 0 {
			inserted++
		} else {
			skipped++
		}
	}
	return inserted, skipped, failed, errors
}

// UpsertEmbedding inserts or updates a single embedding with conflict resolution.
func (r *PostgreSQLVectorStorageRepository) UpsertEmbedding(
	ctx context.Context,
	embedding outbound.VectorEmbedding,
	options outbound.UpsertOptions,
) error {
	// Validate vector dimensions if required
	if options.ValidateVectorDims && len(embedding.Embedding) != 768 {
		return fmt.Errorf("invalid vector dimensions: expected 768, got %d", len(embedding.Embedding))
	}

	qi := GetQueryInterface(ctx, r.pool)
	vectorStr := vectorToString(embedding.Embedding)

	tableName := embeddingsTable
	var query string
	var args []interface{}

	if options.UsePartitionedTable {
		tableName = embeddingsPartitionedTable
		query = fmt.Sprintf(`
			INSERT INTO %s (id, chunk_id, repository_id, embedding, model_version, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (chunk_id, model_version, repository_id) 
			DO UPDATE SET embedding = EXCLUDED.embedding, created_at = EXCLUDED.created_at
		`, tableName)
		args = []interface{}{
			embedding.ID,
			embedding.ChunkID,
			embedding.RepositoryID,
			vectorStr,
			embedding.ModelVersion,
			embedding.CreatedAt,
		}
	} else {
		query = fmt.Sprintf(`
			INSERT INTO %s (id, chunk_id, embedding, model_version, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (chunk_id, model_version) 
			DO UPDATE SET embedding = EXCLUDED.embedding, created_at = EXCLUDED.created_at
		`, tableName)
		args = []interface{}{
			embedding.ID,
			embedding.ChunkID,
			vectorStr,
			embedding.ModelVersion,
			embedding.CreatedAt,
		}
	}

	_, err := qi.Exec(ctx, query, args...)
	return WrapError(err, "upsert embedding")
}

// FindEmbeddingByChunkID retrieves an embedding by chunk ID and model version.
func (r *PostgreSQLVectorStorageRepository) FindEmbeddingByChunkID(
	ctx context.Context,
	chunkID uuid.UUID,
	modelVersion string,
) (*outbound.VectorEmbedding, error) {
	if chunkID == uuid.Nil {
		return nil, ErrInvalidArgument
	}
	if modelVersion == "" {
		return nil, ErrInvalidArgument
	}

	qi := GetQueryInterface(ctx, r.pool)

	// Try regular table first
	query := `
		SELECT id, chunk_id, embedding, model_version, created_at, deleted_at
		FROM codechunking.embeddings
		WHERE chunk_id = $1 AND model_version = $2 AND deleted_at IS NULL
	`

	var embedding outbound.VectorEmbedding
	var deletedAt *time.Time
	var vectorStr string

	err := qi.QueryRow(ctx, query, chunkID, modelVersion).Scan(
		&embedding.ID,
		&embedding.ChunkID,
		&vectorStr,
		&embedding.ModelVersion,
		&embedding.CreatedAt,
		&deletedAt,
	)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil //nolint:nilnil // Not found is not an error condition for Find methods
		}
		return nil, WrapError(err, "find embedding by chunk ID")
	}

	// Parse the vector string back to float64 slice
	vector, err := stringToVector(vectorStr)
	if err != nil {
		return nil, WrapError(err, "parse vector from database")
	}
	embedding.Embedding = vector
	embedding.DeletedAt = deletedAt
	return &embedding, nil
}

// buildEmbeddingQuery builds the SQL query for finding embeddings by repository ID.
func (r *PostgreSQLVectorStorageRepository) buildEmbeddingQuery(
	repositoryID uuid.UUID,
	filters outbound.EmbeddingFilters,
) (string, []interface{}) {
	whereConditions := []string{"c.repository_id = $1", "e.deleted_at IS NULL"}
	args := []interface{}{repositoryID}
	argIndex := 2

	if len(filters.ModelVersions) > 0 {
		placeholders := make([]string, len(filters.ModelVersions))
		for i, version := range filters.ModelVersions {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, version)
			argIndex++
		}
		whereConditions = append(
			whereConditions,
			fmt.Sprintf("e.model_version IN (%s)", strings.Join(placeholders, ",")),
		)
	}

	whereClause := strings.Join(whereConditions, " AND ")

	var selectClause, fromClause string
	if filters.UsePartitionedTable {
		selectClause = "SELECT e.id, e.chunk_id, e.repository_id, e.embedding, e.model_version, e.created_at, e.deleted_at"
		fromClause = fmt.Sprintf("FROM %s e", embeddingsPartitionedTable)
		whereClause = strings.Replace(whereClause, "c.repository_id", "e.repository_id", 1)
		whereClause = strings.Replace(whereClause, "c ON e.chunk_id = c.id", "", 1)
	} else {
		selectClause = "SELECT e.id, e.chunk_id, e.embedding, e.model_version, e.created_at, e.deleted_at"
		fromClause = fmt.Sprintf("FROM %s e JOIN codechunking.code_chunks c ON e.chunk_id = c.id", embeddingsTable)
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf("%s %s WHERE %s ORDER BY e.created_at DESC LIMIT %d OFFSET %d",
		selectClause, fromClause, whereClause, limit, offset)

	return query, args
}

// scanEmbeddingRow scans a single embedding row from the database.
func (r *PostgreSQLVectorStorageRepository) scanEmbeddingRow(
	rows pgx.Rows,
	repositoryID uuid.UUID,
	usePartitionedTable bool,
) (outbound.VectorEmbedding, error) {
	var embedding outbound.VectorEmbedding
	var deletedAt *time.Time
	var vectorStr string

	if usePartitionedTable {
		err := rows.Scan(
			&embedding.ID,
			&embedding.ChunkID,
			&embedding.RepositoryID,
			&vectorStr,
			&embedding.ModelVersion,
			&embedding.CreatedAt,
			&deletedAt,
		)
		if err != nil {
			return embedding, err
		}
	} else {
		err := rows.Scan(
			&embedding.ID,
			&embedding.ChunkID,
			&vectorStr,
			&embedding.ModelVersion,
			&embedding.CreatedAt,
			&deletedAt,
		)
		if err != nil {
			return embedding, err
		}
		embedding.RepositoryID = repositoryID
	}

	// Parse the vector string back to float64 slice
	vector, err := stringToVector(vectorStr)
	if err != nil {
		return embedding, fmt.Errorf("parse vector from database: %w", err)
	}
	embedding.Embedding = vector
	embedding.DeletedAt = deletedAt
	return embedding, nil
}

// FindEmbeddingsByRepositoryID retrieves all embeddings for a specific repository.
func (r *PostgreSQLVectorStorageRepository) FindEmbeddingsByRepositoryID(
	ctx context.Context,
	repositoryID uuid.UUID,
	filters outbound.EmbeddingFilters,
) ([]outbound.VectorEmbedding, error) {
	if repositoryID == uuid.Nil {
		return nil, ErrInvalidArgument
	}

	qi := GetQueryInterface(ctx, r.pool)
	query, args := r.buildEmbeddingQuery(repositoryID, filters)

	rows, err := qi.Query(ctx, query, args...)
	if err != nil {
		return nil, WrapError(err, "find embeddings by repository ID")
	}
	defer rows.Close()

	var embeddings []outbound.VectorEmbedding
	for rows.Next() {
		embedding, err := r.scanEmbeddingRow(rows, repositoryID, filters.UsePartitionedTable)
		if err != nil {
			return nil, WrapError(err, "scan embedding row")
		}
		embeddings = append(embeddings, embedding)
	}

	if err = rows.Err(); err != nil {
		return nil, WrapError(err, "iterate embedding rows")
	}

	return embeddings, nil
}

// DeleteEmbeddingsByChunkIDs performs bulk deletion of embeddings by chunk IDs.
func (r *PostgreSQLVectorStorageRepository) DeleteEmbeddingsByChunkIDs(
	ctx context.Context,
	chunkIDs []uuid.UUID,
	options outbound.DeleteOptions,
) (*outbound.BulkDeleteResult, error) {
	startTime := time.Now()

	if chunkIDs == nil {
		return nil, ErrInvalidArgument
	}

	if len(chunkIDs) == 0 {
		return &outbound.BulkDeleteResult{
			DeletedCount:     0,
			SkippedCount:     0,
			Duration:         time.Since(startTime),
			BatchCount:       0,
			AverageBatchTime: 0,
		}, nil
	}

	qi := GetQueryInterface(ctx, r.pool)

	tableName := embeddingsTable
	if options.UsePartitionedTable {
		tableName = embeddingsPartitionedTable
	}

	var totalDeleted int
	batchCount := 0
	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	// Process in batches
	for i := 0; i < len(chunkIDs); i += batchSize {
		end := i + batchSize
		if end > len(chunkIDs) {
			end = len(chunkIDs)
		}

		batch := chunkIDs[i:end]
		batchCount++

		placeholders := make([]string, len(batch))
		args := make([]interface{}, len(batch))
		for j, chunkID := range batch {
			placeholders[j] = fmt.Sprintf("$%d", j+1)
			args[j] = chunkID
		}

		var query string
		if options.SoftDelete {
			query = fmt.Sprintf(
				"UPDATE %s SET deleted_at = CURRENT_TIMESTAMP WHERE chunk_id IN (%s) AND deleted_at IS NULL",
				tableName,
				strings.Join(placeholders, ","),
			)
		} else {
			query = fmt.Sprintf("DELETE FROM %s WHERE chunk_id IN (%s)",
				tableName, strings.Join(placeholders, ","))
		}

		result, err := qi.Exec(ctx, query, args...)
		if err != nil {
			return nil, WrapError(err, "delete embeddings by chunk IDs")
		}

		totalDeleted += int(result.RowsAffected())
	}

	duration := time.Since(startTime)
	avgBatchTime := duration
	if batchCount > 0 {
		avgBatchTime = duration / time.Duration(batchCount)
	}

	skipped := len(chunkIDs) - totalDeleted

	return &outbound.BulkDeleteResult{
		DeletedCount:     totalDeleted,
		SkippedCount:     skipped,
		Duration:         duration,
		BatchCount:       batchCount,
		AverageBatchTime: avgBatchTime,
	}, nil
}

// buildDeleteByRepositoryQuery builds the delete query based on options.
func (r *PostgreSQLVectorStorageRepository) buildDeleteByRepositoryQuery(options outbound.DeleteOptions) string {
	if options.UsePartitionedTable {
		if options.SoftDelete {
			return fmt.Sprintf(
				"UPDATE %s SET deleted_at = CURRENT_TIMESTAMP WHERE repository_id = $1 AND deleted_at IS NULL",
				embeddingsPartitionedTable,
			)
		}
		return fmt.Sprintf("DELETE FROM %s WHERE repository_id = $1", embeddingsPartitionedTable)
	}

	if options.SoftDelete {
		return fmt.Sprintf(`UPDATE %s SET deleted_at = CURRENT_TIMESTAMP 
				WHERE chunk_id IN (SELECT id FROM codechunking.code_chunks WHERE repository_id = $1) 
				AND deleted_at IS NULL`, embeddingsTable)
	}
	return fmt.Sprintf(`DELETE FROM %s 
			WHERE chunk_id IN (SELECT id FROM codechunking.code_chunks WHERE repository_id = $1)`, embeddingsTable)
}

// DeleteEmbeddingsByRepositoryID deletes all embeddings for a repository.
func (r *PostgreSQLVectorStorageRepository) DeleteEmbeddingsByRepositoryID(
	ctx context.Context,
	repositoryID uuid.UUID,
	options outbound.DeleteOptions,
) (*outbound.BulkDeleteResult, error) {
	startTime := time.Now()

	if repositoryID == uuid.Nil {
		return nil, ErrInvalidArgument
	}

	qi := GetQueryInterface(ctx, r.pool)
	query := r.buildDeleteByRepositoryQuery(options)

	result, err := qi.Exec(ctx, query, repositoryID)
	if err != nil {
		return nil, WrapError(err, "delete embeddings by repository ID")
	}

	duration := time.Since(startTime)
	deletedCount := int(result.RowsAffected())

	return &outbound.BulkDeleteResult{
		DeletedCount:     deletedCount,
		SkippedCount:     0,
		Duration:         duration,
		BatchCount:       1,
		AverageBatchTime: duration,
	}, nil
}

// VectorSimilaritySearch performs similarity search using vector operations.
func (r *PostgreSQLVectorStorageRepository) VectorSimilaritySearch(
	ctx context.Context,
	queryVector []float64,
	options outbound.SimilaritySearchOptions,
) ([]outbound.VectorSimilarityResult, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector cannot be empty")
	}
	if len(queryVector) != 768 {
		return nil, fmt.Errorf("invalid vector dimensions: expected 768, got %d", len(queryVector))
	}
	if options.MaxResults <= 0 {
		return nil, errors.New("max results must be greater than 0")
	}

	qi := GetQueryInterface(ctx, r.pool)

	tableName := "codechunking.embeddings e"
	selectClause := "SELECT e.id, e.chunk_id, e.embedding, e.model_version, e.created_at, e.deleted_at"
	joinClause := "JOIN codechunking.code_chunks c ON e.chunk_id = c.id"

	if options.UsePartitionedTable {
		tableName = "codechunking.embeddings_partitioned e"
		selectClause = "SELECT e.id, e.chunk_id, e.repository_id, e.embedding, e.model_version, e.created_at, e.deleted_at"
		joinClause = ""
	}

	// Convert query vector to pgvector format
	queryVectorStr := vectorToString(queryVector)

	// Use cosine similarity: 1 - (embedding <=> query_vector)
	similarityClause := "1 - (e.embedding <=> $1) AS similarity"
	distanceClause := "(e.embedding <=> $1) AS distance"

	whereConditions := []string{"e.deleted_at IS NULL"}
	args := []interface{}{queryVectorStr}
	argIndex := 2

	if len(options.RepositoryIDs) > 0 {
		placeholders := make([]string, len(options.RepositoryIDs))
		for i, repoID := range options.RepositoryIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, repoID)
			argIndex++
		}

		if options.UsePartitionedTable {
			whereConditions = append(
				whereConditions,
				fmt.Sprintf("e.repository_id IN (%s)", strings.Join(placeholders, ",")),
			)
		} else {
			whereConditions = append(whereConditions, fmt.Sprintf("c.repository_id IN (%s)", strings.Join(placeholders, ",")))
		}
	}

	if len(options.ModelVersions) > 0 {
		placeholders := make([]string, len(options.ModelVersions))
		for i, version := range options.ModelVersions {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, version)
			argIndex++
		}
		whereConditions = append(
			whereConditions,
			fmt.Sprintf("e.model_version IN (%s)", strings.Join(placeholders, ",")),
		)
	}

	if options.MinSimilarity > 0 {
		whereConditions = append(
			whereConditions,
			fmt.Sprintf("1 - (e.embedding <=> '%s') >= %f", queryVectorStr, options.MinSimilarity),
		)
	}

	whereClause := strings.Join(whereConditions, " AND ")

	var query string
	if options.UsePartitionedTable {
		query = fmt.Sprintf(`
			%s, %s, %s, ROW_NUMBER() OVER (ORDER BY (1 - (e.embedding <=> $1)) DESC) as rank
			FROM %s
			WHERE %s
			ORDER BY (1 - (e.embedding <=> $1)) DESC
			LIMIT %d`,
			selectClause, similarityClause, distanceClause, tableName, whereClause, options.MaxResults)
	} else {
		query = fmt.Sprintf(`
			%s, %s, %s, ROW_NUMBER() OVER (ORDER BY (1 - (e.embedding <=> $1)) DESC) as rank
			FROM %s %s
			WHERE %s
			ORDER BY (1 - (e.embedding <=> $1)) DESC
			LIMIT %d`,
			selectClause, similarityClause, distanceClause, tableName, joinClause, whereClause, options.MaxResults)
	}

	rows, err := qi.Query(ctx, query, args...)
	if err != nil {
		return nil, WrapError(err, "vector similarity search")
	}
	defer rows.Close()

	var results []outbound.VectorSimilarityResult
	for rows.Next() {
		var result outbound.VectorSimilarityResult
		var deletedAt *time.Time
		var vectorStr string

		if options.UsePartitionedTable {
			err = rows.Scan(
				&result.Embedding.ID,
				&result.Embedding.ChunkID,
				&result.Embedding.RepositoryID,
				&vectorStr,
				&result.Embedding.ModelVersion,
				&result.Embedding.CreatedAt,
				&deletedAt,
				&result.Similarity,
				&result.Distance,
				&result.Rank,
			)
		} else {
			err = rows.Scan(
				&result.Embedding.ID,
				&result.Embedding.ChunkID,
				&vectorStr,
				&result.Embedding.ModelVersion,
				&result.Embedding.CreatedAt,
				&deletedAt,
				&result.Similarity,
				&result.Distance,
				&result.Rank,
			)
		}

		if err != nil {
			return nil, WrapError(err, "scan similarity search result")
		}

		// Parse the vector string back to float64 slice
		vector, err := stringToVector(vectorStr)
		if err != nil {
			return nil, WrapError(err, "parse vector from similarity search")
		}
		result.Embedding.Embedding = vector
		result.Embedding.DeletedAt = deletedAt
		results = append(results, result)
	}

	if err = rows.Err(); err != nil {
		return nil, WrapError(err, "iterate similarity search results")
	}

	return results, nil
}

// addPartitionedTableStats adds partitioned table statistics to the stats object.
func (r *PostgreSQLVectorStorageRepository) addPartitionedTableStats(
	ctx context.Context,
	qi QueryInterface,
	stats *outbound.StorageStatistics,
	includePartitions bool,
) {
	partitionedStats, err := r.getTableStatistics(ctx, qi, "embeddings_partitioned")
	if err != nil {
		slogger.Error(ctx, "Failed to get partitioned table statistics", slogger.Fields{"error": err.Error()})
	} else {
		stats.PartitionedTable = partitionedStats
	}

	if includePartitions {
		partitions, err := r.getPartitionStatistics(ctx, qi)
		if err != nil {
			slogger.Error(ctx, "Failed to get partition statistics", slogger.Fields{"error": err.Error()})
		} else {
			stats.Partitions = partitions
		}
	}
}

// GetStorageStatistics returns storage statistics for monitoring and optimization.
func (r *PostgreSQLVectorStorageRepository) GetStorageStatistics(
	ctx context.Context,
	options outbound.StatisticsOptions,
) (*outbound.StorageStatistics, error) {
	if !options.IncludeRegular && !options.IncludePartitioned {
		return nil, errors.New("at least one table type must be included in statistics")
	}

	qi := GetQueryInterface(ctx, r.pool)
	stats := &outbound.StorageStatistics{
		LastUpdated: time.Now(),
	}

	if options.IncludeRegular {
		regularStats, err := r.getTableStatistics(ctx, qi, "embeddings")
		if err != nil {
			slogger.Error(ctx, "Failed to get regular table statistics", slogger.Fields{"error": err.Error()})
		} else {
			stats.RegularTable = regularStats
		}
	}

	if options.IncludePartitioned {
		r.addPartitionedTableStats(ctx, qi, stats, options.IncludePartitions)
	}

	// Calculate totals
	if stats.RegularTable != nil {
		stats.TotalEmbeddings += stats.RegularTable.RowCount
		stats.TotalSize += stats.RegularTable.TableSize + stats.RegularTable.IndexSize
	}
	if stats.PartitionedTable != nil {
		stats.TotalEmbeddings += stats.PartitionedTable.RowCount
		stats.TotalSize += stats.PartitionedTable.TableSize + stats.PartitionedTable.IndexSize
	}

	return stats, nil
}

// getTableStatistics retrieves statistics for a single table.
func (r *PostgreSQLVectorStorageRepository) getTableStatistics(
	ctx context.Context,
	qi QueryInterface,
	tableName string,
) (*outbound.TableStatistics, error) {
	query := `
		SELECT 
			relname as full_table_name,
			n_tup_ins as tuple_inserts,
			n_tup_upd as tuple_updates,
			n_tup_del as tuple_deletes,
			seq_scan as seq_scans,
			COALESCE(idx_scan, 0) as index_scans,
			last_vacuum,
			last_analyze
		FROM pg_stat_user_tables 
		WHERE schemaname = 'codechunking' AND relname = $1
	`

	var stats outbound.TableStatistics
	var lastVacuum, lastAnalyze sql.NullTime

	err := qi.QueryRow(ctx, query, tableName).Scan(
		&stats.TableName,
		&stats.TupleInserts,
		&stats.TupleUpdates,
		&stats.TupleDeletes,
		&stats.SeqScans,
		&stats.IndexScans,
		&lastVacuum,
		&lastAnalyze,
	)
	if err != nil {
		if IsNotFoundError(err) {
			// Table doesn't exist, return minimal stats
			return &outbound.TableStatistics{
				TableName: tableName,
			}, nil
		}
		return nil, WrapError(err, "get table statistics")
	}

	if lastVacuum.Valid {
		stats.LastVacuum = &lastVacuum.Time
	}
	if lastAnalyze.Valid {
		stats.LastAnalyze = &lastAnalyze.Time
	}

	// Get size information
	sizeQuery := `
		SELECT 
			pg_total_relation_size(c.oid) as total_size,
			pg_relation_size(c.oid) as table_size,
			pg_total_relation_size(c.oid) - pg_relation_size(c.oid) as index_size,
			GREATEST(COALESCE(c.reltuples, 0)::bigint, 0) as row_count
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1 AND n.nspname = 'codechunking' AND c.relkind = 'r'
	`

	var totalSize int64
	err = qi.QueryRow(ctx, sizeQuery, tableName).Scan(
		&totalSize,
		&stats.TableSize,
		&stats.IndexSize,
		&stats.RowCount,
	)

	if err != nil && !IsNotFoundError(err) {
		return nil, WrapError(err, "get table size statistics")
	}

	return &stats, nil
}

// getPartitionStatistics retrieves statistics for all partitions.
func (r *PostgreSQLVectorStorageRepository) getPartitionStatistics(
	ctx context.Context,
	qi QueryInterface,
) ([]outbound.PartitionStatistics, error) {
	query := `
		SELECT 
			relname as partition_name,
			last_vacuum,
			last_analyze
		FROM pg_stat_user_tables 
		WHERE schemaname = 'codechunking' 
		AND relname LIKE 'embeddings_partitioned_%'
	`

	rows, err := qi.Query(ctx, query)
	if err != nil {
		return nil, WrapError(err, "get partition statistics")
	}
	defer rows.Close()

	var partitions []outbound.PartitionStatistics
	for rows.Next() {
		var partition outbound.PartitionStatistics
		var lastVacuum, lastAnalyze sql.NullTime

		err = rows.Scan(
			&partition.PartitionName,
			&lastVacuum,
			&lastAnalyze,
		)
		if err != nil {
			return nil, WrapError(err, "scan partition statistics")
		}

		if lastVacuum.Valid {
			partition.LastVacuum = &lastVacuum.Time
		}
		if lastAnalyze.Valid {
			partition.LastAnalyze = &lastAnalyze.Time
		}

		// Get size for this partition
		tableName := partition.PartitionName
		sizeQuery := `
			SELECT 
				pg_relation_size(c.oid) as table_size,
				pg_total_relation_size(c.oid) - pg_relation_size(c.oid) as index_size,
				GREATEST(COALESCE(c.reltuples, 0)::bigint, 0) as row_count
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relname = $1 AND n.nspname = 'codechunking' AND c.relkind = 'r'
		`

		err = qi.QueryRow(ctx, sizeQuery, tableName).Scan(
			&partition.TableSize,
			&partition.IndexSize,
			&partition.RowCount,
		)

		if err != nil && !IsNotFoundError(err) {
			slogger.Error(ctx, "Failed to get partition size", slogger.Fields{
				"partition": partition.PartitionName,
				"error":     err,
			})
		}

		partitions = append(partitions, partition)
	}

	if err = rows.Err(); err != nil {
		return nil, WrapError(err, "iterate partition statistics")
	}

	return partitions, nil
}

// BeginTransaction starts a new transaction for atomic vector operations.
func (r *PostgreSQLVectorStorageRepository) BeginTransaction(ctx context.Context) (outbound.VectorTransaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, WrapError(err, "begin transaction")
	}

	return &VectorStorageTransaction{
		tx:   tx,
		repo: r,
	}, nil
}

// VectorStorageTransaction implements the VectorTransaction interface.
type VectorStorageTransaction struct {
	tx   pgx.Tx
	repo *PostgreSQLVectorStorageRepository
}

// BulkInsertEmbeddings performs bulk insert within transaction context.
func (t *VectorStorageTransaction) BulkInsertEmbeddings(
	ctx context.Context,
	embeddings []outbound.VectorEmbedding,
	options outbound.BulkInsertOptions,
) (*outbound.BulkInsertResult, error) {
	// Create a context with the transaction
	txCtx := context.WithValue(ctx, txContextKey{}, t.tx)
	return t.repo.BulkInsertEmbeddings(txCtx, embeddings, options)
}

// UpsertEmbedding performs upsert within transaction context.
func (t *VectorStorageTransaction) UpsertEmbedding(
	ctx context.Context,
	embedding outbound.VectorEmbedding,
	options outbound.UpsertOptions,
) error {
	// Create a context with the transaction
	txCtx := context.WithValue(ctx, txContextKey{}, t.tx)
	return t.repo.UpsertEmbedding(txCtx, embedding, options)
}

// DeleteEmbeddingsByChunkIDs performs bulk delete within transaction context.
func (t *VectorStorageTransaction) DeleteEmbeddingsByChunkIDs(
	ctx context.Context,
	chunkIDs []uuid.UUID,
	options outbound.DeleteOptions,
) (*outbound.BulkDeleteResult, error) {
	// Create a context with the transaction
	txCtx := context.WithValue(ctx, txContextKey{}, t.tx)
	return t.repo.DeleteEmbeddingsByChunkIDs(txCtx, chunkIDs, options)
}

// Commit commits the transaction and makes all changes permanent.
func (t *VectorStorageTransaction) Commit(ctx context.Context) error {
	return WrapError(t.tx.Commit(ctx), "commit transaction")
}

// Rollback rolls back the transaction and undoes all changes.
func (t *VectorStorageTransaction) Rollback(ctx context.Context) error {
	return WrapError(t.tx.Rollback(ctx), "rollback transaction")
}
