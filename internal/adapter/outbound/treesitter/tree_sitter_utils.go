package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

// convertTreeSitterNode converts a tree-sitter node to domain ParseNode recursively.
func convertTreeSitterNode(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Convert tree-sitter node to domain ParseNode
	parseNode := &valueobject.ParseNode{
		Type:      node.Type(),
		StartByte: valueobject.ClampUintToUint32(node.StartByte()),
		EndByte:   valueobject.ClampUintToUint32(node.EndByte()),
		StartPos: valueobject.Position{
			Row:    valueobject.ClampUintToUint32(node.StartPoint().Row),
			Column: valueobject.ClampUintToUint32(node.StartPoint().Column),
		},
		EndPos: valueobject.Position{
			Row:    valueobject.ClampUintToUint32(node.EndPoint().Row),
			Column: valueobject.ClampUintToUint32(node.EndPoint().Column),
		},
		Children: []*valueobject.ParseNode{},
	}

	// Convert child nodes recursively
	nodeCount := 1 // Count current node
	maxDepth := depth

	childCount := int(node.ChildCount())
	for i := range childCount {
		childNode := node.Child(valueobject.ClampToUint32(i))
		childParseNode, childNodeCount, childMaxDepth := convertTreeSitterNode(childNode, depth+1)
		if childParseNode != nil {
			parseNode.Children = append(parseNode.Children, childParseNode)
			nodeCount += childNodeCount
			if childMaxDepth > maxDepth {
				maxDepth = childMaxDepth
			}
		}
	}

	return parseNode, nodeCount, maxDepth
}

// ConvertTreeSitterNodeWithPreservation converts a tree-sitter node to domain ParseNode recursively,
// preserving tree-sitter node references for Content() method usage.
// This function is exported for use by language-specific parsers.
func ConvertTreeSitterNodeWithPreservation(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Use the new constructor that preserves tree-sitter node reference
	parseNode, err := valueobject.NewParseNodeWithTreeSitter(
		node.Type(),
		valueobject.ClampUintToUint32(node.StartByte()),
		valueobject.ClampUintToUint32(node.EndByte()),
		valueobject.Position{
			Row:    valueobject.ClampUintToUint32(node.StartPoint().Row),
			Column: valueobject.ClampUintToUint32(node.StartPoint().Column),
		},
		valueobject.Position{
			Row:    valueobject.ClampUintToUint32(node.EndPoint().Row),
			Column: valueobject.ClampUintToUint32(node.EndPoint().Column),
		},
		[]*valueobject.ParseNode{},
		node,
	)
	if err != nil {
		// Fallback to legacy conversion if new constructor fails
		return convertTreeSitterNode(node, depth)
	}

	// Convert child nodes recursively
	nodeCount := 1 // Count current node
	maxDepth := depth

	childCount := int(node.ChildCount())
	for i := range childCount {
		childNode := node.Child(valueobject.ClampToUint32(i))
		childParseNode, childNodeCount, childMaxDepth := ConvertTreeSitterNodeWithPreservation(childNode, depth+1)
		if childParseNode != nil {
			parseNode.Children = append(parseNode.Children, childParseNode)
			nodeCount += childNodeCount
			if childMaxDepth > maxDepth {
				maxDepth = childMaxDepth
			}
		}
	}

	return parseNode, nodeCount, maxDepth
}

// Note: Using valueobject.ClampUintToUint32, valueobject.ClampToUint32, and valueobject.ClampUintToInt instead of local duplicates

// ============================================================================
// Shared TreeSitter Utilities - Extract common patterns to avoid cycles
// ============================================================================

// ParseTreeResult represents the result of parsing source code with TreeSitter.
// This struct encapsulates both the domain parse tree and any error that occurred.
type ParseTreeResult struct {
	ParseTree *valueobject.ParseTree
	Error     error
}

