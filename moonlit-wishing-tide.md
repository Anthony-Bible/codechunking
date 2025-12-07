# AI Agent CLI Client Implementation Plan

## Overview

Create a search-focused CLI client designed for AI agent consumption with JSON-only output. The client works both as a subcommand (`codechunking client`) and as a standalone binary (`codechunking-client`).

## Requirements Summary

- **Output**: JSON only (structured for AI agent parsing)
- **Operations**: Search-focused with minimal repo management
- **Async**: Both modes via `--wait` flag
- **Structure**: Subcommand + standalone binary

---

## File Structure

```
cmd/
├── client.go                     # Registers "client" subcommand with root
└── client/
    └── main.go                   # Standalone binary entry point

internal/client/
├── client.go                     # HTTP client for API communication
├── client_test.go
├── config.go                     # Configuration (Viper-based)
├── config_test.go
├── output.go                     # JSON response envelope
├── output_test.go
├── poller.go                     # Async job polling
├── poller_test.go
└── commands/
    ├── root.go                   # Root command (shared between modes)
    ├── search.go                 # Primary search command
    ├── search_test.go
    ├── repos.go                  # repos list, get, add
    ├── repos_test.go
    ├── jobs.go                   # jobs get
    ├── jobs_test.go
    ├── health.go                 # health check
    └── health_test.go
```

---

## Implementation Details

### 1. HTTP Client (`internal/client/client.go`)

```go
type Client struct {
    baseURL    string
    httpClient *http.Client
}

type Config struct {
    BaseURL  string        // default: http://localhost:8080
    Timeout  time.Duration // default: 30s
}

// Methods (reuse existing DTOs from internal/application/dto/)
func (c *Client) Search(ctx context.Context, req dto.SearchRequestDTO) (*dto.SearchResponseDTO, error)
func (c *Client) ListRepositories(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error)
func (c *Client) GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error)
func (c *Client) CreateRepository(ctx context.Context, req dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error)
func (c *Client) GetIndexingJob(ctx context.Context, repoID, jobID uuid.UUID) (*dto.IndexingJobResponse, error)
func (c *Client) Health(ctx context.Context) (*dto.HealthResponse, error)
```

### 2. JSON Output Envelope (`internal/client/output.go`)

All output uses consistent envelope:

```go
type Response struct {
    Success   bool        `json:"success"`
    Data      interface{} `json:"data,omitempty"`
    Error     *Error      `json:"error,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
}

type Error struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

func WriteSuccess(w io.Writer, data interface{}) error
func WriteError(w io.Writer, code, message string, details interface{}) error
```

### 3. Commands

#### Search (Primary Command)
```bash
codechunking client search "implement authentication" \
  --limit 10 \
  --repo-names org/repo1,org/repo2 \
  --languages go,python \
  --threshold 0.8 \
  --types function,method
```

Flags (mirror `dto.SearchRequestDTO`):
- `--limit, -l` (int, default 10)
- `--offset` (int, default 0)
- `--repo-ids` ([]uuid)
- `--repo-names` ([]string)
- `--languages` ([]string)
- `--file-types` ([]string)
- `--threshold, -t` (float64, default 0.7)
- `--sort, -s` (string, default "similarity:desc")
- `--types` ([]string)
- `--entity-name` (string)
- `--visibility` ([]string)

#### Repos Commands
```bash
codechunking client repos list --status completed --limit 20
codechunking client repos get <uuid>
codechunking client repos add https://github.com/org/repo --wait --poll-interval 5s
```

`repos add` flags:
- `--name` (string)
- `--description` (string)
- `--branch` (string, default "main")
- `--wait, -w` (bool) - poll until complete
- `--poll-interval` (duration, default 5s)
- `--timeout` (duration, default 30m)

#### Jobs Command
```bash
codechunking client jobs get <repo-id> <job-id>
```

#### Health Command
```bash
codechunking client health
```

### 4. Configuration

**Priority**: CLI flags > Environment > Config file > Defaults

**Environment Variables** (prefix `CODECHUNK_CLIENT_`):
- `CODECHUNK_CLIENT_API_URL` (default: http://localhost:8080)
- `CODECHUNK_CLIENT_TIMEOUT` (default: 30s)

**Global Flags**:
- `--api-url` (string)
- `--timeout` (duration)

### 5. Async Polling (`internal/client/poller.go`)

```go
type Poller struct {
    client       *Client
    interval     time.Duration
    maxWait      time.Duration
}

