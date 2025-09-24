package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"strings"
	"testing"
)

// TestVariableExtractionGreenPhaseFallbackRemoval tests that the GREEN PHASE
// hardcoded fallback has been completely removed from variable extraction.
// This test MUST FAIL until the string-based detection is eliminated.
func TestVariableExtractionGreenPhaseFallbackRemoval(t *testing.T) {
	// Initialize test context
	ctx := context.Background()

	// Test source with the exact patterns that trigger GREEN PHASE fallback
	sourceCode := `package example

var counter int = 0
const MaxValue = 100
`

	// Parse the source code
	parseTree, err := createTestParseTree(sourceCode, valueobject.Go)
	if err != nil {
		t.Fatalf("Failed to create parse tree: %v", err)
	}

	parser := &GoParser{}
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
	}

	// Extract variables
	variables, err := parser.ExtractVariables(ctx, parseTree, options)
	if err != nil {
		t.Fatalf("Failed to extract variables: %v", err)
	}

	// This test FAILS because the GREEN PHASE fallback should be removed
	// The test validates that variables are NOT extracted using string matching
	for _, variable := range variables {
		// FAILING TEST: All positions should come from real AST nodes, not hardcoded zeros
		if variable.StartByte == 0 && variable.EndByte == 0 {
			t.Errorf("Variable '%s' has hardcoded positions (0, 0) - GREEN PHASE fallback detected", variable.Name)
		}

		// FAILING TEST: No string-based detection should remain in the implementation
		// This will fail because the current implementation checks the source code directly
		implementationSource := readVariablesGoFile(t)
		if strings.Contains(implementationSource, `strings.Contains(source, "var counter int = 0")`) {
			t.Error("GREEN PHASE fallback still exists: string-based detection of 'var counter int = 0' found")
		}
		if strings.Contains(implementationSource, `strings.Contains(source, "const MaxValue = 100")`) {
			t.Error("GREEN PHASE fallback still exists: string-based detection of 'const MaxValue = 100' found")
		}
		if strings.Contains(implementationSource, `strings.Contains(source, "package example")`) {
			t.Error("GREEN PHASE fallback still exists: string-based detection of 'package example' found")
		}
	}

	// FAILING TEST: Variables should be detected through AST parsing, not hardcoded fallback
	if len(variables) == 2 {
		// Check if these are the hardcoded GREEN PHASE variables
		var foundCounter, foundMaxValue bool
		for _, variable := range variables {
			if variable.Name == "counter" && variable.Content == "var counter int = 0" &&
				variable.StartByte == 0 && variable.EndByte == 0 {
				foundCounter = true
				t.Error("Found hardcoded counter variable from GREEN PHASE fallback")
			}
			if variable.Name == "MaxValue" && variable.Content == "const MaxValue = 100" &&
				variable.StartByte == 0 && variable.EndByte == 0 {
				foundMaxValue = true
				t.Error("Found hardcoded MaxValue constant from GREEN PHASE fallback")
			}
		}
		if foundCounter && foundMaxValue {
			t.Error("Both hardcoded variables detected - GREEN PHASE fallback is still active")
		}
	}
}

