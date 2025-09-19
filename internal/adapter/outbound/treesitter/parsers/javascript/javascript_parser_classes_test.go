package javascriptparser

import (
	treesitter "codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJavaScriptClassExtraction_SimpleES6Class(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
class SimpleClass {
	constructor(name) {
		this.name = name;
	}

	getName() {
		return this.name;
	}

	setName(name) {
		this.name = name;
	}
}
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 1)

	class := classes[0]
	assert.Equal(t, "SimpleClass", class.Name)
	assert.Equal(t, outbound.ConstructClass, class.Type)

	functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
	require.NoError(t, err)

	var classMethods []outbound.SemanticCodeChunk
	for _, fn := range functions {
		if fn.ParentChunk != nil && fn.ParentChunk.Name == "SimpleClass" {
			classMethods = append(classMethods, fn)
		}
	}
	assert.Len(t, classMethods, 3)

	constructor := classMethods[0]
	assert.Equal(t, "constructor", constructor.Name)
	assert.Equal(t, outbound.ConstructMethod, constructor.Type)
	assert.Len(t, constructor.Parameters, 1)
	assert.Equal(t, "name", constructor.Parameters[0].Name)

	getNameMethod := classMethods[1]
	assert.Equal(t, "getName", getNameMethod.Name)
	assert.Equal(t, outbound.ConstructMethod, getNameMethod.Type)
	assert.Empty(t, getNameMethod.Parameters)

	setNameMethod := classMethods[2]
	assert.Equal(t, "setName", setNameMethod.Name)
	assert.Equal(t, outbound.ConstructMethod, setNameMethod.Type)
	assert.Len(t, setNameMethod.Parameters, 1)
	assert.Equal(t, "name", setNameMethod.Parameters[0].Name)
}

func TestJavaScriptClassExtraction_Inheritance(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
class ParentClass {
	constructor(parentProp) {
		this.parentProp = parentProp;
	}
}

class ChildClass extends ParentClass {
	constructor(parentProp, childProp) {
		super(parentProp);
		this.childProp = childProp;
	}

	childMethod() {
		return this.childProp;
	}
}
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 2)

	var parentClass, childClass outbound.SemanticCodeChunk
	for _, class := range classes {
		switch class.Name {
		case "ParentClass":
			parentClass = class
		case "ChildClass":
			childClass = class
		}
	}

	assert.Equal(t, "ParentClass", parentClass.Name)
	assert.Equal(t, outbound.ConstructClass, parentClass.Type)

	assert.Equal(t, "ChildClass", childClass.Name)
	assert.Equal(t, outbound.ConstructClass, childClass.Type)

	functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
	require.NoError(t, err)

	var childClassMethods []outbound.SemanticCodeChunk
	for _, fn := range functions {
		if fn.ParentChunk != nil && fn.ParentChunk.Name == "ChildClass" {
			childClassMethods = append(childClassMethods, fn)
		}
	}
	assert.Len(t, childClassMethods, 2)
}

func TestJavaScriptClassExtraction_StaticMethods(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
class StaticClass {
	static staticMethod() {
		return "static";
	}

	static get staticProperty() {
		return "property";
	}

	static set staticProperty(value) {
		this._staticProperty = value;
	}

	instanceMethod() {
		return "instance";
	}
}
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 1)

	class := classes[0]
	assert.Equal(t, "StaticClass", class.Name)
	assert.Equal(t, outbound.ConstructClass, class.Type)

	functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
	require.NoError(t, err)

	var classMethods []outbound.SemanticCodeChunk
	for _, fn := range functions {
		if fn.ParentChunk != nil && fn.ParentChunk.Name == "StaticClass" {
			classMethods = append(classMethods, fn)
		}
	}
	assert.Len(t, classMethods, 3)

	staticMethod := classMethods[0]
	assert.Equal(t, "staticMethod", staticMethod.Name)
	assert.True(t, staticMethod.IsStatic)

	instanceMethod := classMethods[2]
	assert.Equal(t, "instanceMethod", instanceMethod.Name)
	assert.False(t, instanceMethod.IsStatic)
}

func TestJavaScriptClassExtraction_GettersAndSetters(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
class GetterSetterClass {
	constructor(value) {
		this._value = value;
	}

	get value() {
		return this._value;
	}

	set value(newValue) {
		this._value = newValue;
	}
}
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 1)

	class := classes[0]
	assert.Equal(t, "GetterSetterClass", class.Name)
	assert.Equal(t, outbound.ConstructClass, class.Type)

	functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
	require.NoError(t, err)

	var classMethods []outbound.SemanticCodeChunk
	for _, fn := range functions {
		if fn.ParentChunk != nil && fn.ParentChunk.Name == "GetterSetterClass" {
			classMethods = append(classMethods, fn)
		}
	}
	assert.Len(t, classMethods, 2)

	getter := classMethods[0]
	assert.Equal(t, "value", getter.Name)
	assert.Equal(t, outbound.ConstructMethod, getter.Type)

	setter := classMethods[1]
	assert.Equal(t, "value", setter.Name)
	assert.Equal(t, outbound.ConstructMethod, setter.Type)
	assert.Len(t, setter.Parameters, 1)
	assert.Equal(t, "newValue", setter.Parameters[0].Name)
}

