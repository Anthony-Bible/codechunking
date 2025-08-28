package valueobject

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// SymlinkType represents the type of symbolic link.
type SymlinkType int

const (
	SymlinkTypeUnknown SymlinkType = iota
	SymlinkTypeFile
	SymlinkTypeDirectory
	SymlinkTypeBroken
)

// SymlinkResolutionPolicy defines how symbolic links should be handled during processing.
type SymlinkResolutionPolicy int

const (
	SymlinkResolutionPolicyUnknown SymlinkResolutionPolicy = iota
	SymlinkResolutionPolicyIgnore                          // Skip symbolic links entirely
	SymlinkResolutionPolicyResolve                         // Follow symbolic links and process targets
	SymlinkResolutionPolicyShallow                         // Process symbolic links but don't follow further links in target
	SymlinkResolutionPolicyDeep                            // Follow symbolic links recursively with cycle detection
)

// SymlinkScope represents whether a symlink points inside or outside the repository.
type SymlinkScope int

const (
	SymlinkScopeUnknown  SymlinkScope = iota
	SymlinkScopeInternal              // Points to a location within the repository
	SymlinkScopeExternal              // Points to a location outside the repository
	SymlinkScopeBroken                // Points to a non-existent location
)

// SymlinkInfo represents a symbolic link with comprehensive metadata.
// It serves as a value object in the domain layer for symlink handling
// throughout the code processing pipeline.
type SymlinkInfo struct {
	path           string
	targetPath     string
	resolvedPath   string
	linkType       SymlinkType
	scope          SymlinkScope
	isBroken       bool
	isCircular     bool
	isRecursive    bool
	depth          int
	linkChain      []string
	repositoryRoot string
	createdAt      time.Time
	updatedAt      time.Time
	metadata       map[string]interface{}
}

// NewSymlinkInfo creates a new SymlinkInfo value object with validation.
func NewSymlinkInfo(path, targetPath string) (SymlinkInfo, error) {
	if path == "" {
		return SymlinkInfo{}, errors.New("symlink path cannot be empty")
	}

	if targetPath == "" {
		return SymlinkInfo{}, errors.New("symlink target path cannot be empty")
	}

	// Normalize paths
	normalizedPath := filepath.Clean(path)
	normalizedTargetPath := filepath.Clean(targetPath)

	// Validate path formats
	if err := validateSymlinkPath(normalizedPath); err != nil {
		return SymlinkInfo{}, fmt.Errorf("invalid symlink path: %w", err)
	}

	if err := validateTargetPath(normalizedTargetPath); err != nil {
		return SymlinkInfo{}, fmt.Errorf("invalid target path: %w", err)
	}

	now := time.Now()
	return SymlinkInfo{
		path:           normalizedPath,
		targetPath:     normalizedTargetPath,
		resolvedPath:   "",
		linkType:       SymlinkTypeUnknown,
		scope:          SymlinkScopeUnknown,
		isBroken:       false,
		isCircular:     false,
		isRecursive:    false,
		depth:          0,
		linkChain:      []string{normalizedPath},
		repositoryRoot: "",
		createdAt:      now,
		updatedAt:      now,
		metadata:       make(map[string]interface{}),
	}, nil
}

// NewSymlinkInfoWithDetails creates a SymlinkInfo with comprehensive details.
func NewSymlinkInfoWithDetails(
	path, targetPath, resolvedPath, repositoryRoot string,
	linkType SymlinkType,
	scope SymlinkScope,
	isBroken, isCircular bool,
	linkChain []string,
	metadata map[string]interface{},
) (SymlinkInfo, error) {
	// Create base symlink
	symlink, err := NewSymlinkInfo(path, targetPath)
	if err != nil {
		return SymlinkInfo{}, err
	}

	// Validate resolved path
	if resolvedPath != "" {
		cleanResolvedPath := filepath.Clean(resolvedPath)
		if err := validateTargetPath(cleanResolvedPath); err != nil {
			return SymlinkInfo{}, fmt.Errorf("invalid resolved path: %w", err)
		}
		symlink.resolvedPath = cleanResolvedPath
	}

	// Validate repository root
	if repositoryRoot != "" {
		cleanRepoRoot := filepath.Clean(repositoryRoot)
		if !filepath.IsAbs(cleanRepoRoot) {
			return SymlinkInfo{}, errors.New("repository root must be absolute path")
		}
		symlink.repositoryRoot = cleanRepoRoot
	}

	// Validate link chain
	if len(linkChain) > 100 {
		return SymlinkInfo{}, errors.New("link chain too long (max 100 links)")
	}

	// Validate metadata
	if len(metadata) > 50 {
		return SymlinkInfo{}, errors.New("too many metadata entries (max 50)")
	}

	symlink.linkType = linkType
	symlink.scope = scope
	symlink.isBroken = isBroken
	symlink.isCircular = isCircular
	symlink.depth = len(linkChain) - 1
	symlink.linkChain = append([]string{}, linkChain...) // Copy slice
	symlink.metadata = copyMetadataMap(metadata)
	symlink.updatedAt = time.Now()

	// Set recursive flag
	symlink.isRecursive = symlink.depth > 0

	return symlink, nil
}

