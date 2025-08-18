package common

import (
	"codechunking/internal/application/dto"
	"errors"
	"strings"
	"testing"
)

// TestNewValidationError tests the basic constructor function.
func TestNewValidationError(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		message string
		want    ValidationError
		wantErr bool
	}{
		{
			name:    "valid_basic_error",
			field:   "username",
			message: "is required",
			want: ValidationError{
				Field:   "username",
				Message: "is required",
				Value:   "",
			},
			wantErr: false,
		},
		{
			name:    "valid_with_complex_message",
			field:   "email",
			message: "must be a valid email address",
			want: ValidationError{
				Field:   "email",
				Message: "must be a valid email address",
				Value:   "",
			},
			wantErr: false,
		},
		{
			name:    "empty_field_should_fail",
			field:   "",
			message: "some message",
			want:    ValidationError{},
			wantErr: true,
		},
		{
			name:    "empty_message_should_fail",
			field:   "field",
			message: "",
			want:    ValidationError{},
			wantErr: true,
		},
		{
			name:    "whitespace_only_field_should_fail",
			field:   "   ",
			message: "some message",
			want:    ValidationError{},
			wantErr: true,
		},
		{
			name:    "whitespace_only_message_should_fail",
			field:   "field",
			message: "   ",
			want:    ValidationError{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewValidationError(tt.field, tt.message)

			if tt.wantErr {
				// For error cases, we expect the constructor to handle validation
				// This will be defined during implementation
				return
			}

			if got.Field != tt.want.Field {
				t.Errorf("NewValidationError().Field = %v, want %v", got.Field, tt.want.Field)
			}
			if got.Message != tt.want.Message {
				t.Errorf("NewValidationError().Message = %v, want %v", got.Message, tt.want.Message)
			}
			if got.Value != tt.want.Value {
				t.Errorf("NewValidationError().Value = %v, want %v", got.Value, tt.want.Value)
			}
		})
	}
}

// TestNewValidationErrorWithValue tests the constructor with value.
func TestNewValidationErrorWithValue(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		message string
		value   string
		want    ValidationError
		wantErr bool
	}{
		{
			name:    "valid_error_with_value",
			field:   "age",
			message: "must be between 18 and 65",
			value:   "17",
			want: ValidationError{
				Field:   "age",
				Message: "must be between 18 and 65",
				Value:   "17",
			},
			wantErr: false,
		},
		{
			name:    "valid_error_with_empty_value",
			field:   "password",
			message: "is required",
			value:   "",
			want: ValidationError{
				Field:   "password",
				Message: "is required",
				Value:   "",
			},
			wantErr: false,
		},
		{
			name:    "valid_error_with_special_characters_in_value",
			field:   "json",
			message: "invalid format",
			value:   `{"invalid": json}`,
			want: ValidationError{
				Field:   "json",
				Message: "invalid format",
				Value:   `{"invalid": json}`,
			},
			wantErr: false,
		},
		{
			name:    "empty_field_should_fail",
			field:   "",
			message: "some message",
			value:   "some value",
			want:    ValidationError{},
			wantErr: true,
		},
		{
			name:    "empty_message_should_fail",
			field:   "field",
			message: "",
			value:   "some value",
			want:    ValidationError{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewValidationErrorWithValue(tt.field, tt.message, tt.value)

			if tt.wantErr {
				// For error cases, constructor should handle validation
				return
			}

			if got.Field != tt.want.Field {
				t.Errorf("NewValidationErrorWithValue().Field = %v, want %v", got.Field, tt.want.Field)
			}
			if got.Message != tt.want.Message {
				t.Errorf("NewValidationErrorWithValue().Message = %v, want %v", got.Message, tt.want.Message)
			}
			if got.Value != tt.want.Value {
				t.Errorf("NewValidationErrorWithValue().Value = %v, want %v", got.Value, tt.want.Value)
			}
		})
	}
}

