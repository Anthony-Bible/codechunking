package valueobject

import "fmt"

// ConnectorStatus represents the operational status of a connector.
type ConnectorStatus string

// Connector status constants.
const (
	ConnectorStatusPending  ConnectorStatus = "pending"
	ConnectorStatusActive   ConnectorStatus = "active"
	ConnectorStatusInactive ConnectorStatus = "inactive"
	ConnectorStatusSyncing  ConnectorStatus = "syncing"
	ConnectorStatusError    ConnectorStatus = "error"
)

func validConnectorStatuses() map[ConnectorStatus]bool {
	return map[ConnectorStatus]bool{
		ConnectorStatusPending:  true,
		ConnectorStatusActive:   true,
		ConnectorStatusInactive: true,
		ConnectorStatusSyncing:  true,
		ConnectorStatusError:    true,
	}
}

// NewConnectorStatus creates a validated ConnectorStatus from a string.
func NewConnectorStatus(s string) (ConnectorStatus, error) {
	cs := ConnectorStatus(s)
	if !validConnectorStatuses()[cs] {
		return "", fmt.Errorf("invalid connector status: %s", s)
	}
	return cs, nil
}

// String returns the string representation of the connector status.
func (cs ConnectorStatus) String() string {
	return string(cs)
}

// IsTerminal reports whether this status is a final, non-recoverable state.
func (cs ConnectorStatus) IsTerminal() bool {
	return cs == ConnectorStatusInactive
}

// CanTransitionTo reports whether the connector can move from this status to target.
func (cs ConnectorStatus) CanTransitionTo(target ConnectorStatus) bool {
	transitions := map[ConnectorStatus][]ConnectorStatus{
		ConnectorStatusPending: {
			ConnectorStatusActive,
			ConnectorStatusError,
		},
		ConnectorStatusActive: {
			ConnectorStatusSyncing,
			ConnectorStatusInactive,
			ConnectorStatusError,
		},
		ConnectorStatusSyncing: {
			ConnectorStatusActive,
			ConnectorStatusError,
		},
		ConnectorStatusInactive: {
			ConnectorStatusActive,
		},
		ConnectorStatusError: {
			ConnectorStatusPending,
			ConnectorStatusInactive,
		},
	}

	for _, valid := range transitions[cs] {
		if valid == target {
			return true
		}
	}
	return false
}

// AllConnectorStatuses returns all valid connector statuses.
func AllConnectorStatuses() []ConnectorStatus {
	statuses := make([]ConnectorStatus, 0, 5)
	for cs := range validConnectorStatuses() {
		statuses = append(statuses, cs)
	}
	return statuses
}
