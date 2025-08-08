-- Rollback initial schema migration

-- Drop triggers
DROP TRIGGER IF EXISTS update_repositories_updated_at ON repositories;
DROP TRIGGER IF EXISTS update_indexing_jobs_updated_at ON indexing_jobs;
DROP TRIGGER IF EXISTS update_code_chunks_updated_at ON code_chunks;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_embeddings_vector;
DROP INDEX IF EXISTS idx_embeddings_deleted_at;
DROP INDEX IF EXISTS idx_embeddings_chunk_id;

DROP INDEX IF EXISTS idx_code_chunks_deleted_at;
DROP INDEX IF EXISTS idx_code_chunks_chunk_type;
DROP INDEX IF EXISTS idx_code_chunks_language;
DROP INDEX IF EXISTS idx_code_chunks_file_path;
DROP INDEX IF EXISTS idx_code_chunks_repository_id;

DROP INDEX IF EXISTS idx_indexing_jobs_deleted_at;
DROP INDEX IF EXISTS idx_indexing_jobs_status;
DROP INDEX IF EXISTS idx_indexing_jobs_repository_id;

DROP INDEX IF EXISTS idx_repositories_deleted_at;
DROP INDEX IF EXISTS idx_repositories_status;
DROP INDEX IF EXISTS idx_repositories_url;

-- Drop tables (in reverse order of dependencies)
DROP TABLE IF EXISTS embeddings;
DROP TABLE IF EXISTS code_chunks;
DROP TABLE IF EXISTS indexing_jobs;
DROP TABLE IF EXISTS repositories;

-- Drop schema
DROP SCHEMA IF EXISTS codechunking CASCADE;