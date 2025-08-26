package valueobject

import (
	"testing"
	"time"
)

func TestNewCloneOptions(t *testing.T) {
	tests := []struct {
		name    string
		depth   int
		branch  string
		shallow bool
		want    CloneOptions
		wantErr bool
	}{
		{
			name:    "should create default full clone options",
			depth:   0,
			branch:  "",
			shallow: false,
			want: CloneOptions{
				depth:        0,
				singleBranch: false,
				branch:       "",
				shallow:      false,
				timeout:      30 * time.Minute,
			},
		},
		{
			name:    "should create shallow clone options with depth 1",
			depth:   1,
			branch:  "main",
			shallow: true,
			want: CloneOptions{
				depth:        1,
				singleBranch: true,
				branch:       "main",
				shallow:      true,
				timeout:      30 * time.Minute,
			},
		},
		{
			name:    "should create shallow clone with custom depth",
			depth:   50,
			branch:  "develop",
			shallow: true,
			want: CloneOptions{
				depth:        50,
				singleBranch: true,
				branch:       "develop",
				shallow:      true,
				timeout:      30 * time.Minute,
			},
		},
		{
			name:    "should reject negative depth",
			depth:   -1,
			branch:  "main",
			shallow: true,
			want:    CloneOptions{},
			wantErr: true,
		},
		{
			name:    "should reject shallow clone with zero depth",
			depth:   0,
			branch:  "main",
			shallow: true,
			want:    CloneOptions{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCloneOptions(tt.depth, tt.branch, tt.shallow)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCloneOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("NewCloneOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_WithTimeout(t *testing.T) {
	tests := []struct {
		name    string
		base    CloneOptions
		timeout time.Duration
		want    CloneOptions
		wantErr bool
	}{
		{
			name:    "should set valid timeout",
			base:    mustCreateCloneOptions(1, "main", true),
			timeout: 10 * time.Minute,
			want: CloneOptions{
				depth:        1,
				singleBranch: true,
				branch:       "main",
				shallow:      true,
				timeout:      10 * time.Minute,
			},
		},
		{
			name:    "should reject zero timeout",
			base:    mustCreateCloneOptions(1, "main", true),
			timeout: 0,
			want:    CloneOptions{},
			wantErr: true,
		},
		{
			name:    "should reject negative timeout",
			base:    mustCreateCloneOptions(1, "main", true),
			timeout: -5 * time.Minute,
			want:    CloneOptions{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.base.WithTimeout(tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("WithTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_WithRecurseSubmodules(t *testing.T) {
	tests := []struct {
		name              string
		base              CloneOptions
		recurseSubmodules bool
		want              CloneOptions
	}{
		{
			name:              "should enable submodule recursion",
			base:              mustCreateCloneOptions(1, "main", true),
			recurseSubmodules: true,
			want: CloneOptions{
				depth:             1,
				singleBranch:      true,
				branch:            "main",
				shallow:           true,
				timeout:           30 * time.Minute,
				recurseSubmodules: true,
			},
		},
		{
			name:              "should disable submodule recursion",
			base:              mustCreateCloneOptions(0, "", false),
			recurseSubmodules: false,
			want: CloneOptions{
				depth:             0,
				singleBranch:      false,
				branch:            "",
				shallow:           false,
				timeout:           30 * time.Minute,
				recurseSubmodules: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.base.WithRecurseSubmodules(tt.recurseSubmodules)
			if !got.Equal(tt.want) {
				t.Errorf("WithRecurseSubmodules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_IsShallowClone(t *testing.T) {
	tests := []struct {
		name string
		opts CloneOptions
		want bool
	}{
		{
			name: "should return true for shallow clone with depth 1",
			opts: mustCreateCloneOptions(1, "main", true),
			want: true,
		},
		{
			name: "should return true for shallow clone with custom depth",
			opts: mustCreateCloneOptions(50, "develop", true),
			want: true,
		},
		{
			name: "should return false for full clone",
			opts: mustCreateCloneOptions(0, "", false),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.IsShallowClone(); got != tt.want {
				t.Errorf("IsShallowClone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_IsSingleBranch(t *testing.T) {
	tests := []struct {
		name string
		opts CloneOptions
		want bool
	}{
		{
			name: "should return true for single branch clone",
			opts: mustCreateCloneOptions(1, "main", true),
			want: true,
		},
		{
			name: "should return false for full clone",
			opts: mustCreateCloneOptions(0, "", false),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.IsSingleBranch(); got != tt.want {
				t.Errorf("IsSingleBranch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_Depth(t *testing.T) {
	tests := []struct {
		name string
		opts CloneOptions
		want int
	}{
		{
			name: "should return depth for shallow clone",
			opts: mustCreateCloneOptions(1, "main", true),
			want: 1,
		},
		{
			name: "should return zero for full clone",
			opts: mustCreateCloneOptions(0, "", false),
			want: 0,
		},
		{
			name: "should return custom depth",
			opts: mustCreateCloneOptions(100, "develop", true),
			want: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.Depth(); got != tt.want {
				t.Errorf("Depth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_Branch(t *testing.T) {
	tests := []struct {
		name string
		opts CloneOptions
		want string
	}{
		{
			name: "should return branch name for single branch clone",
			opts: mustCreateCloneOptions(1, "main", true),
			want: "main",
		},
		{
			name: "should return empty string for full clone",
			opts: mustCreateCloneOptions(0, "", false),
			want: "",
		},
		{
			name: "should return custom branch name",
			opts: mustCreateCloneOptions(1, "feature/awesome", true),
			want: "feature/awesome",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.Branch(); got != tt.want {
				t.Errorf("Branch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_Timeout(t *testing.T) {
	tests := []struct {
		name string
		opts CloneOptions
		want time.Duration
	}{
		{
			name: "should return default timeout",
			opts: mustCreateCloneOptions(1, "main", true),
			want: 30 * time.Minute,
		},
		{
			name: "should return custom timeout",
			opts: mustCreateCloneOptionsWithTimeout(1, "main", true, 10*time.Minute),
			want: 10 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.Timeout(); got != tt.want {
				t.Errorf("Timeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_RecurseSubmodules(t *testing.T) {
	tests := []struct {
		name string
		opts CloneOptions
		want bool
	}{
		{
			name: "should return false by default",
			opts: mustCreateCloneOptions(1, "main", true),
			want: false,
		},
		{
			name: "should return true when enabled",
			opts: mustCreateCloneOptions(1, "main", true).WithRecurseSubmodules(true),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.RecurseSubmodules(); got != tt.want {
				t.Errorf("RecurseSubmodules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_String(t *testing.T) {
	tests := []struct {
		name string
		opts CloneOptions
		want string
	}{
		{
			name: "should format shallow clone options",
			opts: mustCreateCloneOptions(1, "main", true),
			want: "CloneOptions{shallow=true, depth=1, singleBranch=true, branch=main, timeout=30m0s, recurseSubmodules=false}",
		},
		{
			name: "should format full clone options",
			opts: mustCreateCloneOptions(0, "", false),
			want: "CloneOptions{shallow=false, depth=0, singleBranch=false, branch=, timeout=30m0s, recurseSubmodules=false}",
		},
		{
			name: "should format complex clone options",
			opts: mustCreateCloneOptionsWithTimeoutAndSubmodules(50, "develop", true, 15*time.Minute, true),
			want: "CloneOptions{shallow=true, depth=50, singleBranch=true, branch=develop, timeout=15m0s, recurseSubmodules=true}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_Equal(t *testing.T) {
	base := mustCreateCloneOptions(1, "main", true)

	tests := []struct {
		name  string
		opts1 CloneOptions
		opts2 CloneOptions
		want  bool
	}{
		{
			name:  "should return true for identical options",
			opts1: base,
			opts2: base,
			want:  true,
		},
		{
			name:  "should return true for equivalent options",
			opts1: mustCreateCloneOptions(1, "main", true),
			opts2: mustCreateCloneOptions(1, "main", true),
			want:  true,
		},
		{
			name:  "should return false for different depth",
			opts1: mustCreateCloneOptions(1, "main", true),
			opts2: mustCreateCloneOptions(2, "main", true),
			want:  false,
		},
		{
			name:  "should return false for different branch",
			opts1: mustCreateCloneOptions(1, "main", true),
			opts2: mustCreateCloneOptions(1, "develop", true),
			want:  false,
		},
		{
			name:  "should return false for different shallow setting",
			opts1: mustCreateCloneOptions(1, "main", true),
			opts2: mustCreateCloneOptions(0, "", false),
			want:  false,
		},
		{
			name:  "should return false for different timeout",
			opts1: mustCreateCloneOptions(1, "main", true),
			opts2: mustCreateCloneOptionsWithTimeout(1, "main", true, 10*time.Minute),
			want:  false,
		},
		{
			name:  "should return false for different submodule setting",
			opts1: mustCreateCloneOptions(1, "main", true),
			opts2: mustCreateCloneOptions(1, "main", true).WithRecurseSubmodules(true),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts1.Equal(tt.opts2); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneOptions_ToGitArgs(t *testing.T) {
	tests := []struct {
		name string
		opts CloneOptions
		want []string
	}{
		{
			name: "should generate args for shallow single-branch clone",
			opts: mustCreateCloneOptions(1, "main", true),
			want: []string{"--depth=1", "--single-branch", "--branch=main"},
		},
		{
			name: "should generate args for shallow clone with custom depth",
			opts: mustCreateCloneOptions(50, "develop", true),
			want: []string{"--depth=50", "--single-branch", "--branch=develop"},
		},
		{
			name: "should generate args for full clone",
			opts: mustCreateCloneOptions(0, "", false),
			want: []string{},
		},
		{
			name: "should include submodule recursion",
			opts: mustCreateCloneOptions(1, "main", true).WithRecurseSubmodules(true),
			want: []string{"--depth=1", "--single-branch", "--branch=main", "--recurse-submodules"},
		},
		{
			name: "should handle shallow clone without specific branch",
			opts: mustCreateCloneOptions(5, "", true),
			want: []string{"--depth=5", "--single-branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.opts.ToGitArgs()
			if len(got) != len(tt.want) {
				t.Errorf("ToGitArgs() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i, arg := range got {
				if arg != tt.want[i] {
					t.Errorf("ToGitArgs()[%d] = %v, want %v", i, arg, tt.want[i])
				}
			}
		})
	}
}

func TestNewShallowCloneOptions(t *testing.T) {
	tests := []struct {
		name   string
		depth  int
		branch string
		want   CloneOptions
	}{
		{
			name:   "should create shallow clone options with depth 1",
			depth:  1,
			branch: "main",
			want: CloneOptions{
				depth:        1,
				singleBranch: true,
				branch:       "main",
				shallow:      true,
				timeout:      30 * time.Minute,
			},
		},
		{
			name:   "should create shallow clone options with custom depth",
			depth:  100,
			branch: "develop",
			want: CloneOptions{
				depth:        100,
				singleBranch: true,
				branch:       "develop",
				shallow:      true,
				timeout:      30 * time.Minute,
			},
		},
		{
			name:   "should create shallow clone options without specific branch",
			depth:  10,
			branch: "",
			want: CloneOptions{
				depth:        10,
				singleBranch: true,
				branch:       "",
				shallow:      true,
				timeout:      30 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewShallowCloneOptions(tt.depth, tt.branch)
			if !got.Equal(tt.want) {
				t.Errorf("NewShallowCloneOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewFullCloneOptions(t *testing.T) {
	want := CloneOptions{
		depth:        0,
		singleBranch: false,
		branch:       "",
		shallow:      false,
		timeout:      30 * time.Minute,
	}

	got := NewFullCloneOptions()
	if !got.Equal(want) {
		t.Errorf("NewFullCloneOptions() = %v, want %v", got, want)
	}
}

// Helper function to create CloneOptions without error handling for tests.
func mustCreateCloneOptions(depth int, branch string, shallow bool) CloneOptions {
	opts, err := NewCloneOptions(depth, branch, shallow)
	if err != nil {
		panic(err)
	}
	return opts
}

// Helper function to create CloneOptions with timeout.
func mustCreateCloneOptionsWithTimeout(depth int, branch string, shallow bool, timeout time.Duration) CloneOptions {
	opts := mustCreateCloneOptions(depth, branch, shallow)
	optsWithTimeout, err := opts.WithTimeout(timeout)
	if err != nil {
		panic(err)
	}
	return optsWithTimeout
}

// Helper function to create CloneOptions with timeout and submodules.
func mustCreateCloneOptionsWithTimeoutAndSubmodules(
	depth int,
	branch string,
	shallow bool,
	timeout time.Duration,
	recurseSubmodules bool,
) CloneOptions {
	opts := mustCreateCloneOptions(depth, branch, shallow)
	optsWithTimeout, err := opts.WithTimeout(timeout)
	if err != nil {
		panic(err)
	}
	return optsWithTimeout.WithRecurseSubmodules(recurseSubmodules)
}
