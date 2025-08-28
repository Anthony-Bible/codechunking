package valueobject

import (
	"errors"
	"fmt"
	"strings"
)

// LanguageType represents different categories of programming languages.
type LanguageType int

const (
	LanguageTypeUnknown LanguageType = iota
	LanguageTypeCompiled
	LanguageTypeInterpreted
	LanguageTypeScripting
	LanguageTypeMarkup
	LanguageTypeData
	LanguageTypeConfig
	LanguageTypeDatabase
)

// DetectionMethod represents how a language was detected.
type DetectionMethod int

const (
	DetectionMethodUnknown DetectionMethod = iota
	DetectionMethodExtension
	DetectionMethodContent
	DetectionMethodShebang
	DetectionMethodHeuristic
	DetectionMethodFallback
)

// Language represents a programming language with comprehensive metadata.
// It serves as a value object in the domain layer for language identification
// and categorization throughout the code processing pipeline.
type Language struct {
	name            string
	aliases         []string
	extensions      []string
	languageType    LanguageType
	detectionMethod DetectionMethod
	confidence      float64 // 0.0 to 1.0
}

// Common programming languages as constants for consistency.
const (
	LanguageGo         = "Go"
	LanguagePython     = "Python"
	LanguageJavaScript = "JavaScript"
	LanguageTypeScript = "TypeScript"
	LanguageJava       = "Java"
	LanguageC          = "C"
	LanguageCPlusPlus  = "C++"
	LanguageRust       = "Rust"
	LanguageRuby       = "Ruby"
	LanguagePHP        = "PHP"
	LanguageHTML       = "HTML"
	LanguageCSS        = "CSS"
	LanguageSQL        = "SQL"
	LanguageShell      = "Shell"
	LanguageMarkdown   = "Markdown"
	LanguageJSON       = "JSON"
	LanguageYAML       = "YAML"
	LanguageXML        = "XML"
	LanguageUnknown    = "Unknown"
)

// NewLanguage creates a new Language value object with validation.
func NewLanguage(name string) (Language, error) {
	if name == "" {
		return Language{}, errors.New("language name cannot be empty")
	}

	// Normalize language name
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return Language{}, errors.New("language name cannot be empty after normalization")
	}

	// Validate language name format
	if err := validateLanguageName(normalizedName); err != nil {
		return Language{}, fmt.Errorf("invalid language name: %w", err)
	}

	return Language{
		name:            normalizedName,
		aliases:         []string{},
		extensions:      []string{},
		languageType:    LanguageTypeUnknown,
		detectionMethod: DetectionMethodUnknown,
		confidence:      0.0,
	}, nil
}

// NewLanguageWithDetails creates a Language with comprehensive details.
func NewLanguageWithDetails(
	name string,
	aliases []string,
	extensions []string,
	langType LanguageType,
	detectionMethod DetectionMethod,
	confidence float64,
) (Language, error) {
	// Create base language
	lang, err := NewLanguage(name)
	if err != nil {
		return Language{}, err
	}

	// Validate confidence score
	if confidence < 0.0 || confidence > 1.0 {
		return Language{}, errors.New("confidence score must be between 0.0 and 1.0")
	}

	// Validate and normalize aliases
	normalizedAliases, err := validateAndNormalizeAliases(aliases)
	if err != nil {
		return Language{}, fmt.Errorf("invalid aliases: %w", err)
	}

	// Validate and normalize extensions
	normalizedExtensions, err := validateAndNormalizeExtensions(extensions)
	if err != nil {
		return Language{}, fmt.Errorf("invalid extensions: %w", err)
	}

	lang.aliases = normalizedAliases
	lang.extensions = normalizedExtensions
	lang.languageType = langType
	lang.detectionMethod = detectionMethod
	lang.confidence = confidence

	return lang, nil
}

// validateLanguageName validates the language name format.
func validateLanguageName(name string) error {
	if len(name) > 100 {
		return errors.New("language name too long (max 100 characters)")
	}

	// Check for invalid characters
	for _, char := range name {
		if char < ' ' || char > '~' {
			return fmt.Errorf("invalid character in language name: %c", char)
		}
	}

	return nil
}

// validateAndNormalizeAliases validates and normalizes language aliases.
func validateAndNormalizeAliases(aliases []string) ([]string, error) {
	if len(aliases) > 20 {
		return nil, errors.New("too many aliases (max 20)")
	}

	normalized := make([]string, 0, len(aliases))
	seen := make(map[string]bool)

	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}

		if len(trimmed) > 50 {
			return nil, fmt.Errorf("alias too long (max 50 characters): %s", trimmed)
		}

		// Check for duplicates
		lower := strings.ToLower(trimmed)
		if seen[lower] {
			continue // Skip duplicates
		}
		seen[lower] = true

		if err := validateLanguageName(trimmed); err != nil {
			return nil, fmt.Errorf("invalid alias: %w", err)
		}

		normalized = append(normalized, trimmed)
	}

	return normalized, nil
}

