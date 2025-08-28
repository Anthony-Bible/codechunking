package service

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// IntegratedFileProcessor combines file filtering with submodule and symlink handling.
type IntegratedFileProcessor struct {
	fileFilter        outbound.FileFilter
	submoduleDetector outbound.SubmoduleDetector
	symlinkResolver   outbound.SymlinkResolver
}

// ProcessingResult represents the integrated processing result.
type ProcessingResult struct {
	FileInfo           outbound.FileInfo          `json:"file_info"`
	FilterDecision     outbound.FilterDecision    `json:"filter_decision"`
	SubmoduleInfo      *valueobject.SubmoduleInfo `json:"submodule_info,omitempty"`
	SymlinkInfo        *valueobject.SymlinkInfo   `json:"symlink_info,omitempty"`
	ProcessingPath     string                     `json:"processing_path"`
	IsInSubmodule      bool                       `json:"is_in_submodule"`
	IsSymlink          bool                       `json:"is_symlink"`
	FinalShouldProcess bool                       `json:"final_should_process"`
	ProcessingTime     time.Duration              `json:"processing_time"`
	Errors             []string                   `json:"errors,omitempty"`
	Metadata           map[string]interface{}     `json:"metadata,omitempty"`
}

// NewIntegratedFileProcessor creates a new integrated file processor.
func NewIntegratedFileProcessor(
	fileFilter outbound.FileFilter,
	submoduleDetector outbound.SubmoduleDetector,
	symlinkResolver outbound.SymlinkResolver,
) *IntegratedFileProcessor {
	return &IntegratedFileProcessor{
		fileFilter:        fileFilter,
		submoduleDetector: submoduleDetector,
		symlinkResolver:   symlinkResolver,
	}
}

// ProcessFile processes a file through all integrated filters and handlers.
func (p *IntegratedFileProcessor) ProcessFile(
	ctx context.Context,
	filePath string,
	repositoryRoot string,
	fileInfo outbound.FileInfo,
) (*ProcessingResult, error) {
	start := time.Now()

	result := &ProcessingResult{
		FileInfo:       fileInfo,
		ProcessingPath: filePath,
		Metadata:       make(map[string]interface{}),
		Errors:         []string{},
	}

	// 1. Check if file is in a submodule
	isInSubmodule, submoduleInfo, err := p.submoduleDetector.IsSubmoduleDirectory(
		ctx,
		filepath.Dir(filePath),
		repositoryRoot,
	)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("submodule detection error: %v", err))
	} else {
		result.IsInSubmodule = isInSubmodule
		if isInSubmodule && submoduleInfo != nil {
			result.SubmoduleInfo = submoduleInfo
			result.Metadata["submodule_path"] = submoduleInfo.Path()
			result.Metadata["submodule_url"] = submoduleInfo.URL()
		}
	}

	// 2. Check if file is a symlink
	isSymlink, err := p.symlinkResolver.IsSymlink(ctx, filePath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("symlink detection error: %v", err))
	} else {
		result.IsSymlink = isSymlink
		if isSymlink {
			symlinkInfo, err := p.symlinkResolver.ResolveSymlink(ctx, filePath)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("symlink resolution error: %v", err))
			} else {
				result.SymlinkInfo = symlinkInfo
				result.ProcessingPath = symlinkInfo.ResolvedPath()
				result.Metadata["symlink_target"] = symlinkInfo.TargetPath()
				result.Metadata["symlink_scope"] = symlinkInfo.Scope().String()
			}
		}
	}

	// 3. Apply file filtering
	filterDecision, err := p.fileFilter.ShouldProcessFile(ctx, result.ProcessingPath, fileInfo)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("file filter error: %v", err))
		result.FinalShouldProcess = false
	} else {
		result.FilterDecision = filterDecision
		result.FinalShouldProcess = filterDecision.ShouldProcess
	}

	// 4. Apply integrated processing rules
	result.FinalShouldProcess = p.applyIntegratedRules(result)
	result.ProcessingTime = time.Since(start)

	return result, nil
}

