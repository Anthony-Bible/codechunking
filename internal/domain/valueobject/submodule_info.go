package valueobject

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// SubmoduleStatus represents the status of a Git submodule.
type SubmoduleStatus int

const (
	SubmoduleStatusUnknown SubmoduleStatus = iota
	SubmoduleStatusUninitialized
	SubmoduleStatusInitialized
	SubmoduleStatusUpToDate
	SubmoduleStatusOutOfDate
	SubmoduleStatusModified
	SubmoduleStatusConflicted
)

// SubmoduleUpdatePolicy defines how submodules should be handled during processing.
type SubmoduleUpdatePolicy int

const (
	SubmoduleUpdatePolicyUnknown      SubmoduleUpdatePolicy = iota
	SubmoduleUpdatePolicyIgnore                             // Skip submodules entirely
	SubmoduleUpdatePolicyShallow                            // Process submodules but don't recurse
	SubmoduleUpdatePolicyRecursive                          // Process submodules recursively
	SubmoduleUpdatePolicyFollowParent                       // Use parent repository's policy
)

// SubmoduleInfo represents a Git submodule with comprehensive metadata.
// It serves as a value object in the domain layer for submodule handling
// throughout the code processing pipeline.
type SubmoduleInfo struct {
	path       string
	name       string
	url        string
	branch     string
	commitSHA  string
	status     SubmoduleStatus
	isActive   bool
	isNested   bool
	depth      int
	parentPath string
	localPath  string
	createdAt  time.Time
	updatedAt  time.Time
	metadata   map[string]interface{}
}

// NewSubmoduleInfo creates a new SubmoduleInfo value object with validation.
func NewSubmoduleInfo(path, name, repoURL string) (SubmoduleInfo, error) {
	if path == "" {
		return SubmoduleInfo{}, errors.New("submodule path cannot be empty")
	}

	if name == "" {
		return SubmoduleInfo{}, errors.New("submodule name cannot be empty")
	}

	if repoURL == "" {
		return SubmoduleInfo{}, errors.New("submodule repository URL cannot be empty")
	}

	// Normalize path
	normalizedPath := filepath.Clean(path)
	if filepath.IsAbs(normalizedPath) {
		return SubmoduleInfo{}, errors.New("submodule path must be relative")
	}

	// Validate repository URL
	if err := validateRepositoryURL(repoURL); err != nil {
		return SubmoduleInfo{}, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Normalize name
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return SubmoduleInfo{}, errors.New("submodule name cannot be empty after normalization")
	}

	now := time.Now()
	return SubmoduleInfo{
		path:       normalizedPath,
		name:       normalizedName,
		url:        repoURL,
		branch:     "main", // Default branch
		commitSHA:  "",
		status:     SubmoduleStatusUnknown,
		isActive:   true,
		isNested:   false,
		depth:      0,
		parentPath: "",
		localPath:  "",
		createdAt:  now,
		updatedAt:  now,
		metadata:   make(map[string]interface{}),
	}, nil
}

// NewSubmoduleInfoWithDetails creates a SubmoduleInfo with comprehensive details.
func NewSubmoduleInfoWithDetails(
	path, name, repoURL, branch, commitSHA string,
	status SubmoduleStatus,
	isActive, isNested bool,
	depth int,
	parentPath string,
	metadata map[string]interface{},
) (SubmoduleInfo, error) {
	// Create base submodule
	submodule, err := NewSubmoduleInfo(path, name, repoURL)
	if err != nil {
		return SubmoduleInfo{}, err
	}

	// Validate branch
	if branch != "" {
		if err := validateBranchName(branch); err != nil {
			return SubmoduleInfo{}, fmt.Errorf("invalid branch name: %w", err)
		}
		submodule.branch = branch
	}

	// Validate commit SHA
	if commitSHA != "" {
		if err := validateCommitSHA(commitSHA); err != nil {
			return SubmoduleInfo{}, fmt.Errorf("invalid commit SHA: %w", err)
		}
		submodule.commitSHA = commitSHA
	}

	// Validate depth
	if depth < 0 || depth > 100 {
		return SubmoduleInfo{}, errors.New("submodule depth must be between 0 and 100")
	}

	// Validate parent path
	if parentPath != "" {
		cleanParentPath := filepath.Clean(parentPath)
		if filepath.IsAbs(cleanParentPath) {
			return SubmoduleInfo{}, errors.New("parent path must be relative")
		}
		submodule.parentPath = cleanParentPath
	}

	// Validate metadata
	if len(metadata) > 50 {
		return SubmoduleInfo{}, errors.New("too many metadata entries (max 50)")
	}

	submodule.status = status
	submodule.isActive = isActive
	submodule.isNested = isNested
	submodule.depth = depth
	submodule.metadata = copyMetadataMap(metadata)
	submodule.updatedAt = time.Now()

	return submodule, nil
}

