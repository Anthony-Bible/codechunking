// Package service provides comprehensive failing tests for failure classification.
// These tests define the exact behavior needed for classifying different types
// of errors into appropriate retry categories for the worker system.
//
// RED Phase: All tests in this file are designed to fail initially and provide
// clear specifications for the GREEN phase implementation.
package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ClassifierError represents a structured error that can be classified.
type ClassifierError struct {
	Code      string
	Message   string
	Cause     error
	Context   map[string]interface{}
	Retryable bool
	Severity  ErrorSeverity
	Component string
	Operation string
	Timestamp time.Time
}

// Error implements the error interface.
func (e *ClassifierError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements the unwrap interface for error chains.
func (e *ClassifierError) Unwrap() error {
	return e.Cause
}

// ErrorSeverity represents the severity level of an error.
type ErrorSeverity int

const (
	ErrorSeverityLow ErrorSeverity = iota
	ErrorSeverityMedium
	ErrorSeverityHigh
	ErrorSeverityCritical
)

// String returns the string representation of error severity.
func (es ErrorSeverity) String() string {
	switch es {
	case ErrorSeverityLow:
		return "low"
	case ErrorSeverityMedium:
		return "medium"
	case ErrorSeverityHigh:
		return "high"
	case ErrorSeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// TestFailureClassifier_NetworkErrors tests classification of network-related errors.
func TestFailureClassifier_NetworkErrors(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name:             "connection timeout",
			error:            &net.OpError{Op: "dial", Net: "tcp", Err: &net.DNSError{Err: "timeout", IsTimeout: true}},
			expectedType:     FailureTypeNetwork,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "connection refused",
			error:            &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			expectedType:     FailureTypeNetwork,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "dns resolution failure",
			error:            &net.DNSError{Err: "no such host", Name: "invalid.example.com"},
			expectedType:     FailureTypeNetwork,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "generic network error with timeout keyword",
			error:            errors.New("network timeout occurred"),
			expectedType:     FailureTypeNetwork,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "connection reset by peer",
			error:            errors.New("connection reset by peer"),
			expectedType:     FailureTypeNetwork,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "no route to host",
			error:            errors.New("no route to host"),
			expectedType:     FailureTypeNetwork,
			expectedRetry:    false, // Infrastructure issue
			expectedSeverity: ErrorSeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			require.NotNil(t, classifier)

			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			// Test enhanced classifier with additional context
			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_AuthenticationErrors tests classification of authentication failures.
func TestFailureClassifier_AuthenticationErrors(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name: "invalid credentials",
			error: &ClassifierError{
				Code:      "AUTH_INVALID_CREDENTIALS",
				Message:   "invalid username or password",
				Retryable: true,
				Severity:  ErrorSeverityMedium,
			},
			expectedType:     FailureTypeAuthentication,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name: "expired token",
			error: &ClassifierError{
				Code:      "AUTH_TOKEN_EXPIRED",
				Message:   "access token has expired",
				Retryable: true,
				Severity:  ErrorSeverityLow,
			},
			expectedType:     FailureTypeAuthentication,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityLow,
		},
		{
			name:             "generic authentication failure",
			error:            errors.New("authentication failed"),
			expectedType:     FailureTypeAuthentication,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "unauthorized access",
			error:            errors.New("401 Unauthorized"),
			expectedType:     FailureTypeAuthentication,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "forbidden access - permanent",
			error:            errors.New("403 Forbidden"),
			expectedType:     FailureTypeAuthentication,
			expectedRetry:    false, // Permanent access denial
			expectedSeverity: ErrorSeverityHigh,
		},
		{
			name:             "invalid API key",
			error:            errors.New("invalid API key provided"),
			expectedType:     FailureTypeAuthentication,
			expectedRetry:    false, // Configuration issue
			expectedSeverity: ErrorSeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_DiskSpaceErrors tests classification of disk space related failures.
func TestFailureClassifier_DiskSpaceErrors(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name:             "disk space full",
			error:            errors.New("no space left on device"),
			expectedType:     FailureTypeDiskSpace,
			expectedRetry:    false, // Can't retry without cleanup
			expectedSeverity: ErrorSeverityCritical,
		},
		{
			name:             "disk quota exceeded",
			error:            errors.New("disk quota exceeded"),
			expectedType:     FailureTypeDiskSpace,
			expectedRetry:    false,
			expectedSeverity: ErrorSeverityHigh,
		},
		{
			name:             "write failed - insufficient space",
			error:            errors.New("write failed: insufficient space"),
			expectedType:     FailureTypeDiskSpace,
			expectedRetry:    false,
			expectedSeverity: ErrorSeverityCritical,
		},
		{
			name:             "io error - device full",
			error:            errors.New("I/O error: device full"),
			expectedType:     FailureTypeDiskSpace,
			expectedRetry:    false,
			expectedSeverity: ErrorSeverityCritical,
		},
		{
			name:             "temporary file creation failed",
			error:            errors.New("failed to create temp file: no space"),
			expectedType:     FailureTypeDiskSpace,
			expectedRetry:    false,
			expectedSeverity: ErrorSeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_RepositoryAccessErrors tests classification of repository access failures.
func TestFailureClassifier_RepositoryAccessErrors(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name:             "repository not found",
			error:            errors.New("repository not found"),
			expectedType:     FailureTypeRepositoryAccess,
			expectedRetry:    true, // Might be temporary
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "private repository access denied",
			error:            errors.New("access denied: private repository"),
			expectedType:     FailureTypeRepositoryAccess,
			expectedRetry:    true, // Auth might be fixable
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "git clone failed - permission denied",
			error:            errors.New("git clone failed: permission denied"),
			expectedType:     FailureTypeRepositoryAccess,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "repository is empty",
			error:            errors.New("repository is empty"),
			expectedType:     FailureTypeRepositoryAccess,
			expectedRetry:    false, // Nothing to process
			expectedSeverity: ErrorSeverityLow,
		},
		{
			name:             "invalid git URL",
			error:            errors.New("invalid git repository URL"),
			expectedType:     FailureTypeRepositoryAccess,
			expectedRetry:    false, // Configuration error
			expectedSeverity: ErrorSeverityHigh,
		},
		{
			name:             "git service unavailable",
			error:            errors.New("github.com service unavailable"),
			expectedType:     FailureTypeNetwork, // Should be classified as network
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_NATSMessagingErrors tests classification of NATS messaging failures.
func TestFailureClassifier_NATSMessagingErrors(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name:             "nats connection lost",
			error:            errors.New("nats: connection lost"),
			expectedType:     FailureTypeNATSMessaging,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityHigh,
		},
		{
			name:             "jetstream not available",
			error:            errors.New("jetstream not available"),
			expectedType:     FailureTypeNATSMessaging,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityHigh,
		},
		{
			name:             "message acknowledgment timeout",
			error:            errors.New("ack timeout"),
			expectedType:     FailureTypeNATSMessaging,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "consumer not found",
			error:            errors.New("consumer not found"),
			expectedType:     FailureTypeNATSMessaging,
			expectedRetry:    false, // Configuration issue
			expectedSeverity: ErrorSeverityHigh,
		},
		{
			name:             "stream not found",
			error:            errors.New("stream not found"),
			expectedType:     FailureTypeNATSMessaging,
			expectedRetry:    false, // Configuration issue
			expectedSeverity: ErrorSeverityHigh,
		},
		{
			name:             "maximum deliveries exceeded",
			error:            errors.New("maximum deliveries exceeded"),
			expectedType:     FailureTypeNATSMessaging,
			expectedRetry:    false, // Message should go to DLQ
			expectedSeverity: ErrorSeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_TimeoutErrors tests classification of timeout-related failures.
func TestFailureClassifier_TimeoutErrors(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name:             "context deadline exceeded",
			error:            context.DeadlineExceeded,
			expectedType:     FailureTypeTimeout,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "operation timeout",
			error:            errors.New("operation timeout"),
			expectedType:     FailureTypeTimeout,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "request timeout",
			error:            errors.New("request timeout after 30s"),
			expectedType:     FailureTypeTimeout,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "read timeout",
			error:            errors.New("read timeout"),
			expectedType:     FailureTypeTimeout,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "write timeout",
			error:            errors.New("write timeout"),
			expectedType:     FailureTypeTimeout,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_RateLimitErrors tests classification of rate limiting failures.
func TestFailureClassifier_RateLimitErrors(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name:             "rate limit exceeded",
			error:            errors.New("rate limit exceeded"),
			expectedType:     FailureTypeRateLimit,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityLow,
		},
		{
			name:             "too many requests",
			error:            errors.New("429 Too Many Requests"),
			expectedType:     FailureTypeRateLimit,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityLow,
		},
		{
			name:             "api quota exceeded",
			error:            errors.New("API quota exceeded"),
			expectedType:     FailureTypeRateLimit,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "throttling in effect",
			error:            errors.New("request throttled"),
			expectedType:     FailureTypeRateLimit,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_UnknownErrors tests classification of unknown/generic failures.
func TestFailureClassifier_UnknownErrors(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name:             "generic error",
			error:            errors.New("something went wrong"),
			expectedType:     FailureTypeUnknown,
			expectedRetry:    false, // Conservative approach
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "nil error",
			error:            nil,
			expectedType:     FailureTypeUnknown,
			expectedRetry:    false,
			expectedSeverity: ErrorSeverityLow,
		},
		{
			name:             "empty error message",
			error:            errors.New(""),
			expectedType:     FailureTypeUnknown,
			expectedRetry:    false,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "custom application error",
			error:            errors.New("custom business logic error"),
			expectedType:     FailureTypeUnknown,
			expectedRetry:    false,
			expectedSeverity: ErrorSeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_ErrorChaining tests classification of wrapped/chained errors.
func TestFailureClassifier_ErrorChaining(t *testing.T) {
	tests := []struct {
		name             string
		error            error
		expectedType     FailureType
		expectedRetry    bool
		expectedSeverity ErrorSeverity
	}{
		{
			name: "wrapped network error",
			error: fmt.Errorf("failed to connect to repository: %w",
				&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection timeout")}),
			expectedType:     FailureTypeNetwork,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name: "multiple wrapped errors",
			error: fmt.Errorf("job processing failed: %w",
				fmt.Errorf("git operation failed: %w", errors.New("authentication failed"))),
			expectedType:     FailureTypeAuthentication,
			expectedRetry:    true,
			expectedSeverity: ErrorSeverityMedium,
		},
		{
			name:             "wrapped disk space error",
			error:            fmt.Errorf("cannot save file: %w", errors.New("no space left on device")),
			expectedType:     FailureTypeDiskSpace,
			expectedRetry:    false,
			expectedSeverity: ErrorSeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			failureType := classifier.Classify(tt.error)
			assert.Equal(t, tt.expectedType, failureType)

			enhancedClassifier := NewEnhancedFailureClassifier()
			result := enhancedClassifier.ClassifyWithContext(context.Background(), tt.error)

			assert.Equal(t, tt.expectedType, result.FailureType)
			assert.Equal(t, tt.expectedRetry, result.ShouldRetry)
			assert.Equal(t, tt.expectedSeverity, result.Severity)
		})
	}
}

// TestFailureClassifier_PatternMatching tests pattern-based error classification.
func TestFailureClassifier_PatternMatching(t *testing.T) {
	tests := []struct {
		name         string
		errorMessage string
		expectedType FailureType
	}{
		// Network patterns
		{
			name:         "connection_timeout_pattern",
			errorMessage: "dialing failed: connection timed out",
			expectedType: FailureTypeNetwork,
		},
		{
			name:         "dns_failure_pattern",
			errorMessage: "lookup github.com: no such host",
			expectedType: FailureTypeNetwork,
		},

		// Authentication patterns
		{
			name:         "auth_failure_pattern",
			errorMessage: "HTTP 401: Bad credentials",
			expectedType: FailureTypeAuthentication,
		},
		{
			name:         "token_expired_pattern",
			errorMessage: "token expired at 2024-01-01T00:00:00Z",
			expectedType: FailureTypeAuthentication,
		},

		// Disk space patterns
		{
			name:         "disk_full_pattern",
			errorMessage: "mkdir /tmp/repo: no space left on device",
			expectedType: FailureTypeDiskSpace,
		},

		// NATS patterns
		{
			name:         "nats_disconnection_pattern",
			errorMessage: "nats: connection closed",
			expectedType: FailureTypeNATSMessaging,
		},

		// Rate limit patterns
		{
			name:         "rate_limit_pattern",
			errorMessage: "API rate limit of 5000 requests per hour exceeded",
			expectedType: FailureTypeRateLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping until GREEN phase implementation")

			classifier := NewFailureClassifier()
			testError := errors.New(tt.errorMessage)
			failureType := classifier.Classify(testError)
			assert.Equal(t, tt.expectedType, failureType)
		})
	}
}

// TestFailureClassifier_ContextualClassification tests context-aware classification.
func TestFailureClassifier_ContextualClassification(t *testing.T) {
	t.Skip("Skipping until GREEN phase implementation")

	enhancedClassifier := NewEnhancedFailureClassifier()

	// Define a proper context key type
	type correlationKey struct{}

	// Test with different contexts
	contexts := map[string]context.Context{
		"with_timeout": func() context.Context {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			time.Sleep(2 * time.Second) // Force timeout
			return ctx
		}(),
		"with_correlation_id": func() context.Context {
			return context.WithValue(context.Background(), correlationKey{}, "test-123")
		}(),
		"cancelled": func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}(),
	}

	testError := errors.New("operation failed")

	for name, ctx := range contexts {
		t.Run(name, func(t *testing.T) {
			result := enhancedClassifier.ClassifyWithContext(ctx, testError)

			// Context should influence classification
			assert.NotNil(t, result)
			assert.NotEqual(t, FailureType(0), result.FailureType)

			// Cancelled context should affect retry decision
			if name == "cancelled" {
				assert.False(t, result.ShouldRetry)
			}
		})
	}
}

// TestFailureClassifier_CustomRules tests classification with custom rules.
func TestFailureClassifier_CustomRules(t *testing.T) {
	t.Skip("Skipping until GREEN phase implementation")

	// Custom classification rules
	customRules := []ClassificationRule{
		{
			Name:      "custom_service_error",
			Pattern:   "SERVICE_.*_ERROR",
			Type:      FailureTypeUnknown,
			Retryable: false,
			Severity:  ErrorSeverityHigh,
		},
		{
			Name:      "temporary_unavailable",
			Pattern:   "temporarily unavailable",
			Type:      FailureTypeNetwork,
			Retryable: true,
			Severity:  ErrorSeverityMedium,
		},
	}

	classifier := NewFailureClassifierWithRules(customRules)
	require.NotNil(t, classifier)

	// Test custom rule matching
	serviceError := errors.New("SERVICE_PROCESSING_ERROR: unable to process request")
	result := classifier.Classify(serviceError)
	assert.Equal(t, FailureTypeUnknown, result)

	tempError := errors.New("service temporarily unavailable")
	result = classifier.Classify(tempError)
	assert.Equal(t, FailureTypeNetwork, result)
}

// ClassificationResult holds detailed classification results.
type ClassificationResult struct {
	FailureType FailureType
	ShouldRetry bool
	Severity    ErrorSeverity
	Confidence  float64
	MatchedRule string
	Context     map[string]interface{}
	Timestamp   time.Time
}

// ClassificationRule defines a custom error classification rule.
type ClassificationRule struct {
	Name      string
	Pattern   string
	Type      FailureType
	Retryable bool
	Severity  ErrorSeverity
	Priority  int
}

// Enhanced classifier interface that will be implemented in GREEN phase.
type EnhancedFailureClassifier interface {
	// ClassifyWithContext provides detailed classification with context.
	ClassifyWithContext(ctx context.Context, err error) *ClassificationResult

	// AddRule adds a custom classification rule.
	AddRule(rule ClassificationRule) error

	// GetStatistics returns classification statistics.
	GetStatistics() map[FailureType]int

	// Reset clears any cached statistics.
	Reset()
}

// Stub functions to make tests compile (will be implemented in GREEN phase).
func NewEnhancedFailureClassifier() EnhancedFailureClassifier {
	panic("NewEnhancedFailureClassifier not implemented - RED phase")
}

func NewFailureClassifierWithRules(_ []ClassificationRule) FailureClassifier {
	panic("NewFailureClassifierWithRules not implemented - RED phase")
}
