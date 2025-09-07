package pythonparser

import (
	treesitter "codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonClassExtraction(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "SimpleClassWithBasicMethods",
			code: `
class SimpleClass:
    def __init__(self, name):
        self.name = name
    
    def greet(self):
        return f"Hello, {self.name}"
    
    def _private_method(self):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "SimpleClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 7, Column: 9},
					ID:            "SimpleClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__init__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}, {Name: "name"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 25},
							ID:            "__init__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "greet",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 32},
							ID:            "greet",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "_private_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Private,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 8, Column: 4},
							EndPosition:   valueobject.Position{Row: 9, Column: 11},
							ID:            "_private_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ClassWithInheritance",
			code: `
class Parent:
    pass

class Child(Parent):
    def method(self):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Parent",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 8},
					ID:            "Parent",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "Child",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 4, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 11},
					ID:            "Child",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ClassWithMultipleInheritance",
			code: `
class A:
    pass

class B:
    pass

class C(A, B):
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "A",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 8},
					ID:            "A",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "B",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 4, Column: 0},
					EndPosition:   valueobject.Position{Row: 5, Column: 8},
					ID:            "B",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "C",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 7, Column: 0},
					EndPosition:   valueobject.Position{Row: 8, Column: 8},
					ID:            "C",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "ClassWithDecorators",
			code: `
from dataclasses import dataclass

@dataclass
class Person:
    name: str
    age: int
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Person",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Annotations:   []outbound.Annotation{{Name: "dataclass"}},
					StartPosition: valueobject.Position{Row: 3, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 12},
					ID:            "Person",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "name",
							Type:          outbound.ConstructField,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 5, Column: 12},
							ID:            "name",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "age",
							Type:          outbound.ConstructField,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 12},
							ID:            "age",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "NestedClasses",
			code: `
class Outer:
    class Inner:
        def method(self):
            pass
    
    def outer_method(self):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Outer",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 7, Column: 11},
					ID:            "Outer",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "Inner",
							Type:          outbound.ConstructClass,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 4, Column: 15},
							ID:            "Inner",
							ExtractedAt:   time.Time{},
							Hash:          "",
							ChildChunks: []outbound.SemanticCodeChunk{
								{
									Name:          "method",
									Type:          outbound.ConstructMethod,
									Visibility:    outbound.Public,
									Language:      pythonLang,
									Parameters:    []outbound.Parameter{{Name: "self"}},
									StartPosition: valueobject.Position{Row: 3, Column: 8},
									EndPosition:   valueobject.Position{Row: 4, Column: 15},
									ID:            "method",
									ExtractedAt:   time.Time{},
									Hash:          "",
								},
							},
						},
						{
							Name:          "outer_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 7, Column: 11},
							ID:            "outer_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "AbstractBaseClass",
			code: `
from abc import ABC, abstractmethod

class AbstractClass(ABC):
    @abstractmethod
    def abstract_method(self):
        pass
    
    def concrete_method(self):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "AbstractClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 3, Column: 0},
					EndPosition:   valueobject.Position{Row: 8, Column: 11},
					ID:            "AbstractClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "abstract_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "abstractmethod"}},
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "abstract_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "concrete_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 8, Column: 4},
							EndPosition:   valueobject.Position{Row: 9, Column: 11},
							ID:            "concrete_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ClassWithSpecialMethods",
			code: `
class SpecialClass:
    def __init__(self):
        pass
    
    def __str__(self):
        return "SpecialClass"
    
    def __repr__(self):
        return "SpecialClass()"
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "SpecialClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 8, Column: 32},
					ID:            "SpecialClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__init__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 11},
							ID:            "__init__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "__str__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 29},
							ID:            "__str__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "__repr__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 8, Column: 4},
							EndPosition:   valueobject.Position{Row: 9, Column: 32},
							ID:            "__repr__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ClassWithStaticAndClassMethods",
			code: `
class MethodTypes:
    @staticmethod
    def static_method():
        pass
    
    @classmethod
    def class_method(cls):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "MethodTypes",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 7, Column: 11},
					ID:            "MethodTypes",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "static_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "staticmethod"}},
							Parameters:    []outbound.Parameter{},
							StartPosition: valueobject.Position{Row: 3, Column: 4},
							EndPosition:   valueobject.Position{Row: 4, Column: 11},
							ID:            "static_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "class_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "classmethod"}},
							Parameters:    []outbound.Parameter{{Name: "cls"}},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 7, Column: 11},
							ID:            "class_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "GenericClassWithTypeHints",
			code: `
from typing import Generic, TypeVar

T = TypeVar('T')

class Container(Generic[T]):
    def __init__(self, value: T):
        self.value = value
    
    def get_value(self) -> T:
        return self.value
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Container",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 5, Column: 0},
					EndPosition:   valueobject.Position{Row: 9, Column: 24},
					ID:            "Container",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__init__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}, {Name: "value"}},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 7, Column: 25},
							ID:            "__init__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "get_value",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							ReturnType:    "T",
							StartPosition: valueobject.Position{Row: 9, Column: 4},
							EndPosition:   valueobject.Position{Row: 10, Column: 24},
							ID:            "get_value",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ClassWithDocstrings",
			code: `
