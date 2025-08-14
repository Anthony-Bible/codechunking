package entity

import (
	"testing"
)

func TestNewDomainError(t *testing.T) {
	testCases := []struct {
		name     string
		message  string
		code     string
		expected *DomainError
	}{
		{
			name:     "Normal error creation",
			message:  "Something went wrong",
			code:     "SOMETHING_WRONG",
			expected: &DomainError{message: "Something went wrong", code: "SOMETHING_WRONG"},
		},
		{
			name:     "Empty message",
			message:  "",
			code:     "EMPTY_MESSAGE",
			expected: &DomainError{message: "", code: "EMPTY_MESSAGE"},
		},
		{
			name:     "Empty code",
			message:  "Message without code",
			code:     "",
			expected: &DomainError{message: "Message without code", code: ""},
		},
		{
			name:     "Both empty",
			message:  "",
			code:     "",
			expected: &DomainError{message: "", code: ""},
		},
		{
			name:     "Special characters in message",
			message:  "Error with special chars: !@#$%^&*()",
			code:     "SPECIAL_CHARS",
			expected: &DomainError{message: "Error with special chars: !@#$%^&*()", code: "SPECIAL_CHARS"},
		},
		{
			name:     "Unicode characters",
			message:  "Unicode error: 文字化け",
			code:     "UNICODE_ERROR",
			expected: &DomainError{message: "Unicode error: 文字化け", code: "UNICODE_ERROR"},
		},
		{
			name:    "Very long message",
			message: "This is a very long error message that contains a lot of text to test how the domain error handles lengthy messages without any issues",
			code:    "LONG_MESSAGE",
			expected: &DomainError{
				message: "This is a very long error message that contains a lot of text to test how the domain error handles lengthy messages without any issues",
				code:    "LONG_MESSAGE",
			},
		},
		{
			name:     "Newlines in message",
			message:  "Multi-line\nerror\nmessage",
			code:     "MULTILINE",
			expected: &DomainError{message: "Multi-line\nerror\nmessage", code: "MULTILINE"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewDomainError(tc.message, tc.code)

			if err == nil {
				t.Fatal("Expected non-nil error")
			}

			if err.message != tc.expected.message {
				t.Errorf("Expected message '%s', got '%s'", tc.expected.message, err.message)
			}

			if err.code != tc.expected.code {
				t.Errorf("Expected code '%s', got '%s'", tc.expected.code, err.code)
			}
		})
	}
}

func TestDomainError_Error(t *testing.T) {
	testCases := []struct {
		name            string
		message         string
		code            string
		expectedMessage string
	}{
		{
			name:            "Normal error message",
			message:         "Something went wrong",
			code:            "SOMETHING_WRONG",
			expectedMessage: "Something went wrong",
		},
		{
			name:            "Empty message returns empty string",
			message:         "",
			code:            "EMPTY_MESSAGE",
			expectedMessage: "",
		},
		{
			name:            "Message with whitespace",
			message:         "  Error with spaces  ",
			code:            "WHITESPACE",
			expectedMessage: "  Error with spaces  ",
		},
		{
			name:            "Message identical to code",
			message:         "DUPLICATE",
			code:            "DUPLICATE",
			expectedMessage: "DUPLICATE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewDomainError(tc.message, tc.code)

			errorMessage := err.Error()
			if errorMessage != tc.expectedMessage {
				t.Errorf("Expected Error() to return '%s', got '%s'", tc.expectedMessage, errorMessage)
			}
		})
	}
}

func TestDomainError_Code(t *testing.T) {
	testCases := []struct {
		name         string
		message      string
		code         string
		expectedCode string
	}{
		{
			name:         "Normal error code",
			message:      "Something went wrong",
			code:         "SOMETHING_WRONG",
			expectedCode: "SOMETHING_WRONG",
		},
		{
			name:         "Empty code returns empty string",
			message:      "Error message",
			code:         "",
			expectedCode: "",
		},
		{
			name:         "Code with special characters",
			message:      "Error",
			code:         "ERROR_WITH_UNDERSCORES",
			expectedCode: "ERROR_WITH_UNDERSCORES",
		},
		{
			name:         "Numeric code",
			message:      "Numeric error",
			code:         "123456",
			expectedCode: "123456",
		},
		{
			name:         "Mixed case code",
			message:      "Mixed case error",
			code:         "MixedCaseCode",
			expectedCode: "MixedCaseCode",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewDomainError(tc.message, tc.code)

			code := err.Code()
			if code != tc.expectedCode {
				t.Errorf("Expected Code() to return '%s', got '%s'", tc.expectedCode, code)
			}
		})
	}
}

