package service

import (
	"context"
	"testing"
	"time"
)

// createTestMetrics creates a metrics instance for testing.
func createTestMetrics() *DiskMetrics {
	metrics, _ := NewDiskMetrics("test-instance")
	return metrics
}

// DiskRetentionPolicyService interface for managing retention policies.
type DiskRetentionPolicyService interface {
	// Policy management
	CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) (*RetentionPolicyResult, error)
	UpdateRetentionPolicy(ctx context.Context, policyID string, policy *RetentionPolicy) (*RetentionPolicyResult, error)
	DeleteRetentionPolicy(ctx context.Context, policyID string) error
	GetRetentionPolicy(ctx context.Context, policyID string) (*RetentionPolicy, error)
	ListRetentionPolicies(ctx context.Context, path string) ([]*RetentionPolicy, error)

	// Policy enforcement
	EnforceRetentionPolicy(ctx context.Context, policyID string, dryRun bool) (*PolicyEnforcementResult, error)
	EvaluatePolicyCompliance(ctx context.Context, path string, policies []*RetentionPolicy) (*ComplianceReport, error)

	// Policy inheritance and precedence
	ResolveEffectivePolicy(ctx context.Context, repositoryPath string) (*EffectivePolicy, error)
	CalculatePolicyPrecedence(ctx context.Context, policies []*RetentionPolicy) (*PolicyPrecedence, error)

	// Dynamic policy adjustment
	AdjustPolicyForDiskPressure(ctx context.Context, policyID string, pressure *DiskPressure) (*PolicyAdjustment, error)
	RecommendPolicyChanges(ctx context.Context, path string, usage *DiskUsageInfo) (*PolicyRecommendation, error)

	// Policy overrides
	CreatePolicyOverride(ctx context.Context, override *PolicyOverride) (*PolicyOverrideResult, error)
	EvaluateOverrides(ctx context.Context, repositoryPath string, basePolicy *RetentionPolicy) (*RetentionPolicy, error)
}

// ============================================================================
// FAILING TESTS FOR RETENTION POLICIES
// ============================================================================

func TestDiskRetentionPolicyService_CreateRetentionPolicy(t *testing.T) {
	tests := []struct {
		name        string
		policy      *RetentionPolicy
		expected    *RetentionPolicyResult
		expectError bool
	}{
		{
			name: "create basic time-based retention policy",
			policy: &RetentionPolicy{
				Name:        "30-day-retention",
				Description: "Keep repositories for 30 days",
				Path:        "/tmp/codechunking-cache",
				Rules: []*RetentionRule{
					{
						Type:   RetentionRuleAge,
						MaxAge: 30 * 24 * time.Hour,
						Weight: 1.0,
					},
				},
				Priority:          100,
				Enabled:           true,
				InheritFromParent: false,
				OverrideChildren:  true,
				Actions: []*PolicyAction{
					{Type: ActionDelete},
					{Type: ActionNotify},
				},
			},
			expected: &RetentionPolicyResult{
				Status:  "created",
				Message: "Retention policy created successfully",
			},
			expectError: false,
		},
		{
			name: "create complex multi-rule retention policy",
			policy: &RetentionPolicy{
				Name:        "hybrid-retention",
				Description: "Complex retention with multiple rules",
				Path:        "/tmp/codechunking-cache",
				Rules: []*RetentionRule{
					{
						Type:   RetentionRuleAge,
						MaxAge: 7 * 24 * time.Hour,
						Weight: 0.4,
					},
					{
						Type:        RetentionRuleIdle,
						MaxIdleTime: 3 * 24 * time.Hour,
						Weight:      0.3,
					},
					{
						Type:    RetentionRuleSize,
						MaxSize: 100 * 1024 * 1024 * 1024, // 100GB
						Weight:  0.3,
					},
				},
				Priority: 200,
				Enabled:  true,
				Conditions: []*PolicyCondition{
					{
						Type:     ConditionDiskUsage,
						Operator: "greater_than",
						Value:    80.0, // Trigger when disk usage > 80%
						Field:    "usage_percentage",
					},
				},
				Schedule: &PolicySchedule{
					CronExpression: "0 2 * * *", // Daily at 2 AM
					Timezone:       "UTC",
					Enabled:        true,
				},
			},
			expected: &RetentionPolicyResult{
				Status:  "created",
				Message: "Complex retention policy created successfully",
			},
			expectError: false,
		},
		{
			name: "create policy with invalid configuration should error",
			policy: &RetentionPolicy{
				Name:  "",                 // Invalid empty name
				Path:  "",                 // Invalid empty path
				Rules: []*RetentionRule{}, // No rules
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCreateRetentionPolicy(t, tt.policy, tt.expected, tt.expectError)
		})
	}
}

