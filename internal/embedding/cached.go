package embedding

import (
	"context"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/ihgazi/vectorproxy/internal/metrics"
)

// CachedEmbedder wraps an existing Embedder and caches exact string matches
// using an in-memory LRU cache.
type CachedEmbedder struct {
	base  Embedder
	cache *lru.Cache[string, []float32]
}

// NewCachedEmbedder creates a new CachedEmbedder wrapping the provided base Embedder.
// capacity dictates the maximum number of string-to-vector mappings to hold in memory.
func NewCachedEmbedder(base Embedder, capacity int) (*CachedEmbedder, error) {
	cache, err := lru.New[string, []float32](capacity)
	if err != nil {
		return nil, err
	}
	return &CachedEmbedder{
		base:  base,
		cache: cache,
	}, nil
}

// Embed generates an embedding for a single text string. It checks the LRU cache
// first, and if a hit occurs, immediately returns the cached vector. On a miss,
// it calls the underlying embedder and caches the result.
func (c *CachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if vec, ok := c.cache.Get(text); ok {
		metrics.CacheLookupsTotal.WithLabelValues("embedding_cache", "hit").Inc()
		return vec, nil
	}

	metrics.CacheLookupsTotal.WithLabelValues("embedding_cache", "miss").Inc()

	vec, err := c.base.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	c.cache.Add(text, vec)
	return vec, nil
}

// EmbedBatch generates embeddings for a batch of strings. As per the design,
// batch calls bypass the cache entirely and pass through to the base embedder.
func (c *CachedEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return c.base.EmbedBatch(ctx, texts)
}
