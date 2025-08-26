package api

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"fmt"
	"net/http"
	"strings"
)

// Route validation error message constants for consistent error handling.
const (
	ErrEmptyPattern           = "route pattern cannot be empty"
	ErrWhitespacePattern      = "route pattern cannot be only whitespace"
	ErrInvalidPatternFormat   = "invalid route pattern format: must have format 'METHOD /path'"
	ErrEmptyMethod            = "HTTP method cannot be empty"
	ErrInvalidMethod          = "invalid HTTP method: must be one of GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS"
	ErrEmptyPath              = "empty path"
	ErrPathMustStartWithSlash = "path must start with '/'"
	ErrDoubleSlashes          = "path contains double slashes"
	ErrUnmatchedClosingBrace  = "unmatched closing brace"
)

// Error message templates for consistent formatting.
const (
	TmplPathMustStartWithSlash  = "path '%s' in pattern '%s' must start with '/'"
	TmplPathContainsDoubleSlash = "path '%s' in pattern '%s' contains double slashes"
	TmplUnmatchedClosingBrace   = "invalid parameter syntax in pattern '%s': unmatched closing brace at position %d"
	TmplInvalidMethod           = "ErrInvalidMethod: invalid HTTP method '%s' in pattern '%s': must be one of GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS"
)

// Parameter validation error codes for structured error reporting.
const (
	ParamMissingCloseBrace = "PARAM_MISSING_CLOSE_BRACE"
	ParamEmptyName         = "PARAM_EMPTY_NAME"
	ParamInvalidName       = "PARAM_INVALID_NAME"
	ParamDuplicateName     = "PARAM_DUPLICATE_NAME"
)

// Route conflict error types.
const (
	ConflictExactDuplicate = "ExactDuplicate"
	ConflictSameStructure  = "SameStructure"
)

// newParameterSyntaxError creates a structured parameter syntax error with consistent formatting
// This helper consolidates all parameter validation errors into a consistent format.
func newParameterSyntaxError(errorCode, pattern string, details ...interface{}) error {
	return fmt.Errorf(
		"newParameterSyntaxError: invalid parameter syntax %s in pattern '%s': %v",
		errorCode,
		pattern,
		details,
	)
}

// newRouteConflictError creates a structured route conflict error with consistent formatting
// This helper consolidates all route conflict errors into a consistent format for easier debugging.
func newRouteConflictError(conflictType, newPattern, existingPattern string) error {
	return fmt.Errorf(
		"newRouteConflictError: route conflict %s: pattern '%s' conflicts with existing pattern '%s'",
		conflictType,
		newPattern,
		existingPattern,
	)
}

// validHTTPMethods is the package-level variable containing valid HTTP methods for route pattern validation.
// This variable eliminates map allocation on every validatePattern call.
var validHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// isValidHTTPMethod performs case-insensitive validation of HTTP methods without
// string allocation by using strings.EqualFold for comparison.
func isValidHTTPMethod(method string) bool {
	for validMethod := range validHTTPMethods {
		if strings.EqualFold(method, validMethod) {
			return true
		}
	}
	return false
}

// RouteRegistry manages HTTP route registration using Go 1.22+ ServeMux patterns.
type RouteRegistry struct {
	routes   map[string]http.Handler
	patterns []string
	mux      *http.ServeMux
}

// NewRouteRegistry creates a new RouteRegistry.
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{
		routes:   make(map[string]http.Handler),
		patterns: make([]string, 0),
		mux:      http.NewServeMux(),
	}
}

// RegisterAPIRoutes registers all API routes with their handlers.
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

// RegisterRoute registers a single route with the given pattern and handler.
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

// BuildServeMux returns the configured ServeMux.
func (r *RouteRegistry) BuildServeMux() *http.ServeMux {
	return r.mux
}

// HasRoute checks if a route pattern is registered.
func (r *RouteRegistry) HasRoute(pattern string) bool {
	_, exists := r.routes[pattern]
	return exists
}

// RouteCount returns the number of registered routes.
func (r *RouteRegistry) RouteCount() int {
	return len(r.routes)
}

// GetPatterns returns all registered route patterns.
func (r *RouteRegistry) GetPatterns() []string {
	return r.patterns
}

// trimSpaceIfNeeded efficiently trims whitespace only when necessary to avoid allocations.
func trimSpaceIfNeeded(s string) string {
	if len(s) == 0 {
		return s
	}
	// Check if trimming is actually needed to avoid unnecessary allocations
	if s[0] == ' ' || s[0] == '\t' || s[0] == '\r' || s[0] == '\n' ||
		s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r' || s[len(s)-1] == '\n' {
		return strings.TrimSpace(s)
	}
	return s
}

