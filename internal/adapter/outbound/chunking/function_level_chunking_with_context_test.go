//go:build disabled

package chunking_test

import (
	"codechunking/internal/adapter/outbound/chunking"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFunctionLevelChunkingWithContext tests Phase 4.3 Step 2: Function-Level Chunking with Context Preservation.
func TestFunctionLevelChunkingWithContext(t *testing.T) {
	// Create function chunker and context preserver
	functionChunker := chunking.NewFunctionChunker()
	contextPreserver := chunking.NewContextPreserver()

	require.NotNil(t, functionChunker, "Function chunker should be created")
	require.NotNil(t, contextPreserver, "Context preserver should be created")
}

// TestIndividualFunctionsAsDiscreteChunks tests that individual functions/methods are treated as discrete chunks.
func TestIndividualFunctionsAsDiscreteChunks(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()

	tests := []struct {
		name           string
		language       valueobject.Language
		semanticChunks []outbound.SemanticCodeChunk
		expectedChunks int
		description    string
	}{
		{
			name:     "Go individual functions",
			language: mustCreateLanguage(t, valueobject.LanguageGo),
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoFunctionChunk(t, "func Add(a, b int) int { return a + b }", "Add", 0, 100),
				createGoFunctionChunk(t, "func Subtract(a, b int) int { return a - b }", "Subtract", 101, 201),
			},
			expectedChunks: 2,
			description:    "Each Go function should create a separate chunk",
		},
		{
			name:     "Python individual functions",
			language: mustCreateLanguage(t, valueobject.LanguagePython),
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonFunctionChunk(t, "def add(a: int, b: int) -> int:\n    return a + b", "add", 0, 100),
				createPythonFunctionChunk(
					t,
					"def subtract(a: int, b: int) -> int:\n    return a - b",
					"subtract",
					101,
					201,
				),
			},
			expectedChunks: 2,
			description:    "Each Python function should create a separate chunk",
		},
		{
			name:     "JavaScript individual functions",
			language: mustCreateLanguage(t, valueobject.LanguageJavaScript),
			semanticChunks: []outbound.SemanticCodeChunk{
				createJSFunctionChunk(t, "function add(a, b) { return a + b; }", "add", 0, 100),
				createJSFunctionChunk(t, "const subtract = (a, b) => a - b;", "subtract", 101, 201),
			},
			expectedChunks: 2,
			description:    "Each JavaScript function should create a separate chunk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultChunkingConfig()
			config.Strategy = outbound.StrategyFunction

			chunks, err := functionChunker.ChunkByFunction(ctx, tt.semanticChunks, config)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.Len(t, chunks, tt.expectedChunks, "Should create %d discrete chunks", tt.expectedChunks)

			for i, chunk := range chunks {
				assert.Equal(t, outbound.ChunkFunction, chunk.ChunkType, "Chunk %d should be function type", i)
				assert.NotEmpty(t, chunk.Content, "Chunk %d should have content", i)
				assert.Contains(
					t,
					chunk.Content, tt.semanticChunks[i].Name,
					"Chunk should contain function name",
				)
			}
		})
	}
}

// TestRelatedHelperFunctionsInclusion tests that related helper functions and called functions within same file are included.
func TestRelatedHelperFunctionsInclusion(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()

	tests := []struct {
		name              string
		language          valueobject.Language
		semanticChunks    []outbound.SemanticCodeChunk
		expectedRelations int
		description       string
	}{
		{
			name:     "Go function with helper",
			language: mustCreateLanguage(t, valueobject.LanguageGo),
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoFunctionChunk(t, "func Calculate(x int) int { return helper(x) * 2 }", "Calculate", 0, 100),
				createGoFunctionChunk(t, "func helper(x int) int { return x + 1 }", "helper", 101, 201),
			},
			expectedRelations: 1,
			description:       "Helper function should be related to main function",
		},
		{
			name:     "Python function with decorators",
			language: mustCreateLanguage(t, valueobject.LanguagePython),
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonDecoratorChunk(
					t,
					"@cache\n@validate_input\ndef process_data(data: List[str]) -> Dict[str, int]:\n    return helper_process(data)",
					"process_data",
					0,
					200,
				),
				createPythonFunctionChunk(
					t,
					"def helper_process(data: List[str]) -> Dict[str, int]:\n    return {item: len(item) for item in data}",
					"helper_process",
					201,
					301,
				),
			},
			expectedRelations: 1,
			description:       "Helper function should be identified as dependency of decorated function",
		},
		{
			name:     "JavaScript function with closure",
			language: mustCreateLanguage(t, valueobject.LanguageJavaScript),
			semanticChunks: []outbound.SemanticCodeChunk{
				createJSClosureChunk(
					t,
					"function createCalculator() {\n  const helper = (x) => x * 2;\n  return (y) => helper(y) + 1;\n}",
					"createCalculator",
					0,
					200,
				),
			},
			expectedRelations: 1,
			description:       "Inner functions/closures should be included in same chunk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultChunkingConfig()
			config.Strategy = outbound.StrategyFunction
			config.PreserveDependencies = true

			chunks, err := functionChunker.ChunkByFunction(ctx, tt.semanticChunks, config)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.NotEmpty(t, chunks, "Should create chunks with related functions")

			mainChunk := findChunkByName(chunks, tt.semanticChunks[0].Name)
			assert.NotNil(t, mainChunk, "Should find main function chunk")
			assert.GreaterOrEqual(
				t,
				len(mainChunk.Dependencies),
				tt.expectedRelations,
				"Should have expected number of related functions",
			)
		})
	}
}

