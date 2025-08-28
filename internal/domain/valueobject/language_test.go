package valueobject

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLanguage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid language name",
			input:     "Go",
			wantError: false,
		},
		{
			name:      "valid language name with spaces",
			input:     "Visual Basic",
			wantError: false,
		},
		{
			name:      "valid language name with numbers",
			input:     "C++11",
			wantError: false,
		},
		{
			name:      "empty language name",
			input:     "",
			wantError: true,
			errorMsg:  "language name cannot be empty",
		},
		{
			name:      "whitespace only language name",
			input:     "   ",
			wantError: true,
			errorMsg:  "language name cannot be empty after normalization",
		},
		{
			name:      "language name too long",
			input:     strings.Repeat("a", 101),
			wantError: true,
			errorMsg:  "invalid language name: language name too long",
		},
		{
			name:      "language name with control characters",
			input:     "Go\x00Lang",
			wantError: true,
			errorMsg:  "invalid language name: invalid character in language name",
		},
		{
			name:      "language name with newline",
			input:     "Go\nLang",
			wantError: true,
			errorMsg:  "invalid language name: invalid character in language name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang, err := NewLanguage(tt.input)

			if tt.wantError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
				assert.Equal(t, Language{}, lang)
			} else {
				require.NoError(t, err)
				assert.Equal(t, strings.TrimSpace(tt.input), lang.Name())
				assert.Empty(t, lang.Aliases())
				assert.Empty(t, lang.Extensions())
				assert.Equal(t, LanguageTypeUnknown, lang.Type())
				assert.Equal(t, DetectionMethodUnknown, lang.DetectionMethod())
				assert.InDelta(t, 0.0, lang.Confidence(), 0.001)
			}
		})
	}
}

func TestNewLanguageWithDetails(t *testing.T) {
	tests := []struct {
		name            string
		langName        string
		aliases         []string
		extensions      []string
		langType        LanguageType
		detectionMethod DetectionMethod
		confidence      float64
		wantError       bool
		errorMsg        string
	}{
		{
			name:            "valid language with all details",
			langName:        "Go",
			aliases:         []string{"golang", "go-lang"},
			extensions:      []string{".go", ".mod", ".sum"},
			langType:        LanguageTypeCompiled,
			detectionMethod: DetectionMethodExtension,
			confidence:      0.95,
			wantError:       false,
		},
		{
			name:            "confidence too low",
			langName:        "Python",
			aliases:         []string{"py"},
			extensions:      []string{".py"},
			langType:        LanguageTypeInterpreted,
			detectionMethod: DetectionMethodContent,
			confidence:      -0.1,
			wantError:       true,
			errorMsg:        "confidence score must be between 0.0 and 1.0",
		},
		{
			name:            "confidence too high",
			langName:        "Java",
			aliases:         []string{},
			extensions:      []string{".java"},
			langType:        LanguageTypeCompiled,
			detectionMethod: DetectionMethodExtension,
			confidence:      1.5,
			wantError:       true,
			errorMsg:        "confidence score must be between 0.0 and 1.0",
		},
		{
			name:            "too many aliases",
			langName:        "JavaScript",
			aliases:         make([]string, 21), // Too many
			extensions:      []string{".js"},
			langType:        LanguageTypeInterpreted,
			detectionMethod: DetectionMethodExtension,
			confidence:      0.9,
			wantError:       true,
			errorMsg:        "invalid aliases: too many aliases",
		},
		{
			name:            "invalid extension format",
			langName:        "TypeScript",
			aliases:         []string{"ts"},
			extensions:      []string{"ts"}, // Missing dot
			langType:        LanguageTypeCompiled,
			detectionMethod: DetectionMethodExtension,
			confidence:      0.8,
			wantError:       false, // Should auto-add dot
		},
		{
			name:            "duplicate extensions normalized",
			langName:        "C++",
			aliases:         []string{"cpp", "cxx"},
			extensions:      []string{".cpp", ".CPP", ".cxx", ".cpp"}, // Duplicates and case
			langType:        LanguageTypeCompiled,
			detectionMethod: DetectionMethodExtension,
			confidence:      0.9,
			wantError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill aliases if needed for the "too many aliases" test
			if len(tt.aliases) == 21 {
				for i := range 21 {
					tt.aliases[i] = "alias" + strings.Repeat("x", i)
				}
			}

			lang, err := NewLanguageWithDetails(
				tt.langName,
				tt.aliases,
				tt.extensions,
				tt.langType,
				tt.detectionMethod,
				tt.confidence,
			)

			if tt.wantError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
				assert.Equal(t, Language{}, lang)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.langName, lang.Name())
				assert.Equal(t, tt.langType, lang.Type())
				assert.Equal(t, tt.detectionMethod, lang.DetectionMethod())
				assert.InDelta(t, tt.confidence, lang.Confidence(), 0.001)

				// Verify extensions are normalized
				for _, ext := range lang.Extensions() {
					assert.True(t, strings.HasPrefix(ext, "."))
					assert.Equal(t, strings.ToLower(ext), ext)
				}
			}
		})
	}
}

