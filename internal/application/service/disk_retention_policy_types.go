package service

import (
	"time"
)

// RetentionPolicy defines how long repositories should be retained.
type RetentionPolicy struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	Path              string               `json:"path"`
	Rules             []*RetentionRule     `json:"rules"`
	Priority          int                  `json:"priority"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
	CreatedBy         string               `json:"created_by"`
	Enabled           bool                 `json:"enabled"`
	InheritFromParent bool                 `json:"inherit_from_parent"`
	OverrideChildren  bool                 `json:"override_children"`
	Conditions        []*PolicyCondition   `json:"conditions"`
	Actions           []*PolicyAction      `json:"actions"`
	Schedule          *PolicySchedule      `json:"schedule"`
	Notifications     *PolicyNotifications `json:"notifications"`
}

// RetentionRule defines specific retention criteria.
type RetentionRule struct {
	Type            RetentionRuleType `json:"type"`
	MaxAge          time.Duration     `json:"max_age,omitempty"`
	MaxIdleTime     time.Duration     `json:"max_idle_time,omitempty"`
	MaxSize         int64             `json:"max_size,omitempty"`
	KeepLastN       int               `json:"keep_last_n,omitempty"`
	KeepDaily       int               `json:"keep_daily,omitempty"`
	KeepWeekly      int               `json:"keep_weekly,omitempty"`
	KeepMonthly     int               `json:"keep_monthly,omitempty"`
	ExcludePatterns []string          `json:"exclude_patterns,omitempty"`
	IncludePatterns []string          `json:"include_patterns,omitempty"`
	Weight          float64           `json:"weight"`
}

// RetentionRuleType represents different types of retention rules.
type RetentionRuleType int

const (
	RetentionRuleAge RetentionRuleType = iota
	RetentionRuleIdle
	RetentionRuleSize
	RetentionRuleCount
	RetentionRulePattern
	RetentionRuleAccess
	RetentionRuleImportance
)

// PolicyCondition defines when a policy should apply.
type PolicyCondition struct {
	Type     ConditionType `json:"type"`
	Operator string        `json:"operator"` // "equals", "greater_than", "less_than", "contains"
	Value    interface{}   `json:"value"`
	Field    string        `json:"field"`
}

// ConditionType represents different condition types.
type ConditionType int

const (
	ConditionDiskUsage ConditionType = iota
	ConditionTime
	ConditionSize
	ConditionPattern
	ConditionMetadata
)

// PolicyAction defines what actions to take when policy is enforced.
type PolicyAction struct {
	Type       ActionType             `json:"type"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// ActionType represents different policy actions.
type ActionType int

const (
	ActionDelete ActionType = iota
	ActionCompress
	ActionArchive
	ActionMove
	ActionNotify
	ActionLog
)

// PolicySchedule defines when policy enforcement should run.
type PolicySchedule struct {
	CronExpression string    `json:"cron_expression"`
	Timezone       string    `json:"timezone"`
	NextRun        time.Time `json:"next_run"`
	LastRun        time.Time `json:"last_run"`
	Enabled        bool      `json:"enabled"`
}

// PolicyNotifications defines notification settings for policy events.
type PolicyNotifications struct {
	OnViolation   bool     `json:"on_violation"`
	OnEnforcement bool     `json:"on_enforcement"`
	OnError       bool     `json:"on_error"`
	Recipients    []string `json:"recipients"`
	SlackWebhook  string   `json:"slack_webhook,omitempty"`
	EmailTemplate string   `json:"email_template,omitempty"`
}

// RetentionPolicyResult contains the result of policy operations.
type RetentionPolicyResult struct {
	PolicyID         string                  `json:"policy_id"`
	Status           string                  `json:"status"`
	Message          string                  `json:"message"`
	ValidationErrors []PolicyValidationError `json:"validation_errors,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

// PolicyValidationError represents validation errors in policies.
type PolicyValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// PolicyEnforcementResult contains results of policy enforcement.
type PolicyEnforcementResult struct {
	PolicyID         string                  `json:"policy_id"`
	EnforcementID    string                  `json:"enforcement_id"`
	StartTime        time.Time               `json:"start_time"`
	EndTime          time.Time               `json:"end_time"`
	Duration         time.Duration           `json:"duration"`
	ItemsEvaluated   int                     `json:"items_evaluated"`
	ItemsAffected    int                     `json:"items_affected"`
	BytesFreed       int64                   `json:"bytes_freed"`
	DryRun           bool                    `json:"dry_run"`
	Actions          []*EnforcementAction    `json:"actions"`
	Errors           []EnforcementError      `json:"errors"`
	ComplianceStatus ComplianceStatus        `json:"compliance_status"`
	Performance      *EnforcementPerformance `json:"performance"`
}

// EnforcementAction represents an action taken during policy enforcement.
type EnforcementAction struct {
	Type       ActionType    `json:"type"`
	TargetPath string        `json:"target_path"`
	Success    bool          `json:"success"`
	SizeFreed  int64         `json:"size_freed,omitempty"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	Timestamp  time.Time     `json:"timestamp"`
}

