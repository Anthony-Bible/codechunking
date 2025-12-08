package version

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestNewVersionInfo tests the creation of VersionInfo with various states.
func TestNewVersionInfo(t *testing.T) {
	tests := []struct {
		name           string
		setupVersion   string
		setupCommit    string
		setupBuildTime string
		wantVersion    string
		wantCommit     string
		wantBuildTime  string
	}{
		{
			name:           "empty values use defaults",
			setupVersion:   "",
			setupCommit:    "",
			setupBuildTime: "",
			wantVersion:    DefaultVersion,
			wantCommit:     DefaultCommit,
			wantBuildTime:  DefaultBuildTime,
		},
		{
			name:           "all values set",
			setupVersion:   "v1.0.0",
			setupCommit:    "abc123",
			setupBuildTime: "2025-01-01T00:00:00Z",
			wantVersion:    "v1.0.0",
			wantCommit:     "abc123",
			wantBuildTime:  "2025-01-01T00:00:00Z",
		},
		{
			name:           "partial values - only version",
			setupVersion:   "v2.0.0",
			setupCommit:    "",
			setupBuildTime: "",
			wantVersion:    "v2.0.0",
			wantCommit:     DefaultCommit,
			wantBuildTime:  DefaultBuildTime,
		},
		{
			name:           "partial values - only commit",
			setupVersion:   "",
			setupCommit:    "def456",
			setupBuildTime: "",
			wantVersion:    DefaultVersion,
			wantCommit:     "def456",
			wantBuildTime:  DefaultBuildTime,
		},
		{
			name:           "partial values - only build time",
			setupVersion:   "",
			setupCommit:    "",
			setupBuildTime: "2025-06-15T12:30:00Z",
			wantVersion:    DefaultVersion,
			wantCommit:     DefaultCommit,
			wantBuildTime:  "2025-06-15T12:30:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup - set build variables
			ResetBuildVars()
			SetBuildVars(tt.setupVersion, tt.setupCommit, tt.setupBuildTime)

			// Execute
			info := NewVersionInfo()

			// Verify
			if info.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", info.Version, tt.wantVersion)
			}
			if info.Commit != tt.wantCommit {
				t.Errorf("Commit = %q, want %q", info.Commit, tt.wantCommit)
			}
			if info.BuildTime != tt.wantBuildTime {
				t.Errorf("BuildTime = %q, want %q", info.BuildTime, tt.wantBuildTime)
			}

			// Cleanup
			ResetBuildVars()
		})
	}
}

