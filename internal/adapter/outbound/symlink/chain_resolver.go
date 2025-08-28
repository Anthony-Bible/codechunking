package symlink

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"fmt"
)

// ChainResolver handles the complex logic of resolving symlink chains.
// It provides functionality to resolve chains of symbolic links while detecting
// circular references and enforcing depth limits.
type ChainResolver struct {
	resolver  *Resolver
	visited   map[string]bool
	chain     []string
	pathUtils *PathUtils
	mapUtils  *MapUtils
}

// NewChainResolver creates a new chain resolver instance.
func NewChainResolver(resolver *Resolver, initialPath string) *ChainResolver {
	return &ChainResolver{
		resolver:  resolver,
		visited:   make(map[string]bool),
		chain:     []string{initialPath},
		pathUtils: NewPathUtils(),
		mapUtils:  NewMapUtils(),
	}
}

// ResolveChain processes the symlink chain resolution with circular detection.
func (cr *ChainResolver) ResolveChain(
	ctx context.Context,
	symlinkPath string,
	maxDepth int,
) (*valueobject.SymlinkInfo, error) {
	currentPath := symlinkPath
	depth := 0

	for depth < maxDepth {
		if cr.visited[currentPath] {
			return cr.handleCircularReference(symlinkPath, currentPath)
		}

		cr.visited[currentPath] = true

		isSymlink, err := cr.resolver.IsSymlink(ctx, currentPath)
		if err != nil || !isSymlink {
			break
		}

		nextPath, err := cr.getNextPathInChain(ctx, currentPath)
		if err != nil {
			break
		}

		cr.chain = append(cr.chain, nextPath)
		currentPath = nextPath
		depth++
	}

	if depth >= maxDepth {
		return nil, fmt.Errorf("symlink chain exceeds max depth: %d", maxDepth)
	}

	return cr.createFinalSymlinkInfo(ctx, symlinkPath, currentPath)
}

// handleCircularReference creates a symlink info for circular references.
func (cr *ChainResolver) handleCircularReference(
	symlinkPath, currentPath string,
) (*valueobject.SymlinkInfo, error) {
	symlink, err := valueobject.NewSymlinkInfo(symlinkPath, currentPath)
	if err != nil {
		return nil, err
	}

	symlink, err = symlink.WithCircularReference(true, cr.chain)
	if err != nil {
		return nil, err
	}

	return &symlink, nil
}

// getNextPathInChain gets the next path in the symlink chain.
func (cr *ChainResolver) getNextPathInChain(
	ctx context.Context,
	currentPath string,
) (string, error) {
	targetPath, err := cr.resolver.GetSymlinkTarget(ctx, currentPath)
	if err != nil {
		return "", err
	}

	return cr.resolveRelativePath(currentPath, targetPath), nil
}

// createFinalSymlinkInfo creates the final symlink info with all resolution details.
func (cr *ChainResolver) createFinalSymlinkInfo(
	ctx context.Context,
	symlinkPath, currentPath string,
) (*valueobject.SymlinkInfo, error) {
	originalTarget, _ := cr.resolver.GetSymlinkTarget(ctx, symlinkPath)
	symlink, err := valueobject.NewSymlinkInfo(symlinkPath, originalTarget)
	if err != nil {
		return nil, err
	}

	// Set resolution information
	linkType := cr.determineLinkType(currentPath)
	scope := cr.determineScope(currentPath, symlinkPath)
	symlink = symlink.WithResolution(currentPath, linkType, scope)

	// Set chain information if we have multiple links
	if len(cr.chain) > 1 {
		symlink, err = symlink.WithCircularReference(false, cr.chain)
		if err != nil {
			return nil, err
		}
	}

	return &symlink, nil
}

// GetChainLength returns the current length of the resolution chain.
func (cr *ChainResolver) GetChainLength() int {
	return len(cr.chain)
}

// GetVisitedPaths returns a copy of the visited paths map.
func (cr *ChainResolver) GetVisitedPaths() map[string]bool {
	visited := make(map[string]bool)
	for k, v := range cr.visited {
		visited[k] = v
	}
	return visited
}

// Reset clears the resolver state for reuse.
func (cr *ChainResolver) Reset(initialPath string) {
	cr.mapUtils.ClearStringBoolMap(cr.visited)
	cr.chain = cr.chain[:0]
	cr.chain = append(cr.chain, initialPath)
}

// Private helper methods

// resolveRelativePath resolves a relative path against a base path.
func (cr *ChainResolver) resolveRelativePath(basePath, targetPath string) string {
	return cr.pathUtils.ResolveRelativePath(basePath, targetPath)
}

// determineLinkType determines the type of the symlink target.
func (cr *ChainResolver) determineLinkType(path string) valueobject.SymlinkType {
	return cr.pathUtils.DetermineLinkType(path)
}

// determineScope determines if the symlink is internal or external.
func (cr *ChainResolver) determineScope(resolvedPath, symlinkPath string) valueobject.SymlinkScope {
	return cr.pathUtils.DetermineScope(resolvedPath, symlinkPath)
}
