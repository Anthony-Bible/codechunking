# API Documentation

This document provides comprehensive documentation for the CodeChunking REST API.

## Base Information

- **Base URL**: `http://localhost:8080/api/v1` (development)
- **API Version**: 1.0.0
- **Content Type**: `application/json`
- **Authentication**: Currently none required
- **OpenAPI Specification**: Available at `/api/openapi.yaml`

## Health Monitoring

### GET /health

Returns comprehensive health status for the service and all dependencies.

**Response Codes:**
- `200`: Service is healthy
- `503`: Service is unhealthy or degraded

**Response Headers:**
- `X-Health-Check-Duration`: Health check execution time (e.g., "1.2ms")
- `X-NATS-Connection-Status`: NATS connection status ("connected" or "disconnected")
- `X-JetStream-Enabled`: JetStream availability ("enabled" or "disabled")

**Response Body:**
```json
{
  "status": "healthy|degraded|unhealthy",
  "timestamp": "2025-01-08T10:30:00Z",
  "version": "1.0.0",
  "dependencies": {
    "database": {
      "status": "healthy",
      "message": "",
      "response_time": "0.5ms"
    },
    "nats": {
      "status": "healthy",
      "message": "",
      "response_time": "23.5µs",
      "details": {
        "nats_health": {
          "connected": true,
          "uptime": "2h15m30s",
          "reconnects": 0,
          "last_error": "",
          "jetstream_enabled": true,
          "circuit_breaker": "closed",
          "message_metrics": {
            "published_count": 15420,
            "failed_count": 0,
            "average_latency": "1.2ms"
          }
        }
      }
    }
  }
}
```

**Health Status Values:**
- `healthy`: All systems operational
- `degraded`: Some non-critical issues detected
- `unhealthy`: Critical systems failing

**Example:**
```bash
curl http://localhost:8080/health
```

## Repository Management

### POST /repositories

Submit a Git repository for asynchronous indexing.

**Request Body:**
```json
{
  "url": "https://github.com/golang/go",
  "name": "golang/go",
  "description": "The Go programming language",
  "default_branch": "master"
}
```

**Request Validation:**
- `url`: Required, must match pattern `^https?://(github\.com|gitlab\.com|bitbucket\.org)/.+$`
- `name`: Optional, auto-generated from URL if not provided
- `description`: Optional
- `default_branch`: Optional, defaults to "main"

**Response Codes:**
- `202`: Repository accepted for indexing
- `400`: Invalid request (validation errors)
- `409`: Repository already exists (duplicate detection)
- `500`: Internal server error

**Success Response (202):**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "url": "https://github.com/golang/go",
  "normalized_url": "https://github.com/golang/go",
  "name": "golang/go",
  "description": "The Go programming language",
  "default_branch": "master",
  "status": "pending",
  "created_at": "2025-01-08T10:30:00Z",
  "updated_at": "2025-01-08T10:30:00Z"
}
```

**Duplicate Response (409):**
```json
{
  "error": "REPOSITORY_ALREADY_EXISTS",
  "message": "Repository already exists",
  "timestamp": "2025-01-08T10:30:00Z",
  "details": {
    "normalized_url": "https://github.com/golang/go",
    "existing_id": "123e4567-e89b-12d3-a456-426614174000"
  }
}
```

### GET /repositories

List repositories with optional filtering and pagination.

**Query Parameters:**
- `status`: Filter by status (`pending`, `cloning`, `processing`, `completed`, `failed`, `archived`)
- `limit`: Items per page (1-100, default: 20)
- `offset`: Skip items (default: 0)
- `sort`: Sort order (`created_at:asc`, `created_at:desc`, `updated_at:asc`, `updated_at:desc`, `name:asc`, `name:desc`)

**Response:**
```json
{
  "repositories": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "url": "https://github.com/golang/go",
      "name": "golang/go",
      "description": "The Go programming language",
      "default_branch": "master",
      "last_indexed_at": "2025-01-08T11:00:00Z",
      "last_commit_hash": "abc123def456789",
      "total_files": 1250,
      "total_chunks": 5000,
      "status": "completed",
      "created_at": "2025-01-08T10:30:00Z",
      "updated_at": "2025-01-08T11:00:00Z"
    }
  ],
  "pagination": {
    "limit": 20,
    "offset": 0,
    "total": 1,
    "has_more": false
  }
}
```

### GET /repositories/{id}

Get detailed information about a specific repository.

**Path Parameters:**
- `id`: Repository UUID

**Response Codes:**
- `200`: Repository found
- `404`: Repository not found
- `400`: Invalid UUID format

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "url": "https://github.com/golang/go",
  "normalized_url": "https://github.com/golang/go",
  "name": "golang/go",
  "description": "The Go programming language",
  "default_branch": "master",
  "last_indexed_at": "2025-01-08T11:00:00Z",
  "last_commit_hash": "abc123def456789",
  "total_files": 1250,
  "total_chunks": 5000,
  "status": "completed",
  "created_at": "2025-01-08T10:30:00Z",
  "updated_at": "2025-01-08T11:00:00Z",
  "indexing_statistics": {
    "processing_duration": "25m0s",
    "files_processed": 1250,
    "files_skipped": 50,
    "chunks_created": 5000,
    "embeddings_generated": 5000
  }
}
```

### DELETE /repositories/{id}

Delete a repository and all associated data.

**Path Parameters:**
- `id`: Repository UUID

**Response Codes:**
- `204`: Repository deleted successfully
- `404`: Repository not found
- `409`: Cannot delete while processing
- `400`: Invalid UUID format

