package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSemanticTraverserAdapter_ExtractGoFunctions_RegularFunctions tests regular Go function extraction.
// This is a RED PHASE test that defines expected behavior for basic Go function parsing.
func TestSemanticTraverserAdapter_ExtractGoFunctions_RegularFunctions(t *testing.T) {
	sourceCode := `package main

import "fmt"

func Add(a int, b int) int {
	return a + b
}

func main() {
	fmt.Println("Hello, World!")
}`

	expectedFuncs := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructFunction,
			Name:          "Add",
			QualifiedName: "main.Add",
			Content:       "func Add(a int, b int) int {\n\treturn a + b\n}",
			Parameters: []outbound.Parameter{
				{Name: "a", Type: "int", IsOptional: false, IsVariadic: false},
				{Name: "b", Type: "int", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "int",
			Visibility: outbound.Public, // Capitalized name
			IsStatic:   false,
			IsAsync:    false,
			IsAbstract: false,
			IsGeneric:  false,
			StartByte:  22, // Approximately where func starts
			EndByte:    49, // Approximately where func ends
		},
		{
			Type:          outbound.ConstructFunction,
			Name:          "main",
			QualifiedName: "main.main",
			Content:       `func main() {\n\tfmt.Println("Hello, World!")\n}`,
			Parameters:    []outbound.Parameter{},
			ReturnType:    "",
			Visibility:    outbound.Private, // Lowercase name
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    false,
			IsGeneric:     false,
			StartByte:     51, // Approximately where func starts
			EndByte:       86, // Approximately where func ends
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeComments:      true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
		PreservationStrategy: outbound.PreserveModerate,
	}

	testGoFunctionExtraction(t, sourceCode, expectedFuncs, options)
}

// TestSemanticTraverserAdapter_ExtractGoFunctions_Methods tests Go method extraction.
// This is a RED PHASE test that defines expected behavior for Go method parsing.
func TestSemanticTraverserAdapter_ExtractGoFunctions_Methods(t *testing.T) {
	sourceCode := `package main

type Calculator struct {
	result float64
}

func (c *Calculator) Add(value float64) float64 {
	c.result += value
	return c.result
}

func (c Calculator) GetResult() float64 {
	return c.result
}`

	expectedFuncs := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructMethod,
			Name:          "Add",
			QualifiedName: "main.Calculator.Add",
			Content:       "func (c *Calculator) Add(value float64) float64 {\n\tc.result += value\n\treturn c.result\n}",
			Parameters: []outbound.Parameter{
				{Name: "c", Type: "*Calculator", IsOptional: false, IsVariadic: false},
				{Name: "value", Type: "float64", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "float64",
			Visibility: outbound.Public,
			IsStatic:   false,
			IsAsync:    false,
			IsAbstract: false,
			IsGeneric:  false,
		},
		{
			Type:          outbound.ConstructMethod,
			Name:          "GetResult",
			QualifiedName: "main.Calculator.GetResult",
			Content:       "func (c Calculator) GetResult() float64 {\n\treturn c.result\n}",
			Parameters: []outbound.Parameter{
				{Name: "c", Type: "Calculator", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "float64",
			Visibility: outbound.Public,
			IsStatic:   false,
			IsAsync:    false,
			IsAbstract: false,
			IsGeneric:  false,
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoFunctionExtraction(t, sourceCode, expectedFuncs, options)
}

// TestSemanticTraverserAdapter_ExtractGoFunctions_Generics tests generic Go function extraction.
// This is a RED PHASE test that defines expected behavior for Go generic function parsing.
func TestSemanticTraverserAdapter_ExtractGoFunctions_Generics(t *testing.T) {
	sourceCode := `package main

func Map[T any, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

func Identity[T comparable](value T) T {
	return value
}`

	expectedFuncs := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructFunction,
			Name:          "Map",
			QualifiedName: "main.Map",
			Content:       "func Map[T any, U any](slice []T, fn func(T) U) []U {\n\tresult := make([]U, len(slice))\n\tfor i, v := range slice {\n\t\tresult[i] = fn(v)\n\t}\n\treturn result\n}",
			Parameters: []outbound.Parameter{
				{Name: "slice", Type: "[]T", IsOptional: false, IsVariadic: false},
				{Name: "fn", Type: "func(T) U", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "[]U",
			Visibility: outbound.Public,
			IsStatic:   false,
			IsAsync:    false,
			IsAbstract: false,
			IsGeneric:  true,
			GenericParameters: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"any"}},
				{Name: "U", Constraints: []string{"any"}},
			},
		},
		{
			Type:          outbound.ConstructFunction,
			Name:          "Identity",
			QualifiedName: "main.Identity",
			Content:       "func Identity[T comparable](value T) T {\n\treturn value\n}",
			Parameters: []outbound.Parameter{
				{Name: "value", Type: "T", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "T",
			Visibility: outbound.Public,
			IsStatic:   false,
			IsAsync:    false,
			IsAbstract: false,
			IsGeneric:  true,
			GenericParameters: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"comparable"}},
			},
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoFunctionExtraction(t, sourceCode, expectedFuncs, options)
}

// TestSemanticTraverserAdapter_ExtractGoFunctions_Variadic tests variadic Go function extraction.
// This is a RED PHASE test that defines expected behavior for Go variadic function parsing.
func TestSemanticTraverserAdapter_ExtractGoFunctions_Variadic(t *testing.T) {
	sourceCode := `package main

import "fmt"

func Printf(format string, args ...interface{}) (int, error) {
	return fmt.Printf(format, args...)
}

func Sum(numbers ...int) int {
	total := 0
	for _, num := range numbers {
		total += num
	}
	return total
}`

	expectedFuncs := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructFunction,
			Name:          "Printf",
			QualifiedName: "main.Printf",
			Content:       "func Printf(format string, args ...interface{}) (int, error) {\n\treturn fmt.Printf(format, args...)\n}",
			Parameters: []outbound.Parameter{
				{Name: "format", Type: "string", IsOptional: false, IsVariadic: false},
				{Name: "args", Type: "interface{}", IsOptional: false, IsVariadic: true},
			},
			ReturnType: "(int, error)",
			Visibility: outbound.Public,
			IsStatic:   false,
			IsAsync:    false,
			IsAbstract: false,
			IsGeneric:  false,
		},
		{
			Type:          outbound.ConstructFunction,
			Name:          "Sum",
			QualifiedName: "main.Sum",
			Content:       "func Sum(numbers ...int) int {\n\ttotal := 0\n\tfor _, num := range numbers {\n\t\ttotal += num\n\t}\n\treturn total\n}",
			Parameters: []outbound.Parameter{
				{Name: "numbers", Type: "int", IsOptional: false, IsVariadic: true},
			},
			ReturnType: "int",
			Visibility: outbound.Public,
			IsStatic:   false,
			IsAsync:    false,
			IsAbstract: false,
			IsGeneric:  false,
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoFunctionExtraction(t, sourceCode, expectedFuncs, options)
}

// TestSemanticTraverserAdapter_ExtractGoFunctions_WithDocumentation tests documented function extraction.
// This is a RED PHASE test that defines expected behavior for Go function documentation parsing.
func TestSemanticTraverserAdapter_ExtractGoFunctions_WithDocumentation(t *testing.T) {
	sourceCode := `package main

// Add performs addition of two integers and returns the result.
// It takes two parameters: a and b, both of type int.
// Returns the sum as an int.
func Add(a int, b int) int {
	return a + b
}`

	expectedFuncs := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructFunction,
			Name:          "Add",
			QualifiedName: "main.Add",
			Content:       "func Add(a int, b int) int {\n\treturn a + b\n}",
			Documentation: "Add performs addition of two integers and returns the result.\nIt takes two parameters: a and b, both of type int.\nReturns the sum as an int.",
			Parameters: []outbound.Parameter{
				{Name: "a", Type: "int", IsOptional: false, IsVariadic: false},
				{Name: "b", Type: "int", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "int",
			Visibility: outbound.Public,
			IsStatic:   false,
			IsAsync:    false,
			IsAbstract: false,
			IsGeneric:  false,
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeComments:      true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	testGoFunctionExtraction(t, sourceCode, expectedFuncs, options)
}

// testGoFunctionExtraction is a helper function to reduce code duplication in function tests.
func testGoFunctionExtraction(
	t *testing.T,
	sourceCode string,
	expectedFuncs []outbound.SemanticCodeChunk,
	options outbound.SemanticExtractionOptions,
) {
	// Create language
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	// Create parse tree from source code
	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	// Create adapter
	parserFactory := &ParserFactoryImpl{}
	adapter := NewSemanticTraverserAdapter(parserFactory)

	// Extract functions
	result, err := adapter.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, len(expectedFuncs), "Number of extracted functions should match expected")

	// Validate each extracted function
	for i, expected := range expectedFuncs {
		actual := result[i]
		validateSemanticCodeChunk(t, expected, actual)
	}
}

// TestSemanticTraverserAdapter_ExtractGoStructs_Basic tests basic Go struct extraction.
// This is a RED PHASE test that defines expected behavior for basic Go struct parsing.
func TestSemanticTraverserAdapter_ExtractGoStructs_Basic(t *testing.T) {
	sourceCode := `package main

type User struct {
	ID       int    ` + "`json:\"id\" db:\"id\"`" + `
	Name     string ` + "`json:\"name\" db:\"name\"`" + `
	Email    string ` + "`json:\"email\" db:\"email\"`" + `
	Age      int    ` + "`json:\"age\" db:\"age\"`" + `
	IsActive bool   ` + "`json:\"is_active\" db:\"is_active\"`" + `
}`

	expectedStructs := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructStruct,
			Name:          "User",
			QualifiedName: "main.User",
			Content:       "type User struct {\n\tID       int    `json:\"id\" db:\"id\"`\n\tName     string `json:\"name\" db:\"name\"`\n\tEmail    string `json:\"email\" db:\"email\"`\n\tAge      int    `json:\"age\" db:\"age\"`\n\tIsActive bool   `json:\"is_active\" db:\"is_active\"`\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    false,
			IsGeneric:     false,
			ChildChunks:   createExpectedUserStructFields(),
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoStructExtraction(t, sourceCode, expectedStructs, options)
}

// TestSemanticTraverserAdapter_ExtractGoStructs_Embedded tests embedded struct extraction.
// This is a RED PHASE test that defines expected behavior for Go embedded struct parsing.
func TestSemanticTraverserAdapter_ExtractGoStructs_Embedded(t *testing.T) {
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

	expectedStructs := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructStruct,
			Name:          "Person",
			QualifiedName: "main.Person",
			Content:       "type Person struct {\n\tName string\n\tAge  int\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    false,
			IsGeneric:     false,
			ChildChunks:   createExpectedPersonStructFields(),
		},
		{
			Type:          outbound.ConstructStruct,
			Name:          "Employee",
			QualifiedName: "main.Employee",
			Content:       "type Employee struct {\n\tPerson          // Embedded struct\n\tEmployeeID int  `json:\"employee_id\"`\n\tDepartment string\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    false,
			IsGeneric:     false,
			ChildChunks:   createExpectedEmployeeStructFields(),
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoStructExtraction(t, sourceCode, expectedStructs, options)
}

// TestSemanticTraverserAdapter_ExtractGoStructs_Generic tests generic struct extraction.
// This is a RED PHASE test that defines expected behavior for Go generic struct parsing.
func TestSemanticTraverserAdapter_ExtractGoStructs_Generic(t *testing.T) {
	sourceCode := `package main

type Container[T any] struct {
	value T
	count int
}

type Pair[K comparable, V any] struct {
	Key   K ` + "`json:\"key\"`" + `
	Value V ` + "`json:\"value\"`" + `
}`

	expectedStructs := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructStruct,
			Name:          "Container",
			QualifiedName: "main.Container",
			Content:       "type Container[T any] struct {\n\tvalue T\n\tcount int\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    false,
			IsGeneric:     true,
			GenericParameters: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"any"}},
			},
			ChildChunks: createExpectedContainerStructFields(),
		},
		{
			Type:          outbound.ConstructStruct,
			Name:          "Pair",
			QualifiedName: "main.Pair",
			Content:       "type Pair[K comparable, V any] struct {\n\tKey   K `json:\"key\"`\n\tValue V `json:\"value\"`\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    false,
			IsGeneric:     true,
			GenericParameters: []outbound.GenericParameter{
				{Name: "K", Constraints: []string{"comparable"}},
				{Name: "V", Constraints: []string{"any"}},
			},
			ChildChunks: createExpectedPairStructFields(),
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoStructExtraction(t, sourceCode, expectedStructs, options)
}

// testGoStructExtraction is a helper function to reduce code duplication in struct tests.
func testGoStructExtraction(
	t *testing.T,
	sourceCode string,
	expectedStructs []outbound.SemanticCodeChunk,
	options outbound.SemanticExtractionOptions,
) {
	// Create language
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	// Create parse tree from source code
	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	// Create adapter
	parserFactory := &ParserFactoryImpl{}
	adapter := NewSemanticTraverserAdapter(parserFactory)

	// Extract structs (via ExtractClasses which handles structs in Go)
	result, err := adapter.ExtractClasses(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, len(expectedStructs), "Number of extracted structs should match expected")

	// Validate each extracted struct
	for i, expected := range expectedStructs {
		actual := result[i]
		validateSemanticCodeChunk(t, expected, actual)
	}
}

// TestSemanticTraverserAdapter_ExtractGoInterfaces_Basic tests basic Go interface extraction.
// This is a RED PHASE test that defines expected behavior for basic Go interface parsing.
func TestSemanticTraverserAdapter_ExtractGoInterfaces_Basic(t *testing.T) {
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

	expectedInterfaces := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructInterface,
			Name:          "Writer",
			QualifiedName: "main.Writer",
			Content:       "type Writer interface {\n\tWrite(data []byte) (int, error)\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    true,
			IsGeneric:     false,
			ChildChunks:   createExpectedWriterInterfaceMethods(),
		},
		{
			Type:          outbound.ConstructInterface,
			Name:          "Reader",
			QualifiedName: "main.Reader",
			Content:       "type Reader interface {\n\tRead(buffer []byte) (int, error)\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    true,
			IsGeneric:     false,
			ChildChunks:   createExpectedReaderInterfaceMethods(),
		},
		{
			Type:          outbound.ConstructInterface,
			Name:          "ReadWriter",
			QualifiedName: "main.ReadWriter",
			Content:       "type ReadWriter interface {\n\tReader\n\tWriter\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    true,
			IsGeneric:     false,
			ChildChunks:   createExpectedReadWriterInterfaceMethods(),
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoInterfaceExtraction(t, sourceCode, expectedInterfaces, options)
}

// TestSemanticTraverserAdapter_ExtractGoInterfaces_Generic tests generic interface extraction.
// This is a RED PHASE test that defines expected behavior for Go generic interface parsing.
func TestSemanticTraverserAdapter_ExtractGoInterfaces_Generic(t *testing.T) {
	sourceCode := `package main

type Comparable[T any] interface {
	CompareTo(other T) int
}

type Container[T any] interface {
	Add(item T) bool
	Get(index int) (T, error)
	Size() int
}`

	expectedInterfaces := []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructInterface,
			Name:          "Comparable",
			QualifiedName: "main.Comparable",
			Content:       "type Comparable[T any] interface {\n\tCompareTo(other T) int\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    true,
			IsGeneric:     true,
			GenericParameters: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"any"}},
			},
			ChildChunks: createExpectedComparableInterfaceMethods(),
		},
		{
			Type:          outbound.ConstructInterface,
			Name:          "Container",
			QualifiedName: "main.Container",
			Content:       "type Container[T any] interface {\n\tAdd(item T) bool\n\tGet(index int) (T, error)\n\tSize() int\n}",
			Visibility:    outbound.Public,
			IsStatic:      false,
			IsAsync:       false,
			IsAbstract:    true,
			IsGeneric:     true,
			GenericParameters: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"any"}},
			},
			ChildChunks: createExpectedContainerInterfaceMethods(),
		},
	}

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoInterfaceExtraction(t, sourceCode, expectedInterfaces, options)
}

