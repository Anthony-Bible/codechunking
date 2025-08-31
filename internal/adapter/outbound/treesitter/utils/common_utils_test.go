package utils

import (
	"codechunking/internal/domain/valueobject"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateID tests the generateID utility function.
// This is a RED PHASE test that defines expected behavior for generating unique IDs for semantic chunks.
func TestGenerateID(t *testing.T) {
	tests := []struct {
		name            string
		constructType   string
		nameValue       string
		metadata        map[string]interface{}
		expectedPattern string
		shouldBeUnique  bool
	}{
		{
			name:            "generates ID for function",
			constructType:   "function",
			nameValue:       "calculateSum",
			metadata:        map[string]interface{}{"package": "math"},
			expectedPattern: "function_calculatesum_",
			shouldBeUnique:  true,
		},
		{
			name:            "generates ID for struct",
			constructType:   "struct",
			nameValue:       "User",
			metadata:        map[string]interface{}{"package": "models"},
			expectedPattern: "struct_user_",
			shouldBeUnique:  true,
		},
		{
			name:            "generates ID for method",
			constructType:   "method",
			nameValue:       "String",
			metadata:        map[string]interface{}{"receiver": "User", "package": "models"},
			expectedPattern: "method_string_",
			shouldBeUnique:  true,
		},
		{
			name:            "handles empty metadata",
			constructType:   "variable",
			nameValue:       "GlobalVar",
			metadata:        nil,
			expectedPattern: "variable_globalvar_",
			shouldBeUnique:  true,
		},
		{
			name:            "handles special characters in name",
			constructType:   "function",
			nameValue:       "func_with_underscores",
			metadata:        map[string]interface{}{},
			expectedPattern: "function_func_with_underscores_",
			shouldBeUnique:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate multiple IDs to test uniqueness
			ids := make([]string, 3)
			for i := range ids {
				ids[i] = GenerateID(tt.constructType, tt.nameValue, tt.metadata)
			}

			for i, id := range ids {
				// Verify ID format
				assert.Contains(t, id, tt.expectedPattern, "ID %d should contain expected pattern", i)
				assert.Greater(t, len(id), len(tt.expectedPattern), "ID %d should be longer than pattern", i)

				// Verify uniqueness if required
				if tt.shouldBeUnique {
					for j := i + 1; j < len(ids); j++ {
						assert.NotEqual(t, id, ids[j], "ID %d should be unique compared to ID %d", i, j)
					}
				}
			}
		})
	}
}

// TestGenerateHash tests the generateHash utility function.
// This is a RED PHASE test that defines expected behavior for generating content hashes using SHA256.
func TestGenerateHash(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedHash string
		hashLength   int
	}{
		{
			name:       "generates hash for simple content",
			content:    "func main() { fmt.Println(\"hello\") }",
			hashLength: 8, // Should return first 8 characters of SHA256
		},
		{
			name:       "generates hash for empty content",
			content:    "",
			hashLength: 8,
		},
		{
			name:       "generates hash for multiline content",
			content:    "type User struct {\n\tID int\n\tName string\n}",
			hashLength: 8,
		},
		{
			name:       "generates hash for unicode content",
			content:    "// 这是一个测试函数\nfunc test() {}",
			hashLength: 8,
		},
		{
			name:       "generates consistent hash for same content",
			content:    "const PI = 3.14159",
			hashLength: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := GenerateHash(tt.content)
			hash2 := GenerateHash(tt.content)

			// Verify hash properties
			assert.Len(t, hash1, tt.hashLength, "Hash should have expected length")
			assert.Equal(t, hash1, hash2, "Hash should be consistent for same content")
			assert.Regexp(t, "^[a-f0-9]+$", hash1, "Hash should contain only lowercase hex characters")

			// Verify different content produces different hashes
			if tt.content != "" {
				differentHash := GenerateHash(tt.content + " modified")
				assert.NotEqual(t, hash1, differentHash, "Different content should produce different hashes")
			}
		})
	}
}

