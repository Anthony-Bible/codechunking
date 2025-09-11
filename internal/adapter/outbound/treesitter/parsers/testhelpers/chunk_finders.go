package testhelpers

import (
	"codechunking/internal/port/outbound"
)

// FindChunkByType finds the first chunk of the specified type from a slice of chunks.
// Returns nil if no chunk of the specified type is found.
func FindChunkByType(
	chunks []outbound.SemanticCodeChunk,
	chunkType outbound.SemanticConstructType,
) *outbound.SemanticCodeChunk {
	for _, chunk := range chunks {
		if chunk.Type == chunkType {
			return &chunk
		}
	}
	return nil
}

// FindChunksByType finds all chunks of the specified type from a slice of chunks.
// Returns an empty slice if no chunks of the specified type are found.
func FindChunksByType(
	chunks []outbound.SemanticCodeChunk,
	chunkType outbound.SemanticConstructType,
) []outbound.SemanticCodeChunk {
	var result []outbound.SemanticCodeChunk
	for _, chunk := range chunks {
		if chunk.Type == chunkType {
			result = append(result, chunk)
		}
	}
	return result
}

// FindChunkByName finds the first chunk with the specified name from a slice of chunks.
// Returns nil if no chunk with the specified name is found.
func FindChunkByName(chunks []outbound.SemanticCodeChunk, name string) *outbound.SemanticCodeChunk {
	for i, chunk := range chunks {
		if chunk.Name == name {
			return &chunks[i]
		}
	}
	return nil
}
