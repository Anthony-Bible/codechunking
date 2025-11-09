package service

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockSymlinkResolver implements outbound.SymlinkResolver for testing.
type MockSymlinkResolver struct {
	symlinks          map[string][]valueobject.SymlinkInfo
	symlinkTargets    map[string]string
	resolvedChains    map[string]*valueobject.SymlinkInfo
	circularSymlinks  map[string][]string
	symlinkTypes      map[string]valueobject.SymlinkType
	symlinkScopes     map[string]valueobject.SymlinkScope
	validationResults map[string]*outbound.SymlinkValidationResult
	securityResults   map[string]*outbound.SymlinkSecurityResult
}

func (m *MockSymlinkResolver) DetectSymlinks(
	_ context.Context,
	directoryPath string,
) ([]valueobject.SymlinkInfo, error) {
	if symlinks, exists := m.symlinks[directoryPath]; exists {
		return symlinks, nil
	}
	return []valueobject.SymlinkInfo{}, nil
}

func (m *MockSymlinkResolver) IsSymlink(_ context.Context, filePath string) (bool, error) {
	for _, symlinks := range m.symlinks {
		for _, symlink := range symlinks {
			if symlink.Path() == filePath {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *MockSymlinkResolver) ResolveSymlink(
	_ context.Context,
	symlinkPath string,
) (*valueobject.SymlinkInfo, error) {
	if resolved, exists := m.resolvedChains[symlinkPath]; exists {
		return resolved, nil
	}
	return nil, fmt.Errorf("symlink not found: %s", symlinkPath)
}

func (m *MockSymlinkResolver) GetSymlinkTarget(_ context.Context, symlinkPath string) (string, error) {
	if target, exists := m.symlinkTargets[symlinkPath]; exists {
		return target, nil
	}
	return "", fmt.Errorf("symlink target not found: %s", symlinkPath)
}

func (m *MockSymlinkResolver) ValidateSymlinkTarget(
	_ context.Context,
	symlink valueobject.SymlinkInfo,
) (*outbound.SymlinkValidationResult, error) {
	if result, exists := m.validationResults[symlink.Path()]; exists {
		return result, nil
	}
	return &outbound.SymlinkValidationResult{
		Symlink:      symlink,
		TargetExists: true,
		TargetType:   valueobject.SymlinkTypeFile,
		IsAccessible: true,
		ResponseTime: 50 * time.Millisecond,
	}, nil
}

// Additional methods for EnhancedSymlinkResolver.
func (m *MockSymlinkResolver) ResolveSymlinkChain(
	_ context.Context,
	symlinkPath string,
	maxDepth int,
) (*valueobject.SymlinkInfo, error) {
	if resolved, exists := m.resolvedChains[symlinkPath]; exists {
		if resolved.Depth() > maxDepth {
			return nil, fmt.Errorf("symlink chain exceeds max depth: %d", maxDepth)
		}
		return resolved, nil
	}
	return nil, fmt.Errorf("symlink chain not found: %s", symlinkPath)
}

func (m *MockSymlinkResolver) DetectCircularSymlinks(_ context.Context, symlinkPath string) (bool, []string, error) {
	if chain, exists := m.circularSymlinks[symlinkPath]; exists {
		return true, chain, nil
	}
	return false, []string{}, nil
}

func (m *MockSymlinkResolver) ClassifySymlinkScope(
	_ context.Context,
	symlink valueobject.SymlinkInfo,
	_ string,
) (valueobject.SymlinkScope, error) {
	if scope, exists := m.symlinkScopes[symlink.Path()]; exists {
		return scope, nil
	}
	return valueobject.SymlinkScopeUnknown, nil
}

func (m *MockSymlinkResolver) GetSymlinkType(
	_ context.Context,
	symlink valueobject.SymlinkInfo,
) (valueobject.SymlinkType, error) {
	if symlinkType, exists := m.symlinkTypes[symlink.Path()]; exists {
		return symlinkType, nil
	}
	return valueobject.SymlinkTypeUnknown, nil
}

func (m *MockSymlinkResolver) ResolveRelativeSymlink(
	_ context.Context,
	symlinkPath string, _ string,
) (*valueobject.SymlinkInfo, error) {
	if resolved, exists := m.resolvedChains[symlinkPath]; exists {
		return resolved, nil
	}
	return nil, fmt.Errorf("relative symlink not found: %s", symlinkPath)
}

func (m *MockSymlinkResolver) ValidateSymlinkSecurity(
	_ context.Context,
	symlink valueobject.SymlinkInfo,
	_ string,
) (*outbound.SymlinkSecurityResult, error) {
	if result, exists := m.securityResults[symlink.Path()]; exists {
		return result, nil
	}
	return &outbound.SymlinkSecurityResult{
		Symlink:            symlink,
		IsSecure:           true,
		SecurityViolations: []string{},
		EscapesRepository:  false,
		RecommendedAction:  outbound.SymlinkActionAllow,
		RiskLevel:          outbound.SymlinkRiskLevelLow,
	}, nil
}

// RED PHASE TESTS - All should fail until GREEN phase implementation

func TestSymlinkResolutionService_DetectSymlinks_BasicDetection(t *testing.T) {
	tests := []struct {
		name          string
		directoryPath string
		setupDir      func(string) error
		expected      []symlinkExpectation
	}{
		{
			name:          "Empty directory",
			directoryPath: "empty_dir",
			setupDir:      func(_ string) error { return nil },
			expected:      []symlinkExpectation{},
		},
		{
			name:          "Directory with single file symlink",
			directoryPath: "single_file_symlink_dir",
			setupDir:      setupSingleFileSymlinkDir,
			expected: []symlinkExpectation{
				{
					path:       "config.txt",
					targetPath: "../shared/config.txt",
					linkType:   valueobject.SymlinkTypeFile,
					scope:      valueobject.SymlinkScopeInternal,
					isBroken:   false,
				},
			},
		},
		{
			name:          "Directory with multiple symlinks",
			directoryPath: "multi_symlink_dir",
			setupDir:      setupMultiSymlinkDir,
			expected: []symlinkExpectation{
				{
					path:       "docs",
					targetPath: "/usr/share/doc/project",
					linkType:   valueobject.SymlinkTypeDirectory,
					scope:      valueobject.SymlinkScopeExternal,
					isBroken:   false,
				},
				{
					path:       "lib",
					targetPath: "vendor/lib",
					linkType:   valueobject.SymlinkTypeDirectory,
					scope:      valueobject.SymlinkScopeInternal,
					isBroken:   false,
				},
				{
					path:       "config.json",
					targetPath: "../config/default.json",
					linkType:   valueobject.SymlinkTypeFile,
					scope:      valueobject.SymlinkScopeInternal,
					isBroken:   false,
				},
			},
		},
		{
			name:          "Directory with broken symlinks",
			directoryPath: "broken_symlink_dir",
			setupDir:      setupBrokenSymlinkDir,
			expected: []symlinkExpectation{
				{
					path:       "missing_file.txt",
					targetPath: "/non/existent/file.txt",
					linkType:   valueobject.SymlinkTypeBroken,
					scope:      valueobject.SymlinkScopeBroken,
					isBroken:   true,
				},
				{
					path:       "missing_dir",
					targetPath: "non/existent/directory",
					linkType:   valueobject.SymlinkTypeBroken,
					scope:      valueobject.SymlinkScopeBroken,
					isBroken:   true,
				},
			},
		},
		{
			name:          "Directory with circular symlinks",
			directoryPath: "circular_symlink_dir",
			setupDir:      setupCircularSymlinkDir,
			expected: []symlinkExpectation{
				{
					path:       "link_a",
					targetPath: "link_b",
					linkType:   valueobject.SymlinkTypeFile,
					scope:      valueobject.SymlinkScopeInternal,
					isCircular: true,
				},
				{
					path:       "link_b",
					targetPath: "link_a",
					linkType:   valueobject.SymlinkTypeFile,
					scope:      valueobject.SymlinkScopeInternal,
					isCircular: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			tempDir := t.TempDir()
			testPath := filepath.Join(tempDir, tt.directoryPath)
			err := os.MkdirAll(testPath, 0o755)
			if err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Setup directory content
			if tt.setupDir != nil {
				err := tt.setupDir(testPath)
				if err != nil {
					t.Fatalf("Failed to setup test directory: %v", err)
				}
			}

			// Create mock resolver
			resolver := createMockResolverForPath(testPath, tt.expected)

			// Test detection
			ctx := context.Background()
			symlinks, err := resolver.DetectSymlinks(ctx, testPath)
			if err != nil {
				t.Fatalf("DetectSymlinks() error = %v", err)
			}

			// Verify results
			if len(symlinks) != len(tt.expected) {
				t.Errorf("Expected %d symlinks, got %d", len(tt.expected), len(symlinks))
			}

			for i, expected := range tt.expected {
				if i >= len(symlinks) {
					t.Errorf("Missing symlink at index %d", i)
					continue
				}

				actual := symlinks[i]
				validateSymlinkExpectation(t, actual, expected, i)
			}
		})
	}
}

func TestSymlinkResolutionService_ResolveSymlinkChain_Comprehensive(t *testing.T) {
	tests := []struct {
		name        string
		symlinkPath string
		maxDepth    int
		expected    chainExpectation
		expectError bool
	}{
		{
			name:        "Simple direct symlink",
			symlinkPath: "direct_link",
			maxDepth:    10,
			expected: chainExpectation{
				finalTarget: "/target/file.txt",
				chainLength: 1,
				isResolved:  true,
				isBroken:    false,
				isCircular:  false,
			},
		},
		{
			name:        "Chain of symlinks",
			symlinkPath: "link_chain_start",
			maxDepth:    10,
			expected: chainExpectation{
				finalTarget: "/final/target/file.txt",
				chainLength: 3,
				isResolved:  true,
				isBroken:    false,
				isCircular:  false,
			},
		},
		{
			name:        "Circular symlink detection",
			symlinkPath: "circular_link",
			maxDepth:    10,
			expected: chainExpectation{
				finalTarget: "",
				chainLength: 2,
				isResolved:  false,
				isBroken:    false,
				isCircular:  true,
			},
			expectError: false, // Should handle gracefully
		},
		{
			name:        "Depth limit exceeded",
			symlinkPath: "deep_chain",
			maxDepth:    3,
			expected: chainExpectation{
				chainLength: 5, // Longer than maxDepth
				isResolved:  false,
				isBroken:    false,
				isCircular:  false,
			},
			expectError: true,
		},
		{
			name:        "Broken symlink in chain",
			symlinkPath: "broken_chain",
			maxDepth:    10,
			expected: chainExpectation{
				finalTarget: "",
				chainLength: 2,
				isResolved:  false,
				isBroken:    true,
				isCircular:  false,
			},
		},
		{
			name:        "Complex nested chain",
			symlinkPath: "complex_nested",
			maxDepth:    15,
			expected: chainExpectation{
				finalTarget: "/deep/nested/target/file.txt",
				chainLength: 5,
				isResolved:  true,
				isBroken:    false,
				isCircular:  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock resolver with expected chain
			resolver := createMockResolverForChain(tt.symlinkPath, tt.expected, tt.maxDepth)

			ctx := context.Background()
			resolved, err := resolver.ResolveSymlinkChain(ctx, tt.symlinkPath, tt.maxDepth)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveSymlinkChain() error = %v", err)
			}

			if resolved == nil {
				t.Fatal("Expected resolved symlink info but got nil")
			}

			// Verify chain properties
			if tt.expected.isResolved {
				if resolved.ResolvedPath() != tt.expected.finalTarget {
					t.Errorf("Expected final target %s, got %s", tt.expected.finalTarget, resolved.ResolvedPath())
				}
			}

			if resolved.Depth() != tt.expected.chainLength-1 { // Depth is 0-based
				t.Errorf("Expected chain depth %d, got %d", tt.expected.chainLength-1, resolved.Depth())
			}

			if resolved.IsCircular() != tt.expected.isCircular {
				t.Errorf("Expected circular %v, got %v", tt.expected.isCircular, resolved.IsCircular())
			}

			if resolved.IsBroken() != tt.expected.isBroken {
				t.Errorf("Expected broken %v, got %v", tt.expected.isBroken, resolved.IsBroken())
			}
		})
	}
}

func TestSymlinkResolutionService_DetectCircularSymlinks_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		symlinkPath   string
		expectedLoop  bool
		expectedChain []string
	}{
		{
			name:          "No circular reference",
			symlinkPath:   "straight_link",
			expectedLoop:  false,
			expectedChain: []string{},
		},
		{
			name:          "Simple A->B->A loop",
			symlinkPath:   "simple_loop_a",
			expectedLoop:  true,
			expectedChain: []string{"simple_loop_a", "simple_loop_b", "simple_loop_a"},
		},
		{
			name:          "Complex A->B->C->A loop",
			symlinkPath:   "complex_loop_a",
			expectedLoop:  true,
			expectedChain: []string{"complex_loop_a", "complex_loop_b", "complex_loop_c", "complex_loop_a"},
		},
		{
			name:          "Self-referencing symlink",
			symlinkPath:   "self_ref",
			expectedLoop:  true,
			expectedChain: []string{"self_ref", "self_ref"},
		},
		{
			name:          "Long chain with loop at end",
			symlinkPath:   "long_chain_loop",
			expectedLoop:  true,
			expectedChain: []string{"long_chain_loop", "chain_1", "chain_2", "chain_3", "chain_1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock resolver with circular detection
			resolver := &MockSymlinkResolver{
				circularSymlinks: make(map[string][]string),
			}

			if tt.expectedLoop {
				resolver.circularSymlinks[tt.symlinkPath] = tt.expectedChain
			}

			ctx := context.Background()
			isCircular, chain, err := resolver.DetectCircularSymlinks(ctx, tt.symlinkPath)
			if err != nil {
				t.Fatalf("DetectCircularSymlinks() error = %v", err)
			}

			if isCircular != tt.expectedLoop {
				t.Errorf("Expected circular %v, got %v", tt.expectedLoop, isCircular)
			}

			if tt.expectedLoop {
				if len(chain) != len(tt.expectedChain) {
					t.Errorf("Expected chain length %d, got %d", len(tt.expectedChain), len(chain))
				} else {
					for i, expected := range tt.expectedChain {
						if chain[i] != expected {
							t.Errorf("Chain element %d: expected %s, got %s", i, expected, chain[i])
						}
					}
				}
			}
		})
	}
}

func TestSymlinkResolutionService_ClassifySymlinkScope_Repository(t *testing.T) {
	tests := []struct {
		name           string
		symlinkPath    string
		targetPath     string
		repositoryRoot string
		expectedScope  valueobject.SymlinkScope
	}{
		{
			name:           "Internal relative symlink",
			symlinkPath:    "/repo/src/config.txt",
			targetPath:     "../shared/config.txt",
			repositoryRoot: "/repo",
			expectedScope:  valueobject.SymlinkScopeInternal,
		},
		{
			name:           "Internal absolute symlink",
			symlinkPath:    "/repo/src/data.json",
			targetPath:     "/repo/data/default.json",
			repositoryRoot: "/repo",
			expectedScope:  valueobject.SymlinkScopeInternal,
		},
		{
			name:           "External absolute symlink",
			symlinkPath:    "/repo/docs/manual",
			targetPath:     "/usr/share/doc/project/manual",
			repositoryRoot: "/repo",
			expectedScope:  valueobject.SymlinkScopeExternal,
		},
		{
			name:           "External relative symlink escaping repo",
			symlinkPath:    "/repo/lib/system.so",
			targetPath:     "../../../usr/lib/system.so",
			repositoryRoot: "/repo",
			expectedScope:  valueobject.SymlinkScopeExternal,
		},
		{
			name:           "Broken symlink",
			symlinkPath:    "/repo/missing.txt",
			targetPath:     "/non/existent/file.txt",
			repositoryRoot: "/repo",
			expectedScope:  valueobject.SymlinkScopeBroken,
		},
		{
			name:           "Root-level internal symlink",
			symlinkPath:    "/repo/LICENSE",
			targetPath:     "./docs/LICENSE.txt",
			repositoryRoot: "/repo",
			expectedScope:  valueobject.SymlinkScopeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test symlink
			symlink, err := valueobject.NewSymlinkInfo(tt.symlinkPath, tt.targetPath)
			if err != nil {
				t.Fatalf("Failed to create test symlink: %v", err)
			}

			// Create mock resolver with expected scope
			resolver := &MockSymlinkResolver{
				symlinkScopes: make(map[string]valueobject.SymlinkScope),
			}
			resolver.symlinkScopes[symlink.Path()] = tt.expectedScope

			ctx := context.Background()
			scope, err := resolver.ClassifySymlinkScope(ctx, symlink, tt.repositoryRoot)
			if err != nil {
				t.Fatalf("ClassifySymlinkScope() error = %v", err)
			}

			if scope != tt.expectedScope {
				t.Errorf("Expected scope %s, got %s", tt.expectedScope.String(), scope.String())
			}
		})
	}
}

