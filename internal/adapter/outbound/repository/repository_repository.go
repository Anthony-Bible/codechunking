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
	"github.com/jackc/pgx/v5"
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
		return ErrInvalidArgument
	}

	query := `
		INSERT INTO codechunking.repositories (
			id, url, normalized_url, name, description, default_branch, last_indexed_at, 
			last_commit_hash, total_files, total_chunks, status, 
			created_at, updated_at, deleted_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)`

	qi := GetQueryInterface(ctx, r.pool)
	_, err := qi.Exec(ctx, query,
		repository.ID(),
		repository.URL().Raw(),
		repository.URL().Normalized(),
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
		return nil, ErrInvalidArgument
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
	var lastIndexedAt, deletedAt *time.Time
	var totalFiles, totalChunks int
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&id, &repoURL, &name, &description, &defaultBranch, &lastIndexedAt,
		&lastCommitHash, &totalFiles, &totalChunks, &statusStr,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil
		}
		return nil, WrapError(err, "find repository by ID")
	}

	return r.scanRepositoryFromTime(id, repoURL, name, description, defaultBranch, lastIndexedAt, lastCommitHash, totalFiles, totalChunks, statusStr, createdAt, updatedAt, deletedAt)
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
	var lastIndexedAt, deletedAt *time.Time
	var totalFiles, totalChunks int
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&id, &repoURL, &name, &description, &defaultBranch, &lastIndexedAt,
		&lastCommitHash, &totalFiles, &totalChunks, &statusStr,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		if IsNotFoundError(err) {
			return nil, nil
		}
		return nil, WrapError(err, "find repository by URL")
	}

	return r.scanRepositoryFromTime(id, repoURL, name, description, defaultBranch, lastIndexedAt, lastCommitHash, totalFiles, totalChunks, statusStr, createdAt, updatedAt, deletedAt)
}

// FindAll finds repositories with filters
func (r *PostgreSQLRepositoryRepository) FindAll(ctx context.Context, filters outbound.RepositoryFilters) ([]*entity.Repository, int, error) {
	// Validate pagination parameters
	if filters.Limit < 0 {
		return nil, 0, ErrInvalidArgument
	}
	if filters.Limit == 0 {
		return nil, 0, ErrInvalidArgument
	}
	if filters.Offset < 0 {
		return nil, 0, ErrInvalidArgument
	}

	// Validate sort parameter
	if filters.Sort != "" {
		if err := r.validateSortParameter(filters.Sort); err != nil {
			return nil, 0, err
		}
	}

	var whereConditions []string
	var args []interface{}
	argIndex := 1

	// Base query
	baseQuery := `FROM codechunking.repositories WHERE deleted_at IS NULL`

	// Add status filter
	if filters.Status != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filters.Status.String())
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

	// If offset is beyond total count, return empty results
	if filters.Offset >= totalCount {
		return []*entity.Repository{}, totalCount, nil
	}

	// Data query
	orderBy := "ORDER BY created_at DESC"
	if filters.Sort != "" {
		// Parse field:direction format
		parts := strings.Split(filters.Sort, ":")
		field := parts[0]
		direction := "asc"
		if len(parts) > 1 {
			direction = parts[1]
		}

		// Build ORDER BY clause
		switch field {
		case "name":
			if direction == "desc" {
				orderBy = "ORDER BY name DESC"
			} else {
				orderBy = "ORDER BY name ASC"
			}
		case "created_at":
			if direction == "desc" {
				orderBy = "ORDER BY created_at DESC"
			} else {
				orderBy = "ORDER BY created_at ASC"
			}
		case "updated_at":
			if direction == "desc" {
				orderBy = "ORDER BY updated_at DESC"
			} else {
				orderBy = "ORDER BY updated_at ASC"
			}
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
		var lastIndexedAt, deletedAt *time.Time
		var totalFiles, totalChunks int
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &repoURL, &name, &description, &defaultBranch, &lastIndexedAt,
			&lastCommitHash, &totalFiles, &totalChunks, &statusStr,
			&createdAt, &updatedAt, &deletedAt,
		)
		if err != nil {
			return nil, 0, WrapError(err, "scan repository row")
		}

		repo, err := r.scanRepositoryFromTime(id, repoURL, name, description, defaultBranch, lastIndexedAt, lastCommitHash, totalFiles, totalChunks, statusStr, createdAt, updatedAt, deletedAt)
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
		return ErrInvalidArgument
	}

	query := `
		UPDATE codechunking.repositories 
		SET url = $2, normalized_url = $3, name = $4, description = $5, default_branch = $6, 
			last_indexed_at = $7, last_commit_hash = $8, total_files = $9, 
			total_chunks = $10, status = $11, updated_at = $12, deleted_at = $13
		WHERE id = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query,
		repository.ID(),
		repository.URL().Raw(),
		repository.URL().Normalized(),
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
		return ErrInvalidArgument
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

// ExistsByNormalizedURL checks if a repository with the given normalized URL exists
func (r *PostgreSQLRepositoryRepository) ExistsByNormalizedURL(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM codechunking.repositories WHERE normalized_url = $1 AND deleted_at IS NULL)`

	qi := GetQueryInterface(ctx, r.pool)
	var exists bool
	err := qi.QueryRow(ctx, query, url.Normalized()).Scan(&exists)
	if err != nil {
		return false, WrapError(err, "check repository exists by normalized URL")
	}

	return exists, nil
}

