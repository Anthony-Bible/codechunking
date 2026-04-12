package valueobject

import "fmt"

// ConnectorType represents the type of git connector.
type ConnectorType string

// Connector type constants.
const (
	ConnectorTypeGitLab ConnectorType = "gitlab"
)

// NewConnectorType creates a validated ConnectorType from a string.
func NewConnectorType(s string) (ConnectorType, error) {
	ct := ConnectorType(s)
	if ct == ConnectorTypeGitLab {
		return ct, nil
	}
	return "", fmt.Errorf("invalid connector type: %s", s)
}

// String returns the string representation of the connector type.
func (ct ConnectorType) String() string {
	return string(ct)
}

// IsValid reports whether the connector type is a recognized value.
func (ct ConnectorType) IsValid() bool {
	return ct == ConnectorTypeGitLab
}

// AllConnectorTypes returns all valid connector types in a stable order.
func AllConnectorTypes() []ConnectorType {
	return []ConnectorType{
		ConnectorTypeGitLab,
	}
}
