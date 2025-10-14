package repository

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLRepositoryRepository implements the RepositoryRepository interface.
//
// URL Storage and Query Consistency:
// This repository implementation maintains strict consistency between how URLs are stored
// and how they are queried. The architecture uses a dual-URL approach:
//
//  1. Raw URL Storage (url column): Stores the exact URL as provided by users,
//     preserving original casing, .git suffixes, and other user-input characteristics.
//     Used for exact-match queries in FindByURL() and Exists() methods.
//
//  2. Normalized URL Storage (normalized_url column): Stores a canonical form of the URL
//     (lowercase, no .git suffix, etc.) for duplicate detection and semantic queries.
//     Used by FindByNormalizedURL() and ExistsByNormalizedURL() methods.
//
// This design ensures that:
// - Users can retrieve repositories using the exact same URL format they provided
// - The system can detect semantic duplicates (github.com/owner/repo == GitHub.com/Owner/Repo.git)
// - Query methods are consistent with their corresponding storage format.
type PostgreSQLRepositoryRepository struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLRepositoryRepository creates a new PostgreSQL repository repository.
func NewPostgreSQLRepositoryRepository(pool *pgxpool.Pool) *PostgreSQLRepositoryRepository {
	return &PostgreSQLRepositoryRepository{
		pool: pool,
	}
}

// Save saves a repository to the database.
//
// URL Storage Strategy:
// - Stores repository.URL().Raw() in the 'url' column for exact-match queries
// - Stores repository.URL().Normalized() in the 'normalized_url' column for duplicate detection
// This dual storage approach enables both exact lookups and semantic duplicate detection.
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

// FindByID finds a repository by its ID.
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
			return nil, nil //nolint:nilnil // Not found is not an error condition for Find methods
		}
		return nil, WrapError(err, "find repository by ID")
	}

	return r.scanRepositoryFromTime(
		id,
		repoURL,
		name,
		description,
		defaultBranch,
		lastIndexedAt,
		lastCommitHash,
		totalFiles,
		totalChunks,
		statusStr,
		createdAt,
		updatedAt,
		deletedAt,
	)
}

// FindByURL finds a repository by its exact raw URL.
//
// This method queries the 'url' column using the raw URL format (url.Raw())
// to maintain consistency with how URLs are stored during Save() operations.
// Use FindByNormalizedURL() if you need to find repositories by their canonical form.
//
// Returns nil if no repository is found, or an error if the query fails.
func (r *PostgreSQLRepositoryRepository) FindByURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	query := `
		SELECT id, url, name, description, default_branch, last_indexed_at, 
			   last_commit_hash, total_files, total_chunks, status, 
			   created_at, updated_at, deleted_at
		FROM codechunking.repositories 
		WHERE url = $1 AND deleted_at IS NULL`

	return r.queryRepositoryByRawURL(ctx, url, query)
}

func (r *PostgreSQLRepositoryRepository) validateFilters(filters outbound.RepositoryFilters) error {
	if filters.Limit < 0 {
		return ErrInvalidArgument
	}
	if filters.Limit == 0 {
		return ErrInvalidArgument
	}
	if filters.Offset < 0 {
		return ErrInvalidArgument
	}

	if filters.Sort != "" {
		return r.validateSortParameter(filters.Sort)
	}

	return nil
}

func (r *PostgreSQLRepositoryRepository) buildWhereClause(filters outbound.RepositoryFilters) (string, []interface{}) {
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	if filters.Status != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filters.Status.String())
		argIndex++
	}

	if filters.Name != "" {
		condition, arg := buildLikeCondition("name", filters.Name, argIndex)
		whereConditions = append(whereConditions, condition)
		args = append(args, arg)
		argIndex++
	}

	if filters.URL != "" {
		condition, arg := buildLikeCondition("url", filters.URL, argIndex)
		whereConditions = append(whereConditions, condition)
		args = append(args, arg)
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = " AND " + strings.Join(whereConditions, " AND ")
	}

	return whereClause, args
}

