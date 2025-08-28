package symlink

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SecurityValidator handles security validation for symbolic links.
// It provides functionality to validate symlinks against security constraints
// and determine risk levels and recommended actions.
type SecurityValidator struct {
	resolver *Resolver
}

// NewSecurityValidator creates a new security validator instance.
func NewSecurityValidator(resolver *Resolver) *SecurityValidator {
	return &SecurityValidator{
		resolver: resolver,
	}
}

// ValidateSymlinkSecurity checks if following a symlink would violate security constraints.
func (sv *SecurityValidator) ValidateSymlinkSecurity(
	ctx context.Context,
	symlink valueobject.SymlinkInfo,
	repositoryRoot string,
) (*outbound.SymlinkSecurityResult, error) {
	violations := []string{}
	isSecure := true

	// Check if symlink escapes repository
	scope, err := sv.resolver.ClassifySymlinkScope(ctx, symlink, repositoryRoot)
	if err != nil {
		return nil, err
	}

	escapesRepository := scope == valueobject.SymlinkScopeExternal
	if escapesRepository {
		violations = append(violations, "escapes_repository")
		isSecure = false
	}

	// Check for sensitive paths
	targetPath := symlink.TargetPath()
	pointsToSensitive := sv.pointsToSensitivePath(targetPath)
	if pointsToSensitive {
		violations = append(violations, "points_to_sensitive_path")
		isSecure = false
	}

	// Check for circular references
	hasCircular := symlink.IsCircular()
	if hasCircular {
		violations = append(violations, "has_circular_reference")
		isSecure = false
	}

	// Check depth limit
	exceedsDepth := symlink.Depth() > 10
	if exceedsDepth {
		violations = append(violations, "exceeds_depth_limit")
		isSecure = false
	}

	// Determine risk level and action
	riskLevel := sv.calculateRiskLevel(violations)
	action := sv.determineAction(riskLevel)

	return &outbound.SymlinkSecurityResult{
		Symlink:               symlink,
		IsSecure:              isSecure,
		SecurityViolations:    violations,
		EscapesRepository:     escapesRepository,
		PointsToSensitivePath: pointsToSensitive,
		ExceedsDepthLimit:     exceedsDepth,
		HasCircularReference:  hasCircular,
		RecommendedAction:     action,
		RiskLevel:             riskLevel,
	}, nil
}

// ValidateSymlinkTarget checks if a symlink target exists and is accessible.
func (sv *SecurityValidator) ValidateSymlinkTarget(
	ctx context.Context,
	symlink valueobject.SymlinkInfo,
) (*outbound.SymlinkValidationResult, error) {
	start := time.Now()
	targetPath := symlink.TargetPath()

	// Resolve relative path
	resolvedPath := targetPath
	if !filepath.IsAbs(targetPath) {
		baseDir := filepath.Dir(symlink.Path())
		resolvedPath = filepath.Join(baseDir, targetPath)
		resolvedPath = filepath.Clean(resolvedPath)
	}

	// Validate path for security concerns
	if sv.isPathSuspicious(resolvedPath) {
		result := &outbound.SymlinkValidationResult{
			Symlink:      symlink,
			TargetExists: false,
			TargetType:   valueobject.SymlinkTypeBroken,
			IsAccessible: false,
			ResponseTime: time.Since(start),
			Error: &outbound.SymlinkResolutionError{
				Type:      outbound.ErrorTypeSymlinkSecurity,
				Message:   "symlink target path is suspicious or potentially dangerous",
				Symlink:   &symlink,
				Timestamp: time.Now(),
			},
		}
		return result, nil
	}

	// Check if target exists
	info, err := os.Stat(resolvedPath)
	targetExists := err == nil
	isAccessible := targetExists

	var targetType valueobject.SymlinkType
	if targetExists {
		if info.IsDir() {
			targetType = valueobject.SymlinkTypeDirectory
		} else {
			targetType = valueobject.SymlinkTypeFile
		}
	} else {
		targetType = valueobject.SymlinkTypeBroken
	}

	result := &outbound.SymlinkValidationResult{
		Symlink:      symlink,
		TargetExists: targetExists,
		TargetType:   targetType,
		IsAccessible: isAccessible,
		ResponseTime: time.Since(start),
	}

	if !targetExists {
		result.Error = &outbound.SymlinkResolutionError{
			Type:      outbound.ErrorTypeSymlinkBroken,
			Message:   "symlink target does not exist",
			Symlink:   &symlink,
			Timestamp: time.Now(),
		}
	}

	return result, nil
}

