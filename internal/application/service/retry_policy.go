package service

import (
	"context"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
)

// RetryPolicy defines the interface for retry decision making.
type RetryPolicy interface {
	// ShouldRetry determines if an operation should be retried based on the error and attempt number.
	ShouldRetry(ctx context.Context, err error, attempt int) (bool, time.Duration)

	// GetMaxAttempts returns the maximum number of retry attempts allowed.
	GetMaxAttempts() int

	// GetPolicyName returns a human-readable name for the policy.
	GetPolicyName() string

	// Reset resets any internal state of the policy.
	Reset()
}

// FailureClassifier defines the interface for classifying errors into failure types.
type FailureClassifier interface {
	// Classify determines the failure type for a given error.
	Classify(err error) FailureType
}

// FailureType represents different categories of failures for retry logic.
type FailureType int

const (
	FailureTypeUnknown FailureType = iota
	FailureTypeNetwork
	FailureTypeAuthentication
	FailureTypeDiskSpace
	FailureTypeRepositoryAccess
	FailureTypeNATSMessaging
	FailureTypeCircuitBreaker
	FailureTypeTimeout
	FailureTypeRateLimit
)

const (
	unknownFailureTypeStr = "unknown"
)

// String returns the string representation of the failure type.
func (ft FailureType) String() string {
	switch ft {
	case FailureTypeUnknown:
		return unknownFailureTypeStr
	case FailureTypeNetwork:
		return "network"
	case FailureTypeAuthentication:
		return "authentication"
	case FailureTypeDiskSpace:
		return "disk_space"
	case FailureTypeRepositoryAccess:
		return "repository_access"
	case FailureTypeNATSMessaging:
		return "nats_messaging"
	case FailureTypeCircuitBreaker:
		return "circuit_breaker"
	case FailureTypeTimeout:
		return "timeout"
	case FailureTypeRateLimit:
		return "rate_limit"
	default:
		return unknownFailureTypeStr
	}
}

// RetryPolicyConfig holds configuration for retry policies.
type RetryPolicyConfig struct {
	MaxAttempts       int
	BaseDelay         time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
	JitterEnabled     bool
	JitterMaxPercent  int
	CircuitBreaker    *CircuitBreakerConfig
}

// CircuitBreakerConfig holds circuit breaker configuration for retry policies.
type CircuitBreakerConfig struct {
	Enabled               bool
	FailureThreshold      int
	SuccessThreshold      int
	Timeout               time.Duration
	MaxConcurrentRequests int32
}

// Concrete implementations of retry policies

// ExponentialBackoffPolicy implements exponential backoff retry logic.
type ExponentialBackoffPolicy struct {
	config RetryPolicyConfig
	mu     sync.RWMutex
}

// NewExponentialBackoffPolicy creates a new exponential backoff retry policy.
func NewExponentialBackoffPolicy(config RetryPolicyConfig) RetryPolicy {
	return &ExponentialBackoffPolicy{
		config: config,
	}
}

func (p *ExponentialBackoffPolicy) ShouldRetry(ctx context.Context, _ error, attempt int) (bool, time.Duration) {
	// Check context cancellation first
	if ctx.Err() != nil {
		return false, 0
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check if we've exceeded max attempts
	if attempt >= p.config.MaxAttempts {
		return false, 0
	}

	// Calculate exponential backoff delay
	delay := time.Duration(float64(p.config.BaseDelay) * math.Pow(p.config.BackoffMultiplier, float64(attempt-1)))

	// Apply max delay limit
	if p.config.MaxDelay > 0 && delay > p.config.MaxDelay {
		delay = p.config.MaxDelay
	}

	// Apply jitter if enabled
	if p.config.JitterEnabled {
		jitterAmount := float64(delay) * (float64(p.config.JitterMaxPercent) / 100.0)
		jitter := time.Duration(rand.Float64() * jitterAmount) //nolint:gosec // Non-cryptographic use for retry jitter
		delay += jitter
	}

	return true, delay
}

func (p *ExponentialBackoffPolicy) GetMaxAttempts() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.MaxAttempts
}

func (p *ExponentialBackoffPolicy) GetPolicyName() string {
	return "exponential_backoff"
}

func (p *ExponentialBackoffPolicy) Reset() {
	// No internal state to reset for exponential backoff
}

// LinearBackoffPolicy implements linear backoff retry logic.
type LinearBackoffPolicy struct {
	config RetryPolicyConfig
	mu     sync.RWMutex
}

// NewLinearBackoffPolicy creates a new linear backoff retry policy.
func NewLinearBackoffPolicy(config RetryPolicyConfig) RetryPolicy {
	return &LinearBackoffPolicy{
		config: config,
	}
}

func (p *LinearBackoffPolicy) ShouldRetry(ctx context.Context, _ error, attempt int) (bool, time.Duration) {
	if ctx.Err() != nil {
		return false, 0
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if attempt >= p.config.MaxAttempts {
		return false, 0
	}

	// Linear backoff: baseDelay * attempt
	delay := time.Duration(int64(p.config.BaseDelay) * int64(attempt))

	if p.config.MaxDelay > 0 && delay > p.config.MaxDelay {
		delay = p.config.MaxDelay
	}

	return true, delay
}

func (p *LinearBackoffPolicy) GetMaxAttempts() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.MaxAttempts
}

