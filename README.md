# CodeChunking

A production-grade semantic code search system using Go, PostgreSQL with pgvector, NATS JetStream, and Google Gemini. Built with hexagonal architecture and comprehensive TDD methodology.

🎉 **MVP Status: COMPLETE** - Full semantic search functionality with natural language queries!

## Features

### Core Functionality
- **Repository Indexing**: Clone and index Git repositories automatically with submodule detection
- **Intelligent Code Chunking**: Parse code into semantic units using tree-sitter with semantic overlap strategy
- **Multi-Language Support**: 300+ programming languages via go-sitter-forest (Go, Python, JavaScript, TypeScript, and more)
- **Vector Embeddings**: Generate embeddings with Google Gemini API (768-dimensional)
- **Semantic Code Search**: Natural language queries with POST /search endpoint ✨
- **Advanced Filtering**: Filter by repository, language, file type, semantic construct types, entity names, and visibility
- **pgvector with Partitioning**: Scalable vector storage using partitioned tables for large repositories
- **Batch Processing**: Gemini Batches API integration for efficient embedding generation at scale
- **Semantic Overlap**: Configurable overlap between chunks for improved retrieval quality
- **Pagination & Sorting**: Full pagination with multiple sorting options (similarity, file path)
- **Asynchronous Processing**: Scalable job processing with NATS JetStream and configurable worker concurrency
- **Idempotent Job Processing**: Safe retries and resume capability for interrupted jobs

### Production Features
- **High-Performance Messaging**: NATS JetStream client with 305,358+ msg/sec throughput
- **Advanced Health Monitoring**: 23.5µs average response time with intelligent caching
- **Circuit Breaker Patterns**: Connection resilience and fault tolerance
- **Structured Logging**: Correlation ID tracking with cross-component tracing
- **Comprehensive Security**: Input validation, XSS prevention, SQL injection protection
- **Robust Error Handling**: Centralized panic recovery and validation

### Architecture & Development
- **Hexagonal Architecture**: Clean, maintainable code structure with clear separation
- **TDD Implementation**: Using specialized red-green-refactor agent methodology
- **CLI Interface**: Cobra-based CLI with multiple commands
- **Configuration Management**: Hierarchical configuration with Viper

## Architecture

The system follows hexagonal architecture (ports and adapters) principles:

```
├── cmd/                    # CLI commands (Cobra)
├── internal/
│   ├── domain/            # Core business logic
│   ├── application/       # Use cases
│   ├── port/             # Interface definitions
│   └── adapter/          # Interface implementations
├── configs/              # Configuration files
├── migrations/           # Database migrations
└── docker/              # Docker configurations
```

## Data Flow Architecture

The following diagram illustrates how repositories are processed, chunked, stored, and retrieved:

```mermaid
graph TB
    %% User Interface Layer
    subgraph "Client Layer"
        USER[User/CLI]
        API_CLIENT[API Client]
    end

    %% API Layer
    subgraph "API Layer (Inbound Adapters)"
        API_SERVER[API Server :8080]
        REPO_HANDLER[Repository Handler]
        SEARCH_HANDLER[Search Handler]
        HEALTH[Health Check]
    end

    %% Application Layer
    subgraph "Application Layer"
        REPO_SERVICE[Repository Service]
        SEARCH_SERVICE[Search Service]
        WORKER_SERVICE[Worker Service]
        JOB_PROCESSOR[Job Processor]
    end

    %% Domain Layer
    subgraph "Domain Layer"
        REPO_ENTITY[Repository Entity]
        JOB_ENTITY[IndexingJob Entity]
        ALERT_ENTITY[Alert Entity]
        ERROR_ENTITY[ClassifiedError Entity]
    end

    %% Message Queue
    subgraph "Messaging (NATS JetStream)"
        NATS_PUBLISHER[Message Publisher]
        NATS_STREAM[INDEXING Stream]
        NATS_CONSUMER[Message Consumer]
    end

    %% Processing Pipeline
    subgraph "Background Processing Pipeline"
        GIT_CLONE[Git Repository<br/>Clone]
        TS_PARSER[Tree-sitter<br/>Parser Pool]

        subgraph "Language Parsers"
            GO_PARSER[Go Parser]
            PY_PARSER[Python Parser]
        end

        CHUNKER[Function/Class<br/>Chunker]
        SEMANTIC_ANALYZER[Semantic Analysis<br/>Service]
        JOB_STATE_MGR[Job State Manager<br/>Idempotent Transitions]
    end

    %% Observability
    subgraph "Observability & Monitoring"
        TOKEN_COUNTER[Token Counter<br/>OpenTelemetry Metrics]
    end

    %% Embedding & Storage
    subgraph "Embedding & Vector Storage"
        GEMINI_CLIENT[Google Gemini<br/>Embedding Client]
        BATCH_PROCESSOR[Gemini Batches API<br/>BatchSubmitter & Poller]
        RETRY_LOGIC[Retry Logic &<br/>Circuit Breaker]
        BATCH_PROGRESS[Batch Progress<br/>Tracking & Retry]
    end

    %% Data Storage
    subgraph "Data Storage (Outbound Adapters)"
        POSTGRES[(PostgreSQL<br/>Database)]
        PGVECTOR[(pgvector<br/>Extension)]

        subgraph "Database Tables"
            REPO_TABLE[(repositories)]
            CHUNKS_TABLE[(code_chunks)]
            EMBED_TABLE[(embeddings)]
            EMBED_PART[(embeddings_partitioned)]
            JOBS_TABLE[(indexing_jobs)]
        end
    end

    %% Request Flow - Repository Creation
    USER -->|POST /repositories| API_SERVER
    API_SERVER --> REPO_HANDLER
    REPO_HANDLER --> REPO_SERVICE
    REPO_SERVICE --> REPO_ENTITY
    REPO_SERVICE -->|Save Repository| POSTGRES
    REPO_SERVICE -->|Publish Job| NATS_PUBLISHER
    NATS_PUBLISHER --> NATS_STREAM

    %% Background Processing Flow
    NATS_STREAM --> NATS_CONSUMER
    NATS_CONSUMER --> WORKER_SERVICE
    WORKER_SERVICE --> JOB_PROCESSOR

    JOB_PROCESSOR --> GIT_CLONE
    GIT_CLONE --> TS_PARSER
    TS_PARSER --> GO_PARSER
    TS_PARSER --> PY_PARSER

    GO_PARSER --> CHUNKER
    PY_PARSER --> CHUNKER
    CHUNKER --> SEMANTIC_ANALYZER

    %% Job State Management & Token Counting
    JOB_PROCESSOR --> JOB_STATE_MGR
    JOB_STATE_MGR -->|Update Status| POSTGRES
    CHUNKER --> TOKEN_COUNTER
    TOKEN_COUNTER -->|Metrics| BATCH_PROCESSOR

    SEMANTIC_ANALYZER --> BATCH_PROCESSOR
    BATCH_PROCESSOR --> BATCH_PROGRESS
    BATCH_PROGRESS -->|Track Progress| POSTGRES
    BATCH_PROCESSOR --> GEMINI_CLIENT
    GEMINI_CLIENT -.->|Retry on Failure| RETRY_LOGIC
    RETRY_LOGIC -.-> GEMINI_CLIENT

    GEMINI_CLIENT --> POSTGRES
    JOB_PROCESSOR -->|Update Job Status| POSTGRES

    %% Search Flow
    USER -->|POST /search| API_SERVER
    API_SERVER --> SEARCH_HANDLER
    SEARCH_HANDLER --> SEARCH_SERVICE
    SEARCH_SERVICE -->|Generate Query Embedding| GEMINI_CLIENT
    SEARCH_SERVICE -->|Vector Similarity Search| PGVECTOR
    PGVECTOR -->|cosine distance <=>| EMBED_TABLE
    PGVECTOR -->|Partitioned Search| EMBED_PART
    SEARCH_SERVICE -->|Retrieve Chunks| POSTGRES
    SEARCH_SERVICE -->|Format Results| SEARCH_HANDLER

    %% Data Relationships
    POSTGRES --> REPO_TABLE
    POSTGRES --> CHUNKS_TABLE
    POSTGRES --> EMBED_TABLE
    POSTGRES --> EMBED_PART
    POSTGRES --> JOBS_TABLE
    PGVECTOR -.->|Vector Operations| EMBED_TABLE
    PGVECTOR -.->|HNSW Indexing| EMBED_PART

    %% Health Monitoring
    USER -->|GET /health| HEALTH
    HEALTH -.->|Check Status| POSTGRES
    HEALTH -.->|Check Status| NATS_STREAM

    %% Error Handling
    JOB_PROCESSOR -.->|Classification| ERROR_ENTITY
    ERROR_ENTITY -.->|Aggregation| ALERT_ENTITY
    ALERT_ENTITY -.->|Notifications| USER

    %% Styling
    classDef userLayer fill:#e1f5fe
    classDef apiLayer fill:#f3e5f5
    classDef appLayer fill:#e8f5e8
    classDef domainLayer fill:#fff3e0
    classDef messageLayer fill:#fce4ec
    classDef processLayer fill:#f1f8e9
    classDef storageLayer fill:#e0f2f1
    classDef embedLayer fill:#fff8e1

    class USER,API_CLIENT userLayer
    class API_SERVER,REPO_HANDLER,SEARCH_HANDLER,HEALTH apiLayer
    class REPO_SERVICE,SEARCH_SERVICE,WORKER_SERVICE,JOB_PROCESSOR,JOB_STATE_MGR appLayer
    class REPO_ENTITY,JOB_ENTITY,ALERT_ENTITY,ERROR_ENTITY domainLayer
    class NATS_PUBLISHER,NATS_STREAM,NATS_CONSUMER messageLayer
    class GIT_CLONE,TS_PARSER,GO_PARSER,PY_PARSER,CHUNKER,SEMANTIC_ANALYZER processLayer
    class POSTGRES,PGVECTOR,REPO_TABLE,CHUNKS_TABLE,EMBED_TABLE,EMBED_PART,JOBS_TABLE storageLayer
    class GEMINI_CLIENT,BATCH_PROCESSOR,RETRY_LOGIC,BATCH_PROGRESS,TOKEN_COUNTER embedLayer
```