// TestSafeUint32 tests the safeUint32 utility function.
// This is a RED PHASE test that defines expected behavior for safe conversion to uint32.
func TestSafeUint32(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected uint32
	}{
		{
			name:     "converts positive integer",
			value:    42,
			expected: 42,
		},
		{
			name:     "converts zero",
			value:    0,
			expected: 0,
		},
		{
			name:     "converts negative to zero",
			value:    -100,
			expected: 0,
		},
		{
			name:     "converts max int32",
			value:    2147483647,
			expected: 2147483647,
		},
		{
			name:     "handles overflow by capping to max uint32",
			value:    int(^uint32(0)) + 1000, // Larger than max uint32
			expected: ^uint32(0),             // Max uint32 value
		},
		{
			name:     "converts large positive value within uint32 range",
			value:    4294967295, // Max uint32 value
			expected: 4294967295,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeUint32(tt.value)
			assert.Equal(t, tt.expected, result, "Safe uint32 conversion should match expected")
		})
	}
}

// TestGetVisibility tests the getVisibility utility function.
// This is a RED PHASE test that defines expected behavior for determining visibility of identifiers.
func TestGetVisibility(t *testing.T) {
	tests := []struct {
		name               string
		identifier         string
		language           valueobject.Language
		expectedVisibility string
	}{
		{
			name:               "Go public function (capitalized)",
			identifier:         "PublicFunction",
			language:           createMockLanguage("go"),
			expectedVisibility: "public",
		},
		{
			name:               "Go private function (lowercase)",
			identifier:         "privateFunction",
			language:           createMockLanguage("go"),
			expectedVisibility: "private",
		},
		{
			name:               "Go public struct (capitalized)",
			identifier:         "User",
			language:           createMockLanguage("go"),
			expectedVisibility: "public",
		},
		{
			name:               "Go private field (lowercase)",
			identifier:         "id",
			language:           createMockLanguage("go"),
			expectedVisibility: "private",
		},
		{
			name:               "Python public method (no leading underscore)",
			identifier:         "public_method",
			language:           createMockLanguage("python"),
			expectedVisibility: "public",
		},
		{
			name:               "Python private method (leading underscore)",
			identifier:         "_private_method",
			language:           createMockLanguage("python"),
			expectedVisibility: "private",
		},
		{
			name:               "Python dunder method (public despite underscores)",
			identifier:         "__init__",
			language:           createMockLanguage("python"),
			expectedVisibility: "public",
		},
		{
			name:               "TypeScript public method (no modifiers)",
			identifier:         "publicMethod",
			language:           createMockLanguage("typescript"),
			expectedVisibility: "public",
		},
		{
			name:               "JavaScript function (always public)",
			identifier:         "someFunction",
			language:           createMockLanguage("javascript"),
			expectedVisibility: "public",
		},
		{
			name:               "handles empty identifier",
			identifier:         "",
			language:           createMockLanguage("go"),
			expectedVisibility: "private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetVisibility(tt.identifier, tt.language)
			assert.Equal(
				t,
				tt.expectedVisibility,
				result,
				"Visibility should match expected for %s identifier in %s",
				tt.identifier,
				tt.language.Name(),
			)
		})
	}
}

// TestIsPublicIdentifier tests the isPublicIdentifier utility function.
// This is a RED PHASE test that defines expected behavior for checking if identifier is public.
func TestIsPublicIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		language   valueobject.Language
		isPublic   bool
	}{
		{
			name:       "Go capitalized identifier is public",
			identifier: "PublicFunc",
			language:   createMockLanguage("go"),
			isPublic:   true,
		},
		{
			name:       "Go lowercase identifier is private",
			identifier: "privateFunc",
			language:   createMockLanguage("go"),
			isPublic:   false,
		},
		{
			name:       "Go single capital letter is public",
			identifier: "P",
			language:   createMockLanguage("go"),
			isPublic:   true,
		},
		{
			name:       "Python no underscore is public",
			identifier: "public_method",
			language:   createMockLanguage("python"),
			isPublic:   true,
		},
		{
			name:       "Python leading underscore is private",
			identifier: "_private_method",
			language:   createMockLanguage("python"),
			isPublic:   false,
		},
		{
			name:       "Python dunder method is public",
			identifier: "__init__",
			language:   createMockLanguage("python"),
			isPublic:   true,
		},
		{
			name:       "TypeScript/JavaScript is always public",
			identifier: "anyMethod",
			language:   createMockLanguage("typescript"),
			isPublic:   true,
		},
		{
			name:       "empty identifier is private",
			identifier: "",
			language:   createMockLanguage("go"),
			isPublic:   false,
		},
		{
			name:       "numeric start is private",
			identifier: "123invalid",
			language:   createMockLanguage("go"),
			isPublic:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPublicIdentifier(tt.identifier, tt.language)
			assert.Equal(
				t,
				tt.isPublic,
				result,
				"Public identifier check should match expected for %s in %s",
				tt.identifier,
				tt.language.Name(),
			)
		})
	}
}

