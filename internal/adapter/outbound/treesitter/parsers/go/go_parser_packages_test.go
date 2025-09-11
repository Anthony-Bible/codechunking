package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoParserFocusedPackageExtraction(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name           string
		sourceCode     string
		expectedChunks []outbound.SemanticCodeChunk
	}{
		{
			name: "Package declaration with documentation",
			sourceCode: `
// Package mathutils provides mathematical utility functions
package mathutils

// Add adds two integers
func Add(a, b int) int {
	return a + b
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "package:mathutils",
					Type:          outbound.ConstructPackage,
					Name:          "mathutils",
					QualifiedName: "mathutils",
					Documentation: "Package mathutils provides mathematical utility functions",
					Content:       "package mathutils",
					StartByte:     55,
					EndByte:       72,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Package with build tags",
			sourceCode: `
//go:build linux && amd64
// +build linux,amd64

// Package osutils provides OS-specific utilities
package osutils
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "package:osutils",
					Type:          outbound.ConstructPackage,
					Name:          "osutils",
					QualifiedName: "osutils",
					Documentation: "Package osutils provides OS-specific utilities",
					Content:       "package osutils",
					StartByte:     55,
					EndByte:       70,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Package with file-level documentation",
			sourceCode: `
// This file contains utility functions for string manipulation
// and processing.

package stringutils

// Reverse reverses a string
func Reverse(s string) string {
	// implementation
	return s
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "package:stringutils",
					Type:          outbound.ConstructPackage,
					Name:          "stringutils",
					QualifiedName: "stringutils",
					Documentation: "This file contains utility functions for string manipulation and processing.",
					Content:       "package stringutils",
					StartByte:     87,
					EndByte:       106,
					Language:      valueobject.Go,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree, err := parser.Parse(ctx, []byte(tt.sourceCode))
			require.NoError(t, err)
			require.NotNil(t, parseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
			require.NoError(t, err)
			require.NotNil(t, domainTree)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := &treesitter.SemanticExtractionOptions{
				IncludePackages: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only package chunks for this test
			var packageChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructPackage {
					packageChunks = append(packageChunks, chunk)
				}
			}

			assert.Len(t, packageChunks, len(tt.expectedChunks))

			for i, expected := range tt.expectedChunks {
				if i < len(packageChunks) {
					actual := packageChunks[i]
					assert.Equal(t, expected.ChunkID, actual.ChunkID)
					assert.Equal(t, expected.Type, actual.Type)
					assert.Equal(t, expected.Name, actual.Name)
					assert.Equal(t, expected.QualifiedName, actual.QualifiedName)
					assert.Equal(t, expected.Documentation, actual.Documentation)
					assert.Equal(t, expected.Content, actual.Content)
					assert.Equal(t, expected.StartByte, actual.StartByte)
					assert.Equal(t, expected.EndByte, actual.EndByte)
					assert.Equal(t, expected.Language, actual.Language)
				}
			}
		})
	}
}
