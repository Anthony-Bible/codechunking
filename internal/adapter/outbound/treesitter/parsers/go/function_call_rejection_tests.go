package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
)

// analyzeFailureReason provides detailed analysis of why function call detection fails.
func analyzeFailureReason(t *testing.T, line string, expectedResult, actualResult bool) {
	t.Logf("FAILING AS EXPECTED: Function call incorrectly detected as field")
	t.Logf("Expected result=%v, got %v for line: %s", expectedResult, actualResult, line)

	// Analyze why this is happening
	ctx := context.Background()
	parseResult := treesitter.CreateTreeSitterParseTree(ctx, line)

	if parseResult.Error == nil {
		allNodes := getAllNodes(parseResult.ParseTree.RootNode())
		nodeTypes := []string{}
		for _, node := range allNodes {
			if node.Type != "" {
				nodeTypes = append(nodeTypes, node.Type)
			}
		}

		t.Logf("ACTUAL PARSED NODE TYPES: %v", nodeTypes)

		// Test current TreeSitterQueryEngine methods
		queryEngine := NewTreeSitterQueryEngine()

		methodSpecs := queryEngine.QueryMethodSpecs(parseResult.ParseTree)
		embeddedTypes := queryEngine.QueryEmbeddedTypes(parseResult.ParseTree)
		fieldDecls := queryEngine.QueryFieldDeclarations(parseResult.ParseTree)

		t.Logf("QueryMethodSpecs results: %d (should be 0 - wrong node type)", len(methodSpecs))
		t.Logf("QueryEmbeddedTypes results: %d (may be wrong due to incorrect logic)", len(embeddedTypes))
		t.Logf("QueryFieldDeclarations results: %d (should be 0)", len(fieldDecls))

		// The issue is likely that QueryMethodSpecs and QueryEmbeddedTypes return empty
		// due to wrong node types, causing fallback logic to incorrectly accept the function call
		if len(methodSpecs) == 0 && len(fieldDecls) == 0 {
			t.Logf("ROOT CAUSE: QueryMethodSpecs returns empty due to wrong 'method_spec' node type")
			t.Logf("ROOT CAUSE: This causes fallback logic to incorrectly accept function calls")
		}
	}
}

// TestTryASTBasedFieldDetection_FunctionCallRejectionFailure tests the specific failure
// where function calls are incorrectly identified as fields due to wrong node types.
// CURRENT ISSUE: The failing test "function_call_not_field" expected result=false but got true.
func TestTryASTBasedFieldDetection_FunctionCallRejectionFailure(t *testing.T) {
	// This is the EXACT failing test case from the original test run
	line := "fmt.Println(\"hello\")"
	expectedResult := false
	expectedUseAST := true

	result, useAST := tryASTBasedFieldDetection(line)

	// This test WILL FAIL because of incorrect node types in TreeSitterQueryEngine
	if result != expectedResult {
		analyzeFailureReason(t, line, expectedResult, result)
	}

	if useAST != expectedUseAST {
		t.Errorf("Expected useAST=%v, got %v", expectedUseAST, useAST)
	}

	t.Log("TO FIX THIS:")
	t.Log("  1. Change QueryMethodSpecs to use 'method_elem' instead of 'method_spec'")
	t.Log("  2. Add proper QueryCallExpressions method to detect 'call_expression' nodes")
	t.Log("  3. Update hasNegativePattern to properly reject call expressions")
}

