package common

import (
	"strings"
	"testing"

	"codechunking/internal/domain/valueobject"
)

// TestValidateRepositoryStatus_RefactoredBehavior tests that ValidateRepositoryStatus
// has been refactored to use domain layer validation instead of ValidRepositoryStatuses map
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
			t.Errorf("ValidateRepositoryStatus('') should use domain validation with allowEmpty=true and return nil, got: %v", err)
		}
	})
}

// TestValidateRepositoryStatus_DomainLayerIntegration tests integration with domain layer
func TestValidateRepositoryStatus_DomainLayerIntegration(t *testing.T) {
	t.Run("should_behave_identically_to_domain_with_allowEmpty_true", func(t *testing.T) {
		testCases := []string{
			"",           // empty string
			"pending",    // valid status
			"cloning",    // valid status
			"processing", // valid status
			"completed",  // valid status
			"failed",     // valid status
			"archived",   // valid status
			"invalid",    // invalid status
			"PENDING",    // case sensitive - should be invalid
			"unknown",    // invalid status
		}

		for _, status := range testCases {
			t.Run("status_"+status, func(t *testing.T) {
				// Get expected behavior from domain layer
				domainErr := valueobject.ValidateRepositoryStatusString(status, true)

				// Get actual behavior from application function
				appErr := ValidateRepositoryStatus(status)

				// They should behave the same for status validation
				// (ignoring SQL injection validation which is application-layer concern)
				if !isSQLInjection(status) {
					if (domainErr == nil) != (appErr == nil) {
						t.Errorf("Domain and application validation should match for status %q: domain=%v, app=%v",
							status, domainErr, appErr)
					}

					// If both return errors, application should return ValidationError
					if domainErr != nil && appErr != nil {
						if _, ok := appErr.(ValidationError); !ok {
							t.Errorf("Application should return ValidationError, got %T", appErr)
						}
					}
				}
			})
		}
	})
}

// TestValidateRepositoryStatus_NoLongerUsesMap tests that the ValidRepositoryStatuses map is no longer used
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

		validationErr, ok := err.(ValidationError)
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

// TestValidateRepositoryStatus_RefactoringRequirements tests specific requirements for the refactoring
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

				validationErr, ok := err.(ValidationError)
				if !ok {
					t.Errorf("Expected ValidationError but got %T for input %q", err, tt.input)
					return
				}

				if tt.errorMsg != "" && validationErr.Message != tt.errorMsg {
					t.Errorf("Expected error message %q but got %q for input %q",
						tt.errorMsg, validationErr.Message, tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got %v for input %q (%s)", err, tt.input, tt.description)
				}
			}
		})
	}
}

// Helper function to detect if a string contains SQL injection patterns
func isSQLInjection(input string) bool {
	sqlPatterns := []string{"'", ";", "--", "DROP", "SELECT", "UNION", "INSERT", "UPDATE", "DELETE"}
	for _, pattern := range sqlPatterns {
		if strings.Contains(strings.ToUpper(input), strings.ToUpper(pattern)) {
			return true
		}
	}
	return false
}

// TestValidateRepositoryStatus_FailingTestsForRefactoring contains tests that SHOULD FAIL initially
// These tests expect the refactored behavior and will guide the implementation
func TestValidateRepositoryStatus_FailingTestsForRefactoring(t *testing.T) {
	t.Run("domain_integration_test", func(t *testing.T) {
		// This test expects that the function integrates with domain layer
		// It will FAIL until the refactoring is complete

		// Test that function behavior matches domain layer behavior for status validation
		statuses := []string{"", "pending", "invalid", "PENDING"}

		for _, status := range statuses {
			if !isSQLInjection(status) {
				domainErr := valueobject.ValidateRepositoryStatusString(status, true)
				appErr := ValidateRepositoryStatus(status)

				// Convert domain error expectations to application ValidationError expectations
				if domainErr == nil {
					if appErr != nil {
						t.Errorf("Status %q: domain allows but application rejects with: %v", status, appErr)
					}
				} else {
					if appErr == nil {
						t.Errorf("Status %q: domain rejects but application allows", status)
					} else {
						// Application should return ValidationError, not domain error
						if _, ok := appErr.(ValidationError); !ok {
							t.Errorf("Status %q: expected ValidationError but got %T", status, appErr)
						}
						// Application should convert domain error message to "invalid status"
						validationErr := appErr.(ValidationError)
						if validationErr.Message != "invalid status" {
							t.Errorf("Status %q: expected 'invalid status' message but got %q",
								status, validationErr.Message)
						}
					}
				}
			}
		}
	})
}