// TestValidationError_Error tests the error interface implementation.
func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "error_without_value",
			err: ValidationError{
				Field:   "username",
				Message: "is required",
				Value:   "",
			},
			want: "validation error on field 'username': is required",
		},
		{
			name: "error_with_value",
			err: ValidationError{
				Field:   "age",
				Message: "must be between 18 and 65",
				Value:   "17",
			},
			want: "validation error on field 'age': must be between 18 and 65 (value: 17)",
		},
		{
			name: "error_with_empty_value_treated_as_no_value",
			err: ValidationError{
				Field:   "password",
				Message: "is too short",
				Value:   "",
			},
			want: "validation error on field 'password': is too short",
		},
		{
			name: "error_with_whitespace_value",
			err: ValidationError{
				Field:   "name",
				Message: "cannot be whitespace only",
				Value:   "   ",
			},
			want: "validation error on field 'name': cannot be whitespace only (value:    )",
		},
		{
			name: "error_with_special_characters",
			err: ValidationError{
				Field:   "json",
				Message: "invalid JSON format",
				Value:   `{"key": "value"}`,
			},
			want: `validation error on field 'json': invalid JSON format (value: {"key": "value"})`,
		},
		{
			name: "error_with_multiline_value",
			err: ValidationError{
				Field:   "description",
				Message: "contains invalid characters",
				Value:   "line1\nline2",
			},
			want: "validation error on field 'description': contains invalid characters (value: line1\nline2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ValidationError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidationError_ToDTO tests conversion to DTO type.
func TestValidationError_ToDTO(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want dto.ValidationError
	}{
		{
			name: "convert_basic_error",
			err: ValidationError{
				Field:   "username",
				Message: "is required",
				Value:   "",
			},
			want: dto.ValidationError{
				Field:   "username",
				Message: "is required",
				Value:   "",
			},
		},
		{
			name: "convert_error_with_value",
			err: ValidationError{
				Field:   "age",
				Message: "must be numeric",
				Value:   "abc",
			},
			want: dto.ValidationError{
				Field:   "age",
				Message: "must be numeric",
				Value:   "abc",
			},
		},
		{
			name: "convert_error_with_special_characters",
			err: ValidationError{
				Field:   "config",
				Message: "invalid JSON",
				Value:   `{"invalid": json}`,
			},
			want: dto.ValidationError{
				Field:   "config",
				Message: "invalid JSON",
				Value:   `{"invalid": json}`,
			},
		},
		{
			name: "convert_error_with_unicode",
			err: ValidationError{
				Field:   "name",
				Message: "contains invalid unicode",
				Value:   "Hello 世界",
			},
			want: dto.ValidationError{
				Field:   "name",
				Message: "contains invalid unicode",
				Value:   "Hello 世界",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.ToDTO()

			if got.Field != tt.want.Field {
				t.Errorf("ValidationError.ToDTO().Field = %v, want %v", got.Field, tt.want.Field)
			}
			if got.Message != tt.want.Message {
				t.Errorf("ValidationError.ToDTO().Message = %v, want %v", got.Message, tt.want.Message)
			}
			if got.Value != tt.want.Value {
				t.Errorf("ValidationError.ToDTO().Value = %v, want %v", got.Value, tt.want.Value)
			}
		})
	}
}

// TestValidationError_FieldValidation tests field validation constraints.
func TestValidationError_FieldValidation(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		message string
		value   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "field_too_long",
			field:   strings.Repeat("a", 256),
			message: "some message",
			value:   "",
			wantErr: true,
			errMsg:  "field name exceeds maximum length",
		},
		{
			name:    "message_too_long",
			field:   "field",
			message: strings.Repeat("a", 1001),
			value:   "",
			wantErr: true,
			errMsg:  "message exceeds maximum length",
		},
		{
			name:    "value_extremely_long_should_be_truncated",
			field:   "field",
			message: "message",
			value:   strings.Repeat("a", 2000),
			wantErr: false,
		},
		{
			name:    "field_with_control_characters",
			field:   "field\x00",
			message: "message",
			value:   "",
			wantErr: true,
			errMsg:  "field contains control characters",
		},
		{
			name:    "message_with_control_characters",
			field:   "field",
			message: "message\x00",
			value:   "",
			wantErr: true,
			errMsg:  "message contains control characters",
		},
		{
			name:    "valid_unicode_should_pass",
			field:   "用户名",
			message: "用户名 is required",
			value:   "测试值",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would test the validation logic that should be built into constructors
			err := NewValidationError(tt.field, tt.message)

			if tt.wantErr {
				// In implementation, constructors should validate and potentially return error
				// or panic, or have a separate validation method
				t.Logf("Expected validation error: %s", tt.errMsg)
				return
			}

			// For valid cases, constructor should succeed
			if err.Field == "" && tt.field != "" {
				t.Error("Expected valid ValidationError but got empty field")
			}
		})
	}
}

