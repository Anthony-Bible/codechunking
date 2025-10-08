package javascriptparser

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	importTypeES6      = "es6"
	importTypeCommonJS = "commonjs"
	importTypeDynamic  = "dynamic"
	importTypeAMD      = "amd"
)

// extractES6Imports extracts ES6 static import statements (import ... from '...').
func extractES6Imports(parseTree *valueobject.ParseTree, source []byte, now time.Time) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	// Find all import_statement nodes
	importNodes := parseTree.GetNodesByType("import_statement")
	for _, node := range importNodes {
		// Extract the module path from the source field
		path := extractModulePath(parseTree, node)
		if path == "" {
			continue
		}

		content := parseTree.GetNodeText(node)
		startByte := node.StartByte
		endByte := node.EndByte

		metadata := make(map[string]interface{})
		metadata["import_type"] = importTypeES6
		metadata["path_type"] = getPathType(path)

		imports = append(imports, outbound.ImportDeclaration{
			Path:          path,
			StartByte:     startByte,
			EndByte:       endByte,
			StartPosition: valueobject.Position{Row: node.StartPos.Row, Column: node.StartPos.Column},
			EndPosition:   valueobject.Position{Row: node.EndPos.Row, Column: node.EndPos.Column},
			Content:       content,
			ExtractedAt:   now,
			Hash:          generateHash(content),
			Metadata:      metadata,
		})
	}

	return imports
}

// extractDynamicImports extracts dynamic import() calls.
func extractDynamicImports(
	parseTree *valueobject.ParseTree,
	source []byte,
	now time.Time,
) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	// Find all call_expression nodes
	callNodes := parseTree.GetNodesByType("call_expression")
	// Debug: log how many call nodes we found
	// This will help us understand why extraction returns 0
	for _, node := range callNodes {
		// Check if the function being called is "import"
		// The structure is: call_expression > import > import (nested)
		functionNode := findChildByType(node, "import")
		if functionNode == nil {
			// Not an import() call, try checking if it's a require() etc.
			continue
		}

		// Extract the module path from the arguments
		argsNode := findChildByType(node, "arguments")
		if argsNode == nil {
			continue
		}

		path := extractStringFromArguments(parseTree, argsNode)
		if path == "" {
			continue
		}

		content := parseTree.GetNodeText(node)
		startByte := node.StartByte
		endByte := node.EndByte

		metadata := make(map[string]interface{})
		metadata["import_type"] = importTypeDynamic
		metadata["is_dynamic"] = true
		metadata["path_type"] = getPathType(path)

		imports = append(imports, outbound.ImportDeclaration{
			Path:          path,
			StartByte:     startByte,
			EndByte:       endByte,
			StartPosition: valueobject.Position{Row: node.StartPos.Row, Column: node.StartPos.Column},
			EndPosition:   valueobject.Position{Row: node.EndPos.Row, Column: node.EndPos.Column},
			Content:       content,
			ExtractedAt:   now,
			Hash:          generateHash(content),
			Metadata:      metadata,
		})
	}

	return imports
}

// extractCommonJSImports extracts CommonJS require() calls.
func extractCommonJSImports(
	parseTree *valueobject.ParseTree,
	source []byte,
	now time.Time,
) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	// Find all call_expression nodes
	callNodes := parseTree.GetNodesByType("call_expression")
	for _, node := range callNodes {
		// Check if the function being called is "require"
		functionNode := findChildByType(node, "identifier")
		if functionNode == nil {
			continue
		}

		functionName := parseTree.GetNodeText(functionNode)
		if functionName != "require" {
			continue
		}

		// Extract the module path from the arguments
		argsNode := findChildByType(node, "arguments")
		if argsNode == nil {
			continue
		}

		path := extractStringFromArguments(parseTree, argsNode)
		if path == "" {
			continue
		}

		content := parseTree.GetNodeText(node)
		startByte := node.StartByte
		endByte := node.EndByte

		metadata := make(map[string]interface{})
		metadata["import_type"] = importTypeCommonJS
		metadata["path_type"] = getPathType(path)

		imports = append(imports, outbound.ImportDeclaration{
			Path:          path,
			StartByte:     startByte,
			EndByte:       endByte,
			StartPosition: valueobject.Position{Row: node.StartPos.Row, Column: node.StartPos.Column},
			EndPosition:   valueobject.Position{Row: node.EndPos.Row, Column: node.EndPos.Column},
			Content:       content,
			ExtractedAt:   now,
			Hash:          generateHash(content),
			Metadata:      metadata,
		})
	}

	return imports
}