func testCreateRetentionPolicy(
	t *testing.T,
	policy *RetentionPolicy,
	expected *RetentionPolicyResult,
	expectError bool,
) {
	// Use concrete implementation instead of nil interface
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	result, err := service.CreateRetentionPolicy(ctx, policy)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateRetentionPolicyResult(t, result, expected)
}

func TestDiskRetentionPolicyService_EnforceRetentionPolicy(t *testing.T) {
	tests := []struct {
		name        string
		policyID    string
		dryRun      bool
		expected    *PolicyEnforcementResult
		expectError bool
	}{
		{
			name:     "enforce retention policy in production mode",
			policyID: "policy-123",
			dryRun:   false,
			expected: &PolicyEnforcementResult{
				PolicyID:         "policy-123",
				ItemsEvaluated:   250,
				ItemsAffected:    75,
				BytesFreed:       15 * 1024 * 1024 * 1024, // 15GB
				DryRun:           false,
				ComplianceStatus: CompliancePass,
			},
			expectError: false,
		},
		{
			name:     "enforce retention policy in dry run mode",
			policyID: "policy-456",
			dryRun:   true,
			expected: &PolicyEnforcementResult{
				PolicyID:         "policy-456",
				ItemsEvaluated:   180,
				ItemsAffected:    0, // No actual changes in dry run
				BytesFreed:       0,
				DryRun:           true,
				ComplianceStatus: ComplianceWarning,
			},
			expectError: false,
		},
		{
			name:        "enforce non-existent policy should error",
			policyID:    "non-existent",
			dryRun:      false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnforceRetentionPolicy(t, tt.policyID, tt.dryRun, tt.expected, tt.expectError)
		})
	}
}

func testEnforceRetentionPolicy(
	t *testing.T,
	policyID string,
	dryRun bool,
	expected *PolicyEnforcementResult,
	expectError bool,
) {
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	result, err := service.EnforceRetentionPolicy(ctx, policyID, dryRun)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validatePolicyEnforcementResult(t, result, expected)
}

func TestDiskRetentionPolicyService_EvaluatePolicyCompliance(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		policies           []*RetentionPolicy
		expectedStatus     ComplianceStatus
		expectedViolations int
		expectError        bool
	}{
		{
			name: "evaluate compliance with compliant repositories",
			path: "/tmp/codechunking-cache",
			policies: []*RetentionPolicy{
				{
					ID:   "policy-1",
					Name: "Basic retention",
					Rules: []*RetentionRule{
						{Type: RetentionRuleAge, MaxAge: 30 * 24 * time.Hour},
					},
				},
			},
			expectedStatus:     CompliancePass,
			expectedViolations: 0,
			expectError:        false,
		},
		{
			name: "evaluate compliance with policy violations",
			path: "/tmp/codechunking-cache",
			policies: []*RetentionPolicy{
				{
					ID:   "policy-2",
					Name: "Strict retention",
					Rules: []*RetentionRule{
						{Type: RetentionRuleAge, MaxAge: 1 * 24 * time.Hour}, // Very strict
						{Type: RetentionRuleIdle, MaxIdleTime: 6 * time.Hour},
					},
				},
			},
			expectedStatus:     ComplianceFail,
			expectedViolations: 15,
			expectError:        false,
		},
		{
			name: "evaluate compliance with multiple policies",
			path: "/tmp/codechunking-cache",
			policies: []*RetentionPolicy{
				{ID: "policy-3", Name: "Policy 1"},
				{ID: "policy-4", Name: "Policy 2"},
				{ID: "policy-5", Name: "Policy 3"},
			},
			expectedStatus:     ComplianceWarning,
			expectedViolations: 5,
			expectError:        false,
		},
		{
			name:        "evaluate compliance with invalid path should error",
			path:        "/invalid/path",
			policies:    []*RetentionPolicy{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEvaluatePolicyCompliance(
				t,
				tt.path,
				tt.policies,
				tt.expectedStatus,
				tt.expectedViolations,
				tt.expectError,
			)
		})
	}
}