// testGoInterfaceExtraction is a helper function to reduce code duplication in interface tests.
func testGoInterfaceExtraction(
	t *testing.T,
	sourceCode string,
	expectedInterfaces []outbound.SemanticCodeChunk,
	options outbound.SemanticExtractionOptions,
) {
	// Create language
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	// Create parse tree from source code
	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	// Create adapter
	parserFactory := &ParserFactoryImpl{}
	adapter := NewSemanticTraverserAdapter(parserFactory)

	// Extract interfaces
	result, err := adapter.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, len(expectedInterfaces), "Number of extracted interfaces should match expected")

	// Validate each extracted interface
	for i, expected := range expectedInterfaces {
		actual := result[i]
		validateSemanticCodeChunk(t, expected, actual)
	}
}

// TestSemanticTraverserAdapter_ExtractGoVariables_PackageLevel tests package-level variable extraction.
// This is a RED PHASE test that defines expected behavior for Go package-level variable parsing.
func TestSemanticTraverserAdapter_ExtractGoVariables_PackageLevel(t *testing.T) {
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

var Logger *log.Logger

type CustomType int

type UserID = int64`

	expectedVariables := createExpectedPackageLevelVariables()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoVariableExtraction(t, sourceCode, expectedVariables, options)
}

// TestSemanticTraverserAdapter_ExtractGoImports_Individual tests individual import extraction.
// This is a RED PHASE test that defines expected behavior for Go individual import parsing.
func TestSemanticTraverserAdapter_ExtractGoImports_Individual(t *testing.T) {
	sourceCode := `package main

import "fmt"
import "strings"

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)`

	expectedImports := createExpectedIndividualImports()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoImportExtraction(t, sourceCode, expectedImports, options)
}

// TestSemanticTraverserAdapter_ExtractGoImports_WithAliases tests import extraction with aliases.
// This is a RED PHASE test that defines expected behavior for Go import alias parsing.
func TestSemanticTraverserAdapter_ExtractGoImports_WithAliases(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"
	f "fmt"              // Alias
	"context"
	ctx "context"        // Alias
	_ "github.com/lib/pq" // Blank import
	. "math"             // Dot import (wildcard)
	"net/http"
	h "net/http"         // Alias
)`

	expectedImports := createExpectedAliasedImports()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	testGoImportExtraction(t, sourceCode, expectedImports, options)
}

