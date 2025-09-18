-- Rollback migration for enhanced chunk type fields

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Drop the indexes for type-related columns
DROP INDEX IF EXISTS idx_code_chunks_visibility;
DROP INDEX IF EXISTS idx_code_chunks_qualified_name;

-- Remove the enhanced type columns
ALTER TABLE code_chunks
  DROP COLUMN IF EXISTS visibility,
  DROP COLUMN IF EXISTS signature,
  DROP COLUMN IF EXISTS qualified_name;

-- Revert chunk_type values back to generic 'code' for backward compatibility
UPDATE code_chunks
SET chunk_type = 'code'
WHERE chunk_type = 'fragment';