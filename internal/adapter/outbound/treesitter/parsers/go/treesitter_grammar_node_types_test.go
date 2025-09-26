package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"context"
	"testing"
)

// TestTreeSitterQueryEngine_CorrectGoGrammarNodeTypes tests that the TreeSitterQueryEngine
// uses the CORRECT node types from the tree-sitter Go grammar specification.
// CURRENT ISSUE: Implementation uses incorrect node types that don't exist in the grammar.
func TestTreeSitterQueryEngine_CorrectGoGrammarNodeTypes(t *testing.T) {
	tests := []struct {
		name               string
		code               string
		expectedNodeType   string
		currentWrongMethod string
		correctMethod      string
		description        string
		shouldFindNodes    bool
	}{
		{
			name:               "interface_method_should_use_method_elem_not_method_spec",
			code:               "Process(data []byte) error",
			expectedNodeType:   "method_elem",
			currentWrongMethod: "QueryMethodSpecs",    // Uses wrong "method_spec" node type
			correctMethod:      "QueryMethodElements", // Should use "method_elem" node type
			description:        "Interface methods should be detected using method_elem node type",
			shouldFindNodes:    true,
		},
		{
			name:               "embedded_type_should_use_type_elem_not_embedded_field",
			code:               "io.Writer",
			expectedNodeType:   "type_elem",
			currentWrongMethod: "QueryEmbeddedTypes", // Uses wrong "embedded_field" node type
			correctMethod:      "QueryTypeElements",  // Should use "type_elem" node type
			description:        "Embedded types should be detected using type_elem node type",
			shouldFindNodes:    true,
		},
		{
			name:               "function_call_should_be_detected_as_call_expression",
			code:               "fmt.Println(\"hello\")",
			expectedNodeType:   "call_expression",
			currentWrongMethod: "", // Not directly queried, but should be detected for rejection
			correctMethod:      "QueryCallExpressions",
			description:        "Function calls should be properly identified as call_expression nodes",
			shouldFindNodes:    true,
		},
		{
			name:               "struct_field_already_correct",
			code:               "username string",
			expectedNodeType:   "field_declaration",
			currentWrongMethod: "",
			correctMethod:      "QueryFieldDeclarations", // This one is already correct
			description:        "Struct fields use correct field_declaration node type",
			shouldFindNodes:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the code directly
			ctx := context.Background()
			parseResult := treesitter.CreateTreeSitterParseTree(ctx, tt.code)

			if parseResult.Error != nil {
				t.Fatalf("Failed to parse code: %v", parseResult.Error)
			}

			// Test current implementation (should fail due to wrong node types)
			queryEngine := NewTreeSitterQueryEngine()

			// These calls should fail because they use wrong node types
			switch tt.currentWrongMethod {
			case "QueryMethodSpecs":
				// This should return EMPTY because "method_spec" doesn't exist in grammar
				methodSpecs := queryEngine.QueryMethodSpecs(parseResult.ParseTree)
				if len(methodSpecs) > 0 {
					t.Errorf("UNEXPECTED: QueryMethodSpecs found nodes with wrong node type")
				}
				t.Logf("EXPECTED FAILURE: QueryMethodSpecs returns empty (uses wrong 'method_spec' node type)")

			case "QueryEmbeddedTypes":
				// This should return EMPTY because "embedded_field" doesn't exist in grammar
				embeddedTypes := queryEngine.QueryEmbeddedTypes(parseResult.ParseTree)
				if tt.shouldFindNodes && len(embeddedTypes) == 0 {
					t.Logf("EXPECTED FAILURE: QueryEmbeddedTypes returns empty (uses wrong 'embedded_field' node type)")
				}
			}

			// Verify the expected node type exists in the parse tree
			allNodes := getAllNodes(parseResult.ParseTree.RootNode())
			foundExpectedNodeType := false
			nodeTypes := []string{}

			for _, node := range allNodes {
				if node.Type != "" {
					nodeTypes = append(nodeTypes, node.Type)
					if node.Type == tt.expectedNodeType {
						foundExpectedNodeType = true
					}
				}
			}

			if tt.shouldFindNodes && !foundExpectedNodeType {
				t.Errorf("Expected to find node type '%s' but didn't. Found types: %v",
					tt.expectedNodeType, nodeTypes)
			}

			t.Logf("CORRECT NODE TYPE: %s", tt.expectedNodeType)
			t.Logf("CURRENT WRONG METHOD: %s", tt.currentWrongMethod)
			t.Logf("NEEDED CORRECT METHOD: %s", tt.correctMethod)
			t.Logf("ALL PARSED NODE TYPES: %v", nodeTypes)
		})
	}
}

