package valueobject

import (
	"strings"
)

var (
	Go         Language
	Python     Language
	JavaScript Language
	TypeScript Language
)

func init() {
	var err error

	Go, err = NewLanguageWithDetails(
		LanguageGo,
		[]string{},
		[]string{".go"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		1.0,
	)
	if err != nil {
		panic(err)
	}

	Python, err = NewLanguageWithDetails(
		LanguagePython,
		[]string{},
		[]string{".py", ".py3", ".pyc", ".pyo", ".pyw", ".pyx"},
		LanguageTypeInterpreted,
		DetectionMethodExtension,
		1.0,
	)
	if err != nil {
		panic(err)
	}

	JavaScript, err = NewLanguageWithDetails(
		LanguageJavaScript,
		[]string{},
		[]string{".js", ".cjs", ".es", ".es6"},
		LanguageTypeInterpreted,
		DetectionMethodExtension,
		1.0,
	)
	if err != nil {
		panic(err)
	}

	TypeScript, err = NewLanguageWithDetails(
		LanguageTypeScript,
		[]string{},
		[]string{".ts", ".tsx"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		1.0,
	)
	if err != nil {
		panic(err)
	}
}

func GetLanguageByName(name string) *Language {
	switch strings.ToLower(name) {
	case strings.ToLower(LanguageGo):
		return &Go
	case strings.ToLower(LanguagePython):
		return &Python
	case strings.ToLower(LanguageJavaScript):
		return &JavaScript
	case strings.ToLower(LanguageTypeScript):
		return &TypeScript
	default:
		return nil
	}
}
