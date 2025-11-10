package submodule

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultGitCommandExecutor provides a default implementation of Git command execution.
type DefaultGitCommandExecutor struct {
	timeout time.Duration
}

// NewDefaultGitCommandExecutor creates a new DefaultGitCommandExecutor with default timeout.
func NewDefaultGitCommandExecutor() *DefaultGitCommandExecutor {
	return &DefaultGitCommandExecutor{
		timeout: 30 * time.Second,
	}
}

// NewDefaultGitCommandExecutorWithTimeout creates a new DefaultGitCommandExecutor with custom timeout.
func NewDefaultGitCommandExecutorWithTimeout(timeout time.Duration) *DefaultGitCommandExecutor {
	return &DefaultGitCommandExecutor{
		timeout: timeout,
	}
}

// ExecuteGitCommand executes a Git command in the current directory.
func (e *DefaultGitCommandExecutor) ExecuteGitCommand(ctx context.Context, args ...string) (string, error) {
	return e.ExecuteGitCommandInDir(ctx, ".", args...)
}

// ExecuteGitCommandInDir executes a Git command in a specific directory.
func (e *DefaultGitCommandExecutor) ExecuteGitCommandInDir(
	ctx context.Context,
	dir string,
	args ...string,
) (string, error) {
	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "git", args...)
	cmd.Dir = dir

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %w, output: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
