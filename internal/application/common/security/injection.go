package security

import (
	"net/url"
	"strings"
	"unicode"
)

// InjectionValidator provides consolidated XSS and SQL injection detection
type InjectionValidator struct {
	config   *Config
	patterns *Patterns
}

// NewInjectionValidator creates a new injection validator
func NewInjectionValidator(config *Config) *InjectionValidator {
	return &InjectionValidator{
		config:   config,
		patterns: GetPatterns(),
	}
}

// ValidateXSSAttacks checks for XSS attack patterns
func (iv *InjectionValidator) ValidateXSSAttacks(input string) error {
	if !iv.config.EnableXSSProtection {
		return nil
	}

	// Normalize input for consistent checking
	normalized := strings.ToLower(input)

	// Check for script tags and JavaScript
	if iv.patterns.XSSScript.MatchString(normalized) {
		return &SecurityViolation{
			Type:    "xss_script",
			Message: "Script injection detected",
		}
	}

	// Check for event handlers
	if iv.patterns.XSSEvent.MatchString(normalized) {
		return &SecurityViolation{
			Type:    "xss_event",
			Message: "Event handler injection detected",
		}
	}

	// Check for malicious protocols
	if iv.patterns.XSSProtocol.MatchString(input) {
		return &SecurityViolation{
			Type:    "xss_protocol",
			Message: "Malicious protocol detected",
		}
	}

	// Check for HTML entities that could be XSS
	if iv.patterns.XSSEntity.MatchString(normalized) {
		return &SecurityViolation{
			Type:    "xss_entity",
			Message: "Suspicious HTML entity detected",
		}
	}

	// Check string patterns for performance
	for _, pattern := range StringPatterns.XSSPatterns {
		if strings.Contains(normalized, pattern) {
			return &SecurityViolation{
				Type:    "xss_pattern",
				Message: "XSS pattern detected",
			}
		}
	}

	// Check URL-encoded XSS attempts
	if strings.Contains(input, "%") {
		if decoded, err := url.QueryUnescape(input); err == nil && decoded != input {
			if nestedErr := iv.ValidateXSSAttacks(decoded); nestedErr != nil {
				return &SecurityViolation{
					Type:    "xss_encoded",
					Message: "URL-encoded XSS detected",
				}
			}
		}
	}

	// Check for hex-encoded XSS patterns
	if iv.containsHexEncodedXSS(input) {
		return &SecurityViolation{
			Type:    "xss_hex_encoded",
			Message: "Hex-encoded XSS detected",
		}
	}

	return nil
}

// ValidateSQLInjection checks for SQL injection patterns
func (iv *InjectionValidator) ValidateSQLInjection(input string) error {
	if !iv.config.EnableSQLInjection {
		return nil
	}

	// Normalize input for consistent checking
	normalized := strings.ToLower(input)

	// Check for SQL keywords
	if iv.patterns.SQLKeywords.MatchString(normalized) {
		return &SecurityViolation{
			Type:    "sql_keywords",
			Message: "SQL keywords detected",
		}
	}

	// Check for SQL comments
	if iv.patterns.SQLComment.MatchString(input) {
		return &SecurityViolation{
			Type:    "sql_comment",
			Message: "SQL comment detected",
		}
	}

	// Check for SQL quotes
	if iv.patterns.SQLQuotes.MatchString(input) {
		return &SecurityViolation{
			Type:    "sql_quotes",
			Message: "SQL quote pattern detected",
		}
	}

	// Check for UNION attacks
	if iv.patterns.SQLUnion.MatchString(normalized) {
		return &SecurityViolation{
			Type:    "sql_union",
			Message: "SQL UNION attack detected",
		}
	}

	// Check string patterns for performance
	for _, pattern := range StringPatterns.SQLPatterns {
		if strings.Contains(normalized, pattern) {
			return &SecurityViolation{
				Type:    "sql_pattern",
				Message: "SQL injection pattern detected",
			}
		}
	}

	// Check URL-encoded SQL attempts
	if strings.Contains(input, "%27") || strings.Contains(input, "%3B") || strings.Contains(input, "%20") {
		if decoded, err := url.QueryUnescape(input); err == nil && decoded != input {
			if nestedErr := iv.ValidateSQLInjection(decoded); nestedErr != nil {
				return &SecurityViolation{
					Type:    "sql_encoded",
					Message: "URL-encoded SQL injection detected",
				}
			}
		}
	}

	return nil
}

