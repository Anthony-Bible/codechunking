package service

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircuitBreakerTakesPrecedenceOverConnectionStatus tests that circuit breaker state
// takes precedence over connection status when determining health status messages.
// This test WILL FAIL with the current implementation because the health service
// checks connection status (lines 257-263) before circuit breaker status (lines 264-267).
func TestCircuitBreakerTakesPrecedenceOverConnectionStatus(t *testing.T) {
	tests := []struct {
		name            string
		connected       bool
		circuitBreaker  string
		lastError       string
		expectedMessage string
		description     string
	}{
		{
			name:            "disconnected_with_open_circuit_breaker_should_show_circuit_breaker",
			connected:       false,
			circuitBreaker:  "open",
			lastError:       "too many failures",
			expectedMessage: "circuit breaker",
			description:     "When both disconnected and circuit breaker is open, circuit breaker message should take precedence",
		},
		{
			name:            "connected_with_open_circuit_breaker_should_show_circuit_breaker",
			connected:       true,
			circuitBreaker:  "open",
			lastError:       "too many consecutive failures",
			expectedMessage: "circuit breaker",
			description:     "Even when connected, open circuit breaker should be the primary concern",
		},
		{
			name:            "disconnected_with_half_open_circuit_breaker_should_show_circuit_breaker",
			connected:       false,
			circuitBreaker:  "half-open",
			lastError:       "testing recovery",
			expectedMessage: "circuit breaker",
			description:     "Half-open circuit breaker should take precedence over disconnection status",
		},
		{
			name:            "disconnected_with_closed_circuit_breaker_should_show_disconnection",
			connected:       false,
			circuitBreaker:  "closed",
			lastError:       "network timeout",
			expectedMessage: "disconnected",
			description:     "When circuit breaker is closed, connection issues should be shown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
			mockRepo := &mockRepositoryRepo{findAllResult: nil}
			mockJobs := &mockIndexingJobRepo{}

			// Configure the specific scenario
			mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
				Connected:        tt.connected,
				LastError:        tt.lastError,
				Uptime:           "1m",
				Reconnects:       0,
				JetStreamEnabled: true,
				CircuitBreaker:   tt.circuitBreaker,
			})

			service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

			// Execute
			response, err := service.GetHealth(context.Background())

			// Assert - This will fail with current implementation due to wrong precedence order
			require.NoError(t, err)
			natsStatus := response.Dependencies["nats"]
			assert.Equal(t, "unhealthy", natsStatus.Status)

			// The key assertion that will fail: message should contain expected content
			assert.Contains(t, natsStatus.Message, tt.expectedMessage,
				"Test: %s - %s", tt.name, tt.description)

			// Verify circuit breaker state is properly reflected
			natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
			assert.Equal(t, tt.circuitBreaker, natsDetails.CircuitBreaker)
		})
	}
}

// TestCircuitBreakerOpenMessageSpecificity tests that when circuit breaker is open,
// the health check message specifically indicates circuit breaker protection.
// This test WILL FAIL because current implementation may return "NATS disconnected" message
// instead of circuit breaker message when both conditions are true.
func TestCircuitBreakerOpenMessageSpecificity(t *testing.T) {
	mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
	mockRepo := &mockRepositoryRepo{findAllResult: nil}
	mockJobs := &mockIndexingJobRepo{}

	// Configure circuit breaker open with disconnection
	mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
		Connected:        false, // Also disconnected
		LastError:        "too many failures",
		Uptime:           "0s",
		Reconnects:       10,
		JetStreamEnabled: false,
		CircuitBreaker:   "open", // Circuit breaker protection active
	})

	service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

	response, err := service.GetHealth(context.Background())
	require.NoError(t, err)

	natsStatus := response.Dependencies["nats"]
	assert.Equal(t, "unhealthy", natsStatus.Status)

	// This assertion will fail because current implementation returns:
	// "NATS disconnected: too many failures" instead of circuit breaker message
	assert.Contains(t, natsStatus.Message, "circuit breaker",
		"Message should specifically mention circuit breaker when it's open")

	// Should not contain disconnection message when circuit breaker is the primary issue
	assert.NotContains(t, natsStatus.Message, "NATS disconnected",
		"Should not show disconnection message when circuit breaker is open")
}

