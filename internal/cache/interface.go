package cache

import (
	"context"

	"github.com/ihgazi/vectorproxy/internal/store"
)

// SemanticCache defines the interface for checking and updating cached search queries.
type SemanticCache interface {
	// Get checks the cache for a semantically similar query vector.
	// Returns the cached response and true if a hit is found, or nil and false if it is a miss.
	Get(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error)

	// Set stores the query vector and associated search response in the cache.
	Set(ctx context.Context, collection string, vector []float32, resp *store.SearchResponse) error

	// Invalidate deletes all cached entries for a specific collection.
	Invalidate(ctx context.Context, collection string) error

	// Close terminates any active connections or resources used by the cache.
	Close() error
}