class DocumentedClass:
    """This is a class docstring."""
    
    def method(self):
        """This is a method docstring."""
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "DocumentedClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "This is a class docstring.",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 11},
					ID:            "DocumentedClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							Documentation: "This is a method docstring.",
							StartPosition: valueobject.Position{Row: 4, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractClasses(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonFunctionParsing(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "RegularFunctionWithVariousParameters",
			code: `
def regular_function(param1, param2: str, param3=5, *args, **kwargs):
    """A regular function with various parameter types."""
    return param1
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "regular_function",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "A regular function with various parameter types.",
					Parameters: []outbound.Parameter{
						{Name: "param1"},
						{Name: "param2"},
						{Name: "param3", DefaultValue: "5"},
						{Name: "args", IsVariadic: true},
						{Name: "kwargs", IsVariadic: true},
					},
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 18},
					ID:            "regular_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "AsyncFunction",
			code: `
async def async_function():
    await something()
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "async_function",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					IsAsync:       true,
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 21},
					ID:            "async_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "GeneratorFunction",
			code: `
def generator_function():
    yield 1
    yield 2
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "generator_function",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					IsAsync:       false,
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 11},
					ID:            "generator_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "LambdaFunction",
			code: `
lambda_func = lambda x, y: x + y
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "lambda_func",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 31},
					ID:            "lambda_func",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "DecoratedFunction",
			code: `
@decorator
def decorated_function():
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "decorated_function",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Annotations:   []outbound.Annotation{{Name: "decorator"}},
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 8},
					ID:            "decorated_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "FunctionWithTypeHints",
			code: `
def typed_function(x: int, y: str) -> bool:
    return True
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:       "typed_function",
					Type:       outbound.ConstructFunction,
					Visibility: outbound.Public,
					Language:   pythonLang,
					Parameters: []outbound.Parameter{
						{Name: "x"},
						{Name: "y"},
					},
					ReturnType:    "bool",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 17},
					ID:            "typed_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "NestedFunctions",
			code: `
def outer_function():
    def inner_function():
        pass
    inner_function()
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "outer_function",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 4, Column: 20},
					ID:            "outer_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "inner_function",
							Type:          outbound.ConstructFunction,
							Visibility:    outbound.Private,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 11},
							ID:            "inner_function",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "PrivateFunction",
			code: `
def _private_function():
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "_private_function",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Private,
					Language:      pythonLang,
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 8},
					ID:            "_private_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractFunctions(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonMethodParsing(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "InstanceMethod",
			code: `
class TestClass:
    def instance_method(self, param):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "TestClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 11},
					ID:            "TestClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "instance_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}, {Name: "param"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 11},
							ID:            "instance_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ClassMethod",
			code: `
class TestClass:
    @classmethod
    def class_method(cls):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "TestClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 4, Column: 11},
					ID:            "TestClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "class_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "classmethod"}},
							Parameters:    []outbound.Parameter{{Name: "cls"}},
							StartPosition: valueobject.Position{Row: 3, Column: 4},
							EndPosition:   valueobject.Position{Row: 4, Column: 11},
							ID:            "class_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "StaticMethod",
			code: `
class TestClass:
    @staticmethod
    def static_method():
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "TestClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 4, Column: 11},
					ID:            "TestClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "static_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "staticmethod"}},
							Parameters:    []outbound.Parameter{},
							StartPosition: valueobject.Position{Row: 3, Column: 4},
							EndPosition:   valueobject.Position{Row: 4, Column: 11},
							ID:            "static_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "PropertyMethods",
			code: `
class TestClass:
    @property
    def prop(self):
        return self._prop
    
    @prop.setter
    def prop(self, value):
        self._prop = value
    
    @prop.deleter
    def prop(self):
        del self._prop
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "TestClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 11, Column: 19},
					ID:            "TestClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "prop",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "property"}},
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 3, Column: 4},
							EndPosition:   valueobject.Position{Row: 4, Column: 23},
							ID:            "prop",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "prop",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "prop.setter"}},
							Parameters:    []outbound.Parameter{{Name: "self"}, {Name: "value"}},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 7, Column: 24},
							ID:            "prop",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "prop",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "prop.deleter"}},
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 9, Column: 4},
							EndPosition:   valueobject.Position{Row: 10, Column: 19},
							ID:            "prop",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "SpecialMethods",
			code: `
class TestClass:
    def __init__(self):
        pass
    
    def __call__(self):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "TestClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 11},
					ID:            "TestClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__init__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 11},
							ID:            "__init__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "__call__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "__call__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "PrivateMethods",
			code: `
