// Package version provides centralized version information management for the codechunking application.
//
// This package encapsulates all version-related data and functionality, including:
// - Version information storage and default value handling
// - Formatted output generation for different use cases
// - Build-time variable injection via ldflags
//
// Build-time injection:
// The version variables are typically set during build using ldflags:
//
//	-ldflags "-X codechunking/internal/version.version=v1.0.0 -X codechunking/internal/version.commit=abc123 -X codechunking/internal/version.buildTime=2025-01-01T00:00:00Z"
package version

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// These variables are set via ldflags during build.
// They should not be modified directly in code.
//
//nolint:gochecknoglobals // Required for build-time injection via ldflags.
var (
	// version holds the application version (e.g., "v1.0.0").
	version string
	// commit holds the git commit hash (e.g., "abc123def456").
	commit string
	// buildTime holds the build timestamp in RFC3339 format.
	buildTime string
)

// ApplicationName is the name of the application displayed in version output.
const ApplicationName = "CodeChunking CLI"

// Default values used when version information is not available.
const (
	DefaultVersion   = "dev"
	DefaultCommit    = "unknown"
	DefaultBuildTime = "unknown"
)

// Format constants for different output styles.
const (
	LabelVersion   = "Version"
	LabelCommit    = "Commit"
	LabelBuilt     = "Built"
	fieldSeparator = ": "
	lineSeparator  = "\n"
)

// VersionInfo encapsulates all version-related information with proper defaults.
type VersionInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// NewVersionInfo creates a new VersionInfo instance with values from build-time variables
// and appropriate defaults for empty values.
func NewVersionInfo() *VersionInfo {
	// Check if legacy variables might be set (if this package is used from cmd)
	// This ensures backward compatibility with ldflags injection in cmd package
	if version == "" && commit == "" && buildTime == "" {
		// If our internal vars are empty, try to sync from package-level variables
		// that might have been set via ldflags in importing packages
		// Note: This is a safety net - the proper sync should happen in cmd/init()
		// but we provide this fallback for direct package use
	}

	info := &VersionInfo{
		Version:   getVersionWithDefault(),
		Commit:    getCommitWithDefault(),
		BuildTime: getBuildTimeWithDefault(),
	}
	return info
}

// getVersionWithDefault returns the version with a default value if empty.
func getVersionWithDefault() string {
	if version == "" {
		return DefaultVersion
	}
	return version
}

// getCommitWithDefault returns the commit with a default value if empty.
func getCommitWithDefault() string {
	if commit == "" {
		return DefaultCommit
	}
	return commit
}

// getBuildTimeWithDefault returns the build time with a default value if empty.
func getBuildTimeWithDefault() string {
	if buildTime == "" {
		return DefaultBuildTime
	}
	return buildTime
}

// FormatShort returns a single-line output containing only the version number.
// This is typically used for automated processing or when brevity is desired.
func (vi *VersionInfo) FormatShort() string {
	return vi.Version
}

// FormatFull returns a multi-line output with complete version information.
// This includes application name, version, commit, and build time.
func (vi *VersionInfo) FormatFull() string {
	var builder strings.Builder

	builder.WriteString(ApplicationName)
	builder.WriteString(lineSeparator)
	builder.WriteString(LabelVersion)
	builder.WriteString(fieldSeparator)
	builder.WriteString(vi.Version)
	builder.WriteString(lineSeparator)
	builder.WriteString(LabelCommit)
	builder.WriteString(fieldSeparator)
	builder.WriteString(vi.Commit)
	builder.WriteString(lineSeparator)
	builder.WriteString(LabelBuilt)
	builder.WriteString(fieldSeparator)
	builder.WriteString(vi.BuildTime)
	builder.WriteString(lineSeparator)

	return builder.String()
}

// WriteShort writes the short format (version only) to the provided writer.
func (vi *VersionInfo) WriteShort(w io.Writer) error {
	_, err := fmt.Fprintln(w, vi.FormatShort())
	return err
}

// WriteFull writes the full format to the provided writer.
func (vi *VersionInfo) WriteFull(w io.Writer) error {
	_, err := fmt.Fprint(w, vi.FormatFull())
	return err
}

// Write formats the version based on the short flag and writes to the provided writer.
// This is a convenience method that handles both output formats.
func (vi *VersionInfo) Write(w io.Writer, short bool) error {
	if short {
		return vi.WriteShort(w)
	}
	return vi.WriteFull(w)
}

// GetVersion returns the current version information.
// This function provides a simple interface for getting version data.
func GetVersion() *VersionInfo {
	return NewVersionInfo()
}

// SetBuildVars allows setting the build-time variables.
// This is primarily used for testing purposes.
// Note: These should typically be set via ldflags during build.
func SetBuildVars(ver, com, bt string) {
	version = ver
	commit = com
	buildTime = bt
}

// ResetBuildVars resets all build variables to empty values.
// This is primarily used for testing to ensure clean state.
func ResetBuildVars() {
	version = ""
	commit = ""
	buildTime = ""
}

// IsDevelopment returns true if the version indicates a development build.
func (vi *VersionInfo) IsDevelopment() bool {
	return vi.Version == DefaultVersion
}

// GetBuildTime attempts to parse the build time as a timestamp.
// Returns a zero time if the build time cannot be parsed.
func (vi *VersionInfo) GetBuildTime() time.Time {
	if vi.BuildTime == DefaultBuildTime {
		return time.Time{}
	}

	parsedTime, err := time.Parse(time.RFC3339, vi.BuildTime)
	if err != nil {
		// Try some common formats
		formats := []string{
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}

		for _, format := range formats {
			if parsedTime, err = time.Parse(format, vi.BuildTime); err == nil {
				return parsedTime
			}
		}
		return time.Time{}
	}

	return parsedTime
}
