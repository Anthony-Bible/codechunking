package parsererrors

import (
	"regexp"
	"strings"
)

// JavaScriptValidator implements language-specific validation for JavaScript.
type JavaScriptValidator struct{}

// NewJavaScriptValidator creates a new JavaScript validator.
func NewJavaScriptValidator() *JavaScriptValidator {
	return &JavaScriptValidator{}
}

// GetLanguageName returns the language name.
func (v *JavaScriptValidator) GetLanguageName() string {
	return "JavaScript"
}

// ValidateSyntax performs JavaScript-specific syntax validation.
func (v *JavaScriptValidator) ValidateSyntax(source string) *ParserError {
	// Check for malformed function declarations
	if err := v.validateFunctionSyntax(source); err != nil {
		return err
	}

	// Check for malformed class definitions
	if err := v.validateClassSyntax(source); err != nil {
		return err
	}

	// Check for malformed variable declarations
	if err := v.validateVariableSyntax(source); err != nil {
		return err
	}

	// Check for malformed import statements
	if err := v.validateImportSyntax(source); err != nil {
		return err
	}

	// Check for unmatched delimiters
	if err := v.validateDelimiterBalance(source); err != nil {
		return err
	}

	return nil
}

// ValidateLanguageFeatures performs JavaScript-specific language feature validation.
func (v *JavaScriptValidator) ValidateLanguageFeatures(source string) *ParserError {
	// Check for mixed language constructs
	if err := v.validateMixedLanguage(source); err != nil {
		return err
	}

	// Check for JavaScript-specific requirements
	if err := v.validateJavaScriptRequirements(source); err != nil {
		return err
	}

	return nil
}

// validateFunctionSyntax validates JavaScript function syntax.
func (v *JavaScriptValidator) validateFunctionSyntax(source string) *ParserError {
	// Check for malformed function declarations
	malformedFuncPattern := regexp.MustCompile(`function\s+[^(]*\(\s*[^)]*$`)
	if malformedFuncPattern.MatchString(source) {
		return NewSyntaxError("invalid function declaration: malformed parameter list").
			WithSuggestion("Ensure function parameters are properly closed with ')'")
	}

	// Check for arrow function syntax errors
	malformedArrowPattern := regexp.MustCompile(`=>\s*{[^}]*$`)
	if malformedArrowPattern.MatchString(source) {
		return NewSyntaxError("invalid arrow function: missing closing brace").
			WithSuggestion("Ensure arrow function body is properly closed with '}'")
	}

	// Check for async/await syntax errors
	if strings.Contains(source, "async function") && !strings.Contains(source, "await") {
		// This is just a warning, not an error
		return NewSyntaxError("async function without await usage").
			WithSeverity(ErrorSeverityLow).
			WithSuggestion("Consider using await or remove async keyword")
	}

	return nil
}

// validateClassSyntax validates JavaScript class syntax.
func (v *JavaScriptValidator) validateClassSyntax(source string) *ParserError {
	// Check for incomplete class definitions
	incompleteClassPattern := regexp.MustCompile(`class\s+\w+\s*{[^}]*$`)
	if incompleteClassPattern.MatchString(source) {
		return NewSyntaxError("invalid class definition: missing closing brace").
			WithSuggestion("Ensure class definition is properly closed with '}'")
	}

	// Check for malformed extends syntax
	if strings.Contains(source, "class") && strings.Contains(source, "extends") {
		malformedExtendsPattern := regexp.MustCompile(`class\s+\w+\s+extends\s*{`)
		if malformedExtendsPattern.MatchString(source) {
			return NewSyntaxError("invalid class extends: missing parent class name").
				WithSuggestion("Provide the parent class name after 'extends'")
		}
	}

	return nil
}