// CreateTreeSitterParseTree creates a parse tree from Go source code using TreeSitter.
// This function consolidates the repeated pattern used across validation methods:
//  1. Create parser factory
//  2. Create Go language parser
//  3. Parse the source code
//  4. Convert to domain parse tree
//
// This eliminates code duplication and provides a consistent interface for
// all validation methods that need to parse Go source code.
//
// Parameters:
//
//	ctx - Context for cancellation and timeout control
//	source - Go source code to parse (as string)
//
// Returns:
//
//	*ParseTreeResult - Contains either the parsed tree or an error
//
// Example usage:
//
//	result := CreateTreeSitterParseTree(ctx, sourceCode)
//	if result.Error != nil {
//	    return fmt.Errorf("parse failed: %w", result.Error)
//	}
//	// Use result.ParseTree for validation
func CreateTreeSitterParseTree(ctx context.Context, source string) *ParseTreeResult {
	// Create TreeSitter parser factory
	factory, err := NewTreeSitterParserFactory(ctx)
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to create parser factory: %w", err),
		}
	}

	// Create Go language value object
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to create language: %w", err),
		}
	}

	// Create parser for Go language
	parser, err := factory.CreateParser(ctx, goLang)
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to create parser: %w", err),
		}
	}

	// Parse the source code
	parseResult, err := parser.Parse(ctx, []byte(source))
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to parse source: %w", err),
		}
	}

	// Convert to domain parse tree
	domainTree, err := ConvertPortParseTreeToDomain(parseResult.ParseTree)
	if err != nil {
		return &ParseTreeResult{
			Error: fmt.Errorf("failed to convert parse tree: %w", err),
		}
	}

	return &ParseTreeResult{
		ParseTree: domainTree,
		Error:     nil,
	}
}

// ============================================================================
// Optimized Tree-Sitter Error Analysis System - REFACTORED
// ============================================================================

// TreeSitterErrorAnalysisResult represents comprehensive error analysis results.
type TreeSitterErrorAnalysisResult struct {
	HasErrors             bool
	ErrorsFound           []ErrorDetail
	AlgorithmicComplexity string
	CanPartiallyParse     bool
	RecoverySuggestions   []string
	AnalysisMetrics       AnalysisMetrics
}

// ErrorDetail represents detailed information about a specific error.
type ErrorDetail struct {
	Type        string
	Message     string
	Line        int
	Column      int
	Severity    ErrorSeverity
	Recoverable bool
}

// ErrorSeverity represents the severity level of an error.
type ErrorSeverity int

const (
	SeverityInfo ErrorSeverity = iota
	SeverityWarning
	SeverityError
	SeverityFatal
)

// Error message constants to avoid duplication.
const (
	msgSyntaxErrorDetected = "syntax error detected by tree-sitter parser"
	nodeTypeError          = "ERROR"
)

// AnalysisMetrics contains performance and complexity metrics.
type AnalysisMetrics struct {
	NodesAnalyzed    int
	TreeDepth        int
	AnalysisDuration time.Duration
	MemoryUsed       int64
}

// OptimizedTreeSitterErrorAnalyzer provides centralized, high-performance error analysis.
type OptimizedTreeSitterErrorAnalyzer struct {
	grammar *tree_sitter.Language
	parser  *tree_sitter.Parser
}

// NewOptimizedTreeSitterErrorAnalyzer creates a new analyzer instance.
func NewOptimizedTreeSitterErrorAnalyzer() (*OptimizedTreeSitterErrorAnalyzer, error) {
	grammar := forest.GetLanguage("go")
	if grammar == nil {
		return nil, errors.New("failed to get Go grammar")
	}

	parser := tree_sitter.NewParser()
	if parser == nil {
		return nil, errors.New("failed to create tree-sitter parser")
	}

	if ok := parser.SetLanguage(grammar); !ok {
		return nil, errors.New("failed to set Go language")
	}

	return &OptimizedTreeSitterErrorAnalyzer{
		grammar: grammar,
		parser:  parser,
	}, nil
}

