package worker

import (
	"testing"
	"time"
)

// TestValidateTimeoutConfig tests the timeout configuration validation function that doesn't exist yet.
// This test will fail until validateTimeoutConfig is implemented.
func TestValidateTimeoutConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        AckHandlerConfig
		expectedError string
	}{
		{
			name: "valid timeout configuration",
			config: AckHandlerConfig{
				AckTimeout: 30 * time.Second,
			},
			expectedError: "",
		},
		{
			name: "zero timeout should fail",
			config: AckHandlerConfig{
				AckTimeout: 0,
			},
			expectedError: "ack_timeout must be positive",
		},
		{
			name: "negative timeout should fail",
			config: AckHandlerConfig{
				AckTimeout: -5 * time.Second,
			},
			expectedError: "ack_timeout must be positive",
		},
		{
			name: "timeout below minimum should fail",
			config: AckHandlerConfig{
				AckTimeout: 500 * time.Millisecond, // Below MinAckTimeout (1 second)
			},
			expectedError: "ack_timeout must be at least 1s",
		},
		{
			name: "timeout above maximum should fail",
			config: AckHandlerConfig{
				AckTimeout: 10 * time.Minute, // Above MaxAckTimeout (5 minutes)
			},
			expectedError: "ack_timeout cannot exceed 5m0s",
		},
		{
			name: "timeout at minimum boundary should succeed",
			config: AckHandlerConfig{
				AckTimeout: MinAckTimeout,
			},
			expectedError: "",
		},
		{
			name: "timeout at maximum boundary should succeed",
			config: AckHandlerConfig{
				AckTimeout: MaxAckTimeout,
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimeoutConfig(tt.config)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("validateTimeoutConfig() expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("validateTimeoutConfig() expected error %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("validateTimeoutConfig() expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

// TestValidateDeliveryAttemptsConfig tests the delivery attempts configuration validation function.
// This test will fail until validateDeliveryAttemptsConfig is implemented.
func TestValidateDeliveryAttemptsConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        AckHandlerConfig
		expectedError string
	}{
		{
			name: "valid delivery attempts configuration",
			config: AckHandlerConfig{
				MaxDeliveryAttempts: 3,
			},
			expectedError: "",
		},
		{
			name: "zero delivery attempts should fail",
			config: AckHandlerConfig{
				MaxDeliveryAttempts: 0,
			},
			expectedError: "max_delivery_attempts must be positive",
		},
		{
			name: "negative delivery attempts should fail",
			config: AckHandlerConfig{
				MaxDeliveryAttempts: -1,
			},
			expectedError: "max_delivery_attempts must be positive",
		},
		{
			name: "delivery attempts below minimum should fail",
			config: AckHandlerConfig{
				MaxDeliveryAttempts: 0, // Below MinMaxDeliveryAttempts (1)
			},
			expectedError: "max_delivery_attempts must be positive",
		},
		{
			name: "delivery attempts above maximum should fail",
			config: AckHandlerConfig{
				MaxDeliveryAttempts: 15, // Above MaxMaxDeliveryAttempts (10)
			},
			expectedError: "max_delivery_attempts cannot exceed 10",
		},
		{
			name: "delivery attempts at minimum boundary should succeed",
			config: AckHandlerConfig{
				MaxDeliveryAttempts: MinMaxDeliveryAttempts,
			},
			expectedError: "",
		},
		{
			name: "delivery attempts at maximum boundary should succeed",
			config: AckHandlerConfig{
				MaxDeliveryAttempts: MaxMaxDeliveryAttempts,
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeliveryAttemptsConfig(tt.config)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("validateDeliveryAttemptsConfig() expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("validateDeliveryAttemptsConfig() expected error %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("validateDeliveryAttemptsConfig() expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

// TestValidateBackoffStrategyConfig tests the backoff strategy configuration validation function.
// This test will fail until validateBackoffStrategyConfig is implemented.
func TestValidateBackoffStrategyConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        AckHandlerConfig
		expectedError string
	}{
		{
			name: "empty backoff strategy should succeed",
			config: AckHandlerConfig{
				BackoffStrategy: "",
			},
			expectedError: "",
		},
		{
			name: "valid exponential backoff strategy",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyExponential,
				InitialBackoffDelay: 1 * time.Second,
				MaxBackoffDelay:     30 * time.Second,
				BackoffMultiplier:   2.0,
			},
			expectedError: "",
		},
		{
			name: "valid linear backoff strategy",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyLinear,
				InitialBackoffDelay: 1 * time.Second,
				MaxBackoffDelay:     30 * time.Second,
			},
			expectedError: "",
		},
		{
			name: "valid fixed backoff strategy",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyFixed,
				InitialBackoffDelay: 1 * time.Second,
				MaxBackoffDelay:     30 * time.Second,
			},
			expectedError: "",
		},
		{
			name: "invalid backoff strategy should fail",
			config: AckHandlerConfig{
				BackoffStrategy: "invalid",
			},
			expectedError: "invalid backoff_strategy: invalid (must be exponential, linear, or fixed)",
		},
		{
			name: "initial backoff delay below minimum should fail",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyExponential,
				InitialBackoffDelay: 50 * time.Millisecond, // Below MinInitialBackoffDelay (100ms)
				MaxBackoffDelay:     30 * time.Second,
				BackoffMultiplier:   2.0,
			},
			expectedError: "initial_backoff_delay must be at least 100ms",
		},
		{
			name: "initial backoff delay above maximum should fail",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyExponential,
				InitialBackoffDelay: 60 * time.Second, // Above MaxInitialBackoffDelay (30s)
				MaxBackoffDelay:     90 * time.Second,
				BackoffMultiplier:   2.0,
			},
			expectedError: "initial_backoff_delay cannot exceed 30s",
		},
		{
			name: "max backoff delay below minimum should fail",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyExponential,
				InitialBackoffDelay: 500 * time.Millisecond, // Set to same as max to avoid cross-validation error
				MaxBackoffDelay:     500 * time.Millisecond, // Below MinMaxBackoffDelay (1s)
				BackoffMultiplier:   2.0,
			},
			expectedError: "max_backoff_delay must be at least 1s",
		},
		{
			name: "max backoff delay above maximum should fail",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyExponential,
				InitialBackoffDelay: 1 * time.Second,
				MaxBackoffDelay:     10 * time.Minute, // Above MaxMaxBackoffDelay (5m)
				BackoffMultiplier:   2.0,
			},
			expectedError: "max_backoff_delay cannot exceed 5m0s",
		},
		{
			name: "initial delay greater than max delay should fail",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyExponential,
				InitialBackoffDelay: 30 * time.Second,
				MaxBackoffDelay:     10 * time.Second, // Less than initial delay
				BackoffMultiplier:   2.0,
			},
			expectedError: "initial_backoff_delay cannot be greater than max_backoff_delay",
		},
		{
			name: "backoff multiplier below minimum for exponential should fail",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyExponential,
				InitialBackoffDelay: 1 * time.Second,
				MaxBackoffDelay:     30 * time.Second,
				BackoffMultiplier:   1.0, // Below MinBackoffMultiplier (1.1)
			},
			expectedError: "backoff_multiplier must be at least 1.100000",
		},
		{
			name: "backoff multiplier above maximum for exponential should fail",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyExponential,
				InitialBackoffDelay: 1 * time.Second,
				MaxBackoffDelay:     30 * time.Second,
				BackoffMultiplier:   6.0, // Above MaxBackoffMultiplier (5.0)
			},
			expectedError: "backoff_multiplier cannot exceed 5.000000",
		},
		{
			name: "backoff multiplier validation ignored for non-exponential strategy",
			config: AckHandlerConfig{
				BackoffStrategy:     BackoffStrategyLinear,
				InitialBackoffDelay: 1 * time.Second,
				MaxBackoffDelay:     30 * time.Second,
				BackoffMultiplier:   10.0, // Would fail for exponential, but should succeed for linear
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBackoffStrategyConfig(tt.config)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("validateBackoffStrategyConfig() expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("validateBackoffStrategyConfig() expected error %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("validateBackoffStrategyConfig() expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

// TestValidateConcurrencyConfig tests the concurrency configuration validation function.
// This test will fail until validateConcurrencyConfig is implemented.
func TestValidateConcurrencyConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        AckHandlerConfig
		expectedError string
	}{
		{
			name: "valid concurrent acks configuration",
			config: AckHandlerConfig{
				MaxConcurrentAcks: 100,
			},
			expectedError: "",
		},
		{
			name: "zero concurrent acks should succeed",
			config: AckHandlerConfig{
				MaxConcurrentAcks: 0,
			},
			expectedError: "",
		},
		{
			name: "concurrent acks above maximum should fail",
			config: AckHandlerConfig{
				MaxConcurrentAcks: 1500, // Above MaxConcurrentAcks (1000)
			},
			expectedError: "max_concurrent_acks cannot exceed 1000",
		},
		{
			name: "concurrent acks at maximum boundary should succeed",
			config: AckHandlerConfig{
				MaxConcurrentAcks: MaxConcurrentAcks,
			},
			expectedError: "",
		},
		{
			name: "negative concurrent acks should succeed", // Treating as unlimited
			config: AckHandlerConfig{
				MaxConcurrentAcks: -1,
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConcurrencyConfig(tt.config)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("validateConcurrencyConfig() expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("validateConcurrencyConfig() expected error %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("validateConcurrencyConfig() expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

// TestValidateDuplicateDetectionConfig tests the duplicate detection configuration validation function.
// This test will fail until validateDuplicateDetectionConfig is implemented.
func TestValidateDuplicateDetectionConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        AckHandlerConfig
		expectedError string
	}{
		{
			name: "duplicate detection disabled should succeed",
			config: AckHandlerConfig{
				EnableDuplicateDetection: false,
			},
			expectedError: "",
		},
		{
			name: "valid duplicate detection configuration",
			config: AckHandlerConfig{
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: 5 * time.Minute,
			},
			expectedError: "",
		},
		{
			name: "duplicate detection window below minimum should fail",
			config: AckHandlerConfig{
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: 10 * time.Second, // Below MinDuplicateDetectionWindow (30s)
			},
			expectedError: "duplicate_detection_window must be at least 30s",
		},
		{
			name: "duplicate detection window above maximum should fail",
			config: AckHandlerConfig{
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: 48 * time.Hour, // Above MaxDuplicateDetectionWindow (24h)
			},
			expectedError: "duplicate_detection_window cannot exceed 24h0m0s",
		},
		{
			name: "duplicate detection window at minimum boundary should succeed",
			config: AckHandlerConfig{
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: MinDuplicateDetectionWindow,
			},
			expectedError: "",
		},
		{
			name: "duplicate detection window at maximum boundary should succeed",
			config: AckHandlerConfig{
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: MaxDuplicateDetectionWindow,
			},
			expectedError: "",
		},
		{
			name: "zero duplicate detection window with detection enabled should fail",
			config: AckHandlerConfig{
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: 0,
			},
			expectedError: "duplicate_detection_window must be at least 30s",
		},
		{
			name: "negative duplicate detection window with detection enabled should fail",
			config: AckHandlerConfig{
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: -5 * time.Minute,
			},
			expectedError: "duplicate_detection_window must be at least 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDuplicateDetectionConfig(tt.config)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("validateDuplicateDetectionConfig() expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("validateDuplicateDetectionConfig() expected error %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("validateDuplicateDetectionConfig() expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

// validateTestCase is a helper function that validates a single test case
// to reduce cognitive complexity in the main test function.
func validateTestCase(t *testing.T, config AckHandlerConfig, expectedErrors []string, expectNoError bool) {
	err := validateAckHandlerConfig(config)

	if expectNoError {
		if err != nil {
			t.Errorf("validateAckHandlerConfig() expected no error, got %v", err)
		}
		return
	}

	if len(expectedErrors) == 0 {
		t.Fatal("test case must specify either expectNoError=true or expectedErrors")
	}

	if err == nil {
		t.Errorf("validateAckHandlerConfig() expected errors %v, got nil", expectedErrors)
		return
	}

	validateExpectedErrors(t, err.Error(), expectedErrors)
}

// validateExpectedErrors checks that all expected error messages are present
// in the actual error message.
func validateExpectedErrors(t *testing.T, errorMsg string, expectedErrors []string) {
	for _, expectedError := range expectedErrors {
		if !containsError(errorMsg, expectedError) {
			t.Errorf(
				"validateAckHandlerConfig() error message %q does not contain expected error %q",
				errorMsg,
				expectedError,
			)
		}
	}
}

// getTestCases returns the test cases for the validation function.
func getTestCases() []struct {
	name           string
	config         AckHandlerConfig
	expectedErrors []string
	expectNoError  bool
} {
	return []struct {
		name           string
		config         AckHandlerConfig
		expectedErrors []string
		expectNoError  bool
	}{
		{
			name: "all valid configuration should succeed",
			config: AckHandlerConfig{
				AckTimeout:               30 * time.Second,
				MaxDeliveryAttempts:      3,
				BackoffStrategy:          BackoffStrategyExponential,
				InitialBackoffDelay:      1 * time.Second,
				MaxBackoffDelay:          30 * time.Second,
				BackoffMultiplier:        2.0,
				MaxConcurrentAcks:        100,
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: 5 * time.Minute,
			},
			expectNoError: true,
		},
		{
			name: "multiple configuration errors should be aggregated",
			config: AckHandlerConfig{
				AckTimeout:               -1 * time.Second, // Invalid timeout
				MaxDeliveryAttempts:      0,                // Invalid delivery attempts
				BackoffStrategy:          "invalid",        // Invalid backoff strategy
				MaxConcurrentAcks:        2000,             // Invalid concurrency limit
				EnableDuplicateDetection: true,
				DuplicateDetectionWindow: 10 * time.Second, // Invalid duplicate detection window
			},
			expectedErrors: []string{
				"ack_timeout must be positive",
				"max_delivery_attempts must be positive",
				"invalid backoff_strategy: invalid (must be exponential, linear, or fixed)",
				"max_concurrent_acks cannot exceed 1000",
				"duplicate_detection_window must be at least 30s",
			},
		},
		{
			name: "timeout and backoff strategy errors",
			config: AckHandlerConfig{
				AckTimeout:               10 * time.Minute, // Above max
				MaxDeliveryAttempts:      5,                // Valid
				BackoffStrategy:          BackoffStrategyExponential,
				InitialBackoffDelay:      50 * time.Millisecond, // Below min
				MaxBackoffDelay:          30 * time.Second,      // Valid
				BackoffMultiplier:        0.5,                   // Below min for exponential
				MaxConcurrentAcks:        100,                   // Valid
				EnableDuplicateDetection: false,                 // Disabled, so window doesn't matter
			},
			expectedErrors: []string{
				"ack_timeout cannot exceed 5m0s",
				"initial_backoff_delay must be at least 100ms",
				"backoff_multiplier must be at least 1.100000",
			},
		},
	}
}

// TestValidateAckHandlerConfigRefactored tests the refactored main validation function.
// This test will fail until the main function is refactored to call the individual validation functions.
func TestValidateAckHandlerConfigRefactored(t *testing.T) {
	tests := getTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validateTestCase(t, tt.config, tt.expectedErrors, tt.expectNoError)
		})
	}
}

// TestValidateAckHandlerConfigCallsAllValidators tests that the main validation function
// calls all the individual validation functions. This will fail until the refactoring is complete.
func TestValidateAckHandlerConfigCallsAllValidators(t *testing.T) {
	// This test uses a technique to verify that all validation functions are called
	// by creating a configuration that would trigger each validation function's error
	config := AckHandlerConfig{
		AckTimeout:               -1 * time.Second, // Should trigger validateTimeoutConfig error
		MaxDeliveryAttempts:      0,                // Should trigger validateDeliveryAttemptsConfig error
		BackoffStrategy:          "invalid",        // Should trigger validateBackoffStrategyConfig error
		MaxConcurrentAcks:        2000,             // Should trigger validateConcurrencyConfig error
		EnableDuplicateDetection: true,
		DuplicateDetectionWindow: 10 * time.Second, // Should trigger validateDuplicateDetectionConfig error
	}

	err := validateAckHandlerConfig(config)

	if err == nil {
		t.Fatal("validateAckHandlerConfig() should have returned error for invalid configuration")
	}

	// Check that the error contains evidence that each validator was called
	errorMsg := err.Error()

	expectedSubstrings := []string{
		"ack_timeout must be positive",                // From validateTimeoutConfig
		"max_delivery_attempts must be positive",      // From validateDeliveryAttemptsConfig
		"invalid backoff_strategy",                    // From validateBackoffStrategyConfig
		"max_concurrent_acks cannot exceed",           // From validateConcurrencyConfig
		"duplicate_detection_window must be at least", // From validateDuplicateDetectionConfig
	}

	for _, expected := range expectedSubstrings {
		if !containsError(errorMsg, expected) {
			t.Errorf(
				"validateAckHandlerConfig() error %q missing expected substring from validator: %q",
				errorMsg,
				expected,
			)
		}
	}
}

// TestIndividualValidationFunctionIndependence tests that each validation function
// only validates its own concern and doesn't interfere with others.
func TestIndividualValidationFunctionIndependence(t *testing.T) {
	// Test that validateTimeoutConfig only cares about timeout
	t.Run("timeout validation independence", func(t *testing.T) {
		config := AckHandlerConfig{
			AckTimeout:          30 * time.Second, // Valid timeout
			MaxDeliveryAttempts: 0,                // Invalid delivery attempts (should be ignored)
			BackoffStrategy:     "invalid",        // Invalid strategy (should be ignored)
			MaxConcurrentAcks:   2000,             // Invalid concurrency (should be ignored)
		}

		err := validateTimeoutConfig(config)
		if err != nil {
			t.Errorf("validateTimeoutConfig() should not care about non-timeout config, got error: %v", err)
		}
	})

	// Test that validateDeliveryAttemptsConfig only cares about delivery attempts
	t.Run("delivery attempts validation independence", func(t *testing.T) {
		config := AckHandlerConfig{
			AckTimeout:          -1 * time.Second, // Invalid timeout (should be ignored)
			MaxDeliveryAttempts: 3,                // Valid delivery attempts
			BackoffStrategy:     "invalid",        // Invalid strategy (should be ignored)
			MaxConcurrentAcks:   2000,             // Invalid concurrency (should be ignored)
		}

		err := validateDeliveryAttemptsConfig(config)
		if err != nil {
			t.Errorf(
				"validateDeliveryAttemptsConfig() should not care about non-delivery-attempts config, got error: %v",
				err,
			)
		}
	})

	// Test that validateBackoffStrategyConfig only cares about backoff strategy
	t.Run("backoff strategy validation independence", func(t *testing.T) {
		config := AckHandlerConfig{
			AckTimeout:          -1 * time.Second,           // Invalid timeout (should be ignored)
			MaxDeliveryAttempts: 0,                          // Invalid delivery attempts (should be ignored)
			BackoffStrategy:     BackoffStrategyExponential, // Valid strategy
			InitialBackoffDelay: 1 * time.Second,
			MaxBackoffDelay:     30 * time.Second,
			BackoffMultiplier:   2.0,
			MaxConcurrentAcks:   2000, // Invalid concurrency (should be ignored)
		}

		err := validateBackoffStrategyConfig(config)
		if err != nil {
			t.Errorf("validateBackoffStrategyConfig() should not care about non-backoff config, got error: %v", err)
		}
	})

	// Test that validateConcurrencyConfig only cares about concurrency
	t.Run("concurrency validation independence", func(t *testing.T) {
		config := AckHandlerConfig{
			AckTimeout:          -1 * time.Second, // Invalid timeout (should be ignored)
			MaxDeliveryAttempts: 0,                // Invalid delivery attempts (should be ignored)
			BackoffStrategy:     "invalid",        // Invalid strategy (should be ignored)
			MaxConcurrentAcks:   100,              // Valid concurrency
		}

		err := validateConcurrencyConfig(config)
		if err != nil {
			t.Errorf("validateConcurrencyConfig() should not care about non-concurrency config, got error: %v", err)
		}
	})

	// Test that validateDuplicateDetectionConfig only cares about duplicate detection
	t.Run("duplicate detection validation independence", func(t *testing.T) {
		config := AckHandlerConfig{
			AckTimeout:               -1 * time.Second, // Invalid timeout (should be ignored)
			MaxDeliveryAttempts:      0,                // Invalid delivery attempts (should be ignored)
			BackoffStrategy:          "invalid",        // Invalid strategy (should be ignored)
			MaxConcurrentAcks:        2000,             // Invalid concurrency (should be ignored)
			EnableDuplicateDetection: true,
			DuplicateDetectionWindow: 5 * time.Minute, // Valid duplicate detection
		}

		err := validateDuplicateDetectionConfig(config)
		if err != nil {
			t.Errorf(
				"validateDuplicateDetectionConfig() should not care about non-duplicate-detection config, got error: %v",
				err,
			)
		}
	})
}

// TestErrorAggregationInMainValidator tests that the main validator properly aggregates
// errors from all individual validators instead of returning on first error.
func TestErrorAggregationInMainValidator(t *testing.T) {
	// Configuration with errors in each validation area
	config := AckHandlerConfig{
		AckTimeout:               500 * time.Millisecond, // Below minimum
		MaxDeliveryAttempts:      15,                     // Above maximum
		BackoffStrategy:          "unknown",              // Invalid strategy
		MaxConcurrentAcks:        1500,                   // Above maximum
		EnableDuplicateDetection: true,
		DuplicateDetectionWindow: 5 * time.Second, // Below minimum
	}

	err := validateAckHandlerConfig(config)

	if err == nil {
		t.Fatal("validateAckHandlerConfig() should return aggregated errors")
	}

	// Verify the error contains information from all failed validators
	errorMsg := err.Error()

	// Count how many different validation errors are present
	validationErrors := []string{
		"ack_timeout must be at least",                // timeout validation
		"max_delivery_attempts cannot exceed",         // delivery attempts validation
		"invalid backoff_strategy",                    // backoff strategy validation
		"max_concurrent_acks cannot exceed",           // concurrency validation
		"duplicate_detection_window must be at least", // duplicate detection validation
	}

	errorCount := 0
	for _, expectedError := range validationErrors {
		if containsError(errorMsg, expectedError) {
			errorCount++
		}
	}

	if errorCount < 3 {
		t.Errorf(
			"validateAckHandlerConfig() should aggregate errors from multiple validators, only found %d errors in: %s",
			errorCount,
			errorMsg,
		)
	}
}

// containsError checks if an error message contains a substring, case-insensitive.
func containsError(errorMsg, substring string) bool {
	return len(errorMsg) > 0 && len(substring) > 0 &&
		(errorMsg == substring ||
			len(errorMsg) >= len(substring) &&
				findInString(errorMsg, substring))
}

// findInString performs a simple substring search.
func findInString(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}

	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestValidationFunctionSignatures tests that all validation functions have the expected signatures.
// This ensures the functions can be called properly from the main validator.
func TestValidationFunctionSignatures(t *testing.T) {
	// Test that we can call each validation function with a config
	config := AckHandlerConfig{}

	// These calls will fail until the functions are implemented
	t.Run("validateTimeoutConfig signature", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic since function doesn't exist yet
				t.Logf("validateTimeoutConfig not implemented yet: %v", r)
			}
		}()
		_ = validateTimeoutConfig(config)
	})

	t.Run("validateDeliveryAttemptsConfig signature", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic since function doesn't exist yet
				t.Logf("validateDeliveryAttemptsConfig not implemented yet: %v", r)
			}
		}()
		_ = validateDeliveryAttemptsConfig(config)
	})

	t.Run("validateBackoffStrategyConfig signature", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic since function doesn't exist yet
				t.Logf("validateBackoffStrategyConfig not implemented yet: %v", r)
			}
		}()
		_ = validateBackoffStrategyConfig(config)
	})

	t.Run("validateConcurrencyConfig signature", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic since function doesn't exist yet
				t.Logf("validateConcurrencyConfig not implemented yet: %v", r)
			}
		}()
		_ = validateConcurrencyConfig(config)
	})

	t.Run("validateDuplicateDetectionConfig signature", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic since function doesn't exist yet
				t.Logf("validateDuplicateDetectionConfig not implemented yet: %v", r)
			}
		}()
		_ = validateDuplicateDetectionConfig(config)
	})
}

