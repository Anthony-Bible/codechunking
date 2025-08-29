package entity

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAlert_Creation(t *testing.T) {
	t.Run("should create critical error alert", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"system_failure",
			"Critical system failure detected",
			map[string]interface{}{
				"component": "database",
				"host":      "db-primary",
			},
		)

		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, err := NewAlert(
			classifiedError,
			alertType,
			"System failure requires immediate attention",
		)

		assert.NoError(t, err)
		assert.NotNil(t, alert)
		assert.NotEmpty(t, alert.ID())
		assert.Equal(t, classifiedError.ID(), alert.ErrorID())
		assert.Equal(t, alertType, alert.Type())
		assert.Equal(t, "System failure requires immediate attention", alert.Message())
		assert.Equal(t, severity, alert.Severity())
		assert.WithinDuration(t, time.Now(), alert.CreatedAt(), time.Second)
		assert.Equal(t, AlertStatusPending, alert.Status())
		assert.Nil(t, alert.SentAt())
		assert.Empty(t, alert.Recipients())
		assert.Empty(t, alert.DeliveryAttempts())
	})

	t.Run("should create batch aggregation alert", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Create multiple errors for aggregation
		errors := make([]*ClassifiedError, 3)
		for i := range 3 {
			error, _ := NewClassifiedError(
				ctx,
				assert.AnError,
				severity,
				"database_connection_failure",
				"Database connection failed",
				nil,
			)
			errors[i] = error
		}

		aggregation, _ := NewErrorAggregation("database_connection_failure:ERROR", time.Minute*5)
		for _, error := range errors {
			aggregation.AddError(error)
		}

		alertType, _ := valueobject.NewAlertType("BATCH")
		alert, err := NewAggregationAlert(
			aggregation,
			alertType,
			"Multiple database connection failures detected",
		)

		assert.NoError(t, err)
		assert.NotNil(t, alert)
		assert.Equal(t, aggregation.Pattern(), alert.AggregationPattern())
		assert.Equal(t, 3, alert.ErrorCount())
		assert.True(t, alert.IsBatchAlert())
		assert.False(t, alert.IsRealTimeAlert())
	})

	t.Run("should create cascade failure alert", func(t *testing.T) {
		// Create cascade of errors
		aggregations := make([]*ErrorAggregation, 3)
		patterns := []string{
			"database_connection_failure:CRITICAL",
			"api_request_failure:ERROR",
			"worker_processing_failure:ERROR",
		}

		for i, pattern := range patterns {
			agg, _ := NewErrorAggregation(pattern, time.Minute*5)
			aggregations[i] = agg
		}

		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, err := NewCascadeFailureAlert(
			aggregations,
			alertType,
			"Cascade failure detected - multiple systems affected",
		)

		assert.NoError(t, err)
		assert.NotNil(t, alert)
		assert.Len(t, alert.RelatedAggregations(), 3)
		assert.True(t, alert.IsCascadeAlert())
		assert.Equal(t, "cascade_failure", alert.AlertCategory())
	})
}

