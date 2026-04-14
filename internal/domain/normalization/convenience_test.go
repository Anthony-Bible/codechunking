package normalization

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRepositoryURLWithExtraHosts_MixedCaseHost(t *testing.T) {
	// Mixed-case extra hosts must match when CaseSensitiveHosts is false (the default).
	tests := []struct {
		name            string
		rawURL          string
		additionalHosts []string
		wantErr         bool
	}{
		{
			name:            "uppercase host in additionalHosts matches lowercase URL host",
			rawURL:          "https://MYGITLAB.EXAMPLE.COM/group/repo.git",
			additionalHosts: []string{"mygitlab.example.com"},
			wantErr:         false,
		},
		{
			name:            "mixed-case additionalHost matches lowercase URL host",
			rawURL:          "https://mygitlab.example.com/group/repo.git",
			additionalHosts: []string{"MyGitLab.Example.Com"},
			wantErr:         false,
		},
		{
			name:            "host not in additionalHosts is rejected",
			rawURL:          "https://unknown.example.com/group/repo.git",
			additionalHosts: []string{"other.example.com"},
			wantErr:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NormalizeRepositoryURLWithExtraHosts(tc.rawURL, tc.additionalHosts)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}
