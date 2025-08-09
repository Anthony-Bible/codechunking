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
