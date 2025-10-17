package valueobject

import (
	"testing"
)

func TestNewRepositoryStatus_ValidStatuses(t *testing.T) {
	validStatuses := []struct {
		input    string
		expected RepositoryStatus
	}{
		{"pending", RepositoryStatusPending},
		{"cloning", RepositoryStatusCloning},
		{"processing", RepositoryStatusProcessing},
		{"completed", RepositoryStatusCompleted},
		{"failed", RepositoryStatusFailed},
		{"archived", RepositoryStatusArchived},
	}

	for _, tc := range validStatuses {
		t.Run(tc.input, func(t *testing.T) {
			status, err := NewRepositoryStatus(tc.input)
			if err != nil {
				t.Fatalf("Expected no error for valid status %s, got: %v", tc.input, err)
			}

			if status != tc.expected {
				t.Errorf("Expected status %s, got %s", tc.expected, status)
			}
		})
	}
}

func TestNewRepositoryStatus_InvalidStatuses(t *testing.T) {
	invalidStatuses := []string{
		"invalid",
		"PENDING",   // case sensitive
		"Completed", // case sensitive
		"",          // empty string
		" pending",  // leading space
		"pending ",  // trailing space
		"unknown",
		"initializing", // not a valid status
		"paused",       // not a valid status
		"queued",       // not a valid status
	}

	for _, status := range invalidStatuses {
		t.Run(status, func(t *testing.T) {
			_, err := NewRepositoryStatus(status)
			if err == nil {
				t.Fatalf("Expected error for invalid status %s, got none", status)
			}

			expectedError := "invalid repository status: " + status
			if err.Error() != expectedError {
				t.Errorf("Expected error '%s', got '%v'", expectedError, err)
			}
		})
	}
}