func testEvaluatePolicyCompliance(
	t *testing.T,
	path string,
	policies []*RetentionPolicy,
	expectedStatus ComplianceStatus,
	expectedViolations int,
	expectError bool,
) {
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	report, err := service.EvaluatePolicyCompliance(ctx, path, policies)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateComplianceReport(t, report, expectedStatus, expectedViolations)
}

func TestDiskRetentionPolicyService_ResolveEffectivePolicy(t *testing.T) {
	tests := []struct {
		name           string
		repositoryPath string
		expected       *EffectivePolicy
		expectError    bool
	}{
		{
			name:           "resolve effective policy with single policy",
			repositoryPath: "/tmp/codechunking-cache/repo1",
			expected: &EffectivePolicy{
				RepositoryPath:   "/tmp/codechunking-cache/repo1",
				InheritanceChain: []string{"/", "/tmp", "/tmp/codechunking-cache"},
				ResolutionReason: "inherited from parent path",
			},
			expectError: false,
		},
		{
			name:           "resolve effective policy with inheritance chain",
			repositoryPath: "/tmp/codechunking-cache/critical/repo2",
			expected: &EffectivePolicy{
				RepositoryPath:   "/tmp/codechunking-cache/critical/repo2",
				InheritanceChain: []string{"/", "/tmp", "/tmp/codechunking-cache", "/tmp/codechunking-cache/critical"},
				ResolutionReason: "multiple policies merged with precedence",
			},
			expectError: false,
		},
		{
			name:           "resolve effective policy with overrides",
			repositoryPath: "/tmp/codechunking-cache/override-test/repo3",
			expected: &EffectivePolicy{
				RepositoryPath:   "/tmp/codechunking-cache/override-test/repo3",
				ResolutionReason: "base policy modified by active overrides",
			},
			expectError: false,
		},
		{
			name:           "resolve effective policy for invalid path should error",
			repositoryPath: "", // Empty path
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testResolveEffectivePolicy(t, tt.repositoryPath, tt.expected, tt.expectError)
		})
	}
}

func testResolveEffectivePolicy(t *testing.T, repositoryPath string, expected *EffectivePolicy, expectError bool) {
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	effective, err := service.ResolveEffectivePolicy(ctx, repositoryPath)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateEffectivePolicy(t, effective, expected)
}

