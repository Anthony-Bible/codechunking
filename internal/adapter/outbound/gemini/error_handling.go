package gemini

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// HandleHTTPError handles HTTP errors and converts them to EmbeddingError with enhanced context.
func (c *Client) HandleHTTPError(ctx context.Context, response *http.Response) *outbound.EmbeddingError {
	body, readErr := io.ReadAll(response.Body)
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			slogger.Error(ctx, "Failed to close response body", slogger.Fields{
				"error": closeErr.Error(),
			})
		}
	}()

	var errorResp ErrorResponse
	var apiErrorMessage string

	// Try to parse the API error response for more context
	if readErr == nil && len(body) > 0 {
		if unmarshalErr := json.Unmarshal(body, &errorResp); unmarshalErr == nil {
			if errorResp.Error.Message != "" {
				apiErrorMessage = errorResp.Error.Message
			}
		}
	}

	// Log the HTTP error for debugging
	slogger.Error(ctx, "HTTP error received from Gemini API", slogger.Fields{
		"status_code":     response.StatusCode,
		"status":          response.Status,
		"response_length": len(body),
		"api_message":     apiErrorMessage,
	})

	// Create appropriate error based on status code
	switch response.StatusCode {
	case http.StatusUnauthorized:
		message := fmt.Sprintf("Invalid API key provided (HTTP %d)", response.StatusCode)
		if apiErrorMessage != "" {
			message = fmt.Sprintf("Authentication failed (HTTP %d): %s", response.StatusCode, apiErrorMessage)
		}
		return &outbound.EmbeddingError{
			Code:      "invalid_api_key",
			Type:      "auth",
			Message:   message,
			Retryable: false,
		}

	case http.StatusTooManyRequests:
		message := fmt.Sprintf("Rate limit exceeded (HTTP %d). Please retry after some time", response.StatusCode)
		retryAfter := response.Header.Get("Retry-After")
		if retryAfter != "" {
			message = fmt.Sprintf(
				"Rate limit exceeded (HTTP %d). Retry after %s seconds",
				response.StatusCode,
				retryAfter,
			)
		}
		if apiErrorMessage != "" {
			message = fmt.Sprintf("%s: %s", message, apiErrorMessage)
		}
		return &outbound.EmbeddingError{
			Code:      "rate_limit_exceeded",
			Type:      "quota",
			Message:   message,
			Retryable: true,
		}

	case http.StatusBadRequest:
		message := fmt.Sprintf("Invalid request parameters (HTTP %d)", response.StatusCode)
		if apiErrorMessage != "" {
			message = fmt.Sprintf("Bad request (HTTP %d): %s", response.StatusCode, apiErrorMessage)
		}
		return &outbound.EmbeddingError{
			Code:      "invalid_request",
			Type:      "validation",
			Message:   message,
			Retryable: false,
		}

	case http.StatusForbidden:
		message := fmt.Sprintf("Access denied (HTTP %d). Check API key permissions", response.StatusCode)
		if apiErrorMessage != "" {
			message = fmt.Sprintf("Access forbidden (HTTP %d): %s", response.StatusCode, apiErrorMessage)
		}
		return &outbound.EmbeddingError{
			Code:      "access_denied",
			Type:      "auth",
			Message:   message,
			Retryable: false,
		}

	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		message := fmt.Sprintf("Server error (HTTP %d) occurred. Please retry", response.StatusCode)
		if apiErrorMessage != "" {
			message = fmt.Sprintf("Server error (HTTP %d): %s", response.StatusCode, apiErrorMessage)
		}
		return &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   message,
			Retryable: true,
		}

	default:
		message := fmt.Sprintf("HTTP error: %s", response.Status)
		if apiErrorMessage != "" {
			message = fmt.Sprintf("%s - %s", message, apiErrorMessage)
		}
		return &outbound.EmbeddingError{
			Code:      "http_error",
			Type:      "server",
			Message:   message,
			Retryable: response.StatusCode >= 500, // Retry server errors
		}
	}
}

// HandleNetworkError handles network errors and converts them to EmbeddingError.
func (c *Client) HandleNetworkError(ctx context.Context, err error) *outbound.EmbeddingError {
	if errors.Is(err, context.Canceled) {
		return &outbound.EmbeddingError{
			Code:      "request_canceled",
			Type:      "network",
			Message:   "request was canceled",
			Retryable: false,
			Cause:     err,
		}
	}

	// Check for timeout
	var netErr net.Error
	if errors.As(err, &netErr) {
		return &outbound.EmbeddingError{
			Code:      "connection_timeout",
			Type:      "network",
			Message:   "connection timeout",
			Retryable: true,
			Cause:     err,
		}
	}

	// Check for connection refused
	if strings.Contains(err.Error(), "connection refused") {
		return &outbound.EmbeddingError{
			Code:      "connection_refused",
			Type:      "network",
			Message:   "connection refused",
			Retryable: true,
			Cause:     err,
		}
	}

	return &outbound.EmbeddingError{
		Code:      "network_error",
		Type:      "network",
		Message:   err.Error(),
		Retryable: true,
		Cause:     err,
	}
}

// CreateEmbeddingError creates a new EmbeddingError.
func (c *Client) CreateEmbeddingError(
	ctx context.Context,
	code, errorType, message string,
	retryable bool,
	cause error,
) *outbound.EmbeddingError {
	return &outbound.EmbeddingError{
		Code:      code,
		Type:      errorType,
		Message:   message,
		Retryable: retryable,
		Cause:     cause,
	}
}

// ValidateEmbeddingInput validates input before sending embedding request.
func (c *Client) ValidateEmbeddingInput(ctx context.Context, text string, options outbound.EmbeddingOptions) error {
	if text == "" || strings.TrimSpace(text) == "" {
		return &outbound.EmbeddingError{
			Code:      "empty_text",
			Type:      "validation",
			Message:   "text content cannot be empty",
			Retryable: false,
		}
	}

	tokenCount, _ := c.EstimateTokenCount(ctx, text)
	if tokenCount > options.MaxTokens {
		return &outbound.EmbeddingError{
			Code:      "token_limit_exceeded",
			Type:      "validation",
			Message:   fmt.Sprintf("text exceeds maximum token limit of %d", options.MaxTokens),
			Retryable: false,
		}
	}

	return nil
}