func (p *LinearBackoffPolicy) GetPolicyName() string {
	return "linear_backoff"
}

func (p *LinearBackoffPolicy) Reset() {
	// No internal state to reset for linear backoff
}

// FixedDelayPolicy implements fixed delay retry logic.
type FixedDelayPolicy struct {
	config RetryPolicyConfig
	mu     sync.RWMutex
}

// NewFixedDelayPolicy creates a new fixed delay retry policy.
func NewFixedDelayPolicy(config RetryPolicyConfig) RetryPolicy {
	return &FixedDelayPolicy{
		config: config,
	}
}

func (p *FixedDelayPolicy) ShouldRetry(ctx context.Context, _ error, attempt int) (bool, time.Duration) {
	if ctx.Err() != nil {
		return false, 0
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if attempt >= p.config.MaxAttempts {
		return false, 0
	}

	return true, p.config.BaseDelay
}

func (p *FixedDelayPolicy) GetMaxAttempts() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.MaxAttempts
}

func (p *FixedDelayPolicy) GetPolicyName() string {
	return "fixed_delay"
}

func (p *FixedDelayPolicy) Reset() {
	// No internal state to reset for fixed delay
}

// FailureTypeSpecificPolicy implements failure-type-specific retry logic.
type FailureTypeSpecificPolicy struct {
	policyMap  map[FailureType]RetryPolicyConfig
	classifier FailureClassifier
	mu         sync.RWMutex
}

// NewFailureTypeSpecificPolicy creates a new failure-type-specific retry policy.
func NewFailureTypeSpecificPolicy(policyMap map[FailureType]RetryPolicyConfig) RetryPolicy {
	return &FailureTypeSpecificPolicy{
		policyMap:  policyMap,
		classifier: NewFailureClassifier(),
	}
}

func (p *FailureTypeSpecificPolicy) ShouldRetry(ctx context.Context, err error, attempt int) (bool, time.Duration) {
	if ctx.Err() != nil {
		return false, 0
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Classify the error to determine failure type
	failureType := p.classifier.Classify(err)

	// Get policy config for this failure type
	config, exists := p.policyMap[failureType]
	if !exists {
		// Default to unknown failure type policy
		config = p.policyMap[FailureTypeUnknown]
	}

	if attempt >= config.MaxAttempts {
		return false, 0
	}

	// Calculate delay based on backoff multiplier
	var delay time.Duration
	if config.BackoffMultiplier > 0 {
		delay = time.Duration(float64(config.BaseDelay) * math.Pow(config.BackoffMultiplier, float64(attempt-1)))
	} else {
		delay = config.BaseDelay
	}

	if config.MaxDelay > 0 && delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Apply jitter if enabled
	if config.JitterEnabled {
		jitterAmount := float64(delay) * (float64(config.JitterMaxPercent) / 100.0)
		jitter := time.Duration(rand.Float64() * jitterAmount) //nolint:gosec // Non-cryptographic use for retry jitter
		delay += jitter
	}

	return true, delay
}

func (p *FailureTypeSpecificPolicy) GetMaxAttempts() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return the maximum of all configured max attempts
	maxAttempts := 0
	for _, config := range p.policyMap {
		if config.MaxAttempts > maxAttempts {
			maxAttempts = config.MaxAttempts
		}
	}
	return maxAttempts
}

func (p *FailureTypeSpecificPolicy) GetPolicyName() string {
	return "failure_type_specific"
}

func (p *FailureTypeSpecificPolicy) Reset() {
	// No internal state to reset
}

// BasicFailureClassifier implements basic error classification.
type BasicFailureClassifier struct{}

// NewFailureClassifier creates a new failure classifier.
func NewFailureClassifier() FailureClassifier {
	return &BasicFailureClassifier{}
}

func (c *BasicFailureClassifier) Classify(err error) FailureType {
	if err == nil {
		return FailureTypeUnknown
	}

	errMsg := strings.ToLower(err.Error())

	// NATS messaging errors (check before generic network errors)
	if strings.Contains(errMsg, "nats") ||
		strings.Contains(errMsg, "jetstream") ||
		strings.Contains(errMsg, "messaging") {
		return FailureTypeNATSMessaging
	}

	// Network-related errors
	if strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "network") ||
		strings.Contains(errMsg, "dns") ||
		strings.Contains(errMsg, "refused") {
		return FailureTypeNetwork
	}

	// Authentication errors
	if strings.Contains(errMsg, "authentication") ||
		strings.Contains(errMsg, "auth") ||
		strings.Contains(errMsg, "credentials") ||
		strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "permission") {
		return FailureTypeAuthentication
	}

	// Disk space errors
	if strings.Contains(errMsg, "disk space") ||
		strings.Contains(errMsg, "no space") ||
		strings.Contains(errMsg, "space left") {
		return FailureTypeDiskSpace
	}

	// Repository access errors
	if strings.Contains(errMsg, "repository") ||
		strings.Contains(errMsg, "not found") ||
		strings.Contains(errMsg, "access denied") {
		return FailureTypeRepositoryAccess
	}

	// Circuit breaker errors
	if strings.Contains(errMsg, "circuit breaker") ||
		strings.Contains(errMsg, "circuit") {
		return FailureTypeCircuitBreaker
	}

	// Rate limit errors
	if strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "too many requests") {
		return FailureTypeRateLimit
	}

	// Default to unknown
	return FailureTypeUnknown
}
