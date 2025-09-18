-- Migration to enhance code chunks with comprehensive type information
-- This migration adds semantic type fields to enable better search and organization

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Add new columns for enhanced type information
ALTER TABLE code_chunks
  ADD COLUMN IF NOT EXISTS qualified_name VARCHAR(512),
  ADD COLUMN IF NOT EXISTS signature TEXT,
  ADD COLUMN IF NOT EXISTS visibility VARCHAR(20);

-- Create indexes for the new type-related columns for efficient queries
CREATE INDEX IF NOT EXISTS idx_code_chunks_qualified_name ON code_chunks(qualified_name);
CREATE INDEX IF NOT EXISTS idx_code_chunks_visibility ON code_chunks(visibility);

-- Update existing records to have more descriptive chunk_type values
-- Convert generic 'code' type to 'fragment' for better semantic clarity
UPDATE code_chunks
SET chunk_type = 'fragment'
WHERE chunk_type = 'code' OR chunk_type IS NULL;

-- Add comment to document the enhanced schema
COMMENT ON COLUMN code_chunks.chunk_type IS 'Semantic construct type: function, method, class, struct, interface, enum, variable, constant, module, fragment, etc.';
COMMENT ON COLUMN code_chunks.qualified_name IS 'Fully qualified name of the entity (e.g., package.Class.method)';
COMMENT ON COLUMN code_chunks.signature IS 'Function/method signature with parameters and return types';
COMMENT ON COLUMN code_chunks.visibility IS 'Visibility modifier: public, private, protected, internal';