// TestCircuitBreakerStatesHealthImpact tests that different circuit breaker states
// produce appropriate health status and messages.
// These tests WILL FAIL because the current implementation doesn't properly handle
// circuit breaker states as the primary health indicator.
func TestCircuitBreakerStatesHealthImpact(t *testing.T) {
	tests := []struct {
		name                   string
		circuitBreakerState    string
		connected              bool
		expectedStatusHealth   string
		expectedMessageKeyword string
	}{
		{
			name:                   "circuit_breaker_open_makes_service_unhealthy",
			circuitBreakerState:    "open",
			connected:              true, // Even when connected
			expectedStatusHealth:   "unhealthy",
			expectedMessageKeyword: "circuit breaker open",
		},
		{
			name:                   "circuit_breaker_half_open_makes_service_unhealthy",
			circuitBreakerState:    "half-open",
			connected:              true,
			expectedStatusHealth:   "unhealthy",
			expectedMessageKeyword: "circuit breaker half-open",
		},
		{
			name:                   "circuit_breaker_closed_allows_other_checks",
			circuitBreakerState:    "closed",
			connected:              true,
			expectedStatusHealth:   "healthy", // Should pass other checks
			expectedMessageKeyword: "",        // No circuit breaker message expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
			mockRepo := &mockRepositoryRepo{findAllResult: nil}
			mockJobs := &mockIndexingJobRepo{}

			mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
				Connected:        tt.connected,
				LastError:        "",
				Uptime:           "5m",
				Reconnects:       0,
				JetStreamEnabled: true,
				CircuitBreaker:   tt.circuitBreakerState,
			})

			service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

			response, err := service.GetHealth(context.Background())
			require.NoError(t, err)

			natsStatus := response.Dependencies["nats"]
			assert.Equal(t, tt.expectedStatusHealth, natsStatus.Status)

			if tt.expectedMessageKeyword != "" {
				// This will fail for "open" and "half-open" states because current implementation
				// doesn't handle these states when connection is true
				assert.Contains(t, natsStatus.Message, tt.expectedMessageKeyword)
			}

			// Verify circuit breaker state in details
			natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
			assert.Equal(t, tt.circuitBreakerState, natsDetails.CircuitBreaker)
		})
	}
}

// TestAuthenticationErrorStillTakesPrecedenceOverCircuitBreaker verifies that
// authentication/authorization errors maintain highest precedence even over circuit breaker.
// This test should PASS with current implementation, but is included to document
// the expected precedence hierarchy.
func TestAuthenticationErrorStillTakesPrecedenceOverCircuitBreaker(t *testing.T) {
	mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
	mockRepo := &mockRepositoryRepo{findAllResult: nil}
	mockJobs := &mockIndexingJobRepo{}

	// Both authentication error AND circuit breaker open
	mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
		Connected:        false,
		LastError:        "nats: Authorization Violation",
		Uptime:           "0s",
		Reconnects:       0,
		JetStreamEnabled: false,
		CircuitBreaker:   "open", // Should be secondary to auth error
	})

	service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

	response, err := service.GetHealth(context.Background())
	require.NoError(t, err)

	natsStatus := response.Dependencies["nats"]
	assert.Equal(t, "unhealthy", natsStatus.Status)

	// Authentication error should take precedence
	assert.Contains(t, natsStatus.Message, "Authorization Violation")
	assert.NotContains(t, natsStatus.Message, "circuit breaker")
}

// TestCircuitBreakerDetailsReflection tests that circuit breaker information
// is properly included in health response details regardless of message precedence.
// This test should PASS but is included to ensure details are always populated.
func TestCircuitBreakerDetailsReflection(t *testing.T) {
	states := []string{"open", "half-open", "closed"}

	for _, state := range states {
		t.Run("circuit_breaker_state_"+state+"_in_details", func(t *testing.T) {
			mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
			mockRepo := &mockRepositoryRepo{findAllResult: nil}
			mockJobs := &mockIndexingJobRepo{}

			mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
				Connected:        true,
				LastError:        "",
				Uptime:           "10m",
				Reconnects:       0,
				JetStreamEnabled: true,
				CircuitBreaker:   state,
			})

			service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

			response, err := service.GetHealth(context.Background())
			require.NoError(t, err)

			// Circuit breaker state should always be reflected in details
			natsStatus := response.Dependencies["nats"]
			natsDetails := natsStatus.Details["nats_health"].(dto.NATSHealthDetails)
			assert.Equal(t, state, natsDetails.CircuitBreaker,
				"Circuit breaker state should be reflected in health details")
		})
	}
}

