package symlink

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Resolver implements outbound.SymlinkResolver interface for symlink detection and resolution.
type Resolver struct {
	pathUtils *PathUtils
}

// NewResolver creates a new symlink resolver instance.
func NewResolver() *Resolver {
	return &Resolver{
		pathUtils: NewPathUtils(),
	}
}

// DetectSymlinks discovers all symbolic links in a directory tree.
func (r *Resolver) DetectSymlinks(
	_ context.Context,
	directoryPath string,
) ([]valueobject.SymlinkInfo, error) {
	var symlinks []valueobject.SymlinkInfo

	err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Propagate the error
		}

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			targetPath, err := os.Readlink(path)
			if err != nil {
				// Create broken symlink info
				symlink, createErr := valueobject.NewSymlinkInfo(path, "")
				if createErr == nil {
					symlink = symlink.WithBrokenStatus(true)
					symlinks = append(symlinks, symlink)
				}
				return nil
			}

			symlink, err := valueobject.NewSymlinkInfo(path, targetPath)
			if err == nil {
				symlinks = append(symlinks, symlink)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory tree: %w", err)
	}

	return symlinks, nil
}

// IsSymlink determines if a given path is a symbolic link.
func (r *Resolver) IsSymlink(_ context.Context, filePath string) (bool, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file info: %w", err)
	}

	return info.Mode()&os.ModeSymlink != 0, nil
}

// ResolveSymlink resolves a symbolic link to its target path.
func (r *Resolver) ResolveSymlink(
	_ context.Context,
	symlinkPath string,
) (*valueobject.SymlinkInfo, error) {
	targetPath, err := os.Readlink(symlinkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read symlink: %w", err)
	}

	symlink, err := valueobject.NewSymlinkInfo(symlinkPath, targetPath)
	if err != nil {
		return nil, err
	}

	// Try to resolve to absolute path
	resolvedPath := r.resolveToAbsolutePath(symlinkPath, targetPath)

	// Check if the resolved path exists to determine if symlink is broken
	if _, err := os.Stat(resolvedPath); err == nil {
		// Determine link type
		linkType := r.determineLinkType(resolvedPath)
		scope := r.determineScope(resolvedPath, symlinkPath)
		symlink = symlink.WithResolution(resolvedPath, linkType, scope)
	} else {
		// Mark as broken
		symlink = symlink.WithBrokenStatus(true)
	}

	return &symlink, nil
}

// GetSymlinkTarget retrieves the target path of a symbolic link.
func (r *Resolver) GetSymlinkTarget(_ context.Context, symlinkPath string) (string, error) {
	targetPath, err := os.Readlink(symlinkPath)
	if err != nil {
		return "", fmt.Errorf("failed to read symlink: %w", err)
	}
	return targetPath, nil
}

// ValidateSymlinkTarget checks if a symlink target exists and is accessible.
func (r *Resolver) ValidateSymlinkTarget(
	ctx context.Context,
	symlink valueobject.SymlinkInfo,
) (*outbound.SymlinkValidationResult, error) {
	validator := NewSecurityValidator(r)
	return validator.ValidateSymlinkTarget(ctx, symlink)
}

// ResolveSymlinkChain fully resolves a chain of symbolic links.
func (r *Resolver) ResolveSymlinkChain(
	ctx context.Context,
	symlinkPath string,
	maxDepth int,
) (*valueobject.SymlinkInfo, error) {
	chainResolver := NewChainResolver(r, symlinkPath)
	return chainResolver.ResolveChain(ctx, symlinkPath, maxDepth)
}

// DetectCircularSymlinks detects circular references in symbolic links.
func (r *Resolver) DetectCircularSymlinks(
	ctx context.Context,
	symlinkPath string,
) (bool, []string, error) {
	detector := NewCircularDetector(r)
	return detector.DetectCircular(ctx, symlinkPath)
}

// ClassifySymlinkScope determines if a symlink points inside or outside the repository.
func (r *Resolver) ClassifySymlinkScope(
	_ context.Context,
	symlink valueobject.SymlinkInfo,
	repositoryRoot string,
) (valueobject.SymlinkScope, error) {
	targetPath := symlink.TargetPath()

	// Resolve relative path
	resolvedPath := targetPath
	if !filepath.IsAbs(targetPath) {
		baseDir := filepath.Dir(symlink.Path())
		resolvedPath = filepath.Join(baseDir, targetPath)
		resolvedPath = filepath.Clean(resolvedPath)
	}

	// Check if target exists
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return valueobject.SymlinkScopeBroken, nil
	}

	// Check if resolved path is within repository
	absRepoRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return valueobject.SymlinkScopeUnknown, err
	}

	absResolvedPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return valueobject.SymlinkScopeUnknown, err
	}

	// Check if the resolved path is within the repository
	relPath, err := filepath.Rel(absRepoRoot, absResolvedPath)
	if err != nil {
		return valueobject.SymlinkScopeExternal, err
	}

	// If the relative path starts with "..", it's outside the repository
	if strings.HasPrefix(relPath, "..") {
		return valueobject.SymlinkScopeExternal, nil
	}

	return valueobject.SymlinkScopeInternal, nil
}

// GetSymlinkType determines the type of the symlink target.
func (r *Resolver) GetSymlinkType(
	_ context.Context,
	symlink valueobject.SymlinkInfo,
) (valueobject.SymlinkType, error) {
	targetPath := symlink.TargetPath()
	return r.determineLinkType(targetPath), nil
}

// ResolveRelativeSymlink resolves a relative symbolic link within a repository context.
func (r *Resolver) ResolveRelativeSymlink(
	_ context.Context,
	symlinkPath string,
	_ string,
) (*valueobject.SymlinkInfo, error) {
	return r.ResolveSymlink(context.Background(), symlinkPath)
}

// ValidateSymlinkSecurity checks if following a symlink would violate security constraints.
func (r *Resolver) ValidateSymlinkSecurity(
	ctx context.Context,
	symlink valueobject.SymlinkInfo,
	repositoryRoot string,
) (*outbound.SymlinkSecurityResult, error) {
	validator := NewSecurityValidator(r)
	return validator.ValidateSymlinkSecurity(ctx, symlink, repositoryRoot)
}

// Helper methods

func (r *Resolver) resolveToAbsolutePath(symlinkPath, targetPath string) string {
	return r.pathUtils.ResolveRelativePath(symlinkPath, targetPath)
}

func (r *Resolver) determineLinkType(path string) valueobject.SymlinkType {
	return r.pathUtils.DetermineLinkType(path)
}

func (r *Resolver) determineScope(resolvedPath, symlinkPath string) valueobject.SymlinkScope {
	return r.pathUtils.DetermineScope(resolvedPath, symlinkPath)
}
