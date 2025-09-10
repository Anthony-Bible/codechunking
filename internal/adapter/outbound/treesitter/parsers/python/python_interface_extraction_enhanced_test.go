package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findMethodByName finds a method chunk by name within child chunks.
func findMethodByName(chunks []outbound.SemanticCodeChunk, name string) *outbound.SemanticCodeChunk {
	for i, chunk := range chunks {
		if chunk.Name == name && chunk.Type == outbound.ConstructMethod {
			return &chunks[i]
		}
	}
	return nil
}

func TestPythonParser_ExtractInterfaces_DebugParseTree(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	source := `
@runtime_checkable
class MyProtocol(Protocol):
    def method1(self) -> str: ...
    def method2(self, x: int) -> bool: ...
`

	parseTree := createMockParseTreeFromSource(t, language, source)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	t.Logf("Number of interfaces found: %d", len(interfaces))
	for i, iface := range interfaces {
		t.Logf("Interface %d: %+v", i, iface)
	}

	assert.Fail(t, "This test is for debugging - implement proper assertions once we understand what's being found")
}

func TestPythonParser_ExtractInterfaces_DebugOriginalCode(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	sourceCode := `# protocols.py

from typing import Protocol, runtime_checkable
from abc import ABC, abstractmethod

@runtime_checkable
class Drawable(Protocol):
    """Protocol for drawable objects."""
    
    def draw(self) -> None:
        ...
    
    def get_area(self) -> float:
        ...

class Shape(ABC):
    """Abstract base class for shapes."""
    
    @abstractmethod
    def area(self) -> float:
        pass
    
    @abstractmethod
    def perimeter(self) -> float:
        pass
    
    def describe(self) -> str:
        return f"A shape with area {self.area()}"

class Serializable(Protocol):
    def serialize(self) -> dict:
        ...
    
    def deserialize(self, data: dict) -> None:
        ...
`

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	t.Logf("Number of interfaces found: %d", len(interfaces))
	for i, iface := range interfaces {
		t.Logf("Interface %d: Name=%s, Type=%s, Dependencies=%v, Annotations=%v",
			i, iface.Name, iface.Type, iface.Dependencies, iface.Annotations)
	}

	// Debug: Check what class_definition nodes exist
	classNodes := parseTree.GetNodesByType("class_definition")
	t.Logf("Found %d class_definition nodes", len(classNodes))
	for i, node := range classNodes {
		classContent := parseTree.GetNodeText(node)
		t.Logf("Class %d: %s", i, classContent)
	}

	assert.Fail(t, "Debug test - showing information about original test case interface extraction")
}

func TestPythonParser_ExtractInterfaces_Enhanced(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	source := `
from abc import ABC, abstractmethod

class Drawable(ABC):
    @abstractmethod
    def draw(self) -> None: ...

class Shape(Drawable):
    @abstractmethod
    def area(self) -> float: ...
    
    @abstractmethod
    def perimeter(self) -> float: ...

class Serializable(ABC):
    @abstractmethod
    def serialize(self) -> str: ...
`

	parseTree := createMockParseTreeFromSource(t, language, source)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.Len(t, interfaces, 3, "Should find exactly three interfaces")

	drawable := findChunkByName(interfaces, "Drawable")
	assert.NotNil(t, drawable, "Should find Drawable interface")
	if drawable != nil {
		assert.Equal(t, "Drawable", drawable.Name, "Interface name should match")
		assert.Equal(t, outbound.ConstructInterface, drawable.Type, "Interface should be of interface type")
		children := drawable.ChildChunks
		require.Len(t, children, 1, "Should have exactly one method")
		drawMethod := findMethodByName(children, "draw")
		assert.NotNil(t, drawMethod, "Should find draw method")
		if drawMethod != nil {
			assert.Equal(t, "draw", drawMethod.Name, "Method name should match")
			assert.Equal(t, "None", drawMethod.ReturnType, "Return type should match")
			assert.Contains(t, drawMethod.Annotations, "abstractmethod", "Method should be abstract")
		}
	}

	shape := findChunkByName(interfaces, "Shape")
	assert.NotNil(t, shape, "Should find Shape interface")
	if shape != nil {
		assert.Equal(t, "Shape", shape.Name, "Interface name should match")
		assert.Equal(t, outbound.ConstructInterface, shape.Type, "Interface should be of interface type")
		assert.Len(t, shape.Dependencies, 1, "Should have exactly one dependency")
		assert.Equal(t, "Drawable", shape.Dependencies[0], "Dependency should match")
		children := shape.ChildChunks
		require.Len(t, children, 2, "Should have exactly two methods")

		areaMethod := findMethodByName(children, "area")
		assert.NotNil(t, areaMethod, "Should find area method")
		if areaMethod != nil {
			assert.Equal(t, "area", areaMethod.Name, "Method name should match")
			assert.Equal(t, "float", areaMethod.ReturnType, "Return type should match")
			assert.Contains(t, areaMethod.Annotations, "abstractmethod", "Method should be abstract")
		}

		perimeterMethod := findMethodByName(children, "perimeter")
		assert.NotNil(t, perimeterMethod, "Should find perimeter method")
		if perimeterMethod != nil {
			assert.Equal(t, "perimeter", perimeterMethod.Name, "Method name should match")
			assert.Equal(t, "float", perimeterMethod.ReturnType, "Return type should match")
			assert.Contains(t, perimeterMethod.Annotations, "abstractmethod", "Method should be abstract")
		}
	}

	serializable := findChunkByName(interfaces, "Serializable")
	assert.NotNil(t, serializable, "Should find Serializable interface")
	if serializable != nil {
		assert.Equal(t, "Serializable", serializable.Name, "Interface name should match")
		assert.Equal(t, outbound.ConstructInterface, serializable.Type, "Interface should be of interface type")
		children := serializable.ChildChunks
		require.Len(t, children, 1, "Should have exactly one method")
		serializeMethod := findMethodByName(children, "serialize")
		assert.NotNil(t, serializeMethod, "Should find serialize method")
		if serializeMethod != nil {
			assert.Equal(t, "serialize", serializeMethod.Name, "Method name should match")
			assert.Equal(t, "str", serializeMethod.ReturnType, "Return type should match")
			assert.Contains(t, serializeMethod.Annotations, "abstractmethod", "Method should be abstract")
		}
	}
}

