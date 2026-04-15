package valueobject

import (
	"testing"
)

func TestNewEmbeddingIndexStatus_ValidStatuses(t *testing.T) {
	validStatuses := []struct {
		input    string
		expected EmbeddingIndexStatus
	}{
		{"pending", EmbeddingIndexStatusPending},
		{"generating", EmbeddingIndexStatusGenerating},
		{"completed", EmbeddingIndexStatusCompleted},
		{"failed", EmbeddingIndexStatusFailed},
		{"partial", EmbeddingIndexStatusPartial},
	}

	for _, tc := range validStatuses {
		t.Run(tc.input, func(t *testing.T) {
			status, err := NewEmbeddingIndexStatus(tc.input)
			if err != nil {
				t.Fatalf("Expected no error for valid status %s, got: %v", tc.input, err)
			}
			if status != tc.expected {
				t.Errorf("Expected status %s, got %s", tc.expected, status)
			}
		})
	}
}

func TestNewEmbeddingIndexStatus_InvalidStatuses(t *testing.T) {
	invalidStatuses := []string{
		"invalid",
		"PENDING",
		"Completed",
		"",
		" pending",
		"pending ",
		"unknown",
		"indexing",
		"running",
	}

	for _, status := range invalidStatuses {
		t.Run(status, func(t *testing.T) {
			_, err := NewEmbeddingIndexStatus(status)
			if err == nil {
				t.Fatalf("Expected error for invalid status %q, got none", status)
			}
		})
	}
}

