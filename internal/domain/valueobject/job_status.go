package valueobject

import "fmt"

// JobStatus represents the current status of an indexing job.
type JobStatus string

// Job status constants.
const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// validJobStatuses contains all valid job statuses.
var validJobStatuses = map[JobStatus]bool{
	JobStatusPending:   true,
	JobStatusRunning:   true,
	JobStatusCompleted: true,
	JobStatusFailed:    true,
	JobStatusCancelled: true,
}

// NewJobStatus creates a new JobStatus with validation.
func NewJobStatus(status string) (JobStatus, error) {
	s := JobStatus(status)
	if !validJobStatuses[s] {
		return "", fmt.Errorf("invalid job status: %s", status)
	}
	return s, nil
}

// String returns the string representation of the status.
func (s JobStatus) String() string {
	return string(s)
}

// IsTerminal returns true if this status represents a final state.
func (s JobStatus) IsTerminal() bool {
	return s == JobStatusCompleted || s == JobStatusFailed || s == JobStatusCancelled
}

// CanTransitionTo returns true if the status can transition to the target status.
func (s JobStatus) CanTransitionTo(target JobStatus) bool {
	transitions := map[JobStatus][]JobStatus{
		JobStatusPending: {
			JobStatusRunning,
			JobStatusCancelled,
		},
		JobStatusRunning: {
			JobStatusCompleted,
			JobStatusFailed,
			JobStatusCancelled,
		},
		// Terminal states cannot transition
		JobStatusCompleted: {},
		JobStatusFailed:    {},
		JobStatusCancelled: {},
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

// AllJobStatuses returns all valid job statuses.
func AllJobStatuses() []JobStatus {
	statuses := make([]JobStatus, 0, len(validJobStatuses))
	for status := range validJobStatuses {
		statuses = append(statuses, status)
	}
	return statuses
}
