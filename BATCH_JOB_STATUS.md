# Batch Job Status Reference

This document explains how to check the status of both **repository indexing jobs** (database) and **batch embedding jobs** (Google Gemini API).

## Quick Start

```bash
# Check database indexing jobs (repository processing)
./scripts/check_jobs.sh db

# List all batch embedding jobs (requires API key)
./scripts/check_jobs.sh batch-list

# Check specific batch job
./scripts/check_jobs.sh batch <job-id>
```

## Understanding Job Types

### 1. Repository Indexing Jobs (Database)

**What they are**: Jobs that process repositories (git clone, parse, chunk files)
**Storage**: PostgreSQL `codechunking.indexing_jobs` table
**Purpose**: Track the overall repository indexing workflow

**Status values**:
- `pending` - Job queued
- `running` - Currently processing
- `completed` - Successfully finished
- `failed` - Encountered an error
- `cancelled` - Manually cancelled

**Fields tracked**:
- Repository information
- Files processed count
- Chunks created count
- Start/completion timestamps
- Error messages (if failed)

### 2. Batch Embedding Jobs (External API)

**What they are**: Asynchronous embedding generation jobs via Google Gemini Batches API
**Storage**: Google Cloud (accessed via API)
**Purpose**: Generate embeddings for large numbers of code chunks efficiently

**Status values**:
- `PENDING` - Job queued in Google's system
- `PROCESSING` - Google is generating embeddings
- `COMPLETED` - All embeddings generated
- `FAILED` - Job failed
- `CANCELLED` - Job was cancelled

**Fields tracked**:
- Total count of items
- Processed/success/error counts
- Progress percentage
- Processing rate
- Output file URIs

## Configuration Requirements for Production Batch Processing

### Prerequisites for Using Google Gemini Batches API

To use production batch processing (not test mode), you must configure:

1. **API Key**: Set the `CODECHUNK_GEMINI_API_KEY` environment variable
2. **Batch Directories**: Configure input/output directories in your config file
3. **Disable Test Mode**: Set `use_test_embeddings: false` in batch processing config

**Configuration Example** (`configs/config.dev.yaml`):
```yaml
gemini:
  api_key: ${CODECHUNK_GEMINI_API_KEY}  # Set via environment
  batch:
    enabled: true
    input_dir: /tmp/batch_embeddings/input    # Required for production
    output_dir: /tmp/batch_embeddings/output  # Required for production
    poll_interval: 5s
    max_wait_time: 30m

batch_processing:
  enabled: true
  threshold_chunks: 10  # Repositories with >10 chunks use batch processing
  use_test_embeddings: false  # IMPORTANT: Set to false for production batch API
  fallback_to_sequential: true
```

### Verifying Batch Processing is Active

When batch processing is correctly configured, worker logs will show:

```
"Processing batch embedding results (PRODUCTION MODE)"
"chunk_count": 8269
"Using batch embeddings API for production"
```

If you see this message instead, batch processing is **NOT** active:
```
"Production batch processing not implemented - falling back to sequential"
```

This fallback occurs when:
- `use_test_embeddings: true` (test mode enabled)
- Missing `input_dir` or `output_dir` configuration
- Batch processing disabled (`enabled: false`)

## Detailed Usage

### Check Database Jobs

```bash
# Using the wrapper script (recommended)
./scripts/check_jobs.sh db

# Or use SQL directly
psql -U dev -d codechunking -f check_indexing_jobs.sql

# Or via Docker
docker exec codechunking-postgres psql -U dev -d codechunking -c \
  "SELECT * FROM codechunking.indexing_jobs WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT 10;"
```

**Example output**:
```
 job_id                               | status    | repository_name | files_processed | chunks_created
--------------------------------------+-----------+-----------------+-----------------+---------------
 a1b2c3d4-e5f6-7890-abcd-ef1234567890 | completed | my-repo         | 150             | 3421
 b2c3d4e5-f6g7-8901-bcde-fg2345678901 | running   | another-repo    | 45              | 987
```

### Check Batch Embedding Jobs

**Prerequisites**:
```bash
export CODECHUNK_GEMINI_API_KEY=your-api-key-here
```

**List all batch jobs**:
```bash
./scripts/check_jobs.sh batch-list

# Or filter by state
go run scripts/check_batch_jobs.go -list -state COMPLETED
```