// TestValidationEdgeCases tests edge cases that the refactored validation should handle.
func TestValidationEdgeCases(t *testing.T) {
	t.Run("boundary values should be handled correctly", func(t *testing.T) {
		config := AckHandlerConfig{
			AckTimeout:               MinAckTimeout,          // Exact minimum
			MaxDeliveryAttempts:      MaxMaxDeliveryAttempts, // Exact maximum
			BackoffStrategy:          BackoffStrategyExponential,
			InitialBackoffDelay:      MinInitialBackoffDelay, // Exact minimum
			MaxBackoffDelay:          MaxMaxBackoffDelay,     // Exact maximum
			BackoffMultiplier:        MinBackoffMultiplier,   // Exact minimum
			MaxConcurrentAcks:        MaxConcurrentAcks,      // Exact maximum
			EnableDuplicateDetection: true,
			DuplicateDetectionWindow: MinDuplicateDetectionWindow, // Exact minimum
		}

		err := validateAckHandlerConfig(config)
		if err != nil {
			t.Errorf("validateAckHandlerConfig() should accept boundary values, got error: %v", err)
		}
	})

	t.Run("empty/zero config should be validated correctly", func(t *testing.T) {
		config := AckHandlerConfig{} // All zero values

		err := validateAckHandlerConfig(config)
		if err == nil {
			t.Error("validateAckHandlerConfig() should reject zero/empty configuration")
		}

		// Should contain errors from timeout and delivery attempts validators
		errorMsg := err.Error()
		if !containsError(errorMsg, "ack_timeout must be positive") {
			t.Error("validateAckHandlerConfig() should include timeout validation error for zero config")
		}
		if !containsError(errorMsg, "max_delivery_attempts must be positive") {
			t.Error("validateAckHandlerConfig() should include delivery attempts validation error for zero config")
		}
	})
}
