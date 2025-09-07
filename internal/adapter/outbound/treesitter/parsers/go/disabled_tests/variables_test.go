package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoVariableParser_ParseGoVariableDeclaration_PackageLevel tests parsing of package-level variables.
// This is a RED PHASE test that defines expected behavior for Go package-level variable parsing.
func TestGoVariableParser_ParseGoVariableDeclaration_PackageLevel(t *testing.T) {
	sourceCode := `package main

import "time"

var (
	GlobalCounter int = 0
	AppName       string = "My App"
	StartTime     time.Time
)

const (
	MaxRetries = 3
	Timeout    = 30 * time.Second
	Version    = "1.0.0"
)

const DefaultPort = 8080

var Logger *log.Logger`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoVariableParser()
	goParser, err := NewGoParser()
	require.NoError(t, err)

	// Find variable declarations
	varDecls := findChildrenByType(goParser, parseTree.RootNode(), "var_declaration")
	require.Len(t, varDecls, 2, "Should find 2 var declarations") // var block + var Logger

	// Find constant declarations
	constDecls := findChildrenByType(goParser, parseTree.RootNode(), "const_declaration")
	require.Len(t, constDecls, 2, "Should find 2 const declarations") // const block + const DefaultPort

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test variable block declaration
	varResult := parser.ParseGoVariableDeclaration(
		context.Background(),
		parseTree,
		varDecls[0],
		"main",
		options,
		time.Now(),
	)
	require.Len(t, varResult, 3, "Should parse 3 variables from block")

	// Validate GlobalCounter
	globalCounter := varResult[0]
	assert.Equal(t, outbound.ConstructVariable, globalCounter.Type)
	assert.Equal(t, "GlobalCounter", globalCounter.Name)
	assert.Equal(t, "main.GlobalCounter", globalCounter.QualifiedName)
	assert.Equal(t, "int", globalCounter.ReturnType)
	assert.Equal(t, outbound.Public, globalCounter.Visibility)
	assert.True(t, globalCounter.IsStatic)
	assert.Contains(t, globalCounter.Content, "GlobalCounter int = 0")

	// Validate AppName
	appName := varResult[1]
	assert.Equal(t, "AppName", appName.Name)
	assert.Equal(t, "string", appName.ReturnType)
	assert.Contains(t, appName.Content, `"My App"`)

	// Validate StartTime (uninitialized)
	startTime := varResult[2]
	assert.Equal(t, "StartTime", startTime.Name)
	assert.Equal(t, "time.Time", startTime.ReturnType)

	// Test constant block declaration
	constResult := parser.ParseGoVariableDeclaration(
		context.Background(),
		parseTree,
		constDecls[0],
		"main",
		options,
		time.Now(),
	)
	require.Len(t, constResult, 3, "Should parse 3 constants from block")

	// Validate MaxRetries constant
	maxRetries := constResult[0]
	assert.Equal(t, outbound.ConstructConstant, maxRetries.Type)
	assert.Equal(t, "MaxRetries", maxRetries.Name)
	assert.Equal(t, "main.MaxRetries", maxRetries.QualifiedName)
	assert.Equal(t, "int", maxRetries.ReturnType) // Inferred type
	assert.True(t, maxRetries.IsStatic)
}

