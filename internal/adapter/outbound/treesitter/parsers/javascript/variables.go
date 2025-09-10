package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

func extractJavaScriptVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	fmt.Println("extractJavaScriptVariables called")

	var variables []outbound.SemanticCodeChunk

	// Fallback regex-based variable extraction
	sourceText := string(parseTree.Source())
	lines := strings.Split(sourceText, "\n")

	// Regex patterns for variable declarations
	varPatterns := map[string]*regexp.Regexp{
		"var":   regexp.MustCompile(`^\s*var\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`),
		"let":   regexp.MustCompile(`^\s*let\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`),
		"const": regexp.MustCompile(`^\s*const\s+([a-zA-Z_$][a-zA-Z0-9_$]*)`),
	}

	lineNumber := 0
	for _, line := range lines {
		for declType, pattern := range varPatterns {
			matches := pattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				varName := matches[1]
				startByte := 0
				for i := range lineNumber {
					startByte += len(lines[i]) + 1 // +1 for newline character
				}
				startByte += strings.Index(line, varName)
				endByte := startByte + len(varName)

				chunkType := outbound.ConstructVariable
				if declType == "const" {
					chunkType = outbound.ConstructConstant
				}

				chunk := outbound.SemanticCodeChunk{
					ChunkID:       utils.GenerateID(string(chunkType), varName, nil),
					Type:          chunkType,
					Name:          varName,
					QualifiedName: "main." + varName,
					Language:      parseTree.Language(),
					StartByte:     valueobject.ClampToUint32(startByte),
					EndByte:       valueobject.ClampToUint32(endByte),
					StartPosition: valueobject.Position{
						Row:    valueobject.ClampToUint32(lineNumber),
						Column: valueobject.ClampToUint32(strings.Index(line, varName)),
					},
					EndPosition: valueobject.Position{
						Row:    valueobject.ClampToUint32(lineNumber),
						Column: valueobject.ClampToUint32(strings.Index(line, varName) + len(varName)),
					},
					Content: line,
					Metadata: map[string]interface{}{
						"declaration_type": declType,
						"scope":            getScopeForDeclaration(declType),
					},
					Annotations: []outbound.Annotation{},
					ExtractedAt: time.Now(),
					Hash:        utils.GenerateHash(varName),
				}
				variables = append(variables, chunk)
			}
		}
		lineNumber++
	}

	moduleName := "main"

	// Debug: Print available node types in the first few levels
	fmt.Println("=== JavaScript AST Node Types Debug ===")
	root := parseTree.RootNode()
	debugNodeTypes(root, 0, 3)
	fmt.Println("======================================")

	// Find all variable declarations (var)
	varDeclarations := parseTree.GetNodesByType("variable_declaration")
	for _, node := range varDeclarations {
		declarators := findChildrenByType(node, "variable_declarator")
		for _, declarator := range declarators {
			nameNodes := findChildrenByType(declarator, "identifier")
			if len(nameNodes) == 0 {
				// Debug: Check what child nodes are available
				fmt.Printf("No identifier found in variable_declarator. Available children: ")
				for _, child := range declarator.Children {
					fmt.Printf("%s ", child.Type)
				}
				fmt.Println()
				continue
			}
			nameNode := nameNodes[0]

			varName := parseTree.GetNodeText(nameNode)
			startByte := nameNode.StartByte
			endByte := nameNode.EndByte
			startPos := nameNode.StartPos
			endPos := nameNode.EndPos

			chunk := outbound.SemanticCodeChunk{
				ChunkID:       utils.GenerateID(string(outbound.ConstructVariable), varName, nil),
				Type:          outbound.ConstructVariable,
				Name:          varName,
				QualifiedName: moduleName + "." + varName,
				Language:      parseTree.Language(),
				StartByte:     startByte,
				EndByte:       endByte,
				StartPosition: valueobject.Position{Row: startPos.Row, Column: startPos.Column},
				EndPosition:   valueobject.Position{Row: endPos.Row, Column: endPos.Column},
				Content:       parseTree.GetNodeText(declarator),
				Metadata: map[string]interface{}{
					"declaration_type": "var",
					"scope":            "global",
				},
				Annotations: []outbound.Annotation{},
				ExtractedAt: time.Now(),
				Hash:        utils.GenerateHash(varName),
			}
			variables = append(variables, chunk)
		}
	}

	// Find all lexical declarations (let/const)
	lexicalDeclarations := parseTree.GetNodesByType("lexical_declaration")
	for _, node := range lexicalDeclarations {
		declarators := findChildrenByType(node, "variable_declarator")
		for _, declarator := range declarators {
			nameNodes := findChildrenByType(declarator, "identifier")
			if len(nameNodes) == 0 {
				continue
			}
			nameNode := nameNodes[0]

			varName := parseTree.GetNodeText(nameNode)
			startByte := nameNode.StartByte
			endByte := nameNode.EndByte
			startPos := nameNode.StartPos
			endPos := nameNode.EndPos

			// Determine if it's a let or const declaration
			declarationType := "let"
			if len(node.Children) > 0 && parseTree.GetNodeText(node.Children[0]) == "const" {
				declarationType = "const"
			}

			chunkType := outbound.ConstructVariable
			if declarationType == "const" {
				chunkType = outbound.ConstructConstant
			}

			chunk := outbound.SemanticCodeChunk{
				ChunkID:       utils.GenerateID(string(chunkType), varName, nil),
				Type:          chunkType,
				Name:          varName,
				QualifiedName: moduleName + "." + varName,
				Language:      parseTree.Language(),
				StartByte:     startByte,
				EndByte:       endByte,
				StartPosition: valueobject.Position{Row: startPos.Row, Column: startPos.Column},
				EndPosition:   valueobject.Position{Row: endPos.Row, Column: endPos.Column},
				Content:       parseTree.GetNodeText(declarator),
				Metadata: map[string]interface{}{
					"declaration_type": declarationType,
					"scope":            "block",
				},
				Annotations: []outbound.Annotation{},
				ExtractedAt: time.Now(),
				Hash:        utils.GenerateHash(varName),
			}
			variables = append(variables, chunk)
		}
	}

	return variables, nil
}