class TestClass:
    def __private_method(self):
        pass
    
    def _protected_method(self):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "TestClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 11},
					ID:            "TestClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__private_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Private,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 11},
							ID:            "__private_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "_protected_method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Private,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "_protected_method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "MethodOverriding",
			code: `
class Parent:
    def method(self):
        pass

class Child(Parent):
    def method(self):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Parent",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 11},
					ID:            "Parent",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 11},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "Child",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 5, Column: 0},
					EndPosition:   valueobject.Position{Row: 7, Column: 11},
					ID:            "Child",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 7, Column: 11},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractClasses(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonVariableExtraction(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "ModuleLevelVariables",
			code: `
module_var = "value"
another_var = 42
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "module_var",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 20},
					ID:            "module_var",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "another_var",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 17},
					ID:            "another_var",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "ClassVariablesVsInstanceVariables",
			code: `
class TestClass:
    class_var = "shared"
    
    def __init__(self):
        self.instance_var = "instance"
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "TestClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 5, Column: 35},
					ID:            "TestClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "class_var",
							Type:          outbound.ConstructField,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 2, Column: 23},
							ID:            "class_var",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "__init__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 4, Column: 4},
							EndPosition:   valueobject.Position{Row: 5, Column: 35},
							ID:            "__init__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "TypeAnnotatedVariables",
			code: `
var: int = 5
name: str = "test"
values: list[int] = []
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "var",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 12},
					ID:            "var",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "name",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 17},
					ID:            "name",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "values",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 3, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 23},
					ID:            "values",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "PrivateVariables",
			code: `
_private_var = "private"
__dunder_var = "dunder"
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "_private_var",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Private,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 23},
					ID:            "_private_var",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "__dunder_var",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Private,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 22},
					ID:            "__dunder_var",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "Constants",
			code: `
CONSTANT_VALUE = "constant"
MAX_SIZE = 100
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "CONSTANT_VALUE",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 26},
					ID:            "CONSTANT_VALUE",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "MAX_SIZE",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 15},
					ID:            "MAX_SIZE",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "GlobalAndNonlocalVariables",
			code: `
global_var = "global"

def outer():
    nonlocal_var = "nonlocal"
    
    def inner():
        global global_var
        nonlocal nonlocal_var
        local_var = "local"
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "global_var",
					Type:          outbound.ConstructVariable,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 20},
					ID:            "global_var",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "outer",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 3, Column: 0},
					EndPosition:   valueobject.Position{Row: 8, Column: 26},
					ID:            "outer",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "inner",
							Type:          outbound.ConstructFunction,
							Visibility:    outbound.Private,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 9, Column: 26},
							ID:            "inner",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractVariables(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonImportHandling(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "BasicImports",
			code: `
import os
import sys
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "os",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 9},
					ID:            "os",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "sys",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 9},
					ID:            "sys",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "FromImports",
			code: `
from typing import List, Dict
from os import path
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "List",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 25},
					ID:            "List",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "Dict",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 25},
					ID:            "Dict",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "path",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 16},
					ID:            "path",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "AliasedImports",
			code: `
import numpy as np
import pandas as pd
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "np",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 18},
					ID:            "np",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "pd",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 18},
					ID:            "pd",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "WildcardImports",
			code: `
from module import *
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "*",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 20},
					ID:            "*",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "RelativeImports",
			code: `
from .module import name
from ..parent import var
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "name",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 24},
					ID:            "name",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "var",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 24},
					ID:            "var",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "MultipleImports",
			code: `
import a, b, c
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "a",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 14},
					ID:            "a",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "b",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 14},
					ID:            "b",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "c",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 1, Column: 14},
					ID:            "c",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractImports(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonModuleProcessing(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "ModuleLevelDocstring",
			code: `
"""This is a module docstring."""

def function():
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "__main__",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "This is a module docstring.",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 4, Column: 8},
					ID:            "__main__",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "function",
							Type:          outbound.ConstructFunction,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{},
							StartPosition: valueobject.Position{Row: 3, Column: 0},
							EndPosition:   valueobject.Position{Row: 4, Column: 8},
							ID:            "function",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "__all__Declaration",
			code: `
__all__ = ['public_function', 'PUBLIC_CONSTANT']

def public_function():
    pass

PUBLIC_CONSTANT = 42
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "__main__",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 21},
					ID:            "__main__",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "public_function",
							Type:          outbound.ConstructFunction,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{},
							StartPosition: valueobject.Position{Row: 3, Column: 0},
							EndPosition:   valueobject.Position{Row: 4, Column: 8},
							ID:            "public_function",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "PUBLIC_CONSTANT",
							Type:          outbound.ConstructVariable,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 6, Column: 0},
							EndPosition:   valueobject.Position{Row: 6, Column: 21},
							ID:            "PUBLIC_CONSTANT",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "PackageInitProcessing",
			code: `
