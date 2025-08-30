package valueobject

import (
	"strings"
	"testing"
	"time"
)

// RED PHASE TESTS - All tests should pass since they test the existing value object

func TestNewFilterRule_ValidInputs(t *testing.T) {
	tests := []struct {
		name     string
		ruleName string
		pattern  string
		ruleType FilterRuleType
		action   FilterAction
	}{
		{"Extension rule", "Binary Images", ".jpg", FilterRuleTypeExtension, FilterActionExclude},
		{"Magic byte rule", "PNG Detection", "89504E47", FilterRuleTypeMagicByte, FilterActionExclude},
		{"Content pattern", "Go Package", `^package\s+\w+`, FilterRuleTypeContentPattern, FilterActionInclude},
		{"Gitignore rule", "Node Modules", "node_modules/", FilterRuleTypeGitignore, FilterActionExclude},
		{"Size limit rule", "Large Files", ">10MB", FilterRuleTypeSizeLimit, FilterActionExclude},
		{"Custom rule", "Special Logic", "custom:logic", FilterRuleTypeCustom, FilterActionAnalyze},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewFilterRule(tt.ruleName, tt.pattern, tt.ruleType, tt.action)
			if err != nil {
				t.Fatalf("NewFilterRule() error = %v", err)
			}

			validateRuleProperties(t, rule, tt.ruleName, tt.pattern, tt.ruleType, tt.action)
			validateRuleDefaults(t, rule)
		})
	}
}

// validateRuleProperties validates the basic properties of a filter rule.
func validateRuleProperties(
	t *testing.T,
	rule FilterRule,
	expectedName, expectedPattern string,
	expectedType FilterRuleType,
	expectedAction FilterAction,
) {
	t.Helper()

	if rule.Name() != expectedName {
		t.Errorf("Name() = %v, want %v", rule.Name(), expectedName)
	}

	if rule.Pattern() != expectedPattern {
		t.Errorf("Pattern() = %v, want %v", rule.Pattern(), expectedPattern)
	}

	if rule.RuleType() != expectedType {
		t.Errorf("RuleType() = %v, want %v", rule.RuleType(), expectedType)
	}

	if rule.Action() != expectedAction {
		t.Errorf("Action() = %v, want %v", rule.Action(), expectedAction)
	}
}

// validateRuleDefaults validates the default values of a filter rule.
func validateRuleDefaults(t *testing.T, rule FilterRule) {
	t.Helper()

	if rule.Priority() != 0 {
		t.Errorf("Priority() = %v, want 0", rule.Priority())
	}

	if !rule.IsEnabled() {
		t.Error("IsEnabled() should be true by default")
	}

	if rule.Description() != "" {
		t.Errorf("Description() = %v, want empty", rule.Description())
	}

	if rule.ID() == "" {
		t.Error("ID() should not be empty")
	}

	// Metadata should be non-nil
	metadata := rule.Metadata()
	if metadata == nil {
		t.Error("Metadata() should not be nil")
	}
}

func TestNewFilterRule_InvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		ruleName    string
		pattern     string
		ruleType    FilterRuleType
		action      FilterAction
		expectedErr string
	}{
		{"Empty name", "", ".jpg", FilterRuleTypeExtension, FilterActionExclude, "filter rule name cannot be empty"},
		{
			"Empty pattern",
			"Test",
			"",
			FilterRuleTypeExtension,
			FilterActionExclude,
			"filter rule pattern cannot be empty",
		},
		{
			"Whitespace name",
			"   ",
			".jpg",
			FilterRuleTypeExtension,
			FilterActionExclude,
			"filter rule name cannot be empty after normalization",
		},
		{
			"Too long name",
			strings.Repeat("a", 101),
			".jpg",
			FilterRuleTypeExtension,
			FilterActionExclude,
			"rule name too long",
		},
		{
			"Invalid extension pattern",
			"Test",
			"jpg",
			FilterRuleTypeExtension,
			FilterActionExclude,
			"extension pattern must start with '.' or contain '*'",
		},
		{
			"Invalid magic byte pattern",
			"Test",
			"123",
			FilterRuleTypeMagicByte,
			FilterActionExclude,
			"magic byte pattern must have even number of characters",
		},
		{
			"Invalid regex pattern",
			"Test",
			"[",
			FilterRuleTypeContentPattern,
			FilterActionExclude,
			"invalid regex pattern",
		},
		{
			"Invalid size pattern",
			"Test",
			"invalid",
			FilterRuleTypeSizeLimit,
			FilterActionExclude,
			"size limit pattern must be in format",
		},
		{
			"Unknown rule type",
			"Test",
			".jpg",
			FilterRuleTypeUnknown,
			FilterActionExclude,
			"cannot validate unknown filter rule type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFilterRule(tt.ruleName, tt.pattern, tt.ruleType, tt.action)
			if err == nil {
				t.Error("NewFilterRule() expected error, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("NewFilterRule() error = %v, want to contain %v", err, tt.expectedErr)
			}
		})
	}
}