// TestCleanIdentifierName tests identifier name cleaning utility.
// This is a RED PHASE test that defines expected behavior for cleaning identifier names.
func TestCleanIdentifierName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "cleans simple identifier",
			input:    "simpleIdentifier",
			expected: "simpleIdentifier",
		},
		{
			name:     "removes leading/trailing whitespace",
			input:    "  spacedIdentifier  ",
			expected: "spacedIdentifier",
		},
		{
			name:     "removes special characters",
			input:    "identifier*with&special@chars",
			expected: "identifierwithspecialchars",
		},
		{
			name:     "preserves underscores",
			input:    "identifier_with_underscores",
			expected: "identifier_with_underscores",
		},
		{
			name:     "handles numbers",
			input:    "identifier123",
			expected: "identifier123",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "handles only special characters",
			input:    "@#$%^&*()",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanIdentifierName(tt.input)
			assert.Equal(t, tt.expected, result, "Cleaned identifier should match expected")
		})
	}
}

// TestValidateIdentifier tests identifier validation utility.
// This is a RED PHASE test that defines expected behavior for validating identifiers.
func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		language   valueobject.Language
		isValid    bool
		errorMsg   string
	}{
		{
			name:       "valid Go identifier",
			identifier: "validGoIdentifier",
			language:   createMockLanguage("go"),
			isValid:    true,
		},
		{
			name:       "valid Go identifier with underscores",
			identifier: "valid_go_identifier",
			language:   createMockLanguage("go"),
			isValid:    true,
		},
		{
			name:       "invalid Go identifier starting with number",
			identifier: "123invalid",
			language:   createMockLanguage("go"),
			isValid:    false,
			errorMsg:   "identifier cannot start with a number",
		},
		{
			name:       "invalid empty identifier",
			identifier: "",
			language:   createMockLanguage("go"),
			isValid:    false,
			errorMsg:   "identifier cannot be empty",
		},
		{
			name:       "invalid Go keyword",
			identifier: "func",
			language:   createMockLanguage("go"),
			isValid:    false,
			errorMsg:   "identifier is a reserved keyword",
		},
		{
			name:       "valid Python identifier",
			identifier: "valid_python_identifier",
			language:   createMockLanguage("python"),
			isValid:    true,
		},
		{
			name:       "invalid Python keyword",
			identifier: "def",
			language:   createMockLanguage("python"),
			isValid:    false,
			errorMsg:   "identifier is a reserved keyword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid, err := ValidateIdentifier(tt.identifier, tt.language)

			assert.Equal(t, tt.isValid, isValid, "Identifier validation should match expected for %s", tt.identifier)

			if tt.isValid {
				assert.NoError(t, err, "Valid identifier should not produce error")
			} else {
				require.Error(t, err, "Invalid identifier should produce error")
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			}
		})
	}
}

// Mock helper functions.
func createMockLanguage(name string) valueobject.Language {
	lang, _ := valueobject.NewLanguage(name)
	return lang
}

// RED PHASE: These functions don't exist yet and will cause compilation errors.
// This is intentional - the tests define the expected behavior for implementation.

// Expected function signatures that need to be implemented:
// func GenerateID(constructType, nameValue string, metadata map[string]interface{}) string
// func GenerateHash(content string) string
// func SafeUint32(value int) uint32
// func GetVisibility(identifier string, language valueobject.Language) string
// func IsPublicIdentifier(identifier string, language valueobject.Language) bool
// func CleanIdentifierName(input string) string
// func ValidateIdentifier(identifier string, language valueobject.Language) (bool, error)