// buildLikeCondition creates a case-insensitive LIKE condition for the given column.
func buildLikeCondition(column, value string, argIndex int) (string, string) {
	condition := fmt.Sprintf("LOWER(%s) LIKE LOWER($%d)", column, argIndex)
	arg := "%" + value + "%"
	return condition, arg
}

func (r *PostgreSQLRepositoryRepository) buildOrderByClause(sort string) string {
	const (
		orderByCreatedAtDesc = "ORDER BY created_at DESC"
		sortDirectionDesc    = "desc"
	)

	if sort == "" {
		return orderByCreatedAtDesc
	}

	parts := strings.Split(sort, ":")
	field := parts[0]
	direction := "asc"
	if len(parts) > 1 {
		direction = parts[1]
	}

	switch field {
	case "name":
		if direction == sortDirectionDesc {
			return "ORDER BY name DESC"
		}
		return "ORDER BY name ASC"
	case "created_at":
		if direction == sortDirectionDesc {
			return orderByCreatedAtDesc
		}
		return "ORDER BY created_at ASC"
	case "updated_at":
		if direction == sortDirectionDesc {
			return "ORDER BY updated_at DESC"
		}
		return "ORDER BY updated_at ASC"
	default:
		return orderByCreatedAtDesc
	}
}

func (r *PostgreSQLRepositoryRepository) getPaginationParams(filters outbound.RepositoryFilters) (int, int) {
	limit := 50
	if filters.Limit > 0 {
		limit = filters.Limit
	}

	offset := 0
	if filters.Offset > 0 {
		offset = filters.Offset
	}

	return limit, offset
}

// FindAll finds repositories with filters.
func (r *PostgreSQLRepositoryRepository) FindAll(
	ctx context.Context,
	filters outbound.RepositoryFilters,
) ([]*entity.Repository, int, error) {
	slogger.Info(ctx, "FindAll called", slogger.Fields{"limit": filters.Limit, "offset": filters.Offset})

	if err := r.validateFilters(filters); err != nil {
		slogger.Error(ctx, "Filter validation failed", slogger.Fields{"error": err.Error()})
		return nil, 0, err
	}

	totalCount, rows, err := r.executeQuery(ctx, filters)
	if err != nil {
		return nil, 0, err
	}

	if rows == nil {
		slogger.Info(ctx, "No rows returned", slogger.Fields{"total_count": totalCount})
		return []*entity.Repository{}, totalCount, nil
	}
	defer rows.Close()

	repositories, err := r.processRows(ctx, rows)
	if err != nil {
		return nil, 0, err
	}

	slogger.Info(ctx, "FindAll completed", slogger.Fields{
		"count": len(repositories), "total": totalCount,
	})

	return repositories, totalCount, nil
}

// executeQuery builds and executes the repository query.
func (r *PostgreSQLRepositoryRepository) executeQuery(
	ctx context.Context,
	filters outbound.RepositoryFilters,
) (int, pgx.Rows, error) {
	baseQuery := `FROM codechunking.repositories WHERE deleted_at IS NULL`
	whereClause, args := r.buildWhereClause(filters)
	orderBy := r.buildOrderByClause(filters.Sort)
	limit, offset := r.getPaginationParams(filters)

	qi := GetQueryInterface(ctx, r.pool)
	selectColumns := `SELECT id, url, name, description, default_branch, last_indexed_at,
				  last_commit_hash, total_files, total_chunks, status,
				  created_at, updated_at, deleted_at`

	slogger.Info(ctx, "Executing query", slogger.Fields{
		"limit": limit, "offset": offset, "where": whereClause, "order": orderBy,
	})

	totalCount, rows, err := executeCountAndDataQuery(
		ctx, qi, baseQuery, selectColumns, whereClause, orderBy, args, limit, offset,
	)
	if err != nil {
		slogger.Error(ctx, "Query execution failed", slogger.Fields{"error": err.Error()})
		return 0, nil, err
	}

	return totalCount, rows, nil
}

// processRows scans rows and converts them to repository entities.
func (r *PostgreSQLRepositoryRepository) processRows(
	ctx context.Context,
	rows pgx.Rows,
) ([]*entity.Repository, error) {
	var repositories []*entity.Repository

	for rows.Next() {
		repo, err := r.scanRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, repo)
	}

	if err := rows.Err(); err != nil {
		slogger.Error(ctx, "Error iterating rows", slogger.Fields{"error": err.Error()})
		return nil, WrapError(err, "iterate repository rows")
	}

	return repositories, nil
}

