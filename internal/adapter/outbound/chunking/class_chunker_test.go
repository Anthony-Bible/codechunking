package chunking

import (
	"codechunking/internal/port/outbound"
	"context"
	"testing"
)

// TestClassChunkerAddClassOverlapContext tests the addClassOverlapContext method for proper context addition.
func TestClassChunkerAddClassOverlapContext(t *testing.T) {
	tests := []struct {
		name            string
		chunks          []outbound.EnhancedCodeChunk
		overlapSize     int
		expectedContext map[int]string // chunk index -> expected preceding context
		shouldModify    map[int]bool   // chunk index -> should be modified
	}{
		{
			name: "should add overlap between adjacent class chunks",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "class First {\n    method1() {}\n}",
					Language: mustCreateLanguage("javascript"),
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{
							Type:    outbound.ConstructClass,
							Name:    "First",
							Content: "class First {\n    method1() {}\n}",
						},
					},
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "",
					},
				},
				{
					ID:       "chunk-2",
					Content:  "class Second {\n    method2() {}\n}",
					Language: mustCreateLanguage("javascript"),
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{
							Type:    outbound.ConstructClass,
							Name:    "Second",
							Content: "class Second {\n    method2() {}\n}",
						},
					},
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "",
					},
				},
			},
			overlapSize: 500,
			expectedContext: map[int]string{
				0: "", // First chunk should have no preceding context
				1: "class First {",
			},
			shouldModify: map[int]bool{0: false, 1: true},
		},
		{
			name: "should not add overlap when size is zero",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "class First {}",
					Language: mustCreateLanguage("javascript"),
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{Type: outbound.ConstructClass, Name: "First"},
					},
				},
				{
					ID:       "chunk-2",
					Content:  "class Second {}",
					Language: mustCreateLanguage("javascript"),
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{Type: outbound.ConstructClass, Name: "Second"},
					},
				},
			},
			overlapSize: 0,
			expectedContext: map[int]string{
				0: "",
				1: "",
			},
			shouldModify: map[int]bool{0: false, 1: false},
		},
		{
			name: "should append to existing context",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "class First {}",
					Language: mustCreateLanguage("javascript"),
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{Type: outbound.ConstructClass, Name: "First"},
					},
				},
				{
					ID:      "chunk-2",
					Content: "class Second {}",
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{Type: outbound.ConstructClass, Name: "Second"},
					},
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "existing context",
					},
				},
			},
			overlapSize: 500,
			expectedContext: map[int]string{
				0: "",
				1: "class First {\nexisting context",
			},
			shouldModify: map[int]bool{0: false, 1: true},
		},
		{
			name: "should handle single chunk gracefully",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "class OnlyClass {}",
					Language: mustCreateLanguage("javascript"),
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{Type: outbound.ConstructClass, Name: "OnlyClass"},
					},
				},
			},
			overlapSize: 500,
			expectedContext: map[int]string{
				0: "",
			},
			shouldModify: map[int]bool{0: false},
		},
		{
			name:            "should handle empty chunks list",
			chunks:          []outbound.EnhancedCodeChunk{},
			overlapSize:     500,
			expectedContext: map[int]string{},
			shouldModify:    map[int]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewClassChunker()
			ctx := context.Background()
			config := outbound.ChunkingConfiguration{
				OverlapSize: tt.overlapSize,
			}

			chunker.addClassOverlapContext(ctx, tt.chunks, config)

			for i, expected := range tt.expectedContext {
				if i >= len(tt.chunks) {
					continue
				}

				actual := tt.chunks[i].PreservedContext.PrecedingContext
				if actual != expected {
					t.Errorf("Chunk %d preceding context = %q, want %q", i, actual, expected)
				}

				shouldModify := tt.shouldModify[i]
				if shouldModify && actual == "" && expected != "" {
					t.Errorf("Chunk %d should have been modified but wasn't", i)
				}
			}
		})
	}
}

