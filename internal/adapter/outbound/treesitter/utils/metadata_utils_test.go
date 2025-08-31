package utils

import (
	"codechunking/internal/domain/valueobject"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractPrecedingComments tests the extractPrecedingComments utility function.
// This is a RED PHASE test that defines expected behavior for extracting comments before a node.
func TestExtractPrecedingComments(t *testing.T) {
	tests := []struct {
		name             string
		parseTree        *mockParseTreeWithSource
		node             *valueobject.ParseNode
		expectedComments string
	}{
		{
			name: "extracts single line comment",
			parseTree: &mockParseTreeWithSource{
				source: []byte(`package main

// This is a simple function that adds two numbers
func Add(a, b int) int {
	return a + b
}`),
			},
			node: &valueobject.ParseNode{
				Type:     "function_declaration",
				StartPos: valueobject.Position{Row: 3, Column: 0}, // func Add line
			},
			expectedComments: "This is a simple function that adds two numbers",
		},
		{
			name: "extracts multi-line comment block",
			parseTree: &mockParseTreeWithSource{
				source: []byte(`package main

// Calculate performs complex mathematical operations.
// It takes two integer parameters and returns their sum.
// This function is optimized for performance.
func Calculate(x, y int) int {
	return x + y
}`),
			},
			node: &valueobject.ParseNode{
				Type:     "function_declaration",
				StartPos: valueobject.Position{Row: 5, Column: 0}, // func Calculate line
			},
			expectedComments: "Calculate performs complex mathematical operations.\nIt takes two integer parameters and returns their sum.\nThis function is optimized for performance.",
		},
		{
			name: "extracts comments with empty lines between",
			parseTree: &mockParseTreeWithSource{
				source: []byte(`package main

// First comment line
// Second comment line

// Third comment line after empty line
func MyFunction() {
	// implementation
}`),
			},
			node: &valueobject.ParseNode{
				Type:     "function_declaration",
				StartPos: valueobject.Position{Row: 6, Column: 0}, // func MyFunction line
			},
			expectedComments: "First comment line\nSecond comment line\nThird comment line after empty line",
		},
		{
			name: "returns empty for no preceding comments",
			parseTree: &mockParseTreeWithSource{
				source: []byte(`package main

func NoComments() {
	return
}`),
			},
			node: &valueobject.ParseNode{
				Type:     "function_declaration",
				StartPos: valueobject.Position{Row: 2, Column: 0}, // func NoComments line
			},
			expectedComments: "",
		},
		{
			name: "stops at non-comment, non-empty line",
			parseTree: &mockParseTreeWithSource{
				source: []byte(`package main

var GlobalVar int

// This comment belongs to the function
func TestFunc() {
	return
}`),
			},
			node: &valueobject.ParseNode{
				Type:     "function_declaration",
				StartPos: valueobject.Position{Row: 5, Column: 0}, // func TestFunc line
			},
			expectedComments: "This comment belongs to the function",
		},
		{
			name: "handles struct field comments",
			parseTree: &mockParseTreeWithSource{
				source: []byte(`package main

type User struct {
	// User's unique identifier
	ID int
	
	// User's display name
	Name string
}`),
			},
			node: &valueobject.ParseNode{
				Type:     "field_declaration",
				StartPos: valueobject.Position{Row: 4, Column: 1}, // ID field line
			},
			expectedComments: "User's unique identifier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPrecedingComments(tt.parseTree, tt.node)
			assert.Equal(t, tt.expectedComments, result, "Extracted comments should match expected")
		})
	}
}

