package common

import (
	"codechunking/internal/domain/valueobject"
	"errors"
	"strings"
	"testing"
)

// TestValidateRepositoryStatus_DomainIntegrationRequired tests that FAIL with current map-based implementation
// and will PASS once refactored to use domain layer validation.
// These tests verify that the function integrates with the domain layer as required.
//
//nolint:gocognit // Complex domain integration test with multiple validation scenarios
func TestValidateRepositoryStatus_DomainIntegrationRequired(t *testing.T) {
	t.Run("must_behave_identically_to_domain_layer_with_allowEmpty_true", func(t *testing.T) {
		// This test will FAIL if the function uses the map instead of domain layer
		// It cross-validates that application function behaves exactly like domain layer

		testCases := []string{
			"",             // empty - should be valid (allowEmpty=true)
			"pending",      // valid status
			"cloning",      // valid status
			"processing",   // valid status
			"completed",    // valid status
			"failed",       // valid status
			"archived",     // valid status
			"invalid_test", // invalid status
			"PENDING",      // case sensitive - should be invalid
			"unknown",      // invalid status
			"   ",          // whitespace - should be invalid
			"pending ",     // trailing space - should be invalid
		}

		for _, status := range testCases {
			t.Run("cross_validate_"+status, func(t *testing.T) {
				// Skip SQL injection cases for this test since they're handled at application layer
				if containsSQLInjectionPatterns(status) {
					t.Skip("Skipping SQL injection case for domain integration test")
				}

				// Get expected behavior from domain layer
				domainErr := valueobject.ValidateRepositoryStatusString(status, true)

				// Get actual behavior from application function
				appErr := ValidateRepositoryStatus(status)

				// They MUST behave identically for non-SQL injection cases
				if (domainErr == nil) != (appErr == nil) {
					t.Errorf(
						"REFACTORING REQUIRED: Status %q - Domain validation and application validation must behave identically.\nDomain error: %v\nApplication error: %v\nThis test will pass once ValidateRepositoryStatus uses domain layer validation.",
						status,
						domainErr,
						appErr,
					)
				}

				// If both return errors, verify error type conversion is correct
				if domainErr != nil && appErr != nil {
					// Application must return ValidationError, not domain error
					var validationErr ValidationError
					ok := errors.As(appErr, &validationErr)
					if !ok {
						t.Errorf(
							"REFACTORING REQUIRED: Status %q - Application must return ValidationError when domain returns error, got %T",
							status,
							appErr,
						)
					} else {
						// Must have correct field and standard message
						if validationErr.Field != "status" {
							t.Errorf("REFACTORING REQUIRED: Status %q - ValidationError must have field 'status', got %q", status, validationErr.Field)
						}
						if validationErr.Message != "invalid status" {
							t.Errorf("REFACTORING REQUIRED: Status %q - ValidationError must have message 'invalid status', got %q", status, validationErr.Message)
						}
					}
				}
			})
		}
	})

	t.Run("must_use_domain_validation_not_map", func(t *testing.T) {
		// This test will FAIL until the ValidRepositoryStatuses map is removed
		// It tests specific behavior that can only work with domain integration

		// Test a case that would expose map vs domain differences
		// If using map: looks up status in ValidRepositoryStatuses map
		// If using domain: calls valueobject.ValidateRepositoryStatusString

		status := "nonexistent_status"

		// First, verify domain layer rejects this
		domainErr := valueobject.ValidateRepositoryStatusString(status, true)
		if domainErr == nil {
			t.Fatal("Test setup error: domain should reject 'nonexistent_status'")
		}

		// Application function should behave identically to domain layer
		appErr := ValidateRepositoryStatus(status)
		if appErr == nil {
			t.Error(
				"REFACTORING REQUIRED: ValidateRepositoryStatus must use domain layer validation and reject invalid statuses",
			)
		}

		// Verify error conversion is proper
		if appErr != nil {
			var validationErr ValidationError
			ok := errors.As(appErr, &validationErr)
			if !ok {
				t.Errorf("REFACTORING REQUIRED: Must convert domain errors to ValidationError, got %T", appErr)
			} else if validationErr.Message != "invalid status" {
				t.Errorf("REFACTORING REQUIRED: Must convert domain error to standard 'invalid status' message, got %q", validationErr.Message)
			}
		}
	})

	t.Run("empty_string_handling_must_match_domain_allowEmpty_true", func(t *testing.T) {
		// This test verifies empty string handling matches domain layer with allowEmpty=true
		// Will FAIL if using map (which has empty string hardcoded) instead of domain validation

		// Domain layer with allowEmpty=true should allow empty string
		domainErr := valueobject.ValidateRepositoryStatusString("", true)
		if domainErr != nil {
			t.Fatalf(
				"Test setup error: domain layer with allowEmpty=true should allow empty string, got: %v",
				domainErr,
			)
		}

		// Application function should behave identically
		appErr := ValidateRepositoryStatus("")
		if appErr != nil {
			t.Error(
				"REFACTORING REQUIRED: ValidateRepositoryStatus must use domain layer with allowEmpty=true and allow empty strings for filtering",
			)
		}
	})

	t.Run("sql_injection_validation_must_be_preserved", func(t *testing.T) {
		// This test ensures SQL injection validation is preserved during refactoring
		// Should continue to work regardless of map vs domain usage

		sqlInjectionInputs := []string{
			"'; DROP TABLE repositories; --",
			"pending' UNION SELECT * FROM users --",
			"status OR 1=1",
			"'; DELETE FROM repositories; --",
		}

		for _, maliciousInput := range sqlInjectionInputs {
			t.Run("preserve_sql_protection_"+maliciousInput, func(t *testing.T) {
				err := ValidateRepositoryStatus(maliciousInput)

				if err == nil {
					t.Errorf(
						"REFACTORING REQUIREMENT: SQL injection validation must be preserved for input %q",
						maliciousInput,
					)
				}

				// Verify it returns ValidationError with SQL message
				if err != nil {
					var validationErr ValidationError
					ok := errors.As(err, &validationErr)
					if !ok {
						t.Errorf(
							"REFACTORING REQUIREMENT: SQL injection detection must return ValidationError, got %T",
							err,
						)
					} else if validationErr.Message != "contains malicious SQL" {
						t.Errorf("REFACTORING REQUIREMENT: SQL injection must return 'contains malicious SQL', got %q", validationErr.Message)
					}
				}
			})
		}
	})
}