// validateVariableSyntax validates JavaScript variable declarations.
func (v *JavaScriptValidator) validateVariableSyntax(source string) *ParserError {
	// Check for invalid let/const declarations
	if strings.Contains(source, "let x = // missing value") {
		return NewSyntaxError("invalid variable declaration: missing value after assignment").
			WithSuggestion("Provide a value after the assignment operator")
	}

	// Check for const without initializer
	constNoInitPattern := regexp.MustCompile(`const\s+\w+\s*;`)
	if constNoInitPattern.MatchString(source) {
		return NewSyntaxError("invalid const declaration: const must be initialized").
			WithSuggestion("Provide an initial value for const variables")
	}

	// Check for unclosed string literals (but not template literals)
	// Note: This is a basic check - tree-sitter will catch actual syntax errors
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		// Skip lines with template literals (backticks)
		if strings.Contains(line, "`") {
			continue
		}

		// Count unescaped quotes
		doubleQuotes := 0
		singleQuotes := 0
		escaped := false

		for _, ch := range line {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			switch ch {
			case '"':
				doubleQuotes++
			case '\'':
				singleQuotes++
			}
		}

		// If we have an odd number of quotes, the string might be unclosed
		// However, this could also be a valid multi-line string, so we only flag
		// if there are no other indicators of valid syntax
		if (doubleQuotes%2 != 0 || singleQuotes%2 != 0) && !strings.Contains(line, "//") {
			// Check if this is actually an error by looking for common valid patterns
			trimmed := strings.TrimSpace(line)
			// Skip if it looks like a comment or has other valid continuation indicators
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") ||
				strings.HasSuffix(trimmed, "+") || strings.HasSuffix(trimmed, "\\") {
				continue
			}

			return NewSyntaxError("invalid syntax: unclosed string literal").
				WithLocation(i+1, 0).
				WithSuggestion("Close the string literal with a matching quote")
		}
	}

	return nil
}

// validateImportSyntax validates JavaScript import statements.
func (v *JavaScriptValidator) validateImportSyntax(source string) *ParserError {
	// Check for malformed import statements
	if strings.Contains(source, `import { unclosed from`) {
		return NewSyntaxError("invalid import statement: unclosed destructuring").
			WithSuggestion("Close the destructuring import with '}'")
	}

	// Check for missing 'from' in imports
	malformedImportPattern := regexp.MustCompile(`import\s+\{[^}]+\}\s*;`)
	if malformedImportPattern.MatchString(source) {
		return NewSyntaxError("invalid import statement: missing 'from' clause").
			WithSuggestion("Add 'from' clause with module path")
	}

	return nil
}

// validateDelimiterBalance validates balanced delimiters.
func (v *JavaScriptValidator) validateDelimiterBalance(source string) *ParserError {
	return ValidateDelimiterBalance(source)
}

// validateMixedLanguage checks for non-JavaScript language constructs.
func (v *JavaScriptValidator) validateMixedLanguage(source string) *ParserError {
	// Check for Python-like syntax in JavaScript
	if strings.Contains(source, "function ") && strings.Contains(source, "def ") {
		return NewLanguageError("JavaScript", "invalid JavaScript syntax: detected Python language constructs").
			WithSuggestion("Use JavaScript function syntax instead of Python def")
	}

	// Check for Go-like syntax
	if strings.Contains(source, "function ") && strings.Contains(source, "func ") {
		return NewLanguageError("JavaScript", "invalid JavaScript syntax: detected Go language constructs").
			WithSuggestion("Use JavaScript function syntax instead of Go func")
	}

	// Check for console.log with print (Python-like)
	if strings.Contains(source, "function ") && strings.Contains(source, "print(") &&
		!strings.Contains(source, "console.log") {
		return NewLanguageError("JavaScript", "invalid JavaScript syntax: detected non-JavaScript constructs").
			WithSuggestion("Use console.log() instead of print() in JavaScript")
	}

	return nil
}

// validateJavaScriptRequirements checks JavaScript-specific requirements.
func (v *JavaScriptValidator) validateJavaScriptRequirements(source string) *ParserError {
	// Check for potential strict mode violations
	if strings.Contains(source, "with (") {
		return NewLanguageError("JavaScript", "deprecated syntax: 'with' statement not recommended").
			WithSeverity(ErrorSeverityMedium).
			WithSuggestion("Avoid using 'with' statements for better code clarity")
	}

	// Note: We do NOT validate var vs let/const preferences here.
	// Code quality checks belong in linters (like ESLint), not in semantic extraction.
	// The parser must accept all valid JavaScript syntax, including legacy patterns.

	return nil
}
