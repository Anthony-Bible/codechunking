//nolint:dupl // similar structure to repository_status.go and zoekt_index_status.go
package valueobject

import (
	"errors"
	"fmt"
)

// EmbeddingIndexStatus represents the indexing status of the embedding-based semantic search engine.
type EmbeddingIndexStatus string

// Embedding index status constants.
const (
	EmbeddingIndexStatusPending    EmbeddingIndexStatus = "pending"
	EmbeddingIndexStatusGenerating EmbeddingIndexStatus = "generating"
	EmbeddingIndexStatusCompleted  EmbeddingIndexStatus = "completed"
	EmbeddingIndexStatusFailed     EmbeddingIndexStatus = "failed"
	EmbeddingIndexStatusPartial    EmbeddingIndexStatus = "partial"
)

// validEmbeddingIndexStatuses returns all valid embedding index statuses.
func validEmbeddingIndexStatuses() map[EmbeddingIndexStatus]bool {
	return map[EmbeddingIndexStatus]bool{
		EmbeddingIndexStatusPending:    true,
		EmbeddingIndexStatusGenerating: true,
		EmbeddingIndexStatusCompleted:  true,
		EmbeddingIndexStatusFailed:     true,
		EmbeddingIndexStatusPartial:    true,
	}
}

// NewEmbeddingIndexStatus creates a new EmbeddingIndexStatus with validation.
func NewEmbeddingIndexStatus(status string) (EmbeddingIndexStatus, error) {
	s := EmbeddingIndexStatus(status)
	if !validEmbeddingIndexStatuses()[s] {
		return "", fmt.Errorf("invalid embedding index status: %s", status)
	}
	return s, nil
}

// String returns the string representation of the status.
func (s EmbeddingIndexStatus) String() string {
	return string(s)
}

// IsTerminal returns true if this status represents a final state.
func (s EmbeddingIndexStatus) IsTerminal() bool {
	return s == EmbeddingIndexStatusCompleted || s == EmbeddingIndexStatusFailed || s == EmbeddingIndexStatusPartial
}

// CanTransitionTo returns true if the status can transition to the target status.
func (s EmbeddingIndexStatus) CanTransitionTo(target EmbeddingIndexStatus) bool {
	transitions := map[EmbeddingIndexStatus][]EmbeddingIndexStatus{
		EmbeddingIndexStatusPending: {
			EmbeddingIndexStatusGenerating,
			EmbeddingIndexStatusFailed,
		},
		EmbeddingIndexStatusGenerating: {
			EmbeddingIndexStatusCompleted,
			EmbeddingIndexStatusPartial,
			EmbeddingIndexStatusFailed,
		},
		EmbeddingIndexStatusCompleted: {
			EmbeddingIndexStatusGenerating, // For re-indexing
			EmbeddingIndexStatusCompleted,  // Allow idempotent completion
		},
		EmbeddingIndexStatusFailed: {
			EmbeddingIndexStatusPending, // For retry
		},
		EmbeddingIndexStatusPartial: {
			EmbeddingIndexStatusGenerating, // For re-indexing to complete
			EmbeddingIndexStatusFailed,     // If subsequent attempts fail
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

// AllEmbeddingIndexStatuses returns all valid embedding index statuses.
func AllEmbeddingIndexStatuses() []EmbeddingIndexStatus {
	validStatuses := validEmbeddingIndexStatuses()
	statuses := make([]EmbeddingIndexStatus, 0, len(validStatuses))
	for status := range validStatuses {
		statuses = append(statuses, status)
	}
	return statuses
}

// === Embedding Index Status String Validation Functions ===

// checkEmbeddingIndexStatusValidity performs the core validation logic for embedding index status strings.
func checkEmbeddingIndexStatusValidity(status string) (bool, bool) {
	if status == "" {
		return true, false
	}
	return false, validEmbeddingIndexStatuses()[EmbeddingIndexStatus(status)]
}

// IsValidEmbeddingIndexStatusString validates whether a string represents a valid embedding index status.
// Empty strings are considered invalid.
func IsValidEmbeddingIndexStatusString(status string) bool {
	isEmpty, isValid := checkEmbeddingIndexStatusValidity(status)
	if isEmpty {
		return false
	}
	return isValid
}

// IsValidEmbeddingIndexStatusStringWithEmpty validates whether a string represents a valid embedding index status.
// Empty strings are considered valid.
func IsValidEmbeddingIndexStatusStringWithEmpty(status string) bool {
	isEmpty, isValid := checkEmbeddingIndexStatusValidity(status)
	if isEmpty {
		return true
	}
	return isValid
}

// ValidateEmbeddingIndexStatusString validates an embedding index status string and returns a descriptive error if invalid.
func ValidateEmbeddingIndexStatusString(status string, allowEmpty bool) error {
	isEmpty, isValid := checkEmbeddingIndexStatusValidity(status)

	if isEmpty {
		if allowEmpty {
			return nil
		}
		return errors.New("invalid embedding index status: empty string not allowed")
	}

	if !isValid {
		return fmt.Errorf("invalid embedding index status: %s", status)
	}

	return nil
}