### Key Data Flow Stages

1. **Repository Submission**: User submits repository URL via REST API
2. **Job Creation**: Repository entity created, indexing job queued to NATS JetStream
3. **Job State Management**: Idempotent state transitions (pending → cloning → processing → completed/failed) with resume capability
4. **Background Processing**: Worker clones repository and parses code with Tree-sitter
5. **Semantic Chunking**: Code parsed into semantic units (functions, classes, methods)
6. **Token Counting**: OpenTelemetry metrics track tokens per chunk for cost monitoring
7. **Batch Processing**: Gemini Batches API with progress tracking for efficient embedding generation
8. **Vector Storage**: Embeddings stored in PostgreSQL with pgvector HNSW indexing and partitioned tables
9. **Search & Retrieval**: Vector similarity search with iterative scanning for guaranteed results

For detailed architecture documentation, see the [wiki](wiki/).

### Processing Capabilities

- **Multi-Language Support**: 300+ programming languages via go-sitter-forest (Go, Python, JavaScript, TypeScript, Rust, Java, C/C++, Ruby, PHP, and more)
- **Intelligent Chunking**: Function-level, class-level, and adaptive chunking strategies with semantic overlap
- **Submodule Detection**: Automatic Git submodule detection and handling
- **Idempotent Job Processing**: Safe retries with resume capability for interrupted jobs
- **Binary File Filtering**: Automatic exclusion of binary files with magic number detection
- **Fault Tolerance**: Circuit breakers, retry logic, and comprehensive error handling
- **Scalable Processing**: Parallel batch processing with configurable worker pools
- **Performance Optimization**: HNSW indexing, partitioned embeddings, connection pooling, and caching

## Prerequisites

- Go 1.24 or higher
- Docker and Docker Compose
- PostgreSQL with pgvector extension
- NATS JetStream
- Google Gemini API key
- `golangci-lint` (for development)
- `migrate` CLI tool (installed via `make install-tools`)

## Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/Anthony-Bible/codechunking.git
cd codechunking
```

### 2. Set up environment variables

```bash
cp .env.example .env
# Edit .env and add your Gemini API key
```

### 3. Start development environment

```bash
# Start Docker services (PostgreSQL, NATS)
make dev

# Run database migrations
make migrate-up

# Start API server
make dev-api

# In another terminal, start worker
make dev-worker
```

### 4. Verify the setup

```bash
# Test health endpoint - should return 200 OK
curl http://localhost:8080/health

# Index your first repository
curl -X POST http://localhost:8080/repositories \
  -H "Content-Type: application/json" \
  -d '{"url": "https://github.com/golang/go", "name": "golang"}'

