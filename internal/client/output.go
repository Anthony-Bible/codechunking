// Package client provides CLI output formatting utilities for structured JSON responses.
// It defines a standard envelope format for all CLI command outputs with consistent
// success/error handling and timestamp tracking.
package client

import (
	"encoding/json"
	"io"
	"time"
)

// Response represents the JSON output envelope for all CLI command outputs.
// It provides a consistent structure with a success flag, optional data payload,
// optional error information, and a timestamp. The Data and Error fields are
// mutually exclusive and use omitempty to exclude nil values from JSON output.
type Response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *Error      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Error represents structured error information in a CLI response.
// It includes a machine-readable error code, human-readable message,
// and optional details that can contain additional context as either
// a string, map, or array.
type Error struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// WriteSuccess writes a success response to the provided writer in JSON format.
// The data parameter can be any JSON-serializable value and will be included in
// the response envelope with success=true. A timestamp is automatically generated
// at the time of writing. Returns an error if JSON encoding or writing fails.
func WriteSuccess(w io.Writer, data interface{}) error {
	response := Response{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
	}
	return json.NewEncoder(w).Encode(response)
}

// WriteError writes an error response to the provided writer in JSON format.
// The code parameter should be a machine-readable error code (e.g., "NOT_FOUND"),
// the message parameter provides a human-readable description, and the optional
// details parameter can contain additional context as a string, map, or array.
// A timestamp is automatically generated at the time of writing. Returns an error
// if JSON encoding or writing fails.
func WriteError(w io.Writer, code, message string, details interface{}) error {
	response := Response{
		Success: false,
		Error: &Error{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now(),
	}
	return json.NewEncoder(w).Encode(response)
}
