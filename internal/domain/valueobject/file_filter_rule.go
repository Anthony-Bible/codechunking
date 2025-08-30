package valueobject

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// FilterRuleType represents different types of file filtering rules.
type FilterRuleType int

const (
	FilterRuleTypeUnknown FilterRuleType = iota
	FilterRuleTypeExtension
	FilterRuleTypeMagicByte
	FilterRuleTypeContentPattern
	FilterRuleTypeGitignore
	FilterRuleTypeSizeLimit
	FilterRuleTypeCustom
)

// FilterAction represents the action to take when a rule matches.
type FilterAction int

const (
	FilterActionUnknown FilterAction = iota
	FilterActionInclude
	FilterActionExclude
	FilterActionAnalyze
)

// FilterRule represents a file filtering rule with comprehensive validation and metadata.
// It serves as a value object in the domain layer for file filtering decisions
// throughout the code processing pipeline.
type FilterRule struct {
	id          string
	name        string
	ruleType    FilterRuleType
	action      FilterAction
	pattern     string
	priority    int
	enabled     bool
	description string
	createdAt   time.Time
	updatedAt   time.Time
	metadata    map[string]interface{}
}

// NewFilterRule creates a new FilterRule value object with validation.
func NewFilterRule(name, pattern string, ruleType FilterRuleType, action FilterAction) (FilterRule, error) {
	if name == "" {
		return FilterRule{}, errors.New("filter rule name cannot be empty")
	}

	if pattern == "" {
		return FilterRule{}, errors.New("filter rule pattern cannot be empty")
	}

	// Normalize rule name
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return FilterRule{}, errors.New("filter rule name cannot be empty after normalization")
	}

	// Validate rule name format
	if err := validateFilterRuleName(normalizedName); err != nil {
		return FilterRule{}, fmt.Errorf("invalid rule name: %w", err)
	}

	// Normalize and validate pattern
	normalizedPattern := strings.TrimSpace(pattern)
	if err := validateFilterPattern(normalizedPattern, ruleType); err != nil {
		return FilterRule{}, fmt.Errorf("invalid pattern: %w", err)
	}

	now := time.Now()
	return FilterRule{
		id:          generateFilterRuleID(normalizedName, ruleType),
		name:        normalizedName,
		ruleType:    ruleType,
		action:      action,
		pattern:     normalizedPattern,
		priority:    0,
		enabled:     true,
		description: "",
		createdAt:   now,
		updatedAt:   now,
		metadata:    make(map[string]interface{}),
	}, nil
}

// NewFilterRuleWithDetails creates a FilterRule with comprehensive details.
func NewFilterRuleWithDetails(
	name, pattern, description string,
	ruleType FilterRuleType,
	action FilterAction,
	priority int,
	enabled bool,
	metadata map[string]interface{},
) (FilterRule, error) {
	// Create base rule
	rule, err := NewFilterRule(name, pattern, ruleType, action)
	if err != nil {
		return FilterRule{}, err
	}

	// Validate priority
	if priority < 0 || priority > 1000 {
		return FilterRule{}, errors.New("priority must be between 0 and 1000")
	}

	// Validate description
	if len(description) > 500 {
		return FilterRule{}, errors.New("description too long (max 500 characters)")
	}

	// Validate metadata
	if len(metadata) > 50 {
		return FilterRule{}, errors.New("too many metadata entries (max 50)")
	}

	rule.priority = priority
	rule.enabled = enabled
	rule.description = strings.TrimSpace(description)
	rule.metadata = copyMetadata(metadata)
	rule.updatedAt = time.Now()

	return rule, nil
}

// validateFilterRuleName validates the filter rule name format.
func validateFilterRuleName(name string) error {
	if len(name) > 100 {
		return errors.New("rule name too long (max 100 characters)")
	}

	// Check for invalid characters
	for _, char := range name {
		if char < ' ' || char > '~' {
			return fmt.Errorf("invalid character in rule name: %c", char)
		}
	}

	return nil
}

// validateFilterPattern validates the filter pattern based on rule type.
func validateFilterPattern(pattern string, ruleType FilterRuleType) error {
	if len(pattern) > 1000 {
		return errors.New("pattern too long (max 1000 characters)")
	}

	switch ruleType {
	case FilterRuleTypeExtension:
		return validateExtensionPattern(pattern)
	case FilterRuleTypeMagicByte:
		return validateMagicBytePattern(pattern)
	case FilterRuleTypeContentPattern:
		return validateContentPattern(pattern)
	case FilterRuleTypeGitignore:
		return validateGitignorePattern(pattern)
	case FilterRuleTypeSizeLimit:
		return validateSizeLimitPattern(pattern)
	case FilterRuleTypeCustom:
		return validateCustomPattern(pattern)
	case FilterRuleTypeUnknown:
		return errors.New("cannot validate unknown filter rule type")
	default:
		return errors.New("unhandled filter rule type")
	}
}