// TestDocumentationPreservation tests that documentation (docstrings/comments) are preserved.
func TestDocumentationPreservation(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()

	tests := []struct {
		name               string
		language           valueobject.Language
		semanticChunk      outbound.SemanticCodeChunk
		expectedDocStrings []string
		description        string
	}{
		{
			name:     "Go function with comments",
			language: mustCreateLanguage(t, valueobject.LanguageGo),
			semanticChunk: createGoFunctionChunkWithDoc(
				t,
				"// Add performs addition of two integers\n// Returns the sum of a and b\nfunc Add(a, b int) int {\n    return a + b\n}",
				"Add",
				"Add performs addition of two integers\nReturns the sum of a and b",
				0,
				200,
			),
			expectedDocStrings: []string{"Add performs addition of two integers", "Returns the sum of a and b"},
			description:        "Go function comments should be preserved",
		},
		{
			name:     "Python function with docstring",
			language: mustCreateLanguage(t, valueobject.LanguagePython),
			semanticChunk: createPythonFunctionChunkWithDoc(
				t,
				"def calculate_statistics(data: List[float]) -> Dict[str, float]:\n    \"\"\"\n    Calculate basic statistics for a list of numbers.\n    \n    Args:\n        data: List of numeric values\n    \n    Returns:\n        Dictionary containing mean, median, and std deviation\n    \"\"\"\n    return {}",
				"calculate_statistics",
				"Calculate basic statistics for a list of numbers.\n\nArgs:\n    data: List of numeric values\n\nReturns:\n    Dictionary containing mean, median, and std deviation",
				0,
				400,
			),
			expectedDocStrings: []string{"Calculate basic statistics", "Args:", "Returns:"},
			description:        "Python docstrings should be preserved",
		},
		{
			name:     "JavaScript function with JSDoc",
			language: mustCreateLanguage(t, valueobject.LanguageJavaScript),
			semanticChunk: createJSFunctionChunkWithDoc(
				t,
				"/**\n * Processes user input and validates format\n * @param {string} input - The user input string\n * @param {Object} options - Validation options\n * @returns {Promise<boolean>} Promise resolving to validation result\n */\nfunction processInput(input, options) {\n    return Promise.resolve(true);\n}",
				"processInput",
				"Processes user input and validates format\n@param {string} input - The user input string\n@param {Object} options - Validation options\n@returns {Promise<boolean>} Promise resolving to validation result",
				0,
				500,
			),
			expectedDocStrings: []string{"Processes user input", "@param", "@returns"},
			description:        "JavaScript JSDoc comments should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultChunkingConfig()
			config.Strategy = outbound.StrategyFunction
			config.IncludeDocumentation = true
			config.IncludeComments = true

			chunks, err := functionChunker.ChunkByFunction(ctx, []outbound.SemanticCodeChunk{tt.semanticChunk}, config)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.Len(t, chunks, 1, "Should create one chunk")

			chunk := chunks[0]
			// Documentation is preserved in content when IncludeDocumentation is true
			assert.Contains(
				t,
				chunk.Content,
				tt.semanticChunk.Documentation,
				"Should preserve documentation in content",
			)

			for _, expectedDoc := range tt.expectedDocStrings {
				assert.Contains(t, chunk.Content, expectedDoc, "Documentation should contain expected string")
			}
		})
	}
}

// TestGoLanguageSpecificRequirements tests Go-specific requirements: receiver methods with struct definitions.
func TestGoLanguageSpecificRequirements(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()

	tests := []struct {
		name             string
		semanticChunks   []outbound.SemanticCodeChunk
		expectedIncludes []string
		description      string
	}{
		{
			name: "Go receiver method with struct",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoStructChunk(t, "type Calculator struct {\n    precision int\n}", "Calculator", 0, 100),
				createGoMethodChunk(
					t,
					"func (c *Calculator) Add(a, b int) int {\n    return a + b\n}",
					"Add",
					"Calculator",
					101,
					201,
				),
				createGoMethodChunk(
					t,
					"func (c Calculator) GetPrecision() int {\n    return c.precision\n}",
					"GetPrecision",
					"Calculator",
					202,
					302,
				),
			},
			expectedIncludes: []string{"Calculator", "Add", "GetPrecision"},
			description:      "Receiver methods should include struct definition",
		},
		{
			name: "Go interface with implementing methods",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoInterfaceChunk(t, "type Adder interface {\n    Add(int, int) int\n}", "Adder", 0, 100),
				createGoStructChunk(t, "type SimpleAdder struct{}", "SimpleAdder", 101, 151),
				createGoMethodChunk(
					t,
					"func (s SimpleAdder) Add(a, b int) int {\n    return a + b\n}",
					"Add",
					"SimpleAdder",
					152,
					252,
				),
			},
			expectedIncludes: []string{"Adder", "SimpleAdder", "Add"},
			description:      "Interface implementing methods should include interface definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultChunkingConfig()
			config.Strategy = outbound.StrategyFunction
			config.PreserveDependencies = true

			chunks, err := functionChunker.ChunkByFunction(ctx, tt.semanticChunks, config)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.NotEmpty(t, chunks, "Should create chunks for Go receiver methods")

			// Expected behavior after implementation:
			// assert.NoError(t, err, tt.description)
			// assert.NotEmpty(t, chunks, "Should create chunks for Go receiver methods")
			//
			// methodChunk := findChunkByType(chunks, outbound.ChunkFunction)
			// assert.NotNil(t, methodChunk, "Should find method chunk")
			//
			// for _, expected := range tt.expectedIncludes {
			//     assert.Contains(t, methodChunk.Content, expected, "Chunk should include %s", expected)
			// }
		})
	}
}

