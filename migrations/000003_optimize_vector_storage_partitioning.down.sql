-- Migration Rollback: Remove Vector Storage Partitioning Optimizations
-- This migration removes the partitioning optimizations for vector storage

-- Set search path
SET search_path TO codechunking, public;

-- ==============================================================================
-- 1. REMOVE MONITORING VIEW
-- ==============================================================================

DROP VIEW IF EXISTS embeddings_partition_monitoring;

-- ==============================================================================
-- 2. REMOVE PARTITION MANAGEMENT FUNCTIONS
-- ==============================================================================

DROP FUNCTION IF EXISTS get_embeddings_partition_stats();
DROP FUNCTION IF EXISTS add_embeddings_partition(TEXT, INTEGER, INTEGER);

-- ==============================================================================
-- 3. REMOVE PARTITIONED TABLE AND ALL PARTITIONS
-- ==============================================================================

-- Drop partitioned table (this will automatically drop all partitions)
DROP TABLE IF EXISTS embeddings_partitioned CASCADE;

-- ==============================================================================
-- 4. RESET HNSW CONFIGURATION
-- ==============================================================================

-- Reset HNSW search parameters to defaults
ALTER SYSTEM RESET hnsw.ef_search;
SELECT pg_reload_conf(); -- Apply configuration changes