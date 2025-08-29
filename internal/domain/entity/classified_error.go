package entity

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ClassifiedError represents an error that has been classified and enriched with context.
type ClassifiedError struct {
	id            string
	originalError error
	severity      *valueobject.ErrorSeverity
	errorCode     string
	message       string
	context       map[string]interface{}
	timestamp     time.Time
	correlationID string
	component     string
	errorPattern  string
	stackTrace    string
}

var (
	// errorCodeRegex validates error code format - compiled once for performance.
	errorCodeRegex = regexp.MustCompile(`^[a-z0-9_]+$`)

	// idGenerationCounter for more efficient ID generation in high-concurrency scenarios.
	idGenerationCounter int64

	// stackTracePool reuses buffers to reduce memory allocations.
	stackTracePool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 2048) // Smaller initial buffer, expandable
		},
	}
)

// NewClassifiedError creates a new classified error with comprehensive validation and optimized resource usage.
func NewClassifiedError(
	ctx context.Context,
	originalError error,
	severity *valueobject.ErrorSeverity,
	errorCode string,
	message string,
	errorContext map[string]interface{},
) (*ClassifiedError, error) {
	// Input validation with early returns for performance
	if originalError == nil {
		return nil, errors.New("classified_error: original error cannot be nil")
	}
	if severity == nil {
		return nil, errors.New("classified_error: severity cannot be nil")
	}
	if len(errorCode) == 0 {
		return nil, errors.New("classified_error: error code cannot be empty")
	}
	if len(message) == 0 {
		return nil, errors.New("classified_error: message cannot be empty")
	}
	if len(errorCode) > 100 { // Prevent excessively long error codes
		return nil, errors.New("classified_error: error code cannot exceed 100 characters")
	}
	if !errorCodeRegex.MatchString(errorCode) {
		return nil, errors.New(
			"classified_error: error code must contain only lowercase letters, numbers, and underscores",
		)
	}

	// Generate optimized ID
	id := generateOptimizedID()

	// Extract context information efficiently
	correlationID := extractCorrelationIDOptimized(ctx)
	component := extractComponentOptimized(ctx)

	// Generate error pattern with string builder for efficiency
	var patternBuilder strings.Builder
	patternBuilder.Grow(len(errorCode) + len(severity.String()) + 1)
	patternBuilder.WriteString(errorCode)
	patternBuilder.WriteString(":")
	patternBuilder.WriteString(severity.String())
	errorPattern := patternBuilder.String()

	// Capture optimized stack trace
	stackTrace := captureOptimizedStackTrace()

	// Deep copy context to prevent external modifications
	copiedContext := copyContext(errorContext)

	return &ClassifiedError{
		id:            id,
		originalError: originalError,
		severity:      severity,
		errorCode:     errorCode,
		message:       message,
		context:       copiedContext,
		timestamp:     time.Now(),
		correlationID: correlationID,
		component:     component,
		errorPattern:  errorPattern,
		stackTrace:    stackTrace,
	}, nil
}

// ID returns the error ID.
func (ce *ClassifiedError) ID() string {
	return ce.id
}

// OriginalError returns the original error.
func (ce *ClassifiedError) OriginalError() error {
	return ce.originalError
}

// Severity returns the error severity.
func (ce *ClassifiedError) Severity() *valueobject.ErrorSeverity {
	return ce.severity
}

// ErrorCode returns the error code.
func (ce *ClassifiedError) ErrorCode() string {
	return ce.errorCode
}

// Message returns the error message.
func (ce *ClassifiedError) Message() string {
	return ce.message
}

// Context returns the error context.
func (ce *ClassifiedError) Context() map[string]interface{} {
	return ce.context
}

// Timestamp returns the error timestamp.
func (ce *ClassifiedError) Timestamp() time.Time {
	return ce.timestamp
}

// CorrelationID returns the correlation ID.
func (ce *ClassifiedError) CorrelationID() string {
	return ce.correlationID
}

// Component returns the component that generated the error.
func (ce *ClassifiedError) Component() string {
	return ce.component
}

// ErrorPattern returns the error pattern for aggregation.
func (ce *ClassifiedError) ErrorPattern() string {
	return ce.errorPattern
}

// StackTrace returns the stack trace.
func (ce *ClassifiedError) StackTrace() string {
	return ce.stackTrace
}

// RequiresImmediateAlert returns true if this error requires immediate alerting.
func (ce *ClassifiedError) RequiresImmediateAlert() bool {
	return ce.severity.RequiresImmediateAlert()
}