// scanRow scans a single row into a repository entity.
func (r *PostgreSQLRepositoryRepository) scanRow(
	ctx context.Context,
	rows pgx.Rows,
) (*entity.Repository, error) {
	var id uuid.UUID
	var repoURL, name, statusStr string
	var description, defaultBranch, lastCommitHash *string
	var lastIndexedAt, deletedAt *time.Time
	var totalFiles, totalChunks int
	var createdAt, updatedAt time.Time

	if err := rows.Scan(
		&id, &repoURL, &name, &description, &defaultBranch, &lastIndexedAt,
		&lastCommitHash, &totalFiles, &totalChunks, &statusStr,
		&createdAt, &updatedAt, &deletedAt,
	); err != nil {
		slogger.Error(ctx, "Failed to scan row", slogger.Fields{"error": err.Error()})
		return nil, WrapError(err, "scan repository row")
	}

	repo, err := r.scanRepositoryFromTime(
		id, repoURL, name, description, defaultBranch, lastIndexedAt,
		lastCommitHash, totalFiles, totalChunks, statusStr, createdAt, updatedAt, deletedAt,
	)
	if err != nil {
		slogger.Error(ctx, "Failed to convert to entity", slogger.Fields{
			"error": err.Error(), "id": id.String(),
		})
		return nil, err
	}

	return repo, nil
}

// Update updates a repository in the database.
//
// URL Storage Strategy:
// - Updates both 'url' column with repository.URL().Raw() for exact-match queries
// - Updates 'normalized_url' column with repository.URL().Normalized() for duplicate detection
// This maintains consistency with the Save() method's dual storage approach.
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

// Delete soft-deletes a repository by setting deleted_at.
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

// Exists checks if a repository with the given exact raw URL exists.
//
// This method queries the 'url' column using the raw URL format (url.Raw())
// to maintain consistency with how URLs are stored during Save() operations.
// Use ExistsByNormalizedURL() if you need to check existence by canonical form.
//
// Returns true if the repository exists, false otherwise, or an error if the query fails.
func (r *PostgreSQLRepositoryRepository) Exists(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	return r.checkExistenceByRawURL(ctx, url)
}

// ExistsByNormalizedURL checks if a repository with the given normalized URL exists.
//
// This method queries the 'normalized_url' column using the normalized URL format (url.Normalized())
// to check for semantic duplicates regardless of the original raw URL format.
// This is useful for duplicate detection when the raw URL format may vary.
//
// Returns true if a repository with the equivalent normalized URL exists, false otherwise,
// or an error if the query fails.
func (r *PostgreSQLRepositoryRepository) ExistsByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (bool, error) {
	return r.checkExistenceByNormalizedURL(ctx, url)
}

// FindByNormalizedURL finds a repository by its normalized URL.
//
// This method queries the 'normalized_url' column using the normalized URL format (url.Normalized())
// to find repositories based on their canonical form, regardless of how the raw URL was originally formatted.
// This is useful for duplicate detection and semantic lookups.
//
// Returns the repository if found by normalized URL, nil if not found, or an error if the query fails.
func (r *PostgreSQLRepositoryRepository) FindByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	query := `
		SELECT id, url, name, description, default_branch, last_indexed_at, 
			   last_commit_hash, total_files, total_chunks, status, 
			   created_at, updated_at, deleted_at
		FROM codechunking.repositories 
		WHERE normalized_url = $1 AND deleted_at IS NULL`

	return r.queryRepositoryByNormalizedURL(ctx, url, query)
}