// ValidatePathTraversal checks for path traversal attacks
func (iv *InjectionValidator) ValidatePathTraversal(input string) error {
	if !iv.config.EnablePathTraversal {
		return nil
	}

	// Check regex patterns
	if iv.patterns.PathTraversal.MatchString(input) {
		return &SecurityViolation{
			Type:    "path_traversal",
			Message: "Path traversal detected",
		}
	}

	if iv.patterns.PathEncoded.MatchString(strings.ToLower(input)) {
		return &SecurityViolation{
			Type:    "path_encoded",
			Message: "Encoded path traversal detected",
		}
	}

	// Check string patterns
	normalized := strings.ToLower(input)
	for _, pattern := range StringPatterns.PathPatterns {
		if strings.Contains(normalized, strings.ToLower(pattern)) {
			return &SecurityViolation{
				Type:    "path_pattern",
				Message: "Path traversal pattern detected",
			}
		}
	}

	// Check URL-decoded version
	if strings.Contains(input, "%") {
		if decoded, err := url.QueryUnescape(input); err == nil && decoded != input {
			if nestedErr := iv.ValidatePathTraversal(decoded); nestedErr != nil {
				return &SecurityViolation{
					Type:    "path_traversal_encoded",
					Message: "URL-encoded path traversal detected",
				}
			}
		}
	}

	return nil
}

// ValidateControlCharacters checks for control character injection
func (iv *InjectionValidator) ValidateControlCharacters(input string) error {
	if !iv.config.EnableControlCharCheck {
		return nil
	}

	// Check with regex for performance
	if iv.patterns.ControlChars.MatchString(input) {
		return &SecurityViolation{
			Type:    "control_chars",
			Message: "Control characters detected",
		}
	}

	// Check for URL-encoded control characters
	normalized := strings.ToLower(input)
	for _, encoding := range StringPatterns.ControlEncodings {
		if strings.Contains(normalized, encoding) {
			return &SecurityViolation{
				Type:    "control_chars_encoded",
				Message: "URL-encoded control characters detected",
			}
		}
	}

	// Additional check for Unicode control characters
	for _, r := range input {
		if unicode.IsControl(r) && r != '\t' {
			return &SecurityViolation{
				Type:    "control_chars_unicode",
				Message: "Unicode control characters detected",
			}
		}
	}

	return nil
}

// ValidateProtocolAttacks checks for malicious protocol attacks
func (iv *InjectionValidator) ValidateProtocolAttacks(input string) error {
	normalized := strings.ToLower(strings.TrimSpace(input))

	// Check for dangerous protocols
	dangerousProtocols := []string{
		"javascript:",
		"data:",
		"vbscript:",
		"file:",
		"ftp:",
		"about:",
	}

	for _, protocol := range dangerousProtocols {
		if strings.HasPrefix(normalized, protocol) {
			return &SecurityViolation{
				Type:    "protocol_attack",
				Message: "Malicious protocol detected: " + protocol,
			}
		}
	}

	// Check for URL-encoded protocols
	if strings.Contains(input, "%") {
		if decoded, err := url.QueryUnescape(input); err == nil && decoded != input {
			if nestedErr := iv.ValidateProtocolAttacks(decoded); nestedErr != nil {
				return &SecurityViolation{
					Type:    "protocol_attack_encoded",
					Message: "URL-encoded protocol attack detected",
				}
			}
		}
	}

	return nil
}

// ValidateAllInjections performs comprehensive injection validation
func (iv *InjectionValidator) ValidateAllInjections(input string) error {
	// Run all injection validations
	validations := []func(string) error{
		iv.ValidateXSSAttacks,
		iv.ValidateSQLInjection,
		iv.ValidatePathTraversal,
		iv.ValidateControlCharacters,
		iv.ValidateProtocolAttacks,
	}

	for _, validate := range validations {
		if err := validate(input); err != nil {
			return err
		}
	}

	return nil
}

