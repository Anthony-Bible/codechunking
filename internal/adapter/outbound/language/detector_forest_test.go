package language

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
)

// TestForestLangToValueObjectLang tests the conversion from forest language names
// to our valueobject.Language constants.
func TestForestLangToValueObjectLang(t *testing.T) {
	tests := []struct {
		name          string
		forestLang    string
		expectedLang  string
		expectedError bool
		description   string
	}{
		{
			name:          "go language",
			forestLang:    "go",
			expectedLang:  valueobject.LanguageGo,
			expectedError: false,
			description:   "forest returns 'go', should map to 'Go'",
		},
		{
			name:          "python language",
			forestLang:    "python",
			expectedLang:  valueobject.LanguagePython,
			expectedError: false,
			description:   "forest returns 'python', should map to 'Python'",
		},
		{
			name:          "javascript language",
			forestLang:    "javascript",
			expectedLang:  valueobject.LanguageJavaScript,
			expectedError: false,
			description:   "forest returns 'javascript', should map to 'JavaScript'",
		},
		{
			name:          "typescript language",
			forestLang:    "typescript",
			expectedLang:  valueobject.LanguageTypeScript,
			expectedError: false,
			description:   "forest returns 'typescript', should map to 'TypeScript'",
		},
		{
			name:          "c++ with underscore",
			forestLang:    "c_plus_plus",
			expectedLang:  valueobject.LanguageCPlusPlus,
			expectedError: false,
			description:   "forest may return 'c_plus_plus', should map to 'C++'",
		},
		{
			name:          "c++ with short form",
			forestLang:    "cpp",
			expectedLang:  valueobject.LanguageCPlusPlus,
			expectedError: false,
			description:   "forest may return 'cpp', should map to 'C++'",
		},
		{
			name:          "c language",
			forestLang:    "c",
			expectedLang:  valueobject.LanguageC,
			expectedError: false,
			description:   "forest returns 'c', should map to 'C'",
		},
		{
			name:          "java language",
			forestLang:    "java",
			expectedLang:  valueobject.LanguageJava,
			expectedError: false,
			description:   "forest returns 'java', should map to 'Java'",
		},
		{
			name:          "rust language",
			forestLang:    "rust",
			expectedLang:  valueobject.LanguageRust,
			expectedError: false,
			description:   "forest returns 'rust', should map to 'Rust'",
		},
		{
			name:          "ruby language",
			forestLang:    "ruby",
			expectedLang:  valueobject.LanguageRuby,
			expectedError: false,
			description:   "forest returns 'ruby', should map to 'Ruby'",
		},
		{
			name:          "php language",
			forestLang:    "php",
			expectedLang:  valueobject.LanguagePHP,
			expectedError: false,
			description:   "forest returns 'php', should map to 'PHP'",
		},
		{
			name:          "html language",
			forestLang:    "html",
			expectedLang:  valueobject.LanguageHTML,
			expectedError: false,
			description:   "forest returns 'html', should map to 'HTML'",
		},
		{
			name:          "css language",
			forestLang:    "css",
			expectedLang:  valueobject.LanguageCSS,
			expectedError: false,
			description:   "forest returns 'css', should map to 'CSS'",
		},
		{
			name:          "json language",
			forestLang:    "json",
			expectedLang:  valueobject.LanguageJSON,
			expectedError: false,
			description:   "forest returns 'json', should map to 'JSON'",
		},
		{
			name:          "yaml language",
			forestLang:    "yaml",
			expectedLang:  valueobject.LanguageYAML,
			expectedError: false,
			description:   "forest returns 'yaml', should map to 'YAML'",
		},
		{
			name:          "xml language",
			forestLang:    "xml",
			expectedLang:  valueobject.LanguageXML,
			expectedError: false,
			description:   "forest returns 'xml', should map to 'XML'",
		},
		{
			name:          "markdown language",
			forestLang:    "markdown",
			expectedLang:  valueobject.LanguageMarkdown,
			expectedError: false,
			description:   "forest returns 'markdown', should map to 'Markdown'",
		},
		{
			name:          "sql language",
			forestLang:    "sql",
			expectedLang:  valueobject.LanguageSQL,
			expectedError: false,
			description:   "forest returns 'sql', should map to 'SQL'",
		},
		{
			name:          "bash/shell language",
			forestLang:    "bash",
			expectedLang:  valueobject.LanguageShell,
			expectedError: false,
			description:   "forest returns 'bash', should map to 'Shell'",
		},
		{
			name:          "shell language",
			forestLang:    "shell",
			expectedLang:  valueobject.LanguageShell,
			expectedError: false,
			description:   "forest may return 'shell', should map to 'Shell'",
		},
		{
			name:          "c_sharp language",
			forestLang:    "c_sharp",
			expectedLang:  valueobject.LanguageUnknown,
			expectedError: false,
			description:   "forest returns 'c_sharp' but we don't support it yet, should map to 'Unknown'",
		},
		{
			name:          "unknown language",
			forestLang:    "nonexistentlang",
			expectedLang:  valueobject.LanguageUnknown,
			expectedError: false,
			description:   "unsupported language should map to 'Unknown'",
		},
		{
			name:          "empty string",
			forestLang:    "",
			expectedLang:  valueobject.LanguageUnknown,
			expectedError: false,
			description:   "empty language string should map to 'Unknown'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because forestLangToValueObjectLang doesn't exist yet
			langName := forestLangToValueObjectLang(tt.forestLang)

			if langName != tt.expectedLang {
				t.Errorf("forestLangToValueObjectLang(%q) = %q, want %q - %s",
					tt.forestLang, langName, tt.expectedLang, tt.description)
			}
		})
	}
}