// applyIntegratedRules applies rules that combine file filtering with submodule/symlink handling.
func (p *IntegratedFileProcessor) applyIntegratedRules(result *ProcessingResult) bool {
	// Don't process if basic filter says no
	if !result.FilterDecision.ShouldProcess {
		return false
	}

	// Don't process broken or circular symlinks
	if result.IsSymlink && result.SymlinkInfo != nil {
		if result.SymlinkInfo.IsBroken() || result.SymlinkInfo.IsCircular() {
			result.Metadata["skip_reason"] = "broken or circular symlink"
			return false
		}

		// Don't process symlinks pointing outside repository
		if result.SymlinkInfo.Scope() == valueobject.SymlinkScopeExternal {
			result.Metadata["skip_reason"] = "external symlink"
			return false
		}
	}

	// Handle submodule processing policy
	if result.IsInSubmodule && result.SubmoduleInfo != nil {
		// This would use actual policy configuration in real implementation
		policy := valueobject.SubmoduleUpdatePolicyShallow
		if !result.SubmoduleInfo.ShouldProcess(policy) {
			result.Metadata["skip_reason"] = "submodule policy exclusion"
			return false
		}
	}

	return true
}

// RED PHASE INTEGRATION TESTS - All should fail until GREEN phase implementation

func TestFileProcessingIntegration_BasicIntegration(t *testing.T) {
	tests := []struct {
		name              string
		setupRepo         func(string) error
		filePath          string
		expectedProcess   bool
		expectedInSubmod  bool
		expectedIsSymlink bool
		expectedErrors    int
	}{
		{
			name:              "Regular file in regular directory",
			setupRepo:         setupRegularRepo,
			filePath:          "src/main.go",
			expectedProcess:   true,
			expectedInSubmod:  false,
			expectedIsSymlink: false,
			expectedErrors:    0,
		},
		{
			name:              "File in submodule directory",
			setupRepo:         setupRepoWithSubmodules,
			filePath:          "libs/common/utils.go",
			expectedProcess:   true,
			expectedInSubmod:  true,
			expectedIsSymlink: false,
			expectedErrors:    0,
		},
		{
			name:              "Symlink to internal file",
			setupRepo:         setupRepoWithSymlinks,
			filePath:          "config.json",
			expectedProcess:   true,
			expectedInSubmod:  false,
			expectedIsSymlink: true,
			expectedErrors:    0,
		},
		{
			name:              "Symlink to external file",
			setupRepo:         setupRepoWithExternalSymlinks,
			filePath:          "external_config.json",
			expectedProcess:   false, // Should be filtered out
			expectedInSubmod:  false,
			expectedIsSymlink: true,
			expectedErrors:    0,
		},
		{
			name:              "File in submodule with symlink",
			setupRepo:         setupRepoWithSubmoduleSymlinks,
			filePath:          "libs/common/config.json",
			expectedProcess:   true,
			expectedInSubmod:  true,
			expectedIsSymlink: true,
			expectedErrors:    0,
		},
		{
			name:              "Broken symlink",
			setupRepo:         setupRepoWithBrokenSymlinks,
			filePath:          "broken_link.txt",
			expectedProcess:   false, // Should be filtered out
			expectedInSubmod:  false,
			expectedIsSymlink: true,
			expectedErrors:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test repository
			tempDir := t.TempDir()
			err := tt.setupRepo(tempDir)
			if err != nil {
				t.Fatalf("Failed to setup test repo: %v", err)
			}

			// Create integrated processor with mocks
			processor := createIntegratedProcessor(tempDir)

			// Create file info
			fileInfo := outbound.FileInfo{
				Path:        tt.filePath,
				Size:        1024,
				IsDirectory: false,
				ModTime:     time.Now(),
			}

			// Process file
			ctx := context.Background()
			result, err := processor.ProcessFile(ctx, tt.filePath, tempDir, fileInfo)
			if err != nil {
				t.Fatalf("ProcessFile() error = %v", err)
			}

			// Verify results
			if result.FinalShouldProcess != tt.expectedProcess {
				t.Errorf("Expected final process decision %v, got %v", tt.expectedProcess, result.FinalShouldProcess)
			}

			if result.IsInSubmodule != tt.expectedInSubmod {
				t.Errorf("Expected in submodule %v, got %v", tt.expectedInSubmod, result.IsInSubmodule)
			}

			if result.IsSymlink != tt.expectedIsSymlink {
				t.Errorf("Expected is symlink %v, got %v", tt.expectedIsSymlink, result.IsSymlink)
			}

			if len(result.Errors) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectedErrors, len(result.Errors), result.Errors)
			}

			// Verify metadata is populated appropriately
			validateProcessingMetadata(t, result)
		})
	}
}