// TestSemanticTraverserAdapter_ExtractGoPackages_Main tests main package extraction.
// This is a RED PHASE test that defines expected behavior for Go main package parsing.
func TestSemanticTraverserAdapter_ExtractGoPackages_Main(t *testing.T) {
	sourceCode := `// Package main implements a comprehensive example of Go language constructs
// for testing the semantic parser implementation.
package main

import (
	"fmt"
	"strings"
)

const Version = "1.0.0"

type User struct {
	Name string
	ID   int
}

func (u User) String() string {
	return fmt.Sprintf("User{Name: %s, ID: %d}", u.Name, u.ID)
}

func main() {
	user := User{Name: "John", ID: 1}
	fmt.Println(user.String())
}`

	expectedPackages := createExpectedMainPackage()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeComments:      true,
		IncludeDocumentation: true,
		IncludeMetadata:      true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	testGoPackageExtraction(t, sourceCode, expectedPackages, options)
}

// TestSemanticTraverserAdapter_FailsAgainstMockImplementation verifies that tests fail against current mock.
// This is a RED PHASE test that ensures the current mock implementation fails the new comprehensive tests.
func TestSemanticTraverserAdapter_FailsAgainstMockImplementation(t *testing.T) {
	testCurrentMockFailsForFunctions(t)
	testCurrentMockFailsForStructs(t)
	testCurrentMockFailsForInterfaces(t)
	testCurrentMockFailsForVariables(t)
	testCurrentMockFailsForImports(t)
	testCurrentMockFailsForPackages(t)
}

// Helper functions for testing different mock failures.
func testCurrentMockFailsForFunctions(t *testing.T) {
	t.Run("current mock implementation should fail comprehensive function extraction", func(t *testing.T) {
		sourceCode := `package main

func Add(a int, b int) int {
	return a + b
}

func Subtract(a int, b int) int {
	return a - b
}

func main() {
	result := Add(5, 3)
	fmt.Println(result)
}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parserFactory := &ParserFactoryImpl{}
		adapter := NewSemanticTraverserAdapter(parserFactory)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeTypeInfo: true,
			MaxDepth:        10,
		}

		result, err := adapter.ExtractFunctions(context.Background(), parseTree, options)
		require.NoError(t, err)

		// Current mock implementation returns hardcoded "main" function
		assert.Len(t, result, 1, "Mock implementation returns only 1 hardcoded function")
		assert.Equal(t, "main", result[0].Name, "Mock returns hardcoded 'main' function name")
		assert.Equal(t, "main.main", result[0].QualifiedName, "Mock returns hardcoded qualified name")
		assert.Equal(t, "func main() {\n\t// Implementation\n}", result[0].Content, "Mock returns hardcoded content")

		t.Logf(
			"EXPECTED FAILURE: Mock implementation only returns 1 hardcoded function instead of parsing actual 3 functions from source",
		)
		t.Logf("Real implementation needs to parse: Add(int,int)int, Subtract(int,int)int, main()void")
	})
}

func testCurrentMockFailsForStructs(t *testing.T) {
	t.Run("current mock implementation should fail comprehensive struct extraction", func(t *testing.T) {
		sourceCode := `package main

type User struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type Address struct {
	Street string
	City   string
}

type Employee struct {
	User
	Address Address
	Salary  float64
}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parserFactory := &ParserFactoryImpl{}
		adapter := NewSemanticTraverserAdapter(parserFactory)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeTypeInfo: true,
			MaxDepth:        10,
		}

		result, err := adapter.ExtractClasses(context.Background(), parseTree, options)
		require.NoError(t, err)

		// Current mock implementation returns hardcoded "User" struct
		assert.Len(t, result, 1, "Mock implementation returns only 1 hardcoded struct")
		assert.Equal(t, "User", result[0].Name, "Mock returns hardcoded 'User' struct name")
		assert.Equal(t, "main.User", result[0].QualifiedName, "Mock returns hardcoded qualified name")

		t.Logf(
			"EXPECTED FAILURE: Mock implementation only returns 1 hardcoded struct instead of parsing actual 3 structs from source",
		)
		t.Logf("Real implementation needs to parse: User{ID,Name}, Address{Street,City}, Employee{User,Address,Salary}")
	})
}

func testCurrentMockFailsForInterfaces(t *testing.T) {
	t.Run("current mock implementation should fail interface extraction completely", func(t *testing.T) {
		sourceCode := `package main

type Writer interface {
	Write([]byte) (int, error)
}

type Reader interface {
	Read([]byte) (int, error)
}

type ReadWriter interface {
	Reader
	Writer
}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parserFactory := &ParserFactoryImpl{}
		adapter := NewSemanticTraverserAdapter(parserFactory)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeTypeInfo: true,
			MaxDepth:        10,
		}

		result, err := adapter.ExtractInterfaces(context.Background(), parseTree, options)
		require.NoError(t, err)

		// Current mock implementation returns empty slice for interfaces
		assert.Empty(t, result, "Mock implementation returns empty slice for interfaces")

		t.Logf(
			"EXPECTED FAILURE: Mock implementation returns no interfaces instead of parsing actual 3 interfaces from source",
		)
		t.Logf("Real implementation needs to parse: Writer{Write}, Reader{Read}, ReadWriter{Reader,Writer}")
	})
}

func testCurrentMockFailsForVariables(t *testing.T) {
	t.Run("current mock implementation should fail variable extraction completely", func(t *testing.T) {
		sourceCode := `package main

var GlobalVar int = 42
const MaxSize = 100

func main() {
	var localVar string = "hello"
	count := 5
}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parserFactory := &ParserFactoryImpl{}
		adapter := NewSemanticTraverserAdapter(parserFactory)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeTypeInfo: true,
			MaxDepth:        10,
		}

		result, err := adapter.ExtractVariables(context.Background(), parseTree, options)
		require.NoError(t, err)

		// Current mock implementation returns empty slice for variables
		assert.Empty(t, result, "Mock implementation returns empty slice for variables")

		t.Logf(
			"EXPECTED FAILURE: Mock implementation returns no variables instead of parsing actual variables/constants from source",
		)
		t.Logf("Real implementation needs to parse: GlobalVar(int), MaxSize(const), localVar(string), count(int)")
	})
}

func testCurrentMockFailsForImports(t *testing.T) {
	t.Run("current mock implementation should fail import extraction completely", func(t *testing.T) {
		sourceCode := `package main

import "fmt"
import "strings"

import (
	"context"
	"encoding/json"
	h "net/http"
)`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parserFactory := &ParserFactoryImpl{}
		adapter := NewSemanticTraverserAdapter(parserFactory)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeTypeInfo: true,
			MaxDepth:        10,
		}

		result, err := adapter.ExtractImports(context.Background(), parseTree, options)
		require.NoError(t, err)

		// Current mock implementation returns empty slice for imports
		assert.Empty(t, result, "Mock implementation returns empty slice for imports")

		t.Logf(
			"EXPECTED FAILURE: Mock implementation returns no imports instead of parsing actual 5 imports from source",
		)
		t.Logf("Real implementation needs to parse: fmt, strings, context, encoding/json, net/http(alias h)")
	})
}

