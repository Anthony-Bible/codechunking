# Development Guide

This guide covers setting up a development environment and contributing to the CodeChunking project.

## Quick Start

### 1. Prerequisites

Ensure you have these installed:
- **Go 1.24** or higher
- **Docker and Docker Compose**
- **Git**

### 2. Clone and Setup

```bash
git clone <your-repo-url>
cd codechunking

# Copy environment template
cp .env.example .env
# Edit .env and add your Gemini API key
```

### 3. Install Development Tools

```bash
# Install required tools (migrate, goreman, cobra-cli)
make install-tools

# Install golangci-lint manually (required for linting)
# See: https://golangci-lint.run/usage/install/
```

### 4. Start Development Environment

```bash
# Start Docker services (PostgreSQL, NATS)
make dev

# Apply database migrations
make migrate-up

# Start API server (in one terminal)
make dev-api

# Start worker (in another terminal)
make dev-worker
```

### 5. Verify Setup

```bash
# Check health endpoint
curl http://localhost:8080/health

# Should return healthy status with NATS and database info
```

## Development Workflow

### TDD Methodology

This project uses **Test-Driven Development** with specialized agents:

1. **Red Phase**: Use `@agent-red-phase-tester` to write failing tests first
2. **Green Phase**: Use `@agent-green-phase-implementer` to make tests pass
3. **Refactor Phase**: Use `@agent-tdd-refactor-specialist` to clean up code

**Important**: Always write tests before implementing features.

### Code Quality Standards

- **File Size**: Keep files under 1000 lines (preferably under 500)
- **Test Coverage**: Maintain above 80% (currently 68+ passing tests)
- **Test Timeouts**: Always use timeouts: `go test ./... -timeout 10s`
- **Commit Messages**: Use conventional commits (required)

### Running Tests

```bash
# Unit tests only
make test

# Integration tests (starts Docker services)
make test-integration

# All tests with coverage
make test-all

# Coverage report
make test-coverage
```

### Code Quality Checks

```bash
# Format code and tidy modules
make fmt

# Run linter (requires golangci-lint)
make lint

# Build project
make build
```

## Project Architecture

### Hexagonal Architecture

The codebase follows hexagonal (ports and adapters) architecture:

```
internal/
├── domain/              # Business logic (entities, value objects)
├── application/         # Use cases (commands, queries, handlers)
├── port/               # Interface definitions
│   ├── inbound/        # Driving ports (API interfaces)
│   └── outbound/       # Driven ports (repository, messaging)
└── adapter/            # Interface implementations
    ├── inbound/        # HTTP API, CLI
    └── outbound/       # Database, NATS, external APIs
```

### Key Components

**Domain Layer** (`internal/domain/`):
- `entity/` - Core business entities (Repository, IndexingJob)
- `valueobject/` - Value objects (RepositoryURL, JobStatus)
- `service/` - Domain services

**Application Layer** (`internal/application/`):
- `command/` - Command handlers
- `query/` - Query handlers  
- `dto/` - Data transfer objects
- `service/` - Application services

**Adapters** (`internal/adapter/`):
- `inbound/api/` - HTTP REST API
- `inbound/service/` - Service adapters
- `outbound/repository/` - Database implementations
- `outbound/messaging/` - NATS JetStream client

### Configuration

Configuration uses Viper with hierarchy (see `internal/config/config.go`):
1. CLI flags (highest priority)
2. Environment variables (`CODECHUNK_` prefix)
3. Config files (`configs/config.yaml`, `config.dev.yaml`)
4. Defaults (lowest priority)

## Development Commands

All commands are available through the Makefile:

### Environment Management
```bash
make dev          # Start PostgreSQL + NATS
make dev-api      # Run API server
make dev-worker   # Run background worker
make psql         # Connect to development database
make nats-stream-info # Show NATS JetStream info
```

### Testing
```bash
make test                # Unit tests (with timeout)
make test-integration    # Integration tests
make test-coverage       # Generate coverage report
make test-all           # All tests with coverage
```

### Code Quality
```bash
make fmt         # Format code + go mod tidy
make lint        # Run golangci-lint
make build       # Build binary to bin/
```

### Database
```bash
make migrate-up          # Apply all migrations
make migrate-down        # Rollback one migration
make migrate-create name=new_feature # Create new migration
```

## Working with the Codebase

### Adding New Features

1. **Write Tests First** (TDD red phase)
   ```bash
   # Create failing tests that define the expected behavior
   # Use @agent-red-phase-tester if working with Claude
   ```

2. **Implement Minimum Code** (TDD green phase)
   ```bash
   # Write just enough code to make tests pass
   # Use @agent-green-phase-implementer if working with Claude
   ```

3. **Refactor** (TDD refactor phase)
   ```bash
   # Clean up code while keeping tests green
   # Use @agent-tdd-refactor-specialist if working with Claude
   ```

4. **Run Quality Checks**
   ```bash
   make fmt && make lint && make test
   ```

### API Development

When working on the REST API (`internal/adapter/inbound/api/`):

- Follow the existing handler patterns
- Add comprehensive tests (see `*_test.go` files for examples)
- Update OpenAPI spec in `/api/openapi.yaml` if needed
- Test health endpoints and error handling

### Database Changes

1. **Create Migration**
   ```bash
   make migrate-create name=add_new_feature
   ```

2. **Edit Migration Files** in `/migrations/`
   - Write both `up` and `down` migrations
   - Test migrations thoroughly