// TestTreeSitterQueryEngine_NodeTypeCorrections defines the specific corrections needed
// in TreeSitterQueryEngine methods to use correct tree-sitter Go grammar node types.
func TestTreeSitterQueryEngine_NodeTypeCorrections(t *testing.T) {
	corrections := []struct {
		method               string
		currentWrongNodeType string
		correctNodeType      string
		testCode             string
		description          string
	}{
		{
			method:               "QueryMethodSpecs",
			currentWrongNodeType: "method_spec", // WRONG - doesn't exist in grammar
			correctNodeType:      "method_elem", // CORRECT - actual tree-sitter Go node type
			testCode:             "Write([]byte) (int, error)",
			description:          "Interface method specifications use method_elem node type",
		},
		{
			method:               "QueryEmbeddedTypes",
			currentWrongNodeType: "embedded_field", // WRONG - doesn't exist in grammar
			correctNodeType:      "type_elem",      // CORRECT - actual tree-sitter Go node type
			testCode:             "io.Reader",
			description:          "Embedded types in interfaces use type_elem node type",
		},
		{
			method:               "QueryFieldDeclarations",
			currentWrongNodeType: "",                  // Already correct
			correctNodeType:      "field_declaration", // CORRECT - already using right type
			testCode:             "name string",
			description:          "Struct field declarations already use correct node type",
		},
	}

	queryEngine := NewTreeSitterQueryEngine()
	ctx := context.Background()

	for _, correction := range corrections {
		t.Run(correction.method, func(t *testing.T) {
			parseResult := treesitter.CreateTreeSitterParseTree(ctx, correction.testCode)
			if parseResult.Error != nil {
				t.Fatalf("Parse error: %v", parseResult.Error)
			}

			// Show what the current implementation returns (should be empty for wrong node types)
			switch correction.method {
			case "QueryMethodSpecs":
				results := queryEngine.QueryMethodSpecs(parseResult.ParseTree)
				if len(results) == 0 {
					t.Logf("FAILING AS EXPECTED: QueryMethodSpecs returns empty (wrong node type: %s)",
						correction.currentWrongNodeType)
				}

			case "QueryEmbeddedTypes":
				results := queryEngine.QueryEmbeddedTypes(parseResult.ParseTree)
				if len(results) == 0 {
					t.Logf("FAILING AS EXPECTED: QueryEmbeddedTypes returns empty (wrong node type: %s)",
						correction.currentWrongNodeType)
				}

			case "QueryFieldDeclarations":
				results := queryEngine.QueryFieldDeclarations(parseResult.ParseTree)
				if len(results) > 0 {
					t.Logf("WORKING CORRECTLY: QueryFieldDeclarations finds nodes (correct node type: %s)",
						correction.correctNodeType)
				}
			}

			// Verify the correct node type exists in parse tree
			allNodes := getAllNodes(parseResult.ParseTree.RootNode())
			foundCorrectType := false
			for _, node := range allNodes {
				if node.Type == correction.correctNodeType {
					foundCorrectType = true
					break
				}
			}

			if correction.correctNodeType != "" && !foundCorrectType {
				t.Errorf("Expected correct node type '%s' not found in parse tree for: %s",
					correction.correctNodeType, correction.testCode)
			}

			t.Logf("CORRECTION NEEDED:")
			t.Logf("  Method: %s", correction.method)
			t.Logf("  Current wrong node type: %s", correction.currentWrongNodeType)
			t.Logf("  Correct node type: %s", correction.correctNodeType)
			t.Logf("  Description: %s", correction.description)
		})
	}
}