func TestLanguage_HasExtension(t *testing.T) {
	lang, err := NewLanguageWithDetails(
		"Go",
		[]string{"golang"},
		[]string{".go", ".mod", ".sum"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		0.95,
	)
	require.NoError(t, err)

	tests := []struct {
		name      string
		extension string
		want      bool
	}{
		{
			name:      "exact match",
			extension: ".go",
			want:      true,
		},
		{
			name:      "case insensitive match",
			extension: ".GO",
			want:      true,
		},
		{
			name:      "without dot prefix",
			extension: "go",
			want:      true,
		},
		{
			name:      "with whitespace",
			extension: " .go ",
			want:      true,
		},
		{
			name:      "no match",
			extension: ".py",
			want:      false,
		},
		{
			name:      "empty extension",
			extension: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lang.HasExtension(tt.extension)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestLanguage_HasAlias(t *testing.T) {
	lang, err := NewLanguageWithDetails(
		"Go",
		[]string{"golang", "go-lang"},
		[]string{".go"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		0.95,
	)
	require.NoError(t, err)

	tests := []struct {
		name  string
		alias string
		want  bool
	}{
		{
			name:  "exact name match",
			alias: "Go",
			want:  true,
		},
		{
			name:  "case insensitive name match",
			alias: "go",
			want:  true,
		},
		{
			name:  "alias match",
			alias: "golang",
			want:  true,
		},
		{
			name:  "case insensitive alias match",
			alias: "GOLANG",
			want:  true,
		},
		{
			name:  "alias with whitespace",
			alias: " go-lang ",
			want:  true,
		},
		{
			name:  "no match",
			alias: "python",
			want:  false,
		},
		{
			name:  "empty alias",
			alias: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lang.HasAlias(tt.alias)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestLanguage_IsUnknown(t *testing.T) {
	tests := []struct {
		name     string
		langName string
		want     bool
	}{
		{
			name:     "unknown language",
			langName: LanguageUnknown,
			want:     true,
		},
		{
			name:     "known language",
			langName: "Go",
			want:     false,
		},
		{
			name:     "case sensitive unknown check",
			langName: "unknown",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang, err := NewLanguage(tt.langName)
			require.NoError(t, err)

			result := lang.IsUnknown()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestLanguage_String(t *testing.T) {
	tests := []struct {
		name       string
		langName   string
		confidence float64
		want       string
	}{
		{
			name:       "language with confidence",
			langName:   "Go",
			confidence: 0.95,
			want:       "Go (95.0%)",
		},
		{
			name:       "language without confidence",
			langName:   "Python",
			confidence: 0.0,
			want:       "Python",
		},
		{
			name:       "language with low confidence",
			langName:   "JavaScript",
			confidence: 0.123,
			want:       "JavaScript (12.3%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang, err := NewLanguageWithDetails(
				tt.langName,
				[]string{},
				[]string{},
				LanguageTypeUnknown,
				DetectionMethodUnknown,
				tt.confidence,
			)
			require.NoError(t, err)

			result := lang.String()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestLanguage_Equal(t *testing.T) {
	lang1, err := NewLanguageWithDetails(
		"Go",
		[]string{"golang"},
		[]string{".go"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		0.95,
	)
	require.NoError(t, err)

	lang2, err := NewLanguageWithDetails(
		"go",                // Different case
		[]string{"go-lang"}, // Different aliases
		[]string{".mod"},    // Different extensions
		LanguageTypeCompiled,
		DetectionMethodExtension,
		0.80, // Different confidence
	)
	require.NoError(t, err)

	lang3, err := NewLanguageWithDetails(
		"Python",
		[]string{},
		[]string{".py"},
		LanguageTypeInterpreted,
		DetectionMethodContent,
		0.90,
	)
	require.NoError(t, err)

	tests := []struct {
		name  string
		lang1 Language
		lang2 Language
		want  bool
	}{
		{
			name:  "equal languages case insensitive name",
			lang1: lang1,
			lang2: lang2,
			want:  true, // Same name (case insensitive), type, and method
		},
		{
			name:  "different languages",
			lang1: lang1,
			lang2: lang3,
			want:  false,
		},
		{
			name:  "same language with itself",
			lang1: lang1,
			lang2: lang1,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.lang1.Equal(tt.lang2)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestLanguage_WithConfidence(t *testing.T) {
	originalLang, err := NewLanguage("Go")
	require.NoError(t, err)

	tests := []struct {
		name       string
		confidence float64
		wantError  bool
		errorMsg   string
	}{
		{
			name:       "valid confidence",
			confidence: 0.85,
			wantError:  false,
		},
		{
			name:       "minimum confidence",
			confidence: 0.0,
			wantError:  false,
		},
		{
			name:       "maximum confidence",
			confidence: 1.0,
			wantError:  false,
		},
		{
			name:       "confidence too low",
			confidence: -0.1,
			wantError:  true,
			errorMsg:   "confidence score must be between 0.0 and 1.0",
		},
		{
			name:       "confidence too high",
			confidence: 1.1,
			wantError:  true,
			errorMsg:   "confidence score must be between 0.0 and 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLang, err := originalLang.WithConfidence(tt.confidence)

			if tt.wantError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
				assert.Equal(t, Language{}, newLang)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.confidence, newLang.Confidence(), 0.001)
				assert.Equal(t, originalLang.Name(), newLang.Name()) // Other fields unchanged
				// Only check confidence changed when it should actually change
				if tt.confidence != originalLang.Confidence() {
					assert.NotEqual(t, originalLang.Confidence(), newLang.Confidence()) // Confidence changed
				} else {
					assert.InDelta(t, originalLang.Confidence(), newLang.Confidence(), 0.001) // Confidence unchanged
				}
			}
		})
	}
}

func TestLanguage_WithDetectionMethod(t *testing.T) {
	originalLang, err := NewLanguage("Go")
	require.NoError(t, err)

	tests := []struct {
		name   string
		method DetectionMethod
	}{
		{
			name:   "extension method",
			method: DetectionMethodExtension,
		},
		{
			name:   "content method",
			method: DetectionMethodContent,
		},
		{
			name:   "shebang method",
			method: DetectionMethodShebang,
		},
		{
			name:   "heuristic method",
			method: DetectionMethodHeuristic,
		},
		{
			name:   "fallback method",
			method: DetectionMethodFallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLang := originalLang.WithDetectionMethod(tt.method)

			assert.Equal(t, tt.method, newLang.DetectionMethod())
			assert.Equal(t, originalLang.Name(), newLang.Name())                          // Other fields unchanged
			assert.NotEqual(t, originalLang.DetectionMethod(), newLang.DetectionMethod()) // Method changed
		})
	}
}

func TestLanguageType_String(t *testing.T) {
	tests := []struct {
		name     string
		langType LanguageType
		want     string
	}{
		{
			name:     "compiled language",
			langType: LanguageTypeCompiled,
			want:     "Compiled",
		},
		{
			name:     "interpreted language",
			langType: LanguageTypeInterpreted,
			want:     "Interpreted",
		},
		{
			name:     "scripting language",
			langType: LanguageTypeScripting,
			want:     "Scripting",
		},
		{
			name:     "markup language",
			langType: LanguageTypeMarkup,
			want:     "Markup",
		},
		{
			name:     "data language",
			langType: LanguageTypeData,
			want:     "Data",
		},
		{
			name:     "config language",
			langType: LanguageTypeConfig,
			want:     "Configuration",
		},
		{
			name:     "database language",
			langType: LanguageTypeDatabase,
			want:     "Database",
		},
		{
			name:     "unknown language",
			langType: LanguageTypeUnknown,
			want:     LanguageUnknown,
		},
		{
			name:     "invalid language type",
			langType: LanguageType(999),
			want:     LanguageUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.langType.String()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestDetectionMethod_String(t *testing.T) {
	tests := []struct {
		name   string
		method DetectionMethod
		want   string
	}{
		{
			name:   "extension method",
			method: DetectionMethodExtension,
			want:   "Extension",
		},
		{
			name:   "content method",
			method: DetectionMethodContent,
			want:   "Content",
		},
		{
			name:   "shebang method",
			method: DetectionMethodShebang,
			want:   "Shebang",
		},
		{
			name:   "heuristic method",
			method: DetectionMethodHeuristic,
			want:   "Heuristic",
		},
		{
			name:   "fallback method",
			method: DetectionMethodFallback,
			want:   "Fallback",
		},
		{
			name:   "unknown method",
			method: DetectionMethodUnknown,
			want:   LanguageUnknown,
		},
		{
			name:   "invalid method",
			method: DetectionMethod(999),
			want:   LanguageUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method.String()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestValidateExtension(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid extension",
			extension: ".go",
			wantError: false,
		},
		{
			name:      "valid extension with numbers",
			extension: ".c99",
			wantError: false,
		},
		{
			name:      "valid extension with dash",
			extension: ".proto-buf",
			wantError: false,
		},
		{
			name:      "valid extension with underscore",
			extension: ".proto_buf",
			wantError: false,
		},
		{
			name:      "extension too short",
			extension: ".",
			wantError: true,
			errorMsg:  "extension too short",
		},
		{
			name:      "extension without dot",
			extension: "go",
			wantError: true,
			errorMsg:  "extension must start with a dot",
		},
		{
			name:      "extension with space",
			extension: ".go lang",
			wantError: true,
			errorMsg:  "invalid character at position",
		},
		{
			name:      "extension with uppercase",
			extension: ".GO",
			wantError: true,
			errorMsg:  "invalid character at position",
		},
		{
			name:      "extension with special character",
			extension: ".go@",
			wantError: true,
			errorMsg:  "invalid character at position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExtension(tt.extension)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAndNormalizeAliases(t *testing.T) {
	tests := []struct {
		name      string
		aliases   []string
		want      []string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid aliases",
			aliases:   []string{"golang", "go-lang", "Go Language"},
			want:      []string{"golang", "go-lang", "Go Language"},
			wantError: false,
		},
		{
			name:      "aliases with duplicates",
			aliases:   []string{"golang", "GOLANG", "golang"},
			want:      []string{"golang"},
			wantError: false,
		},
		{
			name:      "aliases with empty strings",
			aliases:   []string{"golang", "", "   ", "go-lang"},
			want:      []string{"golang", "go-lang"},
			wantError: false,
		},
		{
			name:      "too many aliases",
			aliases:   make([]string, 21),
			want:      nil,
			wantError: true,
			errorMsg:  "too many aliases",
		},
		{
			name:      "alias too long",
			aliases:   []string{strings.Repeat("a", 51)},
			want:      nil,
			wantError: true,
			errorMsg:  "alias too long",
		},
		{
			name:      "alias with control character",
			aliases:   []string{"go\x00lang"},
			want:      nil,
			wantError: true,
			errorMsg:  "invalid alias",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill aliases for "too many aliases" test
			if len(tt.aliases) == 21 {
				for i := range 21 {
					tt.aliases[i] = "alias" + strings.Repeat("x", i%10)
				}
			}

			result, err := validateAndNormalizeAliases(tt.aliases)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestValidateAndNormalizeExtensions(t *testing.T) {
	tests := []struct {
		name       string
		extensions []string
		want       []string
		wantError  bool
		errorMsg   string
	}{
		{
			name:       "valid extensions",
			extensions: []string{".go", ".mod", ".sum"},
			want:       []string{".go", ".mod", ".sum"},
			wantError:  false,
		},
		{
			name:       "extensions without dots",
			extensions: []string{"go", "mod", "sum"},
			want:       []string{".go", ".mod", ".sum"},
			wantError:  false,
		},
		{
			name:       "mixed case extensions",
			extensions: []string{".GO", ".Mod", ".sum"},
			want:       []string{".go", ".mod", ".sum"},
			wantError:  false,
		},
		{
			name:       "extensions with duplicates",
			extensions: []string{".go", ".GO", "go", ".go"},
			want:       []string{".go"},
			wantError:  false,
		},
		{
			name:       "extensions with empty strings",
			extensions: []string{".go", "", "   ", ".mod"},
			want:       []string{".go", ".mod"},
			wantError:  false,
		},
		{
			name:       "too many extensions",
			extensions: make([]string, 51),
			want:       nil,
			wantError:  true,
			errorMsg:   "too many extensions",
		},
		{
			name:       "extension too long",
			extensions: []string{strings.Repeat("a", 20)},
			want:       nil,
			wantError:  true,
			errorMsg:   "extension too long",
		},
		{
			name:       "invalid extension character",
			extensions: []string{".go@lang"},
			want:       nil,
			wantError:  true,
			errorMsg:   "invalid extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill extensions for "too many extensions" test
			if len(tt.extensions) == 51 {
				for i := range 51 {
					tt.extensions[i] = ".ext" + strings.Repeat("x", i%10)
				}
			}

			result, err := validateAndNormalizeExtensions(tt.extensions)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

// Benchmark tests for performance validation.
func BenchmarkNewLanguage(b *testing.B) {
	for range b.N {
		_, err := NewLanguage("Go")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewLanguageWithDetails(b *testing.B) {
	aliases := []string{"golang", "go-lang"}
	extensions := []string{".go", ".mod", ".sum"}

	for range b.N {
		_, err := NewLanguageWithDetails(
			"Go",
			aliases,
			extensions,
			LanguageTypeCompiled,
			DetectionMethodExtension,
			0.95,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLanguage_HasExtension(b *testing.B) {
	lang, err := NewLanguageWithDetails(
		"Go",
		[]string{"golang"},
		[]string{".go", ".mod", ".sum"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		0.95,
	)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_ = lang.HasExtension(".go")
	}
}

func BenchmarkLanguage_HasAlias(b *testing.B) {
	lang, err := NewLanguageWithDetails(
		"Go",
		[]string{"golang", "go-lang"},
		[]string{".go"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		0.95,
	)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_ = lang.HasAlias("golang")
	}
}
