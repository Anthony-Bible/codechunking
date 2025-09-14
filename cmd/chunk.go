package cmd

import (
	"codechunking/internal/adapter/outbound/chunking"
	"codechunking/internal/adapter/outbound/embeddings/simple"
	ts "codechunking/internal/adapter/outbound/treesitter"
	_ "codechunking/internal/adapter/outbound/treesitter/parsers/go" // Import to register Go parser
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// chunkCmd implements: codechunking chunk --file path.go [--lang go] [--out out.json].
func newChunkCmd() *cobra.Command {
	var filePath string
	var langFlag string
	var outPath string

	cmd := &cobra.Command{
		Use:   "chunk",
		Short: "Parse, extract, chunk and embed a single source file",
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(filePath) == "" {
				return errors.New("--file is required")
			}
			if langFlag == "" {
				langFlag = valueobject.LanguageGo
			}
			return runChunk(filePath, langFlag, outPath)
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Path to source file (required)")
	cmd.Flags().StringVar(&langFlag, "lang", valueobject.LanguageGo, "Language hint (e.g., Go)")
	cmd.Flags().StringVar(&outPath, "out", "", "Optional path to write JSON output")

	_ = cmd.MarkFlagRequired("file")

	return cmd
}

// runChunk performs: parse -> extract functions -> chunk -> embed -> output JSON.
func runChunk(filePath, langName, outPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	enhanced, lang, err := parseAndChunkSource(ctx, filePath, langName)
	if err != nil {
		return err
	}

	outList, err := generateEmbeddings(ctx, enhanced)
	if err != nil {
		return err
	}

	return outputResults(filePath, lang, outList, len(enhanced), outPath)
}

// parseAndChunkSource handles file reading, parsing, function extraction, and chunking.
func parseAndChunkSource(
	ctx context.Context,
	filePath, langName string,
) ([]outbound.EnhancedCodeChunk, valueobject.Language, error) {
	// Read file content
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("read file: %w", err)
	}

	// Prepare language
	lang, err := valueobject.NewLanguage(langName)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("language: %w", err)
	}

	// Build parser factory and parser
	factory, err := ts.NewTreeSitterParserFactory(ctx)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("init parser factory: %w", err)
	}

	parser, err := factory.CreateParser(ctx, lang)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("create parser: %w", err)
	}

	// Parse source
	parseOpts := ts.ParseOptions{
		IncludeComments:   true,
		IncludeWhitespace: false,
		MaxDepth:          0,
		TimeoutMs:         5000,
		EnableStatistics:  true,
		FilePath:          filePath,
		Language:          lang.Name(),
	}
	pr, err := parser.ParseSource(ctx, lang, src, parseOpts)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("parse source: %w", err)
	}
	if pr == nil || pr.ParseTree == nil {
		return nil, valueobject.Language{}, errors.New("no parse tree produced")
	}

	// Convert to domain parse tree for traverser
	domainTree, err := ts.ConvertPortParseTreeToDomain(pr.ParseTree)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("convert parse tree: %w", err)
	}

	// Extract functions
	traverser := ts.NewSemanticTraverserAdapter()
	extractOpts := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeComments:      false,
		IncludeDocumentation: false,
		MaxDepth:             0,
	}
	funcs, err := traverser.ExtractFunctions(ctx, domainTree, extractOpts)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("extract functions: %w", err)
	}

	// Chunk by function
	chunker := chunking.NewFunctionChunker()
	cfg := outbound.ChunkingConfiguration{
		Strategy:             outbound.StrategyFunction,
		ContextPreservation:  outbound.PreserveMinimal,
		MaxChunkSize:         4000,
		MinChunkSize:         200,
		OverlapSize:          0,
		IncludeDocumentation: false,
		IncludeComments:      false,
		PreserveDependencies: false,
		EnableSplitting:      true,
		QualityThreshold:     0.0,
	}
	enhanced, err := chunker.ChunkByFunction(ctx, funcs, cfg)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("chunking: %w", err)
	}

	return enhanced, lang, nil
}

// generateEmbeddings creates embeddings for all enhanced code chunks.
func generateEmbeddings(ctx context.Context, enhanced []outbound.EnhancedCodeChunk) ([]chunkOut, error) {
	embedder := simple.New()
	outList := make([]chunkOut, 0, len(enhanced))

	for i := range enhanced {
		ch := &enhanced[i]
		// Create default embedding options
		options := outbound.EmbeddingOptions{
			TaskType: outbound.TaskTypeSemanticSimilarity,
			Timeout:  30 * time.Second,
		}

		result, err := embedder.GenerateEmbedding(ctx, ch.Content, options)
		if err != nil {
			return nil, fmt.Errorf("embed chunk %s: %w", ch.ID, err)
		}
		preview := ch.Content
		if len(preview) > 160 {
			preview = preview[:160]
		}
		outList = append(outList, chunkOut{
			EnhancedCodeChunk: *ch,
			Embedding:         result.Vector,
			EmbeddingDim:      len(result.Vector),
			ContentPreview:    preview,
		})
	}

	return outList, nil
}

// outputResults formats and writes the results to output.
func outputResults(
	filePath string,
	lang valueobject.Language,
	outList []chunkOut,
	funcCount int,
	outPath string,
) error {
	payload := map[string]any{
		"file":                filePath,
		"language":            lang.Name(),
		"functions_extracted": funcCount,
		"chunks":              outList,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}

	if outPath == "" {
		_, _ = os.Stdout.Write(data)
		_, _ = os.Stdout.WriteString("\n")
	} else {
		if err := os.WriteFile(outPath, data, 0o600); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		slogger.InfoNoCtx("Wrote chunk output", slogger.Fields{"path": outPath})
	}

	return nil
}

// chunkOut represents the output structure for each code chunk.
type chunkOut struct {
	outbound.EnhancedCodeChunk

	Embedding      []float64 `json:"embedding"`
	EmbeddingDim   int       `json:"embedding_dim"`
	ContentPreview string    `json:"content_preview"`
}

func init() { //nolint:gochecknoinits // required by cobra for command registration
	rootCmd.AddCommand(newChunkCmd())
}