// validateAndNormalizeExtensions validates and normalizes file extensions.
func validateAndNormalizeExtensions(extensions []string) ([]string, error) {
	if len(extensions) > 50 {
		return nil, errors.New("too many extensions (max 50)")
	}

	normalized := make([]string, 0, len(extensions))
	seen := make(map[string]bool)

	for _, ext := range extensions {
		trimmed := strings.TrimSpace(ext)
		if trimmed == "" {
			continue
		}

		// Ensure extension starts with a dot
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}

		// Convert to lowercase for consistency
		trimmed = strings.ToLower(trimmed)

		if len(trimmed) > 20 {
			return nil, fmt.Errorf("extension too long (max 20 characters): %s", trimmed)
		}

		// Check for duplicates
		if seen[trimmed] {
			continue // Skip duplicates
		}
		seen[trimmed] = true

		// Validate extension format
		if err := validateExtension(trimmed); err != nil {
			return nil, fmt.Errorf("invalid extension: %w", err)
		}

		normalized = append(normalized, trimmed)
	}

	return normalized, nil
}

// validateExtension validates a single file extension.
func validateExtension(ext string) error {
	if len(ext) < 2 { // Must be at least "."
		return errors.New("extension too short")
	}

	if !strings.HasPrefix(ext, ".") {
		return errors.New("extension must start with a dot")
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	for i, char := range ext[1:] { // Skip the dot
		if (char < 'a' || char > 'z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' {
			return fmt.Errorf("invalid character at position %d: %c", i+1, char)
		}
	}

	return nil
}

// Name returns the language name.
func (l Language) Name() string {
	return l.name
}

// Aliases returns the language aliases.
func (l Language) Aliases() []string {
	// Return a copy to prevent external modification
	aliases := make([]string, len(l.aliases))
	copy(aliases, l.aliases)
	return aliases
}

// Extensions returns the file extensions for this language.
func (l Language) Extensions() []string {
	// Return a copy to prevent external modification
	extensions := make([]string, len(l.extensions))
	copy(extensions, l.extensions)
	return extensions
}

// Type returns the language type category.
func (l Language) Type() LanguageType {
	return l.languageType
}

// DetectionMethod returns how this language was detected.
func (l Language) DetectionMethod() DetectionMethod {
	return l.detectionMethod
}

// Confidence returns the confidence score of the language detection.
func (l Language) Confidence() float64 {
	return l.confidence
}

// IsUnknown returns true if this is an unknown language.
func (l Language) IsUnknown() bool {
	return l.name == LanguageUnknown
}

// HasExtension returns true if the language supports the given extension.
func (l Language) HasExtension(extension string) bool {
	normalized := strings.ToLower(strings.TrimSpace(extension))
	if !strings.HasPrefix(normalized, ".") {
		normalized = "." + normalized
	}

	for _, ext := range l.extensions {
		if ext == normalized {
			return true
		}
	}
	return false
}

// HasAlias returns true if the language has the given alias.
func (l Language) HasAlias(alias string) bool {
	normalized := strings.ToLower(strings.TrimSpace(alias))

	// Check main name
	if strings.ToLower(l.name) == normalized {
		return true
	}

	// Check aliases
	for _, a := range l.aliases {
		if strings.ToLower(a) == normalized {
			return true
		}
	}
	return false
}

// String returns a string representation of the language.
func (l Language) String() string {
	if l.confidence > 0 {
		return fmt.Sprintf("%s (%.1f%%)", l.name, l.confidence*100)
	}
	return l.name
}

// Equal compares two Language instances for equality.
func (l Language) Equal(other Language) bool {
	return strings.EqualFold(l.name, other.name) &&
		l.languageType == other.languageType &&
		l.detectionMethod == other.detectionMethod
}

// WithConfidence returns a new Language with updated confidence score.
func (l Language) WithConfidence(confidence float64) (Language, error) {
	if confidence < 0.0 || confidence > 1.0 {
		return Language{}, errors.New("confidence score must be between 0.0 and 1.0")
	}

	newLang := l
	newLang.confidence = confidence
	return newLang, nil
}

// WithDetectionMethod returns a new Language with updated detection method.
func (l Language) WithDetectionMethod(method DetectionMethod) Language {
	newLang := l
	newLang.detectionMethod = method
	return newLang
}

// String returns a string representation of the language type.
func (lt LanguageType) String() string {
	switch lt {
	case LanguageTypeCompiled:
		return "Compiled"
	case LanguageTypeInterpreted:
		return "Interpreted"
	case LanguageTypeScripting:
		return "Scripting"
	case LanguageTypeMarkup:
		return "Markup"
	case LanguageTypeData:
		return "Data"
	case LanguageTypeConfig:
		return "Configuration"
	case LanguageTypeDatabase:
		return "Database"
	case LanguageTypeUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}

// String returns a string representation of the detection method.
func (dm DetectionMethod) String() string {
	switch dm {
	case DetectionMethodExtension:
		return "Extension"
	case DetectionMethodContent:
		return "Content"
	case DetectionMethodShebang:
		return "Shebang"
	case DetectionMethodHeuristic:
		return "Heuristic"
	case DetectionMethodFallback:
		return "Fallback"
	case DetectionMethodUnknown:
		return LanguageUnknown
	default:
		return LanguageUnknown
	}
}