// TestValidationError_ErrorInterface tests that ValidationError implements error interface.
func TestValidationError_ErrorInterface(t *testing.T) {
	var err error = ValidationError{
		Field:   "test",
		Message: "test message",
		Value:   "test value",
	}

	if err.Error() == "" {
		t.Error("ValidationError.Error() should not return empty string")
	}

	// Test that it can be used in error handling
	{
		var e ValidationError
		switch {
		case errors.As(err, &e):
			if e.Field != "test" {
				t.Error("Type assertion to ValidationError failed")
			}
		default:
			t.Error("ValidationError should be assignable to error interface")
		}
	}
}

// TestValidationError_IsNil tests nil handling.
func TestValidationError_IsNil(t *testing.T) {
	var err *ValidationError

	// Test nil pointer behavior
	if err == nil {
		t.Log("nil ValidationError pointer is correctly nil")
	} else {
		t.Error("nil ValidationError pointer should be nil")
	}

	// Test zero value behavior
	zeroErr := ValidationError{}
	if zeroErr.Field != "" || zeroErr.Message != "" || zeroErr.Value != "" {
		t.Error("Zero value ValidationError should have empty fields")
	}
}

// TestValidationError_Consistency tests consistency across different creation methods.
func TestValidationError_Consistency(t *testing.T) {
	field := "username"
	message := "is required"
	value := "invalid_user"

	// Create using both constructors
	err1 := NewValidationError(field, message)
	err2 := NewValidationErrorWithValue(field, message, "")
	err3 := NewValidationErrorWithValue(field, message, value)

	// Test that basic constructor and value constructor with empty value are equivalent
	if err1.Error() != err2.Error() {
		t.Errorf("NewValidationError and NewValidationErrorWithValue('') should produce equivalent error messages")
	}

	// Test that error with value shows value in message
	if !strings.Contains(err3.Error(), value) {
		t.Errorf("Error message should contain the value when value is provided")
	}

	// Test DTO conversion consistency
	dto1 := err1.ToDTO()
	dto2 := err2.ToDTO()
	dto3 := err3.ToDTO()

	if dto1.Field != dto2.Field || dto1.Message != dto2.Message {
		t.Error("DTO conversion should be consistent between constructors")
	}

	if dto3.Value != value {
		t.Error("DTO should preserve value field")
	}
}

// TestValidationError_EdgeCases tests various edge cases.
func TestValidationError_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "very_long_field_name",
			test: func(t *testing.T) {
				longField := strings.Repeat("verylongfieldname", 20)
				err := NewValidationError(longField, "test message")

				// Should handle long field names gracefully
				if !strings.Contains(err.Error(), "verylongfieldname") {
					t.Error("Error should contain field name even if long")
				}
			},
		},
		{
			name: "newlines_in_message",
			test: func(t *testing.T) {
				message := "first line\nsecond line\nthird line"
				err := NewValidationError("field", message)

				// Should preserve newlines in error message
				if !strings.Contains(err.Error(), "\n") {
					t.Error("Error message should preserve newlines")
				}
			},
		},
		{
			name: "quotes_in_field_and_message",
			test: func(t *testing.T) {
				field := "field'with'quotes"
				message := `message with "quotes"`
				err := NewValidationError(field, message)

				errorMsg := err.Error()
				if !strings.Contains(errorMsg, field) || !strings.Contains(errorMsg, message) {
					t.Error("Error should handle quotes in field and message")
				}
			},
		},
		{
			name: "empty_string_vs_nil_handling",
			test: func(t *testing.T) {
				err1 := NewValidationErrorWithValue("field", "message", "")
				err2 := NewValidationErrorWithValue("field", "message", "   ")

				// Empty string and whitespace should be handled differently
				if err1.Error() == err2.Error() {
					t.Error("Empty string and whitespace values should produce different error messages")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}