// validateRepositoryURL validates a Git repository URL.
func validateRepositoryURL(repoURL string) error {
	if len(repoURL) > 2000 {
		return errors.New("repository URL too long (max 2000 characters)")
	}

	// Handle different URL formats
	if strings.HasPrefix(repoURL, "git@") {
		// SSH format: git@github.com:user/repo.git
		return validateSSHURL(repoURL)
	}

	if strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://") {
		// HTTP/HTTPS format
		return validateHTTPURL(repoURL)
	}

	if strings.HasPrefix(repoURL, "file://") || strings.HasPrefix(repoURL, "/") || strings.HasPrefix(repoURL, "./") ||
		strings.HasPrefix(repoURL, "../") {
		// Local path format
		return validateLocalURL(repoURL)
	}

	return errors.New("unsupported repository URL format")
}

// validateSSHURL validates SSH-style Git URLs.
func validateSSHURL(sshURL string) error {
	// Basic SSH URL validation: git@host:path
	parts := strings.Split(sshURL, ":")
	if len(parts) < 2 {
		return errors.New("invalid SSH URL format")
	}

	userHost := parts[0]
	if !strings.Contains(userHost, "@") {
		return errors.New("SSH URL must contain user@host format")
	}

	return nil
}

// validateHTTPURL validates HTTP/HTTPS Git URLs.
func validateHTTPURL(httpURL string) error {
	parsedURL, err := url.Parse(httpURL)
	if err != nil {
		return fmt.Errorf("invalid HTTP URL: %w", err)
	}

	if parsedURL.Host == "" {
		return errors.New("HTTP URL must have a host")
	}

	return nil
}

// validateLocalURL validates local file path Git URLs.
func validateLocalURL(localURL string) error {
	cleanURL := strings.TrimPrefix(localURL, "file://")
	cleanPath := filepath.Clean(cleanURL)

	if cleanPath == "" || cleanPath == "." {
		return errors.New("local URL cannot be empty or current directory")
	}

	return nil
}

// validateBranchName validates a Git branch name.
func validateBranchName(branch string) error {
	if len(branch) > 250 {
		return errors.New("branch name too long (max 250 characters)")
	}

	// Basic branch name validation (Git rules are complex, this is simplified)
	if strings.HasPrefix(branch, "-") || strings.HasSuffix(branch, ".") {
		return errors.New("invalid branch name format")
	}

	if strings.Contains(branch, "..") || strings.Contains(branch, " ") {
		return errors.New("branch name cannot contain '..' or spaces")
	}

	return nil
}

// validateCommitSHA validates a Git commit SHA.
func validateCommitSHA(sha string) error {
	if len(sha) != 40 && len(sha) != 7 {
		return errors.New("commit SHA must be either 7 characters (short) or 40 characters (full)")
	}

	for _, char := range sha {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return errors.New("commit SHA must contain only hexadecimal characters")
		}
	}

	return nil
}

// copyMetadataMap creates a deep copy of metadata map.
func copyMetadataMap(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return make(map[string]interface{})
	}

	copied := make(map[string]interface{}, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}

// Path returns the submodule path.
func (s SubmoduleInfo) Path() string {
	return s.path
}

// Name returns the submodule name.
func (s SubmoduleInfo) Name() string {
	return s.name
}

// URL returns the submodule repository URL.
func (s SubmoduleInfo) URL() string {
	return s.url
}

// Branch returns the submodule branch.
func (s SubmoduleInfo) Branch() string {
	return s.branch
}

// CommitSHA returns the submodule commit SHA.
func (s SubmoduleInfo) CommitSHA() string {
	return s.commitSHA
}

// Status returns the submodule status.
func (s SubmoduleInfo) Status() SubmoduleStatus {
	return s.status
}

// IsActive returns true if the submodule is active.
func (s SubmoduleInfo) IsActive() bool {
	return s.isActive
}

// IsNested returns true if this is a nested submodule.
func (s SubmoduleInfo) IsNested() bool {
	return s.isNested
}

// Depth returns the nesting depth of the submodule.
func (s SubmoduleInfo) Depth() int {
	return s.depth
}

// ParentPath returns the parent submodule path (if nested).
func (s SubmoduleInfo) ParentPath() string {
	return s.parentPath
}

// LocalPath returns the local filesystem path.
func (s SubmoduleInfo) LocalPath() string {
	return s.localPath
}