func TestAlert_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	severity, _ := valueobject.NewErrorSeverity("ERROR")
	classifiedError, _ := NewClassifiedError(ctx, assert.AnError, severity, "test_error", "Test error", nil)
	alertType, _ := valueobject.NewAlertType("REAL_TIME")

	testCases := []struct {
		name            string
		classifiedError *ClassifiedError
		alertType       *valueobject.AlertType
		message         string
		expectedError   string
	}{
		{
			name:            "nil classified error",
			classifiedError: nil,
			alertType:       alertType,
			message:         "Test message",
			expectedError:   "classified error cannot be nil",
		},
		{
			name:            "nil alert type",
			classifiedError: classifiedError,
			alertType:       nil,
			message:         "Test message",
			expectedError:   "alert type cannot be nil",
		},
		{
			name:            "empty message",
			classifiedError: classifiedError,
			alertType:       alertType,
			message:         "",
			expectedError:   "alert message cannot be empty",
		},
		{
			name:            "message too long",
			classifiedError: classifiedError,
			alertType:       alertType,
			message:         generateLongString(1001), // Over 1000 character limit
			expectedError:   "alert message cannot exceed 1000 characters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alert, err := NewAlert(tc.classifiedError, tc.alertType, tc.message)

			assert.Error(t, err)
			assert.Nil(t, alert)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestAlert_Recipients(t *testing.T) {
	t.Run("should add recipients to alert", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"system_failure",
			"Critical failure",
			nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Critical failure alert")

		recipients := []AlertRecipient{
			{Type: "email", Address: "oncall@company.com", Name: "On-call Team"},
			{Type: "slack", Address: "#alerts", Name: "Alerts Channel"},
			{Type: "pagerduty", Address: "pd-service-key", Name: "PagerDuty Service"},
		}

		err := alert.AddRecipients(recipients)
		assert.NoError(t, err)
		assert.Len(t, alert.Recipients(), 3)

		// Check specific recipients
		emailRecipient := alert.FindRecipient("email", "oncall@company.com")
		assert.NotNil(t, emailRecipient)
		assert.Equal(t, "On-call Team", emailRecipient.Name)
	})

	t.Run("should validate recipients", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := NewClassifiedError(ctx, assert.AnError, severity, "test_error", "Test", nil)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Test alert")

		invalidRecipients := []AlertRecipient{
			{Type: "", Address: "test@example.com", Name: "Test"},
			{Type: "email", Address: "", Name: "Test"},
			{Type: "email", Address: "invalid-email", Name: "Test"},
		}

		err := alert.AddRecipients(invalidRecipients)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid recipient")
	})

	t.Run("should remove recipients", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := NewClassifiedError(ctx, assert.AnError, severity, "test_error", "Test", nil)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Test alert")

		recipients := []AlertRecipient{
			{Type: "email", Address: "test1@example.com", Name: "Test 1"},
			{Type: "email", Address: "test2@example.com", Name: "Test 2"},
		}
		alert.AddRecipients(recipients)

		err := alert.RemoveRecipient("email", "test1@example.com")
		assert.NoError(t, err)
		assert.Len(t, alert.Recipients(), 1)

		remaining := alert.FindRecipient("email", "test2@example.com")
		assert.NotNil(t, remaining)
	})
}