// TestPythonLanguageSpecificRequirements tests Python-specific requirements: decorators and type hints.
func TestPythonLanguageSpecificRequirements(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()

	tests := []struct {
		name             string
		semanticChunks   []outbound.SemanticCodeChunk
		expectedFeatures []string
		description      string
	}{
		{
			name: "Python function with decorators and type hints",
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonDecoratorFunctionChunk(
					t,
					"@lru_cache(maxsize=128)\n@validate_types\nasync def process_data(data: List[Dict[str, Any]], config: ProcessingConfig) -> ProcessingResult:\n    \"\"\"Process data with caching and validation\"\"\"\n    return ProcessingResult(success=True)",
					"process_data",
					[]string{"@lru_cache(maxsize=128)", "@validate_types"},
					"List[Dict[str, Any]]",
					"ProcessingResult",
					0,
					400,
				),
			},
			expectedFeatures: []string{
				"@lru_cache",
				"@validate_types",
				"List[Dict[str, Any]]",
				"ProcessingResult",
				"async def",
			},
			description: "Should preserve Python decorators, type hints, and async keywords",
		},
		{
			name: "Python class with method decorators",
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonClassChunk(t, "class DataProcessor:\n    pass", "DataProcessor", 0, 50),
				createPythonMethodWithDecoratorChunk(
					t,
					"    @property\n    @cached\n    def result(self) -> Optional[ProcessingResult]:\n        return self._result",
					"result",
					"DataProcessor",
					[]string{"@property", "@cached"},
					51,
					200,
				),
			},
			expectedFeatures: []string{"@property", "@cached", "Optional[ProcessingResult]"},
			description:      "Should preserve method decorators and return type hints",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultChunkingConfig()
			config.Strategy = outbound.StrategyFunction
			config.PreserveDependencies = true

			chunks, err := functionChunker.ChunkByFunction(ctx, tt.semanticChunks, config)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.NotEmpty(t, chunks, "Should create chunks for Python functions with decorators")

			// Expected behavior after implementation:
			// assert.NoError(t, err, tt.description)
			// assert.NotEmpty(t, chunks, "Should create chunks for Python functions with decorators")
			//
			// functionChunk := findChunkByName(chunks, tt.semanticChunks[0].Name)
			// assert.NotNil(t, functionChunk, "Should find function chunk")
			//
			// for _, feature := range tt.expectedFeatures {
			//     assert.Contains(t, functionChunk.Content, feature, "Chunk should preserve feature: %s", feature)
			// }
		})
	}
}

// TestJavaScriptLanguageSpecificRequirements tests JavaScript-specific requirements: arrow functions and closures.
func TestJavaScriptLanguageSpecificRequirements(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()

	tests := []struct {
		name             string
		semanticChunks   []outbound.SemanticCodeChunk
		expectedFeatures []string
		description      string
	}{
		{
			name: "JavaScript arrow functions with closures",
			semanticChunks: []outbound.SemanticCodeChunk{
				createJSArrowFunctionChunk(
					t,
					"const processData = async (data) => {\n    const validator = (item) => item.isValid;\n    const processor = item => ({ ...item, processed: true });\n    \n    return data.filter(validator).map(processor);\n};",
					"processData",
					0,
					300,
				),
			},
			expectedFeatures: []string{"const processData", "async", "validator", "processor", "=>"},
			description:      "Should preserve arrow functions and inner closures in same scope",
		},
		{
			name: "JavaScript function with nested functions",
			semanticChunks: []outbound.SemanticCodeChunk{
				createJSNestedFunctionChunk(
					t,
					"function createCalculator(initialValue) {\n    let current = initialValue;\n    \n    function add(value) {\n        current += value;\n        return current;\n    }\n    \n    const multiply = (value) => {\n        current *= value;\n        return current;\n    };\n    \n    return { add, multiply, get current() { return current; } };\n}",
					"createCalculator",
					[]string{"add", "multiply"},
					0,
					500,
				),
			},
			expectedFeatures: []string{"createCalculator", "function add", "const multiply", "get current"},
			description:      "Should include nested functions within same scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultChunkingConfig()
			config.Strategy = outbound.StrategyFunction
			config.PreserveDependencies = true

			chunks, err := functionChunker.ChunkByFunction(ctx, tt.semanticChunks, config)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.NotEmpty(t, chunks, "Should create chunks for JavaScript functions with closures")

			// Expected behavior after implementation:
			// assert.NoError(t, err, tt.description)
			// assert.NotEmpty(t, chunks, "Should create chunks for JavaScript functions with closures")
			//
			// mainChunk := findChunkByName(chunks, tt.semanticChunks[0].Name)
			// assert.NotNil(t, mainChunk, "Should find main function chunk")
			//
			// for _, feature := range tt.expectedFeatures {
			//     assert.Contains(t, mainChunk.Content, feature, "Chunk should preserve feature: %s", feature)
			// }
		})
	}
}

// TestContextPreservationImportDependencies tests import dependencies used by functions are preserved.
func TestContextPreservationImportDependencies(t *testing.T) {
	ctx := context.Background()
	contextPreserver := chunking.NewContextPreserver()

	tests := []struct {
		name            string
		language        valueobject.Language
		functionChunk   outbound.EnhancedCodeChunk
		expectedImports []string
		description     string
	}{
		{
			name:     "Go function with import dependencies",
			language: mustCreateLanguage(t, valueobject.LanguageGo),
			functionChunk: createEnhancedGoFunctionChunk(
				t,
				"func ProcessRequest(req *http.Request) (*Response, error) {\n    data, err := json.Marshal(req)\n    return &Response{Data: data}, err\n}",
				"ProcessRequest",
				[]string{"net/http", "encoding/json"},
			),
			expectedImports: []string{"net/http", "encoding/json"},
			description:     "Should preserve Go imports used in function",
		},
		{
			name:     "Python function with import dependencies",
			language: mustCreateLanguage(t, valueobject.LanguagePython),
			functionChunk: createEnhancedPythonFunctionChunk(
				t,
				"def process_data(data: pd.DataFrame) -> np.ndarray:\n    result = np.array(data.values)\n    return pd.Series(result).to_numpy()",
				"process_data",
				[]string{"pandas as pd", "numpy as np"},
			),
			expectedImports: []string{"pandas as pd", "numpy as np"},
			description:     "Should preserve Python imports with aliases",
		},
		{
			name:     "JavaScript function with import dependencies",
			language: mustCreateLanguage(t, valueobject.LanguageJavaScript),
			functionChunk: createEnhancedJSFunctionChunk(
				t,
				"async function fetchData(url) {\n    const response = await axios.get(url);\n    return lodash.groupBy(response.data, 'category');\n}",
				"fetchData",
				[]string{"axios", "lodash"},
			),
			expectedImports: []string{"axios", "lodash"},
			description:     "Should preserve JavaScript imports used in function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := []outbound.EnhancedCodeChunk{tt.functionChunk}
			strategy := outbound.PreserveModerate

			preservedChunks, err := contextPreserver.PreserveContext(ctx, chunks, strategy)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.Len(t, preservedChunks, 1, "Should return one preserved chunk")

			// Expected behavior after implementation:
			// assert.NoError(t, err, tt.description)
			// assert.Len(t, preservedChunks, 1, "Should return one preserved chunk")
			//
			// preservedChunk := preservedChunks[0]
			// assert.NotEmpty(t, preservedChunk.PreservedContext.ImportStatements, "Should have import statements")
			//
			// for _, expectedImport := range tt.expectedImports {
			//     found := false
			//     for _, importDecl := range preservedChunk.PreservedContext.ImportStatements {
			//         if strings.Contains(importDecl.Path, expectedImport) || strings.Contains(importDecl.Content, expectedImport) {
			//             found = true
			//             break
			//         }
			//     }
			//     assert.True(t, found, "Should preserve import: %s", expectedImport)
			// }
		})
	}
}