// validateSymlinkPath validates a symbolic link path.
func validateSymlinkPath(path string) error {
	if len(path) > 4096 {
		return errors.New("symlink path too long (max 4096 characters)")
	}

	if strings.Contains(path, "\x00") {
		return errors.New("symlink path cannot contain null bytes")
	}

	return nil
}

// validateTargetPath validates a symbolic link target path.
func validateTargetPath(targetPath string) error {
	if len(targetPath) > 4096 {
		return errors.New("target path too long (max 4096 characters)")
	}

	if strings.Contains(targetPath, "\x00") {
		return errors.New("target path cannot contain null bytes")
	}

	return nil
}

// Path returns the symbolic link path.
func (s SymlinkInfo) Path() string {
	return s.path
}

// TargetPath returns the symbolic link target path.
func (s SymlinkInfo) TargetPath() string {
	return s.targetPath
}

// ResolvedPath returns the fully resolved path (if resolved).
func (s SymlinkInfo) ResolvedPath() string {
	return s.resolvedPath
}

// LinkType returns the type of the symbolic link.
func (s SymlinkInfo) LinkType() SymlinkType {
	return s.linkType
}

// Scope returns the scope of the symbolic link.
func (s SymlinkInfo) Scope() SymlinkScope {
	return s.scope
}

// IsBroken returns true if the symbolic link is broken.
func (s SymlinkInfo) IsBroken() bool {
	return s.isBroken
}

// IsCircular returns true if the symbolic link creates a circular reference.
func (s SymlinkInfo) IsCircular() bool {
	return s.isCircular
}

// IsRecursive returns true if this symlink is part of a recursive chain.
func (s SymlinkInfo) IsRecursive() bool {
	return s.isRecursive
}

// Depth returns the depth of the symbolic link chain.
func (s SymlinkInfo) Depth() int {
	return s.depth
}

// LinkChain returns the chain of symbolic links.
func (s SymlinkInfo) LinkChain() []string {
	// Return a copy to prevent external modification
	chain := make([]string, len(s.linkChain))
	copy(chain, s.linkChain)
	return chain
}

// RepositoryRoot returns the repository root path.
func (s SymlinkInfo) RepositoryRoot() string {
	return s.repositoryRoot
}

// CreatedAt returns the symlink creation time.
func (s SymlinkInfo) CreatedAt() time.Time {
	return s.createdAt
}

// UpdatedAt returns the symlink last update time.
func (s SymlinkInfo) UpdatedAt() time.Time {
	return s.updatedAt
}

// Metadata returns a copy of the symlink metadata.
func (s SymlinkInfo) Metadata() map[string]interface{} {
	return copyMetadataMap(s.metadata)
}

// ShouldProcess determines if this symlink should be processed based on policy.
func (s SymlinkInfo) ShouldProcess(policy SymlinkResolutionPolicy) bool {
	// Don't process broken or circular symlinks regardless of policy
	if s.isBroken || s.isCircular {
		return false
	}

	switch policy {
	case SymlinkResolutionPolicyIgnore:
		return false
	case SymlinkResolutionPolicyResolve:
		return true
	case SymlinkResolutionPolicyShallow:
		return s.depth == 0 // Only process direct symlinks
	case SymlinkResolutionPolicyDeep:
		return s.depth < 10 // Prevent excessive recursion
	case SymlinkResolutionPolicyUnknown:
		return false
	default:
		return false
	}
}

// IsWithinRepository returns true if the symlink target is within the repository.
func (s SymlinkInfo) IsWithinRepository() bool {
	return s.scope == SymlinkScopeInternal
}

// IsOutsideRepository returns true if the symlink target is outside the repository.
func (s SymlinkInfo) IsOutsideRepository() bool {
	return s.scope == SymlinkScopeExternal
}

// HasCircularReference returns true if the symlink creates a circular reference.
func (s SymlinkInfo) HasCircularReference() bool {
	return s.isCircular
}

