package valueobject

import (
	"testing"
)

func TestNewJobStatus_ValidStatuses(t *testing.T) {
	validStatuses := []struct {
		input    string
		expected JobStatus
	}{
		{"pending", JobStatusPending},
		{"running", JobStatusRunning},
		{"completed", JobStatusCompleted},
		{"failed", JobStatusFailed},
		{"cancelled", JobStatusCancelled},
	}

	for _, tc := range validStatuses {
		t.Run(tc.input, func(t *testing.T) {
			status, err := NewJobStatus(tc.input)
			if err != nil {
				t.Fatalf("Expected no error for valid status %s, got: %v", tc.input, err)
			}

			if status != tc.expected {
				t.Errorf("Expected status %s, got %s", tc.expected, status)
			}
		})
	}
}

func TestNewJobStatus_InvalidStatuses(t *testing.T) {
	invalidStatuses := []string{
		"invalid",
		"PENDING",   // case sensitive
		"Completed", // case sensitive
		"",          // empty string
		" pending",  // leading space
		"pending ",  // trailing space
		"unknown",
		"queued",       // not a valid job status
		"paused",       // not a valid job status
		"scheduled",    // not a valid job status
		"initializing", // not a valid job status
		"terminated",   // not a valid job status
	}

	for _, status := range invalidStatuses {
		t.Run(status, func(t *testing.T) {
			_, err := NewJobStatus(status)
			if err == nil {
				t.Fatalf("Expected error for invalid status %s, got none", status)
			}

			expectedError := "invalid job status: " + status
			if err.Error() != expectedError {
				t.Errorf("Expected error '%s', got '%v'", expectedError, err)
			}
		})
	}
}