func TestFileProcessingIntegration_SubmodulePolicyIntegration(t *testing.T) {
	tests := []struct {
		name               string
		submoduleDepth     int
		submodulePolicy    valueobject.SubmoduleUpdatePolicy
		expectedProcess    bool
		expectedSkipReason string
	}{
		{
			name:            "Shallow policy - depth 0",
			submoduleDepth:  0,
			submodulePolicy: valueobject.SubmoduleUpdatePolicyShallow,
			expectedProcess: true,
		},
		{
			name:               "Shallow policy - depth 1",
			submoduleDepth:     1,
			submodulePolicy:    valueobject.SubmoduleUpdatePolicyShallow,
			expectedProcess:    false,
			expectedSkipReason: "submodule policy exclusion",
		},
		{
			name:            "Recursive policy - depth 1",
			submoduleDepth:  1,
			submodulePolicy: valueobject.SubmoduleUpdatePolicyRecursive,
			expectedProcess: true,
		},
		{
			name:               "Ignore policy - depth 0",
			submoduleDepth:     0,
			submodulePolicy:    valueobject.SubmoduleUpdatePolicyIgnore,
			expectedProcess:    false,
			expectedSkipReason: "submodule policy exclusion",
		},
		{
			name:            "Follow parent policy - depth 1",
			submoduleDepth:  1,
			submodulePolicy: valueobject.SubmoduleUpdatePolicyFollowParent,
			expectedProcess: true, // Depth 1 is within limit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test setup
			tempDir := t.TempDir()
			processor := createIntegratedProcessor(tempDir)

			// Create submodule info with specific depth
			submoduleInfo, err := valueobject.NewSubmoduleInfo(
				"libs/module",
				"module",
				"https://github.com/example/module.git",
			)
			if err != nil {
				t.Fatalf("Failed to create submodule info: %v", err)
			}

			// Set nesting information
			isNested := tt.submoduleDepth > 0
			parentPath := ""
			if isNested {
				parentPath = "parent/module"
			}
			submoduleInfo, err = submoduleInfo.WithNesting(isNested, tt.submoduleDepth, parentPath)
			if err != nil {
				t.Fatalf("Failed to set submodule nesting: %v", err)
			}

			// Create processing result with submodule
			result := &ProcessingResult{
				FileInfo: outbound.FileInfo{
					Path: "libs/module/file.go",
					Size: 1024,
				},
				FilterDecision: outbound.FilterDecision{
					ShouldProcess: true,
				},
				IsInSubmodule: true,
				SubmoduleInfo: &submoduleInfo,
				Metadata:      make(map[string]interface{}),
			}

			// Apply integrated rules
			finalDecision := processor.applyIntegratedRules(result)

			if finalDecision != tt.expectedProcess {
				t.Errorf("Expected process decision %v, got %v", tt.expectedProcess, finalDecision)
			}

			if tt.expectedSkipReason != "" {
				skipReason, exists := result.Metadata["skip_reason"]
				if !exists {
					t.Error("Expected skip reason in metadata but found none")
				} else if skipReason != tt.expectedSkipReason {
					t.Errorf("Expected skip reason %q, got %q", tt.expectedSkipReason, skipReason)
				}
			}
		})
	}
}

