package javascriptparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

// JSDocInfo holds extracted information from JSDoc comments.
type JSDocInfo struct {
	ParameterTypes map[string]string // Maps parameter name to type
	ReturnType     string            // Return type from @returns tag
	Description    string            // Function description
}

// extractJSDocFromFunction attempts to find and parse JSDoc comments before a function node.
func extractJSDocFromFunction(parseTree *valueobject.ParseTree, functionNode *valueobject.ParseNode) *JSDocInfo {
	// Look for a comment node before the function
	// In tree-sitter, comments are siblings of the function node in the program/block
	commentNode := findJSDocComment(parseTree, functionNode)
	if commentNode == nil {
		return nil
	}

	commentText := parseTree.GetNodeText(commentNode)
	return parseJSDocComment(commentText)
}

// findJSDocComment looks for a JSDoc comment (/** ... */) immediately before the given node.
func findJSDocComment(parseTree *valueobject.ParseTree, targetNode *valueobject.ParseNode) *valueobject.ParseNode {
	// Search backwards in the source from the function's start position
	// Look for /** */ comments before the function
	source := parseTree.Source()
	startByte := targetNode.StartByte

	// Look backwards from the function start to find a JSDoc comment
	// We'll search up to 500 bytes before the function for the comment
	searchStart := uint32(0)
	if startByte > 500 {
		searchStart = startByte - 500
	}

	beforeFunction := valueobject.SanitizeContent(string(source[searchStart:startByte]))

	// Find the last occurrence of /** before the function
	jsdocStart := strings.LastIndex(beforeFunction, "/**")
	if jsdocStart == -1 {
		return nil
	}

	// Find the matching */
	jsdocEnd := strings.Index(beforeFunction[jsdocStart:], "*/")
	if jsdocEnd == -1 {
		return nil
	}

	// Calculate absolute byte positions
	absoluteStart := searchStart + uint32(jsdocStart)
	absoluteEnd := searchStart + uint32(jsdocStart+jsdocEnd+2)

	// Create a synthetic ParseNode for the comment
	// (This is a simplified approach - ideally we'd find the actual comment node from the tree)
	commentNode := &valueobject.ParseNode{
		Type:      "comment",
		StartByte: absoluteStart,
		EndByte:   absoluteEnd,
	}

	return commentNode
}

// parseJSDocComment parses a JSDoc comment string and extracts type information.
func parseJSDocComment(commentText string) *JSDocInfo {
	ctx := context.Background()

	// Get JSDoc grammar
	grammar := forest.GetLanguage("jsdoc")
	if grammar == nil {
		return nil
	}

	// Create parser
	parser := tree_sitter.NewParser()
	if parser == nil {
		return nil
	}

	// Set language
	if !parser.SetLanguage(grammar) {
		return nil
	}

	// Parse the comment
	tree, err := parser.ParseString(ctx, nil, []byte(commentText))
	if err != nil {
		return nil
	}
	defer tree.Close()

	// Extract information from the parsed JSDoc AST
	info := &JSDocInfo{
		ParameterTypes: make(map[string]string),
	}

	root := tree.RootNode()
	source := []byte(commentText)

	// Walk through the document's tags
	for i := range root.ChildCount() {
		child := root.Child(i)
		if child.Type() == "tag" {
			processJSDocTag(child, source, info)
		}
	}

	return info
}

// processJSDocTag processes a single JSDoc tag node.
func processJSDocTag(tagNode tree_sitter.Node, source []byte, info *JSDocInfo) {
	var tagName, tagType, identifier string

	// Extract tag components
	for i := range tagNode.ChildCount() {
		child := tagNode.Child(i)
		childType := child.Type()
		childText := valueobject.SanitizeContent(string(source[child.StartByte():child.EndByte()]))

		switch childType {
		case "tag_name":
			tagName = childText
		case "type":
			tagType = childText
		case "identifier":
			identifier = childText
		case "description":
			// Optionally extract description
			if tagName == "" && info.Description == "" {
				info.Description = strings.TrimSpace(childText)
			}
		}
	}

	// Process based on tag name
	switch tagName {
	case "@param":
		if identifier != "" && tagType != "" {
			// Clean type by removing curly braces if present
			cleanedType := cleanJSDocType(tagType)
			info.ParameterTypes[identifier] = cleanedType
		}
	case "@returns", "@return":
		if tagType != "" {
			// Clean type by removing curly braces if present
			cleanedType := cleanJSDocType(tagType)
			info.ReturnType = cleanedType
		}
	}
}

// cleanJSDocType removes curly braces and whitespace from JSDoc type annotations.
func cleanJSDocType(typeStr string) string {
	// Remove leading/trailing whitespace
	cleaned := strings.TrimSpace(typeStr)

	// Remove curly braces if present
	cleaned = strings.TrimPrefix(cleaned, "{")
	cleaned = strings.TrimSuffix(cleaned, "}")

	return strings.TrimSpace(cleaned)
}

// applyJSDocTypes applies JSDoc type information to extracted parameters.
func applyJSDocTypes(parameters []outbound.Parameter, jsdocInfo *JSDocInfo) []outbound.Parameter {
	if jsdocInfo == nil || len(jsdocInfo.ParameterTypes) == 0 {
		return parameters
	}

	// Create a new slice with updated types
	result := make([]outbound.Parameter, len(parameters))
	for i, param := range parameters {
		result[i] = param
		if paramType, ok := jsdocInfo.ParameterTypes[param.Name]; ok {
			result[i].Type = paramType
		}
	}

	return result
}
