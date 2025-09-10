//go:build disabled
// +build disabled

package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguageDispatcher_NewLanguageDispatcher(t *testing.T) {
	dispatcher, err := NewLanguageDispatcher()
	require.NoError(t, err)
	require.NotNil(t, dispatcher)

	supportedLanguages := dispatcher.GetSupportedLanguages()
	assert.Len(t, supportedLanguages, 3, "Should support 3 languages (Go, Python, and JavaScript)")

	// Check for Go language support
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	assert.True(t, dispatcher.IsLanguageSupported(goLang), "Should support Go language")

	// Check for Python language support
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)
	assert.True(t, dispatcher.IsLanguageSupported(pythonLang), "Should support Python language")

	// Check for JavaScript language support
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)
	assert.True(t, dispatcher.IsLanguageSupported(jsLang), "Should support JavaScript language")
}

func TestLanguageDispatcher_CreateParser(t *testing.T) {
	dispatcher, err := NewLanguageDispatcher()
	require.NoError(t, err)

	t.Run("create_go_parser", func(t *testing.T) {
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parser, err := dispatcher.CreateParser(goLang)
		require.NoError(t, err)
		assert.NotNil(t, parser)
		assert.True(t, parser.IsSupported(goLang))
	})

	t.Run("create_python_parser", func(t *testing.T) {
		pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parser, err := dispatcher.CreateParser(pythonLang)
		require.NoError(t, err)
		assert.NotNil(t, parser)
		assert.True(t, parser.IsSupported(pythonLang))
	})

	t.Run("create_javascript_parser", func(t *testing.T) {
		jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parser, err := dispatcher.CreateParser(jsLang)
		require.NoError(t, err)
		assert.NotNil(t, parser)
		assert.True(t, parser.IsSupported(jsLang))
	})

	t.Run("unsupported_language", func(t *testing.T) {
		javaLang, err := valueobject.NewLanguage(valueobject.LanguageJava)
		require.NoError(t, err)

		parser, err := dispatcher.CreateParser(javaLang)
		require.Error(t, err)
		assert.Nil(t, parser)
		assert.Contains(t, err.Error(), "unsupported language: Java")
	})
}

func TestLanguageDispatcher_GetSupportedLanguages(t *testing.T) {
	dispatcher, err := NewLanguageDispatcher()
	require.NoError(t, err)

	languages := dispatcher.GetSupportedLanguages()
	assert.Len(t, languages, 3)

	// Verify we get copies, not the original slice
	languages[0] = valueobject.Language{}
	newLanguages := dispatcher.GetSupportedLanguages()
	assert.Len(t, newLanguages, 3)
	assert.NotEqual(t, languages[0], newLanguages[0])
}

func TestLanguageDispatcher_IsLanguageSupported(t *testing.T) {
	dispatcher, err := NewLanguageDispatcher()
	require.NoError(t, err)

	testCases := []struct {
		name          string
		language      string
		shouldSupport bool
	}{
		{"Go language", valueobject.LanguageGo, true},
		{"Python language", valueobject.LanguagePython, true},
		{"JavaScript language", valueobject.LanguageJavaScript, true},
		{"Java language", valueobject.LanguageJava, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lang, err := valueobject.NewLanguage(tc.language)
			require.NoError(t, err)

			isSupported := dispatcher.IsLanguageSupported(lang)
			assert.Equal(t, tc.shouldSupport, isSupported)
		})
	}
}