func testCurrentMockFailsForPackages(t *testing.T) {
	t.Run("current mock implementation should return inadequate package information", func(t *testing.T) {
		sourceCode := `// Package utils provides comprehensive utilities
package utils

import (
	"fmt"
	"strings"
)

const Version = "1.0.0"
var GlobalVar int

type MyStruct struct {
	Field string
}

func PublicFunc() string {
	return "hello"
}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parserFactory := &ParserFactoryImpl{}
		adapter := NewSemanticTraverserAdapter(parserFactory)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:       true,
			IncludeTypeInfo:      true,
			IncludeMetadata:      true,
			IncludeDocumentation: true,
			MaxDepth:             10,
		}

		result, err := adapter.ExtractModules(context.Background(), parseTree, options)
		require.NoError(t, err)

		// Current mock implementation returns basic package info without comprehensive analysis
		require.Len(t, result, 1, "Mock should return 1 package")
		pkg := result[0]

		assert.Equal(t, "main", pkg.Name, "Mock returns hardcoded 'main' instead of actual 'utils'")
		assert.Equal(t, "package main", pkg.Content, "Mock returns hardcoded 'package main' content")

		t.Logf(
			"EXPECTED FAILURE: Mock implementation returns basic hardcoded package info without parsing actual package structure",
		)
		t.Logf("Real implementation needs to parse package name 'utils', documentation, and comprehensive metadata")
	})
}

// Helper functions to test extraction with common validation logic.
func testGoVariableExtraction(
	t *testing.T,
	sourceCode string,
	expectedVariables []outbound.SemanticCodeChunk,
	options outbound.SemanticExtractionOptions,
) {
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserFactory := &ParserFactoryImpl{}
	adapter := NewSemanticTraverserAdapter(parserFactory)

	result, err := adapter.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, len(expectedVariables), "Number of extracted variables should match expected")

	for i, expected := range expectedVariables {
		actual := result[i]
		validateSemanticCodeChunk(t, expected, actual)
	}
}

func testGoImportExtraction(
	t *testing.T,
	sourceCode string,
	expectedImports []outbound.ImportDeclaration,
	options outbound.SemanticExtractionOptions,
) {
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserFactory := &ParserFactoryImpl{}
	adapter := NewSemanticTraverserAdapter(parserFactory)

	result, err := adapter.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, len(expectedImports), "Number of extracted imports should match expected")

	for i, expected := range expectedImports {
		actual := result[i]
		validateImportDeclaration(t, expected, actual)
	}
}

func testGoPackageExtraction(
	t *testing.T,
	sourceCode string,
	expectedPackages []outbound.SemanticCodeChunk,
	options outbound.SemanticExtractionOptions,
) {
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserFactory := &ParserFactoryImpl{}
	adapter := NewSemanticTraverserAdapter(parserFactory)

	result, err := adapter.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, len(expectedPackages), "Number of extracted packages should match expected")

	for i, expected := range expectedPackages {
		actual := result[i]
		validatePackageSemanticCodeChunk(t, expected, actual, options)
	}
}

// Validation helper functions.
func validateSemanticCodeChunk(t *testing.T, expected, actual outbound.SemanticCodeChunk) {
	assert.Equal(t, expected.Type, actual.Type, "Type should match")
	assert.Equal(t, expected.Name, actual.Name, "Name should match")
	assert.Equal(t, expected.QualifiedName, actual.QualifiedName, "Qualified name should match")
	assert.Equal(t, expected.Visibility, actual.Visibility, "Visibility should match")
	assert.Equal(t, expected.IsGeneric, actual.IsGeneric, "Generic flag should match")
	assert.Equal(t, expected.ReturnType, actual.ReturnType, "Return type should match")

	// Validate parameters
	validateParameters(t, expected.Parameters, actual.Parameters)

	// Validate generic parameters if present
	validateGenericParameters(t, expected.GenericParameters, actual.GenericParameters, expected.IsGeneric)

	// Validate child chunks if present
	validateChildChunks(t, expected.ChildChunks, actual.ChildChunks)

	// Validate content and other fields
	assert.Contains(t, actual.Content, expected.Name, "Content should contain name")
	if expected.Documentation != "" {
		assert.Equal(t, expected.Documentation, actual.Documentation, "Documentation should match")
	}
	assert.Greater(t, actual.EndByte, actual.StartByte, "End byte should be greater than start byte")
	assert.NotEmpty(t, actual.Hash, "Hash should be generated")
	assert.NotZero(t, actual.ExtractedAt, "ExtractedAt should be set")
}

func validateParameters(t *testing.T, expected, actual []outbound.Parameter) {
	require.Len(t, actual, len(expected), "Parameter count should match")
	for j, expectedParam := range expected {
		actualParam := actual[j]
		assert.Equal(t, expectedParam.Name, actualParam.Name, "Parameter name should match")
		assert.Equal(t, expectedParam.Type, actualParam.Type, "Parameter type should match")
		assert.Equal(t, expectedParam.IsVariadic, actualParam.IsVariadic, "Parameter variadic flag should match")
	}
}

func validateGenericParameters(
	t *testing.T,
	expected, actual []outbound.GenericParameter,
	isGeneric bool,
) {
	if isGeneric {
		require.Len(t, actual, len(expected), "Generic parameter count should match")
		for j, expectedGeneric := range expected {
			actualGeneric := actual[j]
			assert.Equal(t, expectedGeneric.Name, actualGeneric.Name, "Generic parameter name should match")
			assert.Equal(
				t,
				expectedGeneric.Constraints,
				actualGeneric.Constraints,
				"Generic parameter constraints should match",
			)
		}
	}
}

func validateChildChunks(t *testing.T, expected, actual []outbound.SemanticCodeChunk) {
	if len(expected) > 0 {
		require.Len(t, actual, len(expected), "Child chunk count should match")
		for j, expectedChild := range expected {
			actualChild := actual[j]
			assert.Equal(t, expectedChild.Type, actualChild.Type, "Child type should match")
			assert.Equal(t, expectedChild.Name, actualChild.Name, "Child name should match")
			assert.Equal(t, expectedChild.ReturnType, actualChild.ReturnType, "Child data type should match")
			assert.Equal(t, expectedChild.Visibility, actualChild.Visibility, "Child visibility should match")

			// Validate annotations if present
			validateAnnotations(t, expectedChild.Annotations, actualChild.Annotations)
		}
	}
}

func validateAnnotations(t *testing.T, expected, actual []outbound.Annotation) {
	if len(expected) > 0 {
		require.Len(t, actual, len(expected), "Annotation count should match")
		for k, expectedAnnotation := range expected {
			actualAnnotation := actual[k]
			assert.Equal(t, expectedAnnotation.Name, actualAnnotation.Name, "Annotation name should match")
			assert.Equal(
				t,
				expectedAnnotation.Arguments,
				actualAnnotation.Arguments,
				"Annotation arguments should match",
			)
		}
	}
}

func validateImportDeclaration(t *testing.T, expected, actual outbound.ImportDeclaration) {
	assert.Equal(t, expected.Path, actual.Path, "Import path should match")
	assert.Equal(t, expected.Alias, actual.Alias, "Import alias should match")
	assert.Equal(t, expected.IsWildcard, actual.IsWildcard, "Wildcard flag should match")
	assert.Equal(t, expected.ImportedSymbols, actual.ImportedSymbols, "Imported symbols should match")
	assert.Contains(t, actual.Content, expected.Path, "Content should contain import path")
	assert.Greater(t, actual.EndByte, actual.StartByte, "End byte should be greater than start byte")
}

func validatePackageSemanticCodeChunk(
	t *testing.T,
	expected, actual outbound.SemanticCodeChunk,
	options outbound.SemanticExtractionOptions,
) {
	assert.Equal(t, expected.Type, actual.Type, "Package type should match")
	assert.Equal(t, expected.Name, actual.Name, "Package name should match")
	assert.Equal(t, expected.QualifiedName, actual.QualifiedName, "Qualified name should match")
	assert.Equal(t, expected.Visibility, actual.Visibility, "Visibility should match")

	if options.IncludeDocumentation && expected.Documentation != "" {
		assert.Equal(t, expected.Documentation, actual.Documentation, "Documentation should match")
	}

	validatePackageMetadata(t, expected.Metadata, actual.Metadata, options.IncludeMetadata)

	assert.Contains(t, actual.Content, expected.Name, "Content should contain package name")
	assert.Equal(t, uint32(0), actual.StartByte, "Package should start at beginning of file")
	assert.Greater(t, actual.EndByte, actual.StartByte, "End byte should be greater than start byte")
	assert.NotEmpty(t, actual.Hash, "Hash should be generated")
	assert.NotZero(t, actual.ExtractedAt, "ExtractedAt should be set")
}

func validatePackageMetadata(
	t *testing.T,
	expected, actual map[string]interface{},
	includeMetadata bool,
) {
	if includeMetadata && len(expected) > 0 {
		require.NotNil(t, actual, "Metadata should be present")

		// Validate key metadata fields
		for key, expectedValue := range expected {
			actualValue, exists := actual[key]
			require.True(t, exists, "Metadata key '%s' should exist", key)
			assert.Equal(t, expectedValue, actualValue, "Metadata value for '%s' should match", key)
		}
	}
}

// Helper function to create a parse tree from source code using simple parsing.
// This is a GREEN phase implementation that creates real nodes for the semantic traverser to find.
func createMockParseTreeFromSource(
	t *testing.T,
	language valueobject.Language,
	sourceCode string,
) *valueobject.ParseTree {
	// Parse the source code to create actual nodes that the semantic traverser can find
	children := parseGoSourceToNodes(sourceCode)

	rootNode := &valueobject.ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   uint32(len(sourceCode)),
		StartPos:  valueobject.Position{Row: 0, Column: 0},
		EndPos:    valueobject.Position{Row: uint32(strings.Count(sourceCode, "\n")), Column: 0},
		Children:  children,
	}

	metadata := valueobject.ParseMetadata{
		ParseDuration:     time.Millisecond * 10,
		TreeSitterVersion: "0.20.8",
		GrammarVersion:    "1.0.0",
		NodeCount:         len(children) + 1,
		MaxDepth:          3,
	}

	parseTree, err := valueobject.NewParseTree(
		context.Background(),
		language,
		rootNode,
		[]byte(sourceCode),
		metadata,
	)
	require.NoError(t, err)

	return parseTree
}

// parseGoSourceToNodes creates minimal ParseNode structures for Go constructs
// This is a simplified parser for the GREEN phase to make tests pass.
func parseGoSourceToNodes(sourceCode string) []*valueobject.ParseNode {
	var children []*valueobject.ParseNode
	lines := strings.Split(sourceCode, "\n")

	// Add package clause
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Package declaration
		if strings.HasPrefix(trimmed, "package ") {
			startByte := uint32(calculateByteOffset(lines, i))
			endByte := startByte + uint32(len(line))

			packageNode := &valueobject.ParseNode{
				Type:      "package_clause",
				StartByte: startByte,
				EndByte:   endByte,
				StartPos:  valueobject.Position{Row: uint32(i), Column: 0},
				EndPos:    valueobject.Position{Row: uint32(i), Column: uint32(len(line))},
				Children: []*valueobject.ParseNode{
					{
						Type:      "package_identifier",
						StartByte: startByte + 8, // "package "
						EndByte:   endByte,
						StartPos:  valueobject.Position{Row: uint32(i), Column: 8},
						EndPos:    valueobject.Position{Row: uint32(i), Column: uint32(len(line))},
						Children:  []*valueobject.ParseNode{},
					},
				},
			}
			children = append(children, packageNode)
		}

		// Function declarations
		if strings.HasPrefix(trimmed, "func ") {
			startByte := uint32(calculateByteOffset(lines, i))
			endByte := findFunctionEndByte(lines, i, sourceCode)

			functionNode := createFunctionNode(trimmed, startByte, endByte, uint32(i))
			children = append(children, functionNode)
		}

		// Type declarations (structs, interfaces)
		if strings.HasPrefix(trimmed, "type ") &&
			(strings.Contains(trimmed, "struct") || strings.Contains(trimmed, "interface")) {
			startByte := uint32(calculateByteOffset(lines, i))
			endByte := findTypeDeclarationEndByte(lines, i, sourceCode)

			typeNode := createTypeDeclarationNode(trimmed, startByte, endByte, uint32(i))
			children = append(children, typeNode)
		}
	}

	return children
}

// calculateByteOffset calculates the byte offset for a given line.
func calculateByteOffset(lines []string, lineIndex int) int {
	offset := 0
	for i := range lineIndex {
		offset += len(lines[i]) + 1 // +1 for newline
	}
	return offset
}

// createFunctionNode creates a function_declaration node.
func createFunctionNode(funcLine string, startByte, endByte, row uint32) *valueobject.ParseNode {
	// Check if this is a method (has receiver)
	isMethod := isMethodDeclaration(funcLine)
	nodeType := "function_declaration"
	if isMethod {
		nodeType = "method_declaration"
	}

	funcName := extractFunctionName(funcLine)
	identifierNode := createFunctionIdentifier(funcLine, funcName, startByte, row, isMethod)
	children := []*valueobject.ParseNode{identifierNode}

	// Add receiver node for methods
	if isMethod {
		receiverNode := createReceiverNode(funcLine, startByte, row)
		if receiverNode != nil {
			// Insert receiver before identifier
			children = []*valueobject.ParseNode{receiverNode, identifierNode}
		}
	}

	paramStart, paramEnd := findParameterBounds(funcLine)
	children = addParameterNodes(children, funcLine, paramStart, paramEnd, startByte, row)

	return &valueobject.ParseNode{
		Type:      nodeType,
		StartByte: startByte,
		EndByte:   endByte,
		StartPos:  valueobject.Position{Row: row, Column: 0},
		EndPos:    valueobject.Position{Row: row, Column: uint32(len(funcLine))},
		Children:  children,
	}
}

// extractFunctionName extracts function name from function line.
func extractFunctionName(funcLine string) string {
	parts := strings.Fields(funcLine)
	if len(parts) < 2 {
		return ""
	}

	// Handle method with receiver: func (r *Type) MethodName
	if strings.HasPrefix(parts[1], "(") {
		return extractMethodName(parts)
	}

	// Handle regular function: func FunctionName
	if strings.Contains(parts[1], "(") && !strings.HasPrefix(parts[1], "(") {
		return strings.Split(parts[1], "(")[0]
	}

	// Handle generic function: func FunctionName[T any]
	if len(parts) >= 3 && strings.Contains(parts[1], "[") {
		return strings.Split(parts[1], "[")[0]
	}

	return parts[1]
}

// extractMethodName extracts method name from method declaration parts.
func extractMethodName(parts []string) string {
	receiverEnd := findReceiverEnd(parts)
	if receiverEnd == -1 || receiverEnd+1 >= len(parts) {
		return ""
	}

	methodPart := parts[receiverEnd+1]
	if strings.Contains(methodPart, "(") {
		return strings.Split(methodPart, "(")[0]
	}
	return methodPart
}

// findReceiverEnd finds the index where receiver parentheses end.
func findReceiverEnd(parts []string) int {
	parenCount := 0
	for i, part := range parts {
		if strings.Contains(part, "(") {
			parenCount += strings.Count(part, "(")
		}
		if strings.Contains(part, ")") {
			parenCount -= strings.Count(part, ")")
			if parenCount == 0 {
				return i
			}
		}
	}
	return -1
}

// isMethodDeclaration checks if a function line is a method with receiver.
func isMethodDeclaration(funcLine string) bool {
	// Look for pattern: func (receiver) methodName
	return strings.Contains(funcLine, "func (") && strings.Index(funcLine, ")") < strings.LastIndex(funcLine, "(")
}

// createReceiverNode creates a receiver parameter list node for methods.
func createReceiverNode(funcLine string, startByte, row uint32) *valueobject.ParseNode {
	// Find receiver bounds: func (receiver)
	receiverStart := strings.Index(funcLine, "(")
	receiverEnd := strings.Index(funcLine[receiverStart+1:], ")") + receiverStart + 1

	if receiverStart == -1 || receiverEnd <= receiverStart {
		return nil
	}

	receiverText := funcLine[receiverStart+1 : receiverEnd]

	// Create parameter declaration for receiver
	receiverDecl := createParameterDeclarationNode(receiverText, startByte+uint32(receiverStart+1), row)

	return &valueobject.ParseNode{
		Type:      "parameter_list",
		StartByte: startByte + uint32(receiverStart),
		EndByte:   startByte + uint32(receiverEnd+1),
		StartPos:  valueobject.Position{Row: row, Column: uint32(receiverStart)},
		EndPos:    valueobject.Position{Row: row, Column: uint32(receiverEnd + 1)},
		Children:  []*valueobject.ParseNode{receiverDecl},
	}
}

// createFunctionIdentifier creates identifier node for function name.
func createFunctionIdentifier(funcLine, funcName string, startByte, row uint32, isMethod bool) *valueobject.ParseNode {
	var nameIndex int
	if isMethod {
		// For methods, find the function name after the receiver
		receiverEnd := strings.Index(funcLine, ") ")
		if receiverEnd != -1 {
			nameIndex = strings.Index(funcLine[receiverEnd+2:], funcName) + receiverEnd + 2
		} else {
			nameIndex = strings.Index(funcLine, funcName)
		}
	} else {
		nameIndex = strings.Index(funcLine, funcName)
	}

	nodeType := "identifier"
	if isMethod {
		nodeType = "field_identifier"
	}

	return &valueobject.ParseNode{
		Type:      nodeType,
		StartByte: startByte + uint32(nameIndex),
		EndByte:   startByte + uint32(nameIndex) + uint32(len(funcName)),
		StartPos:  valueobject.Position{Row: row, Column: uint32(nameIndex)},
		EndPos:    valueobject.Position{Row: row, Column: uint32(nameIndex) + uint32(len(funcName))},
		Children:  []*valueobject.ParseNode{},
	}
}

// findParameterBounds finds start and end positions of parameter list.
func findParameterBounds(funcLine string) (int, int) {
	if !strings.Contains(funcLine, "(") {
		return 0, 0
	}

	paramStart := strings.Index(funcLine, "(")
	parenCount := 0

	for i, char := range funcLine[paramStart:] {
		switch char {
		case '(':
			parenCount++
		case ')':
			parenCount--
			if parenCount == 0 {
				return paramStart, paramStart + i + 1
			}
		}
	}

	return paramStart, 0
}

// addParameterNodes adds parameter list and return type nodes if present.
func addParameterNodes(
	children []*valueobject.ParseNode,
	funcLine string,
	paramStart, paramEnd int,
	startByte, row uint32,
) []*valueobject.ParseNode {
	if paramStart <= 0 || paramEnd <= paramStart {
		return children
	}

	paramText := funcLine[paramStart+1 : paramEnd-1] // Remove parentheses
	paramListNode := createParameterListNode(
		paramText,
		startByte+uint32(paramStart),
		startByte+uint32(paramEnd),
		row,
	)
	children = append(children, paramListNode)

	// For methods, find the second parameter list
	if isMethodDeclaration(funcLine) {
		secondParamStart, secondParamEnd := findSecondParameterBounds(funcLine, paramEnd)
		if secondParamStart > 0 && secondParamEnd > secondParamStart {
			secondParamText := funcLine[secondParamStart+1 : secondParamEnd-1]
			secondParamListNode := createParameterListNode(
				secondParamText,
				startByte+uint32(secondParamStart),
				startByte+uint32(secondParamEnd),
				row,
			)
			children = append(children, secondParamListNode)
			// Use second param end for return type parsing
			return addReturnTypeNode(children, funcLine, secondParamEnd, startByte, row)
		}
	}

	return addReturnTypeNode(children, funcLine, paramEnd, startByte, row)
}

// findSecondParameterBounds finds the second parameter list in method declarations.
func findSecondParameterBounds(funcLine string, afterFirstParam int) (int, int) {
	// Look for the next opening parenthesis after the first parameter list
	remaining := funcLine[afterFirstParam:]
	paramStart := strings.Index(remaining, "(")
	if paramStart == -1 {
		return 0, 0
	}

	paramStart += afterFirstParam // Make it relative to the full string
	parenCount := 0

	for i, char := range funcLine[paramStart:] {
		switch char {
		case '(':
			parenCount++
		case ')':
			parenCount--
			if parenCount == 0 {
				return paramStart, paramStart + i + 1
			}
		}
	}

	return paramStart, 0
}

// addReturnTypeNode adds return type node if present.
func addReturnTypeNode(
	children []*valueobject.ParseNode,
	funcLine string,
	paramEnd int,
	startByte, row uint32,
) []*valueobject.ParseNode {
	afterParams := strings.TrimSpace(funcLine[paramEnd:])
	if afterParams == "" || strings.HasPrefix(afterParams, "{") {
		return children
	}

	returnType := afterParams
	if braceIndex := strings.Index(afterParams, "{"); braceIndex > 0 {
		returnType = strings.TrimSpace(afterParams[:braceIndex])
	}

	if returnType == "" {
		return children
	}

	returnTypeNode := &valueobject.ParseNode{
		Type:      "type_identifier",
		StartByte: startByte + uint32(paramEnd) + 1, // +1 for space
		EndByte:   startByte + uint32(paramEnd) + 1 + uint32(len(returnType)),
		StartPos:  valueobject.Position{Row: row, Column: uint32(paramEnd) + 1},
		EndPos:    valueobject.Position{Row: row, Column: uint32(paramEnd) + 1 + uint32(len(returnType))},
		Children:  []*valueobject.ParseNode{},
	}

	return append(children, returnTypeNode)
}

// createTypeDeclarationNode creates a type_declaration node for structs and interfaces.
func createTypeDeclarationNode(typeLine string, startByte, endByte, row uint32) *valueobject.ParseNode {
	// Extract type name
	parts := strings.Fields(typeLine)
	var typeName string
	if len(parts) >= 2 {
		typeName = parts[1]
	}

	// Determine if it's a struct or interface
	var typeSpecType string
	var innerType string
	if strings.Contains(typeLine, "struct") {
		innerType = "struct_type"
		typeSpecType = "type_spec"
	} else if strings.Contains(typeLine, "interface") {
		innerType = "interface_type"
		typeSpecType = "type_spec"
	}

	identifierNode := &valueobject.ParseNode{
		Type:      "type_identifier",
		StartByte: startByte + uint32(strings.Index(typeLine, typeName)),
		EndByte:   startByte + uint32(strings.Index(typeLine, typeName)) + uint32(len(typeName)),
		StartPos:  valueobject.Position{Row: row, Column: uint32(strings.Index(typeLine, typeName))},
		EndPos: valueobject.Position{
			Row:    row,
			Column: uint32(strings.Index(typeLine, typeName)) + uint32(len(typeName)),
		},
		Children: []*valueobject.ParseNode{},
	}

	innerTypeNode := &valueobject.ParseNode{
		Type: innerType,
		StartByte: startByte + uint32(
			strings.Index(typeLine, strings.Fields(typeLine)[len(strings.Fields(typeLine))-1]),
		),
		EndByte: endByte,
		StartPos: valueobject.Position{
			Row:    row,
			Column: uint32(strings.Index(typeLine, strings.Fields(typeLine)[len(strings.Fields(typeLine))-1])),
		},
		EndPos:   valueobject.Position{Row: row, Column: uint32(len(typeLine))},
		Children: []*valueobject.ParseNode{},
	}

	typeSpecNode := &valueobject.ParseNode{
		Type:      typeSpecType,
		StartByte: startByte + uint32(strings.Index(typeLine, typeName)),
		EndByte:   endByte,
		StartPos:  valueobject.Position{Row: row, Column: uint32(strings.Index(typeLine, typeName))},
		EndPos:    valueobject.Position{Row: row, Column: uint32(len(typeLine))},
		Children:  []*valueobject.ParseNode{identifierNode, innerTypeNode},
	}

	return &valueobject.ParseNode{
		Type:      "type_declaration",
		StartByte: startByte,
		EndByte:   endByte,
		StartPos:  valueobject.Position{Row: row, Column: 0},
		EndPos:    valueobject.Position{Row: row, Column: uint32(len(typeLine))},
		Children:  []*valueobject.ParseNode{typeSpecNode},
	}
}

// findFunctionEndByte finds the end byte of a function declaration.
func findFunctionEndByte(lines []string, startLine int, sourceCode string) uint32 {
	braceCount := 0
	started := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		for _, char := range line {
			switch char {
			case '{':
				braceCount++
				started = true
			case '}':
				braceCount--
				if started && braceCount == 0 {
					// Found the end, calculate byte offset
					endLineOffset := calculateByteOffset(lines, i)
					endByteInLine := strings.Index(line, "}") + 1
					return uint32(endLineOffset + endByteInLine)
				}
			}
		}
	}

	// Fallback: end of source code
	return uint32(len(sourceCode))
}

// findTypeDeclarationEndByte finds the end byte of a type declaration.
func findTypeDeclarationEndByte(lines []string, startLine int, sourceCode string) uint32 {
	// For simple cases, assume it ends at the end of the declaration
	braceCount := 0
	started := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		for _, char := range line {
			switch char {
			case '{':
				braceCount++
				started = true
			case '}':
				braceCount--
				if started && braceCount == 0 {
					// Found the end
					endLineOffset := calculateByteOffset(lines, i)
					endByteInLine := strings.Index(line, "}") + 1
					return uint32(endLineOffset + endByteInLine)
				}
			}
		}

		// For interface/struct without braces on same line
		if !started && i == startLine && !strings.Contains(line, "{") {
			return uint32(calculateByteOffset(lines, i) + len(line))
		}
	}

	// Fallback
	return uint32(len(sourceCode))
}

// createParameterListNode creates a parameter_list node with parameter_declaration children.
func createParameterListNode(paramText string, startByte, endByte uint32, row uint32) *valueobject.ParseNode {
	var paramDecls []*valueobject.ParseNode

	// Simple parameter parsing - split by comma
	if strings.TrimSpace(paramText) != "" {
		params := strings.Split(paramText, ",")
		currentOffset := startByte + 1 // Skip opening paren

		for _, param := range params {
			param = strings.TrimSpace(param)
			if param == "" {
				continue
			}

			// Create parameter_declaration node
			paramDeclNode := createParameterDeclarationNode(param, currentOffset, row)
			paramDecls = append(paramDecls, paramDeclNode)
			currentOffset += uint32(len(param) + 2) // +2 for ", "
		}
	}

	return &valueobject.ParseNode{
		Type:      "parameter_list",
		StartByte: startByte,
		EndByte:   endByte,
		StartPos:  valueobject.Position{Row: row, Column: startByte},
		EndPos:    valueobject.Position{Row: row, Column: endByte},
		Children:  paramDecls,
	}
}

// createParameterDeclarationNode creates a parameter_declaration node.
func createParameterDeclarationNode(paramText string, startByte uint32, row uint32) *valueobject.ParseNode {
	// Parse parameter: "name type" or just "type"
	parts := strings.Fields(paramText)
	var identifierNode *valueobject.ParseNode
	var typeNode *valueobject.ParseNode

	if len(parts) == 2 {
		// "name type"
		paramName := parts[0]

		identifierNode = &valueobject.ParseNode{
			Type:      "identifier",
			StartByte: startByte,
			EndByte:   startByte + uint32(len(paramName)),
			StartPos:  valueobject.Position{Row: row, Column: startByte},
			EndPos:    valueobject.Position{Row: row, Column: startByte + uint32(len(paramName))},
			Children:  []*valueobject.ParseNode{},
		}

		paramType := parts[1]
		typeStartByte := startByte + uint32(len(paramName)) + 1 // +1 for space

		// Handle pointer types
		if strings.HasPrefix(paramType, "*") {
			// Create pointer_type node with type_identifier child
			typeIdentifierNode := &valueobject.ParseNode{
				Type:      "type_identifier",
				StartByte: typeStartByte + 1, // +1 for *
				EndByte:   startByte + uint32(len(paramText)),
				StartPos:  valueobject.Position{Row: row, Column: typeStartByte + 1},
				EndPos:    valueobject.Position{Row: row, Column: startByte + uint32(len(paramText))},
				Children:  []*valueobject.ParseNode{},
			}

			typeNode = &valueobject.ParseNode{
				Type:      "pointer_type",
				StartByte: typeStartByte,
				EndByte:   startByte + uint32(len(paramText)),
				StartPos:  valueobject.Position{Row: row, Column: typeStartByte},
				EndPos:    valueobject.Position{Row: row, Column: startByte + uint32(len(paramText))},
				Children:  []*valueobject.ParseNode{typeIdentifierNode},
			}
		} else {
			// Regular type
			typeNode = &valueobject.ParseNode{
				Type:      "type_identifier",
				StartByte: typeStartByte,
				EndByte:   startByte + uint32(len(paramText)),
				StartPos:  valueobject.Position{Row: row, Column: typeStartByte},
				EndPos:    valueobject.Position{Row: row, Column: startByte + uint32(len(paramText))},
				Children:  []*valueobject.ParseNode{},
			}
		}
	} else if len(parts) == 1 {
		// Just "type" (anonymous parameter)
		paramType := parts[0]
		typeNode = &valueobject.ParseNode{
			Type:      "type_identifier",
			StartByte: startByte,
			EndByte:   startByte + uint32(len(paramType)),
			StartPos:  valueobject.Position{Row: row, Column: startByte},
			EndPos:    valueobject.Position{Row: row, Column: startByte + uint32(len(paramType))},
			Children:  []*valueobject.ParseNode{},
		}
	}

	children := []*valueobject.ParseNode{}
	if identifierNode != nil {
		children = append(children, identifierNode)
	}
	if typeNode != nil {
		children = append(children, typeNode)
	}

	return &valueobject.ParseNode{
		Type:      "parameter_declaration",
		StartByte: startByte,
		EndByte:   startByte + uint32(len(paramText)),
		StartPos:  valueobject.Position{Row: row, Column: startByte},
		EndPos:    valueobject.Position{Row: row, Column: startByte + uint32(len(paramText))},
		Children:  children,
	}
}

// Expected data creation helpers to reduce repetition.
func createExpectedUserStructFields() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructField,
			Name:          "ID",
			QualifiedName: "main.User.ID",
			Content:       "ID       int    `json:\"id\" db:\"id\"`",
			ReturnType:    "int",
			Visibility:    outbound.Public,
			Annotations: []outbound.Annotation{
				{Name: "json", Arguments: []string{"id"}},
				{Name: "db", Arguments: []string{"id"}},
			},
		},
		{
			Type:          outbound.ConstructField,
			Name:          "Name",
			QualifiedName: "main.User.Name",
			Content:       "Name     string `json:\"name\" db:\"name\"`",
			ReturnType:    "string",
			Visibility:    outbound.Public,
			Annotations: []outbound.Annotation{
				{Name: "json", Arguments: []string{"name"}},
				{Name: "db", Arguments: []string{"name"}},
			},
		},
		{
			Type:          outbound.ConstructField,
			Name:          "Email",
			QualifiedName: "main.User.Email",
			Content:       "Email    string `json:\"email\" db:\"email\"`",
			ReturnType:    "string",
			Visibility:    outbound.Public,
			Annotations: []outbound.Annotation{
				{Name: "json", Arguments: []string{"email"}},
				{Name: "db", Arguments: []string{"email"}},
			},
		},
		{
			Type:          outbound.ConstructField,
			Name:          "Age",
			QualifiedName: "main.User.Age",
			Content:       "Age      int    `json:\"age\" db:\"age\"`",
			ReturnType:    "int",
			Visibility:    outbound.Public,
			Annotations: []outbound.Annotation{
				{Name: "json", Arguments: []string{"age"}},
				{Name: "db", Arguments: []string{"age"}},
			},
		},
		{
			Type:          outbound.ConstructField,
			Name:          "IsActive",
			QualifiedName: "main.User.IsActive",
			Content:       "IsActive bool   `json:\"is_active\" db:\"is_active\"`",
			ReturnType:    "bool",
			Visibility:    outbound.Public,
			Annotations: []outbound.Annotation{
				{Name: "json", Arguments: []string{"is_active"}},
				{Name: "db", Arguments: []string{"is_active"}},
			},
		},
	}
}

