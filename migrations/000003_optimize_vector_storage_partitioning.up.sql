-- Migration: Optimize Vector Storage with Partitioning
-- This migration implements partitioning strategies for optimal vector storage performance

-- Enable required extensions (if not already enabled)
CREATE EXTENSION IF NOT EXISTS "vector";
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Set search path
SET search_path TO codechunking, public;

-- ==============================================================================
-- 1. PARTITIONED EMBEDDINGS TABLE FOR LARGE SCALE REPOSITORIES
-- ==============================================================================

-- Create new partitioned embeddings table for large repositories
CREATE TABLE IF NOT EXISTS embeddings_partitioned (
    id UUID DEFAULT uuid_generate_v4(),
    chunk_id UUID NOT NULL,
    repository_id UUID NOT NULL, -- Added for efficient partitioning
    embedding vector(768), -- Gemini text-embedding-004 produces 768-dimensional vectors
    model_version VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    PRIMARY KEY (id, repository_id), -- Include partition key in primary key
    UNIQUE(chunk_id, model_version, repository_id) -- Include partition key in unique constraint
) PARTITION BY HASH (repository_id); -- Hash partitioning for even distribution

-- Create initial partitions (4 partitions for balanced load distribution)
CREATE TABLE embeddings_partitioned_0 PARTITION OF embeddings_partitioned
    FOR VALUES WITH (modulus 4, remainder 0);

CREATE TABLE embeddings_partitioned_1 PARTITION OF embeddings_partitioned
    FOR VALUES WITH (modulus 4, remainder 1);

CREATE TABLE embeddings_partitioned_2 PARTITION OF embeddings_partitioned
    FOR VALUES WITH (modulus 4, remainder 2);

CREATE TABLE embeddings_partitioned_3 PARTITION OF embeddings_partitioned
    FOR VALUES WITH (modulus 4, remainder 3);

-- ==============================================================================
-- 2. OPTIMIZED INDEXES FOR PARTITIONED TABLES
-- ==============================================================================

