-- Rollback migration for normalized_url column and constraint

SET search_path TO codechunking, public;

-- Drop index
DROP INDEX IF EXISTS idx_repositories_normalized_url;

-- Drop unique constraint
ALTER TABLE repositories 
DROP CONSTRAINT IF EXISTS uk_repositories_normalized_url;

-- Drop normalized_url column
ALTER TABLE repositories 
DROP COLUMN IF EXISTS normalized_url;