func createExpectedPersonStructFields() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructField,
			Name:          "Name",
			QualifiedName: "main.Person.Name",
			Content:       "Name string",
			ReturnType:    "string",
			Visibility:    outbound.Public,
		},
		{
			Type:          outbound.ConstructField,
			Name:          "Age",
			QualifiedName: "main.Person.Age",
			Content:       "Age  int",
			ReturnType:    "int",
			Visibility:    outbound.Public,
		},
	}
}

func createExpectedEmployeeStructFields() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructField,
			Name:          "Person",
			QualifiedName: "main.Employee.Person",
			Content:       "Person          // Embedded struct",
			ReturnType:    "Person",
			Visibility:    outbound.Public,
		},
		{
			Type:          outbound.ConstructField,
			Name:          "EmployeeID",
			QualifiedName: "main.Employee.EmployeeID",
			Content:       "EmployeeID int  `json:\"employee_id\"`",
			ReturnType:    "int",
			Visibility:    outbound.Public,
			Annotations: []outbound.Annotation{
				{Name: "json", Arguments: []string{"employee_id"}},
			},
		},
		{
			Type:          outbound.ConstructField,
			Name:          "Department",
			QualifiedName: "main.Employee.Department",
			Content:       "Department string",
			ReturnType:    "string",
			Visibility:    outbound.Public,
		},
	}
}

