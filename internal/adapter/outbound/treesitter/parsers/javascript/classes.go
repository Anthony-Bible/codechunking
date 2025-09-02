package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	nodeTypeIdentifier = "identifier"
)

// extractJavaScriptClasses extracts JavaScript classes from the parse tree using real AST analysis.
func extractJavaScriptClasses(
	_ context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	if parseTree == nil {
		return nil, errors.New("parse tree cannot be nil")
	}

	if parseTree.Language().Name() != "JavaScript" {
		return nil, fmt.Errorf("unsupported language: %s", parseTree.Language().Name())
	}

	classes := make([]outbound.SemanticCodeChunk, 0)

	// Extract module name from source code
	moduleName := extractModuleName(parseTree)

	// Find all class declarations in the parse tree
	classNodes := parseTree.GetNodesByType("class_declaration")

	for _, classNode := range classNodes {
		class := extractClassFromNode(parseTree, classNode, moduleName, options)
		if class != nil {
			classes = append(classes, *class)
		}
	}

	return classes, nil
}

// extractClassFromNode extracts a class from a class_declaration node.
func extractClassFromNode(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	moduleName string,
	options outbound.SemanticExtractionOptions,
) *outbound.SemanticCodeChunk {
	// Extract class name
	className := extractClassName(parseTree, node)
	if className == "" {
		return nil
	}

	// Check visibility based on naming conventions
	visibility := getJavaScriptVisibility(className)
	if !options.IncludePrivate && visibility == outbound.Private {
		return nil
	}

	now := time.Now()
	qualifiedName := fmt.Sprintf("%s.%s", moduleName, className)

	// Extract inheritance info
	var dependencies []outbound.DependencyReference
	if superClass := extractSuperClass(parseTree, node); superClass != "" {
		dependencies = append(dependencies, outbound.DependencyReference{
			Name: superClass,
			Type: "inheritance",
		})
	}

	// Check for private members
	metadata := make(map[string]interface{})
	if hasPrivateMembers(parseTree, node) {
		metadata["has_private_members"] = true
	}

	return &outbound.SemanticCodeChunk{
		ID:            utils.GenerateID(string(outbound.ConstructClass), className, nil),
		Type:          outbound.ConstructClass,
		Name:          className,
		QualifiedName: qualifiedName,
		Language:      parseTree.Language(),
		StartByte:     node.StartByte,
		EndByte:       node.EndByte,
		Content:       generateClassContent(parseTree, node),
		Visibility:    visibility,
		Dependencies:  dependencies,
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(qualifiedName),
	}
}

// extractClassName extracts the class name from a class_declaration node.
func extractClassName(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Look for identifier child node
	for _, child := range node.Children {
		if child.Type == nodeTypeIdentifier {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
}

// extractSuperClass extracts the superclass name if the class extends another class.
func extractSuperClass(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Look for class_heritage node which contains "extends" information
	for _, child := range node.Children {
		if child.Type == "class_heritage" {
			// Find the identifier after "extends"
			for _, heritageChild := range child.Children {
				if heritageChild.Type == nodeTypeIdentifier {
					return parseTree.GetNodeText(heritageChild)
				}
			}
		}
	}
	return ""
}

// hasPrivateMembers checks if the class has private fields or methods.
func hasPrivateMembers(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) bool {
	content := parseTree.GetNodeText(node)
	// Check for ES2022 private fields (#) or convention-based private (_)
	return strings.Contains(content, "#") || strings.Contains(content, "_")
}

// generateClassContent generates a content summary for the class.
func generateClassContent(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	content := parseTree.GetNodeText(node)

	// Truncate for display purposes
	const maxContentLength = 200
	if len(content) > maxContentLength {
		content = content[:maxContentLength] + " ... }"
	}

	// Clean up whitespace
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\t", " ")
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	return strings.TrimSpace(content)
}