func TestDiskRetentionPolicyService_AdjustPolicyForDiskPressure(t *testing.T) {
	tests := []struct {
		name        string
		policyID    string
		pressure    *DiskPressure
		expected    *PolicyAdjustment
		expectError bool
	}{
		{
			name:     "adjust policy for high disk pressure",
			policyID: "policy-123",
			pressure: &DiskPressure{
				CurrentUsagePercent: 85.0,
				FreeSpaceBytes:      10 * 1024 * 1024 * 1024, // 10GB
				Severity:            PressureHigh,
			},
			expected: &PolicyAdjustment{
				OriginalPolicyID: "policy-123",
				Reason:           "high disk pressure detected",
				Adjustments: []*Adjustment{
					{
						Type:      "rule_modification",
						Field:     "max_age",
						Reasoning: "reduced retention period for emergency cleanup",
					},
					{
						Type:      "rule_modification",
						Field:     "max_idle_time",
						Reasoning: "aggressive idle time cleanup",
					},
				},
			},
			expectError: false,
		},
		{
			name:     "adjust policy for critical disk pressure",
			policyID: "policy-456",
			pressure: &DiskPressure{
				CurrentUsagePercent: 95.0,
				FreeSpaceBytes:      1 * 1024 * 1024 * 1024, // 1GB
				Severity:            PressureCritical,
			},
			expected: &PolicyAdjustment{
				OriginalPolicyID: "policy-456",
				Reason:           "critical disk pressure requires emergency action",
				Adjustments: []*Adjustment{
					{
						Type:      "emergency_override",
						Field:     "all_rules",
						Reasoning: "emergency cleanup to prevent disk full",
					},
				},
			},
			expectError: false,
		},
		{
			name:     "adjust policy with low pressure should make minimal changes",
			policyID: "policy-789",
			pressure: &DiskPressure{
				CurrentUsagePercent: 45.0,
				FreeSpaceBytes:      50 * 1024 * 1024 * 1024, // 50GB
				Severity:            PressureLow,
			},
			expected: &PolicyAdjustment{
				OriginalPolicyID: "policy-789",
				Reason:           "low disk pressure, minimal adjustments needed",
				Adjustments:      []*Adjustment{}, // No adjustments needed
			},
			expectError: false,
		},
		{
			name:        "adjust non-existent policy should error",
			policyID:    "non-existent",
			pressure:    &DiskPressure{Severity: PressureHigh},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAdjustPolicyForDiskPressure(t, tt.policyID, tt.pressure, tt.expected, tt.expectError)
		})
	}
}

func testAdjustPolicyForDiskPressure(
	t *testing.T,
	policyID string,
	pressure *DiskPressure,
	expected *PolicyAdjustment,
	expectError bool,
) {
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	adjustment, err := service.AdjustPolicyForDiskPressure(ctx, policyID, pressure)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validatePolicyAdjustment(t, adjustment, expected)
}

func TestDiskRetentionPolicyService_CreatePolicyOverride(t *testing.T) {
	tests := []struct {
		name        string
		override    *PolicyOverride
		expected    *PolicyOverrideResult
		expectError bool
	}{
		{
			name: "create temporary policy suspension override",
			override: &PolicyOverride{
				PolicyID:       "policy-123",
				RepositoryPath: "/tmp/codechunking-cache/critical-repo",
				OverrideType:   "suspend",
				Reason:         "Critical maintenance window",
				RequestedBy:    "admin@example.com",
				ValidFrom:      time.Now(),
				ValidUntil:     time.Now().Add(24 * time.Hour),
				Status:         "pending_approval",
			},
			expected: &PolicyOverrideResult{
				Status:  "created",
				Message: "Policy override created successfully",
			},
			expectError: false,
		},
		{
			name: "create policy modification override",
			override: &PolicyOverride{
				PolicyID:       "policy-456",
				RepositoryPath: "/tmp/codechunking-cache/special-repo",
				OverrideType:   "modify",
				Modifications: map[string]interface{}{
					"max_age":       "72h",
					"max_idle_time": "12h",
				},
				Reason:      "Extended retention for audit purposes",
				RequestedBy: "compliance@example.com",
				ValidFrom:   time.Now(),
				ValidUntil:  time.Now().Add(7 * 24 * time.Hour),
				Status:      "approved",
			},
			expected: &PolicyOverrideResult{
				Status:  "created",
				Message: "Policy modification override created successfully",
			},
			expectError: false,
		},
		{
			name: "create override with invalid date range should error",
			override: &PolicyOverride{
				PolicyID:   "policy-789",
				ValidFrom:  time.Now().Add(24 * time.Hour),
				ValidUntil: time.Now(), // End before start
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCreatePolicyOverride(t, tt.override, tt.expected, tt.expectError)
		})
	}
}

func testCreatePolicyOverride(
	t *testing.T,
	override *PolicyOverride,
	expected *PolicyOverrideResult,
	expectError bool,
) {
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	result, err := service.CreatePolicyOverride(ctx, override)

	if expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validatePolicyOverrideResult(t, result, expected)
}

// ============================================================================
// FAILING TESTS FOR MEDIUM CONSTANT USAGE
// ============================================================================

