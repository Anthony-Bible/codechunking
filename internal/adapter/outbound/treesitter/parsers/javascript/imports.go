package javascriptparser

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
)

// extractJavaScriptImports extracts import declarations from JavaScript code using regex patterns.
func extractJavaScriptImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	slogger.Debug(ctx, "Starting JavaScript import extraction", slogger.Fields{})

	var imports []outbound.ImportDeclaration
	now := time.Now()

	// Get the full source code text
	source := string(parseTree.Source())

	slogger.Info(ctx, "Source code to parse", slogger.Fields{
		"source_length":  len(source),
		"source_preview": source[:min(200, len(source))],
	})

	// ES6 import patterns
	importPatterns := []string{
		`import\s+(?:\*\s+as\s+(\w+)|{([^}]+)}|(\w+))\s+from\s+['"]([^'"]+)['"]`,
		`import\s+['"]([^'"]+)['"]`,
		`import\s+(?:\*\s+as\s+(\w+)|{([^}]+)}|(\w+))\s+from\s+['"]([^'"]+)['"]\s+assert\s+{[^}]+}`,
	}

	for _, pattern := range importPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(source, -1)

		for _, match := range matches {
			startIndex := strings.Index(source, match[0])
			endIndex := startIndex + len(match[0])

			declaration := createImportDeclaration(source, match, startIndex, endIndex, now, "es6")
			if declaration.Path != "" {
				imports = append(imports, declaration)
			}
		}
	}

	// CommonJS require patterns
	requirePatterns := []string{
		`(?:const|var|let)\s+({[^}]+}|[\w$]+)\s*=\s*require\(['"]([^'"]+)['"]\)`,
		`if\s*\([^)]*\)\s*{[^}]*require\(['"]([^'"]+)['"]\)[^}]*}`,
		`try\s*{[^}]*require\(['"]([^'"]+)['"]\)[^}]*}\s*catch\s*{`,
	}

	for _, pattern := range requirePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(source, -1)

		for _, match := range matches {
			startIndex := strings.Index(source, match[0])
			endIndex := startIndex + len(match[0])

			declaration := createImportDeclaration(source, match, startIndex, endIndex, now, "commonjs")
			if declaration.Path != "" {
				imports = append(imports, declaration)
			}
		}
	}

	// Dynamic import patterns
	dynamicImportPatterns := []string{
		`import\(['"][^'"]+['"]\)`,
		`import\([^)]+\)`,
		`await\s+import\(['"][^'"]+['"]\)`,
	}

	for _, pattern := range dynamicImportPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringIndex(source, -1)

		for _, match := range matches {
			startIndex := match[0]
			endIndex := match[1]
			content := source[startIndex:endIndex]

			declaration := createDynamicImportDeclaration(source, content, startIndex, endIndex, now)
			if declaration.Path != "" {
				imports = append(imports, declaration)
			}
		}
	}

	slogger.Debug(ctx, "JavaScript import extraction completed", slogger.Fields{
		"imports_count": len(imports),
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

func createImportDeclaration(
	source string,
	match []string,
	startByte, endByte int,
	now time.Time,
	importType string,
) outbound.ImportDeclaration {
	var path, alias string
	var importedSymbols []string
	var isWildcard bool
	metadata := make(map[string]interface{})

	// Extract path (always the last non-empty group)
	for i := len(match) - 1; i >= 0; i-- {
		if match[i] != "" && (strings.Contains(match[i], "'") || strings.Contains(match[i], "\"")) {
			path = strings.Trim(match[i], `"'`)
			break
		}
		if match[i] != "" && !strings.Contains(match[0], match[i]) {
			path = match[i]
			break
		}
	}

	// Handle different import styles
	switch importType {
	case "es6":
		if len(match) > 4 && match[1] != "" {
			// Namespace import
			alias = match[1]
			isWildcard = true
			importedSymbols = append(importedSymbols, "*")
			metadata["import_style"] = "namespace"
		} else if len(match) > 4 && match[2] != "" {
			// Named imports
			symbols := parseNamedImports(match[2])
			importedSymbols = append(importedSymbols, symbols.Names...)
			metadata["aliases"] = symbols.Aliases
			metadata["import_style"] = "named"
		} else if len(match) > 4 && match[3] != "" {
			// Default import
			alias = match[3]
			importedSymbols = append(importedSymbols, alias)
			metadata["import_style"] = "default"
		} else if len(match) > 1 && !strings.Contains(match[0], "{") && !strings.Contains(match[0], "*") {
			// Side-effect import
			metadata["import_style"] = "side-effect"
			metadata["has_side_effects"] = true
		}
		metadata["import_type"] = "es6"
	case "commonjs":
		if len(match) > 2 {
			if strings.HasPrefix(match[1], "{") {
				// Destructuring require
				symbols := parseNamedImports(strings.Trim(match[1], "{}"))
				importedSymbols = append(importedSymbols, symbols.Names...)
				metadata["aliases"] = symbols.Aliases
				metadata["import_style"] = "named"
				metadata["has_nested_destructuring"] = strings.Contains(match[1], "{") &&
					strings.Count(match[1], "{") > 1
			} else {
				// Simple require
				alias = match[1]
				importedSymbols = append(importedSymbols, alias)
				metadata["import_style"] = "default"
			}
			path = match[2]
		} else {
			path = match[1]
		}
		metadata["import_type"] = "commonjs"
		metadata["is_conditional"] = strings.Contains(match[0], "if")
		metadata["condition_type"] = "if"
	}

	// Add path metadata
	metadata["path_type"] = getPathType(path)
	metadata["relative_depth"] = getRelativeDepth(path)
	metadata["is_scoped_package"] = strings.HasPrefix(path, "@")

	// Add assertion metadata if present
	metadata["has_assertions"] = strings.Contains(match[0], "assert")
	if strings.Contains(match[0], "json") {
		metadata["assertion_type"] = "json"
	}

	content := source[startByte:endByte]
	hash := generateHash(content)

	return outbound.ImportDeclaration{
		Path:            path,
		Alias:           alias,
		IsWildcard:      isWildcard,
		ImportedSymbols: importedSymbols,
		StartByte:       valueobject.ClampToUint32(startByte),
		EndByte:         valueobject.ClampToUint32(endByte),
		StartPosition:   valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(startByte)},
		EndPosition:     valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(endByte)},
		Content:         content,
		ExtractedAt:     now,
		Hash:            hash,
		Metadata:        metadata,
	}
}

