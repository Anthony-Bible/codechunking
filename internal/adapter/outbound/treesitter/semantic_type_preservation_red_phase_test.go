package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSemanticCodeChunkToCodeChunkConversion_PreservesAllTypeInformation tests that the
// convertSemanticToCodeChunks method properly preserves all semantic type information
// when converting SemanticCodeChunk to CodeChunk.
func TestSemanticCodeChunkToCodeChunkConversion_PreservesAllTypeInformation(t *testing.T) {
	tests := []struct {
		name                  string
		inputSemanticChunk    outbound.SemanticCodeChunk
		expectedType          string
		expectedEntityName    string
		expectedParentEntity  string
		expectedQualifiedName string
		expectedSignature     string
		expectedVisibility    string
	}{
		{
			name: "Function_PreservesAllFields",
			inputSemanticChunk: outbound.SemanticCodeChunk{
				ChunkID:       "test-chunk-1",
				Type:          outbound.ConstructFunction,
				Name:          "calculateTotal",
				QualifiedName: "com.example.Calculator.calculateTotal",
				Language:      mustCreateLanguage("go"),
				StartByte:     100,
				EndByte:       200,
				StartPosition: valueobject.Position{Row: 10, Column: 0},
				EndPosition:   valueobject.Position{Row: 20, Column: 1},
				Content:       "func calculateTotal(items []Item) float64 {\n\treturn 0.0\n}",
				Signature:     "func calculateTotal(items []Item) float64",
				Visibility:    outbound.Public,
				ParentChunk: &outbound.SemanticCodeChunk{
					Name: "Calculator",
					Type: outbound.ConstructClass,
				},
				ExtractedAt: time.Now(),
				Hash:        "hash123",
			},
			expectedType:          "function",
			expectedEntityName:    "calculateTotal",
			expectedParentEntity:  "Calculator",
			expectedQualifiedName: "com.example.Calculator.calculateTotal",
			expectedSignature:     "func calculateTotal(items []Item) float64",
			expectedVisibility:    "public",
		},
		{
			name: "Method_PreservesPrivateVisibility",
			inputSemanticChunk: outbound.SemanticCodeChunk{
				ChunkID:       "test-chunk-2",
				Type:          outbound.ConstructMethod,
				Name:          "validateInput",
				QualifiedName: "com.example.Calculator.validateInput",
				Language:      mustCreateLanguage("go"),
				StartByte:     300,
				EndByte:       400,
				StartPosition: valueobject.Position{Row: 25, Column: 0},
				EndPosition:   valueobject.Position{Row: 30, Column: 1},
				Content:       "func (c *Calculator) validateInput(value float64) bool {\n\treturn value >= 0\n}",
				Signature:     "func (c *Calculator) validateInput(value float64) bool",
				Visibility:    outbound.Private,
				ParentChunk: &outbound.SemanticCodeChunk{
					Name: "Calculator",
					Type: outbound.ConstructClass,
				},
				ExtractedAt: time.Now(),
				Hash:        "hash456",
			},
			expectedType:          "method",
			expectedEntityName:    "validateInput",
			expectedParentEntity:  "Calculator",
			expectedQualifiedName: "com.example.Calculator.validateInput",
			expectedSignature:     "func (c *Calculator) validateInput(value float64) bool",
			expectedVisibility:    "private",
		},
		{
			name: "Class_WithoutParent",
			inputSemanticChunk: outbound.SemanticCodeChunk{
				ChunkID:       "test-chunk-3",
				Type:          outbound.ConstructClass,
				Name:          "Calculator",
				QualifiedName: "com.example.Calculator",
				Language:      mustCreateLanguage("go"),
				StartByte:     500,
				EndByte:       800,
				StartPosition: valueobject.Position{Row: 5, Column: 0},
				EndPosition:   valueobject.Position{Row: 50, Column: 1},
				Content:       "type Calculator struct {\n\tprecision int\n}",
				Signature:     "type Calculator struct",
				Visibility:    outbound.Public,
				ParentChunk:   nil, // No parent
				ExtractedAt:   time.Now(),
				Hash:          "hash789",
			},
			expectedType:          "class",
			expectedEntityName:    "Calculator",
			expectedParentEntity:  "", // Should be empty when no parent
			expectedQualifiedName: "com.example.Calculator",
			expectedSignature:     "type Calculator struct",
			expectedVisibility:    "public",
		},
		{
			name: "Interface_PreservesProtectedVisibility",
			inputSemanticChunk: outbound.SemanticCodeChunk{
				ChunkID:       "test-chunk-4",
				Type:          outbound.ConstructInterface,
				Name:          "Calculable",
				QualifiedName: "com.example.Calculable",
				Language:      mustCreateLanguage("go"),
				StartByte:     900,
				EndByte:       1000,
				StartPosition: valueobject.Position{Row: 55, Column: 0},
				EndPosition:   valueobject.Position{Row: 60, Column: 1},
				Content:       "type Calculable interface {\n\tCalculate() float64\n}",
				Signature:     "type Calculable interface",
				Visibility:    outbound.Protected,
				ParentChunk:   nil,
				ExtractedAt:   time.Now(),
				Hash:          "hash101",
			},
			expectedType:          "interface",
			expectedEntityName:    "Calculable",
			expectedParentEntity:  "",
			expectedQualifiedName: "com.example.Calculable",
			expectedSignature:     "type Calculable interface",
			expectedVisibility:    "protected",
		},
		{
			name: "Variable_PreservesConstantType",
			inputSemanticChunk: outbound.SemanticCodeChunk{
				ChunkID:       "test-chunk-5",
				Type:          outbound.ConstructConstant,
				Name:          "DefaultPrecision",
				QualifiedName: "com.example.Calculator.DefaultPrecision",
				Language:      mustCreateLanguage("go"),
				StartByte:     1100,
				EndByte:       1150,
				StartPosition: valueobject.Position{Row: 65, Column: 0},
				EndPosition:   valueobject.Position{Row: 65, Column: 50},
				Content:       "const DefaultPrecision = 2",
				Signature:     "const DefaultPrecision int",
				Visibility:    outbound.Public,
				ParentChunk: &outbound.SemanticCodeChunk{
					Name: "Calculator",
					Type: outbound.ConstructClass,
				},
				ExtractedAt: time.Now(),
				Hash:        "hash202",
			},
			expectedType:          "constant",
			expectedEntityName:    "DefaultPrecision",
			expectedParentEntity:  "Calculator",
			expectedQualifiedName: "com.example.Calculator.DefaultPrecision",
			expectedSignature:     "const DefaultPrecision int",
			expectedVisibility:    "public",
		},
	}

	parser := &TreeSitterCodeParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act: Convert semantic chunks to code chunks
			result := parser.convertSemanticToCodeChunks(
				"/test/file.go",
				[]outbound.SemanticCodeChunk{tt.inputSemanticChunk},
				"go",
			)

			// Assert: Should return exactly one chunk
			require.Len(t, result, 1, "Should convert exactly one semantic chunk to one code chunk")

			chunk := result[0]

			// Assert: All type information should be preserved
			assert.Equal(t, tt.expectedType, chunk.Type, "Type should be preserved")
			assert.Equal(t, tt.expectedEntityName, chunk.EntityName, "EntityName should be preserved")
			assert.Equal(t, tt.expectedParentEntity, chunk.ParentEntity, "ParentEntity should be preserved")
			assert.Equal(t, tt.expectedQualifiedName, chunk.QualifiedName, "QualifiedName should be preserved")
			assert.Equal(t, tt.expectedSignature, chunk.Signature, "Signature should be preserved")
			assert.Equal(t, tt.expectedVisibility, chunk.Visibility, "Visibility should be preserved")

			// Assert: Basic chunk fields should also be preserved
			assert.Equal(t, "/test/file.go", chunk.FilePath, "FilePath should be set correctly")
			assert.Equal(t, "go", chunk.Language, "Language should be preserved")
			assert.Equal(t, tt.inputSemanticChunk.Content, chunk.Content, "Content should be preserved")
			assert.NotEmpty(t, chunk.ID, "ID should be generated")
			assert.NotEmpty(t, chunk.Hash, "Hash should be generated")
			assert.NotZero(t, chunk.CreatedAt, "CreatedAt should be set")
		})
	}
}

