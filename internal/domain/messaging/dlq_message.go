// Package messaging provides comprehensive Dead Letter Queue (DLQ) domain types and operations
// for handling failed indexing job messages. This package implements enterprise-grade error
// classification, failure pattern analysis, and message transformation capabilities.
//
// Key features:
// - Rich failure type taxonomy with temporary/permanent classification
// - Comprehensive failure context capture for debugging and analytics
// - Performance-optimized classification algorithms
// - Statistical analysis and pattern recognition
// - Message transformation and retry logic
//
// The package is designed for high-throughput scenarios with minimal allocations
// and optimal string processing performance.
package messaging

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Domain-specific error types for enhanced error handling and debugging.

// DLQError represents a domain-specific error in DLQ operations.
type DLQError struct {
	Op      string // The operation that failed
	Code    string // Error code for programmatic handling
	Message string // Human-readable error message
	Err     error  // Underlying error, if any
}

// Error implements the error interface.
func (e *DLQError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("dlq %s: %s (%s): %v", e.Op, e.Message, e.Code, e.Err)
	}
	return fmt.Sprintf("dlq %s: %s (%s)", e.Op, e.Message, e.Code)
}

// Unwrap returns the underlying error for error unwrapping.
func (e *DLQError) Unwrap() error {
	return e.Err
}