func getScopeForDeclaration(declType string) string {
	if declType == "var" {
		return "global"
	}
	return "block"
}

func debugNodeTypes(node *valueobject.ParseNode, currentLevel, maxLevel int) {
	if currentLevel >= maxLevel {
		return
	}

	indent := ""
	for range currentLevel {
		indent += "  "
	}

	fmt.Printf("%s%s (level %d)\n", indent, node.Type, currentLevel)

	for _, child := range node.Children {
		debugNodeTypes(child, currentLevel+1, maxLevel)
	}
}

func extractJavaScriptInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	var interfaces []outbound.SemanticCodeChunk

	moduleName := "main"

	interfaceDeclarations := parseTree.GetNodesByType("interface_declaration")
	for _, node := range interfaceDeclarations {
		nameNodes := findChildrenByType(node, "type_identifier")
		if len(nameNodes) == 0 {
			continue
		}
		nameNode := nameNodes[0]

		interfaceName := parseTree.GetNodeText(nameNode)
		startByte := nameNode.StartByte
		endByte := nameNode.EndByte
		startPos := nameNode.StartPos
		endPos := nameNode.EndPos

		chunk := outbound.SemanticCodeChunk{
			ChunkID:       utils.GenerateID(string(outbound.ConstructInterface), interfaceName, nil),
			Type:          outbound.ConstructInterface,
			Name:          interfaceName,
			QualifiedName: moduleName + "." + interfaceName,
			Language:      parseTree.Language(),
			StartByte:     startByte,
			EndByte:       endByte,
			StartPosition: valueobject.Position{Row: startPos.Row, Column: startPos.Column},
			EndPosition:   valueobject.Position{Row: endPos.Row, Column: endPos.Column},
			Content:       parseTree.GetNodeText(node),
			Metadata: map[string]interface{}{
				"declaration_type": "interface",
				"scope":            "global",
			},
			Annotations: []outbound.Annotation{},
			ExtractedAt: time.Now(),
			Hash:        utils.GenerateHash(interfaceName),
		}
		interfaces = append(interfaces, chunk)
	}

	return interfaces, nil
}