"""Package init module."""

from .module1 import Class1
from .module2 import function2

__all__ = ['Class1', 'function2']
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "__main__",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "Package init module.",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 32},
					ID:            "__main__",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "Class1",
							Type:          outbound.ConstructModule,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 3, Column: 0},
							EndPosition:   valueobject.Position{Row: 3, Column: 30},
							ID:            "Class1",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "function2",
							Type:          outbound.ConstructModule,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 4, Column: 0},
							EndPosition:   valueobject.Position{Row: 4, Column: 32},
							ID:            "function2",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractModules(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonInterfaceProtocol(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "ProtocolClass",
			code: `
from typing import Protocol

class Drawable(Protocol):
    def draw(self) -> None:
        ...
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Drawable",
					Type:          outbound.ConstructInterface,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 3, Column: 0},
					EndPosition:   valueobject.Position{Row: 5, Column: 11},
					ID:            "Drawable",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "draw",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							ReturnType:    "None",
							StartPosition: valueobject.Position{Row: 4, Column: 4},
							EndPosition:   valueobject.Position{Row: 5, Column: 11},
							ID:            "draw",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "AbstractBaseClassInterface",
			code: `
from abc import ABC, abstractmethod

class Shape(ABC):
    @abstractmethod
    def area(self) -> float:
        pass
    
    @abstractmethod
    def perimeter(self) -> float:
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Shape",
					Type:          outbound.ConstructInterface,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 3, Column: 0},
					EndPosition:   valueobject.Position{Row: 9, Column: 11},
					ID:            "Shape",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "area",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "abstractmethod"}},
							Parameters:    []outbound.Parameter{{Name: "self"}},
							ReturnType:    "float",
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "area",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "perimeter",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "abstractmethod"}},
							Parameters:    []outbound.Parameter{{Name: "self"}},
							ReturnType:    "float",
							StartPosition: valueobject.Position{Row: 8, Column: 4},
							EndPosition:   valueobject.Position{Row: 9, Column: 11},
							ID:            "perimeter",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractInterfaces(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonAdvancedFeatures(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "DecoratorParsing",
			code: `
def my_decorator(func):
    def wrapper(*args, **kwargs):
        return func(*args, **kwargs)
    return wrapper

@my_decorator
def decorated_function():
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "my_decorator",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Parameters:    []outbound.Parameter{{Name: "func"}},
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 4, Column: 19},
					ID:            "my_decorator",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:       "wrapper",
							Type:       outbound.ConstructFunction,
							Visibility: outbound.Private,
							Language:   pythonLang,
							Parameters: []outbound.Parameter{
								{Name: "args", IsVariadic: true},
								{Name: "kwargs", IsVariadic: true},
							},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 37},
							ID:            "wrapper",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "decorated_function",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Annotations:   []outbound.Annotation{{Name: "my_decorator"}},
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 6, Column: 0},
					EndPosition:   valueobject.Position{Row: 7, Column: 8},
					ID:            "decorated_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "TypeHintParsing",
			code: `
from typing import Union, Optional, Generic, TypeVar

T = TypeVar('T')

def complex_function(param: Union[str, int], optional: Optional[str] = None) -> Generic[T]:
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:       "complex_function",
					Type:       outbound.ConstructFunction,
					Visibility: outbound.Public,
					Language:   pythonLang,
					Parameters: []outbound.Parameter{
						{Name: "param"},
						{Name: "optional", DefaultValue: "None"},
					},
					ReturnType:    "Generic[T]",
					StartPosition: valueobject.Position{Row: 5, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 8},
					ID:            "complex_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "AsyncAwaitPattern",
			code: `
import asyncio

async def main():
    await asyncio.sleep(1)
    result = await fetch_data()
    return result

async def fetch_data():
    return "data"
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "main",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					IsAsync:       true,
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 3, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 19},
					ID:            "main",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "fetch_data",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					IsAsync:       true,
					Parameters:    []outbound.Parameter{},
					StartPosition: valueobject.Position{Row: 8, Column: 0},
					EndPosition:   valueobject.Position{Row: 9, Column: 17},
					ID:            "fetch_data",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "ContextManagerMethods",
			code: `
class ContextManager:
    def __enter__(self):
        return self
    
    def __exit__(self, exc_type, exc_value, traceback):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "ContextManager",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 11},
					ID:            "ContextManager",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__enter__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 19},
							ID:            "__enter__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:       "__exit__",
							Type:       outbound.ConstructMethod,
							Visibility: outbound.Public,
							Language:   pythonLang,
							Parameters: []outbound.Parameter{
								{Name: "self"},
								{Name: "exc_type"},
								{Name: "exc_value"},
								{Name: "traceback"},
							},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "__exit__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "IteratorProtocol",
			code: `
class MyIterator:
    def __iter__(self):
        return self
    
    def __next__(self):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "MyIterator",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 11},
					ID:            "MyIterator",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__iter__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 19},
							ID:            "__iter__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "__next__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "__next__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "DescriptorProtocol",
			code: `