// NewDLQError creates a new domain-specific DLQ error.
func NewDLQError(op, code, message string, err error) *DLQError {
	return &DLQError{
		Op:      op,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Common error codes for programmatic error handling.
const (
	ErrCodeValidationFailed   = "VALIDATION_FAILED"
	ErrCodeMessageTooLarge    = "MESSAGE_TOO_LARGE"
	ErrCodeInvalidFailureType = "INVALID_FAILURE_TYPE"
	ErrCodeInvalidTimeRange   = "INVALID_TIME_RANGE"
	ErrCodeStatsTooOld        = "STATS_TOO_OLD"
)

// FailureType represents a categorized type of failure that occurred during message processing.
// This enumeration supports both operational decisions (retry/discard) and analytical insights.
// Each failure type has well-defined semantics for temporary vs permanent failure classification.
type FailureType string

// FailureType constants define the taxonomy of failures with clear retry semantics.
// Temporary failures (network, timeout, resource) should be retried with backoff.
// Permanent failures (validation, permission, not found) should not be retried.
const (
	// Temporary failures - these should be retried with exponential backoff.
	FailureTypeNetworkError      FailureType = "NETWORK_ERROR"      // Network connectivity issues, DNS failures
	FailureTypeTimeoutError      FailureType = "TIMEOUT_ERROR"      // Request timeouts, deadline exceeded
	FailureTypeResourceExhausted FailureType = "RESOURCE_EXHAUSTED" // Memory, disk space, rate limits

	// Permanent failures - these should NOT be retried without intervention.
	FailureTypeValidationError    FailureType = "VALIDATION_ERROR"     // Invalid data, schema violations
	FailureTypeProcessingError    FailureType = "PROCESSING_ERROR"     // Business logic failures, data corruption
	FailureTypeSystemError        FailureType = "SYSTEM_ERROR"         // Internal system errors, bugs
	FailureTypePermissionDenied   FailureType = "PERMISSION_DENIED"    // Authentication, authorization failures
	FailureTypeRepositoryNotFound FailureType = "REPOSITORY_NOT_FOUND" // Repository does not exist
)

// NewFailureType creates a new FailureType with validation.
func NewFailureType(failureType string) (FailureType, error) {
	switch failureType {
	case "NETWORK_ERROR":
		return FailureTypeNetworkError, nil
	case "VALIDATION_ERROR":
		return FailureTypeValidationError, nil
	case "PROCESSING_ERROR":
		return FailureTypeProcessingError, nil
	case "SYSTEM_ERROR":
		return FailureTypeSystemError, nil
	case "TIMEOUT_ERROR":
		return FailureTypeTimeoutError, nil
	case "RESOURCE_EXHAUSTED":
		return FailureTypeResourceExhausted, nil
	case "PERMISSION_DENIED":
		return FailureTypePermissionDenied, nil
	case "REPOSITORY_NOT_FOUND":
		return FailureTypeRepositoryNotFound, nil
	default:
		return "", NewDLQError("NewFailureType", ErrCodeInvalidFailureType,
			fmt.Sprintf("unsupported failure type: %s", failureType), nil)
	}
}

// IsTemporary returns true if the failure type is temporary and can be retried.
func (f FailureType) IsTemporary() bool {
	switch f {
	case FailureTypeNetworkError, FailureTypeTimeoutError, FailureTypeResourceExhausted:
		return true
	case FailureTypeValidationError,
		FailureTypeProcessingError,
		FailureTypeSystemError,
		FailureTypePermissionDenied,
		FailureTypeRepositoryNotFound:
		return false
	default:
		return false
	}
}

// IsPermanent returns true if the failure type is permanent and should not be retried.
func (f FailureType) IsPermanent() bool {
	return !f.IsTemporary()
}

// FailureContext holds detailed context about a failure.
type FailureContext struct {
	ErrorMessage   string                 `json:"error_message"`
	StackTrace     string                 `json:"stack_trace,omitempty"`
	ErrorCode      string                 `json:"error_code,omitempty"`
	Component      string                 `json:"component"`
	Operation      string                 `json:"operation"`
	RequestID      string                 `json:"request_id,omitempty"`
	CorrelationID  string                 `json:"correlation_id,omitempty"`
	AdditionalInfo map[string]interface{} `json:"additional_info,omitempty"`
}

// Validate validates the failure context.
func (fc *FailureContext) Validate() error {
	if fc.ErrorMessage == "" {
		return errors.New("error_message is required")
	}
	if fc.Component == "" {
		return errors.New("component is required")
	}
	if fc.Operation == "" {
		return errors.New("operation is required")
	}
	return nil
}

// DLQMessage represents a message in the Dead Letter Queue.
type DLQMessage struct {
	DLQMessageID     string                     `json:"dlq_message_id"`
	OriginalMessage  EnhancedIndexingJobMessage `json:"original_message"`
	FailureType      FailureType                `json:"failure_type"`
	FailureContext   FailureContext             `json:"failure_context"`
	FirstFailedAt    time.Time                  `json:"first_failed_at"`
	LastFailedAt     time.Time                  `json:"last_failed_at"`
	TotalFailures    int                        `json:"total_failures"`
	LastRetryAttempt int                        `json:"last_retry_attempt"`
	DeadLetterReason string                     `json:"dead_letter_reason"`
	ProcessingStage  string                     `json:"processing_stage"`
}

// Validate validates the DLQ message.
func (dlq *DLQMessage) Validate() error {
	if dlq.DLQMessageID == "" {
		return errors.New("dlq_message_id is required")
	}

	if err := dlq.OriginalMessage.Validate(); err != nil {
		return fmt.Errorf("original message validation failed: %w", err)
	}

	if dlq.FailureType == "" {
		return errors.New("failure_type is required")
	}

	if dlq.TotalFailures < 0 {
		return errors.New("total_failures cannot be negative")
	}

	if !dlq.FirstFailedAt.IsZero() && !dlq.LastFailedAt.IsZero() && dlq.LastFailedAt.Before(dlq.FirstFailedAt) {
		return errors.New("last_failed_at cannot be before first_failed_at")
	}

	return nil
}

// IsRetryable returns true if the failure type allows retries.
func (dlq *DLQMessage) IsRetryable() bool {
	return dlq.FailureType.IsTemporary()
}

// FailureDuration calculates the duration between first and last failure.
func (dlq *DLQMessage) FailureDuration() time.Duration {
	return dlq.LastFailedAt.Sub(dlq.FirstFailedAt)
}

// CreateRetryMessage creates a retry message from the DLQ message.
func (dlq *DLQMessage) CreateRetryMessage() (EnhancedIndexingJobMessage, error) {
	if !dlq.IsRetryable() {
		return EnhancedIndexingJobMessage{}, fmt.Errorf("failure type is not retryable: %s", dlq.FailureType)
	}

	retryMessage := dlq.OriginalMessage
	retryMessage.MessageID = GenerateUniqueMessageID()
	retryMessage.RetryAttempt = 0
	retryMessage.Timestamp = time.Now()

	return retryMessage, nil
}

// FailurePattern represents a pattern of failures for analysis.
type FailurePattern struct {
	FailureType     FailureType `json:"failure_type"`
	Component       string      `json:"component"`
	ErrorPattern    string      `json:"error_pattern"`
	OccurrenceCount int         `json:"occurrence_count"`
	FirstSeen       time.Time   `json:"first_seen"`
	LastSeen        time.Time   `json:"last_seen"`
	AffectedRepos   []string    `json:"affected_repos"`
	Severity        string      `json:"severity"`
}

// Validate validates the failure pattern.
func (fp *FailurePattern) Validate() error {
	if fp.OccurrenceCount < 0 {
		return errors.New("occurrence_count cannot be negative")
	}

	validSeverities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}
	if fp.Severity != "" {
		valid := false
		for _, severity := range validSeverities {
			if fp.Severity == severity {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid severity: %s", fp.Severity)
		}
	}

	return nil
}

// IsCritical returns true if the pattern is critical or high severity.
func (fp *FailurePattern) IsCritical() bool {
	return fp.Severity == "HIGH" || fp.Severity == "CRITICAL"
}

// DLQStatistics holds statistics about messages in the DLQ.
type DLQStatistics struct {
	TotalMessages         int                 `json:"total_messages"`
	RetryableMessages     int                 `json:"retryable_messages"`
	PermanentFailures     int                 `json:"permanent_failures"`
	MessagesByFailureType map[FailureType]int `json:"messages_by_failure_type"`
	AverageTimeInDLQ      time.Duration       `json:"average_time_in_dlq"`
	OldestMessageAge      time.Duration       `json:"oldest_message_age"`
	MessagesLastHour      int                 `json:"messages_last_hour"`
	MessagesLastDay       int                 `json:"messages_last_day"`
	RetrySuccessRate      float64             `json:"retry_success_rate"`
	LastUpdated           time.Time           `json:"last_updated"`
}

// Validate validates the DLQ statistics.
func (stats *DLQStatistics) Validate() error {
	if stats.TotalMessages < 0 {
		return errors.New("total_messages cannot be negative")
	}

	if stats.RetrySuccessRate < 0 || stats.RetrySuccessRate > 1.0 {
		return errors.New("retry_success_rate must be between 0 and 1")
	}

	return nil
}

// CalculateTotalMessages calculates total messages from retryable and permanent.
func (stats *DLQStatistics) CalculateTotalMessages() int {
	return stats.RetryableMessages + stats.PermanentFailures
}

// CalculateRetryablePercentage calculates the percentage of retryable messages.
func (stats *DLQStatistics) CalculateRetryablePercentage() float64 {
	total := stats.CalculateTotalMessages()
	if total == 0 {
		return 0
	}
	return float64(stats.RetryableMessages) / float64(total)
}

// CalculatePermanentFailurePercentage calculates the percentage of permanent failures.
func (stats *DLQStatistics) CalculatePermanentFailurePercentage() float64 {
	total := stats.CalculateTotalMessages()
	if total == 0 {
		return 0
	}
	return float64(stats.PermanentFailures) / float64(total)
}

// GenerateDLQMessageID generates a unique DLQ message ID.
func GenerateDLQMessageID() string {
	return fmt.Sprintf("dlq-%d-%s", time.Now().UnixNano(), uuid.New().String()[:8])
}

// Performance-optimized failure classification patterns.
// Using a single-pass approach to minimize string operations and allocations.
var (
	// Network error patterns - prioritized as most common temporary failures.
	networkPatterns = []string{"connection", "dial tcp", "network"}

	// Timeout error patterns.
	timeoutPatterns = []string{"timeout", "deadline exceeded", "timed out"}

	// Validation error patterns.
	validationPatterns = []string{"invalid", "required field"}

	// Permission error patterns.
	permissionPatterns = []string{"permission denied", "access denied"}

	// Repository not found patterns.
	notFoundPatterns = []string{"repository not found", "404 not found"}

	// Resource exhaustion patterns.
	resourcePatterns = []string{"out of memory", "disk full"}
)

// ClassifyFailureFromError classifies failure type based on error message using
// performance-optimized pattern matching. This function is designed for high-throughput
// scenarios and minimizes allocations by performing case-insensitive matching efficiently.
//
// The classification follows a priority order optimized for common failure patterns:
// 1. Network errors (most common temporary failures)
// 2. Timeout errors (second most common temporary)
// 3. Validation errors (common permanent failures)
// 4. Permission errors
// 5. Repository not found errors
// 6. Resource exhaustion errors
// 7. Default to system error.
func ClassifyFailureFromError(errorMessage string) FailureType {
	if errorMessage == "" {
		return FailureTypeSystemError
	}

	// Convert to lower case once for all comparisons.
	errorLower := strings.ToLower(errorMessage)

	// Network errors - most common temporary failures, check first.
	if containsAnyPattern(errorLower, networkPatterns) {
		return FailureTypeNetworkError
	}

	// Timeout errors - second most common temporary failures.
	if containsAnyPattern(errorLower, timeoutPatterns) {
		return FailureTypeTimeoutError
	}

	// Validation errors - common permanent failures.
	if containsAnyPattern(errorLower, validationPatterns) {
		return FailureTypeValidationError
	}

	// Permission errors.
	if containsAnyPattern(errorLower, permissionPatterns) {
		return FailureTypePermissionDenied
	}

	// Repository not found.
	if containsAnyPattern(errorLower, notFoundPatterns) {
		return FailureTypeRepositoryNotFound
	}

	// Resource exhaustion.
	if containsAnyPattern(errorLower, resourcePatterns) {
		return FailureTypeResourceExhausted
	}

	// Default to system error for unclassified failures.
	return FailureTypeSystemError
}

// containsAnyPattern efficiently checks if the error message contains any of the given patterns.
// This helper function reduces code duplication and improves maintainability.
func containsAnyPattern(errorLower string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(errorLower, pattern) {
			return true
		}
	}
	return false
}

// TransformToDLQMessage transforms a regular message to DLQ format.
func TransformToDLQMessage(
	original EnhancedIndexingJobMessage,
	failureType FailureType,
	failureContext FailureContext,
	processingStage string,
) (DLQMessage, error) {
	now := time.Now()

	dlqMessage := DLQMessage{
		DLQMessageID:     GenerateDLQMessageID(),
		OriginalMessage:  original,
		FailureType:      failureType,
		FailureContext:   failureContext,
		FirstFailedAt:    now,
		LastFailedAt:     now,
		TotalFailures:    1,
		LastRetryAttempt: original.RetryAttempt,
		ProcessingStage:  processingStage,
	}

	if err := dlqMessage.Validate(); err != nil {
		return DLQMessage{}, fmt.Errorf("DLQ message validation failed: %w", err)
	}

	return dlqMessage, nil
}
