package valueobject

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ParseTree represents a tree-sitter parse tree as a value object.
type ParseTree struct {
	language       Language
	rootNode       *ParseNode
	source         []byte
	metadata       ParseMetadata
	createdAt      time.Time
	isCleanedUp    bool
	mu             sync.RWMutex
	treeSitterTree *tree_sitter.Tree // Reference to actual tree-sitter tree for proper cleanup
	metrics        *parseTreeMetrics // OTEL metrics
}

// ParseNode represents a node in the parse tree.
type ParseNode struct {
	Type      string
	StartByte uint32
	EndByte   uint32
	StartPos  Position
	EndPos    Position
	Children  []*ParseNode
	tsNode    *tree_sitter.Node // Reference to actual tree-sitter node
}

// Position represents a position in source code.
type Position struct {
	Row    uint32
	Column uint32
}

// ParseMetadata contains metadata about the parse operation.
type ParseMetadata struct {
	ParseDuration     time.Duration
	TreeSitterVersion string
	GrammarVersion    string
	NodeCount         int
	MaxDepth          int
	MemoryUsage       int64
	ErrorCount        int
}

// parseTreeMetrics holds OTEL metrics for ParseTree operations.
type parseTreeMetrics struct {
	parseOperationsCounter   metric.Int64Counter
	parseTimeHistogram       metric.Float64Histogram
	treeNodeCountHistogram   metric.Int64Histogram
	treeDepthHistogram       metric.Int64Histogram
	cleanupOperationsCounter metric.Int64Counter
	memoryUsageGauge         metric.Int64Gauge
}

// NewParseTree creates a new ParseTree value object with validation and production features.
func NewParseTree(
	ctx context.Context,
	language Language,
	rootNode *ParseNode,
	source []byte,
	metadata ParseMetadata,
) (*ParseTree, error) {
	if rootNode == nil {
		slogger.Error(ctx, "Failed to create ParseTree: root node is nil", slogger.Fields{
			"language":      language.Name(),
			"source_length": len(source),
		})
		return nil, errors.New("root node cannot be nil")
	}

	if len(source) == 0 {
		slogger.Error(ctx, "Failed to create ParseTree: empty source code", slogger.Fields{
			"language": language.Name(),
		})
		return nil, errors.New("source code cannot be empty")
	}

	if int64(rootNode.EndByte) > int64(len(source)) {
		slogger.Error(ctx, "Failed to create ParseTree: root node end byte exceeds source length", slogger.Fields{
			"language":      language.Name(),
			"source_length": len(source),
			"root_end_byte": rootNode.EndByte,
		})
		return nil, errors.New("root node end byte exceeds source length")
	}

	// Initialize OTEL metrics
	metrics, err := initParseTreeMetrics()
	if err != nil {
		slogger.Warn(ctx, "Failed to initialize parse tree metrics, continuing without metrics", slogger.Fields{
			"error":    err.Error(),
			"language": language.Name(),
		})
	}

	pt := &ParseTree{
		language:    language,
		rootNode:    rootNode,
		source:      source,
		metadata:    metadata,
		createdAt:   time.Now(),
		isCleanedUp: false,
		metrics:     metrics,
	}

	// Record metrics
	if metrics != nil {
		metrics.recordParseOperation(
			ctx,
			language.Name(),
			metadata.ParseDuration,
			metadata.NodeCount,
			metadata.MaxDepth,
			int64(len(source)),
		)
	}

	slogger.Info(ctx, "ParseTree created successfully", slogger.Fields{
		"language":       language.Name(),
		"node_count":     metadata.NodeCount,
		"max_depth":      metadata.MaxDepth,
		"source_length":  len(source),
		"parse_duration": metadata.ParseDuration.String(),
	})

	return pt, nil
}

// SetTreeSitterTree sets the tree-sitter tree reference for native error detection.
func (pt *ParseTree) SetTreeSitterTree(tree *tree_sitter.Tree) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.treeSitterTree = tree
}

