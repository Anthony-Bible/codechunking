package javascriptparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDynamicImportExtractionDebug(t *testing.T) {
	lang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `const module = import('module');`
	parseTree := createMockParseTreeFromSource(t, lang, sourceCode)

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	// Call the public function through the adapter
	imports, err := ExtractJavaScriptImports(ctx, parseTree, options)
	require.NoError(t, err)

	t.Logf("Extracted %d imports", len(imports))
	for i, imp := range imports {
		t.Logf("Import %d: path='%s', metadata=%v", i, imp.Path, imp.Metadata)
	}

	require.NotEmpty(t, imports, "Should extract at least one dynamic import")
}

func TestCommonJSExtractionDebug(t *testing.T) {
	lang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	sourceCode := `const module1 = require('module1');`
	parseTree := createMockParseTreeFromSource(t, lang, sourceCode)

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeMetadata: true,
	}

	imports, err := ExtractJavaScriptImports(ctx, parseTree, options)
	require.NoError(t, err)

	t.Logf("Extracted %d imports", len(imports))
	for i, imp := range imports {
		t.Logf("Import %d: path='%s', metadata=%v", i, imp.Path, imp.Metadata)
	}

	require.NotEmpty(t, imports, "Should extract at least one CommonJS import")
}