// GetRelativePath returns the symlink path relative to the repository root.
func (s SymlinkInfo) GetRelativePath() string {
	if s.repositoryRoot == "" {
		return s.path
	}

	rel, err := filepath.Rel(s.repositoryRoot, s.path)
	if err != nil {
		return s.path
	}
	return rel
}

// GetRelativeTargetPath returns the target path relative to the repository root.
func (s SymlinkInfo) GetRelativeTargetPath() string {
	if s.repositoryRoot == "" || s.resolvedPath == "" {
		return s.targetPath
	}

	rel, err := filepath.Rel(s.repositoryRoot, s.resolvedPath)
	if err != nil {
		return s.targetPath
	}
	return rel
}

// WithResolution returns a new SymlinkInfo with resolution details.
func (s SymlinkInfo) WithResolution(resolvedPath string, linkType SymlinkType, scope SymlinkScope) SymlinkInfo {
	newSymlink := s
	newSymlink.resolvedPath = filepath.Clean(resolvedPath)
	newSymlink.linkType = linkType
	newSymlink.scope = scope
	newSymlink.updatedAt = time.Now()
	return newSymlink
}

// WithBrokenStatus returns a new SymlinkInfo with updated broken status.
func (s SymlinkInfo) WithBrokenStatus(isBroken bool) SymlinkInfo {
	newSymlink := s
	newSymlink.isBroken = isBroken
	if isBroken {
		newSymlink.scope = SymlinkScopeBroken
		newSymlink.linkType = SymlinkTypeBroken
	}
	newSymlink.updatedAt = time.Now()
	return newSymlink
}

// WithCircularReference returns a new SymlinkInfo with updated circular reference status.
func (s SymlinkInfo) WithCircularReference(isCircular bool, linkChain []string) (SymlinkInfo, error) {
	if len(linkChain) > 100 {
		return SymlinkInfo{}, errors.New("link chain too long (max 100 links)")
	}

	newSymlink := s
	newSymlink.isCircular = isCircular
	newSymlink.linkChain = append([]string{}, linkChain...) // Copy slice
	newSymlink.depth = len(linkChain) - 1
	newSymlink.isRecursive = newSymlink.depth > 0
	newSymlink.updatedAt = time.Now()
	return newSymlink, nil
}

// WithRepositoryRoot returns a new SymlinkInfo with updated repository root.
func (s SymlinkInfo) WithRepositoryRoot(repositoryRoot string) (SymlinkInfo, error) {
	if repositoryRoot != "" {
		cleanRepoRoot := filepath.Clean(repositoryRoot)
		if !filepath.IsAbs(cleanRepoRoot) {
			return SymlinkInfo{}, errors.New("repository root must be absolute path")
		}
		newSymlink := s
		newSymlink.repositoryRoot = cleanRepoRoot
		newSymlink.updatedAt = time.Now()
		return newSymlink, nil
	}

	newSymlink := s
	newSymlink.repositoryRoot = ""
	newSymlink.updatedAt = time.Now()
	return newSymlink, nil
}

// String returns a string representation of the symlink info.
func (s SymlinkInfo) String() string {
	return fmt.Sprintf("%s -> %s (%s, %s)", s.path, s.targetPath, s.linkType.String(), s.scope.String())
}

// Equal compares two SymlinkInfo instances for equality.
func (s SymlinkInfo) Equal(other SymlinkInfo) bool {
	return s.path == other.path &&
		s.targetPath == other.targetPath &&
		s.resolvedPath == other.resolvedPath &&
		s.linkType == other.linkType &&
		s.scope == other.scope
}

// String returns a string representation of the symlink type.
func (st SymlinkType) String() string {
	switch st {
	case SymlinkTypeFile:
		return "File"
	case SymlinkTypeDirectory:
		return "Directory"
	case SymlinkTypeBroken:
		return "Broken"
	case SymlinkTypeUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}

// String returns a string representation of the symlink resolution policy.
func (srp SymlinkResolutionPolicy) String() string {
	switch srp {
	case SymlinkResolutionPolicyIgnore:
		return "Ignore"
	case SymlinkResolutionPolicyResolve:
		return "Resolve"
	case SymlinkResolutionPolicyShallow:
		return "Shallow"
	case SymlinkResolutionPolicyDeep:
		return "Deep"
	case SymlinkResolutionPolicyUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}

// String returns a string representation of the symlink scope.
func (ss SymlinkScope) String() string {
	switch ss {
	case SymlinkScopeInternal:
		return "Internal"
	case SymlinkScopeExternal:
		return "External"
	case SymlinkScopeBroken:
		return "Broken"
	case SymlinkScopeUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}
