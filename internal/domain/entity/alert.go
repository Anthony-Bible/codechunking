package entity

import (
	"codechunking/internal/domain/valueobject"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// AlertStatus represents the status of an alert.
type AlertStatus string

const (
	AlertStatusPending            AlertStatus = "PENDING"
	AlertStatusSent               AlertStatus = "SENT"
	AlertStatusFailed             AlertStatus = "FAILED"
	AlertStatusPartiallyDelivered AlertStatus = "PARTIALLY_DELIVERED"
)

// Production configuration constants.
const (
	maxMessageLength    = 1000
	maxRecipients       = 50  // Limit number of recipients to prevent resource exhaustion
	maxDeliveryAttempts = 100 // Limit delivery attempts history
	maxEscalationRules  = 20  // Limit escalation rules
)

// AlertRecipient represents a recipient for an alert with validation.
type AlertRecipient struct {
	Type    string `json:"type"`
	Address string `json:"address"`
	Name    string `json:"name"`
}

// Validate performs validation on the AlertRecipient.
func (ar *AlertRecipient) Validate() error {
	if len(ar.Type) == 0 {
		return errors.New("recipient type cannot be empty")
	}
	if len(ar.Address) == 0 {
		return errors.New("recipient address cannot be empty")
	}
	if len(ar.Type) > 50 {
		return errors.New("recipient type cannot exceed 50 characters")
	}
	if len(ar.Address) > 255 {
		return errors.New("recipient address cannot exceed 255 characters")
	}
	if ar.Type == "email" && !emailRegex.MatchString(ar.Address) {
		return errors.New("invalid email address format")
	}
	return nil
}

// DeliveryResult represents the result of an alert delivery attempt.
type DeliveryResult struct {
	RecipientType    string    `json:"recipient_type"`
	RecipientAddress string    `json:"recipient_address"`
	Success          bool      `json:"success"`
	AttemptedAt      time.Time `json:"attempted_at"`
	DeliveredAt      time.Time `json:"delivered_at,omitempty"`
	Error            string    `json:"error,omitempty"`
}

// EscalationRule represents a rule for alert escalation.
type EscalationRule struct {
	Condition  string   `json:"condition"`
	Action     string   `json:"action"`
	Recipients []string `json:"recipients"`
}

// Alert represents an alert that can be sent to recipients with thread-safe operations.
type Alert struct {
	// Immutable fields (set once at creation)
	id                  string
	errorID             string
	alertType           *valueobject.AlertType
	severity            *valueobject.ErrorSeverity
	message             string
	createdAt           time.Time
	deduplicationKey    string
	aggregationPattern  string
	errorCount          int
	relatedAggregations []*ErrorAggregation
	alertCategory       string

	// Mutable fields (protected by mutex for thread-safety)
	mu                  sync.RWMutex
	status              AlertStatus
	sentAt              *time.Time
	recipients          []AlertRecipient
	deliveryAttempts    []DeliveryResult
	deduplicationWindow time.Duration
	escalationRules     []EscalationRule

	// Performance optimization: atomic counter for delivery attempts to avoid mutex for reads
	deliveryAttemptCount int64
}

// emailRegex compiled once for performance - thread-safe for reads.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// NewAlert creates a new alert for a single classified error.
func NewAlert(
	classifiedError *ClassifiedError,
	alertType *valueobject.AlertType,
	message string,
) (*Alert, error) {
	if classifiedError == nil {
		return nil, errors.New("classified error cannot be nil")
	}
	if alertType == nil {
		return nil, errors.New("alert type cannot be nil")
	}
	if message == "" {
		return nil, errors.New("alert message cannot be empty")
	}
	if len(message) > maxMessageLength {
		return nil, errors.New("alert message cannot exceed 1000 characters")
	}

	id := generateOptimizedID()
	deduplicationKey := generateDeduplicationKey(classifiedError.ErrorPattern(), alertType)

	return &Alert{
		id:               id,
		errorID:          classifiedError.ID(),
		alertType:        alertType,
		severity:         classifiedError.Severity(),
		message:          message,
		status:           AlertStatusPending,
		createdAt:        time.Now(),
		deduplicationKey: deduplicationKey,
		alertCategory:    "single_error",
	}, nil
}

// NewAggregationAlert creates a new alert for an error aggregation.
func NewAggregationAlert(
	aggregation *ErrorAggregation,
	alertType *valueobject.AlertType,
	message string,
) (*Alert, error) {
	if aggregation == nil {
		return nil, errors.New("aggregation cannot be nil")
	}
	if alertType == nil {
		return nil, errors.New("alert type cannot be nil")
	}
	if message == "" {
		return nil, errors.New("alert message cannot be empty")
	}

	id := generateOptimizedID()
	deduplicationKey := generateDeduplicationKey(aggregation.Pattern(), alertType)

	return &Alert{
		id:                 id,
		alertType:          alertType,
		severity:           aggregation.HighestSeverity(),
		message:            message,
		status:             AlertStatusPending,
		createdAt:          time.Now(),
		deduplicationKey:   deduplicationKey,
		aggregationPattern: aggregation.Pattern(),
		errorCount:         aggregation.Count(),
		alertCategory:      "batch_aggregation",
	}, nil
}

// NewCascadeFailureAlert creates a new alert for cascade failures.
func NewCascadeFailureAlert(
	aggregations []*ErrorAggregation,
	alertType *valueobject.AlertType,
	message string,
) (*Alert, error) {
	if len(aggregations) == 0 {
		return nil, errors.New("aggregations cannot be empty")
	}
	if alertType == nil {
		return nil, errors.New("alert type cannot be nil")
	}
	if message == "" {
		return nil, errors.New("alert message cannot be empty")
	}

	id := generateOptimizedID()
	patterns := make([]string, len(aggregations))
	for i, agg := range aggregations {
		patterns[i] = agg.Pattern()
	}
	deduplicationKey := generateDeduplicationKey(strings.Join(patterns, ","), alertType)

	// Find highest severity across all aggregations
	var highestSeverity *valueobject.ErrorSeverity
	for _, agg := range aggregations {
		if highestSeverity == nil || agg.HighestSeverity().IsHigherPriority(highestSeverity) {
			highestSeverity = agg.HighestSeverity()
		}
	}

	return &Alert{
		id:                  id,
		alertType:           alertType,
		severity:            highestSeverity,
		message:             message,
		status:              AlertStatusPending,
		createdAt:           time.Now(),
		deduplicationKey:    deduplicationKey,
		relatedAggregations: aggregations,
		alertCategory:       "cascade_failure",
	}, nil
}

// Immutable field accessors (no locking needed).

// ID returns the alert ID.
func (a *Alert) ID() string {
	return a.id
}

// ErrorID returns the error ID for single error alerts.
func (a *Alert) ErrorID() string {
	return a.errorID
}

// Type returns the alert type.
func (a *Alert) Type() *valueobject.AlertType {
	return a.alertType
}

// Severity returns the alert severity.
func (a *Alert) Severity() *valueobject.ErrorSeverity {
	return a.severity
}

// Message returns the alert message.
func (a *Alert) Message() string {
	return a.message
}

// CreatedAt returns when the alert was created.
func (a *Alert) CreatedAt() time.Time {
	return a.createdAt
}

// Thread-safe mutable field accessors.

// Status returns the alert status with thread-safe access.
func (a *Alert) Status() AlertStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// SentAt returns when the alert was sent with thread-safe access.
func (a *Alert) SentAt() *time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.sentAt == nil {
		return nil
	}
	// Return copy to prevent external modification
	sentAtCopy := *a.sentAt
	return &sentAtCopy
}

