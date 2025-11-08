package javascriptparser

import (
	"codechunking/internal/domain/valueobject"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindChildByTypeHelper(t *testing.T) {
	lang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	// Test with dynamic import
	parseTree := createMockParseTreeFromSource(t, lang, "const module = import('module');")

	// Get all call_expression nodes
	callNodes := parseTree.GetNodesByType("call_expression")
	t.Logf("Found %d call_expression nodes", len(callNodes))
	require.Len(t, callNodes, 1, "Should find exactly one call_expression")

	callNode := callNodes[0]
	t.Logf("call_expression has %d children", len(callNode.Children))
	for i, child := range callNode.Children {
		t.Logf("  Child %d: type='%s', text='%s'", i, child.Type, parseTree.GetNodeText(child))
	}

	// Test findChildByType for "import"
	importNode := findChildByType(callNode, "import")
	if importNode == nil {
		t.Error("findChildByType returned nil for 'import'")
	} else {
		t.Logf("Found import node: type='%s', text='%s'", importNode.Type, parseTree.GetNodeText(importNode))
		assert.Equal(t, "import", importNode.Type)
	}

	// Test findChildByType for "arguments"
	argsNode := findChildByType(callNode, "arguments")
	require.NotNil(t, argsNode, "Should find arguments node")
	t.Logf("Found arguments node with %d children", len(argsNode.Children))
}

func TestCommonJSRequireExtraction(t *testing.T) {
	lang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	// Test with CommonJS require
	parseTree := createMockParseTreeFromSource(t, lang, "const module1 = require('module1');")

	// Get all call_expression nodes
	callNodes := parseTree.GetNodesByType("call_expression")
	t.Logf("Found %d call_expression nodes", len(callNodes))
	require.Len(t, callNodes, 1, "Should find exactly one call_expression")

	callNode := callNodes[0]
	t.Logf("call_expression has %d children", len(callNode.Children))
	for i, child := range callNode.Children {
		t.Logf("  Child %d: type='%s', text='%s'", i, child.Type, parseTree.GetNodeText(child))
	}

	// Test findChildByType for "identifier"
	identNode := findChildByType(callNode, "identifier")
	require.NotNil(t, identNode, "Should find identifier node")

	functionName := parseTree.GetNodeText(identNode)
	t.Logf("Found identifier: text='%s'", functionName)
	assert.Equal(t, "require", functionName)
}
