package valueobject

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// AlertType represents the type of alert and its delivery characteristics.
type AlertType struct {
	alertType string
}

const (
	RealTimeType = "REAL_TIME"
	BatchType    = "BATCH"
	CascadeType  = "CASCADE"
)

var validAlertTypes = map[string]int{
	RealTimeType: 0,
	CascadeType:  1,
	BatchType:    2,
}

// MessageFormat defines how alert messages should be formatted.
type MessageFormat struct {
	Template             string
	IncludeTimestamp     bool
	IncludeCorrelationID bool
	IncludeStackTrace    bool
	Color                string
}

// NewAlertType creates a new alert type with validation.
func NewAlertType(alertType string) (*AlertType, error) {
	if alertType == "" {
		return nil, errors.New("invalid alert type: cannot be empty")
	}

	if _, exists := validAlertTypes[alertType]; !exists {
		return nil, fmt.Errorf("invalid alert type: %s is not a valid type", alertType)
	}

	return &AlertType{alertType: alertType}, nil
}

// String returns the string representation of the alert type.
func (at *AlertType) String() string {
	return at.alertType
}

// IsRealTime returns true if this is a real-time alert.
func (at *AlertType) IsRealTime() bool {
	return at.alertType == RealTimeType
}

// IsBatch returns true if this is a batch alert.
func (at *AlertType) IsBatch() bool {
	return at.alertType == BatchType
}

// IsCascade returns true if this is a cascade alert.
func (at *AlertType) IsCascade() bool {
	return at.alertType == CascadeType
}

// Priority returns the numeric priority (0 = highest priority).
func (at *AlertType) Priority() int {
	return validAlertTypes[at.alertType]
}

// IsHigherPriority returns true if this alert type has higher priority than other.
func (at *AlertType) IsHigherPriority(other *AlertType) bool {
	return at.Priority() < other.Priority()
}

// RequiresImmediateDelivery returns true if this alert type requires immediate delivery.
func (at *AlertType) RequiresImmediateDelivery() bool {
	return at.alertType == RealTimeType || at.alertType == CascadeType
}

// MaxDeliveryDelay returns the maximum acceptable delay for delivery.
func (at *AlertType) MaxDeliveryDelay() time.Duration {
	switch at.alertType {
	case RealTimeType, CascadeType:
		return 0 // Immediate
	case BatchType:
		return 5 * time.Minute
	default:
		return 0
	}
}

// MaxRetryAttempts returns the maximum number of retry attempts.
func (at *AlertType) MaxRetryAttempts() int {
	switch at.alertType {
	case RealTimeType:
		return 3
	case CascadeType:
		return 5
	case BatchType:
		return 2
	default:
		return 1
	}
}

// PreferredChannels returns the preferred delivery channels for this alert type.
func (at *AlertType) PreferredChannels() []string {
	switch at.alertType {
	case RealTimeType:
		return []string{"pagerduty", "slack", "sms"}
	case CascadeType:
		return []string{"pagerduty", "slack", "sms", "email"}
	case BatchType:
		return []string{"email", "slack"}
	default:
		return []string{}
	}
}

// SupportsEscalation returns true if this alert type supports escalation.
func (at *AlertType) SupportsEscalation() bool {
	return at.alertType == RealTimeType || at.alertType == CascadeType
}

// EscalationDelay returns the delay before escalating.
func (at *AlertType) EscalationDelay() time.Duration {
	switch at.alertType {
	case RealTimeType:
		return 5 * time.Minute
	case CascadeType:
		return 1 * time.Minute
	default:
		return 0
	}
}

// EscalationLevels returns the escalation levels for this alert type.
func (at *AlertType) EscalationLevels() []string {
	switch at.alertType {
	case RealTimeType:
		return []string{"manager", "director", "executive"}
	case CascadeType:
		return []string{"manager", "director", "executive", "cto"}
	default:
		return []string{}
	}
}

// MessageFormat returns the message formatting rules for this alert type.
func (at *AlertType) MessageFormat() MessageFormat {
	switch at.alertType {
	case RealTimeType:
		return MessageFormat{
			Template:             "🚨 URGENT: {message}",
			IncludeTimestamp:     true,
			IncludeCorrelationID: true,
			IncludeStackTrace:    true,
			Color:                "RED",
		}
	case CascadeType:
		return MessageFormat{
			Template:             "💥 CASCADE: {message}",
			IncludeTimestamp:     true,
			IncludeCorrelationID: true,
			IncludeStackTrace:    true,
			Color:                "PURPLE",
		}
	case BatchType:
		return MessageFormat{
			Template:             "📊 BATCH: {message}",
			IncludeTimestamp:     true,
			IncludeCorrelationID: false,
			IncludeStackTrace:    false,
			Color:                "YELLOW",
		}
	default:
		return MessageFormat{}
	}
}

// Equals returns true if both alert types are equal.
func (at *AlertType) Equals(other *AlertType) bool {
	return at.alertType == other.alertType
}

// MarshalJSON implements json.Marshaler.
func (at *AlertType) MarshalJSON() ([]byte, error) {
	return json.Marshal(at.alertType)
}

// UnmarshalJSON implements json.Unmarshaler.
func (at *AlertType) UnmarshalJSON(data []byte) error {
	var alertType string
	if err := json.Unmarshal(data, &alertType); err != nil {
		return err
	}

	alertTypeObj, err := NewAlertType(alertType)
	if err != nil {
		return err
	}

	at.alertType = alertTypeObj.alertType
	return nil
}
