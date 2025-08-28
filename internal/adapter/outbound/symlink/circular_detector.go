package symlink

import (
	"context"
)

// CircularDetector handles the logic for detecting circular symlink references.
// It provides functionality to detect circular references in symbolic link chains
// and maintain the chain of visited paths.
type CircularDetector struct {
	resolver  *Resolver
	visited   map[string]bool
	chain     []string
	pathUtils *PathUtils
	mapUtils  *MapUtils
}

// NewCircularDetector creates a new circular detector instance.
func NewCircularDetector(resolver *Resolver) *CircularDetector {
	return &CircularDetector{
		resolver:  resolver,
		visited:   make(map[string]bool),
		chain:     []string{},
		pathUtils: NewPathUtils(),
		mapUtils:  NewMapUtils(),
	}
}

// DetectCircular processes the circular symlink detection starting from the given path.
func (cd *CircularDetector) DetectCircular(
	ctx context.Context,
	symlinkPath string,
) (bool, []string, error) {
	// Reset state for new detection
	cd.reset()

	currentPath := symlinkPath

	for {
		if cd.visited[currentPath] {
			return cd.handleCircularReference(currentPath)
		}

		cd.visited[currentPath] = true
		cd.chain = append(cd.chain, currentPath)

		isSymlink, err := cd.resolver.IsSymlink(ctx, currentPath)
		if err != nil {
			return false, []string{}, err
		}
		if !isSymlink {
			break
		}

		nextPath, err := cd.getNextPath(ctx, currentPath)
		if err != nil {
			return false, []string{}, err
		}

		currentPath = nextPath
	}

	return false, []string{}, nil
}

// handleCircularReference processes when a circular reference is found.
func (cd *CircularDetector) handleCircularReference(
	currentPath string,
) (bool, []string, error) {
	cycleStart := cd.findCycleStart(currentPath)
	if cycleStart >= 0 {
		cycleChain := make([]string, 0, len(cd.chain[cycleStart:])+1)
		cycleChain = append(cycleChain, cd.chain[cycleStart:]...)
		cycleChain = append(cycleChain, currentPath)
		return true, cycleChain, nil
	}
	return true, append(cd.chain, currentPath), nil
}

// findCycleStart finds where the cycle starts in the chain.
func (cd *CircularDetector) findCycleStart(currentPath string) int {
	for i, path := range cd.chain {
		if path == currentPath {
			return i
		}
	}
	return -1
}

// getNextPath gets the next path in the symlink chain.
func (cd *CircularDetector) getNextPath(
	ctx context.Context,
	currentPath string,
) (string, error) {
	targetPath, err := cd.resolver.GetSymlinkTarget(ctx, currentPath)
	if err != nil {
		return "", err
	}

	return cd.resolveRelativePath(currentPath, targetPath), nil
}

// IsVisited checks if a path has already been visited during detection.
func (cd *CircularDetector) IsVisited(path string) bool {
	return cd.visited[path]
}

// GetChain returns a copy of the current detection chain.
func (cd *CircularDetector) GetChain() []string {
	return cd.mapUtils.CopyStringSlice(cd.chain)
}

// GetVisitedCount returns the number of visited paths.
func (cd *CircularDetector) GetVisitedCount() int {
	return len(cd.visited)
}

// HasCycle checks if there's a cycle starting from the given path without full detection.
func (cd *CircularDetector) HasCycle(path string) bool {
	return cd.visited[path]
}

// reset clears the detector state for reuse.
func (cd *CircularDetector) reset() {
	// Clear visited map efficiently
	cd.mapUtils.ClearStringBoolMap(cd.visited)
	// Reset chain slice
	cd.chain = cd.chain[:0]
}

// GetCycleInfo provides detailed information about a detected cycle.
func (cd *CircularDetector) GetCycleInfo(cyclePath string) *CycleInfo {
	cycleStart := cd.findCycleStart(cyclePath)
	if cycleStart < 0 {
		return nil
	}

	cycleLength := len(cd.chain) - cycleStart
	return &CycleInfo{
		StartIndex:  cycleStart,
		CycleLength: cycleLength,
		CyclePath:   cyclePath,
		FullChain:   cd.GetChain(),
	}
}

// CycleInfo contains information about a detected circular reference.
type CycleInfo struct {
	StartIndex  int      // Index where the cycle begins
	CycleLength int      // Length of the cycle
	CyclePath   string   // Path that creates the cycle
	FullChain   []string // Complete chain including the cycle
}

// Private helper methods

// resolveRelativePath resolves a relative path against a base path.
func (cd *CircularDetector) resolveRelativePath(basePath, targetPath string) string {
	return cd.pathUtils.ResolveRelativePath(basePath, targetPath)
}
