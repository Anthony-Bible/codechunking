package repository

import (
	"strings"
	"testing"
)

// TestBuildMultiRowChunkInsert_SingleRow tests building query for a single chunk.
// This test verifies the basic query structure and parameter placeholders.
func TestBuildMultiRowChunkInsert_SingleRow(t *testing.T) {
	// This will fail because buildMultiRowChunkInsert doesn't exist yet
	query := buildMultiRowChunkInsert(1)

	// Verify query contains the INSERT clause with all 17 columns
	expectedColumns := []string{
		"id", "repository_id", "file_path", "chunk_type", "content", "language",
		"start_line", "end_line", "entity_name", "parent_entity", "content_hash",
		"metadata", "qualified_name", "signature", "visibility", "token_count", "token_counted_at",
	}

	for _, col := range expectedColumns {
		if !strings.Contains(query, col) {
			t.Errorf("Query missing column: %s", col)
		}
	}

	// Verify VALUES clause has exactly 17 parameters for single row
	if !strings.Contains(query, "($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)") {
		t.Error("Query missing correct VALUES clause with $1-$17 parameters for single row")
	}

	// Verify ON CONFLICT clause exists
	if !strings.Contains(query, "ON CONFLICT (repository_id, file_path, content_hash)") {
		t.Error("Query missing ON CONFLICT clause")
	}

	// Verify DO UPDATE SET clause exists with COALESCE logic for token fields
	if !strings.Contains(query, "DO UPDATE SET") {
		t.Error("Query missing DO UPDATE SET clause")
	}

	if !strings.Contains(query, "COALESCE") {
		t.Error("Query missing COALESCE logic for token_count preservation")
	}

	// Verify RETURNING clause exists
	if !strings.Contains(query, "RETURNING id") {
		t.Error("Query missing RETURNING id clause")
	}
}

// TestBuildMultiRowChunkInsert_MultipleRows tests building query for multiple chunks.
// This test verifies correct generation of multiple VALUES clauses with proper parameter numbering.
func TestBuildMultiRowChunkInsert_MultipleRows(t *testing.T) {
	tests := []struct {
		name     string
		numRows  int
		firstRow string // Expected first VALUES row parameters
		lastRow  string // Expected last VALUES row parameters
	}{
		{
			name:     "Two rows",
			numRows:  2,
			firstRow: "($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)",
			lastRow:  "($18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34)",
		},
		{
			name:     "Three rows",
			numRows:  3,
			firstRow: "($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)",
			lastRow:  "($35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49, $50, $51)",
		},
		{
			name:     "Five rows",
			numRows:  5,
			firstRow: "($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)",
			lastRow:  "($69, $70, $71, $72, $73, $74, $75, $76, $77, $78, $79, $80, $81, $82, $83, $84, $85)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail because buildMultiRowChunkInsert doesn't exist yet
			query := buildMultiRowChunkInsert(tt.numRows)

			// Verify first row parameters
			if !strings.Contains(query, tt.firstRow) {
				t.Errorf("Query missing first row with parameters %s", tt.firstRow)
			}

			// Verify last row parameters
			if !strings.Contains(query, tt.lastRow) {
				t.Errorf("Query missing last row with parameters %s", tt.lastRow)
			}

			// Verify VALUES keyword appears exactly once
			valuesCount := strings.Count(query, "VALUES")
			if valuesCount != 1 {
				t.Errorf("Expected exactly 1 VALUES keyword, got %d", valuesCount)
			}

			// Verify correct number of value rows (count commas between rows + 1)
			// Each VALUES row except last should be followed by comma
			valuesSectionStart := strings.Index(query, "VALUES")
			onConflictStart := strings.Index(query, "ON CONFLICT")
			if valuesSectionStart == -1 || onConflictStart == -1 {
				t.Fatal("Query structure invalid - missing VALUES or ON CONFLICT")
			}

			valuesSection := query[valuesSectionStart:onConflictStart]
			// Count opening parentheses in VALUES section (one per row)
			rowCount := strings.Count(valuesSection, "($")
			if rowCount != tt.numRows {
				t.Errorf("Expected %d value rows, found %d", tt.numRows, rowCount)
			}

			// Verify ON CONFLICT and RETURNING clauses still present
			if !strings.Contains(query, "ON CONFLICT (repository_id, file_path, content_hash)") {
				t.Error("Query missing ON CONFLICT clause")
			}

			if !strings.Contains(query, "RETURNING id") {
				t.Error("Query missing RETURNING id clause")
			}
		})
	}
}