func TestDomainError_Message(t *testing.T) {
	testCases := []struct {
		name            string
		message         string
		code            string
		expectedMessage string
	}{
		{
			name:            "Normal message",
			message:         "Something went wrong",
			code:            "SOMETHING_WRONG",
			expectedMessage: "Something went wrong",
		},
		{
			name:            "Empty message returns empty string",
			message:         "",
			code:            "EMPTY_MESSAGE",
			expectedMessage: "",
		},
		{
			name:            "Message with formatting",
			message:         "Error: %s occurred at %d",
			code:            "FORMAT_ERROR",
			expectedMessage: "Error: %s occurred at %d",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewDomainError(tc.message, tc.code)

			message := err.Message()
			if message != tc.expectedMessage {
				t.Errorf("Expected Message() to return '%s', got '%s'", tc.expectedMessage, message)
			}
		})
	}
}

func TestDomainError_ImplementsErrorInterface(t *testing.T) {
	err := NewDomainError("Test error", "TEST_ERROR")

	// Test that DomainError implements the error interface
	var _ error = err

	// Test that it can be used as an error
	errorMessage := err.Error()
	if errorMessage != "Test error" {
		t.Errorf("Expected error message 'Test error', got '%s'", errorMessage)
	}
}

func TestDomainError_Consistency(t *testing.T) {
	// Test that Error() and Message() return the same value
	testCases := []struct {
		name    string
		message string
		code    string
	}{
		{"Normal case", "Test message", "TEST_CODE"},
		{"Empty message", "", "EMPTY"},
		{"Special chars", "Special !@# chars", "SPECIAL"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewDomainError(tc.message, tc.code)

			errorResult := err.Error()
			messageResult := err.Message()

			if errorResult != messageResult {
				t.Errorf("Expected Error() and Message() to return same value. Error(): '%s', Message(): '%s'",
					errorResult, messageResult)
			}

			if errorResult != tc.message {
				t.Errorf("Expected both methods to return '%s', got '%s'", tc.message, errorResult)
			}
		})
	}
}

func TestDomainError_ImmutabilityAfterCreation(t *testing.T) {
	// Test that the error properties don't change after creation
	originalMessage := "Original message"
	originalCode := "ORIGINAL_CODE"

	err := NewDomainError(originalMessage, originalCode)

	// Access the values multiple times
	for i := range 5 {
		if err.Error() != originalMessage {
			t.Errorf("Error message changed after %d accesses", i+1)
		}

		if err.Message() != originalMessage {
			t.Errorf("Message changed after %d accesses", i+1)
		}

		if err.Code() != originalCode {
			t.Errorf("Code changed after %d accesses", i+1)
		}
	}
}

func TestDomainError_EdgeCases(t *testing.T) {
	t.Run("Nil string handling", func(t *testing.T) {
		// Go doesn't have null strings, but test edge cases
		err := NewDomainError("", "")

		if err == nil {
			t.Fatal("Expected non-nil error even with empty strings")
		}

		if err.Error() != "" {
			t.Errorf("Expected empty error message, got '%s'", err.Error())
		}

		if err.Code() != "" {
			t.Errorf("Expected empty code, got '%s'", err.Code())
		}
	})

	t.Run("Very long strings", func(t *testing.T) {
		longMessage := make([]byte, 10000)
		for i := range longMessage {
			longMessage[i] = 'A'
		}
		longMessageStr := string(longMessage)

		longCode := make([]byte, 1000)
		for i := range longCode {
			longCode[i] = 'B'
		}
		longCodeStr := string(longCode)

		err := NewDomainError(longMessageStr, longCodeStr)

		if err.Error() != longMessageStr {
			t.Error("Long message not preserved correctly")
		}

		if err.Code() != longCodeStr {
			t.Error("Long code not preserved correctly")
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		err := NewDomainError("Concurrent test", "CONCURRENT")

		// Test concurrent access (should be safe as it's read-only)
		done := make(chan bool, 10)

		for range 10 {
			go func() {
				for range 100 {
					_ = err.Error()
					_ = err.Message()
					_ = err.Code()
				}
				done <- true
			}()
		}

		for range 10 {
			<-done
		}

		// Verify values are still correct after concurrent access
		if err.Error() != "Concurrent test" {
			t.Error("Error message corrupted after concurrent access")
		}

		if err.Code() != "CONCURRENT" {
			t.Error("Error code corrupted after concurrent access")
		}
	})
}