func TestAlert_DeliveryTracking(t *testing.T) {
	t.Run("should track delivery attempts", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"system_failure",
			"Critical failure",
			nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Critical failure alert")
		recipients := []AlertRecipient{
			{Type: "email", Address: "oncall@company.com", Name: "On-call Team"},
		}
		alert.AddRecipients(recipients)

		// Record successful delivery attempt
		deliveryResult := DeliveryResult{
			RecipientType:    "email",
			RecipientAddress: "oncall@company.com",
			Success:          true,
			AttemptedAt:      time.Now(),
			DeliveredAt:      time.Now(),
			Error:            "",
		}

		err := alert.RecordDeliveryAttempt(deliveryResult)
		assert.NoError(t, err)
		assert.Len(t, alert.DeliveryAttempts(), 1)
		assert.Equal(t, AlertStatusSent, alert.Status())
		assert.NotNil(t, alert.SentAt())
	})

	t.Run("should track failed delivery attempts", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := NewClassifiedError(ctx, assert.AnError, severity, "test_error", "Test", nil)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Test alert")
		recipients := []AlertRecipient{
			{Type: "email", Address: "test@example.com", Name: "Test"},
		}
		alert.AddRecipients(recipients)

		// Record failed delivery attempt
		deliveryResult := DeliveryResult{
			RecipientType:    "email",
			RecipientAddress: "test@example.com",
			Success:          false,
			AttemptedAt:      time.Now(),
			Error:            "SMTP server unreachable",
		}

		err := alert.RecordDeliveryAttempt(deliveryResult)
		assert.NoError(t, err)
		assert.Len(t, alert.DeliveryAttempts(), 1)
		assert.Equal(t, AlertStatusFailed, alert.Status())
		assert.Nil(t, alert.SentAt())
	})

	t.Run("should handle partial delivery success", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"system_failure",
			"Critical failure",
			nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Critical failure alert")
		recipients := []AlertRecipient{
			{Type: "email", Address: "primary@company.com", Name: "Primary"},
			{Type: "slack", Address: "#alerts", Name: "Slack"},
		}
		alert.AddRecipients(recipients)

		// First delivery succeeds
		result1 := DeliveryResult{
			RecipientType:    "email",
			RecipientAddress: "primary@company.com",
			Success:          true,
			AttemptedAt:      time.Now(),
			DeliveredAt:      time.Now(),
		}
		alert.RecordDeliveryAttempt(result1)

		// Second delivery fails
		result2 := DeliveryResult{
			RecipientType:    "slack",
			RecipientAddress: "#alerts",
			Success:          false,
			AttemptedAt:      time.Now(),
			Error:            "Slack API error",
		}
		alert.RecordDeliveryAttempt(result2)

		assert.Len(t, alert.DeliveryAttempts(), 2)
		assert.Equal(t, AlertStatusPartiallyDelivered, alert.Status())
		assert.True(t, alert.HasFailedDeliveries())
		assert.True(t, alert.HasSuccessfulDeliveries())
	})

	t.Run("should support retry logic", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := NewClassifiedError(ctx, assert.AnError, severity, "test_error", "Test", nil)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Test alert")
		recipients := []AlertRecipient{
			{Type: "email", Address: "test@example.com", Name: "Test"},
		}
		alert.AddRecipients(recipients)

		// First attempt fails
		failedResult := DeliveryResult{
			RecipientType:    "email",
			RecipientAddress: "test@example.com",
			Success:          false,
			AttemptedAt:      time.Now(),
			Error:            "Temporary failure",
		}
		alert.RecordDeliveryAttempt(failedResult)

		// Check retry eligibility
		assert.True(t, alert.IsEligibleForRetry())
		assert.Equal(t, 1, alert.RetryAttempts("email", "test@example.com"))

		// Retry succeeds
		retryResult := DeliveryResult{
			RecipientType:    "email",
			RecipientAddress: "test@example.com",
			Success:          true,
			AttemptedAt:      time.Now(),
			DeliveredAt:      time.Now(),
		}
		alert.RecordDeliveryAttempt(retryResult)

		assert.Equal(t, AlertStatusSent, alert.Status())
		assert.Equal(t, 2, alert.RetryAttempts("email", "test@example.com"))
	})
}

func TestAlert_Deduplication(t *testing.T) {
	t.Run("should generate consistent deduplication key", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Create two alerts for similar errors
		error1, _ := NewClassifiedError(ctx, assert.AnError, severity, "database_failure", "DB connection failed", nil)
		error2, _ := NewClassifiedError(ctx, assert.AnError, severity, "database_failure", "DB connection failed", nil)

		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert1, _ := NewAlert(error1, alertType, "Database failure alert")
		alert2, _ := NewAlert(error2, alertType, "Database failure alert")

		// Should have same deduplication key
		assert.Equal(t, alert1.DeduplicationKey(), alert2.DeduplicationKey())
		assert.Contains(t, alert1.DeduplicationKey(), "database_failure")
		assert.Contains(t, alert1.DeduplicationKey(), "ERROR")
	})

	t.Run("should have different deduplication keys for different errors", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		error1, _ := NewClassifiedError(ctx, assert.AnError, severity, "database_failure", "DB failed", nil)
		error2, _ := NewClassifiedError(ctx, assert.AnError, severity, "git_clone_failure", "Git failed", nil)

		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert1, _ := NewAlert(error1, alertType, "Database failure alert")
		alert2, _ := NewAlert(error2, alertType, "Git failure alert")

		assert.NotEqual(t, alert1.DeduplicationKey(), alert2.DeduplicationKey())
	})

	t.Run("should support time-based deduplication windows", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(ctx, assert.AnError, severity, "system_failure", "System failed", nil)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "System failure alert")

		// Set deduplication window
		alert.SetDeduplicationWindow(time.Minute * 5)
		assert.Equal(t, time.Minute*5, alert.DeduplicationWindow())

		// Check if alert is within deduplication window
		assert.True(t, alert.IsWithinDeduplicationWindow())

		// Simulate time passage (would need time manipulation in real implementation)
		// For now, test the concept
		assert.True(t, alert.ShouldSuppressDuplicate(alert))
	})
}