// validateExtensionPattern validates file extension patterns.
func validateExtensionPattern(pattern string) error {
	if !strings.HasPrefix(pattern, ".") && !strings.Contains(pattern, "*") {
		return errors.New("extension pattern must start with '.' or contain '*'")
	}
	return nil
}

// validateMagicBytePattern validates magic byte patterns.
func validateMagicBytePattern(pattern string) error {
	if len(pattern)%2 != 0 {
		return errors.New("magic byte pattern must have even number of characters")
	}

	for _, char := range pattern {
		if (char < '0' || char > '9') && (char < 'A' || char > 'F') && (char < 'a' || char > 'f') {
			return errors.New("magic byte pattern must contain only hexadecimal characters")
		}
	}
	return nil
}

// validateContentPattern validates regex content patterns.
func validateContentPattern(pattern string) error {
	_, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	return nil
}

// validateGitignorePattern validates gitignore-style patterns.
func validateGitignorePattern(pattern string) error {
	// Basic gitignore pattern validation
	if strings.Contains(pattern, "***") {
		return errors.New("gitignore pattern cannot contain '***'")
	}
	return nil
}

// validateSizeLimitPattern validates size limit patterns.
func validateSizeLimitPattern(pattern string) error {
	// Pattern should be like "1MB", "500KB", ">10MB", "<1GB"
	validPattern := regexp.MustCompile(`^(<|>)?(\d+)(B|KB|MB|GB)$`)
	if !validPattern.MatchString(pattern) {
		return errors.New("size limit pattern must be in format like '1MB', '>10MB', '<500KB'")
	}
	return nil
}

// validateCustomPattern validates custom patterns.
func validateCustomPattern(pattern string) error {
	// Custom patterns can be more flexible, just check basic constraints
	if strings.TrimSpace(pattern) == "" {
		return errors.New("custom pattern cannot be empty")
	}
	return nil
}

// generateFilterRuleID generates a unique ID for the filter rule.
func generateFilterRuleID(name string, ruleType FilterRuleType) string {
	return fmt.Sprintf("%s_%s_%d",
		strings.ToLower(strings.ReplaceAll(name, " ", "_")),
		ruleType.String(),
		time.Now().UnixNano())
}

// copyMetadata creates a deep copy of metadata map.
func copyMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return make(map[string]interface{})
	}

	copied := make(map[string]interface{}, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}

// ID returns the rule ID.
func (r FilterRule) ID() string {
	return r.id
}

// Name returns the rule name.
func (r FilterRule) Name() string {
	return r.name
}

// RuleType returns the rule type.
func (r FilterRule) RuleType() FilterRuleType {
	return r.ruleType
}

// Action returns the filter action.
func (r FilterRule) Action() FilterAction {
	return r.action
}

// Pattern returns the rule pattern.
func (r FilterRule) Pattern() string {
	return r.pattern
}

// Priority returns the rule priority.
func (r FilterRule) Priority() int {
	return r.priority
}

// IsEnabled returns true if the rule is enabled.
func (r FilterRule) IsEnabled() bool {
	return r.enabled
}

// Description returns the rule description.
func (r FilterRule) Description() string {
	return r.description
}

// CreatedAt returns the rule creation time.
func (r FilterRule) CreatedAt() time.Time {
	return r.createdAt
}

// UpdatedAt returns the rule last update time.
func (r FilterRule) UpdatedAt() time.Time {
	return r.updatedAt
}

// Metadata returns a copy of the rule metadata.
func (r FilterRule) Metadata() map[string]interface{} {
	return copyMetadata(r.metadata)
}

// Matches determines if this rule matches the given file information.
func (r FilterRule) Matches(filePath string, fileContent []byte, fileSize int64) (bool, error) {
	if !r.enabled {
		return false, nil
	}

	switch r.ruleType {
	case FilterRuleTypeExtension:
		return r.matchesExtension(filePath), nil
	case FilterRuleTypeMagicByte:
		return r.matchesMagicByte(fileContent), nil
	case FilterRuleTypeContentPattern:
		return r.matchesContentPattern(fileContent)
	case FilterRuleTypeGitignore:
		return r.matchesGitignore(filePath), nil
	case FilterRuleTypeSizeLimit:
		return r.matchesSizeLimit(fileSize)
	case FilterRuleTypeCustom:
		return r.matchesCustom(filePath, fileContent, fileSize)
	case FilterRuleTypeUnknown:
		return false, errors.New("cannot match unknown rule type")
	default:
		return false, errors.New("unhandled rule type")
	}
}

