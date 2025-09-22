package testfixtures

import (
	"codechunking/internal/adapter/outbound/treesitter"
	goparser "codechunking/internal/adapter/outbound/treesitter/parsers/go"
	"codechunking/internal/domain/valueobject"
	"context"
)

// parseSourceCodeToParseTree uses the actual Go parser to create a real ParseTree from source code.
func parseSourceCodeToParseTree(sourceCode string) *valueobject.ParseTree {
	parser, err := goparser.NewGoParser()
	if err != nil {
		panic("Failed to create Go parser: " + err.Error())
	}

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		panic("Failed to create language: " + err.Error())
	}

	result, err := parser.ParseSource(context.Background(), language, []byte(sourceCode), treesitter.ParseOptions{})
	if err != nil {
		panic("Failed to parse source code: " + err.Error())
	}

	if !result.Success {
		panic("Parse was not successful")
	}

	// Convert port ParseTree back to domain ParseTree
	domainTree, err := convertPortToDomainParseTree(result.ParseTree)
	if err != nil {
		panic("Failed to convert parse tree: " + err.Error())
	}

	return domainTree
}

// convertPortToDomainParseTree converts a port ParseTree to domain ParseTree.
func convertPortToDomainParseTree(portTree *treesitter.ParseTree) (*valueobject.ParseTree, error) {
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		return nil, err
	}

	// Convert port ParseNode to domain ParseNode
	domainRoot := convertPortToDomainParseNode(portTree.RootNode)

	// Create metadata from result statistics
	metadata, err := valueobject.NewParseMetadata(
		0, // duration will be set from result
		"go-tree-sitter-bare",
		"1.0.0",
	)
	if err != nil {
		return nil, err
	}

	// Create domain ParseTree
	domainTree, err := valueobject.NewParseTree(
		context.Background(),
		language,
		domainRoot,
		[]byte(portTree.Source),
		metadata,
	)
	if err != nil {
		return nil, err
	}

	return domainTree, nil
}

// convertPortToDomainParseNode recursively converts port ParseNode to domain ParseNode.
func convertPortToDomainParseNode(portNode *treesitter.ParseNode) *valueobject.ParseNode {
	if portNode == nil {
		return nil
	}

	domainNode := &valueobject.ParseNode{
		Type:      portNode.Type,
		StartByte: portNode.StartByte,
		EndByte:   portNode.EndByte,
		StartPos: valueobject.Position{
			Row:    portNode.StartPoint.Row,
			Column: portNode.StartPoint.Column,
		},
		EndPos: valueobject.Position{
			Row:    portNode.EndPoint.Row,
			Column: portNode.EndPoint.Column,
		},
		Children: make([]*valueobject.ParseNode, len(portNode.Children)),
	}

	// Convert children recursively
	for i, child := range portNode.Children {
		domainNode.Children[i] = convertPortToDomainParseNode(child)
	}

	return domainNode
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
