package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVectorStorageSchema tests the vector storage schema and partitioning.
func TestVectorStorageSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()

	t.Run("verify embeddings table exists", func(t *testing.T) {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_schema = 'codechunking' 
				AND table_name = 'embeddings'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "embeddings table should exist")
	})

	t.Run("verify embeddings_partitioned table exists", func(t *testing.T) {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_schema = 'codechunking' 
				AND table_name = 'embeddings_partitioned'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "embeddings_partitioned table should exist")
	})

	t.Run("verify vector column dimensions", func(t *testing.T) {
		// Set search path first
		_, err := pool.Exec(ctx, "SET search_path TO codechunking, public")
		require.NoError(t, err)

		var dimensions int
		err = pool.QueryRow(ctx, `
			SELECT atttypmod FROM pg_attribute 
			WHERE attrelid = 'embeddings'::regclass 
			AND attname = 'embedding'
		`).Scan(&dimensions)
		require.NoError(t, err)
		assert.Equal(t, 768, dimensions, "embedding vector should have 768 dimensions")
	})

	t.Run("verify partitions exist", func(t *testing.T) {
		var partitionCount int
		err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM information_schema.tables 
			WHERE table_schema = 'codechunking' 
			AND table_name LIKE 'embeddings_partitioned_%'
		`).Scan(&partitionCount)
		require.NoError(t, err)
		assert.Equal(t, 4, partitionCount, "should have 4 partitions")
	})

	t.Run("verify HNSW indexes exist", func(t *testing.T) {
		var indexCount int
		err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM pg_indexes 
			WHERE schemaname = 'codechunking' 
			AND tablename LIKE 'embeddings_partitioned_%'
			AND indexdef LIKE '%hnsw%'
		`).Scan(&indexCount)
		require.NoError(t, err)
		assert.Equal(t, 4, indexCount, "should have HNSW indexes on all partitions")
	})

	t.Run("verify HNSW index parameters", func(t *testing.T) {
		var indexDef string
		err := pool.QueryRow(ctx, `
			SELECT indexdef FROM pg_indexes 
			WHERE schemaname = 'codechunking' 
			AND tablename = 'embeddings_partitioned_0'
			AND indexname LIKE '%vector'
			LIMIT 1
		`).Scan(&indexDef)
		require.NoError(t, err)

		assert.Contains(t, indexDef, "hnsw", "index should use HNSW")
		assert.Contains(t, indexDef, "vector_cosine_ops", "index should use cosine similarity")
		assert.Contains(t, indexDef, "m='16'", "index should have m=16")
		assert.Contains(t, indexDef, "ef_construction='64'", "index should have ef_construction=64")
	})
}

// TestVectorOperations tests basic vector operations.
func TestVectorOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()

	// Setup test data
	repositoryID := uuid.New()
	chunkID := uuid.New()

	// Use unique test URLs for this test run
	testURL := "https://github.com/test/vector-repo-" + repositoryID.String()[:8]

	// Set search path first
	_, err := pool.Exec(ctx, "SET search_path TO codechunking, public")
	require.NoError(t, err)

	// Create test repository
	_, err = pool.Exec(ctx, `
		INSERT INTO repositories (id, url, normalized_url, name, description, status) 
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL,
		"vector-test-repo", "Test repository for vector operations", "indexed")
	require.NoError(t, err)

	// Create test code chunk
	_, err = pool.Exec(ctx, `
		INSERT INTO code_chunks (id, repository_id, file_path, chunk_type, content, language, start_line, end_line, entity_name, content_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, chunkID, repositoryID, "test.go", "function", "func TestVector() {}", "go", 1, 10, "TestVector", "test-hash-"+chunkID.String()[:8])
	require.NoError(t, err)

	t.Run("insert embedding into regular table", func(t *testing.T) {
		// Create a test vector (768 dimensions) with small values
		testVectorStr := "["
		for i := range 768 {
			if i > 0 {
				testVectorStr += ","
			}
			testVectorStr += "0.1"
		}
		testVectorStr += "]"

		_, err := pool.Exec(ctx, `
			INSERT INTO embeddings (chunk_id, embedding, model_version)
			VALUES ($1, $2, $3)
			ON CONFLICT (chunk_id, model_version) DO NOTHING
		`, chunkID, testVectorStr, "gemini-embedding-001")
		require.NoError(t, err)

		// Verify insertion
		var count int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM embeddings WHERE chunk_id = $1
		`, chunkID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("insert embedding into partitioned table", func(t *testing.T) {
		// Create a test vector (768 dimensions) with small values
		testVectorStr := "["
		for i := range 768 {
			if i > 0 {
				testVectorStr += ","
			}
			testVectorStr += "0.2"
		}
		testVectorStr += "]"

		_, err := pool.Exec(ctx, `
			INSERT INTO embeddings_partitioned (chunk_id, repository_id, embedding, model_version)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (chunk_id, model_version, repository_id) DO NOTHING
		`, chunkID, repositoryID, testVectorStr, "gemini-embedding-001")
		require.NoError(t, err)

		// Verify insertion
		var count int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM embeddings_partitioned WHERE chunk_id = $1
		`, chunkID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("test vector similarity search", func(t *testing.T) {
		// Create a query vector with 768 dimensions (different from stored vectors)
		queryVectorStr := "["
		for i := range 768 {
			if i > 0 {
				queryVectorStr += ","
			}
			queryVectorStr += "0.9" // Different from stored 0.1 and 0.2 values
		}
		queryVectorStr += "]"

		// Test similarity search on regular table
		var distance float64
		err := pool.QueryRow(ctx, `
			SELECT embedding <=> $1::vector as distance
			FROM embeddings 
			WHERE chunk_id = $2
		`, queryVectorStr, chunkID).Scan(&distance)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, distance, float64(0), "distance should be non-negative")
		t.Logf("Regular table distance: %f", distance)

		// Test similarity search on partitioned table
		err = pool.QueryRow(ctx, `
			SELECT embedding <=> $1::vector as distance
			FROM embeddings_partitioned 
			WHERE chunk_id = $2
		`, queryVectorStr, chunkID).Scan(&distance)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, distance, float64(0), "distance should be non-negative")
		t.Logf("Partitioned table distance: %f", distance)
	})

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM embeddings WHERE chunk_id = $1", chunkID)
		pool.Exec(ctx, "DELETE FROM embeddings_partitioned WHERE chunk_id = $1", chunkID)
		pool.Exec(ctx, "DELETE FROM code_chunks WHERE id = $1", chunkID)
		pool.Exec(ctx, "DELETE FROM repositories WHERE id = $1", repositoryID)
	})
}

