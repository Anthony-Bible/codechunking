package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLIndexingJobRepository implements the IndexingJobRepository interface
type PostgreSQLIndexingJobRepository struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLIndexingJobRepository creates a new PostgreSQL indexing job repository
func NewPostgreSQLIndexingJobRepository(pool *pgxpool.Pool) *PostgreSQLIndexingJobRepository {
	return &PostgreSQLIndexingJobRepository{
		pool: pool,
	}
}

// Save saves an indexing job to the database
func (r *PostgreSQLIndexingJobRepository) Save(ctx context.Context, job *entity.IndexingJob) error {
	if job == nil {
		return ErrInvalidArgument
	}

	query := `
		INSERT INTO codechunking.indexing_jobs (
			id, repository_id, status, started_at, completed_at, 
			error_message, files_processed, chunks_created, 
			created_at, updated_at, deleted_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)`

	qi := GetQueryInterface(ctx, r.pool)
	_, err := qi.Exec(ctx, query,
		job.ID(),
		job.RepositoryID(),
		job.Status().String(),
		job.StartedAt(),
		job.CompletedAt(),
		job.ErrorMessage(),
		job.FilesProcessed(),
		job.ChunksCreated(),
		job.CreatedAt(),
		job.UpdatedAt(),
		job.DeletedAt(),
	)
	if err != nil {
		return WrapError(err, "save indexing job")
	}

	return nil
}

// FindByID finds an indexing job by its ID
func (r *PostgreSQLIndexingJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.IndexingJob, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidArgument
	}

	query := `
		SELECT id, repository_id, status, started_at, completed_at, 
			   error_message, files_processed, chunks_created, 
			   created_at, updated_at, deleted_at
		FROM codechunking.indexing_jobs 
		WHERE id = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query, id)

	var repositoryID uuid.UUID
	var statusStr string
	var startedAt, completedAt, deletedAt *time.Time
	var errorMessage *string
	var filesProcessed, chunksCreated int
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&id, &repositoryID, &statusStr, &startedAt, &completedAt,
		&errorMessage, &filesProcessed, &chunksCreated,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil
		}
		return nil, WrapError(err, "find indexing job by ID")
	}

	return r.scanIndexingJob(id, repositoryID, statusStr, startedAt, completedAt, errorMessage, filesProcessed, chunksCreated, createdAt, updatedAt, deletedAt)
}

// FindByRepositoryID finds indexing jobs by repository ID with filters
func (r *PostgreSQLIndexingJobRepository) FindByRepositoryID(ctx context.Context, repositoryID uuid.UUID, filters outbound.IndexingJobFilters) ([]*entity.IndexingJob, int, error) {
	if repositoryID == uuid.Nil {
		return nil, 0, ErrInvalidArgument
	}
	if filters.Limit <= 0 {
		return nil, 0, ErrInvalidArgument
	}
	if filters.Offset < 0 {
		return nil, 0, ErrInvalidArgument
	}

	var whereConditions []string
	var args []interface{}
	argIndex := 1

	// Base query with repository filter
	baseQuery := `FROM codechunking.indexing_jobs WHERE repository_id = $1 AND deleted_at IS NULL`
	args = append(args, repositoryID)
	argIndex++

	// Build where clause (currently no additional filters, but structure is ready)
	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = " AND " + strings.Join(whereConditions, " AND ")
	}

	// Count query
	countQuery := "SELECT COUNT(*) " + baseQuery + whereClause
	qi := GetQueryInterface(ctx, r.pool)

	var totalCount int
	err := qi.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, WrapError(err, "count indexing jobs")
	}

	// Data query
	orderBy := "ORDER BY created_at DESC"

	limit := 50 // default
	if filters.Limit > 0 {
		limit = filters.Limit
	}

	offset := 0
	if filters.Offset > 0 {
		offset = filters.Offset
	}

	dataQuery := `SELECT id, repository_id, status, started_at, completed_at, 
				  error_message, files_processed, chunks_created, 
				  created_at, updated_at, deleted_at ` +
		baseQuery + whereClause + " " + orderBy +
		fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := qi.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, WrapError(err, "query indexing jobs")
	}
	defer rows.Close()

	var jobs []*entity.IndexingJob
	for rows.Next() {
		var id, repoID uuid.UUID
		var statusStr string
		var startedAt, completedAt, deletedAt *time.Time
		var errorMessage *string
		var filesProcessed, chunksCreated int
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &repoID, &statusStr, &startedAt, &completedAt,
			&errorMessage, &filesProcessed, &chunksCreated,
			&createdAt, &updatedAt, &deletedAt,
		)
		if err != nil {
			return nil, 0, WrapError(err, "scan indexing job row")
		}

		job, err := r.scanIndexingJob(id, repoID, statusStr, startedAt, completedAt, errorMessage, filesProcessed, chunksCreated, createdAt, updatedAt, deletedAt)
		if err != nil {
			return nil, 0, err
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, WrapError(err, "iterate indexing job rows")
	}

	return jobs, totalCount, nil
}

// Update updates an indexing job in the database
func (r *PostgreSQLIndexingJobRepository) Update(ctx context.Context, job *entity.IndexingJob) error {
	if job == nil {
		return ErrInvalidArgument
	}

	query := `
		UPDATE codechunking.indexing_jobs 
		SET repository_id = $2, status = $3, started_at = $4, completed_at = $5, 
			error_message = $6, files_processed = $7, chunks_created = $8, 
			updated_at = $9, deleted_at = $10
		WHERE id = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query,
		job.ID(),
		job.RepositoryID(),
		job.Status().String(),
		job.StartedAt(),
		job.CompletedAt(),
		job.ErrorMessage(),
		job.FilesProcessed(),
		job.ChunksCreated(),
		job.UpdatedAt(),
		job.DeletedAt(),
	)
	if err != nil {
		return WrapError(err, "update indexing job")
	}

	if result.RowsAffected() == 0 {
		return WrapError(ErrNotFound, "update indexing job")
	}

	return nil
}

// Delete soft-deletes an indexing job by setting deleted_at
func (r *PostgreSQLIndexingJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrInvalidArgument
	}

	query := `
		UPDATE codechunking.indexing_jobs 
		SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query, id)
	if err != nil {
		return WrapError(err, "delete indexing job")
	}

	if result.RowsAffected() == 0 {
		return WrapError(ErrNotFound, "delete indexing job")
	}

	return nil
}

// scanIndexingJob is a helper function to convert database row to IndexingJob entity
func (r *PostgreSQLIndexingJobRepository) scanIndexingJob(
	id, repositoryID uuid.UUID, statusStr string, startedAt, completedAt *time.Time,
	errorMessage *string, filesProcessed, chunksCreated int, createdAt, updatedAt time.Time, deletedAt *time.Time,
) (*entity.IndexingJob, error) {
	// Parse status
	status, err := valueobject.NewJobStatus(statusStr)
	if err != nil {
		return nil, fmt.Errorf("invalid job status: %w", err)
	}

	return entity.RestoreIndexingJob(
		id, repositoryID, status, startedAt, completedAt,
		errorMessage, filesProcessed, chunksCreated,
		createdAt, updatedAt, deletedAt,
	), nil
}