// TestBuildMultiRowChunkInsert_ParameterIndexing tests parameter numbering correctness.
// This test specifically validates that parameters are numbered sequentially across all rows.
func TestBuildMultiRowChunkInsert_ParameterIndexing(t *testing.T) {
	const numRows = 3
	const columnsPerRow = 17

	// This will fail because buildMultiRowChunkInsert doesn't exist yet
	query := buildMultiRowChunkInsert(numRows)

	// Verify each expected parameter placeholder exists exactly once
	totalParams := numRows * columnsPerRow
	for i := 1; i <= totalParams; i++ {
		// For proper testing, check each parameter number exists in query
		// Parameter $1 should appear, $2 should appear, etc.
		// But we need to be careful about $1 vs $10, $11, etc.
		// So we'll check for the parameter with word boundaries (comma or parenthesis)
		paramPatterns := []string{
			"$" + formatParamNum(i) + ",", // Followed by comma
			"$" + formatParamNum(i) + ")", // Followed by closing paren
		}

		found := false
		for _, pattern := range paramPatterns {
			if strings.Contains(query, pattern) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Parameter $%d not found in query at expected position", i)
		}
	}

	// Verify no parameters beyond the expected total exist
	invalidParam := "$" + formatParamNum(totalParams+1)
	if strings.Contains(query, invalidParam) {
		t.Errorf("Query contains unexpected parameter %s (expected max $%d)", invalidParam, totalParams)
	}
}

// formatParamNum formats an integer parameter number as a string.
func formatParamNum(num int) string {
	if num < 10 {
		return string(rune('0' + num))
	}
	// For two-digit numbers
	tens := num / 10
	ones := num % 10
	return string(rune('0'+tens)) + string(rune('0'+ones))
}

// TestBuildMultiRowChunkInsert_ZeroRows tests handling of zero rows.
// This test verifies error handling or empty string return for invalid input.
func TestBuildMultiRowChunkInsert_ZeroRows(t *testing.T) {
	// This will fail because buildMultiRowChunkInsert doesn't exist yet
	query := buildMultiRowChunkInsert(0)

	// For zero rows, we expect either:
	// 1. Empty string (no-op)
	// 2. Or the function could panic/error (if it validates input)
	// For this test, we'll expect an empty string as the most sensible behavior
	if query != "" {
		t.Errorf("Expected empty query for 0 rows, got: %s", query)
	}
}

// TestBuildMultiRowChunkInsert_NegativeRows tests handling of negative rows.
// This test verifies error handling for invalid negative input.
func TestBuildMultiRowChunkInsert_NegativeRows(t *testing.T) {
	// This will fail because buildMultiRowChunkInsert doesn't exist yet
	query := buildMultiRowChunkInsert(-1)

	// For negative rows, we expect empty string (invalid input should result in no-op)
	if query != "" {
		t.Errorf("Expected empty query for negative rows, got: %s", query)
	}
}

// TestBuildMultiRowChunkInsert_LargeRowCount tests handling of large batch sizes.
// This test verifies the function can handle large numbers of rows (e.g., 1000).
func TestBuildMultiRowChunkInsert_LargeRowCount(t *testing.T) {
	const numRows = 1000
	const columnsPerRow = 17

	// This will fail because buildMultiRowChunkInsert doesn't exist yet
	query := buildMultiRowChunkInsert(numRows)

	// Verify query is not empty
	if query == "" {
		t.Fatal("Expected non-empty query for 1000 rows")
	}

	// Verify first and last parameter numbers are correct
	firstParam := "$1"
	lastParam := "$17000" // 1000 rows * 17 columns = 17000 parameters

	if !strings.Contains(query, firstParam) {
		t.Error("Query missing first parameter $1")
	}

	// Check for last parameter (should appear at end of last VALUES row)
	if !strings.Contains(query, lastParam) {
		t.Errorf("Query missing last parameter %s", lastParam)
	}

	// Verify structure is still valid
	if !strings.Contains(query, "INSERT INTO codechunking.code_chunks") {
		t.Error("Query missing INSERT INTO clause")
	}

	if !strings.Contains(query, "VALUES") {
		t.Error("Query missing VALUES clause")
	}

	if !strings.Contains(query, "ON CONFLICT") {
		t.Error("Query missing ON CONFLICT clause")
	}

	if !strings.Contains(query, "RETURNING id") {
		t.Error("Query missing RETURNING clause")
	}
}