// TestClassChunkerExtractClassOverlapContext tests the extractClassOverlapContext method.
func TestClassChunkerExtractClassOverlapContext(t *testing.T) {
	tests := []struct {
		name          string
		chunk         *outbound.EnhancedCodeChunk
		maxSize       int
		expectedParts []string // Expected parts in order
		shouldContain []string // Strings that should be contained in result
	}{
		{
			name: "should prioritize class definitions over methods and properties",
			chunk: &outbound.EnhancedCodeChunk{
				SemanticConstructs: []outbound.SemanticCodeChunk{
					{
						Type:    outbound.ConstructClass,
						Name:    "TestClass",
						Content: "class TestClass {\n    constructor() {}\n}",
					},
					{
						Type:    outbound.ConstructMethod,
						Name:    "method1",
						Content: "method1() { return 'hello'; }",
					},
					{
						Type:    outbound.ConstructField,
						Name:    "property1",
						Content: "property1 = 'value'",
					},
				},
			},
			maxSize:       500,
			expectedParts: []string{"class TestClass {"},
			shouldContain: []string{"class TestClass {"},
		},
		{
			name: "should include multiple class definitions when available",
			chunk: &outbound.EnhancedCodeChunk{
				SemanticConstructs: []outbound.SemanticCodeChunk{
					{
						Type:    outbound.ConstructClass,
						Name:    "FirstClass",
						Content: "class FirstClass {\n    method1() {}\n}",
					},
					{
						Type:    outbound.ConstructStruct,
						Name:    "DataStruct",
						Content: "struct DataStruct {\n    field string\n}",
					},
				},
			},
			maxSize:       1000,
			expectedParts: []string{"class FirstClass {", "struct DataStruct {"},
			shouldContain: []string{"class FirstClass {", "struct DataStruct {"},
		},
		{
			name: "should respect size limits",
			chunk: &outbound.EnhancedCodeChunk{
				SemanticConstructs: []outbound.SemanticCodeChunk{
					{
						Type:    outbound.ConstructClass,
						Name:    "VeryLongClassName",
						Content: "class VeryLongClassNameWithExtremelyLongNameThatShouldExceedTheSizeLimit {",
					},
					{
						Type:    outbound.ConstructMethod,
						Name:    "method1",
						Content: "method1() { return 'hello'; }",
					},
				},
			},
			maxSize: 50,
			expectedParts: []string{
				"class VeryLongClassNameWithExtremelyLongNameThat",
			}, // truncated to exactly 50 chars
			shouldContain: []string{"class"},
		},
		{
			name: "should include method signatures when no class definitions",
			chunk: &outbound.EnhancedCodeChunk{
				SemanticConstructs: []outbound.SemanticCodeChunk{
					{
						Type:      outbound.ConstructMethod,
						Name:      "testMethod",
						Signature: "testMethod(arg1: string): number",
						Content:   "testMethod(arg1: string): number { return 42; }",
					},
					{
						Type:      outbound.ConstructMethod,
						Name:      "anotherMethod",
						Signature: "anotherMethod(): void",
						Content:   "anotherMethod(): void { console.log('hello'); }",
					},
				},
			},
			maxSize:       500,
			expectedParts: []string{"testMethod(arg1: string): number", "anotherMethod(): void"},
			shouldContain: []string{"testMethod(arg1: string): number", "anotherMethod(): void"},
		},
		{
			name: "should handle empty semantic constructs",
			chunk: &outbound.EnhancedCodeChunk{
				SemanticConstructs: []outbound.SemanticCodeChunk{},
			},
			maxSize:       500,
			expectedParts: nil,
			shouldContain: nil,
		},
		{
			name:          "should handle nil chunk gracefully",
			chunk:         nil,
			maxSize:       500,
			expectedParts: nil,
			shouldContain: nil,
		},
		{
			name: "should return empty string when maxSize is zero",
			chunk: &outbound.EnhancedCodeChunk{
				SemanticConstructs: []outbound.SemanticCodeChunk{
					{
						Type:    outbound.ConstructClass,
						Name:    "TestClass",
						Content: "class TestClass {}",
					},
				},
			},
			maxSize:       0,
			expectedParts: nil,
			shouldContain: nil,
		},
		{
			name: "should return empty string when maxSize is negative",
			chunk: &outbound.EnhancedCodeChunk{
				SemanticConstructs: []outbound.SemanticCodeChunk{
					{
						Type:    outbound.ConstructClass,
						Name:    "TestClass",
						Content: "class TestClass {}",
					},
				},
			},
			maxSize:       -100,
			expectedParts: nil,
			shouldContain: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewClassChunker()
			result := chunker.extractClassOverlapContext(tt.chunk, tt.maxSize)

			// Check that expected parts are in the result in correct order
			if len(tt.expectedParts) > 0 {
				for _, expectedPart := range tt.expectedParts {
					if !containsString(result, expectedPart) {
						t.Errorf("extractClassOverlapContext() result should contain %q, got %q", expectedPart, result)
					}
				}
			} else if result != "" {
				t.Errorf("extractClassOverlapContext() = %q, want empty string", result)
			}

			// Check that required strings are contained
			for _, shouldContain := range tt.shouldContain {
				if !containsString(result, shouldContain) {
					t.Errorf("extractClassOverlapContext() result should contain %q, got %q", shouldContain, result)
				}
			}

			// Check total size doesn't exceed maxSize
			if tt.maxSize > 0 && len(result) > tt.maxSize {
				t.Errorf("extractClassOverlapContext() result size %d exceeds maximum %d", len(result), tt.maxSize)
			}
		})
	}
}