func (p *Poller) WaitForCompletion(ctx context.Context, repoID uuid.UUID) (*dto.RepositoryResponse, error)
```

Outputs progress JSON to stderr while waiting:
```json
{"status": "polling", "repository_id": "...", "current_status": "cloning", "elapsed": "10s"}
```

### 6. Build Configuration

Add to `Makefile`:
```makefile
build-client:
	CGO_ENABLED=0 go build -o bin/codechunking-client ./cmd/client

build-client-cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/codechunking-client-linux-amd64 ./cmd/client
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/codechunking-client-darwin-amd64 ./cmd/client

test-client:
	go test -v -short -timeout 30s ./internal/client/...
```

Note: `CGO_ENABLED=0` for static binary (no tree-sitter dependency in client)

---

## TDD Workflow & Agent Usage

Each phase follows this strict workflow:

```
┌─────────────────────────────────────────────────────────────────┐
│  BEFORE PHASE: Check recent commits (git log --oneline -10)    │
├─────────────────────────────────────────────────────────────────┤
│  1. RED PHASE (@red-phase-tester)                               │
│     → Write failing tests that define expected behavior         │
│     → Commit: "test(client): add failing tests for <component>" │
├─────────────────────────────────────────────────────────────────┤
│  2. GREEN PHASE (@green-phase-implementer)                      │
│     → Write minimal code to make tests pass                     │
│     → Commit: "feat(client): implement <component>"             │
├─────────────────────────────────────────────────────────────────┤
│  3. REFACTOR PHASE (@tdd-refactor-specialist)                   │
│     → Clean up code, improve structure                          │
│     → Commit: "refactor(client): clean up <component>"          │
├─────────────────────────────────────────────────────────────────┤
│  4. REVIEW (@tdd-review-agent)                                  │
│     → Verify implementation completeness                        │
│     → Check for unimplemented tests or unnecessary mocks        │
├─────────────────────────────────────────────────────────────────┤
│  5. VERIFICATION (general-purpose agent)                        │
│     → Run: go test ./internal/client/... -v -timeout 30s        │
│     → Run: go build ./cmd/client (if applicable)                │
│     → Run CLI manually to verify JSON output format             │
│     → Report any issues before proceeding to next phase         │
└─────────────────────────────────────────────────────────────────┘
```

---

## Implementation Phases (Detailed TDD)

### Phase 1: Core Infrastructure - JSON Output

**Pre-check**: `git log --oneline -10` to review recent commits

**Files**: `internal/client/output.go`, `internal/client/output_test.go`

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 1.1 | `@red-phase-tester` | Write tests for `Response` struct, `WriteSuccess()`, `WriteError()` | `test(client): add failing tests for JSON output envelope` |
| 1.2 | `@green-phase-implementer` | Implement output.go to pass tests | `feat(client): implement JSON output envelope` |
| 1.3 | `@tdd-refactor-specialist` | Clean up, add documentation | `refactor(client): clean up output formatting` |
| 1.4 | `@tdd-review-agent` | Review for completeness | - |
| 1.5 | `general-purpose` | Run `go test ./internal/client/... -v` | - |

---

### Phase 2: Core Infrastructure - Configuration

**Pre-check**: `git log --oneline -10`

**Files**: `internal/client/config.go`, `internal/client/config_test.go`

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 2.1 | `@red-phase-tester` | Write tests for `Config` struct, defaults, env var loading | `test(client): add failing tests for client configuration` |
| 2.2 | `@green-phase-implementer` | Implement config.go with Viper | `feat(client): implement client configuration` |
| 2.3 | `@tdd-refactor-specialist` | Refactor config handling | `refactor(client): clean up configuration loading` |
| 2.4 | `@tdd-review-agent` | Review for completeness | - |
| 2.5 | `general-purpose` | Run `go test ./internal/client/... -v` | - |

---

### Phase 3: Core Infrastructure - HTTP Client

**Pre-check**: `git log --oneline -10`

**Files**: `internal/client/client.go`, `internal/client/client_test.go`

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 3.1 | `@red-phase-tester` | Write tests for `Client` struct, HTTP methods, error handling | `test(client): add failing tests for HTTP client` |
| 3.2 | `@green-phase-implementer` | Implement client.go with all API methods | `feat(client): implement HTTP API client` |
| 3.3 | `@tdd-refactor-specialist` | Extract common HTTP logic, improve error handling | `refactor(client): clean up HTTP client implementation` |
| 3.4 | `@tdd-review-agent` | Review for completeness | - |
| 3.5 | `general-purpose` | Run `go test ./internal/client/... -v` | - |

---

### Phase 4: Health Command (First Command - Validates Infrastructure)

**Pre-check**: `git log --oneline -10`

**Files**:
- `internal/client/commands/root.go`
- `internal/client/commands/health.go`, `health_test.go`
- `cmd/client.go`
- `cmd/client/main.go`

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 4.1 | `@red-phase-tester` | Write tests for health command JSON output | `test(client): add failing tests for health command` |
| 4.2 | `@green-phase-implementer` | Implement root.go, health.go, cmd/client.go, cmd/client/main.go | `feat(client): implement health command and CLI structure` |
| 4.3 | `@tdd-refactor-specialist` | Clean up command structure | `refactor(client): clean up health command` |
| 4.4 | `@tdd-review-agent` | Review for completeness | - |
| 4.5 | `general-purpose` | Verify: `go build ./cmd/client && ./bin/codechunking-client health` | - |

---

### Phase 5: Repository Commands (list, get, add without wait)

**Pre-check**: `git log --oneline -10`

**Files**: `internal/client/commands/repos.go`, `repos_test.go`

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 5.1 | `@red-phase-tester` | Write tests for `repos list`, `repos get`, `repos add` | `test(client): add failing tests for repos commands` |
| 5.2 | `@green-phase-implementer` | Implement all three subcommands | `feat(client): implement repos list, get, add commands` |
| 5.3 | `@tdd-refactor-specialist` | Extract shared logic, improve flag handling | `refactor(client): clean up repos commands` |
| 5.4 | `@tdd-review-agent` | Review for completeness | - |
| 5.5 | `general-purpose` | Verify: `./bin/codechunking-client repos list`, `repos get <id>` | - |

---

### Phase 6: Async Polling & --wait Flag

**Pre-check**: `git log --oneline -10`

**Files**: `internal/client/poller.go`, `poller_test.go`, update `repos.go`

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 6.1 | `@red-phase-tester` | Write tests for `Poller`, timeout handling, status polling | `test(client): add failing tests for async job polling` |
| 6.2 | `@green-phase-implementer` | Implement poller.go, add --wait flag to repos add | `feat(client): implement async job polling with --wait flag` |
| 6.3 | `@tdd-refactor-specialist` | Improve polling logic, error messages | `refactor(client): clean up polling implementation` |
| 6.4 | `@tdd-review-agent` | Review for completeness | - |
| 6.5 | `general-purpose` | Test with mock or real API: `repos add <url> --wait` | - |

---

### Phase 7: Search Command (Primary Feature)

**Pre-check**: `git log --oneline -10`

**Files**: `internal/client/commands/search.go`, `search_test.go`

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 7.1 | `@red-phase-tester` | Write tests for all search flags, filtering, output format | `test(client): add failing tests for search command` |
| 7.2 | `@green-phase-implementer` | Implement search.go with all flags | `feat(client): implement search command` |
| 7.3 | `@tdd-refactor-specialist` | Optimize flag parsing, improve UX | `refactor(client): clean up search command` |
| 7.4 | `@tdd-review-agent` | Review for completeness | - |
| 7.5 | `general-purpose` | Verify: `./bin/codechunking-client search "test query"` | - |

---

### Phase 8: Jobs Command

**Pre-check**: `git log --oneline -10`

**Files**: `internal/client/commands/jobs.go`, `jobs_test.go`

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 8.1 | `@red-phase-tester` | Write tests for `jobs get` command | `test(client): add failing tests for jobs command` |
| 8.2 | `@green-phase-implementer` | Implement jobs.go | `feat(client): implement jobs get command` |
| 8.3 | `@tdd-refactor-specialist` | Clean up implementation | `refactor(client): clean up jobs command` |
| 8.4 | `@tdd-review-agent` | Review for completeness | - |
| 8.5 | `general-purpose` | Verify: `./bin/codechunking-client jobs get <repo-id> <job-id>` | - |

---

### Phase 9: Polish & Integration

**Pre-check**: `git log --oneline -10`

**Files**: Makefile, integration tests

| Step | Agent | Action | Commit Message |
|------|-------|--------|----------------|
| 9.1 | `general-purpose` | Add Makefile targets (build-client, test-client) | `build(client): add Makefile targets for client` |
| 9.2 | `@red-phase-tester` | Write integration tests against running API | `test(client): add integration tests` |
| 9.3 | `@green-phase-implementer` | Fix any integration test failures | `fix(client): resolve integration test issues` |
| 9.4 | `@tdd-review-agent` | Final review of entire client | - |
| 9.5 | `general-purpose` | Full verification: run all tests, build, manual CLI testing | - |

---

## Verification Checklist (Run After Each Phase)

The verification agent (`general-purpose`) should run these checks:

```bash
# 1. Run all client tests
go test ./internal/client/... -v -timeout 30s