func TestJobStatus_String(t *testing.T) {
	testCases := []struct {
		status   JobStatus
		expected string
	}{
		{JobStatusPending, "pending"},
		{JobStatusRunning, "running"},
		{JobStatusCompleted, "completed"},
		{JobStatusFailed, "failed"},
		{JobStatusCancelled, "cancelled"},
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

func TestJobStatus_IsTerminal(t *testing.T) {
	testCases := []struct {
		status     JobStatus
		isTerminal bool
	}{
		{JobStatusPending, false},
		{JobStatusRunning, false},
		{JobStatusCompleted, true},
		{JobStatusFailed, true},
		{JobStatusCancelled, true},
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

func TestJobStatus_CanTransitionTo_ValidTransitions(t *testing.T) {
	validTransitions := []struct {
		from JobStatus
		to   JobStatus
	}{
		// From pending
		{JobStatusPending, JobStatusRunning},
		{JobStatusPending, JobStatusCancelled},

		// From running
		{JobStatusRunning, JobStatusCompleted},
		{JobStatusRunning, JobStatusFailed},
		{JobStatusRunning, JobStatusCancelled},

		// Terminal states cannot transition (empty transitions)
		// Note: The implementation shows empty slices for terminal states
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

func TestJobStatus_CanTransitionTo_InvalidTransitions(t *testing.T) {
	invalidTransitions := []struct {
		from JobStatus
		to   JobStatus
	}{
		// Invalid from pending
		{JobStatusPending, JobStatusCompleted}, // must run first
		{JobStatusPending, JobStatusFailed},    // must run first

		// Invalid from running
		{JobStatusRunning, JobStatusPending}, // cannot go back

		// Terminal states cannot transition to anything
		{JobStatusCompleted, JobStatusPending},
		{JobStatusCompleted, JobStatusRunning},
		{JobStatusCompleted, JobStatusFailed},
		{JobStatusCompleted, JobStatusCancelled},

		{JobStatusFailed, JobStatusPending},
		{JobStatusFailed, JobStatusRunning},
		{JobStatusFailed, JobStatusCompleted},
		{JobStatusFailed, JobStatusCancelled},

		{JobStatusCancelled, JobStatusPending},
		{JobStatusCancelled, JobStatusRunning},
		{JobStatusCancelled, JobStatusCompleted},
		{JobStatusCancelled, JobStatusFailed},

		// Self-transitions should be invalid
		{JobStatusPending, JobStatusPending},
		{JobStatusRunning, JobStatusRunning},
		{JobStatusCompleted, JobStatusCompleted},
		{JobStatusFailed, JobStatusFailed},
		{JobStatusCancelled, JobStatusCancelled},
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

func TestJobStatus_CanTransitionTo_EdgeCases(t *testing.T) {
	// Test transition to/from invalid status
	t.Run("Invalid source status", func(t *testing.T) {
		invalidStatus := JobStatus("invalid")
		canTransition := invalidStatus.CanTransitionTo(JobStatusPending)
		if canTransition {
			t.Error("Expected invalid status to not allow transitions")
		}
	})

	t.Run("Invalid target status", func(t *testing.T) {
		invalidTarget := JobStatus("invalid")
		canTransition := JobStatusPending.CanTransitionTo(invalidTarget)
		if canTransition {
			t.Error("Expected transition to invalid status to be disallowed")
		}
	})

	t.Run("Empty status", func(t *testing.T) {
		emptyStatus := JobStatus("")
		canTransition := emptyStatus.CanTransitionTo(JobStatusPending)
		if canTransition {
			t.Error("Expected empty status to not allow transitions")
		}

		canTransition = JobStatusPending.CanTransitionTo(emptyStatus)
		if canTransition {
			t.Error("Expected transition to empty status to be disallowed")
		}
	})
}

func TestAllJobStatuses(t *testing.T) {
	allStatuses := AllJobStatuses()

	expectedCount := 5 // pending, running, completed, failed, cancelled
	if len(allStatuses) != expectedCount {
		t.Errorf("Expected %d statuses, got %d", expectedCount, len(allStatuses))
	}

	expectedStatuses := map[JobStatus]bool{
		JobStatusPending:   true,
		JobStatusRunning:   true,
		JobStatusCompleted: true,
		JobStatusFailed:    true,
		JobStatusCancelled: true,
	}

	for _, status := range allStatuses {
		if !expectedStatuses[status] {
			t.Errorf("Unexpected status in AllJobStatuses: %s", status)
		}
		delete(expectedStatuses, status)
	}

	if len(expectedStatuses) > 0 {
		t.Errorf("Missing statuses in AllJobStatuses: %v", expectedStatuses)
	}
}

func TestJobStatus_Constants(t *testing.T) {
	// Verify the constants have the expected string values
	testCases := []struct {
		constant JobStatus
		value    string
	}{
		{JobStatusPending, "pending"},
		{JobStatusRunning, "running"},
		{JobStatusCompleted, "completed"},
		{JobStatusFailed, "failed"},
		{JobStatusCancelled, "cancelled"},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			if string(tc.constant) != tc.value {
				t.Errorf("Expected constant to have value %s, got %s", tc.value, string(tc.constant))
			}
		})
	}
}

func TestJobStatus_TransitionChains(t *testing.T) {
	// Test complete transition chains
	t.Run("Happy path flow", func(t *testing.T) {
		// pending -> running -> completed
		status := JobStatusPending

		if !status.CanTransitionTo(JobStatusRunning) {
			t.Error("Should be able to transition from pending to running")
		}
		status = JobStatusRunning

		if !status.CanTransitionTo(JobStatusCompleted) {
			t.Error("Should be able to transition from running to completed")
		}

		// Completed is terminal - no further transitions
		status = JobStatusCompleted
		terminalTransitions := []JobStatus{
			JobStatusPending, JobStatusRunning, JobStatusFailed, JobStatusCancelled,
		}

		for _, target := range terminalTransitions {
			if status.CanTransitionTo(target) {
				t.Errorf("Terminal status %s should not be able to transition to %s", status, target)
			}
		}
	})

	t.Run("Failure path", func(t *testing.T) {
		// pending -> running -> failed
		status := JobStatusPending

		if !status.CanTransitionTo(JobStatusRunning) {
			t.Error("Should be able to transition from pending to running")
		}
		status = JobStatusRunning

		if !status.CanTransitionTo(JobStatusFailed) {
			t.Error("Should be able to transition from running to failed")
		}

		// Failed is terminal
		status = JobStatusFailed
		if status.CanTransitionTo(JobStatusPending) {
			t.Error("Failed status should not be able to transition back to pending")
		}
	})

	t.Run("Cancellation paths", func(t *testing.T) {
		// Can cancel from pending
		if !JobStatusPending.CanTransitionTo(JobStatusCancelled) {
			t.Error("Should be able to cancel from pending")
		}

		// Can cancel from running
		if !JobStatusRunning.CanTransitionTo(JobStatusCancelled) {
			t.Error("Should be able to cancel from running")
		}

		// Cannot cancel from terminal states
		terminalStates := []JobStatus{
			JobStatusCompleted,
			JobStatusFailed,
			JobStatusCancelled,
		}

		for _, terminal := range terminalStates {
			if terminal.CanTransitionTo(JobStatusCancelled) {
				t.Errorf("Should not be able to cancel from terminal state %s", terminal)
			}
		}
	})

	t.Run("No restart capability", func(t *testing.T) {
		// Unlike repository status, job status doesn't allow restarting
		terminalStates := []JobStatus{
			JobStatusCompleted,
			JobStatusFailed,
			JobStatusCancelled,
		}

		for _, terminal := range terminalStates {
			if terminal.CanTransitionTo(JobStatusPending) {
				t.Errorf("Terminal status %s should not be able to restart to pending", terminal)
			}
			if terminal.CanTransitionTo(JobStatusRunning) {
				t.Errorf("Terminal status %s should not be able to transition to running", terminal)
			}
		}
	})
}

func TestJobStatus_TerminalStateConsistency(t *testing.T) {
	// Ensure all terminal states are consistently handled
	terminalStates := []JobStatus{
		JobStatusCompleted,
		JobStatusFailed,
		JobStatusCancelled,
	}

	nonTerminalStates := []JobStatus{
		JobStatusPending,
		JobStatusRunning,
	}

	// Test that terminal states correctly identify as terminal
	for _, status := range terminalStates {
		t.Run(status.String()+"_is_terminal", func(t *testing.T) {
			if !status.IsTerminal() {
				t.Errorf("Status %s should be terminal", status)
			}
		})
	}

	// Test that non-terminal states correctly identify as non-terminal
	for _, status := range nonTerminalStates {
		t.Run(status.String()+"_is_not_terminal", func(t *testing.T) {
			if status.IsTerminal() {
				t.Errorf("Status %s should not be terminal", status)
			}
		})
	}

	// Test that terminal states cannot transition to any other state
	for _, terminal := range terminalStates {
		allStates := []JobStatus{
			JobStatusPending,
			JobStatusRunning,
			JobStatusCompleted,
			JobStatusFailed,
			JobStatusCancelled,
		}

		for _, target := range allStates {
			t.Run(terminal.String()+"_cannot_transition_to_"+target.String(), func(t *testing.T) {
				if terminal.CanTransitionTo(target) {
					t.Errorf("Terminal status %s should not be able to transition to %s", terminal, target)
				}
			})
		}
	}
}