// TestClassChunkerExtractClassDefinition tests the extractClassDefinition method.
func TestClassChunkerExtractClassDefinition(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		maxSize        int
		expected       string
		shouldTruncate bool
	}{
		{
			name:     "should extract class definition up to opening brace",
			content:  "class SimpleClass {\n    constructor() {}\n}",
			maxSize:  500,
			expected: "class SimpleClass {",
		},
		{
			name:     "should extract struct definition",
			content:  "type User struct {\n    ID   int\n    Name string\n}",
			maxSize:  500,
			expected: "type User struct {",
		},
		{
			name:     "should extract interface definition",
			content:  "type Reader interface {\n    Read([]byte) (int, error)\n}",
			maxSize:  500,
			expected: "type Reader interface {",
		},
		{
			name:     "should handle multiple lines before opening brace",
			content:  "// Comment line\nexport class ComplexClass extends BaseClass implements SomeInterface {",
			maxSize:  500,
			expected: "// Comment line\nexport class ComplexClass extends BaseClass implements SomeInterface {",
		},
		{
			name:           "should respect size limits",
			content:        "class VeryLongClassNameWithManyParametersAndModifiers extends SomeParentClass implements MultipleInterfaces {",
			maxSize:        30,
			expected:       "class VeryLongClassNameWithMany", // truncated
			shouldTruncate: true,
		},
		{
			name:     "should handle content without opening brace",
			content:  "type SimpleType string",
			maxSize:  500,
			expected: "type SimpleType string",
		},
		{
			name:     "should handle empty content",
			content:  "",
			maxSize:  500,
			expected: "",
		},
		{
			name:     "should handle empty lines",
			content:  "\n\n",
			maxSize:  500,
			expected: "",
		},
		{
			name:     "should handle comments before opening brace properly",
			content:  "// This is a comment\nclass WithComment {",
			maxSize:  500,
			expected: "// This is a comment\nclass WithComment {",
		},
		{
			name:     "should not stop at commented opening brace",
			content:  "class TestClass {\n    // { this is a comment\nclass ActualClass {",
			maxSize:  1000,
			expected: "class TestClass {\n    // { this is a comment\nclass ActualClass {",
		},
		{
			name:     "should return empty when maxSize is zero",
			content:  "class TestClass {}",
			maxSize:  0,
			expected: "",
		},
		{
			name:     "should return empty when maxSize is negative",
			content:  "class TestClass {}",
			maxSize:  -100,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewClassChunker()
			result := chunker.extractClassDefinition(tt.content, tt.maxSize)

			if !tt.shouldTruncate && result != tt.expected {
				t.Errorf("extractClassDefinition() = %q, want %q", result, tt.expected)
			} else if tt.shouldTruncate {
				if len(result) > tt.maxSize {
					t.Errorf("extractClassDefinition() result size %d exceeds maxSize %d", len(result), tt.maxSize)
				}
			}

			// Check that result doesn't exceed maxSize
			if tt.maxSize > 0 && len(result) > tt.maxSize {
				t.Errorf("extractClassDefinition() result size %d exceeds maxSize %d", len(result), tt.maxSize)
			}
		})
	}
}

