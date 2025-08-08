-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create uuid extension for unique identifiers
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create initial schema
CREATE SCHEMA IF NOT EXISTS codechunking;

-- Set search path
SET search_path TO codechunking, public;

-- Repositories table
CREATE TABLE IF NOT EXISTS repositories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url VARCHAR(512) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    default_branch VARCHAR(100),
    last_indexed_at TIMESTAMP WITH TIME ZONE,
    last_commit_hash VARCHAR(40),
    total_files INTEGER DEFAULT 0,
    total_chunks INTEGER DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

-- Indexing jobs table
CREATE TABLE IF NOT EXISTS indexing_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    repository_id UUID REFERENCES repositories(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    files_processed INTEGER DEFAULT 0,
    chunks_created INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

-- Code chunks table
CREATE TABLE IF NOT EXISTS code_chunks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    repository_id UUID REFERENCES repositories(id) ON DELETE CASCADE,
    file_path VARCHAR(512) NOT NULL,
    chunk_type VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    language VARCHAR(50) NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    entity_name VARCHAR(255),
    parent_entity VARCHAR(255),
    content_hash VARCHAR(64) NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    UNIQUE(repository_id, file_path, content_hash)
);

-- Embeddings table with vector column
CREATE TABLE IF NOT EXISTS embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chunk_id UUID REFERENCES code_chunks(id) ON DELETE CASCADE,
    embedding vector(768), -- Gemini text-embedding-004 produces 768-dimensional vectors
    model_version VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    UNIQUE(chunk_id, model_version)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_repositories_url ON repositories(url);
CREATE INDEX IF NOT EXISTS idx_repositories_status ON repositories(status);
CREATE INDEX IF NOT EXISTS idx_repositories_deleted_at ON repositories(deleted_at);
CREATE INDEX IF NOT EXISTS idx_indexing_jobs_repository_id ON indexing_jobs(repository_id);
CREATE INDEX IF NOT EXISTS idx_indexing_jobs_status ON indexing_jobs(status);
CREATE INDEX IF NOT EXISTS idx_indexing_jobs_deleted_at ON indexing_jobs(deleted_at);
CREATE INDEX IF NOT EXISTS idx_code_chunks_repository_id ON code_chunks(repository_id);
CREATE INDEX IF NOT EXISTS idx_code_chunks_file_path ON code_chunks(file_path);
CREATE INDEX IF NOT EXISTS idx_code_chunks_language ON code_chunks(language);
CREATE INDEX IF NOT EXISTS idx_code_chunks_chunk_type ON code_chunks(chunk_type);
CREATE INDEX IF NOT EXISTS idx_code_chunks_deleted_at ON code_chunks(deleted_at);
CREATE INDEX IF NOT EXISTS idx_embeddings_chunk_id ON embeddings(chunk_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_deleted_at ON embeddings(deleted_at);

-- Create HNSW index for vector similarity search
CREATE INDEX IF NOT EXISTS idx_embeddings_vector ON embeddings 
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- Create update timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply update timestamp trigger to tables
CREATE TRIGGER update_repositories_updated_at BEFORE UPDATE ON repositories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_indexing_jobs_updated_at BEFORE UPDATE ON indexing_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_code_chunks_updated_at BEFORE UPDATE ON code_chunks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create views for non-deleted records (soft delete support)
CREATE OR REPLACE VIEW active_repositories AS
    SELECT * FROM repositories WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW active_indexing_jobs AS
    SELECT * FROM indexing_jobs WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW active_code_chunks AS
    SELECT * FROM code_chunks WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW active_embeddings AS
    SELECT * FROM embeddings WHERE deleted_at IS NULL;

-- Grant permissions
GRANT ALL PRIVILEGES ON SCHEMA codechunking TO dev;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA codechunking TO dev;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA codechunking TO dev;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA codechunking TO dev;