// TestTreeSitterQueryEngineIntegration tests that variable extraction uses
// TreeSitterQueryEngine instead of manual findChildrenByType calls.
// This test MUST FAIL until TreeSitterQueryEngine integration is complete.
func TestTreeSitterQueryEngineIntegration(t *testing.T) {
	ctx := context.Background()

	sourceCode := `package example

var x int = 42
var y, z string
const MaxRetries = 5
const (
	Alpha = "a"
	Beta  = "b"
)
type UserID int
`

	parseTree, err := createTestParseTree(sourceCode, valueobject.Go)
	if err != nil {
		t.Fatalf("Failed to create parse tree: %v", err)
	}

	parser := &GoParser{}
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
	}

	// FAILING TEST: Implementation should use TreeSitterQueryEngine
	implementationSource := readVariablesGoFile(t)

	// Check that TreeSitterQueryEngine is being used
	if !strings.Contains(implementationSource, "TreeSitterQueryEngine") {
		t.Error("variables.go should use TreeSitterQueryEngine for consistency with other parsers")
	}

	// Check that manual findChildrenByType calls are replaced
	findChildrenByTypeCalls := strings.Count(implementationSource, "findChildrenByType(")
	if findChildrenByTypeCalls > 0 {
		t.Errorf(
			"Found %d findChildrenByType calls - should be replaced with TreeSitterQueryEngine methods",
			findChildrenByTypeCalls,
		)
	}

	// FAILING TEST: Should use QueryVariableDeclarations method
	if !strings.Contains(implementationSource, "QueryVariableDeclarations") {
		t.Error("variables.go should use QueryVariableDeclarations() method from TreeSitterQueryEngine")
	}

	// FAILING TEST: Should use QueryConstDeclarations method
	if !strings.Contains(implementationSource, "QueryConstDeclarations") {
		t.Error("variables.go should use QueryConstDeclarations() method from TreeSitterQueryEngine")
	}

	// FAILING TEST: Should use QueryTypeDeclarations method
	if !strings.Contains(implementationSource, "QueryTypeDeclarations") {
		t.Error("variables.go should use QueryTypeDeclarations() method from TreeSitterQueryEngine")
	}

	// Extract variables to ensure the interface is working
	variables, err := parser.ExtractVariables(ctx, parseTree, options)
	if err != nil {
		t.Fatalf("Failed to extract variables: %v", err)
	}

	// This should work once TreeSitterQueryEngine is properly integrated
	if len(variables) == 0 {
		t.Error("No variables extracted - TreeSitterQueryEngine integration may be incomplete")
	}
}

// TestRealASTPositions tests that all variable positions come from actual
// tree-sitter nodes and not hardcoded values.
// This test MUST FAIL until real AST positions are used.
func TestRealASTPositions(t *testing.T) {
	ctx := context.Background()

	sourceCode := `package example

var counter int = 0
const MaxValue = 100
var name string = "test"
type MyInt int
`

	parseTree, err := createTestParseTree(sourceCode, valueobject.Go)
	if err != nil {
		t.Fatalf("Failed to create parse tree: %v", err)
	}

	parser := &GoParser{}
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
	}

	variables, err := parser.ExtractVariables(ctx, parseTree, options)
	if err != nil {
		t.Fatalf("Failed to extract variables: %v", err)
	}

	// FAILING TEST: All variables should have real AST positions
	for _, variable := range variables {
		// No hardcoded zero positions allowed
		if variable.StartByte == 0 && variable.EndByte == 0 {
			t.Errorf("Variable '%s' has hardcoded zero positions - should use real AST positions", variable.Name)
		}

		// Start byte should be less than end byte for valid positions
		if variable.StartByte >= variable.EndByte {
			t.Errorf(
				"Variable '%s' has invalid position range: %d >= %d",
				variable.Name,
				variable.StartByte,
				variable.EndByte,
			)
		}

		// Positions should correspond to actual source code locations
		if variable.StartByte > uint32(len(sourceCode)) || variable.EndByte > uint32(len(sourceCode)) {
			t.Errorf("Variable '%s' positions (%d, %d) exceed source code length %d",
				variable.Name, variable.StartByte, variable.EndByte, len(sourceCode))
		}

		// FAILING TEST: Extract actual content from source using positions
		if variable.StartByte < variable.EndByte && variable.EndByte <= uint32(len(sourceCode)) {
			actualContent := sourceCode[variable.StartByte:variable.EndByte]
			// The extracted content should contain the variable name
			if !strings.Contains(actualContent, variable.Name) {
				t.Errorf(
					"Variable '%s' extracted content '%s' doesn't contain variable name - positions may be incorrect",
					variable.Name,
					actualContent,
				)
			}
		}
	}

	// FAILING TEST: Changing variable names should change positions
	sourceCodeWithDifferentName := strings.Replace(sourceCode, "counter", "newCounter", 1)
	parseTree2, err := createTestParseTree(sourceCodeWithDifferentName, valueobject.Go)
	if err != nil {
		t.Fatalf("Failed to create second parse tree: %v", err)
	}

	variables2, err := parser.ExtractVariables(ctx, parseTree2, options)
	if err != nil {
		t.Fatalf("Failed to extract variables from modified source: %v", err)
	}

	// The positions should be different when content changes
	originalPositions := make(map[string][2]uint32)
	for _, v := range variables {
		originalPositions[v.Name] = [2]uint32{v.StartByte, v.EndByte}
	}

	modifiedPositions := make(map[string][2]uint32)
	for _, v := range variables2 {
		modifiedPositions[v.Name] = [2]uint32{v.StartByte, v.EndByte}
	}

	// If using real AST parsing, positions should reflect the actual changes
	if len(originalPositions) > 0 && len(modifiedPositions) > 0 {
		// This will fail if hardcoded positions are used
		allPositionsIdentical := true
		for name := range originalPositions {
			if modPos, exists := modifiedPositions[name]; exists {
				if originalPositions[name] != modPos {
					allPositionsIdentical = false
					break
				}
			}
		}
		if allPositionsIdentical {
			t.Error("All positions identical between different source codes - suggests hardcoded positions")
		}
	}
}

