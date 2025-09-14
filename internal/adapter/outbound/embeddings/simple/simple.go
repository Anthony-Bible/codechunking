package simple

import (
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"time"
)

// Dimension of Gemini text-embedding-004 used in schema; keep same for POC.
const embedDim = 768

// Generator implements a deterministic stub EmbeddingGenerator.
// It produces a fixed-size vector seeded by the SHA256 of the input text.
// This avoids external network calls while exercising the pipeline.
type Generator struct{}

// New creates a new simple embedding generator.
func New() *Generator { return &Generator{} }

// GenerateEmbedding returns a deterministic 768-d float vector for the text.
// The values are in [-1, 1] via a simple PRNG derived from SHA256.
func (g *Generator) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	// Seed from SHA256(text)
	sum := sha256.Sum256([]byte(text))

	// Xorshift64* PRNG seeded from the hash (takes 8 bytes)
	// If the seed is zero, pick a non-zero constant.
	seed := binary.LittleEndian.Uint64(sum[:8])
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	x := seed

	out := make([]float64, embedDim)
	for i := range embedDim {
		// xorshift64*
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		x *= 0x2545F4914F6CDD1D

		// Map to [-1, 1]. Use upper 53 bits to make a float in [0,1).
		// Then scale and shift to [-1,1].
		mantissa := (x >> 11) & ((1 << 53) - 1) // 53-bit mantissa
		f := float64(mantissa) / float64(1<<53) // [0,1)
		out[i] = 2.0*f - 1.0
	}

	// Optional: L2 normalize for stability
	var norm float64
	for _, v := range out {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		inv := 1.0 / norm
		for i := range out {
			out[i] *= inv
		}
	}

	// Create EmbeddingResult to match the interface
	result := &outbound.EmbeddingResult{
		Vector:      out,
		Dimensions:  len(out),
		Model:       "simple-deterministic",
		TaskType:    outbound.TaskTypeRetrievalDocument,
		GeneratedAt: time.Now(),
	}

	return result, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts (not implemented in simple generator).
func (g *Generator) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	results := make([]*outbound.EmbeddingResult, len(texts))
	for i, text := range texts {
		result, err := g.GenerateEmbedding(ctx, text, options)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

// GenerateCodeChunkEmbedding generates an embedding specifically for a CodeChunk.
func (g *Generator) GenerateCodeChunkEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) (*outbound.CodeChunkEmbedding, error) {
	result, err := g.GenerateEmbedding(ctx, chunk.Content, options)
	if err != nil {
		return nil, err
	}

	return &outbound.CodeChunkEmbedding{
		EmbeddingResult:  result,
		ChunkID:          chunk.ID,
		SourceFile:       chunk.FilePath,
		Language:         chunk.Language,
		ChunkType:        "generic",
		QualityScore:     1.0,
		ComplexityScore:  1.0,
		EmbeddingVersion: "simple-v1",
	}, nil
}

// ValidateApiKey always returns nil for simple generator (no API key needed).
func (g *Generator) ValidateApiKey(ctx context.Context) error {
	return nil
}

// GetModelInfo returns information about the simple embedding model.
func (g *Generator) GetModelInfo(ctx context.Context) (*outbound.ModelInfo, error) {
	return &outbound.ModelInfo{
		Name:        "simple-deterministic",
		Version:     "1.0",
		MaxTokens:   10000,
		Dimensions:  embedDim,
		Description: "Simple deterministic embedding generator for testing",
	}, nil
}

// GetSupportedModels returns list of supported models (just the simple one).
func (g *Generator) GetSupportedModels(ctx context.Context) ([]string, error) {
	return []string{"simple-deterministic"}, nil
}

// EstimateTokenCount estimates tokens (simplified for testing).
func (g *Generator) EstimateTokenCount(ctx context.Context, text string) (int, error) {
	// Simple estimation: ~4 chars per token
	return len(text) / 4, nil
}