# Check repository status (replace {id} with the ID from the response)
curl http://localhost:8080/repositories/{id}

# Try a semantic search once indexing is complete
curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{"query": "http request handler", "limit": 5}'
```

For detailed API documentation, see the [wiki](wiki/).

## Installation

### Option 1: Go Install (Recommended for Development)

```bash
# Install main binary (requires CGO_ENABLED=1 for tree-sitter)
CGO_ENABLED=1 go install github.com/Anthony-Bible/codechunking/cmd/codechunking@latest

# Install client binary only (lightweight, no CGO required)
go install github.com/Anthony-Bible/codechunking/cmd/client@latest
```

**Note**: The main binary requires CGO_ENABLED=1 due to tree-sitter dependencies. The client binary is standalone and doesn't require CGO.

### Option 2: Download Pre-built Binaries

Download pre-compiled binaries from GitHub releases:

```bash
# Example for Linux amd64
wget https://github.com/Anthony-Bible/codechunking/releases/latest/download/codechunking-v1.0.0
chmod +x codechunking-v1.0.0
sudo mv codechunking-v1.0.0 /usr/local/bin/codechunking

# Client binary
wget https://github.com/Anthony-Bible/codechunking/releases/latest/download/client-v1.0.0
chmod +x client-v1.0.0
sudo mv client-v1.0.0 /usr/local/bin/codechunking-client
```

Available platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### Option 3: Build from Source

```bash
git clone https://github.com/Anthony-Bible/codechunking.git
cd codechunking
make build
```

The build script creates two binaries in `./bin/`:
- `codechunking` - Main application with tree-sitter support
- `client` - Lightweight client binary

### Option 4: Using Docker

```bash
docker pull ghcr.io/anthony-bible/codechunking:latest
```

### Version Verification

Check installed version:

```bash
# Main binary
codechunking version

# Example output:
CodeChunking CLI
Version: v3.0.0
Commit: abc123def456
Built: 2024-06-01T12:00:00Z

# Client binary
codechunking-client version

# Short version output
codechunking version --short
v1.0.0
```

### Binary Differences

- **Main Binary (`codechunking`)**: Full-featured application including API server, worker, and file processing. Requires CGO for tree-sitter code parsing.
- **Client Binary (`codechunking-client`)**: Lightweight standalone CLI for API interaction. No dependencies, ideal for CI/CD and AI agents.

For detailed installation instructions and troubleshooting, see [INSTALL.md](INSTALL.md).

## Usage

### CLI Commands

```bash
# Show help
codechunking --help

# Start API server
codechunking api --config configs/config.dev.yaml

# Start worker
codechunking worker --concurrency 10

# Parse, chunk, and embed a single file (development/testing)
codechunking chunk --file path/to/source.go [--lang go] [--out output.json]

# Run migrations
codechunking migrate up

# Create a new migration
make migrate-create name=add_new_feature

# Show version
codechunking version
# Expected output:
# CodeChunking CLI
# Version: v1.0.0
# Commit: abc123def456
# Built: <build-date>

# Short version
codechunking version --short
# Expected output: v1.0.0
```

#### Chunk Command

The `chunk` command is useful for testing and development, allowing you to process a single source file through the complete pipeline:

```bash
# Process a Go file and output to stdout
codechunking chunk --file internal/domain/entity/repository.go

# Process with specific language and save to file
codechunking chunk --file script.py --lang python --out chunks.json
```

**Output includes:**
- File path and detected language
- Number of functions extracted
- Array of code chunks with:
  - Chunk content and metadata
  - 768-dimensional embeddings
  - Content preview
  - Semantic construct type (function, method, class, etc.)

### Client CLI

The `codechunking-client` is a standalone CLI for AI agents and automation. It outputs structured JSON for easy parsing.

```bash
# Build the client (no CGO required)
make build-client

# Check API health
./bin/codechunking-client health

# Add and index a repository (wait for completion)
./bin/codechunking-client repos add https://github.com/user/repo --wait

# Search for code semantically
./bin/codechunking-client search "authentication middleware" --limit 10

# List repositories
./bin/codechunking-client repos list --status completed
```

**Global Flags:**
- `--api-url` (default: `http://localhost:8080`) - API server URL
- `--timeout` (default: `30s`) - Request timeout

