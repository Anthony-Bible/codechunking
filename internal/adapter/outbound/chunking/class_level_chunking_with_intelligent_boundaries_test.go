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

// TestClassLevelChunkingWithIntelligentBoundaries tests Phase 4.3 Step 3:
// Class/Struct-Level Intelligent Boundaries for code chunking.
func TestClassLevelChunkingWithIntelligentBoundaries(t *testing.T) {
	// Create class chunker
	classChunker := chunking.NewClassChunker()

	require.NotNil(t, classChunker, "Class chunker should be created")
}

// TestGoStructWithMethodsAsUnifiedChunk tests that Go structs and their methods are grouped as unified chunks.
func TestGoStructWithMethodsAsUnifiedChunk(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

	tests := []struct {
		name               string
		semanticChunks     []outbound.SemanticCodeChunk
		expectedChunkCount int
		expectedIncludes   []string
		description        string
	}{
		{
			name: "Go struct with methods",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoStructChunk(t,
					"type Calculator struct {\n    precision int\n    history []Operation\n}",
					"Calculator", 0, 100),
				createGoMethodChunk(
					t,
					"func (c *Calculator) Add(a, b int) int {\n    result := a + b\n    c.history = append(c.history, Operation{Type: \"add\", Result: result})\n    return result\n}",
					"Add",
					"Calculator",
					101,
					250,
				),
				createGoMethodChunk(t,
					"func (c Calculator) GetHistory() []Operation {\n    return c.history\n}",
					"GetHistory", "Calculator", 251, 350),
				createGoMethodChunk(t,
					"func (c *Calculator) Clear() {\n    c.history = nil\n}",
					"Clear", "Calculator", 351, 400),
			},
			expectedChunkCount: 1,
			expectedIncludes:   []string{"Calculator", "Add", "GetHistory", "Clear", "precision", "history"},
			description:        "Go struct with all its methods should create one unified chunk",
		},
		{
			name: "Go interface with implementing struct",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoInterfaceChunk(t,
					"type Adder interface {\n    Add(int, int) int\n    GetResult() int\n}",
					"Adder", 0, 100),
				createGoStructChunk(t,
					"type SimpleAdder struct {\n    result int\n}",
					"SimpleAdder", 101, 150),
				createGoMethodChunk(t,
					"func (s *SimpleAdder) Add(a, b int) int {\n    s.result = a + b\n    return s.result\n}",
					"Add", "SimpleAdder", 151, 250),
				createGoMethodChunk(t,
					"func (s SimpleAdder) GetResult() int {\n    return s.result\n}",
					"GetResult", "SimpleAdder", 251, 300),
			},
			expectedChunkCount: 2, // Interface as one chunk, implementing struct as another
			expectedIncludes:   []string{"Adder", "SimpleAdder", "Add", "GetResult"},
			description:        "Interface and implementing struct should create separate but related chunks",
		},
		{
			name: "Go struct with embedded fields",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoStructChunk(t,
					"type BaseEntity struct {\n    ID string\n    CreatedAt time.Time\n}",
					"BaseEntity", 0, 100),
				createGoStructChunk(t,
					"type User struct {\n    BaseEntity\n    Name string\n    Email string\n}",
					"User", 101, 200),
				createGoMethodChunk(
					t,
					"func (u *User) Validate() error {\n    if u.Name == \"\" {\n        return errors.New(\"name required\")\n    }\n    return nil\n}",
					"Validate",
					"User",
					201,
					350,
				),
			},
			expectedChunkCount: 2, // BaseEntity and User as separate chunks
			expectedIncludes:   []string{"BaseEntity", "User", "Validate", "Name", "Email"},
			description:        "Embedded struct composition should create related chunks with dependency tracking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultClassChunkingConfig()
			config.Strategy = outbound.StrategyClass

			chunks, err := classChunker.ChunkByClass(ctx, tt.semanticChunks, config)

			// This test will fail initially - this is expected for red phase TDD
			require.NoError(t, err, tt.description)
			assert.Len(t, chunks, tt.expectedChunkCount, "Should create %d class chunks", tt.expectedChunkCount)

			for _, expectedInclude := range tt.expectedIncludes {
				found := false
				for _, chunk := range chunks {
					if strings.Contains(chunk.Content, expectedInclude) {
						found = true
						break
					}
				}
				assert.True(t, found, "Chunks should include %s", expectedInclude)
			}

			// Verify chunk types are class-related
			for _, chunk := range chunks {
				assert.True(t,
					chunk.ChunkType == outbound.ChunkClass ||
						chunk.ChunkType == outbound.ChunkStruct ||
						chunk.ChunkType == outbound.ChunkInterface,
					"Chunk type should be class, struct, or interface")
			}
		})
	}
}

