package valueobject

import (
	"testing"
)

// RED PHASE TDD TESTS - These tests are meant to fail until the functions are implemented
// Functions to be implemented:
// 1. IsValidRepositoryStatusString(status string) bool
// 2. IsValidRepositoryStatusStringWithEmpty(status string) bool
// 3. ValidateRepositoryStatusString(status string, allowEmpty bool) error

// Tests for IsValidRepositoryStatusString function (Red Phase - Function doesn't exist yet).
func TestIsValidRepositoryStatusString_ValidStatuses(t *testing.T) {
	validStatuses := []string{
		"pending",
		"cloning",
		"processing",
		"completed",
		"failed",
		"archived",
	}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			result := IsValidRepositoryStatusString(status)
			if !result {
				t.Errorf("Expected IsValidRepositoryStatusString(%s) to be true, got false", status)
			}
		})
	}
}

func TestIsValidRepositoryStatusString_InvalidStatuses(t *testing.T) {
	invalidStatuses := []string{
		"invalid",
		"PENDING",   // case sensitive
		"Completed", // case sensitive
		"unknown",
		"initializing", // not a valid status
		"paused",       // not a valid status
		"queued",       // not a valid status
		" pending",     // leading space
		"pending ",     // trailing space
		"pen ding",     // space in middle
	}

	for _, status := range invalidStatuses {
		t.Run(status, func(t *testing.T) {
			result := IsValidRepositoryStatusString(status)
			if result {
				t.Errorf("Expected IsValidRepositoryStatusString(%s) to be false, got true", status)
			}
		})
	}
}

func TestIsValidRepositoryStatusString_EmptyString(t *testing.T) {
	// This function should NOT allow empty string (unlike application layer)
	result := IsValidRepositoryStatusString("")
	if result {
		t.Error("Expected IsValidRepositoryStatusString(\"\") to be false, got true")
	}
}

func TestIsValidRepositoryStatusString_EdgeCases(t *testing.T) {
	edgeCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"nil_like_string", "nil", false},
		{"null_like_string", "null", false},
		{"zero_value", "0", false},
		{"boolean_false", "false", false},
		{"boolean_true", "true", false},
		{"numeric_string", "123", false},
		{"special_chars", "pending!", false},
		{"unicode_chars", "péndíng", false},
		{"tab_character", "pending\t", false},
		{"newline_character", "pending\n", false},
		{"carriage_return", "pending\r", false},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsValidRepositoryStatusString(tc.input)
			if result != tc.expected {
				t.Errorf("Expected IsValidRepositoryStatusString(%q) to be %v, got %v", tc.input, tc.expected, result)
			}
		})
	}
}

// Tests for IsValidRepositoryStatusStringWithEmpty function (Red Phase - Function doesn't exist yet).
func TestIsValidRepositoryStatusStringWithEmpty_ValidStatuses(t *testing.T) {
	validStatuses := []string{
		"pending",
		"cloning",
		"processing",
		"completed",
		"failed",
		"archived",
	}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			result := IsValidRepositoryStatusStringWithEmpty(status)
			if !result {
				t.Errorf("Expected IsValidRepositoryStatusStringWithEmpty(%s) to be true, got false", status)
			}
		})
	}
}

func TestIsValidRepositoryStatusStringWithEmpty_EmptyString(t *testing.T) {
	// This function SHOULD allow empty string (like application layer)
	result := IsValidRepositoryStatusStringWithEmpty("")
	if !result {
		t.Error("Expected IsValidRepositoryStatusStringWithEmpty(\"\") to be true, got false")
	}
}

func TestIsValidRepositoryStatusStringWithEmpty_InvalidStatuses(t *testing.T) {
	invalidStatuses := []string{
		"invalid",
		"PENDING",   // case sensitive
		"Completed", // case sensitive
		"unknown",
		"initializing", // not a valid status
		"paused",       // not a valid status
		"queued",       // not a valid status
		" pending",     // leading space
		"pending ",     // trailing space
		"pen ding",     // space in middle
	}

	for _, status := range invalidStatuses {
		t.Run(status, func(t *testing.T) {
			result := IsValidRepositoryStatusStringWithEmpty(status)
			if result {
				t.Errorf("Expected IsValidRepositoryStatusStringWithEmpty(%s) to be false, got true", status)
			}
		})
	}
}