# 2. Build standalone binary (after Phase 4+)
CGO_ENABLED=0 go build -o bin/codechunking-client ./cmd/client

# 3. Build main binary with subcommand
go build -o bin/codechunking .

# 4. Test CLI output is valid JSON (after Phase 4+)
./bin/codechunking-client health 2>/dev/null | jq .

# 5. Check for linting issues
golangci-lint run ./internal/client/...

# 6. Verify no breaking changes to existing code
go test ./... -short -timeout 60s
```

---

## Commit Message Format

All commits follow conventional commits:

- `test(client):` - Test files (red phase)
- `feat(client):` - Implementation (green phase)
- `refactor(client):` - Refactoring (refactor phase)
- `fix(client):` - Bug fixes found during review
- `build(client):` - Build configuration changes
- `docs(client):` - Documentation updates

---

## Critical Files to Reference

| File | Purpose |
|------|---------|
| `cmd/chunk.go` | Cobra command pattern with JSON output |
| `cmd/root.go` | Viper configuration setup |
| `internal/application/dto/search.go` | SearchRequestDTO, SearchResponseDTO |
| `internal/application/dto/repository.go` | Repository DTOs |
| `internal/application/dto/health.go` | HealthResponse |
| `internal/application/dto/job.go` | IndexingJobResponse |
| `api/openapi.yaml` | Full API specification |

---

## Example Output

### Search Success
```json
{
  "success": true,
  "data": {
    "results": [
      {
        "chunk_id": "...",
        "content": "func authenticateUser() {...}",
        "similarity_score": 0.95,
        "repository": {"id": "...", "name": "auth-service"},
        "file_path": "/middleware/auth.go",
        "language": "go",
        "start_line": 15,
        "end_line": 25,
        "type": "function",
        "entity_name": "authenticateUser"
      }
    ],
    "pagination": {"limit": 10, "offset": 0, "total": 42, "has_more": true},
    "search_metadata": {"query": "authentication", "execution_time_ms": 150}
  },
  "timestamp": "2025-01-08T10:30:00Z"
}
```

### Error Response
```json
{
  "success": false,
  "error": {
    "code": "REPOSITORY_NOT_FOUND",
    "message": "Repository with the specified ID was not found",
    "details": {"repository_id": "..."}
  },
  "timestamp": "2025-01-08T10:30:00Z"
}
```