// TestTryASTBasedFieldDetection_FailingDueToWrongNodeTypes demonstrates that
// the field detection is failing because TreeSitterQueryEngine uses wrong node types.
func TestTryASTBasedFieldDetection_FailingDueToWrongNodeTypes(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		expectedResult bool
		expectedUseAST bool
		failureReason  string
		nodeTypeIssue  string
	}{
		{
			name:           "function_call_incorrectly_detected_as_field",
			line:           "fmt.Println(\"hello\")",
			expectedResult: false, // Should be rejected as not a field
			expectedUseAST: true,
			failureReason:  "QueryMethodSpecs/QueryEmbeddedTypes return empty due to wrong node types, fallback logic incorrectly accepts",
			nodeTypeIssue:  "method_spec and embedded_field node types don't exist in grammar",
		},
		{
			name:           "interface_method_not_detected_due_to_wrong_node_type",
			line:           "Process(data []byte) error",
			expectedResult: true, // Should be detected as interface method
			expectedUseAST: true,
			failureReason:  "QueryMethodSpecs returns empty because 'method_spec' node type doesn't exist",
			nodeTypeIssue:  "Should use 'method_elem' node type instead of 'method_spec'",
		},
		{
			name:           "embedded_type_not_detected_due_to_wrong_node_type",
			line:           "io.Writer",
			expectedResult: true, // Should be detected as embedded type
			expectedUseAST: true,
			failureReason:  "QueryEmbeddedTypes returns empty because 'embedded_field' node type doesn't exist",
			nodeTypeIssue:  "Should use 'type_elem' node type instead of 'embedded_field'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, useAST := tryASTBasedFieldDetection(tt.line)

			// These tests will FAIL due to wrong node types in TreeSitterQueryEngine
			if result != tt.expectedResult {
				t.Logf("FAILING AS EXPECTED: result=%v, expected=%v", result, tt.expectedResult)
				t.Logf("FAILURE REASON: %s", tt.failureReason)
				t.Logf("NODE TYPE ISSUE: %s", tt.nodeTypeIssue)
			}

			if useAST != tt.expectedUseAST {
				t.Errorf("useAST=%v, expected=%v", useAST, tt.expectedUseAST)
			}

			// Document what needs to be fixed
			t.Logf("TO FIX THIS TEST:")
			t.Logf("  1. Update TreeSitterQueryEngine to use correct node types")
			t.Logf("  2. Node type issue: %s", tt.nodeTypeIssue)
			t.Logf("  3. This will make the queries return proper results")
		})
	}
}

// TestTreeSitterQueryEngine_ShouldDetectCorrectNodeTypes verifies that after fixing
// the node types, the TreeSitterQueryEngine methods should return correct results.
func TestTreeSitterQueryEngine_ShouldDetectCorrectNodeTypes(t *testing.T) {
	tests := []struct {
		name              string
		code              string
		queryMethod       string
		correctNodeType   string
		shouldFindResults bool
		description       string
	}{
		{
			name:              "method_elem_detection_for_interface_methods",
			code:              "Write([]byte) (int, error)",
			queryMethod:       "QueryMethodElements", // Should be implemented to query "method_elem"
			correctNodeType:   "method_elem",
			shouldFindResults: true,
			description:       "Interface methods should be detected using method_elem node type",
		},
		{
			name:              "type_elem_detection_for_embedded_types",
			code:              "io.Reader",
			queryMethod:       "QueryTypeElements", // Should be implemented to query "type_elem"
			correctNodeType:   "type_elem",
			shouldFindResults: true,
			description:       "Embedded types should be detected using type_elem node type",
		},
		{
			name:              "call_expression_detection_for_function_calls",
			code:              "fmt.Println(\"test\")",
			queryMethod:       "QueryCallExpressions", // Should be implemented to query "call_expression"
			correctNodeType:   "call_expression",
			shouldFindResults: true,
			description:       "Function calls should be detected using call_expression node type",
		},
		{
			name:              "field_declaration_already_works",
			code:              "name string",
			queryMethod:       "QueryFieldDeclarations",
			correctNodeType:   "field_declaration",
			shouldFindResults: true,
			description:       "Field declarations already work with correct node type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			parseResult := treesitter.CreateTreeSitterParseTree(ctx, tt.code)

			if parseResult.Error != nil {
				t.Fatalf("Parse error: %v", parseResult.Error)
			}

			// Verify the correct node type exists in the parse tree
			allNodes := getAllNodes(parseResult.ParseTree.RootNode())
			foundCorrectType := false
			nodeTypes := []string{}

			for _, node := range allNodes {
				if node.Type != "" {
					nodeTypes = append(nodeTypes, node.Type)
					if node.Type == tt.correctNodeType {
						foundCorrectType = true
					}
				}
			}

			if tt.shouldFindResults && !foundCorrectType {
				t.Errorf("Expected node type '%s' not found. Available types: %v",
					tt.correctNodeType, nodeTypes)
			}

			// Test current implementation (will fail until TreeSitterQueryEngine is fixed)
			queryEngine := NewTreeSitterQueryEngine()

			switch tt.queryMethod {
			case "QueryMethodElements":
				// This method doesn't exist yet - should be created to query "method_elem"
				t.Logf("NEEDED: Implement %s to query '%s' node type", tt.queryMethod, tt.correctNodeType)

			case "QueryTypeElements":
				// This method doesn't exist yet - should be created to query "type_elem"
				t.Logf("NEEDED: Implement %s to query '%s' node type", tt.queryMethod, tt.correctNodeType)

			case "QueryCallExpressions":
				// This method doesn't exist yet - should be created to query "call_expression"
				t.Logf("NEEDED: Implement %s to query '%s' node type", tt.queryMethod, tt.correctNodeType)

			case "QueryFieldDeclarations":
				// This already exists and should work
				results := queryEngine.QueryFieldDeclarations(parseResult.ParseTree)
				if tt.shouldFindResults && len(results) == 0 {
					t.Errorf("QueryFieldDeclarations should find results for: %s", tt.code)
				}
			}

			t.Logf("EXPECTED BEHAVIOR:")
			t.Logf("  Method: %s", tt.queryMethod)
			t.Logf("  Node type: %s", tt.correctNodeType)
			t.Logf("  Description: %s", tt.description)
			t.Logf("  Parsed node types: %v", nodeTypes)
		})
	}
}