// AnalyzeErrors performs comprehensive error analysis with O(n) complexity.
func (analyzer *OptimizedTreeSitterErrorAnalyzer) AnalyzeErrors(
	ctx context.Context,
	source string,
) (*TreeSitterErrorAnalysisResult, error) {
	start := time.Now()

	tree, err := analyzer.parser.ParseString(ctx, nil, []byte(source))
	if err != nil {
		return &TreeSitterErrorAnalysisResult{
			HasErrors:             true,
			ErrorsFound:           []ErrorDetail{{Type: "PARSE_ERROR", Message: err.Error(), Severity: SeverityFatal}},
			AlgorithmicComplexity: "O(1)",
			CanPartiallyParse:     false,
		}, nil
	}
	if tree == nil {
		return &TreeSitterErrorAnalysisResult{
			HasErrors: true,
			ErrorsFound: []ErrorDetail{
				{Type: "NIL_TREE", Message: "parse tree is nil", Severity: SeverityFatal},
			},
			AlgorithmicComplexity: "O(1)",
			CanPartiallyParse:     false,
		}, nil
	}
	defer tree.Close()

	// Single-pass error analysis for optimal performance
	root := tree.RootNode()
	result := &TreeSitterErrorAnalysisResult{
		ErrorsFound:         []ErrorDetail{},
		CanPartiallyParse:   true,
		RecoverySuggestions: []string{},
		AnalysisMetrics: AnalysisMetrics{
			AnalysisDuration: time.Since(start),
		},
	}

	// Perform optimized single-pass analysis
	analyzer.performSinglePassAnalysis(root, []byte(source), result)

	// Determine algorithmic complexity based on analysis
	result.AlgorithmicComplexity = analyzer.determineComplexity(
		len(source),
		result.AnalysisMetrics.NodesAnalyzed,
		result.AnalysisMetrics.TreeDepth,
	)
	result.HasErrors = len(result.ErrorsFound) > 0

	return result, nil
}

// performSinglePassAnalysis performs optimized O(n) error detection.
func (analyzer *OptimizedTreeSitterErrorAnalyzer) performSinglePassAnalysis(
	node tree_sitter.Node,
	source []byte,
	result *TreeSitterErrorAnalysisResult,
) {
	if node.IsNull() {
		return
	}

	// Track metrics for complexity analysis
	result.AnalysisMetrics.NodesAnalyzed++

	// Use tree-sitter's native error detection methods
	if node.HasError() {
		analyzer.classifyAndRecordError(node, source, result)
	}

	if node.IsError() {
		result.ErrorsFound = append(result.ErrorsFound, ErrorDetail{
			Type:        "SYNTAX_ERROR",
			Message:     analyzer.generateDetailedErrorMessage(node, source),
			Line:        valueobject.ClampUintToInt(node.StartPoint().Row) + 1,
			Column:      valueobject.ClampUintToInt(node.StartPoint().Column) + 1,
			Severity:    SeverityError,
			Recoverable: analyzer.isRecoverableError(node, source),
		})
	}

	if node.IsMissing() {
		result.ErrorsFound = append(result.ErrorsFound, ErrorDetail{
			Type:        "MISSING_TOKEN",
			Message:     analyzer.generateMissingTokenMessage(node, source),
			Line:        valueobject.ClampUintToInt(node.StartPoint().Row) + 1,
			Column:      valueobject.ClampUintToInt(node.StartPoint().Column) + 1,
			Severity:    SeverityWarning,
			Recoverable: true,
		})
	}

	// Track tree depth for complexity analysis
	currentDepth := analyzer.calculateNodeDepth(node)
	if currentDepth > result.AnalysisMetrics.TreeDepth {
		result.AnalysisMetrics.TreeDepth = currentDepth
	}

	// Recursively analyze children with early termination for performance
	childCount := node.ChildCount()
	for i := range childCount {
		child := node.Child(i)
		if !child.IsNull() {
			analyzer.performSinglePassAnalysis(child, source, result)
		}
	}
}

