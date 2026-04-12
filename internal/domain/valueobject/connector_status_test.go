package valueobject

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConnectorStatus_ValidStatuses(t *testing.T) {
	validStatuses := []struct {
		input    string
		expected ConnectorStatus
	}{
		{"pending", ConnectorStatusPending},
		{"active", ConnectorStatusActive},
		{"inactive", ConnectorStatusInactive},
		{"syncing", ConnectorStatusSyncing},
		{"error", ConnectorStatusError},
	}

	for _, tc := range validStatuses {
		t.Run(tc.input, func(t *testing.T) {
			cs, err := NewConnectorStatus(tc.input)
			require.NoError(t, err, "expected no error for valid connector status %s", tc.input)
			assert.Equal(t, tc.expected, cs)
		})
	}
}

func TestNewConnectorStatus_InvalidStatuses(t *testing.T) {
	invalidStatuses := []string{
		"invalid",
		"ACTIVE",
		"Active",
		"",
		" pending",
		"pending ",
		"running",
		"completed",
		"unknown",
	}

	for _, input := range invalidStatuses {
		t.Run(input, func(t *testing.T) {
			_, err := NewConnectorStatus(input)
			require.Error(t, err, "expected error for invalid connector status %q", input)
		})
	}
}

func TestConnectorStatus_String(t *testing.T) {
	tests := []struct {
		cs       ConnectorStatus
		expected string
	}{
		{ConnectorStatusPending, "pending"},
		{ConnectorStatusActive, "active"},
		{ConnectorStatusInactive, "inactive"},
		{ConnectorStatusSyncing, "syncing"},
		{ConnectorStatusError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cs.String())
		})
	}
}

func TestConnectorStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name   string
		from   ConnectorStatus
		to     ConnectorStatus
		expect bool
	}{
		// pending can go to active or error
		{"pending_to_active", ConnectorStatusPending, ConnectorStatusActive, true},
		{"pending_to_error", ConnectorStatusPending, ConnectorStatusError, true},
		{"pending_to_inactive", ConnectorStatusPending, ConnectorStatusInactive, false},
		{"pending_to_syncing", ConnectorStatusPending, ConnectorStatusSyncing, false},

		// active can sync, go inactive, or error
		{"active_to_syncing", ConnectorStatusActive, ConnectorStatusSyncing, true},
		{"active_to_inactive", ConnectorStatusActive, ConnectorStatusInactive, true},
		{"active_to_error", ConnectorStatusActive, ConnectorStatusError, true},
		{"active_to_pending", ConnectorStatusActive, ConnectorStatusPending, false},

		// syncing can return to active or go to error
		{"syncing_to_active", ConnectorStatusSyncing, ConnectorStatusActive, true},
		{"syncing_to_error", ConnectorStatusSyncing, ConnectorStatusError, true},
		{"syncing_to_inactive", ConnectorStatusSyncing, ConnectorStatusInactive, false},
		{"syncing_to_pending", ConnectorStatusSyncing, ConnectorStatusPending, false},

		// inactive can be reactivated or deleted (to pending)
		{"inactive_to_active", ConnectorStatusInactive, ConnectorStatusActive, true},
		{"inactive_to_pending", ConnectorStatusInactive, ConnectorStatusPending, false},

		// error can be retried (back to pending) or deactivated
		{"error_to_pending", ConnectorStatusError, ConnectorStatusPending, true},
		{"error_to_inactive", ConnectorStatusError, ConnectorStatusInactive, true},
		{"error_to_active", ConnectorStatusError, ConnectorStatusActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.expect, result,
				"transition from %s to %s: expected %v, got %v",
				tt.from, tt.to, tt.expect, result)
		})
	}
}

func TestConnectorStatus_IsTerminal(t *testing.T) {
	t.Run("inactive_is_terminal", func(t *testing.T) {
		assert.True(t, ConnectorStatusInactive.IsTerminal())
	})

	t.Run("non_terminal_statuses", func(t *testing.T) {
		nonTerminal := []ConnectorStatus{
			ConnectorStatusPending,
			ConnectorStatusActive,
			ConnectorStatusSyncing,
			ConnectorStatusError,
		}
		for _, cs := range nonTerminal {
			assert.False(t, cs.IsTerminal(), "expected %s to not be terminal", cs)
		}
	})
}

func TestAllConnectorStatuses(t *testing.T) {
	statuses := AllConnectorStatuses()
	assert.Len(t, statuses, 5, "expected exactly 5 connector statuses")

	statusSet := make(map[ConnectorStatus]bool)
	for _, cs := range statuses {
		statusSet[cs] = true
	}

	assert.True(t, statusSet[ConnectorStatusPending])
	assert.True(t, statusSet[ConnectorStatusActive])
	assert.True(t, statusSet[ConnectorStatusInactive])
	assert.True(t, statusSet[ConnectorStatusSyncing])
	assert.True(t, statusSet[ConnectorStatusError])
}