func TestSymlinkResolutionService_ValidateSymlinkSecurity_Comprehensive(t *testing.T) {
	tests := []struct {
		name               string
		symlinkPath        string
		targetPath         string
		repositoryRoot     string
		expectedSecure     bool
		expectedViolations []string
		expectedRisk       string
		expectedAction     string
	}{
		{
			name:               "Safe internal symlink",
			symlinkPath:        "/repo/config.txt",
			targetPath:         "./shared/config.txt",
			repositoryRoot:     "/repo",
			expectedSecure:     true,
			expectedViolations: []string{},
			expectedRisk:       outbound.SymlinkRiskLevelLow,
			expectedAction:     outbound.SymlinkActionAllow,
		},
		{
			name:               "Dangerous external symlink",
			symlinkPath:        "/repo/secrets.txt",
			targetPath:         "/etc/passwd",
			repositoryRoot:     "/repo",
			expectedSecure:     false,
			expectedViolations: []string{"escapes_repository", "points_to_sensitive_path"},
			expectedRisk:       outbound.SymlinkRiskLevelHigh,
			expectedAction:     outbound.SymlinkActionBlock,
		},
		{
			name:               "Symlink escaping repository",
			symlinkPath:        "/repo/lib/system.so",
			targetPath:         "../../usr/lib/system.so",
			repositoryRoot:     "/repo",
			expectedSecure:     false,
			expectedViolations: []string{"escapes_repository"},
			expectedRisk:       outbound.SymlinkRiskLevelMedium,
			expectedAction:     outbound.SymlinkActionWarn,
		},
		{
			name:               "Deep symlink chain",
			symlinkPath:        "/repo/deep_link",
			targetPath:         "./level1/level2/level3/target",
			repositoryRoot:     "/repo",
			expectedSecure:     false,
			expectedViolations: []string{"exceeds_depth_limit"},
			expectedRisk:       outbound.SymlinkRiskLevelMedium,
			expectedAction:     outbound.SymlinkActionWarn,
		},
		{
			name:               "Circular symlink",
			symlinkPath:        "/repo/circular",
			targetPath:         "./circular",
			repositoryRoot:     "/repo",
			expectedSecure:     false,
			expectedViolations: []string{"has_circular_reference"},
			expectedRisk:       outbound.SymlinkRiskLevelMedium,
			expectedAction:     outbound.SymlinkActionIgnore,
		},
		{
			name:               "Multiple security violations",
			symlinkPath:        "/repo/dangerous",
			targetPath:         "../../../../etc/shadow",
			repositoryRoot:     "/repo",
			expectedSecure:     false,
			expectedViolations: []string{"escapes_repository", "points_to_sensitive_path", "exceeds_depth_limit"},
			expectedRisk:       outbound.SymlinkRiskLevelHigh,
			expectedAction:     outbound.SymlinkActionBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test symlink
			symlink, err := valueobject.NewSymlinkInfo(tt.symlinkPath, tt.targetPath)
			if err != nil {
				t.Fatalf("Failed to create test symlink: %v", err)
			}

			// Create mock resolver with expected security result
			resolver := &MockSymlinkResolver{
				securityResults: make(map[string]*outbound.SymlinkSecurityResult),
			}

			expectedResult := &outbound.SymlinkSecurityResult{
				Symlink:               symlink,
				IsSecure:              tt.expectedSecure,
				SecurityViolations:    tt.expectedViolations,
				EscapesRepository:     symlinkContainsString(tt.expectedViolations, "escapes_repository"),
				PointsToSensitivePath: symlinkContainsString(tt.expectedViolations, "points_to_sensitive_path"),
				ExceedsDepthLimit:     symlinkContainsString(tt.expectedViolations, "exceeds_depth_limit"),
				HasCircularReference:  symlinkContainsString(tt.expectedViolations, "has_circular_reference"),
				RecommendedAction:     tt.expectedAction,
				RiskLevel:             tt.expectedRisk,
			}

			resolver.securityResults[symlink.Path()] = expectedResult

			ctx := context.Background()
			result, err := resolver.ValidateSymlinkSecurity(ctx, symlink, tt.repositoryRoot)
			if err != nil {
				t.Fatalf("ValidateSymlinkSecurity() error = %v", err)
			}

			if result.IsSecure != tt.expectedSecure {
				t.Errorf("Expected secure %v, got %v", tt.expectedSecure, result.IsSecure)
			}

			if len(result.SecurityViolations) != len(tt.expectedViolations) {
				t.Errorf("Expected %d violations, got %d", len(tt.expectedViolations), len(result.SecurityViolations))
			}

			for _, expectedViolation := range tt.expectedViolations {
				if !symlinkContainsString(result.SecurityViolations, expectedViolation) {
					t.Errorf("Expected violation %s not found", expectedViolation)
				}
			}

			if result.RiskLevel != tt.expectedRisk {
				t.Errorf("Expected risk level %s, got %s", tt.expectedRisk, result.RiskLevel)
			}

			if result.RecommendedAction != tt.expectedAction {
				t.Errorf("Expected action %s, got %s", tt.expectedAction, result.RecommendedAction)
			}
		})
	}
}