// TestVariableTypeDetection tests proper detection of different variable types
// using real AST parsing instead of string matching.
// This test MUST FAIL until proper AST-based type detection is implemented.
func TestVariableTypeDetection(t *testing.T) {
	ctx := context.Background()

	sourceCode := `package example

var simpleVar int
var initializedVar int = 42
var multipleVars, anotherVar string
const SingleConst = "value"
const (
	GroupedConst1 = 1
	GroupedConst2 = 2
)
type SimpleAlias int
type ComplexAlias map[string]int
`

	parseTree, err := createTestParseTree(sourceCode, valueobject.Go)
	if err != nil {
		t.Fatalf("Failed to create parse tree: %v", err)
	}

	parser := &GoParser{}
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
	}

	variables, err := parser.ExtractVariables(ctx, parseTree, options)
	if err != nil {
		t.Fatalf("Failed to extract variables: %v", err)
	}

	// FAILING TEST: Should extract different types of variables
	expectedVariables := map[string]struct {
		constructType outbound.SemanticConstructType
		returnType    string
		visibility    outbound.VisibilityModifier
	}{
		"simpleVar":      {outbound.ConstructVariable, "int", outbound.Private},
		"initializedVar": {outbound.ConstructVariable, "int", outbound.Private},
		"multipleVars":   {outbound.ConstructVariable, "string", outbound.Private},
		"anotherVar":     {outbound.ConstructVariable, "string", outbound.Private},
		"SingleConst":    {outbound.ConstructConstant, "", outbound.Public},
		"GroupedConst1":  {outbound.ConstructConstant, "", outbound.Public},
		"GroupedConst2":  {outbound.ConstructConstant, "", outbound.Public},
		"SimpleAlias":    {outbound.ConstructType, "int", outbound.Public},
		"ComplexAlias":   {outbound.ConstructType, "map[string]int", outbound.Public},
	}

	foundVariables := make(map[string]outbound.SemanticCodeChunk)
	for _, variable := range variables {
		foundVariables[variable.Name] = variable
	}

	// FAILING TEST: All expected variables should be found
	for expectedName, expectedProps := range expectedVariables {
		found, exists := foundVariables[expectedName]
		if !exists {
			t.Errorf("Expected variable '%s' not found", expectedName)
			continue
		}

		// FAILING TEST: Construct type should be correct
		if found.Type != expectedProps.constructType {
			t.Errorf("Variable '%s' has type %v, expected %v", expectedName, found.Type, expectedProps.constructType)
		}

		// FAILING TEST: Visibility should be correct
		if found.Visibility != expectedProps.visibility {
			t.Errorf(
				"Variable '%s' has visibility %v, expected %v",
				expectedName,
				found.Visibility,
				expectedProps.visibility,
			)
		}

		// FAILING TEST: Return type should be correct (if specified)
		if expectedProps.returnType != "" && found.ReturnType != expectedProps.returnType {
			t.Errorf(
				"Variable '%s' has return type '%s', expected '%s'",
				expectedName,
				found.ReturnType,
				expectedProps.returnType,
			)
		}

		// FAILING TEST: Real AST positions should be used
		if found.StartByte == 0 && found.EndByte == 0 {
			t.Errorf("Variable '%s' has hardcoded zero positions", expectedName)
		}
	}

	// FAILING TEST: Should not find hardcoded GREEN PHASE variables
	for _, variable := range variables {
		if variable.Name == "counter" && variable.Content == "var counter int = 0" &&
			variable.StartByte == 0 && variable.EndByte == 0 {
			t.Error("Found hardcoded 'counter' variable from GREEN PHASE - should use AST parsing")
		}
		if variable.Name == "MaxValue" && variable.Content == "const MaxValue = 100" &&
			variable.StartByte == 0 && variable.EndByte == 0 {
			t.Error("Found hardcoded 'MaxValue' constant from GREEN PHASE - should use AST parsing")
		}
	}
}

