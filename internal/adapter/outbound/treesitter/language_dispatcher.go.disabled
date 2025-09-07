package treesitter

import (
	goparser "codechunking/internal/adapter/outbound/treesitter/parsers/go"
	javascriptparser "codechunking/internal/adapter/outbound/treesitter/parsers/javascript"
	pythonparser "codechunking/internal/adapter/outbound/treesitter/parsers/python"
	"codechunking/internal/domain/valueobject"
	"fmt"
)

// LanguageDispatcher implements LanguageParserFactory and creates language-specific parsers.
type LanguageDispatcher struct {
	supportedLanguages []valueobject.Language
}

// NewLanguageDispatcher creates a new language parser factory.
func NewLanguageDispatcher() (*LanguageDispatcher, error) {
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Go language: %w", err)
	}

	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Python language: %w", err)
	}

	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JavaScript language: %w", err)
	}

	return &LanguageDispatcher{
		supportedLanguages: []valueobject.Language{goLang, pythonLang, jsLang},
	}, nil
}

// CreateParser creates a language-specific parser for the given language.
func (d *LanguageDispatcher) CreateParser(language valueobject.Language) (LanguageParser, error) {
	switch language.Name() {
	case valueobject.LanguageGo:
		parser, err := goparser.NewGoParser()
		if err != nil {
			return nil, fmt.Errorf("failed to create Go parser: %w", err)
		}
		return parser, nil
	case valueobject.LanguagePython:
		parser, err := pythonparser.NewPythonParser()
		if err != nil {
			return nil, fmt.Errorf("failed to create Python parser: %w", err)
		}
		return parser, nil
	case valueobject.LanguageJavaScript:
		parser, err := javascriptparser.NewJavaScriptParser()
		if err != nil {
			return nil, fmt.Errorf("failed to create JavaScript parser: %w", err)
		}
		return parser, nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", language.Name())
	}
}

// GetSupportedLanguages returns a list of all supported languages.
func (d *LanguageDispatcher) GetSupportedLanguages() []valueobject.Language {
	// Return a copy to prevent external modification
	supportedCopy := make([]valueobject.Language, len(d.supportedLanguages))
	copy(supportedCopy, d.supportedLanguages)
	return supportedCopy
}

// IsLanguageSupported checks if the given language is supported by this factory.
func (d *LanguageDispatcher) IsLanguageSupported(language valueobject.Language) bool {
	for _, supported := range d.supportedLanguages {
		if supported.Name() == language.Name() {
			return true
		}
	}
	return false
}
