package valueobject

import (
	"testing"
)

func TestNewZoektIndexStatus_ValidStatuses(t *testing.T) {
	validStatuses := []struct {
		input    string
		expected ZoektIndexStatus
	}{
		{"pending", ZoektIndexStatusPending},
		{"indexing", ZoektIndexStatusIndexing},
		{"completed", ZoektIndexStatusCompleted},
		{"failed", ZoektIndexStatusFailed},
		{"partial", ZoektIndexStatusPartial},
	}

	for _, tc := range validStatuses {
		t.Run(tc.input, func(t *testing.T) {
			status, err := NewZoektIndexStatus(tc.input)
			if err != nil {
				t.Fatalf("Expected no error for valid status %s, got: %v", tc.input, err)
			}
			if status != tc.expected {
				t.Errorf("Expected status %s, got %s", tc.expected, status)
			}
		})
	}
}

func TestNewZoektIndexStatus_InvalidStatuses(t *testing.T) {
	invalidStatuses := []string{
		"invalid",
		"PENDING",
		"Completed",
		"",
		" pending",
		"pending ",
		"unknown",
		"queued",
		"running",
	}

	for _, status := range invalidStatuses {
		t.Run(status, func(t *testing.T) {
			_, err := NewZoektIndexStatus(status)
			if err == nil {
				t.Fatalf("Expected error for invalid status %q, got none", status)
			}
		})
	}
}