func TestJavaScriptClassExtraction_PrivateFields(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
class PrivateClass {
	#privateField = "private";
	#privateMethod() {
		return this.#privateField;
	}

	publicMethod() {
		return this.#privateMethod();
	}
}
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 1)

	class := classes[0]
	assert.Equal(t, "PrivateClass", class.Name)
	assert.Equal(t, outbound.ConstructClass, class.Type)

	functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
	require.NoError(t, err)

	var classMethods []outbound.SemanticCodeChunk
	for _, fn := range functions {
		if fn.ParentChunk != nil && fn.ParentChunk.Name == "PrivateClass" {
			classMethods = append(classMethods, fn)
		}
	}
	assert.Len(t, classMethods, 2)

	privateMethod := classMethods[0]
	assert.Equal(t, "#privateMethod", privateMethod.Name)
	assert.Equal(t, outbound.Private, privateMethod.Visibility)

	publicMethod := classMethods[1]
	assert.Equal(t, "publicMethod", publicMethod.Name)
	assert.Equal(t, outbound.Public, publicMethod.Visibility)
}

func TestJavaScriptClassExtraction_Decorators(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
@decorator
@anotherDecorator({ option: true })
class DecoratedClass {
	method() {}
}
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 1)

	class := classes[0]
	assert.Equal(t, "DecoratedClass", class.Name)
	assert.Equal(t, outbound.ConstructClass, class.Type)
	assert.Len(t, class.Annotations, 2)

	decorator1 := class.Annotations[0]
	assert.Equal(t, "decorator", decorator1.Name)

	decorator2 := class.Annotations[1]
	assert.Equal(t, "anotherDecorator", decorator2.Name)
}

func TestJavaScriptClassExtraction_AbstractClasses(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
class AbstractClass {
	constructor() {
		if (this.constructor === AbstractClass) {
			throw new Error("Cannot instantiate abstract class");
		}
	}

	abstractMethod() {
		throw new Error("Abstract method must be implemented");
	}
}
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 1)

	class := classes[0]
	assert.Equal(t, "AbstractClass", class.Name)
	assert.Equal(t, outbound.ConstructClass, class.Type)
	assert.True(t, class.IsAbstract)
}

func TestJavaScriptClassExtraction_NestedClasses(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
class OuterClass {
	constructor() {
		this.inner = class InnerClass {
			innerMethod() {}
		};
	}

	static NestedClass = class {
		nestedMethod() {}
	}
}
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 3)

	var outerClass, innerClass, nestedClass outbound.SemanticCodeChunk
	for _, class := range classes {
		switch class.Name {
		case "OuterClass":
			outerClass = class
		case "InnerClass":
			innerClass = class
		case "NestedClass":
			nestedClass = class
		}
	}

	assert.Equal(t, "OuterClass", outerClass.Name)
	assert.Equal(t, outbound.ConstructClass, outerClass.Type)

	assert.Equal(t, "InnerClass", innerClass.Name)
	assert.Equal(t, outbound.ConstructClass, innerClass.Type)

	assert.Equal(t, "NestedClass", nestedClass.Name)
	assert.Equal(t, outbound.ConstructClass, nestedClass.Type)

	functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
	require.NoError(t, err)

	var innerClassMethods, nestedClassMethods []outbound.SemanticCodeChunk
	for _, fn := range functions {
		if fn.ParentChunk != nil {
			switch fn.ParentChunk.Name {
			case "InnerClass":
				innerClassMethods = append(innerClassMethods, fn)
			case "NestedClass":
				nestedClassMethods = append(nestedClassMethods, fn)
			}
		}
	}
	assert.Len(t, innerClassMethods, 1)
	assert.Len(t, nestedClassMethods, 1)
}

func TestJavaScriptClassExtraction_ClassExpressionsBasic(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `
const MyClass = class {
	constructor() {}
	method() {}
};

const NamedClass = class NamedClassExpression {
	constructor() {}
	namedMethod() {}
};
`
	domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	classes, err := adapter.ExtractClasses(ctx, domainTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 2)

	var anonymousClass, namedClass outbound.SemanticCodeChunk
	for _, class := range classes {
		switch class.Name {
		case "":
			anonymousClass = class
		case "NamedClassExpression":
			namedClass = class
		}
	}

	assert.Equal(t, outbound.ConstructClass, anonymousClass.Type)

	assert.Equal(t, "NamedClassExpression", namedClass.Name)
	assert.Equal(t, outbound.ConstructClass, namedClass.Type)

	functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
	require.NoError(t, err)

	var anonymousClassMethods, namedClassMethods []outbound.SemanticCodeChunk
	for _, fn := range functions {
		if fn.ParentChunk != nil {
			switch fn.ParentChunk.Name {
			case "":
				anonymousClassMethods = append(anonymousClassMethods, fn)
			case "NamedClassExpression":
				namedClassMethods = append(namedClassMethods, fn)
			}
		}
	}
	assert.Len(t, anonymousClassMethods, 2)
	assert.Len(t, namedClassMethods, 2)
}