// NewParseMetadata creates a new ParseMetadata value object.
func NewParseMetadata(duration time.Duration, treeSitterVersion, grammarVersion string) (ParseMetadata, error) {
	if duration < 0 {
		return ParseMetadata{}, errors.New("parse duration cannot be negative")
	}

	return ParseMetadata{
		ParseDuration:     duration,
		TreeSitterVersion: treeSitterVersion,
		GrammarVersion:    grammarVersion,
		NodeCount:         0,
		MaxDepth:          0,
	}, nil
}

// Language returns the language of the parse tree.
func (pt *ParseTree) Language() Language {
	return pt.language
}

// RootNode returns the root node of the parse tree.
func (pt *ParseTree) RootNode() *ParseNode {
	return pt.rootNode
}

// Source returns the source code of the parse tree.
func (pt *ParseTree) Source() []byte {
	return pt.source
}

// Metadata returns the metadata of the parse tree.
func (pt *ParseTree) Metadata() ParseMetadata {
	return pt.metadata
}

// CreatedAt returns when the parse tree was created.
func (pt *ParseTree) CreatedAt() time.Time {
	return pt.createdAt
}

// IsCleanedUp returns whether the parse tree has been cleaned up.
func (pt *ParseTree) IsCleanedUp() bool {
	return pt.isCleanedUp
}

// GetNodesByType returns all nodes of a specific type.
func (pt *ParseTree) GetNodesByType(nodeType string) []*ParseNode {
	if pt.isCleanedUp {
		return []*ParseNode{}
	}

	var result []*ParseNode
	pt.collectNodesByType(pt.rootNode, nodeType, &result)
	return result
}

// collectNodesByType recursively collects nodes of a specific type.
func (pt *ParseTree) collectNodesByType(node *ParseNode, nodeType string, result *[]*ParseNode) {
	if node == nil {
		return
	}

	if node.Type == nodeType {
		*result = append(*result, node)
	}

	for _, child := range node.Children {
		pt.collectNodesByType(child, nodeType, result)
	}
}

// GetNodeAtPosition returns the node at a specific position.
func (pt *ParseTree) GetNodeAtPosition(pos Position) *ParseNode {
	if pt.isCleanedUp {
		return nil
	}
	return pt.findNodeAtPosition(pt.rootNode, pos)
}

// findNodeAtPosition recursively finds the node at a position.
func (pt *ParseTree) findNodeAtPosition(node *ParseNode, pos Position) *ParseNode {
	if node == nil {
		return nil
	}

	// Check if position is within this node
	if pos.Row >= node.StartPos.Row && pos.Row <= node.EndPos.Row {
		if pos.Row == node.StartPos.Row && pos.Column < node.StartPos.Column {
			return nil
		}
		if pos.Row == node.EndPos.Row && pos.Column > node.EndPos.Column {
			return nil
		}

		// Check children first (more specific)
		for _, child := range node.Children {
			if childResult := pt.findNodeAtPosition(child, pos); childResult != nil {
				return childResult
			}
		}

		// Return this node if no child contains the position
		return node
	}

	return nil
}

// GetNodeAtByteOffset returns the node at a specific byte offset.
func (pt *ParseTree) GetNodeAtByteOffset(offset uint32) *ParseNode {
	if pt.isCleanedUp {
		return nil
	}
	return pt.findNodeAtByteOffset(pt.rootNode, offset)
}

// findNodeAtByteOffset recursively finds the node at a byte offset.
func (pt *ParseTree) findNodeAtByteOffset(node *ParseNode, offset uint32) *ParseNode {
	if node == nil {
		return nil
	}

	// Check if offset is within this node
	if offset >= node.StartByte && offset <= node.EndByte {
		// Check children first (more specific)
		for _, child := range node.Children {
			if childResult := pt.findNodeAtByteOffset(child, offset); childResult != nil {
				return childResult
			}
		}

		// Return this node if no child contains the offset
		return node
	}

	return nil
}