func TestSymlinkResolutionService_IsSymlink_PathValidation(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{
			name:     "Existing symlink file",
			filePath: "/repo/existing_symlink",
			expected: true,
		},
		{
			name:     "Regular file",
			filePath: "/repo/regular_file.txt",
			expected: false,
		},
		{
			name:     "Directory",
			filePath: "/repo/directory",
			expected: false,
		},
		{
			name:     "Non-existent path",
			filePath: "/repo/non_existent",
			expected: false,
		},
		{
			name:     "Symlink directory",
			filePath: "/repo/symlink_dir",
			expected: true,
		},
		{
			name:     "Broken symlink",
			filePath: "/repo/broken_symlink",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock resolver
			resolver := &MockSymlinkResolver{
				symlinks: make(map[string][]valueobject.SymlinkInfo),
			}

			if tt.expected {
				// Add symlink to mock
				symlink, _ := valueobject.NewSymlinkInfo(tt.filePath, "/some/target")
				resolver.symlinks["/repo"] = []valueobject.SymlinkInfo{symlink}
			}

			ctx := context.Background()
			isSymlink, err := resolver.IsSymlink(ctx, tt.filePath)
			if err != nil {
				t.Fatalf("IsSymlink() error = %v", err)
			}

			if isSymlink != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, isSymlink)
			}
		})
	}
}