func TestFileProcessingIntegration_SymlinkSecurityIntegration(t *testing.T) {
	tests := []struct {
		name               string
		symlinkPath        string
		targetPath         string
		scope              valueobject.SymlinkScope
		isBroken           bool
		isCircular         bool
		expectedProcess    bool
		expectedSkipReason string
	}{
		{
			name:            "Safe internal symlink",
			symlinkPath:     "config.json",
			targetPath:      "./shared/config.json",
			scope:           valueobject.SymlinkScopeInternal,
			isBroken:        false,
			isCircular:      false,
			expectedProcess: true,
		},
		{
			name:               "External symlink",
			symlinkPath:        "system_config.json",
			targetPath:         "/etc/config.json",
			scope:              valueobject.SymlinkScopeExternal,
			isBroken:           false,
			isCircular:         false,
			expectedProcess:    false,
			expectedSkipReason: "external symlink",
		},
		{
			name:               "Broken symlink",
			symlinkPath:        "missing_file.txt",
			targetPath:         "./non/existent.txt",
			scope:              valueobject.SymlinkScopeBroken,
			isBroken:           true,
			isCircular:         false,
			expectedProcess:    false,
			expectedSkipReason: "broken or circular symlink",
		},
		{
			name:               "Circular symlink",
			symlinkPath:        "circular.txt",
			targetPath:         "./circular.txt",
			scope:              valueobject.SymlinkScopeInternal,
			isBroken:           false,
			isCircular:         true,
			expectedProcess:    false,
			expectedSkipReason: "broken or circular symlink",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test setup
			tempDir := t.TempDir()
			processor := createIntegratedProcessor(tempDir)

			// Create symlink info
			symlinkInfo, err := valueobject.NewSymlinkInfo(tt.symlinkPath, tt.targetPath)
			if err != nil {
				t.Fatalf("Failed to create symlink info: %v", err)
			}

			// Configure symlink properties
			symlinkInfo = symlinkInfo.WithResolution("", valueobject.SymlinkTypeFile, tt.scope)
			if tt.isBroken {
				symlinkInfo = symlinkInfo.WithBrokenStatus(true)
			}
			if tt.isCircular {
				symlinkInfo, err = symlinkInfo.WithCircularReference(true, []string{tt.symlinkPath, tt.targetPath})
				if err != nil {
					t.Fatalf("Failed to set circular reference: %v", err)
				}
			}

			// Create processing result with symlink
			result := &ProcessingResult{
				FileInfo: outbound.FileInfo{
					Path: tt.symlinkPath,
					Size: 1024,
				},
				FilterDecision: outbound.FilterDecision{
					ShouldProcess: true,
				},
				IsSymlink:   true,
				SymlinkInfo: &symlinkInfo,
				Metadata:    make(map[string]interface{}),
			}

			// Apply integrated rules
			finalDecision := processor.applyIntegratedRules(result)

			if finalDecision != tt.expectedProcess {
				t.Errorf("Expected process decision %v, got %v", tt.expectedProcess, finalDecision)
			}

			if tt.expectedSkipReason != "" {
				skipReason, exists := result.Metadata["skip_reason"]
				if !exists {
					t.Error("Expected skip reason in metadata but found none")
				} else if skipReason != tt.expectedSkipReason {
					t.Errorf("Expected skip reason %q, got %q", tt.expectedSkipReason, skipReason)
				}
			}
		})
	}
}