class MyDescriptor:
    def __get__(self, obj, owner):
        pass
    
    def __set__(self, obj, value):
        pass
    
    def __delete__(self, obj):
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "MyDescriptor",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 9, Column: 11},
					ID:            "MyDescriptor",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__get__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}, {Name: "obj"}, {Name: "owner"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 11},
							ID:            "__get__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "__set__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}, {Name: "obj"}, {Name: "value"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 11},
							ID:            "__set__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "__delete__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}, {Name: "obj"}},
							StartPosition: valueobject.Position{Row: 8, Column: 4},
							EndPosition:   valueobject.Position{Row: 9, Column: 11},
							ID:            "__delete__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "MetaclassDetection",
			code: `
class MetaClass(type):
    pass

class MyClass(metaclass=MetaClass):
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "MetaClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 8},
					ID:            "MetaClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "MyClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 4, Column: 0},
					EndPosition:   valueobject.Position{Row: 5, Column: 8},
					ID:            "MyClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "PropertyGetterSetterDeleter",
			code: `
class PropertyClass:
    def __init__(self):
        self._value = 0
    
    @property
    def value(self):
        return self._value
    
    @value.setter
    def value(self, val):
        self._value = val
    
    @value.deleter
    def value(self):
        del self._value
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "PropertyClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 13, Column: 20},
					ID:            "PropertyClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "__init__",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 24},
							ID:            "__init__",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "value",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "property"}},
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 5, Column: 4},
							EndPosition:   valueobject.Position{Row: 6, Column: 26},
							ID:            "value",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "value",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "value.setter"}},
							Parameters:    []outbound.Parameter{{Name: "self"}, {Name: "val"}},
							StartPosition: valueobject.Position{Row: 8, Column: 4},
							EndPosition:   valueobject.Position{Row: 9, Column: 24},
							ID:            "value",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "value",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Annotations:   []outbound.Annotation{{Name: "value.deleter"}},
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 11, Column: 4},
							EndPosition:   valueobject.Position{Row: 12, Column: 20},
							ID:            "value",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ExceptionClassInheritance",
			code: `
class CustomError(Exception):
    pass

class SpecificError(CustomError):
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "CustomError",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 2, Column: 8},
					ID:            "CustomError",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "SpecificError",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 4, Column: 0},
					EndPosition:   valueobject.Position{Row: 5, Column: 8},
					ID:            "SpecificError",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractClasses(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonDocumentationStrings(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "ModuleDocstrings",
			code: `
"""This is a module docstring.

It can span multiple lines.
"""

def function():
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "__main__",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "This is a module docstring.\n\nIt can span multiple lines.",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 8, Column: 8},
					ID:            "__main__",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "function",
							Type:          outbound.ConstructFunction,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{},
							StartPosition: valueobject.Position{Row: 7, Column: 0},
							EndPosition:   valueobject.Position{Row: 8, Column: 8},
							ID:            "function",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ClassDocstrings",
			code: `
class DocumentedClass:
    """A well-documented class.
    
    This class does something important.
    """
    
    def method(self):
        """A method docstring."""
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "DocumentedClass",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "A well-documented class.\n\nThis class does something important.",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 9, Column: 11},
					ID:            "DocumentedClass",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							Documentation: "A method docstring.",
							StartPosition: valueobject.Position{Row: 7, Column: 4},
							EndPosition:   valueobject.Position{Row: 9, Column: 11},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "FunctionDocstrings",
			code: `
def documented_function(param1, param2):
    """This function takes two parameters.
    
    Args:
        param1: The first parameter.
        param2: The second parameter.
    
    Returns:
        A string value.
    """
    return "result"
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "documented_function",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "This function takes two parameters.\n\nArgs:\n    param1: The first parameter.\n    param2: The second parameter.\n\nReturns:\n    A string value.",
					Parameters:    []outbound.Parameter{{Name: "param1"}, {Name: "param2"}},
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 11, Column: 20},
					ID:            "documented_function",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "DocstringFormatDetection",
			code: `
def google_style_func(param1: str, param2: int) -> bool:
    """Summary of function.

    Args:
        param1 (str): Description of param1.
        param2 (int): Description of param2.
    
    Returns:
        bool: Description of return value.
    """
    return True

def sphinx_style_func(param1, param2):
    """Summary of function.
    
    :param param1: Description of param1
    :type param1: str
    :param param2: Description of param2
    :type param2: int
    :returns: Description of return value
    :rtype: bool
    """
    return True
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "google_style_func",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "Summary of function.\n\nArgs:\n    param1 (str): Description of param1.\n    param2 (int): Description of param2.\n\nReturns:\n    bool: Description of return value.",
					Parameters: []outbound.Parameter{
						{Name: "param1"},
						{Name: "param2"},
					},
					ReturnType:    "bool",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 11, Column: 16},
					ID:            "google_style_func",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "sphinx_style_func",
					Type:          outbound.ConstructFunction,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "Summary of function.\n\n:param param1: Description of param1\n:type param1: str\n:param param2: Description of param2\n:type param2: int\n:returns: Description of return value\n:rtype: bool",
					Parameters:    []outbound.Parameter{{Name: "param1"}, {Name: "param2"}},
					StartPosition: valueobject.Position{Row: 13, Column: 0},
					EndPosition:   valueobject.Position{Row: 22, Column: 16},
					ID:            "sphinx_style_func",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "MultiLineDocstringParsing",
			code: `