// TestCreatePackageMetadata tests the createPackageMetadata utility function.
// This is a RED PHASE test that defines expected behavior for creating package-level metadata.
func TestCreatePackageMetadata(t *testing.T) {
	tests := []struct {
		name             string
		parseTree        *mockParseTreeWithCounts
		packageName      string
		expectedMetadata map[string]interface{}
	}{
		{
			name:        "creates metadata for main package with main function",
			packageName: "main",
			parseTree: &mockParseTreeWithCounts{
				source: []byte(`package main

import "fmt"
import "strings"

const Version = "1.0.0"
var GlobalVar int

type User struct {
	Name string
}

func main() {
	fmt.Println("Hello")
}

func helper() {
	// helper function
}`),
				nodeCounts: map[string]int{
					"import_declaration":   2,
					"function_declaration": 2,
					"method_declaration":   0,
					"type_declaration":     1,
					"var_declaration":      1,
					"const_declaration":    1,
				},
			},
			expectedMetadata: map[string]interface{}{
				"imports_count":   2,
				"functions_count": 2,
				"types_count":     1,
				"constants_count": 1,
				"variables_count": 1,
				"lines_of_code":   20,
				"has_main":        true,
				"is_executable":   true,
				"complexity":      "low",
				"has_tests":       false,
				"dependencies":    []string{"fmt", "strings"},
			},
		},
		{
			name:        "creates metadata for library package",
			packageName: "utils",
			parseTree: &mockParseTreeWithCounts{
				source: []byte(`package utils

import (
	"encoding/json"
	"net/http"
	"time"
)

const DefaultTimeout = 30 * time.Second

type Config struct {
	Host    string
	Port    int
	Timeout time.Duration
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) Validate() error {
	return nil
}`),
				nodeCounts: map[string]int{
					"import_declaration":   1, // Single import block
					"function_declaration": 1, // NewConfig
					"method_declaration":   1, // Validate method
					"type_declaration":     1, // Config struct
					"var_declaration":      0,
					"const_declaration":    1, // DefaultTimeout
				},
			},
			expectedMetadata: map[string]interface{}{
				"imports_count":   1,
				"functions_count": 2, // functions + methods
				"types_count":     1,
				"constants_count": 1,
				"variables_count": 0,
				"lines_of_code":   23,
				"has_main":        false,
				"is_executable":   false,
				"complexity":      "low",
				"has_tests":       false,
				"dependencies":    []string{"encoding/json", "net/http", "time"},
			},
		},
		{
			name:        "creates metadata for test package",
			packageName: "utils_test",
			parseTree: &mockParseTreeWithCounts{
				source: []byte(`package utils_test

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestFunction(t *testing.T) {
	assert.True(t, true)
}

func BenchmarkFunction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// benchmark code
	}
}`),
				nodeCounts: map[string]int{
					"import_declaration":   1,
					"function_declaration": 2,
					"method_declaration":   0,
					"type_declaration":     0,
					"var_declaration":      0,
					"const_declaration":    0,
				},
			},
			expectedMetadata: map[string]interface{}{
				"imports_count":   1,
				"functions_count": 2,
				"types_count":     0,
				"constants_count": 0,
				"variables_count": 0,
				"lines_of_code":   16,
				"has_main":        false,
				"is_executable":   false,
				"complexity":      "low",
				"has_tests":       true,
				"dependencies":    []string{"testing", "github.com/stretchr/testify/assert"},
			},
		},
		{
			name:        "creates metadata for complex package",
			packageName: "complex",
			parseTree: &mockParseTreeWithCounts{
				source: []byte(strings.Repeat("// Complex package with many constructs\n", 200)), // 200 lines
				nodeCounts: map[string]int{
					"import_declaration":   5,
					"function_declaration": 15,
					"method_declaration":   10,
					"type_declaration":     8,
					"var_declaration":      3,
					"const_declaration":    2,
				},
			},
			expectedMetadata: map[string]interface{}{
				"imports_count":   5,
				"functions_count": 25, // 15 functions + 10 methods
				"types_count":     8,
				"constants_count": 2,
				"variables_count": 3,
				"lines_of_code":   200,
				"has_main":        false,
				"is_executable":   false,
				"complexity":      "high",
				"has_tests":       false,
				"dependencies":    []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreatePackageMetadata(tt.parseTree, tt.packageName)

			// Verify all expected metadata fields
			for key, expectedValue := range tt.expectedMetadata {
				actualValue, exists := result[key]
				require.True(t, exists, "Metadata should contain key %s", key)
				assert.Equal(t, expectedValue, actualValue, "Metadata value for %s should match expected", key)
			}

			// Verify no unexpected fields
			assert.Len(t, result, len(tt.expectedMetadata), "Metadata should have expected number of fields")
		})
	}
}

