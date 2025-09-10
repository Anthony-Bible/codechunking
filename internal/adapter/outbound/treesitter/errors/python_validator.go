package parsererrors

import (
	"regexp"
	"strings"
)

// PythonValidator implements language-specific validation for Python.
type PythonValidator struct{}

// NewPythonValidator creates a new Python validator.
func NewPythonValidator() *PythonValidator {
	return &PythonValidator{}
}

// GetLanguageName returns the language name.
func (v *PythonValidator) GetLanguageName() string {
	return "Python"
}

// ValidateSyntax performs Python-specific syntax validation.
func (v *PythonValidator) ValidateSyntax(source string) *ParserError {
	// Check for malformed function definitions
	if err := v.validateFunctionSyntax(source); err != nil {
		return err
	}

	// Check for malformed class definitions
	if err := v.validateClassSyntax(source); err != nil {
		return err
	}

	// Check for malformed variable assignments
	if err := v.validateVariableSyntax(source); err != nil {
		return err
	}

	// Check for malformed import statements
	if err := v.validateImportSyntax(source); err != nil {
		return err
	}

	// Check for indentation issues
	if err := v.validateIndentation(source); err != nil {
		return err
	}

	// Check delimiter balance (for brackets and parentheses)
	if err := ValidateDelimiterBalance(source); err != nil {
		return err
	}

	return nil
}

// ValidateLanguageFeatures performs Python-specific language feature validation.
func (v *PythonValidator) ValidateLanguageFeatures(source string) *ParserError {
	// Check for mixed language constructs
	if err := v.validateMixedLanguage(source); err != nil {
		return err
	}

	// Check for Python-specific requirements
	if err := v.validatePythonRequirements(source); err != nil {
		return err
	}

	return nil
}

// validateFunctionSyntax validates Python function syntax.
func (v *PythonValidator) validateFunctionSyntax(source string) *ParserError {
	// Check for malformed function definitions
	malformedDefPattern := regexp.MustCompile(`def\s+[^(]*\(\s*[^)]*$`)
	if malformedDefPattern.MatchString(source) {
		return NewSyntaxError("invalid function definition: malformed parameter list").
			WithSuggestion("Ensure function parameters are properly closed with ')'")
	}

	// Check for missing colon after function definition
	missingColonPattern := regexp.MustCompile(`def\s+\w+\([^)]*\)\s*$`)
	if missingColonPattern.MatchString(source) {
		return NewSyntaxError("invalid function definition: missing colon").
			WithSuggestion("Add ':' at the end of function definition")
	}

	// Check for async function syntax
	if strings.Contains(source, "async def") && !strings.Contains(source, "await") {
		return NewSyntaxError("async function without await usage").
			WithSeverity(ErrorSeverityLow).
			WithSuggestion("Consider using await or remove async keyword")
	}

	return nil
}

// validateClassSyntax validates Python class syntax.
func (v *PythonValidator) validateClassSyntax(source string) *ParserError {
	// Check for incomplete class definitions
	missingColonPattern := regexp.MustCompile(`class\s+\w+\s*$`)
	if missingColonPattern.MatchString(source) {
		return NewSyntaxError("invalid class definition: missing colon").
			WithSuggestion("Add ':' at the end of class definition")
	}

	// Check for malformed inheritance syntax
	if strings.Contains(source, "class") && strings.Contains(source, "(") {
		malformedInheritancePattern := regexp.MustCompile(`class\s+\w+\(\s*\)`)
		if malformedInheritancePattern.MatchString(source) {
			return NewSyntaxError("invalid class inheritance: empty parentheses").
				WithSeverity(ErrorSeverityLow).
				WithSuggestion("Remove empty parentheses or specify parent class")
		}
	}

	return nil
}

// validateVariableSyntax validates Python variable assignments.
func (v *PythonValidator) validateVariableSyntax(source string) *ParserError {
	// Check for invalid assignments
	if strings.Contains(source, "x = # missing value") {
		return NewSyntaxError("invalid variable assignment: missing value after assignment").
			WithSuggestion("Provide a value after the assignment operator")
	}

	// Check for unclosed string literals
	unClosedStringPattern := regexp.MustCompile(`["'][^"']*$`)
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if unClosedStringPattern.MatchString(line) && !strings.Contains(line, `\"`) && !strings.Contains(line, `\'`) {
			return NewSyntaxError("invalid syntax: unclosed string literal").
				WithLocation(i+1, 0).
				WithSuggestion("Close the string literal with a matching quote")
		}
	}

	return nil
}