func TestPythonParser_ExtractInterfaces_MultipleInheritance(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	source := `
from abc import ABC
from typing import Protocol

class MyProtocol(Protocol, SomeBaseClass):
    def method1(self) -> str: ...

class AnotherProtocol(SomeBase, Protocol, ThirdBase):
    def method2(self) -> int: ...

class ABCProtocol(ABC, Protocol):
    def method3(self) -> bool: ...
`

	parseTree := createMockParseTreeFromSource(t, language, source)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.Len(t, interfaces, 3, "Should find three interfaces")

	iface1 := findChunkByName(interfaces, "MyProtocol")
	assert.NotNil(t, iface1, "Should find MyProtocol interface")
	if iface1 != nil {
		assert.Equal(t, outbound.ConstructInterface, iface1.Type, "Should be interface type")
		require.Len(t, iface1.Dependencies, 2, "Should have two dependencies")
		assert.Contains(t, iface1.Dependencies, "Protocol", "Should inherit from Protocol")
		assert.Contains(t, iface1.Dependencies, "SomeBaseClass", "Should inherit from SomeBaseClass")
		children := iface1.ChildChunks
		require.Len(t, children, 1, "Should have one method")
	}

	iface2 := findChunkByName(interfaces, "AnotherProtocol")
	assert.NotNil(t, iface2, "Should find AnotherProtocol interface")
	if iface2 != nil {
		assert.Equal(t, outbound.ConstructInterface, iface2.Type, "Should be interface type")
		require.Len(t, iface2.Dependencies, 3, "Should have three dependencies")
		assert.Contains(t, iface2.Dependencies, "SomeBase", "Should inherit from SomeBase")
		assert.Contains(t, iface2.Dependencies, "Protocol", "Should inherit from Protocol")
		assert.Contains(t, iface2.Dependencies, "ThirdBase", "Should inherit from ThirdBase")
		children := iface2.ChildChunks
		require.Len(t, children, 1, "Should have one method")
	}

	iface3 := findChunkByName(interfaces, "ABCProtocol")
	assert.NotNil(t, iface3, "Should find ABCProtocol interface")
	if iface3 != nil {
		assert.Equal(t, outbound.ConstructInterface, iface3.Type, "Should be identified as interface")
		require.Len(t, iface3.Dependencies, 2, "Should have two dependencies")
		assert.Contains(t, iface3.Dependencies, "ABC", "Should inherit from ABC")
		assert.Contains(t, iface3.Dependencies, "Protocol", "Should inherit from Protocol")
		children := iface3.ChildChunks
		require.Len(t, children, 1, "Should have one method")
	}
}