// TestBuildMultiRowChunkInsert_QueryStructure tests overall query structure compliance.
// This test verifies the query follows the exact expected SQL structure.
func TestBuildMultiRowChunkInsert_QueryStructure(t *testing.T) {
	query := buildMultiRowChunkInsert(2)

	// Verify query sections appear in correct order
	sections := []string{
		"INSERT INTO codechunking.code_chunks",
		"VALUES",
		"ON CONFLICT (repository_id, file_path, content_hash)",
		"DO UPDATE SET",
		"RETURNING id",
	}

	lastIndex := -1
	for _, section := range sections {
		index := strings.Index(query, section)
		if index == -1 {
			t.Errorf("Query missing required section: %s", section)
			continue
		}
		if index <= lastIndex {
			t.Errorf("Query sections out of order: %s should appear after previous section", section)
		}
		lastIndex = index
	}

	// Verify UPDATE SET clause includes proper COALESCE logic
	updateSetStart := strings.Index(query, "DO UPDATE SET")
	returningStart := strings.Index(query, "RETURNING")
	if updateSetStart != -1 && returningStart != -1 {
		updateSetClause := query[updateSetStart:returningStart]

		// Should update id to existing value (no change)
		if !strings.Contains(updateSetClause, "id = code_chunks.id") {
			t.Error("UPDATE SET clause should preserve existing chunk ID")
		}

		// Should use COALESCE to prefer existing token_count
		if !strings.Contains(updateSetClause, "token_count = COALESCE(code_chunks.token_count, EXCLUDED.token_count)") {
			t.Error("UPDATE SET clause should use COALESCE to preserve existing token_count")
		}

		// Should use COALESCE to prefer existing token_counted_at
		if !strings.Contains(
			updateSetClause,
			"token_counted_at = COALESCE(code_chunks.token_counted_at, EXCLUDED.token_counted_at)",
		) {
			t.Error("UPDATE SET clause should use COALESCE to preserve existing token_counted_at")
		}
	}
}

// TestBuildMultiRowChunkInsert_ColumnOrder tests that columns appear in exact order.
// This test verifies the column order matches the expected schema.
func TestBuildMultiRowChunkInsert_ColumnOrder(t *testing.T) {
	query := buildMultiRowChunkInsert(1)

	// Expected column order (17 columns)
	expectedOrder := []string{
		"id",
		"repository_id",
		"file_path",
		"chunk_type",
		"content",
		"language",
		"start_line",
		"end_line",
		"entity_name",
		"parent_entity",
		"content_hash",
		"metadata",
		"qualified_name",
		"signature",
		"visibility",
		"token_count",
		"token_counted_at",
	}

	// Extract the column list from INSERT clause
	insertStart := strings.Index(query, "INSERT INTO codechunking.code_chunks")
	valuesStart := strings.Index(query, "VALUES")
	if insertStart == -1 || valuesStart == -1 {
		t.Fatal("Query missing INSERT or VALUES clause")
	}

	columnSection := query[insertStart:valuesStart]

	// Verify each column appears in order
	lastIndex := -1
	for i, col := range expectedOrder {
		index := strings.Index(columnSection, col)
		if index == -1 {
			t.Errorf("Column %d (%s) not found in INSERT clause", i+1, col)
			continue
		}
		if index <= lastIndex {
			t.Errorf("Column %d (%s) appears out of order (should appear after column %d)", i+1, col, i)
		}
		lastIndex = index
	}
}
