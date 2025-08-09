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

// PostgreSQLRepositoryRepository implements the RepositoryRepository interface
type PostgreSQLRepositoryRepository struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLRepositoryRepository creates a new PostgreSQL repository repository
func NewPostgreSQLRepositoryRepository(pool *pgxpool.Pool) *PostgreSQLRepositoryRepository {
	return &PostgreSQLRepositoryRepository{
		pool: pool,
	}
}

// Save saves a repository to the database
func (r *PostgreSQLRepositoryRepository) Save(ctx context.Context, repository *entity.Repository) error {
	if repository == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	query := `
		INSERT INTO codechunking.repositories (
			id, url, name, description, default_branch, last_indexed_at, 
			last_commit_hash, total_files, total_chunks, status, 
			created_at, updated_at, deleted_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)`

	qi := GetQueryInterface(ctx, r.pool)
	_, err := qi.Exec(ctx, query,
		repository.ID(),
		repository.URL().String(),
		repository.Name(),
		repository.Description(),
		repository.DefaultBranch(),
		repository.LastIndexedAt(),
		repository.LastCommitHash(),
		repository.TotalFiles(),
		repository.TotalChunks(),
		repository.Status().String(),
		repository.CreatedAt(),
		repository.UpdatedAt(),
		repository.DeletedAt(),
	)
	if err != nil {
		return WrapError(err, "save repository")
	}

	return nil
}

// FindByID finds a repository by its ID
func (r *PostgreSQLRepositoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Repository, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("id cannot be nil")
	}

	query := `
		SELECT id, url, name, description, default_branch, last_indexed_at, 
			   last_commit_hash, total_files, total_chunks, status, 
			   created_at, updated_at, deleted_at
		FROM codechunking.repositories 
		WHERE id = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query, id)

	var repoURL, name, statusStr string
	var description, defaultBranch, lastCommitHash *string
	var lastIndexedAt, deletedAt *string
	var totalFiles, totalChunks int
	var createdAtStr, updatedAtStr string

	err := row.Scan(
		&id, &repoURL, &name, &description, &defaultBranch, &lastIndexedAt,
		&lastCommitHash, &totalFiles, &totalChunks, &statusStr,
		&createdAtStr, &updatedAtStr, &deletedAt,
	)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil
		}
		return nil, WrapError(err, "find repository by ID")
	}

	return r.scanRepository(id, repoURL, name, description, defaultBranch, lastIndexedAt, lastCommitHash, totalFiles, totalChunks, statusStr, createdAtStr, updatedAtStr, deletedAt)
}

// FindByURL finds a repository by its URL
func (r *PostgreSQLRepositoryRepository) FindByURL(ctx context.Context, url valueobject.RepositoryURL) (*entity.Repository, error) {
	query := `
		SELECT id, url, name, description, default_branch, last_indexed_at, 
			   last_commit_hash, total_files, total_chunks, status, 
			   created_at, updated_at, deleted_at
		FROM codechunking.repositories 
		WHERE url = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query, url.String())

	var id uuid.UUID
	var repoURL, name, statusStr string
	var description, defaultBranch, lastCommitHash *string
	var lastIndexedAt, deletedAt *string
	var totalFiles, totalChunks int
	var createdAtStr, updatedAtStr string

	err := row.Scan(
		&id, &repoURL, &name, &description, &defaultBranch, &lastIndexedAt,
		&lastCommitHash, &totalFiles, &totalChunks, &statusStr,
		&createdAtStr, &updatedAtStr, &deletedAt,
	)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil
		}
		return nil, WrapError(err, "find repository by URL")
	}

	return r.scanRepository(id, repoURL, name, description, defaultBranch, lastIndexedAt, lastCommitHash, totalFiles, totalChunks, statusStr, createdAtStr, updatedAtStr, deletedAt)
}