func TestPythonParser_ExtractInterfaces_MixedABCProtocol(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	source := `
from abc import ABC, abstractmethod
from typing import Protocol

class MixedInterface(ABC, Protocol):
    @abstractmethod
    def method1(self) -> str: ...
    
    def concrete_method(self) -> int:
        return 42

class AnotherMixed(BaseClass, ABC, Protocol):
    @abstractmethod
    def abstract_method(self): ...
`

	parseTree := createMockParseTreeFromSource(t, language, source)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.Len(t, interfaces, 2, "Should find two mixed interfaces")

	iface1 := findChunkByName(interfaces, "MixedInterface")
	assert.NotNil(t, iface1, "Should find MixedInterface")
	if iface1 != nil {
		assert.Equal(
			t,
			outbound.ConstructInterface,
			iface1.Type,
			"Should be identified as interface despite ABC inheritance",
		)
		children := iface1.ChildChunks
		require.Len(t, children, 2, "Should have two methods")

		abstractMethod := findMethodByName(children, "method1")
		assert.NotNil(t, abstractMethod, "Should find abstract method1")
		if abstractMethod != nil {
			assert.Contains(t, abstractMethod.Annotations, "abstractmethod", "method1 should be marked as abstract")
		}

		concreteMethod := findMethodByName(children, "concrete_method")
		assert.NotNil(t, concreteMethod, "Should find concrete_method")
		if concreteMethod != nil {
			assert.NotContains(
				t,
				concreteMethod.Annotations,
				"abstractmethod",
				"concrete_method should not be abstract",
			)
		}
	}

	iface2 := findChunkByName(interfaces, "AnotherMixed")
	assert.NotNil(t, iface2, "Should find AnotherMixed")
	if iface2 != nil {
		assert.Equal(t, outbound.ConstructInterface, iface2.Type, "Should be identified as interface")
		children := iface2.ChildChunks
		abstractMethod := findMethodByName(children, "abstract_method")
		assert.NotNil(t, abstractMethod, "Should find abstract_method")
		if abstractMethod != nil {
			assert.Contains(
				t,
				abstractMethod.Annotations,
				"abstractmethod",
				"abstract_method should be marked as abstract",
			)
		}
	}
}

func TestPythonParser_ExtractInterfaces_ComplexDecorators(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	source := `
from dataclasses import dataclass
from typing import Protocol

@dataclass
@runtime_checkable
class DecoratedProtocol(Protocol):
    @property
    @abstractmethod
    def readonly_prop(self) -> str: ...
    
    @staticmethod
    @abstractmethod
    def static_method() -> bool: ...
    
    @classmethod
    @abstractmethod
    def class_method(cls) -> int: ...
    
    @custom_decorator
    @abstractmethod
    def custom_decorated_method(self): ...
`

	parseTree := createMockParseTreeFromSource(t, language, source)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.Len(t, interfaces, 1, "Should find one interface")
	iface := interfaces[0]
	assert.Equal(t, "DecoratedProtocol", iface.Name, "Interface name should match")
	assert.Contains(t, iface.Annotations, "runtime_checkable", "Interface should be runtime checkable")
	children := iface.ChildChunks
	require.Len(t, children, 4, "Should have four methods")

	prop := findMethodByName(children, "readonly_prop")
	assert.NotNil(t, prop, "Should find readonly_prop")
	if prop != nil {
		assert.Contains(t, prop.Annotations, "abstractmethod", "readonly_prop should be abstract")
		assert.Equal(t, "str", prop.ReturnType, "Return type should match")
	}

	staticMethod := findMethodByName(children, "static_method")
	assert.NotNil(t, staticMethod, "Should find static_method")
	if staticMethod != nil {
		assert.Contains(t, staticMethod.Annotations, "abstractmethod", "static_method should be abstract")
		assert.Contains(t, staticMethod.Annotations, "staticmethod", "static_method should be static")
	}

	classMethod := findMethodByName(children, "class_method")
	assert.NotNil(t, classMethod, "Should find class_method")
	if classMethod != nil {
		assert.Contains(t, classMethod.Annotations, "abstractmethod", "class_method should be abstract")
		assert.Contains(t, classMethod.Annotations, "classmethod", "class_method should be class method")
	}

	customMethod := findMethodByName(children, "custom_decorated_method")
	assert.NotNil(t, customMethod, "Should find custom_decorated_method")
	if customMethod != nil {
		assert.Contains(t, customMethod.Annotations, "abstractmethod", "custom_decorated_method should be abstract")
	}
}

