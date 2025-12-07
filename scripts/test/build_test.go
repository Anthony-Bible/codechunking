package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBuildScriptExecution tests that the build.sh script can be executed.
func TestBuildScriptExecution(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	// Verify script exists and is executable
	_, err := os.Stat(scriptPath)
	if os.IsNotExist(err) {
		t.Fatalf("Build script does not exist at %s", scriptPath)
	}

	// Test script execution with version argument
	cmd := exec.Command("bash", scriptPath, "v1.0.0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully, got error: %v\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
	}

	// Verify script produced output
	if stdout.Len() == 0 {
		t.Error("Expected build script to produce output")
	}
}

// TestBuildScriptVersionFromFile tests that build.sh reads version from VERSION file.
func TestBuildScriptVersionFromFile(t *testing.T) {
	t.Parallel()

	// Create a temporary VERSION file
	tempDir := t.TempDir()
	versionFile := filepath.Join(tempDir, "VERSION")
	versionContent := "v2.1.0-beta"
	err := os.WriteFile(versionFile, []byte(versionContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create VERSION file: %v", err)
	}

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	// Test script execution without version argument (should read from file)
	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("VERSION_FILE=%s", versionFile))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully with VERSION file, got error: %v", err)
	}

	// Verify version was used from file
	output := stdout.String() + stderr.String()
	if !strings.Contains(output, versionContent) {
		t.Errorf("Expected output to contain version %s from VERSION file, got: %s",
			versionContent, output)
	}
}

// TestBuildScriptVersionArgument tests that build.sh accepts version as argument.
func TestBuildScriptVersionArgument(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")
	testVersion := "v3.0.0-alpha.1"

	// Test script execution with version argument
	cmd := exec.Command("bash", scriptPath, testVersion)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully with version argument, got error: %v", err)
	}

	// Verify version was used from argument
	output := stdout.String() + stderr.String()
	if !strings.Contains(output, testVersion) {
		t.Errorf("Expected output to contain version %s from argument, got: %s",
			testVersion, output)
	}
}

// TestBuildScriptGitCommitInfo tests that build.sh includes git commit information.
func TestBuildScriptGitCommitInfo(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	// Test script execution
	cmd := exec.Command("bash", scriptPath, "v1.0.0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully, got error: %v", err)
	}

	output := stdout.String() + stderr.String()

	// Verify git commit information is included
	// These should be part of the ldflags
	if !strings.Contains(output, "git commit") && !strings.Contains(output, "commit") {
		t.Error("Expected build script to include git commit information")
	}
}

// TestBuildScriptBinaryOutput tests that build.sh produces the expected binaries.
func TestBuildScriptBinaryOutput(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")
	tempDir := t.TempDir()

	// Test script execution with custom output directory
	cmd := exec.Command("bash", scriptPath, "v1.0.0")
	cmd.Env = append(os.Environ(), fmt.Sprintf("OUTPUT_DIR=%s", tempDir))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully, got error: %v", err)
	}

	// Verify main binary exists
	mainBinary := filepath.Join(tempDir, "codechunking")
	if _, err := os.Stat(mainBinary); os.IsNotExist(err) {
		t.Errorf("Expected main binary %s to be created", mainBinary)
	}

	// Verify client binary exists
	clientBinary := filepath.Join(tempDir, "client")
	if _, err := os.Stat(clientBinary); os.IsNotExist(err) {
		t.Errorf("Expected client binary %s to be created", clientBinary)
	}
}

// TestBuildScriptLDFlags tests that build.sh uses correct ldflags for version injection.
func TestBuildScriptLDFlags(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")
	testVersion := "v2.5.0-rc1"

	// Test script execution
	cmd := exec.Command("bash", scriptPath, testVersion)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully, got error: %v", err)
	}

	output := stdout.String() + stderr.String()

	// Verify ldflags are being used
	if !strings.Contains(output, "-ldflags") {
		t.Error("Expected build script to use ldflags for version injection")
	}

	// Verify version is in ldflags
	if !strings.Contains(output, testVersion) {
		t.Errorf("Expected ldflags to contain version %s", testVersion)
	}
}