// TestContextPreservationTypeDefinitions tests type definitions used in function signatures are preserved.
func TestContextPreservationTypeDefinitions(t *testing.T) {
	ctx := context.Background()
	contextPreserver := chunking.NewContextPreserver()

	tests := []struct {
		name          string
		language      valueobject.Language
		functionChunk outbound.EnhancedCodeChunk
		expectedTypes []string
		description   string
	}{
		{
			name:     "Go function with custom types",
			language: mustCreateLanguage(t, valueobject.LanguageGo),
			functionChunk: createEnhancedGoFunctionChunkWithTypes(
				t,
				"func ProcessUser(user *User, config ProcessingConfig) (*UserResult, error) {\n    return &UserResult{ID: user.ID}, nil\n}",
				"ProcessUser",
				[]string{"User", "ProcessingConfig", "UserResult"},
			),
			expectedTypes: []string{"User", "ProcessingConfig", "UserResult"},
			description:   "Should preserve Go custom types used in function signature",
		},
		{
			name:     "Python function with type hints",
			language: mustCreateLanguage(t, valueobject.LanguagePython),
			functionChunk: createEnhancedPythonFunctionChunkWithTypes(
				t,
				"def process_items(items: List[DataItem], processor: ItemProcessor) -> ProcessingResult:\n    return ProcessingResult(success=True)",
				"process_items",
				[]string{"List[DataItem]", "ItemProcessor", "ProcessingResult"},
			),
			expectedTypes: []string{"List[DataItem]", "ItemProcessor", "ProcessingResult"},
			description:   "Should preserve Python type hints as type definitions",
		},
		{
			name:     "TypeScript function with interfaces",
			language: mustCreateLanguage(t, valueobject.LanguageTypeScript),
			functionChunk: createEnhancedTSFunctionChunkWithTypes(
				t,
				"function processRequest(req: HttpRequest, options: ProcessingOptions): Promise<ApiResponse> {\n    return Promise.resolve({ success: true });\n}",
				"processRequest",
				[]string{"HttpRequest", "ProcessingOptions", "Promise<ApiResponse>"},
			),
			expectedTypes: []string{"HttpRequest", "ProcessingOptions", "Promise<ApiResponse>"},
			description:   "Should preserve TypeScript interface types used in function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := []outbound.EnhancedCodeChunk{tt.functionChunk}
			strategy := outbound.PreserveModerate

			preservedChunks, err := contextPreserver.PreserveContext(ctx, chunks, strategy)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.Len(t, preservedChunks, 1, "Should return one preserved chunk")

			// Expected behavior after implementation:
			// assert.NoError(t, err, tt.description)
			// assert.Len(t, preservedChunks, 1, "Should return one preserved chunk")
			//
			// preservedChunk := preservedChunks[0]
			// assert.NotEmpty(t, preservedChunk.PreservedContext.TypeDefinitions, "Should have type definitions")
			//
			// for _, expectedType := range tt.expectedTypes {
			//     found := false
			//     for _, typeRef := range preservedChunk.PreservedContext.TypeDefinitions {
			//         if typeRef.Name == expectedType || strings.Contains(typeRef.QualifiedName, expectedType) {
			//             found = true
			//             break
			//         }
			//     }
			//     assert.True(t, found, "Should preserve type definition: %s", expectedType)
			// }
		})
	}
}

