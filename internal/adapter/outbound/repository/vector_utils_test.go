package repository

import (
	"reflect"
	"testing"
)

func TestVectorToString(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected string
	}{
		{
			name:     "empty vector",
			input:    []float64{},
			expected: "[]",
		},
		{
			name:     "single element",
			input:    []float64{1.0},
			expected: "[1]",
		},
		{
			name:     "multiple elements",
			input:    []float64{1.0, 2.5, -3.14},
			expected: "[1,2.5,-3.14]",
		},
		{
			name:     "typical embedding vector",
			input:    []float64{0.1, 0.2, 0.3, -0.1},
			expected: "[0.1,0.2,0.3,-0.1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VectorToString(tt.input)
			if result != tt.expected {
				t.Errorf("VectorToString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestStringToVector(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []float64
		hasError bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []float64{},
			hasError: false,
		},
		{
			name:     "empty brackets",
			input:    "[]",
			expected: []float64{},
			hasError: false,
		},
		{
			name:     "single element",
			input:    "[1.0]",
			expected: []float64{1.0},
			hasError: false,
		},
		{
			name:     "multiple elements",
			input:    "[1.0,2.5,-3.14]",
			expected: []float64{1.0, 2.5, -3.14},
			hasError: false,
		},
		{
			name:     "spaces in input",
			input:    "[1.0, 2.5, -3.14]",
			expected: []float64{1.0, 2.5, -3.14},
			hasError: false,
		},
		{
			name:     "invalid number",
			input:    "[1.0,invalid,3.14]",
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := StringToVector(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("StringToVector() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("StringToVector() unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("StringToVector() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestVectorRoundTrip(t *testing.T) {
	// Test that converting a vector to string and back gives the same result
	original := []float64{0.1, 0.2, 0.3, -0.1, 0.0, 1.5, -2.7}

	vectorString := VectorToString(original)
	recovered, err := StringToVector(vectorString)
	if err != nil {
		t.Fatalf("StringToVector() error: %v", err)
	}

	if !reflect.DeepEqual(original, recovered) {
		t.Errorf("Round trip failed: original %v, recovered %v", original, recovered)
	}
}