// Recipients returns a copy of alert recipients with thread-safe access.
func (a *Alert) Recipients() []AlertRecipient {
	a.mu.RLock()
	defer a.mu.RUnlock()
	// Return copy to prevent external modification
	recipientsCopy := make([]AlertRecipient, len(a.recipients))
	copy(recipientsCopy, a.recipients)
	return recipientsCopy
}

// DeliveryAttempts returns a copy of all delivery attempts with thread-safe access.
func (a *Alert) DeliveryAttempts() []DeliveryResult {
	a.mu.RLock()
	defer a.mu.RUnlock()
	// Return copy to prevent external modification
	attemptsCopy := make([]DeliveryResult, len(a.deliveryAttempts))
	copy(attemptsCopy, a.deliveryAttempts)
	return attemptsCopy
}

// DeliveryAttemptCount returns the number of delivery attempts without locking.
func (a *Alert) DeliveryAttemptCount() int64 {
	return atomic.LoadInt64(&a.deliveryAttemptCount)
}

// AggregationPattern returns the aggregation pattern for batch alerts.
func (a *Alert) AggregationPattern() string {
	return a.aggregationPattern
}

// ErrorCount returns the error count for batch alerts.
func (a *Alert) ErrorCount() int {
	return a.errorCount
}

// RelatedAggregations returns related aggregations for cascade alerts.
func (a *Alert) RelatedAggregations() []*ErrorAggregation {
	return a.relatedAggregations
}