3. **Apply Migration**
   ```bash
   make migrate-up
   ```

### NATS/Messaging

When working with messaging (`internal/adapter/outbound/messaging/`):

- Use the existing NATS client patterns
- Add proper error handling and circuit breakers
- Test connection resilience
- Monitor performance (aim for 305K+ msg/sec throughput)

### Security Features

The codebase includes comprehensive security (see `internal/application/common/security/`):

- **Input Validation**: All user inputs are validated and sanitized
- **XSS Prevention**: HTML entities are escaped
- **SQL Injection Protection**: Only parameterized queries
- **URL Validation**: Strict Git provider whitelist

When adding new endpoints or inputs:
1. Add validation rules
2. Test with fuzzing (see `*_fuzz_test.go` examples)
3. Check the security middleware integration

### Logging and Monitoring

The system uses structured logging (`internal/application/common/logging/`):

- **Correlation IDs**: Track requests across components
- **JSON Format**: Structured for log aggregation
- **Performance Metrics**: Built-in timing and metrics
- **Error Context**: Rich error information

Example logging:
```go
logger := logging.GetLogger(ctx)
logger.Info("Processing repository", 
    "repository_id", repoID,
    "operation", "indexing")
```

## Testing Guidelines

### Test Structure

Follow the existing test patterns:
- Unit tests: `*_test.go` files alongside source
- Integration tests: `*_integration_test.go` 
- Fuzz tests: `*_fuzz_test.go` for input validation

### Test Categories

**Unit Tests**: Fast tests with mocked dependencies
```bash
make test  # Runs with -short flag
```

**Integration Tests**: Real dependencies (Docker)
```bash
make test-integration  # Starts Docker services
```

**Fuzz Tests**: Security validation
```bash
go test -fuzz=FuzzValidateURL ./internal/domain/valueobject/
```

### Mock Usage

Use the mocks in `internal/adapter/outbound/mock/` for testing:
- Database mocks for repository testing
- NATS mocks for messaging testing
- Clean separation between unit and integration tests

## Common Development Tasks

### Adding a New REST Endpoint

1. **Define the handler** in `internal/adapter/inbound/api/`
2. **Add route** in `internal/adapter/inbound/api/routes.go`
3. **Create DTOs** in `internal/application/dto/`
4. **Write tests** with both success and error cases
5. **Update OpenAPI spec** in `/api/openapi.yaml`

### Adding a New Repository Method

1. **Define interface** in `internal/port/outbound/`
2. **Implement method** in `internal/adapter/outbound/repository/`
3. **Add tests** with database integration
4. **Update mock** in `internal/adapter/outbound/mock/`

### Adding Configuration Options

1. **Add to config struct** in `internal/config/config.go`
2. **Update YAML files** in `/configs/`
3. **Add to `.env.example`** with documentation
4. **Test configuration loading**

## Troubleshooting Development Issues

### Common Setup Problems

**Missing golangci-lint**:
```bash
# Install manually from https://golangci-lint.run/usage/install/
# Or use the install script
```

**Docker Services Not Starting**:
```bash
# Check Docker is running
docker ps

# Restart services
make dev
```

**Migration Failures**:
```bash
# Check database connection
make psql

# Check migration status
SELECT * FROM schema_migrations;
```

**NATS Connection Issues**:
```bash
# Check NATS is running and responsive
curl http://localhost:8222

# Check JetStream status
make nats-stream-info
```

### Performance Issues

**Slow Tests**:
- Always use timeouts: `go test ./... -timeout 10s`
- Use `-short` flag for unit tests only
- Check Docker resource allocation

**Health Check Performance**:
- Health endpoint should respond in ~23.5µs
- Check caching is working (5-second TTL)
- Monitor database connection pool

## Contributing Guidelines

### Before Submitting PRs

1. **Run Full Test Suite**
   ```bash
   make test-all
   ```

2. **Check Code Quality**
   ```bash
   make fmt && make lint
   ```

3. **Update Documentation**
   - Update relevant docs if adding features
   - Keep CLAUDE.md updated for development guidance

4. **Use Conventional Commits**
   ```bash
   git commit -m "feat: add repository search endpoint"
   ```

### Code Review Checklist

- [ ] Tests pass and have good coverage
- [ ] Code follows hexagonal architecture patterns
- [ ] Error handling is comprehensive
- [ ] Security validation is in place
- [ ] Logging includes proper context
- [ ] Documentation is updated

## Development Environment Details

### Directory Structure

See the main README.md for the complete directory structure. Key development directories:

- `/configs/` - All configuration files
- `/docker/` - Docker setup and init scripts
- `/migrations/` - Database schema changes
- `/api/` - OpenAPI specification
- `/internal/` - All Go source code
- `/scripts/` - Utility scripts

### Environment Variables

All development environment variables are documented in `.env.example`. Key variables:

- `CODECHUNK_GEMINI_API_KEY` - Required for embeddings
- `CODECHUNK_LOG_LEVEL` - Set to "debug" for development
- `CODECHUNK_DATABASE_*` - Database connection settings
- `CODECHUNK_NATS_*` - NATS connection settings

### Performance Expectations

Based on current test results:
- Health checks: 23.5µs average response time
- NATS throughput: 305,358+ messages/second
- Database queries: Sub-millisecond for indexed lookups

Monitor these metrics during development to ensure no performance regressions.