func TestFileProcessingIntegration_BatchProcessing(t *testing.T) {
	// Test batch processing with mixed file types
	tempDir := t.TempDir()
	processor := createIntegratedProcessor(tempDir)

	// Create diverse set of files
	testFiles := []struct {
		path          string
		isInSubmodule bool
		isSymlink     bool
		shouldProcess bool
	}{
		{"src/main.go", false, false, true},
		{"libs/common/utils.go", true, false, true},
		{"config.json", false, true, true},
		{"external_link.txt", false, true, false},        // External symlink
		{"broken_link.txt", false, true, false},          // Broken symlink
		{"libs/nested/deep/file.go", true, false, false}, // Deep submodule
		{"vendor/lib/binary.so", false, false, false},    // Binary file
	}

	ctx := context.Background()
	var results []*ProcessingResult

	for _, testFile := range testFiles {
		fileInfo := outbound.FileInfo{
			Path:        testFile.path,
			Size:        1024,
			IsDirectory: false,
			ModTime:     time.Now(),
		}

		result, err := processor.ProcessFile(ctx, testFile.path, tempDir, fileInfo)
		if err != nil {
			t.Errorf("ProcessFile(%s) error = %v", testFile.path, err)
			continue
		}

		results = append(results, result)

		// Verify individual results
		if result.FinalShouldProcess != testFile.shouldProcess {
			t.Errorf(
				"File %s: expected process %v, got %v",
				testFile.path,
				testFile.shouldProcess,
				result.FinalShouldProcess,
			)
		}
	}

	// Verify batch statistics
	processedCount := 0
	submoduleCount := 0
	symlinkCount := 0

	for _, result := range results {
		if result.FinalShouldProcess {
			processedCount++
		}
		if result.IsInSubmodule {
			submoduleCount++
		}
		if result.IsSymlink {
			symlinkCount++
		}
	}

	expectedProcessed := 3  // main.go, utils.go, config.json
	expectedSubmodules := 3 // utils.go, deep/file.go, and nested file
	expectedSymlinks := 3   // config.json, external_link.txt, broken_link.txt

	if processedCount != expectedProcessed {
		t.Errorf("Expected %d processed files, got %d", expectedProcessed, processedCount)
	}

	if submoduleCount != expectedSubmodules {
		t.Errorf("Expected %d submodule files, got %d", expectedSubmodules, submoduleCount)
	}

	if symlinkCount != expectedSymlinks {
		t.Errorf("Expected %d symlink files, got %d", expectedSymlinks, symlinkCount)
	}
}

func TestFileProcessingIntegration_ErrorHandling(t *testing.T) {
	// Test comprehensive error handling scenarios
	tests := []struct {
		name           string
		setupError     string
		expectedErrors int
		shouldContinue bool
	}{
		{
			name:           "Submodule detection failure",
			setupError:     "submodule_error",
			expectedErrors: 1,
			shouldContinue: true, // Should continue with other processing
		},
		{
			name:           "Symlink resolution failure",
			setupError:     "symlink_error",
			expectedErrors: 1,
			shouldContinue: true,
		},
		{
			name:           "File filter failure",
			setupError:     "filter_error",
			expectedErrors: 1,
			shouldContinue: false, // Cannot continue without filter decision
		},
		{
			name:           "Multiple component failures",
			setupError:     "multiple_errors",
			expectedErrors: 3,
			shouldContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create processor with error-inducing setup
			tempDir := t.TempDir()
			processor := createIntegratedProcessorWithErrors(tempDir, tt.setupError)

			fileInfo := outbound.FileInfo{
				Path: "test_file.go",
				Size: 1024,
			}

			ctx := context.Background()
			result, err := processor.ProcessFile(ctx, "test_file.go", tempDir, fileInfo)
			// Should not return error from ProcessFile itself
			if err != nil {
				t.Fatalf("ProcessFile() should not return error, got: %v", err)
			}

			// Check error count
			if len(result.Errors) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectedErrors, len(result.Errors), result.Errors)
			}

			// Check processing continuation
			if tt.shouldContinue && result.ProcessingTime == 0 {
				t.Error("Expected processing to continue but it appears to have stopped")
			}
		})
	}
}

// Helper functions

func createIntegratedProcessor(repoPath string) *IntegratedFileProcessor {
	// Create mock components
	fileFilter := &testFileFilter{}
	submoduleDetector := &MockSubmoduleDetector{
		submodules:        make(map[string][]valueobject.SubmoduleInfo),
		submoduleStatuses: make(map[string]valueobject.SubmoduleStatus),
		activeSubmodules:  make(map[string]bool),
	}
	symlinkResolver := &MockSymlinkResolver{
		symlinks:       make(map[string][]valueobject.SymlinkInfo),
		resolvedChains: make(map[string]*valueobject.SymlinkInfo),
		symlinkScopes:  make(map[string]valueobject.SymlinkScope),
	}

	// Setup mock data based on repo path patterns
	setupMockData(repoPath, fileFilter, submoduleDetector, symlinkResolver)

	return NewIntegratedFileProcessor(fileFilter, submoduleDetector, symlinkResolver)
}

