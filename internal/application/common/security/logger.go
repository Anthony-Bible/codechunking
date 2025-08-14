package security

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// SecurityLogger provides structured logging for security events.
type SecurityLogger interface {
	LogViolation(ctx context.Context, event SecurityEvent)
	LogMetric(ctx context.Context, metric SecurityMetric)
	LogAccess(ctx context.Context, access AccessEvent)
}

// SecurityEvent represents a security violation event.
type SecurityEvent struct {
	Type             string            `json:"type"`
	Severity         string            `json:"severity"`
	Message          string            `json:"message"`
	Timestamp        time.Time         `json:"timestamp"`
	RequestID        string            `json:"request_id,omitempty"`
	ClientIP         string            `json:"client_ip,omitempty"`
	UserAgent        string            `json:"user_agent,omitempty"`
	Path             string            `json:"path,omitempty"`
	Method           string            `json:"method,omitempty"`
	ViolationDetails ViolationDetails  `json:"violation_details"`
	Context          map[string]string `json:"context,omitempty"`
}

// ViolationDetails provides specific details about the security violation.
type ViolationDetails struct {
	Field       string `json:"field,omitempty"`
	Value       string `json:"value,omitempty"` // Sanitized version for logging
	Pattern     string `json:"pattern,omitempty"`
	Description string `json:"description,omitempty"`
}

// SecurityMetric represents security-related metrics.
type SecurityMetric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// AccessEvent represents successful or failed access attempts.
type AccessEvent struct {
	Type       string            `json:"type"` // SUCCESS, FAILURE, BLOCKED
	Timestamp  time.Time         `json:"timestamp"`
	ClientIP   string            `json:"client_ip"`
	UserAgent  string            `json:"user_agent,omitempty"`
	Path       string            `json:"path"`
	Method     string            `json:"method"`
	StatusCode int               `json:"status_code"`
	Duration   time.Duration     `json:"duration"`
	Context    map[string]string `json:"context,omitempty"`
}

// DefaultSecurityLogger provides a default implementation.
type DefaultSecurityLogger struct {
	config  *Config
	metrics SecurityMetrics
}

// NewDefaultSecurityLogger creates a new default security logger.
func NewDefaultSecurityLogger(config *Config) *DefaultSecurityLogger {
	return &DefaultSecurityLogger{
		config:  config,
		metrics: NewSecurityMetrics(),
	}
}

// LogViolation logs a security violation event.
func (dsl *DefaultSecurityLogger) LogViolation(ctx context.Context, event SecurityEvent) {
	if !dsl.config.LogSecurityViolations {
		return
	}

	// Sanitize the value before logging to avoid log injection
	event.ViolationDetails.Value = dsl.sanitizeLogValue(event.ViolationDetails.Value)

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Extract request ID from context if available
	if event.RequestID == "" {
		if reqID := ctx.Value("request_id"); reqID != nil {
			if id, ok := reqID.(string); ok {
				event.RequestID = id
			}
		}
	}

	// Log based on severity and configuration
	switch event.Severity {
	case "CRITICAL", "HIGH":
		slog.Error("Security violation",
			"severity", "CRITICAL",
			"type", event.Type,
			"message", event.Message,
			"client_ip", event.ClientIP,
			"path", event.Path,
			"field", event.ViolationDetails.Field,
			"request_id", event.RequestID)
	case "MEDIUM":
		slog.Warn("Security violation",
			"severity", "MEDIUM",
			"type", event.Type,
			"message", event.Message,
			"client_ip", event.ClientIP,
			"path", event.Path,
			"field", event.ViolationDetails.Field,
			"request_id", event.RequestID)
	case "LOW":
		if dsl.config.LogLevel == "DEBUG" {
			slog.Info("Security violation",
				"severity", "LOW",
				"type", event.Type,
				"message", event.Message,
				"client_ip", event.ClientIP,
				"path", event.Path,
				"field", event.ViolationDetails.Field,
				"request_id", event.RequestID)
		}
	default:
		slog.Info("Security event",
			"type", event.Type,
			"message", event.Message,
			"client_ip", event.ClientIP,
			"path", event.Path,
			"field", event.ViolationDetails.Field,
			"request_id", event.RequestID)
	}

	// Update metrics
	dsl.metrics.IncrementViolation(event.Type)
}

// LogMetric logs a security metric.
func (dsl *DefaultSecurityLogger) LogMetric(ctx context.Context, metric SecurityMetric) {
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	// Store metric for monitoring systems
	dsl.metrics.RecordMetric(metric.Name, metric.Value, metric.Labels)

	// Log if debug mode
	if dsl.config.LogLevel == "DEBUG" {
		slog.Debug("Security metric",
			"name", metric.Name,
			"value", metric.Value,
			"labels", metric.Labels)
	}
}

// LogAccess logs an access event.
func (dsl *DefaultSecurityLogger) LogAccess(ctx context.Context, access AccessEvent) {
	if access.Timestamp.IsZero() {
		access.Timestamp = time.Now()
	}

	// Log based on access type
	switch access.Type {
	case "BLOCKED":
		slog.Warn("Access blocked",
			"type", access.Type,
			"method", access.Method,
			"path", access.Path,
			"status_code", access.StatusCode,
			"duration", access.Duration,
			"client_ip", access.ClientIP)
	case "FAILURE":
		slog.Error("Access failed",
			"type", access.Type,
			"method", access.Method,
			"path", access.Path,
			"status_code", access.StatusCode,
			"duration", access.Duration,
			"client_ip", access.ClientIP)
	case "SUCCESS":
		if dsl.config.LogLevel == "DEBUG" {
			slog.Debug("Access successful",
				"type", access.Type,
				"method", access.Method,
				"path", access.Path,
				"status_code", access.StatusCode,
				"duration", access.Duration,
				"client_ip", access.ClientIP)
		}
	}

	// Update metrics
	dsl.metrics.RecordAccess(access.Type, access.StatusCode)
}

