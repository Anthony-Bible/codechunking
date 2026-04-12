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

// NewConnectorStatus creates a validated ConnectorStatus from a string.
func NewConnectorStatus(s string) (ConnectorStatus, error) {
	cs := ConnectorStatus(s)
	switch cs {
	case ConnectorStatusPending, ConnectorStatusActive, ConnectorStatusInactive,
		ConnectorStatusSyncing, ConnectorStatusError:
		return cs, nil
	}
	return "", fmt.Errorf("invalid connector status: %s", s)
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
	switch cs {
	case ConnectorStatusPending:
		return target == ConnectorStatusActive || target == ConnectorStatusError
	case ConnectorStatusActive:
		return target == ConnectorStatusSyncing || target == ConnectorStatusInactive || target == ConnectorStatusError
	case ConnectorStatusSyncing:
		return target == ConnectorStatusActive || target == ConnectorStatusError
	case ConnectorStatusInactive:
		return target == ConnectorStatusActive
	case ConnectorStatusError:
		return target == ConnectorStatusPending || target == ConnectorStatusInactive
	}
	return false
}

// AllConnectorStatuses returns all valid connector statuses in a stable order.
func AllConnectorStatuses() []ConnectorStatus {
	return []ConnectorStatus{
		ConnectorStatusPending,
		ConnectorStatusActive,
		ConnectorStatusInactive,
		ConnectorStatusSyncing,
		ConnectorStatusError,
	}
}
