package repository

import (
	"codechunking/internal/domain/entity"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQL query constants to avoid repetition
const (
	batchJobProgressFields = `
		id, repository_id, indexing_job_id, batch_number, total_batches, chunks_processed,
		status, retry_count, next_retry_at, error_message, gemini_batch_job_id, gemini_file_uri,
		batch_request_data, submission_attempts, next_submission_at,
		created_at, updated_at`
	batchJobProgressTable = "codechunking.batch_job_progress"
)

// PostgreSQLBatchProgressRepository implements the BatchProgressRepository interface.
type PostgreSQLBatchProgressRepository struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLBatchProgressRepository creates a new PostgreSQL batch progress repository.
func NewPostgreSQLBatchProgressRepository(pool *pgxpool.Pool) *PostgreSQLBatchProgressRepository {
	return &PostgreSQLBatchProgressRepository{
		pool: pool,
	}
}

// buildSelectQuery builds a SELECT query with optional WHERE and ORDER BY clauses.
func (r *PostgreSQLBatchProgressRepository) buildSelectQuery(whereClause, orderClause string) string {
	query := fmt.Sprintf("SELECT %s FROM %s", batchJobProgressFields, batchJobProgressTable)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	if orderClause != "" {
		query += " ORDER BY " + orderClause
	}
	return query
}

// Save saves a batch job progress to the database.
func (r *PostgreSQLBatchProgressRepository) Save(ctx context.Context, progress *entity.BatchJobProgress) error {
	if progress == nil {
		return ErrInvalidArgument
	}

	query := `
		INSERT INTO codechunking.batch_job_progress (
			id, repository_id, indexing_job_id, batch_number, total_batches, chunks_processed,
			status, retry_count, next_retry_at, error_message, gemini_batch_job_id, gemini_file_uri,
			batch_request_data, submission_attempts, next_submission_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (indexing_job_id, batch_number) DO UPDATE SET
			repository_id = EXCLUDED.repository_id,
			total_batches = EXCLUDED.total_batches,
			chunks_processed = EXCLUDED.chunks_processed,
			status = EXCLUDED.status,
			retry_count = EXCLUDED.retry_count,
			next_retry_at = EXCLUDED.next_retry_at,
			error_message = EXCLUDED.error_message,
			gemini_batch_job_id = EXCLUDED.gemini_batch_job_id,
			gemini_file_uri = EXCLUDED.gemini_file_uri,
			batch_request_data = EXCLUDED.batch_request_data,
			submission_attempts = EXCLUDED.submission_attempts,
			next_submission_at = EXCLUDED.next_submission_at,
			updated_at = EXCLUDED.updated_at`

	qi := GetQueryInterface(ctx, r.pool)
	_, err := qi.Exec(ctx, query,
		progress.ID(),
		progress.RepositoryID(),
		progress.IndexingJobID(),
		progress.BatchNumber(),
		progress.TotalBatches(),
		progress.ChunksProcessed(),
		progress.Status(),
		progress.RetryCount(),
		progress.NextRetryAt(),
		progress.ErrorMessage(),
		progress.GeminiBatchJobID(),
		progress.GeminiFileURI(),
		progress.BatchRequestData(),
		progress.SubmissionAttempts(),
		progress.NextSubmissionAt(),
		progress.CreatedAt(),
		progress.UpdatedAt(),
	)
	if err != nil {
		return WrapError(err, "save batch job progress")
	}

	return nil
}

// GetByJobID retrieves all batch job progress records for a given indexing job ID.
func (r *PostgreSQLBatchProgressRepository) GetByJobID(
	ctx context.Context,
	jobID uuid.UUID,
) ([]*entity.BatchJobProgress, error) {
	if jobID == uuid.Nil {
		return nil, ErrInvalidArgument
	}

	query := r.buildSelectQuery("indexing_job_id = $1", "batch_number ASC")

	qi := GetQueryInterface(ctx, r.pool)
	rows, err := qi.Query(ctx, query, jobID)
	if err != nil {
		return nil, WrapError(err, "get batch job progress by job ID")
	}
	defer rows.Close()

	batches, err := r.scanBatchProgressRows(rows)
	if err != nil {
		return nil, err
	}

	return batches, nil
}

// GetByID retrieves a batch job progress record by its ID.
func (r *PostgreSQLBatchProgressRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*entity.BatchJobProgress, error) {
	query := r.buildSelectQuery("id = $1", "")

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query, id)

	batch, err := r.scanBatchProgress(row)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil //nolint:nilnil // Not found is not an error condition for Get methods
		}
		return nil, WrapError(err, "get batch job progress by ID")
	}

	return batch, nil
}