// FindByNormalizedURL finds a repository by its normalized URL
func (r *PostgreSQLRepositoryRepository) FindByNormalizedURL(ctx context.Context, url valueobject.RepositoryURL) (*entity.Repository, error) {
	query := `
		SELECT id, url, name, description, default_branch, last_indexed_at, 
			   last_commit_hash, total_files, total_chunks, status, 
			   created_at, updated_at, deleted_at
		FROM codechunking.repositories 
		WHERE normalized_url = $1 AND deleted_at IS NULL`

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query, url.Normalized())

	var id uuid.UUID
	var urlStr, name, statusStr string
	var description, defaultBranch, lastCommitHash *string
	var totalFiles, totalChunks int
	var lastIndexedAt *time.Time
	var createdAt, updatedAt time.Time
	var deletedAt *time.Time

	err := row.Scan(
		&id, &urlStr, &name, &description, &defaultBranch, &lastIndexedAt,
		&lastCommitHash, &totalFiles, &totalChunks, &statusStr,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found, return nil without error
		}
		return nil, WrapError(err, "find repository by normalized URL")
	}

	return r.scanRepositoryFromTime(
		id, urlStr, name, description, defaultBranch,
		lastIndexedAt, lastCommitHash, totalFiles, totalChunks,
		statusStr, createdAt, updatedAt, deletedAt,
	)
}

// scanRepositoryFromTime is a helper function to convert database row to Repository entity when timestamps are already parsed
func (r *PostgreSQLRepositoryRepository) scanRepositoryFromTime(
	id uuid.UUID, urlStr, name string, description, defaultBranch *string,
	lastIndexedAt *time.Time, lastCommitHash *string, totalFiles, totalChunks int, statusStr string, createdAt, updatedAt time.Time, deletedAt *time.Time,
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

	return entity.RestoreRepository(
		id, url, name, description, defaultBranch,
		lastIndexedAt, lastCommitHash, totalFiles, totalChunks,
		status, createdAt, updatedAt, deletedAt,
	), nil
}

// validateSortParameter validates the sort parameter format and fields
func (r *PostgreSQLRepositoryRepository) validateSortParameter(sort string) error {
	parts := strings.Split(sort, ":")
	if len(parts) != 2 {
		// If no colon, assume it's just the field name with default direction
		parts = []string{sort, "asc"}
	}

	field, direction := parts[0], parts[1]

	// Validate field
	validFields := map[string]bool{
		"name":       true,
		"created_at": true,
		"updated_at": true,
	}
	if !validFields[field] {
		return ErrInvalidArgument
	}

	// Validate direction
	validDirections := map[string]bool{
		"asc":  true,
		"desc": true,
	}
	if !validDirections[direction] {
		return ErrInvalidArgument
	}

	return nil
}
