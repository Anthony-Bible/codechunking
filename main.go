// Package main serves as the entry point for the codechunking application.
// It provides a production-grade system for indexing code repositories,
// generating embeddings, and providing semantic code search capabilities.
//
//	@title			Code Chunking Repository Indexing API
//	@version		1.0.0
//	@description	API for submitting code repositories for indexing, chunking, and embedding generation. This service processes Git repositories, extracts code chunks using AST parsing, and generates vector embeddings for semantic search capabilities.
//	@contact.name	Code Chunking API Support
//	@contact.email	support@codechunking.dev
//	@license.name	MIT
//	@license.url	https://opensource.org/licenses/MIT
//	@host			localhost:8080
//	@BasePath		/api/v1
//
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
package main

import "codechunking/cmd"

func main() {
	cmd.Execute()
}
