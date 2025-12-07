package client

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.APIURL != "http://localhost:8080" {
		t.Errorf("DefaultConfig().APIURL = %q, want %q", cfg.APIURL, "http://localhost:8080")
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("DefaultConfig().Timeout = %v, want %v", cfg.Timeout, 30*time.Second)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	if cfg.APIURL != "http://localhost:8080" {
		t.Errorf("APIURL = %q, want %q", cfg.APIURL, "http://localhost:8080")
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 30*time.Second)
	}
}

func TestLoadConfig_CustomAPIURL(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_API_URL", "https://api.example.com")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	if cfg.APIURL != "https://api.example.com" {
		t.Errorf("APIURL = %q, want %q", cfg.APIURL, "https://api.example.com")
	}
}

func TestLoadConfig_CustomTimeout(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "60s")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 60*time.Second)
	}
}

func TestLoadConfig_BothEnvVars(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_API_URL", "https://prod.example.com:8443")
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "2m")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	if cfg.APIURL != "https://prod.example.com:8443" {
		t.Errorf("APIURL = %q, want %q", cfg.APIURL, "https://prod.example.com:8443")
	}
	if cfg.Timeout != 2*time.Minute {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 2*time.Minute)
	}
}

func TestLoadConfig_TimeoutMilliseconds(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "500ms")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	if cfg.Timeout != 500*time.Millisecond {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 500*time.Millisecond)
	}
}

func TestLoadConfig_InvalidTimeout(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "invalid")

	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for invalid timeout")
	}
}

func TestLoadConfig_EmptyTimeout(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "")

	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for empty timeout")
	}
}

func TestLoadConfig_NegativeTimeout(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "-10s")

	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for negative timeout")
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config with http URL",
			config:  Config{APIURL: "http://localhost:8080", Timeout: 30 * time.Second},
			wantErr: false,
		},
		{
			name:    "valid config with https URL",
			config:  Config{APIURL: "https://api.example.com", Timeout: 60 * time.Second},
			wantErr: false,
		},
		{
			name:    "valid config with https URL and port",
			config:  Config{APIURL: "https://api.example.com:8443", Timeout: 1 * time.Minute},
			wantErr: false,
		},
		{
			name:    "valid config with http URL and path",
			config:  Config{APIURL: "http://localhost:8080/api/v1", Timeout: 15 * time.Second},
			wantErr: false,
		},
		{
			name:    "empty API URL returns error",
			config:  Config{APIURL: "", Timeout: 30 * time.Second},
			wantErr: true,
			errMsg:  "API URL",
		},
		{
			name:    "invalid URL scheme returns error",
			config:  Config{APIURL: "ftp://example.com", Timeout: 30 * time.Second},
			wantErr: true,
			errMsg:  "scheme",
		},
		{
			name:    "URL without scheme returns error",
			config:  Config{APIURL: "localhost:8080", Timeout: 30 * time.Second},
			wantErr: true,
			errMsg:  "scheme",
		},
		{
			name:    "zero timeout returns error",
			config:  Config{APIURL: "http://localhost:8080", Timeout: 0},
			wantErr: true,
			errMsg:  "timeout",
		},
		{
			name:    "negative timeout returns error",
			config:  Config{APIURL: "http://localhost:8080", Timeout: -10 * time.Second},
			wantErr: true,
			errMsg:  "timeout",
		},
		{
			name:    "multiple validation errors",
			config:  Config{APIURL: "", Timeout: 0},
			wantErr: true,
		},
		{
			name:    "minimal valid timeout",
			config:  Config{APIURL: "http://localhost:8080", Timeout: 1 * time.Millisecond},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()
			assertValidationResult(t, err, tt.wantErr, tt.errMsg)
		})
	}
}

func assertValidationResult(t *testing.T, err error, wantErr bool, errMsg string) {
	t.Helper()
	if wantErr {
		if err == nil {
			t.Error("Validate() error = nil, want error")
			return
		}
		if errMsg != "" && !strings.Contains(err.Error(), errMsg) {
			t.Errorf("Validate() error = %q, want error containing %q", err.Error(), errMsg)
		}
	} else if err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestLoadConfigAndValidate_ValidDefaults(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestLoadConfigAndValidate_ValidCustomConfig(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_API_URL", "https://api.example.com")
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "45s")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestLoadConfigAndValidate_InvalidURL(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_API_URL", "not-a-valid-url")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Validate() error = nil, want error for invalid URL")
	}
}

func TestLoadConfigAndValidate_InvalidTimeout(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "invalid-duration")

	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for invalid duration")
	}
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	t.Run("DefaultConfig returns same values as LoadConfig with no env vars", func(t *testing.T) {
		t.Parallel()
		defaultCfg := DefaultConfig()

		loadedCfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() unexpected error = %v", err)
		}

		if defaultCfg.APIURL != loadedCfg.APIURL {
			t.Errorf("DefaultConfig().APIURL = %q, LoadConfig().APIURL = %q, want equal",
				defaultCfg.APIURL, loadedCfg.APIURL)
		}

		if defaultCfg.Timeout != loadedCfg.Timeout {
			t.Errorf("DefaultConfig().Timeout = %v, LoadConfig().Timeout = %v, want equal",
				defaultCfg.Timeout, loadedCfg.Timeout)
		}
	})

	t.Run("default config passes validation", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultConfig()

		err := cfg.Validate()
		if err != nil {
			t.Errorf("DefaultConfig().Validate() error = %v, want nil", err)
		}
	})
}