// TestBuildScriptErrorHandling tests error handling for missing dependencies.
func TestBuildScriptErrorHandling(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	// Test script execution with invalid environment
	cmd := exec.Command("bash", scriptPath, "v1.0.0")
	// Remove Go from PATH to simulate missing dependency
	cmd.Env = append(os.Environ(), "PATH=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("Expected build script to fail when Go is not available")
	}

	// Verify error message is helpful
	output := stdout.String() + stderr.String()
	if len(output) == 0 {
		t.Error("Expected build script to provide error message when failing")
	}
}

// TestBuildScriptHelpOption tests that build.sh supports help/usage.
func TestBuildScriptHelpOption(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	// Test script help option
	cmd := exec.Command("bash", scriptPath, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// Help should exit successfully
	if err != nil {
		t.Fatalf("Expected build script help to execute successfully, got error: %v", err)
	}

	// Verify help information is displayed
	output := stdout.String()
	if len(output) == 0 {
		output = stderr.String()
	}

	if !strings.Contains(strings.ToLower(output), "usage") {
		t.Error("Expected build script help to contain usage information")
	}
}

// TestBuildScriptVerboseMode tests verbose mode functionality.
func TestBuildScriptVerboseMode(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	// Test script with verbose flag
	cmd := exec.Command("bash", scriptPath, "-v", "v1.0.0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully in verbose mode, got error: %v", err)
	}

	// Verify verbose output contains build commands
	output := stdout.String() + stderr.String()
	if !strings.Contains(output, "go build") {
		t.Error("Expected verbose mode to show build commands")
	}
}

// TestBuildScriptCleanBuild tests clean build functionality.
func TestBuildScriptCleanBuild(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	// Test script with clean flag
	cmd := exec.Command("bash", scriptPath, "--clean", "v1.0.0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully with clean flag, got error: %v", err)
	}

	// Verify clean build message
	output := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(output), "clean") {
		t.Error("Expected clean build mode to be mentioned in output")
	}
}

// TestBuildScriptCrossPlatform tests that build.sh supports cross-platform builds.
func TestBuildScriptCrossPlatform(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	// Test script with platform arguments
	cmd := exec.Command("bash", scriptPath, "--platform", "linux/amd64", "v1.0.0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected build script to execute successfully with platform flag, got error: %v", err)
	}

	// Verify platform information in output
	output := stdout.String() + stderr.String()
	if !strings.Contains(output, "linux/amd64") {
		t.Error("Expected platform information to be included in build output")
	}
}

// TestBuildScriptParallelBuilds tests parallel build functionality.
func TestBuildScriptParallelBuilds(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	start := time.Now()

	// Run multiple builds in parallel
	cmd1 := exec.Command("bash", scriptPath, "v1.0.0")
	cmd2 := exec.Command("bash", scriptPath, "v1.0.1")

	err1 := cmd1.Run()
	err2 := cmd2.Run()

	duration := time.Since(start)

	if err1 != nil || err2 != nil {
		t.Fatalf("Expected parallel builds to execute successfully, got errors: err1=%v, err2=%v", err1, err2)
	}

	// Builds should complete in reasonable time (parallel execution)
	if duration > 30*time.Second {
		t.Errorf("Builds took too long (%v), expected parallel execution to be faster", duration)
	}
}

// TestBuildScriptVersionValidation tests version input validation.
func TestBuildScriptVersionValidation(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "build.sh")

	invalidVersions := []string{
		"not-a-version",
		"",
		"1.2.3", // Missing v prefix
		"v1.2",  // Incomplete version
	}

	for _, invalidVersion := range invalidVersions {
		t.Run(fmt.Sprintf("InvalidVersion_%s", invalidVersion), func(t *testing.T) {
			t.Parallel()
			var args []string
			if invalidVersion != "" {
				args = append(args, invalidVersion)
			}

			cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err == nil {
				t.Errorf("Expected build script to fail with invalid version '%s'", invalidVersion)
			}

			// Verify validation error message
			output := stderr.String()
			if !strings.Contains(strings.ToLower(output), "invalid") &&
				!strings.Contains(strings.ToLower(output), "version") {
				t.Errorf("Expected version validation error message for '%s', got: %s",
					invalidVersion, output)
			}
		})
	}
}