// TestPythonClassWithMethodsAsUnifiedChunk tests Python class chunking with methods and inheritance.
func TestPythonClassWithMethodsAsUnifiedChunk(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

	tests := []struct {
		name               string
		semanticChunks     []outbound.SemanticCodeChunk
		expectedChunkCount int
		expectedIncludes   []string
		description        string
	}{
		{
			name: "Python class with methods and properties",
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonClassChunk(
					t,
					"class DataProcessor:\n    def __init__(self, config: ProcessingConfig):\n        self._config = config\n        self._results = []",
					"DataProcessor",
					0,
					150,
				),
				createPythonMethodChunk(t,
					"    @property\n    def results(self) -> List[ProcessingResult]:\n        return self._results",
					"results", "DataProcessor", []string{"@property"}, 151, 250),
				createPythonMethodChunk(
					t,
					"    def process_data(self, data: List[Dict]) -> ProcessingResult:\n        # Processing logic here\n        result = ProcessingResult(success=True, data=data)\n        self._results.append(result)\n        return result",
					"process_data",
					"DataProcessor",
					[]string{},
					251,
					450,
				),
				createPythonMethodChunk(t,
					"    def clear_results(self) -> None:\n        self._results.clear()",
					"clear_results", "DataProcessor", []string{}, 451, 500),
			},
			expectedChunkCount: 1,
			expectedIncludes: []string{
				"DataProcessor",
				"__init__",
				"results",
				"process_data",
				"clear_results",
				"@property",
			},
			description: "Python class with all its methods should create one unified chunk",
		},
		{
			name: "Python class inheritance hierarchy",
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonClassChunk(
					t,
					"class Animal:\n    def __init__(self, name: str):\n        self.name = name\n    \n    def speak(self) -> str:\n        raise NotImplementedError",
					"Animal",
					0,
					150,
				),
				createPythonClassChunk(
					t,
					"class Dog(Animal):\n    def __init__(self, name: str, breed: str):\n        super().__init__(name)\n        self.breed = breed",
					"Dog",
					151,
					300,
				),
				createPythonMethodChunk(t,
					"    def speak(self) -> str:\n        return f\"{self.name} barks!\"",
					"speak", "Dog", []string{}, 301, 350),
				createPythonMethodChunk(t,
					"    def get_breed(self) -> str:\n        return self.breed",
					"get_breed", "Dog", []string{}, 351, 400),
			},
			expectedChunkCount: 2, // Animal and Dog as separate chunks
			expectedIncludes:   []string{"Animal", "Dog", "speak", "get_breed", "super().__init__"},
			description:        "Python inheritance should create separate chunks with inheritance relationship tracked",
		},
		{
			name: "Python class with decorators and nested classes",
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonClassWithDecoratorsChunk(t,
					"@dataclass\n@total_ordering\nclass Product:\n    id: str\n    name: str\n    price: Decimal",
					"Product", []string{"@dataclass", "@total_ordering"}, 0, 150),
				createPythonClassChunk(t,
					"    class Category:\n        def __init__(self, name: str):\n            self.name = name",
					"Category", 151, 250),
				createPythonMethodChunk(t,
					"    def __lt__(self, other: 'Product') -> bool:\n        return self.price < other.price",
					"__lt__", "Product", []string{}, 251, 350),
			},
			expectedChunkCount: 1, // Nested class should be included in parent class chunk
			expectedIncludes:   []string{"Product", "Category", "__lt__", "@dataclass", "@total_ordering"},
			description:        "Python class with decorators and nested classes should create unified chunk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultClassChunkingConfig()
			config.Strategy = outbound.StrategyClass

			chunks, err := classChunker.ChunkByClass(ctx, tt.semanticChunks, config)

			// This test will fail initially - this is expected for red phase TDD
			require.NoError(t, err, tt.description)
			assert.Len(t, chunks, tt.expectedChunkCount, "Should create %d class chunks", tt.expectedChunkCount)

			for _, expectedInclude := range tt.expectedIncludes {
				found := false
				for _, chunk := range chunks {
					if strings.Contains(chunk.Content, expectedInclude) {
						found = true
						break
					}
				}
				assert.True(t, found, "Chunks should include %s", expectedInclude)
			}

			// Verify chunk types are class-related
			for _, chunk := range chunks {
				assert.Equal(t, outbound.ChunkClass, chunk.ChunkType, "Chunk type should be class")
			}
		})
	}
}

// TestJavaScriptClassWithMethodsAsUnifiedChunk tests JavaScript/TypeScript class chunking.
func TestJavaScriptClassWithMethodsAsUnifiedChunk(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

	tests := []struct {
		name               string
		language           valueobject.Language
		semanticChunks     []outbound.SemanticCodeChunk
		expectedChunkCount int
		expectedIncludes   []string
		description        string
	}{
		{
			name:     "JavaScript ES6 class with methods",
			language: mustCreateLanguage(t, valueobject.LanguageJavaScript),
			semanticChunks: []outbound.SemanticCodeChunk{
				createJSClassChunk(t,
					"class EventManager {\n    constructor() {\n        this.listeners = new Map();\n    }",
					"EventManager", valueobject.LanguageJavaScript, 0, 100),
				createJSMethodChunk(
					t,
					"    on(event, callback) {\n        if (!this.listeners.has(event)) {\n            this.listeners.set(event, []);\n        }\n        this.listeners.get(event).push(callback);\n    }",
					"on",
					"EventManager",
					valueobject.LanguageJavaScript,
					101,
					250,
				),
				createJSMethodChunk(
					t,
					"    emit(event, ...args) {\n        const callbacks = this.listeners.get(event);\n        if (callbacks) {\n            callbacks.forEach(cb => cb(...args));\n        }\n    }",
					"emit",
					"EventManager",
					valueobject.LanguageJavaScript,
					251,
					400,
				),
			},
			expectedChunkCount: 1,
			expectedIncludes:   []string{"EventManager", "constructor", "on", "emit", "listeners"},
			description:        "JavaScript class with all its methods should create one unified chunk",
		},
		{
			name:     "TypeScript class with inheritance and interfaces",
			language: mustCreateLanguage(t, valueobject.LanguageTypeScript),
			semanticChunks: []outbound.SemanticCodeChunk{
				createTSInterfaceChunk(t,
					"interface Drawable {\n    draw(): void;\n    getArea(): number;\n}",
					"Drawable", 0, 100),
				createTSClassChunk(
					t,
					"abstract class Shape implements Drawable {\n    protected x: number;\n    protected y: number;\n    \n    constructor(x: number, y: number) {\n        this.x = x;\n        this.y = y;\n    }",
					"Shape",
					[]string{"Drawable"},
					101,
					300,
				),
				createTSMethodChunk(t,
					"    abstract getArea(): number;",
					"getArea", "Shape", true, 301, 350),
				createTSClassChunk(
					t,
					"class Circle extends Shape {\n    private radius: number;\n    \n    constructor(x: number, y: number, radius: number) {\n        super(x, y);\n        this.radius = radius;\n    }",
					"Circle",
					[]string{"Shape"},
					351,
					500,
				),
				createTSMethodChunk(
					t,
					"    draw(): void {\n        console.log(`Drawing circle at (${this.x}, ${this.y}) with radius ${this.radius}`);\n    }",
					"draw",
					"Circle",
					false,
					501,
					600,
				),
				createTSMethodChunk(t,
					"    getArea(): number {\n        return Math.PI * this.radius * this.radius;\n    }",
					"getArea", "Circle", false, 601, 700),
			},
			expectedChunkCount: 3, // Interface, abstract class, and concrete class as separate chunks
			expectedIncludes:   []string{"Drawable", "Shape", "Circle", "draw", "getArea", "implements", "extends"},
			description:        "TypeScript inheritance hierarchy should create separate chunks with relationship tracking",
		},
		{
			name:     "JavaScript class with static methods and getters",
			language: mustCreateLanguage(t, valueobject.LanguageJavaScript),
			semanticChunks: []outbound.SemanticCodeChunk{
				createJSClassChunk(
					t,
					"class MathUtils {\n    static PI = 3.14159;\n    \n    constructor(precision = 2) {\n        this.precision = precision;\n    }",
					"MathUtils",
					valueobject.LanguageJavaScript,
					0,
					150,
				),
				createJSStaticMethodChunk(t,
					"    static circleArea(radius) {\n        return MathUtils.PI * radius * radius;\n    }",
					"circleArea", "MathUtils", valueobject.LanguageJavaScript, 151, 250),
				createJSGetterMethodChunk(t,
					"    get formattedPI() {\n        return MathUtils.PI.toFixed(this.precision);\n    }",
					"formattedPI", "MathUtils", valueobject.LanguageJavaScript, 251, 350),
			},
			expectedChunkCount: 1,
			expectedIncludes:   []string{"MathUtils", "PI", "circleArea", "formattedPI", "static", "get"},
			description:        "JavaScript class with static methods and getters should create unified chunk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultClassChunkingConfig()
			config.Strategy = outbound.StrategyClass

			chunks, err := classChunker.ChunkByClass(ctx, tt.semanticChunks, config)

			// This test will fail initially - this is expected for red phase TDD
			require.NoError(t, err, tt.description)
			assert.Len(t, chunks, tt.expectedChunkCount, "Should create %d class chunks", tt.expectedChunkCount)

			for _, expectedInclude := range tt.expectedIncludes {
				found := false
				for _, chunk := range chunks {
					if strings.Contains(chunk.Content, expectedInclude) {
						found = true
						break
					}
				}
				assert.True(t, found, "Chunks should include %s", expectedInclude)
			}

			// Verify chunk types are class-related
			for _, chunk := range chunks {
				assert.True(t,
					chunk.ChunkType == outbound.ChunkClass ||
						chunk.ChunkType == outbound.ChunkInterface,
					"Chunk type should be class or interface")
			}
		})
	}
}

