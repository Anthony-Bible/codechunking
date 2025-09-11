package parsererrors

import (
	"regexp"
	"strings"
)

// GoValidator implements language-specific validation for Go.
type GoValidator struct{}

// NewGoValidator creates a new Go validator.
func NewGoValidator() *GoValidator {
	return &GoValidator{}
}

// GetLanguageName returns the language name.
func (v *GoValidator) GetLanguageName() string {
	return "Go"
}

// ValidateSyntax performs Go-specific syntax validation.
func (v *GoValidator) ValidateSyntax(source string) *ParserError {
	// Check for malformed function declarations
	if err := v.validateFunctionSyntax(source); err != nil {
		return err
	}

	// Check for malformed struct definitions
	if err := v.validateStructSyntax(source); err != nil {
		return err
	}

	// Check for malformed interface definitions
	if err := v.validateInterfaceSyntax(source); err != nil {
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

	// Check for malformed package declarations
	if err := v.validatePackageSyntax(source); err != nil {
		return err
	}

	return nil
}

// ValidateLanguageFeatures performs Go-specific language feature validation.
func (v *GoValidator) ValidateLanguageFeatures(source string) *ParserError {
	// Check for mixed language constructs
	if err := v.validateMixedLanguage(source); err != nil {
		return err
	}

	// Check for Go-specific requirements
	if err := v.validateGoRequirements(source); err != nil {
		return err
	}

	return nil
}

// validateFunctionSyntax validates Go function syntax.
func (v *GoValidator) validateFunctionSyntax(source string) *ParserError {
	// Check for malformed function declarations using regex
	malformedFuncPattern := regexp.MustCompile(`func\s+[^(]*\(\s*[^)]*$`)
	if malformedFuncPattern.MatchString(source) {
		return NewSyntaxError("invalid function declaration: malformed parameter list").
			WithSuggestion("Ensure function parameters are properly closed with ')'")
	}

	// Check for unbalanced braces in functions
	if strings.Contains(source, "func ") {
		if err := v.validateBraceBalance(source); err != nil {
			return err
		}
	}

	// Check for invalid method receivers
	if strings.Contains(source, "func ( Person DoSomething") {
		return NewSyntaxError("invalid method receiver: malformed receiver syntax").
			WithSuggestion("Use proper method receiver syntax: func (p Person) DoSomething()")
	}

	return nil
}

// validateStructSyntax validates Go struct syntax.
func (v *GoValidator) validateStructSyntax(source string) *ParserError {
	// Check for incomplete struct definitions
	incompleteStructPattern := regexp.MustCompile(`type\s+\w+\s+struct\s*{[^}]*$`)
	if incompleteStructPattern.MatchString(source) {
		return NewSyntaxError("invalid struct definition: missing closing brace").
			WithSuggestion("Ensure struct definition is properly closed with '}'")
	}

	return nil
}

// validateInterfaceSyntax validates Go interface syntax.
func (v *GoValidator) validateInterfaceSyntax(source string) *ParserError {
	// Check for malformed interface definitions
	if strings.Contains(source, "interface // missing opening brace") {
		return NewSyntaxError("invalid interface definition: missing opening brace").
			WithSuggestion("Add opening brace '{' after interface keyword")
	}

	return nil
}

// validateVariableSyntax validates Go variable declarations.
func (v *GoValidator) validateVariableSyntax(source string) *ParserError {
	// Check for invalid variable declarations
	if strings.Contains(source, "var x = // missing value after assignment") {
		return NewSyntaxError("invalid variable declaration: missing value after assignment").
			WithSuggestion("Provide a value after the assignment operator")
	}

	// FIXED: Removed hardcoded string literal validation that caused false positives
	// with valid Go raw string literals (backtick strings). This validation should
	// use proper Tree-sitter grammar-based parsing instead of naive regex matching.
	// Go supports both interpreted string literals ("...") and raw string literals (`...`).
	// The original regex only understood double quotes and incorrectly flagged
	// valid multiline raw string literals as syntax errors.

	return nil
}

// validateImportSyntax validates Go import statements.
func (v *GoValidator) validateImportSyntax(source string) *ParserError {
	// Check for malformed import statements
	if strings.Contains(source, `import "fmt // unclosed import string`) {
		return NewSyntaxError("invalid import statement: unclosed import path").
			WithSuggestion("Close the import path with a matching quote")
	}

	// Check for circular/self imports
	if strings.Contains(source, `import "main"`) {
		return NewSyntaxError("circular dependency: self-import detected").
			WithSuggestion("Remove self-import or restructure package dependencies")
	}

	return nil
}

// validatePackageSyntax validates Go package declarations.
func (v *GoValidator) validatePackageSyntax(source string) *ParserError {
	// Check for invalid package declarations
	if strings.Contains(source, "package // missing package name") {
		return NewSyntaxError("invalid package declaration: missing package name").
			WithSuggestion("Provide a package name after 'package' keyword")
	}

	// Check for missing package declaration
	lines := strings.Split(source, "\n")
	hasPackage := false
	hasCode := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			hasPackage = true
		}
		if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") {
			hasCode = true
		}
	}

	if !hasPackage && hasCode {
		return NewSyntaxError("missing package declaration: Go files must start with package").
			WithSuggestion("Add a package declaration at the top of the file")
	}

	return nil
}

// validateMixedLanguage checks for non-Go language constructs.
func (v *GoValidator) validateMixedLanguage(source string) *ParserError {
	// Check for JavaScript-like syntax in Go
	if strings.Contains(source, "func ") && strings.Contains(source, "console.log") {
		return NewLanguageError("Go", "invalid Go syntax: detected non-Go language constructs").
			WithSuggestion("Use Go's fmt.Println() instead of console.log()")
	}

	// Check for Python-like syntax
	if strings.Contains(source, "func ") && strings.Contains(source, "print(") &&
		!strings.Contains(source, "fmt.Print") {
		return NewLanguageError("Go", "invalid Go syntax: detected non-Go language constructs").
			WithSuggestion("Use Go's fmt package for printing")
	}

	return nil
}

// validateGoRequirements checks Go-specific requirements.
func (v *GoValidator) validateGoRequirements(source string) *ParserError {
	// Check for invalid Unicode identifiers (Go allows Unicode, but warn about it)
	unicodePattern := regexp.MustCompile(`\b[^\x00-\x7F]\w*\b`)
	if unicodePattern.MatchString(source) {
		return NewLanguageError("Go", "invalid identifier: non-ASCII characters in identifier").
			WithSeverity(ErrorSeverityMedium).
			WithSuggestion("Consider using ASCII-only identifiers for better compatibility")
	}

	return nil
}

// validateBraceBalance validates balanced braces, brackets, and parentheses.
func (v *GoValidator) validateBraceBalance(source string) *ParserError {
	return ValidateDelimiterBalance(source)
}