// TestCircuitBreakerWithHighReconnectCount tests the interaction between
// circuit breaker state and high reconnect counts.
// This test WILL FAIL because circuit breaker should take precedence over reconnect count.
func TestCircuitBreakerWithHighReconnectCount(t *testing.T) {
	mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
	mockRepo := &mockRepositoryRepo{findAllResult: nil}
	mockJobs := &mockIndexingJobRepo{}

	// High reconnect count (>10) AND circuit breaker open
	mockNATS.SetConnectionHealth(outbound.MessagePublisherHealthStatus{
		Connected:        true,
		LastError:        "circuit breaker protection",
		Uptime:           "2m",
		Reconnects:       15, // Would normally trigger "unstable connection" message
		JetStreamEnabled: true,
		CircuitBreaker:   "open", // Should take precedence
	})

	service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

	response, err := service.GetHealth(context.Background())
	require.NoError(t, err)

	natsStatus := response.Dependencies["nats"]
	assert.Equal(t, "unhealthy", natsStatus.Status)

	// Circuit breaker should take precedence over reconnect count
	// This will fail because current implementation checks reconnect count after connection check
	assert.Contains(t, natsStatus.Message, "circuit breaker")
	assert.NotContains(t, natsStatus.Message, "unstable connection")
	assert.NotContains(t, natsStatus.Message, "reconnects")
}

// TestExpectedPrecedenceOrder documents the expected precedence order for health status determination.
// This is a comprehensive test that WILL FAIL with multiple scenarios because the current
// implementation has incorrect precedence order.
func TestExpectedPrecedenceOrder(t *testing.T) {
	t.Run("precedence_hierarchy_documentation", func(t *testing.T) {
		// This test documents the expected precedence order:
		// 1. Authentication/Authorization errors (highest priority)
		// 2. Circuit breaker state (open/half-open)
		// 3. Connection status (disconnected)
		// 4. JetStream availability
		// 5. Connection stability (high reconnect count)
		// 6. Performance issues (high latency, slow consumer)

		scenarios := []struct {
			name          string
			status        outbound.MessagePublisherHealthStatus
			expectedMsg   string
			failureReason string
		}{
			{
				name: "auth_error_beats_circuit_breaker",
				status: outbound.MessagePublisherHealthStatus{
					Connected:        false,
					LastError:        "nats: Authorization Violation",
					CircuitBreaker:   "open",
					JetStreamEnabled: false,
					Reconnects:       20,
				},
				expectedMsg:   "Authorization Violation",
				failureReason: "Should pass - auth has highest precedence",
			},
			{
				name: "circuit_breaker_beats_disconnection",
				status: outbound.MessagePublisherHealthStatus{
					Connected:        false,
					LastError:        "too many failures",
					CircuitBreaker:   "open",
					JetStreamEnabled: false,
				},
				expectedMsg:   "circuit breaker",
				failureReason: "WILL FAIL - disconnection currently checked first",
			},
			{
				name: "circuit_breaker_beats_jetstream",
				status: outbound.MessagePublisherHealthStatus{
					Connected:        true,
					LastError:        "circuit protection active",
					CircuitBreaker:   "open",
					JetStreamEnabled: false, // Would normally trigger JetStream error
				},
				expectedMsg:   "circuit breaker",
				failureReason: "WILL FAIL - circuit breaker not checked when connected",
			},
			{
				name: "circuit_breaker_beats_high_reconnects",
				status: outbound.MessagePublisherHealthStatus{
					Connected:        true,
					LastError:        "protection enabled",
					CircuitBreaker:   "open",
					JetStreamEnabled: true,
					Reconnects:       25, // Would normally trigger reconnect error
				},
				expectedMsg:   "circuit breaker",
				failureReason: "WILL FAIL - reconnect count checked after connection",
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				mockNATS := testutil.NewMockMessagePublisherWithHealthMonitoring()
				mockRepo := &mockRepositoryRepo{findAllResult: nil}
				mockJobs := &mockIndexingJobRepo{}

				mockNATS.SetConnectionHealth(scenario.status)
				service := NewHealthServiceAdapter(mockRepo, mockJobs, mockNATS, "1.0.0")

				response, err := service.GetHealth(context.Background())
				require.NoError(t, err)

				natsStatus := response.Dependencies["nats"]
				assert.Contains(t, natsStatus.Message, scenario.expectedMsg,
					"Scenario: %s - %s", scenario.name, scenario.failureReason)
			})
		}
	})
}