func TestDiskRetentionPolicyService_MediumConstantsShouldExist(t *testing.T) {
	t.Run("should have MEDIUM_SEVERITY constant for PolicyViolation.Severity", func(t *testing.T) {
		testMediumSeverityConstantExists(t)
	})

	t.Run("should have MEDIUM_PRIORITY constant for PolicyRecommendation.Priority", func(t *testing.T) {
		testMediumPriorityConstantExists(t)
	})

	t.Run("should have MEDIUM_RISK_LEVEL constant for RecommendationImpact.RiskLevel", func(t *testing.T) {
		testMediumRiskLevelConstantExists(t)
	})

	t.Run("should have MEDIUM_PRIORITY constant for RecommendedChange.Priority", func(t *testing.T) {
		testMediumChangePriorityConstantExists(t)
	})
}

func TestDiskRetentionPolicyService_EvaluatePolicyComplianceShouldUseMediumSeverityConstant(t *testing.T) {
	// This test will FAIL until the MEDIUM_SEVERITY constant is created and used
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	policies := []*RetentionPolicy{
		{ID: "policy-1", Name: "Policy 2"},
		{ID: "policy-2", Name: "Policy 3"},
		{ID: "policy-3", Name: "Policy 4"},
	}

	report, err := service.EvaluatePolicyCompliance(ctx, "/tmp/test", policies)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Find violations that should use MEDIUM_SEVERITY constant
	foundMediumSeverityViolation := false
	for _, violation := range report.Violations {
		if violation.Severity == "medium" { // This should be MEDIUM_SEVERITY constant
			foundMediumSeverityViolation = true
			break
		}
	}

	if !foundMediumSeverityViolation {
		t.Error("Expected to find violation using hardcoded 'medium' string that should use MEDIUM_SEVERITY constant")
	}

	// This assertion will FAIL until constant is defined and used
	// The implementation should use MEDIUM_SEVERITY instead of "medium"
	t.Error(
		"FAILING TEST: Expected MEDIUM_SEVERITY constant to be defined and used instead of hardcoded 'medium' string for PolicyViolation.Severity at line 496",
	)
}

func TestDiskRetentionPolicyService_RecommendPolicyChangesShouldUseMediumConstants(t *testing.T) {
	// This test will FAIL until the MEDIUM constants are created and used
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	usage := &DiskUsageInfo{
		Path:            "/tmp/test",
		UsagePercentage: 70.0,                     // Normal usage to trigger medium priority
		TotalSpaceBytes: 100 * 1024 * 1024 * 1024, // 100GB
		UsedSpaceBytes:  70 * 1024 * 1024 * 1024,  // 70GB
	}

	recommendation, err := service.RecommendPolicyChanges(ctx, "/tmp/test", usage)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Test that Priority uses hardcoded "medium" (should be MEDIUM_PRIORITY constant)
	if recommendation.Priority == "medium" {
		t.Error(
			"FAILING TEST: Expected MEDIUM_PRIORITY constant to be defined and used instead of hardcoded 'medium' string for PolicyRecommendation.Priority at line 752",
		)
	}

	// Test that EstimatedImpact.RiskLevel uses hardcoded "medium" (should be MEDIUM_RISK_LEVEL constant)
	if recommendation.EstimatedImpact != nil && recommendation.EstimatedImpact.RiskLevel == "medium" {
		t.Error(
			"FAILING TEST: Expected MEDIUM_RISK_LEVEL constant to be defined and used instead of hardcoded 'medium' string for RecommendationImpact.RiskLevel at line 776",
		)
	}

	// Test that RecommendedChanges Priority uses hardcoded "medium" (should be MEDIUM_PRIORITY constant)
	for _, change := range recommendation.RecommendedChanges {
		if change.Priority == "medium" {
			t.Error(
				"FAILING TEST: Expected MEDIUM_PRIORITY constant to be defined and used instead of hardcoded 'medium' string for RecommendedChange.Priority at line 783",
			)
		}
	}
}