func createDynamicImportDeclaration(
	source, content string,
	startByte, endByte int,
	now time.Time,
) outbound.ImportDeclaration {
	metadata := make(map[string]interface{})
	metadata["import_type"] = "dynamic"
	metadata["is_dynamic"] = true

	// Extract path from import()
	path := ""
	re := regexp.MustCompile(`import\(['"]([^'"]+)['"]\)`)
	if matches := re.FindStringSubmatch(content); len(matches) > 1 {
		path = matches[1]
	}

	// Handle template literal paths
	hasTemplate := strings.Contains(content, "+") || strings.Contains(content, "`")
	metadata["has_template_path"] = hasTemplate

	// Add path metadata
	metadata["path_type"] = getPathType(path)
	metadata["relative_depth"] = getRelativeDepth(path)
	metadata["is_scoped_package"] = strings.HasPrefix(path, "@")

	hash := generateHash(content)

	return outbound.ImportDeclaration{
		Path:          path,
		StartByte:     valueobject.ClampToUint32(startByte),
		EndByte:       valueobject.ClampToUint32(endByte),
		StartPosition: valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(startByte)},
		EndPosition:   valueobject.Position{Row: 0, Column: valueobject.ClampToUint32(endByte)},
		Content:       content,
		ExtractedAt:   now,
		Hash:          hash,
		Metadata:      metadata,
	}
}

type NamedImportSymbols struct {
	Names   []string
	Aliases map[string]string
}

func parseNamedImports(importClause string) NamedImportSymbols {
	symbols := NamedImportSymbols{
		Names:   []string{},
		Aliases: make(map[string]string),
	}

	parts := strings.Split(importClause, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "as") {
			aliasParts := strings.Split(part, "as")
			if len(aliasParts) == 2 {
				name := strings.TrimSpace(aliasParts[0])
				alias := strings.TrimSpace(aliasParts[1])
				symbols.Names = append(symbols.Names, alias)
				symbols.Aliases[alias] = name
			}
		} else {
			symbols.Names = append(symbols.Names, part)
		}
	}

	return symbols
}

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

func getRelativeDepth(path string) int {
	if !strings.HasPrefix(path, ".") {
		return 0
	}

	depth := 0
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == ".." {
			depth++
		}
	}
	return depth
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