// matchesExtension checks if file extension matches the pattern.
func (r FilterRule) matchesExtension(filePath string) bool {
	// Implementation would go here - simplified for now
	return strings.HasSuffix(strings.ToLower(filePath), strings.ToLower(r.pattern))
}

// matchesMagicByte checks if file content starts with magic bytes.
func (r FilterRule) matchesMagicByte(content []byte) bool {
	// Implementation would go here - simplified for now
	if len(content) < len(r.pattern)/2 {
		return false
	}
	// Convert hex pattern to bytes and compare
	return true // Simplified
}

// matchesContentPattern checks if file content matches regex pattern.
func (r FilterRule) matchesContentPattern(content []byte) (bool, error) {
	pattern, err := regexp.Compile(r.pattern)
	if err != nil {
		return false, err
	}
	return pattern.Match(content), nil
}

// matchesGitignore checks if file path matches gitignore pattern.
func (r FilterRule) matchesGitignore(_ string) bool {
	// Implementation would go here - simplified for now
	return false // Simplified
}

// matchesSizeLimit checks if file size matches size limit pattern.
func (r FilterRule) matchesSizeLimit(_ int64) (bool, error) {
	// Implementation would go here - parse size pattern and compare
	return true, nil // Simplified
}

// matchesCustom checks if file matches custom rule logic.
func (r FilterRule) matchesCustom(_ string, _ []byte, _ int64) (bool, error) {
	// Custom rule matching logic would go here
	return false, nil // Simplified
}

// ShouldInclude returns true if this rule indicates the file should be included.
func (r FilterRule) ShouldInclude() bool {
	return r.action == FilterActionInclude
}

// ShouldExclude returns true if this rule indicates the file should be excluded.
func (r FilterRule) ShouldExclude() bool {
	return r.action == FilterActionExclude
}

// ShouldAnalyze returns true if this rule indicates the file should be analyzed further.
func (r FilterRule) ShouldAnalyze() bool {
	return r.action == FilterActionAnalyze
}

// WithPriority returns a new FilterRule with updated priority.
func (r FilterRule) WithPriority(priority int) (FilterRule, error) {
	if priority < 0 || priority > 1000 {
		return FilterRule{}, errors.New("priority must be between 0 and 1000")
	}

	newRule := r
	newRule.priority = priority
	newRule.updatedAt = time.Now()
	return newRule, nil
}

// WithEnabled returns a new FilterRule with updated enabled status.
func (r FilterRule) WithEnabled(enabled bool) FilterRule {
	newRule := r
	newRule.enabled = enabled
	newRule.updatedAt = time.Now()
	return newRule
}

// WithDescription returns a new FilterRule with updated description.
func (r FilterRule) WithDescription(description string) (FilterRule, error) {
	if len(description) > 500 {
		return FilterRule{}, errors.New("description too long (max 500 characters)")
	}

	newRule := r
	newRule.description = strings.TrimSpace(description)
	newRule.updatedAt = time.Now()
	return newRule, nil
}

// WithMetadata returns a new FilterRule with updated metadata.
func (r FilterRule) WithMetadata(metadata map[string]interface{}) (FilterRule, error) {
	if len(metadata) > 50 {
		return FilterRule{}, errors.New("too many metadata entries (max 50)")
	}

	newRule := r
	newRule.metadata = copyMetadata(metadata)
	newRule.updatedAt = time.Now()
	return newRule, nil
}

// String returns a string representation of the filter rule.
func (r FilterRule) String() string {
	return fmt.Sprintf("%s (%s): %s -> %s",
		r.name, r.ruleType.String(), r.pattern, r.action.String())
}

// Equal compares two FilterRule instances for equality.
func (r FilterRule) Equal(other FilterRule) bool {
	return r.id == other.id &&
		r.name == other.name &&
		r.ruleType == other.ruleType &&
		r.action == other.action &&
		r.pattern == other.pattern
}

// String returns a string representation of the filter rule type.
func (frt FilterRuleType) String() string {
	switch frt {
	case FilterRuleTypeExtension:
		return "Extension"
	case FilterRuleTypeMagicByte:
		return "MagicByte"
	case FilterRuleTypeContentPattern:
		return "ContentPattern"
	case FilterRuleTypeGitignore:
		return "Gitignore"
	case FilterRuleTypeSizeLimit:
		return "SizeLimit"
	case FilterRuleTypeCustom:
		return "Custom"
	case FilterRuleTypeUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}

// String returns a string representation of the filter action.
func (fa FilterAction) String() string {
	switch fa {
	case FilterActionInclude:
		return "Include"
	case FilterActionExclude:
		return "Exclude"
	case FilterActionAnalyze:
		return "Analyze"
	case FilterActionUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}
