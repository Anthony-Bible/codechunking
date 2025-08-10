# Deployment Guide

This guide covers deploying the CodeChunking system in production environments.

## Overview

CodeChunking is designed for production deployment with:
- High-performance NATS JetStream messaging (305,358+ msg/sec)
- Advanced health monitoring with 23.5µs response times
- Circuit breaker patterns for resilience
- Comprehensive security validation
- Structured logging with correlation tracking

## Prerequisites

### External Dependencies
- **PostgreSQL 16+** with pgvector extension
- **NATS JetStream 2.10+**
- **Google Gemini API** access
- **Container Runtime** (Docker/Kubernetes)

## Configuration

### Environment Variables

See `.env.example` in the repository root for all available environment variables with examples.

**Required Variables:**
```bash
CODECHUNK_DATABASE_HOST=your-postgres-host
CODECHUNK_DATABASE_PASSWORD=your-secure-password
CODECHUNK_NATS_URL=nats://your-nats-host:4222
CODECHUNK_GEMINI_API_KEY=your-gemini-api-key
```

### Configuration Files

The repository includes production-ready configuration files:

- **Base config**: `configs/config.yaml` 
- **Development**: `configs/config.dev.yaml`
- **Production**: `configs/config.prod.yaml`

Review and customize `configs/config.prod.yaml` for your production environment.

## Docker Deployment

### Using Repository Docker Files

The repository includes production-ready Docker configurations:

**Dockerfile**: See `docker/Dockerfile` in the repository
**Docker Compose**: Use the provided `docker-compose.yml`

```bash
# Copy and customize environment
cp .env.example .env
# Edit .env with your production values

# Deploy with Docker Compose
docker-compose -f docker-compose.yml up -d
```

### Production Environment Setup

1. **Database Setup**: Use the init script at `docker/postgres/init.sql`
2. **Migrations**: Run `make migrate-up` or use the migrate command
3. **SSL/TLS**: Configure your reverse proxy for SSL termination

## Kubernetes Deployment

### Using Repository Examples

While the repository doesn't currently include Kubernetes manifests, you can create them based on the Docker Compose configuration in `docker-compose.yml`.

**Key points from the compose file:**
- PostgreSQL uses `pgvector/pgvector:pg16` image
- NATS uses `nats:2.10-alpine` with JetStream enabled (`-js` flag)
- Application expects config at `/root/configs/`
- Health check endpoint is available at `/health`

### Database Migration

Use the repository's migration system:
```bash
# In your init container or job
./codechunking migrate up --config configs/config.prod.yaml
```

Migration files are located in `/migrations/` directory.

## Monitoring and Observability

### Health Checks

The system provides comprehensive health monitoring at `/health`:

```bash
curl https://your-api-host/health
```

Health check features (see `internal/adapter/inbound/api/health_handler.go`):
- Database connectivity checking
- NATS JetStream status with detailed metrics
- Circuit breaker status
- Response time tracking with custom headers
- 5-second caching for performance (23.5µs average response time)

### Structured Logging

All components use structured JSON logging with correlation IDs. See the logging implementation in:
- `internal/application/common/logging/`
- `internal/adapter/inbound/api/middleware/logging_middleware.go`

Log format includes:
- Correlation IDs for request tracing
- Performance metrics
- Component identification
- Error context and stack traces

## Security Considerations

### Built-in Security Features

The system includes comprehensive security validation (see `internal/application/common/security/`):
- XSS prevention with HTML entity escaping
- SQL injection protection with parameterized queries  
- URL validation with Git provider whitelist
- Unicode attack prevention
- Input sanitization with fuzzing test coverage

### Network Security
- Use TLS for all external communications
- Implement network policies in Kubernetes
- Restrict database access to application components only
- Review the security middleware in `internal/adapter/inbound/api/security_middleware.go`

### Secrets Management
- Store API keys securely (never in version control)
- Use external secret management systems
- The `.env.example` file shows which values need to be secured

## Development vs Production

### Make Targets

Use the provided Makefile for deployment tasks:
- `make build` - Build production binary
- `make build-docker` - Build Docker image
- `make migrate-up` - Apply database migrations
- `make test-integration` - Run integration tests before deployment

### Configuration Hierarchy

The system uses Viper configuration with this priority:
1. Command-line flags (highest)
2. Environment variables (with `CODECHUNK_` prefix)
3. Config files (`config.prod.yaml`)
4. Defaults (lowest)

See `internal/config/config.go` for the complete configuration structure.

## Performance

### Current Metrics
Based on the test suite results:
- **NATS Throughput**: 305,358+ messages/second
- **Health Check Response**: 23.5µs average response time
- **Health Check Caching**: 5-second TTL with memory optimization

### Database Optimization

See the migration files in `/migrations/` for optimized indexes:
- HNSW vector indexes for pgvector
- Unique constraints for duplicate prevention
- Performance indexes on frequently queried fields

### Worker Scaling

Configure worker concurrency using:
```bash
./codechunking worker --concurrency 10
```

Or set via environment variable:
```bash
CODECHUNK_WORKER_CONCURRENCY=10
```

## Troubleshooting

### Common Issues

Refer to the main README.md troubleshooting section for common deployment issues including:
- pgvector extension problems
- NATS connection issues
- Migration failures

### Health Diagnostics

Use the enhanced health endpoint for troubleshooting:
```bash
curl -v https://your-api-host/health
```

Check response headers:
- `X-Health-Check-Duration`: Response time diagnostics
- `X-NATS-Connection-Status`: NATS connectivity
- `X-JetStream-Enabled`: JetStream availability

### Log Analysis

Review structured logs for:
- Correlation IDs to trace requests across components
- Performance metrics for bottleneck identification
- Security validation events
- Circuit breaker state changes

## Repository References

For detailed implementation and configuration examples:

- **Configuration**: `/configs/` directory
- **Docker**: `/docker/` directory and `docker-compose.yml`
- **Database**: `/migrations/` directory
- **Health Monitoring**: `/internal/adapter/inbound/api/health_handler.go`
- **Security**: `/internal/application/common/security/`
- **Logging**: `/internal/application/common/logging/`
- **Environment Variables**: `.env.example`

## Support

For deployment issues:
- Review the troubleshooting section in README.md
- Check the test files for configuration examples
- Examine the Docker Compose setup for service dependencies