// SanitizeContent removes null bytes (0x00) from content to ensure PostgreSQL UTF-8 compatibility.
// PostgreSQL's TEXT columns with UTF-8 encoding cannot store null bytes, which may be present in
// binary files or certain source code files. This function removes all null bytes while preserving
// all other content, including valid UTF-8 sequences.
func SanitizeContent(content string) string {
	// Fast path: if no null bytes, return unchanged
	if !strings.Contains(content, "\x00") {
		return content
	}
	// Remove all null bytes
	return strings.ReplaceAll(content, "\x00", "")
}

// GetNodeText returns the text content of a node with null byte sanitization.
// Content is sanitized to ensure PostgreSQL UTF-8 compatibility by removing null bytes.
func (pt *ParseTree) GetNodeText(node *ParseNode) string {
	if pt.isCleanedUp || node == nil {
		return ""
	}

	// Use Content() method if tree-sitter node is available
	if tsNode := node.TreeSitterNode(); tsNode != nil && !tsNode.IsNull() {
		return SanitizeContent(tsNode.Content(pt.source))
	}

	// Fallback to byte slicing
	if int64(node.EndByte) > int64(len(pt.source)) {
		return ""
	}

	return SanitizeContent(string(pt.source[node.StartByte:node.EndByte]))
}

// GetTreeDepth returns the maximum depth of the parse tree.
func (pt *ParseTree) GetTreeDepth() int {
	if pt.isCleanedUp {
		return 0
	}
	return pt.calculateDepth(pt.rootNode, 1)
}

// calculateDepth recursively calculates the depth of the tree.
func (pt *ParseTree) calculateDepth(node *ParseNode, currentDepth int) int {
	if node == nil {
		return currentDepth - 1
	}

	maxDepth := currentDepth
	for _, child := range node.Children {
		childDepth := pt.calculateDepth(child, currentDepth+1)
		if childDepth > maxDepth {
			maxDepth = childDepth
		}
	}

	return maxDepth
}

// GetTotalNodeCount returns the total number of nodes in the tree.
func (pt *ParseTree) GetTotalNodeCount() int {
	if pt.isCleanedUp {
		return 0
	}
	return pt.countNodes(pt.rootNode)
}

// countNodes recursively counts nodes in the tree.
func (pt *ParseTree) countNodes(node *ParseNode) int {
	if node == nil {
		return 0
	}

	count := 1
	for _, child := range node.Children {
		count += pt.countNodes(child)
	}

	return count
}

// IsWellFormed checks if the parse tree is well-formed.
func (pt *ParseTree) IsWellFormed() (bool, error) {
	if pt.isCleanedUp {
		return false, errors.New("parse tree has been cleaned up")
	}

	return pt.validateNode(pt.rootNode)
}

// validateNode recursively validates a node and its children.
func (pt *ParseTree) validateNode(node *ParseNode) (bool, error) {
	if node == nil {
		return false, errors.New("node is nil")
	}

	// Basic validation
	if node.StartByte > node.EndByte {
		return false, errors.New("node start byte is greater than end byte")
	}

	if int64(node.EndByte) > int64(len(pt.source)) {
		return false, errors.New("node end byte exceeds source length")
	}

	// Validate children
	for _, child := range node.Children {
		isValid, err := pt.validateNode(child)
		if !isValid {
			return false, err
		}
	}

	return true, nil
}

// HasSyntaxErrors checks if the parse tree has syntax errors using tree-sitter's native error detection.
func (pt *ParseTree) HasSyntaxErrors() (bool, error) {
	if pt.isCleanedUp {
		return false, errors.New("parse tree has been cleaned up")
	}

	// First check if we have tree-sitter tree reference for native error detection
	if pt.treeSitterTree != nil {
		rootTSNode := pt.treeSitterTree.RootNode()
		if !rootTSNode.IsNull() {
			return rootTSNode.HasError(), nil
		}
	}

	// Fallback to checking nodes for tree-sitter native methods if available
	return pt.hasErrorNodesWithNativeMethods(pt.rootNode), nil
}

