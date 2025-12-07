// Package main provides the entry point for the standalone CodeChunking CLI client.
// This binary can be distributed independently without tree-sitter dependencies
// since it's built with CGO_ENABLED=0.
//
// Usage:
//
//	codechunking-client health
//	codechunking-client repos list
//	codechunking-client repos get <uuid>
//	codechunking-client repos add <url> --wait
//	codechunking-client search "query" --limit 10
//	codechunking-client jobs get <repo-id> <job-id>
//
// Global flags:
//
//	--api-url    API server URL (default: http://localhost:8080)
//	--timeout    Request timeout duration (default: 30s)
//
// All output is JSON-formatted for consumption by AI agents and automation tools.
package main

import (
	"codechunking/internal/client/commands"
	"os"
)

func main() {
	if err := commands.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