// extractAMDImports extracts AMD require() calls only.
// Note: define() is treated as an export, not an import, so it's handled by module extraction.
func extractAMDImports(parseTree *valueobject.ParseTree, source []byte, now time.Time) []outbound.ImportDeclaration {
	var imports []outbound.ImportDeclaration

	// Find all call_expression nodes
	callNodes := parseTree.GetNodesByType("call_expression")
	for _, node := range callNodes {
		// Check if the function being called is "require" (not "define", which is an export)
		functionNode := findChildByType(node, "identifier")
		if functionNode == nil {
			continue
		}

		functionName := parseTree.GetNodeText(functionNode)
		if functionName != "require" {
			continue
		}

		// Extract module paths from array argument
		argsNode := findChildByType(node, "arguments")
		if argsNode == nil {
			continue
		}

		// Look for array in arguments (tree-sitter uses "array" not "array_expression")
		arrayNode := findChildByType(argsNode, "array")
		if arrayNode == nil {
			continue
		}

		// Extract each string in the array
		paths := extractStringsFromArray(parseTree, arrayNode)
		for _, path := range paths {
			if path == "" {
				continue
			}

			content := parseTree.GetNodeText(node)
			startByte := node.StartByte
			endByte := node.EndByte

			metadata := make(map[string]interface{})
			metadata["import_type"] = importTypeAMD
			metadata["amd_function"] = functionName
			metadata["path_type"] = getPathType(path)

			imports = append(imports, outbound.ImportDeclaration{
				Path:          path,
				StartByte:     startByte,
				EndByte:       endByte,
				StartPosition: valueobject.Position{Row: node.StartPos.Row, Column: node.StartPos.Column},
				EndPosition:   valueobject.Position{Row: node.EndPos.Row, Column: node.EndPos.Column},
				Content:       content,
				ExtractedAt:   now,
				Hash:          generateHash(content),
				Metadata:      metadata,
			})
		}
	}

	return imports
}

// extractModulePath extracts the module path from an import_statement node.
func extractModulePath(parseTree *valueobject.ParseTree, node *valueobject.ParseNode) string {
	// Look for a string node (the source)
	stringNode := findChildByType(node, "string")
	if stringNode == nil {
		return ""
	}

	// Get the text and strip quotes
	text := parseTree.GetNodeText(stringNode)
	return strings.Trim(text, `"'`)
}

// extractStringFromArguments extracts a string from an arguments node.
func extractStringFromArguments(parseTree *valueobject.ParseTree, argsNode *valueobject.ParseNode) string {
	// Find the first string child
	stringNode := findChildByType(argsNode, "string")
	if stringNode == nil {
		return ""
	}

	// Get the text and strip quotes
	text := parseTree.GetNodeText(stringNode)
	return strings.Trim(text, `"'`)
}

// extractStringsFromArray extracts all strings from an array node.
func extractStringsFromArray(parseTree *valueobject.ParseTree, arrayNode *valueobject.ParseNode) []string {
	var result []string

	// Find all string children
	stringNodes := findChildrenByType(arrayNode, "string")
	for _, stringNode := range stringNodes {
		text := parseTree.GetNodeText(stringNode)
		cleaned := strings.Trim(text, `"'`)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}

	return result
}

// extractJavaScriptImports extracts import declarations from JavaScript code using tree-sitter AST traversal.
func extractJavaScriptImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	slogger.Info(ctx, "Extracting imports from parse tree", slogger.Fields{
		"language": parseTree.Language().Name(),
	})

	var imports []outbound.ImportDeclaration
	now := time.Now()
	source := parseTree.Source()

	// Extract ES6 static imports
	es6Imports := extractES6Imports(parseTree, source, now)
	imports = append(imports, es6Imports...)

	// Extract dynamic imports (import(...))
	dynamicImports := extractDynamicImports(parseTree, source, now)
	slogger.Info(ctx, "Dynamic imports extracted", slogger.Fields{
		"count": len(dynamicImports),
	})
	imports = append(imports, dynamicImports...)

	// Extract CommonJS require() calls
	commonJSImports := extractCommonJSImports(parseTree, source, now)
	imports = append(imports, commonJSImports...)

	// Extract AMD require/define calls
	amdImports := extractAMDImports(parseTree, source, now)
	imports = append(imports, amdImports...)

	slogger.Info(ctx, "JavaScript import extraction completed", slogger.Fields{
		"extracted_count": len(imports),
	})

	return imports, nil
}

// ExtractJavaScriptImports extracts import declarations from JavaScript code using regex patterns.
// This is a public wrapper for the private extractJavaScriptImports function.
func ExtractJavaScriptImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	return extractJavaScriptImports(ctx, parseTree, options)
}

// Helper functions for tree-sitter node traversal

func getPathType(path string) string {
	if strings.HasPrefix(path, "@") {
		return "scoped-npm"
	}
	if strings.HasPrefix(path, ".") {
		return "relative"
	}
	if strings.HasPrefix(path, "/") {
		return "absolute"
	}
	if strings.Contains(path, "://") {
		return "protocol"
	}
	return "npm"
}

func generateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func findChildByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	if node == nil {
		return nil
	}

	for _, child := range node.Children {
		if child.Type == nodeType {
			return child
		}
	}
	return nil
}

func findChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	if node == nil {
		return nil
	}

	var matches []*valueobject.ParseNode
	for _, child := range node.Children {
		if child.Type == nodeType {
			matches = append(matches, child)
		}
	}
	return matches
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