func createExpectedContainerStructFields() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructField,
			Name:          "value",
			QualifiedName: "main.Container.value",
			Content:       "value T",
			ReturnType:    "T",
			Visibility:    outbound.Private,
		},
		{
			Type:          outbound.ConstructField,
			Name:          "count",
			QualifiedName: "main.Container.count",
			Content:       "count int",
			ReturnType:    "int",
			Visibility:    outbound.Private,
		},
	}
}

func createExpectedPairStructFields() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructField,
			Name:          "Key",
			QualifiedName: "main.Pair.Key",
			Content:       "Key   K `json:\"key\"`",
			ReturnType:    "K",
			Visibility:    outbound.Public,
			Annotations: []outbound.Annotation{
				{Name: "json", Arguments: []string{"key"}},
			},
		},
		{
			Type:          outbound.ConstructField,
			Name:          "Value",
			QualifiedName: "main.Pair.Value",
			Content:       "Value V `json:\"value\"`",
			ReturnType:    "V",
			Visibility:    outbound.Public,
			Annotations: []outbound.Annotation{
				{Name: "json", Arguments: []string{"value"}},
			},
		},
	}
}

func createExpectedWriterInterfaceMethods() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructMethod,
			Name:          "Write",
			QualifiedName: "main.Writer.Write",
			Content:       "Write(data []byte) (int, error)",
			Parameters: []outbound.Parameter{
				{Name: "data", Type: "[]byte", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "(int, error)",
			Visibility: outbound.Public,
			IsAbstract: true,
		},
	}
}

