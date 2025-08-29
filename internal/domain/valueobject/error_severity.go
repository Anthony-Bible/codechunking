package valueobject

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrorSeverity represents the severity level of an error.
type ErrorSeverity struct {
	level string
}

const (
	CriticalLevel = "CRITICAL"
	ErrorLevel    = "ERROR"
	WarningLevel  = "WARNING"
	InfoLevel     = "INFO"
)

var validLevels = map[string]int{
	CriticalLevel: 0,
	ErrorLevel:    1,
	WarningLevel:  2,
	InfoLevel:     3,
}

// NewErrorSeverity creates a new error severity with validation.
func NewErrorSeverity(level string) (*ErrorSeverity, error) {
	if level == "" {
		return nil, errors.New("invalid error severity: cannot be empty")
	}

	if _, exists := validLevels[level]; !exists {
		return nil, fmt.Errorf("invalid error severity: %s is not a valid level", level)
	}

	return &ErrorSeverity{level: level}, nil
}

// String returns the string representation of the severity.
func (es *ErrorSeverity) String() string {
	return es.level
}

// IsCritical returns true if this is a critical error.
func (es *ErrorSeverity) IsCritical() bool {
	return es.level == CriticalLevel
}

// IsError returns true if this is an error level.
func (es *ErrorSeverity) IsError() bool {
	return es.level == ErrorLevel
}

// IsWarning returns true if this is a warning level.
func (es *ErrorSeverity) IsWarning() bool {
	return es.level == WarningLevel
}

// IsInfo returns true if this is an info level.
func (es *ErrorSeverity) IsInfo() bool {
	return es.level == InfoLevel
}

// Priority returns the numeric priority (0 = highest priority).
func (es *ErrorSeverity) Priority() int {
	return validLevels[es.level]
}

// IsHigherPriority returns true if this severity has higher priority than other.
func (es *ErrorSeverity) IsHigherPriority(other *ErrorSeverity) bool {
	return es.Priority() < other.Priority()
}

// RequiresImmediateAlert returns true if this severity requires immediate alerting.
func (es *ErrorSeverity) RequiresImmediateAlert() bool {
	return es.level == CriticalLevel || es.level == ErrorLevel
}

// Equals returns true if both severities are equal.
func (es *ErrorSeverity) Equals(other *ErrorSeverity) bool {
	return es.level == other.level
}

// MarshalJSON implements json.Marshaler.
func (es *ErrorSeverity) MarshalJSON() ([]byte, error) {
	return json.Marshal(es.level)
}

// UnmarshalJSON implements json.Unmarshaler.
func (es *ErrorSeverity) UnmarshalJSON(data []byte) error {
	var level string
	if err := json.Unmarshal(data, &level); err != nil {
		return err
	}

	severity, err := NewErrorSeverity(level)
	if err != nil {
		return err
	}

	es.level = severity.level
	return nil
}
