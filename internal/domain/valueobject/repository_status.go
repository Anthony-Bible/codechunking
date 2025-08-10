package valueobject

import "fmt"

// RepositoryStatus represents the current status of a repository in the indexing pipeline
type RepositoryStatus string

// Repository status constants
const (
	RepositoryStatusPending    RepositoryStatus = "pending"
	RepositoryStatusCloning    RepositoryStatus = "cloning"
	RepositoryStatusProcessing RepositoryStatus = "processing"
	RepositoryStatusCompleted  RepositoryStatus = "completed"
	RepositoryStatusFailed     RepositoryStatus = "failed"
	RepositoryStatusArchived   RepositoryStatus = "archived"
)

// validStatuses contains all valid repository statuses
var validRepositoryStatuses = map[RepositoryStatus]bool{
	RepositoryStatusPending:    true,
	RepositoryStatusCloning:    true,
	RepositoryStatusProcessing: true,
	RepositoryStatusCompleted:  true,
	RepositoryStatusFailed:     true,
	RepositoryStatusArchived:   true,
}

// NewRepositoryStatus creates a new RepositoryStatus with validation
func NewRepositoryStatus(status string) (RepositoryStatus, error) {
	s := RepositoryStatus(status)
	if !validRepositoryStatuses[s] {
		return "", fmt.Errorf("invalid repository status: %s", status)
	}
	return s, nil
}

// String returns the string representation of the status
func (s RepositoryStatus) String() string {
	return string(s)
}

// IsTerminal returns true if this status represents a final state
func (s RepositoryStatus) IsTerminal() bool {
	return s == RepositoryStatusCompleted || s == RepositoryStatusFailed || s == RepositoryStatusArchived
}

// CanTransitionTo returns true if the status can transition to the target status
func (s RepositoryStatus) CanTransitionTo(target RepositoryStatus) bool {
	transitions := map[RepositoryStatus][]RepositoryStatus{
		RepositoryStatusPending: {
			RepositoryStatusCloning,
			RepositoryStatusFailed,
		},
		RepositoryStatusCloning: {
			RepositoryStatusProcessing,
			RepositoryStatusFailed,
		},
		RepositoryStatusProcessing: {
			RepositoryStatusCompleted,
			RepositoryStatusFailed,
		},
		RepositoryStatusCompleted: {
			RepositoryStatusProcessing, // For re-indexing
			RepositoryStatusArchived,
		},
		RepositoryStatusFailed: {
			RepositoryStatusPending, // For retry
			RepositoryStatusArchived,
		},
		RepositoryStatusArchived: {
			RepositoryStatusPending, // For restoration
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

// AllRepositoryStatuses returns all valid repository statuses
func AllRepositoryStatuses() []RepositoryStatus {
	statuses := make([]RepositoryStatus, 0, len(validRepositoryStatuses))
	for status := range validRepositoryStatuses {
		statuses = append(statuses, status)
	}
	return statuses
}

// === Repository Status String Validation Functions ===
//
// These functions provide flexible validation of repository status strings
// with different handling of empty strings based on use case requirements.

// checkRepositoryStatusValidity performs the core validation logic for repository status strings.
// It examines the input string and returns validation details to support different validation strategies.
//
// Returns:
//   - isEmpty: true if the status string is empty ("")
//   - isValidStatus: true if the status string matches a valid repository status (when not empty)
func checkRepositoryStatusValidity(status string) (isEmpty bool, isValidStatus bool) {
	if status == "" {
		return true, false
	}
	return false, validRepositoryStatuses[RepositoryStatus(status)]
}

// IsValidRepositoryStatusString validates whether a string represents a valid repository status.
// This function is strict about empty strings - they are considered invalid.
//
// Use this function when:
//   - A status value is required (not optional)
//   - Validating user input that must have a concrete status
//   - Ensuring a status field has been explicitly set
//
// Parameters:
//   - status: the string to validate
//
// Returns:
//   - true if the status is a valid, non-empty repository status
//   - false if the status is empty or invalid
func IsValidRepositoryStatusString(status string) bool {
	isEmpty, isValid := checkRepositoryStatusValidity(status)
	if isEmpty {
		return false // Empty strings are not valid for this function
	}
	return isValid
}

// IsValidRepositoryStatusStringWithEmpty validates whether a string represents a valid repository status.
// This function is lenient about empty strings - they are considered valid.
//
// Use this function when:
//   - A status field is optional
//   - Empty strings represent a valid "unset" or "default" state
//   - Validating input where missing status is acceptable
//
// Parameters:
//   - status: the string to validate
//
// Returns:
//   - true if the status is a valid repository status or empty
//   - false if the status is invalid (non-empty but not a recognized status)
func IsValidRepositoryStatusStringWithEmpty(status string) bool {
	isEmpty, isValid := checkRepositoryStatusValidity(status)
	if isEmpty {
		return true // Empty strings are valid for this function
	}
	return isValid
}

// ValidateRepositoryStatusString validates a repository status string and returns a descriptive error if invalid.
// This function provides detailed error information suitable for user feedback or logging.
//
// The validation behavior depends on the allowEmpty parameter:
//   - If allowEmpty is true: empty strings are considered valid (returns nil)
//   - If allowEmpty is false: empty strings are rejected with an error
//
// Use this function when:
//   - You need detailed error information for debugging or user feedback
//   - Different contexts require different empty string handling
//   - Building validation chains where errors need to be propagated
//
// Parameters:
//   - status: the string to validate
//   - allowEmpty: whether empty strings should be considered valid
//
// Returns:
//   - nil if the status is valid according to the allowEmpty setting
//   - error with descriptive message if the status is invalid
func ValidateRepositoryStatusString(status string, allowEmpty bool) error {
	isEmpty, isValid := checkRepositoryStatusValidity(status)

	if isEmpty {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("invalid repository status: empty string not allowed")
	}

	if !isValid {
		return fmt.Errorf("invalid repository status: %s", status)
	}

	return nil
}