func createExpectedReaderInterfaceMethods() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructMethod,
			Name:          "Read",
			QualifiedName: "main.Reader.Read",
			Content:       "Read(buffer []byte) (int, error)",
			Parameters: []outbound.Parameter{
				{Name: "buffer", Type: "[]byte", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "(int, error)",
			Visibility: outbound.Public,
			IsAbstract: true,
		},
	}
}

func createExpectedReadWriterInterfaceMethods() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructInterface,
			Name:          "Reader",
			QualifiedName: "main.ReadWriter.Reader",
			Content:       "Reader",
			Visibility:    outbound.Public,
			IsAbstract:    true,
		},
		{
			Type:          outbound.ConstructInterface,
			Name:          "Writer",
			QualifiedName: "main.ReadWriter.Writer",
			Content:       "Writer",
			Visibility:    outbound.Public,
			IsAbstract:    true,
		},
	}
}

func createExpectedComparableInterfaceMethods() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructMethod,
			Name:          "CompareTo",
			QualifiedName: "main.Comparable.CompareTo",
			Content:       "CompareTo(other T) int",
			Parameters: []outbound.Parameter{
				{Name: "other", Type: "T", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "int",
			Visibility: outbound.Public,
			IsAbstract: true,
		},
	}
}

func createExpectedContainerInterfaceMethods() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructMethod,
			Name:          "Add",
			QualifiedName: "main.Container.Add",
			Content:       "Add(item T) bool",
			Parameters: []outbound.Parameter{
				{Name: "item", Type: "T", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "bool",
			Visibility: outbound.Public,
			IsAbstract: true,
		},
		{
			Type:          outbound.ConstructMethod,
			Name:          "Get",
			QualifiedName: "main.Container.Get",
			Content:       "Get(index int) (T, error)",
			Parameters: []outbound.Parameter{
				{Name: "index", Type: "int", IsOptional: false, IsVariadic: false},
			},
			ReturnType: "(T, error)",
			Visibility: outbound.Public,
			IsAbstract: true,
		},
		{
			Type:          outbound.ConstructMethod,
			Name:          "Size",
			QualifiedName: "main.Container.Size",
			Content:       "Size() int",
			Parameters:    []outbound.Parameter{},
			ReturnType:    "int",
			Visibility:    outbound.Public,
			IsAbstract:    true,
		},
	}
}

