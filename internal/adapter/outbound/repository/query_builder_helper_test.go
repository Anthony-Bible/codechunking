package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildPaginationClause tests the pagination clause building helper.
func TestBuildPaginationClause(t *testing.T) {
	tests := []struct {
		name           string
		limit          int
		offset         int
		expectedClause string
	}{
		{
			name:           "Standard pagination",
			limit:          10,
			offset:         20,
			expectedClause: " LIMIT 10 OFFSET 20",
		},
		{
			name:           "Zero offset",
			limit:          50,
			offset:         0,
			expectedClause: " LIMIT 50 OFFSET 0",
		},
		{
			name:           "Large values",
			limit:          100,
			offset:         1000,
			expectedClause: " LIMIT 100 OFFSET 1000",
		},
		{
			name:           "Default limit",
			limit:          50,
			offset:         25,
			expectedClause: " LIMIT 50 OFFSET 25",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPaginationClause(tt.limit, tt.offset)
			assert.Equal(t, tt.expectedClause, result)
		})
	}
}

// TestExecuteCountAndDataQuery tests the count and data query execution helper.
func TestExecuteCountAndDataQuery(t *testing.T) {
	// This test will use the actual database connection to test the query pattern
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	qi := GetQueryInterface(ctx, pool)

	tests := []struct {
		name          string
		baseQuery     string
		selectColumns string
		whereClause   string
		orderBy       string
		args          []interface{}
		limit         int
		offset        int
		expectError   bool
		expectedCount int
	}{
		{
			name:          "Simple count and data query",
			baseQuery:     "FROM codechunking.repositories WHERE deleted_at IS NULL",
			selectColumns: "SELECT id, url, name, description, default_branch, last_indexed_at, last_commit_hash, total_files, total_chunks, status, created_at, updated_at, deleted_at",
			whereClause:   "",
			orderBy:       "ORDER BY created_at DESC",
			args:          []interface{}{},
			limit:         10,
			offset:        0,
			expectError:   false,
			expectedCount: 0, // No data in clean test DB
		},
		{
			name:          "Query with where clause and parameters",
			baseQuery:     "FROM codechunking.repositories WHERE deleted_at IS NULL",
			selectColumns: "SELECT id, url, name, description, default_branch, last_indexed_at, last_commit_hash, total_files, total_chunks, status, created_at, updated_at, deleted_at",
			whereClause:   " AND status = $1",
			orderBy:       "ORDER BY created_at DESC",
			args:          []interface{}{"pending"},
			limit:         20,
			offset:        5,
			expectError:   false,
			expectedCount: 0, // No data in clean test DB
		},
		{
			name:          "Offset beyond total count should return empty results",
			baseQuery:     "FROM codechunking.repositories WHERE deleted_at IS NULL",
			selectColumns: "SELECT id, url, name, description, default_branch, last_indexed_at, last_commit_hash, total_files, total_chunks, status, created_at, updated_at, deleted_at",
			whereClause:   "",
			orderBy:       "ORDER BY created_at DESC",
			args:          []interface{}{},
			limit:         10,
			offset:        100, // Beyond any possible count in clean test DB
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalCount, rows, err := executeCountAndDataQuery(
				ctx, qi, tt.baseQuery, tt.selectColumns, tt.whereClause,
				tt.orderBy, tt.args, tt.limit, tt.offset,
			)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, totalCount)

			if rows != nil {
				defer rows.Close()
				// Verify rows can be scanned (basic functionality test)
				rowCount := 0
				for rows.Next() {
					rowCount++
				}
				require.NoError(t, rows.Err())
			}
		})
	}
}

// TestExecuteCountAndDataQuery_WithActualData tests the helper with real data.
func TestExecuteCountAndDataQuery_WithActualData(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// Create test repositories
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Save a test repository
	testRepo := createTestRepository(t)
	err := repo.Save(ctx, testRepo)
	require.NoError(t, err)

	qi := GetQueryInterface(ctx, pool)

	// Test with actual data
	totalCount, rows, err := executeCountAndDataQuery(
		ctx,
		qi,
		"FROM codechunking.repositories WHERE deleted_at IS NULL",
		"SELECT id, url, name, description, default_branch, last_indexed_at, last_commit_hash, total_files, total_chunks, status, created_at, updated_at, deleted_at",
		"",
		"ORDER BY created_at DESC",
		[]interface{}{},
		10,
		0,
	)

	require.NoError(t, err)
	assert.Equal(t, 1, totalCount)
	assert.NotNil(t, rows)

	defer rows.Close()
	rowCount := 0
	for rows.Next() {
		rowCount++
	}
	assert.Equal(t, 1, rowCount)
	require.NoError(t, rows.Err())
}