// TestParseDocstring tests documentation string parsing utility.
// This is a RED PHASE test that defines expected behavior for parsing docstrings.
func TestParseDocstring(t *testing.T) {
	tests := []struct {
		name            string
		rawDocstring    string
		expectedSummary string
		expectedDetails string
		expectedTags    map[string][]string
	}{
		{
			name:            "parses simple docstring",
			rawDocstring:    "Calculate performs addition of two integers.",
			expectedSummary: "Calculate performs addition of two integers.",
			expectedDetails: "",
			expectedTags:    map[string][]string{},
		},
		{
			name: "parses docstring with details",
			rawDocstring: `Calculate performs addition of two integers.

This function takes two integer parameters and returns their sum.
It handles overflow by returning the maximum integer value.`,
			expectedSummary: "Calculate performs addition of two integers.",
			expectedDetails: "This function takes two integer parameters and returns their sum.\nIt handles overflow by returning the maximum integer value.",
			expectedTags:    map[string][]string{},
		},
		{
			name: "parses docstring with tags",
			rawDocstring: `Calculate performs addition of two integers.

This function handles integer arithmetic.

@param a The first integer
@param b The second integer
@return The sum of a and b
@throws OverflowError when result exceeds max int`,
			expectedSummary: "Calculate performs addition of two integers.",
			expectedDetails: "This function handles integer arithmetic.",
			expectedTags: map[string][]string{
				"param":  {"a The first integer", "b The second integer"},
				"return": {"The sum of a and b"},
				"throws": {"OverflowError when result exceeds max int"},
			},
		},
		{
			name: "parses Go-style docstring",
			rawDocstring: `NewUser creates a new User instance with the given name and email.

The function validates the email format and returns an error if invalid.
It also normalizes the name by trimming whitespace.

Example:
    user, err := NewUser("John Doe", "john@example.com")
    if err != nil {
        log.Fatal(err)
    }`,
			expectedSummary: "NewUser creates a new User instance with the given name and email.",
			expectedDetails: "The function validates the email format and returns an error if invalid.\nIt also normalizes the name by trimming whitespace.\n\nExample:\n    user, err := NewUser(\"John Doe\", \"john@example.com\")\n    if err != nil {\n        log.Fatal(err)\n    }",
			expectedTags:    map[string][]string{},
		},
		{
			name:            "handles empty docstring",
			rawDocstring:    "",
			expectedSummary: "",
			expectedDetails: "",
			expectedTags:    map[string][]string{},
		},
		{
			name:            "handles whitespace-only docstring",
			rawDocstring:    "   \n\n   \t   \n   ",
			expectedSummary: "",
			expectedDetails: "",
			expectedTags:    map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, details, tags := ParseDocstring(tt.rawDocstring)

			assert.Equal(t, tt.expectedSummary, summary, "Docstring summary should match expected")
			assert.Equal(t, tt.expectedDetails, details, "Docstring details should match expected")
			assert.Equal(t, tt.expectedTags, tags, "Docstring tags should match expected")
		})
	}
}

