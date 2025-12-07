package commands

import (
	"codechunking/internal/client"
	"context"
	"strings"

	"github.com/spf13/cobra"
)

// Error codes for health command failures.
const (
	errCodeInvalidConfig   = "INVALID_CONFIG"
	errCodeClientError     = "CLIENT_ERROR"
	errCodeConnectionError = "CONNECTION_ERROR"
	errCodeTimeoutError    = "TIMEOUT_ERROR"
	errCodeServerError     = "SERVER_ERROR"
	errCodeAPIError        = "API_ERROR"
	errCodeInvalidArgument = "INVALID_ARGUMENT"
	errCodeNotFound        = "NOT_FOUND"
)

// NewHealthCmd creates and returns the health check command.
// The command queries the API server's /health endpoint and outputs
// the response in JSON format. It respects global flags for API URL
// and timeout configuration.
//
// The command writes JSON output to stdout:
//   - Success: {"success": true, "data": {...}}
//   - Failure: {"success": false, "error": {...}}
//
// All errors are handled gracefully with appropriate error codes
// and return nil to prevent cobra from printing usage information.
func NewHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check API server health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			apiURL, _ := cmd.Flags().GetString("api-url")
			timeout, _ := cmd.Flags().GetDuration("timeout")

			cfg := &client.Config{APIURL: apiURL, Timeout: timeout}
			if err := cfg.Validate(); err != nil {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidConfig, err.Error(), nil)
				return nil
			}

			c, err := client.NewClient(cfg)
			if err != nil {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeClientError, err.Error(), nil)
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			health, err := c.Health(ctx)
			if err != nil {
				code := determineErrorCode(err)
				_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
				return nil
			}

			return client.WriteSuccess(cmd.OutOrStdout(), health)
		},
	}
}

// determineErrorCode analyzes the error message to classify the failure type.
// It returns an appropriate error code for common failure scenarios:
//   - CONNECTION_ERROR: Network connectivity issues
//   - TIMEOUT_ERROR: Request timeout or deadline exceeded
//   - NOT_FOUND: HTTP 404 status code
//   - SERVER_ERROR: HTTP 500 status code
//   - API_ERROR: Generic API errors
func determineErrorCode(err error) string {
	errStr := err.Error()
	if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host") {
		return errCodeConnectionError
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
		return errCodeTimeoutError
	}
	if strings.Contains(errStr, "404") {
		return errCodeNotFound
	}
	if strings.Contains(errStr, "500") {
		return errCodeServerError
	}
	return errCodeAPIError
}
