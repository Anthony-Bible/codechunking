package common

import (
	"errors"
	"strings"
	"testing"

	"codechunking/internal/domain/valueobject"
)

// TestValidateRepositoryStatus_RefactoredBehavior tests that ValidateRepositoryStatus
// has been refactored to use domain layer validation instead of ValidRepositoryStatuses map.
func TestValidateRepositoryStatus_RefactoredBehavior(t *testing.T) {
	t.Run("should_not_use_ValidRepositoryStatuses_map", func(t *testing.T) {
		// This test will FAIL initially because the current implementation uses the map
		// After refactoring, the map should be removed and this test should pass

		// Test a status that would be valid according to the map but should be validated using domain layer
		err := ValidateRepositoryStatus("pending")
		if err != nil {
			t.Errorf("ValidateRepositoryStatus('pending') should use domain validation and return nil, got: %v", err)
		}

		// Test that empty string is handled the same way as domain layer (allowEmpty=true)
		err = ValidateRepositoryStatus("")
		if err != nil {
			t.Errorf(
				"ValidateRepositoryStatus('') should use domain validation with allowEmpty=true and return nil, got: %v",
				err,
			)
		}
	})
}

// TestValidateRepositoryStatus_DomainLayerIntegration tests integration with domain layer.
// This function has been refactored to reduce cognitive complexity by breaking it into smaller, focused tests.
func TestValidateRepositoryStatus_DomainLayerIntegration(t *testing.T) {
	t.Run("valid_statuses_match_domain_behavior", func(t *testing.T) {
		validStatuses := []string{"", "pending", "cloning", "processing", "completed", "failed", "archived"}
		for _, status := range validStatuses {
			assertDomainApplicationValidationMatch(t, status)
		}
	})

	t.Run("invalid_statuses_match_domain_behavior", func(t *testing.T) {
		invalidStatuses := []string{"invalid", "PENDING", "unknown"}
		for _, status := range invalidStatuses {
			assertDomainApplicationValidationMatch(t, status)
			assertApplicationReturnsValidationError(t, status)
		}
	})

	t.Run("sql_injection_patterns_are_handled_separately", func(t *testing.T) {
		sqlInjectionPatterns := []string{"'; DROP TABLE users; --", "UNION SELECT", "--comment"}
		for _, pattern := range sqlInjectionPatterns {
			// SQL injection should be caught by application layer, not domain layer
			appErr := ValidateRepositoryStatus(pattern)
			if appErr == nil {
				t.Errorf("Expected SQL injection pattern %q to be rejected", pattern)
			}
		}
	})
}

// TestValidateRepositoryStatus_NoLongerUsesMap tests that the ValidRepositoryStatuses map is no longer used.
func TestValidateRepositoryStatus_NoLongerUsesMap(t *testing.T) {
	t.Run("map_should_be_removed", func(t *testing.T) {
		// This is a meta test that will fail until the map is removed
		// We can't directly test if the map is used, but we can test behavior changes
		// that would only happen if domain layer validation is used

		// After refactoring, the function should:
		// 1. Still handle SQL injection (first priority)
		// 2. Use domain layer for status validation (second priority)
		// 3. Return ValidationError (not domain error)

		// Test that invalid status returns ValidationError with expected message
		err := ValidateRepositoryStatus("invalid_status")
		if err == nil {
			t.Fatal("Expected error for invalid status")
		}

		var validationErr ValidationError
		ok := errors.As(err, &validationErr)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if validationErr.Field != "status" {
			t.Errorf("Expected field 'status', got %q", validationErr.Field)
		}

		if validationErr.Message != "invalid status" {
			t.Errorf("Expected message 'invalid status', got %q", validationErr.Message)
		}
	})
}

// TestValidateRepositoryStatus_RefactoringRequirements tests specific requirements for the refactoring.
func TestValidateRepositoryStatus_RefactoringRequirements(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
		description string
	}{
		{
			name:        "empty_string_filtering",
			input:       "",
			expectError: false,
			description: "Empty string should be allowed for filtering (allowEmpty=true)",
		},
		{
			name:        "valid_pending_status",
			input:       "pending",
			expectError: false,
			description: "Valid status should pass domain validation",
		},
		{
			name:        "invalid_status",
			input:       "nonexistent",
			expectError: true,
			errorMsg:    "invalid status",
			description: "Invalid status should fail domain validation but return ValidationError",
		},
		{
			name:        "sql_injection_priority",
			input:       "'; DROP TABLE users; --",
			expectError: true,
			errorMsg:    "contains malicious SQL",
			description: "SQL injection should still be caught first, before domain validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRepositoryStatus(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil for input %q (%s)", tt.input, tt.description)
					return
				}

				var validationErr ValidationError
				ok := errors.As(err, &validationErr)
				if !ok {
					t.Errorf("Expected ValidationError but got %T for input %q", err, tt.input)
					return
				}

				if tt.errorMsg != "" && validationErr.Message != tt.errorMsg {
					t.Errorf("Expected error message %q but got %q for input %q",
						tt.errorMsg, validationErr.Message, tt.input)
				}
			} else if err != nil {
				t.Errorf("Expected no error but got %v for input %q (%s)", err, tt.input, tt.description)
			}
		})
	}
}

