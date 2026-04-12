package valueobject

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConnectorType_ValidTypes(t *testing.T) {
	validTypes := []struct {
		input    string
		expected ConnectorType
	}{
		{"github_org", ConnectorTypeGitHubOrg},
		{"gitlab_group", ConnectorTypeGitLabGroup},
		{"bitbucket", ConnectorTypeBitbucket},
		{"azure_devops", ConnectorTypeAzureDevOps},
		{"generic", ConnectorTypeGeneric},
	}

	for _, tc := range validTypes {
		t.Run(tc.input, func(t *testing.T) {
			ct, err := NewConnectorType(tc.input)
			require.NoError(t, err, "expected no error for valid connector type %s", tc.input)
			assert.Equal(t, tc.expected, ct)
		})
	}
}

func TestNewConnectorType_InvalidTypes(t *testing.T) {
	invalidTypes := []string{
		"invalid",
		"GitHub_Org",
		"GITHUB_ORG",
		"",
		" github_org",
		"github_org ",
		"github",
		"gitlab",
		"unknown",
	}

	for _, input := range invalidTypes {
		t.Run(input, func(t *testing.T) {
			_, err := NewConnectorType(input)
			require.Error(t, err, "expected error for invalid connector type %q", input)
		})
	}
}

func TestConnectorType_String(t *testing.T) {
	tests := []struct {
		ct       ConnectorType
		expected string
	}{
		{ConnectorTypeGitHubOrg, "github_org"},
		{ConnectorTypeGitLabGroup, "gitlab_group"},
		{ConnectorTypeBitbucket, "bitbucket"},
		{ConnectorTypeAzureDevOps, "azure_devops"},
		{ConnectorTypeGeneric, "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ct.String())
		})
	}
}

func TestConnectorType_IsValid(t *testing.T) {
	t.Run("valid_types_return_true", func(t *testing.T) {
		validTypes := []ConnectorType{
			ConnectorTypeGitHubOrg,
			ConnectorTypeGitLabGroup,
			ConnectorTypeBitbucket,
			ConnectorTypeAzureDevOps,
			ConnectorTypeGeneric,
		}
		for _, ct := range validTypes {
			assert.True(t, ct.IsValid(), "expected %s to be valid", ct)
		}
	})

	t.Run("invalid_type_returns_false", func(t *testing.T) {
		ct := ConnectorType("not_a_real_type")
		assert.False(t, ct.IsValid())
	})
}

func TestAllConnectorTypes(t *testing.T) {
	types := AllConnectorTypes()
	assert.Len(t, types, 5, "expected exactly 5 connector types")

	typeSet := make(map[ConnectorType]bool)
	for _, ct := range types {
		typeSet[ct] = true
	}

	assert.True(t, typeSet[ConnectorTypeGitHubOrg])
	assert.True(t, typeSet[ConnectorTypeGitLabGroup])
	assert.True(t, typeSet[ConnectorTypeBitbucket])
	assert.True(t, typeSet[ConnectorTypeAzureDevOps])
	assert.True(t, typeSet[ConnectorTypeGeneric])
}