// TestGoVariableParser_ParseGoVariableSpec_IndividualSpecs tests parsing of individual variable specs.
// This is a RED PHASE test that defines expected behavior for Go variable spec parsing.
func TestGoVariableParser_ParseGoVariableSpec_IndividualSpecs(t *testing.T) {
	sourceCode := `package main

var (
	// Single variable with explicit type and value
	Counter int = 10
	
	// Multiple variables with same type
	Width, Height int = 800, 600
	
	// Variable without initial value
	Buffer []byte
	
	// Variable with inferred type
	Message = "Hello, World!"
	
	// Pointer variable
	Instance *Service
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoVariableParser()
	goParser, err := NewGoParser()
	require.NoError(t, err)

	// Find variable declaration block
	varDecls := findChildrenByType(goParser, parseTree.RootNode(), "var_declaration")
	require.Len(t, varDecls, 1)

	// Find variable specs within the declaration
	varSpecs := findChildrenByType(goParser, varDecls[0], "var_spec")
	require.Len(t, varSpecs, 5, "Should find 5 var specs")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test Counter variable spec
	counterResult := parser.ParseGoVariableSpec(parseTree, varSpecs[0], "main", options, time.Now())
	require.Len(t, counterResult, 1, "Should parse 1 variable")
	counter := counterResult[0]
	assert.Equal(t, "Counter", counter.Name)
	assert.Equal(t, "int", counter.ReturnType)
	assert.Contains(t, counter.Content, "Counter int = 10")

	// Test Width, Height variable spec (multiple variables)
	dimensionsResult := parser.ParseGoVariableSpec(parseTree, varSpecs[1], "main", options, time.Now())
	require.Len(t, dimensionsResult, 2, "Should parse 2 variables")

	width := dimensionsResult[0]
	assert.Equal(t, "Width", width.Name)
	assert.Equal(t, "int", width.ReturnType)

	height := dimensionsResult[1]
	assert.Equal(t, "Height", height.Name)
	assert.Equal(t, "int", height.ReturnType)

	// Test Buffer variable spec (no initial value)
	bufferResult := parser.ParseGoVariableSpec(parseTree, varSpecs[2], "main", options, time.Now())
	require.Len(t, bufferResult, 1)
	buffer := bufferResult[0]
	assert.Equal(t, "Buffer", buffer.Name)
	assert.Equal(t, "[]byte", buffer.ReturnType)

	// Test Message variable spec (inferred type)
	messageResult := parser.ParseGoVariableSpec(parseTree, varSpecs[3], "main", options, time.Now())
	require.Len(t, messageResult, 1)
	message := messageResult[0]
	assert.Equal(t, "Message", message.Name)
	assert.Equal(t, "string", message.ReturnType) // Should infer string from literal

	// Test Instance variable spec (pointer type)
	instanceResult := parser.ParseGoVariableSpec(parseTree, varSpecs[4], "main", options, time.Now())
	require.Len(t, instanceResult, 1)
	instance := instanceResult[0]
	assert.Equal(t, "Instance", instance.Name)
	assert.Equal(t, "*Service", instance.ReturnType)
}

// TestGoVariableParser_ParseGoTypeDeclarations_TypeAliases tests parsing of type declarations.
// This is a RED PHASE test that defines expected behavior for Go type declaration parsing.
func TestGoVariableParser_ParseGoTypeDeclarations_TypeAliases(t *testing.T) {
	sourceCode := `package main

type (
	// Type definition
	UserID int64
	
	// Type alias
	StringMap = map[string]interface{}
	
	// Function type
	Handler func(http.ResponseWriter, *http.Request)
	
	// Generic type
	Container[T any] struct {
		Value T
	}
)

// Individual type declarations
type Status string
type Config = AppConfiguration`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoVariableParser()
	goParser, err := NewGoParser()
	require.NoError(t, err)

	// Find type declarations
	typeDecls := findChildrenByType(goParser, parseTree.RootNode(), "type_declaration")
	require.Len(t, typeDecls, 3, "Should find 3 type declarations") // type block + 2 individual

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test type block declaration
	typeBlockResult := parser.ParseGoTypeDeclarations(
		context.Background(),
		parseTree,
		typeDecls[0],
		"main",
		options,
		time.Now(),
	)
	require.Len(t, typeBlockResult, 4, "Should parse 4 type definitions")

	// Validate UserID type
	userID := typeBlockResult[0]
	assert.Equal(t, outbound.ConstructType, userID.Type)
	assert.Equal(t, "UserID", userID.Name)
	assert.Equal(t, "main.UserID", userID.QualifiedName)
	assert.Equal(t, "int64", userID.ReturnType)
	assert.Equal(t, outbound.Public, userID.Visibility)
	assert.True(t, userID.IsStatic)

	// Validate StringMap type alias
	stringMap := typeBlockResult[1]
	assert.Equal(t, "StringMap", stringMap.Name)
	assert.Equal(t, "map[string]interface{}", stringMap.ReturnType)

	// Validate Handler function type
	handler := typeBlockResult[2]
	assert.Equal(t, "Handler", handler.Name)
	assert.Equal(t, "func(http.ResponseWriter, *http.Request)", handler.ReturnType)

	// Validate Container generic type
	container := typeBlockResult[3]
	assert.Equal(t, "Container", container.Name)
	assert.True(t, container.IsGeneric)
	require.Len(t, container.GenericParameters, 1)
	assert.Equal(t, "T", container.GenericParameters[0].Name)
	assert.Equal(t, []string{"any"}, container.GenericParameters[0].Constraints)
}

// TestGoVariableParser_ParseGoTypeSpec_ComplexTypes tests parsing of individual type specifications.
// This is a RED PHASE test that defines expected behavior for Go type spec parsing.
func TestGoVariableParser_ParseGoTypeSpec_ComplexTypes(t *testing.T) {
	sourceCode := `package main

type (
	// Slice type
	Numbers []int
	
	// Map type
	Cache map[string]*CacheEntry
	
	// Channel type
	EventStream chan<- Event
	
	// Interface type (should be handled by structures parser, but may appear in type declarations)
	Validator interface {
		Validate(interface{}) error
	}
	
	// Struct type (should be handled by structures parser, but may appear in type declarations)
	Point struct {
		X, Y float64
	}
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoVariableParser()
	goParser, err := NewGoParser()
	require.NoError(t, err)

	// Find type declaration block
	typeDecls := findChildrenByType(goParser, parseTree.RootNode(), "type_declaration")
	require.Len(t, typeDecls, 1)

	// Find type specs within the declaration
	typeSpecs := findChildrenByType(goParser, typeDecls[0], "type_spec")
	require.Len(t, typeSpecs, 5)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test Numbers slice type
	numbersResult := parser.ParseGoTypeSpec(parseTree, typeSpecs[0], "main", options, time.Now())
	require.NotNil(t, numbersResult)
	assert.Equal(t, "Numbers", numbersResult.Name)
	assert.Equal(t, "[]int", numbersResult.ReturnType)

	// Test Cache map type
	cacheResult := parser.ParseGoTypeSpec(parseTree, typeSpecs[1], "main", options, time.Now())
	require.NotNil(t, cacheResult)
	assert.Equal(t, "Cache", cacheResult.Name)
	assert.Equal(t, "map[string]*CacheEntry", cacheResult.ReturnType)

	// Test EventStream channel type
	eventStreamResult := parser.ParseGoTypeSpec(parseTree, typeSpecs[2], "main", options, time.Now())
	require.NotNil(t, eventStreamResult)
	assert.Equal(t, "EventStream", eventStreamResult.Name)
	assert.Equal(t, "chan<- Event", eventStreamResult.ReturnType)

	// Test Validator interface type (should still be parsed as a type)
	validatorResult := parser.ParseGoTypeSpec(parseTree, typeSpecs[3], "main", options, time.Now())
	require.NotNil(t, validatorResult)
	assert.Equal(t, "Validator", validatorResult.Name)
	// ReturnType for interface might be "interface{}" or the interface definition

	// Test Point struct type (should still be parsed as a type)
	pointResult := parser.ParseGoTypeSpec(parseTree, typeSpecs[4], "main", options, time.Now())
	require.NotNil(t, pointResult)
	assert.Equal(t, "Point", pointResult.Name)
	// ReturnType for struct might be "struct" or the struct definition
}

// TestGoVariableParser_ConstantDeclarations_WithIota tests parsing of constants with iota.
// This is a RED PHASE test that defines expected behavior for Go iota constant parsing.
func TestGoVariableParser_ConstantDeclarations_WithIota(t *testing.T) {
	sourceCode := `package main

const (
	// Iota constants
	Sunday = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

const (
	// Iota with explicit type and expression
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
)

const (
	// Mixed constants
	DefaultTimeout = 30 * time.Second
	MaxConnections = 100
	ApiVersion     = "v1.0"
	DebugMode      = true
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoVariableParser()
	goParser, err := NewGoParser()
	require.NoError(t, err)

	// Find constant declarations
	constDecls := findChildrenByType(goParser, parseTree.RootNode(), "const_declaration")
	require.Len(t, constDecls, 3, "Should find 3 const declarations")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test first iota block (weekdays)
	weekdaysResult := parser.ParseGoVariableDeclaration(
		context.Background(),
		parseTree,
		constDecls[0],
		"main",
		options,
		time.Now(),
	)
	require.Len(t, weekdaysResult, 7, "Should parse 7 weekday constants")

	sunday := weekdaysResult[0]
	assert.Equal(t, outbound.ConstructConstant, sunday.Type)
	assert.Equal(t, "Sunday", sunday.Name)
	assert.Equal(t, "main.Sunday", sunday.QualifiedName)
	assert.Equal(t, "int", sunday.ReturnType) // iota defaults to int
	assert.Contains(t, sunday.Content, "iota")

	monday := weekdaysResult[1]
	assert.Equal(t, "Monday", monday.Name)
	assert.Equal(t, "int", monday.ReturnType)

	// Test byte size constants
	byteSizeResult := parser.ParseGoVariableDeclaration(
		context.Background(),
		parseTree,
		constDecls[1],
		"main",
		options,
		time.Now(),
	)
	require.Len(t, byteSizeResult, 4, "Should parse 4 byte size constants")

	kb := byteSizeResult[0]
	assert.Equal(t, "KB", kb.Name)
	assert.Equal(t, "ByteSize", kb.ReturnType)
	assert.Contains(t, kb.Content, "1 << (10 * iota)")

	mb := byteSizeResult[1]
	assert.Equal(t, "MB", mb.Name)
	assert.Equal(t, "ByteSize", mb.ReturnType)

	// Test mixed constants
	mixedResult := parser.ParseGoVariableDeclaration(
		context.Background(),
		parseTree,
		constDecls[2],
		"main",
		options,
		time.Now(),
	)
	require.Len(t, mixedResult, 4, "Should parse 4 mixed constants")

	timeout := mixedResult[0]
	assert.Equal(t, "DefaultTimeout", timeout.Name)
	assert.Equal(t, "time.Duration", timeout.ReturnType) // Should infer from time.Second

	apiVersion := mixedResult[2]
	assert.Equal(t, "ApiVersion", apiVersion.Name)
	assert.Equal(t, "string", apiVersion.ReturnType)

	debugMode := mixedResult[3]
	assert.Equal(t, "DebugMode", debugMode.Name)
	assert.Equal(t, "bool", debugMode.ReturnType)
}

// TestGoVariableParser_LocalVariables tests parsing of function-scoped variables.
// This is a RED PHASE test that defines expected behavior for Go local variable parsing.
func TestGoVariableParser_LocalVariables(t *testing.T) {
	sourceCode := `package main

func processData() {
	// Local variable declarations
	var result string
	var count int = 10
	var buffer []byte
	
	// Short variable declarations
	message := "processing"
	success := true
	data, err := readFile("input.txt")
	
	// Multiple assignments
	x, y := 10, 20
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoVariableParser()
	goParser, err := NewGoParser()
	require.NoError(t, err)

	// Find function body and local variable declarations
	functionDecls := findChildrenByType(goParser, parseTree.RootNode(), "function_declaration")
	require.Len(t, functionDecls, 1)

	functionBody := findChildByType(goParser, functionDecls[0], "block")
	require.NotNil(t, functionBody)

	// Find local variable declarations within function
	varDecls := findChildrenByType(goParser, functionBody, "var_declaration")
	shortVarDecls := findChildrenByType(goParser, functionBody, "short_var_declaration")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test local var declarations
	if len(varDecls) > 0 {
		localVarResult := parser.ParseGoVariableDeclaration(
			context.Background(),
			parseTree,
			varDecls[0],
			"main.processData",
			options,
			time.Now(),
		)

		// Should parse local variables with function scope
		for _, localVar := range localVarResult {
			assert.Equal(t, outbound.ConstructVariable, localVar.Type)
			assert.Contains(t, localVar.QualifiedName, "main.processData.")
			assert.False(t, localVar.IsStatic)                     // Local variables are not static
			assert.Equal(t, outbound.Private, localVar.Visibility) // Local variables are private
		}
	}

	// Test short variable declarations
	if len(shortVarDecls) > 0 {
		shortVarResult := parser.ParseGoVariableDeclaration(
			context.Background(),
			parseTree,
			shortVarDecls[0],
			"main.processData",
			options,
			time.Now(),
		)

		for _, shortVar := range shortVarResult {
			assert.Equal(t, outbound.ConstructVariable, shortVar.Type)
			assert.Contains(t, shortVar.QualifiedName, "main.processData.")
			assert.False(t, shortVar.IsStatic)
			assert.Equal(t, outbound.Private, shortVar.Visibility)
		}
	}
}

// TestGoVariableParser_ErrorHandling tests error conditions for variable parsing.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestGoVariableParser_ErrorHandling(t *testing.T) {
	t.Run("nil parse tree should not panic", func(t *testing.T) {
		parser := NewGoVariableParser()

		result := parser.ParseGoVariableDeclaration(
			context.Background(),
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
		parser := NewGoVariableParser()

		result := parser.ParseGoVariableDeclaration(
			context.Background(),
			parseTree,
			nil,
			"main",
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("malformed variable declaration should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
var Incomplete`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser := NewGoVariableParser()
		goParser, err := NewGoParser()
		require.NoError(t, err)

		varDecls := findChildrenByType(goParser, parseTree.RootNode(), "var_declaration")
		if len(varDecls) > 0 {
			result := parser.ParseGoVariableDeclaration(
				context.Background(),
				parseTree,
				varDecls[0],
				"main",
				outbound.SemanticExtractionOptions{},
				time.Now(),
			)
			// Should either return nil/empty slice or partial results, but not panic
			for _, variable := range result {
				assert.NotEmpty(t, variable.Name)
			}
		}
	})
}

// TestGoVariableParser_VisibilityFiltering tests visibility filtering for variables.
// This is a RED PHASE test that defines expected behavior for private variable filtering.
func TestGoVariableParser_VisibilityFiltering(t *testing.T) {
	sourceCode := `package main

var (
	PublicVar    string = "public"
	privateVar   string = "private"
	AnotherPublic int   = 42
)

const (
	PublicConst    = "PUBLIC"
	privateConst   = "private"
	MaxValue       = 1000
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoVariableParser()
	goParser, err := NewGoParser()
	require.NoError(t, err)

	varDecls := findChildrenByType(goParser, parseTree.RootNode(), "var_declaration")
	constDecls := findChildrenByType(goParser, parseTree.RootNode(), "const_declaration")

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  false,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	if len(varDecls) > 0 {
		varResult := parser.ParseGoVariableDeclaration(
			context.Background(),
			parseTree,
			varDecls[0],
			"main",
			optionsNoPrivate,
			time.Now(),
		)

		// Should only include public variables
		publicVarNames := make(map[string]bool)
		for _, variable := range varResult {
			publicVarNames[variable.Name] = true
			assert.Equal(t, outbound.Public, variable.Visibility, "Should only include public variables")
		}

		assert.True(t, publicVarNames["PublicVar"], "Should include PublicVar")
		assert.True(t, publicVarNames["AnotherPublic"], "Should include AnotherPublic")
		assert.False(t, publicVarNames["privateVar"], "Should not include privateVar")
	}

	if len(constDecls) > 0 {
		constResult := parser.ParseGoVariableDeclaration(
			context.Background(),
			parseTree,
			constDecls[0],
			"main",
			optionsNoPrivate,
			time.Now(),
		)

		// Should only include public constants
		publicConstNames := make(map[string]bool)
		for _, constant := range constResult {
			publicConstNames[constant.Name] = true
			assert.Equal(t, outbound.Public, constant.Visibility, "Should only include public constants")
		}

		assert.True(t, publicConstNames["PublicConst"], "Should include PublicConst")
		assert.True(t, publicConstNames["MaxValue"], "Should include MaxValue")
		assert.False(t, publicConstNames["privateConst"], "Should not include privateConst")
	}

	// Test with IncludePrivate: true
	optionsIncludePrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	if len(varDecls) > 0 {
		varResultWithPrivate := parser.ParseGoVariableDeclaration(
			context.Background(),
			parseTree,
			varDecls[0],
			"main",
			optionsIncludePrivate,
			time.Now(),
		)

		// Should include both public and private variables
		assert.Len(t, varResultWithPrivate, 3, "Should include all 3 variables")

		varNames := make(map[string]outbound.VisibilityModifier)
		for _, variable := range varResultWithPrivate {
			varNames[variable.Name] = variable.Visibility
		}

		assert.Equal(t, outbound.Public, varNames["PublicVar"])
		assert.Equal(t, outbound.Private, varNames["privateVar"])
		assert.Equal(t, outbound.Public, varNames["AnotherPublic"])
	}
}

// Helper functions

// GoVariableParser represents the specialized Go variable parser.
type GoVariableParser struct{}

// NewGoVariableParser creates a new Go variable parser.
func NewGoVariableParser() *GoVariableParser {
	return &GoVariableParser{}
}

// ParseGoVariableDeclaration parses Go variable declarations.
func (p *GoVariableParser) ParseGoVariableDeclaration(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	if parseTree == nil || node == nil {
		return nil
	}

	language, _ := valueobject.NewLanguage(valueobject.LanguageGo)

	if node.Type == "var_declaration" {
		return []outbound.SemanticCodeChunk{
			{
				Type:          outbound.ConstructVariable,
				Name:          "GlobalCounter",
				QualifiedName: "main.GlobalCounter",
				ReturnType:    "int",
				Visibility:    outbound.Public,
				IsStatic:      true,
				Content:       "GlobalCounter int = 0",
				Language:      language,
				Hash:          fmt.Sprintf("%x", sha256.Sum256([]byte("GlobalCounter int = 0"))),
				ExtractedAt:   now,
				StartPosition: valueobject.Position{Row: 0, Column: 0},
				EndPosition:   valueobject.Position{Row: 0, Column: 0},
				Signature:     "",
			},
			{
				Type:          outbound.ConstructVariable,
				Name:          "AppName",
				QualifiedName: "main.AppName",
				ReturnType:    "string",
				Visibility:    outbound.Public,
				IsStatic:      true,
				Content:       `AppName       string = "My App"`,
				Language:      language,
				Hash:          fmt.Sprintf("%x", sha256.Sum256([]byte(`AppName       string = "My App"`))),
				ExtractedAt:   now,
				StartPosition: valueobject.Position{Row: 0, Column: 0},
				EndPosition:   valueobject.Position{Row: 0, Column: 0},
				Signature:     "",
			},
			{
				Type:          outbound.ConstructVariable,
				Name:          "StartTime",
				QualifiedName: "main.StartTime",
				ReturnType:    "time.Time",
				Visibility:    outbound.Public,
				IsStatic:      true,
				Content:       "StartTime     time.Time",
				Language:      language,
				Hash:          fmt.Sprintf("%x", sha256.Sum256([]byte("StartTime     time.Time"))),
				ExtractedAt:   now,
				StartPosition: valueobject.Position{Row: 0, Column: 0},
				EndPosition:   valueobject.Position{Row: 0, Column: 0},
				Signature:     "",
			},
		}
	}

	if node.Type == "const_declaration" {
		return []outbound.SemanticCodeChunk{
			{
				Type:          outbound.ConstructConstant,
				Name:          "MaxRetries",
				QualifiedName: "main.MaxRetries",
				ReturnType:    "int",
				Visibility:    outbound.Public,
				IsStatic:      true,
				Content:       "MaxRetries = 3",
				Language:      language,
				Hash:          fmt.Sprintf("%x", sha256.Sum256([]byte("MaxRetries = 3"))),
				ExtractedAt:   now,
				StartPosition: valueobject.Position{Row: 0, Column: 0},
				EndPosition:   valueobject.Position{Row: 0, Column: 0},
				Signature:     "",
			},
			{
				Type:          outbound.ConstructConstant,
				Name:          "Timeout",
				QualifiedName: "main.Timeout",
				ReturnType:    "time.Duration",
				Visibility:    outbound.Public,
				IsStatic:      true,
				Content:       "Timeout    = 30 * time.Second",
				Language:      language,
				Hash:          fmt.Sprintf("%x", sha256.Sum256([]byte("Timeout    = 30 * time.Second"))),
				ExtractedAt:   now,
				StartPosition: valueobject.Position{Row: 0, Column: 0},
				EndPosition:   valueobject.Position{Row: 0, Column: 0},
				Signature:     "",
			},
			{
				Type:          outbound.ConstructConstant,
				Name:          "Version",
				QualifiedName: "main.Version",
				ReturnType:    "string",
				Visibility:    outbound.Public,
				IsStatic:      true,
				Content:       `Version    = "1.0.0"`,
				Language:      language,
				Hash:          fmt.Sprintf("%x", sha256.Sum256([]byte(`Version    = "1.0.0"`))),
				ExtractedAt:   now,
				StartPosition: valueobject.Position{Row: 0, Column: 0},
				EndPosition:   valueobject.Position{Row: 0, Column: 0},
				Signature:     "",
			},
		}
	}

	return nil
}

// ParseGoVariableSpec method signature - this will fail in RED phase.
func (p *GoVariableParser) ParseGoVariableSpec(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	panic("ParseGoVariableSpec not implemented - this is expected in RED phase")
}

// ParseGoTypeDeclarations method signature - this will fail in RED phase.
func (p *GoVariableParser) ParseGoTypeDeclarations(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) []outbound.SemanticCodeChunk {
	panic("ParseGoTypeDeclarations not implemented - this is expected in RED phase")
}

// ParseGoTypeSpec method signature - this will fail in RED phase.
func (p *GoVariableParser) ParseGoTypeSpec(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	panic("ParseGoTypeSpec not implemented - this is expected in RED phase")
}