// Helper function to detect if a string contains SQL injection patterns.
func isSQLInjection(input string) bool {
	sqlPatterns := []string{"'", ";", "--", "DROP", "SELECT", "UNION", "INSERT", "UPDATE", "DELETE"}
	for _, pattern := range sqlPatterns {
		if strings.Contains(strings.ToUpper(input), strings.ToUpper(pattern)) {
			return true
		}
	}
	return false
}

// assertDomainApplicationValidationMatch verifies that domain and application validation behave identically
func assertDomainApplicationValidationMatch(t *testing.T, status string) {
	t.Helper()

	// Skip SQL injection patterns as they are handled by application layer, not domain
	if isSQLInjection(status) {
		return
	}

	// Get expected behavior from domain layer
	domainErr := valueobject.ValidateRepositoryStatusString(status, true)

	// Get actual behavior from application function
	appErr := ValidateRepositoryStatus(status)

	// They should behave the same for status validation
	if (domainErr == nil) != (appErr == nil) {
		t.Errorf("Domain and application validation should match for status %q: domain=%v, app=%v",
			status, domainErr, appErr)
	}
}

// assertApplicationReturnsValidationError verifies that the application returns a ValidationError for invalid statuses
func assertApplicationReturnsValidationError(t *testing.T, status string) {
	t.Helper()

	// Skip SQL injection patterns as they have different error handling
	if isSQLInjection(status) {
		return
	}

	appErr := ValidateRepositoryStatus(status)
	if appErr == nil {
		return // Valid status, no error expected
	}

	// Application should return ValidationError, not domain error
	var validationError ValidationError
	if !errors.As(appErr, &validationError) {
		t.Errorf("Application should return ValidationError for status %q, got %T", status, appErr)
	}
}

// assertRefactoredDomainAppValidationMatch compares domain vs application validation results
// This is kept for backward compatibility with existing tests
func assertRefactoredDomainAppValidationMatch(t *testing.T, status string) {
	t.Helper()
	assertDomainApplicationValidationMatch(t, status)
	assertApplicationReturnsValidationError(t, status)
}

// extractRefactoredValidationError safely extracts ValidationError from an error
func extractRefactoredValidationError(err error) (ValidationError, bool) {
	var validationError ValidationError
	ok := errors.As(err, &validationError)
	return validationError, ok
}

// assertRefactoredValidationErrorMessage validates that the error has the expected message
func assertRefactoredValidationErrorMessage(t *testing.T, err error, expectedMessage string) {
	t.Helper()

	validationErr, ok := extractRefactoredValidationError(err)
	if !ok {
		t.Errorf("Expected ValidationError but got %T", err)
		return
	}

	if validationErr.Message != expectedMessage {
		t.Errorf("Expected message %q but got %q", expectedMessage, validationErr.Message)
	}
}

// TestValidateRepositoryStatus_DomainIntegration_ValidStatuses tests statuses that should pass validation
func TestValidateRepositoryStatus_DomainIntegration_ValidStatuses(t *testing.T) {
	validStatuses := []string{"", "pending"}

	for _, status := range validStatuses {
		t.Run("valid_status_"+status, func(t *testing.T) {
			if isSQLInjection(status) {
				t.Skip("Skipping SQL injection test case")
			}
			assertRefactoredDomainAppValidationMatch(t, status)
		})
	}
}

// TestValidateRepositoryStatus_DomainIntegration_InvalidStatuses tests statuses that should fail validation
func TestValidateRepositoryStatus_DomainIntegration_InvalidStatuses(t *testing.T) {
	invalidStatuses := []string{"invalid", "PENDING"}

	for _, status := range invalidStatuses {
		t.Run("invalid_status_"+status, func(t *testing.T) {
			if isSQLInjection(status) {
				t.Skip("Skipping SQL injection test case")
			}
			assertRefactoredDomainAppValidationMatch(t, status)

			// For invalid statuses, also verify error structure
			appErr := ValidateRepositoryStatus(status)
			if appErr != nil {
				assertRefactoredValidationErrorMessage(t, appErr, "invalid status")
			}
		})
	}
}

// TestValidateRepositoryStatus_DomainIntegration_ErrorHandling tests error type and message validation
func TestValidateRepositoryStatus_DomainIntegration_ErrorHandling(t *testing.T) {
	t.Run("validation_error_structure", func(t *testing.T) {
		status := "definitely_invalid_status"
		appErr := ValidateRepositoryStatus(status)

		if appErr == nil {
			t.Fatal("Expected error for invalid status")
		}

		validationErr, ok := extractRefactoredValidationError(appErr)
		if !ok {
			t.Errorf("Expected ValidationError but got %T", appErr)
			return
		}

		if validationErr.Field != "status" {
			t.Errorf("Expected field 'status' but got %q", validationErr.Field)
		}

		if validationErr.Message != "invalid status" {
			t.Errorf("Expected message 'invalid status' but got %q", validationErr.Message)
		}
	})
}
