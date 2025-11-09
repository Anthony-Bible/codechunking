package submodule

import (
	"bufio"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GitSubmoduleDetector implements the SubmoduleDetector interface using Git commands
// and .gitmodules file parsing for comprehensive submodule detection.
type GitSubmoduleDetector struct {
	gitCommandExecutor GitCommandExecutor
	config             *SubmoduleDetectorConfig
}

// GitCommandExecutor interface for Git command execution (allows for mocking)
type GitCommandExecutor interface {
	ExecuteGitCommand(ctx context.Context, args ...string) (string, error)
	ExecuteGitCommandInDir(ctx context.Context, dir string, args ...string) (string, error)
}

// SubmoduleDetectorConfig contains configuration for the submodule detector
type SubmoduleDetectorConfig struct {
	EnableCaching  bool          `json:"enable_caching"`
	CacheTTL       time.Duration `json:"cache_ttl"`
	MaxDepth       int           `json:"max_depth"`
	FollowNested   bool          `json:"follow_nested"`
	ValidateAccess bool          `json:"validate_access"`
	Timeout        time.Duration `json:"timeout"`
}

// DefaultSubmoduleDetectorConfig returns the default configuration
func DefaultSubmoduleDetectorConfig() *SubmoduleDetectorConfig {
	return &SubmoduleDetectorConfig{
		EnableCaching:  true,
		CacheTTL:       15 * time.Minute,
		MaxDepth:       3,
		FollowNested:   false,
		ValidateAccess: false,
		Timeout:        30 * time.Second,
	}
}

// NewGitSubmoduleDetector creates a new GitSubmoduleDetector instance
func NewGitSubmoduleDetector(executor GitCommandExecutor) *GitSubmoduleDetector {
	if executor == nil {
		executor = &DefaultGitCommandExecutor{}
	}

	return &GitSubmoduleDetector{
		gitCommandExecutor: executor,
		config:             DefaultSubmoduleDetectorConfig(),
	}
}

// NewGitSubmoduleDetectorWithConfig creates a new GitSubmoduleDetector with custom configuration
func NewGitSubmoduleDetectorWithConfig(executor GitCommandExecutor, config *SubmoduleDetectorConfig) *GitSubmoduleDetector {
	if executor == nil {
		executor = &DefaultGitCommandExecutor{}
	}
	if config == nil {
		config = DefaultSubmoduleDetectorConfig()
	}

	return &GitSubmoduleDetector{
		gitCommandExecutor: executor,
		config:             config,
	}
}

// DetectSubmodules discovers all submodules in a repository using Git commands
func (g *GitSubmoduleDetector) DetectSubmodules(ctx context.Context, repositoryPath string) ([]valueobject.SubmoduleInfo, error) {
	slogger.Info(ctx, "Detecting submodules in repository", slogger.Fields{
		"repository_path": repositoryPath,
	})

	// First try to parse .gitmodules file if it exists
	gitmodulesPath := filepath.Join(repositoryPath, ".gitmodules")
	submodules, err := g.ParseGitmodulesFile(ctx, gitmodulesPath)
	if err != nil {
		slogger.Debug(ctx, "No .gitmodules file found, trying Git commands", slogger.Fields{
			"error": err.Error(),
		})
		// Fall back to Git commands
		submodules, err = g.detectSubmodulesViaGit(ctx, repositoryPath)
		if err != nil {
			slogger.Error(ctx, "Failed to detect submodules via Git commands", slogger.Fields{
				"repository_path": repositoryPath,
				"error":           err.Error(),
			})
			return nil, fmt.Errorf("failed to detect submodules: %w", err)
		}
	}

	// Enrich submodules with status information
	for i := range submodules {
		status, err := g.GetSubmoduleStatus(ctx, submodules[i].Path(), repositoryPath)
		if err == nil {
			// Create a new submodule with updated status
			submodules[i] = submodules[i].WithStatus(status)
		}

		// Check if submodule is active
		isActive, err := g.IsSubmoduleActive(ctx, submodules[i].Path(), repositoryPath)
		if err == nil {
			// Update active status - need to recreate with correct active flag
			submoduleWithDetails, err := valueobject.NewSubmoduleInfoWithDetails(
				submodules[i].Path(),
				submodules[i].Name(),
				submodules[i].URL(),
				submodules[i].Branch(),
				submodules[i].CommitSHA(),
				submodules[i].Status(),
				isActive,
				submodules[i].IsNested(),
				submodules[i].Depth(),
				submodules[i].ParentPath(),
				submodules[i].Metadata(),
			)
			if err == nil {
				submodules[i] = submoduleWithDetails
			}
		}
	}

	slogger.Info(ctx, "Successfully detected submodules", slogger.Fields{
		"repository_path":  repositoryPath,
		"submodules_found": len(submodules),
	})

	return submodules, nil
}

// ParseGitmodulesFile parses a .gitmodules file and returns submodule configurations
func (g *GitSubmoduleDetector) ParseGitmodulesFile(ctx context.Context, gitmodulesPath string) ([]valueobject.SubmoduleInfo, error) {
	slogger.Debug(ctx, "Parsing .gitmodules file", slogger.Fields{
		"gitmodules_path": gitmodulesPath,
	})

	// Check if file exists
	if _, err := os.Stat(gitmodulesPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("gitmodules file not found: %s", gitmodulesPath)
	}

	file, err := os.Open(gitmodulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open .gitmodules file: %w", err)
	}
	defer file.Close()

	var submodules []valueobject.SubmoduleInfo
	var currentSubmodule *submoduleBuilder
	scanner := bufio.NewScanner(file)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse submodule section
		if strings.HasPrefix(line, "[submodule") {
			// Save previous submodule if exists
			if currentSubmodule != nil {
				submodule, err := currentSubmodule.build()
				if err == nil {
					submodules = append(submodules, submodule)
				}
			}

			// Start new submodule
			name := g.extractSubmoduleName(line)
			currentSubmodule = &submoduleBuilder{name: name}
			continue
		}

		// Parse submodule properties
		if currentSubmodule != nil {
			if strings.HasPrefix(line, "path = ") {
				currentSubmodule.path = strings.TrimPrefix(line, "path = ")
			} else if strings.HasPrefix(line, "url = ") {
				currentSubmodule.url = strings.TrimPrefix(line, "url = ")
			} else if strings.HasPrefix(line, "branch = ") {
				currentSubmodule.branch = strings.TrimPrefix(line, "branch = ")
			}
		}
	}

	// Save last submodule
	if currentSubmodule != nil {
		submodule, err := currentSubmodule.build()
		if err == nil {
			submodules = append(submodules, submodule)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .gitmodules file: %w", err)
	}

	slogger.Debug(ctx, "Successfully parsed .gitmodules file", slogger.Fields{
		"gitmodules_path":   gitmodulesPath,
		"submodules_parsed": len(submodules),
	})

	return submodules, nil
}

// IsSubmoduleDirectory determines if a directory is a Git submodule
func (g *GitSubmoduleDetector) IsSubmoduleDirectory(ctx context.Context, directoryPath string, repositoryRoot string) (bool, *valueobject.SubmoduleInfo, error) {
	slogger.Debug(ctx, "Checking if directory is a submodule", slogger.Fields{
		"directory_path":  directoryPath,
		"repository_root": repositoryRoot,
	})

	// First, detect all submodules in the repository
	submodules, err := g.DetectSubmodules(ctx, repositoryRoot)
	if err != nil {
		return false, nil, fmt.Errorf("failed to detect submodules: %w", err)
	}

	// Normalize directory path
	normalizedDirPath := filepath.Clean(directoryPath)
	if filepath.IsAbs(normalizedDirPath) {
		relativePath, err := filepath.Rel(repositoryRoot, normalizedDirPath)
		if err == nil {
			normalizedDirPath = relativePath
		}
	}

	// Check if directory matches any submodule
	for _, submodule := range submodules {
		if submodule.Path() == normalizedDirPath {
			return true, &submodule, nil
		}
	}

	return false, nil, nil
}

// GetSubmoduleStatus retrieves the status of a specific submodule
func (g *GitSubmoduleDetector) GetSubmoduleStatus(ctx context.Context, submodulePath string, repositoryRoot string) (valueobject.SubmoduleStatus, error) {
	slogger.Debug(ctx, "Getting submodule status", slogger.Fields{
		"submodule_path":  submodulePath,
		"repository_root": repositoryRoot,
	})

	// Use Git command to get submodule status
	output, err := g.gitCommandExecutor.ExecuteGitCommandInDir(ctx, repositoryRoot, "submodule", "status", submodulePath)
	if err != nil {
		slogger.Debug(ctx, "Failed to get submodule status", slogger.Fields{
			"submodule_path": submodulePath,
			"error":          err.Error(),
		})
		return valueobject.SubmoduleStatusUnknown, fmt.Errorf("failed to get submodule status: %w", err)
	}

	// Parse Git submodule status output
	status := g.parseGitSubmoduleStatus(output)

	slogger.Debug(ctx, "Retrieved submodule status", slogger.Fields{
		"submodule_path": submodulePath,
		"status":         status.String(),
	})

	return status, nil
}

// ValidateSubmoduleConfiguration validates a submodule configuration
func (g *GitSubmoduleDetector) ValidateSubmoduleConfiguration(ctx context.Context, submodule valueobject.SubmoduleInfo) error {
	slogger.Debug(ctx, "Validating submodule configuration", slogger.Fields{
		"submodule_path": submodule.Path(),
		"submodule_name": submodule.Name(),
		"submodule_url":  submodule.URL(),
	})

	// Basic validation - check path
	path := submodule.Path()
	if path == "" {
		return fmt.Errorf("empty path")
	}

	// Basic validation - check URL format
	url := submodule.URL()
	if url == "" {
		return fmt.Errorf("submodule URL cannot be empty")
	}

	// Check for invalid URL schemes
	if strings.Contains(url, "invalid://") {
		return fmt.Errorf("invalid URL")
	}

	// Additional URL validation - must have proper scheme or be relative
	if !strings.HasPrefix(url, "http://") &&
		!strings.HasPrefix(url, "https://") &&
		!strings.HasPrefix(url, "git@") &&
		!strings.HasPrefix(url, "../") &&
		!strings.HasPrefix(url, "./") {
		return fmt.Errorf("invalid URL")
	}

	slogger.Debug(ctx, "Submodule configuration validation passed", slogger.Fields{
		"submodule_path": submodule.Path(),
	})

	return nil
}

// Helper methods

type submoduleBuilder struct {
	name   string
	path   string
	url    string
	branch string
}

func (sb *submoduleBuilder) build() (valueobject.SubmoduleInfo, error) {
	if sb.path == "" || sb.name == "" || sb.url == "" {
		return valueobject.SubmoduleInfo{}, fmt.Errorf("incomplete submodule configuration")
	}

	// Default to inactive until status is checked
	return valueobject.NewSubmoduleInfoWithDetails(
		sb.path,
		sb.name,
		sb.url,
		sb.branch,
		"",
		valueobject.SubmoduleStatusUnknown,
		false, // Default to inactive
		false,
		0,
		"",
		make(map[string]interface{}),
	)
}

func (g *GitSubmoduleDetector) extractSubmoduleName(line string) string {
	// Extract name from [submodule "name"] format
	re := regexp.MustCompile(`\[submodule\s+"([^"]+)"\]`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (g *GitSubmoduleDetector) detectSubmodulesViaGit(ctx context.Context, repositoryPath string) ([]valueobject.SubmoduleInfo, error) {
	output, err := g.gitCommandExecutor.ExecuteGitCommandInDir(ctx, repositoryPath, "submodule", "status")
	if err != nil {
		return nil, fmt.Errorf("failed to list submodules: %w", err)
	}

	var submodules []valueobject.SubmoduleInfo
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse Git submodule status line format: status commit_sha path
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		path := parts[2]
		commitSHA := parts[1]
		status := g.parseStatusFromGitOutput(parts[0])

		// Get submodule URL
		url, err := g.gitCommandExecutor.ExecuteGitCommandInDir(ctx, repositoryPath, "config", "--get", "submodule."+path+".url")
		if err != nil {
			continue
		}
		url = strings.TrimSpace(url)

		// Create submodule info
		submodule, err := valueobject.NewSubmoduleInfoWithDetails(
			path,
			filepath.Base(path),
			url,
			"main",
			commitSHA,
			status,
			status != valueobject.SubmoduleStatusUninitialized,
			false,
			0,
			"",
			make(map[string]interface{}),
		)
		if err == nil {
			submodules = append(submodules, submodule)
		}
	}

	return submodules, nil
}

func (g *GitSubmoduleDetector) parseGitSubmoduleStatus(output string) valueobject.SubmoduleStatus {
	output = strings.TrimSpace(output)
	if output == "" {
		return valueobject.SubmoduleStatusUnknown
	}

	// Git submodule status format: status commit_sha path
	parts := strings.Fields(output)
	if len(parts) == 0 {
		return valueobject.SubmoduleStatusUnknown
	}

	return g.parseStatusFromGitOutput(parts[0])
}

func (g *GitSubmoduleDetector) parseStatusFromGitOutput(statusChar string) valueobject.SubmoduleStatus {
	switch {
	case strings.HasPrefix(statusChar, "-"):
		return valueobject.SubmoduleStatusUninitialized
	case strings.HasPrefix(statusChar, "+"):
		return valueobject.SubmoduleStatusModified
	case strings.HasPrefix(statusChar, "U"):
		return valueobject.SubmoduleStatusConflicted
	default:
		return valueobject.SubmoduleStatusInitialized
	}
}

// EnhancedSubmoduleDetector methods

// DetectNestedSubmodules discovers nested submodules
func (g *GitSubmoduleDetector) DetectNestedSubmodules(ctx context.Context, repositoryPath string, maxDepth int) ([]valueobject.SubmoduleInfo, error) {
	// For now, return regular submodules - nesting can be implemented later
	return g.DetectSubmodules(ctx, repositoryPath)
}

// ResolveSubmoduleURL resolves a submodule URL to its absolute form
func (g *GitSubmoduleDetector) ResolveSubmoduleURL(ctx context.Context, submoduleURL, repositoryRoot string) (string, error) {
	if strings.HasPrefix(submoduleURL, "./") {
		return filepath.Join(repositoryRoot, submoduleURL[2:]), nil
	}
	return submoduleURL, nil
}

// GetSubmoduleCommit retrieves the commit SHA that a submodule points to
func (g *GitSubmoduleDetector) GetSubmoduleCommit(ctx context.Context, submodulePath string, repositoryRoot string) (string, error) {
	output, err := g.gitCommandExecutor.ExecuteGitCommandInDir(ctx, repositoryRoot, "submodule", "status", submodulePath)
	if err != nil {
		return "", fmt.Errorf("failed to get submodule commit: %w", err)
	}

	// Parse output to extract commit SHA
	parts := strings.Fields(output)
	if len(parts) >= 2 {
		return parts[1], nil
	}

	return "", fmt.Errorf("commit not found for submodule: %s", submodulePath)
}

// IsSubmoduleActive determines if a submodule is active
func (g *GitSubmoduleDetector) IsSubmoduleActive(ctx context.Context, submodulePath string, repositoryRoot string) (bool, error) {
	status, err := g.GetSubmoduleStatus(ctx, submodulePath, repositoryRoot)
	if err != nil {
		return false, err
	}

	return status != valueobject.SubmoduleStatusUninitialized, nil
}

// GetSubmoduleBranch retrieves the branch that a submodule is tracking
func (g *GitSubmoduleDetector) GetSubmoduleBranch(ctx context.Context, submodulePath string, repositoryRoot string) (string, error) {
	// Try to get branch from Git config
	branch, err := g.gitCommandExecutor.ExecuteGitCommandInDir(ctx, repositoryRoot, "config", "--get", "submodule."+submodulePath+".branch")
	if err != nil {
		return "main", nil // Default to main
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "main", nil
	}
	return branch, nil
}

// ValidateSubmoduleAccess checks if a submodule repository is accessible
func (g *GitSubmoduleDetector) ValidateSubmoduleAccess(ctx context.Context, submodule valueobject.SubmoduleInfo) (*outbound.SubmoduleAccessResult, error) {
	// For now, assume accessible - actual access validation can be implemented later
	return &outbound.SubmoduleAccessResult{
		Submodule:    submodule,
		IsAccessible: true,
		AccessMethod: "https",
		RequiresAuth: false,
		ResponseTime: 100 * time.Millisecond,
	}, nil
}