// EnforcementError represents errors during policy enforcement.
type EnforcementError struct {
	Type       string    `json:"type"`
	Message    string    `json:"message"`
	TargetPath string    `json:"target_path,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Severity   string    `json:"severity"`
}

// ComplianceStatus represents policy compliance status.
type ComplianceStatus int

const (
	ComplianceUnknown ComplianceStatus = iota
	CompliancePass
	ComplianceWarning
	ComplianceFail
	ComplianceError
)

// EnforcementPerformance contains performance metrics for enforcement.
type EnforcementPerformance struct {
	ItemsPerSecond float64 `json:"items_per_second"`
	BytesPerSecond int64   `json:"bytes_per_second"`
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    int64   `json:"memory_usage"`
	IOOperations   int64   `json:"io_operations"`
}

// ComplianceReport contains policy compliance analysis.
type ComplianceReport struct {
	EvaluationID    string                    `json:"evaluation_id"`
	Path            string                    `json:"path"`
	Timestamp       time.Time                 `json:"timestamp"`
	OverallStatus   ComplianceStatus          `json:"overall_status"`
	PolicyResults   []*PolicyComplianceResult `json:"policy_results"`
	Violations      []*PolicyViolation        `json:"violations"`
	Recommendations []string                  `json:"recommendations"`
	Summary         *ComplianceSummary        `json:"summary"`
}

// PolicyComplianceResult contains compliance result for a specific policy.
type PolicyComplianceResult struct {
	PolicyID       string           `json:"policy_id"`
	PolicyName     string           `json:"policy_name"`
	Status         ComplianceStatus `json:"status"`
	Message        string           `json:"message"`
	ItemsEvaluated int              `json:"items_evaluated"`
	ItemsViolating int              `json:"items_violating"`
	RuleResults    []*RuleResult    `json:"rule_results"`
}

// RuleResult contains result for a specific retention rule.
type RuleResult struct {
	RuleType         RetentionRuleType `json:"rule_type"`
	Status           ComplianceStatus  `json:"status"`
	ItemsEvaluated   int               `json:"items_evaluated"`
	ItemsViolating   int               `json:"items_violating"`
	EstimatedSavings int64             `json:"estimated_savings"`
}

// PolicyViolation represents a policy violation.
type PolicyViolation struct {
	PolicyID         string            `json:"policy_id"`
	RuleType         RetentionRuleType `json:"rule_type"`
	TargetPath       string            `json:"target_path"`
	Severity         string            `json:"severity"`
	Description      string            `json:"description"`
	CurrentValue     string            `json:"current_value"`
	ExpectedValue    string            `json:"expected_value"`
	EstimatedSavings int64             `json:"estimated_savings"`
	DetectedAt       time.Time         `json:"detected_at"`
}

// ComplianceSummary provides a summary of compliance status.
type ComplianceSummary struct {
	TotalPolicies      int   `json:"total_policies"`
	PoliciesCompliant  int   `json:"policies_compliant"`
	PoliciesViolating  int   `json:"policies_violating"`
	TotalViolations    int   `json:"total_violations"`
	CriticalViolations int   `json:"critical_violations"`
	EstimatedSavingsGB int64 `json:"estimated_savings_gb"`
}

// EffectivePolicy represents the final policy after inheritance and precedence.
type EffectivePolicy struct {
	RepositoryPath   string             `json:"repository_path"`
	FinalPolicy      *RetentionPolicy   `json:"final_policy"`
	SourcePolicies   []*RetentionPolicy `json:"source_policies"`
	InheritanceChain []string           `json:"inheritance_chain"`
	AppliedOverrides []*PolicyOverride  `json:"applied_overrides"`
	ResolutionReason string             `json:"resolution_reason"`
	EffectiveFrom    time.Time          `json:"effective_from"`
	EffectiveUntil   *time.Time         `json:"effective_until,omitempty"`
}

// PolicyPrecedence contains policy precedence calculation results.
type PolicyPrecedence struct {
	Path            string              `json:"path"`
	OrderedPolicies []*PrecedencePolicy `json:"ordered_policies"`
	Resolution      string              `json:"resolution"`
	Conflicts       []*PolicyConflict   `json:"conflicts"`
}

// PrecedencePolicy represents a policy with precedence information.
type PrecedencePolicy struct {
	Policy    *RetentionPolicy `json:"policy"`
	Score     float64          `json:"score"`
	Reasoning []string         `json:"reasoning"`
	IsActive  bool             `json:"is_active"`
}

// PolicyConflict represents a conflict between policies.
type PolicyConflict struct {
	ConflictType string             `json:"conflict_type"`
	Policies     []*RetentionPolicy `json:"policies"`
	Description  string             `json:"description"`
	Resolution   string             `json:"resolution"`
	Severity     string             `json:"severity"`
}

// PolicyAdjustment contains results of dynamic policy adjustment.
type PolicyAdjustment struct {
	OriginalPolicyID string           `json:"original_policy_id"`
	AdjustedPolicy   *RetentionPolicy `json:"adjusted_policy"`
	Adjustments      []*Adjustment    `json:"adjustments"`
	Reason           string           `json:"reason"`
	TemporaryUntil   *time.Time       `json:"temporary_until,omitempty"`
	ApprovedBy       string           `json:"approved_by,omitempty"`
}

// Adjustment represents a specific policy adjustment.
type Adjustment struct {
	Type      string      `json:"type"`
	Field     string      `json:"field"`
	OldValue  interface{} `json:"old_value"`
	NewValue  interface{} `json:"new_value"`
	Reasoning string      `json:"reasoning"`
}

// PolicyRecommendation contains policy change recommendations.
type PolicyRecommendation struct {
	Path                string                `json:"path"`
	CurrentPolicies     []*RetentionPolicy    `json:"current_policies"`
	RecommendedChanges  []*RecommendedChange  `json:"recommended_changes"`
	EstimatedImpact     *RecommendationImpact `json:"estimated_impact"`
	Priority            string                `json:"priority"`
	Reasoning           []string              `json:"reasoning"`
	ImplementationSteps []string              `json:"implementation_steps"`
}

// RecommendedChange represents a recommended policy change.
type RecommendedChange struct {
	ChangeType       string      `json:"change_type"` // "create", "update", "delete", "adjust"
	PolicyID         string      `json:"policy_id,omitempty"`
	Field            string      `json:"field,omitempty"`
	CurrentValue     interface{} `json:"current_value,omitempty"`
	RecommendedValue interface{} `json:"recommended_value"`
	Priority         string      `json:"priority"`
	Reasoning        string      `json:"reasoning"`
}

// RecommendationImpact represents the estimated impact of recommendations.
type RecommendationImpact struct {
	EstimatedSpaceFreedGB int64         `json:"estimated_space_freed_gb"`
	RiskLevel             string        `json:"risk_level"`
	ImplementationTime    time.Duration `json:"implementation_time"`
	MaintenanceOverhead   string        `json:"maintenance_overhead"`
	ComplianceImprovement float64       `json:"compliance_improvement"`
}

// PolicyOverride allows temporary policy modifications.
type PolicyOverride struct {
	ID             string                 `json:"id"`
	PolicyID       string                 `json:"policy_id"`
	RepositoryPath string                 `json:"repository_path"`
	OverrideType   string                 `json:"override_type"` // "suspend", "modify", "extend"
	Modifications  map[string]interface{} `json:"modifications,omitempty"`
	Reason         string                 `json:"reason"`
	RequestedBy    string                 `json:"requested_by"`
	ApprovedBy     string                 `json:"approved_by"`
	ValidFrom      time.Time              `json:"valid_from"`
	ValidUntil     time.Time              `json:"valid_until"`
	Status         string                 `json:"status"`
	CreatedAt      time.Time              `json:"created_at"`
}

// PolicyOverrideResult contains result of override operations.
type PolicyOverrideResult struct {
	OverrideID       string                          `json:"override_id"`
	Status           string                          `json:"status"`
	Message          string                          `json:"message"`
	ValidationErrors []PolicyOverrideValidationError `json:"validation_errors,omitempty"`
	EffectivePolicy  *RetentionPolicy                `json:"effective_policy,omitempty"`
}

// PolicyOverrideValidationError represents validation errors in overrides.
type PolicyOverrideValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}
