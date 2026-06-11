package search

import "context"

// SearchQuery respresents a search request within a specific database collection.
// The user can provide either a raw query text or the corresponding vector embedding.
// The embedding will be generated internally, if not provided.
type SearchQuery struct {
	Collection string
	// The raw query text to be used for search
	Query string
	// Optional pre-computed vector embedding for the query
	Vector []float32
	TopK   int32
	Filter map[string]string
}

// SearchResult represents a single search result returned from the vector store.
type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]any
}

// SearchResponse encapsulates the results of a search query (top K matches).
type SearchResponse struct {
	Results  []SearchResult
	MaxLimit bool // MaxLimit indicates if the search results covers the maximum matches from the vector store
}

// VectorStore defines the interface for a vector database.
// TODO: Implement vector Insert and Delete operations
type VectorStore interface {
	Search(ctx context.Context, req *SearchQuery) (*SearchResponse, error)
	SearchBatch(ctx context.Context, reqs []*SearchQuery) ([]*SearchResponse, error)
	Close() error
}