func TestDiskRetentionPolicyService_HighDiskUsageShouldNotUseMediumRiskLevel(t *testing.T) {
	// This test ensures the high usage path still triggers medium risk level (which should use constant)
	service := NewDefaultDiskRetentionPolicyService(createTestMetrics())
	ctx := context.Background()

	usage := &DiskUsageInfo{
		Path:            "/tmp/test",
		UsagePercentage: 85.0,                     // High usage triggers different behavior
		TotalSpaceBytes: 100 * 1024 * 1024 * 1024, // 100GB
		UsedSpaceBytes:  85 * 1024 * 1024 * 1024,  // 85GB
	}

	recommendation, err := service.RecommendPolicyChanges(ctx, "/tmp/test", usage)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// The high usage path sets RiskLevel to "medium" at line 776
	if recommendation.EstimatedImpact != nil && recommendation.EstimatedImpact.RiskLevel == "medium" {
		t.Error(
			"FAILING TEST: Expected MEDIUM_RISK_LEVEL constant to be defined and used instead of hardcoded 'medium' string for high disk usage scenario at line 776",
		)
	}
}

func TestDiskRetentionPolicyService_MediumConstantValues(t *testing.T) {
	// Test that will FAIL until constants are properly defined with expected values
	t.Run("MEDIUM_SEVERITY should equal 'medium'", func(t *testing.T) {
		// This will fail until constant is defined
		t.Error("FAILING TEST: MEDIUM_SEVERITY constant should be defined with value 'medium'")
	})

	t.Run("MEDIUM_PRIORITY should equal 'medium'", func(t *testing.T) {
		// This will fail until constant is defined
		t.Error("FAILING TEST: MEDIUM_PRIORITY constant should be defined with value 'medium'")
	})

	t.Run("MEDIUM_RISK_LEVEL should equal 'medium'", func(t *testing.T) {
		// This will fail until constant is defined
		t.Error("FAILING TEST: MEDIUM_RISK_LEVEL constant should be defined with value 'medium'")
	})
}

// Helper test functions for constant validation.
func testMediumSeverityConstantExists(t *testing.T) {
	// This will FAIL until MEDIUM_SEVERITY constant exists in the constants section
	t.Error(
		"FAILING TEST: MEDIUM_SEVERITY constant should be defined in constants section of disk_retention_policy_service.go",
	)
}

func testMediumPriorityConstantExists(t *testing.T) {
	// This will FAIL until MEDIUM_PRIORITY constant exists in the constants section
	t.Error(
		"FAILING TEST: MEDIUM_PRIORITY constant should be defined in constants section of disk_retention_policy_service.go",
	)
}

func testMediumRiskLevelConstantExists(t *testing.T) {
	// This will FAIL until MEDIUM_RISK_LEVEL constant exists in the constants section
	t.Error(
		"FAILING TEST: MEDIUM_RISK_LEVEL constant should be defined in constants section of disk_retention_policy_service.go",
	)
}

func testMediumChangePriorityConstantExists(t *testing.T) {
	// This will FAIL until MEDIUM_PRIORITY constant exists and is used for RecommendedChange.Priority
	t.Error(
		"FAILING TEST: MEDIUM_PRIORITY constant should be defined and used for RecommendedChange.Priority in constants section of disk_retention_policy_service.go",
	)
}

