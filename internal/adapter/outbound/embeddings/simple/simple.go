package simple

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
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
func (g *Generator) GenerateEmbedding(_ context.Context, text string) ([]float64, error) {
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

	return out, nil
}