// FindAll finds repositories with filters
func (r *PostgreSQLRepositoryRepository) FindAll(ctx context.Context, filters outbound.RepositoryFilters) ([]*entity.Repository, int, error) {
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	// Base query
	baseQuery := `FROM codechunking.repositories WHERE deleted_at IS NULL`

	// Add status filter
	if filters.Status != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filters.Status.String())
		argIndex++
	}

	// Build where clause
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
		return nil, 0, WrapError(err, "count repositories")
	}

	// Data query
	orderBy := "ORDER BY created_at DESC"
	if filters.Sort != "" {
		switch filters.Sort {
		case "name":
			orderBy = "ORDER BY name ASC"
		case "name_desc":
			orderBy = "ORDER BY name DESC"
		case "created_at":
			orderBy = "ORDER BY created_at ASC"
		case "created_at_desc":
			orderBy = "ORDER BY created_at DESC"
		case "updated_at":
			orderBy = "ORDER BY updated_at ASC"
		case "updated_at_desc":
			orderBy = "ORDER BY updated_at DESC"
		}
	}

	limit := 50 // default
	if filters.Limit > 0 {
		limit = filters.Limit
	}

	offset := 0
	if filters.Offset > 0 {
		offset = filters.Offset
	}

	dataQuery := `SELECT id, url, name, description, default_branch, last_indexed_at, 
				  last_commit_hash, total_files, total_chunks, status, 
				  created_at, updated_at, deleted_at ` +
		baseQuery + whereClause + " " + orderBy +
		fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := qi.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, WrapError(err, "query repositories")
	}
	defer rows.Close()

	var repositories []*entity.Repository
	for rows.Next() {
		var id uuid.UUID
		var repoURL, name, statusStr string
		var description, defaultBranch, lastCommitHash *string
		var lastIndexedAt, deletedAt *string
		var totalFiles, totalChunks int
		var createdAtStr, updatedAtStr string

		err := rows.Scan(
			&id, &repoURL, &name, &description, &defaultBranch, &lastIndexedAt,
			&lastCommitHash, &totalFiles, &totalChunks, &statusStr,
			&createdAtStr, &updatedAtStr, &deletedAt,
		)
		if err != nil {
			return nil, 0, WrapError(err, "scan repository row")
		}

		repo, err := r.scanRepository(id, repoURL, name, description, defaultBranch, lastIndexedAt, lastCommitHash, totalFiles, totalChunks, statusStr, createdAtStr, updatedAtStr, deletedAt)
		if err != nil {
			return nil, 0, err
		}

		repositories = append(repositories, repo)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, WrapError(err, "iterate repository rows")
	}

	return repositories, totalCount, nil
}

// Update updates a repository in the database
func (r *PostgreSQLRepositoryRepository) Update(ctx context.Context, repository *entity.Repository) error {
	if repository == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	query := `
		UPDATE codechunking.repositories 
		SET url = $2, name = $3, description = $4, default_branch = $5, 
			last_indexed_at = $6, last_commit_hash = $7, total_files = $8, 
			total_chunks = $9, status = $10, updated_at = $11, deleted_at = $12
		WHERE id = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query,
		repository.ID(),
		repository.URL().String(),
		repository.Name(),
		repository.Description(),
		repository.DefaultBranch(),
		repository.LastIndexedAt(),
		repository.LastCommitHash(),
		repository.TotalFiles(),
		repository.TotalChunks(),
		repository.Status().String(),
		repository.UpdatedAt(),
		repository.DeletedAt(),
	)
	if err != nil {
		return WrapError(err, "update repository")
	}

	if result.RowsAffected() == 0 {
		return WrapError(ErrNotFound, "update repository")
	}

	return nil
}

// Delete soft-deletes a repository by setting deleted_at
func (r *PostgreSQLRepositoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("id cannot be nil")
	}

	query := `
		UPDATE codechunking.repositories 
		SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query, id)
	if err != nil {
		return WrapError(err, "delete repository")
	}

	if result.RowsAffected() == 0 {
		return WrapError(ErrNotFound, "delete repository")
	}

	return nil
}

// Exists checks if a repository with the given URL exists
func (r *PostgreSQLRepositoryRepository) Exists(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM codechunking.repositories WHERE url = $1 AND deleted_at IS NULL)`

	qi := GetQueryInterface(ctx, r.pool)
	var exists bool
	err := qi.QueryRow(ctx, query, url.String()).Scan(&exists)
	if err != nil {
		return false, WrapError(err, "check repository exists")
	}

	return exists, nil
}

// scanRepository is a helper function to convert database row to Repository entity
func (r *PostgreSQLRepositoryRepository) scanRepository(
	id uuid.UUID, urlStr, name string, description, defaultBranch, lastCommitHash *string,
	lastIndexedAtStr *string, totalFiles, totalChunks int, statusStr, createdAtStr, updatedAtStr string, deletedAtStr *string,
) (*entity.Repository, error) {
	// Parse URL
	url, err := valueobject.NewRepositoryURL(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Parse status
	status, err := valueobject.NewRepositoryStatus(statusStr)
	if err != nil {
		return nil, fmt.Errorf("invalid repository status: %w", err)
	}

	// Parse timestamps
	createdAt, err := parseTimestamp(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}

	updatedAt, err := parseTimestamp(updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at timestamp: %w", err)
	}

	var lastIndexedAt *time.Time
	if lastIndexedAtStr != nil {
		parsed, err := parseTimestamp(*lastIndexedAtStr)
		if err != nil {
			return nil, fmt.Errorf("invalid last_indexed_at timestamp: %w", err)
		}
		lastIndexedAt = &parsed
	}

	var deletedAt *time.Time
	if deletedAtStr != nil {
		parsed, err := parseTimestamp(*deletedAtStr)
		if err != nil {
			return nil, fmt.Errorf("invalid deleted_at timestamp: %w", err)
		}
		deletedAt = &parsed
	}

	return entity.RestoreRepository(
		id, url, name, description, defaultBranch,
		lastIndexedAt, lastCommitHash, totalFiles, totalChunks,
		status, createdAt, updatedAt, deletedAt,
	), nil
}

// parseTimestamp parses a timestamp string from the database
func parseTimestamp(timestamp string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.999999-07:00", timestamp)
}
