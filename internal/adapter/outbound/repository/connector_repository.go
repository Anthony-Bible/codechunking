package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	domainerrors "codechunking/internal/domain/errors/domain"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLConnectorRepository implements the ConnectorRepository interface.
type PostgreSQLConnectorRepository struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLConnectorRepository creates a new PostgreSQL connector repository.
func NewPostgreSQLConnectorRepository(pool *pgxpool.Pool) *PostgreSQLConnectorRepository {
	return &PostgreSQLConnectorRepository{pool: pool}
}

// Save inserts a new connector record into the database.
func (r *PostgreSQLConnectorRepository) Save(ctx context.Context, connector *entity.Connector) error {
	if connector == nil {
		return ErrInvalidArgument
	}

	query := `
		INSERT INTO codechunking.connectors (
			id, name, connector_type, base_url, auth_token,
			status, repository_count, last_sync_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	qi := GetQueryInterface(ctx, r.pool)
	_, err := qi.Exec(ctx, query,
		connector.ID(),
		connector.Name(),
		connector.ConnectorType().String(),
		connector.BaseURL(),
		connector.AuthToken(),
		connector.Status().String(),
		connector.RepositoryCount(),
		connector.LastSyncAt(),
		connector.CreatedAt(),
		connector.UpdatedAt(),
	)
	if err != nil {
		return WrapError(err, "save connector")
	}
	return nil
}

// FindByID retrieves a connector by its UUID.
func (r *PostgreSQLConnectorRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Connector, error) {
	query := `
		SELECT id, name, connector_type, base_url, auth_token,
		       status, repository_count, last_sync_at, created_at, updated_at
		FROM codechunking.connectors
		WHERE id = $1`

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query, id)

	connector, err := scanConnector(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerrors.ErrConnectorNotFound
		}
		return nil, WrapError(err, "find connector by id")
	}
	return connector, nil
}

// FindByName retrieves a connector by its unique name.
func (r *PostgreSQLConnectorRepository) FindByName(ctx context.Context, name string) (*entity.Connector, error) {
	query := `
		SELECT id, name, connector_type, base_url, auth_token,
		       status, repository_count, last_sync_at, created_at, updated_at
		FROM codechunking.connectors
		WHERE name = $1`

	qi := GetQueryInterface(ctx, r.pool)
	row := qi.QueryRow(ctx, query, name)

	connector, err := scanConnector(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerrors.ErrConnectorNotFound
		}
		return nil, WrapError(err, "find connector by name")
	}
	return connector, nil
}

// FindAll returns a paginated list of connectors matching the given filters.
func (r *PostgreSQLConnectorRepository) FindAll(
	ctx context.Context,
	filters outbound.ConnectorFilters,
) ([]*entity.Connector, int, error) {
	baseSelect := `
		SELECT id, name, connector_type, base_url, auth_token,
		       status, repository_count, last_sync_at, created_at, updated_at
		FROM codechunking.connectors`
	baseCount := `SELECT COUNT(*) FROM codechunking.connectors`

	conditions, args := buildConnectorFilterConditions(filters)
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query
	var total int
	qi := GetQueryInterface(ctx, r.pool)
	if err := qi.QueryRow(ctx, baseCount+where, args...).Scan(&total); err != nil {
		return nil, 0, WrapError(err, "count connectors")
	}

	// Data query
	orderBy := buildConnectorOrderBy(filters.Sort)
	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf(
		"%s%s%s LIMIT $%d OFFSET $%d",
		baseSelect, where, orderBy, len(args)+1, len(args)+2,
	)
	args = append(args, limit, offset)

	rows, err := qi.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, WrapError(err, "list connectors")
	}
	defer rows.Close()

	var connectors []*entity.Connector
	for rows.Next() {
		connector, err := scanConnectorFromRows(rows)
		if err != nil {
			return nil, 0, WrapError(err, "scan connector row")
		}
		connectors = append(connectors, connector)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, WrapError(err, "iterate connector rows")
	}

	return connectors, total, nil
}

// Update persists changes to an existing connector.
func (r *PostgreSQLConnectorRepository) Update(ctx context.Context, connector *entity.Connector) error {
	if connector == nil {
		return ErrInvalidArgument
	}

	query := `
		UPDATE codechunking.connectors
		SET name             = $1,
		    connector_type   = $2,
		    base_url         = $3,
		    auth_token       = $4,
		    status           = $5,
		    repository_count = $6,
		    last_sync_at     = $7,
		    updated_at       = $8
		WHERE id = $9`

	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query,
		connector.Name(),
		connector.ConnectorType().String(),
		connector.BaseURL(),
		connector.AuthToken(),
		connector.Status().String(),
		connector.RepositoryCount(),
		connector.LastSyncAt(),
		connector.UpdatedAt(),
		connector.ID(),
	)
	if err != nil {
		return WrapError(err, "update connector")
	}
	if result.RowsAffected() == 0 {
		return domainerrors.ErrConnectorNotFound
	}
	return nil
}

// Delete removes a connector by its UUID.
func (r *PostgreSQLConnectorRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM codechunking.connectors WHERE id = $1`
	qi := GetQueryInterface(ctx, r.pool)
	result, err := qi.Exec(ctx, query, id)
	if err != nil {
		return WrapError(err, "delete connector")
	}
	if result.RowsAffected() == 0 {
		return domainerrors.ErrConnectorNotFound
	}
	return nil
}