// hasErrorNodesWithNativeMethods recursively checks for error nodes using tree-sitter's native methods.
func (pt *ParseTree) hasErrorNodesWithNativeMethods(node *ParseNode) bool {
	if node == nil {
		return false
	}

	// Use tree-sitter native error detection if available
	if tsNode := node.TreeSitterNode(); tsNode != nil && !tsNode.IsNull() {
		return pt.checkTreeSitterNodeErrors(tsNode)
	} else if node.Type == "ERROR" || node.Type == "MISSING" {
		return true
	}

	// Recursively check children
	for _, child := range node.Children {
		if pt.hasErrorNodesWithNativeMethods(child) {
			return true
		}
	}

	return false
}

// checkTreeSitterNodeErrors checks tree-sitter native error methods for a node.
func (pt *ParseTree) checkTreeSitterNodeErrors(tsNode *tree_sitter.Node) bool {
	// Check HasError() - if node or children have errors
	if tsNode.HasError() {
		return true
	}
	// Check IsError() - if node is specifically an error node
	if tsNode.IsError() {
		return true
	}
	// Check IsMissing() - if required tokens are missing
	if tsNode.IsMissing() {
		return true
	}
	return false
}

// ValidateConsistency validates the consistency of the parse tree.
func (pt *ParseTree) ValidateConsistency() (bool, error) {
	if pt.isCleanedUp {
		return false, errors.New("parse tree has been cleaned up")
	}

	// Check if metadata node count matches actual count
	actualCount := pt.GetTotalNodeCount()
	if pt.metadata.NodeCount > 0 && pt.metadata.NodeCount != actualCount {
		return false, errors.New("metadata node count inconsistent with actual tree")
	}

	// Check if metadata max depth matches actual depth
	actualDepth := pt.GetTreeDepth()
	if pt.metadata.MaxDepth > 0 && pt.metadata.MaxDepth != actualDepth {
		return false, fmt.Errorf(
			"metadata max depth %d inconsistent with actual depth %d",
			pt.metadata.MaxDepth,
			actualDepth,
		)
	}

	return true, nil
}

// ToJSON serializes the parse tree to JSON.
func (pt *ParseTree) ToJSON() (string, error) {
	if pt.isCleanedUp {
		return "", errors.New("parse tree has been cleaned up")
	}

	return fmt.Sprintf(`{
  "language": "%s",
  "rootNode": {
    "type": "%s",
    "startByte": %d,
    "endByte": %d
  },
  "metadata": {
    "nodeCount": %d,
    "maxDepth": %d,
    "treeSitterVersion": "%s",
    "grammarVersion": "%s"
  },
  "createdAt": "%s"
}`, pt.language.Name(), pt.rootNode.Type, pt.rootNode.StartByte, pt.rootNode.EndByte,
		pt.metadata.NodeCount, pt.metadata.MaxDepth, pt.metadata.TreeSitterVersion,
		pt.metadata.GrammarVersion, pt.createdAt.Format(time.RFC3339)), nil
}

// ToSExpression converts the parse tree to S-expression format.
func (pt *ParseTree) ToSExpression() (string, error) {
	if pt.isCleanedUp {
		return "", errors.New("parse tree has been cleaned up")
	}

	return pt.nodeToSExpression(pt.rootNode), nil
}

// nodeToSExpression recursively converts a node to S-expression.
func (pt *ParseTree) nodeToSExpression(node *ParseNode) string {
	if node == nil {
		return ""
	}

	if len(node.Children) == 0 {
		return fmt.Sprintf("(%s)", node.Type)
	}

	var parts []string
	parts = append(parts, node.Type)

	for _, child := range node.Children {
		parts = append(parts, pt.nodeToSExpression(child))
	}

	return fmt.Sprintf("(%s)", strings.Join(parts, " "))
}