class MultiLineDocstring:
    """
    This is a multi-line docstring.
    
    It has multiple paragraphs.
    
    And even more text here.
    """
    
    def method(self):
        """
        This method also has a multi-line docstring.
        
        With several paragraphs too.
        """
        pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "MultiLineDocstring",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "This is a multi-line docstring.\n\nIt has multiple paragraphs.\n\nAnd even more text here.",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 11, Column: 11},
					ID:            "MultiLineDocstring",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							Documentation: "This method also has a multi-line docstring.\n\nWith several paragraphs too.",
							StartPosition: valueobject.Position{Row: 9, Column: 4},
							EndPosition:   valueobject.Position{Row: 11, Column: 11},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractClasses(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonIntegration(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name     string
		code     string
		expected []outbound.SemanticCodeChunk
	}{
		{
			name: "FullPythonFileParsing",
			code: `
"""Module docstring for testing."""

import os
from typing import List

CONSTANT = "value"

class ParentClass:
    class_var = 10
    
    def __init__(self, name: str):
        self.name = name

class ChildClass(ParentClass):
    """Child class inheriting from ParentClass."""
    
    def __init__(self, name: str, age: int):
        super().__init__(name)
        self.age = age
    
    @property
    def info(self):
        return f"{self.name} is {self.age} years old"
    
    def greet(self) -> str:
        """Greet method."""
        return f"Hello, {self.name}"

def main_function(param: List[str] = None) -> None:
    """Main function docstring."""
    child = ChildClass("Alice", 30)
    print(child.info)
    print(child.greet())

if __name__ == "__main__":
    main_function()
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "__main__",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					Documentation: "Module docstring for testing.",
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 25, Column: 21},
					ID:            "__main__",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "os",
							Type:          outbound.ConstructModule,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 4, Column: 0},
							EndPosition:   valueobject.Position{Row: 4, Column: 9},
							ID:            "os",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "List",
							Type:          outbound.ConstructModule,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 5, Column: 0},
							EndPosition:   valueobject.Position{Row: 5, Column: 21},
							ID:            "List",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "CONSTANT",
							Type:          outbound.ConstructVariable,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 7, Column: 0},
							EndPosition:   valueobject.Position{Row: 7, Column: 18},
							ID:            "CONSTANT",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "ParentClass",
							Type:          outbound.ConstructClass,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 9, Column: 0},
							EndPosition:   valueobject.Position{Row: 13, Column: 25},
							ID:            "ParentClass",
							ExtractedAt:   time.Time{},
							Hash:          "",
							ChildChunks: []outbound.SemanticCodeChunk{
								{
									Name:          "class_var",
									Type:          outbound.ConstructField,
									Visibility:    outbound.Public,
									Language:      pythonLang,
									StartPosition: valueobject.Position{Row: 10, Column: 4},
									EndPosition:   valueobject.Position{Row: 10, Column: 17},
									ID:            "class_var",
									ExtractedAt:   time.Time{},
									Hash:          "",
								},
								{
									Name:       "__init__",
									Type:       outbound.ConstructMethod,
									Visibility: outbound.Public,
									Language:   pythonLang,
									Parameters: []outbound.Parameter{
										{Name: "self"},
										{Name: "name"},
									},
									StartPosition: valueobject.Position{Row: 12, Column: 4},
									EndPosition:   valueobject.Position{Row: 13, Column: 25},
									ID:            "__init__",
									ExtractedAt:   time.Time{},
									Hash:          "",
								},
							},
						},
						{
							Name:          "ChildClass",
							Type:          outbound.ConstructClass,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Documentation: "Child class inheriting from ParentClass.",
							StartPosition: valueobject.Position{Row: 15, Column: 0},
							EndPosition:   valueobject.Position{Row: 23, Column: 38},
							ID:            "ChildClass",
							ExtractedAt:   time.Time{},
							Hash:          "",
							ChildChunks: []outbound.SemanticCodeChunk{
								{
									Name:       "__init__",
									Type:       outbound.ConstructMethod,
									Visibility: outbound.Public,
									Language:   pythonLang,
									Parameters: []outbound.Parameter{
										{Name: "self"},
										{Name: "name"},
										{Name: "age"},
									},
									StartPosition: valueobject.Position{Row: 18, Column: 4},
									EndPosition:   valueobject.Position{Row: 20, Column: 23},
									ID:            "__init__",
									ExtractedAt:   time.Time{},
									Hash:          "",
								},
								{
									Name:          "info",
									Type:          outbound.ConstructMethod,
									Visibility:    outbound.Public,
									Language:      pythonLang,
									Annotations:   []outbound.Annotation{{Name: "property"}},
									Parameters:    []outbound.Parameter{{Name: "self"}},
									StartPosition: valueobject.Position{Row: 22, Column: 4},
									EndPosition:   valueobject.Position{Row: 23, Column: 50},
									ID:            "info",
									ExtractedAt:   time.Time{},
									Hash:          "",
								},
								{
									Name:          "greet",
									Type:          outbound.ConstructMethod,
									Visibility:    outbound.Public,
									Language:      pythonLang,
									Parameters:    []outbound.Parameter{{Name: "self"}},
									ReturnType:    "str",
									Documentation: "Greet method.",
									StartPosition: valueobject.Position{Row: 25, Column: 4},
									EndPosition:   valueobject.Position{Row: 27, Column: 38},
									ID:            "greet",
									ExtractedAt:   time.Time{},
									Hash:          "",
								},
							},
						},
						{
							Name:          "main_function",
							Type:          outbound.ConstructFunction,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Documentation: "Main function docstring.",
							Parameters: []outbound.Parameter{
								{Name: "param", DefaultValue: "None"},
							},
							ReturnType:    "None",
							StartPosition: valueobject.Position{Row: 29, Column: 0},
							EndPosition:   valueobject.Position{Row: 33, Column: 25},
							ID:            "main_function",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "ClassHierarchyReconstruction",
			code: `
class Animal:
    def speak(self):
        pass

class Dog(Animal):
    def speak(self):
        return "Woof"

class Cat(Animal):
    def speak(self):
        return "Meow"

class Robot:
    def speak(self):
        return "Beep"
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Animal",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 11},
					ID:            "Animal",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "speak",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 11},
							ID:            "speak",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "Dog",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 5, Column: 0},
					EndPosition:   valueobject.Position{Row: 7, Column: 19},
					ID:            "Dog",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "speak",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 7, Column: 19},
							ID:            "speak",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "Cat",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 9, Column: 0},
					EndPosition:   valueobject.Position{Row: 11, Column: 19},
					ID:            "Cat",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "speak",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 10, Column: 4},
							EndPosition:   valueobject.Position{Row: 11, Column: 19},
							ID:            "speak",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "Robot",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 13, Column: 0},
					EndPosition:   valueobject.Position{Row: 15, Column: 19},
					ID:            "Robot",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "speak",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 14, Column: 4},
							EndPosition:   valueobject.Position{Row: 15, Column: 19},
							ID:            "speak",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
		{
			name: "MethodResolutionOrderAnalysis",
			code: `
class A:
    def method(self):
        return "A"

class B(A):
    def method(self):
        return "B"

class C(A):
    def method(self):
        return "C"

class D(B, C):
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "A",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 19},
					ID:            "A",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 19},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "B",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 5, Column: 0},
					EndPosition:   valueobject.Position{Row: 7, Column: 19},
					ID:            "B",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 7, Column: 19},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "C",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 9, Column: 0},
					EndPosition:   valueobject.Position{Row: 11, Column: 19},
					ID:            "C",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "method",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 10, Column: 4},
							EndPosition:   valueobject.Position{Row: 11, Column: 19},
							ID:            "method",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "D",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 13, Column: 0},
					EndPosition:   valueobject.Position{Row: 14, Column: 8},
					ID:            "D",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "CrossReferencesBetweenClassesAndFunctions",
			code: `
class Database:
    def connect(self):
        return Connection()

class Connection:
    def query(self, sql: str):
        return ResultSet()

def execute_query(db: Database, sql: str) -> ResultSet:
    conn = db.connect()
    return conn.query(sql)

class ResultSet:
    pass
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "Database",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 1, Column: 0},
					EndPosition:   valueobject.Position{Row: 3, Column: 26},
					ID:            "Database",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "connect",
							Type:          outbound.ConstructMethod,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							Parameters:    []outbound.Parameter{{Name: "self"}},
							StartPosition: valueobject.Position{Row: 2, Column: 4},
							EndPosition:   valueobject.Position{Row: 3, Column: 26},
							ID:            "connect",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:          "Connection",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 5, Column: 0},
					EndPosition:   valueobject.Position{Row: 7, Column: 25},
					ID:            "Connection",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:       "query",
							Type:       outbound.ConstructMethod,
							Visibility: outbound.Public,
							Language:   pythonLang,
							Parameters: []outbound.Parameter{
								{Name: "self"},
								{Name: "sql"},
							},
							StartPosition: valueobject.Position{Row: 6, Column: 4},
							EndPosition:   valueobject.Position{Row: 7, Column: 25},
							ID:            "query",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
				{
					Name:       "execute_query",
					Type:       outbound.ConstructFunction,
					Visibility: outbound.Public,
					Language:   pythonLang,
					Parameters: []outbound.Parameter{
						{Name: "db"},
						{Name: "sql"},
					},
					ReturnType:    "ResultSet",
					StartPosition: valueobject.Position{Row: 9, Column: 0},
					EndPosition:   valueobject.Position{Row: 11, Column: 25},
					ID:            "execute_query",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
				{
					Name:          "ResultSet",
					Type:          outbound.ConstructClass,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 13, Column: 0},
					EndPosition:   valueobject.Position{Row: 14, Column: 8},
					ID:            "ResultSet",
					ExtractedAt:   time.Time{},
					Hash:          "",
				},
			},
		},
		{
			name: "PackageStructureAnalysis",
			code: `
# __init__.py
from .module1 import ClassA
from .module2 import function_b
from .subpkg.module3 import ClassC

__all__ = ['ClassA', 'function_b', 'ClassC']
`,
			expected: []outbound.SemanticCodeChunk{
				{
					Name:          "__main__",
					Type:          outbound.ConstructModule,
					Visibility:    outbound.Public,
					Language:      pythonLang,
					StartPosition: valueobject.Position{Row: 2, Column: 0},
					EndPosition:   valueobject.Position{Row: 6, Column: 42},
					ID:            "__main__",
					ExtractedAt:   time.Time{},
					Hash:          "",
					ChildChunks: []outbound.SemanticCodeChunk{
						{
							Name:          "ClassA",
							Type:          outbound.ConstructModule,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 2, Column: 0},
							EndPosition:   valueobject.Position{Row: 2, Column: 30},
							ID:            "ClassA",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "function_b",
							Type:          outbound.ConstructModule,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 3, Column: 0},
							EndPosition:   valueobject.Position{Row: 3, Column: 32},
							ID:            "function_b",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
						{
							Name:          "ClassC",
							Type:          outbound.ConstructModule,
							Visibility:    outbound.Public,
							Language:      pythonLang,
							StartPosition: valueobject.Position{Row: 4, Column: 0},
							EndPosition:   valueobject.Position{Row: 4, Column: 39},
							ID:            "ClassC",
							ExtractedAt:   time.Time{},
							Hash:          "",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))
			require.NoError(t, err)
			require.NotNil(t, portResult.ParseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
			require.NoError(t, err)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := outbound.SemanticExtractionOptions{
				IncludePrivate:       true,
				IncludeDocumentation: true,
			}
			chunks, err := adapter.ExtractClasses(ctx, domainTree, options)
			require.NoError(t, err)

			// Set dynamic fields to expected values for comparison
			for i := range tt.expected {
				tt.expected[i].ExtractedAt = chunks[i].ExtractedAt
				tt.expected[i].Hash = chunks[i].Hash
				for j := range tt.expected[i].ChildChunks {
					tt.expected[i].ChildChunks[j].ExtractedAt = chunks[i].ChildChunks[j].ExtractedAt
					tt.expected[i].ChildChunks[j].Hash = chunks[i].ChildChunks[j].Hash
				}
			}

			assert.Equal(t, tt.expected, chunks)
		})
	}
}