func createExpectedPackageLevelVariables() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructVariable,
			Name:          "GlobalCounter",
			QualifiedName: "main.GlobalCounter",
			Content:       "GlobalCounter int = 0",
			ReturnType:    "int",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructVariable,
			Name:          "AppName",
			QualifiedName: "main.AppName",
			Content:       "AppName       string = \"My App\"",
			ReturnType:    "string",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructVariable,
			Name:          "StartTime",
			QualifiedName: "main.StartTime",
			Content:       "StartTime     time.Time",
			ReturnType:    "time.Time",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructVariable,
			Name:          "Logger",
			QualifiedName: "main.Logger",
			Content:       "var Logger *log.Logger",
			ReturnType:    "*log.Logger",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructConstant,
			Name:          "MaxRetries",
			QualifiedName: "main.MaxRetries",
			Content:       "MaxRetries = 3",
			ReturnType:    "int",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructConstant,
			Name:          "Timeout",
			QualifiedName: "main.Timeout",
			Content:       "Timeout    = 30 * time.Second",
			ReturnType:    "time.Duration",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructConstant,
			Name:          "Version",
			QualifiedName: "main.Version",
			Content:       "Version    = \"1.0.0\"",
			ReturnType:    "string",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructConstant,
			Name:          "DefaultPort",
			QualifiedName: "main.DefaultPort",
			Content:       "const DefaultPort = 8080",
			ReturnType:    "int",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructType,
			Name:          "CustomType",
			QualifiedName: "main.CustomType",
			Content:       "type CustomType int",
			ReturnType:    "int",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
		{
			Type:          outbound.ConstructType,
			Name:          "UserID",
			QualifiedName: "main.UserID",
			Content:       "type UserID = int64",
			ReturnType:    "int64",
			Visibility:    outbound.Public,
			IsStatic:      true,
		},
	}
}

func createExpectedIndividualImports() []outbound.ImportDeclaration {
	return []outbound.ImportDeclaration{
		{
			Path:            "fmt",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `import "fmt"`,
		},
		{
			Path:            "strings",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `import "strings"`,
		},
		{
			Path:            "context",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `"context"`,
		},
		{
			Path:            "encoding/json",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `"encoding/json"`,
		},
		{
			Path:            "net/http",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `"net/http"`,
		},
		{
			Path:            "time",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `"time"`,
		},
	}
}

func createExpectedAliasedImports() []outbound.ImportDeclaration {
	return []outbound.ImportDeclaration{
		{
			Path:            "fmt",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `"fmt"`,
		},
		{
			Path:            "fmt",
			Alias:           "f",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `f "fmt"`,
		},
		{
			Path:            "context",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `"context"`,
		},
		{
			Path:            "context",
			Alias:           "ctx",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `ctx "context"`,
		},
		{
			Path:            "github.com/lib/pq",
			Alias:           "_",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `_ "github.com/lib/pq"`,
		},
		{
			Path:            "math",
			Alias:           ".",
			IsWildcard:      true,
			ImportedSymbols: []string{},
			Content:         `. "math"`,
		},
		{
			Path:            "net/http",
			Alias:           "",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `"net/http"`,
		},
		{
			Path:            "net/http",
			Alias:           "h",
			IsWildcard:      false,
			ImportedSymbols: []string{},
			Content:         `h "net/http"`,
		},
	}
}

func createExpectedMainPackage() []outbound.SemanticCodeChunk {
	return []outbound.SemanticCodeChunk{
		{
			Type:          outbound.ConstructPackage,
			Name:          "main",
			QualifiedName: "main",
			Content:       "package main",
			Documentation: "Package main implements a comprehensive example of Go language constructs\nfor testing the semantic parser implementation.",
			Visibility:    outbound.Public,
			IsStatic:      true,
			IsAsync:       false,
			IsAbstract:    false,
			IsGeneric:     false,
			StartByte:     0,
			Metadata: map[string]interface{}{
				"imports_count":   2,
				"constants_count": 1,
				"types_count":     1,
				"functions_count": 2,
				"lines_of_code":   20,
				"complexity":      "low",
				"has_tests":       false,
				"has_main":        true,
				"is_executable":   true,
				"dependencies": []string{
					"fmt",
					"strings",
				},
			},
		},
	}
}