func TestEmbeddingIndexStatus_String(t *testing.T) {
	testCases := []struct {
		status   EmbeddingIndexStatus
		expected string
	}{
		{EmbeddingIndexStatusPending, "pending"},
		{EmbeddingIndexStatusGenerating, "generating"},
		{EmbeddingIndexStatusCompleted, "completed"},
		{EmbeddingIndexStatusFailed, "failed"},
		{EmbeddingIndexStatusPartial, "partial"},
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

func TestEmbeddingIndexStatus_IsTerminal(t *testing.T) {
	testCases := []struct {
		status     EmbeddingIndexStatus
		isTerminal bool
	}{
		{EmbeddingIndexStatusPending, false},
		{EmbeddingIndexStatusGenerating, false},
		{EmbeddingIndexStatusCompleted, true},
		{EmbeddingIndexStatusFailed, true},
		{EmbeddingIndexStatusPartial, true},
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

func TestEmbeddingIndexStatus_CanTransitionTo_ValidTransitions(t *testing.T) {
	validTransitions := []struct {
		from EmbeddingIndexStatus
		to   EmbeddingIndexStatus
	}{
		// From pending
		{EmbeddingIndexStatusPending, EmbeddingIndexStatusGenerating},
		{EmbeddingIndexStatusPending, EmbeddingIndexStatusFailed},

		// From generating
		{EmbeddingIndexStatusGenerating, EmbeddingIndexStatusCompleted},
		{EmbeddingIndexStatusGenerating, EmbeddingIndexStatusPartial},
		{EmbeddingIndexStatusGenerating, EmbeddingIndexStatusFailed},

		// From completed (re-indexing)
		{EmbeddingIndexStatusCompleted, EmbeddingIndexStatusGenerating},
		{EmbeddingIndexStatusCompleted, EmbeddingIndexStatusCompleted}, // idempotent

		// From failed (retry)
		{EmbeddingIndexStatusFailed, EmbeddingIndexStatusPending},

		// From partial
		{EmbeddingIndexStatusPartial, EmbeddingIndexStatusGenerating},
		{EmbeddingIndexStatusPartial, EmbeddingIndexStatusFailed},
	}

	for _, tc := range validTransitions {
		t.Run(tc.from.String()+"_to_"+tc.to.String(), func(t *testing.T) {
			if !tc.from.CanTransitionTo(tc.to) {
				t.Errorf("Expected transition from %s to %s to be valid, but it was not",
					tc.from, tc.to)
			}
		})
	}
}

func TestEmbeddingIndexStatus_CanTransitionTo_InvalidTransitions(t *testing.T) {
	invalidTransitions := []struct {
		from EmbeddingIndexStatus
		to   EmbeddingIndexStatus
	}{
		// Invalid from pending
		{EmbeddingIndexStatusPending, EmbeddingIndexStatusCompleted},
		{EmbeddingIndexStatusPending, EmbeddingIndexStatusPartial},
		{EmbeddingIndexStatusPending, EmbeddingIndexStatusPending},

		// Invalid from generating
		{EmbeddingIndexStatusGenerating, EmbeddingIndexStatusPending},
		{EmbeddingIndexStatusGenerating, EmbeddingIndexStatusGenerating},

		// Invalid from completed
		{EmbeddingIndexStatusCompleted, EmbeddingIndexStatusPending},
		{EmbeddingIndexStatusCompleted, EmbeddingIndexStatusFailed},
		{EmbeddingIndexStatusCompleted, EmbeddingIndexStatusPartial},

		// Invalid from failed
		{EmbeddingIndexStatusFailed, EmbeddingIndexStatusGenerating},
		{EmbeddingIndexStatusFailed, EmbeddingIndexStatusCompleted},
		{EmbeddingIndexStatusFailed, EmbeddingIndexStatusPartial},
		{EmbeddingIndexStatusFailed, EmbeddingIndexStatusFailed},

		// Invalid from partial
		{EmbeddingIndexStatusPartial, EmbeddingIndexStatusPending},
		{EmbeddingIndexStatusPartial, EmbeddingIndexStatusCompleted},
		{EmbeddingIndexStatusPartial, EmbeddingIndexStatusPartial},
	}

	for _, tc := range invalidTransitions {
		t.Run(tc.from.String()+"_to_"+tc.to.String(), func(t *testing.T) {
			if tc.from.CanTransitionTo(tc.to) {
				t.Errorf("Expected transition from %s to %s to be invalid, but it was allowed",
					tc.from, tc.to)
			}
		})
	}
}

func TestEmbeddingIndexStatus_CanTransitionTo_EdgeCases(t *testing.T) {
	t.Run("invalid_source_status", func(t *testing.T) {
		invalid := EmbeddingIndexStatus("invalid")
		if invalid.CanTransitionTo(EmbeddingIndexStatusPending) {
			t.Error("Expected invalid status to not allow any transitions")
		}
	})

	t.Run("invalid_target_status", func(t *testing.T) {
		invalid := EmbeddingIndexStatus("invalid")
		if EmbeddingIndexStatusPending.CanTransitionTo(invalid) {
			t.Error("Expected transition to invalid status to be disallowed")
		}
	})

	t.Run("empty_source_status", func(t *testing.T) {
		empty := EmbeddingIndexStatus("")
		if empty.CanTransitionTo(EmbeddingIndexStatusPending) {
			t.Error("Expected empty status to not allow transitions")
		}
	})

	t.Run("empty_target_status", func(t *testing.T) {
		empty := EmbeddingIndexStatus("")
		if EmbeddingIndexStatusPending.CanTransitionTo(empty) {
			t.Error("Expected transition to empty status to be disallowed")
		}
	})
}

func TestAllEmbeddingIndexStatuses(t *testing.T) {
	allStatuses := AllEmbeddingIndexStatuses()

	expectedCount := 5
	if len(allStatuses) != expectedCount {
		t.Errorf("Expected %d statuses, got %d", expectedCount, len(allStatuses))
	}

	expectedStatuses := map[EmbeddingIndexStatus]bool{
		EmbeddingIndexStatusPending:    true,
		EmbeddingIndexStatusGenerating: true,
		EmbeddingIndexStatusCompleted:  true,
		EmbeddingIndexStatusFailed:     true,
		EmbeddingIndexStatusPartial:    true,
	}

	for _, status := range allStatuses {
		if !expectedStatuses[status] {
			t.Errorf("Unexpected status in AllEmbeddingIndexStatuses: %s", status)
		}
		delete(expectedStatuses, status)
	}

	if len(expectedStatuses) > 0 {
		t.Errorf("Missing statuses in AllEmbeddingIndexStatuses: %v", expectedStatuses)
	}
}

func TestIsValidEmbeddingIndexStatusString(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"pending", true},
		{"generating", true},
		{"completed", true},
		{"failed", true},
		{"partial", true},
		{"", false},
		{"invalid", false},
		{"PENDING", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := IsValidEmbeddingIndexStatusString(tc.input)
			if result != tc.expected {
				t.Errorf("IsValidEmbeddingIndexStatusString(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsValidEmbeddingIndexStatusStringWithEmpty(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"pending", true},
		{"completed", true},
		{"", true}, // empty is valid
		{"invalid", false},
		{"PENDING", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := IsValidEmbeddingIndexStatusStringWithEmpty(tc.input)
			if result != tc.expected {
				t.Errorf("IsValidEmbeddingIndexStatusStringWithEmpty(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestValidateEmbeddingIndexStatusString(t *testing.T) {
	t.Run("valid status", func(t *testing.T) {
		err := ValidateEmbeddingIndexStatusString("pending", false)
		if err != nil {
			t.Errorf("Expected no error for valid status, got: %v", err)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		err := ValidateEmbeddingIndexStatusString("invalid", false)
		if err == nil {
			t.Error("Expected error for invalid status, got none")
		}
	})

	t.Run("empty status not allowed", func(t *testing.T) {
		err := ValidateEmbeddingIndexStatusString("", false)
		if err == nil {
			t.Error("Expected error for empty status when not allowed, got none")
		}
	})

	t.Run("empty status allowed", func(t *testing.T) {
		err := ValidateEmbeddingIndexStatusString("", true)
		if err != nil {
			t.Errorf("Expected no error for empty status when allowed, got: %v", err)
		}
	})
}

func TestEmbeddingIndexStatus_HappyPathFlow(t *testing.T) {
	// Test the complete successful workflow: pending -> generating -> completed
	from := EmbeddingIndexStatusPending
	if !from.CanTransitionTo(EmbeddingIndexStatusGenerating) {
		t.Error("Should be able to start generating from pending")
	}

	from = EmbeddingIndexStatusGenerating
	if !from.CanTransitionTo(EmbeddingIndexStatusCompleted) {
		t.Error("Should be able to complete generating")
	}
}

func TestEmbeddingIndexStatus_FailureAndRetry(t *testing.T) {
	// Test: pending -> generating -> failed -> pending (retry)
	from := EmbeddingIndexStatusGenerating
	if !from.CanTransitionTo(EmbeddingIndexStatusFailed) {
		t.Error("Should be able to fail from generating")
	}

	from = EmbeddingIndexStatusFailed
	if !from.CanTransitionTo(EmbeddingIndexStatusPending) {
		t.Error("Should be able to retry from failed")
	}
}

func TestEmbeddingIndexStatus_PartialAndRegenerate(t *testing.T) {
	// Test: generating -> partial -> generating (re-generate to complete)
	from := EmbeddingIndexStatusGenerating
	if !from.CanTransitionTo(EmbeddingIndexStatusPartial) {
		t.Error("Should be able to transition to partial from generating")
	}

	from = EmbeddingIndexStatusPartial
	if !from.CanTransitionTo(EmbeddingIndexStatusGenerating) {
		t.Error("Should be able to re-generate from partial")
	}
}

func TestEmbeddingIndexStatus_ReIndexing(t *testing.T) {
	// Test re-indexing capability: completed -> generating
	from := EmbeddingIndexStatusCompleted
	if !from.CanTransitionTo(EmbeddingIndexStatusGenerating) {
		t.Error("Should be able to re-generate from completed")
	}
}

func TestEmbeddingIndexStatus_IdempotentCompletion(t *testing.T) {
	// Test idempotent completion: completed -> completed
	from := EmbeddingIndexStatusCompleted
	if !from.CanTransitionTo(EmbeddingIndexStatusCompleted) {
		t.Error("Should allow idempotent completion (completed -> completed)")
	}
}
