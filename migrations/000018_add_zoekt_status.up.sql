-- Add Zoekt and embedding index status fields to repositories table
-- This enables tracking of search engine indexing status independently

-- Set search path
SET search_path TO codechunking, public;

-- Add new columns to track dual-engine indexing status
ALTER TABLE repositories
ADD COLUMN zoekt_index_status VARCHAR(20) DEFAULT 'pending' NOT NULL,
ADD COLUMN embedding_index_status VARCHAR(20) DEFAULT 'pending' NOT NULL,
ADD COLUMN zoekt_last_indexed_at TIMESTAMP NULL,
ADD COLUMN zoekt_shard_count INT DEFAULT 0 NOT NULL,
ADD COLUMN zoekt_commit_hash VARCHAR(40) NULL;

-- Create indexes for efficient queries by index status
CREATE INDEX idx_repositories_zoekt_status ON repositories(zoekt_index_status);
CREATE INDEX idx_repositories_embedding_status ON repositories(embedding_index_status);

-- Add check constraints to ensure valid status values
ALTER TABLE repositories
ADD CONSTRAINT chk_zoekt_index_status CHECK (zoekt_index_status IN ('pending', 'indexing', 'completed', 'failed', 'partial')),
ADD CONSTRAINT chk_embedding_index_status CHECK (embedding_index_status IN ('pending', 'generating', 'completed', 'failed', 'partial'));

-- Backward compatibility: set embedding status to 'completed' for existing completed repositories
UPDATE repositories
SET embedding_index_status = 'completed'
WHERE status = 'completed';

-- Log completion for audit purposes
DO $$
BEGIN
    RAISE NOTICE 'Zoekt and embedding index status columns added to repositories table';
    RAISE NOTICE 'Existing completed repositories marked with embedding_index_status = completed';
END $$;