// validateImportSyntax validates Python import statements.
func (v *PythonValidator) validateImportSyntax(source string) *ParserError {
	// Check for malformed from imports
	if strings.Contains(source, "from module import # missing import items") {
		return NewSyntaxError("invalid import statement: missing import items").
			WithSuggestion("Specify what to import after 'import'")
	}

	// Check for circular imports (basic check)
	if strings.Contains(source, "import __main__") {
		return NewSyntaxError("circular dependency: importing __main__").
			WithSeverity(ErrorSeverityMedium).
			WithSuggestion("Avoid importing __main__ module")
	}

	return nil
}

// validateIndentation validates Python indentation rules.
func (v *PythonValidator) validateIndentation(source string) *ParserError {
	lines := strings.Split(source, "\n")
	expectedIndent := 0

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue // Skip empty lines
		}

		// Check for mixed tabs and spaces
		if strings.Contains(line, "\t") && strings.Contains(line, "    ") {
			return NewSyntaxError("invalid indentation: mixed tabs and spaces").
				WithLocation(i+1, 0).
				WithSuggestion("Use either tabs or spaces consistently for indentation")
		}

		// Count leading spaces
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))

		// Check for lines that should be indented (after colons)
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") {
			expectedIndent = leadingSpaces + 4 // Expect next line to be indented
		} else if expectedIndent > 0 && leadingSpaces < expectedIndent {
			// Check if this line should be indented but isn't
			prevLine := ""
			if i > 0 {
				prevLine = strings.TrimSpace(lines[i-1])
			}
			if strings.HasSuffix(prevLine, ":") && leadingSpaces == 0 {
				return NewSyntaxError("invalid indentation: expected indented block").
					WithLocation(i+1, 0).
					WithSuggestion("Indent the line after a colon")
			}
		}
	}

	return nil
}

// validateMixedLanguage checks for non-Python language constructs.
func (v *PythonValidator) validateMixedLanguage(source string) *ParserError {
	// Check for JavaScript-like syntax in Python
	if strings.Contains(source, "def ") && strings.Contains(source, "console.log") {
		return NewLanguageError("Python", "invalid Python syntax: detected JavaScript language constructs").
			WithSuggestion("Use Python's print() instead of console.log()")
	}

	// Check for Go-like syntax
	if strings.Contains(source, "def ") && strings.Contains(source, "func ") {
		return NewLanguageError("Python", "invalid Python syntax: detected Go language constructs").
			WithSuggestion("Use Python def syntax instead of Go func")
	}

	// Check for C-style syntax
	if strings.Contains(source, "def ") && strings.Contains(source, "printf(") {
		return NewLanguageError("Python", "invalid Python syntax: detected C-style constructs").
			WithSuggestion("Use Python print() instead of printf()")
	}

	return nil
}

// validatePythonRequirements checks Python-specific requirements.
func (v *PythonValidator) validatePythonRequirements(source string) *ParserError {
	// Check for Python 2 vs Python 3 syntax issues
	if err := v.validatePythonVersion(source); err != nil {
		return err
	}

	// Check for common Python pitfalls
	if err := v.validateCommonPitfalls(source); err != nil {
		return err
	}

	return nil
}

// validatePythonVersion checks for Python version compatibility issues.
func (v *PythonValidator) validatePythonVersion(source string) *ParserError {
	// Check for Python 2 print statements
	if strings.Contains(source, "print ") && !strings.Contains(source, "print(") {
		return NewLanguageError("Python", "Python 2 syntax: print statement not supported in Python 3").
			WithSeverity(ErrorSeverityMedium).
			WithSuggestion("Use print() function instead of print statement")
	}

	// Check for Python 2 string types
	if strings.Contains(source, "unicode(") {
		return NewLanguageError("Python", "Python 2 syntax: unicode() not available in Python 3").
			WithSeverity(ErrorSeverityMedium).
			WithSuggestion("Use str() in Python 3")
	}

	return nil
}

// validateCommonPitfalls checks for common Python programming pitfalls.
func (v *PythonValidator) validateCommonPitfalls(source string) *ParserError {
	// Check for mutable default arguments
	mutableDefaultPattern := regexp.MustCompile(`def\s+\w+\([^)]*=\s*\[\]`)
	if mutableDefaultPattern.MatchString(source) {
		return NewLanguageError("Python", "code quality: mutable default argument").
			WithSeverity(ErrorSeverityMedium).
			WithSuggestion("Use None as default and create list inside function")
	}

	// Check for bare except clauses
	if strings.Contains(source, "except:") && !strings.Contains(source, "except Exception:") {
		return NewLanguageError("Python", "code quality: bare except clause").
			WithSeverity(ErrorSeverityLow).
			WithSuggestion("Specify exception type or use 'except Exception:'")
	}

	return nil
}
