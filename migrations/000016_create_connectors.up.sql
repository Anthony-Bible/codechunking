-- Migration: create connectors table
-- This table stores git connector configurations used to automatically
-- discover and index repositories from external git hosting services.

SET search_path TO codechunking, public;

CREATE TABLE IF NOT EXISTS codechunking.connectors (
    id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             VARCHAR(255) NOT NULL UNIQUE,
    connector_type   VARCHAR(50)  NOT NULL,
    base_url         VARCHAR(512) NOT NULL,
    auth_token       TEXT,
    status           VARCHAR(50)  NOT NULL DEFAULT 'pending',
    repository_count INTEGER      NOT NULL DEFAULT 0,
    last_sync_at     TIMESTAMP WITH TIME ZONE,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_connectors_connector_type ON codechunking.connectors (connector_type);
CREATE INDEX IF NOT EXISTS idx_connectors_status         ON codechunking.connectors (status);
CREATE INDEX IF NOT EXISTS idx_connectors_name           ON codechunking.connectors (name);

-- Auto-update updated_at on row modification
CREATE TRIGGER update_connectors_updated_at
    BEFORE UPDATE ON codechunking.connectors
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
