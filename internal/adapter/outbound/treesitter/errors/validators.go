package parsererrors

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ValidationLimits contains configurable limits for validation.
type ValidationLimits struct {
	MaxFileSize       int           // Maximum file size in bytes
	MaxLineLength     int           // Maximum line length
	MaxNestingDepth   int           // Maximum nesting depth
	MaxConstructCount int           // Maximum number of constructs (functions, classes, etc.)
	TimeoutDuration   time.Duration // Maximum processing time
}

// DefaultValidationLimits returns sensible default limits.
func DefaultValidationLimits() *ValidationLimits {
	return &ValidationLimits{
		MaxFileSize:       50 * 1024 * 1024, // 50MB
		MaxLineLength:     10000,            // 10k characters per line
		MaxNestingDepth:   500,              // 500 levels deep
		MaxConstructCount: 25000,            // 25k functions/classes/etc.
		TimeoutDuration:   30 * time.Second, // 30 second timeout
	}
}

// SourceValidator provides comprehensive source code validation.
type SourceValidator struct {
	limits *ValidationLimits
}

// NewSourceValidator creates a new source validator with the given limits.
func NewSourceValidator(limits *ValidationLimits) *SourceValidator {
	if limits == nil {
		limits = DefaultValidationLimits()
	}
	return &SourceValidator{
		limits: limits,
	}
}

// ValidateSource performs comprehensive source code validation.
func (v *SourceValidator) ValidateSource(ctx context.Context, source []byte, language string) error {
	// Check context timeout first
	if err := v.checkTimeout(ctx); err != nil {
		return err
	}

	// Edge case validations
	if err := v.validateEdgeCases(source); err != nil {
		return err.WithLanguage(language)
	}

	// Encoding validation
	if err := v.validateEncoding(source); err != nil {
		return err.WithLanguage(language)
	}

	// Resource limits validation
	if err := v.validateResourceLimits(source); err != nil {
		return err.WithLanguage(language).WithSourceLength(len(source))
	}

	return nil
}

// checkTimeout checks if context has timed out.
func (v *SourceValidator) checkTimeout(ctx context.Context) error {
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return NewTimeoutError("operation timeout: parsing exceeded maximum allowed time", v.limits.TimeoutDuration)
		}
		return NewParserError(ErrorCategoryTimeout, "operation canceled")
	default:
		return nil
	}
}

// validateEdgeCases validates basic edge cases.
func (v *SourceValidator) validateEdgeCases(source []byte) *ParserError {
	if len(source) == 0 {
		return NewEdgeCaseError("empty source: no content to parse")
	}

	sourceStr := string(source)
	if len(strings.TrimSpace(sourceStr)) == 0 {
		return NewEdgeCaseError("empty source: only whitespace content")
	}

	return nil
}

// validateEncoding validates source encoding and character validity.
func (v *SourceValidator) validateEncoding(source []byte) *ParserError {
	// Check for null bytes
	for i, b := range source {
		if b == 0 {
			return NewEncodingError("invalid source: contains null bytes").
				WithDetails("byte_position", i).
				WithSuggestion("Remove null bytes from source code")
		}
	}

	// Check for UTF-8 validity
	if !utf8.Valid(source) {
		return NewEncodingError("invalid encoding: source contains non-UTF8 characters").
			WithSuggestion("Ensure source is saved in UTF-8 encoding")
	}

	// Check for BOM marker (handled gracefully)
	if len(source) >= 3 && source[0] == 0xEF && source[1] == 0xBB && source[2] == 0xBF {
		// BOM detected - this is handled but could be improved
		return NewEncodingError("encoding issue: unexpected BOM marker").
			WithSeverity(ErrorSeverityLow).
			WithSuggestion("Save file without BOM marker for better compatibility")
	}

	// Check for excessive binary-like content (heuristic)
	if err := v.validateBinaryContent(source); err != nil {
		return err
	}

	return nil
}

// validateBinaryContent checks if content appears to be binary.
func (v *SourceValidator) validateBinaryContent(source []byte) *ParserError {
	if len(source) == 0 {
		return nil
	}

	sourceStr := string(source)
	nonPrintableCount := 0

	for _, r := range sourceStr {
		// Count non-printable characters (excluding common whitespace)
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			nonPrintableCount++
		}
	}

	// If more than 10% non-printable, likely binary
	if nonPrintableCount > len(sourceStr)/10 {
		return NewEncodingError("invalid encoding: source contains non-UTF8 characters").
			WithDetails("non_printable_count", nonPrintableCount).
			WithDetails("total_chars", len(sourceStr)).
			WithSuggestion("Ensure source is text-based code, not binary data")
	}

	return nil
}