// UpdateStatus updates the status of a batch job progress record.
func (r *PostgreSQLBatchProgressRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `
		UPDATE codechunking.batch_job_progress
		SET status = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query, id, status)
	if err != nil {
		return WrapError(err, "update batch job progress status")
	}

	if result.RowsAffected() == 0 {
		return WrapError(ErrNotFound, "update batch job progress status")
	}

	return nil
}

// GetNextRetryBatch retrieves the next batch that is ready for retry.
func (r *PostgreSQLBatchProgressRepository) GetNextRetryBatch(ctx context.Context) (*entity.BatchJobProgress, error) {
	query := r.buildSelectQuery(
		"status = '"+entity.StatusRetryScheduled+"' AND next_retry_at <= CURRENT_TIMESTAMP",
		"next_retry_at ASC LIMIT 1",
	)

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query)

	batch, err := r.scanBatchProgress(row)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil //nolint:nilnil // No batch ready for retry is not an error condition
		}
		return nil, WrapError(err, "get next retry batch")
	}

	return batch, nil
}

// GetPendingSubmissionBatch retrieves the next batch ready for submission.
// Uses FOR UPDATE SKIP LOCKED for distributed locking to prevent concurrent workers
// from processing the same batch.
//
// Performance: This query requires a composite index on (status, next_submission_at, created_at)
// for optimal performance under high load.
func (r *PostgreSQLBatchProgressRepository) GetPendingSubmissionBatch(
	ctx context.Context,
) (*entity.BatchJobProgress, error) {
	query := `
		SELECT ` + batchJobProgressFields + `
		FROM ` + batchJobProgressTable + `
		WHERE status = '` + entity.StatusPendingSubmission + `'
		  AND (next_submission_at IS NULL OR next_submission_at <= CURRENT_TIMESTAMP)
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED`

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query)

	batch, err := r.scanBatchProgress(row)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil //nolint:nilnil // No batch ready for submission is not an error condition
		}
		return nil, WrapError(err, "get pending submission batch")
	}

	return batch, nil
}

// GetPendingSubmissionCount returns the count of batches awaiting submission.
func (r *PostgreSQLBatchProgressRepository) GetPendingSubmissionCount(
	ctx context.Context,
) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM ` + batchJobProgressTable + `
		WHERE status = '` + entity.StatusPendingSubmission + `'`

	qi := GetQueryInterface(ctx, r.pool)
	var count int
	err := qi.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, WrapError(err, "get pending submission count")
	}
	return count, nil
}

// MarkCompleted marks a batch as completed with the number of chunks processed.
func (r *PostgreSQLBatchProgressRepository) MarkCompleted(
	ctx context.Context,
	id uuid.UUID,
	chunksProcessed int,
) error {
	query := `
		UPDATE codechunking.batch_job_progress
		SET status = '` + entity.StatusCompleted + `', chunks_processed = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query, id, chunksProcessed)
	if err != nil {
		return WrapError(err, "mark batch job progress completed")
	}

	if result.RowsAffected() == 0 {
		return WrapError(ErrNotFound, "mark batch job progress completed")
	}

	return nil
}

// Delete deletes a batch job progress record by its ID.
func (r *PostgreSQLBatchProgressRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		DELETE FROM codechunking.batch_job_progress
		WHERE id = $1`

	qi := GetQueryInterface(ctx, r.pool)
	_, err := qi.Exec(ctx, query, id)
	if err != nil {
		return WrapError(err, "delete batch job progress")
	}

	return nil
}

// GetPendingGeminiBatches retrieves all batches that are waiting for Gemini API completion.
// These are batches with status='processing' AND gemini_batch_job_id IS NOT NULL.
func (r *PostgreSQLBatchProgressRepository) GetPendingGeminiBatches(
	ctx context.Context,
) ([]*entity.BatchJobProgress, error) {
	query := r.buildSelectQuery(
		"status = '"+entity.StatusProcessing+"' AND gemini_batch_job_id IS NOT NULL",
		"created_at ASC",
	)

	qi := GetQueryInterface(ctx, r.pool)
	rows, err := qi.Query(ctx, query)
	if err != nil {
		return nil, WrapError(err, "get pending gemini batches")
	}
	defer rows.Close()

	batches, err := r.scanBatchProgressRows(rows)
	if err != nil {
		return nil, err
	}

	return batches, nil
}

// scanBatchProgress is a helper to scan a row into a BatchJobProgress entity.
func (r *PostgreSQLBatchProgressRepository) scanBatchProgress(row interface {
	Scan(...interface{}) error
},
) (*entity.BatchJobProgress, error) {
	var id, indexingJobID uuid.UUID
	var repositoryID *uuid.UUID
	var batchNumber, totalBatches, chunksProcessed, retryCount int
	var status string
	var nextRetryAt *time.Time
	var errorMessage *string
	var geminiBatchJobID *string
	var geminiFileURI *string
	var batchRequestData []byte
	var submissionAttempts int
	var nextSubmissionAt *time.Time
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&id, &repositoryID, &indexingJobID, &batchNumber, &totalBatches, &chunksProcessed,
		&status, &retryCount, &nextRetryAt, &errorMessage, &geminiBatchJobID, &geminiFileURI,
		&batchRequestData, &submissionAttempts, &nextSubmissionAt,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	return entity.RestoreBatchJobProgress(
		id, repositoryID, indexingJobID, batchNumber, totalBatches, chunksProcessed,
		status, retryCount, nextRetryAt, errorMessage, geminiBatchJobID, geminiFileURI,
		createdAt, updatedAt,
		batchRequestData, submissionAttempts, nextSubmissionAt,
	), nil
}

// scanBatchProgressRows scans multiple rows into a slice of BatchJobProgress entities.
func (r *PostgreSQLBatchProgressRepository) scanBatchProgressRows(rows interface {
	Next() bool
	Scan(...interface{}) error
	Close()
	Err() error
},
) ([]*entity.BatchJobProgress, error) {
	var batches []*entity.BatchJobProgress

	for rows.Next() {
		batch, err := r.scanBatchProgress(rows)
		if err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}

	if err := rows.Err(); err != nil {
		return nil, WrapError(err, "iterate batch job progress rows")
	}

	if batches == nil {
		return []*entity.BatchJobProgress{}, nil
	}

	return batches, nil
}