func createIntegratedProcessorWithErrors(repoPath string, errorType string) *IntegratedFileProcessor {
	// Create error-inducing mocks
	var fileFilter outbound.FileFilter = &errorFileFilter{errorType: errorType}
	var submoduleDetector outbound.SubmoduleDetector = &errorSubmoduleDetector{errorType: errorType}
	var symlinkResolver outbound.SymlinkResolver = &errorSymlinkResolver{errorType: errorType}

	return NewIntegratedFileProcessor(fileFilter, submoduleDetector, symlinkResolver)
}

func setupMockData(
	repoPath string,
	filter *testFileFilter,
	detector *MockSubmoduleDetector,
	resolver *MockSymlinkResolver,
) {
	// Setup submodule data
	commonSubmodule, _ := valueobject.NewSubmoduleInfo("libs/common", "common", "https://github.com/example/common.git")
	detector.submodules[repoPath] = []valueobject.SubmoduleInfo{commonSubmodule}

	key := fmt.Sprintf("%s:libs/common", repoPath)
	detector.activeSubmodules[key] = true

	// Setup symlink data
	internalSymlink, _ := valueobject.NewSymlinkInfo("config.json", "./shared/config.json")
	internalSymlink = internalSymlink.WithResolution(
		"/repo/shared/config.json",
		valueobject.SymlinkTypeFile,
		valueobject.SymlinkScopeInternal,
	)

	externalSymlink, _ := valueobject.NewSymlinkInfo("external_config.json", "/etc/config.json")
	externalSymlink = externalSymlink.WithResolution("", valueobject.SymlinkTypeFile, valueobject.SymlinkScopeExternal)

	brokenSymlink, _ := valueobject.NewSymlinkInfo("broken_link.txt", "./missing.txt")
	brokenSymlink = brokenSymlink.WithBrokenStatus(true)

	resolver.resolvedChains["config.json"] = &internalSymlink
	resolver.resolvedChains["external_config.json"] = &externalSymlink
	resolver.resolvedChains["broken_link.txt"] = &brokenSymlink

	resolver.symlinkScopes["config.json"] = valueobject.SymlinkScopeInternal
	resolver.symlinkScopes["external_config.json"] = valueobject.SymlinkScopeExternal
	resolver.symlinkScopes["broken_link.txt"] = valueobject.SymlinkScopeBroken
}

func validateProcessingMetadata(t *testing.T, result *ProcessingResult) {
	if result.ProcessingTime <= 0 {
		t.Error("Expected positive processing time")
	}

	if result.Metadata == nil {
		t.Error("Expected metadata to be initialized")
		return
	}

	if result.IsInSubmodule && result.SubmoduleInfo != nil {
		if _, exists := result.Metadata["submodule_path"]; !exists {
			t.Error("Expected submodule_path in metadata for submodule files")
		}
		if _, exists := result.Metadata["submodule_url"]; !exists {
			t.Error("Expected submodule_url in metadata for submodule files")
		}
	}

	if result.IsSymlink && result.SymlinkInfo != nil {
		if _, exists := result.Metadata["symlink_target"]; !exists {
			t.Error("Expected symlink_target in metadata for symlink files")
		}
		if _, exists := result.Metadata["symlink_scope"]; !exists {
			t.Error("Expected symlink_scope in metadata for symlink files")
		}
	}
}

// Error-inducing mock implementations

type errorFileFilter struct {
	errorType string
}

func (e *errorFileFilter) ShouldProcessFile(
	ctx context.Context,
	filePath string,
	fileInfo outbound.FileInfo,
) (outbound.FilterDecision, error) {
	if e.errorType == "filter_error" || e.errorType == "multiple_errors" {
		return outbound.FilterDecision{}, errors.New("mock filter error")
	}
	return outbound.FilterDecision{ShouldProcess: true}, nil
}

func (e *errorFileFilter) DetectBinaryFile(ctx context.Context, filePath string, content []byte) (bool, error) {
	return false, nil
}

func (e *errorFileFilter) DetectBinaryFromPath(ctx context.Context, filePath string) (bool, error) {
	return false, nil
}

func (e *errorFileFilter) MatchesGitignorePatterns(
	ctx context.Context,
	filePath string,
	repoPath string,
) (bool, error) {
	return false, nil
}