// TestSemanticCodeChunkToCodeChunkConversion_HandlesEmptySemanticFields tests that
// conversion handles semantic chunks with empty or missing type information gracefully.
func TestSemanticCodeChunkToCodeChunkConversion_HandlesEmptySemanticFields(t *testing.T) {
	tests := []struct {
		name                  string
		inputSemanticChunk    outbound.SemanticCodeChunk
		expectedType          string
		expectedEntityName    string
		expectedParentEntity  string
		expectedQualifiedName string
		expectedSignature     string
		expectedVisibility    string
	}{
		{
			name: "EmptySemanticFields_ShouldPreserveEmptyValues",
			inputSemanticChunk: outbound.SemanticCodeChunk{
				ChunkID:       "test-chunk-empty",
				Type:          "", // Empty type
				Name:          "", // Empty name
				QualifiedName: "", // Empty qualified name
				Language:      mustCreateLanguage("go"),
				StartByte:     0,
				EndByte:       50,
				StartPosition: valueobject.Position{Row: 1, Column: 0},
				EndPosition:   valueobject.Position{Row: 3, Column: 1},
				Content:       "// Some comment\nvar x = 1",
				Signature:     "", // Empty signature
				Visibility:    "", // Empty visibility
				ParentChunk:   nil,
				ExtractedAt:   time.Now(),
				Hash:          "hash_empty",
			},
			expectedType:          "",
			expectedEntityName:    "",
			expectedParentEntity:  "",
			expectedQualifiedName: "",
			expectedSignature:     "",
			expectedVisibility:    "",
		},
		{
			name: "PartialSemanticFields_ShouldPreserveAvailableData",
			inputSemanticChunk: outbound.SemanticCodeChunk{
				ChunkID:       "test-chunk-partial",
				Type:          outbound.ConstructFunction,
				Name:          "helperFunc",
				QualifiedName: "", // Missing qualified name
				Language:      mustCreateLanguage("go"),
				StartByte:     0,
				EndByte:       100,
				StartPosition: valueobject.Position{Row: 1, Column: 0},
				EndPosition:   valueobject.Position{Row: 5, Column: 1},
				Content:       "func helperFunc() {\n\t// helper logic\n}",
				Signature:     "func helperFunc()", // Has signature
				Visibility:    "",                  // Missing visibility
				ParentChunk:   nil,
				ExtractedAt:   time.Now(),
				Hash:          "hash_partial",
			},
			expectedType:          "function",
			expectedEntityName:    "helperFunc",
			expectedParentEntity:  "",
			expectedQualifiedName: "",
			expectedSignature:     "func helperFunc()",
			expectedVisibility:    "",
		},
	}

	parser := &TreeSitterCodeParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act: Convert semantic chunks to code chunks
			result := parser.convertSemanticToCodeChunks(
				"/test/file.go",
				[]outbound.SemanticCodeChunk{tt.inputSemanticChunk},
				"go",
			)

			// Assert: Should return exactly one chunk
			require.Len(t, result, 1, "Should convert exactly one semantic chunk to one code chunk")

			chunk := result[0]

			// Assert: Should preserve all available type information (including empty values)
			assert.Equal(t, tt.expectedType, chunk.Type, "Type should be preserved (even if empty)")
			assert.Equal(t, tt.expectedEntityName, chunk.EntityName, "EntityName should be preserved (even if empty)")
			assert.Equal(
				t,
				tt.expectedParentEntity,
				chunk.ParentEntity,
				"ParentEntity should be preserved (even if empty)",
			)
			assert.Equal(
				t,
				tt.expectedQualifiedName,
				chunk.QualifiedName,
				"QualifiedName should be preserved (even if empty)",
			)
			assert.Equal(t, tt.expectedSignature, chunk.Signature, "Signature should be preserved (even if empty)")
			assert.Equal(t, tt.expectedVisibility, chunk.Visibility, "Visibility should be preserved (even if empty)")
		})
	}
}

