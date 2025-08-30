package valueobject

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertType_ValidTypes(t *testing.T) {
	t.Run("should create real-time alert type", func(t *testing.T) {
		alertType, err := NewAlertType("REAL_TIME")

		require.NoError(t, err)
		assert.NotNil(t, alertType)
		assert.Equal(t, "REAL_TIME", alertType.String())
		assert.True(t, alertType.IsRealTime())
		assert.False(t, alertType.IsBatch())
		assert.False(t, alertType.IsCascade())
	})

	t.Run("should create batch alert type", func(t *testing.T) {
		alertType, err := NewAlertType("BATCH")

		require.NoError(t, err)
		assert.NotNil(t, alertType)
		assert.Equal(t, "BATCH", alertType.String())
		assert.False(t, alertType.IsRealTime())
		assert.True(t, alertType.IsBatch())
		assert.False(t, alertType.IsCascade())
	})

	t.Run("should create cascade alert type", func(t *testing.T) {
		alertType, err := NewAlertType("CASCADE")

		require.NoError(t, err)
		assert.NotNil(t, alertType)
		assert.Equal(t, "CASCADE", alertType.String())
		assert.False(t, alertType.IsRealTime())
		assert.False(t, alertType.IsBatch())
		assert.True(t, alertType.IsCascade())
	})
}

func TestAlertType_InvalidTypes(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", "invalid alert type: cannot be empty"},
		{"invalid type", "INVALID", "invalid alert type: INVALID is not a valid type"},
		{"lowercase", "real_time", "invalid alert type: real_time is not a valid type"},
		{"mixed case", "Real_Time", "invalid alert type: Real_Time is not a valid type"},
		{"numeric", "1", "invalid alert type: 1 is not a valid type"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alertType, err := NewAlertType(tc.input)

			require.Error(t, err)
			assert.Nil(t, alertType)
			assert.Contains(t, err.Error(), tc.expected)
		})
	}
}

func TestAlertType_Priority(t *testing.T) {
	t.Run("should return correct priority values", func(t *testing.T) {
		realTime, _ := NewAlertType("REAL_TIME")
		cascade, _ := NewAlertType("CASCADE")
		batch, _ := NewAlertType("BATCH")

		assert.Equal(t, 0, realTime.Priority())
		assert.Equal(t, 1, cascade.Priority())
		assert.Equal(t, 2, batch.Priority())
	})

	t.Run("should compare priorities correctly", func(t *testing.T) {
		realTime, _ := NewAlertType("REAL_TIME")
		cascade, _ := NewAlertType("CASCADE")
		batch, _ := NewAlertType("BATCH")

		assert.True(t, realTime.IsHigherPriority(cascade))
		assert.True(t, realTime.IsHigherPriority(batch))
		assert.True(t, cascade.IsHigherPriority(batch))

		assert.False(t, batch.IsHigherPriority(cascade))
		assert.False(t, batch.IsHigherPriority(realTime))
		assert.False(t, cascade.IsHigherPriority(realTime))
	})
}

func TestAlertType_DeliveryCharacteristics(t *testing.T) {
	t.Run("real-time alerts should require immediate delivery", func(t *testing.T) {
		realTime, _ := NewAlertType("REAL_TIME")
		assert.True(t, realTime.RequiresImmediateDelivery())
		assert.InEpsilon(t, 0.0, realTime.MaxDeliveryDelay().Seconds(), 0.001)
		assert.Equal(t, 3, realTime.MaxRetryAttempts())
	})

	t.Run("cascade alerts should require immediate delivery", func(t *testing.T) {
		cascade, _ := NewAlertType("CASCADE")
		assert.True(t, cascade.RequiresImmediateDelivery())
		assert.InEpsilon(t, 0.0, cascade.MaxDeliveryDelay().Seconds(), 0.001)
		assert.Equal(t, 5, cascade.MaxRetryAttempts()) // More retries for cascade
	})

	t.Run("batch alerts should allow delayed delivery", func(t *testing.T) {
		batch, _ := NewAlertType("BATCH")
		assert.False(t, batch.RequiresImmediateDelivery())
		assert.Equal(t, 300, int(batch.MaxDeliveryDelay().Seconds())) // 5 minutes
		assert.Equal(t, 2, batch.MaxRetryAttempts())
	})
}

func TestAlertType_Serialization(t *testing.T) {
	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		alertType, _ := NewAlertType("REAL_TIME")

		jsonData, err := alertType.MarshalJSON()
		require.NoError(t, err)
		assert.JSONEq(t, `"REAL_TIME"`, string(jsonData))
	})

	t.Run("should deserialize from JSON correctly", func(t *testing.T) {
		alertType := &AlertType{}
		err := alertType.UnmarshalJSON([]byte(`"BATCH"`))

		require.NoError(t, err)
		assert.Equal(t, "BATCH", alertType.String())
		assert.True(t, alertType.IsBatch())
	})

	t.Run("should fail to deserialize invalid JSON", func(t *testing.T) {
		alertType := &AlertType{}
		err := alertType.UnmarshalJSON([]byte(`"INVALID"`))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid alert type")
	})
}