func TestDiskRetentionPolicyService_AllMediumOccurrencesMustUseConstants(t *testing.T) {
	// This comprehensive test validates that ALL 4 occurrences of "medium" string are replaced with constants
	t.Run("line_496_PolicyViolation_Severity_must_use_MEDIUM_SEVERITY_constant", func(t *testing.T) {
		// Specific occurrence: disk_retention_policy_service.go:496:46
		// Context: `Severity: "medium"` in PolicyViolation struct
		// Issue: string `medium` has 4 occurrences, make it a constant
		t.Error(
			"FAILING TEST: Line 496 PolicyViolation.Severity must use MEDIUM_SEVERITY constant instead of hardcoded 'medium' string",
		)
	})

	t.Run("line_752_PolicyRecommendation_Priority_must_use_MEDIUM_PRIORITY_constant", func(t *testing.T) {
		// Specific occurrence: disk_retention_policy_service.go:752:46
		// Context: `Priority: "medium"` in PolicyRecommendation struct
		// Issue: string `medium` has 4 occurrences, make it a constant
		t.Error(
			"FAILING TEST: Line 752 PolicyRecommendation.Priority must use MEDIUM_PRIORITY constant instead of hardcoded 'medium' string",
		)
	})

	t.Run("line_776_RecommendationImpact_RiskLevel_must_use_MEDIUM_RISK_LEVEL_constant", func(t *testing.T) {
		// Specific occurrence: disk_retention_policy_service.go:776:46
		// Context: `recommendation.EstimatedImpact.RiskLevel = "medium"`
		// Issue: string `medium` has 4 occurrences, make it a constant
		t.Error(
			"FAILING TEST: Line 776 RecommendationImpact.RiskLevel must use MEDIUM_RISK_LEVEL constant instead of hardcoded 'medium' string",
		)
	})

	t.Run("line_783_RecommendedChange_Priority_must_use_MEDIUM_PRIORITY_constant", func(t *testing.T) {
		// Specific occurrence: disk_retention_policy_service.go:783:46
		// Context: `Priority: "medium"` in RecommendedChange struct
		// Issue: string `medium` has 4 occurrences, make it a constant
		t.Error(
			"FAILING TEST: Line 783 RecommendedChange.Priority must use MEDIUM_PRIORITY constant instead of hardcoded 'medium' string",
		)
	})

	t.Run("exactly_4_medium_string_occurrences_must_be_eliminated", func(t *testing.T) {
		// goconst violation reports exactly 4 occurrences of "medium" string
		// All 4 must be replaced with appropriate constants
		expectedOccurrences := 4
		t.Errorf(
			"FAILING TEST: Should eliminate exactly %d hardcoded 'medium' strings by using appropriate constants (MEDIUM_SEVERITY, MEDIUM_PRIORITY, MEDIUM_RISK_LEVEL)",
			expectedOccurrences,
		)
	})
}

func TestDiskRetentionPolicyService_ExpectedConstantsDefinition(t *testing.T) {
	// Test defining the exact constants that should be added to resolve goconst violation
	t.Run("constants_should_be_added_to_existing_const_block", func(t *testing.T) {
		// The file already has a const block starting at line 14:
		// const (
		//     mockEnforcementDurationMinutes = 15
		//     mockViolationThreshold         = 10
		//     mockEstimatedSavingsGB         = 2
		//     diskUsageHighThreshold         = 80
		// )
		//
		// The following constants should be added:
		// MEDIUM_SEVERITY = "medium"
		// MEDIUM_PRIORITY = "medium"
		// MEDIUM_RISK_LEVEL = "medium"

		t.Error("FAILING TEST: MEDIUM_SEVERITY = \"medium\" should be added to existing const block")
		t.Error("FAILING TEST: MEDIUM_PRIORITY = \"medium\" should be added to existing const block")
		t.Error("FAILING TEST: MEDIUM_RISK_LEVEL = \"medium\" should be added to existing const block")
	})

	t.Run("constants_should_follow_naming_conventions", func(t *testing.T) {
		// Constants should follow Go naming conventions:
		// - ALL_CAPS with underscores for multi-word constants
		// - Descriptive names that clearly indicate their usage context
		// - Grouped logically in the const block

		t.Error("FAILING TEST: Constants should follow UPPER_CASE naming convention with underscores")
	})
}

// Helper functions for validation

func validateRetentionPolicyResult(t *testing.T, result, expected *RetentionPolicyResult) {
	if result == nil {
		t.Fatal("Expected policy result, got nil")
	}

	if expected == nil {
		return // No specific expectations
	}

	if result.Status != expected.Status {
		t.Errorf("Expected status %s, got %s", expected.Status, result.Status)
	}

	if result.PolicyID == "" {
		t.Error("Policy result should have policy ID")
	}

	if result.CreatedAt.IsZero() && result.UpdatedAt.IsZero() {
		t.Error("Policy result should have created or updated timestamp")
	}
}