// TestDetectorDetectFromForest tests the new detectFromForest method that uses
// forest.DetectLanguage for language detection.
func TestDetectorDetectFromForest(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	tests := []struct {
		name               string
		filePath           string
		expectedLanguage   string
		expectedConfidence float64
		expectedMethod     valueobject.DetectionMethod
		expectError        bool
		errorType          string
		description        string
	}{
		{
			name:               "go file detection",
			filePath:           "main.go",
			expectedLanguage:   valueobject.LanguageGo,
			expectedConfidence: 0.75,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectError:        false,
			description:        "forest.DetectLanguage should detect Go from .go extension",
		},
		{
			name:               "python file detection",
			filePath:           "script.py",
			expectedLanguage:   valueobject.LanguagePython,
			expectedConfidence: 0.75,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectError:        false,
			description:        "forest.DetectLanguage should detect Python from .py extension",
		},
		{
			name:               "javascript file detection",
			filePath:           "app.js",
			expectedLanguage:   valueobject.LanguageJavaScript,
			expectedConfidence: 0.75,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectError:        false,
			description:        "forest.DetectLanguage should detect JavaScript from .js extension",
		},
		{
			name:               "typescript file detection",
			filePath:           "component.ts",
			expectedLanguage:   valueobject.LanguageTypeScript,
			expectedConfidence: 0.75,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectError:        false,
			description:        "forest.DetectLanguage should detect TypeScript from .ts extension",
		},
		{
			name:               "cpp file detection",
			filePath:           "program.cpp",
			expectedLanguage:   valueobject.LanguageCPlusPlus,
			expectedConfidence: 0.75,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectError:        false,
			description:        "forest.DetectLanguage should detect C++ from .cpp extension",
		},
		{
			name:               "rust file detection",
			filePath:           "lib.rs",
			expectedLanguage:   valueobject.LanguageRust,
			expectedConfidence: 0.75,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectError:        false,
			description:        "forest.DetectLanguage should detect Rust from .rs extension",
		},
		{
			name:        "empty file path",
			filePath:    "",
			expectError: true,
			errorType:   outbound.ErrorTypeInvalidFile,
			description: "empty file path should return an error",
		},
		{
			name:               "unsupported language detected by forest",
			filePath:           "file.xyz",
			expectedLanguage:   valueobject.LanguageUnknown,
			expectedConfidence: 0.75,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectError:        false,
			description:        "when forest returns unsupported language, should map to Unknown",
		},
		{
			name:        "file with no extension",
			filePath:    "Makefile",
			expectError: true,
			errorType:   outbound.ErrorTypeUnsupportedFormat,
			description: "files with no extension that forest can't detect should error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because detectFromForest method doesn't exist yet
			lang, err := detector.detectFromForest(ctx, tt.filePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("detectFromForest(%q) expected error, got nil - %s",
						tt.filePath, tt.description)
					return
				}

				detectionErr := &outbound.DetectionError{}
				if errors.As(err, &detectionErr) {
					if detectionErr.Type != tt.errorType {
						t.Errorf("detectFromForest(%q) error type = %v, want %v",
							tt.filePath, detectionErr.Type, tt.errorType)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("detectFromForest(%q) unexpected error: %v - %s",
					tt.filePath, err, tt.description)
				return
			}

			// Verify language name
			if lang.Name() != tt.expectedLanguage {
				t.Errorf("detectFromForest(%q) language = %q, want %q - %s",
					tt.filePath, lang.Name(), tt.expectedLanguage, tt.description)
			}

			// Verify confidence score
			if lang.Confidence() != tt.expectedConfidence {
				t.Errorf("detectFromForest(%q) confidence = %.2f, want %.2f - %s",
					tt.filePath, lang.Confidence(), tt.expectedConfidence, tt.description)
			}

			// Verify detection method
			if lang.DetectionMethod() != tt.expectedMethod {
				t.Errorf("detectFromForest(%q) detection method = %v, want %v - %s",
					tt.filePath, lang.DetectionMethod(), tt.expectedMethod, tt.description)
			}
		})
	}
}