func (e *errorFileFilter) LoadGitignorePatterns(
	ctx context.Context,
	repoPath string,
) ([]outbound.GitignorePattern, error) {
	return []outbound.GitignorePattern{}, nil
}

func (e *errorFileFilter) FilterFilesBatch(
	ctx context.Context,
	files []outbound.FileInfo,
	repoPath string,
) ([]outbound.FilterResult, error) {
	return []outbound.FilterResult{}, nil
}

type errorSubmoduleDetector struct {
	errorType string
}

func (e *errorSubmoduleDetector) DetectSubmodules(
	ctx context.Context,
	repositoryPath string,
) ([]valueobject.SubmoduleInfo, error) {
	return []valueobject.SubmoduleInfo{}, nil
}

func (e *errorSubmoduleDetector) ParseGitmodulesFile(
	ctx context.Context,
	gitmodulesPath string,
) ([]valueobject.SubmoduleInfo, error) {
	return []valueobject.SubmoduleInfo{}, nil
}

func (e *errorSubmoduleDetector) IsSubmoduleDirectory(
	ctx context.Context,
	directoryPath string,
	repositoryRoot string,
) (bool, *valueobject.SubmoduleInfo, error) {
	if e.errorType == "submodule_error" || e.errorType == "multiple_errors" {
		return false, nil, errors.New("mock submodule detection error")
	}
	return false, nil, nil
}

func (e *errorSubmoduleDetector) GetSubmoduleStatus(
	ctx context.Context,
	submodulePath string,
	repositoryRoot string,
) (valueobject.SubmoduleStatus, error) {
	return valueobject.SubmoduleStatusUnknown, nil
}

func (e *errorSubmoduleDetector) ValidateSubmoduleConfiguration(
	ctx context.Context,
	submodule valueobject.SubmoduleInfo,
) error {
	return nil
}

type errorSymlinkResolver struct {
	errorType string
}

func (e *errorSymlinkResolver) DetectSymlinks(
	ctx context.Context,
	directoryPath string,
) ([]valueobject.SymlinkInfo, error) {
	return []valueobject.SymlinkInfo{}, nil
}

func (e *errorSymlinkResolver) IsSymlink(ctx context.Context, filePath string) (bool, error) {
	if e.errorType == "symlink_error" || e.errorType == "multiple_errors" {
		return false, errors.New("mock symlink detection error")
	}
	return false, nil
}

func (e *errorSymlinkResolver) ResolveSymlink(
	ctx context.Context,
	symlinkPath string,
) (*valueobject.SymlinkInfo, error) {
	return nil, errors.New("mock symlink resolution error")
}

func (e *errorSymlinkResolver) GetSymlinkTarget(ctx context.Context, symlinkPath string) (string, error) {
	return "", nil
}

func (e *errorSymlinkResolver) ValidateSymlinkTarget(
	ctx context.Context,
	symlink valueobject.SymlinkInfo,
) (*outbound.SymlinkValidationResult, error) {
	return nil, nil
}

// Repository setup functions

func setupRegularRepo(repoPath string) error {
	return os.MkdirAll(filepath.Join(repoPath, "src"), 0o755)
}

func setupRepoWithSubmodules(repoPath string) error {
	gitmodulesContent := `[submodule "common"]
	path = libs/common
	url = https://github.com/example/common.git
`
	return os.WriteFile(filepath.Join(repoPath, ".gitmodules"), []byte(gitmodulesContent), 0o644)
}

func setupRepoWithSymlinks(repoPath string) error {
	// In real implementation, this would create actual symlinks
	return os.MkdirAll(filepath.Join(repoPath, "shared"), 0o755)
}

func setupRepoWithExternalSymlinks(repoPath string) error {
	// In real implementation, this would create external symlinks
	return nil
}

func setupRepoWithSubmoduleSymlinks(repoPath string) error {
	// Combine submodule and symlink setup
	err := setupRepoWithSubmodules(repoPath)
	if err != nil {
		return err
	}
	return setupRepoWithSymlinks(repoPath)
}

func setupRepoWithBrokenSymlinks(repoPath string) error {
	// In real implementation, this would create broken symlinks
	return nil
}