// TestIntelligentBoundaryDetection tests intelligent splitting of oversized classes while preserving semantic integrity.
func TestIntelligentBoundaryDetection(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

	tests := []struct {
		name                string
		semanticChunks      []outbound.SemanticCodeChunk
		maxChunkSize        int
		expectedChunkCount  int
		expectedSplitPoints []string
		description         string
	}{
		{
			name: "Large Go struct split by logical groupings",
			semanticChunks: []outbound.SemanticCodeChunk{
				createLargeGoStructChunk(t, "UserManager", 3000), // 3000 characters
				createGoMethodGroupChunk(
					t,
					"UserManager",
					"authentication",
					[]string{"Login", "Logout", "ValidateToken"},
				),
				createGoMethodGroupChunk(
					t,
					"UserManager",
					"user_management",
					[]string{"CreateUser", "UpdateUser", "DeleteUser"},
				),
				createGoMethodGroupChunk(
					t,
					"UserManager",
					"permissions",
					[]string{"GrantPermission", "RevokePermission", "CheckAccess"},
				),
			},
			maxChunkSize:        1500,
			expectedChunkCount:  3, // Split into logical method groups
			expectedSplitPoints: []string{"authentication", "user_management", "permissions"},
			description:         "Large struct should split by logical method groupings while preserving semantic integrity",
		},
		{
			name: "Large Python class split by functionality",
			semanticChunks: []outbound.SemanticCodeChunk{
				createLargePythonClassChunk(t, "APIHandler", 2500),
				createPythonMethodGroupChunk(
					t,
					"APIHandler",
					"request_handling",
					[]string{"handle_get", "handle_post", "handle_put"},
				),
				createPythonMethodGroupChunk(
					t,
					"APIHandler",
					"validation",
					[]string{"validate_input", "validate_auth", "validate_permissions"},
				),
				createPythonMethodGroupChunk(
					t,
					"APIHandler",
					"response_formatting",
					[]string{"format_success", "format_error", "format_json"},
				),
			},
			maxChunkSize:        1200,
			expectedChunkCount:  3, // Split by functionality groups
			expectedSplitPoints: []string{"request_handling", "validation", "response_formatting"},
			description:         "Large Python class should split by functional groups maintaining class structure",
		},
		{
			name: "TypeScript class too small to split",
			semanticChunks: []outbound.SemanticCodeChunk{
				createTSClassChunk(
					t,
					"class SimpleCalculator {\n    add(a: number, b: number): number { return a + b; }\n    subtract(a: number, b: number): number { return a - b; }\n}",
					"SimpleCalculator",
					[]string{},
					0,
					200,
				),
			},
			maxChunkSize:        1500,
			expectedChunkCount:  1, // Should not split
			expectedSplitPoints: []string{},
			description:         "Small class should remain as single chunk even with splitting enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultClassChunkingConfig()
			config.Strategy = outbound.StrategyClass
			config.MaxChunkSize = tt.maxChunkSize
			config.EnableSplitting = true

			chunks, err := classChunker.ChunkByClass(ctx, tt.semanticChunks, config)

			// This test will fail initially - this is expected for red phase TDD
			require.NoError(t, err, tt.description)
			assert.Len(
				t,
				chunks,
				tt.expectedChunkCount,
				"Should create %d chunks after intelligent splitting",
				tt.expectedChunkCount,
			)

			if tt.expectedChunkCount > 1 {
				// Verify split points are semantically meaningful
				verifySplitPointsPreserved(t, chunks, tt.expectedSplitPoints)

				// Verify each chunk respects size limits
				verifyChunkSizeLimits(t, chunks, tt.maxChunkSize)
			}

			// Verify semantic integrity - all chunks should be valid class constructs
			verifySemanticIntegrity(t, chunks)
		})
	}
}

