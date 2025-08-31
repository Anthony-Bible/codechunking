package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoStructureParser_ParseGoStruct_BasicStruct tests parsing of basic Go structs.
// This is a RED PHASE test that defines expected behavior for basic Go struct parsing.
func TestGoStructureParser_ParseGoStruct_BasicStruct(t *testing.T) {
	sourceCode := `package main

type User struct {
	ID       int    ` + "`json:\"id\" db:\"id\"`" + `
	Name     string ` + "`json:\"name\" db:\"name\"`" + `
	Email    string ` + "`json:\"email\" db:\"email\"`" + `
	Age      int    ` + "`json:\"age\" db:\"age\"`" + `
	IsActive bool   ` + "`json:\"is_active\" db:\"is_active\"`" + `
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	// Find type declaration
	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1, "Should find 1 type declaration")

	typeSpec := findChildByType(typeDecls[0], "type_spec")
	require.NotNil(t, typeSpec, "Should find type_spec")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoStruct(context.Background(), parseTree, typeDecls[0], typeSpec, "main", options, time.Now())
	require.NotNil(t, result, "ParseGoStruct should return a result")

	// Validate struct properties
	assert.Equal(t, outbound.ConstructStruct, result.Type)
	assert.Equal(t, "User", result.Name)
	assert.Equal(t, "main.User", result.QualifiedName)
	assert.Equal(t, outbound.Public, result.Visibility)
	assert.False(t, result.IsGeneric)
	assert.False(t, result.IsAsync)
	assert.False(t, result.IsAbstract)

	// Validate child chunks (struct fields)
	require.Len(t, result.ChildChunks, 5, "Should have 5 field chunks")

	expectedFields := []struct {
		name       string
		fieldType  string
		visibility outbound.VisibilityModifier
		tagCount   int
	}{
		{"ID", "int", outbound.Public, 2},
		{"Name", "string", outbound.Public, 2},
		{"Email", "string", outbound.Public, 2},
		{"Age", "int", outbound.Public, 2},
		{"IsActive", "bool", outbound.Public, 2},
	}

	for i, expected := range expectedFields {
		field := result.ChildChunks[i]
		assert.Equal(t, outbound.ConstructField, field.Type)
		assert.Equal(t, expected.name, field.Name)
		assert.Equal(t, expected.fieldType, field.ReturnType)
		assert.Equal(t, expected.visibility, field.Visibility)
		assert.Len(t, field.Annotations, expected.tagCount)
	}
}

// TestGoStructureParser_ParseGoStruct_EmbeddedStruct tests parsing of structs with embedded fields.
// This is a RED PHASE test that defines expected behavior for Go embedded struct parsing.
func TestGoStructureParser_ParseGoStruct_EmbeddedStruct(t *testing.T) {
	sourceCode := `package main

type Person struct {
	Name string
	Age  int
}

type Employee struct {
	Person          // Embedded struct
	EmployeeID int  ` + "`json:\"employee_id\"`" + `
	Department string
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 2, "Should find 2 type declarations")

	// Test Employee struct (second declaration)
	employeeTypeSpec := findChildByType(typeDecls[1], "type_spec")
	require.NotNil(t, employeeTypeSpec)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoStruct(
		context.Background(),
		parseTree,
		typeDecls[1],
		employeeTypeSpec,
		"main",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	assert.Equal(t, "Employee", result.Name)
	assert.Equal(t, "main.Employee", result.QualifiedName)

	// Should have 3 fields: embedded Person, EmployeeID, Department
	require.Len(t, result.ChildChunks, 3)

	// Check embedded field
	assert.Equal(t, "Person", result.ChildChunks[0].Name)
	assert.Equal(t, "Person", result.ChildChunks[0].ReturnType)
	assert.Equal(t, outbound.Public, result.ChildChunks[0].Visibility)

	// Check regular fields
	assert.Equal(t, "EmployeeID", result.ChildChunks[1].Name)
	assert.Equal(t, "int", result.ChildChunks[1].ReturnType)
	assert.Len(t, result.ChildChunks[1].Annotations, 1) // json tag

	assert.Equal(t, "Department", result.ChildChunks[2].Name)
	assert.Equal(t, "string", result.ChildChunks[2].ReturnType)
}

// TestGoStructureParser_ParseGoStruct_GenericStruct tests parsing of generic Go structs.
// This is a RED PHASE test that defines expected behavior for Go generic struct parsing.
func TestGoStructureParser_ParseGoStruct_GenericStruct(t *testing.T) {
	sourceCode := `package main

type Container[T any] struct {
	value T
	count int
}

type Pair[K comparable, V any] struct {
	Key   K ` + "`json:\"key\"`" + `
	Value V ` + "`json:\"value\"`" + `
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 2, "Should find 2 type declarations")

	// Test Container struct
	containerTypeSpec := findChildByType(typeDecls[0], "type_spec")
	require.NotNil(t, containerTypeSpec)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoStruct(
		context.Background(),
		parseTree,
		typeDecls[0],
		containerTypeSpec,
		"main",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Validate generic struct properties
	assert.Equal(t, "Container", result.Name)
	assert.True(t, result.IsGeneric)

	// Validate generic parameters
	require.Len(t, result.GenericParameters, 1)
	assert.Equal(t, "T", result.GenericParameters[0].Name)
	assert.Equal(t, []string{"any"}, result.GenericParameters[0].Constraints)

	// Validate fields
	require.Len(t, result.ChildChunks, 2)
	assert.Equal(t, "value", result.ChildChunks[0].Name)
	assert.Equal(t, "T", result.ChildChunks[0].ReturnType) // Generic type
	assert.Equal(t, outbound.Private, result.ChildChunks[0].Visibility)

	assert.Equal(t, "count", result.ChildChunks[1].Name)
	assert.Equal(t, "int", result.ChildChunks[1].ReturnType)
	assert.Equal(t, outbound.Private, result.ChildChunks[1].Visibility)
}

// TestGoStructureParser_ParseGoInterface_BasicInterface tests parsing of basic Go interfaces.
// This is a RED PHASE test that defines expected behavior for basic Go interface parsing.
func TestGoStructureParser_ParseGoInterface_BasicInterface(t *testing.T) {
	sourceCode := `package main

type Writer interface {
	Write(data []byte) (int, error)
}

type Reader interface {
	Read(buffer []byte) (int, error)
}

type ReadWriter interface {
	Reader
	Writer
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 3, "Should find 3 type declarations")

	// Test Writer interface
	writerTypeSpec := findChildByType(typeDecls[0], "type_spec")
	require.NotNil(t, writerTypeSpec)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoInterface(
		context.Background(),
		parseTree,
		typeDecls[0],
		writerTypeSpec,
		"main",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Validate interface properties
	assert.Equal(t, outbound.ConstructInterface, result.Type)
	assert.Equal(t, "Writer", result.Name)
	assert.Equal(t, "main.Writer", result.QualifiedName)
	assert.Equal(t, outbound.Public, result.Visibility)
	assert.True(t, result.IsAbstract)
	assert.False(t, result.IsGeneric)

	// Should have one method
	require.Len(t, result.ChildChunks, 1)
	method := result.ChildChunks[0]
	assert.Equal(t, outbound.ConstructMethod, method.Type)
	assert.Equal(t, "Write", method.Name)
	assert.Equal(t, "main.Writer.Write", method.QualifiedName)
	assert.True(t, method.IsAbstract)
	assert.Equal(t, "(int, error)", method.ReturnType)

	// Validate method parameters
	require.Len(t, method.Parameters, 1)
	assert.Equal(t, "data", method.Parameters[0].Name)
	assert.Equal(t, "[]byte", method.Parameters[0].Type)
}

// TestGoStructureParser_ParseGoInterface_GenericInterface tests parsing of generic Go interfaces.
// This is a RED PHASE test that defines expected behavior for Go generic interface parsing.
func TestGoStructureParser_ParseGoInterface_GenericInterface(t *testing.T) {
	sourceCode := `package main

type Comparable[T any] interface {
	CompareTo(other T) int
}

type Container[T any] interface {
	Add(item T) bool
	Get(index int) (T, error)
	Size() int
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 2, "Should find 2 type declarations")

	// Test Container interface
	containerTypeSpec := findChildByType(typeDecls[1], "type_spec")
	require.NotNil(t, containerTypeSpec)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoInterface(
		context.Background(),
		parseTree,
		typeDecls[1],
		containerTypeSpec,
		"main",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Validate generic interface properties
	assert.Equal(t, "Container", result.Name)
	assert.True(t, result.IsGeneric)
	assert.True(t, result.IsAbstract)

	// Validate generic parameters
	require.Len(t, result.GenericParameters, 1)
	assert.Equal(t, "T", result.GenericParameters[0].Name)
	assert.Equal(t, []string{"any"}, result.GenericParameters[0].Constraints)

	// Should have 3 methods
	require.Len(t, result.ChildChunks, 3)

	// Check Add method
	addMethod := result.ChildChunks[0]
	assert.Equal(t, "Add", addMethod.Name)
	assert.Equal(t, "bool", addMethod.ReturnType)
	require.Len(t, addMethod.Parameters, 1)
	assert.Equal(t, "item", addMethod.Parameters[0].Name)
	assert.Equal(t, "T", addMethod.Parameters[0].Type) // Generic type

	// Check Get method
	getMethod := result.ChildChunks[1]
	assert.Equal(t, "Get", getMethod.Name)
	assert.Equal(t, "(T, error)", getMethod.ReturnType)

	// Check Size method
	sizeMethod := result.ChildChunks[2]
	assert.Equal(t, "Size", sizeMethod.Name)
	assert.Equal(t, "int", sizeMethod.ReturnType)
	assert.Empty(t, sizeMethod.Parameters) // No parameters
}

// TestGoStructureParser_ParseGoInterface_EmbeddedInterface tests parsing of interfaces with embedded interfaces.
// This is a RED PHASE test that defines expected behavior for Go embedded interface parsing.
func TestGoStructureParser_ParseGoInterface_EmbeddedInterface(t *testing.T) {
	sourceCode := `package main

type Reader interface {
	Read([]byte) (int, error)
}

type Writer interface {
	Write([]byte) (int, error)
}

type ReadWriter interface {
	Reader
	Writer
	Close() error
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 3, "Should find 3 type declarations")

	// Test ReadWriter interface (third declaration)
	readWriterTypeSpec := findChildByType(typeDecls[2], "type_spec")
	require.NotNil(t, readWriterTypeSpec)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoInterface(
		context.Background(),
		parseTree,
		typeDecls[2],
		readWriterTypeSpec,
		"main",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	assert.Equal(t, "ReadWriter", result.Name)

	// Should have 3 child chunks: embedded Reader, embedded Writer, and Close method
	require.Len(t, result.ChildChunks, 3)

	// Check embedded interfaces
	assert.Equal(t, "Reader", result.ChildChunks[0].Name)
	assert.Equal(t, outbound.ConstructInterface, result.ChildChunks[0].Type)
	assert.True(t, result.ChildChunks[0].IsAbstract)

	assert.Equal(t, "Writer", result.ChildChunks[1].Name)
	assert.Equal(t, outbound.ConstructInterface, result.ChildChunks[1].Type)

	// Check Close method
	closeMethod := result.ChildChunks[2]
	assert.Equal(t, "Close", closeMethod.Name)
	assert.Equal(t, outbound.ConstructMethod, closeMethod.Type)
	assert.Equal(t, "error", closeMethod.ReturnType)
}

// TestGoStructureParser_ParseGoStructFields_ComplexFields tests parsing of complex struct fields.
// This is a RED PHASE test that defines expected behavior for Go struct field parsing.
func TestGoStructureParser_ParseGoStructFields_ComplexFields(t *testing.T) {
	sourceCode := `package main

type ComplexStruct struct {
	// Basic fields
	ID   int
	Name string
	
	// Pointer field
	Parent *ComplexStruct
	
	// Slice field
	Items []Item
	
	// Map field
	Metadata map[string]interface{}
	
	// Channel field
	Events chan Event
	
	// Function field
	Validator func(interface{}) error
	
	// Anonymous field with tags
	User ` + "`json:\"-\" xml:\"user,omitempty\"`" + `
	
	// Private fields
	secret    string
	internal  bool
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	// Find struct type node
	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1)

	typeSpec := findChildByType(typeDecls[0], "type_spec")
	require.NotNil(t, typeSpec)

	structType := findChildByType(typeSpec, "struct_type")
	require.NotNil(t, structType)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoStructFields(parseTree, structType, "main", "ComplexStruct", options, time.Now())
	require.Len(t, result, 10, "Should parse 10 fields")

	// Test a few key fields
	expectedFields := map[string]struct {
		fieldType  string
		visibility outbound.VisibilityModifier
		tagCount   int
	}{
		"ID":        {"int", outbound.Public, 0},
		"Name":      {"string", outbound.Public, 0},
		"Parent":    {"*ComplexStruct", outbound.Public, 0},
		"Items":     {"[]Item", outbound.Public, 0},
		"Metadata":  {"map[string]interface{}", outbound.Public, 0},
		"Events":    {"chan Event", outbound.Public, 0},
		"Validator": {"func(interface{}) error", outbound.Public, 0},
		"User":      {"User", outbound.Public, 2}, // json and xml tags
		"secret":    {"string", outbound.Private, 0},
		"internal":  {"bool", outbound.Private, 0},
	}

	fieldMap := make(map[string]outbound.SemanticCodeChunk)
	for _, field := range result {
		fieldMap[field.Name] = field
	}

	for name, expected := range expectedFields {
		field, exists := fieldMap[name]
		require.True(t, exists, "Field %s should exist", name)

		assert.Equal(t, outbound.ConstructField, field.Type)
		assert.Equal(t, expected.fieldType, field.ReturnType)
		assert.Equal(t, expected.visibility, field.Visibility)
		assert.Len(t, field.Annotations, expected.tagCount)
		assert.Equal(t, "main.ComplexStruct."+name, field.QualifiedName)
	}
}

// TestGoStructureParser_ParseGoInterfaceMethods_ComplexMethods tests parsing of complex interface methods.
// This is a RED PHASE test that defines expected behavior for Go interface method parsing.
func TestGoStructureParser_ParseGoInterfaceMethods_ComplexMethods(t *testing.T) {
	sourceCode := `package main

type ComplexInterface interface {
	// Method with no parameters
	Connect() error
	
	// Method with multiple parameters and returns
	Process(data []byte, options ProcessOptions) (*Result, error)
	
	// Generic method
	Transform[T any](input T) (T, error)
	
	// Variadic method
	Log(level LogLevel, format string, args ...interface{})
	
	// Method with channel parameters
	Subscribe(events <-chan Event) <-chan Result
	
	// Method with function parameter
	Apply(fn func(interface{}) interface{}) error
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	// Find interface type node
	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1)

	typeSpec := findChildByType(typeDecls[0], "type_spec")
	require.NotNil(t, typeSpec)

	interfaceType := findChildByType(typeSpec, "interface_type")
	require.NotNil(t, interfaceType)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoInterfaceMethods(parseTree, interfaceType, "main", "ComplexInterface", options, time.Now())
	require.Len(t, result, 6, "Should parse 6 methods")

	// Test specific methods
	methodMap := make(map[string]outbound.SemanticCodeChunk)
	for _, method := range result {
		methodMap[method.Name] = method
	}

	// Test Connect method (no parameters)
	connect := methodMap["Connect"]
	assert.Equal(t, outbound.ConstructMethod, connect.Type)
	assert.Equal(t, "error", connect.ReturnType)
	assert.Empty(t, connect.Parameters)
	assert.True(t, connect.IsAbstract)

	// Test Process method (multiple params and returns)
	process := methodMap["Process"]
	assert.Equal(t, "(*Result, error)", process.ReturnType)
	require.Len(t, process.Parameters, 2)
	assert.Equal(t, "data", process.Parameters[0].Name)
	assert.Equal(t, "[]byte", process.Parameters[0].Type)
	assert.Equal(t, "options", process.Parameters[1].Name)
	assert.Equal(t, "ProcessOptions", process.Parameters[1].Type)

	// Test Log method (variadic)
	log := methodMap["Log"]
	require.Len(t, log.Parameters, 3)
	assert.Equal(t, "level", log.Parameters[0].Name)
	assert.Equal(t, "format", log.Parameters[1].Name)
	assert.Equal(t, "args", log.Parameters[2].Name)
	assert.True(t, log.Parameters[2].IsVariadic)
	assert.Equal(t, "interface{}", log.Parameters[2].Type)
}

// TestGoStructureParser_ParseGoFieldDeclaration_WithTags tests parsing of individual field declarations with tags.
// This is a RED PHASE test that defines expected behavior for Go field declaration parsing.
func TestGoStructureParser_ParseGoFieldDeclaration_WithTags(t *testing.T) {
	sourceCode := `package main

type TaggedStruct struct {
	SimpleField    string
	TaggedField    int    ` + "`json:\"tagged_field\" xml:\"TaggedField\" db:\"tagged_field,omitempty\"`" + `
	MultipleFields int, string
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	// Find struct type and first field declaration
	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1)

	typeSpec := findChildByType(typeDecls[0], "type_spec")
	structType := findChildByType(typeSpec, "struct_type")
	fieldDecls := findChildrenByType(structType, "field_declaration")
	require.Len(t, fieldDecls, 3, "Should find 3 field declarations")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test tagged field (second field declaration)
	taggedFieldResult := parser.ParseGoFieldDeclaration(
		parseTree,
		fieldDecls[1],
		"main",
		"TaggedStruct",
		options,
		time.Now(),
	)
	require.Len(t, taggedFieldResult, 1, "Should parse 1 field")

	field := taggedFieldResult[0]
	assert.Equal(t, "TaggedField", field.Name)
	assert.Equal(t, "int", field.ReturnType)

	// Validate annotations
	require.Len(t, field.Annotations, 3)

	annotationMap := make(map[string]outbound.Annotation)
	for _, ann := range field.Annotations {
		annotationMap[ann.Name] = ann
	}

	assert.Equal(t, []string{"tagged_field"}, annotationMap["json"].Arguments)
	assert.Equal(t, []string{"TaggedField"}, annotationMap["xml"].Arguments)
	assert.Equal(t, []string{"tagged_field", "omitempty"}, annotationMap["db"].Arguments)
}

// TestGoStructureParser_ErrorHandling tests error conditions for structure parsing.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestGoStructureParser_ErrorHandling(t *testing.T) {
	t.Run("nil parse tree should not panic", func(t *testing.T) {
		parser := NewGoStructureParser()

		result := parser.ParseGoStruct(
			context.Background(),
			nil,
			nil,
			nil,
			"main",
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("nil nodes should not panic", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, "package main")
		parser := NewGoStructureParser()

		result := parser.ParseGoStruct(
			context.Background(),
			parseTree,
			nil,
			nil,
			"main",
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("malformed struct should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
type Incomplete struct {
	Field
}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser := NewGoStructureParser()

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(typeDecls[0], "type_spec")
			if typeSpec != nil {
				result := parser.ParseGoStruct(
					context.Background(),
					parseTree,
					typeDecls[0],
					typeSpec,
					"main",
					outbound.SemanticExtractionOptions{},
					time.Now(),
				)
				// Should either return nil or a partial result, but not panic
				if result != nil {
					assert.NotEmpty(t, result.Name)
				}
			}
		}
	})
}

// TestGoStructureParser_VisibilityFiltering tests visibility filtering for structs and interfaces.
// This is a RED PHASE test that defines expected behavior for private struct/interface filtering.
func TestGoStructureParser_VisibilityFiltering(t *testing.T) {
	sourceCode := `package main

type PublicStruct struct {
	PublicField  string
	privateField int
}

type privateStruct struct {
	Field string
}

type PublicInterface interface {
	PublicMethod()
}

type privateInterface interface {
	method()
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoStructureParser()

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 4)

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  false,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Should filter out private struct
	privateStructTypeSpec := findChildByType(typeDecls[1], "type_spec")
	privateStructResult := parser.ParseGoStruct(
		context.Background(),
		parseTree,
		typeDecls[1],
		privateStructTypeSpec,
		"main",
		optionsNoPrivate,
		time.Now(),
	)
	assert.Nil(t, privateStructResult, "Private struct should be filtered out")

	// Should include public struct
	publicStructTypeSpec := findChildByType(typeDecls[0], "type_spec")
	publicStructResult := parser.ParseGoStruct(
		context.Background(),
		parseTree,
		typeDecls[0],
		publicStructTypeSpec,
		"main",
		optionsNoPrivate,
		time.Now(),
	)
	assert.NotNil(t, publicStructResult, "Public struct should be included")
	assert.Equal(t, "PublicStruct", publicStructResult.Name)

	// Should filter out private fields in public struct
	require.Len(t, publicStructResult.ChildChunks, 1, "Should only include public field")
	assert.Equal(t, "PublicField", publicStructResult.ChildChunks[0].Name)
}

// Helper functions - these will fail in RED phase as expected

// NewGoStructureParser creates a new Go structure parser - this will fail in RED phase.
func NewGoStructureParser() *GoStructureParser {
	panic("NewGoStructureParser not implemented - this is expected in RED phase")
}

// GoStructureParser represents the specialized Go structure parser - this will fail in RED phase.
type GoStructureParser struct{}

// ParseGoStruct method signature - this will fail in RED phase.
func (p *GoStructureParser) ParseGoStruct(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	panic("ParseGoStruct not implemented - this is expected in RED phase")
}

// ParseGoInterface method signature - this will fail in RED phase.
func (p *GoStructureParser) ParseGoInterface(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	typeDecl *valueobject.ParseNode,
	typeSpec *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	panic("ParseGoInterface not implemented - this is expected in RED phase")
}

// ParseGoStructFields method signature - this will fail in RED phase.
func (p *GoStructureParser) ParseGoStructFields(
	parseTree *valueobject.ParseTree,
	structType *valueobject.ParseNode,
	packageName string,
	structName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	panic("ParseGoStructFields not implemented - this is expected in RED phase")
}

// ParseGoInterfaceMethods method signature - this will fail in RED phase.
func (p *GoStructureParser) ParseGoInterfaceMethods(
	parseTree *valueobject.ParseTree,
	interfaceType *valueobject.ParseNode,
	packageName string,
	interfaceName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	panic("ParseGoInterfaceMethods not implemented - this is expected in RED phase")
}

// ParseGoFieldDeclaration method signature - this will fail in RED phase.
func (p *GoStructureParser) ParseGoFieldDeclaration(
	parseTree *valueobject.ParseTree,
	fieldDecl *valueobject.ParseNode,
	packageName string,
	structName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	panic("ParseGoFieldDeclaration not implemented - this is expected in RED phase")
}

// Helper functions that will be implemented in GREEN phase.
func findChildByType(node *valueobject.ParseNode, nodeType string) *valueobject.ParseNode {
	panic("findChildByType not implemented - this is expected in RED phase")
}

func findChildrenByType(node *valueobject.ParseNode, nodeType string) []*valueobject.ParseNode {
	panic("findChildrenByType not implemented - this is expected in RED phase")
}