// TestDetectFromFilePathWithForestFallback tests that forest detection is used
// as a fallback when extension-based detection fails.
func TestDetectFromFilePathWithForestFallback(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	tests := []struct {
		name               string
		filePath           string
		expectedLanguage   string
		expectedMethod     valueobject.DetectionMethod
		expectedConfidence float64
		expectError        bool
		description        string
	}{
		{
			name:               "known extension uses extension detection",
			filePath:           "main.go",
			expectedLanguage:   valueobject.LanguageGo,
			expectedMethod:     valueobject.DetectionMethodExtension,
			expectedConfidence: 0.95,
			expectError:        false,
			description:        "known extensions should use extension detection, not forest",
		},
		{
			name:               "unknown extension falls back to forest",
			filePath:           "script.xyz",
			expectedLanguage:   valueobject.LanguageUnknown,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectedConfidence: 0.75,
			expectError:        false,
			description:        "unknown extensions should fall back to forest detection",
		},
		{
			name:               "jsx file uses forest fallback",
			filePath:           "component.jsx",
			expectedLanguage:   valueobject.LanguageJavaScript,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectedConfidence: 0.75,
			expectError:        false,
			description:        "jsx not in extension map should use forest, detect as JavaScript",
		},
		{
			name:               "tsx file uses forest fallback",
			filePath:           "component.tsx",
			expectedLanguage:   valueobject.LanguageTypeScript,
			expectedMethod:     valueobject.DetectionMethodHeuristic,
			expectedConfidence: 0.75,
			expectError:        false,
			description:        "tsx not in extension map should use forest, detect as TypeScript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because:
			// 1. detectFromForest doesn't exist
			// 2. DetectFromFilePath doesn't call detectFromForest as fallback
			lang, err := detector.DetectFromFilePath(ctx, tt.filePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("DetectFromFilePath(%q) expected error, got nil - %s",
						tt.filePath, tt.description)
				}
				return
			}

			if err != nil {
				t.Errorf("DetectFromFilePath(%q) unexpected error: %v - %s",
					tt.filePath, err, tt.description)
				return
			}

			// Verify language name
			if lang.Name() != tt.expectedLanguage {
				t.Errorf("DetectFromFilePath(%q) language = %q, want %q - %s",
					tt.filePath, lang.Name(), tt.expectedLanguage, tt.description)
			}

			// Verify detection method
			if lang.DetectionMethod() != tt.expectedMethod {
				t.Errorf("DetectFromFilePath(%q) detection method = %v, want %v - %s",
					tt.filePath, lang.DetectionMethod(), tt.expectedMethod, tt.description)
			}

			// Verify confidence score
			if lang.Confidence() != tt.expectedConfidence {
				t.Errorf("DetectFromFilePath(%q) confidence = %.2f, want %.2f - %s",
					tt.filePath, lang.Confidence(), tt.expectedConfidence, tt.description)
			}
		})
	}
}

