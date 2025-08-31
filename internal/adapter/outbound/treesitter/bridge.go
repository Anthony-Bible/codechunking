package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// safeIntToUint32 safely converts int to uint32, clamping negative values to 0
// and values exceeding uint32 max to math.MaxUint32.
func safeIntToUint32(val int) uint32 {
	if val < 0 {
		return 0
	}
	if val > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(val)
}

// safeInt64ToUint64 safely converts int64 to uint64, clamping negative values to 0.
func safeInt64ToUint64(val int64) uint64 {
	if val < 0 {
		return 0
	}
	return uint64(val)
}

// ConvertDomainParseTreeToPort converts domain ParseTree to port ParseTree.
func ConvertDomainParseTreeToPort(domainTree *valueobject.ParseTree) (*ParseTree, error) {
	if domainTree == nil {
		return nil, errors.New("domain parse tree cannot be nil")
	}

	portRootNode, err := convertDomainNodeToPort(domainTree.RootNode())
	if err != nil {
		return nil, fmt.Errorf("failed to convert root node: %w", err)
	}

	portTree := &ParseTree{
		Language:       domainTree.Language().String(),
		RootNode:       portRootNode,
		Source:         string(domainTree.Source()),
		CreatedAt:      domainTree.CreatedAt(),
		TreeSitterTree: nil, // Domain tree doesn't expose raw tree-sitter tree
	}

	return portTree, nil
}

// ConvertDomainParseResultToPort converts domain ParseResult to port ParseResult.
func ConvertDomainParseResultToPort(domainResult *valueobject.ParseResult) (*ParseResult, error) {
	if domainResult == nil {
		return nil, errors.New("domain parse result cannot be nil")
	}

	var portTree *ParseTree
	var err error

	if domainResult.ParseTree() != nil {
		portTree, err = ConvertDomainParseTreeToPort(domainResult.ParseTree())
		if err != nil {
			return nil, fmt.Errorf("failed to convert parse tree: %w", err)
		}
	}

	// Convert statistics
	domainStats := domainResult.GetStatistics()
	portStats := &ParsingStatistics{
		TotalNodes:     safeIntToUint32(domainStats.TotalNodes),
		ErrorNodes:     safeIntToUint32(domainStats.ErrorNodes),
		MissingNodes:   0, // Not available in domain stats
		MaxDepth:       safeIntToUint32(domainStats.MaxDepth),
		ParseDuration:  domainStats.ParseDuration,
		MemoryUsed:     safeInt64ToUint64(domainStats.MemoryUsed),
		BytesProcessed: safeInt64ToUint64(domainStats.BytesProcessed),
		LinesProcessed: safeIntToUint32(domainStats.LinesProcessed),
		TreeSizeBytes:  0, // Not available in domain stats
	}

	// Convert errors (simplified - domain errors are different structure)
	var portErrors []ParseError
	if domainResult.HasErrors() {
		portErrors = append(portErrors, ParseError{
			Type:        "ParseError",
			Message:     "Parse errors occurred",
			Severity:    "error",
			Recoverable: true,
			Timestamp:   time.Now(),
		})
	}

	// Convert warnings (simplified)
	var portWarnings []ParseWarning
	if domainResult.HasWarnings() {
		portWarnings = append(portWarnings, ParseWarning{
			Type:      "ParseWarning",
			Message:   "Parse warnings occurred",
			Timestamp: time.Now(),
		})
	}

	portResult := &ParseResult{
		Success:    domainResult.IsSuccessful(),
		ParseTree:  portTree,
		Errors:     portErrors,
		Warnings:   portWarnings,
		Duration:   domainResult.Duration(),
		Statistics: portStats,
		Metadata:   make(map[string]interface{}),
	}

	return portResult, nil
}

// ConvertPortParseTreeToDomain converts port ParseTree to domain ParseTree.
func ConvertPortParseTreeToDomain(portTree *ParseTree) (*valueobject.ParseTree, error) {
	if portTree == nil {
		return nil, errors.New("port parse tree cannot be nil")
	}

	domainRootNode, err := convertPortNodeToDomain(portTree.RootNode)
	if err != nil {
		return nil, fmt.Errorf("failed to convert root node: %w", err)
	}

	metadata, err := valueobject.NewParseMetadata(
		DefaultBridgeParseDuration,
		"go-tree-sitter-bare",
		"1.0.0",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata: %w", err)
	}

	// Use background context when context is not available from caller
	ctx := context.Background()
	// Convert language string back to Language value object
	language, err := valueobject.NewLanguage(portTree.Language)
	if err != nil {
		return nil, fmt.Errorf("invalid language %s: %w", portTree.Language, err)
	}

	domainTree, err := valueobject.NewParseTree(
		ctx,
		language,
		domainRootNode,
		[]byte(portTree.Source),
		metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create domain parse tree: %w", err)
	}

	return domainTree, nil
}

