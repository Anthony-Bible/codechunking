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

func TestJavaScriptInterfacesExtraction(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("JSDoc typedef interface", func(t *testing.T) {
		sourceCode := `
/**
 * @typedef {Object} UserType
 * @property {string} name - The user's name
 * @property {number} age - The user's age
 * @property {boolean} isActive - Whether the user is active
 */
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 1)

		interfaceType := interfaces[0]
		assert.Equal(t, "UserType", interfaceType.Name)
		assert.Equal(t, outbound.ConstructInterface, interfaceType.Type)

		// Check that metadata contains interface information
		assert.NotNil(t, interfaceType.Metadata)
		assert.Contains(t, interfaceType.Metadata, "jsdoc")

		// Check annotations are present
		assert.Len(t, interfaceType.Annotations, 1)
		assert.Equal(t, "typedef", interfaceType.Annotations[0].Name)
	})

	t.Run("Flow-style type alias", func(t *testing.T) {
		sourceCode := `
type UserType = {
	name: string,
	age: number,
	isActive: boolean
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 1)

		interfaceType := interfaces[0]
		assert.Equal(t, "UserType", interfaceType.Name)
		assert.Equal(t, outbound.ConstructInterface, interfaceType.Type)

		// Check that metadata contains type information
		assert.NotNil(t, interfaceType.Metadata)
		assert.Contains(t, interfaceType.Metadata, "typeAlias")
	})

	t.Run("Complex nested object types", func(t *testing.T) {
		sourceCode := `
/**
 * @typedef {Object} AddressType
 * @property {string} street - Street address
 * @property {string} city - City name
 * @property {string} country - Country name
 */

/**
 * @typedef {Object} UserType
 * @property {string} name - The user's name
 * @property {number} age - The user's age
 * @property {AddressType} address - The user's address
 * @property {Array<string>} hobbies - User's hobbies
 */
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 2)

		addressType := interfaces[0]
		assert.Equal(t, "AddressType", addressType.Name)
		assert.Equal(t, outbound.ConstructInterface, addressType.Type)

		userType := interfaces[1]
		assert.Equal(t, "UserType", userType.Name)
		assert.Equal(t, outbound.ConstructInterface, userType.Type)

		// Check that both have metadata
		assert.NotNil(t, addressType.Metadata)
		assert.NotNil(t, userType.Metadata)
	})

	t.Run("Function parameter object patterns", func(t *testing.T) {
		sourceCode := `
/**
 * Process user data
 * @param {Object} user - The user object
 * @param {string} user.name - User's name
 * @param {number} user.age - User's age
 * @param {boolean} user.isActive - User's active status
 */
function processUser(user) {
	// implementation
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 1)

		interfaceType := interfaces[0]
		assert.Equal(t, "user", interfaceType.Name)
		assert.Equal(t, outbound.ConstructInterface, interfaceType.Type)

		// Check metadata for parameter-based interface
		assert.NotNil(t, interfaceType.Metadata)
		assert.Contains(t, interfaceType.Metadata, "parameterInterface")
	})

	t.Run("Interface with optional properties", func(t *testing.T) {
		sourceCode := `
/**
 * @typedef {Object} OptionalPropsType
 * @property {string} requiredProp - A required property
 * @property {number} [optionalProp] - An optional property
 * @property {boolean} [anotherOptional] - Another optional property
 */
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 1)

		interfaceType := interfaces[0]
		assert.Equal(t, "OptionalPropsType", interfaceType.Name)
		assert.Equal(t, outbound.ConstructInterface, interfaceType.Type)

		// Check that metadata contains information about optional properties
		assert.NotNil(t, interfaceType.Metadata)
		assert.Contains(t, interfaceType.Metadata, "jsdoc")
	})

	t.Run("Union types in interfaces", func(t *testing.T) {
		sourceCode := `
/**
 * @typedef {Object} UnionTypeInterface
 * @property {string|number} id - The identifier
 * @property {"active"|"inactive"|"pending"} status - The status
 */
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 1)

		interfaceType := interfaces[0]
		assert.Equal(t, "UnionTypeInterface", interfaceType.Name)
		assert.Equal(t, outbound.ConstructInterface, interfaceType.Type)

		// Check metadata contains union type information
		assert.NotNil(t, interfaceType.Metadata)
		assert.Contains(t, interfaceType.Metadata, "jsdoc")
	})

	t.Run("Interface with method signatures", func(t *testing.T) {
		sourceCode := `
/**
 * @typedef {Object} MethodInterface
 * @property {string} name - The name
 * @property {function(): void} doSomething - A method
 * @property {function(string, number): boolean} validate - Another method
 */
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 1)

		interfaceType := interfaces[0]
		assert.Equal(t, "MethodInterface", interfaceType.Name)
		assert.Equal(t, outbound.ConstructInterface, interfaceType.Type)

		// Check metadata contains method signature information
		assert.NotNil(t, interfaceType.Metadata)
		assert.Contains(t, interfaceType.Metadata, "jsdoc")
	})

	t.Run("Generic-like interface patterns", func(t *testing.T) {
		sourceCode := `
/**
 * @typedef {Object} GenericLikeInterface
 * @property {Array<T>} items - Collection of items
 * @property {function(T): boolean} predicate - Filter function
 */
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 1)

		interfaceType := interfaces[0]
		assert.Equal(t, "GenericLikeInterface", interfaceType.Name)
		assert.Equal(t, outbound.ConstructInterface, interfaceType.Type)

		// Check metadata contains generic-like information
		assert.NotNil(t, interfaceType.Metadata)
		assert.Contains(t, interfaceType.Metadata, "jsdoc")
	})

	t.Run("Multiple interfaces in one file", func(t *testing.T) {
		sourceCode := `
/**
 * @typedef {Object} FirstInterface
 * @property {string} prop1 - First property
 */

type SecondInterface = {
	prop2: number
};

/**
 * @typedef {Object} ThirdInterface
 * @property {boolean} prop3 - Third property
 */
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 3)

		interface1 := interfaces[0]
		assert.Equal(t, "FirstInterface", interface1.Name)
		assert.Equal(t, outbound.ConstructInterface, interface1.Type)

		interface2 := interfaces[1]
		assert.Equal(t, "SecondInterface", interface2.Name)
		assert.Equal(t, outbound.ConstructInterface, interface2.Type)

		interface3 := interfaces[2]
		assert.Equal(t, "ThirdInterface", interface3.Name)
		assert.Equal(t, outbound.ConstructInterface, interface3.Type)

		// Check all have metadata
		assert.NotNil(t, interface1.Metadata)
		assert.NotNil(t, interface2.Metadata)
		assert.NotNil(t, interface3.Metadata)
	})

	t.Run("Interface inheritance patterns", func(t *testing.T) {
		sourceCode := `
/**
 * @typedef {Object} BaseInterface
 * @property {string} baseProp - Base property
 */

/**
 * @typedef {BaseInterface} ExtendedInterface
 * @property {number} extendedProp - Extended property
 */
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
			IncludeTypeInfo: true,
		}

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 2)

		baseInterface := interfaces[0]
		assert.Equal(t, "BaseInterface", baseInterface.Name)
		assert.Equal(t, outbound.ConstructInterface, baseInterface.Type)

		extendedInterface := interfaces[1]
		assert.Equal(t, "ExtendedInterface", extendedInterface.Name)
		assert.Equal(t, outbound.ConstructInterface, extendedInterface.Type)

		// Check metadata for inheritance information
		assert.NotNil(t, baseInterface.Metadata)
		assert.NotNil(t, extendedInterface.Metadata)
	})
}