func TestRepositoryStatus_String(t *testing.T) {
	testCases := []struct {
		status   RepositoryStatus
		expected string
	}{
		{RepositoryStatusPending, "pending"},
		{RepositoryStatusCloning, "cloning"},
		{RepositoryStatusProcessing, "processing"},
		{RepositoryStatusCompleted, "completed"},
		{RepositoryStatusFailed, "failed"},
		{RepositoryStatusArchived, "archived"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.status.String()
			if result != tc.expected {
				t.Errorf("Expected string %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestRepositoryStatus_IsTerminal(t *testing.T) {
	testCases := []struct {
		status     RepositoryStatus
		isTerminal bool
	}{
		{RepositoryStatusPending, false},
		{RepositoryStatusCloning, false},
		{RepositoryStatusProcessing, false},
		{RepositoryStatusCompleted, true},
		{RepositoryStatusFailed, true},
		{RepositoryStatusArchived, true},
	}

	for _, tc := range testCases {
		t.Run(tc.status.String(), func(t *testing.T) {
			result := tc.status.IsTerminal()
			if result != tc.isTerminal {
				t.Errorf("Expected IsTerminal() to be %v for status %s, got %v",
					tc.isTerminal, tc.status, result)
			}
		})
	}
}

func TestRepositoryStatus_CanTransitionTo_ValidTransitions(t *testing.T) {
	validTransitions := []struct {
		from RepositoryStatus
		to   RepositoryStatus
	}{
		// From pending
		{RepositoryStatusPending, RepositoryStatusCloning},
		{RepositoryStatusPending, RepositoryStatusFailed},

		// From cloning
		{RepositoryStatusCloning, RepositoryStatusProcessing},
		{RepositoryStatusCloning, RepositoryStatusFailed},

		// From processing
		{RepositoryStatusProcessing, RepositoryStatusCompleted},
		{RepositoryStatusProcessing, RepositoryStatusFailed},

		// From completed
		{RepositoryStatusCompleted, RepositoryStatusProcessing}, // re-indexing
		{RepositoryStatusCompleted, RepositoryStatusCompleted},  // idempotent completion (re-indexing same commit)
		{RepositoryStatusCompleted, RepositoryStatusArchived},

		// From failed
		{RepositoryStatusFailed, RepositoryStatusPending}, // retry
		{RepositoryStatusFailed, RepositoryStatusArchived},

		// From archived
		{RepositoryStatusArchived, RepositoryStatusPending}, // restoration
	}

	for _, tc := range validTransitions {
		t.Run(tc.from.String()+"_to_"+tc.to.String(), func(t *testing.T) {
			canTransition := tc.from.CanTransitionTo(tc.to)
			if !canTransition {
				t.Errorf("Expected transition from %s to %s to be valid, but it was not",
					tc.from, tc.to)
			}
		})
	}
}

func TestRepositoryStatus_CanTransitionTo_InvalidTransitions(t *testing.T) {
	invalidTransitions := []struct {
		from RepositoryStatus
		to   RepositoryStatus
	}{
		// Invalid from pending
		{RepositoryStatusPending, RepositoryStatusProcessing}, // must go through cloning
		{RepositoryStatusPending, RepositoryStatusCompleted},
		{RepositoryStatusPending, RepositoryStatusArchived},

		// Invalid from cloning
		{RepositoryStatusCloning, RepositoryStatusPending},
		{RepositoryStatusCloning, RepositoryStatusCompleted}, // must go through processing
		{RepositoryStatusCloning, RepositoryStatusArchived},

		// Invalid from processing
		{RepositoryStatusProcessing, RepositoryStatusPending},
		{RepositoryStatusProcessing, RepositoryStatusCloning},
		{RepositoryStatusProcessing, RepositoryStatusArchived},

		// Invalid from completed
		{RepositoryStatusCompleted, RepositoryStatusPending},
		{RepositoryStatusCompleted, RepositoryStatusCloning},
		{RepositoryStatusCompleted, RepositoryStatusFailed},

		// Invalid from failed
		{RepositoryStatusFailed, RepositoryStatusCloning},
		{RepositoryStatusFailed, RepositoryStatusProcessing},
		{RepositoryStatusFailed, RepositoryStatusCompleted},

		// Invalid from archived
		{RepositoryStatusArchived, RepositoryStatusCloning},
		{RepositoryStatusArchived, RepositoryStatusProcessing},
		{RepositoryStatusArchived, RepositoryStatusCompleted},
		{RepositoryStatusArchived, RepositoryStatusFailed},

		// Self-transitions (should be invalid, except completed->completed for idempotent re-indexing)
		{RepositoryStatusPending, RepositoryStatusPending},
		{RepositoryStatusCloning, RepositoryStatusCloning},
		{RepositoryStatusProcessing, RepositoryStatusProcessing},
		{RepositoryStatusFailed, RepositoryStatusFailed},
		{RepositoryStatusArchived, RepositoryStatusArchived},
	}

	for _, tc := range invalidTransitions {
		t.Run(tc.from.String()+"_to_"+tc.to.String(), func(t *testing.T) {
			canTransition := tc.from.CanTransitionTo(tc.to)
			if canTransition {
				t.Errorf("Expected transition from %s to %s to be invalid, but it was allowed",
					tc.from, tc.to)
			}
		})
	}
}

func TestRepositoryStatus_CanTransitionTo_EdgeCases(t *testing.T) {
	// Test transition to/from invalid status
	t.Run("Invalid source status", func(t *testing.T) {
		invalidStatus := RepositoryStatus("invalid")
		canTransition := invalidStatus.CanTransitionTo(RepositoryStatusPending)
		if canTransition {
			t.Error("Expected invalid status to not allow transitions")
		}
	})

	t.Run("Invalid target status", func(t *testing.T) {
		invalidTarget := RepositoryStatus("invalid")
		canTransition := RepositoryStatusPending.CanTransitionTo(invalidTarget)
		if canTransition {
			t.Error("Expected transition to invalid status to be disallowed")
		}
	})

	t.Run("Empty status", func(t *testing.T) {
		emptyStatus := RepositoryStatus("")
		canTransition := emptyStatus.CanTransitionTo(RepositoryStatusPending)
		if canTransition {
			t.Error("Expected empty status to not allow transitions")
		}

		canTransition = RepositoryStatusPending.CanTransitionTo(emptyStatus)
		if canTransition {
			t.Error("Expected transition to empty status to be disallowed")
		}
	})
}

func TestAllRepositoryStatuses(t *testing.T) {
	allStatuses := AllRepositoryStatuses()

	expectedCount := 6 // pending, cloning, processing, completed, failed, archived
	if len(allStatuses) != expectedCount {
		t.Errorf("Expected %d statuses, got %d", expectedCount, len(allStatuses))
	}

	expectedStatuses := map[RepositoryStatus]bool{
		RepositoryStatusPending:    true,
		RepositoryStatusCloning:    true,
		RepositoryStatusProcessing: true,
		RepositoryStatusCompleted:  true,
		RepositoryStatusFailed:     true,
		RepositoryStatusArchived:   true,
	}

	for _, status := range allStatuses {
		if !expectedStatuses[status] {
			t.Errorf("Unexpected status in AllRepositoryStatuses: %s", status)
		}
		delete(expectedStatuses, status)
	}

	if len(expectedStatuses) > 0 {
		t.Errorf("Missing statuses in AllRepositoryStatuses: %v", expectedStatuses)
	}
}

func TestRepositoryStatus_Constants(t *testing.T) {
	// Verify the constants have the expected string values
	testCases := []struct {
		constant RepositoryStatus
		value    string
	}{
		{RepositoryStatusPending, "pending"},
		{RepositoryStatusCloning, "cloning"},
		{RepositoryStatusProcessing, "processing"},
		{RepositoryStatusCompleted, "completed"},
		{RepositoryStatusFailed, "failed"},
		{RepositoryStatusArchived, "archived"},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			if string(tc.constant) != tc.value {
				t.Errorf("Expected constant to have value %s, got %s", tc.value, string(tc.constant))
			}
		})
	}
}

// Helper function to assert valid repository status transition.
func assertRepoStatusTransition(t *testing.T, from, to RepositoryStatus, operation string) {
	t.Helper()
	if !from.CanTransitionTo(to) {
		t.Errorf("Should be able to %s: transition from %s to %s", operation, from, to)
	}
}

// Helper function to test transitions for multiple repository statuses.
func testMultipleRepoTransitions(t *testing.T, statuses []RepositoryStatus, target RepositoryStatus, operation string) {
	t.Helper()
	for _, status := range statuses {
		t.Run(status.String()+"_to_"+target.String(), func(t *testing.T) {
			assertRepoStatusTransition(t, status, target, operation)
		})
	}
}

func TestRepositoryStatus_HappyPathFlow(t *testing.T) {
	// Test the complete successful workflow: pending -> cloning -> processing -> completed -> archived
	status := RepositoryStatusPending

	assertRepoStatusTransition(t, status, RepositoryStatusCloning, "start cloning")
	status = RepositoryStatusCloning

	assertRepoStatusTransition(t, status, RepositoryStatusProcessing, "start processing")
	status = RepositoryStatusProcessing

	assertRepoStatusTransition(t, status, RepositoryStatusCompleted, "complete processing")
	status = RepositoryStatusCompleted

	assertRepoStatusTransition(t, status, RepositoryStatusArchived, "archive completed repository")
}

func TestRepositoryStatus_FailureTransitions(t *testing.T) {
	// Test failure transitions from non-terminal states
	failableStages := []RepositoryStatus{
		RepositoryStatusPending,
		RepositoryStatusCloning,
		RepositoryStatusProcessing,
	}

	testMultipleRepoTransitions(t, failableStages, RepositoryStatusFailed, "fail")
}

func TestRepositoryStatus_RetryAfterFailure(t *testing.T) {
	// Test retry capability: failed -> pending
	assertRepoStatusTransition(t, RepositoryStatusFailed, RepositoryStatusPending, "retry after failure")
}

func TestRepositoryStatus_ReIndexing(t *testing.T) {
	// Test re-indexing capability: completed -> processing
	assertRepoStatusTransition(t, RepositoryStatusCompleted, RepositoryStatusProcessing, "re-index")
}

func TestRepositoryStatus_ArchiveAndRestore(t *testing.T) {
	// Test archiving from terminal states
	terminalStates := []RepositoryStatus{
		RepositoryStatusCompleted,
		RepositoryStatusFailed,
	}

	testMultipleRepoTransitions(t, terminalStates, RepositoryStatusArchived, "archive")

	// Test restoration: archived -> pending
	assertRepoStatusTransition(t, RepositoryStatusArchived, RepositoryStatusPending, "restore from archive")
}
