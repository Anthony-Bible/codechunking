package javascriptparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithWhitespace(t *testing.T) {
	lang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	// Test with leading newline like in the actual test
	sourceCode := `
const module = import('module');
const module2 = await import('module2');
`
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

	require.Len(t, imports, 2, "Should extract 2 dynamic imports")
}
