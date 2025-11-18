-- Migration: Add metadata columns to embeddings_partitioned for SQL-level filtering
-- Purpose: Enable iterative scanning to work properly with language, chunk_type, and file_path filters
-- Issue: #7 - Enable iterative scanning with metadata filtering

BEGIN;

-- Add metadata columns to embeddings_partitioned table
-- These columns are denormalized from code_chunks for performance
ALTER TABLE codechunking.embeddings_partitioned
  ADD COLUMN language VARCHAR(50),
  ADD COLUMN chunk_type VARCHAR(50),
  ADD COLUMN file_path VARCHAR(512);

-- Create indexes for common filter patterns
-- Single-column indexes for individual filter queries
CREATE INDEX idx_embeddings_partitioned_language
  ON codechunking.embeddings_partitioned (language)
  WHERE deleted_at IS NULL;

CREATE INDEX idx_embeddings_partitioned_chunk_type
  ON codechunking.embeddings_partitioned (chunk_type)
  WHERE deleted_at IS NULL;

-- Composite indexes for combined filter queries (most common patterns)
-- Repository + Language (e.g., "Go code in auth-service repo")
CREATE INDEX idx_embeddings_partitioned_repo_lang
  ON codechunking.embeddings_partitioned (repository_id, language)
  WHERE deleted_at IS NULL;

-- Repository + Chunk Type (e.g., "Functions in payment-processor repo")
CREATE INDEX idx_embeddings_partitioned_repo_type
  ON codechunking.embeddings_partitioned (repository_id, chunk_type)
  WHERE deleted_at IS NULL;

-- Language + Chunk Type (e.g., "Python classes across all repos")
CREATE INDEX idx_embeddings_partitioned_lang_type
  ON codechunking.embeddings_partitioned (language, chunk_type)
  WHERE deleted_at IS NULL;

-- Three-column composite for highly selective queries
-- Repository + Language + Chunk Type (e.g., "Go functions in auth-service")
CREATE INDEX idx_embeddings_partitioned_repo_lang_type
  ON codechunking.embeddings_partitioned (repository_id, language, chunk_type)
  WHERE deleted_at IS NULL;

COMMIT;