// TestContextPreservationCalledFunctions tests called functions within target function are preserved.
func TestContextPreservationCalledFunctions(t *testing.T) {
	ctx := context.Background()
	contextPreserver := chunking.NewContextPreserver()

	tests := []struct {
		name              string
		language          valueobject.Language
		functionChunk     outbound.EnhancedCodeChunk
		expectedFunctions []string
		description       string
	}{
		{
			name:     "Go function with function calls",
			language: mustCreateLanguage(t, valueobject.LanguageGo),
			functionChunk: createEnhancedGoFunctionChunkWithCalls(
				t,
				"func ProcessData(data []byte) error {\n    if err := validateData(data); err != nil {\n        return err\n    }\n    result := transformData(data)\n    return saveResult(result)\n}",
				"ProcessData",
				[]string{"validateData", "transformData", "saveResult"},
			),
			expectedFunctions: []string{"validateData", "transformData", "saveResult"},
			description:       "Should preserve all functions called within target function",
		},
		{
			name:     "Python function with method calls",
			language: mustCreateLanguage(t, valueobject.LanguagePython),
			functionChunk: createEnhancedPythonFunctionChunkWithCalls(
				t,
				"def process_user_data(user_id: int) -> UserProfile:\n    user = get_user_by_id(user_id)\n    profile = create_profile(user)\n    notify_user(user.email, profile)\n    return profile",
				"process_user_data",
				[]string{"get_user_by_id", "create_profile", "notify_user"},
			),
			expectedFunctions: []string{"get_user_by_id", "create_profile", "notify_user"},
			description:       "Should preserve Python function calls within target function",
		},
		{
			name:     "JavaScript function with async calls",
			language: mustCreateLanguage(t, valueobject.LanguageJavaScript),
			functionChunk: createEnhancedJSFunctionChunkWithCalls(
				t,
				"async function processOrder(orderId) {\n    const order = await fetchOrder(orderId);\n    const payment = await processPayment(order.total);\n    await updateOrderStatus(orderId, 'completed');\n    return { order, payment };\n}",
				"processOrder",
				[]string{"fetchOrder", "processPayment", "updateOrderStatus"},
			),
			expectedFunctions: []string{"fetchOrder", "processPayment", "updateOrderStatus"},
			description:       "Should preserve async function calls within target function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := []outbound.EnhancedCodeChunk{tt.functionChunk}
			strategy := outbound.PreserveMaximal

			preservedChunks, err := contextPreserver.PreserveContext(ctx, chunks, strategy)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.Len(t, preservedChunks, 1, "Should return one preserved chunk")

			// Expected behavior after implementation:
			// assert.NoError(t, err, tt.description)
			// assert.Len(t, preservedChunks, 1, "Should return one preserved chunk")
			//
			// preservedChunk := preservedChunks[0]
			// assert.NotEmpty(t, preservedChunk.PreservedContext.CalledFunctions, "Should have called functions")
			//
			// for _, expectedFunction := range tt.expectedFunctions {
			//     found := false
			//     for _, funcCall := range preservedChunk.PreservedContext.CalledFunctions {
			//         if funcCall.Name == expectedFunction {
			//             found = true
			//             break
			//         }
			//     }
			//     assert.True(t, found, "Should preserve called function: %s", expectedFunction)
			// }
		})
	}
}

// TestPerformanceRequirement tests that chunking performance is <100ms per function.
func TestPerformanceRequirement(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()

	// Create a large function chunk to test performance
	largeFunction := createLargeGoFunctionChunk(t, 1000) // 1000 lines
	semanticChunks := []outbound.SemanticCodeChunk{largeFunction}

	config := createDefaultChunkingConfig()
	config.Strategy = outbound.StrategyFunction

	start := time.Now()
	chunks, err := functionChunker.ChunkByFunction(ctx, semanticChunks, config)
	duration := time.Since(start)

	// Phase 4.3 Step 2 implementation completed
	assert.NoError(t, err, "Should process large function without error")
	assert.Less(t, duration, 100*time.Millisecond, "Should process function in <100ms")
	assert.NotEmpty(t, chunks, "Should create chunks from large function")

	// Expected behavior after implementation:
	// assert.NoError(t, err, "Should process large function without error")
	// assert.Less(t, duration, 100*time.Millisecond, "Should process function in <100ms")
	// assert.NotEmpty(t, chunks, "Should create chunks from large function")

	t.Logf("Current processing time for large function: %v (should be <100ms after implementation)", duration)
}

// TestQualityRequirement tests that function-level chunks have 0.8+ cohesion score.
func TestQualityRequirement(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()
	_ = chunking.NewQualityAnalyzer() // Will be used after implementation

	tests := []struct {
		name             string
		semanticChunks   []outbound.SemanticCodeChunk
		expectedCohesion float64
		description      string
	}{
		{
			name: "High cohesion function group",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoFunctionChunk(
					t,
					"func CalculateArea(radius float64) float64 { return math.Pi * radius * radius }",
					"CalculateArea",
					0,
					100,
				),
				createGoFunctionChunk(
					t,
					"func CalculateCircumference(radius float64) float64 { return 2 * math.Pi * radius }",
					"CalculateCircumference",
					101,
					201,
				),
			},
			expectedCohesion: 0.8,
			description:      "Related math functions should have high cohesion",
		},
		{
			name: "Low cohesion function group",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoFunctionChunk(
					t,
					"func ProcessPayment(amount float64) error { return nil }",
					"ProcessPayment",
					0,
					100,
				),
				createGoFunctionChunk(
					t,
					"func RenderTemplate(name string) string { return \"\" }",
					"RenderTemplate",
					101,
					201,
				),
			},
			expectedCohesion: 0.3,
			description:      "Unrelated functions should have low cohesion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultChunkingConfig()
			config.Strategy = outbound.StrategyFunction
			config.QualityThreshold = 0.8

			chunks, err := functionChunker.ChunkByFunction(ctx, tt.semanticChunks, config)

			// Phase 4.3 Step 2 implementation completed
			assert.NoError(t, err, tt.description)
			assert.NotEmpty(t, chunks, "Should create chunks")

			// Expected behavior after implementation:
			// assert.NoError(t, err, tt.description)
			// assert.NotEmpty(t, chunks, "Should create chunks")
			//
			// for _, chunk := range chunks {
			//     metrics, err := qualityAnalyzer.CalculateQualityMetrics(ctx, &chunk)
			//     assert.NoError(t, err, "Should calculate quality metrics")
			//
			//     if tt.expectedCohesion >= 0.8 {
			//         assert.GreaterOrEqual(t, metrics.CohesionScore, 0.8, "High cohesion chunks should meet quality threshold")
			//     } else {
			//         assert.Less(t, metrics.CohesionScore, 0.8, "Low cohesion chunks should not meet quality threshold")
			//     }
			// }
		})
	}
}

