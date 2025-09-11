package utils

import (
	"codechunking/internal/domain/valueobject"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// GenerateID generates a unique ID for semantic chunks with metadata support.
// This implementation matches the production semantic traverser approach.
func GenerateID(constructType, nameValue string, _ map[string]interface{}) string {
	// Create a base ID with construct type and name (matching semantic traverser)
	baseName := strings.ToLower(nameValue)
	baseID := fmt.Sprintf("%s_%s", constructType, baseName)

	// Add timestamp to ensure uniqueness (maintains existing test compatibility)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d", baseID, timestamp)
}

// GenerateHash generates a SHA256 hash of content, returning first 8 characters.
func GenerateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:8]
}

// SafeUint32 safely converts an int to uint32, capping at maximum value to prevent overflow.
func SafeUint32(value int) uint32 {
	if value < 0 {
		return 0
	}
	if value > int(^uint32(0)) {
		return ^uint32(0) // Maximum uint32 value
	}
	return uint32(value)
}

// GetVisibility determines visibility based on identifier name and language.
func GetVisibility(identifier string, language valueobject.Language) string {
	if IsPublicIdentifier(identifier, language) {
		return "public"
	}
	return "private"
}

// IsPublicIdentifier checks if an identifier is public based on language rules.
func IsPublicIdentifier(identifier string, language valueobject.Language) bool {
	if len(identifier) == 0 {
		return false
	}

	// Check for invalid identifiers (starting with numbers)
	if identifier[0] >= '0' && identifier[0] <= '9' {
		return false
	}

	switch language.Name() {
	case "go":
		// Go: public if starts with uppercase letter
		return identifier[0] >= 'A' && identifier[0] <= 'Z'
	case "python", "Python":
		// Python: dunder methods (double underscore) are public, single underscore are private
		if strings.HasPrefix(identifier, "__") && strings.HasSuffix(identifier, "__") {
			return true // Dunder methods like __init__ are public
		}
		// Private if starts with underscore
		return !strings.HasPrefix(identifier, "_")
	case "typescript", "javascript":
		// TypeScript/JavaScript: assume public (no built-in visibility rules)
		return true
	default:
		// Default: assume public
		return true
	}
}

// CleanIdentifierName cleans an identifier by removing invalid characters.
func CleanIdentifierName(input string) string {
	// Trim whitespace
	cleaned := strings.TrimSpace(input)

	// Remove special characters except underscores, letters, and numbers
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	cleaned = reg.ReplaceAllString(cleaned, "")

	return cleaned
}

// ValidateIdentifier validates an identifier according to language rules.
func ValidateIdentifier(identifier string, language valueobject.Language) (bool, error) {
	if identifier == "" {
		return false, errors.New("identifier cannot be empty")
	}

	// Check if starts with number
	if identifier[0] >= '0' && identifier[0] <= '9' {
		return false, errors.New("identifier cannot start with a number")
	}

	// Check for reserved keywords
	if isReservedKeyword(identifier, language) {
		return false, errors.New("identifier is a reserved keyword")
	}

	return true, nil
}

// isReservedKeyword checks if an identifier is a reserved keyword using efficient map lookup.
func isReservedKeyword(identifier string, language valueobject.Language) bool {
	switch language.Name() {
	case "go":
		// Go keywords - local to avoid global variables
		goKeywords := map[string]bool{
			"break": true, "case": true, "chan": true, "const": true, "continue": true,
			"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
			"func": true, "go": true, "goto": true, "if": true, "import": true,
			"interface": true, "map": true, "package": true, "range": true, "return": true,
			"select": true, "struct": true, "switch": true, "type": true, "var": true,
		}
		return goKeywords[identifier]
	case "python":
		// Python keywords - local to avoid global variables
		pythonKeywords := map[string]bool{
			"False": true, "None": true, "True": true, "and": true, "as": true,
			"assert": true, "break": true, "class": true, "continue": true, "def": true,
			"del": true, "elif": true, "else": true, "except": true, "finally": true,
			"for": true, "from": true, "global": true, "if": true, "import": true,
			"in": true, "is": true, "lambda": true, "nonlocal": true, "not": true,
			"or": true, "pass": true, "raise": true, "return": true, "try": true,
			"while": true, "with": true, "yield": true,
		}
		return pythonKeywords[identifier]
	default:
		return false
	}
}