func TestNewFilterRuleWithDetails_ValidInputs(t *testing.T) {
	metadata := map[string]interface{}{
		"source":   "user_config",
		"category": "binary_files",
		"weight":   0.8,
	}

	rule, err := NewFilterRuleWithDetails(
		"Image Files",
		"*.{jpg,jpeg,png,gif}",
		"Filters out binary image files",
		FilterRuleTypeExtension,
		FilterActionExclude,
		50,
		true,
		metadata,
	)
	if err != nil {
		t.Fatalf("NewFilterRuleWithDetails() error = %v", err)
	}

	if rule.Name() != "Image Files" {
		t.Errorf("Name() = %v, want 'Image Files'", rule.Name())
	}

	if rule.Priority() != 50 {
		t.Errorf("Priority() = %v, want 50", rule.Priority())
	}

	if !rule.IsEnabled() {
		t.Error("IsEnabled() should be true")
	}

	if rule.Description() != "Filters out binary image files" {
		t.Errorf("Description() = %v, want 'Filters out binary image files'", rule.Description())
	}

	ruleMetadata := rule.Metadata()
	if len(ruleMetadata) != 3 {
		t.Errorf("Metadata length = %v, want 3", len(ruleMetadata))
	}

	if ruleMetadata["source"] != "user_config" {
		t.Errorf("Metadata[source] = %v, want 'user_config'", ruleMetadata["source"])
	}
}

func TestNewFilterRuleWithDetails_InvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		priority    int
		description string
		metadata    map[string]interface{}
		expectedErr string
	}{
		{"Invalid priority negative", -1, "desc", nil, "priority must be between 0 and 1000"},
		{"Invalid priority too high", 1001, "desc", nil, "priority must be between 0 and 1000"},
		{"Description too long", 100, strings.Repeat("a", 501), nil, "description too long"},
		{"Too much metadata", 100, "desc", createLargeMetadata(), "too many metadata entries"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFilterRuleWithDetails(
				"Test Rule",
				".test",
				tt.description,
				FilterRuleTypeExtension,
				FilterActionExclude,
				tt.priority,
				true,
				tt.metadata,
			)

			if err == nil {
				t.Error("NewFilterRuleWithDetails() expected error, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("NewFilterRuleWithDetails() error = %v, want to contain %v", err, tt.expectedErr)
			}
		})
	}
}

func TestFilterRule_Matches_Extension(t *testing.T) {
	rule, err := NewFilterRule("Image Filter", ".jpg", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{"Exact match", "photo.jpg", true},
		{"Case insensitive", "PHOTO.JPG", true},
		{"No match", "photo.png", false},
		{"Partial match", "jpgfile.txt", false},
		{"Extension in path", "path/to/file.jpg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := rule.Matches(tt.filePath, nil, 0)
			if err != nil {
				t.Fatalf("Matches() error = %v", err)
			}

			if matches != tt.expected {
				t.Errorf("Matches(%s) = %v, want %v", tt.filePath, matches, tt.expected)
			}
		})
	}
}