// scanRepositoryFromTime is a helper function to convert database row to Repository entity when timestamps are already parsed.
func (r *PostgreSQLRepositoryRepository) scanRepositoryFromTime(
	id uuid.UUID,
	urlStr, name string,
	description, defaultBranch *string,
	lastIndexedAt *time.Time,
	lastCommitHash *string,
	totalFiles, totalChunks int,
	statusStr string,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
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

// queryRepositoryByRawURL is a helper method that executes a query against the 'url' column
// using the raw URL format. This ensures consistency across all raw URL-based queries.
func (r *PostgreSQLRepositoryRepository) queryRepositoryByRawURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
	query string,
) (*entity.Repository, error) {
	// Validate inputs for early error detection
	if url.Raw() == "" {
		return nil, ErrInvalidArgument
	}

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query, url.Raw())

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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // Not found is not an error condition for Find methods
		}
		return nil, WrapError(err, "query repository by raw URL")
	}

	return r.scanRepositoryFromTime(
		id, repoURL, name, description, defaultBranch,
		lastIndexedAt, lastCommitHash, totalFiles, totalChunks,
		statusStr, createdAt, updatedAt, deletedAt,
	)
}

// queryRepositoryByNormalizedURL is a helper method that executes a query against the 'normalized_url' column
// using the normalized URL format. This ensures consistency across all normalized URL-based queries.
func (r *PostgreSQLRepositoryRepository) queryRepositoryByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
	query string,
) (*entity.Repository, error) {
	// Validate inputs for early error detection
	if url.Normalized() == "" {
		return nil, ErrInvalidArgument
	}

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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // Not found is not an error condition for Find methods
		}
		return nil, WrapError(err, "query repository by normalized URL")
	}

	return r.scanRepositoryFromTime(
		id, urlStr, name, description, defaultBranch,
		lastIndexedAt, lastCommitHash, totalFiles, totalChunks,
		statusStr, createdAt, updatedAt, deletedAt,
	)
}

// checkExistenceByRawURL is a helper method that checks repository existence using the raw URL format.
// This ensures consistency with storage operations that use raw URLs.
func (r *PostgreSQLRepositoryRepository) checkExistenceByRawURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (bool, error) {
	// Validate inputs for early error detection
	if url.Raw() == "" {
		return false, ErrInvalidArgument
	}

	query := `SELECT EXISTS(SELECT 1 FROM codechunking.repositories WHERE url = $1 AND deleted_at IS NULL)`

	qi := GetQueryInterface(ctx, r.pool)
	var exists bool
	err := qi.QueryRow(ctx, query, url.Raw()).Scan(&exists)
	if err != nil {
		return false, WrapError(err, "check repository exists by raw URL")
	}

	return exists, nil
}

// checkExistenceByNormalizedURL is a helper method that checks repository existence using the normalized URL format.
// This is useful for duplicate detection regardless of the original URL format.
func (r *PostgreSQLRepositoryRepository) checkExistenceByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (bool, error) {
	// Validate inputs for early error detection
	if url.Normalized() == "" {
		return false, ErrInvalidArgument
	}

	query := `SELECT EXISTS(SELECT 1 FROM codechunking.repositories WHERE normalized_url = $1 AND deleted_at IS NULL)`

	qi := GetQueryInterface(ctx, r.pool)
	var exists bool
	err := qi.QueryRow(ctx, query, url.Normalized()).Scan(&exists)
	if err != nil {
		return false, WrapError(err, "check repository exists by normalized URL")
	}

	return exists, nil
}

// validateSortParameter validates the sort parameter format and fields.
func (r *PostgreSQLRepositoryRepository) validateSortParameter(sort string) error {
	parts := strings.Split(sort, ":")
	if len(parts) != 2 { //nolint:mnd // sort parameter format: "field:direction"
		// If no colon, assume it's just the field name with default direction
		parts = []string{sort, "asc"}
	}

	field, direction := parts[0], parts[1]

	if !isValidSortField(field) {
		return ErrInvalidArgument
	}

	if !isValidSortDirection(direction) {
		return ErrInvalidArgument
	}

	return nil
}

// isValidSortField checks if the given field is valid for sorting.
func isValidSortField(field string) bool {
	validFields := map[string]bool{
		"name":       true,
		"created_at": true,
		"updated_at": true,
	}
	return validFields[field]
}

// isValidSortDirection checks if the given direction is valid for sorting.
func isValidSortDirection(direction string) bool {
	return direction == "asc" || direction == "desc"
}