// sanitizeLogValue sanitizes values before logging to prevent log injection.
func (dsl *DefaultSecurityLogger) sanitizeLogValue(value string) string {
	if len(value) == 0 {
		return ""
	}

	// Limit length for logging
	maxLen := 100
	if len(value) > maxLen {
		value = value[:maxLen] + "..."
	}

	// Replace problematic characters that could break log parsing
	sanitized := value
	sanitized = fmt.Sprintf("%q", sanitized) // Go's string quoting handles most issues

	return sanitized
}

// SecurityMetrics tracks security-related metrics.
type SecurityMetrics interface {
	IncrementViolation(violationType string)
	RecordMetric(name string, value float64, labels map[string]string)
	RecordAccess(accessType string, statusCode int)
	GetViolationCount(violationType string) int64
	GetTotalViolations() int64
	GetMetrics() map[string]interface{}
}

// DefaultSecurityMetrics provides a default metrics implementation.
type DefaultSecurityMetrics struct {
	violationCounts map[string]int64
	accessCounts    map[string]int64
	customMetrics   map[string]float64
	mutex           sync.RWMutex
}

// NewSecurityMetrics creates a new security metrics instance.
func NewSecurityMetrics() SecurityMetrics {
	return &DefaultSecurityMetrics{
		violationCounts: make(map[string]int64),
		accessCounts:    make(map[string]int64),
		customMetrics:   make(map[string]float64),
	}
}

// IncrementViolation increments the count for a specific violation type.
func (dsm *DefaultSecurityMetrics) IncrementViolation(violationType string) {
	dsm.mutex.Lock()
	defer dsm.mutex.Unlock()

	dsm.violationCounts[violationType]++
	dsm.violationCounts["total"]++
}

// RecordMetric records a custom security metric.
func (dsm *DefaultSecurityMetrics) RecordMetric(name string, value float64, labels map[string]string) {
	dsm.mutex.Lock()
	defer dsm.mutex.Unlock()

	// Simple implementation - in production this would integrate with proper metrics system
	dsm.customMetrics[name] = value
}

// RecordAccess records an access event.
func (dsm *DefaultSecurityMetrics) RecordAccess(accessType string, statusCode int) {
	dsm.mutex.Lock()
	defer dsm.mutex.Unlock()

	dsm.accessCounts[accessType]++
	dsm.accessCounts[fmt.Sprintf("status_%d", statusCode)]++
}

// GetViolationCount returns the count for a specific violation type.
func (dsm *DefaultSecurityMetrics) GetViolationCount(violationType string) int64 {
	dsm.mutex.RLock()
	defer dsm.mutex.RUnlock()

	return dsm.violationCounts[violationType]
}

// GetTotalViolations returns the total count of all violations.
func (dsm *DefaultSecurityMetrics) GetTotalViolations() int64 {
	dsm.mutex.RLock()
	defer dsm.mutex.RUnlock()

	return dsm.violationCounts["total"]
}

// GetMetrics returns all collected metrics.
func (dsm *DefaultSecurityMetrics) GetMetrics() map[string]interface{} {
	dsm.mutex.RLock()
	defer dsm.mutex.RUnlock()

	metrics := make(map[string]interface{})

	// Copy violation counts
	violations := make(map[string]int64)
	for k, v := range dsm.violationCounts {
		violations[k] = v
	}
	metrics["violations"] = violations

	// Copy access counts
	access := make(map[string]int64)
	for k, v := range dsm.accessCounts {
		access[k] = v
	}
	metrics["access"] = access

	// Copy custom metrics
	custom := make(map[string]float64)
	for k, v := range dsm.customMetrics {
		custom[k] = v
	}
	metrics["custom"] = custom

	return metrics
}

// CreateSecurityEvent creates a new security event.
func CreateSecurityEvent(violationType, message string, severity string) SecurityEvent {
	return SecurityEvent{
		Type:             violationType,
		Severity:         severity,
		Message:          message,
		Timestamp:        time.Now(),
		ViolationDetails: ViolationDetails{},
		Context:          make(map[string]string),
	}
}

// WithField adds field information to a security event.
func (se SecurityEvent) WithField(field, value string) SecurityEvent {
	se.ViolationDetails.Field = field
	se.ViolationDetails.Value = value
	return se
}

// WithContext adds context information to a security event.
func (se SecurityEvent) WithContext(key, value string) SecurityEvent {
	if se.Context == nil {
		se.Context = make(map[string]string)
	}
	se.Context[key] = value
	return se
}

// WithRequest adds request information to a security event.
func (se SecurityEvent) WithRequest(requestID, clientIP, userAgent, path, method string) SecurityEvent {
	se.RequestID = requestID
	se.ClientIP = clientIP
	se.UserAgent = userAgent
	se.Path = path
	se.Method = method
	return se
}

// NullSecurityLogger provides a no-op implementation for testing.
type NullSecurityLogger struct{}

func (nsl *NullSecurityLogger) LogViolation(ctx context.Context, event SecurityEvent) {}
func (nsl *NullSecurityLogger) LogMetric(ctx context.Context, metric SecurityMetric)  {}
func (nsl *NullSecurityLogger) LogAccess(ctx context.Context, access AccessEvent)     {}

// NewNullSecurityLogger creates a null security logger.
func NewNullSecurityLogger() SecurityLogger {
	return &NullSecurityLogger{}
}