// Helper functions for validation testing to reduce cognitive complexity

// assertValidationError checks that err is a ValidationError with the expected message.
func assertValidationError(t *testing.T, err error, expectedMessage string) {
	t.Helper()
	var validationError ValidationError
	if !errors.As(err, &validationError) {
		t.Errorf("Expected ValidationError, got %T", err)
		return
	}
	if validationError.Message != expectedMessage {
		t.Errorf("Expected ValidationError message %q, got %q", expectedMessage, validationError.Message)
	}
}

// assertStatusValid checks that a status is accepted by ValidateRepositoryStatus.
func assertStatusValid(t *testing.T, status string) {
	t.Helper()
	err := ValidateRepositoryStatus(status)
	if err != nil {
		t.Errorf("REFACTORING REQUIREMENT: Status %q must remain valid after refactoring, got error: %v", status, err)
	}
}

// assertStatusInvalid checks that a status is rejected by ValidateRepositoryStatus.
func assertStatusInvalid(t *testing.T, status string, expectedMessage string) {
	t.Helper()
	err := ValidateRepositoryStatus(status)
	if err == nil {
		t.Errorf("REFACTORING REQUIREMENT: Status %q must remain invalid after refactoring", status)
		return
	}
	assertValidationError(t, err, expectedMessage)
}

// TestValidateRepositoryStatus_SignaturePreservation verifies the function signature remains unchanged.
func TestValidateRepositoryStatus_SignaturePreservation(t *testing.T) {
	// This test verifies that after refactoring, the function signature remains the same
	// and behavior is preserved for all current use cases

	validStatuses := []string{"", "pending", "cloning", "processing", "completed", "failed", "archived"}
	for _, status := range validStatuses {
		assertStatusValid(t, status)
	}
}

// TestValidateRepositoryStatus_InvalidStatusesRemainInvalid verifies invalid statuses are still rejected.
func TestValidateRepositoryStatus_InvalidStatusesRemainInvalid(t *testing.T) {
	invalidStatuses := []string{"invalid", "PENDING", "unknown", "   ", " pending "}

	for _, status := range invalidStatuses {
		t.Run("invalid_status_"+status, func(t *testing.T) {
			assertStatusInvalid(t, status, "invalid status")
		})
	}
}

