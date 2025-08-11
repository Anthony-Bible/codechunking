package api

import (
	"fmt"
	"net/http"
	"strings"

	"codechunking/internal/adapter/inbound/api/testutil"
)

// RouteRegistry manages HTTP route registration using Go 1.22+ ServeMux patterns
type RouteRegistry struct {
	routes   map[string]http.Handler
	patterns []string
	mux      *http.ServeMux
}

// NewRouteRegistry creates a new RouteRegistry
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{
		routes:   make(map[string]http.Handler),
		patterns: make([]string, 0),
		mux:      http.NewServeMux(),
	}
}

// RegisterAPIRoutes registers all API routes with their handlers
func (r *RouteRegistry) RegisterAPIRoutes(healthHandler *HealthHandler, repositoryHandler *RepositoryHandler) {
	// Health endpoint
	if err := r.RegisterRoute("GET /health", http.HandlerFunc(healthHandler.GetHealth)); err != nil {
		panic(fmt.Errorf("failed to register health route: %w", err))
	}

	// Repository endpoints
	if err := r.RegisterRoute("POST /repositories", http.HandlerFunc(repositoryHandler.CreateRepository)); err != nil {
		panic(fmt.Errorf("failed to register create repository route: %w", err))
	}
	if err := r.RegisterRoute("GET /repositories", http.HandlerFunc(repositoryHandler.ListRepositories)); err != nil {
		panic(fmt.Errorf("failed to register list repositories route: %w", err))
	}
	if err := r.RegisterRoute("GET /repositories/{id}", http.HandlerFunc(repositoryHandler.GetRepository)); err != nil {
		panic(fmt.Errorf("failed to register get repository route: %w", err))
	}
	if err := r.RegisterRoute("DELETE /repositories/{id}", http.HandlerFunc(repositoryHandler.DeleteRepository)); err != nil {
		panic(fmt.Errorf("failed to register delete repository route: %w", err))
	}

	// Repository job endpoints
	if err := r.RegisterRoute("GET /repositories/{id}/jobs", http.HandlerFunc(repositoryHandler.GetRepositoryJobs)); err != nil {
		panic(fmt.Errorf("failed to register repository jobs route: %w", err))
	}
	if err := r.RegisterRoute("GET /repositories/{id}/jobs/{job_id}", http.HandlerFunc(repositoryHandler.GetIndexingJob)); err != nil {
		panic(fmt.Errorf("failed to register indexing job route: %w", err))
	}
}

// RegisterRoute registers a single route with the given pattern and handler
func (r *RouteRegistry) RegisterRoute(pattern string, handler http.Handler) error {
	// Validate pattern
	if err := r.validatePattern(pattern); err != nil {
		return err
	}

	// Check for conflicts
	if err := r.checkRouteConflict(pattern); err != nil {
		return err
	}

	// Register with ServeMux
	r.mux.Handle(pattern, handler)

	// Store in registry
	r.routes[pattern] = handler
	r.patterns = append(r.patterns, pattern)

	return nil
}

// BuildServeMux returns the configured ServeMux
func (r *RouteRegistry) BuildServeMux() *http.ServeMux {
	return r.mux
}

// HasRoute checks if a route pattern is registered
func (r *RouteRegistry) HasRoute(pattern string) bool {
	_, exists := r.routes[pattern]
	return exists
}

// RouteCount returns the number of registered routes
func (r *RouteRegistry) RouteCount() int {
	return len(r.routes)
}

// GetPatterns returns all registered route patterns
func (r *RouteRegistry) GetPatterns() []string {
	return r.patterns
}

// validatePattern validates a Go 1.22+ ServeMux pattern with detailed error messages
func (r *RouteRegistry) validatePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("route pattern cannot be empty")
	}

	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("route pattern cannot be only whitespace")
	}

	// Split method and path
	parts := strings.SplitN(pattern, " ", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid route pattern '%s': must have format 'METHOD /path' (e.g., 'GET /users')", pattern)
	}

	method, path := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

	// Validate HTTP method
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}

	if method == "" {
		return fmt.Errorf("HTTP method cannot be empty in pattern '%s'", pattern)
	}

	if !validMethods[strings.ToUpper(method)] {
		return fmt.Errorf("invalid HTTP method '%s' in pattern '%s': must be one of GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS", method, pattern)
	}

	// Validate path
	if path == "" {
		return fmt.Errorf("empty path")
	}

	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path '%s' in pattern '%s' must start with '/'", path, pattern)
	}

	// Check for invalid parameter syntax
	if strings.Contains(path, "{") {
		if err := r.validateParameterSyntax(path, pattern); err != nil {
			return err
		}
	}

	// Check for double slashes or other path issues
	if strings.Contains(path, "//") {
		return fmt.Errorf("path '%s' in pattern '%s' contains double slashes", path, pattern)
	}

	return nil
}