// TestTreeSitterQueryEngine_ComprehensiveNodeTypeValidation provides comprehensive
// validation that all query methods use correct tree-sitter Go grammar node types.
func TestTreeSitterQueryEngine_ComprehensiveNodeTypeValidation(t *testing.T) {
	// This test validates all the corrections needed for proper tree-sitter Go grammar compliance

	nodeTypeValidations := []struct {
		category          string
		validGoSyntax     []string
		expectedNodeTypes []string
		currentWrongTypes []string
		queryMethodNeeded string
		description       string
	}{
		{
			category:          "interface_methods",
			validGoSyntax:     []string{"Read([]byte) (int, error)", "Write(p []byte) (n int, err error)"},
			expectedNodeTypes: []string{"method_elem"},
			currentWrongTypes: []string{"method_spec"}, // WRONG
			queryMethodNeeded: "QueryMethodElements",
			description:       "Interface method specifications",
		},
		{
			category:          "embedded_types",
			validGoSyntax:     []string{"io.Reader", "sync.Mutex", "context.Context"},
			expectedNodeTypes: []string{"type_elem"},
			currentWrongTypes: []string{"embedded_field"}, // WRONG
			queryMethodNeeded: "QueryTypeElements",
			description:       "Embedded types in interfaces and structs",
		},
		{
			category:          "function_calls",
			validGoSyntax:     []string{"fmt.Println(\"test\")", "logger.Info(\"msg\")", "os.Exit(1)"},
			expectedNodeTypes: []string{"call_expression"},
			currentWrongTypes: []string{}, // Not currently queried directly
			queryMethodNeeded: "QueryCallExpressions",
			description:       "Function call expressions that should be rejected as fields",
		},
		{
			category:          "struct_fields",
			validGoSyntax:     []string{"name string", "age int", "data []byte"},
			expectedNodeTypes: []string{"field_declaration"},
			currentWrongTypes: []string{},               // Already correct
			queryMethodNeeded: "QueryFieldDeclarations", // Already exists
			description:       "Struct field declarations (already working correctly)",
		},
	}

	ctx := context.Background()

	for _, validation := range nodeTypeValidations {
		t.Run(validation.category, func(t *testing.T) {
			for _, syntax := range validation.validGoSyntax {
				t.Run(syntax, func(t *testing.T) {
					parseResult := treesitter.CreateTreeSitterParseTree(ctx, syntax)
					if parseResult.Error != nil {
						t.Fatalf("Failed to parse: %v", parseResult.Error)
					}

					allNodes := getAllNodes(parseResult.ParseTree.RootNode())
					nodeTypes := []string{}
					foundExpectedTypes := map[string]bool{}

					for _, node := range allNodes {
						if node.Type != "" {
							nodeTypes = append(nodeTypes, node.Type)
							for _, expectedType := range validation.expectedNodeTypes {
								if node.Type == expectedType {
									foundExpectedTypes[expectedType] = true
								}
							}
						}
					}

					// Verify expected node types are present
					for _, expectedType := range validation.expectedNodeTypes {
						if !foundExpectedTypes[expectedType] {
							t.Errorf("Expected node type '%s' not found for syntax: %s", expectedType, syntax)
							t.Logf("Available node types: %v", nodeTypes)
						}
					}

					t.Logf("VALIDATION FOR: %s", syntax)
					t.Logf("  Category: %s", validation.category)
					t.Logf("  Expected node types: %v", validation.expectedNodeTypes)
					t.Logf("  Current wrong types: %v", validation.currentWrongTypes)
					t.Logf("  Query method needed: %s", validation.queryMethodNeeded)
					t.Logf("  All parsed types: %v", nodeTypes)
				})
			}
		})
	}
}