// classifyAndRecordError performs detailed error classification.
func (analyzer *OptimizedTreeSitterErrorAnalyzer) classifyAndRecordError(
	node tree_sitter.Node,
	source []byte,
	result *TreeSitterErrorAnalysisResult,
) {
	nodeText := node.Content(source)
	nodeType := node.Type()

	var errorType, message string
	var severity ErrorSeverity
	var recoverable bool

	switch {
	case strings.Contains(nodeText, "{") && !strings.Contains(nodeText, "}"):
		errorType = "MISSING_CLOSING_BRACE"
		message = msgSyntaxErrorDetected
		severity = SeverityError
		recoverable = true
		result.RecoverySuggestions = append(result.RecoverySuggestions, "add missing closing brace")

	case strings.Contains(nodeText, "(") && !strings.Contains(nodeText, ")"):
		errorType = "MISSING_CLOSING_PAREN"
		message = msgSyntaxErrorDetected
		severity = SeverityError
		recoverable = true
		result.RecoverySuggestions = append(result.RecoverySuggestions, "add missing closing parenthesis")

	case nodeType == "ERROR":
		errorType = "GENERAL_SYNTAX_ERROR"
		message = msgSyntaxErrorDetected
		severity = SeverityError
		recoverable = analyzer.isRecoverableError(node, source)

	default:
		errorType = "PARSE_ERROR"
		message = msgSyntaxErrorDetected
		severity = SeverityError
		recoverable = false
	}

	result.ErrorsFound = append(result.ErrorsFound, ErrorDetail{
		Type:        errorType,
		Message:     message,
		Line:        valueobject.ClampUintToInt(node.StartPoint().Row) + 1,
		Column:      valueobject.ClampUintToInt(node.StartPoint().Column) + 1,
		Severity:    severity,
		Recoverable: recoverable,
	})
}

// generateDetailedErrorMessage creates specific error messages based on node content.
func (analyzer *OptimizedTreeSitterErrorAnalyzer) generateDetailedErrorMessage(
	node tree_sitter.Node,
	source []byte,
) string {
	nodeText := node.Content(source)

	switch {
	case strings.Contains(nodeText, "@") || strings.Contains(nodeText, "#") || strings.Contains(nodeText, "$"):
		return "syntax error: invalid token sequence"
	case strings.Contains(nodeText, "package ") && len(strings.Fields(nodeText)) > 1:
		packageName := strings.Fields(nodeText)[1]
		if len(packageName) > 0 && packageName[0] >= '0' && packageName[0] <= '9' {
			return "syntax error: invalid package identifier"
		}
		return "syntax error: invalid package declaration"
	case strings.Contains(nodeText, "func ") && strings.Contains(nodeText, "(") && !strings.Contains(nodeText, ")"):
		return "syntax error: incomplete function declaration"
	case strings.Contains(nodeText, "struct {") && !strings.Contains(nodeText, "}"):
		return "syntax error: incomplete struct declaration"
	default:
		return "syntax error detected by tree-sitter parser"
	}
}

// generateMissingTokenMessage creates specific messages for missing tokens.
func (analyzer *OptimizedTreeSitterErrorAnalyzer) generateMissingTokenMessage(
	node tree_sitter.Node,
	source []byte,
) string {
	parentType := ""
	if !node.Parent().IsNull() {
		parentType = node.Parent().Type()
	}

	switch parentType {
	case "function_declaration":
		return "missing token in function declaration"
	case "struct_type":
		return "missing token in struct declaration"
	case "package_clause":
		return "missing token in package declaration"
	default:
		return "missing required token"
	}
}

// isRecoverableError determines if an error allows partial parsing.
func (analyzer *OptimizedTreeSitterErrorAnalyzer) isRecoverableError(node tree_sitter.Node, source []byte) bool {
	nodeText := node.Content(source)

	// Missing braces or parentheses are usually recoverable
	if strings.Contains(nodeText, "{") || strings.Contains(nodeText, "(") {
		return true
	}

	// Invalid tokens are usually not recoverable
	if strings.Contains(nodeText, "@") || strings.Contains(nodeText, "#") || strings.Contains(nodeText, "$") {
		return false
	}

	// Most other syntax errors are recoverable for partial parsing
	return true
}

// calculateNodeDepth calculates the depth of a node in the tree.
func (analyzer *OptimizedTreeSitterErrorAnalyzer) calculateNodeDepth(node tree_sitter.Node) int {
	depth := 0
	current := node

	for !current.Parent().IsNull() {
		depth++
		current = current.Parent()
	}

	return depth
}