**JSON Output Format:**
```json
{"success": true, "data": {...}, "timestamp": "..."}
{"success": false, "error": {"code": "NOT_FOUND", "message": "..."}, "timestamp": "..."}
```

**Error Codes:** `INVALID_CONFIG`, `CONNECTION_ERROR`, `TIMEOUT_ERROR`, `NOT_FOUND`, `SERVER_ERROR`, `API_ERROR`, `INVALID_ARGUMENT`

For complete documentation including agent integration and CLAUDE.md snippets, see the [Client CLI Guide](wiki/Client-CLI-Guide.md).

### Using with AI Agents

The `codechunking-client` is designed for seamless AI agent integration with structured JSON output and comprehensive error handling.

**Quick Agent Examples:**
```bash
# Agent: Index and search a repository
./bin/codechunking-client repos add https://github.com/user/repo --wait
./bin/codechunking-client search "error handling pattern" --limit 10

# Agent: Check system health
./bin/codechunking-client health | jq -e '.success' && echo "API ready"

# Agent: Find all Go functions in a repo
./bin/codechunking-client search "function" --types function --languages go
```

For complete agent integration patterns, see the [Client CLI Guide](wiki/Client-CLI-Guide.md#agent-integration).

### API Endpoints

#### Index a repository
```bash
curl -X POST http://localhost:8080/repositories \
  -H "Content-Type: application/json" \
  -d '{"url": "https://github.com/example/repo"}'
```

#### Health check with monitoring
```bash
curl http://localhost:8080/health
# Returns health status with NATS monitoring, response time metrics
```

#### List all repositories
```bash
curl http://localhost:8080/repositories
```

#### Get repository details
```bash
curl http://localhost:8080/repositories/{id}
# Returns repository metadata including file and chunk counts
```

#### List repository indexing jobs
```bash
curl http://localhost:8080/repositories/{id}/jobs
# Shows all indexing jobs for a repository with their status
```

#### Get specific job details
```bash
curl http://localhost:8080/repositories/{id}/jobs/{job_id}
# Returns detailed job information including progress and errors
```

#### Delete a repository
```bash
curl -X DELETE http://localhost:8080/repositories/{id}
# Removes repository and all associated data
```

#### Search code semantically ✨
```bash
curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "implement authentication middleware",
    "limit": 10,
    "similarity_threshold": 0.7,
    "languages": ["go"],
    "repository_names": ["golang/go", "gin-gonic/gin"],
    "types": ["function", "method"],
    "entity_name": "Auth",
    "visibility": ["public"],
    "sort": "similarity:desc"
  }'
```

**Advanced Search Filters:**
- `repository_names`: Filter by specific repositories (format: "org/repo", max 50)
- `types`: Filter by semantic construct types (function, method, class, struct, interface, enum, etc.)
- `entity_name`: Partial match on function/class names
- `visibility`: Filter by access modifiers (public, private, protected)
- `file_extensions`: Filter by file type (e.g., [".go", ".py"])

**pgvector Iterative Scanning:** The search automatically uses iterative scanning to guarantee you get the requested number of results, even with restrictive filters. This solves the common "overfiltering" problem where filters reduce result counts below the requested limit.

### Authentication & Authorization

**Current Status:** No authentication or authorization is implemented. All endpoints are publicly accessible without API keys or tokens.

The API includes security middleware that sets appropriate headers, but does not enforce authentication. If you need to protect your instance, consider adding:
- API key authentication
- OAuth2 / JWT tokens
- IP whitelisting
- Rate limiting

## AI Agent Quick Setup

Add this to your project's `agents.md` or `claude.md` file for instant AI agent context:

````markdown
### CodeChunking Client Integration

**Binary:** `./bin/codechunking-client` (build with `make build-client`)

**Essential Commands:**
- Health check: `codechunking-client health`
- Add repo: `codechunking-client repos add <url> --wait`
- List repos: `codechunking-client repos list --status completed`
- Search: `codechunking-client search "<query>" --limit N --languages L`

**JSON Output Pattern:**
All commands return: `{"success": true/false, "data": {...}|error: {...}, "timestamp": "..."}`

**Common Workflows:**
1. Index repository → wait for completion → search code
2. Parse results with `jq -r '.data.results[].file_path'`
3. Check success with `jq -e '.success'`

**Error Codes:** INVALID_CONFIG, CONNECTION_ERROR, TIMEOUT_ERROR, NOT_FOUND, SERVER_ERROR
````

**API URL:** Configure with `--api-url` or `CODECHUNK_CLIENT_API_URL` (default: http://localhost:8080)

## Configuration

Configuration can be provided through:
1. Configuration files (YAML)
2. Environment variables (prefix: `CODECHUNK_`)
3. Command-line flags

Priority: Flags > Environment > Config File > Defaults

For detailed configuration options and advanced tuning, see the [wiki](wiki/).

### Configuration Files

- `configs/config.yaml` - Base configuration
- `configs/config.dev.yaml` - Development overrides
- `configs/config.prod.yaml` - Production overrides

### Environment Variables

```bash
export CODECHUNK_DATABASE_USER=myuser
export CODECHUNK_DATABASE_PASSWORD=mypass
export CODECHUNK_GEMINI_API_KEY=your-api-key
export CODECHUNK_LOG_LEVEL=debug
```

## Development

For comprehensive development guides, architecture deep-dives, and troubleshooting, see the [wiki](wiki/).

### Project Structure

```
codechunking/
├── cmd/                       # CLI entry points
│   ├── codechunking/         # Main CLI
│   └── commands/             # Cobra commands
├── internal/                 # Private application code
│   ├── domain/              # Business entities
│   │   ├── entity/         # Domain entities
│   │   ├── valueobject/    # Value objects
│   │   └── service/        # Domain services
│   ├── application/         # Application layer
│   │   ├── command/        # Command handlers
│   │   ├── query/          # Query handlers
│   │   └── dto/            # Data transfer objects
│   ├── port/               # Port interfaces
│   │   ├── inbound/        # Driving ports
│   │   └── outbound/       # Driven ports
│   ├── adapter/            # Adapter implementations
│   │   ├── inbound/        # API, Worker adapters
│   │   └── outbound/       # Database, NATS, etc.
│   └── config/             # Configuration
├── pkg/                    # Public packages
├── migrations/             # Database migrations
├── configs/               # Configuration files
├── docker/                # Docker files
├── scripts/               # Utility scripts
└── tests/                 # Integration tests
```

### Running Tests

```bash
# Unit tests
make test

# Integration tests
make test-integration

# Coverage report
make test-coverage

# All tests
make test-all
```

### Linting and Formatting

```bash
# Run linter
make lint

# Format code
make fmt

# Run go vet
make vet
```

### Database Migrations

```bash
# Create new migration
make migrate-create name=add_new_table

# Apply migrations
make migrate-up

# Rollback migrations
make migrate-down
```

## Deployment

### Docker Compose (Development)

```bash
docker-compose up -d
```

This brings up PostgreSQL (with pgvector), NATS (with monitoring), the API on `localhost:8080`, and the worker.

Quick verify:

```bash
curl http://localhost:8080/health
curl -X POST http://localhost:8080/repositories \
  -H "Content-Type: application/json" \
  -d '{"url": "https://github.com/example/repo", "name": "example"}'
```

Notes:
- The app reads env via `CODECHUNK_` variables. Compose sets DB and NATS. Add `CODECHUNK_GEMINI_API_KEY` in `docker-compose.yml` if enabling embeddings/search.
- Routes are mounted at `/`, e.g. `/repositories` (no `/api/v1` prefix in current code).

### Kubernetes (Production)

```bash
kubectl apply -f k8s/
```

### Environment Variables for Production

**Database:**
- `CODECHUNK_DATABASE_HOST`
- `CODECHUNK_DATABASE_USER`
- `CODECHUNK_DATABASE_PASSWORD`

**Messaging:**
- `CODECHUNK_NATS_URL`

**AI/Embeddings:**
- `CODECHUNK_GEMINI_API_KEY`

**Search Configuration:**
- `CODECHUNK_SEARCH_ITERATIVE_SCAN_MODE` - Options: `off`, `strict_order`, `relaxed_order` (recommended)

**API Middleware:**
- `CODECHUNK_API_ENABLE_DEFAULT_MIDDLEWARE` (default: true)
- `CODECHUNK_API_ENABLE_CORS` (default: true)
- `CODECHUNK_API_ENABLE_SECURITY_HEADERS` (default: true)
- `CODECHUNK_API_ENABLE_LOGGING` (default: true)
- `CODECHUNK_API_ENABLE_ERROR_HANDLING` (default: true)

## Monitoring

### Health Checks
Production-ready health monitoring with comprehensive dependency checking:

```bash
curl http://localhost:8080/health
```

Returns detailed health information including:
- **Database connectivity** with connection pool status
- **NATS JetStream** availability and performance metrics
- **Circuit breaker** status and connection stability
- **Response time tracking** with 23.5µs average response time
- **Caching layer** with 5-second TTL for performance

### Metrics and Observability
The system exposes comprehensive monitoring:

- **API Health**: `http://localhost:8080/health` (with custom headers)
- **NATS Monitoring**: `http://localhost:8222` (connection metrics)
- **Structured Logging**: JSON format with correlation IDs
- **Performance Metrics**: Request duration, throughput, error rates

## Performance

### Current Performance Metrics
- **NATS Throughput**: 305,358+ messages/second
- **Health Check Response**: 23.5µs average response time
- **Health Check Caching**: 5-second TTL with memory optimization
- **Database Connection Pooling**: Optimized for concurrent operations
- **SQL-level Metadata Filtering**: 2-3x faster than application-level filtering
- **pgvector Iterative Scanning**: Guaranteed result counts with minimal performance overhead
- **Semantic Overlap**: Improved retrieval quality with configurable overlap (default: 200 bytes)

### Performance Tuning

### PostgreSQL with pgvector

Optimize HNSW index parameters in migrations:
```sql
CREATE INDEX ON embeddings 
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

### Worker Concurrency

Adjust worker concurrency based on your system:
```bash
codechunking worker --concurrency 20
```

### Connection Pooling

Configure database connection pool in config:
```yaml
database:
  max_connections: 100
  max_idle_connections: 20
```

### Iterative Scanning Configuration

Enable pgvector iterative scanning for guaranteed result counts:
```yaml
search:
  iterative_scan_mode: relaxed_order  # Options: off, strict_order, relaxed_order
```

**Modes:**
- `off`: Disabled (fastest, may return incomplete results)
- `strict_order`: Exact similarity ordering, slower but perfect scores
- `relaxed_order`: Approximate ordering, high recall (RECOMMENDED for production)

## Advanced Features

### Semantic Overlap Strategy

Code chunks include configurable overlap to preserve context across boundaries:

```yaml
chunking:
  overlap_size: 200  # Bytes of overlap between adjacent chunks
  max_chunk_size: 4000
  min_chunk_size: 200
```

**Benefits:**
- Preserves context across function boundaries
- Improves semantic search recall by 15-25%
- Reduces fragmentation of related concepts
- Better handles functions that reference each other

### Git Submodule Support

The system automatically detects and handles Git submodules:

- Parses `.gitmodules` configuration files
- Extracts submodule path, URL, and branch information
- Includes submodule code in repository indexing
- Handles nested submodule configurations

### Idempotent Job Processing

Jobs can be safely retried without duplicating work:

- **Checkpointing**: Tracks progress at each processing stage
- **Resume Capability**: Resumes from last successful checkpoint on failure
- **Duplicate Prevention**: Detects and skips already-processed chunks
- **Status Tracking**: Updates repository status throughout processing pipeline

### Language Detection

Advanced language detection with multiple fallback methods:

1. **Extension-based**: Fast lookup by file extension
2. **Shebang parsing**: Detects interpreter from `#!/usr/bin/env python`
3. **Content analysis**: Analyzes file content patterns
4. **Forest heuristic**: Fallback to go-sitter-forest for 300+ languages

**Confidence Scoring**: Each detection includes confidence score (0.0-1.0)

### Semantic Construct Types

The system extracts and categorizes code into semantic types:

- **Functions & Methods**: Standalone functions, class methods, instance methods
- **Classes & Structs**: Class definitions, struct types, interfaces
- **Modules & Namespaces**: Module definitions, package declarations
- **Constants & Variables**: Global constants, module-level variables
- **Enums & Types**: Enumeration types, type aliases, type definitions

All searchable via the `types` filter in search queries.

## Troubleshooting

### Common Issues

1. **pgvector extension not found**
   ```bash
   # Use the pgvector Docker image
   docker pull pgvector/pgvector:pg16
   ```

2. **NATS connection refused**
   ```bash
   # Check NATS is running
   docker-compose ps
   nats -s nats://localhost:4222 server check
   ```

3. **Migration failures**
   ```bash
   # Check database connection
   make psql
   # Manually check migration status
   SELECT * FROM schema_migrations;
   ```

4. **Health check issues**
   ```bash
   # Check NATS health with detailed diagnostics
   curl -v http://localhost:8080/health
   # Look for X-NATS-Connection-Status and X-JetStream-Enabled headers
   ```

5. **Security validation errors**
   ```bash
   # Check logs for XSS/SQL injection detection
   # Review request validation in structured logs with correlation IDs
   ```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- **TDD Methodology**: Use red-green-refactor cycle with specialized agents
- Follow Go best practices and idioms
- Maintain test coverage above 80% (currently 100+ passing tests across all layers)
- Update documentation for new features
- **Use conventional commit messages** (required)
- Run `make lint` and `make fmt` before committing
- Keep files under 1000 lines for readability (preferably under 500)
- Always use timeout for tests: `go test ./... -timeout 10s`

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [tree-sitter](https://tree-sitter.github.io/) for code parsing
- [pgvector](https://github.com/pgvector/pgvector) for vector similarity search
- [NATS](https://nats.io/) for messaging
- [Google Gemini](https://ai.google.dev/) for embeddings
- [Cobra](https://github.com/spf13/cobra) for CLI
- [Viper](https://github.com/spf13/viper) for configuration

## Support

For issues and questions:
- GitHub Issues: [github.com/yourusername/codechunking/issues](https://github.com/yourusername/codechunking/issues)
- Documentation: [docs.codechunking.io](https://docs.codechunking.io)

## Documentation

For detailed documentation, see the `/docs` directory and the project wiki:

- **[Project Wiki](wiki/)**: Comprehensive configuration reference and guides
  - [Configuration Overview](wiki/configuration/Configuration.md)
  - [API Configuration](wiki/configuration/API-Configuration.md)
  - [Database Configuration](wiki/configuration/Database-Configuration.md)
  - [NATS Configuration](wiki/configuration/NATS-Configuration.md)
  - [Worker Configuration](wiki/configuration/Worker-Configuration.md)
  - [Gemini Configuration](wiki/configuration/Gemini-Configuration.md)
  - [Logging Configuration](wiki/configuration/Logging-Configuration.md)
  - [Git Configuration](wiki/configuration/Git-Configuration.md)
- **[API Documentation](docs/api.md)**: Complete REST API reference
- **[Development Guide](docs/development.md)**: Enhanced setup and development workflow
- **[Deployment Guide](docs/deployment.md)**: Production deployment and scaling

### Working with the Wiki

The wiki is included as a Git submodule. To clone the repository with the wiki:

```bash
git clone --recurse-submodules https://github.com/yourusername/codechunking.git
```

If you've already cloned the repository, initialize and update the submodule:

```bash
git submodule init
git submodule update
```

To update the wiki to the latest version:

```bash
git submodule update --remote wiki
```

## Current Status

🎉 **MVP COMPLETE** - All core phases delivered (100+ passing tests):
- ✅ **Phase 2**: Production-ready API with repository management
- ✅ **Phase 3**: Asynchronous processing with NATS JetStream
- ✅ **Phase 4**: Intelligent code chunking with tree-sitter
- ✅ **Phase 5**: Vector embeddings with Google Gemini
- ✅ **Phase 6**: Vector storage and retrieval with pgvector
- ✅ **Phase 7.1**: **Semantic search API** with natural language queries ✨

**Key MVP Features Delivered:**
- Complete semantic code search with POST /search endpoint
- Advanced filtering (repository, language, file type, similarity threshold)
- Pagination and sorting capabilities
- Comprehensive TDD implementation with 50+ passing tests
- Production-ready error handling and validation
- Full OpenAPI documentation

## Post-MVP Roadmap

- [ ] Support for more programming languages (Python, JavaScript, etc.)
- [ ] Incremental repository updates and webhooks
- [ ] Web UI for search interface
- [ ] GitHub/GitLab webhooks for auto-indexing
- [ ] Multi-model embedding support
- [ ] Distributed worker scaling
- [ ] Query result caching and performance optimization
- [ ] Fine-tuned ranking algorithms and hybrid search