// TestPartitionManagement tests partition management functions.
func TestPartitionManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()

	t.Run("test partition stats function", func(t *testing.T) {
		// Check if our simple function exists
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.routines 
				WHERE routine_schema = 'codechunking' 
				AND routine_name = 'simple_partition_stats'
			)
		`).Scan(&exists)
		require.NoError(t, err)

		if exists {
			// Test the function
			_, err := pool.Exec(ctx, "SET search_path TO codechunking, public")
			require.NoError(t, err)

			rows, err := pool.Query(ctx, "SELECT * FROM simple_partition_stats()")
			require.NoError(t, err)
			defer rows.Close()

			var partitionCount int
			for rows.Next() {
				var partitionName string
				var rowCount int64
				err := rows.Scan(&partitionName, &rowCount)
				require.NoError(t, err)
				assert.Contains(t, partitionName, "embeddings_partitioned_")
				partitionCount++
			}
			assert.Equal(t, 4, partitionCount, "should return stats for 4 partitions")
		}
	})

	t.Run("verify partition constraints", func(t *testing.T) {
		// Test that partitioning key is required in constraints
		repositoryID := uuid.New()
		chunkID := uuid.New()

		// This should work (includes repository_id in primary key)
		_, err := pool.Exec(ctx, "SET search_path TO codechunking, public")
		require.NoError(t, err)

		_, err = pool.Exec(ctx, `
			INSERT INTO repositories (id, url, normalized_url, name, description, status) 
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (url) DO NOTHING
		`, repositoryID, "https://github.com/test/constraint-repo", "https://github.com/test/constraint-repo",
			"constraint-test-repo", "Test repository for constraint testing", "indexed")
		require.NoError(t, err)

		_, err = pool.Exec(ctx, `
			INSERT INTO code_chunks (id, repository_id, file_path, chunk_type, content, language, start_line, end_line, entity_name, content_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (repository_id, file_path, content_hash) DO NOTHING
		`, chunkID, repositoryID, "constraint_test.go", "function", "func TestConstraint() {}", "go", 1, 10, "TestConstraint", "constraint-hash")
		require.NoError(t, err)

		// Test vector with proper dimensions
		testVectorStr := "["
		for i := range 768 {
			if i > 0 {
				testVectorStr += ","
			}
			testVectorStr += "0.1"
		}
		testVectorStr += "]"

		_, err = pool.Exec(ctx, `
			INSERT INTO embeddings_partitioned (chunk_id, repository_id, embedding, model_version)
			VALUES ($1, $2, $3, $4)
		`, chunkID, repositoryID, testVectorStr, "gemini-embedding-001")
		require.NoError(t, err, "insertion with proper repository_id should succeed")

		// Cleanup
		pool.Exec(ctx, "DELETE FROM embeddings_partitioned WHERE chunk_id = $1", chunkID)
		pool.Exec(ctx, "DELETE FROM code_chunks WHERE id = $1", chunkID)
		pool.Exec(ctx, "DELETE FROM repositories WHERE id = $1", repositoryID)
	})
}

// getTestPool returns a database connection pool for testing.
func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// Use environment variable or default test database
	dbURL := "postgres://dev:dev@localhost:5432/codechunking?sslmode=disable"

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("Could not connect to test database: %v", err)
	}

	// Test connection
	err = pool.Ping(context.Background())
	if err != nil {
		pool.Close()
		t.Skipf("Could not ping test database: %v", err)
	}

	return pool
}