// TestInheritanceRelationshipPreservation tests that inheritance relationships and dependencies are preserved.
func TestInheritanceRelationshipPreservation(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

	tests := []struct {
		name                  string
		semanticChunks        []outbound.SemanticCodeChunk
		expectedDependencies  []string
		expectedRelationships []string
		description           string
	}{
		{
			name: "Go interface implementation tracking",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoInterfaceChunk(
					t,
					"type Writer interface {\n    Write([]byte) (int, error)\n}",
					"Writer",
					0,
					100,
				),
				createGoInterfaceChunk(
					t,
					"type Reader interface {\n    Read([]byte) (int, error)\n}",
					"Reader",
					101,
					200,
				),
				createGoInterfaceChunk(
					t,
					"type ReadWriter interface {\n    Reader\n    Writer\n}",
					"ReadWriter",
					201,
					300,
				),
				createGoStructChunk(
					t,
					"type BufferedReadWriter struct {\n    buffer []byte\n}",
					"BufferedReadWriter",
					301,
					400,
				),
			},
			expectedDependencies:  []string{"Reader", "Writer"},
			expectedRelationships: []string{"implements", "embeds", "composes"},
			description:           "Interface embedding and implementation should be tracked as dependencies",
		},
		{
			name: "Python class inheritance chain",
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonClassChunk(t, "class Animal:\n    def speak(self): pass", "Animal", 0, 100),
				createPythonClassChunk(t, "class Mammal(Animal):\n    def breathe(self): pass", "Mammal", 101, 200),
				createPythonClassChunk(t, "class Dog(Mammal):\n    def bark(self): pass", "Dog", 201, 300),
				createPythonClassWithMultipleInheritanceChunk(t,
					"class ServiceDog(Dog, Service):\n    def assist(self): pass",
					"ServiceDog", []string{"Dog", "Service"}, 301, 400),
			},
			expectedDependencies:  []string{"Animal", "Mammal", "Dog", "Service"},
			expectedRelationships: []string{"inherits", "extends", "multiple_inheritance"},
			description:           "Python inheritance chains and multiple inheritance should be tracked",
		},
		{
			name: "TypeScript interface and class relationships",
			semanticChunks: []outbound.SemanticCodeChunk{
				createTSInterfaceChunk(
					t,
					"interface Serializable {\n    serialize(): string;\n}",
					"Serializable",
					0,
					100,
				),
				createTSInterfaceChunk(
					t,
					"interface Validatable {\n    validate(): boolean;\n}",
					"Validatable",
					101,
					200,
				),
				createTSClassChunk(t,
					"class Entity implements Serializable, Validatable {\n    id: string;\n}",
					"Entity", []string{"Serializable", "Validatable"}, 201, 350),
				createTSClassChunk(t,
					"class User extends Entity {\n    name: string;\n}",
					"User", []string{"Entity"}, 351, 450),
			},
			expectedDependencies:  []string{"Serializable", "Validatable", "Entity"},
			expectedRelationships: []string{"implements", "extends", "interface_contract"},
			description:           "TypeScript interface implementations and class inheritance should be tracked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultClassChunkingConfig()
			config.Strategy = outbound.StrategyClass
			config.PreserveDependencies = true

			chunks, err := classChunker.ChunkByClass(ctx, tt.semanticChunks, config)

			// This test will fail initially - this is expected for red phase TDD
			require.NoError(t, err, tt.description)
			assert.NotEmpty(t, chunks, "Should create chunks")

			// Verify dependencies are tracked
			verifyDependenciesTracked(t, chunks, tt.expectedDependencies)

			// Verify relationship types are captured
			verifyRelationshipTypes(t, chunks, tt.expectedRelationships)

			// Verify chunks have proper parent-child relationships
			verifyChunkRelationships(t, chunks)
		})
	}
}