func TestPythonParser_ExtractInterfaces_NestedProtocols(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	source := `
from typing import Protocol

class OuterClass:
    class NestedProtocol(Protocol):
        def nested_method(self) -> str: ...
        
    class InnerClass:
        class DeeplyNestedProtocol(Protocol):
            def deep_method(self) -> int: ...

def outer_function():
    class LocalProtocol(Protocol):
        def local_method(self): ...
`

	parseTree := createMockParseTreeFromSource(t, language, source)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.Len(t, interfaces, 3, "Should find three protocols")

	nested := findChunkByName(interfaces, "NestedProtocol")
	assert.NotNil(t, nested, "Should find NestedProtocol")
	if nested != nil {
		assert.Equal(
			t,
			"OuterClass.NestedProtocol",
			nested.QualifiedName,
			"Should have qualified name with outer class",
		)
	}

	deep := findChunkByName(interfaces, "DeeplyNestedProtocol")
	assert.NotNil(t, deep, "Should find DeeplyNestedProtocol")
	if deep != nil {
		assert.Equal(
			t,
			"OuterClass.InnerClass.DeeplyNestedProtocol",
			deep.QualifiedName,
			"Should have deeply qualified name",
		)
	}

	local := findChunkByName(interfaces, "LocalProtocol")
	assert.NotNil(t, local, "Should find LocalProtocol")
	if local != nil {
		assert.Equal(
			t,
			"outer_function.LocalProtocol",
			local.QualifiedName,
			"Should have qualified name with function",
		)
	}
}

func TestPythonParser_ExtractInterfaces_EdgeCases(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	source := `
from typing import Protocol

@runtime_checkable
class RuntimeProtocol(Protocol):
    def concrete_method(self) -> str:
        return "implementation"

class _PrivateProtocol(Protocol):
    def private_method(self): ...

class GenericProtocol(Protocol[T, U]):
    def generic_method(self, item: T) -> U: ...

class EmptyProtocol(Protocol):
    pass

class EmptyProtocolWithEllipsis(Protocol):
    ...

class ProtocolWithDocstring(Protocol):
    """This is a protocol with docstring"""
    def method_with_docstring(self): ...
`

	parseTree := createMockParseTreeFromSource(t, language, source)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.Len(t, interfaces, 6, "Should find six interfaces")

	runtime := findChunkByName(interfaces, "RuntimeProtocol")
	assert.NotNil(t, runtime, "Should find RuntimeProtocol")
	if runtime != nil {
		assert.Contains(t, runtime.Annotations, "runtime_checkable", "Should be marked as runtime_checkable")
		children := runtime.ChildChunks
		concreteMethod := findMethodByName(children, "concrete_method")
		assert.NotNil(t, concreteMethod, "Should find concrete_method")
		if concreteMethod != nil {
			assert.NotContains(
				t,
				concreteMethod.Annotations,
				"abstractmethod",
				"Concrete method should not be abstract",
			)
		}
	}

	private := findChunkByName(interfaces, "_PrivateProtocol")
	assert.NotNil(t, private, "Should find _PrivateProtocol")
	if private != nil {
		// Note: In Python, the concept of "private" is by convention only (leading underscore)
		// We could check if the name starts with underscore if needed
		assert.Equal(t, "_PrivateProtocol", private.Name, "Should preserve private naming convention")
	}

	generic := findChunkByName(interfaces, "GenericProtocol")
	assert.NotNil(t, generic, "Should find GenericProtocol")
	if generic != nil {
		// The base class information would be in Dependencies
		assert.Contains(t, generic.Dependencies, "Protocol[T, U]", "Should preserve generic type parameters")
	}

	empty1 := findChunkByName(interfaces, "EmptyProtocol")
	assert.NotNil(t, empty1, "Should find EmptyProtocol")
	if empty1 != nil {
		children := empty1.ChildChunks
		assert.Empty(t, children, "Empty protocol should have no methods")
	}

	empty2 := findChunkByName(interfaces, "EmptyProtocolWithEllipsis")
	assert.NotNil(t, empty2, "Should find EmptyProtocolWithEllipsis")
	if empty2 != nil {
		children := empty2.ChildChunks
		assert.Empty(t, children, "Empty protocol with ellipsis should have no methods")
	}

	withDocstring := findChunkByName(interfaces, "ProtocolWithDocstring")
	assert.NotNil(t, withDocstring, "Should find ProtocolWithDocstring")
	if withDocstring != nil {
		assert.Equal(
			t,
			"This is a protocol with docstring",
			withDocstring.Documentation,
			"Should have docstring documentation",
		)
		children := withDocstring.ChildChunks
		assert.Len(t, children, 1, "Should have one method despite docstring")
	}
}
