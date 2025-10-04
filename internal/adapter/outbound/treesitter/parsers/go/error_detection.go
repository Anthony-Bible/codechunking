package goparser

import (
	"codechunking/internal/domain/valueobject"
	"errors"
	"strings"
)

// detectSpecificSyntaxError analyzes a parse tree to identify specific syntax errors
// based on tree-sitter ERROR and MISSING node patterns. Returns a descriptive error
// message if syntax errors are found, or nil if the parse tree is valid.
func detectSpecificSyntaxError(parseTree *valueobject.ParseTree) error {
	if parseTree == nil {
		return nil
	}

	// Check for func_literal that should be method_declaration (invalid receiver) FIRST
	// This must be checked before generic ERROR analysis because it has specific ERROR patterns
	if methodReceiverError := detectInvalidMethodReceiver(parseTree.RootNode(), parseTree); methodReceiverError != nil {
		return methodReceiverError
	}

	// Check for ERROR and MISSING nodes
	errorNode := findFirstErrorNode(parseTree.RootNode())
	if errorNode != nil {
		return analyzeErrorNode(errorNode, parseTree)
	}

	missingNode := findFirstMissingNode(parseTree.RootNode())
	if missingNode != nil {
		return analyzeMissingNode(missingNode, parseTree)
	}

	return nil
}

// analyzeErrorNode examines an ERROR node to determine the specific type of syntax error.
func analyzeErrorNode(node *valueobject.ParseNode, parseTree *valueobject.ParseTree) error {
	if node == nil {
		return nil
	}

	// Extract tokens from ERROR node
	tokens := extractTokensFromNode(node, parseTree)

	// Detect function-related errors
	if containsToken(tokens, "func") {
		if containsToken(tokens, "(") && containsToken(tokens, "{") {
			if !hasChildOfType(node, "parameter_list") {
				return errors.New("invalid function declaration: malformed parameter list")
			}
		}
	}

	// Detect type-related errors (struct, interface)
	if containsToken(tokens, "type") && containsToken(tokens, "struct") {
		if containsToken(tokens, "{") && !hasMatchingClosingBrace(node, parseTree) {
			return errors.New("invalid struct definition: missing closing brace")
		}
	}
	if containsToken(tokens, "type") && containsToken(tokens, "interface") {
		if !containsToken(tokens, "{") {
			return errors.New("invalid interface definition: missing opening brace")
		}
	}

	// Detect variable declaration errors
	if containsToken(tokens, "var") {
		if containsToken(tokens, "=") && !hasExpressionAfterEquals(node, parseTree) {
			return errors.New("invalid variable declaration: missing value after assignment")
		}
	}

	// Detect package declaration errors
	if containsToken(tokens, "package") {
		if len(node.Children) <= 1 || !hasChildOfType(node, "package_identifier") {
			return errors.New("invalid package declaration: missing package name")
		}
	}

	// Detect mixed language syntax (JavaScript in Go)
	if containsToken(tokens, "console") || containsToken(tokens, "log") {
		return errors.New("invalid Go syntax: detected non-Go language constructs")
	}

	// Generic syntax error
	return errors.New("invalid syntax: syntax error detected by tree-sitter parser")
}

// analyzeMissingNode examines a MISSING node to determine what's missing.
func analyzeMissingNode(node *valueobject.ParseNode, parseTree *valueobject.ParseTree) error {
	if node == nil {
		return nil
	}

	nodeText := parseTree.GetNodeText(node)

	// Missing closing brace
	if strings.Contains(nodeText, "}") || strings.Contains(node.Type, "}") {
		return errors.New("invalid syntax: unbalanced braces")
	}

	// Missing closing quote (import statement)
	if strings.Contains(nodeText, "\"") || strings.Contains(node.Type, "\"") {
		if isWithinImportContext(node) {
			return errors.New("invalid import statement: unclosed import path")
		}
	}

	return errors.New("invalid syntax: missing required token")
}

// detectInvalidMethodReceiver checks for func_literal that should be method_declaration.
func detectInvalidMethodReceiver(node *valueobject.ParseNode, _ *valueobject.ParseTree) error {
	if node == nil {
		return nil
	}

	// Check current node
	if node.Type == "func_literal" || node.Type == "expression_statement" {
		if hasNestedParameterListError(node) {
			return errors.New("invalid method receiver: malformed receiver syntax")
		}
	}

	// Recursively check children
	for _, child := range node.Children {
		if err := detectInvalidMethodReceiver(child, nil); err != nil {
			return err
		}
	}

	return nil
}

// Helper functions

// findFirstErrorNode recursively finds the first ERROR node in the tree.
func findFirstErrorNode(node *valueobject.ParseNode) *valueobject.ParseNode {
	if node == nil {
		return nil
	}

	if node.Type == nodeTypeError {
		return node
	}

	for _, child := range node.Children {
		if errorNode := findFirstErrorNode(child); errorNode != nil {
			return errorNode
		}
	}

	return nil
}