// CreatedAt returns the submodule creation time.
func (s SubmoduleInfo) CreatedAt() time.Time {
	return s.createdAt
}

// UpdatedAt returns the submodule last update time.
func (s SubmoduleInfo) UpdatedAt() time.Time {
	return s.updatedAt
}

// Metadata returns a copy of the submodule metadata.
func (s SubmoduleInfo) Metadata() map[string]interface{} {
	return copyMetadataMap(s.metadata)
}

// ShouldProcess determines if this submodule should be processed based on policy.
func (s SubmoduleInfo) ShouldProcess(policy SubmoduleUpdatePolicy) bool {
	if !s.isActive {
		return false
	}

	switch policy {
	case SubmoduleUpdatePolicyIgnore:
		return false
	case SubmoduleUpdatePolicyShallow:
		return s.depth == 0 // Only process direct submodules
	case SubmoduleUpdatePolicyRecursive:
		return true // Process all submodules
	case SubmoduleUpdatePolicyFollowParent:
		return s.depth <= 1 // Process up to one level deep
	case SubmoduleUpdatePolicyUnknown:
		return false
	default:
		return false
	}
}

// WithStatus returns a new SubmoduleInfo with updated status.
func (s SubmoduleInfo) WithStatus(status SubmoduleStatus) SubmoduleInfo {
	newSubmodule := s
	newSubmodule.status = status
	newSubmodule.updatedAt = time.Now()
	return newSubmodule
}

// WithCommitSHA returns a new SubmoduleInfo with updated commit SHA.
func (s SubmoduleInfo) WithCommitSHA(commitSHA string) (SubmoduleInfo, error) {
	if commitSHA != "" {
		if err := validateCommitSHA(commitSHA); err != nil {
			return SubmoduleInfo{}, fmt.Errorf("invalid commit SHA: %w", err)
		}
	}

	newSubmodule := s
	newSubmodule.commitSHA = commitSHA
	newSubmodule.updatedAt = time.Now()
	return newSubmodule, nil
}

// WithLocalPath returns a new SubmoduleInfo with updated local path.
func (s SubmoduleInfo) WithLocalPath(localPath string) SubmoduleInfo {
	newSubmodule := s
	newSubmodule.localPath = filepath.Clean(localPath)
	newSubmodule.updatedAt = time.Now()
	return newSubmodule
}

// WithNesting returns a new SubmoduleInfo with updated nesting information.
func (s SubmoduleInfo) WithNesting(isNested bool, depth int, parentPath string) (SubmoduleInfo, error) {
	if depth < 0 || depth > 100 {
		return SubmoduleInfo{}, errors.New("submodule depth must be between 0 and 100")
	}

	cleanParentPath := ""
	if parentPath != "" {
		cleanParentPath = filepath.Clean(parentPath)
		if filepath.IsAbs(cleanParentPath) {
			return SubmoduleInfo{}, errors.New("parent path must be relative")
		}
	}

	newSubmodule := s
	newSubmodule.isNested = isNested
	newSubmodule.depth = depth
	newSubmodule.parentPath = cleanParentPath
	newSubmodule.updatedAt = time.Now()
	return newSubmodule, nil
}

// String returns a string representation of the submodule info.
func (s SubmoduleInfo) String() string {
	return fmt.Sprintf("%s (%s) -> %s [%s]", s.name, s.path, s.url, s.status.String())
}

// Equal compares two SubmoduleInfo instances for equality.
func (s SubmoduleInfo) Equal(other SubmoduleInfo) bool {
	return s.path == other.path &&
		s.name == other.name &&
		s.url == other.url &&
		s.branch == other.branch &&
		s.commitSHA == other.commitSHA
}

// String returns a string representation of the submodule status.
func (ss SubmoduleStatus) String() string {
	switch ss {
	case SubmoduleStatusUninitialized:
		return "Uninitialized"
	case SubmoduleStatusInitialized:
		return "Initialized"
	case SubmoduleStatusUpToDate:
		return "UpToDate"
	case SubmoduleStatusOutOfDate:
		return "OutOfDate"
	case SubmoduleStatusModified:
		return "Modified"
	case SubmoduleStatusConflicted:
		return "Conflicted"
	case SubmoduleStatusUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}

// String returns a string representation of the submodule update policy.
func (sup SubmoduleUpdatePolicy) String() string {
	switch sup {
	case SubmoduleUpdatePolicyIgnore:
		return "Ignore"
	case SubmoduleUpdatePolicyShallow:
		return "Shallow"
	case SubmoduleUpdatePolicyRecursive:
		return "Recursive"
	case SubmoduleUpdatePolicyFollowParent:
		return "FollowParent"
	case SubmoduleUpdatePolicyUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}
