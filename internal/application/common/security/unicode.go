package security

import (
	"strings"
	"unicode"
)

// UnicodeValidator provides optimized Unicode attack detection.
type UnicodeValidator struct {
	config *Config
}

// NewUnicodeValidator creates a new Unicode validator.
func NewUnicodeValidator(config *Config) *UnicodeValidator {
	return &UnicodeValidator{
		config: config,
	}
}

// ValidateUnicodeAttacks checks for various Unicode-based attacks.
func (uv *UnicodeValidator) ValidateUnicodeAttacks(input string) error {
	if !uv.config.EnableUnicodeCheck {
		return nil
	}

	if uv.containsDirectionalOverride(input) {
		return &ViolationError{
			Type:    "unicode_directional_override",
			Message: "Directional override attack detected",
			Field:   "",
		}
	}

	if uv.containsCombiningMarks(input) {
		return &ViolationError{
			Type:    "unicode_combining_marks",
			Message: "Combining marks attack detected",
			Field:   "",
		}
	}

	if uv.containsHomographAttack(input) {
		return &ViolationError{
			Type:    "unicode_homograph",
			Message: "Homograph attack detected",
			Field:   "",
		}
	}

	if uv.containsZeroWidthAttack(input) {
		return &ViolationError{
			Type:    "unicode_zero_width",
			Message: "Zero-width character attack detected",
			Field:   "",
		}
	}

	return nil
}

// containsDirectionalOverride checks for Unicode directional override attacks.
func (uv *UnicodeValidator) containsDirectionalOverride(input string) bool {
	directionalOverrides := []rune{
		'\u202D', // Left-to-Right Override
		'\u202E', // Right-to-Left Override
		'\u061C', // Arabic Letter Mark
		'\u200E', // Left-to-Right Mark
		'\u200F', // Right-to-Left Mark
	}

	for _, char := range input {
		for _, override := range directionalOverrides {
			if char == override {
				return true
			}
		}
	}

	return false
}

// containsCombiningMarks checks for combining mark attacks.
func (uv *UnicodeValidator) containsCombiningMarks(input string) bool {
	for _, char := range input {
		if unicode.In(char, unicode.Mn, unicode.Me, unicode.Mc) {
			return true
		}
	}
	return false
}

// containsHomographAttack detects homograph attacks (mixed scripts).
func (uv *UnicodeValidator) containsHomographAttack(input string) bool {
	scripts := make(map[string]bool)

	for _, char := range input {
		// Skip common characters that appear in multiple scripts
		if unicode.IsDigit(char) || unicode.IsPunct(char) || unicode.IsSpace(char) {
			continue
		}

		// Check specific Unicode ranges for different scripts
		switch {
		case unicode.In(char, unicode.Latin):
			scripts["latin"] = true
		case unicode.In(char, unicode.Cyrillic):
			scripts["cyrillic"] = true
		case unicode.In(char, unicode.Greek):
			scripts["greek"] = true
		case unicode.In(char, unicode.Arabic):
			scripts["arabic"] = true
		case unicode.In(char, unicode.Hebrew):
			scripts["hebrew"] = true
		case unicode.In(char, unicode.Han):
			scripts["han"] = true
		case unicode.In(char, unicode.Hiragana):
			scripts["hiragana"] = true
		case unicode.In(char, unicode.Katakana):
			scripts["katakana"] = true
		}

		// If we have more than one script, it's potentially suspicious
		if len(scripts) > 1 {
			return true
		}
	}

	// Also check for specific confusable characters
	return uv.containsConfusableCharacters(input)
}

// containsConfusableCharacters checks for visually similar characters.
func (uv *UnicodeValidator) containsConfusableCharacters(input string) bool {
	// Common confusable characters that could be used in attacks
	confusables := map[rune]bool{
		'а': true, // Cyrillic 'a' (U+0430) looks like Latin 'a'
		'е': true, // Cyrillic 'e' (U+0435) looks like Latin 'e'
		'о': true, // Cyrillic 'o' (U+043E) looks like Latin 'o'
		'р': true, // Cyrillic 'p' (U+0440) looks like Latin 'p'
		'с': true, // Cyrillic 'c' (U+0441) looks like Latin 'c'
		'х': true, // Cyrillic 'x' (U+0445) looks like Latin 'x'
		'у': true, // Cyrillic 'y' (U+0443) looks like Latin 'y'
		'ρ': true, // Greek rho (U+03C1) looks like Latin 'p'
		'ο': true, // Greek omicron (U+03BF) looks like Latin 'o'
		'α': true, // Greek alpha (U+03B1) looks like Latin 'a'
	}

	for _, char := range input {
		if confusables[char] {
			return true
		}
	}

	return false
}

