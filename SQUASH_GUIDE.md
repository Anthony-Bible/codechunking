# Squash Guide for Branch `25-add-client-cli`

This guide will help you squash 29 commits into 5 logical feature groups.

## Prerequisites

```bash
# Create a backup branch first
git branch 25-add-client-cli-backup

# Start interactive rebase
git rebase -i origin/master
```

## Rebase Instructions

Your editor will open with commits listed **oldest first**. Replace the contents with the following (copy/paste the entire block):

```
# Group 1: Output Formatting (4 commits → 1)
pick 7b0271a test(client): add failing tests for JSON output envelope
squash 45f5b0c feat(client): implement JSON output envelope
squash e2971e3 refactor(client): enhance documentation for output formatting
squash d702413 fix(client): reduce cognitive complexity in output tests

# Group 2: Configuration (5 commits → 1)
pick 85b620e test(client): add failing tests for client configuration
squash 9672bb1 feat(client): implement client configuration
squash 680477e refactor(client): fix linting issues in config tests
squash 0909ae3 refactor(client): clean up configuration handling
squash aaa325c fix(client): reduce cognitive complexity in config tests

# Group 3: HTTP Client (3 commits → 1)
pick 63c4cdb test(client): add failing tests for HTTP client
squash 306e999 feat(client): implement HTTP API client
squash e48b303 refactor(client): extract constants and improve documentation

# Group 4: CLI Commands (15 commits → 1)
pick 5ee635c test(client): add failing tests for health command and CLI structure
squash a62e9da feat(client): implement health command and CLI structure
squash 3a05436 refactor(client): extract constants and improve documentation for commands
squash 7bc4416 test(client): add failing tests for repository commands
squash e260f17 feat(client): implement repository commands (list, get, add)
squash 4bf7f1f refactor(client): extract helper function and constants for repos commands
squash b270df5 test(client): add failing tests for async job polling
squash d29a27b feat(client): implement async job polling with --wait flag
squash 03a4634 refactor(client): improve documentation and extract constants for poller
squash 3a2ac39 test(client): add failing tests for search command
squash 531231c feat(client): implement search command with filtering
squash 6a76446 refactor(client): improve documentation for search command
squash c061134 test(client): add failing tests for jobs command
squash e2e48f9 feat(client): implement jobs command with get subcommand
squash 10ad0b5 refactor(client): add documentation to jobs command

# Group 5: Build, Integration Tests & Docs (3 commits → 1)
pick 5df73f1 build(client): add Makefile targets and standalone binary entry point
squash ec0fb10 test(client): add integration tests for CLI commands
squash c90fc23 test(repository): use uuid-suffixed URLs in tests to avoid collisions
squash 72f88db docs(readme): add Client CLI section with usage, flags, JSON output and error codes
```

Save and close the editor. Git will then prompt you for commit messages for each group.

---

## Commit Messages for Each Group

### Group 1: Output Formatting
When prompted, replace all the commit messages with:

```
feat(client): add JSON output envelope for consistent API responses

Add structured JSON output formatting for the CLI client:
- Response struct with success field, data payload, and timestamp
- Error struct with message and optional details
- WriteSuccess/WriteError functions for consistent output
- Comprehensive test coverage with edge cases

The output envelope ensures all CLI responses follow a consistent
JSON structure that can be easily parsed by scripts and tooling.
```

### Group 2: Configuration
```
feat(client): implement client configuration management

Add configuration layer for the CLI client using Viper:
- Support for config file, environment variables, and flags
- Server URL, timeout, and output format settings
- Config file locations: ./config.yaml, ~/.codechunking/config.yaml
- Environment variable prefix: CODECHUNK_CLIENT_
- Comprehensive test coverage for all config sources

Configuration precedence: flags > env vars > config file > defaults
```

### Group 3: HTTP Client
```
feat(client): implement HTTP API client

Add core HTTP client for communicating with the codechunking API:
- Client struct with configurable base URL and timeout
- Methods for all API endpoints (health, repositories, search, jobs)
- Proper error handling with typed errors
- Request/response marshaling with JSON
- Comprehensive test coverage with mock server

The client provides a clean Go API for interacting with the
codechunking server from the CLI commands.
```

### Group 4: CLI Commands
```
feat(client): implement CLI commands for repository management

Add Cobra-based CLI with comprehensive command structure:

Commands:
- `health` - Check API server health status
- `repos list` - List all repositories
- `repos get <id>` - Get repository details
- `repos add <url>` - Add repository for indexing
  - `--wait` flag for async job polling until completion
- `search <query>` - Search code chunks
  - `--repo` filter by repository
  - `--lang` filter by language
  - `--limit` control result count
- `jobs get <id>` - Get job status and details

Features:
- Consistent JSON output format across all commands
- Async job polling with configurable timeout
- Global flags: --server, --timeout, --output
- Comprehensive test coverage using TDD approach
```

### Group 5: Build & Documentation
```
build(client): add build system, integration tests, and documentation

Build & Distribution:
- Makefile targets: build-client, install-client, client-help
- Standalone binary entry point at cmd/client/main.go
- Binary output to bin/codechunking-client

Testing:
- Integration tests for all CLI commands against live API
- UUID-suffixed URLs in repository tests to avoid collisions
- Test utilities for capturing CLI output

Documentation:
- README section covering CLI usage and examples
- Flag documentation and JSON output format
- Exit codes and error handling guide
```

---

## After Rebasing

```bash
# Verify the squashed commits
git log --oneline -10

# Force push to update remote (CAREFUL!)
git push --force-with-lease origin 25-add-client-cli
```

## If Something Goes Wrong

```bash
# Restore from backup
git checkout 25-add-client-cli
git reset --hard 25-add-client-cli-backup
```

---

## Expected Result

Before: 29 commits
After: 5 commits

```
<new-hash> build(client): add build system, integration tests, and documentation
<new-hash> feat(client): implement CLI commands for repository management
<new-hash> feat(client): implement HTTP API client
<new-hash> feat(client): implement client configuration management
<new-hash> feat(client): add JSON output envelope for consistent API responses
```

Delete this file after completing the squash:
```bash
rm SQUASH_GUIDE.md
```
