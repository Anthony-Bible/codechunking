# CodeChunking

A production-grade code chunking and retrieval system for semantic code search using Go, PostgreSQL with pgvector, NATS JetStream, and Google Gemini.

## Features

- **Repository Indexing**: Clone and index Git repositories automatically
- **Intelligent Code Chunking**: Parse code into semantic units using tree-sitter
- **Vector Embeddings**: Generate embeddings with Google Gemini API
- **Similarity Search**: Fast vector search using PostgreSQL with pgvector
- **Asynchronous Processing**: Scalable job processing with NATS JetStream
- **Hexagonal Architecture**: Clean, maintainable code structure
- **CLI Interface**: Cobra-based CLI with multiple commands
- **Configuration Management**: Flexible configuration with Viper

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

## Prerequisites

- Go 1.24 or higher
- Docker and Docker Compose
- PostgreSQL with pgvector extension
- NATS JetStream
- Google Gemini API key

## Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/yourusername/codechunking.git
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

## Installation

### Using Go

```bash
go install github.com/yourusername/codechunking@latest
```

### Using Docker

```bash
docker pull yourusername/codechunking:latest
```

### From source

```bash
git clone https://github.com/yourusername/codechunking.git
cd codechunking
make build
```

## Usage

### CLI Commands

```bash
# Show help
codechunking --help

# Start API server
codechunking api --config configs/config.dev.yaml

# Start worker
codechunking worker --concurrency 10

# Run migrations
codechunking migrate up

# Show version
codechunking version
```

### API Endpoints

#### Index a repository
```bash
curl -X POST http://localhost:8080/api/v1/repositories \
  -H "Content-Type: application/json" \
  -d '{"url": "https://github.com/example/repo"}'
```

#### Search code
```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query": "implement authentication"}'
```

## Configuration

Configuration can be provided through:
1. Configuration files (YAML)
2. Environment variables (prefix: `CODECHUNK_`)
3. Command-line flags

Priority: Flags > Environment > Config File > Defaults

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

### Kubernetes (Production)

```bash
kubectl apply -f k8s/
```

### Environment Variables for Production

- `CODECHUNK_DATABASE_HOST`
- `CODECHUNK_DATABASE_USER`
- `CODECHUNK_DATABASE_PASSWORD`
- `CODECHUNK_NATS_URL`
- `CODECHUNK_GEMINI_API_KEY`

## Monitoring

The system exposes metrics in Prometheus format:

- API metrics: `http://localhost:8080/metrics`
- Worker metrics: `http://localhost:8081/metrics`
- NATS monitoring: `http://localhost:8222`

## Performance Tuning

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

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and idioms
- Maintain test coverage above 80%
- Update documentation for new features
- Use conventional commit messages
- Run `make lint` before committing

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

## Roadmap

- [ ] Support for more programming languages
- [ ] Incremental repository updates
- [ ] Web UI for search interface
- [ ] GitHub/GitLab webhooks for auto-indexing
- [ ] Multi-model embedding support
- [ ] Distributed worker scaling
- [ ] Query result caching
- [ ] Fine-tuned ranking algorithms