// AlertCategory returns the alert category.
func (a *Alert) AlertCategory() string {
	return a.alertCategory
}

// IsBatchAlert returns true if this is a batch alert.
func (a *Alert) IsBatchAlert() bool {
	return a.alertCategory == "batch_aggregation"
}

// IsRealTimeAlert returns true if this is a real-time alert.
func (a *Alert) IsRealTimeAlert() bool {
	return a.alertType.IsRealTime()
}

// IsCascadeAlert returns true if this is a cascade failure alert.
func (a *Alert) IsCascadeAlert() bool {
	return a.alertCategory == "cascade_failure"
}

// AddRecipients adds recipients to the alert with thread-safe validation and limits.
func (a *Alert) AddRecipients(recipients []AlertRecipient) error {
	if len(recipients) == 0 {
		return nil
	}

	// Pre-validate all recipients before acquiring lock
	for i := range recipients {
		if err := recipients[i].Validate(); err != nil {
			return fmt.Errorf("invalid recipient at index %d: %w", i, err)
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check recipient limits
	if len(a.recipients)+len(recipients) > maxRecipients {
		return fmt.Errorf("cannot add recipients: would exceed maximum limit of %d", maxRecipients)
	}

	// Check for duplicates to prevent spam
	existingRecipients := make(map[string]bool, len(a.recipients))
	for _, existing := range a.recipients {
		key := existing.Type + ":" + existing.Address
		existingRecipients[key] = true
	}

	var newRecipients []AlertRecipient
	for _, recipient := range recipients {
		key := recipient.Type + ":" + recipient.Address
		if !existingRecipients[key] {
			newRecipients = append(newRecipients, recipient)
			existingRecipients[key] = true
		}
	}

	a.recipients = append(a.recipients, newRecipients...)
	return nil
}

// FindRecipient finds a recipient by type and address.
func (a *Alert) FindRecipient(recipientType, address string) *AlertRecipient {
	for i := range a.recipients {
		if a.recipients[i].Type == recipientType && a.recipients[i].Address == address {
			return &a.recipients[i]
		}
	}
	return nil
}

// RemoveRecipient removes a recipient by type and address.
func (a *Alert) RemoveRecipient(recipientType, address string) error {
	for i, recipient := range a.recipients {
		if recipient.Type == recipientType && recipient.Address == address {
			a.recipients = append(a.recipients[:i], a.recipients[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("recipient not found: %s %s", recipientType, address)
}

// RecordDeliveryAttempt records a delivery attempt with thread-safe operations and limits.
func (a *Alert) RecordDeliveryAttempt(result DeliveryResult) error {
	// Validate delivery result
	if err := result.Validate(); err != nil {
		return fmt.Errorf("invalid delivery result: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check delivery attempts limit to prevent memory exhaustion
	if len(a.deliveryAttempts) >= maxDeliveryAttempts {
		// Remove oldest attempt to make room (FIFO)
		a.deliveryAttempts = a.deliveryAttempts[1:]
	}

	a.deliveryAttempts = append(a.deliveryAttempts, result)
	atomic.AddInt64(&a.deliveryAttemptCount, 1)

	// Update alert status based on delivery results
	a.updateStatusUnsafe() // Called under lock, so use unsafe version

	return nil
}

// HasFailedDeliveries returns true if any delivery attempts failed.
func (a *Alert) HasFailedDeliveries() bool {
	for _, attempt := range a.deliveryAttempts {
		if !attempt.Success {
			return true
		}
	}
	return false
}

// HasSuccessfulDeliveries returns true if any delivery attempts succeeded.
func (a *Alert) HasSuccessfulDeliveries() bool {
	for _, attempt := range a.deliveryAttempts {
		if attempt.Success {
			return true
		}
	}
	return false
}

// IsEligibleForRetry returns true if the alert is eligible for retry.
func (a *Alert) IsEligibleForRetry() bool {
	return a.status == AlertStatusFailed || a.status == AlertStatusPartiallyDelivered
}

// RetryAttempts returns the number of retry attempts for a specific recipient.
func (a *Alert) RetryAttempts(recipientType, address string) int {
	count := 0
	for _, attempt := range a.deliveryAttempts {
		if attempt.RecipientType == recipientType && attempt.RecipientAddress == address {
			count++
		}
	}
	return count
}

// DeduplicationKey returns the deduplication key.
func (a *Alert) DeduplicationKey() string {
	return a.deduplicationKey
}

// SetDeduplicationWindow sets the deduplication window with thread-safe access and validation.
func (a *Alert) SetDeduplicationWindow(window time.Duration) error {
	if window < 0 {
		return errors.New("deduplication window cannot be negative")
	}
	if window > 24*time.Hour {
		return errors.New("deduplication window cannot exceed 24 hours")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.deduplicationWindow = window
	return nil
}

// DeduplicationWindow returns the deduplication window with thread-safe access.
func (a *Alert) DeduplicationWindow() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.deduplicationWindow
}

// IsWithinDeduplicationWindow returns true if the alert is within the deduplication window.
func (a *Alert) IsWithinDeduplicationWindow() bool {
	if a.deduplicationWindow == 0 {
		return false
	}
	return time.Since(a.createdAt) < a.deduplicationWindow
}

// ShouldSuppressDuplicate returns true if a duplicate alert should be suppressed.
func (a *Alert) ShouldSuppressDuplicate(other *Alert) bool {
	return a.deduplicationKey == other.deduplicationKey && a.IsWithinDeduplicationWindow()
}

// EscalationRules returns a copy of escalation rules with thread-safe access.
func (a *Alert) EscalationRules() []EscalationRule {
	a.mu.RLock()
	defer a.mu.RUnlock()
	// Return copy to prevent external modification
	rulesCopy := make([]EscalationRule, len(a.escalationRules))
	copy(rulesCopy, a.escalationRules)
	return rulesCopy
}

// SetEscalationRules sets the escalation rules with validation and limits.
func (a *Alert) SetEscalationRules(rules []EscalationRule) error {
	if len(rules) > maxEscalationRules {
		return fmt.Errorf("cannot set escalation rules: exceeds maximum limit of %d", maxEscalationRules)
	}

	// Validate all rules before setting
	for i := range rules {
		if err := rules[i].Validate(); err != nil {
			return fmt.Errorf("invalid escalation rule at index %d: %w", i, err)
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Deep copy to prevent external modifications
	a.escalationRules = make([]EscalationRule, len(rules))
	copy(a.escalationRules, rules)
	return nil
}

// Validate validates an EscalationRule.
func (er *EscalationRule) Validate() error {
	if len(er.Condition) == 0 {
		return errors.New("escalation rule condition cannot be empty")
	}
	if len(er.Action) == 0 {
		return errors.New("escalation rule action cannot be empty")
	}
	if len(er.Recipients) == 0 {
		return errors.New("escalation rule must have at least one recipient")
	}
	if len(er.Recipients) > maxRecipients {
		return fmt.Errorf("escalation rule cannot have more than %d recipients", maxRecipients)
	}
	return nil
}

// IsEligibleForEscalation returns true if the alert is eligible for escalation.
func (a *Alert) IsEligibleForEscalation() bool {
	// Simple implementation: only escalate if delivery failed and we have rules
	return len(a.escalationRules) > 0 && a.HasFailedDeliveries() && !a.HasSuccessfulDeliveries()
}

// GetNextEscalationRule returns the next escalation rule to apply.
func (a *Alert) GetNextEscalationRule() *EscalationRule {
	if !a.IsEligibleForEscalation() {
		return nil
	}
	if len(a.escalationRules) > 0 {
		return &a.escalationRules[0]
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (a *Alert) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                a.id,
		"error_id":          a.errorID,
		"type":              a.alertType.String(),
		"severity":          a.severity.String(),
		"message":           a.message,
		"status":            string(a.status),
		"created_at":        a.createdAt,
		"sent_at":           a.sentAt,
		"recipients":        a.recipients,
		"delivery_attempts": a.deliveryAttempts,
		"deduplication_key": a.deduplicationKey,
		"alert_category":    a.alertCategory,
	})
}

// UnmarshalJSON implements json.Unmarshaler with reduced cognitive complexity.
func (a *Alert) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	a.parseBasicFields(raw)

	if err := a.parseValueObjects(raw); err != nil {
		return err
	}

	if err := a.parseTimestamps(raw); err != nil {
		return err
	}

	a.parseRecipients(raw)

	return nil
}

// parseBasicFields extracts basic string fields from raw JSON data.
func (a *Alert) parseBasicFields(raw map[string]interface{}) {
	a.id = getStringFromRaw(raw, "id")
	a.errorID = getStringFromRaw(raw, "error_id")
	a.message = getStringFromRaw(raw, "message")
	a.status = AlertStatus(getStringFromRaw(raw, "status"))
	a.deduplicationKey = getStringFromRaw(raw, "deduplication_key")
	a.alertCategory = getStringFromRaw(raw, "alert_category")
}

// parseValueObjects extracts and validates value objects from raw JSON data.
func (a *Alert) parseValueObjects(raw map[string]interface{}) error {
	if typeStr := getStringFromRaw(raw, "type"); typeStr != "" {
		alertType, err := valueobject.NewAlertType(typeStr)
		if err != nil {
			return fmt.Errorf("invalid alert type: %w", err)
		}
		a.alertType = alertType
	}

	if severityStr := getStringFromRaw(raw, "severity"); severityStr != "" {
		severity, err := valueobject.NewErrorSeverity(severityStr)
		if err != nil {
			return fmt.Errorf("invalid severity: %w", err)
		}
		a.severity = severity
	}

	return nil
}

// parseTimestamps extracts and validates timestamps from raw JSON data.
func (a *Alert) parseTimestamps(raw map[string]interface{}) error {
	if createdAtStr := getStringFromRaw(raw, "created_at"); createdAtStr != "" {
		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return fmt.Errorf("invalid created_at timestamp: %w", err)
		}
		a.createdAt = createdAt
	}

	if sentAtStr := getStringFromRaw(raw, "sent_at"); sentAtStr != "" {
		sentAt, err := time.Parse(time.RFC3339, sentAtStr)
		if err != nil {
			return fmt.Errorf("invalid sent_at timestamp: %w", err)
		}
		a.sentAt = &sentAt
	}

	return nil
}

// parseRecipients extracts recipients array from raw JSON data.
func (a *Alert) parseRecipients(raw map[string]interface{}) {
	recipientsData, ok := raw["recipients"]
	if !ok {
		return
	}

	recipientsArray, ok := recipientsData.([]interface{})
	if !ok {
		return
	}

	for _, recipientData := range recipientsArray {
		recipientMap, ok := recipientData.(map[string]interface{})
		if !ok {
			continue
		}

		recipient := AlertRecipient{
			Type:    getStringFromRaw(recipientMap, "type"),
			Address: getStringFromRaw(recipientMap, "address"),
			Name:    getStringFromRaw(recipientMap, "name"),
		}
		a.recipients = append(a.recipients, recipient)
	}
}

// Validate validates a DeliveryResult.
func (dr *DeliveryResult) Validate() error {
	if len(dr.RecipientType) == 0 {
		return errors.New("recipient type cannot be empty")
	}
	if len(dr.RecipientAddress) == 0 {
		return errors.New("recipient address cannot be empty")
	}
	if dr.AttemptedAt.IsZero() {
		return errors.New("attempted at timestamp cannot be zero")
	}
	if dr.Success && dr.DeliveredAt.IsZero() {
		return errors.New("delivered at timestamp required for successful deliveries")
	}
	if !dr.Success && len(dr.Error) == 0 {
		return errors.New("error message required for failed deliveries")
	}
	return nil
}

// updateStatusUnsafe updates alert status without locking (must be called under lock).
func (a *Alert) updateStatusUnsafe() {
	if len(a.deliveryAttempts) == 0 {
		a.status = AlertStatusPending
		return
	}

	// Efficiently find latest attempts by recipient using map with pre-allocated size
	latestAttempts := make(map[string]DeliveryResult, len(a.recipients))
	for i := range a.deliveryAttempts {
		attempt := a.deliveryAttempts[i]
		// Use string builder for efficient key creation
		// Create efficient key using simple concatenation to avoid error checking
		key := attempt.RecipientType + ":" + attempt.RecipientAddress

		if latest, exists := latestAttempts[key]; !exists || attempt.AttemptedAt.After(latest.AttemptedAt) {
			latestAttempts[key] = attempt
		}
	}

	// Count successes and failures efficiently
	successCount := 0
	failCount := 0
	for _, attempt := range latestAttempts {
		if attempt.Success {
			successCount++
		} else {
			failCount++
		}
	}

	// Determine overall status and update sentAt timestamp
	now := time.Now()
	switch {
	case successCount > 0 && failCount == 0:
		a.status = AlertStatusSent
		if a.sentAt == nil {
			a.sentAt = &now
		}
	case failCount > 0 && successCount == 0:
		a.status = AlertStatusFailed
	case successCount > 0 && failCount > 0:
		a.status = AlertStatusPartiallyDelivered
		if a.sentAt == nil {
			a.sentAt = &now
		}
	default:
		a.status = AlertStatusPending
	}
}

// generateDeduplicationKey creates a deduplication key using string builder for efficiency.
func generateDeduplicationKey(pattern string, alertType *valueobject.AlertType) string {
	// Use direct string concatenation for simplicity and to avoid error checking
	return pattern + ":" + strings.ToLower(alertType.String())
}

// generateOptimizedID is defined in classified_error.go and shared across the package.

func getStringFromRaw(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
