package valueobject

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorSeverity_ValidLevels(t *testing.T) {
	t.Run("should create critical error severity", func(t *testing.T) {
		severity, err := NewErrorSeverity("CRITICAL")

		assert.NoError(t, err)
		assert.NotNil(t, severity)
		assert.Equal(t, "CRITICAL", severity.String())
		assert.True(t, severity.IsCritical())
		assert.False(t, severity.IsError())
		assert.False(t, severity.IsWarning())
		assert.False(t, severity.IsInfo())
	})

	t.Run("should create error severity", func(t *testing.T) {
		severity, err := NewErrorSeverity("ERROR")

		assert.NoError(t, err)
		assert.NotNil(t, severity)
		assert.Equal(t, "ERROR", severity.String())
		assert.False(t, severity.IsCritical())
		assert.True(t, severity.IsError())
		assert.False(t, severity.IsWarning())
		assert.False(t, severity.IsInfo())
	})

	t.Run("should create warning severity", func(t *testing.T) {
		severity, err := NewErrorSeverity("WARNING")

		assert.NoError(t, err)
		assert.NotNil(t, severity)
		assert.Equal(t, "WARNING", severity.String())
		assert.False(t, severity.IsCritical())
		assert.False(t, severity.IsError())
		assert.True(t, severity.IsWarning())
		assert.False(t, severity.IsInfo())
	})

	t.Run("should create info severity", func(t *testing.T) {
		severity, err := NewErrorSeverity("INFO")

		assert.NoError(t, err)
		assert.NotNil(t, severity)
		assert.Equal(t, "INFO", severity.String())
		assert.False(t, severity.IsCritical())
		assert.False(t, severity.IsError())
		assert.False(t, severity.IsWarning())
		assert.True(t, severity.IsInfo())
	})
}

func TestErrorSeverity_InvalidLevels(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", "invalid error severity: cannot be empty"},
		{"invalid level", "INVALID", "invalid error severity: INVALID is not a valid level"},
		{"lowercase", "critical", "invalid error severity: critical is not a valid level"},
		{"mixed case", "Critical", "invalid error severity: Critical is not a valid level"},
		{"numeric", "1", "invalid error severity: 1 is not a valid level"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			severity, err := NewErrorSeverity(tc.input)

			assert.Error(t, err)
			assert.Nil(t, severity)
			assert.Contains(t, err.Error(), tc.expected)
		})
	}
}

func TestErrorSeverity_Priority(t *testing.T) {
	t.Run("should return correct priority values", func(t *testing.T) {
		critical, _ := NewErrorSeverity("CRITICAL")
		errorSev, _ := NewErrorSeverity("ERROR")
		warning, _ := NewErrorSeverity("WARNING")
		info, _ := NewErrorSeverity("INFO")

		assert.Equal(t, 0, critical.Priority())
		assert.Equal(t, 1, errorSev.Priority())
		assert.Equal(t, 2, warning.Priority())
		assert.Equal(t, 3, info.Priority())
	})

	t.Run("should compare priorities correctly", func(t *testing.T) {
		critical, _ := NewErrorSeverity("CRITICAL")
		errorSev, _ := NewErrorSeverity("ERROR")
		warning, _ := NewErrorSeverity("WARNING")
		info, _ := NewErrorSeverity("INFO")

		assert.True(t, critical.IsHigherPriority(errorSev))
		assert.True(t, critical.IsHigherPriority(warning))
		assert.True(t, critical.IsHigherPriority(info))
		assert.True(t, errorSev.IsHigherPriority(warning))
		assert.True(t, errorSev.IsHigherPriority(info))
		assert.True(t, warning.IsHigherPriority(info))

		assert.False(t, info.IsHigherPriority(warning))
		assert.False(t, warning.IsHigherPriority(errorSev))
		assert.False(t, errorSev.IsHigherPriority(critical))
	})
}

func TestErrorSeverity_RequiresImmediateAlert(t *testing.T) {
	t.Run("critical errors should require immediate alerts", func(t *testing.T) {
		critical, _ := NewErrorSeverity("CRITICAL")
		assert.True(t, critical.RequiresImmediateAlert())
	})

	t.Run("error level should require immediate alerts", func(t *testing.T) {
		errorSev, _ := NewErrorSeverity("ERROR")
		assert.True(t, errorSev.RequiresImmediateAlert())
	})

	t.Run("warning level should not require immediate alerts", func(t *testing.T) {
		warning, _ := NewErrorSeverity("WARNING")
		assert.False(t, warning.RequiresImmediateAlert())
	})

	t.Run("info level should not require immediate alerts", func(t *testing.T) {
		info, _ := NewErrorSeverity("INFO")
		assert.False(t, info.RequiresImmediateAlert())
	})
}

func TestErrorSeverity_Serialization(t *testing.T) {
	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		severity, _ := NewErrorSeverity("CRITICAL")

		jsonData, err := severity.MarshalJSON()
		assert.NoError(t, err)
		assert.JSONEq(t, `"CRITICAL"`, string(jsonData))
	})

	t.Run("should deserialize from JSON correctly", func(t *testing.T) {
		severity := &ErrorSeverity{}
		err := severity.UnmarshalJSON([]byte(`"ERROR"`))

		assert.NoError(t, err)
		assert.Equal(t, "ERROR", severity.String())
		assert.True(t, severity.IsError())
	})

	t.Run("should fail to deserialize invalid JSON", func(t *testing.T) {
		severity := &ErrorSeverity{}
		err := severity.UnmarshalJSON([]byte(`"INVALID"`))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid error severity")
	})
}

func TestErrorSeverity_Equality(t *testing.T) {
	t.Run("should be equal when same severity", func(t *testing.T) {
		severity1, _ := NewErrorSeverity("CRITICAL")
		severity2, _ := NewErrorSeverity("CRITICAL")

		assert.True(t, severity1.Equals(severity2))
		assert.True(t, severity2.Equals(severity1))
	})

	t.Run("should not be equal when different severity", func(t *testing.T) {
		critical, _ := NewErrorSeverity("CRITICAL")
		errorSev, _ := NewErrorSeverity("ERROR")

		assert.False(t, critical.Equals(errorSev))
		assert.False(t, errorSev.Equals(critical))
	})
}