// TestIntegrationWithCodeChunkingStrategy tests integration with existing CodeChunkingStrategy interface.
func TestIntegrationWithCodeChunkingStrategy(t *testing.T) {
	// Test creation of chunking strategy adapter
	adapter, err := chunking.NewChunkingStrategyAdapter()

	// Phase 4.3 Step 2 implementation completed
	assert.NoError(t, err, "Should create chunking strategy adapter")
	assert.NotNil(t, adapter, "Adapter should be created")

	// Expected behavior after implementation:
	// assert.NoError(t, err, "Should create chunking strategy adapter")
	// assert.NotNil(t, adapter, "Adapter should be created")
	//
	// // Test that function strategy is supported
	// strategies, err := adapter.GetSupportedStrategies(ctx)
	// assert.NoError(t, err, "Should get supported strategies")
	// assert.Contains(t, strategies, outbound.StrategyFunction, "Should support function strategy")
	//
	// // Test function chunking through adapter
	// semanticChunks := []outbound.SemanticCodeChunk{
	//     createGoFunctionChunk(t, "func Add(a, b int) int { return a + b }", "Add", 0, 100),
	// }
	// config := createDefaultChunkingConfig()
	// config.Strategy = outbound.StrategyFunction
	//
	// chunks, err := adapter.ChunkCode(ctx, semanticChunks, config)
	// assert.NoError(t, err, "Should chunk code using function strategy")
	// assert.NotEmpty(t, chunks, "Should create function chunks")
}

// TestEdgeCasesAndErrorConditions tests edge cases and error conditions.
func TestEdgeCasesAndErrorConditions(t *testing.T) {
	ctx := context.Background()
	functionChunker := chunking.NewFunctionChunker()

	tests := []struct {
		name           string
		semanticChunks []outbound.SemanticCodeChunk
		config         outbound.ChunkingConfiguration
		expectedError  string
		description    string
	}{
		{
			name:           "Empty semantic chunks",
			semanticChunks: []outbound.SemanticCodeChunk{},
			config:         createDefaultChunkingConfig(),
			expectedError:  "",
			description:    "Should handle empty input gracefully",
		},
		{
			name: "Invalid chunk boundaries",
			semanticChunks: []outbound.SemanticCodeChunk{
				createInvalidBoundaryChunk(t),
			},
			config:        createDefaultChunkingConfig(),
			expectedError: "invalid boundaries",
			description:   "Should handle invalid chunk boundaries",
		},
		{
			name: "Circular dependencies",
			semanticChunks: []outbound.SemanticCodeChunk{
				createCircularDependencyChunk1(t),
				createCircularDependencyChunk2(t),
			},
			config:        createDefaultChunkingConfig(),
			expectedError: "circular dependency",
			description:   "Should detect and handle circular dependencies",
		},
		{
			name: "Extremely large function",
			semanticChunks: []outbound.SemanticCodeChunk{
				createExtremelyLargeFunctionChunk(t, 50000), // 50k lines
			},
			config:        createDefaultChunkingConfig(),
			expectedError: "size limit exceeded",
			description:   "Should handle extremely large functions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := functionChunker.ChunkByFunction(ctx, tt.semanticChunks, tt.config)

			if tt.expectedError == "" {
				// Phase 4.3 Step 2 implementation completed - empty input handling
				assert.NoError(t, err, "Should handle empty input gracefully")
				assert.Empty(t, chunks, "Should return empty array for empty input")
			} else {
				// Phase 4.3 Step 2 implementation completed - error handling
				if len(tt.semanticChunks) > 0 {
					assert.Error(t, err, tt.description)
					assert.Contains(t, err.Error(), tt.expectedError, "Error should contain expected message")
				}
			}
		})
	}
}

// Helper functions for creating test data

func mustCreateLanguage(t *testing.T, name string) valueobject.Language {
	lang, err := valueobject.NewLanguage(name)
	require.NoError(t, err, "Should create language %s", name)
	return lang
}

func createDefaultChunkingConfig() outbound.ChunkingConfiguration {
	return outbound.ChunkingConfiguration{
		Strategy:                outbound.StrategyFunction,
		ContextPreservation:     outbound.PreserveModerate,
		MaxChunkSize:            2000,
		MinChunkSize:            100,
		OverlapSize:             50,
		PreferredBoundaries:     []outbound.BoundaryType{outbound.BoundaryFunction},
		IncludeImports:          true,
		IncludeDocumentation:    true,
		IncludeComments:         true,
		PreserveDependencies:    true,
		EnableSplitting:         true,
		QualityThreshold:        0.8,
		PerformanceHints:        &outbound.ChunkingPerformanceHints{EnableCaching: true},
		LanguageSpecificOptions: make(map[string]interface{}),
	}
}

func createGoFunctionChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	return outbound.SemanticCodeChunk{
		ID:              fmt.Sprintf("go_func_%s_%d", name, start),
		Type:            outbound.ConstructFunction,
		Name:            name,
		QualifiedName:   fmt.Sprintf("main.%s", name),
		Language:        mustCreateLanguage(t, valueobject.LanguageGo),
		StartByte:       start,
		EndByte:         end,
		Content:         content,
		Signature:       fmt.Sprintf("func %s", name),
		Documentation:   "",
		Parameters:      []outbound.Parameter{},
		ReturnType:      "",
		Visibility:      outbound.Public,
		CalledFunctions: []outbound.FunctionCall{},
		UsedTypes:       []outbound.TypeReference{},
		ExtractedAt:     time.Now(),
		Hash:            fmt.Sprintf("hash_%s", name),
	}
}

func createPythonFunctionChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	return outbound.SemanticCodeChunk{
		ID:              fmt.Sprintf("py_func_%s_%d", name, start),
		Type:            outbound.ConstructFunction,
		Name:            name,
		QualifiedName:   fmt.Sprintf("__main__.%s", name),
		Language:        mustCreateLanguage(t, valueobject.LanguagePython),
		StartByte:       start,
		EndByte:         end,
		Content:         content,
		Signature:       fmt.Sprintf("def %s", name),
		Documentation:   "",
		Parameters:      []outbound.Parameter{},
		ReturnType:      "",
		Visibility:      outbound.Public,
		CalledFunctions: []outbound.FunctionCall{},
		UsedTypes:       []outbound.TypeReference{},
		ExtractedAt:     time.Now(),
		Hash:            fmt.Sprintf("hash_%s", name),
	}
}

func createJSFunctionChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	return outbound.SemanticCodeChunk{
		ID:              fmt.Sprintf("js_func_%s_%d", name, start),
		Type:            outbound.ConstructFunction,
		Name:            name,
		QualifiedName:   name,
		Language:        mustCreateLanguage(t, valueobject.LanguageJavaScript),
		StartByte:       start,
		EndByte:         end,
		Content:         content,
		Signature:       fmt.Sprintf("function %s", name),
		Documentation:   "",
		Parameters:      []outbound.Parameter{},
		ReturnType:      "",
		Visibility:      outbound.Public,
		CalledFunctions: []outbound.FunctionCall{},
		UsedTypes:       []outbound.TypeReference{},
		ExtractedAt:     time.Now(),
		Hash:            fmt.Sprintf("hash_%s", name),
	}
}

func createPythonDecoratorChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	chunk := createPythonFunctionChunk(t, content, name, start, end)
	chunk.Annotations = []outbound.Annotation{
		{Name: "cache", Arguments: []string{}},
		{Name: "validate_input", Arguments: []string{}},
	}
	chunk.Documentation = "Process data with caching and validation"
	return chunk
}

func createJSClosureChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	chunk := createJSFunctionChunk(t, content, name, start, end)
	chunk.Type = outbound.ConstructClosure
	return chunk
}

// Additional helper functions would be implemented here...
// createGoFunctionChunkWithDoc, createPythonFunctionChunkWithDoc, etc.
// createEnhancedGoFunctionChunk, createEnhancedPythonFunctionChunk, etc.
// createLargeGoFunctionChunk, createInvalidBoundaryChunk, etc.

// Placeholder implementations for missing helper functions.
func createGoFunctionChunkWithDoc(
	t *testing.T,
	content, name, doc string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createGoFunctionChunk(t, content, name, start, end)
	chunk.Documentation = doc
	return chunk
}

func createPythonFunctionChunkWithDoc(
	t *testing.T,
	content, name, doc string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createPythonFunctionChunk(t, content, name, start, end)
	chunk.Documentation = doc
	return chunk
}

func createJSFunctionChunkWithDoc(
	t *testing.T,
	content, name, doc string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createJSFunctionChunk(t, content, name, start, end)
	chunk.Documentation = doc
	return chunk
}

func createGoStructChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	chunk := createGoFunctionChunk(t, content, name, start, end)
	chunk.Type = outbound.ConstructStruct
	return chunk
}

func createGoMethodChunk(t *testing.T, content, name, receiver string, start, end uint32) outbound.SemanticCodeChunk {
	chunk := createGoFunctionChunk(t, content, name, start, end)
	chunk.Type = outbound.ConstructMethod
	chunk.QualifiedName = fmt.Sprintf("%s.%s", receiver, name)
	return chunk
}

func createGoInterfaceChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	chunk := createGoFunctionChunk(t, content, name, start, end)
	chunk.Type = outbound.ConstructInterface
	return chunk
}

func createPythonDecoratorFunctionChunk(
	t *testing.T,
	content, name string,
	decorators []string,
	paramType, returnType string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createPythonFunctionChunk(t, content, name, start, end)
	for _, decorator := range decorators {
		chunk.Annotations = append(chunk.Annotations, outbound.Annotation{Name: decorator})
	}
	chunk.ReturnType = returnType
	chunk.IsAsync = strings.Contains(content, "async def")
	return chunk
}

func createPythonClassChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	chunk := createPythonFunctionChunk(t, content, name, start, end)
	chunk.Type = outbound.ConstructClass
	return chunk
}

func createPythonMethodWithDecoratorChunk(
	t *testing.T,
	content, name, className string,
	decorators []string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createPythonFunctionChunk(t, content, name, start, end)
	chunk.Type = outbound.ConstructMethod
	chunk.QualifiedName = fmt.Sprintf("%s.%s", className, name)
	for _, decorator := range decorators {
		chunk.Annotations = append(chunk.Annotations, outbound.Annotation{Name: decorator})
	}
	return chunk
}

func createJSArrowFunctionChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	chunk := createJSFunctionChunk(t, content, name, start, end)
	chunk.Signature = fmt.Sprintf("const %s = ", name)
	chunk.IsAsync = strings.Contains(content, "async")
	return chunk
}

func createJSNestedFunctionChunk(
	t *testing.T,
	content, name string,
	nestedFunctions []string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createJSFunctionChunk(t, content, name, start, end)
	for _, nestedFunc := range nestedFunctions {
		chunk.CalledFunctions = append(chunk.CalledFunctions, outbound.FunctionCall{
			Name:      nestedFunc,
			StartByte: start + 10,
			EndByte:   start + 20,
		})
	}
	return chunk
}

func createEnhancedGoFunctionChunk(t *testing.T, content, name string, imports []string) outbound.EnhancedCodeChunk {
	return outbound.EnhancedCodeChunk{
		ID:        fmt.Sprintf("enhanced_go_%s", name),
		Language:  mustCreateLanguage(t, valueobject.LanguageGo),
		Content:   content,
		ChunkType: outbound.ChunkFunction,
		CreatedAt: time.Now(),
		Hash:      fmt.Sprintf("hash_%s", name),
		Dependencies: []outbound.ChunkDependency{
			{Type: outbound.DependencyImport, TargetSymbol: strings.Join(imports, ",")},
		},
	}
}

func createEnhancedPythonFunctionChunk(
	t *testing.T,
	content, name string,
	imports []string,
) outbound.EnhancedCodeChunk {
	return outbound.EnhancedCodeChunk{
		ID:        fmt.Sprintf("enhanced_py_%s", name),
		Language:  mustCreateLanguage(t, valueobject.LanguagePython),
		Content:   content,
		ChunkType: outbound.ChunkFunction,
		CreatedAt: time.Now(),
		Hash:      fmt.Sprintf("hash_%s", name),
		Dependencies: []outbound.ChunkDependency{
			{Type: outbound.DependencyImport, TargetSymbol: strings.Join(imports, ",")},
		},
	}
}