// CheckSecurityViolations performs comprehensive security validation on a symlink.
func (sv *SecurityValidator) CheckSecurityViolations(
	ctx context.Context,
	symlink valueobject.SymlinkInfo,
	repositoryRoot string,
) ([]string, error) {
	violations := []string{}

	// Check repository escape
	scope, err := sv.resolver.ClassifySymlinkScope(ctx, symlink, repositoryRoot)
	if err != nil {
		return nil, err
	}
	if scope == valueobject.SymlinkScopeExternal {
		violations = append(violations, "escapes_repository")
	}

	// Check sensitive paths
	if sv.pointsToSensitivePath(symlink.TargetPath()) {
		violations = append(violations, "points_to_sensitive_path")
	}

	// Check circular references
	if symlink.IsCircular() {
		violations = append(violations, "has_circular_reference")
	}

	// Check depth limits
	if symlink.Depth() > 10 {
		violations = append(violations, "exceeds_depth_limit")
	}

	// Check for path traversal attempts
	if sv.hasPathTraversalAttack(symlink.TargetPath()) {
		violations = append(violations, "path_traversal_attack")
	}

	return violations, nil
}

// Private helper methods

// pointsToSensitivePath checks if the target path points to sensitive system paths.
func (sv *SecurityValidator) pointsToSensitivePath(targetPath string) bool {
	sensitivePaths := []string{
		"/etc/passwd", "/etc/shadow", "/etc/hosts",
		"/proc", "/sys", "/dev", "/root",
		"/home", "/var/log", "/boot",
		"/tmp", "/var/tmp", "/usr/bin", "/bin", "/sbin",
	}

	// Clean the path to handle cases like "/etc/../etc/passwd"
	cleanPath := filepath.Clean(targetPath)

	for _, sensitive := range sensitivePaths {
		if strings.HasPrefix(cleanPath, sensitive) {
			return true
		}
	}
	return false
}

// calculateRiskLevel determines the risk level based on security violations.
func (sv *SecurityValidator) calculateRiskLevel(violations []string) string {
	if len(violations) == 0 {
		return outbound.SymlinkRiskLevelLow
	}

	// High risk violations that pose immediate security risks
	highRiskViolations := map[string]bool{
		"points_to_sensitive_path": true,
		"path_traversal_attack":    true,
		"escapes_repository":       true,
	}

	// Check for high risk violations
	for _, violation := range violations {
		if highRiskViolations[violation] {
			return outbound.SymlinkRiskLevelHigh
		}
	}

	// Medium risk for other violations
	return outbound.SymlinkRiskLevelMedium
}

// determineAction determines the recommended action based on risk level.
func (sv *SecurityValidator) determineAction(riskLevel string) string {
	switch riskLevel {
	case outbound.SymlinkRiskLevelHigh:
		return outbound.SymlinkActionBlock
	case outbound.SymlinkRiskLevelMedium:
		return outbound.SymlinkActionWarn
	default:
		return outbound.SymlinkActionAllow
	}
}

// isPathSuspicious checks for suspicious path patterns that might indicate security issues.
func (sv *SecurityValidator) isPathSuspicious(path string) bool {
	suspiciousPatterns := []string{
		"../", "./", "~/", "//", "\\",
		"%2e%2e", "%2f", "%5c", "..%2f", ".%2e%2f",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// hasPathTraversalAttack checks for path traversal attack patterns.
func (sv *SecurityValidator) hasPathTraversalAttack(targetPath string) bool {
	// Check for common path traversal patterns
	traversalPatterns := []string{
		"../", ".\\", "..\\", "..%2f", "..%5c",
		"%2e%2e%2f", "%2e%2e%5c", "....//", "....\\\\",
	}

	lowerPath := strings.ToLower(targetPath)
	for _, pattern := range traversalPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	// Check for excessive parent directory references
	parentDirCount := strings.Count(targetPath, "..")
	return parentDirCount > 5 // Configurable threshold
}