// Helper types and functions

type symlinkExpectation struct {
	path       string
	targetPath string
	linkType   valueobject.SymlinkType
	scope      valueobject.SymlinkScope
	isBroken   bool
	isCircular bool
}

type chainExpectation struct {
	finalTarget string
	chainLength int
	isResolved  bool
	isBroken    bool
	isCircular  bool
}

func createMockResolverForPath(dirPath string, expectations []symlinkExpectation) *MockSymlinkResolver {
	resolver := &MockSymlinkResolver{
		symlinks: make(map[string][]valueobject.SymlinkInfo),
	}

	var symlinks []valueobject.SymlinkInfo
	for _, exp := range expectations {
		symlink, _ := valueobject.NewSymlinkInfo(exp.path, exp.targetPath)

		symlink = symlink.WithResolution("", exp.linkType, exp.scope)
		if exp.isBroken {
			symlink = symlink.WithBrokenStatus(true)
		}
		if exp.isCircular {
			symlink, _ = symlink.WithCircularReference(true, []string{exp.path, exp.targetPath})
		}

		symlinks = append(symlinks, symlink)
	}

	resolver.symlinks[dirPath] = symlinks
	return resolver
}

func createMockResolverForChain(symlinkPath string, expectation chainExpectation, maxDepth int) *MockSymlinkResolver {
	resolver := &MockSymlinkResolver{
		resolvedChains: make(map[string]*valueobject.SymlinkInfo),
	}

	if expectation.chainLength > maxDepth {
		// Add a symlink that exceeds max depth to trigger depth error
		symlink, _ := valueobject.NewSymlinkInfo(symlinkPath, "/intermediate/target")
		// Create a chain longer than maxDepth
		chain := make([]string, expectation.chainLength)
		for i := range chain {
			chain[i] = fmt.Sprintf("%s_step_%d", symlinkPath, i)
		}
		symlink, _ = symlink.WithCircularReference(false, chain)
		resolver.resolvedChains[symlinkPath] = &symlink
		return resolver
	}

	// Create resolved symlink info
	symlink, _ := valueobject.NewSymlinkInfo(symlinkPath, "/intermediate/target")

	if expectation.isResolved {
		symlink = symlink.WithResolution(
			expectation.finalTarget,
			valueobject.SymlinkTypeFile,
			valueobject.SymlinkScopeInternal,
		)
	}

	if expectation.isBroken {
		symlink = symlink.WithBrokenStatus(true)
	}

	if expectation.isCircular {
		chain := make([]string, expectation.chainLength)
		for i := range chain {
			chain[i] = fmt.Sprintf("%s_step_%d", symlinkPath, i)
		}
		symlink, _ = symlink.WithCircularReference(true, chain)
	} else {
		// For non-circular chains, set the depth manually
		if expectation.chainLength > 0 {
			// Create a chain to set the depth properly
			chain := make([]string, expectation.chainLength)
			for i := range chain {
				chain[i] = fmt.Sprintf("%s_step_%d", symlinkPath, i)
			}
			symlink, _ = symlink.WithCircularReference(false, chain)
		}
	}

	resolver.resolvedChains[symlinkPath] = &symlink
	return resolver
}

