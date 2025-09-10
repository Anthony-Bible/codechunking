package parsererrors

// ValidateDelimiterBalance validates balanced braces, brackets, and parentheses for any language.
func ValidateDelimiterBalance(source string) *ParserError {
	braceCount := 0
	parenCount := 0
	bracketCount := 0

	for i, char := range source {
		switch char {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '(':
			parenCount++
		case ')':
			parenCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}

		// Check for negative counts (more closing than opening)
		if braceCount < 0 {
			return NewSyntaxError("invalid syntax: unbalanced braces").
				WithDetails("position", i).
				WithSuggestion("Check for missing opening brace")
		}
		if parenCount < 0 {
			return NewSyntaxError("invalid syntax: unbalanced parentheses").
				WithDetails("position", i).
				WithSuggestion("Check for missing opening parenthesis")
		}
		if bracketCount < 0 {
			return NewSyntaxError("invalid syntax: unbalanced brackets").
				WithDetails("position", i).
				WithSuggestion("Check for missing opening bracket")
		}
	}

	// Check for unmatched opening delimiters
	if braceCount > 0 {
		return NewSyntaxError("invalid syntax: unbalanced braces").
			WithSuggestion("Check for missing closing brace")
	}
	if parenCount > 0 {
		return NewSyntaxError("invalid syntax: unbalanced parentheses").
			WithSuggestion("Check for missing closing parenthesis")
	}
	if bracketCount > 0 {
		return NewSyntaxError("invalid syntax: unbalanced brackets").
			WithSuggestion("Check for missing closing bracket")
	}

	return nil
}