func TestAlert_Escalation(t *testing.T) {
	t.Run("should escalate undelivered critical alerts", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"system_failure",
			"Critical system failure",
			nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Critical system failure - immediate attention required")

		// Set escalation rules
		escalationRules := []EscalationRule{
			{
				Condition:  "delivery_failed_after_minutes:5",
				Action:     "escalate_to_manager",
				Recipients: []string{"manager@company.com"},
			},
			{
				Condition:  "delivery_failed_after_minutes:15",
				Action:     "escalate_to_executive",
				Recipients: []string{"cto@company.com"},
			},
		}

		err := alert.SetEscalationRules(escalationRules)
		assert.NoError(t, err)
		assert.Len(t, alert.EscalationRules(), 2)

		// Check escalation eligibility
		assert.False(t, alert.IsEligibleForEscalation()) // Too soon

		// Simulate failed delivery
		failedResult := DeliveryResult{
			RecipientType:    "email",
			RecipientAddress: "oncall@company.com",
			Success:          false,
			AttemptedAt:      time.Now(),
			Error:            "Delivery failed",
		}
		alert.RecordDeliveryAttempt(failedResult)

		// Test escalation trigger (would need time manipulation)
		assert.NotNil(t, alert.GetNextEscalationRule())
	})

	t.Run("should stop escalation after successful delivery", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"system_failure",
			"Critical failure",
			nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Critical failure alert")

		escalationRules := []EscalationRule{
			{Condition: "delivery_failed_after_minutes:5", Action: "escalate"},
		}
		alert.SetEscalationRules(escalationRules)

		// Successful delivery
		successResult := DeliveryResult{
			RecipientType:    "email",
			RecipientAddress: "oncall@company.com",
			Success:          true,
			AttemptedAt:      time.Now(),
			DeliveredAt:      time.Now(),
		}
		alert.RecordDeliveryAttempt(successResult)

		// Should not escalate after successful delivery
		assert.False(t, alert.IsEligibleForEscalation())
		assert.Nil(t, alert.GetNextEscalationRule())
	})
}

func TestAlert_Serialization(t *testing.T) {
	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(ctx, assert.AnError, severity, "system_failure", "System failure", nil)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")

		alert, _ := NewAlert(classifiedError, alertType, "Critical system failure detected")

		recipients := []AlertRecipient{
			{Type: "email", Address: "oncall@company.com", Name: "On-call Team"},
		}
		alert.AddRecipients(recipients)

		jsonData, err := alert.MarshalJSON()
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "system_failure")
		assert.Contains(t, string(jsonData), "CRITICAL")
		assert.Contains(t, string(jsonData), "REAL_TIME")
		assert.Contains(t, string(jsonData), "oncall@company.com")
	})

	t.Run("should deserialize from JSON correctly", func(t *testing.T) {
		jsonData := `{
			"id": "alert-123",
			"error_id": "error-456",
			"type": "REAL_TIME",
			"severity": "ERROR",
			"message": "Test alert message",
			"status": "SENT",
			"created_at": "2023-01-01T00:00:00Z",
			"sent_at": "2023-01-01T00:01:00Z",
			"recipients": [
				{"type": "email", "address": "test@example.com", "name": "Test User"}
			],
			"deduplication_key": "test_error:ERROR:realtime"
		}`

		var alert Alert
		err := alert.UnmarshalJSON([]byte(jsonData))

		assert.NoError(t, err)
		assert.Equal(t, "alert-123", alert.ID())
		assert.Equal(t, "error-456", alert.ErrorID())
		assert.Equal(t, AlertStatusSent, alert.Status())
		assert.Len(t, alert.Recipients(), 1)
		assert.Equal(t, "test_error:ERROR:realtime", alert.DeduplicationKey())
	})
}

// Helper function to generate long strings for testing.
func generateLongString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = 'a'
	}
	return string(result)
}