func TestZoektIndexStatus_String(t *testing.T) {
	testCases := []struct {
		status   ZoektIndexStatus
		expected string
	}{
		{ZoektIndexStatusPending, "pending"},
		{ZoektIndexStatusIndexing, "indexing"},
		{ZoektIndexStatusCompleted, "completed"},
		{ZoektIndexStatusFailed, "failed"},
		{ZoektIndexStatusPartial, "partial"},
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

func TestZoektIndexStatus_IsTerminal(t *testing.T) {
	testCases := []struct {
		status     ZoektIndexStatus
		isTerminal bool
	}{
		{ZoektIndexStatusPending, false},
		{ZoektIndexStatusIndexing, false},
		{ZoektIndexStatusCompleted, true},
		{ZoektIndexStatusFailed, true},
		{ZoektIndexStatusPartial, true},
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

func TestZoektIndexStatus_CanTransitionTo_ValidTransitions(t *testing.T) {
	validTransitions := []struct {
		from ZoektIndexStatus
		to   ZoektIndexStatus
	}{
		// From pending
		{ZoektIndexStatusPending, ZoektIndexStatusIndexing},
		{ZoektIndexStatusPending, ZoektIndexStatusFailed},

		// From indexing
		{ZoektIndexStatusIndexing, ZoektIndexStatusCompleted},
		{ZoektIndexStatusIndexing, ZoektIndexStatusPartial},
		{ZoektIndexStatusIndexing, ZoektIndexStatusFailed},

		// From completed (re-indexing)
		{ZoektIndexStatusCompleted, ZoektIndexStatusIndexing},
		{ZoektIndexStatusCompleted, ZoektIndexStatusCompleted}, // idempotent

		// From failed (retry)
		{ZoektIndexStatusFailed, ZoektIndexStatusPending},

		// From partial
		{ZoektIndexStatusPartial, ZoektIndexStatusIndexing},
		{ZoektIndexStatusPartial, ZoektIndexStatusFailed},
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

func TestZoektIndexStatus_CanTransitionTo_InvalidTransitions(t *testing.T) {
	invalidTransitions := []struct {
		from ZoektIndexStatus
		to   ZoektIndexStatus
	}{
		// Invalid from pending
		{ZoektIndexStatusPending, ZoektIndexStatusCompleted},
		{ZoektIndexStatusPending, ZoektIndexStatusPartial},
		{ZoektIndexStatusPending, ZoektIndexStatusPending},

		// Invalid from indexing
		{ZoektIndexStatusIndexing, ZoektIndexStatusPending},
		{ZoektIndexStatusIndexing, ZoektIndexStatusIndexing},

		// Invalid from completed
		{ZoektIndexStatusCompleted, ZoektIndexStatusPending},
		{ZoektIndexStatusCompleted, ZoektIndexStatusFailed},
		{ZoektIndexStatusCompleted, ZoektIndexStatusPartial},

		// Invalid from failed
		{ZoektIndexStatusFailed, ZoektIndexStatusIndexing},
		{ZoektIndexStatusFailed, ZoektIndexStatusCompleted},
		{ZoektIndexStatusFailed, ZoektIndexStatusPartial},
		{ZoektIndexStatusFailed, ZoektIndexStatusFailed},

		// Invalid from partial
		{ZoektIndexStatusPartial, ZoektIndexStatusPending},
		{ZoektIndexStatusPartial, ZoektIndexStatusCompleted},
		{ZoektIndexStatusPartial, ZoektIndexStatusPartial},
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

func TestZoektIndexStatus_CanTransitionTo_EdgeCases(t *testing.T) {
	t.Run("invalid_source_status", func(t *testing.T) {
		invalid := ZoektIndexStatus("invalid")
		if invalid.CanTransitionTo(ZoektIndexStatusPending) {
			t.Error("Expected invalid status to not allow any transitions")
		}
	})

	t.Run("invalid_target_status", func(t *testing.T) {
		invalid := ZoektIndexStatus("invalid")
		if ZoektIndexStatusPending.CanTransitionTo(invalid) {
			t.Error("Expected transition to invalid status to be disallowed")
		}
	})

	t.Run("empty_source_status", func(t *testing.T) {
		empty := ZoektIndexStatus("")
		if empty.CanTransitionTo(ZoektIndexStatusPending) {
			t.Error("Expected empty status to not allow transitions")
		}
	})

	t.Run("empty_target_status", func(t *testing.T) {
		empty := ZoektIndexStatus("")
		if ZoektIndexStatusPending.CanTransitionTo(empty) {
			t.Error("Expected transition to empty status to be disallowed")
		}
	})
}

func TestAllZoektIndexStatuses(t *testing.T) {
	allStatuses := AllZoektIndexStatuses()

	expectedCount := 5
	if len(allStatuses) != expectedCount {
		t.Errorf("Expected %d statuses, got %d", expectedCount, len(allStatuses))
	}

	expectedStatuses := map[ZoektIndexStatus]bool{
		ZoektIndexStatusPending:   true,
		ZoektIndexStatusIndexing:  true,
		ZoektIndexStatusCompleted: true,
		ZoektIndexStatusFailed:    true,
		ZoektIndexStatusPartial:   true,
	}

	for _, status := range allStatuses {
		if !expectedStatuses[status] {
			t.Errorf("Unexpected status in AllZoektIndexStatuses: %s", status)
		}
		delete(expectedStatuses, status)
	}

	if len(expectedStatuses) > 0 {
		t.Errorf("Missing statuses in AllZoektIndexStatuses: %v", expectedStatuses)
	}
}

func TestIsValidZoektIndexStatusString(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"pending", true},
		{"indexing", true},
		{"completed", true},
		{"failed", true},
		{"partial", true},
		{"", false},
		{"invalid", false},
		{"PENDING", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := IsValidZoektIndexStatusString(tc.input)
			if result != tc.expected {
				t.Errorf("IsValidZoektIndexStatusString(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsValidZoektIndexStatusStringWithEmpty(t *testing.T) {
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
			result := IsValidZoektIndexStatusStringWithEmpty(tc.input)
			if result != tc.expected {
				t.Errorf("IsValidZoektIndexStatusStringWithEmpty(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestValidateZoektIndexStatusString(t *testing.T) {
	t.Run("valid status", func(t *testing.T) {
		err := ValidateZoektIndexStatusString("pending", false)
		if err != nil {
			t.Errorf("Expected no error for valid status, got: %v", err)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		err := ValidateZoektIndexStatusString("invalid", false)
		if err == nil {
			t.Error("Expected error for invalid status, got none")
		}
	})

	t.Run("empty status not allowed", func(t *testing.T) {
		err := ValidateZoektIndexStatusString("", false)
		if err == nil {
			t.Error("Expected error for empty status when not allowed, got none")
		}
	})

	t.Run("empty status allowed", func(t *testing.T) {
		err := ValidateZoektIndexStatusString("", true)
		if err != nil {
			t.Errorf("Expected no error for empty status when allowed, got: %v", err)
		}
	})
}

func TestZoektIndexStatus_HappyPathFlow(t *testing.T) {
	// Test the complete successful workflow: pending -> indexing -> completed
	from := ZoektIndexStatusPending
	if !from.CanTransitionTo(ZoektIndexStatusIndexing) {
		t.Error("Should be able to start indexing from pending")
	}

	from = ZoektIndexStatusIndexing
	if !from.CanTransitionTo(ZoektIndexStatusCompleted) {
		t.Error("Should be able to complete indexing")
	}
}

func TestZoektIndexStatus_FailureAndRetry(t *testing.T) {
	// Test: pending -> indexing -> failed -> pending (retry)
	from := ZoektIndexStatusIndexing
	if !from.CanTransitionTo(ZoektIndexStatusFailed) {
		t.Error("Should be able to fail from indexing")
	}

	from = ZoektIndexStatusFailed
	if !from.CanTransitionTo(ZoektIndexStatusPending) {
		t.Error("Should be able to retry from failed")
	}
}

func TestZoektIndexStatus_PartialAndReindex(t *testing.T) {
	// Test: indexing -> partial -> indexing (re-index to complete)
	from := ZoektIndexStatusIndexing
	if !from.CanTransitionTo(ZoektIndexStatusPartial) {
		t.Error("Should be able to transition to partial from indexing")
	}

	from = ZoektIndexStatusPartial
	if !from.CanTransitionTo(ZoektIndexStatusIndexing) {
		t.Error("Should be able to re-index from partial")
	}
}
