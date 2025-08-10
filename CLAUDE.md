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

### Development Tools
- `make install-tools` - Install goreman, migrate CLI, cobra-cli
- `make psql` - Connect to development PostgreSQL database
- `make nats-stream-info` - Show NATS JetStream information

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
- `Repository`: Git repository with indexing status and metadata
- `IndexingJob`: Async job for processing repositories
- Value Objects: `RepositoryURL`, `RepositoryStatus`, `JobStatus`

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

## TDD Requirements
- **IMPORTANT** you MUST Use tdd, the @agent-red-phase-tester for writing failing tests, @agent-green-phase-implementer for getting tests passing, and finally @agent-tdd-refactor-specialist for the refactor phase of tdd, this will ensure a well written and clean program and code
- try keeping files to 500 lines and not above 1000 lines to help with reaadability and parsability
- When running go test ./... AlWAYS use a timeout like go test ./... -timeout 10s
- When commiting use conventional commits