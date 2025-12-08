package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestReleaseScriptExecution tests that release.sh can be executed.
func TestReleaseScriptExecution(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	_, err := os.Stat(scriptPath)
	if os.IsNotExist(err) {
		t.Fatalf("Release script does not exist at %s", scriptPath)
	}

	cmd := exec.Command("bash", scriptPath, "v1.0.0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Expected release script to execute successfully, got error: %v\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
	}

	if stdout.Len() == 0 {
		t.Error("Expected release script to produce output")
	}
}

// TestReleaseScriptVersionArgument tests that release.sh requires version.
func TestReleaseScriptVersionArgument(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	cmd := exec.Command("bash", scriptPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("Expected release script to fail without version argument")
	}

	output := stderr.String()
	if !strings.Contains(strings.ToLower(output), "version") {
		t.Errorf("Expected error message to mention version, got: %s", output)
	}
}

// TestReleaseScriptVersionFileUpdate tests VERSION file update.
func TestReleaseScriptVersionFileUpdate(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	versionFile := filepath.Join(tempDir, "VERSION")
	os.WriteFile(versionFile, []byte("v1.0.0-prev"), 0o644)

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	newVersion := "v2.0.0"

	cmd := exec.Command("bash", scriptPath, newVersion)
	cmd.Env = append(os.Environ(), fmt.Sprintf("VERSION_FILE=%s", versionFile), "DRY_RUN=false")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected release script to execute successfully, got error: %v", err)
	}

	content, _ := os.ReadFile(versionFile)
	updatedVersion := strings.TrimSpace(string(content))
	if updatedVersion != newVersion {
		t.Errorf("Expected VERSION file to be updated to %s, got %s", newVersion, updatedVersion)
	}
}

// TestReleaseScriptBuildExecution tests that build.sh is called.
func TestReleaseScriptBuildExecution(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	cmd := exec.Command("bash", scriptPath, "v1.2.3")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected release script to execute successfully, got error: %v", err)
	}

	output := stdout.String() + stderr.String()
	if !strings.Contains(output, "build") {
		t.Error("Expected release script to run build script")
	}
}

// TestReleaseScriptDirectoryCreation tests release directory creation.
func TestReleaseScriptDirectoryCreation(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	testVersion := "v3.0.0"
	tempDir := t.TempDir()
	releaseDir := filepath.Join(tempDir, "releases")

	cmd := exec.Command("bash", scriptPath, testVersion)
	cmd.Env = append(os.Environ(), fmt.Sprintf("RELEASE_DIR=%s", releaseDir), "DRY_RUN=false")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected release script to execute successfully, got error: %v", err)
	}

	if _, err := os.Stat(releaseDir); os.IsNotExist(err) {
		t.Errorf("Expected release directory %s to be created", releaseDir)
	}

	versionDir := filepath.Join(releaseDir, testVersion)
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		t.Errorf("Expected version directory %s to be created", versionDir)
	}
}

// TestReleaseScriptBinaryCopying tests binary copying with version names.
func TestReleaseScriptBinaryCopying(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	testVersion := "v1.5.0"
	tempDir := t.TempDir()
	releaseDir := filepath.Join(tempDir, "releases")
	buildDir := filepath.Join(tempDir, "build")

	// Create dummy binaries
	os.MkdirAll(buildDir, 0o755)
	os.WriteFile(filepath.Join(buildDir, "codechunking"), []byte("main binary"), 0o755)
	os.WriteFile(filepath.Join(buildDir, "client"), []byte("client binary"), 0o755)

	cmd := exec.Command("bash", scriptPath, testVersion)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("RELEASE_DIR=%s", releaseDir),
		fmt.Sprintf("BUILD_DIR=%s", buildDir),
		"DRY_RUN=false")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected release script to execute successfully, got error: %v", err)
	}

	versionDir := filepath.Join(releaseDir, testVersion)
	mainBinary := filepath.Join(versionDir, fmt.Sprintf("codechunking-%s", testVersion))
	if _, err := os.Stat(mainBinary); os.IsNotExist(err) {
		t.Errorf("Expected versioned main binary %s to be created", mainBinary)
	}
}

// TestReleaseScriptChecksumGeneration tests checksum generation.
func TestReleaseScriptChecksumGeneration(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	testVersion := "v2.1.0"
	tempDir := t.TempDir()
	releaseDir := filepath.Join(tempDir, "releases")
	buildDir := filepath.Join(tempDir, "build")

	// Setup
	versionDir := filepath.Join(releaseDir, testVersion)
	os.MkdirAll(versionDir, 0o755)
	os.MkdirAll(buildDir, 0o755)

	mainBinary := filepath.Join(versionDir, fmt.Sprintf("codechunking-%s", testVersion))
	os.WriteFile(mainBinary, []byte("dummy main binary"), 0o755)

	cmd := exec.Command("bash", scriptPath, testVersion)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("RELEASE_DIR=%s", releaseDir),
		fmt.Sprintf("BUILD_DIR=%s", buildDir),
		"DRY_RUN=false")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected release script to execute successfully, got error: %v", err)
	}

	checksumsFile := filepath.Join(versionDir, "checksums.txt")
	if _, err := os.Stat(checksumsFile); os.IsNotExist(err) {
		t.Errorf("Expected checksums file %s to be created", checksumsFile)
	}
}

// TestReleaseScriptDryRun tests dry run mode.
func TestReleaseScriptDryRun(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	tempDir := t.TempDir()

	cmd := exec.Command("bash", scriptPath, "--dry-run", "v1.0.0-dry")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("RELEASE_DIR=%s", tempDir),
		fmt.Sprintf("VERSION_FILE=%s", filepath.Join(tempDir, "VERSION")))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected release script to execute successfully in dry run mode, got error: %v", err)
	}

	output := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(output), "dry") {
		t.Error("Expected dry run mode to be mentioned in output")
	}
}

// TestReleaseScriptVersionValidation tests input validation.
func TestReleaseScriptVersionValidation(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	invalidVersions := []string{"1.0.0", "v1.2", "not-a-version"}

	for _, invalidVersion := range invalidVersions {
		t.Run(fmt.Sprintf("InvalidVersion_%s", invalidVersion), func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command("bash", scriptPath, invalidVersion)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err == nil {
				t.Errorf("Expected release script to fail with invalid version '%s'", invalidVersion)
			}

			output := stderr.String()
			if !strings.Contains(strings.ToLower(output), "version") {
				t.Errorf("Expected validation error for '%s'", invalidVersion)
			}
		})
	}
}

// TestReleaseScriptGitTag tests git tag creation.
func TestReleaseScriptGitTag(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join("..", "..", "scripts", "release.sh")
	cmd := exec.Command("bash", scriptPath, "v1.4.0")
	cmd.Env = append(os.Environ(), "CREATE_GIT_TAG=true")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Expected release script to execute successfully, got error: %v", err)
	}

	output := stdout.String() + stderr.String()
	if !strings.Contains(output, "tag") {
		t.Error("Expected release script to create git tag")
	}
}