// TestSemanticCodeChunkToCodeChunkConversion_HandlesMultipleChunks tests that
// conversion handles multiple semantic chunks and preserves type information for all.
func TestSemanticCodeChunkToCodeChunkConversion_HandlesMultipleChunks(t *testing.T) {
	// Arrange: Create multiple semantic chunks with different types
	semanticChunks := []outbound.SemanticCodeChunk{
		{
			ChunkID:       "chunk-1",
			Type:          outbound.ConstructClass,
			Name:          "User",
			QualifiedName: "com.example.User",
			Language:      mustCreateLanguage("go"),
			StartByte:     0,
			EndByte:       100,
			Content:       "type User struct { Name string }",
			Signature:     "type User struct",
			Visibility:    outbound.Public,
			ExtractedAt:   time.Now(),
			Hash:          "hash1",
		},
		{
			ChunkID:       "chunk-2",
			Type:          outbound.ConstructMethod,
			Name:          "GetName",
			QualifiedName: "com.example.User.GetName",
			Language:      mustCreateLanguage("go"),
			StartByte:     101,
			EndByte:       200,
			Content:       "func (u *User) GetName() string { return u.Name }",
			Signature:     "func (u *User) GetName() string",
			Visibility:    outbound.Public,
			ParentChunk: &outbound.SemanticCodeChunk{
				Name: "User",
				Type: outbound.ConstructClass,
			},
			ExtractedAt: time.Now(),
			Hash:        "hash2",
		},
		{
			ChunkID:       "chunk-3",
			Type:          outbound.ConstructConstant,
			Name:          "DefaultUserRole",
			QualifiedName: "com.example.DefaultUserRole",
			Language:      mustCreateLanguage("go"),
			StartByte:     201,
			EndByte:       250,
			Content:       "const DefaultUserRole = \"user\"",
			Signature:     "const DefaultUserRole string",
			Visibility:    outbound.Public,
			ExtractedAt:   time.Now(),
			Hash:          "hash3",
		},
	}

	parser := &TreeSitterCodeParser{}

	// Act: Convert multiple semantic chunks to code chunks
	result := parser.convertSemanticToCodeChunks(
		"/test/user.go",
		semanticChunks,
		"go",
	)

	// Assert: Should return exactly three chunks
	require.Len(t, result, 3, "Should convert all three semantic chunks to code chunks")

	// Assert: First chunk (class) type information
	assert.Equal(t, "class", result[0].Type)
	assert.Equal(t, "User", result[0].EntityName)
	assert.Equal(t, "", result[0].ParentEntity) // No parent
	assert.Equal(t, "com.example.User", result[0].QualifiedName)
	assert.Equal(t, "type User struct", result[0].Signature)
	assert.Equal(t, "public", result[0].Visibility)

	// Assert: Second chunk (method) type information
	assert.Equal(t, "method", result[1].Type)
	assert.Equal(t, "GetName", result[1].EntityName)
	assert.Equal(t, "User", result[1].ParentEntity) // Has parent
	assert.Equal(t, "com.example.User.GetName", result[1].QualifiedName)
	assert.Equal(t, "func (u *User) GetName() string", result[1].Signature)
	assert.Equal(t, "public", result[1].Visibility)

	// Assert: Third chunk (constant) type information
	assert.Equal(t, "constant", result[2].Type)
	assert.Equal(t, "DefaultUserRole", result[2].EntityName)
	assert.Equal(t, "", result[2].ParentEntity) // No parent
	assert.Equal(t, "com.example.DefaultUserRole", result[2].QualifiedName)
	assert.Equal(t, "const DefaultUserRole string", result[2].Signature)
	assert.Equal(t, "public", result[2].Visibility)

	// Assert: All chunks should have basic properties set correctly
	for i, chunk := range result {
		assert.Equal(t, "/test/user.go", chunk.FilePath, "FilePath should be set for chunk %d", i)
		assert.Equal(t, "go", chunk.Language, "Language should be set for chunk %d", i)
		assert.NotEmpty(t, chunk.ID, "ID should be generated for chunk %d", i)
		assert.NotEmpty(t, chunk.Hash, "Hash should be generated for chunk %d", i)
		assert.NotZero(t, chunk.CreatedAt, "CreatedAt should be set for chunk %d", i)
		assert.NotEmpty(t, chunk.Content, "Content should be preserved for chunk %d", i)
	}
}

