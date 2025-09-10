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

	return &outbound.SemanticCodeChunk{
		ChunkID:       utils.GenerateID("module", moduleName, nil),
		Type:          outbound.ConstructModule,
		Name:          moduleName,
		QualifiedName: moduleName,
		Language:      parseTree.Language(),
		StartByte:     0,
		EndByte:       utils.SafeUint32(len(source)),
		Content:       string(source),
		Documentation: documentation,
		Visibility:    outbound.Public,
		IsStatic:      true,
		Metadata:      metadata,
		ExtractedAt:   now,
		Hash:          utils.GenerateHash(string(source)),
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
		if child.Type == "expression_statement" {
			stringNode := findChildByType(child, "string")
			if stringNode != nil {
				docstring := parseTree.GetNodeText(stringNode)
				// Clean up the docstring (remove quotes and extra whitespace)
				docstring = strings.Trim(docstring, `"'`)
				docstring = strings.TrimSpace(docstring)
				return docstring
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
		if child.Type == "expression_statement" {
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
		case "identifier":
			varName = parseTree.GetNodeText(child)
		case "string":
			value = parseTree.GetNodeText(child)
			// Remove quotes
			value = strings.Trim(value, `"'`)
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

// extractShebang extracts shebang line if present.
func extractShebang(parseTree *valueobject.ParseTree) string {
	source := parseTree.Source()
	lines := strings.Split(string(source), "\n")

	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		return lines[0]
	}

	return ""
}

// extractModuleComments extracts module-level comments.
func extractModuleComments(parseTree *valueobject.ParseTree) []string {
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

// extractModuleLevelStatements extracts statistics about module-level statements.
func extractModuleLevelStatements(parseTree *valueobject.ParseTree) map[string]int {
	stats := make(map[string]int)
	rootNode := parseTree.RootNode()
	if rootNode == nil {
		return stats
	}

	// Count different types of statements
	for _, child := range rootNode.Children {
		switch child.Type {
		case "import_statement", "import_from_statement":
			stats["imports"]++
		case "function_definition", "async_function_definition":
			stats["functions"]++
		case "class_definition":
			stats["classes"]++
		case "expression_statement":
			// Check if it's an assignment
			if findChildByType(child, "assignment") != nil {
				stats["variables"]++
			}
		}
	}

	return stats
}

// Helper function to get a reasonable module name from the parse tree context.
func getModuleNameFromContext(parseTree *valueobject.ParseTree) string {
	// This is a minimal implementation
	// In a real implementation, this would be derived from the file path
	source := parseTree.Source()

	// Look for common patterns to infer module type
	if strings.Contains(string(source), "if __name__ == \"__main__\"") {
		return "main"
	}

	if strings.Contains(string(source), "def main()") {
		return "main"
	}

	// Check for class definitions to infer module purpose
	if strings.Contains(string(source), "class ") {
		return "module"
	}

	// Default
	return "utility"
}