// TestExtractAnnotationsFromTags tests annotation extraction from struct field tags.
// This is a RED PHASE test that defines expected behavior for parsing struct field tags.
func TestExtractAnnotationsFromTags(t *testing.T) {
	tests := []struct {
		name                string
		tagString           string
		expectedAnnotations []AnnotationData
	}{
		{
			name:      "parses single json tag",
			tagString: `json:"user_id"`,
			expectedAnnotations: []AnnotationData{
				{Name: "json", Arguments: []string{"user_id"}},
			},
		},
		{
			name:      "parses multiple tags",
			tagString: `json:"user_id" db:"user_id" xml:"userId"`,
			expectedAnnotations: []AnnotationData{
				{Name: "json", Arguments: []string{"user_id"}},
				{Name: "db", Arguments: []string{"user_id"}},
				{Name: "xml", Arguments: []string{"userId"}},
			},
		},
		{
			name:      "parses complex json tag with options",
			tagString: `json:"user_id,omitempty" db:"user_id,not null"`,
			expectedAnnotations: []AnnotationData{
				{Name: "json", Arguments: []string{"user_id", "omitempty"}},
				{Name: "db", Arguments: []string{"user_id", "not null"}},
			},
		},
		{
			name:      "parses validation tags",
			tagString: `validate:"required,min=1,max=100" binding:"required"`,
			expectedAnnotations: []AnnotationData{
				{Name: "validate", Arguments: []string{"required", "min=1", "max=100"}},
				{Name: "binding", Arguments: []string{"required"}},
			},
		},
		{
			name:      "handles tag with dash for ignore",
			tagString: `json:"-" db:"user_id"`,
			expectedAnnotations: []AnnotationData{
				{Name: "json", Arguments: []string{"-"}},
				{Name: "db", Arguments: []string{"user_id"}},
			},
		},
		{
			name:                "handles empty tag string",
			tagString:           "",
			expectedAnnotations: []AnnotationData{},
		},
		{
			name:      "handles malformed tags gracefully",
			tagString: `json:"valid" invalid_tag db:"also_valid"`,
			expectedAnnotations: []AnnotationData{
				{Name: "json", Arguments: []string{"valid"}},
				{Name: "db", Arguments: []string{"also_valid"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractAnnotationsFromTags(tt.tagString)

			require.Len(t, result, len(tt.expectedAnnotations), "Number of annotations should match expected")

			for i, expected := range tt.expectedAnnotations {
				assert.Equal(t, expected.Name, result[i].Name, "Annotation %d name should match", i)
				assert.Equal(t, expected.Arguments, result[i].Arguments, "Annotation %d arguments should match", i)
			}
		})
	}
}

// TestCalculateComplexity tests code complexity calculation.
// This is a RED PHASE test that defines expected behavior for calculating code complexity.
func TestCalculateComplexity(t *testing.T) {
	tests := []struct {
		name               string
		metrics            ComplexityMetrics
		expectedComplexity string
		expectedScore      int
	}{
		{
			name: "calculates low complexity",
			metrics: ComplexityMetrics{
				FunctionCount:   5,
				MethodCount:     3,
				TypeCount:       2,
				LinesOfCode:     100,
				CyclomaticScore: 10,
				NestingDepth:    2,
				ImportCount:     3,
			},
			expectedComplexity: "low",
			expectedScore:      15,
		},
		{
			name: "calculates medium complexity",
			metrics: ComplexityMetrics{
				FunctionCount:   15,
				MethodCount:     10,
				TypeCount:       8,
				LinesOfCode:     500,
				CyclomaticScore: 50,
				NestingDepth:    4,
				ImportCount:     10,
			},
			expectedComplexity: "medium",
			expectedScore:      75,
		},
		{
			name: "calculates high complexity",
			metrics: ComplexityMetrics{
				FunctionCount:   50,
				MethodCount:     30,
				TypeCount:       20,
				LinesOfCode:     2000,
				CyclomaticScore: 200,
				NestingDepth:    8,
				ImportCount:     25,
			},
			expectedComplexity: "high",
			expectedScore:      250,
		},
		{
			name: "handles edge case with zero values",
			metrics: ComplexityMetrics{
				FunctionCount:   0,
				MethodCount:     0,
				TypeCount:       0,
				LinesOfCode:     0,
				CyclomaticScore: 0,
				NestingDepth:    0,
				ImportCount:     0,
			},
			expectedComplexity: "low",
			expectedScore:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complexity, score := CalculateComplexity(tt.metrics)

			assert.Equal(t, tt.expectedComplexity, complexity, "Complexity level should match expected")
			assert.Equal(t, tt.expectedScore, score, "Complexity score should match expected")
		})
	}
}

// Mock types for testing.
type mockParseTreeWithSource struct {
	source []byte
}

func (m *mockParseTreeWithSource) Source() []byte {
	return m.source
}

type mockParseTreeWithCounts struct {
	source     []byte
	nodeCounts map[string]int
}

func (m *mockParseTreeWithCounts) Source() []byte {
	return m.source
}

func (m *mockParseTreeWithCounts) GetNodesByType(nodeType string) []*valueobject.ParseNode {
	count := m.nodeCounts[nodeType]
	nodes := make([]*valueobject.ParseNode, count)
	for i := range nodes {
		nodes[i] = &valueobject.ParseNode{Type: nodeType}
	}
	return nodes
}

// Supporting types are defined in metadata_utils.go

// RED PHASE: These functions don't exist yet and will cause compilation errors.
// This is intentional - the tests define the expected behavior for implementation.

// Expected function signatures that need to be implemented:
// func ExtractPrecedingComments(parseTree ParseTreeWithSource, node *valueobject.ParseNode) string
// func CreatePackageMetadata(parseTree ParseTreeWithCounts, packageName string) map[string]interface{}
// func ParseDocstring(rawDocstring string) (summary, details string, tags map[string][]string)
// func ExtractAnnotationsFromTags(tagString string) []AnnotationData
// func CalculateComplexity(metrics ComplexityMetrics) (complexity string, score int)
