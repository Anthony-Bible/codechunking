package common

import (
	"fmt"
	"strconv"
	"strings"

	"codechunking/internal/application/common/security"
	"github.com/google/uuid"
)

// ValidRepositoryStatuses defines all valid repository statuses
var ValidRepositoryStatuses = map[string]bool{
	"":           true, // Empty means no filter
	"pending":    true,
	"cloning":    true,
	"processing": true,
	"completed":  true,
	"failed":     true,
	"archived":   true,
}

// ValidJobStatuses defines all valid job statuses
var ValidJobStatuses = map[string]bool{
	"pending":   true,
	"running":   true,
	"completed": true,
	"failed":    true,
	"cancelled": true,
}

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// ValidateRepositoryURL validates that a repository URL is not empty
func ValidateRepositoryURL(url string) error {
	if strings.TrimSpace(url) == "" {
		return ValidationError{Field: "url", Message: "URL is required"}
	}
	return nil
}

// ValidateRepositoryName validates repository name constraints with security checks
func ValidateRepositoryName(name *string) error {
	if name == nil {
		return nil
	}

	// Length validation
	if len(*name) > 255 {
		return ValidationError{Field: "name", Message: "exceeds maximum length"}
	}

	// Use shared security validator for comprehensive checks
	validator := security.NewInjectionValidator(security.DefaultConfig())
	unicodeValidator := security.NewUnicodeValidator(security.DefaultConfig())

	// Security validation: XSS prevention
	if err := validator.ValidateXSSAttacks(*name); err != nil {
		return ValidationError{Field: "name", Message: "contains malicious content"}
	}

	// Security validation: SQL injection prevention
	if err := validator.ValidateSQLInjection(*name); err != nil {
		return ValidationError{Field: "name", Message: "contains malicious SQL"}
	}

	// Security validation: control characters
	if err := validator.ValidateControlCharacters(*name); err != nil {
		return ValidationError{Field: "name", Message: "contains control characters"}
	}

	// Security validation: suspicious unicode
	if err := unicodeValidator.ValidateUnicodeAttacks(*name); err != nil {
		return ValidationError{Field: "name", Message: "contains suspicious unicode"}
	}

	// Security validation: path traversal
	if err := validator.ValidatePathTraversal(*name); err != nil {
		return ValidationError{Field: "name", Message: "path traversal detected"}
	}

	return nil
}

// ValidateRepositoryStatus validates repository status with security checks
func ValidateRepositoryStatus(status string) error {
	// Security validation: SQL injection prevention using shared utilities
	validator := security.NewInjectionValidator(security.DefaultConfig())
	if err := validator.ValidateSQLInjection(status); err != nil {
		return ValidationError{Field: "status", Message: "contains malicious SQL"}
	}

	if !ValidRepositoryStatuses[status] {
		return ValidationError{Field: "status", Message: "invalid status"}
	}
	return nil
}

// ValidateJobStatus validates job status
func ValidateJobStatus(status string) error {
	if !ValidJobStatuses[status] {
		return ValidationError{Field: "status", Message: fmt.Sprintf("invalid status: %s", status)}
	}
	return nil
}

// ValidateUUID validates that a UUID is not nil/empty
func ValidateUUID(id uuid.UUID, fieldName string) error {
	if id == uuid.Nil {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("%s is required", fieldName)}
	}
	return nil
}

// ValidatePaginationLimit validates pagination limit constraints
func ValidatePaginationLimit(limit int, maxLimit int, fieldName string) error {
	if limit > maxLimit {
		return ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("limit exceeds maximum of %d", maxLimit),
		}
	}
	return nil
}

// Security validation functions required by tests

// ValidateSortParameter validates sort parameters for SQL injection
func ValidateSortParameter(sortParam string) error {
	// Use shared security validator for SQL injection check
	validator := security.NewInjectionValidator(security.DefaultConfig())
	if err := validator.ValidateSQLInjection(sortParam); err != nil {
		return ValidationError{Field: "sort", Message: "contains malicious SQL"}
	}

	// Check for additional SQL keywords that might not be caught by above
	sqlKeywords := []string{"where", "select", "or ", "and ", "union"}
	lowerParam := strings.ToLower(sortParam)
	for _, keyword := range sqlKeywords {
		if strings.Contains(lowerParam, keyword) {
			return ValidationError{Field: "sort", Message: "contains malicious SQL"}
		}
	}

	// Parse sort parameter format: field:direction
	if !strings.Contains(sortParam, ":") {
		return ValidationError{Field: "sort", Message: "invalid format"}
	}

	parts := strings.Split(sortParam, ":")
	if len(parts) != 2 || parts[1] == "" {
		return ValidationError{Field: "sort", Message: "invalid format"}
	}

	field, direction := parts[0], parts[1]

	// Validate field name
	validFields := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"name":       true,
	}

	if !validFields[field] {
		return ValidationError{Field: "sort", Message: "invalid field"}
	}

	// Validate direction
	if direction != "asc" && direction != "desc" {
		return ValidationError{Field: "sort", Message: "invalid sort direction"}
	}

	return nil
}

// ValidatePaginationLimitString validates pagination limit string for SQL injection
func ValidatePaginationLimitString(limitStr string) error {
	// Use shared security validator
	validator := security.NewInjectionValidator(security.DefaultConfig())
	if err := validator.ValidateSQLInjection(limitStr); err != nil {
		return ValidationError{Field: "limit", Message: "contains malicious SQL"}
	}

	_, err := strconv.Atoi(limitStr)
	if err != nil {
		return ValidationError{Field: "limit", Message: "invalid limit"}
	}

	return nil
}