func TestAlertType_Equality(t *testing.T) {
	t.Run("should be equal when same type", func(t *testing.T) {
		alertType1, _ := NewAlertType("REAL_TIME")
		alertType2, _ := NewAlertType("REAL_TIME")

		assert.True(t, alertType1.Equals(alertType2))
		assert.True(t, alertType2.Equals(alertType1))
	})

	t.Run("should not be equal when different type", func(t *testing.T) {
		realTime, _ := NewAlertType("REAL_TIME")
		batch, _ := NewAlertType("BATCH")

		assert.False(t, realTime.Equals(batch))
		assert.False(t, batch.Equals(realTime))
	})
}

func TestAlertType_ChannelRequirements(t *testing.T) {
	t.Run("should define appropriate channels for real-time alerts", func(t *testing.T) {
		realTime, _ := NewAlertType("REAL_TIME")

		channels := realTime.PreferredChannels()
		assert.Contains(t, channels, "pagerduty")
		assert.Contains(t, channels, "slack")
		assert.Contains(t, channels, "sms")
		assert.NotContains(t, channels, "email") // Too slow for real-time
	})

	t.Run("should define appropriate channels for batch alerts", func(t *testing.T) {
		batch, _ := NewAlertType("BATCH")

		channels := batch.PreferredChannels()
		assert.Contains(t, channels, "email")
		assert.Contains(t, channels, "slack")
		assert.NotContains(t, channels, "pagerduty") // Too noisy for batch
		assert.NotContains(t, channels, "sms")       // Too intrusive for batch
	})

	t.Run("should define appropriate channels for cascade alerts", func(t *testing.T) {
		cascade, _ := NewAlertType("CASCADE")

		channels := cascade.PreferredChannels()
		assert.Contains(t, channels, "pagerduty")
		assert.Contains(t, channels, "slack")
		assert.Contains(t, channels, "sms")
		assert.Contains(t, channels, "email") // All channels for critical cascade
	})
}

func TestAlertType_EscalationRules(t *testing.T) {
	t.Run("real-time alerts should have aggressive escalation", func(t *testing.T) {
		realTime, _ := NewAlertType("REAL_TIME")

		assert.True(t, realTime.SupportsEscalation())
		assert.Equal(t, 300, int(realTime.EscalationDelay().Seconds())) // 5 minutes
		assert.Equal(t, []string{"manager", "director", "executive"}, realTime.EscalationLevels())
	})

	t.Run("batch alerts should have minimal escalation", func(t *testing.T) {
		batch, _ := NewAlertType("BATCH")

		assert.False(t, batch.SupportsEscalation())
		assert.Equal(t, 0, int(batch.EscalationDelay().Seconds()))
		assert.Empty(t, batch.EscalationLevels())
	})

	t.Run("cascade alerts should have immediate escalation", func(t *testing.T) {
		cascade, _ := NewAlertType("CASCADE")

		assert.True(t, cascade.SupportsEscalation())
		assert.Equal(t, 60, int(cascade.EscalationDelay().Seconds())) // 1 minute
		assert.Equal(t, []string{"manager", "director", "executive", "cto"}, cascade.EscalationLevels())
	})
}

func TestAlertType_FormattingRules(t *testing.T) {
	t.Run("should define formatting for real-time alerts", func(t *testing.T) {
		realTime, _ := NewAlertType("REAL_TIME")

		format := realTime.MessageFormat()
		assert.Equal(t, "🚨 URGENT: {message}", format.Template)
		assert.True(t, format.IncludeTimestamp)
		assert.True(t, format.IncludeCorrelationID)
		assert.True(t, format.IncludeStackTrace)
		assert.Equal(t, "RED", format.Color)
	})

	t.Run("should define formatting for batch alerts", func(t *testing.T) {
		batch, _ := NewAlertType("BATCH")

		format := batch.MessageFormat()
		assert.Equal(t, "📊 BATCH: {message}", format.Template)
		assert.True(t, format.IncludeTimestamp)
		assert.False(t, format.IncludeCorrelationID) // Less verbose for batch
		assert.False(t, format.IncludeStackTrace)
		assert.Equal(t, "YELLOW", format.Color)
	})

	t.Run("should define formatting for cascade alerts", func(t *testing.T) {
		cascade, _ := NewAlertType("CASCADE")

		format := cascade.MessageFormat()
		assert.Equal(t, "💥 CASCADE: {message}", format.Template)
		assert.True(t, format.IncludeTimestamp)
		assert.True(t, format.IncludeCorrelationID)
		assert.True(t, format.IncludeStackTrace)
		assert.Equal(t, "PURPLE", format.Color)
	})
}
