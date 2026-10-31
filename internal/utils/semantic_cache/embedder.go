package semantic_cache

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// GenerateEmbedding produces a deterministic 64-dimension vector from the
// input text via SHA-256. NOTE: this is NOT a semantic embedding — it only
// supports exact-match caching (identical inputs produce identical vectors;
// similar-but-different inputs will NOT match). The name "embedding" is
// retained for API compatibility. A future upgrade should replace this with
// a real embedding model via the system's embedding relay pipeline (H-42).
func GenerateEmbedding(text string) []float64 {
	h := sha256.Sum256([]byte(text))
	embedding := make([]float64, 64)
	for i := 0; i < 32; i++ {
		// Each byte pair → one float64 in [-1, 1]
		val := float64(binary.BigEndian.Uint16(h[i*2 : i*2+2]))
		embedding[i] = val/32768.0 - 1.0
	}
	// Normalize to unit vector
	var norm float64
	for _, v := range embedding {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	return embedding
}
