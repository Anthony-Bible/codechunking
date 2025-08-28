package symlink

import (
	"codechunking/internal/domain/valueobject"
	"os"
	"path/filepath"
	"strings"
)

// PathUtils provides common path manipulation utilities for symlink operations.
type PathUtils struct{}

// NewPathUtils creates a new PathUtils instance.
func NewPathUtils() *PathUtils {
	return &PathUtils{}
}

// ResolveRelativePath resolves a relative target path against a base symlink path.
// This is a common operation used across chain resolution and circular detection.
func (pu *PathUtils) ResolveRelativePath(basePath, targetPath string) string {
	if filepath.IsAbs(targetPath) {
		return filepath.Clean(targetPath)
	}

	baseDir := filepath.Dir(basePath)
	resolved := filepath.Join(baseDir, targetPath)
	return filepath.Clean(resolved)
}

// DetermineLinkType determines the type of a symlink target (file, directory, or broken).
// This is used consistently across multiple modules for type classification.
func (pu *PathUtils) DetermineLinkType(path string) valueobject.SymlinkType {
	info, err := os.Stat(path)
	if err != nil {
		return valueobject.SymlinkTypeBroken
	}

	if info.IsDir() {
		return valueobject.SymlinkTypeDirectory
	}
	return valueobject.SymlinkTypeFile
}

// DetermineScope determines if a resolved path is internal or external to a symlink's directory.
// This provides a consistent heuristic for scope classification across modules.
func (pu *PathUtils) DetermineScope(resolvedPath, symlinkPath string) valueobject.SymlinkScope {
	// Check if target exists
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return valueobject.SymlinkScopeBroken
	}

	// Get absolute paths for accurate comparison
	absResolvedPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return valueobject.SymlinkScopeUnknown
	}

	absSymlinkDir, err := filepath.Abs(filepath.Dir(symlinkPath))
	if err != nil {
		return valueobject.SymlinkScopeUnknown
	}

	// Check if the resolved path is within the symlink directory tree
	relPath, err := filepath.Rel(absSymlinkDir, absResolvedPath)
	if err != nil {
		return valueobject.SymlinkScopeExternal
	}

	// If the relative path starts with "..", it's outside
	if strings.HasPrefix(relPath, "..") {
		return valueobject.SymlinkScopeExternal
	}

	return valueobject.SymlinkScopeInternal
}

// IsPathWithinRoot checks if a given path is within a specified root directory.
// This is used for repository boundary checking in security validation.
func (pu *PathUtils) IsPathWithinRoot(targetPath, rootPath string) (bool, error) {
	// Resolve to absolute paths
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return false, err
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false, err
	}

	// Check if target is within root
	relPath, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false, err
	}

	// If the relative path starts with "..", it's outside the root
	return !strings.HasPrefix(relPath, ".."), nil
}

// CleanAndNormalizePath performs consistent path cleaning and normalization.
// This ensures paths are in a consistent format for comparison operations.
func (pu *PathUtils) CleanAndNormalizePath(path string) string {
	// Clean the path (remove ./, ../, // etc.)
	cleaned := filepath.Clean(path)

	// Convert to forward slashes for consistency (works on all platforms)
	normalized := filepath.ToSlash(cleaned)

	return normalized
}

// GetCommonPathPrefix finds the common prefix between two paths.
// This is useful for determining relationships between symlinks and their targets.
func (pu *PathUtils) GetCommonPathPrefix(path1, path2 string) string {
	// Normalize both paths
	norm1 := pu.CleanAndNormalizePath(path1)
	norm2 := pu.CleanAndNormalizePath(path2)

	// Split into components
	parts1 := strings.Split(norm1, "/")
	parts2 := strings.Split(norm2, "/")

	// Find common prefix
	var common []string
	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	for i := range minLen {
		if parts1[i] == parts2[i] {
			common = append(common, parts1[i])
		} else {
			break
		}
	}

	if len(common) == 0 {
		return ""
	}

	return strings.Join(common, "/")
}

// MapUtils provides utilities for working with path collections and mappings.
type MapUtils struct{}

// NewMapUtils creates a new MapUtils instance.
func NewMapUtils() *MapUtils {
	return &MapUtils{}
}

// ClearStringBoolMap efficiently clears a map[string]bool for reuse.
// This is used in circular detection and chain resolution to reset state.
func (mu *MapUtils) ClearStringBoolMap(m map[string]bool) {
	for k := range m {
		delete(m, k)
	}
}

// CopyStringSlice creates a deep copy of a string slice.
// This prevents accidental mutations of chain data across operations.
func (mu *MapUtils) CopyStringSlice(src []string) []string {
	if src == nil {
		return nil
	}

	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// ContainsString checks if a string slice contains a specific value.
// This is used for pattern matching and violation checking.
func (mu *MapUtils) ContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RemoveDuplicateStrings removes duplicate strings from a slice while preserving order.
// This is useful for cleaning up violation lists and chain paths.
func (mu *MapUtils) RemoveDuplicateStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