func TestIsValidRepositoryStatusStringWithEmpty_EdgeCases(t *testing.T) {
	edgeCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty_string", "", true}, // This is the key difference from the non-empty version
		{"nil_like_string", "nil", false},
		{"null_like_string", "null", false},
		{"zero_value", "0", false},
		{"boolean_false", "false", false},
		{"boolean_true", "true", false},
		{"numeric_string", "123", false},
		{"special_chars", "pending!", false},
		{"unicode_chars", "péndíng", false},
		{"tab_character", "pending\t", false},
		{"newline_character", "pending\n", false},
		{"carriage_return", "pending\r", false},
		{"space_only", " ", false},
		{"tab_only", "\t", false},
		{"newline_only", "\n", false},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsValidRepositoryStatusStringWithEmpty(tc.input)
			if result != tc.expected {
				t.Errorf(
					"Expected IsValidRepositoryStatusStringWithEmpty(%q) to be %v, got %v",
					tc.input,
					tc.expected,
					result,
				)
			}
		})
	}
}

// Tests for ValidateRepositoryStatusString function (Red Phase - Function doesn't exist yet).
func TestValidateRepositoryStatusString_ValidStatuses_AllowEmptyFalse(t *testing.T) {
	validStatuses := []string{
		"pending",
		"cloning",
		"processing",
		"completed",
		"failed",
		"archived",
	}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			err := ValidateRepositoryStatusString(status, false)
			if err != nil {
				t.Errorf("Expected ValidateRepositoryStatusString(%s, false) to return nil, got: %v", status, err)
			}
		})
	}
}

func TestValidateRepositoryStatusString_ValidStatuses_AllowEmptyTrue(t *testing.T) {
	validStatuses := []string{
		"pending",
		"cloning",
		"processing",
		"completed",
		"failed",
		"archived",
	}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			err := ValidateRepositoryStatusString(status, true)
			if err != nil {
				t.Errorf("Expected ValidateRepositoryStatusString(%s, true) to return nil, got: %v", status, err)
			}
		})
	}
}

func TestValidateRepositoryStatusString_EmptyString_AllowEmptyFalse(t *testing.T) {
	err := ValidateRepositoryStatusString("", false)
	if err == nil {
		t.Error("Expected ValidateRepositoryStatusString(\"\", false) to return an error, got nil")
	}

	expectedErrorMessage := "invalid repository status: empty string not allowed"
	if err.Error() != expectedErrorMessage {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMessage, err.Error())
	}
}

func TestValidateRepositoryStatusString_EmptyString_AllowEmptyTrue(t *testing.T) {
	err := ValidateRepositoryStatusString("", true)
	if err != nil {
		t.Errorf("Expected ValidateRepositoryStatusString(\"\", true) to return nil, got: %v", err)
	}
}

func TestValidateRepositoryStatusString_InvalidStatuses_AllowEmptyFalse(t *testing.T) {
	invalidStatuses := []struct {
		status          string
		expectedMessage string
	}{
		{"invalid", "invalid repository status: invalid"},
		{"PENDING", "invalid repository status: PENDING"},
		{"Completed", "invalid repository status: Completed"},
		{"unknown", "invalid repository status: unknown"},
		{"initializing", "invalid repository status: initializing"},
		{"paused", "invalid repository status: paused"},
		{"queued", "invalid repository status: queued"},
		{" pending", "invalid repository status:  pending"},
		{"pending ", "invalid repository status: pending "},
		{"pen ding", "invalid repository status: pen ding"},
	}

	for _, tc := range invalidStatuses {
		t.Run(tc.status, func(t *testing.T) {
			err := ValidateRepositoryStatusString(tc.status, false)
			if err == nil {
				t.Errorf("Expected ValidateRepositoryStatusString(%s, false) to return an error, got nil", tc.status)
			}

			if err.Error() != tc.expectedMessage {
				t.Errorf("Expected error message '%s', got '%s'", tc.expectedMessage, err.Error())
			}
		})
	}
}