// validateResourceLimits validates memory and resource constraints.
func (v *SourceValidator) validateResourceLimits(source []byte) *ParserError {
	sourceStr := string(source)

	// Check file size limits
	if len(source) > v.limits.MaxFileSize {
		return NewResourceLimitError("memory limit exceeded: file too large to process safely").
			WithDetails("file_size", len(source)).
			WithDetails("max_size", v.limits.MaxFileSize).
			WithSuggestion("Split large files into smaller modules")
	}

	// Check line length limits
	lines := strings.Split(sourceStr, "\n")
	for i, line := range lines {
		if len(line) > v.limits.MaxLineLength {
			return NewResourceLimitError("line too long: exceeds maximum line length limit").
				WithLocation(i+1, 0).
				WithDetails("line_length", len(line)).
				WithDetails("max_length", v.limits.MaxLineLength).
				WithSuggestion("Break long lines into multiple lines")
		}
	}

	// Check nesting depth (simplified brace counting)
	if err := v.validateNestingDepth(sourceStr); err != nil {
		return err
	}

	return nil
}

// validateNestingDepth checks for excessive nesting using brace counting.
func (v *SourceValidator) validateNestingDepth(source string) *ParserError {
	maxDepth := 0
	currentDepth := 0

	// Count nesting using various bracket types
	for i, char := range source {
		switch char {
		case '{', '(', '[':
			currentDepth++
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		case '}', ')', ']':
			currentDepth--
		}

		// Early exit if we exceed limits
		if maxDepth > v.limits.MaxNestingDepth {
			return NewResourceLimitError("recursion limit exceeded: maximum nesting depth reached").
				WithDetails("max_depth", maxDepth).
				WithDetails("limit", v.limits.MaxNestingDepth).
				WithDetails("position", i).
				WithSuggestion("Reduce nesting depth by extracting functions or simplifying structure")
		}
	}

	return nil
}

// ValidateConstructCount validates the number of language constructs.
func (v *SourceValidator) ValidateConstructCount(source string, constructType string) *ParserError {
	// Simple counting based on keywords - this is a heuristic
	var count int

	switch constructType {
	case "functions":
		count = strings.Count(source, "func ") +
			strings.Count(source, "function ") +
			strings.Count(source, "def ")
	case "classes":
		count = strings.Count(source, "class ") +
			strings.Count(source, "struct ") +
			strings.Count(source, "interface ")
	case "variables":
		count = strings.Count(source, "var ") +
			strings.Count(source, "let ") +
			strings.Count(source, "const ")
	default:
		return nil // Unknown construct type
	}

	if count > v.limits.MaxConstructCount {
		return NewResourceLimitError("resource limit exceeded: too many constructs to process").
			WithDetails("construct_type", constructType).
			WithDetails("count", count).
			WithDetails("limit", v.limits.MaxConstructCount).
			WithSuggestion("Split large files with many constructs into smaller modules")
	}

	return nil
}

// LanguageSpecificValidator defines interface for language-specific validation.
type LanguageSpecificValidator interface {
	ValidateSyntax(source string) *ParserError
	ValidateLanguageFeatures(source string) *ParserError
	GetLanguageName() string
}

// ValidatorRegistry manages language-specific validators.
type ValidatorRegistry struct {
	validators map[string]LanguageSpecificValidator
	mu         sync.RWMutex
}

// NewValidatorRegistry creates a new validator registry.
func NewValidatorRegistry() *ValidatorRegistry {
	return &ValidatorRegistry{
		validators: make(map[string]LanguageSpecificValidator),
	}
}

// RegisterValidator registers a language-specific validator.
func (r *ValidatorRegistry) RegisterValidator(language string, validator LanguageSpecificValidator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validators[language] = validator
}

// GetValidator retrieves a language-specific validator.
func (r *ValidatorRegistry) GetValidator(language string) (LanguageSpecificValidator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	validator, exists := r.validators[language]
	return validator, exists
}

// ValidateWithLanguageSpecific performs language-specific validation.
func (r *ValidatorRegistry) ValidateWithLanguageSpecific(source string, language string) *ParserError {
	validator, exists := r.GetValidator(language)
	if !exists {
		return nil // No language-specific validator available
	}

	// Validate syntax
	if err := validator.ValidateSyntax(source); err != nil {
		return err.WithLanguage(language)
	}

	// Validate language-specific features
	if err := validator.ValidateLanguageFeatures(source); err != nil {
		return err.WithLanguage(language)
	}

	return nil
}

// DefaultValidatorRegistry provides a default validator registry instance.
func DefaultValidatorRegistry() *ValidatorRegistry {
	// Create a new registry each time to avoid global state
	return NewValidatorRegistry()
}

// ValidateSourceWithLanguage performs comprehensive validation including language-specific checks.
func ValidateSourceWithLanguage(
	ctx context.Context,
	source []byte,
	language string,
	limits *ValidationLimits,
	registry *ValidatorRegistry,
) *ParserError {
	// Create validator with limits
	validator := NewSourceValidator(limits)

	// Perform general validation
	if err := validator.ValidateSource(ctx, source, language); err != nil {
		var parserErr *ParserError
		if errors.As(err, &parserErr) {
			return parserErr
		}
		// If it's not a ParserError, wrap it
		return NewParserError(ErrorCategoryEdgeCase, err.Error()).WithLanguage(language)
	}

	// Perform language-specific validation if registry provided
	if registry != nil {
		if err := registry.ValidateWithLanguageSpecific(string(source), language); err != nil {
			return err
		}
	}

	return nil
}