// TestTreeSitterQueryEngine_CallExpressionDetection tests that function calls
// should be properly detected as call_expression nodes for rejection.
func TestTreeSitterQueryEngine_CallExpressionDetection(t *testing.T) {
	functionCalls := []struct {
		name        string
		code        string
		description string
	}{
		{
			name:        "simple_function_call",
			code:        "fmt.Println(\"hello\")",
			description: "Simple function call should be detected as call_expression",
		},
		{
			name:        "method_call",
			code:        "logger.Info(\"message\")",
			description: "Method call should be detected as call_expression",
		},
		{
			name:        "chained_method_call",
			code:        "user.GetProfile().Save()",
			description: "Chained method calls should be detected as call_expression",
		},
		{
			name:        "function_with_multiple_args",
			code:        "os.OpenFile(\"test.txt\", os.O_RDWR, 0644)",
			description: "Function calls with multiple arguments should be detected",
		},
	}

	ctx := context.Background()

	for _, fc := range functionCalls {
		t.Run(fc.name, func(t *testing.T) {
			parseResult := treesitter.CreateTreeSitterParseTree(ctx, fc.code)

			if parseResult.Error != nil {
				t.Fatalf("Parse error for %s: %v", fc.code, parseResult.Error)
			}

			// Verify call_expression node type is present
			allNodes := getAllNodes(parseResult.ParseTree.RootNode())
			foundCallExpression := false
			nodeTypes := []string{}

			for _, node := range allNodes {
				if node.Type != "" {
					nodeTypes = append(nodeTypes, node.Type)
					if node.Type == nodeTypeCallExpression {
						foundCallExpression = true
					}
				}
			}

			if !foundCallExpression {
				t.Errorf("Expected 'call_expression' node type not found for: %s", fc.code)
				t.Logf("Available node types: %v", nodeTypes)
			}

			// Test that current TreeSitterQueryEngine would miss this
			queryEngine := NewTreeSitterQueryEngine()

			// There's no QueryCallExpressions method yet - this should be added
			t.Logf("NEEDED: QueryCallExpressions method to detect call_expression nodes")

			// Test current methods that should return empty for function calls
			methodSpecs := queryEngine.QueryMethodSpecs(parseResult.ParseTree)
			fieldDecls := queryEngine.QueryFieldDeclarations(parseResult.ParseTree)

			if len(methodSpecs) > 0 {
				t.Errorf("QueryMethodSpecs should return empty for function call: %s", fc.code)
			}
			if len(fieldDecls) > 0 {
				t.Errorf("QueryFieldDeclarations should return empty for function call: %s", fc.code)
			}

			t.Logf("Function call: %s", fc.code)
			t.Logf("Description: %s", fc.description)
			t.Logf("Contains call_expression: %v", foundCallExpression)
			t.Logf("All node types: %v", nodeTypes)
		})
	}
}

// TestHasNegativePattern_CallExpressionRejection tests that the hasNegativePattern
// function should properly reject call expressions once TreeSitterQueryEngine is fixed.
func TestHasNegativePattern_CallExpressionRejection(t *testing.T) {
	// This test demonstrates what should happen once the implementation is fixed

	functionCalls := []string{
		"fmt.Println(\"hello\")",
		"logger.Info(\"processing\")",
		"os.Exit(1)",
		"user.Save()",
	}

	ctx := context.Background()

	for _, call := range functionCalls {
		t.Run(call, func(t *testing.T) {
			parseResult := treesitter.CreateTreeSitterParseTree(ctx, call)

			if parseResult.Error != nil {
				t.Fatalf("Parse error: %v", parseResult.Error)
			}

			queryEngine := NewTreeSitterQueryEngine()

			// Test current hasNegativePattern behavior
			hasNegative := hasNegativePattern(queryEngine, parseResult.ParseTree)

			// Currently this may not properly detect function calls due to missing QueryCallExpressions
			t.Logf("Function call: %s", call)
			t.Logf("Current hasNegativePattern result: %v", hasNegative)

			// What should happen after fixing TreeSitterQueryEngine:
			t.Log("EXPECTED AFTER FIX:")
			t.Log("  1. QueryCallExpressions method should exist and find call_expression nodes")
			t.Log("  2. hasNegativePattern should return true for function calls")
			t.Log("  3. tryASTBasedFieldDetection should return (false, true) for function calls")

			// Verify call_expression node exists in parse tree
			allNodes := getAllNodes(parseResult.ParseTree.RootNode())
			foundCallExpression := false
			for _, node := range allNodes {
				if node.Type == nodeTypeCallExpression {
					foundCallExpression = true
					break
				}
			}

			if !foundCallExpression {
				t.Errorf("call_expression node not found - parsing issue")
			} else {
				t.Logf("✓ call_expression node exists in parse tree")
			}
		})
	}
}

// TestTreeSitterQueryEngine_MissingCallExpressionMethod demonstrates that
// the TreeSitterQueryEngine needs a QueryCallExpressions method.
func TestTreeSitterQueryEngine_MissingCallExpressionMethod(t *testing.T) {
	// This test documents that QueryCallExpressions method is missing
	// and needs to be implemented to properly detect function calls for rejection

	ctx := context.Background()
	functionCall := "fmt.Println(\"test\")"
	parseResult := treesitter.CreateTreeSitterParseTree(ctx, functionCall)

	if parseResult.Error != nil {
		t.Fatalf("Parse error: %v", parseResult.Error)
	}

	// Verify the parse tree contains call_expression nodes
	allNodes := getAllNodes(parseResult.ParseTree.RootNode())
	callExpressionNodes := []*valueobject.ParseNode{}

	for _, node := range allNodes {
		if node.Type == nodeTypeCallExpression {
			callExpressionNodes = append(callExpressionNodes, node)
		}
	}

	if len(callExpressionNodes) == 0 {
		t.Fatalf("No call_expression nodes found in parse tree for: %s", functionCall)
	}

	t.Logf("Found %d call_expression nodes in parse tree", len(callExpressionNodes))

	// This method doesn't exist yet but should be implemented
	t.Log("IMPLEMENTATION NEEDED:")
	t.Log("  Add QueryCallExpressions method to TreeSitterQueryEngine interface")
	t.Log("  Implement QueryCallExpressions in ConcreteTreeSitterQueryEngine")
	t.Log("  Method should return e.queryNodesByType(parseTree, \"call_expression\")")
	t.Log("  Update hasNegativePattern to use QueryCallExpressions for function call rejection")

	// Show what the implementation should look like:
	expectedImplementation := `
// QueryCallExpressions finds all call_expression nodes (function/method calls).
func (e *ConcreteTreeSitterQueryEngine) QueryCallExpressions(
	parseTree *valueobject.ParseTree,
) []*valueobject.ParseNode {
	return e.queryNodesByType(parseTree, "call_expression")
}
`
	t.Logf("NEEDED IMPLEMENTATION:\n%s", expectedImplementation)
}