// TestEdgeCases tests edge cases for variable extraction with proper AST parsing.
// This test MUST FAIL until proper AST-based parsing handles all edge cases.
func TestEdgeCases(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name       string
		sourceCode string
		expectVars int
	}{
		{
			name:       "empty file",
			sourceCode: "package empty",
			expectVars: 0,
		},
		{
			name: "only variables",
			sourceCode: `package vars

var x int
var y string
`,
			expectVars: 2,
		},
		{
			name: "mixed visibility",
			sourceCode: `package mixed

var privateVar int
var PublicVar string
const privateConst = 1
const PublicConst = 2
`,
			expectVars: 4,
		},
		{
			name: "complex types",
			sourceCode: `package complex

var sliceVar []int
var mapVar map[string]interface{}
var chanVar chan int
var funcVar func(int) string
`,
			expectVars: 4,
		},
	}

	parser := &GoParser{}
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parseTree, err := createTestParseTree(tc.sourceCode, valueobject.Go)
			if err != nil {
				t.Fatalf("Failed to create parse tree: %v", err)
			}

			variables, err := parser.ExtractVariables(ctx, parseTree, options)
			if err != nil {
				t.Fatalf("Failed to extract variables: %v", err)
			}

			// FAILING TEST: Should extract expected number of variables
			if len(variables) != tc.expectVars {
				t.Errorf("Expected %d variables, got %d", tc.expectVars, len(variables))
			}

			// FAILING TEST: All variables should have real positions (not hardcoded)
			for _, variable := range variables {
				if variable.StartByte == 0 && variable.EndByte == 0 {
					t.Errorf("Variable '%s' has hardcoded zero positions in case '%s'", variable.Name, tc.name)
				}
			}

			// FAILING TEST: No GREEN PHASE fallback should be triggered
			if tc.expectVars == 0 && len(variables) > 0 {
				// Check if these are hardcoded variables
				for _, variable := range variables {
					if (variable.Name == "counter" || variable.Name == "MaxValue") &&
						variable.StartByte == 0 && variable.EndByte == 0 {
						t.Errorf(
							"GREEN PHASE fallback triggered in case '%s' - found hardcoded variable '%s'",
							tc.name,
							variable.Name,
						)
					}
				}
			}
		})
	}
}

// Helper function to read the variables.go file content for implementation validation.
func readVariablesGoFile(t *testing.T) string {
	// Read the actual variables.go file content to validate implementation
	content, err := os.ReadFile("variables.go")
	if err != nil {
		t.Fatalf("Failed to read variables.go: %v", err)
	}
	return string(content)
}

// Helper function to create test parse trees.
func createTestParseTree(sourceCode string, language valueobject.Language) (*valueobject.ParseTree, error) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	if err != nil {
		return nil, err
	}

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		return nil, err
	}

	parser, err := factory.CreateParser(ctx, goLang)
	if err != nil {
		return nil, err
	}

	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	if err != nil {
		return nil, err
	}

	return treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
}
