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
- `make dev-down` - Stop development environment
- `make dev-clean` - Stop and remove volumes (clean slate)
- `make migrate-create name=<name>` - Create new migration file
- `make build-docker` - Build Docker images
- `make vet` - Run go vet static analysis
- `make psql` - Connect to development PostgreSQL database
- `make nats-stream-info` - Show NATS JetStream information
- `make nats-consumer-info` - Show NATS consumer information
- `make logs-api` - Show API logs with JSON formatting
- `make logs-worker` - Show worker logs with JSON formatting
- `ast-grep` - Search for AST nodes in Go source code, favor this over regex for code parsing tasks

### Development Services
- **PostgreSQL**: `localhost:5432` (user: dev, password: dev, db: codechunking)
- **NATS**: `localhost:4222` (client), `localhost:8222` (HTTP monitoring)
- **pgAdmin**: `localhost:5050` (email: admin@example.com, password: admin) - requires `docker-compose --profile tools up`

## Architecture Overview

This is a production-grade code chunking and semantic search system using hexagonal architecture:

### Core Components
- **Domain Layer** (`internal/domain/`): Business entities, value objects, domain services
- **Application Layer** (`internal/application/`): Command/query handlers, DTOs
- **Port Layer** (`internal/port/`): Interface definitions for inbound/outbound adapters
- **Adapter Layer** (`internal/adapter/`): Concrete implementations (API, DB, NATS, etc.)

### Key Technologies
- **Language**: Go 1.24 (requires CGO_ENABLED=1 for tree-sitter)
- **Database**: PostgreSQL with pgvector extension
- **Messaging**: NATS JetStream for async job processing
- **Embeddings**: Google Gemini API (gemini-embedding-001 model)
- **CLI**: Cobra framework with Viper configuration
- **Code Parsing**: Tree-sitter for semantic code chunking (go-sitter-forest with 300+ language parsers)

### Tree-sitter Integration
- **Primary Parser Location**: Uses `go-sitter-forest` for comprehensive language support
- **Grammar Development**: Additional working directory at `/tmp/tree-parser-grammar/` for custom grammars
- **Language Support**: Includes parsers for Go, Python, JavaScript, TypeScript, and 300+ other languages
- **Parser Usage**: Custom parsers in `internal/adapter/outbound/treesitter/parsers/` for language-specific chunking
- **Build Requirement**: Must use `CGO_ENABLED=1` for all builds due to tree-sitter C bindings

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

### Logging Best Practices
- Always prefer context-aware functions (`Info`, `Error`, etc.) over no-context versions
- Use structured fields (`slogger.Fields{}`) instead of key-value pairs
- The slogger package automatically handles ApplicationLogger initialization
- All logging output maintains JSON format for production observability

## TDD Requirements

### TDD Workflow
- **IMPORTANT**: MUST use Test-Driven Development with specialized agents:
  - `@agent-red-phase-tester` - Write failing tests first
  - `@agent-green-phase-implementer` - Make tests pass with minimal code
  - `@agent-tdd-refactor-specialist` - Refactor for quality and maintainability

### Testing Principles
- **Focus on Input/Output**: Design tests around input/output pairs rather than mocks
- **Mandatory Timeouts**: Always use timeout flags to prevent hanging tests
  - Example: `go test ./... -timeout 10s`
  - Example: `go test ./internal/path/to/package -run TestFunctionName -v -timeout 30s`
- **Isolation**: Unit tests should be fast and isolated; use short flag (`-short`) for unit tests
- **Coverage**: Maintain high test coverage (currently 100+ passing tests across all layers)

## Development Best Practices

### Code Organization
- Keep files under 500 lines (preferred), max 1000 lines for readability and parsability
- Follow hexagonal architecture patterns (domain, application, port, adapter layers)
- Use conventional commits for all commits (e.g., `feat:`, `fix:`, `refactor:`, `test:`)

### Observability
- **Metrics**: Use OpenTelemetry (OTEL) for all metrics
- **Logging**: Use structured logging with `slogger` package
- **Tracing**: Leverage correlation IDs for request tracking across components

### Documentation
- **Wiki**: Add important information to the wiki (git submodule at `./wiki/`)
  - Configuration guides, architecture decisions, troubleshooting tips
  - Wiki serves as primary documentation for developers and users
- Always usee tree sitter queries and syntax.