// ToGraphQLSchema converts the parse tree to GraphQL schema format.
func (pt *ParseTree) ToGraphQLSchema() (string, error) {
	if pt.isCleanedUp {
		return "", errors.New("parse tree has been cleaned up")
	}

	return `type ParseTree {
  language: String!
  rootNode: ParseNode!
  metadata: ParseMetadata!
  createdAt: String!
}

type ParseNode {
  type: String!
  startByte: Int!
  endByte: Int!
  startPos: Position!
  endPos: Position!
  children: [ParseNode!]!
}

type Position {
  row: Int!
  column: Int!
}

type ParseMetadata {
  nodeCount: Int!
  maxDepth: Int!
  parseDuration: String!
  treeSitterVersion: String!
  grammarVersion: String!
}`, nil
}

// initParseTreeMetrics initializes OTEL metrics for ParseTree operations.
func initParseTreeMetrics() (*parseTreeMetrics, error) {
	meter := otel.Meter("codechunking/parse_tree")

	parseOpsCounter, err := meter.Int64Counter(
		"parse_tree_operations_total",
		metric.WithDescription("Total number of parse tree operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse operations counter: %w", err)
	}

	parseTimeHist, err := meter.Float64Histogram(
		"parse_tree_duration_seconds",
		metric.WithDescription("Duration of parse tree operations in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse time histogram: %w", err)
	}

	nodeCountHist, err := meter.Int64Histogram(
		"parse_tree_node_count",
		metric.WithDescription("Number of nodes in parse tree"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create node count histogram: %w", err)
	}

	depthHist, err := meter.Int64Histogram(
		"parse_tree_depth",
		metric.WithDescription("Depth of parse tree"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create depth histogram: %w", err)
	}

	cleanupCounter, err := meter.Int64Counter(
		"parse_tree_cleanup_operations_total",
		metric.WithDescription("Total number of parse tree cleanup operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cleanup counter: %w", err)
	}

	memoryGauge, err := meter.Int64Gauge(
		"parse_tree_memory_usage_bytes",
		metric.WithDescription("Memory usage of parse tree in bytes"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory gauge: %w", err)
	}

	return &parseTreeMetrics{
		parseOperationsCounter:   parseOpsCounter,
		parseTimeHistogram:       parseTimeHist,
		treeNodeCountHistogram:   nodeCountHist,
		treeDepthHistogram:       depthHist,
		cleanupOperationsCounter: cleanupCounter,
		memoryUsageGauge:         memoryGauge,
	}, nil
}

// recordParseOperation records metrics for a parse operation.
func (m *parseTreeMetrics) recordParseOperation(
	ctx context.Context,
	language string,
	duration time.Duration,
	nodeCount, depth int,
	sourceSize int64,
) {
	attrs := []attribute.KeyValue{
		attribute.String("language", language),
		attribute.String("operation", "create"),
	}

	m.parseOperationsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.parseTimeHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.treeNodeCountHistogram.Record(ctx, int64(nodeCount), metric.WithAttributes(attrs...))
	m.treeDepthHistogram.Record(ctx, int64(depth), metric.WithAttributes(attrs...))
	m.memoryUsageGauge.Record(ctx, sourceSize, metric.WithAttributes(attrs...))
}

// recordCleanupOperation records metrics for a cleanup operation.
func (m *parseTreeMetrics) recordCleanupOperation(ctx context.Context, language string) {
	attrs := []attribute.KeyValue{
		attribute.String("language", language),
		attribute.String("operation", "cleanup"),
	}
	m.cleanupOperationsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// Cleanup cleans up the parse tree resources with proper resource management.
func (pt *ParseTree) Cleanup(ctx context.Context) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.isCleanedUp {
		slogger.Debug(ctx, "ParseTree already cleaned up", slogger.Fields{
			"language": pt.language.Name(),
		})
		return nil
	}

	slogger.Debug(ctx, "Cleaning up ParseTree resources", slogger.Fields{
		"language":   pt.language.Name(),
		"node_count": pt.metadata.NodeCount,
	})

	// Clean up tree-sitter tree if it exists
	if pt.treeSitterTree != nil {
		pt.treeSitterTree.Close()
		pt.treeSitterTree = nil
	}

	// Record cleanup metrics
	if pt.metrics != nil {
		pt.metrics.recordCleanupOperation(ctx, pt.language.Name())
	}

	pt.isCleanedUp = true
	pt.rootNode = nil
	pt.source = nil

	slogger.Info(ctx, "ParseTree resources cleaned up successfully", slogger.Fields{
		"language": pt.language.Name(),
	})

	return nil
}

// NewParseNodeWithTreeSitter creates a new ParseNode with tree-sitter node reference.
func NewParseNodeWithTreeSitter(
	nodeType string,
	startByte, endByte uint32,
	startPos, endPos Position,
	children []*ParseNode,
	tsNode tree_sitter.Node,
) (*ParseNode, error) {
	return &ParseNode{
		Type:      nodeType,
		StartByte: startByte,
		EndByte:   endByte,
		StartPos:  startPos,
		EndPos:    endPos,
		Children:  children,
		tsNode:    &tsNode,
	}, nil
}

// HasTreeSitterNode returns true if this ParseNode has a tree-sitter node reference.
func (pn *ParseNode) HasTreeSitterNode() bool {
	return pn != nil && pn.tsNode != nil
}

// TreeSitterNode returns the tree-sitter node reference if available.
func (pn *ParseNode) TreeSitterNode() *tree_sitter.Node {
	if pn == nil || pn.tsNode == nil {
		return nil
	}
	return pn.tsNode
}

// IsErrorNode checks if a node represents an error.
func (pn *ParseNode) IsErrorNode() bool {
	return pn != nil && pn.Type == "ERROR"
}

// ChildByFieldName returns a child node by its field name from the tree-sitter grammar.
// This method uses tree-sitter's field access capabilities to find named fields
// according to the grammar definition, providing a more reliable way to access
// specific parts of a language construct than manual traversal.
//
// Parameters:
//
//	fieldName - The name of the field as defined in the tree-sitter grammar
//
// Returns:
//
//	*ParseNode - The child node for the specified field, or nil if not found
//
// Example usage:
//
//	// For a method_declaration node, get the receiver field
//	receiverNode := methodNode.ChildByFieldName("receiver")
//
//	// For a function_declaration node, get parameters and result
//	parametersNode := functionNode.ChildByFieldName("parameters")
//	resultNode := functionNode.ChildByFieldName("result")
func (pn *ParseNode) ChildByFieldName(fieldName string) *ParseNode {
	if pn == nil || fieldName == "" {
		return nil
	}

	// Use tree-sitter node field access if available
	if pn.tsNode != nil {
		// Get the child node by field name from tree-sitter
		childTSNode := pn.tsNode.ChildByFieldName(fieldName)
		if childTSNode.IsNull() {
			return nil
		}

		// Get positions with safe conversion from uint to uint32
		childStartByte := childTSNode.StartByte()
		childEndByte := childTSNode.EndByte()

		// Check for potential overflow before conversion
		if childStartByte > 0xFFFFFFFF || childEndByte > 0xFFFFFFFF {
			return nil // Position too large for uint32
		}

		// Find the corresponding ParseNode in our children
		// We need to match by position since ParseNode wraps tree-sitter nodes
		for _, child := range pn.Children {
			if child != nil && child.StartByte == uint32(childStartByte) &&
				child.EndByte == uint32(childEndByte) {
				return child
			}
		}

		// If we have a tree-sitter node but no corresponding ParseNode,
		// we may need to create one (this shouldn't happen in normal use)
		return nil
	}

	// Fallback: If no tree-sitter node, we cannot determine field names reliably
	// Field access requires the grammar information from tree-sitter
	return nil
}