// TestContextPreservationForClasses tests that import statements and type definitions are preserved for class chunks.
func TestContextPreservationForClasses(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

	tests := []struct {
		name                  string
		semanticChunks        []outbound.SemanticCodeChunk
		expectedImports       []string
		expectedTypes         []string
		expectedDocumentation bool
		description           string
	}{
		{
			name: "Go struct with import dependencies",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoStructWithImportsChunk(
					t,
					"type HTTPServer struct {\n    server *http.Server\n    logger *log.Logger\n    config *ServerConfig\n}",
					"HTTPServer",
					[]string{"net/http", "log"},
					[]string{"ServerConfig"},
					0,
					200,
				),
			},
			expectedImports:       []string{"net/http", "log"},
			expectedTypes:         []string{"ServerConfig"},
			expectedDocumentation: false,
			description:           "Go struct should preserve import dependencies and custom types",
		},
		{
			name: "Python class with imports and documentation",
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonClassWithImportsAndDocsChunk(
					t,
					"class DataAnalyzer:\n    \"\"\"\n    A comprehensive data analysis tool.\n    \n    This class provides methods for statistical analysis,\n    data visualization, and report generation.\n    \"\"\"\n    def __init__(self, data: pd.DataFrame):\n        self.data = data\n        self.stats = Statistics()\n",
					"DataAnalyzer",
					[]string{"pandas as pd", "numpy as np", "matplotlib.pyplot as plt"},
					[]string{"Statistics", "DataFrame"},
					"A comprehensive data analysis tool.\n\nThis class provides methods for statistical analysis,\ndata visualization, and report generation.",
					0,
					400,
				),
			},
			expectedImports:       []string{"pandas as pd", "numpy as np", "matplotlib.pyplot as plt"},
			expectedTypes:         []string{"Statistics", "DataFrame"},
			expectedDocumentation: true,
			description:           "Python class should preserve imports, type hints, and comprehensive docstrings",
		},
		{
			name: "TypeScript class with complex type dependencies",
			semanticChunks: []outbound.SemanticCodeChunk{
				createTSClassWithImportsChunk(
					t,
					"class APIClient<T extends ApiResource> implements HttpClient {\n    private baseURL: string;\n    private httpClient: AxiosInstance;\n    private serializer: JsonSerializer<T>;\n    \n    constructor(\n        baseURL: string,\n        config: ClientConfiguration,\n        serializer: JsonSerializer<T>\n    ) {\n        this.baseURL = baseURL;\n        this.httpClient = axios.create(config);\n        this.serializer = serializer;\n    }\n}",
					"APIClient",
					[]string{"axios", "./types/ApiResource", "./interfaces/HttpClient", "./utils/JsonSerializer"},
					[]string{"ApiResource", "HttpClient", "AxiosInstance", "JsonSerializer", "ClientConfiguration"},
					0,
					600,
				),
			},
			expectedImports: []string{"axios", "ApiResource", "HttpClient", "JsonSerializer"},
			expectedTypes: []string{
				"ApiResource",
				"HttpClient",
				"AxiosInstance",
				"JsonSerializer",
				"ClientConfiguration",
			},
			expectedDocumentation: false,
			description:           "TypeScript generic class should preserve complex import and type dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultClassChunkingConfig()
			config.Strategy = outbound.StrategyClass
			config.IncludeImports = true
			config.IncludeDocumentation = tt.expectedDocumentation
			config.PreserveDependencies = true

			chunks, err := classChunker.ChunkByClass(ctx, tt.semanticChunks, config)

			// This test will fail initially - this is expected for red phase TDD
			require.NoError(t, err, tt.description)
			assert.NotEmpty(t, chunks, "Should create chunks")

			chunk := chunks[0] // Focus on first chunk for context verification

			// Verify imports are preserved
			verifyImportsPreserved(t, chunk, tt.expectedImports)

			// Verify type definitions are preserved
			verifyTypesPreserved(t, chunk, tt.expectedTypes)

			// Verify documentation preservation
			verifyDocumentationPreserved(t, chunk, tt.expectedDocumentation)
		})
	}
}

// TestPerformanceRequirementsForClassChunking tests that class chunking meets performance requirements.
func TestPerformanceRequirementsForClassChunking(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

	// Create complex class hierarchy for performance testing
	largeClassHierarchy := createComplexClassHierarchy(t, 50) // 50 classes with methods

	config := createDefaultClassChunkingConfig()
	config.Strategy = outbound.StrategyClass

	start := time.Now()
	chunks, err := classChunker.ChunkByClass(ctx, largeClassHierarchy, config)
	duration := time.Since(start)

	// This test will fail initially - this is expected for red phase TDD
	require.NoError(t, err, "Should process complex class hierarchy without error")
	assert.Less(t, duration, 200*time.Millisecond, "Should process complex hierarchy in <200ms")
	assert.NotEmpty(t, chunks, "Should create chunks from complex hierarchy")

	t.Logf("Current processing time for complex class hierarchy: %v (should be <200ms after implementation)", duration)
}

// TestQualityRequirementsForClassChunks tests that class-level chunks meet quality requirements.
func TestQualityRequirementsForClassChunks(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

	tests := []struct {
		name             string
		semanticChunks   []outbound.SemanticCodeChunk
		expectedCohesion float64
		description      string
	}{
		{
			name: "High cohesion class with related methods",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoStructChunk(t, "type MathCalculator struct {}", "MathCalculator", 0, 50),
				createGoMethodChunk(t, "func (m *MathCalculator) Add(a, b float64) float64 { return a + b }",
					"Add", "MathCalculator", 51, 150),
				createGoMethodChunk(t, "func (m *MathCalculator) Subtract(a, b float64) float64 { return a - b }",
					"Subtract", "MathCalculator", 151, 250),
				createGoMethodChunk(t, "func (m *MathCalculator) Multiply(a, b float64) float64 { return a * b }",
					"Multiply", "MathCalculator", 251, 350),
			},
			expectedCohesion: 0.9,
			description:      "Related mathematical operations should have high cohesion",
		},
		{
			name: "Low cohesion class with unrelated methods",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoStructChunk(t, "type MixedUtility struct {}", "MixedUtility", 0, 50),
				createGoMethodChunk(t, "func (m *MixedUtility) ProcessPayment(amount float64) error { return nil }",
					"ProcessPayment", "MixedUtility", 51, 150),
				createGoMethodChunk(t, "func (m *MixedUtility) RenderTemplate(name string) string { return \"\" }",
					"RenderTemplate", "MixedUtility", 151, 250),
				createGoMethodChunk(t, "func (m *MixedUtility) SendEmail(to string) error { return nil }",
					"SendEmail", "MixedUtility", 251, 350),
			},
			expectedCohesion: 0.3,
			description:      "Unrelated methods should have low cohesion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createDefaultClassChunkingConfig()
			config.Strategy = outbound.StrategyClass
			config.QualityThreshold = 0.8

			chunks, err := classChunker.ChunkByClass(ctx, tt.semanticChunks, config)

			// This test will fail initially - this is expected for red phase TDD
			require.NoError(t, err, tt.description)
			assert.NotEmpty(t, chunks, "Should create chunks")

			// Expected behavior after implementation:
			// for _, chunk := range chunks {
			//     assert.GreaterOrEqual(t, chunk.CohesionScore, tt.expectedCohesion,
			//         "Class chunk should meet expected cohesion score")
			//     if tt.expectedCohesion >= 0.8 {
			//         assert.GreaterOrEqual(t, chunk.QualityMetrics.OverallQuality, 0.8,
			//             "High cohesion class should meet quality threshold")
			//     }
			// }
		})
	}
}

