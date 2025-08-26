package valueobject

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// CloneOptions represents git clone configuration options.
type CloneOptions struct {
	depth             int
	singleBranch      bool
	branch            string
	shallow           bool
	timeout           time.Duration
	recurseSubmodules bool
}

// NewCloneOptions creates a new CloneOptions value object.
func NewCloneOptions(depth int, branch string, shallow bool) (CloneOptions, error) {
	if depth < 0 {
		return CloneOptions{}, fmt.Errorf("depth cannot be negative: %d", depth)
	}

	if shallow && depth == 0 {
		return CloneOptions{}, errors.New("shallow clone requires depth > 0")
	}

	singleBranch := shallow && depth > 0

	return CloneOptions{
		depth:             depth,
		singleBranch:      singleBranch,
		branch:            branch,
		shallow:           shallow,
		timeout:           30 * time.Minute, // default timeout
		recurseSubmodules: false,
	}, nil
}

// NewShallowCloneOptions creates a new shallow clone options.
func NewShallowCloneOptions(depth int, branch string) CloneOptions {
	opts, err := NewCloneOptions(depth, branch, true)
	if err != nil {
		// This shouldn't happen if depth > 0
		panic(fmt.Sprintf("invalid shallow clone options: %v", err))
	}
	return opts
}

// NewFullCloneOptions creates options for a full clone.
func NewFullCloneOptions() CloneOptions {
	opts, _ := NewCloneOptions(0, "", false)
	return opts
}

// WithTimeout returns a new CloneOptions with the specified timeout.
func (co CloneOptions) WithTimeout(timeout time.Duration) (CloneOptions, error) {
	if timeout <= 0 {
		return CloneOptions{}, fmt.Errorf("timeout must be positive: %v", timeout)
	}

	co.timeout = timeout
	return co, nil
}

// WithRecurseSubmodules returns a new CloneOptions with submodule recursion setting.
func (co CloneOptions) WithRecurseSubmodules(recurse bool) CloneOptions {
	co.recurseSubmodules = recurse
	return co
}

// IsShallowClone returns true if this represents a shallow clone.
func (co CloneOptions) IsShallowClone() bool {
	return co.shallow
}

// IsSingleBranch returns true if this clone should fetch only a single branch.
func (co CloneOptions) IsSingleBranch() bool {
	return co.singleBranch
}

// Depth returns the clone depth (0 means unlimited).
func (co CloneOptions) Depth() int {
	return co.depth
}

// Branch returns the specific branch to clone (empty means default).
func (co CloneOptions) Branch() string {
	return co.branch
}

// Timeout returns the clone timeout duration.
func (co CloneOptions) Timeout() time.Duration {
	return co.timeout
}

// RecurseSubmodules returns true if submodules should be recursively cloned.
func (co CloneOptions) RecurseSubmodules() bool {
	return co.recurseSubmodules
}

// ToGitArgs converts the clone options to git command line arguments.
func (co CloneOptions) ToGitArgs() []string {
	var args []string

	if co.shallow && co.depth > 0 {
		args = append(args, "--depth="+strconv.Itoa(co.depth))
	}

	if co.singleBranch {
		args = append(args, "--single-branch")
		if co.branch != "" {
			args = append(args, "--branch="+co.branch)
		}
	}

	if co.recurseSubmodules {
		args = append(args, "--recurse-submodules")
	}

	return args
}

// String returns a string representation of the CloneOptions.
func (co CloneOptions) String() string {
	return fmt.Sprintf(
		"CloneOptions{shallow=%t, depth=%d, singleBranch=%t, branch=%s, timeout=%s, recurseSubmodules=%t}",
		co.shallow,
		co.depth,
		co.singleBranch,
		co.branch,
		co.timeout,
		co.recurseSubmodules,
	)
}

// Equal compares two CloneOptions for equality.
func (co CloneOptions) Equal(other CloneOptions) bool {
	return co.depth == other.depth &&
		co.singleBranch == other.singleBranch &&
		co.branch == other.branch &&
		co.shallow == other.shallow &&
		co.timeout == other.timeout &&
		co.recurseSubmodules == other.recurseSubmodules
}
