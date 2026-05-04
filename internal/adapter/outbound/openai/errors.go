package openai

import (
	"codechunking/internal/port/outbound"
	"net/http"
)

// classifyHTTPStatus maps an HTTP status + parsed error envelope into an
// *outbound.EmbeddingError so downstream code can use the existing
// IsAuthenticationError / IsQuotaError / IsValidationError predicates.
func classifyHTTPStatus(status int, body errorBody) *outbound.EmbeddingError {
	e := &outbound.EmbeddingError{
		Code:    body.Code,
		Message: body.Message,
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		e.Type = "auth"
		if e.Code == "" {
			e.Code = "unauthorized"
		}
		e.Retryable = false
	case status == http.StatusTooManyRequests:
		e.Type = "quota"
		if e.Code == "" {
			e.Code = "rate_limit_exceeded"
		}
		e.Retryable = true
	case status >= 500:
		e.Type = "server"
		if e.Code == "" {
			e.Code = "server_error"
		}
		e.Retryable = true
	case status == http.StatusBadRequest:
		e.Type = "validation"
		if e.Code == "" {
			e.Code = "invalid_input"
		}
		e.Retryable = false
	default:
		e.Type = "unknown"
		if e.Code == "" {
			e.Code = "unknown_error"
		}
		e.Retryable = false
	}
	if e.Message == "" {
		e.Message = http.StatusText(status)
	}
	return e
}