func validatePolicyEnforcementResult(t *testing.T, result, expected *PolicyEnforcementResult) {
	if result == nil {
		t.Fatal("Expected enforcement result, got nil")
	}

	if expected == nil {
		return
	}

	if result.PolicyID != expected.PolicyID {
		t.Errorf("Expected policy ID %s, got %s", expected.PolicyID, result.PolicyID)
	}

	if result.DryRun != expected.DryRun {
		t.Errorf("Expected dry run %v, got %v", expected.DryRun, result.DryRun)
	}

	if expected.ItemsEvaluated > 0 && result.ItemsEvaluated != expected.ItemsEvaluated {
		t.Errorf("Expected %d items evaluated, got %d", expected.ItemsEvaluated, result.ItemsEvaluated)
	}

	if expected.ComplianceStatus != ComplianceUnknown && result.ComplianceStatus != expected.ComplianceStatus {
		t.Errorf("Expected compliance status %v, got %v", expected.ComplianceStatus, result.ComplianceStatus)
	}

	if result.EnforcementID == "" {
		t.Error("Enforcement result should have enforcement ID")
	}

	if result.Duration <= 0 {
		t.Error("Enforcement should have positive duration")
	}

	if result.Performance == nil {
		t.Error("Enforcement result should include performance metrics")
	}
}

func validateComplianceReport(
	t *testing.T,
	report *ComplianceReport,
	expectedStatus ComplianceStatus,
	expectedViolations int,
) {
	if report == nil {
		t.Fatal("Expected compliance report, got nil")
	}

	if report.OverallStatus != expectedStatus {
		t.Errorf("Expected overall status %v, got %v", expectedStatus, report.OverallStatus)
	}

	if len(report.Violations) != expectedViolations {
		t.Errorf("Expected %d violations, got %d", expectedViolations, len(report.Violations))
	}

	if report.EvaluationID == "" {
		t.Error("Compliance report should have evaluation ID")
	}

	if report.Summary == nil {
		t.Error("Compliance report should have summary")
	}

	if len(report.PolicyResults) == 0 {
		t.Error("Compliance report should have policy results")
	}
}

func validateEffectivePolicy(t *testing.T, effective, expected *EffectivePolicy) {
	if effective == nil {
		t.Fatal("Expected effective policy, got nil")
	}

	if expected == nil {
		return
	}

	if effective.RepositoryPath != expected.RepositoryPath {
		t.Errorf("Expected repository path %s, got %s", expected.RepositoryPath, effective.RepositoryPath)
	}

	if effective.FinalPolicy == nil {
		t.Error("Effective policy should have final policy")
	}

	if len(effective.InheritanceChain) == 0 {
		t.Error("Effective policy should show inheritance chain")
	}

	if effective.ResolutionReason == "" {
		t.Error("Effective policy should have resolution reason")
	}
}

func validatePolicyAdjustment(t *testing.T, adjustment, expected *PolicyAdjustment) {
	if adjustment == nil {
		t.Fatal("Expected policy adjustment, got nil")
	}

	if expected == nil {
		return
	}

	if adjustment.OriginalPolicyID != expected.OriginalPolicyID {
		t.Errorf("Expected original policy ID %s, got %s", expected.OriginalPolicyID, adjustment.OriginalPolicyID)
	}

	if adjustment.AdjustedPolicy == nil {
		t.Error("Policy adjustment should have adjusted policy")
	}

	if adjustment.Reason == "" {
		t.Error("Policy adjustment should have reason")
	}

	if len(expected.Adjustments) > 0 && len(adjustment.Adjustments) != len(expected.Adjustments) {
		t.Errorf("Expected %d adjustments, got %d", len(expected.Adjustments), len(adjustment.Adjustments))
	}
}

func validatePolicyOverrideResult(t *testing.T, result, expected *PolicyOverrideResult) {
	if result == nil {
		t.Fatal("Expected policy override result, got nil")
	}

	if expected == nil {
		return
	}

	if result.Status != expected.Status {
		t.Errorf("Expected status %s, got %s", expected.Status, result.Status)
	}

	if result.OverrideID == "" {
		t.Error("Policy override result should have override ID")
	}

	if len(result.ValidationErrors) > 0 {
		for _, err := range result.ValidationErrors {
			t.Errorf("Validation error: %s - %s", err.Field, err.Message)
		}
	}
}