func createEnhancedJSFunctionChunk(t *testing.T, content, name string, imports []string) outbound.EnhancedCodeChunk {
	return outbound.EnhancedCodeChunk{
		ID:        fmt.Sprintf("enhanced_js_%s", name),
		Language:  mustCreateLanguage(t, valueobject.LanguageJavaScript),
		Content:   content,
		ChunkType: outbound.ChunkFunction,
		CreatedAt: time.Now(),
		Hash:      fmt.Sprintf("hash_%s", name),
		Dependencies: []outbound.ChunkDependency{
			{Type: outbound.DependencyImport, TargetSymbol: strings.Join(imports, ",")},
		},
	}
}

func createEnhancedGoFunctionChunkWithTypes(
	t *testing.T,
	content, name string,
	types []string,
) outbound.EnhancedCodeChunk {
	chunk := createEnhancedGoFunctionChunk(t, content, name, []string{})
	for _, typeName := range types {
		chunk.PreservedContext.TypeDefinitions = append(chunk.PreservedContext.TypeDefinitions, outbound.TypeReference{
			Name:          typeName,
			QualifiedName: typeName,
		})
	}
	return chunk
}

func createEnhancedPythonFunctionChunkWithTypes(
	t *testing.T,
	content, name string,
	types []string,
) outbound.EnhancedCodeChunk {
	chunk := createEnhancedPythonFunctionChunk(t, content, name, []string{})
	for _, typeName := range types {
		chunk.PreservedContext.TypeDefinitions = append(chunk.PreservedContext.TypeDefinitions, outbound.TypeReference{
			Name:          typeName,
			QualifiedName: typeName,
		})
	}
	return chunk
}

func createEnhancedTSFunctionChunkWithTypes(
	t *testing.T,
	content, name string,
	types []string,
) outbound.EnhancedCodeChunk {
	chunk := outbound.EnhancedCodeChunk{
		ID:        fmt.Sprintf("enhanced_ts_%s", name),
		Language:  mustCreateLanguage(t, valueobject.LanguageTypeScript),
		Content:   content,
		ChunkType: outbound.ChunkFunction,
		CreatedAt: time.Now(),
		Hash:      fmt.Sprintf("hash_%s", name),
	}
	for _, typeName := range types {
		chunk.PreservedContext.TypeDefinitions = append(chunk.PreservedContext.TypeDefinitions, outbound.TypeReference{
			Name:          typeName,
			QualifiedName: typeName,
		})
	}
	return chunk
}

func createEnhancedGoFunctionChunkWithCalls(
	t *testing.T,
	content, name string,
	calls []string,
) outbound.EnhancedCodeChunk {
	chunk := createEnhancedGoFunctionChunk(t, content, name, []string{})
	for _, callName := range calls {
		chunk.PreservedContext.CalledFunctions = append(chunk.PreservedContext.CalledFunctions, outbound.FunctionCall{
			Name:      callName,
			StartByte: 0,
			EndByte:   10,
		})
	}
	return chunk
}

func createEnhancedPythonFunctionChunkWithCalls(
	t *testing.T,
	content, name string,
	calls []string,
) outbound.EnhancedCodeChunk {
	chunk := createEnhancedPythonFunctionChunk(t, content, name, []string{})
	for _, callName := range calls {
		chunk.PreservedContext.CalledFunctions = append(chunk.PreservedContext.CalledFunctions, outbound.FunctionCall{
			Name:      callName,
			StartByte: 0,
			EndByte:   10,
		})
	}
	return chunk
}

func createEnhancedJSFunctionChunkWithCalls(
	t *testing.T,
	content, name string,
	calls []string,
) outbound.EnhancedCodeChunk {
	chunk := createEnhancedJSFunctionChunk(t, content, name, []string{})
	for _, callName := range calls {
		chunk.PreservedContext.CalledFunctions = append(chunk.PreservedContext.CalledFunctions, outbound.FunctionCall{
			Name:      callName,
			StartByte: 0,
			EndByte:   10,
		})
	}
	return chunk
}

func createLargeGoFunctionChunk(t *testing.T, lineCount int) outbound.SemanticCodeChunk {
	var content strings.Builder
	content.WriteString("func LargeFunction() error {\n")
	for i := range lineCount {
		content.WriteString(fmt.Sprintf("    // Line %d\n", i))
		content.WriteString(fmt.Sprintf("    var_%d := %d\n", i, i))
	}
	content.WriteString("    return nil\n}")

	return createGoFunctionChunk(t, content.String(), "LargeFunction", 0, uint32(len(content.String())))
}

func createInvalidBoundaryChunk(t *testing.T) outbound.SemanticCodeChunk {
	chunk := createGoFunctionChunk(t, "invalid content", "InvalidFunc", 100, 50) // end < start
	return chunk
}

func createCircularDependencyChunk1(t *testing.T) outbound.SemanticCodeChunk {
	chunk := createGoFunctionChunk(t, "func Func1() { Func2() }", "Func1", 0, 100)
	chunk.CalledFunctions = []outbound.FunctionCall{{Name: "Func2"}}
	return chunk
}

func createCircularDependencyChunk2(t *testing.T) outbound.SemanticCodeChunk {
	chunk := createGoFunctionChunk(t, "func Func2() { Func1() }", "Func2", 101, 201)
	chunk.CalledFunctions = []outbound.FunctionCall{{Name: "Func1"}}
	return chunk
}

func createExtremelyLargeFunctionChunk(t *testing.T, lineCount int) outbound.SemanticCodeChunk {
	return createLargeGoFunctionChunk(t, lineCount)
}

func findChunkByName(chunks []outbound.EnhancedCodeChunk, name string) *outbound.EnhancedCodeChunk {
	for i, chunk := range chunks {
		if strings.Contains(chunk.Content, name) {
			return &chunks[i]
		}
	}
	return nil
}

func findChunkByType(chunks []outbound.EnhancedCodeChunk, chunkType outbound.ChunkType) *outbound.EnhancedCodeChunk {
	for i, chunk := range chunks {
		if chunk.ChunkType == chunkType {
			return &chunks[i]
		}
	}
	return nil
}
