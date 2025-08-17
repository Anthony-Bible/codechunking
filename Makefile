.PHONY: dev test build migrate clean help

# Variables
BINARY_NAME=codechunking
DOCKER_COMPOSE=docker compose
GO_CMD=go
MIGRATE_CMD=migrate

# Default target
all: build

## help: Display this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^##' Makefile | sed 's/## /  /'

## dev: Start development environment (Docker services)
dev:
	$(DOCKER_COMPOSE) up -d
	@echo "Development environment started"
	@echo "PostgreSQL: localhost:5432"
	@echo "NATS: localhost:4222"
	@echo "NATS Monitor: http://localhost:8222"

## dev-down: Stop development environment
dev-down:
	$(DOCKER_COMPOSE) down
	@echo "Development environment stopped"

## dev-clean: Stop and remove volumes
dev-clean:
	$(DOCKER_COMPOSE) down -v
	@echo "Development environment cleaned"

## dev-api: Run API server in development mode
dev-api:
	$(GO_CMD) run main.go api --config configs/config.dev.yaml

## dev-worker: Run worker in development mode
dev-worker:
	$(GO_CMD) run main.go worker --config configs/config.dev.yaml

## dev-all: Run both API and worker (requires goreman)
dev-all:
	@command -v goreman >/dev/null 2>&1 || { echo "goreman is required but not installed. Install with: go install github.com/mattn/goreman@latest"; exit 1; }
	@echo "api: make dev-api" > Procfile
	@echo "worker: make dev-worker" >> Procfile
	goreman start

## test: Run unit tests
test:
	$(GO_CMD) test -v -short -timeout 30s ./...

## test-integration: Run integration tests
test-integration:
	$(DOCKER_COMPOSE) up -d
	$(GO_CMD) test -v -tags=integration -timeout 60s ./...

## test-coverage: Generate test coverage report
test-coverage:
	$(GO_CMD) test -v -timeout 60s -coverprofile=coverage.out ./...
	$(GO_CMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-all: Run all tests with coverage
test-all: test test-integration test-coverage

## build: Build the binary
build:
	CGO_ENABLED=0 $(GO_CMD) build -o bin/$(BINARY_NAME) main.go
	@echo "Binary built: bin/$(BINARY_NAME)"

## build-linux: Build for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO_CMD) build -o bin/$(BINARY_NAME)-linux-amd64 main.go
	@echo "Linux binary built: bin/$(BINARY_NAME)-linux-amd64"

## build-docker: Build Docker images
build-docker:
	docker build -f docker/Dockerfile -t $(BINARY_NAME):latest .
	@echo "Docker image built: $(BINARY_NAME):latest"

## migrate-up: Apply all database migrations
migrate-up:
	$(GO_CMD) run main.go migrate up --config configs/config.dev.yaml

## migrate-down: Rollback all database migrations
migrate-down:
	$(GO_CMD) run main.go migrate down

## migrate-create: Create a new migration (usage: make migrate-create name=migration_name)
migrate-create:
	@if [ -z "$(name)" ]; then echo "Error: name is required. Usage: make migrate-create name=migration_name"; exit 1; fi
	$(MIGRATE_CMD) create -ext sql -dir migrations -seq $(name)

## run-api: Run API server with default config
run-api: build
	./bin/$(BINARY_NAME) api

## run-worker: Run worker with default config
run-worker: build
	./bin/$(BINARY_NAME) worker

## run-prod: Run API in production mode
run-prod: build
	./bin/$(BINARY_NAME) --config configs/config.prod.yaml api

## lint: Run linter
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is required but not installed. Install from https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

## fmt: Format code
fmt:
	$(GO_CMD) fmt ./...
	$(GO_CMD) mod tidy

## vet: Run go vet
vet:
	$(GO_CMD) vet ./...

## mod-download: Download dependencies
mod-download:
	$(GO_CMD) mod download

## mod-tidy: Tidy dependencies
mod-tidy:
	$(GO_CMD) mod tidy

## mod-verify: Verify dependencies
mod-verify:
	$(GO_CMD) mod verify

## nats-stream-info: Show NATS stream information
nats-stream-info:
	@command -v nats >/dev/null 2>&1 || { echo "NATS CLI is required but not installed. Install from https://github.com/nats-io/natscli"; exit 1; }
	nats stream info INDEXING

## nats-consumer-info: Show NATS consumer information
nats-consumer-info:
	@command -v nats >/dev/null 2>&1 || { echo "NATS CLI is required but not installed. Install from https://github.com/nats-io/natscli"; exit 1; }
	nats consumer info INDEXING workers

## psql: Connect to PostgreSQL database
psql:
	docker exec -it codechunking-postgres psql -U dev -d codechunking

## logs-api: Show API logs
logs-api:
	docker logs -f codechunking-api 2>&1 | jq '.' || docker logs -f codechunking-api

## logs-worker: Show worker logs
logs-worker:
	docker logs -f codechunking-worker 2>&1 | jq '.' || docker logs -f codechunking-worker

## clean: Clean build artifacts and temporary files
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f Procfile
	rm -rf tmp/
	@echo "Cleaned build artifacts"

## install-tools: Install development tools
install-tools:
	go install github.com/mattn/goreman@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/spf13/cobra-cli@latest
	@echo "Development tools installed"

## version: Show version information
version:
	./bin/$(BINARY_NAME) version || $(GO_CMD) run main.go version