// ValidateOffsetParameter validates offset parameters for SQL injection
func ValidateOffsetParameter(offsetStr string) error {
	// Use shared security validator
	validator := security.NewInjectionValidator(security.DefaultConfig())
	if err := validator.ValidateSQLInjection(offsetStr); err != nil {
		return ValidationError{Field: "offset", Message: "contains malicious SQL"}
	}

	_, err := strconv.Atoi(offsetStr)
	if err != nil {
		return ValidationError{Field: "offset", Message: "invalid offset"}
	}

	return nil
}

// ValidateQueryParameters validates all query parameters for security issues
func ValidateQueryParameters(params map[string]string) error {
	// Use shared security validator for SQL injection checks
	validator := security.NewInjectionValidator(security.DefaultConfig())

	for key, value := range params {
		if err := validator.ValidateSQLInjection(value); err != nil {
			return ValidationError{Field: key, Message: "contains malicious SQL"}
		}

		// Validate specific parameters
		switch key {
		case "sort":
			if err := ValidateSortParameter(value); err != nil {
				return err
			}
		case "status":
			if err := ValidateRepositoryStatus(value); err != nil {
				return err
			}
		case "limit":
			if err := ValidatePaginationLimitString(value); err != nil {
				return err
			}
		case "offset":
			if err := ValidateOffsetParameter(value); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateJSONField validates individual JSON fields for security issues
func ValidateJSONField(fieldName, value string) error {
	// Use shared security validators for comprehensive checks
	validator := security.NewInjectionValidator(security.DefaultConfig())
	unicodeValidator := security.NewUnicodeValidator(security.DefaultConfig())

	// Check for protocol attacks first (most specific)
	if err := validator.ValidateProtocolAttacks(value); err != nil {
		return ValidationError{Field: fieldName, Message: "malicious protocol detected"}
	}

	if err := validator.ValidateXSSAttacks(value); err != nil {
		return ValidationError{Field: fieldName, Message: "malicious content detected"}
	}

	if err := validator.ValidateSQLInjection(value); err != nil {
		return ValidationError{Field: fieldName, Message: "contains malicious SQL"}
	}

	if err := validator.ValidateControlCharacters(value); err != nil {
		return ValidationError{Field: fieldName, Message: "control characters detected"}
	}

	if err := unicodeValidator.ValidateUnicodeAttacks(value); err != nil {
		return ValidationError{Field: fieldName, Message: "suspicious unicode detected"}
	}

	if err := validator.ValidatePathTraversal(value); err != nil {
		return ValidationError{Field: fieldName, Message: "path traversal detected"}
	}

	return nil
}

// EnhancedValidator provides improved validation with configurable security policies
type EnhancedValidator struct {
	config             *security.Config
	injectionValidator *security.InjectionValidator
	unicodeValidator   *security.UnicodeValidator
}

// NewEnhancedValidator creates a new enhanced validator with custom configuration
func NewEnhancedValidator(config *security.Config) *EnhancedValidator {
	if config == nil {
		config = security.DefaultConfig()
	}
	return &EnhancedValidator{
		config:             config,
		injectionValidator: security.NewInjectionValidator(config),
		unicodeValidator:   security.NewUnicodeValidator(config),
	}
}

// ValidateAllSecurityThreats performs comprehensive security validation
func (ev *EnhancedValidator) ValidateAllSecurityThreats(fieldName, value string) error {
	// Run all security validations using shared utilities
	if err := ev.injectionValidator.ValidateAllInjections(value); err != nil {
		if secViol, ok := err.(*security.SecurityViolation); ok {
			return ValidationError{Field: fieldName, Message: secViol.Message}
		}
		return ValidationError{Field: fieldName, Message: err.Error()}
	}

	if err := ev.unicodeValidator.ValidateUnicodeAttacks(value); err != nil {
		if secViol, ok := err.(*security.SecurityViolation); ok {
			return ValidationError{Field: fieldName, Message: secViol.Message}
		}
		return ValidationError{Field: fieldName, Message: err.Error()}
	}

	return nil
}

// ValidateWithCustomRules validates input with custom security rules
func (ev *EnhancedValidator) ValidateWithCustomRules(fieldName, value string, rules []string) error {
	// First run standard security validation
	if err := ev.ValidateAllSecurityThreats(fieldName, value); err != nil {
		return err
	}

	// Then apply custom rules
	for _, rule := range rules {
		switch rule {
		case "no_special_chars":
			if strings.ContainsAny(value, "!@#$%^&*()+={}[]|\\:;\"'<>?,./") {
				return ValidationError{Field: fieldName, Message: "contains special characters"}
			}
		case "ascii_only":
			for _, r := range value {
				if r > 127 {
					return ValidationError{Field: fieldName, Message: "contains non-ASCII characters"}
				}
			}
		case "no_whitespace":
			if strings.ContainsAny(value, " \t\n\r") {
				return ValidationError{Field: fieldName, Message: "contains whitespace"}
			}
		}
	}

	return nil
}

// BackwardCompatibility functions - these maintain the original API while using shared utilities

// containsXSSContent maintains backward compatibility
func containsXSSContent(input string) bool {
	return security.ContainsXSSContent(input)
}

// containsSQLInjectionContent maintains backward compatibility
func containsSQLInjectionContent(input string) bool {
	return security.ContainsSQLInjectionContent(input)
}

// containsPathTraversal maintains backward compatibility
func containsPathTraversal(input string) bool {
	return security.ContainsPathTraversal(input)
}

// containsControlCharacters maintains backward compatibility
func containsControlCharacters(input string) bool {
	return security.ContainsControlCharacters(input)
}

// containsSuspiciousUnicode maintains backward compatibility
func containsSuspiciousUnicode(input string) bool {
	return security.ContainsSuspiciousUnicode(input)
}