// TestForestDetectionCaching tests that forest detection results are properly cached.
func TestForestDetectionCaching(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	// Test file that requires forest detection (not in extension map)
	filePath := "component.jsx"

	// This test will fail because:
	// 1. detectFromForest doesn't exist
	// 2. Forest detection results aren't being cached

	// First detection - should use forest
	lang1, err := detector.DetectFromFilePath(ctx, filePath)
	if err != nil {
		t.Fatalf("First DetectFromFilePath(%q) failed: %v", filePath, err)
	}

	// Second detection - should retrieve from cache
	lang2, err := detector.DetectFromFilePath(ctx, filePath)
	if err != nil {
		t.Fatalf("Second DetectFromFilePath(%q) failed: %v", filePath, err)
	}

	// Both results should be identical
	if lang1.Name() != lang2.Name() {
		t.Errorf("Cached result mismatch: first=%q, second=%q",
			lang1.Name(), lang2.Name())
	}

	if lang1.Confidence() != lang2.Confidence() {
		t.Errorf("Cached confidence mismatch: first=%.2f, second=%.2f",
			lang1.Confidence(), lang2.Confidence())
	}

	if lang1.DetectionMethod() != lang2.DetectionMethod() {
		t.Errorf("Cached detection method mismatch: first=%v, second=%v",
			lang1.DetectionMethod(), lang2.DetectionMethod())
	}

	// Verify the language is JavaScript (forest should detect jsx as javascript)
	if lang1.Name() != valueobject.LanguageJavaScript {
		t.Errorf("Expected JavaScript for .jsx file, got %q", lang1.Name())
	}

	// Verify detection method is heuristic (from forest)
	if lang1.DetectionMethod() != valueobject.DetectionMethodHeuristic {
		t.Errorf("Expected heuristic detection method, got %v", lang1.DetectionMethod())
	}
}

// TestForestDetectionWithEmptyResult tests handling when forest.DetectLanguage
// returns an empty string.
func TestForestDetectionWithEmptyResult(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	// This test will fail because detectFromForest doesn't exist yet
	// We need to test what happens when forest returns ""

	// Test with a file that might not be detected by forest
	filePath := "file.unknown_ext"

	lang, err := detector.detectFromForest(ctx, filePath)
	// When forest returns empty string, we should get Unknown language
	// with low confidence or an error
	if err != nil {
		// It's acceptable to return an error for undetectable files
		detectionErr := &outbound.DetectionError{}
		if errors.As(err, &detectionErr) {
			if detectionErr.Type != outbound.ErrorTypeUnsupportedFormat {
				t.Errorf("Expected ErrorTypeUnsupportedFormat, got %v", detectionErr.Type)
			}
		}
		return
	}

	// If no error, should return Unknown language
	if lang.Name() != valueobject.LanguageUnknown {
		t.Errorf("Expected Unknown language when forest returns empty, got %q", lang.Name())
	}
}

// TestForestLangToValueObjectLangCaseInsensitivity tests that the mapping
// function handles different case variations from forest.
func TestForestLangToValueObjectLangCaseInsensitivity(t *testing.T) {
	tests := []struct {
		name         string
		forestLang   string
		expectedLang string
		description  string
	}{
		{
			name:         "lowercase go",
			forestLang:   "go",
			expectedLang: valueobject.LanguageGo,
			description:  "lowercase 'go' should map to 'Go'",
		},
		{
			name:         "uppercase GO",
			forestLang:   "GO",
			expectedLang: valueobject.LanguageGo,
			description:  "uppercase 'GO' should map to 'Go'",
		},
		{
			name:         "mixed case Python",
			forestLang:   "Python",
			expectedLang: valueobject.LanguagePython,
			description:  "mixed case 'Python' should map to 'Python'",
		},
		{
			name:         "lowercase python",
			forestLang:   "python",
			expectedLang: valueobject.LanguagePython,
			description:  "lowercase 'python' should map to 'Python'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because forestLangToValueObjectLang doesn't exist yet
			langName := forestLangToValueObjectLang(tt.forestLang)

			if langName != tt.expectedLang {
				t.Errorf("forestLangToValueObjectLang(%q) = %q, want %q - %s",
					tt.forestLang, langName, tt.expectedLang, tt.description)
			}
		})
	}
}

// TestDetectFromForestConfidenceScore tests that forest detection
// uses the correct confidence score for heuristic detection.
func TestDetectFromForestConfidenceScore(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	// This test will fail because detectFromForest doesn't exist yet

	lang, err := detector.detectFromForest(ctx, "main.go")
	if err != nil {
		t.Fatalf("detectFromForest failed: %v", err)
	}

	// Forest detection should use heuristic confidence (0.75 based on getConfidenceForMethod)
	expectedConfidence := detector.getConfidenceForMethod(DetectionMethodHeuristic)

	if lang.Confidence() != expectedConfidence {
		t.Errorf("detectFromForest confidence = %.2f, want %.2f (heuristic confidence)",
			lang.Confidence(), expectedConfidence)
	}
}
