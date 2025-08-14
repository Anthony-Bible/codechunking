package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// buildPaginationClause constructs the LIMIT/OFFSET clause for SQL queries.
// It safely formats the limit and offset values into a SQL clause.
func buildPaginationClause(limit, offset int) string {
	return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
}

// executeCountAndDataQuery executes a standardized count + data query pattern used across repository methods.
// It first executes a count query to get the total number of records, then executes the data query
// with pagination. This pattern is commonly used for paginated results with total count information.
//
// Parameters:
//   - ctx: Context for query execution
//   - qi: QueryInterface for database operations
//   - baseQuery: The FROM clause with base WHERE conditions (e.g., "FROM table WHERE deleted_at IS NULL")
//   - selectColumns: The SELECT clause for the data query (e.g., "SELECT id, name, ...")
//   - whereClause: Additional WHERE conditions to append (e.g., " AND status = $1")
//   - orderBy: The ORDER BY clause (e.g., "ORDER BY created_at DESC")
//   - args: Query parameters for placeholders
//   - limit: Maximum number of records to return
//   - offset: Number of records to skip
//
// Returns:
//   - totalCount: Total number of records matching the criteria (ignoring pagination)
//   - rows: Database rows for the paginated data query (nil if offset >= totalCount)
//   - error: Any error encountered during query execution
func executeCountAndDataQuery(
	ctx context.Context,
	qi QueryInterface,
	baseQuery string,
	selectColumns string,
	whereClause string,
	orderBy string,
	args []interface{},
	limit int,
	offset int,
) (int, pgx.Rows, error) {
	// Build and execute count query
	countQuery := "SELECT COUNT(*) " + baseQuery + whereClause

	var totalCount int
	err := qi.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return 0, nil, WrapError(err, "count query")
	}

	// If offset is beyond total count, return empty results early
	// This optimization avoids executing the data query when no results would be returned
	if offset >= totalCount {
		return totalCount, nil, nil
	}

	// Build and execute data query with pagination
	paginationClause := buildPaginationClause(limit, offset)
	dataQuery := selectColumns + " " + baseQuery + whereClause + " " + orderBy + paginationClause

	rows, err := qi.Query(ctx, dataQuery, args...)
	if err != nil {
		return 0, nil, WrapError(err, "data query")
	}

	return totalCount, rows, nil
}