**Check specific job**:
```bash
./scripts/check_jobs.sh batch projects/PROJECT_ID/locations/us-central1/batchJobs/12345

# Or using Go directly
go run scripts/check_batch_jobs.go -job-id "projects/PROJECT_ID/locations/us-central1/batchJobs/12345"
```

**Example output**:
```
Job Details:
────────────────────────────────────────
Job ID:         projects/.../batchJobs/12345
State:          COMPLETED
Model:          gemini-embedding-001
Total Count:    5000
Processed:      5000
Success:        4998
Errors:         2
Created At:     2025-11-22T10:30:00Z
Updated At:     2025-11-22T10:45:23Z
Completed At:   2025-11-22T10:45:23Z
Duration:       15m23s

Progress:
  Percent:      100.00%
  Remaining:    0 items
  Rate:         5.45 items/sec

Output File:    /tmp/batch_embeddings/output/batch_output_20251122_104523.jsonl

✓ Job completed successfully!
```

## SQL Queries

### Common Database Queries

**All active jobs**:
```sql
SELECT ij.id, ij.status, r.name, ij.files_processed, ij.chunks_created
FROM codechunking.indexing_jobs ij
LEFT JOIN codechunking.repositories r ON r.id = ij.repository_id
WHERE ij.deleted_at IS NULL
ORDER BY ij.created_at DESC;
```

**Failed jobs with errors**:
```sql
SELECT ij.id, r.name, ij.error_message, ij.started_at, ij.completed_at
FROM codechunking.indexing_jobs ij
JOIN codechunking.repositories r ON r.id = ij.repository_id
WHERE ij.status = 'failed' AND ij.deleted_at IS NULL
ORDER BY ij.created_at DESC;
```

**Jobs by status summary**:
```sql
SELECT status, COUNT(*) as count,
       SUM(files_processed) as total_files,
       SUM(chunks_created) as total_chunks
FROM codechunking.indexing_jobs
WHERE deleted_at IS NULL
GROUP BY status;
```

## Important Notes

### Batch Embedding Jobs

1. **Not stored in database**: Batch jobs are managed entirely by Google's API
2. **Require API key**: Must set `CODECHUNK_GEMINI_API_KEY` to check status
3. **Job ID format**: Full path like `projects/PROJECT_ID/locations/REGION/batchJobs/JOB_ID`
4. **Transient storage**: Input/output files are in `/tmp/batch_embeddings/`

### Repository Indexing Jobs

1. **Stored in PostgreSQL**: Persisted in the database
2. **No API key needed**: Direct database access
3. **Linked to repositories**: Foreign key to repositories table
4. **Soft deletions**: Check `deleted_at IS NULL` for active jobs

## Troubleshooting

### Database jobs not showing up

```bash
# Ensure database is running
make dev

# Check if migrations ran
make migrate-up

# Verify table exists
docker exec codechunking-postgres psql -U dev -d codechunking -c "\dt codechunking.*"
```

### "Falling back to sequential" - Batch processing not working

**Symptom**: Worker logs show "Production batch processing not implemented - falling back to sequential"

**Causes and Solutions**:

1. **Test mode is enabled** (most common):
   ```yaml
   # In configs/config.dev.yaml
   batch_processing:
     use_test_embeddings: true  # ← Change this to false
   ```
   **Fix**: Set `use_test_embeddings: false` for production batch processing

2. **Missing batch directories**:
   ```yaml
   # In configs/config.dev.yaml or configs/config.yaml
   gemini:
     batch:
       enabled: true
       # Missing: input_dir and output_dir!
   ```
   **Fix**: Add directory configuration:
   ```yaml
   gemini:
     batch:
       enabled: true
       input_dir: /tmp/batch_embeddings/input
       output_dir: /tmp/batch_embeddings/output
   ```

3. **Batch processing disabled**:
   ```yaml
   gemini:
     batch:
       enabled: false  # ← Should be true
   ```
   **Fix**: Set `enabled: true`

4. **Chunk count below threshold**:
   - Default threshold is 10 chunks
   - Repositories with ≤10 chunks use sequential processing automatically
   - Check `batch_processing.threshold_chunks` in config

**Verification**: After fixing, restart the worker and look for:
```
"Processing batch embedding results (PRODUCTION MODE)"
```

