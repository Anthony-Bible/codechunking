package valueobject

import "fmt"

// ConnectorType represents the type of git connector.
type ConnectorType string

// Connector type constants.
const (
	ConnectorTypeGitHubOrg   ConnectorType = "github_org"
	ConnectorTypeGitLabGroup ConnectorType = "gitlab_group"
	ConnectorTypeBitbucket   ConnectorType = "bitbucket"
	ConnectorTypeAzureDevOps ConnectorType = "azure_devops"
	ConnectorTypeGeneric     ConnectorType = "generic"
)

func validConnectorTypes() map[ConnectorType]bool {
	return map[ConnectorType]bool{
		ConnectorTypeGitHubOrg:   true,
		ConnectorTypeGitLabGroup: true,
		ConnectorTypeBitbucket:   true,
		ConnectorTypeAzureDevOps: true,
		ConnectorTypeGeneric:     true,
	}
}

// NewConnectorType creates a validated ConnectorType from a string.
func NewConnectorType(s string) (ConnectorType, error) {
	ct := ConnectorType(s)
	if !validConnectorTypes()[ct] {
		return "", fmt.Errorf("invalid connector type: %s", s)
	}
	return ct, nil
}

// String returns the string representation of the connector type.
func (ct ConnectorType) String() string {
	return string(ct)
}

// IsValid reports whether the connector type is a recognized value.
func (ct ConnectorType) IsValid() bool {
	return validConnectorTypes()[ct]
}

// AllConnectorTypes returns all valid connector types.
func AllConnectorTypes() []ConnectorType {
	types := make([]ConnectorType, 0, 5)
	for ct := range validConnectorTypes() {
		types = append(types, ct)
	}
	return types
}