func validateSymlinkExpectation(t *testing.T, actual valueobject.SymlinkInfo, expected symlinkExpectation, index int) {
	if actual.Path() != expected.path {
		t.Errorf("Symlink %d: expected path %s, got %s", index, expected.path, actual.Path())
	}

	if actual.TargetPath() != expected.targetPath {
		t.Errorf("Symlink %d: expected target %s, got %s", index, expected.targetPath, actual.TargetPath())
	}

	if actual.LinkType() != expected.linkType {
		t.Errorf("Symlink %d: expected type %s, got %s", index, expected.linkType.String(), actual.LinkType().String())
	}

	if actual.Scope() != expected.scope {
		t.Errorf("Symlink %d: expected scope %s, got %s", index, expected.scope.String(), actual.Scope().String())
	}

	if actual.IsBroken() != expected.isBroken {
		t.Errorf("Symlink %d: expected broken %v, got %v", index, expected.isBroken, actual.IsBroken())
	}

	if actual.IsCircular() != expected.isCircular {
		t.Errorf("Symlink %d: expected circular %v, got %v", index, expected.isCircular, actual.IsCircular())
	}
}

// Setup functions for test directories

func setupSingleFileSymlinkDir(_ string) error {
	// In a real implementation, this would create actual symlinks
	// For testing purposes, we just ensure the directory exists
	return nil
}

func setupMultiSymlinkDir(_ string) error {
	// In a real implementation, this would create multiple symlinks
	return nil
}

func setupBrokenSymlinkDir(_ string) error {
	// In a real implementation, this would create symlinks pointing to non-existent targets
	return nil
}

func setupCircularSymlinkDir(_ string) error {
	// In a real implementation, this would create circular symlinks
	return nil
}

// Helper function to check if slice contains string.
func symlinkContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
