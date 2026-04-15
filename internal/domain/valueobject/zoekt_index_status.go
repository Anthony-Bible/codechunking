//nolint:dupl // similar structure to repository_status.go and embedding_index_status.go
package valueobject

import (
	"errors"
	"fmt"
)

// ZoektIndexStatus represents the indexing status of the Zoekt full-text search engine.
type ZoektIndexStatus string

// Zoekt index status constants.
const (
	ZoektIndexStatusPending   ZoektIndexStatus = "pending"
	ZoektIndexStatusIndexing  ZoektIndexStatus = "indexing"
	ZoektIndexStatusCompleted ZoektIndexStatus = "completed"
	ZoektIndexStatusFailed    ZoektIndexStatus = "failed"
	ZoektIndexStatusPartial   ZoektIndexStatus = "partial"
)

// validZoektIndexStatuses returns all valid Zoekt index statuses.
func validZoektIndexStatuses() map[ZoektIndexStatus]bool {
	return map[ZoektIndexStatus]bool{
		ZoektIndexStatusPending:   true,
		ZoektIndexStatusIndexing:  true,
		ZoektIndexStatusCompleted: true,
		ZoektIndexStatusFailed:    true,
		ZoektIndexStatusPartial:   true,
	}
}

// NewZoektIndexStatus creates a new ZoektIndexStatus with validation.
func NewZoektIndexStatus(status string) (ZoektIndexStatus, error) {
	s := ZoektIndexStatus(status)
	if !validZoektIndexStatuses()[s] {
		return "", fmt.Errorf("invalid zoekt index status: %s", status)
	}
	return s, nil
}

// String returns the string representation of the status.
func (s ZoektIndexStatus) String() string {
	return string(s)
}

// IsTerminal returns true if this status represents a final state.
func (s ZoektIndexStatus) IsTerminal() bool {
	return s == ZoektIndexStatusCompleted || s == ZoektIndexStatusFailed || s == ZoektIndexStatusPartial
}

// CanTransitionTo returns true if the status can transition to the target status.
func (s ZoektIndexStatus) CanTransitionTo(target ZoektIndexStatus) bool {
	transitions := map[ZoektIndexStatus][]ZoektIndexStatus{
		ZoektIndexStatusPending: {
			ZoektIndexStatusIndexing,
			ZoektIndexStatusFailed,
		},
		ZoektIndexStatusIndexing: {
			ZoektIndexStatusCompleted,
			ZoektIndexStatusPartial,
			ZoektIndexStatusFailed,
		},
		ZoektIndexStatusCompleted: {
			ZoektIndexStatusIndexing,  // For re-indexing
			ZoektIndexStatusCompleted, // Allow idempotent completion
		},
		ZoektIndexStatusFailed: {
			ZoektIndexStatusPending, // For retry
		},
		ZoektIndexStatusPartial: {
			ZoektIndexStatusIndexing, // For re-indexing to complete
			ZoektIndexStatusFailed,   // If subsequent attempts fail
		},
	}

	validTransitions, exists := transitions[s]
	if !exists {
		return false
	}

	for _, validTarget := range validTransitions {
		if target == validTarget {
			return true
		}
	}
	return false
}

// AllZoektIndexStatuses returns all valid Zoekt index statuses.
func AllZoektIndexStatuses() []ZoektIndexStatus {
	validStatuses := validZoektIndexStatuses()
	statuses := make([]ZoektIndexStatus, 0, len(validStatuses))
	for status := range validStatuses {
		statuses = append(statuses, status)
	}
	return statuses
}

// === Zoekt Index Status String Validation Functions ===

// checkZoektIndexStatusValidity performs the core validation logic for Zoekt index status strings.
func checkZoektIndexStatusValidity(status string) (bool, bool) {
	if status == "" {
		return true, false
	}
	return false, validZoektIndexStatuses()[ZoektIndexStatus(status)]
}

// IsValidZoektIndexStatusString validates whether a string represents a valid Zoekt index status.
// Empty strings are considered invalid.
func IsValidZoektIndexStatusString(status string) bool {
	isEmpty, isValid := checkZoektIndexStatusValidity(status)
	if isEmpty {
		return false
	}
	return isValid
}

// IsValidZoektIndexStatusStringWithEmpty validates whether a string represents a valid Zoekt index status.
// Empty strings are considered valid.
func IsValidZoektIndexStatusStringWithEmpty(status string) bool {
	isEmpty, isValid := checkZoektIndexStatusValidity(status)
	if isEmpty {
		return true
	}
	return isValid
}

// ValidateZoektIndexStatusString validates a Zoekt index status string and returns a descriptive error if invalid.
func ValidateZoektIndexStatusString(status string, allowEmpty bool) error {
	isEmpty, isValid := checkZoektIndexStatusValidity(status)

	if isEmpty {
		if allowEmpty {
			return nil
		}
		return errors.New("invalid zoekt index status: empty string not allowed")
	}

	if !isValid {
		return fmt.Errorf("invalid zoekt index status: %s", status)
	}

	return nil
}