// convertDomainNodeToPort converts domain ParseNode to port ParseNode.
func convertDomainNodeToPort(domainNode *valueobject.ParseNode) (*ParseNode, error) {
	if domainNode == nil {
		return nil, errors.New("domain node cannot be nil")
	}

	// Convert children recursively
	var portChildren []*ParseNode
	for _, child := range domainNode.Children {
		portChild, err := convertDomainNodeToPort(child)
		if err != nil {
			return nil, fmt.Errorf("failed to convert child node: %w", err)
		}
		portChildren = append(portChildren, portChild)
	}

	portNode := &ParseNode{
		Type:      domainNode.Type,
		StartByte: domainNode.StartByte,
		EndByte:   domainNode.EndByte,
		StartPoint: Point{
			Row:    domainNode.StartPos.Row,
			Column: domainNode.StartPos.Column,
		},
		EndPoint: Point{
			Row:    domainNode.EndPos.Row,
			Column: domainNode.EndPos.Column,
		},
		Children:   portChildren,
		FieldName:  "", // Not available in domain node
		IsNamed:    true,
		IsError:    domainNode.Type == "ERROR",
		IsMissing:  false,
		HasChanges: false,
		HasError:   domainNode.Type == "ERROR",
		Text:       "", // Would need source text to populate
	}

	return portNode, nil
}

// convertPortNodeToDomain converts port ParseNode to domain ParseNode.
func convertPortNodeToDomain(portNode *ParseNode) (*valueobject.ParseNode, error) {
	if portNode == nil {
		return nil, errors.New("port node cannot be nil")
	}

	// Convert children recursively
	var domainChildren []*valueobject.ParseNode
	for _, child := range portNode.Children {
		domainChild, err := convertPortNodeToDomain(child)
		if err != nil {
			return nil, fmt.Errorf("failed to convert child node: %w", err)
		}
		domainChildren = append(domainChildren, domainChild)
	}

	domainNode := &valueobject.ParseNode{
		Type:      portNode.Type,
		StartByte: portNode.StartByte,
		EndByte:   portNode.EndByte,
		StartPos: valueobject.Position{
			Row:    portNode.StartPoint.Row,
			Column: portNode.StartPoint.Column,
		},
		EndPos: valueobject.Position{
			Row:    portNode.EndPoint.Row,
			Column: portNode.EndPoint.Column,
		},
		Children: domainChildren,
	}

	return domainNode, nil
}

// DetectLanguageFromFilePath uses forest to detect language from file path.
func DetectLanguageFromFilePath(filePath string) (valueobject.Language, error) {
	if filePath == "" {
		return valueobject.Language{}, errors.New("empty file path")
	}

	// Extract extension for simple detection
	ext := getFileExtension(filePath)
	if ext == "" {
		return valueobject.Language{}, errors.New("no file extension")
	}

	// Map extension to language
	langName := mapExtensionToLanguage(ext)
	if langName == "" {
		return valueobject.Language{}, fmt.Errorf("unsupported file extension: %s", ext)
	}

	return valueobject.NewLanguage(langName)
}

// getFileExtension extracts the file extension from a path.
func getFileExtension(filePath string) string {
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '.' {
			return filePath[i+1:]
		}
		if filePath[i] == '/' || filePath[i] == '\\' {
			break
		}
	}
	return ""
}

// mapExtensionToLanguage maps file extensions to language names.
func mapExtensionToLanguage(ext string) string {
	mapping := map[string]string{
		"go":    valueobject.LanguageGo,
		"py":    valueobject.LanguagePython,
		"js":    valueobject.LanguageJavaScript,
		"jsx":   valueobject.LanguageJavaScript,
		"ts":    valueobject.LanguageTypeScript,
		"tsx":   valueobject.LanguageTypeScript,
		"cpp":   valueobject.LanguageCPlusPlus,
		"cxx":   valueobject.LanguageCPlusPlus,
		"cc":    valueobject.LanguageCPlusPlus,
		"c":     "c",
		"h":     "c",
		"rs":    valueobject.LanguageRust,
		"java":  "java",
		"rb":    "ruby",
		"php":   "php",
		"kt":    "kotlin",
		"swift": "swift",
		"scala": "scala",
		"html":  "html",
		"css":   "css",
		"json":  "json",
		"yaml":  "yaml",
		"yml":   "yaml",
		"xml":   "xml",
		"sql":   "sql",
		"sh":    "bash",
		"bash":  "bash",
	}

	return mapping[ext]
}