// MarshalJSON implements json.Marshaler.
func (ce *ClassifiedError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":             ce.id,
		"error_code":     ce.errorCode,
		"severity":       ce.severity.String(),
		"message":        ce.message,
		"timestamp":      ce.timestamp,
		"correlation_id": ce.correlationID,
		"component":      ce.component,
		"context":        ce.context,
		"error_pattern":  ce.errorPattern,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (ce *ClassifiedError) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Parse severity
	severityStr, ok := raw["severity"].(string)
	if !ok {
		return errors.New("missing or invalid severity")
	}
	severity, err := valueobject.NewErrorSeverity(severityStr)
	if err != nil {
		return err
	}

	// Parse timestamp
	timestampStr, ok := raw["timestamp"].(string)
	if !ok {
		return errors.New("missing or invalid timestamp")
	}
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return err
	}

	// Extract other fields
	ce.id = getStringFromMap(raw, "id")
	ce.errorCode = getStringFromMap(raw, "error_code")
	ce.severity = severity
	ce.message = getStringFromMap(raw, "message")
	ce.timestamp = timestamp
	ce.correlationID = getStringFromMap(raw, "correlation_id")
	ce.component = getStringFromMap(raw, "component")
	ce.errorPattern = getStringFromMap(raw, "error_pattern")

	// Handle context
	if contextData, ok := raw["context"]; ok {
		if contextMap, ok := contextData.(map[string]interface{}); ok {
			ce.context = contextMap
		}
	}

	return nil
}

// Configuration constants for production optimization.
const (
	idLength       = 16
	maxMessageSize = 10000 // 10KB limit for messages to prevent memory issues
	maxContextKeys = 50    // Limit context map size to prevent memory bloat
)

// generateOptimizedID generates a unique ID with better performance characteristics.
func generateOptimizedID() string {
	// Use atomic counter for better performance in high-concurrency scenarios
	count := atomic.AddInt64(&idGenerationCounter, 1)

	// Combine timestamp, counter, and random bytes for uniqueness
	bytes := make([]byte, 8) // Reduced size for better performance
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("%d_%d", time.Now().UnixNano(), count)
	}

	// Create ID with timestamp prefix for better chronological ordering
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%x_%d_%s", timestamp, count, hex.EncodeToString(bytes))
}

// Context key type to prevent collisions.
type contextKey int

const (
	correlationIDContextKey contextKey = iota
	componentContextKey
)

// extractCorrelationIDOptimized extracts correlation ID from context with optimized lookups.
func extractCorrelationIDOptimized(ctx context.Context) string {
	// Try typed key first (preferred approach)
	if corrID := ctx.Value(correlationIDContextKey); corrID != nil {
		if strID, ok := corrID.(string); ok && len(strID) > 0 {
			return strID
		}
	}

	// Try string key for backward compatibility
	if corrID := ctx.Value("correlation_id"); corrID != nil {
		if strID, ok := corrID.(string); ok && len(strID) > 0 {
			return strID
		}
	}

	// Try custom ContextKey type for additional compatibility
	type ContextKey string
	const correlationIDKey ContextKey = "correlation_id"
	if corrID := ctx.Value(correlationIDKey); corrID != nil {
		if strID, ok := corrID.(string); ok && len(strID) > 0 {
			return strID
		}
	}

	// Generate new correlation ID if not found
	return generateOptimizedID()
}

// extractComponentOptimized extracts component from context with optimized lookups.
func extractComponentOptimized(ctx context.Context) string {
	// Try typed key first (preferred approach)
	if comp := ctx.Value(componentContextKey); comp != nil {
		if strComp, ok := comp.(string); ok && len(strComp) > 0 {
			return strComp
		}
	}

	// Try string key for backward compatibility
	if comp := ctx.Value("component"); comp != nil {
		if strComp, ok := comp.(string); ok && len(strComp) > 0 {
			return strComp
		}
	}
	return ""
}

// captureOptimizedStackTrace captures stack trace with memory pooling and efficient filtering.
func captureOptimizedStackTrace() string {
	// Get buffer from pool to reduce allocations
	buf := stackTracePool.Get().([]byte)
	defer stackTracePool.Put(buf)

	// Capture stack trace with initial buffer size
	n := runtime.Stack(buf, false)

	// If buffer was too small, allocate a larger one
	if n == len(buf) {
		// Buffer was full, need larger buffer
		largeBuf := make([]byte, len(buf)*2)
		n = runtime.Stack(largeBuf, false)
		buf = largeBuf
	}

	stack := string(buf[:n])

	// Efficiently filter out internal calls using string builder
	lines := strings.Split(stack, "\n")
	var filteredBuilder strings.Builder
	filteredBuilder.Grow(len(stack) / 2) // Estimate filtered size

	skipInternal := true
	for _, line := range lines {
		// Skip internal calls related to error creation
		if skipInternal {
			if strings.Contains(line, "classified_error.go") ||
				strings.Contains(line, "NewClassifiedError") ||
				strings.Contains(line, "captureOptimizedStackTrace") {
				continue
			}
			skipInternal = false
		}

		if filteredBuilder.Len() > 0 {
			filteredBuilder.WriteString("\n")
		}
		filteredBuilder.WriteString(line)

		// Limit stack trace size to prevent memory issues
		if filteredBuilder.Len() > maxMessageSize {
			filteredBuilder.WriteString("\n... [stack trace truncated]")
			break
		}
	}

	return filteredBuilder.String()
}

// copyContext creates a deep copy of the context map to prevent external modifications.
func copyContext(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return nil
	}

	// Limit context size to prevent memory bloat
	if len(original) > maxContextKeys {
		copied := make(map[string]interface{}, maxContextKeys)
		count := 0
		for k, v := range original {
			if count >= maxContextKeys {
				break
			}
			copied[k] = v
			count++
		}
		return copied
	}

	copied := make(map[string]interface{}, len(original))
	for k, v := range original {
		copied[k] = v
	}
	return copied
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