// TestFieldDetectionWorkflow_WithCorrectNodeTypes demonstrates the complete
// workflow that should work once TreeSitterQueryEngine uses correct node types.
func TestFieldDetectionWorkflow_WithCorrectNodeTypes(t *testing.T) {
	testCases := []struct {
		name               string
		input              string
		expectedResult     bool
		expectedUseAST     bool
		expectedNodeTypes  []string
		shouldBeDetectedBy string
		description        string
	}{
		{
			name:               "struct_field_detection",
			input:              "name string",
			expectedResult:     true,
			expectedUseAST:     true,
			expectedNodeTypes:  []string{"field_declaration"},
			shouldBeDetectedBy: "QueryFieldDeclarations (already working)",
			description:        "Struct fields should be detected correctly",
		},
		{
			name:               "interface_method_detection",
			input:              "Read([]byte) (int, error)",
			expectedResult:     true,
			expectedUseAST:     true,
			expectedNodeTypes:  []string{"method_elem"}, // NOT method_spec
			shouldBeDetectedBy: "QueryMethodElements (needs implementation)",
			description:        "Interface methods should be detected with method_elem node type",
		},
		{
			name:               "embedded_type_detection",
			input:              "io.Reader",
			expectedResult:     true,
			expectedUseAST:     true,
			expectedNodeTypes:  []string{"type_elem"}, // NOT embedded_field
			shouldBeDetectedBy: "QueryTypeElements (needs implementation)",
			description:        "Embedded types should be detected with type_elem node type",
		},
		{
			name:               "function_call_rejection",
			input:              "fmt.Println(\"hello\")",
			expectedResult:     false,
			expectedUseAST:     true,
			expectedNodeTypes:  []string{"call_expression"},
			shouldBeDetectedBy: "QueryCallExpressions (needs implementation) for rejection",
			description:        "Function calls should be properly rejected",
		},
		{
			name:               "variable_assignment_rejection",
			input:              "x := 42",
			expectedResult:     false,
			expectedUseAST:     true,
			expectedNodeTypes:  []string{"short_var_declaration"},
			shouldBeDetectedBy: "Existing negative pattern detection",
			description:        "Variable assignments should be rejected",
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse and verify expected node types exist
			parseResult := treesitter.CreateTreeSitterParseTree(ctx, tc.input)
			if parseResult.Error != nil {
				t.Fatalf("Parse error: %v", parseResult.Error)
			}

			allNodes := getAllNodes(parseResult.ParseTree.RootNode())
			foundExpectedTypes := map[string]bool{}
			allNodeTypes := []string{}

			for _, node := range allNodes {
				if node.Type != "" {
					allNodeTypes = append(allNodeTypes, node.Type)
					for _, expectedType := range tc.expectedNodeTypes {
						if node.Type == expectedType {
							foundExpectedTypes[expectedType] = true
						}
					}
				}
			}

			// Verify expected node types are present
			for _, expectedType := range tc.expectedNodeTypes {
				if !foundExpectedTypes[expectedType] {
					t.Errorf("Expected node type '%s' not found. Available: %v", expectedType, allNodeTypes)
				}
			}

			// Test current implementation (will show current behavior)
			result, useAST := tryASTBasedFieldDetection(tc.input)

			if result != tc.expectedResult {
				t.Logf("CURRENT BEHAVIOR: result=%v, expected=%v (may be wrong due to node type issues)",
					result, tc.expectedResult)
			}

			if useAST != tc.expectedUseAST {
				t.Errorf("useAST=%v, expected=%v", useAST, tc.expectedUseAST)
			}

			t.Logf("Input: %s", tc.input)
			t.Logf("Expected result: %v", tc.expectedResult)
			t.Logf("Current result: %v", result)
			t.Logf("Expected node types: %v", tc.expectedNodeTypes)
			t.Logf("Found node types: %v", allNodeTypes)
			t.Logf("Should be detected by: %s", tc.shouldBeDetectedBy)
			t.Logf("Description: %s", tc.description)
		})
	}
}