func TestValidateRepositoryStatusString_InvalidStatuses_AllowEmptyTrue(t *testing.T) {
	invalidStatuses := []struct {
		status          string
		expectedMessage string
	}{
		{"invalid", "invalid repository status: invalid"},
		{"PENDING", "invalid repository status: PENDING"},
		{"Completed", "invalid repository status: Completed"},
		{"unknown", "invalid repository status: unknown"},
		{"initializing", "invalid repository status: initializing"},
		{"paused", "invalid repository status: paused"},
		{"queued", "invalid repository status: queued"},
		{" pending", "invalid repository status:  pending"},
		{"pending ", "invalid repository status: pending "},
		{"pen ding", "invalid repository status: pen ding"},
	}

	for _, tc := range invalidStatuses {
		t.Run(tc.status, func(t *testing.T) {
			err := ValidateRepositoryStatusString(tc.status, true)
			if err == nil {
				t.Errorf("Expected ValidateRepositoryStatusString(%s, true) to return an error, got nil", tc.status)
			}

			if err.Error() != tc.expectedMessage {
				t.Errorf("Expected error message '%s', got '%s'", tc.expectedMessage, err.Error())
			}
		})
	}
}

func TestValidateRepositoryStatusString_EdgeCases_AllowEmptyFalse(t *testing.T) {
	edgeCases := []struct {
		name            string
		input           string
		expectedMessage string
	}{
		{"nil_like_string", "nil", "invalid repository status: nil"},
		{"null_like_string", "null", "invalid repository status: null"},
		{"zero_value", "0", "invalid repository status: 0"},
		{"boolean_false", "false", "invalid repository status: false"},
		{"boolean_true", "true", "invalid repository status: true"},
		{"numeric_string", "123", "invalid repository status: 123"},
		{"special_chars", "pending!", "invalid repository status: pending!"},
		{"unicode_chars", "péndíng", "invalid repository status: péndíng"},
		{"tab_character", "pending\t", "invalid repository status: pending\t"},
		{"newline_character", "pending\n", "invalid repository status: pending\n"},
		{"carriage_return", "pending\r", "invalid repository status: pending\r"},
		{"space_only", " ", "invalid repository status:  "},
		{"tab_only", "\t", "invalid repository status: \t"},
		{"newline_only", "\n", "invalid repository status: \n"},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRepositoryStatusString(tc.input, false)
			if err == nil {
				t.Errorf("Expected ValidateRepositoryStatusString(%q, false) to return an error, got nil", tc.input)
			}

			if err.Error() != tc.expectedMessage {
				t.Errorf("Expected error message '%s', got '%s'", tc.expectedMessage, err.Error())
			}
		})
	}
}

func TestValidateRepositoryStatusString_EdgeCases_AllowEmptyTrue(t *testing.T) {
	edgeCases := []struct {
		name            string
		input           string
		expectError     bool
		expectedMessage string
	}{
		{"empty_string", "", false, ""}, // Should be valid when allowEmpty=true
		{"nil_like_string", "nil", true, "invalid repository status: nil"},
		{"null_like_string", "null", true, "invalid repository status: null"},
		{"zero_value", "0", true, "invalid repository status: 0"},
		{"boolean_false", "false", true, "invalid repository status: false"},
		{"boolean_true", "true", true, "invalid repository status: true"},
		{"numeric_string", "123", true, "invalid repository status: 123"},
		{"special_chars", "pending!", true, "invalid repository status: pending!"},
		{"unicode_chars", "péndíng", true, "invalid repository status: péndíng"},
		{"tab_character", "pending\t", true, "invalid repository status: pending\t"},
		{"newline_character", "pending\n", true, "invalid repository status: pending\n"},
		{"carriage_return", "pending\r", true, "invalid repository status: pending\r"},
		{"space_only", " ", true, "invalid repository status:  "},
		{"tab_only", "\t", true, "invalid repository status: \t"},
		{"newline_only", "\n", true, "invalid repository status: \n"},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRepositoryStatusString(tc.input, true)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected ValidateRepositoryStatusString(%q, true) to return an error, got nil", tc.input)
				} else if err.Error() != tc.expectedMessage {
					t.Errorf("Expected error message '%s', got '%s'", tc.expectedMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected ValidateRepositoryStatusString(%q, true) to return nil, got: %v", tc.input, err)
				}
			}
		})
	}
}

func TestValidateRepositoryStatusString_ErrorType(t *testing.T) {
	// Test that the function returns the correct error type
	// This ensures consistency with existing domain error patterns
	err := ValidateRepositoryStatusString("invalid", false)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// The error should be a standard Go error with descriptive message
	// Following the pattern from NewRepositoryStatus function
	expectedError := "invalid repository status: invalid"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%v'", expectedError, err)
	}
}