// TestFormatShort tests the short format output.
func TestFormatShort(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "normal version",
			version: "v1.2.3",
			want:    "v1.2.3",
		},
		{
			name:    "default version",
			version: DefaultVersion,
			want:    DefaultVersion,
		},
		{
			name:    "prerelease version",
			version: "v1.0.0-beta.1",
			want:    "v1.0.0-beta.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &VersionInfo{
				Version:   tt.version,
				Commit:    "abc123",
				BuildTime: "2025-01-01T00:00:00Z",
			}

			got := info.FormatShort()
			if got != tt.want {
				t.Errorf("FormatShort() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormatFull tests the full format output.
func TestFormatFull(t *testing.T) {
	info := &VersionInfo{
		Version:   "v1.0.0",
		Commit:    "abc123def456",
		BuildTime: "2025-01-15T10:30:00Z",
	}

	got := info.FormatFull()

	// Check that all expected components are present
	expectedLines := []string{
		ApplicationName,
		LabelVersion + fieldSeparator + "v1.0.0",
		LabelCommit + fieldSeparator + "abc123def456",
		LabelBuilt + fieldSeparator + "2025-01-15T10:30:00Z",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(got, expected) {
			t.Errorf("FormatFull() missing expected content %q\nGot:\n%s", expected, got)
		}
	}

	// Verify it ends with a newline
	if !strings.HasSuffix(got, "\n") {
		t.Error("FormatFull() should end with a newline")
	}
}

// TestWriteShort tests writing short format to a writer.
func TestWriteShort(t *testing.T) {
	info := &VersionInfo{
		Version:   "v1.0.0",
		Commit:    "abc123",
		BuildTime: "2025-01-01T00:00:00Z",
	}

	var buf bytes.Buffer
	err := info.WriteShort(&buf)
	if err != nil {
		t.Errorf("WriteShort() error = %v", err)
	}

	got := buf.String()
	want := "v1.0.0\n"
	if got != want {
		t.Errorf("WriteShort() wrote %q, want %q", got, want)
	}
}

// TestWriteFull tests writing full format to a writer.
func TestWriteFull(t *testing.T) {
	info := &VersionInfo{
		Version:   "v1.0.0",
		Commit:    "abc123",
		BuildTime: "2025-01-01T00:00:00Z",
	}

	var buf bytes.Buffer
	err := info.WriteFull(&buf)
	if err != nil {
		t.Errorf("WriteFull() error = %v", err)
	}

	got := buf.String()

	// Verify expected content
	if !strings.Contains(got, ApplicationName) {
		t.Errorf("WriteFull() missing application name")
	}
	if !strings.Contains(got, "v1.0.0") {
		t.Errorf("WriteFull() missing version")
	}
	if !strings.Contains(got, "abc123") {
		t.Errorf("WriteFull() missing commit")
	}
}

// TestWrite tests the Write method with both short and full modes.
func TestWrite(t *testing.T) {
	info := &VersionInfo{
		Version:   "v2.0.0",
		Commit:    "xyz789",
		BuildTime: "2025-06-01T00:00:00Z",
	}

	t.Run("short mode", func(t *testing.T) {
		var buf bytes.Buffer
		err := info.Write(&buf, true)
		if err != nil {
			t.Errorf("Write(short=true) error = %v", err)
		}

		got := buf.String()
		if got != "v2.0.0\n" {
			t.Errorf("Write(short=true) = %q, want %q", got, "v2.0.0\n")
		}
	})

	t.Run("full mode", func(t *testing.T) {
		var buf bytes.Buffer
		err := info.Write(&buf, false)
		if err != nil {
			t.Errorf("Write(short=false) error = %v", err)
		}

		got := buf.String()
		if !strings.Contains(got, ApplicationName) {
			t.Errorf("Write(short=false) missing application name")
		}
	})
}

// TestSetBuildVars tests setting build variables.
func TestSetBuildVars(t *testing.T) {
	// Cleanup first
	ResetBuildVars()

	// Set values
	SetBuildVars("v3.0.0", "commit123", "2025-12-01T00:00:00Z")

	// Verify through NewVersionInfo
	info := NewVersionInfo()

	if info.Version != "v3.0.0" {
		t.Errorf("After SetBuildVars, Version = %q, want %q", info.Version, "v3.0.0")
	}
	if info.Commit != "commit123" {
		t.Errorf("After SetBuildVars, Commit = %q, want %q", info.Commit, "commit123")
	}
	if info.BuildTime != "2025-12-01T00:00:00Z" {
		t.Errorf("After SetBuildVars, BuildTime = %q, want %q", info.BuildTime, "2025-12-01T00:00:00Z")
	}

	// Cleanup
	ResetBuildVars()
}

// TestResetBuildVars tests resetting build variables.
func TestResetBuildVars(t *testing.T) {
	// Set some values first
	SetBuildVars("v1.0.0", "abc", "2025-01-01T00:00:00Z")

	// Reset
	ResetBuildVars()

	// Verify defaults are used
	info := NewVersionInfo()

	if info.Version != DefaultVersion {
		t.Errorf("After ResetBuildVars, Version = %q, want %q", info.Version, DefaultVersion)
	}
	if info.Commit != DefaultCommit {
		t.Errorf("After ResetBuildVars, Commit = %q, want %q", info.Commit, DefaultCommit)
	}
	if info.BuildTime != DefaultBuildTime {
		t.Errorf("After ResetBuildVars, BuildTime = %q, want %q", info.BuildTime, DefaultBuildTime)
	}
}

// TestIsDevelopment tests the IsDevelopment method.
func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "default version is development",
			version: DefaultVersion,
			want:    true,
		},
		{
			name:    "release version is not development",
			version: "v1.0.0",
			want:    false,
		},
		{
			name:    "prerelease version is not development",
			version: "v1.0.0-beta",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &VersionInfo{
				Version:   tt.version,
				Commit:    "abc123",
				BuildTime: "2025-01-01T00:00:00Z",
			}

			got := info.IsDevelopment()
			if got != tt.want {
				t.Errorf("IsDevelopment() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetBuildTime tests parsing build time into a time.Time.
func TestGetBuildTime(t *testing.T) {
	tests := []struct {
		name      string
		buildTime string
		wantZero  bool
		wantYear  int
		wantMonth time.Month
		wantDay   int
	}{
		{
			name:      "default build time returns zero",
			buildTime: DefaultBuildTime,
			wantZero:  true,
		},
		{
			name:      "RFC3339 format",
			buildTime: "2025-01-15T10:30:00Z",
			wantZero:  false,
			wantYear:  2025,
			wantMonth: time.January,
			wantDay:   15,
		},
		{
			name:      "RFC3339 with timezone offset",
			buildTime: "2025-06-20T14:00:00+02:00",
			wantZero:  false,
			wantYear:  2025,
			wantMonth: time.June,
			wantDay:   20,
		},
		{
			name:      "date only format",
			buildTime: "2025-03-01",
			wantZero:  false,
			wantYear:  2025,
			wantMonth: time.March,
			wantDay:   1,
		},
		{
			name:      "invalid format returns zero",
			buildTime: "not-a-date",
			wantZero:  true,
		},
		{
			name:      "datetime without timezone",
			buildTime: "2025-07-04 12:00:00",
			wantZero:  false,
			wantYear:  2025,
			wantMonth: time.July,
			wantDay:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &VersionInfo{
				Version:   "v1.0.0",
				Commit:    "abc123",
				BuildTime: tt.buildTime,
			}

			got := info.GetBuildTime()

			if tt.wantZero {
				if !got.IsZero() {
					t.Errorf("GetBuildTime() = %v, want zero time", got)
				}
				return
			}

			// Non-zero time expected
			if got.IsZero() {
				t.Fatalf("GetBuildTime() returned zero time, want non-zero")
			}
			if got.Year() != tt.wantYear {
				t.Errorf("GetBuildTime().Year() = %d, want %d", got.Year(), tt.wantYear)
			}
			if got.Month() != tt.wantMonth {
				t.Errorf("GetBuildTime().Month() = %v, want %v", got.Month(), tt.wantMonth)
			}
			if got.Day() != tt.wantDay {
				t.Errorf("GetBuildTime().Day() = %d, want %d", got.Day(), tt.wantDay)
			}
		})
	}
}

// TestGetVersion tests the GetVersion function.
func TestGetVersion(t *testing.T) {
	ResetBuildVars()
	SetBuildVars("v4.0.0", "getversion123", "2025-11-11T11:11:11Z")

	info := GetVersion()

	if info == nil {
		t.Fatal("GetVersion() returned nil")
	}
	if info.Version != "v4.0.0" {
		t.Errorf("GetVersion().Version = %q, want %q", info.Version, "v4.0.0")
	}
	if info.Commit != "getversion123" {
		t.Errorf("GetVersion().Commit = %q, want %q", info.Commit, "getversion123")
	}
	if info.BuildTime != "2025-11-11T11:11:11Z" {
		t.Errorf("GetVersion().BuildTime = %q, want %q", info.BuildTime, "2025-11-11T11:11:11Z")
	}

	ResetBuildVars()
}

// errorWriter is a writer that always returns an error.
type errorWriter struct{}

func (e *errorWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write error")
}

// TestWriteErrors tests error handling in write methods.
func TestWriteErrors(t *testing.T) {
	info := &VersionInfo{
		Version:   "v1.0.0",
		Commit:    "abc123",
		BuildTime: "2025-01-01T00:00:00Z",
	}

	errWriter := &errorWriter{}

	t.Run("WriteShort error", func(t *testing.T) {
		err := info.WriteShort(errWriter)
		if err == nil {
			t.Error("WriteShort() expected error, got nil")
		}
	})

	t.Run("WriteFull error", func(t *testing.T) {
		err := info.WriteFull(errWriter)
		if err == nil {
			t.Error("WriteFull() expected error, got nil")
		}
	})

	t.Run("Write short mode error", func(t *testing.T) {
		err := info.Write(errWriter, true)
		if err == nil {
			t.Error("Write(short=true) expected error, got nil")
		}
	})

	t.Run("Write full mode error", func(t *testing.T) {
		err := info.Write(errWriter, false)
		if err == nil {
			t.Error("Write(short=false) expected error, got nil")
		}
	})
}

// TestApplicationNameConstant verifies the application name constant.
func TestApplicationNameConstant(t *testing.T) {
	if ApplicationName != "CodeChunking CLI" {
		t.Errorf("ApplicationName = %q, want %q", ApplicationName, "CodeChunking CLI")
	}
}

// TestDefaultConstants verifies the default constants.
func TestDefaultConstants(t *testing.T) {
	if DefaultVersion != "dev" {
		t.Errorf("DefaultVersion = %q, want %q", DefaultVersion, "dev")
	}
	if DefaultCommit != "unknown" {
		t.Errorf("DefaultCommit = %q, want %q", DefaultCommit, "unknown")
	}
	if DefaultBuildTime != "unknown" {
		t.Errorf("DefaultBuildTime = %q, want %q", DefaultBuildTime, "unknown")
	}
}