func TestFilterRule_Matches_ContentPattern(t *testing.T) {
	rule, err := NewFilterRule("Go Package", `^package\s+\w+`, FilterRuleTypeContentPattern, FilterActionInclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"Go package", "package main\n\nfunc main() {}", true},
		{"Go package with spaces", "package   utils", true},
		{"No match", "import fmt", false}, // Regex doesn't match import statement
		{"Empty content", "", false},      // Regex doesn't match empty content
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := rule.Matches("test.go", []byte(tt.content), int64(len(tt.content)))
			if err != nil {
				t.Fatalf("Matches() error = %v", err)
			}

			if matches != tt.expected {
				t.Errorf("Matches() = %v, want %v", matches, tt.expected)
			}
		})
	}
}

func TestFilterRule_Matches_Disabled(t *testing.T) {
	rule, err := NewFilterRule("Test Rule", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	// Disable the rule
	disabledRule := rule.WithEnabled(false)

	matches, err := disabledRule.Matches("file.test", nil, 0)
	if err != nil {
		t.Fatalf("Matches() error = %v", err)
	}

	if matches {
		t.Error("Disabled rule should not match")
	}
}

func TestFilterRule_ActionMethods(t *testing.T) {
	tests := []struct {
		action        FilterAction
		shouldIncl    bool
		shouldExcl    bool
		shouldAnalyze bool
	}{
		{FilterActionInclude, true, false, false},
		{FilterActionExclude, false, true, false},
		{FilterActionAnalyze, false, false, true},
		{FilterActionUnknown, false, false, false},
	}

	for _, tt := range tests {
		rule, err := NewFilterRule("Test", ".test", FilterRuleTypeExtension, tt.action)
		if err != nil {
			t.Fatalf("NewFilterRule() error = %v", err)
		}

		if rule.ShouldInclude() != tt.shouldIncl {
			t.Errorf("ShouldInclude() = %v, want %v", rule.ShouldInclude(), tt.shouldIncl)
		}

		if rule.ShouldExclude() != tt.shouldExcl {
			t.Errorf("ShouldExclude() = %v, want %v", rule.ShouldExclude(), tt.shouldExcl)
		}

		if rule.ShouldAnalyze() != tt.shouldAnalyze {
			t.Errorf("ShouldAnalyze() = %v, want %v", rule.ShouldAnalyze(), tt.shouldAnalyze)
		}
	}
}

func TestFilterRule_WithPriority(t *testing.T) {
	rule, err := NewFilterRule("Test", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	newRule, err := rule.WithPriority(100)
	if err != nil {
		t.Fatalf("WithPriority() error = %v", err)
	}

	if newRule.Priority() != 100 {
		t.Errorf("Priority() = %v, want 100", newRule.Priority())
	}

	// Original rule should be unchanged
	if rule.Priority() != 0 {
		t.Errorf("Original rule priority = %v, want 0", rule.Priority())
	}

	// Test invalid priority
	_, err = rule.WithPriority(-1)
	if err == nil {
		t.Error("WithPriority(-1) expected error, got nil")
	}

	_, err = rule.WithPriority(1001)
	if err == nil {
		t.Error("WithPriority(1001) expected error, got nil")
	}
}

func TestFilterRule_WithEnabled(t *testing.T) {
	rule, err := NewFilterRule("Test", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	disabledRule := rule.WithEnabled(false)
	if disabledRule.IsEnabled() {
		t.Error("WithEnabled(false) should disable rule")
	}

	// Original rule should be unchanged
	if !rule.IsEnabled() {
		t.Error("Original rule should remain enabled")
	}

	enabledRule := disabledRule.WithEnabled(true)
	if !enabledRule.IsEnabled() {
		t.Error("WithEnabled(true) should enable rule")
	}
}

func TestFilterRule_WithDescription(t *testing.T) {
	rule, err := NewFilterRule("Test", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	newRule, err := rule.WithDescription("New description")
	if err != nil {
		t.Fatalf("WithDescription() error = %v", err)
	}

	if newRule.Description() != "New description" {
		t.Errorf("Description() = %v, want 'New description'", newRule.Description())
	}

	// Original rule should be unchanged
	if rule.Description() != "" {
		t.Errorf("Original rule description = %v, want empty", rule.Description())
	}

	// Test description too long
	_, err = rule.WithDescription(strings.Repeat("a", 501))
	if err == nil {
		t.Error("WithDescription() with long description expected error, got nil")
	}
}

func TestFilterRule_WithMetadata(t *testing.T) {
	rule, err := NewFilterRule("Test", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	metadata := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	newRule, err := rule.WithMetadata(metadata)
	if err != nil {
		t.Fatalf("WithMetadata() error = %v", err)
	}

	newMetadata := newRule.Metadata()
	if len(newMetadata) != 3 {
		t.Errorf("Metadata length = %v, want 3", len(newMetadata))
	}

	if newMetadata["key1"] != "value1" {
		t.Errorf("Metadata[key1] = %v, want 'value1'", newMetadata["key1"])
	}

	// Original rule should be unchanged
	origMetadata := rule.Metadata()
	if len(origMetadata) != 0 {
		t.Errorf("Original rule metadata length = %v, want 0", len(origMetadata))
	}

	// Test too much metadata
	largeMetadata := createLargeMetadata()
	_, err = rule.WithMetadata(largeMetadata)
	if err == nil {
		t.Error("WithMetadata() with large metadata expected error, got nil")
	}
}

func TestFilterRule_String(t *testing.T) {
	rule, err := NewFilterRule("Image Filter", ".jpg", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	str := rule.String()
	expectedParts := []string{"Image Filter", "Extension", ".jpg", "Exclude"}

	for _, part := range expectedParts {
		if !strings.Contains(str, part) {
			t.Errorf("String() = %v, should contain %v", str, part)
		}
	}
}

func TestFilterRule_Equal(t *testing.T) {
	rule1, err := NewFilterRule("Test", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	rule2, err := NewFilterRule("Test", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	// Rules with same properties but different IDs should not be equal
	if rule1.Equal(rule2) {
		t.Error("Rules with different IDs should not be equal")
	}

	// Same rule should be equal to itself (reflexivity property)
	if !rule1.Equal(rule1) { //nolint:gocritic // Testing reflexivity property is intentional
		t.Error("Rule should be equal to itself")
	}

	rule3, err := NewFilterRule("Different", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}

	if rule1.Equal(rule3) {
		t.Error("Rules with different names should not be equal")
	}
}

func TestFilterRuleType_String(t *testing.T) {
	tests := []struct {
		ruleType FilterRuleType
		expected string
	}{
		{FilterRuleTypeExtension, "Extension"},
		{FilterRuleTypeMagicByte, "MagicByte"},
		{FilterRuleTypeContentPattern, "ContentPattern"},
		{FilterRuleTypeGitignore, "Gitignore"},
		{FilterRuleTypeSizeLimit, "SizeLimit"},
		{FilterRuleTypeCustom, "Custom"},
		{FilterRuleTypeUnknown, LanguageUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.ruleType.String() != tt.expected {
				t.Errorf("String() = %v, want %v", tt.ruleType.String(), tt.expected)
			}
		})
	}
}

func TestFilterAction_String(t *testing.T) {
	tests := []struct {
		action   FilterAction
		expected string
	}{
		{FilterActionInclude, "Include"},
		{FilterActionExclude, "Exclude"},
		{FilterActionAnalyze, "Analyze"},
		{FilterActionUnknown, LanguageUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.action.String() != tt.expected {
				t.Errorf("String() = %v, want %v", tt.action.String(), tt.expected)
			}
		})
	}
}

func TestFilterRule_CreatedAt_UpdatedAt(t *testing.T) {
	before := time.Now()
	rule, err := NewFilterRule("Test", ".test", FilterRuleTypeExtension, FilterActionExclude)
	if err != nil {
		t.Fatalf("NewFilterRule() error = %v", err)
	}
	after := time.Now()

	if rule.CreatedAt().Before(before) || rule.CreatedAt().After(after) {
		t.Errorf("CreatedAt() should be between %v and %v, got %v", before, after, rule.CreatedAt())
	}

	if rule.UpdatedAt().Before(before) || rule.UpdatedAt().After(after) {
		t.Errorf("UpdatedAt() should be between %v and %v, got %v", before, after, rule.UpdatedAt())
	}

	// Test that WithPriority updates UpdatedAt
	time.Sleep(1 * time.Millisecond) // Ensure time difference
	updatedRule, err := rule.WithPriority(100)
	if err != nil {
		t.Fatalf("WithPriority() error = %v", err)
	}

	if !updatedRule.UpdatedAt().After(rule.UpdatedAt()) {
		t.Error("WithPriority() should update UpdatedAt timestamp")
	}

	// CreatedAt should remain unchanged
	if !updatedRule.CreatedAt().Equal(rule.CreatedAt()) {
		t.Error("WithPriority() should not change CreatedAt timestamp")
	}
}

// Helper functions

func createLargeMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	for i := range 51 {
		metadata[string(rune('a'+i))] = i
	}
	return metadata
}

// Test validation functions directly

func TestValidateExtensionPattern(t *testing.T) {
	tests := []struct {
		pattern   string
		shouldErr bool
	}{
		{".jpg", false},
		{"*.jpg", false},
		{"jpg", true}, // Should start with . or contain *
		{"", true},    // Empty pattern
	}

	for _, tt := range tests {
		err := validateExtensionPattern(tt.pattern)
		if tt.shouldErr && err == nil {
			t.Errorf("validateExtensionPattern(%q) expected error, got nil", tt.pattern)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("validateExtensionPattern(%q) unexpected error: %v", tt.pattern, err)
		}
	}
}

func TestValidateMagicBytePattern(t *testing.T) {
	tests := []struct {
		pattern   string
		shouldErr bool
	}{
		{"89504E47", false},         // Valid hex
		{"123", true},               // Odd length
		{"GHIJ", true},              // Invalid hex chars
		{"1234567890ABCDEF", false}, // Valid long hex
		{"", false},                 // Empty is even length
	}

	for _, tt := range tests {
		err := validateMagicBytePattern(tt.pattern)
		if tt.shouldErr && err == nil {
			t.Errorf("validateMagicBytePattern(%q) expected error, got nil", tt.pattern)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("validateMagicBytePattern(%q) unexpected error: %v", tt.pattern, err)
		}
	}
}

func TestValidateContentPattern(t *testing.T) {
	tests := []struct {
		pattern   string
		shouldErr bool
	}{
		{`^package\s+\w+`, false}, // Valid regex
		{`[`, true},               // Invalid regex
		{`.*`, false},             // Simple regex
		{`\d+`, false},            // Number regex
	}

	for _, tt := range tests {
		err := validateContentPattern(tt.pattern)
		if tt.shouldErr && err == nil {
			t.Errorf("validateContentPattern(%q) expected error, got nil", tt.pattern)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("validateContentPattern(%q) unexpected error: %v", tt.pattern, err)
		}
	}
}

func TestValidateGitignorePattern(t *testing.T) {
	tests := []struct {
		pattern   string
		shouldErr bool
	}{
		{"*.log", false},         // Valid pattern
		{"node_modules/", false}, // Directory pattern
		{"***", true},            // Invalid triple asterisk
		{"**/*.js", false},       // Globstar pattern
	}

	for _, tt := range tests {
		err := validateGitignorePattern(tt.pattern)
		if tt.shouldErr && err == nil {
			t.Errorf("validateGitignorePattern(%q) expected error, got nil", tt.pattern)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("validateGitignorePattern(%q) unexpected error: %v", tt.pattern, err)
		}
	}
}

func TestValidateSizeLimitPattern(t *testing.T) {
	tests := []struct {
		pattern   string
		shouldErr bool
	}{
		{"1MB", false},    // Valid size
		{">10MB", false},  // Valid with operator
		{"<500KB", false}, // Valid with operator
		{"1GB", false},    // Valid large size
		{"invalid", true}, // Invalid format
		{"1TB", true},     // Unsupported unit
		{"MB", true},      // Missing number
	}

	for _, tt := range tests {
		err := validateSizeLimitPattern(tt.pattern)
		if tt.shouldErr && err == nil {
			t.Errorf("validateSizeLimitPattern(%q) expected error, got nil", tt.pattern)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("validateSizeLimitPattern(%q) unexpected error: %v", tt.pattern, err)
		}
	}
}
