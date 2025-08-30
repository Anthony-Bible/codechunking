# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Essential Commands
- `make dev` - Start development environment (PostgreSQL, NATS)
- `make dev-api` - Run API server in development mode
- `make dev-worker` - Run worker in development mode
- `make migrate-up` - Apply database migrations
- `make test` - Run unit tests
- `make lint` - Run linter (requires golangci-lint)
- `make fmt` - Format code and tidy modules
- `make build` - Build the binary

### Testing Commands
- `make test` - Unit tests only (short flag)
- `make test-integration` - Integration tests (starts Docker services)
- `make test-coverage` - Generate coverage report
- `make test-all` - Run all tests with coverage
- `go test ./internal/path/to/package -v` - Run tests for specific package
- `go test ./internal/path/to/package -run TestFunctionName -v` - Run specific test function
- `go test ./... -timeout 10s` - Run all tests with timeout (always use timeout)

### Development Tools
- `make install-tools` - Install goreman, migrate CLI, cobra-cli
- `make psql` - Connect to development PostgreSQL database
- `make nats-stream-info` - Show NATS JetStream information
- `make nats-consumer-info` - Show NATS consumer information
- `make logs-api` - Show API logs with JSON formatting
- `make logs-worker` - Show worker logs with JSON formatting
- `ast-grep` - Search for AST nodes in Go source code, favor this over regex for code parsing tasks

## Architecture Overview

This is a production-grade code chunking and semantic search system using hexagonal architecture:

### Core Components
- **Domain Layer** (`internal/domain/`): Business entities, value objects, domain services
- **Application Layer** (`internal/application/`): Command/query handlers, DTOs
- **Port Layer** (`internal/port/`): Interface definitions for inbound/outbound adapters
- **Adapter Layer** (`internal/adapter/`): Concrete implementations (API, DB, NATS, etc.)

### Key Technologies
- **Language**: Go 1.24
- **Database**: PostgreSQL with pgvector extension
- **Messaging**: NATS JetStream for async job processing
- **Embeddings**: Google Gemini API (text-embedding-004 model)
- **CLI**: Cobra framework with Viper configuration
- **Code Parsing**: Tree-sitter for semantic code chunking

### Domain Entities
- `Repository`: Git repository with indexing status, metadata, and file/chunk counts
- `IndexingJob`: Async job for processing repositories with retry and error handling
- `Alert`: Error monitoring and alerting system entities
- `ClassifiedError`: Error classification and aggregation for monitoring
- Value Objects: `RepositoryURL`, `RepositoryStatus`, `JobStatus`, `AlertType`, `ErrorSeverity`, `Language`, `CloneOptions`

### Configuration
- Configuration hierarchy: CLI flags > Environment variables > Config files > Defaults
- Environment variables use `CODECHUNK_` prefix
- Config files: `configs/config.yaml` (base), `config.dev.yaml`, `config.prod.yaml`

### API Structure
- REST API on port 8080 with OpenAPI 3.0 specification
- Main endpoints: `/health`, `/repositories` (POST), `/search` (POST)
- Health checks and metrics endpoints available

### Worker Processing
- Background workers process indexing jobs from NATS JetStream
- Configurable concurrency (default: 5 workers)
- Jobs include: git cloning, code parsing, chunking, embedding generation

## Logging Guidelines

### Structured Logging with slogger Package
The application uses a centralized `slogger` package that wraps the `ApplicationLogger` infrastructure and ensures compliance with Go's structured logging best practices.

**Location**: `internal/application/common/slogger/slogger.go`

### Usage Examples

#### Context-Aware Logging (Preferred)
```go
import "codechunking/internal/application/common/slogger"

// Simple logging with context
slogger.Info(ctx, "Repository created successfully", slogger.Fields{
    "repository_id": repoID,
    "url": repositoryURL,
})

// Error logging with structured fields
slogger.Error(ctx, "Failed to process repository", slogger.Fields{
    "repository_id": repoID,
    "error": err.Error(),
    "operation": "clone_repository",
})

// Using helper functions for cleaner syntax
slogger.Info(ctx, "Processing completed", 
    slogger.Fields2("duration", duration, "records_processed", count))

slogger.Debug(ctx, "Detailed processing info",
    slogger.Fields3("step", "validation", "items", len(items), "batch_size", batchSize))
```

#### No-Context Fallback Functions
For situations where context is not available (initialization, cleanup, etc.):
```go
// Use sparingly - only when context is truly unavailable
slogger.InfoNoCtx("Application starting", slogger.Fields{
    "version": version,
    "environment": env,
})
```

### Migration from Global slog
When migrating existing code from global `slog` usage:

**Before (violates sloglint):**
```go
slog.Info("Processing repository", "url", repositoryURL)
slog.Error("Database error", "error", err)
```

**After (compliant):**
```go
slogger.Info(ctx, "Processing repository", slogger.Fields{"url": repositoryURL})
slogger.Error(ctx, "Database error", slogger.Fields{"error": err})
```

### Key Benefits
- **sloglint Compliant**: No more linting violations for global slog usage  ~
- **Context Propagation**: Automatic correlation ID and request ID handling  
- **Structured Fields**: Consistent field formatting across the application
- **Thread-Safe**: Global singleton with proper initialization
- **Backward Compatible**: Maintains existing ApplicationLogger infrastructure

### Important Notes
- Always prefer context-aware functions (`Info`, `Error`, etc.) over no-context versions
- Use structured fields (`slogger.Fields{}`) instead of key-value pairs
- The slogger package automatically handles ApplicationLogger initialization
- All logging output maintains JSON format for production observability
- **IMPORTANT** You are free to add important information or pages to the wiki, this serves as a place for other users to view information related to the repo

## TDD Requirements
- **IMPORTANT** you MUST Use tdd, the @agent-red-phase-tester for writing failing tests, @agent-green-phase-implementer for getting tests passing, and finally @agent-tdd-refactor-specialist for the refactor phase of tdd, this will ensure a well written and clean program and code
- try keeping files to 500 lines and not above 1000 lines to help with reaadability and parsability
- When running go test ./... AlWAYS use a timeout like go test ./... -timeout 10s
- When commiting use conventional commits
- For metrics use OTEL