package chunking

import (
	"codechunking/internal/domain/valueobject"
)

// mustCreateLanguage creates a Language value object for testing purposes.
func mustCreateLanguage(lang string) valueobject.Language {
	language, err := valueobject.NewLanguage(lang)
	if err != nil {
		panic(err)
	}
	return language
}

// containsString checks if substring exists in string.
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// findStringIndex finds the index of a substring in a string, returns -1 if not found.
func findStringIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// generateLargeContent generates content of specified size for testing.
func generateLargeContent(size int) string {
	content := make([]byte, size)
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}
	return string(content)
}
