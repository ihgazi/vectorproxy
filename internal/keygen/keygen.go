package keygen

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"

	"github.com/ihgazi/vectorproxy/internal/search"
)

// KeyGenerator defines the function signature for extracting a deduplication key.
// Returning an empty string will bypass the coalescing logic.
type KeyGenerator func(req *search.SearchQuery) string

// NewStringKeyGenerator creates a key based on collection and raw query text.
func NewStringKeyGenerator() KeyGenerator {
	return func(req *search.SearchQuery) string {
		if req.Query == "" {
			return ""
		}

		return fmt.Sprintf("str:%s:%s", req.Collection, req.Query)
	}
}

// NewVectorKeyGenerator creates a key based on collection and vector hash.
func NewVectorKeyGenerator() KeyGenerator {
	return func(req *search.SearchQuery) string {
		if len(req.Vector) == 0 {
			return ""
		}

		return fmt.Sprintf("vec:%s:%s", req.Collection, HashVector(req.Vector))
	}
}

// HashVector hashes the vector slice using FNV-1a
func HashVector(vec []float32) string {
	h := fnv.New64a()
	buf := make([]byte, 4)
	for _, f := range vec {
		binary.LittleEndian.PutUint32(buf, math.Float32bits(f))

		h.Write(buf)
	}
	return fmt.Sprintf("%x", h.Sum64())
}