// TestIntegrationWithCodeChunkingStrategyForClasses tests integration with existing CodeChunkingStrategy interface.
func TestIntegrationWithCodeChunkingStrategyForClasses(t *testing.T) {
	ctx := context.Background()

	// Test creation of chunking strategy adapter
	adapter, err := chunking.NewChunkingStrategyAdapter()

	// This test will fail initially - this is expected for red phase TDD
	require.NoError(t, err, "Should create chunking strategy adapter")
	assert.NotNil(t, adapter, "Adapter should be created")

	// Test that class strategy is supported
	strategies, err := adapter.GetSupportedStrategies(ctx)
	require.NoError(t, err, "Should get supported strategies")
	assert.Contains(t, strategies, outbound.StrategyClass, "Should support class strategy")

	// Test class chunking through adapter
	semanticChunks := []outbound.SemanticCodeChunk{
		createGoStructChunk(t, "type User struct { ID int }", "User", 0, 100),
		createGoMethodChunk(t, "func (u User) String() string { return \"\" }", "String", "User", 101, 200),
	}
	config := createDefaultClassChunkingConfig()
	config.Strategy = outbound.StrategyClass

	chunks, err := adapter.ChunkCode(ctx, semanticChunks, config)
	require.NoError(t, err, "Should chunk code using class strategy")
	assert.NotEmpty(t, chunks, "Should create class chunks")
}

// TestEdgeCasesAndErrorConditionsForClasses tests edge cases and error conditions for class chunking.
func TestEdgeCasesAndErrorConditionsForClasses(t *testing.T) {
	ctx := context.Background()
	classChunker := chunking.NewClassChunker()

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
			config:         createDefaultClassChunkingConfig(),
			expectedError:  "",
			description:    "Should handle empty input gracefully",
		},
		{
			name: "Class with circular inheritance",
			semanticChunks: []outbound.SemanticCodeChunk{
				createPythonClassChunk(t, "class A(B): pass", "A", 0, 50),
				createPythonClassChunk(t, "class B(A): pass", "B", 51, 100),
			},
			config:        createDefaultClassChunkingConfig(),
			expectedError: "circular inheritance",
			description:   "Should detect and handle circular inheritance",
		},
		{
			name: "Extremely large class beyond limits",
			semanticChunks: []outbound.SemanticCodeChunk{
				createExtremelyLargeClassChunk(t, "MegaClass", 100000), // 100k characters
			},
			config:        createDefaultClassChunkingConfig(),
			expectedError: "class size exceeds maximum",
			description:   "Should handle extremely large classes gracefully",
		},
		{
			name: "Missing class dependencies",
			semanticChunks: []outbound.SemanticCodeChunk{
				createGoStructChunk(t, "type User struct { Profile *Profile }", "User", 0, 100),
				// Profile struct is missing - should be detected as broken dependency
			},
			config:        createDefaultClassChunkingConfig(),
			expectedError: "unresolved dependency",
			description:   "Should detect missing class dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := classChunker.ChunkByClass(ctx, tt.semanticChunks, tt.config)

			if tt.expectedError == "" {
				// This test will fail initially - this is expected for red phase TDD
				require.NoError(t, err, "Should handle empty input gracefully")
				assert.Empty(t, chunks, "Should return empty array for empty input")
			} else if len(tt.semanticChunks) > 0 {
				// This test will fail initially - this is expected for red phase TDD
				require.Error(t, err, tt.description)
				assert.Contains(t, err.Error(), tt.expectedError, "Error should contain expected message")
			}
		})
	}
}

// Helper functions for test verification