// parseMethodAndPath efficiently parses method and path without slice allocation.
func parseMethodAndPath(pattern string) (method, path string, err error) {
	spaceIndex := strings.Index(pattern, " ")
	if spaceIndex == -1 {
		return "", "", fmt.Errorf("ErrInvalidPatternFormat: %s", ErrInvalidPatternFormat)
	}

	method = trimSpaceIfNeeded(pattern[:spaceIndex])
	path = trimSpaceIfNeeded(pattern[spaceIndex+1:])
	return method, path, nil
}

// validatePattern validates a Go 1.22+ ServeMux pattern with detailed error messages.
func (r *RouteRegistry) validatePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("ErrEmptyPattern: %s", ErrEmptyPattern)
	}

	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("ErrWhitespacePattern: %s", ErrWhitespacePattern)
	}

	// Parse method and path without slice allocation
	method, path, err := parseMethodAndPath(pattern)
	if err != nil {
		return err
	}

	// Validate HTTP method
	if method == "" {
		return fmt.Errorf("ErrEmptyMethod: %s", ErrEmptyMethod)
	}

	if !isValidHTTPMethod(method) {
		return fmt.Errorf(TmplInvalidMethod, method, pattern)
	}

	// Validate path
	if path == "" {
		return fmt.Errorf("%s", ErrEmptyPath)
	}

	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf(TmplPathMustStartWithSlash, path, pattern)
	}

	// Check for invalid parameter syntax
	if strings.Contains(path, "{") {
		if err := r.validateParameterSyntax(path, pattern); err != nil {
			return err
		}
	}

	// Check for double slashes or other path issues
	if strings.Contains(path, "//") {
		return fmt.Errorf(TmplPathContainsDoubleSlash, path, pattern)
	}

	return nil
}

// containsString checks if a slice contains a string without allocation.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// validateParameterSyntax validates Go 1.22+ path parameter syntax with detailed error messages.
// Optimized to use slice instead of map to reduce allocations.
func (r *RouteRegistry) validateParameterSyntax(path, pattern string) error {
	var paramNames []string // Only allocate when first parameter is found
	braceCount := 0

	// Find all parameters and validate syntax
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '{':
			braceCount++

			// Find matching closing brace
			closing := strings.Index(path[i+1:], "}")
			if closing == -1 {
				return newParameterSyntaxError(
					ParamMissingCloseBrace,
					pattern,
					fmt.Sprintf("missing closing brace for parameter starting at position %d", i),
				)
			}

			paramName := path[i+1 : i+1+closing]
			if paramName == "" {
				return newParameterSyntaxError(
					ParamEmptyName,
					pattern,
					fmt.Sprintf("empty parameter name at position %d", i),
				)
			}

			// Validate parameter name (should be alphanumeric and underscores)
			if !isValidParameterName(paramName) {
				return newParameterSyntaxError(
					ParamInvalidName,
					pattern,
					fmt.Sprintf("parameter name '%s' must contain only letters, numbers, and underscores", paramName),
				)
			}

			// Check for duplicate parameter names using slice scan
			if containsString(paramNames, paramName) {
				return newParameterSyntaxError(
					ParamDuplicateName,
					pattern,
					fmt.Sprintf("duplicate parameter name '%s'", paramName),
				)
			}

			// Lazy allocation: only allocate slice when first parameter is found
			if paramNames == nil {
				paramNames = make([]string, 0, 4) //nolint:mnd // typical API path parameter count
			}
			paramNames = append(paramNames, paramName)

			i += closing + 1
		case '}':
			// Unmatched closing brace
			return fmt.Errorf(TmplUnmatchedClosingBrace, pattern, i)
		}
	}

	return nil
}

// isValidParameterName checks if a parameter name is valid (letters, numbers, underscores only).
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

// checkRouteConflict checks for conflicting route patterns with detailed error messages.
func (r *RouteRegistry) checkRouteConflict(newPattern string) error {
	// Parse the new pattern without allocation
	newMethod, newPath, err := parseMethodAndPath(newPattern)
	if err != nil {
		return err // Return the actual error
	}

	// Check against all existing patterns
	for _, existingPattern := range r.patterns {
		existingMethod, existingPath, err := parseMethodAndPath(existingPattern)
		if err != nil {
			continue // Skip malformed patterns
		}

		// Same method, check for path conflicts
		if strings.EqualFold(newMethod, existingMethod) {
			if conflictType := r.getPathConflictType(newPath, existingPath); conflictType != "" {
				return newRouteConflictError(conflictType, newPattern, existingPattern)
			}
		}
	}

	return nil
}

// getPathConflictType analyzes the type of conflict between two paths.
func (r *RouteRegistry) getPathConflictType(path1, path2 string) string {
	if path1 == path2 {
		return ConflictExactDuplicate
	}

	// Normalize paths by replacing parameters with placeholders
	normalized1 := r.normalizePath(path1)
	normalized2 := r.normalizePath(path2)

	if normalized1 == normalized2 {
		return ConflictSameStructure
	}

	return "" // No conflict
}

// normalizePath replaces all parameters with a standard placeholder.
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

// ExtractPathParams extracts path parameters from an HTTP request using Go 1.22+ ServeMux.
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