func TestPythonErrorHandling(t *testing.T) {
	ctx := context.Background()
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	parser, err := factory.CreateParser(ctx, pythonLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name        string
		code        string
		expectError bool
	}{
		{
			name:        "InvalidPythonSyntax",
			code:        `def func(:`,
			expectError: true,
		},
		{
			name:        "UnsupportedLanguageDetection",
			code:        `invalid language construct`,
			expectError: true,
		},
		{
			name:        "NilParseTreeHandling",
			code:        ``,
			expectError: false,
		},
		{
			name:        "MalformedASTNodeHandling",
			code:        `class A: def method(self):`,
			expectError: true,
		},
		{
			name: "IndentationErrorHandling",
			code: `def func():
return 1`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portResult, err := parser.Parse(ctx, []byte(tt.code))

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if portResult.ParseTree != nil {
					domainTree, err := treesitter.ConvertPortParseTreeToDomain(portResult.ParseTree)
					require.NoError(t, err)

					adapter := treesitter.NewSemanticTraverserAdapter()
					options := outbound.SemanticExtractionOptions{
						IncludePrivate:       true,
						IncludeDocumentation: true,
					}
					chunks, err := adapter.ExtractClasses(ctx, domainTree, options)
					require.NoError(t, err)
					assert.NotNil(t, chunks)
				}
			}
		})
	}
}