### Batch jobs failing to list

```bash
# Check API key is set
echo $CODECHUNK_GEMINI_API_KEY

# Verify network connectivity
curl -H "Authorization: Bearer $CODECHUNK_GEMINI_API_KEY" \
  https://generativelanguage.googleapis.com/v1beta/models

# Check batch processing is enabled in config
grep -A 5 "batch:" configs/config.dev.yaml
```

### Permission errors

```bash
# Ensure scripts are executable
chmod +x scripts/check_jobs.sh

# Check file permissions on batch directories
ls -la /tmp/batch_embeddings/
```

### Google API File Download Errors (403 PERMISSION_DENIED)

**Symptom**: Worker logs show:
```
"Failed to get file metadata from Google Files API"
"Error 403, Message: You do not have permission to access the File..."
"error_detail": "This error typically occurs when Google's Batch API returns file IDs that exceed the Files API 40-character limit"
```

**Root Cause**: Google's Batch API returns file IDs that are 42 characters long (e.g., `batch-pwccfe96og36g8qngof6db1dsnywrs1hxhd3`), which exceeds the Files API's documented 40-character limit. This is an inconsistency in Google's API design.

**How the System Handles This**:
1. The code attempts to download results using the full file ID from Google's Files API
2. If Files.Get() fails with a permission error (due to the ID length issue), the system logs a detailed error
3. The system then tries to fall back to local file paths if the file was already downloaded

**Verification**: Check your worker logs for:
```
"Attempting download with full file ID"
"file_id": "batch-pwccfe96og36g8qngof6db1dsnywrs1hxhd3"
"id_length": 42
```

If you see `id_length > 40`, this is the Google API limitation issue.

**Resolution**:
- **Automatic Fix Implemented**: The system now automatically tries multiple strategies:
  1. First attempt: Try full file ID with Files API
  2. Second attempt: If 40-char error detected, remove "batch-" prefix and retry
  3. Third attempt: Fall back to local file handling if available
- **Expected**: Google should fix this API inconsistency on their end
- **Manual Workaround** (if automatic fix fails):
  1. Ensure output directory exists: `/tmp/batch_embeddings/output/`
  2. Check if files are being created in the output directory manually by Google
  3. Verify your API key has proper permissions for both Batch API and Files API

**What to Look For in Logs**:
When the fix is working, you'll see:
```
"File ID exceeds 40-character limit, trying alternative strategies"
"Attempting download without 'batch-' prefix"
"Successfully retrieved File object with full ID"
```

If all strategies fail, you'll see:
```
"All Files API strategies failed, attempting direct batch result download"
```

**Related Files**:
- Implementation: `internal/adapter/outbound/gemini/batch_embedding_client.go:671-760`
- Error handling includes multi-strategy retry logic with detailed logging at each step

## Integration Points

### Where Jobs Are Created

**Repository Indexing Jobs**:
- Created when: Repository added via API (`POST /repositories`)
- Processed by: Worker service (`codechunking worker`)
- Queue: NATS JetStream (`codechunk.indexing.jobs`)

**Batch Embedding Jobs**:
- Created when: Worker processes repositories with batch embeddings enabled
- Config: `configs/config.dev.yaml` → `gemini.batch.enabled: true`
- Triggered by: `BatchEmbeddingClient.CreateBatchEmbeddingJob()`

### Configuration

**Enable batch processing** (`configs/config.dev.yaml`):
```yaml
gemini:
  batch:
    enabled: true
    poll_interval: 5s
    max_wait_time: 30m
```

**Batch directories**:
```yaml
# Defaults if not specified:
input_dir:  /tmp/batch_embeddings/input
output_dir: /tmp/batch_embeddings/output
```

## Files Reference

| File | Purpose |
|------|---------|
| `scripts/check_jobs.sh` | Main wrapper script for checking jobs |
| `scripts/check_batch_jobs.go` | Go program to query Google Gemini Batches API |
| `check_indexing_jobs.sql` | SQL queries for database jobs |
| `internal/adapter/outbound/gemini/batch_embedding_client.go` | Batch job client implementation |

## See Also

- [Google Gemini Batches API Documentation](https://ai.google.dev/gemini-api/docs/batch)
- Project CLAUDE.md for development commands
- `make help` for available Makefile targets
