package engine

import (
	"context"

	"github.com/ihgazi/vectorproxy/internal/search"
)

// VectorStore defines the interface for a vector database.
// TODO: Implement vector Insert and Delete operations
type VectorStore interface {
	Search(ctx context.Context, req search.SearchQuery) (search.SearchResponse, error)
	Close() error
}
