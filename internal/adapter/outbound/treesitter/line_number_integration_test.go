package treesitter_test

// RED PHASE - Issue #58: Integration test for end-to-end line number correctness.
//
// This test uses testdata/multiline.go, a fixture file where FunctionAtLineFifty
// is deliberately placed at line 50. The test asserts that after a full parse
// pipeline (ParseDirectory -> CodeChunk), the resulting chunk reports StartLine == 50.
//
// FAILS because all three bugs combine in the end-to-end path:
//   1. Go parser never populates StartPosition/EndPosition.
//   2. convertSemanticToCodeChunks uses chunk.Content (not full file) for byte scan.
//   3. startByte >> len(chunk.Content), so estimateLineNumber returns the wrong line.

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	goparser "codechunking/internal/adapter/outbound/treesitter/parsers/go"
)

// TestMain re-registers the real Go parser before each test run. This is needed
// because internal package tests (treesitter_pipeline_test.go) overwrite the registry
// with mock parsers via setupTestParsers(), which would cause our integration tests
// to get zero semantic chunks.
func TestMain(m *testing.M) {
	treesitter.RegisterParser("Go", goparser.NewGoParser)
	m.Run()
}

// TestParseDirectory_FunctionAtLineFifty_ReportsCorrectStartLine is an end-to-end
// integration test that parses testdata/multiline.go through the full pipeline and
// asserts the produced CodeChunk for FunctionAtLineFifty has StartLine == 50.
//
// FAILS because the current pipeline reports wrong line numbers for functions that
// do not start at the beginning of the file. See bug description in file header.
func TestParseDirectory_FunctionAtLineFifty_ReportsCorrectStartLine(t *testing.T) {
	treesitter.RegisterParser("Go", goparser.NewGoParser)

	// Locate testdata/multiline.go relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	testdataDir := filepath.Join(filepath.Dir(thisFile), "testdata")

	// Verify the fixture exists.
	fixturePath := filepath.Join(testdataDir, "multiline.go")
	_, err := os.Stat(fixturePath)
	require.NoError(t, err, "testdata/multiline.go fixture must exist; run from repo root")

	ctx := context.Background()
	parser, err := treesitter.NewTreeSitterCodeParser(ctx)
	require.NoError(t, err)

	config := outbound.CodeParsingConfig{
		ChunkSizeBytes:   1024 * 64, // large enough to not split the fixture
		MaxFileSizeBytes: 1024 * 64,
		IncludeTests:     false,
		ExcludeVendor:    false,
	}

	chunks, err := parser.ParseDirectory(ctx, testdataDir, config)
	require.NoError(t, err)
	require.NotEmpty(t, chunks, "expected at least one chunk from testdata/")

	// Find the chunk for FunctionAtLineFifty.
	var target *outbound.CodeChunk
	for i := range chunks {
		if chunks[i].EntityName == "FunctionAtLineFifty" {
			target = &chunks[i]
			break
		}
	}

	require.NotNil(t, target,
		"no CodeChunk with EntityName=FunctionAtLineFifty found; chunks produced: %d", len(chunks))

	// EXPECTED: StartLine == 50 (function is at line 50 of testdata/multiline.go)
	// ACTUAL (buggy): estimateLineNumber(chunkContent, startByte) where startByte >> len(chunkContent)
	//                 returns strings.Count(chunkContent, "\n") + 1 which is ~6 (body line count), not 50.
	if target.StartLine != 50 {
		t.Errorf("StartLine: got %d, want 50; "+
			"FunctionAtLineFifty is at line 50 of testdata/multiline.go but the parser "+
			"reports wrong line numbers because it scans chunk content instead of full file content",
			target.StartLine)
	}

	// EndLine must be >= StartLine and > 50 (the function has a body).
	if target.EndLine < target.StartLine {
		t.Errorf("EndLine (%d) < StartLine (%d): line range is inverted",
			target.EndLine, target.StartLine)
	}

	if target.EndLine <= 50 {
		t.Errorf("EndLine: got %d, want > 50; "+
			"FunctionAtLineFifty has a multi-line body so EndLine must exceed StartLine",
			target.EndLine)
	}
}

// TestParseDirectory_MultipleGoFunctions_AllHaveCorrectLineNumbers verifies that when
// a Go file has multiple functions, every produced CodeChunk has a StartLine that matches
// the actual source line where its function begins, not the chunk-relative line count.
//
// FAILS because the buggy byte-scan truncates to chunk content length for all non-first
// functions, producing identical or wrong line numbers.
func TestParseDirectory_MultipleGoFunctions_AllHaveCorrectLineNumbers(t *testing.T) {
	treesitter.RegisterParser("Go", goparser.NewGoParser)

	// Write a temporary file with two functions at known lines.
	tempDir := t.TempDir()
	src := "package twofunc\n" + // line 1
		"\n" + // line 2
		"func EarlyFunc() {\n" + // line 3
		"\treturn\n" + // line 4
		"}\n" + // line 5
		"\n" + // line 6
		"\n" + // line 7
		"\n" + // line 8
		"\n" + // line 9
		"\n" + // line 10
		"func LateFunc() {\n" + // line 11
		"\tx := 1\n" + // line 12
		"\t_ = x\n" + // line 13
		"\treturn\n" + // line 14
		"}\n" // line 15

	err := os.WriteFile(filepath.Join(tempDir, "twofunc.go"), []byte(src), 0o644)
	require.NoError(t, err)

	ctx := context.Background()
	parser, err := treesitter.NewTreeSitterCodeParser(ctx)
	require.NoError(t, err)

	config := outbound.CodeParsingConfig{
		ChunkSizeBytes:   1024 * 64,
		MaxFileSizeBytes: 1024 * 64,
		IncludeTests:     false,
		ExcludeVendor:    false,
	}

	chunks, err := parser.ParseDirectory(ctx, tempDir, config)
	require.NoError(t, err)
	require.NotEmpty(t, chunks)

	byName := make(map[string]*outbound.CodeChunk)
	for i := range chunks {
		if chunks[i].EntityName != "" {
			name := chunks[i].EntityName
			byName[name] = &chunks[i]
		}
	}

	type want struct {
		startLine int
		endLine   int
	}

	expectations := map[string]want{
		"EarlyFunc": {startLine: 3, endLine: 5},
		"LateFunc":  {startLine: 11, endLine: 15},
	}

	for funcName, w := range expectations {
		chunk, ok := byName[funcName]
		if !ok {
			t.Errorf("no chunk found for function %q", funcName)
			continue
		}

		if chunk.StartLine != w.startLine {
			t.Errorf("%s: StartLine got %d, want %d; "+
				"byte-scan against chunk content (not full file) produces wrong line",
				funcName, chunk.StartLine, w.startLine)
		}

		if chunk.EndLine != w.endLine {
			t.Errorf("%s: EndLine got %d, want %d",
				funcName, chunk.EndLine, w.endLine)
		}
	}
}
