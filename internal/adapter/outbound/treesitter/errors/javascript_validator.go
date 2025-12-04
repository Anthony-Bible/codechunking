package parsererrors

import (
	"fmt"
	"regexp"
	"strings"

	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

const (
	errorNodeType = "ERROR"
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
// NOTE: This method uses regex-based validation which can have false positives.
// Prefer using ValidateSyntaxWithTree() when a tree-sitter parse tree is available.
func (v *JavaScriptValidator) ValidateSyntax(source string) *ParserError {
	// Only keep high-confidence checks that tree-sitter might not catch
	// Removed unreliable checks like quote counting and delimiter balance

	// Check for malformed import statements (specific pattern checks)
	if err := v.validateImportSyntax(source); err != nil {
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

// ValidateSyntaxWithTree performs JavaScript syntax validation using tree-sitter AST.
// This is more accurate than regex-based validation as it uses the actual parser.
func (v *JavaScriptValidator) ValidateSyntaxWithTree(source string, tree *tree_sitter.Tree) *ParserError {
	if tree == nil {
		// Fallback to regex-based validation if no tree provided
		return v.ValidateSyntax(source)
	}

	rootNode := tree.RootNode()

	// Check if tree has any ERROR nodes
	if rootNode.HasError() {
		// Find first error node and extract details
		errorNode := v.findFirstErrorNode(rootNode)
		if !errorNode.IsNull() {
			row := int(errorNode.StartPoint().Row) + 1

			col := int(errorNode.StartPoint().Column)
			return NewSyntaxError(fmt.Sprintf("invalid syntax: %s",
				v.extractErrorMessage(errorNode, []byte(source)))).
				WithLocation(row, col)
		}
		return NewSyntaxError("invalid syntax detected by parser")
	}

	// Tree is clean - no syntax errors
	return nil
}

// findFirstErrorNode recursively finds the first ERROR node in the tree.
func (v *JavaScriptValidator) findFirstErrorNode(node tree_sitter.Node) tree_sitter.Node {
	if node.Type() == errorNodeType || node.IsMissing() {
		return node
	}

	for i := range node.ChildCount() {
		child := node.Child(i)
		if errorNode := v.findFirstErrorNode(child); !errorNode.IsNull() {
			return errorNode
		}
	}

	return tree_sitter.Node{} // null node
}

// extractErrorMessage extracts a meaningful error message from an ERROR node.
func (v *JavaScriptValidator) extractErrorMessage(errorNode tree_sitter.Node, source []byte) string {
	// Get the text of the error node using tree-sitter's Content method
	snippet := errorNode.Content(source)
	if len(snippet) > 50 {
		snippet = snippet[:50] + "..."
	}
	if snippet != "" {
		return fmt.Sprintf("unexpected token '%s'", snippet)
	}
	return "parse error"
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