// containsZeroWidthAttack checks for zero-width character attacks.
func (uv *UnicodeValidator) containsZeroWidthAttack(input string) bool {
	zeroWidthChars := []rune{
		'\u200B', // Zero Width Space
		'\u200C', // Zero Width Non-Joiner
		'\u200D', // Zero Width Joiner
		'\u2060', // Word Joiner
		'\uFEFF', // Zero Width No-Break Space
	}

	for _, char := range input {
		for _, zw := range zeroWidthChars {
			if char == zw {
				return true
			}
		}
	}

	return false
}

// NormalizeUnicode normalizes Unicode input for consistent processing.
func (uv *UnicodeValidator) NormalizeUnicode(input string) string {
	// Remove directional override characters
	normalized := uv.removeDirectionalOverrides(input)

	// Remove zero-width characters
	normalized = uv.removeZeroWidthCharacters(normalized)

	// Normalize whitespace
	normalized = strings.ReplaceAll(normalized, "\u00A0", " ") // Non-breaking space
	normalized = strings.ReplaceAll(normalized, "\u2009", " ") // Thin space
	normalized = strings.ReplaceAll(normalized, "\u202F", " ") // Narrow no-break space

	return normalized
}

// removeDirectionalOverrides removes directional override characters.
func (uv *UnicodeValidator) removeDirectionalOverrides(input string) string {
	var result strings.Builder

	for _, char := range input {
		// Skip directional override characters
		if char == '\u202D' || char == '\u202E' || char == '\u061C' ||
			char == '\u200E' || char == '\u200F' {
			continue
		}
		result.WriteRune(char)
	}

	return result.String()
}

// removeZeroWidthCharacters removes zero-width characters.
func (uv *UnicodeValidator) removeZeroWidthCharacters(input string) string {
	var result strings.Builder

	for _, char := range input {
		// Skip zero-width characters
		if char == '\u200B' || char == '\u200C' || char == '\u200D' ||
			char == '\u2060' || char == '\uFEFF' {
			continue
		}
		result.WriteRune(char)
	}

	return result.String()
}

// ContainsSuspiciousUnicode is a convenience function for backward compatibility.
func ContainsSuspiciousUnicode(input string) bool {
	validator := NewUnicodeValidator(DefaultConfig())
	return validator.ValidateUnicodeAttacks(input) != nil
}

// ViolationError represents a security violation.
type ViolationError struct {
	Type    string
	Message string
	Field   string
	Value   string
}

func (sv *ViolationError) Error() string {
	if sv.Field != "" {
		return sv.Message + " in field: " + sv.Field
	}
	return sv.Message
}

// GetUnicodeStats returns statistics about Unicode usage in the input.
func (uv *UnicodeValidator) GetUnicodeStats(input string) UnicodeStats {
	stats := UnicodeStats{
		Length:     len(input),
		RuneCount:  len([]rune(input)),
		Scripts:    make(map[string]int),
		Categories: make(map[string]int),
	}

	for _, char := range input {
		// Count by script
		switch {
		case unicode.In(char, unicode.Latin):
			stats.Scripts["Latin"]++
		case unicode.In(char, unicode.Cyrillic):
			stats.Scripts["Cyrillic"]++
		case unicode.In(char, unicode.Greek):
			stats.Scripts["Greek"]++
		case unicode.In(char, unicode.Arabic):
			stats.Scripts["Arabic"]++
		case unicode.In(char, unicode.Hebrew):
			stats.Scripts["Hebrew"]++
		case unicode.In(char, unicode.Han):
			stats.Scripts["Han"]++
		case unicode.In(char, unicode.Hiragana):
			stats.Scripts["Hiragana"]++
		case unicode.In(char, unicode.Katakana):
			stats.Scripts["Katakana"]++
		default:
			stats.Scripts["Other"]++
		}

		// Count by category
		switch {
		case unicode.IsLetter(char):
			stats.Categories["Letter"]++
		case unicode.IsDigit(char):
			stats.Categories["Digit"]++
		case unicode.IsPunct(char):
			stats.Categories["Punctuation"]++
		case unicode.IsSpace(char):
			stats.Categories["Space"]++
		case unicode.IsControl(char):
			stats.Categories["Control"]++
		case unicode.IsMark(char):
			stats.Categories["Mark"]++
		default:
			stats.Categories["Other"]++
		}
	}

	return stats
}

// UnicodeStats contains statistics about Unicode usage.
type UnicodeStats struct {
	Length     int            // Byte length
	RuneCount  int            // Rune count
	Scripts    map[string]int // Count by Unicode script
	Categories map[string]int // Count by Unicode category
}
