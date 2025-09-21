package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVariadicParameterParsing focuses specifically on testing the parseGoParameterList function
// to ensure it can handle both parameter_declaration AND variadic_parameter_declaration nodes.
// This test addresses the failing "Variadic_function" case where the parser returns empty Parameters
// instead of the expected [{Name: "nums", Type: "...int"}].
func TestVariadicParameterParsing(t *testing.T) {
	ctx := context.Background()
	parser, err := NewGoParser()
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name               string
		sourceCode         string
		expectedParameters []outbound.Parameter
	}{
		{
			name: "Variadic parameter with name and type",
			sourceCode: `
// Sum calculates the sum of all provided numbers
func Sum(nums ...int) int {
	total := 0
	for _, num := range nums {
		total += num
	}
	return total
}
`,
			expectedParameters: []outbound.Parameter{
				{Name: "nums", Type: "...int"},
			},
		},
		{
			name: "Mixed regular and variadic parameters",
			sourceCode: `
// ProcessMessages processes a prefix and variable number of messages
func ProcessMessages(prefix string, messages ...string) []string {
	result := make([]string, len(messages))
	for i, msg := range messages {
		result[i] = prefix + msg
	}
	return result
}
`,
			expectedParameters: []outbound.Parameter{
				{Name: "prefix", Type: "string"},
				{Name: "messages", Type: "...string"},
			},
		},
		{
			name: "Variadic parameter with complex type",
			sourceCode: `
// CombineSlices combines multiple slices into one
func CombineSlices(slices ...[]int) []int {
	var result []int
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}
`,
			expectedParameters: []outbound.Parameter{
				{Name: "slices", Type: "...[]int"},
			},
		},
		{
			name: "Variadic parameter with interface type",
			sourceCode: `
// LogValues logs multiple values of any type
func LogValues(values ...interface{}) {
	for _, value := range values {
		fmt.Println(value)
	}
}
`,
			expectedParameters: []outbound.Parameter{
				{Name: "values", Type: "...interface{}"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the source code with tree-sitter
			parseTree, err := parser.Parse(ctx, []byte(tt.sourceCode))
			require.NoError(t, err)
			require.NotNil(t, parseTree)

			// Convert to domain tree
			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
			require.NoError(t, err)
			require.NotNil(t, domainTree)

			// Extract function chunks
			adapter := treesitter.NewSemanticTraverserAdapter()
			options := &treesitter.SemanticExtractionOptions{
				IncludeFunctions: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only function chunks
			var funcChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructFunction {
					funcChunks = append(funcChunks, chunk)
				}
			}

			// Should have exactly one function
			require.Len(t, funcChunks, 1, "Expected exactly one function chunk")

			// Verify the parameters match expectations
			actualParams := funcChunks[0].Parameters
			assert.Len(t, actualParams, len(tt.expectedParameters),
				"Parameter count mismatch - parseGoParameterList is not handling variadic_parameter_declaration nodes")

			for i, expectedParam := range tt.expectedParameters {
				if i < len(actualParams) {
					assert.Equal(t, expectedParam.Name, actualParams[i].Name,
						"Parameter name mismatch at index %d", i)
					assert.Equal(t, expectedParam.Type, actualParams[i].Type,
						"Parameter type mismatch at index %d - variadic type should include '...' prefix", i)
				}
			}
		})
	}
}

// TestVariadicParameterParsingUnit tests the parseGoParameterList function directly
// to isolate the issue with variadic parameter detection.
func TestVariadicParameterParsingUnit(t *testing.T) {
	ctx := context.Background()
	parser, err := NewGoParser()
	require.NoError(t, err)
	require.NotNil(t, parser)

	// Source code with variadic parameter
	sourceCode := `func Sum(nums ...int) int { return 0 }`

	// Parse the source code
	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)
	require.NotNil(t, parseTree)

	// Convert to domain tree
	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err)
	require.NotNil(t, domainTree)

	// Find the function declaration node
	functionNodes := domainTree.GetNodesByType("function_declaration")
	require.Len(t, functionNodes, 1, "Should find exactly one function declaration")

	functionNode := functionNodes[0]

	// Find the parameter_list node within the function
	var paramListNode *valueobject.ParseNode
	for _, child := range functionNode.Children {
		if child.Type == "parameter_list" {
			paramListNode = child
			break
		}
	}
	require.NotNil(t, paramListNode, "Should find parameter_list node")

	// Call parseGoParameterList directly to test its behavior
	goParser := &GoParser{}
	parameters := goParser.parseGoParameterList(paramListNode, domainTree)

	// This should fail until the bug is fixed - currently returns empty slice
	// but should return [{Name: "nums", Type: "...int"}]
	assert.Len(t, parameters, 1,
		"parseGoParameterList should detect variadic_parameter_declaration nodes, not just parameter_declaration")

	if len(parameters) > 0 {
		assert.Equal(t, "nums", parameters[0].Name,
			"Variadic parameter name should be extracted correctly")
		assert.Equal(t, "...int", parameters[0].Type,
			"Variadic parameter type should include '...' prefix")
	}

	// Debug: Log the child node types to understand what tree-sitter actually produces
	t.Logf("parameter_list children:")
	for i, child := range paramListNode.Children {
		t.Logf("  [%d] %s: %s", i, child.Type, domainTree.GetNodeText(child))
	}
}