func verifyDependenciesTracked(t *testing.T, chunks []outbound.EnhancedCodeChunk, expectedDeps []string) {
	for _, expectedDep := range expectedDeps {
		found := false
		for _, chunk := range chunks {
			for _, dep := range chunk.Dependencies {
				if strings.Contains(dep.TargetSymbol, expectedDep) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		assert.True(t, found, "Should track dependency: %s", expectedDep)
	}
}

func verifyRelationshipTypes(t *testing.T, chunks []outbound.EnhancedCodeChunk, expectedRels []string) {
	for _, expectedRel := range expectedRels {
		found := false
		for _, chunk := range chunks {
			for _, dep := range chunk.Dependencies {
				if strings.Contains(dep.Relationship, expectedRel) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		assert.True(t, found, "Should capture relationship type: %s", expectedRel)
	}
}

func verifyChunkRelationships(t *testing.T, chunks []outbound.EnhancedCodeChunk) {
	for _, chunk := range chunks {
		if len(chunk.Dependencies) > 0 {
			assert.NotEmpty(t, chunk.RelatedChunks, "Dependent chunks should have related chunk references")
		}
	}
}

func verifyImportsPreserved(t *testing.T, chunk outbound.EnhancedCodeChunk, expectedImports []string) {
	for _, expectedImport := range expectedImports {
		found := false
		for _, importDecl := range chunk.PreservedContext.ImportStatements {
			if strings.Contains(importDecl.Path, expectedImport) ||
				strings.Contains(importDecl.Content, expectedImport) {
				found = true
				break
			}
		}
		assert.True(t, found, "Should preserve import: %s", expectedImport)
	}
}

func verifyTypesPreserved(t *testing.T, chunk outbound.EnhancedCodeChunk, expectedTypes []string) {
	for _, expectedType := range expectedTypes {
		found := false
		for _, typeRef := range chunk.PreservedContext.TypeDefinitions {
			if typeRef.Name == expectedType || strings.Contains(typeRef.QualifiedName, expectedType) {
				found = true
				break
			}
		}
		assert.True(t, found, "Should preserve type definition: %s", expectedType)
	}
}

func verifyDocumentationPreserved(t *testing.T, chunk outbound.EnhancedCodeChunk, expectDocs bool) {
	if expectDocs {
		assert.NotEmpty(t, chunk.Content, "Should preserve documentation in content")
		// Should contain docstring patterns
		assert.True(t,
			strings.Contains(chunk.Content, "\"\"\"") ||
				strings.Contains(chunk.Content, "/**") ||
				strings.Contains(chunk.Content, "//"),
			"Should preserve documentation markers")
	}
}

func verifySplitPointsPreserved(t *testing.T, chunks []outbound.EnhancedCodeChunk, expectedSplitPoints []string) {
	for _, splitPoint := range expectedSplitPoints {
		found := false
		for _, chunk := range chunks {
			if strings.Contains(chunk.Content, splitPoint) {
				found = true
				break
			}
		}
		assert.True(t, found, "Split should preserve semantic group: %s", splitPoint)
	}
}

func verifyChunkSizeLimits(t *testing.T, chunks []outbound.EnhancedCodeChunk, maxSize int) {
	for i, chunk := range chunks {
		assert.LessOrEqual(t, chunk.Size.Bytes, maxSize,
			"Chunk %d should respect max size limit", i)
	}
}

func verifySemanticIntegrity(t *testing.T, chunks []outbound.EnhancedCodeChunk) {
	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk.Content, "Split chunk should have content")
		assert.True(t, chunk.ChunkType == outbound.ChunkClass || chunk.ChunkType == outbound.ChunkStruct,
			"Split chunk should maintain class/struct type")
	}
}

// Helper functions for creating test data
// Note: Basic helper functions (mustCreateLanguage, createGoStructChunk, etc.)
// are defined in function_level_chunking_with_context_test.go

func createDefaultClassChunkingConfig() outbound.ChunkingConfiguration {
	return outbound.ChunkingConfiguration{
		Strategy:                outbound.StrategyClass,
		ContextPreservation:     outbound.PreserveModerate,
		MaxChunkSize:            3000,
		MinChunkSize:            200,
		OverlapSize:             100,
		PreferredBoundaries:     []outbound.BoundaryType{outbound.BoundaryClass, outbound.BoundaryInterface},
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

// Placeholder implementations for helper functions that create test chunks
// These would need to be implemented to support the actual test execution

func createPythonMethodChunk(
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

func createPythonClassWithDecoratorsChunk(
	t *testing.T,
	content, name string,
	decorators []string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createPythonClassChunk(t, content, name, start, end)
	for _, decorator := range decorators {
		chunk.Annotations = append(chunk.Annotations, outbound.Annotation{Name: decorator})
	}
	return chunk
}

func createJSClassChunk(
	t *testing.T,
	content, name string,
	language string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	return outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("js_class_%s_%d", name, start),
		Type:          outbound.ConstructClass,
		Name:          name,
		QualifiedName: name,
		Language:      mustCreateLanguage(t, language),
		StartByte:     start,
		EndByte:       end,
		Content:       content,
		Signature:     fmt.Sprintf("class %s", name),
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s", name),
	}
}

func createJSMethodChunk(
	t *testing.T,
	content, name, className string,
	language string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	return outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("js_method_%s_%s_%d", className, name, start),
		Type:          outbound.ConstructMethod,
		Name:          name,
		QualifiedName: fmt.Sprintf("%s.%s", className, name),
		Language:      mustCreateLanguage(t, language),
		StartByte:     start,
		EndByte:       end,
		Content:       content,
		Signature:     fmt.Sprintf("%s()", name),
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s_%s", className, name),
	}
}

func createTSInterfaceChunk(t *testing.T, content, name string, start, end uint32) outbound.SemanticCodeChunk {
	return outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("ts_interface_%s_%d", name, start),
		Type:          outbound.ConstructInterface,
		Name:          name,
		QualifiedName: name,
		Language:      mustCreateLanguage(t, valueobject.LanguageTypeScript),
		StartByte:     start,
		EndByte:       end,
		Content:       content,
		Signature:     fmt.Sprintf("interface %s", name),
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s", name),
	}
}

func createTSClassChunk(
	t *testing.T,
	content, name string,
	interfaces []string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("ts_class_%s_%d", name, start),
		Type:          outbound.ConstructClass,
		Name:          name,
		QualifiedName: name,
		Language:      mustCreateLanguage(t, valueobject.LanguageTypeScript),
		StartByte:     start,
		EndByte:       end,
		Content:       content,
		Signature:     fmt.Sprintf("class %s", name),
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s", name),
	}

	// Add interface dependencies
	for _, iface := range interfaces {
		chunk.Dependencies = append(chunk.Dependencies, outbound.DependencyReference{
			Name: iface,
			Type: "interface",
		})
	}

	return chunk
}

func createTSMethodChunk(
	t *testing.T,
	content, name, className string,
	isAbstract bool,
	start, end uint32,
) outbound.SemanticCodeChunk {
	return outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("ts_method_%s_%s_%d", className, name, start),
		Type:          outbound.ConstructMethod,
		Name:          name,
		QualifiedName: fmt.Sprintf("%s.%s", className, name),
		Language:      mustCreateLanguage(t, valueobject.LanguageTypeScript),
		StartByte:     start,
		EndByte:       end,
		Content:       content,
		Signature:     fmt.Sprintf("%s()", name),
		IsAbstract:    isAbstract,
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s_%s", className, name),
	}
}

func createJSStaticMethodChunk(
	t *testing.T,
	content, name, className string,
	language string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createJSMethodChunk(t, content, name, className, language, start, end)
	chunk.IsStatic = true
	return chunk
}

func createJSGetterMethodChunk(
	t *testing.T,
	content, name, className string,
	language string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createJSMethodChunk(t, content, name, className, language, start, end)
	chunk.Type = outbound.ConstructProperty
	return chunk
}

// Additional placeholder helper functions for large/complex test data creation.
func createLargeGoStructChunk(t *testing.T, name string, size int) outbound.SemanticCodeChunk {
	content := fmt.Sprintf("type %s struct {\n", name)
	for i := range size / 50 { // Approximate field generation
		content += fmt.Sprintf("    field%d string\n", i)
	}
	content += "}"

	return createGoStructChunk(t, content, name, 0, uint32(len(content)))
}