// containsHexEncodedXSS checks for hex-encoded XSS patterns
func (iv *InjectionValidator) containsHexEncodedXSS(input string) bool {
	hexPatterns := []string{
		"\\x3c", "\\x3e", "\\x27", "\\x22", // <, >, ', "
		"\\x2f", "\\x5c", "\\x3b", // /, \, ;
	}

	inputLower := strings.ToLower(input)
	for _, pattern := range hexPatterns {
		if strings.Contains(inputLower, pattern) {
			return true
		}
	}

	return false
}

// SanitizeInput removes or escapes potentially dangerous content
func (iv *InjectionValidator) SanitizeInput(input string) string {
	// First normalize common encodings
	sanitized := input

	// Remove null bytes
	sanitized = strings.ReplaceAll(sanitized, "\x00", "")

	// Escape HTML special characters
	sanitized = strings.ReplaceAll(sanitized, "&", "&amp;")
	sanitized = strings.ReplaceAll(sanitized, "<", "&lt;")
	sanitized = strings.ReplaceAll(sanitized, ">", "&gt;")
	sanitized = strings.ReplaceAll(sanitized, "\"", "&quot;")
	sanitized = strings.ReplaceAll(sanitized, "'", "&#39;")

	// Remove or escape SQL dangerous characters
	sanitized = strings.ReplaceAll(sanitized, ";", "")
	sanitized = strings.ReplaceAll(sanitized, "--", "")
	sanitized = strings.ReplaceAll(sanitized, "/*", "")
	sanitized = strings.ReplaceAll(sanitized, "*/", "")

	return sanitized
}

// GetInjectionRisk assesses the risk level of input
func (iv *InjectionValidator) GetInjectionRisk(input string) InjectionRisk {
	risk := InjectionRisk{
		Input:       input,
		RiskLevel:   "LOW",
		Violations:  []string{},
		Suggestions: []string{},
	}

	// Check each type of injection
	if err := iv.ValidateXSSAttacks(input); err != nil {
		risk.RiskLevel = "HIGH"
		risk.Violations = append(risk.Violations, "XSS: "+err.Error())
		risk.Suggestions = append(risk.Suggestions, "Remove or escape HTML/JavaScript content")
	}

	if err := iv.ValidateSQLInjection(input); err != nil {
		risk.RiskLevel = "HIGH"
		risk.Violations = append(risk.Violations, "SQL: "+err.Error())
		risk.Suggestions = append(risk.Suggestions, "Use parameterized queries and escape SQL characters")
	}

	if err := iv.ValidatePathTraversal(input); err != nil {
		if risk.RiskLevel != "HIGH" {
			risk.RiskLevel = "MEDIUM"
		}
		risk.Violations = append(risk.Violations, "Path: "+err.Error())
		risk.Suggestions = append(risk.Suggestions, "Validate and sanitize file paths")
	}

	if err := iv.ValidateControlCharacters(input); err != nil {
		if risk.RiskLevel == "LOW" {
			risk.RiskLevel = "MEDIUM"
		}
		risk.Violations = append(risk.Violations, "Control: "+err.Error())
		risk.Suggestions = append(risk.Suggestions, "Remove control characters")
	}

	return risk
}

// InjectionRisk represents the risk assessment of input
type InjectionRisk struct {
	Input       string
	RiskLevel   string // LOW, MEDIUM, HIGH
	Violations  []string
	Suggestions []string
}

// Convenience functions for backward compatibility

// ContainsXSSContent checks for XSS patterns (backward compatibility)
func ContainsXSSContent(input string) bool {
	validator := NewInjectionValidator(DefaultConfig())
	return validator.ValidateXSSAttacks(input) != nil
}

// ContainsSQLInjectionContent checks for SQL injection patterns (backward compatibility)
func ContainsSQLInjectionContent(input string) bool {
	validator := NewInjectionValidator(DefaultConfig())
	return validator.ValidateSQLInjection(input) != nil
}

// ContainsPathTraversal checks for path traversal patterns (backward compatibility)
func ContainsPathTraversal(input string) bool {
	validator := NewInjectionValidator(DefaultConfig())
	return validator.ValidatePathTraversal(input) != nil
}

// ContainsControlCharacters checks for control characters (backward compatibility)
func ContainsControlCharacters(input string) bool {
	validator := NewInjectionValidator(DefaultConfig())
	return validator.ValidateControlCharacters(input) != nil
}