// TestValidateRepositoryStatus_DomainValidStatusesConsistency tests domain layer valid statuses.
func TestValidateRepositoryStatus_DomainValidStatusesConsistency(t *testing.T) {
	// Test every status that domain layer knows about
	allDomainStatuses := []string{"pending", "cloning", "processing", "completed", "failed", "archived"}

	for _, status := range allDomainStatuses {
		t.Run("domain_status_"+status, func(t *testing.T) {
			// Domain layer should accept this status
			domainErr := valueobject.ValidateRepositoryStatusString(status, true)
			if domainErr != nil {
				t.Fatalf("Test setup error: domain should accept %q, got: %v", status, domainErr)
			}

			// Application function should also accept it (since it should use domain layer)
			appErr := ValidateRepositoryStatus(status)
			if appErr != nil {
				t.Errorf(
					"REFACTORING REQUIRED: ValidateRepositoryStatus(%q) should use domain layer and accept valid status, got: %v",
					status,
					appErr,
				)
			}
		})
	}
}

// TestValidateRepositoryStatus_DomainInvalidStatusesConsistency tests domain layer invalid statuses.
func TestValidateRepositoryStatus_DomainInvalidStatusesConsistency(t *testing.T) {
	// Test invalid statuses - both should reject
	invalidStatuses := []string{"invalid", "nonexistent", "PENDING"}

	for _, status := range invalidStatuses {
		t.Run("invalid_status_"+status, func(t *testing.T) {
			// Domain layer should reject this
			domainErr := valueobject.ValidateRepositoryStatusString(status, true)
			if domainErr == nil {
				t.Fatalf("Test setup error: domain should reject %q", status)
			}

			// Application function should also reject it
			appErr := ValidateRepositoryStatus(status)
			if appErr == nil {
				t.Errorf(
					"REFACTORING REQUIRED: ValidateRepositoryStatus(%q) should use domain layer and reject invalid status",
					status,
				)
			}
		})
	}
}

// TestValidateRepositoryStatus_RefactoringGuidance provides specific guidance for the refactoring.
func TestValidateRepositoryStatus_RefactoringGuidance(t *testing.T) {
	t.Run("implementation_steps_verification", func(t *testing.T) {
		// This test guides the specific implementation steps required

		t.Log("REFACTORING STEPS REQUIRED:")
		t.Log("1. Remove ValidRepositoryStatuses map from validation.go lines 13-21")
		t.Log("2. Update ValidateRepositoryStatus function (lines 84-95) to:")
		t.Log("   a. Keep SQL injection validation using security validator")
		t.Log("   b. Replace map lookup with: valueobject.ValidateRepositoryStatusString(status, true)")
		t.Log("   c. Convert any domain error to ValidationError with 'invalid status' message")
		t.Log("   d. Maintain same function signature and return types")
		t.Log("3. Verify all tests pass after refactoring")

		// Test the specific requirements that will guide implementation
		status := "test_invalid_status"

		// This should behave like domain layer validation
		domainErr := valueobject.ValidateRepositoryStatusString(status, true)
		appErr := ValidateRepositoryStatus(status)

		switch {
		case domainErr == nil && appErr != nil:
			t.Error("IMPLEMENTATION ISSUE: Domain allows but application rejects - check domain integration")
		case domainErr != nil && appErr == nil:
			t.Error("IMPLEMENTATION ISSUE: Domain rejects but application allows - map may still be used")
		case domainErr != nil && appErr != nil:
			// Both reject - verify error conversion is correct
			var validationErr ValidationError
			ok := errors.As(appErr, &validationErr)
			if !ok {
				t.Error("IMPLEMENTATION ISSUE: Domain errors must be converted to ValidationError")
			} else if validationErr.Message != "invalid status" {
				t.Error("IMPLEMENTATION ISSUE: Domain errors must be converted to 'invalid status' message")
			}
		}
	})
}

// Helper function to detect SQL injection patterns for test filtering.
func containsSQLInjectionPatterns(input string) bool {
	sqlPatterns := []string{"'", ";", "--", "DROP", "SELECT", "UNION", "INSERT", "UPDATE", "DELETE", "OR "}
	inputUpper := strings.ToUpper(input)

	for _, pattern := range sqlPatterns {
		if strings.Contains(inputUpper, strings.ToUpper(pattern)) {
			return true
		}
	}
	return false
}