// Exists returns true when a connector with the given name already exists.
func (r *PostgreSQLConnectorRepository) Exists(ctx context.Context, name string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM codechunking.connectors WHERE name = $1)`
	qi := GetQueryInterface(ctx, r.pool)
	var exists bool
	if err := qi.QueryRow(ctx, query, name).Scan(&exists); err != nil {
		return false, WrapError(err, "check connector existence")
	}
	return exists, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// rowScanner is satisfied by both pgx.Row and pgx.Rows so that scanConnector
// can be used in both FindByID/FindByName (single row) and FindAll (multi row).
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanConnector(row rowScanner) (*entity.Connector, error) {
	var (
		id              uuid.UUID
		name            string
		connectorType   string
		baseURL         string
		authToken       *string
		status          string
		repositoryCount int
		lastSyncAt      *time.Time
		createdAt       time.Time
		updatedAt       time.Time
	)

	if err := row.Scan(
		&id, &name, &connectorType, &baseURL, &authToken,
		&status, &repositoryCount, &lastSyncAt, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}

	return buildConnectorEntity(id, name, connectorType, baseURL, authToken, status, repositoryCount, lastSyncAt, createdAt, updatedAt)
}

func scanConnectorFromRows(rows pgx.Rows) (*entity.Connector, error) {
	return scanConnector(rows)
}

func buildConnectorEntity(
	id uuid.UUID,
	name, connectorType, baseURL string,
	authToken *string,
	status string,
	repositoryCount int,
	lastSyncAt *time.Time,
	createdAt, updatedAt time.Time,
) (*entity.Connector, error) {
	ct, err := valueobject.NewConnectorType(connectorType)
	if err != nil {
		return nil, fmt.Errorf("invalid connector type in database: %w", err)
	}
	cs, err := valueobject.NewConnectorStatus(status)
	if err != nil {
		return nil, fmt.Errorf("invalid connector status in database: %w", err)
	}
	return entity.RestoreConnector(id, name, ct, baseURL, authToken, cs, repositoryCount, lastSyncAt, createdAt, updatedAt), nil
}

func buildConnectorFilterConditions(filters outbound.ConnectorFilters) ([]string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filters.ConnectorType != nil {
		conditions = append(conditions, fmt.Sprintf("connector_type = $%d", argIdx))
		args = append(args, filters.ConnectorType.String())
		argIdx++
	}

	if filters.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filters.Status.String())
		argIdx++ //nolint:ineffassign // argIdx incremented for future use
	}

	return conditions, args
}

func buildConnectorOrderBy(sort string) string {
	switch sort {
	case "name:asc":
		return " ORDER BY name ASC"
	case "name:desc":
		return " ORDER BY name DESC"
	case "created_at:asc":
		return " ORDER BY created_at ASC"
	case "created_at:desc":
		return " ORDER BY created_at DESC"
	default:
		return " ORDER BY created_at DESC"
	}
}