**Conflict Response (409):**
```json
{
  "error": "REPOSITORY_PROCESSING",
  "message": "Cannot delete repository while indexing is in progress",
  "timestamp": "2025-01-08T10:30:00Z",
  "details": {
    "status": "processing",
    "job_id": "789e0123-e89b-12d3-a456-426614174000"
  }
}
```

## Indexing Job Management

### GET /repositories/{id}/jobs

List indexing jobs for a specific repository.

**Path Parameters:**
- `id`: Repository UUID

**Query Parameters:**
- `limit`: Items per page (1-100, default: 10)
- `offset`: Skip items (default: 0)

**Response:**
```json
{
  "jobs": [
    {
      "id": "789e0123-e89b-12d3-a456-426614174000",
      "repository_id": "123e4567-e89b-12d3-a456-426614174000",
      "status": "completed",
      "started_at": "2025-01-08T10:35:00Z",
      "completed_at": "2025-01-08T11:00:00Z",
      "files_processed": 1250,
      "chunks_created": 5000,
      "duration": "25m0s",
      "error_message": null
    }
  ],
  "pagination": {
    "limit": 10,
    "offset": 0,
    "total": 1,
    "has_more": false
  }
}
```

### GET /repositories/{id}/jobs/{job_id}

Get detailed information about a specific indexing job.

**Path Parameters:**
- `id`: Repository UUID
- `job_id`: Job UUID

**Response:**
```json
{
  "id": "789e0123-e89b-12d3-a456-426614174000",
  "repository_id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "completed",
  "started_at": "2025-01-08T10:35:00Z",
  "completed_at": "2025-01-08T11:00:00Z",
  "files_processed": 1250,
  "files_skipped": 50,
  "chunks_created": 5000,
  "duration": "25m0s",
  "error_message": null,
  "progress": {
    "current_step": "completed",
    "total_steps": 4,
    "step_details": {
      "cloning": "completed",
      "parsing": "completed",
      "chunking": "completed",
      "embedding": "completed"
    }
  }
}
```

## Status Values

### Repository Status Lifecycle
- `pending`: Queued for processing
- `cloning`: Repository is being cloned
- `processing`: Files are being analyzed and chunked
- `completed`: Indexing completed successfully
- `failed`: Indexing failed with errors
- `archived`: Repository has been archived

### Job Status Values
- `pending`: Queued for processing
- `running`: Currently being processed
- `completed`: Successfully completed
- `failed`: Failed with errors
- `cancelled`: Cancelled by user or system

## Error Handling

### Standard Error Response Format
```json
{
  "error": "ERROR_CODE",
  "message": "Human-readable error description",
  "timestamp": "2025-01-08T10:30:00Z",
  "details": {
    "field": "field_name",
    "value": "invalid_value",
    "additional_context": "..."
  }
}
```

### Error Codes

**Client Errors (4xx):**
- `INVALID_REQUEST`: General validation errors (400)
- `INVALID_URL`: Invalid repository URL format (400)
- `INVALID_UUID`: Malformed UUID in path (400)
- `REPOSITORY_NOT_FOUND`: Repository not found (404)
- `JOB_NOT_FOUND`: Indexing job not found (404)
- `REPOSITORY_ALREADY_EXISTS`: Duplicate repository (409)
- `REPOSITORY_PROCESSING`: Cannot modify during processing (409)

**Server Errors (5xx):**
- `INTERNAL_ERROR`: Unexpected server error (500)
- `DATABASE_ERROR`: Database connectivity issues (500)
- `NATS_ERROR`: Message queue connectivity issues (500)
- `SERVICE_UNAVAILABLE`: Service temporarily unavailable (503)

### Validation Error Details

For validation errors, the `details` object contains specific field information:

```json
{
  "error": "INVALID_REQUEST",
  "message": "Validation failed",
  "timestamp": "2025-01-08T10:30:00Z",
  "details": {
    "validation_errors": [
      {
        "field": "url",
        "message": "URL must be from a supported Git provider",
        "value": "https://example.com/repo"
      },
      {
        "field": "name",
        "message": "Name must be between 1 and 255 characters",
        "value": ""
      }
    ]
  }
}
```

## Security Features

### Input Validation
All endpoints implement comprehensive input validation:
- **XSS Prevention**: HTML entities are escaped
- **SQL Injection Protection**: Parameterized queries only
- **URL Validation**: Strict Git provider whitelist
- **Unicode Attack Prevention**: Dangerous Unicode sequences blocked

### Rate Limiting
Currently not implemented but planned for future releases.

### CORS Support
Configurable cross-origin resource sharing for web applications.

### Security Headers
All responses include security headers:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`

## Request/Response Examples

### Complete Repository Submission Flow

1. **Submit Repository:**
```bash
curl -X POST http://localhost:8080/api/v1/repositories \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://github.com/golang/go",
    "description": "The Go programming language"
  }'
```
2. **Check Status:**
```bash
curl http://localhost:8080/api/v1/repositories/123e4567-e89b-12d3-a456-426614174000
```

3. **Monitor Jobs:**
```bash
curl http://localhost:8080/api/v1/repositories/123e4567-e89b-12d3-a456-426614174000/jobs
```

4. **Health Check:**
```bash
curl http://localhost:8080/health
```

## Future Endpoints

### POST /search (Planned)
Semantic search endpoint for finding relevant code chunks.

**Planned Request:**
```json
{
  "query": "implement authentication middleware",
  "limit": 10,
  "repository_ids": ["123e4567-e89b-12d3-a456-426614174000"],
  "file_types": [".go", ".js"],
  "similarity_threshold": 0.7
}
```

This endpoint is referenced in documentation but not yet implemented in the codebase.