// validateParameterSyntax validates Go 1.22+ path parameter syntax with detailed error messages
func (r *RouteRegistry) validateParameterSyntax(path, pattern string) error {
	paramNames := make(map[string]bool)
	braceCount := 0

	// Find all parameters and validate syntax
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '{':
			braceCount++

			// Find matching closing brace
			closing := strings.Index(path[i+1:], "}")
			if closing == -1 {
				return fmt.Errorf("invalid parameter syntax in pattern '%s': missing closing brace for parameter starting at position %d", pattern, i)
			}

			paramName := path[i+1 : i+1+closing]
			if paramName == "" {
				return fmt.Errorf("invalid parameter syntax in pattern '%s': empty parameter name at position %d", pattern, i)
			}

			// Validate parameter name (should be alphanumeric and underscores)
			if !isValidParameterName(paramName) {
				return fmt.Errorf("invalid parameter name '%s' in pattern '%s': parameter names must contain only letters, numbers, and underscores", paramName, pattern)
			}

			// Check for duplicate parameter names
			if paramNames[paramName] {
				return fmt.Errorf("duplicate parameter name '%s' in pattern '%s'", paramName, pattern)
			}
			paramNames[paramName] = true

			i += closing + 1
		case '}':
			// Unmatched closing brace
			return fmt.Errorf("invalid parameter syntax in pattern '%s': unmatched closing brace at position %d", pattern, i)
		}
	}

	return nil
}

// isValidParameterName checks if a parameter name is valid (letters, numbers, underscores only)
func isValidParameterName(name string) bool {
	if name == "" {
		return false
	}

	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}

	return true
}

// checkRouteConflict checks for conflicting route patterns with detailed error messages
func (r *RouteRegistry) checkRouteConflict(newPattern string) error {
	// Parse the new pattern
	newParts := strings.SplitN(newPattern, " ", 2)
	if len(newParts) != 2 {
		return nil // Already handled by validation
	}

	newMethod, newPath := strings.TrimSpace(newParts[0]), strings.TrimSpace(newParts[1])

	// Check against all existing patterns
	for _, existingPattern := range r.patterns {
		existingParts := strings.SplitN(existingPattern, " ", 2)
		if len(existingParts) != 2 {
			continue // Skip malformed patterns
		}

		existingMethod, existingPath := strings.TrimSpace(existingParts[0]), strings.TrimSpace(existingParts[1])

		// Same method, check for path conflicts
		if strings.EqualFold(newMethod, existingMethod) {
			if conflictType := r.getPathConflictType(newPath, existingPath); conflictType != "" {
				return fmt.Errorf("route conflict detected: pattern '%s' conflicts with existing pattern '%s' (%s)",
					newPattern, existingPattern, conflictType)
			}
		}
	}

	return nil
}

// getPathConflictType analyzes the type of conflict between two paths
func (r *RouteRegistry) getPathConflictType(path1, path2 string) string {
	if path1 == path2 {
		return "exact duplicate"
	}

	// Normalize paths by replacing parameters with placeholders
	normalized1 := r.normalizePath(path1)
	normalized2 := r.normalizePath(path2)

	if normalized1 == normalized2 {
		return "same structure with different parameter names"
	}

	return "" // No conflict
}

// normalizePath replaces all parameters with a standard placeholder
func (r *RouteRegistry) normalizePath(path string) string {
	result := path
	searchPos := 0
	for {
		start := strings.Index(result[searchPos:], "{")
		if start == -1 {
			break
		}
		start = searchPos + start
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		// Fix: end is relative to start, so add start to get absolute position
		end = start + end
		result = result[:start] + "{param}" + result[end+1:]
		// Continue searching after the replacement
		searchPos = start + len("{param}")
	}
	return result
}

// ExtractPathParams extracts path parameters from an HTTP request using Go 1.22+ ServeMux
func ExtractPathParams(r *http.Request) map[string]string {
	params := make(map[string]string)

	// Use Go 1.22+ path value extraction
	if id := r.PathValue("id"); id != "" {
		params["id"] = id
	}
	if jobID := r.PathValue("job_id"); jobID != "" {
		params["job_id"] = jobID
	}

	// For testing - use testutil helper if normal extraction failed
	if len(params) == 0 {
		if id := testutil.GetPathParam(r, "id"); id != "" {
			params["id"] = id
		}
		if jobID := testutil.GetPathParam(r, "job_id"); jobID != "" {
			params["job_id"] = jobID
		}
	}

	return params
}