// TestSemanticCodeChunkToCodeChunkConversion_HandlesAllSemanticConstructTypes tests that
// conversion handles all defined semantic construct types correctly.
func TestSemanticCodeChunkToCodeChunkConversion_HandlesAllSemanticConstructTypes(t *testing.T) {
	constructTypes := []struct {
		semanticType outbound.SemanticConstructType
		expectedType string
	}{
		{outbound.ConstructFunction, "function"},
		{outbound.ConstructMethod, "method"},
		{outbound.ConstructClass, "class"},
		{outbound.ConstructStruct, "struct"},
		{outbound.ConstructInterface, "interface"},
		{outbound.ConstructEnum, "enum"},
		{outbound.ConstructVariable, "variable"},
		{outbound.ConstructConstant, "constant"},
		{outbound.ConstructField, "field"},
		{outbound.ConstructProperty, "property"},
		{outbound.ConstructModule, "module"},
		{outbound.ConstructPackage, "package"},
		{outbound.ConstructNamespace, "namespace"},
		{outbound.ConstructType, "type"},
		{outbound.ConstructComment, "comment"},
		{outbound.ConstructDecorator, "decorator"},
		{outbound.ConstructAttribute, "attribute"},
		{outbound.ConstructLambda, "lambda"},
		{outbound.ConstructClosure, "closure"},
		{outbound.ConstructGenerator, "generator"},
		{outbound.ConstructAsyncFunction, "async_function"},
	}

	parser := &TreeSitterCodeParser{}

	for _, ct := range constructTypes {
		t.Run(string(ct.semanticType), func(t *testing.T) {
			// Arrange: Create semantic chunk with specific construct type
			semanticChunk := outbound.SemanticCodeChunk{
				ChunkID:       "test-chunk",
				Type:          ct.semanticType,
				Name:          "TestEntity",
				QualifiedName: "com.test.TestEntity",
				Language:      mustCreateLanguage("go"),
				StartByte:     0,
				EndByte:       100,
				Content:       "// Test content",
				Signature:     "test signature",
				Visibility:    outbound.Public,
				ExtractedAt:   time.Now(),
				Hash:          "testhash",
			}

			// Act: Convert semantic chunk to code chunk
			result := parser.convertSemanticToCodeChunks(
				"/test/file.go",
				[]outbound.SemanticCodeChunk{semanticChunk},
				"go",
			)

			// Assert: Should preserve the construct type correctly
			require.Len(t, result, 1)
			assert.Equal(
				t,
				ct.expectedType,
				result[0].Type,
				"Construct type %s should convert to %s",
				ct.semanticType,
				ct.expectedType,
			)
		})
	}
}

// mustCreateLanguage is a test helper that creates a Language value object or panics.
func mustCreateLanguage(langStr string) valueobject.Language {
	lang, err := valueobject.NewLanguage(langStr)
	if err != nil {
		panic(err)
	}
	return lang
}
