package parsererrors

import (
	"context"
	"fmt"
	"time"
)

// ErrorCategory represents the category of parser error.
type ErrorCategory string

const (
	// Syntax errors.
	ErrorCategorySyntax ErrorCategory = "syntax"

	// Encoding/format errors.
	ErrorCategoryEncoding ErrorCategory = "encoding"

	// Resource limit errors.
	ErrorCategoryResourceLimit ErrorCategory = "resource_limit"

	// Timeout errors.
	ErrorCategoryTimeout ErrorCategory = "timeout"

	// Tree-sitter specific errors.
	ErrorCategoryTreeSitter ErrorCategory = "tree_sitter"

	// Edge case errors.
	ErrorCategoryEdgeCase ErrorCategory = "edge_case"

	// Language-specific errors.
	ErrorCategoryLanguage ErrorCategory = "language"
)

// ErrorSeverity represents the severity level of an error.
type ErrorSeverity string

const (
	ErrorSeverityLow      ErrorSeverity = "low"
	ErrorSeverityMedium   ErrorSeverity = "medium"
	ErrorSeverityHigh     ErrorSeverity = "high"
	ErrorSeverityCritical ErrorSeverity = "critical"
)

// ParserError represents a structured parser error with rich context.
type ParserError struct {
	// Basic error information
	Message  string        `json:"message"`
	Category ErrorCategory `json:"category"`
	Severity ErrorSeverity `json:"severity"`

	// Contextual information
	Language     string `json:"language,omitempty"`
	Operation    string `json:"operation,omitempty"`
	SourceLength int    `json:"source_length,omitempty"`
	LineNumber   int    `json:"line_number,omitempty"`
	ColumnNumber int    `json:"column_number,omitempty"`
	NodeType     string `json:"node_type,omitempty"`

	// Error details
	Details     map[string]any `json:"details,omitempty"`
	Suggestions []string       `json:"suggestions,omitempty"`

	// Metadata
	Timestamp time.Time `json:"timestamp"`
	ErrorID   string    `json:"error_id"`

	// Underlying error
	Cause error `json:"-"`
}

// NewParserError creates a new parser error with the given category and message.
func NewParserError(category ErrorCategory, message string) *ParserError {
	return &ParserError{
		Message:   message,
		Category:  category,
		Severity:  ErrorSeverityMedium, // Default severity
		Timestamp: time.Now(),
		ErrorID:   generateErrorID(),
	}
}

// NewSyntaxError creates a new syntax error.
func NewSyntaxError(message string) *ParserError {
	return NewParserError(ErrorCategorySyntax, message).
		WithSeverity(ErrorSeverityHigh).
		WithSuggestion("Check syntax according to language specification")
}

// NewEncodingError creates a new encoding error.
func NewEncodingError(message string) *ParserError {
	return NewParserError(ErrorCategoryEncoding, message).
		WithSeverity(ErrorSeverityHigh).
		WithSuggestion("Ensure source is valid UTF-8 encoded text")
}

// NewResourceLimitError creates a new resource limit error.
func NewResourceLimitError(message string) *ParserError {
	return NewParserError(ErrorCategoryResourceLimit, message).
		WithSeverity(ErrorSeverityCritical).
		WithSuggestion("Reduce file size or complexity")
}

// NewTimeoutError creates a new timeout error.
func NewTimeoutError(message string, duration time.Duration) *ParserError {
	return NewParserError(ErrorCategoryTimeout, message).
		WithSeverity(ErrorSeverityHigh).
		WithDetails("timeout_duration", duration.String()).
		WithSuggestion("Optimize code structure or increase timeout limit")
}

// NewTreeSitterError creates a new tree-sitter specific error.
func NewTreeSitterError(message string) *ParserError {
	return NewParserError(ErrorCategoryTreeSitter, message).
		WithSeverity(ErrorSeverityCritical).
		WithSuggestion("Check tree-sitter grammar and parser state")
}

// NewEdgeCaseError creates a new edge case error.
func NewEdgeCaseError(message string) *ParserError {
	return NewParserError(ErrorCategoryEdgeCase, message).
		WithSeverity(ErrorSeverityMedium).
		WithSuggestion("Handle this edge case explicitly")
}

