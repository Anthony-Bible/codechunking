package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"time"
)

// extractPythonModules extracts Python module information from the parse tree.
func extractPythonModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var modules []outbound.SemanticCodeChunk
	now := time.Now()

	// Extract the main module information
	module := extractMainModule(parseTree, options, now)
	if module != nil {
		modules = append(modules, *module)
	}

	return modules, nil
}

// extractMainModule extracts information about the main Python module/file.
func extractMainModule(
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	moduleName := extractModuleName(parseTree)

	// Extract module documentation (module-level docstring)
	var documentation string
	if options.IncludeDocumentation {
		documentation = extractModuleDocstring(parseTree)
	}

	// Extract module metadata
	var metadata map[string]interface{}
	if options.IncludeMetadata {
		metadata = extractModuleMetadata(parseTree)
	}

	// Get full source content
	source := parseTree.Source()

	// Sanitize content for PostgreSQL UTF-8 compatibility
	sanitizedContent := valueobject.SanitizeContent(string(source))

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("module", moduleName, nil),
		Type:          outbound.ConstructModule,
		Name:          moduleName,
		QualifiedName: moduleName,
		Language:      parseTree.Language(),
		StartByte:     0,
		EndByte:       utils.SafeUint32(len(source)),
		Content:       sanitizedContent,
		Documentation: documentation,
		Visibility:    outbound.Public,
		IsStatic:      true,
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(sanitizedContent),
	}
}

// extractModuleDocstring extracts the module-level docstring.
func extractModuleDocstring(parseTree *valueobject.ParseTree) string {
	rootNode := parseTree.RootNode()
	if rootNode == nil {
		return ""
	}

	// Look for the first string literal at module level (docstring)
	for _, child := range rootNode.Children {
		if child.Type == nodeTypeExpressionStatement {
			stringNode := findChildByType(child, nodeTypeString)
			if stringNode != nil {
				// Unescape quotes in docstrings to match Python's behavior
				return extractStringContent(parseTree, stringNode)
			}
		}
	}

	return ""
}

// extractModuleMetadata extracts metadata from module-level variables.
func extractModuleMetadata(parseTree *valueobject.ParseTree) map[string]interface{} {
	metadata := make(map[string]interface{})
	rootNode := parseTree.RootNode()
	if rootNode == nil {
		return metadata
	}

	// Look for common module-level metadata variables
	for _, child := range rootNode.Children {
		if child.Type == nodeTypeExpressionStatement {
			assignmentNode := findChildByType(child, "assignment")
			if assignmentNode != nil {
				key, value := extractMetadataFromAssignment(parseTree, assignmentNode)
				if key != "" {
					metadata[key] = value
				}
			}
		}
	}

	return metadata
}

// extractMetadataFromAssignment extracts metadata key-value pairs from assignments.
func extractMetadataFromAssignment(
	parseTree *valueobject.ParseTree,
	assignmentNode *valueobject.ParseNode,
) (string, interface{}) {
	if assignmentNode == nil {
		return "", nil
	}

	var varName, value string

	// Extract variable name and value
	for _, child := range assignmentNode.Children {
		switch child.Type {
		case nodeTypeIdentifier:
			varName = parseTree.GetNodeText(child)
		case nodeTypeString:
			// Extract string content using tree-sitter navigation
			// Unescape quotes for display in module metadata
			value = extractStringContent(parseTree, child)
		}
	}

	// Check for common metadata variables
	switch varName {
	case "__version__":
		return "version", value
	case "__author__":
		return "author", value
	case "__email__":
		return "email", value
	case "__license__":
		return "license", value
	case "__copyright__":
		return "copyright", value
	case "__maintainer__":
		return "maintainer", value
	case "__status__":
		return "status", value
	case "__credits__":
		return "credits", value
	default:
		if strings.HasPrefix(varName, "__") && strings.HasSuffix(varName, "__") {
			// Generic dunder variable
			key := strings.Trim(varName, "_")
			return key, value
		}
	}

	return "", nil
}

// extractModuleComments extracts module-level comments.
func extractModuleComments(parseTree *valueobject.ParseTree) []string {
	if parseTree == nil {
		return []string{}
	}

	var comments []string
	rootNode := parseTree.RootNode()
	if rootNode == nil {
		return comments
	}

	// Look for comment nodes at module level
	for _, child := range rootNode.Children {
		if child.Type == "comment" {
			comment := parseTree.GetNodeText(child)
			// Remove # prefix and trim whitespace
			comment = strings.TrimPrefix(comment, "#")
			comment = strings.TrimSpace(comment)
			if comment != "" {
				comments = append(comments, comment)
			}
		}
	}

	return comments
}
