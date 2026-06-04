package cache

import (
	"context"

	"github.com/ihgazi/vectorproxy/internal/search"
)

// SemanticCache defines the interface for checking and updating cached search queries.
type SemanticCache interface {
	// Get checks the cache for a semantically similar query vector.
	// Returns the cached response and true if a hit is found, or nil and false if it is a miss.
	Get(ctx context.Context, collection string, vector []float32) (*search.SearchResponse, bool, error)

	// Set stores the query vector and associated search response in the cache.
	Set(ctx context.Context, collection string, vector []float32, resp *search.SearchResponse) error

	// Close terminates any active connections or resources used by the cache.
	Close() error
}