// NewLanguageError creates a new language-specific error.
func NewLanguageError(language, message string) *ParserError {
	return NewParserError(ErrorCategoryLanguage, message).
		WithLanguage(language).
		WithSeverity(ErrorSeverityHigh).
		WithSuggestion(fmt.Sprintf("Follow %s language conventions", language))
}

// Error implements the error interface.
func (e *ParserError) Error() string {
	if e.Language != "" && e.Operation != "" {
		return fmt.Sprintf("%s error in %s %s: %s", e.Category, e.Language, e.Operation, e.Message)
	}
	if e.Language != "" {
		return fmt.Sprintf("%s error in %s: %s", e.Category, e.Language, e.Message)
	}
	return fmt.Sprintf("%s error: %s", e.Category, e.Message)
}

// Unwrap returns the underlying error for error chain unwrapping.
func (e *ParserError) Unwrap() error {
	return e.Cause
}

// WithCause adds a cause to the error.
func (e *ParserError) WithCause(cause error) *ParserError {
	e.Cause = cause
	return e
}

// WithDetails adds details to the error.
func (e *ParserError) WithDetails(key string, value any) *ParserError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// WithSuggestion adds a suggestion to the error.
func (e *ParserError) WithSuggestion(suggestion string) *ParserError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// WithLocation adds location information to the error.
func (e *ParserError) WithLocation(line, column int) *ParserError {
	e.LineNumber = line
	e.ColumnNumber = column
	return e
}

// WithSeverity sets the error severity.
func (e *ParserError) WithSeverity(severity ErrorSeverity) *ParserError {
	e.Severity = severity
	return e
}

// WithLanguage sets the language context.
func (e *ParserError) WithLanguage(language string) *ParserError {
	e.Language = language
	return e
}

// WithOperation sets the operation context.
func (e *ParserError) WithOperation(operation string) *ParserError {
	e.Operation = operation
	return e
}

// WithSourceLength sets the source length context.
func (e *ParserError) WithSourceLength(length int) *ParserError {
	e.SourceLength = length
	return e
}

// WithNodeType sets the node type context.
func (e *ParserError) WithNodeType(nodeType string) *ParserError {
	e.NodeType = nodeType
	return e
}

// IsTimeout checks if the error is a timeout error.
func (e *ParserError) IsTimeout() bool {
	return e.Category == ErrorCategoryTimeout
}

// IsResourceLimit checks if the error is a resource limit error.
func (e *ParserError) IsResourceLimit() bool {
	return e.Category == ErrorCategoryResourceLimit
}

// IsSyntax checks if the error is a syntax error.
func (e *ParserError) IsSyntax() bool {
	return e.Category == ErrorCategorySyntax
}

// IsTreeSitter checks if the error is a tree-sitter error.
func (e *ParserError) IsTreeSitter() bool {
	return e.Category == ErrorCategoryTreeSitter
}

// IsCritical checks if the error is critical severity.
func (e *ParserError) IsCritical() bool {
	return e.Severity == ErrorSeverityCritical
}

// IsRecoverable checks if the error might be recoverable.
func (e *ParserError) IsRecoverable() bool {
	return e.Severity == ErrorSeverityLow || e.Severity == ErrorSeverityMedium
}

// TimeoutFromContext creates a timeout error if the context is done.
func TimeoutFromContext(ctx context.Context, operation string) *ParserError {
	if ctx.Err() == context.DeadlineExceeded {
		return NewTimeoutError(
			fmt.Sprintf("operation timeout: %s exceeded maximum allowed time", operation),
			0, // Duration unknown from context
		).WithOperation(operation)
	}
	if ctx.Err() == context.Canceled {
		return NewParserError(ErrorCategoryTimeout, fmt.Sprintf("operation canceled: %s", operation)).
			WithOperation(operation).
			WithSeverity(ErrorSeverityMedium)
	}
	return nil
}

// generateErrorID generates a unique error ID for tracking.
func generateErrorID() string {
	// Simple timestamp-based ID for now
	// In production, this could be more sophisticated (UUID, etc.)
	return fmt.Sprintf("PE_%d", time.Now().UnixNano())
}