// TestClassChunkerExtractMethodSignature tests the extractMethodSignature method.
func TestClassChunkerExtractMethodSignature(t *testing.T) {
	tests := []struct {
		name           string
		chunk          outbound.SemanticCodeChunk
		expected       string
		shouldTruncate bool
	}{
		{
			name: "should return signature when available",
			chunk: outbound.SemanticCodeChunk{
				Name:      "testMethod",
				Signature: "testMethod(arg1 string, arg2 int) error",
				Content:   "testMethod(arg1 string, arg2 int) error {\n    return nil\n}",
			},
			expected:       "testMethod(arg1 string, arg2 int) error",
			shouldTruncate: false,
		},
		{
			name: "should extract first line as fallback when signature is empty",
			chunk: outbound.SemanticCodeChunk{
				Name:    "fallbackMethod",
				Content: "func fallbackMethod() map[string]int {\n    return make(map[string]int)\n}",
			},
			expected:       "func fallbackMethod() map[string]int {",
			shouldTruncate: false,
		},
		{
			name: "should truncate long method signatures",
			chunk: outbound.SemanticCodeChunk{
				Name:    "longMethod",
				Content: "func veryLongMethodName(argument1, argument2, argument3, argument4 string, number1, number2, number3, number4 int, customType some.package.CustomType) (map[string]interface{}, error) {",
			},
			expected:       "func veryLongMethodName(argument1, argument2, argument3, argument4 string, number1, number2, number3, number4 int, customType some.package.CustomType) (map[string]interface{}, error) {",
			shouldTruncate: true,
		},
		{
			name: "should handle JavaScript method signatures",
			chunk: outbound.SemanticCodeChunk{
				Name:      "jsMethod",
				Signature: "public async jsMethod(param1: string): Promise<ReturnType>",
				Content:   "public async jsMethod(param1: string): Promise<ReturnType> { return; }",
			},
			expected:       "public async jsMethod(param1: string): Promise<ReturnType>",
			shouldTruncate: false,
		},
		{
			name: "should handle Python method signatures",
			chunk: outbound.SemanticCodeChunk{
				Name:      "python_method",
				Signature: "def python_method(self, arg1: str, arg2: Optional[int] = None) -> Dict[str, Any]:",
				Content:   "def python_method(self, arg1: str, arg2: Optional[int] = None) -> Dict[str, Any]:\n    pass",
			},
			expected:       "def python_method(self, arg1: str, arg2: Optional[int] = None) -> Dict[str, Any]:",
			shouldTruncate: false,
		},
		{
			name: "should handle empty content gracefully",
			chunk: outbound.SemanticCodeChunk{
				Name:    "emptyMethod",
				Content: "",
			},
			expected:       "",
			shouldTruncate: false,
		},
		{
			name: "should handle content with only whitespace",
			chunk: outbound.SemanticCodeChunk{
				Name:    "whitespaceMethod",
				Content: "   \n\t  ",
			},
			expected:       "   ",
			shouldTruncate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewClassChunker()
			result := chunker.extractMethodSignature(tt.chunk)

			if tt.shouldTruncate && len(result) > 200 {
				if len(result) < 203 || result[len(result)-3:] != "..." {
					t.Errorf("extractMethodSignature() should truncate to 200 chars and add '...', got %q", result)
				}
			} else if result != tt.expected {
				t.Errorf("extractMethodSignature() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestClassChunkerOverlapIntegration tests the integration of class overlap methods.
func TestClassChunkerOverlapIntegration(t *testing.T) {
	tests := []struct {
		name            string
		semanticChunks  []outbound.SemanticCodeChunk
		config          outbound.ChunkingConfiguration
		expectedContext map[int]string // chunk index -> expected preceding context
	}{
		{
			name: "should add correct overlap context in class-based chunking",
			semanticChunks: []outbound.SemanticCodeChunk{
				{
					Name:      "ClassA",
					Type:      outbound.ConstructClass,
					Content:   "class ClassA {\n    method1() {}\n}",
					Language:  mustCreateLanguage("javascript"),
					StartByte: 0,
					EndByte:   100,
				},
				{
					Name:      "ClassB",
					Type:      outbound.ConstructClass,
					Content:   "class ClassB {\n    method2() {}\n}",
					Language:  mustCreateLanguage("javascript"),
					StartByte: 101,
					EndByte:   200,
				},
				{
					Name:      "ClassC",
					Type:      outbound.ConstructClass,
					Content:   "class ClassC {\n    method3() {}\n}",
					Language:  mustCreateLanguage("javascript"),
					StartByte: 201,
					EndByte:   300,
				},
			},
			config: outbound.ChunkingConfiguration{
				OverlapSize:     500,
				EnableSplitting: true,
			},
			expectedContext: map[int]string{
				0: "",
				1: "class ClassA {",
				2: "class ClassB {",
			},
		},
		{
			name: "should handle mixed semantic constructs in overlap",
			semanticChunks: []outbound.SemanticCodeChunk{
				{
					Name:      "FirstClass",
					Type:      outbound.ConstructClass,
					Content:   "class FirstClass {\n    constructor() {}\n}",
					Language:  mustCreateLanguage("javascript"),
					StartByte: 0,
					EndByte:   100,
				},
				{
					Name:      "JustAMethod",
					Type:      outbound.ConstructMethod,
					Signature: "standaloneMethod() {}",
					Content:   "function standaloneMethod() {}",
					Language:  mustCreateLanguage("javascript"),
					StartByte: 101,
					EndByte:   150,
				},
				{
					Name:      "SecondClass",
					Type:      outbound.ConstructClass,
					Content:   "class SecondClass {\n    method() {}\n}",
					Language:  mustCreateLanguage("javascript"),
					StartByte: 151,
					EndByte:   250,
				},
			},
			config: outbound.ChunkingConfiguration{
				OverlapSize:     500,
				EnableSplitting: true,
			},
			expectedContext: map[int]string{
				0: "",
				1: "class FirstClass {",
				2: "standaloneMethod() {}", // Method signature from chunk 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewClassChunker()
			ctx := context.Background()

			// Simulate creating enhanced chunks with overlap
			groups := chunker.groupByClassHierarchy(ctx, tt.semanticChunks)
			var enhancedChunks []outbound.EnhancedCodeChunk

			for _, group := range groups {
				chunks, err := chunker.createEnhancedChunksFromClassGroup(ctx, group, tt.config)
				if err != nil {
					t.Fatalf("Failed to create enhanced chunks: %v", err)
				}
				enhancedChunks = append(enhancedChunks, chunks...)
			}

			// Add overlap context
			if tt.config.OverlapSize > 0 && len(enhancedChunks) > 1 {
				chunker.addClassOverlapContext(ctx, enhancedChunks, tt.config)
			}

			for i, expected := range tt.expectedContext {
				if i >= len(enhancedChunks) {
					continue
				}

				actual := enhancedChunks[i].PreservedContext.PrecedingContext
				if actual != expected {
					t.Errorf("Chunk %d preceding context = %q, want %q", i, actual, expected)
				}
			}
		})
	}
}

// TestClassChunkerEdgeCases tests edge cases for overlap functionality.
func TestClassChunkerEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(*testing.T, *ClassChunker)
	}{
		{
			name: "should handle very long class names in overlap",
			testFunc: func(t *testing.T, chunker *ClassChunker) {
				longClassName := "VeryLongClassNameThatExceedsNormalLimitsAndShouldBeHandledProperly"
				chunk := &outbound.EnhancedCodeChunk{
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{
							Type:    outbound.ConstructClass,
							Name:    longClassName,
							Content: "class " + longClassName + " {",
						},
					},
				}

				result := chunker.extractClassOverlapContext(chunk, 50)
				if len(result) > 50 {
					t.Errorf(
						"extractClassOverlapContext() should respect size limit for long class names, got %q",
						result,
					)
				}
			},
		},
		{
			name: "should handle multiple language class definitions",
			testFunc: func(t *testing.T, chunker *ClassChunker) {
				chunk := &outbound.EnhancedCodeChunk{
					SemanticConstructs: []outbound.SemanticCodeChunk{
						{
							Type:    outbound.ConstructClass,
							Name:    "JSCar",
							Content: "class JSCar {\n    drive() {}\n}",
						},
						{
							Type:    outbound.ConstructStruct,
							Name:    "GoCar",
							Content: "type GoCar struct {\n    Brand string\n}",
						},
					},
				}

				result := chunker.extractClassOverlapContext(chunk, 1000)
				if !containsString(result, "class JSCar {") && !containsString(result, "type GoCar struct {") {
					t.Errorf(
						"extractClassOverlapContext() should handle multiple language class definitions, got %q",
						result,
					)
				}
			},
		},
		{
			name: "should handle malformed class definitions gracefully",
			testFunc: func(t *testing.T, chunker *ClassChunker) {
				content := "class IncompleteClass" // Missing opening brace
				result := chunker.extractClassDefinition(content, 500)
				expected := "class IncompleteClass"
				if result != expected {
					t.Errorf(
						"extractClassDefinition() should handle malformed definitions, got %q want %q",
						result,
						expected,
					)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewClassChunker()
			tt.testFunc(t, chunker)
		})
	}
}
