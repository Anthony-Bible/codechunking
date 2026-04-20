CREATE INDEX IF NOT EXISTS idx_code_chunks_repo_path_lines_active
ON codechunking.code_chunks(repository_id, file_path, start_line, end_line)
WHERE deleted_at IS NULL;