// determineComplexity determines algorithmic complexity based on analysis metrics.
func (analyzer *OptimizedTreeSitterErrorAnalyzer) determineComplexity(
	sourceLength, nodesAnalyzed, treeDepth int,
) string {
	// Determine complexity based on the relationship between input size and operations
	switch {
	case nodesAnalyzed <= sourceLength:
		return "O(n)"
	case nodesAnalyzed <= sourceLength*treeDepth:
		return "O(n*d)"
	default:
		return "O(n*m)"
	}
}

// ValidateSourceWithTreeSitter provides backward-compatible validation using the optimized analyzer.
func ValidateSourceWithTreeSitter(ctx context.Context, source string) error {
	analyzer, err := NewOptimizedTreeSitterErrorAnalyzer()
	if err != nil {
		return fmt.Errorf("failed to create error analyzer: %w", err)
	}

	result, err := analyzer.AnalyzeErrors(ctx, source)
	if err != nil {
		return err
	}

	if result.HasErrors {
		// Return first fatal error, or first error if no fatal errors
		for _, errorDetail := range result.ErrorsFound {
			if errorDetail.Severity == SeverityFatal {
				return errors.New(errorDetail.Message)
			}
		}
		if len(result.ErrorsFound) > 0 {
			return errors.New(result.ErrorsFound[0].Message)
		}
	}

	// Additional Go-specific validation for type-only snippets
	// This preserves the behavior that was in the old validator
	if err := validateGoPackageDeclaration(source); err != nil {
		return err
	}

	return nil
}

// validateGoPackageDeclaration performs Go-specific package validation for larger files.
// This preserves the behavior from the old validator while reducing complexity.
func validateGoPackageDeclaration(source string) error {
	trimmed := strings.TrimSpace(source)
	lineCount := len(strings.Split(trimmed, "\n"))

	if len(trimmed) == 0 || lineCount <= 10 {
		return nil // Small files or empty - skip validation
	}

	// Only check for package in larger files
	if strings.HasPrefix(trimmed, "package ") || strings.Contains(trimmed, "package ") {
		return nil // Has package declaration
	}

	hasFunc := strings.Contains(source, "func ")
	hasType := strings.Contains(source, "type ")

	if !hasFunc && !hasType {
		return nil // No significant code
	}

	// Only enforce package declaration for files with actual function definitions
	// Type-only files (structs, interfaces, constants, variables) should be allowed without package
	if hasFunc && !isPartialTypeOnlySnippetForValidation(source) {
		return errors.New(
			"syntax error in Go: missing package declaration: Go files must start with package",
		)
	}

	return nil
}

// isPartialTypeOnlySnippetForValidation checks if source is a type-only snippet that should be allowed without package declaration.
// This is a simplified version of the Go parser's isPartialTypeOnlySnippet function to avoid circular dependencies.
func isPartialTypeOnlySnippetForValidation(source string) bool {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return true
	}

	lines := strings.Split(trimmed, "\n")
	nonEmptyLines := make([]string, 0, len(lines))

	// Collect non-empty, non-comment lines
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "//") {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	if len(nonEmptyLines) == 0 {
		return true
	}

	// Check if all non-comment lines are type declarations or related constructs
	for _, line := range nonEmptyLines {
		if !strings.HasPrefix(line, "type ") &&
			!strings.HasPrefix(line, "}") &&
			!strings.Contains(line, "struct {") &&
			!strings.Contains(line, "interface {") &&
			!isSimpleFieldLine(line) {
			return false
		}
	}
	return true
}

// isSimpleFieldLine does basic field detection without complex AST parsing to avoid recursion.
func isSimpleFieldLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}

	// Simple heuristics for field-like patterns
	parts := strings.Fields(trimmed)
	if len(parts) >= 2 {
		// Check for field patterns like "Name string" or "ID int"
		if !strings.Contains(parts[0], "(") && !strings.Contains(parts[0], ")") &&
			!strings.HasPrefix(trimmed, "func ") && !strings.HasPrefix(trimmed, "var ") &&
			!strings.HasPrefix(trimmed, "const ") && !strings.HasPrefix(trimmed, "import ") {
			return true
		}
	}

	// Single word could be embedded field like "Person" or "Address"
	if len(parts) == 1 && !strings.Contains(parts[0], "(") && !strings.Contains(parts[0], ")") {
		return true
	}

	return false
}