-- Create HNSW indexes on each partition for optimal vector search
-- Using optimal parameters: m=16, ef_construction=64, cosine similarity
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_0_vector 
    ON embeddings_partitioned_0 
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_1_vector 
    ON embeddings_partitioned_1 
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_2_vector 
    ON embeddings_partitioned_2 
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_3_vector 
    ON embeddings_partitioned_3 
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- Create supporting indexes on each partition
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_0_chunk_id 
    ON embeddings_partitioned_0 (chunk_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_0_repository_id 
    ON embeddings_partitioned_0 (repository_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_0_deleted_at 
    ON embeddings_partitioned_0 (deleted_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_1_chunk_id 
    ON embeddings_partitioned_1 (chunk_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_1_repository_id 
    ON embeddings_partitioned_1 (repository_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_1_deleted_at 
    ON embeddings_partitioned_1 (deleted_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_2_chunk_id 
    ON embeddings_partitioned_2 (chunk_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_2_repository_id 
    ON embeddings_partitioned_2 (repository_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_2_deleted_at 
    ON embeddings_partitioned_2 (deleted_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_3_chunk_id 
    ON embeddings_partitioned_3 (chunk_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_3_repository_id 
    ON embeddings_partitioned_3 (repository_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_partitioned_3_deleted_at 
    ON embeddings_partitioned_3 (deleted_at);

-- ==============================================================================
-- 3. PERFORMANCE TUNING CONFIGURATION
-- ==============================================================================

-- Optimize HNSW search parameters for better recall
-- These can be adjusted at runtime based on performance requirements
ALTER SYSTEM SET hnsw.ef_search = 100; -- Higher recall, slightly slower queries

-- ==============================================================================
-- 4. PARTITION MANAGEMENT FUNCTIONS
-- ==============================================================================

-- Function to add new partitions dynamically
CREATE OR REPLACE FUNCTION add_embeddings_partition(partition_name TEXT, modulus INTEGER, remainder INTEGER)
RETURNS VOID AS $$
DECLARE
    sql_statement TEXT;
BEGIN
    -- Create partition
    sql_statement := format('CREATE TABLE %I PARTITION OF embeddings_partitioned FOR VALUES WITH (modulus %s, remainder %s)', 
                           partition_name, modulus, remainder);
    EXECUTE sql_statement;
    
    -- Add HNSW vector index
    sql_statement := format('CREATE INDEX CONCURRENTLY %I ON %I USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64)', 
                           'idx_' || partition_name || '_vector', partition_name);
    EXECUTE sql_statement;
    
    -- Add supporting indexes
    sql_statement := format('CREATE INDEX CONCURRENTLY %I ON %I (chunk_id)', 
                           'idx_' || partition_name || '_chunk_id', partition_name);
    EXECUTE sql_statement;
    
    sql_statement := format('CREATE INDEX CONCURRENTLY %I ON %I (repository_id)', 
                           'idx_' || partition_name || '_repository_id', partition_name);
    EXECUTE sql_statement;
    
    sql_statement := format('CREATE INDEX CONCURRENTLY %I ON %I (deleted_at)', 
                           'idx_' || partition_name || '_deleted_at', partition_name);
    EXECUTE sql_statement;
END;
$$ LANGUAGE plpgsql;

-- Function to get partition statistics
CREATE OR REPLACE FUNCTION get_embeddings_partition_stats()
RETURNS TABLE(
    partition_name TEXT,
    row_count BIGINT,
    size_pretty TEXT,
    index_size_pretty TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        schemaname||'.'||tablename as partition_name,
        n_tup_ins - n_tup_del as row_count,
        pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size_pretty,
        pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as index_size_pretty
    FROM pg_stat_user_tables 
    WHERE tablename LIKE 'embeddings_partitioned_%'
    ORDER BY tablename;
END;
$$ LANGUAGE plpgsql;

-- ==============================================================================
-- 5. FOREIGN KEY CONSTRAINTS (DEFERRED FOR PERFORMANCE)
-- ==============================================================================

-- Add foreign key constraint to code_chunks (with proper partitioning support)
-- Note: We reference chunk_id directly rather than using repository_id FK
-- to avoid cross-partition constraint issues
ALTER TABLE embeddings_partitioned 
ADD CONSTRAINT fk_embeddings_partitioned_chunk_id 
FOREIGN KEY (chunk_id) REFERENCES code_chunks(id) ON DELETE CASCADE;

-- ==============================================================================
-- 6. COMMENTS AND DOCUMENTATION
-- ==============================================================================

COMMENT ON TABLE embeddings_partitioned IS 
'Partitioned embeddings table optimized for large-scale vector storage. Uses hash partitioning by repository_id for balanced distribution.';

COMMENT ON COLUMN embeddings_partitioned.repository_id IS 
'Repository ID added for efficient partitioning and query optimization.';

COMMENT ON FUNCTION add_embeddings_partition(TEXT, INTEGER, INTEGER) IS 
'Utility function to add new partitions with optimized indexes for horizontal scaling.';

COMMENT ON FUNCTION get_embeddings_partition_stats() IS 
'Monitoring function to track partition sizes and row counts for capacity planning.';

-- ==============================================================================
-- 7. PERFORMANCE MONITORING VIEW
-- ==============================================================================

-- Create view for monitoring partition performance
CREATE OR REPLACE VIEW embeddings_partition_monitoring AS
SELECT 
    schemaname||'.'||tablename as partition_name,
    n_tup_ins - n_tup_del as row_count,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size_pretty,
    pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as index_size_pretty,
    seq_scan,
    seq_tup_read,
    idx_scan,
    idx_tup_fetch,
    n_tup_ins,
    n_tup_upd,
    n_tup_del
FROM pg_stat_user_tables 
WHERE tablename LIKE 'embeddings_partitioned_%'
ORDER BY tablename;

COMMENT ON VIEW embeddings_partition_monitoring IS 
'Performance monitoring view for embeddings partitions showing size, usage, and access patterns.';