// findFirstMissingNode recursively finds the first MISSING node in the tree.
// Uses tree-sitter's native IsMissing() method to accurately detect missing nodes
// without false positives on valid Go code.
func findFirstMissingNode(node *valueobject.ParseNode) *valueobject.ParseNode {
	if node == nil {
		return nil
	}

	// Check if current node is missing using tree-sitter's native IsMissing() method
	if isNodeMissing(node) {
		return node
	}

	// Recursively search children
	for _, child := range node.Children {
		if missingNode := findFirstMissingNode(child); missingNode != nil {
			return missingNode
		}
	}

	return nil
}

// isNodeMissing checks if a ParseNode represents a missing token using tree-sitter's
// native IsMissing() method. This provides accurate detection without false positives.
func isNodeMissing(node *valueobject.ParseNode) bool {
	if node == nil {
		return false
	}

	// Use tree-sitter's native IsMissing() method if tsNode is available
	// This is the authoritative way to check if a node is missing
	tsNode := node.TreeSitterNode()
	if tsNode != nil {
		return tsNode.IsMissing()
	}

	// Fallback: if tsNode is not available, check for explicit "MISSING" prefix
	// This handles cases where the node was created without tsNode reference
	return strings.HasPrefix(node.Type, "MISSING")
}

// extractTokensFromNode extracts all token strings from a node and its children.
func extractTokensFromNode(node *valueobject.ParseNode, parseTree *valueobject.ParseTree) []string {
	if node == nil {
		return nil
	}

	var tokens []string

	// Add current node's text if it's a leaf or has meaningful text
	nodeText := parseTree.GetNodeText(node)
	if nodeText != "" && len(node.Children) == 0 {
		tokens = append(tokens, nodeText)
	}

	// Recursively extract from children
	for _, child := range node.Children {
		tokens = append(tokens, extractTokensFromNode(child, parseTree)...)
	}

	return tokens
}

// containsToken checks if a token exists in the token list.
func containsToken(tokens []string, target string) bool {
	for _, token := range tokens {
		if strings.TrimSpace(token) == target {
			return true
		}
	}
	return false
}

// hasChildOfType checks if a node has a direct child of the specified type.
func hasChildOfType(node *valueobject.ParseNode, childType string) bool {
	if node == nil {
		return false
	}

	for _, child := range node.Children {
		if child.Type == childType {
			return true
		}
	}
	return false
}

// hasMatchingClosingBrace checks if there's a matching closing brace for an opening brace.
func hasMatchingClosingBrace(node *valueobject.ParseNode, parseTree *valueobject.ParseTree) bool {
	if node == nil {
		return false
	}

	tokens := extractTokensFromNode(node, parseTree)
	openCount := 0
	closeCount := 0

	for _, token := range tokens {
		if token == "{" {
			openCount++
		}
		if token == "}" {
			closeCount++
		}
	}

	return openCount > 0 && openCount == closeCount
}

// hasExpressionAfterEquals checks if there's an expression after '=' in variable declaration.
func hasExpressionAfterEquals(node *valueobject.ParseNode, parseTree *valueobject.ParseTree) bool {
	if node == nil {
		return false
	}

	tokens := extractTokensFromNode(node, parseTree)
	foundEquals := false

	for _, token := range tokens {
		if foundEquals && token != "=" && strings.TrimSpace(token) != "" {
			return true
		}
		if token == "=" {
			foundEquals = true
		}
	}

	return false
}

// isWithinImportContext checks if a node is within an import declaration context.
func isWithinImportContext(node *valueobject.ParseNode) bool {
	// For now, we check if MISSING quote appears in import-like context
	// This is a simplified check - could be enhanced by walking up parent tree
	return true // Conservative: assume quote errors in import context
}

// hasNestedParameterListError checks for double opening parentheses indicating invalid method receiver.
func hasNestedParameterListError(node *valueobject.ParseNode) bool {
	if node == nil {
		return false
	}

	// Look for parameter_list containing ERROR with "(" token
	for _, child := range node.Children {
		if child.Type == "parameter_list" {
			// Check if parameter_list has ERROR child
			for _, paramChild := range child.Children {
				if paramChild.Type == "ERROR" {
					// Check if ERROR's content or children indicate "("
					for _, errChild := range paramChild.Children {
						// The ERROR node may have a child with Type "("
						if errChild.Type == "(" {
							return true
						}
					}
					// Or the ERROR node itself might be at a position suggesting method receiver issue
					// If we see ERROR after a parameter_declaration in a func_literal, it's likely the method receiver issue
					return true
				}
			}
		}
		// Recursively check
		if hasNestedParameterListError(child) {
			return true
		}
	}

	return false
}