func createGoMethodGroupChunk(
	t *testing.T,
	structName, groupName string,
	methodNames []string,
) outbound.SemanticCodeChunk {
	content := fmt.Sprintf("// %s methods for %s\n", groupName, structName)
	for _, methodName := range methodNames {
		content += fmt.Sprintf("func (s *%s) %s() error { return nil }\n", structName, methodName)
	}

	return outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("go_method_group_%s_%s", structName, groupName),
		Type:          outbound.ConstructMethod,
		Name:          groupName,
		QualifiedName: fmt.Sprintf("%s.%s", structName, groupName),
		Language:      mustCreateLanguage(t, valueobject.LanguageGo),
		Content:       content,
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s_%s", structName, groupName),
		Metadata:      map[string]interface{}{"method_group": methodNames},
	}
}

func createLargePythonClassChunk(t *testing.T, name string, size int) outbound.SemanticCodeChunk {
	content := fmt.Sprintf("class %s:\n", name)
	content += "    def __init__(self):\n"
	for i := range size / 100 { // Approximate method generation
		content += fmt.Sprintf("        self.attr%d = None\n", i)
	}

	return createPythonClassChunk(t, content, name, 0, uint32(len(content)))
}

func createPythonMethodGroupChunk(
	t *testing.T,
	className, groupName string,
	methodNames []string,
) outbound.SemanticCodeChunk {
	content := fmt.Sprintf("    # %s methods for %s\n", groupName, className)
	for _, methodName := range methodNames {
		content += fmt.Sprintf("    def %s(self):\n        pass\n", methodName)
	}

	return outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("py_method_group_%s_%s", className, groupName),
		Type:          outbound.ConstructMethod,
		Name:          groupName,
		QualifiedName: fmt.Sprintf("%s.%s", className, groupName),
		Language:      mustCreateLanguage(t, valueobject.LanguagePython),
		Content:       content,
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s_%s", className, groupName),
		Metadata:      map[string]interface{}{"method_group": methodNames},
	}
}

func createPythonClassWithMultipleInheritanceChunk(
	t *testing.T,
	content, name string,
	parents []string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createPythonClassChunk(t, content, name, start, end)
	for _, parent := range parents {
		chunk.Dependencies = append(chunk.Dependencies, outbound.DependencyReference{
			Name: parent,
			Type: "inheritance",
		})
	}
	return chunk
}

func createGoStructWithImportsChunk(
	t *testing.T,
	content, name string,
	imports, types []string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createGoStructChunk(t, content, name, start, end)
	for _, imp := range imports {
		chunk.Dependencies = append(chunk.Dependencies, outbound.DependencyReference{
			Name: imp,
			Type: "import",
			Path: imp,
		})
	}
	for _, typ := range types {
		chunk.UsedTypes = append(chunk.UsedTypes, outbound.TypeReference{
			Name:          typ,
			QualifiedName: typ,
		})
	}
	return chunk
}

func createPythonClassWithImportsAndDocsChunk(
	t *testing.T,
	content, name string,
	imports, types []string,
	documentation string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := createPythonClassChunk(t, content, name, start, end)
	chunk.Documentation = documentation
	for _, imp := range imports {
		chunk.Dependencies = append(chunk.Dependencies, outbound.DependencyReference{
			Name: imp,
			Type: "import",
		})
	}
	for _, typ := range types {
		chunk.UsedTypes = append(chunk.UsedTypes, outbound.TypeReference{
			Name:          typ,
			QualifiedName: typ,
		})
	}
	return chunk
}

func createTSClassWithImportsChunk(
	t *testing.T,
	content, name string,
	imports, types []string,
	start, end uint32,
) outbound.SemanticCodeChunk {
	chunk := outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("ts_class_with_imports_%s_%d", name, start),
		Type:          outbound.ConstructClass,
		Name:          name,
		QualifiedName: name,
		Language:      mustCreateLanguage(t, valueobject.LanguageTypeScript),
		StartByte:     start,
		EndByte:       end,
		Content:       content,
		IsGeneric:     strings.Contains(content, "<T"),
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s", name),
	}

	for _, imp := range imports {
		chunk.Dependencies = append(chunk.Dependencies, outbound.DependencyReference{
			Name: imp,
			Type: "import",
		})
	}
	for _, typ := range types {
		chunk.UsedTypes = append(chunk.UsedTypes, outbound.TypeReference{
			Name:          typ,
			QualifiedName: typ,
		})
	}
	return chunk
}

func createComplexClassHierarchy(t *testing.T, classCount int) []outbound.SemanticCodeChunk {
	chunks := make([]outbound.SemanticCodeChunk, 0, classCount*3) // Classes + methods

	for i := range classCount {
		className := fmt.Sprintf("Class%d", i)

		// Create class
		classChunk := createGoStructChunk(t,
			fmt.Sprintf("type %s struct { field int }", className),
			className, uint32(i*100), uint32(i*100+50))
		chunks = append(chunks, classChunk)

		// Create methods for the class
		for j := range 3 {
			methodName := fmt.Sprintf("Method%d", j)
			methodChunk := createGoMethodChunk(t,
				fmt.Sprintf("func (c *%s) %s() error { return nil }", className, methodName),
				methodName, className, uint32(i*100+51+j*20), uint32(i*100+70+j*20))
			chunks = append(chunks, methodChunk)
		}
	}

	return chunks
}

func createExtremelyLargeClassChunk(t *testing.T, name string, size int) outbound.SemanticCodeChunk {
	content := fmt.Sprintf("class %s {\n", name)
	for i := range size / 100 {
		content += fmt.Sprintf("    method%d() { /* ... */ }\n", i)
	}
	content += "}"

	return outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("large_class_%s", name),
		Type:          outbound.ConstructClass,
		Name:          name,
		QualifiedName: name,
		Language:      mustCreateLanguage(t, valueobject.LanguageJavaScript),
		Content:       content,
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("hash_%s", name),
		StartByte:     0,
		EndByte:       uint32(len(content)),
	}
}
