package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
)

// parseSourceCodeToParseTree uses tree-sitter directly to create a real ParseTree from source code.
func parseSourceCodeToParseTree(sourceCode string) *valueobject.ParseTree {
	// Get Go grammar from forest
	grammar := forest.GetLanguage("go")
	if grammar == nil {
		panic("Failed to get Go grammar from forest")
	}

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	if parser == nil {
		panic("Failed to create tree-sitter parser")
	}

	success := parser.SetLanguage(grammar)
	if !success {
		panic("Failed to set Go language")
	}

	// Parse the source code
	tree, err := parser.ParseString(context.Background(), nil, []byte(sourceCode))
	if err != nil {
		panic("Failed to parse Go source: " + err.Error())
	}
	if tree == nil {
		panic("Parse tree should not be nil")
	}
	defer tree.Close()

	// Convert tree-sitter tree to domain ParseNode
	rootTSNode := tree.RootNode()
	rootNode, nodeCount, maxDepth := convertTreeSitterNode(rootTSNode, 0)

	// Create language
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		panic("Failed to create language: " + err.Error())
	}

	// Create metadata with parsing statistics
	metadata, err := valueobject.NewParseMetadata(
		time.Millisecond, // placeholder duration
		"go-tree-sitter-bare",
		"1.0.0",
	)
	if err != nil {
		panic("Failed to create metadata: " + err.Error())
	}

	// Update metadata with actual counts
	metadata.NodeCount = nodeCount
	metadata.MaxDepth = maxDepth

	// Create and return ParseTree
	parseTree, err := valueobject.NewParseTree(
		context.Background(),
		language,
		rootNode,
		[]byte(sourceCode),
		metadata,
	)
	if err != nil {
		panic("Failed to create ParseTree: " + err.Error())
	}

	return parseTree
}

// GoRegularFunctionsParseTree returns a ParseTree for basic Go function testing using real parsing.
func GoRegularFunctionsParseTree() *valueobject.ParseTree {
	sourceCode := `package main

import "fmt"

func Add(a int, b int) int {
	return a + b
}

func main() {
	fmt.Println("Hello, World!")
}`

	return parseSourceCodeToParseTree(sourceCode)
}

// GoMethodsParseTree returns a ParseTree for Go methods testing using real parsing.
func GoMethodsParseTree() *valueobject.ParseTree {
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

	return parseSourceCodeToParseTree(sourceCode)
}

// GoGenericFunctionsParseTree returns a ParseTree for Go generic functions testing using real parsing.
func GoGenericFunctionsParseTree() *valueobject.ParseTree {
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

	return parseSourceCodeToParseTree(sourceCode)
}

// GoVariadicFunctionsParseTree returns a ParseTree for Go variadic functions testing using real parsing.
func GoVariadicFunctionsParseTree() *valueobject.ParseTree {
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

	return parseSourceCodeToParseTree(sourceCode)
}

// GoDocumentedFunctionParseTree returns a ParseTree for documented Go functions testing using real parsing.
func GoDocumentedFunctionParseTree() *valueobject.ParseTree {
	sourceCode := `package main

// Add performs addition of two integers and returns the result.
// It takes two parameters: a and b, both of type int.
// Returns the sum as an int.
func Add(a int, b int) int {
	return a + b
}`

	return parseSourceCodeToParseTree(sourceCode)
}

// GoBasicStructsParseTree returns a ParseTree for basic Go struct testing using real parsing.
func GoBasicStructsParseTree() *valueobject.ParseTree {
	sourceCode := `package main

type User struct {
	ID       int    ` + "`json:\"id\" db:\"id\"`" + `
	Name     string ` + "`json:\"name\" db:\"name\"`" + `
	Email    string ` + "`json:\"email\" db:\"email\"`" + `
	Age      int    ` + "`json:\"age\" db:\"age\"`" + `
	IsActive bool   ` + "`json:\"is_active\" db:\"is_active\"`" + `
}`

	return parseSourceCodeToParseTree(sourceCode)
}

// GoEmbeddedStructsParseTree returns a ParseTree for embedded Go struct testing using real parsing.
func GoEmbeddedStructsParseTree() *valueobject.ParseTree {
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

	return parseSourceCodeToParseTree(sourceCode)
}

// GoGenericStructsParseTree returns a ParseTree for generic Go struct testing using real parsing.
func GoGenericStructsParseTree() *valueobject.ParseTree {
	sourceCode := `package main

type Container[T any] struct {
	value T
	count int
}

type Pair[K comparable, V any] struct {
	Key   K ` + "`json:\"key\"`" + `
	Value V ` + "`json:\"value\"`" + `
}`

	return parseSourceCodeToParseTree(sourceCode)
}

// GoBasicInterfacesParseTree returns a ParseTree for basic Go interface testing using real parsing.
func GoBasicInterfacesParseTree() *valueobject.ParseTree {
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

	return parseSourceCodeToParseTree(sourceCode)
}

// GoGenericInterfacesParseTree returns a ParseTree for generic Go interface testing using real parsing.
func GoGenericInterfacesParseTree() *valueobject.ParseTree {
	sourceCode := `package main

type Comparable[T any] interface {
	CompareTo(other T) int
}

type Container[T any] interface {
	Add(item T) bool
	Get(index int) (T, error)
	Size() int
}`

	return parseSourceCodeToParseTree(sourceCode)
}

// GoPackageLevelVariablesParseTree returns a ParseTree for package-level variables testing using real parsing.
func GoPackageLevelVariablesParseTree() *valueobject.ParseTree {
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

	return parseSourceCodeToParseTree(sourceCode)
}

// GoMainPackageParseTree returns a ParseTree for main package testing using real parsing.
func GoMainPackageParseTree() *valueobject.ParseTree {
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

	return parseSourceCodeToParseTree(sourceCode)
}
